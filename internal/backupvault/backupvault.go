// Package backupvault 提供备份保险箱核心管理逻辑
package backupvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BackupVault 备份保险箱
type BackupVault struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *VaultConfig
	jobs          map[string]*BackupJob
	restorePoints map[string]*RestorePoint
	chains        map[string]*BackupChain
	stopChan      chan struct{}
	running       bool
}

// NewBackupVault 创建备份保险箱
func NewBackupVault(config *VaultConfig) *BackupVault {
	logger := zap.NewNop()
	if config == nil {
		config = DefaultVaultConfig()
	}

	return &BackupVault{
		logger:        logger,
		config:        config,
		jobs:          make(map[string]*BackupJob),
		restorePoints: make(map[string]*RestorePoint),
		chains:        make(map[string]*BackupChain),
		stopChan:      make(chan struct{}),
	}
}

// NewBackupVaultWithLogger 创建带日志的备份保险箱
func NewBackupVaultWithLogger(logger *zap.Logger, config *VaultConfig) *BackupVault {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultVaultConfig()
	}

	return &BackupVault{
		logger:        logger,
		config:        config,
		jobs:          make(map[string]*BackupJob),
		restorePoints: make(map[string]*RestorePoint),
		chains:        make(map[string]*BackupChain),
		stopChan:      make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateJob 创建备份任务
func (v *BackupVault) CreateJob(job *BackupJob) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if job.ID == "" {
		job.ID = generateID()
	}

	// 验证 3-2-1 策略
	if err := v.validate321Strategy(job); err != nil {
		return fmt.Errorf("3-2-1 strategy validation failed: %w", err)
	}

	// 设置默认值
	if job.Strategy == "" {
		job.Strategy = StrategyFull
	}
	if job.Encryption == nil {
		job.Encryption = DefaultEncryptionConfig()
	}
	if job.Retention == nil {
		job.Retention = DefaultRetentionPolicy()
	}

	job.Status = StatusIdle
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()

	// 计算下次运行时间
	if job.Schedule != nil && job.Schedule.Enabled {
		job.NextRun = v.calculateNextRun(job.Schedule)
	}

	v.jobs[job.ID] = job

	v.logger.Info("backup job created",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("strategy", string(job.Strategy)),
		zap.Int("destinations", len(job.Destinations)))

	return nil
}

// validate321Strategy 验证 3-2-1 备份策略
// 3份数据：至少 3 个目标存储
// 2种介质：至少使用 2 种不同的存储介质
// 1份异地：至少 1 个异地存储
func (v *BackupVault) validate321Strategy(job *BackupJob) error {
	if len(job.Destinations) < 3 {
		return fmt.Errorf("3-2-1 strategy requires at least 3 destinations, got %d", len(job.Destinations))
	}

	// 检查 2 种介质
	mediaTypes := make(map[MediaType]bool)
	hasOffsite := false

	for _, dest := range job.Destinations {
		if !dest.Enabled {
			continue
		}
		mediaTypes[dest.Type] = true
		if dest.IsOffsite {
			hasOffsite = true
		}
	}

	if len(mediaTypes) < 2 {
		return fmt.Errorf("3-2-1 strategy requires at least 2 different media types, got %d", len(mediaTypes))
	}

	if !hasOffsite {
		return fmt.Errorf("3-2-1 strategy requires at least 1 offsite destination")
	}

	return nil
}

// calculateNextRun 计算下次运行时间
func (v *BackupVault) calculateNextRun(schedule *Schedule) time.Time {
	now := time.Now()

	switch schedule.Type {
	case ScheduleHourly:
		return now.Add(time.Duration(schedule.Interval) * time.Hour)
	case ScheduleDaily:
		return now.Add(24 * time.Hour)
	case ScheduleWeekly:
		return now.Add(7 * 24 * time.Hour)
	case ScheduleMonthly:
		return now.AddDate(0, 1, 0)
	default:
		return now.Add(24 * time.Hour)
	}
}

// RunBackup 执行备份任务
func (v *BackupVault) RunBackup(jobID string) (*BackupResult, error) {
	v.mu.Lock()
	job, ok := v.jobs[jobID]
	if !ok {
		v.mu.Unlock()
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status == StatusRunning {
		v.mu.Unlock()
		return nil, fmt.Errorf("job is already running: %s", jobID)
	}

	job.Status = StatusRunning
	job.UpdatedAt = time.Now()
	v.mu.Unlock()

	start := time.Now()
	result := &BackupResult{
		JobID:    jobID,
		Strategy: job.Strategy,
	}

	// 模拟备份执行
	size, checksum, err := v.executeBackup(job)
	if err != nil {
		v.mu.Lock()
		job.Status = StatusFailed
		job.UpdatedAt = time.Now()
		v.mu.Unlock()

		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result, err
	}

	// 创建恢复点
	restorePoint := &RestorePoint{
		ID:          generateID(),
		JobID:       jobID,
		Strategy:    job.Strategy,
		Timestamp:   time.Now(),
		Size:        size,
		Checksum:    checksum,
		Verified:    false,
		Encrypted:   job.Encryption != nil && job.Encryption.Enabled,
		Destination: v.getPrimaryDestination(job),
		CreatedAt:   time.Now(),
	}

	// 存储恢复点
	v.mu.Lock()
	v.restorePoints[restorePoint.ID] = restorePoint

	// 更新备份链
	v.updateBackupChain(job, restorePoint)

	job.Status = StatusSuccess
	job.LastRun = time.Now()
	job.UpdatedAt = time.Now()

	if job.Schedule != nil && job.Schedule.Enabled {
		job.NextRun = v.calculateNextRun(job.Schedule)
	}
	v.mu.Unlock()

	result.RestorePoint = restorePoint
	result.Size = size
	result.Duration = time.Since(start)

	// 备份后自动验证
	if v.config.VerifyAfterBackup {
		verified, _ := v.verifyRestorePoint(restorePoint)
		restorePoint.Verified = verified
		result.Verified = verified
	}

	v.logger.Info("backup completed",
		zap.String("job_id", jobID),
		zap.Int64("size", size),
		zap.Duration("duration", result.Duration),
		zap.Bool("verified", result.Verified))

	return result, nil
}

// executeBackup 执行实际备份（模拟）
func (v *BackupVault) executeBackup(job *BackupJob) (int64, string, error) {
	// 模拟备份数据
	size := int64(1024 * 1024) // 1MB 模拟数据

	// 模拟加密
	if job.Encryption != nil && job.Encryption.Enabled {
		if err := v.encryptData([]byte("backup-data")); err != nil {
			return 0, "", fmt.Errorf("encryption failed: %w", err)
		}
	}

	// 计算校验和
	checksum := sha256.Sum256([]byte(fmt.Sprintf("backup-%s-%d", job.ID, time.Now().UnixNano())))
	return size, hex.EncodeToString(checksum[:]), nil
}

// encryptData 使用 AES-256-GCM 加密数据
func (v *BackupVault) encryptData(plaintext []byte) error {
	if v.config.EncryptionKey == "" {
		return fmt.Errorf("encryption key not configured")
	}

	key := []byte(v.config.EncryptionKey)
	if len(key) != 32 {
		// 填充或截断到 32 字节
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密（结果此处不存储，仅为验证加密流程）
	_ = aesGCM.Seal(nonce, nonce, plaintext, nil)
	return nil
}

// getPrimaryDestination 获取主要存储目标
func (v *BackupVault) getPrimaryDestination(job *BackupJob) string {
	for _, dest := range job.Destinations {
		if dest.Enabled {
			return dest.Path
		}
	}
	return ""
}

// updateBackupChain 更新备份链
func (v *BackupVault) updateBackupChain(job *BackupJob, point *RestorePoint) {
	if job.Strategy == StrategyFull {
		// 创建新的备份链
		chain := &BackupChain{
			ID:          generateID(),
			JobID:       job.ID,
			FullBackup:  point,
			ChainLength: 1,
			TotalSize:   point.Size,
			CreatedAt:   time.Now(),
		}
		point.ChainID = chain.ID
		v.chains[chain.ID] = chain
	} else {
		// 查找最新的备份链
		chain := v.findLatestChain(job.ID)
		if chain != nil {
			point.ChainID = chain.ID
			chain.Incrementals = append(chain.Incrementals, point)
			chain.ChainLength++
			chain.TotalSize += point.Size
		}
	}
}

// findLatestChain 查找最新的备份链
func (v *BackupVault) findLatestChain(jobID string) *BackupChain {
	var latest *BackupChain
	for _, chain := range v.chains {
		if chain.JobID == jobID {
			if latest == nil || chain.CreatedAt.After(latest.CreatedAt) {
				latest = chain
			}
		}
	}
	return latest
}

// Restore 从恢复点恢复数据
func (v *BackupVault) Restore(restorePointID string, dest string) error {
	v.mu.RLock()
	point, ok := v.restorePoints[restorePointID]
	v.mu.RUnlock()

	if !ok {
		return fmt.Errorf("restore point not found: %s", restorePointID)
	}

	v.logger.Info("starting restore",
		zap.String("restore_point_id", restorePointID),
		zap.String("destination", dest),
		zap.Int64("size", point.Size))

	// 模拟恢复过程
	if point.Encrypted {
		v.logger.Info("decrypting backup data")
	}

	v.logger.Info("restore completed",
		zap.String("restore_point_id", restorePointID),
		zap.Int64("bytes_restored", point.Size))

	return nil
}

// Verify 验证备份完整性
func (v *BackupVault) Verify(jobID string) (bool, error) {
	v.mu.RLock()
	job, ok := v.jobs[jobID]
	if !ok {
		v.mu.RUnlock()
		return false, fmt.Errorf("job not found: %s", jobID)
	}
	v.mu.RUnlock()

	// 查找该任务的所有恢复点
	points := v.ListRestorePoints(jobID)
	if len(points) == 0 {
		return false, fmt.Errorf("no restore points found for job: %s", jobID)
	}

	// 验证最新的恢复点
	latest := points[len(points)-1]
	verified, err := v.verifyRestorePoint(latest)
	if err != nil {
		return false, err
	}

	v.logger.Info("backup verification completed",
		zap.String("job_id", jobID),
		zap.String("job_name", job.Name),
		zap.Bool("verified", verified))

	return verified, nil
}

// verifyRestorePoint 验证单个恢复点
func (v *BackupVault) verifyRestorePoint(point *RestorePoint) (bool, error) {
	// 模拟验证：检查校验和
	if point.Checksum == "" {
		return false, fmt.Errorf("restore point has no checksum")
	}

	// 模拟验证通过
	return true, nil
}

// GetComplianceReport 获取合规报告
func (v *BackupVault) GetComplianceReport() *ComplianceReport {
	v.mu.RLock()
	defer v.mu.RUnlock()

	report := &ComplianceReport{
		GeneratedAt: time.Now(),
		TotalJobs:   len(v.jobs),
		Violations:  make([]Violation, 0),
		Summary: &ComplianceSummary{
			TotalRestorePoints: len(v.restorePoints),
		},
	}

	for _, job := range v.jobs {
		compliant := true

		// 检查 3 份数据
		enabledDests := 0
		for _, dest := range job.Destinations {
			if dest.Enabled {
				enabledDests++
			}
		}
		if enabledDests < 3 {
			compliant = false
			report.Violations = append(report.Violations, Violation{
				JobID:       job.ID,
				JobName:     job.Name,
				Rule:        "3-copies",
				Description: fmt.Sprintf("需要至少3个备份目标，当前%d个", enabledDests),
				Severity:    "high",
			})
			report.Summary.TotalCopies = enabledDests
		}

		// 检查 2 种介质
		mediaTypes := make(map[MediaType]bool)
		for _, dest := range job.Destinations {
			if dest.Enabled {
				mediaTypes[dest.Type] = true
			}
		}
		if len(mediaTypes) < 2 {
			compliant = false
			report.Violations = append(report.Violations, Violation{
				JobID:       job.ID,
				JobName:     job.Name,
				Rule:        "2-media",
				Description: fmt.Sprintf("需要至少2种存储介质，当前%d种", len(mediaTypes)),
				Severity:    "medium",
			})
		}
		report.Summary.HasTwoMedia = len(mediaTypes) >= 2

		// 检查 1 份异地
		hasOffsite := false
		for _, dest := range job.Destinations {
			if dest.Enabled && dest.IsOffsite {
				hasOffsite = true
				break
			}
		}
		if !hasOffsite {
			compliant = false
			report.Violations = append(report.Violations, Violation{
				JobID:       job.ID,
				JobName:     job.Name,
				Rule:        "1-offsite",
				Description: "需要至少1个异地备份目标",
				Severity:    "high",
			})
		}
		report.Summary.HasOffsite = hasOffsite

		if compliant {
			report.CompliantJobs++
		} else {
			report.NonCompliant++
		}
	}

	return report
}

// ListRestorePoints 列出任务的所有恢复点
func (v *BackupVault) ListRestorePoints(jobID string) []*RestorePoint {
	v.mu.RLock()
	defer v.mu.RUnlock()

	points := make([]*RestorePoint, 0)
	for _, point := range v.restorePoints {
		if point.JobID == jobID {
			points = append(points, point)
		}
	}

	// 按时间排序
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			if points[i].Timestamp.After(points[j].Timestamp) {
				points[i], points[j] = points[j], points[i]
			}
		}
	}

	return points
}

// SetRetention 设置保留策略
func (v *BackupVault) SetRetention(jobID string, policy *RetentionPolicy) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	job, ok := v.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.Retention = policy
	job.UpdatedAt = time.Now()

	v.logger.Info("retention policy updated",
		zap.String("job_id", jobID),
		zap.Int("keep_last", policy.KeepLast),
		zap.Int("keep_daily", policy.KeepDaily))

	return nil
}

// GetJob 获取备份任务
func (v *BackupVault) GetJob(jobID string) (*BackupJob, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	job, ok := v.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return job, nil
}

// ListJobs 列出所有备份任务
func (v *BackupVault) ListJobs() []*BackupJob {
	v.mu.RLock()
	defer v.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(v.jobs))
	for _, job := range v.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// DeleteJob 删除备份任务
func (v *BackupVault) DeleteJob(jobID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, ok := v.jobs[jobID]; !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	delete(v.jobs, jobID)
	return nil
}

// GetRestorePoint 获取恢复点
func (v *BackupVault) GetRestorePoint(id string) (*RestorePoint, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	point, ok := v.restorePoints[id]
	if !ok {
		return nil, fmt.Errorf("restore point not found: %s", id)
	}
	return point, nil
}

// GetBackupChain 获取备份链
func (v *BackupVault) GetBackupChain(chainID string) (*BackupChain, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	chain, ok := v.chains[chainID]
	if !ok {
		return nil, fmt.Errorf("backup chain not found: %s", chainID)
	}
	return chain, nil
}

// ListBackupChains 列出任务的所有备份链
func (v *BackupVault) ListBackupChains(jobID string) []*BackupChain {
	v.mu.RLock()
	defer v.mu.RUnlock()

	chains := make([]*BackupChain, 0)
	for _, chain := range v.chains {
		if chain.JobID == jobID {
			chains = append(chains, chain)
		}
	}
	return chains
}

// GetConfig 获取配置
func (v *BackupVault) GetConfig() *VaultConfig {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cfg := *v.config
	return &cfg
}

// UpdateConfig 更新配置
func (v *BackupVault) UpdateConfig(config *VaultConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if config != nil {
		v.config = config
	}
}

// RunRestoreDrill 运行恢复演练
func (v *BackupVault) RunRestoreDrill(jobID string) (bool, error) {
	v.mu.RLock()
	job, ok := v.jobs[jobID]
	if !ok {
		v.mu.RUnlock()
		return false, fmt.Errorf("job not found: %s", jobID)
	}
	v.mu.RUnlock()

	points := v.ListRestorePoints(jobID)
	if len(points) == 0 {
		return false, fmt.Errorf("no restore points found for job: %s", jobID)
	}

	latest := points[len(points)-1]

	v.logger.Info("starting restore drill",
		zap.String("job_id", jobID),
		zap.String("job_name", job.Name),
		zap.String("restore_point_id", latest.ID))

	// 模拟恢复演练
	// 1. 验证恢复点完整性
	verified, err := v.verifyRestorePoint(latest)
	if err != nil {
		return false, fmt.Errorf("restore point verification failed: %w", err)
	}

	if !verified {
		return false, fmt.Errorf("restore point integrity check failed")
	}

	// 2. 模拟恢复到临时目录
	v.logger.Info("restore drill completed successfully",
		zap.String("job_id", jobID),
		zap.Int64("data_size", latest.Size))

	return true, nil
}

// CleanupExpired 清理过期的恢复点
func (v *BackupVault) CleanupExpired(jobID string) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	job, ok := v.jobs[jobID]
	if !ok {
		return 0, fmt.Errorf("job not found: %s", jobID)
	}

	if job.Retention == nil {
		return 0, nil
	}

	cleaned := 0
	maxAge := time.Duration(job.Retention.MaxAgeDays) * 24 * time.Hour

	for id, point := range v.restorePoints {
		if point.JobID == jobID && time.Since(point.CreatedAt) > maxAge {
			delete(v.restorePoints, id)
			cleaned++
		}
	}

	if cleaned > 0 {
		v.logger.Info("cleaned up expired restore points",
			zap.String("job_id", jobID),
			zap.Int("cleaned", cleaned))
	}

	return cleaned, nil
}
