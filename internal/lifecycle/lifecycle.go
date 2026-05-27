// Package lifecycle 数据生命周期管理模块
// 自动化数据迁移策略、成本优化建议
// 学习群晖Active Backup + TrueNAS分层存储策略
package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StorageTier 存储层级
type StorageTier string

const (
	TierHot    StorageTier = "hot"    // SSD/NVMe - 高频访问
	TierWarm   StorageTier = "warm"   // HDD - 中频访问
	TierCold   StorageTier = "cold"   // 归档存储 - 低频访问
	TierGlacier StorageTier = "glacier" // 云归档 - 极低频
)

// DataCategory 数据分类
type DataCategory string

const (
	CategoryDocument DataCategory = "document"
	CategoryMedia    DataCategory = "media"
	CategoryBackup   DataCategory = "backup"
	CategoryArchive  DataCategory = "archive"
	CategoryTemp     DataCategory = "temp"
)

// AccessPattern 访问模式
type AccessPattern struct {
	LastAccess    time.Time `json:"last_access"`
	AccessCount   int64     `json:"access_count"`
	AvgAccessFreq float64   `json:"avg_access_freq"` // 次/天
	IsPinned      bool      `json:"is_pinned"`       // 钉住不迁移
}

// LifecycleRule 生命周期规则
type LifecycleRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	
	// 匹配条件
	PathPattern string       `json:"path_pattern"` // glob模式
	Category    DataCategory `json:"category,omitempty"`
	MinSize     int64        `json:"min_size,omitempty"` // 字节
	MaxSize     int64        `json:"max_size,omitempty"`
	
	// 生命周期阶段
	Stages      []Stage      `json:"stages"`
	
	// 执行配置
	Schedule    string       `json:"schedule"` // cron表达式
	DryRun      bool         `json:"dry_run"`
	
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Stage 生命周期阶段
type Stage struct {
	Name       string       `json:"name"`
	AfterDays  int          `json:"after_days"` // 创建/修改后多少天
	TargetTier StorageTier  `json:"target_tier"`
	Action     StageAction  `json:"action"`
	Notify     bool         `json:"notify,omitempty"`
}

// StageAction 阶段动作
type StageAction string

const (
	ActionMigrate  StageAction = "migrate"  // 迁移存储层
	ActionCompress StageAction = "compress" // 压缩
	ActionArchive  StageAction = "archive"  // 归档
	ActionDelete   StageAction = "delete"   // 删除
	ActionNotify   StageAction = "notify"   // 仅通知
)

// MigrationTask 迁移任务
type MigrationTask struct {
	ID          string      `json:"id"`
	RuleID      string      `json:"rule_id"`
	SourcePath  string      `json:"source_path"`
	TargetPath  string      `json:"target_path"`
	SourceTier  StorageTier `json:"source_tier"`
	TargetTier  StorageTier `json:"target_tier"`
	FileSize    int64       `json:"file_size"`
	Status      TaskStatus  `json:"status"`
	Progress    float64     `json:"progress"` // 0-100
	Error       string      `json:"error,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// StorageStats 存储统计
type StorageStats struct {
	Tier       StorageTier `json:"tier"`
	TotalSize  int64       `json:"total_size"`
	UsedSize   int64       `json:"used_size"`
	FileCount  int64       `json:"file_count"`
	CostPerGB  float64     `json:"cost_per_gb"` // 每GB每月成本
}

// CostReport 成本报告
type CostReport struct {
	GeneratedAt  time.Time      `json:"generated_at"`
	Period       string         `json:"period"` // monthly, quarterly, yearly
	TierStats    []StorageStats `json:"tier_stats"`
	TotalCost    float64        `json:"total_cost"`
	PotentialSavings float64    `json:"potential_savings"`
	Recommendations  []string   `json:"recommendations"`
}

// Service 生命周期管理服务
type Service struct {
	mu       sync.RWMutex
	rules    map[string]*LifecycleRule
	tasks    map[string]*MigrationTask
	stats    map[StorageTier]*StorageStats
	config   *LifecycleConfig
}

// LifecycleConfig 生命周期配置
type LifecycleConfig struct {
	Enabled          bool    `json:"enabled"`
	ScanInterval     int     `json:"scan_interval"` // 分钟
	MaxConcurrent    int     `json:"max_concurrent"`
	BandwidthLimit   int64   `json:"bandwidth_limit"` // 字节/秒
	HotTierPath      string  `json:"hot_tier_path"`
	WarmTierPath     string  `json:"warm_tier_path"`
	ColdTierPath     string  `json:"cold_tier_path"`
	GlacierEndpoint  string  `json:"glacier_endpoint,omitempty"`
	CostPerGBHot     float64 `json:"cost_per_gb_hot"`
	CostPerGBWarm    float64 `json:"cost_per_gb_warm"`
	CostPerGBCold    float64 `json:"cost_per_gb_cold"`
	CostPerGBGlacier float64 `json:"cost_per_gb_glacier"`
}

// NewService 创建生命周期管理服务
func NewService(config *LifecycleConfig) *Service {
	if config == nil {
		config = &LifecycleConfig{
			Enabled:       true,
			ScanInterval:  60,
			MaxConcurrent: 4,
			CostPerGBHot:     0.023,
			CostPerGBWarm:    0.012,
			CostPerGBCold:    0.004,
			CostPerGBGlacier: 0.001,
		}
	}
	
	return &Service{
		rules:  make(map[string]*LifecycleRule),
		tasks:  make(map[string]*MigrationTask),
		stats:  make(map[StorageTier]*StorageStats),
		config: config,
	}
}

// AddRule 添加生命周期规则
func (s *Service) AddRule(ctx context.Context, rule *LifecycleRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	
	s.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新规则
func (s *Service) UpdateRule(ctx context.Context, rule *LifecycleRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.rules[rule.ID]; !exists {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}
	
	rule.UpdatedAt = time.Now()
	s.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除规则
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.rules[id]; !exists {
		return fmt.Errorf("rule not found: %s", id)
	}
	
	delete(s.rules, id)
	return nil
}

// GetRule 获取规则
func (s *Service) GetRule(ctx context.Context, id string) (*LifecycleRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	rule, exists := s.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	return rule, nil
}

// ListRules 列出所有规则
func (s *Service) ListRules(ctx context.Context) []*LifecycleRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	rules := make([]*LifecycleRule, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}
	return rules
}

// EvaluateRules 评估规则并生成迁移任务
func (s *Service) EvaluateRules(ctx context.Context) ([]*MigrationTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var newTasks []*MigrationTask
	
	for _, rule := range s.rules {
		if !rule.Enabled {
			continue
		}
		
		// 扫描匹配文件
		// 评估是否需要迁移
		// 生成任务
		_ = rule
	}
	
	return newTasks, nil
}

// ExecuteMigration 执行迁移任务
func (s *Service) ExecuteMigration(ctx context.Context, taskID string) error {
	s.mu.Lock()
	task, exists := s.tasks[taskID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	if task.Status != TaskPending {
		s.mu.Unlock()
		return fmt.Errorf("task not in pending status: %s", task.Status)
	}
	
	task.Status = TaskRunning
	now := time.Now()
	task.StartedAt = &now
	s.mu.Unlock()
	
	// 执行迁移逻辑
	// 更新进度
	
	return nil
}

// CancelMigration 取消迁移
func (s *Service) CancelMigration(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	if task.Status != TaskPending && task.Status != TaskRunning {
		return fmt.Errorf("cannot cancel task in status: %s", task.Status)
	}
	
	task.Status = TaskCancelled
	return nil
}

// GetTask 获取任务状态
func (s *Service) GetTask(ctx context.Context, taskID string) (*MigrationTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// ListTasks 列出任务
func (s *Service) ListTasks(ctx context.Context, status TaskStatus) []*MigrationTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	tasks := make([]*MigrationTask, 0)
	for _, task := range s.tasks {
		if status == "" || task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GenerateCostReport 生成成本报告
func (s *Service) GenerateCostReport(ctx context.Context, period string) (*CostReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	report := &CostReport{
		GeneratedAt: time.Now(),
		Period:      period,
		TierStats:   make([]StorageStats, 0),
	}
	
	// 统计各层级使用情况
	var totalCost float64
	for tier, stat := range s.stats {
		cost := float64(stat.UsedSize) / (1024 * 1024 * 1024) * stat.CostPerGB
		totalCost += cost
		
		report.TierStats = append(report.TierStats, StorageStats{
			Tier:      tier,
			TotalSize: stat.TotalSize,
			UsedSize:  stat.UsedSize,
			FileCount: stat.FileCount,
			CostPerGB: stat.CostPerGB,
		})
	}
	
	report.TotalCost = totalCost
	
	// 计算优化建议
	report.Recommendations = s.generateRecommendations()
	
	return report, nil
}

// UpdateStorageStats 更新存储统计
func (s *Service) UpdateStorageStats(ctx context.Context, tier StorageTier, stats *StorageStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.stats[tier] = stats
}

// GetOptimizationSuggestions 获取优化建议
func (s *Service) GetOptimizationSuggestions(ctx context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.generateRecommendations()
}

func (s *Service) generateRecommendations() []string {
	recommendations := make([]string, 0)
	
	// 分析存储使用情况，生成建议
	hotStats := s.stats[TierHot]
	if hotStats != nil && hotStats.UsedSize > 0 {
		// 检查是否有大量冷数据在热存储
		ratio := float64(hotStats.UsedSize) / float64(hotStats.TotalSize)
		if ratio > 0.8 {
			recommendations = append(recommendations, 
				"热存储使用率超过80%，建议迁移不常用数据到温存储层")
		}
	}
	
	return recommendations
}

func generateRuleID() string {
	return fmt.Sprintf("rule_%d", time.Now().UnixNano())
}
