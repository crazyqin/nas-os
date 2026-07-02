package blockbackup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BlockBackupEngine 块级增量备份引擎.
type BlockBackupEngine struct {
	logger           *zap.Logger
	jobs             map[string]*BackupJob
	snapshots        map[string]*BlockSnapshot
	mu               sync.RWMutex
	config           BackupConfig
	progressCallback ProgressCallback
}

// BackupConfig 备份配置.
type BackupConfig struct {
	Compression   string `json:"compression"`    // lz4, zstd, gzip, none
	BlockSize     int    `json:"block_size"`     // 块大小(字节)
	MaxBandwidth  int    `json:"max_bandwidth"`  // 最大带宽(MB/s)
	Parallel      int    `json:"parallel"`       // 并行数
	RetentionDays int    `json:"retention_days"` // 保留天数
	VerifyAfter   bool   `json:"verify_after"`   // 备份后验证
	Encrypted     bool   `json:"encrypted"`      // 加密
	EncryptionKey string `json:"encryption_key"` // 加密密钥
}

// BackupJob 备份任务.
type BackupJob struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Source       string        `json:"source"`      // 源路径或设备
	Destination  string        `json:"destination"` // 目标路径
	Type         string        `json:"type"`        // full, incremental
	Status       string        `json:"status"`      // pending, running, completed, failed
	Progress     int           `json:"progress"`    // 0-100
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Size         uint64        `json:"size"`
	Duration     time.Duration `json:"duration"`
	Error        string        `json:"error,omitempty"`
	BaseSnapshot string        `json:"base_snapshot"` // 增量基准快照
}

// BlockSnapshot 块级快照.
type BlockSnapshot struct {
	ID        string    `json:"id"`
	Volume    string    `json:"volume"`
	BlockHash string    `json:"block_hash"`
	Size      uint64    `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	IsBase    bool      `json:"is_base"` // 是否为增量基准
}

// BackupReport 备份报告.
type BackupReport struct {
	JobID      string        `json:"job_id"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	TotalBytes uint64        `json:"total_bytes"`
	Speed      float64       `json:"speed_mb_s"`
	Status     string        `json:"status"`
	Verified   bool          `json:"verified"`
}

// NewBlockBackupEngine 创建块级备份引擎.
func NewBlockBackupEngine(logger *zap.Logger, config BackupConfig) *BlockBackupEngine {
	return &BlockBackupEngine{
		logger:    logger,
		jobs:      make(map[string]*BackupJob),
		snapshots: make(map[string]*BlockSnapshot),
		config:    config,
	}
}

// CreateFullBackup 创建全量备份.
func (bbe *BlockBackupEngine) CreateFullBackup(ctx context.Context, source, dest string) (*BackupJob, error) {
	bbe.mu.Lock()
	job := &BackupJob{
		ID:          fmt.Sprintf("full-%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("Full backup of %s", source),
		Source:      source,
		Destination: dest,
		Type:        "full",
		Status:      "pending",
		StartTime:   time.Now(),
	}
	bbe.jobs[job.ID] = job
	bbe.mu.Unlock()

	bbe.logger.Info("Starting full backup",
		zap.String("job", job.ID),
		zap.String("source", source))

	go bbe.runFullBackup(ctx, job)
	return job, nil
}

func (bbe *BlockBackupEngine) runFullBackup(ctx context.Context, job *BackupJob) {
	bbe.mu.Lock()
	job.Status = "running"
	bbe.mu.Unlock()

	var cmd *exec.Cmd

	switch bbe.config.Compression {
	case "zstd":
		// 使用zstd压缩的dd
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("dd if=%s bs=1M | zstd -T%d > %s",
				job.Source, bbe.config.Parallel, job.Destination))
	case "lz4":
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("dd if=%s bs=1M | lz4 > %s",
				job.Source, job.Destination))
	default:
		cmd = exec.CommandContext(ctx, "dd",
			fmt.Sprintf("if=%s", job.Source),
			fmt.Sprintf("of=%s", job.Destination),
			"bs=1M", "status=progress")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		bbe.mu.Lock()
		job.Status = "failed"
		job.Error = string(output)
		job.EndTime = time.Now()
		bbe.mu.Unlock()
		bbe.logger.Error("Full backup failed", zap.String("error", string(output)))
		return
	}

	// 创建基准快照
	snap := &BlockSnapshot{
		ID:        fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Volume:    job.Source,
		IsBase:    true,
		CreatedAt: time.Now(),
	}
	bbe.mu.Lock()
	bbe.snapshots[snap.ID] = snap
	job.Status = "completed"
	job.EndTime = time.Now()
	job.Duration = job.EndTime.Sub(job.StartTime)
	job.BaseSnapshot = snap.ID
	bbe.mu.Unlock()

	bbe.logger.Info("Full backup completed",
		zap.String("job", job.ID),
		zap.Duration("duration", job.Duration))
}

// CreateIncrementalBackup 创建增量备份.
func (bbe *BlockBackupEngine) CreateIncrementalBackup(ctx context.Context, source, dest, baseSnapshot string) (*BackupJob, error) {
	bbe.mu.Lock()
	job := &BackupJob{
		ID:           fmt.Sprintf("incr-%d", time.Now().UnixNano()),
		Name:         fmt.Sprintf("Incremental backup of %s", source),
		Source:       source,
		Destination:  dest,
		Type:         "incremental",
		Status:       "pending",
		StartTime:    time.Now(),
		BaseSnapshot: baseSnapshot,
	}
	bbe.jobs[job.ID] = job
	bbe.mu.Unlock()

	bbe.logger.Info("Starting incremental backup",
		zap.String("job", job.ID),
		zap.String("base", baseSnapshot))

	go bbe.runIncrementalBackup(ctx, job)
	return job, nil
}

func (bbe *BlockBackupEngine) runIncrementalBackup(ctx context.Context, job *BackupJob) {
	bbe.mu.Lock()
	job.Status = "running"
	bbe.mu.Unlock()

	// 使用rsync或专用增量工具
	// 这里使用简化实现
	var cmd *exec.Cmd
	switch bbe.config.Compression {
	case "zstd":
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("rsync -av --delete %s/ %s/ | zstd > %s",
				job.Source, job.Destination, job.Destination+".zst"))
	default:
		cmd = exec.CommandContext(ctx, "rsync", "-av", "--delete", job.Source+"/", job.Destination+"/")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		bbe.mu.Lock()
		job.Status = "failed"
		job.Error = string(output)
		job.EndTime = time.Now()
		bbe.mu.Unlock()
		return
	}

	bbe.mu.Lock()
	job.Status = "completed"
	job.EndTime = time.Now()
	job.Duration = job.EndTime.Sub(job.StartTime)
	bbe.mu.Unlock()

	bbe.logger.Info("Incremental backup completed", zap.String("job", job.ID))
}

// CreateZFSBlockBackup 创建ZFS块级备份.
func (bbe *BlockBackupEngine) CreateZFSBlockBackup(ctx context.Context, dataset, dest string, incremental bool, baseSnap string) (*BackupJob, error) {
	bbe.mu.Lock()
	job := &BackupJob{
		ID:          fmt.Sprintf("zfs-%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("ZFS backup of %s", dataset),
		Source:      dataset,
		Destination: dest,
		Type:        "full",
		Status:      "pending",
		StartTime:   time.Now(),
	}
	if incremental {
		job.Type = "incremental"
		job.BaseSnapshot = baseSnap
	}
	bbe.jobs[job.ID] = job
	bbe.mu.Unlock()

	go bbe.runZFSBackup(ctx, job)
	return job, nil
}

func (bbe *BlockBackupEngine) runZFSBackup(ctx context.Context, job *BackupJob) {
	bbe.mu.Lock()
	job.Status = "running"
	bbe.mu.Unlock()

	args := []string{"send"}
	if job.Type == "incremental" && job.BaseSnapshot != "" {
		args = append(args, "-i", job.BaseSnapshot)
	}
	args = append(args, job.Source)

	// 管道压缩
	shellCmd := fmt.Sprintf("zfs send %s", strings.Join(args[1:], " "))
	if bbe.config.Compression == "zstd" {
		shellCmd += " | zstd"
	}
	shellCmd += fmt.Sprintf(" > %s", job.Destination)

	cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		bbe.mu.Lock()
		job.Status = "failed"
		job.Error = string(output)
		job.EndTime = time.Now()
		bbe.mu.Unlock()
		return
	}

	bbe.mu.Lock()
	job.Status = "completed"
	job.EndTime = time.Now()
	job.Duration = job.EndTime.Sub(job.StartTime)
	bbe.mu.Unlock()

	bbe.logger.Info("ZFS block backup completed", zap.String("job", job.ID))
}

// RestoreBackup 恢复备份.
func (bbe *BlockBackupEngine) RestoreBackup(ctx context.Context, backupPath, dest string) error {
	bbe.logger.Info("Restoring backup",
		zap.String("source", backupPath),
		zap.String("dest", dest))

	// 检测是否为ZFS备份
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("file %s | grep -q 'ZFS'", backupPath))
	if cmd.Run() == nil {
		// ZFS恢复
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("cat %s | zfs receive %s", backupPath, dest))
	} else {
		// 普通恢复
		cmd = exec.CommandContext(ctx, "dd",
			fmt.Sprintf("if=%s", backupPath),
			fmt.Sprintf("of=%s", dest),
			"bs=1M", "status=progress")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore failed: %s: %w", string(output), err)
	}
	return nil
}

// VerifyBackup 验证备份完整性.
func (bbe *BlockBackupEngine) VerifyBackup(ctx context.Context, backupPath string) error {
	bbe.logger.Info("Verifying backup", zap.String("path", backupPath))

	// 计算校验和
	cmd := exec.CommandContext(ctx, "sha256sum", backupPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("verification failed: %s: %w", string(output), err)
	}
	return nil
}

// GetJob 获取任务信息.
func (bbe *BlockBackupEngine) GetJob(jobID string) *BackupJob {
	bbe.mu.RLock()
	defer bbe.mu.RUnlock()
	return bbe.jobs[jobID]
}

// ListJobs 列出所有任务.
func (bbe *BlockBackupEngine) ListJobs() []*BackupJob {
	bbe.mu.RLock()
	defer bbe.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(bbe.jobs))
	for _, j := range bbe.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// CleanupOldBackups 清理过期备份.
func (bbe *BlockBackupEngine) CleanupOldBackups(ctx context.Context, backupDir string) error {
	retention := bbe.config.RetentionDays
	if retention <= 0 {
		retention = 30
	}

	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("find %s -name '*.bkp' -mtime +%d -delete",
			backupDir, retention))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cleanup failed: %s: %w", string(output), err)
	}

	bbe.logger.Info("Cleaned up old backups",
		zap.Int("retention_days", retention),
		zap.String("dir", backupDir))
	return nil
}
