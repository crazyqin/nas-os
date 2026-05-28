// Package nasgateway 提供 OAuth2.0 服务端功能
package nasgateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// OAuthServer OAuth2.0服务器.
type OAuthServer struct {
	mu              sync.RWMutex
	clients         map[string]*OAuthClient
	users           map[string]*OAuthUser
	authCodes       map[string]*AuthorizationCode
	accessTokens    map[string]*AccessToken
	refreshTokens   map[string]*RefreshToken
	authorizationTTL time.Duration
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
}

// NewOAuthServer 创建OAuth服务器.
func NewOAuthServer() *OAuthServer {
	return &OAuthServer{
		clients:          make(map[string]*OAuthClient),
		users:            make(map[string]*OAuthUser),
		authCodes:        make(map[string]*AuthorizationCode),
		accessTokens:     make(map[string]*AccessToken),
		refreshTokens:    make(map[string]*RefreshToken),
		authorizationTTL: 10 * time.Minute,
		accessTokenTTL:   time.Hour,
		refreshTokenTTL:  24 * time.Hour * 30,
	}
}

// ========== 客户端管理 ==========

// RegisterClient 注册客户端.
func (s *OAuthServer) RegisterClient(client *OAuthClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if client.ID == "" {
		return fmt.Errorf("客户端ID不能为空")
	}

	if _, exists := s.clients[client.ID]; exists {
		return fmt.Errorf("客户端已存在: %s", client.ID)
	}

	now := time.Now()
	client.CreatedAt = now
	client.UpdatedAt = now
	client.Enabled = true

	if client.AccessTokenTTL == 0 {
		client.AccessTokenTTL = s.accessTokenTTL
	}
	if client.RefreshTokenTTL == 0 {
		client.RefreshTokenTTL = s.refreshTokenTTL
	}

	s.clients[client.ID] = client
	return nil
}

// GetClient 获取客户端.
func (s *OAuthServer) GetClient(clientID string) (*OAuthClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, exists := s.clients[clientID]
	if !exists {
		return nil, ErrOAuthInvalidClient
	}
	return client, nil
}

// ListClients 列出客户端.
func (s *OAuthServer) ListClients() []*OAuthClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*OAuthClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	return clients
}

// UpdateClient 更新客户端.
func (s *OAuthServer) UpdateClient(client *OAuthClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[client.ID]; !exists {
		return ErrOAuthInvalidClient
	}

	client.UpdatedAt = time.Now()
	s.clients[client.ID] = client
	return nil
}

// DeleteClient 删除客户端.
func (s *OAuthServer) DeleteClient(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[clientID]; !exists {
		return ErrOAuthInvalidClient
	}

	delete(s.clients, clientID)
	return nil
}

// ========== 用户管理 ==========

// RegisterUser 注册用户.
func (s *OAuthServer) RegisterUser(user *OAuthUser) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user.ID == "" {
		return fmt.Errorf("用户ID不能为空")
	}

	if _, exists := s.users[user.ID]; exists {
		return fmt.Errorf("用户已存在: %s", user.ID)
	}

	// 密码哈希
	if user.Password != "" {
		user.Password = hashPassword(user.Password)
	}

	user.Enabled = true
	s.users[user.ID] = user
	return nil
}

// GetUser 获取用户.
func (s *OAuthServer) GetUser(userID string) (*OAuthUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}
	return user, nil
}

// ValidateUser 验证用户.
func (s *OAuthServer) ValidateUser(username, password string) (*OAuthUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.Username == username && user.Enabled {
			if user.Password == hashPassword(password) {
				return user, nil
			}
			return nil, fmt.Errorf("密码错误")
		}
	}

	return nil, fmt.Errorf("用户不存在")
}

// ========== 授权码流程 ==========

// CreateAuthorizationCode 创建授权码.
func (s *OAuthServer) CreateAuthorizationCode(clientID, userID, redirectURI string, scopes []string) (*AuthorizationCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证客户端
	client, exists := s.clients[clientID]
	if !exists || !client.Enabled {
		return nil, ErrOAuthInvalidClient
	}

	// 验证重定向URI
	if !isValidRedirectURI(redirectURI, client.RedirectURIs) {
		return nil, fmt.Errorf("无效的重定向URI")
	}

	// 生成授权码
	code := generateToken(32)
	authCode := &AuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scopes:      scopes,
		ExpiresAt:   time.Now().Add(s.authorizationTTL),
		Used:        false,
	}

	s.authCodes[code] = authCode
	return authCode, nil
}

// ValidateAuthorizationCode 验证授权码.
func (s *OAuthServer) ValidateAuthorizationCode(code, clientID, redirectURI string) (*AuthorizationCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	authCode, exists := s.authCodes[code]
	if !exists {
		return nil, ErrOAuthInvalidGrant
	}

	if authCode.Used {
		return nil, ErrOAuthInvalidGrant
	}

	if time.Now().After(authCode.ExpiresAt) {
		delete(s.authCodes, code)
		return nil, ErrOAuthInvalidGrant
	}

	if authCode.ClientID != clientID {
		return nil, ErrOAuthInvalidClient
	}

	if authCode.RedirectURI != redirectURI {
		return nil, fmt.Errorf("重定向URI不匹配")
	}

	authCode.Used = true
	return authCode, nil
}

// ========== 令牌管理 ==========

// IssueAccessToken 签发访问令牌.
func (s *OAuthServer) IssueAccessToken(clientID, userID string, scopes []string) (*TokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists || !client.Enabled {
		return nil, ErrOAuthInvalidClient
	}

	// 生成访问令牌
	token := generateToken(64)
	accessToken := &AccessToken{
		Token:     token,
		Type:      "Bearer",
		ClientID:  clientID,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: time.Now().Add(client.AccessTokenTTL),
		IssuedAt:  time.Now(),
	}

	s.accessTokens[token] = accessToken

	// 生成刷新令牌
	refreshToken := generateToken(64)
	refresh := &RefreshToken{
		Token:     refreshToken,
		ClientID:  clientID,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: time.Now().Add(client.RefreshTokenTTL),
		IssuedAt:  time.Now(),
	}

	s.refreshTokens[refreshToken] = refresh

	return &TokenResponse{
		AccessToken:  token,
		TokenType:    "Bearer",
		ExpiresIn:    int(client.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		Scope:        joinScopes(scopes),
	}, nil
}

// ValidateAccessToken 验证访问令牌.
func (s *OAuthServer) ValidateAccessToken(token string) (*AccessToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accessToken, exists := s.accessTokens[token]
	if !exists {
		return nil, ErrOAuthInvalidToken
	}

	if time.Now().After(accessToken.ExpiresAt) {
		delete(s.accessTokens, token)
		return nil, ErrOAuthInvalidToken
	}

	return accessToken, nil
}

// RefreshAccessToken 刷新访问令牌.
func (s *OAuthServer) RefreshAccessToken(refreshTokenStr, clientID string) (*TokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	refreshToken, exists := s.refreshTokens[refreshTokenStr]
	if !exists {
		return nil, ErrOAuthInvalidToken
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		delete(s.refreshTokens, refreshTokenStr)
		return nil, ErrOAuthInvalidToken
	}

	if refreshToken.ClientID != clientID {
		return nil, ErrOAuthInvalidClient
	}

	client, exists := s.clients[clientID]
	if !exists || !client.Enabled {
		return nil, ErrOAuthInvalidClient
	}

	// 删除旧的访问令牌
	for token, at := range s.accessTokens {
		if at.ClientID == clientID && at.UserID == refreshToken.UserID {
			delete(s.accessTokens, token)
		}
	}

	// 删除旧的刷新令牌
	delete(s.refreshTokens, refreshTokenStr)

	// 签发新的令牌
	return s.issueTokens(client, refreshToken.UserID, refreshToken.Scopes)
}

// RevokeToken 撤销令牌.
func (s *OAuthServer) RevokeToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.accessTokens, token)
	delete(s.refreshTokens, token)
}

// RevokeAllUserTokens 撤销用户所有令牌.
func (s *OAuthServer) RevokeAllUserTokens(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for token, at := range s.accessTokens {
		if at.UserID == userID {
			delete(s.accessTokens, token)
		}
	}

	for token, rt := range s.refreshTokens {
		if rt.UserID == userID {
			delete(s.refreshTokens, token)
		}
	}
}

// ========== 授权处理 ==========

// HandleAuthorizationRequest 处理授权请求.
func (s *OAuthServer) HandleAuthorizationRequest(req *AuthorizationRequest) (string, error) {
	// 验证客户端
	client, err := s.GetClient(req.ClientID)
	if err != nil {
		return "", err
	}

	if !client.Enabled {
		return "", ErrOAuthInvalidClient
	}

	// 验证响应类型
	if req.ResponseType != "code" {
		return "", fmt.Errorf("不支持的响应类型: %s", req.ResponseType)
	}

	// 验证重定向URI
	if !isValidRedirectURI(req.RedirectURI, client.RedirectURIs) {
		return "", fmt.Errorf("无效的重定向URI")
	}

	// 返回授权页面URL（实际应重定向到登录页面）
	return fmt.Sprintf("/oauth/authorize?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		req.ClientID, req.RedirectURI, req.Scope, req.State), nil
}

// HandleTokenRequest 处理令牌请求.
func (s *OAuthServer) HandleTokenRequest(req *TokenRequest) (*TokenResponse, error) {
	switch req.GrantType {
	case GrantTypeAuthorizationCode:
		return s.handleAuthorizationCodeGrant(req)
	case GrantTypeClientCredentials:
		return s.handleClientCredentialsGrant(req)
	case GrantTypePassword:
		return s.handlePasswordGrant(req)
	case GrantTypeRefreshToken:
		return s.handleRefreshTokenGrant(req)
	default:
		return nil, fmt.Errorf("不支持的授权类型: %s", req.GrantType)
	}
}

// handleAuthorizationCodeGrant 处理授权码模式.
func (s *OAuthServer) handleAuthorizationCodeGrant(req *TokenRequest) (*TokenResponse, error) {
	if req.Code == "" {
		return nil, fmt.Errorf("授权码不能为空")
	}

	// 验证授权码
	authCode, err := s.ValidateAuthorizationCode(req.Code, req.ClientID, req.RedirectURI)
	if err != nil {
		return nil, err
	}

	// 签发令牌
	return s.IssueAccessToken(req.ClientID, authCode.UserID, authCode.Scopes)
}

// handleClientCredentialsGrant 处理客户端凭证模式.
func (s *OAuthServer) handleClientCredentialsGrant(req *TokenRequest) (*TokenResponse, error) {
	// 验证客户端凭证
	client, err := s.GetClient(req.ClientID)
	if err != nil {
		return nil, err
	}

	if client.Secret != req.ClientSecret {
		return nil, ErrOAuthInvalidClient
	}

	// 检查是否支持此授权类型
	supported := false
	for _, gt := range client.GrantTypes {
		if gt == GrantTypeClientCredentials {
			supported = true
			break
		}
	}
	if !supported {
		return nil, fmt.Errorf("客户端不支持此授权类型")
	}

	// 签发令牌（无用户）
	return s.IssueAccessToken(req.ClientID, "", client.Scopes)
}

// handlePasswordGrant 处理密码模式.
func (s *OAuthServer) handlePasswordGrant(req *TokenRequest) (*TokenResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("用户名和密码不能为空")
	}

	// 验证客户端
	client, err := s.GetClient(req.ClientID)
	if err != nil {
		return nil, err
	}

	if client.Secret != req.ClientSecret {
		return nil, ErrOAuthInvalidClient
	}

	// 检查是否支持此授权类型
	supported := false
	for _, gt := range client.GrantTypes {
		if gt == GrantTypePassword {
			supported = true
			break
		}
	}
	if !supported {
		return nil, fmt.Errorf("客户端不支持此授权类型")
	}

	// 验证用户
	user, err := s.ValidateUser(req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	// 签发令牌
	scopes := parseScopes(req.Scope)
	return s.IssueAccessToken(req.ClientID, user.ID, scopes)
}

// handleRefreshTokenGrant 处理刷新令牌模式.
func (s *OAuthServer) handleRefreshTokenGrant(req *TokenRequest) (*TokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, fmt.Errorf("刷新令牌不能为空")
	}

	return s.RefreshAccessToken(req.RefreshToken, req.ClientID)
}

// ========== 内部方法 ==========

// issueTokens 签发令牌.
func (s *OAuthServer) issueTokens(client *OAuthClient, userID string, scopes []string) (*TokenResponse, error) {
	accessToken := generateToken(64)
	at := &AccessToken{
		Token:     accessToken,
		Type:      "Bearer",
		ClientID:  client.ID,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: time.Now().Add(client.AccessTokenTTL),
		IssuedAt:  time.Now(),
	}
	s.accessTokens[accessToken] = at

	refreshToken := generateToken(64)
	rt := &RefreshToken{
		Token:     refreshToken,
		ClientID:  client.ID,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: time.Now().Add(client.RefreshTokenTTL),
		IssuedAt:  time.Now(),
	}
	s.refreshTokens[refreshToken] = rt

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(client.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		Scope:        joinScopes(scopes),
	}, nil
}

// ========== 辅助函数 ==========

// generateToken 生成令牌.
func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// hashPassword 密码哈希.
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// isValidRedirectURI 验证重定向URI.
func isValidRedirectURI(uri string, allowed []string) bool {
	for _, a := range allowed {
		if a == uri {
			return true
		}
	}
	return false
}

// parseScopes 解析scope.
func parseScopes(scope string) []string {
	if scope == "" {
		return nil
	}
	scopes := make([]string, 0)
	for _, s := range splitScopes(scope) {
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}

// splitScopes 分割scope.
func splitScopes(scope string) []string {
	result := make([]string, 0)
	current := ""
	for _, c := range scope {
		if c == ' ' || c == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// joinScopes 合并scope.
func joinScopes(scopes []string) string {
	result := ""
	for i, s := range scopes {
		if i > 0 {
			result += " "
		}
		result += s
	}
	return result
}

// ========== 清理过期令牌 ==========

// CleanupExpired 清理过期令牌和授权码.
func (s *OAuthServer) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 清理授权码
	for code, ac := range s.authCodes {
		if now.After(ac.ExpiresAt) || ac.Used {
			delete(s.authCodes, code)
		}
	}

	// 清理访问令牌
	for token, at := range s.accessTokens {
		if now.After(at.ExpiresAt) {
			delete(s.accessTokens, token)
		}
	}

	// 清理刷新令牌
	for token, rt := range s.refreshTokens {
		if now.After(rt.ExpiresAt) {
			delete(s.refreshTokens, token)
		}
	}
}

// GetStats 获取OAuth统计.
func (s *OAuthServer) GetStats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]int{
		"clients":       len(s.clients),
		"users":         len(s.users),
		"auth_codes":    len(s.authCodes),
		"access_tokens": len(s.accessTokens),
		"refresh_tokens": len(s.refreshTokens),
	}
}
