// Package cloudsync provides cloud storage synchronization
// This file implements OAuth2 authentication and secure token storage
package cloudsync

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OAuth2Service OAuth2 认证服务
// 用于管理云存储提供商的 OAuth2 认证流程和安全存储令牌.
type OAuth2Service struct {
	mu sync.RWMutex

	// 配置
	configPath string
	encryptKey []byte // AES 加密密钥

	// OAuth2 配置
	oauthConfigs map[ProviderType]*OAuth2Config

	// HTTP 客户端
	client *http.Client

	// 存储的令牌
	tokens map[string]*OAuth2Token // providerID -> token
}

// OAuth2Config OAuth2 配置.
type OAuth2Config struct {
	ProviderType   ProviderType `json:"providerType"`
	ClientID       string       `json:"clientId"`
	ClientSecret   string       `json:"-"` // 不序列化
	AuthURL        string       `json:"authUrl"`
	TokenURL       string       `json:"tokenUrl"`
	RedirectURL    string       `json:"redirectUrl"`
	Scopes         []string     `json:"scopes"`
	ExtraParams    map[string]string `json:"extraParams,omitempty"`
}

// OAuth2Token OAuth2 令牌.
type OAuth2Token struct {
	ProviderID    string    `json:"providerId"`
	ProviderType  ProviderType `json:"providerType"`
	AccessToken   string    `json:"-"` // 加密存储
	RefreshToken  string    `json:"-"` // 加密存储
	TokenType     string    `json:"tokenType"`
	ExpiresAt     time.Time `json:"expiresAt"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// OAuth2State OAuth2 状态（用于授权流程）.
type OAuth2State struct {
	State        string    `json:"state"`
	ProviderType ProviderType `json:"providerType"`
	ProviderID   string    `json:"providerId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// NewOAuth2Service 创建 OAuth2 认证服务.
func NewOAuth2Service(configPath string) *OAuth2Service {
	return &OAuth2Service{
		configPath:   configPath,
		encryptKey:   generateEncryptKey(),
		oauthConfigs: make(map[ProviderType]*OAuth2Config),
		tokens:       make(map[string]*OAuth2Token),
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

// Initialize 初始化 OAuth2 服务.
func (s *OAuth2Service) Initialize() error {
	// 加载已有令牌
	return s.loadTokens()
}

// loadTokens 加载令牌文件.
func (s *OAuth2Service) loadTokens() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return fmt.Errorf("读取令牌文件失败: %w", err)
	}

	// 解密并解析
	decrypted, err := s.decrypt(data)
	if err != nil {
		return fmt.Errorf("解密令牌失败: %w", err)
	}

	var tokens map[string]*OAuth2Token
	if err := json.Unmarshal(decrypted, &tokens); err != nil {
		return fmt.Errorf("解析令牌失败: %w", err)
	}

	s.mu.Lock()
	s.tokens = tokens
	s.mu.Unlock()

	return nil
}

// saveTokens 保存令牌文件.
func (s *OAuth2Service) saveTokens() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化令牌失败: %w", err)
	}

	// 加密存储
	encrypted, err := s.encrypt(data)
	if err != nil {
		return fmt.Errorf("加密令牌失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 安全存储（权限 0600）
	return os.WriteFile(s.configPath, encrypted, 0600)
}

// SetOAuthConfig 设置 OAuth2 配置.
func (s *OAuth2Service) SetOAuthConfig(providerType ProviderType, config *OAuth2Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.oauthConfigs[providerType] = config
}

// GetOAuthConfig 获取 OAuth2 配置.
func (s *OAuth2Service) GetOAuthConfig(providerType ProviderType) (*OAuth2Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, ok := s.oauthConfigs[providerType]
	if !ok {
		// 返回默认配置
		return getDefaultOAuthConfig(providerType)
	}

	return config, nil
}

// getDefaultOAuthConfig 返回默认 OAuth2 配置.
func getDefaultOAuthConfig(providerType ProviderType) (*OAuth2Config, error) {
	switch providerType {
	case ProviderGoogleDrive:
		return &OAuth2Config{
			ProviderType: ProviderGoogleDrive,
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			Scopes:       []string{"https://www.googleapis.com/auth/drive.file", "https://www.googleapis.com/auth/drive.metadata.readonly"},
		}, nil

	case ProviderOneDrive:
		return &OAuth2Config{
			ProviderType: ProviderOneDrive,
			AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			Scopes:       []string{"files.readwrite", "offline_access"},
		}, nil

	case ProviderDropbox:
		return &OAuth2Config{
			ProviderType: ProviderDropbox,
			AuthURL:      "https://www.dropbox.com/oauth2/authorize",
			TokenURL:     "https://api.dropboxapi.com/oauth2/token",
			Scopes:       []string{"files.content.write", "files.content.read", "files.metadata.write", "files.metadata.read"},
		}, nil

	default:
		return nil, fmt.Errorf("提供商不支持 OAuth2: %s", providerType)
	}
}

// GenerateAuthURL 生成授权 URL.
func (s *OAuth2Service) GenerateAuthURL(providerType ProviderType, providerID, redirectURL string) (string, string, error) {
	config, err := s.GetOAuthConfig(providerType)
	if err != nil {
		return "", "", err
	}

	// 更新重定向 URL
	if redirectURL != "" {
		config.RedirectURL = redirectURL
	}

	// 生成随机状态
	state := generateRandomState()

	// 构建授权 URL
	var urlBuilder strings.Builder
	urlBuilder.WriteString(config.AuthURL)
	urlBuilder.WriteString("?response_type=code")
	urlBuilder.WriteString("&client_id=" + config.ClientID)
	urlBuilder.WriteString("&redirect_uri=" + config.RedirectURL)
	urlBuilder.WriteString("&scope=" + strings.Join(config.Scopes, " "))
	urlBuilder.WriteString("&state=" + state)
	urlBuilder.WriteString("&access_type=offline") // 获取 refresh_token
	urlBuilder.WriteString("&prompt=consent")      // 强制同意以获取新的 refresh_token

	// 添加额外参数
	for key, value := range config.ExtraParams {
		urlBuilder.WriteString("&" + key + "=" + value)
	}

	return urlBuilder.String(), state, nil
}

// HandleAuthCallback 处理授权回调.
func (s *OAuth2Service) HandleAuthCallback(ctx context.Context, providerType ProviderType, providerID, code string) (*OAuth2Token, error) {
	config, err := s.GetOAuthConfig(providerType)
	if err != nil {
		return nil, err
	}

	// 使用授权码换取令牌
	token, err := s.exchangeCodeForToken(ctx, config, code)
	if err != nil {
		return nil, fmt.Errorf("获取令牌失败: %w", err)
	}

	token.ProviderID = providerID
	token.ProviderType = providerType
	token.CreatedAt = time.Now()
	token.UpdatedAt = time.Now()

	// 存储令牌
	s.mu.Lock()
	s.tokens[providerID] = token
	s.mu.Unlock()

	if err := s.saveTokens(); err != nil {
		return nil, fmt.Errorf("保存令牌失败: %w", err)
	}

	return token, nil
}

// exchangeCodeForToken 使用授权码换取令牌.
func (s *OAuth2Service) exchangeCodeForToken(ctx context.Context, config *OAuth2Config, code string) (*OAuth2Token, error) {
	data := fmt.Sprintf("grant_type=authorization_code&code=%s&client_id=%s&client_secret=%s&redirect_uri=%s",
		code, config.ClientID, config.ClientSecret, config.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("令牌交换失败: %s - %s", resp.Status, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &OAuth2Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// RefreshToken 刷新令牌.
func (s *OAuth2Service) RefreshToken(ctx context.Context, providerID string) (*OAuth2Token, error) {
	s.mu.RLock()
	token, ok := s.tokens[providerID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("令牌不存在: %s", providerID)
	}

	config, err := s.GetOAuthConfig(token.ProviderType)
	if err != nil {
		return nil, err
	}

	// 刷新令牌
	data := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s",
		token.RefreshToken, config.ClientID, config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("刷新令牌失败: %s - %s", resp.Status, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 更新令牌
	s.mu.Lock()
	token.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		token.RefreshToken = result.RefreshToken
	}
	token.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	token.UpdatedAt = time.Now()
	s.mu.Unlock()

	if err := s.saveTokens(); err != nil {
		return nil, fmt.Errorf("保存令牌失败: %w", err)
	}

	return token, nil
}

// GetAccessToken 获取访问令牌（自动刷新过期令牌）.
func (s *OAuth2Service) GetAccessToken(ctx context.Context, providerID string) (string, error) {
	s.mu.RLock()
	token, ok := s.tokens[providerID]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("令牌不存在: %s", providerID)
	}

	// 检查是否需要刷新
	if time.Now().After(token.ExpiresAt.Add(-5 * time.Minute)) {
		token, err := s.RefreshToken(ctx, providerID)
		if err != nil {
			return "", fmt.Errorf("刷新令牌失败: %w", err)
		}
		return token.AccessToken, nil
	}

	return token.AccessToken, nil
}

// GetToken 获取令牌信息.
func (s *OAuth2Service) GetToken(providerID string) (*OAuth2Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, ok := s.tokens[providerID]
	if !ok {
		return nil, fmt.Errorf("令牌不存在: %s", providerID)
	}

	return token, nil
}

// DeleteToken 删除令牌.
func (s *OAuth2Service) DeleteToken(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, providerID)

	return s.saveTokens()
}

// ListTokens 列出所有令牌.
func (s *OAuth2Service) ListTokens() []*OAuth2Token {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens := make([]*OAuth2Token, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, token)
	}
	return tokens
}

// ValidateToken 验证令牌有效性.
func (s *OAuth2Service) ValidateToken(ctx context.Context, providerID string) (bool, error) {
	token, err := s.GetToken(providerID)
	if err != nil {
		return false, err
	}

	// 检查过期时间
	if time.Now().After(token.ExpiresAt) {
		// 尝试刷新
		_, err := s.RefreshToken(ctx, providerID)
		if err != nil {
			return false, nil
		}
	}

	return true, nil
}

// ==================== 加密辅助函数 ====================

// generateEncryptKey 生成加密密钥.
func generateEncryptKey() []byte {
	// 从环境变量获取密钥，如果没有则生成固定密钥
	key := os.Getenv("OAUTH2_ENCRYPT_KEY")
	if key != "" {
		// 确保32字节（AES-256）
		if len(key) < 32 {
			key = key + strings.Repeat("x", 32-len(key))
		}
		return []byte(key[:32])
	}

	// 使用固定密钥（实际应用中应该从安全配置获取）
	return []byte("nas-os-oauth2-encryption-key-32b")
}

// encrypt 加密数据.
func (s *OAuth2Service) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptKey)
	if err != nil {
		return nil, err
	}

	// GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 生成 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// 加密
	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

// decrypt 解密数据.
func (s *OAuth2Service) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("数据太短")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

// generateRandomState 生成随机状态字符串.
func generateRandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}