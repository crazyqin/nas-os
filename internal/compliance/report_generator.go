// Package compliancereport 提供合规报告生成功能
package compliance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ReportGenerator 合规报告生成器.
type ReportGenerator struct {
	scanner      *Scanner
	remediation  *RemediationGenerator
	standards    *StandardsManager
	reports      map[string]*ComplianceReport
	reportsByStd map[ComplianceStandard][]string
	mu           sync.RWMutex
}

// NewReportGenerator 创建合规报告生成器.
func NewReportGenerator(standards *StandardsManager) *ReportGenerator {
	return &ReportGenerator{
		scanner:      NewScanner(),
		remediation:  NewRemediationGenerator(),
		standards:    standards,
		reports:      make(map[string]*ComplianceReport),
		reportsByStd: make(map[ComplianceStandard][]string),
	}
}

// GenerateReport 生成合规报告.
func (g *ReportGenerator) GenerateReport(ctx context.Context, req ScanRequest) (*ComplianceReport, error) {
	// 验证标准是否支持
	if !g.standards.IsSupported(req.Standard) {
		return nil, fmt.Errorf("不支持的合规标准: %s", req.Standard)
	}

	// 创建报告
	reportID := GenerateID("cr")
	report := &ComplianceReport{
		ID:        reportID,
		Standard:  req.Standard,
		Status:    ScanStatusRunning,
		Format:    FormatJSON,
		CreatedAt: time.Now(),
	}

	if req.Format != "" {
		report.Format = req.Format
	}

	// 执行扫描
	results := g.scanner.Scan(ctx, req.Categories)
	report.Results = results

	// 统计结果
	for _, r := range results {
		report.TotalChecks++
		switch r.Status {
		case CheckItemPass:
			report.Passed++
		case CheckItemFail:
			report.Failed++
		case CheckItemWarning:
			report.Warnings++
		case CheckItemSkip:
			report.Skipped++
		}
	}

	// 计算合规分数
	if report.TotalChecks > 0 {
		report.Score = (report.Passed * 100) / report.TotalChecks
	}

	// 确定合规状态
	report.ComplianceStatus = g.determineComplianceStatus(report.Score)

	// 生成整改建议
	report.Remediations = g.remediation.Generate(results)

	// 生成摘要
	report.Summary = g.generateSummary(report)

	// 标记完成
	now := time.Now()
	report.CompletedAt = &now
	report.Status = ScanStatusComplete

	// 保存报告
	g.mu.Lock()
	g.reports[reportID] = report
	g.reportsByStd[req.Standard] = append(g.reportsByStd[req.Standard], reportID)
	g.mu.Unlock()

	return report, nil
}

// GetReport 获取报告.
func (g *ReportGenerator) GetReport(id string) (*ComplianceReport, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	report, ok := g.reports[id]
	return report, ok
}

// ListReports 列出报告.
func (g *ReportGenerator) ListReports(standard *ComplianceStandard) []*ComplianceReport {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var reports []*ComplianceReport

	if standard != nil {
		ids, ok := g.reportsByStd[*standard]
		if !ok {
			return reports
		}
		for _, id := range ids {
			if r, exists := g.reports[id]; exists {
				reports = append(reports, r)
			}
		}
	} else {
		for _, r := range g.reports {
			reports = append(reports, r)
		}
	}

	return reports
}

// GetStatus 获取当前合规状态总览.
func (g *ReportGenerator) GetStatus() *ComplianceStatusOverview {
	g.mu.RLock()
	defer g.mu.RUnlock()

	overview := &ComplianceStatusOverview{
		Standards: make([]StandardStatus, 0),
	}

	totalScore := 0
	stdCount := 0
	pendingRemediation := 0

	for _, std := range g.standards.ListStandards() {
		ids, ok := g.reportsByStd[std.ID]
		if !ok || len(ids) == 0 {
			overview.Standards = append(overview.Standards, StandardStatus{
				Standard: std.ID,
				Status:   StatusPendingReview,
				Score:    0,
			})
			continue
		}

		latest := g.reports[ids[len(ids)-1]]
		stdStatus := StandardStatus{
			Standard: std.ID,
			Status:   latest.ComplianceStatus,
			Score:    latest.Score,
			LastScan: latest.CompletedAt,
		}
		overview.Standards = append(overview.Standards, stdStatus)

		totalScore += latest.Score
		stdCount++
		pendingRemediation += len(latest.Remediations)

		if overview.LastScanTime == nil || latest.CompletedAt.After(*overview.LastScanTime) {
			overview.LastScanTime = latest.CompletedAt
		}
	}

	overview.TotalReports = len(g.reports)
	overview.PendingRemediation = pendingRemediation

	if stdCount > 0 {
		overview.OverallScore = totalScore / stdCount
	}

	// 总体状态取最差的
	overview.OverallStatus = g.determineOverallStatus(overview.Standards)

	return overview
}

// determineComplianceStatus 根据分数确定合规状态.
func (g *ReportGenerator) determineComplianceStatus(score int) ComplianceStatus {
	switch {
	case score >= 90:
		return StatusCompliant
	case score >= 60:
		return StatusPendingReview
	default:
		return StatusNonCompliant
	}
}

// determineOverallStatus 确定总体合规状态.
func (g *ReportGenerator) determineOverallStatus(standards []StandardStatus) ComplianceStatus {
	if len(standards) == 0 {
		return StatusPendingReview
	}

	hasNonCompliant := false
	hasPending := false

	for _, s := range standards {
		switch s.Status {
		case StatusNonCompliant:
			hasNonCompliant = true
		case StatusPendingReview:
			hasPending = true
		}
	}

	if hasNonCompliant {
		return StatusNonCompliant
	}
	if hasPending {
		return StatusPendingReview
	}
	return StatusCompliant
}

// generateSummary 生成报告摘要.
func (g *ReportGenerator) generateSummary(report *ComplianceReport) string {
	stdName := string(report.Standard)
	if info, ok := g.standards.GetStandard(report.Standard); ok {
		stdName = info.Name
	}

	statusDesc := "合规"
	switch report.ComplianceStatus {
	case StatusNonCompliant:
		statusDesc = "不合规"
	case StatusPendingReview:
		statusDesc = "待审查"
	}

	return fmt.Sprintf("%s 合规扫描完成: 共 %d 项检查, 通过 %d 项, 失败 %d 项, 警告 %d 项. 合规得分: %d/100, 状态: %s. 共 %d 条整改建议.",
		stdName, report.TotalChecks, report.Passed, report.Failed, report.Warnings,
		report.Score, statusDesc, len(report.Remediations))
}
