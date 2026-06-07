package auditreport

import (
	"testing"
)

func TestReportEngineComplianceReport(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	// 测试 GDPR 合规报告
	report, err := engine.GenerateComplianceReport(StandardGDPR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ID == "" {
		t.Error("expected report ID")
	}
	if report.Standard != StandardGDPR {
		t.Errorf("expected standard GDPR, got %q", report.Standard)
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("expected score between 0-100, got %f", report.Score)
	}
	if report.Passed+report.Failed != report.Total {
		t.Error("passed + failed should equal total")
	}
	if len(report.Sections) == 0 {
		t.Error("expected sections")
	}

	// 检查章节
	for _, section := range report.Sections {
		if section.Title == "" {
			t.Error("expected section title")
		}
		if len(section.Items) == 0 {
			t.Errorf("expected items in section %q", section.Title)
		}
	}
}

func TestReportEngineFIPS140Report(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	report, err := engine.GenerateComplianceReport(StandardFIPS140)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Standard != StandardFIPS140 {
		t.Errorf("expected standard FIPS140, got %q", report.Standard)
	}

	// FIPS 140 应该有密码模块、身份认证、完整性保护、物理安全章节
	expectedSections := 4
	if len(report.Sections) != expectedSections {
		t.Errorf("expected %d sections, got %d", expectedSections, len(report.Sections))
	}
}

func TestReportEngineDJB20Report(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	report, err := engine.GenerateComplianceReport(StandardDJB20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Standard != StandardDJB20 {
		t.Errorf("expected standard 等保2.0, got %q", report.Standard)
	}

	// 等保 2.0 应该有网络安全、主机安全、应用安全、数据安全、安全管理章节
	expectedSections := 5
	if len(report.Sections) != expectedSections {
		t.Errorf("expected %d sections, got %d", expectedSections, len(report.Sections))
	}
}

func TestReportEngineSOC2Report(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	report, err := engine.GenerateComplianceReport(StandardSOC2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Standard != StandardSOC2 {
		t.Errorf("expected standard SOC2, got %q", report.Standard)
	}
}

func TestReportEngineInvalidStandard(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	_, err := engine.GenerateComplianceReport("INVALID")
	if err == nil {
		t.Error("expected error for invalid standard")
	}
}

func TestReportEngineComprehensiveReport(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	// 添加一些测试事件
	analyzer := engine.GetAnalyzer()
	analyzer.AddEvent(&AuditEvent{
		ID:       "test-1",
		UserID:   "user1",
		Action:   "login",
		Resource: "/auth",
		IP:       "192.168.1.1",
		Result:   "success",
	})
	analyzer.AddEvent(&AuditEvent{
		ID:       "test-2",
		UserID:   "user1",
		Action:   "read",
		Resource: "/api/files",
		IP:       "192.168.1.1",
		Result:   "success",
	})

	report, err := engine.GenerateComprehensiveReport("综合安全报告", "2024-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ID == "" {
		t.Error("expected report ID")
	}
	if report.Title != "综合安全报告" {
		t.Errorf("expected title '综合安全报告', got %q", report.Title)
	}
	if report.BaseReport == nil {
		t.Error("expected base report")
	}
	if report.Summary == "" {
		t.Error("expected summary")
	}
}

func TestReportEngineExport(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	// 创建一个报告
	report := manager.GenerateReport(GenerateReportRequest{
		Title:  "测试报告",
		Period: "2024-01",
	})

	// 测试 JSON 导出
	jsonResult, err := engine.ExportReport(ExportRequest{
		ReportID: report.ID,
		Format:   FormatJSON,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jsonResult.Format != FormatJSON {
		t.Errorf("expected format JSON, got %q", jsonResult.Format)
	}
	if jsonResult.Content == "" {
		t.Error("expected content")
	}

	// 测试 HTML 导出
	htmlResult, err := engine.ExportReport(ExportRequest{
		ReportID: report.ID,
		Format:   FormatHTML,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if htmlResult.Format != FormatHTML {
		t.Errorf("expected format HTML, got %q", htmlResult.Format)
	}

	// 测试 PDF 导出
	pdfResult, err := engine.ExportReport(ExportRequest{
		ReportID: report.ID,
		Format:   FormatPDF,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pdfResult.Format != FormatPDF {
		t.Errorf("expected format PDF, got %q", pdfResult.Format)
	}

	// 测试 CSV 导出
	csvResult, err := engine.ExportReport(ExportRequest{
		ReportID: report.ID,
		Format:   FormatCSV,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if csvResult.Format != FormatCSV {
		t.Errorf("expected format CSV, got %q", csvResult.Format)
	}
}

func TestReportEngineExportInvalidFormat(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	report := manager.GenerateReport(GenerateReportRequest{
		Title:  "测试报告",
		Period: "2024-01",
	})

	_, err := engine.ExportReport(ExportRequest{
		ReportID: report.ID,
		Format:   "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestReportEngineExportComplianceReport(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	report, _ := engine.GenerateComplianceReport(StandardGDPR)

	// JSON 导出
	result, err := engine.ExportComplianceReport(report, FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Format != FormatJSON {
		t.Errorf("expected format JSON, got %q", result.Format)
	}
}

func TestReportEngineExportRiskReport(t *testing.T) {
	manager := NewManager()
	engine := NewReportEngine(manager)

	// 添加测试数据
	analyzer := engine.GetAnalyzer()
	analyzer.AddEvent(&AuditEvent{
		UserID:   "user1",
		Action:   "login",
		Resource: "/auth",
		Result:   "success",
	})

	// JSON 导出
	result, err := engine.ExportRiskReport(FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Format != FormatJSON {
		t.Errorf("expected format JSON, got %q", result.Format)
	}
}

func TestTemplateManagerGetTemplate(t *testing.T) {
	tm := NewTemplateManager()

	// 获取 GDPR 模板
	gdpr, err := tm.GetTemplate(StandardGDPR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gdpr.Standard != StandardGDPR {
		t.Errorf("expected GDPR, got %q", gdpr.Standard)
	}

	// 获取 FIPS140 模板
	fips, err := tm.GetTemplate(StandardFIPS140)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fips.Standard != StandardFIPS140 {
		t.Errorf("expected FIPS140, got %q", fips.Standard)
	}

	// 获取等保 2.0 模板
	djb, err := tm.GetTemplate(StandardDJB20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if djb.Standard != StandardDJB20 {
		t.Errorf("expected 等保2.0, got %q", djb.Standard)
	}

	// 获取不存在的模板
	_, err = tm.GetTemplate("INVALID")
	if err == nil {
		t.Error("expected error for invalid standard")
	}
}

func TestTemplateManagerListTemplates(t *testing.T) {
	tm := NewTemplateManager()
	templates := tm.ListTemplates()

	if len(templates) < 4 {
		t.Errorf("expected at least 4 templates, got %d", len(templates))
	}
}
