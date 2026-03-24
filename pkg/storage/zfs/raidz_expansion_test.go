package zfs

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ========== RAIDZLevel 测试 ==========

func TestRAIDZLevelString(t *testing.T) {
	tests := []struct {
		level    RAIDZLevel
		expected string
	}{
		{RAIDZ1, "raidz1"},
		{RAIDZ2, "raidz2"},
		{RAIDZ3, "raidz3"},
		{RAIDZLevel(0), "unknown(0)"},
		{RAIDZLevel(99), "unknown(99)"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("RAIDZLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestParseRAIDZLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected RAIDZLevel
		hasError bool
	}{
		{"raidz", RAIDZ1, false},
		{"raidz1", RAIDZ1, false},
		{"RAIDZ1", RAIDZ1, false},
		{"RAIDZ", RAIDZ1, false},
		{"raidz2", RAIDZ2, false},
		{"RAIDZ2", RAIDZ2, false},
		{"raidz3", RAIDZ3, false},
		{"RAIDZ3", RAIDZ3, false},
		{"invalid", 0, true},
		{"", 0, true},
		{"mirror", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseRAIDZLevel(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ParseRAIDZLevel(%s) should return error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseRAIDZLevel(%s) unexpected error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("ParseRAIDZLevel(%s) = %d, want %d", tt.input, got, tt.expected)
			}
		}
	}
}

// ========== ExpansionConfig 测试 ==========

func TestExpansionConfigValidation(t *testing.T) {
	config := ExpansionConfig{
		PoolName:   "tank",
		NewDisk:    "/dev/sdb",
		RAIDZLevel: RAIDZ2,
		Force:      false,
		DryRun:     true,
	}

	if config.PoolName != "tank" {
		t.Error("PoolName should be tank")
	}
	if config.NewDisk != "/dev/sdb" {
		t.Error("NewDisk should be /dev/sdb")
	}
	if config.RAIDZLevel != RAIDZ2 {
		t.Error("RAIDZLevel should be RAIDZ2")
	}
	if !config.DryRun {
		t.Error("DryRun should be true")
	}
}

// ========== ExpansionStatus 测试 ==========

func TestExpansionStatusDefaults(t *testing.T) {
	status := ExpansionStatus{
		ID:        "test-expansion-1",
		PoolName:  "tank",
		State:     ExpansionStateIdle,
		Progress:  0,
		CanCancel: true,
	}

	if status.ID != "test-expansion-1" {
		t.Error("ID mismatch")
	}
	if status.State != ExpansionStateIdle {
		t.Error("State should be idle")
	}
	if status.Progress != 0 {
		t.Error("Progress should be 0")
	}
	if !status.CanCancel {
		t.Error("Should be cancellable by default")
	}
}

func TestExpansionStateValues(t *testing.T) {
	states := []ExpansionState{
		ExpansionStateIdle,
		ExpansionStatePreparing,
		ExpansionStateRunning,
		ExpansionStatePaused,
		ExpansionStateCompleted,
		ExpansionStateFailed,
		ExpansionStateCancelled,
	}

	for _, state := range states {
		if state == "" {
			t.Error("State should not be empty")
		}
	}
}

// ========== VdevExpansionInfo 测试 ==========

func TestVdevExpansionInfo(t *testing.T) {
	vdev := VdevExpansionInfo{
		VdevType:           "raidz2",
		Width:              6,
		ParityDisks:        2,
		DataDisks:          4,
		CanExpand:          true,
		ExpansionSupported: true,
	}

	if vdev.VdevType != "raidz2" {
		t.Error("VdevType should be raidz2")
	}
	if vdev.Width != 6 {
		t.Error("Width should be 6")
	}
	if vdev.ParityDisks != 2 {
		t.Error("ParityDisks should be 2 for RAIDZ2")
	}
	if vdev.DataDisks != 4 {
		t.Error("DataDisks should be 4")
	}
	if !vdev.CanExpand {
		t.Error("RAIDZ vdev should be expandable")
	}
}

// ========== PoolExpansionInfo 测试 ==========

func TestPoolExpansionInfo(t *testing.T) {
	info := PoolExpansionInfo{
		PoolName:      "tank",
		PoolState:     "ONLINE",
		TotalSize:     10 * 1024 * 1024 * 1024 * 1024, // 10TB
		AllocatedSize: 5 * 1024 * 1024 * 1024 * 1024,  // 5TB
		Vdevs: []VdevExpansionInfo{
			{
				VdevType:           "raidz2",
				Width:              6,
				ParityDisks:        2,
				DataDisks:          4,
				CanExpand:          true,
				ExpansionSupported: true,
			},
		},
		CanExpand: true,
	}

	if info.PoolName != "tank" {
		t.Error("PoolName should be tank")
	}
	if info.PoolState != "ONLINE" {
		t.Error("PoolState should be ONLINE")
	}
	if len(info.Vdevs) != 1 {
		t.Error("Should have 1 vdev")
	}
	if !info.CanExpand {
		t.Error("Pool should be expandable")
	}
}

// ========== RAIDZExpansionManager 测试 ==========

func TestNewRAIDZExpansionManager(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// 管理器可能没有 ZFS 可用（测试环境），但不应该报错
	// 只是 available 标志不同
}

func TestRAIDZExpansionManagerGetStatus(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	status := mgr.GetExpansionStatus()
	if status == nil {
		t.Fatal("Status should not be nil")
	}

	// 初始状态应该是 idle
	if status.State != ExpansionStateIdle {
		t.Errorf("Initial state should be idle, got %s", status.State)
	}
}

func TestRAIDZExpansionManagerHistory(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// 初始历史应该为空
	history := mgr.GetExpansionHistory()
	if len(history) != 0 {
		t.Error("Initial history should be empty")
	}
}

func TestExpansionStatusUpdate(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// 测试状态更新
	updated := false
	mgr.SetStateChangeCallback(func(status *ExpansionStatus) {
		updated = true
	})

	// 手动设置状态
	mgr.mu.Lock()
	mgr.currentStatus = &ExpansionStatus{
		ID:        "test",
		PoolName:  "tank",
		State:     ExpansionStateRunning,
		Progress:  50,
		CanCancel: true,
	}
	mgr.mu.Unlock()

	// 触发更新
	mgr.updateStatus(func(s *ExpansionStatus) {
		s.Progress = 75
	})

	// 验证状态已更新
	status := mgr.GetExpansionStatus()
	if status.Progress != 75 {
		t.Errorf("Progress should be 75, got %f", status.Progress)
	}

	// 回调是异步的，等待一下
	time.Sleep(100 * time.Millisecond)
	if !updated {
		t.Error("Callback should have been called")
	}
}

func TestExpansionErrors(t *testing.T) {
	// 测试错误定义
	errors := []error{
		ErrExpansionInProgress,
		ErrNoExpansionInProgress,
		ErrInvalidRAIDZLevel,
		ErrDiskNotFound,
		ErrDiskInUse,
		ErrPoolNotRAIDZ,
		ErrExpansionNotSupported,
		ErrExpansionFailed,
		ErrExpansionCancelled,
		ErrExpansionPaused,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// ========== parsePoolState 测试 ==========

func TestParsePoolState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`  pool: tank
 state: ONLINE
config:
	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
	  raidz2    ONLINE       0     0     0
`,
			"ONLINE",
		},
		{
			`  pool: broken
 state: DEGRADED
config:
`,
			"DEGRADED",
		},
		{
			"no state here",
			"unknown",
		},
	}

	for _, tt := range tests {
		got := parsePoolState(tt.input)
		if got != tt.expected {
			t.Errorf("parsePoolState() = %s, want %s", got, tt.expected)
		}
	}
}

// ========== generateExpansionID 测试 ==========

func TestGenerateExpansionID(t *testing.T) {
	id1 := generateExpansionID("tank")

	// 等待确保时间戳不同
	time.Sleep(time.Millisecond * 2)

	id2 := generateExpansionID("tank")

	// ID 应该不同（时间戳不同）
	if id1 == id2 {
		t.Error("IDs should be unique")
	}

	// ID 应该包含池名
	if !contains(id1, "tank") {
		t.Errorf("ID should contain pool name: %s", id1)
	}

	// 验证 ID 格式
	if !strings.HasPrefix(id1, "exp-") {
		t.Errorf("ID should start with 'exp-': %s", id1)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ========== ExpansionConfig Progress Callback 测试 ==========

func TestProgressCallback(t *testing.T) {
	var progressValues []float64

	config := ExpansionConfig{
		PoolName:   "tank",
		NewDisk:    "/dev/sdb",
		RAIDZLevel: RAIDZ2,
		ProgressCallback: func(progress float64) {
			progressValues = append(progressValues, progress)
		},
	}

	if config.ProgressCallback == nil {
		t.Error("ProgressCallback should be set")
	}

	// 测试调用回调
	config.ProgressCallback(50.0)
	config.ProgressCallback(100.0)

	if len(progressValues) != 2 {
		t.Errorf("Expected 2 progress values, got %d", len(progressValues))
	}
	if progressValues[0] != 50.0 {
		t.Errorf("First progress should be 50.0, got %f", progressValues[0])
	}
}

// ========== 并发测试 ==========

func TestConcurrentStatusAccess(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// 设置初始状态
	mgr.mu.Lock()
	mgr.currentStatus = &ExpansionStatus{
		ID:       "concurrent-test",
		PoolName: "tank",
		State:    ExpansionStateRunning,
	}
	mgr.mu.Unlock()

	// 并发读写测试
	done := make(chan bool)

	// 读协程
	go func() {
		for i := 0; i < 100; i++ {
			_ = mgr.GetExpansionStatus()
		}
		done <- true
	}()

	// 写协程
	go func() {
		for i := 0; i < 100; i++ {
			mgr.updateStatus(func(s *ExpansionStatus) {
				s.Progress = float64(i)
			})
		}
		done <- true
	}()

	// 等待完成
	<-done
	<-done
}

// ========== 边界条件测试 ==========

func TestExpansionStatusZeroValues(t *testing.T) {
	status := ExpansionStatus{}

	if status.ID != "" {
		t.Error("Default ID should be empty")
	}
	if state := status.State; state != "" {
		t.Error("Default state should be empty")
	}
	if status.Progress != 0 {
		t.Error("Default progress should be 0")
	}
	if status.BytesProcessed != 0 {
		t.Error("Default bytes processed should be 0")
	}
}

func TestExpansionConfigEmptyValues(t *testing.T) {
	config := ExpansionConfig{}

	if config.PoolName != "" {
		t.Error("Default pool name should be empty")
	}
	if config.NewDisk != "" {
		t.Error("Default new disk should be empty")
	}
	if config.Force {
		t.Error("Default force should be false")
	}
}

// ========== 时间相关测试 ==========

func TestExpansionStatusTimeFields(t *testing.T) {
	now := time.Now()
	status := ExpansionStatus{
		StartTime:              now,
		EndTime:                now.Add(time.Hour),
		LastUpdateTime:         now,
		EstimatedTimeRemaining: 30 * time.Minute,
	}

	if status.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
	if status.EndTime.Before(status.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
	if status.EstimatedTimeRemaining != 30*time.Minute {
		t.Error("EstimatedTimeRemaining should be 30 minutes")
	}
}

// ========== VdevExpansionInfo 类型测试 ==========

func TestVdevExpansionInfoTypes(t *testing.T) {
	// 测试各种 VDEV 类型
	vdevTypes := []struct {
		vdevType           string
		parity             int
		expansionSupported bool
	}{
		{"raidz1", 1, true},
		{"raidz2", 2, true},
		{"raidz3", 3, true},
		{"mirror", 0, false},
		{"disk", 0, false},
	}

	for _, tt := range vdevTypes {
		vdev := VdevExpansionInfo{
			VdevType:           tt.vdevType,
			ParityDisks:        tt.parity,
			ExpansionSupported: tt.expansionSupported,
		}

		if vdev.VdevType != tt.vdevType {
			t.Errorf("VdevType mismatch: got %s, want %s", vdev.VdevType, tt.vdevType)
		}
		if vdev.ParityDisks != tt.parity {
			t.Errorf("ParityDisks mismatch for %s: got %d, want %d", tt.vdevType, vdev.ParityDisks, tt.parity)
		}
	}
}

// ========== Context 超时测试 ==========

func TestExpansionWithTimeout(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 尝试获取池信息（可能会因为 ZFS 不可用而失败）
	_, err = mgr.GetPoolExpansionInfo(ctx, "tank")

	// 在没有 ZFS 的环境下，应该返回错误
	if err == nil {
		t.Log("GetPoolExpansionInfo succeeded (ZFS available)")
	} else {
		t.Logf("GetPoolExpansionInfo returned expected error: %v", err)
	}
}

// ========== EstimateExpansionTime 测试 ==========

func TestEstimateExpansionTime(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()

	// 在没有 ZFS 的环境下，应该返回错误
	_, err = mgr.EstimateExpansionTime(ctx, "tank")
	if err == nil {
		t.Log("EstimateExpansionTime succeeded (ZFS available)")
	} else if err != ErrZFSNotAvailable {
		t.Logf("EstimateExpansionTime returned error: %v", err)
	}
}

// ========== Benchmark 测试 ==========

func BenchmarkGetExpansionStatus(b *testing.B) {
	mgr, _ := NewRAIDZExpansionManager("")
	defer mgr.Close()

	mgr.mu.Lock()
	mgr.currentStatus = &ExpansionStatus{
		ID:       "bench-test",
		PoolName: "tank",
		State:    ExpansionStateRunning,
		Progress: 50,
	}
	mgr.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetExpansionStatus()
	}
}

func BenchmarkUpdateStatus(b *testing.B) {
	mgr, _ := NewRAIDZExpansionManager("")
	defer mgr.Close()

	mgr.mu.Lock()
	mgr.currentStatus = &ExpansionStatus{
		ID:       "bench-test",
		PoolName: "tank",
		State:    ExpansionStateRunning,
	}
	mgr.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.updateStatus(func(s *ExpansionStatus) {
			s.Progress = float64(i % 100)
		})
	}
}

func BenchmarkParseRAIDZLevel(b *testing.B) {
	levels := []string{"raidz1", "raidz2", "raidz3", "RAIDZ", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseRAIDZLevel(levels[i%len(levels)])
	}
}

// ========== 暂停/恢复/取消测试 ==========

func TestPauseResumeCancelErrors(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// 没有扩展进行时，暂停应该失败
	err = mgr.PauseExpansion()
	if err != ErrNoExpansionInProgress {
		t.Errorf("Expected ErrNoExpansionInProgress, got %v", err)
	}

	// 没有扩展进行时，恢复应该失败
	err = mgr.ResumeExpansion()
	if err != ErrNoExpansionInProgress {
		t.Errorf("Expected ErrNoExpansionInProgress, got %v", err)
	}

	// 没有扩展进行时，取消应该失败
	err = mgr.CancelExpansion()
	if err != ErrNoExpansionInProgress {
		t.Errorf("Expected ErrNoExpansionInProgress, got %v", err)
	}
}

// ========== CheckExpansionSupport 测试 ==========

func TestCheckExpansionSupport(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	supported, reason := mgr.CheckExpansionSupport()

	// 在测试环境中，ZFS 可能不可用
	if !supported {
		t.Logf("Expansion not supported: %s", reason)
	} else {
		t.Log("Expansion is supported")
	}
}

// ========== ListAvailableDisks 测试 ==========

func TestListAvailableDisks(t *testing.T) {
	mgr, err := NewRAIDZExpansionManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()

	// 在没有 ZFS 的环境下，应该返回错误
	_, err = mgr.ListAvailableDisks(ctx)
	if err == nil {
		t.Log("ListAvailableDisks succeeded (ZFS available)")
	} else if err != ErrZFSNotAvailable {
		t.Logf("ListAvailableDisks returned error: %v", err)
	}
}

// ========== ExpansionHistory 测试 ==========

func TestExpansionHistory(t *testing.T) {
	history := ExpansionHistory{
		Expansions: []ExpansionStatus{
			{
				ID:        "hist-1",
				PoolName:  "tank",
				State:     ExpansionStateCompleted,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-23 * time.Hour),
			},
			{
				ID:        "hist-2",
				PoolName:  "pool",
				State:     ExpansionStateFailed,
				StartTime: time.Now().Add(-12 * time.Hour),
				EndTime:   time.Now().Add(-12 * time.Hour),
				Errors:    []string{"disk error"},
			},
		},
		LastUpdated: time.Now(),
	}

	if len(history.Expansions) != 2 {
		t.Error("Should have 2 expansion records")
	}

	if history.Expansions[0].State != ExpansionStateCompleted {
		t.Error("First expansion should be completed")
	}

	if len(history.Expansions[1].Errors) != 1 {
		t.Error("Second expansion should have 1 error")
	}
}
