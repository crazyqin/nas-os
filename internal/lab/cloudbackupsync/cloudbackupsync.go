// Package cloudbackupsync 提供跨云备份同步功能，支持多目标备份、增量同步、数据去重等。
// 参考群晖 Hyper Backup 设计，支持本地、远程 NAS、S3/OSS/MinIO、WebDAV 等备份目标。
package cloudbackupsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// ========== 错误定义 ==========

var (
	// ErrInvalidConfig 表示配置无效。
	ErrInvalidConfig = errors.New("invalid cloud backup configuration")
	// ErrBackupInProgress 表示备份正在进行中。
	ErrBackupInProgress = errors.New("backup already in progress")
	// ErrBackupNotFound 表示备份记录未找到。
	ErrBackupNotFound = errors.New("backup record not found")
	// ErrTaskNotFound 表示备份任务未找到。
	ErrTaskNotFound = errors.New("backup task not found")
	// ErrTargetNotFound 表示备份目标未找到。
	ErrTargetNotFound = errors.New("backup target not found")
	// ErrRestorePointNotFound 表示恢复点未找到。
	ErrRestorePointNotFound = errors.New("restore point not found")
	// ErrVerificationFailed 表示备份验证失败。
	ErrVerificationFailed = errors.New("backup verification failed")
	// ErrUnsupportedTarget 表示不支持的备份目标类型。
	ErrUnsupportedTarget = errors.New("unsupported backup target type")
)

// ========== 枚举类型 ==========

// TargetType 备份目标类型。
type TargetType string

const (
	// TargetLocal 本地备份。
	TargetLocal TargetType = "local"
	// TargetRemoteNAS 远程NAS。
	TargetRemoteNAS TargetType = "remote_nas"
	// TargetS3 S3/OSS/MinIO 兼容存储。
	TargetS3 TargetType = "s3"
	// TargetWebDAV WebDAV 备份目标。
	TargetWebDAV TargetType = "webdav"
)

// BackupStatus 备份状态。
type BackupStatus string

const (
	// StatusPending 待处理。
	StatusPending BackupStatus = "pending"
	// StatusRunning 运行中。
	StatusRunning BackupStatus = "running"
	// StatusCompleted 已完成。
	StatusCompleted BackupStatus = "completed"
	// StatusFailed 失败。
	StatusFailed BackupStatus = "failed"
	// StatusCancelled 已取消。
	StatusCancelled BackupStatus = "cancelled"
	// StatusVerifying 验证中。
	StatusVerifying BackupStatus = "verifying"
)

// BackupType 备份类型。
type BackupType string

const (
	// TypeFull 完整备份。
	TypeFull BackupType = "full"
	// TypeIncremental 增量备份。
	TypeIncremental BackupType = "incremental"
)

// RetentionType 保留策略类型。
type RetentionType string

const (
	// RetentionByCount 按数量保留。
	RetentionByCount RetentionType = "count"
	// RetentionByAge 按时间保留。
	RetentionByAge RetentionType = "age"
	// RetentionByCombined 组合策略。
	RetentionByCombined RetentionType = "combined"
)

// ========== 类型定义 ==========

// BackupTarget 备份目标配置。
type BackupTarget struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        TargetType        `json:"type"`
	Enabled     bool              `json:"enabled"`
	Config      map[string]string `json:"config"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// RetentionPolicy 保留策略。
type RetentionPolicy struct {
	Type        RetentionType `json:"type"`
	MaxCount    int           `json:"maxCount,omitempty"`
	MaxAgeDays  int           `json:"maxAgeDays,omitempty"`
	KeepDaily   int           `json:"keepDaily,omitempty"`
	KeepWeekly  int           `json:"keepWeekly,omitempty"`
	KeepMonthly int           `json:"keepMonthly,omitempty"`
	KeepYearly  int           `json:"keepYearly,omitempty"`
}

// DefaultRetentionPolicy 默认保留策略。
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		Type:       RetentionByCount,
		MaxCount:   10,
		MaxAgeDays: 90,
	}
}

// ScheduleConfig 备份计划配置。
type ScheduleConfig struct {
	Enabled    bool   `json:"enabled"`
	CronExpr   string `json:"cronExpr"`
	FullBackup string `json:"fullBackup"`
	TimeZone   string `json:"timeZone"`
}

// BlockInfo 块信息。
type BlockInfo struct {
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	RefCount int    `json:"refCount"`
}

// FileManifest 文件清单。
type FileManifest struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
	Checksum string    `json:"checksum"`
	Blocks   []string  `json:"blocks"`
}

// BackupManifest 备份清单。
type BackupManifest struct {
	ID           string            `json:"id"`
	TaskID       string            `json:"taskId"`
	TargetID     string            `json:"targetId"`
	Type         BackupType        `json:"type"`
	BaseBackupID string            `json:"baseBackupId,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	Files        []FileManifest    `json:"files"`
	TotalSize    int64             `json:"totalSize"`
	DedupSize    int64             `json:"dedupSize"`
	Compressed   bool              `json:"compressed"`
	Encrypted    bool              `json:"encrypted"`
	Checksum     string            `json:"checksum"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// BackupTask 备份任务配置。
type BackupTask struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Enabled         bool            `json:"enabled"`
	SourcePath      string          `json:"sourcePath"`
	TargetIDs       []string        `json:"targetIds"`
	Schedule        ScheduleConfig  `json:"schedule"`
	Retention       RetentionPolicy `json:"retention"`
	ExcludePatterns []string        `json:"excludePatterns"`
	EnableDedup     bool            `json:"enableDedup"`
	EnableCompress  bool            `json:"enableCompress"`
	EnableEncrypt   bool            `json:"enableEncrypt"`
	BlockSize       int64           `json:"blockSize"`
	LastFullBackup  time.Time       `json:"lastFullBackup"`
	LastIncrBackup  time.Time       `json:"lastIncrBackup"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// BackupRecord 备份执行记录。
type BackupRecord struct {
	ID               string       `json:"id"`
	TaskID           string       `json:"taskId"`
	TaskName         string       `json:"taskName"`
	TargetID         string       `json:"targetId"`
	TargetName       string       `json:"targetName"`
	Type             BackupType   `json:"type"`
	Status           BackupStatus `json:"status"`
	StartTime        time.Time    `json:"startTime"`
	EndTime          time.Time    `json:"endTime"`
	Duration         int64        `json:"duration"`
	TotalFiles       int64        `json:"totalFiles"`
	TotalSize        int64        `json:"totalSize"`
	DedupedSize      int64        `json:"dedupedSize"`
	TransferredBytes int64        `json:"transferredBytes"`
	Progress         float64      `json:"progress"`
	Error            string       `json:"error,omitempty"`
	Verified         bool         `json:"verified"`
	VerifiedAt       *time.Time   `json:"verifiedAt,omitempty"`
	Checksum         string       `json:"checksum"`
	BaseBackupID     string       `json:"baseBackupId,omitempty"`
}

// RestorePoint 恢复点信息。
type RestorePoint struct {
	RecordID   string     `json:"recordId"`
	TaskID     string     `json:"taskId"`
	TaskName   string     `json:"taskName"`
	TargetID   string     `json:"targetId"`
	TargetName string     `json:"targetName"`
	Type       BackupType `json:"type"`
	CreatedAt  time.Time  `json:"createdAt"`
	TotalSize  int64      `json:"totalSize"`
	FileCount  int64      `json:"fileCount"`
	Verified   bool       `json:"verified"`
	Checksum   string     `json:"checksum"`
}

// VerificationResult 验证结果。
type VerificationResult struct {
	RecordID     string    `json:"recordId"`
	Valid        bool      `json:"valid"`
	TotalFiles   int64     `json:"totalFiles"`
	CheckedFiles int64     `json:"checkedFiles"`
	FailedFiles  []string  `json:"failedFiles,omitempty"`
	Duration     int64     `json:"duration"`
	CheckedAt    time.Time `json:"checkedAt"`
}

// DedupStats 去重统计。
type DedupStats struct {
	TotalBlocks     int64   `json:"totalBlocks"`
	UniqueBlocks    int64   `json:"uniqueBlocks"`
	DuplicateBlocks int64   `json:"duplicateBlocks"`
	OriginalSize    int64   `json:"originalSize"`
	DedupedSize     int64   `json:"dedupedSize"`
	SavedSize       int64   `json:"savedSize"`
	DedupRatio      float64 `json:"dedupRatio"`
}

// SyncStatus 同步状态概览。
type SyncStatus struct {
	TotalTasks        int         `json:"totalTasks"`
	EnabledTasks      int         `json:"enabledTasks"`
	RunningBackups    int         `json:"runningBackups"`
	TotalRecords      int         `json:"totalRecords"`
	TotalTargets      int         `json:"totalTargets"`
	DedupStats        *DedupStats `json:"dedupStats"`
	LastBackupTime    *time.Time  `json:"lastBackupTime,omitempty"`
	StorageUsed       int64       `json:"storageUsed"`
	FailedBackups     int         `json:"failedBackups"`
	SuccessfulBackups int         `json:"successfulBackups"`
}

// ========== 主结构 ==========

// CloudBackupSync 跨云备份同步管理器。
type CloudBackupSync struct {
	mu          sync.RWMutex
	tasks       map[string]*BackupTask
	targets     map[string]*BackupTarget
	records     map[string]*BackupRecord
	manifests   map[string]*BackupManifest
	blockIndex  map[string]*BlockInfo
	dedupStats  *DedupStats
	cronEngine  *cron.Cron
	cronJobs    map[string]cron.EntryID
	storagePath string
	activeJobs  map[string]context.CancelFunc
	initialized bool
}

// NewCloudBackupSync 创建跨云备份同步管理器。
func NewCloudBackupSync(storagePath string) (*CloudBackupSync, error) {
	if storagePath == "" {
		return nil, ErrInvalidConfig
	}

	if err := os.MkdirAll(storagePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage path: %w", err)
	}

	for _, dir := range []string{"tasks", "targets", "records", "manifests", "blocks", "backups"} {
		if err := os.MkdirAll(filepath.Join(storagePath, dir), 0750); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	cbs := &CloudBackupSync{
		tasks:       make(map[string]*BackupTask),
		targets:     make(map[string]*BackupTarget),
		records:     make(map[string]*BackupRecord),
		manifests:   make(map[string]*BackupManifest),
		blockIndex:  make(map[string]*BlockInfo),
		dedupStats:  &DedupStats{},
		cronJobs:    make(map[string]cron.EntryID),
		storagePath: storagePath,
		activeJobs:  make(map[string]context.CancelFunc),
		initialized: true,
	}

	cbs.cronEngine = cron.New(cron.WithLocation(time.Local))

	if err := cbs.loadFromDisk(); err != nil {
		log.Printf("Warning: failed to load backup data: %v", err)
	}

	cbs.cronEngine.Start()
	return cbs, nil
}

// Close 关闭管理器，释放资源。
func (cbs *CloudBackupSync) Close() error {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	for id, cancel := range cbs.activeJobs {
		cancel()
		delete(cbs.activeJobs, id)
	}

	if cbs.cronEngine != nil {
		ctx := cbs.cronEngine.Stop()
		<-ctx.Done()
	}

	return cbs.saveToDisk()
}

// ========== 备份目标管理 ==========

// CreateTarget 创建备份目标。
func (cbs *CloudBackupSync) CreateTarget(target *BackupTarget) error {
	if target == nil || target.Name == "" || target.Type == "" {
		return ErrInvalidConfig
	}
	switch target.Type {
	case TargetLocal, TargetRemoteNAS, TargetS3, TargetWebDAV:
	default:
		return ErrUnsupportedTarget
	}

	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	if target.ID == "" {
		target.ID = generateID("target")
	}
	now := time.Now()
	target.CreatedAt = now
	target.UpdatedAt = now
	cbs.targets[target.ID] = target
	return cbs.saveTarget(target)
}

// UpdateTarget 更新备份目标。
func (cbs *CloudBackupSync) UpdateTarget(target *BackupTarget) error {
	if target == nil || target.ID == "" {
		return ErrInvalidConfig
	}
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	existing, exists := cbs.targets[target.ID]
	if !exists {
		return ErrTargetNotFound
	}
	target.CreatedAt = existing.CreatedAt
	target.UpdatedAt = time.Now()
	cbs.targets[target.ID] = target
	return cbs.saveTarget(target)
}

// DeleteTarget 删除备份目标。
func (cbs *CloudBackupSync) DeleteTarget(targetID string) error {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	if _, exists := cbs.targets[targetID]; !exists {
		return ErrTargetNotFound
	}
	for _, task := range cbs.tasks {
		for _, tid := range task.TargetIDs {
			if tid == targetID {
				return fmt.Errorf("target is in use by task %s", task.ID)
			}
		}
	}
	delete(cbs.targets, targetID)
	return os.Remove(filepath.Join(cbs.storagePath, "targets", targetID+".json"))
}

// GetTarget 获取备份目标。
func (cbs *CloudBackupSync) GetTarget(targetID string) (*BackupTarget, error) {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	target, exists := cbs.targets[targetID]
	if !exists {
		return nil, ErrTargetNotFound
	}
	return target, nil
}

// ListTargets 列出所有备份目标。
func (cbs *CloudBackupSync) ListTargets() []*BackupTarget {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	targets := make([]*BackupTarget, 0, len(cbs.targets))
	for _, t := range cbs.targets {
		targets = append(targets, t)
	}
	return targets
}

// ========== 备份任务管理 ==========

// CreateTask 创建备份任务。
func (cbs *CloudBackupSync) CreateTask(task *BackupTask) error {
	if task == nil || task.Name == "" || task.SourcePath == "" || len(task.TargetIDs) == 0 {
		return ErrInvalidConfig
	}

	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	for _, tid := range task.TargetIDs {
		if _, exists := cbs.targets[tid]; !exists {
			return fmt.Errorf("%w: %s", ErrTargetNotFound, tid)
		}
	}

	if task.ID == "" {
		task.ID = generateID("task")
	}
	if task.BlockSize <= 0 {
		task.BlockSize = 64 * 1024 // 默认64KB
	}
	if task.Retention.Type == "" {
		task.Retention = DefaultRetentionPolicy()
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	cbs.tasks[task.ID] = task

	if task.Schedule.Enabled && task.Schedule.CronExpr != "" {
		if err := cbs.scheduleTask(task); err != nil {
			log.Printf("Warning: failed to schedule task %s: %v", task.ID, err)
		}
	}
	return cbs.saveTask(task)
}

// UpdateTask 更新备份任务。
func (cbs *CloudBackupSync) UpdateTask(task *BackupTask) error {
	if task == nil || task.ID == "" {
		return ErrInvalidConfig
	}
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	existing, exists := cbs.tasks[task.ID]
	if !exists {
		return ErrTaskNotFound
	}
	task.CreatedAt = existing.CreatedAt
	task.UpdatedAt = time.Now()
	cbs.tasks[task.ID] = task

	if entryID, ok := cbs.cronJobs[task.ID]; ok {
		cbs.cronEngine.Remove(entryID)
		delete(cbs.cronJobs, task.ID)
	}
	if task.Schedule.Enabled && task.Schedule.CronExpr != "" {
		if err := cbs.scheduleTask(task); err != nil {
			log.Printf("Warning: failed to schedule task %s: %v", task.ID, err)
		}
	}
	return cbs.saveTask(task)
}

// DeleteTask 删除备份任务。
func (cbs *CloudBackupSync) DeleteTask(taskID string) error {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	if _, exists := cbs.tasks[taskID]; !exists {
		return ErrTaskNotFound
	}
	if entryID, ok := cbs.cronJobs[taskID]; ok {
		cbs.cronEngine.Remove(entryID)
		delete(cbs.cronJobs, taskID)
	}
	delete(cbs.tasks, taskID)
	return os.Remove(filepath.Join(cbs.storagePath, "tasks", taskID+".json"))
}

// GetTask 获取备份任务。
func (cbs *CloudBackupSync) GetTask(taskID string) (*BackupTask, error) {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	task, exists := cbs.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有备份任务。
func (cbs *CloudBackupSync) ListTasks() []*BackupTask {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	tasks := make([]*BackupTask, 0, len(cbs.tasks))
	for _, t := range cbs.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// ========== 备份执行 ==========

// RunBackup 执行备份任务。
func (cbs *CloudBackupSync) RunBackup(ctx context.Context, taskID string, backupType BackupType) ([]*BackupRecord, error) {
	cbs.mu.RLock()
	task, exists := cbs.tasks[taskID]
	if !exists {
		cbs.mu.RUnlock()
		return nil, ErrTaskNotFound
	}
	cbs.mu.RUnlock()

	cbs.mu.RLock()
	for _, record := range cbs.records {
		if record.TaskID == taskID && record.Status == StatusRunning {
			cbs.mu.RUnlock()
			return nil, ErrBackupInProgress
		}
	}
	cbs.mu.RUnlock()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cbs.mu.Lock()
	cbs.activeJobs[taskID] = cancel
	cbs.mu.Unlock()

	defer func() {
		cbs.mu.Lock()
		delete(cbs.activeJobs, taskID)
		cbs.mu.Unlock()
	}()

	var records []*BackupRecord
	for _, targetID := range task.TargetIDs {
		record, err := cbs.executeBackup(ctx, task, targetID, backupType)
		if err != nil {
			log.Printf("Error: backup failed for task %s to target %s: %v", taskID, targetID, err)
			if record != nil {
				record.Status = StatusFailed
				record.Error = err.Error()
				cbs.mu.Lock()
				cbs.records[record.ID] = record
				cbs.mu.Unlock()
			}
			records = append(records, record)
			continue
		}
		records = append(records, record)
	}

	cbs.applyRetentionPolicy(taskID)
	_ = cbs.saveToDisk()
	return records, nil
}

// CancelBackup 取消正在执行的备份。
func (cbs *CloudBackupSync) CancelBackup(taskID string) error {
	cbs.mu.RLock()
	cancel, exists := cbs.activeJobs[taskID]
	cbs.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no active backup for task %s", taskID)
	}
	cancel()
	return nil
}

// executeBackup 执行单目标备份。
func (cbs *CloudBackupSync) executeBackup(ctx context.Context, task *BackupTask, targetID string, backupType BackupType) (*BackupRecord, error) {
	recordID := generateID("record")
	record := &BackupRecord{
		ID: recordID, TaskID: task.ID, TaskName: task.Name,
		TargetID: targetID, Status: StatusRunning, Type: backupType, StartTime: time.Now(),
	}

	cbs.mu.RLock()
	target, exists := cbs.targets[targetID]
	if !exists {
		cbs.mu.RUnlock()
		return record, ErrTargetNotFound
	}
	record.TargetName = target.Name
	cbs.mu.RUnlock()

	cbs.mu.Lock()
	cbs.records[recordID] = record
	cbs.mu.Unlock()

	files, err := cbs.scanSource(task)
	if err != nil {
		return record, fmt.Errorf("failed to scan source: %w", err)
	}
	record.TotalFiles = int64(len(files))

	manifest := &BackupManifest{
		ID: recordID, TaskID: task.ID, TargetID: targetID,
		Type: backupType, CreatedAt: time.Now(),
		Files: make([]FileManifest, 0, len(files)), Metadata: make(map[string]string),
	}

	if backupType == TypeIncremental {
		baseID := cbs.findLastFullBackup(task.ID, targetID)
		if baseID != "" {
			manifest.BaseBackupID = baseID
			record.BaseBackupID = baseID
		} else {
			backupType = TypeFull
			manifest.Type = TypeFull
			record.Type = TypeFull
		}
	}

	var totalSize, dedupedSize int64

	for i, filePath := range files {
		select {
		case <-ctx.Done():
			record.Status = StatusCancelled
			record.Error = "backup cancelled"
			return record, ctx.Err()
		default:
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Warning: failed to read file %s: %v", filePath, err)
			continue
		}

		relPath, _ := filepath.Rel(task.SourcePath, filePath)
		fileSize := int64(len(data))
		totalSize += fileSize
		fileChecksum := sha256Sum(data)

		// 增量备份：检查文件是否变更
		if backupType == TypeIncremental {
			cbs.mu.RLock()
			oldManifest, exists := cbs.manifests[manifest.BaseBackupID]
			cbs.mu.RUnlock()
			if exists {
				changed := true
				for _, f := range oldManifest.Files {
					if f.Path == relPath && f.Checksum == fileChecksum {
						changed = false
						break
					}
				}
				if !changed {
					continue
				}
			}
		}

		// 块级处理和去重
		var blockHashes []string
		var fileDedupSize int64

		if task.EnableDedup {
			blocks := splitIntoBlocks(data, task.BlockSize)
			for _, block := range blocks {
				blockHash := sha256Sum(block)
				blockHashes = append(blockHashes, blockHash)
				cbs.mu.Lock()
				if existing, exists := cbs.blockIndex[blockHash]; exists {
					existing.RefCount++
				} else {
					cbs.blockIndex[blockHash] = &BlockInfo{
						Hash: blockHash, Size: int64(len(block)), RefCount: 1,
					}
					fileDedupSize += int64(len(block))
					_ = cbs.saveBlock(blockHash, block)
				}
				cbs.mu.Unlock()
			}
			dedupedSize += fileDedupSize
		} else {
			dedupedSize += fileSize
			blockHashes = []string{fileChecksum}
			_ = cbs.saveBlock(fileChecksum, data)
		}

		manifest.Files = append(manifest.Files, FileManifest{
			Path: relPath, Size: fileSize, ModTime: time.Now(),
			Checksum: fileChecksum, Blocks: blockHashes,
		})

		cbs.mu.Lock()
		record.Progress = float64(i+1) / float64(len(files)) * 100
		record.TransferredBytes = dedupedSize
		cbs.mu.Unlock()
	}

	endTime := time.Now()
	manifest.TotalSize = totalSize
	manifest.DedupSize = dedupedSize
	manifest.Checksum = cbs.calculateManifestChecksum(manifest)

	record.EndTime = endTime
	record.Duration = int64(endTime.Sub(record.StartTime).Seconds())
	record.TotalSize = totalSize
	record.DedupedSize = dedupedSize
	record.Checksum = manifest.Checksum
	record.Progress = 100
	record.Status = StatusCompleted

	cbs.mu.Lock()
	cbs.records[recordID] = record
	cbs.manifests[recordID] = manifest
	if backupType == TypeFull {
		task.LastFullBackup = endTime
	} else {
		task.LastIncrBackup = endTime
	}
	cbs.mu.Unlock()

	return record, nil
}

// ========== 备份验证 ==========

// VerifyBackup 验证备份完整性。
func (cbs *CloudBackupSync) VerifyBackup(recordID string) (*VerificationResult, error) {
	startTime := time.Now()
	result := &VerificationResult{RecordID: recordID, CheckedAt: startTime}

	cbs.mu.RLock()
	_, exists := cbs.records[recordID]
	manifest, manifestExists := cbs.manifests[recordID]
	cbs.mu.RUnlock()

	if !exists || !manifestExists {
		return result, ErrBackupNotFound
	}

	result.TotalFiles = int64(len(manifest.Files))
	var failedFiles []string
	var checkedFiles int64

	for _, file := range manifest.Files {
		valid := true
		for _, blockHash := range file.Blocks {
			cbs.mu.RLock()
			_, blockExists := cbs.blockIndex[blockHash]
			cbs.mu.RUnlock()
			if !blockExists {
				valid = false
				break
			}
			blockPath := filepath.Join(cbs.storagePath, "blocks", blockHash[:2], blockHash)
			if _, err := os.Stat(blockPath); err != nil {
				valid = false
				break
			}
		}
		if valid {
			checkedFiles++
		} else {
			failedFiles = append(failedFiles, file.Path)
		}
	}

	result.CheckedFiles = checkedFiles
	result.FailedFiles = failedFiles
	result.Duration = time.Since(startTime).Milliseconds()
	result.Valid = len(failedFiles) == 0

	cbs.mu.Lock()
	if record, ok := cbs.records[recordID]; ok {
		record.Verified = result.Valid
		now := time.Now()
		record.VerifiedAt = &now
	}
	cbs.mu.Unlock()

	return result, nil
}

// ========== 恢复管理 ==========

// ListRestorePoints 列出可用的恢复点。
func (cbs *CloudBackupSync) ListRestorePoints(taskID string) ([]*RestorePoint, error) {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	var points []*RestorePoint
	for _, record := range cbs.records {
		if record.TaskID == taskID && record.Status == StatusCompleted {
			points = append(points, &RestorePoint{
				RecordID: record.ID, TaskID: record.TaskID, TaskName: record.TaskName,
				TargetID: record.TargetID, TargetName: record.TargetName, Type: record.Type,
				CreatedAt: record.EndTime, TotalSize: record.TotalSize,
				FileCount: record.TotalFiles, Verified: record.Verified, Checksum: record.Checksum,
			})
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].CreatedAt.After(points[j].CreatedAt) })
	return points, nil
}

// RestoreFromPoint 从恢复点恢复数据。
func (cbs *CloudBackupSync) RestoreFromPoint(ctx context.Context, recordID string, targetPath string, overwrite bool) error {
	cbs.mu.RLock()
	manifest, exists := cbs.manifests[recordID]
	cbs.mu.RUnlock()

	if !exists {
		return ErrRestorePointNotFound
	}

	if manifest.Type == TypeIncremental && manifest.BaseBackupID != "" {
		if err := cbs.restoreBaseBackup(ctx, manifest.BaseBackupID, targetPath, overwrite); err != nil {
			return fmt.Errorf("failed to restore base backup: %w", err)
		}
	}
	return cbs.restoreManifest(ctx, manifest, targetPath, overwrite)
}

func (cbs *CloudBackupSync) restoreBaseBackup(ctx context.Context, recordID string, targetPath string, overwrite bool) error {
	cbs.mu.RLock()
	manifest, exists := cbs.manifests[recordID]
	cbs.mu.RUnlock()

	if !exists {
		return fmt.Errorf("base backup manifest not found: %s", recordID)
	}
	if manifest.Type == TypeIncremental && manifest.BaseBackupID != "" {
		if err := cbs.restoreBaseBackup(ctx, manifest.BaseBackupID, targetPath, overwrite); err != nil {
			return err
		}
	}
	return cbs.restoreManifest(ctx, manifest, targetPath, overwrite)
}

func (cbs *CloudBackupSync) restoreManifest(ctx context.Context, manifest *BackupManifest, targetPath string, overwrite bool) error {
	for _, file := range manifest.Files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		dstPath := filepath.Join(targetPath, file.Path)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
			continue
		}
		if !overwrite {
			if _, err := os.Stat(dstPath); err == nil {
				continue
			}
		}

		var fileData []byte
		for _, blockHash := range file.Blocks {
			cbs.mu.RLock()
			_, exists := cbs.blockIndex[blockHash]
			cbs.mu.RUnlock()
			if !exists {
				return fmt.Errorf("block not found: %s", blockHash)
			}
			blockData, err := cbs.loadBlock(blockHash)
			if err != nil {
				return fmt.Errorf("failed to load block %s: %w", blockHash, err)
			}
			fileData = append(fileData, blockData...)
		}

		if err := os.WriteFile(dstPath, fileData, 0600); err != nil {
			log.Printf("Warning: failed to write file %s: %v", dstPath, err)
		}
	}
	return nil
}

// ========== 状态查询 ==========

// GetSyncStatus 获取同步状态概览。
func (cbs *CloudBackupSync) GetSyncStatus() *SyncStatus {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	status := &SyncStatus{
		TotalTasks:   len(cbs.tasks),
		TotalRecords: len(cbs.records),
		TotalTargets: len(cbs.targets),
		DedupStats:   cbs.dedupStats,
	}

	var lastBackupTime *time.Time
	var storageUsed int64

	for _, task := range cbs.tasks {
		if task.Enabled {
			status.EnabledTasks++
		}
	}
	for _, record := range cbs.records {
		switch record.Status {
		case StatusRunning:
			status.RunningBackups++
		case StatusFailed:
			status.FailedBackups++
		case StatusCompleted:
			status.SuccessfulBackups++
		}
		if lastBackupTime == nil || record.EndTime.After(*lastBackupTime) {
			lastBackupTime = &record.EndTime
		}
		storageUsed += record.TotalSize
	}

	status.LastBackupTime = lastBackupTime
	status.StorageUsed = storageUsed
	return status
}

// GetDedupStats 获取去重统计。
func (cbs *CloudBackupSync) GetDedupStats() *DedupStats {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	stats := *cbs.dedupStats
	stats.TotalBlocks = int64(len(cbs.blockIndex))
	stats.UniqueBlocks = stats.TotalBlocks

	var totalSize int64
	for _, block := range cbs.blockIndex {
		totalSize += block.Size
		if block.RefCount > 1 {
			stats.DuplicateBlocks += int64(block.RefCount - 1)
		}
	}
	stats.DedupedSize = totalSize
	if stats.OriginalSize > 0 {
		stats.DedupRatio = float64(stats.SavedSize) / float64(stats.OriginalSize) * 100
	}
	return &stats
}

// GetBackupRecord 获取备份记录。
func (cbs *CloudBackupSync) GetBackupRecord(recordID string) (*BackupRecord, error) {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	record, exists := cbs.records[recordID]
	if !exists {
		return nil, ErrBackupNotFound
	}
	return record, nil
}

// ListBackupRecords 列出备份记录。
func (cbs *CloudBackupSync) ListBackupRecords(taskID string) []*BackupRecord {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	var records []*BackupRecord
	for _, record := range cbs.records {
		if taskID == "" || record.TaskID == taskID {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartTime.After(records[j].StartTime) })
	return records
}

// ========== 内部方法 ==========

func (cbs *CloudBackupSync) scanSource(task *BackupTask) ([]string, error) {
	var files []string
	err := filepath.Walk(task.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(task.SourcePath, path)
		for _, pattern := range task.ExcludePatterns {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched || strings.Contains(relPath, pattern) {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func (cbs *CloudBackupSync) findLastFullBackup(taskID, targetID string) string {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()

	var lastFull *BackupRecord
	for _, record := range cbs.records {
		if record.TaskID == taskID && record.TargetID == targetID &&
			record.Type == TypeFull && record.Status == StatusCompleted {
			if lastFull == nil || record.EndTime.After(lastFull.EndTime) {
				lastFull = record
			}
		}
	}
	if lastFull != nil {
		return lastFull.ID
	}
	return ""
}

func (cbs *CloudBackupSync) applyRetentionPolicy(taskID string) {
	cbs.mu.RLock()
	task, exists := cbs.tasks[taskID]
	cbs.mu.RUnlock()
	if !exists {
		return
	}

	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	var taskRecords []*BackupRecord
	for _, record := range cbs.records {
		if record.TaskID == taskID && record.Status == StatusCompleted {
			taskRecords = append(taskRecords, record)
		}
	}
	sort.Slice(taskRecords, func(i, j int) bool { return taskRecords[i].EndTime.After(taskRecords[j].EndTime) })

	switch task.Retention.Type {
	case RetentionByCount:
		if task.Retention.MaxCount > 0 && len(taskRecords) > task.Retention.MaxCount {
			for _, record := range taskRecords[task.Retention.MaxCount:] {
				cbs.deleteRecord(record.ID)
			}
		}
	case RetentionByAge:
		if task.Retention.MaxAgeDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -task.Retention.MaxAgeDays)
			for _, record := range taskRecords {
				if record.EndTime.Before(cutoff) {
					cbs.deleteRecord(record.ID)
				}
			}
		}
	}
}

func (cbs *CloudBackupSync) deleteRecord(recordID string) {
	if manifest, exists := cbs.manifests[recordID]; exists {
		for _, file := range manifest.Files {
			for _, blockHash := range file.Blocks {
				if info, ok := cbs.blockIndex[blockHash]; ok {
					info.RefCount--
					if info.RefCount <= 0 {
						delete(cbs.blockIndex, blockHash)
						blockPath := filepath.Join(cbs.storagePath, "blocks", blockHash[:2], blockHash)
						os.Remove(blockPath)
					}
				}
			}
		}
		delete(cbs.manifests, recordID)
	}
	delete(cbs.records, recordID)
	os.Remove(filepath.Join(cbs.storagePath, "records", recordID+".json"))
	os.Remove(filepath.Join(cbs.storagePath, "manifests", recordID+".json"))
}

func (cbs *CloudBackupSync) calculateManifestChecksum(manifest *BackupManifest) string {
	var checksums []string
	for _, file := range manifest.Files {
		checksums = append(checksums, file.Checksum)
	}
	data := strings.Join(checksums, "|")
	return sha256Sum([]byte(data))
}

func sha256Sum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func splitIntoBlocks(data []byte, blockSize int64) [][]byte {
	var blocks [][]byte
	for i := 0; i < len(data); i += int(blockSize) {
		end := i + int(blockSize)
		if end > len(data) {
			end = len(data)
		}
		blocks = append(blocks, data[i:end])
	}
	return blocks
}

func generateID(prefix string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())))
	return prefix + "-" + hex.EncodeToString(hash[:8])
}

func (cbs *CloudBackupSync) scheduleTask(task *BackupTask) error {
	entryID, err := cbs.cronEngine.AddFunc(task.Schedule.CronExpr, func() {
		cbs.RunBackup(context.Background(), task.ID, TypeIncremental)
	})
	if err != nil {
		return err
	}
	cbs.cronJobs[task.ID] = entryID
	return nil
}

// ========== 持久化 ==========

func (cbs *CloudBackupSync) saveToDisk() error {
	for _, target := range cbs.targets {
		if err := cbs.saveTarget(target); err != nil {
			log.Printf("Warning: failed to save target %s: %v", target.ID, err)
		}
	}
	for _, task := range cbs.tasks {
		if err := cbs.saveTask(task); err != nil {
			log.Printf("Warning: failed to save task %s: %v", task.ID, err)
		}
	}
	for _, record := range cbs.records {
		if err := cbs.saveRecord(record); err != nil {
			log.Printf("Warning: failed to save record %s: %v", record.ID, err)
		}
	}
	for _, manifest := range cbs.manifests {
		if err := cbs.saveManifest(manifest); err != nil {
			log.Printf("Warning: failed to save manifest %s: %v", manifest.ID, err)
		}
	}
	return nil
}

func (cbs *CloudBackupSync) saveTarget(target *BackupTarget) error {
	data, err := json.MarshalIndent(target, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cbs.storagePath, "targets", target.ID+".json"), data, 0640)
}

func (cbs *CloudBackupSync) saveTask(task *BackupTask) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cbs.storagePath, "tasks", task.ID+".json"), data, 0640)
}

func (cbs *CloudBackupSync) saveRecord(record *BackupRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cbs.storagePath, "records", record.ID+".json"), data, 0640)
}

func (cbs *CloudBackupSync) saveManifest(manifest *BackupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cbs.storagePath, "manifests", manifest.ID+".json"), data, 0640)
}

func (cbs *CloudBackupSync) saveBlock(hash string, data []byte) error {
	blockDir := filepath.Join(cbs.storagePath, "blocks", hash[:2])
	if err := os.MkdirAll(blockDir, 0750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(blockDir, hash), data, 0640)
}

func (cbs *CloudBackupSync) loadBlock(hash string) ([]byte, error) {
	return os.ReadFile(filepath.Join(cbs.storagePath, "blocks", hash[:2], hash))
}

func (cbs *CloudBackupSync) loadFromDisk() error {
	// 加载备份目标
	targetsDir := filepath.Join(cbs.storagePath, "targets")
	entries, err := os.ReadDir(targetsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(targetsDir, entry.Name()))
			if err != nil {
				continue
			}
			var target BackupTarget
			if err := json.Unmarshal(data, &target); err == nil {
				cbs.targets[target.ID] = &target
			}
		}
	}

	// 加载备份任务
	tasksDir := filepath.Join(cbs.storagePath, "tasks")
	entries, err = os.ReadDir(tasksDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(tasksDir, entry.Name()))
			if err != nil {
				continue
			}
			var task BackupTask
			if err := json.Unmarshal(data, &task); err == nil {
				cbs.tasks[task.ID] = &task
				if task.Schedule.Enabled && task.Schedule.CronExpr != "" {
					_ = cbs.scheduleTask(&task)
				}
			}
		}
	}

	// 加载备份记录
	recordsDir := filepath.Join(cbs.storagePath, "records")
	entries, err = os.ReadDir(recordsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(recordsDir, entry.Name()))
			if err != nil {
				continue
			}
			var record BackupRecord
			if err := json.Unmarshal(data, &record); err == nil {
				cbs.records[record.ID] = &record
			}
		}
	}

	// 加载备份清单
	manifestsDir := filepath.Join(cbs.storagePath, "manifests")
	entries, err = os.ReadDir(manifestsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(manifestsDir, entry.Name()))
			if err != nil {
				continue
			}
			var manifest BackupManifest
			if err := json.Unmarshal(data, &manifest); err == nil {
				cbs.manifests[manifest.ID] = &manifest
			}
		}
	}

	return nil
}

// ========== API 路由注册 ==========

// RegisterRoutes 注册 API 路由。
func RegisterRoutes(rg *gin.RouterGroup, cbs *CloudBackupSync) {
	api := rg.Group("/cloud-backup-sync")

	// 同步状态
	api.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": cbs.GetSyncStatus()})
	})

	api.GET("/dedup-stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"dedupStats": cbs.GetDedupStats()})
	})

	// 备份目标管理
	api.POST("/targets", func(c *gin.Context) {
		var target BackupTarget
		if err := c.ShouldBindJSON(&target); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := cbs.CreateTarget(&target); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"target": target})
	})

	api.GET("/targets", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"targets": cbs.ListTargets()})
	})

	api.GET("/targets/:id", func(c *gin.Context) {
		target, err := cbs.GetTarget(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"target": target})
	})

	api.PUT("/targets/:id", func(c *gin.Context) {
		var target BackupTarget
		if err := c.ShouldBindJSON(&target); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		target.ID = c.Param("id")
		if err := cbs.UpdateTarget(&target); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"target": target})
	})

	api.DELETE("/targets/:id", func(c *gin.Context) {
		if err := cbs.DeleteTarget(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "target deleted"})
	})

	// 备份任务管理
	api.POST("/tasks", func(c *gin.Context) {
		var task BackupTask
		if err := c.ShouldBindJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := cbs.CreateTask(&task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"task": task})
	})

	api.GET("/tasks", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tasks": cbs.ListTasks()})
	})

	api.GET("/tasks/:id", func(c *gin.Context) {
		task, err := cbs.GetTask(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"task": task})
	})

	api.PUT("/tasks/:id", func(c *gin.Context) {
		var task BackupTask
		if err := c.ShouldBindJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		task.ID = c.Param("id")
		if err := cbs.UpdateTask(&task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"task": task})
	})

	api.DELETE("/tasks/:id", func(c *gin.Context) {
		if err := cbs.DeleteTask(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
	})

	// 备份执行
	api.POST("/tasks/:id/run", func(c *gin.Context) {
		backupType := TypeFull
		if c.Query("type") == "incremental" {
			backupType = TypeIncremental
		}
		records, err := cbs.RunBackup(c.Request.Context(), c.Param("id"), backupType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"records": records})
	})

	api.POST("/tasks/:id/cancel", func(c *gin.Context) {
		if err := cbs.CancelBackup(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "backup cancelled"})
	})

	// 备份记录
	api.GET("/records", func(c *gin.Context) {
		taskID := c.Query("taskId")
		c.JSON(http.StatusOK, gin.H{"records": cbs.ListBackupRecords(taskID)})
	})

	api.GET("/records/:id", func(c *gin.Context) {
		record, err := cbs.GetBackupRecord(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"record": record})
	})

	// 备份验证
	api.POST("/records/:id/verify", func(c *gin.Context) {
		result, err := cbs.VerifyBackup(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"verification": result})
	})

	// 恢复点管理
	api.GET("/tasks/:id/restore-points", func(c *gin.Context) {
		points, err := cbs.ListRestorePoints(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"restorePoints": points})
	})

	api.POST("/records/:id/restore", func(c *gin.Context) {
		var req struct {
			TargetPath string `json:"targetPath"`
			Overwrite  bool   `json:"overwrite"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := cbs.RestoreFromPoint(c.Request.Context(), c.Param("id"), req.TargetPath, req.Overwrite); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "restore completed"})
	})
}
