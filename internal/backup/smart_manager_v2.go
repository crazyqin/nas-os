// Package backup 智能备份管理器 v2
// Version: v2.50.0 - 智能备份系统核心模块
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============================================================================
// 智能备份管理器 v2
// ============================================================================

// SmartManagerV2 智能备份管理器 v2
type SmartManagerV2 struct {
	mu sync.RWMutex

	// 配置管理
	config     *SmartBackupConfigV2
	configPath string

	// 备份任务管理
	jobs       map[string]*SmartBackupJobV2
	activeJobs map[string]*ActiveBackupJobV2

	// 备份索引
	backupIndex *BackupIndexV2
	fileIndex   *FileIndex

	// 调度器引用
	scheduler *BackupScheduler

	// 日志
	logger *zap.Logger

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// SmartBackupConfigV2 智能备份配置 v2
type SmartBackupConfigV2 struct {
	BackupPath    string `json:"backup_path"`
	TempPath      string `json:"temp_path"`
	IndexDBPath   string `json:"index_db_path"`
	MaxConcurrent int    `json:"max_concurrent"`

	Compression  CompressionConfigV2  `json:"compression"`
	Encryption   EncryptionConfigV2   `json:"encryption"`
	Incremental  IncrementalConfigV2  `json:"incremental"`
	Versioning   VersioningConfigV2   `json:"versioning"`
	Cleanup      CleanupConfigV2      `json:"cleanup"`
	Performance  PerformanceConfigV2  `json:"performance"`
	Verification VerificationConfigV2 `json:"verification"`
}

// CompressionConfigV2 压缩配置
type CompressionConfigV2 struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"` // gzip, zstd, lz4
	Level     int    `json:"level"`
}

// EncryptionConfigV2 加密配置
type EncryptionConfigV2 struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	KeyPath   string `json:"key_path"`
}

// IncrementalConfigV2 增量备份配置
type IncrementalConfigV2 struct {
	Enabled        bool  `json:"enabled"`
	ChunkSize      int64 `json:"chunk_size"`
	FullBackupDays int   `json:"full_backup_days"`
	MaxIncremental int   `json:"max_incremental"`
	Deduplication  bool  `json:"deduplication"`
}

// VersioningConfigV2 版本管理配置
type VersioningConfigV2 struct {
	Enabled     bool `json:"enabled"`
	MaxVersions int  `json:"max_versions"`
	KeepDaily   int  `json:"keep_daily"`
	KeepWeekly  int  `json:"keep_weekly"`
	KeepMonthly int  `json:"keep_monthly"`
}

// CleanupConfigV2 清理策略配置
type CleanupConfigV2 struct {
	Enabled      bool  `json:"enabled"`
	MaxAge       int   `json:"max_age"`
	MaxSize      int64 `json:"max_size"`
	MinFreeSpace int64 `json:"min_free_space"`
}

// PerformanceConfigV2 性能配置
type PerformanceConfigV2 struct {
	IOBufferSize  int   `json:"io_buffer_size"`
	ParallelFiles int   `json:"parallel_files"`
	QueueSize     int   `json:"queue_size"`
	SpeedLimit    int64 `json:"speed_limit"`
}

// VerificationConfigV2 验证配置
type VerificationConfigV2 struct {
	Enabled       bool   `json:"enabled"`
	VerifyOnWrite bool   `json:"verify_on_write"`
	HashAlgorithm string `json:"hash_algorithm"`
}

// DefaultSmartBackupConfigV2 默认智能备份配置
func DefaultSmartBackupConfigV2() *SmartBackupConfigV2 {
	return &SmartBackupConfigV2{
		BackupPath:    "/var/lib/nas-os/backups",
		TempPath:      "/var/lib/nas-os/backups/temp",
		IndexDBPath:   "/var/lib/nas-os/backups/index.db",
		MaxConcurrent: 3,
		Compression: CompressionConfigV2{
			Enabled:   true,
			Algorithm: "gzip",
			Level:     6,
		},
		Encryption: EncryptionConfigV2{
			Enabled:   false,
			Algorithm: "aes-256-gcm",
		},
		Incremental: IncrementalConfigV2{
			Enabled:        true,
			ChunkSize:      4 * 1024 * 1024,
			FullBackupDays: 7,
			MaxIncremental: 30,
			Deduplication:  true,
		},
		Versioning: VersioningConfigV2{
			Enabled:     true,
			MaxVersions: 100,
			KeepDaily:   7,
			KeepWeekly:  4,
			KeepMonthly: 12,
		},
		Cleanup: CleanupConfigV2{
			Enabled:      true,
			MaxAge:       90,
			MinFreeSpace: 10 * 1024 * 1024 * 1024,
		},
		Performance: PerformanceConfigV2{
			IOBufferSize:  64 * 1024,
			ParallelFiles: runtime.NumCPU(),
			QueueSize:     1000,
		},
		Verification: VerificationConfigV2{
			Enabled:       true,
			VerifyOnWrite: true,
			HashAlgorithm: "sha256",
		},
	}
}

// IndexV2 备份索引
type IndexV2 struct {
	mu       sync.RWMutex
	versions map[string]*VersionV2
}

// BackupIndexV2 备份索引（兼容别名）
type BackupIndexV2 = IndexV2

// VersionV2 备份版本信息
type VersionV2 struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	SnapshotType string            `json:"snapshot_type"`
	CreatedAt    time.Time         `json:"created_at"`
	Size         int64             `json:"size"`
	Checksum     string            `json:"checksum"`
	Path         string            `json:"path"`
	Status       StatusV2          `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
}

// BackupVersionV2 备份版本信息（兼容别名）
type BackupVersionV2 = VersionV2

// StatusV2 备份状态
type StatusV2 string

// BackupStatusV2 备份状态（兼容别名）
type BackupStatusV2 = StatusV2

const (
	// BackupStatusV2Pending indicates the backup is pending.
	BackupStatusV2Pending StatusV2 = "pending"
	// BackupStatusV2Running indicates the backup is running.
	BackupStatusV2Running StatusV2 = "running"
	// BackupStatusV2Completed indicates the backup completed successfully.
	BackupStatusV2Completed StatusV2 = "completed"
	// BackupStatusV2Failed indicates the backup failed.
	BackupStatusV2Failed StatusV2 = "failed"
)

// ActiveBackupJobV2 活动备份作业
type ActiveBackupJobV2 struct {
	ID         string
	ConfigID   string
	Status     BackupStatusV2
	Progress   float64
	StartTime  time.Time
	EndTime    *time.Time
	TotalSize  int64
	Processed  int64
	Error      string
	CancelFunc context.CancelFunc
	Cancelled  bool
}

// ============================================================================
// 创建与初始化
// ============================================================================

// NewSmartManagerV2 创建智能备份管理器 v2
func NewSmartManagerV2(configPath string, logger *zap.Logger) (*SmartManagerV2, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	sm := &SmartManagerV2{
		configPath:  configPath,
		jobs:        make(map[string]*SmartBackupJobV2),
		activeJobs:  make(map[string]*ActiveBackupJobV2),
		backupIndex: &BackupIndexV2{versions: make(map[string]*BackupVersionV2)},
		fileIndex:   NewFileIndex(),
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}

	// 加载配置
	if err := sm.loadConfig(); err != nil {
		logger.Warn("加载配置失败，使用默认配置", zap.Error(err))
		sm.config = DefaultSmartBackupConfigV2()
	}

	// 确保必要目录存在
	if err := sm.ensureDirectories(); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 加载备份索引
	if err := sm.loadBackupIndex(); err != nil && !os.IsNotExist(err) {
		logger.Warn("加载备份索引失败，将使用空索引", zap.Error(err))
	}

	logger.Info("智能备份管理器 v2 初始化完成",
		zap.String("backup_path", sm.config.BackupPath),
	)

	return sm, nil
}

// loadConfig 加载配置
func (sm *SmartManagerV2) loadConfig() error {
	data, err := os.ReadFile(sm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			sm.config = DefaultSmartBackupConfigV2()
			return sm.saveConfig()
		}
		return err
	}
	sm.config = DefaultSmartBackupConfigV2()
	return json.Unmarshal(data, sm.config)
}

// saveConfig 保存配置
func (sm *SmartManagerV2) saveConfig() error {
	data, err := json.MarshalIndent(sm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sm.configPath, data, 0600)
}

// ensureDirectories 确保必要目录存在
func (sm *SmartManagerV2) ensureDirectories() error {
	dirs := []string{sm.config.BackupPath, sm.config.TempPath}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// loadBackupIndex 加载备份索引
func (sm *SmartManagerV2) loadBackupIndex() error {
	data, err := os.ReadFile(sm.config.IndexDBPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &sm.backupIndex.versions)
}

// saveBackupIndex 保存备份索引
func (sm *SmartManagerV2) saveBackupIndex() error {
	sm.backupIndex.mu.RLock()
	data, err := json.MarshalIndent(sm.backupIndex.versions, "", "  ")
	sm.backupIndex.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(sm.config.IndexDBPath, data, 0600)
}

// Close 关闭管理器
func (sm *SmartManagerV2) Close() error {
	sm.mu.Lock()
	for _, job := range sm.activeJobs {
		if job.CancelFunc != nil {
			job.Cancelled = true
			job.CancelFunc()
		}
	}
	if err := sm.saveBackupIndex(); err != nil {
		sm.logger.Warn("保存备份索引失败", zap.Error(err))
	}
	sm.cancel()
	sm.mu.Unlock()
	return nil
}

// ============================================================================
// 备份作业管理
// ============================================================================

// SmartBackupJobV2 智能备份作业定义 v2
type SmartBackupJobV2 struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Source      string           `json:"source"`
	Destination string           `json:"destination"`
	Enabled     bool             `json:"enabled"`
	Schedule    string           `json:"schedule"`
	Priority    int              `json:"priority"`
	Retention   int              `json:"retention"`
	Tags        []string         `json:"tags"`
	Config      SmartJobConfigV2 `json:"config"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	LastRun     *time.Time       `json:"last_run"`
	Status      BackupStatusV2   `json:"status"`
}

// SmartJobConfigV2 作业配置
type SmartJobConfigV2 struct {
	Exclude     []string `json:"exclude,omitempty"`
	PreScript   string   `json:"pre_script,omitempty"`
	PostScript  string   `json:"post_script,omitempty"`
	VerifyAfter bool     `json:"verify_after"`
}

// CreateJob 创建备份作业
func (sm *SmartManagerV2) CreateJob(job *SmartBackupJobV2) error {
	if job.ID == "" {
		job.ID = generateUUIDV2()
	}
	if job.Name == "" {
		return errors.New("作业名称不能为空")
	}
	if job.Source == "" {
		return errors.New("源路径不能为空")
	}
	if _, err := os.Stat(job.Source); err != nil {
		return fmt.Errorf("源路径无效: %w", err)
	}

	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt
	job.Status = BackupStatusV2Pending

	sm.mu.Lock()
	sm.jobs[job.ID] = job
	sm.mu.Unlock()

	sm.logger.Info("创建备份作业", zap.String("id", job.ID), zap.String("name", job.Name))
	return nil
}

// GetJob 获取备份作业
func (sm *SmartManagerV2) GetJob(id string) (*SmartBackupJobV2, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	job, ok := sm.jobs[id]
	if !ok {
		return nil, fmt.Errorf("备份作业不存在: %s", id)
	}
	return job, nil
}

// ListJobs 列出备份作业
func (sm *SmartManagerV2) ListJobs() []*SmartBackupJobV2 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	jobs := make([]*SmartBackupJobV2, 0, len(sm.jobs))
	for _, job := range sm.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs
}

// UpdateJob 更新备份作业
func (sm *SmartManagerV2) UpdateJob(id string, job *SmartBackupJobV2) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.jobs[id]; !ok {
		return fmt.Errorf("备份作业不存在: %s", id)
	}
	job.ID = id
	job.UpdatedAt = time.Now()
	sm.jobs[id] = job
	return nil
}

// DeleteJob 删除备份作业
func (sm *SmartManagerV2) DeleteJob(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.jobs[id]; !ok {
		return fmt.Errorf("备份作业不存在: %s", id)
	}
	delete(sm.jobs, id)
	return nil
}

// ============================================================================
// 备份执行
// ============================================================================

// RunBackup 执行备份
func (sm *SmartManagerV2) RunBackup(jobID string) (*ActiveBackupJobV2, error) {
	sm.mu.Lock()
	job, exists := sm.jobs[jobID]
	if !exists {
		sm.mu.Unlock()
		return nil, errors.New("备份作业不存在")
	}
	if activeJob, exists := sm.activeJobs[jobID]; exists && activeJob.Status == BackupStatusV2Running {
		sm.mu.Unlock()
		return nil, errors.New("备份正在运行")
	}
	if len(sm.activeJobs) >= sm.config.MaxConcurrent {
		sm.mu.Unlock()
		return nil, errors.New("已达到最大并发备份数")
	}

	ctx, cancel := context.WithCancel(sm.ctx)
	activeJob := &ActiveBackupJobV2{
		ID:         generateUUIDV2(),
		ConfigID:   jobID,
		Status:     BackupStatusV2Running,
		StartTime:  time.Now(),
		CancelFunc: cancel,
	}
	sm.activeJobs[jobID] = activeJob
	sm.mu.Unlock()

	go sm.executeBackup(ctx, job, activeJob)
	return activeJob, nil
}

// executeBackup 执行备份核心逻辑
func (sm *SmartManagerV2) executeBackup(ctx context.Context, job *SmartBackupJobV2, activeJob *ActiveBackupJobV2) {
	defer func() {
		sm.mu.Lock()
		delete(sm.activeJobs, job.ID)
		now := time.Now()
		activeJob.EndTime = &now
		sm.mu.Unlock()
	}()

	sm.logger.Info("开始执行备份", zap.String("job_id", job.ID))

	// 创建目标路径
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s", job.Name, timestamp)
	destPath := filepath.Join(sm.config.BackupPath, job.Name, backupName)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		activeJob.Status = BackupStatusV2Failed
		activeJob.Error = fmt.Sprintf("创建目标目录失败: %v", err)
		return
	}

	// 执行压缩备份
	compressedPath := destPath + ".tar.gz"
	cmd := exec.CommandContext(ctx, "tar", "-czf", compressedPath, "-C", job.Source, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		activeJob.Status = BackupStatusV2Failed
		activeJob.Error = fmt.Sprintf("压缩失败: %v, output: %s", err, string(output))
		return
	}

	// 计算校验和
	checksum, err := sm.calculateChecksumV2(compressedPath)
	if err != nil {
		activeJob.Status = BackupStatusV2Failed
		activeJob.Error = fmt.Sprintf("计算校验和失败: %v", err)
		return
	}

	// 记录备份版本
	version := &BackupVersionV2{
		ID:           generateUUIDV2(),
		Name:         job.Name,
		SnapshotType: "full",
		CreatedAt:    time.Now(),
		Checksum:     checksum,
		Path:         compressedPath,
		Status:       BackupStatusV2Completed,
		Tags:         job.Tags,
	}

	sm.backupIndex.mu.Lock()
	sm.backupIndex.versions[version.ID] = version
	sm.backupIndex.mu.Unlock()

	sm.mu.Lock()
	job.LastRun = &activeJob.StartTime
	job.Status = BackupStatusV2Completed
	sm.mu.Unlock()

	if err := sm.saveBackupIndex(); err != nil {
		sm.logger.Warn("保存备份索引失败", zap.Error(err))
	}

	activeJob.Status = BackupStatusV2Completed
	activeJob.Progress = 100

	sm.logger.Info("备份完成", zap.String("job_id", job.ID), zap.String("backup_id", version.ID))
}

// calculateChecksumV2 计算校验和
func (sm *SmartManagerV2) calculateChecksumV2(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ============================================================================
// 恢复操作
// ============================================================================

// RestoreBackup 恢复备份
func (sm *SmartManagerV2) RestoreBackup(backupID, targetPath string, overwrite bool) (*ActiveBackupJobV2, error) {
	sm.backupIndex.mu.RLock()
	version, exists := sm.backupIndex.versions[backupID]
	sm.backupIndex.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("备份不存在: %s", backupID)
	}

	activeJob := &ActiveBackupJobV2{
		ID:        generateUUIDV2(),
		ConfigID:  backupID,
		Status:    BackupStatusV2Running,
		StartTime: time.Now(),
	}

	sm.mu.Lock()
	sm.activeJobs["restore-"+backupID] = activeJob
	sm.mu.Unlock()

	go sm.executeRestoreV2(version, targetPath, activeJob)
	return activeJob, nil
}

// executeRestoreV2 执行恢复
func (sm *SmartManagerV2) executeRestoreV2(version *BackupVersionV2, targetPath string, activeJob *ActiveBackupJobV2) {
	defer func() {
		sm.mu.Lock()
		delete(sm.activeJobs, "restore-"+version.ID)
		now := time.Now()
		activeJob.EndTime = &now
		sm.mu.Unlock()
	}()

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		activeJob.Status = BackupStatusV2Failed
		activeJob.Error = fmt.Sprintf("创建目标目录失败: %v", err)
		return
	}

	cmd := exec.Command("tar", "-xzf", version.Path, "-C", targetPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		activeJob.Status = BackupStatusV2Failed
		activeJob.Error = fmt.Sprintf("解压失败: %v, output: %s", err, string(output))
		return
	}

	activeJob.Status = BackupStatusV2Completed
	activeJob.Progress = 100
	sm.logger.Info("备份恢复完成", zap.String("backup_id", version.ID))
}

// ============================================================================
// 版本管理
// ============================================================================

// ListVersions 列出备份版本
func (sm *SmartManagerV2) ListVersions(name string) []*BackupVersionV2 {
	sm.backupIndex.mu.RLock()
	defer sm.backupIndex.mu.RUnlock()

	versions := make([]*BackupVersionV2, 0)
	for _, v := range sm.backupIndex.versions {
		if name == "" || v.Name == name {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})
	return versions
}

// GetVersion 获取备份版本详情
func (sm *SmartManagerV2) GetVersion(backupID string) (*BackupVersionV2, error) {
	sm.backupIndex.mu.RLock()
	defer sm.backupIndex.mu.RUnlock()

	version, exists := sm.backupIndex.versions[backupID]
	if !exists {
		return nil, fmt.Errorf("备份版本不存在: %s", backupID)
	}
	return version, nil
}

// DeleteVersion 删除备份版本
func (sm *SmartManagerV2) DeleteVersion(backupID string) error {
	sm.backupIndex.mu.Lock()
	defer sm.backupIndex.mu.Unlock()

	version, exists := sm.backupIndex.versions[backupID]
	if !exists {
		return fmt.Errorf("备份版本不存在: %s", backupID)
	}

	if err := os.Remove(version.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除备份文件失败: %w", err)
	}

	delete(sm.backupIndex.versions, backupID)
	if err := sm.saveBackupIndex(); err != nil {
		return fmt.Errorf("保存备份索引失败: %w", err)
	}
	return nil
}

// ============================================================================
// 状态与统计
// ============================================================================

// GetActiveJob 获取活动作业状态
func (sm *SmartManagerV2) GetActiveJob(jobID string) (*ActiveBackupJobV2, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	job, exists := sm.activeJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("活动作业不存在: %s", jobID)
	}
	return job, nil
}

// GetStats 获取统计信息
func (sm *SmartManagerV2) GetStats() *BackupStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := &BackupStats{TotalBackups: len(sm.backupIndex.versions)}
	var totalSize int64
	for _, v := range sm.backupIndex.versions {
		totalSize += v.Size
	}
	stats.TotalSize = totalSize
	stats.TotalSizeHuman = humanReadableSize(totalSize)
	return stats
}

// HealthCheck 健康检查
func (sm *SmartManagerV2) HealthCheck() (*HealthCheckResult, error) {
	result := &HealthCheckResult{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	if freeSpace, err := getFreeSpace(sm.config.BackupPath); err == nil {
		result.Details["free_space"] = humanReadableSize(freeSpace)
	}

	sm.mu.RLock()
	result.Details["active_jobs"] = len(sm.activeJobs)
	result.Details["total_jobs"] = len(sm.jobs)
	sm.mu.RUnlock()

	sm.backupIndex.mu.RLock()
	result.Details["total_backups"] = len(sm.backupIndex.versions)
	sm.backupIndex.mu.RUnlock()

	return result, nil
}

// SetScheduler 设置调度器
func (sm *SmartManagerV2) SetScheduler(scheduler *BackupScheduler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.scheduler = scheduler
}

// ============================================================================
// 辅助函数
// ============================================================================

// generateUUIDV2 生成 UUID
func generateUUIDV2() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}
