// Package costanalysis 提供存储成本分析功能
package costanalysis

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPoolNotFound 存储池不存在.
	ErrPoolNotFound = errors.New("存储池不存在")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrInsufficientData 历史数据不足.
	ErrInsufficientData = errors.New("历史数据不足，至少需要2个数据点")
)

// ========== 存储层级类型 ==========

// StorageTierType 存储介质类型.
type StorageTierType string

const (
	// TierNVMe NVMe固态硬盘.
	TierNVMe StorageTierType = "nvme"
	// TierSSD SATA固态硬盘.
	TierSSD StorageTierType = "ssd"
	// TierHDD 机械硬盘.
	TierHDD StorageTierType = "hdd"
)

// ========== 核心数据结构 ==========

// StoragePool 存储池信息.
type StoragePool struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	TierType      StorageTierType `json:"tier_type"`
	TotalCapacity int64           `json:"total_capacity"` // 字节
	UsedCapacity  int64           `json:"used_capacity"`  // 字节
	// HardwareCost 设备购置成本（元）.
	HardwareCost float64 `json:"hardware_cost"`
	// AnnualPowerCost 年度电力成本（元）.
	AnnualPowerCost float64 `json:"annual_power_cost"`
	// AnnualMaintCost 年度维护成本（元）.
	AnnualMaintCost float64 `json:"annual_maint_cost"`
	// ExpectedLifespanYears 预期使用寿命（年）.
	ExpectedLifespanYears float64   `json:"expected_lifespan_years"`
	CreatedAt             time.Time `json:"created_at"`
}

// CostPerTB 每TB成本分析结果.
type CostPerTB struct {
	PoolID               string          `json:"pool_id"`
	PoolName             string          `json:"pool_name"`
	TierType             StorageTierType `json:"tier_type"`
	TotalCapacityTB      float64         `json:"total_capacity_tb"`
	UsedCapacityTB       float64         `json:"used_capacity_tb"`
	HardwareCostPerTB    float64         `json:"hardware_cost_per_tb"`     // 元/TB
	AnnualPowerCostPerTB float64         `json:"annual_power_cost_per_tb"` // 元/TB/年
	AnnualMaintCostPerTB float64         `json:"annual_maint_cost_per_tb"` // 元/TB/年
	TotalAnnualCostPerTB float64         `json:"total_annual_cost_per_tb"` // 元/TB/年（含折旧）
	MonthlyCostPerTB     float64         `json:"monthly_cost_per_tb"`      // 元/TB/月
	CalculatedAt         time.Time       `json:"calculated_at"`
}

// TierComparison 存储层级成本对比结果.
type TierComparison struct {
	Tiers         []TierCostSummary `json:"tiers"`
	BestValueTier StorageTierType   `json:"best_value_tier"`
	AnalysisNote  string            `json:"analysis_note"`
	ComparedAt    time.Time         `json:"compared_at"`
}

// TierCostSummary 单层级成本摘要.
type TierCostSummary struct {
	TierType          StorageTierType `json:"tier_type"`
	DisplayName       string          `json:"display_name"`
	AvgCostPerTBYear  float64         `json:"avg_cost_per_tb_year"`  // 元/TB/年
	AvgCostPerTBMonth float64         `json:"avg_cost_per_tb_month"` // 元/TB/月
	ReadIOPS          int             `json:"read_iops"`
	WriteIOPS         int             `json:"write_iops"`
	ThroughputMBs     int             `json:"throughput_mbs"`
	Reliability       string          `json:"reliability"` // 高/中/低
	RecommendedUse    string          `json:"recommended_use"`
}

// GrowthDataPoint 历史容量增长数据点.
type GrowthDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	UsedBytes int64     `json:"used_bytes"`
}

// CapacityPlan 容量规划建议.
type CapacityPlan struct {
	PoolID              string               `json:"pool_id"`
	PoolName            string               `json:"pool_name"`
	CurrentCapacityTB   float64              `json:"current_capacity_tb"`
	CurrentUsedTB       float64              `json:"current_used_tb"`
	UsagePercent        float64              `json:"usage_percent"`
	MonthlyGrowthRateGB float64              `json:"monthly_growth_rate_gb"`
	MonthlyGrowthPct    float64              `json:"monthly_growth_pct"`
	DaysUntilFull       float64              `json:"days_until_full"`
	Predictions         []CapacityPrediction `json:"predictions"`
	Recommendations     []string             `json:"recommendations"`
	UrgencyLevel        string               `json:"urgency_level"` // critical/warning/normal
	PlannedAt           time.Time            `json:"planned_at"`
}

// CapacityPrediction 容量预测数据点.
type CapacityPrediction struct {
	Date        time.Time `json:"date"`
	PredictedTB float64   `json:"predicted_tb"`
	UsagePct    float64   `json:"usage_pct"`
}

// ROIAnalysis 云存储 vs 本地存储 ROI分析.
type ROIAnalysis struct {
	LocalStorage        StorageCostBreakdown `json:"local_storage"`
	CloudStorage        CloudCostBreakdown   `json:"cloud_storage"`
	SavingsPerYear      float64              `json:"savings_per_year"`
	SavingsPercent      float64              `json:"savings_percent"`
	RecommendedOption   string               `json:"recommended_option"` // local/cloud/hybrid
	BreakevenMonths     float64              `json:"breakeven_months"`
	AnalysisPeriodYears float64              `json:"analysis_period_years"`
	Assumptions         []string             `json:"assumptions"`
	AnalyzedAt          time.Time            `json:"analyzed_at"`
}

// StorageCostBreakdown 本地存储成本明细.
type StorageCostBreakdown struct {
	HardwareCost     float64 `json:"hardware_cost"`    // 设备购置
	AnnualPower      float64 `json:"annual_power"`     // 年电力
	AnnualMaint      float64 `json:"annual_maint"`     // 年维护
	AnnualBandwidth  float64 `json:"annual_bandwidth"` // 年带宽
	TotalPerYear     float64 `json:"total_per_year"`
	TotalOverPeriod  float64 `json:"total_over_period"`
	CostPerTBPerYear float64 `json:"cost_per_tb_per_year"`
}

// CloudCostBreakdown 云存储成本明细.
type CloudCostBreakdown struct {
	Provider         string  `json:"provider"`
	StoragePerYear   float64 `json:"storage_per_year"`   // 存储费
	BandwidthPerYear float64 `json:"bandwidth_per_year"` // 流量费
	RequestPerYear   float64 `json:"request_per_year"`   // 请求费
	TotalPerYear     float64 `json:"total_per_year"`
	TotalOverPeriod  float64 `json:"total_over_period"`
	CostPerTBPerYear float64 `json:"cost_per_tb_per_year"`
}

// OptimizationSuggestion 成本优化建议.
type OptimizationSuggestion struct {
	ID          string `json:"id"`
	Category    string `json:"category"` // tier_migrate, dedup, archive, capacity, power
	Title       string `json:"title"`
	Description string `json:"description"`
	// PotentialSaving 预估年节省金额（元）.
	PotentialSaving float64 `json:"potential_saving"`
	// Priority 优先级: 1=高 2=中 3=低.
	Priority int    `json:"priority"`
	Action   string `json:"action"` // 建议的具体操作
}

// OptimizationReport 成本优化报告.
type OptimizationReport struct {
	PoolID               string                   `json:"pool_id"`
	TotalPotentialSaving float64                  `json:"total_potential_saving"`
	Suggestions          []OptimizationSuggestion `json:"suggestions"`
	GeneratedAt          time.Time                `json:"generated_at"`
}

// ========== 分析器 ==========

// Analyzer 存储成本分析器.
type Analyzer struct {
	mu    sync.RWMutex
	pools map[string]*StoragePool
	// growthHistory 存储池历史数据.
	growthHistory map[string][]GrowthDataPoint
	config        *AnalysisConfig
}

// AnalysisConfig 分析配置.
type AnalysisConfig struct {
	// CloudStoragePricePerTBMonth 云存储价格（元/TB/月）.
	CloudStoragePricePerTBMonth float64
	// CloudBandwidthPricePerGB 云带宽价格（元/GB）.
	CloudBandwidthPricePerGB float64
	// CloudRequestPricePer10k 云请求价格（元/万次）.
	CloudRequestPricePer10k float64
	// DefaultAnalysisPeriodYears 默认分析周期（年）.
	DefaultAnalysisPeriodYears float64
	// AlertUsageThreshold 告警使用率阈值（0-1）.
	AlertUsageThreshold float64
}

// DefaultAnalysisConfig 返回默认分析配置.
func DefaultAnalysisConfig() *AnalysisConfig {
	return &AnalysisConfig{
		CloudStoragePricePerTBMonth: 200,  // 约¥200/TB/月
		CloudBandwidthPricePerGB:    0.8,  // ¥0.8/GB
		CloudRequestPricePer10k:     0.01, // ¥0.01/万次
		DefaultAnalysisPeriodYears:  3,
		AlertUsageThreshold:         0.85,
	}
}

// DefaultTierProfiles 返回默认层级性能画像.
func DefaultTierProfiles() []TierCostSummary {
	return []TierCostSummary{
		{
			TierType:          TierNVMe,
			DisplayName:       "NVMe SSD",
			AvgCostPerTBYear:  800,
			AvgCostPerTBMonth: 67,
			ReadIOPS:          500000,
			WriteIOPS:         200000,
			ThroughputMBs:     3500,
			Reliability:       "高",
			RecommendedUse:    "数据库、高IO应用、虚拟化",
		},
		{
			TierType:          TierSSD,
			DisplayName:       "SATA SSD",
			AvgCostPerTBYear:  500,
			AvgCostPerTBMonth: 42,
			ReadIOPS:          90000,
			WriteIOPS:         60000,
			ThroughputMBs:     550,
			Reliability:       "高",
			RecommendedUse:    "热数据存储、应用服务器、频繁读写",
		},
		{
			TierType:          TierHDD,
			DisplayName:       "HDD 机械硬盘",
			AvgCostPerTBYear:  120,
			AvgCostPerTBMonth: 10,
			ReadIOPS:          200,
			WriteIOPS:         180,
			ThroughputMBs:     200,
			Reliability:       "中",
			RecommendedUse:    "归档存储、冷数据、大文件备份",
		},
	}
}

// NewAnalyzer 创建成本分析器.
func NewAnalyzer(cfg *AnalysisConfig) *Analyzer {
	if cfg == nil {
		cfg = DefaultAnalysisConfig()
	}
	return &Analyzer{
		pools:         make(map[string]*StoragePool),
		growthHistory: make(map[string][]GrowthDataPoint),
		config:        cfg,
	}
}

// RegisterPool 注册存储池.
func (a *Analyzer) RegisterPool(pool *StoragePool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pools[pool.ID] = pool
}

// RemovePool 移除存储池.
func (a *Analyzer) RemovePool(poolID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pools, poolID)
	delete(a.growthHistory, poolID)
}

// AddGrowthData 添加历史增长数据点.
func (a *Analyzer) AddGrowthData(poolID string, point GrowthDataPoint) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.growthHistory[poolID] = append(a.growthHistory[poolID], point)
}

// GetPool 获取存储池.
func (a *Analyzer) GetPool(poolID string) (*StoragePool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	pool, ok := a.pools[poolID]
	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// ListPools 列出所有存储池.
func (a *Analyzer) ListPools() []*StoragePool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	pools := make([]*StoragePool, 0, len(a.pools))
	for _, p := range a.pools {
		pools = append(pools, p)
	}
	return pools
}

// ========== 核心分析方法 ==========

// CalculateCostPerTB 计算存储池每TB成本.
func (a *Analyzer) CalculateCostPerTB(poolID string) (*CostPerTB, error) {
	a.mu.RLock()
	pool, ok := a.pools[poolID]
	a.mu.RUnlock()

	if !ok {
		return nil, ErrPoolNotFound
	}

	totalTB := float64(pool.TotalCapacity) / (1024 * 1024 * 1024 * 1024)
	usedTB := float64(pool.UsedCapacity) / (1024 * 1024 * 1024 * 1024)
	if totalTB <= 0 {
		return nil, ErrInvalidInput
	}

	lifespan := pool.ExpectedLifespanYears
	if lifespan <= 0 {
		lifespan = 5
	}

	hwCostPerTB := pool.HardwareCost / totalTB
	powerCostPerTB := pool.AnnualPowerCost / totalTB
	maintCostPerTB := pool.AnnualMaintCost / totalTB
	deprecPerTB := hwCostPerTB / lifespan
	totalAnnualPerTB := deprecPerTB + powerCostPerTB + maintCostPerTB

	return &CostPerTB{
		PoolID:               pool.ID,
		PoolName:             pool.Name,
		TierType:             pool.TierType,
		TotalCapacityTB:      totalTB,
		UsedCapacityTB:       usedTB,
		HardwareCostPerTB:    round2(hwCostPerTB),
		AnnualPowerCostPerTB: round2(powerCostPerTB),
		AnnualMaintCostPerTB: round2(maintCostPerTB),
		TotalAnnualCostPerTB: round2(totalAnnualPerTB),
		MonthlyCostPerTB:     round2(totalAnnualPerTB / 12),
		CalculatedAt:         time.Now(),
	}, nil
}

// CompareTiers 对比不同存储层级成本.
func (a *Analyzer) CompareTiers() *TierComparison {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tiers := DefaultTierProfiles()

	// 用已注册存储池的实际数据更新
	tierDataMap := make(map[StorageTierType][]float64)
	for _, pool := range a.pools {
		totalTB := float64(pool.TotalCapacity) / (1024 * 1024 * 1024 * 1024)
		if totalTB <= 0 {
			continue
		}
		lifespan := pool.ExpectedLifespanYears
		if lifespan <= 0 {
			lifespan = 5
		}
		annualCostPerTB := (pool.HardwareCost/totalTB)/lifespan + pool.AnnualPowerCost/totalTB + pool.AnnualMaintCost/totalTB
		tierDataMap[pool.TierType] = append(tierDataMap[pool.TierType], annualCostPerTB)
	}

	// 用实际数据更新平均值
	for i := range tiers {
		if costs, ok := tierDataMap[tiers[i].TierType]; ok && len(costs) > 0 {
			avg := 0.0
			for _, c := range costs {
				avg += c
			}
			avg /= float64(len(costs))
			tiers[i].AvgCostPerTBYear = round2(avg)
			tiers[i].AvgCostPerTBMonth = round2(avg / 12)
		}
	}

	// 找到性价比最高的层级（排除性能需求，仅看成本）
	bestTier := TierHDD
	bestCost := math.MaxFloat64
	for _, t := range tiers {
		if t.AvgCostPerTBYear < bestCost {
			bestCost = t.AvgCostPerTBYear
			bestTier = t.TierType
		}
	}

	return &TierComparison{
		Tiers:         tiers,
		BestValueTier: bestTier,
		AnalysisNote:  "性价比最高的层级（纯成本维度）。实际选择需结合IO性能需求。",
		ComparedAt:    time.Now(),
	}
}

// GenerateCapacityPlan 基于历史增长趋势生成容量规划建议.
func (a *Analyzer) GenerateCapacityPlan(poolID string, predictMonths int) (*CapacityPlan, error) {
	a.mu.RLock()
	pool, ok := a.pools[poolID]
	history := a.growthHistory[poolID]
	a.mu.RUnlock()

	if !ok {
		return nil, ErrPoolNotFound
	}
	if predictMonths <= 0 {
		predictMonths = 12
	}

	totalTB := float64(pool.TotalCapacity) / (1024 * 1024 * 1024 * 1024)
	usedTB := float64(pool.UsedCapacity) / (1024 * 1024 * 1024 * 1024)
	usagePct := 0.0
	if totalTB > 0 {
		usagePct = usedTB / totalTB * 100
	}

	// 计算增长率
	growthRateGB := 0.0
	growthPct := 0.0
	if len(history) >= 2 {
		growthRateGB, growthPct = calculateGrowthRate(history)
	}

	// 计算剩余可用空间
	remainingGB := (totalTB - usedTB) * 1024
	daysUntilFull := math.MaxFloat64
	if growthRateGB > 0 {
		daysUntilFull = remainingGB / growthRateGB * 30 // growthRateGB是月增长率
	}

	// 生成预测
	predictions := make([]CapacityPrediction, 0, predictMonths)
	predictedUsed := usedTB
	for i := 1; i <= predictMonths; i++ {
		date := time.Now().AddDate(0, i, 0)
		if growthPct > 0 {
			predictedUsed *= (1 + growthPct/100)
		} else {
			predictedUsed += growthRateGB / 1024 // GB转TB
		}
		predictedPct := 0.0
		if totalTB > 0 {
			predictedPct = predictedUsed / totalTB * 100
		}
		predictions = append(predictions, CapacityPrediction{
			Date:        date,
			PredictedTB: round2(predictedUsed),
			UsagePct:    round2(predictedPct),
		})
	}

	// 生成建议和紧急程度
	recommendations := make([]string, 0)
	urgency := "normal"

	if daysUntilFull < 90 {
		urgency = "critical"
		recommendations = append(recommendations,
			fmt.Sprintf("存储池将在 %.0f 天内用满，建议立即扩容或迁移数据", daysUntilFull))
	} else if daysUntilFull < 180 {
		urgency = "warning"
		recommendations = append(recommendations,
			fmt.Sprintf("存储池预计 %.0f 天后用满，建议规划扩容方案", daysUntilFull))
	}

	if usagePct > 80 {
		recommendations = append(recommendations, "当前使用率已超过80%，建议清理无用数据或归档冷数据")
	}
	if usagePct > 90 {
		recommendations = append(recommendations, "使用率超过90%，系统性能可能受影响")
	}
	if growthRateGB > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("月均增长 %.1f GB，建议至少预留 %.0f GB 空间",
				growthRateGB, growthRateGB*3))
	}
	if len(history) < 6 {
		recommendations = append(recommendations, "历史数据不足6个月，预测精度有限，建议持续收集数据")
	}

	return &CapacityPlan{
		PoolID:              pool.ID,
		PoolName:            pool.Name,
		CurrentCapacityTB:   round2(totalTB),
		CurrentUsedTB:       round2(usedTB),
		UsagePercent:        round2(usagePct),
		MonthlyGrowthRateGB: round2(growthRateGB),
		MonthlyGrowthPct:    round2(growthPct),
		DaysUntilFull:       round2(daysUntilFull),
		Predictions:         predictions,
		Recommendations:     recommendations,
		UrgencyLevel:        urgency,
		PlannedAt:           time.Now(),
	}, nil
}

// AnalyzeROI 分析云存储 vs 本地存储 ROI.
func (a *Analyzer) AnalyzeROI(poolID string, periodYears float64) (*ROIAnalysis, error) {
	a.mu.RLock()
	pool, ok := a.pools[poolID]
	a.mu.RUnlock()

	if !ok {
		return nil, ErrPoolNotFound
	}

	if periodYears <= 0 {
		periodYears = a.config.DefaultAnalysisPeriodYears
	}

	totalTB := float64(pool.TotalCapacity) / (1024 * 1024 * 1024 * 1024)
	if totalTB <= 0 {
		return nil, ErrInvalidInput
	}

	lifespan := pool.ExpectedLifespanYears
	if lifespan <= 0 {
		lifespan = 5
	}

	// 本地存储成本
	local := StorageCostBreakdown{
		HardwareCost:     pool.HardwareCost,
		AnnualPower:      pool.AnnualPowerCost,
		AnnualMaint:      pool.AnnualMaintCost,
		AnnualBandwidth:  0, // 本地不计带宽
		TotalPerYear:     pool.HardwareCost/lifespan + pool.AnnualPowerCost + pool.AnnualMaintCost,
		CostPerTBPerYear: (pool.HardwareCost/lifespan + pool.AnnualPowerCost + pool.AnnualMaintCost) / totalTB,
	}
	local.TotalOverPeriod = local.TotalPerYear*periodYears + pool.HardwareCost // 含首期购置

	// 云存储成本
	storagePerYear := totalTB * a.config.CloudStoragePricePerTBMonth * 12
	// 估算带宽：假设每月上传/下载约10%容量
	estBandwidthGBPerMonth := totalTB * 1024 * 0.1
	bandwidthPerYear := estBandwidthGBPerMonth * 12 * a.config.CloudBandwidthPricePerGB
	// 估算请求费：假设每月100万次请求
	requestPerYear := 100 * a.config.CloudRequestPricePer10k * 12

	cloud := CloudCostBreakdown{
		Provider:         "主流云存储（参考价）",
		StoragePerYear:   round2(storagePerYear),
		BandwidthPerYear: round2(bandwidthPerYear),
		RequestPerYear:   round2(requestPerYear),
		TotalPerYear:     round2(storagePerYear + bandwidthPerYear + requestPerYear),
		CostPerTBPerYear: round2((storagePerYear + bandwidthPerYear + requestPerYear) / totalTB),
	}
	cloud.TotalOverPeriod = cloud.TotalPerYear * periodYears

	savingsPerYear := cloud.TotalPerYear - local.TotalPerYear
	savingsPct := 0.0
	if cloud.TotalPerYear > 0 {
		savingsPct = savingsPerYear / cloud.TotalPerYear * 100
	}

	recommended := "local"
	breakeven := 0.0
	if local.TotalPerYear < cloud.TotalPerYear {
		recommended = "local"
		breakeven = pool.HardwareCost / (cloud.TotalPerYear - local.TotalPerYear) * 12
	} else {
		recommended = "cloud"
		breakeven = 0
	}

	return &ROIAnalysis{
		LocalStorage:        local,
		CloudStorage:        cloud,
		SavingsPerYear:      round2(savingsPerYear),
		SavingsPercent:      round2(savingsPct),
		RecommendedOption:   recommended,
		BreakevenMonths:     round2(breakeven),
		AnalysisPeriodYears: periodYears,
		Assumptions: []string{
			"云存储价格参考主流云厂商标准存储",
			"带宽估算按月均传输10%容量",
			"请求费估算按月均100万次请求",
			"本地存储使用寿命按厂商规格计算",
			"未计入人力运维成本",
		},
		AnalyzedAt: time.Now(),
	}, nil
}

// GenerateOptimizationReport 生成成本优化建议.
func (a *Analyzer) GenerateOptimizationReport(poolID string) (*OptimizationReport, error) {
	a.mu.RLock()
	pool, ok := a.pools[poolID]
	a.mu.RUnlock()

	if !ok {
		return nil, ErrPoolNotFound
	}

	totalTB := float64(pool.TotalCapacity) / (1024 * 1024 * 1024 * 1024)
	usedTB := float64(pool.UsedCapacity) / (1024 * 1024 * 1024 * 1024)
	usagePct := 0.0
	if totalTB > 0 {
		usagePct = usedTB / totalTB
	}

	suggestions := make([]OptimizationSuggestion, 0)
	idCounter := 0

	nextID := func() string {
		idCounter++
		return fmt.Sprintf("opt-%s-%03d", poolID, idCounter)
	}

	// 1. 存储层级迁移建议
	if pool.TierType == TierNVMe {
		// NVMe成本高，部分数据可迁移到SSD
		potentialMigrate := usedTB * 0.3 // 30%数据可能不需要NVMe
		saving := potentialMigrate * (800 - 500)
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:              nextID(),
			Category:        "tier_migrate",
			Title:           "NVMe → SSD 数据迁移",
			Description:     fmt.Sprintf("约 %.1f TB 非高频访问数据可迁移到SATA SSD层级，节省成本", potentialMigrate),
			PotentialSaving: round2(saving),
			Priority:        2,
			Action:          "分析IO访问模式，将低频数据迁移到SSD层级",
		})
	} else if pool.TierType == TierSSD {
		potentialMigrate := usedTB * 0.4
		saving := potentialMigrate * (500 - 120)
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:              nextID(),
			Category:        "tier_migrate",
			Title:           "SSD → HDD 归档迁移",
			Description:     fmt.Sprintf("约 %.1f TB 冷数据可归档到HDD层级", potentialMigrate),
			PotentialSaving: round2(saving),
			Priority:        3,
			Action:          "设置自动归档策略，超过90天未访问的数据自动迁移到HDD",
		})
	}

	// 2. 数据去重建议
	dedupSaving := usedTB * 0.15 * float64(a.avgCostPerTB(pool.TierType)) // 15%去重率
	suggestions = append(suggestions, OptimizationSuggestion{
		ID:              nextID(),
		Category:        "dedup",
		Title:           "数据去重优化",
		Description:     "启用数据去重功能，预计可节省约15%存储空间",
		PotentialSaving: round2(dedupSaving),
		Priority:        2,
		Action:          "对备份数据和重复文件启用去重",
	})

	// 3. 压缩优化
	compressSaving := usedTB * 0.1 * float64(a.avgCostPerTB(pool.TierType)) // 10%压缩率
	suggestions = append(suggestions, OptimizationSuggestion{
		ID:              nextID(),
		Category:        "dedup",
		Title:           "透明压缩",
		Description:     "启用透明压缩功能，可减少约10%存储占用",
		PotentialSaving: round2(compressSaving),
		Priority:        3,
		Action:          "对文本类、日志类数据启用ZSTD压缩",
	})

	// 4. 容量告警建议
	if usagePct > 0.8 {
		urgencyMsg := "建议扩容"
		if usagePct > 0.9 {
			urgencyMsg = "急需扩容，当前使用率已超过90%"
		}
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:              nextID(),
			Category:        "capacity",
			Title:           "容量扩展",
			Description:     urgencyMsg,
			PotentialSaving: 0,
			Priority:        1,
			Action:          fmt.Sprintf("当前使用率 %.1f%%，建议增加 %.1f TB 容量", usagePct*100, usedTB*0.5),
		})
	}

	// 5. 电力优化（大容量HDD）
	if pool.TierType == TierHDD && totalTB > 50 {
		spindownSaving := pool.AnnualPowerCost * 0.2
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:              nextID(),
			Category:        "power",
			Title:           "磁盘休眠优化",
			Description:     "对不活跃磁盘启用自动休眠功能",
			PotentialSaving: round2(spindownSaving),
			Priority:        3,
			Action:          "配置磁盘空闲15分钟后自动休眠",
		})
	}

	// 6. 生命周期策略
	if usedTB > 10 {
		lifecycleSaving := usedTB * 0.2 * (float64(a.avgCostPerTB(pool.TierType)) - 120)
		if lifecycleSaving > 0 {
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              nextID(),
				Category:        "archive",
				Title:           "数据生命周期管理",
				Description:     "设置数据生命周期策略，自动归档过期数据",
				PotentialSaving: round2(lifecycleSaving),
				Priority:        2,
				Action:          "配置策略：热数据30天→温数据90天→冷数据365天→归档",
			})
		}
	}

	totalSaving := 0.0
	for _, s := range suggestions {
		totalSaving += s.PotentialSaving
	}

	return &OptimizationReport{
		PoolID:               poolID,
		TotalPotentialSaving: round2(totalSaving),
		Suggestions:          suggestions,
		GeneratedAt:          time.Now(),
	}, nil
}

// ========== 辅助函数 ==========

// avgCostPerTB 获取某层级的平均年度成本.
func (a *Analyzer) avgCostPerTB(tier StorageTierType) float64 {
	// 先看注册的存储池
	costs := make([]float64, 0)
	lifespan := 5.0
	for _, pool := range a.pools {
		if pool.TierType != tier {
			continue
		}
		totalTB := float64(pool.TotalCapacity) / (1024 * 1024 * 1024 * 1024)
		if totalTB <= 0 {
			continue
		}
		if pool.ExpectedLifespanYears > 0 {
			lifespan = pool.ExpectedLifespanYears
		}
		cost := (pool.HardwareCost/totalTB)/lifespan + pool.AnnualPowerCost/totalTB + pool.AnnualMaintCost/totalTB
		costs = append(costs, cost)
	}

	if len(costs) > 0 {
		sum := 0.0
		for _, c := range costs {
			sum += c
		}
		return sum / float64(len(costs))
	}

	// 使用默认画像
	for _, t := range DefaultTierProfiles() {
		if t.TierType == tier {
			return t.AvgCostPerTBYear
		}
	}
	return 120 // 默认HDD价格
}

// calculateGrowthRate 基于历史数据计算月均增长率.
func calculateGrowthRate(history []GrowthDataPoint) (monthlyGB float64, monthlyPct float64) {
	if len(history) < 2 {
		return 0, 0
	}

	// 按时间排序（简单冒泡，数据量通常不大）
	sorted := make([]GrowthDataPoint, len(history))
	copy(sorted, history)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Timestamp.After(sorted[j].Timestamp) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// 线性回归计算增长率
	n := float64(len(sorted))
	var sumX, sumY, sumXY, sumX2 float64
	firstTime := sorted[0].Timestamp
	for _, p := range sorted {
		x := p.Timestamp.Sub(firstTime).Hours() / 24 / 30 // 月数
		y := float64(p.UsedBytes) / (1024 * 1024 * 1024)  // GB
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0
	}

	slope := (n*sumXY - sumX*sumY) / denom // GB/月
	avgUsedGB := sumY / n

	monthlyGB = slope
	if avgUsedGB > 0 {
		monthlyPct = slope / avgUsedGB * 100
	}
	if monthlyGB < 0 {
		monthlyGB = 0
		monthlyPct = 0
	}

	return monthlyGB, monthlyPct
}

// round2 保留两位小数.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
