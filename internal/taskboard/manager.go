package taskboard

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 错误定义.
var (
	ErrBoardNotFound   = errors.New("看板不存在")
	ErrTaskNotFound    = errors.New("任务不存在")
	ErrLabelNotFound   = errors.New("标签不存在")
	ErrLabelExists     = errors.New("标签已存在")
	ErrInvalidStatus   = errors.New("无效的状态")
	ErrInvalidProgress = errors.New("无效的进度值")
)

// Manager 看板任务管理器.
type Manager struct {
	mu     sync.RWMutex
	boards map[string]*Board
	tasks  map[string]*TaskCard
	labels map[string]*Label
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		boards: make(map[string]*Board),
		tasks:  make(map[string]*TaskCard),
		labels: make(map[string]*Label),
	}
}

// ========== 看板管理 (BoardManage) ==========

// CreateBoard 创建看板.
func (m *Manager) CreateBoard(name, description, ownerID, createdBy string) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	board := &Board{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   createdBy,
	}

	m.boards[board.ID] = board
	return board, nil
}

// GetBoard 获取看板.
func (m *Manager) GetBoard(id string) (*Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, exists := m.boards[id]
	if !exists {
		return nil, ErrBoardNotFound
	}
	return board, nil
}

// DeleteBoard 删除看板.
func (m *Manager) DeleteBoard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.boards[id]; !exists {
		return ErrBoardNotFound
	}

	// 删除关联的任务
	for _, task := range m.tasks {
		if task.BoardID == id {
			delete(m.tasks, task.ID)
		}
	}

	// 删除关联的标签
	for _, label := range m.labels {
		if label.BoardID == id {
			delete(m.labels, label.ID)
		}
	}

	delete(m.boards, id)
	return nil
}

// ListBoards 列出看板.
func (m *Manager) ListBoards(ownerID string) []*Board {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Board, 0)
	for _, board := range m.boards {
		if ownerID == "" || board.OwnerID == ownerID {
			result = append(result, board)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// ========== 任务卡片 (TaskCard) ==========

// CreateTask 创建任务.
func (m *Manager) CreateTask(boardID, title, description, createdBy string, priority TaskPriority) (*TaskCard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.boards[boardID]; !exists {
		return nil, ErrBoardNotFound
	}

	now := time.Now()
	task := &TaskCard{
		ID:          uuid.New().String(),
		BoardID:     boardID,
		Title:       title,
		Description: description,
		Status:      StatusTodo,
		Priority:    priority,
		Progress:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   createdBy,
	}

	m.tasks[task.ID] = task

	// 更新看板任务计数
	if board, exists := m.boards[boardID]; exists {
		board.TaskCount++
	}

	return task, nil
}

// GetTask 获取任务.
func (m *Manager) GetTask(id string) (*TaskCard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// UpdateTask 更新任务.
func (m *Manager) UpdateTask(id string, updates map[string]interface{}) (*TaskCard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	now := time.Now()
	task.UpdatedAt = now

	if title, ok := updates["title"].(string); ok {
		task.Title = title
	}
	if desc, ok := updates["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := updates["priority"].(TaskPriority); ok {
		task.Priority = priority
	}
	if assigneeID, ok := updates["assignee_id"].(string); ok {
		task.AssigneeID = assigneeID
	}
	if dueDate, ok := updates["due_date"].(*time.Time); ok {
		task.DueDate = dueDate
	}
	if labels, ok := updates["labels"].([]string); ok {
		task.Labels = labels
	}

	return task, nil
}

// DeleteTask 删除任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	// 更新看板任务计数
	if board, exists := m.boards[task.BoardID]; exists {
		board.TaskCount--
	}

	delete(m.tasks, id)
	return nil
}

// ListTasks 列出任务.
func (m *Manager) ListTasks(boardID string, filter TaskFilter) []*TaskCard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TaskCard, 0)
	for _, task := range m.tasks {
		if boardID != "" && task.BoardID != boardID {
			continue
		}
		if !matchesFilter(task, filter) {
			continue
		}
		result = append(result, task)
	}

	sortTasks(result, filter.OrderBy, filter.OrderDesc)

	offset := filter.Offset
	if offset > len(result) {
		offset = len(result)
	}
	end := offset + filter.Limit
	if filter.Limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end]
}

// ========== 状态流转 (StatusFlow) ==========

// TransitionTask 状态流转.
func (m *Manager) TransitionTask(id string, newStatus TaskStatus) (*TaskCard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	if !isValidTransition(task.Status, newStatus) {
		return nil, ErrInvalidStatus
	}

	now := time.Now()
	task.Status = newStatus
	task.UpdatedAt = now

	// 自动更新进度
	switch newStatus {
	case StatusTodo:
		task.Progress = 0
	case StatusInProgress:
		if task.Progress == 0 {
			task.Progress = 10
		}
	case StatusDone:
		task.Progress = 100
	}

	return task, nil
}

// isValidTransition 检查状态流转是否合法.
func isValidTransition(from, to TaskStatus) bool {
	transitions := map[TaskStatus][]TaskStatus{
		StatusTodo:       {StatusInProgress},
		StatusInProgress: {StatusTodo, StatusDone},
		StatusDone:       {StatusInProgress},
	}

	allowed, exists := transitions[from]
	if !exists {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// GetAvailableTransitions 获取可用的状态流转.
func (m *Manager) GetAvailableTransitions(taskID string) ([]TaskStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	transitions := map[TaskStatus][]TaskStatus{
		StatusTodo:       {StatusInProgress},
		StatusInProgress: {StatusTodo, StatusDone},
		StatusDone:       {StatusInProgress},
	}

	return transitions[task.Status], nil
}

// ========== 标签系统 (Label) ==========

// CreateLabel 创建标签.
func (m *Manager) CreateLabel(boardID, name, color string) (*Label, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.boards[boardID]; !exists {
		return nil, ErrBoardNotFound
	}

	// 检查同名标签
	for _, label := range m.labels {
		if label.BoardID == boardID && label.Name == name {
			return nil, ErrLabelExists
		}
	}

	label := &Label{
		ID:        uuid.New().String(),
		BoardID:   boardID,
		Name:      name,
		Color:     color,
		CreatedAt: time.Now(),
	}

	m.labels[label.ID] = label
	return label, nil
}

// GetLabel 获取标签.
func (m *Manager) GetLabel(id string) (*Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	label, exists := m.labels[id]
	if !exists {
		return nil, ErrLabelNotFound
	}
	return label, nil
}

// DeleteLabel 删除标签.
func (m *Manager) DeleteLabel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	label, exists := m.labels[id]
	if !exists {
		return ErrLabelNotFound
	}

	// 从所有任务中移除该标签
	for _, task := range m.tasks {
		if task.BoardID == label.BoardID {
			task.Labels = removeLabel(task.Labels, label.Name)
		}
	}

	delete(m.labels, id)
	return nil
}

// ListLabels 列出看板标签.
func (m *Manager) ListLabels(boardID string) []*Label {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Label, 0)
	for _, label := range m.labels {
		if label.BoardID == boardID {
			result = append(result, label)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// AddLabelToTask 给任务添加标签.
func (m *Manager) AddLabelToTask(taskID, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	label, exists := m.labels[labelID]
	if !exists {
		return ErrLabelNotFound
	}

	if task.BoardID != label.BoardID {
		return ErrLabelNotFound
	}

	// 检查是否已存在
	for _, l := range task.Labels {
		if l == label.Name {
			return nil
		}
	}

	task.Labels = append(task.Labels, label.Name)
	task.UpdatedAt = time.Now()
	return nil
}

// RemoveLabelFromTask 从任务移除标签.
func (m *Manager) RemoveLabelFromTask(taskID, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	label, exists := m.labels[labelID]
	if !exists {
		return ErrLabelNotFound
	}

	task.Labels = removeLabel(task.Labels, label.Name)
	task.UpdatedAt = time.Now()
	return nil
}

// ========== 进度追踪 (Progress) ==========

// UpdateProgress 更新任务进度.
func (m *Manager) UpdateProgress(taskID string, progress int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if progress < 0 || progress > 100 {
		return ErrInvalidProgress
	}

	task, exists := m.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	now := time.Now()
	task.Progress = progress
	task.UpdatedAt = now

	// 自动更新状态
	if progress == 100 && task.Status != StatusDone {
		task.Status = StatusDone
	} else if progress > 0 && progress < 100 && task.Status == StatusTodo {
		task.Status = StatusInProgress
	}

	return nil
}

// GetBoardStats 获取看板统计.
func (m *Manager) GetBoardStats(boardID string) (*BoardStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.boards[boardID]; !exists {
		return nil, ErrBoardNotFound
	}

	stats := &BoardStats{
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
	}

	now := time.Now()
	totalProgress := 0

	for _, task := range m.tasks {
		if task.BoardID != boardID {
			continue
		}

		stats.TotalTasks++
		stats.ByStatus[string(task.Status)]++
		stats.ByPriority[string(task.Priority)]++
		totalProgress += task.Progress

		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != StatusDone {
			stats.OverdueTasks++
		}
	}

	if stats.TotalTasks > 0 {
		stats.AvgProgress = float64(totalProgress) / float64(stats.TotalTasks)
	}

	return stats, nil
}

// GetTaskProgress 获取任务进度.
func (m *Manager) GetTaskProgress(taskID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return 0, ErrTaskNotFound
	}

	return task.Progress, nil
}

// ========== 辅助函数 ==========

func matchesFilter(task *TaskCard, filter TaskFilter) bool {
	if len(filter.Status) > 0 {
		found := false
		for _, s := range filter.Status {
			if s == task.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(filter.Priority) > 0 {
		found := false
		for _, p := range filter.Priority {
			if p == task.Priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if filter.AssigneeID != "" && task.AssigneeID != filter.AssigneeID {
		return false
	}

	if len(filter.Labels) > 0 {
		for _, fl := range filter.Labels {
			found := false
			for _, tl := range task.Labels {
				if tl == fl {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		title := strings.ToLower(task.Title)
		desc := strings.ToLower(task.Description)
		if !strings.Contains(title, search) && !strings.Contains(desc, search) {
			return false
		}
	}

	return true
}

func removeLabel(labels []string, name string) []string {
	result := make([]string, 0, len(labels))
	for _, l := range labels {
		if l != name {
			result = append(result, l)
		}
	}
	return result
}

func sortTasks(tasks []*TaskCard, orderBy string, desc bool) {
	sort.Slice(tasks, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "priority":
			less = comparePriority(tasks[i].Priority, tasks[j].Priority) < 0
		case "due_date":
			if tasks[i].DueDate == nil && tasks[j].DueDate == nil {
				less = false
			} else if tasks[i].DueDate == nil {
				less = true
			} else if tasks[j].DueDate == nil {
				less = false
			} else {
				less = tasks[i].DueDate.Before(*tasks[j].DueDate)
			}
		case "status":
			less = compareStatus(tasks[i].Status, tasks[j].Status) < 0
		default:
			less = tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		}
		if desc {
			return !less
		}
		return less
	})
}

func comparePriority(a, b TaskPriority) int {
	priorityOrder := map[TaskPriority]int{
		PriorityUrgent: 4,
		PriorityHigh:   3,
		PriorityMedium: 2,
		PriorityLow:    1,
	}
	return priorityOrder[a] - priorityOrder[b]
}

func compareStatus(a, b TaskStatus) int {
	statusOrder := map[TaskStatus]int{
		StatusTodo:       1,
		StatusInProgress: 2,
		StatusDone:       3,
	}
	return statusOrder[a] - statusOrder[b]
}
