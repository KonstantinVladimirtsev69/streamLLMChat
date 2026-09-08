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

type userRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository backed by PostgreSQL.
func NewUserRepository(pool *pgxpool.Pool) *userRepo {
	return &userRepo{pool: pool}
}

func (r *userRepo) getDB(ctx context.Context) database.DBTX {
	if tx, ok := database.ExtractTx(ctx); ok {
		return tx
	}
	return r.pool
}

// Create inserts a new user record.
func (r *userRepo) Create(ctx context.Context, user *model.User) error {
	db := r.getDB(ctx)

	query := `
		INSERT INTO users (vk_id, first_name, last_name, avatar_url, ref_code, referred_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at;
	`

	err := db.QueryRow(ctx, query,
		user.VKID,
		user.FirstName,
		user.LastName,
		user.AvatarURL,
		user.RefCode,
		user.ReferredBy,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// GetByID queries a user by primary key.
func (r *userRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, vk_id, first_name, last_name, avatar_url, ref_code, referred_by, created_at, updated_at
		FROM users
		WHERE id = $1;
	`

	var u model.User
	err := db.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.VKID,
		&u.FirstName,
		&u.LastName,
		&u.AvatarURL,
		&u.RefCode,
		&u.ReferredBy,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}

	return &u, nil
}

// GetByVKID queries a user by VK ID.
func (r *userRepo) GetByVKID(ctx context.Context, vkID int64) (*model.User, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, vk_id, first_name, last_name, avatar_url, ref_code, referred_by, created_at, updated_at
		FROM users
		WHERE vk_id = $1;
	`

	var u model.User
	err := db.QueryRow(ctx, query, vkID).Scan(
		&u.ID,
		&u.VKID,
		&u.FirstName,
		&u.LastName,
		&u.AvatarURL,
		&u.RefCode,
		&u.ReferredBy,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query user by vk_id: %w", err)
	}

	return &u, nil
}

// GetByRefCode queries a user by their unique referral code.
func (r *userRepo) GetByRefCode(ctx context.Context, refCode string) (*model.User, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, vk_id, first_name, last_name, avatar_url, ref_code, referred_by, created_at, updated_at
		FROM users
		WHERE ref_code = $1;
	`

	var u model.User
	err := db.QueryRow(ctx, query, refCode).Scan(
		&u.ID,
		&u.VKID,
		&u.FirstName,
		&u.LastName,
		&u.AvatarURL,
		&u.RefCode,
		&u.ReferredBy,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query user by ref_code: %w", err)
	}

	return &u, nil
}
