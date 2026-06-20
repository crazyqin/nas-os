package auditcomplianceengine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ComplianceEngine 审计合规引擎
type ComplianceEngine struct {
	mu         sync.RWMutex
	frameworks map[string]*ComplianceFramework
	controls   map[string]*Control
	findings   map[string]*Finding
	auditLogs  []*AuditEntry
	reports    map[string]*ComplianceReport
	assessor   *ComplianceAssessor
	metrics    *ComplianceMetrics
	config     *EngineConfig
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

// ComplianceFramework 合规框架
type ComplianceFramework struct {
	ID        string
	Name      string
	Type      FrameworkType
	Version   string
	Controls  []string
	Score     float64
	Status    ComplianceStatus
	LastAudit time.Time
	NextAudit time.Time
}

// Control 合规控制
type Control struct {
	ID          string
	FrameworkID string
	Name        string
	Description string
	Category    string
	Status      ControlStatus
	Evidence    []*Evidence
	Tests       []*ControlTest
	LastTested  time.Time
	Score       float64
}

// Evidence 证据
type Evidence struct {
	ID          string
	ControlID   string
	Type        EvidenceType
	Description string
	FilePath    string
	Hash        string
	CollectedAt time.Time
	ValidUntil  time.Time
}

// ControlTest 控制测试
type ControlTest struct {
	ID        string
	ControlID string
	Name      string
	Type      TestType
	Status    TestStatus
	Result    string
	Automated bool
	LastRun   time.Time
	NextRun   time.Time
}

// Finding 发现
type Finding struct {
	ID          string
	ControlID   string
	Severity    FindingSeverity
	Title       string
	Description string
	Evidence    string
	Remediation string
	Status      FindingStatus
	AssignedTo  string
	DueDate     time.Time
	FoundAt     time.Time
	ResolvedAt  time.Time
}

// AuditEntry 审计日志
type AuditEntry struct {
	ID           string
	Timestamp    time.Time
	EventType    EventType
	Actor        string
	ActorType    ActorType
	Resource     string
	ResourceType string
	Action       string
	Result       ActionResult
	Details      map[string]interface{}
	IPAddress    string
	UserAgent    string
	SessionID    string
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID          string
	FrameworkID string
	Type        ReportType
	Period      *ReportPeriod
	Score       float64
	Summary     *ReportSummary
	Findings    []string
	GeneratedAt time.Time
	PublishedAt time.Time
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	Start time.Time
	End   time.Time
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalControls    int
	Passed           int
	Failed           int
	NotApplicable    int
	CriticalFindings int
	HighFindings     int
	MediumFindings   int
	LowFindings      int
}

// ComplianceAssessor 合规评估器
type ComplianceAssessor struct {
	mu          sync.RWMutex
	rules       map[string]*AssessmentRule
	accuracy    float64
	assessments int64
}

// AssessmentRule 评估规则
type AssessmentRule struct {
	ID        string
	Framework string
	Control   string
	Condition string
	Threshold float64
	Automated bool
}

// ComplianceMetrics 合规指标
type ComplianceMetrics struct {
	OverallScore    float64
	FrameworkScores map[string]float64
	OpenFindings    int
	ResolvedFindings int
	AuditTrailSize  int
	ComplianceRate  float64
	AutomationRate  float64
}

// EngineConfig 引擎配置
type EngineConfig struct {
	AuditRetention     time.Duration
	AutoAssess         bool
	AlertOnViolation   bool
	DataRegion         string
	EncryptionRequired bool
}

// 枚举类型
type FrameworkType int

const (
	FrameworkSOC2 FrameworkType = iota
	FrameworkISO27001
	FrameworkGDPR
	FrameworkHIPAA
	FrameworkPCIDSS
	FrameworkCustom
)

type ComplianceStatus int

const (
	ComplianceCompliant ComplianceStatus = iota
	CompliancePartial
	ComplianceNonCompliant
	ComplianceUnknown
)

type ControlStatus int

const (
	ControlImplemented ControlStatus = iota
	ControlPartial
	ControlNotImplemented
	ControlNotApplicable
)

type EvidenceType int

const (
	EvidenceDocument EvidenceType = iota
	EvidenceScreenshot
	EvidenceLog
	EvidenceConfig
	EvidenceTest
)

type TestType int

const (
	TestAutomated TestType = iota
	TestManual
	TestHybrid
)

type TestStatus int

const (
	TestPass TestStatus = iota
	TestFail
	TestSkip
	TestError
)

type FindingSeverity int

const (
	FindingLow FindingSeverity = iota
	FindingMedium
	FindingHigh
	FindingCritical
)

type FindingStatus int

const (
	FindingOpen FindingStatus = iota
	FindingInProgress
	FindingResolved
	FindingAccepted
	FindingFalsePositive
)

type EventType int

const (
	EventAccess EventType = iota
	EventModify
	EventDelete
	EventCreate
	EventConfig
	EventAuth
	EventAdmin
)

type ActorType int

const (
	ActorUser ActorType = iota
	ActorSystem
	ActorAPI
	ActorService
)

type ActionResult int

const (
	ActionSuccess ActionResult = iota
	ActionFailure
	ActionDenied
)

type ReportType int

const (
	ReportTypeAudit ReportType = iota
	ReportTypeCompliance
	ReportTypeExecutive
	ReportTypeTechnical
)

// NewComplianceEngine 创建引擎
func NewComplianceEngine(config *EngineConfig, logger *slog.Logger) *ComplianceEngine {
	ctx, cancel := context.WithCancel(context.Background())

	if config == nil {
		config = &EngineConfig{
			AuditRetention: 365 * 24 * time.Hour,
			AutoAssess:     true,
			DataRegion:     "default",
		}
	}

	if logger == nil {
		logger = slog.Default()
	}

	engine := &ComplianceEngine{
		frameworks: make(map[string]*ComplianceFramework),
		controls:   make(map[string]*Control),
		findings:   make(map[string]*Finding),
		reports:    make(map[string]*ComplianceReport),
		assessor: &ComplianceAssessor{
			rules:    make(map[string]*AssessmentRule),
			accuracy: 1.0,
		},
		metrics: &ComplianceMetrics{
			FrameworkScores: make(map[string]float64),
		},
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	engine.logger.Info("审计合规引擎已初始化",
		"audit_retention", config.AuditRetention,
		"auto_assess", config.AutoAssess,
		"data_region", config.DataRegion,
	)

	return engine
}

// RegisterFramework 注册合规框架
func (e *ComplianceEngine) RegisterFramework(framework *ComplianceFramework) error {
	if framework == nil {
		return ErrNilFramework
	}

	if framework.ID == "" {
		return ErrInvalidFrameworkID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.frameworks[framework.ID]; exists {
		return ErrFrameworkAlreadyExists
	}

	framework.Status = ComplianceUnknown
	framework.Score = 0
	e.frameworks[framework.ID] = framework

	e.logger.Info("合规框架已注册",
		"framework_id", framework.ID,
		"name", framework.Name,
		"type", framework.Type,
	)

	return nil
}

// AddControl 添加控制项
func (e *ComplianceEngine) AddControl(control *Control) error {
	if control == nil {
		return ErrNilControl
	}

	if control.ID == "" {
		return ErrInvalidControlID
	}

	if control.FrameworkID == "" {
		return ErrInvalidFrameworkID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	framework, exists := e.frameworks[control.FrameworkID]
	if !exists {
		return ErrFrameworkNotFound
	}

	if _, exists := e.controls[control.ID]; exists {
		return ErrControlAlreadyExists
	}

	control.Status = ControlNotImplemented
	control.Score = 0
	control.Evidence = make([]*Evidence, 0)
	control.Tests = make([]*ControlTest, 0)
	e.controls[control.ID] = control

	framework.Controls = append(framework.Controls, control.ID)

	e.logger.Info("控制项已添加",
		"control_id", control.ID,
		"framework_id", control.FrameworkID,
		"name", control.Name,
	)

	return nil
}

// RecordAuditEvent 记录审计事件
func (e *ComplianceEngine) RecordAuditEvent(entry *AuditEntry) error {
	if entry == nil {
		return ErrNilAuditEntry
	}

	entry.Timestamp = time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	entry.ID = e.generateAuditID()
	e.auditLogs = append(e.auditLogs, entry)

	e.metrics.AuditTrailSize = len(e.auditLogs)

	e.logger.Debug("审计事件已记录",
		"entry_id", entry.ID,
		"event_type", entry.EventType,
		"actor", entry.Actor,
		"action", entry.Action,
	)

	return nil
}

// RunAssessment 运行合规评估
func (e *ComplianceEngine) RunAssessment(frameworkID string) (*ComplianceReport, error) {
	e.mu.RLock()
	framework, exists := e.frameworks[frameworkID]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrFrameworkNotFound
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	totalControls := len(framework.Controls)
	passed := 0
	failed := 0
	notApplicable := 0
	criticalFindings := 0
	highFindings := 0
	mediumFindings := 0
	lowFindings := 0
	totalScore := 0.0

	for _, controlID := range framework.Controls {
		control, exists := e.controls[controlID]
		if !exists {
			continue
		}

		control.LastTested = now

		switch control.Status {
		case ControlImplemented:
			passed++
			control.Score = 100.0
		case ControlPartial:
			control.Score = 50.0
		case ControlNotImplemented:
			failed++
			control.Score = 0
		case ControlNotApplicable:
			notApplicable++
			control.Score = 0
		}

		totalScore += control.Score

		for _, findingID := range e.getFindingsByControl(controlID) {
			finding := e.findings[findingID]
			switch finding.Severity {
			case FindingCritical:
				criticalFindings++
			case FindingHigh:
				highFindings++
			case FindingMedium:
				mediumFindings++
			case FindingLow:
				lowFindings++
			}
		}
	}

	score := 0.0
	if totalControls-notApplicable > 0 {
		score = totalScore / float64(totalControls-notApplicable)
	}

	framework.Score = score
	framework.LastAudit = now
	framework.NextAudit = now.Add(30 * 24 * time.Hour)

	if score >= 90 {
		framework.Status = ComplianceCompliant
	} else if score >= 60 {
		framework.Status = CompliancePartial
	} else {
		framework.Status = ComplianceNonCompliant
	}

	reportID := e.generateReportID()
	report := &ComplianceReport{
		ID:          reportID,
		FrameworkID: frameworkID,
		Type:        ReportTypeCompliance,
		Period: &ReportPeriod{
			Start: now.Add(-30 * 24 * time.Hour),
			End:   now,
		},
		Score: score,
		Summary: &ReportSummary{
			TotalControls:    totalControls,
			Passed:           passed,
			Failed:           failed,
			NotApplicable:    notApplicable,
			CriticalFindings: criticalFindings,
			HighFindings:     highFindings,
			MediumFindings:   mediumFindings,
			LowFindings:      lowFindings,
		},
		Findings:    e.getFrameworkFindings(frameworkID),
		GeneratedAt: now,
	}

	e.reports[reportID] = report
	e.updateMetrics()

	e.logger.Info("合规评估完成",
		"framework_id", frameworkID,
		"score", score,
		"total_controls", totalControls,
	)

	return report, nil
}

// CreateFinding 创建发现
func (e *ComplianceEngine) CreateFinding(finding *Finding) error {
	if finding == nil {
		return ErrNilFinding
	}

	if finding.ControlID == "" {
		return ErrInvalidControlID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	control, exists := e.controls[finding.ControlID]
	if !exists {
		return ErrControlNotFound
	}

	finding.ID = e.generateFindingID()
	finding.Status = FindingOpen
	finding.FoundAt = time.Now()

	if finding.DueDate.IsZero() {
		finding.DueDate = time.Now().Add(30 * 24 * time.Hour)
	}

	e.findings[finding.ID] = finding

	control.Status = ControlPartial
	control.Score = 50.0

	e.logger.Warn("发现已创建",
		"finding_id", finding.ID,
		"control_id", finding.ControlID,
		"severity", finding.Severity,
		"title", finding.Title,
	)

	return nil
}

// ResolveFinding 解决发现
func (e *ComplianceEngine) ResolveFinding(findingID string, resolution string) error {
	if findingID == "" {
		return ErrInvalidFindingID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	finding, exists := e.findings[findingID]
	if !exists {
		return ErrFindingNotFound
	}

	if finding.Status == FindingResolved || finding.Status == FindingFalsePositive {
		return ErrFindingAlreadyResolved
	}

	finding.Status = FindingResolved
	finding.ResolvedAt = time.Now()

	e.logger.Info("发现已解决",
		"finding_id", findingID,
		"control_id", finding.ControlID,
		"resolution", resolution,
	)

	control, exists := e.controls[finding.ControlID]
	if exists {
		controlFindings := e.getOpenFindingsByControl(finding.ControlID)
		if len(controlFindings) == 0 {
			control.Status = ControlImplemented
			control.Score = 100.0
		}
	}

	e.updateMetrics()

	return nil
}

// GenerateReport 生成合规报告
func (e *ComplianceEngine) GenerateReport(frameworkID string, reportType ReportType) (*ComplianceReport, error) {
	e.mu.RLock()
	framework, exists := e.frameworks[frameworkID]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrFrameworkNotFound
	}

	report, err := e.RunAssessment(frameworkID)
	if err != nil {
		return nil, err
	}

	report.Type = reportType
	report.PublishedAt = time.Now()

	e.logger.Info("合规报告已生成",
		"report_id", report.ID,
		"framework_id", frameworkID,
		"framework_name", framework.Name,
		"type", reportType,
		"score", report.Score,
	)

	return report, nil
}

// SearchAuditLogs 搜索审计日志
func (e *ComplianceEngine) SearchAuditLogs(filter *AuditLogFilter) ([]*AuditEntry, error) {
	if filter == nil {
		return nil, ErrNilFilter
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*AuditEntry

	for _, entry := range e.auditLogs {
		if !e.matchesFilter(entry, filter) {
			continue
		}
		results = append(results, entry)
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	e.logger.Debug("审计日志搜索完成",
		"results_count", len(results),
	)

	return results, nil
}

// GetMetrics 获取指标
func (e *ComplianceEngine) GetMetrics() *ComplianceMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.updateMetrics()

	return &ComplianceMetrics{
		OverallScore:     e.metrics.OverallScore,
		FrameworkScores:  copyMap(e.metrics.FrameworkScores),
		OpenFindings:     e.metrics.OpenFindings,
		ResolvedFindings: e.metrics.ResolvedFindings,
		AuditTrailSize:   e.metrics.AuditTrailSize,
		ComplianceRate:   e.metrics.ComplianceRate,
		AutomationRate:   e.metrics.AutomationRate,
	}
}

// Shutdown 关闭引擎
func (e *ComplianceEngine) Shutdown() {
	e.cancel()
	e.logger.Info("审计合规引擎已关闭")
}

// 内部辅助方法

func (e *ComplianceEngine) generateAuditID() string {
	return fmt.Sprintf("audit_%d_%s", time.Now().UnixNano(), randomHex(8))
}

func (e *ComplianceEngine) generateFindingID() string {
	return fmt.Sprintf("find_%d_%s", time.Now().UnixNano(), randomHex(8))
}

func (e *ComplianceEngine) generateReportID() string {
	return fmt.Sprintf("rpt_%d_%s", time.Now().UnixNano(), randomHex(8))
}

func randomHex(n int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return fmt.Sprintf("%x", h[:n])
}

func (e *ComplianceEngine) getFindingsByControl(controlID string) []string {
	var ids []string
	for id, finding := range e.findings {
		if finding.ControlID == controlID {
			ids = append(ids, id)
		}
	}
	return ids
}

func (e *ComplianceEngine) getOpenFindingsByControl(controlID string) []string {
	var ids []string
	for id, finding := range e.findings {
		if finding.ControlID == controlID && finding.Status == FindingOpen {
			ids = append(ids, id)
		}
	}
	return ids
}

func (e *ComplianceEngine) getFrameworkFindings(frameworkID string) []string {
	var ids []string
	framework, exists := e.frameworks[frameworkID]
	if !exists {
		return ids
	}

	for _, controlID := range framework.Controls {
		control, exists := e.controls[controlID]
		if !exists {
			continue
		}

		for id, finding := range e.findings {
			if finding.ControlID == control.ID {
				ids = append(ids, id)
			}
		}
	}

	return ids
}

func (e *ComplianceEngine) matchesFilter(entry *AuditEntry, filter *AuditLogFilter) bool {
	if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {
		return false
	}

	if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {
		return false
	}

	if filter.EventType >= 0 && entry.EventType != filter.EventType {
		return false
	}

	if filter.Actor != "" && entry.Actor != filter.Actor {
		return false
	}

	if filter.ActorType >= 0 && entry.ActorType != filter.ActorType {
		return false
	}

	if filter.Resource != "" && entry.Resource != filter.Resource {
		return false
	}

	if filter.ResourceType != "" && entry.ResourceType != filter.ResourceType {
		return false
	}

	if filter.Result >= 0 && entry.Result != filter.Result {
		return false
	}

	if filter.IPAddress != "" && entry.IPAddress != filter.IPAddress {
		return false
	}

	return true
}

func (e *ComplianceEngine) updateMetrics() {
	totalScore := 0.0
	totalFrameworks := 0
	openFindings := 0
	resolvedFindings := 0
	automatedTests := 0
	totalTests := 0

	for _, framework := range e.frameworks {
		totalScore += framework.Score
		totalFrameworks++
		e.metrics.FrameworkScores[framework.ID] = framework.Score
	}

	if totalFrameworks > 0 {
		e.metrics.OverallScore = totalScore / float64(totalFrameworks)
	}

	for _, finding := range e.findings {
		if finding.Status == FindingOpen || finding.Status == FindingInProgress {
			openFindings++
		} else if finding.Status == FindingResolved || finding.Status == FindingFalsePositive {
			resolvedFindings++
		}
	}

	e.metrics.OpenFindings = openFindings
	e.metrics.ResolvedFindings = resolvedFindings
	e.metrics.AuditTrailSize = len(e.auditLogs)

	for _, control := range e.controls {
		for _, test := range control.Tests {
			totalTests++
			if test.Automated {
				automatedTests++
			}
		}
	}

	if totalTests > 0 {
		e.metrics.AutomationRate = float64(automatedTests) / float64(totalTests) * 100
	}

	totalFindings := openFindings + resolvedFindings
	if totalFindings > 0 {
		e.metrics.ComplianceRate = float64(resolvedFindings) / float64(totalFindings) * 100
	}
}

func copyMap(m map[string]float64) map[string]float64 {
	result := make(map[string]float64)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// AuditLogFilter 审计日志过滤器
type AuditLogFilter struct {
	StartTime    time.Time
	EndTime      time.Time
	EventType    EventType
	Actor        string
	ActorType    ActorType
	Resource     string
	ResourceType string
	Result       ActionResult
	IPAddress    string
	Limit        int
}
