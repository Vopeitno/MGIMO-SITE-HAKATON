package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"mgimo/backend/internal/transcript"
)

func (s *Server) academicTranscript(w http.ResponseWriter, r *http.Request) {
	p := resourceGuard(w, r, "STUDENT")
	if p == nil {
		return
	}
	year, parseErr := transcript.ParseYear(r.URL.RawQuery)
	if parseErr != nil {
		fail(w, 400, "BAD_REQUEST", "Допустим только параметр year: номер года обучения или all")
		return
	}
	select {
	case s.transcriptSlots <- struct{}{}:
		defer func() { <-s.transcriptSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		fail(w, 429, "TOO_MANY_REQUESTS", "Формирование выписок занято. Повторите через несколько секунд")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		resourceError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	d := transcript.Data{GeneratedAt: time.Now()}
	var enrolled time.Time
	err = tx.QueryRow(ctx, `SELECT st."studentName",st."studentNumber"::text,g."groupNumber"::text,COALESCE(f.title,''),COALESCE(d."directionName",''),st."studentEnrollmentDate" AT TIME ZONE 'Europe/Moscow' FROM students st JOIN groups g ON g."groupID"=st."groupID" LEFT JOIN faculties f ON f."facultyID"=st."facultyID" LEFT JOIN direction d ON d."directionID"=st."directionID" WHERE st."studentID"=NULLIF($1,'')::uuid`, p.StudentID).Scan(&d.Name, &d.Number, &d.Group, &d.Faculty, &d.Direction, &enrolled)
	if err != nil {
		resourceError(w, err)
		return
	}
	if year == -1 {
		year = transcript.CurrentYear(enrolled, d.GeneratedAt)
	}
	d.Period = transcript.PeriodLabel(year)
	rows, err := tx.Query(ctx, `SELECT c.title,c.semester,c.hours,cs.score,cs.credit FROM courses c LEFT JOIN effective_course_scores cs ON cs."courseID"=c."courseID" AND cs."studentID"=NULLIF($1,'')::uuid WHERE EXISTS(SELECT 1 FROM student_course_access a WHERE a."courseID"=c."courseID" AND a."studentID"=NULLIF($1,'')::uuid) AND ($2::int=0 OR (c.semester BETWEEN $2::int*2-1 AND $2::int*2)) ORDER BY c.semester,c.title,c."courseID" LIMIT 2001`, p.StudentID, year)
	if err != nil {
		resourceError(w, err)
		return
	}
	for rows.Next() {
		var c transcript.Course
		if err = rows.Scan(&c.Title, &c.Semester, &c.Hours, &c.Score, &c.Credit); err != nil {
			break
		}
		d.Courses = append(d.Courses, c)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		resourceError(w, err)
		return
	}
	pdf, err := transcript.Render(ctx, d)
	if err != nil {
		fail(w, 500, "PDF_ERROR", "Не удалось сформировать выписку. Повторите позже")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="academic-transcript.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.WriteHeader(200)
	_, _ = w.Write(pdf)
}

// transcriptPeriods returns only curriculum years available to the authenticated student.
func (s *Server) transcriptPeriods(w http.ResponseWriter, r *http.Request) {
	p := resourceGuard(w, r, "STUDENT")
	if p == nil {
		return
	}
	if r.URL.RawQuery != "" {
		fail(w, 400, "BAD_REQUEST", "Параметры не поддерживаются")
		return
	}
	var enrolled time.Time
	err := s.DB.QueryRow(r.Context(), `SELECT "studentEnrollmentDate" AT TIME ZONE 'Europe/Moscow' FROM students WHERE "studentID"=NULLIF($1,'')::uuid`, p.StudentID).Scan(&enrolled)
	if err != nil {
		resourceError(w, err)
		return
	}
	currentYear := transcript.CurrentYear(enrolled, time.Now())
	rows, err := s.DB.Query(r.Context(), `SELECT DISTINCT (c.semester+1)/2 AS year FROM courses c WHERE c.semester>0 AND EXISTS(SELECT 1 FROM student_course_access a WHERE a."studentID"=NULLIF($1,'')::uuid AND a."courseID"=c."courseID") ORDER BY year`, p.StudentID)
	if err != nil {
		resourceError(w, err)
		return
	}
	defer rows.Close()
	curriculum := []int{}
	for rows.Next() {
		var year int
		if err = rows.Scan(&year); err != nil {
			resourceError(w, err)
			return
		}
		curriculum = append(curriculum, year)
	}
	if err = rows.Err(); err != nil {
		resourceError(w, err)
		return
	}
	years := transcript.AvailableYears(currentYear, curriculum)
	write(w, 200, map[string]any{"currentYear": currentYear, "years": years})
}
