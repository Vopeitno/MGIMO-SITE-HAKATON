package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Projections deliberately enumerate public fields; never serialize a database row.
var resourceColumns = map[string]string{
	"students":            "r.\"studentID\", r.\"studentNumber\", r.\"studentName\", r.\"studentEmail\", r.\"studentPhoneNumber\", r.\"studentBirthday\" AT TIME ZONE 'Europe/Moscow' AS \"studentBirthday\", r.\"studentStatus\", r.\"studentEnrollmentDate\" AT TIME ZONE 'Europe/Moscow' AS \"studentEnrollmentDate\", r.\"studentGender\", r.\"groupID\", r.\"facultyID\", r.\"directionID\"",
	"teachers":            "r.\"teacherID\", r.\"teacherNumber\", r.\"teacherName\", r.\"teacherPosition\", r.\"teacherDegree\", r.\"teacherEmail\", r.\"teacherPhoneNumber\", r.\"teacherBirthday\" AT TIME ZONE 'Europe/Moscow' AS \"teacherBirthday\", r.\"teacherStatus\"",
	"employers":           "r.\"employerID\", r.\"employerNumber\", r.\"employerName\", r.\"employerPhoneNumber\", r.\"employerEmail\", r.\"departamentID\", r.\"employerStatus\", r.\"hireAt\", r.\"fireAt\"",
	"courses":             "r.\"courseID\", r.\"title\", r.\"description\", r.\"hours\", r.\"semester\", r.\"facultyID\"",
	"news":                "r.\"newsID\", r.\"deaneryID\", r.\"employerID\", r.\"newsTitle\", r.\"newsDescription\", r.\"createdAt\", r.\"updatedAt\"",
	"buildings":           "r.\"buildingID\", r.\"building\", r.\"buildingAddress\"",
	"sports":              "r.\"sportID\", r.\"sportTitle\", r.\"sportDescription\", r.\"buildingID\", r.\"sportCapacity\", r.\"sportStartAt\", r.\"sportEndAt\"",
	"sport_registrations": "r.\"sportRegistrationID\", r.\"sportID\", r.\"studentID\", r.\"registrationStatus\", r.\"createdAt\", r.\"updatedAt\"",
	"accounts":            "r.\"accountID\", r.\"email\", r.\"accountRole\", r.\"studentID\", r.\"teacherID\", r.\"employerID\", r.\"isActive\", r.\"createdAt\"",
	"notifications":       "r.\"notificationID\", r.\"notificationMessage\", r.\"studentID\", r.\"employerID\", r.\"teacherID\", r.\"notificationTitle\", r.\"notificationCategory\", r.\"notificationCreatedAt\", r.\"notificationReadAt\", r.\"lessonID\", r.\"appoinmentID\", r.\"courseID\", r.\"examID\", r.\"sportID\", r.\"newsID\"",
	"lessons":             "r.\"lessonID\", r.\"groupID\", r.\"classroomID\", r.\"teacherID\", r.\"topic\", r.\"startAt\", r.\"endAt\", r.\"remoteLessonLink\", r.\"lessonStatus\", r.\"courseID\"",
	"courseScore":         "r.\"courseGraduate\", r.\"courseID\", r.\"studentID\", r.\"teacherID\", r.\"score\", r.\"credit\", r.\"created_at\", r.\"updated_at\"",
	"deadlines":           "r.\"deadlineID\", r.\"courseID\", r.\"deadlineTitle\", r.\"deadlineDescription\", r.\"deadlineStartAt\", r.\"deadlineEndAt\"",
	"appointment":         "r.\"appoinmentID\", r.\"studentID\", r.\"employerID\", r.\"buildingID\", r.\"appointmentTitle\", r.\"appointmentDescription\", r.\"fileUrlAttachment\", r.\"appointmentStatus\", r.\"created_at\", r.\"updated_at\", r.\"teacherID\", r.\"createdByEmployerID\", r.\"subjectStudentID\", r.\"appointmentTypeID\"",
	"exams":               "r.\"examID\", r.\"courseID\", r.\"groupID\", r.\"teacherID\", r.\"classroomID\", r.\"examTitle\", r.\"examStartAt\", r.\"examEndAt\", r.\"examStatus\"",
}

const resourceIdentity = `WITH resource_identity AS (SELECT $1::text, $2::text, $3::text, $4::text) `
const resourceFacultyScope = `EXISTS (SELECT 1 FROM faculties f JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID" WHERE f."facultyID"=r."facultyID" AND ds."employerID"=NULLIF($4,'')::uuid)`
const resourceCourseScope = `($1='SUPPORT' OR ($1='DEAN' AND ` + resourceFacultyScope + `) OR ($1='STUDENT' AND EXISTS (SELECT 1 FROM student_course_access a WHERE a."courseID"=r."courseID" AND a."studentID"=NULLIF($2,'')::uuid)) OR ($1='TEACHER' AND EXISTS (SELECT 1 FROM "courseTeacher" ct WHERE ct."courseID"=r."courseID" AND ct."teacherID"=NULLIF($3,'')::uuid)))`
const resourceRecipient = `(($1='STUDENT' AND r."studentID"=NULLIF($2,'')::uuid) OR ($1='TEACHER' AND r."teacherID"=NULLIF($3,'')::uuid) OR ($1 IN ('SUPPORT','DEAN','ADMIN') AND r."employerID"=NULLIF($4,'')::uuid))`
const resourceNewsScope = `($1='SUPPORT' OR ($1='DEAN' AND EXISTS (SELECT 1 FROM deanery_staff ds WHERE ds."deaneryID"=r."deaneryID" AND ds."employerID"=NULLIF($4,'')::uuid)))`

func resourceArgs(p *Principal) []any { return []any{p.Role, p.StudentID, p.TeacherID, p.EmployerID} }

func resourceUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
func resourceNewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
func resourceAllowed(p *Principal, roles string) bool {
	return p != nil && strings.Contains("|"+roles+"|", "|"+p.Role+"|") && p.Role != ""
}
func resourceGuard(w http.ResponseWriter, r *http.Request, roles string) *Principal {
	p := current(r)
	if p == nil {
		fail(w, 401, "UNAUTHORIZED", "Не авторизован")
		return nil
	}
	if !resourceAllowed(p, roles) {
		fail(w, 403, "FORBIDDEN", "Нет прав на объект")
		return nil
	}
	return p
}
func resourceError(w http.ResponseWriter, err error) {
	var pe *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		fail(w, 404, "NOT_FOUND", "Объект не найден")
	case errors.As(err, &pe):
		switch pe.Code {
		case "23505", "23P01", "40001", "40P01":
			fail(w, 409, "CONFLICT", "Конфликт состояния")
		case "23503", "23514", "23502", "22001", "22P02", "22003":
			fail(w, 422, "VALIDATION_ERROR", "Ошибка проверки данных")
		default:
			fail(w, 500, "INTERNAL_ERROR", "Внутренняя ошибка")
		}
	default:
		fail(w, 500, "INTERNAL_ERROR", "Внутренняя ошибка")
	}
}
func resourcePagination(r *http.Request) (int, int, error) {
	limit, offset := 20, 0
	var err error
	for key, dst := range map[string]*int{"limit": &limit, "offset": &offset} {
		if raw, ok := r.URL.Query()[key]; ok {
			if len(raw) != 1 || raw[0] == "" {
				return 0, 0, fmt.Errorf("pagination")
			}
			*dst, err = strconv.Atoi(raw[0])
			if err != nil {
				return 0, 0, err
			}
		}
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return 0, 0, fmt.Errorf("pagination")
	}
	return limit, offset, nil
}
func resourceProjection(table string) string {
	cols := resourceColumns[table]
	if table == "effective_course_scores" {
		cols = resourceColumns["courseScore"]
	}
	if table == "news" {
		cols += `, (SELECT nc.category FROM news_categories nc WHERE nc."newsID"=r."newsID") AS "newsCategory"`
	}

	if table == "courses" {
		cols += `,COALESCE((SELECT jsonb_agg(jsonb_build_object('teacherID',t."teacherID",'teacherName',t."teacherName") ORDER BY t."teacherName",t."teacherID") FROM "courseTeacher" ct JOIN teachers t ON t."teacherID"=ct."teacherID" WHERE ct."courseID"=r."courseID"),'[]'::jsonb) AS "teachers"`
	}
	if table == "students" {
		cols += `,(SELECT to_jsonb(g) FROM groups g WHERE g."groupID"=r."groupID") AS "group",(SELECT to_jsonb(f) FROM faculties f WHERE f."facultyID"=r."facultyID") AS "faculty",(SELECT to_jsonb(d) FROM direction d WHERE d."directionID"=r."directionID") AS "direction"`
	}
	if table == "lessons" {
		cols += `,(SELECT jsonb_build_object('teacherID',t."teacherID",'teacherName',t."teacherName") FROM teachers t WHERE t."teacherID"=r."teacherID") AS "teacher",(SELECT to_jsonb(cr) FROM classroom cr WHERE cr."classroomID"=r."classroomID") AS "classroom",(SELECT to_jsonb(b) FROM buildings b JOIN classroom cr ON cr."buildingID"=b."buildingID" WHERE cr."classroomID"=r."classroomID") AS "building",(SELECT to_jsonb(c) FROM courses c WHERE c."courseID"=r."courseID") AS "course"`
	}
	if table == "notifications" {
		cols += `, CASE WHEN r."newsID" IS NOT NULL THEN '/news/'||r."newsID"::text WHEN r."lessonID" IS NOT NULL THEN '/schedule' WHEN r."appoinmentID" IS NOT NULL THEN '/appointments/'||r."appoinmentID"::text WHEN r."courseID" IS NOT NULL THEN '/courses/'||r."courseID"::text WHEN r."examID" IS NOT NULL THEN '/exams/'||r."examID"::text WHEN r."sportID" IS NOT NULL THEN '/sports/'||r."sportID"::text ELSE NULL END AS "targetPath"`
	}
	if table == "sports" {
		cols += `, (SELECT count(*) FROM sport_registrations sr WHERE sr."sportID"=r."sportID" AND sr."registrationStatus"='enrolled') AS "enrolledCount"`
	}
	return cols
}

type resourceQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func resourceObject(ctx context.Context, q resourceQuerier, table, where string, args ...any) (json.RawMessage, error) {
	var result json.RawMessage
	err := q.QueryRow(ctx, resourceIdentity+`SELECT row_to_json(resource_row) FROM (SELECT `+resourceProjection(table)+` FROM "`+table+`" r WHERE `+where+`) resource_row`, args...).Scan(&result)
	return result, err
}
func resourcePage(ctx context.Context, q resourceQuerier, table, where, order string, limit, offset int, args ...any) (json.RawMessage, error) {
	// Count and page share one statement snapshot, including empty/out-of-range pages.
	n := len(args)
	args = append(args, limit, offset)
	sql := resourceIdentity + `, resource_filtered AS (SELECT ` + resourceProjection(table) + ` FROM "` + table + `" r WHERE ` + where + `), resource_page AS (SELECT * FROM resource_filtered ORDER BY ` + order + fmt.Sprintf(` LIMIT $%d OFFSET $%d) SELECT json_build_object('items',COALESCE((SELECT json_agg(resource_page) FROM resource_page),'[]'::json),'total',(SELECT count(*) FROM resource_filtered),'limit',$%d::int,'offset',$%d::int)`, n+1, n+2, n+1, n+2)
	var result json.RawMessage
	err := q.QueryRow(ctx, sql, args...).Scan(&result)
	return result, err
}
func (s *Server) resourceList(w http.ResponseWriter, r *http.Request, table, where, order string, p *Principal) {
	limit, offset, err := resourcePagination(r)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная пагинация")
		return
	}
	args := resourceArgs(p)
	if table == "notifications" {
		if cat := r.URL.Query().Get("notificationCategory"); cat != "" {
			if !strings.Contains("|news|announcement|schedule_change|deadline|request|exam|sport|", "|"+cat+"|") {
				fail(w, 400, "BAD_REQUEST", "Некорректная категория")
				return
			}
			where += ` AND r."notificationCategory"=$5::notification_category`
			args = append(args, cat)
		}
	}
	result, err := resourcePage(r.Context(), s.DB, table, where, order, limit, offset, args...)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, result)
}
func (s *Server) resourceGet(w http.ResponseWriter, r *http.Request, table, id, scope string, p *Principal) {
	value := r.PathValue(id)
	if table == "news" {
		if _, err := strconv.ParseInt(value, 10, 32); err != nil {
			fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
			return
		}
	} else if !resourceUUID(value) {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
		return
	}
	args := append(resourceArgs(p), value)
	cast := "uuid"
	if table == "news" {
		cast = "int"
	}
	result, err := resourceObject(r.Context(), s.DB, table, `r."`+id+`"=$5::`+cast+` AND (`+scope+`)`, args...)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, result)
}
func (s *Server) registerResources(mux *http.ServeMux) {
	const all = "STUDENT|TEACHER|SUPPORT|DEAN|ADMIN"
	const ordinary = "STUDENT|TEACHER|SUPPORT|DEAN"
	add := func(method, path, roles string, h func(http.ResponseWriter, *http.Request, *Principal)) {
		mux.HandleFunc(method+" /api/v1/MGIMO"+path, func(w http.ResponseWriter, r *http.Request) {
			if p := resourceGuard(w, r, roles); p != nil {
				h(w, r, p)
			}
		})
	}
	add("GET", "/profile/me", all, s.resourceProfile)
	add("GET", "/profile/me/photo", "STUDENT", s.profilePhoto)
	add("PUT", "/profile/me/photo", "STUDENT", s.updateProfilePhoto)
	add("GET", "/profile/me/qr", "STUDENT", s.profileQR)
	add("GET", "/notifications", all, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "notifications", resourceRecipient+studentNotificationVisibility, `"notificationCreatedAt" DESC, "notificationID"`, p)
	})
	add("GET", "/notifications/{notificationID}", all, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceGet(w, r, "notifications", "notificationID", resourceRecipient, p)
	})
	add("PATCH", "/notifications/{notificationID}/read", all, s.resourceRead)
	add("GET", "/courses", ordinary, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "courses", resourceCourseScope, `"title", "courseID"`, p)
	})
	add("GET", "/courses/{courseID}", ordinary, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceGet(w, r, "courses", "courseID", resourceCourseScope, p)
	})
	add("POST", "/courses", "DEAN|SUPPORT", s.resourceCourseWrite)
	add("PATCH", "/courses/{courseID}", "DEAN|SUPPORT", s.resourceCourseWrite)
	add("GET", "/news", all, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "news", "true", `"createdAt" DESC NULLS LAST, "newsID" DESC`, p)
	})
	add("GET", "/news/{newsID}", all, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceGet(w, r, "news", "newsID", "true", p)
	})
	add("POST", "/news", "DEAN|SUPPORT", s.resourceNewsWrite)
	add("PATCH", "/news/{newsID}", "DEAN|SUPPORT", s.resourceNewsWrite)
	add("GET", "/buildings", ordinary, func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "buildings", "true", `"building", "buildingID"`, p)
	})
	add("GET", "/faculty/students", "DEAN", func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "students", resourceFacultyScope, `"studentName", "studentID"`, p)
	})
	add("GET", "/faculty/teachers", "DEAN", func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "teachers", `EXISTS (SELECT 1 FROM "facultyTeacher" ft JOIN faculties f ON f."facultyID"=ft."facultyID" JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID" WHERE ft."teacherID"=r."teacherID" AND ds."employerID"=NULLIF($4,'')::uuid)`, `"teacherName", "teacherID"`, p)
	})
	add("GET", "/sports", "STUDENT", func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "sports", "true", `"sportStartAt", "sportID"`, p)
	})
	add("GET", "/sports/{sportID}", "STUDENT", func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceGet(w, r, "sports", "sportID", "true", p)
	})
	add("GET", "/sports/registrations/me", "STUDENT", func(w http.ResponseWriter, r *http.Request, p *Principal) {
		s.resourceList(w, r, "sport_registrations", `r."studentID"=NULLIF($2,'')::uuid`, `"createdAt" DESC, "sportRegistrationID"`, p)
	})
	add("POST", "/sports/{sportID}/registrations", "STUDENT", s.resourceSportWrite)
	add("DELETE", "/sports/{sportID}/registrations/me", "STUDENT", s.resourceSportWrite)
	add("GET", "/dashboard", ordinary, s.resourceDashboard)
	add("POST", "/admin/accounts", "ADMIN", s.resourceAccountWrite)
	add("PATCH", "/admin/accounts/{accountID}", "ADMIN", s.resourceAccountWrite)
}

func (s *Server) resourceProfile(w http.ResponseWriter, r *http.Request, p *Principal) {
	table, key, id, field := "employers", "employerID", p.EmployerID, "employer"
	if p.Role == "STUDENT" {
		table, key, id, field = "students", "studentID", p.StudentID, "student"
	}
	if p.Role == "TEACHER" {
		table, key, id, field = "teachers", "teacherID", p.TeacherID, "teacher"
	}
	args := append(resourceArgs(p), id)
	profile, err := resourceObject(r.Context(), s.DB, table, `r."`+key+`"=NULLIF($5,'')::uuid`, args...)
	if err != nil {
		resourceError(w, err)
		return
	}
	result := map[string]any{"role": p.Role, field: profile, "deaneries": json.RawMessage(`[]`)}
	if p.Role == "STUDENT" {
		var number *string
		err = s.DB.QueryRow(r.Context(), `SELECT "recordBookNumber" FROM student_record_books WHERE "studentID"=$1::uuid`, id).Scan(&number)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			resourceError(w, err)
			return
		}
		result["recordBookNumber"] = number
	}

	if p.Role == "DEAN" {
		var ds json.RawMessage
		err = s.DB.QueryRow(r.Context(), `SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT d."deaneryID",d.name FROM deaneries d JOIN deanery_staff s ON s."deaneryID"=d."deaneryID" WHERE s."employerID"=$1::uuid ORDER BY d."deaneryID") x`, p.EmployerID).Scan(&ds)
		if err != nil {
			resourceError(w, err)
			return
		}
		result["deaneries"] = ds
	}
	write(w, 200, result)
}
func (s *Server) resourceRead(w http.ResponseWriter, r *http.Request, p *Principal) {
	id := r.PathValue("notificationID")
	if !resourceUUID(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
		return
	}
	args := append(resourceArgs(p), id)
	// Atomic recipient-filtered update; COALESCE preserves the first read timestamp.
	tag, err := s.DB.Exec(r.Context(), resourceIdentity+`UPDATE notifications r SET "notificationReadAt"=COALESCE(r."notificationReadAt",now()) WHERE r."notificationID"=$5::uuid AND `+resourceRecipient, args...)
	if err != nil {
		resourceError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		resourceError(w, pgx.ErrNoRows)
		return
	}
	s.resourceGet(w, r, "notifications", "notificationID", resourceRecipient, p)
}

// Presence is retained separately from values, so nullable PATCH fields can be cleared.
func resourceBody(w http.ResponseWriter, r *http.Request, allowed, required []string) (map[string]json.RawMessage, bool) {
	var body map[string]json.RawMessage
	if !decode(w, r, &body) {
		return nil, false
	}
	if len(body) == 0 {
		fail(w, 422, "VALIDATION_ERROR", "Пустой объект")
		return nil, false
	}
	for k := range body {
		found := false
		for _, a := range allowed {
			if a == k {
				found = true
				break
			}
		}
		if !found {
			fail(w, 422, "VALIDATION_ERROR", "Недопустимое поле")
			return nil, false
		}
	}
	for _, k := range required {
		if _, ok := body[k]; !ok {
			fail(w, 422, "VALIDATION_ERROR", "Отсутствует обязательное поле")
			return nil, false
		}
	}
	return body, true
}
func resourceValue(raw json.RawMessage, kind string, max int, nullable bool) (any, bool) {
	if string(raw) == "null" {
		return nil, nullable
	}
	switch kind {
	case "string", "uuid", "email":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return nil, false
		}
		if max > 0 && utf8.RuneCountInString(v) > max {
			return nil, false
		}
		if kind == "uuid" && !resourceUUID(v) {
			return nil, false
		}
		if kind == "email" {
			a, e := mail.ParseAddress(v)
			if e != nil || a.Address != v {
				return nil, false
			}
		}
		return v, true
	case "int":
		var v int
		if json.Unmarshal(raw, &v) != nil || v < max {
			return nil, false
		}
		return v, true
	case "bool":
		var v bool
		if json.Unmarshal(raw, &v) != nil {
			return nil, false
		}
		return v, true
	}
	return nil, false
}

type resourceField struct {
	name, kind string
	bound      int
	nullable   bool
}

func resourceValues(w http.ResponseWriter, body map[string]json.RawMessage, fields []resourceField) ([]string, []any, bool) {
	names := []string{}
	values := []any{}
	for _, f := range fields {
		if raw, ok := body[f.name]; ok {
			v, valid := resourceValue(raw, f.kind, f.bound, f.nullable)
			if !valid {
				fail(w, 422, "VALIDATION_ERROR", "Некорректное поле: "+f.name)
				return nil, nil, false
			}
			names = append(names, f.name)
			values = append(values, v)
		}
	}
	return names, values, true
}
func resourceRollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }

func (s *Server) resourceCourseWrite(w http.ResponseWriter, r *http.Request, p *Principal) {
	create := r.Method == "POST"
	fields := []resourceField{{"facultyID", "uuid", 0, false}, {"title", "string", 128, false}, {"description", "string", 564, true}, {"hours", "int", 0, false}, {"semester", "int", 1, false}}
	required := []string{}
	if create {
		required = []string{"facultyID", "title", "hours", "semester"}
	}
	body, ok := resourceBody(w, r, []string{"facultyID", "title", "description", "hours", "semester"}, required)
	if !ok {
		return
	}
	names, values, ok := resourceValues(w, body, fields)
	if !ok {
		return
	}
	id := r.PathValue("courseID")
	var err error
	if create {
		id, err = resourceNewID()
	} else if !resourceUUID(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
		return
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		resourceError(w, err)
		return
	}
	defer resourceRollback(tx)
	if !create {
		var locked string
		err = tx.QueryRow(r.Context(), resourceIdentity+`SELECT r."courseID"::text FROM courses r WHERE r."courseID"=$5::uuid AND `+resourceCourseScope+` FOR UPDATE`, append(resourceArgs(p), id)...).Scan(&locked)
		if err != nil {
			resourceError(w, err)
			return
		}
	}
	if raw, exists := body["facultyID"]; exists {
		var faculty string
		_ = json.Unmarshal(raw, &faculty)
		var allowed bool
		err = tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM faculties f WHERE f."facultyID"=$1::uuid AND ($2::text='SUPPORT' OR EXISTS (SELECT 1 FROM deanery_staff ds WHERE ds."deaneryID"=f."deaneryID" AND ds."employerID"=NULLIF($3::text,'')::uuid)))`, faculty, p.Role, p.EmployerID).Scan(&allowed)
		if err != nil {
			resourceError(w, err)
			return
		}
		if !allowed {
			fail(w, 403, "FORBIDDEN", "Нет прав на факультет")
			return
		}
	}
	sql := ""
	args := []any{id}
	if create {
		cols := []string{`"courseID"`}
		slots := []string{"$1::uuid"}
		for i, n := range names {
			cols = append(cols, `"`+n+`"`)
			slots = append(slots, fmt.Sprintf("$%d", i+2))
		}
		sql = `INSERT INTO courses (` + strings.Join(cols, ",") + `) VALUES (` + strings.Join(slots, ",") + `)`
	} else {
		sets := []string{}
		for i, n := range names {
			sets = append(sets, fmt.Sprintf(`"%s"=$%d`, n, i+2))
		}
		sql = `UPDATE courses SET ` + strings.Join(sets, ",") + ` WHERE "courseID"=$1::uuid`
	}
	args = append(args, values...)
	_, err = tx.Exec(r.Context(), sql, args...)
	if err != nil {
		resourceError(w, err)
		return
	}
	result, err := resourceObject(r.Context(), tx, "courses", `r."courseID"=$5::uuid`, append(resourceArgs(p), id)...)
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		resourceError(w, err)
		return
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, result)
}

func (s *Server) resourceNewsWrite(w http.ResponseWriter, r *http.Request, p *Principal) {
	create := r.Method == "POST"
	allowed := []string{"newsTitle", "newsDescription", "newsCategory"}
	required := []string{}
	if create {
		allowed = append(allowed, "deaneryID")
		required = []string{"newsTitle", "newsDescription"}
	}
	body, ok := resourceBody(w, r, allowed, required)
	if !ok {
		return
	}
	var category *string
	rawCategory, categoryProvided := body["newsCategory"]
	if categoryProvided {
		if err := json.Unmarshal(rawCategory, &category); err != nil ||
			(category != nil && *category != "important" && *category != "opportunities" && *category != "campus" && *category != "events") {
			fail(w, 422, "VALIDATION_ERROR", "Выберите известную категорию новости")
			return
		}
		delete(body, "newsCategory")
	}

	names, values, ok := resourceValues(w, body, []resourceField{{"newsTitle", "string", 128, false}, {"newsDescription", "string", 0, false}, {"deaneryID", "int", -2147483648, true}})
	if !ok {
		return
	}
	if !resourceUUID(p.EmployerID) {
		fail(w, 403, "FORBIDDEN", "Не привязан сотрудник")
		return
	}
	var id int
	var err error
	if !create {
		id, err = strconv.Atoi(r.PathValue("newsID"))
		if err != nil {
			fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
			return
		}
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		resourceError(w, err)
		return
	}
	defer resourceRollback(tx)
	if create {
		if p.Role == "DEAN" {
			raw, exists := body["deaneryID"]
			if !exists || string(raw) == "null" {
				fail(w, 422, "VALIDATION_ERROR", "Для деканата обязателен deaneryID")
				return
			}
			var deanery int
			_ = json.Unmarshal(raw, &deanery)
			var member bool
			err = tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM deanery_staff WHERE "employerID"=$1::uuid AND "deaneryID"=$2)`, p.EmployerID, deanery).Scan(&member)
			if err != nil {
				resourceError(w, err)
				return
			}
			if !member {
				fail(w, 403, "FORBIDDEN", "Нет прав на деканат")
				return
			}
		}
		// The documented schema has no identity/sequence for newsID. Serialize allocation.
		if _, err = tx.Exec(r.Context(), `LOCK TABLE news IN EXCLUSIVE MODE`); err != nil {
			resourceError(w, err)
			return
		}
		err = tx.QueryRow(r.Context(), `SELECT COALESCE(max("newsID"),0)+1 FROM news`).Scan(&id)
		if err != nil {
			resourceError(w, err)
			return
		}
		cols := []string{`"newsID"`, `"employerID"`, `"createdAt"`}
		slots := []string{"$1", "$2::uuid", "now()"}
		args := []any{id, p.EmployerID}
		for i, n := range names {
			cols = append(cols, `"`+n+`"`)
			slots = append(slots, fmt.Sprintf("$%d", i+3))
		}
		args = append(args, values...)
		_, err = tx.Exec(r.Context(), `INSERT INTO news (`+strings.Join(cols, ",")+`) VALUES (`+strings.Join(slots, ",")+`)`, args...)
	} else {
		var locked int
		err = tx.QueryRow(r.Context(), resourceIdentity+`SELECT r."newsID" FROM news r WHERE r."newsID"=$5::int AND `+resourceNewsScope+` FOR UPDATE`, append(resourceArgs(p), id)...).Scan(&locked)
		if err != nil {
			resourceError(w, err)
			return
		}
		sets := []string{`"updatedAt"=now()`}
		args := []any{id}
		for i, n := range names {
			sets = append(sets, fmt.Sprintf(`"%s"=$%d`, n, i+2))
		}
		args = append(args, values...)
		_, err = tx.Exec(r.Context(), `UPDATE news SET `+strings.Join(sets, ",")+` WHERE "newsID"=$1`, args...)
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	if categoryProvided {
		if category == nil {
			_, err = tx.Exec(r.Context(), `DELETE FROM news_categories WHERE "newsID"=$1`, id)
		} else {
			_, err = tx.Exec(r.Context(), `INSERT INTO news_categories ("newsID",category) VALUES($1,$2)
               ON CONFLICT ("newsID") DO UPDATE SET category=excluded.category`, id, *category)
		}
		if err != nil {
			resourceError(w, err)
			return
		}
	}

	result, err := resourceObject(r.Context(), tx, "news", `r."newsID"=$5::int`, append(resourceArgs(p), id)...)
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		resourceError(w, err)
		return
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, result)
}

func (s *Server) resourceSportWrite(w http.ResponseWriter, r *http.Request, p *Principal) {
	id := r.PathValue("sportID")
	if !resourceUUID(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
		return
	}
	if !resourceUUID(p.StudentID) {
		fail(w, 403, "FORBIDDEN", "Не привязан студент")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		resourceError(w, err)
		return
	}
	defer resourceRollback(tx)
	var studentLock int
	if err = tx.QueryRow(r.Context(), `SELECT 1 FROM students WHERE "studentID"=$1::uuid FOR UPDATE`, p.StudentID).Scan(&studentLock); err != nil {
		resourceError(w, err)
		return
	}

	// All enrollment/cancellation writers lock the parent first. The subsequent count
	// uses a fresh READ COMMITTED snapshot after a competing writer has committed.
	var capacity int
	var startAt time.Time
	err = tx.QueryRow(r.Context(), `SELECT "sportCapacity","sportStartAt" FROM sports WHERE "sportID"=$1::uuid FOR UPDATE`, id).Scan(&capacity, &startAt)
	if err != nil {
		resourceError(w, err)
		return
	}
	var registration, status string
	err = tx.QueryRow(r.Context(), `SELECT "sportRegistrationID"::text,"registrationStatus"::text FROM sport_registrations WHERE "sportID"=$1::uuid AND "studentID"=$2::uuid FOR UPDATE`, id, p.StudentID).Scan(&registration, &status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		resourceError(w, err)
		return
	}
	exists := err == nil
	if r.Method == "DELETE" {
		if !exists {
			resourceError(w, pgx.ErrNoRows)
			return
		}
		_, err = tx.Exec(r.Context(), `UPDATE sport_registrations SET "registrationStatus"='canceled',"updatedAt"=CASE WHEN "registrationStatus"='canceled' THEN "updatedAt" ELSE now() END WHERE "sportRegistrationID"=$1::uuid`, registration)
		if err != nil {
			resourceError(w, err)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			resourceError(w, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	if exists && status == "enrolled" {
		fail(w, 409, "CONFLICT", "Вы уже записаны")
		return
	}
	var alreadyUsed int
	err = tx.QueryRow(r.Context(), sportWeekLoad+` AND sp."sportID"<>$3::uuid ) used`, p.StudentID, sportWeek(startAt), id).Scan(&alreadyUsed)
	if err != nil {
		resourceError(w, err)
		return
	}
	if alreadyUsed >= 2 {
		fail(w, 409, "CONFLICT", "На одной неделе можно выбрать не больше двух занятий")
		return
	}

	var count int
	err = tx.QueryRow(r.Context(), `SELECT count(*) FROM sport_registrations WHERE "sportID"=$1::uuid AND "registrationStatus"='enrolled'`, id).Scan(&count)
	if err != nil {
		resourceError(w, err)
		return
	}
	if count >= capacity {
		fail(w, 409, "CONFLICT", "Свободных мест нет")
		return
	}
	responseStatus := 200
	if exists {
		_, err = tx.Exec(r.Context(), `UPDATE sport_registrations SET "registrationStatus"='enrolled',"updatedAt"=now() WHERE "sportRegistrationID"=$1::uuid`, registration)
	} else {
		registration, err = resourceNewID()
		if err != nil {
			resourceError(w, err)
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO sport_registrations ("sportRegistrationID","sportID","studentID","registrationStatus","createdAt") VALUES ($1::uuid,$2::uuid,$3::uuid,'enrolled',now())`, registration, id, p.StudentID)
		responseStatus = 201
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	result, err := resourceObject(r.Context(), tx, "sport_registrations", `r."sportRegistrationID"=$5::uuid AND r."studentID"=NULLIF($2,'')::uuid`, append(resourceArgs(p), registration)...)
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		resourceError(w, err)
		return
	}
	write(w, responseStatus, result)
}

func resourceAccountCompatible(role string, student, teacher, employer any) bool {
	switch role {
	case "STUDENT":
		return student != nil && teacher == nil && employer == nil
	case "TEACHER":
		return student == nil && teacher != nil && employer == nil
	case "SUPPORT", "DEAN", "ADMIN":
		return student == nil && teacher == nil && employer != nil
	}
	return false
}
func (s *Server) resourceAccountWrite(w http.ResponseWriter, r *http.Request, p *Principal) {
	create := r.Method == "POST"
	allowed := []string{"accountRole", "isActive"}
	required := []string{}
	fields := []resourceField{{"accountRole", "string", 0, false}, {"isActive", "bool", 0, false}}
	if create {
		allowed = []string{"email", "password", "accountRole", "studentID", "teacherID", "employerID"}
		required = []string{"email", "password", "accountRole"}
		fields = []resourceField{{"email", "email", 258, false}, {"password", "string", 0, false}, {"accountRole", "string", 0, false}, {"studentID", "uuid", 0, true}, {"teacherID", "uuid", 0, true}, {"employerID", "uuid", 0, true}}
	}
	body, ok := resourceBody(w, r, allowed, required)
	if !ok {
		return
	}
	names, values, ok := resourceValues(w, body, fields)
	if !ok {
		return
	}
	data := map[string]any{}
	for i, n := range names {
		data[n] = values[i]
	}
	id := r.PathValue("accountID")
	var err error
	if create {
		id, err = resourceNewID()
	} else if !resourceUUID(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
		return
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	var passwordHash string
	if create {
		password := data["password"].(string)
		if utf8.RuneCountInString(password) < 12 {
			fail(w, 422, "VALIDATION_ERROR", "Пароль короче 12 символов")
			return
		}
		if !resourceAccountCompatible(data["accountRole"].(string), data["studentID"], data["teacherID"], data["employerID"]) {
			fail(w, 422, "VALIDATION_ERROR", "Роль несовместима с персоной")
			return
		}
		passwordHash, err = hashPassword(password)
		if err != nil {
			resourceError(w, err)
			return
		}
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		resourceError(w, err)
		return
	}
	defer resourceRollback(tx)
	var auditRole *string
	var auditActive *bool
	if create {
		// Foreign keys verify existence; the database's CHECK and unique constraints
		// also protect this invariant against concurrent account creation.
		_, err = tx.Exec(r.Context(), `INSERT INTO accounts ("accountID","email","passwordHash","accountRole","studentID","teacherID","employerID") VALUES ($1::uuid,$2,$3,$4::account_role,$5::uuid,$6::uuid,$7::uuid)`, id, data["email"], passwordHash, data["accountRole"], data["studentID"], data["teacherID"], data["employerID"])
	} else {
		var oldRole string
		var oldActive bool
		var student, teacher, employer *string
		err = tx.QueryRow(r.Context(), `SELECT "accountRole"::text,"studentID"::text,"teacherID"::text,"employerID"::text,"isActive" FROM accounts WHERE "accountID"=$1::uuid FOR UPDATE`, id).Scan(&oldRole, &student, &teacher, &employer, &oldActive)
		if err != nil {
			resourceError(w, err)
			return
		}
		auditRole, auditActive = &oldRole, &oldActive
		role := oldRole
		if v, ok := data["accountRole"]; ok {
			role = v.(string)
		}
		var st, te, em any
		if student != nil {
			st = *student
		}
		if teacher != nil {
			te = *teacher
		}
		if employer != nil {
			em = *employer
		}
		if !resourceAccountCompatible(role, st, te, em) {
			fail(w, 422, "VALIDATION_ERROR", "Роль несовместима с персоной")
			return
		}
		_, err = tx.Exec(r.Context(), `UPDATE accounts SET "accountRole"=$2::account_role,"isActive"=COALESCE($3::boolean,"isActive") WHERE "accountID"=$1::uuid`, id, role, data["isActive"])
		if err == nil && (data["isActive"] == false || role != oldRole) {
			_, err = tx.Exec(r.Context(), `UPDATE user_sessions SET "revokedAt"=COALESCE("revokedAt",now()) WHERE "accountID"=$1::uuid AND "revokedAt" IS NULL`, id)
		}
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	action := "updated"
	if create {
		action = "created"
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO staff_account_audit(actor_id,target_id,target_email,action,previous_role,next_role,previous_active,next_active)
		SELECT $1,a."accountID",a.email,$3,$4,a."accountRole"::text,$5,a."isActive"
		FROM accounts a WHERE a."accountID"=$2 AND (a."accountRole"::text IS DISTINCT FROM $4 OR a."isActive" IS DISTINCT FROM $5)`,
		p.AccountID, id, action, auditRole, auditActive)
	if err != nil {
		resourceError(w, err)
		return
	}
	result, err := resourceObject(r.Context(), tx, "accounts", `r."accountID"=$5::uuid`, append(resourceArgs(p), id)...)
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		resourceError(w, err)
		return
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, result)
}

func (s *Server) resourceDashboard(w http.ResponseWriter, r *http.Request, p *Principal) {
	// One repeatable-read transaction gives the dashboard a consistent snapshot.
	tx, err := s.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		resourceError(w, err)
		return
	}
	defer resourceRollback(tx)
	result := map[string]any{"user": map[string]any{"accountID": p.AccountID, "role": p.Role, "email": p.Email, "displayName": p.DisplayName}}
	// No field grants access beyond the corresponding role's object scope.
	lessonScope := `(($1='STUDENT' AND EXISTS (SELECT 1 FROM student_lesson_access a WHERE a."studentID"=NULLIF($2,'')::uuid AND a."lessonID"=r."lessonID")) OR ($1='TEACHER' AND r."teacherID"=NULLIF($3,'')::uuid) OR ($1='DEAN' AND EXISTS (SELECT 1 FROM courses c JOIN faculties f ON f."facultyID"=c."facultyID" JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID" WHERE c."courseID"=r."courseID" AND ds."employerID"=NULLIF($4,'')::uuid)) OR $1='SUPPORT')`
	examScope := `(($1='STUDENT' AND EXISTS (SELECT 1 FROM student_exam_access a WHERE a."studentID"=NULLIF($2,'')::uuid AND a."examID"=r."examID")) OR ($1='TEACHER' AND r."teacherID"=NULLIF($3,'')::uuid) OR ($1='DEAN' AND EXISTS (SELECT 1 FROM courses c JOIN faculties f ON f."facultyID"=c."facultyID" JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID" WHERE c."courseID"=r."courseID" AND ds."employerID"=NULLIF($4,'')::uuid)) OR $1='SUPPORT')`
	scoreScope := `(($1='STUDENT' AND r."studentID"=NULLIF($2,'')::uuid) OR ($1='TEACHER' AND r."teacherID"=NULLIF($3,'')::uuid AND EXISTS (SELECT 1 FROM "courseTeacher" ct WHERE ct."courseID"=r."courseID" AND ct."teacherID"=NULLIF($3,'')::uuid)) OR ($1='DEAN' AND EXISTS (SELECT 1 FROM students st JOIN faculties f ON f."facultyID"=st."facultyID" JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID" WHERE st."studentID"=r."studentID" AND ds."employerID"=NULLIF($4,'')::uuid)))`
	appointmentScope := `(($1='STUDENT' AND r."studentID"=NULLIF($2,'')::uuid) OR ($1='TEACHER' AND r."teacherID"=NULLIF($3,'')::uuid) OR ($1 IN ('SUPPORT','DEAN') AND (r."createdByEmployerID"=NULLIF($4,'')::uuid OR r."employerID"=NULLIF($4,'')::uuid)))`
	deadlineScope := `EXISTS (SELECT 1 FROM courses r WHERE r."courseID"=d."courseID" AND ` + resourceCourseScope + `)`
	sections := []struct{ key, table, where, order string }{
		{"nextLessons", "lessons", lessonScope + ` AND r."startAt">=now() AND r."lessonStatus"<>'canceled'`, `"startAt", "lessonID"`},
		{"courseScores", "effective_course_scores", scoreScope, `"updated_at" DESC, "courseGraduate"`},
		{"notifications", "notifications", resourceRecipient + studentNotificationVisibility, `"notificationCreatedAt" DESC, "notificationID"`},
		{"appointments", "appointment", appointmentScope, `"created_at" DESC, "appoinmentID"`},
		{"news", "news", "true", `"createdAt" DESC NULLS LAST, "newsID" DESC`},
		{"upcomingExams", "exams", examScope + ` AND r."examStartAt">=now() AND r."examStatus"='scheduled'`, `"examStartAt", "examID"`},
		{"sportRegistrations", "sport_registrations", `$1='STUDENT' AND r."studentID"=NULLIF($2,'')::uuid AND r."registrationStatus"='enrolled'`, `"createdAt" DESC, "sportRegistrationID"`},
	}
	for _, section := range sections {
		page, e := resourcePage(r.Context(), tx, section.table, section.where, section.order, 20, 0, resourceArgs(p)...)
		if e != nil {
			resourceError(w, e)
			return
		}
		var parsed struct {
			Items json.RawMessage `json:"items"`
		}
		if e = json.Unmarshal(page, &parsed); e != nil {
			resourceError(w, e)
			return
		}
		result[section.key] = parsed.Items
	}
	// Alias d preserves the outer deadline while course scope uses its own r.
	var deadlines json.RawMessage
	err = tx.QueryRow(r.Context(), resourceIdentity+`SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT `+strings.ReplaceAll(resourceProjection("deadlines"), `r.`, `d.`)+` FROM deadlines d WHERE (`+deadlineScope+`) AND (d."deadlineEndAt" IS NULL OR d."deadlineEndAt">=now()) ORDER BY d."deadlineEndAt" NULLS LAST,d."deadlineID" LIMIT 20) x`, resourceArgs(p)...).Scan(&deadlines)
	if err != nil {
		resourceError(w, err)
		return
	}
	result["deadlines"] = deadlines
	if err = tx.Commit(r.Context()); err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, result)
}
