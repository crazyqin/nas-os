// Package diskpredict - 磁盘故障预测模块
// SMART 数据采集、健康评分、故障预测、生命周期管理
package diskpredict

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// PredictConfig 预测引擎配置
type PredictConfig struct {
	// 健康评分权重
	WeightReallocatedSectors float64 `json:"weight_reallocated_sectors"` // 重分配扇区权重, 默认 0.30
	WeightPendingSectors     float64 `json:"weight_pending_sectors"`     // 待定扇区权重, 默认 0.25
	WeightCRCError           float64 `json:"weight_crc_error"`           // CRC错误权重, 默认 0.15
	WeightTemperature        float64 `json:"weight_temperature"`         // 温度权重, 默认 0.15
	WeightPowerOnHours       float64 `json:"weight_power_on_hours"`      // 通电时间权重, 默认 0.15

	// 告警阈值
	Thresholds map[string]int `json:"thresholds"` // 告警阈值配置

	// 历史记录
	MaxHistoryDays int `json:"max_history_days"` // 最大历史天数, 默认 365

	// 巡检配置
	ScanInterval time.Duration `json:"scan_interval"` // 巡检间隔
	DeviceFilter []string      `json:"device_filter"` // 设备过滤列表（正则）
}

// DefaultPredictConfig 默认预测配置
func DefaultPredictConfig() PredictConfig {
	return PredictConfig{
		WeightReallocatedSectors: 0.30,
		WeightPendingSectors:     0.25,
		WeightCRCError:           0.15,
		WeightTemperature:        0.15,
		WeightPowerOnHours:       0.15,
		Thresholds: map[string]int{
			"reallocated_sectors": 10,
			"pending_sectors":     5,
			"crc_error":          50,
			"temperature":        55,
			"power_on_hours":     40000,
		},
		MaxHistoryDays: 365,
		ScanInterval:   24 * time.Hour,
	}
}

// ============================================================
// 磁盘状态类型
// ============================================================

// DiskStatus 磁盘状态枚举
type DiskStatus string

const (
	StatusHealthy  DiskStatus = "healthy"  // 健康
	StatusWarning  DiskStatus = "warning"  // 警告
	StatusCritical DiskStatus = "critical" // 严重
	StatusFailed   DiskStatus = "failed"   // 故障
)

// ============================================================
// SMART 数据类型
// ============================================================

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID         uint8  `json:"id"`          // 属性ID
	Name       string `json:"name"`        // 属性名称
	Value      uint8  `json:"value"`       // 当前值
	Worst      uint8  `json:"worst"`       // 最差值
	Threshold  uint8  `json:"threshold"`   // 阈值
	RawValue   uint64 `json:"raw_value"`   // 原始值
	IsFailed   bool   `json:"is_failed"`   // 是否失败
	IsCritical bool   `json:"is_critical"` // 是否关键属性
}

// SMARTData SMART 数据
type SMARTData struct {
	Device      string           `json:"device"`       // 设备路径 e.g. /dev/sda
	Model       string           `json:"model"`        // 磁盘型号
	Serial      string           `json:"serial"`       // 序列号
	Capacity    uint64           `json:"capacity"`     // 容量 (字节)
	Temperature int              `json:"temperature"`  // 当前温度 (°C)
	PowerOnHours uint64          `json:"power_on_hours"` // 通电时间 (小时)
	HealthState string           `json:"health_state"` // 健康状态
	Attributes  []SMARTAttribute `json:"attributes"`   // SMART属性列表
	CollectedAt time.Time        `json:"collected_at"` // 采集时间
}

// ============================================================
// 属性分析类型
// ============================================================

// AttributeAnalysis 属性分析结果
type AttributeAnalysis struct {
	ID            uint8   `json:"id"`             // 属性ID
	Name          string  `json:"name"`           // 属性名称
	Value         uint8   `json:"value"`          // 当前值
	Threshold     uint8   `json:"threshold"`      // 阈值
	Score         float64 `json:"score"`          // 属性得分 (0-100)
	Weight        float64 `json:"weight"`         // 权重
	WeightedScore float64 `json:"weighted_score"` // 加权得分
	Status        string  `json:"status"`         // 状态: "normal", "warning", "critical"
	Message       string  `json:"message"`        // 状态消息
}

// ============================================================
// 磁盘信息类型
// ============================================================

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device       string     `json:"device"`        // 设备路径
	Model        string     `json:"model"`         // 型号
	Serial       string     `json:"serial"`        // 序列号
	Capacity     uint64     `json:"capacity"`      // 容量
	Status       DiskStatus `json:"status"`        // 状态
	SMARTEnabled bool       `json:"smart_enabled"` // SMART是否启用
	RegisteredAt time.Time  `json:"registered_at"` // 注册时间
	LastScanAt   time.Time  `json:"last_scan_at"`  // 最后扫描时间
}

// ============================================================
// 预测结果类型
// ============================================================

// PredictionResult 预测结果
type PredictionResult struct {
	Device             string              `json:"device"`               // 设备路径
	Model              string              `json:"model"`                // 型号
	Serial             string              `json:"serial"`               // 序列号
	HealthScore        float64             `json:"health_score"`         // 健康评分 (0-100)
	Status             DiskStatus          `json:"status"`               // 状态
	EstimatedLifeDays  int                 `json:"estimated_life_days"`  // 预计剩余寿命（天）
	EstimatedFailDate  *time.Time          `json:"estimated_fail_date"`  // 预计故障日期
	RiskFactors        []string            `json:"risk_factors"`         // 风险因素
	AnalyzedAttributes []AttributeAnalysis `json:"analyzed_attributes"`  // 分析的属性
	PredictedAt        time.Time           `json:"predicted_at"`         // 预测时间
}

// ============================================================
// 告警类型
// ============================================================

// AlertInfo 告警信息
type AlertInfo struct {
	Device    string    `json:"device"`     // 设备路径
	Level     string    `json:"level"`      // 告警级别: "warning", "critical"
	Message   string    `json:"message"`    // 告警消息
	CreatedAt time.Time `json:"created_at"` // 创建时间
	Resolved  bool      `json:"resolved"`   // 是否已解决
}

// ============================================================
// 统计类型
// ============================================================

// DiskStats 磁盘统计信息
type DiskStats struct {
	TotalDisks    int     `json:"total_disks"`    // 总磁盘数
	HealthyDisks  int     `json:"healthy_disks"`  // 健康磁盘数
	WarningDisks  int     `json:"warning_disks"`  // 警告磁盘数
	CriticalDisks int     `json:"critical_disks"` // 严重磁盘数
	FailedDisks   int     `json:"failed_disks"`   // 故障磁盘数
	AvgHealthScore float64 `json:"avg_health_score"` // 平均健康评分
}

// ============================================================
// 健康报告类型（用于批量巡检）
// ============================================================

// HealthReport 健康巡检报告
type HealthReport struct {
	Disks            []*DiskHealth `json:"disks"`              // 磁盘列表
	OverallHealth    string        `json:"overall_health"`     // 整体健康: "healthy", "warning", "critical"
	CriticalCount    int           `json:"critical_count"`     // 严重问题数
	WarningCount     int           `json:"warning_count"`      // 警告数
	PredictedFailures int          `json:"predicted_failures"` // 预测故障数
	ScanTime         time.Time     `json:"scan_time"`          // 扫描时间
	Duration         time.Duration `json:"duration"`           // 扫描耗时
}

// DiskHealth 磁盘健康状态（简化版）
type DiskHealth struct {
	Device           string    `json:"device"`             // 设备路径
	Score            float64   `json:"score"`              // 健康评分 (0-100)
	Status           string    `json:"status"`             // 状态
	PredictedFailure bool      `json:"predicted_failure"`  // 是否预测故障
	LastCheck        time.Time `json:"last_check"`         // 最后检查时间
	SMARTData        SMARTData `json:"smart_data"`         // SMART数据
}

// ============================================================
// 故障预测类型
// ============================================================

// FailurePrediction 故障预测结果
type FailurePrediction struct {
	Device         string   `json:"device"`          // 设备路径
	Probability    float64  `json:"probability"`     // 故障概率 (0-1)
	EstimatedDays  int      `json:"estimated_days"`  // 预计故障天数
	Confidence     string   `json:"confidence"`      // 置信度: "high", "medium", "low"
	FailureType    string   `json:"failure_type"`    // 预测故障类型
	RiskFactors    []string `json:"risk_factors"`    // 风险因子
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// PredictRequest 预测请求
type PredictRequest struct {
	Device string `json:"device"` // 设备路径
}

// PredictResponse 预测响应
type PredictResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PredictionListResponse 预测列表响应
type PredictionListResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    []PredictionResult  `json:"data,omitempty"`
}

// DiskListResponse 磁盘列表响应
type DiskListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []DiskInfo  `json:"data,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *DiskStats `json:"data,omitempty"`
}

// AlertListResponse 告警列表响应
type AlertListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []AlertInfo `json:"data,omitempty"`
}
