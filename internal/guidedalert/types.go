// Package guidedalert 提供引导式告警系统
package guidedalert

import (
	"time"
)

// Severity 告警严重程度
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
	SeverityNotice   Severity = "notice"
)

// Category 告警类别
type Category string

const (
	CategoryStorage     Category = "storage"
	CategoryNetwork     Category = "network"
	CategoryCompute     Category = "compute"
	CategorySecurity    Category = "security"
	CategoryPerformance Category = "performance"
	CategoryService     Category = "service"
	CategoryHardware    Category = "hardware"
	CategorySystem      Category = "system"
)

// GuidedAlert 引导式告警
type GuidedAlert struct {
	ID                   string                `json:"id"`
	Title                string                `json:"title"`
	Message              string                `json:"message"`
	Severity             Severity              `json:"severity"`
	Category             Category              `json:"category"`
	Source               string                `json:"source,omitempty"`
	Resource             string                `json:"resource,omitempty"`
	Value                interface{}           `json:"value,omitempty"`
	Threshold            interface{}           `json:"threshold,omitempty"`
	TroubleshootingGuide *TroubleshootingGuide `json:"troubleshootingGuide,omitempty"`
	RelatedAlertIDs      []string              `json:"relatedAlertIds,omitempty"`
	Acknowledged         bool                  `json:"acknowledged"`
	Silenced             bool                  `json:"silenced"`
	CreatedAt            time.Time             `json:"createdAt"`
	UpdatedAt            time.Time             `json:"updatedAt,omitempty"`
	ResolvedAt           *time.Time            `json:"resolvedAt,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name                 string                `json:"name"`
	Condition            string                `json:"condition"`
	Severity             Severity              `json:"severity"`
	Category             Category              `json:"category"`
	Enabled              bool                  `json:"enabled"`
	TroubleshootingGuide *TroubleshootingGuide `json:"troubleshootingGuide,omitempty"`
}

// AlertSummary 告警汇总
type AlertSummary struct {
	Total      int              `json:"total"`
	ByCategory map[Category]int `json:"byCategory"`
	BySeverity map[Severity]int `json:"bySeverity"`
}

// TroubleshootingGuide 故障排查指南
type TroubleshootingGuide struct {
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Steps       []TroubleshootingStep `json:"steps"`
	DocsURL     string                `json:"docsUrl,omitempty"`
}

// TroubleshootingStep 故障排查步骤
type TroubleshootingStep struct {
	Order          int    `json:"order"`
	Description    string `json:"description"`
	Command        string `json:"command,omitempty"`
	ExpectedResult string `json:"expectedResult,omitempty"`
}
