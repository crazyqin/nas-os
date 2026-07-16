// Package usbbackup 测试
package usbbackup

import (
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	detector := NewDetector(10 * time.Second)
	config := DefaultConfig()
	config.AutoDetect = false
	return NewManager(config, detector)
}

func TestNewManager(t *testing.T) {
	mgr := newTestManager(t)
	if mgr == nil {
		t.Fatal("管理器不应为nil")
	}
}

func TestCreateTask(t *testing.T) {
	mgr := newTestManager(t)

	task, err := mgr.CreateTask(&CreateTaskRequest{
		Name:       "测试备份",
		Direction:  DirectionNasToUSB,
		Policy:     PolicyManual,
		SourcePath: "/mnt/data",
		DestPath:   "/mnt/usb",
	})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if task.Name != "测试备份" {
		t.Errorf("名称不匹配: %s", task.Name)
	}
}

func TestListTasks(t *testing.T) {
	mgr := newTestManager(t)

	mgr.CreateTask(&CreateTaskRequest{
		Name: "task1", Direction: DirectionNasToUSB,
		Policy: PolicyManual, SourcePath: "/src1", DestPath: "/dst1",
	})
	mgr.CreateTask(&CreateTaskRequest{
		Name: "task2", Direction: DirectionNasToUSB,
		Policy: PolicyManual, SourcePath: "/src2", DestPath: "/dst2",
	})

	tasks := mgr.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("期望2个任务，实际 %d", len(tasks))
	}
}

func TestGetTask(t *testing.T) {
	mgr := newTestManager(t)

	task, _ := mgr.CreateTask(&CreateTaskRequest{
		Name: "test", Direction: DirectionNasToUSB,
		Policy: PolicyManual, SourcePath: "/src", DestPath: "/dst",
	})

	got, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("ID不匹配")
	}
}

func TestDeleteTask(t *testing.T) {
	mgr := newTestManager(t)

	task, _ := mgr.CreateTask(&CreateTaskRequest{
		Name: "to-delete", Direction: DirectionNasToUSB,
		Policy: PolicyManual, SourcePath: "/src", DestPath: "/dst",
	})

	err := mgr.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}

	_, err = mgr.GetTask(task.ID)
	if err == nil {
		t.Error("已删除任务不应存在")
	}
}

func TestGetTaskNotFound(t *testing.T) {
	mgr := newTestManager(t)

	_, err := mgr.GetTask("nonexistent")
	if err == nil {
		t.Error("不存在的任务应返回错误")
	}
}

func TestBackupDirectionConstants(t *testing.T) {
	directions := []BackupDirection{
		DirectionNasToUSB,
		DirectionUSBToNAS,
		DirectionBidirectional,
	}
	for _, d := range directions {
		if d == "" {
			t.Error("方向常量不应为空")
		}
	}
}

func TestBackupPolicyConstants(t *testing.T) {
	policies := []BackupPolicy{
		PolicyOnInsert,
		PolicyScheduled,
		PolicyManual,
	}
	for _, p := range policies {
		if p == "" {
			t.Error("策略常量不应为空")
		}
	}
}
