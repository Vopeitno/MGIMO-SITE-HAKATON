package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"mgimo/backend/internal/config"
)

var staffMaterialRoutes = []studyRoute{
	{"GET", "/learning/materials", "STUDENT|TEACHER", (*Server).staffMaterials},
	{"POST", "/learning/materials", "TEACHER", (*Server).staffMaterialCreate},
	{"PUT", "/learning/materials/{id}", "TEACHER", (*Server).staffMaterialEdit},
	{"DELETE", "/learning/materials/{id}", "TEACHER", (*Server).staffMaterialDelete},
	{"GET", "/learning/materials/{id}/file", "STUDENT|TEACHER", (*Server).staffMaterialFile},
	{"POST", "/learning/materials/{id}/file", "TEACHER", (*Server).staffMaterialFile},
}

const staffMaterialAccess = `((m.teacher_id=$1 AND ` + staffTeachingGroup + `) OR
 (m.published AND EXISTS (SELECT 1 FROM students st WHERE st."studentID"=NULLIF($2,'')::uuid AND
  ((cg."groupID"=st."groupID" AND NOT EXISTS (SELECT 1 FROM student_stream_selections chosen
       WHERE chosen."studentID"=st."studentID" AND chosen."courseID"=cg."courseID")
    AND NOT EXISTS(SELECT 1 FROM course_streams sm WHERE sm."courseGroupID"=cg."courseGroupID"))
   OR EXISTS(SELECT 1 FROM student_stream_selections chosen JOIN course_streams sm USING ("streamID")
     WHERE chosen."studentID"=st."studentID" AND sm."courseGroupID"=cg."courseGroupID")))))`

func (s *Server) staffMaterials(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	course := r.URL.Query().Get("courseID")
	if course != "" && !resourceUUID(course) {
		fail(w, 400, "BAD_REQUEST", "Некорректная дисциплина")
		return
	}
	s.portalPage(w, r, `SELECT m.id,m.title,m.description,m.url,m.published,m.file_name IS NOT NULL AS "hasFile",
		m.created_at AS "createdAt",c.title AS course,c."courseID",g."groupNumber"
		FROM staff_course_materials m JOIN "courseGroup" cg ON cg."courseGroupID"=m.course_group_id
		JOIN courses c USING ("courseID") JOIN groups g USING ("groupID")
		WHERE `+staffMaterialAccess+` AND (NULLIF($3,'')::uuid IS NULL OR cg."courseID"=NULLIF($3,'')::uuid)`,
		`"createdAt" DESC,id`, emptyUUID(p.TeacherID), p.StudentID, course)
}

type staffMaterialText struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func staffMaterialValid(m *staffMaterialText) bool {
	m.URL = strings.TrimSpace(m.URL)
	return serviceText(&m.Title, 150) && serviceText(&m.Description, 3000) &&
		(m.URL == "" || (len(m.URL) <= 1000 && config.ValidSubmissionURL(m.URL)))
}

func (s *Server) staffMaterialCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		staffMaterialText
		GroupID     string `json:"courseGroupID"`
		WaitForFile bool   `json:"waitForFile"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !staffMaterialValid(&body.staffMaterialText) || !resourceUUID(body.GroupID) || (body.WaitForFile && body.URL != "") {
		fail(w, 422, "VALIDATION_ERROR", "Заполните название, описание и группу. Выберите ссылку HTTPS или файл.")
		return
	}
	id := uuid.NewString()
	p := current(r)
	tag, err := s.DB.Exec(r.Context(), `INSERT INTO staff_course_materials(id,course_group_id,teacher_id,title,description,url,published)
	SELECT $3,cg."courseGroupID",$1,$4,$5,NULLIF($6,''),NOT $7 FROM "courseGroup" cg
	WHERE cg."courseGroupID"=$2 AND `+staffTeachingGroup,
		p.TeacherID, body.GroupID, id, body.Title, body.Description, body.URL, body.WaitForFile)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Группа недоступна")
	} else {
		write(w, 201, map[string]string{"id": id})
	}
}

func (s *Server) staffMaterialEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	var body staffMaterialText
	if !decode(w, r, &body) {
		return
	}
	if !staffMaterialValid(&body) {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте название, описание и ссылку HTTPS")
		return
	}
	tag, err := s.DB.Exec(r.Context(), `UPDATE staff_course_materials m SET title=$3,description=$4,url=NULLIF($5,'')
		FROM "courseGroup" cg WHERE m.id=$2 AND m.teacher_id=$1 AND cg."courseGroupID"=m.course_group_id
		AND ($5='' OR (m.file_name IS NULL AND m.published)) AND `+staffTeachingGroup,
		current(r).TeacherID, id, body.Title, body.Description, body.URL)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Материал недоступен или к нему уже прикреплён файл")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) staffMaterialDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	var file string
	err := s.DB.QueryRow(r.Context(), `DELETE FROM staff_course_materials m
		USING "courseGroup" cg WHERE m.id=$2 AND m.teacher_id=$1 AND cg."courseGroupID"=m.course_group_id AND `+staffTeachingGroup+
		` RETURNING COALESCE(m.file_name,'')`, current(r).TeacherID, id).Scan(&file)
	if err != nil {
		resourceError(w, err)
		return
	}
	if file != "" && file == filepath.Base(file) && resourceUUID(strings.TrimSuffix(file, filepath.Ext(file))) {
		_ = os.Remove(filepath.Join(s.attachmentDir(), file))
	}
	write(w, 204, nil)
}

func (s *Server) staffMaterialFile(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	p := current(r)
	args := []any{emptyUUID(p.TeacherID), p.StudentID, id}
	query := `SELECT COALESCE(m.file_name,''),m.published FROM staff_course_materials m JOIN "courseGroup" cg
		ON cg."courseGroupID"=m.course_group_id WHERE m.id=$3 AND ` + staffMaterialAccess
	ctx := r.Context()
	var file string
	var published bool
	if r.Method == "GET" {
		if err := s.DB.QueryRow(ctx, query, args...).Scan(&file, &published); err != nil {
			resourceError(w, err)
			return
		}
		s.serveAttachment(w, r, file)
		return
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		resourceError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, query+` FOR UPDATE OF m`, args...).Scan(&file, &published); err != nil {
		resourceError(w, err)
		return
	}
	if published {
		fail(w, 409, "CONFLICT", "Материал уже опубликован")
		return
	}
	_, saved := s.saveAttachment(w, r, false, func(name string) error {
		if _, err = tx.Exec(ctx, `UPDATE staff_course_materials SET file_name=$2,published=true WHERE id=$1`, id, name); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
	if saved {
		write(w, 204, nil)
	}
}
