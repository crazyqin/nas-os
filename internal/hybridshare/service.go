// Package hybridshare provides cloud-local hybrid storage management.
package hybridshare

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// CloudBackend 云端存储后端接口
// ============================================================

// CloudProvider 云存储提供商接口
type CloudProvider interface {
	// Upload 上传文件到云端
	Upload(localPath, cloudPath string) error
	// Download 从云端下载文件
	Download(cloudPath, localPath string) error
	// Delete 删除云端文件
	Delete(cloudPath string) error
	// List 列出云端文件
	List(prefix string) ([]CloudFileInfo, error)
	// Stat 获取云端文件信息
	Stat(cloudPath string) (*CloudFileInfo, error)
	// GetURL 获取文件的预签名URL
	GetURL(cloudPath string, expireDuration time.Duration) (string, error)
}

// CloudFileInfo 云端文件信息
type CloudFileInfo struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	ETag    string    `json:"etag,omitempty"`
	IsDir   bool      `json:"is_dir"`
}

// ============================================================
// Service 服务层
// ============================================================

// Service 混合共享服务
type Service struct {
	mu             sync.RWMutex
	configs        map[string]*HybridShareConfig       // id -> config
	metadata       map[string]map[string]*FileMetadata // shareID -> filePath -> metadata
	syncTasks      map[string]*SyncTask                // taskID -> task
	syncLogs       []SyncLog                           // 同步日志
	eventLogs      []EventLog                          // 事件日志
	capacityStats  map[string]*CapacityStats           // shareID -> stats
	bandwidthStats map[string]*BandwidthStats          // shareID -> stats
	providers      map[string]CloudProvider            // shareID -> provider

	// 缓存固定文件列表
	pinnedFiles map[string]map[string]bool // shareID -> filePath -> pinned
}

// NewService 创建新的混合共享服务
func NewService() *Service {
	return &Service{
		configs:        make(map[string]*HybridShareConfig),
		metadata:       make(map[string]map[string]*FileMetadata),
		syncTasks:      make(map[string]*SyncTask),
		syncLogs:       make([]SyncLog, 0),
		eventLogs:      make([]EventLog, 0),
		capacityStats:  make(map[string]*CapacityStats),
		bandwidthStats: make(map[string]*BandwidthStats),
		providers:      make(map[string]CloudProvider),
		pinnedFiles:    make(map[string]map[string]bool),
	}
}

// ============================================================
// 混合共享 CRUD
// ============================================================

// CreateShare 创建混合共享
func (s *Service) CreateShare(req CreateShareRequest) (*HybridShareConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 构建配置
	config := DefaultHybridShareConfig()
	config.ID = uuid.New().String()
	config.Name = req.Name
	config.Description = req.Description
	config.Backend = req.Backend
	config.Endpoint = req.Endpoint
	config.Region = req.Region
	config.Bucket = req.Bucket
	config.AccessKey = req.AccessKey
	config.SecretKey = req.SecretKey
	config.BasePath = req.BasePath
	config.LocalCachePath = req.LocalCachePath
	config.UseSSL = req.UseSSL
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if req.CacheSizeBytes > 0 {
		config.CacheSizeBytes = req.CacheSizeBytes
	}
	if req.CachePolicy != "" {
		config.CachePolicy = req.CachePolicy
	}
	if req.SyncStrategy != "" {
		config.SyncStrategy = req.SyncStrategy
	}
	if req.UploadLimitKBps > 0 {
		config.UploadLimitKBps = req.UploadLimitKBps
	}
	if req.DownloadLimitKBps > 0 {
		config.DownloadLimitKBps = req.DownloadLimitKBps
	}
	config.EncryptionEnabled = req.EncryptionEnabled
	config.EncryptionKey = req.EncryptionKey

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 检查名称唯一性
	for _, c := range s.configs {
		if c.Name == config.Name {
			return nil, fmt.Errorf("share name '%s' already exists", config.Name)
		}
	}

	// 创建本地缓存目录
	if err := os.MkdirAll(config.LocalCachePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// 保存配置
	s.configs[config.ID] = &config
	s.metadata[config.ID] = make(map[string]*FileMetadata)
	s.pinnedFiles[config.ID] = make(map[string]bool)

	// 初始化容量统计
	s.capacityStats[config.ID] = &CapacityStats{
		ShareID:         config.ID,
		LocalCacheTotal: config.CacheSizeBytes,
		LocalCacheFree:  config.CacheSizeBytes,
		UpdatedAt:       time.Now(),
	}

	// 初始化带宽统计
	s.bandwidthStats[config.ID] = &BandwidthStats{
		ShareID:   config.ID,
		UpdatedAt: time.Now(),
	}

	// 记录事件
	s.addEventLog(config.ID, "share_created", fmt.Sprintf("混合共享 '%s' 已创建", config.Name), "")

	return &config, nil
}

// GetShare 获取混合共享配置
func (s *Service) GetShare(id string) (*HybridShareConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, ok := s.configs[id]
	if !ok {
		return nil, fmt.Errorf("share not found: %s", id)
	}
	return config, nil
}

// ListShares 列出所有混合共享
func (s *Service) ListShares() []ShareSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make([]ShareSummary, 0, len(s.configs))
	for _, config := range s.configs {
		summary := ShareSummary{
			ID:              config.ID,
			Name:            config.Name,
			Backend:         config.Backend,
			Bucket:          config.Bucket,
			Status:          config.Status,
			Enabled:         config.Enabled,
			LocalCacheTotal: config.CacheSizeBytes,
			CreatedAt:       config.CreatedAt,
		}

		// 统计缓存使用
		if stats, ok := s.capacityStats[config.ID]; ok {
			summary.LocalCacheUsed = stats.LocalCacheUsed
			summary.CloudUsed = stats.CloudUsed
			summary.TotalFiles = stats.TotalFiles
			summary.CachedFiles = stats.CachedFiles
			summary.PendingSync = stats.PendingFiles
			summary.CacheHitRate = stats.CacheHitRate
		}

		summaries = append(summaries, summary)
	}

	// 按创建时间排序
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	return summaries
}

// UpdateShare 更新混合共享配置
func (s *Service) UpdateShare(id string, req UpdateShareRequest) (*HybridShareConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[id]
	if !ok {
		return nil, fmt.Errorf("share not found: %s", id)
	}

	if req.Name != nil {
		// 检查名称唯一性
		for _, c := range s.configs {
			if c.ID != id && c.Name == *req.Name {
				return nil, fmt.Errorf("share name '%s' already exists", *req.Name)
			}
		}
		config.Name = *req.Name
	}
	if req.Description != nil {
		config.Description = *req.Description
	}
	if req.Endpoint != nil {
		config.Endpoint = *req.Endpoint
	}
	if req.Region != nil {
		config.Region = *req.Region
	}
	if req.Bucket != nil {
		config.Bucket = *req.Bucket
	}
	if req.AccessKey != nil {
		config.AccessKey = *req.AccessKey
	}
	if req.SecretKey != nil {
		config.SecretKey = *req.SecretKey
	}
	if req.BasePath != nil {
		config.BasePath = *req.BasePath
	}
	if req.LocalCachePath != nil {
		config.LocalCachePath = *req.LocalCachePath
	}
	if req.CacheSizeBytes != nil {
		config.CacheSizeBytes = *req.CacheSizeBytes
	}
	if req.CachePolicy != nil {
		config.CachePolicy = *req.CachePolicy
	}
	if req.SyncStrategy != nil {
		config.SyncStrategy = *req.SyncStrategy
	}
	if req.SyncCronExpr != nil {
		config.SyncCronExpr = *req.SyncCronExpr
	}
	if req.UploadLimitKBps != nil {
		config.UploadLimitKBps = *req.UploadLimitKBps
	}
	if req.DownloadLimitKBps != nil {
		config.DownloadLimitKBps = *req.DownloadLimitKBps
	}
	if req.EncryptionEnabled != nil {
		config.EncryptionEnabled = *req.EncryptionEnabled
	}
	if req.EncryptionKey != nil {
		config.EncryptionKey = *req.EncryptionKey
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
		if *req.Enabled {
			config.Status = "active"
		} else {
			config.Status = "inactive"
		}
	}

	config.UpdatedAt = time.Now()

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// DeleteShare 删除混合共享
func (s *Service) DeleteShare(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[id]
	if !ok {
		return fmt.Errorf("share not found: %s", id)
	}

	// 检查是否有正在运行的同步任务
	for _, task := range s.syncTasks {
		if task.ShareID == id && task.Status == SyncTaskRunning {
			return fmt.Errorf("cannot delete share with running sync tasks")
		}
	}

	// 清理本地缓存目录(可选)
	// os.RemoveAll(config.LocalCachePath)

	// 删除配置和相关数据
	delete(s.configs, id)
	delete(s.metadata, id)
	delete(s.pinnedFiles, id)
	delete(s.capacityStats, id)
	delete(s.bandwidthStats, id)
	delete(s.providers, id)

	s.addEventLog(id, "share_deleted", fmt.Sprintf("混合共享 '%s' 已删除", config.Name), "")

	return nil
}

// ============================================================
// 文件操作
// ============================================================

// ListFiles 列出文件
func (s *Service) ListFiles(shareID string, path string) ([]FileMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.configs[shareID]; !ok {
		return nil, fmt.Errorf("share not found: %s", shareID)
	}

	metaMap, ok := s.metadata[shareID]
	if !ok {
		return []FileMetadata{}, nil
	}

	files := make([]FileMetadata, 0)
	prefix := path
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for _, meta := range metaMap {
		// 过滤当前路径下的文件
		if prefix == "" {
			// 根目录: 显示不包含子目录分隔符的文件, 或者第一级目录
			relPath := meta.RelativePath
			if !strings.Contains(relPath, "/") {
				files = append(files, *meta)
			} else {
				// 提取第一级目录
				dir := strings.Split(relPath, "/")[0]
				// 检查是否已经添加了这个目录
				found := false
				for _, f := range files {
					if f.FileName == dir && f.IsCached == false {
						found = true
						break
					}
				}
				if !found {
					files = append(files, FileMetadata{
						ShareID:      shareID,
						RelativePath: dir,
						FileName:     dir,
						FileSize:     0,
						Status:       FileStatusSynced,
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					})
				}
			}
		} else if strings.HasPrefix(meta.RelativePath, prefix) {
			// 子目录: 显示当前目录下的直接子项
			relPath := meta.RelativePath[len(prefix):]
			if !strings.Contains(relPath, "/") {
				files = append(files, *meta)
			} else {
				dir := strings.Split(relPath, "/")[0]
				found := false
				for _, f := range files {
					if f.FileName == dir {
						found = true
						break
					}
				}
				if !found {
					files = append(files, FileMetadata{
						ShareID:      shareID,
						RelativePath: prefix + dir,
						FileName:     dir,
						FileSize:     0,
						Status:       FileStatusSynced,
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					})
				}
			}
		}
	}

	return files, nil
}

// GetFileMetadata 获取文件元数据
func (s *Service) GetFileMetadata(shareID, filePath string) (*FileMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metaMap, ok := s.metadata[shareID]
	if !ok {
		return nil, fmt.Errorf("share not found: %s", shareID)
	}

	meta, ok := metaMap[filePath]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// 更新访问时间和次数
	now := time.Now()
	meta.LastAccessAt = &now
	meta.AccessCount++

	return meta, nil
}

// AddFile 添加文件元数据
func (s *Service) AddFile(shareID string, meta *FileMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[shareID]; !ok {
		return fmt.Errorf("share not found: %s", shareID)
	}

	if meta.ID == "" {
		meta.ID = uuid.New().String()
	}
	meta.ShareID = shareID
	meta.CreatedAt = time.Now()
	meta.UpdatedAt = time.Now()

	if s.metadata[shareID] == nil {
		s.metadata[shareID] = make(map[string]*FileMetadata)
	}
	s.metadata[shareID][meta.RelativePath] = meta

	// 更新统计
	s.updateCapacityStats(shareID)

	return nil
}

// ============================================================
// 缓存操作
// ============================================================

// CacheFile 缓存文件到本地
func (s *Service) CacheFile(shareID, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[shareID]
	if !ok {
		return fmt.Errorf("share not found: %s", shareID)
	}

	metaMap, ok := s.metadata[shareID]
	if !ok {
		return fmt.Errorf("share metadata not initialized")
	}

	meta, ok := metaMap[filePath]
	if !ok {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// 检查是否已缓存
	if meta.IsCached {
		return nil
	}

	// 检查缓存空间
	stats := s.capacityStats[shareID]
	if stats.LocalCacheUsed+meta.FileSize > config.CacheSizeBytes {
		// 需要驱逐一些缓存
		if err := s.evictCache(shareID, meta.FileSize); err != nil {
			return fmt.Errorf("cache full, eviction failed: %w", err)
		}
	}

	// 更新元数据
	now := time.Now()
	meta.IsCached = true
	meta.CachedAt = &now
	meta.LastAccessAt = &now
	meta.AccessCount++
	meta.LocalPath = filepath.Join(config.LocalCachePath, filePath)
	meta.UpdatedAt = now

	// 更新统计
	stats.LocalCacheUsed += meta.FileSize
	stats.LocalCacheFree = config.CacheSizeBytes - stats.LocalCacheUsed
	stats.CachedFiles++
	stats.UpdatedAt = now

	s.addEventLog(shareID, "file_cached", fmt.Sprintf("文件 '%s' 已缓存到本地", filePath), "")

	return nil
}

// EvictFromCache 从缓存驱逐文件
func (s *Service) EvictFromCache(shareID, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[shareID]
	if !ok {
		return fmt.Errorf("share not found: %s", shareID)
	}

	metaMap, ok := s.metadata[shareID]
	if !ok {
		return fmt.Errorf("share metadata not initialized")
	}

	meta, ok := metaMap[filePath]
	if !ok {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// 检查是否固定
	if pinned, ok := s.pinnedFiles[shareID]; ok && pinned[filePath] {
		return fmt.Errorf("cannot evict pinned file")
	}

	if !meta.IsCached {
		return nil
	}

	// 更新元数据
	meta.IsCached = false
	meta.LocalPath = ""
	meta.UpdatedAt = time.Now()

	// 更新统计
	stats := s.capacityStats[shareID]
	stats.LocalCacheUsed -= meta.FileSize
	if stats.LocalCacheUsed < 0 {
		stats.LocalCacheUsed = 0
	}
	stats.LocalCacheFree = config.CacheSizeBytes - stats.LocalCacheUsed
	stats.CachedFiles--
	if stats.CachedFiles < 0 {
		stats.CachedFiles = 0
	}
	stats.UpdatedAt = time.Now()

	s.addEventLog(shareID, "cache_evict", fmt.Sprintf("文件 '%s' 已从缓存驱逐", filePath), "")

	return nil
}

// PinFile 固定文件到缓存(不会被驱逐)
func (s *Service) PinFile(shareID, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[shareID]; !ok {
		return fmt.Errorf("share not found: %s", shareID)
	}

	if s.pinnedFiles[shareID] == nil {
		s.pinnedFiles[shareID] = make(map[string]bool)
	}
	s.pinnedFiles[shareID][filePath] = true

	s.addEventLog(shareID, "file_pinned", fmt.Sprintf("文件 '%s' 已固定到缓存", filePath), "")

	return nil
}

// UnpinFile 取消固定文件
func (s *Service) UnpinFile(shareID, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[shareID]; !ok {
		return fmt.Errorf("share not found: %s", shareID)
	}

	if s.pinnedFiles[shareID] != nil {
		delete(s.pinnedFiles[shareID], filePath)
	}

	return nil
}

// evictCache 驱逐缓存(内部方法，调用时需要持有锁)
func (s *Service) evictCache(shareID string, needBytes int64) error {
	config := s.configs[shareID]
	metaMap := s.metadata[shareID]

	if metaMap == nil {
		return fmt.Errorf("no files to evict")
	}

	// 收集可驱逐的文件(非固定且已缓存)
	type evictCandidate struct {
		path  string
		meta  *FileMetadata
		score float64
	}

	candidates := make([]evictCandidate, 0)
	pinned := s.pinnedFiles[shareID]

	for path, meta := range metaMap {
		if !meta.IsCached {
			continue
		}
		if pinned != nil && pinned[path] {
			continue
		}

		var score float64
		switch config.CachePolicy {
		case CachePolicyLRU:
			// 越久没访问, 分数越高(更容易被驱逐)
			if meta.LastAccessAt != nil {
				score = float64(time.Since(*meta.LastAccessAt).Seconds())
			} else {
				score = float64(time.Since(meta.CreatedAt).Seconds())
			}
		case CachePolicyLFU:
			// 访问次数越少, 分数越高
			if meta.AccessCount > 0 {
				score = 1.0 / float64(meta.AccessCount)
			} else {
				score = 1000000
			}
		case CachePolicyFIFO:
			// 越早创建, 分数越高
			score = float64(time.Since(meta.CreatedAt).Seconds())
		case CachePolicyTTL:
			// 如果超过TTL, 分数最高
			if config.CacheTTLHours > 0 && meta.CachedAt != nil {
				ttl := time.Duration(config.CacheTTLHours) * time.Hour
				if time.Since(*meta.CachedAt) > ttl {
					score = 1000000
				} else {
					score = float64(time.Since(*meta.CachedAt).Seconds())
				}
			} else {
				score = float64(time.Since(meta.CreatedAt).Seconds())
			}
		}

		candidates = append(candidates, evictCandidate{
			path:  path,
			meta:  meta,
			score: score,
		})
	}

	// 按分数排序(高分优先驱逐)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// 驱逐文件直到有足够空间
	freedBytes := int64(0)
	for _, c := range candidates {
		if freedBytes >= needBytes {
			break
		}

		c.meta.IsCached = false
		c.meta.LocalPath = ""
		c.meta.UpdatedAt = time.Now()
		freedBytes += c.meta.FileSize

		stats := s.capacityStats[shareID]
		stats.LocalCacheUsed -= c.meta.FileSize
		stats.CachedFiles--

		s.addEventLog(shareID, "cache_evict", fmt.Sprintf("文件 '%s' 因缓存空间不足被驱逐", c.path), "")
	}

	if freedBytes < needBytes {
		return fmt.Errorf("not enough cache space: need %d, freed %d", needBytes, freedBytes)
	}

	return nil
}

// ============================================================
// 同步操作
// ============================================================

// StartSync 启动同步
func (s *Service) StartSync(req SyncRequest) (*SyncTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[req.ShareID]
	if !ok {
		return nil, fmt.Errorf("share not found: %s", req.ShareID)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("share is disabled")
	}

	// 创建同步任务
	task := &SyncTask{
		ID:        uuid.New().String(),
		ShareID:   req.ShareID,
		Direction: req.Direction,
		FilePath:  req.FilePath,
		Status:    SyncTaskPending,
		CreatedAt: time.Now(),
	}

	if task.Direction == "" {
		task.Direction = SyncDirectionUpload
	}

	s.syncTasks[task.ID] = task

	s.addEventLog(req.ShareID, "sync_start",
		fmt.Sprintf("同步任务已创建: %s", task.ID),
		fmt.Sprintf("方向: %s, 文件: %s", task.Direction, task.FilePath))

	return task, nil
}

// GetSyncTask 获取同步任务
func (s *Service) GetSyncTask(taskID string) (*SyncTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.syncTasks[taskID]
	if !ok {
		return nil, fmt.Errorf("sync task not found: %s", taskID)
	}
	return task, nil
}

// ListSyncTasks 列出同步任务
func (s *Service) ListSyncTasks(shareID string) []*SyncTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*SyncTask, 0)
	for _, task := range s.syncTasks {
		if shareID == "" || task.ShareID == shareID {
			tasks = append(tasks, task)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// CancelSyncTask 取消同步任务
func (s *Service) CancelSyncTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.syncTasks[taskID]
	if !ok {
		return fmt.Errorf("sync task not found: %s", taskID)
	}

	if task.Status != SyncTaskPending && task.Status != SyncTaskRunning {
		return fmt.Errorf("task cannot be cancelled in status: %s", task.Status)
	}

	task.Status = SyncTaskCancelled
	now := time.Now()
	task.CompletedAt = &now

	return nil
}

// UpdateSyncTaskProgress 更新同步任务进度
func (s *Service) UpdateSyncTaskProgress(taskID string, progress float64, bytesSynced int64, speedBps int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.syncTasks[taskID]
	if !ok {
		return fmt.Errorf("sync task not found: %s", taskID)
	}

	task.Progress = progress
	task.BytesSynced = bytesSynced
	task.SpeedBps = speedBps

	if task.Status == SyncTaskPending {
		task.Status = SyncTaskRunning
		now := time.Now()
		task.StartedAt = &now
	}

	return nil
}

// CompleteSyncTask 完成同步任务
func (s *Service) CompleteSyncTask(taskID string, success bool, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.syncTasks[taskID]
	if !ok {
		return fmt.Errorf("sync task not found: %s", taskID)
	}

	now := time.Now()
	task.CompletedAt = &now

	if success {
		task.Status = SyncTaskCompleted
		task.Progress = 100
		s.addEventLog(task.ShareID, "sync_complete",
			fmt.Sprintf("同步任务完成: %s", taskID), "")
	} else {
		task.Status = SyncTaskFailed
		task.Error = errMsg
		s.addEventLog(task.ShareID, "sync_failed",
			fmt.Sprintf("同步任务失败: %s", taskID), errMsg)
	}

	// 更新带宽统计
	s.updateBandwidthStats(task.ShareID, task.SpeedBps, task.Direction)

	return nil
}

// ============================================================
// 容量统计
// ============================================================

// GetCapacityStats 获取容量统计
func (s *Service) GetCapacityStats(shareID string) (*CapacityStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.capacityStats[shareID]
	if !ok {
		return nil, fmt.Errorf("share not found: %s", shareID)
	}

	return stats, nil
}

// GetBandwidthStats 获取带宽统计
func (s *Service) GetBandwidthStats(shareID string) (*BandwidthStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.bandwidthStats[shareID]
	if !ok {
		return nil, fmt.Errorf("share not found: %s", shareID)
	}

	return stats, nil
}

// updateCapacityStats 更新容量统计(内部方法，调用时需要持有锁)
func (s *Service) updateCapacityStats(shareID string) {
	config, ok := s.configs[shareID]
	if !ok {
		return
	}

	stats, ok := s.capacityStats[shareID]
	if !ok {
		stats = &CapacityStats{ShareID: shareID}
		s.capacityStats[shareID] = stats
	}

	metaMap := s.metadata[shareID]
	if metaMap == nil {
		return
	}

	// 重新统计
	var totalFiles, cachedFiles, cloudOnlyFiles, syncedFiles, pendingFiles int64
	var localUsed int64
	var totalAccess, cacheHits int64

	for _, meta := range metaMap {
		totalFiles++
		if meta.IsCached {
			cachedFiles++
			localUsed += meta.FileSize
		}
		if meta.Status == FileStatusCloud {
			cloudOnlyFiles++
		}
		if meta.Status == FileStatusSynced {
			syncedFiles++
		}
		if meta.Status == FileStatusPending {
			pendingFiles++
		}
		totalAccess += meta.AccessCount
		if meta.IsCached && meta.AccessCount > 0 {
			cacheHits += meta.AccessCount
		}
	}

	stats.TotalFiles = totalFiles
	stats.CachedFiles = cachedFiles
	stats.CloudOnlyFiles = cloudOnlyFiles
	stats.SyncedFiles = syncedFiles
	stats.PendingFiles = pendingFiles
	stats.LocalCacheUsed = localUsed
	stats.LocalCacheFree = config.CacheSizeBytes - localUsed
	if totalAccess > 0 {
		stats.CacheHitRate = float64(cacheHits) / float64(totalAccess) * 100
	}
	stats.UpdatedAt = time.Now()
}

// updateBandwidthStats 更新带宽统计(内部方法，调用时需要持有锁)
func (s *Service) updateBandwidthStats(shareID string, speedBps int64, direction SyncDirection) {
	stats, ok := s.bandwidthStats[shareID]
	if !ok {
		stats = &BandwidthStats{ShareID: shareID}
		s.bandwidthStats[shareID] = stats
	}

	if direction == SyncDirectionUpload {
		stats.CurrentUploadBps = speedBps
		if speedBps > stats.PeakUploadBps {
			stats.PeakUploadBps = speedBps
		}
		// 简单移动平均
		if stats.AvgUploadBps == 0 {
			stats.AvgUploadBps = speedBps
		} else {
			stats.AvgUploadBps = (stats.AvgUploadBps + speedBps) / 2
		}
	} else {
		stats.CurrentDownloadBps = speedBps
		if speedBps > stats.PeakDownloadBps {
			stats.PeakDownloadBps = speedBps
		}
		if stats.AvgDownloadBps == 0 {
			stats.AvgDownloadBps = speedBps
		} else {
			stats.AvgDownloadBps = (stats.AvgDownloadBps + speedBps) / 2
		}
	}
	stats.UpdatedAt = time.Now()
}

// ============================================================
// 日志
// ============================================================

// GetSyncLogs 获取同步日志
func (s *Service) GetSyncLogs(shareID string, limit int) []SyncLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logs := make([]SyncLog, 0)
	for i := len(s.syncLogs) - 1; i >= 0; i-- {
		if shareID == "" || s.syncLogs[i].ShareID == shareID {
			logs = append(logs, s.syncLogs[i])
			if limit > 0 && len(logs) >= limit {
				break
			}
		}
	}
	return logs
}

// GetEventLogs 获取事件日志
func (s *Service) GetEventLogs(shareID string, limit int) []EventLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logs := make([]EventLog, 0)
	for i := len(s.eventLogs) - 1; i >= 0; i-- {
		if shareID == "" || s.eventLogs[i].ShareID == shareID {
			logs = append(logs, s.eventLogs[i])
			if limit > 0 && len(logs) >= limit {
				break
			}
		}
	}
	return logs
}

// addSyncLog 添加同步日志(内部方法，调用时需要持有锁)
func (s *Service) addSyncLog(shareID, taskID string, level SyncLogLevel, message, filePath, errMsg string) {
	log := SyncLog{
		ID:        uuid.New().String(),
		ShareID:   shareID,
		TaskID:    taskID,
		Level:     level,
		Message:   message,
		FilePath:  filePath,
		Error:     errMsg,
		Timestamp: time.Now(),
	}
	s.syncLogs = append(s.syncLogs, log)

	// 限制日志数量
	if len(s.syncLogs) > 10000 {
		s.syncLogs = s.syncLogs[len(s.syncLogs)-10000:]
	}
}

// addEventLog 添加事件日志(内部方法，调用时需要持有锁)
func (s *Service) addEventLog(shareID, eventType, message, details string) {
	log := EventLog{
		ID:        uuid.New().String(),
		ShareID:   shareID,
		EventType: eventType,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
	s.eventLogs = append(s.eventLogs, log)

	// 限制日志数量
	if len(s.eventLogs) > 10000 {
		s.eventLogs = s.eventLogs[len(s.eventLogs)-10000:]
	}
}

// ============================================================
// 文件哈希工具
// ============================================================

// CalculateFileMD5 计算文件MD5
func CalculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateFileSHA256 计算文件SHA256
func CalculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FormatBytes 格式化字节数为人类可读格式
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
