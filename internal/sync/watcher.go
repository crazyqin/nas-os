package sync

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

// Watcher 增量同步状态追踪器.
// 支持三种模式：inotify（实时监控）、scan（定期全量扫描）、hybrid（两者结合）.
type Watcher struct {
	mu       sync.RWMutex
	taskID   string
	task     *Task
	provider Provider
	store    *StateStore

	// inotify 模式
	fsWatcher *fsnotify.Watcher
	watchDirs map[string]struct{} // 已监控的目录

	// 事件处理
	events      []FileWatchEvent
	eventDebounce map[string]time.Time
	debounceInterval time.Duration

	// 扫描模式
	scanInterval time.Duration
	scanStop     chan struct{}

	// 混合模式：用 inotify 触发增量扫描
	useHybrid bool

	// 回调
	onChanges func(delta *Delta)

	ctx    context.Context
	cancel context.CancelFunc
}

// FileWatchEvent 文件监控事件.
type FileWatchEvent struct {
	Path      string    `json:"path"`
	Op        fsnotify.Op `json:"op"`
	Timestamp time.Time `json:"timestamp"`
	IsDir     bool      `json:"isDir"`
}

// NewWatcher 创建 watcher.
func NewWatcher(taskID string, task *Task, provider Provider, store *StateStore) (*Watcher, error) {
	w := &Watcher{
		taskID:          taskID,
		task:            task,
		provider:        provider,
		store:           store,
		watchDirs:       make(map[string]struct{}),
		eventDebounce:   make(map[string]time.Time),
		debounceInterval: 2 * time.Second,
		scanInterval:    5 * time.Minute,
	}

	switch task.WatchMode {
	case "inotify":
		if err := w.initInotify(); err != nil {
			return nil, err
		}
	case "scan":
		w.scanStop = make(chan struct{})
	case "hybrid":
		if err := w.initInotify(); err != nil {
			return nil, err
		}
		w.scanStop = make(chan struct{})
		w.useHybrid = true
	default:
		// 默认 hybrid
		if err := w.initInotify(); err != nil {
			return nil, err
		}
		w.scanStop = make(chan struct{})
		w.useHybrid = true
	}

	return w, nil
}

// initInotify 初始化 inotify 监控.
func (w *Watcher) initInotify() error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}
	w.fsWatcher = fsWatcher
	return nil
}

// Start 启动 watcher.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.mu.Unlock()

	if w.fsWatcher != nil {
		// 添加初始监控目录
		if err := w.addWatchRecursive(w.task.LocalPath); err != nil {
			return err
		}
		go w.runInotifyLoop()
	}

	if w.scanStop != nil {
		go w.runScanLoop()
	}

	return nil
}

// Stop 停止 watcher.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}
	if w.fsWatcher != nil {
		_ = w.fsWatcher.Close()
	}
	if w.scanStop != nil {
		close(w.scanStop)
	}
	return nil
}

// SetOnChanges 设置变化回调.
func (w *Watcher) SetOnChanges(fn func(delta *Delta)) {
	w.mu.Lock()
	w.onChanges = fn
	w.mu.Unlock()
}

// addWatchRecursive 递归添加监控.
func (w *Watcher) addWatchRecursive(dir string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_ = w.fsWatcher.Add(dir)
	w.watchDirs[dir] = struct{}{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			subDir := filepath.Join(dir, e.Name())
			if err := w.addWatchRecursive(subDir); err != nil {
				// 继续，不阻塞
			}
		}
	}
	return nil
}

// runInotifyLoop 处理 inotify 事件循环.
func (w *Watcher) runInotifyLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleInotifyEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// 记录错误但不中断
			_ = err
		}
	}
}

func (w *Watcher) handleInotifyEvent(event fsnotify.Event) {
	// 过滤 chmod
	if event.Op == fsnotify.Chmod {
		return
	}

	// 忽略非任务路径
	if !strings.HasPrefix(event.Name, w.task.LocalPath) {
		return
	}

	// 新建目录时添加监控
	if event.Op == fsnotify.Create {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			w.mu.Lock()
			_ = w.fsWatcher.Add(event.Name)
			w.watchDirs[event.Name] = struct{}{}
			w.mu.Unlock()
		}
	}

	// 去重：短时间内同一路径忽略多次事件
	w.mu.Lock()
	last := w.eventDebounce[event.Name]
	now := time.Now()
	if !last.IsZero() && now.Sub(last) < w.debounceInterval {
		w.mu.Unlock()
		return
	}
	w.eventDebounce[event.Name] = now
	w.mu.Unlock()

	w.mu.RLock()
	onChanges := w.onChanges
	w.mu.RUnlock()

	if onChanges != nil {
		onChanges(nil) // 触发重新扫描对比（delta 由 engine 层计算）
	}
}

// runScanLoop 定期全量扫描循环.
func (w *Watcher) runScanLoop() {
	ticker := time.NewTicker(w.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.scanStop:
			return
		case <-ticker.C:
			w.triggerScan()
		}
	}
}

// TriggerScan 手动触发一次扫描.
func (w *Watcher) TriggerScan() {
	w.triggerScan()
}

func (w *Watcher) triggerScan() {
	w.mu.RLock()
	onChanges := w.onChanges
	w.mu.RUnlock()

	if onChanges != nil {
		onChanges(nil)
	}
}

// WatchStatus watcher 状态.
type WatchStatus struct {
	TaskID      string            `json:"taskID"`
	Mode        string            `json:"mode"`
	Running     bool              `json:"running"`
	WatchDirs   int               `json:"watchDirs"`
	Debounced   int               `json:"debouncedEvents"`
	LastScan    *time.Time        `json:"lastScan,omitempty"`
	PendingSync bool              `json:"pendingSync"`
}

// Status 返回 watcher 状态.
func (w *Watcher) Status() WatchStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WatchStatus{
		TaskID:    w.taskID,
		Mode:      w.task.WatchMode,
		Running:   w.ctx != nil && w.ctx.Err() == nil,
		WatchDirs: len(w.watchDirs),
		Debounced: len(w.eventDebounce),
	}
}

// ScanLocalFiles 扫描本地目录，返回当前快照.
// 供外部 engine 调用以获取当前状态.
func (w *Watcher) ScanLocalFiles() (*Snapshot, error) {
	scanner := NewSnapshotScanner()
	scanner.SetExcludePatterns(w.task.ExcludePatterns)
	scanner.SetMaxSize(w.task.MaxFileSize)
	if !w.task.ChecksumVerify {
		scanner.SetChecksum(false)
	}

	return scanner.Scan(w.task.LocalPath, 0)
}
