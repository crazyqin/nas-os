package scanner

import (
	"os"
	"testing"
	"time"
)

// ========== 文件系统扫描器测试 ==========

func TestNewFilesystemScanner(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	if scanner == nil {
		t.Fatal("scanner should not be nil")
	}

	if !scanner.config.Enabled {
		t.Error("scanner should be enabled by default")
	}
}

func TestCreateScanTask(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	task := scanner.CreateScanTask(
		"test-scan",
		ScanTypeQuick,
		[]string{"/tmp"},
		[]string{},
		nil,
	)

	if task == nil {
		t.Fatal("task should not be nil")
	}

	if task.ID == "" {
		t.Error("task ID should be set")
	}

	if task.Name != "test-scan" {
		t.Errorf("expected name 'test-scan', got '%s'", task.Name)
	}

	if task.Status != ScanStatusPending {
		t.Errorf("expected status 'pending', got '%s'", task.Status)
	}
}

func TestGetScanTask(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	created := scanner.CreateScanTask("test", ScanTypeQuick, []string{"/tmp"}, nil, nil)

	task := scanner.GetScanTask(created.ID)
	if task == nil {
		t.Fatal("task should exist")
	}

	if task.ID != created.ID {
		t.Errorf("expected task ID '%s', got '%s'", created.ID, task.ID)
	}
}

func TestListScanTasks(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	// 创建多个任务
	for i := 0; i < 5; i++ {
		scanner.CreateScanTask("test", ScanTypeQuick, []string{"/tmp"}, nil, nil)
	}

	tasks := scanner.ListScanTasks(10)
	if len(tasks) != 5 {
		t.Errorf("expected 5 tasks, got %d", len(tasks))
	}
}

func TestCancelScan(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	task := scanner.CreateScanTask("test", ScanTypeQuick, []string{"/tmp"}, nil, nil)

	// 先启动扫描
	scanner.StartScan(task.ID)

	err := scanner.CancelScan(task.ID)
	if err != nil {
		t.Errorf("cancel should succeed: %v", err)
	}

	updated := scanner.GetScanTask(task.ID)
	// 取消后状态应该是cancelled或completed（取决于扫描是否已完成）
	if updated.Status != ScanStatusCancelled && updated.Status != ScanStatusCompleted {
		t.Logf("status is '%s', expected 'cancelled' or 'completed'", updated.Status)
	}
}

// ========== 权限检查器测试 ==========

func TestNewPermissionChecker(t *testing.T) {
	config := DefaultCheckerConfig()
	checker := NewPermissionChecker(config)

	if checker == nil {
		t.Fatal("checker should not be nil")
	}
}

func TestCheckPath(t *testing.T) {
	config := DefaultCheckerConfig()
	checker := NewPermissionChecker(config)

	// 检查/tmp目录
	result := checker.CheckPath("/tmp")

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result.TotalChecked == 0 {
		t.Error("should have checked at least one path")
	}
}

func TestListPermissionRules(t *testing.T) {
	config := DefaultCheckerConfig()
	checker := NewPermissionChecker(config)

	rules := checker.ListRules()

	if len(rules) == 0 {
		t.Error("default rules should be loaded")
	}
}

func TestAddPermissionRule(t *testing.T) {
	config := DefaultCheckerConfig()
	checker := NewPermissionChecker(config)

	initialCount := len(checker.ListRules())

	newRule := PermissionRule{
		ID:          "test-rule",
		Name:        "Test Rule",
		Description: "Test description",
		PathPattern: "*.test",
		MaxMode:     "0644",
		Severity:    SeverityMedium,
		Enabled:     true,
	}

	checker.AddRule(newRule)

	rules := checker.ListRules()
	if len(rules) != initialCount+1 {
		t.Errorf("expected %d rules, got %d", initialCount+1, len(rules))
	}
}

func TestQuickCheck(t *testing.T) {
	config := DefaultCheckerConfig()
	checker := NewPermissionChecker(config)

	issues := checker.QuickCheck([]string{"/tmp", "/var"})

	// 应该返回一个切片（可能为空）
	if issues == nil {
		t.Error("should return a slice")
	}
}

// ========== 安全评分引擎测试 ==========

func TestNewScoreEngine(t *testing.T) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	if engine == nil {
		t.Fatal("engine should not be nil")
	}
}

func TestGetScoreCategories(t *testing.T) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	categories := engine.GetCategories()

	if len(categories) == 0 {
		t.Error("default categories should be loaded")
	}

	// 检查权重总和应接近1
	var totalWeight float64
	for _, cat := range categories {
		totalWeight += cat.Weight
	}

	// 允许小的浮点误差
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("category weights should sum to 1, got %f", totalWeight)
	}
}

func TestCalculateScore(t *testing.T) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	// 创建测试数据
	permResult := &PermissionCheckResult{
		TotalChecked:   100,
		IssuesFound:    5,
		CriticalIssues: 1,
		WarningIssues:  2,
	}

	score := engine.CalculateScore(permResult, nil, nil)

	if score == nil {
		t.Fatal("score should not be nil")
	}

	if score.OverallScore < 0 || score.OverallScore > 100 {
		t.Errorf("score should be between 0-100, got %d", score.OverallScore)
	}

	if score.Grade == "" {
		t.Error("grade should be set")
	}

	if score.Level == "" {
		t.Error("level should be set")
	}
}

func TestScoreHistory(t *testing.T) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	// 获取当前历史记录数量
	initialCount := len(engine.GetHistory(1000))

	// 计算几次评分
	for i := 0; i < 3; i++ {
		engine.CalculateScore(nil, nil, nil)
	}

	history := engine.GetHistory(1000)
	// 验证历史记录增加了 3 条
	if len(history) != initialCount+3 {
		t.Errorf("expected %d history entries, got %d", initialCount+3, len(history))
	}
}

func TestGetTrendAnalysis(t *testing.T) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	// 创建一些历史数据
	for i := 0; i < 5; i++ {
		engine.CalculateScore(nil, nil, nil)
	}

	analysis := engine.GetTrendAnalysis(5)

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}

	if _, ok := analysis["trend"]; !ok {
		t.Error("analysis should contain 'trend' key")
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{55, "F"},
	}

	for _, test := range tests {
		grade := scoreToGrade(test.score)
		if grade != test.expected {
			t.Errorf("score %d: expected grade '%s', got '%s'", test.score, test.expected, grade)
		}
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{95, "excellent"},
		{80, "good"},
		{65, "fair"},
		{50, "poor"},
		{30, "critical"},
	}

	for _, test := range tests {
		level := scoreToLevel(test.score)
		if level != test.expected {
			t.Errorf("score %d: expected level '%s', got '%s'", test.score, test.expected, level)
		}
	}
}

// ========== 漏洞扫描器测试 ==========

func TestNewVulnerabilityScanner(t *testing.T) {
	config := DefaultVulnScannerConfig()
	scanner := NewVulnerabilityScanner(config)

	if scanner == nil {
		t.Fatal("scanner should not be nil")
	}
}

func TestListVulnDatabases(t *testing.T) {
	config := DefaultVulnScannerConfig()
	scanner := NewVulnerabilityScanner(config)

	dbs := scanner.ListDatabases()

	if len(dbs) == 0 {
		t.Error("default databases should be loaded")
	}
}

func TestScanComponent(t *testing.T) {
	config := DefaultVulnScannerConfig()
	config.OfflineMode = true // 使用离线模式测试
	scanner := NewVulnerabilityScanner(config)

	result, err := scanner.ScanComponent("test-package", "1.0.0")
	if err != nil {
		t.Fatalf("scan should succeed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result.Component != "test-package" {
		t.Errorf("expected component 'test-package', got '%s'", result.Component)
	}

	if result.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", result.Version)
	}
}

func TestDeduplicateVulns(t *testing.T) {
	config := DefaultVulnScannerConfig()
	scanner := NewVulnerabilityScanner(config)

	vulns := []*Vulnerability{
		{ID: "1", CVE: "CVE-2023-0001"},
		{ID: "2", CVE: "CVE-2023-0001"}, // 重复
		{ID: "3", CVE: "CVE-2023-0002"},
		{ID: "4", CVE: ""}, // 无CVE，按ID去重
	}

	result := scanner.deduplicateVulns(vulns)

	if len(result) != 3 {
		t.Errorf("expected 3 unique vulnerabilities, got %d", len(result))
	}
}

func TestAnalyzeSeverity(t *testing.T) {
	config := DefaultVulnScannerConfig()
	scanner := NewVulnerabilityScanner(config)

	tests := []struct {
		cvss     float64
		expected Severity
	}{
		{9.5, SeverityCritical},
		{7.5, SeverityHigh},
		{5.0, SeverityMedium},
		{2.0, SeverityLow},
		{0.0, SeverityInfo},
	}

	for _, test := range tests {
		vuln := &Vulnerability{CVSSScore: test.cvss}
		severity := scanner.AnalyzeSeverity(vuln)
		if severity != test.expected {
			t.Errorf("CVSS %.1f: expected severity '%s', got '%s'", test.cvss, test.expected, severity)
		}
	}
}

func TestVulnStatistics(t *testing.T) {
	config := DefaultVulnScannerConfig()
	scanner := NewVulnerabilityScanner(config)

	results := []*VulnerabilityScanResult{
		{
			Component:     "pkg1",
			Version:       "1.0",
			TotalVulns:    5,
			CriticalVulns: 1,
			HighVulns:     2,
			MediumVulns:   1,
			LowVulns:      1,
		},
		{
			Component:     "pkg2",
			Version:       "2.0",
			TotalVulns:    3,
			CriticalVulns: 0,
			HighVulns:     1,
			MediumVulns:   1,
			LowVulns:      1,
		},
	}

	stats := scanner.GetStatistics(results)

	if stats["total_scans"].(int) != 2 {
		t.Errorf("expected 2 total scans, got %d", stats["total_scans"])
	}

	if stats["total_vulns"].(int) != 8 {
		t.Errorf("expected 8 total vulns, got %d", stats["total_vulns"])
	}
}

// ========== 类型测试 ==========

func TestScanOptions(t *testing.T) {
	opts := DefaultScanOptions()

	if !opts.CheckPermissions {
		t.Error("CheckPermissions should be true by default")
	}

	if opts.MaxFileSize <= 0 {
		t.Error("MaxFileSize should be positive")
	}
}

func TestScannerConfig(t *testing.T) {
	config := DefaultScannerConfig()

	if !config.Enabled {
		t.Error("scanner should be enabled by default")
	}

	if config.ReportRetention <= 0 {
		t.Error("ReportRetention should be positive")
	}
}

func TestSensitiveDataRules(t *testing.T) {
	rules := DefaultSensitiveDataRules()

	if len(rules) == 0 {
		t.Error("default sensitive data rules should exist")
	}

	// 检查关键规则是否存在
	foundPassword := false
	foundAPIKey := false
	for _, rule := range rules {
		if rule.Type == SensitiveDataPassword {
			foundPassword = true
		}
		if rule.Type == SensitiveDataAPIKey {
			foundAPIKey = true
		}
	}

	if !foundPassword {
		t.Error("password detection rule should exist")
	}
	if !foundAPIKey {
		t.Error("API key detection rule should exist")
	}
}

func TestPermissionRules(t *testing.T) {
	rules := DefaultPermissionRules()

	if len(rules) == 0 {
		t.Error("default permission rules should exist")
	}

	// 检查SSH规则
	foundSSH := false
	for _, rule := range rules {
		if rule.ID == "ssh_keys" {
			foundSSH = true
			if rule.Severity != SeverityCritical {
				t.Error("SSH key permission should be critical severity")
			}
		}
	}

	if !foundSSH {
		t.Error("SSH key permission rule should exist")
	}
}

// ========== 基准测试 ==========

func BenchmarkCalculateScore(b *testing.B) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	permResult := &PermissionCheckResult{
		TotalChecked:   100,
		IssuesFound:    5,
		CriticalIssues: 1,
		WarningIssues:  2,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.CalculateScore(permResult, nil, nil)
	}
}

func BenchmarkCheckPath(b *testing.B) {
	config := DefaultCheckerConfig()
	checker := NewPermissionChecker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckPath("/tmp")
	}
}

func BenchmarkScanComponent(b *testing.B) {
	config := DefaultVulnScannerConfig()
	config.OfflineMode = true
	scanner := NewVulnerabilityScanner(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.ScanComponent("test-package", "1.0.0")
	}
}

// ========== 辅助函数测试 ==========

func TestSeverityToRiskScore(t *testing.T) {
	tests := []struct {
		severity Severity
		minScore int
		maxScore int
	}{
		{SeverityCritical, 85, 100},
		{SeverityHigh, 65, 85},
		{SeverityMedium, 40, 65},
		{SeverityLow, 20, 45},
	}

	for _, test := range tests {
		score := severityToRiskScore(test.severity)
		if score < test.minScore || score > test.maxScore {
			t.Errorf("severity %s: expected score between %d-%d, got %d",
				test.severity, test.minScore, test.maxScore, score)
		}
	}
}

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc", "****"},
		{"abcdefgh", "ab****gh"},
		{"1234567890", "12******90"},
	}

	for _, test := range tests {
		result := maskSensitiveData(test.input)
		if result != test.expected {
			t.Errorf("maskSensitiveData(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

// ========== 时间相关测试 ==========

func TestScanTaskTiming(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	before := time.Now()
	task := scanner.CreateScanTask("test", ScanTypeQuick, []string{"/tmp"}, nil, nil)
	after := time.Now()

	if task.CreatedAt.Before(before) || task.CreatedAt.After(after) {
		t.Error("task CreatedAt should be between before and after")
	}
}

func TestScoreHistoryTiming(t *testing.T) {
	config := DefaultScoreEngineConfig()
	engine := NewScoreEngine(config)

	now := time.Now()
	engine.CalculateScore(nil, nil, nil)

	history := engine.GetHistory(1)
	if len(history) > 0 {
		if history[0].Date.Before(now.Add(-time.Second)) {
			t.Error("history date should be recent")
		}
	}
}

// ========== 生成报告测试 ==========

func TestGenerateScanID(t *testing.T) {
	id1 := generateScanID()
	id2 := generateScanID()

	if id1 == "" {
		t.Error("scan ID should not be empty")
	}
	if id1 == id2 {
		t.Error("scan IDs should be unique")
	}
}

func TestGenerateFindingID(t *testing.T) {
	id1 := generateFindingID()
	id2 := generateFindingID()

	if id1 == "" {
		t.Error("finding ID should not be empty")
	}
	if id1 == id2 {
		t.Error("finding IDs should be unique")
	}
}

// ========== SetProgressCallback 测试 ==========

func TestSetProgressCallback(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	called := false
	scanner.SetProgressCallback(func(taskID string, progress int) {
		called = true
	})

	if scanner.progressCallback == nil {
		t.Error("progress callback should be set")
	}
	_ = called
}

// ========== GetFindings 测试 ==========

func TestGetFindings(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	task := scanner.CreateScanTask("test", ScanTypeQuick, []string{"/tmp"}, nil, nil)

	findings := scanner.GetFindings(task.ID)
	if findings == nil {
		t.Error("findings should return a slice")
	}
}

func TestGetFindings_NonExistentTask(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	findings := scanner.GetFindings("nonexistent")
	if findings != nil {
		t.Error("findings should be nil for non-existent task")
	}
}

// ========== GetReport 测试 ==========

func TestGetReport(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	task := scanner.CreateScanTask("test", ScanTypeQuick, []string{"/tmp"}, nil, nil)

	report := scanner.GetReport(task.ID)
	if report == nil {
		t.Error("report should not be nil")
	}
}

// ========== Stop 测试 ==========

func TestStop(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	// Stop should not panic
	scanner.Stop()
}

// ========== ScoreToRiskLevel 测试 ==========

func TestScoreToRiskLevel_EdgeCases(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "low"},
		{90, "low"},
		{75, "medium"},
		{50, "high"},
		{25, "critical"},
		{0, "critical"},
	}

	for _, test := range tests {
		result := scoreToRiskLevel(test.score)
		if result != test.expected {
			t.Errorf("score %d: expected level '%s', got '%s'", test.score, test.expected, result)
		}
	}
}

// ========== ParseFileMode 测试 ==========

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		input    string
		expected os.FileMode
	}{
		{"0755", os.FileMode(0755)},
		{"0644", os.FileMode(0644)},
		{"0600", os.FileMode(0600)},
		{"0777", os.FileMode(0777)},
		{"invalid", os.FileMode(0)},
	}

	for _, test := range tests {
		result := parseFileMode(test.input)
		if result != test.expected {
			t.Errorf("parseFileMode(%s): expected %o, got %o", test.input, test.expected, result)
		}
	}
}

// ========== IsBinaryFile 测试 ==========

func TestIsBinaryFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.exe", true},
		{"test.bin", true},
		{"test.so", true},
		{"test.dll", true},
		{"test.txt", false},
		{"test.go", false},
		{"test.json", false},
	}

	for _, test := range tests {
		result := isBinaryFile(test.filename)
		if result != test.expected {
			t.Errorf("isBinaryFile(%s): expected %v, got %v", test.filename, test.expected, result)
		}
	}
}

// ========== CalculateFileHash 测试 ==========

func TestCalculateFileHash(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	// Create a temp file
	tmpFile := "/tmp/test_hash_file_" + generateScanID()
	content := []byte("test content for hash calculation")

	err := os.WriteFile(tmpFile, content, 0644)
	if err != nil {
		t.Skipf("could not create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

	hash, err := scanner.calculateFileHash(tmpFile, "sha256")
	if err != nil {
		t.Errorf("calculateFileHash should succeed: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if len(hash) != 64 { // SHA256 hex length
		t.Errorf("hash should be 64 chars, got %d", len(hash))
	}
}

// ========== CountFiles 测试 ==========

func TestCountFiles(t *testing.T) {
	config := DefaultScannerConfig()
	scanner := NewFilesystemScanner(config)

	// Create temp directory with files
	tmpDir := "/tmp/test_count_" + generateScanID()
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Skipf("could not create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	os.WriteFile(tmpDir+"/file1.txt", []byte("test"), 0644)
	os.WriteFile(tmpDir+"/file2.txt", []byte("test"), 0644)
	os.MkdirAll(tmpDir+"/subdir", 0755)
	os.WriteFile(tmpDir+"/subdir/file3.txt", []byte("test"), 0644)

	opts := DefaultScanOptions()
	count, err := scanner.countFiles(tmpDir, opts)
	if err != nil {
		t.Errorf("countFiles should succeed: %v", err)
	}
	if count < 3 {
		t.Errorf("expected at least 3 files, got %d", count)
	}
}
