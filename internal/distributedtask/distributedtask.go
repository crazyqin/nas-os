// Package distributedtask 实现分布式任务调度系统。
// 支持跨节点任务分发、负载均衡、任务依赖、失败重试、任务优先级。
package distributedtask

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
	TaskRetrying  TaskStatus = "retrying"
	TaskTimeout   TaskStatus = "timeout"
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow    TaskPriority = 1
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 8
	PriorityUrgent TaskPriority = 10
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeBackup      TaskType = "backup"
	TaskTypeSync        TaskType = "sync"
	TaskTypeConvert     TaskType = "convert"
	TaskTypeCompress    TaskType = "compress"
	TaskTypeAnalyze     TaskType = "analyze"
	TaskTypeCleanup     TaskType = "cleanup"
	TaskTypeReplicate   TaskType = "replicate"
	TaskTypeMigrate     TaskType = "migrate"
	TaskTypeIndex       TaskType = "index"
	TaskTypeScan        TaskType = "scan"
	TaskTypeCustom      TaskType = "custom"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeOnline    NodeStatus = "online"
	NodeOffline   NodeStatus = "offline"
	NodeBusy      NodeStatus = "busy"
	NodeDraining  NodeStatus = "draining"
)

// WorkerNode 工作节点
type WorkerNode struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Address      string      `json:"address"`
	Status       NodeStatus  `json:"status"`
	MaxConcurrent int        `json:"maxConcurrent"`
	RunningTasks  int        `json:"runningTasks"`
	CPUCores     int         `json:"cpuCores"`
	MemoryMB     int         `json:"memoryMB"`
	DiskGB       int         `json:"diskGB"`
	Labels       map[string]string `json:"labels"`
	LastHeartbeat time.Time  `json:"lastHeartbeat"`
	RegisteredAt  time.Time  `json:"registeredAt"`
	CompletedTasks int64     `json:"completedTasks"`
	FailedTasks    int64     `json:"failedTasks"`
}

// Task 任务定义
type Task struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         TaskType          `json:"type"`
	Priority     TaskPriority      `json:"priority"`
	Status       TaskStatus        `json:"status"`
	Params       map[string]interface{} `json:"params"`
	Dependencies []string          `json:"dependencies,omitempty"` // 依赖的任务ID
	AssignedNode string            `json:"assignedNode,omitempty"`
	RetryCount   int               `json:"retryCount"`
	MaxRetries   int               `json:"maxRetries"`
	TimeoutSec   int               `json:"timeoutSec"`
	Progress     float64           `json:"progress"` // 0-100
	Result       interface{}       `json:"result,omitempty"`
	Error        string            `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	StartedAt    time.Time         `json:"startedAt,omitempty"`
	CompletedAt  time.Time         `json:"completedAt,omitempty"`
	CreatedBy    string            `json:"createdBy"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID    string      `json:"taskId"`
	Status    TaskStatus  `json:"status"`
	Output    interface{} `json:"output,omitempty"`
	Error     string      `json:"error,omitempty"`
	Duration  int64       `json:"durationMs"`
	NodeID    string      `json:"nodeId"`
	Timestamp time.Time   `json:"timestamp"`
}

// Scheduler 调度器
type Scheduler struct {
	mu          sync.RWMutex
	tasks       map[string]*Task
	nodes       map[string]*WorkerNode
	taskQueue   chan string // task ID queue
	results     []TaskResult
	strategy    ScheduleStrategy
	running     bool
	quit        chan struct{}
}

// ScheduleStrategy 调度策略
type ScheduleStrategy string

const (
	StrategyRoundRobin  ScheduleStrategy = "round-robin"
	StrategyLeastLoad   ScheduleStrategy = "least-load"
	StrategyPriority    ScheduleStrategy = "priority"
	StrategyAffinity    ScheduleStrategy = "affinity" // 基于标签亲和
)

// NewScheduler 创建调度器
func NewScheduler(strategy ScheduleStrategy) *Scheduler {
	if strategy == "" {
		strategy = StrategyLeastLoad
	}
	return &Scheduler{
		tasks:     make(map[string]*Task),
		nodes:     make(map[string]*WorkerNode),
		taskQueue: make(chan string, 10000),
		results:   make([]TaskResult, 0),
		strategy:  strategy,
		quit:      make(chan struct{}),
	}
}

// RegisterNode 注册工作节点
func (s *Scheduler) RegisterNode(node WorkerNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("节点ID不能为空")
	}
	node.RegisteredAt = time.Now()
	node.LastHeartbeat = time.Now()
	node.Status = NodeOnline
	s.nodes[node.ID] = &node
	return nil
}

// RemoveNode 移除工作节点
func (s *Scheduler) RemoveNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nodes[nodeID]; !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	delete(s.nodes, nodeID)
	return nil
}

// Heartbeat 节点心跳
func (s *Scheduler) Heartbeat(nodeID string, runningTasks int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	node.LastHeartbeat = time.Now()
	node.RunningTasks = runningTasks
	if runningTasks >= node.MaxConcurrent {
		node.Status = NodeBusy
	} else {
		node.Status = NodeOnline
	}
	return nil
}

// SubmitTask 提交任务
func (s *Scheduler) SubmitTask(task Task) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}
	if task.TimeoutSec == 0 {
		task.TimeoutSec = 300 // 5分钟默认超时
	}
	task.Status = TaskQueued
	task.CreatedAt = time.Now()

	s.tasks[task.ID] = &task

	// 放入队列
	select {
	case s.taskQueue <- task.ID:
	default:
		task.Status = TaskFailed
		task.Error = "任务队列已满"
		return &task, fmt.Errorf("任务队列已满")
	}

	return &task, nil
}

// CancelTask 取消任务
func (s *Scheduler) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}
	if task.Status == TaskCompleted {
		return fmt.Errorf("任务 %s 已完成，无法取消", taskID)
	}
	task.Status = TaskCancelled
	return nil
}

// GetTask 获取任务
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListTasks 列出任务
func (s *Scheduler) ListTasks(status TaskStatus, limit int) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]Task, 0)
	for _, t := range s.tasks {
		if status == "" || t.Status == status {
			tasks = append(tasks, *t)
		}
	}
	if limit > 0 && limit < len(tasks) {
		tasks = tasks[:limit]
	}
	return tasks
}

// ListNodes 列出节点
func (s *Scheduler) ListNodes() []WorkerNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]WorkerNode, 0, len(s.nodes))
	for _, n := range s.nodes {
		nodes = append(nodes, *n)
	}
	return nodes
}

// selectNode 选择节点
func (s *Scheduler) selectNode(task *Task) *WorkerNode {
	switch s.strategy {
	case StrategyRoundRobin:
		return s.selectRoundRobin()
	case StrategyLeastLoad:
		return s.selectLeastLoad()
	case StrategyAffinity:
		return s.selectByAffinity(task)
	default:
		return s.selectLeastLoad()
	}
}

func (s *Scheduler) selectRoundRobin() *WorkerNode {
	var selected *WorkerNode
	for _, node := range s.nodes {
		if node.Status == NodeOnline && node.RunningTasks < node.MaxConcurrent {
			if selected == nil || node.CompletedTasks < selected.CompletedTasks {
				selected = node
			}
		}
	}
	return selected
}

func (s *Scheduler) selectLeastLoad() *WorkerNode {
	var selected *WorkerNode
	minLoad := 1.0
	for _, node := range s.nodes {
		if node.Status == NodeOnline && node.RunningTasks < node.MaxConcurrent {
			load := float64(node.RunningTasks) / float64(node.MaxConcurrent)
			if load < minLoad {
				minLoad = load
				selected = node
			}
		}
	}
	return selected
}

func (s *Scheduler) selectByAffinity(task *Task) *WorkerNode {
	// 基于标签匹配选择节点
	for _, node := range s.nodes {
		if node.Status != NodeOnline || node.RunningTasks >= node.MaxConcurrent {
			continue
		}
		matched := true
		for k, v := range task.Tags {
			if nodeLabel, ok := node.Labels[k]; !ok || nodeLabel != v {
				matched = false
				break
			}
		}
		if matched {
			return node
		}
	}
	// 亲和失败，回退到最少负载
	return s.selectLeastLoad()
}

// CompleteTask 完成任务
func (s *Scheduler) CompleteTask(taskID string, result interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	task.Status = TaskCompleted
	task.Result = result
	task.CompletedAt = time.Now()
	task.Progress = 100

	if task.AssignedNode != "" {
		if node, ok := s.nodes[task.AssignedNode]; ok {
			node.RunningTasks--
			node.CompletedTasks++
		}
	}

	s.results = append(s.results, TaskResult{
		TaskID:    taskID,
		Status:    TaskCompleted,
		Output:    result,
		Duration:  task.CompletedAt.Sub(task.StartedAt).Milliseconds(),
		NodeID:    task.AssignedNode,
		Timestamp: time.Now(),
	})

	return nil
}

// FailTask 标记任务失败
func (s *Scheduler) FailTask(taskID string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.RetryCount < task.MaxRetries {
		task.RetryCount++
		task.Status = TaskRetrying
		task.Error = errMsg
		// 重新入队
		go func() {
			s.taskQueue <- taskID
		}()
		return nil
	}

	task.Status = TaskFailed
	task.Error = errMsg
	task.CompletedAt = time.Now()

	if task.AssignedNode != "" {
		if node, ok := s.nodes[task.AssignedNode]; ok {
			node.RunningTasks--
			node.FailedTasks++
		}
	}

	return nil
}

// GetStats 获取调度器统计
type SchedulerStats struct {
	TotalTasks    int            `json:"totalTasks"`
	QueuedTasks   int            `json:"queuedTasks"`
	RunningTasks  int            `json:"runningTasks"`
	CompletedTasks int           `json:"completedTasks"`
	FailedTasks   int            `json:"failedTasks"`
	TotalNodes    int            `json:"totalNodes"`
	OnlineNodes   int            `json:"onlineNodes"`
	Strategy      ScheduleStrategy `json:"strategy"`
	QueueLength   int            `json:"queueLength"`
}

func (s *Scheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SchedulerStats{
		TotalTasks: len(s.tasks),
		TotalNodes: len(s.nodes),
		Strategy:   s.strategy,
		QueueLength: len(s.taskQueue),
	}
	for _, t := range s.tasks {
		switch t.Status {
		case TaskQueued:
			stats.QueuedTasks++
		case TaskRunning:
			stats.RunningTasks++
		case TaskCompleted:
			stats.CompletedTasks++
		case TaskFailed:
			stats.FailedTasks++
		}
	}
	for _, n := range s.nodes {
		if n.Status == NodeOnline {
			stats.OnlineNodes++
		}
	}
	return stats
}

// RegisterRoutes 注册 HTTP 路由
func (s *Scheduler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/tasks", s.handleTasks)
	mux.HandleFunc("/api/v1/tasks/submit", s.handleSubmit)
	mux.HandleFunc("/api/v1/tasks/stats", s.handleStats)
	mux.HandleFunc("/api/v1/nodes", s.handleNodes)
	mux.HandleFunc("/api/v1/nodes/register", s.handleRegisterNode)
}

func (s *Scheduler) handleTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.ListTasks("", 100)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *Scheduler) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	result, err := s.SubmitTask(task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Scheduler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Scheduler) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.ListNodes()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (s *Scheduler) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var node WorkerNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := s.RegisterNode(node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}
