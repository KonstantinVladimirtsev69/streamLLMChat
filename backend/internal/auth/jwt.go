package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken is returned when the JWT token signature or format is invalid.
	ErrInvalidToken = errors.New("invalid or malformed token")

	// ErrExpiredToken is returned when the JWT token has expired.
	ErrExpiredToken = errors.New("token has expired")

	// ErrWeakSecret is returned when the JWT secret key is shorter than 16 bytes.
	ErrWeakSecret = errors.New("jwt secret must be at least 16 bytes long")
)

// Claims represents the JWT payload containing user identification and expiration.
type Claims struct {
	UserID int64 `json:"uid"`
	VKID   int64 `json:"vk_id"`
	jwt.RegisteredClaims
}

// TokenManager defines the contract for generating and validating authentication tokens.
type TokenManager interface {
	GenerateToken(userID int64, vkID int64, ttl time.Duration) (string, error)
	ValidateToken(tokenStr string) (*Claims, error)
}

type jwtTokenManager struct {
	secret []byte
}

// NewJWTTokenManager creates a new TokenManager instance using HMAC-SHA256.
func NewJWTTokenManager(secret string) (TokenManager, error) {
	if len(secret) < 16 {
		return nil, ErrWeakSecret
	}
	return &jwtTokenManager{
		secret: []byte(secret),
	}, nil
}

// GenerateToken generates and signs a new JWT token for the specified user and TTL.
func (m *jwtTokenManager) GenerateToken(userID int64, vkID int64, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		VKID:   vkID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenStr, nil
}

// ValidateToken parses, validates the signature, and returns the claims of the token.
func (m *jwtTokenManager) ValidateToken(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, ErrInvalidToken
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
