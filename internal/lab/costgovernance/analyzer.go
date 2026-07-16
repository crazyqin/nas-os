// Package costgovernance 提供多云成本治理功能
package costgovernance

import (
	"math"
	"sort"
	"sync"
)

// Analyzer 成本分析器.
type Analyzer struct {
	mu      sync.RWMutex
	manager *Manager
	trends  map[string][]*CostTrend // provider -> trend data
}

// NewAnalyzer 创建成本分析器.
func NewAnalyzer(manager *Manager) *Analyzer {
	return &Analyzer{
		manager: manager,
		trends:  make(map[string][]*CostTrend),
	}
}

// AddTrendData 添加趋势数据.
func (a *Analyzer) AddTrendData(provider string, trend *CostTrend) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.trends[provider] = append(a.trends[provider], trend)
}

// PredictCost 成本趋势预测（线性回归）.
func (a *Analyzer) PredictCost(provider string, futureDays int) ([]*CostTrend, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, ok := a.trends[provider]
	if !ok || len(data) < 2 {
		return nil, ErrInvalidInput
	}

	// 排序
	sorted := make([]*CostTrend, len(data))
	copy(sorted, data)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	// 线性回归: y = a + b*x
	n := float64(len(sorted))
	baseTime := sorted[0].Date
	var sumX, sumY, sumXY, sumX2 float64
	for _, d := range sorted {
		x := d.Date.Sub(baseTime).Hours() / 24
		y := d.Cost
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return nil, ErrInvalidInput
	}
	b := (n*sumXY - sumX*sumY) / denom
	a0 := (sumY - b*sumX) / n

	// 生成预测
	lastDay := sorted[len(sorted)-1].Date.Sub(baseTime).Hours() / 24
	predictions := make([]*CostTrend, 0, futureDays)
	for i := 1; i <= futureDays; i++ {
		x := lastDay + float64(i)
		cost := a0 + b*x
		if cost < 0 {
			cost = 0
		}
		predictions = append(predictions, &CostTrend{
			Date:     sorted[len(sorted)-1].Date.AddDate(0, 0, i),
			Cost:     math.Round(cost*100) / 100,
			Provider: provider,
		})
	}
	return predictions, nil
}

// DetectAnomalies 异常检测（基于标准差）.
func (a *Analyzer) DetectAnomalies(provider string, threshold float64) []*AnomalyDetection {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, ok := a.trends[provider]
	if !ok || len(data) < 3 {
		return nil
	}

	// 计算均值和标准差
	var sum, sumSq float64
	for _, d := range data {
		sum += d.Cost
		sumSq += d.Cost * d.Cost
	}
	n := float64(len(data))
	mean := sum / n
	variance := (sumSq / n) - (mean * mean)
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)

	// 检测异常值
	anomalies := make([]*AnomalyDetection, 0)
	for _, d := range data {
		deviation := math.Abs(d.Cost-mean) / stddev * 100
		if deviation >= threshold {
			anomalies = append(anomalies, &AnomalyDetection{
				ResourceID:   d.Provider,
				DetectedAt:   d.Date,
				ExpectedCost: math.Round(mean*100) / 100,
				ActualCost:   d.Cost,
				Deviation:    math.Round(deviation*100) / 100,
				Description:  "成本偏离历史均值，可能存在异常支出",
			})
		}
	}
	return anomalies
}

// AnalyzeResourceUtilization 资源利用率分析.
func (a *Analyzer) AnalyzeResourceUtilization(provider CloudProvider) map[string]interface{} {
	usages := a.manager.ListResourceUsages(provider)
	if len(usages) == 0 {
		return map[string]interface{}{
			"total":              0,
			"underutilized":      0,
			"overutilized":       0,
			"avg_cpu_percent":    0.0,
			"avg_memory_percent": 0.0,
		}
	}

	underutilized := 0
	overutilized := 0
	var totalCPU, totalMem float64
	for _, u := range usages {
		totalCPU += u.CPUPercent
		totalMem += u.MemoryPercent
		if u.CPUPercent < 20 && u.MemoryPercent < 30 {
			underutilized++
		}
		if u.CPUPercent > 85 || u.MemoryPercent > 90 {
			overutilized++
		}
	}

	n := float64(len(usages))
	return map[string]interface{}{
		"total":              len(usages),
		"underutilized":      underutilized,
		"overutilized":       overutilized,
		"avg_cpu_percent":    math.Round((totalCPU/n)*100) / 100,
		"avg_memory_percent": math.Round((totalMem/n)*100) / 100,
	}
}

// AnalyzeCapacityRisk 汇总容量水位与低利用成本风险.
func (a *Analyzer) AnalyzeCapacityRisk(provider CloudProvider) *CapacityRiskSummary {
	usages := a.manager.ListResourceUsages(provider)
	summary := &CapacityRiskSummary{
		Provider:        provider,
		TotalResources:  len(usages),
		Recommendations: make([]string, 0),
	}
	if len(usages) == 0 {
		summary.Recommendations = append(summary.Recommendations, "暂无资源数据，建议先接入资源使用采集")
		return summary
	}

	var totalStorageUtilization float64
	for _, u := range usages {
		storageUtilization := 0.0
		if u.StorageTotalGB > 0 {
			storageUtilization = clampRatio(u.StorageUsedGB / u.StorageTotalGB)
			summary.StorageTrackedResources++
			totalStorageUtilization += storageUtilization

			if storageUtilization < 0.2 {
				summary.LowStorageUtilizationResources++
				if u.DailyCost > 0 {
					summary.DailyWasteEstimate += u.DailyCost * 0.2
				}
			}
			if storageUtilization >= 0.85 {
				summary.HighStorageUtilizationResources++
			}
		}

		idle := u.DailyCost > 0 && u.CPUPercent < 15 && u.MemoryPercent < 25
		if idle {
			summary.IdleResources++
			summary.DailyWasteEstimate += u.DailyCost * 0.3
		}

		if u.CPUPercent >= 85 || u.MemoryPercent >= 90 || storageUtilization >= 0.85 {
			summary.OverloadedResources++
		}
	}

	if summary.StorageTrackedResources > 0 {
		summary.AvgStorageUtilizationPercent = math.Round((totalStorageUtilization/float64(summary.StorageTrackedResources))*10000) / 100
	}
	summary.DailyWasteEstimate = math.Round(summary.DailyWasteEstimate*100) / 100
	summary.MonthlyWasteEstimate = math.Round(summary.DailyWasteEstimate*30*100) / 100

	if summary.IdleResources > 0 {
		summary.Recommendations = append(summary.Recommendations, "存在低CPU/内存且仍计费资源，建议缩容、关停或迁移到低成本层")
	}
	if summary.LowStorageUtilizationResources > 0 {
		summary.Recommendations = append(summary.Recommendations, "存在存储利用率低于20%的资源，建议回收空闲容量")
	}
	if summary.HighStorageUtilizationResources > 0 {
		summary.Recommendations = append(summary.Recommendations, "存在存储水位超过85%的资源，建议扩容或启用冷热分层")
	}
	if summary.OverloadedResources > 0 {
		summary.Recommendations = append(summary.Recommendations, "存在接近容量上限的资源，建议纳入容量预警")
	}
	if len(summary.Recommendations) == 0 {
		summary.Recommendations = append(summary.Recommendations, "当前容量与成本风险可控")
	}
	return summary
}

func clampRatio(ratio float64) float64 {
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

// GenerateOptimizationSuggestions 生成成本优化建议.
func (a *Analyzer) GenerateOptimizationSuggestions(provider CloudProvider) []string {
	usages := a.manager.ListResourceUsages(provider)
	suggestions := make([]string, 0)

	for _, u := range usages {
		if u.CPUPercent < 10 && u.DailyCost > 0 {
			suggestions = append(suggestions,
				"资源「"+u.ResourceName+"」CPU使用率低于10%，建议缩容或释放")
		}
		if u.MemoryPercent < 15 && u.DailyCost > 0 {
			suggestions = append(suggestions,
				"资源「"+u.ResourceName+"」内存使用率低于15%，建议降低配置")
		}
		if u.StorageTotalGB > 0 && u.StorageUsedGB/u.StorageTotalGB < 0.2 {
			suggestions = append(suggestions,
				"资源「"+u.ResourceName+"」存储利用率低于20%，建议缩减存储容量")
		}
	}

	// 按区域分析
	regionCosts := make(map[string]float64)
	for _, u := range usages {
		regionCosts[u.Region] += u.DailyCost
	}
	if len(regionCosts) > 1 {
		suggestions = append(suggestions, "检测到多区域部署，建议评估是否可以合并区域以降低网络传输成本")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前资源使用率良好，暂无优化建议")
	}
	return suggestions
}
