package model

import "time"

// Financial balance constants in kopecks (1 ruble = 100 kopecks).
const (
	// WelcomeBonusKopecks represents the welcome bonus for new users (5 rubles = 500 kopecks).
	WelcomeBonusKopecks int64 = 500

	// ReferralRewardKopecks represents the reward granted to the referrer (2 rubles = 200 kopecks).
	ReferralRewardKopecks int64 = 200
)

// Balance represents a user's current account balance in kopecks.
type Balance struct {
	UserID        int64     `json:"user_id"`
	AmountKopecks int64     `json:"amount_kopecks"`
	UpdatedAt     time.Time `json:"updated_at"`
}
