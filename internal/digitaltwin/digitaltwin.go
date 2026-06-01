// Package digitaltwin 实现 NAS 数字孪生
// 提供系统模拟、容量规划、故障预测、性能调优
package digitaltwin

import (
	"fmt"
	"sync"
	"time"
)

// TwinState 孪生状态
type TwinState string

const (
	StateIdle      TwinState = "idle"
	StateSimulating TwinState = "simulating"
	StatePredicting TwinState = "predicting"
	StateOptimizing TwinState = "optimizing"
)

// ComponentType 组件类型
type ComponentType string

const (
	ComponentCPU     ComponentType = "cpu"
	ComponentMemory  ComponentType = "memory"
	ComponentStorage ComponentType = "storage"
	ComponentNetwork ComponentType = "network"
)

// SimulationScenario 模拟场景
type SimulationScenario struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Duration    time.Duration          `json:"duration"`
	CreatedAt   time.Time              `json:"created_at"`
}

// SimulationResult 模拟结果
type SimulationResult struct {
	ScenarioID  string                 `json:"scenario_id"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Metrics     map[string]float64     `json:"metrics"`
	Bottlenecks []string               `json:"bottlenecks"`
	Recommendations []string          `json:"recommendations"`
	Success     bool                   `json:"success"`
}

// Component 系统组件
type Component struct {
	ID         string            `json:"id"`
	Type       ComponentType     `json:"type"`
	Name       string            `json:"name"`
	Capacity   float64           `json:"capacity"`
	Usage      float64           `json:"usage"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Prediction 预测
type Prediction struct {
	ID          string    `json:"id"`
	ComponentID string    `json:"component_id"`
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Confidence  float64   `json:"confidence"`
	TimeHorizon time.Duration `json:"time_horizon"`
	CreatedAt   time.Time `json:"created_at"`
}

// DigitalTwin 数字孪生
type DigitalTwin struct {
	mu          sync.RWMutex
	state       TwinState
	components  map[string]*Component
	scenarios   map[string]*SimulationScenario
	results     map[string]*SimulationResult
	predictions map[string]*Prediction
	lastSync    time.Time
}

// Config 配置
type Config struct{}

// New 创建数字孪生
func New(cfg Config) *DigitalTwin {
	return &DigitalTwin{
		state:       StateIdle,
		components:  make(map[string]*Component),
		scenarios:   make(map[string]*SimulationScenario),
		results:     make(map[string]*SimulationResult),
		predictions: make(map[string]*Prediction),
	}
}

// RegisterComponent 注册组件
func (dt *DigitalTwin) RegisterComponent(comp *Component) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if comp.ID == "" {
		return fmt.Errorf("组件ID不能为空")
	}

	comp.UpdatedAt = time.Now()
	dt.components[comp.ID] = comp
	return nil
}

// UpdateComponentUsage 更新组件使用率
func (dt *DigitalTwin) UpdateComponentUsage(id string, usage float64) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	comp, exists := dt.components[id]
	if !exists {
		return fmt.Errorf("组件不存在: %s", id)
	}

	comp.Usage = usage
	comp.UpdatedAt = time.Now()
	return nil
}

// CreateScenario 创建模拟场景
func (dt *DigitalTwin) CreateScenario(scenario *SimulationScenario) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if scenario.ID == "" {
		return fmt.Errorf("场景ID不能为空")
	}

	scenario.CreatedAt = time.Now()
	dt.scenarios[scenario.ID] = scenario
	return nil
}

// RunSimulation 运行模拟
func (dt *DigitalTwin) RunSimulation(scenarioID string) (*SimulationResult, error) {
	dt.mu.Lock()
	scenario, exists := dt.scenarios[scenarioID]
	if !exists {
		dt.mu.Unlock()
		return nil, fmt.Errorf("场景不存在: %s", scenarioID)
	}
	dt.state = StateSimulating
	dt.mu.Unlock()

	// 模拟运行
	result := &SimulationResult{
		ScenarioID: scenarioID,
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(scenario.Duration),
		Metrics:    make(map[string]float64),
		Success:    true,
	}

	// 计算模拟指标
	dt.mu.RLock()
	for _, comp := range dt.components {
		result.Metrics[string(comp.Type)+"_usage"] = comp.Usage / comp.Capacity
	}
	dt.mu.RUnlock()

	dt.mu.Lock()
	dt.results[scenarioID] = result
	dt.state = StateIdle
	dt.mu.Unlock()

	return result, nil
}

// PredictMetric 预测指标
func (dt *DigitalTwin) PredictMetric(componentID, metric string, horizon time.Duration) (*Prediction, error) {
	dt.mu.RLock()
	comp, exists := dt.components[componentID]
	if !exists {
		dt.mu.RUnlock()
		return nil, fmt.Errorf("组件不存在: %s", componentID)
	}
	dt.mu.RUnlock()

	// 简单预测：基于当前使用率
	predictedValue := comp.Usage * 1.1 // 假设增长10%
	confidence := 0.7

	prediction := &Prediction{
		ID:          fmt.Sprintf("pred-%s-%s-%d", componentID, metric, time.Now().UnixNano()),
		ComponentID: componentID,
		Metric:      metric,
		Value:       predictedValue,
		Confidence:  confidence,
		TimeHorizon: horizon,
		CreatedAt:   time.Now(),
	}

	dt.mu.Lock()
	dt.predictions[prediction.ID] = prediction
	dt.mu.Unlock()

	return prediction, nil
}

// GetBottlenecks 获取瓶颈
func (dt *DigitalTwin) GetBottlenecks() []string {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	var bottlenecks []string
	for _, comp := range dt.components {
		utilization := comp.Usage / comp.Capacity
		if utilization > 0.8 {
			bottlenecks = append(bottlenecks, fmt.Sprintf("%s (%s) 使用率 %.1f%%",
				comp.Name, comp.Type, utilization*100))
		}
	}
	return bottlenecks
}

// GetState 获取孪生状态
func (dt *DigitalTwin) GetState() TwinState {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return dt.state
}

// GetStats 获取统计
func (dt *DigitalTwin) GetStats() map[string]interface{} {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	return map[string]interface{}{
		"state":        dt.state,
		"components":   len(dt.components),
		"scenarios":    len(dt.scenarios),
		"results":      len(dt.results),
		"predictions":  len(dt.predictions),
		"last_sync":    dt.lastSync,
	}
}

// Sync 同步真实系统状态
func (dt *DigitalTwin) Sync() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.lastSync = time.Now()
}
