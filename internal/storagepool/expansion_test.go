package storagepool

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestManager 创建测试用的Manager（使用临时目录）
func createTestManager(t *testing.T) *Manager {
	tmpDir := filepath.Join(os.TempDir(), "nas-os-test-storagepool")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	manager, err := NewManager(tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	return manager
}

func TestBtrfsExpansionService_Create(t *testing.T) {
	manager := createTestManager(t)

	service := NewBtrfsExpansionService(manager)
	if service == nil {
		t.Fatal("创建扩容服务失败")
	}

	// 验证初始状态
	tasks := service.ListTasks("")
	if len(tasks) != 0 {
		t.Errorf("初始任务列表应为空，实际: %d", len(tasks))
	}

	service.Close()
}

func TestBtrfsExpansionService_ExpandValidation(t *testing.T) {
	manager := createTestManager(t)

	service := NewBtrfsExpansionService(manager)
	defer service.Close()

	// 测试: 扩展不存在的池
	_, err := service.Expand("nonexistent", []string{}, ExpansionOptions{})
	if err == nil {
		t.Error("扩展不存在的池应返回错误")
	}
}

func TestExpansionTask_Status(t *testing.T) {
	task := &ExpansionTask{
		ID:        "test_task",
		PoolID:    "pool1",
		Status:    TaskStatusPending,
		StartTime: time.Now(),
	}

	// 验证任务字段
	if task.ID != "test_task" {
		t.Errorf("任务ID不匹配")
	}

	if task.Status != TaskStatusPending {
		t.Errorf("任务初始状态应为pending")
	}

	if task.Progress != 0 {
		t.Errorf("任务初始进度应为0")
	}
}

func TestExpansionOptions_Defaults(t *testing.T) {
	opts := ExpansionOptions{}

	// 验证默认值
	if opts.Mode != "" {
		t.Errorf("默认模式应为空")
	}

	if opts.AutoBalance {
		t.Errorf("默认不应自动平衡")
	}
}

func TestTaskStatus_String(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Errorf("任务状态不应为空字符串")
		}
	}
}

func TestExpansionMode_String(t *testing.T) {
	modes := []ExpansionMode{
		ExpansionModeSingle,
		ExpansionModeMulti,
	}

	for _, mode := range modes {
		if mode == "" {
			t.Errorf("扩容模式不应为空字符串")
		}
	}
}

func TestBtrfsExpansionService_GetTaskProgress_NonExistent(t *testing.T) {
	manager := createTestManager(t)

	service := NewBtrfsExpansionService(manager)
	defer service.Close()

	// 查询不存在的任务
	_, err := service.GetTaskProgress("nonexistent")
	if err == nil {
		t.Error("查询不存在的任务应返回错误")
	}
}

func TestBtrfsExpansionService_CancelTask_NonExistent(t *testing.T) {
	manager := createTestManager(t)

	service := NewBtrfsExpansionService(manager)
	defer service.Close()

	// 取消不存在的任务
	err := service.CancelTask("nonexistent")
	if err == nil {
		t.Error("取消不存在的任务应返回错误")
	}
}

func TestBtrfsExpansionService_ListTasks_Empty(t *testing.T) {
	manager := createTestManager(t)

	service := NewBtrfsExpansionService(manager)
	defer service.Close()

	// 列出空任务列表
	tasks := service.ListTasks("")
	if tasks == nil {
		tasks = []ExpansionTask{} // 初始化为空切片
	}

	if len(tasks) != 0 {
		t.Errorf("空任务列表长度应为0，实际: %d", len(tasks))
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()

	// 验证ID不为空
	if id1 == "" {
		t.Error("任务ID不应为空")
	}

	// 验证ID唯一性
	if id1 == id2 {
		t.Error("任务ID应唯一")
	}

	// 验证ID前缀
	if len(id1) < 6 {
		t.Error("任务ID长度应大于6")
	}
}