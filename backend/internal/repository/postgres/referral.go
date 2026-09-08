package postgres

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/database"
	"backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type referralRepo struct {
	pool *pgxpool.Pool
}

// NewReferralRepository creates a new ReferralRepository backed by PostgreSQL.
func NewReferralRepository(pool *pgxpool.Pool) *referralRepo {
	return &referralRepo{pool: pool}
}

func (r *referralRepo) getDB(ctx context.Context) database.DBTX {
	if tx, ok := database.ExtractTx(ctx); ok {
		return tx
	}
	return r.pool
}

// Create records a new referral relationship in the database.
func (r *referralRepo) Create(ctx context.Context, referrerID, refereeID, rewardKopecks int64) error {
	if referrerID == refereeID {
		return fmt.Errorf("%w: self-referral is prohibited", model.ErrInvalidOperation)
	}

	db := r.getDB(ctx)

	query := `
		INSERT INTO referrals (referrer_id, referee_id, reward_kopecks, created_at)
		VALUES ($1, $2, $3, NOW());
	`

	_, err := db.Exec(ctx, query, referrerID, refereeID, rewardKopecks)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			if pgErr.Code == "23505" {
				return model.ErrUserAlreadyExists // referee already referred
			}
			if pgErr.Code == "23514" { // check violation
				return fmt.Errorf("%w: referral constraint violation", model.ErrInvalidOperation)
			}
		}
		return fmt.Errorf("failed to insert referral: %w", err)
	}

	return nil
}

// GetByRefereeID retrieves referral record for a specific referee.
func (r *referralRepo) GetByRefereeID(ctx context.Context, refereeID int64) (*model.Referral, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, referrer_id, referee_id, reward_kopecks, created_at
		FROM referrals
		WHERE referee_id = $1;
	`

	var ref model.Referral
	err := db.QueryRow(ctx, query, refereeID).Scan(
		&ref.ID,
		&ref.ReferrerID,
		&ref.RefereeID,
		&ref.RewardKopecks,
		&ref.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query referral by referee: %w", err)
	}

	return &ref, nil
}

// ListByReferrerID retrieves all referrals initiated by a referrer.
func (r *referralRepo) ListByReferrerID(ctx context.Context, referrerID int64, limit, offset int) ([]model.Referral, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	db := r.getDB(ctx)

	query := `
		SELECT id, referrer_id, referee_id, reward_kopecks, created_at
		FROM referrals
		WHERE referrer_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := db.Query(ctx, query, referrerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list referrals: %w", err)
	}
	defer rows.Close()

	var referrals []model.Referral
	for rows.Next() {
		var ref model.Referral
		if err := rows.Scan(
			&ref.ID,
			&ref.ReferrerID,
			&ref.RefereeID,
			&ref.RewardKopecks,
			&ref.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan referral row: %w", err)
		}
		referrals = append(referrals, ref)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading referral rows: %w", err)
	}

	return referrals, nil
}
