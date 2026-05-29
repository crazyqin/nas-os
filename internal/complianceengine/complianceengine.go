// Package complianceengine 提供合规审计引擎功能
// 支持合规策略管理、自动化合规检查、合规报告生成
package complianceengine

import (
	"time"
)

// ComplianceEngine 合规审计引擎 (对外入口)
type ComplianceEngine struct {
	manager *Manager
	handler *Handler
}

// NewComplianceEngine 创建合规审计引擎
func NewComplianceEngine(config EngineConfig) *ComplianceEngine {
	manager := NewManager(config)
	handler := NewHandler(manager)

	engine := &ComplianceEngine{
		manager: manager,
		handler: handler,
	}

	// 加载默认规则
	engine.loadDefaultRules()

	return engine
}

// GetManager 获取管理器
func (e *ComplianceEngine) GetManager() *Manager {
	return e.manager
}

// GetHandler 获取处理器
func (e *ComplianceEngine) GetHandler() *Handler {
	return e.handler
}

// loadDefaultRules 加载默认合规规则
func (e *ComplianceEngine) loadDefaultRules() {
	defaultRules := []ComplianceRule{
		// GDPR 规则
		{Standard: StandardGDPR, Category: CategoryDataProtection, Severity: SeverityCritical, Title: "数据加密", Description: "敏感数据必须加密存储", Requirement: "所有个人数据需加密存储和传输", Remediation: "启用 AES-256 加密", Enabled: true},
		{Standard: StandardGDPR, Category: CategoryAccessControl, Severity: SeverityHigh, Title: "访问控制", Description: "实施最小权限原则", Requirement: "基于角色的访问控制", Remediation: "配置 RBAC 权限策略", Enabled: true},
		{Standard: StandardGDPR, Category: CategoryDataProtection, Severity: SeverityHigh, Title: "数据备份", Description: "定期备份数据并测试恢复", Requirement: "每日备份，每月测试恢复", Remediation: "配置自动备份策略", Enabled: true},

		// HIPAA 规则
		{Standard: StandardHIPAA, Category: CategoryNetworkSecurity, Severity: SeverityCritical, Title: "传输加密", Description: "数据传输必须使用TLS加密", Requirement: "TLS 1.2 及以上", Remediation: "配置 HTTPS 和 TLS", Enabled: true},
		{Standard: StandardHIPAA, Category: CategoryAuditLogging, Severity: SeverityCritical, Title: "审计日志", Description: "记录所有数据访问", Requirement: "完整的访问审计日志", Remediation: "启用审计日志功能", Enabled: true},
		{Standard: StandardHIPAA, Category: CategoryAccessControl, Severity: SeverityHigh, Title: "身份认证", Description: "实施强身份认证", Requirement: "MFA 多因素认证", Remediation: "启用双因素认证", Enabled: true},

		// CIS 规则
		{Standard: StandardCIS, Category: CategoryAccessControl, Severity: SeverityHigh, Title: "访问控制策略", Description: "实施严格的访问控制", Requirement: "最小权限原则", Remediation: "配置 ACL 策略", Enabled: true},
		{Standard: StandardCIS, Category: CategoryNetworkSecurity, Severity: SeverityHigh, Title: "网络安全", Description: "网络边界防护", Requirement: "防火墙和入侵检测", Remediation: "配置防火墙规则", Enabled: true},
		{Standard: StandardCIS, Category: CategoryAuditLogging, Severity: SeverityMedium, Title: "审计配置", Description: "审计日志配置合规", Requirement: "日志保留90天以上", Remediation: "配置日志保留策略", Enabled: true},
	}

	for _, rule := range defaultRules {
		e.manager.CreateRule(rule)
	}
}

// QuickScan 快速扫描指定标准
func (e *ComplianceEngine) QuickScan(standard ComplianceStandard) (*ComplianceScan, error) {
	return e.manager.StartScan([]ComplianceStandard{standard})
}

// GenerateReport 生成报告
func (e *ComplianceEngine) GenerateReport(scanID string, format ReportFormat) (*ComplianceReport, error) {
	return e.manager.GenerateReport(scanID, format)
}

// GetComplianceScore 获取合规分数
func (e *ComplianceEngine) GetComplianceScore() float64 {
	stats := e.manager.GetStats()
	return stats.AverageScore
}

// GetLastScanTime 获取最后扫描时间
func (e *ComplianceEngine) GetLastScanTime() *time.Time {
	stats := e.manager.GetStats()
	return stats.LastScanTime
}

// IsEnabled 检查是否启用
func (e *ComplianceEngine) IsEnabled() bool {
	config := e.manager.GetConfig()
	return config.Enabled
}
