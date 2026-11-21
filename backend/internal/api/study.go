package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

type studyRoute struct {
	method, path, role string
	handler            func(*Server, http.ResponseWriter, *http.Request)
}

var studyRoutes = []studyRoute{
	{"GET", "/study/selection", "STUDENT", (*Server).studySelection},
	{"PUT", "/study/selection", "STUDENT", (*Server).studyConfirm},
	{"GET", "/study/management", "DEAN", (*Server).studyManagement},
	{"POST", "/study/periods", "DEAN", (*Server).studyCreatePeriod},
	{"POST", "/study/periods/{periodID}/requirements", "DEAN", (*Server).studyRequirement},
	{"POST", "/study/periods/{periodID}/streams", "DEAN", (*Server).studyStream},
	{"PATCH", "/study/periods/{periodID}", "DEAN", (*Server).studyPublish},
	{"GET", "/study/sport-calendar", "STUDENT", (*Server).sportCalendar},
	{"PUT", "/study/sport-calendar", "STUDENT", (*Server).sportCalendarSave},
	{"GET", "/study/physical-education", "STUDENT", (*Server).studyPhysical},
	{"GET", "/study/sport-semesters", "DEAN", (*Server).sportSemesterList},
	{"POST", "/study/sport-semesters", "DEAN", (*Server).sportSemesterCreate},
	{"POST", "/study/sports", "DEAN", (*Server).studyCreateSport},
	{"GET", "/study/attendance", "TEACHER", (*Server).studySessions},
	{"GET", "/study/attendance/{sportID}", "TEACHER", (*Server).studyRoster},
	{"PUT", "/study/attendance/{sportID}", "TEACHER", (*Server).studyMark},
}

var studyAndSettingsRoutes []studyRoute

func init() {
	studyAndSettingsRoutes = slices.Concat(studyRoutes, studentSettingsRoutes, activityRoutes, campusRoutes,
		campusServiceRoutes, studentSupportRoutes, careerRoutes, portfolioRoutes, staffRoutes, staffMaterialRoutes)
}

func (s *Server) registerStudy(mux *http.ServeMux) {
	for _, route := range studyAndSettingsRoutes {
		mux.HandleFunc(route.method+" /api/v1/MGIMO"+route.path, func(w http.ResponseWriter, r *http.Request) {
			if resourceGuard(w, r, route.role) != nil {
				route.handler(s, w, r)
			}
		})
	}
}

const studyStudentTerm = `
 SELECT st."studentID",st."facultyID",st."directionID",cal.year,
  (cal.year-extract(year FROM st."studentEnrollmentDate")::int)*2+
  CASE WHEN extract(month FROM cal.today) BETWEEN 2 AND 8 THEN 2 ELSE 1 END AS semester
 FROM students st
 CROSS JOIN LATERAL (SELECT now() AT TIME ZONE 'Europe/Moscow' AS today,
  extract(year FROM (now() AT TIME ZONE 'Europe/Moscow')-interval '8 months')::int AS year) cal
 WHERE st."studentID"=$1::uuid AND st."studentStatus"='active'`

const studyNext = `p."facultyID"=st."facultyID" AND p.semester=st.semester+1
 AND p."academicYear"=st.year+1-st.semester%2 AND p."isPublished"`
const studyApplicable = `req.kind<>'physical_education' AND (req."directionID" IS NULL OR req."directionID"=st."directionID")`
const studyStreamLessons = ` FROM "courseGroup" cg JOIN lessons l ON l."groupID"=cg."groupID" AND l."courseID"=cs."courseID"
 WHERE cg."courseGroupID"=cs."courseGroupID" AND l."lessonStatus"<>'canceled'
 AND l."startAt">=cg.start_date AND l."startAt"<cg.end_date `

func studyRead(ctx context.Context, q resourceQuerier, sql string, args ...any) (json.RawMessage, error) {
	var data json.RawMessage
	err := q.QueryRow(ctx, sql, args...).Scan(&data)
	return data, err
}

func (s *Server) studySelection(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH st AS (`+studyStudentTerm+`), plan AS (
 SELECT p.* FROM study_plan_periods p JOIN st ON `+studyNext+`)
 SELECT COALESCE((SELECT jsonb_build_object('periodID',p."periodID",'semester',p.semester,
 'opensAt',p."opensAt",'closesAt',p."closesAt",'open',now()>=p."opensAt" AND now()<p."closesAt" AND p."closesAt">p."opensAt"+interval '1 microsecond' ,
 'requirements',COALESCE((SELECT jsonb_agg(item ORDER BY item->>'title') FROM (
 SELECT jsonb_build_object('courseID',c."courseID",'title',c.title,
 'kind',CASE WHEN bool_or(req.kind='common') THEN 'common' ELSE 'profile' END,
 'streams',COALESCE((SELECT jsonb_agg(jsonb_build_object('streamID',cs."streamID",'teacher',t."teacherName",
 'remaining',greatest(0,cs.capacity-(SELECT count(*) FROM student_stream_selections x WHERE x."streamID"=cs."streamID")),
 'selected',own."streamID" IS NOT NULL,'open',cs."isOpen",
 'lessons',COALESCE((SELECT jsonb_agg(jsonb_build_object('startAt',l."startAt",'endAt',l."endAt") ORDER BY l."startAt") `+studyStreamLessons+`),'[]'))
 ORDER BY t."teacherName",cs."streamID") FROM course_streams cs JOIN teachers t USING ("teacherID")
 LEFT JOIN student_stream_selections own ON own."streamID"=cs."streamID" AND own."studentID"=st."studentID"
 WHERE cs."periodID"=p."periodID" AND cs."courseID"=c."courseID" AND (cs."isOpen" OR own."streamID" IS NOT NULL)), '[]')) item
 FROM study_plan_requirements req JOIN courses c USING ("courseID")
 WHERE req."periodID"=p."periodID" AND `+studyApplicable+` GROUP BY c."courseID") items),'[]'))
 FROM plan p CROSS JOIN st),'null'::jsonb)`, []any{current(r).StudentID})
}

func (s *Server) studyConfirm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PeriodID  string
		StreamIDs []string
	}
	if !decode(w, r, &in) {
		return
	}
	if !resourceUUID(in.PeriodID) || len(in.StreamIDs) == 0 || len(in.StreamIDs) > 500 || !studyUUIDs(in.StreamIDs...) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите по одному потоку для всех дисциплин")
		return
	}
	student := current(r).StudentID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var open bool
		var start, end string
		err := tx.QueryRow(ctx, `WITH st AS (`+studyStudentTerm+`)
  SELECT now()>=p."opensAt" AND now()<p."closesAt" AND p."closesAt">p."opensAt"+interval '1 microsecond' ,study_term_start(p."academicYear",p.semester)::text,study_term_end(p."academicYear",p.semester)::text
  FROM study_plan_periods p JOIN st ON `+studyNext+` WHERE p."periodID"=$2::uuid FOR UPDATE OF p`, student, in.PeriodID).Scan(&open, &start, &end)
		if err != nil {
			return nil, err
		}
		if !open {
			return nil, studyConflict("Окно выбора закрыто")
		}
		var locked int
		if err = tx.QueryRow(ctx, `SELECT 1 FROM students WHERE "studentID"=$1::uuid AND "studentStatus"='active' FOR UPDATE`, student).Scan(&locked); err != nil {
			return nil, err
		}
		var valid, available, overlap bool
		err = tx.QueryRow(ctx, `WITH st AS (`+studyStudentTerm+`),
  required AS (SELECT DISTINCT req."courseID" FROM study_plan_requirements req CROSS JOIN st WHERE req."periodID"=$2::uuid AND `+studyApplicable+`),
  chosen AS (SELECT cs.* FROM course_streams cs WHERE cs."periodID"=$2::uuid AND cs."streamID"=ANY($3::text[]::uuid[])),
  times AS (SELECT time.* FROM chosen cs CROSS JOIN LATERAL (SELECT l."lessonID",l."startAt",l."endAt" `+studyStreamLessons+`) time),
  other AS (SELECT l."startAt",l."endAt" FROM student_lesson_access a JOIN lessons l USING ("lessonID") WHERE a."studentID"=$1::uuid
   AND l."lessonStatus"<>'canceled' AND NOT EXISTS(SELECT 1 FROM required d WHERE d."courseID"=l."courseID")
   AND l."startAt"<$6::text::timestamptz AND l."endAt">$5::text::timestamptz)
  SELECT (SELECT count(*) FROM chosen)=$4 AND (SELECT count(DISTINCT "courseID") FROM chosen)=$4
   AND NOT EXISTS(SELECT "courseID" FROM required EXCEPT SELECT "courseID" FROM chosen)
   AND NOT EXISTS(SELECT "courseID" FROM chosen EXCEPT SELECT "courseID" FROM required),
   NOT EXISTS(SELECT 1 FROM chosen cs WHERE NOT EXISTS(SELECT 1 `+studyStreamLessons+`) OR (
    NOT EXISTS(SELECT 1 FROM student_stream_selections x WHERE x."streamID"=cs."streamID" AND x."studentID"=$1::uuid)
    AND (NOT cs."isOpen" OR cs.capacity<=(SELECT count(*) FROM student_stream_selections x WHERE x."streamID"=cs."streamID")))),
   EXISTS(SELECT 1 FROM times a JOIN times b ON a."lessonID"<b."lessonID" AND a."startAt"<b."endAt" AND b."startAt"<a."endAt")
   OR EXISTS(SELECT 1 FROM times a JOIN other b ON a."startAt"<b."endAt" AND b."startAt"<a."endAt")`,
			student, in.PeriodID, in.StreamIDs, len(in.StreamIDs), start, end).Scan(&valid, &available, &overlap)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, studyConflict("Нужен один поток по каждой обязательной дисциплине")
		}
		if !available {
			return nil, studyConflict("Один из потоков больше недоступен. Обновите список")
		}
		if overlap {
			return nil, studyConflict("Занятия пересекаются. Выберите другое время")
		}
		_, err = tx.Exec(ctx, `INSERT INTO student_stream_selections ("studentID","periodID","courseID","streamID")
  SELECT $1::uuid,"periodID","courseID","streamID" FROM course_streams WHERE "periodID"=$2::uuid AND "streamID"=ANY($3::text[]::uuid[])
  ON CONFLICT ("studentID","periodID","courseID") DO UPDATE SET "streamID"=excluded."streamID","selectedAt"=now()
  WHERE student_stream_selections."streamID"<>excluded."streamID"`, student, in.PeriodID, in.StreamIDs)
		return map[string]bool{"saved": true}, err
	})
}

type studyConflict string

func (e studyConflict) Error() string { return string(e) }
func studyUUIDs(ids ...string) bool {
	for _, id := range ids {
		if !resourceUUID(id) {
			return false
		}
	}
	return true
}
func studyWrite(s *Server, w http.ResponseWriter, r *http.Request, action func(context.Context, pgx.Tx) (any, error)) {
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		resourceError(w, err)
		return
	}
	defer resourceRollback(tx)
	result, err := action(ctx, tx)
	if err == nil {
		err = tx.Commit(ctx)
	}
	var conflict studyConflict
	if errors.As(err, &conflict) {
		fail(w, 409, "CONFLICT", conflict.Error())
		return
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, result)
}
func studyTitle(s string, n int) bool { return strings.TrimSpace(s) != "" && len([]rune(s)) <= n }
