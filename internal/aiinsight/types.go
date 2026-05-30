// Package aiinsight 提供 AI 智能运维洞察功能
// 系统健康评分、异常检测、根因分析、容量预测、性能瓶颈识别、智能告警、运维建议、趋势分析
package aiinsight

import (
	"time"
)

// ========== 系统健康评分 ==========

// HealthScore 系统健康评分
type HealthScore struct {
	Total        int       `json:"total"`         // 总分 (0-100)
	CPU          int       `json:"cpu"`           // CPU 分数
	Memory       int       `json:"memory"`        // 内存分数
	Disk         int       `json:"disk"`          // 磁盘分数
	Network      int       `json:"network"`       // 网络分数
	Grade        string    `json:"grade"`         // 等级: A/B/C/D/F
	AssessedAt   time.Time `json:"assessedAt"`    // 评估时间
	AssessedBy   string    `json:"assessedBy"`    // 评估来源 (scheduler/manual)
}

// HealthGrade 健康等级
const (
	GradeA = "A" // 90-100 优秀
	GradeB = "B" // 80-89 良好
	GradeC = "C" // 60-79 一般
	GradeD = "D" // 40-59 较差
	GradeF = "F" // 0-39 严重
)

// ========== 异常检测 ==========

// Anomaly 检测到的异常
type Anomaly struct {
	ID          string    `json:"id"`           // 唯一标识
	Type        string    `json:"type"`         // 类型: cpu/memory/disk/network/io
	Severity    string    `json:"severity"`     // 严重程度: info/warning/critical
	DetectedAt  time.Time `json:"detectedAt"`   // 检测时间
	Description string    `json:"description"`  // 描述
	MetricValue float64   `json:"metricValue"`  // 当前指标值
	Baseline    float64   `json:"baseline"`     // 基线值
	Deviation   float64   `json:"deviation"`    // 偏差百分比
	Status      string    `json:"status"`       // 状态: open/dismissed/resolved
	DismissedAt *time.Time `json:"dismissedAt,omitempty"` // 忽略时间
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`  // 解决时间
}

// AnomalyType 异常类型
const (
	AnomalyTypeCPU     = "cpu"
	AnomalyTypeMemory  = "memory"
	AnomalyTypeDisk    = "disk"
	AnomalyTypeNetwork = "network"
	AnomalyTypeIO      = "io"
)

// AnomalySeverity 异常严重程度
const (
	AnomalySeverityInfo     = "info"
	AnomalySeverityWarning  = "warning"
	AnomalySeverityCritical = "critical"
)

// AnomalyStatus 异常状态
const (
	AnomalyStatusOpen     = "open"
	AnomalyStatusDismissed = "dismissed"
	AnomalyStatusResolved = "resolved"
)

// ========== 根因分析 ==========

// RootCause 根因分析结果
type RootCause struct {
	ID          string    `json:"id"`           // 唯一标识
	FaultDesc   string    `json:"faultDesc"`    // 故障描述
	RootCause   string    `json:"rootCause"`    // 根因类型: hardware/software/config/load
	Confidence  float64   `json:"confidence"`   // 置信度 (0-1)
	Impact      string    `json:"impact"`       // 影响范围
	Suggestion  string    `json:"suggestion"`   // 建议修复方案
	DetectedAt  time.Time `json:"detectedAt"`   // 检测时间
	Resolved    bool      `json:"resolved"`     // 是否已解决
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"` // 解决时间
}

// RootCauseType 根因类型
const (
	RootCauseTypeHardware = "hardware"
	RootCauseTypeSoftware = "software"
	RootCauseTypeConfig   = "config"
	RootCauseTypeLoad     = "load"
)

// ========== 容量预测 ==========

// CapacityPrediction 容量预测
type CapacityPrediction struct {
	ResourceType  string    `json:"resourceType"`  // 资源类型: cpu/memory/disk/network
	CurrentValue  float64   `json:"currentValue"`  // 当前使用值 (%)
	PredictedValue float64  `json:"predictedValue"` // 预测值 (%)
	ExhaustDate   *time.Time `json:"exhaustDate,omitempty"` // 预计耗尽日期
	Confidence    float64   `json:"confidence"`    // 置信度 (0-1)
	GrowthRate    float64   `json:"growthRate"`    // 增长率 (%/天)
	PredictedAt   time.Time `json:"predictedAt"`   // 预测时间
}

// ResourceType 资源类型
const (
	ResourceTypeCPU     = "cpu"
	ResourceTypeMemory  = "memory"
	ResourceTypeDisk    = "disk"
	ResourceTypeNetwork = "network"
)

// ========== 性能瓶颈 ==========

// Bottleneck 性能瓶颈
type Bottleneck struct {
	ID          string    `json:"id"`           // 唯一标识
	Type        string    `json:"type"`         // 类型: cpu/memory/disk/network
	Severity    string    `json:"severity"`     // 严重程度: low/medium/high/critical
	Impact      string    `json:"impact"`       // 影响范围
	Suggestion  string    `json:"suggestion"`   // 建议操作
	DetectedAt  time.Time `json:"detectedAt"`   // 检测时间
	Resolved    bool      `json:"resolved"`     // 是否已解决
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"` // 解决时间
}

// BottleneckType 瓶颈类型
const (
	BottleneckTypeCPU     = "cpu"
	BottleneckTypeMemory  = "memory"
	BottleneckTypeDisk    = "disk"
	BottleneckTypeNetwork = "network"
)

// BottleneckSeverity 瓶颈严重程度
const (
	BottleneckSeverityLow      = "low"
	BottleneckSeverityMedium   = "medium"
	BottleneckSeverityHigh     = "high"
	BottleneckSeverityCritical = "critical"
)

// ========== 智能告警 ==========

// SmartAlert 智能告警
type SmartAlert struct {
	ID          string     `json:"id"`          // 唯一标识
	Source      string     `json:"source"`      // 告警来源
	Level       string     `json:"level"`       // 级别: info/warning/critical
	Message     string     `json:"message"`     // 告警消息
	AggCount    int        `json:"aggCount"`    // 聚合次数
	FirstTime   time.Time  `json:"firstTime"`   // 首次时间
	LastTime    time.Time  `json:"lastTime"`    // 最后时间
	Status      string     `json:"status"`      // 状态: open/ack/resolved
	AckedAt     *time.Time `json:"ackedAt,omitempty"`     // 确认时间
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`  // 解决时间
	AckedBy     string     `json:"ackedBy,omitempty"`     // 确认人
	ResolvedBy  string     `json:"resolvedBy,omitempty"`  // 解决人
}

// AlertLevel 告警级别
const (
	AlertLevelInfo     = "info"
	AlertLevelWarning  = "warning"
	AlertLevelCritical = "critical"
)

// AlertStatus 告警状态
const (
	AlertStatusOpen     = "open"
	AlertStatusAck      = "ack"
	AlertStatusResolved = "resolved"
)

// ========== 运维建议 ==========

// MaintenanceTip 运维建议
type MaintenanceTip struct {
	ID          string    `json:"id"`          // 唯一标识
	Category    string    `json:"category"`    // 类别: security/performance/reliability
	Priority    string    `json:"priority"`    // 优先级: low/medium/high/critical
	Title       string    `json:"title"`       // 标题
	Description string    `json:"description"` // 描述
	Impact      string    `json:"impact"`      // 影响说明
	Steps       []string  `json:"steps"`       // 操作步骤
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	DismissedAt *time.Time `json:"dismissedAt,omitempty"` // 忽略时间
}

// TipCategory 建议类别
const (
	TipCategorySecurity     = "security"
	TipCategoryPerformance  = "performance"
	TipCategoryReliability  = "reliability"
)

// TipPriority 建议优先级
const (
	TipPriorityLow      = "low"
	TipPriorityMedium   = "medium"
	TipPriorityHigh     = "high"
	TipPriorityCritical = "critical"
)

// ========== 趋势分析 ==========

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	MetricName  string    `json:"metricName"`  // 指标名称
	TimeRange   string    `json:"timeRange"`   // 时间范围: 1h/6h/24h/7d/30d
	AvgValue    float64   `json:"avgValue"`    // 平均值
	MinValue    float64   `json:"minValue"`    // 最小值
	MaxValue    float64   `json:"maxValue"`    // 最大值
	Trend       string    `json:"trend"`       // 趋势方向: up/down/stable
	ChangeRate  float64   `json:"changeRate"`  // 变化率 (%)
	SampleCount int       `json:"sampleCount"` // 样本数
	AnalyzedAt  time.Time `json:"analyzedAt"`  // 分析时间
}

// TrendDirection 趋势方向
const (
	TrendUp     = "up"
	TrendDown   = "down"
	TrendStable = "stable"
)

// ========== 洞察报告 ==========

// InsightReport 洞察报告
type InsightReport struct {
	ID           string       `json:"id"`           // 唯一标识
	GeneratedAt  time.Time    `json:"generatedAt"`  // 生成时间
	HealthScore  HealthScore  `json:"healthScore"`  // 健康评分
	AnomalyCount int          `json:"anomalyCount"` // 异常数
	AlertCount   int          `json:"alertCount"`   // 告警数
	TipCount     int          `json:"tipCount"`     // 建议数
	Summary      string       `json:"summary"`      // 摘要
	Anomalies    []Anomaly    `json:"anomalies"`    // 异常列表
	Alerts       []SmartAlert `json:"alerts"`       // 告警列表
	Tips         []MaintenanceTip `json:"tips"`     // 建议列表
}

// ========== 配置 ==========

// InsightConfig AI 洞察配置
type InsightConfig struct {
	DetectionInterval string  `json:"detectionInterval"` // 检测间隔 (如 "5m", "1h")
	AlertThreshold    float64 `json:"alertThreshold"`    // 告警阈值 (偏差百分比)
	PredictionWindow  string  `json:"predictionWindow"`  // 预测窗口 (如 "7d", "30d")
	EnabledRules      []string `json:"enabledRules"`     // 启用的规则列表
	MaxAnomalyHistory int     `json:"maxAnomalyHistory"`  // 最大异常历史数
	MaxAlertHistory   int     `json:"maxAlertHistory"`    // 最大告警历史数
	ReportRetention   string  `json:"reportRetention"`   // 报告保留时间 (如 "90d")
}

// DefaultInsightConfig 默认配置
func DefaultInsightConfig() InsightConfig {
	return InsightConfig{
		DetectionInterval: "5m",
		AlertThreshold:    20.0, // 20% 偏差触发告警
		PredictionWindow:  "30d",
		EnabledRules: []string{
			"cpu_high",
			"memory_high",
			"disk_full",
			"network_saturation",
			"io_bottleneck",
		},
		MaxAnomalyHistory: 1000,
		MaxAlertHistory:   500,
		ReportRetention:   "90d",
	}
}

// ========== 统计 ==========

// InsightStats 洞察统计信息
type InsightStats struct {
	TotalAnomalies  int       `json:"totalAnomalies"`  // 总异常数
	OpenAnomalies   int       `json:"openAnomalies"`   // 未处理异常
	TotalAlerts     int       `json:"totalAlerts"`     // 总告警数
	OpenAlerts      int       `json:"openAlerts"`      // 未处理告警
	TotalTips       int       `json:"totalTips"`       // 总建议数
	ActiveTips      int       `json:"activeTips"`      // 有效建议
	TotalReports    int       `json:"totalReports"`    // 总报告数
	LastAnalysis    *time.Time `json:"lastAnalysis,omitempty"` // 上次分析时间
	UptimeHours     float64   `json:"uptimeHours"`     // 运行时长 (小时)
}
