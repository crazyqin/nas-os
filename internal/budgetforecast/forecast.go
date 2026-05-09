package budgetforecast

import (
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ForecastEngine 存储预算预测引擎。
type ForecastEngine struct {
	mu        sync.RWMutex
	snapshots []UsageSnapshot
	totalBytes int64
	logger    *zap.Logger
}

// NewForecastEngine 创建预测引擎。
func NewForecastEngine(snapshots []UsageSnapshot, totalBytes int64, logger *zap.Logger) *ForecastEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	// 按日期排序
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Date.Before(snapshots[j].Date)
	})
	return &ForecastEngine{
		snapshots:  snapshots,
		totalBytes: totalBytes,
		logger:     logger,
	}
}

// AddSnapshot 添加使用快照。
func (e *ForecastEngine) AddSnapshot(s UsageSnapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.snapshots = append(e.snapshots, s)
	sort.Slice(e.snapshots, func(i, j int) bool {
		return e.snapshots[i].Date.Before(e.snapshots[j].Date)
	})
	e.logger.Info("添加使用快照",
		zap.Time("date", s.Date),
		zap.Int64("used_bytes", s.UsedBytes),
	)
}

// Forecast 预测未来 N 个月。
func (e *ForecastEngine) Forecast(months int) ForecastResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := ForecastResult{
		GeneratedAt:   time.Now(),
		HistoryPoints: e.snapshots,
	}

	if len(e.snapshots) == 0 {
		return result
	}

	// 计算历史天数
	first := e.snapshots[0].Date
	last := e.snapshots[len(e.snapshots)-1].Date
	result.HistoryDays = int(last.Sub(first).Hours()/24) + 1

	// 当前使用量（最后一个快照）
	lastSnap := e.snapshots[len(e.snapshots)-1]
	result.CurrentUsageGB = float64(lastSnap.UsedBytes) / (1024 * 1024 * 1024)

	// 当前月成本
	usedTB := float64(lastSnap.UsedBytes) / (1024 * 1024 * 1024 * 1024)
	costPerTB := lastSnap.CostPerTB
	if costPerTB == 0 {
		costPerTB = 100 // 默认100元/TB/月
	}
	result.MonthlyCostNow = usedTB * costPerTB

	// 如果磁盘已满
	if e.totalBytes > 0 && lastSnap.UsedBytes >= e.totalBytes {
		result.DaysUntilFull = 0
		result.MonthlyGrowthGB = 0
		result.AnnualCostEst = result.MonthlyCostNow * 12
		e.logger.Warn("磁盘已满或超过容量")
		return result
	}

	// 线性回归
	var slope, intercept, r2 float64
	if len(e.snapshots) >= 2 {
		xs := make([]float64, len(e.snapshots))
		ys := make([]float64, len(e.snapshots))
		for i, s := range e.snapshots {
			xs[i] = s.Date.Sub(first).Hours() / 24 // 天数
			ys[i] = float64(s.UsedBytes) / (1024 * 1024 * 1024)
		}
		slope, intercept, r2 = linearRegression(xs, ys)
		result.MonthlyGrowthGB = slope * 30 // 月增长
	}

	// 生成预测点
	for m := 1; m <= months; m++ {
		days := float64(m) * 30
		predictedGB := intercept + slope*(float64(result.HistoryDays)+days)

		// 置信度随时间衰减
		confidence := r2 * math.Exp(-float64(m)*0.1)
		if confidence < 0 {
			confidence = 0
		}

		predictedDate := last.AddDate(0, m, 0)
		predictedUsedTB := predictedGB / 1024
		predictedCost := predictedUsedTB * costPerTB

		result.Forecast = append(result.Forecast, ForecastPoint{
			Date:          predictedDate,
			PredictedGB:   predictedGB,
			PredictedCost: predictedCost,
			Confidence:    confidence,
		})
	}

	// 预计磁盘满时间
	if e.totalBytes > 0 {
		totalGB := float64(e.totalBytes) / (1024 * 1024 * 1024)
		if slope > 0 {
			daysToFull := (totalGB - intercept) / slope
			if daysToFull > float64(result.HistoryDays) {
				result.DaysUntilFull = int(daysToFull) - result.HistoryDays
			} else {
				result.DaysUntilFull = 0 // 已满或即将满
			}
		} else {
			// 无增长或负增长，磁盘不会满
			if currentUsageGB := float64(lastSnap.UsedBytes) / (1024 * 1024 * 1024); currentUsageGB < totalGB {
				result.DaysUntilFull = 999999
			} else {
				result.DaysUntilFull = 0
			}
		}
	}

	// 年度成本估算
	result.AnnualCostEst = result.MonthlyCostNow * 12
	if result.MonthlyGrowthGB > 0 {
		// 考虑增长趋势的年成本
		annualGrowthTB := result.MonthlyGrowthGB * 12 / 1024
		result.AnnualCostEst = (usedTB + annualGrowthTB/2) * costPerTB * 12
	}

	// 生成预算告警
	result.Alerts = e.generateAlerts(lastSnap, costPerTB, slope, intercept, last)

	return result
}

// generateAlerts 生成预算告警。
func (e *ForecastEngine) generateAlerts(lastSnap UsageSnapshot, costPerTB, slope, intercept float64, lastDate time.Time) []BudgetAlert {
	thresholds := []struct {
		amount   float64
		severity string
	}{
		{5000, "critical"},
		{2000, "critical"},
		{1000, "warning"},
		{500, "info"},
	}

	var alerts []BudgetAlert
	for _, t := range thresholds {
		// 找到预测月成本超过阈值的时间
		// cost = (usedTB + growth_per_day * days) * costPerTB
		// t.amount = (currentTB + slope*days/1024) * costPerTB
		currentTB := float64(lastSnap.UsedBytes) / (1024 * 1024 * 1024 * 1024)

		if slope <= 0 {
			// 无增长，不触发告警
			continue
		}

		// cost = (currentTB + slope_days*days/1024) * costPerTB
		// t.amount = (currentTB + slope*days/(1024*1024*1024)) * costPerTB
		// slope is in GB/day
		targetTB := t.amount / costPerTB
		if targetTB <= currentTB {
			// 当前已超过阈值
			alerts = append(alerts, BudgetAlert{
				Threshold:     t.amount,
				PredictedDate: lastDate,
				Severity:      t.severity,
			})
			continue
		}

		growthTBPerDay := slope / (1024) // slope is GB/day, convert to TB/day
		if growthTBPerDay <= 0 {
			continue
		}
		daysNeeded := (targetTB - currentTB) / growthTBPerDay
		triggerDate := lastDate.AddDate(0, 0, int(daysNeeded))

		alerts = append(alerts, BudgetAlert{
			Threshold:     t.amount,
			PredictedDate: triggerDate,
			Severity:      t.severity,
		})
	}

	return alerts
}

// linearRegression 最小二乘法线性回归。
func linearRegression(x, y []float64) (slope, intercept, r2 float64) {
	n := float64(len(x))
	if n < 2 {
		return 0, 0, 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	// R²
	meanY := sumY / n
	var ssTot, ssRes float64
	for i := range x {
		predicted := slope*x[i] + intercept
		ssRes += (y[i] - predicted) * (y[i] - predicted)
		ssTot += (y[i] - meanY) * (y[i] - meanY)
	}
	if ssTot == 0 {
		r2 = 1
	} else {
		r2 = 1 - ssRes/ssTot
	}

	return slope, intercept, r2
}

// LinearRegression 导出线性回归函数，供测试使用。
func LinearRegression(x, y []float64) (slope, intercept, r2 float64) {
	return linearRegression(x, y)
}
