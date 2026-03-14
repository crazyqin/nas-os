package project

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.tasks)
	assert.NotNil(t, mgr.milestones)
	assert.NotNil(t, mgr.projects)
	assert.NotNil(t, mgr.comments)
	assert.NotNil(t, mgr.history)
}

func TestProjectCRUD(t *testing.T) {
	mgr := NewManager()

	// 创建项目
	project, err := mgr.CreateProject("测试项目", "TEST", "这是一个测试项目", "user1", "user1")
	assert.NoError(t, err)
	assert.NotNil(t, project)
	assert.Equal(t, "测试项目", project.Name)
	assert.Equal(t, "TEST", project.Key)
	assert.Equal(t, "active", project.Status)
	assert.NotEmpty(t, project.ID)

	// 获取项目
	retrieved, err := mgr.GetProject(project.ID)
	assert.NoError(t, err)
	assert.Equal(t, project.ID, retrieved.ID)

	// 更新项目
	updates := map[string]interface{}{
		"name":        "更新后的项目名",
		"description": "更新后的描述",
	}
	updated, err := mgr.UpdateProject(project.ID, updates)
	assert.NoError(t, err)
	assert.Equal(t, "更新后的项目名", updated.Name)
	assert.Equal(t, "更新后的描述", updated.Description)

	// 列出项目
	projects := mgr.ListProjects("user1", 10, 0)
	assert.Len(t, projects, 1)

	// 删除项目
	err = mgr.DeleteProject(project.ID)
	assert.NoError(t, err)

	// 确认删除
	_, err = mgr.GetProject(project.ID)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestMilestoneCRUD(t *testing.T) {
	mgr := NewManager()

	// 先创建项目
	project, err := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	assert.NoError(t, err)

	dueDate := time.Now().AddDate(0, 1, 0)

	// 创建里程碑
	milestone, err := mgr.CreateMilestone("v1.0发布", "第一个版本", project.ID, "user1", &dueDate)
	assert.NoError(t, err)
	assert.NotNil(t, milestone)
	assert.Equal(t, "v1.0发布", milestone.Name)
	assert.Equal(t, "planned", milestone.Status)
	assert.Equal(t, project.ID, milestone.ProjectID)

	// 获取里程碑
	retrieved, err := mgr.GetMilestone(milestone.ID)
	assert.NoError(t, err)
	assert.Equal(t, milestone.ID, retrieved.ID)

	// 更新里程碑
	updates := map[string]interface{}{
		"status": "active",
	}
	updated, err := mgr.UpdateMilestone(milestone.ID, updates)
	assert.NoError(t, err)
	assert.Equal(t, "active", updated.Status)

	// 完成里程碑
	updates = map[string]interface{}{
		"status": "completed",
	}
	updated, err = mgr.UpdateMilestone(milestone.ID, updates)
	assert.NoError(t, err)
	assert.Equal(t, "completed", updated.Status)
	assert.NotNil(t, updated.CompletedAt)

	// 列出里程碑
	milestones := mgr.ListMilestones(project.ID)
	assert.Len(t, milestones, 1)

	// 删除里程碑
	err = mgr.DeleteMilestone(milestone.ID)
	assert.NoError(t, err)

	// 确认删除
	_, err = mgr.GetMilestone(milestone.ID)
	assert.ErrorIs(t, err, ErrMilestoneNotFound)
}

func TestTaskCRUD(t *testing.T) {
	mgr := NewManager()

	// 先创建项目
	project, err := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	assert.NoError(t, err)

	// 创建任务
	task, err := mgr.CreateTask("实现登录功能", "使用JWT实现用户登录", project.ID, "user1", PriorityHigh)
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "实现登录功能", task.Title)
	assert.Equal(t, TaskStatusTodo, task.Status)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Equal(t, project.ID, task.ProjectID)

	// 获取任务
	retrieved, err := mgr.GetTask(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, task.ID, retrieved.ID)

	// 更新任务状态
	updates := map[string]interface{}{
		"status":      TaskStatusInProgress,
		"assignee_id": "user2",
	}
	updated, err := mgr.UpdateTask(task.ID, "user1", updates)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusInProgress, updated.Status)
	assert.Equal(t, "user2", updated.AssigneeID)

	// 完成任务
	updates = map[string]interface{}{
		"status": TaskStatusDone,
	}
	updated, err = mgr.UpdateTask(task.ID, "user1", updates)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusDone, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
	assert.Equal(t, 100, updated.Progress)

	// 列出任务
	filter := TaskFilter{
		ProjectID: project.ID,
	}
	tasks := mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)

	// 删除任务
	err = mgr.DeleteTask(task.ID)
	assert.NoError(t, err)

	// 确认删除
	_, err = mgr.GetTask(task.ID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskFilter(t *testing.T) {
	mgr := NewManager()

	// 创建项目
	project, err := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	assert.NoError(t, err)

	// 创建多个任务
	task1, _ := mgr.CreateTask("任务1", "", project.ID, "user1", PriorityHigh)
	mgr.UpdateTask(task1.ID, "user1", map[string]interface{}{"assignee_id": "user2", "status": TaskStatusInProgress})

	task2, _ := mgr.CreateTask("任务2", "", project.ID, "user1", PriorityLow)
	mgr.UpdateTask(task2.ID, "user1", map[string]interface{}{"assignee_id": "user3"})

	task3, _ := mgr.CreateTask("任务3", "", project.ID, "user1", PriorityUrgent)
	mgr.UpdateTask(task3.ID, "user1", map[string]interface{}{"status": TaskStatusDone})

	// 按状态筛选
	filter := TaskFilter{
		ProjectID: project.ID,
		Status:    []TaskStatus{TaskStatusInProgress},
	}
	tasks := mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task1.ID, tasks[0].ID)

	// 按优先级筛选
	filter = TaskFilter{
		ProjectID: project.ID,
		Priority:  []TaskPriority{PriorityUrgent},
	}
	tasks = mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task3.ID, tasks[0].ID)

	// 按负责人筛选
	filter = TaskFilter{
		ProjectID:  project.ID,
		AssigneeID: "user2",
	}
	tasks = mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task1.ID, tasks[0].ID)

	// 搜索
	filter = TaskFilter{
		ProjectID: project.ID,
		Search:    "任务1",
	}
	tasks = mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)
}

func TestTaskStats(t *testing.T) {
	mgr := NewManager()

	// 创建项目
	project, err := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	assert.NoError(t, err)

	// 创建多个任务
	_, _ = mgr.CreateTask("任务1", "", project.ID, "user1", PriorityHigh)
	task2, _ := mgr.CreateTask("任务2", "", project.ID, "user1", PriorityLow)
	task3, _ := mgr.CreateTask("任务3", "", project.ID, "user1", PriorityUrgent)

	// 更新状态
	mgr.UpdateTask(task2.ID, "user1", map[string]interface{}{"status": TaskStatusInProgress})
	mgr.UpdateTask(task3.ID, "user1", map[string]interface{}{"status": TaskStatusDone, "assignee_id": "user2"})

	// 获取统计
	stats := mgr.GetTaskStats(project.ID)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 1, stats.ByStatus[string(TaskStatusTodo)])
	assert.Equal(t, 1, stats.ByStatus[string(TaskStatusInProgress)])
	assert.Equal(t, 1, stats.ByStatus[string(TaskStatusDone)])
	assert.Equal(t, 1, stats.ByPriority[string(PriorityHigh)])
	assert.Equal(t, 1, stats.ByPriority[string(PriorityLow)])
	assert.Equal(t, 1, stats.ByPriority[string(PriorityUrgent)])
	assert.Equal(t, 1, stats.ByAssignee["user2"])
}

func TestComments(t *testing.T) {
	mgr := NewManager()

	// 创建项目和任务
	project, _ := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	task, _ := mgr.CreateTask("任务", "", project.ID, "user1", PriorityMedium)

	// 添加评论
	comment, err := mgr.AddComment(task.ID, "user1", "这是一个评论")
	assert.NoError(t, err)
	assert.NotNil(t, comment)
	assert.Equal(t, "这是一个评论", comment.Content)
	assert.Equal(t, task.ID, comment.TaskID)

	// 获取评论
	comments := mgr.GetComments(task.ID)
	assert.Len(t, comments, 1)
	assert.Equal(t, comment.ID, comments[0].ID)
}

func TestTaskHistory(t *testing.T) {
	mgr := NewManager()

	// 创建项目和任务
	project, _ := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	task, _ := mgr.CreateTask("任务", "", project.ID, "user1", PriorityMedium)

	// 更新任务（会记录历史）
	_, err := mgr.UpdateTask(task.ID, "user1", map[string]interface{}{
		"status":      TaskStatusInProgress,
		"priority":    PriorityHigh,
		"assignee_id": "user2",
	})
	assert.NoError(t, err)

	// 获取历史
	history := mgr.GetHistory(task.ID)
	assert.NotEmpty(t, history)

	// 验证历史记录
	var statusChange, priorityChange, assigneeChange bool
	for _, h := range history {
		if h.Field == "status" {
			statusChange = true
			assert.Equal(t, string(TaskStatusTodo), h.OldValue)
			assert.Equal(t, string(TaskStatusInProgress), h.NewValue)
		}
		if h.Field == "priority" {
			priorityChange = true
			assert.Equal(t, string(PriorityMedium), h.OldValue)
			assert.Equal(t, string(PriorityHigh), h.NewValue)
		}
		if h.Field == "assignee" {
			assigneeChange = true
			assert.Equal(t, "", h.OldValue)
			assert.Equal(t, "user2", h.NewValue)
		}
	}
	assert.True(t, statusChange, "状态变更应该被记录")
	assert.True(t, priorityChange, "优先级变更应该被记录")
	assert.True(t, assigneeChange, "负责人变更应该被记录")
}

func TestProjectWithMilestone(t *testing.T) {
	mgr := NewManager()

	// 创建项目
	project, _ := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")

	// 创建里程碑
	milestone, _ := mgr.CreateMilestone("v1.0", "", project.ID, "user1", nil)

	// 创建任务并关联里程碑
	task, _ := mgr.CreateTask("任务1", "", project.ID, "user1", PriorityMedium)
	_, err := mgr.UpdateTask(task.ID, "user1", map[string]interface{}{
		"milestone_id": milestone.ID,
	})
	assert.NoError(t, err)

	// 验证关联
	retrieved, _ := mgr.GetTask(task.ID)
	assert.Equal(t, milestone.ID, retrieved.MilestoneID)

	// 删除里程碑应该清除任务的里程碑引用
	err = mgr.DeleteMilestone(milestone.ID)
	assert.NoError(t, err)

	retrieved, _ = mgr.GetTask(task.ID)
	assert.Equal(t, "", retrieved.MilestoneID)
}

func TestDeleteProjectCascades(t *testing.T) {
	mgr := NewManager()

	// 创建项目及关联数据
	project, _ := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	milestone, _ := mgr.CreateMilestone("v1.0", "", project.ID, "user1", nil)
	task, _ := mgr.CreateTask("任务", "", project.ID, "user1", PriorityMedium)

	// 删除项目
	err := mgr.DeleteProject(project.ID)
	assert.NoError(t, err)

	// 验证关联数据被删除
	_, err = mgr.GetMilestone(milestone.ID)
	assert.ErrorIs(t, err, ErrMilestoneNotFound)

	_, err = mgr.GetTask(task.ID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestProjectNotFound(t *testing.T) {
	mgr := NewManager()

	// 尝试获取不存在的项目
	_, err := mgr.GetProject("nonexistent")
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// 尝试更新不存在的项目
	_, err = mgr.UpdateProject("nonexistent", map[string]interface{}{})
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// 尝试删除不存在的项目
	err = mgr.DeleteProject("nonexistent")
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// 尝试为不存在的项目创建里程碑
	_, err = mgr.CreateMilestone("test", "", "nonexistent", "user1", nil)
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// 尝试为不存在的项目创建任务
	_, err = mgr.CreateTask("test", "", "nonexistent", "user1", PriorityMedium)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestTaskProgress(t *testing.T) {
	mgr := NewManager()

	project, _ := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")
	task, _ := mgr.CreateTask("任务", "", project.ID, "user1", PriorityMedium)

	// 更新进度
	_, err := mgr.UpdateTask(task.ID, "user1", map[string]interface{}{
		"progress": 50,
	})
	assert.NoError(t, err)

	retrieved, _ := mgr.GetTask(task.ID)
	assert.Equal(t, 50, retrieved.Progress)

	// 完成状态时自动设置进度为100
	_, err = mgr.UpdateTask(task.ID, "user1", map[string]interface{}{
		"status": TaskStatusDone,
	})
	assert.NoError(t, err)

	retrieved, _ = mgr.GetTask(task.ID)
	assert.Equal(t, 100, retrieved.Progress)
}

func TestDueDateFilter(t *testing.T) {
	mgr := NewManager()

	project, _ := mgr.CreateProject("测试项目", "TEST", "", "user1", "user1")

	// 创建有截止日期的任务
	task1, _ := mgr.CreateTask("任务1", "", project.ID, "user1", PriorityMedium)
	pastDate := time.Now().AddDate(0, 0, -1)
	futureDate := time.Now().AddDate(0, 0, 1)
	mgr.UpdateTask(task1.ID, "user1", map[string]interface{}{"due_date": &pastDate})

	task2, _ := mgr.CreateTask("任务2", "", project.ID, "user1", PriorityMedium)
	mgr.UpdateTask(task2.ID, "user1", map[string]interface{}{"due_date": &futureDate})

	// 筛选过期任务
	now := time.Now()
	filter := TaskFilter{
		ProjectID: project.ID,
		DueBefore: &now,
	}
	tasks := mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task1.ID, tasks[0].ID)

	// 筛选未来任务
	filter = TaskFilter{
		ProjectID: project.ID,
		DueAfter:  &now,
	}
	tasks = mgr.ListTasks(filter)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task2.ID, tasks[0].ID)
}
