// Package usbbackup 提供 USB 设备备份管理功能
package usbbackup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ========== 备份管理器 ==========

// Manager 备份管理器.
type Manager struct {
	mu sync.RWMutex

	// tasks 备份任务列表
	tasks map[string]*BackupTask

	// progresses 备份进度
	progresses map[string]*BackupProgress

	// config 配置
	config *Config

	// detector 设备检测器
	detector *Detector

	// cron 调度器
	cron *cron.Cron

	// cronEntries cron 条目映射
	cronEntries map[string]cron.EntryID

	// running 是否运行中
	running bool

	// ctx / cancel
	ctx    context.Context
	cancel context.CancelFunc

	// taskSem 任务并发信号量
	taskSem chan struct{}
}

// NewManager 创建备份管理器.
func NewManager(config *Config, detector *Detector) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		tasks:       make(map[string]*BackupTask),
		progresses:  make(map[string]*BackupProgress),
		config:      config,
		detector:    detector,
		cron:        cron.New(),
		cronEntries: make(map[string]cron.EntryID),
		ctx:         ctx,
		cancel:      cancel,
		taskSem:     make(chan struct{}, config.MaxConcurrentTasks),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 注册设备事件回调
	if m.detector != nil {
		m.detector.OnEvent(m.onDeviceEvent)
	}

	m.cron.Start()
	m.running = true
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	m.cron.Stop()
	m.running = false
}

// IsRunning 是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== 任务管理 ==========

// CreateTask 创建备份任务.
func (m *Manager) CreateTask(req *CreateTaskRequest) (*BackupTask, error) {
	if err := m.validateCreateRequest(req); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	task := &BackupTask{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Direction:   req.Direction,
		Policy:      req.Policy,
		SourcePath:  req.SourcePath,
		DestPath:    req.DestPath,
		DeviceID:    req.DeviceID,
		CronExpr:   req.CronExpr,
		Filter:      req.Filter,
		Incremental: req.Incremental,
		Enabled:     true,
		Status:      TaskStatusIdle,
		CreatedAt:   now,
	}

	m.tasks[task.ID] = task

	// 如果是定时任务，注册到 cron
	if task.Policy == PolicyScheduled && task.CronExpr != "" {
		if err := m.scheduleTask(task); err != nil {
			delete(m.tasks, task.ID)
			return nil, fmt.Errorf("注册定时任务失败: %w", err)
		}
	}

	return task, nil
}

// DeleteTask 删除备份任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	if task.Status == TaskStatusRunning {
		return ErrTaskRunning
	}

	// 移除 cron 条目
	if entryID, exists := m.cronEntries[id]; exists {
		m.cron.Remove(entryID)
		delete(m.cronEntries, id)
	}

	delete(m.tasks, id)
	delete(m.progresses, id)
	return nil
}

// GetTask 获取备份任务.
func (m *Manager) GetTask(id string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有备份任务.
func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BackupTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// UpdateTask 更新备份任务.
func (m *Manager) UpdateTask(id string, req *CreateTaskRequest) (*BackupTask, error) {
	if err := m.validateCreateRequest(req); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.Status == TaskStatusRunning {
		return nil, ErrTaskRunning
	}

	// 移除旧 cron
	if entryID, exists := m.cronEntries[id]; exists {
		m.cron.Remove(entryID)
		delete(m.cronEntries, id)
	}

	task.Name = req.Name
	task.Direction = req.Direction
	task.Policy = req.Policy
	task.SourcePath = req.SourcePath
	task.DestPath = req.DestPath
	task.DeviceID = req.DeviceID
	task.CronExpr = req.CronExpr
	task.Filter = req.Filter
	task.Incremental = req.Incremental

	// 注册新 cron
	if task.Policy == PolicyScheduled && task.CronExpr != "" {
		if err := m.scheduleTask(task); err != nil {
			return nil, fmt.Errorf("注册定时任务失败: %w", err)
		}
	}

	return task, nil
}

// ========== 任务控制 ==========

// PauseTask 暂停备份任务.
func (m *Manager) PauseTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	if task.Status == TaskStatusRunning {
		return ErrTaskRunning
	}

	task.Status = TaskStatusPaused
	task.Enabled = false
	return nil
}

// ResumeTask 恢复备份任务.
func (m *Manager) ResumeTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusPaused {
		return fmt.Errorf("任务不在暂停状态")
	}

	task.Status = TaskStatusIdle
	task.Enabled = true
	return nil
}

// TriggerTask 手动触发备份任务.
func (m *Manager) TriggerTask(id string) (*BackupProgress, error) {
	m.mu.RLock()
	task, ok := m.tasks[id]
	if !ok {
		m.mu.RUnlock()
		return nil, ErrTaskNotFound
	}
	if task.Status == TaskStatusRunning {
		m.mu.RUnlock()
		return nil, ErrTaskRunning
	}
	if task.Status == TaskStatusPaused {
		m.mu.RUnlock()
		return nil, ErrTaskPaused
	}
	m.mu.RUnlock()

	go m.executeTask(id)
	return m.GetProgress(id), nil
}

// ========== 进度查询 ==========

// GetProgress 获取任务进度.
func (m *Manager) GetProgress(taskID string) *BackupProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if p, ok := m.progresses[taskID]; ok {
		return p
	}
	return &BackupProgress{
		TaskID: taskID,
		Status: TaskStatusIdle,
	}
}

// GetAllProgress 获取所有任务进度.
func (m *Manager) GetAllProgress() []*BackupProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BackupProgress, 0, len(m.progresses))
	for _, p := range m.progresses {
		result = append(result, p)
	}
	return result
}

// ========== 设备事件处理 ==========

// onDeviceEvent 处理设备事件.
func (m *Manager) onDeviceEvent(event USBEvent) {
	if event.Type != USBEventDeviceConnected {
		return
	}

	// 检查是否有匹配的 "插入即备份" 任务
	m.mu.RLock()
	var matchedTasks []string
	for _, task := range m.tasks {
		if !task.Enabled {
			continue
		}
		if task.Policy != PolicyOnInsert {
			continue
		}
		// 匹配设备 ID（空表示任意设备）
		if task.DeviceID != "" && task.DeviceID != event.Device.ID {
			continue
		}
		matchedTasks = append(matchedTasks, task.ID)
	}
	m.mu.RUnlock()

	// 触发备份
	for _, taskID := range matchedTasks {
		go m.executeTask(taskID)
	}
}

// ========== 备份执行 ==========

// executeTask 执行备份任务.
func (m *Manager) executeTask(taskID string) {
	// 并发控制
	select {
	case m.taskSem <- struct{}{}:
		defer func() { <-m.taskSem }()
	case <-m.ctx.Done():
		return
	}

	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return
	}
	if task.Status == TaskStatusPaused {
		m.mu.Unlock()
		return
	}

	task.Status = TaskStatusRunning
	now := time.Now()
	task.LastRun = &now

	progress := &BackupProgress{
		TaskID:    taskID,
		Status:    TaskStatusRunning,
		StartedAt: now,
		UpdatedAt: now,
	}
	m.progresses[taskID] = progress
	m.mu.Unlock()

	// 执行同步
	var err error
	switch task.Direction {
	case DirectionNasToUSB:
		err = m.syncFiles(task.SourcePath, task.DestPath, task.Filter, task.Incremental, progress)
	case DirectionUSBToNAS:
		err = m.syncFiles(task.SourcePath, task.DestPath, task.Filter, task.Incremental, progress)
	case DirectionBidirectional:
		err = m.bidirectionalSync(task.SourcePath, task.DestPath, task.Filter, task.Incremental, progress)
	default:
		err = ErrInvalidDirection
	}

	// 更新状态
	m.mu.Lock()
	if err != nil {
		task.Status = TaskStatusFailed
		task.LastResult = err.Error()
		progress.Status = TaskStatusFailed
		progress.Error = err.Error()
	} else {
		task.Status = TaskStatusCompleted
		task.LastResult = "success"
		progress.Status = TaskStatusCompleted
	}
	progress.UpdatedAt = time.Now()

	// 重置为空闲
	if task.Status == TaskStatusCompleted {
		task.Status = TaskStatusIdle
	}
	m.mu.Unlock()
}

// syncFiles 同步文件.
func (m *Manager) syncFiles(srcDir, dstDir string, filter *FileFilter, incremental bool, progress *BackupProgress) error {
	// 确保目标目录存在
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 收集源文件
	srcFiles, err := m.collectFiles(srcDir, filter)
	if err != nil {
		return fmt.Errorf("扫描源文件失败: %w", err)
	}

	m.mu.Lock()
	progress.TotalFiles = len(srcFiles)
	m.mu.Unlock()

	var totalBytes int64
	for _, f := range srcFiles {
		totalBytes += f.size
	}

	m.mu.Lock()
	progress.TotalBytes = totalBytes
	m.mu.Unlock()

	// 逐文件复制
	for _, f := range srcFiles {
		select {
		case <-m.ctx.Done():
			return fmt.Errorf("任务被取消")
		default:
		}

		relPath, _ := filepath.Rel(srcDir, f.path)
		dstPath := filepath.Join(dstDir, relPath)

		m.mu.Lock()
		progress.CurrentFile = relPath
		progress.UpdatedAt = time.Now()
		m.mu.Unlock()

		// 增量检查
		if incremental {
			if dstInfo, err := os.Stat(dstPath); err == nil {
				if !dstInfo.ModTime().Before(f.modTime) {
					m.mu.Lock()
					progress.SkippedFiles++
					m.mu.Unlock()
					continue
				}
			}
		}

		// 确保子目录存在
		if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
			m.mu.Lock()
			progress.FailedFiles++
			m.mu.Unlock()
			continue
		}

		// 复制文件
		if err := copyFile(f.path, dstPath); err != nil {
			m.mu.Lock()
			progress.FailedFiles++
			m.mu.Unlock()
			continue
		}

		// 保留修改时间
		_ = os.Chtimes(dstPath, f.modTime, f.modTime)

		m.mu.Lock()
		progress.CopiedFiles++
		progress.CopiedBytes += f.size
		progress.UpdatedAt = time.Now()
		m.mu.Unlock()
	}

	return nil
}

// bidirectionalSync 双向同步.
func (m *Manager) bidirectionalSync(dir1, dir2 string, filter *FileFilter, incremental bool, progress *BackupProgress) error {
	// 先从 dir1 → dir2
	if err := m.syncFiles(dir1, dir2, filter, incremental, progress); err != nil {
		return err
	}
	// 再从 dir2 → dir1
	return m.syncFiles(dir2, dir1, filter, incremental, progress)
}

// ========== 文件操作 ==========

// fileEntry 收集到的文件信息.
type fileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// collectFiles 收集目录下所有符合条件的文件.
func (m *Manager) collectFiles(dir string, filter *FileFilter) ([]fileEntry, error) {
	var files []fileEntry

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}

		// 应用过滤器
		if filter != nil && !matchFilter(path, info, filter) {
			return nil
		}

		files = append(files, fileEntry{
			path:    path,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		return nil
	})

	return files, err
}

// matchFilter 检查文件是否匹配过滤条件.
func matchFilter(path string, info os.FileInfo, filter *FileFilter) bool {
	// 扩展名过滤
	if len(filter.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		matched := false
		for _, allowed := range filter.Extensions {
			if strings.ToLower(allowed) == ext {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 文件大小过滤
	if filter.MaxFileSize > 0 && info.Size() > filter.MaxFileSize {
		return false
	}
	if filter.MinFileSize > 0 && info.Size() < filter.MinFileSize {
		return false
	}

	// 日期过滤
	if filter.ModifiedAfter != nil && info.ModTime().Before(*filter.ModifiedAfter) {
		return false
	}
	if filter.ModifiedBefore != nil && info.ModTime().After(*filter.ModifiedBefore) {
		return false
	}

	// 排除模式
	if len(filter.ExcludePatterns) > 0 {
		for _, pattern := range filter.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return false
			}
		}
	}

	return true
}

// copyFile 复制文件.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ========== 定时调度 ==========

// scheduleTask 注册定时任务.
func (m *Manager) scheduleTask(task *BackupTask) error {
	entryID, err := m.cron.AddFunc(task.CronExpr, func() {
		if task.Enabled {
			go m.executeTask(task.ID)
		}
	})
	if err != nil {
		return err
	}
	m.cronEntries[task.ID] = entryID
	return nil
}

// ========== 校验 ==========

// validateCreateRequest 校验创建请求.
func (m *Manager) validateCreateRequest(req *CreateTaskRequest) error {
	if req.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}
	if req.SourcePath == "" {
		return ErrSourcePathEmpty
	}
	if req.DestPath == "" {
		return ErrDestPathEmpty
	}

	// 校验方向
	switch req.Direction {
	case DirectionNasToUSB, DirectionUSBToNAS, DirectionBidirectional:
	default:
		return ErrInvalidDirection
	}

	// 校验策略
	switch req.Policy {
	case PolicyOnInsert, PolicyScheduled, PolicyManual:
	default:
		return ErrInvalidPolicy
	}

	// 定时备份必须有 cron 表达式
	if req.Policy == PolicyScheduled && req.CronExpr == "" {
		return fmt.Errorf("定时备份必须指定 cron 表达式")
	}

	return nil
}
