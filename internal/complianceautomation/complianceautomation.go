// Package complianceautomation 提供自动化合规检查功能
// 支持GDPR、等保2.0、ISO27001等标准的自动化审计和报告
// 参考TrueNAS的企业合规特性和群晖的安全顾问
package complianceautomation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 合规标准
const (
	StandardGDPR     = "GDPR"       // 通用数据保护条例
	StandardMLPS2    = "MLPS2"      // 等保2.0
	StandardISO27001 = "ISO27001"   // ISO 27001
	StandardSOC2     = "SOC2"       // SOC 2
	StandardHIPAA    = "HIPAA"      // HIPAA
	StandardPCI      = "PCI-DSS"    // PCI DSS
)

// 检查状态
const (
	CheckStatusPass    = "pass"    // 通过
	CheckStatusFail    = "fail"    // 不通过
	CheckStatusWarn    = "warn"    // 警告
	CheckStatusSkip    = "skip"    // 跳过
	CheckStatusPending = "pending" // 待检查
)

// 严重级别
const (
	SeverityCritical = "critical" // 严重
	SeverityHigh     = "high"     // 高
	SeverityMedium   = "medium"   // 中
	SeverityLow      = "low"      // 低
	SeverityInfo     = "info"     // 信息
)

var (
	ErrStandardNotFound = errors.New("合规标准不存在")
	ErrCheckNotFound    = errors.New("检查项不存在")
	ErrAuditRunning     = errors.New("审计正在进行中")
)

// ComplianceCheck 合规检查项
type ComplianceCheck struct {
	ID          string   `json:"id"`          // 检查ID
	Standard    string   `json:"standard"`    // 合规标准
	Category    string   `json:"category"`    // 分类
	Name        string   `json:"name"`        // 检查名称
	Description string   `json:"description"` // 描述
	Severity    string   `json:"severity"`    // 严重级别
	Status      string   `json:"status"`      // 检查状态
	Remediation string   `json:"remediation"` // 修复建议
	Evidence    string   `json:"evidence"`    // 证据
	LastChecked time.Time `json:"last_checked"`
}

// AuditTask 审计任务
type AuditTask struct {
	ID          string    `json:"id"`           // 任务ID
	Standard    string    `json:"standard"`     // 合规标准
	Status      string    `json:"status"`       // 状态
	Progress    float64   `json:"progress"`     // 进度
	TotalChecks int       `json:"total_checks"` // 总检查数
	Passed      int       `json:"passed"`       // 通过数
	Failed      int       `json:"failed"`       // 失败数
	Warnings    int       `json:"warnings"`     // 警告数
	Skipped     int       `json:"skipped"`      // 跳过数
	Score       float64   `json:"score"`        // 合规评分（0-100）
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID           string           `json:"id"`
	GeneratedAt  time.Time        `json:"generated_at"`
	Standard     string           `json:"standard"`
	Score        float64          `json:"score"`
	Summary      *AuditTask       `json:"summary"`
	Checks       []*ComplianceCheck `json:"checks"`
	Gaps         []*ComplianceCheck `json:"gaps"`         // 不合规项
	Trends       []*TrendPoint    `json:"trends"`       // 趋势
	Recommendations []string      `json:"recommendations"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Date  time.Time `json:"date"`
	Score float64   `json:"score"`
}

// ComplianceEngine 合规引擎
type ComplianceEngine struct {
	mu          sync.RWMutex
	checks      map[string]*ComplianceCheck
	standards   map[string][]string // standard -> check IDs
	audits      []*AuditTask
	reports     []*ComplianceReport
	history     map[string][]*TrendPoint // standard -> trends
	auditCounter int64
}

// NewComplianceEngine 创建合规引擎
func NewComplianceEngine() *ComplianceEngine {
	e := &ComplianceEngine{
		checks:    make(map[string]*ComplianceCheck),
		standards: make(map[string][]string),
		audits:    make([]*AuditTask, 0),
		reports:   make([]*ComplianceReport, 0),
		history:   make(map[string][]*TrendPoint),
	}
	e.initDefaultChecks()
	return e
}

// initDefaultChecks 初始化默认检查项
func (e *ComplianceEngine) initDefaultChecks() {
	defaults := []*ComplianceCheck{
		// GDPR
		{ID: "gdpr-001", Standard: StandardGDPR, Category: "数据保护", Name: "数据加密存储", Description: "个人数据应加密存储", Severity: SeverityCritical},
		{ID: "gdpr-002", Standard: StandardGDPR, Category: "数据保护", Name: "数据访问审计", Description: "应记录所有个人数据访问", Severity: SeverityHigh},
		{ID: "gdpr-003", Standard: StandardGDPR, Category: "数据保护", Name: "数据删除权", Description: "支持用户请求删除个人数据", Severity: SeverityHigh},
		{ID: "gdpr-004", Standard: StandardGDPR, Category: "数据保护", Name: "数据可携带性", Description: "支持导出用户个人数据", Severity: SeverityMedium},
		{ID: "gdpr-005", Standard: StandardGDPR, Category: "安全", Name: "访问控制", Description: "实施最小权限访问控制", Severity: SeverityCritical},
		// 等保2.0
		{ID: "mlps2-001", Standard: StandardMLPS2, Category: "安全通信网络", Name: "网络架构安全", Description: "网络架构应划分安全域", Severity: SeverityCritical},
		{ID: "mlps2-002", Standard: StandardMLPS2, Category: "安全通信网络", Name: "通信传输安全", Description: "通信传输应加密", Severity: SeverityHigh},
		{ID: "mlps2-003", Standard: StandardMLPS2, Category: "安全区域边界", Name: "边界防护", Description: "应部署防火墙等边界防护设备", Severity: SeverityCritical},
		{ID: "mlps2-004", Standard: StandardMLPS2, Category: "安全计算环境", Name: "身份鉴别", Description: "应采用两种以上鉴别技术", Severity: SeverityCritical},
		{ID: "mlps2-005", Standard: StandardMLPS2, Category: "安全计算环境", Name: "入侵防范", Description: "应能检测和防范入侵行为", Severity: SeverityHigh},
		// ISO27001
		{ID: "iso-001", Standard: StandardISO27001, Category: "访问控制", Name: "用户注册管理", Description: "应有正式的用户注册和注销流程", Severity: SeverityHigh},
		{ID: "iso-002", Standard: StandardISO27001, Category: "访问控制", Name: "特权管理", Description: "应限制和控制特权访问", Severity: SeverityCritical},
		{ID: "iso-003", Standard: StandardISO27001, Category: "密码学", Name: "加密控制", Description: "应制定和实施加密控制策略", Severity: SeverityHigh},
		{ID: "iso-004", Standard: StandardISO27001, Category: "运维安全", Name: "变更管理", Description: "应有正式的变更管理流程", Severity: SeverityMedium},
		{ID: "iso-005", Standard: StandardISO27001, Category: "业务连续性", Name: "备份", Description: "应定期备份信息和软件", Severity: SeverityCritical},
	}

	for _, check := range defaults {
		check.Status = CheckStatusPending
		check.LastChecked = time.Time{}
		e.checks[check.ID] = check
		e.standards[check.Standard] = append(e.standards[check.Standard], check.ID)
	}
}

// UpdateCheckResult 更新检查结果
func (e *ComplianceEngine) UpdateCheckResult(checkID, status, evidence, remediation string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	check, ok := e.checks[checkID]
	if !ok {
		return ErrCheckNotFound
	}
	check.Status = status
	check.Evidence = evidence
	check.Remediation = remediation
	check.LastChecked = time.Now()
	return nil
}

// RunAudit 运行合规审计
func (e *ComplianceEngine) RunAudit(standard string) (*AuditTask, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	checkIDs, ok := e.standards[standard]
	if !ok {
		return nil, ErrStandardNotFound
	}

	e.auditCounter++
	task := &AuditTask{
		ID:          fmt.Sprintf("audit-%d", e.auditCounter),
		Standard:    standard,
		Status:      "running",
		TotalChecks: len(checkIDs),
		StartedAt:   time.Now(),
	}

	passed, failed, warnings, skipped := 0, 0, 0, 0
	for _, id := range checkIDs {
		check := e.checks[id]
		switch check.Status {
		case CheckStatusPass:
			passed++
		case CheckStatusFail:
			failed++
		case CheckStatusWarn:
			warnings++
		default:
			skipped++
		}
	}

	task.Passed = passed
	task.Failed = failed
	task.Warnings = warnings
	task.Skipped = skipped
	task.Progress = 100.0
	task.Status = "completed"
	task.CompletedAt = time.Now()

	if task.TotalChecks > 0 {
		task.Score = float64(passed) / float64(task.TotalChecks) * 100
	}

	e.audits = append(e.audits, task)

	// 记录趋势
	e.history[standard] = append(e.history[standard], &TrendPoint{
		Date:  time.Now(),
		Score: task.Score,
	})

	return task, nil
}

// GenerateReport 生成合规报告
func (e *ComplianceEngine) GenerateReport(standard string) (*ComplianceReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	checkIDs, ok := e.standards[standard]
	if !ok {
		return nil, ErrStandardNotFound
	}

	report := &ComplianceReport{
		ID:          fmt.Sprintf("rpt-%s-%d", standard, time.Now().Unix()),
		GeneratedAt: time.Now(),
		Standard:    standard,
		Checks:      make([]*ComplianceCheck, 0),
		Gaps:        make([]*ComplianceCheck, 0),
		Trends:      e.history[standard],
	}

	var totalScore float64
	for _, id := range checkIDs {
		check := e.checks[id]
		report.Checks = append(report.Checks, check)
		if check.Status == CheckStatusFail {
			gap := *check
			report.Gaps = append(report.Gaps, &gap)
		}
		if check.Status == CheckStatusPass {
			totalScore += 100
		} else if check.Status == CheckStatusWarn {
			totalScore += 50
		}
	}

	if len(checkIDs) > 0 {
		report.Score = totalScore / float64(len(checkIDs))
	}

	// 获取最新审计任务
	for i := len(e.audits) - 1; i >= 0; i-- {
		if e.audits[i].Standard == standard {
			report.Summary = e.audits[i]
			break
		}
	}

	// 生成建议
	report.Recommendations = make([]string, 0)
	for _, gap := range report.Gaps {
		if gap.Remediation != "" {
			report.Recommendations = append(report.Recommendations, gap.Remediation)
		}
	}

	e.reports = append(e.reports, report)
	return report, nil
}

// ListChecks 列出检查项
func (e *ComplianceEngine) ListChecks(standard string) []*ComplianceCheck {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*ComplianceCheck, 0)
	if standard == "" {
		for _, check := range e.checks {
			result = append(result, check)
		}
	} else {
		for _, id := range e.standards[standard] {
			result = append(result, e.checks[id])
		}
	}
	return result
}

// ExportReport 导出报告为JSON
func (e *ComplianceEngine) ExportReport(report *ComplianceReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
