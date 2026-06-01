// Package powerbudget 提供用电分析功能
package powerbudget

import (
	"fmt"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"
)

// Analyzer 用电分析器.
type Analyzer struct {
	engine *Engine
	logger *zap.Logger
}

// NewAnalyzer 创建用电分析器.
func NewAnalyzer(engine *Engine, logger *zap.Logger) *Analyzer {
	return &Analyzer{
		engine: engine,
		logger: logger,
	}
}

// ========== 趋势分析 ==========

// CalculateTrend 计算用电趋势（最近N天）.
func (a *Analyzer) CalculateTrend(days int) TrendDirection {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	if days <= 0 {
		days = 7
	}

	now := time.Now()
	startRecent := now.AddDate(0, 0, -days)
	startPrevious := now.AddDate(0, 0, -days*2)

	var recentEnergy, previousEnergy float64

	for _, r := range a.engine.records {
		if r.Timestamp.After(startRecent) {
			recentEnergy += r.EnergyKWh
		} else if r.Timestamp.After(startPrevious) {
			previousEnergy += r.EnergyKWh
		}
	}

	if previousEnergy == 0 {
		return TrendStable
	}

	changePercent := (recentEnergy - previousEnergy) / previousEnergy * 100.0

	if changePercent > 10 {
		return TrendUp
	} else if changePercent < -10 {
		return TrendDown
	}

	return TrendStable
}

// AnalyzeDailyTrend 分析每日用电趋势.
func (a *Analyzer) AnalyzeDailyTrend(days int) []TrendPoint {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	now := time.Now()
	start := now.AddDate(0, 0, -days)

	dailyMap := make(map[string]*TrendPoint)

	for _, r := range a.engine.records {
		if r.Timestamp.Before(start) {
			continue
		}
		dateKey := r.Timestamp.Format("2006-01-02")
		if _, ok := dailyMap[dateKey]; !ok {
			t, _ := time.Parse("2006-01-02", dateKey)
			dailyMap[dateKey] = &TrendPoint{Date: t}
		}
		dailyMap[dateKey].Energy += r.EnergyKWh
		dailyMap[dateKey].Cost += r.CostCents
	}

	result := make([]TrendPoint, 0, len(dailyMap))
	for _, point := range dailyMap {
		result = append(result, *point)
	}

	sortTrendPoints(result)
	return result
}

// ========== 异常检测 ==========

// AnomalyResult 异常检测结果.
type AnomalyResult struct {
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	Timestamp   time.Time `json:"timestamp"`
	PowerWatts  float64   `json:"power_watts"`
	ExpectedMax float64   `json:"expected_max"`
	Deviation   float64   `json:"deviation"` // 标准差倍数
	Severity    string    `json:"severity"`
}

// DetectAnomalies 检测异常功耗.
func (a *Analyzer) DetectAnomalies(days int) []AnomalyResult {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	if days <= 0 {
		days = 7
	}

	now := time.Now()
	start := now.AddDate(0, 0, -days)

	// 按设备收集功耗数据
	devicePowers := make(map[string][]float64)
	deviceNames := make(map[string]string)
	var recentRecords []*PowerRecord

	for _, r := range a.engine.records {
		if r.Timestamp.Before(start) {
			continue
		}
		devicePowers[r.DeviceID] = append(devicePowers[r.DeviceID], r.PowerWatts)
		deviceNames[r.DeviceID] = r.DeviceName
		recentRecords = append(recentRecords, r)
	}

	var anomalies []AnomalyResult

	for deviceID, powers := range devicePowers {
		if len(powers) < 5 {
			continue
		}

		mean, stddev := calculateStats(powers)
		threshold := mean + 3*stddev

		for _, r := range recentRecords {
			if r.DeviceID != deviceID {
				continue
			}
			if r.PowerWatts > threshold && threshold > 0 {
				deviation := (r.PowerWatts - mean) / stddev
				severity := "warning"
				if deviation > 4 {
					severity = "critical"
				}

				anomalies = append(anomalies, AnomalyResult{
					DeviceID:    deviceID,
					DeviceName:  deviceNames[deviceID],
					Timestamp:   r.Timestamp,
					PowerWatts:  r.PowerWatts,
					ExpectedMax: threshold,
					Deviation:   deviation,
					Severity:    severity,
				})
			}
		}
	}

	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].Deviation > anomalies[j].Deviation
	})

	if anomalies == nil {
		return make([]AnomalyResult, 0)
	}

	return anomalies
}

// ========== 成本优化建议 ==========

// OptimizationSuggestion 优化建议.
type OptimizationSuggestion struct {
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SavingsKWh  int64  `json:"savings_kwh"`  // 预计节省度数
	SavingsCents int64 `json:"savings_cents"` // 预计节省金额（分）
	Priority    string `json:"priority"`
}

// GetOptimizationSuggestions 获取用电优化建议.
func (a *Analyzer) GetOptimizationSuggestions() []OptimizationSuggestion {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	var suggestions []OptimizationSuggestion

	// 1. 高耗电设备建议
	for _, dp := range a.engine.devices {
		if dp.UsagePercent > 50 {
			suggestions = append(suggestions, OptimizationSuggestion{
				Category:    "设备优化",
				Title:       dp.DeviceName + " 耗电占比过高",
				Description: dp.DeviceName + "占总用电" + formatPercent(dp.UsagePercent) + "，建议优化使用模式或考虑升级为节能设备",
				SavingsKWh:  int64(dp.TotalEnergy * 0.1),
				SavingsCents: int64(float64(dp.TotalCost) * 0.1),
				Priority:    "high",
			})
		}
	}

	// 2. 峰时用电建议
	var peakEnergy, offPeakEnergy float64
	for _, r := range a.engine.records {
		hour := r.Timestamp.Hour()
		if hour >= 8 && hour <= 22 {
			peakEnergy += r.EnergyKWh
		} else {
			offPeakEnergy += r.EnergyKWh
		}
	}

	totalEnergy := peakEnergy + offPeakEnergy
	if totalEnergy > 0 {
		peakPercent := peakEnergy / totalEnergy * 100.0
		if peakPercent > 70 {
			electricityPrice := DefaultElectricityPrice
			if a.engine.budget != nil {
				electricityPrice = a.engine.budget.ElectricityPrice
			}
			suggestions = append(suggestions, OptimizationSuggestion{
				Category:    "用电时段",
				Title:       "峰时用电占比过高",
				Description: "峰时用电占" + formatPercent(peakPercent) + "，建议将部分用电转移到谷时（22:00-8:00）以降低成本",
				SavingsKWh:  int64(peakEnergy * 0.15),
				SavingsCents: int64(electricityPrice * peakEnergy * 0.15),
				Priority:    "medium",
			})
		}
	}

	// 3. 待机功耗建议
	standbyPower := a.calculateStandbyPower()
	if standbyPower > 20 {
		electricityPrice := DefaultElectricityPrice
		if a.engine.budget != nil {
			electricityPrice = a.engine.budget.ElectricityPrice
		}
		suggestions = append(suggestions, OptimizationSuggestion{
			Category:    "待机优化",
			Title:       "待机功耗偏高",
			Description: "检测到待机功耗约" + formatPower(standbyPower) + "W，建议关闭不使用的设备或使用智能插座",
			SavingsKWh:  int64(standbyPower * 24 * 30 / 1000),
			SavingsCents: int64(electricityPrice * standbyPower * 24 * 30 / 1000),
			Priority:    "medium",
		})
	}

	// 4. 预算预警建议
	if a.engine.budget != nil {
		prediction := a.PredictMonthly()
		if prediction != nil && prediction.WillExceed {
			suggestions = append(suggestions, OptimizationSuggestion{
				Category:    "预算管理",
				Title:       "本月预计超支",
				Description: "按当前用电趋势，本月预计用电" + formatEnergy(prediction.PredictedKWh) + "，超出预算" + formatCost(float64(prediction.PredictedCost) - a.engine.budget.MonthlyAmount),
				Priority:    "high",
			})
		}
	}

	if len(suggestions) == 0 {
		return make([]OptimizationSuggestion, 0)
	}

	sort.Slice(suggestions, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return priorityOrder[suggestions[i].Priority] < priorityOrder[suggestions[j].Priority]
	})

	return suggestions
}

// ========== 用电预测 ==========

// PredictMonthly 预测月度用电.
func (a *Analyzer) PredictMonthly() *Prediction {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	if a.engine.budget == nil {
		return nil
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	daysElapsed := int(now.Sub(startOfMonth).Hours()/24) + 1
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	daysLeft := daysInMonth - daysElapsed

	// 计算本月已用电量
	var monthEnergy float64
	for _, r := range a.engine.records {
		if r.Timestamp.After(startOfMonth) || r.Timestamp.Equal(startOfMonth) {
			monthEnergy += r.EnergyKWh
		}
	}

	if daysElapsed == 0 {
		return nil
	}

	dailyAvg := monthEnergy / float64(daysElapsed)
	predictedKWh := monthEnergy + dailyAvg*float64(daysLeft)
	predictedCost := int64(predictedKWh * a.engine.budget.ElectricityPrice)

	// 计算置信度（基于数据量）
	var confidence float64
	if daysElapsed >= 14 {
		confidence = 0.85
	} else if daysElapsed >= 7 {
		confidence = 0.7
	} else {
		confidence = 0.5
	}

	// 判断趋势调整
	trend := a.CalculateTrend(7)
	if trend == TrendUp {
		predictedKWh *= 1.1
		predictedCost = int64(predictedKWh * a.engine.budget.ElectricityPrice)
		confidence *= 0.9
	} else if trend == TrendDown {
		predictedKWh *= 0.95
		predictedCost = int64(predictedKWh * a.engine.budget.ElectricityPrice)
	}

	willExceed := float64(predictedCost) > a.engine.budget.MonthlyAmount

	return &Prediction{
		Method:        "线性外推",
		DaysLeft:      daysLeft,
		DailyAvg:      dailyAvg,
		PredictedKWh:  predictedKWh,
		PredictedCost: predictedCost,
		Confidence:    confidence,
		WillExceed:    willExceed,
	}
}

// PredictDevicePredict 预测设备用电.
func (a *Analyzer) PredictDevicePredict(deviceID string, days int) float64 {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	now := time.Now()
	start := now.AddDate(0, 0, -days)

	var totalEnergy float64
	var count int

	for _, r := range a.engine.records {
		if r.DeviceID == deviceID && r.Timestamp.After(start) {
			totalEnergy += r.EnergyKWh
			count++
		}
	}

	if count == 0 || days == 0 {
		return 0
	}

	return totalEnergy / float64(days) * 30.0
}

// ========== 辅助方法 ==========

func (a *Analyzer) calculateStandbyPower() float64 {
	// 计算夜间的平均功率作为待机功耗
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nightStart := today.Add(2 * time.Hour)   // 凌晨2点
	nightEnd := today.Add(5 * time.Hour)     // 凌晨5点

	var totalPower float64
	var count int

	for _, r := range a.engine.records {
		if r.Timestamp.After(nightStart) && r.Timestamp.Before(nightEnd) {
			totalPower += r.PowerWatts
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalPower / float64(count)
}

// calculateStats 计算均值和标准差.
func calculateStats(data []float64) (mean, stddev float64) {
	if len(data) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range data {
		sum += v
	}
	mean = sum / float64(len(data))

	var variance float64
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(data))
	stddev = math.Sqrt(variance)

	return mean, stddev
}

// formatPercent 格式化百分比.
func formatPercent(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}

// formatPower 格式化功率.
func formatPower(watts float64) string {
	return fmt.Sprintf("%.1f", watts)
}

// formatEnergy 格式化电量.
func formatEnergy(kwh float64) string {
	return fmt.Sprintf("%.1f度", kwh)
}
