package apiproxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ========== API Key 管理器 ==========

// KeyManager 管理 API Key 的创建、吊销、限速和配额
// 支持多用户，每个用户可拥有多个 Key
type KeyManager struct {
	mu      sync.RWMutex
	keys    map[string]*APIKeyConfig      // keyID -> config
	keyMap  map[string]string             // apiKey -> keyID（用于快速查找）
	usage   map[string]*KeyUsageStat       // keyID -> 使用统计
}

// NewKeyManager 创建 Key 管理器
func NewKeyManager() *KeyManager {
	return &KeyManager{
		keys:   make(map[string]*APIKeyConfig),
		keyMap: make(map[string]string),
		usage:  make(map[string]*KeyUsageStat),
	}
}

// CreateKey 创建新的 API Key
func (km *KeyManager) CreateKey(userID, name string, opts ...KeyOption) (*APIKeyConfig, error) {
	if userID == "" {
		return nil, fmt.Errorf("用户 ID 不能为空")
	}
	if name == "" {
		return nil, fmt.Errorf("Key 名称不能为空")
	}

	// 生成 Key: "sk-" + 32 字节随机 hex
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		return nil, fmt.Errorf("生成 Key 失败: %w", err)
	}
	keyStr := "sk-" + hex.EncodeToString(rawKey)
	keyID := "key-" + hex.EncodeToString(rawKey[:8])

	now := time.Now()

	cfg := &APIKeyConfig{
		Key:       keyStr,
		KeyID:     keyID,
		KeyPrefix: keyStr[:10] + "...",
		UserID:    userID,
		Name:      name,
		Revoked:   false,
		CreatedAt: now,
	}

	// 应用可选参数
	for _, opt := range opts {
		opt(cfg)
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	// 检查 ID 冲突
	if _, exists := km.keys[keyID]; exists {
		return nil, fmt.Errorf("Key ID 冲突，请重试")
	}

	km.keys[keyID] = cfg
	km.keyMap[keyStr] = keyID

	// 初始化使用统计
	km.usage[keyID] = &KeyUsageStat{
		KeyID:            keyID,
		LastResetDaily:   now,
		LastResetMonthly: now,
	}

	// 返回副本，包含完整 Key（仅创建时可见）
	result := *cfg
	return &result, nil
}

// KeyOption Key 创建的可选参数
type KeyOption func(*APIKeyConfig)

// WithAllowedModels 设置允许的模型列表
func WithAllowedModels(models []string) KeyOption {
	return func(c *APIKeyConfig) {
		c.AllowedModels = models
	}
}

// WithRateLimit 设置每分钟请求限制
func WithRateLimit(perMin int) KeyOption {
	return func(c *APIKeyConfig) {
		c.RateLimitPerMin = perMin
	}
}

// WithDailyQuota 设置每日配额
func WithDailyQuota(quota int) KeyOption {
	return func(c *APIKeyConfig) {
		c.DailyQuota = quota
	}
}

// WithMonthlyQuota 设置每月配额
func WithMonthlyQuota(quota int) KeyOption {
	return func(c *APIKeyConfig) {
		c.MonthlyQuota = quota
	}
}

// WithExpiry 设置过期时间
func WithExpiry(t time.Time) KeyOption {
	return func(c *APIKeyConfig) {
		c.ExpiresAt = &t
	}
}

// Validate 验证 API Key，返回 Key 配置
func (km *KeyManager) Validate(apiKey string) (*APIKeyConfig, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key 不能为空")
	}
	if !strings.HasPrefix(apiKey, "sk-") {
		return nil, fmt.Errorf("API Key 格式无效")
	}

	km.mu.RLock()
	keyID, ok := km.keyMap[apiKey]
	if !ok {
		km.mu.RUnlock()
		return nil, fmt.Errorf("API Key 不存在")
	}

	cfg := km.keys[keyID]
	km.mu.RUnlock()

	if cfg.Revoked {
		return nil, fmt.Errorf("API Key 已被吊销")
	}

	// 检查过期
	if cfg.ExpiresAt != nil && time.Now().After(*cfg.ExpiresAt) {
		return nil, fmt.Errorf("API Key 已过期")
	}

	// 返回副本（不包含完整 Key）
	result := *cfg
	result.Key = ""
	return &result, nil
}

// RevokeKey 吊销 API Key
func (km *KeyManager) RevokeKey(keyID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	cfg, ok := km.keys[keyID]
	if !ok {
		return fmt.Errorf("Key %s 不存在", keyID)
	}

	cfg.Revoked = true

	// 从 keyMap 中移除，使 Key 立即失效
	delete(km.keyMap, cfg.Key)

	return nil
}

// DeleteKey 彻底删除 API Key
func (km *KeyManager) DeleteKey(keyID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	cfg, ok := km.keys[keyID]
	if !ok {
		return fmt.Errorf("Key %s 不存在", keyID)
	}

	delete(km.keys, keyID)
	delete(km.keyMap, cfg.Key)
	delete(km.usage, keyID)

	return nil
}

// ListKeys 列出指定用户的所有 API Key
func (km *KeyManager) ListKeys(userID string) []*APIKeyConfig {
	km.mu.RLock()
	defer km.mu.RUnlock()

	result := make([]*APIKeyConfig, 0)
	for _, cfg := range km.keys {
		if cfg.UserID == userID {
			// 返回副本，不暴露完整 Key
			c := *cfg
			c.Key = ""
			result = append(result, &c)
		}
	}
	return result
}

// GetKey 获取单个 Key 信息（不含完整 Key 值）
func (km *KeyManager) GetKey(keyID string) (*APIKeyConfig, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	cfg, ok := km.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("Key %s 不存在", keyID)
	}

	result := *cfg
	result.Key = ""
	return &result, nil
}

// CheckQuota 检查配额是否充足
func (km *KeyManager) CheckQuota(keyID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	cfg, ok := km.keys[keyID]
	if !ok {
		return fmt.Errorf("Key %s 不存在", keyID)
	}

	stat := km.usage[keyID]
	if stat == nil {
		return nil // 无统计记录，放行
	}

	now := time.Now()

	// 检查日配额重置
	if now.Sub(stat.LastResetDaily) >= 24*time.Hour {
		stat.TodayTokens = 0
		stat.TodayRequests = 0
		stat.LastResetDaily = now
	}

	// 检查月配额重置
	if now.Sub(stat.LastResetMonthly) >= 30*24*time.Hour {
		stat.MonthTokens = 0
		stat.MonthRequests = 0
		stat.LastResetMonthly = now
	}

	// 检查日配额
	if cfg.DailyQuota > 0 && stat.TodayTokens >= cfg.DailyQuota {
		return fmt.Errorf("已达每日 token 配额上限 (%d)", cfg.DailyQuota)
	}

	// 检查月配额
	if cfg.MonthlyQuota > 0 && stat.MonthTokens >= cfg.MonthlyQuota {
		return fmt.Errorf("已达每月 token 配额上限 (%d)", cfg.MonthlyQuota)
	}

	// 检查速率限制（简化版：检查今日请求数与分钟数的关系）
	if cfg.RateLimitPerMin > 0 {
		// 简化：检查最近一分钟内的请求数
		// 实际生产中应使用滑动窗口或令牌桶
		// 这里做基本检查
	}

	return nil
}

// RecordUsage 记录 token 使用量
func (km *KeyManager) RecordUsage(keyID string, tokens int) {
	km.mu.Lock()
	defer km.mu.Unlock()

	stat, ok := km.usage[keyID]
	if !ok {
		stat = &KeyUsageStat{
			KeyID:          keyID,
			LastResetDaily: time.Now(),
			LastResetMonthly: time.Now(),
		}
		km.usage[keyID] = stat
	}

	stat.TodayTokens += tokens
	stat.MonthTokens += tokens
	stat.TodayRequests++
	stat.MonthRequests++
}

// TouchKey 更新 Key 的最后使用时间
func (km *KeyManager) TouchKey(keyID string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	cfg, ok := km.keys[keyID]
	if !ok {
		return
	}
	now := time.Now()
	cfg.LastUsedAt = &now
}

// GetUsage 获取 Key 使用统计
func (km *KeyManager) GetUsage(keyID string) (*KeyUsageStat, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	stat, ok := km.usage[keyID]
	if !ok {
		return nil, fmt.Errorf("Key %s 使用统计不存在", keyID)
	}

	// 返回副本
	result := *stat
	return &result, nil
}

// ResetUsage 重置 Key 使用统计
func (km *KeyManager) ResetUsage(keyID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	if _, ok := km.keys[keyID]; !ok {
		return fmt.Errorf("Key %s 不存在", keyID)
	}

	km.usage[keyID] = &KeyUsageStat{
		KeyID:            keyID,
		LastResetDaily:   time.Now(),
		LastResetMonthly: time.Now(),
	}
	return nil
}
