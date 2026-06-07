// Package alertaggregator 提供跨模块告警聚合、去重、升级、抑制和关联分析功能
package alertaggregator

import (
	"errors"
	"time"
)

// ========== 告警来源模块常量 ==========

// 告警来源模块
const (
	ModuleDisk     = "disk"     // 磁盘模块
	ModuleNetwork  = "network"  // 网络模块
	ModuleSecurity = "security" // 安全模块
	ModuleApp      = "app"      // 应用模块
	ModuleSystem   = "system"   // 系统模块
)

// ========== 告警级别常量 ==========

// 告警级别
const (
	AlertLevelInfo     = "info"     // 信息
	AlertLevelWarning  = "warning"  // 警告
	AlertLevelError    = "error"    // 错误
	AlertLevelCritical = "critical" // 严重
)

// ========== 告警状态常量 ==========

// 告警状态
const (
	AlertStatusActive   = "active"   // 活跃
	AlertStatusResolved = "resolved" // 已解决
	AlertStatusSilenced = "silenced" // 已静默
	AlertStatusExpired  = "expired"  // 已过期
)

// ========== SLA级别常量 ==========

// SLA级别
const (
	SLALevelP1 = "P1" // 最高优先级，5分钟响应
	SLALevelP2 = "P2" // 高优先级，15分钟响应
	SLALevelP3 = "P3" // 中优先级，1小时响应
	SLALevelP4 = "P4" // 低优先级，4小时响应
)

// ========== 错误定义 ==========

var (
	// ErrAlertNotFound 告警不存在
	ErrAlertNotFound = errors.New("alert not found")
	// ErrGroupNotFound 告警组不存在
	ErrGroupNotFound = errors.New("alert group not found")
	// ErrRuleNotFound 规则不存在
	ErrRuleNotFound = errors.New("rule not found")
	// ErrSilenceNotFound 静默规则不存在
	ErrSilenceNotFound = errors.New("silence rule not found")
	// ErrInvalidQuery 无效查询
	ErrInvalidQuery = errors.New("invalid query")
)

// ========== 核心数据类型 ==========

// Alert 告警.
type Alert struct {
	ID          string            `json:"id"`           // 告警唯一ID
	Fingerprint string            `json:"fingerprint"`  // 告警指纹（用于去重）
	Module      string            `json:"module"`       // 来源模块：disk/network/security/app
	Title       string            `json:"title"`        // 告警标题
	Description string            `json:"description"`  // 告警描述
	Level       string            `json:"level"`        // 告警级别：info/warning/error/critical
	Status      string            `json:"status"`       // 告警状态：active/resolved/silenced/expired
	Source      string            `json:"source"`       // 具体来源（如主机名、IP、应用名）
	Labels      map[string]string `json:"labels"`       // 标签
	Annotations map[string]string `json:"annotations"`  // 注解
	GroupID     string            `json:"group_id"`     // 所属告警组ID
	SLA         string            `json:"sla"`          // SLA级别：P1/P2/P3/P4
	Count       int               `json:"count"`        // 聚合计数（相似告警合并后）
	FirstSeen   time.Time         `json:"first_seen"`   // 首次出现时间
	LastSeen    time.Time         `json:"last_seen"`    // 最后出现时间
	ResolvedAt  *time.Time        `json:"resolved_at"`  // 解决时间
	EscalatedAt *time.Time        `json:"escalated_at"` // 升级时间
	SilencedBy  string            `json:"silenced_by"`  // 静默规则ID
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AlertGroup 告警组（用于聚合相似告警）.
type AlertGroup struct {
	ID          string    `json:"id"`          // 告警组ID
	Name        string    `json:"name"`        // 组名称
	Fingerprint string    `json:"fingerprint"` // 组指纹（基于模块、来源、标题模板）
	Module      string    `json:"module"`      // 来源模块
	AlertIDs    []string  `json:"alert_ids"`   // 组内告警ID列表
	Count       int       `json:"count"`       // 告警数量
	MaxLevel    string    `json:"max_level"`   // 最高级别
	FirstSeen   time.Time `json:"first_seen"`  // 首次出现时间
	LastSeen    time.Time `json:"last_seen"`   // 最后出现时间
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AggregationRule 聚合规则.
type AggregationRule struct {
	ID          string        `json:"id"`          // 规则ID
	Name        string        `json:"name"`        // 规则名称
	Description string        `json:"description"` // 规则描述
	Module      string        `json:"module"`      // 适用模块（空表示全部）
	GroupBy     []string      `json:"group_by"`    // 聚合维度：module, source, labels.xxx
	Window      time.Duration `json:"window"`      // 聚合时间窗口
	MaxCount    int           `json:"max_count"`   // 最大聚合数量
	Enabled     bool          `json:"enabled"`     // 是否启用
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// EscalationPolicy 升级策略.
type EscalationPolicy struct {
	ID          string                `json:"id"`          // 策略ID
	Name        string                `json:"name"`        // 策略名称
	Description string                `json:"description"` // 策略描述
	Module      string                `json:"module"`      // 适用模块（空表示全部）
	Level       string                `json:"level"`       // 适用级别（空表示全部）
	Conditions  []EscalationCondition `json:"conditions"`  // 升级条件
	Actions     []EscalationAction    `json:"actions"`     // 升级动作
	Enabled     bool                  `json:"enabled"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// EscalationCondition 升级条件.
type EscalationCondition struct {
	Type     string        `json:"type"`      // 条件类型：duration, count, severity
	Duration time.Duration `json:"duration"`  // 持续时间（用于 duration 类型）
	Count    int           `json:"count"`     // 告警计数（用于 count 类型）
	MinLevel string        `json:"min_level"` // 最低级别（用于 severity 类型）
}

// EscalationAction 升级动作.
type EscalationAction struct {
	Type     string `json:"type"`      // 动作类型：escalate_level, notify, webhook
	Target   string `json:"target"`    // 目标（通知渠道、webhook URL等）
	NewLevel string `json:"new_level"` // 新级别（用于 escalate_level 类型）
	Message  string `json:"message"`   // 消息模板
}

// SilenceRule 静默规则.
type SilenceRule struct {
	ID          string            `json:"id"`          // 规则ID
	Name        string            `json:"name"`        // 规则名称
	Description string            `json:"description"` // 规则描述
	Module      string            `json:"module"`      // 匹配模块（空表示全部）
	Source      string            `json:"source"`      // 匹配来源（正则）
	Level       string            `json:"level"`       // 匹配级别（空表示全部）
	Labels      map[string]string `json:"labels"`      // 匹配标签
	StartsAt    time.Time         `json:"starts_at"`   // 生效开始时间
	EndsAt      time.Time         `json:"ends_at"`     // 生效结束时间
	Creator     string            `json:"creator"`     // 创建者
	Enabled     bool              `json:"enabled"`     // 是否启用
	CreatedAt   time.Time         `json:"created_at"`
}

// CorrelationRule 关联规则.
type CorrelationRule struct {
	ID          string        `json:"id"`           // 规则ID
	Name        string        `json:"name"`         // 规则名称
	Description string        `json:"description"`  // 规则描述
	SourceRules []string      `json:"source_rules"` // 源告警规则ID列表
	TargetRule  string        `json:"target_rule"`  // 目标告警规则ID
	Window      time.Duration `json:"window"`       // 关联时间窗口
	Logic       string        `json:"logic"`        // 关联逻辑：and, or, sequence
	RootCause   string        `json:"root_cause"`   // 根因描述
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// CorrelationResult 关联分析结果.
type CorrelationResult struct {
	ID         string          `json:"id"`         // 结果ID
	RootCause  string          `json:"root_cause"` // 根因描述
	AlertIDs   []string        `json:"alert_ids"`  // 关联的告警ID列表
	RuleID     string          `json:"rule_id"`    // 触发的关联规则ID
	Confidence float64         `json:"confidence"` // 置信度 0-1
	Timeline   []TimelineEntry `json:"timeline"`   // 时间线
	CreatedAt  time.Time       `json:"created_at"`
}

// TimelineEntry 时间线条目.
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	AlertID   string    `json:"alert_id,omitempty"`
	Level     string    `json:"level"`
}

// SLAPolicy SLA策略.
type SLAPolicy struct {
	ID           string        `json:"id"`            // 策略ID
	Name         string        `json:"name"`          // 策略名称
	Level        string        `json:"level"`         // SLA级别：P1/P2/P3/P4
	ResponseTime time.Duration `json:"response_time"` // 响应时间
	ResolveTime  time.Duration `json:"resolve_time"`  // 解决时间
	NotifyBefore time.Duration `json:"notify_before"` // 提前通知时间
	Enabled      bool          `json:"enabled"`
	CreatedAt    time.Time     `json:"created_at"`
}

// SLATracking SLA跟踪记录.
type SLATracking struct {
	ID           string     `json:"id"`            // 跟踪ID
	AlertID      string     `json:"alert_id"`      // 告警ID
	SLAPolicyID  string     `json:"sla_policy_id"` // SLA策略ID
	Level        string     `json:"level"`         // SLA级别
	StartTime    time.Time  `json:"start_time"`    // 开始时间
	ResponseTime *time.Time `json:"response_time"` // 响应时间
	ResolveTime  *time.Time `json:"resolve_time"`  // 解决时间
	ResponseDue  time.Time  `json:"response_due"`  // 响应截止时间
	ResolveDue   time.Time  `json:"resolve_due"`   // 解决截止时间
	IsBreached   bool       `json:"is_breached"`   // 是否违反SLA
	BreachReason string     `json:"breach_reason"` // 违反原因
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AlertRule 自定义告警规则.
type AlertRule struct {
	ID          string            `json:"id"`          // 规则ID
	Name        string            `json:"name"`        // 规则名称
	Description string            `json:"description"` // 规则描述
	Module      string            `json:"module"`      // 适用模块
	Level       string            `json:"level"`       // 触发告警级别
	Conditions  []RuleCondition   `json:"conditions"`  // 触发条件
	Labels      map[string]string `json:"labels"`      // 附加标签
	Annotations map[string]string `json:"annotations"` // 附加注解
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RuleCondition 规则条件.
type RuleCondition struct {
	Field    string `json:"field"`    // 字段名：source, label.xxx, annotation.xxx
	Operator string `json:"operator"` // 操作符：eq, neq, regex, gt, lt, gte, lte, in, contains
	Value    string `json:"value"`    // 值
}

// AlertStats 告警统计.
type AlertStats struct {
	TotalAlerts    int            `json:"total_alerts"`
	ActiveAlerts   int            `json:"active_alerts"`
	ResolvedAlerts int            `json:"resolved_alerts"`
	AlertsByModule map[string]int `json:"alerts_by_module"`
	AlertsByLevel  map[string]int `json:"alerts_by_level"`
	AlertsBySLA    map[string]int `json:"alerts_by_sla"`
	TrendData      []TrendPoint   `json:"trend_data"`
	TopSources     []SourceStat   `json:"top_sources"`
	SLACompliance  float64        `json:"sla_compliance"` // SLA合规率 0-100
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Total     int       `json:"total"`
	Critical  int       `json:"critical"`
	Error     int       `json:"error"`
	Warning   int       `json:"warning"`
	Info      int       `json:"info"`
}

// SourceStat 来源统计.
type SourceStat struct {
	Source   string    `json:"source"`
	Module   string    `json:"module"`
	Count    int       `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

// ========== 请求/响应类型 ==========

// CreateAlertRequest 创建告警请求.
type CreateAlertRequest struct {
	Module      string            `json:"module" binding:"required"` // 来源模块
	Title       string            `json:"title" binding:"required"`  // 告警标题
	Description string            `json:"description"`               // 告警描述
	Level       string            `json:"level" binding:"required"`  // 告警级别
	Source      string            `json:"source"`                    // 具体来源
	Labels      map[string]string `json:"labels"`                    // 标签
	Annotations map[string]string `json:"annotations"`               // 注解
	SLA         string            `json:"sla"`                       // SLA级别
}

// UpdateAlertRequest 更新告警请求.
type UpdateAlertRequest struct {
	Status *string `json:"status,omitempty"` // 告警状态
	Level  *string `json:"level,omitempty"`  // 告警级别
	Notes  *string `json:"notes,omitempty"`  // 备注
}

// QueryAlertsRequest 查询告警请求.
type QueryAlertsRequest struct {
	Module    string     `form:"module"`     // 来源模块
	Level     string     `form:"level"`      // 告警级别
	Status    string     `form:"status"`     // 告警状态
	Source    string     `form:"source"`     // 来源
	SLA       string     `form:"sla"`        // SLA级别
	StartTime *time.Time `form:"start_time"` // 开始时间
	EndTime   *time.Time `form:"end_time"`   // 结束时间
	Keyword   string     `form:"keyword"`    // 关键词
	Page      int        `form:"page"`       // 页码
	PageSize  int        `form:"page_size"`  // 每页数量
}

// CreateAggregationRuleRequest 创建聚合规则请求.
type CreateAggregationRuleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Module      string   `json:"module"`
	GroupBy     []string `json:"group_by"`
	Window      int      `json:"window_seconds"`
	MaxCount    int      `json:"max_count"`
}

// CreateEscalationPolicyRequest 创建升级策略请求.
type CreateEscalationPolicyRequest struct {
	Name        string                `json:"name" binding:"required"`
	Description string                `json:"description"`
	Module      string                `json:"module"`
	Level       string                `json:"level"`
	Conditions  []EscalationCondition `json:"conditions"`
	Actions     []EscalationAction    `json:"actions"`
}

// CreateSilenceRuleRequest 创建静默规则请求.
type CreateSilenceRuleRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Module      string            `json:"module"`
	Source      string            `json:"source"`
	Level       string            `json:"level"`
	Labels      map[string]string `json:"labels"`
	StartsAt    time.Time         `json:"starts_at" binding:"required"`
	EndsAt      time.Time         `json:"ends_at" binding:"required"`
	Creator     string            `json:"creator"`
}

// CreateCorrelationRuleRequest 创建关联规则请求.
type CreateCorrelationRuleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	SourceRules []string `json:"source_rules" binding:"required"`
	TargetRule  string   `json:"target_rule"`
	Window      int      `json:"window_seconds"`
	Logic       string   `json:"logic" binding:"required"` // and, or, sequence
	RootCause   string   `json:"root_cause"`
}

// CreateAlertRuleRequest 创建自定义告警规则请求.
type CreateAlertRuleRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Module      string            `json:"module"`
	Level       string            `json:"level" binding:"required"`
	Conditions  []RuleCondition   `json:"conditions" binding:"required"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// CreateSLAPolicyRequest 创建SLA策略请求.
type CreateSLAPolicyRequest struct {
	Name         string `json:"name" binding:"required"`
	Level        string `json:"level" binding:"required"`
	ResponseTime int    `json:"response_time_seconds"`
	ResolveTime  int    `json:"resolve_time_seconds"`
	NotifyBefore int    `json:"notify_before_seconds"`
}

// StatsQueryRequest 统计查询请求.
type StatsQueryRequest struct {
	StartTime *time.Time `form:"start_time"`
	EndTime   *time.Time `form:"end_time"`
	Module    string     `form:"module"`
	Interval  string     `form:"interval"` // minute, hour, day
}
