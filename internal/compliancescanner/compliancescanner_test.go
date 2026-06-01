// Package compliancescanner 提供安全合规扫描功能
package compliancescanner

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil, nil)
	if engine == nil {
		t.Fatal("NewEngine 返回 nil")
	}

	if engine.IsRunning() {
		t.Error("新创建的引擎不应处于运行状态")
	}
}

func TestEngineStartStop(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 启动引擎
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("启动引擎失败: %v", err)
	}

	if !engine.IsRunning() {
		t.Error("引擎应处于运行状态")
	}

	// 重复启动应失败
	if err := engine.Start(ctx); err == nil {
		t.Error("重复启动应返回错误")
	}

	// 停止引擎
	if err := engine.Stop(); err != nil {
		t.Fatalf("停止引擎失败: %v", err)
	}

	if engine.IsRunning() {
		t.Error("引擎应处于停止状态")
	}
}

func TestRunScan(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	report, err := engine.RunScan(ctx, nil)
	if err != nil {
		t.Fatalf("执行扫描失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为 nil")
	}

	if report.ID == "" {
		t.Error("报告ID不应为空")
	}

	if report.TotalChecks == 0 {
		t.Error("总检查数不应为 0")
	}

	if report.OverallScore < 0 || report.OverallScore > 100 {
		t.Errorf("评分超出范围: %f", report.OverallScore)
	}

	if report.ComplianceLevel == "" {
		t.Error("合规等级不应为空")
	}
}

func TestGetReport(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	report, _ := engine.RunScan(ctx, nil)

	// 获取存在的报告
	fetched, err := engine.GetReport(report.ID)
	if err != nil {
		t.Fatalf("获取报告失败: %v", err)
	}

	if fetched.ID != report.ID {
		t.Errorf("报告ID不匹配: %s != %s", fetched.ID, report.ID)
	}

	// 获取不存在的报告
	_, err = engine.GetReport("non-existent")
	if err == nil {
		t.Error("获取不存在的报告应返回错误")
	}
}

func TestGetAllReports(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 执行多次扫描
	for i := 0; i < 3; i++ {
		engine.RunScan(ctx, nil)
	}

	reports := engine.GetAllReports()
	if len(reports) != 3 {
		t.Errorf("期望 3 个报告，得到 %d", len(reports))
	}
}

func TestScheduleScan(t *testing.T) {
	engine := NewEngine(nil, nil)

	schedule := &ScanSchedule{
		Name:      "test-schedule",
		CronExpr:  "0 0 * * *",
		Standards: []ComplianceStandard{StandardCIS},
		Enabled:   true,
	}

	if err := engine.ScheduleScan(schedule); err != nil {
		t.Fatalf("添加调度失败: %v", err)
	}

	if schedule.ID == "" {
		t.Error("调度ID不应为空")
	}

	// 获取调度
	fetched, err := engine.GetSchedule(schedule.ID)
	if err != nil {
		t.Fatalf("获取调度失败: %v", err)
	}

	if fetched.Name != schedule.Name {
		t.Errorf("调度名称不匹配: %s != %s", fetched.Name, schedule.Name)
	}
}

func TestRemoveSchedule(t *testing.T) {
	engine := NewEngine(nil, nil)

	schedule := &ScanSchedule{
		Name:      "test-schedule",
		CronExpr:  "0 0 * * *",
		Standards: []ComplianceStandard{StandardCIS},
		Enabled:   true,
	}

	engine.ScheduleScan(schedule)

	if err := engine.RemoveSchedule(schedule.ID); err != nil {
		t.Fatalf("移除调度失败: %v", err)
	}

	// 获取已移除的调度应失败
	_, err := engine.GetSchedule(schedule.ID)
	if err == nil {
		t.Error("获取已移除的调度应返回错误")
	}
}

func TestGetAllSchedules(t *testing.T) {
	engine := NewEngine(nil, nil)

	for i := 0; i < 3; i++ {
		engine.ScheduleScan(&ScanSchedule{
			Name:      "schedule-" + string(rune('0'+i)),
			CronExpr:  "0 0 * * *",
			Standards: []ComplianceStandard{StandardCIS},
			Enabled:   true,
		})
	}

	schedules := engine.GetAllSchedules()
	if len(schedules) != 3 {
		t.Errorf("期望 3 个调度，得到 %d", len(schedules))
	}
}

func TestGetStats(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 执行扫描
	engine.RunScan(ctx, nil)

	stats := engine.GetStats()
	if stats == nil {
		t.Fatal("统计信息不应为 nil")
	}

	if stats.TotalScans != 1 {
		t.Errorf("总扫描次数应为 1，得到 %d", stats.TotalScans)
	}
}

func TestRuleManager(t *testing.T) {
	rm := NewRuleManager(nil)

	// 获取所有规则
	rules := rm.GetAllRules()
	if len(rules) == 0 {
		t.Error("应有内置规则")
	}

	// 获取CIS规则
	cisRules := rm.GetRulesByStandard(StandardCIS)
	if len(cisRules) == 0 {
		t.Error("应有CIS规则")
	}

	// 获取等保规则
	mlps2Rules := rm.GetRulesByStandard(StandardMLPS2)
	if len(mlps2Rules) == 0 {
		t.Error("应有等保规则")
	}

	// 获取启用的规则
	enabledRules := rm.GetEnabledRules()
	if len(enabledRules) == 0 {
		t.Error("应有启用的规则")
	}
}

func TestAddRule(t *testing.T) {
	rm := NewRuleManager(nil)

	rule := &ScanRule{
		ID:       "TEST-001",
		Name:     "测试规则",
		Standard: StandardCustom,
		Category: CategorySystemConfig,
		Severity: SeverityMedium,
		Enabled:  true,
	}

	if err := rm.AddRule(rule); err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	// 获取规则
	fetched, err := rm.GetRule("TEST-001")
	if err != nil {
		t.Fatalf("获取规则失败: %v", err)
	}

	if fetched.Name != rule.Name {
		t.Errorf("规则名称不匹配: %s != %s", fetched.Name, rule.Name)
	}

	// 重复添加应失败
	if err := rm.AddRule(rule); err == nil {
		t.Error("重复添加规则应返回错误")
	}
}

func TestDeleteRule(t *testing.T) {
	rm := NewRuleManager(nil)

	rule := &ScanRule{
		ID:       "TEST-001",
		Name:     "测试规则",
		Standard: StandardCustom,
		Category: CategorySystemConfig,
		Severity: SeverityMedium,
		Enabled:  true,
	}

	rm.AddRule(rule)

	if err := rm.DeleteRule("TEST-001"); err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}

	// 获取已删除的规则应失败
	_, err := rm.GetRule("TEST-001")
	if err == nil {
		t.Error("获取已删除的规则应返回错误")
	}
}

func TestEnableDisableRule(t *testing.T) {
	rm := NewRuleManager(nil)

	rule := &ScanRule{
		ID:       "TEST-001",
		Name:     "测试规则",
		Standard: StandardCustom,
		Category: CategorySystemConfig,
		Severity: SeverityMedium,
		Enabled:  true,
	}

	rm.AddRule(rule)

	// 禁用规则
	if err := rm.DisableRule("TEST-001"); err != nil {
		t.Fatalf("禁用规则失败: %v", err)
	}

	fetched, _ := rm.GetRule("TEST-001")
	if fetched.Enabled {
		t.Error("规则应被禁用")
	}

	// 启用规则
	if err := rm.EnableRule("TEST-001"); err != nil {
		t.Fatalf("启用规则失败: %v", err)
	}

	fetched, _ = rm.GetRule("TEST-001")
	if !fetched.Enabled {
		t.Error("规则应被启用")
	}
}

func TestSearchRules(t *testing.T) {
	rm := NewRuleManager(nil)

	rules := rm.SearchRules("SSH")
	if len(rules) == 0 {
		t.Error("应找到SSH相关规则")
	}

	for _, rule := range rules {
		if rule.Name == "" && rule.Description == "" {
			t.Error("搜索结果应包含名称或描述")
		}
	}
}

func TestGetRuleStats(t *testing.T) {
	rm := NewRuleManager(nil)

	stats := rm.GetRuleStats()
	if stats["total"] == 0 {
		t.Error("总规则数不应为 0")
	}

	if stats["enabled"] == 0 {
		t.Error("启用规则数不应为 0")
	}
}

func TestScanner(t *testing.T) {
	scanner := NewScanner(nil, 0)

	// 注册检查函数
	scanner.RegisterCheckFunction("test_check", func(ctx context.Context) (*ScanResult, error) {
		return &ScanResult{
			Category:  CategorySystemConfig,
			Severity:  SeverityMedium,
			Result:    ResultPass,
			CheckedAt: time.Now(),
		}, nil
	})

	// 执行检查
	result, err := scanner.ExecuteCheck(context.Background(), "test_check")
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	if result.Result != ResultPass {
		t.Errorf("检查结果应为 pass，得到 %s", result.Result)
	}
}

func TestScannerExecuteCheckNotFound(t *testing.T) {
	scanner := NewScanner(nil, 0)

	_, err := scanner.ExecuteCheck(context.Background(), "non_existent")
	if err == nil {
		t.Error("执行不存在的检查函数应返回错误")
	}
}

func TestRemediationEngine(t *testing.T) {
	scanner := NewScanner(nil, 0)
	re := NewRemediationEngine(nil, scanner)

	// 注册测试修复函数
	scanner.RegisterFixFunction("test_fix", func(ctx context.Context) error {
		return nil
	})

	// 注册测试检查函数
	scanner.RegisterCheckFunction("test_check", func(ctx context.Context) (*ScanResult, error) {
		return &ScanResult{
			Result: ResultPass,
		}, nil
	})

	// 创建测试结果
	result := &ScanResult{
		ID:       "result-001",
		RuleID:   "TEST-001",
		Category: CategorySystemConfig,
		Severity: SeverityHigh,
		Result:   ResultFail,
	}

	// 测试修复建议
	suggestions := re.SuggestRemediation(result)
	if len(suggestions) == 0 {
		t.Error("应生成修复建议")
	}

	// 测试通过的结果不需要修复
	passResult := &ScanResult{Result: ResultPass}
	suggestions = re.SuggestRemediation(passResult)
	if len(suggestions) != 0 {
		t.Error("通过的结果不应有修复建议")
	}
}

func TestRemediationStats(t *testing.T) {
	scanner := NewScanner(nil, 0)
	re := NewRemediationEngine(nil, scanner)

	stats := re.GetStats()
	if stats["total"] != 0 {
		t.Error("初始修复记录数应为 0")
	}
}

func TestGenerateReport(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	report, _ := engine.RunScan(ctx, nil)

	// 验证报告字段
	if report.TotalChecks == 0 {
		t.Error("总检查数不应为 0")
	}

	if report.PassedChecks+report.FailedChecks+report.WarningChecks+report.SkippedChecks+report.ErrorChecks != report.TotalChecks {
		t.Error("检查数统计不一致")
	}

	if len(report.CategorySummary) == 0 {
		t.Error("应有分类摘要")
	}

	if len(report.SeveritySummary) == 0 {
		t.Error("应有严重级别摘要")
	}
}

func TestComplianceLevel(t *testing.T) {
	engine := NewEngine(nil, nil)

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "A"},
		{85, "B"},
		{70, "C"},
		{50, "D"},
	}

	for _, tt := range tests {
		level := engine.calculateComplianceLevel(tt.score)
		if level != tt.expected {
			t.Errorf("评分 %f 的合规等级应为 %s，得到 %s", tt.score, tt.expected, level)
		}
	}
}

func TestDefaultScanConfig(t *testing.T) {
	config := DefaultScanConfig()

	if config == nil {
		t.Fatal("默认配置不应为 nil")
	}

	if len(config.Standards) == 0 {
		t.Error("默认配置应包含合规标准")
	}

	if config.MaxConcurrent <= 0 {
		t.Error("最大并发数应大于 0")
	}

	if config.Timeout <= 0 {
		t.Error("超时时间应大于 0")
	}
}

func TestGetAllRecords(t *testing.T) {
	scanner := NewScanner(nil, 0)
	re := NewRemediationEngine(nil, scanner)

	records := re.GetAllRecords()
	if len(records) != 0 {
		t.Error("初始修复记录应为空")
	}
}

func TestClearRecords(t *testing.T) {
	scanner := NewScanner(nil, 0)
	re := NewRemediationEngine(nil, scanner)

	re.ClearRecords()

	stats := re.GetStats()
	if stats["total"] != 0 {
		t.Error("清除后修复记录数应为 0")
	}
}
