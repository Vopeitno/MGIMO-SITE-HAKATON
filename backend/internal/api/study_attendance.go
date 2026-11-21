package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const studySportJSON = `jsonb_build_object('sportID',sp."sportID",'title',sp."sportTitle",'startAt',sp."sportStartAt",'endAt',sp."sportEndAt",'building',b.building)`
const studySportInstructor = `EXISTS(SELECT 1 FROM sport_instructors i JOIN teachers t USING ("teacherID")
 WHERE i."sportID"=sp."sportID" AND t."teacherID"=$1::uuid AND ` + studyTeacherActive + `)`

func (s *Server) studyPhysical(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH st AS (`+studyStudentTerm+`),
 term AS (SELECT greatest(1,semester) semester,study_term_start(year,CASE WHEN extract(month FROM now() AT TIME ZONE 'Europe/Moscow') BETWEEN 2 AND 8 THEN 2 ELSE 1 END) starts FROM st),
 visits AS (SELECT a."attendedAt",a."weekStart",sp."sportTitle" title FROM sport_attendance a JOIN sports sp USING ("sportID")
 WHERE a."studentID"=$1::uuid AND a.present AND a."attendedAt">=(SELECT starts FROM term) AND a."attendedAt"<=now())
 SELECT jsonb_build_object('semester',(SELECT semester FROM term),'points',count(*)*5,'credit',count(*)>=20,
 'weeklyVisits',(SELECT count(*) FROM sport_attendance a WHERE a."studentID"=$1::uuid AND a.present AND a."weekStart"=date_trunc('week',now() AT TIME ZONE 'Europe/Moscow')::date),
 'visits',COALESCE(jsonb_agg(to_jsonb(visits) ORDER BY visits."attendedAt" DESC),'[]'),
 'sessions',COALESCE((SELECT jsonb_agg(item ORDER BY item->>'startAt') FROM (
 SELECT `+studySportJSON+` item FROM sports sp LEFT JOIN buildings b USING ("buildingID")
 WHERE sp."sportStartAt">=now() AND EXISTS(SELECT 1 FROM sport_instructors i JOIN teachers t USING ("teacherID")
 WHERE i."sportID"=sp."sportID" AND `+studyTeacherActive+`) ORDER BY sp."sportStartAt",sp."sportID" LIMIT 50) upcoming),'[]'))
 FROM visits`, []any{current(r).StudentID})
}

func (s *Server) studyCreateSport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FacultyID, TeacherID, BuildingID, Title, Room string
		Capacity                                      int
		StartAt, EndAt                                time.Time
	}
	if !decode(w, r, &in) {
		return
	}
	in.Room = strings.Join(strings.Fields(in.Room), " ")
	roomKey := strings.ToLower(in.Room)
	if len([]rune(in.Room)) > 100 || !studyUUIDs(in.FacultyID, in.TeacherID, in.BuildingID) || !studyTitle(in.Title, 128) || in.Capacity < 1 || in.Capacity > 10000 || in.StartAt.IsZero() || !in.EndAt.After(in.StartAt) {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте название, место, преподавателя и время занятия")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var sport string
		dean := current(r).EmployerID
		if in.Room != "" {
			// Serialize availability checks while allowing different halls at the same time.
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(214193029)`); err != nil {
				return nil, err
			}
			var occupied bool
			err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sports sp JOIN sport_session_rooms sr USING ("sportID")
                WHERE sp."buildingID"=$1::uuid AND sr.room_key=$2
                AND sp."sportStartAt"<$4 AND sp."sportEndAt">$3)`, in.BuildingID, roomKey, in.StartAt, in.EndAt).Scan(&occupied)
			if err != nil {
				return nil, err
			}
			if occupied {
				return nil, studyConflict("Зал занят в это время")
			}
		}
		err := tx.QueryRow(ctx, `INSERT INTO sports ("sportID","sportTitle","buildingID","sportCapacity","sportStartAt","sportEndAt")
   SELECT gen_random_uuid(),$4,$5::uuid,$6,$7,$8 FROM (`+studyDeanFaculties+`) f
   JOIN "facultyTeacher" ft USING ("facultyID") JOIN teachers t USING ("teacherID")
   WHERE f."facultyID"=$2::uuid AND t."teacherID"=$3::uuid AND `+studyTeacherActive+`
   RETURNING "sportID"::text`, dean, in.FacultyID, in.TeacherID, strings.TrimSpace(in.Title), in.BuildingID, in.Capacity, in.StartAt, in.EndAt).Scan(&sport)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO sport_instructors ("sportID","teacherID","assignedByEmployerID") VALUES($1::uuid,$2::uuid,$3::uuid)`, sport, in.TeacherID, dean)
		if err == nil && in.Room != "" {
			_, err = tx.Exec(ctx, `INSERT INTO sport_session_rooms ("sportID",room,room_key) VALUES($1::uuid,$2,$3)`, sport, in.Room, roomKey)
		}
		return map[string]string{"sportID": sport}, err
	})
}

func (s *Server) studySessions(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.academicPage(w, r, p, `SELECT `+studySportJSON+` obj,sp."sportStartAt" sort_at,sp."sportID" sort_id FROM sports sp LEFT JOIN buildings b USING ("buildingID")
 WHERE $1='TEACHER' AND $2::text IS NOT NULL AND $4::text IS NOT NULL AND EXISTS(SELECT 1 FROM sport_instructors i JOIN teachers t USING ("teacherID")
 WHERE i."sportID"=sp."sportID" AND t."teacherID"=$3::uuid AND `+studyTeacherActive+`)`, `sort_at DESC,sort_id`)
}

func (s *Server) studyRoster(w http.ResponseWriter, r *http.Request) {
	sport, ok := academicPathUUID(w, r, "sportID")
	if !ok {
		return
	}
	needle := strings.TrimSpace(r.URL.Query().Get("search"))
	if len([]rune(needle)) > 100 {
		fail(w, 422, "VALIDATION_ERROR", "Сократите строку поиска")
		return
	}
	s.academicRead(w, r, `SELECT jsonb_build_object('started',sp."sportStartAt"<=now(),'students',COALESCE((SELECT jsonb_agg(item ORDER BY item->>'name') FROM (
 SELECT jsonb_build_object('studentID',st."studentID",'name',st."studentName",'number',st."studentNumber",'present',COALESCE(a.present,false)) item
 FROM students st JOIN accounts ac USING ("studentID") LEFT JOIN sport_attendance a ON a."sportID"=sp."sportID" AND a."studentID"=st."studentID"
 WHERE st."studentStatus"='active' AND ac."isActive" AND
 CASE WHEN $3='' THEN COALESCE(a.present,false) ELSE position(lower($3) IN lower(st."studentName"))>0 OR st."studentNumber"::text=$3 END
 ORDER BY st."studentName",st."studentID" LIMIT 50) matches),'[]'))
 FROM sports sp WHERE sp."sportID"=$2::uuid AND `+studySportInstructor, []any{current(r).TeacherID, sport, needle})
}

func (s *Server) studyMark(w http.ResponseWriter, r *http.Request) {
	sport, ok := academicPathUUID(w, r, "sportID")
	if !ok {
		return
	}
	var in struct {
		StudentID string
		Present   *bool
	}
	if !decode(w, r, &in) {
		return
	}
	if !resourceUUID(in.StudentID) || in.Present == nil {
		fail(w, 422, "VALIDATION_ERROR", "Укажите студента и отметку посещения")
		return
	}
	teacher := current(r).TeacherID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var started time.Time
		var canMark bool
		err := tx.QueryRow(ctx, `SELECT sp."sportStartAt",sp."sportStartAt"<=now()
   FROM sports sp JOIN sport_instructors i USING ("sportID") JOIN teachers t USING ("teacherID")
   WHERE sp."sportID"=$2::uuid AND t."teacherID"=$1::uuid AND `+studyTeacherActive+` FOR SHARE OF sp,i,t`, teacher, sport).Scan(&started, &canMark)
		if err != nil {
			return nil, err
		}
		if !canMark {
			return nil, studyConflict("Занятие ещё не началось")
		}
		var locked int
		err = tx.QueryRow(ctx, `SELECT 1 FROM students st JOIN accounts ac USING ("studentID")
  WHERE st."studentID"=$1::uuid AND st."studentStatus"='active' AND ac."isActive" FOR UPDATE OF st`, in.StudentID).Scan(&locked)
		if err != nil {
			return nil, err
		}
		var existing bool
		var attended time.Time
		err = tx.QueryRow(ctx, `SELECT COALESCE(a.present,false),COALESCE(a."attendedAt",$3::timestamptz) FROM (SELECT 1) v
  LEFT JOIN sport_attendance a ON a."sportID"=$1::uuid AND a."studentID"=$2::uuid`, sport, in.StudentID, started).Scan(&existing, &attended)
		if err != nil {
			return nil, err
		}
		result := map[string]bool{"present": *in.Present}
		if existing == *in.Present {
			return result, nil
		}
		slot := 1
		if *in.Present {
			err = tx.QueryRow(ctx, `SELECT COALESCE(min(n),0) FROM generate_series(1,2) s(n) WHERE NOT EXISTS(
    SELECT 1 FROM sport_attendance a WHERE a."studentID"=$1::uuid AND a.present AND a."weeklySlot"=n
    AND a."weekStart"=date_trunc('week',$2::timestamptz AT TIME ZONE 'Europe/Moscow')::date)`, in.StudentID, attended).Scan(&slot)
			if err != nil {
				return nil, err
			}
			if slot == 0 {
				return nil, studyConflict("У студента уже два подтверждённых посещения за эту неделю")
			}
		}
		_, err = tx.Exec(ctx, `WITH marked AS (
   INSERT INTO sport_attendance ("studentID","sportID","teacherID",present,"attendedAt","weeklySlot")
   VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6)
   ON CONFLICT ("studentID","sportID") DO UPDATE SET "teacherID"=excluded."teacherID",present=excluded.present,
   "weeklySlot"=excluded."weeklySlot","updatedAt"=now() RETURNING "attendanceID")
   INSERT INTO sport_attendance_history ("attendanceID","teacherID",present) SELECT "attendanceID",$3::uuid,$4 FROM marked`,
			in.StudentID, sport, teacher, *in.Present, attended, slot)
		return result, err
	})
}
