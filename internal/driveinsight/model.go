// Package driveinsight 提供智能存储分层与存储分析功能。
//
// 本模块参考群晖 DSM 7.3 的智能存储分层 (Smarter Tiering) 和存储分析功能：
//   - 智能分层：根据数据年龄和访问模式自动将冷数据迁移至高性价比存储层
//   - 自定义规则：基于年龄、访问频率等条件配置分层策略
//   - 全局存储视图：统一管理分层计划、监控容量、跟踪系统健康状态
//
// 核心组件：
//   - Collector：采集磁盘统计、计算每 TB 成本、温度趋势
//   - TieringEngine：基于年龄和访问模式的自动分层规则引擎
//   - Dashboard：聚合所有存储视图，生成容量预警和健康摘要
package driveinsight

import (
	"time"
)

// ========== 磁盘统计 ==========

// DriveStats 磁盘统计数据。
type DriveStats struct {
	SerialNumber string    `json:"serial_number"` // 序列号
	Model        string    `json:"model"`        // 型号
	DevicePath   string    `json:"device_path"`  // 设备路径，如 /dev/sda
	Interface    string    `json:"interface"`    // 接口类型：SATA/SAS/NVMe/USB
	Type         DriveType `json:"type"`         // 磁盘类型：SSD/HDD/NVMe
	CapacityBytes int64    `json:"capacity_bytes"` // 总容量（字节）
	UsedBytes     int64    `json:"used_bytes"`     // 已用容量（字节）
	FreeBytes     int64    `json:"free_bytes"`     // 可用容量（字节）
	HealthStatus  HealthStatus `json:"health_status"` // 健康状态
	TemperatureC  float64   `json:"temperature_c"`   // 当前温度（摄氏度）
	PowerOnHours  int64     `json:"power_on_hours"`  // 累计通电时间（小时）
	IOPS          float64   `json:"iops"`           // 当前 IOPS
	ThroughputMBps float64  `json:"throughput_mbps"` // 吞吐量（MB/s）
	LastUpdated   time.Time `json:"last_updated"`   // 最后更新时间
}

// DriveType 磁盘类型。
type DriveType string

const (
	DriveTypeHDD  DriveType = "HDD"   // 机械硬盘
	DriveTypeSSD  DriveType = "SSD"   // SATA 固态硬盘
	DriveTypeNVMe DriveType = "NVMe"  // NVMe 固态硬盘
	DriveTypeHybrid DriveType = "Hybrid" // 混合硬盘
)

// HealthStatus 健康状态。
type HealthStatus string

const (
	HealthGood    HealthStatus = "good"     // 健康
	HealthWarning HealthStatus = "warning"  // 警告
	HealthCritical HealthStatus = "critical" // 严重
	HealthUnknown HealthStatus = "unknown"  // 未知
)

// TemperatureTrend 温度趋势记录。
type TemperatureTrend struct {
	SerialNumber string          `json:"serial_number"`
	Readings     []TempReading   `json:"readings"` // 温度读数序列
	MinTemp      float64         `json:"min_temp"`
	MaxTemp      float64         `json:"max_temp"`
	AvgTemp      float64         `json:"avg_temp"`
	Trend        TempTrendDirection `json:"trend"` // 趋势方向
}

// TempReading 单次温度读数。
type TempReading struct {
	Timestamp  time.Time `json:"timestamp"`
	TemperatureC float64 `json:"temperature_c"`
}

// TempTrendDirection 温度趋势方向。
type TempTrendDirection string

const (
	TempTrendRising  TempTrendDirection = "rising"  // 上升
	TempTrendFalling TempTrendDirection = "falling" // 下降
	TempTrendStable  TempTrendDirection = "stable"  // 平稳
)

// ========== 存储分层 ==========

// StorageTier 存储分层定义。
type StorageTier struct {
	Name        string    `json:"name"`          // 层名称，如 "NVMe 热数据层"
	Type        TierType  `json:"type"`         // 层类型：SSD/HDD/混合
	CapacityBytes int64   `json:"capacity_bytes"` // 总容量（字节）
	UsedBytes   int64     `json:"used_bytes"`     // 已用容量（字节）
	FreeBytes   int64     `json:"free_bytes"`     // 可用容量（字节")
	MonthlyCost float64   `json:"monthly_cost"`   // 月成本（元）
	CostPerTB   float64   `json:"cost_per_tb"`    // 每 TB 月成本（元）
	DriveSerials []string `json:"drive_serials"`  // 所属磁盘序列号列表
	Policy      string    `json:"policy"`         // 分层策略名称
}

// TierType 存储层类型。
type TierType string

const (
	TierTypeSSD    TierType = "SSD"    // 固态硬盘层
	TierTypeHDD    TierType = "HDD"    // 机械硬盘层
	TierTypeHybrid TierType = "Hybrid" // 混合层（SSD 缓存 + HDD 容量）
	TierTypeNVMe   TierType = "NVMe"   // NVMe 高性能层
	TierTypeCloud  TierType = "Cloud"  // 云存储层
	TierTypeArchive TierType = "Archive" // 归档层
)

// TierID 分层标识符。
type TierID string

const (
	TierIDHot     TierID = "hot"     // 热数据层（NVMe/SSD）
	TierIDWarm    TierID = "warm"    // 温数据层（SSD）
	TierIDCold    TierID = "cold"    // 冷数据层（HDD）
	TierIDArchive TierID = "archive" // 归档层
)

// ========== 成本报告 ==========

// CostReport 成本分析报告。
type CostReport struct {
	GeneratedAt      time.Time      `json:"generated_at"`
	TotalCapacityTB  float64        `json:"total_capacity_tb"`
	TotalUsedTB      float64        `json:"total_used_tb"`
	TotalMonthlyCost float64        `json:"total_monthly_cost"`
	TotalYearlyCost  float64        `json:"total_yearly_cost"`
	AvgCostPerTB     float64        `json:"avg_cost_per_tb"`     // 加权平均每 TB 月成本
	TierCosts        []TierCostItem `json:"tier_costs"`         // 各层成本明细
	PotentialSavings float64        `json:"potential_savings"`  // 通过分层可节省的月成本
	SavingsPercent   float64        `json:"savings_percent"`    // 节省百分比
}

// TierCostItem 单层成本明细。
type TierCostItem struct {
	TierName    string  `json:"tier_name"`
	TierType    TierType `json:"tier_type"`
	CapacityTB  float64 `json:"capacity_tb"`
	UsedTB      float64 `json:"used_tb"`
	CostPerTB   float64 `json:"cost_per_tb"`
	MonthlyCost float64 `json:"monthly_cost"`
	YearlyCost  float64 `json:"yearly_cost"`
}

// ========== 文件访问模式 ==========

// FileAccessPattern 文件访问模式（用于冷热数据识别）。
type FileAccessPattern struct {
	Path          string    `json:"path"`
	Size          int64     `json:"size"`           // 文件大小（字节")
	ModTime       time.Time `json:"mod_time"`       // 最后修改时间
	AccessTime    time.Time `json:"access_time"`    // 最后访问时间
	AccessCount   int       `json:"access_count"`   // 访问次数
	AccessFreq    AccessFreq `json:"access_freq"`   // 访问频率分类
	DataTier      TierID    `json:"data_tier"`      // 建议分层
	LastIOTime    time.Time `json:"last_io_time"`   // 最后 I/O 时间
}

// AccessFreq 访问频率分类。
type AccessFreq string

const (
	AccessFreqHot  AccessFreq = "hot"  // 热数据：7天内访问
	AccessFreqWarm AccessFreq = "warm" // 温数据：7-30天有访问
	AccessFreqCold AccessFreq = "cold" // 冷数据：30-90天无访问
	AccessFreqFrozen AccessFreq = "frozen" // 冰冷数据：90天以上无访问
)

// ========== 分层规则 ==========

// TieringRule 分层规则定义。
type TieringRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Enabled     bool       `json:"enabled"`
	Priority    int        `json:"priority"`     // 优先级，数字越小优先级越高
	Conditions  []RuleCondition `json:"conditions"`
	TargetTier  TierID     `json:"target_tier"`  // 目标分层
	Action      RuleAction `json:"action"`       // 迁移动作
	Description string     `json:"description"`
}

// RuleCondition 规则条件。
type RuleCondition struct {
	Field    RuleField    `json:"field"`    // 条件字段
	Operator RuleOperator `json:"operator"` // 操作符
	Value    string       `json:"value"`    // 比较值
}

// RuleField 规则条件字段。
type RuleField string

const (
	RuleFieldAge        RuleField = "age"         // 文件年龄（天）
	RuleFieldAccessCount RuleField = "access_count" // 访问次数
	RuleFieldLastAccess  RuleField = "last_access" // 最后访问距今天数
	RuleFieldSize        RuleField = "size"        // 文件大小（MB）
	RuleFileType         RuleField = "file_type"   // 文件类型
)

// RuleOperator 规则操作符。
type RuleOperator string

const (
	OpGreaterThan  RuleOperator = ">"
	OpLessThan     RuleOperator = "<"
	OpEqual        RuleOperator = "="
	OpGreaterEqual RuleOperator = ">="
	OpLessEqual    RuleOperator = "<="
	OpContains     RuleOperator = "contains"
)

// RuleAction 规则动作。
type RuleAction string

const (
	ActionMigrate RuleAction = "migrate" // 迁移到目标层
	ActionPin     RuleAction = "pin"     // 固定在当前层
	ActionArchive RuleAction = "archive" // 归档
	ActionNotify  RuleAction = "notify"  // 仅通知
)

// TieringPlan 分层计划。
type TieringPlan struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	Rules       []TieringRule `json:"rules"`
	SourceTier  TierID        `json:"source_tier"`
	TargetTier  TierID        `json:"target_tier"`
	Schedule    string        `json:"schedule"` // cron 表达式
	LastRun     time.Time     `json:"last_run"`
	NextRun     time.Time     `json:"next_run"`
}

// ========== 仪表盘 ==========

// DashboardData 仪表盘聚合数据。
type DashboardData struct {
	GeneratedAt      time.Time         `json:"generated_at"`
	TotalCapacity    int64             `json:"total_capacity"`     // 总容量（字节）
	TotalUsed        int64             `json:"total_used"`         // 总已用（字节）
	TotalFree        int64             `json:"total_free"`         // 总可用（字节")
	UsagePercent     float64           `json:"usage_percent"`      // 使用率
	Tiers            []StorageTier     `json:"tiers"`              // 各存储层
	Drives           []DriveStats      `json:"drives"`             // 所有磁盘
	CostReport       *CostReport       `json:"cost_report"`        // 成本报告
	CapacityAlerts   []CapacityAlert   `json:"capacity_alerts"`    // 容量预警
	HealthSummary    HealthSummary     `json:"health_summary"`     // 健康摘要
	TieringPlans     []TieringPlan     `json:"tiering_plans"`      // 分层计划
	MigrationPending int               `json:"migration_pending"`  // 待迁移文件数
}

// CapacityAlert 容量预警。
type CapacityAlert struct {
	TierName    string  `json:"tier_name"`
	DriveSerial string  `json:"drive_serial"`
	UsagePercent float64 `json:"usage_percent"`
	Threshold    float64 `json:"threshold"`
	Level        AlertLevel `json:"level"`
	Message      string  `json:"message"`
}

// AlertLevel 预警级别。
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// HealthSummary 健康摘要。
type HealthSummary struct {
	TotalDrives    int           `json:"total_drives"`
	HealthyDrives  int           `json:"healthy_drives"`
	WarningDrives  int           `json:"warning_drives"`
	CriticalDrives int           `json:"critical_drives"`
	AvgTempC       float64       `json:"avg_temp_c"`
	MaxTempC       float64       `json:"max_temp_c"`
	Overall        HealthStatus  `json:"overall"`
	Details        []DriveHealth `json:"details"`
}

// DriveHealth 单盘健康详情。
type DriveHealth struct {
	SerialNumber string       `json:"serial_number"`
	Model        string       `json:"model"`
	Status       HealthStatus `json:"status"`
	TemperatureC float64      `json:"temperature_c"`
	PowerOnHours int64        `json:"power_on_hours"`
	Message      string       `json:"message"`
}
