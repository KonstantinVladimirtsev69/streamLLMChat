package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"backend/internal/auth"
)

func TestVKOAuthClient(t *testing.T) {
	t.Parallel()

	cfg := auth.VKConfig{
		ClientID:     "123456",
		ClientSecret: "secretkey",
		RedirectURI:  "http://localhost:8080/api/v1/auth/vk/callback",
		FrontendURL:  "http://localhost:3000",
		MockAuth:     false,
	}

	client := auth.NewVKOAuthClient(cfg)

	t.Run("GetAuthURL builds valid authorize URL", func(t *testing.T) {
		t.Parallel()
		state := "csrf-token-12345"
		authURL := client.GetAuthURL(state)

		parsed, err := url.Parse(authURL)
		if err != nil {
			t.Fatalf("failed to parse auth URL: %v", err)
		}

		if !strings.Contains(parsed.Host, "oauth.vk.ru") {
			t.Errorf("expected host oauth.vk.ru, got %s", parsed.Host)
		}
		q := parsed.Query()
		if q.Get("client_id") != "123456" {
			t.Errorf("expected client_id 123456, got %s", q.Get("client_id"))
		}
		if q.Get("redirect_uri") != "http://localhost:8080/api/v1/auth/vk/callback" {
			t.Errorf("expected redirect_uri, got %s", q.Get("redirect_uri"))
		}
		if q.Get("state") != state {
			t.Errorf("expected state %s, got %s", state, q.Get("state"))
		}
	})

	t.Run("GetAuthURL respects custom BaseURL from VKConfig", func(t *testing.T) {
		t.Parallel()
		customCfg := cfg
		customCfg.BaseURL = "https://custom-oauth.example.com"
		customClient := auth.NewVKOAuthClient(customCfg)

		authURL := customClient.GetAuthURL("custom-state")
		parsed, err := url.Parse(authURL)
		if err != nil {
			t.Fatalf("failed to parse auth URL: %v", err)
		}
		if parsed.Host != "custom-oauth.example.com" {
			t.Errorf("expected host custom-oauth.example.com, got %s", parsed.Host)
		}
	})

	t.Run("ExchangeCode handles success with mocked HTTP server", func(t *testing.T) {
		t.Parallel()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "access_token") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token": "mock-access-token",
					"expires_in":   0,
					"user_id":      999888,
				})
				return
			}
			if strings.Contains(r.URL.Path, "users.get") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"response": []map[string]any{
						{
							"id":         999888,
							"first_name": "Алексей",
							"last_name":  "Смирнов",
							"photo_200":  "https://vk.com/photo200.jpg",
						},
					},
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer ts.Close()

		testClient := auth.NewVKOAuthClient(auth.VKConfig{
			ClientID:     "123456",
			ClientSecret: "secretkey",
			RedirectURI:  "http://localhost:8080/api/v1/auth/vk/callback",
			HTTPClient:   ts.Client(),
		})

		type baseURLExchanger interface {
			SetBaseURL(url string)
		}
		if b, ok := testClient.(baseURLExchanger); ok {
			b.SetBaseURL(ts.URL)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		profile, err := testClient.ExchangeCode(ctx, "valid-auth-code")
		if err != nil {
			t.Fatalf("expected successful exchange, got: %v", err)
		}

		if profile.ID != 999888 {
			t.Errorf("expected ID 999888, got %d", profile.ID)
		}
		if profile.FirstName != "Алексей" {
			t.Errorf("expected FirstName Алексей, got %s", profile.FirstName)
		}
		if profile.AvatarURL != "https://vk.com/photo200.jpg" {
			t.Errorf("expected AvatarURL, got %s", profile.AvatarURL)
		}
	})

	t.Run("IsMock returns true when MockAuth is true or client_id is empty", func(t *testing.T) {
		t.Parallel()
		mockClient1 := auth.NewVKOAuthClient(auth.VKConfig{MockAuth: true})
		if !mockClient1.IsMock() {
			t.Errorf("expected mockClient1 to be mock")
		}

		mockClient2 := auth.NewVKOAuthClient(auth.VKConfig{ClientID: ""})
		if !mockClient2.IsMock() {
			t.Errorf("expected mockClient2 to be mock")
		}
	})
}
