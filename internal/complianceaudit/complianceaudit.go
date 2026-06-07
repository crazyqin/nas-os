// Package complianceaudit 提供合规审计功能，支持多标准合规检查和审计报告生成
package complianceaudit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Engine 合规审计引擎
type Engine struct {
	manager    *Manager
	scanner    *Scanner
	policy     *PolicyEngine
	reporter   *Reporter
	remediator *Remediator
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewEngine 创建合规审计引擎
func NewEngine(store *Store, logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}

	mgr := NewManager(store, logger)
	policy := NewPolicyEngine(logger)
	scanner := NewScanner(logger)
	reporter := NewReporter(logger)
	remediator := NewRemediator(logger)

	// 注册内置合规检查项
	registerBuiltinChecks(mgr, scanner)

	return &Engine{
		manager:    mgr,
		scanner:    scanner,
		policy:     policy,
		reporter:   reporter,
		remediator: remediator,
		logger:     logger,
	}
}

// GetManager 获取管理器
func (e *Engine) GetManager() *Manager { return e.manager }

// GetScanner 获取扫描器
func (e *Engine) GetScanner() *Scanner { return e.scanner }

// GetPolicyEngine 获取策略引擎
func (e *Engine) GetPolicyEngine() *PolicyEngine { return e.policy }

// GetReporter 获取报告生成器
func (e *Engine) GetReporter() *Reporter { return e.reporter }

// GetRemediator 获取修复建议器
func (e *Engine) GetRemediator() *Remediator { return e.remediator }

// RunFullAudit 执行完整合规审计
func (e *Engine) RunFullAudit(ctx context.Context) (*AuditResult, error) {
	e.logger.Info("starting full compliance audit")

	// 1. 执行扫描
	report := e.manager.RunFullScan(ctx)

	// 2. 应用策略评估
	policyResults := e.policy.EvaluateAll(report)

	// 3. 生成修复建议
	remediations := e.remediator.GenerateRemediations(report.Findings)

	// 4. 生成报告
	fullReport := e.reporter.GenerateFullReport(report, policyResults, remediations)

	result := &AuditResult{
		Report:        report,
		PolicyResults: policyResults,
		Remediations:  remediations,
		FullReport:    fullReport,
		CompletedAt:   time.Now(),
	}

	e.logger.Info("full compliance audit completed",
		zap.Int("total_checks", report.Summary.TotalChecks),
		zap.Float64("score", report.Summary.OverallScore),
		zap.String("risk_level", string(report.Summary.RiskLevel)),
	)

	return result, nil
}

// RunCategoryAudit 执行指定类别审计
func (e *Engine) RunCategoryAudit(ctx context.Context, category CheckCategory) (*ComplianceReport, error) {
	e.logger.Info("starting category audit", zap.String("category", string(category)))

	checks := e.manager.ListChecks()
	var categoryChecks []string
	for _, c := range checks {
		if c.Category == category {
			categoryChecks = append(categoryChecks, c.Name)
		}
	}

	if len(categoryChecks) == 0 {
		return nil, fmt.Errorf("no checks found for category %q", category)
	}

	// 执行该类别下的所有检查
	report := &ComplianceReport{
		ID:          fmt.Sprintf("category_report_%s_%d", category, time.Now().Unix()),
		Title:       fmt.Sprintf("%s 类别审计报告", category),
		GeneratedAt: time.Now(),
		Period: ReportPeriod{
			Start: time.Now().AddDate(0, 0, -1),
			End:   time.Now(),
		},
		Summary:      &ReportSummary{},
		Standards:    make([]*StandardReport, 0),
		Findings:     make([]*Finding, 0),
		Remediations: make([]*RemediationItem, 0),
		Format:       FormatJSON,
	}

	for _, name := range categoryChecks {
		result, err := e.manager.RunSingleCheck(ctx, name)
		if err != nil {
			e.logger.Warn("check failed", zap.String("name", name), zap.Error(err))
			continue
		}

		report.Summary.TotalChecks++
		switch result.Status {
		case StatusPass:
			report.Summary.Passed++
		case StatusFail:
			report.Summary.Failed++
			report.Findings = append(report.Findings, &Finding{
				ID:          fmt.Sprintf("finding_%s_%d", name, time.Now().UnixNano()),
				Title:       result.Message,
				Description: fmt.Sprintf("检查项 %s 未通过", name),
				RiskLevel:   result.RiskLevel,
				Standard:    result.Standard,
				Category:    result.Category,
				Status:      result.Status,
			})
		case StatusWarn:
			report.Summary.Warnings++
		}
	}

	if report.Summary.TotalChecks > 0 {
		report.Summary.OverallScore = float64(report.Summary.Passed) / float64(report.Summary.TotalChecks) * 100
	}
	report.Summary.RiskLevel = e.manager.calculateRiskLevel(report.Summary.OverallScore)

	return report, nil
}

// registerBuiltinChecks 注册内置合规检查项
func registerBuiltinChecks(mgr *Manager, scanner *Scanner) {
	// 配置审计检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "config_audit",
		standard:    StandardMLPS2,
		category:    CategoryAccessControl,
		description: "系统安全配置审计",
		scanner:     scanner,
		scanFunc:    scanner.ScanSystemConfig,
	})

	// 文件权限检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "file_permissions",
		standard:    StandardMLPS2,
		category:    CategoryAccessControl,
		description: "敏感目录文件权限检查",
		scanner:     scanner,
		scanFunc:    scanner.ScanFilePermissions,
	})

	// 密码策略检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "password_policy",
		standard:    StandardGDPR,
		category:    CategoryPasswordPolicy,
		description: "用户密码策略审计",
		scanner:     scanner,
		scanFunc:    scanner.ScanPasswordPolicy,
	})

	// 网络暴露检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "network_exposure",
		standard:    StandardISO27001,
		category:    CategoryNetworkSecurity,
		description: "网络服务暴露检测",
		scanner:     scanner,
		scanFunc:    scanner.ScanNetworkExposure,
	})

	// 加密状态检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "encryption_status",
		standard:    StandardGDPR,
		category:    CategoryEncryption,
		description: "数据加密状态检查",
		scanner:     scanner,
		scanFunc:    scanner.ScanEncryptionStatus,
	})

	// GDPR 数据保护检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "gdpr_data_protection",
		standard:    StandardGDPR,
		category:    CategoryDataProtection,
		description: "GDPR 数据保护合规检查",
		scanner:     scanner,
		scanFunc:    scanner.ScanDataProtection,
	})

	// 等保2.0 审计日志检查
	mgr.RegisterCheck(&builtinCheck{
		name:        "mlps2_audit_log",
		standard:    StandardMLPS2,
		category:    CategoryAuditLog,
		description: "等保2.0 审计日志合规检查",
		scanner:     scanner,
		scanFunc:    scanner.ScanAuditLog,
	})
}

// builtinCheck 内置合规检查项实现
type builtinCheck struct {
	name        string
	standard    ComplianceStandard
	category    CheckCategory
	description string
	scanner     *Scanner
	scanFunc    func(ctx *CheckContext) *CheckResult
}

func (c *builtinCheck) Name() string                 { return c.name }
func (c *builtinCheck) Standard() ComplianceStandard { return c.standard }
func (c *builtinCheck) Category() CheckCategory      { return c.category }
func (c *builtinCheck) Description() string          { return c.description }

func (c *builtinCheck) Check(ctx *CheckContext) *CheckResult {
	return c.scanFunc(ctx)
}

func (c *builtinCheck) GetRemediation(result *CheckResult) *Remediation {
	remediator := NewRemediator(nil)
	return remediator.GetRemediation(result)
}

// AuditResult 完整审计结果
type AuditResult struct {
	Report        *ComplianceReport  `json:"report"`
	PolicyResults []*PolicyResult    `json:"policy_results"`
	Remediations  []*RemediationItem `json:"remediations"`
	FullReport    *FullReport        `json:"full_report"`
	CompletedAt   time.Time          `json:"completed_at"`
}
