package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		body         string
		draft, valid bool
	}{
		{`{}`, true, true}, {`{}`, false, false},
		{`{"appointmentTypeID":null}`, true, true}, {`{"appointmentTypeID":null}`, false, false},
		{`{"appointmentTitle":null}`, true, true}, {`{"appointmentTitle":12}`, true, false},
		{`{"appointmentTypeID":"wrong"}`, true, false},
		{`{"appointmentTitle":"` + strings.Repeat("я", 100) + `"}`, true, true},
		{`{"appointmentTitle":"` + strings.Repeat("я", 101) + `"}`, true, false},
		{`{"appointmentTypeID":"11111111-1111-4111-8111-111111111111","appointmentTitle":" ","appointmentDescription":"x"}`, false, false},
		{`{"appointmentTypeID":"11111111-1111-4111-8111-111111111111","appointmentTitle":"x","appointmentDescription":"y"}`, false, true},
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tc.body), &raw); err != nil {
			t.Fatal(err)
		}
		_, err := requestFields(raw, tc.draft)
		if (err == nil) != tc.valid {
			t.Errorf("body=%s draft=%v err=%v", tc.body, tc.draft, err)
		}
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=", "limit=1&limit=2", "offset=-1", "offset=9223372036854775808"} {
		if _, _, err := requestPageParams(httptest.NewRequest("GET", "/?"+query, nil)); err == nil {
			t.Errorf("accepted %s", query)
		}
	}
	l, o, err := requestPageParams(httptest.NewRequest("GET", "/", nil))
	if err != nil || l != 20 || o != 0 {
		t.Fatalf("defaults %d %d %v", l, o, err)
	}
}
func TestRequestTransitions(t *testing.T) {
	statuses := []string{"creating", "proccess", "ready", "canceled"}
	for _, old := range statuses {
		for _, next := range statuses {
			want := old == "creating" && (next == "proccess" || next == "canceled") || old == "proccess" && (next == "ready" || next == "canceled")
			if got := requestTransition(old, next, "причина") == nil; got != want {
				t.Errorf("%s -> %s allowed=%v", old, next, got)
			}
		}
	}
	for _, reason := range []string{"", " \n", strings.Repeat("я", 565)} {
		if requestTransition("proccess", "canceled", reason) == nil {
			t.Errorf("accepted invalid reason %q", reason)
		}
	}
}
func TestRequestOwnership(t *testing.T) {
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for role, col := range map[string]string{"STUDENT": "studentID", "TEACHER": "teacherID", "DEAN": "createdByEmployerID"} {
		p := &Principal{Role: role, StudentID: id, TeacherID: id, EmployerID: id}
		if !requestOwned(p, map[string]any{col: strings.ToUpper(id)}) {
			t.Fatal(role)
		}
		if requestOwned(p, map[string]any{"employerID": id}) {
			t.Fatal("assignee considered author")
		}
	}
	if requestOwned(&Principal{Role: "STUDENT"}, map[string]any{"studentID": ""}) {
		t.Fatal("empty owner accepted")
	}
}
func TestRequestErrorResponses(t *testing.T) {
	for _, err := range []error{requestBad(403, "denied"), errors.New("passportNumber secret database error")} {
		w := httptest.NewRecorder()
		requestError(w, err)
		if strings.Contains(w.Body.String(), "passport") || w.Code < 400 {
			t.Fatalf("unsafe response %s", w.Body.String())
		}
	}
}

// Integration tests use a private schema and the real migration; no shared data
// is modified. Set REQUEST_TEST_DATABASE_URL to enable PostgreSQL scenarios.
type requestFixture struct {
	pool                                                  *pgxpool.Pool
	mux                                                   *http.ServeMux
	student, other, teacher, dean, support, second, third *Principal
	typeID                                                string
}

func requestTestID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func requestTestFixture(t *testing.T) *requestFixture {
	t.Helper()
	dsn := os.Getenv("REQUEST_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("REQUEST_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("request_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, e := admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
		if e != nil {
			t.Error(e)
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",pg_catalog"
	cfg.MaxConns = 12
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	sql, err := os.ReadFile("../../migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := strings.ReplaceAll(string(sql), "public.", schema+".")
	migration = strings.ReplaceAll(migration, "search_path = public,", "search_path = "+schema+",")
	if _, err = pool.Exec(ctx, migration); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, sql, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO groups ("groupID","groupNumber") VALUES (1,101)`)
	exec(`INSERT INTO deaneries ("deaneryID",name) VALUES (1,'Первый'),(2,'Второй')`)
	exec(`INSERT INTO faculties ("facultyID","deaneryID",title) VALUES ($1,1,'Первый'),($2,2,'Второй')`, requestTestID(90), requestTestID(91))
	makePerson := func(role string, n int, faculty string) *Principal {
		id, account := requestTestID(n), requestTestID(n+100)
		p := &Principal{AccountID: account, Role: role}
		if role == "STUDENT" {
			p.StudentID = id
			exec(`INSERT INTO students ("studentID","studentNumber","studentName","studentEmail","studentPassword","studentPhoneNumber","studentBirthday","passportSeria","passportNumber","passportWhoIssued","passportDataIssue","passportCodeDivison","passportCityBorn","passportRegistration","studentStatus","studentEnrollmentDate","studentGender","groupID","facultyID") VALUES ($1,$2,'Студент',$3,'unused',$3,now(),'0000','000000','secret','secret','secret','secret','secret','active',now(),'муж',1,$4)`, id, n, fmt.Sprintf("person%d@example.test", n), faculty)
			exec(`INSERT INTO accounts ("accountID",email,"passwordHash","accountRole","studentID") VALUES ($1,$2,'unused','STUDENT',$3)`, account, fmt.Sprintf("person%d@example.test", n), id)
		} else if role == "TEACHER" {
			p.TeacherID = id
			exec(`INSERT INTO teachers ("teacherID","teacherNumber","teacherName","teacherPosition","teacherDegree","teacherEmail","teacherPassword","teacherPhoneNumber","teacherBirthday","teacherStatus") VALUES ($1,$2,'Преподаватель','позиция','степень',$3,'unused',$3,now(),'active')`, id, n, fmt.Sprintf("person%d@example.test", n))
			exec(`INSERT INTO accounts ("accountID",email,"passwordHash","accountRole","teacherID") VALUES ($1,$2,'unused','TEACHER',$3)`, account, fmt.Sprintf("person%d@example.test", n), id)
			exec(`INSERT INTO "facultyTeacher" ("facultyID","teacherID") VALUES ($1,$2)`, faculty, id)
		} else {
			p.EmployerID = id
			exec(`INSERT INTO employers ("employerID","employerNumber","employerName","employerPhoneNumber","employerEmail","passportSeria","passportNumber","passportWhoIssued","passportDataIssue","passportCodeDivison","passportCityBorn","passportRegistration","employerStatus","hireAt") VALUES ($1,$2,'Сотрудник',$3,$3,'0000','000000','secret','secret','secret','secret','secret','active',now())`, id, n, fmt.Sprintf("person%d@example.test", n))
			exec(`INSERT INTO accounts ("accountID",email,"passwordHash","accountRole","employerID") VALUES ($1,$2,'unused',$3,$4)`, account, fmt.Sprintf("person%d@example.test", n), role, id)
		}
		return p
	}
	f := &requestFixture{pool: pool, typeID: requestTestID(80)}
	f.student = makePerson("STUDENT", 1, requestTestID(90))
	f.other = makePerson("STUDENT", 2, requestTestID(91))
	f.teacher = makePerson("TEACHER", 3, requestTestID(90))
	f.dean = makePerson("DEAN", 4, "")
	f.support = makePerson("SUPPORT", 5, "")
	f.second = makePerson("SUPPORT", 6, "")
	f.third = makePerson("SUPPORT", 7, "")
	exec(`INSERT INTO deanery_staff ("deaneryID","employerID") VALUES (1,$1)`, f.dean.EmployerID)
	exec(`INSERT INTO appointment_types ("appointmentTypeID","appointmentTypeTitle","isActive") VALUES ($1,'Справка',true),($2,'Архивный',false)`, f.typeID, requestTestID(81))
	f.mux = http.NewServeMux()
	(&Server{DB: pool}).registerRequests(f.mux)
	return f
}
func (f *requestFixture) call(p *Principal, method, path, body string) (int, map[string]any) {
	r := httptest.NewRequest(method, "/api/v1/MGIMO"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
	}
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	var v map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v
}
func (f *requestFixture) create(t *testing.T, p *Principal) string {
	t.Helper()
	status, v := f.call(p, "POST", "/requests", fmt.Sprintf(`{"appointmentTypeID":%q,"appointmentTitle":"Справка","appointmentDescription":"Описание"}`, f.typeID))
	if status != 201 {
		t.Fatalf("create: %d %v", status, v)
	}
	return requestString(v["appoinmentID"])
}
func requestTestStatus(t *testing.T, want, got int, v map[string]any) {
	t.Helper()
	if want != got {
		t.Fatalf("want %d got %d: %v", want, got, v)
	}
}
func TestRequestIntegrationAccessAndLifecycle(t *testing.T) {
	f := requestTestFixture(t)
	for _, p := range []*Principal{f.student, f.teacher, f.dean} {
		id := f.create(t, p)
		status, v := f.call(p, "GET", "/requests/"+id, "")
		requestTestStatus(t, 200, status, v)
		encoded, _ := json.Marshal(v)
		if strings.Contains(string(encoded), "passport") || strings.Contains(string(encoded), "Password") {
			t.Fatal("private data leaked")
		}
	}
	id := f.create(t, f.student)
	for _, tc := range []struct {
		p    *Principal
		want int
	}{{nil, 401}, {f.other, 403}, {f.dean, 200}, {f.support, 403}, {&Principal{Role: "ADMIN"}, 403}} {
		for _, suffix := range []string{"", "/status", "/history"} {
			status, v := f.call(tc.p, "GET", "/requests/"+id+suffix, "")
			requestTestStatus(t, tc.want, status, v)
		}
	}
	foreign := f.create(t, f.other)
	status, v := f.call(f.dean, "GET", "/requests/"+foreign, "")
	requestTestStatus(t, 403, status, v)
	status, v = f.call(f.student, "GET", "/requests?limit=1&offset=100", "")
	requestTestStatus(t, 200, status, v)
	if v["total"] != float64(2) || len(v["items"].([]any)) != 0 {
		t.Fatalf("page %v", v)
	}
	status, v = f.call(f.student, "GET", "/requests/types", "")
	requestTestStatus(t, 200, status, v)
	if v["total"] != float64(1) {
		t.Fatal(v)
	}
	status, v = f.call(f.student, "GET", "/requests/types?activeOnly=false", "")
	requestTestStatus(t, 200, status, v)
	if v["total"] != float64(2) {
		t.Fatal(v)
	}
	status, v = f.call(f.student, "POST", "/requests", `{"studentID":"spoofed"}`)
	requestTestStatus(t, 400, status, v)
	status, v = f.call(f.support, "POST", "/requests/"+id+"/claim", "")
	requestTestStatus(t, 200, status, v)
	if v["appointmentStatus"] != "proccess" {
		t.Fatal(v)
	}
	status, v = f.call(f.second, "PATCH", "/requests/"+id+"/status", `{"appointmentStatus":"ready"}`)
	requestTestStatus(t, 403, status, v)
	status, v = f.call(f.support, "PATCH", "/requests/"+id+"/status", `{"appointmentStatus":"canceled"}`)
	requestTestStatus(t, 422, status, v)
	status, v = f.call(f.support, "PATCH", "/requests/"+id+"/status", `{"appointmentStatus":"ready"}`)
	requestTestStatus(t, 200, status, v)
	status, v = f.call(f.support, "PATCH", "/requests/"+id+"/status", `{"appointmentStatus":"proccess"}`)
	requestTestStatus(t, 409, status, v)
	status, v = f.call(f.student, "GET", "/requests/"+id+"/history", "")
	requestTestStatus(t, 200, status, v)
	if v["total"] != float64(3) {
		t.Fatal(v)
	}
}
func TestRequestIntegrationDrafts(t *testing.T) {
	f := requestTestFixture(t)
	status, v := f.call(f.student, "POST", "/requests/drafts", `{}`)
	requestTestStatus(t, 201, status, v)
	id := requestString(v["appointmentDraftID"])
	path := "/requests/drafts/" + id
	status, v = f.call(f.other, "GET", path, "")
	requestTestStatus(t, 403, status, v)
	status, v = f.call(f.student, "POST", path+"/submit", "")
	requestTestStatus(t, 422, status, v)
	status, v = f.call(f.student, "PATCH", path, fmt.Sprintf(`{"appointmentTypeID":%q,"appointmentTitle":"Справка","appointmentDescription":"Текст"}`, f.typeID))
	requestTestStatus(t, 200, status, v)
	status, v = f.call(f.student, "PATCH", path, `{"appointmentTitle":null}`)
	requestTestStatus(t, 200, status, v)
	if v["appointmentTitle"] != nil {
		t.Fatal("null did not clear field")
	}
	status, v = f.call(f.student, "POST", path+"/submit", "")
	requestTestStatus(t, 422, status, v)
	status, v = f.call(f.student, "PATCH", path, `{"appointmentTitle":"Новое название"}`)
	requestTestStatus(t, 200, status, v)
	results := make(chan int, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			code, _ := f.call(f.student, "POST", path+"/submit", "")
			results <- code
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for code := range results {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	for _, method := range []string{"PATCH", "DELETE"} {
		status, v = f.call(f.student, method, path, `{"appointmentTitle":"Нельзя"}`)
		requestTestStatus(t, 409, status, v)
	}
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("appointments=%d %v", count, err)
	}
	status, v = f.call(f.student, "GET", "/requests/drafts", "")
	requestTestStatus(t, 200, status, v)
	if v["total"] != float64(0) {
		t.Fatal(v)
	}
	status, v = f.call(f.teacher, "POST", "/requests/drafts", `{}`)
	requestTestStatus(t, 201, status, v)
	path = "/requests/drafts/" + requestString(v["appointmentDraftID"])
	status, v = f.call(f.teacher, "DELETE", path, "")
	requestTestStatus(t, 204, status, v)
	status, v = f.call(f.teacher, "GET", path, "")
	requestTestStatus(t, 404, status, v)
}
func TestRequestIntegrationConcurrentAssignments(t *testing.T) {
	f := requestTestFixture(t)
	a, b := f.create(t, f.student), f.create(t, f.teacher)
	// Same support, different appointments: only one may become active.
	start := make(chan struct{})
	out := make(chan int, 2)
	var wg sync.WaitGroup
	for _, id := range []string{a, b} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			status, _ := f.call(f.support, "POST", "/requests/"+id+"/claim", "")
			out <- status
		}(id)
	}
	close(start)
	wg.Wait()
	close(out)
	counts := map[int]int{}
	for status := range out {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	var claimed, unclaimed string
	if err := f.pool.QueryRow(context.Background(), `SELECT "appoinmentID"::text FROM appointment WHERE "employerID"=$1`, f.support.EmployerID).Scan(&claimed); err != nil {
		t.Fatal(err)
	}
	if claimed == a {
		unclaimed = b
	} else {
		unclaimed = a
	}
	status, v := f.call(f.second, "POST", "/requests/"+unclaimed+"/claim", "")
	requestTestStatus(t, 200, status, v)
	status, v = f.call(f.support, "POST", "/requests/"+claimed+"/transfer", fmt.Sprintf(`{"employerID":%q}`, f.second.EmployerID))
	requestTestStatus(t, 409, status, v)
	status, v = f.call(f.support, "POST", "/requests/"+claimed+"/transfer", fmt.Sprintf(`{"employerID":%q}`, f.third.EmployerID))
	requestTestStatus(t, 200, status, v)
	status, v = f.call(f.support, "GET", "/requests/"+claimed, "")
	requestTestStatus(t, 403, status, v)
	status, v = f.call(f.third, "GET", "/requests/"+claimed+"/assignments", "")
	requestTestStatus(t, 200, status, v)
	if v["total"] != float64(2) {
		t.Fatal(v)
	}
	status, v = f.call(f.support, "GET", "/requests/unassigned", "")
	requestTestStatus(t, 200, status, v)
	// Free the second support, then two supports race to claim one new request.
	status, v = f.call(f.second, "PATCH", "/requests/"+unclaimed+"/status", `{"appointmentStatus":"canceled","reason":"Отклонена"}`)
	requestTestStatus(t, 200, status, v)
	c := f.create(t, f.student)
	start = make(chan struct{})
	out = make(chan int, 2)
	for _, p := range []*Principal{f.support, f.second} {
		wg.Add(1)
		go func(p *Principal) {
			defer wg.Done()
			<-start
			code, _ := f.call(p, "POST", "/requests/"+c+"/claim", "")
			out <- code
		}(p)
	}
	close(start)
	wg.Wait()
	close(out)
	counts = map[int]int{}
	for code := range out {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestRequestIntegrationRollback(t *testing.T) {
	f := requestTestFixture(t)
	// Make audit writes fail. Neither appointment creation nor draft state may commit.
	_, err := f.pool.Exec(context.Background(), `ALTER TABLE appointment_history ADD CONSTRAINT reject_history CHECK ("appointmentStatus" <> 'creating')`)
	if err != nil {
		t.Fatal(err)
	}
	status, v := f.call(f.student, "POST", "/requests/drafts", fmt.Sprintf(`{"appointmentTypeID":%q,"appointmentTitle":"Название","appointmentDescription":"Описание"}`, f.typeID))
	requestTestStatus(t, 201, status, v)
	id := requestString(v["appointmentDraftID"])
	status, v = f.call(f.student, "POST", "/requests/drafts/"+id+"/submit", "")
	requestTestStatus(t, 422, status, v)
	var count int
	var draftStatus string
	if err = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial create: %d %v", count, err)
	}
	if err = f.pool.QueryRow(context.Background(), `SELECT "draftStatus"::text FROM appointment_drafts WHERE "appointmentDraftID"=$1`, id).Scan(&draftStatus); err != nil || draftStatus != "editing" {
		t.Fatalf("partial submit: %s %v", draftStatus, err)
	}
}

func TestRequestIntegrationDeliveryAndRecovery(t *testing.T) {
	f := requestTestFixture(t)
	building := requestTestID(500)
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO buildings("buildingID",building) VALUES($1,'Корпус выдачи')`, building); err != nil {
		t.Fatal(err)
	}
	status, v := f.call(f.student, "POST", "/requests/drafts", fmt.Sprintf(`{"appointmentTypeID":%q,"appointmentTitle":"Справка","appointmentDescription":"Способ получения: Забрать в деканате"}`, f.typeID))
	requestTestStatus(t, 201, status, v)
	id := requestString(v["appointmentDraftID"])
	path := "/requests/drafts/" + id + "/submit"
	status, v = f.call(f.student, "POST", path, `{"buildingID":"wrong"}`)
	requestTestStatus(t, 422, status, v)
	status, v = f.call(f.student, "POST", path, `{"buildingID":null,"studentID":"wrong"}`)
	requestTestStatus(t, 400, status, v)
	status, v = f.call(f.student, "POST", path, fmt.Sprintf(`{"buildingID":%q}`, building))
	requestTestStatus(t, 201, status, v)
	if v["buildingID"] != building || v["appoinmentID"] != id {
		t.Fatal("delivery/recoverable ID not persisted", v)
	}
	status, v = f.call(f.student, "POST", path, fmt.Sprintf(`{"buildingID":%q}`, building))
	requestTestStatus(t, 409, status, v)
	status, v = f.call(f.student, "GET", "/requests/"+id, "")
	requestTestStatus(t, 200, status, v)
	status, v = f.call(f.other, "GET", "/requests/"+id, "")
	requestTestStatus(t, 403, status, v)
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate submission", count, err)
	}
}
