package netscan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Scanner 网络扫描管理器.
type Scanner struct {
	taskMgr *TaskManager
	mu      sync.RWMutex
}

// NewScanner 创建网络扫描管理器.
func NewScanner(maxConcurrent int) *Scanner {
	return &Scanner{
		taskMgr: NewTaskManager(maxConcurrent),
	}
}

// StartDiscovery 启动设备发现任务.
func (s *Scanner) StartDiscovery(ctx context.Context, config DiscoveryConfig) (*ScanTask, error) {
	task := s.createTask("discovery", config.Network, config)

	go s.runDiscoveryTask(ctx, task, config)

	return task, nil
}

// StartPortScan 启动端口扫描任务.
func (s *Scanner) StartPortScan(ctx context.Context, config PortScanConfig) (*ScanTask, error) {
	task := s.createTask("portscan", config.Target, config)

	go s.runPortScanTask(ctx, task, config)

	return task, nil
}

// StartServiceDetect 启动服务识别任务.
func (s *Scanner) StartServiceDetect(ctx context.Context, config ServiceDetectConfig) (*ScanTask, error) {
	task := s.createTask("servicedetect", config.Target, config)

	go s.runServiceDetectTask(ctx, task, config)

	return task, nil
}

// StartTopologyScan 启动拓扑扫描任务.
func (s *Scanner) StartTopologyScan(ctx context.Context, network string) (*ScanTask, error) {
	task := s.createTask("topology", network, network)

	go s.runTopologyTask(ctx, task, network)

	return task, nil
}

// GetTask 获取任务.
func (s *Scanner) GetTask(taskID string) (*ScanTask, bool) {
	return s.taskMgr.GetTask(taskID)
}

// ListTasks 列出所有任务.
func (s *Scanner) ListTasks() []*ScanTask {
	return s.taskMgr.ListTasks()
}

// CancelTask 取消任务.
func (s *Scanner) CancelTask(taskID string) error {
	return s.taskMgr.CancelTask(taskID)
}

// createTask 创建任务.
func (s *Scanner) createTask(taskType, target string, config interface{}) *ScanTask {
	task := &ScanTask{
		ID:        uuid.New().String(),
		Type:      taskType,
		Target:    target,
		Status:    "running",
		StartTime: time.Now(),
		Config:    config,
	}
	s.taskMgr.AddTask(task)
	return task
}

// runDiscoveryTask 运行设备发现任务.
func (s *Scanner) runDiscoveryTask(ctx context.Context, task *ScanTask, config DiscoveryConfig) {
	discoverer := NewDiscoverer(config)
	result, err := discoverer.Discover(ctx)

	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		return
	}

	task.Status = "completed"
	task.Progress = 100
	task.Result = result
}

// runPortScanTask 运行端口扫描任务.
func (s *Scanner) runPortScanTask(ctx context.Context, task *ScanTask, config PortScanConfig) {
	scanner := NewPortScanner(config)
	result, err := scanner.Scan(ctx)

	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		return
	}

	task.Status = "completed"
	task.Progress = 100
	task.Result = result
}

// runServiceDetectTask 运行服务识别任务.
func (s *Scanner) runServiceDetectTask(ctx context.Context, task *ScanTask, config ServiceDetectConfig) {
	detector := NewServiceDetector(config)
	result, err := detector.Detect(ctx)

	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		return
	}

	task.Status = "completed"
	task.Progress = 100
	task.Result = result
}

// runTopologyTask 运行拓扑扫描任务.
func (s *Scanner) runTopologyTask(ctx context.Context, task *ScanTask, network string) {
	result, err := DiscoverTopology(ctx, network)

	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		return
	}

	task.Status = "completed"
	task.Progress = 100
	task.Result = result
}

// AddTask 添加任务.
func (tm *TaskManager) AddTask(task *ScanTask) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tasks[task.ID] = task
}

// GetTask 获取任务.
func (tm *TaskManager) GetTask(taskID string) (*ScanTask, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	task, ok := tm.tasks[taskID]
	return task, ok
}

// ListTasks 列出所有任务.
func (tm *TaskManager) ListTasks() []*ScanTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*ScanTask, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CancelTask 取消任务.
func (tm *TaskManager) CancelTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在：%s", taskID)
	}

	if task.Status != "running" {
		return fmt.Errorf("任务不在运行状态")
	}

	task.Status = "cancelled"
	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)

	return nil
}

// CleanFinished 清理已完成的任务.
func (tm *TaskManager) CleanFinished() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	count := 0
	for id, task := range tm.tasks {
		if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
			delete(tm.tasks, id)
			count++
		}
	}
	return count
}
