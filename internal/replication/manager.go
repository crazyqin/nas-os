package replication

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Type 复制类型.
type Type string

// 复制类型常量.
const (
	TypeRealtime      Type = "realtime"      // 实时同步
	TypeScheduled     Type = "scheduled"     // 定时复制
	TypeBidirectional Type = "bidirectional" // 双向复制
)

// Status 复制状态.
type Status string

// 复制状态常量.
const (
	StatusIdle      Status = "idle"      // 空闲
	StatusSyncing   Status = "syncing"   // 同步中
	StatusPaused    Status = "paused"    // 已暂停
	StatusError     Status = "error"     // 错误
	StatusCompleted Status = "completed" // 已完成
)

// Task 复制任务.
type Task struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	SourcePath       string    `json:"source_path"`
	TargetPath       string    `json:"target_path"`
	TargetHost       string    `json:"target_host,omitempty"` // 空表示本地
	Type             Type      `json:"type"`
	Status           Status    `json:"status"`
	Schedule         string    `json:"schedule,omitempty"` // cron 表达式
	LastSyncAt       time.Time `json:"last_sync_at,omitempty"`
	NextSyncAt       time.Time `json:"next_sync_at,omitempty"`
	BytesTransferred int64     `json:"bytes_transferred"`
	TotalBytes       int64     `json:"total_bytes"`
	FilesCount       int       `json:"files_count"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Enabled          bool      `json:"enabled"`
	Compress         bool      `json:"compress"`
	DeleteExtraneous bool      `json:"delete_extraneous"` // 删除目标端多余文件
}

// Config 复制配置.
type Config struct {
	MaxConcurrentTasks int    `json:"max_concurrent"`
	BandwidthLimit     int    `json:"bandwidth_limit"` // KB/s, 0 表示不限
	SSHKeyPath         string `json:"ssh_key_path"`
	Retries            int    `json:"retries"`
	Timeout            int    `json:"timeout"` // 秒
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		MaxConcurrentTasks: 2,
		BandwidthLimit:     0,
		SSHKeyPath:         "~/.ssh/id_rsa",
		Retries:            3,
		Timeout:            3600,
	}
}

// Manager 复制管理器.
type Manager struct {
	mu          sync.RWMutex
	config      *Config
	tasks       map[string]*Task
	configPath  string
	stopChan    chan struct{}
	wg          sync.WaitGroup
	watcher     *Watcher                  // 实时监控器
	bidiSync    *BidirectionalSyncManager // 双向同步管理器
	conflictDet *ConflictDetector         // 冲突检测器
}

// NewManager 创建复制管理器.
func NewManager(configPath string, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 创建冲突检测器
	conflictDetector := NewConflictDetector(ConflictNewerWins)

	// 创建文件监控器
	watcher, err := NewWatcher(conflictDetector)
	if err != nil {
		return nil, fmt.Errorf("创建监控器失败：%w", err)
	}

	m := &Manager{
		config:      config,
		tasks:       make(map[string]*Task),
		configPath:  configPath,
		stopChan:    make(chan struct{}),
		watcher:     watcher,
		bidiSync:    NewBidirectionalSyncManager(watcher, conflictDetector),
		conflictDet: conflictDetector,
	}

	// 加载配置
	if err := m.loadConfig(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// 保存默认配置
		if err := m.saveConfig(); err != nil {
			return nil, err
		}
	}

	// 启动调度器
	go m.startScheduler()

	// 启动实时监控
	m.watcher.Start()
	m.bidiSync.Start()

	return m, nil
}

// CreateTask 创建复制任务.
func (m *Manager) CreateTask(task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task.ID = generateTaskID()
	task.Status = StatusIdle
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	// 计算下次同步时间
	if task.Type == TypeScheduled && task.Schedule != "" {
		if err := m.calculateNextSync(task); err != nil {
			return err
		}
	}

	m.tasks[task.ID] = task
	return m.saveConfig()
}

// UpdateTask 更新复制任务.
func (m *Manager) UpdateTask(id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("任务不存在：%s", id)
	}

	for key, value := range updates {
		switch key {
		case "name":
			if v, ok := value.(string); ok {
				task.Name = v
			}
		case "schedule":
			if v, ok := value.(string); ok {
				task.Schedule = v
				if err := m.calculateNextSync(task); err != nil {
					return err
				}
			}
		case "enabled":
			if v, ok := value.(bool); ok {
				task.Enabled = v
			}
		case "compress":
			if v, ok := value.(bool); ok {
				task.Compress = v
			}
		case "delete_extraneous":
			if v, ok := value.(bool); ok {
				task.DeleteExtraneous = v
			}
		}
	}

	task.UpdatedAt = time.Now()
	return m.saveConfig()
}

// DeleteTask 删除复制任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; !exists {
		return fmt.Errorf("任务不存在：%s", id)
	}

	delete(m.tasks, id)
	return m.saveConfig()
}

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetTask 获取任务详情.
func (m *Manager) GetTask(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在：%s", id)
	}
	return task, nil
}

// StartSync 手动触发同步.
func (m *Manager) StartSync(id string) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("任务不存在：%s", id)
	}

	if task.Status == StatusSyncing {
		m.mu.Unlock()
		return fmt.Errorf("任务正在同步中")
	}

	task.Status = StatusSyncing
	task.ErrorMessage = ""
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.executeSync(task)
	}()

	return nil
}

// PauseTask 暂停任务.
func (m *Manager) PauseTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("任务不存在：%s", id)
	}

	task.Status = StatusPaused
	task.UpdatedAt = time.Now()
	return m.saveConfig()
}

// ResumeTask 恢复任务.
func (m *Manager) ResumeTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("任务不存在：%s", id)
	}

	task.Status = StatusIdle
	task.UpdatedAt = time.Now()
	return m.saveConfig()
}

// executeSync 执行同步.
func (m *Manager) executeSync(task *Task) {
	startTime := time.Now()

	// 构建 rsync 命令
	args := []string{
		"-av",
		"--progress",
	}

	// 带宽限制
	if m.config.BandwidthLimit > 0 {
		args = append(args, "--bwlimit", fmt.Sprintf("%d", m.config.BandwidthLimit))
	}

	// 压缩
	if task.Compress {
		args = append(args, "-z")
	}

	// 删除多余文件
	if task.DeleteExtraneous {
		args = append(args, "--delete")
	}

	source := task.SourcePath
	target := task.TargetPath

	// 远程目标
	if task.TargetHost != "" {
		target = fmt.Sprintf("%s:%s", task.TargetHost, task.TargetPath)
	}

	args = append(args, source, target)

	cmd := exec.Command("rsync", args...)

	// 执行命令
	output, err := cmd.CombinedOutput()

	m.mu.Lock()
	defer m.mu.Unlock()

	task.UpdatedAt = time.Now()
	task.LastSyncAt = startTime

	if err != nil {
		task.Status = StatusError
		task.ErrorMessage = string(output)
	} else {
		task.Status = StatusCompleted
		task.ErrorMessage = ""
		// 解析输出获取统计信息
		m.parseRsyncOutput(task, string(output))
	}

	// 计算下次同步时间
	if task.Type == TypeScheduled {
		_ = m.calculateNextSync(task)
	}

	_ = m.saveConfig()
}

// parseRsyncOutput 解析 rsync 输出获取详细统计.
func (m *Manager) parseRsyncOutput(task *Task, output string) {
	task.FilesCount = 0
	task.BytesTransferred = 0

	// 解析发送字节数: "sent 123,456 bytes"
	if sentMatch := regexp.MustCompile(`sent\s+([\d,]+)\s+bytes`).FindStringSubmatch(output); len(sentMatch) > 1 {
		if val, err := parseNumber(sentMatch[1]); err == nil {
			task.BytesTransferred = val
		}
	}

	// 解析总大小: "total size is 1,234,567,890"
	if sizeMatch := regexp.MustCompile(`total size is\s+([\d,]+)`).FindStringSubmatch(output); len(sizeMatch) > 1 {
		if val, err := parseNumber(sizeMatch[1]); err == nil {
			task.TotalBytes = val
		}
	}

	// 解析传输文件数: "Number of regular files transferred: 567"
	if filesMatch := regexp.MustCompile(`Number of regular files transferred:\s+(\d+)`).FindStringSubmatch(output); len(filesMatch) > 1 {
		if val, err := strconv.Atoi(filesMatch[1]); err == nil {
			task.FilesCount = val
		}
	}

	// 备选：如果没有找到传输文件数，尝试解析文件列表
	// rsync -v 会列出每个文件，格式如: "filename"
	if task.FilesCount == 0 {
		lines := strings.Split(output, "\n")
		count := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// 跳过空行和统计行
			if line == "" || strings.HasPrefix(line, "sent ") ||
				strings.HasPrefix(line, "total size") ||
				strings.HasPrefix(line, "speedup") ||
				strings.Contains(line, "bytes/sec") {
				continue
			}
			// 简单计数非统计行
			count++
		}
		if count > 0 {
			task.FilesCount = count
		}
	}
}

// RsyncStats rsync 详细统计信息.
type RsyncStats struct {
	BytesSent        int64   `json:"bytes_sent"`        // 发送字节数
	BytesReceived    int64   `json:"bytes_received"`    // 接收字节数
	BytesPerSecond   float64 `json:"bytes_per_second"`  // 传输速度 (bytes/sec)
	TotalSize        int64   `json:"total_size"`        // 总大小
	Speedup          float64 `json:"speedup"`           // 加速比
	FilesTransferred int     `json:"files_transferred"` // 传输文件数
	TotalFiles       int     `json:"total_files"`       // 总文件数
}

// parseNumber 解析带逗号的数字字符串.
func parseNumber(s string) (int64, error) {
	// 移除逗号
	s = strings.ReplaceAll(s, ",", "")
	return strconv.ParseInt(s, 10, 64)
}

// calculateNextSync 计算下次同步时间.
func (m *Manager) calculateNextSync(task *Task) error {
	if task.Schedule == "" {
		return nil
	}

	// 简单实现：每小时/每天/每周
	switch task.Schedule {
	case "hourly":
		task.NextSyncAt = time.Now().Add(time.Hour)
	case "daily":
		task.NextSyncAt = time.Now().Add(24 * time.Hour)
	case "weekly":
		task.NextSyncAt = time.Now().Add(7 * 24 * time.Hour)
	default:
		// cron 表达式解析 (简化实现)
		task.NextSyncAt = time.Now().Add(time.Hour)
	}

	return nil
}

// startScheduler 启动调度器.
func (m *Manager) startScheduler() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkScheduledTasks()
		case <-m.stopChan:
			return
		}
	}
}

// checkScheduledTasks 检查定时任务.
func (m *Manager) checkScheduledTasks() {
	m.mu.RLock()
	var toSync []*Task
	now := time.Now()

	for _, task := range m.tasks {
		if task.Enabled &&
			task.Type == TypeScheduled &&
			task.Status != StatusSyncing &&
			!task.NextSyncAt.IsZero() &&
			now.After(task.NextSyncAt) {
			toSync = append(toSync, task)
		}
	}
	m.mu.RUnlock()

	for _, task := range toSync {
		_ = m.StartSync(task.ID)
	}
}

// loadConfig 加载配置.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, task := range tasks {
		m.tasks[task.ID] = task
	}

	return nil
}

// saveConfig 保存配置.
func (m *Manager) saveConfig() error {
	// 确保目录存在
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0640)
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	close(m.stopChan)
	m.wg.Wait()

	// 停止实时监控
	if m.watcher != nil {
		m.watcher.Stop()
	}
	if m.bidiSync != nil {
		m.bidiSync.Stop()
	}
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var (
		total      = len(m.tasks)
		syncing    = 0
		paused     = 0
		errors     = 0
		totalBytes int64
	)

	for _, task := range m.tasks {
		switch task.Status {
		case StatusSyncing:
			syncing++
		case StatusPaused:
			paused++
		case StatusError:
			errors++
		}
		totalBytes += task.BytesTransferred
	}

	// 获取监控统计
	var watcherStats *WatcherStats
	if m.watcher != nil {
		stats := m.watcher.GetStats()
		watcherStats = &stats
	}

	return map[string]interface{}{
		"total_tasks":       total,
		"syncing":           syncing,
		"paused":            paused,
		"errors":            errors,
		"bytes_transferred": totalBytes,
		"watcher_stats":     watcherStats,
	}
}

// GetConflicts 获取冲突列表.
func (m *Manager) GetConflicts(taskID string) []*ConflictInfo {
	if m.conflictDet == nil {
		return nil
	}
	return m.conflictDet.GetConflicts(taskID)
}

// ResolveConflict 手动解决冲突.
func (m *Manager) ResolveConflict(conflictID string, strategy ConflictStrategy) error {
	if m.conflictDet == nil {
		return fmt.Errorf("冲突检测器未初始化")
	}

	m.mu.RLock()
	conflicts := m.conflictDet.GetConflicts("")
	var conflict *ConflictInfo
	for _, c := range conflicts {
		if c.ID == conflictID {
			conflict = c
			break
		}
	}
	m.mu.RUnlock()

	if conflict == nil {
		return fmt.Errorf("冲突不存在：%s", conflictID)
	}

	conflict.Strategy = strategy
	return m.conflictDet.ResolveConflict(conflict)
}

// generateTaskID 生成任务 ID.
func generateTaskID() string {
	return fmt.Sprintf("repl-%d", time.Now().UnixNano())
}
