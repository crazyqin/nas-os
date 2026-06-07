// Package databreach 提供数据泄露应急响应管理功能
package databreach

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBreachNotFound 泄露事件不存在.
	ErrBreachNotFound = fmt.Errorf("泄露事件不存在")
	// ErrInvalidBreachType 无效的泄露类型.
	ErrInvalidBreachType = fmt.Errorf("无效的泄露类型")
	// ErrInvalidClassification 无效的数据分类.
	ErrInvalidClassification = fmt.Errorf("无效的数据分类")
	// ErrInvalidStatus 无效的状态.
	ErrInvalidStatus = fmt.Errorf("无效的状态")
	// ErrNotificationTimerAlreadyStarted 通知计时器已启动.
	ErrNotificationTimerAlreadyStarted = fmt.Errorf("通知计时器已启动")
	// ErrNotificationDeadlineNotSet 通知截止时间未设置.
	ErrNotificationDeadlineNotSet = fmt.Errorf("通知截止时间未设置")
)

// ========== 泄露类型枚举 ==========

// BreachType 泄露类型.
type BreachType string

const (
	// BreachTypeUnauthorizedAccess 未授权访问.
	BreachTypeUnauthorizedAccess BreachType = "UnauthorizedAccess"
	// BreachTypeDataLeak 数据泄漏.
	BreachTypeDataLeak BreachType = "DataLeak"
	// BreachTypeRansomware 勒索软件.
	BreachTypeRansomware BreachType = "Ransomware"
	// BreachTypeInsiderThreat 内部威胁.
	BreachTypeInsiderThreat BreachType = "InsiderThreat"
	// BreachTypeMisconfig 配置错误.
	BreachTypeMisconfig BreachType = "Misconfig"
	// BreachTypeThirdParty 第三方泄露.
	BreachTypeThirdParty BreachType = "ThirdParty"
)

// validBreachTypes 有效的泄露类型集合.
var validBreachTypes = map[BreachType]bool{
	BreachTypeUnauthorizedAccess: true,
	BreachTypeDataLeak:           true,
	BreachTypeRansomware:         true,
	BreachTypeInsiderThreat:      true,
	BreachTypeMisconfig:          true,
	BreachTypeThirdParty:         true,
}

// ========== 数据分类枚举 ==========

// DataClassification 数据分类.
type DataClassification string

const (
	// ClassificationPublic 公开数据.
	ClassificationPublic DataClassification = "Public"
	// ClassificationInternal 内部数据.
	ClassificationInternal DataClassification = "Internal"
	// ClassificationConfidential 机密数据.
	ClassificationConfidential DataClassification = "Confidential"
	// ClassificationRestricted 受限数据.
	ClassificationRestricted DataClassification = "Restricted"
	// ClassificationPII 个人身份信息.
	ClassificationPII DataClassification = "PII"
	// ClassificationPHI 个人健康信息.
	ClassificationPHI DataClassification = "PHI"
)

// validClassifications 有效的数据分类集合.
var validClassifications = map[DataClassification]bool{
	ClassificationPublic:       true,
	ClassificationInternal:     true,
	ClassificationConfidential: true,
	ClassificationRestricted:   true,
	ClassificationPII:          true,
	ClassificationPHI:          true,
}

// ========== 事件状态 ==========

// BreachStatus 泄露事件状态.
type BreachStatus string

const (
	// StatusReported 已报告.
	StatusReported BreachStatus = "Reported"
	// StatusInvestigating 调查中.
	StatusInvestigating BreachStatus = "Investigating"
	// StatusContained 已遏制.
	StatusContained BreachStatus = "Contained"
	// StatusResolved 已解决.
	StatusResolved BreachStatus = "Resolved"
	// StatusClosed 已关闭.
	StatusClosed BreachStatus = "Closed"
)

// validStatuses 有效的状态集合.
var validStatuses = map[BreachStatus]bool{
	StatusReported:      true,
	StatusInvestigating: true,
	StatusContained:     true,
	StatusResolved:      true,
	StatusClosed:        true,
}

// ========== 通知方式 ==========

// NotificationMethod 通知方式.
type NotificationMethod string

const (
	// MethodEmail 邮件通知.
	MethodEmail NotificationMethod = "Email"
	// MethodSMS 短信通知.
	MethodSMS NotificationMethod = "SMS"
	// MethodPhone 电话通知.
	MethodPhone NotificationMethod = "Phone"
	// MethodLetter 书面通知.
	MethodLetter NotificationMethod = "Letter"
	// MethodPortal 门户网站公告.
	MethodPortal NotificationMethod = "Portal"
)

// ========== 通知状态 ==========

// NotificationStatus 通知状态.
type NotificationStatus string

const (
	// NotifyStatusPending 待发送.
	NotifyStatusPending NotificationStatus = "Pending"
	// NotifyStatusSent 已发送.
	NotifyStatusSent NotificationStatus = "Sent"
	// NotifyStatusDelivered 已送达.
	NotifyStatusDelivered NotificationStatus = "Delivered"
	// NotifyStatusFailed 发送失败.
	NotifyStatusFailed NotificationStatus = "Failed"
)

// ========== 核心数据结构 ==========

// BreachIncident 数据泄露事件.
type BreachIncident struct {
	ID                string             `json:"id"`
	DiscoveredAt      time.Time          `json:"discovered_at"`
	ReportedAt        time.Time          `json:"reported_at"`
	BreachType        BreachType         `json:"breach_type"`
	ImpactScope       string             `json:"impact_scope"`
	Classification    DataClassification `json:"classification"`
	AffectedRecords   int                `json:"affected_records"`
	Status            BreachStatus       `json:"status"`
	Description       string             `json:"description,omitempty"`
	Source            string             `json:"source,omitempty"`
	AffectedSystems   []string           `json:"affected_systems,omitempty"`
	ContainmentAction string             `json:"containment_action,omitempty"`
	RootCause         string             `json:"root_cause,omitempty"`
	Remediation       string             `json:"remediation,omitempty"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// NotificationRecord 通知记录.
type NotificationRecord struct {
	ID         string             `json:"id"`
	BreachID   string             `json:"breach_id"`
	NotifiedAt time.Time          `json:"notified_at"`
	Recipient  string             `json:"recipient"`
	Method     NotificationMethod `json:"method"`
	Status     NotificationStatus `json:"status"`
	Content    string             `json:"content,omitempty"`
	AckAt      *time.Time         `json:"ack_at,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID          string     `json:"id"`
	BreachID    string     `json:"breach_id"`
	GeneratedAt time.Time  `json:"generated_at"`
	Deadline    time.Time  `json:"deadline"`
	Completed   bool       `json:"completed"`
	Summary     string     `json:"summary"`
	GDPR33      bool       `json:"gdpr_article_33"`
	GDPR34      bool       `json:"gdpr_article_34"`
	Details     string     `json:"details,omitempty"`
	SubmittedTo string     `json:"submitted_to,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

// notificationTimer 通知计时器.
type notificationTimer struct {
	StartedAt time.Time
	Deadline  time.Time
}

// Manager 数据泄露事件管理器.
type Manager struct {
	mu            sync.RWMutex
	incidents     map[string]*BreachIncident
	notifications map[string][]NotificationRecord
	reports       map[string]*ComplianceReport
	timers        map[string]*notificationTimer
	nextID        int
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		incidents:     make(map[string]*BreachIncident),
		notifications: make(map[string][]NotificationRecord),
		reports:       make(map[string]*ComplianceReport),
		timers:        make(map[string]*notificationTimer),
		nextID:        1,
	}
}

// ========== 核心函数 ==========

// ReportBreach 报告新的泄露事件.
func (m *Manager) ReportBreach(incident BreachIncident) (*BreachIncident, error) {
	if !validBreachTypes[incident.BreachType] {
		return nil, ErrInvalidBreachType
	}
	if !validClassifications[incident.Classification] {
		return nil, ErrInvalidClassification
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	incident.ID = fmt.Sprintf("BREACH-%06d", m.nextID)
	m.nextID++
	incident.ReportedAt = time.Now()
	incident.Status = StatusReported
	incident.UpdatedAt = time.Now()

	if incident.DiscoveredAt.IsZero() {
		incident.DiscoveredAt = incident.ReportedAt
	}

	m.incidents[incident.ID] = &incident
	return &incident, nil
}

// GetBreach 获取泄露事件详情.
func (m *Manager) GetBreach(id string) (*BreachIncident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	incident, ok := m.incidents[id]
	if !ok {
		return nil, ErrBreachNotFound
	}
	return incident, nil
}

// ListBreaches 按状态列出泄露事件（空字符串表示全部）.
func (m *Manager) ListBreaches(status string) ([]BreachIncident, error) {
	if status != "" && !validStatuses[BreachStatus(status)] {
		return nil, ErrInvalidStatus
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]BreachIncident, 0)
	for _, inc := range m.incidents {
		if status == "" || string(inc.Status) == status {
			result = append(result, *inc)
		}
	}
	return result, nil
}

// UpdateBreachStatus 更新泄露事件状态.
func (m *Manager) UpdateBreachStatus(id string, status string) error {
	if !validStatuses[BreachStatus(status)] {
		return ErrInvalidStatus
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	incident, ok := m.incidents[id]
	if !ok {
		return ErrBreachNotFound
	}

	incident.Status = BreachStatus(status)
	incident.UpdatedAt = time.Now()
	return nil
}

// StartNotificationTimer 启动72小时通知计时器（GDPR Article 33）.
func (m *Manager) StartNotificationTimer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.incidents[id]; !ok {
		return ErrBreachNotFound
	}

	if _, exists := m.timers[id]; exists {
		return ErrNotificationTimerAlreadyStarted
	}

	now := time.Now()
	m.timers[id] = &notificationTimer{
		StartedAt: now,
		Deadline:  now.Add(72 * time.Hour),
	}
	return nil
}

// GetNotificationDeadline 获取通知截止时间.
func (m *Manager) GetNotificationDeadline(id string) (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.incidents[id]; !ok {
		return time.Time{}, ErrBreachNotFound
	}

	timer, ok := m.timers[id]
	if !ok {
		return time.Time{}, ErrNotificationDeadlineNotSet
	}

	return timer.Deadline, nil
}

// AddNotification 添加通知记录.
func (m *Manager) AddNotification(id string, record NotificationRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.incidents[id]; !ok {
		return ErrBreachNotFound
	}

	record.ID = fmt.Sprintf("NOTIFY-%s-%03d", id, len(m.notifications[id])+1)
	record.BreachID = id
	if record.NotifiedAt.IsZero() {
		record.NotifiedAt = time.Now()
	}
	if record.Status == "" {
		record.Status = NotifyStatusPending
	}

	m.notifications[id] = append(m.notifications[id], record)
	return nil
}

// GetNotifications 获取泄露事件的通知记录.
func (m *Manager) GetNotifications(id string) ([]NotificationRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.incidents[id]; !ok {
		return nil, ErrBreachNotFound
	}

	records := m.notifications[id]
	if records == nil {
		return []NotificationRecord{}, nil
	}
	return records, nil
}

// GenerateComplianceReport 生成合规报告.
func (m *Manager) GenerateComplianceReport(id string) (*ComplianceReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	incident, ok := m.incidents[id]
	if !ok {
		return nil, ErrBreachNotFound
	}

	// 检查是否需要 GDPR Article 34（通知数据主体）
	needsGDPR34 := incident.Classification == ClassificationPII ||
		incident.Classification == ClassificationPHI ||
		incident.Classification == ClassificationRestricted

	report := &ComplianceReport{
		ID:          fmt.Sprintf("RPT-%s-%d", id, time.Now().Unix()),
		BreachID:    id,
		GeneratedAt: time.Now(),
		GDPR33:      true, // 通知监管机构
		GDPR34:      needsGDPR34,
		Summary:     fmt.Sprintf("泄露事件 %s 合规报告 - 类型: %s, 分类: %s, 影响记录: %d", id, incident.BreachType, incident.Classification, incident.AffectedRecords),
	}

	// 设置截止时间
	if timer, ok := m.timers[id]; ok {
		report.Deadline = timer.Deadline
	} else {
		report.Deadline = incident.DiscoveredAt.Add(72 * time.Hour)
	}

	// 检查通知完成情况
	notifications := m.notifications[id]
	allNotified := true
	for _, n := range notifications {
		if n.Status != NotifyStatusDelivered && n.Status != NotifyStatusSent {
			allNotified = false
			break
		}
	}

	report.Completed = allNotified && incident.Status == StatusResolved

	m.reports[report.ID] = report
	return report, nil
}

// CalculateRiskScore 计算风险评分（1-100）.
func (m *Manager) CalculateRiskScore(id string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	incident, ok := m.incidents[id]
	if !ok {
		return 0, ErrBreachNotFound
	}

	// 泄露类型权重（0-40分）
	typeScore := 0
	switch incident.BreachType {
	case BreachTypeRansomware:
		typeScore = 40
	case BreachTypeUnauthorizedAccess:
		typeScore = 35
	case BreachTypeInsiderThreat:
		typeScore = 30
	case BreachTypeDataLeak:
		typeScore = 25
	case BreachTypeThirdParty:
		typeScore = 20
	case BreachTypeMisconfig:
		typeScore = 15
	}

	// 数据分类权重（0-35分）
	classScore := 0
	switch incident.Classification {
	case ClassificationPHI:
		classScore = 35
	case ClassificationPII:
		classScore = 30
	case ClassificationRestricted:
		classScore = 25
	case ClassificationConfidential:
		classScore = 20
	case ClassificationInternal:
		classScore = 10
	case ClassificationPublic:
		classScore = 5
	}

	// 受影响记录数权重（0-25分）
	recordScore := 0
	records := incident.AffectedRecords
	switch {
	case records > 1000000:
		recordScore = 25
	case records > 100000:
		recordScore = 20
	case records > 10000:
		recordScore = 15
	case records > 1000:
		recordScore = 10
	case records > 100:
		recordScore = 5
	default:
		recordScore = 1
	}

	score := typeScore + classScore + recordScore
	// 确保在 1-100 范围内
	score = int(math.Max(1, math.Min(100, float64(score))))

	return score, nil
}

// GetBreachStatistics 获取泄露事件统计.
func (m *Manager) GetBreachStatistics() (map[string]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]int)

	// 按状态统计
	stats["total"] = len(m.incidents)
	for _, inc := range m.incidents {
		stats[string(inc.Status)]++
		stats[string(inc.BreachType)]++
	}

	return stats, nil
}

// ExportGDPRReport 导出GDPR Article 33/34报告（JSON格式）.
func (m *Manager) ExportGDPRReport(id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	incident, ok := m.incidents[id]
	if !ok {
		return nil, ErrBreachNotFound
	}

	// 构建GDPR报告
	type GDPRReport struct {
		ReportType        string               `json:"report_type"`
		IncidentID        string               `json:"incident_id"`
		DiscoveredAt      time.Time            `json:"discovered_at"`
		ReportedAt        time.Time            `json:"reported_at"`
		BreachType        BreachType           `json:"breach_type"`
		Classification    DataClassification   `json:"classification"`
		AffectedRecords   int                  `json:"affected_records"`
		ImpactScope       string               `json:"impact_scope"`
		Description       string               `json:"description"`
		ContainmentAction string               `json:"containment_action"`
		RootCause         string               `json:"root_cause"`
		Remediation       string               `json:"remediation"`
		Notifications     []NotificationRecord `json:"notifications"`
		Deadline          time.Time            `json:"notification_deadline"`
		Status            BreachStatus         `json:"status"`
	}

	needsGDPR34 := incident.Classification == ClassificationPII ||
		incident.Classification == ClassificationPHI ||
		incident.Classification == ClassificationRestricted

	reportType := "GDPR Article 33 - 监管机构通知"
	if needsGDPR34 {
		reportType = "GDPR Article 33/34 - 监管机构及数据主体通知"
	}

	deadline := time.Time{}
	if timer, ok := m.timers[id]; ok {
		deadline = timer.Deadline
	}

	report := GDPRReport{
		ReportType:        reportType,
		IncidentID:        incident.ID,
		DiscoveredAt:      incident.DiscoveredAt,
		ReportedAt:        incident.ReportedAt,
		BreachType:        incident.BreachType,
		Classification:    incident.Classification,
		AffectedRecords:   incident.AffectedRecords,
		ImpactScope:       incident.ImpactScope,
		Description:       incident.Description,
		ContainmentAction: incident.ContainmentAction,
		RootCause:         incident.RootCause,
		Remediation:       incident.Remediation,
		Notifications:     m.notifications[id],
		Deadline:          deadline,
		Status:            incident.Status,
	}

	return json.MarshalIndent(report, "", "  ")
}
