// Package active 备份恢复模块
// 提供单文件恢复和整机恢复接口
package active

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RestoreMode 恢复模式
type RestoreMode string

const (
	RestoreModeSingleFile RestoreMode = "single_file" // 单文件恢复
	RestoreModeFullImage  RestoreMode = "full_image"  // 整机恢复
	RestoreModeVolume     RestoreMode = "volume"      // 卷恢复
	RestoreModeDirectory  RestoreMode = "directory"    // 目录恢复
)

// RestoreStatus 恢复任务状态
type RestoreStatus string

const (
	RestoreStatusPending   RestoreStatus = "pending"
	RestoreStatusRunning   RestoreStatus = "running"
	RestoreStatusCompleted RestoreStatus = "completed"
	RestoreStatusFailed    RestoreStatus = "failed"
	RestoreStatusCancelled RestoreStatus = "cancelled"
)

// RestoreTask 恢复任务
type RestoreTask struct {
	ID            string        `json:"id"`
	JobID         string        `json:"job_id"`         // 源备份任务 ID
	SnapshotID    string        `json:"snapshot_id"`    // 恢复目标快照
	Mode          RestoreMode   `json:"mode"`           // 恢复模式
	TargetPath    string        `json:"target_path"`    // 恢复目标路径
	Files         []string      `json:"files"`          // 单文件恢复时的文件列表
	Status        RestoreStatus `json:"status"`
	Progress      float64       `json:"progress"`       // 0.0 ~ 1.0
	FilesRestored int           `json:"files_restored"`
	TotalFiles    int           `json:"total_files"`
	BytesRestored int64         `json:"bytes_restored"`
	Error         string        `json:"error,omitempty"`
	Options       RestoreExecOptions `json:"options"`
	CreatedAt     time.Time     `json:"created_at"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
}

// RestoreExecOptions 恢复执行选项
type RestoreExecOptions struct {
	OverwriteExisting bool `json:"overwrite_existing"` // 覆盖已存在的文件
	RestoreACL        bool `json:"restore_acl"`        // 恢复 ACL 权限
	RestoreTimestamps bool `json:"restore_timestamps"` // 恢复时间戳
	VerifyAfterRestore bool `json:"verify_after_restore"` // 恢复后验证
	DryRun            bool `json:"dry_run"`            // 试运行（不实际写入）
}

// RestoreManager 恢复管理器
type RestoreManager struct {
	mu       sync.RWMutex
	manager  *BackupManager
	tasks    map[string]*RestoreTask
	logger   *zap.Logger
}

// NewRestoreManager 创建恢复管理器
func NewRestoreManager(manager *BackupManager, logger *zap.Logger) *RestoreManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RestoreManager{
		manager: manager,
		tasks:   make(map[string]*RestoreTask),
		logger:  logger,
	}
}

// CreateSingleFileRestore 创建单文件恢复任务
func (rm *RestoreManager) CreateSingleFileRestore(ctx context.Context, jobID, snapshotID string, files []string, targetPath string, opts RestoreExecOptions) (*RestoreTask, error) {
	// 验证快照存在
	if _, err := rm.manager.GetSnapshot(snapshotID); err != nil {
		return nil, fmt.Errorf("快照 %s 不存在: %w", snapshotID, err)
	}

	task := &RestoreTask{
		ID:         uuid.New().String(),
		JobID:      jobID,
		SnapshotID: snapshotID,
		Mode:       RestoreModeSingleFile,
		TargetPath: targetPath,
		Files:      files,
		Status:     RestoreStatusPending,
		Options:    opts,
		CreatedAt:  time.Now(),
	}

	rm.mu.Lock()
	rm.tasks[task.ID] = task
	rm.mu.Unlock()

	rm.logger.Info("单文件恢复任务创建",
		zap.String("task_id", task.ID),
		zap.Int("file_count", len(files)),
		zap.String("target", targetPath))

	return task, nil
}

// CreateFullRestore 创建整机恢复任务
func (rm *RestoreManager) CreateFullRestore(ctx context.Context, jobID, snapshotID string, targetPath string, opts RestoreExecOptions) (*RestoreTask, error) {
	snap, err := rm.manager.GetSnapshot(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("快照 %s 不存在: %w", snapshotID, err)
	}

	task := &RestoreTask{
		ID:         uuid.New().String(),
		JobID:      jobID,
		SnapshotID: snapshotID,
		Mode:       RestoreModeFullImage,
		TargetPath: targetPath,
		Status:     RestoreStatusPending,
		TotalFiles: snap.FileCount,
		Options:    opts,
		CreatedAt:  time.Now(),
	}

	rm.mu.Lock()
	rm.tasks[task.ID] = task
	rm.mu.Unlock()

	rm.logger.Info("整机恢复任务创建",
		zap.String("task_id", task.ID),
		zap.String("snapshot_id", snapshotID),
		zap.String("target", targetPath))

	return task, nil
}

// ExecuteRestore 执行恢复任务
func (rm *RestoreManager) ExecuteRestore(ctx context.Context, taskID string) error {
	rm.mu.Lock()
	task, exists := rm.tasks[taskID]
	if !exists {
		rm.mu.Unlock()
		return fmt.Errorf("恢复任务 %s 不存在", taskID)
	}
	if task.Status != RestoreStatusPending {
		rm.mu.Unlock()
		return fmt.Errorf("恢复任务 %s 状态不允许执行: %s", taskID, task.Status)
	}
	task.Status = RestoreStatusRunning
	now := time.Now()
	task.StartedAt = &now
	rm.mu.Unlock()

	var err error
	switch task.Mode {
	case RestoreModeSingleFile:
		err = rm.executeSingleFileRestore(ctx, task)
	case RestoreModeFullImage:
		err = rm.executeFullRestore(ctx, task)
	case RestoreModeDirectory:
		err = rm.executeDirectoryRestore(ctx, task)
	default:
		err = fmt.Errorf("不支持的恢复模式: %s", task.Mode)
	}

	rm.mu.Lock()
	endNow := time.Now()
	task.CompletedAt = &endNow

	if err != nil {
		task.Status = RestoreStatusFailed
		task.Error = err.Error()
		rm.logger.Error("恢复任务失败",
			zap.String("task_id", taskID),
			zap.Error(err))
	} else {
		task.Status = RestoreStatusCompleted
		task.Progress = 1.0
		rm.logger.Info("恢复任务完成",
			zap.String("task_id", taskID),
			zap.Int("files_restored", task.FilesRestored))
	}
	rm.mu.Unlock()

	return err
}

// GetRestoreTask 获取恢复任务
func (rm *RestoreManager) GetRestoreTask(taskID string) (*RestoreTask, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	task, exists := rm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("恢复任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListRestoreTasks 列出恢复任务
func (rm *RestoreManager) ListRestoreTasks(jobID string) []*RestoreTask {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*RestoreTask, 0)
	for _, task := range rm.tasks {
		if jobID == "" || task.JobID == jobID {
			result = append(result, task)
		}
	}
	return result
}

// CancelRestore 取消恢复任务
func (rm *RestoreManager) CancelRestore(taskID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	task, exists := rm.tasks[taskID]
	if !exists {
		return fmt.Errorf("恢复任务 %s 不存在", taskID)
	}
	if task.Status != RestoreStatusRunning {
		return fmt.Errorf("恢复任务未在运行中")
	}

	task.Status = RestoreStatusCancelled
	return nil
}

// ListRestorePoints 列出可用的恢复点（快照）
func (rm *RestoreManager) ListRestorePoints(jobID string) []RestorePoint {
	snapshots := rm.manager.ListSnapshots(jobID)
	points := make([]RestorePoint, 0, len(snapshots))

	for _, snap := range snapshots {
		rp := RestorePoint{
			SnapshotID:  snap.ID,
			JobID:       snap.JobID,
			BackupType:  snap.BackupType,
			Size:        snap.Size,
			FileCount:   snap.FileCount,
			CreatedAt:   snap.CreatedAt,
		}
		points = append(points, rp)
	}

	return points
}

// RestorePoint 恢复点
type RestorePoint struct {
	SnapshotID string     `json:"snapshot_id"`
	JobID      string     `json:"job_id"`
	BackupType BackupType `json:"backup_type"`
	Size       int64      `json:"size"`
	FileCount  int        `json:"file_count"`
	CreatedAt  time.Time  `json:"created_at"`
}

// executeSingleFileRestore 执行单文件恢复
func (rm *RestoreManager) executeSingleFileRestore(ctx context.Context, task *RestoreTask) error {
	rm.logger.Info("开始单文件恢复",
		zap.String("task_id", task.ID),
		zap.Int("files", len(task.Files)))

	// 构建恢复链
	chain := rm.manager.buildRestoreChain(task.SnapshotID)
	if len(chain) == 0 {
		return fmt.Errorf("恢复链为空")
	}

	task.TotalFiles = len(task.Files)

	for i, file := range task.Files {
		select {
		case <-ctx.Done():
			return fmt.Errorf("恢复被取消")
		default:
		}

		// 逐文件恢复
		if err := rm.restoreFile(ctx, chain, file, task.TargetPath, task.Options); err != nil {
			rm.logger.Warn("恢复文件失败",
				zap.String("file", file),
				zap.Error(err))
			if !task.Options.OverwriteExisting {
				continue
			}
			return err
		}

		task.FilesRestored++
		task.Progress = float64(i+1) / float64(task.TotalFiles)
	}

	return nil
}

// executeFullRestore 执行整机恢复
func (rm *RestoreManager) executeFullRestore(ctx context.Context, task *RestoreTask) error {
	rm.logger.Info("开始整机恢复",
		zap.String("task_id", task.ID),
		zap.String("snapshot_id", task.SnapshotID))

	// 使用 BackupManager 的恢复功能
	if err := rm.manager.Restore(ctx, task.SnapshotID, task.TargetPath); err != nil {
		return err
	}

	snap, _ := rm.manager.GetSnapshot(task.SnapshotID)
	if snap != nil {
		task.FilesRestored = snap.FileCount
		task.TotalFiles = snap.FileCount
	}

	return nil
}

// executeDirectoryRestore 执行目录恢复
func (rm *RestoreManager) executeDirectoryRestore(ctx context.Context, task *RestoreTask) error {
	rm.logger.Info("开始目录恢复",
		zap.String("task_id", task.ID))

	chain := rm.manager.buildRestoreChain(task.SnapshotID)
	if len(chain) == 0 {
		return fmt.Errorf("恢复链为空")
	}

	if err := os.MkdirAll(task.TargetPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 遍历快照中的文件并恢复
	for _, snap := range chain {
		if err := rm.restoreSnapshotToDir(ctx, snap, task.TargetPath, task.Options); err != nil {
			return fmt.Errorf("恢复快照 %s 到目录失败: %w", snap.ID, err)
		}
	}

	return nil
}

// restoreFile 恢复单个文件
func (rm *RestoreManager) restoreFile(ctx context.Context, chain []*BackupSnapshot, fileName, targetDir string, opts RestoreExecOptions) error {
	targetPath := filepath.Join(targetDir, fileName)

	// 检查目标文件是否存在
	if !opts.OverwriteExisting {
		if _, err := os.Stat(targetPath); err == nil {
			return fmt.Errorf("目标文件已存在: %s", targetPath)
		}
	}

	// 从恢复链中找到包含该文件的快照
	for _, snap := range chain {
		snapFile := filepath.Join(snap.Path, fileName)
		if _, err := os.Stat(snapFile); os.IsNotExist(err) {
			continue
		}

		// 确保目标目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		// 复制文件
		if err := copyFile(snapFile, targetPath); err != nil {
			return fmt.Errorf("复制文件失败: %w", err)
		}

		// 恢复时间戳
		if opts.RestoreTimestamps {
			info, _ := os.Stat(snapFile)
			if info != nil {
				os.Chtimes(targetPath, info.ModTime(), info.ModTime())
			}
		}

		rm.logger.Debug("文件已恢复",
			zap.String("source", snapFile),
			zap.String("target", targetPath))
		return nil
	}

	return fmt.Errorf("文件 %s 在恢复链中未找到", fileName)
}

// restoreSnapshotToDir 恢复快照内容到目录
func (rm *RestoreManager) restoreSnapshotToDir(ctx context.Context, snap *BackupSnapshot, targetDir string, opts RestoreExecOptions) error {
	entries, err := os.ReadDir(snap.Path)
	if err != nil {
		return fmt.Errorf("读取快照目录失败: %w", err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return fmt.Errorf("恢复被取消")
		default:
		}

		srcPath := filepath.Join(snap.Path, entry.Name())
		dstPath := filepath.Join(targetDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			continue
		}

		if !opts.OverwriteExisting {
			if _, err := os.Stat(dstPath); err == nil {
				continue
			}
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

// GetSnapshot 获取快照（代理 BackupManager 方法）
// 这是 BackupManager 上的方法，此处为引用
func (bm *BackupManager) GetSnapshot(snapshotID string) (*BackupSnapshot, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	snap, exists := bm.snapshots[snapshotID]
	if !exists {
		return nil, fmt.Errorf("快照 %s 不存在", snapshotID)
	}
	return snap, nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := sourceFile.Read(buf)
		if n > 0 {
			if _, werr := destFile.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}
	return nil
}

// SaveTasks 保存恢复任务到磁盘
func (rm *RestoreManager) SaveTasks(path string) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	data, err := json.MarshalIndent(rm.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadTasks 从磁盘加载恢复任务
func (rm *RestoreManager) LoadTasks(path string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &rm.tasks)
}
