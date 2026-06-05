// Package smartcachetier 提供多级缓存智能管理功能
package smartcachetier

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 多级缓存智能管理器.
type Manager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	tiers        map[TierLevel]*tierData   // 层级数据
	entries      map[string]*CacheEntry    // key -> CacheEntry
	config       *CacheConfig
	totalHits    int64
	totalMisses  int64
	promotionCnt int64
	demotionCnt  int64
	configPath   string
}

// tierData 层级内部数据.
type tierData struct {
	info    TierInfo
	entries map[string]struct{} // 该层级的 key 集合
}

// NewManager 创建多级缓存智能管理器.
func NewManager(logger *zap.Logger, configPath string) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:     logger,
		tiers:      make(map[TierLevel]*tierData),
		entries:    make(map[string]*CacheEntry),
		config:     defaultCacheConfig(),
		configPath: configPath,
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			logger.Warn("加载缓存配置失败，使用默认配置", zap.Error(err))
		}
	}

	return m
}

// defaultCacheConfig 默认缓存配置.
func defaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultPolicy: PolicyLRU,
		PromotionPolicy: PromotionPolicy{
			HitThreshold:    10,
			HitRateMin:      0.5,
			IdleDurationSec: 300,
		},
		EnableAutoTiering: true,
	}
}

// ========== 层级管理 ==========

// CreateTier 创建缓存层级.
func (m *Manager) CreateTier(req TierCreateRequest) (*TierInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tiers[req.Level]; exists {
		return nil, ErrDuplicateTier
	}

	policy := req.Policy
	if policy == "" {
		policy = m.config.DefaultPolicy
	}
	if !isValidPolicy(policy) {
		return nil, ErrInvalidPolicy
	}

	maxEntries := req.MaxEntries
	if maxEntries == 0 {
		maxEntries = 10000
	}

	info := TierInfo{
		Level:      req.Level,
		Name:       req.Name,
		DevicePath: req.DevicePath,
		TotalBytes: req.TotalBytes,
		UsedBytes:  0,
		EntryCount: 0,
		Policy:     policy,
		MaxEntries: maxEntries,
		CreatedAt:  time.Now(),
	}

	m.tiers[req.Level] = &tierData{
		info:    info,
		entries: make(map[string]struct{}),
	}

	m.logger.Info("缓存层级已创建",
		zap.Int("level", int(req.Level)),
		zap.String("name", req.Name),
	)

	return &info, nil
}

// GetTier 获取缓存层级信息.
func (m *Manager) GetTier(level TierLevel) (*TierInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	td, exists := m.tiers[level]
	if !exists {
		return nil, ErrCacheTierNotFound
	}

	info := td.info
	return &info, nil
}

// ListTiers 列出所有缓存层级.
func (m *Manager) ListTiers() []TierInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tiers := make([]TierInfo, 0, len(m.tiers))
	for _, td := range m.tiers {
		tiers = append(tiers, td.info)
	}
	return tiers
}

// DeleteTier 删除缓存层级.
func (m *Manager) DeleteTier(level TierLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.tiers[level]
	if !exists {
		return ErrCacheTierNotFound
	}

	// 删除该层级的所有缓存条目
	for key := range td.entries {
		delete(m.entries, key)
	}
	delete(m.tiers, level)

	m.logger.Info("缓存层级已删除", zap.Int("level", int(level)))
	return nil
}

// ========== 缓存操作 ==========

// Set 设置缓存条目（默认放入最低层级 HDD）.
func (m *Manager) Set(req CacheSetRequest) (*CacheEntry, error) {
	return m.SetToTier(req, TierHDD)
}

// SetToTier 设置缓存条目到指定层级.
func (m *Manager) SetToTier(req CacheSetRequest, level TierLevel) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	td, exists := m.tiers[level]
	if !exists {
		return nil, ErrCacheTierNotFound
	}

	// 检查容量
	if td.info.UsedBytes+req.Size > td.info.TotalBytes {
		return nil, ErrCacheTierFull
	}
	if td.info.EntryCount >= td.info.MaxEntries {
		return nil, ErrCacheTierFull
	}

	// 如果已存在，先移除旧的
	if old, ok := m.entries[req.Key]; ok {
		m.removeFromTier(old.Key, old.Tier)
	}

	entry := &CacheEntry{
		Key:        req.Key,
		Size:       req.Size,
		Tier:       level,
		HitCount:   0,
		LastAccess: time.Now(),
		CreatedAt:  time.Now(),
	}

	m.entries[req.Key] = entry
	td.entries[req.Key] = struct{}{}
	td.info.UsedBytes += req.Size
	td.info.EntryCount++

	return entry, nil
}

// Get 获取缓存条目并增加命中计数.
func (m *Manager) Get(key string) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		m.totalMisses++
		return nil, ErrCacheEntryNotFound
	}

	entry.HitCount++
	entry.LastAccess = time.Now()
	m.totalHits++

	// 更新层级命中统计
	if td, ok := m.tiers[entry.Tier]; ok {
		td.info.HitCount++
	}

	return entry, nil
}

// Delete 删除缓存条目.
func (m *Manager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		return ErrCacheEntryNotFound
	}

	m.removeFromTier(key, entry.Tier)
	delete(m.entries, key)
	return nil
}

// removeFromTier 从层级中移除条目（内部方法，需持有锁）.
func (m *Manager) removeFromTier(key string, level TierLevel) {
	td, exists := m.tiers[level]
	if !exists {
		return
	}

	if entry, ok := m.entries[key]; ok {
		td.info.UsedBytes -= entry.Size
		td.info.EntryCount--
	}
	delete(td.entries, key)
}

// ========== 智能分层 ==========

// PromoteEntry 将条目提升到更高层级.
func (m *Manager) PromoteEntry(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		return ErrCacheEntryNotFound
	}

	if entry.Tier >= TierNVMe {
		return nil // 已在最高层级
	}

	targetLevel := entry.Tier + 1
	td, exists := m.tiers[targetLevel]
	if !exists {
		return ErrCacheTierNotFound
	}

	if td.info.UsedBytes+entry.Size > td.info.TotalBytes {
		return ErrCacheTierFull
	}

	// 从旧层级移除
	m.removeFromTier(key, entry.Tier)

	// 添加到新层级
	entry.Tier = targetLevel
	td.entries[key] = struct{}{}
	td.info.UsedBytes += entry.Size
	td.info.EntryCount++
	m.promotionCnt++

	m.logger.Debug("缓存条目已提升",
		zap.String("key", key),
		zap.Int("from", int(entry.Tier-1)),
		zap.Int("to", int(targetLevel)),
	)

	return nil
}

// DemoteEntry 将条目降低到更低层级.
func (m *Manager) DemoteEntry(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		return ErrCacheEntryNotFound
	}

	if entry.Tier <= TierHDD {
		return nil // 已在最低层级
	}

	targetLevel := entry.Tier - 1
	td, exists := m.tiers[targetLevel]
	if !exists {
		return ErrCacheTierNotFound
	}

	// 从旧层级移除
	m.removeFromTier(key, entry.Tier)

	// 添加到新层级
	entry.Tier = targetLevel
	td.entries[key] = struct{}{}
	td.info.UsedBytes += entry.Size
	td.info.EntryCount++
	m.demotionCnt++

	m.logger.Debug("缓存条目已降级",
		zap.String("key", key),
		zap.Int("from", int(entry.Tier+1)),
		zap.Int("to", int(targetLevel)),
	)

	return nil
}

// RunAutoTiering 执行自动分层（根据访问频率提升/降级）.
func (m *Manager) RunAutoTiering() (promoted, demoted int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.EnableAutoTiering {
		return 0, 0
	}

	policy := m.config.PromotionPolicy

	for key, entry := range m.entries {
		// 提升条件：命中次数超过阈值且不在最高层级
		if entry.HitCount >= int64(policy.HitThreshold) && entry.Tier < TierNVMe {
			if m.promoteEntryLocked(key) {
				promoted++
			}
		}

		// 降级条件：长时间未访问且不在最低层级
		idleSec := int(time.Since(entry.LastAccess).Seconds())
		if idleSec > policy.IdleDurationSec && entry.Tier > TierHDD {
			if m.demoteEntryLocked(key) {
				demoted++
			}
		}
	}

	return promoted, demoted
}

// promoteEntryLocked 内部提升方法（需持有锁）.
func (m *Manager) promoteEntryLocked(key string) bool {
	entry := m.entries[key]
	targetLevel := entry.Tier + 1
	td, exists := m.tiers[targetLevel]
	if !exists {
		return false
	}
	if td.info.UsedBytes+entry.Size > td.info.TotalBytes {
		return false
	}

	m.removeFromTier(key, entry.Tier)
	entry.Tier = targetLevel
	td.entries[key] = struct{}{}
	td.info.UsedBytes += entry.Size
	td.info.EntryCount++
	m.promotionCnt++
	return true
}

// demoteEntryLocked 内部降级方法（需持有锁）.
func (m *Manager) demoteEntryLocked(key string) bool {
	entry := m.entries[key]
	targetLevel := entry.Tier - 1
	td, exists := m.tiers[targetLevel]
	if !exists {
		return false
	}

	m.removeFromTier(key, entry.Tier)
	entry.Tier = targetLevel
	td.entries[key] = struct{}{}
	td.info.UsedBytes += entry.Size
	td.info.EntryCount++
	m.demotionCnt++
	return true
}

// ========== 统计 ==========

// GetStats 获取缓存统计信息.
func (m *Manager) GetStats() *CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRequests := m.totalHits + m.totalMisses
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(m.totalHits) / float64(totalRequests)
	}

	totalUsed := uint64(0)
	totalEntries := 0
	tierStats := make([]TierStats, 0, len(m.tiers))

	for _, td := range m.tiers {
		totalUsed += td.info.UsedBytes
		totalEntries += td.info.EntryCount

		tHitRate := 0.0
		if td.info.HitCount > 0 {
			tHitRate = float64(td.info.HitCount) / float64(totalRequests)
		}

		tierStats = append(tierStats, TierStats{
			Level:      td.info.Level,
			Name:       td.info.Name,
			EntryCount: td.info.EntryCount,
			UsedBytes:  td.info.UsedBytes,
			HitCount:   td.info.HitCount,
			HitRate:    tHitRate,
		})
	}

	return &CacheStats{
		TotalEntries:   totalEntries,
		TotalUsedBytes: totalUsed,
		TotalHitCount:  m.totalHits,
		HitRate:        hitRate,
		Tiers:          tierStats,
		PromotionCount: m.promotionCnt,
		DemotionCount:  m.demotionCnt,
	}
}

// ========== 配置管理 ==========

// GetConfig 获取缓存配置.
func (m *Manager) GetConfig() *CacheConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新缓存配置.
func (m *Manager) UpdateConfig(config *CacheConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !isValidPolicy(config.DefaultPolicy) {
		return ErrInvalidPolicy
	}

	m.config = config
	m.logger.Info("缓存配置已更新")
	return nil
}

// ========== 内部方法 ==========

func isValidPolicy(p CachePolicy) bool {
	return p == PolicyLRU || p == PolicyLFU || p == PolicyARC
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config CacheConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	m.config = &config
	return nil
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
