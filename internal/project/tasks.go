// Package project provides task tracking functionality
package project

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskTracker 任务追踪器
type TaskTracker struct {
	mu      sync.RWMutex
	manager *Manager
}

// NewTaskTracker 创建任务追踪器
func NewTaskTracker(mgr *Manager) *TaskTracker {
	return &TaskTracker{
		manager: mgr,
	}
}

// TaskTransitions 任务状态流转规则
var TaskTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusTodo: {
		TaskStatusInProgress,
		TaskStatusCancelled,
	},
	TaskStatusInProgress: {
		TaskStatusReview,
		TaskStatusTodo,
		TaskStatusCancelled,
	},
	TaskStatusReview: {
		TaskStatusDone,
		TaskStatusInProgress,
		TaskStatusCancelled,
	},
	TaskStatusDone: {
		TaskStatusInProgress, // 允许重新打开
	},
	TaskStatusCancelled: {
		TaskStatusTodo, // 允许重新开始
	},
}

// TaskTransitionError 任务状态流转错误
// TaskTransitionError 任务状态流转错误
type TaskTransitionError struct {
	From TaskStatus
	To   TaskStatus
}

func (e *TaskTransitionError) Error() string {
	return "invalid task transition from " + string(e.From) + " to " + string(e.To)
}

// TransitionTask 任务状态流转
func (t *TaskTracker) TransitionTask(taskID, userID string, newStatus TaskStatus) (*Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	// 检查状态流转是否合法
	if !isValidTransition(task.Status, newStatus) {
		return nil, &TaskTransitionError{
			From: task.Status,
			To:   newStatus,
		}
	}

	updates := map[string]interface{}{
		"status": newStatus,
	}

	// 自动设置进度
	if newStatus == TaskStatusDone {
		updates["progress"] = 100
	} else if newStatus == TaskStatusInProgress && task.Progress == 0 {
		updates["progress"] = 10
	}

	return t.manager.UpdateTask(taskID, userID, updates)
}

// isValidTransition 检查状态流转是否合法
func isValidTransition(from, to TaskStatus) bool {
	if from == to {
		return true
	}
	allowed, exists := TaskTransitions[from]
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

// AssignTask 分配任务
func (t *TaskTracker) AssignTask(taskID, assigneeID, assignerID string) (*Task, error) {
	updates := map[string]interface{}{
		"assignee_id": assigneeID,
	}
	return t.manager.UpdateTask(taskID, assignerID, updates)
}

// SetTaskMilestone 设置任务里程碑
func (t *TaskTracker) SetTaskMilestone(taskID, milestoneID, userID string) (*Task, error) {
	updates := map[string]interface{}{
		"milestone_id": milestoneID,
	}
	return t.manager.UpdateTask(taskID, userID, updates)
}

// SetTaskProgress 设置任务进度
func (t *TaskTracker) SetTaskProgress(taskID string, progress int, userID string) (*Task, error) {
	if progress < 0 || progress > 100 {
		return nil, errors.New("进度必须在 0-100 之间")
	}

	updates := map[string]interface{}{
		"progress": progress,
	}

	// 自动更新状态
	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	if progress == 100 && task.Status != TaskStatusDone {
		updates["status"] = TaskStatusDone
	} else if progress > 0 && progress < 100 && task.Status == TaskStatusTodo {
		updates["status"] = TaskStatusInProgress
	}

	return t.manager.UpdateTask(taskID, userID, updates)
}

// SetTaskDueDate 设置任务截止日期
func (t *TaskTracker) SetTaskDueDate(taskID string, dueDate *time.Time, userID string) (*Task, error) {
	updates := map[string]interface{}{
		"due_date": dueDate,
	}
	return t.manager.UpdateTask(taskID, userID, updates)
}

// AddTaskTags 添加任务标签
func (t *TaskTracker) AddTaskTags(taskID string, tags []string, userID string) (*Task, error) {
	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	existingTags := make(map[string]bool)
	for _, tag := range task.Tags {
		existingTags[tag] = true
	}

	for _, tag := range tags {
		if !existingTags[tag] {
			task.Tags = append(task.Tags, tag)
		}
	}

	updates := map[string]interface{}{
		"tags": task.Tags,
	}
	return t.manager.UpdateTask(taskID, userID, updates)
}

// RemoveTaskTag 移除任务标签
func (t *TaskTracker) RemoveTaskTag(taskID, tag string, userID string) (*Task, error) {
	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	newTags := make([]string, 0)
	for _, t := range task.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}

	updates := map[string]interface{}{
		"tags": newTags,
	}
	return t.manager.UpdateTask(taskID, userID, updates)
}

// CreateSubTask 创建子任务
func (t *TaskTracker) CreateSubTask(parentID, title, description, reporterID string, priority TaskPriority) (*Task, error) {
	parent, err := t.manager.GetTask(parentID)
	if err != nil {
		return nil, err
	}

	task, err := t.manager.CreateTask(title, description, parent.ProjectID, reporterID, priority)
	if err != nil {
		return nil, err
	}

	// 设置父任务关联
	updates := map[string]interface{}{
		"parent_id":    parentID,
		"milestone_id": parent.MilestoneID,
		"assignee_id":  parent.AssigneeID,
	}
	_, err = t.manager.UpdateTask(task.ID, reporterID, updates)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// GetSubTasks 获取子任务列表
func (t *TaskTracker) GetSubTasks(parentID string) ([]*Task, error) {
	filter := TaskFilter{
		ProjectID: "",
		Limit:     1000,
	}

	allTasks := t.manager.ListTasks(filter)
	subTasks := make([]*Task, 0)
	for _, task := range allTasks {
		if task.ParentID == parentID {
			subTasks = append(subTasks, task)
		}
	}
	return subTasks, nil
}

// TaskTimeEntry 工时记录
type TaskTimeEntry struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	UserID      string    `json:"user_id"`
	Description string    `json:"description,omitempty"`
	Hours       float64   `json:"hours"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

// TimeTracker 工时追踪器
type TimeTracker struct {
	mu      sync.RWMutex
	entries map[string][]*TaskTimeEntry // taskID -> entries
}

// NewTimeTracker 创建工时追踪器
func NewTimeTracker() *TimeTracker {
	return &TimeTracker{
		entries: make(map[string][]*TaskTimeEntry),
	}
}

// AddTimeEntry 添加工时记录
func (tt *TimeTracker) AddTimeEntry(taskID, userID, description string, hours float64, date time.Time) (*TaskTimeEntry, error) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	entry := &TaskTimeEntry{
		ID:          uuid.New().String(),
		TaskID:      taskID,
		UserID:      userID,
		Description: description,
		Hours:       hours,
		Date:        date,
		CreatedAt:   time.Now(),
	}

	tt.entries[taskID] = append(tt.entries[taskID], entry)
	return entry, nil
}

// GetTimeEntries 获取任务工时记录
func (tt *TimeTracker) GetTimeEntries(taskID string) []*TaskTimeEntry {
	tt.mu.RLock()
	defer tt.mu.RUnlock()
	return append([]*TaskTimeEntry{}, tt.entries[taskID]...)
}

// GetTotalHours 获取任务总工时
func (tt *TimeTracker) GetTotalHours(taskID string) float64 {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	var total float64
	for _, entry := range tt.entries[taskID] {
		total += entry.Hours
	}
	return total
}

// TaskWatcher 任务观察者
type TaskWatcher struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// WatchManager 关注管理器
type WatchManager struct {
	mu       sync.RWMutex
	watchers map[string][]*TaskWatcher // taskID -> watchers
}

// NewWatchManager 创建关注管理器
func NewWatchManager() *WatchManager {
	return &WatchManager{
		watchers: make(map[string][]*TaskWatcher),
	}
}

// AddWatcher 添加关注者
func (wm *WatchManager) AddWatcher(taskID, userID string) (*TaskWatcher, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// 检查是否已关注
	for _, w := range wm.watchers[taskID] {
		if w.UserID == userID {
			return w, nil
		}
	}

	watcher := &TaskWatcher{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}

	wm.watchers[taskID] = append(wm.watchers[taskID], watcher)
	return watcher, nil
}

// RemoveWatcher 移除关注者
func (wm *WatchManager) RemoveWatcher(taskID, userID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	watchers := wm.watchers[taskID]
	newWatchers := make([]*TaskWatcher, 0)
	for _, w := range watchers {
		if w.UserID != userID {
			newWatchers = append(newWatchers, w)
		}
	}
	wm.watchers[taskID] = newWatchers
}

// GetWatchers 获取任务关注者
func (wm *WatchManager) GetWatchers(taskID string) []*TaskWatcher {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return append([]*TaskWatcher{}, wm.watchers[taskID]...)
}

// IsWatcher 检查是否关注
func (wm *WatchManager) IsWatcher(taskID, userID string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	for _, w := range wm.watchers[taskID] {
		if w.UserID == userID {
			return true
		}
	}
	return false
}
