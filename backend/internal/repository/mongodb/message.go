package mongodb

import (
	"context"
	"fmt"
	"time"

	"backend/internal/database"
	"backend/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type messageRepo struct {
	coll *mongo.Collection
}

// NewMessageRepository creates a new MessageRepository backed by MongoDB.
func NewMessageRepository(db *mongo.Database) *messageRepo {
	return &messageRepo{coll: db.Collection(database.MessagesCollection)}
}

// Create inserts a new message into chat conversation.
func (r *messageRepo) Create(ctx context.Context, msg *model.Message) error {
	if msg.ID.IsZero() {
		msg.ID = bson.NewObjectID()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	_, err := r.coll.InsertOne(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}
	return nil
}

// ListByChatID lists messages for a given chat ordered chronologically.
func (r *messageRepo) ListByChatID(ctx context.Context, chatID bson.ObjectID, limit, offset int64) ([]model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.D{{Key: "chat_id", Value: chatID}}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: 1}}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var messages []model.Message
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}
	return messages, nil
}

// DeleteByChatID deletes all messages associated with the specified chat ID.
func (r *messageRepo) DeleteByChatID(ctx context.Context, chatID bson.ObjectID) error {
	filter := bson.D{{Key: "chat_id", Value: chatID}}
	_, err := r.coll.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete messages for chat: %w", err)
	}
	return nil
}
