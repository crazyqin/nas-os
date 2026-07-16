// Package cms 提供任务调度器
package cms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TaskDispatcher 任务调度器.
type TaskDispatcher struct {
	config    FleetConfig
	tasks     map[string]*TaskInfo
	nodeTasks map[string][]string // nodeID -> taskIDs
	logger    *zap.Logger
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	dataFile  string
}

// TaskInfo 任务信息.
type TaskInfo struct {
	TaskID      string                 `json:"taskId"`
	DeviceID    string                 `json:"deviceId"`
	TaskType    string                 `json:"taskType"`
	Priority    int                    `json:"priority"`
	Status      string                 `json:"status"` // pending, running, completed, failed, cancelled
	Params      map[string]interface{} `json:"params"`
	Progress    float64                `json:"progress"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	StartedAt   time.Time              `json:"startedAt,omitempty"`
	CompletedAt time.Time              `json:"completedAt,omitempty"`
	ExpiresAt   time.Time              `json:"expiresAt"`
	MaxRetries  int                    `json:"maxRetries"`
	RetryCount  int                    `json:"retryCount"`
}

// TaskDispatchRequest 任务分发请求.
type TaskDispatchRequest struct {
	TaskType   string                 `json:"taskType"`
	DeviceID   string                 `json:"deviceId"` // 可选，空则自动选择
	Priority   int                    `json:"priority"`
	Params     map[string]interface{} `json:"params"`
	ExpiresIn  time.Duration          `json:"expiresIn"`
	MaxRetries int                    `json:"maxRetries"`
}

// TaskDispatchResult 任务分发结果.
type TaskDispatchResult struct {
	TaskID  string `json:"taskId"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// NodeTask 节点任务.
type NodeTask struct {
	TaskID   string                 `json:"taskId"`
	TaskType string                 `json:"taskType"`
	Status   string                 `json:"status"`
	Progress float64                `json:"progress"`
	Params   map[string]interface{} `json:"params"`
	Error    string                 `json:"error,omitempty"`
}

// NewTaskDispatcher 创建任务调度器.
func NewTaskDispatcher(config FleetConfig, logger *zap.Logger) (*TaskDispatcher, error) {
	ctx, cancel := context.WithCancel(context.Background())

	td := &TaskDispatcher{
		config:    config,
		tasks:     make(map[string]*TaskInfo),
		nodeTasks: make(map[string][]string),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		dataFile:  filepath.Join(config.DataDir, "tasks.json"),
	}

	// 加载持久化数据
	if err := td.loadState(); err != nil {
		logger.Warn("加载任务状态失败", zap.Error(err))
	}

	return td, nil
}

// Start 启动任务调度器.
func (td *TaskDispatcher) Start() {
	td.logger.Info("任务调度器启动")
}

// Stop 停止任务调度器.
func (td *TaskDispatcher) Stop() {
	td.cancel()
	td.saveState()
	td.logger.Info("任务调度器停止")
}

// Dispatch 分发任务.
func (td *TaskDispatcher) Dispatch(req TaskDispatchRequest) (*TaskDispatchResult, error) {
	td.mu.Lock()
	defer td.mu.Unlock()

	taskID := generateTaskID()

	task := &TaskInfo{
		TaskID:     taskID,
		DeviceID:   req.DeviceID,
		TaskType:   req.TaskType,
		Priority:   req.Priority,
		Status:     "pending",
		Params:     req.Params,
		Progress:   0,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(req.ExpiresIn),
		MaxRetries: req.MaxRetries,
		RetryCount: 0,
	}

	td.tasks[taskID] = task
	if req.DeviceID != "" {
		td.nodeTasks[req.DeviceID] = append(td.nodeTasks[req.DeviceID], taskID)
	}

	td.logger.Info("任务已创建",
		zap.String("task_id", taskID),
		zap.String("task_type", req.TaskType),
		zap.String("device_id", req.DeviceID))

	return &TaskDispatchResult{
		TaskID:  taskID,
		Status:  "pending",
		Message: "任务已创建，等待执行",
	}, nil
}

// GetNodeTasks 获取节点任务列表.
func (td *TaskDispatcher) GetNodeTasks(deviceID string) ([]NodeTask, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()

	taskIDs := td.nodeTasks[deviceID]
	tasks := make([]NodeTask, 0, len(taskIDs))

	for _, taskID := range taskIDs {
		if task, ok := td.tasks[taskID]; ok {
			tasks = append(tasks, NodeTask{
				TaskID:   task.TaskID,
				TaskType: task.TaskType,
				Status:   task.Status,
				Progress: task.Progress,
				Params:   task.Params,
				Error:    task.Error,
			})
		}
	}

	return tasks, nil
}

// CancelTask 取消任务.
func (td *TaskDispatcher) CancelTask(taskID string) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	task, ok := td.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	task.Status = "cancelled"
	task.CompletedAt = time.Now()

	td.logger.Info("任务已取消", zap.String("task_id", taskID))

	return nil
}

// UpdateTaskProgress 更新任务进度.
func (td *TaskDispatcher) UpdateTaskProgress(deviceID string, progress TaskProgress) {
	td.mu.Lock()
	defer td.mu.Unlock()

	task, ok := td.tasks[progress.TaskID]
	if !ok {
		td.logger.Warn("任务不存在", zap.String("task_id", progress.TaskID))
		return
	}

	task.Status = progress.Status
	task.Progress = progress.Progress
	task.Error = progress.Error

	if progress.Status == "completed" {
		task.CompletedAt = time.Now()
	} else if progress.Status == "running" && task.StartedAt.IsZero() {
		task.StartedAt = time.Now()
	}

	td.logger.Debug("任务进度更新",
		zap.String("task_id", progress.TaskID),
		zap.Float64("progress", progress.Progress))
}

// GetTask 获取任务详情.
func (td *TaskDispatcher) GetTask(taskID string) (*TaskInfo, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()

	task, ok := td.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	return task, nil
}

// ListTasks 列出所有任务.
func (td *TaskDispatcher) ListTasks(filter TaskFilter) []*TaskInfo {
	td.mu.RLock()
	defer td.mu.RUnlock()

	result := make([]*TaskInfo, 0)
	for _, task := range td.tasks {
		if filter.Match(task) {
			result = append(result, task)
		}
	}
	return result
}

// TaskFilter 任务过滤器.
type TaskFilter struct {
	TaskType string
	Status   string
	DeviceID string
}

// Match 检查是否匹配.
func (f TaskFilter) Match(task *TaskInfo) bool {
	if f.TaskType != "" && task.TaskType != f.TaskType {
		return false
	}
	if f.Status != "" && task.Status != f.Status {
		return false
	}
	if f.DeviceID != "" && task.DeviceID != f.DeviceID {
		return false
	}
	return true
}

// loadState 加载持久化状态.
func (td *TaskDispatcher) loadState() error {
	data, err := os.ReadFile(td.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Tasks     map[string]*TaskInfo
		NodeTasks map[string][]string
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	td.tasks = state.Tasks
	td.nodeTasks = state.NodeTasks

	return nil
}

// saveState 保存持久化状态.
func (td *TaskDispatcher) saveState() error {
	td.mu.RLock()
	defer td.mu.RUnlock()

	state := struct {
		Tasks     map[string]*TaskInfo
		NodeTasks map[string][]string
	}{
		Tasks:     td.tasks,
		NodeTasks: td.nodeTasks,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(td.dataFile, data, 0640)
}

// generateTaskID 生成任务ID.
func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
