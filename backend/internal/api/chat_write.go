package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type chatPayload struct {
	Text      string `json:"text"`
	MessageID string `json:"messageID"`
}

func readMessagePayload(w http.ResponseWriter, r *http.Request) (chatPayload, bool) {
	var payload chatPayload
	if !decode(w, r, &payload) {
		return payload, false
	}
	if !messagesText(payload.Text) {
		fail(w, 422, "VALIDATION_ERROR", "Введите сообщение до 10 000 символов")
		return payload, false
	}
	if payload.MessageID == "" {
		payload.MessageID = uuid.NewString()
		return payload, true
	}
	id, err := messagesUUID(payload.MessageID)
	if err != nil {
		fail(w, 422, "VALIDATION_ERROR", "Некорректный идентификатор сообщения")
		return payload, false
	}
	payload.MessageID = id
	return payload, true
}

func writeChatMessage[T any](ctx context.Context, w http.ResponseWriter, col *mongo.Collection, filter bson.M, value T) (T, bool) {
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	err := col.FindOneAndUpdate(ctx, filter, bson.M{"$setOnInsert": value}, opts).Decode(&value)
	if mongo.IsDuplicateKeyError(err) {
		// A concurrent retry of the exact same message can find the committed row.
		err = col.FindOne(ctx, filter).Decode(&value)
		if errors.Is(err, mongo.ErrNoDocuments) {
			fail(w, 409, "CONFLICT", "Идентификатор уже используется другим сообщением")
			return value, false
		}
	}
	if err != nil {
		messagesUnavailable(w)
		return value, false
	}
	return value, true
}

// Acknowledgements without a cutoff are supported for older callers. New
// viewers provide the last message actually loaded in this conversation.
func chatReadCutoff(ctx context.Context, w http.ResponseWriter, r *http.Request, col *mongo.Collection, filter bson.M, key, id string) bool {
	cutoff := r.URL.Query().Get("through")
	if cutoff == "" {
		filter["createdAt"] = bson.M{"$lte": time.Now().UTC()}
		return true
	}
	cutoff, err := messagesUUID(cutoff)
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректная граница прочтения")
		return false
	}
	var msg struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	err = col.FindOne(ctx, bson.M{"_id": cutoff, key: id}).Decode(&msg)
	if errors.Is(err, mongo.ErrNoDocuments) {
		fail(w, 404, "NOT_FOUND", "Сообщение недоступно")
		return false
	}
	if err != nil {
		messagesUnavailable(w)
		return false
	}
	filter["$or"] = bson.A{bson.M{"createdAt": bson.M{"$lt": msg.CreatedAt}}, bson.M{"createdAt": msg.CreatedAt, "_id": bson.M{"$lte": cutoff}}}
	return true
}
