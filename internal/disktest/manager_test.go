package disktest

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestRunBench(t *testing.T) {
	m := NewManager()

	result, err := m.RunBench("/dev/sda", nil)
	if err != nil {
		t.Fatalf("run bench failed: %v", err)
	}
	if result.Device != "/dev/sda" {
		t.Errorf("expected /dev/sda, got '%s'", result.Device)
	}
	if result.SeqRead == 0 {
		t.Error("expected non-zero seq read")
	}
	if result.SeqWrite == 0 {
		t.Error("expected non-zero seq write")
	}
	if result.RandReadIOPS == 0 {
		t.Error("expected non-zero rand read IOPS")
	}
	if result.AvgLatency == 0 {
		t.Error("expected non-zero avg latency")
	}

	// 空设备名
	_, err = m.RunBench("", nil)
	if err == nil {
		t.Error("expected error for empty device")
	}
}

func TestRunBenchWithConfig(t *testing.T) {
	m := NewManager()

	config := &TestConfig{
		BlockSize:  8192,
		TestSize:   512 * 1024 * 1024,
		QueueDepth: 16,
		Duration:   5,
	}

	result, err := m.RunBench("/dev/sdb", config)
	if err != nil {
		t.Fatalf("run bench with config failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestRunSMART(t *testing.T) {
	m := NewManager()

	data, err := m.RunSMART("/dev/sda")
	if err != nil {
		t.Fatalf("run SMART failed: %v", err)
	}
	if data.Device != "/dev/sda" {
		t.Errorf("expected /dev/sda, got '%s'", data.Device)
	}
	if data.Status != "PASSED" {
		t.Errorf("expected PASSED, got '%s'", data.Status)
	}
	if data.HealthScore == 0 {
		t.Error("expected non-zero health score")
	}
	if data.Temp == 0 {
		t.Error("expected non-zero temp")
	}

	// 空设备名
	_, err = m.RunSMART("")
	if err == nil {
		t.Error("expected error for empty device")
	}
}

func TestRunBadBlocks(t *testing.T) {
	m := NewManager()

	task, err := m.RunBadBlocks("/dev/sda")
	if err != nil {
		t.Fatalf("run bad blocks failed: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status != "completed" {
		t.Errorf("expected completed, got '%s'", task.Status)
	}
	if task.Progress != 100 {
		t.Errorf("expected 100 progress, got %.0f", task.Progress)
	}
	if task.Result == nil {
		t.Error("expected non-nil result")
	}

	// 空设备名
	_, err = m.RunBadBlocks("")
	if err == nil {
		t.Error("expected error for empty device")
	}
}

func TestRunLatencyTest(t *testing.T) {
	m := NewManager()

	result, err := m.RunLatencyTest("/dev/sda", 500)
	if err != nil {
		t.Fatalf("run latency test failed: %v", err)
	}
	if result.Device != "/dev/sda" {
		t.Errorf("expected /dev/sda, got '%s'", result.Device)
	}
	if result.Latency == 0 {
		t.Error("expected non-zero latency")
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got '%s'", result.Status)
	}

	// 默认 count
	result, err = m.RunLatencyTest("/dev/sda", 0)
	if err != nil {
		t.Fatalf("run latency test with default count failed: %v", err)
	}

	// 空设备名
	_, err = m.RunLatencyTest("", 100)
	if err == nil {
		t.Error("expected error for empty device")
	}
}

func TestGetTask(t *testing.T) {
	m := NewManager()

	// 先创建任务
	task, _ := m.RunBadBlocks("/dev/sda")

	// 获取任务
	got := m.GetTask(task.ID)
	if got == nil {
		t.Fatal("expected task")
	}
	if got.ID != task.ID {
		t.Errorf("expected task ID '%s', got '%s'", task.ID, got.ID)
	}

	// 不存在的任务
	if m.GetTask("nonexistent") != nil {
		t.Error("expected nil for nonexistent task")
	}
}

func TestListTasks(t *testing.T) {
	m := NewManager()

	// 初始应为空
	tasks := m.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}

	// 创建几个任务
	m.RunBadBlocks("/dev/sda")
	m.RunBadBlocks("/dev/sdb")

	tasks = m.ListTasks()
	if len(tasks) < 2 {
		t.Errorf("expected at least 2 tasks, got %d", len(tasks))
	}
}

func TestCancelTask(t *testing.T) {
	m := NewManager()

	// 创建一个任务
	task, _ := m.RunBadBlocks("/dev/sda")

	// 已完成的任务不能取消
	err := m.CancelTask(task.ID)
	if err == nil {
		t.Error("expected error cancelling completed task")
	}

	// 不存在的任务
	err = m.CancelTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}
