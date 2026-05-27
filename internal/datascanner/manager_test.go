// Package datascanner 测试
package datascanner

import (
	"testing"
)

// ========== 任务管理测试 ==========

func TestCreateTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{
		Name:      "测试扫描",
		Path:      "/data/test",
		Recursive: true,
		FileTypes: []FileType{FileTypeText, FileTypePDF},
		PIITypes:  []PIIType{PIIIDCard, PIIPhone},
	})
	if task == nil {
		t.Fatal("任务不应为nil")
	}
	if task.Name != "测试扫描" {
		t.Errorf("名称不匹配: %s", task.Name)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("状态应为pending: %s", task.Status)
	}
	if len(task.FileTypes) != 2 {
		t.Errorf("文件类型数量不匹配: %d", len(task.FileTypes))
	}
}

func TestCreateTaskDefaults(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{
		Name: "全量扫描",
		Path: "/data",
	})
	if len(task.FileTypes) != 4 {
		t.Errorf("默认应有4种文件类型: %d", len(task.FileTypes))
	}
	if len(task.PIITypes) != 10 {
		t.Errorf("默认应有10种PII类型: %d", len(task.PIITypes))
	}
}

func TestGetTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("名称不匹配")
	}
}

func TestGetTaskNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetTask("nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("期望 ErrTaskNotFound，实际: %v", err)
	}
}

func TestListTasks(t *testing.T) {
	m := NewManager()
	m.CreateTask(CreateTaskRequest{Name: "task1", Path: "/a"})
	m.CreateTask(CreateTaskRequest{Name: "task2", Path: "/b"})

	tasks := m.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("期望2个任务，实际 %d", len(tasks))
	}
}

func TestDeleteTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "to delete", Path: "/tmp"})

	err := m.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = m.GetTask(task.ID)
	if err == nil {
		t.Error("已删除任务不应存在")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	m := NewManager()
	err := m.DeleteTask("nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("期望 ErrTaskNotFound，实际: %v", err)
	}
}

// ========== 任务生命周期测试 ==========

func TestStartTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "start test", Path: "/data"})

	started, err := m.StartTask(task.ID)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if started.Status != TaskStatusRunning {
		t.Errorf("状态应为running: %s", started.Status)
	}
	if started.StartedAt == nil {
		t.Error("启动时间不应为nil")
	}
}

func TestStartTaskAlreadyRunning(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})
	m.StartTask(task.ID)

	_, err := m.StartTask(task.ID)
	if err != ErrTaskRunning {
		t.Errorf("期望 ErrTaskRunning，实际: %v", err)
	}
}

func TestPauseTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "pause test", Path: "/data"})
	m.StartTask(task.ID)

	paused, err := m.PauseTask(task.ID)
	if err != nil {
		t.Fatalf("暂停失败: %v", err)
	}
	if paused.Status != TaskStatusPaused {
		t.Errorf("状态应为paused: %s", paused.Status)
	}
}

func TestPauseTaskNotRunning(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})

	_, err := m.PauseTask(task.ID)
	if err != ErrTaskNotRunning {
		t.Errorf("期望 ErrTaskNotRunning，实际: %v", err)
	}
}

func TestCancelTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "cancel test", Path: "/data"})
	m.StartTask(task.ID)

	canceled, err := m.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("取消失败: %v", err)
	}
	if canceled.Status != TaskStatusCanceled {
		t.Errorf("状态应为canceled: %s", canceled.Status)
	}
	if canceled.CompletedAt == nil {
		t.Error("完成时间不应为nil")
	}
}

func TestCompleteTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})

	err := m.CompleteTask(task.ID, 100)
	if err != nil {
		t.Fatalf("标记完成失败: %v", err)
	}

	got, _ := m.GetTask(task.ID)
	if got.Status != TaskStatusDone {
		t.Errorf("状态应为done: %s", got.Status)
	}
	if got.Progress != 1.0 {
		t.Errorf("进度应为1.0: %f", got.Progress)
	}
}

func TestUpdateTaskProgress(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})

	err := m.UpdateTaskProgress(task.ID, 50, 100)
	if err != nil {
		t.Fatalf("更新进度失败: %v", err)
	}

	got, _ := m.GetTask(task.ID)
	if got.Progress != 0.5 {
		t.Errorf("进度应为0.5: %f", got.Progress)
	}
	if got.ScannedFiles != 50 {
		t.Errorf("已扫描数应为50: %d", got.ScannedFiles)
	}
}

// ========== PII 检测测试 ==========

func TestScanIDCard(t *testing.T) {
	m := NewManager()
	content := "用户身份证号：110101199001011234，请妥善保管"
	results := m.ScanContent(content, "test.txt", []PIIType{PIIIDCard})

	if len(results) == 0 {
		t.Fatal("应检测到身份证号")
	}
	if results[0].PIIType != PIIIDCard {
		t.Errorf("类型应为id_card: %s", results[0].PIIType)
	}
	if results[0].RiskLevel != RiskHigh {
		t.Errorf("风险应为high: %s", results[0].RiskLevel)
	}
}

func TestScanPhone(t *testing.T) {
	m := NewManager()
	content := "联系方式：13812345678"
	results := m.ScanContent(content, "test.txt", []PIIType{PIIPhone})

	if len(results) == 0 {
		t.Fatal("应检测到手机号")
	}
	if results[0].PIIType != PIIPhone {
		t.Errorf("类型应为phone: %s", results[0].PIIType)
	}
}

func TestScanEmail(t *testing.T) {
	m := NewManager()
	content := "邮箱：test@example.com"
	results := m.ScanContent(content, "test.txt", []PIIType{PIIEmail})

	if len(results) == 0 {
		t.Fatal("应检测到邮箱")
	}
	if results[0].PIIType != PIIEmail {
		t.Errorf("类型应为email: %s", results[0].PIIType)
	}
}

func TestScanLicensePlate(t *testing.T) {
	m := NewManager()
	content := "车牌号：京A12345"
	results := m.ScanContent(content, "test.txt", []PIIType{PIILicensePlate})

	if len(results) == 0 {
		t.Fatal("应检测到车牌号")
	}
	if results[0].PIIType != PIILicensePlate {
		t.Errorf("类型应为license_plate: %s", results[0].PIIType)
	}
}

func TestScanCreditCode(t *testing.T) {
	m := NewManager()
	content := "统一社会信用代码：91110000MA01ABCDE0"
	results := m.ScanContent(content, "test.txt", []PIIType{PIICreditCode})

	if len(results) == 0 {
		t.Fatal("应检测到统一社会信用代码")
	}
	if results[0].PIIType != PIICreditCode {
		t.Errorf("类型应为credit_code: %s", results[0].PIIType)
	}
}

func TestScanMultiplePII(t *testing.T) {
	m := NewManager()
	content := "姓名：张三\n电话：13812345678\n邮箱：zhangsan@test.com\n身份证：110101199001011234"
	results := m.ScanContent(content, "test.txt", nil)

	if len(results) < 3 {
		t.Errorf("应检测到至少3条PII: %d", len(results))
	}
}

func TestScanContentNoMatch(t *testing.T) {
	m := NewManager()
	content := "这是一段普通的文本内容，不包含任何敏感信息。"
	results := m.ScanContent(content, "test.txt", nil)

	if len(results) != 0 {
		t.Errorf("不应检测到PII: %d", len(results))
	}
}

func TestMaskText(t *testing.T) {
	// 测试身份证脱敏
	masked := maskText("110101199001011234", PIIIDCard)
	if masked == "110101199001011234" {
		t.Error("身份证号应被脱敏")
	}

	// 测试手机号脱敏
	masked = maskText("13812345678", PIIPhone)
	if masked == "13812345678" {
		t.Error("手机号应被脱敏")
	}
	if masked != "138****5678" {
		t.Errorf("手机号脱敏格式不正确: %s", masked)
	}

	// 测试邮箱脱敏
	masked = maskText("test@example.com", PIIEmail)
	if masked == "test@example.com" {
		t.Error("邮箱应被脱敏")
	}
}

// ========== 扫描结果管理测试 ==========

func TestSubmitAndGetResults(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})

	err := m.SubmitResult(task.ID, ScanResult{
		FilePath:    "test.txt",
		LineNumber:  1,
		PIIType:     PIIPhone,
		MatchedText: "138****5678",
		RiskLevel:   RiskMedium,
		RiskScore:   60,
	})
	if err != nil {
		t.Fatalf("提交结果失败: %v", err)
	}

	results, total, err := m.GetResults(task.ID, "", "", 50, 0)
	if err != nil {
		t.Fatalf("获取结果失败: %v", err)
	}
	if total != 1 {
		t.Errorf("期望1条结果: %d", total)
	}
	if len(results) != 1 {
		t.Errorf("结果数不匹配: %d", len(results))
	}
}

func TestGetResultsWithFilter(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})

	m.SubmitResult(task.ID, ScanResult{FilePath: "a.txt", PIIType: PIIPhone, RiskLevel: RiskMedium, RiskScore: 60})
	m.SubmitResult(task.ID, ScanResult{FilePath: "b.txt", PIIType: PIIIDCard, RiskLevel: RiskHigh, RiskScore: 95})
	m.SubmitResult(task.ID, ScanResult{FilePath: "c.txt", PIIType: PIIEmail, RiskLevel: RiskMedium, RiskScore: 50})

	// 按风险等级过滤
	results, total, _ := m.GetResults(task.ID, "high", "", 50, 0)
	if total != 1 {
		t.Errorf("高风险应为1条: %d", total)
	}

	// 按PII类型过滤
	results, total, _ = m.GetResults(task.ID, "", "phone", 50, 0)
	if total != 1 {
		t.Errorf("手机号应为1条: %d", total)
	}

	// 分页
	results, total, _ = m.GetResults(task.ID, "", "", 1, 0)
	if len(results) != 1 {
		t.Errorf("limit=1 应返回1条: %d", len(results))
	}
	if total != 3 {
		t.Errorf("总数应为3: %d", total)
	}
}

func TestGetResult(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})
	m.SubmitResult(task.ID, ScanResult{FilePath: "test.txt", PIIType: PIIPhone, RiskLevel: RiskMedium, RiskScore: 60})

	results, _, _ := m.GetResults(task.ID, "", "", 50, 0)
	result, err := m.GetResult(results[0].ID)
	if err != nil {
		t.Fatalf("获取单条结果失败: %v", err)
	}
	if result.PIIType != PIIPhone {
		t.Errorf("PII类型不匹配: %s", result.PIIType)
	}
}

func TestSubmitResultTaskNotFound(t *testing.T) {
	m := NewManager()
	err := m.SubmitResult("nonexistent", ScanResult{FilePath: "test.txt"})
	if err != ErrTaskNotFound {
		t.Errorf("期望 ErrTaskNotFound: %v", err)
	}
}

// ========== 白名单测试 ==========

func TestCreateWhitelist(t *testing.T) {
	m := NewManager()
	rule := m.CreateWhitelist(CreateWhitelistRequest{
		Name:         "系统目录排除",
		ExcludeDirs:  []string{"/proc", "/sys"},
		ExcludeExts:  []string{".exe", ".bin"},
		ExcludeFiles: []string{"/etc/passwd"},
		MarkedFiles:  []string{"/data/reviewed.txt"},
	})
	if rule == nil {
		t.Fatal("白名单不应为nil")
	}
	if rule.Name != "系统目录排除" {
		t.Errorf("名称不匹配: %s", rule.Name)
	}
	if len(rule.ExcludeDirs) != 2 {
		t.Errorf("排除目录数不匹配: %d", len(rule.ExcludeDirs))
	}
}

func TestUpdateWhitelist(t *testing.T) {
	m := NewManager()
	rule := m.CreateWhitelist(CreateWhitelistRequest{
		Name:        "旧名称",
		ExcludeDirs: []string{"/tmp"},
	})

	newName := "新名称"
	updated, err := m.UpdateWhitelist(rule.ID, UpdateWhitelistRequest{
		Name:        &newName,
		ExcludeDirs: []string{"/tmp", "/var"},
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "新名称" {
		t.Errorf("名称未更新: %s", updated.Name)
	}
	if len(updated.ExcludeDirs) != 2 {
		t.Errorf("排除目录应为2个: %d", len(updated.ExcludeDirs))
	}
}

func TestDeleteWhitelist(t *testing.T) {
	m := NewManager()
	rule := m.CreateWhitelist(CreateWhitelistRequest{Name: "temp"})

	err := m.DeleteWhitelist(rule.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = m.GetWhitelist(rule.ID)
	if err == nil {
		t.Error("已删除白名单不应存在")
	}
}

func TestIsFileExcluded(t *testing.T) {
	m := NewManager()
	rule := m.CreateWhitelist(CreateWhitelistRequest{
		Name:         "test",
		ExcludeDirs:  []string{"/proc"},
		ExcludeExts:  []string{".exe"},
		ExcludeFiles: []string{"/etc/passwd"},
		MarkedFiles:  []string{"/data/reviewed.txt"},
	})

	tests := []struct {
		path     string
		expected bool
	}{
		{"/proc/1/status", true},
		{"/data/app.exe", true},
		{"/etc/passwd", true},
		{"/data/reviewed.txt", true},
		{"/home/user/file.txt", false},
	}

	for _, tt := range tests {
		got := m.IsFileExcluded(rule.ID, tt.path)
		if got != tt.expected {
			t.Errorf("path=%s: 期望 %v，实际 %v", tt.path, tt.expected, got)
		}
	}
}

func TestIsFileExcludedNoWhitelist(t *testing.T) {
	m := NewManager()
	got := m.IsFileExcluded("nonexistent", "/some/file.txt")
	if got {
		t.Error("不存在的白名单应返回false")
	}
}

// ========== 报告生成测试 ==========

func TestGenerateReport(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "report test", Path: "/data"})
	m.CompleteTask(task.ID, 10)

	// 提交一些结果
	m.SubmitResult(task.ID, ScanResult{FilePath: "a.txt", PIIType: PIIPhone, RiskLevel: RiskMedium, RiskScore: 60})
	m.SubmitResult(task.ID, ScanResult{FilePath: "b.txt", PIIType: PIIIDCard, RiskLevel: RiskHigh, RiskScore: 95})
	m.SubmitResult(task.ID, ScanResult{FilePath: "a.txt", PIIType: PIIEmail, RiskLevel: RiskMedium, RiskScore: 50})

	report, err := m.GenerateReport(task.ID, ReportJSON)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.Summary.TotalFindings != 3 {
		t.Errorf("总发现数应为3: %d", report.Summary.TotalFindings)
	}
	if report.Summary.RiskDist.High != 1 {
		t.Errorf("高风险应为1: %d", report.Summary.RiskDist.High)
	}
	if report.Summary.RiskDist.Medium != 2 {
		t.Errorf("中风险应为2: %d", report.Summary.RiskDist.Medium)
	}
	if len(report.TopRiskFiles) != 2 {
		t.Errorf("Top风险文件应为2: %d", len(report.TopRiskFiles))
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})
	report, _ := m.GenerateReport(task.ID, ReportJSON)

	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("获取报告失败: %v", err)
	}
	if got.TaskID != task.ID {
		t.Errorf("TaskID不匹配")
	}
}

func TestListReports(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})
	m.GenerateReport(task.ID, ReportJSON)
	m.GenerateReport(task.ID, ReportCSV)

	reports := m.ListReports(task.ID)
	if len(reports) != 2 {
		t.Errorf("报告数应为2: %d", len(reports))
	}
}

func TestGenerateReportTaskNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GenerateReport("nonexistent", ReportJSON)
	if err == nil {
		t.Error("不存在的任务应返回错误")
	}
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(CreateTaskRequest{Name: "test", Path: "/data"})
	m.CompleteTask(task.ID, 50)

	m.SubmitResult(task.ID, ScanResult{PIIType: PIIPhone, RiskLevel: RiskMedium, RiskScore: 60})
	m.SubmitResult(task.ID, ScanResult{PIIType: PIIIDCard, RiskLevel: RiskHigh, RiskScore: 95})
	m.SubmitResult(task.ID, ScanResult{PIIType: PIIPhone, RiskLevel: RiskMedium, RiskScore: 60})

	stats, err := m.GetStats(task.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}
	if stats.TotalFindings != 3 {
		t.Errorf("总发现数应为3: %d", stats.TotalFindings)
	}
	if stats.TotalFiles != 50 {
		t.Errorf("总文件数应为50: %d", stats.TotalFiles)
	}
	if stats.PIIDist[PIIPhone] != 2 {
		t.Errorf("手机号应出现2次: %d", stats.PIIDist[PIIPhone])
	}
}

func TestGetStatsTaskNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetStats("nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("期望 ErrTaskNotFound: %v", err)
	}
}

// ========== 脱敏策略测试 ==========

func TestGetDesensitizeStrategies(t *testing.T) {
	m := NewManager()
	strategies := m.GetDesensitizeStrategies()
	if len(strategies) == 0 {
		t.Error("应有脱敏策略")
	}

	// 验证包含常见PII类型
	found := make(map[PIIType]bool)
	for _, s := range strategies {
		found[s.PIIType] = true
	}
	if !found[PIIIDCard] {
		t.Error("应包含身份证脱敏策略")
	}
	if !found[PIIPhone] {
		t.Error("应包含手机号脱敏策略")
	}
}

// ========== 合规映射测试 ==========

func TestComplianceMapping(t *testing.T) {
	m := NewManager()
	content := "身份证：110101199001011234"
	results := m.ScanContent(content, "test.txt", []PIIType{PIIIDCard})

	if len(results) == 0 {
		t.Fatal("应检测到身份证号")
	}

	// 身份证号应映射到所有三个合规标准
	compliance := results[0].Compliance
	if len(compliance) < 2 {
		t.Errorf("身份证号合规标准应至少2个: %d", len(compliance))
	}

	hasCSL := false
	hasPIPL := false
	for _, c := range compliance {
		if c == ComplianceCSL {
			hasCSL = true
		}
		if c == CompliancePIPL {
			hasPIPL = true
		}
	}
	if !hasCSL {
		t.Error("身份证号应映射到网络安全法")
	}
	if !hasPIPL {
		t.Error("身份证号应映射到个人信息保护法")
	}
}

// ========== 风险评分测试 ==========

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected RiskLevel
	}{
		{95, RiskHigh},
		{70, RiskHigh},
		{60, RiskMedium},
		{40, RiskMedium},
		{30, RiskLow},
		{0, RiskLow},
	}

	for _, tt := range tests {
		got := scoreToLevel(tt.score)
		if got != tt.expected {
			t.Errorf("score=%f: 期望 %s，实际 %s", tt.score, tt.expected, got)
		}
	}
}
