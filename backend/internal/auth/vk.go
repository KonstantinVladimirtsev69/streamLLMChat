package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// VKConfig holds configuration parameters for VK OAuth 2.0.
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

// VKOAuthClient defines methods for OAuth 2.0 interaction with VK.
type VKOAuthClient interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*VKProfile, error)
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
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://oauth.vk.ru"
	}
	apiBaseURL := cfg.APIBaseURL
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
	c.baseURL = url
	c.apiBaseURL = url
}

func (c *vkOAuthClient) IsMock() bool {
	return c.cfg.MockAuth || c.cfg.ClientID == ""
}

// GetAuthURL generates the full authorization URL for VK OAuth.
func (c *vkOAuthClient) GetAuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURI)
	q.Set("response_type", "code")
	q.Set("v", "5.131")
	q.Set("state", state)
	return fmt.Sprintf("%s/authorize?%s", c.baseURL, q.Encode())
}

type vkTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	UserID      int64  `json:"user_id"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type vkUsersGetResponse struct {
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

// ExchangeCode exchanges the authorization code for access token and user profile.
func (c *vkOAuthClient) ExchangeCode(ctx context.Context, code string) (*VKProfile, error) {
	if code == "" {
		return nil, fmt.Errorf("%w: code is empty", ErrOAuthFailed)
	}

	// 1. Exchange code for access_token
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
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read token response: %v", ErrOAuthFailed, err)
	}

	var tokenData vkTokenResponse
	if err := json.Unmarshal(body, &tokenData); err != nil {
		return nil, fmt.Errorf("%w: failed to parse token json: %v", ErrOAuthFailed, err)
	}

	if tokenData.Error != "" || tokenData.AccessToken == "" {
		return nil, fmt.Errorf("%w: %s (%s)", ErrOAuthFailed, tokenData.Error, tokenData.ErrorDesc)
	}

	// 2. Fetch user profile from api.vk.ru (or configured apiBaseURL)
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
	defer func() {
		_ = userResp.Body.Close()
	}()

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read profile response: %v", ErrOAuthFailed, err)
	}

	var userGetData vkUsersGetResponse
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
