package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const studyDeanFaculties = `SELECT f."facultyID",f.title FROM faculties f JOIN deanery_staff ds USING ("deaneryID") WHERE ds."employerID"=$1::uuid`
const studyTeacherActive = `t."teacherStatus"='active' AND EXISTS(SELECT 1 FROM accounts ac WHERE ac."teacherID"=t."teacherID" AND ac."isActive")`

func (s *Server) studyManagement(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH faculty AS (`+studyDeanFaculties+`)
 SELECT jsonb_build_object(
 'faculties',COALESCE((SELECT jsonb_agg(to_jsonb(f) ORDER BY title) FROM faculty f),'[]'),
 'directions',COALESCE((SELECT jsonb_agg(to_jsonb(d) ORDER BY "directionName") FROM direction d JOIN faculty USING ("facultyID")),'[]'),
 'courses',COALESCE((SELECT jsonb_agg(to_jsonb(c) ORDER BY c.title) FROM courses c JOIN faculty USING ("facultyID")),'[]'),
 'teachers',COALESCE((SELECT jsonb_agg(jsonb_build_object('facultyID',ft."facultyID",'teacherID',t."teacherID",'name',t."teacherName") ORDER BY t."teacherName")
 FROM "facultyTeacher" ft JOIN faculty USING ("facultyID") JOIN teachers t USING ("teacherID") WHERE `+studyTeacherActive+`),'[]'),
 'rooms',COALESCE((SELECT jsonb_agg(to_jsonb(cl)||jsonb_build_object('building',b.building) ORDER BY b.building,cl."classroomNumber") FROM classroom cl JOIN buildings b USING ("buildingID")),'[]'),
 'buildings',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY b.building) FROM buildings b),'[]'),
 'periods',COALESCE((SELECT jsonb_agg(to_jsonb(p)||jsonb_build_object('closed',p."closesAt"<=now() OR p."closesAt"<=p."opensAt"+interval '1 microsecond',
  'requirements',COALESCE((SELECT jsonb_agg(to_jsonb(req)||jsonb_build_object('title',c.title,'direction',d."directionName") ORDER BY c.title) FROM study_plan_requirements req JOIN courses c USING ("courseID") LEFT JOIN direction d USING ("directionID") WHERE req."periodID"=p."periodID"),'[]'),
  'streams',COALESCE((SELECT jsonb_agg(to_jsonb(cs)||jsonb_build_object('title',c.title,'teacher',t."teacherName",
  'enrolled',(SELECT count(*) FROM student_stream_selections x WHERE x."streamID"=cs."streamID"),
  'startAt',(SELECT min(l."startAt") `+studyStreamLessons+`)) ORDER BY c.title,cs."streamID") FROM course_streams cs JOIN courses c USING ("courseID") JOIN teachers t USING ("teacherID") WHERE cs."periodID"=p."periodID"),'[]'))
  ORDER BY p."academicYear" DESC,p.semester) FROM study_plan_periods p JOIN faculty USING ("facultyID")),'[]'))`, []any{current(r).EmployerID})
}

func (s *Server) studyCreatePeriod(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FacultyID              string
		AcademicYear, Semester int
		OpensAt, ClosesAt      time.Time
	}
	if !decode(w, r, &in) {
		return
	}
	if !resourceUUID(in.FacultyID) || in.AcademicYear < 1990 || in.AcademicYear > 2200 || in.Semester < 1 || in.Semester > 20 || in.OpensAt.IsZero() || !in.ClosesAt.After(in.OpensAt) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите факультет, семестр и окно выбора")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		return studyRead(ctx, tx, `INSERT INTO study_plan_periods ("facultyID","academicYear",semester,"opensAt","closesAt","createdByEmployerID")
   SELECT "facultyID",$3,$4,$5,$6,$1::uuid FROM (`+studyDeanFaculties+`) f WHERE "facultyID"=$2::uuid
   RETURNING to_jsonb(study_plan_periods)`, current(r).EmployerID, in.FacultyID, in.AcademicYear, in.Semester, in.OpensAt, in.ClosesAt)
	})
}

type studyPeriod struct {
	Faculty    string
	Semester   int
	Published  bool
	Start, End time.Time
}

func studyLockPeriod(ctx context.Context, tx pgx.Tx, dean, period string) (studyPeriod, error) {
	var p studyPeriod
	err := tx.QueryRow(ctx, `SELECT p."facultyID"::text,p.semester,p."isPublished",
 study_term_start(p."academicYear",p.semester),study_term_end(p."academicYear",p.semester)
 FROM study_plan_periods p JOIN (`+studyDeanFaculties+`) f USING ("facultyID") WHERE p."periodID"=$2::uuid FOR UPDATE OF p`, dean, period).Scan(&p.Faculty, &p.Semester, &p.Published, &p.Start, &p.End)
	return p, err
}

func (s *Server) studyRequirement(w http.ResponseWriter, r *http.Request) {
	period, ok := academicPathUUID(w, r, "periodID")
	if !ok {
		return
	}
	var in struct {
		CourseID, Title, Kind, DirectionID string
		Hours                              int
	}
	if !decode(w, r, &in) {
		return
	}
	validKind := in.Kind == "common" || in.Kind == "physical_education" || in.Kind == "profile"
	validDirection := (in.Kind == "profile" && resourceUUID(in.DirectionID)) || (in.Kind != "profile" && in.DirectionID == "")
	validCourse := (in.CourseID != "" && resourceUUID(in.CourseID)) || (in.CourseID == "" && studyTitle(in.Title, 128) && in.Hours > 0 && in.Hours <= 10000)
	if !validKind || !validDirection || !validCourse {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте дисциплину, часы и направление")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		p, err := studyLockPeriod(ctx, tx, current(r).EmployerID, period)
		if err != nil {
			return nil, err
		}
		if p.Published {
			return nil, studyConflict("Состав опубликованного плана зафиксирован")
		}
		course := in.CourseID
		if course == "" {
			err = tx.QueryRow(ctx, `INSERT INTO courses ("courseID","facultyID",title,hours,semester)
    VALUES(gen_random_uuid(),$1::uuid,$2,$3,$4) RETURNING "courseID"::text`, p.Faculty, strings.TrimSpace(in.Title), in.Hours, p.Semester).Scan(&course)
			if err != nil {
				return nil, err
			}
		}
		var incompatible bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM study_plan_requirements WHERE "periodID"=$1::uuid AND "courseID"=$2::uuid
    AND (kind='physical_education' OR $3='physical_education' OR kind='common' OR $3='common'))`, period, course, in.Kind).Scan(&incompatible)
		if err != nil {
			return nil, err
		}
		if incompatible {
			return nil, studyConflict("Дисциплина уже включена в план в несовместимой категории")
		}
		return studyRead(ctx, tx, `INSERT INTO study_plan_requirements ("periodID","courseID","directionID",kind)
   SELECT $1::uuid,c."courseID",NULLIF($3,'')::uuid,$4 FROM courses c
   WHERE c."courseID"=$2::uuid AND c."facultyID"=$5::uuid AND c.semester=$6
   AND ($3='' OR EXISTS(SELECT 1 FROM direction d WHERE d."directionID"=NULLIF($3,'')::uuid AND d."facultyID"=c."facultyID"))
   RETURNING to_jsonb(study_plan_requirements)`, period, course, in.DirectionID, in.Kind, p.Faculty, p.Semester)
	})
}

func (s *Server) studyStream(w http.ResponseWriter, r *http.Request) {
	period, ok := academicPathUUID(w, r, "periodID")
	if !ok {
		return
	}
	var in struct {
		CourseID, TeacherID   string
		ClassroomID, Capacity int
		StartAt, EndAt        time.Time
		Weeks                 int
	}
	if !decode(w, r, &in) {
		return
	}
	duration := in.EndAt.Sub(in.StartAt)
	if !studyUUIDs(in.CourseID, in.TeacherID) || in.ClassroomID <= 0 || in.Capacity < 1 || in.Capacity > 10000 ||
		in.Weeks < 1 || in.Weeks > 40 || in.StartAt.IsZero() || duration <= 0 || duration > 24*time.Hour {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте расписание, преподавателя, аудиторию и вместимость")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		p, err := studyLockPeriod(ctx, tx, current(r).EmployerID, period)
		if err != nil {
			return nil, err
		}
		lastEnd := in.EndAt.AddDate(0, 0, 7*(in.Weeks-1))
		if in.StartAt.Before(p.Start) || lastEnd.After(p.End) {
			return nil, studyConflict("Занятия должны проходить в пределах семестра")
		}
		var topic string
		err = tx.QueryRow(ctx, `SELECT c.title FROM courses c WHERE c."courseID"=$1::uuid
   AND EXISTS(SELECT 1 FROM study_plan_requirements req WHERE req."periodID"=$2::uuid AND req."courseID"=c."courseID" AND req.kind<>'physical_education')
   AND EXISTS(SELECT 1 FROM "facultyTeacher" ft JOIN teachers t USING ("teacherID") WHERE ft."facultyID"=$3::uuid AND t."teacherID"=$4::uuid AND `+studyTeacherActive+`)`, in.CourseID, period, p.Faculty, in.TeacherID).Scan(&topic)
		if err != nil {
			return nil, err
		}
		// Serialize timetable changes made by this workflow across faculties.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(214193028)`); err != nil {
			return nil, err
		}
		var collision bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM generate_series(0,$1::int-1) AS week(n)
   JOIN lessons l ON l."startAt"<$3::timestamptz+week.n*interval '168 hours' AND l."endAt">$2::timestamptz+week.n*interval '168 hours'
   WHERE l."lessonStatus"<>'canceled' AND (l."classroomID"=$4 OR l."teacherID"=$5::uuid))`, in.Weeks, in.StartAt, in.EndAt, in.ClassroomID, in.TeacherID).Scan(&collision)
		if err != nil {
			return nil, err
		}
		if collision {
			return nil, studyConflict("Преподаватель или аудитория заняты в это время")
		}
		var group int
		err = tx.QueryRow(ctx, `WITH id AS (SELECT nextval('study_group_id_seq')::int AS n) INSERT INTO groups ("groupID","groupNumber") SELECT n,n FROM id RETURNING "groupID"`).Scan(&group)
		if err != nil {
			return nil, err
		}
		var courseGroup string
		err = tx.QueryRow(ctx, `INSERT INTO "courseGroup" ("courseGroupID","courseID","groupID",start_date,end_date)
   VALUES(gen_random_uuid(),$1::uuid,$2,$3,$4) RETURNING "courseGroupID"::text`, in.CourseID, group, p.Start, p.End).Scan(&courseGroup)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO "courseTeacher" ("courseID","teacherID") VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, in.CourseID, in.TeacherID)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO lessons ("lessonID","groupID","classroomID","teacherID",topic,"startAt","endAt","lessonStatus","courseID")
   SELECT nextval('study_lesson_id_seq'),$1,$2,$3::uuid,$4,$5::timestamptz+n*interval '168 hours',$6::timestamptz+n*interval '168 hours','active',$7::uuid
   FROM generate_series(0,$8::int-1) week(n)`, group, in.ClassroomID, in.TeacherID, topic, in.StartAt, in.EndAt, in.CourseID, in.Weeks)
		if err != nil {
			return nil, err
		}
		return studyRead(ctx, tx, `INSERT INTO course_streams ("periodID","courseID","courseGroupID","teacherID",capacity)
   VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5) RETURNING to_jsonb(course_streams)`, period, in.CourseID, courseGroup, in.TeacherID, in.Capacity)
	})
}

func (s *Server) studyPublish(w http.ResponseWriter, r *http.Request) {
	period, ok := academicPathUUID(w, r, "periodID")
	if !ok {
		return
	}
	var in struct{ Action string }
	if !decode(w, r, &in) {
		return
	}
	if in.Action != "publish" && in.Action != "close" {
		fail(w, 422, "VALIDATION_ERROR", "Укажите действие")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		p, err := studyLockPeriod(ctx, tx, current(r).EmployerID, period)
		if err != nil {
			return nil, err
		}
		if in.Action == "close" {
			if !p.Published {
				return nil, studyConflict("Сначала опубликуйте план")
			}
			return studyRead(ctx, tx, `UPDATE study_plan_periods SET "closesAt"=least("closesAt",greatest(clock_timestamp(),"opensAt"+interval '1 microsecond'))
    WHERE "periodID"=$1::uuid RETURNING to_jsonb(study_plan_periods)`, period)
		}
		var missing bool
		err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM study_plan_requirements req WHERE req."periodID"=$1::uuid)
   OR EXISTS(SELECT 1 FROM study_plan_requirements req WHERE req."periodID"=$1::uuid AND req.kind<>'physical_education'
   AND NOT EXISTS(SELECT 1 FROM course_streams cs WHERE cs."periodID"=req."periodID" AND cs."courseID"=req."courseID" AND cs."isOpen"
   AND EXISTS(SELECT 1 `+studyStreamLessons+`)))`, period).Scan(&missing)
		if err != nil {
			return nil, err
		}
		if missing {
			return nil, studyConflict("Добавьте дисциплины и хотя бы один поток для каждой дисциплины с выбором времени")
		}
		return studyRead(ctx, tx, `UPDATE study_plan_periods SET "isPublished"=true WHERE "periodID"=$1::uuid RETURNING to_jsonb(study_plan_periods)`, period)
	})
}
