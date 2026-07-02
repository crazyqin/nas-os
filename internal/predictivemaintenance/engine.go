// Package predictivemaintenance 基于硬件指标的预测性维护
// 参考群晖 Active Insight 的硬件健康预测功能
package predictivemaintenance

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Config 预测性维护配置.
type Config struct {
	Enabled          bool    `json:"enabled"`
	CheckIntervalMin int     `json:"checkIntervalMin"`
	AlertThreshold   float64 `json:"alertThreshold"` // 0-100, 低于此值触发告警
	RetentionDays    int     `json:"retentionDays"`
}

// ComponentType 组件类型.
type ComponentType string

const (
	ComponentCPU     ComponentType = "cpu"
	ComponentMemory  ComponentType = "memory"
	ComponentDisk    ComponentType = "disk"
	ComponentFan     ComponentType = "fan"
	ComponentPSU     ComponentType = "psu"
	ComponentNetwork ComponentType = "network"
	ComponentGPU     ComponentType = "gpu"
)

// HealthStatus 健康状态.
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"
	StatusWarning  HealthStatus = "warning"
	StatusCritical HealthStatus = "critical"
	StatusFailed   HealthStatus = "failed"
)

// ComponentHealth 组件健康状态.
type ComponentHealth struct {
	ID            string        `json:"id"`
	Type          ComponentType `json:"type"`
	Name          string        `json:"name"`
	HealthScore   float64       `json:"healthScore"` // 0-100
	Status        HealthStatus  `json:"status"`
	Temperature   float64       `json:"temperature,omitempty"`
	UsagePercent  float64       `json:"usagePercent,omitempty"`
	PredictedDays int           `json:"predictedDays"` // 预计剩余寿命（天）
	Anomalies     []Anomaly     `json:"anomalies"`
	LastChecked   time.Time     `json:"lastChecked"`
}

// Anomaly 异常记录.
type Anomaly struct {
	Timestamp   time.Time `json:"timestamp"`
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Expected    float64   `json:"expected"`
	Deviation   float64   `json:"deviation"`
	Description string    `json:"description"`
}

// MaintenanceSchedule 维护计划.
type MaintenanceSchedule struct {
	ID          string        `json:"id"`
	ComponentID string        `json:"componentId"`
	Type        string        `json:"type"` // preventive/corrective/predictive
	Priority    int           `json:"priority"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	ScheduledAt *time.Time    `json:"scheduledAt,omitempty"`
	EstDuration time.Duration `json:"estDuration"`
	Status      string        `json:"status"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// MetricPoint 指标数据点.
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// PredictionResult 预测结果.
type PredictionResult struct {
	ComponentID   string    `json:"componentId"`
	CurrentValue  float64   `json:"currentValue"`
	PredictedPeak float64   `json:"predictedPeak"`
	DaysToFailure int       `json:"daysToFailure"`
	Confidence    float64   `json:"confidence"`
	Trend         string    `json:"trend"` // rising/stable/declining
	PredictedAt   time.Time `json:"predictedAt"`
}

// Engine 预测性维护引擎.
type Engine struct {
	mu          sync.RWMutex
	cfg         Config
	components  map[string]*ComponentHealth
	history     map[string][]MetricPoint
	schedules   map[string]*MaintenanceSchedule
	predictions map[string]*PredictionResult
}

// New 创建引擎.
func New(cfg Config) *Engine {
	if cfg.CheckIntervalMin == 0 {
		cfg.CheckIntervalMin = 30
	}
	if cfg.AlertThreshold == 0 {
		cfg.AlertThreshold = 30
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 90
	}
	return &Engine{
		cfg:         cfg,
		components:  make(map[string]*ComponentHealth),
		history:     make(map[string][]MetricPoint),
		schedules:   make(map[string]*MaintenanceSchedule),
		predictions: make(map[string]*PredictionResult),
	}
}

// RegisterComponent 注册组件.
func (e *Engine) RegisterComponent(id string, compType ComponentType, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.components[id] = &ComponentHealth{
		ID:          id,
		Type:        compType,
		Name:        name,
		HealthScore: 100,
		Status:      StatusHealthy,
		LastChecked: time.Now(),
	}
}

// RecordMetric 记录指标.
func (e *Engine) RecordMetric(componentID string, value float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history[componentID] = append(e.history[componentID], MetricPoint{
		Timestamp: time.Now(),
		Value:     value,
	})
	// 清理过期数据
	cutoff := time.Now().AddDate(0, 0, -e.cfg.RetentionDays)
	points := e.history[componentID]
	start := 0
	for start < len(points) && points[start].Timestamp.Before(cutoff) {
		start++
	}
	e.history[componentID] = points[start:]
}

// Predict 预测组件寿命.
func (e *Engine) Predict(ctx context.Context, componentID string) (*PredictionResult, error) {
	e.mu.RLock()
	history := e.history[componentID]
	e.mu.RUnlock()

	if len(history) < 10 {
		return nil, fmt.Errorf("insufficient data for prediction: need at least 10 points, got %d", len(history))
	}

	// 线性回归预测趋势
	values := make([]float64, len(history))
	for i, p := range history {
		values[i] = p.Value
	}

	slope, intercept := linearRegression(values)
	currentValue := values[len(values)-1]
	predictedPeak := slope*float64(len(values)+30) + intercept

	// 计算到阈值的天数
	daysToFailure := 365
	if slope > 0 {
		remaining := 100 - currentValue
		if remaining > 0 {
			daysToFailure = int(remaining / slope)
		}
	}

	// 置信度计算
	confidence := calculateConfidence(values, slope, intercept)

	// 趋势判断
	trend := "stable"
	if slope > 0.5 {
		trend = "rising"
	} else if slope < -0.5 {
		trend = "declining"
	}

	result := &PredictionResult{
		ComponentID:   componentID,
		CurrentValue:  currentValue,
		PredictedPeak: math.Max(0, math.Min(100, predictedPeak)),
		DaysToFailure: daysToFailure,
		Confidence:    confidence,
		Trend:         trend,
		PredictedAt:   time.Now(),
	}

	e.mu.Lock()
	e.predictions[componentID] = result
	e.mu.Unlock()

	return result, nil
}

// GetHealth 获取组件健康状态.
func (e *Engine) GetHealth(componentID string) (*ComponentHealth, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	comp, ok := e.components[componentID]
	if !ok {
		return nil, fmt.Errorf("component not found: %s", componentID)
	}
	return comp, nil
}

// ListComponents 列出所有组件.
func (e *Engine) ListComponents() []*ComponentHealth {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*ComponentHealth, 0, len(e.components))
	for _, c := range e.components {
		result = append(result, c)
	}
	return result
}

// CreateSchedule 创建维护计划.
func (e *Engine) CreateSchedule(componentID, schedType, title, desc string, priority int) (*MaintenanceSchedule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.components[componentID]; !ok {
		return nil, fmt.Errorf("component not found: %s", componentID)
	}
	schedule := &MaintenanceSchedule{
		ID:          fmt.Sprintf("maint-%d", time.Now().UnixNano()),
		ComponentID: componentID,
		Type:        schedType,
		Priority:    priority,
		Title:       title,
		Description: desc,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	e.schedules[schedule.ID] = schedule
	return schedule, nil
}

// ListSchedules 列出维护计划.
func (e *Engine) ListSchedules() []*MaintenanceSchedule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*MaintenanceSchedule, 0, len(e.schedules))
	for _, s := range e.schedules {
		result = append(result, s)
	}
	return result
}

// CheckAll 检查所有组件健康状态.
func (e *Engine) CheckAll(ctx context.Context) []*ComponentHealth {
	e.mu.RLock()
	ids := make([]string, 0, len(e.components))
	for id := range e.components {
		ids = append(ids, id)
	}
	e.mu.RUnlock()

	var results []*ComponentHealth
	for _, id := range ids {
		comp, err := e.GetHealth(id)
		if err != nil {
			continue
		}
		// 尝试预测
		if pred, err := e.Predict(ctx, id); err == nil {
			comp.PredictedDays = pred.DaysToFailure
			if pred.Confidence > 0.7 {
				if pred.DaysToFailure < 30 {
					comp.Status = StatusCritical
					comp.HealthScore = math.Max(0, 100-float64(30-pred.DaysToFailure)*3)
				} else if pred.DaysToFailure < 90 {
					comp.Status = StatusWarning
					comp.HealthScore = math.Max(30, 100-float64(90-pred.DaysToFailure))
				}
			}
		}
		comp.LastChecked = time.Now()
		results = append(results, comp)
	}
	return results
}

// linearRegression 简单线性回归.
func linearRegression(data []float64) (slope, intercept float64) {
	n := float64(len(data))
	if n < 2 {
		return 0, data[0]
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return
}

// calculateConfidence 计算 R² 置信度.
func calculateConfidence(data []float64, slope, intercept float64) float64 {
	if len(data) < 3 {
		return 0.5
	}
	var ssRes, ssTot float64
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))
	for i, v := range data {
		predicted := slope*float64(i) + intercept
		ssRes += (v - predicted) * (v - predicted)
		ssTot += (v - mean) * (v - mean)
	}
	if ssTot == 0 {
		return 0.5
	}
	r2 := 1 - ssRes/ssTot
	if r2 < 0 {
		return 0
	}
	if r2 > 1 {
		return 1
	}
	return r2
}
