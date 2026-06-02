package aisysadmin

import (
	"context"
	"testing"
	"time"
)

func TestNewAISysAdmin(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
		LogRetention:   30,
	}

	admin := New(config)
	if admin == nil {
		t.Fatal("New returned nil")
	}

	if admin.config != config {
		t.Error("config not set correctly")
	}

	if admin.running {
		t.Error("expected running to be false")
	}
}

func TestStartStop(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)

	// 启动
	err := admin.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !admin.running {
		t.Error("expected running to be true")
	}

	// 重复启动
	err = admin.Start()
	if err == nil {
		t.Error("expected error when starting again")
	}

	// 停止
	err = admin.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if admin.running {
		t.Error("expected running to be false")
	}

	// 重复停止
	err = admin.Stop()
	if err == nil {
		t.Error("expected error when stopping again")
	}
}

func TestExecuteCommand(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 执行重启命令
	cmd, err := admin.ExecuteCommand(ctx, "重启服务")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}

	if cmd.Status != CommandStatusCompleted {
		t.Errorf("expected status completed, got %s", cmd.Status)
	}

	if cmd.Action != "restart" {
		t.Errorf("expected action restart, got %s", cmd.Action)
	}

	// 执行清理命令
	cmd, err = admin.ExecuteCommand(ctx, "清理缓存")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}

	if cmd.Action != "clean" {
		t.Errorf("expected action clean, got %s", cmd.Action)
	}

	// 执行未知命令
	cmd, err = admin.ExecuteCommand(ctx, "做一些事情")
	if err != nil {
		t.Fatalf("ExecuteCommand returned unexpected error: %v", err)
	}
	if cmd.Status != CommandStatusFailed {
		t.Errorf("expected status failed for unknown command, got %s", cmd.Status)
	}
	if cmd.Error == "" {
		t.Error("expected error message for unknown command")
	}
}

func TestExecuteCommandNotRunning(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
	}

	admin := New(config)
	ctx := context.Background()

	_, err := admin.ExecuteCommand(ctx, "重启服务")
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestDiagnoseSystem(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	results, err := admin.DiagnoseSystem(ctx)
	if err != nil {
		t.Fatalf("DiagnoseSystem failed: %v", err)
	}

	// 应该有5个诊断结果 (CPU, 内存, 磁盘, 网络, 服务)
	if len(results) != 5 {
		t.Fatalf("expected 5 diagnosis results, got %d", len(results))
	}

	// 验证每个诊断结果
	categories := map[string]bool{
		"cpu":     false,
		"memory":  false,
		"disk":    false,
		"network": false,
		"service": false,
	}

	for _, r := range results {
		if _, ok := categories[r.Category]; ok {
			categories[r.Category] = true
		}
		if r.Status != DiagnosisStatusHealthy {
			t.Errorf("expected healthy status for %s, got %s", r.Category, r.Status)
		}
	}

	for cat, found := range categories {
		if !found {
			t.Errorf("missing diagnosis for category: %s", cat)
		}
	}
}

func TestAutoRepair(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	issue := &DiagnosisResult{
		ID:         "test_issue_1",
		Category:   "cpu",
		Component:  "处理器",
		Status:     DiagnosisStatusWarning,
		Message:    "CPU 使用率过高",
		Severity:   SeverityHigh,
		Timestamp:  time.Now(),
	}

	log, err := admin.AutoRepair(ctx, issue)
	if err != nil {
		t.Fatalf("AutoRepair failed: %v", err)
	}

	if !log.Success {
		t.Error("expected repair to be successful")
	}

	if log.Action != "auto_repair_cpu" {
		t.Errorf("expected action auto_repair_cpu, got %s", log.Action)
	}
}

func TestAutoRepairDisabled(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     false,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	issue := &DiagnosisResult{
		ID:         "test_issue_1",
		Category:   "cpu",
		Component:  "处理器",
		Status:     DiagnosisStatusWarning,
		Message:    "CPU 使用率过高",
		Severity:   SeverityHigh,
		Timestamp:  time.Now(),
	}

	_, err := admin.AutoRepair(ctx, issue)
	if err == nil {
		t.Error("expected error when auto repair is disabled")
	}
}

func TestGetOperationHistory(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 执行几个命令
	admin.ExecuteCommand(ctx, "重启服务")
	admin.ExecuteCommand(ctx, "清理缓存")
	admin.ExecuteCommand(ctx, "备份配置")

	// 获取历史
	history := admin.GetOperationHistory(10)
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	// 获取限制数量的历史
	history = admin.GetOperationHistory(2)
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
}

func TestGetSystemSummary(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 执行一些操作
	admin.ExecuteCommand(ctx, "重启服务")
	admin.ExecuteCommand(ctx, "清理缓存")
	admin.ExecuteCommand(ctx, "未知命令")

	// 获取摘要
	summary := admin.GetSystemSummary()

	if summary.TotalCommands != 3 {
		t.Errorf("expected 3 total commands, got %d", summary.TotalCommands)
	}

	if summary.SuccessCommands != 2 {
		t.Errorf("expected 2 success commands, got %d", summary.SuccessCommands)
	}

	if summary.FailedCommands != 1 {
		t.Errorf("expected 1 failed command, got %d", summary.FailedCommands)
	}
}

func TestAddAndGetRules(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)

	// 添加规则
	rule := &AutomationRule{
		ID:          "rule_1",
		Name:        "CPU高负载告警",
		Description: "当CPU使用率超过90%时触发",
		Condition:   "cpu > 90",
		Action:      "notify",
		Enabled:     true,
	}

	err := admin.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// 验证规则已添加
	rules := admin.GetRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	if rules[0].Name != "CPU高负载告警" {
		t.Errorf("expected rule name 'CPU高负载告警', got '%s'", rules[0].Name)
	}
}

func TestAddDuplicateRule(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)

	rule := &AutomationRule{
		ID:   "rule_1",
		Name: "测试规则",
	}

	// 添加规则
	admin.AddRule(rule)

	// 尝试添加重复规则
	err := admin.AddRule(rule)
	if err == nil {
		t.Error("expected error when adding duplicate rule")
	}
}

func TestRemoveRule(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)

	rule := &AutomationRule{
		ID:   "rule_1",
		Name: "测试规则",
	}

	// 添加规则
	admin.AddRule(rule)

	// 移除规则
	err := admin.RemoveRule("rule_1")
	if err != nil {
		t.Fatalf("RemoveRule failed: %v", err)
	}

	// 验证规则已移除
	rules := admin.GetRules()
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}

	// 尝试移除不存在的规则
	err = admin.RemoveRule("nonexistent")
	if err == nil {
		t.Error("expected error when removing nonexistent rule")
	}
}

func TestEvaluateRules(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 添加规则
	rule := &AutomationRule{
		ID:       "rule_1",
		Name:     "CPU高负载告警",
		Condition: "cpu > 90",
		Action:   "notify",
		Enabled:  true,
	}
	admin.AddRule(rule)

	// 评估规则
	logs, err := admin.EvaluateRules(ctx)
	if err != nil {
		t.Fatalf("EvaluateRules failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	if logs[0].Action != "notify" {
		t.Errorf("expected action notify, got %s", logs[0].Action)
	}
}

func TestEvaluateRulesDisabled(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    false,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	_, err := admin.EvaluateRules(ctx)
	if err == nil {
		t.Error("expected error when rules are disabled")
	}
}

func TestGetCommands(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 执行命令
	admin.ExecuteCommand(ctx, "重启服务")
	admin.ExecuteCommand(ctx, "清理缓存")
	admin.ExecuteCommand(ctx, "备份配置")

	// 获取所有命令
	commands := admin.GetCommands("", 0)
	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}

	// 获取已完成的命令
	commands = admin.GetCommands(CommandStatusCompleted, 0)
	if len(commands) != 3 {
		t.Fatalf("expected 3 completed commands, got %d", len(commands))
	}

	// 获取限制数量的命令
	commands = admin.GetCommands("", 2)
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(commands))
	}
}

func TestGetDiagnoses(t *testing.T) {
	config := &Config{
		MaxHistory:     1000,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 执行诊断
	admin.DiagnoseSystem(ctx)

	// 获取所有诊断
	diagnoses := admin.GetDiagnoses("", 0)
	if len(diagnoses) != 5 {
		t.Fatalf("expected 5 diagnoses, got %d", len(diagnoses))
	}

	// 获取特定类别的诊断
	diagnoses = admin.GetDiagnoses("cpu", 0)
	if len(diagnoses) != 1 {
		t.Fatalf("expected 1 cpu diagnosis, got %d", len(diagnoses))
	}

	// 获取限制数量的诊断
	diagnoses = admin.GetDiagnoses("", 2)
	if len(diagnoses) != 2 {
		t.Fatalf("expected 2 diagnoses, got %d", len(diagnoses))
	}
}

func TestMaxHistoryLimit(t *testing.T) {
	config := &Config{
		MaxHistory:     3,
		AutoRepair:     true,
		DiagInterval:   5 * time.Minute,
		CommandTimeout: 30 * time.Second,
		EnableRules:    true,
	}

	admin := New(config)
	admin.Start()
	defer admin.Stop()

	ctx := context.Background()

	// 执行超过限制的命令
	for i := 0; i < 5; i++ {
		admin.ExecuteCommand(ctx, "重启服务")
	}

	// 验证历史记录限制
	commands := admin.GetCommands("", 0)
	if len(commands) != 3 {
		t.Fatalf("expected 3 commands (max history), got %d", len(commands))
	}
}
