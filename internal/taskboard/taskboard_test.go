package taskboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.boards)
	assert.NotNil(t, m.tasks)
	assert.NotNil(t, m.labels)
}

// ========== 看板管理测试 ==========

func TestCreateBoard(t *testing.T) {
	m := NewManager()
	board, err := m.CreateBoard("测试看板", "测试描述", "user1", "user1")
	require.NoError(t, err)
	assert.NotEmpty(t, board.ID)
	assert.Equal(t, "测试看板", board.Name)
	assert.Equal(t, "测试描述", board.Description)
	assert.Equal(t, "user1", board.OwnerID)
}

func TestGetBoard(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")

	got, err := m.GetBoard(board.ID)
	require.NoError(t, err)
	assert.Equal(t, board.ID, got.ID)

	_, err = m.GetBoard("not-exist")
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestDeleteBoard(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	err := m.DeleteBoard(board.ID)
	require.NoError(t, err)

	_, err = m.GetBoard(board.ID)
	assert.ErrorIs(t, err, ErrBoardNotFound)

	// 验证关联任务已删除
	tasks := m.ListTasks(board.ID, TaskFilter{})
	assert.Empty(t, tasks)

	// 删除不存在的看板
	err = m.DeleteBoard("not-exist")
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestListBoards(t *testing.T) {
	m := NewManager()
	m.CreateBoard("看板1", "", "user1", "user1")
	m.CreateBoard("看板2", "", "user1", "user1")
	m.CreateBoard("看板3", "", "user2", "user2")

	boards := m.ListBoards("")
	assert.Len(t, boards, 3)

	boards = m.ListBoards("user1")
	assert.Len(t, boards, 2)

	boards = m.ListBoards("user2")
	assert.Len(t, boards, 1)
}

// ========== 任务卡片测试 ==========

func TestCreateTask(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")

	task, err := m.CreateTask(board.ID, "任务1", "描述", "user1", PriorityHigh)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "任务1", task.Title)
	assert.Equal(t, StatusTodo, task.Status)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Equal(t, 0, task.Progress)

	// 更新看板任务计数
	got, _ := m.GetBoard(board.ID)
	assert.Equal(t, 1, got.TaskCount)

	// 创建到不存在的看板
	_, err = m.CreateTask("not-exist", "任务", "", "user1", PriorityLow)
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestUpdateTask(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	updates := map[string]interface{}{
		"title":       "新标题",
		"description": "新描述",
		"priority":    PriorityHigh,
		"assignee_id": "user2",
	}

	updated, err := m.UpdateTask(task.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, "新标题", updated.Title)
	assert.Equal(t, "新描述", updated.Description)
	assert.Equal(t, PriorityHigh, updated.Priority)
	assert.Equal(t, "user2", updated.AssigneeID)

	// 更新不存在的任务
	_, err = m.UpdateTask("not-exist", updates)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestDeleteTask(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	err := m.DeleteTask(task.ID)
	require.NoError(t, err)

	_, err = m.GetTask(task.ID)
	assert.ErrorIs(t, err, ErrTaskNotFound)

	// 验证看板任务计数
	got, _ := m.GetBoard(board.ID)
	assert.Equal(t, 0, got.TaskCount)

	// 删除不存在的任务
	err = m.DeleteTask("not-exist")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestListTasks(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)
	m.CreateTask(board.ID, "任务2", "", "user1", PriorityHigh)
	m.CreateTask(board.ID, "任务3", "", "user1", PriorityMedium)

	// 列出所有
	tasks := m.ListTasks(board.ID, TaskFilter{})
	assert.Len(t, tasks, 3)

	// 按优先级筛选
	tasks = m.ListTasks(board.ID, TaskFilter{Priority: []TaskPriority{PriorityHigh}})
	assert.Len(t, tasks, 1)
	assert.Equal(t, "任务2", tasks[0].Title)

	// 搜索
	tasks = m.ListTasks(board.ID, TaskFilter{Search: "任务1"})
	assert.Len(t, tasks, 1)

	// 分页
	tasks = m.ListTasks(board.ID, TaskFilter{Limit: 2})
	assert.Len(t, tasks, 2)
}

// ========== 状态流转测试 ==========

func TestTransitionTask(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	// todo -> in_progress
	updated, err := m.TransitionTask(task.ID, StatusInProgress)
	require.NoError(t, err)
	assert.Equal(t, StatusInProgress, updated.Status)
	assert.Equal(t, 10, updated.Progress) // 自动设置进度

	// in_progress -> done
	updated, err = m.TransitionTask(task.ID, StatusDone)
	require.NoError(t, err)
	assert.Equal(t, StatusDone, updated.Status)
	assert.Equal(t, 100, updated.Progress) // 自动设置进度

	// done -> in_progress (允许回退)
	updated, err = m.TransitionTask(task.ID, StatusInProgress)
	require.NoError(t, err)
	assert.Equal(t, StatusInProgress, updated.Status)
}

func TestTransitionTaskInvalid(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	// todo -> done (不允许跳过)
	_, err := m.TransitionTask(task.ID, StatusDone)
	assert.ErrorIs(t, err, ErrInvalidStatus)

	// 不存在的任务
	_, err = m.TransitionTask("not-exist", StatusInProgress)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestGetAvailableTransitions(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	// todo 状态
	transitions, err := m.GetAvailableTransitions(task.ID)
	require.NoError(t, err)
	assert.Len(t, transitions, 1)
	assert.Contains(t, transitions, StatusInProgress)

	// in_progress 状态
	m.TransitionTask(task.ID, StatusInProgress)
	transitions, _ = m.GetAvailableTransitions(task.ID)
	assert.Len(t, transitions, 2)
	assert.Contains(t, transitions, StatusTodo)
	assert.Contains(t, transitions, StatusDone)
}

// ========== 标签系统测试 ==========

func TestCreateLabel(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")

	label, err := m.CreateLabel(board.ID, "bug", "#ff0000")
	require.NoError(t, err)
	assert.NotEmpty(t, label.ID)
	assert.Equal(t, "bug", label.Name)
	assert.Equal(t, "#ff0000", label.Color)

	// 重复标签名
	_, err = m.CreateLabel(board.ID, "bug", "#0000ff")
	assert.ErrorIs(t, err, ErrLabelExists)

	// 不存在的看板
	_, err = m.CreateLabel("not-exist", "label", "#000000")
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestDeleteLabel(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	label, _ := m.CreateLabel(board.ID, "bug", "#ff0000")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)
	m.AddLabelToTask(task.ID, label.ID)

	err := m.DeleteLabel(label.ID)
	require.NoError(t, err)

	_, err = m.GetLabel(label.ID)
	assert.ErrorIs(t, err, ErrLabelNotFound)

	// 验证任务标签已移除
	got, _ := m.GetTask(task.ID)
	assert.Empty(t, got.Labels)
}

func TestListLabels(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	m.CreateLabel(board.ID, "feature", "#00ff00")
	m.CreateLabel(board.ID, "bug", "#ff0000")
	m.CreateLabel(board.ID, "docs", "#0000ff")

	labels := m.ListLabels(board.ID)
	assert.Len(t, labels, 3)
	// 验证按名称排序
	assert.Equal(t, "bug", labels[0].Name)
	assert.Equal(t, "docs", labels[1].Name)
	assert.Equal(t, "feature", labels[2].Name)
}

func TestAddRemoveLabelToTask(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	label, _ := m.CreateLabel(board.ID, "bug", "#ff0000")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	err := m.AddLabelToTask(task.ID, label.ID)
	require.NoError(t, err)

	got, _ := m.GetTask(task.ID)
	assert.Contains(t, got.Labels, "bug")

	// 重复添加不会重复
	m.AddLabelToTask(task.ID, label.ID)
	got, _ = m.GetTask(task.ID)
	assert.Len(t, got.Labels, 1)

	// 移除标签
	err = m.RemoveLabelFromTask(task.ID, label.ID)
	require.NoError(t, err)
	got, _ = m.GetTask(task.ID)
	assert.Empty(t, got.Labels)
}

// ========== 进度追踪测试 ==========

func TestUpdateProgress(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	err := m.UpdateProgress(task.ID, 50)
	require.NoError(t, err)

	got, _ := m.GetTask(task.ID)
	assert.Equal(t, 50, got.Progress)
	assert.Equal(t, StatusInProgress, got.Status) // 自动切换状态

	// 完成进度
	err = m.UpdateProgress(task.ID, 100)
	require.NoError(t, err)
	got, _ = m.GetTask(task.ID)
	assert.Equal(t, 100, got.Progress)
	assert.Equal(t, StatusDone, got.Status)

	// 无效进度值
	err = m.UpdateProgress(task.ID, -1)
	assert.ErrorIs(t, err, ErrInvalidProgress)

	err = m.UpdateProgress(task.ID, 101)
	assert.ErrorIs(t, err, ErrInvalidProgress)

	// 不存在的任务
	err = m.UpdateProgress("not-exist", 50)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestGetBoardStats(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)
	m.CreateTask(board.ID, "任务2", "", "user1", PriorityHigh)
	task3, _ := m.CreateTask(board.ID, "任务3", "", "user1", PriorityMedium)

	m.TransitionTask(task3.ID, StatusInProgress)
	m.UpdateProgress(task3.ID, 50)

	stats, err := m.GetBoardStats(board.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalTasks)
	assert.Equal(t, 2, stats.ByStatus[string(StatusTodo)])
	assert.Equal(t, 1, stats.ByStatus[string(StatusInProgress)])
	assert.Equal(t, 1, stats.ByPriority[string(PriorityLow)])
	assert.Equal(t, 1, stats.ByPriority[string(PriorityHigh)])
	assert.Equal(t, 1, stats.ByPriority[string(PriorityMedium)])
	assert.InDelta(t, 16.67, stats.AvgProgress, 0.1)

	// 不存在的看板
	_, err = m.GetBoardStats("not-exist")
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestGetTaskProgress(t *testing.T) {
	m := NewManager()
	board, _ := m.CreateBoard("测试看板", "", "user1", "user1")
	task, _ := m.CreateTask(board.ID, "任务1", "", "user1", PriorityLow)

	progress, err := m.GetTaskProgress(task.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, progress)

	m.UpdateProgress(task.ID, 75)
	progress, _ = m.GetTaskProgress(task.ID)
	assert.Equal(t, 75, progress)
}
