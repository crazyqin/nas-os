package storagereclaim

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

// ReclaimManager 存储回收管理器。
type ReclaimManager struct {
	mu             sync.RWMutex
	config         *ReclaimConfig
	files          map[string]*FileInfo       // 文件信息
	recycleBin     map[string]*RecycleBinItem // 回收站
	duplicates     map[string]*DuplicateGroup // 重复文件组
	scanResult     *ScanResult
	lastScanTime   time.Time
	scanStatus     ScanStatus
	reclaimHistory []*ReclaimTask
}

// NewReclaimManager 创建存储回收管理器。
func NewReclaimManager(config *ReclaimConfig) *ReclaimManager {
	if config == nil {
		config = DefaultReclaimConfig()
	}
	return &ReclaimManager{
		config:         config,
		files:          make(map[string]*FileInfo),
		recycleBin:     make(map[string]*RecycleBinItem),
		duplicates:     make(map[string]*DuplicateGroup),
		scanStatus:     ScanStatusIdle,
		reclaimHistory: make([]*ReclaimTask, 0),
	}
}

// GetConfig 获取配置。
func (rm *ReclaimManager) GetConfig() *ReclaimConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.config
}

// UpdateConfig 更新配置。
func (rm *ReclaimManager) UpdateConfig(config *ReclaimConfig) error {
	if config == nil {
		return ErrInvalidConfig
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config = config
	return nil
}

// GetScanStatus 获取扫描状态。
func (rm *ReclaimManager) GetScanStatus() ScanStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.scanStatus
}

// GetLastScanResult 获取最后一次扫描结果。
func (rm *ReclaimManager) GetLastScanResult() *ScanResult {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.scanResult
}

// Scan 扫描指定路径。
func (rm *ReclaimManager) Scan(paths []string) (*ScanResult, error) {
	rm.mu.Lock()
	if rm.scanStatus == ScanStatusScanning {
		rm.mu.Unlock()
		return nil, ErrScanInProgress
	}
	rm.scanStatus = ScanStatusScanning
	rm.mu.Unlock()

	startTime := time.Now()
	scanID := fmt.Sprintf("scan-%d", startTime.Unix())

	result := &ScanResult{
		ScanID:    scanID,
		StartedAt: startTime,
	}

	// 如果没有指定路径，使用默认路径
	if len(paths) == 0 {
		paths = rm.config.ScanPaths
	}

	var errors []string

	// 扫描目录
	for _, path := range paths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				errors = append(errors, fmt.Sprintf("访问 %s 失败: %v", filePath, err))
				return nil
			}

			// 跳过目录
			if info.IsDir() {
				return nil
			}

			result.TotalFiles++
			result.TotalSize += info.Size()

			// 创建文件信息
			fileInfo := rm.analyzeFile(filePath, info)
			rm.mu.Lock()
			rm.files[fileInfo.ID] = fileInfo
			rm.mu.Unlock()

			if fileInfo.IsJunk {
				result.JunkFiles++
				result.JunkSize += fileInfo.Size
				if fileInfo.ReclaimScore >= rm.config.ReclaimThreshold {
					result.Reclaimable += fileInfo.Size
				}
			}

			return nil
		})

		if err != nil {
			errors = append(errors, fmt.Sprintf("扫描路径 %s 失败: %v", path, err))
		}
	}

	// 检测重复文件
	duplicates := rm.detectDuplicates()
	result.Duplicates = len(duplicates)

	finishTime := time.Now()
	result.FinishedAt = finishTime
	result.Duration = finishTime.Sub(startTime)
	result.Errors = errors

	rm.mu.Lock()
	rm.scanResult = result
	rm.scanStatus = ScanStatusCompleted
	rm.lastScanTime = finishTime
	rm.mu.Unlock()

	return result, nil
}

// analyzeFile 分析单个文件。
func (rm *ReclaimManager) analyzeFile(filePath string, info os.FileInfo) *FileInfo {
	ext := filepath.Ext(filePath)
	name := info.Name()
	modTime := info.ModTime()

	// 生成文件 ID
	hash := sha256.New()
	hash.Write([]byte(filePath))
	fileID := fmt.Sprintf("%x", hash.Sum(nil))[:16]

	// 获取文件所有者
	owner := "unknown"

	file := &FileInfo{
		ID:         fileID,
		Path:       filePath,
		Name:       name,
		Size:       info.Size(),
		Extension:  ext,
		Owner:      owner,
		CreatedAt:  modTime, // 简化处理，使用 ModTime
		ModifiedAt: modTime,
		AccessedAt: modTime,
		Importance: ImportanceMedium,
	}

	// 检测垃圾文件
	rm.detectJunkFile(file)

	// 计算回收评分
	file.ReclaimScore = rm.calculateReclaimScore(file)

	return file
}

// detectJunkFile 检测垃圾文件。
func (rm *ReclaimManager) detectJunkFile(file *FileInfo) {
	ext := strings.ToLower(file.Extension)
	name := strings.ToLower(file.Name)

	// 检测临时文件
	for _, tempExt := range rm.config.TempExtensions {
		if ext == tempExt || strings.HasSuffix(name, tempExt) {
			file.IsJunk = true
			file.JunkType = JunkTypeTemp
			file.Importance = ImportanceLow
			return
		}
	}

	// 检测缓存文件
	for _, cachePath := range rm.config.CachePaths {
		if strings.HasPrefix(file.Path, cachePath) {
			file.IsJunk = true
			file.JunkType = JunkTypeCache
			file.Importance = ImportanceLow
			return
		}
	}

	// 检测旧日志文件
	if ext == ".log" || strings.Contains(name, ".log.") {
		daysSinceModified := time.Since(file.ModifiedAt).Hours() / 24
		if daysSinceModified > float64(rm.config.OldLogDays) {
			file.IsJunk = true
			file.JunkType = JunkTypeOldLog
			file.Importance = ImportanceLow
			return
		}
	}

	// 检测孤立快照
	if strings.Contains(name, ".snap") || strings.Contains(name, "snapshot") {
		daysSinceModified := time.Since(file.ModifiedAt).Hours() / 24
		if daysSinceModified > float64(rm.config.OldSnapshotDays) {
			file.IsJunk = true
			file.JunkType = JunkTypeOrphanSnapshot
			file.Importance = ImportanceLow
			return
		}
	}
}

// calculateReclaimScore 计算回收评分。
func (rm *ReclaimManager) calculateReclaimScore(file *FileInfo) float64 {
	var score float64

	// 文件大小评分（越大分数越高）
	sizeScore := float64(file.Size) / (1024 * 1024 * 1024) // GB
	if sizeScore > 100 {
		sizeScore = 100
	}

	// 访问时间评分（越久没访问分数越高）
	daysSinceAccess := time.Since(file.AccessedAt).Hours() / 24
	accessScore := daysSinceAccess / 30 * 100 // 30天 = 100分
	if accessScore > 100 {
		accessScore = 100
	}

	// 重要性评分（重要性越低分数越高）
	importanceScore := float64(4-file.Importance) / 3 * 100

	// 加权计算
	score = sizeScore*rm.config.SizeWeight +
		accessScore*rm.config.AccessWeight +
		importanceScore*rm.config.ImportanceWeight

	// 垃圾文件加分
	if file.IsJunk {
		score += 20
	}

	if score > 100 {
		score = 100
	}

	return score
}

// detectDuplicates 检测重复文件。
func (rm *ReclaimManager) detectDuplicates() map[string]*DuplicateGroup {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 清空之前的重复文件组
	rm.duplicates = make(map[string]*DuplicateGroup)

	// 按文件大小分组
	sizeGroups := make(map[int64][]*FileInfo)
	for _, file := range rm.files {
		if file.Size > 0 {
			sizeGroups[file.Size] = append(sizeGroups[file.Size], file)
		}
	}

	// 对相同大小的文件计算哈希
	for _, files := range sizeGroups {
		if len(files) < 2 {
			continue
		}

		hashGroups := make(map[string][]*FileInfo)
		for _, file := range files {
			hash, err := rm.calculateFileHash(file.Path)
			if err != nil {
				continue
			}
			file.ContentHash = hash
			hashGroups[hash] = append(hashGroups[hash], file)
		}

		// 找出真正的重复文件
		for hash, files := range hashGroups {
			if len(files) < 2 {
				continue
			}

			group := &DuplicateGroup{
				Hash:      hash,
				FileSize:  files[0].Size,
				FileCount: len(files),
				Files:     files,
			}

			// 计算浪费的空间
			group.TotalSize = group.FileSize * int64(group.FileCount)
			group.WastedSize = group.TotalSize - group.FileSize

			// 标记重复文件
			for i, file := range files {
				if i > 0 {
					file.IsJunk = true
					file.JunkType = JunkTypeDuplicate
					file.DuplicateOf = files[0].ID
					file.Importance = ImportanceLow
					file.ReclaimScore = rm.calculateReclaimScore(file)
				}
			}

			rm.duplicates[hash] = group
		}
	}

	return rm.duplicates
}

// calculateFileHash 计算文件哈希（使用 xxhash）。
func (rm *ReclaimManager) calculateFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%016x", h.Sum64()), nil
}

// GetFiles 获取文件列表。
func (rm *ReclaimManager) GetFiles(junkOnly bool, minScore float64) []*FileInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []*FileInfo
	for _, file := range rm.files {
		if file.IsDeleted {
			continue
		}
		if junkOnly && !file.IsJunk {
			continue
		}
		if minScore > 0 && file.ReclaimScore < minScore {
			continue
		}
		result = append(result, file)
	}
	return result
}

// GetFile 获取单个文件信息。
func (rm *ReclaimManager) GetFile(fileID string) (*FileInfo, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	file, ok := rm.files[fileID]
	return file, ok
}

// GetDuplicates 获取重复文件组。
func (rm *ReclaimManager) GetDuplicates() []*DuplicateGroup {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []*DuplicateGroup
	for _, group := range rm.duplicates {
		result = append(result, group)
	}
	return result
}

// ReclaimSpace 回收空间。
func (rm *ReclaimManager) ReclaimSpace(minScore float64, junkTypes []JunkFileType, maxFiles int, dryRun bool) (*ReclaimTask, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	startTime := time.Now()
	taskID := fmt.Sprintf("reclaim-%d", startTime.Unix())

	task := &ReclaimTask{
		ID:        taskID,
		StartedAt: startTime,
		Status:    "running",
		DryRun:    dryRun,
	}

	// 过滤要回收的文件
	var filesToReclaim []*FileInfo
	for _, file := range rm.files {
		if file.IsDeleted {
			continue
		}
		if file.ReclaimScore < minScore {
			continue
		}
		if len(junkTypes) > 0 {
			matched := false
			for _, jt := range junkTypes {
				if file.JunkType == jt {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filesToReclaim = append(filesToReclaim, file)

		if maxFiles > 0 && len(filesToReclaim) >= maxFiles {
			break
		}
	}

	task.FileCount = len(filesToReclaim)

	// 执行回收
	for _, file := range filesToReclaim {
		if dryRun {
			task.Reclaimed += file.Size
			continue
		}

		// 移入回收站
		err := rm.moveToRecycleBin(file)
		if err != nil {
			task.FailedCount++
			task.Errors = append(task.Errors, fmt.Sprintf("回收 %s 失败: %v", file.Path, err))
			continue
		}

		task.Reclaimed += file.Size
	}

	finishTime := time.Now()
	task.FinishedAt = &finishTime
	task.Status = "completed"

	if !dryRun {
		rm.reclaimHistory = append(rm.reclaimHistory, task)
	}

	return task, nil
}

// moveToRecycleBin 移动文件到回收站。
func (rm *ReclaimManager) moveToRecycleBin(file *FileInfo) error {
	// 确保回收站目录存在
	if err := os.MkdirAll(rm.config.RecycleBinPath, 0755); err != nil {
		return fmt.Errorf("创建回收站目录失败: %w", err)
	}

	// 生成回收站中的文件名
	recycleName := fmt.Sprintf("%s_%s", file.ID, file.Name)
	recyclePath := filepath.Join(rm.config.RecycleBinPath, recycleName)

	// 移动文件
	if err := os.Rename(file.Path, recyclePath); err != nil {
		return fmt.Errorf("移动文件失败: %w", err)
	}

	// 更新文件状态
	now := time.Now()
	file.IsDeleted = true
	file.DeletedAt = &now

	// 添加到回收站
	purgeTime := now.AddDate(0, 0, rm.config.RetentionDays)
	rm.recycleBin[file.ID] = &RecycleBinItem{
		FileID:       file.ID,
		OriginalPath: file.Path,
		Name:         file.Name,
		Size:         file.Size,
		DeletedAt:    now,
		Status:       RecycleStatusActive,
		PurgeAt:      &purgeTime,
	}

	return nil
}

// GetRecycleBin 获取回收站内容。
func (rm *ReclaimManager) GetRecycleBin(limit, offset int) []*RecycleBinItem {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var items []*RecycleBinItem
	for _, item := range rm.recycleBin {
		items = append(items, item)
	}

	// 排序（按删除时间倒序）
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].DeletedAt.After(items[i].DeletedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// 分页
	if offset >= len(items) {
		return []*RecycleBinItem{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}

	return items[offset:end]
}

// GetRecycleBinStats 获取回收站统计。
func (rm *ReclaimManager) GetRecycleBinStats() *RecycleBinStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := &RecycleBinStats{
		ItemCount: len(rm.recycleBin),
	}

	for _, item := range rm.recycleBin {
		stats.TotalSize += item.Size
		if stats.OldestItem == nil || item.DeletedAt.Before(*stats.OldestItem) {
			stats.OldestItem = &item.DeletedAt
		}
		if stats.NewestItem == nil || item.DeletedAt.After(*stats.NewestItem) {
			stats.NewestItem = &item.DeletedAt
		}
	}

	return stats
}

// RestoreFromRecycleBin 从回收站恢复文件。
func (rm *ReclaimManager) RestoreFromRecycleBin(fileID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	item, ok := rm.recycleBin[fileID]
	if !ok {
		return ErrFileNotFound
	}

	if item.Status == RecycleStatusPurged {
		return ErrAlreadyDeleted
	}

	// 检查原始路径的目录是否存在
	dir := filepath.Dir(item.OriginalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 构造回收站中的路径
	recycleName := fmt.Sprintf("%s_%s", item.FileID, item.Name)
	recyclePath := filepath.Join(rm.config.RecycleBinPath, recycleName)

	// 恢复文件
	if err := os.Rename(recyclePath, item.OriginalPath); err != nil {
		return ErrRestoreFailed
	}

	// 更新文件状态
	if file, ok := rm.files[fileID]; ok {
		file.IsDeleted = false
		file.DeletedAt = nil
	}

	// 从回收站移除
	delete(rm.recycleBin, fileID)

	return nil
}

// PurgeRecycleBin 清空回收站。
func (rm *ReclaimManager) PurgeRecycleBin(olderThanDays int) (int64, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var purgedSize int64
	var toDelete []string

	for id, item := range rm.recycleBin {
		if item.Status == RecycleStatusPurged {
			continue
		}

		if olderThanDays > 0 {
			daysSinceDeleted := time.Since(item.DeletedAt).Hours() / 24
			if daysSinceDeleted < float64(olderThanDays) {
				continue
			}
		}

		// 删除文件
		recycleName := fmt.Sprintf("%s_%s", item.FileID, item.Name)
		recyclePath := filepath.Join(rm.config.RecycleBinPath, recycleName)
		if err := os.Remove(recyclePath); err != nil {
			continue // 静默处理删除失败
		}

		purgedSize += item.Size
		item.Status = RecycleStatusPurged
		toDelete = append(toDelete, id)
	}

	// 从回收站移除已清理的项目
	for _, id := range toDelete {
		delete(rm.recycleBin, id)
	}

	return purgedSize, nil
}

// GetStorageOverview 获取存储空间总览。
func (rm *ReclaimManager) GetStorageOverview() *StorageOverview {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	overview := &StorageOverview{
		TotalCapacity: 1024 * 1024 * 1024 * 1024, // 1TB 默认
	}

	dirStats := make(map[string]*DirectoryStats)
	typeStats := make(map[string]*FileTypeStats)
	userStats := make(map[string]*UserStats)

	for _, file := range rm.files {
		if file.IsDeleted {
			continue
		}

		overview.UsedSpace += file.Size
		overview.FileCount++

		if file.IsJunk {
			overview.JunkSpace += file.Size
			overview.JunkCount++
			if file.ReclaimScore >= rm.config.ReclaimThreshold {
				overview.Reclaimable += file.Size
			}
		}

		if file.JunkType == JunkTypeDuplicate {
			overview.DuplicateSpace += file.Size
		}

		// 目录统计
		dir := filepath.Dir(file.Path)
		if _, ok := dirStats[dir]; !ok {
			dirStats[dir] = &DirectoryStats{Path: dir}
		}
		ds := dirStats[dir]
		ds.FileCount++
		ds.TotalSize += file.Size
		if file.IsJunk {
			ds.JunkCount++
			ds.JunkSize += file.Size
			if file.ReclaimScore >= rm.config.ReclaimThreshold {
				ds.Reclaimable += file.Size
			}
		}

		// 文件类型统计
		ext := file.Extension
		if ext == "" {
			ext = "(无扩展名)"
		}
		if _, ok := typeStats[ext]; !ok {
			typeStats[ext] = &FileTypeStats{Extension: ext}
		}
		ts := typeStats[ext]
		ts.Count++
		ts.TotalSize += file.Size

		// 用户统计
		if _, ok := userStats[file.Owner]; !ok {
			userStats[file.Owner] = &UserStats{Owner: file.Owner}
		}
		us := userStats[file.Owner]
		us.FileCount++
		us.TotalSize += file.Size
		if file.IsJunk {
			us.JunkCount++
			us.JunkSize += file.Size
		}
	}

	// 计算比例和平均值
	for _, ds := range dirStats {
		if ds.TotalSize > 0 {
			ds.ReclaimRatio = float64(ds.Reclaimable) / float64(ds.TotalSize)
		}
		overview.DirectoryStats = append(overview.DirectoryStats, ds)
	}

	for _, ts := range typeStats {
		if ts.Count > 0 {
			ts.AvgSize = ts.TotalSize / int64(ts.Count)
		}
		overview.FileTypeStats = append(overview.FileTypeStats, ts)
	}

	for _, us := range userStats {
		overview.UserStats = append(overview.UserStats, us)
	}

	overview.FreeSpace = overview.TotalCapacity - overview.UsedSpace

	return overview
}

// GetReclaimHistory 获取回收历史。
func (rm *ReclaimManager) GetReclaimHistory(limit int) []*ReclaimTask {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if limit <= 0 || limit > len(rm.reclaimHistory) {
		limit = len(rm.reclaimHistory)
	}

	// 返回最近的任务
	start := len(rm.reclaimHistory) - limit
	if start < 0 {
		start = 0
	}

	return rm.reclaimHistory[start:]
}
