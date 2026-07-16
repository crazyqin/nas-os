package projectcenter

import (
	"fmt"
	"testing"
)

// ========== KanbanManager 测试 ==========

func TestCreateBoard(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	board, err := mgr.CreateBoard("proj1", "开发看板")
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	if board.Name != "开发看板" {
		t.Errorf("expected name '开发看板', got '%s'", board.Name)
	}
	if board.ProjectID != "proj1" {
		t.Errorf("expected project ID 'proj1', got '%s'", board.ProjectID)
	}
	if len(board.Columns) != 4 {
		t.Errorf("expected 4 default columns, got %d", len(board.Columns))
	}
}

func TestCreateBoardEmptyName(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	_, err := mgr.CreateBoard("proj1", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetBoard(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	created, _ := mgr.CreateBoard("proj1", "测试看板")

	fetched, err := mgr.GetBoard(created.ID)
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
}

func TestGetBoardNotFound(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	_, err := mgr.GetBoard("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent board")
	}
}

func TestGetProjectBoard(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	mgr.CreateBoard("proj1", "项目1看板")
	mgr.CreateBoard("proj2", "项目2看板")

	board, err := mgr.GetProjectBoard("proj1")
	if err != nil {
		t.Fatalf("GetProjectBoard failed: %v", err)
	}

	if board.Name != "项目1看板" {
		t.Errorf("expected name '项目1看板', got '%s'", board.Name)
	}
}

func TestDeleteBoard(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	board, _ := mgr.CreateBoard("proj1", "要删除的看板")

	err := mgr.DeleteBoard(board.ID)
	if err != nil {
		t.Fatalf("DeleteBoard failed: %v", err)
	}

	_, err = mgr.GetBoard(board.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestAddColumn(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	board, _ := mgr.CreateBoard("proj1", "测试看板")

	newCol := KanbanColumn{
		Name:     "测试列",
		Status:   TaskStatusBlocked,
		WIPLimit: 3,
	}

	err := mgr.AddColumn(board.ID, newCol)
	if err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	fetched, _ := mgr.GetBoard(board.ID)
	if len(fetched.Columns) != 5 {
		t.Errorf("expected 5 columns, got %d", len(fetched.Columns))
	}
}

func TestRemoveColumn(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	board, _ := mgr.CreateBoard("proj1", "测试看板")

	// 先添加一列
	newCol := KanbanColumn{
		ID:     "custom_col",
		Name:   "自定义列",
		Status: TaskStatusBlocked,
	}
	mgr.AddColumn(board.ID, newCol)

	// 然后删除
	err := mgr.RemoveColumn(board.ID, "custom_col")
	if err != nil {
		t.Fatalf("RemoveColumn failed: %v", err)
	}

	fetched, _ := mgr.GetBoard(board.ID)
	if len(fetched.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(fetched.Columns))
	}
}

func TestSetColumnWIP(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	board, _ := mgr.CreateBoard("proj1", "测试看板")

	err := mgr.SetColumnWIP(board.ID, "col_in_progress", 10)
	if err != nil {
		t.Fatalf("SetColumnWIP failed: %v", err)
	}

	fetched, _ := mgr.GetBoard(board.ID)
	for _, col := range fetched.Columns {
		if col.ID == "col_in_progress" {
			if col.WIPLimit != 10 {
				t.Errorf("expected WIP limit 10, got %d", col.WIPLimit)
			}
		}
	}
}

func TestGetBoardView(t *testing.T) {
	taskMgr := NewTaskManager()
	kanbanMgr := NewKanbanManager(taskMgr)

	board, _ := kanbanMgr.CreateBoard("proj1", "测试看板")

	// 创建不同状态的任务
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "待办任务1"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "待办任务2"}, "user1")

	task3, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "进行中任务"}, "user1")
	taskMgr.MoveTask(task3.ID, MoveTaskRequest{Status: TaskStatusInProgress})

	view, err := kanbanMgr.GetBoardView(board.ID)
	if err != nil {
		t.Fatalf("GetBoardView failed: %v", err)
	}

	todoTasks := view["col_todo"]
	if len(todoTasks) != 2 {
		t.Errorf("expected 2 todo tasks, got %d", len(todoTasks))
	}

	inProgressTasks := view["col_in_progress"]
	if len(inProgressTasks) != 1 {
		t.Errorf("expected 1 in-progress task, got %d", len(inProgressTasks))
	}
}

func TestBoardFilters(t *testing.T) {
	taskMgr := NewTaskManager()
	kanbanMgr := NewKanbanManager(taskMgr)

	board, _ := kanbanMgr.CreateBoard("proj1", "过滤测试看板")

	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "用户1任务", AssigneeID: "user1"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "用户2任务", AssigneeID: "user2"}, "user1")

	// 设置过滤器
	kanbanMgr.UpdateFilters(board.ID, KanbanFilters{
		AssigneeIDs: []string{"user1"},
	})

	view, _ := kanbanMgr.GetBoardView(board.ID)
	todoTasks := view["col_todo"]

	if len(todoTasks) != 1 {
		t.Errorf("expected 1 filtered task, got %d", len(todoTasks))
	}
}

func TestGetBoardStats(t *testing.T) {
	taskMgr := NewTaskManager()
	kanbanMgr := NewKanbanManager(taskMgr)

	board, _ := kanbanMgr.CreateBoard("proj1", "统计测试看板")

	// 创建任务
	task1, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务1"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务2"}, "user1")
	taskMgr.CreateTask("proj1", CreateTaskRequest{Title: "任务3"}, "user1")

	// 完成一个任务
	taskMgr.MoveTask(task1.ID, MoveTaskRequest{Status: TaskStatusDone})

	stats, err := kanbanMgr.GetBoardStats(board.ID)
	if err != nil {
		t.Fatalf("GetBoardStats failed: %v", err)
	}

	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", stats.TotalTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.CompletionRate < 33.3 || stats.CompletionRate > 33.4 {
		t.Errorf("expected completion rate ~33.3%%, got %.1f%%", stats.CompletionRate)
	}
}

func TestCheckWIPLimit(t *testing.T) {
	taskMgr := NewTaskManager()
	kanbanMgr := NewKanbanManager(taskMgr)

	board, _ := kanbanMgr.CreateBoard("proj1", "WIP测试看板")

	// 设置 WIP 限制为 2
	kanbanMgr.SetColumnWIP(board.ID, "col_in_progress", 2)

	// 创建 2 个进行中的任务
	for i := 0; i < 2; i++ {
		task, _ := taskMgr.CreateTask("proj1", CreateTaskRequest{Title: fmt.Sprintf("任务%d", i)}, "user1")
		taskMgr.MoveTask(task.ID, MoveTaskRequest{Status: TaskStatusInProgress})
	}

	exceeded, current, limit, err := kanbanMgr.CheckWIPLimit(board.ID, "col_in_progress")
	if err != nil {
		t.Fatalf("CheckWIPLimit failed: %v", err)
	}

	if !exceeded {
		t.Error("expected WIP limit to be exceeded")
	}
	if current != 2 {
		t.Errorf("expected current count 2, got %d", current)
	}
	if limit != 2 {
		t.Errorf("expected limit 2, got %d", limit)
	}
}

func TestListBoards(t *testing.T) {
	taskMgr := NewTaskManager()
	mgr := NewKanbanManager(taskMgr)

	mgr.CreateBoard("proj1", "看板1")
	mgr.CreateBoard("proj1", "看板2")
	mgr.CreateBoard("proj2", "看板3")

	boards := mgr.ListBoards("proj1")
	if len(boards) != 2 {
		t.Errorf("expected 2 boards for proj1, got %d", len(boards))
	}

	allBoards := mgr.ListBoards("")
	if len(allBoards) != 3 {
		t.Errorf("expected 3 total boards, got %d", len(allBoards))
	}
}
