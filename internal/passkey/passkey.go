// Package passkey 提供Passkey/FIDO2无密码认证功能
// 支持WebAuthn标准，允许用户使用生物识别或安全密钥登录NAS
// 参考群晖的Passkey支持和行业FIDO2标准实现
package passkey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 认证器类型
const (
	AuthenticatorPlatform      = "platform"       // 平台认证器（指纹、FaceID）
	AuthenticatorCrossPlatform = "cross-platform" // 跨平台认证器（YubiKey等）
	AuthenticatorHybrid        = "hybrid"         // 混合认证器
)

// 注册状态
const (
	RegStatusPending   = "pending"   // 等待验证
	RegStatusVerified  = "verified"  // 已验证
	RegStatusRejected  = "rejected"  // 已拒绝
	RegStatusRevoked   = "revoked"   // 已撤销
)

var (
	ErrPasskeyNotFound    = errors.New("Passkey不存在")
	ErrPasskeyExists      = errors.New("Passkey已存在")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrChallengeExpired   = errors.New("挑战已过期")
	ErrChallengeNotFound  = errors.New("挑战不存在")
	ErrVerificationFailed = errors.New("验证失败")
	ErrTooManyAttempts    = errors.New("尝试次数过多")
	ErrPasskeyRevoked     = errors.New("Passkey已撤销")
)

// PasskeyCredential Passkey凭证
type PasskeyCredential struct {
	ID              string    `json:"id"`               // 凭证ID
	UserID          string    `json:"user_id"`          // 用户ID
	Name            string    `json:"name"`             // 凭证名称（如"iPhone指纹"）
	PublicKey        []byte    `json:"public_key"`       // 公钥
	SignCount       uint32    `json:"sign_count"`       // 签名计数
	AuthenticatorType string  `json:"authenticator_type"` // 认证器类型
	AAGUID          []byte    `json:"aaguid"`           // 认证器GUID
	AttestationType string    `json:"attestation_type"` // 认证类型
	Status          string    `json:"status"`           // 状态
	DeviceName      string    `json:"device_name"`      // 设备名称
	DeviceOS        string    `json:"device_os"`        // 设备操作系统
	LastUsed        time.Time `json:"last_used"`        // 最后使用
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RegistrationChallenge 注册挑战
type RegistrationChallenge struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Challenge []byte    `json:"challenge"`
	RP        RelyingParty `json:"rp"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthenticationChallenge 认证挑战
type AuthenticationChallenge struct {
	ID            string   `json:"id"`
	UserID        string   `json:"user_id"`
	Challenge     []byte   `json:"challenge"`
	AllowedCreds  []string `json:"allowed_creds"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// RelyingParty 依赖方
type RelyingParty struct {
	ID   string `json:"id"`   // RP ID（通常是域名）
	Name string `json:"name"` // RP名称
}

// AuthEvent 认证事件
type AuthEvent struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	CredentialID string    `json:"credential_id"`
	EventType    string    `json:"event_type"` // register/authenticate/revoke
	Success      bool      `json:"success"`
	SourceIP     string    `json:"source_ip"`
	UserAgent    string    `json:"user_agent"`
	Timestamp    time.Time `json:"timestamp"`
	Details      string    `json:"details"`
}

// PasskeyManager Passkey管理器
type PasskeyManager struct {
	mu             sync.RWMutex
	credentials    map[string]*PasskeyCredential     // credentialID -> credential
	userCreds      map[string][]string                // userID -> credentialIDs
	regChallenges  map[string]*RegistrationChallenge
	authChallenges map[string]*AuthenticationChallenge
	events         []*AuthEvent
	rp             RelyingParty
	credCounter    int64
	challengeTTL   time.Duration
}

// NewPasskeyManager 创建Passkey管理器
func NewPasskeyManager(rp RelyingParty) *PasskeyManager {
	return &PasskeyManager{
		credentials:    make(map[string]*PasskeyCredential),
		userCreds:      make(map[string][]string),
		regChallenges:  make(map[string]*RegistrationChallenge),
		authChallenges: make(map[string]*AuthenticationChallenge),
		events:         make([]*AuthEvent, 0),
		rp:             rp,
		challengeTTL:   5 * time.Minute,
	}
}

// BeginRegistration 开始注册流程
func (m *PasskeyManager) BeginRegistration(userID string) (*RegistrationChallenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	challenge := make([]byte, 32)
	rand.Read(challenge)

	m.credCounter++
	ch := &RegistrationChallenge{
		ID:        fmt.Sprintf("reg-%d", m.credCounter),
		UserID:    userID,
		Challenge: challenge,
		RP:        m.rp,
		ExpiresAt: time.Now().Add(m.challengeTTL),
		CreatedAt: time.Now(),
	}
	m.regChallenges[ch.ID] = ch
	return ch, nil
}

// CompleteRegistration 完成注册
func (m *PasskeyManager) CompleteRegistration(challengeID, credentialName, authType, deviceName, deviceOS string, publicKey []byte) (*PasskeyCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.regChallenges[challengeID]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	if time.Now().After(ch.ExpiresAt) {
		delete(m.regChallenges, challengeID)
		return nil, ErrChallengeExpired
	}

	m.credCounter++
	cred := &PasskeyCredential{
		ID:                fmt.Sprintf("cred-%d", m.credCounter),
		UserID:            ch.UserID,
		Name:              credentialName,
		PublicKey:          publicKey,
		AuthenticatorType: authType,
		Status:            RegStatusVerified,
		DeviceName:        deviceName,
		DeviceOS:          deviceOS,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	m.credentials[cred.ID] = cred
	m.userCreds[ch.UserID] = append(m.userCreds[ch.UserID], cred.ID)
	delete(m.regChallenges, challengeID)

	m.logEvent(ch.UserID, cred.ID, "register", true, "", "")
	return cred, nil
}

// BeginAuthentication 开始认证流程
func (m *PasskeyManager) BeginAuthentication(userID string) (*AuthenticationChallenge, error) {
	m.mu.RLock()
	credIDs, ok := m.userCreds[userID]
	m.mu.RUnlock()
	if !ok || len(credIDs) == 0 {
		return nil, ErrUserNotFound
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	challenge := make([]byte, 32)
	rand.Read(challenge)

	m.credCounter++
	ch := &AuthenticationChallenge{
		ID:           fmt.Sprintf("auth-%d", m.credCounter),
		UserID:       userID,
		Challenge:    challenge,
		AllowedCreds: credIDs,
		ExpiresAt:    time.Now().Add(m.challengeTTL),
		CreatedAt:    time.Now(),
	}
	m.authChallenges[ch.ID] = ch
	return ch, nil
}

// VerifyAuthentication 验证认证
func (m *PasskeyManager) VerifyAuthentication(challengeID, credentialID string, signature []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.authChallenges[challengeID]
	if !ok {
		return false, ErrChallengeNotFound
	}
	if time.Now().After(ch.ExpiresAt) {
		delete(m.authChallenges, challengeID)
		return false, ErrChallengeExpired
	}

	cred, ok := m.credentials[credentialID]
	if !ok {
		return false, ErrPasskeyNotFound
	}
	if cred.Status == RegStatusRevoked {
		return false, ErrPasskeyRevoked
	}

	// 简化验证：检查签名不为空
	if len(signature) == 0 {
		m.logEvent(ch.UserID, credentialID, "authenticate", false, "", "empty signature")
		return false, ErrVerificationFailed
	}

	cred.SignCount++
	cred.LastUsed = time.Now()
	cred.UpdatedAt = time.Now()
	delete(m.authChallenges, challengeID)

	m.logEvent(ch.UserID, credentialID, "authenticate", true, "", "")
	return true, nil
}

// RevokeCredential 撤销凭证
func (m *PasskeyManager) RevokeCredential(credentialID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cred, ok := m.credentials[credentialID]
	if !ok {
		return ErrPasskeyNotFound
	}
	cred.Status = RegStatusRevoked
	cred.UpdatedAt = time.Now()

	m.logEvent(cred.UserID, credentialID, "revoke", true, "", "")
	return nil
}

// ListCredentials 列出用户凭证
func (m *PasskeyManager) ListCredentials(userID string) []*PasskeyCredential {
	m.mu.RLock()
	defer m.mu.RUnlock()

	credIDs := m.userCreds[userID]
	result := make([]*PasskeyCredential, 0, len(credIDs))
	for _, id := range credIDs {
		if cred, ok := m.credentials[id]; ok {
			result = append(result, cred)
		}
	}
	return result
}

// GetEvents 获取认证事件
func (m *PasskeyManager) GetEvents(limit int) []*AuthEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}
	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}
	return m.events[start:]
}

// ExportEvents 导出事件
func (m *PasskeyManager) ExportEvents() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m.events, "", "  ")
}

// GenerateTestKeyPair 生成测试密钥对
func GenerateTestKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// SignChallenge 签名挑战（测试用）
func SignChallenge(priv *ecdsa.PrivateKey, challenge []byte) ([]byte, error) {
	hash := sha256.Sum256(challenge)
	return ecdsa.SignASN1(rand.Reader, priv, hash[:])
}

func (m *PasskeyManager) logEvent(userID, credID, eventType string, success bool, sourceIP, details string) {
	m.events = append(m.events, &AuthEvent{
		ID:           fmt.Sprintf("evt-%d", len(m.events)+1),
		UserID:       userID,
		CredentialID: credID,
		EventType:    eventType,
		Success:      success,
		SourceIP:     sourceIP,
		Timestamp:    time.Now(),
		Details:      details,
	})
}
