package projectboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	assert.NotNil(t, e)
	assert.NotNil(t, e.projects)
	assert.NotNil(t, e.boards)
	assert.NotNil(t, e.cards)
	assert.NotNil(t, e.labels)
}

// ========== 项目管理测试 ==========

func TestCreateProject(t *testing.T) {
	e := NewEngine()
	project, err := e.CreateProject("测试项目", "项目描述", "user1", "user1")
	require.NoError(t, err)
	assert.NotEmpty(t, project.ID)
	assert.Equal(t, "测试项目", project.Name)
	assert.Equal(t, ProjectStatusActive, project.Status)
	assert.Equal(t, "user1", project.OwnerID)
	assert.Contains(t, project.MemberIDs, "user1")
}

func TestGetProject(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")

	got, err := e.GetProject(project.ID)
	require.NoError(t, err)
	assert.Equal(t, project.ID, got.ID)

	_, err = e.GetProject("not-exist")
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestUpdateProject(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")

	updated, err := e.UpdateProject(project.ID, map[string]interface{}{
		"name":        "新项目名",
		"description": "新描述",
		"status":      ProjectStatusArchived,
	})
	require.NoError(t, err)
	assert.Equal(t, "新项目名", updated.Name)
	assert.Equal(t, "新描述", updated.Description)
	assert.Equal(t, ProjectStatusArchived, updated.Status)

	_, err = e.UpdateProject("not-exist", nil)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestDeleteProject(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	err := e.DeleteProject(project.ID)
	require.NoError(t, err)

	_, err = e.GetProject(project.ID)
	assert.ErrorIs(t, err, ErrProjectNotFound)

	// 验证关联数据已删除
	boards := e.ListBoards(project.ID)
	assert.Empty(t, boards)

	err = e.DeleteProject("not-exist")
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestListProjects(t *testing.T) {
	e := NewEngine()
	e.CreateProject("项目1", "", "user1", "user1")
	e.CreateProject("项目2", "", "user1", "user1")
	e.CreateProject("项目3", "", "user2", "user2")

	projects := e.ListProjects("")
	assert.Len(t, projects, 3)

	projects = e.ListProjects("user1")
	assert.Len(t, projects, 2)
}

func TestAddRemoveProjectMember(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")

	err := e.AddProjectMember(project.ID, "user2")
	require.NoError(t, err)

	got, _ := e.GetProject(project.ID)
	assert.Contains(t, got.MemberIDs, "user2")

	// 重复添加
	err = e.AddProjectMember(project.ID, "user2")
	require.NoError(t, err)

	err = e.RemoveProjectMember(project.ID, "user2")
	require.NoError(t, err)

	got, _ = e.GetProject(project.ID)
	assert.NotContains(t, got.MemberIDs, "user2")
}

// ========== 看板管理测试 ==========

func TestCreateBoard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")

	board, err := e.CreateBoard(project.ID, "看板1", "描述")
	require.NoError(t, err)
	assert.NotEmpty(t, board.ID)
	assert.Equal(t, project.ID, board.ProjectID)
	assert.Len(t, board.Columns, 4) // 默认4列

	got, _ := e.GetProject(project.ID)
	assert.Equal(t, 1, got.BoardCount)

	_, err = e.CreateBoard("not-exist", "看板", "")
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestGetBoard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")

	got, err := e.GetBoard(board.ID)
	require.NoError(t, err)
	assert.Equal(t, board.ID, got.ID)

	_, err = e.GetBoard("not-exist")
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestDeleteBoard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	err := e.DeleteBoard(board.ID)
	require.NoError(t, err)

	_, err = e.GetBoard(board.ID)
	assert.ErrorIs(t, err, ErrBoardNotFound)

	cards := e.ListCards(board.ID, CardFilter{})
	assert.Empty(t, cards)

	got, _ := e.GetProject(project.ID)
	assert.Equal(t, 0, got.BoardCount)
}

func TestListBoards(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	e.CreateBoard(project.ID, "看板1", "")
	e.CreateBoard(project.ID, "看板2", "")

	boards := e.ListBoards(project.ID)
	assert.Len(t, boards, 2)
}

// ========== 卡片管理测试 ==========

func TestCreateCard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")

	card, err := e.CreateCard(board.ID, "任务1", "描述", "user1", PriorityHigh)
	require.NoError(t, err)
	assert.NotEmpty(t, card.ID)
	assert.Equal(t, "任务1", card.Title)
	assert.Equal(t, CardStatusTodo, card.Status)
	assert.Equal(t, PriorityHigh, card.Priority)

	got, _ := e.GetBoard(board.ID)
	assert.Equal(t, 1, got.CardCount)

	_, err = e.CreateCard("not-exist", "任务", "", "user1", PriorityLow)
	assert.ErrorIs(t, err, ErrBoardNotFound)
}

func TestGetCard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	got, err := e.GetCard(card.ID)
	require.NoError(t, err)
	assert.Equal(t, card.ID, got.ID)

	_, err = e.GetCard("not-exist")
	assert.ErrorIs(t, err, ErrCardNotFound)
}

func TestUpdateCard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	updated, err := e.UpdateCard(card.ID, map[string]interface{}{
		"title":        "新标题",
		"description":  "新描述",
		"priority":     PriorityUrgent,
		"assignee_id":  "user2",
		"story_points": 5,
		"estimate_hrs": 8.0,
	})
	require.NoError(t, err)
	assert.Equal(t, "新标题", updated.Title)
	assert.Equal(t, PriorityUrgent, updated.Priority)
	assert.Equal(t, "user2", updated.AssigneeID)
	assert.Equal(t, 5, updated.StoryPoints)

	_, err = e.UpdateCard("not-exist", nil)
	assert.ErrorIs(t, err, ErrCardNotFound)
}

func TestDeleteCard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	err := e.DeleteCard(card.ID)
	require.NoError(t, err)

	_, err = e.GetCard(card.ID)
	assert.ErrorIs(t, err, ErrCardNotFound)

	got, _ := e.GetBoard(board.ID)
	assert.Equal(t, 0, got.CardCount)

	err = e.DeleteCard("not-exist")
	assert.ErrorIs(t, err, ErrCardNotFound)
}

func TestListCards(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
	e.CreateCard(board.ID, "任务2", "", "user1", PriorityHigh)
	e.CreateCard(board.ID, "任务3", "", "user1", PriorityMedium)

	cards := e.ListCards(board.ID, CardFilter{})
	assert.Len(t, cards, 3)

	cards = e.ListCards(board.ID, CardFilter{Priority: []CardPriority{PriorityHigh}})
	assert.Len(t, cards, 1)

	cards = e.ListCards(board.ID, CardFilter{Search: "任务1"})
	assert.Len(t, cards, 1)

	cards = e.ListCards(board.ID, CardFilter{Limit: 2})
	assert.Len(t, cards, 2)
}

func TestMoveCard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	// 移动到进行中列
	inProgressCol := board.Columns[1]
	updated, err := e.MoveCard(card.ID, inProgressCol.ID)
	require.NoError(t, err)
	assert.Equal(t, CardStatusInProgress, updated.Status)
	assert.Equal(t, inProgressCol.ID, updated.ColumnID)

	// 移动到完成列
	doneCol := board.Columns[3]
	updated, err = e.MoveCard(card.ID, doneCol.ID)
	require.NoError(t, err)
	assert.Equal(t, CardStatusDone, updated.Status)
	assert.Equal(t, 100, updated.Progress)

	_, err = e.MoveCard("not-exist", inProgressCol.ID)
	assert.ErrorIs(t, err, ErrCardNotFound)
}

func TestAssignCard(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	updated, err := e.AssignCard(card.ID, "user2")
	require.NoError(t, err)
	assert.Equal(t, "user2", updated.AssigneeID)
}

func TestUpdateCardProgress(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

	err := e.UpdateCardProgress(card.ID, 50)
	require.NoError(t, err)

	got, _ := e.GetCard(card.ID)
	assert.Equal(t, 50, got.Progress)

	err = e.UpdateCardProgress(card.ID, -1)
	assert.ErrorIs(t, err, ErrInvalidProgress)

	err = e.UpdateCardProgress(card.ID, 101)
	assert.ErrorIs(t, err, ErrInvalidProgress)
}

func TestSubtask(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")
	parent, _ := e.CreateCard(board.ID, "父任务", "", "user1", PriorityLow)
	child, _ := e.CreateCard(board.ID, "子任务", "", "user1", PriorityLow)

	err := e.AddSubtask(parent.ID, child.ID)
	require.NoError(t, err)

	got, _ := e.GetCard(parent.ID)
	assert.Contains(t, got.SubtaskIDs, child.ID)

	childGot, _ := e.GetCard(child.ID)
	assert.Equal(t, parent.ID, childGot.ParentID)
}

// ========== 标签测试 ==========

func TestLabel(t *testing.T) {
	t.Run("CreateLabel", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")

		label, err := e.CreateLabel(project.ID, "bug", "#ff0000")
		require.NoError(t, err)
		assert.NotEmpty(t, label.ID)
		assert.Equal(t, "bug", label.Name)

		_, err = e.CreateLabel(project.ID, "bug", "#0000ff")
		assert.ErrorIs(t, err, ErrLabelExists)

		_, err = e.CreateLabel("not-exist", "label", "#000000")
		assert.ErrorIs(t, err, ErrProjectNotFound)
	})

	t.Run("ListLabels", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		e.CreateLabel(project.ID, "feature", "#00ff00")
		e.CreateLabel(project.ID, "bug", "#ff0000")
		e.CreateLabel(project.ID, "docs", "#0000ff")

		labels := e.ListLabels(project.ID)
		assert.Len(t, labels, 3)
		assert.Equal(t, "bug", labels[0].Name)
		assert.Equal(t, "docs", labels[1].Name)
		assert.Equal(t, "feature", labels[2].Name)
	})

	t.Run("AddRemoveLabelFromCard", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		label, _ := e.CreateLabel(project.ID, "bug", "#ff0000")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

		err := e.AddLabelToCard(card.ID, label.ID)
		require.NoError(t, err)

		got, _ := e.GetCard(card.ID)
		assert.Contains(t, got.Labels, "bug")

		// 重复添加
		e.AddLabelToCard(card.ID, label.ID)
		got, _ = e.GetCard(card.ID)
		assert.Len(t, got.Labels, 1)

		err = e.RemoveLabelFromCard(card.ID, label.ID)
		require.NoError(t, err)
		got, _ = e.GetCard(card.ID)
		assert.Empty(t, got.Labels)
	})

	t.Run("DeleteLabel", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		label, _ := e.CreateLabel(project.ID, "bug", "#ff0000")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		e.AddLabelToCard(card.ID, label.ID)

		err := e.DeleteLabel(label.ID)
		require.NoError(t, err)

		got, _ := e.GetCard(card.ID)
		assert.Empty(t, got.Labels)
	})
}

// ========== 时间追踪测试 ==========

func TestTimeTracking(t *testing.T) {
	t.Run("LogTime", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

		entry, err := e.LogTime(card.ID, "user1", 2.5, "编码", time.Now())
		require.NoError(t, err)
		assert.NotEmpty(t, entry.ID)
		assert.Equal(t, 2.5, entry.Hours)

		got, _ := e.GetCard(card.ID)
		assert.Equal(t, 2.5, got.SpentHrs)
	})

	t.Run("ListTimeEntries", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

		e.LogTime(card.ID, "user1", 2, "编码", time.Now())
		e.LogTime(card.ID, "user1", 3, "测试", time.Now())

		entries := e.ListTimeEntries(card.ID)
		assert.Len(t, entries, 2)
	})
}

// ========== 项目统计测试 ==========

func TestGetProjectStats(t *testing.T) {
	e := NewEngine()
	project, _ := e.CreateProject("测试项目", "", "user1", "user1")
	board, _ := e.CreateBoard(project.ID, "看板1", "")

	card1, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
	e.UpdateCard(card1.ID, map[string]interface{}{"story_points": 3, "estimate_hrs": 8})

	card2, _ := e.CreateCard(board.ID, "任务2", "", "user1", PriorityHigh)
	e.UpdateCard(card2.ID, map[string]interface{}{"story_points": 5, "estimate_hrs": 16})

	// 移动任务1到完成
	doneCol := board.Columns[3]
	e.MoveCard(card1.ID, doneCol.ID)

	stats, err := e.GetProjectStats(project.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalCards)
	assert.Equal(t, 1, stats.ByStatus[string(CardStatusDone)])
	assert.Equal(t, 1, stats.ByStatus[string(CardStatusTodo)])
	assert.Equal(t, 8, stats.TotalPoints)
	assert.Equal(t, 3, stats.CompletedPoints)
	assert.Equal(t, 24.0, stats.TotalHours)

	_, err = e.GetProjectStats("not-exist")
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

// ========== 工作流测试 ==========

func TestWorkflow(t *testing.T) {
	t.Run("CreateWorkflow", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		we := NewWorkflowEngine(e)

		transitions := []Transition{
			{ID: "t1", Name: "开始", FromStatus: CardStatusTodo, ToStatus: CardStatusInProgress},
			{ID: "t2", Name: "完成", FromStatus: CardStatusInProgress, ToStatus: CardStatusDone},
		}

		workflow, err := we.CreateWorkflow(project.ID, "标准流程", transitions, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, workflow.ID)
		assert.Len(t, workflow.Transitions, 2)
	})

	t.Run("ExecuteTransition", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

		we := NewWorkflowEngine(e)
		transitions := []Transition{
			{ID: "t1", Name: "开始", FromStatus: CardStatusTodo, ToStatus: CardStatusInProgress},
			{ID: "t2", Name: "完成", FromStatus: CardStatusInProgress, ToStatus: CardStatusDone},
		}
		workflow, _ := we.CreateWorkflow(project.ID, "标准流程", transitions, nil)

		// 执行转换
		updated, err := we.ExecuteTransition(card.ID, workflow.ID, CardStatusInProgress)
		require.NoError(t, err)
		assert.Equal(t, CardStatusInProgress, updated.Status)

		updated, err = we.ExecuteTransition(card.ID, workflow.ID, CardStatusDone)
		require.NoError(t, err)
		assert.Equal(t, CardStatusDone, updated.Status)
		assert.Equal(t, 100, updated.Progress)
	})

	t.Run("InvalidTransition", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

		we := NewWorkflowEngine(e)
		transitions := []Transition{
			{ID: "t1", Name: "开始", FromStatus: CardStatusTodo, ToStatus: CardStatusInProgress},
		}
		workflow, _ := we.CreateWorkflow(project.ID, "标准流程", transitions, nil)

		// 无效转换
		_, err := we.ExecuteTransition(card.ID, workflow.ID, CardStatusDone)
		assert.ErrorIs(t, err, ErrInvalidTransition)
	})

	t.Run("GetAvailableTransitions", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)

		we := NewWorkflowEngine(e)
		transitions := []Transition{
			{ID: "t1", Name: "开始", FromStatus: CardStatusTodo, ToStatus: CardStatusInProgress},
		}
		workflow, _ := we.CreateWorkflow(project.ID, "标准流程", transitions, nil)

		available, err := we.GetAvailableTransitions(card.ID, workflow.ID)
		require.NoError(t, err)
		assert.Len(t, available, 1)
		assert.Contains(t, available, CardStatusInProgress)
	})

	t.Run("AddRemoveTransition", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		we := NewWorkflowEngine(e)
		workflow, _ := we.CreateWorkflow(project.ID, "标准流程", nil, nil)

		transition := Transition{ID: "t1", Name: "开始", FromStatus: CardStatusTodo, ToStatus: CardStatusInProgress}
		err := we.AddTransition(workflow.ID, transition)
		require.NoError(t, err)

		got, _ := we.GetWorkflow(workflow.ID)
		assert.Len(t, got.Transitions, 1)

		err = we.RemoveTransition(workflow.ID, "t1")
		require.NoError(t, err)

		got, _ = we.GetWorkflow(workflow.ID)
		assert.Empty(t, got.Transitions)
	})

	t.Run("AutomationRules", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		we := NewWorkflowEngine(e)
		workflow, _ := we.CreateWorkflow(project.ID, "标准流程", nil, nil)

		rule := AutomationRule{
			ID:        "r1",
			Name:      "完成时清空",
			Trigger:   "card_moved",
			Condition: "high_priority",
			Actions:   []string{"set_progress_100"},
			Enabled:   true,
		}

		err := we.AddAutomationRule(workflow.ID, rule)
		require.NoError(t, err)

		err = we.ToggleAutomationRule(workflow.ID, "r1")
		require.NoError(t, err)

		got, _ := we.GetWorkflow(workflow.ID)
		assert.False(t, got.Rules[0].Enabled)

		err = we.RemoveAutomationRule(workflow.ID, "r1")
		require.NoError(t, err)
	})
}

// ========== 敏捷管理测试 ==========

func TestAgile(t *testing.T) {
	t.Run("CreateSprint", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		am := NewAgileManager(e)

		start := time.Now()
		end := start.AddDate(0, 0, 14)

		sprint, err := am.CreateSprint(project.ID, "Sprint 1", "完成核心功能", start, end)
		require.NoError(t, err)
		assert.NotEmpty(t, sprint.ID)
		assert.Equal(t, "planned", sprint.Status)
	})

	t.Run("StartCompleteSprint", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		am := NewAgileManager(e)

		start := time.Now()
		end := start.AddDate(0, 0, 14)
		sprint, _ := am.CreateSprint(project.ID, "Sprint 1", "", start, end)

		started, err := am.StartSprint(sprint.ID)
		require.NoError(t, err)
		assert.Equal(t, "active", started.Status)

		completed, err := am.CompleteSprint(sprint.ID)
		require.NoError(t, err)
		assert.Equal(t, "completed", completed.Status)
	})

	t.Run("AddRemoveCardFromSprint", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		am := NewAgileManager(e)

		start := time.Now()
		end := start.AddDate(0, 0, 14)
		sprint, _ := am.CreateSprint(project.ID, "Sprint 1", "", start, end)

		err := am.AddCardToSprint(sprint.ID, card.ID)
		require.NoError(t, err)

		cards, _ := am.GetSprintCards(sprint.ID)
		assert.Len(t, cards, 1)

		err = am.RemoveCardFromSprint(sprint.ID, card.ID)
		require.NoError(t, err)

		cards, _ = am.GetSprintCards(sprint.ID)
		assert.Empty(t, cards)
	})

	t.Run("GetSprintProgress", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		am := NewAgileManager(e)

		start := time.Now()
		end := start.AddDate(0, 0, 14)
		sprint, _ := am.CreateSprint(project.ID, "Sprint 1", "", start, end)

		card1, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		e.UpdateCard(card1.ID, map[string]interface{}{"story_points": 3})
		am.AddCardToSprint(sprint.ID, card1.ID)

		card2, _ := e.CreateCard(board.ID, "任务2", "", "user1", PriorityHigh)
		e.UpdateCard(card2.ID, map[string]interface{}{"story_points": 5})
		am.AddCardToSprint(sprint.ID, card2.ID)

		progress, err := am.GetSprintProgress(sprint.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, progress["total_cards"])
		assert.Equal(t, 8, progress["total_points"])
	})

	t.Run("VelocityData", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		am := NewAgileManager(e)

		// 创建并完成一个迭代
		start := time.Now().AddDate(0, -1, 0)
		end := start.AddDate(0, 0, 14)
		sprint, _ := am.CreateSprint(project.ID, "Sprint 1", "", start, end)

		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		e.UpdateCard(card.ID, map[string]interface{}{"story_points": 5})
		am.AddCardToSprint(sprint.ID, card.ID)

		am.StartSprint(sprint.ID)
		e.MoveCard(card.ID, board.Columns[3].ID) // 移动到完成列
		am.CompleteSprint(sprint.ID)

		velocity, err := am.GetVelocityData(project.ID)
		require.NoError(t, err)
		assert.Len(t, velocity, 1)
		assert.Equal(t, 5, velocity[0].Completed)

		avgVelocity, err := am.GetAverageVelocity(project.ID, 3)
		require.NoError(t, err)
		assert.Equal(t, 5.0, avgVelocity)
	})

	t.Run("EstimateCapacity", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		am := NewAgileManager(e)

		start := time.Now()
		end := start.AddDate(0, 0, 14)

		capacity, err := am.EstimateSprintCapacity(project.ID, start, end)
		require.NoError(t, err)
		assert.Equal(t, 14, capacity["days"])
	})
}

// ========== 甘特图测试 ==========

func TestGantt(t *testing.T) {
	t.Run("AddRemoveDependency", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card1, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		card2, _ := e.CreateCard(board.ID, "任务2", "", "user1", PriorityLow)

		gm := NewGanttManager(e)

		err := gm.AddDependency(card1.ID, card2.ID, DependencyFS)
		require.NoError(t, err)

		got, _ := e.GetCard(card1.ID)
		assert.Contains(t, got.Dependencies, card2.ID)

		err = gm.RemoveDependency(card1.ID, card2.ID)
		require.NoError(t, err)

		got, _ = e.GetCard(card1.ID)
		assert.Empty(t, got.Dependencies)
	})

	t.Run("CircularDependency", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")
		card1, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		card2, _ := e.CreateCard(board.ID, "任务2", "", "user1", PriorityLow)

		gm := NewGanttManager(e)

		err := gm.AddDependency(card1.ID, card2.ID, DependencyFS)
		require.NoError(t, err)

		// 尝试创建循环依赖
		err = gm.AddDependency(card2.ID, card1.ID, DependencyFS)
		assert.ErrorIs(t, err, ErrCircularDependency)
	})

	t.Run("GetGanttTasks", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")

		now := time.Now()
		card1, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		e.UpdateCard(card1.ID, map[string]interface{}{
			"start_date": now,
			"due_date":   now.AddDate(0, 0, 3),
		})
		card2, _ := e.CreateCard(board.ID, "任务2", "", "user1", PriorityLow)
		e.UpdateCard(card2.ID, map[string]interface{}{
			"start_date": now,
			"due_date":   now.AddDate(0, 0, 5),
		})

		gm := NewGanttManager(e)
		tasks := gm.GetGanttTasks(board.ID)
		assert.Len(t, tasks, 2)
	})

	t.Run("ResourceUtilization", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")

		now := time.Now()
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		e.UpdateCard(card.ID, map[string]interface{}{
			"assignee_id": "user2",
			"start_date":  now,
			"due_date":    now.AddDate(0, 0, 5),
		})

		gm := NewGanttManager(e)
		utilization := gm.GetResourceUtilization(board.ID, now, now.AddDate(0, 0, 14))
		assert.Contains(t, utilization, "user2")
	})

	t.Run("GetTimeline", func(t *testing.T) {
		e := NewEngine()
		project, _ := e.CreateProject("测试项目", "", "user1", "user1")
		board, _ := e.CreateBoard(project.ID, "看板1", "")

		now := time.Now()
		card, _ := e.CreateCard(board.ID, "任务1", "", "user1", PriorityLow)
		e.UpdateCard(card.ID, map[string]interface{}{
			"start_date": now,
			"due_date":   now.AddDate(0, 0, 5),
		})

		gm := NewGanttManager(e)
		timeline := gm.GetTimeline(board.ID)
		assert.NotNil(t, timeline)
		assert.Equal(t, 1, timeline["total_tasks"])
	})
}
