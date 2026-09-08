package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// ChatsCollection is the MongoDB collection name for chat sessions.
	ChatsCollection = "chats"

	// MessagesCollection is the MongoDB collection name for chat messages.
	MessagesCollection = "messages"
)

// NewMongoClient establishes a connection to MongoDB using the provided URI.
func NewMongoClient(ctx context.Context, uri string) (*mongo.Client, error) {
	clientOptions := options.Client().
		ApplyURI(uri).
		SetConnectTimeout(10 * time.Second).
		SetMaxPoolSize(50).
		SetMinPoolSize(5)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongo client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("failed to ping mongo server: %w", err)
	}

	return client, nil
}

// EnsureIndexes creates required compound indexes on MongoDB collections.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	// 1. Compound index on chats: { user_id: 1, updated_at: -1 }
	chatIndexes := db.Collection(ChatsCollection).Indexes()
	chatIndexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "updated_at", Value: -1},
		},
		Options: options.Index().SetName("idx_chats_user_updated"),
	}

	if _, err := chatIndexes.CreateOne(ctx, chatIndexModel); err != nil {
		return fmt.Errorf("failed to create index on chats collection: %w", err)
	}

	// 2. Compound index on messages: { chat_id: 1, created_at: 1 }
	msgIndexes := db.Collection(MessagesCollection).Indexes()
	msgIndexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "chat_id", Value: 1},
			{Key: "created_at", Value: 1},
		},
		Options: options.Index().SetName("idx_messages_chat_created"),
	}

	if _, err := msgIndexes.CreateOne(ctx, msgIndexModel); err != nil {
		return fmt.Errorf("failed to create index on messages collection: %w", err)
	}

	return nil
}
