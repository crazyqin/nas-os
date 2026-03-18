package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Monitor 插件状态监控器
type Monitor struct {
	manager   *Manager
	states    map[string]*HealthStatus
	alertChan chan Alert
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	config    MonitorConfig
	stateFile string
}

// HealthStatus 插件健康状态
type HealthStatus struct {
	PluginID          string                 `json:"plugin_id"`
	Status            StatusType             `json:"status"`
	LastCheckTime     time.Time              `json:"last_check_time"`
	LastHealthyTime   time.Time              `json:"last_healthy_time"`
	ConsecutiveErrors int                    `json:"consecutive_errors"`
	LastError         string                 `json:"last_error,omitempty"`
	LastErrorTime     time.Time              `json:"last_error_time,omitempty"`
	Metrics           map[string]interface{} `json:"metrics,omitempty"`
	UptimeSeconds     int64                  `json:"uptime_seconds"`
	StartTime         time.Time              `json:"start_time"`
	RestartCount      int                    `json:"restart_count"`
	LastRestartTime   time.Time              `json:"last_restart_time,omitempty"`
}

// StatusType 插件状态类型
type StatusType string

const (
	// StatusHealthy indicates the plugin is running normally
	StatusHealthy   StatusType = "healthy"
	StatusDegraded  StatusType = "degraded"
	StatusUnhealthy StatusType = "unhealthy"
	StatusUnknown   StatusType = "unknown"
)

// Alert 插件告警
type Alert struct {
	PluginID  string                 `json:"plugin_id"`
	Type      AlertType              `json:"type"`
	Severity  AlertSeverity          `json:"severity"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeHealthChanged   AlertType = "health_changed"
	AlertTypePluginCrashed   AlertType = "plugin_crashed"
	AlertTypePluginRecovered AlertType = "plugin_recovered"
	AlertTypeAutoRestarted   AlertType = "auto_restarted"
	AlertTypeHighErrorRate   AlertType = "high_error_rate"
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityError    AlertSeverity = "error"
	SeverityCritical AlertSeverity = "critical"
)

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval      time.Duration `json:"check_interval"`      // 检查间隔
	UnhealthyThreshold int           `json:"unhealthy_threshold"` // 不健康阈值（连续错误次数）
	AutoRestart        bool          `json:"auto_restart"`        // 自动重启
	MaxRestarts        int           `json:"max_restarts"`        // 最大重启次数
	RestartCooldown    time.Duration `json:"restart_cooldown"`    // 重启冷却时间
	StatePersistence   bool          `json:"state_persistence"`   // 状态持久化
}

// DefaultMonitorConfig 默认监控配置
var DefaultMonitorConfig = MonitorConfig{
	CheckInterval:      30 * time.Second,
	UnhealthyThreshold: 3,
	AutoRestart:        true,
	MaxRestarts:        5,
	RestartCooldown:    5 * time.Minute,
	StatePersistence:   true,
}

// NewMonitor 创建插件监控器
func NewMonitor(manager *Manager, config MonitorConfig) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Monitor{
		manager:   manager,
		states:    make(map[string]*HealthStatus),
		alertChan: make(chan Alert, 100),
		ctx:       ctx,
		cancel:    cancel,
		config:    config,
	}

	if config.StatePersistence {
		m.stateFile = filepath.Join(manager.configDir, "plugin_health.json")
		m.loadPersistedStates()
	}

	return m
}

// Start 启动监控
func (m *Monitor) Start() {
	go m.monitorLoop()
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.cancel()
}

// GetAlerts 获取告警通道
func (m *Monitor) GetAlerts() <-chan Alert {
	return m.alertChan
}

// GetPluginStatus 获取插件状态
func (m *Monitor) GetPluginStatus(pluginID string) *HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[pluginID]
}

// GetAllStatuses 获取所有插件状态
func (m *Monitor) GetAllStatuses() map[string]*HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*HealthStatus)
	for k, v := range m.states {
		result[k] = v
	}
	return result
}

// GetHealthyCount 获取健康插件数量
func (m *Monitor) GetHealthyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, status := range m.states {
		if status.Status == StatusHealthy {
			count++
		}
	}
	return count
}

// GetUnhealthyPlugins 获取不健康的插件列表
func (m *Monitor) GetUnhealthyPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var unhealthy []string
	for id, status := range m.states {
		if status.Status == StatusUnhealthy || status.Status == StatusDegraded {
			unhealthy = append(unhealthy, id)
		}
	}
	return unhealthy
}

// ForceHealthCheck 强制执行健康检查
func (m *Monitor) ForceHealthCheck(pluginID string) *HealthStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, exists := m.states[pluginID]
	if !exists {
		status = &HealthStatus{
			PluginID: pluginID,
		}
		m.states[pluginID] = status
	}

	m.checkPluginHealth(pluginID, status)
	return status
}

// monitorLoop 监控循环
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runHealthChecks()
		}
	}
}

// runHealthChecks 执行健康检查
func (m *Monitor) runHealthChecks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取所有插件状态
	pluginStates := m.manager.List()

	for _, state := range pluginStates {
		if !state.Enabled {
			continue
		}

		status, exists := m.states[state.ID]
		if !exists {
			status = &HealthStatus{
				PluginID:  state.ID,
				StartTime: state.InstalledAt,
			}
			m.states[state.ID] = status
		}

		// 执行健康检查
		m.checkPluginHealth(state.ID, status)
	}

	// 持久化状态
	if m.config.StatePersistence {
		m.persistStates()
	}
}

// checkPluginHealth 检查单个插件健康状态
func (m *Monitor) checkPluginHealth(pluginID string, status *HealthStatus) {
	status.LastCheckTime = time.Now()

	// 获取插件实例
	inst, exists := m.manager.loader.GetInstance(pluginID)
	if !exists {
		status.Status = StatusUnknown
		status.LastError = "插件实例不存在"
		status.ConsecutiveErrors++
		return
	}

	// 检查运行状态
	if !inst.Running {
		if inst.Enabled {
			// 应该运行但未运行，可能崩溃了
			status.Status = StatusUnhealthy
			status.ConsecutiveErrors++
			status.LastError = "插件未运行"
			status.LastErrorTime = time.Now()

			// 发送告警
			m.sendAlert(Alert{
				PluginID:  pluginID,
				Type:      AlertTypePluginCrashed,
				Severity:  SeverityError,
				Message:   fmt.Sprintf("插件 %s 未运行", pluginID),
				Timestamp: time.Now(),
			})

			// 尝试自动重启
			if m.config.AutoRestart && status.RestartCount < m.config.MaxRestarts {
				m.attemptRestart(pluginID, status)
			}
		} else {
			status.Status = StatusUnknown
		}
		return
	}

	// 执行插件自定义健康检查
	if healthChecker, ok := inst.Plugin.(HealthChecker); ok {
		if err := healthChecker.HealthCheck(); err != nil {
			status.ConsecutiveErrors++
			status.LastError = err.Error()
			status.LastErrorTime = time.Now()

			if status.ConsecutiveErrors >= m.config.UnhealthyThreshold {
				oldStatus := status.Status
				status.Status = StatusUnhealthy

				if oldStatus != StatusUnhealthy {
					m.sendAlert(Alert{
						PluginID:  pluginID,
						Type:      AlertTypeHealthChanged,
						Severity:  SeverityWarning,
						Message:   fmt.Sprintf("插件 %s 状态变为不健康: %s", pluginID, err.Error()),
						Timestamp: time.Now(),
						Details: map[string]interface{}{
							"previous_status": oldStatus,
							"error":           err.Error(),
						},
					})
				}
			} else {
				status.Status = StatusDegraded
			}
			return
		}
	}

	// 健康检查通过
	previousStatus := status.Status
	status.Status = StatusHealthy
	status.LastHealthyTime = time.Now()
	status.ConsecutiveErrors = 0
	status.LastError = ""
	status.UptimeSeconds = int64(time.Since(status.StartTime).Seconds())

	// 收集指标
	if metricsCollector, ok := inst.Plugin.(MetricsCollector); ok {
		status.Metrics = metricsCollector.CollectMetrics()
	}

	// 如果从非健康状态恢复
	if previousStatus == StatusUnhealthy || previousStatus == StatusDegraded {
		m.sendAlert(Alert{
			PluginID:  pluginID,
			Type:      AlertTypePluginRecovered,
			Severity:  SeverityInfo,
			Message:   fmt.Sprintf("插件 %s 已恢复健康", pluginID),
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"previous_status": previousStatus,
			},
		})
	}
}

// attemptRestart 尝试重启插件
func (m *Monitor) attemptRestart(pluginID string, status *HealthStatus) {
	// 检查冷却时间
	if !status.LastRestartTime.IsZero() {
		if time.Since(status.LastRestartTime) < m.config.RestartCooldown {
			return
		}
	}

	// 尝试重启
	err := m.manager.Enable(pluginID)
	if err != nil {
		log.Printf("自动重启插件 %s 失败: %v", pluginID, err)
		return
	}

	status.RestartCount++
	status.LastRestartTime = time.Now()
	status.StartTime = time.Now()

	m.sendAlert(Alert{
		PluginID:  pluginID,
		Type:      AlertTypeAutoRestarted,
		Severity:  SeverityWarning,
		Message:   fmt.Sprintf("插件 %s 已自动重启 (第 %d 次)", pluginID, status.RestartCount),
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"restart_count": status.RestartCount,
		},
	})
}

// sendAlert 发送告警
func (m *Monitor) sendAlert(alert Alert) {
	select {
	case m.alertChan <- alert:
	default:
		log.Printf("告警通道已满，丢弃告警: %+v", alert)
	}
}

// persistStates 持久化状态
func (m *Monitor) persistStates() {
	if m.stateFile == "" {
		return
	}

	data, err := json.MarshalIndent(m.states, "", "  ")
	if err != nil {
		log.Printf("序列化插件状态失败: %v", err)
		return
	}

	if err := os.WriteFile(m.stateFile, data, 0644); err != nil {
		log.Printf("保存插件状态失败: %v", err)
	}
}

// loadPersistedStates 加载持久化状态
func (m *Monitor) loadPersistedStates() {
	if m.stateFile == "" {
		return
	}

	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("读取插件状态文件失败: %v", err)
		}
		return
	}

	if err := json.Unmarshal(data, &m.states); err != nil {
		log.Printf("解析插件状态失败: %v", err)
	}
}

// HealthChecker 健康检查接口
type HealthChecker interface {
	HealthCheck() error
}

// MetricsCollector 指标收集接口
type MetricsCollector interface {
	CollectMetrics() map[string]interface{}
}

// GetMonitorSummary 获取监控摘要
func (m *Monitor) GetMonitorSummary() MonitorSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := MonitorSummary{
		TotalPlugins:  len(m.states),
		LastCheckTime: time.Now(),
	}

	for _, status := range m.states {
		switch status.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusDegraded:
			summary.Degraded++
		case StatusUnhealthy:
			summary.Unhealthy++
		default:
			summary.Unknown++
		}
	}

	return summary
}

// MonitorSummary 监控摘要
type MonitorSummary struct {
	TotalPlugins  int       `json:"total_plugins"`
	Healthy       int       `json:"healthy"`
	Degraded      int       `json:"degraded"`
	Unhealthy     int       `json:"unhealthy"`
	Unknown       int       `json:"unknown"`
	LastCheckTime time.Time `json:"last_check_time"`
}

// ResetRestartCount 重置重启计数
func (m *Monitor) ResetRestartCount(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, exists := m.states[pluginID]; exists {
		status.RestartCount = 0
	}
}

// ClearError 清除错误状态
func (m *Monitor) ClearError(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, exists := m.states[pluginID]; exists {
		status.ConsecutiveErrors = 0
		status.LastError = ""
	}
}
