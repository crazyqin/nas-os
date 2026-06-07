// Package tiercost 提供存储分层成本分析功能
package tiercost

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// ========== 分析器 ==========

// TierCostAnalyzer 存储分层成本分析器.
type TierCostAnalyzer struct {
	mu sync.RWMutex

	// tiers 存储层信息.
	tiers map[TierType]*TierInfo

	// datasets 数据集信息.
	datasets map[string]*DatasetInfo

	// costHistory 历史成本记录.
	costHistory []CostTrend

	// pricing 存储单价配置.
	pricing DefaultPricing

	// logger 日志.
	logger *slog.Logger
}

// NewTierCostAnalyzer 创建分层成本分析器.
func NewTierCostAnalyzer(pricing *DefaultPricing) *TierCostAnalyzer {
	p := DefaultPricingConfig()
	if pricing != nil {
		p = *pricing
	}

	return &TierCostAnalyzer{
		tiers:       make(map[TierType]*TierInfo),
		datasets:    make(map[string]*DatasetInfo),
		costHistory: make([]CostTrend, 0),
		pricing:     p,
		logger:      slog.Default(),
	}
}

// ========== 存储层管理 ==========

// RegisterTier 注册存储层.
func (a *TierCostAnalyzer) RegisterTier(info *TierInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 计算使用率
	if info.Capacity > 0 {
		info.Utilization = float64(info.Used) / float64(info.Capacity)
	}

	// 设置默认单价
	if info.UnitPrice <= 0 {
		info.UnitPrice = a.getUnitPrice(info.Name)
	}

	// 设置显示名称
	if info.DisplayName == "" {
		info.DisplayName = tierDisplayName(info.Name)
	}

	a.tiers[info.Name] = info
	a.logger.Info("注册存储层", "tier", info.Name, "capacity_tb", float64(info.Capacity)/(1024*1024*1024*1024))
}

// RemoveTier 移除存储层.
func (a *TierCostAnalyzer) RemoveTier(name TierType) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.tiers, name)
}

// GetTier 获取存储层信息.
func (a *TierCostAnalyzer) GetTier(name TierType) (*TierInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tier, ok := a.tiers[name]
	if !ok {
		return nil, ErrTierNotFound
	}
	return tier, nil
}

// ListTiers 列出所有存储层.
func (a *TierCostAnalyzer) ListTiers() []*TierInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tiers := make([]*TierInfo, 0, len(a.tiers))
	for _, t := range a.tiers {
		tiers = append(tiers, t)
	}
	return tiers
}

// ========== 数据集管理 ==========

// RegisterDataset 注册数据集.
func (a *TierCostAnalyzer) RegisterDataset(ds *DatasetInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.datasets[ds.Name] = ds
}

// RemoveDataset 移除数据集.
func (a *TierCostAnalyzer) RemoveDataset(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.datasets, name)
}

// ListDatasets 列出所有数据集.
func (a *TierCostAnalyzer) ListDatasets() []*DatasetInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	datasets := make([]*DatasetInfo, 0, len(a.datasets))
	for _, ds := range a.datasets {
		datasets = append(datasets, ds)
	}
	return datasets
}

// ========== 成本分析 ==========

// AnalyzeCost 生成分层成本分析报告.
func (a *TierCostAnalyzer) AnalyzeCost() *CostReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &CostReport{
		GeneratedAt: time.Now(),
	}

	// 计算各层成本
	totalCost := 0.0
	details := make([]TierCostDetail, 0, len(a.tiers))

	for _, tier := range a.tiers {
		usedTB := float64(tier.Used) / (1024 * 1024 * 1024 * 1024)
		capacityTB := float64(tier.Capacity) / (1024 * 1024 * 1024 * 1024)
		annualCost := usedTB * tier.UnitPrice

		detail := TierCostDetail{
			TierName:    tier.Name,
			DisplayName: tier.DisplayName,
			CapacityTB:  round2(capacityTB),
			UsedTB:      round2(usedTB),
			Utilization: round2(tier.Utilization),
			UnitPrice:   tier.UnitPrice,
			AnnualCost:  round2(annualCost),
		}
		details = append(details, detail)
		totalCost += annualCost
	}

	// 计算成本占比
	for i := range details {
		if totalCost > 0 {
			details[i].CostPercentage = round2(details[i].AnnualCost / totalCost * 100)
		}
	}

	report.TotalCost = round2(totalCost)
	report.TierBreakdown = details

	// 生成推荐建议
	recommendations, savings := a.generateRecommendations()
	report.Recommendations = recommendations
	report.SavingsPotential = round2(savings)

	// 记录历史成本
	a.recordCostHistory(totalCost)

	return report
}

// GetRecommendations 获取分层建议.
func (a *TierCostAnalyzer) GetRecommendations() ([]TierRecommendation, float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.generateRecommendations()
}

// GetCostTrends 获取成本趋势.
func (a *TierCostAnalyzer) GetCostTrends(months int) []CostTrend {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if months <= 0 {
		months = 12
	}

	// 返回历史数据 + 预测数据
	trends := make([]CostTrend, 0, months)

	// 复制历史数据
	for _, t := range a.costHistory {
		trends = append(trends, t)
	}

	// 如果历史数据不足，生成模拟数据
	if len(trends) < 3 {
		baseCost := a.calculateCurrentMonthlyCost()
		now := time.Now()
		for i := len(trends); i < 6; i++ {
			date := now.AddDate(0, i-6, 0)
			cost := baseCost * (1 + float64(i)*0.02) // 模拟2%月增长
			trends = append(trends, CostTrend{
				Date:        date,
				Cost:        round2(cost),
				IsProjected: false,
			})
		}
	}

	// 生成预测数据（基于线性回归）
	if len(trends) >= 2 {
		lastTrend := trends[len(trends)-1]
		growthRate := a.estimateGrowthRate(trends)

		for i := 1; i <= months; i++ {
			date := lastTrend.Date.AddDate(0, i, 0)
			projected := lastTrend.Cost * (1 + growthRate*float64(i))
			trends = append(trends, CostTrend{
				Date:          date,
				ProjectedCost: round2(projected),
				IsProjected:   true,
			})
		}
	}

	// 截取指定月数
	if len(trends) > months*2 {
		trends = trends[len(trends)-months*2:]
	}

	return trends
}

// SimulateTierPlan 模拟分层方案.
func (a *TierCostAnalyzer) SimulateTierPlan(req *SimulateRequest) (*SimulateResponse, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if req == nil || len(req.Datasets) == 0 {
		return nil, ErrInvalidInput
	}

	// 计算当前成本
	currentCost := 0.0
	currentByTier := make(map[TierType]float64)
	for _, ds := range req.Datasets {
		tier, ok := a.tiers[ds.CurrentTier]
		price := a.getUnitPrice(ds.CurrentTier)
		if ok {
			price = tier.UnitPrice
		}
		sizeTB := float64(ds.Size) / (1024 * 1024 * 1024 * 1024)
		cost := sizeTB * price
		currentCost += cost
		currentByTier[ds.CurrentTier] += cost
	}

	// 计算模拟成本
	simulatedCost := 0.0
	simByTier := make(map[TierType]float64)
	simUsedByTier := make(map[TierType]float64)
	for _, ds := range req.Datasets {
		targetTier := ds.CurrentTier
		if assigned, ok := req.TierAssignments[ds.Name]; ok {
			targetTier = assigned
		}
		tier, ok := a.tiers[targetTier]
		price := a.getUnitPrice(targetTier)
		if ok {
			price = tier.UnitPrice
		}
		sizeTB := float64(ds.Size) / (1024 * 1024 * 1024 * 1024)
		cost := sizeTB * price
		simulatedCost += cost
		simByTier[targetTier] += cost
		simUsedByTier[targetTier] += sizeTB
	}

	savings := currentCost - simulatedCost
	savingsPct := 0.0
	if currentCost > 0 {
		savingsPct = savings / currentCost * 100
	}

	// 构建对比详情
	details := make([]SimulateTierDetail, 0)
	for _, tierType := range []TierType{TierNVMe, TierSSD, TierHDD} {
		tier, ok := a.tiers[tierType]
		displayName := tierDisplayName(tierType)
		price := a.getUnitPrice(tierType)
		if ok {
			displayName = tier.DisplayName
			price = tier.UnitPrice
		}
		details = append(details, SimulateTierDetail{
			TierName:        tierType,
			DisplayName:     displayName,
			CurrentUsedTB:   round2(currentByTier[tierType] / price),
			SimulatedUsedTB: round2(simUsedByTier[tierType]),
			UnitPrice:       price,
			CurrentCost:     round2(currentByTier[tierType]),
			SimulatedCost:   round2(simByTier[tierType]),
		})
	}

	return &SimulateResponse{
		CurrentCost:    round2(currentCost),
		SimulatedCost:  round2(simulatedCost),
		Savings:        round2(savings),
		SavingsPercent: round2(savingsPct),
		Details:        details,
	}, nil
}

// UpdatePricing 更新存储单价.
func (a *TierCostAnalyzer) UpdatePricing(req *PricingUpdateRequest) error {
	if req == nil {
		return ErrInvalidInput
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if req.NVMePricePerTBYear != nil {
		if *req.NVMePricePerTBYear <= 0 {
			return ErrInvalidPricing
		}
		a.pricing.NVMePricePerTBYear = *req.NVMePricePerTBYear
		a.logger.Info("更新NVMe单价", "price", *req.NVMePricePerTBYear)
	}
	if req.SSDPricePerTBYear != nil {
		if *req.SSDPricePerTBYear <= 0 {
			return ErrInvalidPricing
		}
		a.pricing.SSDPricePerTBYear = *req.SSDPricePerTBYear
		a.logger.Info("更新SSD单价", "price", *req.SSDPricePerTBYear)
	}
	if req.HDDPricePerTBYear != nil {
		if *req.HDDPricePerTBYear <= 0 {
			return ErrInvalidPricing
		}
		a.pricing.HDDPricePerTBYear = *req.HDDPricePerTBYear
		a.logger.Info("更新HDD单价", "price", *req.HDDPricePerTBYear)
	}

	// 同步更新已注册存储层的单价
	for _, tier := range a.tiers {
		tier.UnitPrice = a.getUnitPrice(tier.Name)
	}

	return nil
}

// GetPricing 获取当前存储单价配置.
func (a *TierCostAnalyzer) GetPricing() DefaultPricing {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pricing
}

// ========== 内部方法 ==========

// getUnitPrice 获取存储层单价（需持有锁）.
func (a *TierCostAnalyzer) getUnitPrice(tier TierType) float64 {
	switch tier {
	case TierNVMe:
		return a.pricing.NVMePricePerTBYear
	case TierSSD:
		return a.pricing.SSDPricePerTBYear
	case TierHDD:
		return a.pricing.HDDPricePerTBYear
	default:
		return a.pricing.HDDPricePerTBYear
	}
}

// generateRecommendations 生成分层建议（需持有锁）.
func (a *TierCostAnalyzer) generateRecommendations() ([]TierRecommendation, float64) {
	recommendations := make([]TierRecommendation, 0)
	totalSavings := 0.0

	for _, ds := range a.datasets {
		rec := a.evaluateDataset(ds)
		if rec != nil {
			recommendations = append(recommendations, *rec)
			totalSavings += rec.EstSavings
		}
	}

	return recommendations, totalSavings
}

// evaluateDataset 评估单个数据集的分层建议（需持有锁）.
func (a *TierCostAnalyzer) evaluateDataset(ds *DatasetInfo) *TierRecommendation {
	sizeTB := float64(ds.Size) / (1024 * 1024 * 1024 * 1024)
	currentPrice := a.getUnitPrice(ds.CurrentTier)

	var recommendedTier TierType
	var reason string

	switch ds.AccessFrequency {
	case "hot":
		// 热数据：保留在高速层或升级
		if ds.CurrentTier == TierHDD {
			recommendedTier = TierSSD
			reason = "热数据访问频繁，建议迁移到SSD层提升性能"
		} else {
			return nil // 已经在合适层
		}
	case "warm":
		// 温数据：建议SSD层
		if ds.CurrentTier == TierNVMe {
			recommendedTier = TierSSD
			reason = "温数据不需要NVMe性能，迁移到SSD可降低成本"
		} else if ds.CurrentTier == TierHDD {
			recommendedTier = TierSSD
			reason = "温数据偶尔访问，SSD层兼顾性能和成本"
		} else {
			return nil
		}
	case "cold":
		// 冷数据：迁移到HDD
		if ds.CurrentTier != TierHDD {
			recommendedTier = TierHDD
			reason = "冷数据很少访问，迁移到HDD层可大幅降低成本"
		} else {
			return nil
		}
	default:
		// 基于最后访问时间判断
		if time.Since(ds.LastAccessTime) > 90*24*time.Hour {
			if ds.CurrentTier != TierHDD {
				recommendedTier = TierHDD
				reason = fmt.Sprintf("超过90天未访问，建议归档到HDD层")
			} else {
				return nil
			}
		} else if time.Since(ds.LastAccessTime) < 7*24*time.Hour {
			if ds.CurrentTier == TierHDD {
				recommendedTier = TierSSD
				reason = "近期频繁访问，建议迁移到SSD层"
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	if recommendedTier == ds.CurrentTier {
		return nil
	}

	newPrice := a.getUnitPrice(recommendedTier)
	savings := sizeTB * (currentPrice - newPrice)

	return &TierRecommendation{
		DatasetName:     ds.Name,
		CurrentTier:     ds.CurrentTier,
		RecommendedTier: recommendedTier,
		EstSavings:      round2(savings),
		Reason:          reason,
	}
}

// calculateCurrentMonthlyCost 计算当前月度成本（需持有锁）.
func (a *TierCostAnalyzer) calculateCurrentMonthlyCost() float64 {
	total := 0.0
	for _, tier := range a.tiers {
		usedTB := float64(tier.Used) / (1024 * 1024 * 1024 * 1024)
		total += usedTB * tier.UnitPrice / 12
	}
	return total
}

// recordCostHistory 记录历史成本（需持有锁）.
func (a *TierCostAnalyzer) recordCostHistory(annualCost float64) {
	monthlyCost := annualCost / 12
	now := time.Now()

	// 避免同月重复记录
	if len(a.costHistory) > 0 {
		last := a.costHistory[len(a.costHistory)-1]
		if last.Date.Year() == now.Year() && last.Date.Month() == now.Month() {
			a.costHistory[len(a.costHistory)-1].Cost = round2(monthlyCost)
			return
		}
	}

	a.costHistory = append(a.costHistory, CostTrend{
		Date: now,
		Cost: round2(monthlyCost),
	})

	// 保留最近24个月
	if len(a.costHistory) > 24 {
		a.costHistory = a.costHistory[len(a.costHistory)-24:]
	}
}

// estimateGrowthRate 估算月增长率（需持有锁）.
func (a *TierCostAnalyzer) estimateGrowthRate(trends []CostTrend) float64 {
	if len(trends) < 2 {
		return 0.02 // 默认2%
	}

	// 简单线性回归
	n := float64(len(trends))
	var sumX, sumY, sumXY, sumX2 float64
	for i, t := range trends {
		x := float64(i)
		y := t.Cost
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 || sumY == 0 {
		return 0.02
	}

	slope := (n*sumXY - sumX*sumY) / denom
	avgY := sumY / n

	rate := slope / avgY
	if rate < 0 {
		rate = 0
	}
	if rate > 0.5 {
		rate = 0.5 // 上限50%
	}

	return rate
}

// ========== 辅助函数 ==========

// tierDisplayName 获取存储层显示名称.
func tierDisplayName(tier TierType) string {
	switch tier {
	case TierNVMe:
		return "NVMe SSD"
	case TierSSD:
		return "SATA SSD"
	case TierHDD:
		return "HDD 机械硬盘"
	default:
		return string(tier)
	}
}

// round2 保留两位小数.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
