// Package compliancereport 数据泄露通知报告
package compliancereport

import (
	"fmt"
	"time"
)

// BreachSeverity 数据泄露严重程度.
type BreachSeverity string

const (
	BreachSeverityLow      BreachSeverity = "low"
	BreachSeverityMedium   BreachSeverity = "medium"
	BreachSeverityHigh     BreachSeverity = "high"
	BreachSeverityCritical BreachSeverity = "critical"
)

// BreachStatus 数据泄露状态.
type BreachStatus string

const (
	BreachStatusDetected    BreachStatus = "detected"    // 已检测
	BreachStatusContained   BreachStatus = "contained"   // 已遏制
	BreachStatusInvestigating BreachStatus = "investigating" // 调查中
	BreachStatusRemediating BreachStatus = "remediating" // 修复中
	BreachStatusClosed      BreachStatus = "closed"      // 已关闭
)

// BreachType 数据泄露类型.
type BreachType string

const (
	BreachTypeUnauthorizedAccess BreachType = "unauthorized_access" // 未授权访问
	BreachTypeDataTheft          BreachType = "data_theft"          // 数据窃取
	BreachTypeDataLoss           BreachType = "data_loss"           // 数据丢失
	BreachTypeRansomware         BreachType = "ransomware"          // 勒索软件
	BreachTypeInsiderThreat      BreachType = "insider_threat"      // 内部威胁
	BreachTypeMisconfig          BreachType = "misconfiguration"    // 配置错误
	BreachTypePhishing           BreachType = "phishing"            // 钓鱼攻击
)

// AffectedDataCategory 受影响数据类别.
type AffectedDataCategory struct {
	Category       CCPADataCategory `json:"category"`
	Description    string           `json:"description"`
	RecordCount    int              `json:"record_count"` // 受影响记录数
	IsEncrypted    bool             `json:"is_encrypted"` // 数据是否加密
}

// NotificationRecipient 通知接收方.
type NotificationRecipient struct {
	RecipientID   string    `json:"recipient_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // "authority", "individual", "partner"
	Email         string    `json:"email"`
	NotifiedAt    *time.Time `json:"notified_at,omitempty"`
	NotifiedMethod string   `json:"notified_method,omitempty"` // "email", "letter", "phone", "portal"
	Acknowledged  bool      `json:"acknowledged"`
}

// BreachTimelineEvent 泄露事件时间线.
type BreachTimelineEvent struct {
	EventID     string    `json:"event_id"`
	Timestamp   time.Time `json:"timestamp"`
	EventType   string    `json:"event_type"` // "detection", "containment", "investigation", "notification", "remediation"
	Description string    `json:"description"`
	Actor       string    `json:"actor"` // 操作者
}

// BreachNotificationReport 数据泄露通知报告.
type BreachNotificationReport struct {
	ReportID           string                    `json:"report_id"`
	BreachID           string                    `json:"breach_id"`
	GeneratedAt        time.Time                 `json:"generated_at"`
	Status             BreachStatus              `json:"status"`
	Severity           BreachSeverity            `json:"severity"`
	BreachType         BreachType                `json:"breach_type"`
	Title              string                    `json:"title"`
	Description        string                    `json:"description"`
	DetectedAt         time.Time                 `json:"detected_at"`
	OccurredAt         *time.Time                `json:"occurred_at,omitempty"` // 实际发生时间（如果已知）
	ContainedAt        *time.Time                `json:"contained_at,omitempty"`
	ClosedAt           *time.Time                `json:"closed_at,omitempty"`

	// 影响范围
	AffectedData       []AffectedDataCategory    `json:"affected_data"`
	TotalRecordsAffected int                   `json:"total_records_affected"`
	AffectedUserCount  int                       `json:"affected_user_count"`
	AffectedRegions    []string                  `json:"affected_regions"`

	// 根因分析
	RootCause          string                    `json:"root_cause"`
	AttackVector       string                    `json:"attack_vector,omitempty"`
	VulnerabilityRef   string                    `json:"vulnerability_ref,omitempty"` // CVE 等

	// 通知
	Notifications      []NotificationRecipient   `json:"notifications"`
	RegulatoryDeadline *time.Time                `json:"regulatory_deadline,omitempty"` // 法规通知截止
	NotificationCompliant bool                  `json:"notification_compliant"` // 是否在期限内通知

	// 响应措施
	Timeline           []BreachTimelineEvent     `json:"timeline"`
	RemediationActions []string                  `json:"remediation_actions"`
	PreventiveActions  []string                  `json:"preventive_actions"`

	// 法规相关
	ApplicableLaws     []string                  `json:"applicable_laws"` // GDPR, CCPA 等
	AuthorityNotified  bool                      `json:"authority_notified"`
	IndividualsNotified bool                     `json:"individuals_notified"`

	Summary            BreachSummary             `json:"summary"`
}

// BreachSummary 泄露摘要.
type BreachSummary struct {
	TotalTimlineHours     float64  `json:"total_timeline_hours"` // 从发现到关闭的总小时数
	ResponseTimeHours     float64  `json:"response_time_hours"`  // 从发现到遏制的小时数
	NotificationTimeHours float64  `json:"notification_time_hours"` // 从发现到通知的小时数
	ComplianceRating      string   `json:"compliance_rating"`    // "excellent", "good", "needs_improvement", "poor"
	LessonsLearned        []string `json:"lessons_learned"`
}

// BreachReportGenerator 泄露报告生成器.
type BreachReportGenerator struct{}

// NewBreachReportGenerator 创建泄露报告生成器.
func NewBreachReportGenerator() *BreachReportGenerator {
	return &BreachReportGenerator{}
}

// GenerateBreachReport 生成数据泄露通知报告.
func (g *BreachReportGenerator) GenerateBreachReport(config BreachReportConfig) *BreachNotificationReport {
	report := &BreachNotificationReport{
		ReportID:            GenerateID("breach"),
		BreachID:            config.BreachID,
		GeneratedAt:         time.Now(),
		Status:              config.Status,
		Severity:            config.Severity,
		BreachType:          config.BreachType,
		Title:               config.Title,
		Description:         config.Description,
		DetectedAt:          config.DetectedAt,
		OccurredAt:          config.OccurredAt,
		ContainedAt:         config.ContainedAt,
		ClosedAt:            config.ClosedAt,
		AffectedData:        config.AffectedData,
		AffectedUserCount:   config.AffectedUserCount,
		AffectedRegions:     config.AffectedRegions,
		RootCause:           config.RootCause,
		AttackVector:        config.AttackVector,
		VulnerabilityRef:    config.VulnerabilityRef,
		Notifications:       config.Notifications,
		RegulatoryDeadline:  config.RegulatoryDeadline,
		Timeline:            config.Timeline,
		RemediationActions:  config.RemediationActions,
		PreventiveActions:   config.PreventiveActions,
		ApplicableLaws:      config.ApplicableLaws,
	}

	// 计算受影响总记录数
	total := 0
	for _, ad := range config.AffectedData {
		total += ad.RecordCount
	}
	report.TotalRecordsAffected = total

	// 计算通知合规性
	report.AuthorityNotified = g.isAuthorityNotified(config.Notifications)
	report.IndividualsNotified = g.isIndividualsNotified(config.Notifications)
	report.NotificationCompliant = g.checkNotificationCompliance(config)

	// 生成摘要
	report.Summary = g.generateBreachSummary(report)

	return report
}

// BreachReportConfig 泄露报告配置.
type BreachReportConfig struct {
	BreachID           string
	Status             BreachStatus
	Severity           BreachSeverity
	BreachType         BreachType
	Title              string
	Description        string
	DetectedAt         time.Time
	OccurredAt         *time.Time
	ContainedAt        *time.Time
	ClosedAt           *time.Time
	AffectedData       []AffectedDataCategory
	AffectedUserCount  int
	AffectedRegions    []string
	RootCause          string
	AttackVector       string
	VulnerabilityRef   string
	Notifications      []NotificationRecipient
	RegulatoryDeadline *time.Time
	Timeline           []BreachTimelineEvent
	RemediationActions []string
	PreventiveActions  []string
	ApplicableLaws     []string
}

// isAuthorityNotified 是否已通知监管机构.
func (g *BreachReportGenerator) isAuthorityNotified(notifications []NotificationRecipient) bool {
	for _, n := range notifications {
		if n.Type == "authority" && n.NotifiedAt != nil {
			return true
		}
	}
	return false
}

// isIndividualsNotified 是否已通知受影响个人.
func (g *BreachReportGenerator) isIndividualsNotified(notifications []NotificationRecipient) bool {
	for _, n := range notifications {
		if n.Type == "individual" && n.NotifiedAt != nil {
			return true
		}
	}
	return false
}

// checkNotificationCompliance 检查通知合规性.
func (g *BreachReportGenerator) checkNotificationCompliance(config BreachReportConfig) bool {
	if config.RegulatoryDeadline == nil {
		return true // 无截止日期要求
	}

	for _, n := range config.Notifications {
		if n.Type == "authority" && n.NotifiedAt != nil {
			return n.NotifiedAt.Before(*config.RegulatoryDeadline)
		}
	}

	// 如果有截止日期但未通知监管机构
	return false
}

// generateBreachSummary 生成泄露事件摘要.
func (g *BreachReportGenerator) generateBreachSummary(report *BreachNotificationReport) BreachSummary {
	summary := BreachSummary{}

	// 计算总时间线
	if report.ClosedAt != nil {
		summary.TotalTimlineHours = report.ClosedAt.Sub(report.DetectedAt).Hours()
	} else {
		summary.TotalTimlineHours = time.Since(report.DetectedAt).Hours()
	}

	// 计算响应时间
	if report.ContainedAt != nil {
		summary.ResponseTimeHours = report.ContainedAt.Sub(report.DetectedAt).Hours()
	}

	// 计算通知时间
	var firstNotify *time.Time
	for _, n := range report.Notifications {
		if n.NotifiedAt != nil {
			if firstNotify == nil || n.NotifiedAt.Before(*firstNotify) {
				firstNotify = n.NotifiedAt
			}
		}
	}
	if firstNotify != nil {
		summary.NotificationTimeHours = firstNotify.Sub(report.DetectedAt).Hours()
	}

	// 评估合规评级
	summary.ComplianceRating = g.rateCompliance(report, summary)

	// 经验教训
	summary.LessonsLearned = g.generateLessonsLearned(report, summary)

	return summary
}

// rateCompliance 评估响应合规性.
func (g *BreachReportGenerator) rateCompliance(report *BreachNotificationReport, summary BreachSummary) string {
	score := 100

	// 响应时间评分
	if summary.ResponseTimeHours > 72 {
		score -= 30
	} else if summary.ResponseTimeHours > 24 {
		score -= 15
	}

	// 通知时间评分（GDPR 要求 72 小时内通知监管机构）
	if summary.NotificationTimeHours > 72 {
		score -= 30
	} else if summary.NotificationTimeHours > 48 {
		score -= 15
	}

	// 是否通知了监管机构
	if !report.AuthorityNotified {
		score -= 20
	}

	// 是否通知了受影响个人
	if !report.IndividualsNotified && report.AffectedUserCount > 0 {
		score -= 10
	}

	// 是否有根因分析
	if report.RootCause == "" {
		score -= 10
	}

	switch {
	case score >= 85:
		return "excellent"
	case score >= 70:
		return "good"
	case score >= 50:
		return "needs_improvement"
	default:
		return "poor"
	}
}

// generateLessonsLearned 生成经验教训.
func (g *BreachReportGenerator) generateLessonsLearned(report *BreachNotificationReport, summary BreachSummary) []string {
	var lessons []string

	if summary.ResponseTimeHours > 24 {
		lessons = append(lessons, fmt.Sprintf("事件响应时间为 %.1f 小时，应缩短至 24 小时以内", summary.ResponseTimeHours))
	}

	if report.RootCause != "" {
		lessons = append(lessons, fmt.Sprintf("根本原因: %s — 需针对性加固", report.RootCause))
	}

	if report.AttackVector != "" {
		lessons = append(lessons, fmt.Sprintf("攻击向量: %s — 需加强该方面防护", report.AttackVector))
	}

	// 检查是否有未加密的受影响数据
	for _, ad := range report.AffectedData {
		if !ad.IsEncrypted && ad.RecordCount > 0 {
			lessons = append(lessons, fmt.Sprintf("%s 类数据未加密存储，%d 条记录受影响", ad.Category, ad.RecordCount))
		}
	}

	if len(lessons) == 0 {
		lessons = append(lessons, "整体响应表现良好，继续保持现有安全流程")
	}

	return lessons
}
