// Package netdataex 提供 Netdata 高级系统监控功能
package netdataex

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// Manager 监控管理器
type Manager struct {
	mu          sync.RWMutex
	config      *NetdataConfig
	metrics     map[string]*MetricSeries
	alertRules  map[string]*AlertRule
	alertEvents map[string]*AlertEvent
	dashboards  map[string]*Dashboard
}

// NewManager 创建监控管理器
func NewManager(config *NetdataConfig) *Manager {
	if config == nil {
		config = &NetdataConfig{
			NetdataURL:       "http://localhost:19999",
			RetentionDays:    30,
			SamplingInterval: 10,
			ExportEnabled:    true,
		}
	}
	return &Manager{
		config:      config,
		metrics:     make(map[string]*MetricSeries),
		alertRules:  make(map[string]*AlertRule),
		alertEvents: make(map[string]*AlertEvent),
		dashboards:  make(map[string]*Dashboard),
	}
}

// GetMetrics 获取指标数据
func (m *Manager) GetMetrics(name string, from, to time.Time) (*MetricSeries, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	series, ok := m.metrics[name]
	if !ok {
		return nil, fmt.Errorf("metric %s not found", name)
	}

	filtered := &MetricSeries{
		Name:        series.Name,
		Unit:        series.Unit,
		Type:        series.Type,
		Aggregation: series.Aggregation,
	}

	for _, p := range series.Points {
		if (p.Timestamp.Equal(from) || p.Timestamp.After(from)) &&
			(p.Timestamp.Equal(to) || p.Timestamp.Before(to)) {
			filtered.Points = append(filtered.Points, p)
		}
	}

	return filtered, nil
}

// GetLatestMetric 获取最新指标
func (m *Manager) GetLatestMetric(name string) (*MetricPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	series, ok := m.metrics[name]
	if !ok {
		return nil, fmt.Errorf("metric %s not found", name)
	}

	if len(series.Points) == 0 {
		return nil, fmt.Errorf("no data points for metric %s", name)
	}

	return &series.Points[len(series.Points)-1], nil
}

// GetAllMetrics 获取所有指标
func (m *Manager) GetAllMetrics() ([]MetricSeries, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MetricSeries, 0, len(m.metrics))
	for _, series := range m.metrics {
		result = append(result, *series)
	}
	return result, nil
}

// CreateAlertRule 创建告警规则
func (m *Manager) CreateAlertRule(rule AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("alert rule ID is required")
	}

	m.alertRules[rule.ID] = &rule
	return nil
}

// GetAlertRules 获取所有告警规则
func (m *Manager) GetAlertRules() ([]AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AlertRule, 0, len(m.alertRules))
	for _, rule := range m.alertRules {
		result = append(result, *rule)
	}
	return result, nil
}

// GetAlertEvents 获取告警事件
func (m *Manager) GetAlertEvents(severity string, limit int) ([]AlertEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AlertEvent, 0)
	for _, event := range m.alertEvents {
		if severity == "" || string(event.Severity) == severity {
			result = append(result, *event)
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// AcknowledgeAlert 确认告警
func (m *Manager) AcknowledgeAlert(eventID, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	event, ok := m.alertEvents[eventID]
	if !ok {
		return fmt.Errorf("alert event %s not found", eventID)
	}

	event.Acknowledged = true
	event.AckedBy = user
	return nil
}

// CreateDashboard 创建仪表板
func (m *Manager) CreateDashboard(dashboard Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if dashboard.ID == "" {
		return fmt.Errorf("dashboard ID is required")
	}

	now := time.Now()
	dashboard.CreatedAt = now
	dashboard.UpdatedAt = now

	m.dashboards[dashboard.ID] = &dashboard
	return nil
}

// GetDashboard 获取仪表板
func (m *Manager) GetDashboard(dashboardID string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard %s not found", dashboardID)
	}

	return dashboard, nil
}

// ListDashboards 获取所有仪表板
func (m *Manager) ListDashboards() ([]Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Dashboard, 0, len(m.dashboards))
	for _, d := range m.dashboards {
		result = append(result, *d)
	}
	return result, nil
}

// UpdateDashboard 更新仪表板
func (m *Manager) UpdateDashboard(dashboard Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.dashboards[dashboard.ID]
	if !ok {
		return fmt.Errorf("dashboard %s not found", dashboard.ID)
	}

	dashboard.CreatedAt = existing.CreatedAt
	dashboard.UpdatedAt = time.Now()

	m.dashboards[dashboard.ID] = &dashboard
	return nil
}

// GetHealthReport 获取健康报告
func (m *Manager) GetHealthReport() (*HealthReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &HealthReport{
		Score: 95,
		CPU: ComponentHealth{
			Status: "healthy",
			Value:  "25%",
		},
		Memory: ComponentHealth{
			Status: "healthy",
			Value:  "60%",
		},
		Disk: ComponentHealth{
			Status: "healthy",
			Value:  "45%",
		},
		Network: ComponentHealth{
			Status: "healthy",
			Value:  "100Mbps",
		},
		Temperature: ComponentHealth{
			Status: "healthy",
			Value:  "45°C",
		},
		Power: ComponentHealth{
			Status: "healthy",
			Value:  "120W",
		},
		Uptime:         86400,
		Recommendations: []string{},
	}

	return report, nil
}

// ExportMetrics 导出指标
func (m *Manager) ExportMetrics(format string, from, to time.Time) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch format {
	case "json":
		result := make(map[string]interface{})
		for name, series := range m.metrics {
			filtered := &MetricSeries{
				Name: series.Name,
				Unit: series.Unit,
				Type: series.Type,
			}
			for _, p := range series.Points {
				if (p.Timestamp.Equal(from) || p.Timestamp.After(from)) &&
					(p.Timestamp.Equal(to) || p.Timestamp.Before(to)) {
					filtered.Points = append(filtered.Points, p)
				}
			}
			result[name] = filtered
		}
		return json.MarshalIndent(result, "", "  ")

	case "prometheus":
		var output string
		for name, series := range m.metrics {
			if len(series.Points) == 0 {
				continue
			}
			latest := series.Points[len(series.Points)-1]
			output += fmt.Sprintf("# HELP %s %s\n", name, series.Unit)
			output += fmt.Sprintf("# TYPE %s %s\n", name, series.Type)
			output += fmt.Sprintf("%s %g\n", name, latest.Value)
		}
		return []byte(output), nil

	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// 添加指标数据点
func (m *Manager) AddMetricPoint(name string, point MetricPoint, series *MetricSeries) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.metrics[name]
	if !ok {
		m.metrics[name] = series
		m.metrics[name].Points = []MetricPoint{point}
		return
	}

	existing.Points = append(existing.Points, point)
}

// 添加告警事件
func (m *Manager) AddAlertEvent(event AlertEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertEvents[event.ID] = &event
}

// checkAlerts 检查告警规则
func (m *Manager) checkAlerts(name string, value float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.alertRules {
		if rule.Metric != name || !rule.Enabled {
			continue
		}

		triggered := false
		switch rule.Condition {
		case ConditionGT:
			triggered = value > rule.Threshold
		case ConditionLT:
			triggered = value < rule.Threshold
		case ConditionEQ:
			triggered = math.Abs(value-rule.Threshold) < 0.0001
		}

		if triggered {
			event := AlertEvent{
				ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
				RuleID:    rule.ID,
				Metric:    name,
				Value:     value,
				Threshold: rule.Threshold,
				Severity:  rule.Severity,
				Message:   fmt.Sprintf("Alert: %s %s %f (current: %f)", name, rule.Condition, rule.Threshold, value),
				CreatedAt: time.Now(),
			}
			m.alertEvents[event.ID] = &event

			now := time.Now()
			rule.LastFired = &now
		}
	}
}
