// Package storagetiering - 智能存储分层引擎
// SSD/HDD/Cold 三级存储自动分层
// 基于访问频率、文件大小、文件类型智能判定数据温度
// 参考群晖存储管理器和 TrueNAS 自动分层功能
package storagetiering

import (
	"fmt"
	"time"
)

// ============================================================
// 存储层级定义 (SSD / HDD / Cold 三级)
// ============================================================

// Tier 存储层级
type Tier int

const (
	TierSSD  Tier = iota // 热数据层 - 高速 SSD
	TierHDD              // 温数据层 - 机械硬盘
	TierCold             // 冷数据层 - 归档/低成本存储
)

// String 返回层级名称
func (t Tier) String() string {
	switch t {
	case TierSSD:
		return "ssd"
	case TierHDD:
		return "hdd"
	case TierCold:
		return "cold"
	default:
		return "unknown"
	}
}

// ParseTier 解析层级名称
func ParseTier(s string) Tier {
	switch s {
	case "ssd":
		return TierSSD
	case "hdd":
		return TierHDD
	case "cold":
		return TierCold
	default:
		return TierHDD
	}
}

// ============================================================
// 数据温度
// ============================================================

// DataTemperature 数据温度
type DataTemperature int

const (
	TempHot  DataTemperature = iota // 热数据
	TempWarm                        // 温数据
	TempCold                        // 冷数据
)

// String 返回温度名称
func (dt DataTemperature) String() string {
	switch dt {
	case TempHot:
		return "hot"
	case TempWarm:
		return "warm"
	case TempCold:
		return "cold"
	default:
		return "unknown"
	}
}

// ============================================================
// 迁移任务状态
// ============================================================

// MigrationState 迁移任务状态
type MigrationState int

const (
	StatePending    MigrationState = iota // 等待执行
	StateRunning                          // 执行中
	StatePaused                           // 已暂停
	StateCompleted                        // 已完成
	StateFailed                           // 失败
	StateCancelled                        // 已取消
)

// String 返回状态名称
func (s MigrationState) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// ============================================================
// 文件元数据
// ============================================================

// FileEntry 文件条目
type FileEntry struct {
	Path        string          `json:"path"`         // 文件路径
	Size        int64           `json:"size"`         // 文件大小 (bytes)
	CurrentTier Tier            `json:"current_tier"` // 当前所在层级
	Temperature DataTemperature `json:"temperature"`  // 数据温度
	HeatScore   float64         `json:"heat_score"`   // 热度评分 (0-100)
	ContentType string          `json:"content_type"` // MIME 类型
	Checksum    string          `json:"checksum"`     // CRC32 校验和
	CreatedAt   time.Time       `json:"created_at"`   // 创建时间
	ModifiedAt  time.Time       `json:"modified_at"`  // 最后修改时间
	AccessedAt  time.Time       `json:"accessed_at"`  // 最后访问时间
	AccessCount int64           `json:"access_count"` // 总访问次数
	ReadCount   int64           `json:"read_count"`   // 读取次数
	WriteCount  int64           `json:"write_count"`  // 写入次数
	Tags        []string        `json:"tags,omitempty"`
}

// AccessRecord 访问记录
type AccessRecord struct {
	Path      string    `json:"path"`
	OpType    string    `json:"op_type"` // "read", "write"
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================
// 迁移任务
// ============================================================

// MigrationTask 迁移任务
type MigrationTask struct {
	ID          string        `json:"id"`
	FilePath    string        `json:"file_path"`
	FromTier    Tier          `json:"from_tier"`
	ToTier      Tier          `json:"to_tier"`
	FileSize    int64         `json:"file_size"`
	State       MigrationState `json:"state"`
	Reason      string        `json:"reason"`
	ChecksumSrc string        `json:"checksum_src"`  // 源文件校验和
	ChecksumDst string        `json:"checksum_dst"`  // 目标文件校验和
	Progress    float64       `json:"progress"`      // 进度 0-100
	Error       string        `json:"error,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// MigrationHistoryItem 迁移历史记录
type MigrationHistoryItem struct {
	TaskID    string        `json:"task_id"`
	FilePath  string        `json:"file_path"`
	FromTier  Tier          `json:"from_tier"`
	ToTier    Tier          `json:"to_tier"`
	FileSize  int64         `json:"file_size"`
	State     MigrationState `json:"state"`
	Reason    string        `json:"reason"`
	Timestamp time.Time     `json:"timestamp"`
}

// ============================================================
// 统计信息
// ============================================================

// TierStats 单层统计
type TierStats struct {
	Tier        Tier    `json:"tier"`
	TotalBytes  int64   `json:"total_bytes"`  // 总容量
	UsedBytes   int64   `json:"used_bytes"`   // 已使用
	FreeBytes   int64   `json:"free_bytes"`   // 可用
	FileCount   int     `json:"file_count"`   // 文件数量
	UsageRatio  float64 `json:"usage_ratio"`  // 使用率 (0-1)
}

// Stats 总体统计
type Stats struct {
	Tiers           []TierStats           `json:"tiers"`
	TotalMigrations int64                 `json:"total_migrations"`
	ActiveMigrations int                  `json:"active_migrations"`
	HitRate         float64               `json:"hit_rate"`          // 分层命中率
	RecentHistory   []MigrationHistoryItem `json:"recent_history"`   // 最近迁移历史
	LastAnalysis    *time.Time            `json:"last_analysis,omitempty"`
}

// ============================================================
// 配置
// ============================================================

// TierCapacity 层级容量配置
type TierCapacity struct {
	Tier       Tier  `json:"tier"`
	TotalBytes int64 `json:"total_bytes"` // 总容量
}

// Thresholds 温度阈值配置
type Thresholds struct {
	HotMinScore  float64 `json:"hot_min_score"`  // 热数据最低分 (默认 70)
	WarmMinScore float64 `json:"warm_min_score"` // 温数据最低分 (默认 30)
	// 低于 WarmMinScore 的为冷数据
}

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	AnalysisInterval  time.Duration `json:"analysis_interval"`   // 分析间隔
	HistoryWindowDays int           `json:"history_window_days"` // 访问历史窗口天数
	MinAccessCount    int64         `json:"min_access_count"`    // 最小访问次数阈值
}

// MigratorConfig 迁移器配置
type MigratorConfig struct {
	MaxConcurrent int   `json:"max_concurrent"` // 最大并发迁移数
	BatchSize     int   `json:"batch_size"`     // 批量大小
	VerifyChecksum bool `json:"verify_checksum"` // 迁移前后校验
	MaxFileSize   int64 `json:"max_file_size"`  // 单文件最大迁移大小
}

// PolicyConfig 策略配置
type PolicyConfig struct {
	Thresholds       Thresholds     `json:"thresholds"`
	TierCapacities   []TierCapacity `json:"tier_capacities"`
	CapacityHighPct  float64        `json:"capacity_high_pct"` // 触发迁移的容量阈值 (默认 0.85)
	CapacityLowPct   float64        `json:"capacity_low_pct"`  // 迁移目标容量阈值 (默认 0.70)
	FileTypeBoosts   map[string]float64 `json:"file_type_boosts"` // 文件类型热度加成
	LargeFilePenalty float64        `json:"large_file_penalty"` // 大文件热度惩罚系数
}

// Config 总配置
type Config struct {
	Tiers      []TierCapacity  `json:"tiers"`
	Thresholds Thresholds      `json:"thresholds"`
	Analyzer   AnalyzerConfig  `json:"analyzer"`
	Migrator   MigratorConfig  `json:"migrator"`
	Policy     PolicyConfig    `json:"policy"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Tiers: []TierCapacity{
			{Tier: TierSSD, TotalBytes: 500 * 1024 * 1024 * 1024},   // 500GB SSD
			{Tier: TierHDD, TotalBytes: 4 * 1024 * 1024 * 1024 * 1024}, // 4TB HDD
			{Tier: TierCold, TotalBytes: 8 * 1024 * 1024 * 1024 * 1024}, // 8TB Cold
		},
		Thresholds: Thresholds{
			HotMinScore:  70.0,
			WarmMinScore: 30.0,
		},
		Analyzer: AnalyzerConfig{
			AnalysisInterval:  30 * time.Minute,
			HistoryWindowDays: 30,
			MinAccessCount:    3,
		},
		Migrator: MigratorConfig{
			MaxConcurrent:  4,
			BatchSize:      50,
			VerifyChecksum: true,
			MaxFileSize:    10 * 1024 * 1024 * 1024, // 10GB
		},
		Policy: PolicyConfig{
			Thresholds: Thresholds{
				HotMinScore:  70.0,
				WarmMinScore: 30.0,
			},
			CapacityHighPct: 0.85,
			CapacityLowPct:  0.70,
			FileTypeBoosts: map[string]float64{
				".db":    20.0, // 数据库文件
				".sql":   15.0,
				".iso":   -10.0, // ISO 文件倾向冷层
				".log":   -5.0,  // 日志倾向冷层
				".tar":   -10.0,
				".gz":    -10.0,
				".mp4":   -5.0,
				".bak":   -15.0, // 备份文件
			},
			LargeFilePenalty: 0.3, // 大于 1GB 的文件热度惩罚
		},
	}
}

// Validate 校验配置
func (c Config) Validate() error {
	if len(c.Tiers) == 0 {
		return fmt.Errorf("at least one tier must be configured")
	}
	for _, t := range c.Tiers {
		if t.TotalBytes <= 0 {
			return fmt.Errorf("tier %s capacity must be positive", t.Tier)
		}
	}
	if c.Thresholds.HotMinScore <= c.Thresholds.WarmMinScore {
		return fmt.Errorf("hot_min_score (%v) must be greater than warm_min_score (%v)",
			c.Thresholds.HotMinScore, c.Thresholds.WarmMinScore)
	}
	if c.Migrator.MaxConcurrent <= 0 {
		return fmt.Errorf("max_concurrent must be positive")
	}
	if c.Policy.CapacityHighPct <= c.Policy.CapacityLowPct {
		return fmt.Errorf("capacity_high_pct must be greater than capacity_low_pct")
	}
	return nil
}
