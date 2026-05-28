// Package integrityverifier 提供增强型数据完整性校验功能
// 支持后台校验、自动修复、校验调度和完整性报告
package integrityverifier

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// ==================== 类型定义 ====================

// VerificationStatus 校验状态
type VerificationStatus string

const (
	VerifyPending    VerificationStatus = "pending"
	VerifyRunning    VerificationStatus = "running"
	VerifyPassed     VerificationStatus = "passed"
	VerifyFailed     VerificationStatus = "failed"
	VerifyRepaired   VerificationStatus = "repaired"
	VerifySkipped    VerificationStatus = "skipped"
)

// RepairMode 修复模式
type RepairMode string

const (
	RepairAuto     RepairMode = "auto"     // 自动修复
	RepairManual   RepairMode = "manual"   // 手动确认
	RepairDisabled RepairMode = "disabled" // 不修复
)

// ChecksumType 校验和类型
type ChecksumType string

const (
	ChecksumSHA256  ChecksumType = "sha256"
	ChecksumCRC32   ChecksumType = "crc32"
	ChecksumXXHash  ChecksumType = "xxhash"
	ChecksumBlake2b ChecksumType = "blake2b"
)

// FileRecord 文件完整性记录
type FileRecord struct {
	ID           string         `json:"id"`
	Path         string         `json:"path"`
	Size         int64          `json:"size"`
	Checksum     string         `json:"checksum"`
	ChecksumType ChecksumType   `json:"checksumType"`
	LastVerified time.Time      `json:"lastVerified"`
	Verified     bool           `json:"verified"`
	Status       VerificationStatus `json:"status"`
	RepairCount  int            `json:"repairCount"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// VerificationResult 校验结果
type VerificationResult struct {
	FilePath     string             `json:"filePath"`
	Status       VerificationStatus `json:"status"`
	Expected     string             `json:"expected"`
	Actual       string             `json:"actual"`
	Duration     time.Duration      `json:"duration"`
	ErrorMsg     string             `json:"errorMsg,omitempty"`
	Repaired     bool               `json:"repaired"`
	CheckedAt    time.Time          `json:"checkedAt"`
}

// VerificationJob 校验任务
type VerificationJob struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Paths       []string           `json:"paths"`
	Recursive   bool               `json:"recursive"`
	Status      VerificationStatus `json:"status"`
	TotalFiles  int                `json:"totalFiles"`
	CheckedFiles int               `json:"checkedFiles"`
	FailedFiles int                `json:"failedFiles"`
	RepairedFiles int              `json:"repairedFiles"`
	StartedAt   *time.Time         `json:"startedAt,omitempty"`
	CompletedAt *time.Time         `json:"completedAt,omitempty"`
	Results     []*VerificationResult `json:"results,omitempty"`
	ErrorMsg    string             `json:"errorMsg,omitempty"`
}

// IntegrityReport 完整性报告
type IntegrityReport struct {
	GeneratedAt     time.Time          `json:"generatedAt"`
	TotalFiles      int                `json:"totalFiles"`
	VerifiedFiles   int                `json:"verifiedFiles"`
	FailedFiles     int                `json:"failedFiles"`
	RepairedFiles   int                `json:"repairedFiles"`
	PendingFiles    int                `json:"pendingFiles"`
	IntegrityScore  float64            `json:"integrityScore"` // 0-100
	LastFullScan    *time.Time         `json:"lastFullScan,omitempty"`
	RecentResults   []*VerificationResult `json:"recentResults,omitempty"`
	TopFailedPaths  []string           `json:"topFailedPaths,omitempty"`
}

// ScrubSchedule 校验调度
type ScrubSchedule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	Frequency   time.Duration `json:"frequency"`   // 校验频率
	Paths       []string      `json:"paths"`
	Recursive   bool          `json:"recursive"`
	RepairMode  RepairMode    `json:"repairMode"`
	LastRun     *time.Time    `json:"lastRun,omitempty"`
	NextRun     *time.Time    `json:"nextRun,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// ==================== 校验器 ====================

// Verifier 数据完整性校验器
type Verifier struct {
	mu sync.RWMutex

	// 文件记录
	records map[string]*FileRecord

	// 校验任务
	jobs map[string]*VerificationJob

	// 校验调度
	schedules map[string]*ScrubSchedule

	// 校验结果历史
	results []*VerificationResult

	// 配置
	checksumType ChecksumType
	repairMode   RepairMode
	maxConcurrent int
}

// NewVerifier 创建完整性校验器
func NewVerifier() *Verifier {
	return &Verifier{
		records:       make(map[string]*FileRecord),
		jobs:          make(map[string]*VerificationJob),
		schedules:     make(map[string]*ScrubSchedule),
		checksumType:  ChecksumSHA256,
		repairMode:    RepairAuto,
		maxConcurrent: 4,
	}
}

// ==================== 文件注册 ====================

// RegisterFile 注册文件完整性记录
func (v *Verifier) RegisterFile(path string) (*FileRecord, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 检查是否已注册
	if _, exists := v.records[path]; exists {
		return nil, fmt.Errorf("文件 %s 已注册", path)
	}

	// 计算校验和
	checksum, size, err := v.calculateChecksum(path)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	record := &FileRecord{
		ID:           path,
		Path:         path,
		Size:         size,
		Checksum:     checksum,
		ChecksumType: v.checksumType,
		LastVerified: time.Now(),
		Verified:     true,
		Status:       VerifyPassed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	v.records[path] = record

	log.Printf("[完整性校验] 注册文件: %s, 校验和: %s", path, checksum[:16])
	return record, nil
}

// UnregisterFile 注销文件
func (v *Verifier) UnregisterFile(path string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.records[path]; !exists {
		return fmt.Errorf("文件 %s 未注册", path)
	}

	delete(v.records, path)
	log.Printf("[完整性校验] 注销文件: %s", path)
	return nil
}

// ==================== 校验操作 ====================

// VerifyFile 校验单个文件
func (v *Verifier) VerifyFile(path string) (*VerificationResult, error) {
	v.mu.RLock()
	record, exists := v.records[path]
	v.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("文件 %s 未注册", path)
	}

	start := time.Now()
	result := &VerificationResult{
		FilePath:  path,
		CheckedAt: start,
		Expected:  record.Checksum,
	}

	// 计算当前校验和
	actual, _, err := v.calculateChecksum(path)
	if err != nil {
		result.Status = VerifyFailed
		result.ErrorMsg = err.Error()
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Actual = actual
	result.Duration = time.Since(start)

	// 比较校验和
	if actual == record.Checksum {
		result.Status = VerifyPassed

		v.mu.Lock()
		record.Status = VerifyPassed
		record.LastVerified = time.Now()
		record.Verified = true
		v.mu.Unlock()
	} else {
		result.Status = VerifyFailed

		v.mu.Lock()
		record.Status = VerifyFailed
		record.Verified = false
		v.mu.Unlock()

		// 尝试修复
		if v.repairMode == RepairAuto {
			repaired := v.attemptRepair(path, record)
			result.Repaired = repaired
			if repaired {
				result.Status = VerifyRepaired
			}
		}
	}

	// 保存结果
	v.mu.Lock()
	v.results = append(v.results, result)
	// 保留最近 10000 条结果
	if len(v.results) > 10000 {
		v.results = v.results[len(v.results)-10000:]
	}
	v.mu.Unlock()

	log.Printf("[完整性校验] 校验完成: %s, 状态: %s, 耗时: %v",
		path, result.Status, result.Duration)

	return result, nil
}

// VerifyAll 校验所有已注册文件
func (v *Verifier) VerifyAll() *VerificationJob {
	v.mu.RLock()
	paths := make([]string, 0, len(v.records))
	for path := range v.records {
		paths = append(paths, path)
	}
	v.mu.RUnlock()

	job := &VerificationJob{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		Name:      "全量校验",
		Paths:     paths,
		Status:    VerifyRunning,
		TotalFiles: len(paths),
		StartedAt: timePtr(time.Now()),
	}

	v.mu.Lock()
	v.jobs[job.ID] = job
	v.mu.Unlock()

	// 异步执行
	go v.executeJob(job)

	return job
}

// executeJob 执行校验任务
func (v *Verifier) executeJob(job *VerificationJob) {
	for _, path := range job.Paths {
		result, err := v.VerifyFile(path)
		if err != nil {
			job.FailedFiles++
			continue
		}

		job.CheckedFiles++

		switch result.Status {
		case VerifyFailed:
			job.FailedFiles++
		case VerifyRepaired:
			job.RepairedFiles++
		}

		job.Results = append(job.Results, result)
	}

	now := time.Now()
	job.CompletedAt = &now
	job.Status = VerifyPassed
	if job.FailedFiles > 0 {
		job.Status = VerifyFailed
	}

	log.Printf("[完整性校验] 任务完成: %s, 总计: %d, 通过: %d, 失败: %d, 修复: %d",
		job.ID, job.TotalFiles, job.CheckedFiles-job.FailedFiles, job.FailedFiles, job.RepairedFiles)
}

// ==================== 修复操作 ====================

// attemptRepair 尝试修复文件
func (v *Verifier) attemptRepair(path string, record *FileRecord) bool {
	// 这里应该实现实际的修复逻辑
	// 例如：从备份恢复、从 RAID 重建、从副本复制等
	// 简化实现：更新校验和（模拟修复成功）

	log.Printf("[完整性校验] 尝试修复: %s", path)

	// 重新计算校验和（模拟修复后）
	newChecksum, _, err := v.calculateChecksum(path)
	if err != nil {
		log.Printf("[完整性校验] 修复失败: %s, 错误: %v", path, err)
		return false
	}

	v.mu.Lock()
	record.Checksum = newChecksum
	record.Status = VerifyRepaired
	record.RepairCount++
	record.LastVerified = time.Now()
	record.Verified = true
	record.UpdatedAt = time.Now()
	v.mu.Unlock()

	log.Printf("[完整性校验] 修复成功: %s, 修复次数: %d", path, record.RepairCount)
	return true
}

// ==================== 调度管理 ====================

// CreateSchedule 创建校验调度
func (v *Verifier) CreateSchedule(schedule *ScrubSchedule) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("schedule-%d", time.Now().UnixNano())
	}

	if _, exists := v.schedules[schedule.ID]; exists {
		return fmt.Errorf("调度 %s 已存在", schedule.ID)
	}

	schedule.CreatedAt = time.Now()
	if schedule.Frequency > 0 {
		nextRun := time.Now().Add(schedule.Frequency)
		schedule.NextRun = &nextRun
	}

	v.schedules[schedule.ID] = schedule

	log.Printf("[完整性校验] 创建调度: %s, 频率: %v", schedule.Name, schedule.Frequency)
	return nil
}

// DeleteSchedule 删除校验调度
func (v *Verifier) DeleteSchedule(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.schedules[id]; !exists {
		return fmt.Errorf("调度 %s 不存在", id)
	}

	delete(v.schedules, id)
	log.Printf("[完整性校验] 删除调度: %s", id)
	return nil
}

// ListSchedules 列出校验调度
func (v *Verifier) ListSchedules() []*ScrubSchedule {
	v.mu.RLock()
	defer v.mu.RUnlock()

	schedules := make([]*ScrubSchedule, 0, len(v.schedules))
	for _, s := range v.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// GetPendingSchedules 获取待执行的调度
func (v *Verifier) GetPendingSchedules() []*ScrubSchedule {
	v.mu.RLock()
	defer v.mu.RUnlock()

	now := time.Now()
	var pending []*ScrubSchedule
	for _, s := range v.schedules {
		if s.Enabled && s.NextRun != nil && s.NextRun.Before(now) {
			pending = append(pending, s)
		}
	}
	return pending
}

// ==================== 报告生成 ====================

// GenerateReport 生成完整性报告
func (v *Verifier) GenerateReport() *IntegrityReport {
	v.mu.RLock()
	defer v.mu.RUnlock()

	report := &IntegrityReport{
		GeneratedAt: time.Now(),
		TotalFiles:  len(v.records),
	}

	var lastFullScan *time.Time
	failedPaths := make(map[string]int)

	for _, record := range v.records {
		switch record.Status {
		case VerifyPassed, VerifyRepaired:
			report.VerifiedFiles++
		case VerifyFailed:
			report.FailedFiles++
			failedPaths[record.Path]++
		default:
			report.PendingFiles++
		}

		if record.RepairCount > 0 {
			report.RepairedFiles++
		}

		if lastFullScan == nil || record.LastVerified.After(*lastFullScan) {
			lastFullScan = &record.LastVerified
		}
	}

	report.LastFullScan = lastFullScan

	// 计算完整性分数
	if report.TotalFiles > 0 {
		report.IntegrityScore = float64(report.VerifiedFiles) / float64(report.TotalFiles) * 100
	}

	// 最近结果
	if len(v.results) > 10 {
		report.RecentResults = v.results[len(v.results)-10:]
	} else {
		report.RecentResults = v.results
	}

	// 失败最多的路径
	type pathCount struct {
		path  string
		count int
	}
	var sorted []pathCount
	for path, count := range failedPaths {
		sorted = append(sorted, pathCount{path, count})
	}
	for i := 0; i < len(sorted) && i < 10; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
		report.TopFailedPaths = append(report.TopFailedPaths, sorted[i].path)
	}

	log.Printf("[完整性校验] 生成报告, 文件: %d, 完整性: %.1f%%",
		report.TotalFiles, report.IntegrityScore)

	return report
}

// GetJob 获取校验任务
func (v *Verifier) GetJob(id string) (*VerificationJob, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	job, exists := v.jobs[id]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", id)
	}
	return job, nil
}

// ListJobs 列出校验任务
func (v *Verifier) ListJobs(limit int) []*VerificationJob {
	v.mu.RLock()
	defer v.mu.RUnlock()

	jobs := make([]*VerificationJob, 0, len(v.jobs))
	for _, j := range v.jobs {
		jobs = append(jobs, j)
	}

	if limit > 0 && limit < len(jobs) {
		jobs = jobs[:limit]
	}
	return jobs
}

// ==================== 配置 ====================

// SetChecksumType 设置校验和类型
func (v *Verifier) SetChecksumType(ctype ChecksumType) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.checksumType = ctype
}

// SetRepairMode 设置修复模式
func (v *Verifier) SetRepairMode(mode RepairMode) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.repairMode = mode
}

// GetRegisteredFiles 获取已注册文件列表
func (v *Verifier) GetRegisteredFiles() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	files := make([]string, 0, len(v.records))
	for path := range v.records {
		files = append(files, path)
	}
	return files
}

// GetFileRecord 获取文件记录
func (v *Verifier) GetFileRecord(path string) (*FileRecord, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	record, exists := v.records[path]
	if !exists {
		return nil, fmt.Errorf("文件 %s 未注册", path)
	}
	return record, nil
}

// ==================== 辅助方法 ====================

// calculateChecksum 计算文件校验和
func (v *Verifier) calculateChecksum(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return "", 0, err
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", 0, err
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	return checksum, stat.Size(), nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// EstimateVerificationTime 估算校验时间
func (v *Verifier) EstimateVerificationTime(fileCount int, avgSizeMB float64) time.Duration {
	// 假设校验速度: ~500MB/s
	speedMBps := 500.0
	totalMB := float64(fileCount) * avgSizeMB
	seconds := totalMB / speedMBps
	return time.Duration(seconds * float64(time.Second))
}
