package projecthub

import (
	"testing"
	"time"
)

func TestNewProjectHub(t *testing.T) {
	config := Config{DataPath: "/tmp/test"}
	hub := New(config)
	if hub == nil {
		t.Fatal("New() returned nil")
	}
	if hub.started {
		t.Error("hub should not be started initially")
	}
}

func TestStartStop(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})

	// 测试启动
	if err := hub.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if !hub.started {
		t.Error("hub should be started after Start()")
	}

	// 测试重复启动
	if err := hub.Start(); err == nil {
		t.Error("Start() should fail when already started")
	}

	// 测试停止
	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if hub.started {
		t.Error("hub should not be started after Stop()")
	}

	// 测试重复停止
	if err := hub.Stop(); err == nil {
		t.Error("Stop() should fail when not started")
	}
}

func TestCreateProject(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 测试创建项目
	project := Project{
		ID:          "proj-1",
		Name:        "测试项目",
		Description: "这是一个测试项目",
	}

	created, err := hub.CreateProject(project)
	if err != nil {
		t.Fatalf("CreateProject() failed: %v", err)
	}
	if created.ID != "proj-1" {
		t.Errorf("expected ID proj-1, got %s", created.ID)
	}
	if created.Status != "active" {
		t.Errorf("expected status active, got %s", created.Status)
	}

	// 测试创建重复项目
	_, err = hub.CreateProject(project)
	if err == nil {
		t.Error("CreateProject() should fail for duplicate ID")
	}

	// 测试缺少ID
	_, err = hub.CreateProject(Project{Name: "无ID项目"})
	if err == nil {
		t.Error("CreateProject() should fail without ID")
	}

	// 测试缺少Name
	_, err = hub.CreateProject(Project{ID: "proj-2"})
	if err == nil {
		t.Error("CreateProject() should fail without Name")
	}
}

func TestGetProject(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})

	// 测试获取项目
	project, err := hub.GetProject("proj-1")
	if err != nil {
		t.Fatalf("GetProject() failed: %v", err)
	}
	if project.Name != "项目1" {
		t.Errorf("expected name 项目1, got %s", project.Name)
	}

	// 测试获取不存在的项目
	_, err = hub.GetProject("nonexistent")
	if err == nil {
		t.Error("GetProject() should fail for nonexistent project")
	}
}

func TestListProjects(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 测试空列表
	projects := hub.ListProjects()
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}

	// 创建多个项目
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.CreateProject(Project{ID: "proj-2", Name: "项目2"})
	hub.CreateProject(Project{ID: "proj-3", Name: "项目3"})

	// 测试列表
	projects = hub.ListProjects()
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}
}

func TestCreateTask(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})

	// 测试创建任务
	deadline := time.Now().AddDate(0, 0, 7)
	task := Task{
		ID:          "task-1",
		Title:       "完成设计",
		Description: "完成UI设计",
		Priority:    TaskPriorityHigh,
		Deadline:    &deadline,
	}

	created, err := hub.CreateTask("proj-1", task)
	if err != nil {
		t.Fatalf("CreateTask() failed: %v", err)
	}
	if created.Status != TaskStatusTodo {
		t.Errorf("expected status todo, got %s", created.Status)
	}
	if created.Priority != TaskPriorityHigh {
		t.Errorf("expected priority high, got %s", created.Priority)
	}

	// 测试创建重复任务
	_, err = hub.CreateTask("proj-1", task)
	if err == nil {
		t.Error("CreateTask() should fail for duplicate ID")
	}

	// 测试在不存在的项目中创建任务
	_, err = hub.CreateTask("nonexistent", Task{ID: "task-2", Title: "任务2"})
	if err == nil {
		t.Error("CreateTask() should fail for nonexistent project")
	}

	// 测试缺少ID
	_, err = hub.CreateTask("proj-1", Task{Title: "无ID任务"})
	if err == nil {
		t.Error("CreateTask() should fail without ID")
	}

	// 测试缺少Title
	_, err = hub.CreateTask("proj-1", Task{ID: "task-3"})
	if err == nil {
		t.Error("CreateTask() should fail without Title")
	}
}

func TestUpdateTask(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目和任务
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.CreateTask("proj-1", Task{ID: "task-1", Title: "原始任务"})

	// 测试更新任务
	updates := map[string]interface{}{
		"title":       "更新后的任务",
		"description": "新描述",
		"priority":    TaskPriorityCritical,
		"status":      TaskStatusInProgress,
	}

	updated, err := hub.UpdateTask("task-1", updates)
	if err != nil {
		t.Fatalf("UpdateTask() failed: %v", err)
	}
	if updated.Title != "更新后的任务" {
		t.Errorf("expected title 更新后的任务, got %s", updated.Title)
	}
	if updated.Priority != TaskPriorityCritical {
		t.Errorf("expected priority critical, got %s", updated.Priority)
	}
	if updated.Status != TaskStatusInProgress {
		t.Errorf("expected status in_progress, got %s", updated.Status)
	}

	// 测试更新为完成状态
	updates = map[string]interface{}{
		"status": TaskStatusDone,
	}
	updated, err = hub.UpdateTask("task-1", updates)
	if err != nil {
		t.Fatalf("UpdateTask() failed: %v", err)
	}
	if updated.CompletedAt == nil {
		t.Error("CompletedAt should be set when status is done")
	}

	// 测试更新不存在的任务
	_, err = hub.UpdateTask("nonexistent", updates)
	if err == nil {
		t.Error("UpdateTask() should fail for nonexistent task")
	}
}

func TestAssignTask(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目、任务和成员
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.CreateTask("proj-1", Task{ID: "task-1", Title: "任务1"})
	hub.AddTeamMember(TeamMember{ID: "member-1", Name: "张三"})

	// 测试分配任务
	if err := hub.AssignTask("task-1", "member-1"); err != nil {
		t.Fatalf("AssignTask() failed: %v", err)
	}

	// 验证分配
	task, _ := hub.UpdateTask("task-1", map[string]interface{}{})
	if task.AssigneeID != "member-1" {
		t.Errorf("expected assignee member-1, got %s", task.AssigneeID)
	}

	// 测试分配不存在的任务
	if err := hub.AssignTask("nonexistent", "member-1"); err == nil {
		t.Error("AssignTask() should fail for nonexistent task")
	}

	// 测试分配给不存在的成员
	if err := hub.AssignTask("task-1", "nonexistent"); err == nil {
		t.Error("AssignTask() should fail for nonexistent member")
	}
}

func TestAddMilestone(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})

	// 测试添加里程碑
	dueDate := time.Now().AddDate(0, 1, 0)
	milestone := Milestone{
		ID:          "ms-1",
		Name:        "第一阶段",
		Description: "完成核心功能",
		DueDate:     &dueDate,
	}

	created, err := hub.AddMilestone("proj-1", milestone)
	if err != nil {
		t.Fatalf("AddMilestone() failed: %v", err)
	}
	if created.Name != "第一阶段" {
		t.Errorf("expected name 第一阶段, got %s", created.Name)
	}

	// 测试添加重复里程碑
	_, err = hub.AddMilestone("proj-1", milestone)
	if err == nil {
		t.Error("AddMilestone() should fail for duplicate ID")
	}

	// 测试在不存在的项目中添加里程碑
	_, err = hub.AddMilestone("nonexistent", Milestone{ID: "ms-2", Name: "里程碑2"})
	if err == nil {
		t.Error("AddMilestone() should fail for nonexistent project")
	}

	// 测试缺少ID
	_, err = hub.AddMilestone("proj-1", Milestone{Name: "无ID里程碑"})
	if err == nil {
		t.Error("AddMilestone() should fail without ID")
	}

	// 测试缺少Name
	_, err = hub.AddMilestone("proj-1", Milestone{ID: "ms-3"})
	if err == nil {
		t.Error("AddMilestone() should fail without Name")
	}
}

func TestLogTime(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目和任务
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.CreateTask("proj-1", Task{ID: "task-1", Title: "任务1"})
	hub.AddTeamMember(TeamMember{ID: "member-1", Name: "张三"})

	// 测试记录工时
	entry := TimeEntry{
		ID:          "entry-1",
		MemberID:    "member-1",
		Hours:       4.5,
		Description: "开发前端",
	}

	created, err := hub.LogTime("task-1", entry)
	if err != nil {
		t.Fatalf("LogTime() failed: %v", err)
	}
	if created.Hours != 4.5 {
		t.Errorf("expected hours 4.5, got %f", created.Hours)
	}

	// 测试缺少ID
	_, err = hub.LogTime("task-1", TimeEntry{MemberID: "member-1", Hours: 2})
	if err == nil {
		t.Error("LogTime() should fail without ID")
	}

	// 测试缺少MemberID
	_, err = hub.LogTime("task-1", TimeEntry{ID: "entry-2", Hours: 2})
	if err == nil {
		t.Error("LogTime() should fail without MemberID")
	}

	// 测试负数工时
	_, err = hub.LogTime("task-1", TimeEntry{ID: "entry-3", MemberID: "member-1", Hours: -1})
	if err == nil {
		t.Error("LogTime() should fail for negative hours")
	}

	// 测试在不存在的任务上记录工时
	_, err = hub.LogTime("nonexistent", TimeEntry{ID: "entry-4", MemberID: "member-1", Hours: 2})
	if err == nil {
		t.Error("LogTime() should fail for nonexistent task")
	}
}

func TestGetProjectStats(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目和任务
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.AddTeamMember(TeamMember{ID: "member-1", Name: "张三"})

	// 创建已完成的任务
	hub.CreateTask("proj-1", Task{ID: "task-1", Title: "任务1"})
	hub.UpdateTask("task-1", map[string]interface{}{"status": TaskStatusDone})

	// 创建进行中的任务
	hub.CreateTask("proj-1", Task{ID: "task-2", Title: "任务2"})

	// 创建逾期任务
	deadline := time.Now().AddDate(0, 0, -1) // 昨天
	hub.CreateTask("proj-1", Task{ID: "task-3", Title: "任务3", Deadline: &deadline})

	// 记录工时
	hub.LogTime("task-1", TimeEntry{ID: "entry-1", MemberID: "member-1", Hours: 8})
	hub.LogTime("task-2", TimeEntry{ID: "entry-2", MemberID: "member-1", Hours: 4})

	// 获取统计
	stats, err := hub.GetProjectStats("proj-1")
	if err != nil {
		t.Fatalf("GetProjectStats() failed: %v", err)
	}

	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", stats.TotalTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.OverdueTasks != 1 {
		t.Errorf("expected 1 overdue task, got %d", stats.OverdueTasks)
	}
	if stats.TotalHours != 12 {
		t.Errorf("expected 12 total hours, got %f", stats.TotalHours)
	}
	if stats.Progress < 33.33 || stats.Progress > 33.34 {
		t.Errorf("expected progress ~33.33, got %f", stats.Progress)
	}

	// 测试不存在的项目
	_, err = hub.GetProjectStats("nonexistent")
	if err == nil {
		t.Error("GetProjectStats() should fail for nonexistent project")
	}
}

func TestGetMemberWorkload(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目、任务和成员
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.AddTeamMember(TeamMember{ID: "member-1", Name: "张三"})

	// 创建任务并分配
	hub.CreateTask("proj-1", Task{ID: "task-1", Title: "任务1"})
	hub.AssignTask("task-1", "member-1")

	hub.CreateTask("proj-1", Task{ID: "task-2", Title: "任务2"})
	hub.AssignTask("task-2", "member-1")

	// 完成一个任务
	hub.UpdateTask("task-1", map[string]interface{}{"status": TaskStatusDone})

	// 创建逾期任务
	deadline := time.Now().AddDate(0, 0, -1)
	hub.CreateTask("proj-1", Task{ID: "task-3", Title: "任务3", Deadline: &deadline})
	hub.AssignTask("task-3", "member-1")

	// 记录工时
	hub.LogTime("task-1", TimeEntry{ID: "entry-1", MemberID: "member-1", Hours: 8})
	hub.LogTime("task-2", TimeEntry{ID: "entry-2", MemberID: "member-1", Hours: 4})

	// 获取工作量
	workload, err := hub.GetMemberWorkload("member-1")
	if err != nil {
		t.Fatalf("GetMemberWorkload() failed: %v", err)
	}

	if workload.ActiveTasks != 2 {
		t.Errorf("expected 2 active tasks, got %d", workload.ActiveTasks)
	}
	if workload.TotalHours != 12 {
		t.Errorf("expected 12 total hours, got %f", workload.TotalHours)
	}
	if workload.OverdueTasks != 1 {
		t.Errorf("expected 1 overdue task, got %d", workload.OverdueTasks)
	}

	// 测试不存在的成员
	_, err = hub.GetMemberWorkload("nonexistent")
	if err == nil {
		t.Error("GetMemberWorkload() should fail for nonexistent member")
	}
}

func TestGetUpcomingDeadlines(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})

	// 创建即将到期的任务（3天内）
	deadline1 := time.Now().AddDate(0, 0, 2)
	hub.CreateTask("proj-1", Task{ID: "task-1", Title: "即将到期", Deadline: &deadline1})

	// 创建已完成的任务（应该被过滤）
	deadline2 := time.Now().AddDate(0, 0, 1)
	hub.CreateTask("proj-1", Task{ID: "task-2", Title: "已完成", Deadline: &deadline2})
	hub.UpdateTask("task-2", map[string]interface{}{"status": TaskStatusDone})

	// 创建远期任务（应该被过滤）
	deadline3 := time.Now().AddDate(0, 0, 10)
	hub.CreateTask("proj-1", Task{ID: "task-3", Title: "远期任务", Deadline: &deadline3})

	// 创建没有截止日期的任务（应该被过滤）
	hub.CreateTask("proj-1", Task{ID: "task-4", Title: "无截止日期"})

	// 获取7天内到期的任务
	upcoming := hub.GetUpcomingDeadlines(7)
	if len(upcoming) != 1 {
		t.Errorf("expected 1 upcoming deadline, got %d", len(upcoming))
	}
	if len(upcoming) > 0 && upcoming[0].ID != "task-1" {
		t.Errorf("expected task-1, got %s", upcoming[0].ID)
	}

	// 测试0天
	upcoming = hub.GetUpcomingDeadlines(0)
	if len(upcoming) != 0 {
		t.Errorf("expected 0 upcoming deadlines for 0 days, got %d", len(upcoming))
	}
}

func TestAddTeamMember(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 测试添加成员
	member := TeamMember{
		ID:    "member-1",
		Name:  "张三",
		Email: "zhangsan@example.com",
		Role:  "developer",
	}

	created, err := hub.AddTeamMember(member)
	if err != nil {
		t.Fatalf("AddTeamMember() failed: %v", err)
	}
	if created.Name != "张三" {
		t.Errorf("expected name 张三, got %s", created.Name)
	}

	// 测试添加重复成员
	_, err = hub.AddTeamMember(member)
	if err == nil {
		t.Error("AddTeamMember() should fail for duplicate ID")
	}

	// 测试缺少ID
	_, err = hub.AddTeamMember(TeamMember{Name: "李四"})
	if err == nil {
		t.Error("AddTeamMember() should fail without ID")
	}

	// 测试缺少Name
	_, err = hub.AddTeamMember(TeamMember{ID: "member-2"})
	if err == nil {
		t.Error("AddTeamMember() should fail without Name")
	}
}

func TestAddProjectMember(t *testing.T) {
	hub := New(Config{DataPath: "/tmp/test"})
	hub.Start()

	// 创建项目和成员
	hub.CreateProject(Project{ID: "proj-1", Name: "项目1"})
	hub.AddTeamMember(TeamMember{ID: "member-1", Name: "张三"})

	// 测试添加成员到项目
	if err := hub.AddProjectMember("proj-1", "member-1"); err != nil {
		t.Fatalf("AddProjectMember() failed: %v", err)
	}

	// 验证成员已添加
	project, _ := hub.GetProject("proj-1")
	found := false
	for _, id := range project.Members {
		if id == "member-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("member-1 should be in project members")
	}

	// 测试重复添加
	if err := hub.AddProjectMember("proj-1", "member-1"); err == nil {
		t.Error("AddProjectMember() should fail for duplicate member")
	}

	// 测试添加到不存在的项目
	if err := hub.AddProjectMember("nonexistent", "member-1"); err == nil {
		t.Error("AddProjectMember() should fail for nonexistent project")
	}

	// 测试添加不存在的成员
	if err := hub.AddProjectMember("proj-1", "nonexistent"); err == nil {
		t.Error("AddProjectMember() should fail for nonexistent member")
	}
}
