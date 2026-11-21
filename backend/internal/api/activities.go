package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var activityRoutes = []studyRoute{
	{"GET", "/activities", "STUDENT", (*Server).studentActivities},
	{"PUT", "/activities/{eventID}/registration", "STUDENT", (*Server).activityRegistration},
	{"GET", "/activities/events", "DEAN", (*Server).activityManagement},
	{"POST", "/activities/events", "DEAN", (*Server).activityPublish},
	{"PUT", "/activities/events/{eventID}/host", "DEAN", (*Server).activityAssignHost},
	{"GET", "/activities/{eventID}/ticket", "STUDENT", (*Server).activityTicketQR},
	{"GET", "/activities/{eventID}/ticket.pdf", "STUDENT", (*Server).activityTicketPDF},
	{"GET", "/activities/check-in", "TEACHER|SUPPORT", (*Server).activityCheckInList},
	{"GET", "/activities/events/{eventID}/registrations", "SUPPORT", (*Server).activityRoster},
	{"PUT", "/activities/events/{eventID}/registrations/{studentID}", "SUPPORT", (*Server).activityAttendance},
	{"POST", "/activities/events/{eventID}/check-in", "TEACHER", (*Server).activityScan},
}

const eventJSON = `jsonb_build_object('eventID',e."eventID",'title',e.title,'category',e.category,
 'description',e.description,'startAt',e."startAt",'endAt',e."endAt",'building',b.building,
 'remaining',greatest(0,e.capacity-(SELECT count(*) FROM campus_event_registrations r
   WHERE r."eventID"=e."eventID" AND r."canceledAt" IS NULL)))`

func (s *Server) studentActivities(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH personal AS (
       SELECT r."eventID",r."attendedAt",e.category,e."startAt",e."endAt"
       FROM campus_event_registrations r JOIN campus_events e USING ("eventID")
       WHERE r."studentID"=$1::uuid AND r."canceledAt" IS NULL
    ), upcoming AS (
        SELECT `+eventJSON+`||jsonb_build_object('registered',p."eventID" IS NOT NULL) item,e."startAt" starts
        FROM campus_events e JOIN buildings b USING ("buildingID") LEFT JOIN personal p USING ("eventID") WHERE e."startAt">now()
        UNION ALL
        SELECT `+studySportJSON+`||jsonb_build_object('eventID',sp."sportID",'category','sport','description',sp."sportDescription",
         'remaining',greatest(0,sp."sportCapacity"-(SELECT count(*) FROM sport_registrations rr
             WHERE rr."sportID"=sp."sportID" AND rr."registrationStatus"='enrolled')),
         'registered',EXISTS(SELECT 1 FROM sport_registrations rr WHERE rr."sportID"=sp."sportID" AND rr."studentID"=$1::uuid AND rr."registrationStatus"='enrolled')),
         sp."sportStartAt" FROM sports sp JOIN buildings b USING ("buildingID") WHERE `+calendarUpcoming+`
    )
    SELECT jsonb_build_object(
      'events',COALESCE((SELECT jsonb_agg(item ORDER BY starts,item->>'eventID') FROM
          (SELECT item,starts FROM upcoming ORDER BY starts,item->>'eventID' LIMIT 250) page),'[]'),
       'myTickets',COALESCE((SELECT jsonb_agg(`+eventJSON+`||jsonb_build_object('registered',true,'attended',p."attendedAt" IS NOT NULL)
         ORDER BY (e."endAt">=now()) DESC,e."startAt" DESC)
         FROM personal p JOIN campus_events e USING ("eventID") JOIN buildings b USING ("buildingID")),'[]'),
      'weekCount',(SELECT count(*) FROM upcoming WHERE starts<now()+interval '7 days'),
      'visits',(SELECT count(*) FROM personal WHERE "attendedAt" IS NOT NULL)+
               (SELECT count(*) FROM sport_attendance WHERE "studentID"=$1::uuid AND present),
      'volunteerMinutes',COALESCE((SELECT sum(extract(epoch FROM ("endAt"-"startAt"))/60)
            FROM personal WHERE "attendedAt" IS NOT NULL AND category='volunteering'),0),
      'registrations',(SELECT count(*) FROM upcoming WHERE item->>'registered'='true'),
      'interests',COALESCE((SELECT jsonb_agg(category ORDER BY category) FROM (SELECT DISTINCT category FROM personal) a),'[]'))`, []any{current(r).StudentID})
}

func (s *Server) activityRegistration(w http.ResponseWriter, r *http.Request) {
	event, ok := academicPathUUID(w, r, "eventID")
	if !ok {
		return
	}
	var in struct{ Registered *bool }
	if !decode(w, r, &in) {
		return
	}
	if in.Registered == nil {
		fail(w, 422, "VALIDATION_ERROR", "Укажите действие с записью")
		return
	}
	student := current(r).StudentID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var open bool
		var cap int
		err := tx.QueryRow(ctx, `SELECT "startAt">clock_timestamp(),capacity FROM campus_events WHERE "eventID"=$1::uuid FOR UPDATE`, event).Scan(&open, &cap)
		if err != nil {
			return nil, err
		}
		if !open {
			return nil, studyConflict("Мероприятие уже началось")
		}
		if !*in.Registered {
			_, err = tx.Exec(ctx, `UPDATE campus_event_registrations SET "canceledAt"=now()
              WHERE "eventID"=$1::uuid AND "studentID"=$2::uuid AND "canceledAt" IS NULL AND "attendedAt" IS NULL`, event, student)
			return map[string]bool{"registered": false}, err
		}
		var allowed, hasSeat bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM students WHERE "studentID"=$2::uuid AND "studentStatus"='active'),
          (SELECT count(*) FROM campus_event_registrations WHERE "eventID"=$1::uuid AND "studentID"<>$2::uuid AND "canceledAt" IS NULL)<$3`, event, student, cap).Scan(&allowed, &hasSeat)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, studyConflict("Запись доступна действующим студентам")
		}
		if !hasSeat {
			return nil, studyConflict("Свободных мест нет")
		}
		_, err = tx.Exec(ctx, `INSERT INTO campus_event_registrations ("eventID","studentID") VALUES($1::uuid,$2::uuid)
          ON CONFLICT ("eventID","studentID") DO UPDATE SET "canceledAt"=NULL,"registeredAt"=now(),"ticketID"=gen_random_uuid()
          WHERE campus_event_registrations."canceledAt" IS NOT NULL`, event, student)
		return map[string]bool{"registered": true}, err
	})
}

func (s *Server) activityManagement(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH faculty AS (`+studyDeanFaculties+`)
    SELECT jsonb_build_object('faculties',COALESCE((SELECT jsonb_agg(to_jsonb(f) ORDER BY title) FROM faculty f),'[]'),
       'buildings',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY building) FROM buildings b),'[]'),
        'teachers',COALESCE((SELECT jsonb_agg(jsonb_build_object('facultyID',ft."facultyID",'teacherID',t."teacherID",'name',t."teacherName") ORDER BY t."teacherName")
         FROM "facultyTeacher" ft JOIN faculty USING ("facultyID") JOIN teachers t USING ("teacherID") WHERE `+studyTeacherActive+`),'[]'),
       'events',COALESCE((SELECT jsonb_agg(`+eventJSON+`||jsonb_build_object('facultyID',e."facultyID",'teacherID',h."teacherID") ORDER BY e."startAt" DESC) FROM campus_events e
          JOIN buildings b USING ("buildingID") JOIN faculty USING ("facultyID") LEFT JOIN campus_event_hosts h USING ("eventID")),'[]'))`, []any{current(r).EmployerID})
}

func (s *Server) activityPublish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FacultyID, BuildingID, Title, Description, Category, TeacherID string
		StartAt, EndAt                                                 time.Time
		Capacity                                                       int
	}
	if !decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if !studyUUIDs(in.FacultyID, in.BuildingID, in.TeacherID) || !studyTitle(in.Title, 128) || len([]rune(in.Description)) > 2000 ||
		(in.Category != "community" && in.Category != "career" && in.Category != "volunteering") ||
		!in.StartAt.After(time.Now()) || !in.EndAt.After(in.StartAt) || in.Capacity < 1 || in.Capacity > 10000 {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте название, категорию, место, время и вместимость")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		return studyRead(ctx, tx, `WITH created AS (
          INSERT INTO campus_events ("facultyID","createdByEmployerID","buildingID",title,description,category,"startAt","endAt",capacity)
          SELECT "facultyID",$1::uuid,$3::uuid,$4,$5,$6,$7,$8,$9 FROM (`+studyDeanFaculties+`) f
          WHERE f."facultyID"=$2::uuid AND `+activityHostEligible+`
          RETURNING "eventID"
        ) INSERT INTO campus_event_hosts ("eventID","teacherID")
          SELECT "eventID",$10::uuid FROM created RETURNING jsonb_build_object('eventID',"eventID")`,
			current(r).EmployerID, in.FacultyID, in.BuildingID, in.Title, in.Description, in.Category, in.StartAt, in.EndAt, in.Capacity, in.TeacherID)
	})
}

func (s *Server) activityRoster(w http.ResponseWriter, r *http.Request) {
	event, ok := academicPathUUID(w, r, "eventID")
	if !ok {
		return
	}
	s.academicRead(w, r, `SELECT jsonb_build_object('started',e."startAt"<=now(),'students',COALESCE(
        (SELECT jsonb_agg(jsonb_build_object('studentID',st."studentID",'name',st."studentName",
          'number',st."studentNumber",'present',r."attendedAt" IS NOT NULL) ORDER BY st."studentName",st."studentID")
         FROM campus_event_registrations r JOIN students st USING ("studentID")
         WHERE r."eventID"=e."eventID" AND r."canceledAt" IS NULL),'[]'))
      FROM campus_events e WHERE e."eventID"=$1::uuid`, []any{event})
}

func (s *Server) activityAttendance(w http.ResponseWriter, r *http.Request) {
	event, ok := academicPathUUID(w, r, "eventID")
	if !ok {
		return
	}
	student, ok := academicPathUUID(w, r, "studentID")
	if !ok {
		return
	}
	var in struct{ Present *bool }
	if !decode(w, r, &in) {
		return
	}
	if in.Present == nil {
		fail(w, 422, "VALIDATION_ERROR", "Укажите присутствие")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var started bool
		err := tx.QueryRow(ctx, `SELECT e."startAt"<=now() FROM campus_events e WHERE e."eventID"=$1::uuid FOR SHARE OF e`, event).Scan(&started)
		if err != nil {
			return nil, err
		}
		if !started {
			return nil, studyConflict("Событие ещё не началось")
		}
		return studyRead(ctx, tx, `UPDATE campus_event_registrations SET "attendedAt"=CASE WHEN $3
          THEN COALESCE("attendedAt",now()) ELSE NULL END,
          "checkedByAccountID"=CASE WHEN $3 THEN COALESCE("checkedByAccountID",$4::uuid) END,
          "checkMethod"=CASE WHEN $3 THEN COALESCE("checkMethod",'support') END
          WHERE "eventID"=$1::uuid AND "studentID"=$2::uuid AND "canceledAt" IS NULL
          RETURNING jsonb_build_object('present',"attendedAt" IS NOT NULL)`, event, student, *in.Present, current(r).AccountID)
	})
}
