// Package storage 提供存储自愈功能（参考 TrueNAS/OpenZFS 设计）
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 自愈系统常量 ==========

// ChecksumAlgorithm 校验算法类型.
type ChecksumAlgorithm string

const (
	ChecksumSHA256   ChecksumAlgorithm = "sha256"
	ChecksumSHA512   ChecksumAlgorithm = "sha512"
	ChecksumBLAKE3   ChecksumAlgorithm = "blake3"
	ChecksumFletcher ChecksumAlgorithm = "fletcher" // ZFS 默认
)

// HealthScore 健康评分级别.
type HealthScore int

const (
	HealthScoreExcellent HealthScore = 100 // 完全健康
	HealthScoreGood      HealthScore = 80  // 轻微问题
	HealthScoreWarning   HealthScore = 60  // 需要关注
	HealthScoreCritical  HealthScore = 40  // 严重问题
	HealthScoreFailed    HealthScore = 20  // 需要立即修复
	HealthScoreDead      HealthScore = 0   // 不可恢复
)

// SelfHealState 自愈状态.
type SelfHealState string

const (
	SelfHealStateIdle      SelfHealState = "idle"      // 空闲
	SelfHealStateScanning  SelfHealState = "scanning"  // 扫描中
	SelfHealStateRepairing SelfHealState = "repairing" // 修复中
	SelfHealStateFailed    SelfHealState = "failed"    // 修复失败
)

// ========== 数据结构 ==========

// SelfHealConfig 自愈配置.
type SelfHealConfig struct {
	// 校验算法
	ChecksumAlgorithm ChecksumAlgorithm `json:"checksum_algorithm"`

	// 扫描间隔（小时）
	ScanIntervalHours int `json:"scan_interval_hours"`

	// 自动修复开关
	AutoRepair bool `json:"auto_repair"`

	// 最大并发修复数
	MaxConcurrentRepairs int `json:"max_concurrent_repairs"`

	// 修复超时（秒）
	RepairTimeoutSeconds int `json:"repair_timeout_seconds"`

	// 健康评分阈值（低于此值触发自动修复）
	HealthThreshold HealthScore `json:"health_threshold"`

	// 校验块大小（字节）
	ChecksumBlockSize int `json:"checksum_block_size"`

	// 历史保留天数
	HistoryRetentionDays int `json:"history_retention_days"`

	// 数据目录
	DataDir string `json:"data_dir"`
}

// DefaultSelfHealConfig 默认配置.
var DefaultSelfHealConfig = SelfHealConfig{
	ChecksumAlgorithm:    ChecksumSHA256,
	ScanIntervalHours:    24,
	AutoRepair:           true,
	MaxConcurrentRepairs: 4,
	RepairTimeoutSeconds: 300,
	HealthThreshold:      HealthScoreWarning,
	ChecksumBlockSize:    64 * 1024, // 64KB 块
	HistoryRetentionDays: 90,
	DataDir:              "/var/lib/nas-os/selfheal",
}

// SelfHealManager 自愈管理器.
type SelfHealManager struct {
	config    SelfHealConfig
	storage   *Manager
	state     SelfHealState
	health    StorageHealthReport
	errors    []ChecksumError
	repairs   []RepairRecord
	stats     SelfHealStats
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	logger    *zap.Logger
	callbacks SelfHealCallbacks

	// 校验数据库
	checksumDB *ChecksumDatabase
}

// SelfHealCallbacks 自愈事件回调.
type SelfHealCallbacks struct {
	OnErrorDetected   func(error ChecksumError)
	OnRepairStarted   func(repair RepairRecord)
	OnRepairCompleted func(repair RepairRecord, success bool)
	OnHealthChanged   func(oldScore, newScore HealthScore)
	OnScanCompleted   func(report ScanReport)
}

// ChecksumError 校验错误.
type ChecksumError struct {
	ID          string            `json:"id"`
	Volume      string            `json:"volume"`
	Path        string            `json:"path"`
	BlockOffset int64             `json:"block_offset"`
	BlockSize   int               `json:"block_size"`
	Expected    string            `json:"expected_checksum"`
	Actual      string            `json:"actual_checksum"`
	Algorithm   ChecksumAlgorithm `json:"algorithm"`
	DetectedAt  time.Time         `json:"detected_at"`
	Recovered   bool              `json:"recovered"`
	RecoveredAt time.Time         `json:"recovered_at,omitempty"`
	Attempts    int               `json:"repair_attempts"`
}

// RepairRecord 修复记录.
type RepairRecord struct {
	ID          string        `json:"id"`
	ErrorID     string        `json:"error_id"`
	Volume      string        `json:"volume"`
	Path        string        `json:"path"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	Method      RepairMethod   `json:"method"`
	Source      string        `json:"source"` // 数据来源（备份/镜像等）
	Details     string        `json:"details,omitempty"`
}

// RepairMethod 修复方法.
type RepairMethod string

const (
	RepairMethodMirror    RepairMethod = "mirror"    // 从镜像副本恢复
	RepairMethodBackup    RepairMethod = "backup"    // 从备份恢复
	RepairMethodSnapshot  RepairMethod = "snapshot"  // 从快照恢复
	RepairMethodParity    RepairMethod = "parity"    // 从 RAID parity 恢复
	RepairMethodRebuild   RepairMethod = "rebuild"   // 重建数据
	RepairMethodMarkBad   RepairMethod = "mark_bad"  // 标记坏块（无法修复）
)

// ScanReport 扫描报告.
type ScanReport struct {
	ID            string        `json:"id"`
	Volume        string        `json:"volume"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Duration      time.Duration `json:"duration"`
	TotalBlocks   int64         `json:"total_blocks"`
	CheckedBlocks int64         `json:"checked_blocks"`
	ErrorCount    int           `json:"error_count"`
	RepairedCount int           `json:"repaired_count"`
	SkippedCount  int           `json:"skipped_count"`
	BytesScanned  int64         `json:"bytes_scanned"`
	SpeedMBps     float64       `json:"speed_mbps"`
	HealthScore   HealthScore   `json:"health_score"`
}

// StorageHealthReport 存储健康报告.
type StorageHealthReport struct {
	Volume          string      `json:"volume"`
	OverallScore    HealthScore `json:"overall_score"`
	DataIntegrity   HealthScore `json:"data_integrity"`
	MetadataHealth  HealthScore `json:"metadata_health"`
	RedundancyStatus string     `json:"redundancy_status"`
	ErrorCount      int         `json:"error_count"`
	UnresolvedCount int         `json:"unresolved_count"`
	LastScan        time.Time   `json:"last_scan"`
	LastRepair      time.Time   `json:"last_repair"`
	Recommendations []string    `json:"recommendations"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// SelfHealStats 自愈统计.
type SelfHealStats struct {
	TotalScans        int           `json:"total_scans"`
	TotalErrors       int           `json:"total_errors"`
	TotalRepairs      int           `json:"total_repairs"`
	SuccessfulRepairs int           `json:"successful_repairs"`
	FailedRepairs     int           `json:"failed_repairs"`
	LastScanTime      time.Time     `json:"last_scan_time"`
	TotalBytesScanned int64         `json:"total_bytes_scanned"`
	AverageScanSpeed  float64       `json:"average_scan_speed_mbps"`
	Uptime            time.Duration `json:"uptime"`
}

// ChecksumDatabase 校验数据库（存储已知校验值）.
type ChecksumDatabase struct {
	dataDir string
	entries map[string]*ChecksumEntry // path -> entry
	mu      sync.RWMutex
}

// ChecksumEntry 校验条目.
type ChecksumEntry struct {
	Path        string            `json:"path"`
	Volume      string            `json:"volume"`
	Checksum    string            `json:"checksum"`
	Algorithm   ChecksumAlgorithm `json:"algorithm"`
	Size        int64             `json:"size"`
	BlockCount  int64             `json:"block_count"`
	VerifiedAt  time.Time         `json:"verified_at"`
	ModifiedAt  time.Time         `json:"modified_at"`
	BlockHashes []string          `json:"block_hashes,omitempty"` // 分块校验
}

// ========== 创建与初始化 ==========

// NewSelfHealManager 创建自愈管理器.
func NewSelfHealManager(config SelfHealConfig, storage *Manager, logger *zap.Logger) (*SelfHealManager, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage manager 不能为空")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 初始化校验数据库
	checksumDB, err := NewChecksumDatabase(config.DataDir)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("初始化校验数据库失败: %w", err)
	}

	sh := &SelfHealManager{
		config:    config,
		storage:   storage,
		state:     SelfHealStateIdle,
		health:    StorageHealthReport{OverallScore: HealthScoreExcellent},
		errors:    make([]ChecksumError, 0),
		repairs:   make([]RepairRecord, 0),
		stats:     SelfHealStats{},
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		checksumDB: checksumDB,
	}

	// 加载历史数据
	_ = sh.loadHistory()

	return sh, nil
}

// Start 启动自愈管理器.
func (sh *SelfHealManager) Start() error {
	sh.logger.Info("启动存储自愈管理器",
		zap.String("algorithm", string(sh.config.ChecksumAlgorithm)),
		zap.Bool("auto_repair", sh.config.AutoRepair))

	// 启动定时扫描
	sh.wg.Add(1)
	go sh.scanWorker()

	// 启动健康检查
	sh.wg.Add(1)
	go sh.healthCheckWorker()

	return nil
}

// Stop 停止自愈管理器.
func (sh *SelfHealManager) Stop() {
	sh.cancel()
	sh.wg.Wait()
	_ = sh.saveHistory()
	sh.logger.Info("存储自愈管理器已停止")
}

// ========== 核心功能 ==========

// scanWorker 定时扫描工作线程.
func (sh *SelfHealManager) scanWorker() {
	defer sh.wg.Done()

	interval := time.Duration(sh.config.ScanIntervalHours) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动后立即执行一次扫描
	sh.performFullScan()

	for {
		select {
		case <-sh.ctx.Done():
			return
		case <-ticker.C:
			sh.performFullScan()
		}
	}
}

// healthCheckWorker 健康检查工作线程.
func (sh *SelfHealManager) healthCheckWorker() {
	defer sh.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-sh.ctx.Done():
			return
		case <-ticker.C:
			sh.updateHealthScore()
		}
	}
}

// PerformFullScan 执行完整扫描（公开方法）.
func (sh *SelfHealManager) PerformFullScan(volumeName string) (*ScanReport, error) {
	return sh.scanVolume(volumeName)
}

// performFullScan 执行所有卷的扫描.
func (sh *SelfHealManager) performFullScan() {
	sh.mu.Lock()
	if sh.state != SelfHealStateIdle {
		sh.mu.Unlock()
		return
	}
	sh.state = SelfHealStateScanning
	sh.mu.Unlock()

	defer func() {
		sh.mu.Lock()
		sh.state = SelfHealStateIdle
		sh.mu.Unlock()
	}()

	volumes := sh.storage.ListVolumes()
	for _, vol := range volumes {
		report, err := sh.scanVolume(vol.Name)
		if err != nil {
			sh.logger.Error("扫描卷失败",
				zap.String("volume", vol.Name),
				zap.Error(err))
			continue
		}

		// 触发回调
		if sh.callbacks.OnScanCompleted != nil {
			sh.callbacks.OnScanCompleted(*report)
		}

		// 如果发现错误且启用自动修复
		if report.ErrorCount > 0 && sh.config.AutoRepair {
			sh.autoRepairErrors(vol.Name)
		}
	}
}

// scanVolume 扫描指定卷.
func (sh *SelfHealManager) scanVolume(volumeName string) (*ScanReport, error) {
	vol, err := sh.storage.GetVolume(volumeName)
	if err != nil {
		return nil, err
	}

	report := &ScanReport{
		ID:        fmt.Sprintf("scan-%d", time.Now().UnixNano()),
		Volume:    volumeName,
		StartedAt: time.Now(),
	}

	sh.logger.Info("开始扫描卷", zap.String("volume", volumeName))

	// 遍历卷中所有文件
	err = sh.scanDirectory(vol.MountPoint, report)
	if err != nil {
		sh.logger.Error("扫描目录失败", zap.Error(err))
	}

	report.CompletedAt = time.Now()
	report.Duration = report.CompletedAt.Sub(report.StartedAt)

	// 计算扫描速度
	if report.Duration > 0 && report.BytesScanned > 0 {
		report.SpeedMBps = float64(report.BytesScanned) / float64(report.Duration.Milliseconds()) * 1000 / 1024 / 1024
	}

	// 计算健康评分
	report.HealthScore = sh.calculateHealthScore(report)

	// 更新统计
	sh.mu.Lock()
	sh.stats.TotalScans++
	sh.stats.TotalErrors += report.ErrorCount
	sh.stats.TotalBytesScanned += report.BytesScanned
	sh.stats.LastScanTime = report.CompletedAt
	sh.mu.Unlock()

	sh.logger.Info("扫描完成",
		zap.String("volume", volumeName),
		zap.Int64("blocks", report.CheckedBlocks),
		zap.Int("errors", report.ErrorCount),
		zap.Duration("duration", report.Duration),
		zap.String("health_score", fmt.Sprintf("%d", report.HealthScore)))

	return report, nil
}

// scanDirectory 扫描目录.
func (sh *SelfHealManager) scanDirectory(rootPath string, report *ScanReport) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略访问错误
		}

		// 只处理普通文件
		if !info.Mode().IsRegular() {
			return nil
		}

		// 计算校验值
		checksum, err := sh.calculateFileChecksum(path)
		if err != nil {
			sh.logger.Debug("计算校验失败", zap.String("path", path), zap.Error(err))
			report.SkippedCount++
			return nil
		}

		// 与已知校验值比较
		expected, exists := sh.checksumDB.Get(path)
		if exists {
			if expected != checksum {
				// 发现校验错误
				errRecord := ChecksumError{
					ID:         fmt.Sprintf("err-%d", time.Now().UnixNano()),
					Path:       path,
					Expected:   expected,
					Actual:     checksum,
					Algorithm:  sh.config.ChecksumAlgorithm,
					DetectedAt: time.Now(),
				}
				sh.recordError(errRecord)
				report.ErrorCount++
			}
		} else {
			// 新文件，记录校验值
			sh.checksumDB.Set(path, checksum, sh.config.ChecksumAlgorithm, info.Size())
		}

		report.CheckedBlocks++
		report.BytesScanned += info.Size()

		return nil
	})
}

// calculateFileChecksum 计算文件校验值.
func (sh *SelfHealManager) calculateFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	buf := make([]byte, sh.config.ChecksumBlockSize)

	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}
		hash.Write(buf[:n])
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// recordError 记录校验错误.
func (sh *SelfHealManager) recordError(err ChecksumError) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.errors = append(sh.errors, err)

	// 限制历史记录
	if len(sh.errors) > 1000 {
		sh.errors = sh.errors[len(sh.errors)-1000:]
	}

	// 触发回调
	if sh.callbacks.OnErrorDetected != nil {
		go sh.callbacks.OnErrorDetected(err)
	}

	sh.logger.Warn("发现校验错误",
		zap.String("path", err.Path),
		zap.String("expected", err.Expected),
		zap.String("actual", err.Actual))
}

// ========== 自动修复 ==========

// autoRepairErrors 自动修复错误.
func (sh *SelfHealManager) autoRepairErrors(volumeName string) {
	sh.mu.Lock()
	if sh.state == SelfHealStateRepairing {
		sh.mu.Unlock()
		return
	}
	sh.state = SelfHealStateRepairing

	// 获取待修复的错误
	var pending []ChecksumError
	for _, e := range sh.errors {
		if !e.Recovered && e.Volume == volumeName {
			pending = append(pending, e)
		}
	}
	sh.mu.Unlock()

	defer func() {
		sh.mu.Lock()
		sh.state = SelfHealStateIdle
		sh.mu.Unlock()
	}()

	if len(pending) == 0 {
		return
	}

	sh.logger.Info("开始自动修复", zap.Int("error_count", len(pending)))

	// 限制并发修复
	sem := make(chan struct{}, sh.config.MaxConcurrentRepairs)
	for _, err := range pending {
		sem <- struct{}{}
		go func(e ChecksumError) {
			defer func() { <-sem }()
			sh.repairError(e)
		}(err)
	}

	// 等待所有修复完成
	for i := 0; i < sh.config.MaxConcurrentRepairs; i++ {
		sem <- struct{}{}
	}
}

// repairError 修复单个错误.
func (sh *SelfHealManager) repairError(err ChecksumError) bool {
	record := RepairRecord{
		ID:        fmt.Sprintf("repair-%d", time.Now().UnixNano()),
		ErrorID:   err.ID,
		Volume:    err.Volume,
		Path:      err.Path,
		StartedAt: time.Now(),
		Method:    sh.selectRepairMethod(err),
	}

	// 触发回调
	if sh.callbacks.OnRepairStarted != nil {
		go sh.callbacks.OnRepairStarted(record)
	}

	var success bool
	var details string
	var source string

	switch record.Method {
	case RepairMethodMirror:
		success, source, details = sh.repairFromMirror(err)
	case RepairMethodSnapshot:
		success, source, details = sh.repairFromSnapshot(err)
	case RepairMethodBackup:
		success, source, details = sh.repairFromBackup(err)
	case RepairMethodParity:
		success, source, details = sh.repairFromParity(err)
	case RepairMethodMarkBad:
		success = false
		details = "无法修复，已标记为坏块"
	}

	record.CompletedAt = time.Now()
	record.Duration = record.CompletedAt.Sub(record.StartedAt)
	record.Success = success
	record.Source = source
	record.Details = details

	// 记录修复
	sh.mu.Lock()
	sh.repairs = append(sh.repairs, record)
	if success {
		sh.stats.SuccessfulRepairs++
		// 更新错误状态
		for i, e := range sh.errors {
			if e.ID == err.ID {
				sh.errors[i].Recovered = true
				sh.errors[i].RecoveredAt = time.Now()
				sh.errors[i].Attempts++
				break
			}
		}
	} else {
		sh.stats.FailedRepairs++
		for i, e := range sh.errors {
			if e.ID == err.ID {
				sh.errors[i].Attempts++
				break
			}
		}
	}
	sh.stats.TotalRepairs++
	sh.mu.Unlock()

	// 触发回调
	if sh.callbacks.OnRepairCompleted != nil {
		go sh.callbacks.OnRepairCompleted(record, success)
	}

	sh.logger.Info("修复完成",
		zap.String("path", err.Path),
		zap.Bool("success", success),
		zap.String("method", string(record.Method)),
		zap.Duration("duration", record.Duration))

	return success
}

// selectRepairMethod 选择修复方法.
func (sh *SelfHealManager) selectRepairMethod(err ChecksumError) RepairMethod {
	// 优先级：镜像 > 快照 > 备份 > parity > 标记坏块

	// 检查是否有镜像副本（RAID1）
	vol, e := sh.storage.GetVolume(err.Volume)
	if e == nil && vol.DataProfile == "raid1" {
		return RepairMethodMirror
	}

	// 检查是否有快照
	if sh.hasSnapshot(err.Volume, err.Path) {
		return RepairMethodSnapshot
	}

	// 检查是否有备份
	if sh.hasBackup(err.Volume, err.Path) {
		return RepairMethodBackup
	}

	// RAID5/6 可以从 parity 恢复
	if e == nil && (vol.DataProfile == "raid5" || vol.DataProfile == "raid6") {
		return RepairMethodParity
	}

	// 无法修复
	return RepairMethodMarkBad
}

// repairFromMirror 从镜像副本修复.
func (sh *SelfHealManager) repairFromMirror(err ChecksumError) (bool, string, string) {
	// TODO: 实现 Btrfs RAID1 镜像恢复
	// 使用 btrfs device stats 检查镜像状态
	return false, "", "镜像恢复功能待实现"
}

// repairFromSnapshot 从快照修复.
func (sh *SelfHealManager) repairFromSnapshot(err ChecksumError) (bool, string, string) {
	// TODO: 查找最近的快照并恢复文件
	// 1. 查找包含该文件的快照
	// 2. 比较快照中的校验值
	// 3. 复制正确的数据
	return false, "", "快照恢复功能待实现"
}

// repairFromBackup 从备份修复.
func (sh *SelfHealManager) repairFromBackup(err ChecksumError) (bool, string, string) {
	// TODO: 从远程备份恢复
	return false, "", "备份恢复功能待实现"
}

// repairFromParity 从 RAID parity 修复.
func (sh *SelfHealManager) repairFromParity(err ChecksumError) (bool, string, string) {
	// TODO: 实现 RAID5/RAID6 parity 恢复
	// Btrfs 的 scrub 功能会自动处理
	return false, "", "parity 恢复功能待实现"
}

// hasSnapshot 检查是否有快照.
func (sh *SelfHealManager) hasSnapshot(volume, path string) bool {
	// TODO: 检查快照列表
	return false
}

// hasBackup 检查是否有备份.
func (sh *SelfHealManager) hasBackup(volume, path string) bool {
	// TODO: 检查备份配置
	return false
}

// ========== 健康评分 ==========

// updateHealthScore 更新健康评分.
func (sh *SelfHealManager) updateHealthScore() {
	sh.mu.RLock()
	errors := len(sh.errors)
	unresolved := 0
	for _, e := range sh.errors {
		if !e.Recovered {
			unresolved++
		}
	}
	sh.mu.RUnlock()

	oldScore := sh.health.OverallScore

	// 计算新评分
	newScore := HealthScoreExcellent
	if unresolved > 0 {
		// 根据未解决错误数量降级
		if unresolved >= 100 {
			newScore = HealthScoreFailed
		} else if unresolved >= 50 {
			newScore = HealthScoreCritical
		} else if unresolved >= 10 {
			newScore = HealthScoreWarning
		} else if unresolved >= 1 {
			newScore = HealthScoreGood
		}
	}

	// 更新报告
	sh.mu.Lock()
	sh.health.ErrorCount = errors
	sh.health.UnresolvedCount = unresolved
	sh.health.OverallScore = newScore
	sh.health.UpdatedAt = time.Now()

	// 生成建议
	sh.health.Recommendations = sh.generateRecommendations(newScore, unresolved)
	sh.mu.Unlock()

	// 触发回调
	if oldScore != newScore && sh.callbacks.OnHealthChanged != nil {
		go sh.callbacks.OnHealthChanged(oldScore, newScore)
	}

	sh.logger.Debug("健康评分更新",
		zap.Int("score", int(newScore)),
		zap.Int("errors", errors),
		zap.Int("unresolved", unresolved))
}

// calculateHealthScore 计算健康评分.
func (sh *SelfHealManager) calculateHealthScore(report *ScanReport) HealthScore {
	if report.ErrorCount == 0 {
		return HealthScoreExcellent
	}

	// 错误率
	errorRate := float64(report.ErrorCount) / float64(report.CheckedBlocks)
	if errorRate > 0.01 { // 1% 以上错误率
		return HealthScoreFailed
	} else if errorRate > 0.001 { // 0.1% 以上
		return HealthScoreCritical
	} else if errorRate > 0.0001 { // 0.01% 以上
		return HealthScoreWarning
	}

	return HealthScoreGood
}

// generateRecommendations 生成建议.
func (sh *SelfHealManager) generateRecommendations(score HealthScore, unresolved int) []string {
	recommendations := make([]string, 0)

	switch score {
	case HealthScoreFailed:
		recommendations = append(recommendations, "存储系统严重损坏，建议立即更换故障磁盘")
		recommendations = append(recommendations, "立即备份数据到外部存储")
	case HealthScoreCritical:
		recommendations = append(recommendations, "存在大量数据损坏，建议尽快修复")
		recommendations = append(recommendations, "检查磁盘 SMART 状态")
	case HealthScoreWarning:
		recommendations = append(recommendations, "发现数据校验错误，建议执行修复")
		recommendations = append(recommendations, "检查存储冗余状态")
	case HealthScoreGood:
		if unresolved > 0 {
			recommendations = append(recommendations, "存在少量校验错误，建议定期检查")
		}
	}

	return recommendations
}

// ========== 公开 API ==========

// GetHealthReport 获取健康报告.
func (sh *SelfHealManager) GetHealthReport() StorageHealthReport {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.health
}

// GetErrors 获取错误列表.
func (sh *SelfHealManager) GetErrors(limit int) []ChecksumError {
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	if limit <= 0 || limit > len(sh.errors) {
		limit = len(sh.errors)
	}

	start := len(sh.errors) - limit
	if start < 0 {
		start = 0
	}

	return sh.errors[start:]
}

// GetRepairHistory 获取修复历史.
func (sh *SelfHealManager) GetRepairHistory(limit int) []RepairRecord {
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	if limit <= 0 || limit > len(sh.repairs) {
		limit = len(sh.repairs)
	}

	start := len(sh.repairs) - limit
	if start < 0 {
		start = 0
	}

	return sh.repairs[start:]
}

// GetStats 获取统计信息.
func (sh *SelfHealManager) GetStats() SelfHealStats {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.stats
}

// GetState 获取当前状态.
func (sh *SelfHealManager) GetState() SelfHealState {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.state
}

// SetCallbacks 设置回调.
func (sh *SelfHealManager) SetCallbacks(callbacks SelfHealCallbacks) {
	sh.callbacks = callbacks
}

// ForceScan 强制扫描指定卷.
func (sh *SelfHealManager) ForceScan(volumeName string) (*ScanReport, error) {
	return sh.scanVolume(volumeName)
}

// ForceRepair 强制修复指定错误.
func (sh *SelfHealManager) ForceRepair(errorID string) (bool, error) {
	sh.mu.RLock()
	var target *ChecksumError
	for _, e := range sh.errors {
		if e.ID == errorID {
			target = &e
			break
		}
	}
	sh.mu.RUnlock()

	if target == nil {
		return false, fmt.Errorf("错误记录不存在: %s", errorID)
	}

	return sh.repairError(*target), nil
}

// ========== 校验数据库 ==========

// NewChecksumDatabase 创建校验数据库.
func NewChecksumDatabase(dataDir string) (*ChecksumDatabase, error) {
	db := &ChecksumDatabase{
		dataDir: dataDir,
		entries: make(map[string]*ChecksumEntry),
	}

	// 加载已有数据
	dbFile := filepath.Join(dataDir, "checksums.json")
	data, err := os.ReadFile(dbFile)
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil
		}
		return nil, err
	}

	var entries []*ChecksumEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	for _, e := range entries {
		db.entries[e.Path] = e
	}

	return db, nil
}

// Get 获取校验值.
func (db *ChecksumDatabase) Get(path string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if entry, exists := db.entries[path]; exists {
		return entry.Checksum, true
	}
	return "", false
}

// Set 设置校验值.
func (db *ChecksumDatabase) Set(path, checksum string, algorithm ChecksumAlgorithm, size int64) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.entries[path] = &ChecksumEntry{
		Path:       path,
		Checksum:   checksum,
		Algorithm:  algorithm,
		Size:       size,
		VerifiedAt: time.Now(),
	}
}

// Save 保存数据库.
func (db *ChecksumDatabase) Save() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	entries := make([]*ChecksumEntry, 0, len(db.entries))
	for _, e := range db.entries {
		entries = append(entries, e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	dbFile := filepath.Join(db.dataDir, "checksums.json")
	return os.WriteFile(dbFile, data, 0640)
}

// ========== 持久化 ==========

func (sh *SelfHealManager) saveHistory() error {
	// 保存错误记录
	errorsFile := filepath.Join(sh.config.DataDir, "errors.json")
	errorsData, err := json.MarshalIndent(sh.errors, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(errorsFile, errorsData, 0640); err != nil {
		return err
	}

	// 保存修复记录
	repairsFile := filepath.Join(sh.config.DataDir, "repairs.json")
	repairsData, err := json.MarshalIndent(sh.repairs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(repairsFile, repairsData, 0640); err != nil {
		return err
	}

	// 保存校验数据库
	return sh.checksumDB.Save()
}

func (sh *SelfHealManager) loadHistory() error {
	// 加载错误记录
	errorsFile := filepath.Join(sh.config.DataDir, "errors.json")
	errorsData, err := os.ReadFile(errorsFile)
	if err == nil {
		var errors []ChecksumError
		if err := json.Unmarshal(errorsData, &errors); err == nil {
			sh.errors = errors
		}
	}

	// 加载修复记录
	repairsFile := filepath.Join(sh.config.DataDir, "repairs.json")
	repairsData, err := os.ReadFile(repairsFile)
	if err == nil {
		var repairs []RepairRecord
		if err := json.Unmarshal(repairsData, &repairs); err == nil {
			sh.repairs = repairs
		}
	}

	return nil
}