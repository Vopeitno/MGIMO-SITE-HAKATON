package api

import (
	"encoding/json"
	"net/http"

	"mgimo/backend/internal/config"
)

func (s *Server) deadlineLinkPatch(w http.ResponseWriter, r *http.Request) {
	p := resourceGuard(w, r, "TEACHER")
	if p == nil {
		return
	}
	id, err := academicInteger(r.PathValue("deadlineID"))
	if err != nil || id < 1 {
		fail(w, 400, "BAD_REQUEST", "Некорректный дедлайн")
		return
	}
	// RawMessage distinguishes an explicit null (reset) from a missing property.
	var body struct {
		URL json.RawMessage `json:"submissionURL"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.URL) == 0 {
		fail(w, 400, "BAD_REQUEST", "Требуется submissionURL")
		return
	}
	var link *string
	if err = json.Unmarshal(body.URL, &link); err != nil || (link != nil && !config.ValidSubmissionURL(*link)) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите полную HTTPS-ссылку без логина и пароля или null для сброса")
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		resourceError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// Lock the assignment's course and teaching assignment for the whole update,
	// preventing a concurrent reassignment/removal from bypassing course ownership.
	var courseID string
	err = tx.QueryRow(r.Context(), `SELECT c."courseID"::text FROM deadlines d JOIN courses c ON c."courseID"=d."courseID" JOIN "courseTeacher" ct ON ct."courseID"=c."courseID" WHERE d."deadlineID"=$1 AND ct."teacherID"=NULLIF($2,'')::uuid FOR UPDATE OF d,c,ct`, id, p.TeacherID).Scan(&courseID)
	if err != nil {
		resourceError(w, err)
		return
	}
	if link == nil {
		_, err = tx.Exec(r.Context(), `DELETE FROM deadline_links WHERE "deadlineID"=$1`, id)
	} else {
		_, err = tx.Exec(r.Context(), `INSERT INTO deadline_links("deadlineID","submissionURL","updatedByTeacherID","updatedAt") VALUES($1,$2,$3,now()) ON CONFLICT("deadlineID") DO UPDATE SET "submissionURL"=EXCLUDED."submissionURL","updatedByTeacherID"=EXCLUDED."updatedByTeacherID","updatedAt"=now()`, id, *link, p.TeacherID)
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		resourceError(w, err)
		return
	}
	moodle := s.Config.MoodleURL
	if moodle == "" {
		moodle = config.DefaultMoodleURL
	}
	effective := moodle
	custom := link
	if link != nil {
		effective = *link
	}
	write(w, 200, map[string]any{"deadlineID": id, "submissionURL": effective, "customSubmissionURL": custom})
}
