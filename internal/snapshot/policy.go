// Package snapshot 提供快照策略管理功能
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PolicyType 策略类型
type PolicyType string

const (
	// PolicyTypeManual 手动快照
	PolicyTypeManual PolicyType = "manual"
	// PolicyTypeScheduled 定时快照
	PolicyTypeScheduled PolicyType = "scheduled"
	// PolicyTypeApplicationConsistent 应用一致性快照
	PolicyTypeApplicationConsistent PolicyType = "application_consistent"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	// ScheduleTypeHourly 每小时
	ScheduleTypeHourly ScheduleType = "hourly"
	// ScheduleTypeDaily 每天
	ScheduleTypeDaily ScheduleType = "daily"
	// ScheduleTypeWeekly 每周
	ScheduleTypeWeekly ScheduleType = "weekly"
	// ScheduleTypeMonthly 每月
	ScheduleTypeMonthly ScheduleType = "monthly"
	// ScheduleTypeCustom 自定义 cron 表达式
	ScheduleTypeCustom ScheduleType = "custom"
)

// RetentionPolicyType 保留策略类型
type RetentionPolicyType string

const (
	// RetentionByCount 按数量保留
	RetentionByCount RetentionPolicyType = "by_count"
	// RetentionByAge 按时间保留
	RetentionByAge RetentionPolicyType = "by_age"
	// RetentionBySize 按存储空间限制
	RetentionBySize RetentionPolicyType = "by_size"
	// RetentionCombined 组合策略
	RetentionCombined RetentionPolicyType = "combined"
)

// RetentionPolicy 保留策略配置
type RetentionPolicy struct {
	// Type 策略类型
	Type RetentionPolicyType `json:"type"`

	// MaxCount 保留最近 N 个快照（按数量）
	MaxCount int `json:"maxCount,omitempty"`

	// MaxAgeDays 保留最近 N 天的快照（按时间）
	MaxAgeDays int `json:"maxAgeDays,omitempty"`

	// MaxSizeBytes 最大存储空间（字节）
	MaxSizeBytes int64 `json:"maxSizeBytes,omitempty"`

	// 组合策略的子策略
	CountPolicy *RetentionPolicy `json:"countPolicy,omitempty"`
	AgePolicy   *RetentionPolicy `json:"agePolicy,omitempty"`
	SizePolicy  *RetentionPolicy `json:"sizePolicy,omitempty"`
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	// Type 调度类型
	Type ScheduleType `json:"type"`

	// CronExpression cron 表达式（custom 类型使用）
	CronExpression string `json:"cronExpression,omitempty"`

	// Hour 小时（daily/weekly/monthly 使用）
	Hour int `json:"hour,omitempty"`

	// Minute 分钟
	Minute int `json:"minute,omitempty"`

	// DayOfWeek 星期几（weekly 使用，0=周日）
	DayOfWeek int `json:"dayOfWeek,omitempty"`

	// DayOfMonth 每月几号（monthly 使用）
	DayOfMonth int `json:"dayOfMonth,omitempty"`

	// IntervalHours 间隔小时数（hourly 使用）
	IntervalHours int `json:"intervalHours,omitempty"`
}

// ScriptConfig 脚本配置（应用一致性快照）
type ScriptConfig struct {
	// PreSnapshotScript 快照前执行的脚本
	PreSnapshotScript string `json:"preSnapshotScript,omitempty"`

	// PostSnapshotScript 快照后执行的脚本
	PostSnapshotScript string `json:"postSnapshotScript,omitempty"`

	// TimeoutSeconds 脚本执行超时时间
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`

	// ContinueOnFailure 脚本失败是否继续创建快照
	ContinueOnFailure bool `json:"continueOnFailure,omitempty"`
}

// Policy 快照策略
type Policy struct {
	// ID 策略 ID
	ID string `json:"id"`

	// Name 策略名称
	Name string `json:"name"`

	// Description 策略描述
	Description string `json:"description,omitempty"`

	// Type 策略类型
	Type PolicyType `json:"type"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// VolumeName 目标卷名
	VolumeName string `json:"volumeName"`

	// SubvolumeName 目标子卷名
	SubvolumeName string `json:"subvolumeName"`

	// SnapshotDir 快照存储目录（相对于卷根目录）
	SnapshotDir string `json:"snapshotDir"`

	// SnapshotPrefix 快照名称前缀
	SnapshotPrefix string `json:"snapshotPrefix"`

	// ReadOnly 是否创建只读快照
	ReadOnly bool `json:"readOnly"`

	// Schedule 调度配置（定时快照）
	Schedule *ScheduleConfig `json:"schedule,omitempty"`

	// Retention 保留策略
	Retention *RetentionPolicy `json:"retention"`

	// Scripts 脚本配置（应用一致性快照）
	Scripts *ScriptConfig `json:"scripts,omitempty"`

	// Tags 标签
	Tags []string `json:"tags,omitempty"`

	// Metadata 元数据
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`

	// LastRunAt 最后执行时间
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`

	// NextRunAt 下次执行时间
	NextRunAt *time.Time `json:"nextRunAt,omitempty"`

	// LastError 最后错误信息
	LastError string `json:"lastError,omitempty"`

	// Stats 执行统计
	Stats PolicyStats `json:"stats"`
}

// PolicyStats 策略执行统计
type PolicyStats struct {
	// TotalRuns 总执行次数
	TotalRuns int `json:"totalRuns"`

	// SuccessfulRuns 成功次数
	SuccessfulRuns int `json:"successfulRuns"`

	// FailedRuns 失败次数
	FailedRuns int `json:"failedRuns"`

	// LastSuccessAt 最后成功时间
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`

	// LastFailureAt 最后失败时间
	LastFailureAt *time.Time `json:"lastFailureAt,omitempty"`

	// TotalSnapshotsCreated 创建的快照总数
	TotalSnapshotsCreated int `json:"totalSnapshotsCreated"`

	// TotalSnapshotsDeleted 删除的快照总数（清理）
	TotalSnapshotsDeleted int `json:"totalSnapshotsDeleted"`

	// TotalBytesSaved 保存的总字节数
	TotalBytesSaved int64 `json:"totalBytesSaved"`
}

// PolicyManager 策略管理器
type PolicyManager struct {
	mu sync.RWMutex

	// policies 策略存储
	policies map[string]*Policy

	// configPath 配置文件路径
	configPath string

	// storageMgr 存储管理器接口
	storageMgr StorageManager

	// scheduler 调度器
	scheduler *Scheduler

	// executor 快照执行器
	executor *SnapshotExecutor

	// cleaner 清理器
	cleaner *RetentionCleaner

	// hooks 事件钩子
	hooks PolicyHooks
}

// StorageManager 存储管理器接口
type StorageManager interface {
	CreateSnapshot(volumeName, subvolName, snapshotName string, readOnly bool) (interface{}, error)
	DeleteSnapshot(volumeName, snapshotName string) error
	ListSnapshots(volumeName string) ([]interface{}, error)
	GetVolume(volumeName string) interface{}
}

// PolicyHooks 策略事件钩子
type PolicyHooks struct {
	// OnBeforeSnapshot 快照创建前回调
	OnBeforeSnapshot func(policy *Policy) error

	// OnAfterSnapshot 快照创建后回调
	OnAfterSnapshot func(policy *Policy, snapshotName string, err error)

	// OnSnapshotDeleted 快照删除回调
	OnSnapshotDeleted func(policy *Policy, snapshotName string)

	// OnPolicyEnabled 策略启用回调
	OnPolicyEnabled func(policy *Policy)

	// OnPolicyDisabled 策略禁用回调
	OnPolicyDisabled func(policy *Policy)
}

// NewPolicyManager 创建策略管理器
func NewPolicyManager(configPath string, storageMgr StorageManager) *PolicyManager {
	pm := &PolicyManager{
		policies:   make(map[string]*Policy),
		configPath: configPath,
		storageMgr: storageMgr,
	}

	// 初始化调度器
	pm.scheduler = NewScheduler(pm)

	// 初始化执行器
	pm.executor = NewSnapshotExecutor(storageMgr)

	// 初始化清理器
	pm.cleaner = NewRetentionCleaner(storageMgr)

	return pm
}

// Initialize 初始化策略管理器
func (pm *PolicyManager) Initialize() error {
	// 加载配置
	if err := pm.loadConfig(); err != nil {
		// 配置文件不存在是正常的
		pm.mu.Lock()
		pm.policies = make(map[string]*Policy)
		pm.mu.Unlock()
	}

	// 启动调度器
	pm.scheduler.Start()

	// 注册所有启用的策略
	pm.mu.RLock()
	for _, policy := range pm.policies {
		if policy.Enabled && policy.Type == PolicyTypeScheduled {
			_ = pm.scheduler.AddJob(policy)
		}
	}
	pm.mu.RUnlock()

	return nil
}

// Close 关闭策略管理器
func (pm *PolicyManager) Close() {
	pm.scheduler.Stop()
}

// ========== 策略 CRUD ==========

// CreatePolicy 创建策略
func (pm *PolicyManager) CreatePolicy(policy *Policy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 验证
	if err := pm.validatePolicy(policy); err != nil {
		return err
	}

	// 生成 ID
	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}

	// 设置时间戳
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	// 应用默认值
	pm.applyDefaults(policy)

	// 保存
	pm.policies[policy.ID] = policy

	if err := pm.saveConfig(); err != nil {
		delete(pm.policies, policy.ID)
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 如果是定时策略且已启用，添加到调度器
	if policy.Enabled && policy.Type == PolicyTypeScheduled {
		_ = pm.scheduler.AddJob(policy)
	}

	return nil
}

// GetPolicy 获取策略
func (pm *PolicyManager) GetPolicy(id string) (*Policy, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policy, ok := pm.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	return policy, nil
}

// ListPolicies 列出所有策略
func (pm *PolicyManager) ListPolicies() []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Policy, 0, len(pm.policies))
	for _, p := range pm.policies {
		result = append(result, p)
	}
	return result
}

// ListPoliciesByVolume 列出指定卷的策略
func (pm *PolicyManager) ListPoliciesByVolume(volumeName string) []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Policy, 0)
	for _, p := range pm.policies {
		if p.VolumeName == volumeName {
			result = append(result, p)
		}
	}
	return result
}

// UpdatePolicy 更新策略
func (pm *PolicyManager) UpdatePolicy(id string, updates *Policy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	existing, ok := pm.policies[id]
	if !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	// 验证更新
	if err := pm.validatePolicy(updates); err != nil {
		return err
	}

	// 保留不可变字段
	updates.ID = id
	updates.CreatedAt = existing.CreatedAt
	updates.UpdatedAt = time.Now()

	// 更新策略
	pm.policies[id] = updates

	// 更新调度器
	if existing.Type == PolicyTypeScheduled {
		pm.scheduler.RemoveJob(id)
		if updates.Enabled && updates.Type == PolicyTypeScheduled {
			_ = pm.scheduler.AddJob(updates)
		}
	}

	if err := pm.saveConfig(); err != nil {
		pm.policies[id] = existing
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// DeletePolicy 删除策略
func (pm *PolicyManager) DeletePolicy(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policy, ok := pm.policies[id]
	if !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	// 从调度器移除
	if policy.Type == PolicyTypeScheduled {
		pm.scheduler.RemoveJob(id)
	}

	// 删除
	delete(pm.policies, id)

	if err := pm.saveConfig(); err != nil {
		pm.policies[id] = policy
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// EnablePolicy 启用/禁用策略
func (pm *PolicyManager) EnablePolicy(id string, enabled bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policy, ok := pm.policies[id]
	if !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	policy.Enabled = enabled
	policy.UpdatedAt = time.Now()

	// 更新调度器
	if policy.Type == PolicyTypeScheduled {
		if enabled {
			_ = pm.scheduler.AddJob(policy)
		} else {
			pm.scheduler.RemoveJob(id)
		}
	}

	// 触发钩子
	if pm.hooks.OnPolicyEnabled != nil && enabled {
		pm.hooks.OnPolicyEnabled(policy)
	} else if pm.hooks.OnPolicyDisabled != nil && !enabled {
		pm.hooks.OnPolicyDisabled(policy)
	}

	return pm.saveConfig()
}

// ========== 快照执行 ==========

// ExecutePolicy 手动执行策略
func (pm *PolicyManager) ExecutePolicy(id string) (string, error) {
	pm.mu.RLock()
	policy, ok := pm.policies[id]
	pm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("策略不存在: %s", id)
	}

	return pm.executePolicy(policy)
}

// executePolicy 执行策略（内部方法）
func (pm *PolicyManager) executePolicy(policy *Policy) (string, error) {
	// 触发前置钩子
	if pm.hooks.OnBeforeSnapshot != nil {
		if err := pm.hooks.OnBeforeSnapshot(policy); err != nil {
			return "", fmt.Errorf("前置钩子失败: %w", err)
		}
	}

	// 执行快照
	snapshotName, err := pm.executor.Execute(policy)

	// 更新统计
	pm.mu.Lock()
	policy.Stats.TotalRuns++
	now := time.Now()
	policy.LastRunAt = &now

	if err != nil {
		policy.Stats.FailedRuns++
		policy.Stats.LastFailureAt = &now
		policy.LastError = err.Error()
	} else {
		policy.Stats.SuccessfulRuns++
		policy.Stats.LastSuccessAt = &now
		policy.Stats.TotalSnapshotsCreated++
		policy.LastError = ""

		// 计算下次执行时间
		if policy.Type == PolicyTypeScheduled && policy.Schedule != nil {
			nextRun := pm.scheduler.CalculateNextRun(policy)
			policy.NextRunAt = &nextRun
		}
	}
	pm.mu.Unlock()

	// 保存状态
	pm.saveConfig()

	// 触发后置钩子
	if pm.hooks.OnAfterSnapshot != nil {
		pm.hooks.OnAfterSnapshot(policy, snapshotName, err)
	}

	// 执行清理
	if policy.Retention != nil {
		go pm.runCleanup(policy)
	}

	return snapshotName, err
}

// runCleanup 执行清理
func (pm *PolicyManager) runCleanup(policy *Policy) {
	deleted, err := pm.cleaner.Clean(policy)
	if err != nil {
		fmt.Printf("清理策略 %s 的快照失败: %v\n", policy.Name, err)
		return
	}

	pm.mu.Lock()
	policy.Stats.TotalSnapshotsDeleted += len(deleted)
	pm.mu.Unlock()
	pm.saveConfig()
}

// ========== 验证和默认值 ==========

func (pm *PolicyManager) validatePolicy(policy *Policy) error {
	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if policy.VolumeName == "" {
		return fmt.Errorf("目标卷名不能为空")
	}

	if policy.Type == PolicyTypeScheduled && policy.Schedule == nil {
		return fmt.Errorf("定时策略必须配置调度")
	}

	if policy.Schedule != nil && policy.Schedule.Type == ScheduleTypeCustom {
		if policy.Schedule.CronExpression == "" {
			return fmt.Errorf("自定义调度必须提供 cron 表达式")
		}
		// 验证 cron 表达式
		if !pm.scheduler.ValidateCron(policy.Schedule.CronExpression) {
			return fmt.Errorf("无效的 cron 表达式: %s", policy.Schedule.CronExpression)
		}
	}

	if policy.Retention == nil {
		return fmt.Errorf("必须配置保留策略")
	}

	return nil
}

func (pm *PolicyManager) applyDefaults(policy *Policy) {
	if policy.Type == "" {
		policy.Type = PolicyTypeManual
	}

	if policy.SnapshotDir == "" {
		policy.SnapshotDir = ".snapshots"
	}

	if policy.Retention == nil {
		policy.Retention = &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 10,
		}
	}

	if policy.Scripts != nil && policy.Scripts.TimeoutSeconds == 0 {
		policy.Scripts.TimeoutSeconds = 300 // 默认 5 分钟超时
	}
}

// ========== 配置持久化 ==========

func (pm *PolicyManager) loadConfig() error {
	data, err := os.ReadFile(pm.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &pm.policies)
}

func (pm *PolicyManager) saveConfig() error {
	data, err := json.MarshalIndent(pm.policies, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(pm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(pm.configPath, data, 0644)
}

// SetHooks 设置事件钩子
func (pm *PolicyManager) SetHooks(hooks PolicyHooks) {
	pm.hooks = hooks
}

// GetScheduler 获取调度器
func (pm *PolicyManager) GetScheduler() *Scheduler {
	return pm.scheduler
}

// GetExecutor 获取执行器
func (pm *PolicyManager) GetExecutor() *SnapshotExecutor {
	return pm.executor
}

// GetCleaner 获取清理器
func (pm *PolicyManager) GetCleaner() *RetentionCleaner {
	return pm.cleaner
}
