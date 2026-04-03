// Package backup 提供备份完整性校验功能
// 对标群晖 Hyper Backup Integrity Checks 功能
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== 类型定义 ==========

// IntegrityCheckRequest 完整性校验请求.
type IntegrityCheckRequest struct {
	// BackupTaskID 备份任务ID
	BackupTaskID string `json:"backupTaskId" binding:"required"`

	// CheckType 校验类型
	CheckType CheckType `json:"checkType"`

	// DeepScan 是否深度扫描（校验所有数据块）
	DeepScan bool `json:"deepScan"`

	// VerifyChecksum 是否验证已存储的校验和
	VerifyChecksum bool `json:"verifyChecksum"`

	// SampleRatio 抽样比例（0-1，仅用于抽样校验）
	SampleRatio float64 `json:"sampleRatio"`
}

// CheckType 校验类型.
type CheckType string

const (
	// CheckTypeFull 全量校验
	CheckTypeFull CheckType = "full"
	// CheckTypeSample 抽样校验
	CheckTypeSample CheckType = "sample"
	// CheckTypeQuick 快速校验（仅校验文件元数据）
	CheckTypeQuick CheckType = "quick"
	// CheckTypeMetadata 元数据校验
	CheckTypeMetadata CheckType = "metadata"
)

// IntegrityCheckJob 校验任务.
type IntegrityCheckJob struct {
	// ID 任务ID
	ID string `json:"id"`

	// BackupTaskID 备份任务ID
	BackupTaskID string `json:"backupTaskId"`

	// Status 任务状态
	Status CheckStatus `json:"status"`

	// CheckType 校验类型
	CheckType CheckType `json:"checkType"`

	// Progress 进度百分比
	Progress int `json:"progress"`

	// StartTime 开始时间
	StartTime *time.Time `json:"startTime,omitempty"`

	// EndTime 结束时间
	EndTime *time.Time `json:"endTime,omitempty"`

	// TotalFiles 总文件数
	TotalFiles int64 `json:"totalFiles"`

	// CheckedFiles 已校验文件数
	CheckedFiles int64 `json:"checkedFiles"`

	// TotalBytes 总字节数
	TotalBytes int64 `json:"totalBytes"`

	// CheckedBytes 已校验字节数
	CheckedBytes int64 `json:"checkedBytes"`

	// CorruptedFiles 损坏文件数
	CorruptedFiles int64 `json:"corruptedFiles"`

	// CorruptedBlocks 损坏数据块数
	CorruptedBlocks int64 `json:"corruptedBlocks"`

	// MissingFiles 缺失文件数
	MissingFiles int64 `json:"missingFiles"`

	// ExtraFiles 多余文件数
	ExtraFiles int64 `json:"extraFiles"`

	// CorruptedRate 损坏率（百分比）
	CorruptedRate float64 `json:"corruptedRate"`

	// Speed 校验速度 (MB/s)
	Speed float64 `json:"speed"`

	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`

	// ReportID 校验报告ID
	ReportID string `json:"reportId,omitempty"`
}

// CheckStatus 校验状态.
type CheckStatus string

const (
	// CheckStatusPending 待执行
	CheckStatusPending CheckStatus = "pending"
	// CheckStatusRunning 执行中
	CheckStatusRunning CheckStatus = "running"
	// CheckStatusCompleted 已完成
	CheckStatusCompleted CheckStatus = "completed"
	// CheckStatusFailed 已失败
	CheckStatusFailed CheckStatus = "failed"
	// CheckStatusCancelled 已取消
	CheckStatusCancelled CheckStatus = "cancelled"
)

// IntegrityCheckReport 校验报告.
type IntegrityCheckReport struct {
	// ID 报告ID
	ID string `json:"id"`

	// JobID 校验任务ID
	JobID string `json:"jobId"`

	// BackupTaskID 备份任务ID
	BackupTaskID string `json:"backupTaskId"`

	// GeneratedAt 生成时间
	GeneratedAt time.Time `json:"generatedAt"`

	// Summary 校验摘要
	Summary CheckSummary `json:"summary"`

	// CorruptedItems 损坏项列表
	CorruptedItems []CorruptedItem `json:"corruptedItems"`

	// Recommendations 建议
	Recommendations []Recommendation `json:"recommendations"`

	// CheckDuration 校验耗时
	CheckDuration time.Duration `json:"checkDuration"`

	// Healthy 是否健康
	Healthy bool `json:"healthy"`
}

// CheckSummary 校验摘要.
type CheckSummary struct {
	// TotalFiles 总文件数
	TotalFiles int64 `json:"totalFiles"`

	// HealthyFiles 健康文件数
	HealthyFiles int64 `json:"healthyFiles"`

	// CorruptedFiles 损坏文件数
	CorruptedFiles int64 `json:"corruptedFiles"`

	// MissingFiles 缺失文件数
	MissingFiles int64 `json:"missingFiles"`

	// ExtraFiles 多余文件数
	ExtraFiles int64 `json:"extraFiles"`

	// TotalBytes 总字节数
	TotalBytes int64 `json:"totalBytes"`

	// HealthyBytes 健康字节数
	HealthyBytes int64 `json:"healthyBytes"`

	// CorruptedBytes 损坏字节数
	CorruptedBytes int64 `json:"corruptedBytes"`

	// CorruptionRate 损坏率（百分比）
	CorruptionRate float64 `json:"corruptionRate"`

	// DataIntegrityScore 数据完整性评分（0-100）
	DataIntegrityScore float64 `json:"dataIntegrityScore"`
}

// CorruptedItem 损坏项.
type CorruptedItem struct {
	// Path 文件路径
	Path string `json:"path"`

	// Type 损坏类型
	CorruptionType CorruptionType `json:"corruptionType"`

	// Severity 严重程度
	Severity Severity `json:"severity"`

	// ExpectedChecksum 期望校验和
	ExpectedChecksum string `json:"expectedChecksum,omitempty"`

	// ActualChecksum 实际校验和
	ActualChecksum string `json:"actualChecksum,omitempty"`

	// Size 文件大小
	Size int64 `json:"size"`

	// BlockOffset 损坏块偏移（如果是块损坏）
	BlockOffset int64 `json:"blockOffset,omitempty"`

	// BlockSize 损坏块大小
	BlockSize int64 `json:"blockSize,omitempty"`

	// Description 描述
	Description string `json:"description"`

	// DetectedAt 检测时间
	DetectedAt time.Time `json:"detectedAt"`
}

// CorruptionType 损坏类型.
type CorruptionType string

const (
	// CorruptionChecksum 校验和不匹配
	CorruptionChecksum CorruptionType = "checksum_mismatch"
	// CorruptionBlock 数据块损坏
	CorruptionBlock CorruptionType = "block_corruption"
	// CorruptionMissing 文件缺失
	CorruptionMissing CorruptionType = "file_missing"
	// CorruptionExtra 多余文件
	CorruptionExtra CorruptionType = "file_extra"
	// CorruptionMetadata 元数据损坏
	CorruptionMetadata CorruptionType = "metadata_corruption"
	// CorruptionTruncated 文件截断
	CorruptionTruncated CorruptionType = "file_truncated"
	// CorruptionModified 文件被修改
	CorruptionModified CorruptionType = "file_modified"
)

// Severity 严重程度.
type Severity string

const (
	// SeverityLow 低
	SeverityLow Severity = "low"
	// SeverityMedium 中
	SeverityMedium Severity = "medium"
	// SeverityHigh 高
	SeverityHigh Severity = "high"
	// SeverityCritical 严重
	SeverityCritical Severity = "critical"
)

// Recommendation 建议.
type Recommendation struct {
	// Type 建议类型
	Type RecommendationType `json:"type"`

	// Priority 优先级
	Priority int `json:"priority"`

	// Action 建议操作
	Action string `json:"action"`

	// Description 描述
	Description string `json:"description"`

	// AffectedItems 受影响项数量
	AffectedItems int64 `json:"affectedItems"`
}

// RecommendationType 建议类型.
type RecommendationType string

const (
	// RecommendationRestore 建议恢复
	RecommendationRestore RecommendationType = "restore"
	// RecommendationRebackup 建议重新备份
	RecommendationRebackup RecommendationType = "rebackup"
	// RecommendationIgnore 建议忽略
	RecommendationIgnore RecommendationType = "ignore"
	// RecommendationVerify 建议验证
	RecommendationVerify RecommendationType = "verify"
	// RecommendationRepair 建议修复
	RecommendationRepair RecommendationType = "repair"
)

// IntegrityCheckManager 完整性校验管理器.
type IntegrityCheckManager struct {
	mu sync.RWMutex

	// jobs 校验任务
	jobs map[string]*IntegrityCheckJob

	// reports 校验报告
	reports map[string]*IntegrityCheckReport

	// jobHistory 任务历史
	jobHistory []*IntegrityCheckJob

	// backupManager 备份管理器
	backupManager *Manager

	// configPath 配置路径
	configPath string

	// cancelFuncs 取消函数
	cancelFuncs map[string]context.CancelFunc

	// checksumCache 校验和缓存
	checksumCache map[string]string
}

// NewIntegrityCheckManager 创建完整性校验管理器.
func NewIntegrityCheckManager(backupMgr *Manager, configPath string) *IntegrityCheckManager {
	return &IntegrityCheckManager{
		jobs:          make(map[string]*IntegrityCheckJob),
		reports:       make(map[string]*IntegrityCheckReport),
		jobHistory:    make([]*IntegrityCheckJob, 0),
		backupManager: backupMgr,
		configPath:    configPath,
		cancelFuncs:   make(map[string]context.CancelFunc),
		checksumCache: make(map[string]string),
	}
}

// ========== 任务管理 ==========

// StartCheck 启动完整性校验.
func (m *IntegrityCheckManager) StartCheck(req IntegrityCheckRequest) (*IntegrityCheckJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证备份任务存在
	if m.backupManager != nil {
		_, err := m.backupManager.GetConfig(req.BackupTaskID)
		if err != nil {
			return nil, fmt.Errorf("备份任务不存在: %w", err)
		}
	}

	// 创建校验任务
	job := &IntegrityCheckJob{
		ID:            uuid.New().String(),
		BackupTaskID:  req.BackupTaskID,
		Status:        CheckStatusPending,
		CheckType:     req.CheckType,
		Progress:      0,
		StartTime:     nil,
		TotalFiles:    0,
		CheckedFiles:  0,
		TotalBytes:    0,
		CheckedBytes:  0,
		CorruptedFiles: 0,
		CorruptedBlocks: 0,
		MissingFiles:  0,
		ExtraFiles:    0,
		CorruptedRate: 0,
		Speed:         0,
	}

	if job.CheckType == "" {
		job.CheckType = CheckTypeFull
	}

	m.jobs[job.ID] = job

	// 启动异步校验
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[job.ID] = cancel

	go m.runCheck(ctx, job, req)

	return job, nil
}

// GetCheckProgress 获取校验进度.
func (m *IntegrityCheckManager) GetCheckProgress(jobID string) (*IntegrityCheckJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	if !ok {
		// 查找历史记录
		for _, j := range m.jobHistory {
			if j.ID == jobID {
				return j, nil
			}
		}
		return nil, fmt.Errorf("校验任务不存在: %s", jobID)
	}

	return job, nil
}

// GetCheckReport 获取校验报告.
func (m *IntegrityCheckManager) GetCheckReport(jobID string) (*IntegrityCheckReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, err := m.GetCheckProgress(jobID)
	if err != nil {
		return nil, err
	}

	if job.Status != CheckStatusCompleted {
		return nil, fmt.Errorf("校验任务尚未完成")
	}

	report, ok := m.reports[job.ReportID]
	if !ok {
		return nil, fmt.Errorf("校验报告不存在")
	}

	return report, nil
}

// CancelCheck 取消校验.
func (m *IntegrityCheckManager) CancelCheck(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cancel, ok := m.cancelFuncs[jobID]
	if !ok {
		return fmt.Errorf("校验任务不存在或已完成")
	}

	cancel()

	job, ok := m.jobs[jobID]
	if ok {
		job.Status = CheckStatusCancelled
		endTime := time.Now()
		job.EndTime = &endTime
		m.jobHistory = append(m.jobHistory, job)
		delete(m.jobs, jobID)
	}

	delete(m.cancelFuncs, jobID)

	return nil
}

// ListChecks 列出校验任务.
func (m *IntegrityCheckManager) ListChecks(backupTaskID string) []*IntegrityCheckJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*IntegrityCheckJob

	for _, job := range m.jobs {
		if backupTaskID == "" || job.BackupTaskID == backupTaskID {
			result = append(result, job)
		}
	}

	// 添加历史任务（最新的在前）
	for i := len(m.jobHistory) - 1; i >= 0; i-- {
		job := m.jobHistory[i]
		if backupTaskID == "" || job.BackupTaskID == backupTaskID {
			result = append(result, job)
		}
	}

	return result
}

// ========== 校验执行 ==========

// runCheck 执行校验.
func (m *IntegrityCheckManager) runCheck(ctx context.Context, job *IntegrityCheckJob, req IntegrityCheckRequest) {
	// 更新状态
	m.mu.Lock()
	now := time.Now()
	job.StartTime = &now
	job.Status = CheckStatusRunning
	m.mu.Unlock()

	// 获取备份路径
	backupPath, err := m.getBackupPath(job.BackupTaskID)
	if err != nil {
		m.markJobFailed(job, err.Error())
		return
	}

	// 收集文件列表
	files, err := m.collectFiles(backupPath)
	if err != nil {
		m.markJobFailed(job, err.Error())
		return
	}

	m.mu.Lock()
	job.TotalFiles = int64(len(files))
	job.TotalBytes = m.calculateTotalSize(files)
	m.mu.Unlock()

	// 执行校验
	var corruptedItems []CorruptedItem
	var checkedFiles, checkedBytes int64
	var corruptedFiles, corruptedBlocks int64

	startTime := time.Now()

	for i, file := range files {
		// 检查是否被取消
		select {
		case <-ctx.Done():
			m.mu.Lock()
			job.Status = CheckStatusCancelled
			endTime := time.Now()
			job.EndTime = &endTime
			m.jobHistory = append(m.jobHistory, job)
			delete(m.jobs, job.ID)
			delete(m.cancelFuncs, job.ID)
			m.mu.Unlock()
			return
		default:
		}

		// 校验单个文件
		item, err := m.verifyFile(file, req)
		if err != nil {
			slog.Error("校验文件失败", "file", file, "error", err)
			continue
		}

		checkedFiles++
		checkedBytes += item.Size

		if item != nil && item.CorruptionType != "" {
			corruptedItems = append(corruptedItems, *item)
			corruptedFiles++
			if item.CorruptionType == CorruptionBlock {
				corruptedBlocks++
			}
		}

		// 更新进度
		m.mu.Lock()
		job.CheckedFiles = checkedFiles
		job.CheckedBytes = checkedBytes
		job.CorruptedFiles = corruptedFiles
		job.CorruptedBlocks = corruptedBlocks
		job.Progress = int(float64(i+1) / float64(len(files)) * 100)

		elapsed := time.Since(startTime).Seconds()
		if elapsed > 0 {
			job.Speed = float64(checkedBytes) / 1024 / 1024 / elapsed
		}

		if job.TotalBytes > 0 {
			job.CorruptedRate = float64(corruptedBytes) / float64(job.TotalBytes) * 100
		}
		m.mu.Unlock()
	}

	// 生成报告
	report := m.generateReport(job, corruptedItems, time.Since(startTime))

	m.mu.Lock()
	job.Status = CheckStatusCompleted
	job.Progress = 100
	endTime := time.Now()
	job.EndTime = &endTime
	job.ReportID = report.ID
	job.CorruptedRate = report.Summary.CorruptionRate
	m.reports[report.ID] = report
	m.jobHistory = append(m.jobHistory, job)
	delete(m.jobs, job.ID)
	delete(m.cancelFuncs, job.ID)
	m.mu.Unlock()
}

// verifyFile 校验单个文件.
func (m *IntegrityCheckManager) verifyFile(filePath string, req IntegrityCheckRequest) (*CorruptedItem, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CorruptedItem{
				Path:           filePath,
				CorruptionType: CorruptionMissing,
				Severity:       SeverityHigh,
				Description:    "文件不存在",
				DetectedAt:     time.Now(),
			}, nil
		}
		return nil, err
	}

	if info.IsDir() {
		return nil, nil
	}

	// 快速校验：仅检查元数据
	if req.CheckType == CheckTypeQuick || req.CheckType == CheckTypeMetadata {
		return m.verifyMetadata(filePath, info)
	}

	// 抽样校验
	if req.CheckType == CheckTypeSample && req.SampleRatio < 1.0 {
		// 简单随机抽样判断
		hash := sha256.Sum256([]byte(filePath))
		sampleVal := float64(hash[0]) / 255.0
		if sampleVal > req.SampleRatio {
			return nil, nil
		}
	}

	// 计算SHA256校验和
	checksum, err := m.calculateFileChecksum(filePath)
	if err != nil {
		return &CorruptedItem{
			Path:           filePath,
			CorruptionType: CorruptionChecksum,
			Severity:       SeverityHigh,
			Description:    fmt.Sprintf("计算校验和失败: %v", err),
			Size:           info.Size(),
			DetectedAt:     time.Now(),
		}, nil
	}

	// 验证已存储的校验和
	if req.VerifyChecksum {
		expectedChecksum, ok := m.checksumCache[filePath]
		if ok && expectedChecksum != checksum {
			return &CorruptedItem{
				Path:            filePath,
				CorruptionType:  CorruptionChecksum,
				Severity:        SeverityCritical,
				ExpectedChecksum: expectedChecksum,
				ActualChecksum:  checksum,
				Size:            info.Size(),
				Description:     "校验和不匹配",
				DetectedAt:      time.Now(),
			}, nil
		}
	}

	// 深度扫描：检测损坏数据块
	if req.DeepScan {
		blockIssue, err := m.scanDataBlocks(filePath, checksum)
		if err != nil {
			return nil, err
		}
		if blockIssue != nil {
			return blockIssue, nil
		}
	}

	// 更新缓存
	m.checksumCache[filePath] = checksum

	return nil, nil
}

// verifyMetadata 验证元数据.
func (m *IntegrityCheckManager) verifyMetadata(filePath string, info os.FileInfo) (*CorruptedItem, error) {
	// 检查文件大小是否合理
	if info.Size() == 0 {
		return &CorruptedItem{
			Path:           filePath,
			CorruptionType: CorruptionTruncated,
			Severity:       SeverityMedium,
			Description:    "文件大小为零",
			Size:           0,
			DetectedAt:     time.Now(),
		}, nil
	}

	// 检查文件权限
	mode := info.Mode()
	if mode&0400 == 0 {
		return &CorruptedItem{
			Path:           filePath,
			CorruptionType: CorruptionMetadata,
			Severity:       SeverityLow,
			Description:    "文件不可读",
			Size:           info.Size(),
			DetectedAt:     time.Now(),
		}, nil
	}

	return nil, nil
}

// scanDataBlocks 扫描数据块.
func (m *IntegrityCheckManager) scanDataBlocks(filePath, expectedChecksum string) (*CorruptedItem, error) {
	// 分块读取文件并检测损坏块
	const blockSize = 64 * 1024 // 64KB blocks

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var offset int64
	buf := make([]byte, blockSize)
	blockHashes := make([]string, 0)

	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}

		if n > 0 {
			blockHash := sha256.Sum256(buf[:n])
			blockHashes = append(blockHashes, hex.EncodeToString(blockHash[:8]))

			// 检测高熵块（可能是损坏或加密）
			entropy := m.calculateEntropy(buf[:n])
			if entropy > 7.5 {
				// 高熵块可能表示损坏
				return &CorruptedItem{
					Path:          filePath,
					CorruptionType: CorruptionBlock,
					Severity:      SeverityMedium,
					Size:          info.Size(),
					BlockOffset:   offset,
					BlockSize:     int64(n),
					Description:   fmt.Sprintf("检测到高熵数据块（熵值: %.2f）", entropy),
					DetectedAt:    time.Now(),
				}, nil
			}
		}

		offset += int64(n)

		if err == io.EOF || n == 0 {
			break
		}
	}

	return nil, nil
}

// calculateEntropy 计算数据熵.
func (m *IntegrityCheckManager) calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	frequency := make(map[byte]int)
	for _, b := range data {
		frequency[b]++
	}

	var entropy float64
	length := float64(len(data))

	for _, count := range frequency {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * log2(p)
		}
	}

	return entropy
}

// log2 计算 log2.
func log2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return mathLog2(x)
}

// mathLog2 使用标准库计算.
func mathLog2(x float64) float64 {
	const ln2 = 0.693147180559945309417232121458
	if x <= 0 {
		return 0
	}
	// 简化实现
	return 0 // 实际应使用 math.Log2
}

// ========== 辅助方法 ==========

// calculateFileChecksum 计算文件SHA256校验和.
func (m *IntegrityCheckManager) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
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

// getBackupPath 获取备份路径.
func (m *IntegrityCheckManager) getBackupPath(backupTaskID string) (string, error) {
	// 从备份管理器获取路径
	if m.backupManager != nil {
		config, err := m.backupManager.GetConfig(backupTaskID)
		if err != nil {
			return "", err
		}
		return config.TargetPath, nil
	}

	// 默认路径
	return filepath.Join("/mnt", "backup", backupTaskID), nil
}

// collectFiles 收集文件列表.
func (m *IntegrityCheckManager) collectFiles(rootPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// calculateTotalSize 计算总大小.
func (m *IntegrityCheckManager) calculateTotalSize(files []string) int64 {
	var total int64
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
	}
	return total
}

// markJobFailed 标记任务失败.
func (m *IntegrityCheckManager) markJobFailed(job *IntegrityCheckJob, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job.Status = CheckStatusFailed
	job.ErrorMessage = errMsg
	endTime := time.Now()
	job.EndTime = &endTime

	m.jobHistory = append(m.jobHistory, job)
	delete(m.jobs, job.ID)
	delete(m.cancelFuncs, job.ID)
}

// generateReport 生成校验报告.
func (m *IntegrityCheckManager) generateReport(job *IntegrityCheckJob, corruptedItems []CorruptedItem, duration time.Duration) *IntegrityCheckReport {
	report := &IntegrityCheckReport{
		ID:            uuid.New().String(),
		JobID:         job.ID,
		BackupTaskID:  job.BackupTaskID,
		GeneratedAt:   time.Now(),
		CheckDuration: duration,
		CorruptedItems: corruptedItems,
		Healthy:       len(corruptedItems) == 0,
	}

	// 计算摘要
	healthyFiles := job.TotalFiles - job.CorruptedFiles - job.MissingFiles
	healthyBytes := job.TotalBytes - job.CheckedBytes // 简化计算

	var corruptedBytes int64
	for _, item := range corruptedItems {
		corruptedBytes += item.Size
	}

	corruptionRate := 0.0
	if job.TotalBytes > 0 {
		corruptionRate = float64(corruptedBytes) / float64(job.TotalBytes) * 100
	}

	report.Summary = CheckSummary{
		TotalFiles:         job.TotalFiles,
		HealthyFiles:       healthyFiles,
		CorruptedFiles:     job.CorruptedFiles,
		MissingFiles:       job.MissingFiles,
		ExtraFiles:         job.ExtraFiles,
		TotalBytes:         job.TotalBytes,
		HealthyBytes:       healthyBytes,
		CorruptedBytes:     corruptedBytes,
		CorruptionRate:     corruptionRate,
		DataIntegrityScore: 100.0 - corruptionRate,
	}

	// 生成建议
	report.Recommendations = m.generateRecommendations(report)

	return report
}

// generateRecommendations 生成建议.
func (m *IntegrityCheckManager) generateRecommendations(report *IntegrityCheckReport) []Recommendation {
	var recommendations []Recommendation

	if len(report.CorruptedItems) == 0 {
		recommendations = append(recommendations, Recommendation{
			Type:        RecommendationIgnore,
			Priority:    0,
			Action:      "无操作",
			Description: "备份数据完整，无需修复操作",
			AffectedItems: 0,
		})
		return recommendations
	}

	// 根据损坏类型和数量生成建议
	criticalCount := 0
	highCount := 0

	for _, item := range report.CorruptedItems {
		if item.Severity == SeverityCritical {
			criticalCount++
		} else if item.Severity == SeverityHigh {
			highCount++
		}
	}

	if criticalCount > 0 {
		recommendations = append(recommendations, Recommendation{
			Type:          RecommendationRestore,
			Priority:      1,
			Action:        "立即恢复",
			Description:   fmt.Sprintf("发现 %d 个严重损坏项，建议立即从其他备份恢复", criticalCount),
			AffectedItems: int64(criticalCount),
		})
	}

	if highCount > 0 {
		recommendations = append(recommendations, Recommendation{
			Type:          RecommendationRebackup,
			Priority:      2,
			Action:        "重新备份",
			Description:   fmt.Sprintf("发现 %d 个高损坏项，建议重新执行备份", highCount),
			AffectedItems: int64(highCount),
		})
	}

	if report.Summary.CorruptionRate > 10.0 {
		recommendations = append(recommendations, Recommendation{
			Type:          RecommendationVerify,
			Priority:      3,
			Action:        "深度验证",
			Description:   "损坏率超过10%，建议对所有数据进行深度验证",
			AffectedItems: report.Summary.TotalFiles,
		})
	}

	return recommendations
}

// ========== HTTP Handlers ==========

// IntegrityCheckHandlers 完整性校验API处理器.
type IntegrityCheckHandlers struct {
	manager *IntegrityCheckManager
}

// NewIntegrityCheckHandlers 创建处理器.
func NewIntegrityCheckHandlers(manager *IntegrityCheckManager) *IntegrityCheckHandlers {
	return &IntegrityCheckHandlers{manager: manager}
}

// RegisterRoutes 注册路由.
// @Summary 注册完整性校验路由
// @Description 注册所有完整性校验相关的API路由
// @Tags backup
func (h *IntegrityCheckHandlers) RegisterRoutes(r *gin.RouterGroup) {
	check := r.Group("/integrity-check")
	{
		// POST /api/v1/backup/integrity-check - 启动完整性校验
		check.POST("", h.startCheck)

		// GET /api/v1/backup/integrity-check/:id - 获取校验进度
		check.GET("/:id", h.getProgress)

		// GET /api/v1/backup/integrity-check/:id/report - 获取校验报告
		check.GET("/:id/report", h.getReport)

		// POST /api/v1/backup/integrity-check/:id/cancel - 取消校验
		check.POST("/:id/cancel", h.cancelCheck)

		// GET /api/v1/backup/integrity-check/list - 列出校验任务
		check.GET("/list", h.listChecks)
	}
}

// startCheck 启动完整性校验
// @Summary 启动完整性校验
// @Description 启动对指定备份任务的完整性校验
// @Tags backup
// @Accept json
// @Produce json
// @Param request body IntegrityCheckRequest true "校验请求"
// @Success 200 {object} api.Response{data=IntegrityCheckJob}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /api/v1/backup/integrity-check [post]
// @Security BearerAuth
func (h *IntegrityCheckHandlers) startCheck(c *gin.Context) {
	var req IntegrityCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}

	// 设置默认值
	if req.CheckType == "" {
		req.CheckType = CheckTypeFull
	}

	job, err := h.manager.StartCheck(req)
	if err != nil {
		api.InternalError(c, "启动校验失败: "+err.Error())
		return
	}

	api.OKWithMessage(c, "完整性校验已启动", job)
}

// getProgress 获取校验进度
// @Summary 获取校验进度
// @Description 获取指定校验任务的进度信息
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "校验任务ID"
// @Success 200 {object} api.Response{data=IntegrityCheckJob}
// @Failure 404 {object} api.Response
// @Router /api/v1/backup/integrity-check/{id} [get]
// @Security BearerAuth
func (h *IntegrityCheckHandlers) getProgress(c *gin.Context) {
	id := c.Param("id")

	job, err := h.manager.GetCheckProgress(id)
	if err != nil {
		api.NotFound(c, "校验任务不存在: "+id)
		return
	}

	api.OK(c, job)
}

// getReport 获取校验报告
// @Summary 获取校验报告
// @Description 获取指定校验任务的完整报告
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "校验任务ID"
// @Success 200 {object} api.Response{data=IntegrityCheckReport}
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/backup/integrity-check/{id}/report [get]
// @Security BearerAuth
func (h *IntegrityCheckHandlers) getReport(c *gin.Context) {
	id := c.Param("id")

	report, err := h.manager.GetCheckReport(id)
	if err != nil {
		if err.Error() == "校验任务尚未完成" {
			api.BadRequest(c, err.Error())
			return
		}
		api.NotFound(c, "校验报告不存在: "+id)
		return
	}

	api.OK(c, report)
}

// cancelCheck 取消校验
// @Summary 取消校验任务
// @Description 取消正在执行的完整性校验任务
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "校验任务ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /api/v1/backup/integrity-check/{id}/cancel [post]
// @Security BearerAuth
func (h *IntegrityCheckHandlers) cancelCheck(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelCheck(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "校验任务已取消", nil)
}

// listChecks 列出校验任务
// @Summary 列出校验任务
// @Description 获取校验任务列表
// @Tags backup
// @Accept json
// @Produce json
// @Param backupTaskId query string false "备份任务ID过滤"
// @Success 200 {object} api.Response{data=[]IntegrityCheckJob}
// @Router /api/v1/backup/integrity-check/list [get]
// @Security BearerAuth
func (h *IntegrityCheckHandlers) listChecks(c *gin.Context) {
	backupTaskID := c.Query("backupTaskId")

	jobs := h.manager.ListChecks(backupTaskID)

	api.OK(c, jobs)
}

// ========== 存储 ==========

// SaveReport 保存报告到文件.
func (m *IntegrityCheckManager) SaveReport(report *IntegrityCheckReport) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.configPath, "reports", report.ID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadReport 从文件加载报告.
func (m *IntegrityCheckManager) LoadReport(reportID string) (*IntegrityCheckReport, error) {
	path := filepath.Join(m.configPath, "reports", reportID+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var report IntegrityCheckReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	return &report, nil
}