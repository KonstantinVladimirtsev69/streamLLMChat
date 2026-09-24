package auth_test

import (
	"context"
	"encoding/json"
	"io"
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

	t.Run("GetAuthURL builds valid VK ID authorize URL with PKCE", func(t *testing.T) {
		t.Parallel()
		state := "csrf-token-12345"
		challenge := "challenge-code-s256"
		authURL := client.GetAuthURL(state, challenge)

		parsed, err := url.Parse(authURL)
		if err != nil {
			t.Fatalf("failed to parse auth URL: %v", err)
		}

		if !strings.Contains(parsed.Host, "id.vk.ru") && !strings.Contains(parsed.Host, "id.vk.com") {
			t.Errorf("expected host id.vk.ru or id.vk.com, got %s", parsed.Host)
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
		if q.Get("code_challenge") != challenge {
			t.Errorf("expected code_challenge %s, got %s", challenge, q.Get("code_challenge"))
		}
		if q.Get("code_challenge_method") != "s256" {
			t.Errorf("expected code_challenge_method s256, got %s", q.Get("code_challenge_method"))
		}
		if !strings.Contains(q.Get("scope"), "vkid.personal_info") {
			t.Errorf("expected scope to contain vkid.personal_info, got %s", q.Get("scope"))
		}
	})

	t.Run("ExchangeCode handles VK ID OAuth2.1 flow with PKCE and user_info", func(t *testing.T) {
		t.Parallel()

		var tokenRequestBody string
		var userInfoRequested bool

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "oauth2/auth") {
				bodyBytes, _ := io.ReadAll(r.Body)
				tokenRequestBody = string(bodyBytes)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token": "vkid-access-token-999",
					"token_type":   "Bearer",
					"expires_in":   3600,
					"user_id":      999888,
				})
				return
			}
			if strings.Contains(r.URL.Path, "oauth2/user_info") {
				userInfoRequested = true
				_ = json.NewEncoder(w).Encode(map[string]any{
					"user": map[string]any{
						"user_id":    999888,
						"first_name": "Алексей",
						"last_name":  "Смирнов",
						"avatar":     "https://vk.com/photo200.jpg",
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
			BaseURL:      ts.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		profile, err := testClient.ExchangeCode(ctx, auth.ExchangeParams{
			Code:         "valid-auth-code",
			CodeVerifier: "verifier-12345",
			DeviceID:     "device-abc",
			State:        "state-xyz",
		})
		if err != nil {
			t.Fatalf("expected successful exchange, got: %v", err)
		}

		if !strings.Contains(tokenRequestBody, "code_verifier=verifier-12345") {
			t.Errorf("expected token request body to contain code_verifier, got: %s", tokenRequestBody)
		}
		if !strings.Contains(tokenRequestBody, "device_id=device-abc") {
			t.Errorf("expected token request body to contain device_id, got: %s", tokenRequestBody)
		}
		if !userInfoRequested {
			t.Errorf("expected user_info endpoint to be called")
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
