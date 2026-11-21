package api

import (
	"context"
	"net/http"
)

type messagePerson struct {
	AccountID   string `json:"accountID"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

const messagePersonSQL = `SELECT a."accountID"::text,COALESCE(st."studentName",t."teacherName",e."employerName",'Участник'),a."accountRole"::text FROM public.accounts a LEFT JOIN public.students st ON st."studentID"=a."studentID" LEFT JOIN public.teachers t ON t."teacherID"=a."teacherID" LEFT JOIN public.employers e ON e."employerID"=a."employerID" `

func (s *Server) messagePeople(ctx context.Context, query string, args ...any) ([]messagePerson, error) {
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := []messagePerson{}
	for rows.Next() {
		var p messagePerson
		if err = rows.Scan(&p.AccountID, &p.DisplayName, &p.Role); err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	return people, rows.Err()
}
func (s *Server) deaneryContacts(w http.ResponseWriter, r *http.Request) {
	p := resourceGuard(w, r, "STUDENT")
	if p == nil {
		return
	}
	if s.DB == nil {
		messagesUnavailable(w)
		return
	}
	if r.URL.RawQuery != "" {
		fail(w, 400, "BAD_REQUEST", "Параметры не поддерживаются")
		return
	}
	people, err := s.messagePeople(r.Context(), messagePersonSQL+`WHERE a."isActive" AND a."accountRole"='DEAN' AND e."employerStatus"='active' AND EXISTS(SELECT 1 FROM public.students own JOIN public.faculties f ON f."facultyID"=own."facultyID" JOIN public.deanery_staff ds ON ds."deaneryID"=f."deaneryID" WHERE own."studentID"=NULLIF($1,'')::uuid AND ds."employerID"=a."employerID") ORDER BY e."employerName",a."accountID"`, p.StudentID)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 200, map[string]any{"items": people})
}
func (s *Server) conversationParticipants(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	conversation, ok := s.messagesConversation(w, r, ctx, account)
	if !ok {
		return
	}
	people, err := s.messagePeople(ctx, messagePersonSQL+`WHERE a."accountID"::text=ANY($1::text[]) AND a."isActive" ORDER BY a."accountID"`, conversation.Participants)
	if err != nil {
		messagesUnavailable(w)
		return
	}
	write(w, 200, map[string]any{"items": people})
}
