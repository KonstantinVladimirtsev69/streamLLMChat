package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/repository/postgres"
	"backend/internal/server/handler"
	"backend/internal/server/middleware"
	"backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := "postgres://postgres:postgres@127.0.0.1:5432/llmchat?sslmode=disable" //nolint:gosec

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping postgres integration test: %v", err)
	}

	if err := database.Up(dbURL); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return pool
}

func TestAuthHandler(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	referralRepo := postgres.NewReferralRepository(pool)
	txManager := database.NewTxManager(pool)
	refGen := auth.NewRefCodeGenerator(8)

	authSvc := service.NewAuthService(userRepo, balanceRepo, referralRepo, txManager, refGen, "http://localhost:3000")
	tokenManager, _ := auth.NewJWTTokenManager("super-secret-key-at-least-16-chars")

	mockVKClient := auth.NewVKOAuthClient(auth.VKConfig{MockAuth: true})

	h := handler.NewAuthHandler(authSvc, mockVKClient, tokenManager, handler.AuthHandlerConfig{
		FrontendURL: "http://localhost:3000",
		SessionTTL:  24 * time.Hour,
	})

	t.Run("Login redirects to mock in mock mode", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/vk/login", nil)
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("expected 302 Found, got %d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if !strings.Contains(loc, "/api/v1/auth/mock") {
			t.Errorf("expected location to contain /api/v1/auth/mock, got %s", loc)
		}
	})

	t.Run("MockLogin creates user, issues cookie and redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/mock?vk_id=888111&name=DevTester", nil)
		rec := httptest.NewRecorder()

		h.MockLogin(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("expected 302 Found, got %d", rec.Code)
		}
		if rec.Header().Get("Location") != "http://localhost:3000/chat" {
			t.Errorf("expected redirect to http://localhost:3000/chat, got %s", rec.Header().Get("Location"))
		}

		cookies := rec.Result().Cookies()
		var foundAuthToken bool
		for _, c := range cookies {
			if c.Name == "auth_token" && c.Value != "" && c.HttpOnly {
				foundAuthToken = true
			}
		}
		if !foundAuthToken {
			t.Errorf("expected HttpOnly auth_token cookie to be set")
		}
	})

	t.Run("Me returns profile when authenticated", func(t *testing.T) {
		// 1. Mock login to get user
		reqLogin := httptest.NewRequest(http.MethodGet, "/api/v1/auth/mock?vk_id=888222&name=MeUser", nil)
		recLogin := httptest.NewRecorder()
		h.MockLogin(recLogin, reqLogin)

		var authToken string
		for _, c := range recLogin.Result().Cookies() {
			if c.Name == "auth_token" {
				authToken = c.Value
			}
		}

		// 2. Call /me with AuthMiddleware
		authMw := middleware.AuthMiddleware(tokenManager)
		meHandler := authMw(http.HandlerFunc(h.Me))

		reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		//nolint:gosec // test cookie in mock request
		reqMe.AddCookie(&http.Cookie{Name: "auth_token", Value: authToken})
		recMe := httptest.NewRecorder()

		meHandler.ServeHTTP(recMe, reqMe)

		if recMe.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", recMe.Code, recMe.Body.String())
		}

		var dto service.UserDTO
		if err := json.Unmarshal(recMe.Body.Bytes(), &dto); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if dto.VKID != 888222 {
			t.Errorf("expected VKID 888222, got %d", dto.VKID)
		}
		if dto.BalanceKopecks != 500 {
			t.Errorf("expected 500 kopecks welcome bonus, got %d", dto.BalanceKopecks)
		}
		if dto.BalanceRub != 5.00 {
			t.Errorf("expected 5.00 rub, got %f", dto.BalanceRub)
		}
		if dto.RefCode == "" || dto.RefLink == "" {
			t.Errorf("expected non-empty ref_code and ref_link")
		}
	})

	t.Run("Logout clears auth_token cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		rec := httptest.NewRecorder()

		h.Logout(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var cleared bool
		for _, c := range rec.Result().Cookies() {
			if c.Name == "auth_token" && c.MaxAge < 0 {
				cleared = true
			}
		}
		if !cleared {
			t.Errorf("expected auth_token cookie to have MaxAge < 0")
		}
	})
}
