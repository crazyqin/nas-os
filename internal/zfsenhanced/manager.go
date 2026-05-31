// Package zfsenhanced ZFS增强管理模块 - 业务逻辑层
package zfsenhanced

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
	"github.com/google/uuid"
)

type ScrubScheduleStrategy string
const (
	ScrubStrategyManual ScrubScheduleStrategy = "manual"
	ScrubStrategyTimed  ScrubScheduleStrategy = "timed"
	ScrubStrategySmart  ScrubScheduleStrategy = "smart"
)
type ScrubJobStatus string
const (
	ScrubStatusRunning   ScrubJobStatus = "running"
	ScrubStatusPaused    ScrubJobStatus = "paused"
	ScrubStatusCompleted ScrubJobStatus = "completed"
	ScrubStatusFailed    ScrubJobStatus = "failed"
)
type ScrubPriority int
const (
	ScrubPriorityLow    ScrubPriority = 1
	ScrubPriorityNormal ScrubPriority = 5
	ScrubPriorityHigh   ScrubPriority = 10
)
type RepairStatus string
const (
	RepairStatusPending   RepairStatus = "pending"
	RepairStatusRunning   RepairStatus = "running"
	RepairStatusCompleted RepairStatus = "completed"
	RepairStatusFailed    RepairStatus = "failed"
)
type ScrubSchedulePolicy struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	PoolName      string                `json:"pool_name"`
	Strategy      ScrubScheduleStrategy `json:"strategy"`
	IntervalDays  int                   `json:"interval_days"`
	PreferredHour int                   `json:"preferred_hour"`
	Priority      ScrubPriority         `json:"priority"`
	BandwidthMBps int                   `json:"bandwidth_mbps"`
	IOThreshold   int                   `json:"io_threshold"`
	AutoPause     bool                  `json:"auto_pause"`
	Enabled       bool                  `json:"enabled"`
	LastScrub     *time.Time            `json:"last_scrub,omitempty"`
	NextScrub     *time.Time            `json:"next_scrub,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}
type ScrubJob struct {
	ID        string         `json:"id"`
	PolicyID  string         `json:"policy_id"`
	PoolName  string         `json:"pool_name"`
	Status    ScrubJobStatus `json:"status"`
	Progress  float64        `json:"progress"`
	StartTime time.Time      `json:"start_time"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
	Errors    int64          `json:"errors"`
	Repaired  int64          `json:"repaired"`
	Bandwidth int64          `json:"bandwidth"`
	ErrorMsg  string         `json:"error_msg,omitempty"`
}
type SnapshotLifecyclePolicy struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	TemplateName    string     `json:"template_name,omitempty"`
	PoolName        string     `json:"pool_name"`
	Datasets        []string   `json:"datasets"`
	Schedule        string     `json:"schedule"`
	IntervalMinutes int        `json:"interval_minutes"`
	RetentionCount  int        `json:"retention_count"`
	RetentionDays   int        `json:"retention_days"`
	Prefix          string     `json:"prefix"`
	Recursive       bool       `json:"recursive"`
	Enabled         bool       `json:"enabled"`
	AutoCreate      bool       `json:"auto_create"`
	AutoDestroy     bool       `json:"auto_destroy"`
	LastRun         *time.Time `json:"last_run,omitempty"`
	NextRun         *time.Time `json:"next_run,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
type SnapshotTemplate struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	RetentionCount int    `json:"retention_count"`
	RetentionDays  int    `json:"retention_days"`
	Schedule       string `json:"schedule"`
	Recursive      bool   `json:"recursive"`
}
type AutoRepairTask struct {
	ID            string        `json:"id"`
	PoolName      string        `json:"pool_name"`
	ErrorType     string        `json:"error_type"`
	Severity      AlertSeverity `json:"severity"`
	Status        RepairStatus  `json:"status"`
	Priority      ScrubPriority `json:"priority"`
	ErrorCount    int64         `json:"error_count"`
	RepairedCount int64         `json:"repaired_count"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       *time.Time    `json:"end_time,omitempty"`
	Details       string        `json:"details,omitempty"`
	ErrorMsg      string        `json:"error_msg,omitempty"`
}
type RepairQueue struct { mu sync.Mutex; tasks []*AutoRepairTask }
func (rq *RepairQueue) Push(task *AutoRepairTask) {
	rq.mu.Lock(); defer rq.mu.Unlock()
	rq.tasks = append(rq.tasks, task)
	sort.Slice(rq.tasks, func(i, j int) bool { return rq.tasks[i].Priority > rq.tasks[j].Priority })
}
func (rq *RepairQueue) Pop() *AutoRepairTask {
	rq.mu.Lock(); defer rq.mu.Unlock()
	if len(rq.tasks) == 0 { return nil }
	task := rq.tasks[0]; rq.tasks = rq.tasks[1:]; return task
}
func (rq *RepairQueue) Len() int { rq.mu.Lock(); defer rq.mu.Unlock(); return len(rq.tasks) }
type PoolAnalysis struct {
	PoolName          string              `json:"pool_name"`
	Timestamp         time.Time           `json:"timestamp"`
	HealthScore       float64             `json:"health_score"`
	CapacityTrend     CapacityTrend       `json:"capacity_trend"`
	Fragmentation     float64             `json:"fragmentation"`
	PredictedFullDate *time.Time          `json:"predicted_full_date,omitempty"`
	DailyGrowthBytes  float64             `json:"daily_growth_bytes"`
	CompressionStats  *CompressionStats   `json:"compression_stats,omitempty"`
	DedupStats        *DedupStats         `json:"dedup_stats,omitempty"`
	DiskHealth        []DiskHealthSummary `json:"disk_health"`
	Recommendations   []string            `json:"recommendations"`
}
type DiskHealthSummary struct {
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Temperature     int     `json:"temperature"`
	PowerOnHours    int64   `json:"power_on_hours"`
	Reallocated     int64   `json:"reallocated_sectors"`
	HealthPercent   float64 `json:"health_percent"`
}
type RealtimeMetrics struct {
	PoolName      string    `json:"pool_name"`
	Timestamp     time.Time `json:"timestamp"`
	ReadIOPS      int64     `json:"read_iops"`
	WriteIOPS     int64     `json:"write_iops"`
	ReadMBps      float64   `json:"read_mbps"`
	WriteMBps     float64   `json:"write_mbps"`
	ReadLatency   float64   `json:"read_latency_ms"`
	WriteLatency  float64   `json:"write_latency_ms"`
	HealthScore   float64   `json:"health_score"`
	UsedPercent   float64   `json:"used_percent"`
	Fragmentation float64   `json:"fragmentation"`
}
type Manager struct {
	mu sync.RWMutex; poolMgr *PoolManager; integrityChecker *IntegrityChecker; perfMonitor *PerformanceMonitor
	scrubPolicies map[string]*ScrubSchedulePolicy; scrubJobs map[string]*ScrubJob
	snapshotPolicies map[string]*SnapshotLifecyclePolicy; snapshotTemplates map[string]*SnapshotTemplate
	repairQueue *RepairQueue; repairTasks map[string]*AutoRepairTask
	realtimeHistory map[string][]RealtimeMetrics; poolAnalyses map[string]*PoolAnalysis
}
func NewManager(alertConfig AlertConfig) *Manager {
	poolMgr := NewPoolManager(alertConfig)
	m := &Manager{poolMgr: poolMgr, integrityChecker: NewIntegrityChecker(poolMgr, DefaultIntegrityConfig()), perfMonitor: NewPerformanceMonitor(poolMgr),
		scrubPolicies: make(map[string]*ScrubSchedulePolicy), scrubJobs: make(map[string]*ScrubJob),
		snapshotPolicies: make(map[string]*SnapshotLifecyclePolicy), snapshotTemplates: make(map[string]*SnapshotTemplate),
		repairQueue: &RepairQueue{tasks: make([]*AutoRepairTask, 0)}, repairTasks: make(map[string]*AutoRepairTask),
		realtimeHistory: make(map[string][]RealtimeMetrics), poolAnalyses: make(map[string]*PoolAnalysis)}
	m.registerDefaultTemplates()
	return m
}
func (m *Manager) registerDefaultTemplates() {
	m.snapshotTemplates["hourly"] = &SnapshotTemplate{Name: "hourly", Description: "每小时快照保留48份", RetentionCount: 48, RetentionDays: 2, Schedule: "0 * * * *", Recursive: true}
	m.snapshotTemplates["daily"] = &SnapshotTemplate{Name: "daily", Description: "每日快照保留30份", RetentionCount: 30, RetentionDays: 30, Schedule: "0 0 * * *", Recursive: true}
	m.snapshotTemplates["weekly"] = &SnapshotTemplate{Name: "weekly", Description: "每周快照保留12份", RetentionCount: 12, RetentionDays: 84, Schedule: "0 0 * * 0", Recursive: false}
}

// ========== Scrub调度 ==========
func (m *Manager) CreateScrubPolicy(policy *ScrubSchedulePolicy) (*ScrubSchedulePolicy, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	if policy.PoolName == "" { return nil, fmt.Errorf("池名称不能为空") }
	if policy.IntervalDays <= 0 { policy.IntervalDays = 14 }
	if policy.Priority == 0 { policy.Priority = ScrubPriorityNormal }
	policy.ID = uuid.New().String(); now := time.Now(); policy.CreatedAt = now; policy.UpdatedAt = now
	if policy.Strategy == ScrubStrategyTimed && policy.IntervalDays > 0 {
		next := now.AddDate(0, 0, policy.IntervalDays)
		if policy.PreferredHour > 0 { next = time.Date(next.Year(), next.Month(), next.Day(), policy.PreferredHour, 0, 0, 0, next.Location()) }
		policy.NextScrub = &next
	}
	m.scrubPolicies[policy.ID] = policy; return policy, nil
}
func (m *Manager) UpdateScrubPolicy(id string, update *ScrubSchedulePolicy) (*ScrubSchedulePolicy, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	policy, ok := m.scrubPolicies[id]; if !ok { return nil, fmt.Errorf("策略 %s 不存在", id) }
	if update.Name != "" { policy.Name = update.Name }; if update.Strategy != "" { policy.Strategy = update.Strategy }
	if update.IntervalDays > 0 { policy.IntervalDays = update.IntervalDays }; if update.Priority > 0 { policy.Priority = update.Priority }
	policy.BandwidthMBps = update.BandwidthMBps; policy.IOThreshold = update.IOThreshold
	policy.AutoPause = update.AutoPause; policy.Enabled = update.Enabled; policy.UpdatedAt = time.Now()
	return policy, nil
}
func (m *Manager) DeleteScrubPolicy(id string) error {
	m.mu.Lock(); defer m.mu.Unlock()
	if _, ok := m.scrubPolicies[id]; !ok { return fmt.Errorf("策略 %s 不存在", id) }
	delete(m.scrubPolicies, id); return nil
}
func (m *Manager) ListScrubPolicies() []*ScrubSchedulePolicy {
	m.mu.RLock(); defer m.mu.RUnlock()
	policies := make([]*ScrubSchedulePolicy, 0, len(m.scrubPolicies))
	for _, p := range m.scrubPolicies { cp := *p; policies = append(policies, &cp) }; return policies
}
func (m *Manager) GetScrubPolicy(id string) (*ScrubSchedulePolicy, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	p, ok := m.scrubPolicies[id]; if !ok { return nil, fmt.Errorf("策略 %s 不存在", id) }; cp := *p; return &cp, nil
}
func (m *Manager) RunScrub(ctx context.Context, poolName string, priority ScrubPriority) (*ScrubJob, error) {
	m.mu.RLock(); _, exists := m.poolMgr.pools[poolName]; m.mu.RUnlock()
	if !exists { if _, err := m.poolMgr.GetPoolStatus(ctx, poolName); err != nil { return nil, fmt.Errorf("池 %s 不存在", poolName) } }
	job := &ScrubJob{ID: uuid.New().String(), PoolName: poolName, Status: ScrubStatusRunning, StartTime: time.Now()}
	m.mu.Lock(); m.scrubJobs[job.ID] = job; m.mu.Unlock()
	result, err := m.integrityChecker.RunScrub(ctx, poolName)
	m.mu.Lock(); now := time.Now(); job.EndTime = &now
	if err != nil { job.Status = ScrubStatusFailed; job.ErrorMsg = err.Error() } else {
		job.Status = ScrubStatusCompleted; job.Errors = result.ErrorsFound; job.Repaired = result.RepairsMade; job.Progress = 100 }
	m.mu.Unlock(); return job, nil
}
func (m *Manager) CancelScrub(ctx context.Context, poolName string) error { return m.poolMgr.CancelScrub(ctx, poolName) }
func (m *Manager) GetScrubJobs(poolName string) []*ScrubJob {
	m.mu.RLock(); defer m.mu.RUnlock()
	jobs := make([]*ScrubJob, 0)
	for _, j := range m.scrubJobs { if poolName == "" || j.PoolName == poolName { cp := *j; jobs = append(jobs, &cp) } }
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].StartTime.After(jobs[j].StartTime) }); return jobs
}
func (m *Manager) CheckAndRunScheduledScrubs(ctx context.Context) ([]string, error) {
	m.mu.RLock(); var toRun []*ScrubSchedulePolicy
	for _, p := range m.scrubPolicies { if !p.Enabled || p.Strategy == ScrubStrategyManual { continue }; if p.NextScrub != nil && time.Now().After(*p.NextScrub) { cp := *p; toRun = append(toRun, &cp) } }
	m.mu.RUnlock()
	var triggered []string
	for _, policy := range toRun {
		if policy.Strategy == ScrubStrategySmart && policy.IOThreshold > 0 {
			metrics, err := m.perfMonitor.CollectMetrics(ctx, policy.PoolName)
			if err == nil && (metrics.ReadIOPS+metrics.WriteIOPS) > int64(policy.IOThreshold) { continue }
		}
		job, err := m.RunScrub(ctx, policy.PoolName, policy.Priority)
		if err == nil { triggered = append(triggered, job.ID); m.mu.Lock(); if p, ok := m.scrubPolicies[policy.ID]; ok { now := time.Now(); p.LastScrub = &now; next := now.AddDate(0, 0, p.IntervalDays); p.NextScrub = &next }; m.mu.Unlock() }
	}; return triggered, nil
}

// ========== 快照生命周期 ==========
func (m *Manager) CreateSnapshotLifecycle(policy *SnapshotLifecyclePolicy) (*SnapshotLifecyclePolicy, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	if policy.PoolName == "" { return nil, fmt.Errorf("池名称不能为空") }; if len(policy.Datasets) == 0 { return nil, fmt.Errorf("数据集列表不能为空") }
	if policy.TemplateName != "" { if tmpl, ok := m.snapshotTemplates[policy.TemplateName]; ok { if policy.RetentionCount == 0 { policy.RetentionCount = tmpl.RetentionCount }; if policy.RetentionDays == 0 { policy.RetentionDays = tmpl.RetentionDays }; if policy.Schedule == "" { policy.Schedule = tmpl.Schedule }; policy.Recursive = tmpl.Recursive } }
	if policy.RetentionCount <= 0 { policy.RetentionCount = 10 }; if policy.IntervalMinutes <= 0 { policy.IntervalMinutes = 60 }; if policy.Prefix == "" { policy.Prefix = "auto-" }
	policy.ID = uuid.New().String(); now := time.Now(); policy.CreatedAt = now; policy.UpdatedAt = now
	next := now.Add(time.Duration(policy.IntervalMinutes) * time.Minute); policy.NextRun = &next
	m.snapshotPolicies[policy.ID] = policy; return policy, nil
}
func (m *Manager) UpdateSnapshotLifecycle(id string, update *SnapshotLifecyclePolicy) (*SnapshotLifecyclePolicy, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	policy, ok := m.snapshotPolicies[id]; if !ok { return nil, fmt.Errorf("策略 %s 不存在", id) }
	if update.Name != "" { policy.Name = update.Name }; if len(update.Datasets) > 0 { policy.Datasets = update.Datasets }
	if update.IntervalMinutes > 0 { policy.IntervalMinutes = update.IntervalMinutes }; if update.RetentionCount > 0 { policy.RetentionCount = update.RetentionCount }
	if update.RetentionDays > 0 { policy.RetentionDays = update.RetentionDays }; if update.Prefix != "" { policy.Prefix = update.Prefix }
	policy.Recursive = update.Recursive; policy.Enabled = update.Enabled; policy.AutoCreate = update.AutoCreate; policy.AutoDestroy = update.AutoDestroy; policy.UpdatedAt = time.Now()
	return policy, nil
}
func (m *Manager) DeleteSnapshotLifecycle(id string) error {
	m.mu.Lock(); defer m.mu.Unlock()
	if _, ok := m.snapshotPolicies[id]; !ok { return fmt.Errorf("策略 %s 不存在", id) }; delete(m.snapshotPolicies, id); return nil
}
func (m *Manager) ListSnapshotLifecycles() []*SnapshotLifecyclePolicy {
	m.mu.RLock(); defer m.mu.RUnlock()
	policies := make([]*SnapshotLifecyclePolicy, 0, len(m.snapshotPolicies))
	for _, p := range m.snapshotPolicies { cp := *p; policies = append(policies, &cp) }; return policies
}
func (m *Manager) GetSnapshotLifecycle(id string) (*SnapshotLifecyclePolicy, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	p, ok := m.snapshotPolicies[id]; if !ok { return nil, fmt.Errorf("策略 %s 不存在", id) }; cp := *p; return &cp, nil
}
func (m *Manager) ExecuteSnapshotLifecycle(ctx context.Context, policyID string) error {
	m.mu.RLock(); policy, ok := m.snapshotPolicies[policyID]
	if !ok { m.mu.RUnlock(); return fmt.Errorf("策略 %s 不存在", policyID) }; if !policy.Enabled { m.mu.RUnlock(); return fmt.Errorf("策略 %s 已禁用", policyID) }
	policyCopy := *policy; m.mu.RUnlock()
	var lastErr error; snapshotName := fmt.Sprintf("%s%s", policyCopy.Prefix, time.Now().Format("20060102-150405"))
	for _, dataset := range policyCopy.Datasets { if err := m.poolMgr.CreateSnapshot(ctx, dataset, snapshotName, policyCopy.Recursive); err != nil { lastErr = err } }
	if policyCopy.AutoDestroy { for _, dataset := range policyCopy.Datasets { m.cleanupDatasetSnapshots(ctx, policyCopy.PoolName, dataset, policyCopy.Prefix, policyCopy.RetentionCount, policyCopy.RetentionDays) } }
	m.mu.Lock(); if p, exists := m.snapshotPolicies[policyID]; exists { now := time.Now(); p.LastRun = &now; next := now.Add(time.Duration(p.IntervalMinutes) * time.Minute); p.NextRun = &next }; m.mu.Unlock()
	return lastErr
}
func (m *Manager) CheckAndRunSnapshotLifecycles(ctx context.Context) ([]string, error) {
	m.mu.RLock(); var toRun []string
	for id, p := range m.snapshotPolicies { if !p.Enabled || !p.AutoCreate { continue }; if p.NextRun != nil && time.Now().After(*p.NextRun) { toRun = append(toRun, id) } }
	m.mu.RUnlock(); var triggered []string
	for _, id := range toRun { if err := m.ExecuteSnapshotLifecycle(ctx, id); err == nil { triggered = append(triggered, id) } }; return triggered, nil
}
func (m *Manager) ListSnapshotTemplates() []SnapshotTemplate {
	m.mu.RLock(); defer m.mu.RUnlock()
	templates := make([]SnapshotTemplate, 0, len(m.snapshotTemplates))
	for _, t := range m.snapshotTemplates { templates = append(templates, *t) }; return templates
}

// ========== 自动修复 ==========
func (m *Manager) DetectAndRepair(ctx context.Context, poolName string) ([]*AutoRepairTask, error) {
	var tasks []*AutoRepairTask
	checkResult, err := m.integrityChecker.CheckChecksums(ctx, poolName)
	if err == nil && checkResult.ErrorsFound > 0 {
		task := &AutoRepairTask{ID: uuid.New().String(), PoolName: poolName, ErrorType: "checksum", Severity: AlertSeverityWarning, Status: RepairStatusPending, Priority: ScrubPriorityHigh, ErrorCount: checkResult.ErrorsFound, StartTime: time.Now()}
		m.mu.Lock(); m.repairTasks[task.ID] = task; m.mu.Unlock(); m.repairQueue.Push(task); tasks = append(tasks, task)
	}
	errorDist, err := m.integrityChecker.GetErrorDistribution(ctx, poolName)
	if err == nil && len(errorDist) > 0 {
		var totalIOErrors int64; for _, v := range errorDist { totalIOErrors += v }
		if totalIOErrors > 0 {
			task := &AutoRepairTask{ID: uuid.New().String(), PoolName: poolName, ErrorType: "io", Severity: AlertSeverityCritical, Status: RepairStatusPending, Priority: ScrubPriorityHigh, ErrorCount: totalIOErrors, StartTime: time.Now()}
			m.mu.Lock(); m.repairTasks[task.ID] = task; m.mu.Unlock(); m.repairQueue.Push(task); tasks = append(tasks, task)
		}
	}
	if len(tasks) > 0 { go m.processRepairQueue(context.Background()) }; return tasks, nil
}
func (m *Manager) ProcessRepairQueue(ctx context.Context) { m.processRepairQueue(ctx) }
func (m *Manager) GetRepairTasks(poolName string) []*AutoRepairTask {
	m.mu.RLock(); defer m.mu.RUnlock()
	tasks := make([]*AutoRepairTask, 0)
	for _, t := range m.repairTasks { if poolName == "" || t.PoolName == poolName { cp := *t; tasks = append(tasks, &cp) } }
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].StartTime.After(tasks[j].StartTime) }); return tasks
}

// ========== 性能监控 ==========
func (m *Manager) GetRealtimeMetrics(ctx context.Context, poolName string) (*RealtimeMetrics, error) {
	pool, err := m.poolMgr.GetPoolStatus(ctx, poolName); if err != nil { return nil, err }
	perfMetrics, err := m.perfMonitor.CollectMetrics(ctx, poolName); if err != nil { return nil, err }
	metrics := &RealtimeMetrics{PoolName: poolName, Timestamp: time.Now(), ReadIOPS: perfMetrics.ReadIOPS, WriteIOPS: perfMetrics.WriteIOPS,
		ReadMBps: float64(perfMetrics.ReadThroughput) / (1024 * 1024), WriteMBps: float64(perfMetrics.WriteThroughput) / (1024 * 1024),
		ReadLatency: perfMetrics.ReadLatencyMs, WriteLatency: perfMetrics.WriteLatencyMs, HealthScore: m.calculateHealthScore(pool), UsedPercent: pool.UsedPercent, Fragmentation: pool.Fragmentation}
	m.mu.Lock(); m.realtimeHistory[poolName] = append(m.realtimeHistory[poolName], *metrics)
	if len(m.realtimeHistory[poolName]) > 1000 { m.realtimeHistory[poolName] = m.realtimeHistory[poolName][len(m.realtimeHistory[poolName])-1000:] }; m.mu.Unlock()
	return metrics, nil
}
func (m *Manager) GetMetricsHistory(poolName string, limit int) []RealtimeMetrics {
	m.mu.RLock(); defer m.mu.RUnlock()
	history, exists := m.realtimeHistory[poolName]; if !exists { return nil }
	if limit <= 0 || limit > len(history) { limit = len(history) }; start := len(history) - limit
	result := make([]RealtimeMetrics, limit); copy(result, history[start:]); return result
}
func (m *Manager) GetPerformanceRecommendations(ctx context.Context, poolName string) ([]PerformanceTuningRecommendation, error) {
	return m.perfMonitor.GenerateTuningRecommendations(ctx, poolName)
}

// ========== 存储池分析 ==========
func (m *Manager) AnalyzePool(ctx context.Context, poolName string) (*PoolAnalysis, error) {
	pool, err := m.poolMgr.GetPoolStatus(ctx, poolName); if err != nil { return nil, err }
	analysis := &PoolAnalysis{PoolName: poolName, Timestamp: time.Now(), Fragmentation: pool.Fragmentation, DiskHealth: make([]DiskHealthSummary, 0), Recommendations: make([]string, 0)}
	analysis.HealthScore = m.calculateHealthScore(pool)
	capTrend, err := m.perfMonitor.GetCapacityTrend(ctx, poolName)
	if err == nil { analysis.CapacityTrend = *capTrend; analysis.DailyGrowthBytes = capTrend.GrowthRateDay; if capTrend.DaysUntilFull > 0 { fullDate := time.Now().AddDate(0, 0, capTrend.DaysUntilFull); analysis.PredictedFullDate = &fullDate } }
	compStats, err := m.perfMonitor.GetCompressionStats(ctx, poolName, poolName); if err == nil { analysis.CompressionStats = compStats }
	dedupStats, err := m.perfMonitor.GetDedupStats(ctx, poolName); if err == nil { analysis.DedupStats = dedupStats }
	for _, disk := range pool.Disks {
		summary := DiskHealthSummary{Name: disk.Name, Status: string(disk.Status)}
		if disk.SMART != nil { summary.Temperature = disk.SMART.Temperature; summary.PowerOnHours = disk.SMART.PowerOnHours; summary.Reallocated = disk.SMART.ReallocatedSectors; summary.HealthPercent = m.calculateDiskHealthPercent(disk.SMART) }
		analysis.DiskHealth = append(analysis.DiskHealth, summary)
	}
	analysis.Recommendations = m.generatePoolRecommendations(pool, analysis)
	m.mu.Lock(); m.poolAnalyses[poolName] = analysis; m.mu.Unlock(); return analysis, nil
}
func (m *Manager) GetPoolAnalysis(poolName string) (*PoolAnalysis, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	analysis, ok := m.poolAnalyses[poolName]; if !ok { return nil, fmt.Errorf("池 %s 没有分析数据", poolName) }; return analysis, nil
}
func (m *Manager) GetCapacityTrend(ctx context.Context, poolName string) (*CapacityTrend, error) { return m.perfMonitor.GetCapacityTrend(ctx, poolName) }
func (m *Manager) GetFragmentation(ctx context.Context, poolName string) (float64, error) {
	pool, err := m.poolMgr.GetPoolStatus(ctx, poolName); if err != nil { return 0, err }; return pool.Fragmentation, nil
}
func (m *Manager) GetPoolManager() *PoolManager { return m.poolMgr }
func (m *Manager) ListPools(ctx context.Context) ([]PoolInfo, error) { return m.poolMgr.ListPools(ctx) }
func (m *Manager) GetPoolStatus(ctx context.Context, name string) (*PoolInfo, error) { return m.poolMgr.GetPoolStatus(ctx, name) }
func (m *Manager) GetAlerts() []Alert { return m.poolMgr.GetAlerts() }
func (m *Manager) AcknowledgeAlert(alertID, ackedBy string) error { return m.poolMgr.AcknowledgeAlert(alertID, ackedBy) }
func (m *Manager) ResolveAlert(alertID string) error { return m.poolMgr.ResolveAlert(alertID) }
func (m *Manager) GetAlertConfig() AlertConfig { return m.poolMgr.GetAlertConfig() }
func (m *Manager) UpdateAlertConfig(config AlertConfig) { m.poolMgr.UpdateAlertConfig(config) }

func (m *Manager) cleanupDatasetSnapshots(ctx context.Context, poolName, dataset, prefix string, maxCount, maxDays int) {
	snapshots, err := m.poolMgr.ListSnapshots(ctx, poolName); if err != nil { return }
	var matched []SnapshotInfo
	for _, snap := range snapshots { if snap.Dataset == dataset && len(snap.SnapshotName) > len(prefix) && snap.SnapshotName[:len(prefix)] == prefix { matched = append(matched, snap) } }
	sort.Slice(matched, func(i, j int) bool { return matched[i].CreatedAt.After(matched[j].CreatedAt) })
	if maxCount > 0 && len(matched) > maxCount { for _, snap := range matched[maxCount:] { _ = m.poolMgr.DeleteSnapshot(ctx, dataset, snap.SnapshotName, false) } }
	if maxDays > 0 { cutoff := time.Now().AddDate(0, 0, -maxDays); for _, snap := range matched { if snap.CreatedAt.Before(cutoff) { _ = m.poolMgr.DeleteSnapshot(ctx, dataset, snap.SnapshotName, false) } } }
}
func (m *Manager) processRepairQueue(ctx context.Context) {
	for m.repairQueue.Len() > 0 { task := m.repairQueue.Pop(); if task == nil { break }
		m.mu.Lock(); task.Status = RepairStatusRunning; m.mu.Unlock()
		result, err := m.integrityChecker.RunScrub(ctx, task.PoolName)
		m.mu.Lock(); now := time.Now(); task.EndTime = &now
		if err != nil { task.Status = RepairStatusFailed; task.ErrorMsg = err.Error() } else { task.Status = RepairStatusCompleted; task.RepairedCount = result.RepairsMade }; m.mu.Unlock()
	}
}
func (m *Manager) calculateHealthScore(pool *PoolInfo) float64 {
	score := 100.0
	switch pool.Status { case PoolStatusDegraded: score -= 30; case PoolStatusFaulted: score -= 60; case PoolStatusOffline, PoolStatusUnavail: score -= 80 }
	if pool.UsedPercent > 90 { score -= 20 } else if pool.UsedPercent > 80 { score -= 10 }
	totalErrors := pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors
	if totalErrors > 100 { score -= 20 } else if totalErrors > 10 { score -= 10 }
	if pool.Fragmentation > 50 { score -= 10 } else if pool.Fragmentation > 30 { score -= 5 }
	if score < 0 { score = 0 }; return score
}
func (m *Manager) calculateDiskHealthPercent(smart *SMARTInfo) float64 {
	health := 100.0
	if smart.ReallocatedSectors > 0 { health -= float64(smart.ReallocatedSectors) * 2 }
	if smart.PendingSectors > 0 { health -= float64(smart.PendingSectors) * 3 }
	if smart.Temperature > 50 { health -= float64(smart.Temperature-50) * 1.5 }
	if health < 0 { health = 0 }; return health
}
func (m *Manager) generatePoolRecommendations(pool *PoolInfo, analysis *PoolAnalysis) []string {
	var recs []string
	if pool.UsedPercent > 85 { recs = append(recs, fmt.Sprintf("池 %s 使用率 %.1f%%，建议扩容或清理数据", pool.Name, pool.UsedPercent)) }
	if pool.Fragmentation > 30 { recs = append(recs, fmt.Sprintf("池 %s 碎片率 %.1f%%，建议执行 scrub 或清理旧快照", pool.Name, pool.Fragmentation)) }
	if pool.Status != PoolStatusOnline { recs = append(recs, fmt.Sprintf("池 %s 状态 %s，建议检查磁盘健康并替换故障盘", pool.Name, pool.Status)) }
	totalErrors := pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors
	if totalErrors > 0 { recs = append(recs, fmt.Sprintf("池 %s 存在 %d 个错误，建议执行 scrub 检查", pool.Name, totalErrors)) }
	if analysis.DailyGrowthBytes > 0 && analysis.PredictedFullDate != nil { daysLeft := int(time.Until(*analysis.PredictedFullDate).Hours() / 24); if daysLeft < 30 { recs = append(recs, fmt.Sprintf("按当前增速，池 %s 预计 %d 天后满，建议尽快扩容", pool.Name, daysLeft)) } }
	if len(recs) == 0 { recs = append(recs, "池运行状况良好，无需特别操作") }; return recs
}
