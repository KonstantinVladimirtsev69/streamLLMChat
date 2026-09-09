package postgres

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/database"
	"backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type balanceRepo struct {
	pool *pgxpool.Pool
}

// NewBalanceRepository creates a new BalanceRepository backed by PostgreSQL.
func NewBalanceRepository(pool *pgxpool.Pool) *balanceRepo {
	return &balanceRepo{pool: pool}
}

func (r *balanceRepo) getDB(ctx context.Context) database.DBTX {
	if tx, ok := database.ExtractTx(ctx); ok {
		return tx
	}
	return r.pool
}

// GetByUserID queries current balance for a user.
func (r *balanceRepo) GetByUserID(ctx context.Context, userID int64) (*model.Balance, error) {
	db := r.getDB(ctx)

	query := `
		SELECT user_id, amount_kopecks, updated_at
		FROM balances
		WHERE user_id = $1;
	`

	var b model.Balance
	err := db.QueryRow(ctx, query, userID).Scan(
		&b.UserID,
		&b.AmountKopecks,
		&b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query balance: %w", err)
	}

	return &b, nil
}

// AddBonus atomically credits a bonus or deposit amount to user balance and records a ledger transaction.
func (r *balanceRepo) AddBonus(ctx context.Context, userID int64, amount int64, txType model.TransactionType, refID *string, description string) (*model.Balance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: bonus amount must be positive", model.ErrInvalidOperation)
	}

	db := r.getDB(ctx)

	upsertQuery := `
		INSERT INTO balances (user_id, amount_kopecks, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET amount_kopecks = balances.amount_kopecks + EXCLUDED.amount_kopecks,
		    updated_at = NOW()
		RETURNING amount_kopecks, updated_at;
	`

	var b model.Balance
	b.UserID = userID
	err := db.QueryRow(ctx, upsertQuery, userID, amount).Scan(&b.AmountKopecks, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to credit balance: %w", err)
	}

	txQuery := `
		INSERT INTO balance_transactions (user_id, amount_kopecks, balance_after_kopecks, type, reference_id, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW());
	`

	_, err = db.Exec(ctx, txQuery, userID, amount, b.AmountKopecks, string(txType), refID, description)
	if err != nil {
		return nil, fmt.Errorf("failed to insert balance transaction ledger: %w", err)
	}

	return &b, nil
}

// Deduct atomically deducts an amount from user balance ensuring non-negative balance, and records a ledger transaction.
func (r *balanceRepo) Deduct(ctx context.Context, userID int64, amount int64, txType model.TransactionType, refID *string, description string) (*model.Balance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: deduction amount must be positive", model.ErrInvalidOperation)
	}

	db := r.getDB(ctx)

	// Atomic conditional deduction
	updateQuery := `
		UPDATE balances
		SET amount_kopecks = amount_kopecks - $1,
		    updated_at = NOW()
		WHERE user_id = $2 AND amount_kopecks >= $1
		RETURNING amount_kopecks, updated_at;
	`

	var b model.Balance
	b.UserID = userID
	err := db.QueryRow(ctx, updateQuery, amount, userID).Scan(&b.AmountKopecks, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Determine if balance exists or insufficient
			var currentAmount int64
			checkErr := db.QueryRow(ctx, "SELECT amount_kopecks FROM balances WHERE user_id = $1", userID).Scan(&currentAmount)
			if errors.Is(checkErr, pgx.ErrNoRows) {
				return nil, model.ErrNotFound
			}
			return nil, model.ErrInsufficientBalance
		}
		return nil, fmt.Errorf("failed to deduct balance: %w", err)
	}

	txQuery := `
		INSERT INTO balance_transactions (user_id, amount_kopecks, balance_after_kopecks, type, reference_id, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW());
	`

	_, err = db.Exec(ctx, txQuery, userID, -amount, b.AmountKopecks, string(txType), refID, description)
	if err != nil {
		return nil, fmt.Errorf("failed to insert deduction transaction ledger: %w", err)
	}

	return &b, nil
}

// DeductUsage atomically debits token usage from user balance allowing overdraft (D-05), and records a ledger transaction.
func (r *balanceRepo) DeductUsage(ctx context.Context, userID int64, amount int64, txType model.TransactionType, refID *string, description string) (*model.Balance, error) {
	if amount < 0 {
		return nil, fmt.Errorf("%w: deduction amount cannot be negative", model.ErrInvalidOperation)
	}
	if amount == 0 {
		return r.GetByUserID(ctx, userID)
	}

	db := r.getDB(ctx)

	// Atomic update without non-negative check to support overdraft for in-flight completions
	updateQuery := `
		UPDATE balances
		SET amount_kopecks = amount_kopecks - $1,
		    updated_at = NOW()
		WHERE user_id = $2
		RETURNING amount_kopecks, updated_at;
	`

	var b model.Balance
	b.UserID = userID
	err := db.QueryRow(ctx, updateQuery, amount, userID).Scan(&b.AmountKopecks, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to deduct balance usage: %w", err)
	}

	txQuery := `
		INSERT INTO balance_transactions (user_id, amount_kopecks, balance_after_kopecks, type, reference_id, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW());
	`

	_, err = db.Exec(ctx, txQuery, userID, -amount, b.AmountKopecks, string(txType), refID, description)
	if err != nil {
		return nil, fmt.Errorf("failed to insert usage deduction transaction ledger: %w", err)
	}

	return &b, nil
}

// GetTransactions queries transaction history for a user.
func (r *balanceRepo) GetTransactions(ctx context.Context, userID int64, limit, offset int) ([]model.BalanceTransaction, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	db := r.getDB(ctx)

	query := `
		SELECT id, user_id, amount_kopecks, balance_after_kopecks, type, reference_id, description, created_at
		FROM balance_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []model.BalanceTransaction
	for rows.Next() {
		var tx model.BalanceTransaction
		var txTypeStr string
		if err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.AmountKopecks,
			&tx.BalanceAfterKopecks,
			&txTypeStr,
			&tx.ReferenceID,
			&tx.Description,
			&tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		tx.Type = model.TransactionType(txTypeStr)
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading transaction rows: %w", err)
	}

	return transactions, nil
}
