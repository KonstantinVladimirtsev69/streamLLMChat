package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Message represents a single turn in a chat conversation.
type Message struct {
	ID               bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ChatID           bson.ObjectID `bson:"chat_id" json:"chat_id"`
	UserID           int64         `bson:"user_id" json:"user_id"`
	Role             string        `bson:"role" json:"role"`
	Content          string        `bson:"content" json:"content"`
	PromptTokens     int           `bson:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int           `bson:"completion_tokens" json:"completion_tokens"`
	TotalTokens      int           `bson:"total_tokens" json:"total_tokens"`
	CostKopecks      int64         `bson:"cost_kopecks" json:"cost_kopecks"`
	CreatedAt        time.Time     `bson:"created_at" json:"created_at"`
}
