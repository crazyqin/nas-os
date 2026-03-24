// Package cloudfuse provides cloud storage mounting via FUSE
// 本地缓存管理 - 提高性能，减少网络请求
package cloudfuse

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CacheManager 缓存管理器
type CacheManager struct {
	mu          sync.RWMutex
	cacheDir    string
	maxSize     int64                    // 最大缓存大小（字节）
	usedSize    int64                    // 已用缓存大小
	entries     map[string]*list.Element // path -> list element
	lruList     *list.List
	entries2    map[string]*CacheEntry // path -> entry（用于快速查找）
	ctx         context.Context
	cancel      context.CancelFunc
	cleanTicker *time.Ticker
	hits        int64
	misses      int64
	evictions   int64
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(cacheDir string, maxSize int64) (*CacheManager, error) {
	// maxSize 单位是MB，转换为字节
	maxBytes := maxSize * 1024 * 1024

	ctx, cancel := context.WithCancel(context.Background())

	cm := &CacheManager{
		cacheDir: cacheDir,
		maxSize:  maxBytes,
		entries:  make(map[string]*list.Element),
		lruList:  list.New(),
		entries2: make(map[string]*CacheEntry),
		ctx:      ctx,
		cancel:   cancel,
	}

	// 确保缓存目录存在
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 扫描现有缓存
	if err := cm.scanExistingCache(); err != nil {
		// 忽略错误，继续运行
		fmt.Printf("扫描现有缓存失败: %v\n", err)
	}

	// 启动定期清理
	cm.cleanTicker = time.NewTicker(5 * time.Minute)
	go cm.cleanupLoop()

	return cm, nil
}

// Get 获取缓存
func (cm *CacheManager) Get(path string) (string, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entry, ok := cm.entries2[path]
	if !ok {
		cm.misses++
		return "", false
	}

	// 检查文件是否存在
	if _, err := os.Stat(entry.LocalPath); os.IsNotExist(err) {
		delete(cm.entries2, path)
		if elem, ok := cm.entries[path]; ok {
			cm.lruList.Remove(elem)
			delete(cm.entries, path)
		}
		cm.misses++
		return "", false
	}

	// 更新LRU
	if elem, ok := cm.entries[path]; ok {
		cm.lruList.MoveToFront(elem)
	}

	entry.AccessTime = time.Now()
	entry.HitCount++
	cm.hits++

	return entry.LocalPath, true
}

// Put 添加缓存
func (cm *CacheManager) Put(path, localPath string, size int64) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查是否已存在
	if entry, ok := cm.entries2[path]; ok {
		// 更新现有条目
		cm.usedSize -= entry.Size
		entry.LocalPath = localPath
		entry.Size = size
		entry.ModTime = time.Now()
		entry.AccessTime = time.Now()
		cm.usedSize += size

		// 更新LRU
		if elem, ok := cm.entries[path]; ok {
			cm.lruList.MoveToFront(elem)
		}
		return nil
	}

	// 检查是否需要清理空间
	for cm.usedSize+size > cm.maxSize {
		cm.evictOne()
	}

	// 创建新条目
	entry := &CacheEntry{
		Path:       path,
		LocalPath:  localPath,
		Size:       size,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		HitCount:   0,
	}

	elem := cm.lruList.PushFront(path)
	cm.entries[path] = elem
	cm.entries2[path] = entry
	cm.usedSize += size

	return nil
}

// Remove 删除缓存
func (cm *CacheManager) Remove(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entry, ok := cm.entries2[path]
	if !ok {
		return
	}

	// 删除文件（忽略错误，文件可能不存在）
	_ = os.Remove(entry.LocalPath)

	// 更新统计
	cm.usedSize -= entry.Size
	delete(cm.entries2, path)

	if elem, ok := cm.entries[path]; ok {
		cm.lruList.Remove(elem)
		delete(cm.entries, path)
	}

	cm.evictions++
}

// GetCachePath 获取缓存路径
func (cm *CacheManager) GetCachePath(remotePath string) string {
	// 使用远程路径的哈希作为文件名，避免路径冲突
	// 但保持目录结构便于调试
	safePath := filepath.Join(cm.cacheDir, remotePath)
	return safePath
}

// Clear 清空缓存
func (cm *CacheManager) Clear() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 删除所有缓存文件
	for path, entry := range cm.entries2 {
		_ = os.Remove(entry.LocalPath)
		delete(cm.entries2, path)
		if elem, ok := cm.entries[path]; ok {
			cm.lruList.Remove(elem)
			delete(cm.entries, path)
		}
	}

	cm.usedSize = 0
	cm.evictions += int64(len(cm.entries2))
	cm.entries2 = make(map[string]*CacheEntry)
	cm.entries = make(map[string]*list.Element)
	cm.lruList = list.New()

	return nil
}

// Stats 返回缓存统计
func (cm *CacheManager) Stats() (hits, misses, evictions int64, usedSize, maxSize int64) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.hits, cm.misses, cm.evictions, cm.usedSize, cm.maxSize
}

// HitRate 返回缓存命中率
func (cm *CacheManager) HitRate() float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	total := cm.hits + cm.misses
	if total == 0 {
		return 0
	}
	return float64(cm.hits) / float64(total)
}

// UsedSize 返回已用缓存大小
func (cm *CacheManager) UsedSize() int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.usedSize
}

// Close 关闭缓存管理器
func (cm *CacheManager) Close() error {
	cm.cancel()
	if cm.cleanTicker != nil {
		cm.cleanTicker.Stop()
	}
	return nil
}

// evictOne 淘汰一个缓存条目
func (cm *CacheManager) evictOne() {
	if cm.lruList.Len() == 0 {
		return
	}

	// 从LRU列表尾部获取最久未使用的条目
	elem := cm.lruList.Back()
	if elem == nil {
		return
	}

	path, ok := elem.Value.(string)
	if !ok {
		cm.lruList.Remove(elem)
		delete(cm.entries, path)
		return
	}

	entry, ok := cm.entries2[path]
	if !ok {
		cm.lruList.Remove(elem)
		delete(cm.entries, path)
		return
	}

	// 删除文件（忽略错误）
	_ = os.Remove(entry.LocalPath)

	// 更新统计
	cm.usedSize -= entry.Size
	delete(cm.entries2, path)
	cm.lruList.Remove(elem)
	delete(cm.entries, path)
	cm.evictions++
}

// cleanupLoop 定期清理过期缓存
func (cm *CacheManager) cleanupLoop() {
	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-cm.cleanTicker.C:
			cm.cleanup()
		}
	}
}

// cleanup 清理过期或无效的缓存
func (cm *CacheManager) cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 清理不存在的文件
	var toRemove []string
	for path, entry := range cm.entries2 {
		if _, err := os.Stat(entry.LocalPath); os.IsNotExist(err) {
			toRemove = append(toRemove, path)
		}
	}

	for _, path := range toRemove {
		entry := cm.entries2[path]
		cm.usedSize -= entry.Size
		delete(cm.entries2, path)
		if elem, ok := cm.entries[path]; ok {
			cm.lruList.Remove(elem)
			delete(cm.entries, path)
		}
	}

	// 如果使用量超过90%，强制清理
	if cm.usedSize > cm.maxSize*9/10 {
		for cm.usedSize > cm.maxSize*8/10 && cm.lruList.Len() > 0 {
			cm.evictOne()
		}
	}
}

// scanExistingCache 扫描现有缓存
func (cm *CacheManager) scanExistingCache() error {
	return filepath.Walk(cm.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误
		}

		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(cm.cacheDir, path)
		if err != nil {
			return nil
		}

		// 创建缓存条目
		entry := &CacheEntry{
			Path:       "/" + filepath.ToSlash(relPath),
			LocalPath:  path,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			AccessTime: info.ModTime(),
		}

		elem := cm.lruList.PushFront(entry.Path)
		cm.entries[entry.Path] = elem
		cm.entries2[entry.Path] = entry
		cm.usedSize += entry.Size

		return nil
	})
}
