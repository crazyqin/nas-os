package smarthealth

import (
	"time"
)

// DiskStatus 磁盘状态.
type DiskStatus string

const (
	// DiskStatusHealthy 健康状态.
	DiskStatusHealthy DiskStatus = "healthy"
	// DiskStatusWarning 警告状态.
	DiskStatusWarning DiskStatus = "warning"
	// DiskStatusCritical 严重警告状态.
	DiskStatusCritical DiskStatus = "critical"
	// DiskStatusFailed 故障状态.
	DiskStatusFailed DiskStatus = "failed"
	// DiskStatusUnknown 未知状态.
	DiskStatusUnknown DiskStatus = "unknown"
)

// DiskType 磁盘类型.
type DiskType string

const (
	// DiskTypeHDD 机械硬盘.
	DiskTypeHDD DiskType = "hdd"
	// DiskTypeSSD 固态硬盘.
	DiskTypeSSD DiskType = "ssd"
	// DiskTypeNVMe NVMe 硬盘.
	DiskTypeNVMe DiskType = "nvme"
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	// AlertLevelInfo 信息级别.
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning 警告级别.
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelCritical 严重级别.
	AlertLevelCritical AlertLevel = "critical"
	// AlertLevelEmergency 紧急级别.
	AlertLevelEmergency AlertLevel = "emergency"
)

// DiskHealth 磁盘健康信息.
type DiskHealth struct {
	DiskID      string     `json:"diskId"`
	Device      string     `json:"device"`      // 设备路径, e.g., /dev/sda
	Model       string     `json:"model"`       // 型号
	Serial      string     `json:"serial"`      // 序列号
	Type        DiskType   `json:"type"`        // 磁盘类型
	Capacity    int64      `json:"capacity"`    // 容量 (bytes)
	Status      DiskStatus `json:"status"`      // 健康状态
	HealthScore int        `json:"healthScore"` // 健康评分 (0-100)

	// S.M.A.R.T. 属性
	SMARTAttributes []SMARTAttribute `json:"smartAttributes"`

	// 关键指标
	Temperature     int   `json:"temperature"`     // 温度 (°C)
	PowerOnHours    int64 `json:"powerOnHours"`    // 通电时间 (小时)
	PowerCycleCount int64 `json:"powerCycleCount"` // 通电次数
	ReallocatedSectors int64 `json:"reallocatedSectors"` // 重分配扇区数
	PendingSectors  int64 `json:"pendingSectors"`  // 待映射扇区数
	UncorrectableErrors int64 `json:"uncorrectableErrors"` // 不可纠正错误数

	// 统计信息
	TotalReads  int64 `json:"totalReads"`  // 总读取量 (bytes)
	TotalWrites int64 `json:"totalWrites"` // 总写入量 (bytes)

	// 预测信息
	Prediction *FailurePrediction `json:"prediction,omitempty"`

	// 更新时间
	LastUpdated time.Time `json:"lastUpdated"`
	LastScanned time.Time `json:"lastScanned"`
}

// SMARTAttribute S.M.A.R.T. 属性.
type SMARTAttribute struct {
	ID          int    `json:"id"`          // 属性 ID
	Name        string `json:"name"`        // 属性名称
	Value       int    `json:"value"`       // 当前值
	Worst       int    `json:"worst"`       // 最差值
	Threshold   int    `json:"threshold"`   // 阈值
	RawValue    int64  `json:"rawValue"`    // 原始值
	Failed      bool   `json:"failed"`      // 是否失败
	Description string `json:"description"` // 描述
}

// HealthScore 健康评分.
type HealthScore struct {
	DiskID      string     `json:"diskId"`
	Device      string     `json:"device"`
	Score       int        `json:"score"`       // 0-100
	Status      DiskStatus `json:"status"`
	Factors     []ScoreFactor `json:"factors"` // 评分因素
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// ScoreFactor 评分因素.
type ScoreFactor struct {
	Name    string  `json:"name"`
	Weight  float64 `json:"weight"`  // 权重 (0-1)
	Impact  float64 `json:"impact"`  // 影响分
	Detail  string  `json:"detail"`  // 详细说明
}

// FailurePrediction 故障预测.
type FailurePrediction struct {
	DiskID          string    `json:"diskId"`
	Device          string    `json:"device"`
	PredictedAt     time.Time `json:"predictedAt"`

	// 预测结果
	FailureProbability float64 `json:"failureProbability"` // 故障概率 (0-1)
	EstimatedDaysLeft  int     `json:"estimatedDaysLeft"`  // 预计剩余天数
	RiskLevel          string  `json:"riskLevel"`          // 风险等级 (low/medium/high/critical)

	// 趋势分析
	TemperatureTrend   TrendData `json:"temperatureTrend"`
	ReallocatedTrend   TrendData `json:"reallocatedTrend"`
	PendingTrend       TrendData `json:"pendingTrend"`
	PerformanceTrend   TrendData `json:"performanceTrend"`

	// 建议
	Recommendations []string `json:"recommendations,omitempty"`
}

// TrendData 趋势数据.
type TrendData struct {
	Direction string    `json:"direction"` // up/down/stable
	Slope     float64   `json:"slope"`     // 斜率
	Current   float64   `json:"current"`   // 当前值
	Previous  float64   `json:"previous"`  // 上一个值
	Change    float64   `json:"change"`    // 变化率 (%)
	Since     time.Time `json:"since"`     // 数据起始时间
}

// HealthAlert 健康告警.
type HealthAlert struct {
	ID        string     `json:"id"`
	DiskID    string     `json:"diskId"`
	Device    string     `json:"device"`
	Level     AlertLevel `json:"level"`
	Type      string     `json:"type"`      // 告警类型
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Value     interface{} `json:"value,omitempty"`
	Threshold interface{} `json:"threshold,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	AckedAt   time.Time `json:"ackedAt,omitempty"`
	Resolved  bool      `json:"resolved"`
	ResolvedAt time.Time `json:"resolvedAt,omitempty"`
}

// DiskInfo 磁盘基本信息.
type DiskInfo struct {
	Device   string   `json:"device"`
	Model    string   `json:"model"`
	Serial   string   `json:"serial"`
	Type     DiskType `json:"type"`
	Capacity int64    `json:"capacity"`
	WWN      string   `json:"wwn,omitempty"`
	Firmware string   `json:"firmware,omitempty"`
	Transport string  `json:"transport,omitempty"` // SATA/SAS/NVMe
}

// HealthReport 健康报告.
type HealthReport struct {
	GeneratedAt time.Time    `json:"generatedAt"`
	Summary     ReportSummary `json:"summary"`
	Disks       []DiskHealth `json:"disks"`
	Alerts      []HealthAlert `json:"alerts"`
	Trends      []DiskTrend  `json:"trends"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	TotalDisks    int `json:"totalDisks"`
	HealthyDisks  int `json:"healthyDisks"`
	WarningDisks  int `json:"warningDisks"`
	CriticalDisks int `json:"criticalDisks"`
	FailedDisks   int `json:"failedDisks"`
	TotalAlerts   int `json:"totalAlerts"`
	AvgHealthScore float64 `json:"avgHealthScore"`
}

// DiskTrend 磁盘趋势.
type DiskTrend struct {
	DiskID    string    `json:"diskId"`
	Device    string    `json:"device"`
	Timestamp time.Time `json:"timestamp"`
	Score     int       `json:"score"`
	Temperature int     `json:"temperature"`
	ReallocatedSectors int64 `json:"reallocatedSectors"`
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Devices []string `json:"devices,omitempty"` // 为空则扫描所有
	Force   bool     `json:"force"`             // 强制重新扫描
}

// Response API 响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
