// Package diskhealthai 提供智能磁盘诊断功能，基于 SMART 数据的深度分析与故障预测
package diskhealthai

import (
	"time"
)

// HealthStatus 磁盘健康状态
type HealthStatus string

const (
	StatusExcellent HealthStatus = "excellent" // 优秀
	StatusGood      HealthStatus = "good"      // 良好
	StatusFair      HealthStatus = "fair"      // 一般
	StatusPoor      HealthStatus = "poor"      // 较差
	StatusCritical  HealthStatus = "critical"  // 临界
	StatusFailed    HealthStatus = "failed"    // 已失败
)

// DiskInfo 磁盘基础信息
type DiskInfo struct {
	Device        string       `json:"device"`         // 设备名称
	Model         string       `json:"model"`          // 型号
	Serial        string       `json:"serial"`         // 序列号
	CapacityBytes uint64       `json:"capacity_bytes"` // 容量字节数
	Firmware      string       `json:"firmware"`       // 固件版本
	Interface     string       `json:"interface"`      // 接口类型（SATA/NVMe/SAS）
	Status        HealthStatus `json:"status"`         // 当前状态
	SMARTEnabled  bool         `json:"smart_enabled"`  // SMART 是否启用
	RegisteredAt  time.Time    `json:"registered_at"`  // 注册时间
	LastScanAt    time.Time    `json:"last_scan_at"`   // 最后扫描时间
}

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID          int    `json:"id"`          // 属性 ID
	Name        string `json:"name"`        // 属性名称
	Value       int    `json:"value"`       // 当前值（0-253）
	Worst       int    `json:"worst"`       // 历史最差值
	Threshold   int    `json:"threshold"`   // 阈值
	RawValue    uint64 `json:"raw_value"`   // 原始值
	IsCritical  bool   `json:"is_critical"` // 是否为关键指标
	Failed      bool   `json:"failed"`      // 是否已失败
	Description string `json:"description"` // 属性说明
}

// SMARTSnapshot SMART 数据快照（单次采集）
type SMARTSnapshot struct {
	Device             string           `json:"device"`
	Model              string           `json:"model"`
	Serial             string           `json:"serial"`
	Temperature        int              `json:"temperature"`           // 当前温度（℃）
	MaxTemperature     int              `json:"max_temperature"`       // 历史最高温度
	PowerOnHours       uint64           `json:"power_on_hours"`        // 通电时间（小时）
	PowerCycleCount    uint64           `json:"power_cycle_count"`     // 通电周期数
	ReallocatedSects   uint64           `json:"reallocated_sectors"`   // 重映射扇区数
	PendingSects       uint64           `json:"pending_sectors"`       // 待映射扇区数
	UncorrectableSects uint64           `json:"uncorrectable_sectors"` // 不可修复扇区数
	TotalLBAsRead      uint64           `json:"total_lbas_read"`       // 总读取 LBAs
	TotalLBAsWritten   uint64           `json:"total_lbas_written"`    // 总写入 LBAs
	SeekErrorRate      uint64           `json:"seek_error_rate"`       // 寻道错误率
	SpinRetryCount     uint64           `json:"spin_retry_count"`      // 主轴重试次数
	Attributes         []SMARTAttribute `json:"attributes"`            // SMART 属性列表
	CollectedAt        time.Time        `json:"collected_at"`          // 采集时间
}

// HealthReport 健康诊断报告
type HealthReport struct {
	Device            string              `json:"device"`
	Model             string              `json:"model"`
	Serial            string              `json:"serial"`
	HealthScore       float64             `json:"health_score"`        // 综合健康评分（0-100）
	Status            HealthStatus        `json:"status"`              // 健康状态
	Grade             string              `json:"grade"`               // 评级（A-F）
	EstimatedLifeDays int                 `json:"estimated_life_days"` // 预估剩余寿命（天）
	EstimatedFailDate *time.Time          `json:"estimated_fail_date,omitempty"`
	RiskLevel         string              `json:"risk_level"`         // 风险等级
	RiskFactors       []RiskFactor        `json:"risk_factors"`       // 风险因素列表
	DimensionScores   DimensionScores     `json:"dimension_scores"`   // 各维度评分
	AttributeAnalysis []AttributeAnalysis `json:"attribute_analysis"` // 属性分析
	TrendAnalysis     *TrendAnalysis      `json:"trend_analysis,omitempty"`
	Recommendations   []string            `json:"recommendations"` // 维护建议
	AnalyzedAt        time.Time           `json:"analyzed_at"`     // 分析时间
}

// RiskFactor 风险因素
type RiskFactor struct {
	ID     string  `json:"id"`     // 风险因素 ID
	Name   string  `json:"name"`   // 风险因素名称
	Level  string  `json:"level"`  // 级别（low/medium/high/critical）
	Weight float64 `json:"weight"` // 权重
	Detail string  `json:"detail"` // 详细说明
}

// DimensionScores 各维度评分
type DimensionScores struct {
	SMARTScore       float64 `json:"smart_score"`       // SMART 属性评分
	TemperatureScore float64 `json:"temperature_score"` // 温度评分
	PowerOnScore     float64 `json:"power_on_score"`    // 通电时间评分
	WorkloadScore    float64 `json:"workload_score"`    // 工作负载评分
	AgeScore         float64 `json:"age_score"`         // 年龄评分
}

// AttributeAnalysis 单个 SMART 属性分析结果
type AttributeAnalysis struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Value         int     `json:"value"`
	Threshold     int     `json:"threshold"`
	Score         float64 `json:"score"`          // 该属性得分（0-100）
	Weight        float64 `json:"weight"`         // 权重
	WeightedScore float64 `json:"weighted_score"` // 加权得分
	Status        string  `json:"status"`         // normal/warning/critical
	Message       string  `json:"message"`
	Trend         string  `json:"trend"` // improving/stable/declining
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	HealthTrend          string  `json:"health_trend"`          // 健康趋势（improving/stable/declining）
	TemperatureTrend     string  `json:"temperature_trend"`     // 温度趋势
	WorkloadTrend        string  `json:"workload_trend"`        // 工作负载趋势
	ProjectedScore90D    float64 `json:"projected_score_90d"`   // 90天后预估评分
	ProjectionConfidence float64 `json:"projection_confidence"` // 预测置信度
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	Device          string    `json:"device"`
	ReadThroughput  float64   `json:"read_throughput_mbps"`  // 读取吞吐量（MB/s）
	WriteThroughput float64   `json:"write_throughput_mbps"` // 写入吞吐量（MB/s）
	ReadLatency     float64   `json:"read_latency_ms"`       // 读取延迟（ms）
	WriteLatency    float64   `json:"write_latency_ms"`      // 写入延迟（ms）
	IOPS            int       `json:"iops"`                  // IOPS
	MeasuredAt      time.Time `json:"measured_at"`
}

// LifecycleStage 生命周期阶段
type LifecycleStage string

const (
	LifecycleNew       LifecycleStage = "new"         // 新盘
	LifecycleEarly     LifecycleStage = "early"       // 早期
	LifecycleMidLife   LifecycleStage = "mid_life"    // 中期
	LifecycleMature    LifecycleStage = "mature"      // 成熟期
	LifecycleWearOut   LifecycleStage = "wear_out"    // 磨损期
	LifecycleEndOfLife LifecycleStage = "end_of_life" // 末期
)

// Lifecycle 生命周期评估
type Lifecycle struct {
	Device        string         `json:"device"`
	Stage         LifecycleStage `json:"stage"`          // 当前阶段
	AgeDays       int            `json:"age_days"`       // 使用天数
	PowerOnHours  uint64         `json:"power_on_hours"` // 通电时间
	ExpectedYears float64        `json:"expected_years"` // 预期寿命（年）
	UsedPercent   float64        `json:"used_percent"`   // 已使用百分比
}

// Alert 告警信息
type Alert struct {
	ID        string      `json:"id"`
	Device    string      `json:"device"`
	Level     string      `json:"level"` // info/warning/critical
	Type      string      `json:"type"`  // 告警类型
	Message   string      `json:"message"`
	Value     interface{} `json:"value,omitempty"`
	Threshold interface{} `json:"threshold,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	Resolved  bool        `json:"resolved"`
}

// ========== API 请求/响应类型 ==========

// DiagnoseRequest 诊断请求
type DiagnoseRequest struct {
	Device string `json:"device"` // 设备名，空则诊断所有
}

// DiagnoseResponse 诊断响应
type DiagnoseResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    []HealthReport `json:"data,omitempty"`
}

// TrendRequest 趋势分析请求
type TrendRequest struct {
	Device    string `json:"device"`
	DaysRange int    `json:"days_range"` // 分析天数范围，默认90天
}

// TrendResponse 趋势分析响应
type TrendResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *TrendAnalysis `json:"data,omitempty"`
}

// AlertListResponse 告警列表响应
type AlertListResponse struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    []Alert `json:"data,omitempty"`
}

// LifecycleResponse 生命周期响应
type LifecycleResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *Lifecycle `json:"data,omitempty"`
}

// PerformanceResponse 性能指标响应
type PerformanceResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    *PerformanceMetrics `json:"data,omitempty"`
}
