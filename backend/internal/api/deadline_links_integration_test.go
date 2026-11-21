package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"mgimo/backend/internal/config"
)

// Read-only integration test against a disposable local database; no assignment URLs
// are written to the cloud, the database, or the application's actual configuration.
func TestDeadlineLinksReadOnly(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("local TEST_DATABASE_URL required")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid test database")
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "mgimo_") || (cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "localhost") {
		t.Fatal("local mgimo_ test database required")
	}
	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var student string
	var deadline int
	err = db.QueryRow(context.Background(), `SELECT st."studentID"::text,d."deadlineID" FROM students st JOIN "courseGroup" cg ON cg."groupID"=st."groupID" JOIN deadlines d ON d."courseID"=cg."courseID" ORDER BY st."studentID",d."deadlineID" LIMIT 1`).Scan(&student, &deadline)
	if err != nil {
		t.Fatal(err)
	}
	link := config.DefaultMoodleURL
	s := &Server{DB: db, Config: config.Config{MoodleURL: link}}
	r := httptest.NewRequest("GET", prefix+"/academic/deadlines?limit=100", nil)
	r = r.WithContext(context.WithValue(r.Context(), principalKey{}, &Principal{Role: "STUDENT", StudentID: student}))
	w := httptest.NewRecorder()
	s.academicDeadlines(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		Items []struct {
			ID  int     `json:"deadlineID"`
			URL *string `json:"submissionURL"`
		}
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range result.Items {
		if item.ID == deadline {
			found = true
			if item.URL == nil || *item.URL != link {
				t.Fatal("configured link not returned")
			}
		}
		if item.ID == 2147483647 {
			t.Fatal("configuration introduced a non-accessible assignment")
		}
	}
	if !found {
		t.Fatal("accessible deadline not returned")
	}

	w = httptest.NewRecorder()
	s.academicDeadlines(w, r)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal("missing configuration failed")
	}
	for _, item := range result.Items {
		if item.URL == nil || *item.URL != config.DefaultMoodleURL {
			t.Fatal("missing Moodle fallback")
		}
	}
	w = httptest.NewRecorder()
	s.academicDeadlines(w, httptest.NewRequest("GET", prefix+"/academic/deadlines", nil))
	if w.Code != 401 {
		t.Fatal("unauthenticated access")
	}
}
