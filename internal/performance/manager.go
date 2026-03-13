package performance

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 性能监控管理器
type Manager struct {
	logger     *zap.Logger
	config     Config

	collector  *SystemCollector
	storage    *StorageCollector
	health     *HealthChecker
	alerts     *AlertManager
	prometheus *PrometheusExporter
	monitor    *PerformanceMonitor

	mu         sync.RWMutex
	running    bool
	cancel     context.CancelFunc
}

// Config 配置
type Config struct {
	// 采集间隔
	CollectInterval time.Duration `json:"collect_interval"`
	// 健康检查间隔
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	// 告警检查间隔
	AlertCheckInterval time.Duration `json:"alert_check_interval"`
	// 历史数据大小
	HistorySize int `json:"history_size"`
	// Prometheus 端口
	PrometheusAddr string `json:"prometheus_addr"`
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		CollectInterval:     10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		AlertCheckInterval:  30 * time.Second,
		HistorySize:         100,
		PrometheusAddr:      ":9090",
	}
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, config Config) *Manager {
	m := &Manager{
		logger: logger,
		config: config,
	}

	// 初始化各组件
	m.collector = NewSystemCollector(logger, config.HistorySize)
	m.storage = NewStorageCollector(logger, m.collector, config.HistorySize)
	m.health = NewHealthChecker(logger, m.collector, m.storage)
	m.monitor = NewPerformanceMonitor(logger)
	m.alerts = NewAlertManager(logger, m.collector, m.storage, m.health)
	m.prometheus = NewPrometheusExporterExtended(m.monitor, m.collector, m.storage, m.health, m.alerts)

	return m
}

// Start 启动监控
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	m.running = true
	ctx, m.cancel = context.WithCancel(ctx)

	// 启动健康检查
	m.health.Start(ctx)

	// 启动告警检查
	m.alerts.Start(ctx, m.config.AlertCheckInterval)

	m.logger.Info("性能监控管理器已启动")

	return nil
}

// Stop 停止监控
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	if m.cancel != nil {
		m.cancel()
	}

	m.running = false
	m.logger.Info("性能监控管理器已停止")
}

// Collect 立即采集指标
func (m *Manager) Collect() *SystemMetricsSummary {
	return m.collector.Collect()
}

// GetHealth 获取健康状态
func (m *Manager) GetHealth() *SystemHealth {
	return m.health.GetHealth()
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts() []*Alert {
	return m.alerts.GetAlerts()
}

// GetMetrics 获取性能指标
func (m *Manager) GetMetrics() *Metrics {
	return m.monitor.GetMetrics()
}

// GetStorageMetrics 获取存储指标
func (m *Manager) GetStorageMetrics() *StorageMetrics {
	return m.storage.Collect()
}

// GetPrometheusHandler 获取 Prometheus 处理器
func (m *Manager) GetPrometheusHandler() *PrometheusExporter {
	return m.prometheus
}

// GetMonitor 获取性能监控器
func (m *Manager) GetMonitor() *PerformanceMonitor {
	return m.monitor
}

// GetCollector 获取系统收集器
func (m *Manager) GetCollector() *SystemCollector {
	return m.collector
}

// GetStorageCollector 获取存储收集器
func (m *Manager) GetStorageCollector() *StorageCollector {
	return m.storage
}

// GetHealthChecker 获取健康检查器
func (m *Manager) GetHealthChecker() *HealthChecker {
	return m.health
}

// GetAlertManager 获取告警管理器
func (m *Manager) GetAlertManager() *AlertManager {
	return m.alerts
}

// SetAlertCallback 设置告警回调
func (m *Manager) SetAlertCallback(callback func(alert *Alert)) {
	m.alerts.SetCallbacks(callback, nil)
}

// RecordAPICall 记录 API 调用
func (m *Manager) RecordAPICall(path, method string, duration time.Duration, statusCode int) {
	m.monitor.RecordAPICall(path, method, duration, statusCode)
}

// RecordFileOperation 记录文件操作
func (m *Manager) RecordFileOperation(opType string, duration time.Duration, bytes int64) {
	switch opType {
	case "list":
		m.monitor.RecordFileList(duration)
	case "upload":
		m.monitor.RecordUpload(bytes)
	case "download":
		m.monitor.RecordDownload(bytes)
	case "thumbnail":
		m.monitor.RecordThumbnail(duration)
	}
}

// Middleware API 性能监控中间件
func (m *Manager) Middleware() interface{} {
	return m.monitor.Middleware()
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetAPIHandlers 获取 API 处理器
func (m *Manager) GetAPIHandlers() *APIHandlers {
	return NewAPIHandlers(
		m.logger,
		m.collector,
		m.storage,
		m.health,
		m.alerts,
		m.prometheus,
		m.monitor,
	)
}