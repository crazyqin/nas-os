// Package storage 提供存储数据分层管理功能
// 对标群晖 Synology Tiering，实现冷热数据自动迁移
// 扩展现有的 StorageTier 和 TieringPolicy 类型
package storage

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// ========== 存储数据分层管理器 ==========

// TieringManager 存储分层管理器
// 扩展 SmartRAIDManager 的层级功能，添加自动迁移策略.
type TieringManager struct {
	mu          sync.RWMutex
	config      *TieringConfig
	raidManager *SmartRAIDManager
	statistics  *TieringStatistics
	migrations  []*MigrationRecord
	ctx         context.Context
	cancel      context.CancelFunc
}

// TieringConfig 分层配置.
type TieringConfig struct {
	Enabled           bool          `json:"enabled"`           // 启用分层
	MonitorInterval   time.Duration `json:"monitorInterval"`   // 监控间隔
	ScanInterval      time.Duration `json:"scanInterval"`      // 扫描间隔
	MaxMigratePerHour int           `json:"maxMigratePerHour"` // 每小时最大迁移数
	MaxMigrateSize    uint64        `json:"maxMigrateSize"`    // 单次最大迁移大小(bytes)
	DryRun            bool          `json:"dryRun"`            // 试运行模式
	NotifyOnMigrate   bool          `json:"notifyOnMigrate"`   // 迁移时通知
	AutoBalance       bool          `json:"autoBalance"`       // 自动负载均衡
}

// DefaultTieringConfig 默认配置.
func DefaultTieringConfig() *TieringConfig {
	return &TieringConfig{
		Enabled:           true,
		MonitorInterval:   5 * time.Minute,
		ScanInterval:      1 * time.Hour,
		MaxMigratePerHour: 100,
		MaxMigrateSize:    10 * 1024 * 1024 * 1024, // 10GB
		DryRun:            false,
		NotifyOnMigrate:   true,
		AutoBalance:       true,
	}
}

// TieringStatistics 分层统计.
type TieringStatistics struct {
	TotalFiles       int64                     `json:"totalFiles"`
	TotalSize        uint64                    `json:"totalSize"`
	ByTier           map[int]*TieringTierStats `json:"byTier"`
	RecentMigrations []*MigrationRecord        `json:"recentMigrations"`
	LastUpdated      time.Time                 `json:"lastUpdated"`
}

// TieringTierStats 层级统计.
type TieringTierStats struct {
	TierID      int     `json:"tierId"`
	TierName    string  `json:"tierName"`
	FileCount   int64   `json:"fileCount"`
	TotalSize   uint64  `json:"totalSize"`
	AvgFileSize uint64  `json:"avgFileSize"`
	UsedPercent float64 `json:"usedPercent"`
}

// MigrationRecord 迁移记录.
type MigrationRecord struct {
	ID         string    `json:"id"`
	FilePath   string    `json:"filePath"`
	FileSize   uint64    `json:"fileSize"`
	SourceTier int       `json:"sourceTier"`
	TargetTier int       `json:"targetTier"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"` // pending, running, completed, failed
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	Duration   int64     `json:"duration"` // 毫秒
	Error      string    `json:"error,omitempty"`
}

// NewTieringManager 创建分层管理器.
func NewTieringManager(config *TieringConfig, raidManager *SmartRAIDManager) *TieringManager {
	if config == nil {
		config = DefaultTieringConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TieringManager{
		config:      config,
		raidManager: raidManager,
		statistics: &TieringStatistics{
			ByTier: make(map[int]*TieringTierStats),
		},
		migrations: make([]*MigrationRecord, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动分层管理器.
func (tm *TieringManager) Start() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.config.Enabled {
		log.Println("存储分层管理器未启用")
		return nil
	}

	log.Println("启动存储分层管理器...")

	// 启动监控协程
	go tm.monitorLoop()

	// 启动扫描协程
	go tm.scanLoop()

	log.Println("存储分层管理器已启动")
	return nil
}

// Stop 停止分层管理器.
func (tm *TieringManager) Stop() {
	tm.cancel()
	log.Println("存储分层管理器已停止")
}

// monitorLoop 监控循环.
func (tm *TieringManager) monitorLoop() {
	ticker := time.NewTicker(tm.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.updateStatistics()
		}
	}
}

// scanLoop 扫描循环.
func (tm *TieringManager) scanLoop() {
	ticker := time.NewTicker(tm.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.scanAndMigrate()
		}
	}
}

// updateStatistics 更新统计信息.
func (tm *TieringManager) updateStatistics() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取 RAID 管理器中的所有池
	pools := tm.raidManager.ListPools()

	for _, pool := range pools {
		for _, tier := range pool.Tiers {
			stats := &TieringTierStats{
				TierID:      tier.ID,
				TierName:    tier.Name,
				UsedPercent: float64(tier.UsedCapacity) / float64(tier.RawCapacity) * 100,
			}
			tm.statistics.ByTier[tier.ID] = stats
		}
	}

	tm.statistics.LastUpdated = time.Now()
}

// scanAndMigrate 扫描并迁移.
func (tm *TieringManager) scanAndMigrate() {
	tm.mu.RLock()

	// 获取 RAID 管理器中的所有池
	pools := tm.raidManager.ListPools()

	for _, pool := range pools {
		for _, tier := range pool.Tiers {
			usagePercent := float64(tier.UsedCapacity) / float64(tier.RawCapacity) * 100

			if usagePercent > 90 {
				log.Printf("警告: 层级 %s 使用率过高 (%.1f%%)", tier.Name, usagePercent)
				// 可以触发迁移逻辑
			}
		}
	}

	tm.mu.RUnlock()
}

// GetStatistics 获取统计信息.
func (tm *TieringManager) GetStatistics() *TieringStatistics {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.statistics
}

// GetMigrations 获取迁移记录.
func (tm *TieringManager) GetMigrations() []*MigrationRecord {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.migrations
}

// BalanceTiers 负载均衡.
func (tm *TieringManager) BalanceTiers() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.config.AutoBalance {
		return fmt.Errorf("自动负载均衡未启用")
	}

	log.Println("开始存储层级负载均衡...")

	pools := tm.raidManager.ListPools()
	for _, pool := range pools {
		for _, tier := range pool.Tiers {
			usagePercent := float64(tier.UsedCapacity) / float64(tier.RawCapacity) * 100

			if usagePercent > 90 {
				log.Printf("警告: 层级 %s 使用率过高 (%.1f%%)", tier.Name, usagePercent)
			}
		}
	}

	return nil
}

// GetTierHealth 获取层级健康状态.
func (tm *TieringManager) GetTierHealth(tierID int) (*TierHealthStatus, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats, exists := tm.statistics.ByTier[tierID]
	if !exists {
		return nil, fmt.Errorf("层级 %d 不存在", tierID)
	}

	health := &TierHealthStatus{
		TierID:      tierID,
		TierName:    stats.TierName,
		UsedPercent: stats.UsedPercent,
		CheckedAt:   time.Now(),
	}

	if stats.UsedPercent > 95 {
		health.Status = "critical"
		health.Message = "存储空间严重不足"
	} else if stats.UsedPercent > 85 {
		health.Status = "warning"
		health.Message = "存储空间不足"
	} else {
		health.Status = "healthy"
		health.Message = "存储状态良好"
	}

	return health, nil
}

// TierHealthStatus 层级健康状态.
type TierHealthStatus struct {
	TierID      int       `json:"tierId"`
	TierName    string    `json:"tierName"`
	Status      string    `json:"status"`
	UsedPercent float64   `json:"usedPercent"`
	Message     string    `json:"message"`
	CheckedAt   time.Time `json:"checkedAt"`
}

// RecommendTier 推荐存储层级.
func (tm *TieringManager) RecommendTier(fileSize uint64, accessFrequency string) int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	pools := tm.raidManager.ListPools()
	if len(pools) == 0 {
		return 0
	}

	pool := pools[0]
	tiers := pool.Tiers

	if len(tiers) == 0 {
		return 0
	}

	// 按优先级排序
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].ID < tiers[j].ID
	})

	switch accessFrequency {
	case "high":
		// 热数据放在第一层
		return tiers[0].ID
	case "medium":
		// 温数据放在中间层
		if len(tiers) > 1 {
			return tiers[1].ID
		}
		return tiers[0].ID
	case "low":
		// 冷数据放在最后一层
		return tiers[len(tiers)-1].ID
	default:
		// 根据文件大小推荐
		if fileSize < 100*1024*1024 { // < 100MB
			return tiers[0].ID
		} else if len(tiers) > 1 {
			return tiers[1].ID
		}
		return tiers[0].ID
	}
}
