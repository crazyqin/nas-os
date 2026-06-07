// Package cloudsync provides cloud storage synchronization
// This file implements real-time file watching and sync
package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// RealtimeSync 实时同步服务
// 监控本地文件变化并自动触发同步.
type RealtimeSync struct {
	mu sync.RWMutex

	// 文件监控器
	watcher   *fsnotify.Watcher
	watchDirs map[string]string // 本地路径 -> 任务ID

	// 同步管理器
	manager *Manager

	// 任务队列
	taskQueue chan string

	// 去重缓冲（避免短时间内多次触发）
	pendingChanges map[string]time.Time
	debounceDelay  time.Duration

	// 状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRealtimeSync 创建实时同步服务.
func NewRealtimeSync(manager *Manager) *RealtimeSync {
	return &RealtimeSync{
		manager:        manager,
		watchDirs:      make(map[string]string),
		taskQueue:      make(chan string, 100),
		pendingChanges: make(map[string]time.Time),
		debounceDelay:  2 * time.Second, // 2秒去重延迟
	}
}

// Start 启动实时同步服务.
func (r *RealtimeSync) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("实时同步服务已运行")
	}

	// 创建文件监控器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监控器失败: %w", err)
	}

	r.watcher = watcher
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.running = true

	// 启动事件处理循环
	go r.processEvents()

	// 启动任务执行循环
	go r.executeTasks()

	// 启动去重清理循环
	go r.cleanupPendingChanges()

	return nil
}

// Stop 停止实时同步服务.
func (r *RealtimeSync) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	r.running = false

	if r.cancel != nil {
		r.cancel()
	}

	if r.watcher != nil {
		_ = r.watcher.Close()
	}

	return nil
}

// AddWatch 添加监控目录.
func (r *RealtimeSync) AddWatch(localPath, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 确保目录存在
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		if err := os.MkdirAll(localPath, 0750); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	// 添加监控
	if r.watcher != nil {
		if err := r.watcher.Add(localPath); err != nil {
			return fmt.Errorf("添加监控失败: %w", err)
		}

		// 递归监控子目录
		err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return r.watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("递归监控失败: %w", err)
		}
	}

	r.watchDirs[localPath] = taskID

	return nil
}

// RemoveWatch 移除监控目录.
func (r *RealtimeSync) RemoveWatch(localPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.watcher != nil {
		_ = r.watcher.Remove(localPath)

		// 移除子目录监控
		for watchPath := range r.watchDirs {
			if strings.HasPrefix(watchPath, localPath) {
				_ = r.watcher.Remove(watchPath)
			}
		}
	}

	delete(r.watchDirs, localPath)

	return nil
}

// ListWatches 列出所有监控目录.
func (r *RealtimeSync) ListWatches() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range r.watchDirs {
		result[k] = v
	}
	return result
}

// processEvents 处理文件变化事件.
func (r *RealtimeSync) processEvents() {
	for {
		select {
		case <-r.ctx.Done():
			return

		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}

			r.handleEvent(event)

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			// 记录错误但继续运行
			fmt.Printf("实时同步监控错误: %v\n", err)
		}
	}
}

// handleEvent 处理单个文件事件.
func (r *RealtimeSync) handleEvent(event fsnotify.Event) {
	// 过滤不需要的事件
	if event.Op == fsnotify.Chmod {
		return // 忽略权限变化
	}

	// 查找对应的任务ID
	r.mu.RLock()
	var taskID string
	for localPath, tid := range r.watchDirs {
		if strings.HasPrefix(event.Name, localPath) {
			taskID = tid
			break
		}
	}
	r.mu.RUnlock()

	if taskID == "" {
		return // 没有找到对应的任务
	}

	// 添加到待处理队列（去重）
	r.mu.Lock()
	r.pendingChanges[event.Name] = time.Now()
	r.mu.Unlock()

	// 如果是新目录，添加监控
	if event.Op == fsnotify.Create {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			r.mu.Lock()
			if r.watcher != nil {
				_ = r.watcher.Add(event.Name)
			}
			r.mu.Unlock()
		}
	}

	// 触发任务执行
	select {
	case r.taskQueue <- taskID:
	default:
		// 队列满了，忽略
	}
}

// executeTasks 执行同步任务.
func (r *RealtimeSync) executeTasks() {
	// 执行去重的任务
	var pendingTasks map[string]bool
	var lastExecute time.Time

	for {
		select {
		case <-r.ctx.Done():
			return

		case taskID := <-r.taskQueue:
			// 收集待执行任务
			if pendingTasks == nil {
				pendingTasks = make(map[string]bool)
			}
			pendingTasks[taskID] = true

			// 等待去重延迟
			if time.Since(lastExecute) < r.debounceDelay {
				continue
			}

		case <-time.After(r.debounceDelay):
			// 延迟后执行收集的任务
		}

		if len(pendingTasks) == 0 {
			continue
		}

		// 执行任务
		for taskID := range pendingTasks {
			_, _ = r.manager.RunSyncTask(taskID)
		}

		pendingTasks = nil
		lastExecute = time.Now()
	}
}

// cleanupPendingChanges 清理过期的待处理变化.
func (r *RealtimeSync) cleanupPendingChanges() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for path, t := range r.pendingChanges {
				if now.Sub(t) > 60*time.Second {
					delete(r.pendingChanges, path)
				}
			}
			r.mu.Unlock()
		}
	}
}

// SyncStatus 实时同步状态.
type RealtimeSyncStatus struct {
	Running        bool              `json:"running"`
	WatchCount     int               `json:"watchCount"`
	WatchDirs      map[string]string `json:"watchDirs"`
	PendingChanges int               `json:"pendingChanges"`
	QueueLength    int               `json:"queueLength"`
}

// GetStatus 获取实时同步状态.
func (r *RealtimeSync) GetStatus() *RealtimeSyncStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &RealtimeSyncStatus{
		Running:        r.running,
		WatchCount:     len(r.watchDirs),
		WatchDirs:      r.watchDirs,
		PendingChanges: len(r.pendingChanges),
		QueueLength:    len(r.taskQueue),
	}
}
