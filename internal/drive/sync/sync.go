package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SyncDirection 同步方向.
type SyncDirection string

const (
	// SyncBidirectional 双向同步.
	SyncBidirectional SyncDirection = "bidirectional"
	// SyncUpload 仅上传.
	SyncUpload SyncDirection = "upload_only"
	// SyncDownload 仅下载.
	SyncDownload SyncDirection = "download_only"
)

// SyncStatus 同步状态.
type SyncStatus string

const (
	StatusSyncing  SyncStatus = "syncing"
	StatusSynced   SyncStatus = "synced"
	StatusError    SyncStatus = "error"
	StatusConflict SyncStatus = "conflict"
	StatusPending  SyncStatus = "pending"
)

// ConflictStrategy 冲突处理策略.
type ConflictStrategy string

const (
	ConflictNewerWins ConflictStrategy = "newer_wins"
	ConflictKeepBoth  ConflictStrategy = "keep_both"
	ConflictAsk       ConflictStrategy = "ask"
)

// BandwidthConfig 带宽控制配置.
type BandwidthConfig struct {
	UploadBytesPerSec   int64 `json:"upload_bytes_per_sec"`    // 上传限速，0=不限
	DownloadBytesPerSec int64 `json:"download_bytes_per_sec"`  // 下载限速，0=不限
}

// VersionConfig 版本历史配置.
type VersionConfig struct {
	Enabled    bool `json:"enabled"`     // 是否启用版本历史
	MaxVersions int `json:"max_versions"` // 最大保留版本数
	RetentionDays int `json:"retention_days"` // 版本保留天数
}

// SyncConfig 同步配置.
type SyncConfig struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	LocalPath       string           `json:"local_path"`
	RemotePath      string           `json:"remote_path"`
	Direction       SyncDirection    `json:"direction"`
	ConflictPolicy  ConflictStrategy `json:"conflict_policy"`
	Bandwidth       BandwidthConfig  `json:"bandwidth"`
	VersionHistory  VersionConfig    `json:"version_history"`
	ExcludePatterns []string         `json:"exclude_patterns"` // 排除文件模式（glob）
	IncludePatterns []string         `json:"include_patterns"` // 包含文件模式
	Interval        time.Duration    `json:"interval"`         // 自动同步间隔
	Enabled         bool             `json:"enabled"`
}

// FileEntry 文件条目（本地/远端通用）.
type FileEntry struct {
	Path         string      `json:"path"`
	Size         int64       `json:"size"`
	ModTime      time.Time   `json:"mod_time"`
	Checksum     string      `json:"checksum"`
	IsDir        bool        `json:"is_dir"`
	Version      int64       `json:"version"`
	SyncStatus   SyncStatus  `json:"sync_status"`
	LastSyncedAt *time.Time  `json:"last_synced_at,omitempty"`
	Error        string      `json:"error,omitempty"`
}

// SyncEvent 同步事件.
type SyncEvent struct {
	Type      string     `json:"type"`       // "sync_start", "sync_complete", "conflict", "error", "file_synced"
	Path      string     `json:"path"`
	Direction string     `json:"direction"`  // "upload", "download", "bidirectional"
	Timestamp time.Time  `json:"timestamp"`
	Error     string     `json:"error,omitempty"`
	Details   string     `json:"details,omitempty"`
}

// EventHandler 同步事件回调.
type EventHandler func(event SyncEvent)

// RemoteStorage 远端存储接口.
type RemoteStorage interface {
	// List 列出远端路径下的文件.
	List(ctx context.Context, remotePath string) ([]*FileEntry, error)
	// Get 获取远端文件到本地.
	Get(ctx context.Context, remotePath, localPath string) error
	// Put 上传本地文件到远端.
	Put(ctx context.Context, localPath, remotePath string) error
	// Delete 删除远端文件.
	Delete(ctx context.Context, remotePath string) error
	// Stat 获取远端文件信息.
	Stat(ctx context.Context, remotePath string) (*FileEntry, error)
	// Move 移动/重命名远端文件.
	Move(ctx context.Context, oldPath, newPath string) error
}

// SyncEngine 同步引擎.
type SyncEngine struct {
	mu           sync.RWMutex
	config       *SyncConfig
	remote       RemoteStorage
	db           SyncDB
	eventHandler EventHandler

	// 带宽控制
	uploadLimiter   *RateLimiter
	downloadLimiter *RateLimiter

	// 状态跟踪
	localIndex  map[string]*FileEntry // path -> entry
	remoteIndex map[string]*FileEntry
	status      SyncStatus
	stats       SyncStats

	// 版本历史
	versionMgr *VersionManager

	// 冲突处理器
	conflictResolver *ConflictResolver

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	running bool
}

// SyncStats 同步统计.
type SyncStats struct {
	TotalFiles     int       `json:"total_files"`
	SyncedFiles    int       `json:"synced_files"`
	ConflictFiles  int       `json:"conflict_files"`
	ErrorFiles     int       `json:"error_files"`
	UploadedBytes  int64     `json:"uploaded_bytes"`
	DownloadedBytes int64    `json:"downloaded_bytes"`
	StartTime      time.Time `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
}

// SyncDB 同步状态数据库接口.
type SyncDB interface {
	// GetFileRecord 获取文件同步记录.
	GetFileRecord(localPath string) (*FileEntry, error)
	// PutFileRecord 保存文件同步记录.
	PutFileRecord(entry *FileEntry) error
	// DeleteFileRecord 删除文件同步记录.
	DeleteFileRecord(localPath string) error
	// ListRecords 列出所有同步记录.
	ListRecords() ([]*FileEntry, error)
	// Close 关闭数据库.
	Close() error
}

// NewSyncEngine 创建同步引擎.
func NewSyncEngine(cfg *SyncConfig, remote RemoteStorage, db SyncDB) *SyncEngine {
	engine := &SyncEngine{
		config:       cfg,
		remote:       remote,
		db:           db,
		localIndex:   make(map[string]*FileEntry),
		remoteIndex:  make(map[string]*FileEntry),
		conflictResolver: NewConflictResolver(cfg.ConflictPolicy),
	}

	// 初始化带宽限制器
	if cfg.Bandwidth.UploadBytesPerSec > 0 {
		engine.uploadLimiter = NewRateLimiter(cfg.Bandwidth.UploadBytesPerSec)
	}
	if cfg.Bandwidth.DownloadBytesPerSec > 0 {
		engine.downloadLimiter = NewRateLimiter(cfg.Bandwidth.DownloadBytesPerSec)
	}

	// 初始化版本管理器
	if cfg.VersionHistory.Enabled {
		engine.versionMgr = NewVersionManager(cfg.VersionHistory)
	}

	return engine
}

// Start 启动同步引擎.
func (e *SyncEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("sync engine already running")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.running = true
	e.status = StatusSyncing

	go e.syncLoop()

	slog.Info("sync engine started", "config", e.config.Name, "direction", e.config.Direction)
	return nil
}

// Stop 停止同步引擎.
func (e *SyncEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.cancel()
	e.running = false
	e.status = StatusSynced

	slog.Info("sync engine stopped", "config", e.config.Name)
	return nil
}

// SyncNow 立即执行一次同步.
func (e *SyncEngine) SyncNow(ctx context.Context) (*SyncStats, error) {
	e.mu.Lock()
	if !e.running {
		e.ctx = ctx
	}
	e.mu.Unlock()

	stats, err := e.performSync(ctx)
	if err != nil {
		e.emitEvent(SyncEvent{
			Type:      "error",
			Timestamp: time.Now(),
			Error:     err.Error(),
		})
	}
	return stats, err
}

// GetStatus 获取同步状态.
func (e *SyncEngine) GetStatus() SyncStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// GetStats 获取同步统计.
func (e *SyncEngine) GetStats() SyncStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// SetEventHandler 设置事件回调.
func (e *SyncEngine) SetEventHandler(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandler = handler
}

// syncLoop 同步循环.
func (e *SyncEngine) syncLoop() {
	if e.config.Interval <= 0 {
		slog.Info("sync interval not set, running one-time sync")
		e.performSync(e.ctx)
		return
	}

	ticker := time.NewTicker(e.config.Interval)
	defer ticker.Stop()

	// 首次立即执行
	e.performSync(e.ctx)

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if _, err := e.performSync(e.ctx); err != nil {
				slog.Error("periodic sync failed", "error", err)
			}
		}
	}
}

// performSync 执行一次完整同步.
func (e *SyncEngine) performSync(ctx context.Context) (*SyncStats, error) {
	startTime := time.Now()

	e.emitEvent(SyncEvent{
		Type:      "sync_start",
		Timestamp: startTime,
	})

	// 1. 扫描本地文件
	localFiles, err := e.scanLocal(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan local: %w", err)
	}

	// 2. 扫描远端文件
	remoteFiles, err := e.scanRemote(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan remote: %w", err)
	}

	// 3. 构建索引
	e.buildIndex(localFiles, remoteFiles)

	// 4. 分析变更
	actions := e.analyzeChanges(ctx)

	// 5. 执行同步动作
	stats, err := e.executeActions(ctx, actions)
	if err != nil {
		return nil, fmt.Errorf("execute sync: %w", err)
	}

	stats.StartTime = startTime
	now := time.Now()
	stats.EndTime = &now

	e.mu.Lock()
	e.stats = *stats
	if stats.ErrorFiles == 0 && stats.ConflictFiles == 0 {
		e.status = StatusSynced
	} else if stats.ConflictFiles > 0 {
		e.status = StatusConflict
	}
	e.mu.Unlock()

	e.emitEvent(SyncEvent{
		Type:      "sync_complete",
		Timestamp: time.Now(),
		Details:   fmt.Sprintf("synced=%d, conflicts=%d, errors=%d", stats.SyncedFiles, stats.ConflictFiles, stats.ErrorFiles),
	})

	return stats, nil
}

// scanLocal 扫描本地文件.
func (e *SyncEngine) scanLocal(ctx context.Context) ([]*FileEntry, error) {
	var entries []*FileEntry

	err := filepath.Walk(e.config.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误文件
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		relPath, err := filepath.Rel(e.config.LocalPath, path)
		if err != nil {
			return nil
		}

		// 检查排除模式
		if e.shouldExclude(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entry := &FileEntry{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		// 计算文件校验和
		if !info.IsDir() {
			checksum, err := e.computeChecksum(path)
			if err != nil {
				slog.Warn("failed to compute checksum", "path", path, "error", err)
			} else {
				entry.Checksum = checksum
			}
		}

		entries = append(entries, entry)
		return nil
	})

	return entries, err
}

// scanRemote 扫描远端文件.
func (e *SyncEngine) scanRemote(ctx context.Context) ([]*FileEntry, error) {
	entries, err := e.remote.List(ctx, e.config.RemotePath)
	if err != nil {
		return nil, err
	}

	// 过滤排除模式
	var filtered []*FileEntry
	for _, entry := range entries {
		if !e.shouldExclude(entry.Path) {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

// buildIndex 构建本地和远端文件索引.
func (e *SyncEngine) buildIndex(local, remote []*FileEntry) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.localIndex = make(map[string]*FileEntry)
	for _, entry := range local {
		e.localIndex[entry.Path] = entry
	}

	e.remoteIndex = make(map[string]*FileEntry)
	for _, entry := range remote {
		e.remoteIndex[entry.Path] = entry
	}
}

// SyncAction 同步动作.
type SyncAction struct {
	Type     string     // "upload", "download", "delete_local", "delete_remote", "conflict"
	Path     string
	Local    *FileEntry
	Remote   *FileEntry
	Previous *FileEntry // 之前同步的记录
}

// analyzeChanges 分析文件变更，生成同步动作列表.
func (e *SyncEngine) analyzeChanges(ctx context.Context) []*SyncAction {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var actions []*SyncAction

	// 获取所有已知路径
	allPaths := make(map[string]bool)
	for path := range e.localIndex {
		allPaths[path] = true
	}
	for path := range e.remoteIndex {
		allPaths[path] = true
	}

	for path := range allPaths {
		if ctx.Err() != nil {
			break
		}

		local := e.localIndex[path]
		remote := e.remoteIndex[path]
		previous, _ := e.db.GetFileRecord(path)

		action := e.resolveAction(path, local, remote, previous)
		if action != nil {
			actions = append(actions, action)
		}
	}

	return actions
}

// resolveAction 解析单个文件的同步动作.
func (e *SyncEngine) resolveAction(path string, local, remote, previous *FileEntry) *SyncAction {
	// 双向都有文件
	if local != nil && remote != nil {
		// 校验和相同，无需同步
		if local.Checksum == remote.Checksum && local.Checksum != "" {
			return nil
		}

		// 检查是否双方都有修改
		localChanged := previous == nil || local.Checksum != previous.Checksum
		remoteChanged := previous == nil || remote.Checksum != previous.Checksum

		if localChanged && remoteChanged {
			// 冲突
			return &SyncAction{
				Type:   "conflict",
				Path:   path,
				Local:  local,
				Remote: remote,
				Previous: previous,
			}
		}

		if localChanged && (e.config.Direction == SyncBidirectional || e.config.Direction == SyncUpload) {
			return &SyncAction{Type: "upload", Path: path, Local: local, Remote: remote, Previous: previous}
		}

		if remoteChanged && (e.config.Direction == SyncBidirectional || e.config.Direction == SyncDownload) {
			return &SyncAction{Type: "download", Path: path, Local: local, Remote: remote, Previous: previous}
		}

		return nil
	}

	// 仅本地存在
	if local != nil && remote == nil {
		// 检查是否之前同步过（即远端被删除）
		if previous != nil {
			if e.config.Direction == SyncBidirectional || e.config.Direction == SyncDownload {
				return &SyncAction{Type: "delete_local", Path: path, Local: local, Previous: previous}
			}
		} else {
			// 新文件，上传
			if e.config.Direction == SyncBidirectional || e.config.Direction == SyncUpload {
				return &SyncAction{Type: "upload", Path: path, Local: local}
			}
		}
		return nil
	}

	// 仅远端存在
	if remote != nil && local == nil {
		if previous != nil {
			// 本地被删除
			if e.config.Direction == SyncBidirectional || e.config.Direction == SyncUpload {
				return &SyncAction{Type: "delete_remote", Path: path, Remote: remote, Previous: previous}
			}
		} else {
			// 新远端文件，下载
			if e.config.Direction == SyncBidirectional || e.config.Direction == SyncDownload {
				return &SyncAction{Type: "download", Path: path, Remote: remote}
			}
		}
		return nil
	}

	return nil
}

// executeActions 执行同步动作.
func (e *SyncEngine) executeActions(ctx context.Context, actions []*SyncAction) (*SyncStats, error) {
	stats := &SyncStats{TotalFiles: len(actions)}

	for _, action := range actions {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		switch action.Type {
		case "upload":
			err := e.doUpload(ctx, action)
			if err != nil {
				stats.ErrorFiles++
				slog.Error("upload failed", "path", action.Path, "error", err)
			} else {
				stats.SyncedFiles++
				e.emitEvent(SyncEvent{Type: "file_synced", Path: action.Path, Direction: "upload", Timestamp: time.Now()})
			}

		case "download":
			err := e.doDownload(ctx, action)
			if err != nil {
				stats.ErrorFiles++
				slog.Error("download failed", "path", action.Path, "error", err)
			} else {
				stats.SyncedFiles++
				e.emitEvent(SyncEvent{Type: "file_synced", Path: action.Path, Direction: "download", Timestamp: time.Now()})
			}

		case "delete_local":
			localPath := filepath.Join(e.config.LocalPath, action.Path)
			if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
				stats.ErrorFiles++
			} else {
				stats.SyncedFiles++
				e.db.DeleteFileRecord(action.Path)
			}

		case "delete_remote":
			remotePath := filepath.Join(e.config.RemotePath, action.Path)
			if err := e.remote.Delete(ctx, remotePath); err != nil {
				stats.ErrorFiles++
			} else {
				stats.SyncedFiles++
				e.db.DeleteFileRecord(action.Path)
			}

		case "conflict":
			resolved, err := e.conflictResolver.Resolve(ctx, action, e.config.LocalPath, e.config.RemotePath, e.remote)
			if err != nil {
				stats.ErrorFiles++
				slog.Error("conflict resolution failed", "path", action.Path, "error", err)
			} else if resolved {
				stats.SyncedFiles++
			} else {
				stats.ConflictFiles++
			}
			e.emitEvent(SyncEvent{Type: "conflict", Path: action.Path, Timestamp: time.Now()})
		}
	}

	return stats, nil
}

// doUpload 执行上传.
func (e *SyncEngine) doUpload(ctx context.Context, action *SyncAction) error {
	localPath := filepath.Join(e.config.LocalPath, action.Path)
	remotePath := filepath.Join(e.config.RemotePath, action.Path)

	// 版本历史：上传前保存当前远端版本
	if e.versionMgr != nil && action.Remote != nil {
		if err := e.versionMgr.SaveRemoteVersion(ctx, e.remote, action.Remote); err != nil {
			slog.Warn("failed to save version history", "path", action.Path, "error", err)
		}
	}

	// 带宽控制
	if e.uploadLimiter != nil {
		if err := e.uploadLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit: %w", err)
		}
	}

	if err := e.remote.Put(ctx, localPath, remotePath); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	// 更新同步记录
	entry := action.Local.Copy()
	now := time.Now()
	entry.LastSyncedAt = &now
	entry.SyncStatus = StatusSynced
	e.db.PutFileRecord(entry)

	return nil
}

// doDownload 执行下载.
func (e *SyncEngine) doDownload(ctx context.Context, action *SyncAction) error {
	localPath := filepath.Join(e.config.LocalPath, action.Path)
	remotePath := filepath.Join(e.config.RemotePath, action.Path)

	// 版本历史：下载前保存当前本地版本
	if e.versionMgr != nil && action.Local != nil {
		if err := e.versionMgr.SaveLocalVersion(action.Local, e.config.LocalPath); err != nil {
			slog.Warn("failed to save local version", "path", action.Path, "error", err)
		}
	}

	// 带宽控制
	if e.downloadLimiter != nil {
		if err := e.downloadLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit: %w", err)
		}
	}

	if err := e.remote.Get(ctx, remotePath, localPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// 更新同步记录
	entry := action.Remote.Copy()
	now := time.Now()
	entry.LastSyncedAt = &now
	entry.SyncStatus = StatusSynced
	e.db.PutFileRecord(entry)

	return nil
}

// shouldExclude 检查路径是否应排除.
func (e *SyncEngine) shouldExclude(relPath string) bool {
	// 始终排除隐藏文件和系统文件
	base := filepath.Base(relPath)
	if strings.HasPrefix(base, ".") && base != "." {
		return true
	}

	// 检查排除模式
	for _, pattern := range e.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}
		// 也匹配文件名
		matched, err = filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// computeChecksum 计算文件SHA256校验和.
func (e *SyncEngine) computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// emitEvent 发送同步事件.
func (e *SyncEngine) emitEvent(event SyncEvent) {
	e.mu.RLock()
	handler := e.eventHandler
	e.mu.RUnlock()

	if handler != nil {
		handler(event)
	}
}

// Copy 创建FileEntry的副本.
func (f *FileEntry) Copy() *FileEntry {
	cp := *f
	if f.LastSyncedAt != nil {
		t := *f.LastSyncedAt
		cp.LastSyncedAt = &t
	}
	return &cp
}
