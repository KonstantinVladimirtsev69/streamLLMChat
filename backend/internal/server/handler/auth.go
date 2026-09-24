package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/server/middleware"
	"backend/internal/service"
)

// AuthHandlerConfig holds HTTP and cookie configuration for authentication endpoints.
type AuthHandlerConfig struct {
	CookieDomain string
	CookieSecure bool
	FrontendURL  string
	SessionTTL   time.Duration
}

// AuthHandler handles authentication routes: VK OAuth login/callback, mock login, me, and logout.
type AuthHandler struct {
	authService  service.AuthService
	vkClient     auth.VKOAuthClient
	tokenManager auth.TokenManager
	cfg          AuthHandlerConfig
}

// NewAuthHandler constructs a new AuthHandler with dependencies.
func NewAuthHandler(
	authService service.AuthService,
	vkClient auth.VKOAuthClient,
	tokenManager auth.TokenManager,
	cfg AuthHandlerConfig,
) *AuthHandler {
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:3000"
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 30 * 24 * time.Hour
	}
	return &AuthHandler{
		authService:  authService,
		vkClient:     vkClient,
		tokenManager: tokenManager,
		cfg:          cfg,
	}
}

type oauthStatePayload struct {
	State        string `json:"state"`
	RefCode      string `json:"ref_code,omitempty"`
	ReturnTo     string `json:"return_to,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty"`
}

// Login initiates the VK OAuth flow or redirects to mock in dev mode.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	refCode := strings.TrimSpace(r.URL.Query().Get("ref"))
	returnTo := strings.TrimSpace(r.URL.Query().Get("return_to"))

	if h.vkClient.IsMock() {
		mockURL := fmt.Sprintf("/api/v1/auth/mock?ref=%s&return_to=%s", url.QueryEscape(refCode), url.QueryEscape(returnTo))
		h.redirect(w, r, mockURL, http.StatusFound)
		return
	}

	// Generate PKCE code verifier and challenge
	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		h.redirectError(w, r, "pkce_failed")
		return
	}

	// Generate random 16-byte state
	var randBytes [16]byte
	_, _ = rand.Read(randBytes[:])
	stateHex := hex.EncodeToString(randBytes[:])

	payload := oauthStatePayload{
		State:        stateHex,
		RefCode:      refCode,
		ReturnTo:     returnTo,
		CodeVerifier: verifier,
	}
	payloadBytes, _ := json.Marshal(payload)
	cookieVal := hex.EncodeToString(payloadBytes)

	//nolint:gosec // cookie security dynamically configured via CookieSecure
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    cookieVal,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.CookieSecure,
	})

	authURL := h.vkClient.GetAuthURL(stateHex, challenge)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback processes the response from VK OAuth.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		h.redirectError(w, r, "oauth_"+errParam)
		return
	}

	stateParam := q.Get("state")
	codeParam := q.Get("code")
	deviceIDParam := q.Get("device_id")

	// Read and validate state cookie
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" {
		h.redirectError(w, r, "missing_state")
		return
	}

	decodedBytes, err := hex.DecodeString(stateCookie.Value)
	if err != nil {
		h.redirectError(w, r, "invalid_state")
		return
	}

	var stateData oauthStatePayload
	if err := json.Unmarshal(decodedBytes, &stateData); err != nil || stateData.State != stateParam {
		h.redirectError(w, r, "state_mismatch")
		return
	}

	// Clear oauth_state cookie
	//nolint:gosec // cookie security dynamically configured via CookieSecure
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.CookieSecure,
	})

	// Exchange code for profile
	profile, err := h.vkClient.ExchangeCode(r.Context(), auth.ExchangeParams{
		Code:         codeParam,
		CodeVerifier: stateData.CodeVerifier,
		DeviceID:     deviceIDParam,
		State:        stateParam,
	})
	if err != nil {
		h.redirectError(w, r, "code_exchange_failed")
		return
	}

	user, err := h.authService.AuthenticateOrRegister(r.Context(), *profile, stateData.RefCode)
	if err != nil {
		h.redirectError(w, r, "registration_failed")
		return
	}

	// Issue JWT token
	tokenStr, err := h.tokenManager.GenerateToken(user.ID, user.VKID, h.cfg.SessionTTL)
	if err != nil {
		h.redirectError(w, r, "token_generation_failed")
		return
	}

	// Set auth_token session cookie
	//nolint:gosec // cookie security dynamically configured via CookieSecure
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenStr,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.CookieSecure,
	})

	safePath := "/chat"
	if stateData.ReturnTo != "" && strings.HasPrefix(stateData.ReturnTo, "/") && !strings.HasPrefix(stateData.ReturnTo, "//") && !strings.Contains(stateData.ReturnTo, `\`) {
		safePath = stateData.ReturnTo
	}
	targetURL := h.cfg.FrontendURL + safePath

	//nolint:gosec // targetURL is strictly built from configured FrontendURL and sanitized relative path
	http.Redirect(w, r, targetURL, http.StatusFound)
}

// MockLogin allows instant dev/test login without external VK API calls.
func (h *AuthHandler) MockLogin(w http.ResponseWriter, r *http.Request) {
	if !h.vkClient.IsMock() {
		http.Error(w, "mock login is disabled in production", http.StatusForbidden)
		return
	}

	q := r.URL.Query()
	vkIDStr := q.Get("vk_id")
	var vkID int64 = 999001
	if vkIDStr != "" {
		if parsed, err := strconv.ParseInt(vkIDStr, 10, 64); err == nil && parsed > 0 {
			vkID = parsed
		}
	}

	name := q.Get("name")
	if name == "" {
		name = "Тестовый Пользователь"
	}
	refCode := q.Get("ref")
	returnTo := q.Get("return_to")

	profile := auth.VKProfile{
		ID:        vkID,
		FirstName: name,
		LastName:  "Dev",
		AvatarURL: "https://api.dicebear.com/7.x/identicon/svg?seed=dev",
	}

	user, err := h.authService.AuthenticateOrRegister(r.Context(), profile, refCode)
	if err != nil {
		http.Error(w, fmt.Sprintf("mock auth failed: %v", err), http.StatusInternalServerError)
		return
	}

	tokenStr, err := h.tokenManager.GenerateToken(user.ID, user.VKID, h.cfg.SessionTTL)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	//nolint:gosec // cookie security dynamically configured via CookieSecure
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenStr,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.CookieSecure,
	})

	safePath := "/chat"
	if returnTo != "" && strings.HasPrefix(returnTo, "/") && !strings.HasPrefix(returnTo, "//") && !strings.Contains(returnTo, `\`) {
		safePath = returnTo
	}
	targetURL := h.cfg.FrontendURL + safePath

	//nolint:gosec // targetURL is strictly built from configured FrontendURL and sanitized relative path
	http.Redirect(w, r, targetURL, http.StatusFound)
}

// Me returns the authenticated user's profile, balance, and referral link.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	profile, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch user profile"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

// Logout clears the auth_token cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	//nolint:gosec // cookie security dynamically configured via CookieSecure
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.CookieSecure,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (h *AuthHandler) redirect(w http.ResponseWriter, r *http.Request, target string, status int) {
	if strings.HasPrefix(target, "/") {
		proto := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "https"
		}
		host := r.Host
		if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}
		target = fmt.Sprintf("%s://%s%s", proto, host, target)
	}
	//nolint:gosec // target is controlled application redirect path
	http.Redirect(w, r, target, status)
}

func (h *AuthHandler) redirectError(w http.ResponseWriter, r *http.Request, reason string) {
	target := fmt.Sprintf("%s/?error=%s", h.cfg.FrontendURL, url.QueryEscape(reason))
	//nolint:gosec // target is strictly built from configured FrontendURL
	http.Redirect(w, r, target, http.StatusFound)
}
