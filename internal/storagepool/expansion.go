// Package storagepool - 存储池扩容服务
// 支持btrfs/ZFS存储池的在线扩容
package storagepool

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ExpansionMode 扩容模式
type ExpansionMode string

const (
	ExpansionModeSingle ExpansionMode = "single" // 单盘扩展（RAIDZ）
	ExpansionModeMulti  ExpansionMode = "multi"  // 多盘添加
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ExpansionOptions 扩容选项
type ExpansionOptions struct {
	Mode        ExpansionMode `json:"mode"`
	Background  bool          `json:"background"`
	AutoBalance bool          `json:"autoBalance"` // btrfs: 自动balance
	Priority    int           `json:"priority"`
}

// ExpansionTask 扩容任务
type ExpansionTask struct {
	ID           string      `json:"id"`
	PoolID       string      `json:"poolId"`
	Status       TaskStatus  `json:"status"`
	Progress     float64     `json:"progress"` // 0-100
	StartTime    time.Time   `json:"startTime"`
	EndTime      time.Time   `json:"endTime,omitempty"`
	ETA          time.Time   `json:"eta,omitempty"`
	BytesMoved   int64       `json:"bytesMoved"`
	BytesTotal   int64       `json:"bytesTotal"`
	AddedDevices []string    `json:"addedDevices"`
	CurrentPhase string      `json:"currentPhase"` // adding, balancing, verifying
	ErrorMsg     string      `json:"errorMsg,omitempty"`
}

// ExpansionService 扩容服务接口
type ExpansionService interface {
	// Expand 扩展存储池
	Expand(poolID string, devices []string, opts ExpansionOptions) (*ExpansionTask, error)

	// ConvertRAIDLevel 转换RAID级别
	ConvertRAIDLevel(poolID string, targetLevel RAIDLevel) (*ExpansionTask, error)

	// GetTaskProgress 查询任务进度
	GetTaskProgress(taskID string) (*ExpansionTask, error)

	// CancelTask 取消任务
	CancelTask(taskID string) error

	// RollbackExpansion 回滚扩展
	RollbackExpansion(taskID string) error

	// ListTasks 列出所有任务
	ListTasks(poolID string) []*ExpansionTask
}

// BtrfsExpansionService btrfs扩容实现
type BtrfsExpansionService struct {
	manager  *Manager
	tasks    map[string]*ExpansionTask
	taskMu   sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewBtrfsExpansionService 创建btrfs扩容服务
func NewBtrfsExpansionService(manager *Manager) *BtrfsExpansionService {
	ctx, cancel := context.WithCancel(context.Background())
	return &BtrfsExpansionService{
		manager: manager,
		tasks:   make(map[string]*ExpansionTask),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Expand 扩展btrfs存储池
func (s *BtrfsExpansionService) Expand(poolID string, devices []string, opts ExpansionOptions) (*ExpansionTask, error) {
	// 验证池存在
	pool, err := s.manager.GetPool(poolID)
	if err != nil {
		return nil, fmt.Errorf("存储池不存在: %w", err)
	}

	// 验证文件系统类型
	if pool.FileSystem != "btrfs" {
		return nil, fmt.Errorf("仅支持btrfs存储池扩容")
	}

	// 验证设备可用性
	for _, dev := range devices {
		// 使用Manager的私有方法验证（同包访问）
		if _, err := s.manager.getAvailableDevice(dev); err != nil {
			return nil, fmt.Errorf("设备不可用: %s", dev)
		}
	}

	// 创建任务
	taskID := generateTaskID()
	task := &ExpansionTask{
		ID:           taskID,
		PoolID:       poolID,
		Status:       TaskStatusPending,
		Progress:     0,
		StartTime:    time.Now(),
		AddedDevices: devices,
		CurrentPhase: "adding",
	}

	s.taskMu.Lock()
	s.tasks[taskID] = task
	s.taskMu.Unlock()

	// 执行扩容
	go s.executeExpansion(task, pool, devices, opts)

	return task, nil
}

// executeExpansion 执行扩容流程
func (s *BtrfsExpansionService) executeExpansion(task *ExpansionTask, pool *Pool, devices []string, opts ExpansionOptions) {
	s.updateTaskStatus(task.ID, TaskStatusRunning)

	// 阶段1: 添加设备
	task.CurrentPhase = "adding"
	s.updateTaskProgress(task.ID, 10)

	for _, dev := range devices {
		if err := s.addDeviceToBtrfs(pool.MountPoint, dev); err != nil {
			s.updateTaskError(task.ID, fmt.Sprintf("添加设备失败: %v", err))
			return
		}
	}

	s.updateTaskProgress(task.ID, 30)

	// 阶段2: 数据平衡（如果启用）
	if opts.AutoBalance {
		task.CurrentPhase = "balancing"
		s.updateTaskProgress(task.ID, 40)

		if err := s.startBalance(pool.MountPoint, task.ID); err != nil {
			s.updateTaskError(task.ID, fmt.Sprintf("平衡失败: %v", err))
			return
		}

		// 监控balance进度
		s.monitorBalanceProgress(task.ID, pool.MountPoint)
	}

	// 阶段3: 验证
	task.CurrentPhase = "verifying"
	s.updateTaskProgress(task.ID, 95)

	if err := s.verifyPool(pool.ID); err != nil {
		s.updateTaskError(task.ID, fmt.Sprintf("验证失败: %v", err))
		return
	}

	// 完成
	task.CurrentPhase = "completed"
	s.updateTaskStatus(task.ID, TaskStatusCompleted)
	s.updateTaskProgress(task.ID, 100)
	task.EndTime = time.Now()
}

// addDeviceToBtrfs 添加设备到btrfs池
func (s *BtrfsExpansionService) addDeviceToBtrfs(mountPoint, device string) error {
	// TODO: 实现实际命令执行
	// cmd := exec.Command("btrfs", "device", "add", device, mountPoint)
	// return cmd.Run()

	// 模拟实现
	return nil
}

// startBalance 启动数据平衡
func (s *BtrfsExpansionService) startBalance(mountPoint, taskID string) error {
	// TODO: 实现实际命令执行
	// cmd := exec.Command("btrfs", "balance", "start", mountPoint)
	// return cmd.Run()

	return nil
}

// monitorBalanceProgress 监控balance进度
func (s *BtrfsExpansionService) monitorBalanceProgress(taskID, mountPoint string) {
	// TODO: 实现进度监控
	// 定期执行: btrfs balance status <mountPoint>
	// 解析输出更新进度

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 模拟进度更新
			s.taskMu.Lock()
			task := s.tasks[taskID]
			if task == nil || task.Status != TaskStatusRunning {
				s.taskMu.Unlock()
				return
			}
			// 模拟进度递增
			if task.Progress < 90 {
				task.Progress += 5
			}
			s.taskMu.Unlock()
		}
	}
}

// verifyPool 验证存储池状态
func (s *BtrfsExpansionService) verifyPool(poolID string) error {
	// TODO: 实现验证逻辑
	return nil
}

// ConvertRAIDLevel 转换RAID级别
func (s *BtrfsExpansionService) ConvertRAIDLevel(poolID string, targetLevel RAIDLevel) (*ExpansionTask, error) {
	pool, err := s.manager.GetPool(poolID)
	if err != nil {
		return nil, err
	}

	if pool.FileSystem != "btrfs" {
		return nil, fmt.Errorf("仅支持btrfs RAID级别转换")
	}

	// 创建转换任务
	taskID := generateTaskID()
	task := &ExpansionTask{
		ID:        taskID,
		PoolID:    poolID,
		Status:    TaskStatusPending,
		StartTime: time.Now(),
	}

	s.taskMu.Lock()
	s.tasks[taskID] = task
	s.taskMu.Unlock()

	// 执行转换
	go s.executeRAIDConversion(task, pool, targetLevel)

	return task, nil
}

// executeRAIDConversion 执行RAID级别转换
func (s *BtrfsExpansionService) executeRAIDConversion(task *ExpansionTask, pool *Pool, targetLevel RAIDLevel) {
	s.updateTaskStatus(task.ID, TaskStatusRunning)
	task.CurrentPhase = "converting"

	// TODO: 实现 btrfs balance -mconvert=raidX -dconvert=raidX

	s.updateTaskProgress(task.ID, 100)
	s.updateTaskStatus(task.ID, TaskStatusCompleted)
	task.EndTime = time.Now()
}

// GetTaskProgress 查询任务进度
func (s *BtrfsExpansionService) GetTaskProgress(taskID string) (*ExpansionTask, error) {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// CancelTask 取消任务
func (s *BtrfsExpansionService) CancelTask(taskID string) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if task.Status != TaskStatusRunning {
		return fmt.Errorf("任务不在运行状态")
	}

	// TODO: 实现取消逻辑
	// btrfs: btrfs balance cancel <mountPoint>

	task.Status = TaskStatusCancelled
	task.EndTime = time.Now()
	return nil
}

// RollbackExpansion 回滚扩展
func (s *BtrfsExpansionService) RollbackExpansion(taskID string) error {
	s.taskMu.RLock()
	task, ok := s.tasks[taskID]
	s.taskMu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	// btrfs: btrfs device delete <device> <mountPoint>
	for _, dev := range task.AddedDevices {
		pool, _ := s.manager.GetPool(task.PoolID)
		if err := s.removeDeviceFromBtrfs(pool.MountPoint, dev); err != nil {
			return fmt.Errorf("回滚失败: %w", err)
		}
	}

	return nil
}

// removeDeviceFromBtrfs 从btrfs池移除设备
func (s *BtrfsExpansionService) removeDeviceFromBtrfs(mountPoint, device string) error {
	// TODO: 实现实际命令执行
	return nil
}

// ListTasks 列出所有任务
func (s *BtrfsExpansionService) ListTasks(poolID string) []*ExpansionTask {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	var result []*ExpansionTask
	for _, task := range s.tasks {
		if poolID == "" || task.PoolID == poolID {
			result = append(result, task)
		}
	}
	return result
}

// updateTaskStatus 更新任务状态
func (s *BtrfsExpansionService) updateTaskStatus(taskID string, status TaskStatus) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.Status = status
	}
}

// updateTaskProgress 更新任务进度
func (s *BtrfsExpansionService) updateTaskProgress(taskID string, progress float64) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.Progress = progress
	}
}

// updateTaskError 更新任务错误
func (s *BtrfsExpansionService) updateTaskError(taskID string, errMsg string) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.Status = TaskStatusFailed
		task.ErrorMsg = errMsg
		task.EndTime = time.Now()
	}
}

// Close 关闭服务
func (s *BtrfsExpansionService) Close() {
	s.cancel()
}

// 辅助函数
func generateTaskID() string {
	return fmt.Sprintf("expand_%d", time.Now().UnixNano())
}