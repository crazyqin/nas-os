// Package compliance 提供合规报告功能
package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Report 合规报告
type Report struct {
	ID              string        `json:"id"`
	Timestamp       time.Time     `json:"timestamp"`
	OverallLevel    Level         `json:"overall_level"`
	PassedCount     int           `json:"passed_count"`
	FailedCount     int           `json:"failed_count"`
	Results         []CheckResult `json:"results"`
	Summary         string        `json:"summary"`
	Recommendations []string      `json:"recommendations,omitempty"`
}

// ReportGenerator 报告生成器
type ReportGenerator struct {
	outputDir string
	checker   *Checker
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(outputDir string, checker *Checker) (*ReportGenerator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	return &ReportGenerator{
		outputDir: outputDir,
		checker:   checker,
	}, nil
}

// GenerateReport 生成合规报告
func (g *ReportGenerator) GenerateReport(ctx context.Context) (*Report, error) {
	report, err := g.checker.RunChecks(ctx)
	if err != nil {
		return nil, err
	}

	// 生成摘要
	report.Summary = g.generateSummary(report)

	// 生成建议
	report.Recommendations = g.generateRecommendations(report)

	// 保存报告
	if err := g.saveReport(report); err != nil {
		return nil, err
	}

	return report, nil
}

// GenerateReportByType 按类型生成报告
func (g *ReportGenerator) GenerateReportByType(ctx context.Context, checkType CheckType) (*Report, error) {
	report, err := g.checker.RunChecksByType(ctx, checkType)
	if err != nil {
		return nil, err
	}

	report.Summary = g.generateSummary(report)
	report.Recommendations = g.generateRecommendations(report)

	if err := g.saveReport(report); err != nil {
		return nil, err
	}

	return report, nil
}

// generateSummary 生成摘要
func (g *ReportGenerator) generateSummary(report *Report) string {
	total := len(report.Results)
	passRate := 0.0
	if total > 0 {
		passRate = float64(report.PassedCount) / float64(total) * 100
	}

	return fmt.Sprintf("合规检查完成: 共 %d 项检查，通过 %d 项，失败 %d 项，通过率 %.1f%%，总体合规级别: %s",
		total, report.PassedCount, report.FailedCount, passRate, report.OverallLevel)
}

// generateRecommendations 生成建议
func (g *ReportGenerator) generateRecommendations(report *Report) []string {
	var recommendations []string

	for _, result := range report.Results {
		if !result.Passed {
			rec := fmt.Sprintf("[%s] %s: %s", result.Type, result.Name, result.Message)
			recommendations = append(recommendations, rec)
		}
	}

	return recommendations
}

// saveReport 保存报告
func (g *ReportGenerator) saveReport(report *Report) error {
	filename := filepath.Join(g.outputDir, fmt.Sprintf("compliance_%s.json", report.Timestamp.Format("2006-01-02_150405")))

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0640)
}

// LoadReport 加载报告
func (g *ReportGenerator) LoadReport(filename string) (*Report, error) {
	data, err := os.ReadFile(filepath.Join(g.outputDir, filename))
	if err != nil {
		return nil, err
	}

	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	return &report, nil
}

// ListReports 列出所有报告
func (g *ReportGenerator) ListReports() ([]string, error) {
	entries, err := os.ReadDir(g.outputDir)
	if err != nil {
		return nil, err
	}

	var reports []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			reports = append(reports, entry.Name())
		}
	}

	return reports, nil
}

// GetLatestReport 获取最新报告
func (g *ReportGenerator) GetLatestReport() (*Report, error) {
	reports, err := g.ListReports()
	if err != nil {
		return nil, err
	}

	if len(reports) == 0 {
		return nil, fmt.Errorf("没有找到合规报告")
	}

	// 按时间排序，取最新的
	latest := reports[len(reports)-1]
	return g.LoadReport(latest)
}

// ExportToText 导出为文本格式
func (g *ReportGenerator) ExportToText(report *Report) string {
	var text string
	text += "=== NAS-OS 合规检查报告 ===\n"
	text += fmt.Sprintf("报告ID: %s\n", report.ID)
	text += fmt.Sprintf("检查时间: %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	text += fmt.Sprintf("总体合规级别: %s\n", report.OverallLevel)
	text += fmt.Sprintf("通过/失败: %d/%d\n\n", report.PassedCount, report.FailedCount)
	text += fmt.Sprintf("摘要: %s\n\n", report.Summary)

	text += "详细结果:\n"
	for _, r := range report.Results {
		status := "✓ 通过"
		if !r.Passed {
			status = "✗ 失败"
		}
		text += fmt.Sprintf("  - [%s] %s: %s\n", r.Type, r.Name, status)
	}

	if len(report.Recommendations) > 0 {
		text += "\n改进建议:\n"
		for _, rec := range report.Recommendations {
			text += fmt.Sprintf("  - %s\n", rec)
		}
	}

	return text
}
