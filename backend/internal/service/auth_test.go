package service_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/model"
	"backend/internal/repository/postgres"
	"backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@127.0.0.1:5432/llmchat?sslmode=disable" //nolint:gosec
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping postgres integration test (postgres not reachable: %v)", err)
	}

	if err := database.Up(dbURL); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return pool
}

func TestAuthService(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	referralRepo := postgres.NewReferralRepository(pool)
	txManager := database.NewTxManager(pool)
	refGen := auth.NewRefCodeGenerator(8)

	authSvc := service.NewAuthService(userRepo, balanceRepo, referralRepo, txManager, refGen, "http://localhost:3000")

	uniqueVKID := time.Now().UnixNano()

	t.Run("first time user registration gets 500 kopecks welcome bonus", func(t *testing.T) {
		profile := service.VKProfile{
			ID:        uniqueVKID,
			FirstName: "Иван",
			LastName:  "Иванов",
			AvatarURL: "https://example.com/avatar.jpg",
		}

		user, err := authSvc.AuthenticateOrRegister(ctx, profile, "")
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}
		if user.ID == 0 {
			t.Errorf("expected non-zero user ID")
		}
		if user.VKID != uniqueVKID {
			t.Errorf("expected VKID %d, got %d", uniqueVKID, user.VKID)
		}

		// Verify balance is 500 kopecks
		bal, err := balanceRepo.GetByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("failed to get balance: %v", err)
		}
		if bal.AmountKopecks != 500 {
			t.Errorf("expected balance 500, got %d", bal.AmountKopecks)
		}

		// Verify transactions ledger contains welcome_bonus
		txs, err := balanceRepo.GetTransactions(ctx, user.ID, 10, 0)
		if err != nil {
			t.Fatalf("failed to get transactions: %v", err)
		}
		if len(txs) != 1 {
			t.Fatalf("expected 1 transaction, got %d", len(txs))
		}
		if txs[0].Type != model.TxWelcomeBonus {
			t.Errorf("expected transaction type welcome_bonus, got %s", txs[0].Type)
		}
		if txs[0].AmountKopecks != 500 {
			t.Errorf("expected transaction amount 500, got %d", txs[0].AmountKopecks)
		}
	})

	t.Run("existing user login does not duplicate bonus", func(t *testing.T) {
		profile := service.VKProfile{
			ID:        uniqueVKID,
			FirstName: "Иван",
			LastName:  "Иванов",
			AvatarURL: "https://example.com/avatar.jpg",
		}

		user, err := authSvc.AuthenticateOrRegister(ctx, profile, "")
		if err != nil {
			t.Fatalf("failed to login: %v", err)
		}

		// Balance should still be 500 kopecks
		bal, err := balanceRepo.GetByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("failed to get balance: %v", err)
		}
		if bal.AmountKopecks != 500 {
			t.Errorf("expected balance 500, got %d", bal.AmountKopecks)
		}

		// Transactions count still 1
		txs, _ := balanceRepo.GetTransactions(ctx, user.ID, 10, 0)
		if len(txs) != 1 {
			t.Errorf("expected 1 transaction, got %d", len(txs))
		}
	})

	t.Run("registration via referral code rewards both users", func(t *testing.T) {
		// Existing user (referrer)
		referrerUser, err := userRepo.GetByVKID(ctx, uniqueVKID)
		if err != nil {
			t.Fatalf("failed to get referrer: %v", err)
		}

		refereeVKID := uniqueVKID + 1
		refereeProfile := service.VKProfile{
			ID:        refereeVKID,
			FirstName: "Петр",
			LastName:  "Петров",
			AvatarURL: "https://example.com/petr.jpg",
		}

		refereeUser, err := authSvc.AuthenticateOrRegister(ctx, refereeProfile, referrerUser.RefCode)
		if err != nil {
			t.Fatalf("failed to register referee: %v", err)
		}

		// Referee balance should be 500 kopecks
		refereeBal, err := balanceRepo.GetByUserID(ctx, refereeUser.ID)
		if err != nil {
			t.Fatalf("failed to get referee balance: %v", err)
		}
		if refereeBal.AmountKopecks != 500 {
			t.Errorf("expected referee balance 500, got %d", refereeBal.AmountKopecks)
		}

		// Referrer balance should have increased by 200 kopecks (500 + 200 = 700)
		referrerBal, err := balanceRepo.GetByUserID(ctx, referrerUser.ID)
		if err != nil {
			t.Fatalf("failed to get referrer balance: %v", err)
		}
		if referrerBal.AmountKopecks != 700 {
			t.Errorf("expected referrer balance 700, got %d", referrerBal.AmountKopecks)
		}

		// Check referral row
		ref, err := referralRepo.GetByRefereeID(ctx, refereeUser.ID)
		if err != nil {
			t.Fatalf("failed to get referral: %v", err)
		}
		if ref.ReferrerID != referrerUser.ID {
			t.Errorf("expected referrer ID %d, got %d", referrerUser.ID, ref.ReferrerID)
		}

		// Check transactions of referrer for referral_reward
		refTxs, err := balanceRepo.GetTransactions(ctx, referrerUser.ID, 10, 0)
		if err != nil {
			t.Fatalf("failed to get referrer txs: %v", err)
		}
		if len(refTxs) != 2 {
			t.Fatalf("expected 2 transactions for referrer, got %d", len(refTxs))
		}
		if refTxs[0].Type != model.TxReferralReward {
			t.Errorf("expected latest tx type referral_reward, got %s", refTxs[0].Type)
		}
		if refTxs[0].AmountKopecks != 200 {
			t.Errorf("expected referral reward 200, got %d", refTxs[0].AmountKopecks)
		}
	})

	t.Run("get profile returns formatted balance and ref_link", func(t *testing.T) {
		user, _ := userRepo.GetByVKID(ctx, uniqueVKID)

		dto, err := authSvc.GetProfile(ctx, user.ID)
		if err != nil {
			t.Fatalf("failed to get profile: %v", err)
		}

		if dto.BalanceKopecks != 700 {
			t.Errorf("expected 700 kopecks, got %d", dto.BalanceKopecks)
		}
		if dto.BalanceRub != 7.00 {
			t.Errorf("expected 7.00 rub, got %f", dto.BalanceRub)
		}
		expectedRefLink := fmt.Sprintf("http://localhost:3000/?ref=%s", user.RefCode)
		if dto.RefLink != expectedRefLink {
			t.Errorf("expected refLink %s, got %s", expectedRefLink, dto.RefLink)
		}
	})
}
