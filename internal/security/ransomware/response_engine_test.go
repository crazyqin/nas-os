package ransomware

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ========== Mock 实现 ==========

// mockThreatDetector 模拟威胁检测器.
type mockThreatDetector struct {
	threatLevel ThreatLevel
	threats     []*DetectionResult
}

func (m *mockThreatDetector) EvaluateThreat(result *DetectionResult) ThreatLevel {
	if m.threatLevel != "" {
		return m.threatLevel
	}
	return result.ThreatLevel
}

func (m *mockThreatDetector) GetActiveThreats() []*DetectionResult {
	return m.threats
}

// mockSnapshotCreator 模拟快照创建器.
type mockSnapshotCreator struct {
	snapshots []string
	mu        sync.Mutex
	fail      bool
}

func (m *mockSnapshotCreator) CreateSnapshot(name string, subvolume string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fail {
		return "", fmt.Errorf("快照创建失败")
	}
	path := subvolume + "/.snapshots/" + name
	m.snapshots = append(m.snapshots, name)
	return path, nil
}

func (m *mockSnapshotCreator) ListSnapshots(subvolume string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshots, nil
}

// mockNetworkIsolator 模拟网络隔离器.
type mockNetworkIsolator struct {
	isolated []string
	mu       sync.Mutex
	fail     bool
}

func (m *mockNetworkIsolator) IsolateCIDR(cidr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fail {
		return fmt.Errorf("隔离失败")
	}
	m.isolated = append(m.isolated, cidr)
	return nil
}

func (m *mockNetworkIsolator) RestoreCIDR(cidr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, c := range m.isolated {
		if c == cidr {
			m.isolated = append(m.isolated[:i], m.isolated[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockNetworkIsolator) GetIsolatedCIDRs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.isolated))
	copy(result, m.isolated)
	return result
}

// mockProcessKiller 模拟进程终止器.
type mockProcessKiller struct {
	killedPIDs  []int
	killedNames []string
	mu          sync.Mutex
	fail        bool
}

func (m *mockProcessKiller) KillProcess(pid int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fail {
		return fmt.Errorf("终止进程失败")
	}
	m.killedPIDs = append(m.killedPIDs, pid)
	return nil
}

func (m *mockProcessKiller) KillProcessByName(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fail {
		return fmt.Errorf("按名称终止进程失败")
	}
	m.killedNames = append(m.killedNames, name)
	return nil
}

func (m *mockProcessKiller) IsProcessRunning(pid int) bool {
	return false
}

// ========== 辅助函数 ==========

// newTestDetectionResult 创建测试用检测结果.
func newTestDetectionResult(level ThreatLevel, confidence float64, processPID int) *DetectionResult {
	result := &DetectionResult{
		ID:              "det-test-001",
		Timestamp:       time.Now(),
		ThreatLevel:     level,
		DetectionType:   DetectionTypeBehavior,
		FilePath:        "/data/documents/report.docx",
		Confidence:      confidence,
		SuggestedAction: "隔离并终止可疑进程",
	}

	if processPID > 0 {
		result.ProcessInfo = &ProcessInfo{
			PID:     processPID,
			Name:    "suspicious_process",
			Path:    "/tmp/suspicious",
			CmdLine: "/tmp/suspicious --encrypt",
			User:    "nobody",
		}
	}

	return result
}

func newTestResponseEngine(detector ThreatDetector, snapshot SnapshotCreator, isolator NetworkIsolator, killer ProcessKiller) *ResponseEngine {
	config := DefaultResponseConfig()
	return NewResponseEngine(config, detector, snapshot, isolator, killer, nil)
}

// ========== 测试用例 ==========

// TestNewResponseEngine 测试创建响应引擎.
func TestNewResponseEngine(t *testing.T) {
	detector := &mockThreatDetector{}
	snapshot := &mockSnapshotCreator{}
	isolator := &mockNetworkIsolator{}
	killer := &mockProcessKiller{}

	engine := newTestResponseEngine(detector, snapshot, isolator, killer)

	if engine == nil {
		t.Fatal("响应引擎创建失败")
	}

	if !engine.config.Enabled {
		t.Error("响应引擎应默认启用")
	}

	if engine.config.MaxLevel != ResponseLevelNetworkIsolation {
		t.Errorf("最大响应级别应为 %d，实际为 %d", ResponseLevelNetworkIsolation, engine.config.MaxLevel)
	}
}

// TestEvaluateResponseLevel 测试响应级别评估.
func TestEvaluateResponseLevel(t *testing.T) {
	engine := newTestResponseEngine(&mockThreatDetector{}, nil, nil, nil)

	tests := []struct {
		name      string
		result    *DetectionResult
		wantLevel ResponseLevel
	}{
		{
			name:      "nil结果应返回告警级别",
			result:    nil,
			wantLevel: ResponseLevelAlert,
		},
		{
			name:      "低置信度应返回告警级别",
			result:    newTestDetectionResult(ThreatLevelCritical, 0.3, 1234),
			wantLevel: ResponseLevelAlert,
		},
		{
			name:      "关键威胁高置信度有进程信息应返回网络阻断",
			result:    newTestDetectionResult(ThreatLevelCritical, 0.95, 1234),
			wantLevel: ResponseLevelNetworkIsolation,
		},
		{
			name:      "关键威胁高置信度无进程信息应返回快照保护",
			result:    newTestDetectionResult(ThreatLevelCritical, 0.9, 0),
			wantLevel: ResponseLevelSnapshot,
		},
		{
			name:      "高威胁高置信度有进程信息应返回快照保护",
			result:    newTestDetectionResult(ThreatLevelHigh, 0.85, 1234),
			wantLevel: ResponseLevelSnapshot,
		},
		{
			name:      "高威胁低置信度有进程信息应返回进程隔离",
			result:    newTestDetectionResult(ThreatLevelHigh, 0.7, 1234),
			wantLevel: ResponseLevelProcessIsolation,
		},
		{
			name:      "中等威胁有进程信息应返回进程隔离",
			result:    newTestDetectionResult(ThreatLevelMedium, 0.7, 1234),
			wantLevel: ResponseLevelProcessIsolation,
		},
		{
			name:      "中等威胁无进程信息应返回告警",
			result:    newTestDetectionResult(ThreatLevelMedium, 0.7, 0),
			wantLevel: ResponseLevelAlert,
		},
		{
			name:      "低威胁应返回告警",
			result:    newTestDetectionResult(ThreatLevelLow, 0.8, 0),
			wantLevel: ResponseLevelAlert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := engine.evaluateResponseLevel(tt.result)
			if level != tt.wantLevel {
				t.Errorf("期望响应级别 %d (%s)，实际 %d (%s)",
					tt.wantLevel, tt.wantLevel.String(), level, level.String())
			}
		})
	}
}

// TestHandleThreat_AlertOnly 测试仅告警的响应.
func TestHandleThreat_AlertOnly(t *testing.T) {
	detector := &mockThreatDetector{}
	killer := &mockProcessKiller{}
	engine := newTestResponseEngine(detector, nil, nil, killer)

	// 低威胁结果，仅触发告警
	result := newTestDetectionResult(ThreatLevelLow, 0.5, 0)

	actions, err := engine.HandleThreat(result)
	if err != nil {
		t.Fatalf("处理威胁失败: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("期望 1 个动作，实际 %d 个", len(actions))
	}

	if actions[0].Level != ResponseLevelAlert {
		t.Errorf("期望告警级别，实际 %d", actions[0].Level)
	}
}

// TestHandleThreat_ProcessIsolation 测试进程隔离响应.
func TestHandleThreat_ProcessIsolation(t *testing.T) {
	detector := &mockThreatDetector{}
	killer := &mockProcessKiller{}
	engine := newTestResponseEngine(detector, nil, nil, killer)

	result := newTestDetectionResult(ThreatLevelMedium, 0.7, 5678)

	actions, err := engine.HandleThreat(result)
	if err != nil {
		t.Fatalf("处理威胁失败: %v", err)
	}

	if len(actions) != 2 {
		t.Fatalf("期望 2 个动作（告警+进程隔离），实际 %d 个", len(actions))
	}

	// 验证进程被终止
	killer.mu.Lock()
	if len(killer.killedPIDs) != 1 || killer.killedPIDs[0] != 5678 {
		t.Errorf("期望终止 PID 5678，实际: %v", killer.killedPIDs)
	}
	killer.mu.Unlock()
}

// TestHandleThreat_FullResponse 测试完整响应链（告警+进程隔离+快照+网络阻断）.
func TestHandleThreat_FullResponse(t *testing.T) {
	detector := &mockThreatDetector{}
	snapshot := &mockSnapshotCreator{}
	isolator := &mockNetworkIsolator{}
	killer := &mockProcessKiller{}
	engine := newTestResponseEngine(detector, snapshot, isolator, killer)

	// 关键威胁，高置信度，有进程信息，应触发全部四级响应
	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 9999)

	actions, err := engine.HandleThreat(result)
	if err != nil {
		t.Fatalf("处理威胁失败: %v", err)
	}

	if len(actions) != 4 {
		t.Fatalf("期望 4 个动作，实际 %d 个", len(actions))
	}

	// 验证各级别
	expectedLevels := []ResponseLevel{
		ResponseLevelAlert,
		ResponseLevelProcessIsolation,
		ResponseLevelSnapshot,
		ResponseLevelNetworkIsolation,
	}
	for i, expected := range expectedLevels {
		if actions[i].Level != expected {
			t.Errorf("动作[%d] 期望级别 %d，实际 %d", i, expected, actions[i].Level)
		}
		if !actions[i].Success {
			t.Errorf("动作[%d] 应成功，实际失败: %s", i, actions[i].Error)
		}
	}

	// 验证进程被终止
	killer.mu.Lock()
	if len(killer.killedPIDs) != 1 || killer.killedPIDs[0] != 9999 {
		t.Errorf("期望终止 PID 9999，实际: %v", killer.killedPIDs)
	}
	killer.mu.Unlock()

	// 验证快照被创建
	snapshot.mu.Lock()
	if len(snapshot.snapshots) != 1 {
		t.Errorf("期望创建 1 个快照，实际 %d 个", len(snapshot.snapshots))
	}
	snapshot.mu.Unlock()

	// 验证网络被隔离
	isolator.mu.Lock()
	if len(isolator.isolated) != len(DefaultResponseConfig().NetworkIsolationCIDRs) {
		t.Errorf("期望隔离 %d 个网段，实际 %d 个",
			len(DefaultResponseConfig().NetworkIsolationCIDRs), len(isolator.isolated))
	}
	isolator.mu.Unlock()
}

// TestHandleThreat_Disabled 测试禁用状态.
func TestHandleThreat_Disabled(t *testing.T) {
	config := DefaultResponseConfig()
	config.Enabled = false
	engine := NewResponseEngine(config, &mockThreatDetector{}, nil, nil, nil, nil)

	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 1234)
	actions, err := engine.HandleThreat(result)

	if err != nil {
		t.Fatalf("禁用状态下不应报错: %v", err)
	}
	if actions != nil {
		t.Error("禁用状态下不应返回动作")
	}
}

// TestHandleThreat_MaxLevel 测试最大级别限制.
func TestHandleThreat_MaxLevel(t *testing.T) {
	config := DefaultResponseConfig()
	config.MaxLevel = ResponseLevelProcessIsolation // 限制最大级别为进程隔离

	snapshot := &mockSnapshotCreator{}
	isolator := &mockNetworkIsolator{}
	killer := &mockProcessKiller{}

	engine := NewResponseEngine(config, &mockThreatDetector{}, snapshot, isolator, killer, nil)

	// 即使是关键威胁，也不应超过进程隔离级别
	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 1234)
	actions, _ := engine.HandleThreat(result)

	for _, a := range actions {
		if a.Level > ResponseLevelProcessIsolation {
			t.Errorf("不应执行超过进程隔离级别的动作，实际级别: %d", a.Level)
		}
	}

	// 快照和网络不应被触发
	snapshot.mu.Lock()
	if len(snapshot.snapshots) != 0 {
		t.Error("不应创建快照")
	}
	snapshot.mu.Unlock()

	isolator.mu.Lock()
	if len(isolator.isolated) != 0 {
		t.Error("不应隔离网络")
	}
	isolator.mu.Unlock()
}

// TestEscalateResponse 测试响应升级.
func TestEscalateResponse(t *testing.T) {
	snapshot := &mockSnapshotCreator{}
	killer := &mockProcessKiller{}
	engine := newTestResponseEngine(&mockThreatDetector{}, snapshot, nil, killer)

	result := newTestDetectionResult(ThreatLevelHigh, 0.85, 1234)

	// 从告警级别升级
	action := engine.escalateResponse(result, ResponseLevelAlert)
	if action.Level != ResponseLevelProcessIsolation {
		t.Errorf("期望升级到进程隔离级别，实际 %d", action.Level)
	}
	if !action.Success {
		t.Errorf("升级应成功: %s", action.Error)
	}
}

// TestEscalateResponse_MaxLevel 测试到最大级别后无法继续升级.
func TestEscalateResponse_MaxLevel(t *testing.T) {
	config := DefaultResponseConfig()
	config.MaxLevel = ResponseLevelSnapshot
	engine := NewResponseEngine(config, &mockThreatDetector{}, nil, nil, nil, nil)

	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 1234)

	// 从快照级别（最大级别）升级应失败
	action := engine.escalateResponse(result, ResponseLevelSnapshot)
	if action.Success {
		t.Error("超过最大级别后不应成功")
	}
}

// TestExecuteResponse_ProcessKillFailure 测试进程终止失败的回退.
func TestExecuteResponse_ProcessKillFailure(t *testing.T) {
	killer := &mockProcessKiller{fail: true}
	engine := newTestResponseEngine(&mockThreatDetector{}, nil, nil, killer)

	result := newTestDetectionResult(ThreatLevelMedium, 0.7, 1234)
	action := engine.executeResponse(ResponseLevelProcessIsolation, result)

	if action.Success {
		t.Error("进程终止器失败时，动作应标记为失败")
	}
	if action.Error == "" {
		t.Error("失败时应有错误信息")
	}
}

// TestExecuteResponse_SnapshotFailure 测试快照创建失败.
func TestExecuteResponse_SnapshotFailure(t *testing.T) {
	snapshot := &mockSnapshotCreator{fail: true}
	engine := newTestResponseEngine(&mockThreatDetector{}, snapshot, nil, nil)

	result := newTestDetectionResult(ThreatLevelHigh, 0.85, 0)
	action := engine.executeResponse(ResponseLevelSnapshot, result)

	if action.Success {
		t.Error("快照创建失败时，动作应标记为失败")
	}
}

// TestExecuteResponse_NetworkIsolationFailure 测试网络隔离失败.
func TestExecuteResponse_NetworkIsolationFailure(t *testing.T) {
	isolator := &mockNetworkIsolator{fail: true}
	engine := newTestResponseEngine(&mockThreatDetector{}, nil, isolator, nil)

	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 0)
	action := engine.executeResponse(ResponseLevelNetworkIsolation, result)

	if action.Success {
		t.Error("网络隔离失败时，动作应标记为失败")
	}
}

// TestExecuteResponse_NilProcessInfo 测试无进程信息时的进程隔离.
func TestExecuteResponse_NilProcessInfo(t *testing.T) {
	engine := newTestResponseEngine(&mockThreatDetector{}, nil, nil, &mockProcessKiller{})

	result := newTestDetectionResult(ThreatLevelMedium, 0.7, 0) // 无进程信息
	action := engine.executeResponse(ResponseLevelProcessIsolation, result)

	if action.Success {
		t.Error("无进程信息时，进程隔离应失败")
	}
}

// TestGetActions 测试获取动作列表.
func TestGetActions(t *testing.T) {
	detector := &mockThreatDetector{}
	snapshot := &mockSnapshotCreator{}
	isolator := &mockNetworkIsolator{}
	killer := &mockProcessKiller{}
	engine := newTestResponseEngine(detector, snapshot, isolator, killer)

	// 触发一次完整响应
	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 1234)
	_, _ = engine.HandleThreat(result)

	actions := engine.GetActions(10)
	if len(actions) != 4 {
		t.Errorf("期望 4 个动作，实际 %d", len(actions))
	}

	// 测试限制数量
	actions = engine.GetActions(2)
	if len(actions) != 2 {
		t.Errorf("限制 2 个时应返回 2 个，实际 %d", len(actions))
	}
}

// TestGetStats 测试获取统计信息.
func TestGetStats(t *testing.T) {
	detector := &mockThreatDetector{}
	killer := &mockProcessKiller{}
	engine := newTestResponseEngine(detector, nil, nil, killer)

	// 触发两次响应
	result1 := newTestDetectionResult(ThreatLevelLow, 0.5, 0)
	_, _ = engine.HandleThreat(result1)

	result2 := newTestDetectionResult(ThreatLevelMedium, 0.7, 1234)
	_, _ = engine.HandleThreat(result2)

	stats := engine.GetStats()

	if stats.TotalResponses != 3 { // 1 alert + 1 alert + 1 process isolation
		t.Errorf("期望总响应数 3，实际 %d", stats.TotalResponses)
	}

	if stats.SuccessCount == 0 {
		t.Error("应有成功的响应")
	}

	if stats.ByLevel[ResponseLevelAlert] != 2 {
		t.Errorf("期望告警次数 2，实际 %d", stats.ByLevel[ResponseLevelAlert])
	}
}

// TestRestoreNetwork 测试恢复网络.
func TestRestoreNetwork(t *testing.T) {
	isolator := &mockNetworkIsolator{}
	engine := newTestResponseEngine(&mockThreatDetector{}, nil, isolator, nil)

	// 先隔离
	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 0)
	_, _ = engine.HandleThreat(result)

	// 验证已隔离
	isolator.mu.Lock()
	isolated := len(isolator.isolated)
	isolator.mu.Unlock()
	if isolated == 0 {
		t.Skip("未隔离任何网段，跳过恢复测试")
	}

	// 恢复
	if err := engine.RestoreNetwork(); err != nil {
		t.Fatalf("恢复网络失败: %v", err)
	}

	isolator.mu.Lock()
	if len(isolator.isolated) != 0 {
		t.Errorf("恢复后不应有隔离网段，实际 %d 个", len(isolator.isolated))
	}
	isolator.mu.Unlock()
}

// TestResponseLevelString 测试响应级别字符串.
func TestResponseLevelString(t *testing.T) {
	tests := []struct {
		level ResponseLevel
		want  string
	}{
		{ResponseLevelAlert, "告警通知"},
		{ResponseLevelProcessIsolation, "进程隔离"},
		{ResponseLevelSnapshot, "快照保护"},
		{ResponseLevelNetworkIsolation, "网络阻断"},
		{ResponseLevel(99), "未知级别"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("ResponseLevel(%d).String() = %s, 期望 %s", tt.level, got, tt.want)
		}
	}
}

// TestDefaultResponseConfig 测试默认配置.
func TestDefaultResponseConfig(t *testing.T) {
	config := DefaultResponseConfig()

	if !config.Enabled {
		t.Error("默认配置应启用")
	}
	if config.MaxLevel != ResponseLevelNetworkIsolation {
		t.Errorf("默认最大级别应为 %d，实际 %d", ResponseLevelNetworkIsolation, config.MaxLevel)
	}
	if config.ConfidenceThreshold != 0.6 {
		t.Errorf("默认置信度阈值应为 0.6，实际 %f", config.ConfidenceThreshold)
	}
	if config.SnapshotPrefix != "ransomware-protection" {
		t.Errorf("默认快照前缀应为 ransomware-protection，实际 %s", config.SnapshotPrefix)
	}
	if len(config.NetworkIsolationCIDRs) == 0 {
		t.Error("默认配置应有网络隔离白名单")
	}
}

// TestExtractSubvolume 测试子卷路径提取.
func TestExtractSubvolume(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/data/documents/file.txt", "/data"},
		{"/mnt/storage/photos/img.jpg", "/mnt/storage"},
		{"/home/user/file.txt", "/home"},
		{"/other/path/file.txt", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractSubvolume(tt.path)
		if got != tt.want {
			t.Errorf("extractSubvolume(%q) = %q, 期望 %q", tt.path, got, tt.want)
		}
	}
}

// TestHandleThreat_NilDependencies 测试依赖为nil时的容错.
func TestHandleThreat_NilDependencies(t *testing.T) {
	config := DefaultResponseConfig()
	// 所有依赖都为nil
	engine := NewResponseEngine(config, &mockThreatDetector{}, nil, nil, nil, nil)

	result := newTestDetectionResult(ThreatLevelCritical, 0.95, 1234)
	actions, err := engine.HandleThreat(result)

	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}

	// 应返回4个动作（其中部分失败）
	if len(actions) != 4 {
		t.Errorf("期望 4 个动作，实际 %d", len(actions))
	}

	// 告警应成功（不需要依赖）
	if !actions[0].Success {
		t.Error("告警动作应成功")
	}

	// 其他应失败（依赖为nil）
	for i := 1; i < len(actions); i++ {
		if actions[i].Success {
			t.Errorf("动作[%d] 依赖为nil时不应成功", i)
		}
	}
}
