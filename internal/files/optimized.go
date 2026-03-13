package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nfnt/resize"
	"go.uber.org/zap"
)

// FileListCache 文件列表缓存
type FileListCache struct {
	cache     map[string]*CachedFileList
	mu        sync.RWMutex
	maxSize   int
	ttl       time.Duration
	hits      int64
	misses    int64
	evictions int64
}

// CachedFileList 缓存的文件列表
type CachedFileList struct {
	Files     []FileInfo `json:"files"`
	Path      string     `json:"path"`
	CachedAt  time.Time  `json:"cachedAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
}

// NewFileListCache 创建文件列表缓存
func NewFileListCache(maxSize int, ttl time.Duration) *FileListCache {
	flc := &FileListCache{
		cache:   make(map[string]*CachedFileList),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// 启动后台清理
	go flc.startCleanup()

	return flc
}

// Get 获取缓存的文件列表
func (c *FileListCache) Get(path string) (*CachedFileList, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[path]
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	atomic.AddInt64(&c.hits, 1)
	return entry, true
}

// Set 设置缓存
func (c *FileListCache) Set(path string, files []FileInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果超过容量，删除过期的或最旧的条目
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	c.cache[path] = &CachedFileList{
		Files:     files,
		Path:      path,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete 删除缓存
func (c *FileListCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, path)
}

// Invalidate 使指定路径及其子路径的缓存失效
func (c *FileListCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.cache {
		if key == path || strings.HasPrefix(key, path+string(os.PathSeparator)) {
			delete(c.cache, key)
		}
	}
}

// Stats 获取缓存统计
func (c *FileListCache) Stats() (hits, misses, evictions int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses),
		atomic.LoadInt64(&c.evictions), len(c.cache)
}

// evictOldest 淘汰最旧的条目
func (c *FileListCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.cache {
		if oldestKey == "" || entry.CachedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CachedAt
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
		atomic.AddInt64(&c.evictions, 1)
	}
}

// startCleanup 启动后台清理
func (c *FileListCache) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.ExpiresAt) {
				delete(c.cache, key)
				atomic.AddInt64(&c.evictions, 1)
			}
		}
		c.mu.Unlock()
	}
}

// ThumbnailCache 缩略图缓存（带大小限制）
type ThumbnailCache struct {
	cache     map[string]*ThumbnailEntry
	mu        sync.RWMutex
	maxSize   int   // 最大条目数
	maxBytes  int64 // 最大字节数
	curBytes  int64
	hits      int64
	misses    int64
	evictions int64
}

// ThumbnailEntry 缩略图缓存条目
type ThumbnailEntry struct {
	Thumbnail string    `json:"thumbnail"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int       `json:"size"` // 文件原始大小，用于检测变化
	ModTime   time.Time `json:"modTime"`
	CachedAt  time.Time `json:"cachedAt"`
	Bytes     int       `json:"bytes"` // 缓存条目字节大小
}

// NewThumbnailCache 创建缩略图缓存
func NewThumbnailCache(maxSize int, maxBytes int64) *ThumbnailCache {
	tc := &ThumbnailCache{
		cache:    make(map[string]*ThumbnailEntry),
		maxSize:  maxSize,
		maxBytes: maxBytes,
	}

	// 启动后台清理
	go tc.startCleanup()

	return tc
}

// Get 获取缩略图
func (c *ThumbnailCache) Get(path string, size int, modTime time.Time) (*ThumbnailEntry, bool) {
	cacheKey := fmt.Sprintf("%s:%d", path, size)

	c.mu.RLock()
	entry, ok := c.cache[cacheKey]
	c.mu.RUnlock()

	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// 检查文件是否已修改
	if entry.Size != size || !entry.ModTime.Equal(modTime) {
		c.mu.Lock()
		delete(c.cache, cacheKey)
		c.curBytes -= int64(entry.Bytes)
		c.mu.Unlock()
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	atomic.AddInt64(&c.hits, 1)
	return entry, true
}

// Set 设置缩略图
func (c *ThumbnailCache) Set(path string, size int, modTime time.Time, thumbnail string, width, height int) {
	cacheKey := fmt.Sprintf("%s:%d", path, size)
	bytes := len(thumbnail)

	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否已存在
	if entry, ok := c.cache[cacheKey]; ok {
		c.curBytes -= int64(entry.Bytes)
	}

	// 如果超过容量，淘汰旧条目
	for (len(c.cache) >= c.maxSize || c.curBytes+int64(bytes) > c.maxBytes) && len(c.cache) > 0 {
		c.evictOldestLocked()
	}

	c.cache[cacheKey] = &ThumbnailEntry{
		Thumbnail: thumbnail,
		Width:     width,
		Height:    height,
		Size:      size,
		ModTime:   modTime,
		CachedAt:  time.Now(),
		Bytes:     bytes,
	}
	c.curBytes += int64(bytes)
}

// Stats 获取统计信息
func (c *ThumbnailCache) Stats() (hits, misses, evictions int64, size int, bytes int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses),
		atomic.LoadInt64(&c.evictions), len(c.cache), c.curBytes
}

// evictOldestLocked 淘汰最旧的条目（已持有锁）
func (c *ThumbnailCache) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	var oldestBytes int

	for key, entry := range c.cache {
		if oldestKey == "" || entry.CachedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CachedAt
			oldestBytes = entry.Bytes
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
		c.curBytes -= int64(oldestBytes)
		atomic.AddInt64(&c.evictions, 1)
	}
}

// startCleanup 启动后台清理
func (c *ThumbnailCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		// 清理超过1小时的条目
		cutoff := time.Now().Add(-1 * time.Hour)
		for key, entry := range c.cache {
			if entry.CachedAt.Before(cutoff) {
				c.curBytes -= int64(entry.Bytes)
				delete(c.cache, key)
				atomic.AddInt64(&c.evictions, 1)
			}
		}
		c.mu.Unlock()
	}
}

// OptimizedManager 优化的文件管理器
type OptimizedManager struct {
	*Manager       // 继承原有功能
	fileListCache  *FileListCache
	thumbnailCache *ThumbnailCache
	logger         *zap.Logger

	// 并发控制
	thumbnailWorkers int
	thumbnailQueue   chan thumbnailRequest
	thumbnailWg      sync.WaitGroup
}

type thumbnailRequest struct {
	path      string
	thumbSize uint
	result    chan<- thumbnailResult
}

type thumbnailResult struct {
	thumbnail string
	width     int
	height    int
}

// NewOptimizedManager 创建优化的文件管理器
func NewOptimizedManager(config PreviewConfig, logger *zap.Logger) *OptimizedManager {
	m := NewManager(config)

	om := &OptimizedManager{
		Manager:          m,
		fileListCache:    NewFileListCache(1000, 30*time.Second),
		thumbnailCache:   NewThumbnailCache(5000, 100*1024*1024), // 5000条目，100MB
		logger:           logger,
		thumbnailWorkers: 4,
		thumbnailQueue:   make(chan thumbnailRequest, 100),
	}

	// 启动缩略图工作池
	om.startThumbnailWorkers()

	return om
}

// startThumbnailWorkers 启动缩略图工作池
func (m *OptimizedManager) startThumbnailWorkers() {
	for i := 0; i < m.thumbnailWorkers; i++ {
		go m.thumbnailWorker()
	}
}

// thumbnailWorker 缩略图工作线程
func (m *OptimizedManager) thumbnailWorker() {
	for req := range m.thumbnailQueue {
		thumbnail, w, h := m.generateThumbnailInternal(req.path, req.thumbSize)
		req.result <- thumbnailResult{thumbnail, w, h}
		m.thumbnailWg.Done()
	}
}

// ListFilesCached 带缓存的文件列表
func (m *OptimizedManager) ListFilesCached(dirPath string, generateThumbnails bool) ([]FileInfo, error) {
	// 尝试从缓存获取
	if cached, ok := m.fileListCache.Get(dirPath); ok {
		if m.logger != nil {
			m.logger.Debug("File list cache hit", zap.String("path", dirPath))
		}
		return cached.Files, nil
	}

	// 获取文件列表
	files, err := m.ListFiles(dirPath, false) // 先不生成缩略图
	if err != nil {
		return nil, err
	}

	// 缓存结果
	m.fileListCache.Set(dirPath, files)

	// 如果需要缩略图，异步生成
	if generateThumbnails {
		go m.generateThumbnailsAsync(dirPath, files)
	}

	return files, nil
}

// generateThumbnailsAsync 异步生成缩略图
func (m *OptimizedManager) generateThumbnailsAsync(dirPath string, files []FileInfo) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range files {
		if files[i].IsDir {
			continue
		}

		fileType := m.GetFileType(files[i].Path)
		if fileType != FileTypeImage && fileType != FileTypeVideo {
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			var thumb string
			var w, h int

			if fileType == FileTypeImage {
				thumb, w, h = m.GetThumbnailCached(files[idx].Path, 256)
			} else if fileType == FileTypeVideo && m.config.EnableVideoThumb {
				thumb = m.GenerateVideoThumbnail(files[idx].Path)
			}

			if thumb != "" {
				mu.Lock()
				files[idx].Thumbnail = thumb
				if w > 0 {
					files[idx].Width = w
					files[idx].Height = h
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// 更新缓存
	m.fileListCache.Set(dirPath, files)
}

// GetThumbnailCached 获取缩略图（带缓存）
func (m *OptimizedManager) GetThumbnailCached(path string, thumbSize uint) (string, int, int) {
	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, 0
	}

	// 检查缓存
	if entry, ok := m.thumbnailCache.Get(path, int(info.Size()), info.ModTime()); ok {
		return entry.Thumbnail, entry.Width, entry.Height
	}

	// 生成缩略图
	thumbnail, w, h := m.generateThumbnailInternal(path, thumbSize)

	// 缓存结果
	if thumbnail != "" {
		m.thumbnailCache.Set(path, int(info.Size()), info.ModTime(), thumbnail, w, h)
	}

	return thumbnail, w, h
}

// generateThumbnailInternal 内部缩略图生成
func (m *OptimizedManager) generateThumbnailInternal(path string, thumbSize uint) (string, int, int) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, 0
	}
	defer file.Close()

	var img image.Image
	ext := strings.ToLower(filepath.Ext(path))

	// 根据格式解码
	switch ext {
	case ".png":
		img, err = png.Decode(file)
	case ".gif":
		// 使用 jpeg 编码，不支持 gif 动画
		img, err = png.Decode(file) // 尝试作为静态图像
		if err != nil {
			img, err = jpeg.Decode(file)
		}
	default:
		img, err = jpeg.Decode(file)
	}

	if err != nil {
		return "", 0, 0
	}

	// 获取原始尺寸
	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// 生成缩略图
	thumb := resize.Thumbnail(thumbSize, thumbSize, img, resize.Lanczos3)

	// 编码为 base64
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return "", 0, 0
	}

	thumbBase64 := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return thumbBase64, origW, origH
}

// InvalidateCache 使缓存失效
func (m *OptimizedManager) InvalidateCache(path string) {
	m.fileListCache.Invalidate(path)
}

// GetCacheStats 获取缓存统计
func (m *OptimizedManager) GetCacheStats() map[string]interface{} {
	hits, misses, evictions, size := m.fileListCache.Stats()
	thits, tmisses, tevictions, tsize, tbytes := m.thumbnailCache.Stats()

	var hitRate, tHitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	ttotal := thits + tmisses
	if ttotal > 0 {
		tHitRate = float64(thits) / float64(ttotal) * 100
	}

	return map[string]interface{}{
		"fileList": map[string]interface{}{
			"hits":      hits,
			"misses":    misses,
			"evictions": evictions,
			"size":      size,
			"hitRate":   hitRate,
		},
		"thumbnail": map[string]interface{}{
			"hits":      thits,
			"misses":    tmisses,
			"evictions": tevictions,
			"size":      tsize,
			"bytes":     tbytes,
			"hitRate":   tHitRate,
		},
	}
}

// Close 关闭管理器
func (m *OptimizedManager) Close() {
	close(m.thumbnailQueue)
	m.thumbnailWg.Wait()
}

// SearchCache 搜索结果缓存
type SearchCache struct {
	cache   map[string]*CachedSearch
	mu      sync.RWMutex
	maxSize int
	ttl     time.Duration
}

// CachedSearch 缓存的搜索结果
type CachedSearch struct {
	Query     string      `json:"query"`
	Result    interface{} `json:"result"`
	CachedAt  time.Time   `json:"cachedAt"`
	ExpiresAt time.Time   `json:"expiresAt"`
}

// NewSearchCache 创建搜索缓存
func NewSearchCache(maxSize int, ttl time.Duration) *SearchCache {
	return &SearchCache{
		cache:   make(map[string]*CachedSearch),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get 获取缓存的搜索结果
func (c *SearchCache) Get(query string) (interface{}, bool) {
	// 计算查询哈希作为键
	key := hashQuery(query)

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Result, true
}

// Set 设置搜索结果缓存
func (c *SearchCache) Set(query string, result interface{}) {
	key := hashQuery(query)

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.cache) >= c.maxSize {
		// 删除最旧的
		var oldestKey string
		var oldestTime time.Time
		for k, e := range c.cache {
			if oldestKey == "" || e.CachedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.CachedAt
			}
		}
		if oldestKey != "" {
			delete(c.cache, oldestKey)
		}
	}

	c.cache[key] = &CachedSearch{
		Query:     query,
		Result:    result,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// hashQuery 计算查询的哈希
func hashQuery(query string) string {
	// 对查询进行规范化和哈希
	h := sha256.New()
	h.Write([]byte(query))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// BatchOperation 批量操作结果
type BatchOperation struct {
	Success []string `json:"success"`
	Failed  []struct {
		Path  string `json:"path"`
		Error string `json:"error"`
	} `json:"failed"`
}

// BatchDelete 批量删除文件
func (m *OptimizedManager) BatchDelete(paths []string) *BatchOperation {
	result := &BatchOperation{
		Success: make([]string, 0),
		Failed: make([]struct {
			Path  string `json:"path"`
			Error string `json:"error"`
		}, 0),
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			result.Failed = append(result.Failed, struct {
				Path  string `json:"path"`
				Error string `json:"error"`
			}{path, err.Error()})
		} else {
			result.Success = append(result.Success, path)
			// 使相关缓存失效
			m.InvalidateCache(filepath.Dir(path))
		}
	}

	return result
}

// BatchRename 批量重命名
func (m *OptimizedManager) BatchRename(renames map[string]string) *BatchOperation {
	result := &BatchOperation{
		Success: make([]string, 0),
		Failed: make([]struct {
			Path  string `json:"path"`
			Error string `json:"error"`
		}, 0),
	}

	for oldPath, newPath := range renames {
		if err := os.Rename(oldPath, newPath); err != nil {
			result.Failed = append(result.Failed, struct {
				Path  string `json:"path"`
				Error string `json:"error"`
			}{oldPath, err.Error()})
		} else {
			result.Success = append(result.Success, oldPath)
			m.InvalidateCache(filepath.Dir(oldPath))
			m.InvalidateCache(filepath.Dir(newPath))
		}
	}

	return result
}

// SortFiles 排序文件列表
func SortFiles(files []FileInfo, sortBy string, desc bool) {
	sort.Slice(files, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = files[i].Name < files[j].Name
		case "size":
			less = files[i].Size < files[j].Size
		case "modTime":
			less = files[i].ModTime < files[j].ModTime
		case "type":
			less = files[i].Type < files[j].Type
		default:
			// 默认按名称排序，目录在前
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			less = files[i].Name < files[j].Name
		}
		if desc {
			return !less
		}
		return less
	})
}

// FilterFiles 过滤文件列表
func FilterFiles(files []FileInfo, filter FileFilter) []FileInfo {
	result := make([]FileInfo, 0)
	for _, f := range files {
		if filter.Match(f) {
			result = append(result, f)
		}
	}
	return result
}

// FileFilter 文件过滤器
type FileFilter struct {
	NamePattern string   `json:"namePattern"`
	Types       []string `json:"types"`
	MinSize     int64    `json:"minSize"`
	MaxSize     int64    `json:"maxSize"`
}

// Match 检查文件是否匹配过滤器
func (f FileFilter) Match(file FileInfo) bool {
	// 名称模式
	if f.NamePattern != "" {
		matched, _ := filepath.Match(f.NamePattern, file.Name)
		if !matched {
			return false
		}
	}

	// 文件类型
	if len(f.Types) > 0 {
		found := false
		for _, t := range f.Types {
			if string(file.Type) == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 文件大小
	if f.MinSize > 0 && file.Size < f.MinSize {
		return false
	}
	if f.MaxSize > 0 && file.Size > f.MaxSize {
		return false
	}

	return true
}

// ToJSON 导出为 JSON
func (f *FileInfo) ToJSON() string {
	data, _ := json.MarshalIndent(f, "", "  ")
	return string(data)
}

// FileInfoFromJSON 从 JSON 解析
func FileInfoFromJSON(data string) (*FileInfo, error) {
	var info FileInfo
	err := json.Unmarshal([]byte(data), &info)
	return &info, err
}
