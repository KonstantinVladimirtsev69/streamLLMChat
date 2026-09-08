package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Chat represents an LLM conversation session belonging to a user.
type Chat struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    int64         `bson:"user_id" json:"user_id"`
	Title     string        `bson:"title" json:"title"`
	Model     string        `bson:"model" json:"model"`
	CreatedAt time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at" json:"updated_at"`
}
