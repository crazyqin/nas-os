// Package smartenergyforecast 提供智能能耗预测功能
package smartenergyforecast

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// SmartEnergyForecast 智能耗预测器
type SmartEnergyForecast struct {
	mu            sync.RWMutex
	config        *Config
	readings      []*EnergyReading
	forecasts     []*EnergyForecast
	anomalies     []*EnergyAnomaly
	optimizations []*EnergyOptimization
	budget        *EnergyBudget
	dailyReports  map[string]*DailyReport
	running       bool
	stopCh        chan struct{}
}

// Config 配置
type Config struct {
	ForecastWindow     time.Duration `json:"forecast_window"`       // 预测窗口
	HistoryDepth       time.Duration `json:"history_depth"`         // 历史深度
	AnomalyThreshold   float64       `json:"anomaly_threshold"`     // 异常阈值（标准差倍数）
	BudgetAlertRatio   float64       `json:"budget_alert_ratio"`    // 预算告警比例
	MinDataPoints      int           `json:"min_data_points"`       // 最小数据点数
	SmoothingFactor    float64       `json:"smoothing_factor"`      // 平滑因子（0-1）
	UpdateInterval     time.Duration `json:"update_interval"`       // 更新间隔
	DefaultPricePerKwh float64       `json:"default_price_per_kwh"` // 默认电价
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ForecastWindow:     24 * time.Hour,
		HistoryDepth:       7 * 24 * time.Hour,
		AnomalyThreshold:   2.0,
		BudgetAlertRatio:   0.8,
		MinDataPoints:      10,
		SmoothingFactor:    0.3,
		UpdateInterval:     time.Hour,
		DefaultPricePerKwh: 0.56,
	}
}

// EnergyReading 能耗读数
type EnergyReading struct {
	Timestamp   time.Time `json:"timestamp"`
	Consumption float64   `json:"consumption"`  // 千瓦时
	Device      string    `json:"device"`       // 设备名称
	Category    string    `json:"category"`     // 类别
	PowerFactor float64   `json:"power_factor"` // 功率因数
	Voltage     float64   `json:"voltage"`      // 电压
	Current     float64   `json:"current"`      // 电流
}

// EnergyForecast 能耗预测
type EnergyForecast struct {
	Timestamp   time.Time     `json:"timestamp"`
	Period      time.Duration `json:"period"`
	Expected    float64       `json:"expected"`   // 预期能耗（kWh）
	Lower       float64       `json:"lower"`      // 下限
	Upper       float64       `json:"upper"`      // 上限
	Confidence  float64       `json:"confidence"` // 置信度
	Trend       string        `json:"trend"`      // 趋势：rising/stable/falling
	GeneratedAt time.Time     `json:"generated_at"`
}

// EnergyAnomaly 能耗异常
type EnergyAnomaly struct {
	Timestamp   time.Time      `json:"timestamp"`
	Reading     *EnergyReading `json:"reading"`
	Expected    float64        `json:"expected"`
	Actual      float64        `json:"actual"`
	Deviation   float64        `json:"deviation"` // 偏差百分比
	Severity    string         `json:"severity"`  // high/medium/low
	Description string         `json:"description"`
	DetectedAt  time.Time      `json:"detected_at"`
}

// EnergyOptimization 节能优化建议
type EnergyOptimization struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"` // 类别
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Savings     float64   `json:"savings"`     // 预计节省（kWh）
	CostSaving  float64   `json:"cost_saving"` // 预计节省费用
	Priority    string    `json:"priority"`    // high/medium/low
	Device      string    `json:"device"`      // 相关设备
	CreatedAt   time.Time `json:"created_at"`
}

// EnergyBudget 能耗预算
type EnergyBudget struct {
	DailyBudget   float64 `json:"daily_budget"`   // 每日预算（kWh）
	WeeklyBudget  float64 `json:"weekly_budget"`  // 每周预算
	MonthlyBudget float64 `json:"monthly_budget"` // 每月预算
	PricePerKwh   float64 `json:"price_per_kwh"`  // 电价
}

// BudgetStatus 预算状态
type BudgetStatus struct {
	DailyUsed     float64 `json:"daily_used"`
	DailyBudget   float64 `json:"daily_budget"`
	DailyRatio    float64 `json:"daily_ratio"`
	WeeklyUsed    float64 `json:"weekly_used"`
	WeeklyBudget  float64 `json:"weekly_budget"`
	WeeklyRatio   float64 `json:"weekly_ratio"`
	MonthlyUsed   float64 `json:"monthly_used"`
	MonthlyBudget float64 `json:"monthly_budget"`
	MonthlyRatio  float64 `json:"monthly_ratio"`
	Status        string  `json:"status"` // normal/warning/critical
}

// DailyReport 每日报告
type DailyReport struct {
	Date            time.Time          `json:"date"`
	TotalConsumed   float64            `json:"total_consumed"`
	AvgConsumption  float64            `json:"avg_consumption"`
	PeakConsumption float64            `json:"peak_consumption"`
	PeakTime        time.Time          `json:"peak_time"`
	DeviceBreakdown map[string]float64 `json:"device_breakdown"`
	AnomaliesCount  int                `json:"anomalies_count"`
	CostEstimate    float64            `json:"cost_estimate"`
}

// WeeklyTrend 每周趋势
type WeeklyTrend struct {
	WeekStart      time.Time `json:"week_start"`
	WeekEnd        time.Time `json:"week_end"`
	DailyData      []DayData `json:"daily_data"`
	TotalConsumed  float64   `json:"total_consumed"`
	AvgDaily       float64   `json:"avg_daily"`
	TrendDirection string    `json:"trend_direction"`
	ComparedToPrev float64   `json:"compared_to_prev"` // 与上周对比百分比
}

// DayData 每日数据
type DayData struct {
	Date        time.Time `json:"date"`
	Consumption float64   `json:"consumption"`
	Cost        float64   `json:"cost"`
}

// New 创建智能能耗预测器
func New(config *Config) *SmartEnergyForecast {
	if config == nil {
		config = DefaultConfig()
	}
	return &SmartEnergyForecast{
		config:        config,
		readings:      make([]*EnergyReading, 0),
		forecasts:     make([]*EnergyForecast, 0),
		anomalies:     make([]*EnergyAnomaly, 0),
		optimizations: make([]*EnergyOptimization, 0),
		dailyReports:  make(map[string]*DailyReport),
		stopCh:        make(chan struct{}),
	}
}

// Start 启动预测器
func (s *SmartEnergyForecast) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("predictor already running")
	}

	s.running = true
	go s.backgroundTask(ctx)

	return nil
}

// Stop 停止预测器
func (s *SmartEnergyForecast) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopCh)
}

// RecordReading 记录能耗数据
func (s *SmartEnergyForecast) RecordReading(reading *EnergyReading) error {
	if reading == nil {
		return fmt.Errorf("reading cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.readings = append(s.readings, reading)

	// 裁剪旧数据
	cutoff := time.Now().Add(-s.config.HistoryDepth)
	newReadings := make([]*EnergyReading, 0)
	for _, r := range s.readings {
		if r.Timestamp.After(cutoff) {
			newReadings = append(newReadings, r)
		}
	}
	s.readings = newReadings

	return nil
}

// Forecast 预测未来能耗
func (s *SmartEnergyForecast) Forecast(ctx context.Context, duration time.Duration) ([]*EnergyForecast, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.readings) < s.config.MinDataPoints {
		return nil, fmt.Errorf("insufficient data points: have %d, need %d", len(s.readings), s.config.MinDataPoints)
	}

	// 计算历史统计
	stats := s.calculateStats()

	// 生成预测
	forecasts := make([]*EnergyForecast, 0)
	now := time.Now()

	// 按小时生成预测
	hours := int(duration.Hours())
	if hours == 0 {
		hours = 1
	}

	for h := 1; h <= hours; h++ {
		forecastTime := now.Add(time.Duration(h) * time.Hour)

		// 使用指数平滑预测
		expected := s.exponentialSmoothing(stats, h)

		// 计算置信区间
		stdDev := stats.StdDev
		lower := expected - 1.96*stdDev
		if lower < 0 {
			lower = 0
		}
		upper := expected + 1.96*stdDev

		// 计算置信度（随时间衰减）
		confidence := math.Max(0.5, 1.0-float64(h)*0.02)

		// 判断趋势
		trend := s.determineTrend(stats)

		forecast := &EnergyForecast{
			Timestamp:   forecastTime,
			Period:      time.Hour,
			Expected:    expected,
			Lower:       lower,
			Upper:       upper,
			Confidence:  confidence,
			Trend:       trend,
			GeneratedAt: now,
		}
		forecasts = append(forecasts, forecast)
	}

	return forecasts, nil
}

// DetectAnomalies 检测能耗异常
func (s *SmartEnergyForecast) DetectAnomalies(ctx context.Context) ([]*EnergyAnomaly, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.readings) < s.config.MinDataPoints {
		return nil, fmt.Errorf("insufficient data points")
	}

	stats := s.calculateStats()
	anomalies := make([]*EnergyAnomaly, 0)

	// 检测最近的读数
	threshold := s.config.AnomalyThreshold
	cutoff := time.Now().Add(-24 * time.Hour)

	for _, reading := range s.readings {
		if reading.Timestamp.Before(cutoff) {
			continue
		}

		deviation := math.Abs(reading.Consumption-stats.Mean) / stats.StdDev

		if deviation > threshold {
			severity := "low"
			if deviation > threshold*2 {
				severity = "high"
			} else if deviation > threshold*1.5 {
				severity = "medium"
			}

			expected := stats.Mean
			actual := reading.Consumption
			devPercent := (actual - expected) / expected * 100

			description := fmt.Sprintf("能耗异常: 实际 %.2f kWh, 预期 %.2f kWh, 偏差 %.1f%%",
				actual, expected, devPercent)

			anomaly := &EnergyAnomaly{
				Timestamp:   reading.Timestamp,
				Reading:     reading,
				Expected:    expected,
				Actual:      actual,
				Deviation:   devPercent,
				Severity:    severity,
				Description: description,
				DetectedAt:  time.Now(),
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	s.anomalies = append(s.anomalies, anomalies...)
	return anomalies, nil
}

// GetOptimizations 获取优化建议
func (s *SmartEnergyForecast) GetOptimizations() []*EnergyOptimization {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.optimizations) == 0 {
		s.mu.RUnlock()
		s.mu.Lock()
		s.generateOptimizations()
		s.mu.Unlock()
		s.mu.RLock()
	}

	return s.optimizations
}

// GetDailyReport 获取每日报告
func (s *SmartEnergyForecast) GetDailyReport() *DailyReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	today := time.Now().Format("2006-01-02")
	if report, ok := s.dailyReports[today]; ok {
		return report
	}

	// 生成今日报告
	s.mu.RUnlock()
	s.mu.Lock()
	report := s.generateDailyReport(time.Now())
	s.dailyReports[today] = report
	s.mu.Unlock()
	s.mu.RLock()

	return report
}

// GetWeeklyTrend 获取每周趋势
func (s *SmartEnergyForecast) GetWeeklyTrend() *WeeklyTrend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	weekEnd := weekStart.AddDate(0, 0, 6)

	dailyData := make([]DayData, 0)
	totalConsumed := 0.0

	for d := 0; d < 7; d++ {
		day := weekStart.AddDate(0, 0, d)
		consumption := s.getConsumptionForDay(day)
		cost := consumption * s.config.DefaultPricePerKwh

		dailyData = append(dailyData, DayData{
			Date:        day,
			Consumption: consumption,
			Cost:        cost,
		})
		totalConsumed += consumption
	}

	avgDaily := totalConsumed / 7
	prevWeekConsumption := s.getConsumptionForPeriod(weekStart.AddDate(0, 0, -7), weekStart)
	comparison := 0.0
	if prevWeekConsumption > 0 {
		comparison = (totalConsumed - prevWeekConsumption) / prevWeekConsumption * 100
	}

	trendDirection := "stable"
	if comparison > 5 {
		trendDirection = "rising"
	} else if comparison < -5 {
		trendDirection = "falling"
	}

	return &WeeklyTrend{
		WeekStart:      weekStart,
		WeekEnd:        weekEnd,
		DailyData:      dailyData,
		TotalConsumed:  totalConsumed,
		AvgDaily:       avgDaily,
		TrendDirection: trendDirection,
		ComparedToPrev: comparison,
	}
}

// SetBudget 设置能耗预算
func (s *SmartEnergyForecast) SetBudget(budget *EnergyBudget) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.budget = budget
}

// GetBudgetStatus 获取预算状态
func (s *SmartEnergyForecast) GetBudgetStatus() *BudgetStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.budget == nil {
		return &BudgetStatus{
			Status: "no_budget",
		}
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	dailyUsed := s.getConsumptionForPeriod(today, now)
	weeklyUsed := s.getConsumptionForPeriod(weekStart, now)
	monthlyUsed := s.getConsumptionForPeriod(monthStart, now)

	dailyRatio := dailyUsed / s.budget.DailyBudget
	weeklyRatio := weeklyUsed / s.budget.WeeklyBudget
	monthlyRatio := monthlyUsed / s.budget.MonthlyBudget

	status := "normal"
	maxRatio := math.Max(dailyRatio, math.Max(weeklyRatio, monthlyRatio))
	if maxRatio >= 1.0 {
		status = "critical"
	} else if maxRatio >= s.config.BudgetAlertRatio {
		status = "warning"
	}

	return &BudgetStatus{
		DailyUsed:     dailyUsed,
		DailyBudget:   s.budget.DailyBudget,
		DailyRatio:    dailyRatio,
		WeeklyUsed:    weeklyUsed,
		WeeklyBudget:  s.budget.WeeklyBudget,
		WeeklyRatio:   weeklyRatio,
		MonthlyUsed:   monthlyUsed,
		MonthlyBudget: s.budget.MonthlyBudget,
		MonthlyRatio:  monthlyRatio,
		Status:        status,
	}
}

// CalculateCost 计算电费
func (s *SmartEnergyForecast) CalculateCost(reading *EnergyReading, pricePerKwh float64) float64 {
	if reading == nil || pricePerKwh <= 0 {
		return 0
	}
	return reading.Consumption * pricePerKwh
}

// stats 统计信息
type stats struct {
	Mean   float64
	StdDev float64
	Min    float64
	Max    float64
	Count  int
}

// calculateStats 计算统计数据
func (s *SmartEnergyForecast) calculateStats() *stats {
	if len(s.readings) == 0 {
		return &stats{}
	}

	sum := 0.0
	minVal := math.MaxFloat64
	maxVal := -math.MaxFloat64

	for _, r := range s.readings {
		sum += r.Consumption
		if r.Consumption < minVal {
			minVal = r.Consumption
		}
		if r.Consumption > maxVal {
			maxVal = r.Consumption
		}
	}

	mean := sum / float64(len(s.readings))

	// 计算标准差
	sumSquaredDiff := 0.0
	for _, r := range s.readings {
		diff := r.Consumption - mean
		sumSquaredDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSquaredDiff / float64(len(s.readings)))

	return &stats{
		Mean:   mean,
		StdDev: stdDev,
		Min:    minVal,
		Max:    maxVal,
		Count:  len(s.readings),
	}
}

// exponentialSmoothing 指数平滑预测
func (s *SmartEnergyForecast) exponentialSmoothing(stats *stats, stepsAhead int) float64 {
	alpha := s.config.SmoothingFactor

	// 使用历史数据进行指数平滑
	smoothed := stats.Mean
	for i := 1; i <= stepsAhead; i++ {
		// 简化：使用均值作为新观测值
		smoothed = alpha*stats.Mean + (1-alpha)*smoothed
	}

	return smoothed
}

// determineTrend 判断趋势
func (s *SmartEnergyForecast) determineTrend(stats *stats) string {
	if len(s.readings) < 2 {
		return "stable"
	}

	// 比较最近一半和前一半的数据
	mid := len(s.readings) / 2
	firstHalfSum := 0.0
	secondHalfSum := 0.0

	for i := 0; i < mid; i++ {
		firstHalfSum += s.readings[i].Consumption
	}
	for i := mid; i < len(s.readings); i++ {
		secondHalfSum += s.readings[i].Consumption
	}

	firstHalfAvg := firstHalfSum / float64(mid)
	secondHalfAvg := secondHalfSum / float64(len(s.readings)-mid)

	change := (secondHalfAvg - firstHalfAvg) / firstHalfAvg * 100

	if change > 10 {
		return "rising"
	} else if change < -10 {
		return "falling"
	}
	return "stable"
}

// getConsumptionForDay 获取某天的能耗
func (s *SmartEnergyForecast) getConsumptionForDay(day time.Time) float64 {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	total := 0.0
	for _, r := range s.readings {
		if r.Timestamp.After(dayStart) && r.Timestamp.Before(dayEnd) {
			total += r.Consumption
		}
	}
	return total
}

// getConsumptionForPeriod 获取某段时间的能耗
func (s *SmartEnergyForecast) getConsumptionForPeriod(start, end time.Time) float64 {
	total := 0.0
	for _, r := range s.readings {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			total += r.Consumption
		}
	}
	return total
}

// generateOptimizations 生成优化建议
func (s *SmartEnergyForecast) generateOptimizations() {
	if len(s.readings) < s.config.MinDataPoints {
		return
	}

	// 分析设备能耗
	deviceConsumption := make(map[string]float64)
	deviceReadings := make(map[string][]*EnergyReading)

	for _, r := range s.readings {
		deviceConsumption[r.Device] += r.Consumption
		deviceReadings[r.Device] = append(deviceReadings[r.Device], r)
	}

	id := 1
	for device, consumption := range deviceConsumption {
		avgConsumption := consumption / float64(len(deviceReadings[device]))

		// 检查高能耗设备
		if avgConsumption > 1.0 { // 超过1kWh
			s.optimizations = append(s.optimizations, &EnergyOptimization{
				ID:          fmt.Sprintf("OPT-%03d", id),
				Category:    "设备优化",
				Title:       fmt.Sprintf("优化 %s 设备能耗", device),
				Description: fmt.Sprintf("%s 平均能耗 %.2f kWh，建议检查设备运行模式", device, avgConsumption),
				Savings:     avgConsumption * 0.15,
				CostSaving:  avgConsumption * 0.15 * s.config.DefaultPricePerKwh,
				Priority:    "high",
				Device:      device,
				CreatedAt:   time.Now(),
			})
			id++
		}

		// 检查功率因数
		avgPF := 0.0
		for _, r := range deviceReadings[device] {
			avgPF += r.PowerFactor
		}
		avgPF /= float64(len(deviceReadings[device]))

		if avgPF < 0.85 && avgPF > 0 {
			s.optimizations = append(s.optimizations, &EnergyOptimization{
				ID:          fmt.Sprintf("OPT-%03d", id),
				Category:    "功率因数",
				Title:       fmt.Sprintf("改善 %s 功率因数", device),
				Description: fmt.Sprintf("%s 平均功率因数 %.2f，建议安装功率因数补偿装置", device, avgPF),
				Savings:     consumption * 0.05,
				CostSaving:  consumption * 0.05 * s.config.DefaultPricePerKwh,
				Priority:    "medium",
				Device:      device,
				CreatedAt:   time.Now(),
			})
			id++
		}
	}

	// 添加通用建议
	s.optimizations = append(s.optimizations, &EnergyOptimization{
		ID:          fmt.Sprintf("OPT-%03d", id),
		Category:    "使用习惯",
		Title:       "优化用电时间",
		Description: "建议将高能耗设备运行时间调整到电价低谷时段",
		Savings:     2.0,
		CostSaving:  2.0 * s.config.DefaultPricePerKwh * 0.5,
		Priority:    "medium",
		Device:      "",
		CreatedAt:   time.Now(),
	})
}

// generateDailyReport 生成每日报告
func (s *SmartEnergyForecast) generateDailyReport(day time.Time) *DailyReport {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	totalConsumed := 0.0
	peakConsumption := 0.0
	peakTime := dayStart
	deviceBreakdown := make(map[string]float64)
	count := 0

	for _, r := range s.readings {
		if r.Timestamp.After(dayStart) && r.Timestamp.Before(dayEnd) {
			totalConsumed += r.Consumption
			deviceBreakdown[r.Device] += r.Consumption
			count++

			if r.Consumption > peakConsumption {
				peakConsumption = r.Consumption
				peakTime = r.Timestamp
			}
		}
	}

	avgConsumption := 0.0
	if count > 0 {
		avgConsumption = totalConsumed / float64(count)
	}

	anomaliesCount := 0
	for _, a := range s.anomalies {
		if a.Timestamp.After(dayStart) && a.Timestamp.Before(dayEnd) {
			anomaliesCount++
		}
	}

	return &DailyReport{
		Date:            day,
		TotalConsumed:   totalConsumed,
		AvgConsumption:  avgConsumption,
		PeakConsumption: peakConsumption,
		PeakTime:        peakTime,
		DeviceBreakdown: deviceBreakdown,
		AnomaliesCount:  anomaliesCount,
		CostEstimate:    totalConsumed * s.config.DefaultPricePerKwh,
	}
}

// backgroundTask 后台任务
func (s *SmartEnergyForecast) backgroundTask(ctx context.Context) {
	ticker := time.NewTicker(s.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runBackgroundUpdate()
		}
	}
}

// runBackgroundUpdate 执行后台更新
func (s *SmartEnergyForecast) runBackgroundUpdate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 生成优化建议
	s.optimizations = make([]*EnergyOptimization, 0)
	s.generateOptimizations()

	// 生成每日报告
	report := s.generateDailyReport(time.Now())
	today := time.Now().Format("2006-01-02")
	s.dailyReports[today] = report
}
