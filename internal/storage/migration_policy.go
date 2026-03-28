// Package storage 提供迁移策略管理器
// 实现数据迁移策略的定义、管理和调度
package storage

import (
	"sync"
	"time"
)

// MigrationPolicyManager 迁移策略管理器
// 管理所有迁移策略的创建、更新、删除和查询
type MigrationPolicyManager struct {
	mu sync.RWMutex

	// 所有策略
	policies map[string]*MigrationPolicy

	// 按状态索引
	activePolicies   map[string]*MigrationPolicy
	inactivePolicies map[string]*MigrationPolicy
}

// MigrationPolicy 迁移策略
type MigrationPolicy struct {
	// 基本信息
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Type        PolicyType   `json:"type"`
	Priority    int          `json:"priority"`

	// 策略模式
	Mode PolicyMode `json:"mode"`

	// 调度配置
	Schedule    ScheduleConfig `json:"schedule"`
	NextRunTime time.Time       `json:"nextRunTime"`
	LastRunTime time.Time       `json:"lastRunTime"`

	// 迁移条件
	Conditions MigrationConditions `json:"conditions"`

	// 目标配置
	SourcePool TierPoolType `json:"sourcePool"`
	TargetPool TierPoolType `json:"targetPool"`

	// 限流配置
	MaxMigrationSize  int64         `json:"maxMigrationSize"`  // 单次最大迁移量（字节）
	MaxMigrationFiles int           `json:"maxMigrationFiles"` // 单次最大文件数
	BandwidthLimit    int64         `json:"bandwidthLimit"`   // 带宽限制（字节/秒）
	TimeWindow        TimeWindow    `json:"timeWindow"`       // 执行时间窗口

	// 统计
	Stats MigrationStats `json:"stats"`

	// 元数据
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy"`
}

// PolicyType 策略类型
type PolicyType string

const (
	PolicyTypePromote PolicyType = "promote" // 冷数据提升到热池
	PolicyTypeDemote  PolicyType = "demote"  // 热数据降级到冷池
	PolicyTypeBalance PolicyType = "balance" // 负载均衡
)

// PolicyMode 策略模式
type PolicyMode string

const (
	PolicyModeAuto    PolicyMode = "auto"    // 自动调度
	PolicyModeManual  PolicyMode = "manual"  // 手动触发
	PolicyModeSchedule PolicyMode = "schedule" // 定时调度
)

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Cron      string    `json:"cron"`      // Cron表达式
	Interval  Duration  `json:"interval"`  // 间隔时间
	StartTime time.Time `json:"startTime"` // 开始时间
	EndTime   time.Time `json:"endTime"`   // 结束时间（可选）
}

// Duration 自定义Duration类型用于JSON序列化
type Duration time.Duration

// MigrationConditions 迁移条件
type MigrationConditions struct {
	// 访问频率条件
	MinAccessCount   int     `json:"minAccessCount"`   // 最小访问次数
	MaxAccessCount   int     `json:"maxAccessCount"`   // 最大访问次数
	AccessFrequency  float64 `json:"accessFrequency"`  // 访问频率阈值

	// 时间条件
	MinAgeDays int `json:"minAgeDays"` // 最小存活天数
	MaxAgeDays int `json:"maxAgeDays"` // 最大存活天数

	// 文件大小条件
	MinFileSize int64 `json:"minFileSize"` // 最小文件大小
	MaxFileSize int64 `json:"maxFileSize"` // 最大文件大小

	// 文件类型条件
	IncludeExtensions []string `json:"includeExtensions"` // 包含的扩展名
	ExcludeExtensions []string `json:"excludeExtensions"` // 排除的扩展名

	// 路径条件
	IncludePaths []string `json:"includePaths"` // 包含的路径
	ExcludePaths []string `json:"excludePaths"` // 排除的路径

	// 池容量条件
	SourcePoolMinFreePercent float64 `json:"sourcePoolMinFreePercent"` // 源池最小空闲百分比
	TargetPoolMaxUsedPercent float64 `json:"targetPoolMaxUsedPercent"` // 目标池最大使用百分比
}

// TimeWindow 执行时间窗口
type TimeWindow struct {
	StartHour int `json:"startHour"` // 开始小时（0-23）
	EndHour   int `json:"endHour"`   // 结束小时（0-23）
	Days      []int `json:"days"`     // 执行的星期几（0=周日，1-6=周一到周六）
}

// MigrationStats 迁移统计
type MigrationStats struct {
	TotalRuns        int64     `json:"totalRuns"`
	SuccessRuns      int64     `json:"successRuns"`
	FailedRuns       int64     `json:"failedRuns"`
	TotalMigrated     int64     `json:"totalMigrated"`     // 总迁移文件数
	TotalBytesMigrated int64   `json:"totalBytesMigrated"` // 总迁移字节数
	LastRunDuration   Duration  `json:"lastRunDuration"`
	AverageRunDuration Duration `json:"averageRunDuration"`
}

// NewMigrationPolicyManager 创建迁移策略管理器
func NewMigrationPolicyManager() *MigrationPolicyManager {
	return &MigrationPolicyManager{
		policies:         make(map[string]*MigrationPolicy),
		activePolicies:   make(map[string]*MigrationPolicy),
		inactivePolicies: make(map[string]*MigrationPolicy),
	}
}

// GetActivePolicies 获取所有活跃策略
func (m *MigrationPolicyManager) GetActivePolicies() []*MigrationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*MigrationPolicy, 0, len(m.activePolicies))
	for _, p := range m.activePolicies {
		policies = append(policies, p)
	}
	return policies
}

// GetAllPolicies 获取所有策略
func (m *MigrationPolicyManager) GetAllPolicies() []*MigrationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*MigrationPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// GetPolicy 获取指定策略
func (m *MigrationPolicyManager) GetPolicy(id string) *MigrationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policies[id]
}

// CreatePolicy 创建策略
func (m *MigrationPolicyManager) CreatePolicy(policy *MigrationPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generatePolicyID()
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy

	if policy.Enabled {
		m.activePolicies[policy.ID] = policy
	} else {
		m.inactivePolicies[policy.ID] = policy
	}

	return nil
}

// UpdatePolicy 更新策略
func (m *MigrationPolicyManager) UpdatePolicy(policy *MigrationPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policy.ID]; !exists {
		return ErrPolicyNotFound
	}

	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy

	// 更新索引
	delete(m.activePolicies, policy.ID)
	delete(m.inactivePolicies, policy.ID)

	if policy.Enabled {
		m.activePolicies[policy.ID] = policy
	} else {
		m.inactivePolicies[policy.ID] = policy
	}

	return nil
}

// UpdatePolicyNextRun 更新策略下次运行时间
func (m *MigrationPolicyManager) UpdatePolicyNextRun(id string, nextRun time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}

	policy.NextRunTime = nextRun
	policy.UpdatedAt = time.Now()
	return nil
}

// DeletePolicy 删除策略
func (m *MigrationPolicyManager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return ErrPolicyNotFound
	}

	delete(m.policies, id)
	delete(m.activePolicies, id)
	delete(m.inactivePolicies, id)

	return nil
}

// EnablePolicy 启用策略
func (m *MigrationPolicyManager) EnablePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}

	policy.Enabled = true
	policy.UpdatedAt = time.Now()
	delete(m.inactivePolicies, id)
	m.activePolicies[id] = policy

	return nil
}

// DisablePolicy 禁用策略
func (m *MigrationPolicyManager) DisablePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}

	policy.Enabled = false
	policy.UpdatedAt = time.Now()
	delete(m.activePolicies, id)
	m.inactivePolicies[id] = policy

	return nil
}

// 错误定义
var ErrPolicyNotFound = &SchedulerError{Message: "策略不存在"}

// generatePolicyID 生成策略ID
func generatePolicyID() string {
	return "policy_" + time.Now().Format("20060102150405")
}