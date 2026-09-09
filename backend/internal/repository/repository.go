package repository

import (
	"context"

	"backend/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserRepository defines persistence operations for user accounts.
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByVKID(ctx context.Context, vkID int64) (*model.User, error)
	GetByRefCode(ctx context.Context, refCode string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
}

// BalanceRepository defines operations for balance inquiries and atomic balance mutations.
type BalanceRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*model.Balance, error)
	AddBonus(ctx context.Context, userID int64, amount int64, txType model.TransactionType, refID *string, description string) (*model.Balance, error)
	Deduct(ctx context.Context, userID int64, amount int64, txType model.TransactionType, refID *string, description string) (*model.Balance, error)
	DeductUsage(ctx context.Context, userID int64, amount int64, txType model.TransactionType, refID *string, description string) (*model.Balance, error)
	GetTransactions(ctx context.Context, userID int64, limit, offset int) ([]model.BalanceTransaction, error)
}

// ChatRepository defines CRUD operations for chat sessions in MongoDB.
type ChatRepository interface {
	Create(ctx context.Context, chat *model.Chat) error
	GetByID(ctx context.Context, id bson.ObjectID, userID int64) (*model.Chat, error)
	ListByUserID(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error)
	UpdateTitle(ctx context.Context, id bson.ObjectID, userID int64, title string) error
	Touch(ctx context.Context, id bson.ObjectID, userID int64, model string) error
	Delete(ctx context.Context, id bson.ObjectID, userID int64) error
}

// ReferralRepository defines persistence operations for user referral relations.
type ReferralRepository interface {
	Create(ctx context.Context, referrerID, refereeID, rewardKopecks int64) error
	GetByRefereeID(ctx context.Context, refereeID int64) (*model.Referral, error)
	ListByReferrerID(ctx context.Context, referrerID int64, limit, offset int) ([]model.Referral, error)
}

// MessageRepository defines append and retrieval operations for chat messages in MongoDB.
type MessageRepository interface {
	Create(ctx context.Context, msg *model.Message) error
	ListByChatID(ctx context.Context, chatID bson.ObjectID, limit, offset int64) ([]model.Message, error)
	DeleteByChatID(ctx context.Context, chatID bson.ObjectID) error
}
