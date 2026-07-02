// Package storage RAIDZ Expansion 核心业务服务
// 对标 TrueNAS 24.10 Electric Eel RAIDZ Expansion 特性
// 兵部 Round 142

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	zfs "nas-os/pkg/storage/zfs"
)

// ========== 核心错误定义 ==========

var (
	ErrExpansionNotReady    = fmt.Errorf("pool not ready for expansion")
	ErrDiskTooSmall         = fmt.Errorf("new disk too small")
	ErrResilverInProgress   = fmt.Errorf("resilver operation in progress")
	ErrScrubInProgress      = fmt.Errorf("scrub operation in progress")
	ErrPoolNotHealthy       = fmt.Errorf("pool health check failed")
	ErrExpansionAlreadyRuns = fmt.Errorf("expansion already running on this pool")
)

// ========== RAIDZ 扩展服务 ==========

// RAIDZExpansionService RAIDZ扩展核心服务
// 整合 ZFS 命令调用、进度监控和异步任务管理.
type RAIDZExpansionService struct {
	mu sync.RWMutex

	// ZFS 扩展管理器
	zfsManager *zfs.RAIDZExpansionManager

	// 当前活跃任务
	activeTasks map[string]*ExpansionTask

	// 任务历史
	taskHistory []*ExpansionTask

	// 配置路径
	configPath string

	// 进度更新回调
	progressCallbacks map[string]func(*ExpansionProgress)

	// 状态变更回调
	stateCallbacks []func(*ExpansionTask)
}

// ExpansionTask 扩展任务.
type ExpansionTask struct {
	ID             string            `json:"id"`
	PoolName       string            `json:"pool_name"`
	NewDisk        string            `json:"new_disk"`
	RAIDZLevel     string            `json:"raidz_level"`
	Status         ExpansionStatus   `json:"status"`
	Progress       float64           `json:"progress"`
	BytesProcessed uint64            `json:"bytes_processed"`
	TotalBytes     uint64            `json:"total_bytes"`
	SpeedMBps      float64           `json:"speed_mbps"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        *time.Time        `json:"end_time,omitempty"`
	ETA            time.Duration     `json:"eta"`
	Errors         []string          `json:"errors,omitempty"`
	Warnings       []string          `json:"warnings,omitempty"`
	CanPause       bool              `json:"can_pause"`
	CanCancel      bool              `json:"can_cancel"`
	CanResume      bool              `json:"can_resume"`
	PauseCount     int               `json:"pause_count"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	LastUpdate     time.Time         `json:"last_update"`
}

// ExpansionStatus 扩展任务状态.
type ExpansionStatus string

const (
	StatusIdle      ExpansionStatus = "idle"
	StatusPreparing ExpansionStatus = "preparing"
	StatusRunning   ExpansionStatus = "running"
	StatusPaused    ExpansionStatus = "paused"
	StatusCompleted ExpansionStatus = "completed"
	StatusFailed    ExpansionStatus = "failed"
	StatusCancelled ExpansionStatus = "cancelled"
)

// ExpansionProgress 进度详情.
type ExpansionProgress struct {
	TaskID         string        `json:"task_id"`
	Percentage     float64       `json:"percentage"`
	BytesProcessed uint64        `json:"bytes_processed"`
	BytesTotal     uint64        `json:"bytes_total"`
	SpeedMBps      float64       `json:"speed_mbps"`
	ETA            time.Duration `json:"eta"`
	Elapsed        time.Duration `json:"elapsed"`
	Phase          string        `json:"phase"`
	PhaseProgress  float64       `json:"phase_progress"`
	LastUpdate     time.Time     `json:"last_update"`
}

// ExpansionEligibilityResult 扩展资格检查结果.
type ExpansionEligibilityResult struct {
	PoolName         string           `json:"pool_name"`
	Eligible         bool             `json:"eligible"`
	RAIDZLevel       string           `json:"raidz_level"`
	CurrentWidth     int              `json:"current_width"`
	NewWidth         int              `json:"new_width"`
	CapacityGain     uint64           `json:"capacity_gain"`
	CurrentCapacity  uint64           `json:"current_capacity"`
	NewCapacity      uint64           `json:"new_capacity"`
	Warnings         []string         `json:"warnings"`
	PreChecks        []PreCheckResult `json:"pre_checks"`
	EstimatedTime    time.Duration    `json:"estimated_time"`
	DiskRequirements DiskRequirements `json:"disk_requirements"`
}

// PreCheckResult 预检查结果.
type PreCheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Required bool   `json:"required"`
}

// DiskRequirements 磁盘要求.
type DiskRequirements struct {
	MinSizeGB     int      `json:"min_size_gb"`
	RecommendedGB int      `json:"recommended_gb"`
	Interfaces    []string `json:"interfaces"`
	MustMatchSize bool     `json:"must_match_size"`
}

// NewRAIDZExpansionService 创建扩展服务.
func NewRAIDZExpansionService(configPath string) (*RAIDZExpansionService, error) {
	// 创建 ZFS 管理器
	zfsManager, err := zfs.NewRAIDZExpansionManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create ZFS manager: %w", err)
	}

	service := &RAIDZExpansionService{
		zfsManager:        zfsManager,
		activeTasks:       make(map[string]*ExpansionTask),
		taskHistory:       make([]*ExpansionTask, 0, 100),
		configPath:        configPath,
		progressCallbacks: make(map[string]func(*ExpansionProgress)),
		stateCallbacks:    make([]func(*ExpansionTask), 0),
	}

	// 加载历史记录
	if configPath != "" {
		_ = service.loadHistory()
	}

	return service, nil
}

// ========== 核心业务方法 ==========

// CheckExpansionEligibility 检查池是否满足扩展条件.
func (s *RAIDZExpansionService) CheckExpansionEligibility(ctx context.Context, poolName string) (*ExpansionEligibilityResult, error) {
	result := &ExpansionEligibilityResult{
		PoolName:  poolName,
		Eligible:  false,
		Warnings:  []string{},
		PreChecks: []PreCheckResult{},
	}

	// 检查 ZFS 可用性
	if !s.zfsManager.IsAvailable() {
		return nil, zfs.ErrZFSNotAvailable
	}

	// 检查系统是否支持 RAIDZ 扩展
	supported, reason := s.zfsManager.CheckExpansionSupport()
	if !supported {
		result.Warnings = append(result.Warnings, reason)
		result.PreChecks = append(result.PreChecks, PreCheckResult{
			Name:     "zfs_version",
			Passed:   false,
			Message:  reason,
			Required: true,
		})
		return result, nil
	}

	// 获取池扩展信息
	poolInfo, err := s.zfsManager.GetPoolExpansionInfo(ctx, poolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool info: %w", err)
	}

	// 检查池状态
	result.CurrentCapacity = poolInfo.TotalSize
	if poolInfo.PoolState != "ONLINE" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("池状态为 %s，需要 ONLINE", poolInfo.PoolState))
		result.PreChecks = append(result.PreChecks, PreCheckResult{
			Name:     "pool_health",
			Passed:   false,
			Message:  poolInfo.PoolState,
			Required: true,
		})
		return result, ErrPoolNotHealthy
	}

	result.PreChecks = append(result.PreChecks, PreCheckResult{
		Name:     "pool_health",
		Passed:   true,
		Message:  "ONLINE",
		Required: true,
	})

	// 检查是否有正在进行的 scrub/resilver
	if err := s.checkNoActiveScrub(ctx, poolName, result); err != nil {
		return result, err
	}

	// 检查 VDEV 类型
	if !poolInfo.CanExpand {
		result.Warnings = append(result.Warnings, poolInfo.Reason)
		result.PreChecks = append(result.PreChecks, PreCheckResult{
			Name:     "vdev_type",
			Passed:   false,
			Message:  poolInfo.Reason,
			Required: true,
		})
		return result, nil
	}

	// 解析 RAIDZ 信息
	for _, vdev := range poolInfo.Vdevs {
		if vdev.VdevType == "raidz1" || vdev.VdevType == "raidz2" || vdev.VdevType == "raidz3" {
			result.RAIDZLevel = vdev.VdevType
			result.CurrentWidth = vdev.Width
			result.NewWidth = vdev.Width + 1

			// 计算容量增益
			result.CapacityGain = s.calculateCapacityGain(vdev, poolInfo.TotalSize)
			result.NewCapacity = poolInfo.TotalSize + result.CapacityGain

			// 磁盘要求
			result.DiskRequirements = DiskRequirements{
				MinSizeGB:     int(poolInfo.TotalSize / uint64(vdev.Width) / (1024 * 1024 * 1024)),
				RecommendedGB: int(poolInfo.TotalSize / uint64(vdev.Width) / (1024 * 1024 * 1024)),
				Interfaces:    []string{"SATA", "NVMe", "SAS"},
				MustMatchSize: false, // OpenZFS 支持不同大小磁盘
			}
		}
	}

	result.Eligible = true
	result.PreChecks = append(result.PreChecks, PreCheckResult{
		Name:     "vdev_type",
		Passed:   true,
		Message:  fmt.Sprintf("%s (%d disks)", result.RAIDZLevel, result.CurrentWidth),
		Required: true,
	})

	// 预估时间
	estimatedTime, _ := s.zfsManager.EstimateExpansionTime(ctx, poolName)
	result.EstimatedTime = estimatedTime

	return result, nil
}

// checkNoActiveScrub 检查是否有活跃的 scrub/resilver.
func (s *RAIDZExpansionService) checkNoActiveScrub(ctx context.Context, poolName string, result *ExpansionEligibilityResult) error {
	cmd := exec.CommandContext(ctx, "zpool", "status", poolName)
	output, err := cmd.Output()
	if err != nil {
		return nil // 无法检查，跳过
	}

	outputStr := string(output)

	// 检查 scrub 状态
	if strings.Contains(outputStr, "scan: scrub in progress") {
		result.Warnings = append(result.Warnings, "当前有 scrub 操作正在进行")
		result.PreChecks = append(result.PreChecks, PreCheckResult{
			Name:     "scrub_status",
			Passed:   false,
			Message:  "scrub in progress",
			Required: true,
		})
		return ErrScrubInProgress
	}

	// 检查 resilver 状态
	if strings.Contains(outputStr, "scan: resilver in progress") {
		result.Warnings = append(result.Warnings, "当前有 resilver 操作正在进行")
		result.PreChecks = append(result.PreChecks, PreCheckResult{
			Name:     "resilver_status",
			Passed:   false,
			Message:  "resilver in progress",
			Required: true,
		})
		return ErrResilverInProgress
	}

	result.PreChecks = append(result.PreChecks, PreCheckResult{
		Name:     "scan_status",
		Passed:   true,
		Message:  "no active scan",
		Required: true,
	})

	return nil
}

// calculateCapacityGain 计算容量增益.
func (s *RAIDZExpansionService) calculateCapacityGain(vdev zfs.VdevExpansionInfo, totalSize uint64) uint64 {
	// RAIDZ 扩展后，新磁盘的容量按数据盘比例分配
	// 公式: 新容量 = 新磁盘容量 * (数据盘数 / 总盘数)
	// 对于 N 盘 RAIDZ-P: 数据盘 = N-P, 扩展后 = N+1 盘

	diskSize := totalSize / uint64(vdev.Width)
	parityDisks := vdev.ParityDisks
	dataDisks := vdev.Width - parityDisks

	// 扩展后数据盘比例
	newDataDisks := dataDisks + 1 // 新盘成为数据盘
	newTotal := vdev.Width + 1

	// 容量增益 = 新磁盘大小 * 新数据盘比例
	return diskSize * uint64(newDataDisks) / uint64(newTotal)
}

// ========== 扩展操作 ==========

// StartExpansion 启动 RAIDZ 扩展任务.
func (s *RAIDZExpansionService) StartExpansion(ctx context.Context, poolName, newDisk string, force bool) (*ExpansionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有任务
	if task, exists := s.activeTasks[poolName]; exists && task.Status == StatusRunning {
		return nil, ErrExpansionAlreadyRuns
	}

	// 检查资格
	eligibility, err := s.CheckExpansionEligibility(ctx, poolName)
	if err != nil {
		return nil, err
	}

	if !eligibility.Eligible && !force {
		return nil, fmt.Errorf("pool not eligible for expansion: %v", eligibility.Warnings)
	}

	// 验证磁盘
	if err := s.zfsManager.ValidateDisk(ctx, newDisk); err != nil {
		return nil, fmt.Errorf("disk validation failed: %w", err)
	}

	// 创建任务
	taskID := generateTaskID(poolName)
	task := &ExpansionTask{
		ID:         taskID,
		PoolName:   poolName,
		NewDisk:    newDisk,
		RAIDZLevel: eligibility.RAIDZLevel,
		Status:     StatusPreparing,
		Progress:   0,
		TotalBytes: eligibility.CurrentCapacity,
		StartTime:  time.Now(),
		CanPause:   true,
		CanCancel:  true,
		Warnings:   eligibility.Warnings,
		LastUpdate: time.Now(),
	}

	s.activeTasks[poolName] = task

	// 解析 RAIDZ 级别
	raidzLevel, _ := zfs.ParseRAIDZLevel(eligibility.RAIDZLevel)

	// 构建 ZFS 扩展配置
	config := zfs.ExpansionConfig{
		PoolName:   poolName,
		NewDisk:    newDisk,
		RAIDZLevel: raidzLevel,
		Force:      force,
	}

	// 异步执行扩展
	go s.executeExpansionAsync(config, task)

	return task, nil
}

// executeExpansionAsync 异步执行扩展.
func (s *RAIDZExpansionService) executeExpansionAsync(config zfs.ExpansionConfig, task *ExpansionTask) {
	ctx := context.Background()

	// 更新状态为运行中
	s.updateTaskStatus(task, StatusRunning)

	// 设置进度回调
	s.zfsManager.SetStateChangeCallback(func(status *zfs.ExpansionStatus) {
		s.updateTaskFromZFS(task, status)
	})

	// 启动 ZFS 扩展
	zfsStatus, err := s.zfsManager.StartExpansion(ctx, config)
	if err != nil {
		s.updateTaskStatus(task, StatusFailed)
		task.Errors = append(task.Errors, err.Error())
		s.addToHistory(task)
		return
	}

	// 监控进度
	s.monitorExpansionProgress(ctx, task, zfsStatus)
}

// monitorExpansionProgress 监控扩展进度.
func (s *RAIDZExpansionService) monitorExpansionProgress(ctx context.Context, task *ExpansionTask, initialStatus *zfs.ExpansionStatus) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取最新状态
			status := s.zfsManager.GetExpansionStatus()
			s.updateTaskFromZFS(task, status)

			// 检查是否完成
			if status.State == zfs.ExpansionStateCompleted {
				s.updateTaskStatus(task, StatusCompleted)
				task.Progress = 100
				s.addToHistory(task)
				return
			}

			if status.State == zfs.ExpansionStateFailed {
				s.updateTaskStatus(task, StatusFailed)
				task.Errors = append(task.Errors, "Expansion failed")
				s.addToHistory(task)
				return
			}

			if status.State == zfs.ExpansionStateCancelled {
				s.updateTaskStatus(task, StatusCancelled)
				s.addToHistory(task)
				return
			}

			// 触发进度回调
			s.triggerProgressCallback(task)

		case <-ctx.Done():
			// 上下文取消
			s.updateTaskStatus(task, StatusCancelled)
			s.addToHistory(task)
			return
		}
	}
}

// updateTaskFromZFS 从 ZFS 状态更新任务.
func (s *RAIDZExpansionService) updateTaskFromZFS(task *ExpansionTask, status *zfs.ExpansionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.Progress = status.Progress
	task.BytesProcessed = status.BytesProcessed
	task.TotalBytes = status.TotalBytes
	task.SpeedMBps = status.Speed / 1024 / 1024 // 转换为 MB/s
	task.ETA = status.EstimatedTimeRemaining
	task.LastUpdate = time.Now()

	if len(status.Errors) > 0 {
		task.Errors = append(task.Errors, status.Errors...)
	}
	if len(status.Warnings) > 0 {
		task.Warnings = append(task.Warnings, status.Warnings...)
	}

	// 更新能力
	task.CanPause = !status.CanResume && status.State == zfs.ExpansionStateRunning
	task.CanCancel = status.CanCancel && status.State == zfs.ExpansionStateRunning
	task.CanResume = status.CanResume && status.State == zfs.ExpansionStatePaused
}

// updateTaskStatus 更新任务状态.
func (s *RAIDZExpansionService) updateTaskStatus(task *ExpansionTask, status ExpansionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.Status = status
	task.LastUpdate = time.Now()

	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		now := time.Now()
		task.EndTime = &now
		task.CanPause = false
		task.CanCancel = false
		task.CanResume = false
	}

	// 触发状态回调
	for _, callback := range s.stateCallbacks {
		go callback(task)
	}
}

// ========== 任务控制 ==========

// PauseExpansion 暂停扩展任务.
func (s *RAIDZExpansionService) PauseExpansion(poolName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.activeTasks[poolName]
	if !exists {
		return fmt.Errorf("no active task for pool %s", poolName)
	}

	if !task.CanPause {
		return fmt.Errorf("task cannot be paused in current state: %s", task.Status)
	}

	if err := s.zfsManager.PauseExpansion(); err != nil {
		return err
	}

	task.Status = StatusPaused
	task.PauseCount++
	task.CanPause = false
	task.CanResume = true
	task.LastUpdate = time.Now()

	return nil
}

// ResumeExpansion 恢复扩展任务.
func (s *RAIDZExpansionService) ResumeExpansion(poolName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.activeTasks[poolName]
	if !exists {
		return fmt.Errorf("no active task for pool %s", poolName)
	}

	if !task.CanResume {
		return fmt.Errorf("task cannot be resumed in current state: %s", task.Status)
	}

	if err := s.zfsManager.ResumeExpansion(); err != nil {
		return err
	}

	task.Status = StatusRunning
	task.CanPause = true
	task.CanResume = false
	task.LastUpdate = time.Now()

	return nil
}

// CancelExpansion 取消扩展任务.
func (s *RAIDZExpansionService) CancelExpansion(poolName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.activeTasks[poolName]
	if !exists {
		return fmt.Errorf("no active task for pool %s", poolName)
	}

	if !task.CanCancel {
		return fmt.Errorf("task cannot be cancelled in current state: %s", task.Status)
	}

	if err := s.zfsManager.CancelExpansion(); err != nil {
		return err
	}

	task.Status = StatusCancelled
	now := time.Now()
	task.EndTime = &now
	task.CanPause = false
	task.CanCancel = false
	task.CanResume = false
	task.LastUpdate = time.Now()

	s.addToHistory(task)
	delete(s.activeTasks, poolName)

	return nil
}

// ========== 状态查询 ==========

// GetExpansionStatus 获取扩展状态.
func (s *RAIDZExpansionService) GetExpansionStatus(poolName string) (*ExpansionTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.activeTasks[poolName]
	if !exists {
		return &ExpansionTask{
			PoolName: poolName,
			Status:   StatusIdle,
		}, nil
	}

	return task, nil
}

// GetAllActiveTasks 获取所有活跃任务.
func (s *RAIDZExpansionService) GetAllActiveTasks() []*ExpansionTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ExpansionTask, 0, len(s.activeTasks))
	for _, task := range s.activeTasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetTaskHistory 获取任务历史.
func (s *RAIDZExpansionService) GetTaskHistory(limit int) []*ExpansionTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.taskHistory) {
		limit = len(s.taskHistory)
	}

	// 返回最近的记录
	start := len(s.taskHistory) - limit
	if start < 0 {
		start = 0
	}

	return s.taskHistory[start:]
}

// ListAvailableDisks 获取可用磁盘列表.
func (s *RAIDZExpansionService) ListAvailableDisks(ctx context.Context) ([]AvailableDiskInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.zfsManager.IsAvailable() {
		return nil, zfs.ErrZFSNotAvailable
	}

	// 获取 ZFS 管理器的磁盘列表
	diskPaths, err := s.zfsManager.ListAvailableDisks(ctx)
	if err != nil {
		return nil, err
	}

	// 扩展磁盘信息
	var disks []AvailableDiskInfo
	for _, path := range diskPaths {
		diskInfo := s.getDiskDetails(ctx, path)
		if diskInfo != nil {
			disks = append(disks, *diskInfo)
		}
	}

	return disks, nil
}

// AvailableDiskInfo 可用磁盘信息.
type AvailableDiskInfo struct {
	Path      string `json:"path"`
	Model     string `json:"model"`
	SizeGB    int    `json:"size_gb"`
	Interface string `json:"interface"`
	Healthy   bool   `json:"healthy"`
	Available bool   `json:"available"`
}

// getDiskDetails 获取磁盘详细信息.
func (s *RAIDZExpansionService) getDiskDetails(ctx context.Context, diskPath string) *AvailableDiskInfo {
	info := &AvailableDiskInfo{
		Path:      diskPath,
		Available: true,
	}

	// 使用 lsblk 获取磁盘信息
	cmd := exec.CommandContext(ctx, "lsblk", "-b", "-d", "-o", "SIZE,MODEL,ROTA", diskPath)
	output, err := cmd.Output()
	if err != nil {
		return info
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 1 {
			sizeBytes, _ := strconv.ParseUint(fields[0], 10, 64)
			info.SizeGB = int(sizeBytes / (1024 * 1024 * 1024))
		}
		if len(fields) >= 2 {
			info.Model = fields[1]
		}
		if len(fields) >= 3 {
			// ROTA: 0 = SSD, 1 = HDD
			if fields[2] == "0" {
				info.Interface = "SSD"
			} else {
				info.Interface = "HDD"
			}
		}
	}

	// 检查 SMART 健康状态（可选）
	smartCmd := exec.CommandContext(ctx, "smartctl", "-H", diskPath)
	smartOutput, err := smartCmd.Output()
	if err == nil {
		info.Healthy = strings.Contains(string(smartOutput), "PASSED")
	} else {
		info.Healthy = true // 默认健康
	}

	return info
}

// ========== 回调注册 ==========

// RegisterProgressCallback 注册进度回调.
func (s *RAIDZExpansionService) RegisterProgressCallback(poolName string, callback func(*ExpansionProgress)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressCallbacks[poolName] = callback
}

// RegisterStateCallback 注册状态变更回调.
func (s *RAIDZExpansionService) RegisterStateCallback(callback func(*ExpansionTask)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateCallbacks = append(s.stateCallbacks, callback)
}

// triggerProgressCallback 触发进度回调.
func (s *RAIDZExpansionService) triggerProgressCallback(task *ExpansionTask) {
	s.mu.RLock()
	callback, exists := s.progressCallbacks[task.PoolName]
	s.mu.RUnlock()

	if exists && callback != nil {
		progress := &ExpansionProgress{
			TaskID:         task.ID,
			Percentage:     task.Progress,
			BytesProcessed: task.BytesProcessed,
			BytesTotal:     task.TotalBytes,
			SpeedMBps:      task.SpeedMBps,
			ETA:            task.ETA,
			Elapsed:        time.Since(task.StartTime),
			Phase:          "data_migration",
			PhaseProgress:  task.Progress,
			LastUpdate:     task.LastUpdate,
		}
		go callback(progress)
	}
}

// ========== 历史管理 ==========

// addToHistory 添加到历史记录.
func (s *RAIDZExpansionService) addToHistory(task *ExpansionTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从活跃任务移除
	delete(s.activeTasks, task.PoolName)

	// 添加到历史
	s.taskHistory = append(s.taskHistory, task)

	// 保留最近 100 条
	if len(s.taskHistory) > 100 {
		s.taskHistory = s.taskHistory[len(s.taskHistory)-100:]
	}

	// 保存到文件
	_ = s.saveHistory()
}

// loadHistory 加载历史记录.
func (s *RAIDZExpansionService) loadHistory() error {
	if s.configPath == "" {
		return nil
	}

	historyPath := filepath.Join(filepath.Dir(s.configPath), "raidz_expansion_history.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.taskHistory)
}

// saveHistory 保存历史记录.
func (s *RAIDZExpansionService) saveHistory() error {
	if s.configPath == "" {
		return nil
	}

	historyPath := filepath.Join(filepath.Dir(s.configPath), "raidz_expansion_history.json")
	data, err := json.MarshalIndent(s.taskHistory, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0640)
}

// ========== 辅助方法 ==========

// generateTaskID 生成任务 ID.
func generateTaskID(poolName string) string {
	return fmt.Sprintf("raidz-exp-%s-%d", poolName, time.Now().UnixNano())
}

// EstimateExpansionTime 预估扩展时间.
func (s *RAIDZExpansionService) EstimateExpansionTime(ctx context.Context, poolName string) (time.Duration, error) {
	return s.zfsManager.EstimateExpansionTime(ctx, poolName)
}

// IsAvailable 检查服务是否可用.
func (s *RAIDZExpansionService) IsAvailable() bool {
	return s.zfsManager.IsAvailable()
}

// Close 关闭服务.
func (s *RAIDZExpansionService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 取消所有活跃任务
	for _, task := range s.activeTasks {
		if task.Status == StatusRunning || task.Status == StatusPaused {
			_ = s.zfsManager.CancelExpansion()
			task.Status = StatusCancelled
			now := time.Now()
			task.EndTime = &now
		}
	}

	// 保存历史
	_ = s.saveHistory()

	// 关闭 ZFS 管理器
	return s.zfsManager.Close()
}
