package api

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/skip2/go-qrcode"
	"golang.org/x/image/draw"
)

const profilePhotoLimit = 2 << 20

func (s *Server) profilePhoto(w http.ResponseWriter, r *http.Request, p *Principal) {
	var jpeg []byte
	err := s.DB.QueryRow(r.Context(), `SELECT jpeg FROM student_profile_photos WHERE "studentID"=$1::uuid`, p.StudentID).Scan(&jpeg)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(jpeg)
}

func (s *Server) updateProfilePhoto(w http.ResponseWriter, r *http.Request, p *Principal) {
	r.Body = http.MaxBytesReader(w, r.Body, profilePhotoLimit+(32<<10))
	parts, err := r.MultipartReader()
	if err != nil {
		fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Ожидается изображение в поле file")
		return
	}
	part, err := parts.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		fail(w, 400, "BAD_REQUEST", "Выберите файл изображения")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(part, profilePhotoLimit+1))
	if len(raw) > profilePhotoLimit {
		fail(w, 413, "PAYLOAD_TOO_LARGE", "Фото должно быть не больше 2 МБ")
		return
	}
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Не удалось прочитать файл")
		return
	}
	if _, err := parts.NextPart(); !errors.Is(err, io.EOF) {
		fail(w, 400, "BAD_REQUEST", "Разрешено загрузить один файл")
		return
	}
	jpeg, err := normalizeProfilePhoto(raw)
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", err.Error())
		return
	}
	_, err = s.DB.Exec(r.Context(), `INSERT INTO student_profile_photos ("studentID",jpeg) VALUES($1::uuid,$2)
        ON CONFLICT ("studentID") DO UPDATE SET jpeg=excluded.jpeg,"updatedAt"=now()`, p.StudentID, jpeg)
	if err != nil {
		resourceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(jpeg)
}

// Re-encode the image to remove metadata and store only a small square avatar.
func normalizeProfilePhoto(raw []byte) ([]byte, error) {
	info, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, errors.New("Выберите настоящее изображение JPG или PNG")
	}
	if info.Width < 1 || info.Height < 1 || info.Width > 6000 || info.Height > 6000 ||
		int64(info.Width)*int64(info.Height) > 16000000 {
		return nil, errors.New("Слишком большое разрешение изображения")
	}
	source, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("Файл изображения повреждён")
	}
	bounds := source.Bounds()
	square := min(bounds.Dx(), bounds.Dy())
	topLeft := bounds.Min.Add(image.Pt((bounds.Dx()-square)/2, (bounds.Dy()-square)/2))
	crop := image.Rectangle{Min: topLeft, Max: topLeft.Add(image.Pt(square, square))}
	side := min(square, 512)
	target := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(target, target.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.BiLinear.Scale(target, target.Bounds(), source, crop, draw.Over, nil)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, target, &jpeg.Options{Quality: 88}); err != nil {
		return nil, errors.New("Не удалось обработать изображение")
	}
	return encoded.Bytes(), nil
}

func (s *Server) profileQR(w http.ResponseWriter, r *http.Request, p *Principal) {
	var name string
	var number int
	err := s.DB.QueryRow(r.Context(), `SELECT "studentName","studentNumber" FROM students WHERE "studentID"=$1::uuid`, p.StudentID).Scan(&name, &number)
	if err != nil {
		resourceError(w, err)
		return
	}
	// Encode a shareable student card without connection details or contacts.
	name = strings.Join(strings.Fields(name), " ")
	card := "ФИО: " + name + "\nНомер студента: " + strconv.Itoa(number)
	png, err := qrcode.Encode(card, qrcode.Medium, 300)
	if err != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось сформировать QR-профиль")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}
