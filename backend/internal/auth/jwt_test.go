package auth_test

import (
	"errors"
	"testing"
	"time"

	"backend/internal/auth"
)

const testSecret = "test-jwt-secret-at-least-16-chars-long"

func TestJWTTokenManager(t *testing.T) {
	t.Parallel()

	t.Run("weak secret rejected", func(t *testing.T) {
		t.Parallel()
		_, err := auth.NewJWTTokenManager("too-short")
		if !errors.Is(err, auth.ErrWeakSecret) {
			t.Fatalf("expected ErrWeakSecret, got %v", err)
		}
	})

	t.Run("generate and validate token successfully", func(t *testing.T) {
		t.Parallel()
		mgr, err := auth.NewJWTTokenManager(testSecret)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		tokenStr, err := mgr.GenerateToken(42, 10042, 1*time.Hour)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		claims, err := mgr.ValidateToken(tokenStr)
		if err != nil {
			t.Fatalf("expected valid token, got error: %v", err)
		}

		if claims.UserID != 42 {
			t.Errorf("expected UserID 42, got %d", claims.UserID)
		}
		if claims.VKID != 10042 {
			t.Errorf("expected VKID 10042, got %d", claims.VKID)
		}
		if claims.Subject != "42" {
			t.Errorf("expected Subject '42', got '%s'", claims.Subject)
		}
	})

	t.Run("expired token returns ErrExpiredToken", func(t *testing.T) {
		t.Parallel()
		mgr, err := auth.NewJWTTokenManager(testSecret)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		// Negative TTL
		tokenStr, err := mgr.GenerateToken(42, 10042, -1*time.Minute)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = mgr.ValidateToken(tokenStr)
		if !errors.Is(err, auth.ErrExpiredToken) {
			t.Fatalf("expected ErrExpiredToken, got %v", err)
		}
	})

	t.Run("token with different secret is rejected", func(t *testing.T) {
		t.Parallel()
		mgr1, _ := auth.NewJWTTokenManager(testSecret)
		mgr2, _ := auth.NewJWTTokenManager("another-different-valid-secret-key")

		tokenStr, err := mgr1.GenerateToken(42, 10042, 1*time.Hour)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = mgr2.ValidateToken(tokenStr)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("empty or garbage token is rejected", func(t *testing.T) {
		t.Parallel()
		mgr, _ := auth.NewJWTTokenManager(testSecret)

		_, err := mgr.ValidateToken("")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken for empty, got %v", err)
		}

		_, err = mgr.ValidateToken("invalid.token.string")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken for garbage, got %v", err)
		}
	})
}
