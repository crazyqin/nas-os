// Package guidedalert 提供引导式告警系统
// 对标 TrueNAS 26 Guided Alerts
// 每条告警附带排查步骤、修复引导、根因分析，菜单指示器引导用户
package guidedalert

import (
	"fmt"
	"sync"
	"time"
)

// AlertSeverity 告警等级
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
	SeverityFatal    AlertSeverity = "fatal"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	StatusOpen       AlertStatus = "open"
	StatusAcknowledged AlertStatus = "acknowledged"
	StatusInProgress AlertStatus = "in_progress"
	StatusResolved   AlertStatus = "resolved"
	StatusSuppressed AlertStatus = "suppressed"
)

// GuidedStep 引导步骤
type GuidedStep struct {
	StepNumber    int    `json:"stepNumber"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Action        string `json:"action"`        // CLI命令或API调用
	ExpectedResult string `json:"expectedResult"` // 期望结果
	IsOptional    bool   `json:"isOptional"`
	AutoFix       bool   `json:"autoFix"`       // 是否支持自动修复
}

// RootCause 根因分析
type RootCause struct {
	Pattern     string  `json:"pattern"`     // 匹配模式
	Probability float64 `json:"probability"` // 可能性百分比
	Category    string  `json:"category"`    // hardware|software|config|network
	Description string  `json:"description"`
	Fix         string  `json:"fix"`
}

// GuidedAlert 引导式告警
type GuidedAlert struct {
	ID            string        `json:"id"`
	Code          string        `json:"code"`          // 告警代码如 SMART_WARN
	Title         string        `json:"title"`
	Message       string        `json:"message"`
	Severity      AlertSeverity `json:"severity"`
	Status        AlertStatus   `json:"status"`
	Source        string        `json:"source"`        // 来源模块
	Component     string        `json:"component"`     // 组件 disk|pool|network|service
	ResourceID    string        `json:"resourceId"`    // 关联资源ID
	MenuPath      []string      `json:"menuPath"`      // 菜单路径，用于UI导航 ["存储", "磁盘管理", "sda"]
	GuidedSteps   []GuidedStep  `json:"guidedSteps"`   // 排查引导步骤
	RootCauses    []RootCause   `json:"rootCauses"`    // 根因分析
	DocsURL       string        `json:"docsUrl"`       // 相关文档URL
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	ResolvedAt    *time.Time    `json:"resolvedAt,omitempty"`
	AckedBy       string        `json:"ackedBy,omitempty"`
	AutoFixable   bool          `json:"autoFixable"`   // 是否支持自动修复
}

// AlertRule 告警规则
type AlertRule struct {
	Code        string        `json:"code"`
	Title       string        `json:"title"`
	Severity    AlertSeverity `json:"severity"`
	Component   string        `json:"component"`
	GuidedSteps []GuidedStep  `json:"guidedSteps"`
	RootCauses  []RootCause   `json:"rootCauses"`
	DocsURL     string        `json:"docsUrl"`
	AutoFixable bool          `json:"autoFixable"`
	MenuPathFn  func(resourceID string) []string `json:"-"` // 动态菜单路径
}

// GuidedAlertManager 引导告警管理器
type GuidedAlertManager struct {
	alerts  map[string]*GuidedAlert
	rules   map[string]*AlertRule
	mu      sync.RWMutex
	counter int64
}

// NewManager 创建管理器
func NewManager() *GuidedAlertManager {
	m := &GuidedAlertManager{
		alerts: make(map[string]*GuidedAlert),
		rules:  make(map[string]*AlertRule),
	}
	m.registerBuiltinRules()
	return m
}

// RegisterRule 注册告警规则
func (m *GuidedAlertManager) RegisterRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.Code] = rule
}

// Fire 触发告警
func (m *GuidedAlertManager) Fire(code, message, resourceID string) *GuidedAlert {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule, ok := m.rules[code]
	if !ok {
		// 未知告警代码，创建基础告警
		m.counter++
		alert := &GuidedAlert{
			ID:        fmt.Sprintf("GA-%d", m.counter),
			Code:      code,
			Title:     code,
			Message:   message,
			Severity:  SeverityWarning,
			Status:    StatusOpen,
			Component: "unknown",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.alerts[alert.ID] = alert
		return alert
	}
	// 检查是否已存在相同资源的同类型告警
	for _, existing := range m.alerts {
		if existing.Code == code && existing.ResourceID == resourceID && existing.Status != StatusResolved {
			existing.Message = message
			existing.UpdatedAt = time.Now()
			return existing
		}
	}
	m.counter++
	var menuPath []string
	if rule.MenuPathFn != nil {
		menuPath = rule.MenuPathFn(resourceID)
	}
	alert := &GuidedAlert{
		ID:            fmt.Sprintf("GA-%d", m.counter),
		Code:          code,
		Title:         rule.Title,
		Message:       message,
		Severity:      rule.Severity,
		Status:        StatusOpen,
		Component:     rule.Component,
		ResourceID:    resourceID,
		MenuPath:      menuPath,
		GuidedSteps:   rule.GuidedSteps,
		RootCauses:    rule.RootCauses,
		DocsURL:       rule.DocsURL,
		AutoFixable:   rule.AutoFixable,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.alerts[alert.ID] = alert
	return alert
}

// Acknowledge 确认告警
func (m *GuidedAlertManager) Acknowledge(id, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	alert.Status = StatusAcknowledged
	alert.AckedBy = user
	alert.UpdatedAt = time.Now()
	return nil
}

// Resolve 解决告警
func (m *GuidedAlertManager) Resolve(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	now := time.Now()
	alert.Status = StatusResolved
	alert.ResolvedAt = &now
	alert.UpdatedAt = now
	return nil
}

// List 获取告警列表
func (m *GuidedAlertManager) List(status AlertStatus, severity AlertSeverity) []*GuidedAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*GuidedAlert
	for _, alert := range m.alerts {
		if status != "" && alert.Status != status {
			continue
		}
		if severity != "" && alert.Severity != severity {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// Get 获取单个告警
func (m *GuidedAlertManager) Get(id string) (*GuidedAlert, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alert, ok := m.alerts[id]
	return alert, ok
}

// MenuIndicator 菜单指示器
type MenuIndicator struct {
	Path       []string `json:"path"`       // 菜单路径
	AlertCount int      `json:"alertCount"` // 告警数量
	MaxSeverity AlertSeverity `json:"maxSeverity"` // 最严重等级
}

// GetMenuIndicators 获取菜单指示器（UI用）
func (m *GuidedAlertManager) GetMenuIndicators() []MenuIndicator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	indicators := make(map[string]*MenuIndicator)
	for _, alert := range m.alerts {
		if alert.Status == StatusResolved || alert.Status == StatusSuppressed {
			continue
		}
		if len(alert.MenuPath) == 0 {
			continue
		}
		key := ""
		for _, p := range alert.MenuPath {
			key += p + "/"
		}
		ind, ok := indicators[key]
		if !ok {
			ind = &MenuIndicator{Path: alert.MenuPath}
			indicators[key] = ind
		}
		ind.AlertCount++
		if severityOrder(alert.Severity) > severityOrder(ind.MaxSeverity) {
			ind.MaxSeverity = alert.Severity
		}
	}
	result := make([]MenuIndicator, 0, len(indicators))
	for _, ind := range indicators {
		result = append(result, *ind)
	}
	return result
}

func severityOrder(s AlertSeverity) int {
	switch s {
	case SeverityInfo:
		return 0
	case SeverityWarning:
		return 1
	case SeverityCritical:
		return 2
	case SeverityFatal:
		return 3
	default:
		return -1
	}
}

// registerBuiltinRules 注册内置告警规则
func (m *GuidedAlertManager) registerBuiltinRules() {
	m.rules["SMART_WARNING"] = &AlertRule{
		Code:      "SMART_WARNING",
		Title:     "磁盘SMART健康告警",
		Severity:  SeverityWarning,
		Component: "disk",
		GuidedSteps: []GuidedStep{
			{StepNumber: 1, Title: "查看磁盘详情", Description: "检查SMART详细指标", Action: "smartctl -a /dev/{disk}", ExpectedResult: "查看Reallocated_Sector_Ct等关键指标"},
			{StepNumber: 2, Title: "运行自检", Description: "执行磁盘短自检", Action: "smartctl -t short /dev/{disk}", ExpectedResult: "自检应在2分钟内完成"},
			{StepNumber: 3, Title: "检查日志", Description: "查看系统日志中的磁盘错误", Action: "dmesg | grep {disk}", ExpectedResult: "无I/O error或坏扇区记录"},
			{StepNumber: 4, Title: "备份数据", Description: "立即备份该磁盘上的重要数据", IsOptional: false},
			{StepNumber: 5, Title: "更换磁盘", Description: "如坏扇区持续增长，准备更换磁盘", IsOptional: true},
		},
		RootCauses: []RootCause{
			{Pattern: "Reallocated_Sector_Ct", Probability: 80, Category: "hardware", Description: "磁盘坏扇区重分配，磁盘即将故障", Fix: "更换磁盘"},
			{Pattern: "Current_Pending_Sector", Probability: 60, Category: "hardware", Description: "待处理坏扇区", Fix: "运行长自检尝试修复，持续增长则更换"},
			{Pattern: "UDMA_CRC_Error_Count", Probability: 30, Category: "hardware", Description: "SATA线缆或接口问题", Fix: "更换SATA线缆"},
		},
		AutoFixable: false,
		MenuPathFn: func(resourceID string) []string {
			return []string{"存储", "磁盘管理", resourceID}
		},
	}

	m.rules["POOL_DEGRADED"] = &AlertRule{
		Code:      "POOL_DEGRADED",
		Title:     "存储池降级",
		Severity:  SeverityCritical,
		Component: "pool",
		GuidedSteps: []GuidedStep{
			{StepNumber: 1, Title: "检查池状态", Description: "查看存储池详细状态", Action: "zpool status {pool}", ExpectedResult: "查看DEGRADED设备"},
			{StepNumber: 2, Title: "替换故障设备", Description: "如有备用盘，执行替换", Action: "zpool replace {pool} {old} {new}", ExpectedResult: "池开始 resilver"},
			{StepNumber: 3, Title: "监控Resilver", Description: "等待数据重建完成", Action: "zpool status {pool}", ExpectedResult: "resilver完成，状态变为ONLINE"},
		},
		AutoFixable: false,
		MenuPathFn: func(resourceID string) []string {
			return []string{"存储", "存储池管理", resourceID}
		},
	}

	m.rules["DISK_SPACE_LOW"] = &AlertRule{
		Code:      "DISK_SPACE_LOW",
		Title:     "磁盘空间不足",
		Severity:  SeverityWarning,
		Component: "storage",
		GuidedSteps: []GuidedStep{
			{StepNumber: 1, Title: "查看使用情况", Description: "检查各目录占用", Action: "df -h && du -sh /*", ExpectedResult: "定位大文件和目录"},
			{StepNumber: 2, Title: "清理临时文件", Description: "删除不必要的临时文件", Action: "rm -rf /tmp/* /var/tmp/*", AutoFix: true},
			{StepNumber: 3, Title: "清理Docker", Description: "清理未使用的Docker资源", Action: "docker system prune -af", AutoFix: true},
			{StepNumber: 4, Title: "扩容存储", Description: "如需更多空间，考虑扩容", IsOptional: true},
		},
		AutoFixable: true,
		MenuPathFn: func(resourceID string) []string {
			return []string{"存储", "存储池管理"}
		},
	}
}
