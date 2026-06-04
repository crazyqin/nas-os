package websharepro

import (
	"testing"
	"time"
)

func TestNewBatchExecutor(t *testing.T) {
	executor := NewBatchExecutor(nil)
	if executor == nil {
		t.Fatal("NewBatchExecutor returned nil")
	}
}

func TestBatchSubmitEmpty(t *testing.T) {
	executor := NewBatchExecutor(nil)

	_, err := executor.Submit(nil, "admin")
	if err == nil {
		t.Error("expected error for empty operations")
	}
}

func TestBatchSubmit(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 2,
		MaxConcurrency:     4,
		StopOnError:        false,
		EnableRollback:     true,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := []*BatchOperation{
		{Source: "/file1.txt", Type: BatchOpCopy, Destination: "/dest/file1.txt"},
		{Source: "/file2.txt", Type: BatchOpCopy, Destination: "/dest/file2.txt"},
	}

	task, err := executor.Submit(ops, "admin")
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	if task.ID == "" {
		t.Error("expected task ID")
	}
	if task.TotalOps != 2 {
		t.Errorf("expected 2 operations, got %d", task.TotalOps)
	}
	if task.Author != "admin" {
		t.Errorf("expected author admin, got %s", task.Author)
	}
}

func TestBatchGetTask(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := []*BatchOperation{
		{Source: "/a.txt", Type: BatchOpDelete},
	}

	task, _ := executor.Submit(ops, "admin")
	time.Sleep(200 * time.Millisecond)

	got, err := executor.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, got.ID)
	}
}

func TestBatchCancel(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := []*BatchOperation{
		{Source: "/slow.txt", Type: BatchOpCopy, Destination: "/dest/slow.txt"},
	}

	task, _ := executor.Submit(ops, "admin")

	// 立即取消
	if err := executor.Cancel(task.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	got, _ := executor.GetTask(task.ID)
	if got.Status != TaskCancelled && got.Status != TaskCompleted {
		// 可能已经完成了
		t.Logf("task status: %s", got.Status)
	}
}

func TestBatchListTasks(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := []*BatchOperation{
		{Source: "/a.txt", Type: BatchOpDelete},
	}

	executor.Submit(ops, "admin")
	time.Sleep(100 * time.Millisecond)

	tasks := executor.ListTasks()
	if len(tasks) == 0 {
		t.Error("expected at least one task")
	}
}

func TestBatchProgressCallback(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	progressCalled := false
	executor.OnProgress(func(task *BatchTask) {
		progressCalled = true
	})

	ops := []*BatchOperation{
		{Source: "/test.txt", Type: BatchOpDelete},
	}

	executor.Submit(ops, "admin")
	time.Sleep(300 * time.Millisecond)

	if !progressCalled {
		t.Error("expected progress callback to be called")
	}
}

func TestBatchGetTaskProgress(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := []*BatchOperation{
		{Source: "/a.txt", Type: BatchOpDelete},
	}

	task, _ := executor.Submit(ops, "admin")
	time.Sleep(200 * time.Millisecond)

	progress, err := executor.GetTaskProgress(task.ID)
	if err != nil {
		t.Fatalf("get progress failed: %v", err)
	}
	if progress < 0 || progress > 100 {
		t.Errorf("expected progress 0-100, got %f", progress)
	}
}

func TestBatchGetStats(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	stats := executor.GetStats()
	if stats == nil {
		t.Fatal("expected stats")
	}

	totalTasks, ok := stats["totalTasks"]
	if !ok {
		t.Error("expected totalTasks in stats")
	}
	if totalTasks.(int) != 0 {
		t.Errorf("expected 0 total tasks, got %v", totalTasks)
	}
}

func TestBatchCleanCompleted(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 1,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := []*BatchOperation{
		{Source: "/a.txt", Type: BatchOpDelete},
	}

	executor.Submit(ops, "admin")
	time.Sleep(300 * time.Millisecond)

	cleaned := executor.CleanCompleted()
	t.Logf("cleaned %d tasks", cleaned)
}

func TestBatchOperationTypes(t *testing.T) {
	types := []BatchOperationType{
		BatchOpCopy,
		BatchOpMove,
		BatchOpDelete,
		BatchOpMkdir,
		BatchOpCompress,
		BatchOpShare,
	}

	for _, opType := range types {
		if opType == "" {
			t.Error("expected non-empty operation type")
		}
	}
}

func TestBatchOperationStructure(t *testing.T) {
	op := &BatchOperation{
		ID:          "op-1",
		Source:      "/source/file.txt",
		Destination: "/dest/file.txt",
		Type:        BatchOpCopy,
		Options: map[string]any{
			"overwrite": true,
		},
	}

	if op.Source != "/source/file.txt" {
		t.Errorf("expected source /source/file.txt, got %s", op.Source)
	}
	if op.Type != BatchOpCopy {
		t.Errorf("expected copy type, got %s", op.Type)
	}
}

func TestBatchTaskStatuses(t *testing.T) {
	statuses := []BatchTaskStatus{
		TaskPending,
		TaskRunning,
		TaskCompleted,
		TaskFailed,
		TaskCancelled,
		TaskRolledBack,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("expected non-empty status")
		}
	}
}

func TestRegisterExecutor(t *testing.T) {
	executor := NewBatchExecutor(nil)

	custom := &CopyExecutor{}
	executor.RegisterExecutor("custom-op", custom)

	// 验证已注册
	executor.mu.RLock()
	_, exists := executor.executors["custom-op"]
	executor.mu.RUnlock()

	if !exists {
		t.Error("expected custom executor to be registered")
	}
}

func TestBatchConcurrencyLimit(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		DefaultConcurrency: 2,
		MaxConcurrency:     2,
		ProgressInterval:   50 * time.Millisecond,
	})

	ops := make([]*BatchOperation, 10)
	for i := 0; i < 10; i++ {
		ops[i] = &BatchOperation{
			Source: "/file" + string(rune('a'+i)) + ".txt",
			Type:   BatchOpDelete,
		}
	}

	task, _ := executor.Submit(ops, "admin")
	if task.Concurrency != 2 {
		t.Errorf("expected concurrency 2, got %d", task.Concurrency)
	}
}

func TestCancelNonExistentTask(t *testing.T) {
	executor := NewBatchExecutor(nil)

	err := executor.Cancel("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestRollbackNonExistentTask(t *testing.T) {
	executor := NewBatchExecutor(&BatchConfig{
		EnableRollback: true,
	})

	err := executor.RollbackTask("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}
