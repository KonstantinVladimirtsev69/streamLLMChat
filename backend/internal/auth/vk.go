package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrOAuthFailed is returned when VK OAuth token exchange or profile retrieval fails.
	ErrOAuthFailed = errors.New("vk oauth exchange failed")
)

// VKProfile contains public profile information retrieved from VK API.
type VKProfile struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}

// VKConfig holds configuration parameters for VK OAuth / VK ID.
type VKConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	BaseURL      string
	APIBaseURL   string
	FrontendURL  string
	MockAuth     bool
	HTTPClient   *http.Client
}

// ExchangeParams holds authorization code and optional PKCE parameters for VK ID exchange.
type ExchangeParams struct {
	Code         string
	CodeVerifier string
	DeviceID     string
	State        string
}

// VKOAuthClient defines methods for OAuth 2.0 and VK ID interaction with VK.
type VKOAuthClient interface {
	GetAuthURL(state string, codeChallenge ...string) string
	ExchangeCode(ctx context.Context, code string) (*VKProfile, error)
	ExchangeCodeWithParams(ctx context.Context, params ExchangeParams) (*VKProfile, error)
	IsMock() bool
}

type vkOAuthClient struct {
	cfg        VKConfig
	httpClient *http.Client
	baseURL    string // overrideable in tests
	apiBaseURL string
}

// NewVKOAuthClient creates an OAuth client based on configuration.
func NewVKOAuthClient(cfg VKConfig) VKOAuthClient {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://id.vk.ru"
	}
	apiBaseURL := strings.TrimRight(cfg.APIBaseURL, "/")
	if apiBaseURL == "" {
		apiBaseURL = "https://api.vk.ru"
	}
	return &vkOAuthClient{
		cfg:        cfg,
		httpClient: client,
		baseURL:    baseURL,
		apiBaseURL: apiBaseURL,
	}
}

// SetBaseURL allows overriding the base URL for unit/integration testing.
func (c *vkOAuthClient) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
	c.apiBaseURL = strings.TrimRight(url, "/")
}

func (c *vkOAuthClient) IsMock() bool {
	return c.cfg.MockAuth || c.cfg.ClientID == ""
}

// GetAuthURL generates the full authorization URL for VK ID / VK OAuth.
func (c *vkOAuthClient) GetAuthURL(state string, codeChallenge ...string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)

	if strings.Contains(c.baseURL, "id.vk") {
		q.Set("scope", "vkid.personal_info")
	} else {
		q.Set("v", "5.131")
	}

	if len(codeChallenge) > 0 && codeChallenge[0] != "" {
		q.Set("code_challenge", codeChallenge[0])
		q.Set("code_challenge_method", "S256")
	}
	return fmt.Sprintf("%s/authorize?%s", c.baseURL, q.Encode())
}

// ExchangeCode exchanges code for profile using default parameters.
func (c *vkOAuthClient) ExchangeCode(ctx context.Context, code string) (*VKProfile, error) {
	return c.ExchangeCodeWithParams(ctx, ExchangeParams{Code: code})
}

// ExchangeCodeWithParams exchanges code and PKCE parameters for user profile.
func (c *vkOAuthClient) ExchangeCodeWithParams(ctx context.Context, params ExchangeParams) (*VKProfile, error) {
	if params.Code == "" {
		return nil, fmt.Errorf("%w: code is empty", ErrOAuthFailed)
	}

	if strings.Contains(c.baseURL, "id.vk") {
		return c.exchangeVKID(ctx, params)
	}
	return c.exchangeLegacyOAuth(ctx, params.Code)
}

func (c *vkOAuthClient) exchangeVKID(ctx context.Context, params ExchangeParams) (*VKProfile, error) {
	tokenURL := fmt.Sprintf("%s/oauth2/auth", c.baseURL)
	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("client_id", c.cfg.ClientID)
	formData.Set("code", params.Code)
	formData.Set("redirect_uri", c.cfg.RedirectURI)
	if params.CodeVerifier != "" {
		formData.Set("code_verifier", params.CodeVerifier)
	}
	if params.DeviceID != "" {
		formData.Set("device_id", params.DeviceID)
	}
	if params.State != "" {
		formData.Set("state", params.State)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: network error: %v", ErrOAuthFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read token response: %v", ErrOAuthFailed, err)
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
		UserID      any    `json:"user_id"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenData); err != nil {
		return nil, fmt.Errorf("%w: failed to parse token json: %v", ErrOAuthFailed, err)
	}
	if tokenData.Error != "" || tokenData.AccessToken == "" {
		return nil, fmt.Errorf("%w: %s (%s)", ErrOAuthFailed, tokenData.Error, tokenData.ErrorDesc)
	}

	userInfoURL := fmt.Sprintf("%s/oauth2/user_info", c.baseURL)
	uForm := url.Values{}
	uForm.Set("client_id", c.cfg.ClientID)
	uForm.Set("access_token", tokenData.AccessToken)

	uReq, err := http.NewRequestWithContext(ctx, http.MethodPost, userInfoURL, strings.NewReader(uForm.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to build user_info request: %w", err)
	}
	uReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	uResp, err := c.httpClient.Do(uReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to fetch profile: %v", ErrOAuthFailed, err)
	}
	defer func() { _ = uResp.Body.Close() }()

	uBody, err := io.ReadAll(uResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read profile response: %v", ErrOAuthFailed, err)
	}

	var userInfoResp struct {
		User struct {
			UserID    any    `json:"user_id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Avatar    string `json:"avatar"`
		} `json:"user"`
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if err := json.Unmarshal(uBody, &userInfoResp); err != nil {
		return nil, fmt.Errorf("%w: failed to parse user_info json: %v", ErrOAuthFailed, err)
	}
	if userInfoResp.Error != "" {
		return nil, fmt.Errorf("%w: %s (%s)", ErrOAuthFailed, userInfoResp.Error, userInfoResp.ErrorDesc)
	}

	uid := parseUserID(userInfoResp.User.UserID)
	if uid == 0 {
		uid = parseUserID(tokenData.UserID)
	}

	return &VKProfile{
		ID:        uid,
		FirstName: userInfoResp.User.FirstName,
		LastName:  userInfoResp.User.LastName,
		AvatarURL: userInfoResp.User.Avatar,
	}, nil
}

func (c *vkOAuthClient) exchangeLegacyOAuth(ctx context.Context, code string) (*VKProfile, error) {
	tokenURL := fmt.Sprintf("%s/access_token", c.baseURL)
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("client_secret", c.cfg.ClientSecret)
	q.Set("redirect_uri", c.cfg.RedirectURI)
	q.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build token request: %w", err)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: network error: %v", ErrOAuthFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read token response: %v", ErrOAuthFailed, err)
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
		UserID      int64  `json:"user_id"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenData); err != nil {
		return nil, fmt.Errorf("%w: failed to parse token json: %v", ErrOAuthFailed, err)
	}
	if tokenData.Error != "" || tokenData.AccessToken == "" {
		return nil, fmt.Errorf("%w: %s (%s)", ErrOAuthFailed, tokenData.Error, tokenData.ErrorDesc)
	}

	userGetURL := fmt.Sprintf("%s/method/users.get", c.apiBaseURL)
	uq := url.Values{}
	uq.Set("user_ids", fmt.Sprintf("%d", tokenData.UserID))
	uq.Set("fields", "photo_200")
	uq.Set("access_token", tokenData.AccessToken)
	uq.Set("v", "5.131")

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, userGetURL+"?"+uq.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build users.get request: %w", err)
	}

	userResp, err := c.httpClient.Do(userReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to fetch profile: %v", ErrOAuthFailed, err)
	}
	defer func() { _ = userResp.Body.Close() }()

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read profile response: %v", ErrOAuthFailed, err)
	}

	var userGetData struct {
		Response []struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Photo200  string `json:"photo_200"`
		} `json:"response"`
		Error *struct {
			ErrorCode int    `json:"error_code"`
			ErrorMsg  string `json:"error_msg"`
		} `json:"error"`
	}
	if err := json.Unmarshal(userBody, &userGetData); err != nil {
		return nil, fmt.Errorf("%w: failed to parse profile json: %v", ErrOAuthFailed, err)
	}
	if userGetData.Error != nil {
		return nil, fmt.Errorf("%w: %s (code %d)", ErrOAuthFailed, userGetData.Error.ErrorMsg, userGetData.Error.ErrorCode)
	}
	if len(userGetData.Response) == 0 {
		return nil, fmt.Errorf("%w: empty profile list in vk response", ErrOAuthFailed)
	}

	raw := userGetData.Response[0]
	return &VKProfile{
		ID:        raw.ID,
		FirstName: raw.FirstName,
		LastName:  raw.LastName,
		AvatarURL: raw.Photo200,
	}, nil
}

func parseUserID(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		id, _ := strconv.ParseInt(val, 10, 64)
		return id
	case json.Number:
		id, _ := val.Int64()
		return id
	default:
		return 0
	}
}
