package replication

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher 文件监控器
type Watcher struct {
	mu            sync.RWMutex
	watcher       *fsnotify.Watcher
	tasks         map[string]*ReplicationTask // 任务ID -> 任务
	pathToTask    map[string]string           // 监控路径 -> 任务ID
	conflictDet   *ConflictDetector
	eventChan     chan FileEvent
	stopChan      chan struct{}
	wg            sync.WaitGroup
	debounceDelay time.Duration
	batchedEvents map[string]time.Time // 路径 -> 最后事件时间
}

// FileEvent 文件事件
type FileEvent struct {
	TaskID     string    `json:"task_id"`
	Path       string    `json:"path"`
	Operation  string    `json:"operation"` // create, write, remove, rename, chmod
	Timestamp  time.Time `json:"timestamp"`
	IsDir      bool      `json:"is_dir"`
}

// NewWatcher 创建文件监控器
func NewWatcher(conflictDetector *ConflictDetector) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建 fsnotify watcher 失败：%w", err)
	}

	return &Watcher{
		watcher:       fsWatcher,
		tasks:         make(map[string]*ReplicationTask),
		pathToTask:    make(map[string]string),
		conflictDet:   conflictDetector,
		eventChan:     make(chan FileEvent, 1000),
		stopChan:      make(chan struct{}),
		debounceDelay: 2 * time.Second,
		batchedEvents: make(map[string]time.Time),
	}, nil
}

// AddTask 添加监控任务
func (w *Watcher) AddTask(task *ReplicationTask) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if task.Type != TypeRealtime && task.Type != TypeBidirectional {
		return fmt.Errorf("任务类型不支持实时监控")
	}

	// 检查源路径是否存在
	if _, err := os.Stat(task.SourcePath); os.IsNotExist(err) {
		return fmt.Errorf("源路径不存在：%s", task.SourcePath)
	}

	// 添加监控
	if err := w.watcher.Add(task.SourcePath); err != nil {
		return fmt.Errorf("添加监控失败：%w", err)
	}

	// 递归添加子目录
	if err := w.addSubdirectories(task.SourcePath); err != nil {
		w.watcher.Remove(task.SourcePath)
		return fmt.Errorf("添加子目录监控失败：%w", err)
	}

	w.tasks[task.ID] = task
	w.pathToTask[task.SourcePath] = task.ID

	return nil
}

// RemoveTask 移除监控任务
func (w *Watcher) RemoveTask(taskID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	task, exists := w.tasks[taskID]
	if !exists {
		return nil
	}

	// 移除监控
	w.watcher.Remove(task.SourcePath)
	delete(w.tasks, taskID)
	delete(w.pathToTask, task.SourcePath)

	return nil
}

// addSubdirectories 递归添加子目录监控
func (w *Watcher) addSubdirectories(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				// 记录警告但继续
				fmt.Printf("警告：无法监控目录 %s：%v\n", path, err)
			}
		}
		return nil
	})
}

// Start 启动监控
func (w *Watcher) Start() {
	w.wg.Add(1)
	go w.run()
}

// Stop 停止监控
func (w *Watcher) Stop() {
	close(w.stopChan)
	w.watcher.Close()
	w.wg.Wait()
}

// run 运行监控循环
func (w *Watcher) run() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.debounceDelay)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("监控错误：%v\n", err)

		case <-ticker.C:
			w.processBatchedEvents()
		}
	}
}

// handleEvent 处理文件事件
func (w *Watcher) handleEvent(event fsnotify.Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// 查找对应的任务
	var matchedTask *ReplicationTask
	var relativePath string

	for _, task := range w.tasks {
		if strings.HasPrefix(event.Name, task.SourcePath) {
			matchedTask = task
			relativePath = strings.TrimPrefix(event.Name, task.SourcePath)
			relativePath = strings.TrimPrefix(relativePath, "/")
			break
		}
	}

	if matchedTask == nil {
		return
	}

	// 如果任务暂停或禁用，忽略事件
	if matchedTask.Status == StatusPaused || !matchedTask.Enabled {
		return
	}

	// 处理新目录创建
	if event.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.watcher.Add(event.Name)
		}
	}

	// 防抖：记录事件
	w.batchedEvents[event.Name] = time.Now()
}

// processBatchedEvents 处理批量事件
func (w *Watcher) processBatchedEvents() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	for path, eventTime := range w.batchedEvents {
		if now.Sub(eventTime) >= w.debounceDelay {
			// 查找任务并生成同步事件
			for _, task := range w.tasks {
				if strings.HasPrefix(path, task.SourcePath) {
					relativePath := strings.TrimPrefix(path, task.SourcePath)
					relativePath = strings.TrimPrefix(relativePath, "/")

					// 检查冲突
					conflict, err := w.conflictDet.DetectConflict(task, relativePath)
					if err != nil {
						fmt.Printf("冲突检测失败：%v\n", err)
					}
					if conflict != nil {
						w.conflictDet.AddConflict(conflict)
						if err := w.conflictDet.ResolveConflict(conflict); err != nil {
							fmt.Printf("冲突解决失败：%v\n", err)
						}
						continue
					}

					// 发送同步事件
					info, _ := os.Stat(path)
					select {
					case w.eventChan <- FileEvent{
						TaskID:    task.ID,
						Path:      path,
						Timestamp: time.Now(),
						IsDir:     info != nil && info.IsDir(),
					}:
					default:
						// 通道满，丢弃事件
					}

					break
				}
			}
			delete(w.batchedEvents, path)
		}
	}
}

// Events 获取事件通道
func (w *Watcher) Events() <-chan FileEvent {
	return w.eventChan
}

// WatcherStats 监控统计
type WatcherStats struct {
	TasksWatched  int `json:"tasks_watched"`
	EventsPending int `json:"events_pending"`
}

// GetStats 获取监控统计
func (w *Watcher) GetStats() WatcherStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WatcherStats{
		TasksWatched:  len(w.tasks),
		EventsPending: len(w.batchedEvents),
	}
}

// BidirectionalSyncManager 双向同步管理器
type BidirectionalSyncManager struct {
	mu           sync.RWMutex
	tasks        map[string]*ReplicationTask
	watcher      *Watcher
	conflictDet  *ConflictDetector
	syncChan     chan string // 任务ID 通道
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewBidirectionalSyncManager 创建双向同步管理器
func NewBidirectionalSyncManager(watcher *Watcher, conflictDetector *ConflictDetector) *BidirectionalSyncManager {
	return &BidirectionalSyncManager{
		tasks:       make(map[string]*ReplicationTask),
		watcher:     watcher,
		conflictDet: conflictDetector,
		syncChan:    make(chan string, 100),
		stopChan:    make(chan struct{}),
	}
}

// AddTask 添加双向同步任务
func (m *BidirectionalSyncManager) AddTask(task *ReplicationTask) error {
	if task.Type != TypeBidirectional {
		return fmt.Errorf("任务类型不是双向同步")
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 添加到监控器（源端）
	if err := m.watcher.AddTask(task); err != nil {
		return err
	}

	return nil
}

// RemoveTask 移除双向同步任务
func (m *BidirectionalSyncManager) RemoveTask(taskID string) {
	m.mu.Lock()
	delete(m.tasks, taskID)
	m.mu.Unlock()

	m.watcher.RemoveTask(taskID)
}

// Start 启动双向同步
func (m *BidirectionalSyncManager) Start() {
	m.wg.Add(1)
	go m.run()
}

// Stop 停止双向同步
func (m *BidirectionalSyncManager) Stop() {
	close(m.stopChan)
	m.wg.Wait()
}

// run 运行双向同步循环
func (m *BidirectionalSyncManager) run() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return

		case event := <-m.watcher.Events():
			m.processEvent(event)

		case taskID := <-m.syncChan:
			m.syncTask(taskID)
		}
	}
}

// processEvent 处理文件事件
func (m *BidirectionalSyncManager) processEvent(event FileEvent) {
	m.mu.RLock()
	task, exists := m.tasks[event.TaskID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// 双向同步：同时更新目标端
	fmt.Printf("双向同步事件：%s -> %s\n", event.Path, task.TargetPath)
	// 实际同步逻辑由 Manager 执行
}

// syncTask 同步任务
func (m *BidirectionalSyncManager) syncTask(taskID string) {
	m.mu.RLock()
	task, exists := m.tasks[taskID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	fmt.Printf("执行双向同步：%s <-> %s\n", task.SourcePath, task.TargetPath)
	// 实际同步逻辑由 Manager 执行
}