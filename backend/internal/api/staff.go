package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var staffRoutes = []studyRoute{
	{"GET", "/staff/teacher/groups", "TEACHER", (*Server).staffTeacherGroups},
	{"GET", "/staff/teacher/roster", "TEACHER", (*Server).staffTeacherRoster},
	{"GET", "/staff/dean/requests", "DEAN", (*Server).staffDeanRequests},
	{"GET", "/staff/dean/reports", "DEAN", (*Server).staffDeanReports},
	{"GET", "/staff/support/faculties", "SUPPORT", (*Server).staffSupportFaculties},
	{"GET", "/staff/support/colleagues", "SUPPORT", (*Server).staffSupportColleagues},
	{"GET", "/staff/support/analytics", "SUPPORT", (*Server).staffSupportAnalytics},
	{"GET", "/staff/directories", "ADMIN|SUPPORT", (*Server).staffDirectories},
	{"GET", "/admin/accounts", "ADMIN", (*Server).staffAccounts},
	{"GET", "/admin/persons", "ADMIN", (*Server).staffPersons},
	{"GET", "/admin/audit", "ADMIN", (*Server).staffAudit},
	{"GET", "/admin/overview", "ADMIN", (*Server).staffAdminOverview},
	{"POST", "/admin/buildings", "ADMIN", (*Server).staffBuildingSave},
	{"PUT", "/admin/buildings/{id}", "ADMIN", (*Server).staffBuildingSave},
	{"POST", "/admin/classrooms", "ADMIN", (*Server).staffClassroomSave},
	{"PUT", "/admin/classrooms/{id}", "ADMIN", (*Server).staffClassroomSave},
}

// A group for a common course uses the course's teachers. A selected stream has its own assigned teacher.
const staffTeachingGroup = `(EXISTS (SELECT 1 FROM course_streams stream WHERE stream."courseGroupID"=cg."courseGroupID" AND stream."teacherID"=$1)
 OR (NOT EXISTS (SELECT 1 FROM course_streams stream WHERE stream."courseGroupID"=cg."courseGroupID")
 AND EXISTS (SELECT 1 FROM "courseTeacher" ct WHERE ct."courseID"=cg."courseID" AND ct."teacherID"=$1)))`

func (s *Server) staffTeacherGroups(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT cg."courseGroupID",cg."courseID",c.title,g."groupID",g."groupNumber",
	c.semester,cg.start_date AS "startsAt",cg.end_date AS "endsAt",
	(SELECT count(*) FROM students st JOIN student_course_teacher_access a USING ("studentID")
	 WHERE a."courseID"=cg."courseID" AND a."teacherID"=$1 AND
	   (st."groupID"=cg."groupID" OR EXISTS (SELECT 1 FROM student_stream_selections sel
	      JOIN course_streams sm USING ("streamID") WHERE sel."studentID"=st."studentID" AND sm."courseGroupID"=cg."courseGroupID"))) AS "studentCount"
	FROM "courseGroup" cg JOIN groups g USING ("groupID") JOIN courses c USING ("courseID")
	WHERE `+staffTeachingGroup, `"groupNumber",semester,title,"courseGroupID"`, current(r).TeacherID)
}

func (s *Server) staffTeacherRoster(w http.ResponseWriter, r *http.Request) {
	courseID := r.URL.Query().Get("courseID")
	if !resourceUUID(courseID) {
		fail(w, 400, "BAD_REQUEST", "Выберите дисциплину")
		return
	}
	s.portalPage(w, r, `SELECT DISTINCT st."studentID",st."studentNumber",st."studentName",g."groupNumber",
		cs."courseGraduate",cs.score,cs.credit,COALESCE(cs."teacherID"=$1,true) AS editable
		FROM student_course_teacher_access a JOIN students st USING ("studentID") JOIN groups g USING ("groupID")
		LEFT JOIN effective_course_scores cs ON cs."studentID"=st."studentID" AND cs."courseID"=a."courseID"
		WHERE a."teacherID"=$1 AND a."courseID"=$2`, `"groupNumber","studentName","studentID"`, current(r).TeacherID, courseID)
}

func (s *Server) staffDeanRequests(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT a."appoinmentID",a."appointmentTitle",a."appointmentDescription",a."appointmentStatus",a.created_at,
		st."studentName",st."studentNumber" FROM appointment a JOIN students st USING ("studentID")
		WHERE EXISTS (SELECT 1 FROM (`+studyDeanFaculties+`) permitted WHERE permitted."facultyID"=st."facultyID")`,
		`created_at DESC,"appoinmentID"`, current(r).EmployerID)
}

func (s *Server) staffDeanReports(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH faculties_in_scope AS (`+studyDeanFaculties+`),
	student_scope AS (SELECT st.* FROM students st JOIN faculties_in_scope f USING ("facultyID")),
	contingent AS (SELECT f.title,COUNT(st."studentID") AS total,
		COUNT(st."studentID") FILTER (WHERE st."studentStatus"='active') AS active
		FROM faculties_in_scope f LEFT JOIN student_scope st USING ("facultyID") GROUP BY f."facultyID",f.title),
	debts AS (SELECT st."studentName",st."studentNumber",c.title,cs.score
		FROM student_scope st JOIN "courseScore" cs USING ("studentID") JOIN courses c USING ("courseID")
		WHERE cs.credit IS FALSE),
	visits AS (SELECT st."studentName",st."studentNumber",COUNT(a."attendanceID") AS visits
		FROM student_scope st JOIN sport_attendance a USING ("studentID") WHERE a.present
		GROUP BY st."studentID",st."studentName",st."studentNumber")
	SELECT jsonb_build_object('contingent',COALESCE((SELECT jsonb_agg(to_jsonb(c) ORDER BY title) FROM contingent c),'[]'),
		'debts',COALESCE((SELECT jsonb_agg(to_jsonb(d) ORDER BY "studentNumber", title) FROM debts d),'[]'),
		'attendance',COALESCE((SELECT jsonb_agg(to_jsonb(v) ORDER BY "studentNumber") FROM visits v),'[]'))`, []any{current(r).EmployerID})
}

func (s *Server) staffSupportFaculties(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w,r,`SELECT "facultyID",title FROM faculties`,`title,"facultyID"`)
}

func (s *Server) staffSupportColleagues(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w,r,`SELECT e."employerID",e."employerName" FROM employers e JOIN accounts a USING ("employerID")
		WHERE e."employerStatus"='active' AND a."accountRole"='SUPPORT' AND a."isActive" AND e."employerID"<>$1`,
		`"employerName","employerID"`,current(r).EmployerID)
}

func (s *Server) staffSupportAnalytics(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `WITH facts AS (SELECT t.id,t.priority,t.closed_at,t.created_at,
		(SELECT min(msg.created_at) FROM support_ticket_messages msg WHERE msg.ticket_id=t.id AND NOT msg.from_student) AS first_reply
		FROM support_tickets t)
	SELECT jsonb_build_object('open',count(*) FILTER(WHERE closed_at IS NULL),
		'urgent',count(*) FILTER(WHERE closed_at IS NULL AND priority='urgent'),
		'closed',count(*) FILTER(WHERE closed_at IS NOT NULL),
		'averageFirstReplyMinutes',round(avg(extract(epoch FROM first_reply-created_at)/60),1))
	FROM facts`, nil)
}

func (s *Server) staffDirectories(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `SELECT jsonb_build_object(
	'buildings',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY b.building,b."buildingID") FROM buildings b),'[]'),
	'classrooms',COALESCE((SELECT jsonb_agg(to_jsonb(c) ORDER BY c."buildingID",c."classroomNumber",c."classroomID") FROM classroom c),'[]'),
	'teachers',COALESCE((SELECT jsonb_agg(jsonb_build_object('teacherID',t."teacherID",'teacherName',t."teacherName") ORDER BY t."teacherName")
		FROM teachers t WHERE t."teacherStatus"='active'),'[]'))`, nil)
}

func (s *Server) staffAccounts(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT a."accountID",a.email,a."accountRole"::text AS "accountRole",a."isActive",a."createdAt",
		COALESCE(st."studentName",t."teacherName",e."employerName",a.email) AS "displayName"
		FROM accounts a LEFT JOIN students st USING ("studentID") LEFT JOIN teachers t USING ("teacherID")
		LEFT JOIN employers e USING ("employerID")`, `"displayName","accountID"`)
}

func (s *Server) staffPersons(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT st."studentID" AS id,st."studentName" AS name,st."studentNumber" AS number,'studentID' AS "personKey"
		FROM students st WHERE st."studentStatus"='active' AND NOT EXISTS(SELECT 1 FROM accounts a WHERE a."studentID"=st."studentID")
		UNION ALL SELECT t."teacherID",t."teacherName",t."teacherNumber",'teacherID' FROM teachers t
		WHERE t."teacherStatus"='active' AND NOT EXISTS(SELECT 1 FROM accounts a WHERE a."teacherID"=t."teacherID")
		UNION ALL SELECT e."employerID",e."employerName",e."employerNumber",'employerID' FROM employers e
		WHERE e."employerStatus"='active' AND NOT EXISTS(SELECT 1 FROM accounts a WHERE a."employerID"=e."employerID")`, `name,id`)
}

func (s *Server) staffAudit(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT log.id,a.email AS "actorEmail",log.target_email AS "targetEmail",log.action,log.previous_role AS "previousRole",
		log.next_role AS "nextRole",log.previous_active AS "previousActive",log.next_active AS "nextActive",log.changed_at AS "changedAt"
		FROM staff_account_audit log LEFT JOIN accounts a ON a."accountID"=log.actor_id`, `"changedAt" DESC,id`)
}

func (s *Server) staffAdminOverview(w http.ResponseWriter, r *http.Request) {
	var counts json.RawMessage
	err := s.DB.QueryRow(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY role),'[]') FROM (
		SELECT "accountRole"::text AS role,count(*) AS total,count(*) FILTER (WHERE "isActive") AS active
		FROM accounts GROUP BY "accountRole"
	) x`).Scan(&counts)
	if err != nil {
		resourceError(w, err)
		return
	}
	permissions := map[string][]string{}
	for _, rule := range contractRules {
		for _, role := range rule.roles {
			if role != "PUBLIC" {
				permissions[role] = append(permissions[role], rule.method+" "+strings.TrimPrefix(rule.path, prefix))
			}
		}
	}
	for _, route := range studyAndSettingsRoutes {
		for _, role := range strings.Split(route.role, "|") {
			permissions[role] = append(permissions[role], route.method+" "+route.path)
		}
	}
	write(w, 200, map[string]any{"accountCounts": counts, "permissions": permissions, "mongoConnected": s.Mongo != nil,
		"secureCookies": s.Config.SecureCookies})
}

func (s *Server) staffBuildingSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Building string `json:"building"`
		Address  string `json:"address"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Address = strings.TrimSpace(body.Address)
	if !serviceText(&body.Building, 256) || utf8.RuneCountInString(body.Address) > 256 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название корпуса и адрес длиной до 256 символов")
		return
	}
	ctx := r.Context()
	id := uuid.NewString()
	if r.Method == "POST" {
		_, err := s.DB.Exec(ctx, `INSERT INTO buildings("buildingID",building,"buildingAddress") VALUES($1,$2,NULLIF($3,''))`, id, body.Building, body.Address)
		if err != nil {
			resourceError(w, err)
			return
		}
		write(w, 201, map[string]string{"id": id})
		return
	}
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(ctx, `UPDATE buildings SET building=$2,"buildingAddress"=NULLIF($3,'') WHERE "buildingID"=$1`, id, body.Building, body.Address)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Корпус не найден")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) staffClassroomSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BuildingID string `json:"buildingID"`
		Number     int32  `json:"number"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !resourceUUID(body.BuildingID) || body.Number < 1 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите корпус и положительный номер аудитории")
		return
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		resourceError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `LOCK TABLE classroom IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		resourceError(w, err)
		return
	}
	var id int64
	if r.Method == "PUT" {
		id, err = strconv.ParseInt(r.PathValue("id"), 10, 32)
		if err != nil || id < 1 {
			fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор аудитории")
			return
		}
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM classroom WHERE "buildingID"=$1 AND "classroomNumber"=$2 AND "classroomID"<>$3)`, body.BuildingID, body.Number, id).Scan(&exists)
	if err != nil {
		resourceError(w, err)
		return
	}
	if exists {
		fail(w, 409, "CONFLICT", "В корпусе уже есть аудитория с этим номером")
		return
	}
	if r.Method == "POST" {
		err = tx.QueryRow(ctx, `INSERT INTO classroom("classroomID","buildingID","classroomNumber")
			SELECT COALESCE(max("classroomID"),0)+1,$1,$2 FROM classroom RETURNING "classroomID"`, body.BuildingID, body.Number).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, `UPDATE classroom SET "buildingID"=$2,"classroomNumber"=$3 WHERE "classroomID"=$1 RETURNING "classroomID"`, id, body.BuildingID, body.Number).Scan(&id)
	}
	if err != nil {
		resourceError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		resourceError(w, err)
		return
	}
	if r.Method == "POST" {
		write(w, 201, map[string]int64{"id": id})
	} else {
		write(w, 204, nil)
	}
}
