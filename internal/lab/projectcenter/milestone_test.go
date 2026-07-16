package projectcenter

import (
	"testing"
	"time"
)

// ========== MilestoneManager 测试 ==========

func TestCreateMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	dueDate := time.Now().Add(30 * 24 * time.Hour)
	req := CreateMilestoneRequest{
		Name:        "里程碑1",
		Description: "第一个里程碑",
		DueDate:     &dueDate,
	}

	ms, err := mgr.CreateMilestone("proj1", req)
	if err != nil {
		t.Fatalf("CreateMilestone failed: %v", err)
	}

	if ms.Name != "里程碑1" {
		t.Errorf("expected name '里程碑1', got '%s'", ms.Name)
	}
	if ms.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", ms.Status)
	}
}

func TestCreateMilestoneEmptyName(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	req := CreateMilestoneRequest{
		Name: "",
	}

	_, err := mgr.CreateMilestone("proj1", req)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	created, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "测试里程碑",
	})

	fetched, err := mgr.GetMilestone(created.ID)
	if err != nil {
		t.Fatalf("GetMilestone failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
}

func TestUpdateMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	created, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "原始名称",
	})

	newDueDate := time.Now().Add(60 * 24 * time.Hour)
	updated, err := mgr.UpdateMilestone(created.ID, "新名称", "新描述", &newDueDate)
	if err != nil {
		t.Fatalf("UpdateMilestone failed: %v", err)
	}

	if updated.Name != "新名称" {
		t.Errorf("expected name '新名称', got '%s'", updated.Name)
	}
	if updated.Description != "新描述" {
		t.Errorf("expected description '新描述', got '%s'", updated.Description)
	}
}

func TestDeleteMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	created, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "要删除的里程碑",
	})

	err := mgr.DeleteMilestone(created.ID)
	if err != nil {
		t.Fatalf("DeleteMilestone failed: %v", err)
	}

	_, err = mgr.GetMilestone(created.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestAddTaskToMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	ms, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "里程碑",
	})

	task, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title: "里程碑任务",
	}, "user1")

	err := mgr.AddTaskToMilestone(ms.ID, task.ID)
	if err != nil {
		t.Fatalf("AddTaskToMilestone failed: %v", err)
	}

	// 不会重复添加
	err = mgr.AddTaskToMilestone(ms.ID, task.ID)
	if err != nil {
		t.Fatalf("AddTaskToMilestone should not error on duplicate: %v", err)
	}

	fetched, _ := mgr.GetMilestone(ms.ID)
	if len(fetched.TaskIDs) != 1 {
		t.Errorf("expected 1 task in milestone, got %d", len(fetched.TaskIDs))
	}
}

func TestRemoveTaskFromMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	ms, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "里程碑",
	})

	task, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title: "里程碑任务",
	}, "user1")

	mgr.AddTaskToMilestone(ms.ID, task.ID)

	err := mgr.RemoveTaskFromMilestone(ms.ID, task.ID)
	if err != nil {
		t.Fatalf("RemoveTaskFromMilestone failed: %v", err)
	}

	fetched, _ := mgr.GetMilestone(ms.ID)
	if len(fetched.TaskIDs) != 0 {
		t.Errorf("expected 0 tasks in milestone, got %d", len(fetched.TaskIDs))
	}
}

func TestCompleteMilestone(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	ms, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "里程碑",
	})

	completed, err := mgr.CompleteMilestone(ms.ID)
	if err != nil {
		t.Fatalf("CompleteMilestone failed: %v", err)
	}

	if completed.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", completed.Status)
	}
	if completed.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if completed.Progress != 100 {
		t.Errorf("expected progress 100, got %.2f", completed.Progress)
	}
}

func TestMilestoneProgress(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	ms, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "进度测试里程碑",
	})

	// 添加3个任务
	var tasks []*Task
	for i := 0; i < 3; i++ {
		task, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
			Title: "里程碑任务",
		}, "user1")
		mgr.AddTaskToMilestone(ms.ID, task.ID)
		tasks = append(tasks, task)
	}

	// 完成1个
	taskMgr.MoveTask(tasks[0].ID, MoveTaskRequest{Status: TaskStatusDone})

	progress, err := mgr.RefreshProgress(ms.ID)
	if err != nil {
		t.Fatalf("RefreshProgress failed: %v", err)
	}

	expected := 100.0 / 3.0
	if abs(progress-expected) > 0.01 {
		t.Errorf("expected progress ~%.2f, got %.2f", expected, progress)
	}

	fetched, _ := mgr.GetMilestone(ms.ID)
	if fetched.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", fetched.Status)
	}
}

func TestListProjectMilestones(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	due1 := time.Now().Add(10 * 24 * time.Hour)
	due2 := time.Now().Add(5 * 24 * time.Hour)
	due3 := time.Now().Add(20 * 24 * time.Hour)

	mgr.CreateMilestone("proj1", CreateMilestoneRequest{Name: "MS1", DueDate: &due1})
	mgr.CreateMilestone("proj1", CreateMilestoneRequest{Name: "MS2", DueDate: &due2})
	mgr.CreateMilestone("proj2", CreateMilestoneRequest{Name: "MS3", DueDate: &due3})

	milestones := mgr.ListProjectMilestones("proj1")
	if len(milestones) != 2 {
		t.Errorf("expected 2 milestones for proj1, got %d", len(milestones))
	}

	// 应该按 DueDate 排序
	if milestones[0].Name != "MS2" {
		t.Errorf("expected first milestone 'MS2', got '%s'", milestones[0].Name)
	}
}

func TestGetMilestoneTasks(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	ms, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name: "里程碑",
	})

	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务1"}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务2"}, "user1")

	mgr.AddTaskToMilestone(ms.ID, task1.ID)
	mgr.AddTaskToMilestone(ms.ID, task2.ID)

	tasks, err := mgr.GetMilestoneTasks(ms.ID)
	if err != nil {
		t.Fatalf("GetMilestoneTasks failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestGetMilestoneProgressDetails(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	dueDate := time.Now().Add(7 * 24 * time.Hour)
	ms, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name:    "进度详情里程碑",
		DueDate: &dueDate,
	})

	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务1"}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务2"}, "user1")
	task3, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务3"}, "user1")

	mgr.AddTaskToMilestone(ms.ID, task1.ID)
	mgr.AddTaskToMilestone(ms.ID, task2.ID)
	mgr.AddTaskToMilestone(ms.ID, task3.ID)

	taskMgr.MoveTask(task1.ID, MoveTaskRequest{Status: TaskStatusInProgress})
	taskMgr.MoveTask(task2.ID, MoveTaskRequest{Status: TaskStatusDone})

	progress, err := mgr.GetMilestoneProgress(ms.ID)
	if err != nil {
		t.Fatalf("GetMilestoneProgress failed: %v", err)
	}

	if progress.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", progress.TotalTasks)
	}
	if progress.TasksByStatus["in_progress"] != 1 {
		t.Errorf("expected 1 in_progress task, got %d", progress.TasksByStatus["in_progress"])
	}
	if progress.TasksByStatus["done"] != 1 {
		t.Errorf("expected 1 done task, got %d", progress.TasksByStatus["done"])
	}
}

func TestMilestoneTimeline(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewMilestoneManager(taskMgr)

	start1 := time.Now().Add(-5 * 24 * time.Hour)
	start2 := time.Now().Add(-10 * 24 * time.Hour)

	ms1, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{Name: "MS1"})
	ms2, _ := mgr.CreateMilestone("proj1", CreateMilestoneRequest{Name: "MS2"})

	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:     "任务1",
		StartDate: &start1,
	}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:     "任务2",
		StartDate: &start2,
	}, "user1")

	mgr.AddTaskToMilestone(ms1.ID, task1.ID)
	mgr.AddTaskToMilestone(ms2.ID, task2.ID)

	timeline := mgr.GetTimeline("proj1")
	if len(timeline) != 2 {
		t.Errorf("expected 2 timeline items, got %d", len(timeline))
	}

	// 应该按开始日期排序，MS2 更早
	if timeline[0].Name != "MS2" {
		t.Errorf("expected first item 'MS2', got '%s'", timeline[0].Name)
	}
}
