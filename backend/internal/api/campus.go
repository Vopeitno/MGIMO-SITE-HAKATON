package api

import (
	"net/http"
	"time"
)

var campusRoutes = []studyRoute{
	{"GET", "/campus/rooms", "STUDENT|TEACHER|DEAN|SUPPORT", (*Server).campusRooms},
}

func (s *Server) campusRooms(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	from, errFrom := time.Parse(time.RFC3339, query.Get("from"))
	to, errTo := time.Parse(time.RFC3339, query.Get("to"))
	building := query.Get("buildingID")
	if errFrom != nil || errTo != nil || len(query["from"]) != 1 || len(query["to"]) != 1 ||
		!to.After(from) || to.Sub(from) > 24*time.Hour ||
		(building != "" && !resourceUUID(building)) || len(query["buildingID"]) > 1 {
		fail(w, 422, "VALIDATION_ERROR", "Выберите корпус и интервал длительностью до 24 часов")
		return
	}
	s.academicRead(w, r, `WITH availability AS (
		SELECT room."classroomID",room."classroomNumber",b."buildingID",b.building,
		    NOT (EXISTS(SELECT 1 FROM lessons l WHERE l."classroomID"=room."classroomID"
		          AND l."lessonStatus"<>'canceled' AND l."startAt"<$2 AND l."endAt">$1)
		    OR EXISTS(SELECT 1 FROM exams e WHERE e."classroomID"=room."classroomID"
		          AND e."examStatus"<>'canceled' AND e."examStartAt"<$2 AND e."examEndAt">$1)
		    OR EXISTS(SELECT 1 FROM sport_schedule_occurrences o JOIN sport_weekly_slots slot USING ("slotID")
		          JOIN sports sp USING ("sportID") WHERE slot."classroomID"=room."classroomID"
		          AND sp."sportStartAt"<$2 AND sp."sportEndAt">$1)
		    OR EXISTS(SELECT 1 FROM sport_session_rooms sr JOIN sports sp USING ("sportID")
		          WHERE sp."buildingID"=room."buildingID" AND sr.room_key='аудитория '||room."classroomNumber"
		          AND sp."sportStartAt"<$2 AND sp."sportEndAt">$1)) AS available
		FROM classroom room JOIN buildings b USING ("buildingID")
		WHERE NULLIF($3,'')::uuid IS NULL OR b."buildingID"=NULLIF($3,'')::uuid
	)
	SELECT jsonb_build_object('items',COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.building,a."classroomNumber",a."classroomID"),'[]'),
	    'available',count(*) FILTER (WHERE available),'total',count(*)) FROM availability a`, []any{from, to, building})
}
