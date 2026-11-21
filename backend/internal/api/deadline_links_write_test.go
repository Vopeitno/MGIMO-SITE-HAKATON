package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mgimo/backend/internal/config"
)

func TestDeadlineLinkEditing(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("local TEST_DATABASE_URL required")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "mgimo_") || (cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "localhost") {
		t.Fatal("local disposable mgimo_ database required")
	}
	ctx := context.Background()
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "deadline_link_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.ConnConfig.RuntimeParams["search_path"] = name + ",pg_catalog"
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(ctx, `CREATE TABLE teachers("teacherID" uuid PRIMARY KEY);
 CREATE TABLE faculties("facultyID" uuid PRIMARY KEY,"deaneryID" int);
 CREATE TABLE deanery_staff("deaneryID" int,"employerID" uuid);
 CREATE TABLE courses("courseID" uuid PRIMARY KEY,"facultyID" uuid,title text);
 CREATE TABLE students("studentID" uuid PRIMARY KEY,"groupID" int);
 CREATE TABLE "courseGroup"("courseID" uuid,"groupID" int);
 CREATE VIEW student_course_access AS SELECT st."studentID",cg."courseID" FROM students st JOIN "courseGroup" cg USING ("groupID");
 CREATE TABLE "courseTeacher"("courseID" uuid REFERENCES courses,"teacherID" uuid REFERENCES teachers,PRIMARY KEY("courseID","teacherID"));
 CREATE TABLE deadlines("deadlineID" int PRIMARY KEY,"courseID" uuid REFERENCES courses,"deadlineTitle" text,"deadlineDescription" text,"deadlineEndAt" timestamptz);
 `)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/002_deadline_links.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, strings.ReplaceAll(string(migration), "public.", schema+".")); err != nil {
		t.Fatal(err)
	}
	teacher, other, student, course := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO teachers VALUES($1),($2)`, []any{teacher, other}},
		{`INSERT INTO courses("courseID",title) VALUES($1,'Предмет')`, []any{course}},
		{`INSERT INTO "courseTeacher" VALUES($1,$2)`, []any{course, teacher}},
		{`INSERT INTO students VALUES($1,1)`, []any{student}},
		{`INSERT INTO "courseGroup" VALUES($1,1)`, []any{course}},
		{`INSERT INTO deadlines VALUES(1,$1,'Задание','Описание',now())`, []any{course}},
	} {
		if _, err = db.Exec(ctx, tc.sql, tc.args...); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{DB: db, Config: config.Config{MoodleURL: config.DefaultMoodleURL}}
	call := func(p *Principal, body string, want int) {
		t.Helper()
		r := httptest.NewRequest("PATCH", prefix+"/academic/deadlines/1/submission-link", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.SetPathValue("deadlineID", "1")
		r = r.WithContext(context.WithValue(ctx, principalKey{}, p))
		w := httptest.NewRecorder()
		s.deadlineLinkPatch(w, r)
		if w.Code != want {
			t.Fatalf("got %d want %d: %s", w.Code, want, w.Body.String())
		}
	}
	own := &Principal{Role: "TEACHER", TeacherID: teacher}
	link := "https://lms.example.test/mod/assign/view.php?id=100"
	call(own, `{"submissionURL":"`+link+`"}`, 200)
	var saved, author string
	if err = db.QueryRow(ctx, `SELECT "submissionURL","updatedByTeacherID"::text FROM deadline_links WHERE "deadlineID"=1`).Scan(&saved, &author); err != nil || saved != link || author != teacher {
		t.Fatal("saved URL/audit mismatch", err)
	}
	call(&Principal{Role: "TEACHER", TeacherID: other}, `{"submissionURL":"https://lms.example.test/wrong"}`, 404)
	for _, role := range []string{"STUDENT", "DEAN", "SUPPORT", "ADMIN"} {
		call(&Principal{Role: role}, `{"submissionURL":null}`, 403)
	}
	call(nil, `{"submissionURL":null}`, 401)
	for _, body := range []string{`{"submissionURL":"javascript:alert(1)"}`, `{"submissionURL":"https://user:password@lms.example.test/"}`, `{"submissionURL":"http://lms.example.test/"}`, `{"submissionURL":42}`} {
		call(own, body, 422)
	}
	call(own, `{}`, 400)
	call(own, `{"submissionURL":null,"teacherID":"wrong"}`, 400)
	read := func(want string) {
		t.Helper()
		r := httptest.NewRequest("GET", prefix+"/academic/deadlines", nil)
		r = r.WithContext(context.WithValue(ctx, principalKey{}, &Principal{Role: "STUDENT", StudentID: student}))
		w := httptest.NewRecorder()
		s.academicDeadlines(w, r)
		var result struct {
			Items []struct {
				URL string `json:"submissionURL"`
			}
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Items) != 1 || result.Items[0].URL != want {
			t.Fatalf("student read: %s", w.Body.String())
		}
	}
	read(link)
	call(own, `{"submissionURL":null}`, 200)
	read(config.DefaultMoodleURL)
	call(own, `{"submissionURL":"`+link+`"}`, 200)
	if _, err = db.Exec(ctx, `DELETE FROM "courseTeacher" WHERE "teacherID"=$1`, teacher); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `DELETE FROM teachers WHERE "teacherID"=$1`, teacher); err != nil {
		t.Fatal(err)
	}
	var authorAfter *string
	if err = db.QueryRow(ctx, `SELECT "updatedByTeacherID"::text FROM deadline_links WHERE "deadlineID"=1`).Scan(&authorAfter); err != nil || authorAfter != nil {
		t.Fatal("teacher delete must preserve link and clear audit", err)
	}
	if _, err = db.Exec(ctx, `DELETE FROM deadlines WHERE "deadlineID"=1`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM deadline_links`).Scan(&count); err != nil || count != 0 {
		t.Fatal("deadline cascade failed", err)
	}
}
