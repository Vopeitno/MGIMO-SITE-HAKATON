package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"mgimo/backend/internal/config"
	"mgimo/backend/internal/password"
)

const prefix = "/api/v1/MGIMO"

type Principal struct{ AccountID, Role, Email, DisplayName, StudentID, TeacherID, EmployerID, SessionID string }
type principalKey struct{}
type bucket struct {
	count int
	until time.Time
}
type Server struct {
	DB              *pgxpool.Pool
	Mongo           *mongo.Database
	Config          config.Config
	mu              sync.Mutex
	limits          map[string]bucket
	dummyHash       string
	transcriptSlots chan struct{}
}

func current(r *http.Request) *Principal {
	p, _ := r.Context().Value(principalKey{}).(*Principal)
	return p
}
func hashPassword(value string) (string, error) { return password.Hash(value) }
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != 204 {
		_ = json.NewEncoder(w).Encode(value)
	}
}
func fail(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"code": code, "message": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := strings.Split(r.Header.Get("Content-Type"), ";")[0]; ct != "application/json" {
		fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Ожидается application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректный JSON или неизвестное поле")
		return false
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		fail(w, 400, "BAD_REQUEST", "Ожидается один JSON объект")
		return false
	}
	return true
}

func New(db *pgxpool.Pool, mongoDB *mongo.Database, c config.Config) (*Server, error) {
	h, e := password.Hash("invalid-account-constant-cost-verification")
	if e != nil {
		return nil, e
	}
	return &Server{DB: db, Mongo: mongoDB, Config: c, limits: map[string]bucket{}, dummyHash: h, transcriptSlots: make(chan struct{}, 2)}, nil
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.DB == nil || s.DB.Ping(ctx) != nil {
			fail(w, 503, "NOT_READY", "PostgreSQL недоступен")
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST "+prefix+"/auth/login", s.login)
	mux.HandleFunc("POST "+prefix+"/auth/refresh", s.refresh)
	mux.HandleFunc("POST "+prefix+"/auth/logout", s.logout)
	mux.HandleFunc("GET "+prefix+"/auth/me", func(w http.ResponseWriter, r *http.Request) { write(w, 200, userView(current(r))) })
	s.registerAcademic(mux)
	s.registerStudy(mux)
	mux.HandleFunc("GET "+publicPortfolioPath, s.portfolioPublic)
	mux.HandleFunc("GET "+publicResumePath, s.portfolioResume)
	mux.HandleFunc("GET "+prefix+"/academic/transcript.pdf", s.academicTranscript)
	mux.HandleFunc("GET "+prefix+"/academic/transcript/periods", s.transcriptPeriods)
	s.registerRequests(mux)
	s.registerResources(mux)
	s.registerMessages(mux)
	return s.middleware(mux)
}
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Request-ID", uuid.NewString())
		defer func() {
			if v := recover(); v != nil {
				slog.Error("ошибка обработчика", "request_id", w.Header().Get("X-Request-ID"))
				fail(w, 500, "INTERNAL_ERROR", "Внутренняя ошибка сервера")
			}
		}()
		if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != s.Config.Origin {
			fail(w, 403, "FORBIDDEN", "Недопустимый источник запроса")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "FORBIDDEN", "Межсайтовый запрос запрещён")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/health/") {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, prefix+"/") {
			fail(w, 404, "NOT_FOUND", "Маршрут не найден")
			return
		}
		authAction := r.Method == "POST" && (r.URL.Path == prefix+"/auth/login" || r.URL.Path == prefix+"/auth/refresh")
		if !s.rateAllowed(r, authAction) {
			w.Header().Set("Retry-After", "60")
			fail(w, 429, "TOO_MANY_REQUESTS", "Слишком много запросов, повторите позже")
			return
		}
		publicPortfolio := r.Method == "GET" && (contractPathMatch(publicPortfolioPath, r.URL.Path) || contractPathMatch(publicResumePath, r.URL.Path))
		if !authAction && !publicPortfolio {
			p, err := s.authenticate(r)
			if err != nil && r.URL.Path == prefix+"/auth/logout" {
				p, err = s.authenticateRefresh(r)
			}
			if err != nil {
				fail(w, 401, "UNAUTHORIZED", "Войдите в систему")
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
			if !contractRoleAllowed(r, p.Role) {
				fail(w, 403, "FORBIDDEN", "Недостаточно прав")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) rateAllowed(r *http.Request, auth bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	key := r.RemoteAddr
	// Адрес прокси не берётся из недоверенных X-Forwarded-For.
	if host, _, err := net.SplitHostPort(key); err == nil {
		key = host
	}
	if auth {
		key = "auth:" + key
	}
	if len(s.limits) > 10000 {
		for k, v := range s.limits {
			if now.After(v.until) {
				delete(s.limits, k)
			}
		}
		if len(s.limits) > 10000 {
			return false
		}
	}
	b := s.limits[key]
	if now.After(b.until) {
		b = bucket{until: now.Add(time.Minute)}
	}
	b.count++
	s.limits[key] = b
	limit := 300
	if auth {
		limit = 20
	}
	return b.count <= limit
}
