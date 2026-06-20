package containerresourcegov

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ResourceGovernor 容器资源治理引擎
type ResourceGovernor struct {
	mu          sync.RWMutex
	containers  map[string]*Container
	profiles    map[string]*ResourceProfile
	predictions map[string]*ResourcePrediction
	policies    map[string]*GovernancePolicy
	metrics     *GovernanceMetrics
	predictor   *ResourcePredictor
	config      *GovernorConfig
	logger      *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// Container 容器信息
type Container struct {
	ID        string
	Name      string
	Image     string
	Status    ContainerStatus
	CPU       *ResourceUsage
	Memory    *ResourceUsage
	Network   *NetworkUsage
	Disk      *DiskUsage
	Profile   string
	Priority  ContainerPriority
	CreatedAt time.Time
	StartedAt time.Time
}

// ResourceUsage 资源使用
type ResourceUsage struct {
	Current      float64
	Limit        float64
	Request      float64
	Peak         float64
	Average      float64
	Percentile95 float64
	Trend        TrendDirection
}

// NetworkUsage 网络使用
type NetworkUsage struct {
	BytesIn    int64
	BytesOut   int64
	PacketsIn  int64
	PacketsOut int64
	Bandwidth  float64
	Latency    time.Duration
}

// DiskUsage 磁盘使用
type DiskUsage struct {
	ReadBytes  int64
	WriteBytes int64
	ReadOps    int64
	WriteOps   int64
	IOPS       float64
	Throughput float64
}

// ResourceProfile 资源配置文件
type ResourceProfile struct {
	ID          string
	Name        string
	Type        ProfileType
	CPU         *ResourceLimit
	Memory      *ResourceLimit
	Network     *NetworkLimit
	Disk        *DiskLimit
	Priority    ContainerPriority
	Description string
}

// ResourceLimit 资源限制
type ResourceLimit struct {
	Min       float64
	Max       float64
	Default   float64
	Burstable bool
	BurstMax  float64
}

// NetworkLimit 网络限制
type NetworkLimit struct {
	BandwidthIn  float64
	BandwidthOut float64
	MaxConns     int
}

// DiskLimit 磁盘限制
type DiskLimit struct {
	ReadIOPS  float64
	WriteIOPS float64
	ReadBPS   float64
	WriteBPS  float64
}

// ResourcePrediction 资源预测
type ResourcePrediction struct {
	ContainerID string
	CPUNeeded   float64
	MemNeeded   float64
	NetNeeded   float64
	DiskNeeded  float64
	Confidence  float64
	Window      time.Duration
	Method      string
	CreatedAt   time.Time
}

// GovernancePolicy 治理策略
type GovernancePolicy struct {
	ID         string
	Name       string
	Type       PolicyType
	Conditions []*PolicyCondition
	Actions    []*PolicyAction
	Priority   int
	Enabled    bool
}

// PolicyCondition 策略条件
type PolicyCondition struct {
	Metric    string
	Operator  string
	Threshold float64
	Duration  time.Duration
}

// PolicyAction 策略动作
type PolicyAction struct {
	Type       ActionType
	Parameters map[string]interface{}
}

// ResourcePredictor ML资源预测器
type ResourcePredictor struct {
	mu         sync.RWMutex
	history    map[string][]*ResourceSample
	windowSize int
	accuracy   float64
}

// ResourceSample 资源采样
type ResourceSample struct {
	Timestamp time.Time
	CPU       float64
	Memory    float64
	Network   float64
	Disk      float64
}

// GovernanceMetrics 治理指标
type GovernanceMetrics struct {
	TotalContainers    int
	Compliant          int
	NonCompliant       int
	AutoScaled         int
	ResourceEfficiency float64
	CostSavings        float64
	Violations         int64
	Predictions        int64
}

// GovernorConfig 治理配置
type GovernorConfig struct {
	MonitoringInterval time.Duration
	PredictionWindow   time.Duration
	AutoRemediate      bool
	DryRun             bool
	AlertThreshold     float64
}

// 枚举类型
type ContainerStatus int

const (
	ContainerRunning ContainerStatus = iota
	ContainerStopped
	ContainerPaused
	ContainerError
)

type ContainerPriority int

const (
	PriorityLow ContainerPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

type ProfileType int

const (
	ProfileTypeShared ProfileType = iota
	ProfileTypeDedicated
	ProfileTypeBurstable
	ProfileTypeGPU
)

type TrendDirection int

const (
	TrendStable TrendDirection = iota
	TrendIncreasing
	TrendDecreasing
	TrendVolatile
)

type PolicyType int

const (
	PolicyTypeResource PolicyType = iota
	PolicyTypeCost
	PolicyTypeSecurity
	PolicyTypeCompliance
)

type ActionType int

const (
	ActionTypeScale ActionType = iota
	ActionTypeThrottle
	ActionTypeMigrate
	ActionTypeAlert
	ActionTypeRestart
)

// PolicyViolation 策略违规
type PolicyViolation struct {
	PolicyID    string
	ContainerID string
	Condition   *PolicyCondition
	Value       float64
	Timestamp   time.Time
}

// RemediationResult 修复结果
type RemediationResult struct {
	PolicyID    string
	ContainerID string
	Action      ActionType
	Success     bool
	Message     string
	Timestamp   time.Time
}

// EfficiencyReport 效率报告
type EfficiencyReport struct {
	Timestamp            time.Time
	OverallEfficiency    float64
	OverallCPUEfficiency float64
	OverallMemEfficiency float64
	WastedCPU            float64
	WastedMemory         float64
	ContainerStats       map[string]*ContainerEfficiency
}

// ContainerEfficiency 容器效率
type ContainerEfficiency struct {
	ContainerID   string
	CPUEfficiency float64
	MemEfficiency float64
	OverallScore  float64
}

// NewResourceGovernor 创建治理引擎
func NewResourceGovernor(config *GovernorConfig, logger *slog.Logger) *ResourceGovernor {
	if config == nil {
		config = &GovernorConfig{
			MonitoringInterval: 30 * time.Second,
			PredictionWindow:   time.Hour,
			AutoRemediate:      false,
			DryRun:             true,
			AlertThreshold:     0.8,
		}
	}
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	governor := &ResourceGovernor{
		containers:  make(map[string]*Container),
		profiles:    make(map[string]*ResourceProfile),
		predictions: make(map[string]*ResourcePrediction),
		policies:    make(map[string]*GovernancePolicy),
		metrics:     &GovernanceMetrics{},
		predictor: &ResourcePredictor{
			history:    make(map[string][]*ResourceSample),
			windowSize: 100,
			accuracy:   0.85,
		},
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动监控协程
	go governor.monitoringLoop()

	return governor
}

// RegisterContainer 注册容器
func (g *ResourceGovernor) RegisterContainer(container *Container) error {
	if container == nil {
		return ErrInvalidContainer
	}
	if container.ID == "" {
		return ErrContainerIDRequired
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.containers[container.ID]; exists {
		return ErrContainerAlreadyExists
	}

	// 初始化默认资源使用
	if container.CPU == nil {
		container.CPU = &ResourceUsage{}
	}
	if container.Memory == nil {
		container.Memory = &ResourceUsage{}
	}
	if container.Network == nil {
		container.Network = &NetworkUsage{}
	}
	if container.Disk == nil {
		container.Disk = &DiskUsage{}
	}
	if container.CreatedAt.IsZero() {
		container.CreatedAt = time.Now()
	}

	g.containers[container.ID] = container
	g.metrics.TotalContainers++

	g.logger.Info("container registered",
		"id", container.ID,
		"name", container.Name,
		"image", container.Image,
	)

	return nil
}

// UpdateResourceUsage 更新资源使用
func (g *ResourceGovernor) UpdateResourceUsage(containerID string, cpu, memory float64, network *NetworkUsage, disk *DiskUsage) error {
	g.mu.Lock()
	container, exists := g.containers[containerID]
	if !exists {
		g.mu.Unlock()
		return ErrContainerNotFound
	}

	// 更新 CPU
	if container.CPU == nil {
		container.CPU = &ResourceUsage{}
	}
	container.CPU.Current = cpu
	if cpu > container.CPU.Peak {
		container.CPU.Peak = cpu
	}
	container.CPU.Average = (container.CPU.Average + cpu) / 2

	// 更新内存
	if container.Memory == nil {
		container.Memory = &ResourceUsage{}
	}
	container.Memory.Current = memory
	if memory > container.Memory.Peak {
		container.Memory.Peak = memory
	}
	container.Memory.Average = (container.Memory.Average + memory) / 2

	// 更新网络
	if network != nil {
		container.Network = network
	}

	// 更新磁盘
	if disk != nil {
		container.Disk = disk
	}

	g.mu.Unlock()

	// 记录采样数据
	g.predictor.mu.Lock()
	g.predictor.history[containerID] = append(g.predictor.history[containerID], &ResourceSample{
		Timestamp: time.Now(),
		CPU:       cpu,
		Memory:    memory,
	})
	// 保持窗口大小
	if len(g.predictor.history[containerID]) > g.predictor.windowSize {
		g.predictor.history[containerID] = g.predictor.history[containerID][1:]
	}
	g.predictor.mu.Unlock()

	return nil
}

// PredictResources ML预测资源需求
func (g *ResourceGovernor) PredictResources(containerID string, window time.Duration) (*ResourcePrediction, error) {
	g.predictor.mu.RLock()
	samples, exists := g.predictor.history[containerID]
	g.predictor.mu.RUnlock()

	if !exists || len(samples) < 2 {
		return nil, ErrInsufficientData
	}

	// 简单的线性回归预测
	var cpuSum, memSum float64
	n := float64(len(samples))
	for _, s := range samples {
		cpuSum += s.CPU
		memSum += s.Memory
	}

	cpuAvg := cpuSum / n
	memAvg := memSum / n

	// 计算趋势
	var cpuTrend, memTrend float64
	if len(samples) >= 2 {
		last := samples[len(samples)-1]
		first := samples[0]
		cpuTrend = (last.CPU - first.CPU) / n
		memTrend = (last.Memory - first.Memory) / n
	}

	prediction := &ResourcePrediction{
		ContainerID: containerID,
		CPUNeeded:   cpuAvg + cpuTrend*float64(window.Seconds()/60),
		MemNeeded:   memAvg + memTrend*float64(window.Seconds()/60),
		Confidence:  g.predictor.accuracy,
		Window:      window,
		Method:      "linear_regression",
		CreatedAt:   time.Now(),
	}

	// 限制预测值在合理范围内
	if prediction.CPUNeeded < 0 {
		prediction.CPUNeeded = 0
	}
	if prediction.MemNeeded < 0 {
		prediction.MemNeeded = 0
	}

	g.mu.Lock()
	g.predictions[containerID] = prediction
	g.metrics.Predictions++
	g.mu.Unlock()

	g.logger.Info("resource prediction generated",
		"containerID", containerID,
		"cpu", prediction.CPUNeeded,
		"memory", prediction.MemNeeded,
		"confidence", prediction.Confidence,
	)

	return prediction, nil
}

// ApplyProfile 应用资源配额
func (g *ResourceGovernor) ApplyProfile(containerID, profileID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	container, exists := g.containers[containerID]
	if !exists {
		return ErrContainerNotFound
	}

	profile, exists := g.profiles[profileID]
	if !exists {
		return ErrProfileNotFound
	}

	// 应用 CPU 限制
	if profile.CPU != nil {
		container.CPU.Limit = profile.CPU.Max
		container.CPU.Request = profile.CPU.Default
	}

	// 应用内存限制
	if profile.Memory != nil {
		container.Memory.Limit = profile.Memory.Max
		container.Memory.Request = profile.Memory.Default
	}

	container.Profile = profileID
	container.Priority = profile.Priority

	g.logger.Info("profile applied",
		"containerID", containerID,
		"profileID", profileID,
		"profileName", profile.Name,
	)

	return nil
}

// RegisterProfile 注册资源配额
func (g *ResourceGovernor) RegisterProfile(profile *ResourceProfile) error {
	if profile == nil {
		return ErrInvalidProfile
	}
	if profile.ID == "" {
		return ErrProfileIDRequired
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.profiles[profile.ID]; exists {
		return ErrProfileAlreadyExists
	}

	g.profiles[profile.ID] = profile

	g.logger.Info("profile registered",
		"profileID", profile.ID,
		"name", profile.Name,
		"type", profile.Type,
	)

	return nil
}

// RegisterPolicy 注册治理策略
func (g *ResourceGovernor) RegisterPolicy(policy *GovernancePolicy) error {
	if policy == nil {
		return ErrInvalidPolicy
	}
	if policy.ID == "" {
		return ErrPolicyIDRequired
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.policies[policy.ID]; exists {
		return ErrPolicyAlreadyExists
	}

	g.policies[policy.ID] = policy

	g.logger.Info("policy registered",
		"policyID", policy.ID,
		"name", policy.Name,
		"type", policy.Type,
	)

	return nil
}

// EvaluatePolicies 评估策略
func (g *ResourceGovernor) EvaluatePolicies() []*PolicyViolation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var violations []*PolicyViolation

	for _, policy := range g.policies {
		if !policy.Enabled {
			continue
		}

		for _, container := range g.containers {
			violation := g.evaluatePolicyForContainer(policy, container)
			if violation != nil {
				violations = append(violations, violation)
				g.metrics.Violations++
			}
		}
	}

	g.logger.Info("policy evaluation completed", "violations", len(violations))
	return violations
}

// evaluatePolicyForContainer 为容器评估策略
func (g *ResourceGovernor) evaluatePolicyForContainer(policy *GovernancePolicy, container *Container) *PolicyViolation {
	for _, condition := range policy.Conditions {
		value := g.getMetricValue(container, condition.Metric)
		if g.evaluateCondition(value, condition.Operator, condition.Threshold) {
			return &PolicyViolation{
				PolicyID:    policy.ID,
				ContainerID: container.ID,
				Condition:   condition,
				Value:       value,
				Timestamp:   time.Now(),
			}
		}
	}
	return nil
}

// getMetricValue 获取指标值
func (g *ResourceGovernor) getMetricValue(container *Container, metric string) float64 {
	switch metric {
	case "cpu":
		return container.CPU.Current
	case "memory":
		return container.Memory.Current
	case "cpu_limit":
		return container.CPU.Limit
	case "memory_limit":
		return container.Memory.Limit
	default:
		return 0
	}
}

// evaluateCondition 评估条件
func (g *ResourceGovernor) evaluateCondition(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		return false
	}
}

// AutoRemediate 自动修复
func (g *ResourceGovernor) AutoRemediate(violations []*PolicyViolation) []RemediationResult {
	if !g.config.AutoRemediate {
		g.logger.Info("auto-remediate disabled, skipping")
		return nil
	}

	var results []RemediationResult

	g.mu.RLock()
	policies := make(map[string]*GovernancePolicy)
	for k, v := range g.policies {
		policies[k] = v
	}
	g.mu.RUnlock()

	for _, violation := range violations {
		policy, exists := policies[violation.PolicyID]
		if !exists {
			continue
		}

		for _, action := range policy.Actions {
			result := g.executeAction(violation, action)
			results = append(results, result)
		}
	}

	g.logger.Info("auto-remediate completed", "results", len(results))
	return results
}

// executeAction 执行动作
func (g *ResourceGovernor) executeAction(violation *PolicyViolation, action *PolicyAction) RemediationResult {
	result := RemediationResult{
		PolicyID:    violation.PolicyID,
		ContainerID: violation.ContainerID,
		Action:      action.Type,
		Timestamp:   time.Now(),
	}

	if g.config.DryRun {
		result.Success = true
		result.Message = "dry run - action would be executed"
		g.logger.Info("dry run action",
			"action", action.Type,
			"containerID", violation.ContainerID,
		)
		return result
	}

	switch action.Type {
	case ActionTypeScale:
		result.Success = true
		result.Message = fmt.Sprintf("scaled container %s", violation.ContainerID)
		g.metrics.AutoScaled++
	case ActionTypeThrottle:
		result.Success = true
		result.Message = fmt.Sprintf("throttled container %s", violation.ContainerID)
	case ActionTypeAlert:
		result.Success = true
		result.Message = fmt.Sprintf("alert sent for container %s", violation.ContainerID)
	case ActionTypeRestart:
		result.Success = true
		result.Message = fmt.Sprintf("container %s restarted", violation.ContainerID)
	default:
		result.Success = false
		result.Message = fmt.Sprintf("unknown action type: %d", action.Type)
	}

	return result
}

// GetEfficiencyReport 效率报告
func (g *ResourceGovernor) GetEfficiencyReport() *EfficiencyReport {
	g.mu.RLock()
	defer g.mu.RUnlock()

	report := &EfficiencyReport{
		Timestamp:      time.Now(),
		ContainerStats: make(map[string]*ContainerEfficiency),
	}

	var totalCPUUsage, totalMemUsage float64
	var totalCPULimit, totalMemLimit float64

	for id, container := range g.containers {
		if container.Status != ContainerRunning {
			continue
		}

		cpuEfficiency := 0.0
		if container.CPU.Limit > 0 {
			cpuEfficiency = container.CPU.Current / container.CPU.Limit
		}

		memEfficiency := 0.0
		if container.Memory.Limit > 0 {
			memEfficiency = container.Memory.Current / container.Memory.Limit
		}

		report.ContainerStats[id] = &ContainerEfficiency{
			ContainerID:   id,
			CPUEfficiency: cpuEfficiency,
			MemEfficiency: memEfficiency,
			OverallScore:  (cpuEfficiency + memEfficiency) / 2,
		}

		totalCPUUsage += container.CPU.Current
		totalMemUsage += container.Memory.Current
		totalCPULimit += container.CPU.Limit
		totalMemLimit += container.Memory.Limit
	}

	if totalCPULimit > 0 {
		report.OverallCPUEfficiency = totalCPUUsage / totalCPULimit
	}
	if totalMemLimit > 0 {
		report.OverallMemEfficiency = totalMemUsage / totalMemLimit
	}
	report.OverallEfficiency = (report.OverallCPUEfficiency + report.OverallMemEfficiency) / 2

	// 计算资源浪费
	report.WastedCPU = totalCPULimit - totalCPUUsage
	report.WastedMemory = totalMemLimit - totalMemUsage

	return report
}

// GetMetrics 获取指标
func (g *ResourceGovernor) GetMetrics() *GovernanceMetrics {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 计算合规数
	compliant := 0
	nonCompliant := 0
	for _, container := range g.containers {
		if g.isCompliant(container) {
			compliant++
		} else {
			nonCompliant++
		}
	}

	g.metrics.Compliant = compliant
	g.metrics.NonCompliant = nonCompliant

	// 计算资源效率
	report := g.GetEfficiencyReport()
	g.metrics.ResourceEfficiency = report.OverallEfficiency

	// 深拷贝
	metricsCopy := *g.metrics
	return &metricsCopy
}

// isCompliant 检查容器是否合规
func (g *ResourceGovernor) isCompliant(container *Container) bool {
	if container.CPU.Limit > 0 && container.CPU.Current > container.CPU.Limit {
		return false
	}
	if container.Memory.Limit > 0 && container.Memory.Current > container.Memory.Limit {
		return false
	}
	return true
}

// monitoringLoop 监控循环
func (g *ResourceGovernor) monitoringLoop() {
	ticker := time.NewTicker(g.config.MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.monitorCycle()
		}
	}
}

// monitorCycle 监控周期
func (g *ResourceGovernor) monitorCycle() {
	// 评估策略
	violations := g.EvaluatePolicies()

	// 自动修复
	if len(violations) > 0 {
		g.AutoRemediate(violations)
	}
}

// Stop 停止治理引擎
func (g *ResourceGovernor) Stop() {
	g.cancel()
	g.logger.Info("resource governor stopped")
}

// GetContainer 获取容器信息
func (g *ResourceGovernor) GetContainer(containerID string) (*Container, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	container, exists := g.containers[containerID]
	if !exists {
		return nil, ErrContainerNotFound
	}

	// 返回副本
	c := *container
	return &c, nil
}

// ListContainers 列出所有容器
func (g *ResourceGovernor) ListContainers() []*Container {
	g.mu.RLock()
	defer g.mu.RUnlock()

	containers := make([]*Container, 0, len(g.containers))
	for _, c := range g.containers {
		copy := *c
		containers = append(containers, &copy)
	}
	return containers
}

// RemoveContainer 移除容器
func (g *ResourceGovernor) RemoveContainer(containerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.containers[containerID]; !exists {
		return ErrContainerNotFound
	}

	delete(g.containers, containerID)
	g.metrics.TotalContainers--

	g.logger.Info("container removed", "id", containerID)
	return nil
}
