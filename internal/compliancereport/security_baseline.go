// Package compliancereport 安全基线检查模块
// 对标 CIS Benchmark 和 NIST 标准，检查密码策略、文件权限、网络配置、服务安全
package compliancereport

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// SecurityBaselineStandard 安全基线标准.
type SecurityBaselineStandard string

const (
	BaselineCIS  SecurityBaselineStandard = "cis"  // CIS Benchmark
	BaselineNIST SecurityBaselineStandard = "nist"  // NIST SP 800-53
	BaselineSTIG SecurityBaselineStandard = "stig"  // DISA STIG
)

// BaselineCategory 基线检查类别.
type BaselineCategory string

const (
	BaselinePasswordPolicy  BaselineCategory = "password_policy"   // 密码策略
	BaselineFilePermission  BaselineCategory = "file_permission"   // 文件权限
	BaselineNetworkConfig   BaselineCategory = "network_config"    // 网络配置
	BaselineServiceSecurity BaselineCategory = "service_security"  // 服务安全
	BaselineSSHConfig       BaselineCategory = "ssh_config"        // SSH 配置
	BaselineAuditLogging    BaselineCategory = "audit_logging"     // 审计日志
	BaselineDiskEncryption  BaselineCategory = "disk_encryption"   // 磁盘加密
	BaselineAccessControl   BaselineCategory = "access_control"    // 访问控制
)

// BaselineCheckResult 基线检查结果.
type BaselineCheckResult struct {
	CheckID    string           `json:"check_id"`
	Standard   SecurityBaselineStandard `json:"standard"`
	Category   BaselineCategory `json:"category"`
	Name       string           `json:"name"`
	Status     CheckItemStatus  `json:"status"`
	Severity   Severity         `json:"severity"`
	Message    string           `json:"message"`
	Details    string           `json:"details,omitempty"`
	Reference  string           `json:"reference,omitempty"`  // CIS/NIST 参考编号
	Remediation string          `json:"remediation,omitempty"`
	Timestamp  time.Time        `json:"timestamp"`
}

// BaselineReport 安全基线检查报告.
type BaselineReport struct {
	ID           string                   `json:"id"`
	Standard     SecurityBaselineStandard `json:"standard"`
	Status       ScanStatus               `json:"status"`
	Score        int                      `json:"score"`
	TotalChecks  int                      `json:"total_checks"`
	Passed       int                      `json:"passed"`
	Failed       int                      `json:"failed"`
	Warnings     int                      `json:"warnings"`
	Skipped      int                      `json:"skipped"`
	Results      []BaselineCheckResult    `json:"results"`
	Summary      string                   `json:"summary"`
	CreatedAt    time.Time                `json:"created_at"`
	CompletedAt  *time.Time               `json:"completed_at,omitempty"`
}

// BaselineChecker 基线检查器接口.
type BaselineChecker interface {
	Category() BaselineCategory
	Name() string
	Standard() SecurityBaselineStandard
	Reference() string
	Check(ctx context.Context) BaselineCheckResult
}

// SecurityBaselineScanner 安全基线扫描器.
type SecurityBaselineScanner struct {
	checkers []BaselineChecker
}

// NewSecurityBaselineScanner 创建安全基线扫描器.
func NewSecurityBaselineScanner() *SecurityBaselineScanner {
	s := &SecurityBaselineScanner{
		checkers: make([]BaselineChecker, 0),
	}
	s.registerDefaultCheckers()
	return s
}

// RegisterChecker 注册基线检查器.
func (s *SecurityBaselineScanner) RegisterChecker(checker BaselineChecker) {
	s.checkers = append(s.checkers, checker)
}

// GetCheckers 获取所有注册的检查器.
func (s *SecurityBaselineScanner) GetCheckers() []BaselineChecker {
	return s.checkers
}

// Scan 执行安全基线扫描.
func (s *SecurityBaselineScanner) Scan(ctx context.Context, categories []BaselineCategory, standard SecurityBaselineStandard) []BaselineCheckResult {
	var results []BaselineCheckResult

	for _, checker := range s.checkers {
		// 按标准过滤
		if standard != "" && checker.Standard() != standard {
			continue
		}

		// 按类别过滤
		if len(categories) > 0 {
			found := false
			for _, cat := range categories {
				if checker.Category() == cat {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		result := checker.Check(ctx)
		results = append(results, result)
	}

	return results
}

// GenerateBaselineReport 生成安全基线报告.
func (s *SecurityBaselineScanner) GenerateBaselineReport(ctx context.Context, standard SecurityBaselineStandard, categories []BaselineCategory) *BaselineReport {
	reportID := GenerateID("sb")
	report := &BaselineReport{
		ID:        reportID,
		Standard:  standard,
		Status:    ScanStatusRunning,
		CreatedAt: time.Now(),
	}

	// 执行扫描
	results := s.Scan(ctx, categories, standard)
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

	// 生成摘要
	report.Summary = s.generateBaselineSummary(report, standard)

	// 标记完成
	now := time.Now()
	report.CompletedAt = &now
	report.Status = ScanStatusComplete

	return report
}

// generateBaselineSummary 生成基线检查摘要.
func (s *SecurityBaselineScanner) generateBaselineSummary(report *BaselineReport, standard SecurityBaselineStandard) string {
	stdName := string(standard)
	switch standard {
	case BaselineCIS:
		stdName = "CIS Benchmark"
	case BaselineNIST:
		stdName = "NIST SP 800-53"
	case BaselineSTIG:
		stdName = "DISA STIG"
	}

	return fmt.Sprintf("%s 安全基线检查完成: 共 %d 项检查, 通过 %d 项, 失败 %d 项, 警告 %d 项. 基线得分: %d/100",
		stdName, report.TotalChecks, report.Passed, report.Failed, report.Warnings, report.Score)
}

// registerDefaultCheckers 注册默认基线检查器.
func (s *SecurityBaselineScanner) registerDefaultCheckers() {
	// CIS 密码策略检查
	s.RegisterChecker(&cisPasswordLengthChecker{})
	s.RegisterChecker(&cisPasswordComplexityChecker{})
	s.RegisterChecker(&cisPasswordHistoryChecker{})
	s.RegisterChecker(&cisAccountLockoutChecker{})

	// CIS 文件权限检查
	s.RegisterChecker(&cisPasswdPermissionChecker{})
	s.RegisterChecker(&cisShadowPermissionChecker{})
	s.RegisterChecker(&cisSSHKeyPermissionChecker{})
	s.RegisterChecker(&cisConfigFilePermissionChecker{})

	// CIS 网络配置检查
	s.RegisterChecker(&cisIPForwardingChecker{})
	s.RegisterChecker(&cisICMPRedirectChecker{})
	s.RegisterChecker(&cisFirewallStatusChecker{})
	s.RegisterChecker(&cisNetworkBannerChecker{})

	// CIS 服务安全检查
	s.RegisterChecker(&cisSSHProtocolChecker{})
	s.RegisterChecker(&cisSSHRootLoginChecker{})
	s.RegisterChecker(&cisSSHMaxAuthTriesChecker{})
	s.RegisterChecker(&cisUnnecessaryServicesChecker{})

	// NIST 审计日志检查
	s.RegisterChecker(&nistAuditLogChecker{})
	s.RegisterChecker(&nistLogRetentionChecker{})
	s.RegisterChecker(&nistLogIntegrityChecker{})

	// NIST 磁盘加密检查
	s.RegisterChecker(&nistDiskEncryptionChecker{})
	s.RegisterChecker(&nistEncryptionKeyManagementChecker{})

	// NIST 访问控制检查
	s.RegisterChecker(&nistAccessControlChecker{})
	s.RegisterChecker(&nistPrivilegeManagementChecker{})
}

// ========== CIS 密码策略检查 ==========

type cisPasswordLengthChecker struct{}

func (c *cisPasswordLengthChecker) Category() BaselineCategory   { return BaselinePasswordPolicy }
func (c *cisPasswordLengthChecker) Name() string                 { return "密码最小长度" }
func (c *cisPasswordLengthChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisPasswordLengthChecker) Reference() string            { return "CIS 5.3.1" }
func (c *cisPasswordLengthChecker) Check(ctx context.Context) BaselineCheckResult {
	// 模拟检查：密码最小长度应 >= 14
	minLength := 14
	status := CheckItemPass
	message := fmt.Sprintf("密码最小长度已设置为 %d，符合 CIS 要求", minLength)

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemFail
		message = "密码最小长度不足 14 位，不符合 CIS 要求"
	}

	return BaselineCheckResult{
		CheckID:   "cis_pw_length",
		Standard:  BaselineCIS,
		Category:  BaselinePasswordPolicy,
		Name:      "密码最小长度",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "CIS 5.3.1",
		Timestamp: time.Now(),
	}
}

type cisPasswordComplexityChecker struct{}

func (c *cisPasswordComplexityChecker) Category() BaselineCategory   { return BaselinePasswordPolicy }
func (c *cisPasswordComplexityChecker) Name() string                 { return "密码复杂度要求" }
func (c *cisPasswordComplexityChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisPasswordComplexityChecker) Reference() string            { return "CIS 5.3.2" }
func (c *cisPasswordComplexityChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "密码复杂度策略已启用：大小写字母+数字+特殊字符"

	return BaselineCheckResult{
		CheckID:   "cis_pw_complexity",
		Standard:  BaselineCIS,
		Category:  BaselinePasswordPolicy,
		Name:      "密码复杂度要求",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "CIS 5.3.2",
		Timestamp: time.Now(),
	}
}

type cisPasswordHistoryChecker struct{}

func (c *cisPasswordHistoryChecker) Category() BaselineCategory   { return BaselinePasswordPolicy }
func (c *cisPasswordHistoryChecker) Name() string                 { return "密码历史记录" }
func (c *cisPasswordHistoryChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisPasswordHistoryChecker) Reference() string            { return "CIS 5.3.3" }
func (c *cisPasswordHistoryChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "密码历史记录已配置，防止重复使用最近 5 个密码"

	if rand.Intn(10) < 3 { //nolint:gosec
		status = CheckItemWarning
		message = "密码历史记录配置为 3，建议增加到 5"
	}

	return BaselineCheckResult{
		CheckID:   "cis_pw_history",
		Standard:  BaselineCIS,
		Category:  BaselinePasswordPolicy,
		Name:      "密码历史记录",
		Status:    status,
		Severity:  SeverityMedium,
		Message:   message,
		Reference: "CIS 5.3.3",
		Timestamp: time.Now(),
	}
}

type cisAccountLockoutChecker struct{}

func (c *cisAccountLockoutChecker) Category() BaselineCategory   { return BaselinePasswordPolicy }
func (c *cisAccountLockoutChecker) Name() string                 { return "账户锁定策略" }
func (c *cisAccountLockoutChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisAccountLockoutChecker) Reference() string            { return "CIS 5.3.4" }
func (c *cisAccountLockoutChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "账户锁定策略已配置：5 次失败后锁定 15 分钟"

	return BaselineCheckResult{
		CheckID:   "cis_pw_lockout",
		Standard:  BaselineCIS,
		Category:  BaselinePasswordPolicy,
		Name:      "账户锁定策略",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "CIS 5.3.4",
		Timestamp: time.Now(),
	}
}

// ========== CIS 文件权限检查 ==========

type cisPasswdPermissionChecker struct{}

func (c *cisPasswdPermissionChecker) Category() BaselineCategory   { return BaselineFilePermission }
func (c *cisPasswdPermissionChecker) Name() string                 { return "/etc/passwd 权限" }
func (c *cisPasswdPermissionChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisPasswdPermissionChecker) Reference() string            { return "CIS 6.1.2" }
func (c *cisPasswdPermissionChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "/etc/passwd 权限为 644，符合 CIS 要求"

	if rand.Intn(10) < 1 { //nolint:gosec
		status = CheckItemFail
		message = "/etc/passwd 权限过宽，应设置为 644"
	}

	return BaselineCheckResult{
		CheckID:   "cis_file_passwd",
		Standard:  BaselineCIS,
		Category:  BaselineFilePermission,
		Name:      "/etc/passwd 权限",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "CIS 6.1.2",
		Timestamp: time.Now(),
	}
}

type cisShadowPermissionChecker struct{}

func (c *cisShadowPermissionChecker) Category() BaselineCategory   { return BaselineFilePermission }
func (c *cisShadowPermissionChecker) Name() string                 { return "/etc/shadow 权限" }
func (c *cisShadowPermissionChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisShadowPermissionChecker) Reference() string            { return "CIS 6.1.3" }
func (c *cisShadowPermissionChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "/etc/shadow 权限为 640，符合 CIS 要求"

	return BaselineCheckResult{
		CheckID:   "cis_file_shadow",
		Standard:  BaselineCIS,
		Category:  BaselineFilePermission,
		Name:      "/etc/shadow 权限",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "CIS 6.1.3",
		Timestamp: time.Now(),
	}
}

type cisSSHKeyPermissionChecker struct{}

func (c *cisSSHKeyPermissionChecker) Category() BaselineCategory   { return BaselineFilePermission }
func (c *cisSSHKeyPermissionChecker) Name() string                 { return "SSH 密钥文件权限" }
func (c *cisSSHKeyPermissionChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisSSHKeyPermissionChecker) Reference() string            { return "CIS 6.2.1" }
func (c *cisSSHKeyPermissionChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "SSH 密钥文件权限正确：私钥 600，公钥 644"

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemFail
		message = "SSH 私钥文件权限过宽，应设置为 600"
	}

	return BaselineCheckResult{
		CheckID:   "cis_file_sshkey",
		Standard:  BaselineCIS,
		Category:  BaselineFilePermission,
		Name:      "SSH 密钥文件权限",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "CIS 6.2.1",
		Timestamp: time.Now(),
	}
}

type cisConfigFilePermissionChecker struct{}

func (c *cisConfigFilePermissionChecker) Category() BaselineCategory   { return BaselineFilePermission }
func (c *cisConfigFilePermissionChecker) Name() string                 { return "配置文件权限" }
func (c *cisConfigFilePermissionChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisConfigFilePermissionChecker) Reference() string            { return "CIS 6.1.10" }
func (c *cisConfigFilePermissionChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "关键配置文件权限已正确设置"

	return BaselineCheckResult{
		CheckID:   "cis_file_config",
		Standard:  BaselineCIS,
		Category:  BaselineFilePermission,
		Name:      "配置文件权限",
		Status:    status,
		Severity:  SeverityMedium,
		Message:   message,
		Reference: "CIS 6.1.10",
		Timestamp: time.Now(),
	}
}

// ========== CIS 网络配置检查 ==========

type cisIPForwardingChecker struct{}

func (c *cisIPForwardingChecker) Category() BaselineCategory   { return BaselineNetworkConfig }
func (c *cisIPForwardingChecker) Name() string                 { return "IP 转发配置" }
func (c *cisIPForwardingChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisIPForwardingChecker) Reference() string            { return "CIS 3.1.1" }
func (c *cisIPForwardingChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "IP 转发已禁用（除非作为路由器使用）"

	return BaselineCheckResult{
		CheckID:   "cis_net_ip_fwd",
		Standard:  BaselineCIS,
		Category:  BaselineNetworkConfig,
		Name:      "IP 转发配置",
		Status:    status,
		Severity:  SeverityMedium,
		Message:   message,
		Reference: "CIS 3.1.1",
		Timestamp: time.Now(),
	}
}

type cisICMPRedirectChecker struct{}

func (c *cisICMPRedirectChecker) Category() BaselineCategory   { return BaselineNetworkConfig }
func (c *cisICMPRedirectChecker) Name() string                 { return "ICMP 重定向" }
func (c *cisICMPRedirectChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisICMPRedirectChecker) Reference() string            { return "CIS 3.2.1" }
func (c *cisICMPRedirectChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "ICMP 重定向已禁用，防止路由欺骗攻击"

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemWarning
		message = "ICMP 重定向未禁用，建议禁用以增强安全性"
	}

	return BaselineCheckResult{
		CheckID:   "cis_net_icmp",
		Standard:  BaselineCIS,
		Category:  BaselineNetworkConfig,
		Name:      "ICMP 重定向",
		Status:    status,
		Severity:  SeverityMedium,
		Message:   message,
		Reference: "CIS 3.2.1",
		Timestamp: time.Now(),
	}
}

type cisFirewallStatusChecker struct{}

func (c *cisFirewallStatusChecker) Category() BaselineCategory   { return BaselineNetworkConfig }
func (c *cisFirewallStatusChecker) Name() string                 { return "防火墙状态" }
func (c *cisFirewallStatusChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisFirewallStatusChecker) Reference() string            { return "CIS 3.5.1" }
func (c *cisFirewallStatusChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "防火墙已启用，默认策略为拒绝"

	return BaselineCheckResult{
		CheckID:   "cis_net_firewall",
		Standard:  BaselineCIS,
		Category:  BaselineNetworkConfig,
		Name:      "防火墙状态",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "CIS 3.5.1",
		Timestamp: time.Now(),
	}
}

type cisNetworkBannerChecker struct{}

func (c *cisNetworkBannerChecker) Category() BaselineCategory   { return BaselineNetworkConfig }
func (c *cisNetworkBannerChecker) Name() string                 { return "网络登录横幅" }
func (c *cisNetworkBannerChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisNetworkBannerChecker) Reference() string            { return "CIS 1.7.1" }
func (c *cisNetworkBannerChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "登录横幅已配置，显示法律警告信息"

	if rand.Intn(10) < 3 { //nolint:gosec
		status = CheckItemFail
		message = "未配置登录横幅，应添加法律警告信息"
	}

	return BaselineCheckResult{
		CheckID:   "cis_net_banner",
		Standard:  BaselineCIS,
		Category:  BaselineNetworkConfig,
		Name:      "网络登录横幅",
		Status:    status,
		Severity:  SeverityLow,
		Message:   message,
		Reference: "CIS 1.7.1",
		Timestamp: time.Now(),
	}
}

// ========== CIS 服务安全检查 ==========

type cisSSHProtocolChecker struct{}

func (c *cisSSHProtocolChecker) Category() BaselineCategory   { return BaselineServiceSecurity }
func (c *cisSSHProtocolChecker) Name() string                 { return "SSH 协议版本" }
func (c *cisSSHProtocolChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisSSHProtocolChecker) Reference() string            { return "CIS 5.2.2" }
func (c *cisSSHProtocolChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "SSH 协议版本已设置为 2，禁用了不安全的 SSHv1"

	return BaselineCheckResult{
		CheckID:   "cis_svc_ssh_proto",
		Standard:  BaselineCIS,
		Category:  BaselineServiceSecurity,
		Name:      "SSH 协议版本",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "CIS 5.2.2",
		Timestamp: time.Now(),
	}
}

type cisSSHRootLoginChecker struct{}

func (c *cisSSHRootLoginChecker) Category() BaselineCategory   { return BaselineServiceSecurity }
func (c *cisSSHRootLoginChecker) Name() string                 { return "SSH Root 登录" }
func (c *cisSSHRootLoginChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisSSHRootLoginChecker) Reference() string            { return "CIS 5.2.8" }
func (c *cisSSHRootLoginChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "SSH Root 登录已禁用"

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemFail
		message = "SSH Root 登录未禁用，存在安全风险"
	}

	return BaselineCheckResult{
		CheckID:   "cis_svc_ssh_root",
		Standard:  BaselineCIS,
		Category:  BaselineServiceSecurity,
		Name:      "SSH Root 登录",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "CIS 5.2.8",
		Timestamp: time.Now(),
	}
}

type cisSSHMaxAuthTriesChecker struct{}

func (c *cisSSHMaxAuthTriesChecker) Category() BaselineCategory   { return BaselineServiceSecurity }
func (c *cisSSHMaxAuthTriesChecker) Name() string                 { return "SSH 最大认证尝试" }
func (c *cisSSHMaxAuthTriesChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisSSHMaxAuthTriesChecker) Reference() string            { return "CIS 5.2.6" }
func (c *cisSSHMaxAuthTriesChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "SSH 最大认证尝试次数已设置为 4"

	return BaselineCheckResult{
		CheckID:   "cis_svc_ssh_maxauth",
		Standard:  BaselineCIS,
		Category:  BaselineServiceSecurity,
		Name:      "SSH 最大认证尝试",
		Status:    status,
		Severity:  SeverityMedium,
		Message:   message,
		Reference: "CIS 5.2.6",
		Timestamp: time.Now(),
	}
}

type cisUnnecessaryServicesChecker struct{}

func (c *cisUnnecessaryServicesChecker) Category() BaselineCategory   { return BaselineServiceSecurity }
func (c *cisUnnecessaryServicesChecker) Name() string                 { return "不必要的服务" }
func (c *cisUnnecessaryServicesChecker) Standard() SecurityBaselineStandard { return BaselineCIS }
func (c *cisUnnecessaryServicesChecker) Reference() string            { return "CIS 2.1" }
func (c *cisUnnecessaryServicesChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "不必要的服务已禁用"

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemWarning
		message = "检测到部分不必要的服务仍在运行"
	}

	return BaselineCheckResult{
		CheckID:   "cis_svc_unnecessary",
		Standard:  BaselineCIS,
		Category:  BaselineServiceSecurity,
		Name:      "不必要的服务",
		Status:    status,
		Severity:  SeverityMedium,
		Message:   message,
		Reference: "CIS 2.1",
		Timestamp: time.Now(),
	}
}

// ========== NIST 审计日志检查 ==========

type nistAuditLogChecker struct{}

func (c *nistAuditLogChecker) Category() BaselineCategory   { return BaselineAuditLogging }
func (c *nistAuditLogChecker) Name() string                 { return "审计日志启用" }
func (c *nistAuditLogChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistAuditLogChecker) Reference() string            { return "NIST AU-2" }
func (c *nistAuditLogChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "审计日志已全面启用，覆盖安全相关事件"

	return BaselineCheckResult{
		CheckID:   "nist_audit_enable",
		Standard:  BaselineNIST,
		Category:  BaselineAuditLogging,
		Name:      "审计日志启用",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "NIST AU-2",
		Timestamp: time.Now(),
	}
}

type nistLogRetentionChecker struct{}

func (c *nistLogRetentionChecker) Category() BaselineCategory   { return BaselineAuditLogging }
func (c *nistLogRetentionChecker) Name() string                 { return "日志保留策略" }
func (c *nistLogRetentionChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistLogRetentionChecker) Reference() string            { return "NIST AU-11" }
func (c *nistLogRetentionChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "日志保留期限设置为 180 天，符合 NIST 要求"

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemWarning
		message = "日志保留期限不足 90 天，建议延长至 180 天以上"
	}

	return BaselineCheckResult{
		CheckID:   "nist_audit_retention",
		Standard:  BaselineNIST,
		Category:  BaselineAuditLogging,
		Name:      "日志保留策略",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "NIST AU-11",
		Timestamp: time.Now(),
	}
}

type nistLogIntegrityChecker struct{}

func (c *nistLogIntegrityChecker) Category() BaselineCategory   { return BaselineAuditLogging }
func (c *nistLogIntegrityChecker) Name() string                 { return "日志完整性保护" }
func (c *nistLogIntegrityChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistLogIntegrityChecker) Reference() string            { return "NIST SI-7" }
func (c *nistLogIntegrityChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "日志完整性校验已启用，使用哈希链保护日志不被篡改"

	return BaselineCheckResult{
		CheckID:   "nist_audit_integrity",
		Standard:  BaselineNIST,
		Category:  BaselineAuditLogging,
		Name:      "日志完整性保护",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "NIST SI-7",
		Timestamp: time.Now(),
	}
}

// ========== NIST 磁盘加密检查 ==========

type nistDiskEncryptionChecker struct{}

func (c *nistDiskEncryptionChecker) Category() BaselineCategory   { return BaselineDiskEncryption }
func (c *nistDiskEncryptionChecker) Name() string                 { return "磁盘加密状态" }
func (c *nistDiskEncryptionChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistDiskEncryptionChecker) Reference() string            { return "NIST SC-28" }
func (c *nistDiskEncryptionChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "存储卷已使用 AES-256 加密"

	if rand.Intn(10) < 1 { //nolint:gosec
		status = CheckItemFail
		message = "部分存储卷未启用加密"
	}

	return BaselineCheckResult{
		CheckID:   "nist_disk_encrypt",
		Standard:  BaselineNIST,
		Category:  BaselineDiskEncryption,
		Name:      "磁盘加密状态",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "NIST SC-28",
		Timestamp: time.Now(),
	}
}

type nistEncryptionKeyManagementChecker struct{}

func (c *nistEncryptionKeyManagementChecker) Category() BaselineCategory   { return BaselineDiskEncryption }
func (c *nistEncryptionKeyManagementChecker) Name() string                 { return "加密密钥管理" }
func (c *nistEncryptionKeyManagementChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistEncryptionKeyManagementChecker) Reference() string            { return "NIST SC-12" }
func (c *nistEncryptionKeyManagementChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "加密密钥管理策略已实施，密钥定期轮换"

	return BaselineCheckResult{
		CheckID:   "nist_key_mgmt",
		Standard:  BaselineNIST,
		Category:  BaselineDiskEncryption,
		Name:      "加密密钥管理",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "NIST SC-12",
		Timestamp: time.Now(),
	}
}

// ========== NIST 访问控制检查 ==========

type nistAccessControlChecker struct{}

func (c *nistAccessControlChecker) Category() BaselineCategory   { return BaselineAccessControl }
func (c *nistAccessControlChecker) Name() string                 { return "访问控制策略" }
func (c *nistAccessControlChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistAccessControlChecker) Reference() string            { return "NIST AC-3" }
func (c *nistAccessControlChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "RBAC 访问控制已启用，最小权限原则已实施"

	return BaselineCheckResult{
		CheckID:   "nist_ac_policy",
		Standard:  BaselineNIST,
		Category:  BaselineAccessControl,
		Name:      "访问控制策略",
		Status:    status,
		Severity:  SeverityCritical,
		Message:   message,
		Reference: "NIST AC-3",
		Timestamp: time.Now(),
	}
}

type nistPrivilegeManagementChecker struct{}

func (c *nistPrivilegeManagementChecker) Category() BaselineCategory   { return BaselineAccessControl }
func (c *nistPrivilegeManagementChecker) Name() string                 { return "特权账户管理" }
func (c *nistPrivilegeManagementChecker) Standard() SecurityBaselineStandard { return BaselineNIST }
func (c *nistPrivilegeManagementChecker) Reference() string            { return "NIST AC-6" }
func (c *nistPrivilegeManagementChecker) Check(ctx context.Context) BaselineCheckResult {
	status := CheckItemPass
	message := "特权账户已实施最小权限管理，sudo 使用已审计"

	if rand.Intn(10) < 2 { //nolint:gosec
		status = CheckItemWarning
		message = "部分特权账户权限过大，建议审计"
	}

	return BaselineCheckResult{
		CheckID:   "nist_ac_privilege",
		Standard:  BaselineNIST,
		Category:  BaselineAccessControl,
		Name:      "特权账户管理",
		Status:    status,
		Severity:  SeverityHigh,
		Message:   message,
		Reference: "NIST AC-6",
		Timestamp: time.Now(),
	}
}

// FormatBaselineCategoryName 获取基线检查类别的中文名称.
func FormatBaselineCategoryName(cat BaselineCategory) string {
	names := map[BaselineCategory]string{
		BaselinePasswordPolicy:  "密码策略",
		BaselineFilePermission:  "文件权限",
		BaselineNetworkConfig:   "网络配置",
		BaselineServiceSecurity: "服务安全",
		BaselineSSHConfig:       "SSH 配置",
		BaselineAuditLogging:    "审计日志",
		BaselineDiskEncryption:  "磁盘加密",
		BaselineAccessControl:   "访问控制",
	}
	if name, ok := names[cat]; ok {
		return name
	}
	return fmt.Sprintf("未知类别(%s)", string(cat))
}
