package projectcenter

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// MilestoneManager 里程碑管理器
type MilestoneManager struct {
	mu         sync.RWMutex
	milestones map[string]*Milestone
	taskMgr    *TaskManager
	nextID     int
}

// NewMilestoneManager 创建里程碑管理器
func NewMilestoneManager(taskMgr *TaskManager) *MilestoneManager {
	return &MilestoneManager{
		milestones: make(map[string]*Milestone),
		taskMgr:    taskMgr,
		nextID:     1,
	}
}

// CreateMilestone 创建里程碑
func (m *MilestoneManager) CreateMilestone(projectID string, req CreateMilestoneRequest) (*Milestone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("milestone name is required")
	}

	id := fmt.Sprintf("ms_%d", m.nextID)
	m.nextID++

	now := time.Now()
	milestone := &Milestone{
		ID:          id,
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		DueDate:     req.DueDate,
		TaskIDs:     []string{},
		Progress:    0,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.milestones[id] = milestone
	return milestone, nil
}

// GetMilestone 获取里程碑
func (m *MilestoneManager) GetMilestone(milestoneID string) (*Milestone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return nil, fmt.Errorf("milestone %s not found", milestoneID)
	}
	return ms, nil
}

// UpdateMilestone 更新里程碑
func (m *MilestoneManager) UpdateMilestone(milestoneID string, name, description string, dueDate *time.Time) (*Milestone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return nil, fmt.Errorf("milestone %s not found", milestoneID)
	}

	if name != "" {
		ms.Name = name
	}
	if description != "" {
		ms.Description = description
	}
	if dueDate != nil {
		ms.DueDate = dueDate
	}

	ms.UpdatedAt = time.Now()
	return ms, nil
}

// DeleteMilestone 删除里程碑
func (m *MilestoneManager) DeleteMilestone(milestoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.milestones[milestoneID]; !exists {
		return fmt.Errorf("milestone %s not found", milestoneID)
	}

	delete(m.milestones, milestoneID)
	return nil
}

// AddTaskToMilestone 添加任务到里程碑
func (m *MilestoneManager) AddTaskToMilestone(milestoneID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return fmt.Errorf("milestone %s not found", milestoneID)
	}

	// 检查任务是否存在
	if _, err := m.taskMgr.GetTask(taskID); err != nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	// 避免重复添加
	for _, id := range ms.TaskIDs {
		if id == taskID {
			return nil
		}
	}

	ms.TaskIDs = append(ms.TaskIDs, taskID)
	ms.UpdatedAt = time.Now()

	// 更新里程碑状态
	m.updateMilestoneStatus(ms)
	return nil
}

// RemoveTaskFromMilestone 从里程碑移除任务
func (m *MilestoneManager) RemoveTaskFromMilestone(milestoneID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return fmt.Errorf("milestone %s not found", milestoneID)
	}

	newTaskIDs := []string{}
	for _, id := range ms.TaskIDs {
		if id != taskID {
			newTaskIDs = append(newTaskIDs, id)
		}
	}
	ms.TaskIDs = newTaskIDs
	ms.UpdatedAt = time.Now()

	// 更新里程碑状态
	m.updateMilestoneStatus(ms)
	return nil
}

// CompleteMilestone 标记里程碑完成
func (m *MilestoneManager) CompleteMilestone(milestoneID string) (*Milestone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return nil, fmt.Errorf("milestone %s not found", milestoneID)
	}

	now := time.Now()
	ms.CompletedAt = &now
	ms.Status = "completed"
	ms.Progress = 100
	ms.UpdatedAt = now

	return ms, nil
}

// RefreshProgress 刷新里程碑进度
func (m *MilestoneManager) RefreshProgress(milestoneID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return 0, fmt.Errorf("milestone %s not found", milestoneID)
	}

	m.updateMilestoneStatus(ms)
	return ms.Progress, nil
}

// updateMilestoneStatus 内部：更新里程碑状态和进度
func (m *MilestoneManager) updateMilestoneStatus(ms *Milestone) {
	if len(ms.TaskIDs) == 0 {
		ms.Progress = 0
		ms.Status = "pending"
		return
	}

	completed := 0
	inProgress := 0
	for _, taskID := range ms.TaskIDs {
		task, err := m.taskMgr.GetTask(taskID)
		if err != nil {
			continue
		}
		switch task.Status {
		case TaskStatusDone:
			completed++
		case TaskStatusInProgress, TaskStatusReview:
			inProgress++
		}
	}

	total := len(ms.TaskIDs)
	ms.Progress = float64(completed) / float64(total) * 100

	if completed == total {
		now := time.Now()
		ms.Status = "completed"
		ms.CompletedAt = &now
	} else if inProgress > 0 {
		ms.Status = "in_progress"
	} else {
		ms.Status = "pending"
	}

	// 检查是否过期
	if ms.DueDate != nil && ms.DueDate.Before(time.Now()) && ms.Status != "completed" {
		ms.Status = "overdue"
	}
}

// ListProjectMilestones 列出项目里程碑
func (m *MilestoneManager) ListProjectMilestones(projectID string) []*Milestone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var milestones []*Milestone
	for _, ms := range m.milestones {
		if ms.ProjectID == projectID {
			milestones = append(milestones, ms)
		}
	}

	sort.Slice(milestones, func(i, j int) bool {
		if milestones[i].DueDate == nil {
			return false
		}
		if milestones[j].DueDate == nil {
			return true
		}
		return milestones[i].DueDate.Before(*milestones[j].DueDate)
	})

	return milestones
}

// GetMilestoneTasks 获取里程碑关联的任务
func (m *MilestoneManager) GetMilestoneTasks(milestoneID string) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return nil, fmt.Errorf("milestone %s not found", milestoneID)
	}

	var tasks []*Task
	for _, taskID := range ms.TaskIDs {
		if task, err := m.taskMgr.GetTask(taskID); err == nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// GetMilestoneProgress 获取里程碑进度详情
func (m *MilestoneManager) GetMilestoneProgress(milestoneID string) (*MilestoneProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ms, exists := m.milestones[milestoneID]
	if !exists {
		return nil, fmt.Errorf("milestone %s not found", milestoneID)
	}

	progress := &MilestoneProgress{
		MilestoneID:    milestoneID,
		Name:           ms.Name,
		TotalTasks:     len(ms.TaskIDs),
		Status:         ms.Status,
		OverallProgress: ms.Progress,
		TasksByStatus:  make(map[string]int),
	}

	if ms.DueDate != nil {
		progress.DueDate = ms.DueDate
		daysRemaining := int(time.Until(*ms.DueDate).Hours() / 24)
		if daysRemaining < 0 {
			daysRemaining = 0
		}
		progress.DaysRemaining = daysRemaining
	}

	for _, taskID := range ms.TaskIDs {
		task, err := m.taskMgr.GetTask(taskID)
		if err != nil {
			continue
		}
		progress.TasksByStatus[string(task.Status)]++
	}

	return progress, nil
}

// MilestoneProgress 里程碑进度详情
type MilestoneProgress struct {
	MilestoneID     string         `json:"milestone_id"`
	Name            string         `json:"name"`
	TotalTasks      int            `json:"total_tasks"`
	Status          string         `json:"status"`
	OverallProgress float64        `json:"overall_progress"`
	DueDate         *time.Time     `json:"due_date,omitempty"`
	DaysRemaining   int            `json:"days_remaining"`
	TasksByStatus   map[string]int `json:"tasks_by_status"`
}

// GetTimeline 获取里程碑时间线
func (m *MilestoneManager) GetTimeline(projectID string) []*MilestoneTimelineItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var items []*MilestoneTimelineItem
	for _, ms := range m.milestones {
		if ms.ProjectID != projectID {
			continue
		}

		item := &MilestoneTimelineItem{
			ID:       ms.ID,
			Name:     ms.Name,
			Status:   ms.Status,
			Progress: ms.Progress,
			DueDate:  ms.DueDate,
		}

		// 计算开始日期（最早任务的开始日期）
		var earliestStart *time.Time
		for _, taskID := range ms.TaskIDs {
			task, err := m.taskMgr.GetTask(taskID)
			if err != nil {
				continue
			}
			if task.StartDate != nil {
				if earliestStart == nil || task.StartDate.Before(*earliestStart) {
					earliestStart = task.StartDate
				}
			}
		}
		item.StartDate = earliestStart

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].StartDate == nil {
			return false
		}
		if items[j].StartDate == nil {
			return true
		}
		return items[i].StartDate.Before(*items[j].StartDate)
	})

	return items
}

// MilestoneTimelineItem 里程碑时间线项
type MilestoneTimelineItem struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Status   string     `json:"status"`
	Progress float64    `json:"progress"`
	StartDate *time.Time `json:"start_date,omitempty"`
	DueDate  *time.Time `json:"due_date,omitempty"`
}
