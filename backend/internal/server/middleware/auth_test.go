package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/server/middleware"
)

const testSecret = "test-jwt-secret-at-least-16-chars-long"

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	tokenManager, err := auth.NewJWTTokenManager(testSecret)
	if err != nil {
		t.Fatalf("failed to create token manager: %v", err)
	}

	authMw := middleware.AuthMiddleware(tokenManager)

	// Protected handler that verifies user ID from context
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "missing context user", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"user_id": userID})
	})

	handler := authMw(protectedHandler)

	t.Run("authorized via cookie", func(t *testing.T) {
		t.Parallel()

		token, err := tokenManager.GenerateToken(123, 456, time.Hour)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		//nolint:gosec // test cookie in mock request
		req.AddCookie(&http.Cookie{
			Name:  "auth_token",
			Value: token,
		})

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["user_id"] != float64(123) {
			t.Errorf("expected user_id 123, got %v", resp["user_id"])
		}
	})

	t.Run("authorized via Bearer header", func(t *testing.T) {
		t.Parallel()

		token, err := tokenManager.GenerateToken(789, 1011, time.Hour)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["user_id"] != float64(789) {
			t.Errorf("expected user_id 789, got %v", resp["user_id"])
		}
	})

	t.Run("unauthorized when no token", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("unauthorized with expired token", func(t *testing.T) {
		t.Parallel()

		token, _ := tokenManager.GenerateToken(123, 456, -time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		//nolint:gosec // test cookie in mock request
		req.AddCookie(&http.Cookie{
			Name:  "auth_token",
			Value: token,
		})

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("unauthorized with invalid signature", func(t *testing.T) {
		t.Parallel()

		otherManager, _ := auth.NewJWTTokenManager("another-different-valid-secret-key")
		token, _ := otherManager.GenerateToken(123, 456, time.Hour)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})
}
