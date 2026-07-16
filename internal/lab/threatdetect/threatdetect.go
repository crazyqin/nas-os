// Package threatdetect 提供AI驱动的安全威胁检测和响应功能
// 对标群晖 ActiveProtect Manager 2.0 的威胁检测能力
package threatdetect

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ThreatLevel 威胁等级.
type ThreatLevel int

const (
	LevelLow      ThreatLevel = iota // 低风险
	LevelMedium                      // 中风险
	LevelHigh                        // 高风险
	LevelCritical                    // 严重风险
)

func (l ThreatLevel) String() string {
	switch l {
	case LevelLow:
		return "low"
	case LevelMedium:
		return "medium"
	case LevelHigh:
		return "high"
	case LevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// EventType 事件类型.
type EventType string

const (
	EventBruteForce        EventType = "brute_force"          // 暴力破解
	EventRansomware        EventType = "ransomware"           // 勒索软件
	EventDataExfil         EventType = "data_exfiltration"    // 数据外泄
	EventBulkDelete        EventType = "bulk_delete"          // 批量删除
	EventPrivilegeEscalate EventType = "privilege_escalation" // 权限提升
	EventAnomalousAccess   EventType = "anomalous_access"     // 异常访问
	EventMalwareDetected   EventType = "malware_detected"     // 恶意软件
	EventUnusualLogin      EventType = "unusual_login"        // 异常登录
)

// SecurityEvent 安全事件.
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Type        EventType              `json:"type"`
	Level       ThreatLevel            `json:"level"`
	Source      string                 `json:"source"`
	User        string                 `json:"user"`
	IP          string                 `json:"ip,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	ResolvedBy  string                 `json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	Quarantine  bool                   `json:"quarantine"`
}

// DetectionRule 检测规则.
type DetectionRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	EventType   EventType   `json:"event_type"`
	Level       ThreatLevel `json:"level"`
	Enabled     bool        `json:"enabled"`
	Threshold   int         `json:"threshold"`
	WindowSec   int         `json:"window_sec"`
	Action      string      `json:"action"` // log/alert/quarantine/block
	Description string      `json:"description"`
}

// QuarantineEntry 隔离条目.
type QuarantineEntry struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	Reason       string     `json:"reason"`
	EventID      string     `json:"event_id"`
	QuarantineAt time.Time  `json:"quarantine_at"`
	ReleasedAt   *time.Time `json:"released_at,omitempty"`
	ReleasedBy   string     `json:"released_by,omitempty"`
	Hash         string     `json:"hash,omitempty"`
	Size         int64      `json:"size"`
}

// ThreatMetrics 威胁指标.
type ThreatMetrics struct {
	TotalEvents      int64                 `json:"total_events"`
	EventsByLevel    map[ThreatLevel]int64 `json:"events_by_level"`
	EventsByType     map[EventType]int64   `json:"events_by_type"`
	ResolvedEvents   int64                 `json:"resolved_events"`
	QuarantinedItems int64                 `json:"quarantined_items"`
	ActiveRules      int                   `json:"active_rules"`
	FalsePositives   int64                 `json:"false_positives"`
}

// ThreatDetector 威胁检测器.
type ThreatDetector struct {
	mu         sync.RWMutex
	events     []*SecurityEvent
	rules      map[string]*DetectionRule
	quarantine map[string]*QuarantineEntry
	metrics    *ThreatMetrics
	accessLog  map[string][]time.Time // IP -> access times
	logger     *slog.Logger
}

// NewThreatDetector 创建威胁检测器.
func NewThreatDetector(logger *slog.Logger) *ThreatDetector {
	if logger == nil {
		logger = slog.Default()
	}

	td := &ThreatDetector{
		events:     make([]*SecurityEvent, 0),
		rules:      make(map[string]*DetectionRule),
		quarantine: make(map[string]*QuarantineEntry),
		metrics: &ThreatMetrics{
			EventsByLevel: make(map[ThreatLevel]int64),
			EventsByType:  make(map[EventType]int64),
		},
		accessLog: make(map[string][]time.Time),
		logger:    logger,
	}

	td.registerDefaultRules()
	return td
}

// registerDefaultRules 注册默认检测规则.
func (td *ThreatDetector) registerDefaultRules() {
	defaults := []*DetectionRule{
		{
			ID: "builtin-brute-force", Name: "暴力破解检测",
			EventType: EventBruteForce, Level: LevelHigh,
			Enabled: true, Threshold: 5, WindowSec: 300, Action: "alert",
			Description: "5分钟内同一IP登录失败超过5次",
		},
		{
			ID: "builtin-bulk-delete", Name: "批量删除检测",
			EventType: EventBulkDelete, Level: LevelHigh,
			Enabled: true, Threshold: 50, WindowSec: 60, Action: "quarantine",
			Description: "1分钟内删除超过50个文件",
		},
		{
			ID: "builtin-ransomware", Name: "勒索软件检测",
			EventType: EventRansomware, Level: LevelCritical,
			Enabled: true, Threshold: 10, WindowSec: 60, Action: "quarantine",
			Description: "1分钟内大量文件被加密重命名",
		},
		{
			ID: "builtin-unusual-login", Name: "异常登录检测",
			EventType: EventUnusualLogin, Level: LevelMedium,
			Enabled: true, Threshold: 1, WindowSec: 0, Action: "log",
			Description: "非工作时间或异地登录",
		},
		{
			ID: "builtin-data-exfil", Name: "数据外泄检测",
			EventType: EventDataExfil, Level: LevelCritical,
			Enabled: true, Threshold: 1073741824, WindowSec: 3600, Action: "alert",
			Description: "1小时内上传/传输超过1GB数据到外部",
		},
	}

	for _, r := range defaults {
		td.rules[r.ID] = r
	}
}

// AddRule 添加检测规则.
func (td *ThreatDetector) AddRule(rule *DetectionRule) error {
	if rule == nil || rule.ID == "" {
		return errors.New("invalid rule")
	}
	td.mu.Lock()
	defer td.mu.Unlock()

	td.rules[rule.ID] = rule
	td.logger.Info("检测规则已添加", "id", rule.ID, "name", rule.Name)
	return nil
}

// RemoveRule 移除检测规则.
func (td *ThreatDetector) RemoveRule(ruleID string) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	if _, ok := td.rules[ruleID]; !ok {
		return errors.New("rule not found")
	}
	delete(td.rules, ruleID)
	return nil
}

// ProcessEvent 处理安全事件.
func (td *ThreatDetector) ProcessEvent(event *SecurityEvent) error {
	if event == nil {
		return errors.New("event cannot be nil")
	}

	td.mu.Lock()
	defer td.mu.Unlock()

	event.Timestamp = time.Now()
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}

	td.events = append(td.events, event)
	td.metrics.TotalEvents++
	td.metrics.EventsByLevel[event.Level]++
	td.metrics.EventsByType[event.Type]++

	// 检查是否需要自动隔离
	for _, rule := range td.rules {
		if rule.EventType == event.Type && rule.Enabled && rule.Action == "quarantine" {
			if event.Path != "" {
				td.quarantineFile(event.Path, event.Description, event.ID)
				event.Quarantine = true
			}
		}
	}

	td.logger.Warn("安全事件",
		"id", event.ID, "type", event.Type,
		"level", event.Level, "source", event.Source, "user", event.User,
	)
	return nil
}

// DetectBruteForce 检测暴力破解.
func (td *ThreatDetector) DetectBruteForce(ip string, success bool) *SecurityEvent {
	if success {
		return nil
	}

	td.mu.Lock()
	defer td.mu.Unlock()

	key := "bf:" + ip
	now := time.Now()
	td.accessLog[key] = append(td.accessLog[key], now)

	// 清理过期记录
	window := 5 * time.Minute
	var recent []time.Time
	for _, t := range td.accessLog[key] {
		if now.Sub(t) <= window {
			recent = append(recent, t)
		}
	}
	td.accessLog[key] = recent

	if len(recent) >= 5 {
		return &SecurityEvent{
			Type:        EventBruteForce,
			Level:       LevelHigh,
			Source:      "threat_detector",
			IP:          ip,
			Description: fmt.Sprintf("IP %s 在5分钟内登录失败%d次", ip, len(recent)),
			Details:     map[string]interface{}{"attempts": len(recent)},
		}
	}
	return nil
}

// DetectRansomware 检测勒索软件行为.
func (td *ThreatDetector) DetectRansomware(user string, renamedFiles int) *SecurityEvent {
	if renamedFiles >= 10 {
		return &SecurityEvent{
			Type:        EventRansomware,
			Level:       LevelCritical,
			Source:      "threat_detector",
			User:        user,
			Description: fmt.Sprintf("用户 %s 在短时间内重命名%d个文件，疑似勒索软件", user, renamedFiles),
			Details:     map[string]interface{}{"renamed_files": renamedFiles},
			Quarantine:  true,
		}
	}
	return nil
}

// DetectBulkDelete 检测批量删除.
func (td *ThreatDetector) DetectBulkDelete(user string, deletedCount int) *SecurityEvent {
	if deletedCount >= 50 {
		return &SecurityEvent{
			Type:        EventBulkDelete,
			Level:       LevelHigh,
			Source:      "threat_detector",
			User:        user,
			Description: fmt.Sprintf("用户 %s 在1分钟内删除%d个文件", user, deletedCount),
			Details:     map[string]interface{}{"deleted_count": deletedCount},
		}
	}
	return nil
}

// ScanFile 扫描文件（启发式检测）.
func (td *ThreatDetector) ScanFile(path string, content []byte) *SecurityEvent {
	suspiciousExts := []string{".encrypted", ".locked", ".crypto", ".enc"}
	for _, ext := range suspiciousExts {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return &SecurityEvent{
				Type:        EventRansomware,
				Level:       LevelCritical,
				Source:      "file_scanner",
				Path:        path,
				Description: fmt.Sprintf("检测到可疑加密文件: %s", path),
				Quarantine:  true,
			}
		}
	}
	return nil
}

// quarantineFile 隔离文件.
func (td *ThreatDetector) quarantineFile(path string, reason string, eventID string) {
	entry := &QuarantineEntry{
		ID:           fmt.Sprintf("q-%d", time.Now().UnixNano()),
		Path:         path,
		Reason:       reason,
		EventID:      eventID,
		QuarantineAt: time.Now(),
	}
	td.quarantine[entry.ID] = entry
	td.metrics.QuarantinedItems++
	td.logger.Warn("文件已隔离", "path", path, "reason", reason)
}

// ReleaseQuarantine 释放隔离文件.
func (td *ThreatDetector) ReleaseQuarantine(entryID string, releasedBy string) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	entry, ok := td.quarantine[entryID]
	if !ok {
		return errors.New("quarantine entry not found")
	}
	if entry.ReleasedAt != nil {
		return errors.New("already released")
	}

	now := time.Now()
	entry.ReleasedAt = &now
	entry.ReleasedBy = releasedBy
	td.metrics.QuarantinedItems--
	return nil
}

// ResolveEvent 解决安全事件.
func (td *ThreatDetector) ResolveEvent(eventID string, resolvedBy string, falsePositive bool) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	for _, evt := range td.events {
		if evt.ID == eventID {
			evt.Resolved = true
			evt.ResolvedBy = resolvedBy
			now := time.Now()
			evt.ResolvedAt = &now
			td.metrics.ResolvedEvents++
			if falsePositive {
				td.metrics.FalsePositives++
			}
			return nil
		}
	}
	return errors.New("event not found")
}

// GetEvents 获取安全事件.
func (td *ThreatDetector) GetEvents(level ThreatLevel, resolved bool, limit int) []*SecurityEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()

	var result []*SecurityEvent
	for i := len(td.events) - 1; i >= 0; i-- {
		evt := td.events[i]
		if evt.Level < level || evt.Resolved != resolved {
			continue
		}
		result = append(result, evt)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetQuarantineList 获取隔离列表.
func (td *ThreatDetector) GetQuarantineList(includeReleased bool) []*QuarantineEntry {
	td.mu.RLock()
	defer td.mu.RUnlock()

	var result []*QuarantineEntry
	for _, entry := range td.quarantine {
		if !includeReleased && entry.ReleasedAt != nil {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// GetMetrics 获取威胁指标.
func (td *ThreatDetector) GetMetrics() *ThreatMetrics {
	td.mu.RLock()
	defer td.mu.RUnlock()

	m := &ThreatMetrics{
		TotalEvents:      td.metrics.TotalEvents,
		EventsByLevel:    make(map[ThreatLevel]int64),
		EventsByType:     make(map[EventType]int64),
		ResolvedEvents:   td.metrics.ResolvedEvents,
		QuarantinedItems: td.metrics.QuarantinedItems,
		FalsePositives:   td.metrics.FalsePositives,
	}
	for k, v := range td.metrics.EventsByLevel {
		m.EventsByLevel[k] = v
	}
	for k, v := range td.metrics.EventsByType {
		m.EventsByType[k] = v
	}
	for _, r := range td.rules {
		if r.Enabled {
			m.ActiveRules++
		}
	}
	return m
}
