package search

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// Watcher 文件监控器
type Watcher struct {
	engine   *Engine
	watcher  *fsnotify.Watcher
	paths    []string
	logger   *zap.Logger
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	debounce time.Duration
	pending  map[string]time.Time
}

// WatcherConfig 监控器配置
type WatcherConfig struct {
	Paths    []string      // 监控路径
	Debounce time.Duration // 防抖时间
}

// NewWatcher 创建文件监控器
func NewWatcher(engine *Engine, config WatcherConfig, logger *zap.Logger) (*Watcher, error) {
	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if config.Debounce == 0 {
		config.Debounce = 500 * time.Millisecond
	}

	w := &Watcher{
		engine:   engine,
		watcher:  fswatcher,
		paths:    config.Paths,
		logger:   logger,
		debounce: config.Debounce,
		stopChan: make(chan struct{}),
		pending:  make(map[string]time.Time),
	}

	return w, nil
}

// Start 启动监控
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	// 添加监控路径
	for _, path := range w.paths {
		if err := w.addWatch(path); err != nil {
			w.logger.Warn("添加监控路径失败",
				zap.String("path", path),
				zap.Error(err))
		}
	}

	// 启动事件处理
	go w.processEvents()
	// 启动防抖处理
	go w.processPending()

	w.logger.Info("文件监控器启动",
		zap.Strings("paths", w.paths))

	return nil
}

// addWatch 添加监控路径
func (w *Watcher) addWatch(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// 添加目录监控
	if info.IsDir() {
		if err := w.watcher.Add(path); err != nil {
			return err
		}

		// 递归添加子目录
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(path, entry.Name())
				// 跳过隐藏目录和排除目录
				if strings.HasPrefix(entry.Name(), ".") || w.engine.excludeDirs[entry.Name()] {
					continue
				}
				_ = w.addWatch(subPath)
			}
		}
	}

	return nil
}

// processEvents 处理文件事件
func (w *Watcher) processEvents() {
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
			w.logger.Warn("监控器错误", zap.Error(err))
		}
	}
}

// handleEvent 处理单个事件
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// 跳过排除的文件
	if w.engine.shouldExclude(path) {
		return
	}

	// 添加到待处理队列（防抖）
	w.mu.Lock()
	w.pending[path] = time.Now()
	w.mu.Unlock()
}

// processPending 处理待更新队列
func (w *Watcher) processPending() {
	ticker := time.NewTicker(w.debounce)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.processBatch()
		}
	}
}

// processBatch 批量处理
func (w *Watcher) processBatch() {
	w.mu.Lock()
	pending := w.pending
	w.pending = make(map[string]time.Time)
	w.mu.Unlock()

	for path := range pending {
		// 检查文件是否存在
		info, err := os.Stat(path)
		if err != nil {
			// 文件已删除，从索引中移除
			_ = w.engine.Delete(path)
			continue
		}

		// 重新索引文件
		if !info.IsDir() {
			if err := w.engine.IndexFile(path); err != nil {
				w.logger.Debug("索引文件失败",
					zap.String("path", path),
					zap.Error(err))
			}
		}
	}
}

// AddPath 添加监控路径
func (w *Watcher) AddPath(path string) error {
	w.mu.Lock()
	w.paths = append(w.paths, path)
	w.mu.Unlock()

	return w.addWatch(path)
}

// RemovePath 移除监控路径
func (w *Watcher) RemovePath(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 从路径列表中移除
	for i, p := range w.paths {
		if p == path {
			w.paths = append(w.paths[:i], w.paths[i+1:]...)
			break
		}
	}

	return w.watcher.Remove(path)
}

// Stop 停止监控
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopChan)
	w.watcher.Close()

	w.logger.Info("文件监控器停止")
}
