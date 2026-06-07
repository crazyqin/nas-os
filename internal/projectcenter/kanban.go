package projectcenter

import (
	"fmt"
	"sync"
	"time"
)

// KanbanManager 看板管理器
type KanbanManager struct {
	mu      sync.RWMutex
	boards  map[string]*KanbanBoard
	taskMgr *TaskManager
	nextID  int
}

// NewKanbanManager 创建看板管理器
func NewKanbanManager(taskMgr *TaskManager) *KanbanManager {
	return &KanbanManager{
		boards:  make(map[string]*KanbanBoard),
		taskMgr: taskMgr,
		nextID:  1,
	}
}

// CreateBoard 创建看板
func (m *KanbanManager) CreateBoard(projectID, name string) (*KanbanBoard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("board name is required")
	}

	id := fmt.Sprintf("board_%d", m.nextID)
	m.nextID++

	board := &KanbanBoard{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Columns:   defaultColumns(),
		Filters:   KanbanFilters{},
		CreatedAt: time.Now(),
	}

	m.boards[id] = board
	return board, nil
}

// defaultColumns 默认看板列
func defaultColumns() []KanbanColumn {
	return []KanbanColumn{
		{ID: "col_todo", Name: "待办", Status: TaskStatusTodo, Order: 1, WIPLimit: 0},
		{ID: "col_in_progress", Name: "进行中", Status: TaskStatusInProgress, Order: 2, WIPLimit: 5},
		{ID: "col_review", Name: "审核", Status: TaskStatusReview, Order: 3, WIPLimit: 3},
		{ID: "col_done", Name: "完成", Status: TaskStatusDone, Order: 4, WIPLimit: 0},
	}
}

// GetBoard 获取看板
func (m *KanbanManager) GetBoard(boardID string) (*KanbanBoard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}
	return board, nil
}

// GetProjectBoard 获取项目的看板
func (m *KanbanManager) GetProjectBoard(projectID string) (*KanbanBoard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, board := range m.boards {
		if board.ProjectID == projectID {
			return board, nil
		}
	}
	return nil, fmt.Errorf("no board found for project %s", projectID)
}

// DeleteBoard 删除看板
func (m *KanbanManager) DeleteBoard(boardID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.boards[boardID]; !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	delete(m.boards, boardID)
	return nil
}

// UpdateBoardColumns 更新看板列
func (m *KanbanManager) UpdateBoardColumns(boardID string, columns []KanbanColumn) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	board.Columns = columns
	return nil
}

// AddColumn 添加看板列
func (m *KanbanManager) AddColumn(boardID string, col KanbanColumn) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	if col.ID == "" {
		col.ID = fmt.Sprintf("col_%d", len(board.Columns)+1)
	}
	if col.Order == 0 {
		col.Order = len(board.Columns) + 1
	}

	board.Columns = append(board.Columns, col)
	return nil
}

// RemoveColumn 移除看板列
func (m *KanbanManager) RemoveColumn(boardID, columnID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	newColumns := []KanbanColumn{}
	for _, col := range board.Columns {
		if col.ID != columnID {
			newColumns = append(newColumns, col)
		}
	}
	board.Columns = newColumns
	return nil
}

// SetColumnWIP 设置列在制品限制
func (m *KanbanManager) SetColumnWIP(boardID, columnID string, wipLimit int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	for i, col := range board.Columns {
		if col.ID == columnID {
			board.Columns[i].WIPLimit = wipLimit
			return nil
		}
	}
	return fmt.Errorf("column %s not found", columnID)
}

// CheckWIPLimit 检查 WIP 限制
func (m *KanbanManager) CheckWIPLimit(boardID, columnID string) (bool, int, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, exists := m.boards[boardID]
	if !exists {
		return false, 0, 0, fmt.Errorf("board %s not found", boardID)
	}

	var targetCol *KanbanColumn
	for _, col := range board.Columns {
		if col.ID == columnID {
			cc := col
			targetCol = &cc
			break
		}
	}

	if targetCol == nil {
		return false, 0, 0, fmt.Errorf("column %s not found", columnID)
	}

	if targetCol.WIPLimit <= 0 {
		return false, 0, 0, nil // no limit
	}

	// 统计当前列中的任务数量
	count := 0
	tasks, _, _ := m.taskMgr.ListProjectTasks(board.ProjectID, ListOptions{
		Status:   string(targetCol.Status),
		PageSize: 1000,
	})
	count = len(tasks)

	exceeded := count >= targetCol.WIPLimit
	return exceeded, count, targetCol.WIPLimit, nil
}

// GetBoardView 获取看板视图数据（按列分组的任务）
func (m *KanbanManager) GetBoardView(boardID string) (map[string][]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	view := make(map[string][]*Task)

	for _, col := range board.Columns {
		tasks, _, _ := m.taskMgr.ListProjectTasks(board.ProjectID, ListOptions{
			Status:   string(col.Status),
			PageSize: 1000,
			SortBy:   "priority",
		})

		// 应用过滤器
		if len(board.Filters.AssigneeIDs) > 0 || len(board.Filters.Priorities) > 0 || len(board.Filters.Tags) > 0 {
			tasks = applyFilters(tasks, board.Filters)
		}

		view[col.ID] = tasks
	}

	return view, nil
}

// UpdateFilters 更新看板过滤器
func (m *KanbanManager) UpdateFilters(boardID string, filters KanbanFilters) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	board.Filters = filters
	return nil
}

// GetBoardStats 获取看板统计
func (m *KanbanManager) GetBoardStats(boardID string) (*BoardStatsExtended, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	stats := &BoardStatsExtended{
		Columns: make(map[string]int),
	}

	tasks, total, _ := m.taskMgr.ListProjectTasks(board.ProjectID, ListOptions{PageSize: 10000})
	stats.TotalTasks = total

	for _, task := range tasks {
		stats.Columns[string(task.Status)]++

		if task.Status == TaskStatusDone {
			stats.CompletedTasks++
		}

		if task.Status != TaskStatusDone && task.DueDate != nil && task.DueDate.Before(time.Now()) {
			stats.OverdueTasks++
		}
	}

	if stats.TotalTasks > 0 {
		stats.CompletionRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	return stats, nil
}

// BoardStatsExtended 扩展看板统计
type BoardStatsExtended struct {
	TotalTasks     int            `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	OverdueTasks   int            `json:"overdue_tasks"`
	CompletionRate float64        `json:"completion_rate"`
	Columns        map[string]int `json:"columns"` // status -> count
}

// applyFilters 应用过滤器
func applyFilters(tasks []*Task, filters KanbanFilters) []*Task {
	var result []*Task
	for _, task := range tasks {
		if !matchAssignee(task, filters.AssigneeIDs) {
			continue
		}
		if !matchPriority(task, filters.Priorities) {
			continue
		}
		if !matchTags(task, filters.Tags) {
			continue
		}
		result = append(result, task)
	}
	return result
}

func matchAssignee(task *Task, assigneeIDs []string) bool {
	if len(assigneeIDs) == 0 {
		return true
	}
	for _, id := range assigneeIDs {
		if task.AssigneeID == id {
			return true
		}
	}
	return false
}

func matchPriority(task *Task, priorities []string) bool {
	if len(priorities) == 0 {
		return true
	}
	for _, p := range priorities {
		if string(task.Priority) == p {
			return true
		}
	}
	return false
}

func matchTags(task *Task, tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	tagSet := make(map[string]bool)
	for _, t := range task.Tags {
		tagSet[t] = true
	}
	for _, ft := range tags {
		if tagSet[ft] {
			return true
		}
	}
	return false
}

// ListBoards 列出看板
func (m *KanbanManager) ListBoards(projectID string) []*KanbanBoard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var boards []*KanbanBoard
	for _, board := range m.boards {
		if projectID == "" || board.ProjectID == projectID {
			boards = append(boards, board)
		}
	}
	return boards
}
