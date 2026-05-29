// Package alertguided 提供引导式故障排除告警系统
// 对标 TrueNAS 26 的 Guided Alerts 功能
package alertguided

import (
	"time"
)

// Severity 告警严重级别
type Severity string

const (
	SeverityInfo      Severity = "INFO"
	SeverityWarning   Severity = "WARNING"
	SeverityCritical  Severity = "CRITICAL"
	SeverityEmergency Severity = "EMERGENCY"
)

// AlertStatus 告警处置状态
type AlertStatus string

const (
	StatusOpen       AlertStatus = "OPEN"
	StatusInProgress AlertStatus = "IN_PROGRESS"
	StatusResolved   AlertStatus = "RESOLVED"
	StatusDismissed  AlertStatus = "DISMISSED"
)

// Category 告警分类
type Category string

const (
	CategoryStorage     Category = "storage"
	CategoryNetwork     Category = "network"
	CategorySecurity    Category = "security"
	CategoryPerformance Category = "performance"
	CategoryHardware    Category = "hardware"
	CategorySystem      Category = "system"
)

// AutoFixAction 自动修复动作
type AutoFixAction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	RiskLevel   string `json:"riskLevel"` // low/medium/high
	RequiresAck bool   `json:"requiresAck"`
}

// TroubleshootingStep 排查步骤
type TroubleshootingStep struct {
	Order          int    `json:"order"`
	Description    string `json:"description"`
	Command        string `json:"command,omitempty"`
	ExpectedResult string `json:"expectedResult,omitempty"`
	IsOptional     bool   `json:"isOptional,omitempty"`
}

// TroubleshootingGuide 排查指南
type TroubleshootingGuide struct {
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Steps       []TroubleshootingStep `json:"steps"`
	DocsURL     string               `json:"docsUrl,omitempty"`
}

// ContextInfo 上下文信息
type ContextInfo struct {
	SystemLoad    float64  `json:"systemLoad,omitempty"`
	DiskUsage     float64  `json:"diskUsage,omitempty"`
	MemoryUsage   float64  `json:"memoryUsage,omitempty"`
	NetworkStatus string   `json:"networkStatus,omitempty"`
	ActiveAlerts  int      `json:"activeAlerts"`
	RelatedItems  []string `json:"relatedItems,omitempty"`
}

// StatusChange 状态变更记录
type StatusChange struct {
	From      AlertStatus `json:"from"`
	To        AlertStatus `json:"to"`
	ChangedAt time.Time   `json:"changedAt"`
	ChangedBy string      `json:"changedBy,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

// GuidedAlert 引导式告警
type GuidedAlert struct {
	ID                   string               `json:"id"`
	Title                string               `json:"title"`
	Message              string               `json:"message"`
	Severity             Severity             `json:"severity"`
	Category             Category             `json:"category"`
	Status               AlertStatus          `json:"status"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`
	Acknowledged         bool                 `json:"acknowledged"`
	Silenced             bool                 `json:"silenced"`
	TroubleshootingGuide *TroubleshootingGuide `json:"troubleshootingGuide,omitempty"`
	AutoFixActions       []AutoFixAction      `json:"autoFixActions,omitempty"`
	Context              *ContextInfo         `json:"context,omitempty"`
	RelatedAlertIDs      []string             `json:"relatedAlertIds,omitempty"`
	AggregationKey       string               `json:"aggregationKey,omitempty"`
	Count                int                  `json:"count"`
	StatusHistory        []StatusChange       `json:"statusHistory,omitempty"`
	Tags                 []string             `json:"tags,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name                string               `json:"name"`
	Condition           string               `json:"condition"`
	Severity            Severity             `json:"severity"`
	Category            Category             `json:"category"`
	AggregationKey      string               `json:"aggregationKey,omitempty"`
	TroubleshootingGuide *TroubleshootingGuide `json:"troubleshootingGuide,omitempty"`
	AutoFixActions      []AutoFixAction      `json:"autoFixActions,omitempty"`
	Tags                []string             `json:"tags,omitempty"`
}

// AlertSummary 告警汇总
type AlertSummary struct {
	Total         int             `json:"total"`
	ByCategory    map[Category]int `json:"byCategory"`
	BySeverity    map[Severity]int `json:"bySeverity"`
	ByStatus      map[AlertStatus]int `json:"byStatus"`
	Acknowledged  int             `json:"acknowledged"`
	Silenced      int             `json:"silenced"`
	Aggregated    int             `json:"aggregated"`
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name           string               `json:"name" binding:"required"`
	Condition      string               `json:"condition" binding:"required"`
	Severity       Severity             `json:"severity" binding:"required"`
	Category       Category             `json:"category" binding:"required"`
	AggregationKey string               `json:"aggregationKey,omitempty"`
	Tags           []string             `json:"tags,omitempty"`
}

// UpdateStatusRequest 更新状态请求
type UpdateStatusRequest struct {
	Status AlertStatus `json:"status" binding:"required"`
	Reason string      `json:"reason,omitempty"`
}

// AcknowledgeRequest 确认告警请求
type AcknowledgeRequest struct {
	Reason string `json:"reason,omitempty"`
}
