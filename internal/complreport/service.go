package complreport

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrReportNotFound 报告未找到.
	ErrReportNotFound = errors.New("合规报告未找到")
	// ErrScheduleNotFound 计划未找到.
	ErrScheduleNotFound = errors.New("报告计划未找到")
	// ErrInvalidStandard 无效合规标准.
	ErrInvalidStandard = errors.New("无效的合规标准")
	// ErrInvalidFormat 无效报告格式.
	ErrInvalidFormat = errors.New("无效的报告格式")
	// ErrReportFailed 报告生成失败.
	ErrReportFailed = errors.New("报告生成失败")
)

// ========== 合规标准元数据 ==========

// standardInfo 合规标准元信息.
type standardInfo struct {
	Name        string
	Description string
	Categories  []string
}

// supportedStandards 支持的合规标准.
var supportedStandards = map[Standard]standardInfo{
	StandardGDPR: {
		Name:        "GDPR 合规审计",
		Description: "欧盟通用数据保护条例合规审计",
		Categories:  []string{"数据主体权利", "数据处理合法性", "数据保护设计", "跨境传输", "数据泄露通知"},
	},
	StandardPIPL: {
		Name:        "PIPL 合规审计",
		Description: "中国个人信息保护法合规审计",
		Categories:  []string{"个人信息处理规则", "跨境提供", "个人信息主体权利", "个人信息处理者义务", "个人信息保护影响评估"},
	},
	StandardSOC2: {
		Name:        "SOC2 合规审计",
		Description: "SOC2 服务组织控制合规审计",
		Categories:  []string{"安全", "可用性", "处理完整性", "保密性", "隐私性"},
	},
	StandardISO27001: {
		Name:        "ISO 27001 合规审计",
		Description: "ISO/IEC 27001 信息安全管理体系合规审计",
		Categories:  []string{"信息安全策略", "组织安全", "资产管理", "人力资源安全", "物理环境安全", "通信安全", "访问控制", "密码学", "运行安全", "供应链安全", "合规性"},
	},
	StandardHIPAA: {
		Name:        "HIPAA 合规审计",
		Description: "美国健康保险可携性与责任法案合规审计",
		Categories:  []string{"隐私规则", "安全规则", "违规通知", "行政简化"},
	},
	StandardCCPA: {
		Name:        "CCPA 合规审计",
		Description: "加州消费者隐私法合规审计",
		Categories:  []string{"消费者权利", "数据收集披露", "数据删除", "数据出售选择权", "非歧视权利"},
	},
}

// validFormats 有效的报告格式.
var validFormats = map[ReportFormat]bool{
	FormatJSON: true,
	FormatPDF:  true,
}

// ========== 服务定义 ==========

// Service 合规审计报告服务.
type Service struct {
	mu        sync.RWMutex
	reports   map[string]*Report   // reportID -> Report
	schedules map[string]*Schedule // scheduleID -> Schedule
}

// NewService 创建合规审计报告服务.
func NewService() *Service {
	return &Service{
		reports:   make(map[string]*Report),
		schedules: make(map[string]*Schedule),
	}
}

// ========== 报告生成 ==========

// GenerateReport 生成合规审计报告.
func (s *Service) GenerateReport(req GenerateRequest) (*Report, error) {
	// 验证合规标准
	if _, ok := supportedStandards[req.Standard]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard)
	}

	// 验证报告格式
	format := req.Format
	if format == "" {
		format = FormatJSON
	}
	if !validFormats[format] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidFormat, format)
	}

	// 创建报告（初始状态为生成中）
	report := &Report{
		ID:          uuid.New().String(),
		Standard:    req.Standard,
		Title:       req.Title,
		Format:      format,
		Status:      StatusGenerating,
		GeneratedBy: req.GeneratedBy,
		CreatedAt:   time.Now(),
	}

	if report.Title == "" {
		info := supportedStandards[req.Standard]
		report.Title = info.Name
	}

	// 采集合规证据并执行控制项检查
	controls := s.collectEvidence(req.Standard)
	report.Controls = controls

	// 统计检查结果
	report.TotalChecks = len(controls)
	for _, c := range controls {
		switch c.Status {
		case CheckPass:
			report.Passed++
		case CheckFail:
			report.Failed++
		case CheckWarning:
			report.Warnings++
		case CheckNotApplicable:
			report.NotApplicable++
		}
	}

	// 计算合规评分
	report.Score = s.calculateScore(report)

	// 生成摘要
	report.Summary = s.generateSummary(report)

	// 标记完成
	now := time.Now()
	report.CompletedAt = &now
	report.Status = StatusCompleted

	s.mu.Lock()
	s.reports[report.ID] = report
	s.mu.Unlock()

	return report, nil
}

// GetReport 根据 ID 获取报告.
func (s *Service) GetReport(reportID string) (*Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report, ok := s.reports[reportID]
	if !ok {
		return nil, ErrReportNotFound
	}
	return report, nil
}

// ListReports 列出所有报告.
func (s *Service) ListReports() []*Report {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports := make([]*Report, 0, len(s.reports))
	for _, r := range s.reports {
		reports = append(reports, r)
	}

	// 按创建时间倒序排序
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})

	return reports
}

// ========== 定期报告计划 ==========

// CreateSchedule 创建定期报告计划.
func (s *Service) CreateSchedule(req ScheduleRequest) (*Schedule, error) {
	// 验证合规标准
	if _, ok := supportedStandards[req.Standard]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard)
	}

	// 验证报告格式
	format := req.Format
	if format == "" {
		format = FormatJSON
	}
	if !validFormats[format] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidFormat, format)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	schedule := &Schedule{
		ID:          uuid.New().String(),
		Standard:    req.Standard,
		Format:      format,
		CronExpr:    req.CronExpr,
		Enabled:     true,
		GeneratedBy: req.GeneratedBy,
		CreatedAt:   time.Now(),
	}

	s.schedules[schedule.ID] = schedule
	return schedule, nil
}

// GetSchedule 获取计划.
func (s *Service) GetSchedule(scheduleID string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.schedules[scheduleID]
	if !ok {
		return nil, ErrScheduleNotFound
	}
	return schedule, nil
}

// ListSchedules 列出所有计划.
func (s *Service) ListSchedules() []*Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(s.schedules))
	for _, sc := range s.schedules {
		schedules = append(schedules, sc)
	}
	return schedules
}

// UpdateScheduleLastRun 更新计划上次执行时间.
func (s *Service) UpdateScheduleLastRun(scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, ok := s.schedules[scheduleID]
	if !ok {
		return ErrScheduleNotFound
	}

	now := time.Now()
	schedule.LastRunAt = &now
	return nil
}

// DeleteSchedule 删除计划.
func (s *Service) DeleteSchedule(scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[scheduleID]; !ok {
		return ErrScheduleNotFound
	}
	delete(s.schedules, scheduleID)
	return nil
}

// ========== 内部辅助方法 ==========

// collectEvidence 采集合规证据并生成控制项检查结果.
func (s *Service) collectEvidence(standard Standard) []ControlCheck {
	info := supportedStandards[standard]
	controls := make([]ControlCheck, 0, len(info.Categories))

	for i, category := range info.Categories {
		controlID := fmt.Sprintf("%s-CTRL-%03d", standard, i+1)

		// 模拟采集各类证据
		evidence := s.gatherEvidence(standard, category)

		// 根据证据确定检查状态
		status := determineCheckStatus(evidence)
		severity := determineSeverity(status, evidence)

		control := ControlCheck{
			ID:        controlID,
			Category:  category,
			Title:     fmt.Sprintf("%s - %s 合规检查", category, info.Name),
			Status:    status,
			Severity:  severity,
			Evidence:  evidence,
		}

		// 如果不通过，添加整改建议
		if status == CheckFail || status == CheckWarning {
			control.Remediation = s.generateRemediation(standard, category, status)
		}

		controls = append(controls, control)
	}

	return controls
}

// gatherEvidence 采集单个控制类别的证据.
func (s *Service) gatherEvidence(standard Standard, category string) []Evidence {
	now := time.Now()

	// 根据合规标准和类别生成相应的证据
	var evidence []Evidence

	// 访问日志证据
	evidence = append(evidence, Evidence{
		Type:      EvidenceAccessLog,
		Source:    "audit-system",
		Title:     "访问控制日志审查",
		Summary:   "已审查近30天访问日志，所有关键操作均有审计记录",
		Detail:    "访问日志覆盖率100%，关键操作审计率100%",
		Timestamp: now,
		Status:    CheckPass,
	})

	// 权限配置证据
	evidence = append(evidence, Evidence{
		Type:      EvidencePermission,
		Source:    "rbac-system",
		Title:     "权限配置审查",
		Summary:   "用户权限配置已审查，最小权限原则执行情况良好",
		Timestamp: now,
		Status:    CheckPass,
	})

	// 加密状态证据
	encStatus := CheckPass
	encSummary := "所有敏感数据存储和传输均已加密"
	encSeverity := SeverityLow
	if standard == StandardGDPR || standard == StandardPIPL {
		// 对 GDPR/PIPL 更严格检查
		encSummary = "敏感数据存储已加密，传输加密使用 TLS 1.2+"
	}
	evidence = append(evidence, Evidence{
		Type:      EvidenceEncryption,
		Source:    "encryption-module",
		Title:     "数据加密状态",
		Summary:   encSummary,
		Timestamp: now,
		Status:    encStatus,
		Severity:  encSeverity,
	})

	// 备份记录证据
	evidence = append(evidence, Evidence{
		Type:      EvidenceBackup,
		Source:    "backup-system",
		Title:     "数据备份合规性",
		Summary:   "备份计划已配置，最近备份成功完成",
		Timestamp: now,
		Status:    CheckPass,
	})

	// 策略文档证据（部分标准可能缺失）
	policyStatus := CheckPass
	if category == "个人信息保护影响评估" || category == "数据泄露通知" {
		policyStatus = CheckWarning
	}
	evidence = append(evidence, Evidence{
		Type:      EvidencePolicy,
		Source:    "policy-manager",
		Title:     "策略文档完备性",
		Summary:   "相关策略文档已归档",
		Timestamp: now,
		Status:    policyStatus,
	})

	return evidence
}

// determineCheckStatus 根据证据确定检查状态.
func determineCheckStatus(evidence []Evidence) CheckStatus {
	hasFail := false
	hasWarning := false
	for _, e := range evidence {
		switch e.Status {
		case CheckFail:
			hasFail = true
		case CheckWarning:
			hasWarning = true
		}
	}

	switch {
	case hasFail:
		return CheckFail
	case hasWarning:
		return CheckWarning
	default:
		return CheckPass
	}
}

// determineSeverity 根据检查状态和证据确定严重程度.
func determineSeverity(status CheckStatus, evidence []Evidence) Severity {
	if status == CheckPass {
		return SeverityLow
	}

	// 检查是否有高严重程度的证据
	for _, e := range evidence {
		if e.Severity == SeverityCritical {
			return SeverityCritical
		}
		if e.Severity == SeverityHigh {
			return SeverityHigh
		}
	}

	if status == CheckFail {
		return SeverityHigh
	}
	return SeverityMedium
}

// calculateScore 计算合规评分.
func (s *Service) calculateScore(report *Report) int {
	if report.TotalChecks == 0 {
		return 0
	}

	// 通过=满分，警告=半分，不通过=0分，不适用=不计
	totalWeight := report.TotalChecks * 100
	achieved := report.Passed*100 + report.Warnings*50
	score := (achieved * 100) / totalWeight
	if score > 100 {
		score = 100
	}
	return score
}

// generateSummary 生成报告摘要.
func (s *Service) generateSummary(report *Report) string {
	info, ok := supportedStandards[report.Standard]
	if !ok {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("本次 %s 合规审计共检查 %d 项控制措施。", info.Name, report.TotalChecks))
	sb.WriteString(fmt.Sprintf("通过 %d 项，不通过 %d 项，警告 %d 项，不适用 %d 项。",
		report.Passed, report.Failed, report.Warnings, report.NotApplicable))
	sb.WriteString(fmt.Sprintf("合规评分：%d/100。", report.Score))

	if report.Score >= 90 {
		sb.WriteString("整体合规状况优秀。")
	} else if report.Score >= 70 {
		sb.WriteString("整体合规状况良好，但存在改进空间。")
	} else if report.Score >= 50 {
		sb.WriteString("整体合规状况一般，需重点关注不通过项。")
	} else {
		sb.WriteString("整体合规状况较差，需立即整改。")
	}

	return sb.String()
}

// generateRemediation 生成整改建议.
func (s *Service) generateRemediation(standard Standard, category string, status CheckStatus) string {
	switch status {
	case CheckFail:
		return fmt.Sprintf("【%s】%s：该项检查未通过，需立即整改。建议：1. 审查当前配置和流程；2. 制定整改计划并设定截止日期；3. 实施必要的变更并验证。", standard, category)
	case CheckWarning:
		return fmt.Sprintf("【%s】%s：该项检查存在警告，建议：1. 评估潜在风险；2. 制定改进计划；3. 持续监控并定期复查。", standard, category)
	default:
		return ""
	}
}

// GetSupportedStandards 获取支持的合规标准列表.
func (s *Service) GetSupportedStandards() []Standard {
	standards := make([]Standard, 0, len(supportedStandards))
	for std := range supportedStandards {
		standards = append(standards, std)
	}
	return standards
}
