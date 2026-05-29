// Package gdprscanner 测试
package gdprscanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewScannerManager(t *testing.T) {
	m := NewScannerManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}

	patterns := m.GetPatterns()
	if len(patterns) == 0 {
		t.Fatal("should have default patterns")
	}

	// 验证默认模式数量
	if len(patterns) != 7 {
		t.Errorf("expected 7 default patterns, got %d", len(patterns))
	}
}

func TestSensitivityLevelString(t *testing.T) {
	tests := []struct {
		level    SensitivityLevel
		expected string
	}{
		{SensitivityLow, "低"},
		{SensitivityMedium, "中"},
		{SensitivityHigh, "高"},
		{SensitivityLevel(99), "未知"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("SensitivityLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestScanSingleFile(t *testing.T) {
	m := NewScannerManager()

	// 创建临时测试文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := `姓名: 张三
手机: 13812345678
邮箱: zhangsan@example.com
身份证: 110101199001011234
银行卡: 6225881234567890
IP: 192.168.1.100`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	report, err := m.ScanFiles(ScanRequest{
		Path: testFile,
	})
	if err != nil {
		t.Fatalf("scan files failed: %v", err)
	}

	if report.TotalMatches == 0 {
		t.Error("expected matches but got 0")
	}

	if len(report.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}

	// 验证检测到的 PII 类型
	foundCategories := make(map[PIICategory]bool)
	for _, match := range result.Matches {
		foundCategories[match.Category] = true
	}

	expectedCategories := []PIICategory{CategoryPhone, CategoryEmail, CategoryIDCard, CategoryBankCard, CategoryIPAddress}
	for _, cat := range expectedCategories {
		if !foundCategories[cat] {
			t.Errorf("expected to find category %s", cat)
		}
	}
}

func TestScanDirectory(t *testing.T) {
	m := NewScannerManager()

	// 创建临时目录结构
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 文件1：包含 PII
	file1 := filepath.Join(tmpDir, "data.txt")
	content1 := "手机: 13912345678\n邮箱: test@test.com"
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("write file1: %v", err)
	}

	// 文件2：不包含 PII
	file2 := filepath.Join(subDir, "clean.txt")
	content2 := "这是一段普通文本，没有敏感信息。"
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	report, err := m.ScanFiles(ScanRequest{
		Path: tmpDir,
	})
	if err != nil {
		t.Fatalf("scan directory failed: %v", err)
	}

	if report.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", report.TotalFiles)
	}

	if report.TotalMatches == 0 {
		t.Error("expected matches in file1")
	}
}

func TestScanWithExcludedDirs(t *testing.T) {
	m := NewScannerManager()

	tmpDir := t.TempDir()
	excludeDir := filepath.Join(tmpDir, "exclude")
	if err := os.MkdirAll(excludeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 被排除目录中的文件
	excludeFile := filepath.Join(excludeDir, "secret.txt")
	content := "身份证: 110101199001011234"
	if err := os.WriteFile(excludeFile, []byte(content), 0644); err != nil {
		t.Fatalf("write exclude file: %v", err)
	}

	// 正常文件
	normalFile := filepath.Join(tmpDir, "normal.txt")
	content2 := "普通文本"
	if err := os.WriteFile(normalFile, []byte(content2), 0644); err != nil {
		t.Fatalf("write normal file: %v", err)
	}

	report, err := m.ScanFiles(ScanRequest{
		Path:        tmpDir,
		ExcludeDirs: []string{"exclude"},
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// 排除目录中的文件不应被扫描
	if report.TotalMatches > 0 {
		t.Error("expected no matches since excluded dir contains the PII")
	}
}

func TestScanWithCustomExtensions(t *testing.T) {
	m := NewScannerManager()

	tmpDir := t.TempDir()

	// .log 文件包含 PII
	logFile := filepath.Join(tmpDir, "app.log")
	logContent := "用户手机: 15012345678"
	if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// .xyz 文件也包含 PII，但不在扩展名列表中
	xyzFile := filepath.Join(tmpDir, "data.xyz")
	xyzContent := "邮箱: secret@example.com"
	if err := os.WriteFile(xyzFile, []byte(xyzContent), 0644); err != nil {
		t.Fatalf("write xyz: %v", err)
	}

	// 只扫描 .log 文件
	report, err := m.ScanFiles(ScanRequest{
		Path:       tmpDir,
		Extensions: []string{".log"},
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if report.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", report.TotalFiles)
	}
}

func TestClassifyPII(t *testing.T) {
	m := NewScannerManager()

	matches := []PIIMatch{
		{Category: CategoryIDCard},
		{Category: CategoryIDCard},
		{Category: CategoryPhone},
		{Category: CategoryEmail},
		{Category: CategoryEmail},
		{Category: CategoryEmail},
		{Category: CategoryBankCard},
	}

	summary := m.ClassifyPII(matches)

	if summary.IDCardCount != 2 {
		t.Errorf("expected 2 ID cards, got %d", summary.IDCardCount)
	}
	if summary.PhoneCount != 1 {
		t.Errorf("expected 1 phone, got %d", summary.PhoneCount)
	}
	if summary.EmailCount != 3 {
		t.Errorf("expected 3 emails, got %d", summary.EmailCount)
	}
	if summary.BankCardCount != 1 {
		t.Errorf("expected 1 bank card, got %d", summary.BankCardCount)
	}
}

func TestGenerateComplianceReport(t *testing.T) {
	m := NewScannerManager()

	results := []*ScanResult{
		{
			FilePath:   "test1.txt",
			TotalMatch: 3,
			Matches: []PIIMatch{
				{Category: CategoryIDCard},
				{Category: CategoryPhone},
				{Category: CategoryEmail},
			},
		},
		{
			FilePath:   "test2.txt",
			TotalMatch: 1,
			Matches: []PIIMatch{
				{Category: CategoryBankCard},
			},
		},
	}

	report := m.GenerateComplianceReport(results)

	if report.ID == "" {
		t.Error("report should have an ID")
	}
	if report.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", report.TotalFiles)
	}
	if report.TotalMatches != 4 {
		t.Errorf("expected 4 matches, got %d", report.TotalMatches)
	}
	if report.RiskLevel != "高" {
		t.Errorf("expected high risk level, got %s", report.RiskLevel)
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func TestRiskLevelCalculation(t *testing.T) {
	m := NewScannerManager()

	tests := []struct {
		name     string
		summary  CategorySummary
		total    int
		expected string
	}{
		{
			name:     "高风险-有身份证",
			summary:  CategorySummary{IDCardCount: 1},
			total:    1,
			expected: "高",
		},
		{
			name:     "高风险-有银行卡",
			summary:  CategorySummary{BankCardCount: 1},
			total:    1,
			expected: "高",
		},
		{
			name:     "中风险-大量手机号",
			summary:  CategorySummary{PhoneCount: 20},
			total:    20,
			expected: "中",
		},
		{
			name:     "安全-无匹配",
			summary:  CategorySummary{},
			total:    0,
			expected: "安全",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			riskLevel := m.calculateRiskLevel(tt.summary, tt.total)
			if riskLevel != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, riskLevel)
			}
		})
	}
}

func TestSuggestMasking(t *testing.T) {
	m := NewScannerManager()

	suggestions := m.SuggestMasking()
	if len(suggestions) == 0 {
		t.Fatal("expected masking suggestions")
	}

	// 验证每个类别都有建议
	categories := make(map[PIICategory]bool)
	for _, s := range suggestions {
		categories[s.Category] = true
		if s.Strategy == "" {
			t.Errorf("strategy should not be empty for %s", s.Category)
		}
		if s.Example == "" {
			t.Errorf("example should not be empty for %s", s.Category)
		}
	}

	expectedCategories := []PIICategory{
		CategoryIDCard, CategoryPhone, CategoryEmail,
		CategoryBankCard, CategoryPassport, CategoryLicensePlate,
		CategoryIPAddress,
	}
	for _, cat := range expectedCategories {
		if !categories[cat] {
			t.Errorf("missing masking suggestion for category %s", cat)
		}
	}
}

func TestGetReport(t *testing.T) {
	m := NewScannerManager()

	// 创建临时文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "邮箱: test@example.com"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// 扫描生成报告
	report, err := m.ScanFiles(ScanRequest{Path: testFile})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// 获取报告
	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("get report failed: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected report ID %s, got %s", report.ID, got.ID)
	}

	// 获取不存在的报告
	_, err = m.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestListReports(t *testing.T) {
	m := NewScannerManager()

	// 初始应该为空
	reports := m.ListReports()
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}

	// 创建临时文件并扫描
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "手机: 13800138000"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	m.ScanFiles(ScanRequest{Path: testFile})

	reports = m.ListReports()
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}
}

func TestScanEmptyPath(t *testing.T) {
	m := NewScannerManager()

	_, err := m.ScanFiles(ScanRequest{Path: ""})
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestScanInvalidPath(t *testing.T) {
	m := NewScannerManager()

	_, err := m.ScanFiles(ScanRequest{Path: "/nonexistent/path/that/does/not/exist"})
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestScanFileWithNoPII(t *testing.T) {
	m := NewScannerManager()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "clean.txt")
	content := "这是一段完全普通的文本，不包含任何个人信息。"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	report, err := m.ScanFiles(ScanRequest{Path: testFile})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if report.TotalMatches != 0 {
		t.Errorf("expected 0 matches, got %d", report.TotalMatches)
	}

	if report.RiskLevel != "安全" {
		t.Errorf("expected safe risk level, got %s", report.RiskLevel)
	}
}

func TestGetPatterns(t *testing.T) {
	m := NewScannerManager()

	patterns := m.GetPatterns()
	if len(patterns) != 7 {
		t.Errorf("expected 7 patterns, got %d", len(patterns))
	}

	// 验证每个模式都有必要的字段
	for _, p := range patterns {
		if p.Name == "" {
			t.Error("pattern name should not be empty")
		}
		if p.Pattern == "" {
			t.Error("pattern regex should not be empty")
		}
		if p.Category == "" {
			t.Error("pattern category should not be empty")
		}
	}
}
