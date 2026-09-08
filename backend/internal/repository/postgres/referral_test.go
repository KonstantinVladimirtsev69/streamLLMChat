package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"backend/internal/model"
	"backend/internal/repository/postgres"
)

func TestReferralRepository(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	userRepo := postgres.NewUserRepository(pool)
	refRepo := postgres.NewReferralRepository(pool)

	uniqueSuffix := time.Now().UnixNano()
	referrer := &model.User{
		VKID:      uniqueSuffix,
		FirstName: "Referrer",
		LastName:  "User",
		RefCode:   fmt.Sprintf("ref1_%d", uniqueSuffix),
	}
	if err := userRepo.Create(ctx, referrer); err != nil {
		t.Fatalf("failed to create referrer: %v", err)
	}

	referee := &model.User{
		VKID:      uniqueSuffix + 1,
		FirstName: "Referee",
		LastName:  "User",
		RefCode:   fmt.Sprintf("ref2_%d", uniqueSuffix),
	}
	if err := userRepo.Create(ctx, referee); err != nil {
		t.Fatalf("failed to create referee: %v", err)
	}

	t.Run("create and get referral", func(t *testing.T) {
		err := refRepo.Create(ctx, referrer.ID, referee.ID, 200)
		if err != nil {
			t.Fatalf("failed to create referral: %v", err)
		}

		ref, err := refRepo.GetByRefereeID(ctx, referee.ID)
		if err != nil {
			t.Fatalf("failed to get referral: %v", err)
		}
		if ref.ReferrerID != referrer.ID {
			t.Errorf("expected referrer ID %d, got %d", referrer.ID, ref.ReferrerID)
		}
		if ref.RefereeID != referee.ID {
			t.Errorf("expected referee ID %d, got %d", referee.ID, ref.RefereeID)
		}
		if ref.RewardKopecks != 200 {
			t.Errorf("expected reward 200, got %d", ref.RewardKopecks)
		}
	})

	t.Run("duplicate referee fails", func(t *testing.T) {
		err := refRepo.Create(ctx, referrer.ID, referee.ID, 200)
		if !errors.Is(err, model.ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists for duplicate referee, got %v", err)
		}
	})

	t.Run("self-referral fails", func(t *testing.T) {
		err := refRepo.Create(ctx, referrer.ID, referrer.ID, 200)
		if err == nil {
			t.Fatalf("expected error on self-referral, got nil")
		}
	})

	t.Run("list by referrer", func(t *testing.T) {
		refs, err := refRepo.ListByReferrerID(ctx, referrer.ID, 10, 0)
		if err != nil {
			t.Fatalf("failed to list referrals: %v", err)
		}
		if len(refs) == 0 {
			t.Fatalf("expected at least 1 referral, got %d", len(refs))
		}
	})
}
