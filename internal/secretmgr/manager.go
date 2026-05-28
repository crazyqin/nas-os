// Package secretmgr 提供统一密钥/凭据管理
package secretmgr

import (
	"fmt"
	"sync"
	"time"
)

// Manager 密钥管理器.
type Manager struct {
	mu       sync.RWMutex
	secrets  map[string]*Secret
	versions map[string][]*SecretVersion
	logs     []*AccessLog
	stats    SecretStats
}

// NewManager 创建密钥管理器.
func NewManager() *Manager {
	return &Manager{
		secrets:  make(map[string]*Secret),
		versions: make(map[string][]*SecretVersion),
	}
}

// CreateSecret 创建密钥.
func (m *Manager) CreateSecret(req CreateSecretRequest) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	secretType := req.Type
	if secretType == "" {
		secretType = SecretTypeGeneric
	}

	id := fmt.Sprintf("sec-%d", time.Now().UnixNano())
	secret := &Secret{
		ID:          id,
		Name:        req.Name,
		Type:        secretType,
		Description: req.Description,
		Value:       req.Value,
		Metadata:    req.Metadata,
		Tags:        req.Tags,
		Status:      SecretStatusActive,
		ExpiresAt:   req.ExpiresAt,
		RotateDays:  req.RotateDays,
		Version:     1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.secrets[id] = secret

	// 保存初始版本
	m.versions[id] = append(m.versions[id], &SecretVersion{
		Version:   1,
		Value:     req.Value,
		CreatedAt: time.Now(),
	})

	m.stats.TotalSecrets++
	m.stats.ActiveSecrets++

	return secret, nil
}

// GetSecret 获取密钥.
func (m *Manager) GetSecret(id string) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.secrets[id]
	if !ok {
		return nil, ErrSecretNotFound
	}

	// 检查过期
	if s.ExpiresAt != nil && s.ExpiresAt.Before(time.Now()) {
		s.Status = SecretStatusExpired
	}

	// 记录访问
	now := time.Now()
	s.LastUsed = &now
	m.stats.TotalAccess++

	return s, nil
}

// ListSecrets 列出所有密钥.
func (m *Manager) ListSecrets() []*Secret {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secrets := make([]*Secret, 0, len(m.secrets))
	for _, s := range m.secrets {
		secrets = append(secrets, s)
	}
	return secrets
}

// DeleteSecret 删除密钥.
func (m *Manager) DeleteSecret(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.secrets[id]; !ok {
		return ErrSecretNotFound
	}

	delete(m.secrets, id)
	delete(m.versions, id)
	m.stats.TotalSecrets--
	m.stats.ActiveSecrets--
	return nil
}

// UpdateSecret 更新密钥.
func (m *Manager) UpdateSecret(id string, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.secrets[id]
	if !ok {
		return ErrSecretNotFound
	}

	s.Value = value
	s.Version++
	s.UpdatedAt = time.Now()

	m.versions[id] = append(m.versions[id], &SecretVersion{
		Version:   s.Version,
		Value:     value,
		CreatedAt: time.Now(),
	})

	return nil
}

// RotateSecret 轮换密钥.
func (m *Manager) RotateSecret(id string, newValue string) error {
	return m.UpdateSecret(id, newValue)
}

// GetVersions 获取密钥版本历史.
func (m *Manager) GetVersions(id string) []*SecretVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.versions[id]
}

// RevokeSecret 撤销密钥.
func (m *Manager) RevokeSecret(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.secrets[id]
	if !ok {
		return ErrSecretNotFound
	}

	s.Status = SecretStatusRevoked
	s.UpdatedAt = time.Now()
	m.stats.ActiveSecrets--
	return nil
}

// GetStats 获取统计.
func (m *Manager) GetStats() SecretStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
