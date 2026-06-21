package hdddedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// NewEngine 创建去重压缩引擎
func NewEngine(config *HDDDedupConfig) *Engine {
	if config == nil {
		config = DefaultHDDDedupConfig()
	}

	return &Engine{
		config:     config,
		jobs:       make(map[string]*DedupJob),
		policies:   make(map[string]*CompressPolicy),
		schedules:  make(map[string]*DedupSchedule),
		reports:    make([]*EfficiencyReport, 0),
		chunkIndex: make(map[string]*DedupChunk),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("引擎已在运行")
	}

	e.running = true
	log.Println("[HDDDedup] 去重压缩引擎启动")

	// 启动调度器
	if e.config.ScheduleEnabled {
		go e.scheduleWorker()
	}

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	close(e.stopCh)
	e.running = false
	log.Println("[HDDDedup] 去重压缩引擎停止")
}

// IsRunning 检查是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// CreateDedupJob 创建去重任务
func (e *Engine) CreateDedupJob(targetPath string) (*DedupJob, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil, fmt.Errorf("引擎未运行")
	}

	// 验证路径
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("目标路径不存在: %s", targetPath)
	}

	job := &DedupJob{
		ID:         uuid.New().String(),
		TargetPath: targetPath,
		Status:     JobStatusPending,
		StartTime:  time.Now(),
	}

	e.jobs[job.ID] = job

	// 异步执行去重任务
	go e.executeDedupJob(job)

	log.Printf("[HDDDedup] 创建去重任务: %s -> %s", job.ID, targetPath)
	return job, nil
}

// executeDedupJob 执行去重任务
func (e *Engine) executeDedupJob(job *DedupJob) {
	e.mu.Lock()
	job.Status = JobStatusRunning
	e.mu.Unlock()

	log.Printf("[HDDDedup] 开始执行去重任务: %s", job.ID)

	// 扫描文件
	var totalFiles int64
	var processed int64
	var dedupCount int64
	var savedBytes int64

	err := filepath.Walk(job.TargetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		if info.IsDir() {
			return nil
		}

		totalFiles++

		// 检查是否应该压缩
		if !e.shouldProcess(path, info) {
			return nil
		}

		// 处理文件
		saved, deduped, processErr := e.processFile(path)
		if processErr != nil {
			log.Printf("[HDDDedup] 处理文件失败 %s: %v", path, processErr)
			return nil
		}

		processed++
		dedupCount += deduped
		savedBytes += saved

		// 更新进度
		e.mu.Lock()
		job.TotalFiles = totalFiles
		job.Processed = processed
		job.DedupCount = dedupCount
		job.SavedBytes = savedBytes
		if totalFiles > 0 {
			job.Progress = float64(processed) / float64(totalFiles) * 100
		}
		e.mu.Unlock()

		return nil
	})

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	job.EndTime = &now

	if err != nil {
		job.Status = JobStatusFailed
		job.ErrorMsg = err.Error()
		log.Printf("[HDDDedup] 去重任务失败: %s - %v", job.ID, err)
	} else {
		job.Status = JobStatusCompleted
		job.TotalFiles = totalFiles
		job.Processed = processed
		job.DedupCount = dedupCount
		job.SavedBytes = savedBytes
		job.Progress = 100
		log.Printf("[HDDDedup] 去重任务完成: %s - 处理 %d 文件, 去重 %d 块, 节省 %d 字节",
			job.ID, processed, dedupCount, savedBytes)
	}

	// 生成报告
	e.generateReport(job)
}

// shouldProcess 检查文件是否应该处理
func (e *Engine) shouldProcess(path string, info os.FileInfo) bool {
	// 检查文件大小
	if info.Size() < int64(e.config.ChunkSize) {
		return false
	}

	// 检查压缩策略
	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}
		if matchPolicy(path, info, policy) {
			return true
		}
	}

	// 默认处理
	return true
}

// matchPolicy 检查文件是否匹配策略
func matchPolicy(path string, info os.FileInfo, policy *CompressPolicy) bool {
	// 检查文件大小
	if info.Size() < policy.MinSize {
		return false
	}

	// 检查扩展名
	if len(policy.Extensions) > 0 {
		ext := filepath.Ext(path)
		matched := false
		for _, allowedExt := range policy.Extensions {
			if ext == allowedExt {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// processFile 处理单个文件
func (e *Engine) processFile(path string) (savedBytes int64, dedupCount int64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	buf := make([]byte, e.config.ChunkSize)
	var totalSaved int64
	var totalDedup int64

	for {
		n, readErr := file.Read(buf)
		if n == 0 {
			break
		}
		if readErr != nil && readErr != io.EOF {
			return totalSaved, totalDedup, readErr
		}

		chunk := buf[:n]
		hash := sha256.Sum256(chunk)
		hashStr := hex.EncodeToString(hash[:])

		e.mu.Lock()
		existing, exists := e.chunkIndex[hashStr]
		if exists {
			// 重复块
			existing.RefCount++
			totalDedup++
			totalSaved += int64(n)
		} else {
			// 新块
			e.chunkIndex[hashStr] = &DedupChunk{
				Hash:     hashStr,
				Size:     n,
				RefCount: 1,
			}
		}
		e.mu.Unlock()

		if readErr == io.EOF {
			break
		}
	}

	return totalSaved, totalDedup, nil
}

// generateReport 生成效率报告
func (e *Engine) generateReport(job *DedupJob) {
	report := &EfficiencyReport{
		ID:          uuid.New().String(),
		GeneratedAt: time.Now(),
		TotalData:   job.TotalFiles * int64(e.config.ChunkSize), // 估算
		DedupedData: job.SavedBytes,
		TotalSaved:  job.SavedBytes,
		ChunkCount:  job.TotalFiles,
		UniqueChunks: job.TotalFiles - job.DedupCount,
	}

	if report.TotalData > 0 {
		report.DedupRatio = float64(report.TotalSaved) / float64(report.TotalData) * 100
	}

	e.reports = append(e.reports, report)
}

// GetJob 获取任务状态
func (e *Engine) GetJob(jobID string) (*DedupJob, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	job, exists := e.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}

	return job, nil
}

// ListJobs 列出所有任务
func (e *Engine) ListJobs() []*DedupJob {
	e.mu.RLock()
	defer e.mu.RUnlock()

	jobs := make([]*DedupJob, 0, len(e.jobs))
	for _, job := range e.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetEfficiencyReport 获取效率报告
func (e *Engine) GetEfficiencyReport() *EfficiencyReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.reports) == 0 {
		return &EfficiencyReport{
			GeneratedAt: time.Now(),
		}
	}

	return e.reports[len(e.reports)-1]
}

// scheduleWorker 调度工作器
func (e *Engine) scheduleWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkSchedules()
		}
	}
}

// checkSchedules 检查调度任务
func (e *Engine) checkSchedules() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	for _, schedule := range e.schedules {
		if !schedule.Enabled {
			continue
		}

		if schedule.NextRun != nil && now.After(*schedule.NextRun) {
			log.Printf("[HDDDedup] 触发调度任务: %s", schedule.Name)
			// 这里应该解析 cron 表达式并计算下次运行时间
			// 简化实现：每小时执行一次
			nextRun := now.Add(1 * time.Hour)
			schedule.NextRun = &nextRun
			schedule.LastRun = &now

			go func(path string) {
				e.CreateDedupJob(path)
			}(schedule.TargetPath)
		}
	}
}

// CreateSchedule 创建调度
func (e *Engine) CreateSchedule(schedule *DedupSchedule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	// 设置首次运行时间
	now := time.Now()
	schedule.NextRun = &now

	e.schedules[schedule.ID] = schedule
	log.Printf("[HDDDedup] 创建调度: %s - %s", schedule.ID, schedule.Name)

	return nil
}

// CreatePolicy 创建压缩策略
func (e *Engine) CreatePolicy(policy *CompressPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}

	e.policies[policy.ID] = policy
	log.Printf("[HDDDedup] 创建压缩策略: %s - %s", policy.ID, policy.Name)

	return nil
}

// ListPolicies 列出压缩策略
func (e *Engine) ListPolicies() []*CompressPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*CompressPolicy, 0, len(e.policies))
	for _, policy := range e.policies {
		policies = append(policies, policy)
	}
	return policies
}

// UpdateConfig 更新配置
func (e *Engine) UpdateConfig(config *HDDDedupConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config = config
	log.Printf("[HDDDedup] 配置已更新")
}

// GetConfig 获取配置
func (e *Engine) GetConfig() *HDDDedupConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.config
}

// GetChunkStats 获取数据块统计
func (e *Engine) GetChunkStats() (totalChunks int, uniqueChunks int, totalSize int64) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, chunk := range e.chunkIndex {
		totalChunks += chunk.RefCount
		uniqueChunks++
		totalSize += int64(chunk.Size) * int64(chunk.RefCount)
	}

	return totalChunks, uniqueChunks, totalSize
}
