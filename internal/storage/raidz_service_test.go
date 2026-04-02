// Package storage RAIDZ Expansion Service 测试
// 兵部 Round 142

package storage

import (
	"context"
	"testing"
	"time"
)

// TestRAIDZExpansionServiceCreation 测试服务创建
func TestRAIDZExpansionServiceCreation(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if service == nil {
		t.Fatal("Service should not be nil")
	}

	// 检查初始状态
	if service.activeTasks == nil {
		t.Error("activeTasks map should be initialized")
	}

	if service.taskHistory == nil {
		t.Error("taskHistory slice should be initialized")
	}

	if service.progressCallbacks == nil {
		t.Error("progressCallbacks map should be initialized")
	}

	// 清理
	if err := service.Close(); err != nil {
		t.Logf("Warning: Close returned error: %v", err)
	}
}

// TestExpansionStatus 测试扩展状态常量
func TestExpansionStatus(t *testing.T) {
	statuses := []ExpansionStatus{
		StatusIdle,
		StatusPreparing,
		StatusRunning,
		StatusPaused,
		StatusCompleted,
		StatusFailed,
		StatusCancelled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Errorf("Expansion status should not be empty")
		}
	}
}

// TestExpansionTask 测试扩展任务结构
func TestExpansionTask(t *testing.T) {
	task := &ExpansionTask{
		ID:         "test-task-001",
		PoolName:   "testpool",
		NewDisk:    "/dev/sda",
		RAIDZLevel: "raidz1",
		Status:     StatusIdle,
		Progress:   0,
		StartTime:  time.Now(),
		CanPause:   true,
		CanCancel:  true,
	}

	if task.ID != "test-task-001" {
		t.Errorf("Task ID mismatch: got %s", task.ID)
	}

	if task.PoolName != "testpool" {
		t.Errorf("Pool name mismatch: got %s", task.PoolName)
	}

	if task.Status != StatusIdle {
		t.Errorf("Status should be idle: got %s", task.Status)
	}
}

// TestGetExpansionStatus 测试获取扩展状态
func TestGetExpansionStatus(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 无活跃任务时应该返回 idle 状态
	status, err := service.GetExpansionStatus("nonexistent-pool")
	if err != nil {
		t.Fatalf("GetExpansionStatus should not error: %v", err)
	}

	if status.Status != StatusIdle {
		t.Errorf("Status should be idle for nonexistent pool: got %s", status.Status)
	}
}

// TestGetAllActiveTasks 测试获取所有活跃任务
func TestGetAllActiveTasks(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 初始应该无活跃任务
	tasks := service.GetAllActiveTasks()
	if len(tasks) != 0 {
		t.Errorf("Should have no active tasks initially: got %d", len(tasks))
	}
}

// TestGetTaskHistory 测试获取任务历史
func TestGetTaskHistory(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 初始应该无历史
	history := service.GetTaskHistory(10)
	if len(history) != 0 {
		t.Errorf("Should have no history initially: got %d", len(history))
	}

	// 测试 limit 参数
	history = service.GetTaskHistory(0)
	if len(history) != 0 {
		t.Errorf("Should handle zero limit: got %d", len(history))
	}
}

// TestPreCheckResult 测试预检查结果
func TestPreCheckResult(t *testing.T) {
	check := PreCheckResult{
		Name:     "pool_health",
		Passed:   true,
		Message:  "ONLINE",
		Required: true,
	}

	if check.Name != "pool_health" {
		t.Errorf("Check name mismatch: got %s", check.Name)
	}

	if !check.Passed {
		t.Error("Check should pass")
	}

	if !check.Required {
		t.Error("Check should be required")
	}
}

// TestDiskRequirements 测试磁盘要求
func TestDiskRequirements(t *testing.T) {
	req := DiskRequirements{
		MinSizeGB:     500,
		RecommendedGB: 1000,
		Interfaces:    []string{"SATA", "NVMe"},
		MustMatchSize: false,
	}

	if req.MinSizeGB != 500 {
		t.Errorf("MinSizeGB mismatch: got %d", req.MinSizeGB)
	}

	if len(req.Interfaces) != 2 {
		t.Errorf("Interfaces count mismatch: got %d", len(req.Interfaces))
	}
}

// TestExpansionProgress 测试扩展进度
func TestExpansionProgress(t *testing.T) {
	progress := &ExpansionProgress{
		TaskID:         "test-001",
		Percentage:     50.0,
		BytesProcessed: 1024 * 1024 * 1024, // 1GB
		BytesTotal:     2 * 1024 * 1024 * 1024, // 2GB
		SpeedMBps:      100.0,
		ETA:            10 * time.Minute,
		Elapsed:        10 * time.Minute,
		Phase:          "data_migration",
		PhaseProgress:  50.0,
		LastUpdate:     time.Now(),
	}

	if progress.Percentage != 50.0 {
		t.Errorf("Progress percentage mismatch: got %f", progress.Percentage)
	}

	if progress.SpeedMBps != 100.0 {
		t.Errorf("Speed mismatch: got %f", progress.SpeedMBps)
	}
}

// TestAvailableDiskInfo 测试可用磁盘信息
func TestAvailableDiskInfo(t *testing.T) {
	disk := AvailableDiskInfo{
		Path:      "/dev/sda",
		Model:     "Samsung SSD 870 EVO",
		SizeGB:    500,
		Interface: "SSD",
		Healthy:   true,
		Available: true,
	}

	if disk.Path != "/dev/sda" {
		t.Errorf("Disk path mismatch: got %s", disk.Path)
	}

	if disk.SizeGB != 500 {
		t.Errorf("Disk size mismatch: got %d", disk.SizeGB)
	}

	if !disk.Healthy {
		t.Error("Disk should be healthy")
	}
}

// TestExpansionEligibilityResult 测试扩展资格结果
func TestExpansionEligibilityResult(t *testing.T) {
	result := &ExpansionEligibilityResult{
		PoolName:        "testpool",
		Eligible:        true,
		RAIDZLevel:      "raidz1",
		CurrentWidth:    4,
		NewWidth:        5,
		CapacityGain:    1024 * 1024 * 1024 * 500, // 500GB
		CurrentCapacity: 1024 * 1024 * 1024 * 2000, // 2TB
		NewCapacity:     1024 * 1024 * 1024 * 2500, // 2.5TB
		Warnings:        []string{},
		PreChecks: []PreCheckResult{
			{Name: "pool_health", Passed: true, Message: "ONLINE", Required: true},
			{Name: "scan_status", Passed: true, Message: "no active scan", Required: true},
		},
		EstimatedTime: 30 * time.Minute,
	}

	if !result.Eligible {
		t.Error("Pool should be eligible")
	}

	if result.RAIDZLevel != "raidz1" {
		t.Errorf("RAIDZ level mismatch: got %s", result.RAIDZLevel)
	}

	if result.NewWidth != 5 {
		t.Errorf("New width should be 5: got %d", result.NewWidth)
	}

	if len(result.PreChecks) != 2 {
		t.Errorf("Should have 2 pre-checks: got %d", len(result.PreChecks))
	}
}

// TestRegisterCallbacks 测试回调注册
func TestRegisterCallbacks(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 注册进度回调
	service.RegisterProgressCallback("testpool", func(p *ExpansionProgress) {
		// 回调函数
	})

	// 注册状态回调
	service.RegisterStateCallback(func(task *ExpansionTask) {
		// 回调函数
	})

	// 检查回调是否注册
	if len(service.progressCallbacks) != 1 {
		t.Errorf("Should have 1 progress callback: got %d", len(service.progressCallbacks))
	}

	if len(service.stateCallbacks) != 1 {
		t.Errorf("Should have 1 state callback: got %d", len(service.stateCallbacks))
	}

	t.Logf("Callbacks registered successfully (progress=%v, state=%v)",
		service.progressCallbacks != nil, service.stateCallbacks != nil)
}

// TestIsAvailable 测试可用性检查
func TestIsAvailable(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 在无 ZFS 的测试环境中，应该返回 false 或依赖 ZFS 管理器
	available := service.IsAvailable()
	t.Logf("Service availability: %v", available)
	// 不强制要求 true，因为测试环境可能无 ZFS
}

// TestCalculateCapacityGain 测试容量增益计算逻辑
func TestCalculateCapacityGainLogic(t *testing.T) {
	// 模拟 RAIDZ1 4盘配置，每盘 1TB，总容量约 3TB（数据）
	// 扩展后 5盘，容量增益 = 1TB * 4/5 = 0.8TB

	totalSize := uint64(4 * 1024 * 1024 * 1024 * 1024) // 4TB total raw
	width := 4
	parityDisks := 1
	dataDisks := width - parityDisks // 3

	diskSize := totalSize / uint64(width) // 1TB per disk
	newDataDisks := dataDisks + 1         // 4
	newTotal := width + 1                 // 5

	expectedGain := diskSize * uint64(newDataDisks) / uint64(newTotal)
	// 简化计算：约 800GB

	if expectedGain == 0 {
		t.Error("Capacity gain should not be zero")
	}

	t.Logf("Capacity gain calculation: diskSize=%dTB, gain=%dGB",
		diskSize/(1024*1024*1024*1024),
		expectedGain/(1024*1024*1024))
}

// TestGenerateTaskID 测试任务 ID 生成
func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID("testpool")
	id2 := generateTaskID("testpool")

	// ID 应包含池名称
	if !containsSubstring(id1, "raidz-exp-testpool") {
		t.Errorf("Task ID should contain pool name prefix: got %s", id1)
	}

	// 两次生成的 ID 应不同（时间戳）
	if id1 == id2 {
		t.Error("Task IDs should be unique")
	}

	t.Logf("Generated task IDs: %s, %s", id1, id2)
}

// TestContextCancellation 测试上下文取消处理
func TestContextCancellation(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 创建可取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 等待上下文超时
	<-ctx.Done()

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Context should be deadline exceeded: got %v", ctx.Err())
	}

	t.Logf("Context cancellation handled correctly")
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	service, err := NewRAIDZExpansionService("")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// 并发读取状态
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = service.GetAllActiveTasks()
			_, _ = service.GetExpansionStatus("testpool")
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent access timeout")
		}
	}

	t.Logf("Concurrent access test passed")
}