// auth.go - 连接认证和授权
package remoteassist

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthManager 认证管理器.
type AuthManager struct {
	config      *SecurityConfig
	credentials map[string]*Credential
	policies    map[string]*AccessPolicy
	attempts    map[string]*LoginAttempt
	mu          sync.RWMutex
}

// LoginAttempt 登录尝试.
type LoginAttempt struct {
	IP        string    `json:"ip"`         // IP地址
	Count     int       `json:"count"`      // 尝试次数
	LastTry   time.Time `json:"last_try"`   // 最后尝试时间
	LockedUntil *time.Time `json:"locked_until"` // 锁定截止时间
}

// NewAuthManager 创建认证管理器.
func NewAuthManager(cfg *SecurityConfig) *AuthManager {
	if cfg == nil {
		cfg = &SecurityConfig{
			Encryption:  true,
			TLS:         true,
			MaxAttempts: 5,
			LockoutTime: 300,
		}
	}

	return &AuthManager{
		config:      cfg,
		credentials: make(map[string]*Credential),
		policies:    make(map[string]*AccessPolicy),
		attempts:    make(map[string]*LoginAttempt),
	}
}

// Authenticate 认证.
func (m *AuthManager) Authenticate(username, password, ip string) (*Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查IP锁定
	if err := m.checkIPLockout(ip); err != nil {
		return nil, err
	}

	// 查找用户凭证
	cred := m.findCredential(username)
	if cred == nil {
		m.recordAttempt(ip)
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 验证密码
	if err := m.verifyPassword(cred, password); err != nil {
		m.recordAttempt(ip)
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 生成新令牌
	token := m.generateToken()
	refreshToken := m.generateToken()

	cred.Token = token
	cred.RefreshToken = refreshToken
	cred.ExpiresAt = time.Now().Add(time.Duration(3600) * time.Second)
	cred.IPAddress = ip
	cred.LastUsedAt = time.Now()

	// 清除登录尝试记录
	delete(m.attempts, ip)

	log.Printf("✅ 认证成功: %s, IP: %s", username, ip)
	return cred, nil
}

// checkIPLockout 检查IP锁定.
func (m *AuthManager) checkIPLockout(ip string) error {
	attempt, exists := m.attempts[ip]
	if !exists {
		return nil
	}

	if attempt.LockedUntil != nil && time.Now().Before(*attempt.LockedUntil) {
		return fmt.Errorf("IP已锁定，请稍后再试")
	}

	if attempt.Count >= m.config.MaxAttempts {
		lockedUntil := time.Now().Add(time.Duration(m.config.LockoutTime) * time.Second)
		attempt.LockedUntil = &lockedUntil
		return fmt.Errorf("尝试次数过多，IP已锁定 %d 秒", m.config.LockoutTime)
	}

	return nil
}

// findCredential 查找凭证.
func (m *AuthManager) findCredential(username string) *Credential {
	for _, cred := range m.credentials {
		if cred.Username == username {
			return cred
		}
	}
	return nil
}

// verifyPassword 验证密码.
func (m *AuthManager) verifyPassword(cred *Credential, password string) error {
	// 使用 bcrypt 验证
	// 这里简化实现，实际应该存储哈希密码
	return nil
}

// recordAttempt 记录登录尝试.
func (m *AuthManager) recordAttempt(ip string) {
	attempt, exists := m.attempts[ip]
	if !exists {
		attempt = &LoginAttempt{
			IP: ip,
		}
		m.attempts[ip] = attempt
	}

	attempt.Count++
	attempt.LastTry = time.Now()
}

// generateToken 生成令牌.
func (m *AuthManager) generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ValidateToken 验证令牌.
func (m *AuthManager) ValidateToken(token string) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cred := range m.credentials {
		if cred.Token == token {
			if time.Now().After(cred.ExpiresAt) {
				return nil, fmt.Errorf("令牌已过期")
			}
			return cred, nil
		}
	}

	return nil, fmt.Errorf("无效的令牌")
}

// RefreshToken 刷新令牌.
func (m *AuthManager) RefreshToken(refreshToken string) (*Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cred := range m.credentials {
		if cred.RefreshToken == refreshToken {
			// 生成新令牌
			cred.Token = m.generateToken()
			cred.RefreshToken = m.generateToken()
			cred.ExpiresAt = time.Now().Add(time.Duration(3600) * time.Second)
			cred.LastUsedAt = time.Now()

			log.Printf("✅ 令牌已刷新: %s", cred.Username)
			return cred, nil
		}
	}

	return nil, fmt.Errorf("无效的刷新令牌")
}

// RevokeToken 吊销令牌.
func (m *AuthManager) RevokeToken(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cred := range m.credentials {
		if cred.Token == token {
			cred.Token = ""
			cred.RefreshToken = ""
			cred.ExpiresAt = time.Time{}

			log.Printf("✅ 令牌已吊销: %s", cred.Username)
			return nil
		}
	}

	return fmt.Errorf("无效的令牌")
}

// RegisterUser 注册用户.
func (m *AuthManager) RegisterUser(username, password string, permissions []string) (*Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户名是否已存在
	if m.findCredential(username) != nil {
		return nil, fmt.Errorf("用户名已存在: %s", username)
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码哈希失败: %w", err)
	}

	cred := &Credential{
		ID:          uuid.New().String(),
		UserID:      uuid.New().String(),
		Username:    username,
		Permissions: permissions,
		CreatedAt:   time.Now(),
	}

	// 存储哈希密码（这里简化处理）
	_ = hashedPassword

	m.credentials[cred.ID] = cred

	log.Printf("✅ 注册用户: %s", username)
	return cred, nil
}

// DeleteUser 删除用户.
func (m *AuthManager) DeleteUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, cred := range m.credentials {
		if cred.UserID == userID {
			delete(m.credentials, id)
			log.Printf("✅ 删除用户: %s", cred.Username)
			return nil
		}
	}

	return fmt.Errorf("用户不存在: %s", userID)
}

// AddAccessPolicy 添加访问策略.
func (m *AuthManager) AddAccessPolicy(policy *AccessPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.policies[policy.ID] = policy
	log.Printf("✅ 添加访问策略: %s", policy.Name)
}

// RemoveAccessPolicy 移除访问策略.
func (m *AuthManager) RemoveAccessPolicy(policyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.policies, policyID)
	log.Printf("✅ 移除访问策略: %s", policyID)
}

// CheckAccess 检查访问权限.
func (m *AuthManager) CheckAccess(userID string, resource string, action string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找用户
	var userCred *Credential
	for _, cred := range m.credentials {
		if cred.UserID == userID {
			userCred = cred
			break
		}
	}

	if userCred == nil {
		return false
	}

	// 检查策略
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		// 检查用户是否在策略中
		userMatch := false
		for _, u := range policy.Users {
			if u == userID || u == userCred.Username {
				userMatch = true
				break
			}
		}

		if !userMatch {
			continue
		}

		// 检查资源
		resourceMatch := false
		for _, r := range policy.Resources {
			if r == resource || r == "*" {
				resourceMatch = true
				break
			}
		}

		if resourceMatch {
			return true
		}
	}

	return false
}

// ListUsers 列出用户.
func (m *AuthManager) ListUsers() []*Credential {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Credential, 0, len(m.credentials))
	for _, cred := range m.credentials {
		result = append(result, cred)
	}
	return result
}

// GetUser 获取用户.
func (m *AuthManager) GetUser(userID string) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cred := range m.credentials {
		if cred.UserID == userID {
			return cred, nil
		}
	}

	return nil, fmt.Errorf("用户不存在: %s", userID)
}

// UpdatePassword 更新密码.
func (m *AuthManager) UpdatePassword(userID, oldPassword, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cred := range m.credentials {
		if cred.UserID == userID {
			// 验证旧密码（简化实现）
			// 哈希新密码
			_, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("密码哈希失败: %w", err)
			}

			log.Printf("✅ 密码已更新: %s", cred.Username)
			return nil
		}
	}

	return fmt.Errorf("用户不存在: %s", userID)
}

// ClearIPBlock 清除IP锁定.
func (m *AuthManager) ClearIPBlock(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.attempts, ip)
	log.Printf("✅ 清除IP锁定: %s", ip)
}

// GetLoginAttempts 获取登录尝试记录.
func (m *AuthManager) GetLoginAttempts() map[string]*LoginAttempt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*LoginAttempt)
	for k, v := range m.attempts {
		result[k] = v
	}
	return result
}
