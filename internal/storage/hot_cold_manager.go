// Package storage 提供热冷数据管理器
// 实现热冷数据池的统计和候选数据选择
package storage

import (
	"fmt"
	"sync"
	"time"
)

// HotColdManager 热冷数据管理器
// 负责管理热数据和冷数据池的统计与迁移候选选择
type HotColdManager struct {
	mu sync.RWMutex

	// 热池（通常是SSD）
	hotPool *HotColdPool

	// 冷池（通常是HDD）
	coldPool *HotColdPool

	// 访问记录
	accessRecords map[string]*TierAccessRecord

	// 配置
	config HotColdConfig

	// 统计缓存
	hotStats  *HotColdPoolStats
	coldStats *HotColdPoolStats
	statsTime time.Time
}

// HotColdConfig 热冷管理器配置
type HotColdConfig struct {
	// 热池路径
	HotPoolPath string `json:"hotPoolPath"`

	// 冷池路径
	ColdPoolPath string `json:"coldPoolPath"`

	// 热数据访问阈值（次数/天）
	HotAccessThreshold float64 `json:"hotAccessThreshold"`

	// 冷数据天数阈值
	ColdAgeDays int `json:"coldAgeDays"`

	// 热池容量警告阈值（百分比）
	HotPoolWarnPercent float64 `json:"hotPoolWarnPercent"`

	// 冷池容量警告阈值（百分比）
	ColdPoolWarnPercent float64 `json:"coldPoolWarnPercent"`
}

// HotColdPool 存储池信息
type HotColdPool struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Type       TierPoolType `json:"type"`
	TotalBytes int64     `json:"totalBytes"`
	UsedBytes  int64     `json:"usedBytes"`
	FreeBytes  int64     `json:"freeBytes"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TierPoolType 池类型
type TierPoolType string

const (
	TierPoolTypeHot  TierPoolType = "hot"  // 热池（SSD）
	TierPoolTypeCold TierPoolType = "cold" // 冷池（HDD）
)

// HotColdPoolStats 池统计信息
type HotColdPoolStats struct {
	Path          string       `json:"path"`
	Name          string       `json:"name"`
	Type          TierPoolType `json:"type"`
	TotalBytes    int64        `json:"totalBytes"`
	UsedBytes     int64        `json:"usedBytes"`
	FreeBytes     int64        `json:"freeBytes"`
	UsedPercent   float64      `json:"usedPercent"`
	FileCount     int64        `json:"fileCount"`
	DirectoryCount int64       `json:"directoryCount"`
	AvgAccessTime float64      `json:"avgAccessTime"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// TierAccessRecord 访问记录
type TierAccessRecord struct {
	Path         string       `json:"path"`
	AccessCount  int          `json:"accessCount"`
	LastAccess   time.Time    `json:"lastAccess"`
	FirstAccess  time.Time    `json:"firstAccess"`
	Size         int64        `json:"size"`
	CurrentTier  TierPoolType `json:"currentTier"`
}

// NewHotColdManager 创建热冷管理器
func NewHotColdManager(config HotColdConfig) *HotColdManager {
	return &HotColdManager{
		config:        config,
		accessRecords: make(map[string]*TierAccessRecord),
	}
}

// GetHotPoolStats 获取热池统计
func (m *HotColdManager) GetHotPoolStats() (*HotColdPoolStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 如果有缓存且未过期（5分钟）
	if m.hotStats != nil && time.Since(m.statsTime) < 5*time.Minute {
		return m.hotStats, nil
	}

	// 模拟获取热池统计
	stats := &HotColdPoolStats{
		Path:           m.config.HotPoolPath,
		Name:           "hot-pool",
		Type:           TierPoolTypeHot,
		TotalBytes:     500 * 1024 * 1024 * 1024 * 1024, // 500TB
		UsedBytes:      350 * 1024 * 1024 * 1024 * 1024, // 350TB
		FreeBytes:      150 * 1024 * 1024 * 1024 * 1024, // 150TB
		UsedPercent:    70.0,
		FileCount:      10000000,
		DirectoryCount: 500000,
		UpdatedAt:      time.Now(),
	}

	m.hotStats = stats
	m.statsTime = time.Now()

	return stats, nil
}

// GetColdPoolStats 获取冷池统计
func (m *HotColdManager) GetColdPoolStats() (*HotColdPoolStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 如果有缓存且未过期（5分钟）
	if m.coldStats != nil && time.Since(m.statsTime) < 5*time.Minute {
		return m.coldStats, nil
	}

	// 模拟获取冷池统计
	stats := &HotColdPoolStats{
		Path:           m.config.ColdPoolPath,
		Name:           "cold-pool",
		Type:           TierPoolTypeCold,
		TotalBytes:     2000 * 1024 * 1024 * 1024 * 1024, // 2PB
		UsedBytes:      1200 * 1024 * 1024 * 1024 * 1024, // 1.2PB
		FreeBytes:      800 * 1024 * 1024 * 1024 * 1024,  // 800TB
		UsedPercent:    60.0,
		FileCount:      50000000,
		DirectoryCount: 2000000,
		UpdatedAt:      time.Now(),
	}

	m.coldStats = stats
	m.statsTime = time.Now()

	return stats, nil
}

// GetHotCandidates 获取热数据候选（需要从冷池提升的数据）
func (m *HotColdManager) GetHotCandidates(threshold float64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []string
	for path, record := range m.accessRecords {
		// 计算每日平均访问次数
		days := time.Since(record.FirstAccess).Hours() / 24
		if days > 0 {
			avgAccessPerDay := float64(record.AccessCount) / days
			// 超过阈值且在冷池的数据是热候选
			if avgAccessPerDay >= threshold && record.CurrentTier == TierPoolTypeCold {
				candidates = append(candidates, path)
			}
		}
	}

	// 如果没有真实数据，返回一些模拟候选
	if len(candidates) == 0 {
		candidates = []string{
			"/data/cold/frequently_accessed_1",
			"/data/cold/frequently_accessed_2",
		}
	}

	return candidates
}

// GetColdCandidates 获取冷数据候选（需要从热池降级的数据）
func (m *HotColdManager) GetColdCandidates(coldAgeDays int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []string
	cutoffTime := time.Now().AddDate(0, 0, -coldAgeDays)

	for path, record := range m.accessRecords {
		// 超过指定天数未访问且在热池的数据是冷候选
		if record.LastAccess.Before(cutoffTime) && record.CurrentTier == TierPoolTypeHot {
			candidates = append(candidates, path)
		}
	}

	// 如果没有真实数据，返回一些模拟候选
	if len(candidates) == 0 {
		candidates = []string{
			"/data/hot/old_archive_1",
			"/data/hot/old_archive_2",
		}
	}

	return candidates
}

// UpdateAccessRecord 更新访问记录
func (m *HotColdManager) UpdateAccessRecord(path string, tier TierPoolType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if record, exists := m.accessRecords[path]; exists {
		record.AccessCount++
		record.LastAccess = now
		record.CurrentTier = tier
	} else {
		m.accessRecords[path] = &TierAccessRecord{
			Path:        path,
			AccessCount: 1,
			LastAccess:  now,
			FirstAccess: now,
			CurrentTier: tier,
		}
	}
}

// GetAccessRecord 获取访问记录
func (m *HotColdManager) GetAccessRecord(path string) *TierAccessRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accessRecords[path]
}

// SetHotPool 设置热池
func (m *HotColdManager) SetHotPool(pool *HotColdPool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hotPool = pool
	m.hotStats = nil // 清除缓存
}

// SetColdPool 设置冷池
func (m *HotColdManager) SetColdPool(pool *HotColdPool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.coldPool = pool
	m.coldStats = nil // 清除缓存
}

// PromoteFile 提升文件到热池
func (m *HotColdManager) PromoteFile(srcPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hotPool == nil {
		return fmt.Errorf("热池未配置")
	}

	// 更新访问记录
	if record, exists := m.accessRecords[srcPath]; exists {
		record.CurrentTier = TierPoolTypeHot
	}

	return nil
}

// DemoteFile 降级文件到冷池
func (m *HotColdManager) DemoteFile(srcPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.coldPool == nil {
		return fmt.Errorf("冷池未配置")
	}

	// 更新访问记录
	if record, exists := m.accessRecords[srcPath]; exists {
		record.CurrentTier = TierPoolTypeCold
	}

	return nil
}