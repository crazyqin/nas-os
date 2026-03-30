package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// WebAuthnManager WebAuthn 管理器（简化版本）
// 注意：完整的 WebAuthn 实现需要前端配合和 HTTPS 环境.
type WebAuthnManager struct {
	mu          sync.RWMutex
	credentials map[string][]*WebAuthnCredential // userID -> credentials
	sessions    map[string]*WebAuthnSession      // sessionID -> SessionData
	rpID        string
	rpName      string
}

// WebAuthnSession WebAuthn 会话.
type WebAuthnSession struct {
	UserID     string
	Username   string
	Challenge  string
	ExpiresAt  time.Time
	IsRegister bool
}

// WebAuthnConfig WebAuthn 配置.
type WebAuthnConfig struct {
	RPDisplayName string
	RPID          string
	RPOrigins     []string
}

// NewWebAuthnManager 创建 WebAuthn 管理器.
func NewWebAuthnManager(cfg WebAuthnConfig) (*WebAuthnManager, error) {
	return &WebAuthnManager{
		credentials: make(map[string][]*WebAuthnCredential),
		sessions:    make(map[string]*WebAuthnSession),
		rpID:        cfg.RPID,
		rpName:      cfg.RPDisplayName,
	}, nil
}

// generateChallenge 生成挑战.
func generateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateSessionID 生成会话 ID.
func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// BeginRegistration 开始注册流程.
func (m *WebAuthnManager) BeginRegistration(userID, username, displayName string) (string, interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	challenge, err := generateChallenge()
	if err != nil {
		return "", nil, err
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return "", nil, err
	}

	// 存储会话
	m.sessions[sessionID] = &WebAuthnSession{
		UserID:     userID,
		Username:   username,
		Challenge:  challenge,
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		IsRegister: true,
	}

	// 返回注册选项（简化版本，实际应返回完整的 WebAuthn PublicKeyCredentialCreationOptions）
	options := map[string]interface{}{
		"challenge": challenge,
		"rp": map[string]string{
			"name": m.rpName,
			"id":   m.rpID,
		},
		"user": map[string]string{
			"id":          userID,
			"name":        username,
			"displayName": displayName,
		},
		"pubKeyCredParams": []map[string]interface{}{
			{"type": "public-key", "alg": -7},   // ES256
			{"type": "public-key", "alg": -257}, // RS256
		},
		"timeout":     60000,
		"attestation": "none",
	}

	return sessionID, options, nil
}

// FinishRegistration 完成注册流程.
func (m *WebAuthnManager) FinishRegistration(sessionID string, responseData interface{}) (*WebAuthnCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}

	if time.Now().After(session.ExpiresAt) {
		delete(m.sessions, sessionID)
		return nil, fmt.Errorf("会话已过期")
	}

	// 简化验证：实际应验证 response 中的 attestationObject 和 clientDataJSON
	// 这里假设前端已正确完成 WebAuthn 流程
	responseMap, ok := responseData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	// 生成凭据 ID
	credID, ok := responseMap["id"].(string)
	if !ok {
		credID = sessionID
	}

	// 存储凭据（简化版本）
	credential := &WebAuthnCredential{
		ID:              credID,
		PublicKey:       []byte("public-key-placeholder"),
		AttestationType: "none",
		Transport:       []string{"internal"},
		CreatedAt:       time.Now(),
	}

	m.credentials[session.UserID] = append(m.credentials[session.UserID], credential)

	// 清理会话
	delete(m.sessions, sessionID)

	return credential, nil
}

// BeginAuthentication 开始认证流程.
func (m *WebAuthnManager) BeginAuthentication(userID string) (string, interface{}, error) {
	m.mu.RLock()
	creds := m.credentials[userID]
	m.mu.RUnlock()

	if len(creds) == 0 {
		return "", nil, fmt.Errorf("用户没有注册的安全密钥")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	challenge, err := generateChallenge()
	if err != nil {
		return "", nil, err
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return "", nil, err
	}

	// 存储会话
	m.sessions[sessionID] = &WebAuthnSession{
		UserID:     userID,
		Challenge:  challenge,
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		IsRegister: false,
	}

	// 返回认证选项（简化版本）
	allowCredentials := make([]map[string]interface{}, len(creds))
	for i, cred := range creds {
		allowCredentials[i] = map[string]interface{}{
			"type": "public-key",
			"id":   cred.ID,
		}
	}

	options := map[string]interface{}{
		"challenge":        challenge,
		"timeout":          60000,
		"rpId":             m.rpID,
		"allowCredentials": allowCredentials,
	}

	return sessionID, options, nil
}

// FinishAuthentication 完成认证流程.
func (m *WebAuthnManager) FinishAuthentication(sessionID string, responseData interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return "", fmt.Errorf("会话不存在")
	}

	if time.Now().After(session.ExpiresAt) {
		delete(m.sessions, sessionID)
		return "", fmt.Errorf("会话已过期")
	}

	// 简化验证：实际应验证 signature 和 authenticatorData
	// 这里假设前端已正确完成 WebAuthn 流程

	// 更新凭据的最后使用时间
	now := time.Now()
	for _, cred := range m.credentials[session.UserID] {
		cred.LastUsedAt = &now
	}

	// 清理会话
	delete(m.sessions, sessionID)

	return session.UserID, nil
}

// GetCredentials 获取用户的凭据列表.
func (m *WebAuthnManager) GetCredentials(userID string) []*WebAuthnCredential {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.credentials[userID]
}

// RemoveCredential 移除凭据.
func (m *WebAuthnManager) RemoveCredential(userID, credentialID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	creds := m.credentials[userID]
	for i, cred := range creds {
		if cred.ID == credentialID {
			m.credentials[userID] = append(creds[:i], creds[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("凭据不存在")
}
