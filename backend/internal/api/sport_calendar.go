package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const calendarInstructor = `EXISTS(SELECT 1 FROM sport_instructors i JOIN teachers t USING ("teacherID")
 WHERE i."sportID"=sp."sportID" AND ` + studyTeacherActive + `)`

const calendarUpcoming = `sp."sportStartAt">now() AND sp."buildingID" IS NOT NULL AND ` + calendarInstructor

const calendarWeek = `sp."sportStartAt">=(week.monday::timestamp AT TIME ZONE 'Europe/Moscow')
 AND sp."sportStartAt"<((week.monday+7)::timestamp AT TIME ZONE 'Europe/Moscow')`

func calendarParseWeek(value string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", value)
	return t, err == nil && t.Weekday() == time.Monday && t.Year() >= 2000 && t.Year() <= 2200
}

func (s *Server) sportCalendar(w http.ResponseWriter, r *http.Request) {
	week := r.URL.Query().Get("week")
	if week != "" {
		if _, ok := calendarParseWeek(week); !ok {
			fail(w, 422, "VALIDATION_ERROR", "Укажите понедельник выбранной недели")
			return
		}
	}
	s.academicRead(w, r, `WITH week AS (
        SELECT COALESCE(NULLIF($2,'')::date,
            (SELECT date_trunc('week',min(sp."sportStartAt") AT TIME ZONE 'Europe/Moscow')::date
             FROM sports sp WHERE `+calendarUpcoming+`),
            date_trunc('week',now() AT TIME ZONE 'Europe/Moscow')::date) monday
    ), catalog AS (
        SELECT DISTINCT sp."buildingID",b.building,sp."sportTitle" title
        FROM sports sp JOIN buildings b USING ("buildingID") WHERE `+calendarUpcoming+`
    )
    SELECT jsonb_build_object('week',week.monday,
         'periods',$3::text::jsonb,
        'used',(SELECT count(*) FROM (
           SELECT a."sportID" FROM sport_attendance a WHERE a."studentID"=$1::uuid AND a.present AND a."weekStart"=week.monday
           UNION SELECT sp."sportID" FROM sports sp JOIN sport_registrations r USING ("sportID")
            WHERE r."studentID"=$1::uuid AND r."registrationStatus"='enrolled'
            AND sp."sportStartAt"<=now() AND `+calendarWeek+`) used),
        'defaultBuildingID' ,(SELECT sp."buildingID" FROM sports sp WHERE `+calendarUpcoming+` ORDER BY sp."sportStartAt",sp."sportID" LIMIT 1),
        'buildings',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY b.building,b."buildingID") FROM (SELECT DISTINCT "buildingID",building FROM catalog) b),'[]'),
        'types',COALESCE((SELECT jsonb_agg(title ORDER BY title) FROM (SELECT DISTINCT title FROM catalog) t),'[]'),
        'sessions',COALESCE((SELECT jsonb_agg(`+studySportJSON+`||jsonb_build_object(
             'teacher',COALESCE((SELECT string_agg(t."teacherName",', ' ORDER BY t."teacherName")
              FROM sport_instructors i JOIN teachers t USING ("teacherID") WHERE i."sportID"=sp."sportID"),''),
            'buildingID',sp."buildingID",'room',room.room,'reservable',`+calendarUpcoming+`,
            'remaining',greatest(0,sp."sportCapacity"-(SELECT count(*) FROM sport_registrations x WHERE x."sportID"=sp."sportID" AND x."registrationStatus"='enrolled')),
            'cancellable',sp."sportStartAt">now(),'reserved',own."sportID" IS NOT NULL
        ) ORDER BY sp."sportStartAt",sp."sportTitle",sp."sportID")
        FROM sports sp JOIN buildings b USING ("buildingID") LEFT JOIN sport_session_rooms room USING ("sportID")
        LEFT JOIN sport_registrations own ON own."sportID"=sp."sportID" AND own."studentID"=$1::uuid AND own."registrationStatus"='enrolled'
        WHERE `+calendarWeek+` AND (`+calendarInstructor+` OR own."sportID" IS NOT NULL)), '[]'))
    FROM week`, []any{current(r).StudentID, week, sportPeriodsJSON})
}

func (s *Server) sportCalendarSave(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Week     string
		SportIDs []string
	}
	if !decode(w, r, &in) {
		return
	}
	monday, ok := calendarParseWeek(in.Week)
	if !ok || in.SportIDs == nil || len(in.SportIDs) > 2 || !studyUUIDs(in.SportIDs...) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите неделю и выберите не больше двух занятий")
		return
	}
	desired := make(map[string]bool, len(in.SportIDs))
	for _, id := range in.SportIDs {
		desired[strings.ToLower(id)] = true
	}
	if len(desired) != len(in.SportIDs) {
		fail(w, 422, "VALIDATION_ERROR", "Уберите повторяющиеся занятия")
		return
	}
	student := current(r).StudentID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		// Legacy single-session reservations use the same student → sport lock order.
		var locked int
		err := tx.QueryRow(ctx, `SELECT 1 FROM students WHERE "studentID"=$1::uuid AND "studentStatus"='active' FOR UPDATE`, student).Scan(&locked)
		if err != nil {
			return nil, err
		}
		var used int
		err = tx.QueryRow(ctx, sportWeekLoad+` AND sp."sportStartAt"<=now() ) used`, student, in.Week).Scan(&used)
		if err != nil {
			return nil, err
		}
		if len(in.SportIDs) > 0 && used+len(in.SportIDs) > 2 {
			return nil, studyConflict("На одной неделе можно выбрать не больше двух занятий")
		}

		rows, err := tx.Query(ctx, `WITH week AS (SELECT $2::text::date monday)
          SELECT sp."sportID"::text FROM sports sp CROSS JOIN week
          WHERE `+calendarWeek+` AND sp."sportStartAt">now() AND sp."buildingID" IS NOT NULL AND (
              sp."sportID"=ANY($3::text[]::uuid[]) OR EXISTS(SELECT 1 FROM sport_registrations x
              WHERE x."studentID"=$1::uuid AND x."sportID"=sp."sportID" AND x."registrationStatus"='enrolled'))
          ORDER BY sp."sportID" FOR UPDATE OF sp`, student, in.Week, in.SportIDs)
		if err != nil {
			return nil, err
		}
		lockedIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		var valid, available, overlap bool
		err = tx.QueryRow(ctx, `WITH chosen AS (
             SELECT sp.* FROM sports sp WHERE sp."sportID"=ANY($2::text[]::uuid[])
               AND sp."sportID"=ANY($3::text[]::uuid[]) AND (`+calendarUpcoming+` OR EXISTS(
               SELECT 1 FROM sport_registrations own WHERE own."sportID"=sp."sportID"
               AND own."studentID"=$1::uuid AND own."registrationStatus"='enrolled'))
          )
          SELECT (SELECT count(*) FROM chosen)=cardinality($2::text[]),
           NOT EXISTS(SELECT 1 FROM chosen c WHERE c."sportCapacity"<=(SELECT count(*) FROM sport_registrations x
             WHERE x."sportID"=c."sportID" AND x."studentID"<>$1::uuid AND x."registrationStatus"='enrolled')),
           EXISTS(SELECT 1 FROM chosen a JOIN chosen b ON a."sportID"<b."sportID"
             AND a."sportStartAt"<b."sportEndAt" AND b."sportStartAt"<a."sportEndAt")`, student, in.SportIDs, lockedIDs).Scan(&valid, &available, &overlap)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, studyConflict("Одно из занятий недоступно для записи")
		}
		if !available {
			return nil, studyConflict("Свободные места закончились. Обновите календарь")
		}
		if overlap {
			return nil, studyConflict("Выбранные занятия физкультурой пересекаются")
		}
		if _, err = tx.Exec(ctx, `UPDATE sport_registrations SET "registrationStatus"='canceled',"updatedAt"=now()
             WHERE "studentID"=$1::uuid AND "sportID"=ANY($2::text[]::uuid[]) AND "registrationStatus"='enrolled'
              AND NOT ("sportID"=ANY($3::text[]::uuid[]))`, student, lockedIDs, in.SportIDs); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO sport_registrations ("sportRegistrationID","sportID","studentID","registrationStatus","createdAt")
          SELECT gen_random_uuid(),id::uuid,$1::uuid,'enrolled',now() FROM unnest($2::text[]) ids(id)
          ON CONFLICT ("sportID","studentID") DO UPDATE SET "registrationStatus"='enrolled',"updatedAt"=now()
          WHERE sport_registrations."registrationStatus"<>'enrolled'`, student, in.SportIDs)
		sort.Strings(in.SportIDs)
		return map[string]any{"week": monday.Format("2006-01-02"), "sportIDs": in.SportIDs}, err
	})
}
