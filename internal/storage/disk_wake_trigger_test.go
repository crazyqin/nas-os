package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewDiskWakeTrigger(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	if trigger == nil {
		t.Fatal("expected non-nil trigger")
	}

	if trigger.config == nil {
		t.Error("expected default config to be set")
	}
}

func TestDefaultWakeTriggerConfig(t *testing.T) {
	config := DefaultWakeTriggerConfig()

	if config.StaggerDelayMs != 500 {
		t.Errorf("expected stagger delay 500, got %d", config.StaggerDelayMs)
	}

	if config.MaxConcurrentWakeups != 3 {
		t.Errorf("expected max concurrent wakeups 3, got %d", config.MaxConcurrentWakeups)
	}

	if !config.EnablePreWake {
		t.Error("expected pre-wake to be enabled")
	}
}

func TestRequestWakeUp_EmptyDiskPath(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	req := &WakeRequest{
		DiskPath: "",
	}

	_, err := trigger.RequestWakeUp(req)
	if err == nil {
		t.Error("expected error for empty disk path")
	}
}

func TestRequestWakeUp_UnregisteredDisk(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	req := &WakeRequest{
		DiskPath: "/dev/sda",
	}

	_, err := trigger.RequestWakeUp(req)
	if err == nil {
		t.Error("expected error for unregistered disk")
	}
}

func TestRequestWakeUp_AlreadyActive(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	// 注册磁盘（默认为active状态）
	powerMgr.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda"})

	req := &WakeRequest{
		DiskPath: "/dev/sda",
		Type:     WakeRequestImmediate,
	}

	result, err := trigger.RequestWakeUp(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected success for already active disk")
	}

	if result.DurationMs != 0 {
		t.Errorf("expected 0 duration for already active disk, got %d", result.DurationMs)
	}
}

func TestRequestWakeUp_ImmediateWakeup(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)
	trigger.Start()
	defer trigger.Stop()

	// 注册磁盘并设置为休眠状态
	powerMgr.RegisterDisk(&DiskPowerConfig{
		DevicePath:    "/dev/sda",
		DelayWakeUpMs: 10, // 快速测试
	})
	powerMgr.SetState("/dev/sda", DiskPowerSleeping, "test")

	req := &WakeRequest{
		DiskPath: "/dev/sda",
		Type:     WakeRequestImmediate,
		Reason:   "user_access",
	}

	// 等待唤醒完成
	time.Sleep(100 * time.Millisecond)

	_, err := trigger.RequestWakeUp(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 检查磁盘状态
	state, _ := powerMgr.GetState("/dev/sda")
	if state != DiskPowerActive {
		t.Errorf("expected active state after wake, got %s", state)
	}
}

func TestRequestWakeUp_BatchedRequest(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)

	config := &WakeTriggerConfig{
		StaggerDelayMs:       10,
		MaxConcurrentWakeups: 3,
		BatchWindowMs:        100,
		WakeTimeoutMs:        5000,
	}

	trigger := NewDiskWakeTrigger(ctx, config, powerMgr)
	trigger.Start()
	defer trigger.Stop()

	// 注册磁盘
	powerMgr.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda", DelayWakeUpMs: 10})
	powerMgr.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sdb", DelayWakeUpMs: 10})
	powerMgr.SetState("/dev/sda", DiskPowerSleeping, "test")
	powerMgr.SetState("/dev/sdb", DiskPowerSleeping, "test")

	// 发送批量请求
	req1 := &WakeRequest{
		DiskPath: "/dev/sda",
		Type:     WakeRequestBatched,
	}
	req2 := &WakeRequest{
		DiskPath: "/dev/sdb",
		Type:     WakeRequestBatched,
	}

	trigger.RequestWakeUp(req1)
	trigger.RequestWakeUp(req2)

	// 等待批量窗口处理
	time.Sleep(200 * time.Millisecond)

	// 检查统计
	stats := trigger.GetStats()
	if stats.TotalWakeups < 1 {
		t.Errorf("expected at least 1 wakeup, got %d", stats.TotalWakeups)
	}
}

func TestRecordAccess_WakeTrigger(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	trigger.recordAccess("/dev/sda")
	trigger.recordAccess("/dev/sda")
	trigger.recordAccess("/dev/sda")

	patterns := trigger.GetAccessPatterns()
	pattern, exists := patterns["/dev/sda"]
	if !exists {
		t.Fatal("expected access pattern to be recorded")
	}

	if pattern.AccessCount != 3 {
		t.Errorf("expected 3 access count, got %d", pattern.AccessCount)
	}
}

func TestGetStats(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	stats := trigger.GetStats()

	if stats.PendingRequests != 0 {
		t.Errorf("expected 0 pending requests, got %d", stats.PendingRequests)
	}

	if stats.QueueLength != 0 {
		t.Errorf("expected 0 queue length, got %d", stats.QueueLength)
	}
}

func TestWakeTriggerCallbacks(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)
	trigger.Start()
	defer trigger.Stop()

	var successCalled bool
	var _ bool // failure callback placeholder
	var mu sync.Mutex

	trigger.SetCallbacks(
		func(result WakeResult) {
			mu.Lock()
			successCalled = true
			mu.Unlock()
		},
		func(result WakeResult) {
			// failure callback - unused in this test
		},
	)

	// 注册磁盘并唤醒
	powerMgr.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda", DelayWakeUpMs: 10})
	powerMgr.SetState("/dev/sda", DiskPowerSleeping, "test")

	req := &WakeRequest{
		DiskPath: "/dev/sda",
		Type:     WakeRequestImmediate,
	}

	trigger.RequestWakeUp(req)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !successCalled {
		t.Error("expected success callback to be called")
	}
	mu.Unlock()
}

func TestCancelRequest(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	// 加入队列的请求
	req := &WakeRequest{
		ID:       "test-cancel-1",
		DiskPath: "/dev/sda",
		Type:     WakeRequestScheduled,
	}

	trigger.mu.Lock()
	trigger.pendingRequests[req.ID] = req
	trigger.mu.Unlock()

	// 取消请求
	cancelled := trigger.CancelRequest(req.ID)
	if !cancelled {
		t.Error("expected request to be cancelled")
	}

	// 检查是否已删除
	trigger.mu.RLock()
	_, exists := trigger.pendingRequests[req.ID]
	trigger.mu.RUnlock()

	if exists {
		t.Error("expected request to be removed from pending")
	}
}

func TestUpdateConfig(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	newConfig := &WakeTriggerConfig{
		StaggerDelayMs:       1000,
		MaxConcurrentWakeups: 5,
	}

	err := trigger.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := trigger.GetConfig()
	if config.StaggerDelayMs != 1000 {
		t.Errorf("expected stagger delay 1000, got %d", config.StaggerDelayMs)
	}
}

func TestUpdateConfig_Nil(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	err := trigger.UpdateConfig(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestGetWakeHistory(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	// 添加一些历史记录
	trigger.mu.Lock()
	trigger.resultsHistory = append(trigger.resultsHistory,
		WakeResult{RequestID: "1", DiskPath: "/dev/sda", Success: true},
		WakeResult{RequestID: "2", DiskPath: "/dev/sdb", Success: true},
		WakeResult{RequestID: "3", DiskPath: "/dev/sdc", Success: false},
	)
	trigger.mu.Unlock()

	history := trigger.GetWakeHistory(2)
	if len(history) != 2 {
		t.Errorf("expected 2 history items, got %d", len(history))
	}
}

func TestPreWakeMonitor(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)

	config := DefaultWakeTriggerConfig()
	config.EnablePreWake = true
	config.PreWakeAccessThreshold = 5

	trigger := NewDiskWakeTrigger(ctx, config, powerMgr)

	// 设置访问模式（高峰时段）
	trigger.mu.Lock()
	trigger.accessPatterns["/dev/sda"] = &DiskAccessPattern{
		DiskPath:      "/dev/sda",
		AvgAccessHour: 10.0, // 高于阈值
		PeakHours:     []int{time.Now().Hour()},
	}
	trigger.mu.Unlock()

	// 注册磁盘
	powerMgr.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda", DelayWakeUpMs: 10})
	powerMgr.SetState("/dev/sda", DiskPowerSleeping, "test")

	// 检查预唤醒候选（不启动完整监控）
	trigger.checkPreWakeCandidates()

	// 由于是异步执行，等待一下
	time.Sleep(50 * time.Millisecond)
}

func TestStop_WakeTrigger(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)
	trigger.Start()

	// Should not block
	trigger.Stop()

	// Verify context is cancelled
	select {
	case <-trigger.ctx.Done():
		// Expected
	default:
		t.Error("expected context to be cancelled after Stop()")
	}
}

func TestCalculatePeakHours(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)
	trigger := NewDiskWakeTrigger(ctx, nil, powerMgr)

	hourlyAccess := map[int]int{
		0:  5,
		8:  20,  // 高峰
		12: 15,
		18: 25,  // 最高峰
		22: 10,
	}

	peakHours := trigger.calculatePeakHours(hourlyAccess)

	if len(peakHours) == 0 {
		t.Error("expected at least one peak hour")
	}

	// 最高峰应该是18点
	if peakHours[0] != 18 {
		t.Errorf("expected peak hour 18, got %d", peakHours[0])
	}
}

func TestRequestWakeUp_WithRetry(t *testing.T) {
	ctx := context.Background()
	powerMgr := NewDiskPowerManager(ctx)

	config := &WakeTriggerConfig{
		StaggerDelayMs:       10,
		MaxConcurrentWakeups: 1,
		WakeTimeoutMs:        5000,
		WakeRetryCount:       2,
		WakeRetryDelayMs:     50,
	}

	trigger := NewDiskWakeTrigger(ctx, config, powerMgr)
	trigger.Start()
	defer trigger.Stop()

	// 注册磁盘
	powerMgr.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda", DelayWakeUpMs: 10})
	powerMgr.SetState("/dev/sda", DiskPowerSleeping, "test")

	req := &WakeRequest{
		DiskPath: "/dev/sda",
		Type:     WakeRequestImmediate,
	}

	// 正常情况下应该成功
	result, err := trigger.RequestWakeUp(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected successful wake")
	}
}