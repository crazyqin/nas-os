package apikeymgr

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// NewAPIKeyManager 创建API Key管理器
func NewAPIKeyManager(cfg ManagerConfig) *APIKeyManager {
	return &APIKeyManager{
		keys:   make(map[string]*APIKey),
		config: cfg,
	}
}

// CreateKey 创建API Key
func (m *APIKeyManager) CreateKey(req CreateKeyRequest) (*APIKey, string, error) {
	if req.UserID == "" {
		return nil, "", ErrUserIDRequired
	}
	if req.Name == "" {
		return nil, "", ErrNameRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户密钥数量限制
	count := 0
	for _, k := range m.keys {
		if k.UserID == req.UserID && k.Status == StatusActive {
			count++
		}
	}
	if count >= m.config.MaxKeysPerUser {
		return nil, "", ErrMaxKeysReached
	}

	// 生成随机密钥
	rawKey, err := generateRandomKey(m.config.KeyLength)
	if err != nil {
		return nil, "", fmt.Errorf("generate key failed: %w", err)
	}

	// 计算哈希
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// 密钥前缀（用于显示）
	prefix := rawKey[:8]

	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = m.config.DefaultRateLimit
	}

	keyID := fmt.Sprintf("key-%d", time.Now().UnixNano())
	now := time.Now()

	key := &APIKey{
		ID:          keyID,
		UserID:      req.UserID,
		Name:        req.Name,
		KeyHash:     keyHash,
		Prefix:      prefix,
		Permissions: req.Permissions,
		Status:      StatusActive,
		RateLimit:   rateLimit,
		CreatedAt:   now,
	}

	if req.ExpiresIn != 0 {
		exp := now.AddDate(0, 0, req.ExpiresIn)
		key.ExpiresAt = &exp
	}

	m.keys[keyID] = key
	return key, rawKey, nil
}

// ValidateKey 验证API Key
func (m *APIKeyManager) ValidateKey(rawKey string) (*APIKey, error) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key := range m.keys {
		if key.KeyHash == keyHash {
			if key.Status == StatusRevoked {
				return nil, ErrKeyRevoked
			}
			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				return nil, ErrKeyExpired
			}
			// 更新使用统计
			now := time.Now()
			key.LastUsedAt = &now
			key.UsageCount++
			return key, nil
		}
	}
	return nil, ErrKeyNotFound
}

// RevokeKey 吊销密钥
func (m *APIKeyManager) RevokeKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, exists := m.keys[keyID]
	if !exists {
		return ErrKeyNotFound
	}
	now := time.Now()
	key.Status = StatusRevoked
	key.RevokedAt = &now
	return nil
}

// GetKey 获取密钥信息
func (m *APIKeyManager) GetKey(keyID string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key, exists := m.keys[keyID]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return key, nil
}

// ListUserKeys 列出用户的所有密钥
func (m *APIKeyManager) ListUserKeys(userID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*APIKey, 0)
	for _, key := range m.keys {
		if key.UserID == userID {
			result = append(result, key)
		}
	}
	return result
}

// RotateKey 轮换密钥
func (m *APIKeyManager) RotateKey(keyID string) (*APIKey, string, error) {
	m.mu.Lock()
	oldKey, exists := m.keys[keyID]
	if !exists {
		m.mu.Unlock()
		return nil, "", ErrKeyNotFound
	}

	// 吊销旧密钥
	now := time.Now()
	oldKey.Status = StatusRevoked
	oldKey.RevokedAt = &now
	m.mu.Unlock()

	// 创建新密钥
	req := CreateKeyRequest{
		UserID:      oldKey.UserID,
		Name:        oldKey.Name + " (rotated)",
		Permissions: oldKey.Permissions,
		RateLimit:   oldKey.RateLimit,
	}
	if oldKey.ExpiresAt != nil {
		req.ExpiresIn = int(time.Until(*oldKey.ExpiresAt).Hours() / 24)
	}
	return m.CreateKey(req)
}

// GetUsageStats 获取使用统计
func (m *APIKeyManager) GetUsageStats(keyID string) (*KeyUsageStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key, exists := m.keys[keyID]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return &KeyUsageStats{
		KeyID:      key.ID,
		TotalCalls: key.UsageCount,
		LastUsedAt: key.LastUsedAt,
	}, nil
}

// CleanupExpired 清理过期密钥
func (m *APIKeyManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	now := time.Now()
	for _, key := range m.keys {
		if key.Status == StatusActive && key.ExpiresAt != nil && key.ExpiresAt.Before(now) {
			key.Status = StatusExpired
			count++
		}
	}
	return count
}

func generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
