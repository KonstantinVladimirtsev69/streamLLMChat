package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/model"
	"backend/internal/repository"
)

// VKProfile contains sanitized profile information retrieved from VK API.
type VKProfile struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}

// UserDTO represents the public profile and balance data returned to the client.
type UserDTO struct {
	ID             int64   `json:"id"`
	VKID           int64   `json:"vk_id"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	AvatarURL      string  `json:"avatar_url"`
	RefCode        string  `json:"ref_code"`
	RefLink        string  `json:"ref_link"`
	BalanceRub     float64 `json:"balance_rub"`
	BalanceKopecks int64   `json:"balance_kopecks"`
}

// AuthService defines business operations for user authentication, registration, and referrals.
type AuthService interface {
	AuthenticateOrRegister(ctx context.Context, profile VKProfile, refCode string) (*model.User, error)
	GetProfile(ctx context.Context, userID int64) (*UserDTO, error)
}

type authService struct {
	userRepo     repository.UserRepository
	balanceRepo  repository.BalanceRepository
	referralRepo repository.ReferralRepository
	txManager    database.TxManager
	refGen       auth.RefCodeGenerator
	frontendURL  string
}

// NewAuthService constructs a new AuthService instance with all dependencies.
func NewAuthService(
	userRepo repository.UserRepository,
	balanceRepo repository.BalanceRepository,
	referralRepo repository.ReferralRepository,
	txManager database.TxManager,
	refGen auth.RefCodeGenerator,
	frontendURL string,
) AuthService {
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	return &authService{
		userRepo:     userRepo,
		balanceRepo:  balanceRepo,
		referralRepo: referralRepo,
		txManager:    txManager,
		refGen:       refGen,
		frontendURL:  frontendURL,
	}
}

// AuthenticateOrRegister identifies an existing user or creates a new one with bonuses in a single transaction.
func (s *authService) AuthenticateOrRegister(ctx context.Context, profile VKProfile, refCode string) (*model.User, error) {
	// 1. Check if user already exists
	existingUser, err := s.userRepo.GetByVKID(ctx, profile.ID)
	if err == nil && existingUser != nil {
		return existingUser, nil
	}
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// 2. Register new user atomically within transaction
	var createdUser *model.User
	txErr := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Double check within transaction in case of concurrent registration
		u, err := s.userRepo.GetByVKID(txCtx, profile.ID)
		if err == nil && u != nil {
			createdUser = u
			return nil
		}

		// Resolve referrer if refCode was provided
		var referrerID *int64
		if refCode != "" {
			refUser, err := s.userRepo.GetByRefCode(txCtx, refCode)
			if err == nil && refUser != nil {
				referrerID = &refUser.ID
			}
		}

		newUser := &model.User{
			VKID:       profile.ID,
			FirstName:  profile.FirstName,
			LastName:   profile.LastName,
			AvatarURL:  profile.AvatarURL,
			RefCode:    s.refGen.GenerateRefCode(),
			ReferredBy: referrerID,
		}

		if err := s.userRepo.Create(txCtx, newUser); err != nil {
			return fmt.Errorf("failed to insert user: %w", err)
		}

		// Initial welcome bonus (+500 kopecks / 5.00 rubles)
		desc := "Приветственный бонус при регистрации"
		if _, err := s.balanceRepo.AddBonus(txCtx, newUser.ID, model.WelcomeBonusKopecks, model.TxWelcomeBonus, nil, desc); err != nil {
			return fmt.Errorf("failed to credit welcome bonus: %w", err)
		}

		// Process referral reward (+200 kopecks / 2.00 rubles to referrer)
		if referrerID != nil && *referrerID != newUser.ID {
			if err := s.referralRepo.Create(txCtx, *referrerID, newUser.ID, model.ReferralRewardKopecks); err == nil {
				refDesc := fmt.Sprintf("Реферальное вознаграждение за пользователя %d", newUser.ID)
				refIDStr := strconv.FormatInt(newUser.ID, 10)
				_, _ = s.balanceRepo.AddBonus(txCtx, *referrerID, model.ReferralRewardKopecks, model.TxReferralReward, &refIDStr, refDesc)
			}
		}

		createdUser = newUser
		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	return createdUser, nil
}

// GetProfile retrieves user details, computes referral link and formats balance.
func (s *authService) GetProfile(ctx context.Context, userID int64) (*UserDTO, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var balanceKopecks int64
	bal, err := s.balanceRepo.GetByUserID(ctx, userID)
	if err == nil && bal != nil {
		balanceKopecks = bal.AmountKopecks
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	dto := &UserDTO{
		ID:             user.ID,
		VKID:           user.VKID,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		AvatarURL:      user.AvatarURL,
		RefCode:        user.RefCode,
		RefLink:        fmt.Sprintf("%s/?ref=%s", s.frontendURL, user.RefCode),
		BalanceKopecks: balanceKopecks,
		BalanceRub:     float64(balanceKopecks) / 100.0,
	}

	return dto, nil
}
