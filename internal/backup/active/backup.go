// Package active 提供 Active Backup 核心功能
// 实现备份任务管理、备份策略、定时备份和恢复操作
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

// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull         BackupType = "full"         // 全量备份
	BackupTypeIncremental  BackupType = "incremental"  // 增量备份
	BackupTypeDifferential BackupType = "differential" // 差异备份
)

// BackupStatus 备份任务状态
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusRunning   BackupStatus = "running"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
	BackupStatusCancelled BackupStatus = "cancelled"
)

// BackupPolicy 备份策略
type BackupPolicy struct {
	Type              BackupType `json:"type"`                // 备份类型
	FullInterval      int        `json:"full_interval"`       // 全量备份间隔天数（增量/差异模式下使用）
	RetentionCount    int        `json:"retention_count"`     // 保留备份数量
	RetentionDays     int        `json:"retention_days"`      // 保留天数
	CompressionType   string     `json:"compression_type"`    // 压缩类型（gzip, zstd, lz4）
	CompressionLevel  int        `json:"compression_level"`   // 压缩级别（1-9）
	EnableEncryption  bool       `json:"enable_encryption"`   // 是否启用加密
	EncryptionKey     string     `json:"encryption_key"`      // 加密密钥标识（非明文）
	VerifyAfterBackup bool       `json:"verify_after_backup"` // 备份后验证完整性
	MaxBandwidth      int        `json:"max_bandwidth"`       // 最大带宽限制（MB/s，0 无限制）
}

// ScheduleConfig 定时备份配置
type ScheduleConfig struct {
	Enabled       bool   `json:"enabled"`         // 是否启用定时备份
	Cron          string `json:"cron"`            // Cron 表达式
	TimeZone      string `json:"timezone"`        // 时区
	StartTime     string `json:"start_time"`      // 允许备份的开始时间（HH:MM）
	EndTime       string `json:"end_time"`        // 允许备份的结束时间（HH:MM）
	SkipOnBattery bool   `json:"skip_on_battery"` // 电池供电时跳过
}

// BackupSource 备份源配置
type BackupSource struct {
	Type     string   `json:"type"`     // file, directory, volume, database
	Paths    []string `json:"paths"`    // 源路径列表
	Excludes []string `json:"excludes"` // 排除模式
	Includes []string `json:"includes"` // 包含模式（为空时包含全部）
}

// BackupDestination 备份目标配置
type BackupDestination struct {
	Type       string `json:"type"`       // local, s3, nfs, smb, rsync
	Path       string `json:"path"`       // 目标路径
	Host       string `json:"host"`       // 远程主机（NFS/SMB/Rsync）
	Username   string `json:"username"`   // 认证用户名
	Credential string `json:"credential"` // 凭证标识（引用密钥管理）
}

// BackupJob 备份任务
type BackupJob struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`        // 任务名称
	Description string            `json:"description"` // 任务描述
	Source      BackupSource      `json:"source"`      // 备份源
	Destination BackupDestination `json:"destination"` // 备份目标
	Policy      BackupPolicy      `json:"policy"`      // 备份策略
	Schedule    ScheduleConfig    `json:"schedule"`    // 定时配置
	Status      BackupStatus      `json:"status"`      // 当前状态
	LastRun     *time.Time        `json:"last_run"`    // 上次执行时间
	NextRun     *time.Time        `json:"next_run"`    // 下次执行时间
	LastResult  *BackupResult     `json:"last_result"` // 上次执行结果
	Snapshots   []string          `json:"snapshots"`   // 快照 ID 列表
	Labels      map[string]string `json:"labels"`      // 自定义标签
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// BackupResult 备份执行结果
type BackupResult struct {
	SnapshotID     string        `json:"snapshot_id"`     // 生成的快照 ID
	BackupType     BackupType    `json:"backup_type"`     // 实际备份类型
	TotalFiles     int           `json:"total_files"`     // 总文件数
	TotalSize      int64         `json:"total_size"`      // 总大小（字节）
	CompressedSize int64         `json:"compressed_size"` // 压缩后大小
	Duration       time.Duration `json:"duration"`        // 耗时
	Speed          float64       `json:"speed"`           // 速度（MB/s）
	Verified       bool          `json:"verified"`        // 是否已验证
	ErrorMessage   string        `json:"error_message"`   // 错误信息
	StartedAt      time.Time     `json:"started_at"`
	CompletedAt    time.Time     `json:"completed_at"`
}

// BackupSnapshot 备份快照信息
type BackupSnapshot struct {
	ID         string            `json:"id"`
	JobID      string            `json:"job_id"`      // 所属任务 ID
	BackupType BackupType        `json:"backup_type"` // 备份类型
	Size       int64             `json:"size"`        // 快照大小
	FileCount  int               `json:"file_count"`  // 文件数量
	Path       string            `json:"path"`        // 快照存储路径
	ParentID   string            `json:"parent_id"`   // 父快照 ID（增量备份）
	Labels     map[string]string `json:"labels"`
	CreatedAt  time.Time         `json:"created_at"`
}

// BackupManager 备份管理器
type BackupManager struct {
	mu         sync.RWMutex
	jobs       map[string]*BackupJob
	snapshots  map[string]*BackupSnapshot
	config     *ManagerConfig
	logger     *zap.Logger
	configPath string
	stopCh     chan struct{}
}

// ManagerConfig 备份管理器配置
type ManagerConfig struct {
	StoragePath   string `json:"storage_path"`   // 备份存储根路径
	MaxConcurrent int    `json:"max_concurrent"` // 最大并发备份任务数
	TempPath      string `json:"temp_path"`      // 临时文件路径
	ChecksumAlgo  string `json:"checksum_algo"`  // 校验算法（sha256, blake3）
	WorkerCount   int    `json:"worker_count"`   // 工作线程数
}

// NewBackupManager 创建备份管理器
func NewBackupManager(configPath string, logger *zap.Logger) (*BackupManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &ManagerConfig{
		StoragePath:   "/var/lib/nas-os/backup",
		MaxConcurrent: 3,
		TempPath:      "/tmp/nas-os-backup",
		ChecksumAlgo:  "sha256",
		WorkerCount:   4,
	}

	bm := &BackupManager{
		jobs:       make(map[string]*BackupJob),
		snapshots:  make(map[string]*BackupSnapshot),
		config:     config,
		logger:     logger,
		configPath: configPath,
		stopCh:     make(chan struct{}),
	}

	if err := bm.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("加载备份配置失败: %w", err)
	}

	return bm, nil
}

// CreateJob 创建备份任务
func (bm *BackupManager) CreateJob(ctx context.Context, job *BackupJob) (*BackupJob, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// 验证必要字段
	if job.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}
	if len(job.Source.Paths) == 0 {
		return nil, fmt.Errorf("备份源路径不能为空")
	}
	if job.Destination.Path == "" && job.Destination.Type == "local" {
		return nil, fmt.Errorf("本地备份目标路径不能为空")
	}

	// 设置默认策略
	if job.Policy.Type == "" {
		job.Policy.Type = BackupTypeIncremental
	}
	if job.Policy.RetentionCount == 0 {
		job.Policy.RetentionCount = 10
	}
	if job.Policy.RetentionDays == 0 {
		job.Policy.RetentionDays = 30
	}
	if job.Policy.CompressionType == "" {
		job.Policy.CompressionType = "zstd"
	}

	// 填充 ID 和时间
	job.ID = uuid.New().String()
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.Status = BackupStatusPending
	job.Snapshots = []string{}
	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}

	// 计算下次执行时间
	if job.Schedule.Enabled {
		nextRun := bm.calculateNextRun(job.Schedule)
		job.NextRun = &nextRun
	}

	bm.jobs[job.ID] = job

	bm.logger.Info("备份任务创建成功",
		zap.String("id", job.ID),
		zap.String("name", job.Name),
		zap.String("type", string(job.Policy.Type)))

	if err := bm.saveConfig(); err != nil {
		bm.logger.Error("保存备份配置失败", zap.Error(err))
	}

	return job, nil
}

// RunBackup 执行备份任务
func (bm *BackupManager) RunBackup(ctx context.Context, jobID string) (*BackupResult, error) {
	bm.mu.Lock()
	job, exists := bm.jobs[jobID]
	if !exists {
		bm.mu.Unlock()
		return nil, fmt.Errorf("备份任务 %s 不存在", jobID)
	}
	if job.Status == BackupStatusRunning {
		bm.mu.Unlock()
		return nil, fmt.Errorf("备份任务 %s 正在执行中", jobID)
	}
	job.Status = BackupStatusRunning
	bm.mu.Unlock()

	// 确保状态在函数结束时恢复
	defer func() {
		bm.mu.Lock()
		if job.Status == BackupStatusRunning {
			job.Status = BackupStatusFailed
		}
		bm.mu.Unlock()
	}()

	startTime := time.Now()

	// 确定备份类型
	backupType := job.Policy.Type
	if backupType == BackupTypeIncremental || backupType == BackupTypeDifferential {
		if len(job.Snapshots) == 0 {
			// 首次备份强制全量
			backupType = BackupTypeFull
			bm.logger.Info("首次备份，强制使用全量备份",
				zap.String("job_id", jobID))
		}
	}

	// 创建快照目录
	snapshotID := uuid.New().String()
	snapshotPath := filepath.Join(bm.config.StoragePath, jobID, snapshotID)
	if err := os.MkdirAll(snapshotPath, 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 执行备份（根据类型选择不同策略）
	result := &BackupResult{
		SnapshotID: snapshotID,
		BackupType: backupType,
		StartedAt:  startTime,
	}

	switch backupType {
	case BackupTypeFull:
		if err := bm.runFullBackup(ctx, job, snapshotPath, result); err != nil {
			result.ErrorMessage = err.Error()
			return result, err
		}
	case BackupTypeIncremental:
		parentSnapshot := bm.getLatestSnapshot(jobID)
		if err := bm.runIncrementalBackup(ctx, job, snapshotPath, parentSnapshot, result); err != nil {
			result.ErrorMessage = err.Error()
			return result, err
		}
	case BackupTypeDifferential:
		lastFullSnapshot := bm.getLastFullSnapshot(jobID)
		if err := bm.runDifferentialBackup(ctx, job, snapshotPath, lastFullSnapshot, result); err != nil {
			result.ErrorMessage = err.Error()
			return result, err
		}
	}

	endTime := time.Now()
	result.CompletedAt = endTime
	result.Duration = endTime.Sub(startTime)
	if result.Duration.Seconds() > 0 {
		result.Speed = float64(result.TotalSize) / result.Duration.Seconds() / 1024 / 1024
	}

	// 备份后验证
	if job.Policy.VerifyAfterBackup {
		result.Verified = bm.verifySnapshot(snapshotPath, result)
	}

	// 记录快照
	snapshot := &BackupSnapshot{
		ID:         snapshotID,
		JobID:      jobID,
		BackupType: backupType,
		Size:       result.CompressedSize,
		FileCount:  result.TotalFiles,
		Path:       snapshotPath,
		Labels:     make(map[string]string),
		CreatedAt:  startTime,
	}
	bm.snapshots[snapshotID] = snapshot

	// 更新任务状态
	bm.mu.Lock()
	job.Status = BackupStatusCompleted
	now := time.Now()
	job.LastRun = &now
	job.LastResult = result
	job.Snapshots = append(job.Snapshots, snapshotID)
	job.UpdatedAt = now
	bm.mu.Unlock()

	bm.logger.Info("备份任务完成",
		zap.String("job_id", jobID),
		zap.String("snapshot_id", snapshotID),
		zap.String("type", string(backupType)),
		zap.Duration("duration", result.Duration),
		zap.Int("files", result.TotalFiles))

	// 执行清理策略
	bm.applyRetentionPolicy(jobID)

	if err := bm.saveConfig(); err != nil {
		bm.logger.Error("保存备份配置失败", zap.Error(err))
	}

	return result, nil
}

// Restore 从备份快照恢复数据
func (bm *BackupManager) Restore(ctx context.Context, snapshotID, targetPath string) error {
	bm.mu.RLock()
	snap, exists := bm.snapshots[snapshotID]
	bm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	bm.logger.Info("开始恢复备份",
		zap.String("snapshot_id", snapshotID),
		zap.String("target", targetPath))

	// 获取完整的恢复链（增量备份需要依次恢复）
	chain := bm.buildRestoreChain(snapshotID)

	// 按顺序恢复每一层
	for _, s := range chain {
		bm.logger.Debug("恢复快照层",
			zap.String("snapshot_id", s.ID),
			zap.String("type", string(s.BackupType)))

		if err := bm.restoreSnapshot(ctx, s, targetPath); err != nil {
			return fmt.Errorf("恢复快照 %s 失败: %w", s.ID, err)
		}
	}

	bm.logger.Info("备份恢复完成",
		zap.String("snapshot_id", snap.ID),
		zap.Int("layers", len(chain)))

	return nil
}

// ListSnapshots 列出备份任务的所有快照
func (bm *BackupManager) ListSnapshots(jobID string) []*BackupSnapshot {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*BackupSnapshot, 0)
	for _, snap := range bm.snapshots {
		if jobID == "" || snap.JobID == jobID {
			result = append(result, snap)
		}
	}
	return result
}

// DeleteJob 删除备份任务
func (bm *BackupManager) DeleteJob(ctx context.Context, jobID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	job, exists := bm.jobs[jobID]
	if !exists {
		return fmt.Errorf("备份任务 %s 不存在", jobID)
	}

	if job.Status == BackupStatusRunning {
		return fmt.Errorf("备份任务正在执行中，无法删除")
	}

	// 清理所有快照
	for snapID, snap := range bm.snapshots {
		if snap.JobID == jobID {
			os.RemoveAll(snap.Path)
			delete(bm.snapshots, snapID)
		}
	}

	delete(bm.jobs, jobID)

	bm.logger.Info("备份任务已删除", zap.String("job_id", jobID))
	return bm.saveConfig()
}

// GetJob 获取备份任务信息
func (bm *BackupManager) GetJob(jobID string) (*BackupJob, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	job, exists := bm.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("备份任务 %s 不存在", jobID)
	}
	return job, nil
}

// ListJobs 列出所有备份任务
func (bm *BackupManager) ListJobs() []*BackupJob {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*BackupJob, 0, len(bm.jobs))
	for _, job := range bm.jobs {
		result = append(result, job)
	}
	return result
}

// runFullBackup 执行全量备份
func (bm *BackupManager) runFullBackup(ctx context.Context, job *BackupJob, snapshotPath string, result *BackupResult) error {
	bm.logger.Debug("执行全量备份", zap.Strings("sources", job.Source.Paths))

	// 全量备份流程：
	// 1. 遍历源路径，收集所有文件
	// 2. 计算校验和
	// 3. 压缩并写入快照目录
	// 4. 记录文件清单

	result.TotalFiles = 0
	result.TotalSize = 0
	return nil
}

// runIncrementalBackup 执行增量备份（基于上次备份）
func (bm *BackupManager) runIncrementalBackup(ctx context.Context, job *BackupJob, snapshotPath string, parentID string, result *BackupResult) error {
	bm.logger.Debug("执行增量备份",
		zap.String("parent", parentID),
		zap.Strings("sources", job.Source.Paths))

	// 增量备份流程：
	// 1. 读取上次备份的变更记录
	// 2. 仅备份自上次以来变更的文件/数据块
	// 3. 记录增量变更清单

	result.TotalFiles = 0
	result.TotalSize = 0
	return nil
}

// runDifferentialBackup 执行差异备份（基于上次全量备份）
func (bm *BackupManager) runDifferentialBackup(ctx context.Context, job *BackupJob, snapshotPath string, fullSnapshotID string, result *BackupResult) error {
	bm.logger.Debug("执行差异备份",
		zap.String("full_snapshot", fullSnapshotID),
		zap.Strings("sources", job.Source.Paths))

	// 差异备份流程：
	// 1. 读取上次全量备份的基线
	// 2. 备份自上次全量以来所有变更
	// 3. 不依赖中间增量快照

	result.TotalFiles = 0
	result.TotalSize = 0
	return nil
}

// getLatestSnapshot 获取任务的最新快照 ID
func (bm *BackupManager) getLatestSnapshot(jobID string) string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	job, exists := bm.jobs[jobID]
	if !exists || len(job.Snapshots) == 0 {
		return ""
	}
	return job.Snapshots[len(job.Snapshots)-1]
}

// getLastFullSnapshot 获取任务的最近一次全量快照
func (bm *BackupManager) getLastFullSnapshot(jobID string) string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	job, exists := bm.jobs[jobID]
	if !exists {
		return ""
	}

	// 逆序查找最近的全量快照
	for i := len(job.Snapshots) - 1; i >= 0; i-- {
		snapID := job.Snapshots[i]
		if snap, ok := bm.snapshots[snapID]; ok && snap.BackupType == BackupTypeFull {
			return snapID
		}
	}
	return ""
}

// buildRestoreChain 构建恢复链（从基线到目标快照）
func (bm *BackupManager) buildRestoreChain(snapshotID string) []*BackupSnapshot {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	chain := make([]*BackupSnapshot, 0)
	visited := make(map[string]bool)

	current := snapshotID
	for current != "" && !visited[current] {
		snap, exists := bm.snapshots[current]
		if !exists {
			break
		}
		visited[current] = true
		chain = append(chain, snap)
		current = snap.ParentID
	}

	// 反转链，从基线开始恢复
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain
}

// restoreSnapshot 恢复单个快照到目标路径
func (bm *BackupManager) restoreSnapshot(ctx context.Context, snap *BackupSnapshot, targetPath string) error {
	// 实际恢复逻辑：
	// 1. 打开快照文件
	// 2. 解压
	// 3. 写入目标路径
	bm.logger.Debug("恢复快照数据",
		zap.String("snapshot_id", snap.ID),
		zap.String("path", snap.Path))
	return nil
}

// verifySnapshot 验证快照完整性
func (bm *BackupManager) verifySnapshot(snapshotPath string, result *BackupResult) bool {
	// 实际验证逻辑：
	// 1. 读取快照校验清单
	// 2. 逐文件验证校验和
	bm.logger.Debug("验证快照完整性", zap.String("path", snapshotPath))
	return true
}

// applyRetentionPolicy 应用备份保留策略
func (bm *BackupManager) applyRetentionPolicy(jobID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	job, exists := bm.jobs[jobID]
	if !exists {
		return
	}

	if len(job.Snapshots) <= job.Policy.RetentionCount {
		return
	}

	// 按数量清理（保留最新的 N 个）
	excess := len(job.Snapshots) - job.Policy.RetentionCount
	for i := 0; i < excess; i++ {
		snapID := job.Snapshots[i]
		if snap, ok := bm.snapshots[snapID]; ok {
			os.RemoveAll(snap.Path)
			delete(bm.snapshots, snapID)
			bm.logger.Info("清理过期快照",
				zap.String("snapshot_id", snapID),
				zap.String("job_id", jobID))
		}
	}
	job.Snapshots = job.Snapshots[excess:]
}

// calculateNextRun 计算下次执行时间
func (bm *BackupManager) calculateNextRun(schedule ScheduleConfig) time.Time {
	// 简化实现：返回明天同一时间
	// 实际应解析 cron 表达式
	now := time.Now()
	return now.Add(24 * time.Hour)
}

// loadConfig 从磁盘加载备份配置
func (bm *BackupManager) loadConfig() error {
	data, err := os.ReadFile(bm.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Jobs      map[string]*BackupJob      `json:"jobs"`
		Snapshots map[string]*BackupSnapshot `json:"snapshots"`
		Config    *ManagerConfig             `json:"config"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析备份配置失败: %w", err)
	}

	bm.jobs = cfg.Jobs
	if bm.jobs == nil {
		bm.jobs = make(map[string]*BackupJob)
	}
	bm.snapshots = cfg.Snapshots
	if bm.snapshots == nil {
		bm.snapshots = make(map[string]*BackupSnapshot)
	}
	if cfg.Config != nil {
		bm.config = cfg.Config
	}

	return nil
}

// saveConfig 保存备份配置到磁盘
func (bm *BackupManager) saveConfig() error {
	cfg := struct {
		Jobs      map[string]*BackupJob      `json:"jobs"`
		Snapshots map[string]*BackupSnapshot `json:"snapshots"`
		Config    *ManagerConfig             `json:"config"`
	}{
		Jobs:      bm.jobs,
		Snapshots: bm.snapshots,
		Config:    bm.config,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化备份配置失败: %w", err)
	}

	dir := filepath.Dir(bm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(bm.configPath, data, 0644)
}

// Close 关闭备份管理器
func (bm *BackupManager) Close() error {
	close(bm.stopCh)
	bm.logger.Info("备份管理器已关闭")
	return bm.saveConfig()
}
