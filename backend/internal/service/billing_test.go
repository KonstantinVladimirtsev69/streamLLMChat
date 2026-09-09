package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"backend/internal/model"
	"backend/internal/repository/postgres"
	"backend/internal/service"
)

func TestBillingService(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	billingSvc := service.NewBillingService(balanceRepo)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Биллинг",
		LastName:  "Юзер",
		RefCode:   fmt.Sprintf("bill_%d", uniqueVKID),
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	t.Run("CanGenerate returns false for user without balance record", func(t *testing.T) {
		canGen, bal, err := billingSvc.CanGenerate(ctx, 99999999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if canGen {
			t.Errorf("expected canGen false, got true")
		}
		if bal != 0 {
			t.Errorf("expected balance 0, got %d", bal)
		}
	})

	t.Run("CanGenerate with 0 and negative balance", func(t *testing.T) {
		// Seed 0 balance by adding 10 and deducting 10
		_, err := balanceRepo.AddBonus(ctx, user.ID, 10, model.TxWelcomeBonus, nil, "seed")
		if err != nil {
			t.Fatalf("failed to seed balance: %v", err)
		}
		_, err = balanceRepo.Deduct(ctx, user.ID, 10, model.TxTokenCharge, nil, "deduct to 0")
		if err != nil {
			t.Fatalf("failed to deduct: %v", err)
		}

		canGen, bal, err := billingSvc.CanGenerate(ctx, user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if canGen {
			t.Errorf("expected canGen false for 0 balance, got true")
		}
		if bal != 0 {
			t.Errorf("expected bal 0, got %d", bal)
		}

		// Overdraft to -5
		_, err = balanceRepo.DeductUsage(ctx, user.ID, 5, model.TxTokenCharge, nil, "overdraft to -5")
		if err != nil {
			t.Fatalf("failed to deduct: %v", err)
		}

		canGen, bal, err = billingSvc.CanGenerate(ctx, user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if canGen {
			t.Errorf("expected canGen false for negative balance, got true")
		}
		if bal != -5 {
			t.Errorf("expected bal -5, got %d", bal)
		}
	})

	t.Run("CanGenerate and ChargeTokens with positive balance and overdraft", func(t *testing.T) {
		// Add 50 kopecks (was -5, now 45)
		_, err := balanceRepo.AddBonus(ctx, user.ID, 50, model.TxDeposit, nil, "deposit")
		if err != nil {
			t.Fatalf("failed to add bonus: %v", err)
		}

		canGen, bal, err := billingSvc.CanGenerate(ctx, user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !canGen {
			t.Errorf("expected canGen true, got false")
		}
		if bal != 45 {
			t.Errorf("expected balance 45, got %d", bal)
		}

		// Charge 0 kopecks: no-op
		b0, err := billingSvc.ChargeTokens(ctx, user.ID, model.TokenUsage{CostKopecks: 0}, "deepseek/deepseek-chat", nil)
		if err != nil {
			t.Fatalf("unexpected error charging 0: %v", err)
		}
		if b0.AmountKopecks != 45 {
			t.Errorf("expected balance 45, got %d", b0.AmountKopecks)
		}

		// Charge 20 kopecks (balance becomes 25)
		ref := "req_bill_1"
		usage := model.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			CostKopecks:      20,
		}
		b1, err := billingSvc.ChargeTokens(ctx, user.ID, usage, "deepseek/deepseek-chat", &ref)
		if err != nil {
			t.Fatalf("unexpected error charging 20: %v", err)
		}
		if b1.AmountKopecks != 25 {
			t.Errorf("expected balance 25, got %d", b1.AmountKopecks)
		}

		// Charge 40 kopecks: overdraft to -15
		usage2 := model.TokenUsage{
			PromptTokens:     200,
			CompletionTokens: 100,
			TotalTokens:      300,
			CostKopecks:      40,
		}
		ref2 := "req_bill_2"
		b2, err := billingSvc.ChargeTokens(ctx, user.ID, usage2, "deepseek/deepseek-chat", &ref2)
		if err != nil {
			t.Fatalf("unexpected error overdraft charging 40: %v", err)
		}
		if b2.AmountKopecks != -15 {
			t.Errorf("expected balance -15, got %d", b2.AmountKopecks)
		}
	})
}
