package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestHealthChecker(t *testing.T) (*Manager, *HealthChecker) {
	t.Helper()

	config := SimpleClusterConfig{
		Name:              "test-cluster",
		NodeID:            "local-node-001",
		DiscoveryPort:     8081,
		HeartbeatInterval: 5,
		HeartbeatTimeout:  15,
		DataDir:           t.TempDir(),
	}

	logger := zap.NewNop()
	manager, err := NewManager(config, logger)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	hc := NewHealthChecker(manager, nil)
	return manager, hc
}

func TestNewHealthChecker(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	if hc == nil {
		t.Fatal("expected non-nil health checker")
	}
	if hc.running {
		t.Error("expected health checker to not be running initially")
	}
}

func TestDefaultHealthCheckConfig(t *testing.T) {
	cfg := DefaultHealthCheckConfig()

	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("expected check interval 30s, got %v", cfg.CheckInterval)
	}
	if cfg.HeartbeatTimeout != 60*time.Second {
		t.Errorf("expected heartbeat timeout 60s, got %v", cfg.HeartbeatTimeout)
	}
	if cfg.CPUThreshold != 80.0 {
		t.Errorf("expected CPU threshold 80.0, got %f", cfg.CPUThreshold)
	}
	if cfg.MemoryThreshold != 85.0 {
		t.Errorf("expected memory threshold 85.0, got %f", cfg.MemoryThreshold)
	}
	if cfg.DiskThreshold != 90.0 {
		t.Errorf("expected disk threshold 90.0, got %f", cfg.DiskThreshold)
	}
	if cfg.NetworkLatencyThreshold != 1000.0 {
		t.Errorf("expected network latency threshold 1000.0, got %f", cfg.NetworkLatencyThreshold)
	}
	if cfg.MaxHistoryCount != 1000 {
		t.Errorf("expected max history count 1000, got %d", cfg.MaxHistoryCount)
	}
	if !cfg.AutoRecoveryCheck {
		t.Error("expected auto recovery check to be enabled")
	}
	if cfg.RecoveryCheckInterval != 5*time.Minute {
		t.Errorf("expected recovery check interval 5m, got %v", cfg.RecoveryCheckInterval)
	}
}

func TestHealthCheckerStartStop(t *testing.T) {
	_, hc := setupTestHealthChecker(t)
	ctx := context.Background()

	// 启动
	err := hc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start health checker: %v", err)
	}
	if !hc.IsRunning() {
		t.Error("expected health checker to be running")
	}

	// 重复启动
	err = hc.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running health checker")
	}

	// 停止
	hc.Stop()
	if hc.IsRunning() {
		t.Error("expected health checker to not be running after stop")
	}
}

func TestDefaultChecks(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 启动健康检查器以初始化默认检查
	ctx := context.Background()
	hc.Start(ctx)
	defer hc.Stop()

	checks := hc.ListChecks()
	if len(checks) < 5 {
		t.Errorf("expected at least 5 default checks, got %d", len(checks))
	}

	// 验证默认检查类型
	checkTypes := make(map[CheckType]bool)
	for _, check := range checks {
		checkTypes[check.Type] = true
	}

	expectedTypes := []CheckType{CheckTypeHeartbeat, CheckTypeCPU, CheckTypeMemory, CheckTypeDisk, CheckTypeNetwork}
	for _, expectedType := range expectedTypes {
		if !checkTypes[expectedType] {
			t.Errorf("expected check type %s not found", expectedType)
		}
	}
}

func TestDefaultAlertRules(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 启动健康检查器以初始化默认规则
	ctx := context.Background()
	hc.Start(ctx)
	defer hc.Stop()

	rules := hc.ListAlertRules()
	if len(rules) < 4 {
		t.Errorf("expected at least 4 default alert rules, got %d", len(rules))
	}

	// 验证默认规则
	ruleIDs := make(map[string]bool)
	for _, rule := range rules {
		ruleIDs[rule.ID] = true
	}

	expectedRules := []string{"rule-cpu-high", "rule-memory-high", "rule-disk-high", "rule-heartbeat-timeout"}
	for _, expectedRule := range expectedRules {
		if !ruleIDs[expectedRule] {
			t.Errorf("expected rule %s not found", expectedRule)
		}
	}
}

func TestAddCheck(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 添加自定义检查
	check := &HealthCheck{
		ID:        "custom-check-001",
		Name:      "自定义检查",
		Type:      CheckTypeCustom,
		Enabled:   true,
		Threshold: 50.0,
	}

	err := hc.AddCheck(check)
	if err != nil {
		t.Fatalf("failed to add check: %v", err)
	}

	// 验证检查已添加
	addedCheck, err := hc.GetCheck("custom-check-001")
	if err != nil {
		t.Fatalf("failed to get check: %v", err)
	}
	if addedCheck.Name != "自定义检查" {
		t.Errorf("expected check name '自定义检查', got '%s'", addedCheck.Name)
	}

	// 添加重复检查
	err = hc.AddCheck(check)
	if err == nil {
		t.Error("expected error when adding duplicate check")
	}

	// 添加无 ID 检查
	err = hc.AddCheck(&HealthCheck{})
	if err == nil {
		t.Error("expected error when adding check without ID")
	}
}

func TestRemoveCheck(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 启动健康检查器以初始化默认检查
	ctx := context.Background()
	hc.Start(ctx)
	defer hc.Stop()

	// 移除默认检查
	err := hc.RemoveCheck("check-cpu")
	if err != nil {
		t.Fatalf("failed to remove check: %v", err)
	}

	// 验证检查已移除
	_, err = hc.GetCheck("check-cpu")
	if err == nil {
		t.Error("expected error when getting removed check")
	}

	// 移除不存在的检查
	err = hc.RemoveCheck("non-existent")
	if err == nil {
		t.Error("expected error when removing non-existent check")
	}
}

func TestAddAlertRule(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 添加自定义规则
	rule := &AlertRule{
		ID:        "custom-rule-001",
		Name:      "自定义规则",
		CheckType: CheckTypeCPU,
		Condition: "gt",
		Threshold: 90.0,
		Severity:  AlertSeverityError,
		Duration:  10 * time.Minute,
		Enabled:   true,
	}

	err := hc.AddAlertRule(rule)
	if err != nil {
		t.Fatalf("failed to add alert rule: %v", err)
	}

	// 验证规则已添加
	rules := hc.ListAlertRules()
	found := false
	for _, r := range rules {
		if r.ID == "custom-rule-001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom rule to be in list")
	}

	// 添加重复规则
	err = hc.AddAlertRule(rule)
	if err == nil {
		t.Error("expected error when adding duplicate rule")
	}
}

func TestRemoveAlertRule(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 启动健康检查器以初始化默认规则
	ctx := context.Background()
	hc.Start(ctx)
	defer hc.Stop()

	// 移除默认规则
	err := hc.RemoveAlertRule("rule-cpu-high")
	if err != nil {
		t.Fatalf("failed to remove alert rule: %v", err)
	}

	// 验证规则已移除
	rules := hc.ListAlertRules()
	for _, rule := range rules {
		if rule.ID == "rule-cpu-high" {
			t.Error("expected rule to be removed")
		}
	}

	// 移除不存在的规则
	err = hc.RemoveAlertRule("non-existent")
	if err == nil {
		t.Error("expected error when removing non-existent rule")
	}
}

func TestClusterHealthStatus(t *testing.T) {
	manager, hc := setupTestHealthChecker(t)

	// 清除自动添加的本地节点，使用测试专用节点
	manager.nodesMutex.Lock()
	manager.nodes = make(map[string]*Member)
	manager.nodes["node-001"] = &Member{
		ID:     "node-001",
		Status: StatusOnline,
	}
	manager.nodes["node-002"] = &Member{
		ID:     "node-002",
		Status: StatusDegraded,
	}
	manager.nodes["node-003"] = &Member{
		ID:     "node-003",
		Status: StatusOffline,
	}
	manager.nodesMutex.Unlock()

	// 获取健康状态
	status := hc.GetClusterHealth()
	if status.TotalNodes != 3 {
		t.Errorf("expected total nodes 3, got %d", status.TotalNodes)
	}
	if status.HealthyNodes != 1 {
		t.Errorf("expected healthy nodes 1, got %d", status.HealthyNodes)
	}
	if status.DegradedNodes != 1 {
		t.Errorf("expected degraded nodes 1, got %d", status.DegradedNodes)
	}
	if status.OfflineNodes != 1 {
		t.Errorf("expected offline nodes 1, got %d", status.OfflineNodes)
	}

	// 整体状态应该是不健康的（有离线节点）
	if status.OverallLevel != HealthLevelUnhealthy {
		t.Errorf("expected overall level 'unhealthy', got '%s'", status.OverallLevel)
	}
}

func TestNodeHealth(t *testing.T) {
	manager, hc := setupTestHealthChecker(t)

	// 添加节点
	manager.nodesMutex.Lock()
	manager.nodes["node-001"] = &Member{
		ID:     "node-001",
		Status: StatusOnline,
	}
	manager.nodesMutex.Unlock()

	// 获取节点健康状态
	level, err := hc.GetNodeHealth("node-001")
	if err != nil {
		t.Fatalf("failed to get node health: %v", err)
	}
	if level != HealthLevelHealthy {
		t.Errorf("expected health level 'healthy', got '%s'", level)
	}

	// 获取不存在的节点
	_, err = hc.GetNodeHealth("non-existent")
	if err == nil {
		t.Error("expected error when getting health of non-existent node")
	}
}

func TestAlertManagement(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 添加告警
	hc.mu.Lock()
	hc.alerts["alert-001"] = &Alert{
		ID:        "alert-001",
		Severity:  AlertSeverityWarning,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	hc.alerts["alert-002"] = &Alert{
		ID:        "alert-002",
		Severity:  AlertSeverityError,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	hc.mu.Unlock()

	// 获取活跃告警
	activeAlerts := hc.GetAlerts("active", "", 0)
	if len(activeAlerts) != 2 {
		t.Errorf("expected 2 active alerts, got %d", len(activeAlerts))
	}

	// 确认告警
	err := hc.AcknowledgeAlert("alert-001")
	if err != nil {
		t.Fatalf("failed to acknowledge alert: %v", err)
	}

	// 获取已确认告警
	ackAlerts := hc.GetAlerts("acknowledged", "", 0)
	if len(ackAlerts) != 1 {
		t.Errorf("expected 1 acknowledged alert, got %d", len(ackAlerts))
	}

	// 解决告警
	err = hc.ResolveAlert("alert-002")
	if err != nil {
		t.Fatalf("failed to resolve alert: %v", err)
	}

	// 获取已解决告警
	resolvedAlerts := hc.GetAlerts("resolved", "", 0)
	if len(resolvedAlerts) != 1 {
		t.Errorf("expected 1 resolved alert, got %d", len(resolvedAlerts))
	}

	// 确认不存在的告警
	err = hc.AcknowledgeAlert("non-existent")
	if err == nil {
		t.Error("expected error when acknowledging non-existent alert")
	}

	// 解决已解决的告警
	err = hc.ResolveAlert("alert-002")
	if err == nil {
		t.Error("expected error when resolving already resolved alert")
	}
}

func TestAlertSeverityFilter(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 添加不同严重级别的告警
	hc.mu.Lock()
	hc.alerts["alert-001"] = &Alert{
		ID:        "alert-001",
		Severity:  AlertSeverityWarning,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	hc.alerts["alert-002"] = &Alert{
		ID:        "alert-002",
		Severity:  AlertSeverityError,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	hc.alerts["alert-003"] = &Alert{
		ID:        "alert-003",
		Severity:  AlertSeverityCritical,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	hc.mu.Unlock()

	// 按严重级别过滤
	warningAlerts := hc.GetAlerts("", AlertSeverityWarning, 0)
	if len(warningAlerts) != 1 {
		t.Errorf("expected 1 warning alert, got %d", len(warningAlerts))
	}

	errorAlerts := hc.GetAlerts("", AlertSeverityError, 0)
	if len(errorAlerts) != 1 {
		t.Errorf("expected 1 error alert, got %d", len(errorAlerts))
	}

	criticalAlerts := hc.GetAlerts("", AlertSeverityCritical, 0)
	if len(criticalAlerts) != 1 {
		t.Errorf("expected 1 critical alert, got %d", len(criticalAlerts))
	}
}

func TestAlertLimit(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 添加多个告警
	hc.mu.Lock()
	for i := 0; i < 10; i++ {
		hc.alerts[fmt.Sprintf("alert-%03d", i)] = &Alert{
			ID:        fmt.Sprintf("alert-%03d", i),
			Severity:  AlertSeverityWarning,
			Status:    "active",
			CreatedAt: time.Now(),
		}
	}
	hc.mu.Unlock()

	// 获取限制数量的告警
	alerts := hc.GetAlerts("active", "", 5)
	if len(alerts) != 5 {
		t.Errorf("expected 5 alerts, got %d", len(alerts))
	}
}

func TestOnAlert(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 注册告警处理器
	alertReceived := false
	hc.OnAlert(func(alert *Alert) {
		alertReceived = true
	})

	// 模拟触发告警
	hc.mu.Lock()
	alert := &Alert{
		ID:        "test-alert",
		Severity:  AlertSeverityWarning,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	hc.alerts["test-alert"] = alert

	// 手动触发处理器
	for _, handler := range hc.alertHandlers {
		handler(alert)
	}
	hc.mu.Unlock()

	if !alertReceived {
		t.Error("expected alert handler to be called")
	}
}

func TestCheckHistory(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 添加历史记录
	hc.mu.Lock()
	hc.history = []*HealthCheckStatus{
		{
			Timestamp: time.Now().Add(-2 * time.Hour),
			Level:     HealthLevelHealthy,
			Checks:    map[string]HealthLevel{"check-cpu": HealthLevelHealthy},
		},
		{
			Timestamp: time.Now().Add(-1 * time.Hour),
			Level:     HealthLevelDegraded,
			Checks:    map[string]HealthLevel{"check-cpu": HealthLevelDegraded},
		},
		{
			Timestamp: time.Now(),
			Level:     HealthLevelHealthy,
			Checks:    map[string]HealthLevel{"check-cpu": HealthLevelHealthy},
		},
	}
	hc.mu.Unlock()

	// 获取历史
	history := hc.GetHistory(10)
	if len(history) != 3 {
		t.Errorf("expected 3 history records, got %d", len(history))
	}

	// 获取限制数量的历史
	history = hc.GetHistory(2)
	if len(history) != 2 {
		t.Errorf("expected 2 history records, got %d", len(history))
	}
}

func TestStats(t *testing.T) {
	_, hc := setupTestHealthChecker(t)

	// 启动健康检查器以初始化默认检查和规则
	ctx := context.Background()
	hc.Start(ctx)
	defer hc.Stop()

	stats := hc.GetStats()
	if stats["total_checks"] != 5 {
		t.Errorf("expected total_checks 5, got %v", stats["total_checks"])
	}
	if stats["total_rules"] != 4 {
		t.Errorf("expected total_rules 4, got %v", stats["total_rules"])
	}
}

func TestToJSON(t *testing.T) {
	manager, hc := setupTestHealthChecker(t)

	// 清除自动添加的本地节点，使用测试专用节点
	manager.nodesMutex.Lock()
	manager.nodes = make(map[string]*Member)
	manager.nodes["node-001"] = &Member{
		ID:     "node-001",
		Status: StatusOnline,
	}
	manager.nodesMutex.Unlock()

	// 导出 JSON
	jsonData, err := hc.ToJSON()
	if err != nil {
		t.Fatalf("failed to export to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("expected non-empty JSON data")
	}

	// 验证 JSON 可解析
	var status ClusterHealthStatus
	err = json.Unmarshal(jsonData, &status)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if status.TotalNodes != 1 {
		t.Errorf("expected total nodes 1, got %d", status.TotalNodes)
	}
}

func TestHealthLevels(t *testing.T) {
	levels := []HealthLevel{
		HealthLevelHealthy,
		HealthLevelDegraded,
		HealthLevelUnhealthy,
		HealthLevelCritical,
		HealthLevelUnknown,
	}

	for _, level := range levels {
		if level == "" {
			t.Error("expected non-empty health level")
		}
	}
}

func TestCheckTypes(t *testing.T) {
	types := []CheckType{
		CheckTypeHeartbeat,
		CheckTypeCPU,
		CheckTypeMemory,
		CheckTypeDisk,
		CheckTypeNetwork,
		CheckTypeService,
		CheckTypeCustom,
	}

	for _, checkType := range types {
		if checkType == "" {
			t.Error("expected non-empty check type")
		}
	}
}

func TestAlertSeverities(t *testing.T) {
	severities := []AlertSeverity{
		AlertSeverityInfo,
		AlertSeverityWarning,
		AlertSeverityError,
		AlertSeverityCritical,
	}

	for _, severity := range severities {
		if severity == "" {
			t.Error("expected non-empty alert severity")
		}
	}
}

func TestHealthCheckConfigCustom(t *testing.T) {
	cfg := &HealthCheckConfig{
		CheckInterval:           10 * time.Second,
		HeartbeatTimeout:        30 * time.Second,
		CPUThreshold:            70.0,
		MemoryThreshold:         75.0,
		DiskThreshold:           85.0,
		NetworkLatencyThreshold: 500.0,
		MaxHistoryCount:         500,
		AutoRecoveryCheck:       false,
		RecoveryCheckInterval:   2 * time.Minute,
	}

	manager, _ := setupTestHealthChecker(t)
	hc := NewHealthChecker(manager, cfg)

	if hc.config.CheckInterval != 10*time.Second {
		t.Errorf("expected check interval 10s, got %v", hc.config.CheckInterval)
	}
	if hc.config.CPUThreshold != 70.0 {
		t.Errorf("expected CPU threshold 70.0, got %f", hc.config.CPUThreshold)
	}
}
