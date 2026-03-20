package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// VerificationConfig 验证配置
type VerificationConfig struct {
	// 验证方式
	VerifyChecksum  bool `json:"verify_checksum"`  // 校验和验证
	VerifyStructure bool `json:"verify_structure"` // 结构验证
	VerifyIntegrity bool `json:"verify_integrity"` // 完整性验证
	VerifyDecrypt   bool `json:"verify_decrypt"`   // 解密验证（如果加密）

	// 验证深度
	SampleRate float64       `json:"sample_rate"` // 抽样率 (0-1)
	MaxFiles   int           `json:"max_files"`   // 最大文件数
	Timeout    time.Duration `json:"timeout"`     // 超时时间

	// 自动修复
	AutoRepair bool `json:"auto_repair"` // 自动修复
	MaxRetries int  `json:"max_retries"` // 最大重试次数
}

// VerificationResult 验证结果
type VerificationResult struct {
	SnapshotID    string                `json:"snapshot_id"`
	StartTime     time.Time             `json:"start_time"`
	EndTime       time.Time             `json:"end_time"`
	Duration      time.Duration         `json:"duration"`
	Status        VerificationStatus    `json:"status"`
	TotalFiles    int                   `json:"total_files"`
	VerifiedFiles int                   `json:"verified_files"`
	FailedFiles   int                   `json:"failed_files"`
	SkippedFiles  int                   `json:"skipped_files"`
	TotalSize     int64                 `json:"total_size"`
	VerifiedSize  int64                 `json:"verified_size"`
	Errors        []VerificationError   `json:"errors,omitempty"`
	Warnings      []VerificationWarning `json:"warnings,omitempty"`
	RepairedFiles int                   `json:"repaired_files"`
}

// VerificationStatus 验证状态
type VerificationStatus string

const (
	// VerificationStatusPassed 验证通过
	VerificationStatusPassed VerificationStatus = "passed"
	// VerificationStatusFailed 验证失败
	VerificationStatusFailed VerificationStatus = "failed"
	// VerificationStatusPartial 部分验证
	VerificationStatusPartial VerificationStatus = "partial"
	// VerificationStatusTimeout 验证超时
	VerificationStatusTimeout VerificationStatus = "timeout"
	// VerificationStatusCancelled 验证取消
	VerificationStatusCancelled VerificationStatus = "cancelled"
)

// VerificationError 验证错误
type VerificationError struct {
	Path     string `json:"path"`
	Type     string `json:"type"` // checksum, structure, integrity, decrypt
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// VerificationWarning 验证警告
type VerificationWarning struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// VerificationManager 验证管理器
type VerificationManager struct {
	config        *VerificationConfig
	backupManager *IncrementalBackup
	encryption    *EncryptionManager
	results       map[string]*VerificationResult
	mu            sync.RWMutex
	logger        *zap.Logger
}

// NewVerificationManager 创建验证管理器
func NewVerificationManager(config *VerificationConfig, backup *IncrementalBackup, encryption *EncryptionManager, logger *zap.Logger) *VerificationManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &VerificationManager{
		config:        config,
		backupManager: backup,
		encryption:    encryption,
		results:       make(map[string]*VerificationResult),
		logger:        logger,
	}
}

// VerifySnapshot 验证快照
func (vm *VerificationManager) VerifySnapshot(ctx context.Context, snapshotID string) (*VerificationResult, error) {
	if vm.backupManager == nil {
		return nil, errors.New("backup manager is nil")
	}
	snapshot, err := vm.backupManager.GetSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	result := &VerificationResult{
		SnapshotID: snapshotID,
		StartTime:  time.Now(),
		Status:     VerificationStatusPassed,
		Errors:     make([]VerificationError, 0),
		Warnings:   make([]VerificationWarning, 0),
	}

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		vm.mu.Lock()
		vm.results[snapshotID] = result
		vm.mu.Unlock()
	}()

	vm.logger.Info("Starting snapshot verification",
		zap.String("snapshot_id", snapshotID),
		zap.Int("files", len(snapshot.Files)),
	)

	// 验证文件
	for path, fileInfo := range snapshot.Files {
		select {
		case <-ctx.Done():
			result.Status = VerificationStatusCancelled
			return result, ctx.Err()
		default:
		}

		// 检查文件数限制
		if vm.config.MaxFiles > 0 && result.VerifiedFiles >= vm.config.MaxFiles {
			result.SkippedFiles++
			continue
		}

		// 抽样检查
		if vm.config.SampleRate < 1.0 && !vm.shouldVerify(result.TotalFiles) {
			result.SkippedFiles++
			continue
		}

		result.TotalFiles++
		result.TotalSize += fileInfo.Size

		// 执行验证
		if err := vm.verifyFile(ctx, path, fileInfo, snapshot); err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, *err)

			// 尝试修复
			if vm.config.AutoRepair {
				if repaired := vm.repairFile(path, fileInfo, snapshot); repaired {
					result.RepairedFiles++
					result.FailedFiles--
				}
			}
		} else {
			result.VerifiedFiles++
			result.VerifiedSize += fileInfo.Size
		}
	}

	// 确定最终状态
	if result.FailedFiles > 0 && result.VerifiedFiles > 0 {
		result.Status = VerificationStatusPartial
	} else if result.FailedFiles > 0 {
		result.Status = VerificationStatusFailed
	}

	vm.logger.Info("Snapshot verification completed",
		zap.String("snapshot_id", snapshotID),
		zap.String("status", string(result.Status)),
		zap.Int("verified", result.VerifiedFiles),
		zap.Int("failed", result.FailedFiles),
	)

	return result, nil
}

// verifyFile 验证单个文件
func (vm *VerificationManager) verifyFile(ctx context.Context, path string, info FileInfo, snapshot *Snapshot) *VerificationError {
	// 结构验证
	if vm.config.VerifyStructure {
		if err := vm.verifyStructure(path, info); err != nil {
			return err
		}
	}

	// 校验和验证
	if vm.config.VerifyChecksum && info.Checksum != "" {
		if err := vm.verifyChecksum(path, info); err != nil {
			return err
		}
	}

	// 完整性验证
	if vm.config.VerifyIntegrity {
		if err := vm.verifyIntegrity(path, info, snapshot); err != nil {
			return err
		}
	}

	// 解密验证
	if vm.config.VerifyDecrypt && vm.encryption != nil {
		if err := vm.verifyDecryption(path, info); err != nil {
			return err
		}
	}

	return nil
}

// verifyStructure 验证文件结构
func (vm *VerificationManager) verifyStructure(path string, info FileInfo) *VerificationError {
	// 检查文件是否存在
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &VerificationError{
				Path:    path,
				Type:    "structure",
				Message: "file not found",
			}
		}
		return &VerificationError{
			Path:    path,
			Type:    "structure",
			Message: fmt.Sprintf("cannot open file: %v", err),
		}
	}
	defer func() { _ = file.Close() }()

	// 检查文件大小
	stat, err := file.Stat()
	if err != nil {
		return &VerificationError{
			Path:    path,
			Type:    "structure",
			Message: fmt.Sprintf("cannot stat file: %v", err),
		}
	}

	if stat.Size() != info.Size {
		return &VerificationError{
			Path:     path,
			Type:     "structure",
			Message:  "file size mismatch",
			Expected: fmt.Sprintf("%d", info.Size),
			Actual:   fmt.Sprintf("%d", stat.Size()),
		}
	}

	// 检查文件模式
	if stat.Mode() != info.Mode {
		// 文件模式不匹配通常只是警告
		return nil
	}

	return nil
}

// verifyChecksum 验证校验和
func (vm *VerificationManager) verifyChecksum(path string, info FileInfo) *VerificationError {
	file, err := os.Open(path)
	if err != nil {
		return &VerificationError{
			Path:    path,
			Type:    "checksum",
			Message: fmt.Sprintf("cannot open file: %v", err),
		}
	}
	defer func() { _ = file.Close() }()

	// 计算校验和
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return &VerificationError{
			Path:    path,
			Type:    "checksum",
			Message: fmt.Sprintf("cannot compute checksum: %v", err),
		}
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != info.Checksum {
		return &VerificationError{
			Path:     path,
			Type:     "checksum",
			Message:  "checksum mismatch",
			Expected: info.Checksum,
			Actual:   actual,
		}
	}

	return nil
}

// verifyIntegrity 验证完整性
func (vm *VerificationManager) verifyIntegrity(path string, info FileInfo, snapshot *Snapshot) *VerificationError {
	// 验证数据块
	for _, chunkID := range info.Chunks {
		if _, exists := vm.backupManager.chunkStore.Get(chunkID); !exists {
			return &VerificationError{
				Path:    path,
				Type:    "integrity",
				Message: fmt.Sprintf("missing chunk: %s", chunkID),
			}
		}
	}

	return nil
}

// verifyDecryption 验证解密
func (vm *VerificationManager) verifyDecryption(path string, info FileInfo) *VerificationError {
	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return &VerificationError{
			Path:    path,
			Type:    "decrypt",
			Message: fmt.Sprintf("cannot read file: %v", err),
		}
	}

	// 尝试解密（使用第一个可用的密钥）
	keyIDs := vm.encryption.ListKeys()
	if len(keyIDs) == 0 {
		return &VerificationError{
			Path:    path,
			Type:    "decrypt",
			Message: "no encryption key available",
		}
	}

	for _, keyID := range keyIDs {
		_, err := vm.encryption.Decrypt(data, keyID)
		if err == nil {
			return nil // 解密成功
		}
	}

	return &VerificationError{
		Path:    path,
		Type:    "decrypt",
		Message: "decryption failed with all available keys",
	}
}

// shouldVerify 决定是否验证（抽样）
func (vm *VerificationManager) shouldVerify(index int) bool {
	// 使用简单的随机抽样逻辑
	return float64(index%100)/100.0 < vm.config.SampleRate
}

// repairFile 修复文件
func (vm *VerificationManager) repairFile(path string, info FileInfo, snapshot *Snapshot) bool {
	vm.logger.Debug("Attempting to repair file", zap.String("path", path))

	// 尝试从备份恢复文件
	for _, chunkID := range info.Chunks {
		data, exists := vm.backupManager.chunkStore.Get(chunkID)
		if !exists {
			continue
		}

		// 写入文件
		if err := os.WriteFile(path, data, info.Mode); err != nil {
			vm.logger.Warn("Failed to repair file",
				zap.String("path", path),
				zap.Error(err),
			)
			return false
		}

		// 验证修复结果
		if err := vm.verifyChecksum(path, info); err == nil {
			vm.logger.Info("File repaired successfully", zap.String("path", path))
			return true
		}
	}

	return false
}

// VerifyAll 验证所有快照
func (vm *VerificationManager) VerifyAll(ctx context.Context) ([]*VerificationResult, error) {
	if vm.backupManager == nil {
		return nil, errors.New("backup manager is nil")
	}
	snapshots := vm.backupManager.ListSnapshots()
	results := make([]*VerificationResult, 0, len(snapshots))

	for _, snapshot := range snapshots {
		result, err := vm.VerifySnapshot(ctx, snapshot.ID)
		if err != nil {
			vm.logger.Warn("Snapshot verification failed",
				zap.String("snapshot_id", snapshot.ID),
				zap.Error(err),
			)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetResult 获取验证结果
func (vm *VerificationManager) GetResult(snapshotID string) (*VerificationResult, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	result, exists := vm.results[snapshotID]
	if !exists {
		return nil, ErrBackupNotFound
	}
	return result, nil
}

// GetResults 获取所有结果
func (vm *VerificationManager) GetResults() []*VerificationResult {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	results := make([]*VerificationResult, 0, len(vm.results))
	for _, result := range vm.results {
		results = append(results, result)
	}
	return results
}

// QuickVerify 快速验证
func (vm *VerificationManager) QuickVerify(ctx context.Context, snapshotID string) (*VerificationResult, error) {
	// 保存原始配置
	originalRate := vm.config.SampleRate
	originalMax := vm.config.MaxFiles

	// 设置快速验证参数
	vm.config.SampleRate = 0.1 // 只验证 10%
	vm.config.MaxFiles = 100   // 最多 100 个文件

	defer func() {
		vm.config.SampleRate = originalRate
		vm.config.MaxFiles = originalMax
	}()

	return vm.VerifySnapshot(ctx, snapshotID)
}

// FullVerify 完整验证
func (vm *VerificationManager) FullVerify(ctx context.Context, snapshotID string) (*VerificationResult, error) {
	// 保存原始配置
	originalRate := vm.config.SampleRate
	originalMax := vm.config.MaxFiles

	// 设置完整验证参数
	vm.config.SampleRate = 1.0 // 验证所有文件
	vm.config.MaxFiles = 0     // 无限制

	defer func() {
		vm.config.SampleRate = originalRate
		vm.config.MaxFiles = originalMax
	}()

	return vm.VerifySnapshot(ctx, snapshotID)
}

// ValidateSnapshot 验证快照是否可用
func (vm *VerificationManager) ValidateSnapshot(snapshotID string) (bool, error) {
	if vm.backupManager == nil {
		return false, errors.New("backup manager is nil")
	}
	snapshot, err := vm.backupManager.GetSnapshot(snapshotID)
	if err != nil {
		return false, err
	}

	// 检查快照状态
	if snapshot.Status != SnapshotStatusCompleted {
		return false, errors.New("snapshot not completed")
	}

	// 检查是否有文件
	if len(snapshot.Files) == 0 {
		return false, errors.New("snapshot has no files")
	}

	// 检查数据块
	for _, chunkID := range snapshot.Chunks {
		if _, exists := vm.backupManager.chunkStore.Get(chunkID); !exists {
			return false, fmt.Errorf("missing chunk: %s", chunkID)
		}
	}

	return true, nil
}

// VerificationStats 验证统计
type VerificationStats struct {
	TotalVerifications int           `json:"total_verifications"`
	PassedCount        int           `json:"passed_count"`
	FailedCount        int           `json:"failed_count"`
	PartialCount       int           `json:"partial_count"`
	LastVerification   time.Time     `json:"last_verification"`
	AverageDuration    time.Duration `json:"average_duration"`
	TotalFilesVerified int           `json:"total_files_verified"`
	TotalFilesFailed   int           `json:"total_files_failed"`
	TotalFilesRepaired int           `json:"total_files_repaired"`
}

// GetStats 获取统计
func (vm *VerificationManager) GetStats() VerificationStats {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	stats := VerificationStats{
		TotalVerifications: len(vm.results),
	}

	var totalDuration time.Duration

	for _, result := range vm.results {
		switch result.Status {
		case VerificationStatusPassed:
			stats.PassedCount++
		case VerificationStatusFailed:
			stats.FailedCount++
		case VerificationStatusPartial:
			stats.PartialCount++
		}

		stats.TotalFilesVerified += result.VerifiedFiles
		stats.TotalFilesFailed += result.FailedFiles
		stats.TotalFilesRepaired += result.RepairedFiles
		totalDuration += result.Duration

		if result.StartTime.After(stats.LastVerification) {
			stats.LastVerification = result.StartTime
		}
	}

	if stats.TotalVerifications > 0 {
		stats.AverageDuration = totalDuration / time.Duration(stats.TotalVerifications)
	}

	return stats
}

// ClearResults 清除结果
func (vm *VerificationManager) ClearResults() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.results = make(map[string]*VerificationResult)
}

// ScheduledVerification 计划验证
type ScheduledVerification struct {
	SnapshotID string        `json:"snapshot_id"`
	Interval   time.Duration `json:"interval"`
	LastRun    time.Time     `json:"last_run"`
	NextRun    time.Time     `json:"next_run"`
	Enabled    bool          `json:"enabled"`
}

// VerificationScheduler 验证调度器
type VerificationScheduler struct {
	verifier  *VerificationManager
	schedules map[string]*ScheduledVerification
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	logger    *zap.Logger
}

// NewVerificationScheduler 创建验证调度器
func NewVerificationScheduler(verifier *VerificationManager, logger *zap.Logger) *VerificationScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &VerificationScheduler{
		verifier:  verifier,
		schedules: make(map[string]*ScheduledVerification),
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
	}
}

// Start 启动调度器
func (vs *VerificationScheduler) Start() {
	vs.wg.Add(1)
	go vs.loop()
}

// Stop 停止调度器
func (vs *VerificationScheduler) Stop() {
	vs.cancel()
	vs.wg.Wait()
}

// loop 调度循环
func (vs *VerificationScheduler) loop() {
	defer vs.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-vs.ctx.Done():
			return
		case <-ticker.C:
			vs.checkSchedules()
		}
	}
}

// checkSchedules 检查调度
func (vs *VerificationScheduler) checkSchedules() {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	now := time.Now()

	for _, schedule := range vs.schedules {
		if !schedule.Enabled {
			continue
		}

		if now.After(schedule.NextRun) || now.Equal(schedule.NextRun) {
			go vs.runVerification(schedule)
		}
	}
}

// runVerification 运行验证
func (vs *VerificationScheduler) runVerification(schedule *ScheduledVerification) {
	vs.logger.Info("Running scheduled verification",
		zap.String("snapshot_id", schedule.SnapshotID),
	)

	_, err := vs.verifier.QuickVerify(vs.ctx, schedule.SnapshotID)
	if err != nil {
		vs.logger.Error("Scheduled verification failed",
			zap.String("snapshot_id", schedule.SnapshotID),
			zap.Error(err),
		)
	}

	vs.mu.Lock()
	schedule.LastRun = time.Now()
	schedule.NextRun = schedule.LastRun.Add(schedule.Interval)
	vs.mu.Unlock()
}

// AddSchedule 添加调度
func (vs *VerificationScheduler) AddSchedule(snapshotID string, interval time.Duration) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.schedules[snapshotID] = &ScheduledVerification{
		SnapshotID: snapshotID,
		Interval:   interval,
		NextRun:    time.Now().Add(interval),
		Enabled:    true,
	}
}

// RemoveSchedule 移除调度
func (vs *VerificationScheduler) RemoveSchedule(snapshotID string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.schedules, snapshotID)
}

// GetSchedules 获取调度列表
func (vs *VerificationScheduler) GetSchedules() []*ScheduledVerification {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	schedules := make([]*ScheduledVerification, 0, len(vs.schedules))
	for _, s := range vs.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}
