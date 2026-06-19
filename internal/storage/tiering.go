// Package storage 提供存储数据分层管理功能
// 对标群晖 Synology Tiering，实现冷热数据自动迁移
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

// TierManager 存储分层管理器
type TierManager struct {
	mu          sync.RWMutex
	config      *TierConfig
	tiers       map[string]*StorageTier   // tierID -> tier
	volumes     map[string]*Volume        // volumeID -> volume
	policies    map[string]*TierPolicy    // policyID -> policy
	statistics  *TierStatistics
	ctx         context.Context
	cancel      context.CancelFunc
}

// TierConfig 分层配置
type TierConfig struct {
	Enabled           bool          `json:"enabled"`           // 启用分层
	MonitorInterval   time.Duration `json:"monitorInterval"`   // 监控间隔
	ScanInterval      time.Duration `json:"scanInterval"`      // 扫描间隔
	MaxMigratePerHour int           `json:"maxMigratePerHour"` // 每小时最大迁移数
	MaxMigrateSize    int64         `json:"maxMigrateSize"`    // 单次最大迁移大小(bytes)
	DryRun            bool          `json:"dryRun"`            // 试运行模式
	NotifyOnMigrate   bool          `json:"notifyOnMigrate"`   // 迁移时通知
	AutoBalance       bool          `json:"autoBalance"`       // 自动负载均衡
}

// DefaultTierConfig 默认配置
func DefaultTierConfig() *TierConfig {
	return &TierConfig{
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

// StorageTier 存储层级
type StorageTier struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        TierType  `json:"type"`        // hot, warm, cold, archive
	Priority    int       `json:"priority"`    // 优先级，越小越快
	Devices     []string  `json:"devices"`     // 设备列表
	TotalSpace  int64     `json:"totalSpace"`  // 总空间
	UsedSpace   int64     `json:"usedSpace"`   // 已用空间
	FreeSpace   int64     `json:"freeSpace"`   // 可用空间
	ReadSpeed   int64     `json:"readSpeed"`   // 读取速度 MB/s
	WriteSpeed  int64     `json:"writeSpeed"`  // 写入速度 MB/s
	IOPS        int       `json:"iops"`        // IOPS
	Latency     float64   `json:"latency"`     // 延迟 ms
	Status      string    `json:"status"`      // online, offline, degraded
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TierType 存储层级类型
type TierType string

const (
	TierTypeHot      TierType = "hot"      // 热数据：NVMe SSD
	TierTypeWarm     TierType = "warm"     // 温数据：SATA SSD
	TierTypeCold     TierType = "cold"     // 冷数据：HDD
	TierTypeArchive  TierType = "archive"  // 归档：大容量HDD/磁带
)

// Volume 存储卷
type Volume struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TierID      string    `json:"tierId"`      // 所在层级
	Path        string    `json:"path"`
	TotalSpace  int64     `json:"totalSpace"`
	UsedSpace   int64     `json:"usedSpace"`
	MountPoint  string    `json:"mountPoint"`
	FileSystem  string    `json:"fileSystem"`  // ext4, xfs, btrfs, zfs
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

// TierPolicy 分层策略
type TierPolicy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Enabled     bool             `json:"enabled"`
	Rules       []*TierRule      `json:"rules"`
	Actions     []*TierAction    `json:"actions"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

// TierRule 分层规则
type TierRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        RuleType    `json:"type"`        // age, access, size, type
	Operator    string      `json:"operator"`    // gt, lt, eq, between
	Value       interface{} `json:"value"`
	TargetTier  string      `json:"targetTier"`  // 目标层级
	Priority    int         `json:"priority"`
}

// RuleType 规则类型
type RuleType string

const (
	RuleTypeAge      RuleType = "age"       // 基于文件年龄
	RuleTypeAccess   RuleType = "access"    // 基于访问频率
	RuleTypeSize     RuleType = "size"      // 基于文件大小
	RuleTypeFileType RuleType = "filetype"  // 基于文件类型
	RuleTypePath     RuleType = "path"      // 基于路径
	RuleTypeOwner    RuleType = "owner"     // 基于所有者
)

// TierAction 分层动作
type TierAction struct {
	Type       ActionType `json:"type"`       // migrate, replicate, archive, delete
	SourceTier string     `json:"sourceTier"`
	TargetTier string     `json:"targetTier"`
	Condition  string     `json:"condition"`
}

// ActionType 动作类型
type ActionType string

const (
	ActionTypeMigrate   ActionType = "migrate"   // 迁移
	ActionTypeReplicate ActionType = "replicate"  // 复制
	ActionTypeArchive   ActionType = "archive"    // 归档
	ActionTypeDelete    ActionType = "delete"     // 删除
)

// TierStatistics 分层统计
type TierStatistics struct {
	TotalFiles       int64              `json:"totalFiles"`
	TotalSize        int64              `json:"totalSize"`
	ByTier           map[string]*TierStats `json:"byTier"`
	RecentMigrations []*MigrationRecord `json:"recentMigrations"`
	LastUpdated      time.Time          `json:"lastUpdated"`
}

// TierStats 层级统计
type TierStats struct {
	FileCount    int64   `json:"fileCount"`
	TotalSize    int64   `json:"totalSize"`
	AvgFileSize  int64   `json:"avgFileSize"`
	UsedPercent  float64 `json:"usedPercent"`
	ReadIOPS     int     `json:"readIops"`
	WriteIOPS    int     `json:"writeIops"`
	AvgLatency   float64 `json:"avgLatency"`
}

// MigrationRecord 迁移记录
type MigrationRecord struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"filePath"`
	FileSize    int64     `json:"fileSize"`
	SourceTier  string    `json:"sourceTier"`
	TargetTier  string    `json:"targetTier"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"`      // pending, running, completed, failed
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Duration    int64     `json:"duration"`    // 毫秒
	Error       string    `json:"error,omitempty"`
}

// NewTierManager 创建分层管理器
func NewTierManager(config *TierConfig) *TierManager {
	if config == nil {
		config = DefaultTierConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TierManager{
		config:     config,
		tiers:      make(map[string]*StorageTier),
		volumes:    make(map[string]*Volume),
		policies:   make(map[string]*TierPolicy),
		statistics: &TierStatistics{
			ByTier: make(map[string]*TierStats),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动分层管理器
func (tm *TierManager) Start() error {
	tm.mu.Lock()
	defer tm.tm.mu.Unlock()

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

// Stop 停止分层管理器
func (tm *TierManager) Stop() {
	tm.cancel()
	log.Println("存储分层管理器已停止")
}

// monitorLoop 监控循环
func (tm *TierManager) monitorLoop() {
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

// scanLoop 扫描循环
func (tm *TierManager) scanLoop() {
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

// updateStatistics 更新统计信息
func (tm *TierManager) updateStatistics() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 更新各层级统计
	for tierID, tier := range tm.tiers {
		stats := &TierStats{
			UsedPercent: float64(tier.UsedSpace) / float64(tier.TotalSpace) * 100,
		}
		tm.statistics.ByTier[tierID] = stats
	}

	tm.statistics.LastUpdated = time.Now()
}

// scanAndMigrate 扫描并迁移
func (tm *TierManager) scanAndMigrate() {
	tm.mu.RLock()
	policies := make([]*TierPolicy, 0, len(tm.policies))
	for _, p := range tm.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	tm.mu.RUnlock()

	// 按优先级排序
	sort.Slice(policies, func(i, j int) bool {
		return i < j // 简化排序
	})

	// 执行策略
	for _, policy := range policies {
		tm.executePolicy(policy)
	}
}

// executePolicy 执行策略
func (tm *TierManager) executePolicy(policy *TierPolicy) {
	log.Printf("执行分层策略: %s", policy.Name)

	// 遍历规则
	for _, rule := range policy.Rules {
		// 根据规则类型查找符合条件的文件
		files := tm.findFilesByRule(rule)
		
		// 执行迁移
		for _, file := range files {
			if tm.config.DryRun {
				log.Printf("[试运行] 将迁移文件 %s 从 %s 到 %s", file, rule.TargetTier)
			} else {
				tm.migrateFile(file, rule.TargetTier, policy.Name)
			}
		}
	}
}

// findFilesByRule 根据规则查找文件
func (tm *TierManager) findFilesByRule(rule *TierRule) []string {
	// 简化实现，返回示例文件
	return []string{}
}

// migrateFile 迁移文件
func (tm *TierManager) migrateFile(filePath, targetTier, reason string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 创建迁移记录
	record := &MigrationRecord{
		ID:         fmt.Sprintf("mig_%d", time.Now().UnixNano()),
		FilePath:   filePath,
		TargetTier: targetTier,
		Reason:     reason,
		Status:     "pending",
		StartTime:  time.Now(),
	}

	// 添加到统计
	tm.statistics.RecentMigrations = append(tm.statistics.RecentMigrations, record)

	log.Printf("开始迁移文件 %s 到 %s 层", filePath, targetTier)
	return nil
}

// AddTier 添加存储层级
func (tm *TierManager) AddTier(tier *StorageTier) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tiers[tier.ID]; exists {
		return fmt.Errorf("层级 %s 已存在", tier.ID)
	}

	tier.CreatedAt = time.Now()
	tier.UpdatedAt = time.Now()
	tm.tiers[tier.ID] = tier

	log.Printf("添加存储层级: %s (%s)", tier.Name, tier.Type)
	return nil
}

// RemoveTier 移除存储层级
func (tm *TierManager) RemoveTier(tierID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tier, exists := tm.tiers[tierID]
	if !exists {
		return fmt.Errorf("层级 %s 不存在", tierID)
	}

	if tier.UsedSpace > 0 {
		return fmt.Errorf("层级 %s 仍有数据，请先迁移", tierID)
	}

	delete(tm.tiers, tierID)
	log.Printf("移除存储层级: %s", tierID)
	return nil
}

// AddPolicy 添加分层策略
func (tm *TierManager) AddPolicy(policy *TierPolicy) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	tm.policies[policy.ID] = policy

	log.Printf("添加分层策略: %s", policy.Name)
	return nil
}

// GetStatistics 获取统计信息
func (tm *TierManager) GetStatistics() *TierStatistics {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.statistics
}

// GetTiers 获取所有层级
func (tm *TierManager) GetTiers() []*StorageTier {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tiers := make([]*StorageTier, 0, len(tm.tiers))
	for _, tier := range tm.tiers {
		tiers = append(tiers, tier)
	}

	// 按优先级排序
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].Priority < tiers[j].Priority
	})

	return tiers
}

// GetPolicies 获取所有策略
func (tm *TierManager) GetPolicies() []*TierPolicy {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	policies := make([]*TierPolicy, 0, len(tm.policies))
	for _, policy := range tm.policies {
		policies = append(policies, policy)
	}

	return policies
}

// BalanceTiers 负载均衡
func (tm *TierManager) BalanceTiers() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.config.AutoBalance {
		return fmt.Errorf("自动负载均衡未启用")
	}

	log.Println("开始存储层级负载均衡...")

	// 检查各层级使用率
	for _, tier := range tm.tiers {
		usagePercent := float64(tier.UsedSpace) / float64(tier.TotalSpace) * 100
		
		if usagePercent > 90 {
			log.Printf("警告: 层级 %s 使用率过高 (%.1f%%)", tier.Name, usagePercent)
			// 触发数据迁移
			tm.triggerMigration(tier, "high_usage")
		}
	}

	return nil
}

// triggerMigration 触发迁移
func (tm *TierManager) triggerMigration(tier *StorageTier, reason string) {
	log.Printf("触发层级 %s 的数据迁移，原因: %s", tier.Name, reason)
	// 实际实现中，这里会查找并迁移符合条件的文件
}

// GetTierHealth 获取层级健康状态
func (tm *TierManager) GetTierHealth(tierID string) (*TierHealthStatus, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tier, exists := tm.tiers[tierID]
	if !exists {
		return nil, fmt.Errorf("层级 %s 不存在", tierID)
	}

	health := &TierHealthStatus{
		TierID:      tierID,
		Status:      tier.Status,
		UsedPercent: float64(tier.UsedSpace) / float64(tier.TotalSpace) * 100,
		FreeSpace:   tier.FreeSpace,
		TotalSpace:  tier.TotalSpace,
		CheckedAt:   time.Now(),
	}

	// 判断健康状态
	if health.UsedPercent > 95 {
		health.Status = "critical"
		health.Message = "存储空间严重不足"
	} else if health.UsedPercent > 85 {
		health.Status = "warning"
		health.Message = "存储空间不足"
	} else {
		health.Status = "healthy"
		health.Message = "存储状态良好"
	}

	return health, nil
}

// TierHealthStatus 层级健康状态
type TierHealthStatus struct {
	TierID      string    `json:"tierId"`
	Status      string    `json:"status"`
	UsedPercent float64   `json:"usedPercent"`
	FreeSpace   int64     `json:"freeSpace"`
	TotalSpace  int64     `json:"totalSpace"`
	Message     string    `json:"message"`
	CheckedAt   time.Time `json:"checkedAt"`
}

// RecommendTier 推荐存储层级
func (tm *TierManager) RecommendTier(fileSize int64, accessFrequency string, fileType string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 根据访问频率推荐
	switch accessFrequency {
	case "high":
		return tm.findTierByType(TierTypeHot)
	case "medium":
		return tm.findTierByType(TierTypeWarm)
	case "low":
		return tm.findTierByType(TierTypeCold)
	case "archive":
		return tm.findTierByType(TierTypeArchive)
	default:
		// 根据文件大小推荐
		if fileSize < 100*1024*1024 { // < 100MB
			return tm.findTierByType(TierTypeHot)
		} else if fileSize < 1*1024*1024*1024 { // < 1GB
			return tm.findTierByType(TierTypeWarm)
		} else {
			return tm.findTierByType(TierTypeCold)
		}
	}
}

// findTierByType 根据类型查找层级
func (tm *TierManager) findTierByType(tierType TierType) string {
	for _, tier := range tm.tiers {
		if tier.Type == tierType && tier.Status == "online" {
			return tier.ID
		}
	}
	return ""
}
