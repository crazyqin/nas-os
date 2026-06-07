// Package apikey 密钥管理核心实现
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// KeyManager API 密钥管理器
type KeyManager struct {
	mu          sync.RWMutex
	keys        map[string]*APIKey  // keyID -> APIKey
	userKeys    map[string][]string // userID -> keyIDs
	keyHashes   map[string]string   // keyHash -> keyID（用于快速验证）
	configPath  string
	policy      APIKeyPolicy
	usageLog    *UsageLogger
	auditLogger AuditLogger
}

// AuditLogger 审计日志接口
type AuditLogger interface {
	LogAPIKeyEvent(event, keyID, userID, ip, status, reason string, details map[string]interface{})
}

// NewKeyManager 创建密钥管理器
func NewKeyManager(configPath string, policy APIKeyPolicy) (*KeyManager, error) {
	m := &KeyManager{
		keys:       make(map[string]*APIKey),
		userKeys:   make(map[string][]string),
		keyHashes:  make(map[string]string),
		configPath: configPath,
		policy:     policy,
		usageLog:   NewUsageLogger(filepath.Join(filepath.Dir(configPath), "apikey_usage.log")),
	}

	// 加载现有密钥
	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("加载配置失败：%w", err)
	}

	return m, nil
}

// loadConfig 加载配置
func (m *KeyManager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil // 文件不存在，首次初始化
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	var config struct {
		Keys     map[string]*APIKey  `json:"keys"`
		UserKeys map[string][]string `json:"user_keys"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败：%w", err)
	}

	m.keys = config.Keys
	m.userKeys = config.UserKeys

	// 重建哈希索引
	for _, key := range m.keys {
		m.keyHashes[key.KeyHash] = key.ID
	}

	return nil
}

// saveConfig 保存配置
func (m *KeyManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	config := struct {
		Keys     map[string]*APIKey  `json:"keys"`
		UserKeys map[string][]string `json:"user_keys"`
	}{
		Keys:     m.keys,
		UserKeys: m.userKeys,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	// 密钥配置文件权限必须为 0600（STIG 要求）
	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// ========== 密钥创建和管理 ==========

// CreateKey 创建 API 密钥
func (m *KeyManager) CreateKey(userID string, req APIKeyCreateRequest) (*APIKeyCreateResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户密钥数量限制
	keyCount := len(m.userKeys[userID])
	if keyCount >= m.policy.MaxKeysPerUser {
		return nil, errors.New(ErrMaxKeysExceeded)
	}

	// 验证请求
	if len(req.Name) < 3 {
		return nil, errors.New("密钥名称至少 3 个字符")
	}

	// 生成密钥
	keyID := uuid.New().String()
	rawKey, keyPrefix := m.generateKey()

	// 计算哈希（不存储原始密钥）
	keyHash := sha256.Sum256([]byte(rawKey))
	hashStr := hex.EncodeToString(keyHash[:])

	// 设置默认值
	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = m.policy.DefaultRateLimit
	}

	// 检查过期时间
	if req.ExpiresAt != nil {
		maxExpiry := time.Now().AddDate(0, 0, m.policy.MaxExpiryDays)
		if req.ExpiresAt.After(maxExpiry) {
			return nil, fmt.Errorf("密钥有效期不能超过 %d 天", m.policy.MaxExpiryDays)
		}
	}

	// 创建密钥对象
	key := &APIKey{
		ID:          keyID,
		Name:        req.Name,
		KeyHash:     hashStr,
		KeyPrefix:   keyPrefix,
		UserID:      userID,
		Permissions: req.Permissions,
		Scopes:      req.Scopes,
		RateLimit:   rateLimit,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   time.Now(),
		Enabled:     true,
		Description: req.Description,
		SourceIPs:   req.SourceIPs,
	}

	// 存储
	m.keys[keyID] = key
	m.userKeys[userID] = append(m.userKeys[userID], keyID)
	m.keyHashes[hashStr] = keyID

	// 保存配置
	if err := m.saveConfig(); err != nil {
		// 回滚
		delete(m.keys, keyID)
		m.userKeys[userID] = m.userKeys[userID][:len(m.userKeys[userID])-1]
		delete(m.keyHashes, hashStr)
		return nil, err
	}

	// 记录审计日志
	if m.policy.EnableAudit && m.auditLogger != nil {
		m.auditLogger.LogAPIKeyEvent("key_create", keyID, userID, "", "success", "", map[string]interface{}{
			"name":       req.Name,
			"scopes":     req.Scopes,
			"expires_at": req.ExpiresAt,
		})
	}

	return &APIKeyCreateResponse{
		ID:        keyID,
		Name:      req.Name,
		Key:       rawKey, // 仅返回一次
		KeyPrefix: keyPrefix,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: key.CreatedAt,
		Warning:   "请妥善保管此密钥，系统不会再次显示完整密钥",
	}, nil
}

// generateKey 生成安全的 API 密钥
func (m *KeyManager) generateKey() (string, string) {
	// 生成随机密钥
	b := make([]byte, m.policy.MinKeyLength)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败是致命错误
	}

	// 密钥格式：nas_<random_hex>
	key := "nas_" + hex.EncodeToString(b)
	prefix := key[:12] // 前 12 字符作为识别前缀

	return key, prefix
}

// ValidateKey 验证 API 密钥
func (m *KeyManager) ValidateKey(rawKey string, sourceIP string) (*APIKey, error) {
	// 检查密钥格式
	if !strings.HasPrefix(rawKey, "nas_") || len(rawKey) < m.policy.MinKeyLength+4 {
		return nil, errors.New(ErrKeyFormatInvalid)
	}

	// 计算哈希
	keyHash := sha256.Sum256([]byte(rawKey))
	hashStr := hex.EncodeToString(keyHash[:])

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找密钥
	keyID, exists := m.keyHashes[hashStr]
	if !exists {
		return nil, errors.New(ErrKeyInvalid)
	}

	key, exists := m.keys[keyID]
	if !exists {
		return nil, errors.New(ErrKeyNotFound)
	}

	// 检查状态
	if !key.Enabled {
		return nil, errors.New(ErrKeyDisabled)
	}

	// 检查过期
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		if m.policy.AutoDisableExpired {
			key.Enabled = false
		}
		return nil, errors.New(ErrKeyExpired)
	}

	// 检查源 IP
	if len(key.SourceIPs) > 0 && !isIPAllowed(sourceIP, key.SourceIPs) {
		// 记录审计日志
		if m.policy.EnableAudit && m.auditLogger != nil {
			m.auditLogger.LogAPIKeyEvent("key_validate", keyID, key.UserID, sourceIP, "failure", ErrIPNotAllowed, nil)
		}
		return nil, errors.New(ErrIPNotAllowed)
	}

	return key, nil
}

// isIPAllowed 检查 IP 是否在允许列表中
func isIPAllowed(ip string, allowedCIDRs []string) bool {
	// 简化实现：检查 IP 是否在 CIDR 列表中
	// 完整实现需要使用 net 包进行 CIDR 匹配
	for _, cidr := range allowedCIDRs {
		if cidr == ip {
			return true
		}
		// 处理 CIDR 格式（如 192.168.1.0/24）
		if strings.HasSuffix(cidr, "/0") {
			return true // 允许所有
		}
		// 简单前缀匹配（生产环境应使用标准 CIDR 库）
		prefix := strings.Split(cidr, "/")[0]
		if strings.HasPrefix(ip, prefix[:len(prefix)-1]) {
			return true
		}
	}
	return false
}

// ========== 密钥操作 ==========

// GetKey 获取密钥信息（不含敏感数据）
func (m *KeyManager) GetKey(keyID string) (*APIKeySummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[keyID]
	if !exists {
		return nil, errors.New(ErrKeyNotFound)
	}

	return m.toSummary(key), nil
}

// ListKeys 列出用户的所有密钥
func (m *KeyManager) ListKeys(userID string) (*APIKeyListResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyIDs := m.userKeys[userID]
	keys := make([]APIKeySummary, 0, len(keyIDs))

	for _, keyID := range keyIDs {
		key, exists := m.keys[keyID]
		if !exists {
			continue
		}
		keys = append(keys, *m.toSummary(key))
	}

	return &APIKeyListResponse{
		Keys:  keys,
		Total: len(keys),
	}, nil
}

// UpdateKey 更新密钥
func (m *KeyManager) UpdateKey(keyID string, userID string, req APIKeyUpdateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return errors.New(ErrKeyNotFound)
	}

	// 验证所有权
	if key.UserID != userID {
		return errors.New(ErrPermissionDenied)
	}

	// 更新字段
	if req.Name != nil {
		if len(*req.Name) < 3 {
			return errors.New("密钥名称至少 3 个字符")
		}
		key.Name = *req.Name
	}
	if req.Permissions != nil {
		key.Permissions = req.Permissions
	}
	if req.Scopes != nil {
		key.Scopes = req.Scopes
	}
	if req.RateLimit != nil {
		key.RateLimit = *req.RateLimit
	}
	if req.ExpiresAt != nil {
		maxExpiry := time.Now().AddDate(0, 0, m.policy.MaxExpiryDays)
		if req.ExpiresAt.After(maxExpiry) {
			return fmt.Errorf("密钥有效期不能超过 %d 天", m.policy.MaxExpiryDays)
		}
		key.ExpiresAt = req.ExpiresAt
	}
	if req.Enabled != nil {
		key.Enabled = *req.Enabled
	}
	if req.Description != nil {
		key.Description = *req.Description
	}
	if req.SourceIPs != nil {
		key.SourceIPs = req.SourceIPs
	}

	now := time.Now()
	key.UpdatedAt = &now

	// 保存
	if err := m.saveConfig(); err != nil {
		return err
	}

	// 记录审计日志
	if m.policy.EnableAudit && m.auditLogger != nil {
		m.auditLogger.LogAPIKeyEvent("key_update", keyID, userID, "", "success", "", map[string]interface{}{
			"changes": req,
		})
	}

	return nil
}

// DeleteKey 删除密钥
func (m *KeyManager) DeleteKey(keyID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return errors.New(ErrKeyNotFound)
	}

	// 验证所有权
	if key.UserID != userID {
		return errors.New(ErrPermissionDenied)
	}

	// 删除
	delete(m.keys, keyID)
	delete(m.keyHashes, key.KeyHash)

	// 从用户列表中移除
	userKeys := m.userKeys[userID]
	for i, id := range userKeys {
		if id == keyID {
			m.userKeys[userID] = append(userKeys[:i], userKeys[i+1:]...)
			break
		}
	}

	// 保存
	if err := m.saveConfig(); err != nil {
		return err
	}

	// 记录审计日志
	if m.policy.EnableAudit && m.auditLogger != nil {
		m.auditLogger.LogAPIKeyEvent("key_delete", keyID, userID, "", "success", "", nil)
	}

	return nil
}

// RecordUsage 记录密钥使用
func (m *KeyManager) RecordUsage(keyID string, action, resource, sourceIP string, statusCode int, responseMs int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return
	}

	key.UsedCount++
	now := time.Now()
	key.LastUsedAt = &now

	// 记录使用日志
	if m.usageLog != nil {
		m.usageLog.Log(APIKeyUsage{
			KeyID:      keyID,
			Timestamp:  now,
			Action:     action,
			Resource:   resource,
			SourceIP:   sourceIP,
			StatusCode: statusCode,
			ResponseMs: responseMs,
		})
	}
}

// CheckPermission 检查密钥权限
func (m *KeyManager) CheckPermission(key *APIKey, resource, action string) bool {
	// 检查范围
	for _, scope := range key.Scopes {
		if scope.Resource == resource || scope.Resource == "*" {
			for _, a := range scope.Actions {
				if a == action || a == "*" {
					return true
				}
			}
		}
	}

	// 检查权限列表
	for _, perm := range key.Permissions {
		if perm == "*" || perm == resource+"."+action {
			return true
		}
	}

	return false
}

// ========== 辅助方法 ==========

// toSummary 转换为摘要
func (m *KeyManager) toSummary(key *APIKey) *APIKeySummary {
	isExpired := key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt)
	return &APIKeySummary{
		ID:          key.ID,
		Name:        key.Name,
		KeyPrefix:   key.KeyPrefix,
		Permissions: key.Permissions,
		Scopes:      key.Scopes,
		ExpiresAt:   key.ExpiresAt,
		CreatedAt:   key.CreatedAt,
		LastUsedAt:  key.LastUsedAt,
		UsedCount:   key.UsedCount,
		Enabled:     key.Enabled,
		IsExpired:   isExpired,
	}
}

// GetStats 获取统计信息
func (m *KeyManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_keys":    len(m.keys),
		"total_users":   len(m.userKeys),
		"enabled_keys":  0,
		"disabled_keys": 0,
		"expired_keys":  0,
	}

	now := time.Now()
	for _, key := range m.keys {
		if key.Enabled {
			stats["enabled_keys"] = stats["enabled_keys"].(int) + 1
		} else {
			stats["disabled_keys"] = stats["disabled_keys"].(int) + 1
		}
		if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
			stats["expired_keys"] = stats["expired_keys"].(int) + 1
		}
	}

	return stats
}

// SetAuditLogger 设置审计日志器
func (m *KeyManager) SetAuditLogger(logger AuditLogger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditLogger = logger
}

// CleanExpiredKeys 清理过期密钥
func (m *KeyManager) CleanExpiredKeys() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for _, key := range m.keys {
		if key.ExpiresAt != nil && now.After(*key.ExpiresAt) && m.policy.AutoDisableExpired {
			key.Enabled = false
			count++
		}
	}

	if count > 0 {
		_ = m.saveConfig()
	}

	return count
}
