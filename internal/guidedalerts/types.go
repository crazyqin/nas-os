// Package guidedalerts 提供智能引导告警系统
// 支持告警规则定义、严重级别、引导式修复、告警路由、历史记录与统计、静默/抑制规则、告警升级机制。
package guidedalerts

import "time"

// AlertSeverity 告警严重级别.
type AlertSeverity int

const (
	SeverityInfo     AlertSeverity = iota // 信息
	SeverityWarning                       // 警告
	SeverityCritical                      // 严重
)

func (s AlertSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ParseSeverity 解析严重级别字符串.
func ParseSeverity(s string) AlertSeverity {
	switch s {
	case "info":
		return SeverityInfo
	case "warning":
		return SeverityWarning
	case "critical":
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

// AlertCategory 告警分类.
type AlertCategory string

const (
	CategoryCPU      AlertCategory = "cpu"
	CategoryMemory   AlertCategory = "memory"
	CategoryDisk     AlertCategory = "disk"
	CategoryNetwork  AlertCategory = "network"
	CategoryService  AlertCategory = "service"
	CategoryStorage  AlertCategory = "storage"
	CategoryHardware AlertCategory = "hardware"
	CategorySystem   AlertCategory = "system"
)

// AlertStatus 告警状态.
type AlertStatus string

const (
	StatusActive       AlertStatus = "active"
	StatusAcknowledged AlertStatus = "acknowledged"
	StatusSilenced     AlertStatus = "silenced"
	StatusResolved     AlertStatus = "resolved"
	StatusEscalated    AlertStatus = "escalated"
)

// Alert 告警实例.
type Alert struct {
	ID           string            `json:"id"`
	RuleID       string            `json:"ruleId"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Severity     AlertSeverity     `json:"severity"`
	Category     AlertCategory     `json:"category"`
	Source       string            `json:"source"`
	Status       AlertStatus       `json:"status"`
	Labels       map[string]string `json:"labels,omitempty"`
	Guidance     *Guidance         `json:"guidance,omitempty"`
	MenuHint     *MenuHint         `json:"menuHint,omitempty"`
	AutoFix      *AutoFix          `json:"autoFix,omitempty"`
	Escalation   *Escalation       `json:"escalation,omitempty"`
	Count        int               `json:"count"` // 聚合计数
	FirstSeen    time.Time         `json:"firstSeen"`
	LastSeen     time.Time         `json:"lastSeen"`
	LastNotified *time.Time        `json:"lastNotified,omitempty"`
	AckedAt      *time.Time        `json:"ackedAt,omitempty"`
	ResolvedAt   *time.Time        `json:"resolvedAt,omitempty"`
}

// Guidance 引导式修复指引.
type Guidance struct {
	Steps        []RepairStep `json:"steps"`
	DocURL       string       `json:"docUrl,omitempty"`
	VideoURL     string       `json:"videoUrl,omitempty"`
	Difficulty   string       `json:"difficulty"` // easy, medium, hard
	EstimatedMin int          `json:"estimatedMin"`
}

// RepairStep 修复步骤.
type RepairStep struct {
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	NeedsRoot   bool   `json:"needsRoot,omitempty"`
}

// MenuHint 菜单提示.
type MenuHint struct {
	MenuItem   string `json:"menuItem"`
	MenuPath   string `json:"menuPath"`
	Badge      bool   `json:"badge"`
	BadgeCount int    `json:"badgeCount"`
}

// AutoFix 自动修复.
type AutoFix struct {
	Available bool     `json:"available"`
	Commands  []string `json:"commands,omitempty"`
	NeedsRoot bool     `json:"needsRoot"`
	RiskLevel string   `json:"riskLevel"` // low, medium, high
	AutoApply bool     `json:"autoApply"` // 是否自动应用
}

// Escalation 升级策略.
type Escalation struct {
	Enabled        bool          `json:"enabled"`
	Timeout        time.Duration `json:"timeout"`        // 未处理多久后升级
	MaxLevel       int           `json:"maxLevel"`       // 最大升级级别
	CurrentLevel   int           `json:"currentLevel"`   // 当前升级级别
	NextEscalation *time.Time    `json:"nextEscalation"` // 下次升级时间
	EscalatedTo    []string      `json:"escalatedTo"`    // 已通知的升级目标
}

// SilenceRule 静默规则.
type SilenceRule struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Matchers  []LabelMatcher `json:"matchers"`
	StartsAt  time.Time      `json:"startsAt"`
	EndsAt    time.Time      `json:"endsAt"`
	CreatedBy string         `json:"createdBy"`
	Comment   string         `json:"comment,omitempty"`
	Enabled   bool           `json:"enabled"`
}

// LabelMatcher 标签匹配器.
type LabelMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"` // true=正匹配, false=取反
}

// InhibitionRule 抑制规则.
type InhibitionRule struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	SourceMatchers []LabelMatcher `json:"sourceMatchers"` // 触发抑制的告警
	TargetMatchers []LabelMatcher `json:"targetMatchers"` // 被抑制的告警
	Equal          []string       `json:"equal"`          // 需要相等的标签
	Enabled        bool           `json:"enabled"`
}

// AlertHistory 告警历史记录.
type AlertHistory struct {
	AlertID   string    `json:"alertId"`
	Action    string    `json:"action"` // created, acknowledged, silenced, resolved, escalated
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user,omitempty"`
	Details   string    `json:"details,omitempty"`
}

// AlertStats 告警统计.
type AlertStats struct {
	Total         int            `json:"total"`
	BySeverity    map[string]int `json:"bySeverity"`
	ByCategory    map[string]int `json:"byCategory"`
	ByStatus      map[string]int `json:"byStatus"`
	ActiveCount   int            `json:"activeCount"`
	SilencedCount int            `json:"silencedCount"`
	ResolvedCount int            `json:"resolvedCount"`
	AvgResolveMin float64        `json:"avgResolveMin"` // 平均解决时间（分钟）
}

// RouteChannel 路由通道.
type RouteChannel struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Type    string        `json:"type"` // email, webhook, slack, syslog, script
	Config  ChannelConfig `json:"config"`
	Enabled bool          `json:"enabled"`
}

// ChannelConfig 通道配置.
type ChannelConfig struct {
	Endpoint string            `json:"endpoint,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Template string            `json:"template,omitempty"`
	Timeout  time.Duration     `json:"timeout,omitempty"`
}

// RouteRule 路由规则.
type RouteRule struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Matchers []LabelMatcher `json:"matchers"`
	Channels []string       `json:"channels"` // 通道ID列表
	Continue bool           `json:"continue"` // 是否继续匹配下一条规则
	Priority int            `json:"priority"`
	Enabled  bool           `json:"enabled"`
}
