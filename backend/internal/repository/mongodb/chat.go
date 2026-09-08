package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/internal/database"
	"backend/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type chatRepo struct {
	coll *mongo.Collection
}

// NewChatRepository creates a new ChatRepository backed by MongoDB.
func NewChatRepository(db *mongo.Database) *chatRepo {
	return &chatRepo{coll: db.Collection(database.ChatsCollection)}
}

// Create inserts a new chat record.
func (r *chatRepo) Create(ctx context.Context, chat *model.Chat) error {
	if chat.ID.IsZero() {
		chat.ID = bson.NewObjectID()
	}
	now := time.Now()
	chat.CreatedAt = now
	chat.UpdatedAt = now

	_, err := r.coll.InsertOne(ctx, chat)
	if err != nil {
		return fmt.Errorf("failed to insert chat: %w", err)
	}
	return nil
}

// GetByID finds a chat by its ID and owner userID.
func (r *chatRepo) GetByID(ctx context.Context, id bson.ObjectID, userID int64) (*model.Chat, error) {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "user_id", Value: userID},
	}

	var chat model.Chat
	err := r.coll.FindOne(ctx, filter).Decode(&chat)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find chat: %w", err)
	}
	return &chat, nil
}

// ListByUserID lists chats belonging to a user sorted by last activity.
func (r *chatRepo) ListByUserID(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error) {
	if limit <= 0 {
		limit = 20
	}
	filter := bson.D{{Key: "user_id", Value: userID}}
	opts := options.Find().
		SetSort(bson.D{{Key: "updated_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list chats: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var chats []model.Chat
	if err := cursor.All(ctx, &chats); err != nil {
		return nil, fmt.Errorf("failed to decode chats: %w", err)
	}
	return chats, nil
}

// UpdateTitle updates the title and updated_at timestamp of a chat.
func (r *chatRepo) UpdateTitle(ctx context.Context, id bson.ObjectID, userID int64, title string) error {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "user_id", Value: userID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "title", Value: title},
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	result, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update chat title: %w", err)
	}
	if result.MatchedCount == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Delete removes a chat by ID.
func (r *chatRepo) Delete(ctx context.Context, id bson.ObjectID, userID int64) error {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "user_id", Value: userID},
	}

	result, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete chat: %w", err)
	}
	if result.DeletedCount == 0 {
		return model.ErrNotFound
	}
	return nil
}
