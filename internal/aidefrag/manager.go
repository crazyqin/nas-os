package aidefrag

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Manager AI 磁盘碎片整理管理器.
type Manager struct {
	mu       sync.RWMutex
	config   DefragConfig
	disks    map[string]*DiskInfo
	jobs     map[string]*DefragJob
	policies map[string]*DefragPolicy
	running  bool
	defragging bool
	stopCh   chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg DefragConfig) *Manager {
	if cfg.ScanInterval == 0 {
		cfg.ScanInterval = time.Hour
	}
	if cfg.FragThreshold == 0 {
		cfg.FragThreshold = 10.0
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 1
	}
	if cfg.IOLimitMBps == 0 {
		cfg.IOLimitMBps = 100
	}
	return &Manager{
		config:   cfg,
		disks:    make(map[string]*DiskInfo),
		jobs:     make(map[string]*DefragJob),
		policies: make(map[string]*DefragPolicy),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.scanLoop()
	log.Println("[AIDefrag] AI 磁盘碎片整理已启动")
	return nil
}

// Stop 停止.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[AIDefrag] AI 磁盘碎片整理已停止")
}

// ========== 磁盘管理 ==========

// RegisterDisk 注册磁盘.
func (m *Manager) RegisterDisk(disk *DiskInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disks[disk.ID] = disk
	return nil
}

// UnregisterDisk 注销磁盘.
func (m *Manager) UnregisterDisk(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.disks[id]; !ok {
		return ErrDiskNotFound
	}
	delete(m.disks, id)
	return nil
}

// GetDisk 获取磁盘信息.
func (m *Manager) GetDisk(id string) (*DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	disk, ok := m.disks[id]
	if !ok {
		return nil, ErrDiskNotFound
	}
	return disk, nil
}

// ListDisks 列出磁盘.
func (m *Manager) ListDisks() []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*DiskInfo
	for _, d := range m.disks {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Device < result[j].Device
	})
	return result
}

// ========== 整理任务 ==========

// StartDefrag 启动碎片整理.
func (m *Manager) StartDefrag(diskID, targetPath string) (*DefragJob, error) {
	m.mu.Lock()
	if m.defragging {
		m.mu.Unlock()
		return nil, ErrDefragRunning
	}
	disk, ok := m.disks[diskID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrDiskNotFound
	}
	job := &DefragJob{
		ID:         fmt.Sprintf("defrag-%d", time.Now().Unix()),
		DiskID:     diskID,
		TargetPath: targetPath,
		State:      StateScanning,
		StartedAt:  time.Now(),
	}
	m.jobs[job.ID] = job
	m.defragging = true
	m.mu.Unlock()

	go m.executeDefrag(job, disk)
	return job, nil
}

// StopDefrag 停止整理.
func (m *Manager) StopDefrag() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.defragging {
		return ErrDefragNotRunning
	}
	m.defragging = false
	return nil
}

// GetJob 获取任务.
func (m *Manager) GetJob(id string) (*DefragJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, ErrDiskNotFound
	}
	return job, nil
}

// ListJobs 列出任务.
func (m *Manager) ListJobs(limit int) []*DefragJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*DefragJob
	for _, j := range m.jobs {
		result = append(result, j)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ========== 碎片分析 ==========

// AnalyzeFragments 分析碎片.
func (m *Manager) AnalyzeFragments(diskID string) ([]FileFragment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.disks[diskID]; !ok {
		return nil, ErrDiskNotFound
	}
	// 返回模拟分析结果
	return []FileFragment{
		{Path: "/data/large-file.bin", Size: 1024 * 1024 * 500, Fragments: 42, Heat: HeatHot, Priority: 90},
		{Path: "/data/archive.tar.gz", Size: 1024 * 1024 * 200, Fragments: 15, Heat: HeatCold, Priority: 30},
		{Path: "/data/db.sqlite", Size: 1024 * 1024 * 50, Fragments: 8, Heat: HeatHot, Priority: 95},
	}, nil
}

// ========== 策略管理 ==========

// CreatePolicy 创建策略.
func (m *Manager) CreatePolicy(policy *DefragPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.Enabled = true
	policy.CreatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.policies, id)
	return nil
}

// ListPolicies 列出策略.
func (m *Manager) ListPolicies() []*DefragPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*DefragPolicy
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// ========== 统计 ==========

// GetStats 获取统计.
func (m *Manager) GetStats() DefragStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := DefragStats{
		TotalDisks:      len(m.disks),
		FileSystemStats: make(map[string]int),
	}
	var totalFrag float64
	for _, d := range m.disks {
		stats.FileSystemStats[string(d.FileSystem)]++
		if d.FragPercent >= m.config.FragThreshold {
			stats.NeedDefrag++
		}
		totalFrag += d.FragPercent
	}
	if stats.TotalDisks > 0 {
		stats.AvgFragPercent = totalFrag / float64(stats.TotalDisks)
	}
	for _, j := range m.jobs {
		stats.TotalJobs++
		if j.State == StateCompleted {
			stats.CompletedJobs++
			stats.TotalFragments += j.FragmentsReduced
			stats.TotalBytesSaved += j.ProcessedBytes
		}
	}
	return stats
}

// ========== 内部 ==========

func (m *Manager) executeDefrag(job *DefragJob, disk *DiskInfo) {
	m.mu.Lock()
	job.State = StateDefragging
	m.mu.Unlock()

	// 模拟碎片整理
	totalSteps := 10
	for i := 0; i < totalSteps; i++ {
		m.mu.RLock()
		if !m.defragging {
			m.mu.RUnlock()
			job.State = StateIdle
			return
		}
		m.mu.RUnlock()

		m.mu.Lock()
		job.Progress = float64(i+1) / float64(totalSteps) * 100
		job.ProcessedFiles = int64(float64(job.TotalFiles) * job.Progress / 100)
		job.SpeedMBps = 80.5
		job.FragmentsReduced = int64(float64(i+1) * 10)
		m.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
	}

	m.mu.Lock()
	job.State = StateCompleted
	job.Progress = 100
	job.FinishedAt = time.Now()
	disk.FragPercent = 2.0
	disk.LastDefrag = time.Now()
	disk.NeedsDefrag = false
	m.defragging = false
	m.mu.Unlock()
}

func (m *Manager) scanLoop() {
	ticker := time.NewTicker(m.config.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.scanDisks()
		}
	}
}

func (m *Manager) scanDisks() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, disk := range m.disks {
		if disk.FragPercent >= m.config.FragThreshold {
			disk.NeedsDefrag = true
		}
	}
}
