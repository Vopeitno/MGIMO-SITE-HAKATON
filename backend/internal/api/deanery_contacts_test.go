package api

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDeaneryContacts(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("local DB required")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "mgimo_") || (cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "localhost") {
		t.Fatal("local database required")
	}
	ctx := context.Background()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Server{DB: db}
	var student string
	if err = db.QueryRow(ctx, `SELECT "studentID"::text FROM students ORDER BY "studentID" LIMIT 1`).Scan(&student); err != nil {
		t.Fatal(err)
	}
	call := func(p *Principal, path string) (int, []messagePerson) {
		r := httptest.NewRequest("GET", path, nil)
		r = r.WithContext(context.WithValue(ctx, principalKey{}, p))
		w := httptest.NewRecorder()
		s.deaneryContacts(w, r)
		var result struct {
			Items []messagePerson `json:"items"`
		}
		json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result.Items
	}
	code, people := call(&Principal{Role: "STUDENT", StudentID: student}, "/")
	if code != 200 {
		t.Fatal(code)
	}
	for _, person := range people {
		if person.Role != "DEAN" || person.DisplayName == "" {
			t.Fatal("invalid contact")
		}
		var eligible bool
		err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM students st JOIN faculties f ON f."facultyID"=st."facultyID" JOIN deanery_staff ds ON ds."deaneryID"=f."deaneryID" JOIN accounts a ON a."employerID"=ds."employerID" WHERE st."studentID"=$1 AND a."accountID"=$2 AND a."isActive")`, student, person.AccountID).Scan(&eligible)
		if err != nil || !eligible {
			t.Fatal("foreign faculty contact", err)
		}
	}
	code, people = call(&Principal{Role: "STUDENT", StudentID: uuid.NewString()}, "/")
	if code != 200 || len(people) != 0 {
		t.Fatal("student scope bypass")
	}
	for _, role := range []string{"DEAN", "TEACHER", "SUPPORT", "ADMIN"} {
		if code, _ = call(&Principal{Role: role}, "/"); code != 403 {
			t.Fatal("role allowed", role, code)
		}
	}
	if code, _ = call(nil, "/"); code != 401 {
		t.Fatal("anonymous allowed")
	}
	if code, _ = call(&Principal{Role: "STUDENT", StudentID: student}, "/?facultyID=other"); code != 400 {
		t.Fatal("faculty override allowed")
	}
}
