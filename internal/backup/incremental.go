package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrBackupNotFound     = errors.New("backup not found")
	ErrBackupInProgress   = errors.New("backup already in progress")
	ErrInvalidConfig      = errors.New("invalid backup configuration")
	ErrVerificationFailed = errors.New("backup verification failed")
)

// IncrementalBackup 增量备份管理器
type IncrementalBackup struct {
	config     *BackupConfig
	snapshots  map[string]*Snapshot
	fileIndex  *FileIndex
	chunkStore *ChunkStore
	activeJobs map[string]*BackupJob
	mu         sync.RWMutex
	logger     *zap.Logger

	// 性能优化
	changeDetector *ChangeDetector
	compressor     *Compressor
}

// Snapshot 快照
type Snapshot struct {
	ID        string              `json:"id"`
	CreatedAt time.Time           `json:"created_at"`
	Type      SnapshotType        `json:"type"`
	BaseID    string              `json:"base_id,omitempty"` // 增量备份的基础快照
	Files     map[string]FileInfo `json:"files"`
	Chunks    []string            `json:"chunks"`
	Size      int64               `json:"size"`
	Duration  time.Duration       `json:"duration"`
	Status    SnapshotStatus      `json:"status"`
	Error     string              `json:"error,omitempty"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
}

// SnapshotType 快照类型
type SnapshotType string

const (
	SnapshotTypeFull SnapshotType = "full"
	SnapshotTypeInc  SnapshotType = "incremental"
	SnapshotTypeDiff SnapshotType = "differential"
)

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	SnapshotStatusPending   SnapshotStatus = "pending"
	SnapshotStatusRunning   SnapshotStatus = "running"
	SnapshotStatusCompleted SnapshotStatus = "completed"
	SnapshotStatusFailed    SnapshotStatus = "failed"
)

// FileInfo 文件信息
type FileInfo struct {
	Path     string      `json:"path"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"mod_time"`
	Mode     os.FileMode `json:"mode"`
	Checksum string      `json:"checksum,omitempty"`
	Chunks   []string    `json:"chunks,omitempty"`
}

// BackupJob 备份作业
type BackupJob struct {
	ID          string
	Source      string
	Destination string
	Type        SnapshotType
	Progress    float64
	StartTime   time.Time
	Status      SnapshotStatus
	Cancel      context.CancelFunc
}

// FileIndex 文件索引（用于快速变更检测）
type FileIndex struct {
	entries map[string]*IndexEntry
	mu      sync.RWMutex
}

// IndexEntry 索引条目
type IndexEntry struct {
	Path        string
	Checksum    string
	Size        int64
	ModTime     time.Time
	LastChecked time.Time
}

// NewFileIndex 创建文件索引
func NewFileIndex() *FileIndex {
	return &FileIndex{
		entries: make(map[string]*IndexEntry),
	}
}

// Update 更新索引
func (fi *FileIndex) Update(path string, checksum string, size int64, modTime time.Time) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.entries[path] = &IndexEntry{
		Path:        path,
		Checksum:    checksum,
		Size:        size,
		ModTime:     modTime,
		LastChecked: time.Now(),
	}
}

// Get 获取索引条目
func (fi *FileIndex) Get(path string) *IndexEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.entries[path]
}

// Remove 移除索引条目
func (fi *FileIndex) Remove(path string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	delete(fi.entries, path)
}

// GetAll 获取所有条目
func (fi *FileIndex) GetAll() []*IndexEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	entries := make([]*IndexEntry, 0, len(fi.entries))
	for _, entry := range fi.entries {
		entries = append(entries, entry)
	}
	return entries
}

// ChangeDetector 变更检测器
type ChangeDetector struct {
	fileIndex *FileIndex
}

// NewChangeDetector 创建变更检测器
func NewChangeDetector(fileIndex *FileIndex) *ChangeDetector {
	return &ChangeDetector{
		fileIndex: fileIndex,
	}
}

// DetectChanges 检测变更文件
func (cd *ChangeDetector) DetectChanges(ctx context.Context, source string) ([]string, []string, []string, error) {
	var added, modified, deleted []string

	// 遍历源目录
	currentFiles := make(map[string]bool)

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		currentFiles[relPath] = true

		// 检查索引
		entry := cd.fileIndex.Get(relPath)
		if entry == nil {
			// 新文件
			added = append(added, relPath)
		} else {
			// 检查是否修改
			if info.ModTime().After(entry.ModTime) || info.Size() != entry.Size {
				modified = append(modified, relPath)
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// 检测删除的文件
	for _, entry := range cd.fileIndex.GetAll() {
		if !currentFiles[entry.Path] {
			deleted = append(deleted, entry.Path)
		}
	}

	return added, modified, deleted, nil
}

// ChunkStore 块存储（用于去重）
type ChunkStore struct {
	chunks map[string][]byte
	refs   map[string]int // 引用计数
	path   string
	mu     sync.RWMutex
}

// NewChunkStore 创建块存储
func NewChunkStore(path string) *ChunkStore {
	return &ChunkStore{
		chunks: make(map[string][]byte),
		refs:   make(map[string]int),
		path:   path,
	}
}

// Store 存储块
func (cs *ChunkStore) Store(id string, data []byte) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 检查是否已存在
	if _, exists := cs.chunks[id]; exists {
		cs.refs[id]++
		return nil
	}

	// 存储新块
	cs.chunks[id] = data
	cs.refs[id] = 1

	return nil
}

// Get 获取块
func (cs *ChunkStore) Get(id string) ([]byte, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	data, exists := cs.chunks[id]
	return data, exists
}

// Remove 移除块（减少引用计数）
func (cs *ChunkStore) Remove(id string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if refs, exists := cs.refs[id]; exists {
		refs--
		if refs <= 0 {
			delete(cs.chunks, id)
			delete(cs.refs, id)
		} else {
			cs.refs[id] = refs
		}
	}
}

// Stats 块存储统计
func (cs *ChunkStore) Stats() map[string]interface{} {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var totalSize int64
	for _, data := range cs.chunks {
		totalSize += int64(len(data))
	}

	return map[string]interface{}{
		"total_chunks": len(cs.chunks),
		"total_size":   totalSize,
		"total_refs":   len(cs.refs),
	}
}

// Compressor 压缩器
type Compressor struct {
	algorithm string
	level     int
}

// NewCompressor 创建压缩器
func NewCompressor(algorithm string, level int) *Compressor {
	return &Compressor{
		algorithm: algorithm,
		level:     level,
	}
}

// NewIncrementalBackup 创建增量备份管理器
func NewIncrementalBackup(config *BackupConfig, logger *zap.Logger) *IncrementalBackup {
	return &IncrementalBackup{
		config:         config,
		snapshots:      make(map[string]*Snapshot),
		fileIndex:      NewFileIndex(),
		chunkStore:     NewChunkStore(config.ChunkPath),
		activeJobs:     make(map[string]*BackupJob),
		changeDetector: NewChangeDetector(NewFileIndex()),
		compressor:     NewCompressor("gzip", 6),
		logger:         logger,
	}
}

// CreateSnapshot 创建快照
func (ib *IncrementalBackup) CreateSnapshot(ctx context.Context, source string, dest string, snapshotType SnapshotType) (*Snapshot, error) {
	ib.mu.Lock()

	// 检查是否有进行中的作业
	for _, job := range ib.activeJobs {
		if job.Source == source && job.Status == SnapshotStatusRunning {
			ib.mu.Unlock()
			return nil, ErrBackupInProgress
		}
	}

	// 创建作业
	jobID := generateID()
	job := &BackupJob{
		ID:          jobID,
		Source:      source,
		Destination: dest,
		Type:        snapshotType,
		Progress:    0,
		StartTime:   time.Now(),
		Status:      SnapshotStatusRunning,
	}

	ib.activeJobs[jobID] = job

	// 创建快照
	snapshot := &Snapshot{
		ID:        jobID,
		CreatedAt: time.Now(),
		Type:      snapshotType,
		Files:     make(map[string]FileInfo),
		Chunks:    make([]string, 0),
		Status:    SnapshotStatusRunning,
	}

	ib.mu.Unlock()

	// 执行备份
	var err error
	switch snapshotType {
	case SnapshotTypeFull:
		err = ib.performFullBackup(ctx, snapshot, source, dest)
	case SnapshotTypeInc:
		err = ib.performIncrementalBackup(ctx, snapshot, source, dest)
	case SnapshotTypeDiff:
		err = ib.performDifferentialBackup(ctx, snapshot, source, dest)
	}

	ib.mu.Lock()
	if err != nil {
		snapshot.Status = SnapshotStatusFailed
		snapshot.Error = err.Error()
		job.Status = SnapshotStatusFailed
	} else {
		snapshot.Status = SnapshotStatusCompleted
		job.Status = SnapshotStatusCompleted
	}
	snapshot.Duration = time.Since(job.StartTime)
	ib.snapshots[jobID] = snapshot
	delete(ib.activeJobs, jobID)
	ib.mu.Unlock()

	return snapshot, err
}

// performFullBackup 执行完整备份
func (ib *IncrementalBackup) performFullBackup(ctx context.Context, snapshot *Snapshot, source, dest string) error {
	ib.logger.Info("Starting full backup",
		zap.String("snapshot_id", snapshot.ID),
		zap.String("source", source),
	)

	var totalSize int64

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		// 计算文件校验和
		checksum, chunks, err := ib.processFile(path, snapshot)
		if err != nil {
			ib.logger.Warn("Failed to process file",
				zap.String("path", path),
				zap.Error(err),
			)
			return nil // 继续处理其他文件
		}

		// 更新文件索引
		ib.fileIndex.Update(relPath, checksum, info.Size(), info.ModTime())

		// 记录文件信息
		snapshot.Files[relPath] = FileInfo{
			Path:     relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			Checksum: checksum,
			Chunks:   chunks,
		}

		totalSize += info.Size()

		return nil
	})

	if err != nil {
		return err
	}

	snapshot.Size = totalSize

	ib.logger.Info("Full backup completed",
		zap.String("snapshot_id", snapshot.ID),
		zap.Int64("size", totalSize),
		zap.Duration("duration", snapshot.Duration),
	)

	return nil
}

// performIncrementalBackup 执行增量备份
func (ib *IncrementalBackup) performIncrementalBackup(ctx context.Context, snapshot *Snapshot, source, dest string) error {
	ib.logger.Info("Starting incremental backup",
		zap.String("snapshot_id", snapshot.ID),
	)

	// 找到最新的完整备份作为基础
	ib.mu.RLock()
	var baseSnapshot *Snapshot
	for _, s := range ib.snapshots {
		if s.Type == SnapshotTypeFull && s.Status == SnapshotStatusCompleted {
			if baseSnapshot == nil || s.CreatedAt.After(baseSnapshot.CreatedAt) {
				baseSnapshot = s
			}
		}
	}
	ib.mu.RUnlock()

	if baseSnapshot == nil {
		// 没有完整备份，执行完整备份
		return ib.performFullBackup(ctx, snapshot, source, dest)
	}

	snapshot.BaseID = baseSnapshot.ID

	// 检测变更
	added, modified, deleted, err := ib.changeDetector.DetectChanges(ctx, source)
	if err != nil {
		return err
	}

	ib.logger.Info("Changes detected",
		zap.Int("added", len(added)),
		zap.Int("modified", len(modified)),
		zap.Int("deleted", len(deleted)),
	)

	// 处理变更文件
	var totalSize int64

	for _, path := range added {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fullPath := filepath.Join(source, path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		checksum, chunks, err := ib.processFile(fullPath, snapshot)
		if err != nil {
			continue
		}

		ib.fileIndex.Update(path, checksum, info.Size(), info.ModTime())

		snapshot.Files[path] = FileInfo{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			Checksum: checksum,
			Chunks:   chunks,
		}

		totalSize += info.Size()
	}

	for _, path := range modified {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fullPath := filepath.Join(source, path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		checksum, chunks, err := ib.processFile(fullPath, snapshot)
		if err != nil {
			continue
		}

		ib.fileIndex.Update(path, checksum, info.Size(), info.ModTime())

		snapshot.Files[path] = FileInfo{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			Checksum: checksum,
			Chunks:   chunks,
		}

		totalSize += info.Size()
	}

	snapshot.Size = totalSize

	return nil
}

// performDifferentialBackup 执行差异备份
func (ib *IncrementalBackup) performDifferentialBackup(ctx context.Context, snapshot *Snapshot, source, dest string) error {
	// 差异备份与增量备份类似，但基于最近的完整备份
	return ib.performIncrementalBackup(ctx, snapshot, source, dest)
}

// processFile 处理文件（分块、去重、存储）
func (ib *IncrementalBackup) processFile(path string, snapshot *Snapshot) (string, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = file.Close() }()

	// 计算完整文件校验和
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", nil, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// 分块处理
	chunks := make([]string, 0)

	// 简化实现：整个文件作为一个块
	file.Seek(0, 0)
	data, err := io.ReadAll(file)
	if err != nil {
		return "", nil, err
	}

	chunkID := checksum
	if err := ib.chunkStore.Store(chunkID, data); err != nil {
		return "", nil, err
	}

	chunks = append(chunks, chunkID)
	snapshot.Chunks = append(snapshot.Chunks, chunkID)

	return checksum, chunks, nil
}

// GetSnapshot 获取快照
func (ib *IncrementalBackup) GetSnapshot(id string) (*Snapshot, error) {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	snapshot, exists := ib.snapshots[id]
	if !exists {
		return nil, ErrBackupNotFound
	}
	return snapshot, nil
}

// ListSnapshots 列出快照
func (ib *IncrementalBackup) ListSnapshots() []*Snapshot {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	snapshots := make([]*Snapshot, 0, len(ib.snapshots))
	for _, snapshot := range ib.snapshots {
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

// DeleteSnapshot 删除快照
func (ib *IncrementalBackup) DeleteSnapshot(id string) error {
	ib.mu.Lock()
	defer ib.mu.Unlock()

	snapshot, exists := ib.snapshots[id]
	if !exists {
		return ErrBackupNotFound
	}

	// 清理块引用
	for _, chunkID := range snapshot.Chunks {
		ib.chunkStore.Remove(chunkID)
	}

	delete(ib.snapshots, id)
	return nil
}

// GetStats 获取统计
func (ib *IncrementalBackup) GetStats() map[string]interface{} {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	var totalSize int64
	for _, snapshot := range ib.snapshots {
		totalSize += snapshot.Size
	}

	return map[string]interface{}{
		"total_snapshots": len(ib.snapshots),
		"total_size":      totalSize,
		"active_jobs":     len(ib.activeJobs),
		"chunk_stats":     ib.chunkStore.Stats(),
	}
}

// generateID 生成ID
func generateID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}
