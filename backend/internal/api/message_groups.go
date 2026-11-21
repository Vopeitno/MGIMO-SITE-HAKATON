package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const groupCollection = "message_groups"
const groupMessageCollection = "group_messages"

type groupConversation struct {
	GroupID     string    `json:"groupID" bson:"_id"`
	Title       string    `json:"title" bson:"title"`
	Kind        string    `json:"kind" bson:"kind"`
	Members     []string  `json:"-" bson:"members,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt" bson:"updatedAt"`
	Unread      int64     `json:"unread" bson:"-"`
	MemberCount int       `json:"memberCount" bson:"-"`
}
type groupMessage struct {
	ID          string    `json:"messageID" bson:"_id"`
	GroupID     string    `json:"groupID" bson:"groupID"`
	Sender      string    `json:"senderAccountID" bson:"senderAccountID"`
	Text        string    `json:"text" bson:"text"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
	ReadBy      []string  `json:"-" bson:"readBy,omitempty"`
	DisplayName string    `json:"senderName" bson:"-"`
	Read        bool      `json:"read" bson:"-"`
}

func initializeGroups(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(groupCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "members", Value: 1}},
	})
	if err != nil {
		return err
	}
	_, err = db.Collection(groupMessageCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "groupID", Value: 1}, {Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}},
	})
	return err
}

func (s *Server) registerGroups(mux *http.ServeMux) {
	mux.HandleFunc("GET "+messagesPrefix+"/contacts", s.chatContacts)
	mux.HandleFunc("GET "+messagesPrefix+"/groups", s.groupsList)
	mux.HandleFunc("POST "+messagesPrefix+"/groups", s.groupCreate)
	mux.HandleFunc("GET "+messagesPrefix+"/groups/{groupID}/messages", s.groupMessages)
	mux.HandleFunc("POST "+messagesPrefix+"/groups/{groupID}/messages", s.groupSend)
	mux.HandleFunc("POST "+messagesPrefix+"/groups/{groupID}/read", s.groupRead)
}

func academicChatID(id int) string { return "study-" + strconv.Itoa(id) }

func (s *Server) groupAccess(ctx context.Context, w http.ResponseWriter, r *http.Request, account string) (string, bool) {
	id := r.PathValue("groupID")
	if raw, isAcademic := strings.CutPrefix(id, "study-"); isAcademic {
		n, err := strconv.Atoi(raw)
		if err != nil || academicChatID(n) != id || n <= 0 {
			fail(w, 400, "BAD_REQUEST", "Некорректная группа")
			return "", false
		}
		p := current(r)
		var one int
		err = s.DB.QueryRow(ctx, `SELECT 1 FROM (`+academicChats+`) accessible WHERE "groupID"=$4`, p.Role, p.StudentID, p.TeacherID, n).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "NOT_FOUND", "Чат группы недоступен")
			return "", false
		}
		if err != nil {
			messagesUnavailable(w)
			return "", false
		}
		return id, true
	}
	id, err := messagesUUID(id)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная группа")
		return "", false
	}
	err = s.Mongo.Collection(groupCollection).FindOne(ctx, bson.M{"_id": id, "members": account}).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		fail(w, 404, "NOT_FOUND", "Чат группы недоступен")
		return "", false
	}
	if err != nil {
		messagesUnavailable(w)
		return "", false
	}
	return id, true
}

func (s *Server) groupsList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	rows, err := s.academicChatList(ctx, current(r))
	if err != nil {
		messagesUnavailable(w)
		return
	}
	items := make([]groupConversation, 0, len(rows))
	for _, row := range rows {
		items = append(items, groupConversation{GroupID: academicChatID(row.ID), Title: fmt.Sprintf("Группа %d · общий чат", row.Number), Kind: "academic"})
	}
	found, err := s.Mongo.Collection(groupCollection).Find(ctx, bson.M{"members": account})
	if err != nil {
		messagesUnavailable(w)
		return
	}
	var custom []groupConversation
	if err = found.All(ctx, &custom); err != nil {
		messagesUnavailable(w)
		return
	}
	for _, item := range custom {
		item.MemberCount = len(item.Members)
		items = append(items, item)
	}
	if len(items) > 0 {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.GroupID)
		}
		unread := bson.M{"$and": bson.A{
			bson.M{"$ne": bson.A{"$senderAccountID", account}},
			bson.M{"$not": bson.A{bson.M{"$in": bson.A{account, bson.M{"$ifNull": bson.A{"$readBy", bson.A{}}}}}}},
		}}
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"groupID": bson.M{"$in": ids}}}},
			{{Key: "$group", Value: bson.M{"_id": "$groupID", "last": bson.M{"$max": "$createdAt"},
				"unread": bson.M{"$sum": bson.M{"$cond": bson.A{unread, 1, 0}}}}}},
		}
		cursor, err := s.Mongo.Collection(groupMessageCollection).Aggregate(ctx, pipeline)
		if err != nil {
			messagesUnavailable(w)
			return
		}
		var stats []struct {
			ID     string    `bson:"_id"`
			Last   time.Time `bson:"last"`
			Unread int64     `bson:"unread"`
		}
		if err = cursor.All(ctx, &stats); err != nil {
			messagesUnavailable(w)
			return
		}
		index := make(map[string]int, len(items))
		for i, item := range items {
			index[item.GroupID] = i
		}
		for _, row := range stats {
			i := index[row.ID]
			items[i].Unread = row.Unread
			if row.Last.After(items[i].UpdatedAt) {
				items[i].UpdatedAt = row.Last
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	write(w, 200, map[string]any{"items": items})
}

func (s *Server) groupCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	var in struct {
		Title            string
		MemberAccountIDs []string
		ContactNumbers   map[string]int32
	}
	if !decode(w, r, &in) {
		return
	}
	title := strings.Join(strings.Fields(in.Title), " ")
	if title == "" || utf8.RuneCountInString(title) > 80 || len(in.MemberAccountIDs) < 2 || len(in.MemberAccountIDs) > 31 {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название до 80 символов и выберите от 2 до 31 собеседника")
		return
	}
	ids := make([]string, 0, len(in.MemberAccountIDs))
	seen := map[string]bool{account: true}
	for _, raw := range in.MemberAccountIDs {
		id, err := messagesUUID(raw)
		if err != nil || seen[id] {
			fail(w, 422, "VALIDATION_ERROR", "Проверьте список участников")
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(in.ContactNumbers) > len(ids) {
		fail(w, 422, "VALIDATION_ERROR", "Проверьте номера участников")
		return
	}
	for id, number := range in.ContactNumbers {
		if !seen[id] || id == account || number < 1 {
			fail(w, 422, "VALIDATION_ERROR", "Проверьте номера участников")
			return
		}
	}
	allowed, err := s.eligibleChatContacts(ctx, current(r), ids, in.ContactNumbers)
	if err != nil {
		messagesUnavailable(w)
		return
	}
	if !allowed {
		fail(w, 403, "FORBIDDEN", "Можно добавить только доступные контакты")
		return
	}
	item := groupConversation{GroupID: uuid.NewString(), Title: title, Kind: "custom", Members: append(ids, account),
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond), MemberCount: len(ids) + 1}
	if _, err = s.Mongo.Collection(groupCollection).InsertOne(ctx, item); err != nil {
		messagesUnavailable(w)
		return
	}
	write(w, 201, item)
}

func (s *Server) groupMessages(w http.ResponseWriter, r *http.Request) {
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
	id, ok := s.groupAccess(ctx, w, r, account)
	if !ok {
		return
	}
	col := s.Mongo.Collection(groupMessageCollection)
	filter := bson.M{"groupID": id}
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
	items := []groupMessage{}
	if err = cursor.All(ctx, &items); err != nil {
		messagesUnavailable(w)
		return
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, msg := range items {
		if !seen[msg.Sender] {
			ids = append(ids, msg.Sender)
			seen[msg.Sender] = true
		}
	}
	people, err := s.messagePeople(ctx, messagePersonSQL+` WHERE a."accountID"=ANY($1::text[]::uuid[])`, ids)
	if err != nil {
		messagesUnavailable(w)
		return
	}
	names := map[string]string{}
	for _, person := range people {
		names[person.AccountID] = person.DisplayName
	}
	for i := range items {
		items[i].DisplayName = names[items[i].Sender]
		if items[i].DisplayName == "" {
			items[i].DisplayName = "Участник"
		}
		for _, reader := range items[i].ReadBy {
			if reader == account {
				items[i].Read = true
				break
			}
		}
	}
	write(w, 200, messagesPage[groupMessage]{items, total, limit, offset})
}

func (s *Server) groupSend(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	body, ok := readMessagePayload(w, r)
	if !ok {
		return
	}
	id, ok := s.groupAccess(ctx, w, r, account)
	if !ok {
		return
	}
	m := groupMessage{ID: body.MessageID, GroupID: id, Sender: account, Text: body.Text,
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond), DisplayName: current(r).DisplayName}
	filter := bson.M{"_id": m.ID, "groupID": id, "senderAccountID": account, "text": body.Text}
	m, ok = writeChatMessage(ctx, w, s.Mongo.Collection(groupMessageCollection), filter, m)
	if !ok {
		return
	}
	m.DisplayName = current(r).DisplayName
	write(w, 201, m)
}

func (s *Server) groupRead(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, account, ok := s.messagesContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	id, ok := s.groupAccess(ctx, w, r, account)
	if !ok {
		return
	}
	col := s.Mongo.Collection(groupMessageCollection)
	filter := bson.M{"groupID": id, "senderAccountID": bson.M{"$ne": account}, "readBy": bson.M{"$ne": account}}
	if !chatReadCutoff(ctx, w, r, col, filter, "groupID", id) {
		return
	}
	if _, err := col.UpdateMany(ctx, filter, bson.M{"$addToSet": bson.M{"readBy": account}}); err != nil {
		messagesUnavailable(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
