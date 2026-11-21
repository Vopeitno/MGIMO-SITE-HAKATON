package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"mgimo/backend/internal/password"
)

var studentSettingsRoutes = []studyRoute{
	{"GET", "/profile/me/settings/notifications", "STUDENT", (*Server).settingsNotifications},
	{"PUT", "/profile/me/settings/notifications", "STUDENT", (*Server).settingsSaveNotifications},
	{"GET", "/profile/me/settings/sessions", "STUDENT", (*Server).settingsSessions},
	{"DELETE", "/profile/me/settings/sessions/{sessionID}", "STUDENT", (*Server).settingsEndSession},
	{"PUT", "/profile/me/settings/password", "STUDENT", (*Server).settingsPassword},
	{"GET", "/profile/me/settings/documents", "STUDENT", (*Server).settingsDocuments},
	{"PUT", "/profile/me/settings/documents/{documentID}", "STUDENT", (*Server).settingsConsent},
}

const studentNotificationVisibility = ` AND NOT EXISTS(SELECT 1 FROM student_notification_preferences pref WHERE pref."studentID"=NULLIF($2,'')::uuid AND pref.category=r."notificationCategory")`

func (s *Server) settingsNotifications(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `SELECT jsonb_build_object('muted',COALESCE(jsonb_agg(category ORDER BY category),'[]'::jsonb))
 FROM student_notification_preferences WHERE "studentID"=$1::uuid`, []any{current(r).StudentID})
}

func (s *Server) settingsSaveNotifications(w http.ResponseWriter, r *http.Request) {
	var in struct{ Muted []string }
	if !decode(w, r, &in) {
		return
	}
	if in.Muted == nil || len(in.Muted) > 7 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите список категорий")
		return
	}
	student := current(r).StudentID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var locked int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM students WHERE "studentID"=$1::uuid FOR UPDATE`, student).Scan(&locked); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM student_notification_preferences WHERE "studentID"=$1::uuid`, student); err != nil {
			return nil, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO student_notification_preferences ("studentID",category)
   SELECT DISTINCT $1::uuid,kind::notification_category FROM unnest($2::text[]) AS kinds(kind)`, student, in.Muted)
		return map[string][]string{"muted": in.Muted}, err
	})
}

func (s *Server) settingsSessions(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.academicRead(w, r, `SELECT COALESCE(jsonb_agg(jsonb_build_object(
  'sessionID',"sessionID",'createdAt',"createdAt",'expiresAt',"expiresAt",'current',"sessionID"=$2::uuid) ORDER BY ("sessionID"=$2::uuid) DESC,"createdAt" DESC),'[]')
 FROM user_sessions WHERE "accountID"=$1::uuid AND "revokedAt" IS NULL AND "expiresAt">now()`, []any{p.AccountID, p.SessionID})
}

func (s *Server) settingsEndSession(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	session, ok := academicPathUUID(w, r, "sessionID")
	if !ok {
		return
	}
	if session == p.SessionID {
		fail(w, 409, "CONFLICT", "Для текущего сеанса используйте выход из аккаунта")
		return
	}
	result, err := s.DB.Exec(r.Context(), `UPDATE user_sessions SET "revokedAt"=COALESCE("revokedAt",now()) WHERE "accountID"=$1::uuid AND "sessionID"=$2::uuid`, p.AccountID, session)
	if err != nil {
		resourceError(w, err)
		return
	}
	if result.RowsAffected() == 0 {
		resourceError(w, pgx.ErrNoRows)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) settingsPassword(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	var in struct{ CurrentPassword, NewPassword string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.CurrentPassword) == 0 || len(in.CurrentPassword) > 1024 || len([]rune(in.NewPassword)) < 12 || len(in.NewPassword) > 1024 || in.CurrentPassword == in.NewPassword {
		fail(w, 422, "VALIDATION_ERROR", "Новый пароль должен отличаться и содержать не менее 12 символов")
		return
	}
	var previous string
	if err := s.DB.QueryRow(r.Context(), `SELECT "passwordHash" FROM accounts WHERE "accountID"=$1::uuid`, p.AccountID).Scan(&previous); err != nil {
		resourceError(w, err)
		return
	}
	if !password.Verify(in.CurrentPassword, previous) {
		fail(w, 422, "VALIDATION_ERROR", "Текущий пароль неверен")
		return
	}
	hash, err := password.Hash(in.NewPassword)
	if err != nil {
		resourceError(w, err)
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		result, err := tx.Exec(ctx, `UPDATE accounts SET "passwordHash"=$2 WHERE "accountID"=$1::uuid AND "passwordHash"=$3
   AND EXISTS(SELECT 1 FROM user_sessions us WHERE us."sessionID"=$4::uuid AND us."accountID"=accounts."accountID" AND us."revokedAt" IS NULL AND us."expiresAt">now())`,
			p.AccountID, hash, previous, p.SessionID)
		if err != nil {
			return nil, err
		}
		if result.RowsAffected() != 1 {
			return nil, studyConflict("Пароль или сеанс изменился. Повторите вход")
		}
		_, err = tx.Exec(ctx, `UPDATE user_sessions SET "revokedAt"=now() WHERE "accountID"=$1::uuid AND "sessionID"<>$2::uuid AND "revokedAt" IS NULL`, p.AccountID, p.SessionID)
		return map[string]bool{"saved": true}, err
	})
}

func (s *Server) settingsDocuments(w http.ResponseWriter, r *http.Request) {
	s.academicRead(w, r, `SELECT COALESCE(jsonb_agg(jsonb_build_object(
 'documentID',d."documentID",'title',d.title,'version',d.version,'body',d.body,'publishedAt',d."publishedAt",
 'requiresConsent',d."requiresConsent",'accepted',choice.accepted,'changedAt',choice."changedAt") ORDER BY d."publishedAt" DESC,d.title),'[]')
 FROM student_documents d LEFT JOIN LATERAL (
   SELECT accepted,"changedAt" FROM student_document_choices WHERE "studentID"=$1::uuid AND "documentID"=d."documentID" ORDER BY "changedAt" DESC,"choiceID" DESC LIMIT 1
 ) choice ON true WHERE d."publishedAt"<=now()`, []any{current(r).StudentID})
}

func (s *Server) settingsConsent(w http.ResponseWriter, r *http.Request) {
	doc, ok := academicPathUUID(w, r, "documentID")
	if !ok {
		return
	}
	var in struct{ Accepted *bool }
	if !decode(w, r, &in) {
		return
	}
	if in.Accepted == nil {
		fail(w, 422, "VALIDATION_ERROR", "Укажите решение")
		return
	}
	student := current(r).StudentID
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var locked int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM students WHERE "studentID"=$1::uuid FOR UPDATE`, student).Scan(&locked); err != nil {
			return nil, err
		}
		var previous bool
		err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT accepted FROM student_document_choices
   WHERE "studentID"=$1::uuid AND "documentID"=d."documentID" ORDER BY "changedAt" DESC,"choiceID" DESC LIMIT 1),false)
   FROM student_documents d WHERE d."documentID"=$2::uuid AND d."publishedAt"<=now() AND d."requiresConsent" FOR SHARE`, student, doc).Scan(&previous)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		if previous != *in.Accepted {
			_, err = tx.Exec(ctx, `INSERT INTO student_document_choices ("studentID","documentID",accepted) VALUES($1::uuid,$2::uuid,$3)`, student, doc, *in.Accepted)
		}
		return map[string]bool{"accepted": *in.Accepted}, err
	})
}
