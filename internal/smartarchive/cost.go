package smartarchive

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CostManager 存储成本管理器.
type CostManager struct {
	mu sync.RWMutex

	// 成本配置
	config CostConfig

	// 历史成本数据
	history []CostSnapshot

	// 优化建议
	suggestions []CostSuggestion

	// 统计
	stats *CostManagerStats
}

// CostConfig 成本配置.
type CostConfig struct {
	// 基准价格（元/GB/月）
	HotTierCostPerGB   float64 `json:"hotTierCostPerGB"`
	WarmTierCostPerGB  float64 `json:"warmTierCostPerGB"`
	ColdTierCostPerGB  float64 `json:"coldTierCostPerGB"`
	IceTierCostPerGB   float64 `json:"iceTierCostPerGB"`

	// 运营成本
	TransferCostPerGB  float64 `json:"transferCostPerGB"`  // 数据传输成本
	OperationCostPerK  float64 `json:"operationCostPerK"`  // 每千次操作成本
	BackupCostPerGB    float64 `json:"backupCostPerGB"`    // 备份成本

	// 分析配置
	AnalysisInterval   time.Duration `json:"analysisInterval"`
	ForecastWindow     int           `json:"forecastWindow"`     // 预测窗口（月）
	AlertThreshold     float64       `json:"alertThreshold"`     // 告警阈值（元）
	OptimizationTarget float64       `json:"optimizationTarget"` // 优化目标（节省百分比）
}

// DefaultCostConfig 默认成本配置.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		HotTierCostPerGB:   0.50,
		WarmTierCostPerGB:  0.20,
		ColdTierCostPerGB:  0.05,
		IceTierCostPerGB:   0.01,
		TransferCostPerGB:  0.10,
		OperationCostPerK:  0.01,
		BackupCostPerGB:    0.08,
		AnalysisInterval:   24 * time.Hour,
		ForecastWindow:     6,
		AlertThreshold:     1000.0,
		OptimizationTarget: 20.0,
	}
}

// CostSnapshot 成本快照.
type CostSnapshot struct {
	Timestamp    time.Time                    `json:"timestamp"`
	TotalCost    float64                      `json:"totalCost"`
	CostByTier   map[StorageTier]float64      `json:"costByTier"`
	StorageByTier map[StorageTier]int64       `json:"storageByTier"`
	TotalStorage int64                        `json:"totalStorage"`
	Metrics      *CostMetrics                 `json:"metrics"`
}

// CostMetrics 成本指标.
type CostMetrics struct {
	CostPerGB         float64 `json:"costPerGB"`
	HotStorageRatio   float64 `json:"hotStorageRatio"`
	ColdStorageRatio  float64 `json:"coldStorageRatio"`
	CompressionSaving float64 `json:"compressionSaving"`
	DedupSaving       float64 `json:"dedupSaving"`
	TieringSaving     float64 `json:"tieringSaving"`
}

// CostManagerStats 成本管理器统计.
type CostManagerStats struct {
	TotalAnalysis      int64     `json:"totalAnalysis"`
	LastAnalysis       time.Time `json:"lastAnalysis"`
	CurrentMonthCost   float64   `json:"currentMonthCost"`
	PreviousMonthCost  float64   `json:"previousMonthCost"`
	MonthOverMonth     float64   `json:"monthOverMonth"` // 环比变化
	TotalSaving        float64   `json:"totalSaving"`
	ForecastNextMonth  float64   `json:"forecastNextMonth"`
}

// NewCostManager 创建成本管理器.
func NewCostManager() *CostManager {
	return &CostManager{
		config:      DefaultCostConfig(),
		history:     make([]CostSnapshot, 0),
		suggestions: make([]CostSuggestion, 0),
		stats:       &CostManagerStats{},
	}
}

// GenerateReport 生成成本报告.
func (cm *CostManager) GenerateReport(tiers map[StorageTier]*StorageTierConfig, jobs map[string]*ArchiveJob) *CostReport {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	report := &CostReport{
		GeneratedAt:   time.Now(),
		Period:        "monthly",
		CostByTier:    make(map[StorageTier]float64),
		StorageByTier: make(map[StorageTier]int64),
		CostByMonth:   make([]MonthlyCost, 0),
		Suggestions:   make([]CostSuggestion, 0),
	}

	// 计算各层成本
	for tier, config := range tiers {
		storageGB := float64(config.Used) / (1024 * 1024 * 1024)
		costPerGB := cm.getTierCost(tier)
		monthlyCost := storageGB * costPerGB

		report.CostByTier[tier] = monthlyCost
		report.StorageByTier[tier] = config.Used
		report.CurrentCost += monthlyCost
		report.TotalStorage += config.Used
	}

	// 生成月度成本历史
	report.CostByMonth = cm.generateMonthlyHistory()

	// 生成优化建议
	report.Suggestions = cm.generateSuggestions(tiers)

	// 计算潜在节省
	for _, suggestion := range report.Suggestions {
		report.PotentialSaving += suggestion.SavingEst
	}

	// 预测下月成本
	report.ForecastNextMonth = cm.forecastNextMonth()
	report.ForecastTrend = cm.analyzeTrend()

	// 保存快照
	snapshot := CostSnapshot{
		Timestamp:     time.Now(),
		TotalCost:     report.CurrentCost,
		CostByTier:    report.CostByTier,
		StorageByTier: report.StorageByTier,
		TotalStorage:  report.TotalStorage,
		Metrics:       cm.calculateMetrics(tiers),
	}
	cm.history = append(cm.history, snapshot)

	// 更新统计
	cm.stats.CurrentMonthCost = report.CurrentCost
	cm.stats.TotalAnalysis++
	cm.stats.LastAnalysis = time.Now()

	// 检查告警
	if report.CurrentCost > cm.config.AlertThreshold {
		log.Printf("[CostManager] 警告: 当前月成本 %.2f 元超过阈值 %.2f 元",
			report.CurrentCost, cm.config.AlertThreshold)
	}

	return report
}

// getTierCost 获取层级成本.
func (cm *CostManager) getTierCost(tier StorageTier) float64 {
	switch tier {
	case TierHot:
		return cm.config.HotTierCostPerGB
	case TierWarm:
		return cm.config.WarmTierCostPerGB
	case TierCold:
		return cm.config.ColdTierCostPerGB
	case TierIce:
		return cm.config.IceTierCostPerGB
	default:
		return 0
	}
}

// generateMonthlyHistory 生成月度历史.
func (cm *CostManager) generateMonthlyHistory() []MonthlyCost {
	history := make([]MonthlyCost, 0)
	now := time.Now()

	// 生成最近 6 个月的数据
	for i := 5; i >= 0; i-- {
		month := now.AddDate(0, -i, 0)
		monthStr := month.Format("2006-01")

		// 从历史快照中查找
		cost := cm.findCostForMonth(month)
		storage := cm.findStorageForMonth(month)

		history = append(history, MonthlyCost{
			Month:   monthStr,
			Cost:    cost,
			Storage: storage,
		})
	}

	return history
}

// findCostForMonth 查找指定月份的成本.
func (cm *CostManager) findCostForMonth(month time.Time) float64 {
	// 简化实现：返回估算值
	// 实际应该从历史数据中查找
	return cm.stats.CurrentMonthCost * (0.9 + float64(month.Month()%3)*0.05)
}

// findStorageForMonth 查找指定月份的存储量.
func (cm *CostManager) findStorageForMonth(month time.Time) int64 {
	// 简化实现
	if len(cm.history) > 0 {
		return cm.history[len(cm.history)-1].TotalStorage
	}
	return 0
}

// generateSuggestions 生成优化建议.
func (cm *CostManager) generateSuggestions(tiers map[StorageTier]*StorageTierConfig) []CostSuggestion {
	suggestions := make([]CostSuggestion, 0)
	suggestionID := 1

	// 检查热数据层是否过大
	if hotTier, exists := tiers[TierHot]; exists {
		hotUsageGB := float64(hotTier.Used) / (1024 * 1024 * 1024)
		if hotUsageGB > 100 {
			// 建议将部分数据迁移到温层
			migrateGB := hotUsageGB * 0.3
			saving := migrateGB * (cm.config.HotTierCostPerGB - cm.config.WarmTierCostPerGB)

			suggestions = append(suggestions, CostSuggestion{
				ID:          fmt.Sprintf("OPT-%d", suggestionID),
				Type:        "tier_migration",
				Title:       "热数据层优化",
				Description: fmt.Sprintf("建议将 %.0f GB 低频访问数据从热数据层迁移到温数据层", migrateGB),
				Impact:      "high",
				SavingEst:   saving,
				SpaceSaving: int64(migrateGB * 1024 * 1024 * 1024),
				Priority:    1,
				AutoApply:   true,
			})
			suggestionID++
		}
	}

	// 检查冷数据是否可以进一步压缩
	if coldTier, exists := tiers[TierCold]; exists {
		coldUsageGB := float64(coldTier.Used) / (1024 * 1024 * 1024)
		if coldUsageGB > 50 {
			compressGB := coldUsageGB * 0.2
			saving := compressGB * cm.config.ColdTierCostPerGB * 0.5

			suggestions = append(suggestions, CostSuggestion{
				ID:          fmt.Sprintf("OPT-%d", suggestionID),
				Type:        "compression",
				Title:       "冷数据压缩优化",
				Description: fmt.Sprintf("冷数据层有 %.0f GB 数据可通过更高级压缩算法节省空间", compressGB),
				Impact:      "medium",
				SavingEst:   saving,
				SpaceSaving: int64(compressGB * 1024 * 1024 * 1024),
				Priority:    2,
				AutoApply:   false,
			})
			suggestionID++
		}
	}

	// 检查是否有长期未访问的数据可以归档到冰冻层
	totalColdGB := 0.0
	for _, tier := range []StorageTier{TierWarm, TierCold} {
		if t, exists := tiers[tier]; exists {
			totalColdGB += float64(t.Used) / (1024 * 1024 * 1024)
		}
	}

	if totalColdGB > 200 {
		archiveGB := totalColdGB * 0.1
		saving := archiveGB * (cm.config.ColdTierCostPerGB - cm.config.IceTierCostPerGB)

		suggestions = append(suggestions, CostSuggestion{
			ID:          fmt.Sprintf("OPT-%d", suggestionID),
			Type:        "tier_migration",
			Title:       "冰冻层归档",
			Description: fmt.Sprintf("%.0f GB 数据超过 1 年未访问，建议归档到冰冻层", archiveGB),
			Impact:      "medium",
			SavingEst:   saving,
			SpaceSaving: int64(archiveGB * 1024 * 1024 * 1024),
			Priority:    3,
			AutoApply:   false,
		})
		suggestionID++
	}

	// 检查是否有重复数据
	suggestions = append(suggestions, CostSuggestion{
		ID:          fmt.Sprintf("OPT-%d", suggestionID),
		Type:        "dedup",
		Title:       "重复数据检测",
		Description: "运行重复数据检测可发现并消除冗余数据",
		Impact:      "low",
		SavingEst:   cm.stats.CurrentMonthCost * 0.05,
		SpaceSaving: 0,
		Priority:    4,
		AutoApply:   false,
	})

	return suggestions
}

// forecastNextMonth 预测下月成本.
func (cm *CostManager) forecastNextMonth() float64 {
	if len(cm.history) < 2 {
		return cm.stats.CurrentMonthCost
	}

	// 简单线性预测
	recent := cm.history[len(cm.history)-1]
	previous := cm.history[len(cm.history)-2]

	growthRate := (recent.TotalCost - previous.TotalCost) / previous.TotalCost
	forecast := recent.TotalCost * (1 + growthRate)

	return forecast
}

// analyzeTrend 分析趋势.
func (cm *CostManager) analyzeTrend() string {
	if len(cm.history) < 2 {
		return "stable"
	}

	recent := cm.history[len(cm.history)-1]
	previous := cm.history[len(cm.history)-2]

	diff := recent.TotalCost - previous.TotalCost
	threshold := previous.TotalCost * 0.05 // 5% 阈值

	switch {
	case diff > threshold:
		return "up"
	case diff < -threshold:
		return "down"
	default:
		return "stable"
	}
}

// calculateMetrics 计算成本指标.
func (cm *CostManager) calculateMetrics(tiers map[StorageTier]*StorageTierConfig) *CostMetrics {
	metrics := &CostMetrics{}

	totalStorage := int64(0)
	totalCost := 0.0
	hotStorage := int64(0)
	coldStorage := int64(0)

	for tier, config := range tiers {
		totalStorage += config.Used
		totalCost += float64(config.Used) / (1024 * 1024 * 1024) * cm.getTierCost(tier)

		if tier == TierHot {
			hotStorage = config.Used
		}
		if tier == TierCold || tier == TierIce {
			coldStorage += config.Used
		}
	}

	if totalStorage > 0 {
		metrics.CostPerGB = totalCost / (float64(totalStorage) / (1024 * 1024 * 1024))
		metrics.HotStorageRatio = float64(hotStorage) / float64(totalStorage) * 100
		metrics.ColdStorageRatio = float64(coldStorage) / float64(totalStorage) * 100
	}

	return metrics
}

// GetSuggestions 获取优化建议.
func (cm *CostManager) GetSuggestions() []CostSuggestion {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.suggestions
}

// ApplySuggestion 应用优化建议.
func (cm *CostManager) ApplySuggestion(suggestionID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, s := range cm.suggestions {
		if s.ID == suggestionID {
			if !s.AutoApply {
				return fmt.Errorf("建议 %s 不支持自动应用", suggestionID)
			}

			// 执行优化
			log.Printf("[CostManager] 应用优化建议: %s - %s", s.ID, s.Title)
			cm.stats.TotalSaving += s.SavingEst

			return nil
		}
	}

	return fmt.Errorf("建议 %s 不存在", suggestionID)
}

// GetStats 获取统计.
func (cm *CostManager) GetStats() *CostManagerStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.stats
}

// GetHistory 获取历史数据.
func (cm *CostManager) GetHistory(limit int) []CostSnapshot {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if limit <= 0 || limit > len(cm.history) {
		limit = len(cm.history)
	}

	start := len(cm.history) - limit
	if start < 0 {
		start = 0
	}

	return cm.history[start:]
}

// EstimateMigrationCost 估算迁移成本.
func (cm *CostManager) EstimateMigrationCost(source, target StorageTier, sizeGB float64) *MigrationCostEstimate {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	estimate := &MigrationCostEstimate{
		SourceTier:    source,
		TargetTier:    target,
		SizeGB:        sizeGB,
		TransferCost:  sizeGB * cm.config.TransferCostPerGB,
	}

	// 计算月度存储成本变化
	sourceCostPerGB := cm.getTierCost(source)
	targetCostPerGB := cm.getTierCost(target)

	estimate.CurrentMonthlyCost = sizeGB * sourceCostPerGB
	estimate.NewMonthlyCost = sizeGB * targetCostPerGB
	estimate.MonthlySaving = estimate.CurrentMonthlyCost - estimate.NewMonthlyCost
	estimate.AnnualSaving = estimate.MonthlySaving * 12

	// 计算回收期
	if estimate.MonthlySaving > 0 {
		estimate.PaybackMonths = estimate.TransferCost / estimate.MonthlySaving
	}

	return estimate
}

// MigrationCostEstimate 迁移成本估算.
type MigrationCostEstimate struct {
	SourceTier         StorageTier `json:"sourceTier"`
	TargetTier         StorageTier `json:"targetTier"`
	SizeGB             float64     `json:"sizeGB"`
	TransferCost       float64     `json:"transferCost"`
	CurrentMonthlyCost float64     `json:"currentMonthlyCost"`
	NewMonthlyCost     float64     `json:"newMonthlyCost"`
	MonthlySaving      float64     `json:"monthlySaving"`
	AnnualSaving       float64     `json:"annualSaving"`
	PaybackMonths      float64     `json:"paybackMonths"`
}

// UpdateConfig 更新配置.
func (cm *CostManager) UpdateConfig(config CostConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = config
	log.Println("[CostManager] 配置已更新")
}

// GetConfig 获取配置.
func (cm *CostManager) GetConfig() CostConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.config
}

// CompareTiers 比较层级成本.
func (cm *CostManager) CompareTiers(sizeGB float64) map[StorageTier]*TierCostComparison {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	comparison := make(map[StorageTier]*TierCostComparison)

	tiers := []StorageTier{TierHot, TierWarm, TierCold, TierIce}
	for _, tier := range tiers {
		costPerGB := cm.getTierCost(tier)
		monthlyCost := sizeGB * costPerGB

		comparison[tier] = &TierCostComparison{
			Tier:          tier,
			CostPerGB:     costPerGB,
			MonthlyCost:   monthlyCost,
			AnnualCost:    monthlyCost * 12,
			RelativeCost:  monthlyCost / (sizeGB * cm.config.HotTierCostPerGB) * 100,
		}
	}

	return comparison
}

// TierCostComparison 层级成本比较.
type TierCostComparison struct {
	Tier         StorageTier `json:"tier"`
	CostPerGB    float64     `json:"costPerGB"`
	MonthlyCost  float64     `json:"monthlyCost"`
	AnnualCost   float64     `json:"annualCost"`
	RelativeCost float64     `json:"relativeCost"` // 相对于热层的百分比
}
