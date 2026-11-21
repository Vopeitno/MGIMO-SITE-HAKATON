package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type sportSlot struct {
	Weekday, ClassroomID, Capacity int
	TeacherID, StartTime, EndTime  string
}

func (s *Server) sportSemesterList(w http.ResponseWriter, r *http.Request) {
	faculty := r.URL.Query().Get("facultyID")
	if !resourceUUID(faculty) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите факультет")
		return
	}
	s.academicRead(w, r, `WITH faculty AS (`+studyDeanFaculties+`)
     SELECT jsonb_build_object('periods',$3::text::jsonb,'titles',COALESCE((SELECT jsonb_agg(t.title ORDER BY t.title) FROM (SELECT DISTINCT "sportTitle" title FROM sports) t),'[]'),
     'plans',COALESCE((SELECT jsonb_agg(jsonb_build_object('title',p.title,'academicYear',p."academicYear",'term',p.term,
       'slots',COALESCE((SELECT jsonb_agg(jsonb_build_object(
        'weekday',sl.weekday,'startTime',to_char(sl."startTime",'HH24:MI'),'endTime',to_char(sl."endTime",'HH24:MI'),
        'classroomID',sl."classroomID",'teacherID',sl."teacherID",'capacity',sl.capacity)
        ORDER BY sl.weekday,sl."startTime",sl."slotID") FROM sport_weekly_slots sl WHERE sl."scheduleID"=p."scheduleID"),'[]'))
        ORDER BY p."academicYear" DESC,p.term DESC,p.title) FROM sport_semester_schedules p WHERE p."facultyID"=f."facultyID"),'[]'))
     FROM faculty f WHERE f."facultyID"=$2::uuid`, []any{current(r).EmployerID, faculty, sportPeriodsJSON})
}

func (s *Server) sportSemesterCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FacultyID, Title   string
		AcademicYear, Term int
		Slots              []sportSlot
	}
	if !decode(w, r, &in) {
		return
	}
	in.Title = strings.Join(strings.Fields(in.Title), " ")
	if !resourceUUID(in.FacultyID) || !studyTitle(in.Title, 128) || in.AcademicYear < 2000 || in.AcademicYear > 2199 ||
		(in.Term != 1 && in.Term != 2) || len(in.Slots) == 0 || len(in.Slots) > 42 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите вид спорта, семестр и занятия на неделе")
		return
	}
	dayCounts := [8]int{}
	for _, slot := range in.Slots {
		if !validSportPeriod(slot.StartTime, slot.EndTime) || slot.Weekday < 1 || slot.Weekday > 7 || slot.ClassroomID < 1 ||
			!resourceUUID(slot.TeacherID) || slot.Capacity < 1 || slot.Capacity > 10000 {
			fail(w, 422, "VALIDATION_ERROR", "Выберите время пары, зал, преподавателя и число мест")
			return
		}
		dayCounts[slot.Weekday]++
		if dayCounts[slot.Weekday] > 6 {
			fail(w, 422, "VALIDATION_ERROR", "На один день можно поставить не больше шести занятий этого вида спорта")
			return
		}
	}
	dean := current(r).EmployerID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(214193028),pg_advisory_xact_lock(214193029)`); err != nil {
			return nil, err
		}
		var schedule string
		err := tx.QueryRow(ctx, `INSERT INTO sport_semester_schedules ("facultyID",title,title_key,"academicYear",term)
        SELECT "facultyID",$3,$4,$5,$6 FROM (`+studyDeanFaculties+`) f WHERE "facultyID"=$2::uuid
        RETURNING "scheduleID"::text`, dean, in.FacultyID, in.Title, strings.ToLower(in.Title), in.AcademicYear, in.Term).Scan(&schedule)
		if err != nil {
			return nil, err
		}
		total := 0
		for _, slot := range in.Slots {
			published, err := publishSportSlot(ctx, tx, schedule, dean, in.FacultyID, in.Title, in.AcademicYear, in.Term, slot)
			if err != nil {
				return nil, err
			}
			total += published
		}
		return map[string]any{"scheduleID": schedule, "sessions": total}, nil
	})
}

const sportSlotTimes = `WITH term AS (
    SELECT (study_term_start($1::int,$2::int) AT TIME ZONE 'Europe/Moscow')::date first_day,
           (study_term_end($1::int,$2::int) AT TIME ZONE 'Europe/Moscow')::date last_day
), dates AS (
    SELECT first_day+i AS day FROM term CROSS JOIN LATERAL generate_series(0,last_day-first_day-1) g(i)
    WHERE extract(isodow FROM first_day+i)=$3::int
), times AS (
    SELECT (day+$4::text::time) AT TIME ZONE 'Europe/Moscow' starts,
           (day+$5::text::time) AT TIME ZONE 'Europe/Moscow' ends FROM dates
), upcoming AS (SELECT starts,ends FROM times WHERE starts>now()) `

func publishSportSlot(ctx context.Context, tx pgx.Tx, schedule, dean, faculty, title string, year, term int, s sportSlot) (int, error) {
	var building string
	var number int
	err := tx.QueryRow(ctx, `SELECT cl."buildingID"::text,cl."classroomNumber" FROM classroom cl
     WHERE cl."classroomID"=$1 AND EXISTS(SELECT 1 FROM "facultyTeacher" ft JOIN teachers t USING ("teacherID")
      WHERE ft."facultyID"=$2::uuid AND t."teacherID"=$3::uuid AND `+studyTeacherActive+`)`, s.ClassroomID, faculty, s.TeacherID).Scan(&building, &number)
	if err != nil {
		return 0, err
	}
	room := fmt.Sprintf("Аудитория %d", number)
	roomKey := strings.ToLower(room)
	params := []any{year, term, s.Weekday, s.StartTime, s.EndTime}
	var clash string
	err = tx.QueryRow(ctx, sportSlotTimes+`
     SELECT COALESCE((SELECT to_char(min(u.starts) AT TIME ZONE 'Europe/Moscow','DD.MM.YYYY HH24:MI') FROM upcoming u
       WHERE EXISTS(SELECT 1 FROM sports sp JOIN sport_session_rooms sr USING ("sportID")
            WHERE sp."buildingID"=$6::uuid AND sr.room_key=$7 AND sp."sportStartAt"<u.ends AND sp."sportEndAt">u.starts)
       OR EXISTS(SELECT 1 FROM lessons l WHERE l."classroomID"=$8 AND l."lessonStatus"<>'canceled'
            AND l."startAt"<u.ends AND l."endAt">u.starts)
       OR EXISTS(SELECT 1 FROM exams e WHERE e."classroomID"=$8 AND e."examStatus"='scheduled'
            AND e."examStartAt"<u.ends AND e."examEndAt">u.starts)),'')`, append(params, building, roomKey, s.ClassroomID)...).Scan(&clash)
	if err != nil {
		return 0, err
	}
	if clash != "" {
		return 0, studyConflict(fmt.Sprintf("%s занята %s МСК", room, clash))
	}
	var slot string
	err = tx.QueryRow(ctx, `INSERT INTO sport_weekly_slots ("scheduleID",weekday,"startTime","endTime","classroomID","teacherID",capacity)
        VALUES($1::uuid,$2,$3::text::time,$4::text::time,$5,$6::uuid,$7) RETURNING "slotID"::text`,
		schedule, s.Weekday, s.StartTime, s.EndTime, s.ClassroomID, s.TeacherID, s.Capacity).Scan(&slot)
	if err != nil {
		return 0, err
	}
	var count int
	err = tx.QueryRow(ctx, sportSlotTimes+`, inserted AS (
      INSERT INTO sports ("sportID","sportTitle","buildingID","sportCapacity","sportStartAt","sportEndAt")
      SELECT gen_random_uuid(),$6,$7::uuid,$8,starts,ends FROM upcoming RETURNING "sportID"
    ), instructors AS (
      INSERT INTO sport_instructors ("sportID","teacherID","assignedByEmployerID")
      SELECT "sportID",$9::uuid,$10::uuid FROM inserted
    ), rooms AS (
      INSERT INTO sport_session_rooms ("sportID",room,room_key) SELECT "sportID",$11,$12 FROM inserted
    ), occurrences AS (
      INSERT INTO sport_schedule_occurrences ("sportID","slotID") SELECT "sportID",$13::uuid FROM inserted RETURNING 1
    ) SELECT count(*) FROM occurrences`, append(params, title, building, s.Capacity, s.TeacherID, dean, room, roomKey, slot)...).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, studyConflict("В выбранном семестре не осталось дат для указанных дней")
	}
	return count, nil
}
