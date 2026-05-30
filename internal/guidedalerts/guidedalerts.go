// Package guidedalerts 提供智能引导告警系统
// 学习 TrueNAS 26 Guided Alerts 特性：
// - 告警自动关联修复指引
// - 菜单指示器引导用户到问题区域
// - 告警优先级智能排序
// - 一键修复常见问题
package guidedalerts

import (
	"sync"
	"time"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
	SeveritySuccess  AlertSeverity = "success"
)

// AlertCategory 告警分类
type AlertCategory string

const (
	CategoryStorage   AlertCategory = "storage"
	CategoryNetwork   AlertCategory = "network"
	CategorySecurity  AlertCategory = "security"
	CategoryHardware  AlertCategory = "hardware"
	CategoryService   AlertCategory = "service"
	CategorySystem    AlertCategory = "system"
)

// Alert 告警信息
type Alert struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Severity    AlertSeverity `json:"severity"`
	Category    AlertCategory `json:"category"`
	Source      string        `json:"source"`      // 来源模块
	Guidance    *Guidance     `json:"guidance"`    // 修复指引
	MenuHint    *MenuHint     `json:"menuHint"`    // 菜单提示
	AutoFix     *AutoFix      `json:"autoFix"`     // 自动修复
	Acked       bool          `json:"acked"`
	Resolved    bool          `json:"resolved"`
	CreatedAt   time.Time     `json:"createdAt"`
	ResolvedAt  *time.Time    `json:"resolvedAt"`
}

// Guidance 修复指引
type Guidance struct {
	Steps       []string `json:"steps"`       // 修复步骤
	DocURL      string   `json:"docUrl"`      // 文档链接
	VideoURL    string   `json:"videoUrl"`    // 视频教程
	Difficulty  string   `json:"difficulty"`  // easy, medium, hard
	EstimatedMin int     `json:"estimatedMin"` // 预计耗时（分钟）
}

// MenuHint 菜单提示
type MenuHint struct {
	MenuItem   string `json:"menuItem"`   // 菜单项名称
	MenuPath   string `json:"menuPath"`   // 菜单路径
	Badge      bool   `json:"badge"`      // 是否显示徽章
	BadgeCount int    `json:"badgeCount"` // 徽章数字
}

// AutoFix 自动修复
type AutoFix struct {
	Available bool   `json:"available"` // 是否可自动修复
	Command   string `json:"command"`   // 修复命令
	NeedsRoot bool   `json:"needsRoot"` // 是否需要root权限
	RiskLevel string `json:"riskLevel"` // low, medium, high
}

// AlertManager 告警管理器
type AlertManager struct {
	mu       sync.RWMutex
	alerts   map[string]*Alert
	rules    []*AlertRule
	listeners []AlertListener
}

// AlertRule 告警规则
type AlertRule struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Category  AlertCategory `json:"category"`
	Condition string        `json:"condition"`
	Severity  AlertSeverity `json:"severity"`
	Enabled   bool          `json:"enabled"`
}

// AlertListener 告警监听器
type AlertListener interface {
	OnAlert(alert *Alert)
	OnResolve(alert *Alert)
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts: make(map[string]*Alert),
		rules:  make([]*AlertRule, 0),
	}
}

// CreateAlert 创建告警
func (am *AlertManager) CreateAlert(alert *Alert) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}
	am.alerts[alert.ID] = alert

	// 通知监听器
	for _, listener := range am.listeners {
		go listener.OnAlert(alert)
	}
}

// GetAlert 获取告警
func (am *AlertManager) GetAlert(id string) (*Alert, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	alert, ok := am.alerts[id]
	return alert, ok
}

// ListAlerts 列出告警
func (am *AlertManager) ListAlerts(severity AlertSeverity, category AlertCategory, unresolvedOnly bool) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*Alert
	for _, alert := range am.alerts {
		if severity != "" && alert.Severity != severity {
			continue
		}
		if category != "" && alert.Category != category {
			continue
		}
		if unresolvedOnly && alert.Resolved {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// AcknowledgeAlert 确认告警
func (am *AlertManager) AcknowledgeAlert(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, ok := am.alerts[id]
	if !ok {
		return ErrAlertNotFound
	}
	alert.Acked = true
	return nil
}

// ResolveAlert 解决告警
func (am *AlertManager) ResolveAlert(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, ok := am.alerts[id]
	if !ok {
		return ErrAlertNotFound
	}
	alert.Resolved = true
	now := time.Now()
	alert.ResolvedAt = &now

	for _, listener := range am.listeners {
		go listener.OnResolve(alert)
	}
	return nil
}

// GetMenuBadges 获取菜单徽章
func (am *AlertManager) GetMenuBadges() map[string]int {
	am.mu.RLock()
	defer am.mu.RUnlock()

	badges := make(map[string]int)
	for _, alert := range am.alerts {
		if alert.Resolved || alert.Acked {
			continue
		}
		if alert.MenuHint != nil && alert.MenuHint.Badge {
			badges[alert.MenuHint.MenuPath]++
		}
	}
	return badges
}

// AddListener 添加监听器
func (am *AlertManager) AddListener(listener AlertListener) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.listeners = append(am.listeners, listener)
}

// AddRule 添加规则
func (am *AlertManager) AddRule(rule *AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

// GetStats 获取告警统计
func (am *AlertManager) GetStats() map[string]int {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := map[string]int{
		"total":     0,
		"critical":  0,
		"warning":   0,
		"info":      0,
		"resolved":  0,
		"unresolved": 0,
	}

	for _, alert := range am.alerts {
		stats["total"]++
		stats[string(alert.Severity)]++
		if alert.Resolved {
			stats["resolved"]++
		} else {
			stats["unresolved"]++
		}
	}
	return stats
}

// 错误定义
var ErrAlertNotFound = &AlertError{"alert not found"}

type AlertError struct {
	msg string
}

func (e *AlertError) Error() string {
	return e.msg
}
