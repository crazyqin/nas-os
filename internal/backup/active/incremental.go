// Package active 增量备份与去重模块
// 实现块级变更追踪、增量备份和数据去重引擎
package active

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ChangeType 变更类型.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create" // 新增
	ChangeTypeModify ChangeType = "modify" // 修改
	ChangeTypeDelete ChangeType = "delete" // 删除
	ChangeTypeRename ChangeType = "rename" // 重命名
)

// FileChange 文件变更记录.
type FileChange struct {
	Path      string     `json:"path"`      // 文件路径
	OldPath   string     `json:"old_path"`  // 原路径（重命名时）
	Type      ChangeType `json:"type"`      // 变更类型
	Size      int64      `json:"size"`      // 文件大小
	Checksum  string     `json:"checksum"`  // 文件校验和
	Timestamp time.Time  `json:"timestamp"` // 变更时间
}

// BlockInfo 数据块信息.
type BlockInfo struct {
	Index     int    `json:"index"`      // 块序号
	Offset    int64  `json:"offset"`     // 文件内偏移量
	Size      int    `json:"size"`       // 块大小
	Checksum  string `json:"checksum"`   // 块校验和
	RefCount  int    `json:"ref_count"`  // 引用计数（去重用）
	StorePath string `json:"store_path"` // 存储路径
}

// ChangeTracker 变更追踪器
// 监控文件系统变更，为增量备份提供变更清单.
type ChangeTracker struct {
	mu           sync.RWMutex
	basePath     string            // 被监控的根路径
	lastSnapshot map[string]string // path -> checksum（上次快照的文件清单）
	currentState map[string]string // path -> checksum（当前状态）
	changes      []FileChange      // 变更记录
	blockSize    int               // 块大小（字节）
	logger       *zap.Logger
	dbPath       string // 追踪数据库路径
}

// NewChangeTracker 创建变更追踪器.
func NewChangeTracker(basePath, dbPath string, blockSize int, logger *zap.Logger) (*ChangeTracker, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if blockSize == 0 {
		blockSize = 4 * 1024 * 1024 // 默认 4MB
	}

	ct := &ChangeTracker{
		basePath:     basePath,
		lastSnapshot: make(map[string]string),
		currentState: make(map[string]string),
		changes:      make([]FileChange, 0),
		blockSize:    blockSize,
		logger:       logger,
		dbPath:       dbPath,
	}

	// 加载上次快照状态
	if err := ct.loadState(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("加载变更追踪状态失败: %w", err)
	}

	return ct, nil
}

// Scan 扫描文件系统变更.
func (ct *ChangeTracker) Scan() ([]FileChange, error) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.changes = make([]FileChange, 0)
	ct.currentState = make(map[string]string)

	// 遍历目录，计算每个文件的校验和
	err := filepath.Walk(ct.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, _ := filepath.Rel(ct.basePath, path)

		// 计算文件校验和
		checksum, err := ct.computeFileChecksum(path)
		if err != nil {
			ct.logger.Warn("计算校验和失败",
				zap.String("path", path),
				zap.Error(err))
			return nil
		}

		ct.currentState[relPath] = checksum

		// 检查是否为新增或修改
		oldChecksum, existed := ct.lastSnapshot[relPath]
		if !existed {
			ct.changes = append(ct.changes, FileChange{
				Path:      relPath,
				Type:      ChangeTypeCreate,
				Size:      info.Size(),
				Checksum:  checksum,
				Timestamp: info.ModTime(),
			})
		} else if oldChecksum != checksum {
			ct.changes = append(ct.changes, FileChange{
				Path:      relPath,
				Type:      ChangeTypeModify,
				Size:      info.Size(),
				Checksum:  checksum,
				Timestamp: info.ModTime(),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描文件系统失败: %w", err)
	}

	// 检查已删除的文件
	for path := range ct.lastSnapshot {
		if _, exists := ct.currentState[path]; !exists {
			ct.changes = append(ct.changes, FileChange{
				Path:      path,
				Type:      ChangeTypeDelete,
				Timestamp: time.Now(),
			})
		}
	}

	ct.logger.Info("变更扫描完成",
		zap.Int("total_files", len(ct.currentState)),
		zap.Int("changes", len(ct.changes)))

	return ct.changes, nil
}

// Commit 将当前状态保存为新基线.
func (ct *ChangeTracker) Commit() error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.lastSnapshot = make(map[string]string, len(ct.currentState))
	for path, checksum := range ct.currentState {
		ct.lastSnapshot[path] = checksum
	}
	ct.changes = make([]FileChange, 0)

	return ct.saveState()
}

// GetChanges 获取当前变更列表.
func (ct *ChangeTracker) GetChanges() []FileChange {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make([]FileChange, len(ct.changes))
	copy(result, ct.changes)
	return result
}

// computeFileChecksum 计算文件 SHA256 校验和.
func (ct *ChangeTracker) computeFileChecksum(path string) (string, error) {
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

// loadState 加载追踪状态.
func (ct *ChangeTracker) loadState() error {
	data, err := os.ReadFile(ct.dbPath)
	if err != nil {
		return err
	}

	var state struct {
		LastSnapshot map[string]string `json:"last_snapshot"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("解析追踪状态失败: %w", err)
	}

	ct.lastSnapshot = state.LastSnapshot
	if ct.lastSnapshot == nil {
		ct.lastSnapshot = make(map[string]string)
	}

	return nil
}

// saveState 保存追踪状态.
func (ct *ChangeTracker) saveState() error {
	data, err := json.MarshalIndent(struct {
		LastSnapshot map[string]string `json:"last_snapshot"`
		UpdatedAt    time.Time         `json:"updated_at"`
	}{
		LastSnapshot: ct.lastSnapshot,
		UpdatedAt:    time.Now(),
	}, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(ct.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(ct.dbPath, data, 0644)
}

// ==================== 块级增量备份 ====================

// BlockLevelBackupEngine 块级增量备份引擎
// 将文件切分为固定大小的数据块，仅备份变更的块.
type BlockLevelBackupEngine struct {
	mu         sync.RWMutex
	blockSize  int                   // 块大小（字节）
	blockIndex map[string]*BlockInfo // checksum -> block info
	dedup      *DedupEngine          // 去重引擎
	logger     *zap.Logger
}

// NewBlockLevelBackupEngine 创建块级备份引擎.
func NewBlockLevelBackupEngine(blockSize int, logger *zap.Logger) *BlockLevelBackupEngine {
	if blockSize == 0 {
		blockSize = 4 * 1024 * 1024 // 4MB
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &BlockLevelBackupEngine{
		blockSize:  blockSize,
		blockIndex: make(map[string]*BlockInfo),
		dedup:      NewDedupEngine(blockSize, logger),
		logger:     logger,
	}
}

// Backup 执行块级增量备份
// 返回新增/变更的块数量和总大小.
func (be *BlockLevelBackupEngine) Backup(filePath, outputDir string) (int, int64, error) {
	be.mu.Lock()
	defer be.mu.Unlock()

	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	_, err = f.Stat()
	if err != nil {
		return 0, 0, fmt.Errorf("获取文件信息失败: %w", err)
	}

	newBlocks := 0
	totalSize := int64(0)
	buf := make([]byte, be.blockSize)
	blockIndex := 0

	for {
		n, err := f.Read(buf)
		if n == 0 {
			break
		}

		blockData := buf[:n]
		checksum := be.dedup.ComputeChecksum(blockData)

		// 检查块是否已存在（去重）
		if existing, exists := be.blockIndex[checksum]; exists {
			existing.RefCount++
			blockIndex++
			continue
		}

		// 写入新块
		blockPath := filepath.Join(outputDir, fmt.Sprintf("block_%08d_%s", blockIndex, checksum[:16]))
		if err := os.WriteFile(blockPath, blockData, 0644); err != nil {
			return 0, 0, fmt.Errorf("写入数据块失败: %w", err)
		}

		blockInfo := &BlockInfo{
			Index:     blockIndex,
			Offset:    int64(blockIndex) * int64(be.blockSize),
			Size:      n,
			Checksum:  checksum,
			RefCount:  1,
			StorePath: blockPath,
		}

		be.blockIndex[checksum] = blockInfo
		newBlocks++
		totalSize += int64(n)
		blockIndex++

		if err != nil {
			break
		}
	}

	be.logger.Debug("块级备份完成",
		zap.String("file", filePath),
		zap.Int("total_blocks", blockIndex),
		zap.Int("new_blocks", newBlocks),
		zap.Int64("new_size", totalSize))

	return newBlocks, totalSize, nil
}

// ==================== 去重引擎 ====================

// DedupEngine 去重引擎
// 使用滚动哈希 + 强校验实现数据块级去重.
type DedupEngine struct {
	mu         sync.RWMutex
	blockSize  int
	chunkIndex map[string]*ChunkEntry // checksum -> chunk info
	stats      DedupStats
	logger     *zap.Logger
}

// ChunkEntry 数据块条目.
type ChunkEntry struct {
	Checksum  string    `json:"checksum"`
	Size      int       `json:"size"`
	RefCount  int       `json:"ref_count"`
	FirstSeen time.Time `json:"first_seen"`
	StorePath string    `json:"store_path"`
}

// DedupStats 去重统计.
type DedupStats struct {
	TotalChunks     int   `json:"total_chunks"`     // 总处理块数
	UniqueChunks    int   `json:"unique_chunks"`    // 唯一块数
	DuplicateChunks int   `json:"duplicate_chunks"` // 重复块数
	TotalBytes      int64 `json:"total_bytes"`      // 总处理字节数
	SavedBytes      int64 `json:"saved_bytes"`      // 节省字节数
}

// NewDedupEngine 创建去重引擎.
func NewDedupEngine(blockSize int, logger *zap.Logger) *DedupEngine {
	if blockSize == 0 {
		blockSize = 4 * 1024 * 1024
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &DedupEngine{
		blockSize:  blockSize,
		chunkIndex: make(map[string]*ChunkEntry),
		logger:     logger,
	}
}

// Dedup 对数据块进行去重检查
// 返回：是否为新块、校验和.
func (de *DedupEngine) Dedup(data []byte) (bool, string) {
	de.mu.Lock()
	defer de.mu.Unlock()

	checksum := de.ComputeChecksum(data)
	de.stats.TotalChunks++
	de.stats.TotalBytes += int64(len(data))

	if entry, exists := de.chunkIndex[checksum]; exists {
		// 已存在相同数据块
		entry.RefCount++
		de.stats.DuplicateChunks++
		de.stats.SavedBytes += int64(len(data))
		return false, checksum
	}

	// 新数据块
	de.chunkIndex[checksum] = &ChunkEntry{
		Checksum:  checksum,
		Size:      len(data),
		RefCount:  1,
		FirstSeen: time.Now(),
	}
	de.stats.UniqueChunks++

	return true, checksum
}

// ComputeChecksum 计算数据块的 SHA256 校验和.
func (de *DedupEngine) ComputeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// GetStats 获取去重统计信息.
func (de *DedupEngine) GetStats() DedupStats {
	de.mu.RLock()
	defer de.mu.RUnlock()
	return de.stats
}

// GetDedupRatio 获取去重率（0.0 ~ 1.0）.
func (de *DedupEngine) GetDedupRatio() float64 {
	de.mu.RLock()
	defer de.mu.RUnlock()

	if de.stats.TotalBytes == 0 {
		return 0.0
	}
	return float64(de.stats.SavedBytes) / float64(de.stats.TotalBytes)
}

// SaveIndex 保存去重索引到磁盘.
func (de *DedupEngine) SaveIndex(indexPath string) error {
	de.mu.RLock()
	defer de.mu.RUnlock()

	data, err := json.MarshalIndent(struct {
		ChunkIndex map[string]*ChunkEntry `json:"chunk_index"`
		Stats      DedupStats             `json:"stats"`
		UpdatedAt  time.Time              `json:"updated_at"`
	}{
		ChunkIndex: de.chunkIndex,
		Stats:      de.stats,
		UpdatedAt:  time.Now(),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化去重索引失败: %w", err)
	}

	dir := filepath.Dir(indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建索引目录失败: %w", err)
	}

	return os.WriteFile(indexPath, data, 0644)
}

// LoadIndex 从磁盘加载去重索引.
func (de *DedupEngine) LoadIndex(indexPath string) error {
	de.mu.Lock()
	defer de.mu.Unlock()

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	var index struct {
		ChunkIndex map[string]*ChunkEntry `json:"chunk_index"`
		Stats      DedupStats             `json:"stats"`
	}

	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("解析去重索引失败: %w", err)
	}

	de.chunkIndex = index.ChunkIndex
	if de.chunkIndex == nil {
		de.chunkIndex = make(map[string]*ChunkEntry)
	}
	de.stats = index.Stats

	de.logger.Info("去重索引加载完成",
		zap.Int("chunks", len(de.chunkIndex)),
		zap.Int64("saved_bytes", de.stats.SavedBytes))

	return nil
}
