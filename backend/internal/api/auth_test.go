package api

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"mgimo/backend/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokens(t *testing.T) {
	s := &Server{Config: config.Config{JWTSecret: strings.Repeat("s", 32)}}
	p := &Principal{AccountID: uuid.NewString()}
	sid := uuid.NewString()
	token, e := s.sign(p, sid, "access", 15*time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	c, e := s.parse(token, "access")
	if e != nil || c.Subject != p.AccountID || c.SessionID != sid {
		t.Fatal("invalid claims", e)
	}
	if _, e = s.parse(token, "refresh"); e == nil {
		t.Fatal("access принят вместо refresh")
	}
	if _, e = s.parse(token+"x", "access"); e == nil {
		t.Fatal("подделанный токен принят")
	}
	expired, e := s.sign(p, sid, "access", -time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.parse(expired, "access"); e == nil {
		t.Fatal("просроченный токен принят")
	}
	none, e := jwt.NewWithClaims(jwt.SigningMethodNone, claims{sid, "access", c.RegisteredClaims}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.parse(none, "access"); e == nil {
		t.Fatal("alg=none принят")
	}
}
func TestCookie(t *testing.T) {
	s := &Server{Config: config.Config{SecureCookies: true}}
	w := httptest.NewRecorder()
	s.setCookie(w, "opaque", 604800)
	cs := w.Result().Cookies()
	if len(cs) != 1 || !cs[0].HttpOnly || !cs[0].Secure || cs[0].SameSite != http.SameSiteStrictMode || cs[0].Path != prefix+"/auth" {
		t.Fatal("небезопасный cookie")
	}
}
func TestDecode(t *testing.T) {
	for _, body := range []string{`{"unknown":1}`, `{} {}`, `null null`, `{`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var value struct {
			Email string `json:"email"`
		}
		if decode(w, r, &value) {
			t.Errorf("принят %s", body)
		}
	}
}
func TestOriginGuard(t *testing.T) {
	s := &Server{Config: config.Config{Origin: "https://app.example.test"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", prefix+"/auth/refresh", nil)
	r.Header.Set("Origin", "https://evil.example.test")
	s.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("межсайтовый запрос дошёл до обработчика")
	})).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestRateLimit(t *testing.T) {
	s := &Server{limits: map[string]bucket{}}
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "[::1]:1000"
	for i := 0; i < 20; i++ {
		if !s.rateAllowed(r, true) {
			t.Fatal(i)
		}
	}
	r.RemoteAddr = "[::1]:1001"
	if s.rateAllowed(r, true) {
		t.Fatal("смена порта обходит ограничение")
	}
}
