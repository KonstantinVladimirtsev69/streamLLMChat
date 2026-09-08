package model

import "time"

// TransactionType defines the category of a balance transaction.
type TransactionType string

const (
	// TxWelcomeBonus is credited upon user initial registration.
	TxWelcomeBonus TransactionType = "welcome_bonus"

	// TxReferralReward is credited to referrer when referee registers.
	TxReferralReward TransactionType = "referral_reward"

	// TxTokenCharge is debited for LLM token usage.
	TxTokenCharge TransactionType = "token_charge"

	// TxDeposit is credited via payment gateways (v2).
	TxDeposit TransactionType = "deposit"

	// TxAdjustment represents manual or administrative adjustments.
	TxAdjustment TransactionType = "adjustment"
)

// BalanceTransaction represents an immutable ledger entry for any balance modification.
type BalanceTransaction struct {
	ID                  int64           `json:"id"`
	UserID              int64           `json:"user_id"`
	AmountKopecks       int64           `json:"amount_kopecks"`
	BalanceAfterKopecks int64           `json:"balance_after_kopecks"`
	Type                TransactionType `json:"type"`
	ReferenceID         *string         `json:"reference_id,omitempty"`
	Description         string          `json:"description"`
	CreatedAt           time.Time       `json:"created_at"`
}
