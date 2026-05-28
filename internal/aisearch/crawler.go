// Package aisearch 提供文件系统爬虫功能
package aisearch

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Crawler 文件系统爬虫
type Crawler struct {
	mu       sync.RWMutex
	roots    []string
	excludes []string
	callback func(*FileInfo) error
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	stats    *CrawlerStats
}

// CrawlerStats 爬虫统计
type CrawlerStats struct {
	TotalDirs   int64     `json:"totalDirs"`
	TotalFiles  int64     `json:"totalFiles"`
	TotalSize   int64     `json:"totalSize"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Duration    time.Duration `json:"duration"`
	Errors      int64     `json:"errors"`
	LastError   string    `json:"lastError,omitempty"`
}

// NewCrawler 创建爬虫
func NewCrawler(roots []string, excludes []string) *Crawler {
	return &Crawler{
		roots:    roots,
		excludes: excludes,
		stopCh:   make(chan struct{}),
		stats:    &CrawlerStats{},
	}
}

// Crawl 遍历目录
func (c *Crawler) Crawl(rootPath string, callback func(*FileInfo) error) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("爬虫正在运行中")
	}
	c.running = true
	c.callback = callback
	c.stats = &CrawlerStats{
		StartTime: time.Now(),
	}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.running = false
		c.stats.EndTime = time.Now()
		c.stats.Duration = c.stats.EndTime.Sub(c.stats.StartTime)
		c.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听停止信号
	go func() {
		select {
		case <-c.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	return c.walkDirectory(ctx, rootPath)
}

// CrawlAll 遍历所有根目录
func (c *Crawler) CrawlAll(callback func(*FileInfo) error) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("爬虫正在运行中")
	}
	c.running = true
	c.callback = callback
	c.stats = &CrawlerStats{
		StartTime: time.Now(),
	}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.running = false
		c.stats.EndTime = time.Now()
		c.stats.Duration = c.stats.EndTime.Sub(c.stats.StartTime)
		c.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听停止信号
	go func() {
		select {
		case <-c.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	for _, root := range c.roots {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.walkDirectory(ctx, root); err != nil {
			log.Printf("遍历目录 %s 失败: %v", root, err)
			c.mu.Lock()
			c.stats.Errors++
			c.stats.LastError = err.Error()
			c.mu.Unlock()
		}
	}

	return nil
}

// CrawlAsync 异步遍历目录
func (c *Crawler) CrawlAsync(callback func(*FileInfo) error) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.CrawlAll(callback); err != nil {
			log.Printf("爬虫错误: %v", err)
		}
	}()
}

// Stop 停止爬虫
func (c *Crawler) Stop() error {
	close(c.stopCh)
	c.wg.Wait()
	return nil
}

// GetStats 获取爬虫统计
func (c *Crawler) GetStats() *CrawlerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	stats := *c.stats
	return &stats
}

// IsRunning 是否正在运行
func (c *Crawler) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// walkDirectory 递归遍历目录
func (c *Crawler) walkDirectory(ctx context.Context, dirPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 检查是否排除
	if c.isExcluded(dirPath) {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("读取目录 %s 失败: %w", dirPath, err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fullPath := filepath.Join(dirPath, entry.Name())

		// 检查是否排除
		if c.isExcluded(fullPath) {
			continue
		}

		if entry.IsDir() {
			c.mu.Lock()
			c.stats.TotalDirs++
			c.mu.Unlock()

			if err := c.walkDirectory(ctx, fullPath); err != nil {
				log.Printf("遍历子目录 %s 失败: %v", fullPath, err)
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				log.Printf("获取文件信息 %s 失败: %v", fullPath, err)
				continue
			}

			fileInfo := &FileInfo{
				Path:       fullPath,
				Name:       entry.Name(),
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
				CreatedAt:  info.ModTime(),
				IsDir:      false,
				Extension:  strings.ToLower(filepath.Ext(entry.Name())),
				MimeType:   getMimeType(strings.ToLower(filepath.Ext(entry.Name()))),
			}

			c.mu.Lock()
			c.stats.TotalFiles++
			c.stats.TotalSize += info.Size()
			c.mu.Unlock()

			if c.callback != nil {
				if err := c.callback(fileInfo); err != nil {
					c.mu.Lock()
					c.stats.Errors++
					c.stats.LastError = err.Error()
					c.mu.Unlock()
					log.Printf("处理文件 %s 回调失败: %v", fullPath, err)
				}
			}
		}
	}

	return nil
}

// isExcluded 检查是否排除
func (c *Crawler) isExcluded(path string) bool {
	for _, exclude := range c.excludes {
		if strings.HasPrefix(path, exclude) {
			return true
		}
		// 检查目录名
		dir := filepath.Dir(path)
		if strings.Contains(dir, exclude) {
			return true
		}
		// 检查文件名
		name := filepath.Base(path)
		if name == exclude {
			return true
		}
	}

	// 排除隐藏文件和目录
	name := filepath.Base(path)
	if strings.HasPrefix(name, ".") {
		return true
	}

	return false
}

// FileWatcher 文件监听器
type FileWatcher struct {
	mu       sync.RWMutex
	paths    []string
	callback func(*FileEvent) error
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	interval time.Duration
	snapshot map[string]time.Time
}

// NewFileWatcher 创建文件监听器
func NewFileWatcher(paths []string, interval time.Duration) *FileWatcher {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &FileWatcher{
		paths:    paths,
		stopCh:   make(chan struct{}),
		interval: interval,
		snapshot: make(map[string]time.Time),
	}
}

// Watch 监听文件变化
func (fw *FileWatcher) Watch(callback func(*FileEvent) error) error {
	fw.mu.Lock()
	if fw.running {
		fw.mu.Unlock()
		return fmt.Errorf("监听器正在运行中")
	}
	fw.running = true
	fw.callback = callback
	fw.mu.Unlock()

	// 初始化快照
	fw.initSnapshot()

	fw.wg.Add(1)
	go fw.watchLoop()

	return nil
}

// Stop 停止监听
func (fw *FileWatcher) Stop() error {
	close(fw.stopCh)
	fw.wg.Wait()
	return nil
}

// IsRunning 是否正在运行
func (fw *FileWatcher) IsRunning() bool {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	return fw.running
}

// initSnapshot 初始化快照
func (fw *FileWatcher) initSnapshot() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	for _, path := range fw.paths {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				fw.snapshot[p] = info.ModTime()
			}
			return nil
		})
	}
}

// watchLoop 监听循环
func (fw *FileWatcher) watchLoop() {
	defer fw.wg.Done()

	ticker := time.NewTicker(fw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopCh:
			fw.mu.Lock()
			fw.running = false
			fw.mu.Unlock()
			return
		case <-ticker.C:
			fw.detectChanges()
		}
	}
}

// detectChanges 检测变化
func (fw *FileWatcher) detectChanges() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	currentSnapshot := make(map[string]time.Time)

	for _, path := range fw.paths {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				currentSnapshot[p] = info.ModTime()
			}
			return nil
		})
	}

	// 检测新增和修改
	for path, modTime := range currentSnapshot {
		oldModTime, exists := fw.snapshot[path]
		if !exists {
			// 新增文件
			if fw.callback != nil {
				fw.callback(&FileEvent{
					Type:     "create",
					FilePath: path,
					FileInfo: &FileInfo{
						Path:       path,
						Name:       filepath.Base(path),
						ModifiedAt: modTime,
					},
				})
			}
		} else if modTime.After(oldModTime) {
			// 修改文件
			if fw.callback != nil {
				fw.callback(&FileEvent{
					Type:     "modify",
					FilePath: path,
					FileInfo: &FileInfo{
						Path:       path,
						Name:       filepath.Base(path),
						ModifiedAt: modTime,
					},
				})
			}
		}
	}

	// 检测删除
	for path := range fw.snapshot {
		if _, exists := currentSnapshot[path]; !exists {
			if fw.callback != nil {
				fw.callback(&FileEvent{
					Type:     "delete",
					FilePath: path,
				})
			}
		}
	}

	// 更新快照
	fw.snapshot = currentSnapshot
}
