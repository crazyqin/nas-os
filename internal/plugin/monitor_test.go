package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// createTestMonitor 创建测试监控器
func createTestMonitor(t *testing.T) *Monitor {
	mgr, err := NewManager(ManagerConfig{
		PluginDir: "/tmp/test-plugins-monitor",
		ConfigDir: "/tmp/test-config-monitor",
		DataDir:   "/tmp/test-data-monitor",
	})
	if err != nil {
		t.Skipf("无法创建测试管理器: %v", err)
		return nil
	}
	return NewMonitor(mgr, DefaultMonitorConfig)
}

func TestMonitor_New(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}
	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.states)
	assert.NotNil(t, monitor.alertChan)
}

func TestMonitor_GetPluginStatus(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	// 获取不存在的插件状态
	status := monitor.GetPluginStatus("nonexistent")
	assert.Nil(t, status)

	// 添加状态
	monitor.mu.Lock()
	monitor.states["test-plugin"] = &HealthStatus{
		PluginID:      "test-plugin",
		Status:        StatusHealthy,
		LastCheckTime: time.Now(),
	}
	monitor.mu.Unlock()

	// 获取存在的状态
	status = monitor.GetPluginStatus("test-plugin")
	assert.NotNil(t, status)
	assert.Equal(t, "test-plugin", status.PluginID)
	assert.Equal(t, StatusHealthy, status.Status)
}

func TestMonitor_GetAllStatuses(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	// 添加多个状态
	monitor.mu.Lock()
	monitor.states["plugin1"] = &HealthStatus{
		PluginID: "plugin1",
		Status:   StatusHealthy,
	}
	monitor.states["plugin2"] = &HealthStatus{
		PluginID: "plugin2",
		Status:   StatusUnhealthy,
	}
	monitor.mu.Unlock()

	statuses := monitor.GetAllStatuses()
	assert.Len(t, statuses, 2)
}

func TestMonitor_GetHealthyCount(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	monitor.mu.Lock()
	monitor.states["plugin1"] = &HealthStatus{PluginID: "plugin1", Status: StatusHealthy}
	monitor.states["plugin2"] = &HealthStatus{PluginID: "plugin2", Status: StatusHealthy}
	monitor.states["plugin3"] = &HealthStatus{PluginID: "plugin3", Status: StatusUnhealthy}
	monitor.mu.Unlock()

	count := monitor.GetHealthyCount()
	assert.Equal(t, 2, count)
}

func TestMonitor_GetUnhealthyPlugins(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	monitor.mu.Lock()
	monitor.states["plugin1"] = &HealthStatus{PluginID: "plugin1", Status: StatusHealthy}
	monitor.states["plugin2"] = &HealthStatus{PluginID: "plugin2", Status: StatusUnhealthy}
	monitor.states["plugin3"] = &HealthStatus{PluginID: "plugin3", Status: StatusDegraded}
	monitor.mu.Unlock()

	unhealthy := monitor.GetUnhealthyPlugins()
	assert.Len(t, unhealthy, 2)
	assert.Contains(t, unhealthy, "plugin2")
	assert.Contains(t, unhealthy, "plugin3")
}

func TestMonitor_GetMonitorSummary(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	monitor.mu.Lock()
	monitor.states["plugin1"] = &HealthStatus{PluginID: "plugin1", Status: StatusHealthy}
	monitor.states["plugin2"] = &HealthStatus{PluginID: "plugin2", Status: StatusUnhealthy}
	monitor.states["plugin3"] = &HealthStatus{PluginID: "plugin3", Status: StatusDegraded}
	monitor.states["plugin4"] = &HealthStatus{PluginID: "plugin4", Status: StatusUnknown}
	monitor.mu.Unlock()

	summary := monitor.GetMonitorSummary()
	assert.Equal(t, 4, summary.TotalPlugins)
	assert.Equal(t, 1, summary.Healthy)
	assert.Equal(t, 1, summary.Unhealthy)
	assert.Equal(t, 1, summary.Degraded)
	assert.Equal(t, 1, summary.Unknown)
}

func TestHealthStatus(t *testing.T) {
	now := time.Now()
	status := &HealthStatus{
		PluginID:          "test-plugin",
		Status:            StatusHealthy,
		LastCheckTime:     now,
		LastHealthyTime:   now,
		ConsecutiveErrors: 0,
		UptimeSeconds:     3600,
		StartTime:         now.Add(-time.Hour),
		RestartCount:      0,
	}

	assert.Equal(t, "test-plugin", status.PluginID)
	assert.Equal(t, StatusHealthy, status.Status)
	assert.Equal(t, int64(3600), status.UptimeSeconds)
}

func TestAlert(t *testing.T) {
	alert := Alert{
		PluginID:  "test-plugin",
		Type:      AlertTypeHealthChanged,
		Severity:  SeverityWarning,
		Message:   "Plugin health changed",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"previous_status": "healthy",
		},
	}

	assert.Equal(t, "test-plugin", alert.PluginID)
	assert.Equal(t, AlertTypeHealthChanged, alert.Type)
	assert.Equal(t, SeverityWarning, alert.Severity)
}

func TestMonitorConfig(t *testing.T) {
	config := DefaultMonitorConfig

	assert.Equal(t, 30*time.Second, config.CheckInterval)
	assert.Equal(t, 3, config.UnhealthyThreshold)
	assert.True(t, config.AutoRestart)
	assert.Equal(t, 5, config.MaxRestarts)
	assert.Equal(t, 5*time.Minute, config.RestartCooldown)
	assert.True(t, config.StatePersistence)
}

func TestMonitor_ResetRestartCount(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	monitor.mu.Lock()
	monitor.states["test-plugin"] = &HealthStatus{
		PluginID:     "test-plugin",
		RestartCount: 5,
	}
	monitor.mu.Unlock()

	monitor.ResetRestartCount("test-plugin")

	status := monitor.GetPluginStatus("test-plugin")
	assert.Equal(t, 0, status.RestartCount)
}

func TestMonitor_ClearError(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	monitor.mu.Lock()
	monitor.states["test-plugin"] = &HealthStatus{
		PluginID:          "test-plugin",
		ConsecutiveErrors: 3,
		LastError:         "some error",
	}
	monitor.mu.Unlock()

	monitor.ClearError("test-plugin")

	status := monitor.GetPluginStatus("test-plugin")
	assert.Equal(t, 0, status.ConsecutiveErrors)
	assert.Equal(t, "", status.LastError)
}

func TestMonitor_ForceHealthCheck(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	// 对不存在的插件执行检查
	status := monitor.ForceHealthCheck("nonexistent")
	assert.NotNil(t, status)
	assert.Equal(t, "nonexistent", status.PluginID)
	assert.Equal(t, StatusUnknown, status.Status)
}

func TestMonitor_SendAlert(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	// 启动 goroutine 接收告警
	received := make(chan Alert, 1)
	go func() {
		for alert := range monitor.GetAlerts() {
			received <- alert
			return
		}
	}()

	// 发送告警
	monitor.sendAlert(Alert{
		PluginID:  "test-plugin",
		Type:      AlertTypePluginCrashed,
		Severity:  SeverityError,
		Message:   "Test alert",
		Timestamp: time.Now(),
	})

	// 等待接收
	select {
	case alert := <-received:
		assert.Equal(t, "test-plugin", alert.PluginID)
		assert.Equal(t, AlertTypePluginCrashed, alert.Type)
	case <-time.After(time.Second):
		t.Fatal("Did not receive alert")
	}
}

func TestMonitor_StartStop(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	monitor.Start()
	time.Sleep(50 * time.Millisecond)
	monitor.Stop()
}

func TestStatusTypes(t *testing.T) {
	assert.Equal(t, StatusType("healthy"), StatusHealthy)
	assert.Equal(t, StatusType("degraded"), StatusDegraded)
	assert.Equal(t, StatusType("unhealthy"), StatusUnhealthy)
	assert.Equal(t, StatusType("unknown"), StatusUnknown)
}

func TestAlertTypes(t *testing.T) {
	assert.Equal(t, AlertType("health_changed"), AlertTypeHealthChanged)
	assert.Equal(t, AlertType("plugin_crashed"), AlertTypePluginCrashed)
	assert.Equal(t, AlertType("plugin_recovered"), AlertTypePluginRecovered)
	assert.Equal(t, AlertType("auto_restarted"), AlertTypeAutoRestarted)
	assert.Equal(t, AlertType("high_error_rate"), AlertTypeHighErrorRate)
}

func TestAlertSeverities(t *testing.T) {
	assert.Equal(t, AlertSeverity("info"), SeverityInfo)
	assert.Equal(t, AlertSeverity("warning"), SeverityWarning)
	assert.Equal(t, AlertSeverity("error"), SeverityError)
	assert.Equal(t, AlertSeverity("critical"), SeverityCritical)
}

// MockHealthChecker 模拟健康检查器
type MockHealthChecker struct {
	healthy bool
	err     error
}

func (m *MockHealthChecker) HealthCheck() error {
	if m.healthy {
		return nil
	}
	return m.err
}

// MockMetricsCollector 模拟指标收集器
type MockMetricsCollector struct {
	metrics map[string]interface{}
}

func (m *MockMetricsCollector) CollectMetrics() map[string]interface{} {
	return m.metrics
}

func TestHealthChecker(t *testing.T) {
	checker := &MockHealthChecker{healthy: true}
	assert.NoError(t, checker.HealthCheck())

	checker.healthy = false
	checker.err = assert.AnError
	assert.Error(t, checker.HealthCheck())
}

func TestMetricsCollector(t *testing.T) {
	collector := &MockMetricsCollector{
		metrics: map[string]interface{}{
			"requests": 100,
			"errors":   5,
		},
	}

	metrics := collector.CollectMetrics()
	assert.Equal(t, 100, metrics["requests"])
	assert.Equal(t, 5, metrics["errors"])
}

func TestMonitor_Concurrent(t *testing.T) {
	monitor := createTestMonitor(t)
	if monitor == nil {
		return
	}

	done := make(chan bool)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(id int) {
			monitor.mu.Lock()
			monitor.states["plugin-"+string(rune('0'+id))] = &HealthStatus{
				PluginID: "plugin-" + string(rune('0'+id)),
				Status:   StatusHealthy,
			}
			monitor.mu.Unlock()
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			_ = monitor.GetAllStatuses()
			_ = monitor.GetHealthyCount()
			done <- true
		}()
	}

	// 等待所有操作完成
	for i := 0; i < 15; i++ {
		<-done
	}
}
