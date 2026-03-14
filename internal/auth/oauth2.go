package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuth2Provider OAuth2 提供商配置
type OAuth2Provider struct {
	Name         string   `json:"name"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	UserInfoURL  string   `json:"user_info_url"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
	Enabled      bool     `json:"enabled"`
}

// OAuth2Token OAuth2 令牌
type OAuth2Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// OAuth2UserInfo OAuth2 用户信息
type OAuth2UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Username  string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
}

// OAuth2State OAuth2 状态（防止 CSRF）
type OAuth2State struct {
	State     string    `json:"state"`
	Provider  string    `json:"provider"`
	Redirect  string    `json:"redirect"`
	CreatedAt time.Time `json:"created_at"`
}

// OAuth2Manager OAuth2 管理器
type OAuth2Manager struct {
	mu         sync.RWMutex
	providers  map[string]*OAuth2Provider
	states     map[string]*OAuth2State
	httpClient *http.Client
}

var (
	ErrOAuth2ProviderNotFound = errors.New("OAuth2 提供商未找到")
	ErrOAuth2StateInvalid     = errors.New("OAuth2 状态无效或已过期")
	ErrOAuth2TokenInvalid     = errors.New("OAuth2 令牌无效")
	ErrOAuth2UserInfoFailed   = errors.New("获取用户信息失败")
)

// NewOAuth2Manager 创建 OAuth2 管理器
func NewOAuth2Manager() *OAuth2Manager {
	return &OAuth2Manager{
		providers: make(map[string]*OAuth2Provider),
		states:    make(map[string]*OAuth2State),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterProvider 注册 OAuth2 提供商
func (m *OAuth2Manager) RegisterProvider(provider OAuth2Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[provider.Name] = &provider
	return nil
}

// GetProvider 获取 OAuth2 提供商
func (m *OAuth2Manager) GetProvider(name string) (*OAuth2Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[name]
	if !ok {
		return nil, ErrOAuth2ProviderNotFound
	}
	return provider, nil
}

// ListProviders 列出所有已启用的提供商
func (m *OAuth2Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var providers []string
	for name, p := range m.providers {
		if p.Enabled {
			providers = append(providers, name)
		}
	}
	return providers
}

// GenerateAuthURL 生成 OAuth2 授权 URL
func (m *OAuth2Manager) GenerateAuthURL(providerName, redirect string) (string, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return "", err
	}

	// 生成随机状态
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// 存储状态
	m.mu.Lock()
	m.states[state] = &OAuth2State{
		State:     state,
		Provider:  providerName,
		Redirect:  redirect,
		CreatedAt: time.Now(),
	}
	m.mu.Unlock()

	// 清理过期状态
	go m.cleanupStates()

	// 构建授权 URL
	params := url.Values{
		"client_id":     {provider.ClientID},
		"redirect_uri":  {provider.RedirectURL},
		"response_type": {"code"},
		"scope":         {strings.Join(provider.Scopes, " ")},
		"state":         {state},
	}

	authURL := provider.AuthURL + "?" + params.Encode()
	return authURL, nil
}

// ValidateState 验证 OAuth2 状态
func (m *OAuth2Manager) ValidateState(state string) (*OAuth2State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.states[state]
	if !ok {
		return nil, ErrOAuth2StateInvalid
	}

	// 状态有效期 10 分钟
	if time.Since(s.CreatedAt) > 10*time.Minute {
		delete(m.states, state)
		return nil, ErrOAuth2StateInvalid
	}

	// 删除已使用的状态
	delete(m.states, state)
	return s, nil
}

// ExchangeCode 使用授权码交换令牌
func (m *OAuth2Manager) ExchangeCode(ctx context.Context, providerName, code string) (*OAuth2Token, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"code":          {code},
		"redirect_uri":  {provider.RedirectURL},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", provider.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	token := &OAuth2Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	return token, nil
}

// GetUserInfo 获取用户信息
func (m *OAuth2Manager) GetUserInfo(ctx context.Context, providerName string, token *OAuth2Token) (*OAuth2UserInfo, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token.TokenType+" "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrOAuth2UserInfoFailed
	}

	var userInfo OAuth2UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	userInfo.Provider = providerName
	return &userInfo, nil
}

// RefreshToken 刷新令牌
func (m *OAuth2Manager) RefreshToken(ctx context.Context, providerName, refreshToken string) (*OAuth2Token, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", provider.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrOAuth2TokenInvalid
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	token := &OAuth2Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	return token, nil
}

// cleanupStates 清理过期状态
func (m *OAuth2Manager) cleanupStates() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for state, s := range m.states {
		if time.Since(s.CreatedAt) > 10*time.Minute {
			delete(m.states, state)
		}
	}
}

// ========== 预定义提供商配置 ==========

// GetGoogleOAuth2Config 获取 Google OAuth2 配置
func GetGoogleOAuth2Config(clientID, clientSecret, redirectURL string) OAuth2Provider {
	return OAuth2Provider{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Enabled:      true,
	}
}

// GetGitHubOAuth2Config 获取 GitHub OAuth2 配置
func GetGitHubOAuth2Config(clientID, clientSecret, redirectURL string) OAuth2Provider {
	return OAuth2Provider{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Enabled:      true,
	}
}

// GetMicrosoftOAuth2Config 获取 Microsoft OAuth2 配置
func GetMicrosoftOAuth2Config(clientID, clientSecret, redirectURL string) OAuth2Provider {
	return OAuth2Provider{
		Name:         "microsoft",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		UserInfoURL:  "https://graph.microsoft.com/v1.0/me",
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Enabled:      true,
	}
}

// GetWeChatOAuth2Config 获取微信 OAuth2 配置
func GetWeChatOAuth2Config(appID, appSecret, redirectURL string) OAuth2Provider {
	return OAuth2Provider{
		Name:         "wechat",
		ClientID:     appID,
		ClientSecret: appSecret,
		AuthURL:      "https://open.weixin.qq.com/connect/qrconnect",
		TokenURL:     "https://api.weixin.qq.com/sns/oauth2/access_token",
		UserInfoURL:  "https://api.weixin.qq.com/sns/userinfo",
		RedirectURL:  redirectURL,
		Scopes:       []string{"snsapi_login"},
		Enabled:      true,
	}
}
