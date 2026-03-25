package project

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 错误定义.
var (
	// ErrTaskNotFound 任务不存在错误.
	ErrTaskNotFound = errors.New("任务不存在")
	// ErrMilestoneNotFound 里程碑不存在错误.
	ErrMilestoneNotFound = errors.New("里程碑不存在")
	// ErrProjectNotFound 项目不存在错误.
	ErrProjectNotFound = errors.New("项目不存在")
	// ErrInvalidStatus 无效状态错误.
	ErrInvalidStatus = errors.New("无效的状态")
	// ErrPermissionDenied 权限不足错误.
	ErrPermissionDenied = errors.New("权限不足")
)

// Manager 项目管理器.
type Manager struct {
	mu         sync.RWMutex
	tasks      map[string]*Task
	milestones map[string]*Milestone
	projects   map[string]*Project
	comments   map[string][]*TaskComment
	history    map[string][]*TaskHistory
}

// NewManager 创建项目管理器.
func NewManager() *Manager {
	return &Manager{
		tasks:      make(map[string]*Task),
		milestones: make(map[string]*Milestone),
		projects:   make(map[string]*Project),
		comments:   make(map[string][]*TaskComment),
		history:    make(map[string][]*TaskHistory),
	}
}

// ========== 项目管理 ==========

// CreateProject 创建项目.
func (m *Manager) CreateProject(name, key, description, ownerID, createdBy string) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	project := &Project{
		ID:          uuid.New().String(),
		Name:        name,
		Key:         key,
		Description: description,
		Status:      "active",
		OwnerID:     ownerID,
		MemberIDs:   []string{ownerID},
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   createdBy,
	}

	m.projects[project.ID] = project
	return project, nil
}

// GetProject 获取项目.
func (m *Manager) GetProject(id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}
	return project, nil
}

// UpdateProject 更新项目.
func (m *Manager) UpdateProject(id string, updates map[string]interface{}) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	project.UpdatedAt = now

	if name, ok := updates["name"].(string); ok {
		project.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		project.Description = desc
	}
	if status, ok := updates["status"].(string); ok {
		project.Status = status
	}

	return project, nil
}

// DeleteProject 删除项目.
func (m *Manager) DeleteProject(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[id]; !exists {
		return ErrProjectNotFound
	}

	delete(m.projects, id)

	// 删除关联的里程碑和任务
	for _, task := range m.tasks {
		if task.ProjectID == id {
			delete(m.tasks, task.ID)
		}
	}
	for _, milestone := range m.milestones {
		if milestone.ProjectID == id {
			delete(m.milestones, milestone.ID)
		}
	}

	return nil
}

// ListProjects 列出项目.
func (m *Manager) ListProjects(userID string, limit, offset int) []*Project {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Project, 0)
	for _, project := range m.projects {
		if userID == "" || project.OwnerID == userID || contains(project.MemberIDs, userID) {
			result = append(result, project)
		}
	}

	// 按创建时间倒序
	sortByTime(result, false)

	if offset > len(result) {
		offset = len(result)
	}
	end := offset + limit
	if limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end]
}

// ========== 里程碑管理 ==========

// CreateMilestone 创建里程碑.
func (m *Manager) CreateMilestone(name, description, projectID, createdBy string, dueDate *time.Time) (*Milestone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	milestone := &Milestone{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		ProjectID:   projectID,
		Status:      "planned",
		DueDate:     dueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   createdBy,
	}

	m.milestones[milestone.ID] = milestone
	return milestone, nil
}

// GetMilestone 获取里程碑.
func (m *Manager) GetMilestone(id string) (*Milestone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	milestone, exists := m.milestones[id]
	if !exists {
		return nil, ErrMilestoneNotFound
	}
	return milestone, nil
}

// UpdateMilestone 更新里程碑.
func (m *Manager) UpdateMilestone(id string, updates map[string]interface{}) (*Milestone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	milestone, exists := m.milestones[id]
	if !exists {
		return nil, ErrMilestoneNotFound
	}

	now := time.Now()
	milestone.UpdatedAt = now

	if name, ok := updates["name"].(string); ok {
		milestone.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		milestone.Description = desc
	}
	if status, ok := updates["status"].(string); ok {
		milestone.Status = status
		if status == "completed" {
			nowCopy := now
			milestone.CompletedAt = &nowCopy
		}
	}
	if dueDate, ok := updates["due_date"].(*time.Time); ok {
		milestone.DueDate = dueDate
	}

	return milestone, nil
}

// DeleteMilestone 删除里程碑.
func (m *Manager) DeleteMilestone(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.milestones[id]; !exists {
		return ErrMilestoneNotFound
	}

	delete(m.milestones, id)

	// 清除关联任务的里程碑引用
	for _, task := range m.tasks {
		if task.MilestoneID == id {
			task.MilestoneID = ""
		}
	}

	return nil
}

// ListMilestones 列出里程碑.
func (m *Manager) ListMilestones(projectID string) []*Milestone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Milestone, 0)
	for _, milestone := range m.milestones {
		if projectID == "" || milestone.ProjectID == projectID {
			result = append(result, milestone)
		}
	}

	sortMilestonesByTime(result, false)
	return result
}

// ========== 任务管理 ==========

// CreateTask 创建任务.
func (m *Manager) CreateTask(title, description, projectID, reporterID string, priority TaskPriority) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	task := &Task{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Status:      TaskStatusTodo,
		Priority:    priority,
		ReporterID:  reporterID,
		ProjectID:   projectID,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   reporterID,
	}

	m.tasks[task.ID] = task
	m.recordHistory(task.ID, "created", "", "task", reporterID)

	// 更新项目任务计数
	if project, exists := m.projects[projectID]; exists {
		project.TaskCount++
	}

	return task, nil
}

// GetTask 获取任务.
func (m *Manager) GetTask(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// UpdateTask 更新任务.
func (m *Manager) UpdateTask(id, userID string, updates map[string]interface{}) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	now := time.Now()
	task.UpdatedAt = now

	// 记录变更历史
	if status, ok := updates["status"].(TaskStatus); ok && task.Status != status {
		m.recordHistory(id, "status", string(task.Status), string(status), userID)
		task.Status = status
		if status == TaskStatusDone {
			task.CompletedAt = &now
			task.Progress = 100
		}
	}
	if priority, ok := updates["priority"].(TaskPriority); ok && task.Priority != priority {
		m.recordHistory(id, "priority", string(task.Priority), string(priority), userID)
		task.Priority = priority
	}
	if title, ok := updates["title"].(string); ok {
		m.recordHistory(id, "title", task.Title, title, userID)
		task.Title = title
	}
	if desc, ok := updates["description"].(string); ok {
		task.Description = desc
	}
	if assigneeID, ok := updates["assignee_id"].(string); ok && task.AssigneeID != assigneeID {
		m.recordHistory(id, "assignee", task.AssigneeID, assigneeID, userID)
		task.AssigneeID = assigneeID
	}
	if milestoneID, ok := updates["milestone_id"].(string); ok && task.MilestoneID != milestoneID {
		m.recordHistory(id, "milestone", task.MilestoneID, milestoneID, userID)
		task.MilestoneID = milestoneID
	}
	if dueDate, ok := updates["due_date"].(*time.Time); ok {
		task.DueDate = dueDate
	}
	if progress, ok := updates["progress"].(int); ok && progress >= 0 && progress <= 100 {
		task.Progress = progress
	}
	if estimatedHours, ok := updates["estimated_hours"].(float64); ok {
		task.EstimatedHours = estimatedHours
	}
	if actualHours, ok := updates["actual_hours"].(float64); ok {
		task.ActualHours = actualHours
	}
	if tags, ok := updates["tags"].([]string); ok {
		task.Tags = tags
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

	delete(m.tasks, id)
	delete(m.comments, id)
	delete(m.history, id)

	// 更新项目任务计数
	if project, exists := m.projects[task.ProjectID]; exists {
		project.TaskCount--
		if task.Status == TaskStatusDone {
			project.DoneCount--
		}
	}

	return nil
}

// ListTasks 列出任务.
func (m *Manager) ListTasks(filter TaskFilter) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range m.tasks {
		if !m.matchesFilter(task, filter) {
			continue
		}
		result = append(result, task)
	}

	// 排序
	sortTasks(result, filter.OrderBy, filter.OrderDesc)

	// 分页
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

// GetTaskStats 获取任务统计.
func (m *Manager) GetTaskStats(projectID string) TaskStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := TaskStats{
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
		ByAssignee: make(map[string]int),
	}

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	for _, task := range m.tasks {
		if projectID != "" && task.ProjectID != projectID {
			continue
		}

		stats.Total++
		stats.ByStatus[string(task.Status)]++
		stats.ByPriority[string(task.Priority)]++

		if task.AssigneeID != "" {
			stats.ByAssignee[task.AssigneeID]++
		}

		// 过期任务
		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			stats.Overdue++
		}

		// 本周创建
		if task.CreatedAt.After(weekAgo) {
			stats.CreatedThisWeek++
		}

		// 本周完成
		if task.CompletedAt != nil && task.CompletedAt.After(weekAgo) {
			stats.CompletedThisWeek++
		}
	}

	return stats
}

// ========== 评论管理 ==========

// AddComment 添加评论.
func (m *Manager) AddComment(taskID, userID, content string) (*TaskComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return nil, ErrTaskNotFound
	}

	now := time.Now()
	comment := &TaskComment{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		UserID:    userID,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.comments[taskID] = append(m.comments[taskID], comment)
	return comment, nil
}

// GetComments 获取任务评论.
func (m *Manager) GetComments(taskID string) []*TaskComment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*TaskComment{}, m.comments[taskID]...)
}

// ========== 历史记录 ==========

// recordHistory 记录历史.
func (m *Manager) recordHistory(taskID, field, oldValue, newValue, userID string) {
	entry := &TaskHistory{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Field:     field,
		OldValue:  oldValue,
		NewValue:  newValue,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	m.history[taskID] = append(m.history[taskID], entry)
}

// GetHistory 获取任务历史.
func (m *Manager) GetHistory(taskID string) []*TaskHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*TaskHistory{}, m.history[taskID]...)
}

// ========== 辅助函数 ==========

func (m *Manager) matchesFilter(task *Task, filter TaskFilter) bool {
	if len(filter.Status) > 0 && !containsStatus(filter.Status, task.Status) {
		return false
	}
	if len(filter.Priority) > 0 && !containsPriority(filter.Priority, task.Priority) {
		return false
	}
	if filter.AssigneeID != "" && task.AssigneeID != filter.AssigneeID {
		return false
	}
	if filter.ReporterID != "" && task.ReporterID != filter.ReporterID {
		return false
	}
	if filter.ProjectID != "" && task.ProjectID != filter.ProjectID {
		return false
	}
	if filter.MilestoneID != "" && task.MilestoneID != filter.MilestoneID {
		return false
	}
	if filter.DueBefore != nil && task.DueDate != nil && !task.DueDate.Before(*filter.DueBefore) {
		return false
	}
	if filter.DueAfter != nil && task.DueDate != nil && !task.DueDate.After(*filter.DueAfter) {
		return false
	}
	if filter.Search != "" && !matchSearch(task, filter.Search) {
		return false
	}
	return true
}

func containsStatus(statuses []TaskStatus, status TaskStatus) bool {
	for _, s := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

func containsPriority(priorities []TaskPriority, priority TaskPriority) bool {
	for _, p := range priorities {
		if p == priority {
			return true
		}
	}
	return false
}

func matchSearch(task *Task, search string) bool {
	// 简单的标题/描述搜索
	return containsSubstring(task.Title, search) ||
		containsSubstring(task.Description, search)
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func sortByTime(projects []*Project, asc bool) {
	// 简单冒泡排序
	n := len(projects)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			cond := projects[j].CreatedAt.Before(projects[j+1].CreatedAt)
			if asc {
				cond = projects[j].CreatedAt.After(projects[j+1].CreatedAt)
			}
			if cond {
				projects[j], projects[j+1] = projects[j+1], projects[j]
			}
		}
	}
}

func sortMilestonesByTime(milestones []*Milestone, asc bool) {
	n := len(milestones)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			cond := milestones[j].CreatedAt.Before(milestones[j+1].CreatedAt)
			if asc {
				cond = milestones[j].CreatedAt.After(milestones[j+1].CreatedAt)
			}
			if cond {
				milestones[j], milestones[j+1] = milestones[j+1], milestones[j]
			}
		}
	}
}

func sortTasks(tasks []*Task, orderBy string, desc bool) {
	n := len(tasks)
	if n <= 1 {
		return
	}

	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			var cond bool
			switch orderBy {
			case "priority":
				cond = comparePriority(tasks[j].Priority, tasks[j+1].Priority) < 0
			case "due_date":
				if tasks[j].DueDate == nil && tasks[j+1].DueDate == nil {
					continue
				}
				if tasks[j].DueDate == nil {
					cond = true
				} else if tasks[j+1].DueDate == nil {
					cond = false
				} else {
					cond = tasks[j].DueDate.After(*tasks[j+1].DueDate)
				}
			case "status":
				cond = compareStatus(tasks[j].Status, tasks[j+1].Status) < 0
			default:
				cond = tasks[j].CreatedAt.Before(tasks[j+1].CreatedAt)
			}
			if desc {
				cond = !cond
			}
			if cond {
				tasks[j], tasks[j+1] = tasks[j+1], tasks[j]
			}
		}
	}
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
		TaskStatusTodo:       1,
		TaskStatusInProgress: 2,
		TaskStatusReview:     3,
		TaskStatusDone:       4,
		TaskStatusCancelled:  5,
	}
	return statusOrder[a] - statusOrder[b]
}
