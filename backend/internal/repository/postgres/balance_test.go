package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backend/internal/database"
	"backend/internal/model"
	"backend/internal/repository/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@127.0.0.1:5432/llmchat?sslmode=disable" //nolint:gosec // local docker-compose test credentials
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping postgres integration test (postgres not reachable: %v)", err)
	}

	// Ensure migrations are applied
	if err := database.Up(dbURL); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return pool
}

func TestBalanceLifecycleAndTransactions(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	txManager := database.NewTxManager(pool)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Иван",
		LastName:  "Тестовый",
		AvatarURL: "https://example.com/avatar.jpg",
		RefCode:   fmt.Sprintf("ref_%d", uniqueVKID),
	}

	// 1. Create user and credit welcome bonus inside transaction
	err := txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := userRepo.Create(txCtx, user); err != nil {
			return err
		}

		_, err := balanceRepo.AddBonus(
			txCtx,
			user.ID,
			model.WelcomeBonusKopecks,
			model.TxWelcomeBonus,
			nil,
			"Приветственный бонус за регистрацию",
		)
		return err
	})
	if err != nil {
		t.Fatalf("failed to create user and credit welcome bonus: %v", err)
	}

	// 2. Verify initial balance is 500 kopecks
	balance, err := balanceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if balance.AmountKopecks != model.WelcomeBonusKopecks {
		t.Fatalf("expected balance %d, got %d", model.WelcomeBonusKopecks, balance.AmountKopecks)
	}

	// 3. Deduct tokens (150 kopecks)
	refID := "msg_12345"
	deductedBalance, err := balanceRepo.Deduct(ctx, user.ID, 150, model.TxTokenCharge, &refID, "Списание за токены")
	if err != nil {
		t.Fatalf("failed to deduct balance: %v", err)
	}
	if deductedBalance.AmountKopecks != 350 {
		t.Fatalf("expected balance 350, got %d", deductedBalance.AmountKopecks)
	}

	// 4. Try to deduct more than balance (e.g., 500 kopecks from 350)
	_, err = balanceRepo.Deduct(ctx, user.ID, 500, model.TxTokenCharge, nil, "Слишком большое списание")
	if !errors.Is(err, model.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}

	// Verify balance didn't change
	balance, err = balanceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to query balance: %v", err)
	}
	if balance.AmountKopecks != 350 {
		t.Fatalf("expected balance 350 after failed deduction, got %d", balance.AmountKopecks)
	}

	// 5. Verify transaction ledger
	txs, err := balanceRepo.GetTransactions(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get transactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions (bonus + deduct), got %d", len(txs))
	}
}

func TestConcurrentBalanceDeductions(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Конкурентный",
		LastName:  "Пользователь",
		RefCode:   fmt.Sprintf("conc_%d", uniqueVKID),
	}

	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Set initial balance to 350 kopecks
	if _, err := balanceRepo.AddBonus(ctx, user.ID, 350, model.TxWelcomeBonus, nil, "Начальный баланс"); err != nil {
		t.Fatalf("failed to add initial balance: %v", err)
	}

	// Concurrently attempt 10 deductions of 100 kopecks each
	// Initial balance: 350. Exactly 3 deductions should succeed (3 * 100 = 300),
	// 7 should fail with ErrInsufficientBalance, final balance must be 50.
	const totalRoutines = 10
	const deductAmount = 100

	var successCount int32
	var failCount int32
	var wg sync.WaitGroup

	for i := 0; i < totalRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := balanceRepo.Deduct(ctx, user.ID, deductAmount, model.TxTokenCharge, nil, "Конкурентное списание")
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if errors.Is(err, model.ErrInsufficientBalance) {
				atomic.AddInt32(&failCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 3 {
		t.Errorf("expected 3 successful deductions, got %d", successCount)
	}
	if failCount != 7 {
		t.Errorf("expected 7 failed deductions, got %d", failCount)
	}

	finalBalance, err := balanceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get final balance: %v", err)
	}
	if finalBalance.AmountKopecks != 50 {
		t.Errorf("expected final balance 50, got %d", finalBalance.AmountKopecks)
	}
}

func TestTransactionRollbackOnPanicOrError(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	txManager := database.NewTxManager(pool)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Откатный",
		LastName:  "Пользователь",
		RefCode:   fmt.Sprintf("roll_%d", uniqueVKID),
	}

	// Deliberately return error inside transaction
	testErr := errors.New("deliberate transaction error")
	err := txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := userRepo.Create(txCtx, user); err != nil {
			return err
		}
		if _, err := balanceRepo.AddBonus(txCtx, user.ID, 500, model.TxWelcomeBonus, nil, "Бонус"); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	// Verify user was NOT created due to rollback
	_, err = userRepo.GetByVKID(ctx, uniqueVKID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after rollback, got %v", err)
	}
}

func TestBalanceDeductUsage_OverdraftAndLedger(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Овердрафт",
		LastName:  "Тест",
		RefCode:   fmt.Sprintf("od_%d", uniqueVKID),
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Initial balance: 10 kopecks
	if _, err := balanceRepo.AddBonus(ctx, user.ID, 10, model.TxWelcomeBonus, nil, "Старт 10 коп"); err != nil {
		t.Fatalf("failed to seed balance: %v", err)
	}

	// 2. DeductUsage 0 kopecks: no-op, returns 10 kopecks
	b0, err := balanceRepo.DeductUsage(ctx, user.ID, 0, model.TxTokenCharge, nil, "0 tokens")
	if err != nil {
		t.Fatalf("unexpected error on 0 kopecks deduct: %v", err)
	}
	if b0.AmountKopecks != 10 {
		t.Errorf("expected 10 kopecks, got %d", b0.AmountKopecks)
	}

	// 3. DeductUsage 25 kopecks: overdraft from 10 to -15 kopecks
	refID := "req_test_123"
	b1, err := balanceRepo.DeductUsage(ctx, user.ID, 25, model.TxTokenCharge, &refID, "Списание за 50 токенов")
	if err != nil {
		t.Fatalf("unexpected error on overdraft deduct: %v", err)
	}
	if b1.AmountKopecks != -15 {
		t.Errorf("expected balance -15, got %d", b1.AmountKopecks)
	}

	// 4. Verify balance persisted in DB
	curBalance, err := balanceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if curBalance.AmountKopecks != -15 {
		t.Errorf("expected persisted balance -15, got %d", curBalance.AmountKopecks)
	}

	// 5. Verify transactions ledger
	txs, err := balanceRepo.GetTransactions(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get transactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions (bonus + token_charge), got %d", len(txs))
	}

	chargeTx := txs[0]
	if chargeTx.Type != model.TxTokenCharge {
		t.Errorf("expected tx type %s, got %s", model.TxTokenCharge, chargeTx.Type)
	}
	if chargeTx.AmountKopecks != -25 {
		t.Errorf("expected tx amount -25, got %d", chargeTx.AmountKopecks)
	}
	if chargeTx.BalanceAfterKopecks != -15 {
		t.Errorf("expected balance after -15, got %d", chargeTx.BalanceAfterKopecks)
	}
	if chargeTx.ReferenceID == nil || *chargeTx.ReferenceID != refID {
		t.Errorf("expected reference_id %s, got %v", refID, chargeTx.ReferenceID)
	}
}
