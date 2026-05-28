// Package smartpricing - 智能定价分析管理器
// 多云存储成本对比、存储方案推荐、成本优化建议
package smartpricing

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// Manager 智能定价分析管理器
type Manager struct {
	mu sync.RWMutex

	config   SmartPricingConfig
	plans    []StoragePlan
	trends   []CostTrendPoint
	alerts   []BudgetAlert

	// 历史成本数据
	costHistory map[string][]CostTrendPoint // provider -> history
}

// BudgetAlert 预算告警
type BudgetAlert struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	Threshold float64   `json:"threshold"`
	Current   float64   `json:"current"`
	Triggered bool      `json:"triggered"`
	CreatedAt time.Time `json:"created_at"`
}

// NewManager 创建智能定价分析管理器
func NewManager(config SmartPricingConfig) *Manager {
	m := &Manager{
		config:      config,
		plans:       make([]StoragePlan, 0),
		trends:      make([]CostTrendPoint, 0),
		alerts:      make([]BudgetAlert, 0),
		costHistory: make(map[string][]CostTrendPoint),
	}

	// 初始化默认存储方案
	m.initDefaultPlans()

	log.Printf("SmartPricing Manager initialized with %d plans", len(m.plans))
	return m
}

// initDefaultPlans 初始化默认存储方案
func (m *Manager) initDefaultPlans() {
	m.plans = []StoragePlan{
		// AWS S3
		{
			ID:              "aws-s3-standard",
			Name:            "AWS S3 Standard",
			Provider:        ProviderAWSS3,
			Tier:            TierStandard,
			Region:          "us-east-1",
			StoragePriceGB:  0.023,
			RequestPrice1K:  0.0004,
			TransferPriceGB: 0.09,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "AWS S3标准存储，适合频繁访问的数据",
			CreatedAt:       time.Now(),
		},
		{
			ID:              "aws-s3-ia",
			Name:            "AWS S3 Infrequent Access",
			Provider:        ProviderAWSS3,
			Tier:            TierInfrequent,
			Region:          "us-east-1",
			StoragePriceGB:  0.0125,
			RequestPrice1K:  0.001,
			TransferPriceGB: 0.09,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "AWS S3低频访问存储，适合偶尔访问的数据",
			CreatedAt:       time.Now(),
		},
		{
			ID:              "aws-s3-glacier",
			Name:            "AWS S3 Glacier",
			Provider:        ProviderAWSS3,
			Tier:            TierArchive,
			Region:          "us-east-1",
			StoragePriceGB:  0.004,
			RequestPrice1K:  0.03,
			TransferPriceGB: 0.09,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "AWS S3归档存储，适合长期保存的数据",
			CreatedAt:       time.Now(),
		},
		// 阿里云 OSS
		{
			ID:              "aliyun-oss-standard",
			Name:            "阿里云 OSS 标准存储",
			Provider:        ProviderAliyunOSS,
			Tier:            TierStandard,
			Region:          "cn-hangzhou",
			StoragePriceGB:  0.12,
			RequestPrice1K:  0.01,
			TransferPriceGB: 0.50,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "阿里云OSS标准存储，适合频繁访问的数据",
			CreatedAt:       time.Now(),
		},
		{
			ID:              "aliyun-oss-ia",
			Name:            "阿里云 OSS 低频访问存储",
			Provider:        ProviderAliyunOSS,
			Tier:            TierInfrequent,
			Region:          "cn-hangzhou",
			StoragePriceGB:  0.08,
			RequestPrice1K:  0.10,
			TransferPriceGB: 0.50,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "阿里云OSS低频访问存储，适合偶尔访问的数据",
			CreatedAt:       time.Now(),
		},
		{
			ID:              "aliyun-oss-archive",
			Name:            "阿里云 OSS 归档存储",
			Provider:        ProviderAliyunOSS,
			Tier:            TierArchive,
			Region:          "cn-hangzhou",
			StoragePriceGB:  0.033,
			RequestPrice1K:  0.10,
			TransferPriceGB: 0.50,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "阿里云OSS归档存储，适合长期保存的数据",
			CreatedAt:       time.Now(),
		},
		// 腾讯云 COS
		{
			ID:              "tencent-cos-standard",
			Name:            "腾讯云 COS 标准存储",
			Provider:        ProviderTencentCOS,
			Tier:            TierStandard,
			Region:          "ap-guangzhou",
			StoragePriceGB:  0.118,
			RequestPrice1K:  0.01,
			TransferPriceGB: 0.50,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "腾讯云COS标准存储，适合频繁访问的数据",
			CreatedAt:       time.Now(),
		},
		{
			ID:              "tencent-cos-ia",
			Name:            "腾讯云 COS 低频访问存储",
			Provider:        ProviderTencentCOS,
			Tier:            TierInfrequent,
			Region:          "ap-guangzhou",
			StoragePriceGB:  0.08,
			RequestPrice1K:  0.10,
			TransferPriceGB: 0.50,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "腾讯云COS低频访问存储，适合偶尔访问的数据",
			CreatedAt:       time.Now(),
		},
		{
			ID:              "tencent-cos-archive",
			Name:            "腾讯云 COS 归档存储",
			Provider:        ProviderTencentCOS,
			Tier:            TierArchive,
			Region:          "ap-guangzhou",
			StoragePriceGB:  0.033,
			RequestPrice1K:  0.10,
			TransferPriceGB: 0.50,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "腾讯云COS归档存储，适合长期保存的数据",
			CreatedAt:       time.Now(),
		},
		// MinIO
		{
			ID:              "minio-standard",
			Name:            "MinIO 标准存储",
			Provider:        ProviderMinIO,
			Tier:            TierStandard,
			Region:          "local",
			StoragePriceGB:  0.05,  // 基于硬件成本估算
			RequestPrice1K:  0.001,
			TransferPriceGB: 0.01,
			MinStorageGB:    0,
			MaxStorageGB:    math.MaxFloat64,
			Description:     "MinIO本地存储，适合私有云部署",
			CreatedAt:       time.Now(),
		},
	}
}

// GetPlans 获取所有存储方案
func (m *Manager) GetPlans() []StoragePlan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plans
}

// GetPlansByProvider 按提供商筛选方案
func (m *Manager) GetPlansByProvider(provider StorageProvider) []StoragePlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []StoragePlan
	for _, plan := range m.plans {
		if plan.Provider == provider {
			result = append(result, plan)
		}
	}
	return result
}

// CompareCost 成本对比
func (m *Manager) CompareCost(req CostCompareRequest) (*CostCompareResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if req.StorageGB <= 0 {
		return nil, fmt.Errorf("storage_gb must be positive")
	}

	var comparisons []ProviderCost

	// 筛选符合条件的方案
	for _, plan := range m.plans {
		// 检查提供商筛选
		if len(req.Providers) > 0 {
			found := false
			for _, p := range req.Providers {
				if p == plan.Provider {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查存储层级筛选
		if len(req.Tiers) > 0 {
			found := false
			for _, t := range req.Tiers {
				if t == plan.Tier {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查存储量范围
		if req.StorageGB < plan.MinStorageGB || req.StorageGB > plan.MaxStorageGB {
			continue
		}

		// 计算成本
		storageCost := req.StorageGB * plan.StoragePriceGB
		requestCost := float64(req.MonthlyReads+req.MonthlyWrites) / 1000 * plan.RequestPrice1K
		transferCost := req.TransferGB * plan.TransferPriceGB
		totalMonthly := storageCost + requestCost + transferCost

		comparisons = append(comparisons, ProviderCost{
			Provider:        plan.Provider,
			Tier:            plan.Tier,
			Region:          plan.Region,
			StorageCost:     storageCost,
			RequestCost:     requestCost,
			TransferCost:    transferCost,
			TotalMonthly:    totalMonthly,
			TotalYearly:     totalMonthly * 12,
			StoragePriceGB:  plan.StoragePriceGB,
			ConfidenceLevel: "high",
		})
	}

	if len(comparisons) == 0 {
		return nil, fmt.Errorf("no matching storage plans found")
	}

	// 按总成本排序
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].TotalMonthly < comparisons[j].TotalMonthly
	})

	result := &CostCompareResult{
		Request:     req,
		Comparisons: comparisons,
		BestOption:  &comparisons[0],
		GeneratedAt: time.Now(),
	}

	return result, nil
}

// GetRecommendations 获取优化建议
func (m *Manager) GetRecommendations(storageGB float64, currentProvider StorageProvider, currentTier StorageTier, monthlyCost float64) (*RecommendationsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recommendations []OptimizationRecommendation

	// 获取当前方案成本
	currentCost := monthlyCost
	if currentCost == 0 {
		// 估算当前成本
		for _, plan := range m.plans {
			if plan.Provider == currentProvider && plan.Tier == currentTier {
				currentCost = storageGB * plan.StoragePriceGB
				break
			}
		}
	}

	// 1. 存储层级迁移建议
	if currentTier == TierStandard {
		// 建议迁移到低频存储
		for _, plan := range m.plans {
			if plan.Provider == currentProvider && plan.Tier == TierInfrequent {
				newCost := storageGB * plan.StoragePriceGB
				savings := currentCost - newCost
				if savings > 0 {
					recommendations = append(recommendations, OptimizationRecommendation{
						ID:          fmt.Sprintf("tier-migration-%s-%s", currentProvider, plan.ID),
						Type:        OptTierMigration,
						Title:       "迁移到低频访问存储",
						Description: fmt.Sprintf("将数据从标准存储迁移到低频访问存储，预计每月节省 $%.2f", savings),
						Priority:    PriorityMedium,
						EstimatedSavingsMonthly: savings,
						EstimatedSavingsYearly:  savings * 12,
						SavingsPercent:          (savings / currentCost) * 100,
						Difficulty:              "medium",
						CurrentProvider:         currentProvider,
						CurrentTier:             currentTier,
						RecommendedProvider:     currentProvider,
						RecommendedTier:         TierInfrequent,
						CreatedAt:               time.Now(),
					})
				}
				break
			}
		}
	}

	// 2. 提供商切换建议
	// 找到当前提供商的所有方案
	var currentPlans []StoragePlan
	for _, plan := range m.plans {
		if plan.Provider == currentProvider {
			currentPlans = append(currentPlans, plan)
		}
	}

	// 对比其他提供商
	for _, plan := range m.plans {
		if plan.Provider == currentProvider {
			continue
		}
		if plan.Tier != currentTier {
			continue
		}

		newCost := storageGB * plan.StoragePriceGB
		savings := currentCost - newCost

		if savings > 0 {
			recommendations = append(recommendations, OptimizationRecommendation{
				ID:          fmt.Sprintf("provider-switch-%s-%s", currentProvider, plan.Provider),
				Type:        OptProviderSwitch,
				Title:       fmt.Sprintf("切换到 %s", plan.Provider),
				Description: fmt.Sprintf("切换到 %s %s 存储，预计每月节省 $%.2f", plan.Provider, plan.Tier, savings),
				Priority:    PriorityHigh,
				EstimatedSavingsMonthly: savings,
				EstimatedSavingsYearly:  savings * 12,
				SavingsPercent:          (savings / currentCost) * 100,
				Difficulty:              "hard",
				CurrentProvider:         currentProvider,
				CurrentTier:             currentTier,
				RecommendedProvider:     plan.Provider,
				RecommendedTier:         plan.Tier,
				CreatedAt:               time.Now(),
			})
		}
	}

	// 3. 生命周期策略建议
	if storageGB > 100 { // 大于100GB才建议
		archiveCost := storageGB * 0.004 // 使用AWS Glacier价格估算
		savings := currentCost - archiveCost
		if savings > 0 {
			recommendations = append(recommendations, OptimizationRecommendation{
				ID:          "lifecycle-archive",
				Type:        OptLifecycle,
				Title:       "实施生命周期策略",
				Description: fmt.Sprintf("将超过90天未访问的数据自动迁移到归档存储，预计每月节省 $%.2f", savings*0.3),
				Priority:    PriorityLow,
				EstimatedSavingsMonthly: savings * 0.3,
				EstimatedSavingsYearly:  savings * 0.3 * 12,
				SavingsPercent:          (savings * 0.3 / currentCost) * 100,
				Difficulty:              "easy",
				CreatedAt:               time.Now(),
			})
		}
	}

	// 按节省金额排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].EstimatedSavingsMonthly > recommendations[j].EstimatedSavingsMonthly
	})

	// 计算总节省
	totalSavings := 0.0
	for _, rec := range recommendations {
		totalSavings += rec.EstimatedSavingsMonthly
	}

	response := &RecommendationsResponse{
		Recommendations: recommendations,
		TotalSavings:    totalSavings,
		GeneratedAt:     time.Now(),
	}

	return response, nil
}

// GetCostTrends 获取成本趋势
func (m *Manager) GetCostTrends(req CostTrendRequest) (*CostTrendResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 如果没有历史数据，生成模拟数据
	if len(m.costHistory) == 0 {
		m.generateMockTrends()
	}

	var trends []CostTrendPoint
	for _, points := range m.costHistory {
		for _, point := range points {
			if !req.StartDate.IsZero() && point.Date.Before(req.StartDate) {
				continue
			}
			if !req.EndDate.IsZero() && point.Date.After(req.EndDate) {
				continue
			}
			if req.Provider != "" && point.Provider != string(req.Provider) {
				continue
			}
			trends = append(trends, point)
		}
	}

	// 按时间排序
	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Date.Before(trends[j].Date)
	})

	// 计算摘要
	summary := m.calculateTrendSummary(trends)

	response := &CostTrendResponse{
		Request:     req,
		Trends:      trends,
		Summary:     summary,
		GeneratedAt: time.Now(),
	}

	return response, nil
}

// generateMockTrends 生成模拟趋势数据
func (m *Manager) generateMockTrends() {
	providers := []string{"aws_s3", "aliyun_oss", "tencent_cos", "minio"}
	baseCosts := map[string]float64{
		"aws_s3":      100,
		"aliyun_oss":  150,
		"tencent_cos": 140,
		"minio":       80,
	}

	for _, provider := range providers {
		baseCost := baseCosts[provider]
		points := make([]CostTrendPoint, 0)

		for i := 0; i < 12; i++ {
			date := time.Now().AddDate(0, -11+i, 0)
			// 模拟成本增长
			growth := 1.0 + float64(i)*0.02
			cost := baseCost * growth

			points = append(points, CostTrendPoint{
				Date:      date,
				Cost:      cost,
				StorageGB: 1000 * growth,
				Provider:  provider,
				Tier:      "standard",
			})
		}

		m.costHistory[provider] = points
	}
}

// calculateTrendSummary 计算趋势摘要
func (m *Manager) calculateTrendSummary(trends []CostTrendPoint) TrendSummary {
	if len(trends) == 0 {
		return TrendSummary{}
	}

	totalCost := 0.0
	maxCost := math.Inf(-1)
	minCost := math.Inf(1)

	for _, t := range trends {
		totalCost += t.Cost
		if t.Cost > maxCost {
			maxCost = t.Cost
		}
		if t.Cost < minCost {
			minCost = t.Cost
		}
	}

	avgCost := totalCost / float64(len(trends))

	// 计算成本变化
	firstCost := trends[0].Cost
	lastCost := trends[len(trends)-1].Cost
	costChange := 0.0
	if firstCost > 0 {
		costChange = ((lastCost - firstCost) / firstCost) * 100
	}

	return TrendSummary{
		TotalCost:      totalCost,
		AvgMonthlyCost: avgCost,
		MaxCost:        maxCost,
		MinCost:        minCost,
		CostChange:     costChange,
		GrowthRate:     2.0, // 模拟月增长率
	}
}

// AddCostHistory 添加成本历史数据
func (m *Manager) AddCostHistory(provider string, point CostTrendPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.costHistory[provider] = append(m.costHistory[provider], point)
	log.Printf("Added cost history for %s: $%.2f", provider, point.Cost)
}