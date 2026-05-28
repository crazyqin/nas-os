package teamworkspace

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	wsList := m.ListWorkspaces()
	if len(wsList) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(wsList))
	}
}

func TestCreateAndGetWorkspace(t *testing.T) {
	m := NewManager()

	err := m.CreateWorkspace(&Workspace{
		ID: "ws-1", Name: "研发团队", Description: "产品研发", Owner: "user1",
	})
	if err != nil {
		t.Fatalf("create workspace failed: %v", err)
	}

	ws := m.GetWorkspace("ws-1")
	if ws == nil {
		t.Fatal("expected workspace")
	}
	if ws.Name != "研发团队" {
		t.Errorf("expected '研发团队', got '%s'", ws.Name)
	}
	if ws.MemberCount != 1 {
		t.Errorf("expected 1 member, got %d", ws.MemberCount)
	}

	// 重复创建
	err = m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "重复"})
	if err == nil {
		t.Error("expected error for duplicate workspace")
	}

	// 空 ID
	err = m.CreateWorkspace(&Workspace{Name: "空"})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestDeleteWorkspace(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "测试", Owner: "user1"})

	err := m.DeleteWorkspace("ws-1")
	if err != nil {
		t.Fatalf("delete workspace failed: %v", err)
	}

	if m.GetWorkspace("ws-1") != nil {
		t.Error("expected nil after delete")
	}

	err = m.DeleteWorkspace("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestMemberManagement(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "团队", Owner: "user1"})

	// 添加成员
	err := m.AddMember("ws-1", "user2", "member")
	if err != nil {
		t.Fatalf("add member failed: %v", err)
	}

	err = m.AddMember("ws-1", "user3", "guest")
	if err != nil {
		t.Fatalf("add member failed: %v", err)
	}

	ws := m.GetWorkspace("ws-1")
	if ws.MemberCount != 3 {
		t.Errorf("expected 3 members, got %d", ws.MemberCount)
	}

	// 重复添加
	err = m.AddMember("ws-1", "user2", "member")
	if err == nil {
		t.Error("expected error for duplicate member")
	}

	// 无效角色
	err = m.AddMember("ws-1", "user4", "superadmin")
	if err == nil {
		t.Error("expected error for invalid role")
	}

	// 不存在的工作区
	err = m.AddMember("nonexistent", "user5", "member")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}

	// 移除成员
	err = m.RemoveMember("ws-1", "user3")
	if err != nil {
		t.Fatalf("remove member failed: %v", err)
	}
	if ws.MemberCount != 2 {
		t.Errorf("expected 2 members, got %d", ws.MemberCount)
	}

	// 移除不存在的成员
	err = m.RemoveMember("ws-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent member")
	}
}

func TestBoardAndTask(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "团队", Owner: "user1"})

	// 获取默认看板
	board := m.GetBoard("ws-1", "ws-1-board-default")
	if board == nil {
		t.Fatal("expected default board")
	}
	if len(board.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(board.Columns))
	}

	// 创建新看板
	err := m.CreateBoard("ws-1", &Board{ID: "board-2", Name: "Bug 跟踪", Columns: []string{"新建", "修复中", "已验证"}})
	if err != nil {
		t.Fatalf("create board failed: %v", err)
	}

	board2 := m.GetBoard("ws-1", "board-2")
	if board2 == nil || board2.Name != "Bug 跟踪" {
		t.Error("expected board 'Bug 跟踪'")
	}

	// 不存在的看板
	if m.GetBoard("ws-1", "nonexistent") != nil {
		t.Error("expected nil for nonexistent board")
	}

	// 创建任务
	err = m.CreateTask("ws-1", "ws-1-board-default", &Task{
		ID: "task-1", Title: "实现登录功能", Priority: PriorityHigh,
		ColumnID: "待办", Assignee: "user1", Tags: []string{"feature"},
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	// 任务移动
	err = m.MoveTask("ws-1", "task-1", "进行中")
	if err != nil {
		t.Fatalf("move task failed: %v", err)
	}

	// 任务分配
	err = m.AssignTask("task-1", "user2")
	if err != nil {
		t.Fatalf("assign task failed: %v", err)
	}

	// 不存在的任务
	err = m.MoveTask("ws-1", "nonexistent", "已完成")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestGetTasksWithFilters(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "团队", Owner: "user1"})

	m.CreateTask("ws-1", "b1", &Task{ID: "t1", Title: "任务A", Priority: PriorityHigh, ColumnID: "待办", Assignee: "user1", Tags: []string{"feature"}})
	m.CreateTask("ws-1", "b1", &Task{ID: "t2", Title: "任务B", Priority: PriorityLow, ColumnID: "进行中", Assignee: "user2", Tags: []string{"bug"}})
	m.CreateTask("ws-1", "b1", &Task{ID: "t3", Title: "任务C", Priority: PriorityHigh, ColumnID: "待办", Assignee: "user1", Tags: []string{"feature"}})

	// 按列过滤
	tasks := m.GetTasks("ws-1", map[string]string{"column": "待办"})
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks in 待办, got %d", len(tasks))
	}

	// 按指派人过滤
	tasks = m.GetTasks("ws-1", map[string]string{"assignee": "user2"})
	if len(tasks) != 1 {
		t.Errorf("expected 1 task for user2, got %d", len(tasks))
	}

	// 按优先级过滤
	tasks = m.GetTasks("ws-1", map[string]string{"priority": "high"})
	if len(tasks) != 2 {
		t.Errorf("expected 2 high priority tasks, got %d", len(tasks))
	}

	// 按标签过滤
	tasks = m.GetTasks("ws-1", map[string]string{"tag": "bug"})
	if len(tasks) != 1 {
		t.Errorf("expected 1 bug task, got %d", len(tasks))
	}

	// 无过滤
	tasks = m.GetTasks("ws-1", nil)
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestCalendarEvents(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "团队", Owner: "user1"})

	now := time.Now()
	err := m.AddEvent("ws-1", &CalendarEvent{
		ID: "evt-1", Title: "周会",
		StartTime: now, EndTime: now.Add(time.Hour),
		Participants: []string{"user1", "user2"},
	})
	if err != nil {
		t.Fatalf("add event failed: %v", err)
	}

	err = m.AddEvent("ws-1", &CalendarEvent{
		ID: "evt-2", Title: "代码评审",
		StartTime: now.Add(2 * time.Hour), EndTime: now.Add(3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("add event failed: %v", err)
	}

	// 查询时间范围
	events := m.GetEvents("ws-1", now.Add(-time.Hour), now.Add(90*time.Minute))
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	// 查询更大范围
	events = m.GetEvents("ws-1", now.Add(-time.Hour), now.Add(4*time.Hour))
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	// 不存在的工作区
	err = m.AddEvent("nonexistent", &CalendarEvent{Title: "x"})
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestShareFile(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "团队", Owner: "user1"})

	err := m.ShareFile("ws-1", &SharedFile{
		ID: "file-1", FileName: "设计稿.pdf", Size: 1024 * 1024,
		Uploader: "user1", Scope: "team",
	})
	if err != nil {
		t.Fatalf("share file failed: %v", err)
	}

	err = m.ShareFile("ws-1", &SharedFile{
		FileName: "需求文档.docx", Size: 512 * 1024,
		Uploader: "user2", Scope: "public",
	})
	if err != nil {
		t.Fatalf("share file failed: %v", err)
	}

	// 不存在的工作区
	err = m.ShareFile("nonexistent", &SharedFile{FileName: "x"})
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestActivities(t *testing.T) {
	m := NewManager()

	m.CreateWorkspace(&Workspace{ID: "ws-1", Name: "团队", Owner: "user1"})
	m.AddMember("ws-1", "user2", "member")
	m.CreateTask("ws-1", "b1", &Task{ID: "t1", Title: "任务", ColumnID: "待办"})
	m.MoveTask("ws-1", "t1", "进行中")
	m.ShareFile("ws-1", &SharedFile{FileName: "file.txt", Uploader: "user1"})

	activities := m.GetActivities("ws-1", 0)
	if len(activities) < 4 {
		t.Errorf("expected at least 4 activities, got %d", len(activities))
	}

	// 限制数量
	activities = m.GetActivities("ws-1", 2)
	if len(activities) != 2 {
		t.Errorf("expected 2 activities, got %d", len(activities))
	}

	// 空工作区
	activities = m.GetActivities("nonexistent", 0)
	if len(activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(activities))
	}
}
