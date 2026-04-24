package copernicus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/xjock/sentinel-crawler/internal/config"
)

// tokenResponse OAuth2 token 响应
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// authenticator 管理 Copernicus Data Space 认证
type authenticator struct {
	cfg         config.CopernicusConfig
	httpClient  *http.Client
	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

// newAuthenticator 创建认证管理器
func newAuthenticator(cfg config.CopernicusConfig, httpClient *http.Client) *authenticator {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	}
	return &authenticator{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

// Token 获取有效 access token（自动刷新）
func (a *authenticator) Token(ctx context.Context) (string, error) {
	a.mu.RLock()
	token := a.accessToken
	expiresAt := a.expiresAt
	a.mu.RUnlock()

	// 提前 60 秒刷新
	if token != "" && time.Now().Add(60*time.Second).Before(expiresAt) {
		return token, nil
	}

	return a.refresh(ctx)
}

// refresh 从 Token 端点获取新 token
func (a *authenticator) refresh(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 双重检查
	if a.accessToken != "" && time.Now().Add(60*time.Second).Before(a.expiresAt) {
		return a.accessToken, nil
	}

	// 无凭证配置，返回空 token（搜索 API 公开访问）
	if a.cfg.ClientID == "" && a.cfg.ClientSecret == "" && a.cfg.Username == "" && a.cfg.Password == "" {
		return "", nil
	}

	data := url.Values{}

	// 优先使用 Client Credentials
	if a.cfg.ClientID != "" && a.cfg.ClientSecret != "" {
		data.Set("grant_type", "client_credentials")
		data.Set("client_id", a.cfg.ClientID)
		data.Set("client_secret", a.cfg.ClientSecret)
	} else if a.cfg.Username != "" && a.cfg.Password != "" {
		// 备选：Password Grant
		data.Set("grant_type", "password")
		data.Set("client_id", "cdse-public")
		data.Set("username", a.cfg.Username)
		data.Set("password", a.cfg.Password)
	} else {
		return "", fmt.Errorf("incomplete credentials configured for copernicus")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	a.accessToken = tr.AccessToken
	a.expiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)

	return tr.AccessToken, nil
}
