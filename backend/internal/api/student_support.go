package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

var studentSupportRoutes = []studyRoute{
	{"GET", "/support/articles", "STUDENT|SUPPORT", (*Server).supportArticles},
	{"POST", "/support/articles", "SUPPORT", (*Server).supportArticleSave},
	{"PUT", "/support/articles/{id}", "SUPPORT", (*Server).supportArticleSave},
	{"DELETE", "/support/articles/{id}", "SUPPORT", (*Server).supportArticleArchive},
	{"GET", "/support/tickets", "STUDENT|SUPPORT", (*Server).supportTickets},
	{"POST", "/support/tickets", "STUDENT", (*Server).supportTicketCreate},
	{"GET", "/support/tickets/{id}", "STUDENT|SUPPORT", (*Server).supportTicketRead},
	{"POST", "/support/tickets/{id}/messages", "STUDENT|SUPPORT", (*Server).supportTicketReply},
	{"GET", "/support/operators", "STUDENT", (*Server).supportOperators},
}

func (s *Server) supportArticles(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT id, question, answer, updated_at AS "updatedAt"
		FROM support_articles WHERE archived_at IS NULL`, `"updatedAt" DESC, id`)
}

func (s *Server) supportArticleSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !serviceText(&body.Question, 180) || !serviceText(&body.Answer, 5000) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите вопрос и ответ")
		return
	}
	ctx := r.Context()
	if r.Method == http.MethodPost {
		id := uuid.NewString()
		_, err := s.DB.Exec(ctx, `INSERT INTO support_articles(id,question,answer) VALUES($1,$2,$3)`, id, body.Question, body.Answer)
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
	tag, err := s.DB.Exec(ctx, `UPDATE support_articles SET question=$2,answer=$3,updated_at=now()
		WHERE id=$1 AND archived_at IS NULL`, id, body.Question, body.Answer)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Статья не найдена")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) supportArticleArchive(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(r.Context(), `UPDATE support_articles SET archived_at=COALESCE(archived_at,now()) WHERE id=$1`, id)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Статья не найдена")
	} else {
		write(w, 204, nil)
	}
}

const supportTicketFields = `t.id, t.category, t.priority, t.created_at AS "createdAt",
 t.updated_at AS "updatedAt", t.closed_at AS "closedAt",
 (assigned."accountID" IS NOT NULL) AS assigned,
 COALESCE(assigned."accountID"=$3::uuid,false) AS "assignedToMe",
 CASE WHEN $1::boolean THEN st."studentName" END AS "studentName",
 CASE WHEN $1::boolean THEN st."studentNumber" END AS "studentNumber"`

const supportTicketSource = `FROM support_tickets t JOIN students st ON t.student_id=st."studentID"
 LEFT JOIN accounts assigned ON assigned."accountID"=t.assigned_to AND assigned."isActive" AND assigned."accountRole"='SUPPORT'
 WHERE ($1::boolean OR t.student_id=NULLIF($2,'')::uuid)`

func supportTicketArgs(p *Principal) []any {
	return []any{p.Role == "SUPPORT", p.StudentID, p.AccountID}
}

func (s *Server) supportTickets(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, "SELECT "+supportTicketFields+" "+supportTicketSource,
		`"closedAt" NULLS FIRST, priority DESC, "updatedAt" DESC, id`, supportTicketArgs(current(r))...)
}

func (s *Server) supportTicketRead(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	var data json.RawMessage
	err := s.DB.QueryRow(r.Context(), `SELECT to_jsonb(ticket) || jsonb_build_object('messages',(
		SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'fromStudent',from_student,'message',message,
			'createdAt',created_at) ORDER BY created_at,id),'[]'::jsonb)
		FROM support_ticket_messages WHERE ticket_id=ticket.id))
		FROM (SELECT `+supportTicketFields+" "+supportTicketSource+` AND t.id=$4) ticket`,
		append(supportTicketArgs(current(r)), id)...).Scan(&data)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, data)
}

func (s *Server) supportTicketCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Category string `json:"category"`
		Priority string `json:"priority"`
		Message  string `json:"message"`
	}
	if !decode(w, r, &body) {
		return
	}
	switch body.Category {
	case "study", "schedule", "technical", "dormitory", "other", "consultation":
	default:
		fail(w, 422, "VALIDATION_ERROR", "Выберите тему обращения")
		return
	}
	if (body.Priority != "normal" && body.Priority != "urgent") || !serviceText(&body.Message, 5000) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите приоритет и текст обращения")
		return
	}
	id := uuid.NewString()
	_, err := s.DB.Exec(r.Context(), `WITH created AS (
		INSERT INTO support_tickets(id,student_id,category,priority) VALUES($1,$2,$3,$4) RETURNING id
	)
	INSERT INTO support_ticket_messages(id,ticket_id,from_student,message) SELECT $5,id,true,$6 FROM created`,
		id, current(r).StudentID, body.Category, body.Priority, uuid.NewString(), body.Message)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 201, map[string]string{"id": id})
}

func (s *Server) supportTicketReply(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	var body struct {
		Message string `json:"message"`
		Close   bool   `json:"close"`
	}
	if !decode(w, r, &body) {
		return
	}
	p := current(r)
	staff := p.Role == "SUPPORT"
	if (body.Close && !staff) || !serviceText(&body.Message, 5000) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите текст сообщения и допустимое действие")
		return
	}
	// The update and message insert share a statement, so a losing concurrent
	// operator cannot append a reply or close the winning operator's ticket.
	tag, err := s.DB.Exec(r.Context(), `WITH changed AS (
		UPDATE support_tickets t SET updated_at=clock_timestamp(),
			closed_at=CASE WHEN $6 THEN clock_timestamp() END,
			assigned_to=CASE WHEN $2::boolean THEN $3::uuid ELSE assigned_to END
		WHERE t.id=$1 AND closed_at IS NULL AND (
			(NOT $2 AND student_id=NULLIF($4,'')::uuid) OR
			($2 AND (assigned_to=$3 OR NOT EXISTS (
				SELECT 1 FROM accounts a WHERE a."accountID"=t.assigned_to AND a."accountRole"='SUPPORT' AND a."isActive"
			)))
		) RETURNING id
	)
	INSERT INTO support_ticket_messages(id,ticket_id,from_student,message) SELECT $5,id,NOT $2,$7 FROM changed`,
		id, staff, p.AccountID, p.StudentID, uuid.NewString(), body.Close, body.Message)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Обращение закрыто или недоступно для ответа")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) supportOperators(w http.ResponseWriter, r *http.Request) {
	people, err := s.messagePeople(r.Context(), messagePersonSQL+` WHERE a."isActive" AND
		a."accountRole"='SUPPORT' AND e."employerStatus"='active' ORDER BY e."employerName",a."accountID"`)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, map[string]any{"items": people})
}
