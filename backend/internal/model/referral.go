package model

import "time"

// Referral captures the link between a referrer and referee with reward amount.
type Referral struct {
	ID            int64     `json:"id"`
	ReferrerID    int64     `json:"referrer_id"`
	RefereeID     int64     `json:"referee_id"`
	RewardKopecks int64     `json:"reward_kopecks"`
	CreatedAt     time.Time `json:"created_at"`
}
