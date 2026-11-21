package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"mgimo/backend/internal/config"
	"mgimo/backend/internal/transcript"
)

var portfolioRoutes = []studyRoute{
	{"GET", "/portfolio/resume.pdf", "STUDENT", (*Server).portfolioResume},
	{"GET", "/portfolio", "STUDENT", (*Server).portfolioRead},
	{"PUT", "/portfolio", "STUDENT", (*Server).portfolioSaveBio},
	{"PUT", "/portfolio/sharing", "STUDENT", (*Server).portfolioSharing},
	{"POST", "/portfolio/items", "STUDENT", (*Server).portfolioItemSave},
	{"PUT", "/portfolio/items/{id}", "STUDENT", (*Server).portfolioItemSave},
	{"DELETE", "/portfolio/items/{id}", "STUDENT", (*Server).portfolioItemDelete},
}

const publicPortfolioPath = prefix + "/public/portfolios/{id}"
const publicResumePath = publicPortfolioPath + "/resume.pdf"

const portfolioFields = `'displayName',st."studentName", 'bio',COALESCE(p.bio,''), 'items',COALESCE((
	SELECT jsonb_agg(jsonb_build_object('id',item.id,'kind',item.kind,'title',item.title,
		'description',item.description,'url',item.url) ORDER BY item.created_at DESC,item.id)
	FROM student_portfolio_items item WHERE item.student_id=st."studentID"
),'[]'::jsonb)`

const portfolioOwnerSource = ` FROM students st LEFT JOIN student_portfolios p ON p.student_id=st."studentID"
	WHERE st."studentID"=$1`

const portfolioPublicSource = ` FROM student_portfolios p JOIN students st ON st."studentID"=p.student_id
	WHERE p.share_id=$1 AND EXISTS (SELECT 1 FROM accounts a WHERE a."studentID"=p.student_id
		AND a."isActive" AND a."accountRole"='STUDENT')`

func (s *Server) portfolioRead(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `SELECT jsonb_build_object(`+portfolioFields+`,'shareID',p.share_id)`+portfolioOwnerSource, []any{current(r).StudentID})
}

func (s *Server) portfolioPublic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	id := r.PathValue("id")
	if !resourceUUID(id) {
		fail(w, 404, "NOT_FOUND", "Портфолио по этой ссылке недоступно")
		return
	}
	s.academicRead(w, r, `SELECT jsonb_build_object(`+portfolioFields+`)`+portfolioPublicSource, []any{id})
}

func (s *Server) portfolioResume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	source := portfolioPublicSource
	if id == "" {
		id = current(r).StudentID
		source = portfolioOwnerSource
	} else {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		if !resourceUUID(id) {
			fail(w, 404, "NOT_FOUND", "Портфолио по этой ссылке недоступно")
			return
		}
	}

	select {
	case s.transcriptSlots <- struct{}{}:
		defer func() { <-s.transcriptSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		fail(w, 429, "TOO_MANY_REQUESTS", "Формирование документов занято. Повторите через несколько секунд")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var raw json.RawMessage
	err := s.DB.QueryRow(ctx, `SELECT jsonb_build_object(`+portfolioFields+`)`+source, id).Scan(&raw)
	if err != nil {
		resourceError(w, err)
		return
	}
	var data transcript.Resume
	if err := json.Unmarshal(raw, &data); err != nil {
		fail(w, 500, "PDF_ERROR", "Не удалось прочитать данные резюме")
		return
	}
	pdf, err := transcript.RenderResume(ctx, data)
	if err != nil {
		fail(w, 500, "PDF_ERROR", "Не удалось сформировать резюме")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="student-resume.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.WriteHeader(200)
	_, _ = w.Write(pdf)
}

func (s *Server) portfolioSaveBio(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Bio *string `json:"bio"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Bio == nil {
		fail(w, 422, "VALIDATION_ERROR", "Передайте описание портфолио")
		return
	}
	bio := strings.TrimSpace(*body.Bio)
	if utf8.RuneCountInString(bio) > 1500 {
		fail(w, 422, "VALIDATION_ERROR", "Описание должно содержать до 1500 символов")
		return
	}
	_, err := s.DB.Exec(r.Context(), `INSERT INTO student_portfolios(student_id,bio) VALUES($1,$2)
		ON CONFLICT(student_id) DO UPDATE SET bio=EXCLUDED.bio`, current(r).StudentID, bio)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 204, nil)
}

func (s *Server) portfolioSharing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Published *bool `json:"published"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Published == nil {
		fail(w, 422, "VALIDATION_ERROR", "Укажите доступность портфолио")
		return
	}
	var shareID *string
	if *body.Published {
		id := uuid.NewString()
		shareID = &id
	}
	err := s.DB.QueryRow(r.Context(), `INSERT INTO student_portfolios(student_id,share_id) VALUES($1,$2)
		ON CONFLICT(student_id) DO UPDATE SET share_id=CASE WHEN EXCLUDED.share_id IS NULL THEN NULL
			ELSE COALESCE(student_portfolios.share_id,EXCLUDED.share_id) END RETURNING share_id::text`,
		current(r).StudentID, shareID).Scan(&shareID)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, map[string]any{"shareID": shareID})
}

func (s *Server) portfolioItemSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind        string `json:"kind"`
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}
	if !decode(w, r, &body) {
		return
	}
	switch body.Kind {
	case "project", "certificate", "achievement", "publication", "volunteering", "skill":
	default:
		fail(w, 422, "VALIDATION_ERROR", "Выберите тип записи")
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	if !serviceText(&body.Title, 150) || !serviceText(&body.Description, 3000) ||
		(body.URL != "" && (len(body.URL) > 1000 || !config.ValidSubmissionURL(body.URL))) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название, описание и при необходимости HTTPS-ссылку")
		return
	}
	ctx := r.Context()
	student := current(r).StudentID
	if r.Method == http.MethodPost {
		id := uuid.NewString()
		_, err := s.DB.Exec(ctx, `INSERT INTO student_portfolio_items(id,student_id,kind,title,description,url)
			VALUES($1,$2,$3,$4,$5,NULLIF($6,''))`, id, student, body.Kind, body.Title, body.Description, body.URL)
		if err != nil {
			resourceError(w, err)
			return
		}
		write(w, 201, map[string]string{"id": id})
		return
	}
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(ctx, `UPDATE student_portfolio_items SET kind=$3,title=$4,description=$5,url=NULLIF($6,'')
		WHERE id=$1 AND student_id=$2`, id, student, body.Kind, body.Title, body.Description, body.URL)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Запись не найдена")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) portfolioItemDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(r.Context(), `DELETE FROM student_portfolio_items WHERE id=$1 AND student_id=$2`, id, current(r).StudentID)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Запись не найдена")
	} else {
		write(w, 204, nil)
	}
}
