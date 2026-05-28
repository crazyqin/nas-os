// Package guidedalert 提供引导式告警系统
// 对标 TrueNAS 26 Guided Alerts
package guidedalert

import "time"

// Severity 告警严重级别
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Category 告警分类
type Category string

const (
	CategoryStorage     Category = "storage"
	CategoryNetwork     Category = "network"
	CategorySecurity    Category = "security"
	CategoryPerformance Category = "performance"
	CategoryHardware    Category = "hardware"
)

// TroubleshootingStep 排查步骤
type TroubleshootingStep struct {
	Order           int    `json:"order"`
	Description     string `json:"description"`
	Command         string `json:"command,omitempty"`
	ExpectedResult  string `json:"expectedResult,omitempty"`
}

// TroubleshootingGuide 排查指南
type TroubleshootingGuide struct {
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Steps       []TroubleshootingStep `json:"steps"`
	DocsURL     string               `json:"docsUrl,omitempty"`
}

// GuidedAlert 引导式告警
type GuidedAlert struct {
	ID                 string               `json:"id"`
	Title              string               `json:"title"`
	Message            string               `json:"message"`
	Severity           Severity             `json:"severity"`
	Category           Category             `json:"category"`
	CreatedAt          time.Time            `json:"createdAt"`
	Acknowledged       bool                 `json:"acknowledged"`
	Silenced           bool                 `json:"silenced"`
	TroubleshootingGuide *TroubleshootingGuide `json:"troubleshootingGuide,omitempty"`
	RelatedAlertIDs    []string             `json:"relatedAlertIds,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name                string               `json:"name"`
	Condition           string               `json:"condition"`
	Severity            Severity             `json:"severity"`
	Category            Category             `json:"category"`
	TroubleshootingGuide *TroubleshootingGuide `json:"troubleshootingGuide,omitempty"`
}

// AlertSummary 告警汇总
type AlertSummary struct {
	Total    int            `json:"total"`
	ByCategory map[Category]int `json:"byCategory"`
	BySeverity map[Severity]int `json:"bySeverity"`
}
