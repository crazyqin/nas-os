// Package dataintegrity 提供数据完整性校验功能
// 参考 TrueNAS Self-Healing Checksums 设计，提供文件校验和计算、损坏检测、
// 自动修复建议、完整性报告等功能。
package dataintegrity

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
)

// ========== 错误定义 ==========

var (
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrChecksumNotFound 未找到存储的校验和记录.
	ErrChecksumNotFound = errors.New("未找到校验和记录")
	// ErrIntegrityJobNotFound 完整性检查任务不存在.
	ErrIntegrityJobNotFound = errors.New("完整性检查任务不存在")
	// ErrJobAlreadyRunning 已有任务正在运行.
	ErrJobAlreadyRunning = errors.New("已有任务正在运行")
	// ErrJobNotRunning 当前没有运行中的任务.
	ErrJobNotRunning = errors.New("当前没有运行中的任务")
	// ErrInvalidAlgorithm 不支持的校验算法.
	ErrInvalidAlgorithm = errors.New("不支持的校验算法")
	// ErrCorruptionDetected 检测到数据损坏.
	ErrCorruptionDetected = errors.New("检测到数据损坏")
	// ErrRepairFailed 修复失败.
	ErrRepairFailed = errors.New("修复失败")
	// ErrPoolNotFound 存储池不存在.
	ErrPoolNotFound = errors.New("存储池不存在")
	// ErrPathRequired 必须指定路径.
	ErrPathRequired = errors.New("必须指定路径")
)

// ========== 校验算法类型 ==========

// Algorithm 校验算法.
type Algorithm string

const (
	// AlgorithmMD5 MD5 算法（快速，适合非安全场景）.
	AlgorithmMD5 Algorithm = "md5"
	// AlgorithmSHA256 SHA-256 算法（默认）.
	AlgorithmSHA256 Algorithm = "sha256"
	// AlgorithmSHA512 SHA-512 算法.
	AlgorithmSHA512 Algorithm = "sha512"
	// AlgorithmBLAKE2b BLAKE2b-256 算法.
	AlgorithmBLAKE2b Algorithm = "blake2b"
	// AlgorithmCRC32 CRC-32 算法（快速但不防篡改）.
	AlgorithmCRC32 Algorithm = "crc32"
)

// supportedAlgorithms 支持的算法集合.
var supportedAlgorithms = map[Algorithm]bool{
	AlgorithmMD5:     true,
	AlgorithmSHA256:  true,
	AlgorithmSHA512:  true,
	AlgorithmBLAKE2b: true,
	AlgorithmCRC32:   true,
}

// IsSupportedAlgorithm 检查算法是否受支持.
func IsSupportedAlgorithm(algo Algorithm) bool {
	return supportedAlgorithms[algo]
}

// ========== 完整性状态类型 ==========

// IntegrityStatus 完整性状态.
type IntegrityStatus string

const (
	// StatusIntact 完整.
	StatusIntact IntegrityStatus = "intact"
	// StatusCorrupted 损坏.
	StatusCorrupted IntegrityStatus = "corrupted"
	// StatusUnknown 未知（未校验过）.
	StatusUnknown IntegrityStatus = "unknown"
	// StatusRepaired 已修复.
	StatusRepaired IntegrityStatus = "repaired"
	// StatusRepairFailed 修复失败.
	StatusRepairFailed IntegrityStatus = "repair_failed"
)

// ========== 任务状态类型 ==========

// JobState 完整性检查任务状态.
type JobState string

const (
	// JobStatePending 等待执行.
	JobStatePending JobState = "pending"
	// JobStateRunning 运行中.
	JobStateRunning JobState = "running"
	// JobStateCompleted 已完成.
	JobStateCompleted JobState = "completed"
	// JobStateFailed 失败.
	JobStateFailed JobState = "failed"
	// JobStateCancelled 已取消.
	JobStateCancelled JobState = "cancelled"
)

// ========== 核心数据结构 ==========

// FileChecksum 文件校验和记录.
type FileChecksum struct {
	ID        int64     `json:"id"`
	FilePath  string    `json:"file_path"`
	Algorithm Algorithm `json:"algorithm"`
	Checksum  string    `json:"checksum"`
	FileSize  int64     `json:"file_size"`
	ModTime   time.Time `json:"mod_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IntegrityCheck 完整性检查记录.
type IntegrityCheck struct {
	ID           int64           `json:"id"`
	FilePath     string          `json:"file_path"`
	Algorithm    Algorithm       `json:"algorithm"`
	ExpectedHash string          `json:"expected_hash"`
	ActualHash   string          `json:"actual_hash"`
	Status       IntegrityStatus `json:"status"`
	Message      string          `json:"message"`
	CheckedAt    time.Time       `json:"checked_at"`
}

// RepairSuggestion 修复建议.
type RepairSuggestion struct {
	FilePath    string          `json:"file_path"`
	Status      IntegrityStatus `json:"status"`
	Strategy    RepairStrategy  `json:"strategy"`
	Description string          `json:"description"`
	Sources     []RepairSource  `json:"sources,omitempty"`
}

// RepairStrategy 修复策略.
type RepairStrategy string

const (
	// StrategySnapshotRestore 从快照恢复.
	StrategySnapshotRestore RepairStrategy = "snapshot_restore"
	// StrategyReplicaRestore 从副本恢复.
	StrategyReplicaRestore RepairStrategy = "replica_restore"
	// StrategyBackupRestore 从备份恢复.
	StrategyBackupRestore RepairStrategy = "backup_restore"
	// StrategyScrubRepair 通过ZFS Scrub修复.
	StrategyScrubRepair RepairStrategy = "scrub_repair"
	// StrategyManual 人工处理.
	StrategyManual RepairStrategy = "manual"
)

// RepairSource 修复来源.
type RepairSource struct {
	Type    string `json:"type"`
	Source  string `json:"source"`
	ModTime string `json:"mod_time"`
	Size    int64  `json:"size"`
}

// IntegrityJob 完整性检查任务.
type IntegrityJob struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Algorithm  Algorithm `json:"algorithm"`
	Recursive  bool      `json:"recursive"`
	State      JobState  `json:"state"`
	Progress   float64   `json:"progress"`
	TotalFiles int       `json:"total_files"`
	Scanned    int       `json:"scanned"`
	Intact     int       `json:"intact"`
	Corrupted  int       `json:"corrupted"`
	Unknown    int       `json:"unknown"`
	ErrorMsg   string    `json:"error_msg,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at,omitempty"`
}

// IntegrityReport 完整性报告.
type IntegrityReport struct {
	GeneratedAt  time.Time            `json:"generated_at"`
	Summary      ReportSummary        `json:"summary"`
	Files        []FileIntegrityEntry `json:"files"`
	RepairNeeded []RepairSuggestion   `json:"repair_needed,omitempty"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	TotalFiles   int       `json:"total_files"`
	Intact       int       `json:"intact"`
	Corrupted    int       `json:"corrupted"`
	Unknown      int       `json:"unknown"`
	Repaired     int       `json:"repaired"`
	TotalSize    int64     `json:"total_size"`
	LastScanTime time.Time `json:"last_scan_time"`
}

// FileIntegrityEntry 文件完整性条目.
type FileIntegrityEntry struct {
	FilePath  string          `json:"file_path"`
	FileSize  int64           `json:"file_size"`
	Algorithm Algorithm       `json:"algorithm"`
	Checksum  string          `json:"checksum"`
	Status    IntegrityStatus `json:"status"`
	LastCheck time.Time       `json:"last_check"`
}

// ========== 请求/响应结构 ==========

// CalculateChecksumRequest 计算校验和请求.
type CalculateChecksumRequest struct {
	FilePath  string    `json:"file_path" binding:"required"`
	Algorithm Algorithm `json:"algorithm"`
}

// VerifyFileRequest 校验文件请求.
type VerifyFileRequest struct {
	FilePath string `json:"file_path" binding:"required"`
}

// CreateJobRequest 创建完整性检查任务请求.
type CreateJobRequest struct {
	Name      string    `json:"name" binding:"required"`
	Path      string    `json:"path" binding:"required"`
	Algorithm Algorithm `json:"algorithm"`
	Recursive bool      `json:"recursive"`
}

// RepairRequest 修复请求.
type RepairRequest struct {
	FilePath string         `json:"file_path" binding:"required"`
	Strategy RepairStrategy `json:"strategy" binding:"required"`
	Source   string         `json:"source"`
}

// ReportRequest 报告请求.
type ReportRequest struct {
	Path      string    `json:"path"`
	Algorithm Algorithm `json:"algorithm"`
}

// ========== 接口定义 ==========

// ChecksumStore 校验和存储接口.
type ChecksumStore interface {
	SaveChecksum(cs *FileChecksum) error
	GetChecksum(filePath string, algo Algorithm) (*FileChecksum, error)
	ListChecksums(pathPrefix string, algo Algorithm) ([]*FileChecksum, error)
	DeleteChecksum(filePath string, algo Algorithm) error
	SaveIntegrityCheck(check *IntegrityCheck) error
	GetIntegrityHistory(filePath string, limit int) ([]*IntegrityCheck, error)
	SaveJob(job *IntegrityJob) error
	GetJob(jobID int64) (*IntegrityJob, error)
	ListJobs(limit int) ([]*IntegrityJob, error)
}

// FileSystemProvider 文件系统接口.
type FileSystemProvider interface {
	Stat(path string) (*FileInfo, error)
	ListDir(path string) ([]*FileInfo, error)
	ReadFile(path string, offset int64, size int) ([]byte, error)
	GetFileSize(path string) (int64, error)
}

// SnapshotProvider 快照提供者接口.
type SnapshotProvider interface {
	ListSnapshots(path string) ([]*SnapshotInfo, error)
	RestoreFromSnapshot(snapshotID string, filePath string) error
}

// SnapshotInfo 快照信息.
type SnapshotInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

// ReplicationProvider 副本提供者接口.
type ReplicationProvider interface {
	ListReplicas(filePath string) ([]*ReplicaInfo, error)
	RestoreFromReplica(replicaID string, targetPath string) error
}

// ReplicaInfo 副本信息.
type ReplicaInfo struct {
	ID      string    `json:"id"`
	Source  string    `json:"source"`
	Target  string    `json:"target"`
	Status  string    `json:"status"`
	ModTime time.Time `json:"mod_time"`
	Size    int64     `json:"size"`
}

// FileInfo 文件信息.
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

// ========== Manager ==========

// Manager 数据完整性校验管理器.
type Manager struct {
	mu           sync.RWMutex
	store        ChecksumStore
	fs           FileSystemProvider
	snapProvider SnapshotProvider
	repProvider  ReplicationProvider
	logger       *zap.Logger
	jobs         map[int64]*IntegrityJob
	jobCancel    map[int64]context.CancelFunc
	nextJobID    int64
	stopCh       chan struct{}
	running      bool
	defaultAlgo  Algorithm
}

// NewManager 创建数据完整性管理器.
func NewManager(store ChecksumStore, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		store:       store,
		logger:      logger,
		jobs:        make(map[int64]*IntegrityJob),
		jobCancel:   make(map[int64]context.CancelFunc),
		stopCh:      make(chan struct{}),
		defaultAlgo: AlgorithmSHA256,
	}
}

// SetFileSystemProvider 设置文件系统提供者.
func (m *Manager) SetFileSystemProvider(fs FileSystemProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fs = fs
}

// SetSnapshotProvider 设置快照提供者.
func (m *Manager) SetSnapshotProvider(sp SnapshotProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapProvider = sp
}

// SetReplicationProvider 设置副本提供者.
func (m *Manager) SetReplicationProvider(rp ReplicationProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repProvider = rp
}

// SetDefaultAlgorithm 设置默认校验算法.
func (m *Manager) SetDefaultAlgorithm(algo Algorithm) error {
	if !IsSupportedAlgorithm(algo) {
		return ErrInvalidAlgorithm
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultAlgo = algo
	return nil
}

// GetDefaultAlgorithm 获取默认校验算法.
func (m *Manager) GetDefaultAlgorithm() Algorithm {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultAlgo
}

// Start 启动管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()
	m.logger.Info("data integrity manager started")
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	for id, cancel := range m.jobCancel {
		cancel()
		delete(m.jobCancel, id)
	}
	m.logger.Info("data integrity manager stopped")
}

// IsRunning 返回管理器是否正在运行.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== 校验和计算 ==========

// CalculateChecksum 计算文件校验和并存储.
func (m *Manager) CalculateChecksum(ctx context.Context, filePath string, algo Algorithm) (*FileChecksum, error) {
	if filePath == "" {
		return nil, ErrPathRequired
	}
	if algo == "" {
		m.mu.RLock()
		algo = m.defaultAlgo
		m.mu.RUnlock()
	}
	if !IsSupportedAlgorithm(algo) {
		return nil, ErrInvalidAlgorithm
	}

	info, err := m.getFileInfo(filePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir {
		return nil, fmt.Errorf("路径是目录而非文件: %s", filePath)
	}

	checksum, err := m.computeHash(ctx, filePath, algo)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	now := time.Now()
	cs := &FileChecksum{
		FilePath:  filePath,
		Algorithm: algo,
		Checksum:  checksum,
		FileSize:  info.Size,
		ModTime:   info.ModTime,
		CreatedAt: now,
		UpdatedAt: now,
	}

	existing, err := m.store.GetChecksum(filePath, algo)
	if err == nil && existing != nil {
		cs.ID = existing.ID
		cs.CreatedAt = existing.CreatedAt
	}

	if err := m.store.SaveChecksum(cs); err != nil {
		return nil, fmt.Errorf("保存校验和失败: %w", err)
	}

	m.logger.Info("checksum calculated",
		zap.String("file", filePath),
		zap.String("algorithm", string(algo)),
		zap.String("checksum", checksum),
	)
	return cs, nil
}

// CalculateChecksumBatch 批量计算目录下文件的校验和.
func (m *Manager) CalculateChecksumBatch(ctx context.Context, dirPath string, algo Algorithm, recursive bool) ([]*FileChecksum, error) {
	if dirPath == "" {
		return nil, ErrPathRequired
	}
	if algo == "" {
		m.mu.RLock()
		algo = m.defaultAlgo
		m.mu.RUnlock()
	}

	files, err := m.listFiles(dirPath, recursive)
	if err != nil {
		return nil, err
	}

	var results []*FileChecksum
	for _, fi := range files {
		if fi.IsDir {
			continue
		}
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		cs, err := m.CalculateChecksum(ctx, fi.Path, algo)
		if err != nil {
			m.logger.Warn("failed to calculate checksum", zap.String("file", fi.Path), zap.Error(err))
			continue
		}
		results = append(results, cs)
	}
	return results, nil
}

// ========== 文件验证 ==========

// VerifyFile 验证单个文件的完整性.
func (m *Manager) VerifyFile(ctx context.Context, filePath string) (*IntegrityCheck, error) {
	if filePath == "" {
		return nil, ErrPathRequired
	}

	m.mu.RLock()
	algo := m.defaultAlgo
	m.mu.RUnlock()

	stored, err := m.store.GetChecksum(filePath, algo)
	if err != nil {
		check := &IntegrityCheck{
			FilePath:  filePath,
			Algorithm: algo,
			Status:    StatusUnknown,
			Message:   "未找到已存储的校验和记录",
			CheckedAt: time.Now(),
		}
		return check, ErrChecksumNotFound
	}

	actualChecksum, err := m.computeHash(ctx, filePath, algo)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	now := time.Now()
	check := &IntegrityCheck{
		FilePath:     filePath,
		Algorithm:    algo,
		ExpectedHash: stored.Checksum,
		ActualHash:   actualChecksum,
		CheckedAt:    now,
	}

	if stored.Checksum == actualChecksum {
		check.Status = StatusIntact
		check.Message = "文件完整性校验通过"
	} else {
		check.Status = StatusCorrupted
		check.Message = fmt.Sprintf("文件已损坏: 预期 %s, 实际 %s", stored.Checksum, actualChecksum)
		m.logger.Error("file corruption detected",
			zap.String("file", filePath),
			zap.String("expected", stored.Checksum),
			zap.String("actual", actualChecksum),
		)
	}

	_ = m.store.SaveIntegrityCheck(check)
	return check, nil
}

// VerifyDirectory 验证目录下所有文件的完整性.
func (m *Manager) VerifyDirectory(ctx context.Context, dirPath string, recursive bool) ([]*IntegrityCheck, error) {
	if dirPath == "" {
		return nil, ErrPathRequired
	}

	files, err := m.listFiles(dirPath, recursive)
	if err != nil {
		return nil, err
	}

	var results []*IntegrityCheck
	for _, fi := range files {
		if fi.IsDir {
			continue
		}
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		check, err := m.VerifyFile(ctx, fi.Path)
		if check != nil {
			results = append(results, check)
		}
		if err != nil {
			m.logger.Debug("verify skipped", zap.String("file", fi.Path), zap.Error(err))
		}
	}
	return results, nil
}

// ========== 完整性检查任务 ==========

// CreateJob 创建完整性检查任务.
func (m *Manager) CreateJob(req CreateJobRequest) (*IntegrityJob, error) {
	if req.Path == "" {
		return nil, ErrPathRequired
	}
	if req.Algorithm == "" {
		m.mu.RLock()
		req.Algorithm = m.defaultAlgo
		m.mu.RUnlock()
	}
	if !IsSupportedAlgorithm(req.Algorithm) {
		return nil, ErrInvalidAlgorithm
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("integrity-check-%d", time.Now().Unix())
	}

	m.mu.Lock()
	for _, j := range m.jobs {
		if j.State == JobStateRunning {
			m.mu.Unlock()
			return nil, ErrJobAlreadyRunning
		}
	}

	m.nextJobID++
	job := &IntegrityJob{
		ID:        m.nextJobID,
		Name:      req.Name,
		Path:      req.Path,
		Algorithm: req.Algorithm,
		Recursive: req.Recursive,
		State:     JobStatePending,
		Progress:  0,
	}
	m.jobs[job.ID] = job
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.SaveJob(job)
	}

	m.logger.Info("integrity job created",
		zap.Int64("job_id", job.ID),
		zap.String("path", req.Path),
		zap.String("algorithm", string(req.Algorithm)),
	)
	return job, nil
}

// StartJob 启动完整性检查任务.
func (m *Manager) StartJob(jobID int64) error {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return ErrIntegrityJobNotFound
	}
	if job.State == JobStateRunning {
		m.mu.Unlock()
		return ErrJobAlreadyRunning
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.jobCancel[jobID] = cancel
	job.State = JobStateRunning
	job.StartedAt = time.Now()
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.SaveJob(job)
	}

	go m.runJob(ctx, job, cancel)
	return nil
}

// CancelJob 取消任务.
func (m *Manager) CancelJob(jobID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cancel, ok := m.jobCancel[jobID]
	if !ok {
		return ErrJobNotRunning
	}

	cancel()
	delete(m.jobCancel, jobID)

	job := m.jobs[jobID]
	if job != nil {
		job.State = JobStateCancelled
		job.EndedAt = time.Now()
		if m.store != nil {
			_ = m.store.SaveJob(job)
		}
	}
	return nil
}

// GetJob 获取任务信息.
func (m *Manager) GetJob(jobID int64) (*IntegrityJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[jobID]
	if !ok {
		return nil, ErrIntegrityJobNotFound
	}
	return job, nil
}

// ListJobs 列出所有任务.
func (m *Manager) ListJobs() []*IntegrityJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*IntegrityJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// runJob 执行完整性检查任务.
func (m *Manager) runJob(ctx context.Context, job *IntegrityJob, cancel context.CancelFunc) {
	defer func() {
		m.mu.Lock()
		delete(m.jobCancel, job.ID)
		m.mu.Unlock()
	}()

	files, err := m.listFiles(job.Path, job.Recursive)
	if err != nil {
		m.failJob(job, fmt.Sprintf("列出文件失败: %v", err))
		return
	}

	var regularFiles []*FileInfo
	for _, fi := range files {
		if !fi.IsDir {
			regularFiles = append(regularFiles, fi)
		}
	}

	m.mu.Lock()
	job.TotalFiles = len(regularFiles)
	m.mu.Unlock()

	m.logger.Info("integrity job scanning",
		zap.Int64("job_id", job.ID),
		zap.Int("total_files", job.TotalFiles),
	)

	for i, fi := range regularFiles {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			job.State = JobStateCancelled
			job.EndedAt = time.Now()
			m.mu.Unlock()
			return
		default:
		}

		check, _ := m.VerifyFile(ctx, fi.Path)
		m.mu.Lock()
		job.Scanned = i + 1
		if job.TotalFiles > 0 {
			job.Progress = float64(job.Scanned) / float64(job.TotalFiles)
		}
		if check != nil {
			switch check.Status {
			case StatusIntact:
				job.Intact++
			case StatusCorrupted:
				job.Corrupted++
			default:
				job.Unknown++
			}
		} else {
			job.Unknown++
		}
		m.mu.Unlock()

		if i%100 == 0 && m.store != nil {
			_ = m.store.SaveJob(job)
		}
	}

	m.mu.Lock()
	job.State = JobStateCompleted
	job.Progress = 1.0
	job.EndedAt = time.Now()
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.SaveJob(job)
	}

	m.logger.Info("integrity job completed",
		zap.Int64("job_id", job.ID),
		zap.Int("total", job.TotalFiles),
		zap.Int("intact", job.Intact),
		zap.Int("corrupted", job.Corrupted),
		zap.Int("unknown", job.Unknown),
	)
}

func (m *Manager) failJob(job *IntegrityJob, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job.State = JobStateFailed
	job.ErrorMsg = msg
	job.EndedAt = time.Now()
	if m.store != nil {
		_ = m.store.SaveJob(job)
	}
	m.logger.Error("integrity job failed", zap.Int64("job_id", job.ID), zap.String("error", msg))
}

// ========== 修复建议 ==========

// GetRepairSuggestions 获取文件的修复建议.
func (m *Manager) GetRepairSuggestions(ctx context.Context, filePath string) (*RepairSuggestion, error) {
	if filePath == "" {
		return nil, ErrPathRequired
	}

	check, err := m.VerifyFile(ctx, filePath)
	if err != nil && err != ErrChecksumNotFound {
		return nil, err
	}

	suggestion := &RepairSuggestion{FilePath: filePath}

	if check == nil || check.Status != StatusCorrupted {
		suggestion.Status = StatusIntact
		suggestion.Strategy = StrategyManual
		suggestion.Description = "文件完整性正常，无需修复"
		return suggestion, nil
	}

	suggestion.Status = StatusCorrupted
	var sources []RepairSource

	m.mu.RLock()
	snapProv := m.snapProvider
	repProv := m.repProvider
	m.mu.RUnlock()

	if snapProv != nil {
		snapshots, err := snapProv.ListSnapshots(filePath)
		if err == nil && len(snapshots) > 0 {
			for _, snap := range snapshots {
				sources = append(sources, RepairSource{
					Type: "snapshot", Source: snap.ID,
					ModTime: snap.CreatedAt.Format(time.RFC3339), Size: snap.Size,
				})
			}
			suggestion.Strategy = StrategySnapshotRestore
			suggestion.Description = fmt.Sprintf("发现 %d 个可用快照，建议从最近的快照恢复", len(snapshots))
		}
	}

	if repProv != nil && suggestion.Strategy == "" {
		replicas, err := repProv.ListReplicas(filePath)
		if err == nil && len(replicas) > 0 {
			for _, rep := range replicas {
				if rep.Status == "healthy" || rep.Status == "ok" {
					sources = append(sources, RepairSource{
						Type: "replica", Source: rep.ID,
						ModTime: rep.ModTime.Format(time.RFC3339), Size: rep.Size,
					})
				}
			}
			if len(sources) > 0 {
				suggestion.Strategy = StrategyReplicaRestore
				suggestion.Description = fmt.Sprintf("发现 %d 个健康副本，建议从副本恢复", len(sources))
			}
		}
	}

	if suggestion.Strategy == "" {
		suggestion.Strategy = StrategyScrubRepair
		suggestion.Description = "未找到可用的快照或副本，建议运行 ZFS Scrub 尝试自动修复"
	}

	suggestion.Sources = sources
	return suggestion, nil
}

// ========== 完整性报告 ==========

// GenerateReport 生成完整性报告.
func (m *Manager) GenerateReport(ctx context.Context, req ReportRequest) (*IntegrityReport, error) {
	report := &IntegrityReport{GeneratedAt: time.Now()}

	var algo Algorithm
	if req.Algorithm != "" {
		algo = req.Algorithm
	} else {
		m.mu.RLock()
		algo = m.defaultAlgo
		m.mu.RUnlock()
	}

	checksums, err := m.store.ListChecksums(req.Path, algo)
	if err != nil {
		return nil, fmt.Errorf("获取校验和记录失败: %w", err)
	}

	var entries []FileIntegrityEntry
	var corrupted []RepairSuggestion
	summary := ReportSummary{}

	for _, cs := range checksums {
		entry := FileIntegrityEntry{
			FilePath: cs.FilePath, FileSize: cs.FileSize,
			Algorithm: cs.Algorithm, Checksum: cs.Checksum, LastCheck: cs.UpdatedAt,
		}

		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}

		check, err := m.VerifyFile(ctx, cs.FilePath)
		if err != nil && err != ErrChecksumNotFound {
			entry.Status = StatusUnknown
			summary.Unknown++
		} else if check != nil {
			entry.Status = check.Status
			entry.LastCheck = check.CheckedAt
			switch check.Status {
			case StatusIntact:
				summary.Intact++
			case StatusCorrupted:
				summary.Corrupted++
				suggestion, sugErr := m.GetRepairSuggestions(ctx, cs.FilePath)
				if sugErr == nil {
					corrupted = append(corrupted, *suggestion)
				}
			default:
				summary.Unknown++
			}
		} else {
			entry.Status = StatusUnknown
			summary.Unknown++
		}

		summary.TotalSize += cs.FileSize
		entries = append(entries, entry)
	}

	summary.TotalFiles = len(entries)
	report.Summary = summary
	report.Files = entries
	report.RepairNeeded = corrupted
	return report, nil
}

// ========== 文件历史 ==========

// GetFileHistory 获取文件的完整性检查历史.
func (m *Manager) GetFileHistory(filePath string, limit int) ([]*IntegrityCheck, error) {
	if filePath == "" {
		return nil, ErrPathRequired
	}
	if limit <= 0 {
		limit = 20
	}
	return m.store.GetIntegrityHistory(filePath, limit)
}

// ListChecksums 列出已存储的校验和记录.
func (m *Manager) ListChecksums(pathPrefix string, algo Algorithm) ([]*FileChecksum, error) {
	if algo == "" {
		m.mu.RLock()
		algo = m.defaultAlgo
		m.mu.RUnlock()
	}
	return m.store.ListChecksums(pathPrefix, algo)
}

// DeleteChecksum 删除校验和记录.
func (m *Manager) DeleteChecksum(filePath string, algo Algorithm) error {
	if filePath == "" {
		return ErrPathRequired
	}
	if algo == "" {
		m.mu.RLock()
		algo = m.defaultAlgo
		m.mu.RUnlock()
	}
	return m.store.DeleteChecksum(filePath, algo)
}

// ========== 内部辅助函数 ==========

// computeHash 计算文件校验和.
func (m *Manager) computeHash(ctx context.Context, filePath string, algo Algorithm) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrFileNotFound
		}
		return "", err
	}
	defer f.Close()

	var hashWriter interface {
		io.Writer
		Sum(b []byte) []byte
	}

	switch algo {
	case AlgorithmMD5:
		hashWriter = md5.New()
	case AlgorithmSHA256:
		hashWriter = sha256.New()
	case AlgorithmSHA512:
		hashWriter = sha512.New()
	case AlgorithmBLAKE2b:
		b2b, err := blake2b.New256(nil)
		if err != nil {
			return "", fmt.Errorf("创建BLAKE2b哈希失败: %w", err)
		}
		hashWriter = b2b
	case AlgorithmCRC32:
		hashWriter = crc32.NewIEEE()
	default:
		return "", ErrInvalidAlgorithm
	}

	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err := hashWriter.Write(buf[:n]); err != nil {
				return "", fmt.Errorf("写入哈希失败: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("读取文件失败: %w", readErr)
		}
	}
	return hex.EncodeToString(hashWriter.Sum(nil)), nil
}

// getFileInfo 获取文件信息.
func (m *Manager) getFileInfo(path string) (*FileInfo, error) {
	m.mu.RLock()
	fs := m.fs
	m.mu.RUnlock()

	if fs != nil {
		return fs.Stat(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	return &FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// listFiles 列出目录下的文件.
func (m *Manager) listFiles(dirPath string, recursive bool) ([]*FileInfo, error) {
	m.mu.RLock()
	fs := m.fs
	m.mu.RUnlock()

	if fs != nil {
		return fs.ListDir(dirPath)
	}

	var files []*FileInfo
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		files = append(files, &FileInfo{
			Name:    info.Name(),
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		})
		if !recursive && info.IsDir() && path != dirPath {
			return filepath.SkipDir
		}
		return nil
	}

	if err := filepath.Walk(dirPath, walkFn); err != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", err)
	}
	return files, nil
}
