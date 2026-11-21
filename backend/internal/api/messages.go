package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const messagesPrefix = "/api/v1/MGIMO/messages"
const messagesMaxText = 10000

type messageConversation struct {
	ConversationID string    `json:"conversationID" bson:"_id"`
	Participants   []string  `json:"participants" bson:"participants"`
	UpdatedAt      time.Time `json:"updatedAt" bson:"updatedAt"`
}
type privateMessage struct {
	MessageID       string     `json:"messageID" bson:"_id"`
	ConversationID  string     `json:"conversationID" bson:"conversationID"`
	SenderAccountID string     `json:"senderAccountID" bson:"senderAccountID"`
	Text            string     `json:"text" bson:"text"`
	CreatedAt       time.Time  `json:"createdAt" bson:"createdAt"`
	ReadAt          *time.Time `json:"readAt" bson:"readAt"`
}
type messagesPage[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int64 `json:"limit"`
	Offset int64 `json:"offset"`
}

// InitMessages only creates indexes. Connection lifecycle belongs to the caller.
func (s *Server) InitMessages(ctx context.Context) error {
	if s.Mongo == nil {
		return errors.New("messages: MongoDB is unavailable")
	}
	_, err := s.Mongo.Collection("message_conversations").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "participants", Value: 1}, {Key: "updatedAt", Value: -1}, {Key: "_id", Value: 1}}})
	if err != nil {
		return err
	}
	_, err = s.Mongo.Collection("private_messages").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "conversationID", Value: 1}, {Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}}},
		{Keys: bson.D{{Key: "conversationID", Value: 1}, {Key: "senderAccountID", Value: 1}, {Key: "readAt", Value: 1}}},
	})
	if err != nil {
		return err
	}
	return initializeGroups(ctx, s.Mongo)
}
func (s *Server) registerMessages(mux *http.ServeMux) {
	s.registerGroups(mux)
	mux.HandleFunc("GET "+messagesPrefix+"/deanery-contacts", s.deaneryContacts)
	mux.HandleFunc("GET "+messagesPrefix+"/conversations/{conversationID}/participants", s.conversationParticipants)
	mux.HandleFunc("GET "+messagesPrefix+"/conversations", s.messagesConversations)
	mux.HandleFunc("POST "+messagesPrefix+"/conversations", s.messagesConversationCreate)
	mux.HandleFunc("GET "+messagesPrefix+"/conversations/{conversationID}/messages", s.messagesList)
	mux.HandleFunc("POST "+messagesPrefix+"/conversations/{conversationID}/messages", s.messagesSend)
	mux.HandleFunc("POST "+messagesPrefix+"/conversations/{conversationID}/read", s.messagesRead)
}
func messagesUUID(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return "", errors.New("invalid UUID")
	}
	return id.String(), nil
}
func messagesPair(a, b string) (string, []string, error) {
	a, err := messagesUUID(a)
	if err != nil {
		return "", nil, err
	}
	b, err = messagesUUID(b)
	if err != nil {
		return "", nil, err
	}
	if a == b {
		return "", nil, errors.New("self conversation")
	}
	ids := []string{a, b}
	sort.Strings(ids)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(ids, ":"))).String(), ids, nil
}
func messagesPagination(r *http.Request) (int64, int64, error) {
	limit, offset := int64(20), int64(0)
	for _, key := range []string{"limit", "offset"} {
		vals, exists := r.URL.Query()[key]
		if !exists {
			continue
		}
		if len(vals) != 1 {
			return 0, 0, errors.New("duplicate pagination parameter")
		}
		n, err := strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		if key == "limit" {
			limit = n
		} else {
			offset = n
		}
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return 0, 0, errors.New("pagination out of bounds")
	}
	return limit, offset, nil
}
func messagesText(text string) bool {
	return utf8.ValidString(text) && utf8.RuneCountInString(text) <= messagesMaxText && strings.TrimSpace(text) != ""
}
func messagesUnavailable(w http.ResponseWriter) {
	fail(w, 503, "MESSAGES_UNAVAILABLE", "Сервис сообщений недоступен")
}
func (s *Server) messagesContext(w http.ResponseWriter, r *http.Request) (context.Context, context.CancelFunc, string, bool) {
	p := current(r)
	if p == nil {
		fail(w, 401, "UNAUTHORIZED", "Необходима авторизация")
		return nil, nil, "", false
	}
	id, err := messagesUUID(p.AccountID)
	if err != nil {
		fail(w, 401, "UNAUTHORIZED", "Недействительная учётная запись")
		return nil, nil, "", false
	}
	if s.Mongo == nil || s.DB == nil {
		messagesUnavailable(w)
		return nil, nil, "", false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	return ctx, cancel, id, true
}

// Active-account checks are reads only; there are no cross-database transactions or cascades.
func (s *Server) messagesAccounts(w http.ResponseWriter, ctx context.Context, ids []string) bool {
	var count int
	err := s.DB.QueryRow(ctx, `SELECT count(*) FROM public.accounts WHERE "accountID"::text = ANY($1::text[]) AND "isActive" = true`, ids).Scan(&count)
	if err != nil {
		messagesUnavailable(w)
		return false
	}
	if count != len(ids) {
		fail(w, 403, "FORBIDDEN", "Участники должны иметь действующие учётные записи")
		return false
	}
	return true
}
func (s *Server) messagesConversation(w http.ResponseWriter, r *http.Request, ctx context.Context, account string) (messageConversation, bool) {
	var c messageConversation
	id, err := messagesUUID(r.PathValue("conversationID"))
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор диалога")
		return c, false
	}
	// Filtering by membership avoids disclosing existence of another user's conversation.
	err = s.Mongo.Collection("message_conversations").FindOne(ctx, bson.M{"_id": id, "participants": account}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		fail(w, 404, "NOT_FOUND", "Диалог не найден")
		return c, false
	}
	if err != nil {
		messagesUnavailable(w)
		return c, false
	}
	if len(c.Participants) != 2 {
		messagesUnavailable(w)
		return c, false
	}
	expected, _, err := messagesPair(c.Participants[0], c.Participants[1])
	if err != nil || expected != c.ConversationID {
		messagesUnavailable(w)
		return c, false
	}
	if !s.messagesAccounts(w, ctx, c.Participants) {
		return c, false
	}
	return c, true
}
func (s *Server) messagesConversations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	limit, offset, err := messagesPagination(r)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная пагинация")
		return
	}
	if !s.messagesAccounts(w, ctx, []string{account}) {
		return
	}
	col := s.Mongo.Collection("message_conversations")
	filter := bson.M{"participants": account}
	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		messagesUnavailable(w)
		return
	}
	cursor, err := col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}, {Key: "_id", Value: 1}}).SetLimit(limit).SetSkip(offset))
	if err != nil {
		messagesUnavailable(w)
		return
	}
	defer cursor.Close(ctx)
	items := []messageConversation{}
	if err = cursor.All(ctx, &items); err != nil {
		messagesUnavailable(w)
		return
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, c := range items {
		if len(c.Participants) != 2 {
			messagesUnavailable(w)
			return
		}
		expected, _, pairErr := messagesPair(c.Participants[0], c.Participants[1])
		if pairErr != nil || expected != c.ConversationID {
			messagesUnavailable(w)
			return
		}
		for _, id := range c.Participants {
			if !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
	}
	people, err := s.messagePeople(ctx, messagePersonSQL+`WHERE a."accountID"::text=ANY($1::text[]) AND a."isActive" ORDER BY a."accountID"`, ids)
	if err != nil {
		messagesUnavailable(w)
		return
	}
	if len(people) != len(ids) {
		fail(w, 403, "FORBIDDEN", "Участники должны иметь действующие учётные записи")
		return
	}
	byID := map[string]messagePerson{}
	for _, person := range people {
		byID[person.AccountID] = person
	}

	unread := map[string]int64{}
	if len(items) > 0 {
		conversationIDs := make([]string, 0, len(items))
		for _, c := range items {
			conversationIDs = append(conversationIDs, c.ConversationID)
		}
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"conversationID": bson.M{"$in": conversationIDs}, "senderAccountID": bson.M{"$ne": account}, "readAt": nil}}},
			{{Key: "$group", Value: bson.M{"_id": "$conversationID", "count": bson.M{"$sum": 1}}}},
		}
		result, err := s.Mongo.Collection("private_messages").Aggregate(ctx, pipeline)
		if err != nil {
			messagesUnavailable(w)
			return
		}
		var stats []struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err = result.All(ctx, &stats); err != nil {
			messagesUnavailable(w)
			return
		}
		for _, row := range stats {
			unread[row.ID] = row.Count
		}
	}
	type summary struct {
		messageConversation
		Members []messagePerson `json:"members"`
		Unread  int64           `json:"unread"`
	}
	summaries := make([]summary, 0, len(items))
	for _, c := range items {
		members := make([]messagePerson, 0, len(c.Participants))
		for _, id := range c.Participants {
			members = append(members, byID[id])
		}
		summaries = append(summaries, summary{c, members, unread[c.ConversationID]})
	}
	write(w, 200, messagesPage[summary]{summaries, total, limit, offset})
}
func (s *Server) messagesConversationCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	var body struct {
		ParticipantAccountID string `json:"participantAccountID"`
	}
	if !decode(w, r, &body) {
		return
	}
	id, ids, err := messagesPair(account, body.ParticipantAccountID)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Укажите другую учётную запись с корректным UUID")
		return
	}
	if !s.messagesAccounts(w, ctx, ids) {
		return
	}
	c := messageConversation{id, ids, time.Now().UTC()}
	col := s.Mongo.Collection("message_conversations")
	result, err := col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$setOnInsert": c}, options.UpdateOne().SetUpsert(true))
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		messagesUnavailable(w)
		return
	}
	status := 200
	if err == nil && result.UpsertedCount == 1 {
		status = 201
	}
	if err = col.FindOne(ctx, bson.M{"_id": id, "participants": account}).Decode(&c); err != nil {
		messagesUnavailable(w)
		return
	}
	write(w, status, c)
}
func (s *Server) messagesList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	limit, offset, err := messagesPagination(r)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная пагинация")
		return
	}
	c, ok := s.messagesConversation(w, r, ctx, account)
	if !ok {
		return
	}
	col := s.Mongo.Collection("private_messages")
	filter := bson.M{"conversationID": c.ConversationID}
	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		messagesUnavailable(w)
		return
	}
	cursor, err := col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(limit).SetSkip(offset))
	if err != nil {
		messagesUnavailable(w)
		return
	}
	defer cursor.Close(ctx)
	items := []privateMessage{}
	if err = cursor.All(ctx, &items); err != nil {
		messagesUnavailable(w)
		return
	}
	write(w, 200, messagesPage[privateMessage]{items, total, limit, offset})
}
func (s *Server) messagesSend(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	body, ok := readMessagePayload(w, r)
	if !ok {
		return
	}

	c, ok := s.messagesConversation(w, r, ctx, account)
	if !ok {
		return
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	m := privateMessage{body.MessageID, c.ConversationID, account, body.Text, now, nil}
	m, ok = writeChatMessage(ctx, w, s.Mongo.Collection("private_messages"),
		bson.M{"_id": m.MessageID, "conversationID": c.ConversationID, "senderAccountID": account, "text": body.Text}, m)
	if !ok {
		return
	}
	// This is a separate write, not a transaction: a 503 may follow a persisted message.
	if _, err := s.Mongo.Collection("message_conversations").UpdateOne(ctx, bson.M{"_id": c.ConversationID}, bson.M{"$max": bson.M{"updatedAt": m.CreatedAt}}); err != nil {
		messagesUnavailable(w)
		return
	}
	write(w, 201, m)
}
func (s *Server) messagesRead(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	c, ok := s.messagesConversation(w, r, ctx, account)
	if !ok {
		return
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	filter := bson.M{"conversationID": c.ConversationID, "senderAccountID": bson.M{"$ne": account}, "readAt": nil}
	col := s.Mongo.Collection("private_messages")
	if !chatReadCutoff(ctx, w, r, col, filter, "conversationID", c.ConversationID) {
		return
	}
	if _, err := col.UpdateMany(ctx, filter, bson.M{"$set": bson.M{"readAt": now}}); err != nil {
		messagesUnavailable(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
