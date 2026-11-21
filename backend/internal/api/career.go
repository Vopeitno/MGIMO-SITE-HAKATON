package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"mgimo/backend/internal/config"
)

var careerRoutes = []studyRoute{
	{"GET", "/career/offers", "STUDENT|DEAN", (*Server).careerOffers},
	{"POST", "/career/offers", "DEAN", (*Server).careerOfferSave},
	{"PUT", "/career/offers/{id}", "DEAN", (*Server).careerOfferSave},
	{"DELETE", "/career/offers/{id}", "DEAN", (*Server).careerOfferArchive},
	{"GET", "/career/faculties", "DEAN", (*Server).careerFaculties},
	{"GET", "/career/eligibility", "STUDENT", (*Server).careerEligibility},
	{"GET", "/career/practices", "STUDENT|DEAN", (*Server).careerPractices},
	{"POST", "/career/practices", "DEAN", (*Server).careerPracticeAssign},
	{"PUT", "/career/practices/{id}/review", "DEAN", (*Server).careerPracticeReview},
	{"GET", "/career/practices/{id}/report", "STUDENT|DEAN", (*Server).careerPracticeReport},
	{"POST", "/career/practices/{id}/report", "STUDENT", (*Server).careerPracticeReport},
}

// Same September boundary and enrollment-year convention as transcript.CurrentYear.
const careerStudyYear = `greatest(1, extract(year from (now() AT TIME ZONE 'Europe/Moscow')-interval '8 months')::int
	- extract(year from st."studentEnrollmentDate")::int + 1)`

func (s *Server) careerFaculties(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, studyDeanFaculties, `title, "facultyID"`, current(r).EmployerID)
}

func (s *Server) careerEligibility(w http.ResponseWriter, r *http.Request) {
	var year int
	err := s.DB.QueryRow(r.Context(), `SELECT `+careerStudyYear+` FROM students st WHERE st."studentID"=$1`, current(r).StudentID).Scan(&year)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, map[string]any{"studyYear": year, "practiceAvailable": year >= 3})
}

const careerFacultyAccess = `EXISTS (
	SELECT 1 FROM faculties f JOIN deanery_staff ds USING ("deaneryID")
	WHERE f."facultyID"=ofr.faculty_id AND ds."employerID"=NULLIF($1,'')::uuid
)`

func (s *Server) careerOffers(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.portalPage(w, r, `SELECT ofr.id,ofr.kind,ofr.title,ofr.organization,ofr.location,ofr.description,ofr.url,
		published_at AS "publishedAt",faculty_id AS "facultyID",f.title AS faculty
		FROM career_offers ofr JOIN faculties f ON f."facultyID"=ofr.faculty_id
		WHERE archived_at IS NULL AND ((`+careerFacultyAccess+`) OR EXISTS (
			SELECT 1 FROM students st WHERE st."studentID"=NULLIF($2,'')::uuid AND st."facultyID"=ofr.faculty_id
		))`, `"publishedAt" DESC,id`, p.EmployerID, p.StudentID)
}

func (s *Server) careerOfferSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FacultyID    string `json:"facultyID"`
		Kind         string `json:"kind"`
		Title        string `json:"title"`
		Organization string `json:"organization"`
		Location     string `json:"location"`
		Description  string `json:"description"`
		URL          string `json:"url"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	if !resourceUUID(body.FacultyID) || (body.Kind != "vacancy" && body.Kind != "internship") ||
		!serviceText(&body.Title, 150) || !serviceText(&body.Organization, 160) || !serviceText(&body.Location, 120) ||
		!serviceText(&body.Description, 3000) || len(body.URL) > 1000 || !config.ValidSubmissionURL(body.URL) {
		fail(w, 422, "VALIDATION_ERROR", "Заполните поля и укажите ссылку для отклика по HTTPS")
		return
	}
	ctx := r.Context()
	employer := current(r).EmployerID
	id := uuid.NewString()
	if r.Method == http.MethodPost {
		tag, err := s.DB.Exec(ctx, `INSERT INTO career_offers(id,faculty_id,kind,title,organization,location,description,url)
			SELECT $2, f."facultyID",$4,$5,$6,$7,$8,$9 FROM (`+studyDeanFaculties+`) f WHERE f."facultyID"=$3`,
			employer, id, body.FacultyID, body.Kind, body.Title, body.Organization, body.Location, body.Description, body.URL)
		if err != nil {
			resourceError(w, err)
		} else if tag.RowsAffected() == 0 {
			fail(w, 404, "NOT_FOUND", "Факультет недоступен")
		} else {
			write(w, 201, map[string]string{"id": id})
		}
		return
	}
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(ctx, `UPDATE career_offers ofr SET faculty_id=$3,kind=$4,title=$5,organization=$6,
		location=$7,description=$8,url=$9 WHERE ofr.id=$2 AND archived_at IS NULL AND `+careerFacultyAccess+`
		AND EXISTS (SELECT 1 FROM (`+studyDeanFaculties+`) permitted WHERE permitted."facultyID"=$3)`,
		employer, id, body.FacultyID, body.Kind, body.Title, body.Organization, body.Location, body.Description, body.URL)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Предложение или факультет недоступны")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) careerOfferArchive(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(r.Context(), `UPDATE career_offers ofr SET archived_at=COALESCE(archived_at,now())
		WHERE id=$2 AND `+careerFacultyAccess, current(r).EmployerID, id)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Предложение недоступно")
	} else {
		write(w, 204, nil)
	}
}

const careerPracticeAccess = `(
	(p.student_id=NULLIF($2,'')::uuid AND ` + careerStudyYear + `>=3) OR EXISTS (
		SELECT 1 FROM (` + studyDeanFaculties + `) permitted WHERE permitted."facultyID"=st."facultyID"
	)
)`

func (s *Server) careerPractices(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.portalPage(w, r, `SELECT p.id, p.organization,starts_on AS "startsOn",ends_on AS "endsOn",
		report_due_on AS "reportDueOn",state,feedback,report_file IS NOT NULL AS "hasReport",
		st."studentNumber",st."studentName"
		FROM career_practices p JOIN students st ON st."studentID"=p.student_id
		WHERE `+careerPracticeAccess, `"startsOn" DESC,id`, emptyUUID(p.EmployerID), p.StudentID)
}

func emptyUUID(value string) string {
	if value == "" {
		return uuid.Nil.String()
	}
	return value
}

func (s *Server) careerPracticeAssign(w http.ResponseWriter, r *http.Request) {
	var body struct {
		StudentNumber int32  `json:"studentNumber"`
		Organization  string `json:"organization"`
		StartsOn      string `json:"startsOn"`
		EndsOn        string `json:"endsOn"`
		ReportDueOn   string `json:"reportDueOn"`
	}
	if !decode(w, r, &body) {
		return
	}
	start, e1 := time.Parse(time.DateOnly, body.StartsOn)
	end, e2 := time.Parse(time.DateOnly, body.EndsOn)
	due, e3 := time.Parse(time.DateOnly, body.ReportDueOn)
	if body.StudentNumber < 1 || !serviceText(&body.Organization, 160) || e1 != nil || e2 != nil || e3 != nil ||
		end.Before(start) || due.Before(end) {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте номер студента, организацию и порядок дат")
		return
	}
	id := uuid.NewString()
	tag, err := s.DB.Exec(r.Context(), `INSERT INTO career_practices(id,student_id,organization,starts_on,ends_on,report_due_on)
		SELECT $2,st."studentID",$4,$5,$6,$7 FROM students st
		JOIN (`+studyDeanFaculties+`) f ON f."facultyID"=st."facultyID"
		WHERE st."studentNumber"=$3 AND st."studentStatus"='active' AND `+careerStudyYear+`>=3`,
		current(r).EmployerID, id, body.StudentNumber, body.Organization, start, end, due)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Активный студент вашего факультета с третьего курса не найден")
	} else {
		write(w, 201, map[string]string{"id": id})
	}
}

func (s *Server) careerPracticeReview(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	var body struct {
		State    string `json:"state"`
		Feedback string `json:"feedback"`
	}
	if !decode(w, r, &body) {
		return
	}
	valid := serviceText(&body.Feedback, 2000)
	if (body.State != "accepted" && body.State != "revision") || (!valid && (body.Feedback != "" || body.State == "revision")) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите решение и комментарий для доработки")
		return
	}
	tag, err := s.DB.Exec(r.Context(), `UPDATE career_practices p SET state=$4,feedback=NULLIF($5,'')
		FROM students st WHERE p.student_id=st."studentID" AND p.id=$3 AND state='submitted' AND `+careerPracticeAccess,
		current(r).EmployerID, "", id, body.State, body.Feedback)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Отчёт недоступен или уже рассмотрен")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) careerPracticeReport(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	p := current(r)
	ctx := r.Context()
	query := `SELECT state, COALESCE(report_file,'') FROM career_practices p
		JOIN students st ON st."studentID"=p.student_id WHERE p.id=$3 AND ` + careerPracticeAccess
	args := []any{emptyUUID(p.EmployerID), p.StudentID, id}
	var state, previous string
	if r.Method == http.MethodGet {
		if err := s.DB.QueryRow(ctx, query, args...).Scan(&state, &previous); err != nil {
			resourceError(w, err)
			return
		}
		s.serveAttachment(w, r, previous)
		return
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		resourceError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, query+` FOR UPDATE OF p`, args...).Scan(&state, &previous)
	if err != nil {
		resourceError(w, err)
		return
	}
	if state != "assigned" && state != "revision" {
		fail(w, 409, "CONFLICT", "Отчёт уже отправлен или принят")
		return
	}
	_, saved := s.saveAttachment(w, r, true, func(name string) error {
		if _, err := tx.Exec(ctx, `UPDATE career_practices SET state='submitted',report_file=$2,feedback=NULL WHERE id=$1`, id, name); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
	if saved {
		if previous != "" && previous == filepath.Base(previous) && resourceUUID(strings.TrimSuffix(previous, filepath.Ext(previous))) {
			// The replacement is committed; a cleanup failure cannot roll it back.
			_ = os.Remove(filepath.Join(s.attachmentDir(), previous))
		}
		write(w, 204, nil)
	}
}
