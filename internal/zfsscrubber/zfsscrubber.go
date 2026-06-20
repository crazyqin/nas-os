// Package zfsscrubber 实现 ZFS 智能数据清洗模块，对标 TrueNAS 数据保护能力
//
// 提供定时清洗调度、数据完整性校验、自动修复、清洗报告生成和存储健康监控功能。
package zfsscrubber

import (
	"fmt"
	"sync"
	"time"
)

// ScrubFrequency 清洗频率
type ScrubFrequency string

const (
	FrequencyDaily   ScrubFrequency = "daily"
	FrequencyWeekly  ScrubFrequency = "weekly"
	FrequencyMonthly ScrubFrequency = "monthly"
)

// ScrubStatus 清洗状态
type ScrubStatus string

const (
	ScrubPending    ScrubStatus = "pending"
	ScrubRunning    ScrubStatus = "running"
	ScrubCompleted  ScrubStatus = "completed"
	ScrubFailed     ScrubStatus = "failed"
	ScrubCancelled  ScrubStatus = "cancelled"
)

// ChecksumAlgorithm 校验和算法
type ChecksumAlgorithm string

const (
	ChecksumSHA256   ChecksumAlgorithm = "sha256"
	ChecksumFletcher4 ChecksumAlgorithm = "fletcher4"
	ChecksumSHA512   ChecksumAlgorithm = "sha512"
)

// HealthLevel 健康等级
type HealthLevel string

const (
	HealthGood      HealthLevel = "good"
	HealthWarning   HealthLevel = "warning"
	HealthCritical  HealthLevel = "critical"
	HealthUnknown   HealthLevel = "unknown"
)

// ScrubSchedule 清洗调度配置
type ScrubSchedule struct {
	ID        string         `json:"id"`
	PoolID    string         `json:"pool_id"`
	Frequency ScrubFrequency `json:"frequency"`
	DayOfWeek int            `json:"day_of_week"` // 0-6, 0=Sunday (仅 weekly 有效)
	DayOfMonth int           `json:"day_of_month"` // 1-31 (仅 monthly 有效)
	Hour      int            `json:"hour"`         // 0-23
	Enabled   bool           `json:"enabled"`
	NextRun   time.Time      `json:"next_run"`
	LastRun   *time.Time     `json:"last_run,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// ScrubJob 清洗任务实例
type ScrubJob struct {
	ID           string        `json:"id"`
	ScheduleID   string        `json:"schedule_id"`
	PoolID       string        `json:"pool_id"`
	Status       ScrubStatus   `json:"status"`
	Progress     float64       `json:"progress"`
	StartedAt    time.Time     `json:"started_at"`
	FinishedAt   *time.Time    `json:"finished_at,omitempty"`
	Duration     time.Duration `json:"duration"`
	BytesScanned int64         `json:"bytes_scanned"`
	BytesTotal   int64         `json:"bytes_total"`
	Errors       int           `json:"errors"`
	Repaired     int           `json:"repaired"`
}

// DataBlock 数据块信息
type DataBlock struct {
	ID            string            `json:"id"`
	PoolID        string            `json:"pool_id"`
	Dataset       string            `json:"dataset"`
	BlockNumber   int64             `json:"block_number"`
	Size          int64             `json:"size"`
	Checksum      string            `json:"checksum"`
	Algorithm     ChecksumAlgorithm `json:"algorithm"`
	Valid         bool              `json:"valid"`
	LastVerified  time.Time         `json:"last_verified"`
	RepairCount   int               `json:"repair_count"`
}

// IntegrityResult 完整性校验结果
type IntegrityResult struct {
	BlockID       string    `json:"block_id"`
	PoolID        string    `json:"pool_id"`
	Dataset       string    `json:"dataset"`
	BlockNumber   int64     `json:"block_number"`
	Expected      string    `json:"expected_checksum"`
	Actual        string    `json:"actual_checksum"`
	Valid         bool      `json:"valid"`
	Repaired      bool      `json:"repaired"`
	RepairSource  string    `json:"repair_source,omitempty"` // 修复来源：mirror, raidz, etc.
	Timestamp     time.Time `json:"timestamp"`
}

// RepairAction 修复动作
type RepairAction struct {
	ID            string        `json:"id"`
	BlockID       string        `json:"block_id"`
	PoolID        string        `json:"pool_id"`
	Action        string        `json:"action"` // "copy_from_mirror", "reconstruct_raidz", "replace_disk"
	Source        string        `json:"source"`
	Target        string        `json:"target"`
	Status        string        `json:"status"` // "pending", "in_progress", "completed", "failed"
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    *time.Time    `json:"finished_at,omitempty"`
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
}

// ScrubReport 清洗报告
type ScrubReport struct {
	ID              string             `json:"id"`
	PoolID          string             `json:"pool_id"`
	PoolName        string             `json:"pool_name"`
	JobID           string             `json:"job_id"`
	StartedAt       time.Time          `json:"started_at"`
	FinishedAt      time.Time          `json:"finished_at"`
	Duration        time.Duration      `json:"duration"`
	BlocksScanned   int64              `json:"blocks_scanned"`
	BlocksTotal     int64              `json:"blocks_total"`
	BytesScanned    int64              `json:"bytes_scanned"`
	ErrorsFound     int                `json:"errors_found"`
	ErrorsRepaired  int                `json:"errors_repaired"`
	ErrorsUnrepairable int            `json:"errors_unrepairable"`
	IntegrityResults []*IntegrityResult `json:"integrity_results,omitempty"`
	RepairActions   []*RepairAction    `json:"repair_actions,omitempty"`
	Recommendations []string           `json:"recommendations,omitempty"`
	Timestamp       time.Time          `json:"timestamp"`
}

// DiskSMART 磁盘 SMART 信息
type DiskSMART struct {
	DevicePath          string    `json:"device_path"`
	Model               string    `json:"model"`
	Serial              string    `json:"serial"`
	Capacity            int64     `json:"capacity"`
	Temperature         float64   `json:"temperature"`
	PowerOnHours        int64     `json:"power_on_hours"`
	ReallocatedSectors  int64     `json:"reallocated_sectors"`
	PendingSectors      int64     `json:"pending_sectors"`
	OfflineUncorrectable int64    `json:"offline_uncorrectable"`
	HealthStatus        HealthLevel `json:"health_status"`
	LastChecked         time.Time `json:"last_checked"`
}

// PoolHealth 池健康状态
type PoolHealth struct {
	PoolID          string       `json:"pool_id"`
	PoolName        string       `json:"pool_name"`
	OverallHealth   HealthLevel  `json:"overall_health"`
	Status          string       `json:"status"`
	TotalSize       int64        `json:"total_size"`
	UsedSize        int64        `json:"used_size"`
	FreeSize        int64        `json:"free_size"`
	Fragmentation   float64      `json:"fragmentation"`
	ScrubErrors     int          `json:"scrub_errors"`
	ChecksumErrors  int          `json:"checksum_errors"`
	DegradedDevices int          `json:"degraded_devices"`
	FailedDevices   int          `json:"failed_devices"`
	Disks           []*DiskSMART `json:"disks"`
	LastScrub       *time.Time   `json:"last_scrub,omitempty"`
	Timestamp       time.Time    `json:"timestamp"`
}

// HealthAlert 健康告警
type HealthAlert struct {
	ID         string      `json:"id"`
	PoolID     string      `json:"pool_id"`
	DevicePath string      `json:"device_path,omitempty"`
	Level      HealthLevel `json:"level"`
	Message    string      `json:"message"`
	Timestamp  time.Time   `json:"timestamp"`
	Acked      bool        `json:"acked"`
}

// ScrubberConfig 清洗器配置
type ScrubberConfig struct {
	DefaultFrequency     ScrubFrequency     `json:"default_frequency"`
	AutoRepairEnabled    bool               `json:"auto_repair_enabled"`
	MaxConcurrentScans   int                `json:"max_concurrent_scans"`
	ScanBandwidthLimit   int64              `json:"scan_bandwidth_limit"` // bytes/sec
	AlertThresholdDays   int                `json:"alert_threshold_days"`
	SMARTCheckInterval   time.Duration      `json:"smart_check_interval"`
	RetryFailedRepairs   int                `json:"retry_failed_repairs"`
}

// ZFSScrubber ZFS 智能数据清洗器
type ZFSScrubber struct {
	mu              sync.RWMutex
	config          *ScrubberConfig
	schedules       map[string]*ScrubSchedule
	jobs            map[string]*ScrubJob
	reports         map[string]*ScrubReport
	poolHealths     map[string]*PoolHealth
	alerts          map[string]*HealthAlert
	blockStore      map[string]*DataBlock    // 模拟数据块存储
	repairActions   map[string]*RepairAction
	stopChan        chan struct{}
	running         bool
}

// NewZFSScrubber 创建 ZFS 智能数据清洗器
func NewZFSScrubber(config *ScrubberConfig) *ZFSScrubber {
	if config == nil {
		config = &ScrubberConfig{
			DefaultFrequency:   FrequencyWeekly,
			AutoRepairEnabled:  true,
			MaxConcurrentScans: 1,
			ScanBandwidthLimit: 0, // 无限制
			AlertThresholdDays: 30,
			SMARTCheckInterval: 24 * time.Hour,
			RetryFailedRepairs: 3,
		}
	}

	return &ZFSScrubber{
		config:        config,
		schedules:     make(map[string]*ScrubSchedule),
		jobs:          make(map[string]*ScrubJob),
		reports:       make(map[string]*ScrubReport),
		poolHealths:   make(map[string]*PoolHealth),
		alerts:        make(map[string]*HealthAlert),
		blockStore:    make(map[string]*DataBlock),
		repairActions: make(map[string]*RepairAction),
		stopChan:      make(chan struct{}),
	}
}

// Start 启动清洗器
func (s *ZFSScrubber) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.scheduleLoop()
}

// Stop 停止清洗器
func (s *ZFSScrubber) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
}

// IsRunning 检查是否运行中
func (s *ZFSScrubber) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// scheduleLoop 调度循环
func (s *ZFSScrubber) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkAndRunScheduledJobs()
		}
	}
}

// checkAndRunScheduledJobs 检查并运行计划任务
func (s *ZFSScrubber) checkAndRunScheduledJobs() {
	s.mu.Lock()
	now := time.Now()
	jobsToRun := make([]*ScrubSchedule, 0)

	for _, schedule := range s.schedules {
		if schedule.Enabled && now.After(schedule.NextRun) {
			jobsToRun = append(jobsToRun, schedule)
		}
	}
	s.mu.Unlock()

	for _, schedule := range jobsToRun {
		s.executeScrubJob(schedule)
	}
}

// CreateSchedule 创建清洗调度
func (s *ZFSScrubber) CreateSchedule(schedule *ScrubSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if schedule.ID == "" {
		return ErrScheduleIDRequired
	}
	if schedule.PoolID == "" {
		return ErrPoolIDRequired
	}
	if _, exists := s.schedules[schedule.ID]; exists {
		return ErrScheduleExists
	}

	// 验证频率配置
	if err := s.validateFrequency(schedule); err != nil {
		return err
	}

	schedule.CreatedAt = time.Now()
	schedule.NextRun = s.calculateNextRun(schedule)
	s.schedules[schedule.ID] = schedule
	return nil
}

// validateFrequency 验证频率配置
func (s *ZFSScrubber) validateFrequency(schedule *ScrubSchedule) error {
	switch schedule.Frequency {
	case FrequencyDaily:
		// 每日任务无需额外验证
	case FrequencyWeekly:
		if schedule.DayOfWeek < 0 || schedule.DayOfWeek > 6 {
			return ErrInvalidDayOfWeek
		}
	case FrequencyMonthly:
		if schedule.DayOfMonth < 1 || schedule.DayOfMonth > 31 {
			return ErrInvalidDayOfMonth
		}
	default:
		return ErrInvalidFrequency
	}

	if schedule.Hour < 0 || schedule.Hour > 23 {
		return ErrInvalidHour
	}
	return nil
}

// calculateNextRun 计算下次运行时间
func (s *ZFSScrubber) calculateNextRun(schedule *ScrubSchedule) time.Time {
	now := time.Now()
	var next time.Time

	switch schedule.Frequency {
	case FrequencyDaily:
		next = time.Date(now.Year(), now.Month(), now.Day(), schedule.Hour, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 0, 1)
		}
	case FrequencyWeekly:
		daysUntilTarget := (schedule.DayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntilTarget == 0 {
			targetTime := time.Date(now.Year(), now.Month(), now.Day(), schedule.Hour, 0, 0, 0, now.Location())
			if targetTime.Before(now) {
				daysUntilTarget = 7
			}
		}
		next = time.Date(now.Year(), now.Month(), now.Day()+daysUntilTarget, schedule.Hour, 0, 0, 0, now.Location())
	case FrequencyMonthly:
		next = time.Date(now.Year(), now.Month(), schedule.DayOfMonth, schedule.Hour, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
	}

	return next
}

// GetSchedule 获取调度配置
func (s *ZFSScrubber) GetSchedule(id string) (*ScrubSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, exists := s.schedules[id]
	if !exists {
		return nil, ErrScheduleNotFound
	}
	return schedule, nil
}

// ListSchedules 列出所有调度
func (s *ZFSScrubber) ListSchedules() []*ScrubSchedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]*ScrubSchedule, 0, len(s.schedules))
	for _, sch := range s.schedules {
		schedules = append(schedules, sch)
	}
	return schedules
}

// UpdateSchedule 更新调度配置
func (s *ZFSScrubber) UpdateSchedule(schedule *ScrubSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.schedules[schedule.ID]; !exists {
		return ErrScheduleNotFound
	}

	if err := s.validateFrequency(schedule); err != nil {
		return err
	}

	schedule.NextRun = s.calculateNextRun(schedule)
	s.schedules[schedule.ID] = schedule
	return nil
}

// DeleteSchedule 删除调度
func (s *ZFSScrubber) DeleteSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.schedules[id]; !exists {
		return ErrScheduleNotFound
	}

	delete(s.schedules, id)
	return nil
}

// ExecuteScrub 手动执行清洗
func (s *ZFSScrubber) ExecuteScrub(poolID string) (*ScrubJob, error) {
	if poolID == "" {
		return nil, ErrPoolIDRequired
	}

	schedule := &ScrubSchedule{
		ID:     fmt.Sprintf("manual_%d", time.Now().UnixNano()),
		PoolID: poolID,
	}

	return s.executeScrubJob(schedule)
}

// executeScrubJob 执行清洗任务
func (s *ZFSScrubber) executeScrubJob(schedule *ScrubSchedule) (*ScrubJob, error) {
	s.mu.Lock()

	// 检查是否已有运行中的任务
	for _, job := range s.jobs {
		if job.PoolID == schedule.PoolID && job.Status == ScrubRunning {
			s.mu.Unlock()
			return nil, ErrScrubAlreadyRunning
		}
	}

	job := &ScrubJob{
		ID:         fmt.Sprintf("job_%d", time.Now().UnixNano()),
		ScheduleID: schedule.ID,
		PoolID:     schedule.PoolID,
		Status:     ScrubRunning,
		StartedAt:  time.Now(),
	}
	s.jobs[job.ID] = job

	// 更新调度的最后运行时间
	if sched, exists := s.schedules[schedule.ID]; exists {
		now := time.Now()
		sched.LastRun = &now
		sched.NextRun = s.calculateNextRun(sched)
	}
	s.mu.Unlock()

	// 异步执行清洗
	go s.runScrubJob(job)

	return job, nil
}

// runScrubJob 运行清洗任务
func (s *ZFSScrubber) runScrubJob(job *ScrubJob) {
	// 模拟清洗过程
	poolBlocks := s.getPoolBlocks(job.PoolID)
	job.BytesTotal = s.calculateTotalBytes(poolBlocks)

	for i, block := range poolBlocks {
		select {
		case <-s.stopChan:
			s.mu.Lock()
			job.Status = ScrubCancelled
			now := time.Now()
			job.FinishedAt = &now
			job.Duration = now.Sub(job.StartedAt)
			s.mu.Unlock()
			return
		default:
		}

		// 校验数据块
		result := s.verifyBlock(block)

		s.mu.Lock()
		job.BytesScanned += block.Size
		job.Progress = float64(i+1) / float64(len(poolBlocks)) * 100

		if !result.Valid {
			job.Errors++
			// 自动修复
			if s.config.AutoRepairEnabled {
				repaired := s.repairBlock(block, result)
				if repaired {
					job.Repaired++
				}
			}
		}
		s.mu.Unlock()

		// 模拟扫描延迟
		time.Sleep(10 * time.Millisecond)
	}

	// 完成任务
	s.mu.Lock()
	job.Status = ScrubCompleted
	now := time.Now()
	job.FinishedAt = &now
	job.Duration = now.Sub(job.StartedAt)
	s.mu.Unlock()

	// 生成报告
	s.generateReport(job)
}

// getPoolBlocks 获取池中的数据块
func (s *ZFSScrubber) getPoolBlocks(poolID string) []*DataBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blocks := make([]*DataBlock, 0)
	for _, block := range s.blockStore {
		if block.PoolID == poolID {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// calculateTotalBytes 计算总字节数
func (s *ZFSScrubber) calculateTotalBytes(blocks []*DataBlock) int64 {
	var total int64
	for _, block := range blocks {
		total += block.Size
	}
	return total
}

// verifyBlock 校验数据块
func (s *ZFSScrubber) verifyBlock(block *DataBlock) *IntegrityResult {
	// 模拟校验和验证
	expected := block.Checksum
	actual := s.computeChecksum(block)

	result := &IntegrityResult{
		BlockID:    block.ID,
		PoolID:     block.PoolID,
		Dataset:    block.Dataset,
		BlockNumber: block.BlockNumber,
		Expected:   expected,
		Actual:     actual,
		Valid:      expected == actual,
		Timestamp:  time.Now(),
	}

	return result
}

// computeChecksum 计算校验和
func (s *ZFSScrubber) computeChecksum(block *DataBlock) string {
	// 模拟校验和计算
	// 实际实现应使用 ZFS 的校验和算法
	return block.Checksum
}

// repairBlock 修复数据块
func (s *ZFSScrubber) repairBlock(block *DataBlock, result *IntegrityResult) bool {
	action := &RepairAction{
		ID:        fmt.Sprintf("repair_%d", time.Now().UnixNano()),
		BlockID:   block.ID,
		PoolID:    block.PoolID,
		Action:    "copy_from_mirror",
		Source:    "mirror_copy",
		Target:    block.ID,
		Status:    "in_progress",
		StartedAt: time.Now(),
	}

	s.mu.Lock()
	s.repairActions[action.ID] = action
	s.mu.Unlock()

	// 模拟修复过程
	time.Sleep(5 * time.Millisecond)

	// 更新修复状态
	s.mu.Lock()
	action.Status = "completed"
	action.Success = true
	now := time.Now()
	action.FinishedAt = &now

	// 更新数据块
	block.RepairCount++
	block.Checksum = result.Expected // 使用正确的校验和
	block.LastVerified = time.Now()

	result.Repaired = true
	result.RepairSource = "mirror"
	s.mu.Unlock()

	return true
}

// GetJob 获取清洗任务
func (s *ZFSScrubber) GetJob(id string) (*ScrubJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs 列出清洗任务
func (s *ZFSScrubber) ListJobs(poolID string) []*ScrubJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*ScrubJob, 0)
	for _, job := range s.jobs {
		if poolID == "" || job.PoolID == poolID {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// CancelJob 取消清洗任务
func (s *ZFSScrubber) CancelJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != ScrubRunning {
		return ErrJobNotRunning
	}

	job.Status = ScrubCancelled
	now := time.Now()
	job.FinishedAt = &now
	job.Duration = now.Sub(job.StartedAt)
	return nil
}

// RegisterBlock 注册数据块（用于测试）
func (s *ZFSScrubber) RegisterBlock(block *DataBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockStore[block.ID] = block
}

// GetReport 获取清洗报告
func (s *ZFSScrubber) GetReport(id string) (*ScrubReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report, exists := s.reports[id]
	if !exists {
		return nil, ErrReportNotFound
	}
	return report, nil
}

// ListReports 列出清洗报告
func (s *ZFSScrubber) ListReports(poolID string) []*ScrubReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports := make([]*ScrubReport, 0)
	for _, report := range s.reports {
		if poolID == "" || report.PoolID == poolID {
			reports = append(reports, report)
		}
	}
	return reports
}

// generateReport 生成清洗报告
func (s *ZFSScrubber) generateReport(job *ScrubJob) {
	report := &ScrubReport{
		ID:              fmt.Sprintf("report_%d", time.Now().UnixNano()),
		PoolID:          job.PoolID,
		JobID:           job.ID,
		StartedAt:       job.StartedAt,
		FinishedAt:      *job.FinishedAt,
		Duration:        job.Duration,
		BlocksScanned:   int64(len(s.getPoolBlocks(job.PoolID))),
		BytesScanned:    job.BytesScanned,
		ErrorsFound:     job.Errors,
		ErrorsRepaired:  job.Repaired,
		ErrorsUnrepairable: job.Errors - job.Repaired,
		Timestamp:       time.Now(),
	}

	// 生成建议
	report.Recommendations = s.generateRecommendations(report)

	s.mu.Lock()
	s.reports[report.ID] = report
	s.mu.Unlock()
}

// generateRecommendations 生成建议
func (s *ZFSScrubber) generateRecommendations(report *ScrubReport) []string {
	recommendations := make([]string, 0)

	if report.ErrorsFound > 0 {
		if report.ErrorsUnrepairable > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("发现 %d 个无法修复的错误，建议检查磁盘健康状态", report.ErrorsUnrepairable))
		}
		if report.ErrorsRepaired > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("成功修复 %d 个数据块错误", report.ErrorsRepaired))
		}
	}

	if report.Duration > 2*time.Hour {
		recommendations = append(recommendations, "清洗耗时较长，建议在低峰期执行")
	}

	return recommendations
}

// CheckPoolHealth 检查池健康状态
func (s *ZFSScrubber) CheckPoolHealth(poolID string) (*PoolHealth, error) {
	s.mu.RLock()
	health, exists := s.poolHealths[poolID]
	s.mu.RUnlock()

	if !exists {
		// 创建默认健康状态
		health = &PoolHealth{
			PoolID:        poolID,
			OverallHealth: HealthUnknown,
			Timestamp:     time.Now(),
		}
		s.mu.Lock()
		s.poolHealths[poolID] = health
		s.mu.Unlock()
	}

	return health, nil
}

// UpdatePoolHealth 更新池健康状态
func (s *ZFSScrubber) UpdatePoolHealth(health *PoolHealth) {
	s.mu.Lock()
	defer s.mu.Unlock()

	health.Timestamp = time.Now()
	s.poolHealths[health.PoolID] = health

	// 检查是否需要告警
	s.checkHealthAlerts(health)
}

// ListPoolHealths 列出所有池健康状态
func (s *ZFSScrubber) ListPoolHealths() []*PoolHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	healths := make([]*PoolHealth, 0, len(s.poolHealths))
	for _, h := range s.poolHealths {
		healths = append(healths, h)
	}
	return healths
}

// CheckDiskSMART 检查磁盘 SMART 状态
func (s *ZFSScrubber) CheckDiskSMART(devicePath string) (*DiskSMART, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 查找磁盘
	for _, health := range s.poolHealths {
		for _, disk := range health.Disks {
			if disk.DevicePath == devicePath {
				return disk, nil
			}
		}
	}

	return nil, ErrDiskNotFound
}

// checkHealthAlerts 检查健康告警
func (s *ZFSScrubber) checkHealthAlerts(health *PoolHealth) {
	// 检查设备健康
	for _, disk := range health.Disks {
		if disk.HealthStatus == HealthCritical {
			alert := &HealthAlert{
				ID:         fmt.Sprintf("alert_%d", time.Now().UnixNano()),
				PoolID:     health.PoolID,
				DevicePath: disk.DevicePath,
				Level:      HealthCritical,
				Message:    fmt.Sprintf("磁盘 %s SMART 状态异常，建议立即更换", disk.DevicePath),
				Timestamp:  time.Now(),
			}
			s.alerts[alert.ID] = alert
		}
	}

	// 检查池健康
	if health.OverallHealth == HealthCritical {
		alert := &HealthAlert{
			ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
			PoolID:    health.PoolID,
			Level:     HealthCritical,
			Message:   fmt.Sprintf("存储池 %s 状态异常，请立即检查", health.PoolName),
			Timestamp: time.Now(),
		}
		s.alerts[alert.ID] = alert
	}
}

// GetAlert 获取告警
func (s *ZFSScrubber) GetAlert(id string) (*HealthAlert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alert, exists := s.alerts[id]
	if !exists {
		return nil, ErrAlertNotFound
	}
	return alert, nil
}

// ListAlerts 列出告警
func (s *ZFSScrubber) ListAlerts(poolID string, unackedOnly bool) []*HealthAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alerts := make([]*HealthAlert, 0)
	for _, alert := range s.alerts {
		if poolID != "" && alert.PoolID != poolID {
			continue
		}
		if unackedOnly && alert.Acked {
			continue
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// AcknowledgeAlert 确认告警
func (s *ZFSScrubber) AcknowledgeAlert(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert, exists := s.alerts[id]
	if !exists {
		return ErrAlertNotFound
	}

	alert.Acked = true
	return nil
}

// GetConfig 获取配置
func (s *ZFSScrubber) GetConfig() *ScrubberConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig 更新配置
func (s *ZFSScrubber) UpdateConfig(config *ScrubberConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetRepairAction 获取修复动作
func (s *ZFSScrubber) GetRepairAction(id string) (*RepairAction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	action, exists := s.repairActions[id]
	if !exists {
		return nil, ErrRepairActionNotFound
	}
	return action, nil
}

// ListRepairActions 列出修复动作
func (s *ZFSScrubber) ListRepairActions(poolID string) []*RepairAction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	actions := make([]*RepairAction, 0)
	for _, action := range s.repairActions {
		if poolID == "" || action.PoolID == poolID {
			actions = append(actions, action)
		}
	}
	return actions
}
