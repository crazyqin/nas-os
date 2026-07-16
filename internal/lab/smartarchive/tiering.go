package smartarchive

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TierManager 存储分层管理器.
type TierManager struct {
	mu sync.RWMutex

	// 存储层配置
	tiers map[StorageTier]*StorageTierConfig

	// 层级迁移历史
	migrations []TierMigration

	// 配置
	config TierManagerConfig
}

// TierManagerConfig 分层管理器配置.
type TierManagerConfig struct {
	// 自动平衡
	EnableAutoBalance bool          `json:"enableAutoBalance"`
	BalanceThreshold  float64       `json:"balanceThreshold"` // 触发平衡的使用率差异阈值
	BalanceCooldown   time.Duration `json:"balanceCooldown"`

	// 迁移配置
	MaxConcurrentMigrations int           `json:"maxConcurrentMigrations"`
	MigrationBatchSize      int           `json:"migrationBatchSize"`
	MigrationTimeout        time.Duration `json:"migrationTimeout"`

	// 健康检查
	HealthCheckInterval time.Duration `json:"healthCheckInterval"`
	AlertThreshold      float64       `json:"alertThreshold"` // 告警阈值
}

// DefaultTierManagerConfig 默认分层管理器配置.
func DefaultTierManagerConfig() TierManagerConfig {
	return TierManagerConfig{
		EnableAutoBalance:       true,
		BalanceThreshold:        20.0,
		BalanceCooldown:         1 * time.Hour,
		MaxConcurrentMigrations: 3,
		MigrationBatchSize:      100,
		MigrationTimeout:        2 * time.Hour,
		HealthCheckInterval:     5 * time.Minute,
		AlertThreshold:          90.0,
	}
}

// TierMigration 层级迁移记录.
type TierMigration struct {
	ID         string      `json:"id"`
	SourceTier StorageTier `json:"sourceTier"`
	TargetTier StorageTier `json:"targetTier"`
	StartTime  time.Time   `json:"startTime"`
	EndTime    time.Time   `json:"endTime,omitempty"`
	Status     string      `json:"status"`
	FilesMoved int64       `json:"filesMoved"`
	BytesMoved int64       `json:"bytesMoved"`
	Reason     string      `json:"reason"`
	Error      string      `json:"error,omitempty"`
}

// TierHealth 层级健康状态.
type TierHealth struct {
	Tier            StorageTier `json:"tier"`
	Status          string      `json:"status"` // healthy/warning/critical/offline
	UsagePercent    float64     `json:"usagePercent"`
	IOPSUsage       float64     `json:"iopsUsage"`
	ThroughputUsage float64     `json:"throughputUsage"`
	LastCheck       time.Time   `json:"lastCheck"`
	Issues          []string    `json:"issues,omitempty"`
}

// NewTierManager 创建分层管理器.
func NewTierManager() *TierManager {
	return &TierManager{
		tiers:      make(map[StorageTier]*StorageTierConfig),
		migrations: make([]TierMigration, 0),
		config:     DefaultTierManagerConfig(),
	}
}

// RegisterTier 注册存储层.
func (tm *TierManager) RegisterTier(config *StorageTierConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tiers[config.Tier]; exists {
		return fmt.Errorf("存储层 %s 已注册", config.Tier)
	}

	tm.tiers[config.Tier] = config
	log.Printf("[TierManager] 注册存储层: %s (%s)", config.Name, config.Tier)
	return nil
}

// UpdateTier 更新存储层配置.
func (tm *TierManager) UpdateTier(config *StorageTierConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tiers[config.Tier]; !exists {
		return fmt.Errorf("存储层 %s 未注册", config.Tier)
	}

	tm.tiers[config.Tier] = config
	return nil
}

// GetTier 获取存储层配置.
func (tm *TierManager) GetTier(tier StorageTier) (*StorageTierConfig, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	config, exists := tm.tiers[tier]
	if !exists {
		return nil, fmt.Errorf("存储层 %s 不存在", tier)
	}

	return config, nil
}

// ListTiers 列出所有存储层.
func (tm *TierManager) ListTiers() []*StorageTierConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tiers := make([]*StorageTierConfig, 0, len(tm.tiers))
	for _, t := range tm.tiers {
		tiers = append(tiers, t)
	}

	return tiers
}

// GetTierForFile 根据文件特征推荐存储层.
func (tm *TierManager) GetTierForFile(pattern *AccessPattern) StorageTier {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 根据热度评分决定层级
	switch {
	case pattern.HeatScore >= 80:
		return TierHot
	case pattern.HeatScore >= 40:
		return TierWarm
	case pattern.HeatScore >= 10:
		return TierCold
	default:
		return TierIce
	}
}

// CanMigrate 检查是否可以迁移.
func (tm *TierManager) CanMigrate(source, target StorageTier) (bool, string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	sourceConfig, exists := tm.tiers[source]
	if !exists {
		return false, fmt.Sprintf("源存储层 %s 不存在", source)
	}

	targetConfig, exists := tm.tiers[target]
	if !exists {
		return false, fmt.Sprintf("目标存储层 %s 不存在", target)
	}

	if !sourceConfig.Enabled {
		return false, fmt.Sprintf("源存储层 %s 已禁用", source)
	}

	if !targetConfig.Enabled {
		return false, fmt.Sprintf("目标存储层 %s 已禁用", target)
	}

	// 检查目标层空间
	available := targetConfig.Capacity - targetConfig.Used
	if available <= 0 {
		return false, fmt.Sprintf("目标存储层 %s 空间不足", target)
	}

	return true, ""
}

// ExecuteMigration 执行层级迁移.
func (tm *TierManager) ExecuteMigration(source, target StorageTier, files []string, reason string) (*TierMigration, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 检查是否可以迁移
	sourceConfig, exists := tm.tiers[source]
	if !exists {
		return nil, fmt.Errorf("源存储层 %s 不存在", source)
	}

	targetConfig, exists := tm.tiers[target]
	if !exists {
		return nil, fmt.Errorf("目标存储层 %s 不存在", target)
	}

	if !sourceConfig.Enabled || !targetConfig.Enabled {
		return nil, fmt.Errorf("存储层已禁用")
	}

	// 创建迁移记录
	migration := &TierMigration{
		ID:         generateID(),
		SourceTier: source,
		TargetTier: target,
		StartTime:  time.Now(),
		Status:     "running",
		Reason:     reason,
	}

	// 执行迁移（简化实现）
	migration.FilesMoved = int64(len(files))
	migration.Status = "completed"
	now := time.Now()
	migration.EndTime = now

	tm.migrations = append(tm.migrations, *migration)

	log.Printf("[TierManager] 完成迁移: %s -> %s, %d 个文件", source, target, len(files))
	return migration, nil
}

// CheckHealth 检查存储层健康状态.
func (tm *TierManager) CheckHealth() map[StorageTier]*TierHealth {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	healthMap := make(map[StorageTier]*TierHealth)

	for tier, config := range tm.tiers {
		health := &TierHealth{
			Tier:      tier,
			LastCheck: time.Now(),
			Issues:    make([]string, 0),
		}

		// 计算使用率
		if config.Capacity > 0 {
			health.UsagePercent = float64(config.Used) / float64(config.Capacity) * 100
		}

		// 判断状态
		switch {
		case !config.Enabled:
			health.Status = "offline"
		case health.UsagePercent >= tm.config.AlertThreshold:
			health.Status = "critical"
			health.Issues = append(health.Issues, fmt.Sprintf("使用率 %.1f%% 超过告警阈值", health.UsagePercent))
		case health.UsagePercent >= float64(config.Threshold):
			health.Status = "warning"
			health.Issues = append(health.Issues, fmt.Sprintf("使用率 %.1f%% 接近阈值 %d%%", health.UsagePercent, config.Threshold))
		default:
			health.Status = "healthy"
		}

		healthMap[tier] = health
	}

	return healthMap
}

// GetBalanceRecommendations 获取平衡建议.
func (tm *TierManager) GetBalanceRecommendations() []BalanceRecommendation {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	recommendations := make([]BalanceRecommendation, 0)

	// 分析各层使用率
	usageMap := make(map[StorageTier]float64)
	for tier, config := range tm.tiers {
		if config.Capacity > 0 {
			usageMap[tier] = float64(config.Used) / float64(config.Capacity) * 100
		}
	}

	// 检查是否需要平衡
	avgUsage := 0.0
	for _, usage := range usageMap {
		avgUsage += usage
	}
	if len(usageMap) > 0 {
		avgUsage /= float64(len(usageMap))
	}

	for tier, usage := range usageMap {
		diff := usage - avgUsage
		if diff > tm.config.BalanceThreshold {
			// 需要迁移出
			recommendations = append(recommendations, BalanceRecommendation{
				SourceTier: tier,
				TargetTier: tm.findLessUsedTier(usageMap, tier),
				Action:     "migrate_out",
				Reason:     fmt.Sprintf("使用率 %.1f%% 高于平均 %.1f%%", usage, avgUsage),
				Priority:   int(diff),
				EstSaving:  int64(diff * float64(tm.tiers[tier].Capacity) / 100),
			})
		} else if diff < -tm.config.BalanceThreshold {
			// 可以迁入
			recommendations = append(recommendations, BalanceRecommendation{
				SourceTier: tm.findMostUsedTier(usageMap, tier),
				TargetTier: tier,
				Action:     "migrate_in",
				Reason:     fmt.Sprintf("使用率 %.1f%% 低于平均 %.1f%%", usage, avgUsage),
				Priority:   int(-diff),
			})
		}
	}

	return recommendations
}

// BalanceRecommendation 平衡建议.
type BalanceRecommendation struct {
	SourceTier StorageTier `json:"sourceTier"`
	TargetTier StorageTier `json:"targetTier"`
	Action     string      `json:"action"`
	Reason     string      `json:"reason"`
	Priority   int         `json:"priority"`
	EstSaving  int64       `json:"estSaving,omitempty"`
}

// findLessUsedTier 找到使用率最低的层级.
func (tm *TierManager) findLessUsedTier(usageMap map[StorageTier]float64, exclude StorageTier) StorageTier {
	minUsage := 100.0
	minTier := TierIce

	for tier, usage := range usageMap {
		if tier == exclude {
			continue
		}
		if usage < minUsage {
			minUsage = usage
			minTier = tier
		}
	}

	return minTier
}

// findMostUsedTier 找到使用率最高的层级.
func (tm *TierManager) findMostUsedTier(usageMap map[StorageTier]float64, exclude StorageTier) StorageTier {
	maxUsage := 0.0
	maxTier := TierHot

	for tier, usage := range usageMap {
		if tier == exclude {
			continue
		}
		if usage > maxUsage {
			maxUsage = usage
			maxTier = tier
		}
	}

	return maxTier
}

// GetMigrationHistory 获取迁移历史.
func (tm *TierManager) GetMigrationHistory(limit int) []TierMigration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if limit <= 0 || limit > len(tm.migrations) {
		limit = len(tm.migrations)
	}

	// 返回最近的迁移记录
	start := len(tm.migrations) - limit
	if start < 0 {
		start = 0
	}

	return tm.migrations[start:]
}

// GetTierStats 获取层级统计.
func (tm *TierManager) GetTierStats() map[StorageTier]*TierStatsInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := make(map[StorageTier]*TierStatsInfo)

	for tier, config := range tm.tiers {
		info := &TierStatsInfo{
			Tier:      tier,
			Name:      config.Name,
			Capacity:  config.Capacity,
			Used:      config.Used,
			Available: config.Capacity - config.Used,
			CostPerGB: config.CostPerGB,
			Enabled:   config.Enabled,
		}

		if config.Capacity > 0 {
			info.UsagePercent = float64(config.Used) / float64(config.Capacity) * 100
		}

		info.MonthlyCost = float64(config.Used) / (1024 * 1024 * 1024) * config.CostPerGB

		stats[tier] = info
	}

	return stats
}

// TierStatsInfo 层级统计信息.
type TierStatsInfo struct {
	Tier         StorageTier `json:"tier"`
	Name         string      `json:"name"`
	Capacity     int64       `json:"capacity"`
	Used         int64       `json:"used"`
	Available    int64       `json:"available"`
	UsagePercent float64     `json:"usagePercent"`
	CostPerGB    float64     `json:"costPerGB"`
	MonthlyCost  float64     `json:"monthlyCost"`
	Enabled      bool        `json:"enabled"`
}
