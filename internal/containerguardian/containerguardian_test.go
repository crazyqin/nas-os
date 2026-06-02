package containerguardian

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cg := New()
	if cg == nil {
		t.Fatal("New() 返回 nil")
	}

	if cg.policies == nil {
		t.Error("policies map 未初始化")
	}
	if cg.networkPolicies == nil {
		t.Error("networkPolicies map 未初始化")
	}
	if cg.resourceLimits == nil {
		t.Error("resourceLimits map 未初始化")
	}
	if cg.auditLog == nil {
		t.Error("auditLog slice 未初始化")
	}
	if cg.scanResults == nil {
		t.Error("scanResults map 未初始化")
	}
	if cg.containers == nil {
		t.Error("containers map 未初始化")
	}
	if cg.vulnDB == nil {
		t.Error("vulnDB map 未初始化")
	}
}

func TestScanImage_Success(t *testing.T) {
	cg := New()

	// 扫描已知有漏洞的镜像
	result, err := cg.ScanImage("nginx:1.20")
	if err != nil {
		t.Fatalf("ScanImage 失败: %v", err)
	}

	if result == nil {
		t.Fatal("ScanResult 为 nil")
	}

	if result.ImageName != "nginx:1.20" {
		t.Errorf("镜像名称不匹配: got %s, want nginx:1.20", result.ImageName)
	}

	if result.IsClean {
		t.Error("预期有漏洞，但 IsClean=true")
	}

	if len(result.Vulnerabilities) == 0 {
		t.Error("预期有漏洞，但漏洞列表为空")
	}

	if result.Score >= 100 {
		t.Errorf("评分应小于100: got %.2f", result.Score)
	}
}

func TestScanImage_CleanImage(t *testing.T) {
	cg := New()

	// 扫描没有漏洞的镜像
	result, err := cg.ScanImage("alpine:3.14")
	if err != nil {
		t.Fatalf("ScanImage 失败: %v", err)
	}

	if !result.IsClean {
		t.Error("预期无漏洞，但 IsClean=false")
	}

	if len(result.Vulnerabilities) != 0 {
		t.Errorf("预期无漏洞，但发现 %d 个", len(result.Vulnerabilities))
	}

	if result.Score != 100.0 {
		t.Errorf("评分应为100: got %.2f", result.Score)
	}
}

func TestScanImage_EmptyName(t *testing.T) {
	cg := New()

	_, err := cg.ScanImage("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestScanImage_GetScanResult(t *testing.T) {
	cg := New()

	// 先扫描
	_, err := cg.ScanImage("redis:6.0")
	if err != nil {
		t.Fatalf("ScanImage 失败: %v", err)
	}

	// 获取扫描结果
	result, err := cg.GetScanResult("redis:6.0")
	if err != nil {
		t.Fatalf("GetScanResult 失败: %v", err)
	}

	if result.ImageName != "redis:6.0" {
		t.Errorf("镜像名称不匹配: got %s", result.ImageName)
	}
}

func TestGetScanResult_NotFound(t *testing.T) {
	cg := New()

	_, err := cg.GetScanResult("nonexistent:latest")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetScanResult_EmptyName(t *testing.T) {
	cg := New()

	_, err := cg.GetScanResult("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestMonitorContainer_Success(t *testing.T) {
	cg := New()

	status, err := cg.MonitorContainer("container-001")
	if err != nil {
		t.Fatalf("MonitorContainer 失败: %v", err)
	}

	if status == nil {
		t.Fatal("ContainerStatus 为 nil")
	}

	if status.ContainerID != "container-001" {
		t.Errorf("容器ID不匹配: got %s", status.ContainerID)
	}

	if !status.Running {
		t.Error("预期容器运行中")
	}
}

func TestMonitorContainer_EmptyID(t *testing.T) {
	cg := New()

	_, err := cg.MonitorContainer("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestSetResourceLimits_Success(t *testing.T) {
	cg := New()

	limits := &ResourceLimits{
		CPUQuota:    100000,
		MemoryLimit: 512 * 1024 * 1024,
		PidsLimit:   100,
		IOReadBPS:   10 * 1024 * 1024,
		IOWriteBPS:  5 * 1024 * 1024,
	}

	err := cg.SetResourceLimits("container-001", limits)
	if err != nil {
		t.Fatalf("SetResourceLimits 失败: %v", err)
	}

	// 验证可以获取
	result, err := cg.GetResourceLimits("container-001")
	if err != nil {
		t.Fatalf("GetResourceLimits 失败: %v", err)
	}

	if result.CPUQuota != 100000 {
		t.Errorf("CPUQuota 不匹配: got %d, want 100000", result.CPUQuota)
	}

	if result.MemoryLimit != 512*1024*1024 {
		t.Errorf("MemoryLimit 不匹配: got %d", result.MemoryLimit)
	}
}

func TestSetResourceLimits_EmptyContainerID(t *testing.T) {
	cg := New()

	limits := &ResourceLimits{CPUQuota: 100000}
	err := cg.SetResourceLimits("", limits)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestSetResourceLimits_NilLimits(t *testing.T) {
	cg := New()

	err := cg.SetResourceLimits("container-001", nil)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestSetResourceLimits_NegativeValues(t *testing.T) {
	cg := New()

	limits := &ResourceLimits{CPUQuota: -100}
	err := cg.SetResourceLimits("container-001", limits)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetResourceLimits_NotFound(t *testing.T) {
	cg := New()

	_, err := cg.GetResourceLimits("nonexistent")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetResourceLimits_EmptyID(t *testing.T) {
	cg := New()

	_, err := cg.GetResourceLimits("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestAddNetworkPolicy_Success(t *testing.T) {
	cg := New()

	policy := &NetworkPolicy{
		Name:         "test-policy",
		AllowIngress: false,
		AllowEgress:  true,
		AllowedPorts: []int{80, 443},
		BlockedPorts: []int{22, 3306},
	}

	err := cg.AddNetworkPolicy("container-001", policy)
	if err != nil {
		t.Fatalf("AddNetworkPolicy 失败: %v", err)
	}

	if !policy.IsActive {
		t.Error("预期策略被激活")
	}
}

func TestAddNetworkPolicy_EmptyContainerID(t *testing.T) {
	cg := New()

	policy := &NetworkPolicy{Name: "test"}
	err := cg.AddNetworkPolicy("", policy)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestAddNetworkPolicy_NilPolicy(t *testing.T) {
	cg := New()

	err := cg.AddNetworkPolicy("container-001", nil)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestAddNetworkPolicy_EmptyName(t *testing.T) {
	cg := New()

	policy := &NetworkPolicy{}
	err := cg.AddNetworkPolicy("container-001", policy)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestRemoveNetworkPolicy_Success(t *testing.T) {
	cg := New()

	// 先添加
	policy := &NetworkPolicy{Name: "test-policy"}
	cg.AddNetworkPolicy("container-001", policy)

	// 再移除
	err := cg.RemoveNetworkPolicy("container-001")
	if err != nil {
		t.Fatalf("RemoveNetworkPolicy 失败: %v", err)
	}
}

func TestRemoveNetworkPolicy_NotFound(t *testing.T) {
	cg := New()

	err := cg.RemoveNetworkPolicy("nonexistent")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestRemoveNetworkPolicy_EmptyID(t *testing.T) {
	cg := New()

	err := cg.RemoveNetworkPolicy("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestCreatePolicy_Success(t *testing.T) {
	cg := New()

	policy := &SecurityPolicy{
		Name:        "high-security",
		Description: "高安全等级策略",
		MaxSeverity: SeverityMedium,
	}

	err := cg.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("CreatePolicy 失败: %v", err)
	}

	if !policy.IsActive {
		t.Error("预期策略被激活")
	}
}

func TestCreatePolicy_DuplicateName(t *testing.T) {
	cg := New()

	policy1 := &SecurityPolicy{Name: "test-policy"}
	policy2 := &SecurityPolicy{Name: "test-policy"}

	cg.CreatePolicy(policy1)
	err := cg.CreatePolicy(policy2)
	if err == nil {
		t.Error("预期返回重复名称错误，但为 nil")
	}
}

func TestCreatePolicy_NilPolicy(t *testing.T) {
	cg := New()

	err := cg.CreatePolicy(nil)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestCreatePolicy_EmptyName(t *testing.T) {
	cg := New()

	policy := &SecurityPolicy{}
	err := cg.CreatePolicy(policy)
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestApplyPolicy_Success(t *testing.T) {
	cg := New()

	// 创建策略
	policy := &SecurityPolicy{
		Name:          "test-policy",
		EnforceLimits: true,
		ResourceLimits: &ResourceLimits{
			CPUQuota:    50000,
			MemoryLimit: 256 * 1024 * 1024,
		},
	}
	cg.CreatePolicy(policy)

	// 应用策略
	err := cg.ApplyPolicy("container-001", "test-policy")
	if err != nil {
		t.Fatalf("ApplyPolicy 失败: %v", err)
	}

	// 验证资源限制被应用
	limits, err := cg.GetResourceLimits("container-001")
	if err != nil {
		t.Fatalf("GetResourceLimits 失败: %v", err)
	}

	if limits.CPUQuota != 50000 {
		t.Errorf("CPUQuota 不匹配: got %d, want 50000", limits.CPUQuota)
	}
}

func TestApplyPolicy_PolicyNotFound(t *testing.T) {
	cg := New()

	err := cg.ApplyPolicy("container-001", "nonexistent")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestApplyPolicy_InactivePolicy(t *testing.T) {
	cg := New()

	policy := &SecurityPolicy{Name: "inactive-policy", IsActive: false}
	cg.policies["inactive-policy"] = policy

	err := cg.ApplyPolicy("container-001", "inactive-policy")
	if err == nil {
		t.Error("预期返回策略未激活错误，但为 nil")
	}
}

func TestApplyPolicy_EmptyContainerID(t *testing.T) {
	cg := New()

	err := cg.ApplyPolicy("", "test-policy")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestApplyPolicy_EmptyPolicyName(t *testing.T) {
	cg := New()

	err := cg.ApplyPolicy("container-001", "")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetAuditLog_AllLogs(t *testing.T) {
	cg := New()

	// 执行一些操作生成日志
	cg.ScanImage("nginx:1.20")
	cg.MonitorContainer("container-001")

	logs := cg.GetAuditLog("")
	if len(logs) < 2 {
		t.Errorf("预期至少2条日志，但得到 %d", len(logs))
	}
}

func TestGetAuditLog_FilterByContainer(t *testing.T) {
	cg := New()

	// 执行操作
	cg.MonitorContainer("container-001")
	cg.MonitorContainer("container-002")

	logs := cg.GetAuditLog("container-001")
	for _, entry := range logs {
		if entry.ContainerID != "container-001" {
			t.Errorf("日志包含非目标容器: %s", entry.ContainerID)
		}
	}
}

func TestGetSecurityScore_WithScan(t *testing.T) {
	cg := New()

	// 先扫描
	cg.ScanImage("container-001")

	score, err := cg.GetSecurityScore("container-001")
	if err != nil {
		t.Fatalf("GetSecurityScore 失败: %v", err)
	}

	if score < 0 || score > 100 {
		t.Errorf("评分超出范围: %.2f", score)
	}
}

func TestGetSecurityScore_WithNetworkPolicy(t *testing.T) {
	cg := New()

	// 添加网络策略（允许双向通信）
	policy := &NetworkPolicy{
		Name:         "test",
		AllowIngress: true,
		AllowEgress:  true,
	}
	cg.AddNetworkPolicy("container-001", policy)

	score, err := cg.GetSecurityScore("container-001")
	if err != nil {
		t.Fatalf("GetSecurityScore 失败: %v", err)
	}

	// 双向通信应扣分
	if score >= 100 {
		t.Error("预期评分低于100")
	}
}

func TestGetSecurityScore_EmptyID(t *testing.T) {
	cg := New()

	_, err := cg.GetSecurityScore("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetRemediation_WithVulnerabilities(t *testing.T) {
	cg := New()

	// 扫描有漏洞的镜像
	cg.ScanImage("nginx:1.20")

	suggestions, err := cg.GetRemediation("nginx:1.20")
	if err != nil {
		t.Fatalf("GetRemediation 失败: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("预期有修复建议")
	}
}

func TestGetRemediation_NoPolicies(t *testing.T) {
	cg := New()

	// 创建一个空的扫描结果
	cg.scanResults["container-001"] = &ScanResult{
		ImageName:       "container-001",
		Vulnerabilities: make([]Vulnerability, 0),
	}

	suggestions, err := cg.GetRemediation("container-001")
	if err != nil {
		t.Fatalf("GetRemediation 失败: %v", err)
	}

	// 应该建议添加网络策略和资源限制
	hasNetworkSuggestion := false
	hasResourceSuggestion := false
	for _, s := range suggestions {
		if contains(s, "网络隔离策略") {
			hasNetworkSuggestion = true
		}
		if contains(s, "资源限制") {
			hasResourceSuggestion = true
		}
	}

	if !hasNetworkSuggestion {
		t.Error("预期有网络策略建议")
	}
	if !hasResourceSuggestion {
		t.Error("预期有资源限制建议")
	}
}

func TestGetRemediation_EmptyID(t *testing.T) {
	cg := New()

	_, err := cg.GetRemediation("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestListPolicies(t *testing.T) {
	cg := New()

	cg.CreatePolicy(&SecurityPolicy{Name: "policy-1"})
	cg.CreatePolicy(&SecurityPolicy{Name: "policy-2"})

	policies := cg.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("预期2个策略，但得到 %d", len(policies))
	}
}

func TestDeletePolicy_Success(t *testing.T) {
	cg := New()

	cg.CreatePolicy(&SecurityPolicy{Name: "test-policy"})

	err := cg.DeletePolicy("test-policy")
	if err != nil {
		t.Fatalf("DeletePolicy 失败: %v", err)
	}

	// 验证已删除
	policies := cg.ListPolicies()
	if len(policies) != 0 {
		t.Errorf("预期0个策略，但得到 %d", len(policies))
	}
}

func TestDeletePolicy_NotFound(t *testing.T) {
	cg := New()

	err := cg.DeletePolicy("nonexistent")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestDeletePolicy_EmptyName(t *testing.T) {
	cg := New()

	err := cg.DeletePolicy("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetVulnerabilityStats(t *testing.T) {
	cg := New()

	cg.ScanImage("nginx:1.20")

	stats, err := cg.GetVulnerabilityStats("nginx:1.20")
	if err != nil {
		t.Fatalf("GetVulnerabilityStats 失败: %v", err)
	}

	// nginx:1.20 应该有 High 和 Medium 漏洞
	if stats[SeverityHigh] == 0 && stats[SeverityMedium] == 0 {
		t.Error("预期有 High 或 Medium 漏洞")
	}
}

func TestGetVulnerabilityStats_NotFound(t *testing.T) {
	cg := New()

	_, err := cg.GetVulnerabilityStats("nonexistent")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestGetVulnerabilityStats_EmptyName(t *testing.T) {
	cg := New()

	_, err := cg.GetVulnerabilityStats("")
	if err == nil {
		t.Error("预期返回错误，但为 nil")
	}
}

func TestAddVulnerability(t *testing.T) {
	cg := New()

	vuln := Vulnerability{
		ID:       "VULN-CUSTOM",
		CVE:      "CVE-2023-9999",
		Severity: SeverityCritical,
		Package:  "custom-pkg",
		Version:  "1.0.0",
		FixedIn:  "1.0.1",
	}

	cg.AddVulnerability("custom-image:1.0", vuln)

	// 扫描该镜像验证漏洞被添加
	result, err := cg.ScanImage("custom-image:1.0")
	if err != nil {
		t.Fatalf("ScanImage 失败: %v", err)
	}

	if len(result.Vulnerabilities) != 1 {
		t.Errorf("预期1个漏洞，但得到 %d", len(result.Vulnerabilities))
	}
}

func TestFormatReport(t *testing.T) {
	cg := New()

	// 设置一些数据
	cg.ScanImage("container-001")
	cg.SetResourceLimits("container-001", &ResourceLimits{
		CPUQuota:    100000,
		MemoryLimit: 512 * 1024 * 1024,
	})
	cg.AddNetworkPolicy("container-001", &NetworkPolicy{
		Name:         "test-policy",
		AllowIngress: false,
		AllowEgress:  true,
	})

	report := cg.FormatReport("container-001")
	if report == "" {
		t.Error("报告为空")
	}

	// 验证报告包含关键信息
	if !contains(report, "容器安全报告") {
		t.Error("报告缺少标题")
	}
	if !contains(report, "安全评分") {
		t.Error("报告缺少安全评分")
	}
}

func TestSeverityLevel_String(t *testing.T) {
	tests := []struct {
		level    SeverityLevel
		expected string
	}{
		{SeverityCritical, "Critical"},
		{SeverityHigh, "High"},
		{SeverityMedium, "Medium"},
		{SeverityLow, "Low"},
		{SeverityLevel(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("SeverityLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	cg := New()

	done := make(chan bool, 10)

	// 并发写入
	for i := 0; i < 5; i++ {
		go func(id int) {
			cg.SetResourceLimits("container-001", &ResourceLimits{
				CPUQuota: int64(id * 1000),
			})
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			cg.GetResourceLimits("container-001")
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAuditLogTimestamps(t *testing.T) {
	cg := New()

	start := time.Now()
	cg.ScanImage("test-image")
	cg.MonitorContainer("container-001")
	end := time.Now()

	logs := cg.GetAuditLog("")
	for _, entry := range logs {
		if entry.Timestamp.Before(start) || entry.Timestamp.After(end) {
			t.Errorf("审计日志时间戳超出范围: %v", entry.Timestamp)
		}
	}
}

// contains 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
