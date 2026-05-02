package licensescan

import (
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.scanner == nil {
		t.Fatal("Manager.scanner is nil")
	}
	if len(m.policies) == 0 {
		t.Fatal("Manager.policies is empty, expected default policy")
	}
	if _, ok := m.policies["default"]; !ok {
		t.Fatal("Default policy not found")
	}
}

func TestCreateAndListPolicy(t *testing.T) {
	m := NewManager()

	p := &Policy{
		ID:        "test-policy",
		Name:      "测试策略",
		Whitelist: []string{"MIT"},
		Blacklist: []string{"AGPL-3.0"},
	}

	if err := m.CreatePolicy(p); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	policies := m.ListPolicies()
	if len(policies) != 2 {
		t.Fatalf("ListPolicies returned %d policies, want 2", len(policies))
	}

	// 创建重复策略应失败
	if err := m.CreatePolicy(p); err == nil {
		t.Error("Expected error for duplicate policy, got nil")
	}
}

func TestCreatePolicyValidation(t *testing.T) {
	m := NewManager()

	// 空名称应失败
	if err := m.CreatePolicy(&Policy{ID: "test"}); err == nil {
		t.Error("Expected error for empty name, got nil")
	}

	// 自动生成ID
	p := &Policy{Name: "自动ID策略"}
	if err := m.CreatePolicy(p); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if p.ID == "" {
		t.Error("Expected auto-generated ID, got empty")
	}
}

func TestGetPolicy(t *testing.T) {
	m := NewManager()

	// 获取默认策略
	p, err := m.GetPolicy("default")
	if err != nil {
		t.Fatalf("GetPolicy(default) failed: %v", err)
	}
	if p.Name != "默认策略" {
		t.Errorf("Default policy name = %q, want %q", p.Name, "默认策略")
	}

	// 获取不存在的策略
	_, err = m.GetPolicy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent policy, got nil")
	}
}

func TestUpdatePolicy(t *testing.T) {
	m := NewManager()

	p, _ := m.GetPolicy("default")
	p.Name = "更新后的策略"
	if err := m.UpdatePolicy(p); err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	updated, _ := m.GetPolicy("default")
	if updated.Name != "更新后的策略" {
		t.Errorf("Updated policy name = %q, want %q", updated.Name, "更新后的策略")
	}

	// 更新不存在的策略
	if err := m.UpdatePolicy(&Policy{ID: "nonexistent"}); err == nil {
		t.Error("Expected error for updating nonexistent policy, got nil")
	}
}

func TestDeletePolicy(t *testing.T) {
	m := NewManager()

	// 创建并删除
	m.CreatePolicy(&Policy{ID: "deleteme", Name: "要删除"})
	if err := m.DeletePolicy("deleteme"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	// 删除默认策略应失败
	if err := m.DeletePolicy("default"); err == nil {
		t.Error("Expected error for deleting default policy, got nil")
	}

	// 删除不存在的策略
	if err := m.DeletePolicy("nonexistent"); err == nil {
		t.Error("Expected error for deleting nonexistent policy, got nil")
	}
}

func TestGoModScan(t *testing.T) {
	m := NewManager()

	// 创建临时go.mod文件
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	writeTestFile(t, goModPath, `module test-project

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/gorilla/mux v1.8.0
	github.com/google/uuid v1.3.0
	golang.org/x/text v0.14.0
)
`)

	result, err := m.RunGoModScan(goModPath, "")
	if err != nil {
		t.Fatalf("RunGoModScan failed: %v", err)
	}

	if result.Status != StatusComplete {
		t.Errorf("Status = %q, want %q", result.Status, StatusComplete)
	}
	if result.ScanType != ScanTypeGoMod {
		t.Errorf("ScanType = %q, want %q", result.ScanType, ScanTypeGoMod)
	}
	if len(result.Licenses) != 4 {
		t.Fatalf("Got %d licenses, want 4", len(result.Licenses))
	}

	// 验证许可证分类
	for _, lic := range result.Licenses {
		switch lic.Source {
		case "github.com/gin-gonic/gin":
			if lic.Category != CategoryPermissive {
				t.Errorf("gin license category = %q, want %q", lic.Category, CategoryPermissive)
			}
		case "github.com/gorilla/mux":
			if lic.Category != CategoryPermissive {
				t.Errorf("gorilla/mux license category = %q, want %q", lic.Category, CategoryPermissive)
			}
		case "golang.org/x/text":
			if lic.Category != CategoryPermissive {
				t.Errorf("golang.org/x/text license category = %q, want %q", lic.Category, CategoryPermissive)
			}
		}
	}

	// 扫描结果应被保存
	savedResult, err := m.GetScanResult(result.ID)
	if err != nil {
		t.Fatalf("GetScanResult failed: %v", err)
	}
	if savedResult.ID != result.ID {
		t.Errorf("Saved result ID mismatch")
	}

	// ListScans应返回该结果
	scans := m.ListScans()
	if len(scans) == 0 {
		t.Error("ListScans returned empty after scan")
	}
}

func TestGoModScanWithViolations(t *testing.T) {
	m := NewManager()

	// 创建包含GPL依赖的go.mod
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	writeTestFile(t, goModPath, `module test-project

go 1.21

require (
	github.com/some/project v1.0.0
)
`)

	// 自定义策略，把某个模块标记为黑名单
	p := &Policy{
		ID:        "strict",
		Name:      "严格策略",
		Blacklist: []string{"unknown"},
		DefaultList: ListBlacklist,
	}
	m.CreatePolicy(p)

	result, err := m.RunGoModScan(goModPath, "strict")
	if err != nil {
		t.Fatalf("RunGoModScan failed: %v", err)
	}

	// 应该有违规（unknown在黑名单策略下会被标记为denied）
	if len(result.Violations) == 0 {
		t.Error("Expected violations with strict policy")
	}
}

func TestDashboardData(t *testing.T) {
	m := NewManager()

	// 空状态仪表盘
	data := m.GetDashboardData()
	if data.ComplianceRate != 0 {
		t.Errorf("Empty compliance rate = %f, want 0", data.ComplianceRate)
	}

	// 执行一个扫描
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	writeTestFile(t, goModPath, `module test
go 1.21
require github.com/gin-gonic/gin v1.9.1
`)
	m.RunGoModScan(goModPath, "")

	data = m.GetDashboardData()
	if data.TotalScans != 1 {
		t.Errorf("TotalScans = %d, want 1", data.TotalScans)
	}
	if data.ComplianceRate != 100 {
		t.Errorf("ComplianceRate = %f, want 100", data.ComplianceRate)
	}
	if data.LicenseBreakdown[CategoryPermissive] == 0 {
		t.Error("Expected permissive licenses in breakdown")
	}
}

func TestAlerts(t *testing.T) {
	m := NewManager()

	alertReceived := false
	m.SetAlertFunc(func(alert Alert) {
		alertReceived = true
	})

	// 执行一个会产生违规的扫描
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	writeTestFile(t, goModPath, `module test
go 1.21
require github.com/some/unknown v1.0.0
`)

	p := &Policy{
		ID:          "alert-test",
		Blacklist:   []string{"unknown"},
		DefaultList: ListBlacklist,
	}
	m.CreatePolicy(p)
	m.RunGoModScan(goModPath, "alert-test")

	if !alertReceived {
		t.Error("Expected alert callback to be called")
	}

	alerts := m.GetAlerts()
	if len(alerts) == 0 {
		t.Error("Expected alerts to be stored")
	}
}

func TestReportGeneration(t *testing.T) {
	m := NewManager()

	// 执行扫描
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	writeTestFile(t, goModPath, `module test
go 1.21
require github.com/gin-gonic/gin v1.9.1
`)
	m.RunGoModScan(goModPath, "")

	// 生成JSON报告
	report, err := m.GenerateReport("测试报告", FormatJSON, nil)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if report.Title != "测试报告" {
		t.Errorf("Report title = %q, want %q", report.Title, "测试报告")
	}
	if report.Summary.TotalScans != 1 {
		t.Errorf("TotalScans = %d, want 1", report.Summary.TotalScans)
	}

	// 生成HTML报告
	reportHTML, err := m.GenerateReport("HTML报告", FormatHTML, nil)
	if err != nil {
		t.Fatalf("GenerateReport HTML failed: %v", err)
	}

	rg := NewReportGenerator()
	htmlBytes, err := rg.GenerateHTML(reportHTML)
	if err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}
	if len(htmlBytes) == 0 {
		t.Error("Generated HTML is empty")
	}

	// 生成JSON
	jsonBytes, err := rg.GenerateJSON(report)
	if err != nil {
		t.Fatalf("GenerateJSON failed: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("Generated JSON is empty")
	}

	// 获取报告
	saved, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if saved.ID != report.ID {
		t.Error("Saved report ID mismatch")
	}

	// 列出报告
	reports := m.ListReports()
	if len(reports) < 2 {
		t.Errorf("ListReports returned %d, want >= 2", len(reports))
	}

	// 不存在的报告
	_, err = m.GetReport("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent report")
	}

	// 没有扫描结果时生成报告
	m2 := NewManager()
	_, err = m2.GenerateReport("空报告", FormatJSON, []string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for report with no scan results")
	}
}

func TestScannerGoMod(t *testing.T) {
	policy := &Policy{
		Whitelist: []string{"MIT", "Apache-2.0", "BSD-3-Clause"},
		Blacklist: []string{"AGPL-3.0"},
	}
	scanner := NewScanner(policy)

	// 创建临时go.mod
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	writeTestFile(t, goModPath, `module test
go 1.21
require (
	github.com/gin-gonic/gin v1.9.1
	golang.org/x/text v0.14.0
)
`)

	result, err := scanner.ScanGoMod(goModPath)
	if err != nil {
		t.Fatalf("ScanGoMod failed: %v", err)
	}
	if result.Status != StatusComplete {
		t.Errorf("Status = %q, want %q", result.Status, StatusComplete)
	}
	if result.Summary.TotalPackages != 2 {
		t.Errorf("TotalPackages = %d, want 2", result.Summary.TotalPackages)
	}
}

func TestScannerInvalidPath(t *testing.T) {
	scanner := NewScanner(nil)
	_, err := scanner.ScanGoMod("/nonexistent/path/go.mod")
	if err == nil {
		t.Error("Expected error for nonexistent go.mod path")
	}
}

func TestScheduler(t *testing.T) {
	m := NewManager()
	s := NewScheduler(m)

	// 添加任务
	task := ScheduledTask{
		Name:     "测试任务",
		ScanType: ScanTypeGoMod,
		Targets:  []string{"/tmp/go.mod"},
		Interval: 1 * time.Hour,
		Enabled:  true,
	}
	s.AddTask(task)

	tasks := s.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("ListTasks returned %d, want 1", len(tasks))
	}
	if tasks[0].Name != "测试任务" {
		t.Errorf("Task name = %q, want %q", tasks[0].Name, "测试任务")
	}

	// 移除任务
	if !s.RemoveTask(tasks[0].ID) {
		t.Error("RemoveTask returned false")
	}

	if len(s.ListTasks()) != 0 {
		t.Error("Expected empty task list after removal")
	}

	// 移除不存在的任务
	if s.RemoveTask("nonexistent") {
		t.Error("RemoveTask should return false for nonexistent task")
	}

	// 启动/停止
	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	// 重复启动/停止不应panic
	s.Start()
	s.Stop()
	s.Stop()
}

func TestGetPolicyOrDefault(t *testing.T) {
	m := NewManager()

	// 空policyID应返回默认
	p := m.getPolicyOrDefault("")
	if p.ID != "default" {
		t.Errorf("getPolicyOrDefault('') returned ID %q, want 'default'", p.ID)
	}

	// 存在的策略
	m.CreatePolicy(&Policy{ID: "custom", Name: "自定义"})
	p = m.getPolicyOrDefault("custom")
	if p.ID != "custom" {
		t.Errorf("getPolicyOrDefault('custom') returned ID %q, want 'custom'", p.ID)
	}

	// 不存在的策略应返回默认
	p = m.getPolicyOrDefault("nonexistent")
	if p.ID != "default" {
		t.Errorf("getPolicyOrDefault('nonexistent') returned ID %q, want 'default'", p.ID)
	}
}

func TestBuildReportSummary(t *testing.T) {
	results := []ScanResult{
		{
			Status:    StatusComplete,
			Licenses:  []License{{Name: "MIT", Compliance: ComplianceAllowed}},
			Summary:   ScanSummary{TotalLicenses: 1},
			Violations: nil,
		},
		{
			Status:    StatusComplete,
			Licenses:  []License{{Name: "AGPL", Compliance: ComplianceDenied}},
			Summary:   ScanSummary{TotalLicenses: 1},
			Violations: []Violation{{LicenseName: "AGPL", ListType: ListBlacklist, Severity: SeverityHigh}},
		},
		{
			Status:    StatusComplete,
			Licenses:  []License{{Name: "LGPL", Compliance: ComplianceReview}},
			Summary:   ScanSummary{TotalLicenses: 1},
			Violations: []Violation{{LicenseName: "LGPL", ListType: ListGraylist, Severity: SeverityMedium}},
		},
	}

	summary := buildReportSummary(results)

	if summary.TotalScans != 3 {
		t.Errorf("TotalScans = %d, want 3", summary.TotalScans)
	}
	if summary.TotalLicenses != 3 {
		t.Errorf("TotalLicenses = %d, want 3", summary.TotalLicenses)
	}
	if summary.Compliant != 1 {
		t.Errorf("Compliant = %d, want 1", summary.Compliant)
	}
	if summary.NonCompliant != 2 {
		t.Errorf("NonCompliant = %d, want 2", summary.NonCompliant)
	}
	if summary.NeedsReview != 1 {
		t.Errorf("NeedsReview = %d, want 1", summary.NeedsReview)
	}
}

// writeTestFile 辅助函数：写入测试文件.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTestFile failed: %v", err)
	}
}
