// Package dataintegrity 提供数据完整性校验功能
package dataintegrity

import (
	"context"
	"golang.org/x/crypto/blake2b"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

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

	// 取消所有运行中的任务
	for id, cancel := range m.jobCancel {
		cancel()
		delete(m.jobCancel, id)
	}

	m.logger.Info("data integrity manager stopped")
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

	// 获取文件信息
	info, err := m.getFileInfo(filePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir {
		return nil, fmt.Errorf("路径是目录而非文件: %s", filePath)
	}

	// 计算校验和
	checksum, err := m.computeHash(ctx, filePath, algo)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	// 构建记录
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

	// 尝试更新已有记录
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
	var errs []error

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
			m.logger.Warn("failed to calculate checksum",
				zap.String("file", fi.Path),
				zap.Error(err),
			)
			errs = append(errs, err)
			continue
		}
		results = append(results, cs)
	}

	if len(errs) > 0 {
		m.logger.Warn("batch checksum completed with errors",
			zap.Int("total", len(files)),
			zap.Int("success", len(results)),
			zap.Int("errors", len(errs)),
		)
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

	// 获取已存储的校验和
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

	// 计算当前校验和
	actualChecksum, err := m.computeHash(ctx, filePath, algo)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	// 对比
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

	// 保存检查记录
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
			m.logger.Debug("verify skipped",
				zap.String("file", fi.Path),
				zap.Error(err),
			)
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
	// 检查是否有运行中的任务
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

	// 持久化
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

	// 保存状态
	if m.store != nil {
		_ = m.store.SaveJob(job)
	}

	// 异步执行
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

	// 列出文件
	files, err := m.listFiles(job.Path, job.Recursive)
	if err != nil {
		m.failJob(job, fmt.Sprintf("列出文件失败: %v", err))
		return
	}

	// 过滤非文件
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

		check, err := m.VerifyFile(ctx, fi.Path)
		m.mu.Lock()
		job.Scanned = i + 1
		job.Progress = float64(job.Scanned) / float64(job.TotalFiles)
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

		if err != nil && err != ErrChecksumNotFound {
			m.logger.Debug("verify error in job",
				zap.String("file", fi.Path),
				zap.Error(err),
			)
		}

		// 定期保存进度
		if i%100 == 0 && m.store != nil {
			_ = m.store.SaveJob(job)
		}
	}

	// 完成
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

	m.logger.Error("integrity job failed",
		zap.Int64("job_id", job.ID),
		zap.String("error", msg),
	)
}

// ========== 修复建议 ==========

// GetRepairSuggestions 获取文件的修复建议.
func (m *Manager) GetRepairSuggestions(ctx context.Context, filePath string) (*RepairSuggestion, error) {
	if filePath == "" {
		return nil, ErrPathRequired
	}

	// 先验证文件
	check, err := m.VerifyFile(ctx, filePath)
	if err != nil && err != ErrChecksumNotFound {
		return nil, err
	}

	suggestion := &RepairSuggestion{
		FilePath: filePath,
	}

	if check == nil || check.Status != StatusCorrupted {
		suggestion.Status = StatusIntact
		suggestion.Strategy = StrategyManual
		suggestion.Description = "文件完整性正常，无需修复"
		return suggestion, nil
	}

	suggestion.Status = StatusCorrupted

	// 查找可用修复来源
	var sources []RepairSource

	// 1. 检查快照
	m.mu.RLock()
	snapProv := m.snapProvider
	repProv := m.repProvider
	m.mu.RUnlock()

	if snapProv != nil {
		snapshots, err := snapProv.ListSnapshots(filePath)
		if err == nil && len(snapshots) > 0 {
			for _, snap := range snapshots {
				sources = append(sources, RepairSource{
					Type:    "snapshot",
					Source:  snap.ID,
					ModTime: snap.CreatedAt.Format(time.RFC3339),
					Size:    snap.Size,
				})
			}
			suggestion.Strategy = StrategySnapshotRestore
			suggestion.Description = fmt.Sprintf("发现 %d 个可用快照，建议从最近的快照恢复", len(snapshots))
		}
	}

	// 2. 检查副本
	if repProv != nil && suggestion.Strategy == "" {
		replicas, err := repProv.ListReplicas(filePath)
		if err == nil && len(replicas) > 0 {
			for _, rep := range replicas {
				if rep.Status == "healthy" || rep.Status == "ok" {
					sources = append(sources, RepairSource{
						Type:    "replica",
						Source:  rep.ID,
						ModTime: rep.ModTime.Format(time.RFC3339),
						Size:    rep.Size,
					})
				}
			}
			if len(sources) > 0 {
				suggestion.Strategy = StrategyReplicaRestore
				suggestion.Description = fmt.Sprintf("发现 %d 个健康副本，建议从副本恢复", len(sources))
			}
		}
	}

	// 3. 无可用来源时建议 Scrub 或人工处理
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
	report := &IntegrityReport{
		GeneratedAt: time.Now(),
	}

	// 获取所有校验和记录
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
			FilePath:  cs.FilePath,
			FileSize:  cs.FileSize,
			Algorithm: cs.Algorithm,
			Checksum:  cs.Checksum,
			LastCheck: cs.UpdatedAt,
		}

		// 验证当前文件
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
				// 获取修复建议
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

	// 创建 hash writer
	var hashWriter interface {
		io.Writer
		Sum(b []byte) []byte
	}

	switch algo {
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

	// 支持 context 取消的读取
	buf := make([]byte, 64*1024) // 64KB buffer
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

	// 默认使用 os
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

	// 默认使用 filepath.Walk
	var files []*FileInfo
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
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
