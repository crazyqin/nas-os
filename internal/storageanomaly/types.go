// Package storageanomaly - 存储异常检测模块
// 基于 AI 的存储异常检测，检测异常访问模式、潜在数据损坏、异常容量增长等
package storageanomaly

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// AnomalyConfig 异常检测配置
type AnomalyConfig struct {
	// 检测配置
	EnableAccessPattern  bool    `json:"enable_access_pattern"`  // 启用访问模式检测
	EnableDataCorruption bool    `json:"enable_data_corruption"` // 启用数据损坏检测
	EnableCapacityGrowth bool    `json:"enable_capacity_growth"` // 启用容量增长检测
	EnableIOPSAnomaly    bool    `json:"enable_iops_anomaly"`    // 启用IOPS异常检测
	EnableLatencyAnomaly bool    `json:"enable_latency_anomaly"` // 启用延迟异常检测

	// 阈值配置
	CapacityGrowthThreshold  float64 `json:"capacity_growth_threshold"`   // 容量增长阈值 (百分比/天), 默认 10%
	IOPSSpikeThreshold       float64 `json:"iops_spike_threshold"`        // IOPS峰值阈值 (倍数), 默认 3倍
	LatencySpikeThreshold    float64 `json:"latency_spike_threshold"`     // 延迟峰值阈值 (毫秒), 默认 100ms
	AccessPatternThreshold   float64 `json:"access_pattern_threshold"`    // 访问模式阈值

	// 采样配置
	SampleInterval   time.Duration `json:"sample_interval"`    // 采样间隔, 默认 5分钟
	HistoryWindow    time.Duration `json:"history_window"`     // 历史窗口, 默认 24小时
	MinSamples       int           `json:"min_samples"`        // 最小样本数, 默认 20

	// 告警配置
	AlertCooldown    time.Duration `json:"alert_cooldown"`     // 告警冷却时间, 默认 1小时
	MaxAlertsPerHour int           `json:"max_alerts_per_hour"` // 每小时最大告警数, 默认 10
}

// DefaultAnomalyConfig 默认异常检测配置
func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		EnableAccessPattern:  true,
		EnableDataCorruption: true,
		EnableCapacityGrowth: true,
		EnableIOPSAnomaly:    true,
		EnableLatencyAnomaly: true,
		CapacityGrowthThreshold:  10.0,
		IOPSSpikeThreshold:       3.0,
		LatencySpikeThreshold:    100.0,
		AccessPatternThreshold:   0.8,
		SampleInterval:           5 * time.Minute,
		HistoryWindow:            24 * time.Hour,
		MinSamples:               20,
		AlertCooldown:            1 * time.Hour,
		MaxAlertsPerHour:         10,
	}
}

// ============================================================
// 异常类型
// ============================================================

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyTypeAccessPattern  AnomalyType = "access_pattern"   // 访问模式异常
	AnomalyTypeDataCorruption AnomalyType = "data_corruption"  // 数据损坏
	AnomalyTypeCapacityGrowth AnomalyType = "capacity_growth"  // 容量增长异常
	AnomalyTypeIOPSSpike      AnomalyType = "iops_spike"       // IOPS峰值异常
	AnomalyTypeLatencySpike   AnomalyType = "latency_spike"    // 延迟峰值异常
	AnomalyTypeDiskFailure    AnomalyType = "disk_failure"     // 磁盘故障预测
)

// ============================================================
// 异常严重程度
// ============================================================

// AnomalySeverity 异常严重程度
type AnomalySeverity string

const (
	SeverityInfo     AnomalySeverity = "info"     // 信息
	SeverityWarning  AnomalySeverity = "warning"  // 警告
	SeverityCritical AnomalySeverity = "critical" // 严重
	SeverityFatal    AnomalySeverity = "fatal"    // 致命
)

// ============================================================
// 存储指标类型
// ============================================================

// StorageMetrics 存储指标
type StorageMetrics struct {
	DeviceID      string    `json:"device_id"`      // 设备ID
	MountPoint    string    `json:"mount_point"`    // 挂载点
	CollectedAt   time.Time `json:"collected_at"`   // 采集时间

	// 容量指标
	TotalSpace    uint64 `json:"total_space"`     // 总空间 (字节)
	UsedSpace     uint64 `json:"used_space"`      // 已用空间 (字节)
	FreeSpace     uint64 `json:"free_space"`      // 可用空间 (字节)
	UsagePercent  float64 `json:"usage_percent"`  // 使用率 (%)

	// 性能指标
	ReadIOPS      float64 `json:"read_iops"`      // 读IOPS
	WriteIOPS     float64 `json:"write_iops"`     // 写IOPS
	ReadLatency   float64 `json:"read_latency"`   // 读延迟 (ms)
	WriteLatency  float64 `json:"write_latency"`  // 写延迟 (ms)
	ReadBandwidth  float64 `json:"read_bandwidth"`  // 读带宽 (MB/s)
	WriteBandwidth float64 `json:"write_bandwidth"` // 写带宽 (MB/s)

	// 访问指标
	AccessCount   int64  `json:"access_count"`    // 访问次数
	UniqueUsers   int    `json:"unique_users"`    // 唯一用户数
	HotFiles      int    `json:"hot_files"`       // 热点文件数

	// 健康指标
	ErrorCount    int64  `json:"error_count"`     // 错误次数
	CorruptedFiles int   `json:"corrupted_files"` // 损坏文件数
}

// AccessPattern 访问模式
type AccessPattern struct {
	Timestamp    time.Time `json:"timestamp"`     // 时间戳
	Operation    string    `json:"operation"`     // 操作类型 (read/write/delete)
	FileType     string    `json:"file_type"`     // 文件类型
	UserID       string    `json:"user_id"`       // 用户ID
	FileSize     int64     `json:"file_size"`     // 文件大小
	FilePath     string    `json:"file_path"`     // 文件路径
	SourceIP     string    `json:"source_ip"`     // 源IP
}

// ============================================================
// 异常规则类型
// ============================================================

// AnomalyRule 异常检测规则
type AnomalyRule struct {
	ID          string        `json:"id"`           // 规则ID
	Name        string        `json:"name"`         // 规则名称
	Description string        `json:"description"`  // 描述
	Type        AnomalyType   `json:"type"`         // 异常类型
	Enabled     bool          `json:"enabled"`      // 是否启用
	Severity    AnomalySeverity `json:"severity"`   // 默认严重程度
	Conditions  []Condition   `json:"conditions"`   // 触发条件
	Actions     []Action      `json:"actions"`      // 触发动作
	CreatedAt   time.Time     `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time     `json:"updated_at"`   // 更新时间
}

// Condition 触发条件
type Condition struct {
	Field    string      `json:"field"`    // 字段
	Operator string      `json:"operator"` // 操作符 (gt, lt, eq, gte, lte, between)
	Value    interface{} `json:"value"`    // 阈值
	Duration string      `json:"duration"` // 持续时间
}

// Action 触发动作
type Action struct {
	Type     string            `json:"type"`     // 动作类型 (alert, log, webhook, email)
	Config   map[string]string `json:"config"`   // 动作配置
}

// ============================================================
// 异常事件类型
// ============================================================

// AnomalyEvent 异常事件
type AnomalyEvent struct {
	ID          string          `json:"id"`           // 事件ID
	RuleID      string          `json:"rule_id"`      // 触发规则ID
	RuleName    string          `json:"rule_name"`    // 规则名称
	Type        AnomalyType     `json:"type"`         // 异常类型
	Severity    AnomalySeverity `json:"severity"`     // 严重程度
	DeviceID    string          `json:"device_id"`    // 设备ID
	MountPoint  string          `json:"mount_point"`  // 挂载点
	Title       string          `json:"title"`        // 事件标题
	Description string          `json:"description"`  // 详细描述
	Metrics     *StorageMetrics `json:"metrics"`      // 相关指标
	Evidence    []string        `json:"evidence"`     // 证据
	Suggestions []string        `json:"suggestions"`  // 建议操作
	DetectedAt  time.Time       `json:"detected_at"`  // 检测时间
	AckedAt     *time.Time      `json:"acked_at"`     // 确认时间
	AckedBy     string          `json:"acked_by"`     // 确认人
	Resolved    bool            `json:"resolved"`     // 是否已解决
	ResolvedAt  *time.Time      `json:"resolved_at"`  // 解决时间
}

// ============================================================
// 检测结果类型
// ============================================================

// DetectionResult 检测结果
type DetectionResult struct {
	HasAnomaly    bool             `json:"has_anomaly"`    // 是否有异常
	AnomalyCount  int              `json:"anomaly_count"`  // 异常数量
	Events        []AnomalyEvent   `json:"events"`         // 异常事件列表
	Summary       string           `json:"summary"`        // 检测摘要
	AnalyzedAt    time.Time        `json:"analyzed_at"`    // 分析时间
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// AnomalyResponse 异常响应
type AnomalyResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// EventListResponse 事件列表响应
type EventListResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    []AnomalyEvent `json:"data,omitempty"`
}

// RuleListResponse 规则列表响应
type RuleListResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    []AnomalyRule `json:"data,omitempty"`
}

// MetricsResponse 指标响应
type MetricsResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *StorageMetrics `json:"data,omitempty"`
}

// AnomalyStats 异常统计
type AnomalyStats struct {
	TotalEvents     int `json:"total_events"`      // 总事件数
	UnackedEvents   int `json:"unacked_events"`    // 未确认事件数
	UnresolvedEvents int `json:"unresolved_events"` // 未解决事件数
	CriticalEvents  int `json:"critical_events"`   // 严重事件数
	WarningEvents   int `json:"warning_events"`    // 警告事件数
	ActiveRules     int `json:"active_rules"`      // 活跃规则数
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *AnomalyStats `json:"data,omitempty"`
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name        string          `json:"name"`         // 规则名称
	Description string          `json:"description"`  // 描述
	Type        AnomalyType     `json:"type"`         // 异常类型
	Severity    AnomalySeverity `json:"severity"`     // 严重程度
	Conditions  []Condition     `json:"conditions"`   // 条件
	Actions     []Action        `json:"actions"`      // 动作
}

// UpdateRuleRequest 更新规则请求
type UpdateRuleRequest struct {
	ID          string          `json:"id"`           // 规则ID
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Enabled     *bool           `json:"enabled,omitempty"`
	Severity    AnomalySeverity `json:"severity,omitempty"`
	Conditions  []Condition     `json:"conditions,omitempty"`
	Actions     []Action        `json:"actions,omitempty"`
}

// AckEventRequest 确认事件请求
type AckEventRequest struct {
	EventID string `json:"event_id"` // 事件ID
	UserID  string `json:"user_id"`  // 确认人
}

// ResolveEventRequest 解决事件请求
type ResolveEventRequest struct {
	EventID string `json:"event_id"` // 事件ID
	Note    string `json:"note"`     // 解决备注
}
