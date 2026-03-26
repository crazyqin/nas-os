// Package nvmeof - 连接状态监控
// 监控 NVMe-oF 连接状态、性能指标和告警
package nvmeof

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ========== 连接监控器 ==========

// ConnectionMonitor 连接监控器
type ConnectionMonitor struct {
	mu sync.RWMutex

	config *NVMeOFConfig

	// 监控状态
	running atomic.Bool

	// 连接状态缓存
	connectionStates map[string]*ConnectionStatus

	// 性能指标
	metrics *ConnectionMetrics

	// 告警回调
	alertHandler AlertHandler

	// 事件通道
	eventCh chan<- NVMeOFEvent
}

// ConnectionStatus 连接状态
type ConnectionStatus struct {
	// 基本信息
	ControllerName string          `json:"controllerName"`
	SubsystemNQN   string          `json:"subsystemNqn"`
	Transport      TransportType   `json:"transport"`
	Address        string          `json:"address"`
	Port           string          `json:"port"`

	// 连接状态
	State           ConnectionState `json:"state"`
	LastStateChange time.Time       `json:"lastStateChange"`
	ConnectedAt     time.Time       `json:"connectedAt,omitempty"`

	// 重连信息
	ReconnectAttempts int       `json:"reconnectAttempts"`
	LastReconnect     time.Time `json:"lastReconnect,omitempty"`
	NextReconnect     time.Time `json:"nextReconnect,omitempty"`

	// 性能指标
	Latency          uint64        `json:"latency"`          // 微秒
	ThroughputRead   uint64        `json:"throughputRead"`   // bytes/s
	ThroughputWrite  uint64        `json:"throughputWrite"`  // bytes/s
	IOps             uint64        `json:"ioPs"`             // IOPS
	QueueDepth       uint32        `json:"queueDepth"`       // 当前队列深度
	MaxQueueDepth    uint32        `json:"maxQueueDepth"`    // 最大队列深度

	// 错误统计
	Errors          uint64 `json:"errors"`
	TimeoutErrors   uint64 `json:"timeoutErrors"`
	TransportErrors uint64 `json:"transportErrors"`

	// 健康状态
	HealthScore     int       `json:"healthScore"`     // 0-100
	LastHealthCheck time.Time `json:"lastHealthCheck"`
}

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	mu sync.RWMutex

	// 汇总指标
	TotalConnections    int            `json:"totalConnections"`
	ActiveConnections   int            `json:"activeConnections"`
	FailedConnections   int            `json:"failedConnections"`
	TotalBytesRead      uint64         `json:"totalBytesRead"`
	TotalBytesWritten   uint64         `json:"totalBytesWritten"`
	TotalIOps           uint64         `json:"totalIOps"`
	AvgLatency          uint64         `json:"avgLatency"`       // 微秒
	P99Latency          uint64         `json:"p99Latency"`       // 微秒
	MaxLatency          uint64         `json:"maxLatency"`       // 微秒

	// 历史数据
	History []MetricPoint `json:"history,omitempty"`
}

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	IOps      uint64    `json:"ioPs"`
	Latency   uint64    `json:"latency"`
	ReadBW    uint64    `json:"readBw"`   // bytes/s
	WriteBW   uint64    `json:"writeBw"`  // bytes/s
}

// AlertHandler 告警处理器
type AlertHandler func(alert *ConnectionAlert)

// ConnectionAlert 连接告警
type ConnectionAlert struct {
	Type        AlertType       `json:"type"`
	Severity    AlertSeverity   `json:"severity"`
	Controller  string          `json:"controller"`
	Subsystem   string          `json:"subsystem,omitempty"`
	Message     string          `json:"message"`
	Value       interface{}     `json:"value,omitempty"`
	Threshold   interface{}     `json:"threshold,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
}

// AlertType 告警类型
type AlertType string

const (
	// AlertTypeConnectionLost 连接丢失
	AlertTypeConnectionLost AlertType = "connection_lost"
	// AlertTypeHighLatency 高延迟
	AlertTypeHighLatency AlertType = "high_latency"
	// AlertTypeHighErrorRate 高错误率
	AlertTypeHighErrorRate AlertType = "high_error_rate"
	// AlertTypeQueueDepthHigh 队列深度高
	AlertTypeQueueDepthHigh AlertType = "queue_depth_high"
	// AlertTypeReconnectFailed 重连失败
	AlertTypeReconnectFailed AlertType = "reconnect_failed"
	// AlertTypeHealthDegraded 健康状态降级
	AlertTypeHealthDegraded AlertType = "health_degraded"
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	// AlertSeverityInfo 信息
	AlertSeverityInfo AlertSeverity = "info"
	// AlertSeverityWarning 警告
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityCritical 严重
	AlertSeverityCritical AlertSeverity = "critical"
)

// NewConnectionMonitor 创建连接监控器
func NewConnectionMonitor(config *NVMeOFConfig) *ConnectionMonitor {
	return &ConnectionMonitor{
		config:           config,
		connectionStates: make(map[string]*ConnectionStatus),
		metrics:          &ConnectionMetrics{},
	}
}

// Start 启动监控
func (m *ConnectionMonitor) Start(ctx context.Context) {
	if !m.running.CompareAndSwap(false, true) {
		return
	}

	go m.monitorLoop(ctx)
}

// Stop 停止监控
func (m *ConnectionMonitor) Stop() {
	m.running.Store(false)
}

// SetEventChannel 设置事件通道
func (m *ConnectionMonitor) SetEventChannel(ch chan<- NVMeOFEvent) {
	m.eventCh = ch
}

// SetAlertHandler 设置告警处理器
func (m *ConnectionMonitor) SetAlertHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandler = handler
}

// monitorLoop 监控循环
func (m *ConnectionMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.Monitoring.MetricsInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.running.Load() {
				return
			}
			m.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (m *ConnectionMonitor) collectMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 实际实现需要:
	// - 读取 /sys/class/nvme/nvmeX/statistics/
	// - 或执行 nvme list
	// - 或使用 libnvme

	// 更新指标
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	// 添加历史数据点
	if m.config.Monitoring.IOStats {
		point := MetricPoint{
			Timestamp: time.Now(),
			IOps:      m.metrics.TotalIOps,
			Latency:   m.metrics.AvgLatency,
			ReadBW:    m.metrics.TotalBytesRead,
			WriteBW:   m.metrics.TotalBytesWritten,
		}

		// 保留最近 1000 个数据点
		if len(m.metrics.History) >= 1000 {
			m.metrics.History = m.metrics.History[1:]
		}
		m.metrics.History = append(m.metrics.History, point)
	}

	// 检查告警阈值
	m.checkAlerts()
}

// checkAlerts 检查告警
func (m *ConnectionMonitor) checkAlerts() {
	thresholds := m.config.Monitoring.AlertThresholds

	// 检查延迟
	if m.metrics.AvgLatency > uint64(thresholds.LatencyHigh) {
		m.sendAlert(&ConnectionAlert{
			Type:      AlertTypeHighLatency,
			Severity:  AlertSeverityWarning,
			Message:   "Average latency exceeds threshold",
			Value:     m.metrics.AvgLatency,
			Threshold: thresholds.LatencyHigh,
			Timestamp: time.Now(),
		})
	}

	// 检查队列深度
	// 这里简化实现，实际需要遍历所有连接状态
}

// sendAlert 发送告警
func (m *ConnectionMonitor) sendAlert(alert *ConnectionAlert) {
	if m.alertHandler != nil {
		m.alertHandler(alert)
	}

	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:    EventAlert,
			Message: alert.Message,
			Error:   fmt.Errorf("alert: %s", alert.Type),
			Time:    alert.Timestamp,
		}
	}
}

// ========== 状态更新方法 ==========

// UpdateConnectionState 更新连接状态
func (m *ConnectionMonitor) UpdateConnectionStatus(ctrlName string, status *ConnectionStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查状态变化
	oldStatus, exists := m.connectionStates[ctrlName]
	if exists && oldStatus.State != status.State {
		// 状态变化，发送事件
		if m.eventCh != nil {
			m.eventCh <- NVMeOFEvent{
				Type:        EventConnectionStateChanged,
				Message:     fmt.Sprintf("Connection %s state changed: %s -> %s", ctrlName, oldStatus.State, status.State),
				Controller:  ctrlName,
				Time:        time.Now(),
			}
		}
	}

	m.connectionStates[ctrlName] = status
}

// RemoveConnection 移除连接
func (m *ConnectionMonitor) RemoveConnection(ctrlName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.connectionStates, ctrlName)
}

// GetConnectionStatus 获取连接状态
func (m *ConnectionMonitor) GetConnectionStatus(ctrlName string) (*ConnectionStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.connectionStates[ctrlName]
	return status, exists
}

// GetAllConnectionStatuses 获取所有连接状态
func (m *ConnectionMonitor) GetAllConnectionStatuses() map[string]*ConnectionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ConnectionStatus, len(m.connectionStates))
	for k, v := range m.connectionStates {
		result[k] = v
	}
	return result
}

// GetMetrics 获取指标
func (m *ConnectionMonitor) GetMetrics() *ConnectionMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	// 返回副本（不复制锁）
	metrics := &ConnectionMetrics{
		TotalConnections:    m.metrics.TotalConnections,
		ActiveConnections:   m.metrics.ActiveConnections,
		FailedConnections:   m.metrics.FailedConnections,
		TotalBytesRead:      m.metrics.TotalBytesRead,
		TotalBytesWritten:   m.metrics.TotalBytesWritten,
		TotalIOps:           m.metrics.TotalIOps,
		AvgLatency:          m.metrics.AvgLatency,
		P99Latency:          m.metrics.P99Latency,
		MaxLatency:          m.metrics.MaxLatency,
	}
	if len(m.metrics.History) > 0 {
		metrics.History = make([]MetricPoint, len(m.metrics.History))
		copy(metrics.History, m.metrics.History)
	}
	return metrics
}

// GetHealthScore 获取健康分数
func (m *ConnectionMonitor) GetHealthScore(ctrlName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, exists := m.connectionStates[ctrlName]; exists {
		return status.HealthScore
	}
	return 0
}

// ========== 辅助方法 ==========

// CalculateHealthScore 计算健康分数
func CalculateHealthScore(status *ConnectionStatus) int {
	score := 100

	// 连接状态影响
	if status.State != ConnectionStateUp {
		score -= 50
	}

	// 错误率影响
	if status.Errors > 0 {
		errorRate := float64(status.Errors) / float64(status.IOps+1) * 100
		score -= int(errorRate * 0.5)
	}

	// 延迟影响
	if status.Latency > 1000 { // > 1ms
		score -= 10
	}
	if status.Latency > 10000 { // > 10ms
		score -= 20
	}

	if score < 0 {
		score = 0
	}

	return score
}

