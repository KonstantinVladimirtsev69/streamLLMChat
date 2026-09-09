package service

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/model"
	"backend/internal/repository"
)

// BillingService coordinates balance inquiries and token charges for LLM completions.
type BillingService interface {
	CanGenerate(ctx context.Context, userID int64) (bool, int64, error)
	ChargeTokens(ctx context.Context, userID int64, usage model.TokenUsage, modelID string, refID *string) (*model.Balance, error)
}

type billingService struct {
	balanceRepo repository.BalanceRepository
}

// NewBillingService creates a new BillingService instance.
func NewBillingService(balanceRepo repository.BalanceRepository) BillingService {
	return &billingService{
		balanceRepo: balanceRepo,
	}
}

// CanGenerate checks if user has sufficient positive balance to initiate generation.
func (s *billingService) CanGenerate(ctx context.Context, userID int64) (bool, int64, error) {
	b, err := s.balanceRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to fetch user balance: %w", err)
	}

	return b.AmountKopecks > 0, b.AmountKopecks, nil
}

// ChargeTokens debits token costs from user balance with overdraft support and ledger recording.
func (s *billingService) ChargeTokens(ctx context.Context, userID int64, usage model.TokenUsage, modelID string, refID *string) (*model.Balance, error) {
	if usage.CostKopecks <= 0 {
		return s.balanceRepo.GetByUserID(ctx, userID)
	}

	description := fmt.Sprintf("LLM token charge (%s): %d tokens", modelID, usage.TotalTokens)
	return s.balanceRepo.DeductUsage(ctx, userID, usage.CostKopecks, model.TxTokenCharge, refID, description)
}
