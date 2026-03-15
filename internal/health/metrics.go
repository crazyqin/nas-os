// Package health 提供系统健康检查功能
// metrics.go - 指标收集器 v2.51.0
package health

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// MetricsCollector 系统指标收集器
type MetricsCollector struct {
	mu sync.RWMutex

	// 系统指标
	cpuUsage    *prometheus.GaugeVec
	memoryUsage *prometheus.GaugeVec
	diskUsage   *prometheus.GaugeVec
	networkIO   *prometheus.GaugeVec

	// 健康检查指标
	checkStatus   *prometheus.GaugeVec
	checkDuration *prometheus.HistogramVec
	checkCount    *prometheus.CounterVec

	// 告警指标
	alertStatus *prometheus.GaugeVec
	alertCount  *prometheus.CounterVec
	alertFired  *prometheus.CounterVec

	// 自定义指标
	customMetrics map[string]prometheus.Collector

	// 采集配置
	collectInterval time.Duration
	enabledMetrics  map[string]bool

	// 运行时状态
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// 依赖
	healthChecker *HealthChecker
	alertManager  *AlertManager
	logger        *zap.Logger

	// 历史数据存储
	timeSeries    map[string]*TimeSeriesData
	maxDataPoints int
}

// TimeSeriesData 时间序列数据
type TimeSeriesData struct {
	Name   string
	Points []DataPoint
}

// DataPoint 数据点
type DataPoint struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

// MetricsConfig 指标收集配置
type MetricsConfig struct {
	Namespace       string
	CollectInterval time.Duration
	EnabledMetrics  []string
	MaxDataPoints   int
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(hc *HealthChecker, am *AlertManager, config *MetricsConfig, opts ...MetricsOption) *MetricsCollector {
	if config == nil {
		config = &MetricsConfig{
			Namespace:       "nasos",
			CollectInterval: 30 * time.Second,
			MaxDataPoints:   1000,
		}
	}

	mc := &MetricsCollector{
		healthChecker:   hc,
		alertManager:    am,
		collectInterval: config.CollectInterval,
		enabledMetrics:  make(map[string]bool),
		customMetrics:   make(map[string]prometheus.Collector),
		timeSeries:      make(map[string]*TimeSeriesData),
		maxDataPoints:   config.MaxDataPoints,
		stopCh:          make(chan struct{}),
	}

	// 初始化启用的指标
	for _, m := range config.EnabledMetrics {
		mc.enabledMetrics[m] = true
	}

	// 注册 Prometheus 指标
	mc.registerMetrics(config.Namespace)

	// 应用选项
	for _, opt := range opts {
		opt(mc)
	}

	return mc
}

// MetricsOption 指标收集器选项
type MetricsOption func(*MetricsCollector)

// WithMetricsLogger 设置日志器
func WithMetricsLogger(logger *zap.Logger) MetricsOption {
	return func(mc *MetricsCollector) {
		mc.logger = logger
	}
}

// registerMetrics 注册 Prometheus 指标
func (mc *MetricsCollector) registerMetrics(namespace string) {
	// 系统指标
	mc.cpuUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "cpu_usage_percent",
			Help:      "Current CPU usage percentage",
		},
		[]string{"core"},
	)

	mc.memoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		},
		[]string{"type"},
	)

	mc.diskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "disk_usage_bytes",
			Help:      "Current disk usage in bytes",
		},
		[]string{"path", "type"},
	)

	mc.networkIO = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "network_io_bytes_total",
			Help:      "Total network I/O in bytes",
		},
		[]string{"interface", "direction"},
	)

	// 健康检查指标
	mc.checkStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "health",
			Name:      "check_status",
			Help:      "Health check status (1=healthy, 0=degraded, -1=unhealthy)",
		},
		[]string{"check", "type"},
	)

	mc.checkDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "health",
			Name:      "check_duration_seconds",
			Help:      "Health check duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"check", "type"},
	)

	mc.checkCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "health",
			Name:      "check_total",
			Help:      "Total number of health checks",
		},
		[]string{"check", "type", "status"},
	)

	// 告警指标
	mc.alertStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "alert",
			Name:      "status",
			Help:      "Alert status (1=firing, 0=resolved)",
		},
		[]string{"alert", "severity"},
	)

	mc.alertCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "alert",
			Name:      "total",
			Help:      "Total number of alerts",
		},
		[]string{"severity"},
	)

	mc.alertFired = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "alert",
			Name:      "fired_total",
			Help:      "Total number of fired alerts",
		},
		[]string{"alert", "severity"},
	)

	// 注册默认指标
	prometheus.MustRegister(
		mc.cpuUsage,
		mc.memoryUsage,
		mc.diskUsage,
		mc.networkIO,
		mc.checkStatus,
		mc.checkDuration,
		mc.checkCount,
		mc.alertStatus,
		mc.alertCount,
		mc.alertFired,
	)
}

// Start 启动指标收集
func (mc *MetricsCollector) Start() {
	mc.mu.Lock()
	if mc.running {
		mc.mu.Unlock()
		return
	}
	mc.running = true
	mc.mu.Unlock()

	mc.wg.Add(1)
	go mc.collectLoop()
}

// Stop 停止指标收集
func (mc *MetricsCollector) Stop() {
	mc.mu.Lock()
	mc.running = false
	mc.mu.Unlock()

	close(mc.stopCh)
	mc.wg.Wait()

	// 注销 Prometheus 指标
	prometheus.Unregister(mc.cpuUsage)
	prometheus.Unregister(mc.memoryUsage)
	prometheus.Unregister(mc.diskUsage)
	prometheus.Unregister(mc.networkIO)
	prometheus.Unregister(mc.checkStatus)
	prometheus.Unregister(mc.checkDuration)
	prometheus.Unregister(mc.checkCount)
	prometheus.Unregister(mc.alertStatus)
	prometheus.Unregister(mc.alertCount)
	prometheus.Unregister(mc.alertFired)
}

// collectLoop 指标采集循环
func (mc *MetricsCollector) collectLoop() {
	defer mc.wg.Done()

	ticker := time.NewTicker(mc.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.collect()
		}
	}
}

// collect 执行指标采集
func (mc *MetricsCollector) collect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 采集系统指标
	mc.collectSystemMetrics()

	// 采集健康检查指标
	if mc.healthChecker != nil {
		mc.collectHealthMetrics(ctx)
	}

	// 采集告警指标
	if mc.alertManager != nil {
		mc.collectAlertMetrics()
	}
}

// collectSystemMetrics 采集系统指标
func (mc *MetricsCollector) collectSystemMetrics() {
	// CPU 指标
	cpuUsage := mc.getCPUUsage()
	mc.cpuUsage.WithLabelValues("total").Set(cpuUsage)
	mc.recordTimeSeries("cpu_usage_total", cpuUsage, nil)

	// 内存指标
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mc.memoryUsage.WithLabelValues("alloc").Set(float64(m.Alloc))
	mc.memoryUsage.WithLabelValues("sys").Set(float64(m.Sys))
	mc.memoryUsage.WithLabelValues("heap").Set(float64(m.HeapAlloc))
	mc.memoryUsage.WithLabelValues("stack").Set(float64(m.StackInuse))

	mc.recordTimeSeries("memory_alloc", float64(m.Alloc), nil)
	mc.recordTimeSeries("memory_sys", float64(m.Sys), nil)

	// GC 指标
	mc.recordTimeSeries("gc_count", float64(m.NumGC), nil)
	mc.recordTimeSeries("gc_pause_total", float64(m.PauseTotalNs), nil)
}

// collectHealthMetrics 采集健康检查指标
func (mc *MetricsCollector) collectHealthMetrics(ctx context.Context) {
	report := mc.healthChecker.Check(ctx)

	for name, result := range report.Checks {
		// 状态指标
		statusValue := mc.statusToValue(result.Status)
		mc.checkStatus.WithLabelValues(name, string(result.Type)).Set(statusValue)

		// 耗时指标
		mc.checkDuration.WithLabelValues(name, string(result.Type)).Observe(result.Duration.Seconds())

		// 计数指标
		mc.checkCount.WithLabelValues(name, string(result.Type), string(result.Status)).Inc()

		// 记录时间序列
		mc.recordTimeSeries(
			fmt.Sprintf("health_%s_status", name),
			statusValue,
			map[string]string{"type": string(result.Type)},
		)
	}

	// 总体状态
	mc.recordTimeSeries("health_overall_status", mc.statusToValue(report.Status), nil)
}

// collectAlertMetrics 采集告警指标
func (mc *MetricsCollector) collectAlertMetrics() {
	alerts := mc.alertManager.GetAlerts()

	severityCounts := map[AlertSeverity]int{
		SeverityCritical: 0,
		SeverityWarning:  0,
		SeverityInfo:     0,
	}

	for _, alert := range alerts {
		if alert.State == AlertStateFiring {
			mc.alertStatus.WithLabelValues(alert.Name, string(alert.Severity)).Set(1)
			severityCounts[alert.Severity]++
		} else {
			mc.alertStatus.WithLabelValues(alert.Name, string(alert.Severity)).Set(0)
		}
	}

	// 更新时间序列
	mc.recordTimeSeries("alerts_critical_active", float64(severityCounts[SeverityCritical]), nil)
	mc.recordTimeSeries("alerts_warning_active", float64(severityCounts[SeverityWarning]), nil)
	mc.recordTimeSeries("alerts_info_active", float64(severityCounts[SeverityInfo]), nil)
}

// statusToValue 将状态转换为数值
func (mc *MetricsCollector) statusToValue(status HealthStatus) float64 {
	switch status {
	case StatusHealthy:
		return 1
	case StatusDegraded:
		return 0
	case StatusUnhealthy:
		return -1
	default:
		return -2
	}
}

// getCPUUsage 获取 CPU 使用率
func (mc *MetricsCollector) getCPUUsage() float64 {
	// 简化实现
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 基于 GC 频率估算负载
	avgGC := float64(m.PauseTotalNs) / float64(m.NumGC+1) / 1e6
	if avgGC > 10 {
		return 75.0 + avgGC/10
	}
	return 30.0 + float64(m.NumGC%10)*3
}

// recordTimeSeries 记录时间序列数据
func (mc *MetricsCollector) recordTimeSeries(name string, value float64, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	ts, exists := mc.timeSeries[name]
	if !exists {
		ts = &TimeSeriesData{
			Name:   name,
			Points: make([]DataPoint, 0, mc.maxDataPoints),
		}
		mc.timeSeries[name] = ts
	}

	// 添加新数据点
	point := DataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}
	ts.Points = append(ts.Points, point)

	// 限制数据点数量
	if len(ts.Points) > mc.maxDataPoints {
		ts.Points = ts.Points[len(ts.Points)-mc.maxDataPoints:]
	}
}

// GetTimeSeries 获取时间序列数据
func (mc *MetricsCollector) GetTimeSeries(name string) (*TimeSeriesData, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	ts, exists := mc.timeSeries[name]
	return ts, exists
}

// GetAllTimeSeries 获取所有时间序列
func (mc *MetricsCollector) GetAllTimeSeries() map[string]*TimeSeriesData {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*TimeSeriesData, len(mc.timeSeries))
	for k, v := range mc.timeSeries {
		result[k] = v
	}
	return result
}

// RegisterCustomMetric 注册自定义指标
func (mc *MetricsCollector) RegisterCustomMetric(name string, collector prometheus.Collector) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.customMetrics[name]; exists {
		return fmt.Errorf("metric %s already registered", name)
	}

	if err := prometheus.Register(collector); err != nil {
		return fmt.Errorf("failed to register metric: %w", err)
	}

	mc.customMetrics[name] = collector
	return nil
}

// UnregisterCustomMetric 注销自定义指标
func (mc *MetricsCollector) UnregisterCustomMetric(name string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	collector, exists := mc.customMetrics[name]
	if !exists {
		return fmt.Errorf("metric %s not found", name)
	}

	prometheus.Unregister(collector)
	delete(mc.customMetrics, name)
	return nil
}

// RecordCustomMetric 记录自定义指标值
func (mc *MetricsCollector) RecordCustomMetric(name string, value float64, labels map[string]string) {
	mc.recordTimeSeries(name, value, labels)
}

// GetAggregatedStats 获取聚合统计
func (mc *MetricsCollector) GetAggregatedStats() *AggregatedStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := &AggregatedStats{
		Timestamp: time.Now(),
		Metrics:   make(map[string]MetricSummary),
	}

	for name, ts := range mc.timeSeries {
		if len(ts.Points) == 0 {
			continue
		}

		summary := MetricSummary{
			Count:     len(ts.Points),
			FirstTime: ts.Points[0].Timestamp,
			LastTime:  ts.Points[len(ts.Points)-1].Timestamp,
		}

		// 计算统计值
		var sum, min, max float64
		min = ts.Points[0].Value
		max = ts.Points[0].Value

		for _, p := range ts.Points {
			sum += p.Value
			if p.Value < min {
				min = p.Value
			}
			if p.Value > max {
				max = p.Value
			}
		}

		summary.Sum = sum
		summary.Min = min
		summary.Max = max
		summary.Avg = sum / float64(len(ts.Points))
		summary.Latest = ts.Points[len(ts.Points)-1].Value

		stats.Metrics[name] = summary
	}

	return stats
}

// AggregatedStats 聚合统计
type AggregatedStats struct {
	Timestamp time.Time                `json:"timestamp"`
	Metrics   map[string]MetricSummary `json:"metrics"`
}

// MetricSummary 指标摘要
type MetricSummary struct {
	Count     int       `json:"count"`
	FirstTime time.Time `json:"first_time"`
	LastTime  time.Time `json:"last_time"`
	Sum       float64   `json:"sum"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Avg       float64   `json:"avg"`
	Latest    float64   `json:"latest"`
}

// ExportPrometheus 导出 Prometheus 格式文本
func (mc *MetricsCollector) ExportPrometheus() string {
	// 使用 Prometheus 默认注册器导出
	// 实际实现中可以使用 promhttp.Handler()
	return "# Prometheus metrics available at /metrics endpoint"
}

// CollectOnce 执行一次即时采集
func (mc *MetricsCollector) CollectOnce(ctx context.Context) *CollectionResult {
	result := &CollectionResult{
		Timestamp: time.Now(),
		System:    mc.collectSystemOnce(),
		Health:    mc.collectHealthOnce(ctx),
		Alerts:    mc.collectAlertsOnce(),
	}

	return result
}

// collectSystemOnce 采集一次系统指标
func (mc *MetricsCollector) collectSystemOnce() *SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &SystemMetrics{
		CPUUsage:     mc.getCPUUsage(),
		MemoryAlloc:  m.Alloc,
		MemorySys:    m.Sys,
		MemoryHeap:   m.HeapAlloc,
		MemoryStack:  m.StackInuse,
		GCCount:      m.NumGC,
		GCPauseTotal: m.PauseTotalNs,
		Goroutines:   runtime.NumGoroutine(),
	}
}

// collectHealthOnce 采集一次健康检查指标
func (mc *MetricsCollector) collectHealthOnce(ctx context.Context) *HealthMetrics {
	if mc.healthChecker == nil {
		return nil
	}

	report := mc.healthChecker.Check(ctx)

	return &HealthMetrics{
		OverallStatus: string(report.Status),
		TotalChecks:   report.Summary.Total,
		Healthy:       report.Summary.Healthy,
		Unhealthy:     report.Summary.Unhealthy,
		Degraded:      report.Summary.Degraded,
		Uptime:        report.Uptime.String(),
	}
}

// collectAlertsOnce 采集一次告警指标
func (mc *MetricsCollector) collectAlertsOnce() *AlertMetrics {
	if mc.alertManager == nil {
		return nil
	}

	stats := mc.alertManager.Stats()

	return &AlertMetrics{
		TotalAlerts:    stats.TotalAlerts,
		ActiveAlerts:   stats.ActiveAlerts,
		CriticalAlerts: stats.CriticalAlerts,
		WarningAlerts:  stats.WarningAlerts,
		InfoAlerts:     stats.InfoAlerts,
	}
}

// CollectionResult 采集结果
type CollectionResult struct {
	Timestamp time.Time      `json:"timestamp"`
	System    *SystemMetrics `json:"system,omitempty"`
	Health    *HealthMetrics `json:"health,omitempty"`
	Alerts    *AlertMetrics  `json:"alerts,omitempty"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryAlloc  uint64  `json:"memory_alloc"`
	MemorySys    uint64  `json:"memory_sys"`
	MemoryHeap   uint64  `json:"memory_heap"`
	MemoryStack  uint64  `json:"memory_stack"`
	GCCount      uint32  `json:"gc_count"`
	GCPauseTotal uint64  `json:"gc_pause_total"`
	Goroutines   int     `json:"goroutines"`
}

// HealthMetrics 健康检查指标
type HealthMetrics struct {
	OverallStatus string `json:"overall_status"`
	TotalChecks   int    `json:"total_checks"`
	Healthy       int    `json:"healthy"`
	Unhealthy     int    `json:"unhealthy"`
	Degraded      int    `json:"degraded"`
	Uptime        string `json:"uptime"`
}

// AlertMetrics 告警指标
type AlertMetrics struct {
	TotalAlerts    int `json:"total_alerts"`
	ActiveAlerts   int `json:"active_alerts"`
	CriticalAlerts int `json:"critical_alerts"`
	WarningAlerts  int `json:"warning_alerts"`
	InfoAlerts     int `json:"info_alerts"`
}

// GetMetricNames 获取所有指标名称
func (mc *MetricsCollector) GetMetricNames() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	names := make([]string, 0, len(mc.timeSeries))
	for name := range mc.timeSeries {
		names = append(names, name)
	}
	return names
}

// PurgeOldData 清理过期数据
func (mc *MetricsCollector) PurgeOldData(olderThan time.Duration) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	purged := 0

	for name, ts := range mc.timeSeries {
		// 找到第一个在截止时间之后的点
		firstValid := -1
		for i, p := range ts.Points {
			if p.Timestamp.After(cutoff) {
				firstValid = i
				break
			}
		}

		if firstValid > 0 {
			ts.Points = ts.Points[firstValid:]
			purged += firstValid
		} else if firstValid == -1 {
			// 所有点都过期
			delete(mc.timeSeries, name)
			purged += len(ts.Points)
		}
	}

	return purged
}
