// Package filetimemachine2 提供文件系统时间机器功能
package filetimemachine2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TimeMachineEngine 时间机器引擎.
type TimeMachineEngine struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	storageRoot     string
	snapshots       map[string]*Snapshot
	snapshotFiles   map[string][]*FileEntry // snapshotID -> 文件列表
	retentionConfig *RetentionConfig
	stopChan        chan struct{}
}

// NewEngine 创建时间机器引擎.
func NewEngine(storageRoot string, logger *zap.Logger) (*TimeMachineEngine, error) {
	if err := os.MkdirAll(storageRoot, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	e := &TimeMachineEngine{
		logger:        logger,
		storageRoot:   storageRoot,
		snapshots:     make(map[string]*Snapshot),
		snapshotFiles: make(map[string][]*FileEntry),
		retentionConfig: &RetentionConfig{
			Enabled: true,
			Rules: []RetentionRule{
				{Name: "每小时保留", Interval: "1h", Count: 24, Priority: 1},
				{Name: "每天保留", Interval: "1d", Count: 30, Priority: 2},
				{Name: "每月保留", Interval: "1m", Count: 12, Priority: 3},
			},
			MaxSnapshots: 200,
			MaxTotalSize: 100 * 1024 * 1024 * 1024, // 100GB
			AutoCleanup:  true,
		},
		stopChan: make(chan struct{}),
	}
	return e, nil
}

// Stop 停止引擎.
func (e *TimeMachineEngine) Stop() {
	close(e.stopChan)
}

// ==================== 快照管理 ====================

// CreateSnapshot 创建快照.
func (e *TimeMachineEngine) CreateSnapshot(req CreateSnapshotRequest) (*Snapshot, error) {
	if err := e.validateRootPath(req.RootPath); err != nil {
		return nil, err
	}

	id := generateID()
	now := time.Now()
	snapshot := &Snapshot{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      SnapshotCreating,
		CreatedAt:   now,
		RootPath:    req.RootPath,
		Tags:        req.Tags,
		IsAuto:      false,
	}

	// 存储快照元数据
	snapshotDir := filepath.Join(e.storageRoot, id)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 扫描文件系统
	var totalSize int64
	var fileCount, dirCount int
	var entries []*FileEntry

	err := filepath.WalkDir(req.RootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			e.logger.Warn("遍历文件跳过", zap.String("path", path), zap.Error(err))
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(req.RootPath, path)

		entry := &FileEntry{
			Name:    d.Name(),
			Path:    relPath,
			IsDir:   d.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime(),
		}

		if !d.IsDir() {
			hash, err := e.fileHash(path)
			if err == nil {
				entry.Hash = hash
			}
			totalSize += info.Size()
			fileCount++
		} else {
			dirCount++
		}

		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		snapshot.Status = SnapshotFailed
		snapshot.ErrorMsg = err.Error()
		e.mu.Lock()
		e.snapshots[id] = snapshot
		e.mu.Unlock()
		return snapshot, fmt.Errorf("扫描文件系统失败: %w", err)
	}

	snapshot.Status = SnapshotCompleted
	snapshot.Size = totalSize
	snapshot.FileCount = fileCount
	snapshot.DirCount = dirCount

	e.mu.Lock()
	e.snapshots[id] = snapshot
	e.snapshotFiles[id] = entries
	e.mu.Unlock()

	// 自动打标签
	if len(req.Tags) == 0 {
		snapshot.Tags = e.autoTag(now)
	}

	e.logger.Info("快照创建完成",
		zap.String("id", id),
		zap.String("name", req.Name),
		zap.Int("files", fileCount),
		zap.Int64("size", totalSize),
	)

	return snapshot, nil
}

// DeleteSnapshot 删除快照.
func (e *TimeMachineEngine) DeleteSnapshot(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	snapshot, ok := e.snapshots[id]
	if !ok {
		return fmt.Errorf("快照不存在: %s", id)
	}
	if snapshot.Status == SnapshotDeleting {
		return fmt.Errorf("快照正在删除中: %s", id)
	}

	snapshot.Status = SnapshotDeleting
	snapshotDir := filepath.Join(e.storageRoot, id)

	if err := os.RemoveAll(snapshotDir); err != nil {
		snapshot.Status = SnapshotCompleted
		return fmt.Errorf("删除快照文件失败: %w", err)
	}

	delete(e.snapshots, id)
	delete(e.snapshotFiles, id)

	e.logger.Info("快照已删除", zap.String("id", id))
	return nil
}

// ListSnapshots 列出快照.
func (e *TimeMachineEngine) ListSnapshots() []SnapshotListItem {
	e.mu.RLock()
	defer e.mu.RUnlock()

	items := make([]SnapshotListItem, 0, len(e.snapshots))
	for _, s := range e.snapshots {
		if s.Status == SnapshotCompleted || s.Status == SnapshotExpired {
			items = append(items, SnapshotListItem{
				ID:        s.ID,
				Name:      s.Name,
				Status:    s.Status,
				CreatedAt: s.CreatedAt,
				Size:      s.Size,
				FileCount: s.FileCount,
				Tags:      s.Tags,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

// GetSnapshot 获取快照详情.
func (e *TimeMachineEngine) GetSnapshot(id string) (*Snapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s, ok := e.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", id)
	}
	return s, nil
}

// ==================== 快照浏览 ====================

// BrowseSnapshot 浏览快照内容.
func (e *TimeMachineEngine) BrowseSnapshot(id string, subPath string) (*SnapshotContent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	snapshot, ok := e.snapshots[id]
	if !ok || snapshot.Status != SnapshotCompleted {
		return nil, fmt.Errorf("快照不存在或未完成: %s", id)
	}

	files, ok := e.snapshotFiles[id]
	if !ok {
		return nil, fmt.Errorf("快照文件数据不存在: %s", id)
	}

	// 清理路径
	subPath = strings.TrimPrefix(subPath, "/")
	subPath = strings.TrimSuffix(subPath, "/")

	var entries []FileEntry
	for _, f := range files {
		fPath := strings.TrimPrefix(f.Path, "/")
		if subPath == "" {
			// 根目录：只显示顶层文件和目录
			if fPath == "" || !strings.Contains(fPath, "/") {
				entries = append(entries, *f)
			}
		} else {
			// 子目录：显示该目录下的直接子项
			if strings.HasPrefix(fPath, subPath+"/") {
				remainder := strings.TrimPrefix(fPath, subPath+"/")
				if !strings.Contains(remainder, "/") {
					entries = append(entries, *f)
				}
			}
		}
	}

	return &SnapshotContent{
		SnapshotID: id,
		Path:       subPath,
		Entries:    entries,
		TotalCount: len(entries),
	}, nil
}

// GetFileContent 获取快照中的文件内容.
func (e *TimeMachineEngine) GetFileContent(snapshotID, filePath string) ([]byte, error) {
	e.mu.RLock()
	snapshot, ok := e.snapshots[snapshotID]
	if !ok || snapshot.Status != SnapshotCompleted {
		e.mu.RUnlock()
		return nil, fmt.Errorf("快照不存在或未完成: %s", snapshotID)
	}
	files := e.snapshotFiles[snapshotID]
	e.mu.RUnlock()

	// 查找文件
	for _, f := range files {
		if f.Path == filePath && !f.IsDir {
			// 从原始路径读取（简化实现：快照数据直接指向原始文件）
			fullPath := filepath.Join(snapshot.RootPath, filePath)
			return os.ReadFile(fullPath)
		}
	}
	return nil, fmt.Errorf("文件不存在: %s", filePath)
}

// ==================== Diff 引擎 ====================

// DiffSnapshots 对比两个快照.
func (e *TimeMachineEngine) DiffSnapshots(snapshotA, snapshotB string) (*DiffResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sa, ok := e.snapshots[snapshotA]
	if !ok || sa.Status != SnapshotCompleted {
		return nil, fmt.Errorf("快照A不存在或未完成: %s", snapshotA)
	}
	sb, ok := e.snapshots[snapshotB]
	if !ok || sb.Status != SnapshotCompleted {
		return nil, fmt.Errorf("快照B不存在或未完成: %s", snapshotB)
	}

	filesA := e.snapshotFiles[snapshotA]
	filesB := e.snapshotFiles[snapshotB]

	// 构建文件映射
	mapA := make(map[string]*FileEntry)
	mapB := make(map[string]*FileEntry)
	for _, f := range filesA {
		if !f.IsDir {
			mapA[f.Path] = f
		}
	}
	for _, f := range filesB {
		if !f.IsDir {
			mapB[f.Path] = f
		}
	}

	var changes []FileDiff
	stats := DiffStats{}

	// 检查B中有而A中没有的（新增）
	for path, fb := range mapB {
		if fa, exists := mapA[path]; exists {
			if fa.Hash != fb.Hash {
				// 修改
				diff := FileDiff{
					Path:    path,
					Type:    DiffModified,
					OldSize: fa.Size,
					NewSize: fb.Size,
					OldHash: fa.Hash,
					NewHash: fb.Hash,
				}
				// 文本文件生成行级diff
				if !e.isBinaryByExt(path) {
					diff.LineDiffs = e.computeLineDiff(
						filepath.Join(sa.RootPath, path),
						filepath.Join(sb.RootPath, path),
					)
				} else {
					diff.IsBinary = true
				}
				changes = append(changes, diff)
				stats.Modified++
				stats.BytesAdded += fb.Size - fa.Size
			}
		} else {
			// 新增
			changes = append(changes, FileDiff{
				Path:    path,
				Type:    DiffAdded,
				NewSize: fb.Size,
				NewHash: fb.Hash,
			})
			stats.Added++
			stats.BytesAdded += fb.Size
		}
	}

	// 检查A中有而B中没有的（删除）
	for path, fa := range mapA {
		if _, exists := mapB[path]; !exists {
			changes = append(changes, FileDiff{
				Path:    path,
				Type:    DiffDeleted,
				OldSize: fa.Size,
				OldHash: fa.Hash,
			})
			stats.Deleted++
			stats.BytesDeleted += fa.Size
		}
	}

	// 按路径排序
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})

	stats.Total = stats.Added + stats.Modified + stats.Deleted + stats.Renamed

	return &DiffResult{
		SnapshotA:  snapshotA,
		SnapshotB:  snapshotB,
		Changes:    changes,
		Stats:      stats,
		ComparedAt: time.Now(),
	}, nil
}

// ==================== 恢复功能 ====================

// RestoreSnapshot 恢复快照.
func (e *TimeMachineEngine) RestoreSnapshot(snapshotID string, req RestoreRequest) (*RestoreResult, error) {
	e.mu.RLock()
	snapshot, ok := e.snapshots[snapshotID]
	if !ok || snapshot.Status != SnapshotCompleted {
		e.mu.RUnlock()
		return nil, fmt.Errorf("快照不存在或未完成: %s", snapshotID)
	}
	files := e.snapshotFiles[snapshotID]
	e.mu.RUnlock()

	result := &RestoreResult{IsDryRun: req.DryRun}

	// 确定要恢复的文件
	targetFiles := make([]*FileEntry, 0)
	if len(req.SourcePaths) > 0 {
		// 按指定路径恢复
		pathSet := make(map[string]bool)
		for _, p := range req.SourcePaths {
			pathSet[p] = true
		}
		for _, f := range files {
			if pathSet[f.Path] || e.isSubPath(f.Path, req.SourcePaths) {
				targetFiles = append(targetFiles, f)
			}
		}
	} else {
		targetFiles = files
	}

	overwrite := req.OverwriteMode == "overwrite"

	for _, f := range targetFiles {
		destPath := filepath.Join(req.TargetPath, f.Path)

		if f.IsDir {
			if !req.DryRun {
				if err := os.MkdirAll(destPath, 0755); err != nil {
					result.FailedFiles++
					result.FailedPaths = append(result.FailedPaths, f.Path)
					continue
				}
			}
			result.RestoredDirs++
			continue
		}

		// 检查目标文件是否存在
		if !overwrite && !req.DryRun {
			if _, err := os.Stat(destPath); err == nil {
				result.SkippedFiles++
				continue
			}
		}

		if !req.DryRun {
			// 确保父目录存在
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				result.FailedFiles++
				result.FailedPaths = append(result.FailedPaths, f.Path)
				continue
			}

			// 从原始路径复制文件
			srcPath := filepath.Join(snapshot.RootPath, f.Path)
			if err := e.copyFile(srcPath, destPath); err != nil {
				result.FailedFiles++
				result.FailedPaths = append(result.FailedPaths, f.Path)
				continue
			}
		}

		result.RestoredFiles++
		result.TotalBytes += f.Size
	}

	return result, nil
}

// ==================== 时间线 ====================

// GetTimeline 获取时间线数据.
func (e *TimeMachineEngine) GetTimeline(granularity AggregationGranularity, startTime, endTime time.Time) (*TimelineData, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if granularity == "" {
		granularity = GranularityDay
	}

	// 收集时间范围内的快照
	var snapshots []*Snapshot
	for _, s := range e.snapshots {
		if s.Status == SnapshotCompleted &&
			(startTime.IsZero() || !s.CreatedAt.Before(startTime)) &&
			(endTime.IsZero() || !s.CreatedAt.After(endTime)) {
			snapshots = append(snapshots, s)
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	// 按粒度分桶
	buckets := e.aggregateByGranularity(snapshots, granularity)

	return &TimelineData{
		Granularity: granularity,
		Buckets:     buckets,
		Total:       len(snapshots),
	}, nil
}

// ==================== 保留策略 ====================

// GetRetentionConfig 获取保留策略配置.
func (e *TimeMachineEngine) GetRetentionConfig() *RetentionConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.retentionConfig
}

// UpdateRetentionConfig 更新保留策略.
func (e *TimeMachineEngine) UpdateRetentionConfig(req UpdateRetentionRequest) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.retentionConfig.Enabled = req.Enabled
	e.retentionConfig.Rules = req.Rules
	e.retentionConfig.MaxSnapshots = req.MaxSnapshots
	e.retentionConfig.MaxTotalSize = req.MaxTotalSize
	e.retentionConfig.AutoCleanup = req.AutoCleanup
}

// CleanupExpired 清理过期快照.
func (e *TimeMachineEngine) CleanupExpired() (*CleanupResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.retentionConfig.Enabled || !e.retentionConfig.AutoCleanup {
		return &CleanupResult{}, nil
	}

	result := &CleanupResult{}
	now := time.Now()

	// 按创建时间排序所有已完成快照
	var sorted []*Snapshot
	for _, s := range e.snapshots {
		if s.Status == SnapshotCompleted {
			sorted = append(sorted, s)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	// 应用保留规则
	keep := make(map[string]bool)
	for _, rule := range e.retentionConfig.Rules {
		cutoff := e.parseInterval(rule.Interval)
		if cutoff == 0 {
			continue
		}
		count := 0
		for _, s := range sorted {
			if now.Sub(s.CreatedAt) <= cutoff && count < rule.Count {
				keep[s.ID] = true
				count++
			}
		}
	}

	// 删除未保留的快照
	for _, s := range sorted {
		if !keep[s.ID] {
			snapshotDir := filepath.Join(e.storageRoot, s.ID)
			if err := os.RemoveAll(snapshotDir); err == nil {
				result.DeletedCount++
				result.ReclaimedBytes += s.Size
				result.DeletedIDs = append(result.DeletedIDs, s.ID)
				delete(e.snapshots, s.ID)
				delete(e.snapshotFiles, s.ID)
			}
		}
	}

	now2 := time.Now()
	e.retentionConfig.LastCleanupAt = &now2
	return result, nil
}

// ==================== 存储统计 ====================

// GetStorageStats 获取存储统计.
func (e *TimeMachineEngine) GetStorageStats() *StorageStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &StorageStats{}
	hashSet := make(map[string]int64) // hash -> size

	var oldest, newest *time.Time

	for _, s := range e.snapshots {
		if s.Status != SnapshotCompleted {
			continue
		}
		stats.TotalSnapshots++
		stats.TotalSize += s.Size

		if oldest == nil || s.CreatedAt.Before(*oldest) {
			t := s.CreatedAt
			oldest = &t
		}
		if newest == nil || s.CreatedAt.After(*newest) {
			t := s.CreatedAt
			newest = &t
		}

		// 统计唯一数据
		for _, f := range e.snapshotFiles[s.ID] {
			if f.Hash != "" {
				if _, exists := hashSet[f.Hash]; !exists {
					hashSet[f.Hash] = f.Size
				}
			}
		}
	}

	stats.OldestSnapshot = oldest
	stats.NewestSnapshot = newest

	for _, size := range hashSet {
		stats.UniqueSize += size
	}

	if stats.TotalSize > 0 {
		stats.DedupRate = 1.0 - float64(stats.UniqueSize)/float64(stats.TotalSize)
	}

	// 简化压缩率估算
	if stats.UniqueSize > 0 {
		stats.CompressedSize = int64(float64(stats.UniqueSize) * 0.7) // 假设压缩率30%
		stats.CompressRate = 1.0 - float64(stats.CompressedSize)/float64(stats.UniqueSize)
	}

	if stats.TotalSnapshots > 0 {
		stats.AvgSnapshotSize = stats.TotalSize / int64(stats.TotalSnapshots)
	}

	return stats
}

// ==================== 搜索 ====================

// SearchFiles 搜索文件版本.
func (e *TimeMachineEngine) SearchFiles(req SearchRequest) (*SearchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	var entries []SearchEntry

	for _, s := range e.snapshots {
		if s.Status != SnapshotCompleted {
			continue
		}
		if !startTime.IsZero() && s.CreatedAt.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && s.CreatedAt.After(endTime) {
			continue
		}
		if req.Tag != "" && !e.hasTag(s.Tags, req.Tag) {
			continue
		}

		for _, f := range e.snapshotFiles[s.ID] {
			if f.IsDir {
				continue
			}
			if req.FileName != "" && !e.matchWildcard(f.Name, req.FileName) {
				continue
			}
			if req.MinSize > 0 && f.Size < req.MinSize {
				continue
			}
			if req.MaxSize > 0 && f.Size > req.MaxSize {
				continue
			}

			entries = append(entries, SearchEntry{
				SnapshotID:   s.ID,
				SnapshotName: s.Name,
				FilePath:     f.Path,
				FileName:     f.Name,
				FileSize:     f.Size,
				ModTime:      f.ModTime,
				SnapshotTime: s.CreatedAt,
			})

			if len(entries) >= limit {
				break
			}
		}
		if len(entries) >= limit {
			break
		}
	}

	return &SearchResult{
		Total:   len(entries),
		Entries: entries,
	}, nil
}

// ==================== 标签管理 ====================

// AddTags 添加标签.
func (e *TimeMachineEngine) AddTags(id string, tags []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s, ok := e.snapshots[id]
	if !ok {
		return fmt.Errorf("快照不存在: %s", id)
	}

	tagSet := make(map[string]bool)
	for _, t := range s.Tags {
		tagSet[t] = true
	}
	for _, t := range tags {
		tagSet[t] = true
	}

	s.Tags = make([]string, 0, len(tagSet))
	for t := range tagSet {
		s.Tags = append(s.Tags, t)
	}
	sort.Strings(s.Tags)
	return nil
}

// ==================== 内部辅助函数 ====================

// validateRootPath 验证根路径.
func (e *TimeMachineEngine) validateRootPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("路径不存在: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录: %s", path)
	}
	return nil
}

// fileHash 计算文件哈希.
func (e *TimeMachineEngine) fileHash(path string) (string, error) {
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

// generateID 生成唯一ID.
func generateID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (8 * (i % 8)))
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

// autoTag 自动打标签.
func (e *TimeMachineEngine) autoTag(t time.Time) []string {
	tags := []string{"auto"}

	hour := t.Hour()
	switch {
	case hour >= 6 && hour < 12:
		tags = append(tags, "morning")
	case hour >= 12 && hour < 18:
		tags = append(tags, "afternoon")
	case hour >= 18 && hour < 22:
		tags = append(tags, "evening")
	default:
		tags = append(tags, "night")
	}

	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		tags = append(tags, "weekend")
	} else {
		tags = append(tags, "weekday")
	}

	return tags
}

// isBinaryByExt 根据扩展名判断是否为二进制文件.
func (e *TimeMachineEngine) isBinaryByExt(path string) bool {
	binaryExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".sqlite": true, ".db": true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExts[ext]
}

// computeLineDiff 计算文本文件行级差异（简化实现）.
func (e *TimeMachineEngine) computeLineDiff(fileA, fileB string) []LineDiff {
	dataA, errA := os.ReadFile(fileA)
	dataB, errB := os.ReadFile(fileB)
	if errA != nil || errB != nil {
		return nil
	}

	linesA := strings.Split(string(dataA), "\n")
	linesB := strings.Split(string(dataB), "\n")

	var diffs []LineDiff
	maxLen := len(linesA)
	if len(linesB) > maxLen {
		maxLen = len(linesB)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(linesA) {
			diffs = append(diffs, LineDiff{
				OldLine: 0,
				NewLine: i + 1,
				Type:    "added",
				Content: linesB[i],
			})
		} else if i >= len(linesB) {
			diffs = append(diffs, LineDiff{
				OldLine: i + 1,
				NewLine: 0,
				Type:    "deleted",
				Content: linesA[i],
			})
		} else if linesA[i] != linesB[i] {
			diffs = append(diffs, LineDiff{
				OldLine: i + 1,
				NewLine: 0,
				Type:    "deleted",
				Content: linesA[i],
			})
			diffs = append(diffs, LineDiff{
				OldLine: 0,
				NewLine: i + 1,
				Type:    "added",
				Content: linesB[i],
			})
		} else {
			diffs = append(diffs, LineDiff{
				OldLine: i + 1,
				NewLine: i + 1,
				Type:    "equal",
				Content: linesA[i],
			})
		}
	}

	return diffs
}

// aggregateByGranularity 按粒度聚合.
func (e *TimeMachineEngine) aggregateByGranularity(snapshots []*Snapshot, granularity AggregationGranularity) []TimelineBucket {
	if len(snapshots) == 0 {
		return nil
	}

	bucketMap := make(map[string]*TimelineBucket)
	var order []string

	for _, s := range snapshots {
		key := e.bucketKey(s.CreatedAt, granularity)
		if _, exists := bucketMap[key]; !exists {
			start, end := e.bucketRange(s.CreatedAt, granularity)
			bucketMap[key] = &TimelineBucket{
				StartTime: start,
				EndTime:   end,
			}
			order = append(order, key)
		}
		bucket := bucketMap[key]
		bucket.SnapshotIDs = append(bucket.SnapshotIDs, s.ID)
		bucket.SnapshotCount++
		bucket.TotalSize += s.Size
	}

	buckets := make([]TimelineBucket, 0, len(order))
	for _, key := range order {
		buckets = append(buckets, *bucketMap[key])
	}
	return buckets
}

// bucketKey 生成桶键.
func (e *TimeMachineEngine) bucketKey(t time.Time, granularity AggregationGranularity) string {
	switch granularity {
	case GranularityHour:
		return t.Format("2006-01-02-15")
	case GranularityDay:
		return t.Format("2006-01-02")
	case GranularityWeek:
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case GranularityMonth:
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// bucketRange 计算桶时间范围.
func (e *TimeMachineEngine) bucketRange(t time.Time, granularity AggregationGranularity) (time.Time, time.Time) {
	switch granularity {
	case GranularityHour:
		start := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
		return start, start.Add(time.Hour)
	case GranularityDay:
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return start, start.AddDate(0, 0, 1)
	case GranularityWeek:
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := time.Date(t.Year(), t.Month(), t.Day()-weekday+1, 0, 0, 0, 0, t.Location())
		return start, start.AddDate(0, 0, 7)
	case GranularityMonth:
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		return start, start.AddDate(0, 1, 0)
	default:
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return start, start.AddDate(0, 0, 1)
	}
}

// parseInterval 解析时间间隔.
func (e *TimeMachineEngine) parseInterval(interval string) time.Duration {
	switch interval {
	case "1h", "1hour":
		return time.Hour
	case "1d", "1day":
		return 24 * time.Hour
	case "1w", "1week":
		return 7 * 24 * time.Hour
	case "1m", "1month":
		return 30 * 24 * time.Hour
	case "1y", "1year":
		return 365 * 24 * time.Hour
	default:
		return 0
	}
}

// hasTag 检查标签.
func (e *TimeMachineEngine) hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// matchWildcard 通配符匹配.
func (e *TimeMachineEngine) matchWildcard(name, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(name, parts[0]) && strings.HasSuffix(name, parts[1])
		}
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(pattern))
}

// isSubPath 检查是否为子路径.
func (e *TimeMachineEngine) isSubPath(path string, parents []string) bool {
	for _, p := range parents {
		if strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// copyFile 复制文件.
func (e *TimeMachineEngine) copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	// 保留权限
	info, err := in.Stat()
	if err == nil {
		os.Chmod(dst, info.Mode())
	}

	return nil
}
