package projectcenter

import (
	"testing"
	"time"
)

// ========== StatsManager 测试 ==========

func TestGetProjectStats(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	// 创建任务
	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:    "任务1",
		Priority: PriorityHigh,
	}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:    "任务2",
		Priority: PriorityMedium,
	}, "user1")
	task3, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:    "任务3",
		Priority: PriorityLow,
	}, "user2")

	// 完成任务1
	taskMgr.MoveTask(task1.ID, MoveTaskRequest{Status: TaskStatusDone})

	// 设置过期任务
	pastDate := time.Now().Add(-24 * time.Hour)
	taskMgr.UpdateTask(task3.ID, UpdateTaskRequest{
		DueDate: &pastDate,
	})

	// 创建里程碑
	dueDate := time.Now().Add(30 * 24 * time.Hour)
	ms, _ := milestoneMgr.CreateMilestone("proj1", CreateMilestoneRequest{
		Name:    "里程碑1",
		DueDate: &dueDate,
	})
	milestoneMgr.CompleteMilestone(ms.ID)

	stats, err := statsMgr.GetProjectStats("proj1")
	if err != nil {
		t.Fatalf("GetProjectStats failed: %v", err)
	}

	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", stats.TotalTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.TasksByPriority["high"] != 1 {
		t.Errorf("expected 1 high priority task, got %d", stats.TasksByPriority["high"])
	}
	if stats.MilestoneStats.Total != 1 {
		t.Errorf("expected 1 milestone, got %d", stats.MilestoneStats.Total)
	}
	if stats.MilestoneStats.Completed != 1 {
		t.Errorf("expected 1 completed milestone, got %d", stats.MilestoneStats.Completed)
	}
}

func TestGetMemberWorkloads(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	// 用户1有3个任务
	taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:         "用户1任务1",
		AssigneeID:    "user1",
		EstimateHours: 10,
	}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:         "用户1任务2",
		AssigneeID:    "user1",
		EstimateHours: 8,
	}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:         "用户1任务3",
		AssigneeID:    "user1",
		EstimateHours: 12,
	}, "user1")

	// 用户2有1个任务
	taskMgr.CreateTask("proj1", CreateTaskRequest{
		Title:         "用户2任务1",
		AssigneeID:    "user2",
		EstimateHours: 5,
	}, "user2")

	// 完成用户1的一个任务
	taskMgr.MoveTask(task2.ID, MoveTaskRequest{Status: TaskStatusDone})

	workloads := statsMgr.GetMemberWorkloads("proj1")

	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d", len(workloads))
	}

	for _, wl := range workloads {
		if wl.UserID == "user1" {
			if wl.TotalTasks != 3 {
				t.Errorf("expected user1 to have 3 tasks, got %d", wl.TotalTasks)
			}
			if wl.CompletedTasks != 1 {
				t.Errorf("expected user1 to have 1 completed task, got %d", wl.CompletedTasks)
			}
			if wl.ActiveTasks != 2 {
				t.Errorf("expected user1 to have 2 active tasks, got %d", wl.ActiveTasks)
			}
		}
	}
}

func TestGetStatusDistribution(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	// 创建不同状态的任务
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "待办任务"}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "进行中任务"}, "user1")
	task3, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "完成任务"}, "user1")

	taskMgr.MoveTask(task2.ID, MoveTaskRequest{Status: TaskStatusInProgress})
	taskMgr.MoveTask(task3.ID, MoveTaskRequest{Status: TaskStatusDone})

	dist := statsMgr.GetStatusDistribution("proj1")

	if dist["todo"].Count != 1 {
		t.Errorf("expected 1 todo, got %d", dist["todo"].Count)
	}
	if dist["in_progress"].Count != 1 {
		t.Errorf("expected 1 in_progress, got %d", dist["in_progress"].Count)
	}
	if dist["done"].Count != 1 {
		t.Errorf("expected 1 done, got %d", dist["done"].Count)
	}

	// 验证百分比
	if dist["todo"].Percentage < 33.3 || dist["todo"].Percentage > 33.4 {
		t.Errorf("expected ~33.3%% for todo, got %.1f%%", dist["todo"].Percentage)
	}
}

func TestGetPriorityDistribution(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "高优先级", Priority: PriorityHigh}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "高优先级2", Priority: PriorityHigh}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "中优先级", Priority: PriorityMedium}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "低优先级", Priority: PriorityLow}, "user1")

	dist := statsMgr.GetPriorityDistribution("proj1")

	if dist["high"].Count != 2 {
		t.Errorf("expected 2 high priority, got %d", dist["high"].Count)
	}
	if dist["medium"].Count != 1 {
		t.Errorf("expected 1 medium priority, got %d", dist["medium"].Count)
	}
	if dist["low"].Count != 1 {
		t.Errorf("expected 1 low priority, got %d", dist["low"].Count)
	}
}

func TestGetWeeklyProgress(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	// 创建任务
	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务1"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务2"}, "user1")

	// 完成一个任务
	taskMgr.MoveTask(task1.ID, MoveTaskRequest{Status: TaskStatusDone})

	progress := statsMgr.GetWeeklyProgress("proj1", 4)

	if len(progress) != 4 {
		t.Errorf("expected 4 weekly items, got %d", len(progress))
	}

	// 每个条目应该有正确的结构
	for _, item := range progress {
		if item.WeekStart.After(item.WeekEnd) {
			t.Error("week start should be before week end")
		}
	}
}

func TestGetProjectSummary(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	// 创建任务
	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务1"}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务2"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务3"}, "user1")

	// 设置任务状态
	taskMgr.MoveTask(task1.ID, MoveTaskRequest{Status: TaskStatusDone})
	taskMgr.MoveTask(task2.ID, MoveTaskRequest{Status: TaskStatusInProgress})

	// 创建里程碑
	milestoneMgr.CreateMilestone("proj1", CreateMilestoneRequest{Name: "MS1"})

	summary, err := statsMgr.GetProjectSummary("proj1")
	if err != nil {
		t.Fatalf("GetProjectSummary failed: %v", err)
	}

	if summary.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", summary.TotalTasks)
	}
	if summary.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", summary.CompletedTasks)
	}
	if summary.InProgressTasks != 1 {
		t.Errorf("expected 1 in-progress task, got %d", summary.InProgressTasks)
	}
	if summary.TotalMilestones != 1 {
		t.Errorf("expected 1 milestone, got %d", summary.TotalMilestones)
	}
}

func TestGetBurndownData(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	// 创建任务
	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务1"}, "user1")
	task2, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务2"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务3"}, "user1")

	// 完成任务
	taskMgr.MoveTask(task1.ID, MoveTaskRequest{Status: TaskStatusDone})
	taskMgr.MoveTask(task2.ID, MoveTaskRequest{Status: TaskStatusDone})

	startDate := time.Now().Add(-7 * 24 * time.Hour)
	endDate := time.Now()

	data := statsMgr.GetBurndownData("proj1", startDate, endDate)

	if len(data) == 0 {
		t.Fatal("expected burndown data")
	}

	// 最后一天应该显示已完成2个
	lastDay := data[len(data)-1]
	if lastDay.Completed != 2 {
		t.Errorf("expected 2 completed on last day, got %d", lastDay.Completed)
	}
	if lastDay.Actual != 1 {
		t.Errorf("expected 1 remaining on last day, got %d", lastDay.Actual)
	}
}

func TestStatsEmptyProject(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	stats, err := statsMgr.GetProjectStats("empty_project")
	if err != nil {
		t.Fatalf("GetProjectStats failed: %v", err)
	}

	if stats.TotalTasks != 0 {
		t.Errorf("expected 0 tasks, got %d", stats.TotalTasks)
	}
	if stats.CompletionRate != 0 {
		t.Errorf("expected 0%% completion rate, got %.2f%%", stats.CompletionRate)
	}
}

func TestStatusDistItemLabels(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	task, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务"}, "user1")
	taskMgr.MoveTask(task.ID, MoveTaskRequest{Status: TaskStatusInProgress})

	dist := statsMgr.GetStatusDistribution("proj1")

	if dist["in_progress"].Label != "进行中" {
		t.Errorf("expected label '进行中', got '%s'", dist["in_progress"].Label)
	}
}

func TestPriorityDistItemLabels(t *testing.T) {
	taskMgr := NewTaskManager()
	milestoneMgr := NewMilestoneManager(taskMgr)
	statsMgr := NewStatsManager(taskMgr, milestoneMgr)

	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务", Priority: PriorityUrgent}, "user1")

	dist := statsMgr.GetPriorityDistribution("proj1")

	if dist["urgent"].Label != "紧急" {
		t.Errorf("expected label '紧急', got '%s'", dist["urgent"].Label)
	}
}
