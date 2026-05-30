// Package sprintboard 提供敏捷看板核心管理逻辑
package sprintboard

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 看板管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	boards    map[string]*Board
	sprints   map[string]*Sprint
	tasks     map[string]*Task
	stopChan  chan struct{}
	running   bool
}

// NewManager 创建看板管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Manager{
		logger:   logger,
		boards:   make(map[string]*Board),
		sprints:  make(map[string]*Sprint),
		tasks:    make(map[string]*Task),
		stopChan: make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateBoard 创建看板
func (m *Manager) CreateBoard(req *CreateBoardRequest) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaultColumns := []Column{
		{ID: generateID(), Name: "待办", Status: TaskStatusTodo, Position: 0},
		{ID: generateID(), Name: "进行中", Status: TaskStatusInProgress, Position: 1},
		{ID: generateID(), Name: "评审", Status: TaskStatusReview, Position: 2},
		{ID: generateID(), Name: "完成", Status: TaskStatusDone, Position: 3},
	}

	board := &Board{
		ID:          generateID(),
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Columns:     defaultColumns,
		OwnerID:     req.OwnerID,
		MemberIDs:   []string{req.OwnerID},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.boards[board.ID] = board

	m.logger.Info("board created",
		zap.String("board_id", board.ID),
		zap.String("name", board.Name),
		zap.String("type", string(board.Type)))

	return board, nil
}

// GetBoard 获取看板
func (m *Manager) GetBoard(id string) (*Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[id]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", id)
	}
	return board, nil
}

// ListBoards 列出所有看板
func (m *Manager) ListBoards() []*Board {
	m.mu.RLock()
	defer m.mu.RUnlock()

	boards := make([]*Board, 0, len(m.boards))
	for _, b := range m.boards {
		boards = append(boards, b)
	}
	return boards
}

// DeleteBoard 删除看板
func (m *Manager) DeleteBoard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.boards[id]; !ok {
		return fmt.Errorf("board not found: %s", id)
	}

	// 删除关联的 sprint 和 task
	for sid, sprint := range m.sprints {
		if sprint.BoardID == id {
			delete(m.sprints, sid)
		}
	}
	for tid, task := range m.tasks {
		if task.BoardID == id {
			delete(m.tasks, tid)
		}
	}

	delete(m.boards, id)
	return nil
}

// CreateSprint 创建 Sprint
func (m *Manager) CreateSprint(req *CreateSprintRequest) (*Sprint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.boards[req.BoardID]; !ok {
		return nil, fmt.Errorf("board not found: %s", req.BoardID)
	}

	if req.EndDate.Before(req.StartDate) {
		return nil, fmt.Errorf("end date must be after start date")
	}

	sprint := &Sprint{
		ID:        generateID(),
		BoardID:   req.BoardID,
		Name:      req.Name,
		Goal:      req.Goal,
		Status:    SprintStatusPlanning,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Capacity:  req.Capacity,
		Tasks:     make([]*Task, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.sprints[sprint.ID] = sprint

	m.logger.Info("sprint created",
		zap.String("sprint_id", sprint.ID),
		zap.String("name", sprint.Name))

	return sprint, nil
}

// GetSprint 获取 Sprint
func (m *Manager) GetSprint(id string) (*Sprint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sprint, ok := m.sprints[id]
	if !ok {
		return nil, fmt.Errorf("sprint not found: %s", id)
	}
	return sprint, nil
}

// ListSprints 列出看板的所有 Sprint
func (m *Manager) ListSprints(boardID string) []*Sprint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sprints := make([]*Sprint, 0)
	for _, s := range m.sprints {
		if boardID == "" || s.BoardID == boardID {
			sprints = append(sprints, s)
		}
	}

	sort.Slice(sprints, func(i, j int) bool {
		return sprints[i].StartDate.After(sprints[j].StartDate)
	})

	return sprints
}

// StartSprint 启动 Sprint
func (m *Manager) StartSprint(id string) (*Sprint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sprint, ok := m.sprints[id]
	if !ok {
		return nil, fmt.Errorf("sprint not found: %s", id)
	}

	if sprint.Status != SprintStatusPlanning {
		return nil, fmt.Errorf("sprint must be in planning status to start, current: %s", sprint.Status)
	}

	// 检查是否有其他活跃 Sprint
	for _, s := range m.sprints {
		if s.BoardID == sprint.BoardID && s.Status == SprintStatusActive {
			return nil, fmt.Errorf("board already has an active sprint: %s", s.ID)
		}
	}

	sprint.Status = SprintStatusActive
	sprint.UpdatedAt = time.Now()

	// 更新任务状态
	for _, task := range m.tasks {
		if task.SprintID == id && task.Status == TaskStatusBacklog {
			task.Status = TaskStatusTodo
			task.UpdatedAt = time.Now()
		}
	}

	m.logger.Info("sprint started", zap.String("sprint_id", id))
	return sprint, nil
}

// CompleteSprint 完成 Sprint
func (m *Manager) CompleteSprint(id string) (*Sprint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sprint, ok := m.sprints[id]
	if !ok {
		return nil, fmt.Errorf("sprint not found: %s", id)
	}

	if sprint.Status != SprintStatusActive {
		return nil, fmt.Errorf("sprint must be active to complete, current: %s", sprint.Status)
	}

	sprint.Status = SprintStatusComplete
	sprint.UpdatedAt = time.Now()

	// 将未完成的任务移回 backlog
	for _, task := range m.tasks {
		if task.SprintID == id && task.Status != TaskStatusDone {
			task.SprintID = ""
			task.Status = TaskStatusBacklog
			task.UpdatedAt = time.Now()
		}
	}

	m.logger.Info("sprint completed", zap.String("sprint_id", id))
	return sprint, nil
}

// AddTask 添加任务
func (m *Manager) AddTask(req *CreateTaskRequest) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.boards[req.BoardID]; !ok {
		return nil, fmt.Errorf("board not found: %s", req.BoardID)
	}

	if req.SprintID != "" {
		if _, ok := m.sprints[req.SprintID]; !ok {
			return nil, fmt.Errorf("sprint not found: %s", req.SprintID)
		}
	}

	if req.Type == "" {
		req.Type = TaskTypeTask
	}
	if req.Priority == "" {
		req.Priority = PriorityMedium
	}

	status := TaskStatusBacklog
	if req.SprintID != "" {
		status = TaskStatusTodo
	}

	task := &Task{
		ID:          generateID(),
		BoardID:     req.BoardID,
		SprintID:    req.SprintID,
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Status:      status,
		Priority:    req.Priority,
		AssigneeID:  req.AssigneeID,
		StoryPoints: req.StoryPoints,
		Tags:        req.Tags,
		SwimLaneID:  req.SwimLaneID,
		ParentID:    req.ParentID,
		DueDate:     req.DueDate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.tasks[task.ID] = task

	// 更新 Sprint 的任务列表
	if task.SprintID != "" {
		if sprint, ok := m.sprints[task.SprintID]; ok {
			sprint.Tasks = append(sprint.Tasks, task)
			sprint.UpdatedAt = time.Now()
		}
	}

	m.logger.Info("task created",
		zap.String("task_id", task.ID),
		zap.String("title", task.Title))

	return task, nil
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// ListTasks 列出任务
func (m *Manager) ListTasks(boardID, sprintID string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, t := range m.tasks {
		if boardID != "" && t.BoardID != boardID {
			continue
		}
		if sprintID != "" && t.SprintID != sprintID {
			continue
		}
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// MoveTask 移动任务到新状态
func (m *Manager) MoveTask(taskID string, req *MoveTaskRequest) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	oldStatus := task.Status
	task.Status = req.TargetStatus
	task.UpdatedAt = time.Now()

	if req.SwimLaneID != "" {
		task.SwimLaneID = req.SwimLaneID
	}

	// 如果移动到完成状态，记录完成时间
	if req.TargetStatus == TaskStatusDone && task.CompletedAt == nil {
		now := time.Now()
		task.CompletedAt = &now
	}

	// 如果从完成状态移出，清除完成时间
	if req.TargetStatus != TaskStatusDone {
		task.CompletedAt = nil
	}

	m.logger.Info("task moved",
		zap.String("task_id", taskID),
		zap.String("from", string(oldStatus)),
		zap.String("to", string(req.TargetStatus)))

	return task, nil
}

// UpdateTask 更新任务
func (m *Manager) UpdateTask(id string, req *CreateTaskRequest) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	task.Title = req.Title
	task.Description = req.Description
	task.Type = req.Type
	task.Priority = req.Priority
	task.AssigneeID = req.AssigneeID
	task.StoryPoints = req.StoryPoints
	task.Tags = req.Tags
	task.SwimLaneID = req.SwimLaneID
	task.DueDate = req.DueDate
	task.UpdatedAt = time.Now()

	// 如果 Sprint 变更
	if req.SprintID != task.SprintID {
		// 从旧 Sprint 移除
		if task.SprintID != "" {
			if oldSprint, ok := m.sprints[task.SprintID]; ok {
				for i, t := range oldSprint.Tasks {
					if t.ID == id {
						oldSprint.Tasks = append(oldSprint.Tasks[:i], oldSprint.Tasks[i+1:]...)
						break
					}
				}
			}
		}

		task.SprintID = req.SprintID

		// 添加到新 Sprint
		if task.SprintID != "" {
			if newSprint, ok := m.sprints[task.SprintID]; ok {
				newSprint.Tasks = append(newSprint.Tasks, task)
			}
		}
	}

	return task, nil
}

// DeleteTask 删除任务
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	// 从 Sprint 中移除
	if task.SprintID != "" {
		if sprint, ok := m.sprints[task.SprintID]; ok {
			for i, t := range sprint.Tasks {
				if t.ID == id {
					sprint.Tasks = append(sprint.Tasks[:i], sprint.Tasks[i+1:]...)
					break
				}
			}
		}
	}

	delete(m.tasks, id)
	return nil
}

// GetMetrics 获取 Sprint 指标
func (m *Manager) GetMetrics(sprintID string) (*SprintMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sprint, ok := m.sprints[sprintID]
	if !ok {
		return nil, fmt.Errorf("sprint not found: %s", sprintID)
	}

	tasksByStatus := make(map[string]int)
	tasksByPriority := make(map[string]int)
	tasksByAssignee := make(map[string]int)

	totalPoints := 0
	completedPoints := 0
	completedTasks := 0
	overdueTasks := 0
	blockedTasks := 0
	now := time.Now()

	for _, task := range m.tasks {
		if task.SprintID != sprintID {
			continue
		}

		totalPoints += task.StoryPoints
		tasksByStatus[string(task.Status)]++
		tasksByPriority[string(task.Priority)]++

		if task.AssigneeID != "" {
			tasksByAssignee[task.AssigneeID]++
		}

		if task.Status == TaskStatusDone {
			completedTasks++
			completedPoints += task.StoryPoints
		}

		if task.Status == TaskStatusBlocked {
			blockedTasks++
		}

		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			overdueTasks++
		}
	}

	totalTasks := len(sprint.Tasks)
	progress := float64(0)
	if totalPoints > 0 {
		progress = float64(completedPoints) / float64(totalPoints) * 100
	}

	daysTotal := int(sprint.EndDate.Sub(sprint.StartDate).Hours() / 24)
	daysElapsed := int(now.Sub(sprint.StartDate).Hours() / 24)
	daysRemaining := daysTotal - daysElapsed

	if daysRemaining < 0 {
		daysRemaining = 0
	}
	if daysElapsed < 0 {
		daysElapsed = 0
	}

	velocity := float64(0)
	if daysElapsed > 0 {
		velocity = float64(completedPoints) / float64(daysElapsed)
	}

	return &SprintMetrics{
		SprintID:        sprintID,
		SprintName:      sprint.Name,
		TotalTasks:      totalTasks,
		CompletedTasks:  completedTasks,
		TotalPoints:     totalPoints,
		CompletedPoints: completedPoints,
		Velocity:        velocity,
		Progress:        progress,
		DaysRemaining:   daysRemaining,
		DaysElapsed:     daysElapsed,
		TasksByStatus:   tasksByStatus,
		TasksByPriority: tasksByPriority,
		TasksByAssignee: tasksByAssignee,
		OverdueTasks:    overdueTasks,
		BlockedTasks:    blockedTasks,
	}, nil
}

// GenerateBurndown 生成燃尽图数据
func (m *Manager) GenerateBurndown(sprintID string) ([]BurndownDay, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sprint, ok := m.sprints[sprintID]
	if !ok {
		return nil, fmt.Errorf("sprint not found: %s", sprintID)
	}

	// 计算总故事点
	totalPoints := 0
	for _, task := range m.tasks {
		if task.SprintID == sprintID {
			totalPoints += task.StoryPoints
		}
	}

	daysTotal := int(sprint.EndDate.Sub(sprint.StartDate).Hours() / 24)
	if daysTotal <= 0 {
		daysTotal = 1
	}

	idealPointsPerDay := float64(totalPoints) / float64(daysTotal)

	// 收集任务完成时间
	completionByDay := make(map[string]struct {
		TasksCompleted  int
		PointsCompleted int
	})

	for _, task := range m.tasks {
		if task.SprintID == sprintID && task.CompletedAt != nil {
			dateKey := task.CompletedAt.Format("2006-01-02")
			entry := completionByDay[dateKey]
			entry.TasksCompleted++
			entry.PointsCompleted += task.StoryPoints
			completionByDay[dateKey] = entry
		}
	}

	// 生成每日数据
	burndown := make([]BurndownDay, 0, daysTotal+1)
	remainingPoints := totalPoints

	for i := 0; i <= daysTotal; i++ {
		date := sprint.StartDate.AddDate(0, 0, i)
		dateKey := date.Format("2006-01-02")

		if completion, ok := completionByDay[dateKey]; ok {
			remainingPoints -= completion.PointsCompleted
		}

		if remainingPoints < 0 {
			remainingPoints = 0
		}

		ideal := int(math.Round(float64(totalPoints) - idealPointsPerDay*float64(i)))
		if ideal < 0 {
			ideal = 0
		}

		completion := completionByDay[dateKey]

		burndown = append(burndown, BurndownDay{
			Date:             date,
			RemainingPoints:  remainingPoints,
			IdealPoints:      ideal,
			TasksCompleted:   completion.TasksCompleted,
			PointsCompleted:  completion.PointsCompleted,
		})
	}

	return burndown, nil
}

// AddSwimLane 添加泳道
func (m *Manager) AddSwimLane(boardID string, req *CreateSwimLaneRequest) (*SwimLane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	lane := &SwimLane{
		ID:          generateID(),
		BoardID:     boardID,
		Name:        req.Name,
		Description: req.Description,
		Position:    len(board.SwimLanes),
		CreatedAt:   time.Now(),
	}

	board.SwimLanes = append(board.SwimLanes, lane)
	board.UpdatedAt = time.Now()

	return lane, nil
}

// RemoveSwimLane 移除泳道
func (m *Manager) RemoveSwimLane(boardID, laneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	for i, lane := range board.SwimLanes {
		if lane.ID == laneID {
			board.SwimLanes = append(board.SwimLanes[:i], board.SwimLanes[i+1:]...)
			board.UpdatedAt = time.Now()

			// 清除关联任务的泳道
			for _, task := range m.tasks {
				if task.SwimLaneID == laneID {
					task.SwimLaneID = ""
					task.UpdatedAt = time.Now()
				}
			}
			return nil
		}
	}

	return fmt.Errorf("swim lane not found: %s", laneID)
}
