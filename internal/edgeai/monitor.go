// Package edgeai 提供推理性能监控功能
package edgeai

import (
	"log"
	"sync"
	"time"
)

// DefaultResourceMonitor 默认资源监控器
type DefaultResourceMonitor struct {
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
	interval   time.Duration
	usage      *ResourceUsage
	history    []ResourceUsage
	maxHistory int
	onUpdate   func(*ResourceUsage)
}

// NewDefaultResourceMonitor 创建默认资源监控器
func NewDefaultResourceMonitor(maxHistory int) *DefaultResourceMonitor {
	return &DefaultResourceMonitor{
		stopCh:     make(chan struct{}),
		usage:      &ResourceUsage{},
		history:    make([]ResourceUsage, 0),
		maxHistory: maxHistory,
	}
}

// Start 开始监控
func (m *DefaultResourceMonitor) Start(interval time.Duration) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.interval = interval
	m.mu.Unlock()

	go m.monitorLoop()
	log.Printf("资源监控器已启动，间隔: %v", interval)
}

// Stop 停止监控
func (m *DefaultResourceMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	log.Printf("资源监控器已停止")
}

// GetUsage 获取当前资源使用情况
func (m *DefaultResourceMonitor) GetUsage() (*ResourceUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 模拟资源使用情况
	usage := &ResourceUsage{
		CPU: CPUUsage{
			Usage:     m.simulateCPUUsage(),
			Cores:     8,
			Threads:   16,
			Frequency: 3.6,
		},
		GPU: GPUUsage{
			Available:   true,
			Usage:       m.simulateGPUUsage(),
			MemoryUsed:  2 * 1024 * 1024 * 1024, // 2GB
			MemoryTotal: 8 * 1024 * 1024 * 1024, // 8GB
			Temperature: 65.0,
			PowerUsage:  150.0,
		},
		Memory: MemoryUsage{
			Total:     16 * 1024 * 1024 * 1024, // 16GB
			Used:      8 * 1024 * 1024 * 1024,  // 8GB
			Available: 8 * 1024 * 1024 * 1024,  // 8GB
			Usage:     50.0,
		},
		Disk: DiskUsage{
			Total:     1024 * 1024 * 1024 * 1024, // 1TB
			Used:      500 * 1024 * 1024 * 1024,  // 500GB
			Available: 524 * 1024 * 1024 * 1024,  // 524GB
			Usage:     48.8,
		},
	}

	return usage, nil
}

// GetHistory 获取资源使用历史
func (m *DefaultResourceMonitor) GetHistory() []ResourceUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]ResourceUsage, len(m.history))
	copy(history, m.history)
	return history
}

// SetUpdateCallback 设置更新回调
func (m *DefaultResourceMonitor) SetUpdateCallback(callback func(*ResourceUsage)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = callback
}

// IsRunning 是否正在运行
func (m *DefaultResourceMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// monitorLoop 监控循环
func (m *DefaultResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.updateUsage()
		}
	}
}

// updateUsage 更新资源使用情况
func (m *DefaultResourceMonitor) updateUsage() {
	usage, err := m.GetUsage()
	if err != nil {
		log.Printf("获取资源使用情况失败: %v", err)
		return
	}

	m.mu.Lock()
	m.usage = usage
	m.history = append(m.history, *usage)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}
	callback := m.onUpdate
	m.mu.Unlock()

	// 触发回调
	if callback != nil {
		callback(usage)
	}
}

// simulateCPUUsage 模拟 CPU 使用率
func (m *DefaultResourceMonitor) simulateCPUUsage() float64 {
	// 模拟 CPU 使用率波动
	return 30.0 + float64(time.Now().UnixNano()%40)
}

// simulateGPUUsage 模拟 GPU 使用率
func (m *DefaultResourceMonitor) simulateGPUUsage() float64 {
	// 模拟 GPU 使用率波动
	return 20.0 + float64(time.Now().UnixNano()%60)
}

// InferenceMonitor 推理监控器
type InferenceMonitor struct {
	mu         sync.RWMutex
	metrics    *InferenceMetrics
	history    []InferenceMetrics
	maxHistory int
	onUpdate   func(*InferenceMetrics)
}

// InferenceMetrics 推理指标
type InferenceMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalRequests   int64     `json:"totalRequests"`
	SuccessRequests int64     `json:"successRequests"`
	FailedRequests  int64     `json:"failedRequests"`
	AvgLatency      float64   `json:"avgLatency"` // ms
	P95Latency      float64   `json:"p95Latency"` // ms
	P99Latency      float64   `json:"p99Latency"` // ms
	Throughput      float64   `json:"throughput"` // 推理/秒
	GPUUtilization  float64   `json:"gpuUtilization"`
	CPUUtilization  float64   `json:"cpuUtilization"`
	MemoryUsage     int64     `json:"memoryUsage"`
	QueuedRequests  int64     `json:"queuedRequests"`
}

// NewInferenceMonitor 创建推理监控器
func NewInferenceMonitor(maxHistory int) *InferenceMonitor {
	return &InferenceMonitor{
		metrics:    &InferenceMetrics{},
		history:    make([]InferenceMetrics, 0),
		maxHistory: maxHistory,
	}
}

// Update 更新指标
func (m *InferenceMonitor) Update(stats *InferStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = &InferenceMetrics{
		Timestamp:       time.Now(),
		TotalRequests:   stats.TotalRequests,
		SuccessRequests: stats.SuccessRequests,
		FailedRequests:  stats.FailedRequests,
		AvgLatency:      stats.AvgLatency,
		P95Latency:      stats.P95Latency,
		P99Latency:      stats.P99Latency,
		GPUUtilization:  stats.GPUUtilization,
		CPUUtilization:  stats.CPUUtilization,
		QueuedRequests:  stats.QueuedRequests,
	}

	m.history = append(m.history, *m.metrics)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}

	// 触发回调
	if m.onUpdate != nil {
		m.onUpdate(m.metrics)
	}
}

// GetMetrics 获取当前指标
func (m *InferenceMonitor) GetMetrics() *InferenceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// GetHistory 获取指标历史
func (m *InferenceMonitor) GetHistory() []InferenceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]InferenceMetrics, len(m.history))
	copy(history, m.history)
	return history
}

// SetUpdateCallback 设置更新回调
func (m *InferenceMonitor) SetUpdateCallback(callback func(*InferenceMetrics)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = callback
}

// PerformanceProfiler 性能分析器
type PerformanceProfiler struct {
	mu       sync.RWMutex
	profiles map[string]*ProfileData
	enabled  bool
}

// ProfileData 性能数据
type ProfileData struct {
	ModelID    string        `json:"modelId"`
	Operation  string        `json:"operation"`
	Duration   time.Duration `json:"duration"`
	MemoryUsed int64         `json:"memoryUsed"`
	Timestamp  time.Time     `json:"timestamp"`
}

// NewPerformanceProfiler 创建性能分析器
func NewPerformanceProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{
		profiles: make(map[string]*ProfileData),
		enabled:  false,
	}
}

// Enable 启用性能分析
func (p *PerformanceProfiler) Enable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = true
}

// Disable 禁用性能分析
func (p *PerformanceProfiler) Disable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = false
}

// IsEnabled 是否启用
func (p *PerformanceProfiler) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// StartProfiling 开始分析
func (p *PerformanceProfiler) StartProfiling(modelID, operation string) func() {
	if !p.IsEnabled() {
		return func() {}
	}

	start := time.Now()
	startMemory := p.getCurrentMemory()

	return func() {
		if !p.IsEnabled() {
			return
		}

		duration := time.Since(start)
		endMemory := p.getCurrentMemory()

		p.mu.Lock()
		key := modelID + ":" + operation
		p.profiles[key] = &ProfileData{
			ModelID:    modelID,
			Operation:  operation,
			Duration:   duration,
			MemoryUsed: endMemory - startMemory,
			Timestamp:  time.Now(),
		}
		p.mu.Unlock()
	}
}

// GetProfile 获取性能数据
func (p *PerformanceProfiler) GetProfile(modelID, operation string) *ProfileData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := modelID + ":" + operation
	return p.profiles[key]
}

// GetAllProfiles 获取所有性能数据
func (p *PerformanceProfiler) GetAllProfiles() map[string]*ProfileData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	profiles := make(map[string]*ProfileData)
	for k, v := range p.profiles {
		profiles[k] = v
	}

	return profiles
}

// Clear 清除性能数据
func (p *PerformanceProfiler) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profiles = make(map[string]*ProfileData)
}

// getCurrentMemory 获取当前内存使用
func (p *PerformanceProfiler) getCurrentMemory() int64 {
	// 简化实现：返回模拟值
	return 1024 * 1024 * 100 // 100MB
}

// AlertManager 告警管理器
type AlertManager struct {
	mu       sync.RWMutex
	rules    []AlertRule
	alerts   []Alert
	callback func(Alert)
}

// AlertRule 告警规则
type AlertRule struct {
	Name      string  `json:"name"`
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
	Operator  string  `json:"operator"` // gt, lt, eq
	Level     string  `json:"level"`    // info, warning, error
	Enabled   bool    `json:"enabled"`
}

// Alert 告警
type Alert struct {
	Rule      AlertRule `json:"rule"`
	Value     float64   `json:"value"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp"`
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	am := &AlertManager{
		rules:  make([]AlertRule, 0),
		alerts: make([]Alert, 0),
	}

	// 添加默认规则
	am.AddRule(AlertRule{
		Name:      "GPU 使用率过高",
		Metric:    "gpu_utilization",
		Threshold: 90,
		Operator:  "gt",
		Level:     "warning",
		Enabled:   true,
	})

	am.AddRule(AlertRule{
		Name:      "内存使用率过高",
		Metric:    "memory_usage",
		Threshold: 90,
		Operator:  "gt",
		Level:     "error",
		Enabled:   true,
	})

	am.AddRule(AlertRule{
		Name:      "推理延迟过高",
		Metric:    "latency",
		Threshold: 1000,
		Operator:  "gt",
		Level:     "warning",
		Enabled:   true,
	})

	return am
}

// AddRule 添加告警规则
func (am *AlertManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

// Check 检查指标
func (am *AlertManager) Check(metrics *InferenceMetrics, resourceUsage *ResourceUsage) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]Alert, 0)

	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		var value float64
		var triggered bool

		switch rule.Metric {
		case "gpu_utilization":
			value = resourceUsage.GPU.Usage
		case "cpu_utilization":
			value = resourceUsage.CPU.Usage
		case "memory_usage":
			value = resourceUsage.Memory.Usage
		case "latency":
			value = metrics.AvgLatency
		case "queue_length":
			value = float64(metrics.QueuedRequests)
		default:
			continue
		}

		switch rule.Operator {
		case "gt":
			triggered = value > rule.Threshold
		case "lt":
			triggered = value < rule.Threshold
		case "eq":
			triggered = value == rule.Threshold
		}

		if triggered {
			alert := Alert{
				Rule:      rule,
				Value:     value,
				Message:   rule.Name,
				Level:     rule.Level,
				Timestamp: time.Now(),
			}
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// GetAlerts 获取所有告警
func (am *AlertManager) GetAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]Alert, len(am.alerts))
	copy(alerts, am.alerts)
	return alerts
}

// SetCallback 设置告警回调
func (am *AlertManager) SetCallback(callback func(Alert)) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.callback = callback
}
