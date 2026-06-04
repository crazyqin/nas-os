package ransomshield

import (
	"os"
	"testing"
	"time"
)

func TestNewResponder(t *testing.T) {
	r := NewResponder("/tmp/quarantine")
	if r == nil {
		t.Fatal("NewResponder returned nil")
	}

	if len(r.policies) == 0 {
		t.Error("expected default policies to be loaded")
	}
}

func TestResponder_HandleThreat(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	event := ThreatEvent{
		ID:          "test-threat-1",
		Level:       ThreatLevelCritical,
		SourcePath:  "/data/important.txt",
		ProcessName: "malware",
		ProcessID:   0, // no real process
		CreatedAt:   time.Now(),
	}

	actions := r.HandleThreat(event)
	if len(actions) == 0 {
		t.Error("expected at least one response action")
	}

	// 验证动作类型包含 snapshot 和 lockdown
	actionTypes := make(map[ActionType]bool)
	for _, a := range actions {
		actionTypes[a.Type] = true
	}

	if !actionTypes[ActionTypeSnapshot] {
		t.Error("expected snapshot action for critical threat")
	}
	if !actionTypes[ActionTypeLockdown] {
		t.Error("expected lockdown action for critical threat")
	}
}

func TestResponder_LowThreatResponse(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	event := ThreatEvent{
		ID:          "test-low",
		Level:       ThreatLevelLow,
		SourcePath:  "/data/suspicious.txt",
		ProcessName: "unknown",
		ProcessID:   0,
		CreatedAt:   time.Now(),
	}

	actions := r.HandleThreat(event)

	// Low threat should only trigger alert
	for _, a := range actions {
		if a.Type != ActionTypeAlert {
			t.Errorf("low threat should only trigger alert, got %s", a.Type)
		}
	}
}

func TestResponder_BlockIP(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	ip := "192.168.1.100"
	if err := r.BlockIP(ip, 10*time.Minute); err != nil {
		t.Fatalf("BlockIP failed: %v", err)
	}

	if !r.IsIPBlocked(ip) {
		t.Error("expected IP to be blocked")
	}

	r.UnblockIP(ip)
	if r.IsIPBlocked(ip) {
		t.Error("expected IP to be unblocked after UnblockIP")
	}
}

func TestResponder_QuarantineFile(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewResponder(tmpDir)
	r.Start()
	defer r.Stop()

	// 创建测试文件
	testFile := tmpDir + "/test-quarantine.txt"
	writeTestFile(testFile, "suspicious content")

	event := ThreatEvent{
		ID:         "test-quarantine",
		Level:      ThreatLevelCritical,
		SourcePath: testFile,
		CreatedAt:  time.Now(),
	}

	err := r.executeQuarantine(event)
	if err != nil {
		t.Fatalf("quarantine failed: %v", err)
	}

	// 原文件应该被移走
	if fileExists(testFile) {
		t.Error("original file should have been moved to quarantine")
	}
}

func TestResponder_GetActionLog(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	// 触发几个响应
	for i := 0; i < 5; i++ {
		event := ThreatEvent{
			ID:         "test-log",
			Level:      ThreatLevelMedium,
			SourcePath: "/data/test.txt",
			CreatedAt:  time.Now(),
		}
		r.HandleThreat(event)
	}

	log := r.GetActionLog(100)
	if len(log) == 0 {
		t.Error("expected action log entries")
	}
}

func TestResponder_GetStats(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	event := ThreatEvent{
		ID:         "test-stats",
		Level:      ThreatLevelHigh,
		SourcePath: "/data/test.txt",
		CreatedAt:  time.Now(),
	}

	r.HandleThreat(event)

	stats := r.GetStats()
	if stats.TotalActions == 0 {
		t.Error("expected TotalActions > 0")
	}
}

func TestResponder_PolicyManagement(t *testing.T) {
	r := NewResponder(t.TempDir())

	// 获取默认策略
	policies := r.GetPolicies()
	if len(policies) == 0 {
		t.Error("expected default policies")
	}

	// 添加自定义策略
	customPolicy := ResponsePolicy{
		ID:       "custom-1",
		Name:     "Custom Policy",
		Level:    ThreatLevelMedium,
		Actions:  []ActionType{ActionTypeAlert},
		Priority: 5,
		Enabled:  true,
	}
	r.AddPolicy(customPolicy)

	newPolicies := r.GetPolicies()
	found := false
	for _, p := range newPolicies[ThreatLevelMedium] {
		if p.ID == "custom-1" {
			found = true
		}
	}
	if !found {
		t.Error("custom policy not found after adding")
	}
}

func TestResponder_BlockedProcesses(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	// 阻断一个不存在的进程（不会真正kill，但会记录）
	event := ThreatEvent{
		ID:          "test-block-proc",
		Level:       ThreatLevelCritical,
		SourcePath:  "/data/test.txt",
		ProcessName: "suspicious",
		ProcessID:   99999, // 不太可能存在的PID
		CreatedAt:   time.Now(),
	}

	r.executeBlock(event)

	blocked := r.GetBlockedProcesses()
	// 可能为 0（如果进程不存在），但代码路径应该被覆盖
	t.Logf("blocked processes: %d", len(blocked))
}

func TestResponder_NetworkRules(t *testing.T) {
	r := NewResponder(t.TempDir())
	r.Start()
	defer r.Stop()

	r.BlockIP("10.0.0.1", 5*time.Minute)
	r.BlockIP("10.0.0.2", 0)

	rules := r.GetNetworkRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 network rules, got %d", len(rules))
	}
}

// 辅助函数
func writeTestFile(path, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		panic(err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
