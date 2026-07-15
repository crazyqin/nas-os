// Package smartalert 提供引导式告警系统（Guided Alerts）。
// 对标 TrueNAS 26 的 Guided Alerts 功能，每条告警附带排查步骤和修复引导。
package smartalert

import (
	"time"
)

// Severity 告警严重等级.
type Severity string

const (
	SeverityCritical Severity = "critical" // 紧急：需立即处理
	SeverityWarning  Severity = "warning"  // 警告：需尽快处理
	SeverityInfo     Severity = "info"     // 信息：仅供参考
	SeverityResolved Severity = "resolved" // 已解决
)

// SeverityWeight 严重等级权重（越大越严重）.
var SeverityWeight = map[Severity]int{
	SeverityInfo:     1,
	SeverityWarning:  2,
	SeverityCritical: 3,
	SeverityResolved: 0,
}

// Category 告警分类.
type Category string

const (
	CategoryDisk     Category = "disk"        // 磁盘故障
	CategorySpace    Category = "space"       // 空间不足
	CategoryPerf     Category = "performance" // 性能异常
	CategoryNetwork  Category = "network"     // 网络问题
	CategorySecurity Category = "security"    // 安全威胁
	CategoryService  Category = "service"     // 服务故障
	CategorySystem   Category = "system"      // 系统异常
)

// AlertState 告警状态.
type AlertState string

const (
	StateActive       AlertState = "active"       // 活跃
	StateAcknowledged AlertState = "acknowledged" // 已确认
	StateSilenced     AlertState = "silenced"     // 已静默
	StateEscalated    AlertState = "escalated"    // 已升级
	StateResolved     AlertState = "resolved"     // 已解决
)

// SmartAlert 智能告警条目.
// 每条告警附带排查步骤、修复命令和参考文档链接.
type SmartAlert struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Severity         Severity          `json:"severity"`
	OriginalSeverity Severity          `json:"original_severity"` // 原始严重等级
	Category         Category          `json:"category"`
	State            AlertState        `json:"state"`
	Source           string            `json:"source"`           // 告警来源模块
	Resource         string            `json:"resource"`         // 关联资源（如 /dev/sda, pool tank）
	Labels           map[string]string `json:"labels,omitempty"` // 附加标签

	// 引导信息
	TroubleshootSteps []TroubleshootStep `json:"troubleshoot_steps"`      // 排查步骤
	FixCommands       []FixCommand       `json:"fix_commands"`            // 修复命令
	References        []string           `json:"references"`              // 参考文档链接
	RootCauseID       string             `json:"root_cause_id,omitempty"` // 关联的根因ID

	// 生命周期
	FirstSeen      time.Time  `json:"first_seen"`
	LastSeen       time.Time  `json:"last_seen"`
	EscalatedAt    *time.Time `json:"escalated_at,omitempty"`    // 升级时间
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"` // 确认时间
	AcknowledgedBy string     `json:"acknowledged_by,omitempty"` // 确认人
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// TroubleshootStep 排查步骤.
type TroubleshootStep struct {
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`  // 检查命令
	Expected    string `json:"expected,omitempty"` // 期望输出
}

// FixCommand 修复命令.
type FixCommand struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Command         string `json:"command"`
	Destructive     bool   `json:"destructive"`      // 是否为破坏性操作
	RequiresConfirm bool   `json:"requires_confirm"` // 是否需要确认
}

// Guide 告警处置引导（API 返回给前端的完整引导信息）.
type Guide struct {
	Alert       *SmartAlert      `json:"alert"`
	Summary     string           `json:"summary"`               // 根因概述
	Correlation *CorrelationInfo `json:"correlation,omitempty"` // 关联信息
}

// CorrelationInfo 告警关联信息.
type CorrelationInfo struct {
	RootCauseID     string   `json:"root_cause_id"`
	Description     string   `json:"description"`
	RelatedAlertIDs []string `json:"related_alert_ids"` // 关联的其他告警
}

// SilenceRule 告警静默规则.
type SilenceRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    Category  `json:"category,omitempty"` // 按分类静默（空=全部）
	AlertID     string    `json:"alert_id,omitempty"` // 按告警ID静默
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	Enabled     bool      `json:"enabled"`
}

// EscalationPolicy 告警升级策略.
type EscalationPolicy struct {
	UpgradeAfter time.Duration `json:"upgrade_after"` // 未处理多久后升级
	MaxSeverity  Severity      `json:"max_severity"`  // 最高升到什么级别
}

// RootCause 根因条目.
type RootCause struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	Category        Category `json:"category"`
	RelatedAlertIDs []string `json:"related_alert_ids"` // 关联的告警ID
}

// ========== API 请求/响应类型 ==========

// SilenceRequest 创建静默规则请求.
type SilenceRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	AlertID     string   `json:"alert_id"`
	DurationMin int      `json:"duration_min" binding:"required,min=1"` // 静默时长（分钟）
	CreatedBy   string   `json:"created_by"`
}

// AcknowledgeRequest 确认告警请求.
type AcknowledgeRequest struct {
	Operator string `json:"operator"` // 操作人
}

// ListQuery 告警列表查询参数.
type ListQuery struct {
	Category Category   `form:"category"`
	Severity Severity   `form:"severity"`
	State    AlertState `form:"state"`
}
