package diskhealth

import (
	"fmt"
	"sync"
	"time"
)

// AlertManager 告警管理器
type AlertManager struct {
	alerts      map[string]*Alert
	rules       []*AlertRule
	subscribers []AlertSubscriber
	mu          sync.RWMutex
}

// AlertRule 告警规则
type AlertRule struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Device   string     `json:"device"`   // 空表示所有设备
	Level    AlertLevel `json:"level"`
	Condition string    `json:"condition"` // score_below/temp_above/etc
	Threshold float64   `json:"threshold"`
	Enabled  bool       `json:"enabled"`
}

// AlertSubscriber 告警订阅者
type AlertSubscriber interface {
	OnAlert(alert *Alert)
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts: make(map[string]*Alert),
		rules:  getDefaultRules(),
	}
}

// getDefaultRules 获取默认规则
func getDefaultRules() []*AlertRule {
	return []*AlertRule{
		{
			ID:        "health-critical",
			Name:      "健康评分严重",
			Level:     AlertLevelCritical,
			Condition: "score_below",
			Threshold: 30,
			Enabled:   true,
		},
		{
			ID:        "health-warning",
			Name:      "健康评分警告",
			Level:     AlertLevelWarning,
			Condition: "score_below",
			Threshold: 60,
			Enabled:   true,
		},
		{
			ID:        "temp-critical",
			Name:      "温度过高",
			Level:     AlertLevelCritical,
			Condition: "temp_above",
			Threshold: 65,
			Enabled:   true,
		},
		{
			ID:        "temp-warning",
			Name:      "温度偏高",
			Level:     AlertLevelWarning,
			Condition: "temp_above",
			Threshold: 55,
			Enabled:   true,
		},
		{
			ID:        "smart-failed",
			Name:      "S.M.A.R.T. 失败",
			Level:     AlertLevelEmergency,
			Condition: "smart_failed",
			Threshold: 0,
			Enabled:   true,
		},
		{
			ID:        "reallocated-sectors",
			Name:      "重分配扇区",
			Level:     AlertLevelWarning,
			Condition: "reallocated_above",
			Threshold: 0,
			Enabled:   true,
		},
	}
}

// CheckAndAlert 检查并生成告警
func (m *AlertManager) CheckAndAlert(info *DiskInfo, assessment *HealthAssessment) []*Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newAlerts []*Alert

	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Device != "" && rule.Device != info.Device {
			continue
		}

		var triggered bool
		var title, message string

		switch rule.Condition {
		case "score_below":
			if float64(assessment.Score) < rule.Threshold {
				triggered = true
				title = fmt.Sprintf("磁盘 %s 健康评分过低", info.Device)
				message = fmt.Sprintf("健康评分: %d (阈值: %.0f)", assessment.Score, rule.Threshold)
			}
		case "temp_above":
			if float64(info.Temperature) > rule.Threshold {
				triggered = true
				title = fmt.Sprintf("磁盘 %s 温度过高", info.Device)
				message = fmt.Sprintf("当前温度: %d°C (阈值: %.0f°C)", info.Temperature, rule.Threshold)
			}
		case "smart_failed":
			for _, attr := range info.SMARTAttrs {
				if attr.Failed {
					triggered = true
					title = fmt.Sprintf("磁盘 %s S.M.A.R.T. 检测失败", info.Device)
					message = fmt.Sprintf("属性 %s (ID:%d) 已失败", attr.Name, attr.ID)
					break
				}
			}
		case "reallocated_above":
			for _, attr := range info.SMARTAttrs {
				if attr.ID == 5 && attr.RawValue > int64(rule.Threshold) {
					triggered = true
					title = fmt.Sprintf("磁盘 %s 存在重分配扇区", info.Device)
					message = fmt.Sprintf("重分配扇区数: %d", attr.RawValue)
					break
				}
			}
		}

		if triggered {
			alertID := fmt.Sprintf("%s-%s-%d", rule.ID, info.Device, time.Now().Unix())
			alert := &Alert{
				ID:        alertID,
				Device:    info.Device,
				Level:     rule.Level,
				Title:     title,
				Message:   message,
				CreatedAt: time.Now(),
			}
			m.alerts[alertID] = alert
			newAlerts = append(newAlerts, alert)

			// 通知订阅者
			for _, sub := range m.subscribers {
				go sub.OnAlert(alert)
			}
		}
	}

	return newAlerts
}

// GetAlerts 获取告警
func (m *AlertManager) GetAlerts(device string, includeAcked bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Alert
	for _, a := range m.alerts {
		if device != "" && a.Device != device {
			continue
		}
		if !includeAcked && a.AckedAt != nil {
			continue
		}
		result = append(result, a)
	}
	return result
}

// AckAlert 确认告警
func (m *AlertManager) AckAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %s not found", alertID)
	}

	now := time.Now()
	alert.AckedAt = &now
	return nil
}

// ResolveAlert 解决告警
func (m *AlertManager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %s not found", alertID)
	}

	now := time.Now()
	alert.ResolvedAt = &now
	return nil
}

// Subscribe 订阅告警
func (m *AlertManager) Subscribe(subscriber AlertSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers = append(m.subscribers, subscriber)
}

// AddRule 添加规则
func (m *AlertManager) AddRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// GetRules 获取规则
func (m *AlertManager) GetRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rules
}

// CleanOldAlerts 清理旧告警
func (m *AlertManager) CleanOldAlerts(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0
	for id, alert := range m.alerts {
		if alert.CreatedAt.Before(cutoff) && alert.AckedAt != nil {
			delete(m.alerts, id)
			count++
		}
	}
	return count
}
