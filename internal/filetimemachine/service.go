// Package filetimemachine 提供文件时光机核心业务逻辑
package filetimemachine

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Service 文件时光机服务
type Service struct {
	config     *FileTimeMachineConfig
	snapshots  map[string]*Snapshot
	policies   map[string]*SnapshotPolicy
	locks      map[string]*FileLock
	trashItems map[string]*TrashItem
	versions   map[string][]*FileVersion // key: file_path
	mu         sync.RWMutex
}

// NewService 创建服务实例
func NewService(config *FileTimeMachineConfig) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		config:     config,
		snapshots:  make(map[string]*Snapshot),
		policies:   make(map[string]*SnapshotPolicy),
		locks:      make(map[string]*FileLock),
		trashItems: make(map[string]*TrashItem),
		versions:   make(map[string][]*FileVersion),
	}
}

// ==================== 快照管理 ====================

// CreateSnapshot 创建快照
func (s *Service) CreateSnapshot(ctx context.Context, name, description string, trigger SnapshotTrigger, paths []string, tags []string) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := &Snapshot{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Status:      SnapshotStatusCreating,
		Trigger:     trigger,
		Path:        strings.Join(paths, ","),
		Tags:        tags,
		CreatedAt:   time.Now(),
	}

	// 模拟创建快照过程
	totalFiles, totalSize, err := s.scanPaths(ctx, paths)
	if err != nil {
		snapshot.Status = SnapshotStatusFailed
		s.snapshots[snapshot.ID] = snapshot
		return snapshot, fmt.Errorf("扫描路径失败: %w", err)
	}

	snapshot.TotalFiles = totalFiles
	snapshot.TotalSize = totalSize
	snapshot.DeltaSize = totalSize // 首次快照，增量等于全量
	snapshot.Status = SnapshotStatusActive

	s.snapshots[snapshot.ID] = snapshot

	// 更新文件版本
	s.updateVersions(snapshot, paths)

	return snapshot, nil
}

// GetSnapshot 获取快照详情
func (s *Service) GetSnapshot(id string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, ok := s.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", id)
	}
	return snapshot, nil
}

// ListSnapshots 列出快照
func (s *Service) ListSnapshots(opts ListOptions) (*PageResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snapshots []SnapshotSummary
	for _, snap := range s.snapshots {
		if opts.Status != "" && string(snap.Status) != opts.Status {
			continue
		}
		snapshots = append(snapshots, SnapshotSummary{
			ID:         snap.ID,
			Name:       snap.Name,
			Status:     snap.Status,
			Trigger:    snap.Trigger,
			TotalFiles: snap.TotalFiles,
			TotalSize:  snap.TotalSize,
			DeltaSize:  snap.DeltaSize,
			CreatedAt:  snap.CreatedAt,
		})
	}

	// 排序
	sort.Slice(snapshots, func(i, j int) bool {
		if opts.SortOrder == "asc" {
			return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
		}
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	// 分页
	total := len(snapshots)
	page := opts.Page
	pageSize := opts.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &PageResult{
		Items:      snapshots[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// DeleteSnapshot 删除快照
func (s *Service) DeleteSnapshot(id string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[id]
	if !ok {
		return fmt.Errorf("快照不存在: %s", id)
	}

	// 检查是否有锁定的文件
	if !force {
		for _, lock := range s.locks {
			// 简化检查：如果有任何锁定文件且非软锁，拒绝删除
			if lock.LockType != LockTypeSoft {
				return fmt.Errorf("存在锁定文件，无法删除快照: %s", lock.FilePath)
			}
		}
	}

	snapshot.Status = SnapshotStatusDeleting
	delete(s.snapshots, id)
	return nil
}

// ==================== 差异对比 ====================

// CompareSnapshots 比较两个快照
func (s *Service) CompareSnapshots(oldID, newID string) (*SnapshotDiff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	oldSnap, ok := s.snapshots[oldID]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", oldID)
	}

	newSnap, ok := s.snapshots[newID]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", newID)
	}

	// 模拟差异分析
	diff := &SnapshotDiff{
		OldSnapshotID: oldID,
		NewSnapshotID: newID,
		GeneratedAt:   time.Now(),
	}

	// 简化：基于路径数量差异模拟
	oldPaths := strings.Split(oldSnap.Path, ",")
	newPaths := strings.Split(newSnap.Path, ",")

	oldPathSet := make(map[string]bool)
	for _, p := range oldPaths {
		oldPathSet[p] = true
	}

	newPathSet := make(map[string]bool)
	for _, p := range newPaths {
		newPathSet[p] = true
	}

	var files []FileDiff

	// 检查新增和修改
	for _, p := range newPaths {
		if !oldPathSet[p] {
			files = append(files, FileDiff{
				FilePath: p,
				Type:     DiffAdded,
				FileType: FileTypeText,
				NewSize:  1024, // 模拟大小
			})
			diff.Added++
		} else {
			// 模拟修改
			files = append(files, FileDiff{
				FilePath:   p,
				Type:       DiffModified,
				FileType:   FileTypeText,
				OldVersion: oldID,
				NewVersion: newID,
				OldSize:    1024,
				NewSize:    1536,
				Additions:  10,
				Deletions:  5,
			})
			diff.Modified++
		}
	}

	// 检查删除
	for _, p := range oldPaths {
		if !newPathSet[p] {
			files = append(files, FileDiff{
				FilePath: p,
				Type:     DiffDeleted,
				FileType: FileTypeText,
				OldSize:  1024,
			})
			diff.Deleted++
		}
	}

	diff.TotalFiles = len(files)
	diff.Files = files
	diff.TotalAdditions = diff.Added*10 + diff.Modified*10
	diff.TotalDeletions = diff.Modified * 5

	return diff, nil
}

// CompareFileVersions 比较文件的不同版本
func (s *Service) CompareFileVersions(filePath, oldVersionID, newVersionID string) (*FileDiff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.versions[filePath]
	if !ok {
		return nil, fmt.Errorf("文件版本不存在: %s", filePath)
	}

	var oldVer, newVer *FileVersion
	for _, v := range versions {
		if v.ID == oldVersionID {
			oldVer = v
		}
		if v.ID == newVersionID {
			newVer = v
		}
	}

	if oldVer == nil {
		return nil, fmt.Errorf("旧版本不存在: %s", oldVersionID)
	}
	if newVer == nil {
		return nil, fmt.Errorf("新版本不存在: %s", newVersionID)
	}

	diff := &FileDiff{
		FilePath:   filePath,
		Type:       DiffModified,
		FileType:   newVer.FileType,
		OldVersion: oldVersionID,
		NewVersion: newVersionID,
		OldSize:    oldVer.Size,
		NewSize:    newVer.Size,
	}

	// 根据文件类型生成差异
	switch newVer.FileType {
	case FileTypeText, FileTypeCode:
		diff.Hunks = s.generateTextDiff(oldVer, newVer)
		for _, hunk := range diff.Hunks {
			diff.Additions += hunk.NewLines
			diff.Deletions += hunk.OldLines
		}
	case FileTypeImage:
		diff.ImageDiff = &ImageDiff{
			Similarity: 0.85,
		}
	default:
		diff.BinaryDiff = true
	}

	return diff, nil
}

// ==================== 文件恢复 ====================

// RestoreFile 恢复文件到指定快照版本
func (s *Service) RestoreFile(ctx context.Context, snapshotID, filePath, targetPath string) (*FileVersion, error) {
	s.mu.RLock()
	snapshot, ok := s.snapshots[snapshotID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("快照不存在: %s", snapshotID)
	}
	s.mu.RUnlock()

	// 检查文件锁
	s.mu.RLock()
	if lock, exists := s.locks[filePath]; exists {
		if lock.LockType == LockTypeHard {
			s.mu.RUnlock()
			return nil, fmt.Errorf("文件被硬锁锁定，无法恢复: %s", filePath)
		}
	}
	s.mu.RUnlock()

	if targetPath == "" {
		targetPath = filePath
	}

	// 创建恢复的文件版本
	version := &FileVersion{
		ID:          uuid.New().String(),
		SnapshotID:  snapshotID,
		FilePath:    filePath,
		FullPath:    targetPath,
		FileType:    FileTypeText,
		Size:        1024,
		ModTime:     snapshot.CreatedAt,
		Permissions: "0644",
		MD5:         s.calculateMD5(filePath),
		SHA256:      s.calculateSHA256(filePath),
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.versions[filePath] = append(s.versions[filePath], version)
	s.mu.Unlock()

	return version, nil
}

// RollbackToSnapshot 回滚到指定快照
func (s *Service) RollbackToSnapshot(ctx context.Context, req RollbackRequest) (*RollbackResult, error) {
	s.mu.RLock()
	snapshot, ok := s.snapshots[req.SnapshotID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("快照不存在: %s", req.SnapshotID)
	}
	s.mu.RUnlock()

	result := &RollbackResult{
		ID:         uuid.New().String(),
		SnapshotID: req.SnapshotID,
		Status:     RollbackRunning,
		Mode:       req.Mode,
		StartedAt:  time.Now(),
	}

	// 如果需要先备份
	if req.BackupFirst {
		backupSnap, err := s.CreateSnapshot(ctx,
			fmt.Sprintf("rollback-backup-%s", result.ID),
			"回滚前自动备份",
			TriggerManual,
			[]string{req.TargetPath},
			[]string{"rollback-backup"},
		)
		if err != nil {
			result.Status = RollbackFailed
			result.Errors = append(result.Errors, RollbackError{
				Error: fmt.Sprintf("创建备份失败: %v", err),
			})
			return result, err
		}
		result.BackupSnapshot = backupSnap.ID
	}

	// 模拟回滚过程
	if req.DryRun {
		result.Status = RollbackCompleted
		result.TotalFiles = int(snapshot.TotalFiles)
		result.RestoredFiles = int(snapshot.TotalFiles)
		now := time.Now()
		result.CompletedAt = &now
		result.Duration = time.Second
		return result, nil
	}

	// 实际回滚（模拟）
	result.TotalFiles = int(snapshot.TotalFiles)
	result.RestoredFiles = int(snapshot.TotalFiles)
	result.Status = RollbackCompleted
	now := time.Now()
	result.CompletedAt = &now
	result.Duration = time.Second * 5

	return result, nil
}

// ==================== 批量回滚 ====================

// BatchRollback 批量文件回滚
func (s *Service) BatchRollback(ctx context.Context, snapshotID string, filePaths []string, mode RollbackMode) (*RollbackResult, error) {
	s.mu.RLock()
	_, ok := s.snapshots[snapshotID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("快照不存在: %s", snapshotID)
	}
	s.mu.RUnlock()

	result := &RollbackResult{
		ID:         uuid.New().String(),
		SnapshotID: snapshotID,
		Status:     RollbackRunning,
		Mode:       mode,
		StartedAt:  time.Now(),
		TotalFiles: len(filePaths),
	}

	// 检查文件锁
	var errors []RollbackError
	restoredCount := 0
	skippedCount := 0

	for _, filePath := range filePaths {
		s.mu.RLock()
		if lock, exists := s.locks[filePath]; exists {
			if lock.LockType == LockTypeHard {
				errors = append(errors, RollbackError{
					FilePath: filePath,
					Error:    "文件被硬锁锁定",
				})
				skippedCount++
				s.mu.RUnlock()
				continue
			}
		}
		s.mu.RUnlock()

		// 模拟恢复
		restoredCount++
	}

	result.RestoredFiles = restoredCount
	result.SkippedFiles = skippedCount
	result.FailedFiles = len(errors)
	result.Errors = errors
	result.Status = RollbackCompleted
	if len(errors) > 0 && restoredCount > 0 {
		result.Status = RollbackPartial
	}

	now := time.Now()
	result.CompletedAt = &now
	result.Duration = time.Duration(len(filePaths)) * time.Millisecond * 100

	return result, nil
}

// ==================== 快照策略管理 ====================

// CreatePolicy 创建快照策略
func (s *Service) CreatePolicy(policy *SnapshotPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	// 计算下次执行时间
	policy.NextRunAt = s.calculateNextRun(policy)

	s.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取策略
func (s *Service) GetPolicy(id string) (*SnapshotPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, ok := s.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出策略
func (s *Service) ListPolicies() []*SnapshotPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]*SnapshotPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdatePolicy 更新策略
func (s *Service) UpdatePolicy(id string, policy *SnapshotPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.policies[id]
	if !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	policy.ID = id
	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()
	policy.NextRunAt = s.calculateNextRun(policy)

	s.policies[id] = policy
	return nil
}

// DeletePolicy 删除策略
func (s *Service) DeletePolicy(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.policies[id]; !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}
	delete(s.policies, id)
	return nil
}

// ==================== 回收站管理 ====================

// MoveToTrash 移动到回收站
func (s *Service) MoveToTrash(filePath, deletedBy, source string) (*TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查文件锁
	if lock, exists := s.locks[filePath]; exists {
		if lock.LockType != LockTypeSoft {
			return nil, fmt.Errorf("文件被锁定，无法删除: %s", filePath)
		}
	}

	item := &TrashItem{
		ID:           uuid.New().String(),
		OriginalPath: filePath,
		TrashPath:    filepath.Join(s.config.TrashRoot, filepath.Base(filePath)),
		FileName:     filepath.Base(filePath),
		FileType:     FileTypeText,
		Size:         1024,
		DeletedAt:    time.Now(),
		DeletedBy:    deletedBy,
		Source:       source,
		Status:       TrashStatusActive,
		MD5:          s.calculateMD5(filePath),
	}

	// 设置自动清除时间
	if s.config.TrashAutoPurgeDays > 0 {
		purgeAt := time.Now().AddDate(0, 0, s.config.TrashAutoPurgeDays)
		item.AutoPurgeAt = &purgeAt
	}

	s.trashItems[item.ID] = item
	return item, nil
}

// ListTrashItems 列出回收站项目
func (s *Service) ListTrashItems(opts ListOptions) (*PageResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*TrashItem
	for _, item := range s.trashItems {
		if item.Status == TrashStatusActive {
			items = append(items, item)
		}
	}

	// 排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].DeletedAt.After(items[j].DeletedAt)
	})

	// 分页
	total := len(items)
	page := opts.Page
	pageSize := opts.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &PageResult{
		Items:      items[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// RestoreFromTrash 从回收站恢复
func (s *Service) RestoreFromTrash(id string, restorePath string) (*TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.trashItems[id]
	if !ok {
		return nil, fmt.Errorf("回收站项目不存在: %s", id)
	}

	if item.Status != TrashStatusActive {
		return nil, fmt.Errorf("项目状态不正确: %s", item.Status)
	}

	if restorePath == "" {
		restorePath = item.OriginalPath
	}

	item.Status = TrashStatusRestored
	item.RestorePath = restorePath
	now := time.Now()
	item.RestoredAt = &now

	return item, nil
}

// PurgeTrashItem 彻底删除回收站项目
func (s *Service) PurgeTrashItem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.trashItems[id]
	if !ok {
		return fmt.Errorf("回收站项目不存在: %s", id)
	}

	item.Status = TrashStatusPurged
	delete(s.trashItems, id)
	return nil
}

// EmptyTrash 清空回收站
func (s *Service) EmptyTrash() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, item := range s.trashItems {
		if item.Status == TrashStatusActive {
			item.Status = TrashStatusPurged
			delete(s.trashItems, id)
			count++
		}
	}

	return count, nil
}

// ==================== 文件锁定 ====================

// LockFile 锁定文件
func (s *Service) LockFile(filePath string, lockType LockType, lockedBy, reason string, expiresAt *time.Time) (*FileLock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已锁定
	if existingLock, exists := s.locks[filePath]; exists {
		if existingLock.LockType == LockTypeHard && lockType != LockTypeHard {
			return nil, fmt.Errorf("文件已被硬锁锁定: %s", filePath)
		}
	}

	lock := &FileLock{
		ID:        uuid.New().String(),
		FilePath:  filePath,
		LockType:  lockType,
		LockedBy:  lockedBy,
		Reason:    reason,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	s.locks[filePath] = lock
	return lock, nil
}

// UnlockFile 解锁文件
func (s *Service) UnlockFile(filePath string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock, exists := s.locks[filePath]
	if !exists {
		return fmt.Errorf("文件未锁定: %s", filePath)
	}

	if lock.LockType == LockTypeHard && !force {
		return fmt.Errorf("硬锁需要强制解锁: %s", filePath)
	}

	delete(s.locks, filePath)
	return nil
}

// ListLocks 列出文件锁
func (s *Service) ListLocks() []*FileLock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	locks := make([]*FileLock, 0, len(s.locks))
	for _, lock := range s.locks {
		locks = append(locks, lock)
	}
	return locks
}

// IsFileLocked 检查文件是否被锁定
func (s *Service) IsFileLocked(filePath string) (bool, *FileLock) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lock, exists := s.locks[filePath]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if lock.ExpiresAt != nil && lock.ExpiresAt.Before(time.Now()) {
		delete(s.locks, filePath)
		return false, nil
	}

	return true, lock
}

// ==================== 存储分析 ====================

// GetStorageUsage 获取存储使用情况
func (s *Service) GetStorageUsage() (*StorageUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	usage := &StorageUsage{
		ByTier:    make(map[StorageTier]int64),
		ByPath:    make(map[string]int64),
		ByPolicy:  make(map[string]int64),
		UpdatedAt: time.Now(),
	}

	// 计算快照占用
	for _, snap := range s.snapshots {
		usage.SnapshotSpace += snap.TotalSize
	}

	// 计算回收站占用
	for _, item := range s.trashItems {
		if item.Status == TrashStatusActive {
			usage.TrashSpace += item.Size
		}
	}

	// 模拟总空间和使用率
	usage.TotalSpace = 1024 * 1024 * 1024 * 100 // 100GB
	usage.UsedSpace = usage.SnapshotSpace + usage.TrashSpace + usage.ActiveSpace
	usage.AvailableSpace = usage.TotalSpace - usage.UsedSpace
	usage.UsagePercent = float64(usage.UsedSpace) / float64(usage.TotalSpace) * 100

	// 按层级分布
	usage.ByTier[TierHot] = usage.SnapshotSpace / 2
	usage.ByTier[TierWarm] = usage.SnapshotSpace / 3
	usage.ByTier[TierCold] = usage.SnapshotSpace / 6

	return usage, nil
}

// GetSnapshotStorageInfo 获取快照存储详情
func (s *Service) GetSnapshotStorageInfo(snapshotID string) (*SnapshotStorageInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", snapshotID)
	}

	info := &SnapshotStorageInfo{
		SnapshotID:    snap.ID,
		SnapshotName:  snap.Name,
		TotalSize:     snap.TotalSize,
		DeltaSize:     snap.DeltaSize,
		Deduplication: snap.TotalSize / 10, // 模拟去重节省 10%
		Compression:   snap.TotalSize / 5,  // 模拟压缩节省 20%
		EffectiveSize: snap.TotalSize - snap.TotalSize/10 - snap.TotalSize/5,
		Tier:          TierHot,
		CreatedAt:     snap.CreatedAt,
	}

	return info, nil
}

// ==================== 健康检查 ====================

// HealthCheck 健康检查
func (s *Service) HealthCheck() *HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &HealthStatus{
		Status:         "healthy",
		TotalSnapshots: len(s.snapshots),
		TrashItems:     0,
		LockedFiles:    len(s.locks),
		CheckedAt:      time.Now(),
	}

	// 统计活跃快照
	for _, snap := range s.snapshots {
		if snap.Status == SnapshotStatusActive {
			status.ActiveSnapshots++
			if snap.CreatedAt.After(status.LastSnapshot) {
				status.LastSnapshot = snap.CreatedAt
			}
		}
	}

	// 统计回收站
	for _, item := range s.trashItems {
		if item.Status == TrashStatusActive {
			status.TrashItems++
		}
	}

	// 计算存储使用
	usage, _ := s.GetStorageUsage()
	if usage != nil {
		status.StorageUsage = usage.UsedSpace
		if usage.UsagePercent > 90 {
			status.Status = "degraded"
			status.Errors = append(status.Errors, "存储空间不足")
		}
	}

	// 检查策略执行
	for _, policy := range s.policies {
		if policy.NextRunAt != nil && policy.NextRunAt.Before(status.NextScheduled) {
			status.NextScheduled = *policy.NextRunAt
		}
	}

	return status
}

// ==================== 内部方法 ====================

// scanPaths 扫描路径，统计文件数量和大小
func (s *Service) scanPaths(ctx context.Context, paths []string) (int64, int64, error) {
	var totalFiles int64
	var totalSize int64

	for _, path := range paths {
		err := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 检查排除模式
			for _, pattern := range s.config.ExcludePatterns {
				if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if !info.IsDir() {
				totalFiles++
				totalSize += info.Size()
			}

			return nil
		})

		if err != nil {
			return 0, 0, err
		}
	}

	return totalFiles, totalSize, nil
}

// updateVersions 更新文件版本
func (s *Service) updateVersions(snapshot *Snapshot, paths []string) {
	for _, path := range paths {
		version := &FileVersion{
			ID:          uuid.New().String(),
			SnapshotID:  snapshot.ID,
			FilePath:    path,
			FullPath:    path,
			FileType:    FileTypeText,
			Size:        1024,
			ModTime:     snapshot.CreatedAt,
			Permissions: "0644",
			MD5:         s.calculateMD5(path),
			SHA256:      s.calculateSHA256(path),
			CreatedAt:   time.Now(),
		}
		s.versions[path] = append(s.versions[path], version)
	}
}

// generateTextDiff 生成文本差异
func (s *Service) generateTextDiff(oldVer, newVer *FileVersion) []DiffHunk {
	// 简化实现：返回模拟差异
	return []DiffHunk{
		{
			OldStart: 1,
			OldLines: 5,
			NewStart: 1,
			NewLines: 7,
			Content:  "@@ -1,5 +1,7 @@\n context\n-old line\n+new line 1\n+new line 2\n context",
			Lines: []DiffLine{
				{Type: DiffUnchanged, OldLine: 1, NewLine: 1, Content: "context"},
				{Type: DiffDeleted, OldLine: 2, NewLine: 0, Content: "old line"},
				{Type: DiffAdded, OldLine: 0, NewLine: 2, Content: "new line 1"},
				{Type: DiffAdded, OldLine: 0, NewLine: 3, Content: "new line 2"},
				{Type: DiffUnchanged, OldLine: 3, NewLine: 4, Content: "context"},
			},
		},
	}
}

// calculateNextRun 计算下次执行时间
func (s *Service) calculateNextRun(policy *SnapshotPolicy) *time.Time {
	now := time.Now()
	var next time.Time

	switch policy.Schedule {
	case ScheduleMinutely:
		next = now.Add(time.Duration(policy.ScheduleValue) * time.Minute)
	case ScheduleHourly:
		next = now.Add(time.Duration(policy.ScheduleValue) * time.Hour)
	case ScheduleDaily:
		next = now.AddDate(0, 0, policy.ScheduleValue)
	case ScheduleWeekly:
		next = now.AddDate(0, 0, 7*policy.ScheduleValue)
	case ScheduleMonthly:
		next = now.AddDate(0, policy.ScheduleValue, 0)
	default:
		next = now.Add(24 * time.Hour)
	}

	return &next
}

// calculateMD5 计算 MD5
func (s *Service) calculateMD5(data string) string {
	h := md5.New()
	io.WriteString(h, data)
	return hex.EncodeToString(h.Sum(nil))
}

// calculateSHA256 计算 SHA256
func (s *Service) calculateSHA256(data string) string {
	h := sha256.New()
	io.WriteString(h, data)
	return hex.EncodeToString(h.Sum(nil))
}
