package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const attachmentLimit = 10 * 1024 * 1024

func (s *Server) registerAttachments(mux *http.ServeMux) {
	for _, path := range []string{"/requests/{appoinmentID}/attachment", "/requests/drafts/{appointmentDraftID}/attachment"} {
		mux.HandleFunc("GET "+prefix+path, s.attachment)
		mux.HandleFunc("POST "+prefix+path, s.attachment)
	}
}

// Client filenames never become storage paths; MIME alone is not trusted.
func attachmentType(data []byte, name string) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	invalid := errors.New("формат или содержимое файла не разрешены")
	switch ext {
	case ".png", ".jpg", ".jpeg":
		cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
			return "", invalid
		}
		if (ext == ".png" && format != "png") || (ext != ".png" && format != "jpeg") {
			return "", invalid
		}
		// Decode the full stream too, so a truncated image is not accepted.
		if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
			return "", invalid
		}
		if format == "png" {
			return ".png", nil
		}
		return ".jpg", nil
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data[max(0, len(data)-1024):], []byte("%%EOF")) {
			return "", invalid
		}
		return ext, nil
	case ".docx":
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil || len(z.File) > 500 {
			return "", invalid
		}
		total := uint64(0)
		content, document := false, false
		for _, f := range z.File {
			total += f.UncompressedSize64
			if total > 40*1024*1024 || f.UncompressedSize64 > 10*1024*1024 {
				return "", invalid
			}
			lower := strings.ToLower(f.Name)
			if strings.Contains(lower, "..") || strings.HasPrefix(lower, "/") || strings.Contains(lower, "vbaproject") || strings.Contains(lower, "embeddings/") {
				return "", invalid
			}
			if lower == "[content_types].xml" || lower == "word/document.xml" {
				stream, e := f.Open()
				if e != nil {
					return "", invalid
				}
				d := xml.NewDecoder(io.LimitReader(stream, 10*1024*1024))
				for {
					_, e = d.Token()
					if e != nil {
						break
					}
				}
				stream.Close()
				if e != io.EOF {
					return "", invalid
				}
				if lower == "word/document.xml" {
					document = true
				} else {
					content = true
				}
			}
		}
		if !content || !document {
			return "", invalid
		}
		return ext, nil
	}
	return "", invalid
}

var attachmentScanSlots = make(chan struct{}, 2)

func attachmentScan(ctx context.Context, path string) error {
	select {
	case attachmentScanSlots <- struct{}{}:
		defer func() { <-attachmentScanSlots }()
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("Антивирус занят, повторите загрузку через несколько секунд")
	}
	scanner := os.Getenv("CLAMSCAN_PATH")
	if scanner == "" {
		scanner = "clamscan"
	}
	binary, e := exec.LookPath(scanner)
	if e != nil {
		return errors.New("антивирус недоступен")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	args := []string{"--no-summary"}
	if dir := os.Getenv("CLAMAV_DATABASE_DIR"); dir != "" {
		args = append(args, "--database="+dir)
	}
	args = append(args, "--", path)
	if e = exec.CommandContext(ctx, binary, args...).Run(); e != nil {
		return errors.New("файл не прошёл антивирусную проверку")
	}
	return nil
}
func (s *Server) attachment(w http.ResponseWriter, r *http.Request) {
	p, e := requestAuthorize(r, "STUDENT|TEACHER|DEAN|SUPPORT")
	if e != nil {
		requestError(w, e)
		return
	}
	draft := r.PathValue("appointmentDraftID") != ""
	id := r.PathValue("appoinmentID")
	table, key, columns := "appointment", "appoinmentID", requestColumns
	if draft {
		id = r.PathValue("appointmentDraftID")
		table, key, columns = "appointment_drafts", "appointmentDraftID", requestDraftColumns
	}
	if !requestUUID(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный UUID")
		return
	}
	tx, e := s.DB.Begin(r.Context())
	if e != nil {
		requestError(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	row, e := requestOne(r.Context(), tx, `SELECT `+columns+` FROM "`+table+`" WHERE "`+key+`"=$1 FOR UPDATE`, id)
	if e == nil {
		if draft {
			if !requestOwned(p, row) {
				e = requestBad(403, "Нет доступа к черновику")
			} else if r.Method == "POST" && row["draftStatus"] != "editing" {
				e = requestBad(409, "Черновик уже отправлен")
			}
		} else {
			e = requestAccess(r.Context(), tx, p, row)
		}
	}
	if e != nil {
		requestError(w, e)
		return
	}
	if r.Method == "GET" {
		s.serveAttachment(w, r, requestString(row["fileUrlAttachment"]))
		return
	}
	name, ok := s.saveAttachment(w, r, false, func(name string) error {
		_, err := tx.Exec(r.Context(), `UPDATE "`+table+`" SET "fileUrlAttachment"=$1,"`+map[bool]string{true: "updatedAt", false: "updated_at"}[draft]+`"=now() WHERE "`+key+`"=$2`, name, id)
		if err != nil {
			return err
		}
		return tx.Commit(r.Context())
	})
	// Старые файлы сохраняются: отправленный черновик может разделять файл с заявкой.
	if ok {
		write(w, 200, map[string]any{key: id, "fileUrlAttachment": name})
	}
}

func (s *Server) attachmentDir() string {
	if s.Config.UploadDir != "" {
		return s.Config.UploadDir
	}
	return "uploads"
}

func (s *Server) serveAttachment(w http.ResponseWriter, r *http.Request, stored string) {
	dir := s.attachmentDir()
	base := filepath.Base(stored)
	if stored == "" || stored != base || !requestUUID(strings.TrimSuffix(base, filepath.Ext(base))) {
		fail(w, 404, "NOT_FOUND", "Вложение отсутствует")
		return
	}
	f, e := os.Open(filepath.Join(dir, base))
	if e != nil {
		fail(w, 404, "NOT_FOUND", "Файл не найден")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		fail(w, 404, "NOT_FOUND", "Файл не найден")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="document`+filepath.Ext(base)+`"`)
	http.ServeContent(w, r, "document", info.ModTime(), f)
	return
}

func (s *Server) saveAttachment(w http.ResponseWriter, r *http.Request, documentsOnly bool,
	commit func(string) error) (string, bool) {
	dir := s.attachmentDir()
	r.Body = http.MaxBytesReader(w, r.Body, attachmentLimit+64*1024)
	if e := r.ParseMultipartForm(attachmentLimit + 64*1024); e != nil {
		fail(w, 413, "PAYLOAD_TOO_LARGE", "Некорректное вложение или превышен лимит 10 МБ")
		return "", false
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 || len(r.MultipartForm.Value) > 0 {
		fail(w, 400, "BAD_REQUEST", "Ожидается единственный файл file")
		return "", false
	}
	f, header, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "BAD_REQUEST", "Файл отсутствует")
		return "", false
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, attachmentLimit+1))
	if e != nil || len(data) > attachmentLimit {
		fail(w, 413, "PAYLOAD_TOO_LARGE", "Лимит вложения 10 МБ")
		return "", false
	}
	ext, e := attachmentType(data, header.Filename)
	if e != nil {
		fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", e.Error())
		return "", false
	}
	if documentsOnly && ext != ".pdf" && ext != ".docx" {
		fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Отчёт должен быть в PDF или DOCX")
		return "", false
	}
	mime := http.DetectContentType(data)
	allowed := map[string]string{".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".docx": "application/zip"}
	if !strings.HasPrefix(mime, allowed[ext]) {
		fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Содержимое не соответствует формату")
		return "", false
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		fail(w, 503, "STORAGE_UNAVAILABLE", "Хранилище недоступно")
		return "", false
	}
	name := uuid.NewString() + ext
	path := filepath.Join(dir, name)
	out, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		fail(w, 503, "STORAGE_UNAVAILABLE", "Хранилище недоступно")
		return "", false
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()
	_, e = out.Write(data)
	closeErr := out.Close()
	if e != nil || closeErr != nil {
		fail(w, 503, "STORAGE_UNAVAILABLE", "Не удалось сохранить файл")
		return "", false
	}
	if e = attachmentScan(r.Context(), path); e != nil {
		fail(w, 503, "FILE_SCAN_FAILED", e.Error())
		return "", false
	}
	if err := commit(name); err != nil {
		fail(w, 503, "STORAGE_UNAVAILABLE", "Не удалось сохранить вложение")
		return "", false
	}
	committed = true
	return name, true
}
