// Package auth SSO Server协议扩展
// 支持OAuth2/OIDC/SAML协议，第三方应用SSO集成
// 对标群晖 SSO Server
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 常量 ==========

const (
	// TokenExpiry Token有效期
	TokenExpiry = 1 * time.Hour
	// RefreshTokenExpiry Refresh Token有效期
	RefreshTokenExpiry = 30 * 24 * time.Hour
	// CodeExpiry 授权码有效期
	CodeExpiry = 10 * time.Minute
	// MaxClients 最大客户端数
	MaxClients = 100
)

// ========== OAuth2 类型 ==========

// GrantType 授权类型
type GrantType string

const (
	GrantAuthorizationCode GrantType = "authorization_code"
	GrantClientCredentials GrantType = "client_credentials"
	GrantRefreshToken      GrantType = "refresh_token"
)

// ResponseType 响应类型
type ResponseType string

const (
	ResponseTypeCode  ResponseType = "code"
	ResponseTypeToken ResponseType = "token"
)

// OAuthClient OAuth2客户端
type OAuthClient struct {
	ID           string   `json:"id"`
	Secret       string   `json:"secret"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	RedirectURIs []string `json:"redirect_uris"`
	Scopes       []string `json:"scopes"`
	GrantTypes   []GrantType `json:"grant_types"`
	Enabled      bool     `json:"enabled"`
	IsInternal   bool     `json:"is_internal"` // 内部应用
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AuthorizationCode 授权码
type AuthorizationCode struct {
	Code        string    `json:"code"`
	ClientID    string    `json:"client_id"`
	UserID      string    `json:"user_id"`
	RedirectURI string    `json:"redirect_uri"`
	Scopes      []string  `json:"scopes"`
	ExpiresAt   time.Time `json:"expires_at"`
	Used        bool      `json:"used"`
}

// TokenPair Token对
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	Scope        string    `json:"scope"`
	IssuedAt     time.Time `json:"issued_at"`
}

// AccessTokenClaims Access Token声明
type AccessTokenClaims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	ClientID  string   `json:"client_id"`
	Scopes    []string `json:"scope"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	TokenType string   `json:"token_type"`
}

// RefreshToken Refresh Token
type RefreshToken struct {
	Token     string    `json:"token"`
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

// OIDCDiscovery OIDC发现文档
type OIDCDiscovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	JwksURI               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	GrantTypesSupported   []string `json:"grant_types_supported"`
	SubjectTypesSupported []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported       []string `json:"claims_supported"`
}

// IDToken ID Token (OIDC)
type IDToken struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	Nonce     string `json:"nonce,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Picture   string `json:"picture,omitempty"`
}

// SAMLRequest SAML认证请求
type SAMLRequest struct {
	ID           string    `json:"id"`
	Issuer       string    `json:"issuer"`
	NameIDPolicy string    `json:"name_id_policy"`
	Destination  string    `json:"destination"`
	AssertionURL string    `json:"assertion_url"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SAMLResponse SAML认证响应
type SAMLResponse struct {
	ID            string    `json:"id"`
	InResponseTo  string    `json:"in_response_to"`
	Issuer        string    `json:"issuer"`
	Status        string    `json:"status"`
	NameID        string    `json:"name_id"`
	SessionIndex  string    `json:"session_index"`
	Attributes    map[string][]string `json:"attributes"`
	AssertionXML  string    `json:"assertion_xml"`
	SignedAt      time.Time `json:"signed_at"`
}

// SSOUser SSO用户信息 (OIDC UserInfo)
type SSOUser struct {
	Sub       string `json:"sub"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Picture   string `json:"picture,omitempty"`
	Groups    []string `json:"groups,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

// SSOServer SSO服务器
type SSOServer struct {
	mu             sync.RWMutex
	clients        map[string]*OAuthClient
	codes          map[string]*AuthorizationCode
	tokens         map[string]*AccessTokenClaims
	refreshTokens  map[string]*RefreshToken
	users          map[string]*SSOUser
	privateKey     *rsa.PrivateKey
	publicKey      *rsa.PublicKey
	baseURL        string
	issuer         string
}

// NewSSOServer 创建SSO服务器
func NewSSOServer(baseURL string) (*SSOServer, error) {
	// 生成RSA密钥对（用于签名Token）
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成RSA密钥失败: %w", err)
	}

	server := &SSOServer{
		clients:       make(map[string]*OAuthClient),
		codes:         make(map[string]*AuthorizationCode),
		tokens:        make(map[string]*AccessTokenClaims),
		refreshTokens: make(map[string]*RefreshToken),
		users:         make(map[string]*SSOUser),
		privateKey:    privateKey,
		publicKey:     &privateKey.PublicKey,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		issuer:        strings.TrimSuffix(baseURL, "/"),
	}

	return server, nil
}

// RegisterClient 注册OAuth2客户端
func (s *SSOServer) RegisterClient(client *OAuthClient) error {
	if client.Name == "" {
		return fmt.Errorf("客户端名不能为空")
	}
	if len(client.RedirectURIs) == 0 {
		return fmt.Errorf("重定向URI不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.clients) >= MaxClients {
		return fmt.Errorf("已达到最大客户端数 (%d)", MaxClients)
	}

	if client.ID == "" {
		client.ID = uuid.New().String()
	}
	if client.Secret == "" {
		client.Secret = generateRandomString(32)
	}
	if len(client.GrantTypes) == 0 {
		client.GrantTypes = []GrantType{GrantAuthorizationCode, GrantRefreshToken}
	}
	if len(client.Scopes) == 0 {
		client.Scopes = []string{"openid", "profile", "email"}
	}
	client.Enabled = true
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()

	s.clients[client.ID] = client
	return nil
}

// GetClient 获取客户端
func (s *SSOServer) GetClient(clientID string) (*OAuthClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	client, ok := s.clients[clientID]
	if !ok {
		return nil, fmt.Errorf("客户端不存在")
	}
	return client, nil
}

// ListClients 列出所有客户端
func (s *SSOServer) ListClients() []*OAuthClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*OAuthClient, 0, len(s.clients))
	for _, c := range s.clients {
		result = append(result, c)
	}
	return result
}

// DeleteClient 删除客户端
func (s *SSOServer) DeleteClient(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[clientID]; !ok {
		return fmt.Errorf("客户端不存在")
	}
	delete(s.clients, clientID)
	return nil
}

// Authorize 生成授权码 (OAuth2 Authorization Endpoint)
func (s *SSOServer) Authorize(clientID, redirectURI, scope, state, userID string) (string, error) {
	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("无效的client_id")
	}
	if !client.Enabled {
		return "", fmt.Errorf("客户端已禁用")
	}

	// 验证redirect_uri
	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		return "", fmt.Errorf("无效的redirect_uri")
	}

	// 生成授权码
	code := &AuthorizationCode{
		Code:        generateRandomString(32),
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scopes:      strings.Split(scope, " "),
		ExpiresAt:   time.Now().Add(CodeExpiry),
		Used:        false,
	}

	s.mu.Lock()
	s.codes[code.Code] = code
	s.mu.Unlock()

	return code.Code, nil
}

// ExchangeToken 交换Token (OAuth2 Token Endpoint)
func (s *SSOServer) ExchangeToken(grantType GrantType, code, clientID, clientSecret, refreshToken, scope string) (*TokenPair, error) {
	switch grantType {
	case GrantAuthorizationCode:
		return s.exchangeAuthorizationCode(code, clientID, clientSecret)
	case GrantRefreshToken:
		return s.exchangeRefreshToken(refreshToken, clientID, clientSecret)
	case GrantClientCredentials:
		return s.exchangeClientCredentials(clientID, clientSecret, scope)
	default:
		return nil, fmt.Errorf("不支持的grant_type: %s", grantType)
	}
}

func (s *SSOServer) exchangeAuthorizationCode(code, clientID, clientSecret string) (*TokenPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	authCode, ok := s.codes[code]
	if !ok {
		return nil, fmt.Errorf("无效的授权码")
	}
	if authCode.Used {
		return nil, fmt.Errorf("授权码已使用")
	}
	if authCode.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("授权码已过期")
	}
	if authCode.ClientID != clientID {
		return nil, fmt.Errorf("client_id不匹配")
	}

	// 验证client_secret
	client, ok := s.clients[clientID]
	if !ok || client.Secret != clientSecret {
		return nil, fmt.Errorf("客户端认证失败")
	}

	// 标记授权码已使用
	authCode.Used = true

	// 生成Token对
	return s.generateTokenPair(clientID, authCode.UserID, authCode.Scopes)
}

func (s *SSOServer) exchangeRefreshToken(refreshToken, clientID, clientSecret string) (*TokenPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rt, ok := s.refreshTokens[refreshToken]
	if !ok {
		return nil, fmt.Errorf("无效的refresh_token")
	}
	if rt.Revoked {
		return nil, fmt.Errorf("refresh_token已撤销")
	}
	if rt.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("refresh_token已过期")
	}
	if rt.ClientID != clientID {
		return nil, fmt.Errorf("client_id不匹配")
	}

	// 验证client_secret
	client, ok := s.clients[clientID]
	if !ok || client.Secret != clientSecret {
		return nil, fmt.Errorf("客户端认证失败")
	}

	// 撤销旧的refresh_token
	rt.Revoked = true

	// 生成新Token对
	return s.generateTokenPair(clientID, rt.UserID, rt.Scopes)
}

func (s *SSOServer) exchangeClientCredentials(clientID, clientSecret, scope string) (*TokenPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[clientID]
	if !ok || client.Secret != clientSecret {
		return nil, fmt.Errorf("客户端认证失败")
	}

	// 检查是否支持client_credentials
	supported := false
	for _, gt := range client.GrantTypes {
		if gt == GrantClientCredentials {
			supported = true
			break
		}
	}
	if !supported {
		return nil, fmt.Errorf("客户端不支持client_credentials")
	}

	scopes := strings.Split(scope, " ")
	return s.generateTokenPair(clientID, "", scopes)
}

func (s *SSOServer) generateTokenPair(clientID, userID string, scopes []string) (*TokenPair, error) {
	now := time.Now()

	// 生成access_token
	accessToken := generateRandomString(64)
	claims := &AccessTokenClaims{
		Issuer:    s.issuer,
		Subject:   userID,
		ClientID:  clientID,
		Scopes:    scopes,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(TokenExpiry).Unix(),
		TokenType: "Bearer",
	}
	s.tokens[accessToken] = claims

	// 生成refresh_token
	refreshToken := generateRandomString(64)
	s.refreshTokens[refreshToken] = &RefreshToken{
		Token:     refreshToken,
		ClientID:  clientID,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: now.Add(RefreshTokenExpiry),
		Revoked:   false,
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(TokenExpiry.Seconds()),
		Scope:        strings.Join(scopes, " "),
		IssuedAt:     now,
	}, nil
}

// ValidateToken 验证Access Token
func (s *SSOServer) ValidateToken(accessToken string) (*AccessTokenClaims, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	claims, ok := s.tokens[accessToken]
	if !ok {
		return nil, fmt.Errorf("无效的access_token")
	}
	if claims.ExpiresAt < time.Now().Unix() {
		return nil, fmt.Errorf("access_token已过期")
	}
	return claims, nil
}

// RevokeToken 撤销Token
func (s *SSOServer) RevokeToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 尝试作为access_token撤销
	if claims, ok := s.tokens[token]; ok {
		claims.ExpiresAt = 0
		return nil
	}

	// 尝试作为refresh_token撤销
	if rt, ok := s.refreshTokens[token]; ok {
		rt.Revoked = true
		return nil
	}

	return fmt.Errorf("token不存在")
}

// GetUserInfo 获取用户信息 (OIDC UserInfo Endpoint)
func (s *SSOServer) GetUserInfo(accessToken string) (*SSOUser, error) {
	claims, err := s.ValidateToken(accessToken)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	user, ok := s.users[claims.Subject]
	s.mu.RUnlock()

	if !ok {
		// 返回基本用户信息
		return &SSOUser{
			Sub:  claims.Subject,
			Name: claims.Subject,
		}, nil
	}

	return user, nil
}

// GetOIDCDiscovery 获取OIDC发现文档
func (s *SSOServer) GetOIDCDiscovery() *OIDCDiscovery {
	return &OIDCDiscovery{
		Issuer:                s.issuer,
		AuthorizationEndpoint: s.baseURL + "/oauth/authorize",
		TokenEndpoint:         s.baseURL + "/oauth/token",
		UserinfoEndpoint:      s.baseURL + "/oauth/userinfo",
		JwksURI:               s.baseURL + "/.well-known/jwks.json",
		ScopesSupported:       []string{"openid", "profile", "email", "groups"},
		ResponseTypesSupported: []string{"code", "token"},
		GrantTypesSupported:   []string{"authorization_code", "client_credentials", "refresh_token"},
		SubjectTypesSupported: []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},
		ClaimsSupported:       []string{"sub", "name", "email", "iss", "aud", "exp", "iat", "nonce"},
	}
}

// GetJWKS 获取JWKS
func (s *SSOServer) GetJWKS() map[string]interface{} {
	// 简化实现 - 返回公钥信息
	n := base64.RawURLEncoding.EncodeToString(s.publicKey.N.Bytes())
	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": "nas-os-key-1",
				"n":   n,
				"e":   "AQAB",
			},
		},
	}
}

// GetStats 获取SSO统计信息
func (s *SSOServer) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeTokens := 0
	for _, c := range s.tokens {
		if c.ExpiresAt > time.Now().Unix() {
			activeTokens++
		}
	}

	return map[string]interface{}{
		"clients":       len(s.clients),
		"active_tokens": activeTokens,
		"users":         len(s.users),
	}
}

// ========== HTTP Handlers ==========

// SSOHandlers SSO HTTP处理器
type SSOHandlers struct {
	server *SSOServer
}

func NewSSOHandlers(server *SSOServer) *SSOServer {
	return server
}

func (h *SSOServer) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/authorize", h.handleAuthorize)
	mux.HandleFunc(prefix+"/token", h.handleToken)
	mux.HandleFunc(prefix+"/userinfo", h.handleUserInfo)
	mux.HandleFunc(prefix+"/revoke", h.handleRevoke)
	mux.HandleFunc(prefix+"/clients", h.handleClients)
	mux.HandleFunc(prefix+"/clients/", h.handleClientByID)
	mux.HandleFunc(prefix+"/.well-known/openid-configuration", h.handleOIDCDiscovery)
	mux.HandleFunc(prefix+"/.well-known/jwks.json", h.handleJWKS)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
}

func (h *SSOServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	scope := r.URL.Query().Get("scope")
	state := r.URL.Query().Get("state")
	userID := r.URL.Query().Get("user_id") // 简化：实际应通过登录页面获取

	code, err := h.Authorize(clientID, redirectURI, scope, state, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 重定向回客户端
	redirectURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, code, state)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *SSOServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	grantType := GrantType(r.FormValue("grant_type"))
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	refreshToken := r.FormValue("refresh_token")
	scope := r.FormValue("scope")

	tokenPair, err := h.ExchangeToken(grantType, code, clientID, clientSecret, refreshToken, scope)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenPair)
}

func (h *SSOServer) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	accessToken := r.Header.Get("Authorization")
	accessToken = strings.TrimPrefix(accessToken, "Bearer ")

	user, err := h.GetUserInfo(accessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *SSOServer) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.FormValue("token")
	if err := h.RevokeToken(token); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
}

func (h *SSOServer) handleClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		clients := h.ListClients()
		json.NewEncoder(w).Encode(map[string]interface{}{"clients": clients})
	case http.MethodPost:
		var client OAuthClient
		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.RegisterClient(&client); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(client)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SSOServer) handleClientByID(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimPrefix(r.URL.Path, "/api/v1/sso/clients/")
	if clientID == "" {
		http.Error(w, "Missing client ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		client, err := h.GetClient(clientID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(client)
	case http.MethodDelete:
		if err := h.DeleteClient(clientID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SSOServer) handleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	discovery := h.GetOIDCDiscovery()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(discovery)
}

func (h *SSOServer) handleJWKS(w http.ResponseWriter, r *http.Request) {
	jwks := h.GetJWKS()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

func (h *SSOServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.GetStats())
}

// ========== 辅助函数 ==========

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:length]
}

// hashToken 对Token进行SHA-256哈希（用于日志记录）
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:8])
}
