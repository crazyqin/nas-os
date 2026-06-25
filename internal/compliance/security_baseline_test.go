// Package compliancereport 安全基线检查测试
package compliance

import (
	"context"
	"testing"
)

// ========== SecurityBaselineScanner 测试 ==========

func TestNewSecurityBaselineScanner(t *testing.T) {
	s := NewSecurityBaselineScanner()
	if s == nil {
		t.Fatal("NewSecurityBaselineScanner should not return nil")
	}

	checkers := s.GetCheckers()
	if len(checkers) == 0 {
		t.Error("scanner should have default checkers registered")
	}
	if len(checkers) != 23 {
		t.Errorf("expected 23 default checkers, got %d", len(checkers))
	}
}

func TestBaselineScannerCategories(t *testing.T) {
	s := NewSecurityBaselineScanner()

	categoryCount := make(map[BaselineCategory]int)
	for _, c := range s.GetCheckers() {
		categoryCount[c.Category()]++
	}

	// 检查每个类别都有检查器
	expectedCats := []BaselineCategory{
		BaselinePasswordPolicy,
		BaselineFilePermission,
		BaselineNetworkConfig,
		BaselineServiceSecurity,
		BaselineAuditLogging,
		BaselineDiskEncryption,
		BaselineAccessControl,
	}

	for _, cat := range expectedCats {
		if categoryCount[cat] == 0 {
			t.Errorf("category %s should have at least 1 checker", cat)
		}
	}
}

func TestBaselineScannerStandards(t *testing.T) {
	s := NewSecurityBaselineScanner()

	standardCount := make(map[SecurityBaselineStandard]int)
	for _, c := range s.GetCheckers() {
		standardCount[c.Standard()]++
	}

	if standardCount[BaselineCIS] == 0 {
		t.Error("should have CIS checkers")
	}
	if standardCount[BaselineNIST] == 0 {
		t.Error("should have NIST checkers")
	}
}

func TestBaselineScanAll(t *testing.T) {
	s := NewSecurityBaselineScanner()
	results := s.Scan(context.Background(), nil, "")

	if len(results) == 0 {
		t.Error("scan should return results")
	}

	for _, r := range results {
		if r.CheckID == "" {
			t.Error("result should have a check_id")
		}
		if r.Standard == "" {
			t.Error("result should have a standard")
		}
		if r.Category == "" {
			t.Error("result should have a category")
		}
		if r.Name == "" {
			t.Error("result should have a name")
		}
		if r.Status == "" {
			t.Error("result should have a status")
		}
		if r.Timestamp.IsZero() {
			t.Error("result should have a timestamp")
		}
		if r.Reference == "" {
			t.Error("result should have a reference")
		}
	}
}

func TestBaselineScanByStandard(t *testing.T) {
	s := NewSecurityBaselineScanner()

	// 只扫描 CIS
	cisResults := s.Scan(context.Background(), nil, BaselineCIS)
	if len(cisResults) == 0 {
		t.Error("CIS scan should return results")
	}
	for _, r := range cisResults {
		if r.Standard != BaselineCIS {
			t.Errorf("expected CIS standard, got %s", r.Standard)
		}
	}

	// 只扫描 NIST
	nistResults := s.Scan(context.Background(), nil, BaselineNIST)
	if len(nistResults) == 0 {
		t.Error("NIST scan should return results")
	}
	for _, r := range nistResults {
		if r.Standard != BaselineNIST {
			t.Errorf("expected NIST standard, got %s", r.Standard)
		}
	}
}

func TestBaselineScanByCategory(t *testing.T) {
	s := NewSecurityBaselineScanner()

	categories := []BaselineCategory{BaselinePasswordPolicy}
	results := s.Scan(context.Background(), categories, "")

	if len(results) == 0 {
		t.Error("password policy scan should return results")
	}

	for _, r := range results {
		if r.Category != BaselinePasswordPolicy {
			t.Errorf("expected password_policy category, got %s", r.Category)
		}
	}
}

func TestBaselineScanMultipleCategories(t *testing.T) {
	s := NewSecurityBaselineScanner()

	categories := []BaselineCategory{BaselinePasswordPolicy, BaselineNetworkConfig}
	results := s.Scan(context.Background(), categories, "")

	if len(results) == 0 {
		t.Error("scan should return results")
	}

	for _, r := range results {
		found := false
		for _, cat := range categories {
			if r.Category == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("result category %s should be in requested categories", r.Category)
		}
	}
}

func TestBaselineRegisterChecker(t *testing.T) {
	s := NewSecurityBaselineScanner()
	initialCount := len(s.GetCheckers())

	s.RegisterChecker(&mockBaselineChecker{
		cat:      BaselinePasswordPolicy,
		name:     "custom_check",
		standard: BaselineCIS,
		ref:      "CIS 99.99",
		status:   CheckItemPass,
	})

	if len(s.GetCheckers()) != initialCount+1 {
		t.Errorf("expected %d checkers after register, got %d", initialCount+1, len(s.GetCheckers()))
	}
}

// ========== GenerateBaselineReport 测试 ==========

func TestGenerateBaselineReport(t *testing.T) {
	s := NewSecurityBaselineScanner()

	report := s.GenerateBaselineReport(context.Background(), BaselineCIS, nil)

	if report == nil {
		t.Fatal("report should not be nil")
	}
	if report.ID == "" {
		t.Error("report should have an ID")
	}
	if report.Standard != BaselineCIS {
		t.Errorf("expected standard CIS, got %s", report.Standard)
	}
	if report.Status != ScanStatusComplete {
		t.Errorf("expected status complete, got %s", report.Status)
	}
	if report.TotalChecks == 0 {
		t.Error("report should have checks")
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("score should be 0-100, got %d", report.Score)
	}
	if report.CompletedAt == nil {
		t.Error("report should have completion time")
	}
	if report.Summary == "" {
		t.Error("report should have a summary")
	}
}

func TestGenerateBaselineReportWithCategories(t *testing.T) {
	s := NewSecurityBaselineScanner()

	categories := []BaselineCategory{BaselinePasswordPolicy}
	report := s.GenerateBaselineReport(context.Background(), BaselineCIS, categories)

	if report == nil {
		t.Fatal("report should not be nil")
	}

	for _, r := range report.Results {
		if r.Category != BaselinePasswordPolicy {
			t.Errorf("expected password_policy category, got %s", r.Category)
		}
	}
}

func TestGenerateBaselineReportNIST(t *testing.T) {
	s := NewSecurityBaselineScanner()

	report := s.GenerateBaselineReport(context.Background(), BaselineNIST, nil)

	if report == nil {
		t.Fatal("report should not be nil")
	}
	if report.Standard != BaselineNIST {
		t.Errorf("expected standard NIST, got %s", report.Standard)
	}
	if report.TotalChecks == 0 {
		t.Error("NIST report should have checks")
	}
}

// ========== FormatBaselineCategoryName 测试 ==========

func TestFormatBaselineCategoryName(t *testing.T) {
	tests := []struct {
		cat      BaselineCategory
		expected string
	}{
		{BaselinePasswordPolicy, "密码策略"},
		{BaselineFilePermission, "文件权限"},
		{BaselineNetworkConfig, "网络配置"},
		{BaselineServiceSecurity, "服务安全"},
		{BaselineSSHConfig, "SSH 配置"},
		{BaselineAuditLogging, "审计日志"},
		{BaselineDiskEncryption, "磁盘加密"},
		{BaselineAccessControl, "访问控制"},
		{BaselineCategory("unknown"), "未知类别(unknown)"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cat), func(t *testing.T) {
			result := FormatBaselineCategoryName(tt.cat)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// ========== CIS 密码策略检查测试 ==========

func TestCISPasswordLengthChecker(t *testing.T) {
	checker := &cisPasswordLengthChecker{}

	if checker.Category() != BaselinePasswordPolicy {
		t.Errorf("expected password_policy, got %s", checker.Category())
	}
	if checker.Standard() != BaselineCIS {
		t.Errorf("expected CIS, got %s", checker.Standard())
	}
	if checker.Reference() != "CIS 5.3.1" {
		t.Errorf("expected CIS 5.3.1, got %s", checker.Reference())
	}

	result := checker.Check(context.Background())
	if result.CheckID != "cis_pw_length" {
		t.Errorf("expected check_id cis_pw_length, got %s", result.CheckID)
	}
	if result.Category != BaselinePasswordPolicy {
		t.Errorf("expected password_policy, got %s", result.Category)
	}
	if result.Status != CheckItemPass && result.Status != CheckItemFail {
		t.Errorf("unexpected status: %s", result.Status)
	}
}

func TestCISPasswordComplexityChecker(t *testing.T) {
	checker := &cisPasswordComplexityChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_pw_complexity" {
		t.Errorf("expected check_id cis_pw_complexity, got %s", result.CheckID)
	}
	if result.Status != CheckItemPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

func TestCISPasswordHistoryChecker(t *testing.T) {
	checker := &cisPasswordHistoryChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_pw_history" {
		t.Errorf("expected check_id cis_pw_history, got %s", result.CheckID)
	}
}

func TestCISAccountLockoutChecker(t *testing.T) {
	checker := &cisAccountLockoutChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_pw_lockout" {
		t.Errorf("expected check_id cis_pw_lockout, got %s", result.CheckID)
	}
	if result.Status != CheckItemPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

// ========== CIS 文件权限检查测试 ==========

func TestCISPasswdPermissionChecker(t *testing.T) {
	checker := &cisPasswdPermissionChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_file_passwd" {
		t.Errorf("expected check_id cis_file_passwd, got %s", result.CheckID)
	}
	if result.Category != BaselineFilePermission {
		t.Errorf("expected file_permission, got %s", result.Category)
	}
}

func TestCISshadowPermissionChecker(t *testing.T) {
	checker := &cisShadowPermissionChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_file_shadow" {
		t.Errorf("expected check_id cis_file_shadow, got %s", result.CheckID)
	}
	if result.Status != CheckItemPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

func TestCISshKeyPermissionChecker(t *testing.T) {
	checker := &cisSSHKeyPermissionChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_file_sshkey" {
		t.Errorf("expected check_id cis_file_sshkey, got %s", result.CheckID)
	}
}

func TestCISConfigFilePermissionChecker(t *testing.T) {
	checker := &cisConfigFilePermissionChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_file_config" {
		t.Errorf("expected check_id cis_file_config, got %s", result.CheckID)
	}
}

// ========== CIS 网络配置检查测试 ==========

func TestCISIPForwardingChecker(t *testing.T) {
	checker := &cisIPForwardingChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_net_ip_fwd" {
		t.Errorf("expected check_id cis_net_ip_fwd, got %s", result.CheckID)
	}
	if result.Category != BaselineNetworkConfig {
		t.Errorf("expected network_config, got %s", result.Category)
	}
}

func TestCISICMPRedirectChecker(t *testing.T) {
	checker := &cisICMPRedirectChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_net_icmp" {
		t.Errorf("expected check_id cis_net_icmp, got %s", result.CheckID)
	}
}

func TestCISFirewallStatusChecker(t *testing.T) {
	checker := &cisFirewallStatusChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_net_firewall" {
		t.Errorf("expected check_id cis_net_firewall, got %s", result.CheckID)
	}
	if result.Status != CheckItemPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

func TestCISNetworkBannerChecker(t *testing.T) {
	checker := &cisNetworkBannerChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_net_banner" {
		t.Errorf("expected check_id cis_net_banner, got %s", result.CheckID)
	}
}

// ========== CIS 服务安全检查测试 ==========

func TestCISSSHProtocolChecker(t *testing.T) {
	checker := &cisSSHProtocolChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_svc_ssh_proto" {
		t.Errorf("expected check_id cis_svc_ssh_proto, got %s", result.CheckID)
	}
	if result.Category != BaselineServiceSecurity {
		t.Errorf("expected service_security, got %s", result.Category)
	}
}

func TestCISSSHRootLoginChecker(t *testing.T) {
	checker := &cisSSHRootLoginChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_svc_ssh_root" {
		t.Errorf("expected check_id cis_svc_ssh_root, got %s", result.CheckID)
	}
}

func TestCISSSHMaxAuthTriesChecker(t *testing.T) {
	checker := &cisSSHMaxAuthTriesChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_svc_ssh_maxauth" {
		t.Errorf("expected check_id cis_svc_ssh_maxauth, got %s", result.CheckID)
	}
}

func TestCISUnnecessaryServicesChecker(t *testing.T) {
	checker := &cisUnnecessaryServicesChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "cis_svc_unnecessary" {
		t.Errorf("expected check_id cis_svc_unnecessary, got %s", result.CheckID)
	}
}

// ========== NIST 审计日志检查测试 ==========

func TestNISTAuditLogChecker(t *testing.T) {
	checker := &nistAuditLogChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_audit_enable" {
		t.Errorf("expected check_id nist_audit_enable, got %s", result.CheckID)
	}
	if result.Standard != BaselineNIST {
		t.Errorf("expected NIST, got %s", result.Standard)
	}
	if result.Category != BaselineAuditLogging {
		t.Errorf("expected audit_logging, got %s", result.Category)
	}
}

func TestNISTLogRetentionChecker(t *testing.T) {
	checker := &nistLogRetentionChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_audit_retention" {
		t.Errorf("expected check_id nist_audit_retention, got %s", result.CheckID)
	}
}

func TestNISTLogIntegrityChecker(t *testing.T) {
	checker := &nistLogIntegrityChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_audit_integrity" {
		t.Errorf("expected check_id nist_audit_integrity, got %s", result.CheckID)
	}
}

// ========== NIST 磁盘加密检查测试 ==========

func TestNISTDiskEncryptionChecker(t *testing.T) {
	checker := &nistDiskEncryptionChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_disk_encrypt" {
		t.Errorf("expected check_id nist_disk_encrypt, got %s", result.CheckID)
	}
	if result.Category != BaselineDiskEncryption {
		t.Errorf("expected disk_encryption, got %s", result.Category)
	}
}

func TestNISTEncryptionKeyManagementChecker(t *testing.T) {
	checker := &nistEncryptionKeyManagementChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_key_mgmt" {
		t.Errorf("expected check_id nist_key_mgmt, got %s", result.CheckID)
	}
}

// ========== NIST 访问控制检查测试 ==========

func TestNISTAccessControlChecker(t *testing.T) {
	checker := &nistAccessControlChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_ac_policy" {
		t.Errorf("expected check_id nist_ac_policy, got %s", result.CheckID)
	}
	if result.Category != BaselineAccessControl {
		t.Errorf("expected access_control, got %s", result.Category)
	}
}

func TestNISTPrivilegeManagementChecker(t *testing.T) {
	checker := &nistPrivilegeManagementChecker{}
	result := checker.Check(context.Background())

	if result.CheckID != "nist_ac_privilege" {
		t.Errorf("expected check_id nist_ac_privilege, got %s", result.CheckID)
	}
}

// ========== 基线标准常量测试 ==========

func TestBaselineStandardConstants(t *testing.T) {
	standards := []SecurityBaselineStandard{
		BaselineCIS,
		BaselineNIST,
		BaselineSTIG,
	}

	seen := make(map[SecurityBaselineStandard]bool)
	for _, s := range standards {
		if s == "" {
			t.Error("standard constant should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate standard: %s", s)
		}
		seen[s] = true
	}
}

func TestBaselineCategoryConstants(t *testing.T) {
	cats := []BaselineCategory{
		BaselinePasswordPolicy,
		BaselineFilePermission,
		BaselineNetworkConfig,
		BaselineServiceSecurity,
		BaselineSSHConfig,
		BaselineAuditLogging,
		BaselineDiskEncryption,
		BaselineAccessControl,
	}

	if len(cats) != 8 {
		t.Errorf("expected 8 baseline categories, got %d", len(cats))
	}
}

// ========== mockBaselineChecker ==========

type mockBaselineChecker struct {
	cat      BaselineCategory
	name     string
	standard SecurityBaselineStandard
	ref      string
	status   CheckItemStatus
}

func (m *mockBaselineChecker) Category() BaselineCategory         { return m.cat }
func (m *mockBaselineChecker) Name() string                       { return m.name }
func (m *mockBaselineChecker) Standard() SecurityBaselineStandard { return m.standard }
func (m *mockBaselineChecker) Reference() string                  { return m.ref }
func (m *mockBaselineChecker) Check(ctx context.Context) BaselineCheckResult {
	return BaselineCheckResult{
		CheckID:   "mock_" + m.name,
		Standard:  m.standard,
		Category:  m.cat,
		Name:      m.name,
		Status:    m.status,
		Severity:  SeverityMedium,
		Message:   "mock check result",
		Reference: m.ref,
	}
}

// ========== 集成测试 ==========

func TestBaselineFullWorkflow(t *testing.T) {
	s := NewSecurityBaselineScanner()

	// 1. 扫描所有基线
	allResults := s.Scan(context.Background(), nil, "")
	if len(allResults) == 0 {
		t.Fatal("should have results")
	}

	// 2. 生成 CIS 报告
	cisReport := s.GenerateBaselineReport(context.Background(), BaselineCIS, nil)
	if cisReport == nil {
		t.Fatal("CIS report should not be nil")
	}
	if cisReport.Standard != BaselineCIS {
		t.Errorf("expected CIS, got %s", cisReport.Standard)
	}
	if cisReport.TotalChecks == 0 {
		t.Error("CIS report should have checks")
	}

	// 3. 生成 NIST 报告
	nistReport := s.GenerateBaselineReport(context.Background(), BaselineNIST, nil)
	if nistReport == nil {
		t.Fatal("NIST report should not be nil")
	}
	if nistReport.Standard != BaselineNIST {
		t.Errorf("expected NIST, got %s", nistReport.Standard)
	}
	if nistReport.TotalChecks == 0 {
		t.Error("NIST report should have checks")
	}

	// 4. 验证分数合理
	if cisReport.Score < 0 || cisReport.Score > 100 {
		t.Errorf("CIS score should be 0-100, got %d", cisReport.Score)
	}
	if nistReport.Score < 0 || nistReport.Score > 100 {
		t.Errorf("NIST score should be 0-100, got %d", nistReport.Score)
	}

	// 5. 验证摘要非空
	if cisReport.Summary == "" {
		t.Error("CIS report should have summary")
	}
	if nistReport.Summary == "" {
		t.Error("NIST report should have summary")
	}
}

func TestBaselineScanWithAllCategories(t *testing.T) {
	s := NewSecurityBaselineScanner()

	allCategories := []BaselineCategory{
		BaselinePasswordPolicy,
		BaselineFilePermission,
		BaselineNetworkConfig,
		BaselineServiceSecurity,
		BaselineAuditLogging,
		BaselineDiskEncryption,
		BaselineAccessControl,
	}

	results := s.Scan(context.Background(), allCategories, BaselineCIS)

	if len(results) == 0 {
		t.Error("scan with all categories should return results")
	}

	// 验证每个结果的类别都在请求列表中
	for _, r := range results {
		found := false
		for _, cat := range allCategories {
			if r.Category == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("result category %s should be in requested categories", r.Category)
		}
	}
}

func TestBaselineReportScoreCalculation(t *testing.T) {
	s := NewSecurityBaselineScanner()

	// 多次执行扫描，验证分数计算
	for i := 0; i < 5; i++ {
		report := s.GenerateBaselineReport(context.Background(), BaselineCIS, nil)

		// 分数 = (通过数 * 100) / 总检查数
		expectedScore := 0
		if report.TotalChecks > 0 {
			expectedScore = (report.Passed * 100) / report.TotalChecks
		}

		if report.Score != expectedScore {
			t.Errorf("iteration %d: expected score %d, got %d", i, expectedScore, report.Score)
		}
	}
}
