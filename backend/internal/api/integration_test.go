package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mgimo/backend/internal/config"
)

// Requires a disposable LOCAL database populated by manage migrate + seed.
// Refuses arbitrary database names to protect real project data.
func TestIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("задайте TEST_DATABASE_URL для изолированной локальной базы")
	}
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "mgimo_") || (cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "localhost") {
		t.Fatal("требуется локальная тестовая база с префиксом mgimo_")
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	db, e := pgxpool.NewWithConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s, e := New(db, nil, config.Config{JWTSecret: strings.Repeat("x", 32), Origin: "http://localhost:5173", UploadDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	handler := s.Handler()
	type client struct {
		token  string
		cookie *http.Cookie
	}
	remoteAddr := "127.0.0.1:1000"
	call := func(c client, method, path string, body any) (int, map[string]any, *httptest.ResponseRecorder) {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, prefix+path, bytes.NewReader(data))
		r.RemoteAddr = remoteAddr
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		if c.cookie != nil {
			r.AddCookie(c.cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result, w
	}
	expect := func(t *testing.T, c client, method, path string, body any, want int) map[string]any {
		t.Helper()
		status, v, w := call(c, method, path, body)
		if status != want {
			t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, status, want, w.Body.String())
		}
		return v
	}
	login := func(email string) client {
		t.Helper()
		secret := os.Getenv("DEMO_PASSWORD")
		if secret == "" {
			t.Fatal("DEMO_PASSWORD обязателен")
		}
		status, v, w := call(client{}, "POST", "/auth/login", map[string]any{"email": email, "password": secret})
		if status != 200 {
			t.Fatalf("вход %s: %s", email, w.Body.String())
		}
		return client{v["accessToken"].(string), w.Result().Cookies()[0]}
	}
	id := func(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
	a, b, teacher, support, dean, admin := login("student1@example.test"), login("student2@example.test"), login("teacher@example.test"), login("support@example.test"), login("dean@example.test"), login("admin@example.test")
	t.Run("сервисы кампуса и конкурирующие бронирования", func(t *testing.T) {
		remoteAddr = "127.0.0.2:1000"
		t.Cleanup(func() { remoteAddr = "127.0.0.1:1000" })
		const benefits = "/campus/benefits"
		const reports = "/campus/lost-reports"
		const resources = "/campus/resources"
		const slots = "/campus/resource-slots"
		suffix := uuid.NewString()
		offer := map[string]string{"title": "Льгота " + suffix, "description": "Условия получения", "url": "https://example.test/details"}
		expect(t, a, "POST", benefits, offer, 403)
		benefit := expect(t, dean, "POST", benefits, offer, 201)["id"].(string)
		offer["description"] = "Обновлённые условия"
		expect(t, dean, "PUT", benefits+"/"+benefit, offer, 204)
		list := expect(t, a, "GET", benefits, nil, 200)["items"].([]any)
		if len(list) == 0 || list[0].(map[string]any)["description"] != offer["description"] {
			t.Fatalf("Студент не видит действующую льготу: %v", list)
		}
		expect(t, dean, "DELETE", benefits+"/"+benefit, nil, 204)

		report := expect(t, a, "POST", reports, map[string]string{
			"kind": "lost", "title": "Пропажа " + suffix, "location": "Учебный корпус", "description": "Папка",
		}, 201)["id"].(string)
		if items := expect(t, b, "GET", reports, nil, 200)["items"].([]any); len(items) != 0 {
			t.Fatalf("Другому студенту показали чужое сообщение: %v", items)
		}
		reply := map[string]string{"resolution": "Папка передана в бюро находок"}
		expect(t, dean, "PUT", reports+"/"+report+"/resolution", reply, 403)
		expect(t, support, "PUT", reports+"/"+report+"/resolution", reply, 204)
		list = expect(t, a, "GET", reports, nil, 200)["items"].([]any)
		if len(list) == 0 || list[0].(map[string]any)["resolution"] != reply["resolution"] {
			t.Fatalf("Ответ не показан владельцу сообщения: %v", list)
		}

		resource := expect(t, support, "POST", resources, map[string]string{
			"kind": "equipment", "title": "Ресурс " + suffix, "location": "Учебный корпус",
		}, 201)["id"].(string)
		start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Minute)
		slot := map[string]string{"resourceID": resource, "startsAt": start.Format(time.RFC3339),
			"endsAt": start.Add(time.Hour).Format(time.RFC3339)}
		slotID := expect(t, support, "POST", slots, slot, 201)["id"].(string)
		expect(t, support, "POST", slots, slot, 409)
		path := slots + "/" + slotID + "/reservation"
		statuses := make([]int, 2)
		var wg sync.WaitGroup
		for i, student := range []client{a, b} {
			wg.Add(1)
			go func(i int, student client) {
				defer wg.Done()
				statuses[i], _, _ = call(student, "PUT", path, nil)
			}(i, student)
		}
		wg.Wait()
		if !((statuses[0] == 204 && statuses[1] == 409) || (statuses[0] == 409 && statuses[1] == 204)) {
			t.Fatalf("Ожидалась одна успешная бронь: %v", statuses)
		}
		owner, other := a, b
		if statuses[1] == 204 {
			owner, other = b, a
		}
		expect(t, other, "DELETE", path, nil, 404)
		expect(t, support, "DELETE", slots+"/"+slotID, nil, 409)
		found := false
		for _, value := range expect(t, other, "GET", slots, nil, 200)["items"].([]any) {
			item := value.(map[string]any)
			if item["id"] == slotID {
				found = item["reserved"] == true && item["mine"] == false && item["studentNumber"] == nil && item["studentName"] == nil
			}
		}
		if !found {
			t.Fatal("Занятый слот должен скрывать личность владельца")
		}
		expect(t, owner, "DELETE", path, nil, 204)
		expect(t, other, "PUT", path, nil, 204)
		expect(t, other, "DELETE", path, nil, 204)
		expect(t, support, "DELETE", slots+"/"+slotID, nil, 204)
	})
	t.Run("публичная карточка вещи и частное обращение", func(t *testing.T) {
		remoteAddr = "127.0.0.4:1000"
		t.Cleanup(func() { remoteAddr = "127.0.0.1:1000" })
		const items = "/campus/lost-items"
		const reports = "/campus/lost-reports"
		body := map[string]string{"title": "Чёрный шарф", "kind": "found", "location": "Учебный корпус"}
		expect(t, a, "POST", items, body, 403)
		item := expect(t, support, "POST", items, body, 201)["id"].(string)
		entry := expect(t, a, "GET", items, nil, 200)["items"].([]any)[0].(map[string]any)
		if entry["id"] != item || entry["title"] != body["title"] || len(entry) != 5 {
			t.Fatalf("Неверные публичные поля карточки: %v", entry)
		}
		report := map[string]string{"itemID": item, "kind": "lost", "title": body["title"], "location": body["location"], "description": "Моя вещь с вышивкой"}
		expect(t, a, "POST", reports, report, 201)
		row := expect(t, a, "GET", reports, nil, 200)["items"].([]any)[0].(map[string]any)
		if row["itemID"] != item || row["itemTitle"] != body["title"] {
			t.Fatalf("Привязка вещи не сохранилась: %v", row)
		}
		expect(t, support, "DELETE", items+"/"+item, nil, 204)
		expect(t, a, "POST", reports, report, 409)
		delete(report, "itemID")
		expect(t, a, "POST", reports, report, 201)
	})
	t.Run("карьера, граница третьего курса и отчёт", func(t *testing.T) {
		remoteAddr = "127.0.0.5:1000"
		t.Cleanup(func() { remoteAddr = "127.0.0.1:1000" })
		const offers = "/career/offers"
		const practices = "/career/practices"
		ctx := context.Background()
		faculty := expect(t, dean, "GET", "/career/faculties", nil, 200)["items"].([]any)[0].(map[string]any)["facultyID"].(string)
		offer := map[string]string{"facultyID": faculty, "kind": "internship", "title": "Практикум", "organization": "Организация",
			"location": "Москва", "description": "Описание", "url": "https://example.test/apply"}
		expect(t, a, "POST", offers, offer, 403)
		offerID := expect(t, dean, "POST", offers, offer, 201)["id"].(string)
		if got := expect(t, a, "GET", offers, nil, 200)["items"].([]any); len(got) != 1 || got[0].(map[string]any)["url"] != offer["url"] {
			t.Fatalf("Предложение не видно студенту факультета: %v", got)
		}
		offer["title"] = "Обновлённое предложение"
		expect(t, dean, "PUT", offers+"/"+offerID, offer, 204)
		expect(t, dean, "DELETE", offers+"/"+offerID, nil, 204)

		var previous time.Time
		err := db.QueryRow(ctx, `SELECT "studentEnrollmentDate" FROM students WHERE "studentNumber"=2026001`).Scan(&previous)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec(ctx, `UPDATE students SET "studentEnrollmentDate"=$1 WHERE "studentNumber"=2026001`, previous)
		})
		setYear := func(year int) {
			_, err := db.Exec(ctx, `UPDATE students SET "studentEnrollmentDate"=
				date_trunc('year', (now() AT TIME ZONE 'Europe/Moscow')-interval '8 months')+interval '8 months'-make_interval(years=>$1)
				WHERE "studentNumber"=2026001`, year-1)
			if err != nil {
				t.Fatal(err)
			}
		}
		input := map[string]any{"studentNumber": 2026001, "organization": "Организация", "startsOn": "2026-11-20", "endsOn": "2026-12-01", "reportDueOn": "2026-12-03"}
		setYear(2)
		if expect(t, a, "GET", "/career/eligibility", nil, 200)["practiceAvailable"] != false {
			t.Fatal("Практика доступна до третьего курса")
		}
		expect(t, dean, "POST", practices, input, 404)
		setYear(3)
		if expect(t, a, "GET", "/career/eligibility", nil, 200)["studyYear"] != float64(3) {
			t.Fatal("Не определён третий курс")
		}
		practiceID := expect(t, dean, "POST", practices, input, 201)["id"].(string)
		expect(t, dean, "POST", practices, input, 409)
		reportPath := practices + "/" + practiceID + "/report"
		expect(t, b, "GET", reportPath, nil, 404)
		expect(t, a, "GET", reportPath, nil, 404)
		upload := func() {
			var body bytes.Buffer
			multipartBody := multipart.NewWriter(&body)
			file, err := multipartBody.CreateFormFile("file", "report.pdf")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = file.Write([]byte("%PDF-1.4\n1 0 obj <<>> endobj\n%%EOF\n"))
			if err = multipartBody.Close(); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", prefix+reportPath, &body)
			r.RemoteAddr = remoteAddr
			r.Header.Set("Content-Type", multipartBody.FormDataContentType())
			r.Header.Set("Authorization", "Bearer "+a.token)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 204 {
				t.Fatalf("Отчёт не загружен: %d %s", w.Code, w.Body.String())
			}
		}
		if os.Getenv("TEST_CAREER_REPORT_UPLOAD") == "1" {
			upload()
			expect(t, dean, "GET", reportPath, nil, 200)
			expect(t, a, "GET", reportPath, nil, 200)
			reviewPath := practices + "/" + practiceID + "/review"
			expect(t, dean, "PUT", reviewPath, map[string]string{"state": "revision", "feedback": "Добавьте выводы"}, 204)
			upload()
			expect(t, dean, "PUT", reviewPath, map[string]string{"state": "accepted"}, 204)
		} else {
			expect(t, dean, "PUT", practices+"/"+practiceID+"/review", map[string]string{"state": "accepted"}, 409)
		}
	})
	t.Run("поддержка, консультация и изоляция переписки", func(t *testing.T) {
		remoteAddr = "127.0.0.3:1000"
		t.Cleanup(func() { remoteAddr = "127.0.0.1:1000" })
		const articles = "/support/articles"
		const tickets = "/support/tickets"
		draft := map[string]string{"question": "Когда работает поддержка?", "answer": "Тестовый ответ"}
		expect(t, a, "POST", articles, draft, 403)
		articleID := expect(t, support, "POST", articles, draft, 201)["id"].(string)
		draft["answer"] = "Вопросы можно отправлять через форму"
		expect(t, support, "PUT", articles+"/"+articleID, draft, 204)
		found := false
		for _, value := range expect(t, a, "GET", articles, nil, 200)["items"].([]any) {
			item := value.(map[string]any)
			if item["id"] == articleID && item["answer"] == draft["answer"] {
				found = true
			}
		}
		if !found {
			t.Fatal("Студент не видит опубликованную статью")
		}
		expect(t, support, "DELETE", articles+"/"+articleID, nil, 204)
		if people := expect(t, a, "GET", "/support/operators", nil, 200)["items"].([]any); len(people) != 1 {
			t.Fatalf("Ожидался один тестовый оператор: %v", people)
		}

		input := map[string]string{"category": "consultation", "priority": "normal", "message": "Могу в среду после пары"}
		ticketID := expect(t, a, "POST", tickets, input, 201)["id"].(string)
		path := tickets + "/" + ticketID
		expect(t, b, "GET", path, nil, 404)
		expect(t, b, "POST", path+"/messages", map[string]string{"message": "Чужой ответ"}, 409)
		expect(t, teacher, "GET", path, nil, 403)
		ticket := expect(t, support, "GET", path, nil, 200)
		if ticket["category"] != "consultation" || ticket["assigned"] != false || len(ticket["messages"].([]any)) != 1 {
			t.Fatalf("Оператор не видит новый запрос на консультацию: %v", ticket)
		}
		expect(t, support, "POST", path+"/messages", map[string]string{"message": "Подойдёт среда в 15:00?"}, 204)
		expect(t, a, "POST", path+"/messages", map[string]string{"message": "Да, спасибо"}, 204)
		expect(t, a, "POST", path+"/messages", map[string]any{"message": "Подтверждаю", "close": true}, 422)
		expect(t, support, "POST", path+"/messages", map[string]any{"message": "Время согласовано", "close": true}, 204)
		ticket = expect(t, a, "GET", path, nil, 200)
		if ticket["closedAt"] == nil || ticket["studentName"] != nil || len(ticket["messages"].([]any)) != 4 {
			t.Fatalf("Не сохранилась история консультации: %v", ticket)
		}
		expect(t, a, "POST", path+"/messages", map[string]string{"message": "Закрытое обращение"}, 409)
	})
	t.Run("личное портфолио и отзыв публичной ссылки", func(t *testing.T) {
		remoteAddr = "127.0.0.6:1000"
		t.Cleanup(func() { remoteAddr = "127.0.0.1:1000" })
		const path = "/portfolio"
		const published = "/public/portfolios/"
		expect(t, client{}, "GET", path, nil, 401)
		expect(t, dean, "GET", path, nil, 403)
		empty := expect(t, a, "GET", path, nil, 200)
		if empty["shareID"] != nil || len(empty["items"].([]any)) != 0 {
			t.Fatal("Новое портфолио должно быть пустым и личным")
		}
		bio := map[string]string{"bio": "Занимаюсь студенческими проектами"}
		expect(t, a, "PUT", path, bio, 204)
		body := map[string]string{"kind": "project", "title": "Открытый проект", "description": "Описание работы", "url": "https://example.test/project"}
		id := expect(t, a, "POST", path+"/items", body, 201)["id"].(string)
		expect(t, b, "PUT", path+"/items/"+id, body, 404)
		expect(t, b, "DELETE", path+"/items/"+id, nil, 404)
		if got := expect(t, b, "GET", path, nil, 200); len(got["items"].([]any)) != 0 {
			t.Fatal("Видна чужая карточка")
		}
		expect(t, client{}, "GET", published+id, nil, 404)
		body["url"] = "javascript:alert(1)"
		expect(t, a, "PUT", path+"/items/"+id, body, 422)
		body["url"] = "https://example.test/project"
		body["title"] = "Обновлённый проект"
		expect(t, a, "PUT", path+"/items/"+id, body, 204)
		publish := map[string]bool{"published": true}
		shareID := expect(t, a, "PUT", path+"/sharing", publish, 200)["shareID"].(string)
		if repeat := expect(t, a, "PUT", path+"/sharing", publish, 200); repeat["shareID"] != shareID {
			t.Fatal("Повторная публикация сменила действующую ссылку")
		}
		code, public, w := call(client{}, "GET", published+shareID, nil)
		if code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("Публичный ответ: %d %s", code, w.Body.String())
		}
		if len(public) != 3 || public["displayName"] == nil || public["bio"] != bio["bio"] || public["items"].([]any)[0].(map[string]any)["title"] != body["title"] {
			t.Fatalf("Неверный состав публичного ответа: %v", public)
		}
		for _, resolved := range []struct {
			c    client
			path string
		}{{a, path + "/resume.pdf"}, {client{}, published + shareID + "/resume.pdf"}, {a, "/academic/transcript.pdf?year=1"}} {
			code, _, pdf := call(resolved.c, "GET", resolved.path, nil)
			if code != 200 || pdf.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF-")) {
				t.Fatalf("Не сформирован PDF %s: %d %s", resolved.path, code, pdf.Body.String())
			}
		}
		expect(t, a, "PUT", path+"/sharing", map[string]bool{"published": false}, 200)
		expect(t, client{}, "GET", published+shareID, nil, 404)
		expect(t, client{}, "GET", published+shareID+"/resume.pdf", nil, 404)
		nextID := expect(t, a, "PUT", path+"/sharing", publish, 200)["shareID"].(string)
		if nextID == shareID {
			t.Fatal("Отключённая ссылка восстановилась")
		}
		expect(t, client{}, "GET", published+shareID, nil, 404)
		expect(t, a, "DELETE", path+"/items/"+id, nil, 204)
		if got := expect(t, client{}, "GET", published+nextID, nil, 200); len(got["items"].([]any)) != 0 {
			t.Fatal("Удалённая карточка осталась публичной")
		}
		expect(t, a, "PUT", path+"/sharing", map[string]bool{"published": false}, 200)
	})
	t.Run("кабинеты сотрудников, материалы и аудит", func(t *testing.T) {
		remoteAddr = "127.0.0.7:1000"
		t.Cleanup(func() { remoteAddr = "127.0.0.1:1000" })
		for _, check := range []struct {
			client client
			path   string
		}{
			{teacher, "/staff/teacher/groups"}, {teacher, "/staff/teacher/roster?courseID=" + id(40)},
			{dean, "/staff/dean/reports"}, {dean, "/staff/dean/requests"},
			{support, "/staff/support/analytics"}, {support, "/staff/directories"},
			{support, "/staff/support/colleagues"}, {support, "/staff/support/faculties"},
			{admin, "/admin/accounts"}, {admin, "/admin/persons"}, {admin, "/admin/audit"}, {admin, "/admin/overview"},
		} {
			expect(t, check.client, "GET", check.path, nil, 200)
			expect(t, a, "GET", check.path, nil, 403)
		}
		groups := expect(t, teacher, "GET", "/staff/teacher/groups", nil, 200)["items"].([]any)
		if len(groups) < 2 {
			t.Fatal("Связи групп преподавателя отсутствуют")
		}
		group := groups[0].(map[string]any)["courseGroupID"].(string)
		val := map[string]any{"courseGroupID": group, "title": "Методичка", "description": "Проверьте главу 2", "url": "https://example.test/handout"}
		material := expect(t, teacher, "POST", "/learning/materials", val, 201)["id"].(string)
		if len(expect(t, a, "GET", "/learning/materials?courseID="+id(40), nil, 200)["items"].([]any)) != 1 ||
			len(expect(t, b, "GET", "/learning/materials?courseID="+id(40), nil, 200)["items"].([]any)) != 0 {
			t.Fatal("Неверная область видимости материала")
		}
		expect(t, a, "DELETE", "/learning/materials/"+material, nil, 403)
		expect(t, teacher, "PUT", "/learning/materials/"+material, map[string]string{"title": "Редакция 2", "description": "Новое описание", "url": "https://example.test/new"}, 204)
		expect(t, teacher, "DELETE", "/learning/materials/"+material, nil, 204)
		draft := expect(t, teacher, "POST", "/learning/materials", map[string]any{
			"courseGroupID": group, "title": "Материал с файлом", "description": "Курс", "waitForFile": true,
		}, 201)["id"].(string)
		filePath := "/learning/materials/" + draft + "/file"
		expect(t, a, "GET", filePath, nil, 404)
		if os.Getenv("TEST_CAREER_REPORT_UPLOAD") == "1" {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			f, err := form.CreateFormFile("file", "material.pdf")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = f.Write([]byte("%PDF-1.4\n1 0 obj <<>> endobj\n%%EOF\n"))
			if err = form.Close(); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", prefix+filePath, &body)
			r.RemoteAddr = remoteAddr
			r.Header.Set("Content-Type", form.FormDataContentType())
			r.Header.Set("Authorization", "Bearer "+teacher.token)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 204 {
				t.Fatalf("Материал не загружен: %d %s", w.Code, w.Body.String())
			}
			expect(t, a, "GET", filePath, nil, 200)
			expect(t, b, "GET", filePath, nil, 404)
		}
		expect(t, teacher, "DELETE", "/learning/materials/"+draft, nil, 204)
		building := expect(t, admin, "POST", "/admin/buildings", map[string]string{"building": "Тестовый корпус роли", "address": "Тестовый адрес"}, 201)["id"].(string)
		room := fmt.Sprint(expect(t, admin, "POST", "/admin/classrooms", map[string]any{"buildingID": building, "number": 501}, 201)["id"])
		expect(t, admin, "POST", "/admin/classrooms", map[string]any{"buildingID": building, "number": 501}, 409)
		expect(t, admin, "PUT", "/admin/classrooms/"+room, map[string]any{"buildingID": building, "number": 502}, 204)
		expect(t, admin, "PATCH", "/admin/accounts/"+id(120), map[string]bool{"isActive": false}, 200)
		expect(t, teacher, "GET", "/staff/teacher/groups", nil, 401)
		audit := expect(t, admin, "GET", "/admin/audit", nil, 200)["items"].([]any)
		if len(audit) == 0 || audit[0].(map[string]any)["nextActive"] != false {
			t.Fatal("Отключение не попало в журнал")
		}
		expect(t, admin, "PATCH", "/admin/accounts/"+id(120), map[string]bool{"isActive": true}, 200)
		teacher = login("teacher@example.test")
	})
	t.Run("без авторизации и подмена роли", func(t *testing.T) {
		expect(t, client{}, "GET", "/profile/me", nil, 401)
		expect(t, a, "POST", "/auth/login", map[string]any{"email": "student1@example.test", "password": "bad", "role": "ADMIN"}, 400)
		expect(t, a, "GET", "/faculty/students", nil, 403)
	})
	t.Run("свободные аудитории по интервалу", func(t *testing.T) {
		ctx := context.Background()
		var lesson, exam time.Time
		if err := db.QueryRow(ctx, `SELECT "startAt" FROM lessons WHERE "lessonID"=1`).Scan(&lesson); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(ctx, `SELECT "examStartAt" FROM exams WHERE "examID"=$1::uuid`, id(91)).Scan(&exam); err != nil {
			t.Fatal(err)
		}
		path := func(start, end time.Time) string {
			return "/campus/rooms?" + neturl.Values{"buildingID": {id(60)}, "from": {start.Format(time.RFC3339)}, "to": {end.Format(time.RFC3339)}}.Encode()
		}
		available := func(start, end time.Time, want bool) {
			t.Helper()
			result := expect(t, a, "GET", path(start, end), nil, 200)
			items := result["items"].([]any)
			if len(items) != 1 || items[0].(map[string]any)["available"] != want {
				t.Fatalf("ожидалось available=%t: %v", want, result)
			}
		}
		available(lesson.Add(-30*time.Minute), lesson, true)
		available(lesson, lesson.Add(time.Minute), false)
		available(exam, exam.Add(time.Minute), false)
		sport := time.Now().Add(45 * 24 * time.Hour)
		expect(t, dean, "POST", "/study/sports", map[string]any{"facultyID": id(1), "teacherID": id(20), "buildingID": id(60),
			"title": "Занятие для проверки аудитории", "room": "Аудитория 101", "capacity": 8, "startAt": sport, "endAt": sport.Add(time.Hour)}, 200)
		available(sport, sport.Add(time.Minute), false)
		expect(t, a, "GET", path(lesson, lesson.Add(25*time.Hour)), nil, 422)
		expect(t, admin, "GET", path(lesson, lesson.Add(time.Minute)), nil, 403)
	})

	t.Run("закрытое фото, QR и номер зачётной книжки", func(t *testing.T) {
		const photo = "/profile/me/photo"
		_, err := db.Exec(context.Background(), `INSERT INTO student_record_books ("studentID","recordBookNumber")
          VALUES($1::uuid,'ТЕСТ-001') ON CONFLICT ("studentID") DO NOTHING`, id(10))
		if err != nil {
			t.Fatal(err)
		}
		if got := expect(t, a, "GET", "/profile/me", nil, 200)["recordBookNumber"]; got != "ТЕСТ-001" {
			t.Fatalf("неверный номер зачётной книжки: %v", got)
		}
		if got := expect(t, b, "GET", "/profile/me", nil, 200)["recordBookNumber"]; got != nil {
			t.Fatal("выдан номер другого студента")
		}
		expect(t, b, "GET", photo, nil, 204)
		expect(t, teacher, "GET", photo, nil, 403)
		upload := func(raw []byte, want int) *httptest.ResponseRecorder {
			t.Helper()
			var body bytes.Buffer
			mp := multipart.NewWriter(&body)
			part, err := mp.CreateFormFile("file", "portrait.png")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = part.Write(raw); err != nil {
				t.Fatal(err)
			}
			if err = mp.Close(); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("PUT", prefix+photo, &body)
			r.Header.Set("Content-Type", mp.FormDataContentType())
			r.Header.Set("Authorization", "Bearer "+a.token)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatalf("загрузка фото: %d, ожидалось %d", w.Code, want)
			}
			return w
		}
		upload([]byte("не изображение"), 422)
		picture := image.NewRGBA(image.Rect(0, 0, 640, 550))
		picture.Set(320, 275, color.White)
		var pngBytes bytes.Buffer
		if err = png.Encode(&pngBytes, picture); err != nil {
			t.Fatal(err)
		}
		uploaded := upload(pngBytes.Bytes(), 200)
		jpg, err := jpeg.Decode(uploaded.Body)
		if err != nil || jpg.Bounds().Dx() != 512 || jpg.Bounds().Dy() != 512 {
			t.Fatalf("аватар должен быть JPEG 512×512: %v", err)
		}
		status, _, file := call(a, "GET", photo, nil)
		if status != 200 || file.Header().Get("Content-Type") != "image/jpeg" {
			t.Fatal("фото студента не возвращается")
		}
		expect(t, b, "GET", photo, nil, 204)
		status, _, code := call(a, "GET", "/profile/me/qr", nil)
		if status != 200 || code.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("QR-профиль: %d", status)
		}
		if _, err = png.Decode(code.Body); err != nil {
			t.Fatal(err)
		}
		expect(t, client{}, "GET", "/profile/me/qr", nil, 401)
	})

	t.Run("учебный план и физкультура", func(t *testing.T) {
		var year, semester int
		var start, previousTermStart time.Time
		err := db.QueryRow(context.Background(), `WITH st AS (`+studyStudentTerm+`)
        SELECT year+1-semester%2,semester+1,study_term_start(year+1-semester%2,semester+1),study_term_start(year-semester%2,semester-1) FROM st`, id(10)).Scan(&year, &semester, &start, &previousTermStart)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		p := expect(t, dean, "POST", "/study/periods", map[string]any{"facultyID": id(1), "academicYear": year, "semester": semester, "opensAt": now.Add(-time.Hour), "closesAt": now.Add(time.Hour)}, 200)
		pid := p["periodID"].(string)
		path := "/study/periods/" + pid
		expect(t, a, "GET", "/study/management", nil, 403)
		expect(t, dean, "GET", "/study/management", nil, 200)
		course := func(title, kind, direction string) string {
			return expect(t, dean, "POST", path+"/requirements", map[string]any{"title": title, "kind": kind, "directionID": direction, "hours": 32}, 200)["courseID"].(string)
		}
		common := course("Общая проверочная дисциплина", "common", "")
		profile := course("Профильная проверочная дисциплина", "profile", id(2))
		third := course("Вторая общая проверочная дисциплина", "common", "")
		course("Физкультура следующего семестра", "physical_education", "")
		expect(t, dean, "PATCH", path, map[string]string{"action": "publish"}, 409)
		stream := func(course string, at time.Time, capacity int) string {
			return expect(t, dean, "POST", path+"/streams", map[string]any{"courseID": course, "teacherID": id(20), "classroomID": 1, "capacity": capacity, "startAt": at, "endAt": at.Add(time.Hour), "weeks": 2}, 200)["streamID"].(string)
		}
		first := start.Add(72 * time.Hour)
		one := stream(common, first, 1)
		two := stream(profile, first.Add(time.Hour), 5)
		three := stream(third, first.Add(2*time.Hour), 5)
		expect(t, dean, "PATCH", path, map[string]string{"action": "publish"}, 200)
		plan := expect(t, a, "GET", "/study/selection", nil, 200)
		if plan["periodID"] != pid || len(plan["requirements"].([]any)) != 3 {
			t.Fatalf("unexpected plan: %v", plan)
		}
		selection := func(ids ...string) map[string]any { return map[string]any{"periodID": pid, "streamIDs": ids} }
		expect(t, a, "PUT", "/study/selection", selection(one), 409)
		expect(t, a, "PUT", "/study/selection", selection(one, one), 409)
		// An existing mandatory lesson in the student's main group blocks the entire set.
		_, err = db.Exec(context.Background(), `INSERT INTO lessons ("lessonID","groupID","classroomID","teacherID",topic,"startAt","endAt","lessonStatus","courseID")
          VALUES (900000,1,1,$1::uuid,'Проверка пересечения',$2,$3,'active',$4::uuid)`, id(20), first.Add(30*time.Minute), first.Add(45*time.Minute), id(40))
		if err != nil {
			t.Fatal(err)
		}
		expect(t, a, "PUT", "/study/selection", selection(one, two, three), 409)
		// The same occupied slot is available as walk-in sport. It does not
		// become part of the required academic stream selection.
		sportAt := first.Add(30 * time.Minute)
		sport := expect(t, dean, "POST", "/study/sports", map[string]any{
			"facultyID": id(1), "teacherID": id(20), "buildingID": id(60), "title": "Бадминтон",
			"capacity": 24, "startAt": sportAt, "endAt": sportAt.Add(time.Hour),
		}, 200)["sportID"]
		pe := expect(t, a, "GET", "/study/physical-education", nil, 200)
		visible := false
		for _, item := range pe["sessions"].([]any) {
			if item.(map[string]any)["sportID"] == sport {
				visible = true
			}
		}
		if !visible {
			t.Fatal("спортивное занятие, совпадающее с парой, должно быть доступно")
		}

		if _, err = db.Exec(context.Background(), `DELETE FROM lessons WHERE "lessonID"=900000`); err != nil {
			t.Fatal(err)
		}
		var concurrent sync.WaitGroup
		results := make(chan int, 2)
		for _, student := range []client{a, b} {
			concurrent.Add(1)
			go func() {
				defer concurrent.Done()
				code, _, w := call(student, "PUT", "/study/selection", selection(one, two, three))
				if code != 200 && code != 409 {
					t.Errorf("selection response %d: %s", code, w.Body.String())
				}
				results <- code
			}()
		}
		concurrent.Wait()
		if x, y := <-results, <-results; x+y != 609 {
			t.Fatalf("one seat: %d %d", x, y)
		}
		var owner string
		if err = db.QueryRow(context.Background(), `SELECT "studentID"::text FROM student_stream_selections WHERE "streamID"=$1::uuid`, one).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		winner := a
		if owner == id(11) {
			winner = b
		}
		expect(t, winner, "PUT", "/study/selection", selection(one, two, three), 200)
		day := first.In(time.FixedZone("MSK", 10800)).Format("2006-01-02")
		v := expect(t, winner, "GET", "/schedule?date="+day, nil, 200)
		if len(v["lessons"].([]any)) != 3 {
			t.Fatalf("selected lessons unavailable: %v", v)
		}
		expect(t, dean, "PATCH", path, map[string]string{"action": "close"}, 200)
		expect(t, winner, "PUT", "/study/selection", selection(one, two, three), 409)
		sessions := []string{}
		// Last week's attendance is assigned by the time of the actual lesson.
		week := now.In(time.FixedZone("MSK", 10800)).Truncate(time.Minute)
		week = time.Date(week.Year(), week.Month(), week.Day(), 12, 0, 0, 0, week.Location()).AddDate(0, 0, -(int(week.Weekday())+6)%7-7)
		for i := 0; i < 3; i++ {
			at := week.AddDate(0, 0, i)
			sessions = append(sessions, expect(t, dean, "POST", "/study/sports", map[string]any{"facultyID": id(1), "teacherID": id(20), "buildingID": id(60), "title": "Тестовое занятие", "capacity": 24, "startAt": at, "endAt": at.Add(time.Hour)}, 200)["sportID"].(string))
		}
		expect(t, teacher, "GET", "/study/attendance", nil, 200)
		attendance := "/study/attendance/"
		roster := expect(t, teacher, "GET", attendance+sessions[0]+"?search=2026001", nil, 200)
		if len(roster["students"].([]any)) != 1 {
			t.Fatalf("wrong roster: %v", roster)
		}
		mark := func(who client, i int, present bool, status int) {
			expect(t, who, "PUT", attendance+sessions[i], map[string]any{"studentID": id(10), "present": present}, status)
		}
		mark(a, 0, true, 403)
		mark(teacher, 0, true, 200)
		mark(teacher, 0, true, 200)
		mark(teacher, 1, true, 200)
		mark(teacher, 2, true, 409)
		mark(teacher, 0, false, 200)
		mark(teacher, 2, true, 200)
		expect(t, a, "GET", "/study/physical-education", nil, 200)

		// A complete previous semester lets the journal verify the threshold at any date.
		previousYear := previousTermStart.In(time.FixedZone("MSK", 10800)).AddDate(0, -8, 0).Year()
		pePlan := expect(t, dean, "POST", "/study/periods", map[string]any{"facultyID": id(1), "academicYear": previousYear, "semester": semester - 2, "opensAt": now.Add(-time.Hour), "closesAt": now.Add(time.Hour)}, 200)
		pePath := "/study/periods/" + pePlan["periodID"].(string)
		peCourse := expect(t, dean, "POST", pePath+"/requirements", map[string]any{"title": "Физкультура прошедшего семестра", "kind": "physical_education", "hours": 40}, 200)["courseID"].(string)
		expect(t, dean, "PATCH", pePath, map[string]string{"action": "publish"}, 200)
		score := func(points int, credit bool) {
			t.Helper()
			result := expect(t, a, "GET", "/academic/subjects/"+peCourse+"/results", nil, 200)
			rows := result["scores"].([]any)
			if len(rows) != 1 {
				t.Fatalf("ожидалась одна запись по физкультуре: %v", rows)
			}
			row := rows[0].(map[string]any)
			if row["score"] != float64(points) || row["credit"] != credit {
				t.Fatalf("ожидалось %d/%t: %v", points, credit, row)
			}
		}
		monday := previousTermStart.In(time.FixedZone("MSK", 10800)).AddDate(0, 0, 7)
		monday = monday.AddDate(0, 0, -(int(monday.Weekday())+6)%7)
		var last string
		for i := 0; i < 20; i++ {
			at := monday.AddDate(0, 0, i/2*7+i%2).Add(12 * time.Hour)
			sport := expect(t, dean, "POST", "/study/sports", map[string]any{"facultyID": id(1), "teacherID": id(20), "buildingID": id(60), "title": "Учебный журнал", "capacity": 24, "startAt": at, "endAt": at.Add(time.Hour)}, 200)["sportID"].(string)
			last = attendance + sport
			expect(t, teacher, "PUT", last, map[string]any{"studentID": id(10), "present": true}, 200)
			if i == 18 {
				score(95, false)
			}
		}
		score(100, true)
		expect(t, teacher, "PUT", last, map[string]any{"studentID": id(10), "present": false}, 200)
		score(95, false)

	})

	t.Run("календарь спорта и атомарное бронирование", func(t *testing.T) {
		loc := time.FixedZone("MSK", 10800)
		today := time.Now().In(loc)
		start := time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, 7-(int(today.Weekday())+6)%7)
		week := start.Format("2006-01-02")
		slot := func(title, room string, status int) map[string]any {
			return expect(t, dean, "POST", "/study/sports", map[string]any{"facultyID": id(1), "teacherID": id(20), "buildingID": id(60),
				"title": title, "room": room, "capacity": 1, "startAt": start, "endAt": start.Add(time.Hour)}, status)
		}
		first := slot("Теннис для календаря", "Зал 1", 200)["sportID"].(string)
		slot("Бадминтон для календаря", "зал 1", 409)
		second := slot("Бадминтон для календаря", "Зал 2", 200)["sportID"].(string)
		endpoint := "/study/sport-calendar"
		view := expect(t, a, "GET", endpoint+"?week="+week, nil, 200)
		if view["week"] != week || view["defaultBuildingID"] != id(60) {
			t.Fatalf("неверные настройки недели: %v", view)
		}
		matches := 0
		for _, raw := range view["sessions"].([]any) {
			item := raw.(map[string]any)
			if item["sportID"] == first || item["sportID"] == second {
				if item["room"] == nil || item["reservable"] != true {
					t.Fatalf("нет зала или занятия недоступны: %v", item)
				}
				matches++
			}
		}
		if matches != 2 {
			t.Fatalf("в календаре должны быть оба параллельных занятия")
		}
		save := func(who client, status int, ids []string) {
			expect(t, who, "PUT", endpoint, map[string]any{"week": week, "sportIDs": ids}, status)
		}
		later := func(title string, delta time.Duration) string {
			at := start.Add(delta)
			return expect(t, dean, "POST", "/study/sports", map[string]any{"facultyID": id(1), "teacherID": id(20), "buildingID": id(60),
				"title": title, "room": "Зал 3", "capacity": 12, "startAt": at, "endAt": at.Add(time.Hour)}, 200)["sportID"].(string)
		}
		third, fourth := later("Третий слот", 2*time.Hour), later("Четвёртый слот", 4*time.Hour)
		save(a, 422, []string{first, third, fourth})
		save(a, 200, []string{third, fourth})
		expect(t, a, "POST", "/sports/"+first+"/registrations", nil, 409)
		save(a, 200, []string{})
		save(a, 409, []string{first, second})
		save(a, 200, []string{first})
		save(b, 409, []string{first})
		save(a, 200, []string{})
		save(b, 200, []string{first})
		fresh := expect(t, b, "GET", endpoint+"?week="+week, nil, 200)
		found := false
		for _, raw := range fresh["sessions"].([]any) {
			item := raw.(map[string]any)
			if item["sportID"] == first && item["reserved"] == true && item["remaining"] == float64(0) {
				found = true
			}
		}
		if !found {
			t.Fatal("подтверждённая запись не вернулась в календарь")
		}
	})
	t.Run("расписание спорта на весь семестр", func(t *testing.T) {
		loc := time.FixedZone("MSK", 10800)
		now := time.Now().In(loc)
		year := now.Year()
		term := 2
		if now.Month() < 9 {
			term = 1
			if now.Month() < 2 {
				year--
			}
		}
		if _, err := db.Exec(context.Background(), `INSERT INTO classroom ("classroomID","buildingID","classroomNumber") VALUES(3812,$1::uuid,812)`, id(60)); err != nil {
			t.Fatal(err)
		}
		endpoint := "/study/sport-semesters"
		expect(t, a, "GET", endpoint+"?facultyID="+id(1), nil, 403)
		plan := func(title string, room int) map[string]any {
			return map[string]any{"facultyID": id(1), "academicYear": year, "term": term, "title": title, "slots": []map[string]any{
				{"weekday": 2, "startTime": "08:10", "endTime": "09:40", "classroomID": room, "teacherID": id(20), "capacity": 12},
				{"weekday": 4, "startTime": "08:10", "endTime": "09:40", "classroomID": room, "teacherID": id(20), "capacity": 12},
			}}
		}
		one := expect(t, dean, "POST", endpoint, plan("Семестровый баскетбол", 1), 200)
		if one["sessions"].(float64) < 25 {
			t.Fatalf("семестр должен содержать еженедельные повторения: %v", one)
		}
		expect(t, dean, "POST", endpoint, plan("Семестровый тест другого спорта", 1), 409)
		expect(t, dean, "POST", endpoint, plan("Семестровый бадминтон", 3812), 200)
		catalog := expect(t, dean, "GET", endpoint+"?facultyID="+id(1), nil, 200)
		if len(catalog["plans"].([]any)) != 2 {
			t.Fatalf("неудачная публикация должна откатиться: %v", catalog)
		}
		var dates int
		if err := db.QueryRow(context.Background(), `SELECT count(*) FROM sports s JOIN sport_schedule_occurrences occ USING ("sportID")
            JOIN sport_weekly_slots slot USING ("slotID") JOIN sport_session_rooms room USING ("sportID")
            WHERE slot."scheduleID"=$1::uuid
            AND extract(isodow FROM s."sportStartAt" AT TIME ZONE 'Europe/Moscow')=slot.weekday
            AND to_char(s."sportStartAt" AT TIME ZONE 'Europe/Moscow','HH24:MI')='08:10'`, one["scheduleID"]).Scan(&dates); err != nil || float64(dates) != one["sessions"] {
			t.Fatalf("некорректно сгенерированные даты: %d %v", dates, err)
		}
	})

	t.Run("настройки и данные профиля", func(t *testing.T) {
		base := "/profile/me/settings"
		expect(t, a, "PUT", base+"/notifications", map[string]any{"muted": []string{"news"}}, 200)
		view := expect(t, a, "GET", base+"/notifications", nil, 200)
		if len(view["muted"].([]any)) != 1 {
			t.Fatalf("настройка не сохранилась: %v", view)
		}
		notices := expect(t, a, "GET", "/notifications", nil, 200)
		if notices["total"] != float64(0) {
			t.Fatalf("выключенная категория видна: %v", notices)
		}
		expect(t, a, "PUT", base+"/notifications", map[string]any{"muted": []string{"unknown"}}, 422)
		expect(t, a, "PUT", base+"/notifications", map[string]any{"muted": []string{}}, 200)
		doc := uuid.NewString()
		if _, err := db.Exec(context.Background(), `INSERT INTO student_documents ("documentID",title,version,body,"requiresConsent","publishedAt")
             VALUES($1::uuid,'Материал теста','1','Тест в изолированной базе',true,now())`, doc); err != nil {
			t.Fatal(err)
		}
		answer := base + "/documents/" + doc
		expect(t, a, "PUT", answer, map[string]bool{"accepted": true}, 200)
		expect(t, a, "PUT", answer, map[string]bool{"accepted": true}, 200)
		expect(t, a, "PUT", answer, map[string]bool{"accepted": false}, 200)
		var changes int
		if err := db.QueryRow(context.Background(), `SELECT count(*) FROM student_document_choices WHERE "documentID"=$1::uuid`, doc).Scan(&changes); err != nil || changes != 2 {
			t.Fatalf("неверная история: %d %v", changes, err)
		}
		expect(t, b, "GET", base+"/documents", nil, 200)

		extra := login("student1@example.test")
		_, _, payload := call(a, "GET", base+"/sessions", nil)
		var sessions []struct {
			SessionID string
			Current   bool
		}
		if err := json.Unmarshal(payload.Body.Bytes(), &sessions); err != nil || len(sessions) != 2 {
			t.Fatalf("неверные сеансы: %s", payload.Body.String())
		}
		for _, session := range sessions {
			if session.Current {
				expect(t, a, "DELETE", base+"/sessions/"+session.SessionID, nil, 409)
				expect(t, b, "DELETE", base+"/sessions/"+session.SessionID, nil, 404)
			} else {
				expect(t, a, "DELETE", base+"/sessions/"+session.SessionID, nil, 204)
			}
		}
		expect(t, extra, "GET", "/auth/me", nil, 401)
		extra = login("student1@example.test")
		old := os.Getenv("DEMO_PASSWORD")
		expect(t, extra, "PUT", base+"/password", map[string]string{"currentPassword": "incorrect", "newPassword": "Local-Only-New-Password"}, 422)
		expect(t, extra, "PUT", base+"/password", map[string]string{"currentPassword": old, "newPassword": "Local-Only-New-Password"}, 200)
		expect(t, a, "GET", "/auth/me", nil, 401)
		expect(t, extra, "GET", "/auth/me", nil, 200)
		expect(t, extra, "PUT", base+"/password", map[string]string{"currentPassword": "Local-Only-New-Password", "newPassword": old}, 200)
		a = login("student1@example.test")
	})

	t.Run("афиша, запись и подтверждённое посещение", func(t *testing.T) {
		start := time.Now().Add(24 * time.Hour)
		info := map[string]any{"title": "Волонтёрская встреча теста", "description": "Локальная проверка",
			"facultyID": id(1), "buildingID": id(60), "category": "volunteering", "capacity": 1, "teacherID": id(20),
			"startAt": start, "endAt": start.Add(90 * time.Minute)}
		expect(t, teacher, "POST", "/activities/events", info, 403)
		event := expect(t, dean, "POST", "/activities/events", info, 200)["eventID"].(string)
		roster := "/activities/events/" + event + "/registrations"
		expect(t, a, "GET", roster, nil, 403)
		expect(t, dean, "GET", "/activities/events", nil, 200)
		expect(t, a, "GET", "/activities", nil, 200)
		registration := "/activities/" + event + "/registration"
		reserve := map[string]bool{"registered": true}
		results := make(chan int, 2)
		for _, who := range []client{a, b} {
			go func(c client) { status, _, _ := call(c, "PUT", registration, reserve); results <- status }(who)
		}
		if x, y := <-results, <-results; x+y != 609 {
			t.Fatalf("одно место: %d и %d", x, y)
		}
		attendees := expect(t, support, "GET", roster, nil, 200)["students"].([]any)
		if len(attendees) != 1 {
			t.Fatalf("нужен один участник, получено %v", attendees)
		}
		first := a
		if attendees[0].(map[string]any)["studentID"] == id(11) {
			first = b
		}
		expect(t, first, "PUT", registration, map[string]bool{"registered": false}, 200)
		expect(t, a, "PUT", registration, reserve, 200)
		ticket := "/activities/" + event + "/ticket"
		expect(t, b, "GET", ticket, nil, 404)
		expect(t, dean, "GET", ticket+".pdf", nil, 403)
		status, _, image := call(a, "GET", ticket, nil)
		if status != 200 || image.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("QR: http %d", status)
		}
		if _, err := png.Decode(image.Body); err != nil {
			t.Fatal(err)
		}
		codeStatus, _, pdf := call(a, "GET", ticket+".pdf", nil)
		if codeStatus != 200 || pdf.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF")) {
			t.Fatalf("некорректный билет PDF: http %d", codeStatus)
		}
		readTicket := func() string {
			var value string
			if err := db.QueryRow(context.Background(), `SELECT "ticketID"::text FROM campus_event_registrations WHERE "eventID"=$1::uuid AND "studentID"=$2::uuid`, event, id(10)).Scan(&value); err != nil {
				t.Fatal(err)
			}
			return activityTicketPrefix + value
		}
		old := readTicket()
		expect(t, a, "PUT", registration, map[string]bool{"registered": false}, 200)
		expect(t, a, "GET", ticket, nil, 404)
		expect(t, a, "PUT", registration, reserve, 200)
		value := readTicket()
		if old == value {
			t.Fatal("билет повторно использован после отмены")
		}
		list := expect(t, a, "GET", "/activities", nil, 200)["myTickets"].([]any)
		if len(list) != 1 {
			t.Fatal("билет отсутствует в личном списке")
		}
		scan := "/activities/events/" + event + "/check-in"
		expect(t, dean, "PUT", roster+"/"+id(10), map[string]bool{"present": true}, 403)
		expect(t, dean, "GET", roster, nil, 403)
		expect(t, teacher, "PUT", roster+"/"+id(10), map[string]bool{"present": true}, 403)
		expect(t, support, "PUT", roster+"/"+id(10), map[string]bool{"present": true}, 409)
		expect(t, teacher, "POST", scan, map[string]string{"code": value}, 409)
		if _, err := db.Exec(context.Background(), `UPDATE campus_events SET "startAt"=now()-interval '10 minutes',
           "endAt"=now()+interval '80 minutes' WHERE "eventID"=$1::uuid`, event); err != nil {
			t.Fatal(err)
		}
		expect(t, a, "POST", scan, map[string]string{"code": value}, 403)
		expect(t, support, "POST", scan, map[string]string{"code": value}, 403)
		expect(t, teacher, "POST", scan, map[string]string{"code": old}, 404)
		if _, err := db.Exec(context.Background(), `DELETE FROM campus_event_hosts WHERE "eventID"=$1::uuid`, event); err != nil {
			t.Fatal(err)
		}
		expect(t, teacher, "POST", scan, map[string]string{"code": value}, 404)
		expect(t, dean, "PUT", "/activities/events/"+event+"/host", map[string]string{"teacherID": id(20)}, 200)
		expect(t, teacher, "GET", "/activities/check-in", nil, 200)
		result := expect(t, teacher, "POST", scan, map[string]string{"code": value}, 200)
		if result["already"] != false {
			t.Fatal("первая отметка не записана")
		}
		result = expect(t, teacher, "POST", scan, map[string]string{"code": value}, 200)
		if result["already"] != true {
			t.Fatal("не распознана повторная отметка")
		}
		expect(t, support, "PUT", roster+"/"+id(10), map[string]bool{"present": false}, 200)
		expect(t, support, "PUT", roster+"/"+id(10), map[string]bool{"present": true}, 200)
		student := expect(t, a, "GET", "/activities", nil, 200)
		if student["volunteerMinutes"] != float64(90) {
			t.Fatalf("неверный учёт волонтёрства: %v", student["volunteerMinutes"])
		}
		stranger := expect(t, b, "GET", "/activities", nil, 200)
		if stranger["volunteerMinutes"] != float64(0) {
			t.Fatal("чужие подтверждения попали в паспорт")
		}
	})

	t.Run("чтение по всем ролям", func(t *testing.T) {
		day := time.Now().In(time.FixedZone("MSK", 10800)).Format("2006-01-02")
		for _, c := range []client{a, b, teacher, dean} {
			for _, path := range []string{"/auth/me", "/profile/me", "/schedule?date=" + day, "/schedule/period?from=" + day + "&to=" + day, "/schedule/changes", "/academic/subjects", "/academic/grades", "/academic/deadlines", "/academic/subjects/" + id(40) + "/results", "/academic/exams", "/notifications", "/courses", "/news", "/buildings", "/dashboard"} {
				status, _, w := call(c, "GET", path, nil)
				if status != 200 {
					t.Errorf("GET %s: %d %s", path, status, w.Body.String())
				}
			}
		}
		for _, path := range []string{"/profile/me", "/requests/unassigned", "/requests/assigned", "/notifications", "/news", "/buildings"} {
			expect(t, support, "GET", path, nil, 200)
		}
		expect(t, dean, "GET", "/faculty/students", nil, 200)
		expect(t, dean, "GET", "/faculty/teachers", nil, 200)
	})
	t.Run("профиль без секретов и изоляция", func(t *testing.T) {
		v := expect(t, a, "GET", "/profile/me", nil, 200)
		student := v["student"].(map[string]any)
		for _, field := range []string{"studentBirthday", "studentEnrollmentDate"} {
			if _, err := time.Parse(time.RFC3339, student[field].(string)); err != nil {
				t.Fatal("невалидное время "+field, err)
			}
		}
		if student["group"] == nil || student["faculty"] == nil || student["direction"] == nil {
			t.Fatal("профиль без учебных связей")
		}

		raw, _ := json.Marshal(v)
		for _, secret := range []string{"passport", "Password", "passwordHash"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Fatal("утечка " + secret)
			}
		}
		expect(t, a, "GET", "/lessons/2", nil, 403)
		expect(t, a, "GET", "/schedule?date="+time.Now().Format("2006-01-02")+"&groupID=2", nil, 403)
		expect(t, b, "GET", "/requests/"+id(81), nil, 403)
	})
	var requestID string
	t.Run("создание и обработка заявки", func(t *testing.T) {
		body := map[string]any{"appointmentTypeID": id(80), "appointmentTitle": "Справка", "appointmentDescription": "Интеграционная проверка"}
		v := expect(t, a, "POST", "/requests", body, 201)
		requestID = v["appoinmentID"].(string)
		expect(t, a, "GET", "/requests/"+requestID, nil, 200)
		expect(t, b, "GET", "/requests/"+requestID, nil, 403)
		expect(t, a, "PATCH", "/requests/"+requestID+"/status", map[string]any{"appointmentStatus": "ready"}, 403)
		expect(t, support, "POST", "/requests/"+requestID+"/claim", nil, 200)
		expect(t, support, "PATCH", "/requests/"+requestID+"/status", map[string]any{"appointmentStatus": "canceled"}, 422)
		expect(t, support, "PATCH", "/requests/"+requestID+"/status", map[string]any{"appointmentStatus": "ready"}, 200)
		expect(t, support, "PATCH", "/requests/"+requestID+"/status", map[string]any{"appointmentStatus": "proccess"}, 409)
		history := expect(t, a, "GET", "/requests/"+requestID+"/history", nil, 200)
		if history["total"].(float64) < 3 {
			t.Fatal("нет полной истории")
		}
	})
	t.Run("черновик отправляется один раз", func(t *testing.T) {
		v := expect(t, teacher, "POST", "/requests/drafts", map[string]any{}, 201)
		draft := v["appointmentDraftID"].(string)
		expect(t, a, "GET", "/requests/drafts/"+draft, nil, 403)
		expect(t, teacher, "PATCH", "/requests/drafts/"+draft, map[string]any{"appointmentTypeID": id(80), "appointmentTitle": "Вопрос", "appointmentDescription": "Описание"}, 200)
		expect(t, teacher, "POST", "/requests/drafts/"+draft+"/submit", nil, 201)
		expect(t, teacher, "POST", "/requests/drafts/"+draft+"/submit", nil, 409)
	})
	t.Run("порог зачёта и экзамен", func(t *testing.T) {
		for _, score := range []int{59, 60, 100} {
			v := expect(t, teacher, "PATCH", "/academic/grades/"+id(70), map[string]any{"score": score}, 200)
			if v["credit"] != (score >= 60) {
				t.Fatal("неправильный зачёт")
			}
		}
		expect(t, teacher, "PATCH", "/academic/grades/"+id(70), map[string]any{"score": 101}, 422)
		expect(t, a, "PATCH", "/academic/grades/"+id(70), map[string]any{"score": 100}, 403)
		status, _, w := call(teacher, "POST", "/academic/exams/"+id(91)+"/results", map[string]any{"studentID": id(10), "score": nil})
		if status != 201 && status != 409 {
			t.Fatal(w.Body.String())
		}
		expect(t, a, "GET", "/academic/exams/"+id(91)+"/results", nil, 200)
	})
	t.Run("расписание и уведомление", func(t *testing.T) {
		expect(t, support, "PATCH", "/lessons/1", map[string]any{"lessonStatus": "rescheduled", "reason": "Проверка изменения расписания"}, 200)
		expect(t, a, "GET", "/schedule/changes", nil, 200)
		expect(t, a, "PATCH", "/notifications/"+id(90)+"/read", nil, 200)
	})
	t.Run("последнее спортивное место атомарно", func(t *testing.T) {
		for _, c := range []client{a, b} {
			call(c, "DELETE", "/sports/"+id(92)+"/registrations/me", nil)
		}
		codes := make(chan int, 2)
		var wg sync.WaitGroup
		for _, c := range []client{a, b} {
			wg.Add(1)
			go func(c client) {
				defer wg.Done()
				status, _, _ := call(c, "POST", "/sports/"+id(92)+"/registrations", nil)
				codes <- status
			}(c)
		}
		wg.Wait()
		close(codes)
		success, conflict := 0, 0
		for status := range codes {
			if status == 201 || status == 200 {
				success++
			} else if status == 409 {
				conflict++
			} else {
				t.Errorf("неожиданный статус %d", status)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatalf("success=%d conflict=%d", success, conflict)
		}
	})
	t.Run("новости и курсы", func(t *testing.T) {
		v := expect(t, dean, "POST", "/courses", map[string]any{"facultyID": id(1), "title": "Курс " + uuid.NewString(), "hours": 36, "semester": 2}, 201)
		course := v["courseID"].(string)
		expect(t, dean, "PATCH", "/courses/"+course, map[string]any{"hours": 72}, 200)
		expect(t, dean, "DELETE", "/courses/"+course, nil, 403)
		v = expect(t, support, "POST", "/news", map[string]any{"newsTitle": "Проверка", "newsDescription": "Учебная новость", "newsCategory": "important"}, 201)
		news := "/news/" + fmt.Sprint(v["newsID"])
		if v["newsCategory"] != "important" {
			t.Fatal("категория не сохранилась")
		}
		expect(t, a, "PATCH", news, map[string]any{"newsCategory": "events"}, 403)
		expect(t, dean, "PATCH", news, map[string]any{"newsCategory": "events"}, 404)
		expect(t, support, "PATCH", news, map[string]any{"newsCategory": "unknown"}, 422)
		expect(t, support, "PATCH", news, map[string]any{"newsCategory": "campus"}, 200)
		v = expect(t, a, "GET", news, nil, 200)
		if v["newsCategory"] != "campus" || v["newsDescription"] != "Учебная новость" {
			t.Fatalf("неверная карточка: %v", v)
		}
		v = expect(t, support, "PATCH", news, map[string]any{"newsTitle": "Обновлена", "newsCategory": nil}, 200)
		if v["newsCategory"] != nil {
			t.Fatal("категория не снята")
		}
		own := expect(t, dean, "POST", "/news", map[string]any{"newsTitle": "Новость деканата", "newsDescription": "Тестовый текст", "deaneryID": 1, "newsCategory": "events"}, 201)
		v = expect(t, dean, "PATCH", "/news/"+fmt.Sprint(own["newsID"]), map[string]any{"newsCategory": "opportunities"}, 200)
		if v["newsCategory"] != "opportunities" {
			t.Fatal("деканат не обновил свою категорию")
		}
		expect(t, support, "DELETE", "/news/"+news, nil, 403)
	})
	t.Run("смена роли и отзыв сессии", func(t *testing.T) {
		expect(t, admin, "PATCH", "/admin/accounts/"+id(111), map[string]any{"accountRole": "ADMIN"}, 422)
		expect(t, admin, "PATCH", "/admin/accounts/"+id(111), map[string]any{"isActive": false}, 200)
		expect(t, b, "GET", "/profile/me", nil, 401)
		expect(t, admin, "PATCH", "/admin/accounts/"+id(111), map[string]any{"isActive": true}, 200)
	})
	t.Run("refresh rotation и немедленный logout", func(t *testing.T) {
		status, v, w := call(client{cookie: a.cookie}, "POST", "/auth/refresh", nil)
		if status != 200 {
			t.Fatal(w.Body.String())
		}
		rotated := client{v["accessToken"].(string), w.Result().Cookies()[0]}
		expect(t, client{cookie: a.cookie}, "POST", "/auth/refresh", nil, 401)
		expect(t, rotated, "POST", "/auth/logout", nil, 204)
		expect(t, rotated, "GET", "/auth/me", nil, 401)
		expect(t, a, "GET", "/auth/me", nil, 401)
	})
}
