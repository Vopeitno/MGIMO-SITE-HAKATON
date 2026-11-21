package api

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMessagesPair(t *testing.T) {
	a := "11111111-1111-4111-8111-111111111111"
	b := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	id, participants, err := messagesPair(b, a)
	if err != nil {
		t.Fatal(err)
	}
	want := uuid.NewSHA1(uuid.NameSpaceOID, []byte(a+":"+b)).String()
	if id != want || participants[0] != a || participants[1] != b {
		t.Fatalf("unexpected pair: %s %v", id, participants)
	}
	reverse, _, err := messagesPair(a, strings.ToUpper(b))
	if err != nil || reverse != id {
		t.Fatalf("pair is not canonical: %s %v", reverse, err)
	}
	for _, pair := range [][2]string{{a, a}, {a, "bad"}, {uuid.Nil.String(), b}} {
		if _, _, err := messagesPair(pair[0], pair[1]); err == nil {
			t.Errorf("accepted invalid pair %v", pair)
		}
	}
}
func TestMessagesPagination(t *testing.T) {
	cases := []struct {
		query         string
		limit, offset int64
		invalid       bool
	}{
		{"", 20, 0, false}, {"?limit=100&offset=100000", 100, 100000, false},
		{"?limit=0", 0, 0, true}, {"?limit=101", 0, 0, true}, {"?offset=-1", 0, 0, true},
		{"?offset=100001", 0, 0, true}, {"?limit=no", 0, 0, true}, {"?limit=", 0, 0, true},
		{"?limit=1&limit=2", 0, 0, true}, {"?offset=9223372036854775808", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			limit, offset, err := messagesPagination(httptest.NewRequest("GET", "/"+tc.query, nil))
			if (err != nil) != tc.invalid {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.invalid && (limit != tc.limit || offset != tc.offset) {
				t.Fatalf("got %d %d", limit, offset)
			}
		})
	}
}
func TestMessagesText(t *testing.T) {
	for _, text := range []string{"", " \n\t", strings.Repeat("я", 10001), string([]byte{0xff})} {
		if messagesText(text) {
			t.Errorf("accepted invalid text of length %d", len(text))
		}
	}
	for _, text := range []string{"Привет", strings.Repeat("я", 10000), "  текст\n"} {
		if !messagesText(text) {
			t.Errorf("rejected valid text of length %d", len(text))
		}
	}
}
func TestMessagesSerialization(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	original := privateMessage{"message", "conversation", "sender", "Привет", now, nil}
	raw, err := bson.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var restored privateMessage
	if err = bson.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.MessageID != original.MessageID || restored.Text != original.Text || !restored.CreatedAt.Equal(now) || restored.ReadAt != nil {
		t.Fatalf("invalid BSON round trip: %+v", restored)
	}
	raw, err = json.Marshal(restored)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value["messageID"] != "message" || value["readAt"] != nil {
		t.Fatalf("invalid JSON: %s", raw)
	}
	if _, exists := value["_id"]; exists {
		t.Fatal("MongoDB _id leaked into JSON")
	}
}
func TestInitMessagesUnavailable(t *testing.T) {
	if err := (&Server{}).InitMessages(t.Context()); err == nil {
		t.Fatal("expected unavailable MongoDB error")
	}
}

func TestMessagesRoutesAuthorizationAndUnavailable(t *testing.T) {
	routes := []struct{ method, path string }{
		{"GET", "/conversations"}, {"POST", "/conversations"},
		{"GET", "/conversations/11111111-1111-4111-8111-111111111111/messages"},
		{"POST", "/conversations/11111111-1111-4111-8111-111111111111/messages"},
		{"POST", "/conversations/11111111-1111-4111-8111-111111111111/read"},
	}
	mux := http.NewServeMux()
	(&Server{}).registerMessages(mux)
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			r := httptest.NewRequest(route.method, messagesPrefix+route.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != 401 {
				t.Fatalf("unauthenticated status: %d", w.Code)
			}
			for _, role := range []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"} {
				p := &Principal{AccountID: "11111111-1111-4111-8111-111111111111", Role: role}
				authenticated := r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
				w = httptest.NewRecorder()
				mux.ServeHTTP(w, authenticated)
				if w.Code != 503 {
					t.Fatalf("role %s: want unavailable, got %d: %s", role, w.Code, w.Body.String())
				}
			}
		})
	}
}

// Uses existing active PostgreSQL accounts and an isolated, disposable MongoDB database.
func TestMessagesIntegration(t *testing.T) {
	pgURI, mongoURI := os.Getenv("MESSAGES_TEST_POSTGRES_URI"), os.Getenv("MESSAGES_TEST_MONGO_URI")
	if pgURI == "" || mongoURI == "" {
		t.Skip("set MESSAGES_TEST_POSTGRES_URI and MESSAGES_TEST_MONGO_URI")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, pgURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT "accountID"::text FROM public.accounts WHERE "isActive"=true ORDER BY "accountID" LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatal("integration database requires two active accounts")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("mgimo_messages_test_" + strings.ReplaceAll(uuid.NewString(), "-", ""))
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.Drop(cleanup)
		_ = client.Disconnect(cleanup)
	})
	server := &Server{DB: pool, Mongo: db}
	if err = server.InitMessages(ctx); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server.registerMessages(mux)
	request := func(account, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, messagesPrefix+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r = r.WithContext(context.WithValue(ctx, principalKey{}, &Principal{AccountID: account, Role: "STUDENT"}))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	principalRequest := func(p *Principal, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, messagesPrefix+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r.WithContext(context.WithValue(ctx, principalKey{}, p)))
		return w
	}
	t.Run("групповые чаты и прочтение по участнику", func(t *testing.T) {
		lookup := func(email string) *Principal {
			t.Helper()
			p, err := scanPrincipal(pool.QueryRow(ctx, principalSQL+` WHERE a.email=$1`, email))
			if err != nil {
				t.Skip("нужны стандартные локальные тестовые аккаунты")
			}
			return p
		}
		student := lookup("student1@example.test")
		other := lookup("student2@example.test")
		teacher := lookup("teacher@example.test")
		support := lookup("support@example.test")
		call := func(p *Principal, method, path, body string, want int) []byte {
			t.Helper()
			response := principalRequest(p, method, path, body)
			if response.Code != want {
				t.Fatalf("%s %s: %d ожидалось %d: %s", method, path, response.Code, want, response.Body.String())
			}
			return response.Body.Bytes()
		}
		contacts := call(student, "GET", "/contacts", "", 200)
		if strings.Contains(string(contacts), "password") || strings.Contains(string(contacts), "@example.test") {
			t.Fatal("контакты содержат конфиденциальные поля")
		}
		if strings.Contains(string(contacts), other.AccountID) {
			t.Fatal("чужая группа попала в обзор контактов")
		}
		for _, person := range []struct {
			number    string
			principal *Principal
			role      string
		}{
			{"2026002", other, "STUDENT"}, {"2001", teacher, "TEACHER"}, {"300", support, "SUPPORT"},
		} {
			var page messagesPage[messagePerson]
			found := call(student, "GET", "/contacts?number="+person.number, "", 200)
			if err := json.Unmarshal(found, &page); err != nil || len(page.Items) != 1 || page.Items[0].AccountID != person.principal.AccountID || page.Items[0].Role != person.role {
				t.Fatalf("контакт по номеру не найден: %s", found)
			}
		}
		var none messagesPage[messagePerson]
		if err := json.Unmarshal(call(student, "GET", "/contacts?number=202600", "", 200), &none); err != nil || none.Total != 0 {
			t.Fatal("номер должен совпадать полностью")
		}
		call(student, "GET", "/contacts?number=2026002&number=2001", "", 400)

		firstGroup := "/groups/study-1"
		call(student, "GET", firstGroup+"/messages", "", 200)
		call(other, "GET", firstGroup+"/messages", "", 404)
		call(teacher, "GET", firstGroup+"/messages", "", 200)
		requestGroup := func(members ...string) string {
			raw, _ := json.Marshal(map[string]any{"title": "Изолированная группа теста", "memberAccountIDs": members})
			return string(raw)
		}
		call(student, "POST", "/groups", requestGroup(teacher.AccountID, other.AccountID), 403)
		byNumber := func(number int) string {
			raw, _ := json.Marshal(map[string]any{"title": "Группа из поиска по номеру",
				"memberAccountIDs": []string{teacher.AccountID, other.AccountID},
				"contactNumbers":   map[string]int{other.AccountID: number},
			})
			return string(raw)
		}
		call(student, "POST", "/groups", byNumber(2026003), 403)
		call(student, "POST", "/groups", byNumber(2026002), 201)

		created := call(student, "POST", "/groups", requestGroup(teacher.AccountID, support.AccountID), 201)
		var group groupConversation
		if err := json.Unmarshal(created, &group); err != nil || group.MemberCount != 3 {
			t.Fatalf("группа не создана: %s", created)
		}
		path := "/groups/" + group.GroupID
		clientID := uuid.NewString()
		payload, _ := json.Marshal(map[string]string{"text": "Проверка групповой переписки", "messageID": clientID})
		call(student, "POST", path+"/messages", string(payload), 201)
		call(student, "POST", path+"/messages", string(payload), 201)
		call(other, "GET", path+"/messages", "", 404)
		call(other, "POST", path+"/read?through="+clientID, "", 404)
		call(other, "POST", path+"/messages", string(payload), 404)
		unread := func(p *Principal) int64 {
			var list struct{ Items []groupConversation }
			if err := json.Unmarshal(call(p, "GET", "/groups", "", 200), &list); err != nil {
				t.Fatal(err)
			}
			for _, item := range list.Items {
				if item.GroupID == group.GroupID {
					return item.Unread
				}
			}
			t.Fatal("нет группы")
			return -1
		}
		if unread(teacher) != 1 || unread(support) != 1 || unread(student) != 0 {
			t.Fatal("неверные счётчики до прочтения")
		}
		call(teacher, "POST", path+"/read?through="+clientID, "", 204)
		if unread(teacher) != 0 || unread(support) != 1 {
			t.Fatal("прочтение преподавателя затронуло другого участника")
		}
		var page messagesPage[groupMessage]
		err := json.Unmarshal(call(teacher, "GET", path+"/messages", "", 200), &page)
		if err != nil || page.Total != 1 || len(page.Items) != 1 || !page.Items[0].Read || page.Items[0].DisplayName != student.DisplayName {
			t.Fatalf("некорректная история группы: %+v %v", page, err)
		}
	})
	first := request(ids[0], "POST", "/conversations", `{"participantAccountID":"`+ids[1]+`"}`)
	if first.Code != 201 {
		t.Fatalf("create: %d %s", first.Code, first.Body.String())
	}
	var conversation messageConversation
	if err = json.Unmarshal(first.Body.Bytes(), &conversation); err != nil {
		t.Fatal(err)
	}
	again := request(ids[1], "POST", "/conversations", `{"participantAccountID":"`+ids[0]+`"}`)
	if again.Code != 200 {
		t.Fatalf("duplicate create: %d %s", again.Code, again.Body.String())
	}
	var same messageConversation
	if err = json.Unmarshal(again.Body.Bytes(), &same); err != nil || same.ConversationID != conversation.ConversationID {
		t.Fatal("duplicate conversation")
	}
	summaryResponse := request(ids[0], "GET", "/conversations", "")
	if summaryResponse.Code != 200 {
		t.Fatalf("conversation summaries: %d %s", summaryResponse.Code, summaryResponse.Body.String())
	}
	var summaries struct {
		Items []struct {
			ID      string          `json:"conversationID"`
			Members []messagePerson `json:"members"`
		}
	}
	if err = json.Unmarshal(summaryResponse.Body.Bytes(), &summaries); err != nil || len(summaries.Items) != 1 || summaries.Items[0].ID != conversation.ConversationID || len(summaries.Items[0].Members) != 2 {
		t.Fatal("invalid conversation summary", err)
	}
	path := "/conversations/" + conversation.ConversationID
	messageID := uuid.NewString()
	payload := `{"text":"Привет","messageID":"` + messageID + `"}`
	sent := request(ids[0], "POST", path+"/messages", payload)
	if w := request(ids[0], "POST", path+"/messages", payload); w.Code != 201 {
		t.Fatalf("повтор отправки: %d %s", w.Code, w.Body.String())
	}
	if sent.Code != 201 {
		t.Fatalf("send: %d %s", sent.Code, sent.Body.String())
	}
	people := request(ids[0], "GET", path+"/participants", "")
	if people.Code != 200 {
		t.Fatalf("participants: %d %s", people.Code, people.Body.String())
	}
	var profiles struct {
		Items []map[string]any `json:"items"`
	}
	if err = json.Unmarshal(people.Body.Bytes(), &profiles); err != nil || len(profiles.Items) != 2 {
		t.Fatal("participant profiles", err)
	}
	for _, person := range profiles.Items {
		if len(person) != 3 || person["accountID"] == nil || person["displayName"] == nil || person["role"] == nil {
			t.Fatal("participant exposed extra fields")
		}
	}
	outsider := uuid.NewString()
	for _, route := range []struct{ method, suffix, body string }{{"GET", "/participants", ""}, {"GET", "/messages", ""}, {"POST", "/messages", `{"text":"чужое"}`}, {"POST", "/read", ""}} {
		w := request(outsider, route.method, path+route.suffix, route.body)
		if w.Code != 404 {
			t.Fatalf("outsider %s: %d %s", route.method, w.Code, w.Body.String())
		}
	}
	if w := request(ids[0], "POST", path+"/read", ""); w.Code != 204 {
		t.Fatalf("sender read: %d", w.Code)
	}
	list := func() []privateMessage {
		w := request(ids[1], "GET", path+"/messages", "")
		if w.Code != 200 {
			t.Fatalf("list: %d %s", w.Code, w.Body.String())
		}
		var page messagesPage[privateMessage]
		if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("unexpected page: %+v", page)
		}
		return page.Items
	}
	if list()[0].ReadAt != nil {
		t.Fatal("sender marked outgoing message read")
	}
	if w := request(ids[1], "POST", path+"/read", ""); w.Code != 204 {
		t.Fatalf("recipient read: %d", w.Code)
	}
	read := list()[0].ReadAt
	if read == nil {
		t.Fatal("recipient read marker missing")
	}
	if w := request(ids[1], "POST", path+"/read", ""); w.Code != 204 {
		t.Fatalf("repeat read: %d", w.Code)
	}
	if !list()[0].ReadAt.Equal(*read) {
		t.Fatal("repeat read overwrote marker")
	}
}
