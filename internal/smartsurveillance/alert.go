// Package smartsurveillance 提供智能监控中心功能
// alert.go - 智能告警，支持实时推送、告警规则、告警升级
package smartsurveillance

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AlertManager 告警管理器
type AlertManager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	engine        *SurveillanceEngine
	alerts        map[string]*Alert
	rules         []AlertRule
	notifyEnabled bool
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	CameraID    string          `json:"camera_id,omitempty"` // 空表示所有摄像头
	ZoneID      string          `json:"zone_id,omitempty"`   // 空表示所有区域
	DetectTypes []DetectionType `json:"detect_types,omitempty"`
	MinConf     float64         `json:"min_confidence"` // 最小置信度
	Level       AlertLevel      `json:"level"`
	NotifyEmail bool            `json:"notify_email"`
	NotifyPush  bool            `json:"notify_push"`
	CooldownMin int             `json:"cooldown_minutes"` // 冷却时间
	LastTrigger time.Time       `json:"last_trigger"`
}

// NewAlertManager 创建告警管理器
func NewAlertManager(logger *zap.Logger, engine *SurveillanceEngine) *AlertManager {
	am := &AlertManager{
		logger:        logger,
		engine:        engine,
		alerts:        make(map[string]*Alert),
		notifyEnabled: true,
	}

	// 初始化默认规则
	am.initDefaultRules()
	return am
}

// initDefaultRules 初始化默认告警规则
func (am *AlertManager) initDefaultRules() {
	am.rules = []AlertRule{
		{
			ID:          "rule-stranger",
			Name:        "陌生人告警",
			Enabled:     true,
			DetectTypes: []DetectionType{DetectionTypeFace},
			MinConf:     0.7,
			Level:       AlertLevelWarning,
			NotifyPush:  true,
			CooldownMin: 5,
		},
		{
			ID:          "rule-intrusion",
			Name:        "入侵告警",
			Enabled:     true,
			DetectTypes: []DetectionType{DetectionTypeIntrusion},
			MinConf:     0.8,
			Level:       AlertLevelCritical,
			NotifyEmail: true,
			NotifyPush:  true,
			CooldownMin: 1,
		},
		{
			ID:          "rule-vehicle",
			Name:        "车辆识别",
			Enabled:     true,
			DetectTypes: []DetectionType{DetectionTypePlate},
			MinConf:     0.85,
			Level:       AlertLevelInfo,
			NotifyPush:  true,
			CooldownMin: 10,
		},
	}
}

// CreateAlert 创建告警
func (am *AlertManager) CreateAlert(alert *Alert) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert.ID == "" {
		alert.ID = uuid.New().String()
	}

	alert.Status = AlertStatusPending
	alert.Timestamp = time.Now()

	am.alerts[alert.ID] = alert
	am.logger.Info("告警已创建",
		zap.String("id", alert.ID),
		zap.String("level", string(alert.Level)),
		zap.String("title", alert.Title))
	return nil
}

// ProcessEvent 处理事件并生成告警
func (am *AlertManager) ProcessEvent(event *Event) *Alert {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 匹配规则
	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		// 检查摄像头
		if rule.CameraID != "" && rule.CameraID != event.CameraID {
			continue
		}

		// 检查区域
		if rule.ZoneID != "" && rule.ZoneID != event.ZoneID {
			continue
		}

		// 检查检测类型
		if !am.matchesType(rule, event.Type) {
			continue
		}

		// 检查置信度
		if event.Confidence < rule.MinConf {
			continue
		}

		// 检查冷却时间
		if !rule.LastTrigger.IsZero() {
			cooldown := time.Duration(rule.CooldownMin) * time.Minute
			if time.Since(rule.LastTrigger) < cooldown {
				continue
			}
		}

		// 生成告警
		camera, _ := am.engine.GetCamera(event.CameraID)
		cameraName := event.CameraID
		if camera != nil {
			cameraName = camera.Name
		}

		alert := &Alert{
			ID:          uuid.New().String(),
			CameraID:    event.CameraID,
			CameraName:  cameraName,
			EventID:     event.ID,
			Level:       rule.Level,
			Status:      AlertStatusActive,
			Title:       rule.Name,
			Description: event.Description,
			Timestamp:   time.Now(),
			NotifySent:  false,
		}

		am.alerts[alert.ID] = alert
		rule.LastTrigger = time.Now()

		am.logger.Warn("告警触发",
			zap.String("alert_id", alert.ID),
			zap.String("rule", rule.Name),
			zap.String("camera", cameraName))

		return alert
	}

	return nil
}

// matchesType 匹配检测类型
func (am *AlertManager) matchesType(rule AlertRule, detectType DetectionType) bool {
	if len(rule.DetectTypes) == 0 {
		return true
	}
	for _, t := range rule.DetectTypes {
		if t == detectType {
			return true
		}
	}
	return false
}

// AckAlert 确认告警
func (am *AlertManager) AckAlert(alertID string, ackedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	now := time.Now()
	alert.Status = AlertStatusAcked
	alert.AckedAt = &now
	alert.AckedBy = ackedBy

	am.logger.Info("告警已确认",
		zap.String("alert_id", alertID),
		zap.String("acked_by", ackedBy))
	return nil
}

// ResolveAlert 解决告警
func (am *AlertManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	now := time.Now()
	alert.Status = AlertStatusResolved
	alert.ResolvedAt = &now

	am.logger.Info("告警已解决", zap.String("alert_id", alertID))
	return nil
}

// DismissAlert 忽略告警
func (am *AlertManager) DismissAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	alert.Status = AlertStatusDismissed
	am.logger.Info("告警已忽略", zap.String("alert_id", alertID))
	return nil
}

// GetAlert 获取告警
func (am *AlertManager) GetAlert(alertID string) (*Alert, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return nil, ErrAlertNotFound
	}
	return alert, nil
}

// QueryAlerts 查询告警
func (am *AlertManager) QueryAlerts(query AlertQuery) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*Alert
	for _, alert := range am.alerts {
		if !matchAlert(alert, query) {
			continue
		}
		result = append(result, alert)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 分页
	start := (query.Page - 1) * query.PageSize
	if start >= len(result) {
		return nil
	}
	end := start + query.PageSize
	if end > len(result) {
		end = len(result)
	}
	return result[start:end]
}

// matchAlert 匹配告警
func matchAlert(alert *Alert, query AlertQuery) bool {
	if query.CameraID != "" && alert.CameraID != query.CameraID {
		return false
	}
	if query.StartTime != nil && alert.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && alert.Timestamp.After(*query.EndTime) {
		return false
	}
	if len(query.Levels) > 0 {
		found := false
		for _, l := range query.Levels {
			if alert.Level == l {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(query.Statuses) > 0 {
		found := false
		for _, s := range query.Statuses {
			if alert.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// GetActiveAlerts 获取活跃告警
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var active []*Alert
	for _, alert := range am.alerts {
		if alert.Status == AlertStatusActive || alert.Status == AlertStatusPending {
			active = append(active, alert)
		}
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].Timestamp.After(active[j].Timestamp)
	})

	return active
}

// GetAlertStats 获取告警统计
func (am *AlertManager) GetAlertStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := map[string]interface{}{
		"total":     len(am.alerts),
		"pending":   0,
		"active":    0,
		"acked":     0,
		"resolved":  0,
		"dismissed": 0,
		"by_level":  make(map[AlertLevel]int),
	}

	for _, alert := range am.alerts {
		switch alert.Status {
		case AlertStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case AlertStatusActive:
			stats["active"] = stats["active"].(int) + 1
		case AlertStatusAcked:
			stats["acked"] = stats["acked"].(int) + 1
		case AlertStatusResolved:
			stats["resolved"] = stats["resolved"].(int) + 1
		case AlertStatusDismissed:
			stats["dismissed"] = stats["dismissed"].(int) + 1
		}

		byLevel := stats["by_level"].(map[AlertLevel]int)
		byLevel[alert.Level]++
	}

	return stats
}

// AddRule 添加告警规则
func (am *AlertManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if rule.ID == "" {
		rule.ID = "rule-" + uuid.New().String()[:8]
	}
	am.rules = append(am.rules, rule)
}

// GetRules 获取所有规则
func (am *AlertManager) GetRules() []AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.rules
}
