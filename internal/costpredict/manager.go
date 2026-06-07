// Package costpredict - 成本预测管理器
// 基于历史数据的成本预测、存储增长趋势预测、预算超支告警
package costpredict

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// Manager 成本预测管理器
type Manager struct {
	mu sync.RWMutex

	config        CostPredictConfig
	costHistory   []CostRecord
	growthHistory []GrowthRecord
	alertConfigs  []AlertConfig
	activeAlerts  []BudgetAlert

	// 预测缓存
	forecastCache map[string]*ForecastResult
}

// NewManager 创建成本预测管理器
func NewManager(config CostPredictConfig) *Manager {
	m := &Manager{
		config:        config,
		costHistory:   make([]CostRecord, 0),
		growthHistory: make([]GrowthRecord, 0),
		alertConfigs:  make([]AlertConfig, 0),
		activeAlerts:  make([]BudgetAlert, 0),
		forecastCache: make(map[string]*ForecastResult),
	}

	// 生成模拟历史数据
	m.generateMockHistory()

	log.Printf("CostPredict Manager initialized with %d cost records", len(m.costHistory))
	return m
}

// generateMockHistory 生成模拟历史数据
func (m *Manager) generateMockHistory() {
	baseCost := 100.0
	baseStorage := int64(500 * 1024 * 1024 * 1024) // 500GB in bytes
	storageTypes := []StorageType{StorageTypeSSD, StorageTypeHDD, StorageTypeNVMe}

	for i := 0; i < 12; i++ {
		date := time.Now().AddDate(0, -11+i, 0)
		growthFactor := 1.0 + float64(i)*0.03

		for _, storageType := range storageTypes {
			cost := baseCost * growthFactor * (1.0 + float64(i%3)*0.1)
			storage := int64(float64(baseStorage) * growthFactor)

			m.costHistory = append(m.costHistory, CostRecord{
				Time:          date,
				Department:    "engineering",
				Project:       "nas-os",
				StorageType:   storageType,
				Cost:          cost,
				UsedCapacity:  int64(float64(storage) * 0.7),
				TotalCapacity: storage,
			})

			m.growthHistory = append(m.growthHistory, GrowthRecord{
				ID:         fmt.Sprintf("growth-%s-%d", storageType, i),
				Date:       date,
				StorageGB:  float64(storage) / (1024 * 1024 * 1024),
				GrowthGB:   float64(storage) * 0.03 / (1024 * 1024 * 1024),
				GrowthRate: 3.0,
				Provider:   string(storageType),
				Tier:       "standard",
				CreatedAt:  time.Now(),
			})
		}
	}
}

// GetForecast 获取成本预测
func (m *Manager) GetForecast(method PredictionMethod, horizon ForecastHorizon, provider string) (*ForecastResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 筛选历史数据
	var history []CostRecord
	for _, record := range m.costHistory {
		// 按存储类型筛选（如果有指定）
		if provider == "" || string(record.StorageType) == provider {
			history = append(history, record)
		}
	}

	if len(history) < 3 {
		return nil, fmt.Errorf("insufficient historical data: need at least 3 records, got %d", len(history))
	}

	// 按时间排序
	sort.Slice(history, func(i, j int) bool {
		return history[i].Time.Before(history[j].Time)
	})

	// 根据方法生成预测
	var forecasts []ForecastPoint
	var accuracy float64
	var confidence ConfidenceLevel

	switch method {
	case MethodLinearRegression:
		forecasts, accuracy, confidence = m.linearRegression(history, horizon)
	case MethodExponentialSmoothing:
		forecasts, accuracy, confidence = m.exponentialSmoothing(history, horizon)
	case MethodMovingAverage:
		forecasts, accuracy, confidence = m.movingAverage(history, horizon)
	default:
		return nil, fmt.Errorf("unsupported prediction method: %s", method)
	}

	// 计算趋势
	trend := m.calculateTrend(history)
	growthRate := m.calculateGrowthRate(history)

	result := &ForecastResult{
		ID:             fmt.Sprintf("forecast-%s-%s-%d", method, horizon, time.Now().Unix()),
		Method:         method,
		Horizon:        horizon,
		HistoricalData: history,
		Forecasts:      forecasts,
		Accuracy:       accuracy,
		Confidence:     confidence,
		Trend:          trend,
		GrowthRate:     growthRate,
		GeneratedAt:    time.Now(),
	}

	return result, nil
}

// linearRegression 线性回归预测
func (m *Manager) linearRegression(history []CostRecord, horizon ForecastHorizon) ([]ForecastPoint, float64, ConfidenceLevel) {
	n := len(history)
	if n < 2 {
		return nil, 0, ConfidenceLow
	}

	// 计算线性回归系数
	var sumX, sumY, sumXY, sumX2 float64
	for i, record := range history {
		x := float64(i)
		y := record.Cost
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / float64(n)

	// 计算R²
	meanY := sumY / float64(n)
	var ssTotal, ssResidual float64
	for i, record := range history {
		predicted := slope*float64(i) + intercept
		ssTotal += (record.Cost - meanY) * (record.Cost - meanY)
		ssResidual += (record.Cost - predicted) * (record.Cost - predicted)
	}

	r2 := 1 - ssResidual/ssTotal
	accuracy := r2 * 100

	// 确定置信度
	var confidence ConfidenceLevel
	if accuracy >= 80 {
		confidence = ConfidenceHigh
	} else if accuracy >= 60 {
		confidence = ConfidenceMedium
	} else {
		confidence = ConfidenceLow
	}

	// 生成预测点
	months := getHorizonMonths(horizon)
	forecasts := make([]ForecastPoint, months)
	lastDate := history[n-1].Time

	for i := 0; i < months; i++ {
		x := float64(n + i)
		predicted := slope*x + intercept

		// 计算置信区间
		stdError := math.Sqrt(ssResidual / float64(n-2))
		margin := 1.96 * stdError * math.Sqrt(1+1/float64(n))

		date := lastDate.AddDate(0, i+1, 0)
		forecasts[i] = ForecastPoint{
			Date:       date,
			Cost:       predicted,
			Confidence: confidence,
			UpperBound: predicted + margin,
			LowerBound: math.Max(0, predicted-margin),
		}
	}

	return forecasts, accuracy, confidence
}

// exponentialSmoothing 指数平滑预测
func (m *Manager) exponentialSmoothing(history []CostRecord, horizon ForecastHorizon) ([]ForecastPoint, float64, ConfidenceLevel) {
	n := len(history)
	if n < 2 {
		return nil, 0, ConfidenceLow
	}

	alpha := 0.3 // 平滑系数
	smoothed := make([]float64, n)
	smoothed[0] = history[0].Cost

	// 计算指数平滑值
	for i := 1; i < n; i++ {
		smoothed[i] = alpha*history[i].Cost + (1-alpha)*smoothed[i-1]
	}

	// 计算准确度
	var totalError float64
	for i := 1; i < n; i++ {
		error := math.Abs(history[i].Cost - smoothed[i-1])
		totalError += error / history[i].Cost
	}
	mape := totalError / float64(n-1)
	accuracy := math.Max(0, (1-mape)*100)

	// 确定置信度
	var confidence ConfidenceLevel
	if accuracy >= 80 {
		confidence = ConfidenceHigh
	} else if accuracy >= 60 {
		confidence = ConfidenceMedium
	} else {
		confidence = ConfidenceLow
	}

	// 生成预测点
	months := getHorizonMonths(horizon)
	forecasts := make([]ForecastPoint, months)
	lastDate := history[n-1].Time
	lastSmoothed := smoothed[n-1]

	// 计算趋势
	trend := (smoothed[n-1] - smoothed[0]) / float64(n-1)

	for i := 0; i < months; i++ {
		predicted := lastSmoothed + trend*float64(i+1)
		margin := predicted * 0.1 // 10%误差范围

		date := lastDate.AddDate(0, i+1, 0)
		forecasts[i] = ForecastPoint{
			Date:       date,
			Cost:       predicted,
			Confidence: confidence,
			UpperBound: predicted + margin,
			LowerBound: math.Max(0, predicted-margin),
		}
	}

	return forecasts, accuracy, confidence
}

// movingAverage 移动平均预测
func (m *Manager) movingAverage(history []CostRecord, horizon ForecastHorizon) ([]ForecastPoint, float64, ConfidenceLevel) {
	n := len(history)
	window := 3 // 移动窗口
	if n < window {
		return nil, 0, ConfidenceLow
	}

	// 计算移动平均
	averages := make([]float64, n-window+1)
	for i := 0; i <= n-window; i++ {
		sum := 0.0
		for j := 0; j < window; j++ {
			sum += history[i+j].Cost
		}
		averages[i] = sum / float64(window)
	}

	// 计算准确度
	var totalError float64
	for i := window; i < n; i++ {
		error := math.Abs(history[i].Cost - averages[i-window])
		totalError += error / history[i].Cost
	}
	mape := totalError / float64(n-window)
	accuracy := math.Max(0, (1-mape)*100)

	// 确定置信度
	var confidence ConfidenceLevel
	if accuracy >= 80 {
		confidence = ConfidenceHigh
	} else if accuracy >= 60 {
		confidence = ConfidenceMedium
	} else {
		confidence = ConfidenceLow
	}

	// 生成预测点
	months := getHorizonMonths(horizon)
	forecasts := make([]ForecastPoint, months)
	lastDate := history[n-1].Time
	lastAvg := averages[len(averages)-1]

	// 计算趋势
	trend := 0.0
	if len(averages) >= 2 {
		trend = (averages[len(averages)-1] - averages[0]) / float64(len(averages)-1)
	}

	for i := 0; i < months; i++ {
		predicted := lastAvg + trend*float64(i+1)
		margin := predicted * 0.15 // 15%误差范围

		date := lastDate.AddDate(0, i+1, 0)
		forecasts[i] = ForecastPoint{
			Date:       date,
			Cost:       predicted,
			Confidence: confidence,
			UpperBound: predicted + margin,
			LowerBound: math.Max(0, predicted-margin),
		}
	}

	return forecasts, accuracy, confidence
}

// GetGrowthForecast 获取存储增长预测
func (m *Manager) GetGrowthForecast(provider string, capacityGB float64) (*GrowthForecast, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 筛选增长数据
	var history []GrowthRecord
	for _, record := range m.growthHistory {
		if provider == "" || record.Provider == provider {
			history = append(history, record)
		}
	}

	if len(history) < 2 {
		return nil, fmt.Errorf("insufficient growth data")
	}

	// 按时间排序
	sort.Slice(history, func(i, j int) bool {
		return history[i].Date.Before(history[j].Date)
	})

	// 计算平均增长率
	totalGrowthRate := 0.0
	for _, record := range history {
		totalGrowthRate += record.GrowthRate
	}
	avgGrowthRate := totalGrowthRate / float64(len(history))

	currentStorage := history[len(history)-1].StorageGB

	// 生成增长预测
	months := 12
	forecasts := make([]GrowthPoint, months)
	lastDate := history[len(history)-1].Date

	for i := 0; i < months; i++ {
		growthFactor := 1.0 + avgGrowthRate/100.0
		futureStorage := currentStorage * math.Pow(growthFactor, float64(i+1))

		date := lastDate.AddDate(0, i+1, 0)
		forecasts[i] = GrowthPoint{
			Date:      date,
			StorageGB: futureStorage,
			GrowthGB:  futureStorage - currentStorage,
		}
	}

	// 计算达到容量的时间
	daysToCapacity := 0
	if capacityGB > 0 && avgGrowthRate > 0 {
		monthsToCapacity := math.Log(capacityGB/currentStorage) / math.Log(1+avgGrowthRate/100)
		daysToCapacity = int(monthsToCapacity * 30)
	}

	result := &GrowthForecast{
		ID:              fmt.Sprintf("growth-%s-%d", provider, time.Now().Unix()),
		CurrentStorage:  currentStorage,
		Forecasts:       forecasts,
		GrowthRate:      avgGrowthRate,
		StorageCapacity: capacityGB,
		DaysToCapacity:  daysToCapacity,
		GeneratedAt:     time.Now(),
	}

	return result, nil
}

// SetAlertConfig 设置预算告警配置
func (m *Manager) SetAlertConfig(req AlertConfigRequest) (*AlertConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证阈值
	if req.WarningThreshold >= req.CriticalThreshold {
		return nil, fmt.Errorf("warning threshold must be less than critical threshold")
	}

	config := AlertConfig{
		ID:                fmt.Sprintf("alert-%d", time.Now().Unix()),
		Name:              req.Name,
		Type:              req.Type,
		Budget:            req.Budget,
		WarningThreshold:  req.WarningThreshold,
		CriticalThreshold: req.CriticalThreshold,
		Enabled:           req.Enabled,
		Provider:          req.Provider,
		Region:            req.Region,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	m.alertConfigs = append(m.alertConfigs, config)

	// 检查是否触发告警
	m.checkAlerts()

	log.Printf("Alert config created: %s (budget: $%.2f)", config.Name, config.Budget)
	return &config, nil
}

// GetAlertConfigs 获取告警配置列表
func (m *Manager) GetAlertConfigs() []AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertConfigs
}

// GetActiveAlerts 获取活跃告警
func (m *Manager) GetActiveAlerts() []BudgetAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeAlerts
}

// checkAlerts 检查告警
func (m *Manager) checkAlerts() {
	for _, config := range m.alertConfigs {
		if !config.Enabled {
			continue
		}

		// 计算当前成本
		currentCost := 0.0
		for _, record := range m.costHistory {
			if config.Provider == "" || string(record.StorageType) == config.Provider {
				currentCost += record.Cost
			}
		}

		// 检查阈值
		utilization := (currentCost / config.Budget) * 100

		if utilization >= config.CriticalThreshold {
			alert := BudgetAlert{
				Department:     "default",
				Project:        "nas-os",
				BudgetAmount:   config.Budget,
				PredictedCost:  currentCost,
				OverrunAmount:  currentCost - config.Budget,
				OverrunPercent: utilization,
				AlertLevel:     "critical",
			}
			m.activeAlerts = append(m.activeAlerts, alert)
		} else if utilization >= config.WarningThreshold {
			alert := BudgetAlert{
				Department:     "default",
				Project:        "nas-os",
				BudgetAmount:   config.Budget,
				PredictedCost:  currentCost,
				OverrunAmount:  0,
				OverrunPercent: utilization,
				AlertLevel:     "warning",
			}
			m.activeAlerts = append(m.activeAlerts, alert)
		}
	}
}

// GetReport 获取预测报告
func (m *Manager) GetReport(provider string) (*PredictionReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取成本预测
	costForecast, err := m.getForecastInternal(MethodLinearRegression, Horizon1Year, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cost forecast: %w", err)
	}

	// 获取增长预测
	growthForecast, err := m.getGrowthForecastInternal(provider, 10000) // 10TB容量
	if err != nil {
		return nil, fmt.Errorf("failed to generate growth forecast: %w", err)
	}

	// 计算当前成本
	currentCost := 0.0
	for _, record := range m.costHistory {
		if provider == "" || string(record.StorageType) == provider {
			currentCost += record.Cost
		}
	}

	// 获取活跃告警
	activeAlerts := m.getActiveAlertsInternal()

	// 生成建议
	recommendations := m.generateRecommendations(currentCost, growthForecast)

	report := &PredictionReport{
		ID:    fmt.Sprintf("report-%s-%d", provider, time.Now().Unix()),
		Title: fmt.Sprintf("成本预测报告 - %s", provider),
		Summary: ReportSummary{
			CurrentMonthlyCost: currentCost,
			ForecastedCost:     costForecast.Forecasts[len(costForecast.Forecasts)-1].Cost,
			CostChange:         costForecast.GrowthRate,
			CurrentStorage:     growthForecast.CurrentStorage,
			ForecastedStorage:  growthForecast.Forecasts[len(growthForecast.Forecasts)-1].StorageGB,
			GrowthRate:         growthForecast.GrowthRate,
			BudgetUtilization:  (currentCost / 1000) * 100, // 假设1000预算
			ActiveAlerts:       len(activeAlerts),
		},
		CostForecast:    *costForecast,
		GrowthForecast:  *growthForecast,
		Alerts:          activeAlerts,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
		ValidUntil:      time.Now().AddDate(0, 1, 0), // 1个月有效
	}

	return report, nil
}

// getForecastInternal 内部获取预测
func (m *Manager) getForecastInternal(method PredictionMethod, horizon ForecastHorizon, provider string) (*ForecastResult, error) {
	var history []CostRecord
	for _, record := range m.costHistory {
		if provider == "" || string(record.StorageType) == provider {
			history = append(history, record)
		}
	}

	if len(history) < 3 {
		return nil, fmt.Errorf("insufficient data")
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Time.Before(history[j].Time)
	})

	forecasts, accuracy, confidence := m.linearRegression(history, horizon)

	return &ForecastResult{
		ID:             fmt.Sprintf("forecast-%d", time.Now().Unix()),
		Method:         method,
		Horizon:        horizon,
		HistoricalData: history,
		Forecasts:      forecasts,
		Accuracy:       accuracy,
		Confidence:     confidence,
		Trend:          m.calculateTrend(history),
		GrowthRate:     m.calculateGrowthRate(history),
		GeneratedAt:    time.Now(),
	}, nil
}

// getGrowthForecastInternal 内部获取增长预测
func (m *Manager) getGrowthForecastInternal(provider string, capacityGB float64) (*GrowthForecast, error) {
	var history []GrowthRecord
	for _, record := range m.growthHistory {
		if provider == "" || record.Provider == provider {
			history = append(history, record)
		}
	}

	if len(history) < 2 {
		return nil, fmt.Errorf("insufficient data")
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Date.Before(history[j].Date)
	})

	totalGrowthRate := 0.0
	for _, record := range history {
		totalGrowthRate += record.GrowthRate
	}
	avgGrowthRate := totalGrowthRate / float64(len(history))

	currentStorage := history[len(history)-1].StorageGB
	months := 12
	forecasts := make([]GrowthPoint, months)
	lastDate := history[len(history)-1].Date

	for i := 0; i < months; i++ {
		growthFactor := 1.0 + avgGrowthRate/100.0
		futureStorage := currentStorage * math.Pow(growthFactor, float64(i+1))
		date := lastDate.AddDate(0, i+1, 0)
		forecasts[i] = GrowthPoint{
			Date:      date,
			StorageGB: futureStorage,
			GrowthGB:  futureStorage - currentStorage,
		}
	}

	daysToCapacity := 0
	if capacityGB > 0 && avgGrowthRate > 0 {
		monthsToCapacity := math.Log(capacityGB/currentStorage) / math.Log(1+avgGrowthRate/100)
		daysToCapacity = int(monthsToCapacity * 30)
	}

	return &GrowthForecast{
		ID:              fmt.Sprintf("growth-%d", time.Now().Unix()),
		CurrentStorage:  currentStorage,
		Forecasts:       forecasts,
		GrowthRate:      avgGrowthRate,
		StorageCapacity: capacityGB,
		DaysToCapacity:  daysToCapacity,
		GeneratedAt:     time.Now(),
	}, nil
}

// getActiveAlertsInternal 内部获取活跃告警
func (m *Manager) getActiveAlertsInternal() []BudgetAlert {
	return m.activeAlerts
}

// generateRecommendations 生成建议
func (m *Manager) generateRecommendations(currentCost float64, growth *GrowthForecast) []string {
	var recommendations []string

	if growth.GrowthRate > 5.0 {
		recommendations = append(recommendations, "存储增长速度较快，建议评估数据生命周期策略")
	}

	if currentCost > 500 {
		recommendations = append(recommendations, "考虑使用低频访问存储降低成本")
		recommendations = append(recommendations, "评估是否有未使用的存储可以清理")
	}

	if growth.DaysToCapacity > 0 && growth.DaysToCapacity < 180 {
		recommendations = append(recommendations, fmt.Sprintf("预计 %.0f 天内达到存储容量上限，建议提前扩容", float64(growth.DaysToCapacity)))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前成本和增长趋势正常")
	}

	return recommendations
}

// calculateTrend 计算趋势
func (m *Manager) calculateTrend(history []CostRecord) string {
	if len(history) < 2 {
		return "stable"
	}

	first := history[0].Cost
	last := history[len(history)-1].Cost
	change := ((last - first) / first) * 100

	if change > 10 {
		return "increasing"
	} else if change < -10 {
		return "decreasing"
	}
	return "stable"
}

// calculateGrowthRate 计算增长率
func (m *Manager) calculateGrowthRate(history []CostRecord) float64 {
	if len(history) < 2 {
		return 0
	}

	totalGrowth := 0.0
	for i := 1; i < len(history); i++ {
		growth := ((history[i].Cost - history[i-1].Cost) / history[i-1].Cost) * 100
		totalGrowth += growth
	}

	return totalGrowth / float64(len(history)-1)
}

// getHorizonMonths 获取预测月数
func getHorizonMonths(horizon ForecastHorizon) int {
	switch horizon {
	case Horizon3Months:
		return 3
	case Horizon6Months:
		return 6
	case Horizon1Year:
		return 12
	case Horizon2Years:
		return 24
	default:
		return 12
	}
}

// AddCostRecord 添加成本记录
func (m *Manager) AddCostRecord(record CostRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.costHistory = append(m.costHistory, record)
	m.checkAlerts()
	log.Printf("Added cost record: $%.2f for %s", record.Cost, record.StorageType)
}
