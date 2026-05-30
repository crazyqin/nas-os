// Package storagecost - 存储成本分析管理器
// 支持成本计算、成本优化建议、存储使用趋势、预算告警
package storagecost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 存储成本分析管理器
type Manager struct {
	mu sync.RWMutex

	// 存储资产
	assets map[string]*StorageAsset

	// 成本记录
	costRecords map[string]*CostRecord

	// 容量采样
	capacitySamples []CapacitySample

	// 存储池
	storagePools map[string]*StoragePool

	// 预算计划
	budgetPlans map[string]*BudgetPlan

	// 成本趋势
	costTrends []CostTrend

	// 成本告警
	costAlerts []CostAlert

	// 优化建议缓存
	optimizationReport *OptimizationReport

	// 效率报告缓存
	efficiencyReport *EfficiencyReport

	// 对比结果缓存
	comparisonResult *ComparisonResult

	// TCO配置
	tcoConfig TCOConfig

	// 预测配置
	forecastConfig ForecastConfig

	// 告警阈值
	alertThresholds map[string]float64

	// 存储成本配置
	storageCostConfig *StorageCostConfig
}

// NewManager 创建存储成本分析管理器
func NewManager() *Manager {
	return &Manager{
		assets:          make(map[string]*StorageAsset),
		costRecords:     make(map[string]*CostRecord),
		capacitySamples: make([]CapacitySample, 0),
		storagePools:    make(map[string]*StoragePool),
		budgetPlans:     make(map[string]*BudgetPlan),
		costTrends:      make([]CostTrend, 0),
		costAlerts:      make([]CostAlert, 0),
		tcoConfig:       DefaultTCOConfig(),
		forecastConfig:  DefaultForecastConfig(),
		alertThresholds: map[string]float64{
			"utilization": 80.0,
			"budget":      90.0,
		},
	}
}

// NewManagerWithConfig 创建带配置的存储成本分析管理器
func NewManagerWithConfig(config *StorageCostConfig) *Manager {
	mgr := NewManager()
	mgr.storageCostConfig = config
	return mgr
}

// ============================================================
// 存储资产管理
// ============================================================

// CreateAsset 创建存储资产
func (m *Manager) CreateAsset(asset StorageAsset) (*StorageAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if asset.ID == "" {
		asset.ID = uuid.New().String()
	}

	if _, exists := m.assets[asset.ID]; exists {
		return nil, fmt.Errorf("资产 %s 已存在", asset.ID)
	}

	m.assets[asset.ID] = &asset
	log.Printf("[存储成本] 创建资产: %s - %s", asset.ID, asset.Name)
	return &asset, nil
}

// GetAsset 获取存储资产
func (m *Manager) GetAsset(id string) (*StorageAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, exists := m.assets[id]
	if !exists {
		return nil, fmt.Errorf("资产 %s 不存在", id)
	}
	return asset, nil
}

// ListAssets 列出存储资产
func (m *Manager) ListAssets() []*StorageAsset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*StorageAsset
	for _, asset := range m.assets {
		result = append(result, asset)
	}
	return result
}

// ============================================================
// TCO 分析
// ============================================================

// CalculateTCO 计算总拥有成本
func (m *Manager) CalculateTCO(assetID string) (*TCOResult, error) {
	m.mu.RLock()
	asset, exists := m.assets[assetID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("资产 %s 不存在", assetID)
	}
	config := m.tcoConfig
	m.mu.RUnlock()

	years := config.AnalysisPeriodYears
	if years <= 0 {
		years = 5
	}

	// 计算各年度成本
	var annualCosts []AnnualCost
	totalCost := 0.0

	for year := 1; year <= years; year++ {
		// 硬件折旧
		var hardwareCost float64
		switch config.DepreciationMethod {
		case "declining_balance":
			// 余额递减法
			remainingValue := asset.PurchaseCost * math.Pow(1-0.2, float64(year-1))
			hardwareCost = remainingValue * 0.2
		default:
			// 直线法
			hardwareCost = asset.PurchaseCost / float64(years)
		}

		// 电力成本
		powerCost := asset.AnnualPowerKWh * asset.PowerCostPerKWh

		// 冷却成本（电力的30%）
		coolingCost := powerCost * 0.3

		// 机架成本
		rackCost := float64(asset.RackUnits) * asset.RackCostPerUnit

		// 维护成本
		maintenanceCost := asset.PurchaseCost * config.MaintenanceRate

		// 人工成本（估算）
		laborCost := 5000.0 // 固定估算

		annualCost := AnnualCost{
			Year:        year,
			Hardware:    hardwareCost,
			Power:       powerCost,
			Cooling:     coolingCost,
			Rack:        rackCost,
			Maintenance: maintenanceCost,
			Labor:       laborCost,
			Total:       hardwareCost + powerCost + coolingCost + rackCost + maintenanceCost + laborCost,
		}

		annualCosts = append(annualCosts, annualCost)
		totalCost += annualCost.Total
	}

	// 计算成本明细
	costBreakdown := CostBreakdown{
		Hardware:    asset.PurchaseCost,
		Power:       asset.AnnualPowerKWh * asset.PowerCostPerKWh * float64(years),
		Cooling:     asset.AnnualPowerKWh * asset.PowerCostPerKWh * 0.3 * float64(years),
		Rack:        float64(asset.RackUnits) * asset.RackCostPerUnit * float64(years),
		Maintenance: asset.PurchaseCost * config.MaintenanceRate * float64(years),
		Labor:       5000.0 * float64(years),
		Total:       totalCost,
	}

	// 计算每TB成本
	costPerTB := 0.0
	if asset.CapacityTB > 0 {
		costPerTB = totalCost / asset.CapacityTB / float64(years)
	}

	// 计算NPV
	npv := 0.0
	for i, ac := range annualCosts {
		discountFactor := math.Pow(1+config.DiscountRate, float64(i+1))
		npv += ac.Total / discountFactor
	}

	result := &TCOResult{
		AssetID:        assetID,
		AssetName:      asset.Name,
		AnalysisPeriod: years,
		TotalCost:      totalCost,
		CostPerTB:      costPerTB,
		CostBreakdown:  costBreakdown,
		AnnualCosts:    annualCosts,
		NPV:            npv,
		CalculatedAt:   time.Now(),
	}

	log.Printf("[存储成本] TCO分析完成: %s, 总成本: %.2f", assetID, totalCost)
	return result, nil
}

// ============================================================
// 容量预测
// ============================================================

// RecordCapacitySample 记录容量采样
func (m *Manager) RecordCapacitySample(sample CapacitySample) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sample.Timestamp = time.Now()
	m.capacitySamples = append(m.capacitySamples, sample)

	// 保留最近1000个采样点
	if len(m.capacitySamples) > 1000 {
		m.capacitySamples = m.capacitySamples[len(m.capacitySamples)-1000:]
	}

	log.Printf("[存储成本] 记录容量采样: 已用 %.2f TB / %.2f TB", sample.UsedTB, sample.TotalTB)
}

// ForecastCapacity 容量预测
func (m *Manager) ForecastCapacity() (*CapacityForecast, error) {
	m.mu.RLock()
	samples := m.capacitySamples
	config := m.forecastConfig
	m.mu.RUnlock()

	if len(samples) < 2 {
		return nil, fmt.Errorf("采样数据不足，至少需要2个采样点")
	}

	// 获取最新采样
	latest := samples[len(samples)-1]

	// 计算增长趋势
	var growthRate float64
	var avgDailyGrowthGB float64

	if len(samples) >= 2 {
		first := samples[0]
		days := latest.Timestamp.Sub(first.Timestamp).Hours() / 24
		if days > 0 {
			totalGrowth := latest.UsedTB - first.UsedTB
			growthRate = (totalGrowth / first.UsedTB) * 100
			avgDailyGrowthGB = (totalGrowth * 1024) / days
		}
	}

	growthTrend := GrowthTrend{
		Period:           "daily",
		GrowthRate:       growthRate,
		GrowthTB:         latest.UsedTB - samples[0].UsedTB,
		AvgDailyGrowthGB: avgDailyGrowthGB,
	}

	// 计算容量耗尽天数
	runwayDays := 0
	var runwayDate *time.Time
	if avgDailyGrowthGB > 0 {
		remainingGB := (latest.TotalTB - latest.UsedTB) * 1024
		runwayDays = int(remainingGB / avgDailyGrowthGB)
		date := time.Now().AddDate(0, 0, runwayDays)
		runwayDate = &date
	}

	// 生成预测点
	var forecasts []ForecastPoint
	for month := 1; month <= config.ForecastMonths; month++ {
		futureDate := time.Now().AddDate(0, month, 0)
		projectedUsed := latest.UsedTB + (avgDailyGrowthGB*float64(month*30))/1024

		utilization := (projectedUsed / latest.TotalTB) * 100
		confidence := 95.0 - float64(month)*2 // 越远置信度越低

		forecasts = append(forecasts, ForecastPoint{
			Date:            futureDate,
			ProjectedUsedTB: projectedUsed,
			Utilization:     utilization,
			Confidence:      confidence,
		})
	}

	// 生成建议
	recommendation := "当前容量充足"
	if runwayDays > 0 && runwayDays < 90 {
		recommendation = "建议尽快扩容，预计" + fmt.Sprintf("%d", runwayDays) + "天后容量耗尽"
	} else if runwayDays > 0 && runwayDays < 180 {
		recommendation = "建议规划扩容，预计" + fmt.Sprintf("%d", runwayDays) + "天后容量耗尽"
	}

	forecast := &CapacityForecast{
		CurrentUsedTB:      latest.UsedTB,
		CurrentTotalTB:     latest.TotalTB,
		CurrentUtilization: latest.Utilization,
		GrowthTrend:        growthTrend,
		Forecasts:          forecasts,
		RunwayDays:         runwayDays,
		RunwayDate:         runwayDate,
		Confidence:         95.0,
		Recommendation:     recommendation,
		CalculatedAt:       time.Now(),
	}

	log.Printf("[存储成本] 容量预测完成, 剩余天数: %d", runwayDays)
	return forecast, nil
}

// ============================================================
// 成本优化建议
// ============================================================

// GenerateOptimizationReport 生成优化建议报告
func (m *Manager) GenerateOptimizationReport() *OptimizationReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &OptimizationReport{
		GeneratedAt: time.Now(),
	}

	// 模拟分层建议
	report.TieringSuggestions = []TieringSuggestion{
		{
			SourceTier:      "hot",
			TargetTier:      "warm",
			EligibleDataTB:  5.0,
			AnnualSaving:    2000.0,
			SavingPercent:   20.0,
			Confidence:      85.0,
			Rationale:       "30天未访问的数据建议迁移到温存储",
		},
		{
			SourceTier:      "warm",
			TargetTier:      "cold",
			EligibleDataTB:  10.0,
			AnnualSaving:    5000.0,
			SavingPercent:   40.0,
			Confidence:      90.0,
			Rationale:       "90天未访问的数据建议迁移到冷存储",
		},
	}

	// 去重收益
	report.Deduplication = DeduplicationBenefit{
		TotalDataTB:      50.0,
		DedupRatio:       1.5,
		SpaceSavedTB:     16.7,
		CostSavedPerYear: 8000.0,
		DedupEnabled:     false,
	}

	// 压缩收益
	report.Compression = CompressionBenefit{
		TotalDataTB:       50.0,
		CompressionRatio:  2.0,
		SpaceSavedTB:      25.0,
		CostSavedPerYear:  12000.0,
		CompressionEnabled: true,
	}

	// 计算总节省
	report.TotalAnnualSaving = 0
	for _, s := range report.TieringSuggestions {
		report.TotalAnnualSaving += s.AnnualSaving
	}
	report.TotalAnnualSaving += report.Deduplication.CostSavedPerYear
	report.TotalAnnualSaving += report.Compression.CostSavedPerYear

	// 生成建议
	report.TopRecommendations = []Recommendation{
		{
			Priority:    "high",
			Category:    "tiering",
			Title:       "启用自动分层存储",
			Description: "将冷数据自动迁移到低成本存储层",
			SavingEst:   7000.0,
			Effort:      "medium",
		},
		{
			Priority:    "medium",
			Category:    "dedup",
			Title:       "启用数据去重",
			Description: "消除重复数据，节省存储空间",
			SavingEst:   8000.0,
			Effort:      "low",
		},
	}

	m.optimizationReport = report

	log.Printf("[存储成本] 生成优化报告, 预估年节省: %.2f", report.TotalAnnualSaving)
	return report
}

// ============================================================
// 存储效率报告
// ============================================================

// GenerateEfficiencyReport 生成效率报告
func (m *Manager) GenerateEfficiencyReport() *EfficiencyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &EfficiencyReport{
		Overall: EfficiencyMetrics{
			CompressionRatio:   2.0,
			DeduplicationRatio: 1.5,
			ThinProvisionRatio: 1.2,
			SpaceUtilization:   65.0,
			RawCapacityTB:      100.0,
			UsableCapacityTB:   80.0,
			EffectiveCapacityTB: 240.0,
			OverheadPercent:    20.0,
		},
		ByStoragePool: make(map[string]EfficiencyMetrics),
		ByDataType:    make(map[string]EfficiencyMetrics),
		Trend: []EfficiencyTrend{
			{
				Date:             time.Now().AddDate(0, -2, 0),
				CompressionRatio: 1.8,
				DedupRatio:       1.4,
				Utilization:      60.0,
			},
			{
				Date:             time.Now().AddDate(0, -1, 0),
				CompressionRatio: 1.9,
				DedupRatio:       1.45,
				Utilization:      62.0,
			},
			{
				Date:             time.Now(),
				CompressionRatio: 2.0,
				DedupRatio:       1.5,
				Utilization:      65.0,
			},
		},
		Benchmarks: EfficiencyBenchmark{
			IndustryAvgCompression: 2.5,
			IndustryAvgDedup:       2.0,
			IndustryAvgUtilization: 70.0,
			YourRank:               "average",
		},
		GeneratedAt: time.Now(),
	}

	m.efficiencyReport = report

	log.Printf("[存储成本] 生成效率报告")
	return report
}

// ============================================================
// 存储池管理
// ============================================================

// CreateStoragePool 创建存储池
func (m *Manager) CreateStoragePool(pool StoragePool) (*StoragePool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool.ID == "" {
		pool.ID = uuid.New().String()
	}

	if _, exists := m.storagePools[pool.ID]; exists {
		return nil, fmt.Errorf("存储池 %s 已存在", pool.ID)
	}

	pool.Utilization = (pool.UsedTB / pool.TotalTB) * 100
	pool.AvailableTB = pool.TotalTB - pool.UsedTB

	m.storagePools[pool.ID] = &pool
	log.Printf("[存储成本] 创建存储池: %s - %s", pool.ID, pool.Name)
	return &pool, nil
}

// ListStoragePools 列出存储池
func (m *Manager) ListStoragePools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*StoragePool
	for _, pool := range m.storagePools {
		result = append(result, pool)
	}
	return result
}

// ============================================================
// 预算管理
// ============================================================

// CreateBudgetPlan 创建预算计划
func (m *Manager) CreateBudgetPlan(plan BudgetPlan) (*BudgetPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if plan.ID == "" {
		plan.ID = uuid.New().String()
	}

	if _, exists := m.budgetPlans[plan.ID]; exists {
		return nil, fmt.Errorf("预算计划 %s 已存在", plan.ID)
	}

	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()
	plan.Status = "draft"

	// 计算已分配和剩余预算
	allocated := 0.0
	for _, item := range plan.LineItems {
		allocated += item.Amount
	}
	plan.AllocatedBudget = allocated
	plan.RemainingBudget = plan.TotalBudget - allocated

	m.budgetPlans[plan.ID] = &plan
	log.Printf("[存储成本] 创建预算计划: %s - %s", plan.ID, plan.Name)
	return &plan, nil
}

// GetBudgetPlan 获取预算计划
func (m *Manager) GetBudgetPlan(id string) (*BudgetPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, exists := m.budgetPlans[id]
	if !exists {
		return nil, fmt.Errorf("预算计划 %s 不存在", id)
	}
	return plan, nil
}

// ListBudgetPlans 列出预算计划
func (m *Manager) ListBudgetPlans() []*BudgetPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*BudgetPlan
	for _, plan := range m.budgetPlans {
		result = append(result, plan)
	}
	return result
}

// ============================================================
// 成本趋势
// ============================================================

// AddCostTrend 添加成本趋势数据
func (m *Manager) AddCostTrend(trend CostTrend) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.costTrends = append(m.costTrends, trend)

	// 检查预算告警
	if trend.CostPerTB > m.alertThresholds["utilization"] {
		log.Printf("[存储成本] 告警: 成本超过阈值, 当前: %.2f, 阈值: %.2f", trend.CostPerTB, m.alertThresholds["utilization"])
	}
}

// GetCostTrends 获取成本趋势
func (m *Manager) GetCostTrends() []CostTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.costTrends
}

// ============================================================
// 多维对比
// ============================================================

// CompareStorageOptions 对比存储方案
func (m *Manager) CompareStorageOptions(options []StorageOption) *ComparisonResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &ComparisonResult{
		Options:     options,
		Dimensions:  []ComparisonDimension{DimCostPerTB, DimPerformance, DimReliability, DimScalability, DimEfficiency, DimTCO},
		Scores:      make(map[string]map[string]float64),
		GeneratedAt: time.Now(),
	}

	// 计算各维度得分
	for _, option := range options {
		scores := make(map[string]float64)

		// 成本得分（越低越好）
		scores[string(DimCostPerTB)] = 100 - (option.CostPerTBYear / 1000 * 100)

		// 性能得分
		scores[string(DimPerformance)] = float64(option.IOPSCapability) / 10000 * 100

		// 可靠性得分
		scores[string(DimReliability)] = option.Availability

		// 可扩展性得分
		scores[string(DimScalability)] = option.ScalabilityScore

		// 效率得分
		scores[string(DimEfficiency)] = 80.0 // 默认

		// TCO得分
		scores[string(DimTCO)] = 100 - (option.TCO5Year / 100000 * 100)

		result.Scores[option.ID] = scores
	}

	// 计算综合得分和排名
	type optionScore struct {
		id      string
		name    string
		average float64
	}

	var rankings []optionScore
	for _, option := range options {
		total := 0.0
		count := 0
		for _, score := range result.Scores[option.ID] {
			total += score
			count++
		}
		avg := total / float64(count)
		rankings = append(rankings, optionScore{id: option.ID, name: option.Name, average: avg})
	}

	// 排序
	for i := 0; i < len(rankings)-1; i++ {
		for j := i + 1; j < len(rankings); j++ {
			if rankings[j].average > rankings[i].average {
				rankings[i], rankings[j] = rankings[j], rankings[i]
			}
		}
	}

	// 生成排名
	for i, r := range rankings {
		ranking := OptionRanking{
			OptionID:     r.id,
			OptionName:   r.name,
			OverallScore: r.average,
			Rank:         i + 1,
		}
		result.Rankings = append(result.Rankings, ranking)

		if i == 0 {
			result.BestOption = &ranking
		}
	}

	m.comparisonResult = result

	log.Printf("[存储成本] 存储方案对比完成, 方案数: %d", len(options))
	return result
}

// ============================================================
// 配置管理
// ============================================================

// GetTCOConfig 获取TCO配置
func (m *Manager) GetTCOConfig() TCOConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tcoConfig
}

// UpdateTCOConfig 更新TCO配置
func (m *Manager) UpdateTCOConfig(config TCOConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tcoConfig = config
	log.Printf("[存储成本] 更新TCO配置")
}

// GetForecastConfig 获取预测配置
func (m *Manager) GetForecastConfig() ForecastConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.forecastConfig
}

// UpdateForecastConfig 更新预测配置
func (m *Manager) UpdateForecastConfig(config ForecastConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forecastConfig = config
	log.Printf("[存储成本] 更新预测配置")
}

// SetAlertThreshold 设置告警阈值
func (m *Manager) SetAlertThreshold(key string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertThresholds[key] = value
	log.Printf("[存储成本] 设置告警阈值: %s = %.2f", key, value)
}

// ============================================================
// 成本记录管理 (新增)
// ============================================================

// AddCostRecord 添加成本记录
func (m *Manager) AddCostRecord(record CostRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	if _, exists := m.costRecords[record.ID]; exists {
		return fmt.Errorf("成本记录 %s 已存在", record.ID)
	}

	// 计算月度成本
	if record.MonthlyCost == 0 {
		record.MonthlyCost = record.CapacityGB * record.PricePerGB
	}

	m.costRecords[record.ID] = &record
	log.Printf("[存储成本] 添加成本记录: %s - %s", record.ID, record.VolumeName)

	// 检查是否需要生成告警
	m.checkBudgetAlerts()

	return nil
}

// GetCostSummary 获取成本汇总
func (m *Manager) GetCostSummary() (*CostSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &CostSummary{
		CostByType: make(map[string]float64),
		Trend:      make([]CostTrendPoint, 0),
	}

	for _, record := range m.costRecords {
		summary.TotalMonthlyCost += record.MonthlyCost
		summary.TotalCapacity += record.CapacityGB
		summary.UsedCapacity += record.UsedGB
		summary.CostByType[record.StorageType] += record.MonthlyCost
	}

	// 计算每TB成本
	if summary.TotalCapacity > 0 {
		summary.CostPerTB = (summary.TotalMonthlyCost / summary.TotalCapacity) * 1024
	}

	// 生成趋势数据 (最近30天)
	for i := 29; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		dayStr := date.Format("2006-01-02")
		summary.Trend = append(summary.Trend, CostTrendPoint{
			Date:         dayStr,
			Cost:         summary.TotalMonthlyCost / 30,
			Capacity:     summary.TotalCapacity,
			UsedCapacity: summary.UsedCapacity,
		})
	}

	return summary, nil
}

// GetCostTrendByDays 获取成本趋势
func (m *Manager) GetCostTrendByDays(days int) ([]CostTrendPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if days <= 0 {
		days = 30
	}

	trend := make([]CostTrendPoint, 0, days)

	// 计算总成本
	totalCost := 0.0
	totalCapacity := 0.0
	usedCapacity := 0.0
	for _, record := range m.costRecords {
		totalCost += record.MonthlyCost
		totalCapacity += record.CapacityGB
		usedCapacity += record.UsedGB
	}

	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		dayStr := date.Format("2006-01-02")
		trend = append(trend, CostTrendPoint{
			Date:         dayStr,
			Cost:         totalCost / float64(days),
			Capacity:     totalCapacity,
			UsedCapacity: usedCapacity,
		})
	}

	return trend, nil
}

// GetCostAlerts 获取成本告警
func (m *Manager) GetCostAlerts() ([]CostAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.costAlerts, nil
}

// checkBudgetAlerts 检查预算告警 (内部方法，调用时需持有锁)
func (m *Manager) checkBudgetAlerts() {
	if m.storageCostConfig == nil {
		return
	}

	totalCost := 0.0
	for _, record := range m.costRecords {
		totalCost += record.MonthlyCost
	}

	threshold := m.storageCostConfig.AlertThreshold
	if threshold <= 0 {
		threshold = 80.0
	}

	budgetLimit := m.storageCostConfig.BudgetLimit
	if budgetLimit > 0 {
		usagePercent := (totalCost / budgetLimit) * 100
		if usagePercent >= threshold {
			severity := "medium"
			if usagePercent >= 95 {
				severity = "critical"
			} else if usagePercent >= 90 {
				severity = "high"
			}

			alert := CostAlert{
				ID:          uuid.New().String(),
				Threshold:   threshold,
				CurrentCost: totalCost,
				Severity:    severity,
				Message:     fmt.Sprintf("当前成本 %.2f 已达预算 %.2f 的 %.1f%%", totalCost, budgetLimit, usagePercent),
				CreatedAt:   time.Now(),
			}
			m.costAlerts = append(m.costAlerts, alert)
		}
	}
}

// GenerateOptimizationSuggestions 生成优化建议
func (m *Manager) GenerateOptimizationSuggestions() ([]OptimizationSuggestion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suggestions := make([]OptimizationSuggestion, 0)

	// 分析成本记录
	for _, record := range m.costRecords {
		usagePercent := 0.0
		if record.CapacityGB > 0 {
			usagePercent = (record.UsedGB / record.CapacityGB) * 100
		}

		// 低利用率 -> 冷存储建议
		if usagePercent < 30 {
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              uuid.New().String(),
				Type:            "ColdStorage",
				EstimatedSaving: record.MonthlyCost * 0.6,
				Description:     fmt.Sprintf("卷 %s 利用率仅 %.1f%%，建议迁移到冷存储", record.VolumeName, usagePercent),
				Priority:        "high",
			})
		}

		// SSD 大容量 -> 分层建议
		if record.StorageType == "SSD" && record.CapacityGB > 1000 {
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              uuid.New().String(),
				Type:            "Tier",
				EstimatedSaving: record.MonthlyCost * 0.3,
				Description:     fmt.Sprintf("卷 %s 使用 SSD 但容量较大，建议分层存储", record.VolumeName),
				Priority:        "medium",
			})
		}
	}

	// 通用建议
	if len(m.costRecords) > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:              uuid.New().String(),
			Type:            "Dedup",
			EstimatedSaving: 500.0,
			Description:     "启用数据去重可节省存储空间",
			Priority:        "medium",
		})

		suggestions = append(suggestions, OptimizationSuggestion{
			ID:              uuid.New().String(),
			Type:            "Compress",
			EstimatedSaving: 300.0,
			Description:     "启用数据压缩可降低存储成本",
			Priority:        "low",
		})
	}

	return suggestions, nil
}

// EstimateMonthlyCost 估算月度成本
func (m *Manager) EstimateMonthlyCost(storageType string, sizeGB float64) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sizeGB <= 0 {
		return 0, fmt.Errorf("存储大小必须大于0")
	}

	var pricePerGB float64

	// 优先使用配置中的价格
	if m.storageCostConfig != nil {
		switch storageType {
		case "SSD":
			pricePerGB = m.storageCostConfig.DefaultPriceSSD
		case "HDD":
			pricePerGB = m.storageCostConfig.DefaultPriceHDD
		}
	}

	// 默认价格
	if pricePerGB <= 0 {
		switch storageType {
		case "SSD":
			pricePerGB = 0.5
		case "HDD":
			pricePerGB = 0.1
		case "NVMe":
			pricePerGB = 1.0
		case "Cloud":
			pricePerGB = 0.2
		default:
			return 0, fmt.Errorf("不支持的存储类型: %s", storageType)
		}
	}

	return sizeGB * pricePerGB, nil
}

// SetBudgetAlert 设置预算告警
func (m *Manager) SetBudgetAlert(threshold float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if threshold <= 0 || threshold > 100 {
		return fmt.Errorf("阈值必须在 0-100 之间")
	}

	if m.storageCostConfig == nil {
		m.storageCostConfig = &StorageCostConfig{
			Currency:       "CNY",
			AlertThreshold: threshold,
		}
	} else {
		m.storageCostConfig.AlertThreshold = threshold
	}

	log.Printf("[存储成本] 设置预算告警阈值: %.1f%%", threshold)
	return nil
}

// ExportCostReport 导出成本报告
func (m *Manager) ExportCostReport(format string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch format {
	case "csv":
		return m.exportCSV()
	case "json":
		return m.exportJSON()
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// exportCSV 导出CSV格式
func (m *Manager) exportCSV() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("ID,VolumeID,VolumeName,StorageType,CapacityGB,UsedGB,PricePerGB,MonthlyCost,Provider\n")

	for _, record := range m.costRecords {
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%s,%.2f,%.2f,%.4f,%.2f,%s\n",
			record.ID, record.VolumeID, record.VolumeName, record.StorageType,
			record.CapacityGB, record.UsedGB, record.PricePerGB, record.MonthlyCost, record.Provider))
	}

	return buf.Bytes(), nil
}

// exportJSON 导出JSON格式
func (m *Manager) exportJSON() ([]byte, error) {
	records := make([]*CostRecord, 0, len(m.costRecords))
	for _, record := range m.costRecords {
		records = append(records, record)
	}
	return json.MarshalIndent(records, "", "  ")
}
