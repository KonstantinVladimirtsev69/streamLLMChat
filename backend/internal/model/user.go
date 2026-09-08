package model

import "time"

// User represents a registered user authenticated via VK OAuth.
type User struct {
	ID         int64     `json:"id"`
	VKID       int64     `json:"vk_id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	AvatarURL  string    `json:"avatar_url"`
	RefCode    string    `json:"ref_code"`
	ReferredBy *int64    `json:"referred_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
