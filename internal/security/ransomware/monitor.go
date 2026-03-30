// Package ransomware provides ransomware detection and protection for NAS-OS
// File: monitor.go - 文件事件监控器（fsnotify/inotify封装）
package ransomware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

// FileEventMonitor 文件事件监控器
// 使用fsnotify实时捕获文件系统事件
type FileEventMonitor struct {
	config            MonitorConfig
	watcher           *fsnotify.Watcher
	eventChan         chan FileEvent
	internalChan      chan fsnotify.Event
	errorChan         chan error
	buffer            *EventBuffer
	rateLimiter       *RateLimiter
	processInfoGetter ProcessInfoGetter
	running           bool
	mu                sync.RWMutex
	watchedPaths      map[string]bool
	startTime         time.Time
	stats             MonitorStats
	statsMu           sync.RWMutex
}

// MonitorStats 监控器统计信息
type MonitorStats struct {
	TotalEvents    int64                   `json:"total_events"`
	EventsByType   map[FileOperation]int64 `json:"events_by_type"`
	EventsByPath   map[string]int64        `json:"events_by_path"`
	LastError      string                  `json:"last_error,omitempty"`
	LastErrorTime  *time.Time              `json:"last_error_time,omitempty"`
	WatchedPaths   int                     `json:"watched_paths"`
	BufferUsage    int                     `json:"buffer_usage"`
	BufferCapacity int                     `json:"buffer_capacity"`
}

// EventBuffer 事件缓冲队列
type EventBuffer struct {
	events   []FileEvent
	capacity int
	position int
	mu       sync.RWMutex
	overflow int64
}

// RateLimiter 速率限制器
type RateLimiter struct {
	maxEventsPerSecond int
	eventCounts        map[string]int
	lastReset          time.Time
	mu                 sync.Mutex
}

// ProcessInfoGetter 进程信息获取接口
type ProcessInfoGetter interface {
	GetProcessInfo(path string) (*ProcessInfo, error)
}

// NewFileEventMonitor 创建文件事件监控器
func NewFileEventMonitor(config MonitorConfig) (*FileEventMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	// 确保缓冲区大小合理
	bufferSize := config.MaxEvents
	if bufferSize <= 0 {
		bufferSize = 10000
	}

	monitor := &FileEventMonitor{
		config:       config,
		watcher:      watcher,
		eventChan:    make(chan FileEvent, bufferSize),
		internalChan: watcher.Events,
		errorChan:    make(chan error, 100),
		buffer:       NewEventBuffer(bufferSize),
		rateLimiter:  NewRateLimiter(1000), // 默认1000事件/秒
		watchedPaths: make(map[string]bool),
		startTime:    time.Now(),
	}

	monitor.stats.EventsByType = make(map[FileOperation]int64)
	monitor.stats.EventsByPath = make(map[string]int64)

	return monitor, nil
}

// SetProcessInfoGetter 设置进程信息获取器
func (m *FileEventMonitor) SetProcessInfoGetter(getter ProcessInfoGetter) {
	m.processInfoGetter = getter
}

// Start 启动监控器
func (m *FileEventMonitor) Start(ctx context.Context) (<-chan FileEvent, <-chan error, error) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return m.eventChan, m.errorChan, fmt.Errorf("monitor already running")
	}
	m.running = true
	m.mu.Unlock()

	// 添加监控路径
	for _, path := range m.config.WatchPaths {
		if err := m.AddWatchPath(path); err != nil {
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			return nil, nil, fmt.Errorf("failed to add watch path %s: %w", path, err)
		}
	}

	// 启动事件处理循环
	go m.processLoop(ctx)

	// 启动错误处理循环
	go m.errorLoop(ctx)

	// 启动速率限制器重置
	go m.rateLimiterResetLoop(ctx)

	return m.eventChan, m.errorChan, nil
}

// Stop 停止监控器
func (m *FileEventMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false

	if m.watcher != nil {
		return m.watcher.Close()
	}

	close(m.eventChan)
	close(m.errorChan)

	return nil
}

// AddWatchPath 添加监控路径
func (m *FileEventMonitor) AddWatchPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查路径是否已监控
	if m.watchedPaths[path] {
		return nil
	}

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// 检查是否在排除路径中
	for _, exclude := range m.config.ExcludePaths {
		if strings.HasPrefix(path, exclude) {
			return nil // 排除路径不监控
		}
	}

	// 添加监控
	if info.IsDir() {
		// 递归添加目录监控
		err := m.addRecursiveWatch(path)
		if err != nil {
			return err
		}
	} else {
		// 添加单个文件监控
		if err := m.watcher.Add(path); err != nil {
			return fmt.Errorf("failed to watch file %s: %w", path, err)
		}
	}

	m.watchedPaths[path] = true
	m.statsMu.Lock()
	m.stats.WatchedPaths = len(m.watchedPaths)
	m.statsMu.Unlock()

	return nil
}

// RemoveWatchPath 移除监控路径
func (m *FileEventMonitor) RemoveWatchPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.watchedPaths[path] {
		return nil
	}

	if err := m.watcher.Remove(path); err != nil {
		return fmt.Errorf("failed to remove watch for %s: %w", path, err)
	}

	delete(m.watchedPaths, path)
	m.statsMu.Lock()
	m.stats.WatchedPaths = len(m.watchedPaths)
	m.statsMu.Unlock()

	return nil
}

// addRecursiveWatch 递归添加目录监控
func (m *FileEventMonitor) addRecursiveWatch(rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		// 检查是否在排除路径中
		for _, exclude := range m.config.ExcludePaths {
			if strings.HasPrefix(path, exclude) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 只监控目录（fsnotify会自动监控目录内的文件）
		if info.IsDir() {
			if err := m.watcher.Add(path); err != nil {
				// 记录错误但继续
				m.sendError(fmt.Errorf("failed to watch directory %s: %w", path, err))
				return nil
			}
		}

		return nil
	})
}

// processLoop 事件处理循环
func (m *FileEventMonitor) processLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-m.internalChan:
			if !ok {
				return
			}
			m.handleFsnotifyEvent(event)
		}
	}
}

// handleFsnotifyEvent 处理fsnotify事件
func (m *FileEventMonitor) handleFsnotifyEvent(event fsnotify.Event) {
	// 速率限制检查
	if !m.rateLimiter.Allow(event.Name) {
		return // 超过速率限制，丢弃事件
	}

	// 转换事件类型
	fileEvent := m.convertEvent(event)

	// 检查是否在排除路径中
	for _, exclude := range m.config.ExcludePaths {
		if strings.HasPrefix(fileEvent.Path, exclude) {
			return
		}
	}

	// 检查文件大小限制
	if fileEvent.Size > m.config.MaxFileSize && m.config.MaxFileSize > 0 {
		return // 文件过大，跳过
	}

	// 获取进程信息（如果可用）
	if m.processInfoGetter != nil {
		procInfo, err := m.processInfoGetter.GetProcessInfo(fileEvent.Path)
		if err == nil && procInfo != nil {
			fileEvent.ProcessPID = procInfo.PID
			fileEvent.ProcessName = procInfo.Name
		}
	}

	// 添加到缓冲队列
	m.buffer.Add(fileEvent)

	// 更新统计
	m.updateStats(fileEvent)

	// 发送到输出通道
	select {
	case m.eventChan <- fileEvent:
	default:
		// 通道满，丢弃事件
		m.statsMu.Lock()
		m.stats.BufferUsage = m.buffer.Size()
		m.statsMu.Unlock()
	}
}

// convertEvent 转换fsnotify事件为FileEvent
func (m *FileEventMonitor) convertEvent(event fsnotify.Event) FileEvent {
	now := time.Now()
	path := event.Name
	ext := filepath.Ext(path)

	// 获取文件大小
	var size int64
	info, err := os.Stat(path)
	if err == nil {
		size = info.Size()
	}

	// 确定操作类型
	operation := m.determineOperation(event)

	return FileEvent{
		ID:        uuid.New().String(),
		Timestamp: now,
		Path:      path,
		Operation: operation,
		Size:      size,
		Extension: ext,
		Metadata: map[string]interface{}{
			"fsnotify_op": event.Op.String(),
		},
	}
}

// determineOperation 确定文件操作类型
func (m *FileEventMonitor) determineOperation(event fsnotify.Event) FileOperation {
	op := event.Op

	// fsnotify操作优先级判断
	if op&fsnotify.Create != 0 {
		// 检查是否是新目录
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			// 新目录创建，需要添加监控
			go m.AddWatchPath(event.Name)
		}
		return FileOpCreate
	}

	if op&fsnotify.Write != 0 {
		return FileOpModify
	}

	if op&fsnotify.Remove != 0 {
		// 移除目录监控
		go m.RemoveWatchPath(event.Name)
		return FileOpDelete
	}

	if op&fsnotify.Rename != 0 {
		// Rename可能是重命名或移动
		// 如果文件还存在，是源文件重命名（移动）
		if _, err := os.Stat(event.Name); os.IsNotExist(err) {
			return FileOpDelete // 原路径文件已不存在
		}
		return FileOpRename
	}

	if op&fsnotify.Chmod != 0 {
		return FileOpModify // 权限变更视为修改
	}

	return FileOpModify // 默认
}

// errorLoop 错误处理循环
func (m *FileEventMonitor) errorLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.sendError(err)
		}
	}
}

// sendError 发送错误
func (m *FileEventMonitor) sendError(err error) {
	m.statsMu.Lock()
	m.stats.LastError = err.Error()
	m.stats.LastErrorTime = new(time.Time)
	*m.stats.LastErrorTime = time.Now()
	m.statsMu.Unlock()

	select {
	case m.errorChan <- err:
	default:
		// 错误通道满，丢弃
	}
}

// rateLimiterResetLoop 速率限制器重置循环
func (m *FileEventMonitor) rateLimiterResetLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.rateLimiter.Reset()
		}
	}
}

// updateStats 更新统计信息
func (m *FileEventMonitor) updateStats(event FileEvent) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	m.stats.TotalEvents++
	m.stats.EventsByType[event.Operation]++
	m.stats.EventsByPath[event.Path]++
	m.stats.BufferUsage = m.buffer.Size()
	m.stats.BufferCapacity = m.buffer.Capacity()
}

// GetStats 获取统计信息
func (m *FileEventMonitor) GetStats() MonitorStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	stats := m.stats
	return stats
}

// GetBufferedEvents 获取缓冲的事件
func (m *FileEventMonitor) GetBufferedEvents(limit int) []FileEvent {
	return m.buffer.GetRecent(limit)
}

// ClearBuffer 清除缓冲区
func (m *FileEventMonitor) ClearBuffer() {
	m.buffer.Clear()
}

// IsRunning 检查是否正在运行
func (m *FileEventMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetWatchedPaths 获取监控路径列表
func (m *FileEventMonitor) GetWatchedPaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paths := make([]string, 0, len(m.watchedPaths))
	for path := range m.watchedPaths {
		paths = append(paths, path)
	}
	return paths
}

// ========== EventBuffer ==========

// NewEventBuffer 创建事件缓冲队列
func NewEventBuffer(capacity int) *EventBuffer {
	return &EventBuffer{
		events:   make([]FileEvent, capacity),
		capacity: capacity,
		position: 0,
	}
}

// Add 添加事件到缓冲队列
func (b *EventBuffer) Add(event FileEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events[b.position] = event
	b.position = (b.position + 1) % b.capacity

	if b.position == 0 {
		b.overflow++
	}
}

// GetRecent 获取最近的事件
func (b *EventBuffer) GetRecent(limit int) []FileEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > b.capacity {
		limit = b.capacity
	}

	size := b.Size()
	if limit > size {
		limit = size
	}

	result := make([]FileEvent, limit)

	// 从当前位置向前读取
	start := b.position - limit
	if start < 0 {
		start += b.capacity
	}

	for i := 0; i < limit; i++ {
		idx := (start + i) % b.capacity
		result[i] = b.events[idx]
	}

	return result
}

// Size 获取缓冲区大小
func (b *EventBuffer) Size() int {
	return b.position
}

// Capacity 获取缓冲区容量
func (b *EventBuffer) Capacity() int {
	return b.capacity
}

// Clear 清除缓冲区
func (b *EventBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = make([]FileEvent, b.capacity)
	b.position = 0
	b.overflow = 0
}

// Overflow 获取溢出次数
func (b *EventBuffer) Overflow() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.overflow
}

// ========== RateLimiter ==========

// NewRateLimiter 创建速率限制器
func NewRateLimiter(maxEventsPerSecond int) *RateLimiter {
	return &RateLimiter{
		maxEventsPerSecond: maxEventsPerSecond,
		eventCounts:        make(map[string]int),
		lastReset:          time.Now(),
	}
}

// Allow 检查是否允许事件
func (r *RateLimiter) Allow(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否需要重置
	if time.Since(r.lastReset) >= time.Second {
		r.eventCounts = make(map[string]int)
		r.lastReset = time.Now()
	}

	count := r.eventCounts[path]
	if count >= r.maxEventsPerSecond {
		return false
	}

	r.eventCounts[path]++
	return true
}

// Reset 重置计数器
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventCounts = make(map[string]int)
	r.lastReset = time.Now()
}

// ========== LinuxProcessInfoGetter ==========

// LinuxProcessInfoGetter Linux进程信息获取器
type LinuxProcessInfoGetter struct{}

// NewLinuxProcessInfoGetter 创建Linux进程信息获取器
func NewLinuxProcessInfoGetter() *LinuxProcessInfoGetter {
	return &LinuxProcessInfoGetter{}
}

// GetProcessInfo 获取进程信息（通过/proc文件系统）
func (g *LinuxProcessInfoGetter) GetProcessInfo(filePath string) (*ProcessInfo, error) {
	// 注意：要获取修改文件的进程信息需要更复杂的方法
	// 这里提供简化实现，实际应用中可以使用auditd或eBPF

	// 返回基本信息
	return &ProcessInfo{
		PID:  0, // 无法直接获取
		Name: "",
		Path: "",
		User: "",
	}, nil
}

// ========== 配置加载 ==========

// DefaultMonitorConfig 返回默认监控配置
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		Enabled:             true,
		WatchPaths:          []string{"/data", "/mnt", "/shares", "/home"},
		ExcludePaths:        []string{"/proc", "/sys", "/dev", "/run", "/tmp", "/var/cache"},
		MaxFileSize:         100 * 1024 * 1024, // 100MB
		EntropyThreshold:    7.5,
		EncryptionThreshold: 10,
		BehaviorWindow:      5 * time.Minute,
		MaxEvents:           10000,
		AlertCooldown:       5 * time.Minute,
	}
}
