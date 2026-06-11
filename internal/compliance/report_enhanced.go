// Package compliance 提供 CIS、STIG、GDPR 合规检查和报告功能
//
// CIS (Center for Internet Security) 基准检查
// STIG (Security Technical Implementation Guide) 合规检查
// GDPR (General Data Protection Regulation) 数据保护检查
package compliance

import (
	"fmt"
	"sync"
	"time"
)

// ComplianceStandard 合规标准
type ComplianceStandard string

const (
	StandardCIS   ComplianceStandard = "CIS"
	StandardSTIG  ComplianceStandard = "STIG"
	StandardGDPR  ComplianceStandard = "GDPR"
	StandardCCPA  ComplianceStandard = "CCPA"
	StandardHIPAA ComplianceStandard = "HIPAA"
	StandardSOC2  ComplianceStandard = "SOC2"
	StandardISO27001 ComplianceStandard = "ISO27001"
)

// ComplianceCheckStatus 合规检查状态
type ComplianceCheckStatus string

const (
	StatusCompliant    ComplianceCheckStatus = "compliant"
	StatusNonCompliant ComplianceCheckStatus = "non-compliant"
	StatusPartial      ComplianceCheckStatus = "partial"
	StatusNotApplicable ComplianceCheckStatus = "not-applicable"
	StatusError        ComplianceCheckStatus = "error"
)

// BenchmarkCategory 基准类别
type BenchmarkCategory string

const (
	CategoryInitialSetup    BenchmarkCategory = "initial_setup"
	CategoryServices        BenchmarkCategory = "services"
	CategoryNetworkConfig   BenchmarkCategory = "network_config"
	CategoryLogging         BenchmarkCategory = "logging"
	CategoryAccessControl   BenchmarkCategory = "access_control"
	CategoryMaintenance     BenchmarkCategory = "maintenance"
	CategorySystemSettings  BenchmarkCategory = "system_settings"
	CategoryDataProtection  BenchmarkCategory = "data_protection"
	CategoryPrivacy         BenchmarkCategory = "privacy"
	CategoryEncryption      BenchmarkCategory = "encryption"
)

// ComplianceCheck 合规检查项
type ComplianceCheck struct {
	ID          string              `json:"id"`
	Standard    ComplianceStandard  `json:"standard"`
	Category    BenchmarkCategory   `json:"category"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Severity    string              `json:"severity"` // critical, high, medium, low
	Requirement string              `json:"requirement"`
	Remediation string              `json:"remediation"`
	References  []string            `json:"references,omitempty"`
	Automated   bool                `json:"automated"`
}

// ComplianceCheckResult 合规检查结果
type ComplianceCheckResult struct {
	CheckID     string                `json:"check_id"`
	Standard    ComplianceStandard    `json:"standard"`
	Category    BenchmarkCategory     `json:"category"`
	Title       string                `json:"title"`
	Status      ComplianceCheckStatus `json:"status"`
	Evidence    string                `json:"evidence,omitempty"`
	ActualValue string                `json:"actual_value,omitempty"`
	ExpectedValue string             `json:"expected_value,omitempty"`
	Remediation string                `json:"remediation,omitempty"`
	CheckedAt   time.Time             `json:"checked_at"`
}

// CISBenchmark CIS 基准配置
type CISBenchmark struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Level       int    `json:"level"` // 1 = 基本, 2 = 扩展
	Description string `json:"description"`
}

// STIGCheck STIG 检查配置
type STIGCheck struct {
	ID          string `json:"id"`
	VulnID      string `json:"vuln_id"` // 漏洞 ID
	GroupID     string `json:"group_id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CheckText   string `json:"check_text"`
	FixText     string `json:"fix_text"`
}

// GDPRArticle GDPR 条款
type GDPRArticle struct {
	Article     string   `json:"article"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Requirements []string `json:"requirements"`
}

// ComplianceScanner 合规扫描器
type ComplianceScanner struct {
	mu            sync.RWMutex
	checks        map[string]*ComplianceCheck
	cisBenchmark  *CISBenchmark
	stigChecks    map[string]*STIGCheck
	gdprArticles  map[string]*GDPRArticle
	lastScanTime  time.Time
}

// NewComplianceScanner 创建合规扫描器
func NewComplianceScanner() *ComplianceScanner {
	scanner := &ComplianceScanner{
		checks:       make(map[string]*ComplianceCheck),
		stigChecks:   make(map[string]*STIGCheck),
		gdprArticles: make(map[string]*GDPRArticle),
	}

	// 初始化 CIS 基准
	scanner.initCISBenchmark()

	// 初始化 STIG 检查
	scanner.initSTIGChecks()

	// 初始化 GDPR 条款
	scanner.initGDPRArticles()

	return scanner
}

// initCISBenchmark 初始化 CIS 基准
func (s *ComplianceScanner) initCISBenchmark() {
	s.cisBenchmark = &CISBenchmark{
		ID:          "cis-nas-os-v1.0",
		Name:        "CIS NAS-OS Benchmark",
		Version:     "1.0.0",
		Level:       1,
		Description: "CIS NAS-OS 安全基准配置指南",
	}

	// 注册 CIS 检查项
	cisChecks := []*ComplianceCheck{
		{
			ID: "CIS-1.1.1", Standard: StandardCIS, Category: CategoryInitialSetup,
			Title: "文件系统配置", Description: "确保文件系统配置符合安全要求",
			Severity: "high", Requirement: "文件系统应使用安全挂载选项",
			Remediation: "配置 noexec, nosuid, nodev 等挂载选项", Automated: true,
		},
		{
			ID: "CIS-1.1.2", Standard: StandardCIS, Category: CategoryInitialSetup,
			Title: "磁盘空间限制", Description: "确保为 /tmp 分区设置独立分区和空间限制",
			Severity: "medium", Requirement: "/tmp 应有独立分区和空间限制",
			Remediation: "为 /tmp 创建独立分区并设置空间限制", Automated: true,
		},
		{
			ID: "CIS-2.1.1", Standard: StandardCIS, Category: CategoryServices,
			Title: "不必要的服务", Description: "确保不必要的服务已禁用",
			Severity: "high", Requirement: "禁用不需要的网络服务",
			Remediation: "使用 systemctl disable 禁用不必要的服务", Automated: true,
		},
		{
			ID: "CIS-2.2.1", Standard: StandardCIS, Category: CategoryServices,
			Title: "时间同步", Description: "确保时间同步服务已配置",
			Severity: "high", Requirement: "系统应使用 NTP 进行时间同步",
			Remediation: "配置并启用 chronyd 或 ntpd 服务", Automated: true,
		},
		{
			ID: "CIS-3.1.1", Standard: StandardCIS, Category: CategoryNetworkConfig,
			Title: "IP 转发", Description: "确保 IP 转发已禁用（除非需要）",
			Severity: "high", Requirement: "net.ipv4.ip_forward 应设为 0",
			Remediation: "在 /etc/sysctl.conf 中设置 net.ipv4.ip_forward = 0", Automated: true,
		},
		{
			ID: "CIS-3.2.1", Standard: StandardCIS, Category: CategoryNetworkConfig,
			Title: "防火墙配置", Description: "确保防火墙已启用并正确配置",
			Severity: "critical", Requirement: "应启用主机防火墙并限制入站流量",
			Remediation: "配置 iptables 或 firewalld 规则", Automated: true,
		},
		{
			ID: "CIS-4.1.1", Standard: StandardCIS, Category: CategoryLogging,
			Title: "审计日志", Description: "确保审计日志已启用",
			Severity: "critical", Requirement: "应启用系统审计日志记录",
			Remediation: "启用 auditd 服务并配置审计规则", Automated: true,
		},
		{
			ID: "CIS-4.1.2", Standard: StandardCIS, Category: CategoryLogging,
			Title: "日志轮转", Description: "确保日志轮转已配置",
			Severity: "medium", Requirement: "日志文件应有轮转策略",
			Remediation: "配置 logrotate 进行日志轮转", Automated: true,
		},
		{
			ID: "CIS-5.1.1", Standard: StandardCIS, Category: CategoryAccessControl,
			Title: "SSH 配置", Description: "确保 SSH 服务配置安全",
			Severity: "critical", Requirement: "SSH 应禁用 root 登录和密码认证",
			Remediation: "配置 PermitRootLogin no 和 PasswordAuthentication no", Automated: true,
		},
		{
			ID: "CIS-5.2.1", Standard: StandardCIS, Category: CategoryAccessControl,
			Title: "密码策略", Description: "确保密码策略符合安全要求",
			Severity: "high", Requirement: "密码最小长度 14 位，包含大小写字母、数字和特殊字符",
			Remediation: "配置 PAM 密码策略", Automated: true,
		},
		{
			ID: "CIS-6.1.1", Standard: StandardCIS, Category: CategoryMaintenance,
			Title: "系统更新", Description: "确保系统已安装最新安全更新",
			Severity: "critical", Requirement: "系统应定期更新安全补丁",
			Remediation: "配置自动安全更新", Automated: true,
		},
		{
			ID: "CIS-7.1.1", Standard: StandardCIS, Category: CategoryDataProtection,
			Title: "数据加密", Description: "确保敏感数据已加密存储",
			Severity: "critical", Requirement: "敏感数据应使用 AES-256 加密",
			Remediation: "启用 ZFS 加密或 LUKS 全盘加密", Automated: true,
		},
	}

	for _, check := range cisChecks {
		s.checks[check.ID] = check
	}
}

// initSTIGChecks 初始化 STIG 检查
func (s *ComplianceScanner) initSTIGChecks() {
	stigChecks := []*STIGCheck{
		{
			ID: "STIG-V-230221", VulnID: "V-230221", GroupID: "SRG-OS-000023",
			Severity: "medium",
			Title: "系统必须显示标准 DoD 登录警告",
			Description: "系统必须在登录前显示标准 DoD 登录警告消息",
			CheckText: "检查 /etc/issue 文件是否包含标准 DoD 警告",
			FixText: "在 /etc/issue 文件中添加标准 DoD 登录警告",
		},
		{
			ID: "STIG-V-230222", VulnID: "V-230222", GroupID: "SRG-OS-000024",
			Severity: "high",
			Title: "必须禁用不必要的文件系统",
			Description: "必须禁用不必要的文件系统类型",
			CheckText: "检查 /etc/modprobe.d/ 是否禁用了 cramfs, freevxfs, hfs, hfsplus, udf",
			FixText: "在 /etc/modprobe.d/ 中添加 install cramfs /bin/true 等配置",
		},
		{
			ID: "STIG-V-230223", VulnID: "V-230223", GroupID: "SRG-OS-000096",
			Severity: "high",
			Title: "必须禁用 USB 存储设备",
			Description: "必须禁用 USB 存储设备以防止未授权数据传输",
			CheckText: "检查 USB 存储模块是否已禁用",
			FixText: "在 /etc/modprobe.d/ 中添加 install usb-storage /bin/true",
		},
		{
			ID: "STIG-V-230224", VulnID: "V-230224", GroupID: "SRG-OS-000120",
			Severity: "critical",
			Title: "必须启用 SELinux",
			Description: "必须启用 SELinux 并设置为 enforcing 模式",
			CheckText: "检查 /etc/selinux/config 中 SELINUX=enforcing",
			FixText: "设置 SELINUX=enforcing 并重启系统",
		},
		{
			ID: "STIG-V-230225", VulnID: "V-230225", GroupID: "SRG-OS-000250",
			Severity: "high",
			Title: "SSH 必须使用 FIPS 批准的算法",
			Description: "SSH 必须使用 FIPS 140-2 批准的加密算法",
			CheckText: "检查 /etc/ssh/sshd_config 中的加密算法配置",
			FixText: "配置 Ciphers 和 MACs 为 FIPS 批准的算法",
		},
	}

	for _, check := range stigChecks {
		s.stigChecks[check.ID] = check
	}
}

// initGDPRArticles 初始化 GDPR 条款
func (s *ComplianceScanner) initGDPRArticles() {
	articles := map[string]*GDPRArticle{
		"Article-5": {
			Article: "Article 5",
			Title: "个人数据处理原则",
			Description: "个人数据的处理应遵循合法性、公正性和透明性原则",
			Requirements: []string{
				"数据处理需有合法依据",
				"数据收集需有明确目的",
				"数据应准确并及时更新",
				"数据存储不得超过必要期限",
				"数据处理需确保安全性",
			},
		},
		"Article-6": {
			Article: "Article 6",
			Title: "数据处理的合法性",
			Description: "个人数据处理的合法性基础",
			Requirements: []string{
				"获得数据主体同意",
				"履行合同义务所必需",
				"遵守法律义务所必需",
				"保护数据主体或他人重大利益",
				"执行公共利益或行使官方权力",
				"追求合法利益",
			},
		},
		"Article-17": {
			Article: "Article 17",
			Title: "删除权（被遗忘权）",
			Description: "数据主体有权要求删除其个人数据",
			Requirements: []string{
				"提供数据删除请求处理机制",
				"在 30 天内响应删除请求",
				"通知第三方删除相关数据",
				"记录删除请求处理过程",
			},
		},
		"Article-25": {
			Article: "Article 25",
			Title: "数据保护设计和默认设置",
			Description: "数据保护应融入系统设计和默认设置",
			Requirements: []string{
				"实施数据最小化原则",
				"默认设置应为最高隐私保护",
				"实施假名化和加密措施",
				"确保数据处理的可审计性",
			},
		},
		"Article-32": {
			Article: "Article 32",
			Title: "处理安全性",
			Description: "确保个人数据处理的安全性",
			Requirements: []string{
				"实施适当的加密措施",
				"确保系统的机密性、完整性、可用性和弹性",
				"实施灾难恢复能力",
				"定期测试和评估安全措施",
			},
		},
		"Article-33": {
			Article: "Article 33",
			Title: "数据泄露通知",
			Description: "向监管机构报告数据泄露",
			Requirements: []string{
				"在 72 小时内报告数据泄露",
				"记录数据泄露事件详情",
				"评估数据泄露的影响",
				"实施补救措施",
			},
		},
		"Article-35": {
			Article: "Article 35",
			Title: "数据保护影响评估",
			Description: "高风险数据处理活动需进行影响评估",
			Requirements: []string{
				"识别高风险数据处理活动",
				"评估数据处理对隐私的影响",
				"制定风险缓解措施",
				"记录评估结果",
			},
		},
	}

	for id, article := range articles {
		s.gdprArticles[id] = article
	}
}

// RunCISCheck 运行 CIS 基准检查
func (s *ComplianceScanner) RunCISCheck() *ComplianceStandardReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := &ComplianceStandardReport{
		ID:        fmt.Sprintf("cis-report-%d", time.Now().UnixNano()),
		Standard:  StandardCIS,
		Benchmark: s.cisBenchmark,
		CheckedAt: time.Now(),
	}

	var results []ComplianceCheckResult
	for _, check := range s.checks {
		if check.Standard == StandardCIS {
			result := s.executeCISCheck(check)
			results = append(results, result)
		}
	}

	report.Results = results
	report.Summary = s.calculateSummary(results)

	return report
}

// executeCISCheck 执行单个 CIS 检查
func (s *ComplianceScanner) executeCISCheck(check *ComplianceCheck) ComplianceCheckResult {
	result := ComplianceCheckResult{
		CheckID:   check.ID,
		Standard:  check.Standard,
		Category:  check.Category,
		Title:     check.Title,
		CheckedAt: time.Now(),
	}

	// 模拟检查逻辑
	switch check.ID {
	case "CIS-1.1.1":
		result.Status = StatusCompliant
		result.Evidence = "文件系统使用安全挂载选项"
		result.ActualValue = "noexec,nosuid,nodev"
		result.ExpectedValue = "noexec,nosuid,nodev"
	case "CIS-2.1.1":
		result.Status = StatusCompliant
		result.Evidence = "不必要的服务已禁用"
	case "CIS-3.2.1":
		result.Status = StatusCompliant
		result.Evidence = "防火墙已启用"
	case "CIS-4.1.1":
		result.Status = StatusCompliant
		result.Evidence = "审计日志已启用"
	case "CIS-5.1.1":
		result.Status = StatusNonCompliant
		result.Evidence = "SSH 允许 root 登录"
		result.ActualValue = "PermitRootLogin yes"
		result.ExpectedValue = "PermitRootLogin no"
		result.Remediation = check.Remediation
	case "CIS-7.1.1":
		result.Status = StatusCompliant
		result.Evidence = "数据加密已启用"
	default:
		result.Status = StatusCompliant
		result.Evidence = "检查通过"
	}

	return result
}

// RunSTIGCheck 运行 STIG 合规检查
func (s *ComplianceScanner) RunSTIGCheck() *ComplianceStandardReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := &ComplianceStandardReport{
		ID:        fmt.Sprintf("stig-report-%d", time.Now().UnixNano()),
		Standard:  StandardSTIG,
		CheckedAt: time.Now(),
	}

	var results []ComplianceCheckResult
	for _, stigCheck := range s.stigChecks {
		result := s.executeSTIGCheck(stigCheck)
		results = append(results, result)
	}

	report.Results = results
	report.Summary = s.calculateSummary(results)

	return report
}

// executeSTIGCheck 执行单个 STIG 检查
func (s *ComplianceScanner) executeSTIGCheck(stigCheck *STIGCheck) ComplianceCheckResult {
	result := ComplianceCheckResult{
		CheckID:   stigCheck.ID,
		Standard:  StandardSTIG,
		Title:     stigCheck.Title,
		CheckedAt: time.Now(),
	}

	// 模拟检查逻辑
	switch stigCheck.ID {
	case "STIG-V-230221":
		result.Status = StatusCompliant
		result.Evidence = "登录警告已配置"
	case "STIG-V-230222":
		result.Status = StatusCompliant
		result.Evidence = "不必要的文件系统已禁用"
	case "STIG-V-230223":
		result.Status = StatusNonCompliant
		result.Evidence = "USB 存储设备未禁用"
		result.Remediation = stigCheck.FixText
	case "STIG-V-230224":
		result.Status = StatusCompliant
		result.Evidence = "SELinux 已启用并设置为 enforcing"
	case "STIG-V-230225":
		result.Status = StatusCompliant
		result.Evidence = "SSH 使用 FIPS 批准的算法"
	default:
		result.Status = StatusCompliant
	}

	return result
}

// RunGDPRCheck 运行 GDPR 合规检查
func (s *ComplianceScanner) RunGDPRCheck() *ComplianceStandardReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := &ComplianceStandardReport{
		ID:        fmt.Sprintf("gdpr-report-%d", time.Now().UnixNano()),
		Standard:  StandardGDPR,
		CheckedAt: time.Now(),
	}

	var results []ComplianceCheckResult

	// 检查 GDPR 条款
	for articleID, article := range s.gdprArticles {
		result := s.executeGDPRCheck(articleID, article)
		results = append(results, result)
	}

	// 添加额外的 GDPR 检查
	gdprChecks := []*ComplianceCheck{
		{
			ID: "GDPR-DATA-001", Standard: StandardGDPR, Category: CategoryDataProtection,
			Title: "个人数据加密", Description: "确保所有个人数据已加密存储",
			Severity: "critical",
		},
		{
			ID: "GDPR-DATA-002", Standard: StandardGDPR, Category: CategoryDataProtection,
			Title: "数据保留期限", Description: "确保数据保留期限已配置",
			Severity: "high",
		},
		{
			ID: "GDPR-PRIV-001", Standard: StandardGDPR, Category: CategoryPrivacy,
			Title: "隐私政策", Description: "确保隐私政策已发布并可访问",
			Severity: "high",
		},
		{
			ID: "GDPR-PRIV-002", Standard: StandardGDPR, Category: CategoryPrivacy,
			Title: "数据主体权利", Description: "确保数据主体权利请求处理机制已就绪",
			Severity: "critical",
		},
		{
			ID: "GDPR-ENCR-001", Standard: StandardGDPR, Category: CategoryEncryption,
			Title: "传输加密", Description: "确保数据传输使用 TLS 1.2+",
			Severity: "critical",
		},
	}

	for _, check := range gdprChecks {
		result := s.executeGDPRDataCheck(check)
		results = append(results, result)
	}

	report.Results = results
	report.Summary = s.calculateSummary(results)

	return report
}

// executeGDPRCheck 执行 GDPR 条款检查
func (s *ComplianceScanner) executeGDPRCheck(articleID string, article *GDPRArticle) ComplianceCheckResult {
	result := ComplianceCheckResult{
		CheckID:   articleID,
		Standard:  StandardGDPR,
		Category:  CategoryPrivacy,
		Title:     article.Title,
		CheckedAt: time.Now(),
	}

	// 模拟检查逻辑
	switch articleID {
	case "Article-5":
		result.Status = StatusCompliant
		result.Evidence = "数据处理遵循基本原理"
	case "Article-17":
		result.Status = StatusCompliant
		result.Evidence = "数据删除机制已实现"
	case "Article-25":
		result.Status = StatusPartial
		result.Evidence = "部分数据保护设计已实现"
		result.Remediation = "建议实施更多隐私默认设置"
	case "Article-32":
		result.Status = StatusCompliant
		result.Evidence = "数据加密和安全措施已实施"
	case "Article-33":
		result.Status = StatusNonCompliant
		result.Evidence = "数据泄露通知机制未完全实现"
		result.Remediation = "实施 72 小时内数据泄露通知机制"
	case "Article-35":
		result.Status = StatusCompliant
		result.Evidence = "数据保护影响评估已进行"
	default:
		result.Status = StatusCompliant
	}

	return result
}

// executeGDPRDataCheck 执行 GDPR 数据保护检查
func (s *ComplianceScanner) executeGDPRDataCheck(check *ComplianceCheck) ComplianceCheckResult {
	result := ComplianceCheckResult{
		CheckID:   check.ID,
		Standard:  check.Standard,
		Category:  check.Category,
		Title:     check.Title,
		CheckedAt: time.Now(),
	}

	// 模拟检查逻辑
	switch check.ID {
	case "GDPR-DATA-001":
		result.Status = StatusCompliant
		result.Evidence = "个人数据使用 AES-256-GCM 加密"
	case "GDPR-DATA-002":
		result.Status = StatusCompliant
		result.Evidence = "数据保留期限已配置为 365 天"
	case "GDPR-PRIV-001":
		result.Status = StatusCompliant
		result.Evidence = "隐私政策已发布"
	case "GDPR-PRIV-002":
		result.Status = StatusNonCompliant
		result.Evidence = "数据主体权利请求处理机制未完全实现"
		result.Remediation = "实现数据访问、更正、删除和可携带性请求的自动化处理"
	case "GDPR-ENCR-001":
		result.Status = StatusCompliant
		result.Evidence = "所有传输使用 TLS 1.3"
	default:
		result.Status = StatusCompliant
	}

	return result
}

// calculateSummary 计算检查摘要
func (s *ComplianceScanner) calculateSummary(results []ComplianceCheckResult) ComplianceSummary {
	summary := ComplianceSummary{
		TotalChecks: len(results),
	}

	for _, result := range results {
		switch result.Status {
		case StatusCompliant:
			summary.Compliant++
		case StatusNonCompliant:
			summary.NonCompliant++
		case StatusPartial:
			summary.Partial++
		case StatusNotApplicable:
			summary.NotApplicable++
		case StatusError:
			summary.Errors++
		}
	}

	if summary.TotalChecks > 0 {
		summary.ComplianceRate = float64(summary.Compliant) / float64(summary.TotalChecks) * 100
	}

	return summary
}

// ComplianceStandardReport 合规标准报告
type ComplianceStandardReport struct {
	ID        string               `json:"id"`
	Standard  ComplianceStandard   `json:"standard"`
	Benchmark *CISBenchmark        `json:"benchmark,omitempty"`
	Results   []ComplianceCheckResult `json:"results"`
	Summary   ComplianceSummary    `json:"summary"`
	CheckedAt time.Time            `json:"checked_at"`
}

// ComplianceSummary 合规摘要
type ComplianceSummary struct {
	TotalChecks    int     `json:"total_checks"`
	Compliant      int     `json:"compliant"`
	NonCompliant   int     `json:"non_compliant"`
	Partial        int     `json:"partial"`
	NotApplicable  int     `json:"not_applicable"`
	Errors         int     `json:"errors"`
	ComplianceRate float64 `json:"compliance_rate"`
}

// RunFullComplianceScan 运行完整合规扫描
func (s *ComplianceScanner) RunFullComplianceScan() *FullComplianceReport {
	s.mu.Lock()
	s.lastScanTime = time.Now()
	s.mu.Unlock()

	report := &FullComplianceReport{
		ID:        fmt.Sprintf("full-compliance-%d", time.Now().UnixNano()),
		CheckedAt: time.Now(),
	}

	// 运行所有标准检查
	report.CISReport = s.RunCISCheck()
	report.STIGReport = s.RunSTIGCheck()
	report.GDPRReport = s.RunGDPRCheck()

	// 计算总体合规分数
	report.OverallScore = s.calculateOverallScore(report)

	// 生成建议
	report.Recommendations = s.generateFullRecommendations(report)

	return report
}

// FullComplianceReport 完整合规报告
type FullComplianceReport struct {
	ID              string                    `json:"id"`
	CISReport       *ComplianceStandardReport `json:"cis_report"`
	STIGReport      *ComplianceStandardReport `json:"stig_report"`
	GDPRReport      *ComplianceStandardReport `json:"gdpr_report"`
	OverallScore    float64                   `json:"overall_score"`
	Recommendations []string                  `json:"recommendations"`
	CheckedAt       time.Time                 `json:"checked_at"`
}

// calculateOverallScore 计算总体合规分数
func (s *ComplianceScanner) calculateOverallScore(report *FullComplianceReport) float64 {
	totalChecks := 0
	totalCompliant := 0

	if report.CISReport != nil {
		totalChecks += report.CISReport.Summary.TotalChecks
		totalCompliant += report.CISReport.Summary.Compliant
	}

	if report.STIGReport != nil {
		totalChecks += report.STIGReport.Summary.TotalChecks
		totalCompliant += report.STIGReport.Summary.Compliant
	}

	if report.GDPRReport != nil {
		totalChecks += report.GDPRReport.Summary.TotalChecks
		totalCompliant += report.GDPRReport.Summary.Compliant
	}

	if totalChecks == 0 {
		return 0
	}

	return float64(totalCompliant) / float64(totalChecks) * 100
}

// generateFullRecommendations 生成完整合规建议
func (s *ComplianceScanner) generateFullRecommendations(report *FullComplianceReport) []string {
	var recommendations []string

	// CIS 建议
	if report.CISReport != nil {
		for _, result := range report.CISReport.Results {
			if result.Status == StatusNonCompliant {
				recommendations = append(recommendations, fmt.Sprintf("[CIS] %s: %s", result.Title, result.Remediation))
			}
		}
	}

	// STIG 建议
	if report.STIGReport != nil {
		for _, result := range report.STIGReport.Results {
			if result.Status == StatusNonCompliant {
				recommendations = append(recommendations, fmt.Sprintf("[STIG] %s: %s", result.Title, result.Remediation))
			}
		}
	}

	// GDPR 建议
	if report.GDPRReport != nil {
		for _, result := range report.GDPRReport.Results {
			if result.Status == StatusNonCompliant || result.Status == StatusPartial {
				recommendations = append(recommendations, fmt.Sprintf("[GDPR] %s: %s", result.Title, result.Remediation))
			}
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统符合所有合规要求，建议定期执行合规扫描")
	}

	return recommendations
}

// GetComplianceChecks 获取所有合规检查项
func (s *ComplianceScanner) GetComplianceChecks() []*ComplianceCheck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checks []*ComplianceCheck
	for _, check := range s.checks {
		checks = append(checks, check)
	}

	return checks
}

// GetCISBenchmark 获取 CIS 基准配置
func (s *ComplianceScanner) GetCISBenchmark() *CISBenchmark {
	return s.cisBenchmark
}

// GetSTIGChecks 获取 STIG 检查配置
func (s *ComplianceScanner) GetSTIGChecks() []*STIGCheck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checks []*STIGCheck
	for _, check := range s.stigChecks {
		checks = append(checks, check)
	}

	return checks
}

// GetGDPRArticles 获取 GDPR 条款
func (s *ComplianceScanner) GetGDPRArticles() []*GDPRArticle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var articles []*GDPRArticle
	for _, article := range s.gdprArticles {
		articles = append(articles, article)
	}

	return articles
}
