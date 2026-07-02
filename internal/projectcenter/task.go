package projectcenter

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// TaskManager 任务管理器.
type TaskManager struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	comments map[string][]*Comment // taskID -> comments
	nextID   int
}

// NewTaskManager 创建任务管理器.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks:    make(map[string]*Task),
		comments: make(map[string][]*Comment),
		nextID:   1,
	}
}

// CreateTask 创建任务.
func (m *TaskManager) CreateTask(projectID string, req CreateTaskRequest, reporterID string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Title == "" {
		return nil, fmt.Errorf("task title is required")
	}

	id := fmt.Sprintf("task_%d", m.nextID)
	m.nextID++

	now := time.Now()
	task := &Task{
		ID:            id,
		ProjectID:     projectID,
		Title:         req.Title,
		Description:   req.Description,
		Status:        TaskStatusTodo,
		Priority:      req.Priority,
		AssigneeID:    req.AssigneeID,
		ReporterID:    reporterID,
		Tags:          req.Tags,
		ParentTaskID:  req.ParentTaskID,
		SubtaskIDs:    []string{},
		Dependencies:  req.Dependencies,
		EstimateHours: req.EstimateHours,
		StartDate:     req.StartDate,
		DueDate:       req.DueDate,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if task.Priority == "" {
		task.Priority = PriorityMedium
	}

	m.tasks[id] = task

	// 如果有父任务，添加到父任务的子任务列表
	if req.ParentTaskID != "" {
		if parent, exists := m.tasks[req.ParentTaskID]; exists {
			parent.SubtaskIDs = append(parent.SubtaskIDs, id)
			parent.UpdatedAt = now
		}
	}

	return task, nil
}

// GetTask 获取任务.
func (m *TaskManager) GetTask(taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// UpdateTask 更新任务.
func (m *TaskManager) UpdateTask(taskID string, req UpdateTaskRequest) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	if req.Title != "" {
		task.Title = req.Title
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.Status != "" {
		oldStatus := task.Status
		task.Status = req.Status
		if req.Status == TaskStatusDone && oldStatus != TaskStatusDone {
			now := time.Now()
			task.CompletedAt = &now
		}
	}
	if req.Priority != "" {
		task.Priority = req.Priority
	}
	if req.AssigneeID != "" {
		task.AssigneeID = req.AssigneeID
	}
	if req.Tags != nil {
		task.Tags = req.Tags
	}
	if req.Dependencies != nil {
		task.Dependencies = req.Dependencies
	}
	if req.EstimateHours != nil {
		task.EstimateHours = *req.EstimateHours
	}
	if req.ActualHours != nil {
		task.ActualHours = *req.ActualHours
	}
	if req.StartDate != nil {
		task.StartDate = req.StartDate
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}

	task.UpdatedAt = time.Now()
	return task, nil
}

// DeleteTask 删除任务.
func (m *TaskManager) DeleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// 删除所有子任务
	for _, subtaskID := range task.SubtaskIDs {
		delete(m.tasks, subtaskID)
		delete(m.comments, subtaskID)
	}

	// 从父任务中移除
	if task.ParentTaskID != "" {
		if parent, exists := m.tasks[task.ParentTaskID]; exists {
			newSubtasks := []string{}
			for _, id := range parent.SubtaskIDs {
				if id != taskID {
					newSubtasks = append(newSubtasks, id)
				}
			}
			parent.SubtaskIDs = newSubtasks
		}
	}

	delete(m.tasks, taskID)
	delete(m.comments, taskID)
	return nil
}

// MoveTask 移动任务状态.
func (m *TaskManager) MoveTask(taskID string, req MoveTaskRequest) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	oldStatus := task.Status
	task.Status = req.Status
	task.UpdatedAt = time.Now()

	if req.Status == TaskStatusDone && oldStatus != TaskStatusDone {
		task.CompletedAt = &task.UpdatedAt
	}

	return task, nil
}

// ListProjectTasks 列出项目任务.
func (m *TaskManager) ListProjectTasks(projectID string, opts ListOptions) ([]*Task, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, task := range m.tasks {
		if task.ProjectID == projectID {
			if opts.Status == "" || string(task.Status) == opts.Status {
				tasks = append(tasks, task)
			}
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		switch opts.SortBy {
		case "priority":
			return priorityOrder(tasks[i].Priority) > priorityOrder(tasks[j].Priority)
		case "due_date":
			if tasks[i].DueDate == nil {
				return false
			}
			if tasks[j].DueDate == nil {
				return true
			}
			return tasks[i].DueDate.Before(*tasks[j].DueDate)
		case "created_at":
			if opts.Order == "desc" {
				return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
			}
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		default:
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		}
	})

	total := len(tasks)

	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}

	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize
	if start > total {
		return []*Task{}, total, nil
	}
	if end > total {
		end = total
	}

	return tasks[start:end], total, nil
}

// ListTasksByAssignee 列出分配给某用户的任务.
func (m *TaskManager) ListTasksByAssignee(assigneeID string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, task := range m.tasks {
		if task.AssigneeID == assigneeID && task.Status != TaskStatusDone {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetOverdueTasks 获取过期任务.
func (m *TaskManager) GetOverdueTasks(projectID string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	now := time.Now()
	for _, task := range m.tasks {
		if task.ProjectID == projectID &&
			task.Status != TaskStatusDone &&
			task.DueDate != nil &&
			task.DueDate.Before(now) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// AddComment 添加评论.
func (m *TaskManager) AddComment(taskID, userID string, req CreateCommentRequest) (*Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	mentions := extractMentions(req.Content)

	comment := &Comment{
		ID:        fmt.Sprintf("comment_%d_%s", time.Now().UnixNano(), taskID),
		TaskID:    taskID,
		UserID:    userID,
		Content:   req.Content,
		Mentions:  mentions,
		ParentID:  req.ParentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.comments[taskID] = append(m.comments[taskID], comment)
	return comment, nil
}

// GetComments 获取任务评论.
func (m *TaskManager) GetComments(taskID string) ([]*Comment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.tasks[taskID]; !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	comments := m.comments[taskID]
	if comments == nil {
		return []*Comment{}, nil
	}
	return comments, nil
}

// GetSubtasks 获取子任务.
func (m *TaskManager) GetSubtasks(taskID string) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	var subtasks []*Task
	for _, subID := range task.SubtaskIDs {
		if sub, exists := m.tasks[subID]; exists {
			subtasks = append(subtasks, sub)
		}
	}
	return subtasks, nil
}

// GetTaskProgress 计算任务进度（基于子任务完成比例）.
func (m *TaskManager) GetTaskProgress(taskID string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return 0, fmt.Errorf("task %s not found", taskID)
	}

	if len(task.SubtaskIDs) == 0 {
		if task.Status == TaskStatusDone {
			return 100, nil
		}
		return 0, nil
	}

	completed := 0
	for _, subID := range task.SubtaskIDs {
		if sub, exists := m.tasks[subID]; exists && sub.Status == TaskStatusDone {
			completed++
		}
	}

	return float64(completed) / float64(len(task.SubtaskIDs)) * 100, nil
}

// GetDependencyChain 获取任务依赖链.
func (m *TaskManager) GetDependencyChain(taskID string) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	visited := make(map[string]bool)
	var chain []*Task
	var walk func(t *Task)
	walk = func(t *Task) {
		for _, depID := range t.Dependencies {
			if visited[depID] {
				continue
			}
			visited[depID] = true
			if dep, exists := m.tasks[depID]; exists {
				chain = append(chain, dep)
				walk(dep)
			}
		}
	}

	walk(task)
	return chain, nil
}

// CheckDependencyBlocked 检查任务是否因依赖阻塞.
func (m *TaskManager) CheckDependencyBlocked(taskID string) (bool, []string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return false, nil, fmt.Errorf("task %s not found", taskID)
	}

	var blockedBy []string
	for _, depID := range task.Dependencies {
		if dep, exists := m.tasks[depID]; exists {
			if dep.Status != TaskStatusDone {
				blockedBy = append(blockedBy, depID)
			}
		}
	}

	return len(blockedBy) > 0, blockedBy, nil
}

// GetTasksByTags 按标签筛选任务.
func (m *TaskManager) GetTasksByTags(projectID string, tags []string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Task
	for _, task := range m.tasks {
		if task.ProjectID != projectID {
			continue
		}
		if hasAnyTag(task.Tags, tags) {
			result = append(result, task)
		}
	}
	return result
}

// BatchUpdateStatus 批量更新任务状态.
func (m *TaskManager) BatchUpdateStatus(taskIDs []string, status TaskStatus) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	updated := 0
	now := time.Now()
	for _, id := range taskIDs {
		if task, exists := m.tasks[id]; exists {
			task.Status = status
			task.UpdatedAt = now
			if status == TaskStatusDone {
				task.CompletedAt = &now
			}
			updated++
		}
	}
	return updated, nil
}

// helper functions

func priorityOrder(p TaskPriority) int {
	switch p {
	case PriorityUrgent:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}

// extractMentions 从文本中提取 @mentions.
func extractMentions(content string) []string {
	var mentions []string
	// 分割并处理所有 @ 符号
	parts := strings.Split(content, "@")
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		// 提取用户名（直到遇到分隔符）
		name := ""
		for _, c := range part {
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
				name += string(c)
			} else {
				break
			}
		}
		if name != "" {
			mentions = append(mentions, name)
		}
	}
	return mentions
}

func hasAnyTag(taskTags, filterTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range taskTags {
		tagSet[t] = true
	}
	for _, ft := range filterTags {
		if tagSet[ft] {
			return true
		}
	}
	return false
}
