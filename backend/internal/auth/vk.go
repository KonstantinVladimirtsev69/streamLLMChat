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

// VKProfile contains public profile information retrieved from VK API or VK ID.
type VKProfile struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}

// VKConfig holds configuration parameters for VK ID / VK OAuth 2.1.
type VKConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	FrontendURL  string
	BaseURL      string
	MockAuth     bool
	HTTPClient   *http.Client
}

// ExchangeParams contains parameters required for exchanging an authorization code.
type ExchangeParams struct {
	Code         string
	CodeVerifier string
	DeviceID     string
	State        string
}

// VKOAuthClient defines methods for OAuth interaction with VK / VK ID.
type VKOAuthClient interface {
	GetAuthURL(state string, codeChallenge ...string) string
	ExchangeCode(ctx context.Context, params ExchangeParams) (*VKProfile, error)
	IsMock() bool
}

type vkOAuthClient struct {
	cfg        VKConfig
	httpClient *http.Client
	baseURL    string
}

// NewVKOAuthClient creates an OAuth client based on configuration.
func NewVKOAuthClient(cfg VKConfig) VKOAuthClient {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://id.vk.ru"
	}
	return &vkOAuthClient{
		cfg:        cfg,
		httpClient: client,
		baseURL:    baseURL,
	}
}

// SetBaseURL allows overriding the base URL for unit/integration testing.
func (c *vkOAuthClient) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

func (c *vkOAuthClient) IsMock() bool {
	return c.cfg.MockAuth || c.cfg.ClientID == ""
}

// GetAuthURL generates the authorization URL for VK ID OAuth 2.1 (or fallback).
func (c *vkOAuthClient) GetAuthURL(state string, codeChallenge ...string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)

	if len(codeChallenge) > 0 && codeChallenge[0] != "" {
		q.Set("code_challenge", codeChallenge[0])
		q.Set("code_challenge_method", "s256")
		q.Set("scope", "vkid.personal_info")
	} else {
		q.Set("v", "5.131")
	}
	return fmt.Sprintf("%s/authorize?%s", c.baseURL, q.Encode())
}

type vkTokenResponse struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresIn   int         `json:"expires_in"`
	UserID      json.Number `json:"user_id"`
	Error       string      `json:"error"`
	ErrorDesc   string      `json:"error_description"`
}

type vkUserInfoResponse struct {
	User struct {
		ID        json.Number `json:"id"`
		UserID    json.Number `json:"user_id"`
		FirstName string      `json:"first_name"`
		LastName  string      `json:"last_name"`
		Avatar    string      `json:"avatar"`
		AvatarURL string      `json:"avatar_url"`
		Photo200  string      `json:"photo_200"`
	} `json:"user"`
	Error     string `json:"error"`
	ErrorDesc string `json:"error_description"`
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
func (c *vkOAuthClient) ExchangeCode(ctx context.Context, params ExchangeParams) (*VKProfile, error) {
	if params.Code == "" {
		return nil, fmt.Errorf("%w: code is empty", ErrOAuthFailed)
	}

	tokenURL := fmt.Sprintf("%s/oauth2/auth", c.baseURL)
	if strings.Contains(c.baseURL, "oauth.vk.") {
		tokenURL = fmt.Sprintf("%s/access_token", c.baseURL)
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", params.Code)
	data.Set("client_id", c.cfg.ClientID)
	data.Set("client_secret", c.cfg.ClientSecret)
	data.Set("redirect_uri", c.cfg.RedirectURI)
	if params.CodeVerifier != "" {
		data.Set("code_verifier", params.CodeVerifier)
	}
	if params.DeviceID != "" {
		data.Set("device_id", params.DeviceID)
	}
	if params.State != "" {
		data.Set("state", params.State)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
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

	var tokenData vkTokenResponse
	if err := json.Unmarshal(body, &tokenData); err != nil {
		return nil, fmt.Errorf("%w: failed to parse token json: %v", ErrOAuthFailed, err)
	}

	if tokenData.Error != "" || tokenData.AccessToken == "" {
		return nil, fmt.Errorf("%w: %s (%s)", ErrOAuthFailed, tokenData.Error, tokenData.ErrorDesc)
	}

	// First try modern VK ID user_info endpoint
	profile, err := c.fetchUserInfo(ctx, tokenData.AccessToken)
	if err == nil && profile != nil && profile.ID > 0 {
		return profile, nil
	}

	// Fallback to api.vk.com/method/users.get
	userID, _ := tokenData.UserID.Int64()
	return c.fetchUsersGet(ctx, tokenData.AccessToken, userID)
}

func (c *vkOAuthClient) fetchUserInfo(ctx context.Context, accessToken string) (*VKProfile, error) {
	userInfoURL := fmt.Sprintf("%s/oauth2/user_info", c.baseURL)
	data := url.Values{}
	data.Set("client_id", c.cfg.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, userInfoURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user_info status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res vkUserInfoResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.Error != "" {
		return nil, fmt.Errorf("user_info error: %s (%s)", res.Error, res.ErrorDesc)
	}

	uid, _ := res.User.UserID.Int64()
	if uid == 0 {
		uid, _ = res.User.ID.Int64()
	}
	if uid == 0 && res.User.FirstName == "" {
		return nil, errors.New("empty user_info")
	}

	avatar := res.User.Avatar
	if avatar == "" {
		avatar = res.User.AvatarURL
	}
	if avatar == "" {
		avatar = res.User.Photo200
	}

	return &VKProfile{
		ID:        uid,
		FirstName: res.User.FirstName,
		LastName:  res.User.LastName,
		AvatarURL: avatar,
	}, nil
}

func (c *vkOAuthClient) fetchUsersGet(ctx context.Context, accessToken string, userID int64) (*VKProfile, error) {
	userGetURL := "https://api.vk.com/method/users.get"
	if strings.Contains(c.baseURL, "127.0.0.1") || strings.Contains(c.baseURL, "localhost") {
		userGetURL = fmt.Sprintf("%s/method/users.get", c.baseURL)
	}

	uq := url.Values{}
	if userID > 0 {
		uq.Set("user_ids", strconv.FormatInt(userID, 10))
	}
	uq.Set("fields", "photo_200")
	uq.Set("access_token", accessToken)
	uq.Set("v", "5.131")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userGetURL+"?"+uq.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build users.get request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to fetch profile: %v", ErrOAuthFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read profile response: %v", ErrOAuthFailed, err)
	}

	var res vkUsersGetResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: failed to parse profile json: %v", ErrOAuthFailed, err)
	}
	if res.Error != nil {
		return nil, fmt.Errorf("%w: %s (code %d)", ErrOAuthFailed, res.Error.ErrorMsg, res.Error.ErrorCode)
	}
	if len(res.Response) == 0 {
		return nil, fmt.Errorf("%w: empty profile list in vk response", ErrOAuthFailed)
	}

	raw := res.Response[0]
	return &VKProfile{
		ID:        raw.ID,
		FirstName: raw.FirstName,
		LastName:  raw.LastName,
		AvatarURL: raw.Photo200,
	}, nil
}
