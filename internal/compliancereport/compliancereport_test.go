package compliancereport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== types.go 测试 ==========

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("test")
	id2 := GenerateID("test")

	if id1 == "" {
		t.Error("GenerateID should not return empty string")
	}
	if id1 == id2 {
		t.Error("GenerateID should return unique IDs")
	}
	if !strings.HasPrefix(id1, "test_") {
		t.Errorf("expected prefix 'test_', got '%s'", id1)
	}
}

func TestComplianceStandardConstants(t *testing.T) {
	standards := []ComplianceStandard{
		StandardGDPR, StandardSOC2, StandardDJBH, StandardISO27001, StandardHIPAA,
	}
	seen := make(map[ComplianceStandard]bool)
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

func TestCheckCategoryConstants(t *testing.T) {
	cats := []CheckCategory{
		CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity, CategoryBackup, CategoryNetwork,
	}
	if len(cats) != 5 {
		t.Errorf("expected 5 check categories, got %d", len(cats))
	}
}

func TestScanStatusConstants(t *testing.T) {
	if ScanStatusPending != "pending" {
		t.Error("ScanStatusPending should be 'pending'")
	}
	if ScanStatusRunning != "running" {
		t.Error("ScanStatusRunning should be 'running'")
	}
	if ScanStatusComplete != "complete" {
		t.Error("ScanStatusComplete should be 'complete'")
	}
	if ScanStatusFailed != "failed" {
		t.Error("ScanStatusFailed should be 'failed'")
	}
}

func TestComplianceStatusConstants(t *testing.T) {
	if StatusCompliant != "compliant" {
		t.Error("StatusCompliant should be 'compliant'")
	}
	if StatusNonCompliant != "non_compliant" {
		t.Error("StatusNonCompliant should be 'non_compliant'")
	}
	if StatusPendingReview != "pending_review" {
		t.Error("StatusPendingReview should be 'pending_review'")
	}
}

// ========== standards.go 测试 ==========

func TestNewStandardsManager(t *testing.T) {
	sm := NewStandardsManager()
	if sm == nil {
		t.Fatal("NewStandardsManager should not return nil")
	}
}

func TestListStandards(t *testing.T) {
	sm := NewStandardsManager()
	standards := sm.ListStandards()

	if len(standards) != 5 {
		t.Errorf("expected 5 standards, got %d", len(standards))
	}

	ids := make(map[ComplianceStandard]bool)
	for _, s := range standards {
		ids[s.ID] = true
		if s.Name == "" {
			t.Errorf("standard %s should have a name", s.ID)
		}
		if s.Description == "" {
			t.Errorf("standard %s should have a description", s.ID)
		}
		if len(s.Categories) == 0 {
			t.Errorf("standard %s should have categories", s.ID)
		}
	}

	expected := []ComplianceStandard{StandardGDPR, StandardSOC2, StandardDJBH, StandardISO27001, StandardHIPAA}
	for _, e := range expected {
		if !ids[e] {
			t.Errorf("missing standard: %s", e)
		}
	}
}

func TestGetStandard(t *testing.T) {
	sm := NewStandardsManager()

	// 存在的标准
	s, ok := sm.GetStandard(StandardGDPR)
	if !ok {
		t.Fatal("should find GDPR standard")
	}
	if s.ID != StandardGDPR {
		t.Errorf("expected %s, got %s", StandardGDPR, s.ID)
	}
	if s.Version != "2016/679" {
		t.Errorf("expected version '2016/679', got '%s'", s.Version)
	}

	// 不存在的标准
	_, ok = sm.GetStandard(ComplianceStandard("nonexistent"))
	if ok {
		t.Error("should not find nonexistent standard")
	}
}

func TestIsSupported(t *testing.T) {
	sm := NewStandardsManager()

	if !sm.IsSupported(StandardGDPR) {
		t.Error("GDPR should be supported")
	}
	if !sm.IsSupported(StandardSOC2) {
		t.Error("SOC2 should be supported")
	}
	if !sm.IsSupported(StandardDJBH) {
		t.Error("DJBH should be supported")
	}
	if sm.IsSupported(ComplianceStandard("nonexistent")) {
		t.Error("nonexistent should not be supported")
	}
}

func TestStandardCategories(t *testing.T) {
	sm := NewStandardsManager()

	gdpr, _ := sm.GetStandard(StandardGDPR)
	// GDPR 应包含访问控制、数据加密、日志完整性
	found := make(map[CheckCategory]bool)
	for _, c := range gdpr.Categories {
		found[c] = true
	}
	if !found[CategoryAccessControl] {
		t.Error("GDPR should include access control")
	}
	if !found[CategoryDataEncryption] {
		t.Error("GDPR should include data encryption")
	}
	if !found[CategoryLogIntegrity] {
		t.Error("GDPR should include log integrity")
	}

	soc2, _ := sm.GetStandard(StandardSOC2)
	if len(soc2.Categories) != 5 {
		t.Errorf("SOC2 should have 5 categories, got %d", len(soc2.Categories))
	}
}

// ========== scanner.go 测试 ==========

func TestNewScanner(t *testing.T) {
	s := NewScanner()
	if s == nil {
		t.Fatal("NewScanner should not return nil")
	}

	checkers := s.GetCheckers()
	if len(checkers) == 0 {
		t.Error("scanner should have default checkers registered")
	}
	if len(checkers) != 15 {
		t.Errorf("expected 15 default checkers, got %d", len(checkers))
	}
}

func TestScannerCategories(t *testing.T) {
	s := NewScanner()

	categoryCount := make(map[CheckCategory]int)
	for _, c := range s.GetCheckers() {
		categoryCount[c.Category()]++
	}

	// 每个类别至少有 1 个检查项
	expectedCats := []CheckCategory{
		CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity,
		CategoryBackup, CategoryNetwork,
	}
	for _, cat := range expectedCats {
		if categoryCount[cat] == 0 {
			t.Errorf("category %s should have at least 1 checker", cat)
		}
	}
}

func TestScannerScanAll(t *testing.T) {
	s := NewScanner()
	results := s.Scan(context.Background(), nil)

	if len(results) == 0 {
		t.Error("scan should return results")
	}

	for _, r := range results {
		if r.CheckID == "" {
			t.Error("result should have a check_id")
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
	}
}

func TestScannerScanByCategory(t *testing.T) {
	s := NewScanner()

	categories := []CheckCategory{CategoryAccessControl}
	results := s.Scan(context.Background(), categories)

	if len(results) != 3 {
		t.Errorf("expected 3 access control checkers, got %d", len(results))
	}

	for _, r := range results {
		if r.Category != CategoryAccessControl {
			t.Errorf("expected access_control category, got %s", r.Category)
		}
	}
}

func TestScannerScanMultipleCategories(t *testing.T) {
	s := NewScanner()

	categories := []CheckCategory{CategoryNetwork, CategoryBackup}
	results := s.Scan(context.Background(), categories)

	// Network: 3, Backup: 3
	if len(results) != 6 {
		t.Errorf("expected 6 results for network+backup, got %d", len(results))
	}
}

func TestRegisterChecker(t *testing.T) {
	s := NewScanner()
	initialCount := len(s.GetCheckers())

	s.RegisterChecker(&mockChecker{
		cat:    CategoryAccessControl,
		name:   "custom_check",
		status: CheckItemPass,
	})

	if len(s.GetCheckers()) != initialCount+1 {
		t.Errorf("expected %d checkers after register, got %d", initialCount+1, len(s.GetCheckers()))
	}
}

func TestFormatCategoryName(t *testing.T) {
	tests := []struct {
		cat      CheckCategory
		expected string
	}{
		{CategoryAccessControl, "访问控制审计"},
		{CategoryDataEncryption, "数据加密状态"},
		{CategoryLogIntegrity, "日志完整性"},
		{CategoryBackup, "备份合规性"},
		{CategoryNetwork, "网络安全配置"},
		{CheckCategory("unknown"), "未知类别(unknown)"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cat), func(t *testing.T) {
			result := FormatCategoryName(tt.cat)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// mockChecker 模拟检查器.
type mockChecker struct {
	cat    CheckCategory
	name   string
	status CheckItemStatus
}

func (m *mockChecker) Category() CheckCategory { return m.cat }
func (m *mockChecker) Name() string            { return m.name }
func (m *mockChecker) Check(ctx context.Context) ScanResult {
	return ScanResult{
		CheckID:  "mock_" + m.name,
		Category: m.cat,
		Name:     m.name,
		Status:   m.status,
		Severity: SeverityMedium,
		Message:  "mock check result",
	}
}

// ========== remediation.go 测试 ==========

func TestNewRemediationGenerator(t *testing.T) {
	g := NewRemediationGenerator()
	if g == nil {
		t.Fatal("NewRemediationGenerator should not return nil")
	}
}

func TestRemediationGenerate(t *testing.T) {
	g := NewRemediationGenerator()

	results := []ScanResult{
		{CheckID: "ac_mfa", Category: CategoryAccessControl, Name: "MFA", Status: CheckItemFail, Severity: SeverityCritical, Message: "MFA not enabled"},
		{CheckID: "de_key_mgmt", Category: CategoryDataEncryption, Name: "Key Mgmt", Status: CheckItemWarning, Severity: SeverityHigh, Message: "Key rotation overdue"},
		{CheckID: "ac_policy", Category: CategoryAccessControl, Name: "Policy", Status: CheckItemPass, Severity: SeverityHigh, Message: "OK"},
		{CheckID: "bk_policy", Category: CategoryBackup, Name: "Backup", Status: CheckItemFail, Severity: SeverityCritical, Message: "No backup"},
	}

	remediations := g.Generate(results)

	// 应该为 3 个失败/警告项生成建议（跳过 pass 的）
	if len(remediations) != 3 {
		t.Errorf("expected 3 remediations, got %d", len(remediations))
	}

	// 检查 MFA 建议
	var mfaRem *Remediation
	for i, r := range remediations {
		if r.CheckID == "ac_mfa" {
			mfaRem = &remediations[i]
			break
		}
	}
	if mfaRem == nil {
		t.Fatal("should have MFA remediation")
	}
	if mfaRem.Priority != SeverityCritical {
		t.Errorf("MFA remediation should be critical, got %s", mfaRem.Priority)
	}
	if len(mfaRem.Steps) == 0 {
		t.Error("MFA remediation should have steps")
	}
}

func TestRemediationGenerateAllPass(t *testing.T) {
	g := NewRemediationGenerator()

	results := []ScanResult{
		{CheckID: "test1", Status: CheckItemPass},
		{CheckID: "test2", Status: CheckItemPass},
	}

	remediations := g.Generate(results)
	if len(remediations) != 0 {
		t.Errorf("expected 0 remediations for all-pass, got %d", len(remediations))
	}
}

func TestRemediationGenericFallback(t *testing.T) {
	g := NewRemediationGenerator()

	results := []ScanResult{
		{CheckID: "unknown_check", Category: CategoryNetwork, Name: "Unknown", Status: CheckItemFail, Severity: SeverityMedium, Message: "something wrong"},
	}

	remediations := g.Generate(results)
	if len(remediations) != 1 {
		t.Fatalf("expected 1 remediation, got %d", len(remediations))
	}

	r := remediations[0]
	if r.Priority != SeverityMedium {
		t.Errorf("expected medium priority, got %s", r.Priority)
	}
	if len(r.Steps) == 0 {
		t.Error("generic remediation should have steps")
	}
}

func TestRemediationBackupRestoreTest(t *testing.T) {
	g := NewRemediationGenerator()

	results := []ScanResult{
		{CheckID: "bk_restore_test", Category: CategoryBackup, Name: "Restore Test", Status: CheckItemWarning, Severity: SeverityMedium, Message: "restore test overdue"},
	}

	remediations := g.Generate(results)
	if len(remediations) != 1 {
		t.Fatalf("expected 1 remediation, got %d", len(remediations))
	}

	if remediations[0].Title != "执行备份恢复测试" {
		t.Errorf("unexpected title: %s", remediations[0].Title)
	}
}

// ========== report_generator.go 测试 ==========

func TestNewReportGenerator(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)
	if rg == nil {
		t.Fatal("NewReportGenerator should not return nil")
	}
}

func TestGenerateReport(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	req := ScanRequest{
		Standard: StandardGDPR,
		Format:   FormatJSON,
	}

	report, err := rg.GenerateReport(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report == nil {
		t.Fatal("report should not be nil")
	}
	if report.ID == "" {
		t.Error("report should have an ID")
	}
	if report.Standard != StandardGDPR {
		t.Errorf("expected standard %s, got %s", StandardGDPR, report.Standard)
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
	if report.Format != FormatJSON {
		t.Errorf("expected format json, got %s", report.Format)
	}
}

func TestGenerateReportUnsupportedStandard(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	req := ScanRequest{
		Standard: ComplianceStandard("invalid"),
	}

	_, err := rg.GenerateReport(context.Background(), req)
	if err == nil {
		t.Error("should return error for unsupported standard")
	}
}

func TestGenerateReportWithCategories(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	req := ScanRequest{
		Standard:   StandardSOC2,
		Categories: []CheckCategory{CategoryNetwork},
	}

	report, err := rg.GenerateReport(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	// 应该只有网络相关的检查
	for _, r := range report.Results {
		if r.Category != CategoryNetwork {
			t.Errorf("expected only network results, got %s", r.Category)
		}
	}
	if report.TotalChecks != 3 {
		t.Errorf("expected 3 network checks, got %d", report.TotalChecks)
	}
}

func TestGetReport(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	req := ScanRequest{Standard: StandardGDPR}
	report, _ := rg.GenerateReport(context.Background(), req)

	found, ok := rg.GetReport(report.ID)
	if !ok {
		t.Error("should find the generated report")
	}
	if found.ID != report.ID {
		t.Errorf("expected report ID %s, got %s", report.ID, found.ID)
	}

	_, ok = rg.GetReport("nonexistent")
	if ok {
		t.Error("should not find nonexistent report")
	}
}

func TestListReports(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	// 生成两个不同标准的报告
	rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})
	rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardSOC2})

	// 列出所有
	all := rg.ListReports(nil)
	if len(all) != 2 {
		t.Errorf("expected 2 reports, got %d", len(all))
	}

	// 按标准筛选
	gdpr := StandardGDPR
	gdprReports := rg.ListReports(&gdpr)
	if len(gdprReports) != 1 {
		t.Errorf("expected 1 GDPR report, got %d", len(gdprReports))
	}

	// 不存在的标准
	hipaa := StandardHIPAA
	hipaaReports := rg.ListReports(&hipaa)
	if len(hipaaReports) != 0 {
		t.Errorf("expected 0 HIPAA reports, got %d", len(hipaaReports))
	}
}

func TestGetStatus(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	// 初始状态
	status := rg.GetStatus()
	if status.TotalReports != 0 {
		t.Errorf("expected 0 total reports initially, got %d", status.TotalReports)
	}
	if len(status.Standards) != 5 {
		t.Errorf("expected 5 standards in overview, got %d", len(status.Standards))
	}

	// 全部应为 pending_review（无扫描记录）
	for _, s := range status.Standards {
		if s.Status != StatusPendingReview {
			t.Errorf("standard %s should be pending_review initially, got %s", s.Standard, s.Status)
		}
	}

	// 生成一个报告后再查
	rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	status = rg.GetStatus()
	if status.TotalReports != 1 {
		t.Errorf("expected 1 total report, got %d", status.TotalReports)
	}
	if status.LastScanTime == nil {
		t.Error("should have last scan time after generating a report")
	}
}

func TestDetermineComplianceStatus(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	tests := []struct {
		score    int
		expected ComplianceStatus
	}{
		{100, StatusCompliant},
		{90, StatusCompliant},
		{89, StatusPendingReview},
		{60, StatusPendingReview},
		{59, StatusNonCompliant},
		{0, StatusNonCompliant},
	}

	for _, tt := range tests {
		status := rg.determineComplianceStatus(tt.score)
		if status != tt.expected {
			t.Errorf("score %d: expected %s, got %s", tt.score, tt.expected, status)
		}
	}
}

// ========== pdf_exporter.go 测试 ==========

func TestNewPDFExporter(t *testing.T) {
	sm := NewStandardsManager()
	e := NewPDFExporter(sm)
	if e == nil {
		t.Fatal("NewPDFExporter should not return nil")
	}
}

func TestExportToText(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)
	e := NewPDFExporter(sm)

	report, _ := rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	text := e.ExportToText(report)
	if text == "" {
		t.Error("exported text should not be empty")
	}
	if !strings.Contains(text, "NAS-OS 合规检查报告") {
		t.Error("text should contain title")
	}
	if !strings.Contains(text, report.ID) {
		t.Error("text should contain report ID")
	}
}

func TestExportToHTML(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)
	e := NewPDFExporter(sm)

	report, _ := rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardSOC2})

	html := e.ExportToHTML(report)
	if html == "" {
		t.Error("exported HTML should not be empty")
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("should be valid HTML")
	}
	if !strings.Contains(html, "NAS-OS 合规检查报告") {
		t.Error("HTML should contain title")
	}
	if !strings.Contains(html, report.ID) {
		t.Error("HTML should contain report ID")
	}
}

func TestExportToTextContainsRemediations(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)
	e := NewPDFExporter(sm)

	report, _ := rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardDJBH})

	text := e.ExportToText(report)
	// 如果有整改建议，文本中应包含
	if len(report.Remediations) > 0 && !strings.Contains(text, "整改建议") {
		t.Error("text should contain remediation section when remediations exist")
	}
}

func TestFormatStatus(t *testing.T) {
	sm := NewStandardsManager()
	e := NewPDFExporter(sm)

	tests := []struct {
		status   ComplianceStatus
		expected string
	}{
		{StatusCompliant, "✅ 合规"},
		{StatusNonCompliant, "❌ 不合规"},
		{StatusPendingReview, "⏳ 待审查"},
		{ComplianceStatus("unknown"), "unknown"},
	}

	for _, tt := range tests {
		result := e.formatStatus(tt.status)
		if result != tt.expected {
			t.Errorf("status %s: expected '%s', got '%s'", tt.status, tt.expected, result)
		}
	}
}

// ========== handlers.go 测试 ==========

func setupTestRouter() (*gin.Engine, *Handlers) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)
	h := NewHandlers(rg, sm)

	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	return r, h
}

func TestListStandardsHandler(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/standards", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "gdpr") {
		t.Error("response should contain gdpr")
	}
	if !strings.Contains(body, "soc2") {
		t.Error("response should contain soc2")
	}
}

func TestTriggerScanHandler(t *testing.T) {
	r, h := setupTestRouter()

	// 先生成一个报告以供后续测试
	h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	// 触发扫描
	w := httptest.NewRecorder()
	body := `{"standard": "gdpr"}`
	req, _ := http.NewRequest("POST", "/api/v1/compliance-report/scan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTriggerScanHandlerInvalidStandard(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	body := `{"standard": "invalid"}`
	req, _ := http.NewRequest("POST", "/api/v1/compliance-report/scan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTriggerScanHandlerMissingBody(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/compliance-report/scan", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListReportsHandler(t *testing.T) {
	r, h := setupTestRouter()

	// 先生成报告
	h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})
	h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardSOC2})

	// 列出所有
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListReportsHandlerWithFilter(t *testing.T) {
	r, h := setupTestRouter()

	h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports?standard=gdpr", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetReportHandler(t *testing.T) {
	r, h := setupTestRouter()

	report, _ := h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports/"+report.ID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, report.ID) {
		t.Error("response should contain report ID")
	}
}

func TestGetReportHandlerNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestExportReportHTML(t *testing.T) {
	r, h := setupTestRouter()

	report, _ := h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports/"+report.ID+"/export?format=html", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected HTML content type, got %s", ct)
	}
}

func TestExportReportText(t *testing.T) {
	r, h := setupTestRouter()

	report, _ := h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardSOC2})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports/"+report.ID+"/export?format=text", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("expected plain text content type, got %s", ct)
	}
}

func TestExportReportNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports/nonexistent/export", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestExportReportInvalidFormat(t *testing.T) {
	r, h := setupTestRouter()

	report, _ := h.generator.GenerateReport(context.Background(), ScanRequest{Standard: StandardGDPR})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/reports/"+report.ID+"/export?format=csv", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetStatusHandler(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/compliance-report/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "overall_status") {
		t.Error("response should contain overall_status")
	}
}

// ========== 集成测试 ==========

func TestFullWorkflow(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)

	// 1. 列出标准
	standards := sm.ListStandards()
	if len(standards) == 0 {
		t.Fatal("should have standards")
	}

	// 2. 对每个标准执行扫描
	for _, std := range standards {
		report, err := rg.GenerateReport(context.Background(), ScanRequest{
			Standard: std.ID,
		})
		if err != nil {
			t.Fatalf("failed to scan %s: %v", std.ID, err)
		}
		if report.Status != ScanStatusComplete {
			t.Errorf("report for %s should be complete", std.ID)
		}
	}

	// 3. 检查状态总览
	status := rg.GetStatus()
	if status.TotalReports != 5 {
		t.Errorf("expected 5 reports, got %d", status.TotalReports)
	}
	if status.LastScanTime == nil {
		t.Error("should have last scan time")
	}

	// 4. 验证所有标准都有记录
	for _, s := range status.Standards {
		if s.Status == StatusPendingReview && s.Score == 0 {
			// 有扫描记录的标准不应是 pending_review + score 0
			t.Errorf("standard %s should have been scanned", s.Standard)
		}
	}
}

func TestPDFExportWorkflow(t *testing.T) {
	sm := NewStandardsManager()
	rg := NewReportGenerator(sm)
	e := NewPDFExporter(sm)

	report, _ := rg.GenerateReport(context.Background(), ScanRequest{Standard: StandardDJBH})

	// 导出为文本
	text := e.ExportToText(report)
	if !strings.Contains(text, report.ID) {
		t.Error("text export should contain report ID")
	}

	// 导出为 HTML
	html := e.ExportToHTML(report)
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML export should be valid HTML")
	}
}
