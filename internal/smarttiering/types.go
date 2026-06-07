// Package smarttiering - 智能数据分层模块
// AI驱动的数据热度预测、自动迁移、成本优化、性能监控
// 参考群晖 DSM 7.3 智能分层功能
package smarttiering

import (
	"time"
)

// ============================================================
// 存储层级定义
// ============================================================

// StorageTier 存储层级
type StorageTier int

const (
	TierHot     StorageTier = iota // 热数据层 (NVMe SSD)
	TierWarm                       // 温数据层 (SATA SSD)
	TierCold                       // 冷数据层 (HDD)
	TierArchive                    // 归档层 (低成本存储)
)

// String 返回层级名称
func (t StorageTier) String() string {
	switch t {
	case TierHot:
		return "hot"
	case TierWarm:
		return "warm"
	case TierCold:
		return "cold"
	case TierArchive:
		return "archive"
	default:
		return "unknown"
	}
}

// ParseTier 解析层级名称
func ParseTier(s string) StorageTier {
	switch s {
	case "hot":
		return TierHot
	case "warm":
		return TierWarm
	case "cold":
		return TierCold
	case "archive":
		return TierArchive
	default:
		return TierCold
	}
}

// ============================================================
// 数据文件元数据
// ============================================================

// FileMetadata 文件元数据
type FileMetadata struct {
	Path        string      `json:"path"`           // 文件路径
	Size        int64       `json:"size"`           // 文件大小 (bytes)
	CurrentTier StorageTier `json:"current_tier"`   // 当前所在层级
	CreatedAt   time.Time   `json:"created_at"`     // 创建时间
	ModifiedAt  time.Time   `json:"modified_at"`    // 最后修改时间
	AccessedAt  time.Time   `json:"accessed_at"`    // 最后访问时间
	AccessCount int64       `json:"access_count"`   // 访问次数
	ReadCount   int64       `json:"read_count"`     // 读取次数
	WriteCount  int64       `json:"write_count"`    // 写入次数
	HeatScore   float64     `json:"heat_score"`     // 热度评分 (0-100)
	Tags        []string    `json:"tags,omitempty"` // 用户标签
	ContentType string      `json:"content_type"`   // 内容类型
}

// AccessRecord 访问记录
type AccessRecord struct {
	Path      string    `json:"path"`
	OpType    string    `json:"op_type"` // "read", "write", "delete"
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================
// 热度预测配置
// ============================================================

// PredictorConfig 热度预测器配置
type PredictorConfig struct {
	Enabled           bool    `json:"enabled"`
	UpdateIntervalSec int     `json:"update_interval_sec"` // 更新间隔 (秒), 默认 300
	HistoryWindowDays int     `json:"history_window_days"` // 历史窗口 (天), 默认 30
	DecayFactor       float64 `json:"decay_factor"`        // 时间衰减因子, 默认 0.95
	WeightRecency     float64 `json:"weight_recency"`      // 最近访问权重, 默认 0.4
	WeightFrequency   float64 `json:"weight_frequency"`    // 访问频率权重, 默认 0.3
	WeightSize        float64 `json:"weight_size"`         // 文件大小权重, 默认 0.1
	WeightPattern     float64 `json:"weight_pattern"`      // 访问模式权重, 默认 0.2
}

// DefaultPredictorConfig 默认预测器配置
func DefaultPredictorConfig() PredictorConfig {
	return PredictorConfig{
		Enabled:           true,
		UpdateIntervalSec: 300,
		HistoryWindowDays: 30,
		DecayFactor:       0.95,
		WeightRecency:     0.4,
		WeightFrequency:   0.3,
		WeightSize:        0.1,
		WeightPattern:     0.2,
	}
}

// ============================================================
// 迁移配置
// ============================================================

// MigratorConfig 迁移引擎配置
type MigratorConfig struct {
	Enabled              bool    `json:"enabled"`
	BatchSize            int     `json:"batch_size"`             // 批量迁移大小, 默认 100
	MaxConcurrent        int     `json:"max_concurrent"`         // 最大并发迁移数, 默认 4
	MigrationIntervalSec int     `json:"migration_interval_sec"` // 迁移检查间隔 (秒), 默认 600
	MinIdleHours         float64 `json:"min_idle_hours"`         // 最小空闲时间 (小时), 默认 2.0
	HotThreshold         float64 `json:"hot_threshold"`          // 热数据阈值, 默认 70
	WarmThreshold        float64 `json:"warm_threshold"`         // 温数据阈值, 默认 40
	ColdThreshold        float64 `json:"cold_threshold"`         // 冷数据阈值, 默认 15
	DryRun               bool    `json:"dry_run"`                // 试运行模式
}

// DefaultMigratorConfig 默认迁移配置
func DefaultMigratorConfig() MigratorConfig {
	return MigratorConfig{
		Enabled:              true,
		BatchSize:            100,
		MaxConcurrent:        4,
		MigrationIntervalSec: 600,
		MinIdleHours:         2.0,
		HotThreshold:         70,
		WarmThreshold:        40,
		ColdThreshold:        15,
		DryRun:               false,
	}
}

// ============================================================
// 成本优化
// ============================================================

// TierCostProfile 层级成本配置
type TierCostProfile struct {
	Tier           StorageTier `json:"tier"`
	CostPerGBMonth float64     `json:"cost_per_gb_month"`    // 每GB每月成本 (元)
	ReadIOPS       float64     `json:"read_iops"`            // 读IOPS
	WriteIOPS      float64     `json:"write_iops"`           // 写IOPS
	ReadBandwidth  float64     `json:"read_bandwidth_mbps"`  // 读带宽 (MB/s)
	WriteBandwidth float64     `json:"write_bandwidth_mbps"` // 写带宽 (MB/s)
	LatencyMs      float64     `json:"latency_ms"`           // 平均延迟 (ms)
}

// DefaultTierCosts 默认层级成本
func DefaultTierCosts() []TierCostProfile {
	return []TierCostProfile{
		{Tier: TierHot, CostPerGBMonth: 2.0, ReadIOPS: 100000, WriteIOPS: 80000, ReadBandwidth: 7000, WriteBandwidth: 5000, LatencyMs: 0.1},
		{Tier: TierWarm, CostPerGBMonth: 0.8, ReadIOPS: 50000, WriteIOPS: 30000, ReadBandwidth: 3000, WriteBandwidth: 2000, LatencyMs: 0.3},
		{Tier: TierCold, CostPerGBMonth: 0.2, ReadIOPS: 5000, WriteIOPS: 3000, ReadBandwidth: 500, WriteBandwidth: 300, LatencyMs: 5.0},
		{Tier: TierArchive, CostPerGBMonth: 0.05, ReadIOPS: 500, WriteIOPS: 200, ReadBandwidth: 100, WriteBandwidth: 50, LatencyMs: 20.0},
	}
}

// CostOptimizerConfig 成本优化器配置
type CostOptimizerConfig struct {
	Enabled             bool    `json:"enabled"`
	BudgetPerMonth      float64 `json:"budget_per_month"`      // 月度预算 (元)
	OptimizeIntervalSec int     `json:"optimize_interval_sec"` // 优化检查间隔 (秒), 默认 3600
	CostWeight          float64 `json:"cost_weight"`           // 成本权重, 默认 0.6
	PerfWeight          float64 `json:"perf_weight"`           // 性能权重, 默认 0.4
}

// DefaultCostOptimizerConfig 默认成本优化配置
func DefaultCostOptimizerConfig() CostOptimizerConfig {
	return CostOptimizerConfig{
		Enabled:             true,
		BudgetPerMonth:      1000,
		OptimizeIntervalSec: 3600,
		CostWeight:          0.6,
		PerfWeight:          0.4,
	}
}

// CostReport 成本报告
type CostReport struct {
	TotalCostPerMonth   float64                 `json:"total_cost_per_month"`
	TierCosts           map[StorageTier]float64 `json:"tier_costs"`
	TierSizesGB         map[StorageTier]float64 `json:"tier_sizes_gb"`
	SavingsPercent      float64                 `json:"savings_percent"`
	OptimalCostPerMonth float64                 `json:"optimal_cost_per_month"`
	Recommendations     []CostRecommendation    `json:"recommendations"`
	GeneratedAt         time.Time               `json:"generated_at"`
}

// CostRecommendation 成本优化建议
type CostRecommendation struct {
	FilePath        string      `json:"file_path"`
	CurrentTier     StorageTier `json:"current_tier"`
	RecommendedTier StorageTier `json:"recommended_tier"`
	CurrentCost     float64     `json:"current_cost"`
	RecommendedCost float64     `json:"recommended_cost"`
	Savings         float64     `json:"savings"`
	Reason          string      `json:"reason"`
}

// ============================================================
// 监控指标
// ============================================================

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled            bool `json:"enabled"`
	MetricsIntervalSec int  `json:"metrics_interval_sec"` // 指标采集间隔 (秒), 默认 60
	RetentionDays      int  `json:"retention_days"`       // 指标保留天数, 默认 30
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		Enabled:            true,
		MetricsIntervalSec: 60,
		RetentionDays:      30,
	}
}

// TieringMetrics 分层指标
type TieringMetrics struct {
	Timestamp        time.Time               `json:"timestamp"`
	TierDistribution map[StorageTier]int64   `json:"tier_distribution"`  // 各层级文件数
	TierSizesGB      map[StorageTier]float64 `json:"tier_sizes_gb"`      // 各层级大小 (GB)
	MigrationCount   int64                   `json:"migration_count"`    // 迁移次数
	MigrationBytesGB float64                 `json:"migration_bytes_gb"` // 迁移数据量 (GB)
	AvgHeatScores    map[StorageTier]float64 `json:"avg_heat_scores"`    // 各层级平均热度
	HitRates         map[StorageTier]float64 `json:"hit_rates"`          // 各层级命中率
	TotalFiles       int64                   `json:"total_files"`
	TotalSizeGB      float64                 `json:"total_size_gb"`
}

// MigrationEvent 迁移事件
type MigrationEvent struct {
	ID          string        `json:"id"`
	FilePath    string        `json:"file_path"`
	FromTier    StorageTier   `json:"from_tier"`
	ToTier      StorageTier   `json:"to_tier"`
	FileSize    int64         `json:"file_size"`
	Reason      string        `json:"reason"`
	Status      string        `json:"status"` // "pending", "running", "completed", "failed"
	Error       string        `json:"error,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// ============================================================
// 分层管理器配置
// ============================================================

// Config 智能分层总配置
type Config struct {
	Predictor     PredictorConfig     `json:"predictor"`
	Migrator      MigratorConfig      `json:"migrator"`
	CostOptimizer CostOptimizerConfig `json:"cost_optimizer"`
	Monitor       MonitorConfig       `json:"monitor"`
	TierCosts     []TierCostProfile   `json:"tier_costs"`
}

// DefaultConfig 默认总配置
func DefaultConfig() Config {
	return Config{
		Predictor:     DefaultPredictorConfig(),
		Migrator:      DefaultMigratorConfig(),
		CostOptimizer: DefaultCostOptimizerConfig(),
		Monitor:       DefaultMonitorConfig(),
		TierCosts:     DefaultTierCosts(),
	}
}

// ============================================================
// 新增类型：TierPolicy, TierRule, DataPlacement, TierStats, MigrationJob, AccessPattern
// ============================================================

// TierPolicy 分层策略
type TierPolicy struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Enabled     bool       `json:"enabled"`
	Priority    int        `json:"priority"`
	Rules       []TierRule `json:"rules"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TierRule 分层规则
type TierRule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	TargetTier StorageTier     `json:"target_tier"`
	Conditions []TierCondition `json:"conditions"`
	Action     string          `json:"action"` // promote, demote, pin
	Enabled    bool            `json:"enabled"`
}

// TierCondition 分层条件
type TierCondition struct {
	Field    string `json:"field"`    // heat_score, file_size, access_count, days_since_access, content_type
	Operator string `json:"operator"` // gt, lt, gte, lte, eq, in, contains
	Value    string `json:"value"`
}

// DataPlacement 数据放置信息
type DataPlacement struct {
	FilePath        string      `json:"file_path"`
	CurrentTier     StorageTier `json:"current_tier"`
	RecommendedTier StorageTier `json:"recommended_tier"`
	HeatScore       float64     `json:"heat_score"`
	FileSize        int64       `json:"file_size"`
	LastAccess      time.Time   `json:"last_access"`
	AccessCount     int64       `json:"access_count"`
	Reason          string      `json:"reason"`
	Confidence      float64     `json:"confidence"`
}

// TierStats 分层统计
type TierStats struct {
	GeneratedAt      time.Time          `json:"generated_at"`
	TotalFiles       int64              `json:"total_files"`
	TotalSizeGB      float64            `json:"total_size_gb"`
	TierDistribution map[string]int64   `json:"tier_distribution"`
	TierSizesGB      map[string]float64 `json:"tier_sizes_gb"`
	AvgHeatScores    map[string]float64 `json:"avg_heat_scores"`
	HitRates         map[string]float64 `json:"hit_rates"`
	MigrationCount   int64              `json:"migration_count"`
	MigrationBytesGB float64            `json:"migration_bytes_gb"`
	PolicyCount      int                `json:"policy_count"`
	ActiveMigrations int                `json:"active_migrations"`
}

// MigrationJob 迁移任务
type MigrationJob struct {
	ID          string      `json:"id"`
	FilePath    string      `json:"file_path"`
	FromTier    StorageTier `json:"from_tier"`
	ToTier      StorageTier `json:"to_tier"`
	FileSize    int64       `json:"file_size"`
	Reason      string      `json:"reason"`
	Status      string      `json:"status"` // pending, running, completed, failed
	Progress    float64     `json:"progress"`
	Error       string      `json:"error,omitempty"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Duration    string      `json:"duration,omitempty"`
}

// AccessPattern 访问模式
type AccessPattern struct {
	FilePath          string      `json:"file_path"`
	TotalAccesses     int64       `json:"total_accesses"`
	ReadCount         int64       `json:"read_count"`
	WriteCount        int64       `json:"write_count"`
	LastAccess        time.Time   `json:"last_access"`
	AvgAccessInterval float64     `json:"avg_access_interval_hours"`
	PeakHour          int         `json:"peak_hour"`
	Pattern           string      `json:"pattern"` // burst, steady, periodic, cold
	HeatScore         float64     `json:"heat_score"`
	PredictedTier     StorageTier `json:"predicted_tier"`
}
