package api

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mgimo/backend/internal/config"
)

// registerAcademic registers the academic API. Authentication
// middleware must wrap the mux; object authorization is enforced here as well.
func (s *Server) registerAcademic(mux *http.ServeMux) {
	mux.HandleFunc("PATCH /api/v1/MGIMO/lessons/{lessonID}", s.academicLessonPatch)
	mux.HandleFunc("POST /api/v1/MGIMO/academic/grades", s.academicGradeCreate)
	mux.HandleFunc("PATCH /api/v1/MGIMO/academic/grades/{courseGraduate}", s.academicGradePatch)
	mux.HandleFunc("POST /api/v1/MGIMO/academic/exams/{examID}/results", s.academicExamResultCreate)
	mux.HandleFunc("PATCH /api/v1/MGIMO/academic/exam-results/{examResultID}", s.academicExamResultPatch)
	mux.HandleFunc("GET /api/v1/MGIMO/schedule", s.academicSchedule)
	mux.HandleFunc("GET /api/v1/MGIMO/schedule/period", s.academicSchedulePeriod)
	mux.HandleFunc("GET /api/v1/MGIMO/schedule/changes", s.academicChanges)
	mux.HandleFunc("GET /api/v1/MGIMO/lessons/{lessonID}", s.academicLesson)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/subjects", s.academicSubjects)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/grades", s.academicGrades)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/deadlines", s.academicDeadlines)
	mux.HandleFunc("PATCH "+prefix+"/academic/deadlines/{deadlineID}/submission-link", s.deadlineLinkPatch)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/subjects/{courseID}/results", s.academicResults)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/exams", s.academicExams)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/exams/{examID}", s.academicExam)
	mux.HandleFunc("GET /api/v1/MGIMO/academic/exams/{examID}/results", s.academicExamResults)
}

var academicUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func academicAllowRole(w http.ResponseWriter, r *http.Request, support bool) *Principal {
	p := current(r)
	if p == nil {
		fail(w, 401, "UNAUTHORIZED", "Требуется авторизация")
		return nil
	}
	allowed := academicRoleAllowed(p.Role, support)
	if !allowed {
		fail(w, 403, "FORBIDDEN", "Недостаточно прав")
		return nil
	}
	return p
}

func academicRoleAllowed(role string, support bool) bool {
	return role == "STUDENT" || role == "TEACHER" || role == "DEAN" || (support && role == "SUPPORT")
}

// The first four SQL arguments are always the principal's role and person IDs.
// Empty IDs are converted to NULL before PostgreSQL's uuid cast.
func academicArgs(p *Principal) []any { return []any{p.Role, p.StudentID, p.TeacherID, p.EmployerID} }

// All SQL fragments are constants owned by this package; no request text is
// interpolated into identifiers, predicates, or ordering clauses.
const academicCourseAccess = `(($1='STUDENT' AND EXISTS (
 SELECT 1 FROM student_course_access a
 WHERE a."studentID"=NULLIF($2,'')::uuid AND a."courseID"=c."courseID"))
 OR ($1='TEACHER' AND EXISTS (SELECT 1 FROM "courseTeacher" ct
 WHERE ct."courseID"=c."courseID" AND ct."teacherID"=NULLIF($3,'')::uuid))
 OR ($1='DEAN' AND EXISTS (SELECT 1 FROM faculties f JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID"
 WHERE f."facultyID"=c."facultyID" AND ds."employerID"=NULLIF($4,'')::uuid)))`

const academicLessonAccess = `(($1='STUDENT' AND EXISTS (SELECT 1 FROM student_lesson_access a
 WHERE a."studentID"=NULLIF($2,'')::uuid AND a."lessonID"=l."lessonID"))
 OR ($1='TEACHER' AND l."teacherID"=NULLIF($3,'')::uuid)
 OR ($1='DEAN' AND EXISTS (SELECT 1 FROM courses c JOIN faculties f ON f."facultyID"=c."facultyID"
 JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID"
 WHERE c."courseID"=l."courseID" AND ds."employerID"=NULLIF($4,'')::uuid)) OR $1='SUPPORT')`

const academicExamAccess = `(($1='STUDENT' AND EXISTS (SELECT 1 FROM student_exam_access a
 WHERE a."studentID"=NULLIF($2,'')::uuid AND a."examID"=e."examID"))
 OR ($1='TEACHER' AND e."teacherID"=NULLIF($3,'')::uuid)
 OR ($1='DEAN' AND EXISTS (SELECT 1 FROM courses c JOIN faculties f ON f."facultyID"=c."facultyID"
 JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID"
 WHERE c."courseID"=e."courseID" AND ds."employerID"=NULLIF($4,'')::uuid)))`

const academicScoreAccess = `($1<>'STUDENT' OR g."studentID"=NULLIF($2,'')::uuid)
 AND EXISTS (SELECT 1 FROM student_course_access a WHERE a."studentID"=g."studentID" AND a."courseID"=c."courseID")
 AND ($1<>'TEACHER' OR EXISTS(SELECT 1 FROM student_course_teacher_access a
 WHERE a."studentID"=g."studentID" AND a."courseID"=c."courseID" AND a."teacherID"=NULLIF($3,'')::uuid))`

func academicPagination(q url.Values) (int, int, error) {
	limit, offset := 20, 0
	for _, v := range []struct {
		name   string
		target *int
	}{{"limit", &limit}, {"offset", &offset}} {
		if values, ok := q[v.name]; ok {
			if len(values) != 1 {
				return 0, 0, errors.New("duplicate pagination parameter")
			}
			n, err := strconv.Atoi(values[0])
			if err != nil {
				return 0, 0, err
			}
			*v.target = n
		}
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return 0, 0, errors.New("invalid pagination")
	}
	return limit, offset, nil
}

func academicInteger(value string) (int, error) {
	n, err := strconv.ParseInt(value, 10, 32)
	return int(n), err
}

func academicOptionalInteger(w http.ResponseWriter, r *http.Request, key string) (any, bool) {
	values, ok := r.URL.Query()[key]
	if !ok {
		return nil, true
	}
	if len(values) != 1 {
		fail(w, 400, "BAD_REQUEST", "Параметр повторяется: "+key)
		return nil, false
	}
	n, err := academicInteger(values[0])
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректный параметр: "+key)
		return nil, false
	}
	return n, true
}

func academicCourseFilter(w http.ResponseWriter, r *http.Request) (any, bool) {
	values, ok := r.URL.Query()["courseID"]
	if !ok {
		return nil, true
	}
	if len(values) != 1 || !academicUUID.MatchString(values[0]) {
		fail(w, 400, "BAD_REQUEST", "Некорректный courseID")
		return nil, false
	}
	return values[0], true
}

func academicDBError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "NOT_FOUND", "Запись не найдена")
		return
	}
	fail(w, 500, "INTERNAL_ERROR", "Не удалось прочитать учебные данные")
}

// Fetch JSON from PostgreSQL to preserve the schema's exact camel-case keys and
// nullable fields without leaking any columns from unrelated joined tables.
func (s *Server) academicRead(w http.ResponseWriter, r *http.Request, sql string, args []any) {
	var data json.RawMessage
	if err := s.DB.QueryRow(r.Context(), sql, args...).Scan(&data); err != nil {
		academicDBError(w, err)
		return
	}
	write(w, 200, data)
}

func (s *Server) academicPage(w http.ResponseWriter, r *http.Request, p *Principal, source, order string, extra ...any) {
	limit, offset, err := academicPagination(r.URL.Query())
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректные limit или offset")
		return
	}
	args := append(academicArgs(p), extra...)
	// The two final placeholders follow the constant source's arguments.
	a, b := strconv.Itoa(len(args)+1), strconv.Itoa(len(args)+2)
	sql := `WITH available AS (` + source + `), page AS (SELECT obj FROM available ORDER BY ` + order + ` LIMIT $` + a + ` OFFSET $` + b + `)
 SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(obj) FROM page),'[]'::jsonb),
 'total',(SELECT count(*) FROM available),'limit',$` + a + `::int,'offset',$` + b + `::int)`
	args = append(args, limit, offset)
	s.academicRead(w, r, sql, args)
}

func academicDates(q url.Values, period bool) (time.Time, time.Time, error) {
	keys := []string{"date"}
	if period {
		keys = []string{"from", "to"}
	}
	dates := make([]time.Time, 0, len(keys))
	// University calendar dates are interpreted in Moscow, independently of the
	// database session timezone or host timezone (including the inclusive end day).
	zone := time.FixedZone("Europe/Moscow", 3*60*60)
	for _, key := range keys {
		v := q[key]
		if len(v) != 1 {
			return time.Time{}, time.Time{}, errors.New("missing or duplicate date")
		}
		d, err := time.ParseInLocation("2006-01-02", v[0], zone)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		dates = append(dates, d)
	}
	if !period {
		return dates[0], dates[0], nil
	}
	if dates[1].Before(dates[0]) {
		return time.Time{}, time.Time{}, errors.New("reversed period")
	}
	return dates[0], dates[1], nil
}

const academicLessonJSON = `to_jsonb(l)||jsonb_build_object(
 'teacher',(SELECT jsonb_build_object('teacherID',t."teacherID",'teacherName',t."teacherName") FROM teachers t WHERE t."teacherID"=l."teacherID"),
 'classroom',(SELECT to_jsonb(cr) FROM classroom cr WHERE cr."classroomID"=l."classroomID"),
 'building',(SELECT to_jsonb(b) FROM buildings b JOIN classroom cr ON cr."buildingID"=b."buildingID" WHERE cr."classroomID"=l."classroomID"),
 'course',(SELECT to_jsonb(c) FROM courses c WHERE c."courseID"=l."courseID"))`

func (s *Server) academicSchedule(w http.ResponseWriter, r *http.Request) {
	s.academicScheduleRead(w, r, false)
}
func (s *Server) academicSchedulePeriod(w http.ResponseWriter, r *http.Request) {
	s.academicScheduleRead(w, r, true)
}
func (s *Server) academicScheduleRead(w http.ResponseWriter, r *http.Request, period bool) {
	p := academicAllowRole(w, r, true)
	if p == nil {
		return
	}
	from, to, err := academicDates(r.URL.Query(), period)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная дата или период")
		return
	}
	group, ok := academicOptionalInteger(w, r, "groupID")
	if !ok {
		return
	}
	if p.Role == "STUDENT" && group != nil {
		var own bool
		err = s.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM students WHERE "studentID"=NULLIF($1,'')::uuid AND "groupID"=$2 UNION ALL SELECT 1 FROM student_lesson_access a JOIN lessons l USING ("lessonID") WHERE a."studentID"=NULLIF($1,'')::uuid AND l."groupID"=$2)`, p.StudentID, group).Scan(&own)
		if err != nil {
			academicDBError(w, err)
			return
		}
		if !own {
			fail(w, 403, "FORBIDDEN", "Недоступная группа")
			return
		}
	}
	args := append(academicArgs(p), from, to.AddDate(0, 0, 1), group)
	var lessons json.RawMessage
	err = s.DB.QueryRow(r.Context(), `SELECT COALESCE(jsonb_agg(`+academicLessonJSON+` ORDER BY l."startAt",l."lessonID"),'[]'::jsonb)
 FROM lessons l WHERE `+academicLessonAccess+` AND l."startAt">=$5 AND l."startAt"<$6 AND ($7::int IS NULL OR l."groupID"=$7)`, args...).Scan(&lessons)
	if err != nil {
		academicDBError(w, err)
		return
	}
	if period {
		write(w, 200, map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "lessons": lessons})
	} else {
		write(w, 200, map[string]any{"date": from.Format("2006-01-02"), "lessons": lessons})
	}
}

func (s *Server) academicLesson(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, true)
	if p == nil {
		return
	}
	id, err := academicInteger(r.PathValue("lessonID"))
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректный lessonID")
		return
	}
	var exists, allowed bool
	err = s.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM lessons WHERE "lessonID"=$5),EXISTS(SELECT 1 FROM lessons l WHERE "lessonID"=$5 AND `+academicLessonAccess+`)`, append(academicArgs(p), id)...).Scan(&exists, &allowed)
	if err != nil {
		academicDBError(w, err)
		return
	}
	if exists && !allowed {
		fail(w, 403, "FORBIDDEN", "Нет доступа к занятию")
		return
	}
	s.academicRead(w, r, `SELECT `+academicLessonJSON+` FROM lessons l WHERE `+academicLessonAccess+` AND l."lessonID"=$5`, append(academicArgs(p), id))

}

func (s *Server) academicChanges(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, true)
	if p == nil {
		return
	}
	id, ok := academicOptionalInteger(w, r, "lessonID")
	if !ok {
		return
	}
	s.academicPage(w, r, p, `SELECT to_jsonb(h) obj,h."changedAt" sort_at,h."lessonChangeID" sort_id FROM lesson_changes h
 JOIN lessons l ON l."lessonID"=h."lessonID" WHERE `+academicLessonAccess+` AND ($5::int IS NULL OR l."lessonID"=$5)`, `sort_at DESC,sort_id DESC`, id)
}

const academicCourseJSON = `to_jsonb(c)||jsonb_build_object('teachers',COALESCE((
 SELECT jsonb_agg(jsonb_build_object('teacherID',t."teacherID",'teacherName',t."teacherName") ORDER BY t."teacherName",t."teacherID")
 FROM "courseTeacher" ct JOIN teachers t ON t."teacherID"=ct."teacherID"
 WHERE ct."courseID"=c."courseID" AND ($1<>'STUDENT' OR EXISTS(
 SELECT 1 FROM student_course_teacher_access a WHERE a."studentID"=NULLIF($2,'')::uuid AND a."courseID"=c."courseID" AND a."teacherID"=t."teacherID"))),'[]'::jsonb))`

func (s *Server) academicSubjects(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	s.academicPage(w, r, p, `SELECT `+academicCourseJSON+` obj,c.title sort_title,c."courseID" sort_id FROM courses c WHERE `+academicCourseAccess, `sort_title,sort_id`)
}

func (s *Server) academicGrades(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	course, ok := academicCourseFilter(w, r)
	if !ok {
		return
	}
	s.academicPage(w, r, p, `SELECT to_jsonb(g) obj,g.created_at sort_at,g."courseGraduate" sort_id FROM effective_course_scores g
 JOIN courses c ON c."courseID"=g."courseID" WHERE `+academicCourseAccess+` AND `+academicScoreAccess+`
 AND ($5::uuid IS NULL OR c."courseID"=$5)`, `sort_at DESC,sort_id`, course)
}

func (s *Server) academicDeadlines(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	course, ok := academicCourseFilter(w, r)
	if !ok {
		return
	}
	moodle := s.Config.MoodleURL
	if moodle == "" {
		moodle = config.DefaultMoodleURL
	}
	s.academicPage(w, r, p, `SELECT to_jsonb(d)||jsonb_build_object('submissionURL',COALESCE(dl."submissionURL",$6::text),'customSubmissionURL',dl."submissionURL") obj,d."deadlineEndAt" sort_at,d."deadlineID" sort_id FROM deadlines d
 JOIN courses c ON c."courseID"=d."courseID" LEFT JOIN deadline_links dl ON dl."deadlineID"=d."deadlineID" WHERE `+academicCourseAccess+` AND ($5::uuid IS NULL OR c."courseID"=$5)`, `sort_at NULLS LAST,sort_id`, course, moodle)
}

func (s *Server) academicExams(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	course, ok := academicCourseFilter(w, r)
	if !ok {
		return
	}
	s.academicPage(w, r, p, `SELECT to_jsonb(e) obj,e."examStartAt" sort_at,e."examID" sort_id FROM exams e
 WHERE `+academicExamAccess+` AND ($5::uuid IS NULL OR e."courseID"=$5)`, `sort_at,sort_id`, course)
}

func academicPathUUID(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	id := r.PathValue(key)
	if !academicUUID.MatchString(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный "+key)
		return "", false
	}
	return id, true
}

func (s *Server) academicExam(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	id, ok := academicPathUUID(w, r, "examID")
	if !ok {
		return
	}
	s.academicRead(w, r, `SELECT to_jsonb(e) FROM exams e WHERE `+academicExamAccess+` AND e."examID"=$5::uuid`, append(academicArgs(p), id))
}

func (s *Server) academicExamResults(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	id, ok := academicPathUUID(w, r, "examID")
	if !ok {
		return
	}
	// A missing or inaccessible parent is not an empty successful result page.
	var exists bool
	err := s.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM exams e WHERE `+academicExamAccess+` AND e."examID"=$5::uuid)`, append(academicArgs(p), id)...).Scan(&exists)
	if err != nil {
		academicDBError(w, err)
		return
	}
	if !exists {
		fail(w, 404, "NOT_FOUND", "Экзамен не найден")
		return
	}
	s.academicPage(w, r, p, `SELECT to_jsonb(er) obj,er."studentID" sort_id FROM exam_results er JOIN exams e ON e."examID"=er."examID"
 WHERE `+academicExamAccess+` AND e."examID"=$5::uuid AND ($1<>'STUDENT' OR er."studentID"=NULLIF($2,'')::uuid)`, `sort_id`, id)
}

func (s *Server) academicResults(w http.ResponseWriter, r *http.Request) {
	p := academicAllowRole(w, r, false)
	if p == nil {
		return
	}
	id, ok := academicPathUUID(w, r, "courseID")
	if !ok {
		return
	}
	// One statement gives all result sections a consistent database snapshot.
	s.academicRead(w, r, `SELECT jsonb_build_object('course',`+academicCourseJSON+`,
 'scores',COALESCE((SELECT jsonb_agg(to_jsonb(g) ORDER BY g."courseGraduate") FROM effective_course_scores g
 WHERE g."courseID"=c."courseID" AND `+academicScoreAccess+`),'[]'::jsonb),
 'deadlines',COALESCE((SELECT jsonb_agg(to_jsonb(d) ORDER BY d."deadlineEndAt",d."deadlineID") FROM deadlines d
 WHERE d."courseID"=c."courseID"),'[]'::jsonb),
 'exams',COALESCE((SELECT jsonb_agg(to_jsonb(e) ORDER BY e."examStartAt",e."examID") FROM exams e
 WHERE e."courseID"=c."courseID" AND `+academicExamAccess+`),'[]'::jsonb),
 'examResults',COALESCE((SELECT jsonb_agg(to_jsonb(er) ORDER BY er."examResultID") FROM exam_results er JOIN exams e ON e."examID"=er."examID"
 WHERE e."courseID"=c."courseID" AND `+academicExamAccess+` AND ($1<>'STUDENT' OR er."studentID"=NULLIF($2,'')::uuid)),'[]'::jsonb))
 FROM courses c WHERE c."courseID"=$5::uuid AND `+academicCourseAccess, append(academicArgs(p), id))
}

func academicWriter(w http.ResponseWriter, r *http.Request, role string) *Principal {
	p := current(r)
	if p == nil {
		fail(w, 401, "UNAUTHORIZED", "Требуется авторизация")
		return nil
	}
	if p.Role != role {
		fail(w, 403, "FORBIDDEN", "Недостаточно прав")
		return nil
	}
	return p
}

func academicWriteError(w http.ResponseWriter, err error) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505", "40001", "40P01":
			fail(w, 409, "CONFLICT", "Конфликт изменения данных")
			return
		case "23503", "23514", "22001", "22003":
			fail(w, 422, "VALIDATION_ERROR", "Данные нарушают ограничения")
			return
		}
	}
	academicDBError(w, err)
}

type academicLessonState struct {
	LessonID         int       `json:"lessonID"`
	GroupID          int       `json:"groupID"`
	ClassroomID      int       `json:"classroomID"`
	TeacherID        string    `json:"teacherID"`
	Topic            string    `json:"topic"`
	StartAt          time.Time `json:"startAt"`
	EndAt            time.Time `json:"endAt"`
	RemoteLessonLink *string   `json:"remoteLessonLink"`
	LessonStatus     string    `json:"lessonStatus"`
	CourseID         string    `json:"courseID"`
}

type academicLessonPatchBody struct {
	StartAt          json.RawMessage `json:"startAt"`
	EndAt            json.RawMessage `json:"endAt"`
	ClassroomID      json.RawMessage `json:"classroomID"`
	TeacherID        json.RawMessage `json:"teacherID"`
	RemoteLessonLink json.RawMessage `json:"remoteLessonLink"`
	LessonStatus     json.RawMessage `json:"lessonStatus"`
	Reason           *string         `json:"reason"`
}

// RawMessage distinguishes omission from explicit null; only the link is nullable.
func academicApplyLesson(old academicLessonState, in academicLessonPatchBody) (academicLessonState, error) {
	next := old
	if in.Reason == nil || utf8.RuneCountInString(*in.Reason) > 564 {
		return next, errors.New("Укажите причину длиной до 564 символов")
	}
	fields := []struct {
		raw    json.RawMessage
		target any
	}{
		{in.StartAt, &next.StartAt}, {in.EndAt, &next.EndAt}, {in.ClassroomID, &next.ClassroomID},
		{in.TeacherID, &next.TeacherID}, {in.LessonStatus, &next.LessonStatus},
	}
	count := 0
	for _, f := range fields {
		if len(f.raw) == 0 {
			continue
		}
		count++
		if strings.TrimSpace(string(f.raw)) == "null" {
			return next, errors.New("Обязательное поле не может быть null")
		}
		if err := json.Unmarshal(f.raw, f.target); err != nil {
			return next, errors.New("Некорректное значение поля занятия")
		}
	}
	if len(in.RemoteLessonLink) > 0 {
		count++
		if err := json.Unmarshal(in.RemoteLessonLink, &next.RemoteLessonLink); err != nil {
			return next, errors.New("Некорректная ссылка")
		}
	}
	if count == 0 {
		return next, errors.New("Укажите хотя бы одно изменение")
	}
	if !next.EndAt.After(next.StartAt) {
		return next, errors.New("Конец занятия должен быть позже начала")
	}
	if !academicUUID.MatchString(next.TeacherID) {
		return next, errors.New("Некорректный teacherID")
	}
	if int64(next.ClassroomID) < -2147483648 || int64(next.ClassroomID) > 2147483647 {
		return next, errors.New("Некорректный classroomID")
	}
	if next.LessonStatus != "active" && next.LessonStatus != "canceled" && next.LessonStatus != "rescheduled" {
		return next, errors.New("Некорректный статус занятия")
	}
	if next.RemoteLessonLink != nil {
		link := *next.RemoteLessonLink
		u, err := url.ParseRequestURI(link)
		if utf8.RuneCountInString(link) > 1024 || err != nil || u.Scheme == "" {
			return next, errors.New("Некорректный URI занятия")
		}
	}
	return next, nil
}

func (s *Server) academicLessonPatch(w http.ResponseWriter, r *http.Request) {
	p := academicWriter(w, r, "SUPPORT")
	if p == nil {
		return
	}
	id, err := academicInteger(r.PathValue("lessonID"))
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректный lessonID")
		return
	}
	var in academicLessonPatchBody
	if !decode(w, r, &in) {
		return
	}
	if !academicUUID.MatchString(p.EmployerID) {
		fail(w, 403, "FORBIDDEN", "Не определён сотрудник")
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		academicWriteError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// No sequence/default exists in the agreed schema. Serialize allocation of the
	// integer history key before taking the lesson row lock (consistent lock order).
	if _, err = tx.Exec(r.Context(), `LOCK TABLE lesson_changes IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		academicWriteError(w, err)
		return
	}
	var raw json.RawMessage
	if err = tx.QueryRow(r.Context(), `SELECT to_jsonb(l) FROM lessons l WHERE "lessonID"=$1 FOR UPDATE`, id).Scan(&raw); err != nil {
		academicWriteError(w, err)
		return
	}
	var old academicLessonState
	if err = json.Unmarshal(raw, &old); err != nil {
		academicWriteError(w, err)
		return
	}
	next, err := academicApplyLesson(old, in)
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", err.Error())
		return
	}
	err = tx.QueryRow(r.Context(), `UPDATE lessons l SET "startAt"=$2,"endAt"=$3,"classroomID"=$4,"teacherID"=$5::uuid,
 "remoteLessonLink"=$6,"lessonStatus"=$7::lesson_status WHERE "lessonID"=$1 RETURNING to_jsonb(l)`,
		id, next.StartAt, next.EndAt, next.ClassroomID, next.TeacherID, next.RemoteLessonLink, next.LessonStatus).Scan(&raw)
	if err != nil {
		academicWriteError(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO lesson_changes ("lessonChangeID","lessonID","changedAt","oldStartAt","oldEndAt","newStartAt","newEndAt",
 "oldRoomID","newRoomID","oldTeacherID","newTeacherID","reason","changedByEmployerID","oldLessonStatus","newLessonStatus","oldRemoteLessonLink","newRemoteLessonLink")
 SELECT COALESCE(MAX("lessonChangeID"),0)+1,$1,now(),$2,$3,$4,$5,$6,$7,$8::uuid,$9::uuid,$10,$11::uuid,$12::lesson_status,$13::lesson_status,$14,$15 FROM lesson_changes`,
		id, old.StartAt, old.EndAt, next.StartAt, next.EndAt, old.ClassroomID, next.ClassroomID, old.TeacherID, next.TeacherID, *in.Reason, p.EmployerID, old.LessonStatus, next.LessonStatus, old.RemoteLessonLink, next.RemoteLessonLink)
	if err != nil {
		academicWriteError(w, err)
		return
	}
	// Notify every student in the group and both affected teachers. UNION avoids
	// duplicate teacher notifications when the teacher did not change.
	_, err = tx.Exec(r.Context(), `INSERT INTO notifications ("notificationID","studentID","teacherID","notificationTitle","notificationMessage","notificationCategory","notificationCreatedAt","lessonID")
 SELECT gen_random_uuid(),rec.student,rec.teacher,'Изменение расписания','Обновлено занятие. Откройте карточку занятия для подробностей.','schedule_change',now(),$1
 FROM (SELECT "studentID" student,NULL::uuid teacher FROM students WHERE "groupID"=$2
 UNION SELECT NULL::uuid,$3::uuid UNION SELECT NULL::uuid,$4::uuid) rec`, id, old.GroupID, old.TeacherID, next.TeacherID)
	if err != nil {
		academicWriteError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		academicWriteError(w, err)
		return
	}
	write(w, 200, raw)
}

type academicGradeBody struct {
	CourseID  string          `json:"courseID"`
	StudentID string          `json:"studentID"`
	Score     json.RawMessage `json:"score"`
}

type academicScoreBody struct {
	Score json.RawMessage `json:"score"`
}
type academicExamResultBody struct {
	StudentID string          `json:"studentID"`
	Score     json.RawMessage `json:"score"`
}

func academicParseScore(raw json.RawMessage, nullable bool) (*int, error) {
	if len(raw) == 0 {
		return nil, errors.New("Укажите score")
	}
	if strings.TrimSpace(string(raw)) == "null" {
		if nullable {
			return nil, nil
		}
		return nil, errors.New("score не может быть null")
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil || n < 0 || n > 100 {
		return nil, errors.New("score должен быть целым числом от 0 до 100")
	}
	return &n, nil
}

// Lock all authorization edges, so group reassignment/course assignment removal
// cannot interleave between permission checking and committing a score.
func academicGradePermission(r *http.Request, tx pgx.Tx, teacher, course, student string) error {
	var one int
	if err := tx.QueryRow(r.Context(), `SELECT 1 FROM students WHERE "studentID"=$1::uuid FOR SHARE`, student).Scan(&one); err != nil {
		return err
	}
	return tx.QueryRow(r.Context(), `SELECT 1 FROM "courseTeacher" ct
 WHERE ct."courseID"=$1::uuid AND ct."teacherID"=$2::uuid AND EXISTS(
 SELECT 1 FROM student_course_teacher_access a WHERE a."courseID"=ct."courseID" AND a."teacherID"=ct."teacherID" AND a."studentID"=$3::uuid)
 AND NOT EXISTS(SELECT 1 FROM study_plan_periods p JOIN study_plan_requirements req USING ("periodID")
 JOIN students st ON st."facultyID"=p."facultyID"
 WHERE req.kind='physical_education' AND req."courseID"=ct."courseID" AND p."isPublished" AND st."studentID"=$3::uuid
 AND p."academicYear"=extract(year FROM st."studentEnrollmentDate")::int+(p.semester-1)/2)
 FOR SHARE OF ct`, course, teacher, student).Scan(&one)
}

func academicPermissionError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 403, "FORBIDDEN", "Студент или курс недоступен преподавателю")
		return
	}
	academicWriteError(w, err)
}

func (s *Server) academicGradeCreate(w http.ResponseWriter, r *http.Request) {
	p := academicWriter(w, r, "TEACHER")
	if p == nil {
		return
	}
	var in academicGradeBody
	if !decode(w, r, &in) {
		return
	}
	score, err := academicParseScore(in.Score, false)
	if err != nil || !academicUUID.MatchString(in.CourseID) || !academicUUID.MatchString(in.StudentID) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите UUID курса, студента и score от 0 до 100")
		return
	}
	s.academicSaveGrade(w, r, p, "", in.CourseID, in.StudentID, *score)
}

func (s *Server) academicGradePatch(w http.ResponseWriter, r *http.Request) {
	p := academicWriter(w, r, "TEACHER")
	if p == nil {
		return
	}
	id, ok := academicPathUUID(w, r, "courseGraduate")
	if !ok {
		return
	}
	var in academicScoreBody
	if !decode(w, r, &in) {
		return
	}
	score, err := academicParseScore(in.Score, false)
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", err.Error())
		return
	}
	s.academicSaveGrade(w, r, p, id, "", "", *score)
}

func (s *Server) academicSaveGrade(w http.ResponseWriter, r *http.Request, p *Principal, id, course, student string, score int) {
	if !academicUUID.MatchString(p.TeacherID) {
		fail(w, 403, "FORBIDDEN", "Не определён преподаватель")
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		academicWriteError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	creating := id == ""
	if !creating {
		err = tx.QueryRow(r.Context(), `SELECT "courseID"::text,"studentID"::text FROM "courseScore" WHERE "courseGraduate"=$1::uuid FOR UPDATE`, id).Scan(&course, &student)
		if err != nil {
			academicWriteError(w, err)
			return
		}
	}
	if err = academicGradePermission(r, tx, p.TeacherID, course, student); err != nil {
		academicPermissionError(w, err)
		return
	}
	var raw json.RawMessage
	if creating {
		id = uuid.NewString()
		err = tx.QueryRow(r.Context(), `INSERT INTO "courseScore" AS g ("courseGraduate","courseID","studentID","teacherID",score,credit,created_at,updated_at)
 VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,now(),now()) RETURNING to_jsonb(g)`, id, course, student, p.TeacherID, score, score >= 60).Scan(&raw)
	} else {
		err = tx.QueryRow(r.Context(), `UPDATE "courseScore" g SET score=$2,credit=$3,"teacherID"=$4::uuid,updated_at=now()
 WHERE "courseGraduate"=$1::uuid RETURNING to_jsonb(g)`, id, score, score >= 60, p.TeacherID).Scan(&raw)
	}
	if err != nil {
		academicWriteError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		academicWriteError(w, err)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	write(w, status, raw)
}

func (s *Server) academicExamResultCreate(w http.ResponseWriter, r *http.Request) {
	p := academicWriter(w, r, "TEACHER")
	if p == nil {
		return
	}
	exam, ok := academicPathUUID(w, r, "examID")
	if !ok {
		return
	}
	var in academicExamResultBody
	if !decode(w, r, &in) {
		return
	}
	score, err := academicParseScore(in.Score, true)
	if err != nil || !academicUUID.MatchString(in.StudentID) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите UUID студента и score от 0 до 100 либо null")
		return
	}
	s.academicSaveExamResult(w, r, p, "", exam, in.StudentID, score)
}

func (s *Server) academicExamResultPatch(w http.ResponseWriter, r *http.Request) {
	p := academicWriter(w, r, "TEACHER")
	if p == nil {
		return
	}
	id, ok := academicPathUUID(w, r, "examResultID")
	if !ok {
		return
	}
	var in academicScoreBody
	if !decode(w, r, &in) {
		return
	}
	score, err := academicParseScore(in.Score, true)
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", err.Error())
		return
	}
	s.academicSaveExamResult(w, r, p, id, "", "", score)
}

func (s *Server) academicSaveExamResult(w http.ResponseWriter, r *http.Request, p *Principal, id, exam, student string, score *int) {
	if !academicUUID.MatchString(p.TeacherID) {
		fail(w, 403, "FORBIDDEN", "Не определён преподаватель")
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		academicWriteError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	creating := id == ""
	if !creating {
		err = tx.QueryRow(r.Context(), `SELECT "examID"::text,"studentID"::text FROM exam_results WHERE "examResultID"=$1::uuid FOR UPDATE`, id).Scan(&exam, &student)
		if err != nil {
			academicWriteError(w, err)
			return
		}
	}
	var course, teacher string
	var group int
	err = tx.QueryRow(r.Context(), `SELECT "courseID"::text,"teacherID"::text,"groupID" FROM exams WHERE "examID"=$1::uuid FOR SHARE`, exam).Scan(&course, &teacher, &group)
	if err != nil {
		academicWriteError(w, err)
		return
	}
	if !strings.EqualFold(teacher, p.TeacherID) {
		fail(w, 403, "FORBIDDEN", "Экзамен другого преподавателя")
		return
	}
	if err = academicGradePermission(r, tx, p.TeacherID, course, student); err != nil {
		academicPermissionError(w, err)
		return
	}
	var sameGroup bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM student_exam_access WHERE "studentID"=$1::uuid AND "examID"=$2::uuid)`, student, exam).Scan(&sameGroup)
	if err != nil {
		academicWriteError(w, err)
		return
	}
	if !sameGroup {
		fail(w, 403, "FORBIDDEN", "Студент не принадлежит группе экзамена")
		return
	}
	var raw json.RawMessage
	if creating {
		err = tx.QueryRow(r.Context(), `INSERT INTO exam_results AS er ("examResultID","examID","studentID",score,"createdAt")
 VALUES ($1::uuid,$2::uuid,$3::uuid,$4,now()) RETURNING to_jsonb(er)`, uuid.NewString(), exam, student, score).Scan(&raw)
	} else {
		err = tx.QueryRow(r.Context(), `UPDATE exam_results er SET score=$2,"updatedAt"=now() WHERE "examResultID"=$1::uuid RETURNING to_jsonb(er)`, id, score).Scan(&raw)
	}
	if err != nil {
		academicWriteError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		academicWriteError(w, err)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	write(w, status, raw)
}
