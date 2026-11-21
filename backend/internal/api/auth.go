package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"mgimo/backend/internal/password"
)

type claims struct {
	SessionID string `json:"sid"`
	Kind      string `json:"kind"`
	jwt.RegisteredClaims
}

func tokenHash(value string) string {
	v := sha256.Sum256([]byte(value))
	return hex.EncodeToString(v[:])
}
func userView(p *Principal) map[string]any {
	return map[string]any{"accountID": p.AccountID, "role": p.Role, "email": p.Email, "displayName": p.DisplayName}
}
func (s *Server) sign(p *Principal, sid, kind string, ttl time.Duration) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{sid, kind, jwt.RegisteredClaims{Issuer: "mgimo", Audience: jwt.ClaimStrings{"mgimo-api"}, Subject: p.AccountID, ID: uuid.NewString(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl))}}).SignedString([]byte(s.Config.JWTSecret))
}
func (s *Server) parse(value, kind string) (*claims, error) {
	c := &claims{}
	t, e := jwt.ParseWithClaims(value, c, func(t *jwt.Token) (any, error) { return []byte(s.Config.JWTSecret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("mgimo"), jwt.WithAudience("mgimo-api"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if e != nil || !t.Valid || c.Kind != kind {
		return nil, errors.New("invalid token")
	}
	if _, e = uuid.Parse(c.Subject); e != nil {
		return nil, e
	}
	if _, e = uuid.Parse(c.SessionID); e != nil {
		return nil, e
	}
	return c, nil
}

const principalSQL = `SELECT a."accountID"::text,a."accountRole"::text,a.email,COALESCE(st."studentName",t."teacherName",e."employerName",''),COALESCE(a."studentID"::text,''),COALESCE(a."teacherID"::text,''),COALESCE(a."employerID"::text,'') FROM accounts a LEFT JOIN students st ON st."studentID"=a."studentID" LEFT JOIN teachers t ON t."teacherID"=a."teacherID" LEFT JOIN employers e ON e."employerID"=a."employerID" `

func scanPrincipal(row pgx.Row) (*Principal, error) {
	p := &Principal{}
	e := row.Scan(&p.AccountID, &p.Role, &p.Email, &p.DisplayName, &p.StudentID, &p.TeacherID, &p.EmployerID)
	return p, e
}
func (s *Server) authenticate(r *http.Request) (*Principal, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, errors.New("no bearer")
	}
	c, e := s.parse(strings.TrimPrefix(h, "Bearer "), "access")
	if e != nil {
		return nil, e
	}
	p, err := scanPrincipal(s.DB.QueryRow(r.Context(), principalSQL+`JOIN user_sessions us ON us."accountID"=a."accountID" WHERE a."accountID"=$1 AND a."isActive" AND us."sessionID"=$2 AND us."revokedAt" IS NULL AND us."expiresAt">now()`, c.Subject, c.SessionID))
	if err == nil {
		p.SessionID = c.SessionID
	}
	return p, err
}
func (s *Server) authenticateRefresh(r *http.Request) (*Principal, error) {
	cookie, e := r.Cookie("mgimo_refresh")
	if e != nil {
		return nil, e
	}
	c, e := s.parse(cookie.Value, "refresh")
	if e != nil {
		return nil, e
	}
	return scanPrincipal(s.DB.QueryRow(r.Context(), principalSQL+`JOIN user_sessions us ON us."accountID"=a."accountID" WHERE a."accountID"=$1 AND a."isActive" AND us."sessionID"=$2 AND us."refreshTokenHash"=$3 AND us."revokedAt" IS NULL AND us."expiresAt">now()`, c.Subject, c.SessionID, tokenHash(cookie.Value)))
}
func (s *Server) setCookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: "mgimo_refresh", Value: value, Path: prefix + "/auth", HttpOnly: true, Secure: s.Config.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	addr, e := mail.ParseAddress(in.Email)
	if e != nil || addr.Address != in.Email || len(in.Email) > 258 || len(in.Password) > 1024 || in.Password == "" {
		fail(w, 400, "BAD_REQUEST", "Укажите email и пароль")
		return
	}
	var h string
	var active bool
	e = s.DB.QueryRow(r.Context(), `SELECT "passwordHash","isActive" FROM accounts WHERE email=$1`, strings.ToLower(strings.TrimSpace(in.Email))).Scan(&h, &active)
	if errors.Is(e, pgx.ErrNoRows) {
		password.Verify(in.Password, s.dummyHash)
		fail(w, 401, "UNAUTHORIZED", "Неверный логин или пароль")
		return
	}
	if e != nil {
		fail(w, 503, "DATABASE_UNAVAILABLE", "Сервис временно недоступен")
		return
	}
	if !password.Verify(in.Password, h) || !active {
		fail(w, 401, "UNAUTHORIZED", "Неверный логин или пароль")
		return
	}
	p, e := scanPrincipal(s.DB.QueryRow(r.Context(), principalSQL+`WHERE a.email=$1`, strings.ToLower(in.Email)))
	if e != nil {
		fail(w, 503, "DATABASE_UNAVAILABLE", "Сервис временно недоступен")
		return
	}
	sid := uuid.NewString()
	refresh, e := s.sign(p, sid, "refresh", 7*24*time.Hour)
	if e != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось создать сессию")
		return
	}
	access, e := s.sign(p, sid, "access", 15*time.Minute)
	if e != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось создать сессию")
		return
	}
	tag, e := s.DB.Exec(r.Context(), `INSERT INTO user_sessions ("sessionID","accountID","refreshTokenHash","expiresAt") SELECT $1,"accountID",$2,$3 FROM accounts WHERE "accountID"=$4 AND "isActive" AND "passwordHash"=$5 FOR SHARE`, sid, tokenHash(refresh), time.Now().Add(7*24*time.Hour), p.AccountID, h)
	if e != nil {
		fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось сохранить сессию")
		return
	}
	if tag.RowsAffected() != 1 {
		fail(w, 401, "UNAUTHORIZED", "Аккаунт отключён")
		return
	}
	s.setCookie(w, refresh, 604800)
	write(w, 200, map[string]any{"accessToken": access, "tokenType": "Bearer", "expiresIn": 900, "user": userView(p)})
}
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, e := r.Cookie("mgimo_refresh")
	if e != nil {
		fail(w, 401, "UNAUTHORIZED", "Сессия отсутствует")
		return
	}
	c, e := s.parse(cookie.Value, "refresh")
	if e != nil {
		s.setCookie(w, "", -1)
		fail(w, 401, "UNAUTHORIZED", "Сессия истекла")
		return
	}
	tx, e := s.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "DATABASE_UNAVAILABLE", "Сервис временно недоступен")
		return
	}
	defer tx.Rollback(r.Context())
	p, e := scanPrincipal(tx.QueryRow(r.Context(), principalSQL+`JOIN user_sessions us ON us."accountID"=a."accountID" WHERE a."accountID"=$1 AND a."isActive" AND us."sessionID"=$2 AND us."refreshTokenHash"=$3 AND us."revokedAt" IS NULL AND us."expiresAt">now() FOR UPDATE OF us,a`, c.Subject, c.SessionID, tokenHash(cookie.Value)))
	if e != nil {
		fail(w, 401, "UNAUTHORIZED", "Сессия недействительна")
		return
	}
	rt, e := s.sign(p, c.SessionID, "refresh", 7*24*time.Hour)
	if e != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось обновить сессию")
		return
	}
	at, e := s.sign(p, c.SessionID, "access", 15*time.Minute)
	if e != nil {
		fail(w, 500, "INTERNAL_ERROR", "Не удалось обновить сессию")
		return
	}
	_, e = tx.Exec(r.Context(), `UPDATE user_sessions SET "refreshTokenHash"=$1,"expiresAt"=$2 WHERE "sessionID"=$3`, tokenHash(rt), time.Now().Add(7*24*time.Hour), c.SessionID)
	if e != nil || tx.Commit(r.Context()) != nil {
		fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось сохранить сессию")
		return
	}
	s.setCookie(w, rt, 604800)
	write(w, 200, map[string]any{"accessToken": at, "tokenType": "Bearer", "expiresIn": 900, "user": userView(p)})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	c, e := s.parse(value, "access")
	if e != nil {
		if cookie, err := r.Cookie("mgimo_refresh"); err == nil {
			c, e = s.parse(cookie.Value, "refresh")
		}
	}
	if e != nil {
		fail(w, 401, "UNAUTHORIZED", "Сессия отсутствует")
		return
	}
	_, e = s.DB.Exec(r.Context(), `UPDATE user_sessions SET "revokedAt"=now() WHERE "sessionID"=$1 AND "accountID"=$2`, c.SessionID, current(r).AccountID)
	if e != nil {
		fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось завершить сессию")
		return
	}
	s.setCookie(w, "", -1)
	w.WriteHeader(204)
}
