package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// Only fields in messagePersonSQL are exposed. Existing conversations remain
// private irrespective of whether the peer is still in the contact directory.
const chatContactScope = `a."isActive" AND a."accountID"<>$5::uuid AND (
  ($1='STUDENT' AND (
    (a."accountRole"='STUDENT' AND st."studentStatus"='active' AND EXISTS(
      SELECT 1 FROM students own WHERE own."studentID"=NULLIF($2,'')::uuid AND own."groupID"=st."groupID"))
    OR (a."accountRole"='TEACHER' AND t."teacherStatus"='active' AND EXISTS(
      SELECT 1 FROM student_course_teacher_access x WHERE x."studentID"=NULLIF($2,'')::uuid AND x."teacherID"=t."teacherID"))
    OR (a."accountRole"='DEAN' AND e."employerStatus"='active' AND EXISTS(
      SELECT 1 FROM students own JOIN faculties f USING ("facultyID") JOIN deanery_staff ds USING ("deaneryID")
      WHERE own."studentID"=NULLIF($2,'')::uuid AND ds."employerID"=e."employerID"))
    OR (a."accountRole"='SUPPORT' AND e."employerStatus"='active')
  ))
  OR ($1='TEACHER' AND a."accountRole"='STUDENT' AND st."studentStatus"='active' AND EXISTS(
    SELECT 1 FROM student_course_teacher_access x WHERE x."teacherID"=NULLIF($3,'')::uuid AND x."studentID"=st."studentID"))
  OR ($1='DEAN' AND a."accountRole"='STUDENT' AND st."studentStatus"='active' AND EXISTS(
    SELECT 1 FROM faculties f JOIN deanery_staff ds USING ("deaneryID")
    WHERE f."facultyID"=st."facultyID" AND ds."employerID"=NULLIF($4,'')::uuid))
  OR ($1='SUPPORT' AND a."accountRole" IN ('STUDENT','TEACHER','DEAN') AND EXISTS(
    SELECT 1 FROM appointment req WHERE req."employerID"=NULLIF($4,'')::uuid AND
      (req."studentID"=a."studentID" OR req."teacherID"=a."teacherID" OR req."createdByEmployerID"=a."employerID")))
 )`

const chatPersonNumber = `COALESCE(st."studentNumber",t."teacherNumber",e."employerNumber")`
const chatPersonEmployed = `((st."studentID" IS NOT NULL AND st."studentStatus"<>'expelled')
 OR (t."teacherID" IS NOT NULL AND t."teacherStatus"<>'fired')
 OR (e."employerID" IS NOT NULL AND e."employerStatus"<>'fired'))`

func chatContactArgs(p *Principal) []any {
	return append(academicArgs(p), p.AccountID)
}

func (s *Server) chatContacts(w http.ResponseWriter, r *http.Request) {
	p := resourceGuard(w, r, "STUDENT|TEACHER|DEAN|SUPPORT")
	if p == nil {
		return
	}
	limit, offset, err := messagesPagination(r)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная пагинация")
		return
	}
	var number int64
	if values, present := r.URL.Query()["number"]; present {
		var parseErr error
		if len(values) == 1 {
			number, parseErr = strconv.ParseInt(values[0], 10, 32)
		}
		if len(values) != 1 || parseErr != nil || number < 1 {
			fail(w, 400, "BAD_REQUEST", "Укажите полный номер студента или сотрудника")
			return
		}
	}
	s.academicRead(w, r, `WITH contacts("accountID","displayName",role) AS (`+messagePersonSQL+` WHERE
      ($8::int=0 AND `+chatContactScope+`) OR ($8::int>0 AND a."isActive" AND a."accountID"<>$5::uuid
           AND `+chatPersonEmployed+` AND `+chatPersonNumber+`=$8::int)),
      page AS (SELECT * FROM contacts ORDER BY "displayName","accountID" LIMIT $6 OFFSET $7)
      SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p."displayName",p."accountID") FROM page p),'[]'),
       'total',(SELECT count(*) FROM contacts),'limit',$6::int,'offset',$7::int)`, append(chatContactArgs(p), limit, offset, number))
}

func (s *Server) eligibleChatContacts(ctx context.Context, p *Principal, ids []string, numbers map[string]int32) (bool, error) {
	raw, err := json.Marshal(numbers)
	if err != nil {
		return false, err
	}
	people, err := s.messagePeople(ctx, messagePersonSQL+` WHERE a."accountID"=ANY($6::text[]::uuid[])
      AND ((`+chatContactScope+`) OR (a."isActive" AND a."accountID"<>$5::uuid AND `+chatPersonEmployed+`
      AND ($7::text::jsonb ->> a."accountID"::text)=(`+chatPersonNumber+`)::text))`, append(chatContactArgs(p), ids, string(raw))...)
	return len(people) == len(ids), err
}

const academicChats = `SELECT g."groupID",g."groupNumber"
 FROM groups g WHERE
 ($1='STUDENT' AND EXISTS(SELECT 1 FROM students st WHERE st."studentID"=NULLIF($2,'')::uuid
  AND st."studentStatus"='active' AND (st."groupID"=g."groupID" OR EXISTS(
    SELECT 1 FROM student_stream_selections sel JOIN course_streams stream USING ("streamID")
    JOIN "courseGroup" cg ON cg."courseGroupID"=stream."courseGroupID"
    WHERE sel."studentID"=st."studentID" AND cg."groupID"=g."groupID" AND cg.end_date>now()
  ))))
 OR ($1='TEACHER' AND EXISTS(SELECT 1 FROM teachers t JOIN lessons l USING ("teacherID")
  WHERE t."teacherID"=NULLIF($3,'')::uuid AND t."teacherStatus"='active' AND l."groupID"=g."groupID"
  AND l."lessonStatus"<>'canceled' AND l."startAt">=(date_trunc('year',(now() AT TIME ZONE 'Europe/Moscow')-interval '8 months')+interval '8 months') AT TIME ZONE 'Europe/Moscow'))`

type academicChat struct{ ID, Number int }

func (s *Server) academicChatList(ctx context.Context, p *Principal) ([]academicChat, error) {
	rows, err := s.DB.Query(ctx, academicChats+` ORDER BY g."groupNumber",g."groupID"`, p.Role, p.StudentID, p.TeacherID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (academicChat, error) {
		var item academicChat
		err := row.Scan(&item.ID, &item.Number)
		return item, err
	})
}
