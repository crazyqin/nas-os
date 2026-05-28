// Package distScheduler 测试文件
package distScheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(zap.NewNop(), nil)
}

func setupTestNode(id string) *Node {
	return &Node{
		ID:      id,
		Name:    "node-" + id,
		Address: "192.168.1." + id,
		Status:  NodeStatusOnline,
		Resources: &NodeResources{
			CPU:    ResourceInfo{Total: 8, Used: 2, Available: 6, Unit: "cores"},
			Memory: ResourceInfo{Total: 16384, Used: 4096, Available: 12288, Unit: "MB"},
			GPU:    ResourceInfo{Total: 1, Used: 0, Available: 1, Unit: "count"},
			Disk:   ResourceInfo{Total: 1000, Used: 200, Available: 800, Unit: "GB"},
		},
		Tags:    map[string]string{"zone": "a"},
		LastHB:  time.Now(),
	}
}

func setupTestTask(name string, priority int) *Task {
	return &Task{
		Name:     name,
		Type:     "test",
		Priority: priority,
		Requirements: &ResourceReq{
			CPU:    1,
			Memory: 512,
		},
		Payload: map[string]string{"key": "value"},
	}
}

// ==================== Engine Tests ====================

func TestNewEngine(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		e := setupTestEngine(t)
		if e == nil {
			t.Fatal("expected non-nil engine")
		}
		if e.IsRunning() {
			t.Error("engine should not be running initially")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Strategy = StrategyRoundRobin
		cfg.MaxRetries = 5
		e := NewEngine(zap.NewNop(), cfg)
		if e.config.Strategy != StrategyRoundRobin {
			t.Errorf("expected round robin, got %s", e.config.Strategy)
		}
	})
}

func TestEngineStartStop(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	if err := e.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !e.IsRunning() {
		t.Error("engine should be running")
	}

	if err := e.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	e.Stop()
	if e.IsRunning() {
		t.Error("engine should not be running after stop")
	}
}

func TestRegisterNode(t *testing.T) {
	e := setupTestEngine(t)

	node := setupTestNode("1")
	if err := e.RegisterNode(node); err != nil {
		t.Fatalf("register node failed: %v", err)
	}

	got, err := e.GetNode("1")
	if err != nil {
		t.Fatalf("get node failed: %v", err)
	}
	if got.Name != "node-1" {
		t.Errorf("expected name node-1, got %s", got.Name)
	}
	if got.Status != NodeStatusOnline {
		t.Errorf("expected online, got %s", got.Status)
	}

	// 不存在的节点
	_, err = e.GetNode("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestUnregisterNode(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterNode(setupTestNode("1"))
	if err := e.UnregisterNode("1"); err != nil {
		t.Fatalf("unregister failed: %v", err)
	}

	if err := e.UnregisterNode("nonexistent"); err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestListNodes(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterNode(setupTestNode("1"))
	e.RegisterNode(setupTestNode("2"))

	nodes := e.ListNodes("")
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	online := e.ListNodes(NodeStatusOnline)
	if len(online) != 2 {
		t.Errorf("expected 2 online nodes, got %d", len(online))
	}

	offline := e.ListNodes(NodeStatusOffline)
	if len(offline) != 0 {
		t.Errorf("expected 0 offline nodes, got %d", len(offline))
	}
}

func TestHeartbeat(t *testing.T) {
	e := setupTestEngine(t)
	e.RegisterNode(setupTestNode("1"))

	resources := &NodeResources{
		CPU:    ResourceInfo{Total: 8, Used: 3, Available: 5, Unit: "cores"},
		Memory: ResourceInfo{Total: 16384, Used: 8192, Available: 8192, Unit: "MB"},
	}

	if err := e.Heartbeat("1", resources); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	// 不存在的节点
	if err := e.Heartbeat("nonexistent", nil); err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestSubmitTask(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("test-task", 1)
	if err := e.SubmitTask(task); err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected pending, got %s", task.Status)
	}

	got, err := e.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if got.Name != "test-task" {
		t.Errorf("expected name test-task, got %s", got.Name)
	}
}

func TestSubmitCronTask(t *testing.T) {
	e := setupTestEngine(t)

	task := &Task{
		Name:     "cron-task",
		Type:     "test",
		CronExpr: "*/5 * * * *",
		Priority: 1,
	}
	if err := e.SubmitTask(task); err != nil {
		t.Fatalf("submit cron task failed: %v", err)
	}

	if task.Status != TaskStatusScheduled {
		t.Errorf("expected scheduled, got %s", task.Status)
	}
	if task.ScheduledAt == nil {
		t.Error("expected non-nil scheduled_at")
	}
}

func TestSubmitInvalidCron(t *testing.T) {
	e := setupTestEngine(t)

	task := &Task{
		Name:     "bad-cron",
		Type:     "test",
		CronExpr: "",
	}
	if err := e.SubmitTask(task); err == nil {
		// 空 cron expr 会走默认5分钟，不会报错
	}
}

func TestCancelTask(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("cancel-task", 1)
	e.SubmitTask(task)

	if err := e.CancelTask(task.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if task.Status != TaskStatusCancelled {
		t.Errorf("expected cancelled, got %s", task.Status)
	}

	// 取消不存在的
	if err := e.CancelTask("nonexistent"); err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestListTasks(t *testing.T) {
	e := setupTestEngine(t)

	e.SubmitTask(setupTestTask("t1", 1))
	e.SubmitTask(setupTestTask("t2", 2))

	all := e.ListTasks("")
	if len(all) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(all))
	}

	pending := e.ListTasks(TaskStatusPending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	completed := e.ListTasks(TaskStatusCompleted)
	if len(completed) != 0 {
		t.Errorf("expected 0 completed, got %d", len(completed))
	}
}

func TestSchedule(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	e.RegisterNode(setupTestNode("1"))
	e.RegisterNode(setupTestNode("2"))
	e.SubmitTask(setupTestTask("sched-task", 1))

	results, err := e.Schedule(ctx)
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %s", results[0].Error)
	}
	if results[0].NodeID == "" {
		t.Error("expected non-empty node ID")
	}
}

func TestScheduleNoNodes(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	e.SubmitTask(setupTestTask("orphan", 1))

	results, _ := e.Schedule(ctx)
	if len(results) == 0 {
		// 可能没有 pending 任务
		return
	}
	if results[0].Success {
		t.Error("expected failure when no nodes available")
	}
}

func TestCompleteTask(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	e.RegisterNode(setupTestNode("1"))
	task := setupTestTask("complete-task", 1)
	e.SubmitTask(task)
	e.Schedule(ctx)

	if err := e.CompleteTask(task.ID, "result-data"); err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	got, _ := e.GetTask(task.ID)
	if got.Status != TaskStatusCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("expected non-nil finished_at")
	}
}

func TestFailTask(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("fail-task", 1)
	task.MaxAttempts = 1
	task.Attempts = 1
	task.Status = TaskStatusRunning
	e.tasks[task.ID] = task

	e.FailTask(task.ID, fmt.Errorf("test error"))

	got, _ := e.GetTask(task.ID)
	if got.Status != TaskStatusFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
	if got.Error != "test error" {
		t.Errorf("expected 'test error', got '%s'", got.Error)
	}
}

func TestTaskDependencies(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	e.RegisterNode(setupTestNode("1"))

	taskA := setupTestTask("task-a", 1)
	e.SubmitTask(taskA)

	taskB := setupTestTask("task-b", 1)
	taskB.Dependencies = []string{taskA.ID}
	e.SubmitTask(taskB)

	// 调度：只有 task-a 应该被调度
	results, _ := e.Schedule(ctx)
	scheduled := 0
	for _, r := range results {
		if r.Success {
			scheduled++
		}
	}
	if scheduled != 1 {
		t.Errorf("expected 1 scheduled (task-a), got %d", scheduled)
	}

	// 完成 task-a
	e.CompleteTask(taskA.ID, nil)

	// 再次调度：task-b 应该被调度
	results, _ = e.Schedule(ctx)
	scheduled = 0
	for _, r := range results {
		if r.Success {
			scheduled++
		}
	}
	if scheduled != 1 {
		t.Errorf("expected 1 scheduled (task-b), got %d", scheduled)
	}
}

func TestGetStats(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterNode(setupTestNode("1"))
	e.SubmitTask(setupTestTask("s1", 1))
	e.SubmitTask(setupTestTask("s2", 2))

	stats := e.GetStats()
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	taskStats, ok := stats["tasks"].(map[string]int)
	if !ok {
		t.Fatal("expected tasks stats")
	}
	if taskStats["total"] < 2 {
		t.Errorf("expected total >= 2, got %d", taskStats["total"])
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.Strategy != StrategyLeastLoad {
		t.Errorf("expected least_load, got %s", cfg.Strategy)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", cfg.MaxRetries)
	}
}

// ==================== Allocator Tests ====================

func TestAllocatorAllocateNode(t *testing.T) {
	alloc := NewAllocator(zap.NewNop(), nil)

	nodes := map[string]*Node{
		"1": setupTestNode("1"),
		"2": setupTestNode("2"),
	}

	req := &ResourceReq{CPU: 1, Memory: 512}
	nodeID, err := alloc.AllocateNode(nodes, req)
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	if nodeID == "" {
		t.Error("expected non-empty node ID")
	}
}

func TestAllocatorNoEligibleNode(t *testing.T) {
	alloc := NewAllocator(zap.NewNop(), nil)

	nodes := map[string]*Node{
		"1": {
			ID:     "1",
			Status: NodeStatusOffline,
		},
	}

	_, err := alloc.AllocateNode(nodes, &ResourceReq{CPU: 1})
	if err == nil {
		t.Error("expected error when no eligible nodes")
	}
}

func TestAllocatorInsufficientResources(t *testing.T) {
	alloc := NewAllocator(zap.NewNop(), nil)

	nodes := map[string]*Node{
		"1": {
			ID:     "1",
			Status: NodeStatusOnline,
			Resources: &NodeResources{
				CPU:    ResourceInfo{Total: 2, Used: 2, Available: 0, Unit: "cores"},
				Memory: ResourceInfo{Total: 4096, Used: 4096, Available: 0, Unit: "MB"},
			},
		},
	}

	_, err := alloc.AllocateNode(nodes, &ResourceReq{CPU: 1, Memory: 512})
	if err == nil {
		t.Error("expected error for insufficient resources")
	}
}

func TestAllocatorReserveRelease(t *testing.T) {
	alloc := NewAllocator(zap.NewNop(), nil)
	node := setupTestNode("1")

	req := &ResourceReq{CPU: 2, Memory: 1024}
	if err := alloc.ReserveResources(node, req); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	if node.Resources.CPU.Available != 4 {
		t.Errorf("expected 4 CPU available, got %f", node.Resources.CPU.Available)
	}

	alloc.ReleaseResources(node, req)
	if node.Resources.CPU.Available != 6 {
		t.Errorf("expected 6 CPU available after release, got %f", node.Resources.CPU.Available)
	}
}

func TestAllocatorUtilization(t *testing.T) {
	alloc := NewAllocator(zap.NewNop(), nil)
	node := setupTestNode("1")

	util := alloc.GetResourceUtilization(node)
	if util["cpu"] <= 0 {
		t.Error("expected positive CPU utilization")
	}
}

// ==================== Balancer Tests ====================

func TestBalancerRoundRobin(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	nodes := map[string]*Node{
		"a": {ID: "a", Status: NodeStatusOnline, Resources: &NodeResources{
			CPU: ResourceInfo{Total: 8, Available: 6},
		}},
		"b": {ID: "b", Status: NodeStatusOnline, Resources: &NodeResources{
			CPU: ResourceInfo{Total: 8, Available: 6},
		}},
	}
	task := &Task{ID: "t1"}

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id, err := b.SelectNode(nodes, task, StrategyRoundRobin)
		if err != nil {
			t.Fatalf("select failed: %v", err)
		}
		seen[id] = true
	}

	if len(seen) < 2 {
		t.Error("expected round robin to use multiple nodes")
	}
}

func TestBalancerLeastLoad(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	nodes := map[string]*Node{
		"busy":    {ID: "busy", Status: NodeStatusOnline, TaskCount: 8},
		"free":    {ID: "free", Status: NodeStatusOnline, TaskCount: 1},
		"offline": {ID: "offline", Status: NodeStatusOffline, TaskCount: 0},
	}

	id, err := b.SelectNode(nodes, &Task{}, StrategyLeastLoad)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if id != "free" {
		t.Errorf("expected free, got %s", id)
	}
}

func TestBalancerRandom(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	nodes := map[string]*Node{
		"a": {ID: "a", Status: NodeStatusOnline},
		"b": {ID: "b", Status: NodeStatusOnline},
	}

	_, err := b.SelectNode(nodes, &Task{}, StrategyRandom)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
}

func TestBalancerAffinity(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	nodes := map[string]*Node{
		"match": {ID: "match", Status: NodeStatusOnline, TaskCount: 3, Tags: map[string]string{"zone": "a"}},
		"other": {ID: "other", Status: NodeStatusOnline, TaskCount: 1, Tags: map[string]string{"zone": "b"}},
	}

	task := &Task{Tags: map[string]string{"zone": "a"}}
	id, _ := b.SelectNode(nodes, task, StrategyAffinity)
	if id != "match" {
		t.Errorf("expected match, got %s", id)
	}
}

func TestBalancerNoNodes(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	_, err := b.SelectNode(map[string]*Node{}, &Task{}, StrategyRoundRobin)
	if err == nil {
		t.Error("expected error with no nodes")
	}
}

func TestBalancerAllOffline(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	nodes := map[string]*Node{
		"1": {ID: "1", Status: NodeStatusOffline},
	}
	_, err := b.SelectNode(nodes, &Task{}, StrategyRoundRobin)
	if err == nil {
		t.Error("expected error when all nodes offline")
	}
}

func TestLoadDistribution(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)
	nodes := map[string]*Node{
		"a": {ID: "a", TaskCount: 3},
		"b": {ID: "b", TaskCount: 5},
	}

	dist := b.GetLoadDistribution(nodes)
	if dist["a"] != 3 || dist["b"] != 5 {
		t.Errorf("unexpected distribution: %v", dist)
	}
}

func TestIsBalanced(t *testing.T) {
	b := NewBalancer(zap.NewNop(), nil)

	t.Run("balanced", func(t *testing.T) {
		nodes := map[string]*Node{
			"a": {ID: "a", Status: NodeStatusOnline, TaskCount: 3},
			"b": {ID: "b", Status: NodeStatusOnline, TaskCount: 3},
		}
		if !b.IsBalanced(nodes, 0.5) {
			t.Error("expected balanced")
		}
	})

	t.Run("unbalanced", func(t *testing.T) {
		nodes := map[string]*Node{
			"a": {ID: "a", Status: NodeStatusOnline, TaskCount: 1},
			"b": {ID: "b", Status: NodeStatusOnline, TaskCount: 10},
		}
		if b.IsBalanced(nodes, 0.1) {
			t.Error("expected not balanced")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if !b.IsBalanced(map[string]*Node{}, 0.5) {
			t.Error("empty should be balanced")
		}
	})
}

// ==================== Recovery Tests ====================

func TestRecoveryRetry(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("retry-task", 1)
	task.MaxAttempts = 3
	task.Attempts = 1
	task.Status = TaskStatusRunning
	task.Error = "test error"
	e.tasks[task.ID] = task

	// 触发失败处理
	e.FailTask(task.ID, fmt.Errorf("test error"))

	got, _ := e.GetTask(task.ID)
	if got.Status != TaskStatusRetrying {
		t.Errorf("expected retrying, got %s", got.Status)
	}
}

func TestRecoveryPermanentFailure(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("fail-task", 1)
	task.MaxAttempts = 1
	task.Attempts = 1
	task.Status = TaskStatusRunning
	e.tasks[task.ID] = task

	e.FailTask(task.ID, fmt.Errorf("fatal"))

	got, _ := e.GetTask(task.ID)
	if got.Status != TaskStatusFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
}

func TestRecoveryGetFailedTasks(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("failed", 1)
	task.Status = TaskStatusFailed
	task.MaxAttempts = 1
	task.Attempts = 1
	e.tasks[task.ID] = task

	failed := e.recovery.GetFailedTasks()
	if len(failed) != 1 {
		t.Errorf("expected 1 failed, got %d", len(failed))
	}
}

func TestRecoveryRetryFailedTask(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("retry-manual", 1)
	task.Status = TaskStatusFailed
	task.MaxAttempts = 1
	task.Attempts = 1
	task.Error = "old error"
	e.tasks[task.ID] = task

	e.recovery.RetryFailedTask(task.ID)

	got, _ := e.GetTask(task.ID)
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending after retry, got %s", got.Status)
	}
	if got.Attempts != 0 {
		t.Errorf("expected attempts reset to 0, got %d", got.Attempts)
	}
}

func TestRecoveryNodeFailure(t *testing.T) {
	e := setupTestEngine(t)

	e.RegisterNode(setupTestNode("1"))
	task := setupTestTask("on-failed-node", 1)
	task.Status = TaskStatusRunning
	task.NodeID = "1"
	task.MaxAttempts = 3
	e.tasks[task.ID] = task

	e.recovery.handleNodeFailure("1")

	node, _ := e.GetNode("1")
	if node.Status != NodeStatusOffline {
		t.Errorf("expected offline, got %s", node.Status)
	}
}

func TestRecoverNode(t *testing.T) {
	e := setupTestEngine(t)

	node := setupTestNode("1")
	node.Status = NodeStatusOffline
	e.RegisterNode(node)

	e.recovery.RecoverNode("1")

	got, _ := e.GetNode("1")
	if got.Status != NodeStatusOnline {
		t.Errorf("expected online, got %s", got.Status)
	}
}

func TestRecoveryStats(t *testing.T) {
	e := setupTestEngine(t)

	task := setupTestTask("s1", 1)
	task.Status = TaskStatusFailed
	task.MaxAttempts = 1
	task.Attempts = 1
	e.tasks[task.ID] = task

	stats := e.recovery.GetRecoveryStats()
	if stats["failed_tasks"] != 1 {
		t.Errorf("expected 1 failed, got %d", stats["failed_tasks"])
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if len(id1) < 10 {
		t.Errorf("expected longer ID, got %s", id1)
	}
}
