// Package smartdatatiering 智能数据分层模块
// 基于访问频率、数据温度、成本效益的自动数据迁移和分层存储管理
package smartdatatiering

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// StorageTier 存储层级
type StorageTier string

const (
	TierHot    StorageTier = "hot"    // 热存储：SSD/NVMe，高性能
	TierWarm   StorageTier = "warm"   // 温存储：SATA SSD，中等性能
	TierCold   StorageTier = "cold"   // 冷存储：HDD，大容量
	TierArchive StorageTier = "archive" // 归档存储：磁带/云存储，最低成本
)

// DataTemperature 数据温度
type DataTemperature string

const (
	TempHot   DataTemperature = "hot"   // 频繁访问
	TempWarm  DataTemperature = "warm"  // 偶尔访问
	TempCold  DataTemperature = "cold"  // 很少访问
	TempFrozen DataTemperature = "frozen" // 几乎不访问
)

// TieringPolicy 分层策略
type TieringPolicy struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	Enabled            bool             `json:"enabled"`
	TierTransitions    []TierTransition `json:"tier_transitions"`
	MonitorIntervalMin int              `json:"monitor_interval_min"`
	AutoMigrate        bool             `json:"auto_migrate"`
	MaxMigrationsPerDay int             `json:"max_migrations_per_day"`
	CostOptimization   bool             `json:"cost_optimization"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// TierTransition 层级转换规则
type TierTransition struct {
	FromTier         StorageTier     `json:"from_tier"`
	ToTier           StorageTier     `json:"to_tier"`
	Temperature      DataTemperature `json:"temperature"`
	DaysInactive     int             `json:"days_inactive"`
	AccessThreshold  float64         `json:"access_threshold"` // 每日访问次数阈值
	Priority         int             `json:"priority"`
}

// DataItem 数据项
type DataItem struct {
	ID               string          `json:"id"`
	Path             string          `json:"path"`
	Size             int64           `json:"size"`
	CurrentTier      StorageTier     `json:"current_tier"`
	Temperature      DataTemperature `json:"temperature"`
	AccessCount      int64           `json:"access_count"`
	LastAccessed     time.Time       `json:"last_accessed"`
	LastModified     time.Time       `json:"last_modified"`
	CreatedAt        time.Time       `json:"created_at"`
	AccessFrequency  float64         `json:"access_frequency"` // 每日访问频率
	EstimatedCost    float64         `json:"estimated_cost"`
	MigrationHistory []MigrationRecord `json:"migration_history,omitempty"`
}

// MigrationRecord 迁移记录
type MigrationRecord struct {
	FromTier    StorageTier `json:"from_tier"`
	ToTier      StorageTier `json:"to_tier"`
	Timestamp   time.Time   `json:"timestamp"`
	Size        int64       `json:"size"`
	Reason      string      `json:"reason"`
	Duration    int         `json:"duration_seconds"`
	Success     bool        `json:"success"`
}

// TierConfig 层级配置
type TierConfig struct {
	Tier         StorageTier `json:"tier"`
	Type         string      `json:"type"` // ssd, hdd, tape, cloud
	Capacity     int64       `json:"capacity_bytes"`
	Used         int64       `json:"used_bytes"`
	CostPerGB    float64     `json:"cost_per_gb_month"`
	ReadSpeedMB  float64     `json:"read_speed_mbps"`
	WriteSpeedMB float64     `json:"write_speed_mbps"`
	IOPS         int         `json:"iops"`
	Latency      float64     `json:"latency_ms"`
	Available    bool        `json:"available"`
}

// MigrationJob 迁移任务
type MigrationJob struct {
	ID          string      `json:"id"`
	DataItemID  string      `json:"data_item_id"`
	FromTier    StorageTier `json:"from_tier"`
	ToTier      StorageTier `json:"to_tier"`
	Size        int64       `json:"size"`
	Status      string      `json:"status"` // pending, running, completed, failed
	Progress    float64     `json:"progress"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// TieringStats 分层统计
type TieringStats struct {
	TotalItems       int                    `json:"total_items"`
	TotalSize        int64                  `json:"total_size_bytes"`
	TierDistribution map[StorageTier]int    `json:"tier_distribution"`
	TierSizes        map[StorageTier]int64  `json:"tier_sizes"`
	TemperatureDist  map[DataTemperature]int `json:"temperature_distribution"`
	TotalMigrations  int                    `json:"total_migrations"`
	ActiveMigrations int                    `json:"active_migrations"`
	CostSavings      float64                `json:"cost_savings_monthly"`
	OptimizationScore float64               `json:"optimization_score"` // 0-100
	LastOptimization *time.Time             `json:"last_optimization,omitempty"`
}

// SmartDataTiering 智能数据分层管理器
type SmartDataTiering struct {
	mu           sync.RWMutex
	policies     map[string]*TieringPolicy
	dataItems    map[string]*DataItem
	tierConfigs  map[StorageTier]*TierConfig
	migrationJobs []MigrationJob
	config       *TieringManagerConfig
}

// TieringManagerConfig 管理器配置
type TieringManagerConfig struct {
	DefaultPolicyID    string  `json:"default_policy_id"`
	AutoTieringEnabled bool    `json:"auto_tiering_enabled"`
	MonitorIntervalMin int     `json:"monitor_interval_min"`
	MaxConcurrentMigrations int `json:"max_concurrent_migrations"`
	CostThreshold      float64 `json:"cost_threshold"`
	PerformanceWeight  float64 `json:"performance_weight"`
	CostWeight         float64 `json:"cost_weight"`
}

// NewSmartDataTiering 创建智能数据分层管理器
func NewSmartDataTiering(config *TieringManagerConfig) *SmartDataTiering {
	if config == nil {
		config = &TieringManagerConfig{
			AutoTieringEnabled:      true,
			MonitorIntervalMin:      30,
			MaxConcurrentMigrations: 3,
			CostThreshold:           0.1,
			PerformanceWeight:       0.6,
			CostWeight:              0.4,
		}
	}

	sdt := &SmartDataTiering{
		policies:    make(map[string]*TieringPolicy),
		dataItems:   make(map[string]*DataItem),
		tierConfigs: make(map[StorageTier]*TierConfig),
		config:      config,
	}

	// 初始化默认层级配置
	sdt.initDefaultTierConfigs()

	return sdt
}

// initDefaultTierConfigs 初始化默认层级配置
func (sdt *SmartDataTiering) initDefaultTierConfigs() {
	sdt.tierConfigs[TierHot] = &TierConfig{
		Tier:         TierHot,
		Type:         "nvme_ssd",
		Capacity:     1024 * 1024 * 1024 * 1024, // 1TB
		CostPerGB:    0.10,
		ReadSpeedMB:  7000,
		WriteSpeedMB: 5000,
		IOPS:         1000000,
		Latency:      0.02,
		Available:    true,
	}

	sdt.tierConfigs[TierWarm] = &TierConfig{
		Tier:         TierWarm,
		Type:         "sata_ssd",
		Capacity:     4 * 1024 * 1024 * 1024 * 1024, // 4TB
		CostPerGB:    0.05,
		ReadSpeedMB:  560,
		WriteSpeedMB: 530,
		IOPS:         100000,
		Latency:      0.1,
		Available:    true,
	}

	sdt.tierConfigs[TierCold] = &TierConfig{
		Tier:         TierCold,
		Type:         "hdd",
		Capacity:     20 * 1024 * 1024 * 1024 * 1024, // 20TB
		CostPerGB:    0.02,
		ReadSpeedMB:  200,
		WriteSpeedMB: 180,
		IOPS:         200,
		Latency:      5.0,
		Available:    true,
	}

	sdt.tierConfigs[TierArchive] = &TierConfig{
		Tier:         TierArchive,
		Type:         "cloud_tape",
		Capacity:     100 * 1024 * 1024 * 1024 * 1024, // 100TB
		CostPerGB:    0.005,
		ReadSpeedMB:  100,
		WriteSpeedMB: 50,
		IOPS:         10,
		Latency:      100.0,
		Available:    true,
	}
}

// AddDataItem 添加数据项
func (sdt *SmartDataTiering) AddDataItem(item *DataItem) error {
	sdt.mu.Lock()
	defer sdt.mu.Unlock()

	if item.ID == "" {
		return fmt.Errorf("data item ID is required")
	}

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	if item.LastAccessed.IsZero() {
		item.LastAccessed = time.Now()
	}

	// 计算访问频率
	item.AccessFrequency = sdt.calculateAccessFrequency(item)

	// 评估温度
	item.Temperature = sdt.evaluateTemperature(item)

	// 如果未指定层级，根据温度推荐
	if item.CurrentTier == "" {
		item.CurrentTier = sdt.recommendTier(item)
	}

	sdt.dataItems[item.ID] = item
	return nil
}

// CreatePolicy 创建分层策略
func (sdt *SmartDataTiering) CreatePolicy(policy *TieringPolicy) error {
	sdt.mu.Lock()
	defer sdt.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	sdt.policies[policy.ID] = policy
	return nil
}

// AnalyzeAndMigrate 分析并执行迁移
func (sdt *SmartDataTiering) AnalyzeAndMigrate() ([]MigrationJob, error) {
	sdt.mu.Lock()
	defer sdt.mu.Unlock()

	var jobs []MigrationJob

	// 更新所有数据项的温度和频率
	for _, item := range sdt.dataItems {
		item.AccessFrequency = sdt.calculateAccessFrequency(item)
		item.Temperature = sdt.evaluateTemperature(item)
	}

	// 获取默认策略
	policy := sdt.getDefaultPolicy()
	if policy == nil {
		return jobs, fmt.Errorf("no default policy configured")
	}

	// 检查每个数据项是否需要迁移
	for _, item := range sdt.dataItems {
		targetTier := sdt.determineTargetTier(item, policy)
		if targetTier != "" && targetTier != item.CurrentTier {
			// 检查是否可以迁移
			if sdt.canMigrate(item, targetTier) {
				job := MigrationJob{
					ID:         fmt.Sprintf("mig_%s_%d", item.ID, time.Now().UnixNano()),
					DataItemID: item.ID,
					FromTier:   item.CurrentTier,
					ToTier:     targetTier,
					Size:       item.Size,
					Status:     "pending",
					Progress:   0,
				}
				jobs = append(jobs, job)

				// 记录迁移历史
				item.MigrationHistory = append(item.MigrationHistory, MigrationRecord{
					FromTier:  item.CurrentTier,
					ToTier:    targetTier,
					Timestamp: time.Now(),
					Size:      item.Size,
					Reason:    fmt.Sprintf("温度变化: %s -> 优化分层", item.Temperature),
					Success:   true,
				})

				// 更新当前层级
				item.CurrentTier = targetTier
			}
		}
	}

	sdt.migrationJobs = append(sdt.migrationJobs, jobs...)
	return jobs, nil
}

// GetOptimizationReport 获取优化报告
func (sdt *SmartDataTiering) GetOptimizationReport() *TieringStats {
	sdt.mu.RLock()
	defer sdt.mu.RUnlock()

	stats := &TieringStats{
		TierDistribution: make(map[StorageTier]int),
		TierSizes:        make(map[StorageTier]int64),
		TemperatureDist:  make(map[DataTemperature]int),
	}

	for _, item := range sdt.dataItems {
		stats.TotalItems++
		stats.TotalSize += item.Size
		stats.TierDistribution[item.CurrentTier]++
		stats.TierSizes[item.CurrentTier] += item.Size
		stats.TemperatureDist[item.Temperature]++
	}

	for _, job := range sdt.migrationJobs {
		stats.TotalMigrations++
		if job.Status == "running" {
			stats.ActiveMigrations++
		}
	}

	// 计算成本节省
	stats.CostSavings = sdt.calculateCostSavings()

	// 计算优化分数
	stats.OptimizationScore = sdt.calculateOptimizationScore()

	return stats
}

// GetDataItemsByTier 按层级获取数据项
func (sdt *SmartDataTiering) GetDataItemsByTier(tier StorageTier) []DataItem {
	sdt.mu.RLock()
	defer sdt.mu.RUnlock()

	var items []DataItem
	for _, item := range sdt.dataItems {
		if item.CurrentTier == tier {
			items = append(items, *item)
		}
	}

	// 按访问频率排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].AccessFrequency > items[j].AccessFrequency
	})

	return items
}

// GetHotDataItems 获取热数据项
func (sdt *SmartDataTiering) GetHotDataItems() []DataItem {
	return sdt.GetDataItemsByTier(TierHot)
}

// GetColdDataItems 获取冷数据项
func (sdt *SmartDataTiering) GetColdDataItems() []DataItem {
	return sdt.GetDataItemsByTier(TierCold)
}

// EstimateMigrationCost 估算迁移成本
func (sdt *SmartDataTiering) EstimateMigrationCost(itemID string, targetTier StorageTier) (float64, error) {
	sdt.mu.RLock()
	defer sdt.mu.RUnlock()

	item, exists := sdt.dataItems[itemID]
	if !exists {
		return 0, fmt.Errorf("data item not found: %s", itemID)
	}

	targetConfig, exists := sdt.tierConfigs[targetTier]
	if !exists {
		return 0, fmt.Errorf("target tier not configured: %s", targetTier)
	}

	// 计算迁移成本（基于数据大小和网络/IO成本）
	sizeGB := float64(item.Size) / (1024 * 1024 * 1024)
	costPerGB := 0.01 // 迁移成本
迁移成本 := sizeGB * costPerGB

	// 计算存储成本差异
	currentConfig := sdt.tierConfigs[item.CurrentTier]
	costDiff := (currentConfig.CostPerGB - targetConfig.CostPerGB) * sizeGB

	return 迁移成本 + costDiff, nil
}

// MarshalJSON 序列化
func (sdt *SmartDataTiering) MarshalJSON() ([]byte, error) {
	sdt.mu.RLock()
	defer sdt.mu.RUnlock()

	return json.Marshal(struct {
		Policies     map[string]*TieringPolicy    `json:"policies"`
		DataItems    map[string]*DataItem          `json:"data_items"`
		TierConfigs  map[StorageTier]*TierConfig   `json:"tier_configs"`
		MigrationJobs []MigrationJob               `json:"migration_jobs"`
		Config       *TieringManagerConfig         `json:"config"`
	}{
		Policies:      sdt.policies,
		DataItems:     sdt.dataItems,
		TierConfigs:   sdt.tierConfigs,
		MigrationJobs: sdt.migrationJobs,
		Config:        sdt.config,
	})
}

// 内部方法

func (sdt *SmartDataTiering) calculateAccessFrequency(item *DataItem) float64 {
	if item.CreatedAt.IsZero() {
		return 0
	}

	daysSinceCreation := time.Since(item.CreatedAt).Hours() / 24
	if daysSinceCreation == 0 {
		return float64(item.AccessCount)
	}

	return float64(item.AccessCount) / daysSinceCreation
}

func (sdt *SmartDataTiering) evaluateTemperature(item *DataItem) DataTemperature {
	freq := item.AccessFrequency
	daysSinceLastAccess := time.Since(item.LastAccessed).Hours() / 24

	switch {
	case freq >= 10 || daysSinceLastAccess < 1:
		return TempHot
	case freq >= 1 || daysSinceLastAccess < 7:
		return TempWarm
	case freq >= 0.1 || daysSinceLastAccess < 30:
		return TempCold
	default:
		return TempFrozen
	}
}

func (sdt *SmartDataTiering) recommendTier(item *DataItem) StorageTier {
	switch item.Temperature {
	case TempHot:
		return TierHot
	case TempWarm:
		return TierWarm
	case TempCold:
		return TierCold
	case TempFrozen:
		return TierArchive
	default:
		return TierWarm
	}
}

func (sdt *SmartDataTiering) getDefaultPolicy() *TieringPolicy {
	if sdt.config.DefaultPolicyID != "" {
		if policy, exists := sdt.policies[sdt.config.DefaultPolicyID]; exists {
			return policy
		}
	}

	// 返回第一个启用的策略
	for _, policy := range sdt.policies {
		if policy.Enabled {
			return policy
		}
	}

	return nil
}

func (sdt *SmartDataTiering) determineTargetTier(item *DataItem, policy *TieringPolicy) StorageTier {
	for _, transition := range policy.TierTransitions {
		if item.CurrentTier == transition.FromTier {
			// 检查温度匹配
			if item.Temperature == transition.Temperature {
				return transition.ToTier
			}

			// 检查不活跃天数
			daysInactive := int(time.Since(item.LastAccessed).Hours() / 24)
			if daysInactive >= transition.DaysInactive {
				return transition.ToTier
			}

			// 检查访问阈值
			if item.AccessFrequency < transition.AccessThreshold {
				return transition.ToTier
			}
		}
	}

	return "" // 不需要迁移
}

func (sdt *SmartDataTiering) canMigrate(item *DataItem, targetTier StorageTier) bool {
	targetConfig, exists := sdt.tierConfigs[targetTier]
	if !exists || !targetConfig.Available {
		return false
	}

	// 检查目标层级是否有足够空间
	availableSpace := targetConfig.Capacity - targetConfig.Used
	if item.Size > availableSpace {
		return false
	}

	// 更新使用量
	targetConfig.Used += item.Size
	currentConfig := sdt.tierConfigs[item.CurrentTier]
	if currentConfig != nil {
		currentConfig.Used -= item.Size
	}

	return true
}

func (sdt *SmartDataTiering) calculateCostSavings() float64 {
	totalSavings := 0.0

	for _, item := range sdt.dataItems {
		currentConfig := sdt.tierConfigs[item.CurrentTier]
		if currentConfig == nil {
			continue
		}

		sizeGB := float64(item.Size) / (1024 * 1024 * 1024)

		// 计算如果在热存储的成本
		hotConfig := sdt.tierConfigs[TierHot]
		hotCost := hotConfig.CostPerGB * sizeGB

		// 实际成本
		actualCost := currentConfig.CostPerGB * sizeGB

		// 节省的成本
		savings := hotCost - actualCost
		if savings > 0 {
			totalSavings += savings
		}
	}

	return totalSavings
}

func (sdt *SmartDataTiering) calculateOptimizationScore() float64 {
	if len(sdt.dataItems) == 0 {
		return 100.0
	}

	optimizedCount := 0
	for _, item := range sdt.dataItems {
		expectedTier := sdt.recommendTier(item)
		if item.CurrentTier == expectedTier {
			optimizedCount++
		}
	}

	return float64(optimizedCount) / float64(len(sdt.dataItems)) * 100
}

// GenerateDefaultPolicy 生成默认分层策略
func GenerateDefaultPolicy() *TieringPolicy {
	return &TieringPolicy{
		ID:          "default_tiering",
		Name:        "默认智能分层策略",
		Description: "基于数据温度的自动分层存储策略",
		Enabled:     true,
		TierTransitions: []TierTransition{
			{FromTier: TierHot, ToTier: TierWarm, Temperature: TempWarm, DaysInactive: 7, AccessThreshold: 1.0, Priority: 1},
			{FromTier: TierWarm, ToTier: TierCold, Temperature: TempCold, DaysInactive: 30, AccessThreshold: 0.1, Priority: 2},
			{FromTier: TierCold, ToTier: TierArchive, Temperature: TempFrozen, DaysInactive: 90, AccessThreshold: 0.01, Priority: 3},
			{FromTier: TierArchive, ToTier: TierCold, Temperature: TempCold, DaysInactive: 0, AccessThreshold: 0.5, Priority: 4},
			{FromTier: TierCold, ToTier: TierWarm, Temperature: TempWarm, DaysInactive: 0, AccessThreshold: 5.0, Priority: 5},
			{FromTier: TierWarm, ToTier: TierHot, Temperature: TempHot, DaysInactive: 0, AccessThreshold: 20.0, Priority: 6},
		},
		MonitorIntervalMin:   30,
		AutoMigrate:          true,
		MaxMigrationsPerDay:  100,
		CostOptimization:     true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
}
