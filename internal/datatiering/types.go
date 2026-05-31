// Package datatiering 智能数据分层
// 对标群晖 Smart Tiering，根据访问频率和文件年龄自动迁移数据到不同存储层
package datatiering

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// StorageTier 存储层级
type StorageTier string

const (
	TierHot  StorageTier = "hot"  // SSD/NVMe 高速层
	TierWarm StorageTier = "warm" // HDD 普通层
	TierCold StorageTier = "cold" // 归档/远程 低速层
)

// TierPolicy 分层策略
type TierPolicy struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Enabled      bool        `json:"enabled"`
	HotToWarmDays  int       `json:"hot_to_warm_days"`  // 热→温 天数阈值
	WarmToColdDays int       `json:"warm_to_cold_days"` // 温→冷 天数阈值
	MinFileSize    int64     `json:"min_file_size"`     // 最小文件大小
	MaxFileSize    int64     `json:"max_file_size"`     // 最大文件大小
	IncludeExts    []string  `json:"include_exts"`      // 包含的扩展名
	ExcludeExts    []string  `json:"exclude_exts"`      // 排除的扩展名
	ExcludePaths   []string  `json:"exclude_paths"`     // 排除的路径
	Schedule       string    `json:"schedule"`           // cron 表达式
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TieredFile 分层文件
type TieredFile struct {
	Path          string      `json:"path"`
	Size          int64       `json:"size"`
	CurrentTier   StorageTier `json:"current_tier"`
	TargetTier    StorageTier `json:"target_tier,omitempty"`
	LastAccessed  time.Time   `json:"last_accessed"`
	LastModified  time.Time   `json:"last_modified"`
	AccessCount   int64       `json:"access_count"`
	AccessPattern string      `json:"access_pattern"` // sequential/random/read-heavy
}

// MigrationJob 迁移任务
type MigrationJob struct {
	ID          string       `json:"id"`
	PolicyID    string       `json:"policy_id"`
	Status      JobStatus    `json:"status"`
	FromTier    StorageTier  `json:"from_tier"`
	ToTier      StorageTier  `json:"to_tier"`
	TotalFiles  int          `json:"total_files"`
	TotalSize   int64        `json:"total_size"`
	Migrated    int          `json:"migrated"`
	Failed      int          `json:"failed"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// JobStatus 任务状态
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// TierStats 分层统计
type TierStats struct {
	Tier       StorageTier `json:"tier"`
	FileCount  int         `json:"file_count"`
	TotalSize  int64       `json:"total_size"`
	UsedPercent float64    `json:"used_percent"`
	AvailableGB float64    `json:"available_gb"`
}

// TieringReport 分层报告
type TieringReport struct {
	Tiers       []TierStats    `json:"tiers"`
	TotalFiles  int            `json:"total_files"`
	TotalSize   int64          `json:"total_size"`
	RecentJobs  []MigrationJob `json:"recent_jobs"`
	Suggestions []string       `json:"suggestions"`
	GeneratedAt time.Time      `json:"generated_at"`
}

// Manager 智能数据分层管理器
type Manager struct {
	mu       sync.RWMutex
	policies map[string]*TierPolicy
	files    map[string]*TieredFile
	jobs     map[string]*MigrationJob
	tierCap  map[StorageTier]int64 // 各层容量
	stopCh   chan struct{}
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		policies: make(map[string]*TierPolicy),
		files:    make(map[string]*TieredFile),
		jobs:     make(map[string]*MigrationJob),
		tierCap: map[StorageTier]int64{
			TierHot:  500 * 1024 * 1024 * 1024,  // 500GB
			TierWarm: 2000 * 1024 * 1024 * 1024, // 2TB
			TierCold: 8000 * 1024 * 1024 * 1024, // 8TB
		},
		stopCh: make(chan struct{}),
	}
}

// AddPolicy 添加分层策略
func (m *Manager) AddPolicy(policy *TierPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("策略ID不能为空")
	}
	if policy.HotToWarmDays <= 0 {
		policy.HotToWarmDays = 30
	}
	if policy.WarmToColdDays <= 0 {
		policy.WarmToColdDays = 90
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(policy *TierPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policy.ID]; !exists {
		return fmt.Errorf("策略不存在: %s", policy.ID)
	}
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(policyID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policyID]; !exists {
		return false
	}
	delete(m.policies, policyID)
	return true
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []TierPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]TierPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, *p)
	}
	return result
}

// RegisterFile 注册文件元数据
func (m *Manager) RegisterFile(file *TieredFile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[file.Path] = file
}

// AnalyzeAndMigrate 分析并执行迁移
func (m *Manager) AnalyzeAndMigrate(policyID string) (*MigrationJob, error) {
	m.mu.RLock()
	policy, exists := m.policies[policyID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", policyID)
	}

	job := &MigrationJob{
		ID:       fmt.Sprintf("job_%d", time.Now().UnixNano()),
		PolicyID: policyID,
		Status:   JobPending,
	}

	// 分析文件，确定需要迁移的文件
	toMigrate := m.analyzeFiles(policy)
	job.TotalFiles = len(toMigrate)

	var totalSize int64
	for _, f := range toMigrate {
		totalSize += f.Size
	}
	job.TotalSize = totalSize

	if len(toMigrate) == 0 {
		job.Status = JobCompleted
		now := time.Now()
		job.CompletedAt = &now
		m.mu.Lock()
		m.jobs[job.ID] = job
		m.mu.Unlock()
		return job, nil
	}

	// 确定迁移方向
	job.FromTier = toMigrate[0].CurrentTier
	job.ToTier = toMigrate[0].TargetTier

	// 启动异步迁移
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go m.executeMigration(job, toMigrate, policy)
	return job, nil
}

func (m *Manager) analyzeFiles(policy *TierPolicy) []*TieredFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	toMigrate := make([]*TieredFile, 0)

	for _, file := range m.files {
		// 检查文件大小
		if policy.MinFileSize > 0 && file.Size < policy.MinFileSize {
			continue
		}
		if policy.MaxFileSize > 0 && file.Size > policy.MaxFileSize {
			continue
		}

		// 检查排除路径
		excluded := false
		for _, ep := range policy.ExcludePaths {
			if len(file.Path) >= len(ep) && file.Path[:len(ep)] == ep {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		daysSinceAccess := int(now.Sub(file.LastAccessed).Hours() / 24)

		var targetTier StorageTier
		switch file.CurrentTier {
		case TierHot:
			if daysSinceAccess >= policy.HotToWarmDays {
				targetTier = TierWarm
			}
		case TierWarm:
			if daysSinceAccess >= policy.WarmToColdDays {
				targetTier = TierCold
			}
		case TierCold:
			// 冷数据不自动提升，除非频繁访问
			if file.AccessCount > 100 && daysSinceAccess < 7 {
				targetTier = TierWarm
			}
		}

		if targetTier != "" && targetTier != file.CurrentTier {
			f := *file
			f.TargetTier = targetTier
			toMigrate = append(toMigrate, &f)
		}
	}

	// 按大小排序，优先迁移大文件
	sort.Slice(toMigrate, func(i, j int) bool {
		return toMigrate[i].Size > toMigrate[j].Size
	})

	return toMigrate
}

func (m *Manager) executeMigration(job *MigrationJob, files []*TieredFile, policy *TierPolicy) {
	m.mu.Lock()
	now := time.Now()
	job.Status = JobRunning
	job.StartedAt = &now
	m.mu.Unlock()

	for _, file := range files {
		// 模拟迁移
		m.mu.Lock()
		if f, exists := m.files[file.Path]; exists {
			f.CurrentTier = file.TargetTier
			f.TargetTier = ""
		}
		job.Migrated++
		m.mu.Unlock()
	}

	m.mu.Lock()
	completed := time.Now()
	job.Status = JobCompleted
	job.CompletedAt = &completed
	m.mu.Unlock()
}

// GetJob 获取迁移任务
func (m *Manager) GetJob(jobID string) (*MigrationJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}
	return job, nil
}

// ListJobs 列出所有任务
func (m *Manager) ListJobs() []MigrationJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MigrationJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		result = append(result, *j)
	}
	return result
}

// GetTierStats 获取分层统计
func (m *Manager) GetTierStats() []TierStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[StorageTier]*TierStats{
		TierHot:  {Tier: TierHot},
		TierWarm: {Tier: TierWarm},
		TierCold: {Tier: TierCold},
	}

	for _, file := range m.files {
		if s, ok := stats[file.CurrentTier]; ok {
			s.FileCount++
			s.TotalSize += file.Size
		}
	}

	capGB := float64(1024 * 1024 * 1024)
	for tier, s := range stats {
		cap := float64(m.tierCap[tier])
		if cap > 0 {
			s.UsedPercent = float64(s.TotalSize) / cap * 100
			s.AvailableGB = (cap - float64(s.TotalSize)) / capGB
		}
	}

	result := make([]TierStats, 0, 3)
	for _, s := range stats {
		result = append(result, *s)
	}
	return result
}

// GetReport 获取分层报告
func (m *Manager) GetReport() *TieringReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &TieringReport{
		Tiers:       make([]TierStats, 0),
		RecentJobs:  make([]MigrationJob, 0),
		Suggestions: make([]string, 0),
		GeneratedAt: time.Now(),
	}

	stats := map[StorageTier]*TierStats{
		TierHot:  {Tier: TierHot},
		TierWarm: {Tier: TierWarm},
		TierCold: {Tier: TierCold},
	}

	for _, file := range m.files {
		report.TotalFiles++
		report.TotalSize += file.Size
		if s, ok := stats[file.CurrentTier]; ok {
			s.FileCount++
			s.TotalSize += file.Size
		}
	}

	for _, s := range stats {
		report.Tiers = append(report.Tiers, *s)
	}

	// 生成建议
	for _, s := range stats {
		if s.UsedPercent > 80 {
			report.Suggestions = append(report.Suggestions,
				fmt.Sprintf("%s 层使用率 %.1f%%，建议扩容或迁移数据", s.Tier, s.UsedPercent))
		}
	}

	// 最近任务
	for _, j := range m.jobs {
		report.RecentJobs = append(report.RecentJobs, *j)
	}

	return report
}
