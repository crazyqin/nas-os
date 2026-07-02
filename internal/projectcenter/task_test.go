package projectcenter

import (
	"fmt"
	"testing"
	"time"
)

// ========== TaskManager 测试 ==========

func TestCreateTask(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title:       "测试任务",
		Description: "这是一个测试任务",
		Priority:    PriorityHigh,
		AssigneeID:  "user1",
		Tags:        []string{"test", "dev"},
	}

	task, err := mgr.CreateTask("proj1", req, "reporter1")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.Title != "测试任务" {
		t.Errorf("expected title '测试任务', got '%s'", task.Title)
	}
	if task.Priority != PriorityHigh {
		t.Errorf("expected priority 'high', got '%s'", task.Priority)
	}
	if task.Status != TaskStatusTodo {
		t.Errorf("expected status 'todo', got '%s'", task.Status)
	}
}

func TestCreateTaskEmptyTitle(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title: "",
	}

	_, err := mgr.CreateTask("proj1", req, "reporter1")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestGetTask(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title: "获取任务测试",
	}

	created, _ := mgr.CreateTask("proj1", req, "reporter1")

	fetched, err := mgr.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, fetched.ID)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	mgr := NewTaskManager()

	_, err := mgr.GetTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestUpdateTask(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title: "原始标题",
	}

	created, _ := mgr.CreateTask("proj1", req, "reporter1")

	newTitle := "新标题"
	newDesc := "新描述"
	updateReq := UpdateTaskRequest{
		Title:       newTitle,
		Description: newDesc,
		Status:      TaskStatusInProgress,
		Priority:    PriorityUrgent,
	}

	updated, err := mgr.UpdateTask(created.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	if updated.Title != newTitle {
		t.Errorf("expected title '%s', got '%s'", newTitle, updated.Title)
	}
	if updated.Status != TaskStatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", updated.Status)
	}
}

func TestUpdateTaskComplete(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title: "完成任务测试",
	}

	created, _ := mgr.CreateTask("proj1", req, "reporter1")

	updateReq := UpdateTaskRequest{
		Status: TaskStatusDone,
	}

	updated, err := mgr.UpdateTask(created.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	if updated.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}
}

func TestDeleteTask(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title: "删除任务测试",
	}

	created, _ := mgr.CreateTask("proj1", req, "reporter1")

	err := mgr.DeleteTask(created.ID)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	_, err = mgr.GetTask(created.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestMoveTask(t *testing.T) {
	mgr := NewTaskManager()

	req := CreateTaskRequest{
		Title: "移动任务测试",
	}

	created, _ := mgr.CreateTask("proj1", req, "reporter1")

	moveReq := MoveTaskRequest{
		Status: TaskStatusInProgress,
	}

	moved, err := mgr.MoveTask(created.ID, moveReq)
	if err != nil {
		t.Fatalf("MoveTask failed: %v", err)
	}

	if moved.Status != TaskStatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", moved.Status)
	}
}

func TestListProjectTasks(t *testing.T) {
	mgr := NewTaskManager()

	for i := 0; i < 5; i++ {
		req := CreateTaskRequest{
			Title: fmt.Sprintf("任务 %d", i),
		}
		mgr.CreateTask("proj1", req, "reporter1")
	}

	// 创建其他项目的任务
	otherReq := CreateTaskRequest{
		Title: "其他项目任务",
	}
	mgr.CreateTask("proj2", otherReq, "reporter1")

	tasks, total, err := mgr.ListProjectTasks("proj1", ListOptions{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListProjectTasks failed: %v", err)
	}

	if total != 5 {
		t.Errorf("expected 5 tasks, got %d", total)
	}
	if len(tasks) != 5 {
		t.Errorf("expected 5 tasks in result, got %d", len(tasks))
	}
}

func TestListTasksWithPagination(t *testing.T) {
	mgr := NewTaskManager()

	for i := 0; i < 10; i++ {
		req := CreateTaskRequest{
			Title: fmt.Sprintf("任务 %d", i),
		}
		mgr.CreateTask("proj1", req, "reporter1")
	}

	tasks, total, _ := mgr.ListProjectTasks("proj1", ListOptions{
		Page:     1,
		PageSize: 3,
	})

	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks per page, got %d", len(tasks))
	}
}

func TestListTasksByAssignee(t *testing.T) {
	mgr := NewTaskManager()

	req1 := CreateTaskRequest{
		Title:      "用户1的任务",
		AssigneeID: "user1",
	}
	mgr.CreateTask("proj1", req1, "reporter1")

	req2 := CreateTaskRequest{
		Title:      "用户2的任务",
		AssigneeID: "user2",
	}
	mgr.CreateTask("proj1", req2, "reporter1")

	req3 := CreateTaskRequest{
		Title:      "用户1已完成的任务",
		AssigneeID: "user1",
	}
	created3, _ := mgr.CreateTask("proj1", req3, "reporter1")
	mgr.MoveTask(created3.ID, MoveTaskRequest{Status: TaskStatusDone})

	tasks := mgr.ListTasksByAssignee("user1")
	if len(tasks) != 1 {
		t.Errorf("expected 1 active task for user1, got %d", len(tasks))
	}
}

func TestGetOverdueTasks(t *testing.T) {
	mgr := NewTaskManager()

	pastDate := time.Now().Add(-24 * time.Hour)
	futureDate := time.Now().Add(24 * time.Hour)

	req1 := CreateTaskRequest{
		Title:   "过期任务",
		DueDate: &pastDate,
	}
	mgr.CreateTask("proj1", req1, "reporter1")

	req2 := CreateTaskRequest{
		Title:   "未过期任务",
		DueDate: &futureDate,
	}
	mgr.CreateTask("proj1", req2, "reporter1")

	overdue := mgr.GetOverdueTasks("proj1")
	if len(overdue) != 1 {
		t.Errorf("expected 1 overdue task, got %d", len(overdue))
	}
}

func TestSubtasks(t *testing.T) {
	mgr := NewTaskManager()

	parentReq := CreateTaskRequest{
		Title: "父任务",
	}
	parent, _ := mgr.CreateTask("proj1", parentReq, "reporter1")

	childReq := CreateTaskRequest{
		Title:        "子任务",
		ParentTaskID: parent.ID,
	}
	child, _ := mgr.CreateTask("proj1", childReq, "reporter1")

	subtasks, err := mgr.GetSubtasks(parent.ID)
	if err != nil {
		t.Fatalf("GetSubtasks failed: %v", err)
	}

	if len(subtasks) != 1 {
		t.Errorf("expected 1 subtask, got %d", len(subtasks))
	}
	if subtasks[0].ID != child.ID {
		t.Errorf("expected subtask ID '%s', got '%s'", child.ID, subtasks[0].ID)
	}
}

func TestTaskProgress(t *testing.T) {
	mgr := NewTaskManager()

	parentReq := CreateTaskRequest{
		Title: "父任务",
	}
	parent, _ := mgr.CreateTask("proj1", parentReq, "reporter1")

	// 创建3个子任务
	for i := 0; i < 3; i++ {
		childReq := CreateTaskRequest{
			Title:        fmt.Sprintf("子任务 %d", i),
			ParentTaskID: parent.ID,
		}
		mgr.CreateTask("proj1", childReq, "reporter1")
	}

	// 完成1个子任务
	subtasks, _ := mgr.GetSubtasks(parent.ID)
	mgr.MoveTask(subtasks[0].ID, MoveTaskRequest{Status: TaskStatusDone})

	progress, err := mgr.GetTaskProgress(parent.ID)
	if err != nil {
		t.Fatalf("GetTaskProgress failed: %v", err)
	}

	expectedProgress := 100.0 / 3.0
	if abs(progress-expectedProgress) > 0.01 {
		t.Errorf("expected progress ~%.2f, got %.2f", expectedProgress, progress)
	}
}

func TestComments(t *testing.T) {
	mgr := NewTaskManager()

	taskReq := CreateTaskRequest{
		Title: "评论测试任务",
	}
	task, _ := mgr.CreateTask("proj1", taskReq, "reporter1")

	commentReq := CreateCommentRequest{
		Content: "这是一条评论 @user2 请查看",
	}

	comment, err := mgr.AddComment(task.ID, "user1", commentReq)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	if len(comment.Mentions) != 1 || comment.Mentions[0] != "user2" {
		t.Errorf("expected mentions [user2], got %v", comment.Mentions)
	}

	comments, err := mgr.GetComments(task.ID)
	if err != nil {
		t.Fatalf("GetComments failed: %v", err)
	}

	if len(comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(comments))
	}
}

func TestDependencyBlocked(t *testing.T) {
	mgr := NewTaskManager()

	req1 := CreateTaskRequest{
		Title: "依赖任务",
	}
	dep, _ := mgr.CreateTask("proj1", req1, "reporter1")

	req2 := CreateTaskRequest{
		Title:        "被阻塞的任务",
		Dependencies: []string{dep.ID},
	}
	blocked, _ := mgr.CreateTask("proj1", req2, "reporter1")

	isBlocked, blockedBy, err := mgr.CheckDependencyBlocked(blocked.ID)
	if err != nil {
		t.Fatalf("CheckDependencyBlocked failed: %v", err)
	}

	if !isBlocked {
		t.Error("expected task to be blocked")
	}
	if len(blockedBy) != 1 || blockedBy[0] != dep.ID {
		t.Errorf("expected blocked by [%s], got %v", dep.ID, blockedBy)
	}

	// 完成依赖任务
	mgr.MoveTask(dep.ID, MoveTaskRequest{Status: TaskStatusDone})

	isBlocked, _, _ = mgr.CheckDependencyBlocked(blocked.ID)
	if isBlocked {
		t.Error("expected task to be unblocked after dependency completed")
	}
}

func TestBatchUpdateStatus(t *testing.T) {
	mgr := NewTaskManager()

	var taskIDs []string
	for i := 0; i < 3; i++ {
		req := CreateTaskRequest{
			Title: fmt.Sprintf("批量任务 %d", i),
		}
		task, _ := mgr.CreateTask("proj1", req, "reporter1")
		taskIDs = append(taskIDs, task.ID)
	}

	updated, err := mgr.BatchUpdateStatus(taskIDs, TaskStatusInProgress)
	if err != nil {
		t.Fatalf("BatchUpdateStatus failed: %v", err)
	}

	if updated != 3 {
		t.Errorf("expected 3 updated, got %d", updated)
	}

	for _, id := range taskIDs {
		task, _ := mgr.GetTask(id)
		if task.Status != TaskStatusInProgress {
			t.Errorf("expected task %s status 'in_progress', got '%s'", id, task.Status)
		}
	}
}

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"@user1 你好", []string{"user1"}},
		{"请 @admin 和 @mod 查看", []string{"admin", "mod"}},
		{"没有提及", nil},
		{"@user1,@user2 提及", []string{"user1", "user2"}},
	}

	for _, tt := range tests {
		result := extractMentions(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("extractMentions(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i, m := range result {
			if m != tt.expected[i] {
				t.Errorf("extractMentions(%q)[%d] = %s, want %s", tt.input, i, m, tt.expected[i])
			}
		}
	}
}

// helper.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
