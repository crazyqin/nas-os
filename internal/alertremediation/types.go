// Package alertremediation 提供引导式告警修复引擎
// 对标 TrueNAS 26 的 Guided Alerts 功能，当系统产生告警时，
// 不仅显示告警内容，还提供排查步骤、一键修复操作和根因分析。
package alertremediation

import (
	"time"
)

// AlertSeverity 告警严重级别.
type AlertSeverity string

const (
	// SeverityEmergency 紧急：系统不可用，需立即处理
	SeverityEmergency AlertSeverity = "emergency"
	// SeverityCritical 严重：核心功能受损，需优先处理
	SeverityCritical AlertSeverity = "critical"
	// SeverityWarning 警告：性能或可靠性受影响
	SeverityWarning AlertSeverity = "warning"
	// SeverityInfo 信息：仅供参考，无需立即处理
	SeverityInfo AlertSeverity = "info"
)

// SeverityOrder 告警级别排序权重（数值越大越紧急）.
var SeverityOrder = map[AlertSeverity]int{
	SeverityInfo:      1,
	SeverityWarning:   2,
	SeverityCritical:  3,
	SeverityEmergency: 4,
}

// RemediationStatus 修复状态.
type RemediationStatus string

const (
	// StatusPending 等待处理
	StatusPending RemediationStatus = "pending"
	// StatusInProgress 处理中
	StatusInProgress RemediationStatus = "in_progress"
	// StatusCompleted 已完成
	StatusCompleted RemediationStatus = "completed"
	// StatusFailed 处理失败
	StatusFailed RemediationStatus = "failed"
	// StatusSkipped 已跳过
	StatusSkipped RemediationStatus = "skipped"
)

// ActionType 修复动作类型.
type ActionType string

const (
	// ActionServiceRestart 重启服务
	ActionServiceRestart ActionType = "service_restart"
	// ActionDiskCleanup 磁盘清理
	ActionDiskCleanup ActionType = "disk_cleanup"
	// ActionConfigChange 修改配置
	ActionConfigChange ActionType = "config_change"
	// ActionScript 执行脚本
	ActionScript ActionType = "script"
	// ActionCommand 执行命令
	ActionCommand ActionType = "command"
	// ActionZFSRepair ZFS 修复
	ActionZFSRepair ActionType = "zfs_repair"
	// ActionNetworkReset 网络重置
	ActionNetworkReset ActionType = "network_reset"
	// ActionLogRotation 日志轮转
	ActionLogRotation ActionType = "log_rotation"
	// ActionNotifyUser 通知用户
	ActionNotifyUser ActionType = "notify_user"
)

// AlertCategory 告警分类.
type AlertCategory string

const (
	// CatStorage 存储类
	CatStorage AlertCategory = "storage"
	// CatNetwork 网络类
	CatNetwork AlertCategory = "network"
	// CatSystem 系统类
	CatSystem AlertCategory = "system"
	// CatService 服务类
	CatService AlertCategory = "service"
	// CatSecurity 安全类
	CatSecurity AlertCategory = "security"
)

// Alert 告警信息.
type Alert struct {
	ID          string            `json:"id"`
	RuleID      string            `json:"rule_id,omitempty"` // 匹配的规则ID
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	Severity    AlertSeverity     `json:"severity"`
	Category    AlertCategory     `json:"category"`
	Source      string            `json:"source"`           // 告警来源（模块名）
	Labels      map[string]string `json:"labels,omitempty"` // 附加标签
	Timestamp   time.Time         `json:"timestamp"`
	Acknowledged bool             `json:"acknowledged"`
	Resolved    bool              `json:"resolved"`
}

// RemediationRule 告警修复规则.
// 定义告警匹配条件和修复方案模板.
type RemediationRule struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Category     AlertCategory  `json:"category"`
	Severity     AlertSeverity  `json:"severity"`
	MatchFunc    MatchFunction  `json:"-"`          // 告警匹配函数
	RootCause    string         `json:"root_cause"` // 根因描述
	Steps        []StepTemplate `json:"steps"`      // 排查步骤模板
	Actions      []ActionTemplate `json:"actions"`  // 修复动作模板
	Enabled      bool           `json:"enabled"`
	Priority     int            `json:"priority"` // 规则优先级（越小越优先）
}

// MatchFunction 告警匹配函数签名.
// 返回 true 表示该规则适用于此告警.
type MatchFunction func(alert *Alert) bool

// StepTemplate 排查步骤模板.
type StepTemplate struct {
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"` // 可选的排查命令
}

// ActionTemplate 修复动作模板.
type ActionTemplate struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        ActionType `json:"type"`
	Command     string     `json:"command,omitempty"`  // 执行的命令
	Parameters  map[string]string `json:"parameters,omitempty"` // 动态参数
	Destructive bool       `json:"destructive"`        // 是否为破坏性操作
	RequiresConfirm bool   `json:"requires_confirm"`   // 是否需要确认
}

// RemediationPlan 修复方案.
// 由 RemediationEngine.Analyze 生成，包含完整的修复引导信息.
type RemediationPlan struct {
	ID          string           `json:"id"`
	AlertID     string           `json:"alert_id"`
	RuleID      string           `json:"rule_id"`
	Alert       *Alert           `json:"alert"`
	RootCause   RootCauseAnalysis `json:"root_cause"`
	Steps       []RemediationStep  `json:"steps"`
	Actions     []RemediationAction `json:"actions"`
	Status      RemediationStatus  `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
}

// RootCauseAnalysis 根因分析结果.
type RootCauseAnalysis struct {
	Summary     string   `json:"summary"`               // 根因概述
	Description string   `json:"description"`           // 详细描述
	PossibleCauses []string `json:"possible_causes"`     // 可能的原因列表
	Impact      string   `json:"impact"`                 // 影响范围
	References  []string `json:"references,omitempty"`   // 参考文档链接
}

// RemediationStep 排查步骤.
type RemediationStep struct {
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Completed   bool   `json:"completed"`
}

// RemediationAction 修复动作.
type RemediationAction struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Type            ActionType `json:"type"`
	Command         string     `json:"command,omitempty"`
	Parameters      map[string]string `json:"parameters,omitempty"`
	Destructive     bool       `json:"destructive"`
	RequiresConfirm bool       `json:"requires_confirm"`
	Status          RemediationStatus `json:"status"`
	Result          string     `json:"result,omitempty"`   // 执行结果描述
	ExecutedAt      *time.Time `json:"executed_at,omitempty"`
}

// ActionResult 动作执行结果.
type ActionResult struct {
	ActionID  string           `json:"action_id"`
	Success   bool             `json:"success"`
	Message   string           `json:"message"`
	Output    string           `json:"output,omitempty"`  // 命令输出
	Error     string           `json:"error,omitempty"`   // 错误信息
	Duration  time.Duration    `json:"duration"`
	Timestamp time.Time        `json:"timestamp"`
}

// AnalyzeRequest 手动分析请求.
type AnalyzeRequest struct {
	Title    string            `json:"title" binding:"required"`
	Message  string            `json:"message" binding:"required"`
	Severity AlertSeverity     `json:"severity"`
	Category AlertCategory     `json:"category"`
	Source   string            `json:"source"`
	Labels   map[string]string `json:"labels,omitempty"`
}
