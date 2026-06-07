// Package smartrebalance 智能存储再平衡引擎
// 对标群晖存储管理器的自动再平衡、TrueNAS 的 ZFS 自动均衡
// 根据磁盘利用率、I/O 模式、数据热度自动迁移数据，优化存储池性能
package smartrebalance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PoolStatus 存储池状态
type PoolStatus string

const (
	PoolStatusHealthy     PoolStatus = "healthy"
	PoolStatusImbalanced  PoolStatus = "imbalanced"
	PoolStatusRebalancing PoolStatus = "rebalancing"
	PoolStatusDegraded    PoolStatus = "degraded"
	PoolStatusError       PoolStatus = "error"
)

// DiskTier 磁盘层级
type DiskTier string

const (
	TierSSD  DiskTier = "ssd"
	TierHDD  DiskTier = "hdd"
	TierNVMe DiskTier = "nvme"
	TierSMR  DiskTier = "smr"
)

// RebalanceStrategy 再平衡策略
type RebalanceStrategy string

const (
	StrategyCapacity    RebalanceStrategy = "capacity"    // 按容量均衡
	StrategyPerformance RebalanceStrategy = "performance" // 按性能均衡
	StrategyHeatAware   RebalanceStrategy = "heat_aware"  // 按数据热度均衡
	StrategyHybrid      RebalanceStrategy = "hybrid"      // 混合策略
)

// StoragePool 存储池信息
type StoragePool struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      PoolStatus        `json:"status"`
	Disks       []DiskInfo        `json:"disks"`
	TotalBytes  uint64            `json:"total_bytes"`
	UsedBytes   uint64            `json:"used_bytes"`
	FreeBytes   uint64            `json:"free_bytes"`
	Utilization float64           `json:"utilization"` // 0.0-1.0
	Imbalance   float64           `json:"imbalance"`   // 不均衡度 0.0-1.0
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Model       string   `json:"model"`
	Tier        DiskTier `json:"tier"`
	TotalBytes  uint64   `json:"total_bytes"`
	UsedBytes   uint64   `json:"used_bytes"`
	Utilization float64  `json:"utilization"`
	Temperature int      `json:"temperature"`  // 摄氏度
	HealthScore float64  `json:"health_score"` // 0.0-1.0
	ReadIOPS    float64  `json:"read_iops"`
	WriteIOPS   float64  `json:"write_iops"`
}

// RebalanceJob 再平衡任务
type RebalanceJob struct {
	ID          string            `json:"id"`
	PoolID      string            `json:"pool_id"`
	Strategy    RebalanceStrategy `json:"strategy"`
	Status      JobStatus         `json:"status"`
	Progress    float64           `json:"progress"` // 0.0-1.0
	SourceDisk  string            `json:"source_disk"`
	TargetDisk  string            `json:"target_disk"`
	BytesMoved  uint64            `json:"bytes_moved"`
	TotalBytes  uint64            `json:"total_bytes"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// RebalancePolicy 再平衡策略配置
type RebalancePolicy struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Enabled      bool              `json:"enabled"`
	Strategy     RebalanceStrategy `json:"strategy"`
	Threshold    float64           `json:"threshold"`      // 触发阈值 0.0-1.0
	Schedule     string            `json:"schedule"`       // cron 表达式
	MaxBandwidth int64             `json:"max_bandwidth"`  // 最大带宽 bytes/s
	MinFreeSpace uint64            `json:"min_free_space"` // 最小剩余空间
	ExcludePools []string          `json:"exclude_pools"`
	ExcludeDisks []string          `json:"exclude_disks"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// RebalanceMetrics 再平衡指标
type RebalanceMetrics struct {
	TotalRebalances  int64         `json:"total_rebalances"`
	TotalBytesMoved  uint64        `json:"total_bytes_moved"`
	AvgDuration      time.Duration `json:"avg_duration"`
	LastRebalance    *time.Time    `json:"last_rebalance,omitempty"`
	CurrentImbalance float64       `json:"current_imbalance"`
	TargetImbalance  float64       `json:"target_imbalance"`
	PoolsMonitored   int           `json:"pools_monitored"`
	DisksMonitored   int           `json:"disks_monitored"`
}

// Manager 智能再平衡管理器
type Manager struct {
	mu         sync.RWMutex
	pools      map[string]*StoragePool
	jobs       map[string]*RebalanceJob
	policies   map[string]*RebalancePolicy
	metrics    *RebalanceMetrics
	cancelFunc context.CancelFunc
}

// NewManager 创建再平衡管理器
func NewManager() *Manager {
	return &Manager{
		pools:    make(map[string]*StoragePool),
		jobs:     make(map[string]*RebalanceJob),
		policies: make(map[string]*RebalancePolicy),
		metrics:  &RebalanceMetrics{},
	}
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	go m.monitorLoop(ctx)
	go m.rebalanceLoop(ctx)

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// RegisterPool 注册存储池
func (m *Manager) RegisterPool(pool *StoragePool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pools[pool.ID] = pool
	m.metrics.PoolsMonitored = len(m.pools)
}

// GetPool 获取存储池
func (m *Manager) GetPool(id string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool, ok := m.pools[id]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", id)
	}
	return pool, nil
}

// ListPools 列出所有存储池
func (m *Manager) ListPools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pools := make([]*StoragePool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools
}

// CreatePolicy 创建再平衡策略
func (m *Manager) CreatePolicy(policy *RebalancePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取策略
func (m *Manager) GetPolicy(id string) (*RebalancePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return policy, nil
}

// TriggerRebalance 手动触发再平衡
func (m *Manager) TriggerRebalance(poolID string, strategy RebalanceStrategy) (*RebalanceJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	// 计算不均衡度
	imbalance := calculateImbalance(pool)
	pool.Imbalance = imbalance

	if imbalance < 0.1 {
		return nil, fmt.Errorf("pool %s is already balanced (imbalance: %.2f%%)", poolID, imbalance*100)
	}

	// 找到源盘和目标盘
	source, target := findRebalanceTargets(pool, strategy)
	if source == nil || target == nil {
		return nil, fmt.Errorf("no suitable rebalance targets found")
	}

	job := &RebalanceJob{
		ID:         fmt.Sprintf("rebalance-%s-%d", poolID, time.Now().UnixNano()),
		PoolID:     poolID,
		Strategy:   strategy,
		Status:     JobStatusPending,
		SourceDisk: source.ID,
		TargetDisk: target.ID,
		StartedAt:  timePtr(time.Now()),
	}

	m.jobs[job.ID] = job
	pool.Status = PoolStatusRebalancing

	go m.executeRebalance(job, pool, source, target)

	return job, nil
}

// GetJob 获取任务
func (m *Manager) GetJob(id string) (*RebalanceJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %s not found", id)
	}
	return job, nil
}

// ListJobs 列出所有任务
func (m *Manager) ListJobs() []*RebalanceJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*RebalanceJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// GetMetrics 获取指标
func (m *Manager) GetMetrics() *RebalanceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// AnalyzePool 分析存储池均衡状态
func (m *Manager) AnalyzePool(poolID string) (*RebalanceAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	analysis := &RebalanceAnalysis{
		PoolID:      poolID,
		PoolName:    pool.Name,
		AnalyzedAt:  time.Now(),
		Imbalance:   calculateImbalance(pool),
		DiskDetails: make([]DiskAnalysis, 0, len(pool.Disks)),
	}

	avgUtil := pool.Utilization
	for _, disk := range pool.Disks {
		da := DiskAnalysis{
			DiskID:      disk.ID,
			Model:       disk.Model,
			Tier:        disk.Tier,
			Utilization: disk.Utilization,
			Deviation:   disk.Utilization - avgUtil,
			HealthScore: disk.HealthScore,
		}
		if da.Deviation > 0.15 {
			da.Recommendation = "建议迁出数据"
		} else if da.Deviation < -0.15 {
			da.Recommendation = "可接收迁移数据"
		} else {
			da.Recommendation = "均衡状态"
		}
		analysis.DiskDetails = append(analysis.DiskDetails, da)
	}

	if analysis.Imbalance > 0.3 {
		analysis.Recommendation = "严重不均衡，建议立即再平衡"
		analysis.SuggestedStrategy = StrategyHeatAware
	} else if analysis.Imbalance > 0.15 {
		analysis.Recommendation = "轻度不均衡，建议计划再平衡"
		analysis.SuggestedStrategy = StrategyCapacity
	} else {
		analysis.Recommendation = "均衡状态，无需操作"
	}

	return analysis, nil
}

// RebalanceAnalysis 再平衡分析结果
type RebalanceAnalysis struct {
	PoolID            string            `json:"pool_id"`
	PoolName          string            `json:"pool_name"`
	AnalyzedAt        time.Time         `json:"analyzed_at"`
	Imbalance         float64           `json:"imbalance"`
	Recommendation    string            `json:"recommendation"`
	SuggestedStrategy RebalanceStrategy `json:"suggested_strategy,omitempty"`
	DiskDetails       []DiskAnalysis    `json:"disk_details"`
}

// DiskAnalysis 磁盘分析
type DiskAnalysis struct {
	DiskID         string   `json:"disk_id"`
	Model          string   `json:"model"`
	Tier           DiskTier `json:"tier"`
	Utilization    float64  `json:"utilization"`
	Deviation      float64  `json:"deviation"`
	HealthScore    float64  `json:"health_score"`
	Recommendation string   `json:"recommendation"`
}

// calculateImbalance 计算存储池不均衡度
func calculateImbalance(pool *StoragePool) float64 {
	if len(pool.Disks) <= 1 {
		return 0
	}

	var totalUtil float64
	for _, d := range pool.Disks {
		totalUtil += d.Utilization
	}
	avgUtil := totalUtil / float64(len(pool.Disks))

	var variance float64
	for _, d := range pool.Disks {
		diff := d.Utilization - avgUtil
		variance += diff * diff
	}
	variance /= float64(len(pool.Disks))

	// 标准差作为不均衡度
	return sqrt(variance)
}

// findRebalanceTargets 找到再平衡的源盘和目标盘
func findRebalanceTargets(pool *StoragePool, strategy RebalanceStrategy) (*DiskInfo, *DiskInfo) {
	if len(pool.Disks) < 2 {
		return nil, nil
	}

	var source, target *DiskInfo
	maxUtil := -1.0
	minUtil := 2.0

	for i := range pool.Disks {
		d := &pool.Disks[i]
		util := d.Utilization

		switch strategy {
		case StrategyPerformance:
			util = (d.ReadIOPS + d.WriteIOPS) / 10000 // 归一化
		case StrategyHeatAware:
			util = d.Utilization * (1 + d.HealthScore)
		}

		if util > maxUtil {
			maxUtil = util
			source = d
		}
		if util < minUtil {
			minUtil = util
			target = d
		}
	}

	if source == nil || target == nil || source.ID == target.ID {
		return nil, nil
	}

	return source, target
}

// executeRebalance 执行再平衡
func (m *Manager) executeRebalance(job *RebalanceJob, pool *StoragePool, source, target *DiskInfo) {
	m.mu.Lock()
	job.Status = JobStatusRunning
	m.mu.Unlock()

	// 模拟再平衡过程
	totalBytes := uint64(float64(source.UsedBytes) * 0.1) // 迁移 10% 数据
	job.TotalBytes = totalBytes

	// 更新池状态
	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		completedAt := time.Now()
		job.CompletedAt = &completedAt
		pool.Status = PoolStatusHealthy
		pool.Imbalance = calculateImbalance(pool)
		m.metrics.TotalRebalances++
		m.metrics.TotalBytesMoved += job.BytesMoved
		m.metrics.LastRebalance = &completedAt
	}()

	// 模拟数据迁移
	steps := 10
	for i := 0; i < steps; i++ {
		job.BytesMoved = uint64(float64(totalBytes) * float64(i+1) / float64(steps))
		job.Progress = float64(i+1) / float64(steps)
		time.Sleep(100 * time.Millisecond)
	}

	job.Status = JobStatusCompleted
	job.Progress = 1.0
}

// monitorLoop 监控循环
func (m *Manager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkPools()
		}
	}
}

// rebalanceLoop 自动再平衡循环
func (m *Manager) rebalanceLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.autoRebalance()
		}
	}
}

// checkPools 检查存储池状态
func (m *Manager) checkPools() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pool := range m.pools {
		imbalance := calculateImbalance(pool)
		pool.Imbalance = imbalance

		if imbalance > 0.3 {
			pool.Status = PoolStatusImbalanced
		}
	}
}

// autoRebalance 自动再平衡
func (m *Manager) autoRebalance() {
	m.mu.RLock()
	policies := make([]*RebalancePolicy, 0, len(m.policies))
	for _, p := range m.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	m.mu.RUnlock()

	for _, policy := range policies {
		for _, pool := range m.ListPools() {
			if contains(policy.ExcludePools, pool.ID) {
				continue
			}
			if pool.Imbalance > policy.Threshold {
				m.TriggerRebalance(pool.ID, policy.Strategy)
			}
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func timePtr(t time.Time) *time.Time {
	return &t
}
