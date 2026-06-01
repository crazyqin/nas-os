// Package nasprognostics NAS 健康预后引擎
// 对标群晖存储管理器的 S.M.A.R.T 预测、TrueNAS 的 Self-Healing
// 基于历史数据和趋势分析，预测硬件故障、容量瓶颈、性能退化
package nasprognostics

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ComponentType 组件类型
type ComponentType string

const (
	ComponentDisk     ComponentType = "disk"
	ComponentCPU      ComponentType = "cpu"
	ComponentMemory   ComponentType = "memory"
	ComponentNetwork  ComponentType = "network"
	ComponentPSU      ComponentType = "psu"
	ComponentFan      ComponentType = "fan"
	ComponentRAID     ComponentType = "raid"
	ComponentPool     ComponentType = "pool"
)

// HealthLevel 健康等级
type HealthLevel string

const (
	HealthExcellent HealthLevel = "excellent"
	HealthGood      HealthLevel = "good"
	HealthFair      HealthLevel = "fair"
	HealthPoor      HealthLevel = "poor"
	HealthCritical  HealthLevel = "critical"
)

// FailureRisk 故障风险等级
type FailureRisk string

const (
	RiskLow      FailureRisk = "low"
	RiskMedium   FailureRisk = "medium"
	RiskHigh     FailureRisk = "high"
	RiskCritical FailureRisk = "critical"
)

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendImproving TrendDirection = "improving"
	TrendStable    TrendDirection = "stable"
	TrendDegrading TrendDirection = "degrading"
)

// Component 组件信息
type Component struct {
	ID           string        `json:"id"`
	Type         ComponentType `json:"type"`
	Name         string        `json:"name"`
	Model        string        `json:"model,omitempty"`
	SerialNumber string        `json:"serial_number,omitempty"`
	HealthLevel  HealthLevel   `json:"health_level"`
	HealthScore  float64       `json:"health_score"` // 0-100
	FailureRisk  FailureRisk   `json:"failure_risk"`
	Trend        TrendDirection `json:"trend"`
	Metrics      []Metric      `json:"metrics"`
	Predictions  []Prediction  `json:"predictions"`
	LastChecked  time.Time     `json:"last_checked"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Metric 指标数据点
type Metric struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
}

// Prediction 预测结果
type Prediction struct {
	ID          string        `json:"id"`
	ComponentID string        `json:"component_id"`
	Type        string        `json:"type"`        // failure, capacity, performance
	Probability float64       `json:"probability"` // 0.0-1.0
	TimeToEvent time.Duration `json:"time_to_event"`
	Description string        `json:"description"`
	Confidence  float64       `json:"confidence"`  // 0.0-1.0
	GeneratedAt time.Time     `json:"generated_at"`
	Actions     []string      `json:"actions"`
}

// CapacityForecast 容量预测
type CapacityForecast struct {
	ComponentID   string    `json:"component_id"`
	ComponentName string    `json:"component_name"`
	CurrentUsage  float64   `json:"current_usage"`  // 0.0-1.0
	TotalBytes    uint64    `json:"total_bytes"`
	UsedBytes     uint64    `json:"used_bytes"`
	GrowthRate    float64   `json:"growth_rate"`    // bytes/day
	DaysRemaining int       `json:"days_remaining"`
	FullDate      time.Time `json:"full_date"`
	Confidence    float64   `json:"confidence"`
}

// PerformanceTrend 性能趋势
type PerformanceTrend struct {
	ComponentID string        `json:"component_id"`
	MetricName  string        `json:"metric_name"`
	Current     float64       `json:"current"`
	Average     float64       `json:"average"`
	Min         float64       `json:"min"`
	Max         float64       `json:"max"`
	Trend       TrendDirection `json:"trend"`
	ChangeRate  float64       `json:"change_rate"` // 每天变化率
	Period      string        `json:"period"`      // 分析周期
}

// PrognosticsReport 预后报告
type PrognosticsReport struct {
	ID             string              `json:"id"`
	GeneratedAt    time.Time           `json:"generated_at"`
	OverallHealth  HealthLevel         `json:"overall_health"`
	OverallScore   float64             `json:"overall_score"`
	Components     []Component         `json:"components"`
	Predictions    []Prediction        `json:"predictions"`
	Forecasts      []CapacityForecast  `json:"forecasts"`
	Trends         []PerformanceTrend  `json:"trends"`
	Recommendations []Recommendation   `json:"recommendations"`
	Summary        string              `json:"summary"`
}

// Recommendation 建议
type Recommendation struct {
	ID          string `json:"id"`
	Priority    int    `json:"priority"` // 1-5
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
}

// Manager 预后管理器
type Manager struct {
	mu          sync.RWMutex
	components  map[string]*Component
	predictions map[string]*Prediction
	reports     []*PrognosticsReport
	history     map[string][]Metric // 组件指标历史
	cancelFunc  context.CancelFunc
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		components:  make(map[string]*Component),
		predictions: make(map[string]*Prediction),
		reports:     make([]*PrognosticsReport, 0),
		history:     make(map[string][]Metric),
	}
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	go m.collectLoop(ctx)
	go m.predictLoop(ctx)

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// RegisterComponent 注册组件
func (m *Manager) RegisterComponent(comp *Component) {
	m.mu.Lock()
	defer m.mu.Unlock()
	comp.LastChecked = time.Now()
	m.components[comp.ID] = comp
}

// GetComponent 获取组件
func (m *Manager) GetComponent(id string) (*Component, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	comp, ok := m.components[id]
	if !ok {
		return nil, fmt.Errorf("component %s not found", id)
	}
	return comp, nil
}

// ListComponents 列出组件
func (m *Manager) ListComponents(compType ComponentType) []*Component {
	m.mu.RLock()
	defer m.mu.RUnlock()
	comps := make([]*Component, 0)
	for _, c := range m.components {
		if compType != "" && c.Type != compType {
			continue
		}
		comps = append(comps, c)
	}
	return comps
}

// RecordMetric 记录指标
func (m *Manager) RecordMetric(componentID string, metric Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	key := componentID + ":" + metric.Name
	m.history[key] = append(m.history[key], metric)

	// 保留最近 1000 个数据点
	if len(m.history[key]) > 1000 {
		m.history[key] = m.history[key][len(m.history[key])-1000:]
	}

	// 更新组件当前指标
	if comp, ok := m.components[componentID]; ok {
		found := false
		for i, m := range comp.Metrics {
			if m.Name == metric.Name {
				comp.Metrics[i] = metric
				found = true
				break
			}
		}
		if !found {
			comp.Metrics = append(comp.Metrics, metric)
		}
		comp.LastChecked = time.Now()
	}
}

// GetPredictions 获取预测
func (m *Manager) GetPredictions(componentID string) []Prediction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	preds := make([]Prediction, 0)
	for _, p := range m.predictions {
		if componentID != "" && p.ComponentID != componentID {
			continue
		}
		preds = append(preds, *p)
	}
	return preds
}

// ForecastCapacity 容量预测
func (m *Manager) ForecastCapacity(componentID string) (*CapacityForecast, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	comp, ok := m.components[componentID]
	if !ok {
		return nil, fmt.Errorf("component %s not found", componentID)
	}

	// 查找容量相关指标
	var totalBytes, usedBytes uint64
	var utilization float64

	for _, metric := range comp.Metrics {
		switch metric.Name {
		case "total_bytes":
			totalBytes = uint64(metric.Value)
		case "used_bytes":
			usedBytes = uint64(metric.Value)
		case "utilization":
			utilization = metric.Value
		}
	}

	if totalBytes == 0 {
		return nil, fmt.Errorf("no capacity data for component %s", componentID)
	}

	// 计算增长率
	growthRate := m.calculateGrowthRate(componentID, "used_bytes")

	forecast := &CapacityForecast{
		ComponentID:   componentID,
		ComponentName: comp.Name,
		CurrentUsage:  utilization,
		TotalBytes:    totalBytes,
		UsedBytes:     usedBytes,
		GrowthRate:    growthRate,
		Confidence:    0.75,
	}

	// 计算剩余天数
	if growthRate > 0 {
		remainingBytes := float64(totalBytes-usedBytes)
		if remainingBytes > 0 {
			days := remainingBytes / growthRate
			forecast.DaysRemaining = int(days)
			forecast.FullDate = time.Now().AddDate(0, 0, int(days))
		}
	}

	return forecast, nil
}

// AnalyzeTrend 分析趋势
func (m *Manager) AnalyzeTrend(componentID, metricName string, period time.Duration) (*PerformanceTrend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := componentID + ":" + metricName
	metrics, ok := m.history[key]
	if !ok || len(metrics) < 2 {
		return nil, fmt.Errorf("insufficient data for trend analysis")
	}

	// 筛选时间范围内的数据
	cutoff := time.Now().Add(-period)
	filtered := make([]Metric, 0)
	for _, m := range metrics {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}

	if len(filtered) < 2 {
		return nil, fmt.Errorf("insufficient data in period")
	}

	// 计算统计值
	var sum, minVal, maxVal float64
	minVal = math.MaxFloat64
	maxVal = -math.MaxFloat64

	for _, m := range filtered {
		sum += m.Value
		if m.Value < minVal {
			minVal = m.Value
		}
		if m.Value > maxVal {
			maxVal = m.Value
		}
	}
	avg := sum / float64(len(filtered))

	// 计算趋势（线性回归斜率）
	slope := calculateSlope(filtered)

	trend := TrendStable
	if slope > 0.01 {
		trend = TrendDegrading
	} else if slope < -0.01 {
		trend = TrendImproving
	}

	return &PerformanceTrend{
		ComponentID: componentID,
		MetricName:  metricName,
		Current:     filtered[len(filtered)-1].Value,
		Average:     avg,
		Min:         minVal,
		Max:         maxVal,
		Trend:       trend,
		ChangeRate:  slope * 86400, // 每天变化率
		Period:      period.String(),
	}, nil
}

// GenerateReport 生成预后报告
func (m *Manager) GenerateReport() *PrognosticsReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &PrognosticsReport{
		ID:          fmt.Sprintf("report-%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
		Components:  make([]Component, 0),
		Predictions: make([]Prediction, 0),
		Forecasts:   make([]CapacityForecast, 0),
		Trends:      make([]PerformanceTrend, 0),
		Recommendations: make([]Recommendation, 0),
	}

	var totalScore float64
	componentCount := 0

	for _, comp := range m.components {
		report.Components = append(report.Components, *comp)
		totalScore += comp.HealthScore
		componentCount++

		// 收集预测
		for _, pred := range m.predictions {
			if pred.ComponentID == comp.ID {
				report.Predictions = append(report.Predictions, *pred)
			}
		}
	}

	if componentCount > 0 {
		report.OverallScore = totalScore / float64(componentCount)
		report.OverallHealth = scoreToLevel(report.OverallScore)
	}

	// 生成建议
	report.Recommendations = m.generateRecommendations(report)
	report.Summary = m.generateSummary(report)

	m.reports = append(m.reports, report)

	return report
}

// ListReports 列出报告
func (m *Manager) ListReports(limit int) []*PrognosticsReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.reports) {
		limit = len(m.reports)
	}

	start := len(m.reports) - limit
	if start < 0 {
		start = 0
	}
	return m.reports[start:]
}

// GetReport 获取报告
func (m *Manager) GetReport(id string) (*PrognosticsReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("report %s not found", id)
}

// calculateGrowthRate 计算增长率
func (m *Manager) calculateGrowthRate(componentID, metricName string) float64 {
	key := componentID + ":" + metricName
	metrics, ok := m.history[key]
	if !ok || len(metrics) < 2 {
		return 0
	}

	return calculateSlope(metrics) * 86400 // 每天
}

// calculateSlope 计算线性回归斜率
func calculateSlope(metrics []Metric) float64 {
	n := float64(len(metrics))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	baseTime := metrics[0].Timestamp.Unix()

	for _, m := range metrics {
		x := float64(m.Timestamp.Unix()-baseTime)
		y := m.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denom
}

// scoreToLevel 评分转等级
func scoreToLevel(score float64) HealthLevel {
	switch {
	case score >= 90:
		return HealthExcellent
	case score >= 75:
		return HealthGood
	case score >= 50:
		return HealthFair
	case score >= 25:
		return HealthPoor
	default:
		return HealthCritical
	}
}

// generateRecommendations 生成建议
func (m *Manager) generateRecommendations(report *PrognosticsReport) []Recommendation {
	recs := make([]Recommendation, 0)
	priority := 1

	// 检查高风险组件
	for _, comp := range report.Components {
		if comp.FailureRisk == RiskCritical || comp.FailureRisk == RiskHigh {
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-%d", priority),
				Priority:    priority,
				Category:    "hardware",
				Title:       fmt.Sprintf("组件 %s 需要关注", comp.Name),
				Description: fmt.Sprintf("健康评分 %.0f，故障风险 %s", comp.HealthScore, comp.FailureRisk),
				Action:      "检查组件状态，考虑预防性更换",
				Impact:      "避免突发故障导致数据丢失",
			})
			priority++
		}
	}

	// 检查容量预测
	for _, forecast := range report.Forecasts {
		if forecast.DaysRemaining < 30 {
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-%d", priority),
				Priority:    priority,
				Category:    "capacity",
				Title:       fmt.Sprintf("容量即将耗尽: %s", forecast.ComponentName),
				Description: fmt.Sprintf("预计 %d 天后容量满（%s）", forecast.DaysRemaining, forecast.FullDate.Format("2006-01-02")),
				Action:      "清理数据或扩展存储容量",
				Impact:      "容量满将影响系统正常运行",
			})
			priority++
		}
	}

	// 检查性能退化
	for _, trend := range report.Trends {
		if trend.Trend == TrendDegrading {
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-%d", priority),
				Priority:    priority,
				Category:    "performance",
				Title:       fmt.Sprintf("性能退化: %s", trend.MetricName),
				Description: fmt.Sprintf("组件 %s 的 %s 指标持续下降", trend.ComponentID, trend.MetricName),
				Action:      "检查是否有资源竞争或硬件老化",
				Impact:      "性能持续下降影响用户体验",
			})
			priority++
		}
	}

	return recs
}

// generateSummary 生成摘要
func (m *Manager) generateSummary(report *PrognosticsReport) string {
	summary := fmt.Sprintf("系统整体健康状态: %s (%.0f/100)\n", report.OverallHealth, report.OverallScore)
	summary += fmt.Sprintf("监控组件数: %d\n", len(report.Components))

	if len(report.Predictions) > 0 {
		summary += fmt.Sprintf("活跃预测: %d\n", len(report.Predictions))
	}

	if len(report.Recommendations) > 0 {
		summary += fmt.Sprintf("待处理建议: %d\n", len(report.Recommendations))
	}

	return summary
}

// collectLoop 数据采集循环
func (m *Manager) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// predictLoop 预测循环
func (m *Manager) predictLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runPredictions()
		}
	}
}

// collectMetrics 采集指标
func (m *Manager) collectMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, comp := range m.components {
		comp.LastChecked = time.Now()
	}
}

// runPredictions 运行预测
func (m *Manager) runPredictions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, comp := range m.components {
		// 基于健康评分生成预测
		if comp.HealthScore < 50 {
			pred := &Prediction{
				ID:          fmt.Sprintf("pred-%s-%d", comp.ID, time.Now().UnixNano()),
				ComponentID: comp.ID,
				Type:        "failure",
				Probability: (100 - comp.HealthScore) / 100,
				Description: fmt.Sprintf("组件 %s 健康评分较低，存在故障风险", comp.Name),
				Confidence:  0.7,
				GeneratedAt: time.Now(),
				Actions:     []string{"检查组件状态", "准备备用方案"},
			}
			m.predictions[pred.ID] = pred

			// 更新故障风险
			if pred.Probability > 0.7 {
				comp.FailureRisk = RiskCritical
			} else if pred.Probability > 0.5 {
				comp.FailureRisk = RiskHigh
			} else if pred.Probability > 0.3 {
				comp.FailureRisk = RiskMedium
			} else {
				comp.FailureRisk = RiskLow
			}
		}
	}
}
