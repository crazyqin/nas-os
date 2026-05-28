// Package diskpredict 提供磁盘故障预测功能，基于 SMART 数据分析
package diskpredict

import (
	"time"
)

// DiskStatus 磁盘状态
type DiskStatus string

const (
	StatusHealthy  DiskStatus = "healthy"  // 健康
	StatusWarning  DiskStatus = "warning"  // 警告
	StatusCritical DiskStatus = "critical" // 临界
	StatusFailed   DiskStatus = "failed"   // 已失败
)

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device       string     `json:"device"`        // 设备名称，如 sda, nvme0n1
	Model        string     `json:"model"`         // 磁盘型号
	Serial       string     `json:"serial"`        // 序列号
	Capacity     uint64     `json:"capacity"`      // 容量（字节）
	Status       DiskStatus `json:"status"`        // 当前状态
	SMARTEnabled bool       `json:"smart_enabled"` // SMART 是否启用
	RegisteredAt time.Time  `json:"registered_at"` // 注册时间
	LastScanAt   time.Time  `json:"last_scan_at"`  // 最后扫描时间
}

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID         int    `json:"id"`          // 属性 ID
	Name       string `json:"name"`        // 属性名称
	Value      int    `json:"value"`       // 当前值（0-253）
	Worst      int    `json:"worst"`       // 历史最差值
	Threshold  int    `json:"threshold"`   // 阈值
	RawValue   uint64 `json:"raw_value"`   // 原始值
	IsCritical bool   `json:"is_critical"` // 是否为关键指标
	IsFailed   bool   `json:"is_failed"`   // 是否失败
}

// SMARTData SMART 数据集合
type SMARTData struct {
	Device         string           `json:"device"`
	Model          string           `json:"model"`
	Serial         string           `json:"serial"`
	Temperature    int              `json:"temperature"`     // 当前温度（摄氏度）
	PowerOnHours   uint64           `json:"power_on_hours"`  // 通电时间（小时）
	PowerCycleCount uint64          `json:"power_cycle_count"` // 通电周期数
	Attributes     []SMARTAttribute `json:"attributes"`      // SMART 属性列表
	CollectedAt    time.Time        `json:"collected_at"`    // 采集时间
}

// PredictionResult 预测结果
type PredictionResult struct {
	Device             string    `json:"device"`               // 设备名称
	Model              string    `json:"model"`                // 磁盘型号
	Serial             string    `json:"serial"`               // 序列号
	HealthScore        float64   `json:"health_score"`         // 健康评分（0-100）
	Status             DiskStatus `json:"status"`              // 状态
	EstimatedLifeDays  int       `json:"estimated_life_days"`  // 预测剩余寿命（天）
	EstimatedFailDate  *time.Time `json:"estimated_fail_date,omitempty"` // 预测故障日期
	RiskFactors        []string  `json:"risk_factors"`         // 风险因素
	AnalyzedAttributes []AttributeAnalysis `json:"analyzed_attributes"` // 分析的属性
	PredictedAt        time.Time `json:"predicted_at"`         // 预测时间
}

// AttributeAnalysis 属性分析结果
type AttributeAnalysis struct {
	ID          int     `json:"id"`           // 属性 ID
	Name        string  `json:"name"`         // 属性名称
	Value       int     `json:"value"`        // 当前值
	Threshold   int     `json:"threshold"`    // 阈值
	Score       float64 `json:"score"`        // 该属性得分（0-100）
	Weight      float64 `json:"weight"`       // 权重
	WeightedScore float64 `json:"weighted_score"` // 加权得分
	Status      string  `json:"status"`       // 状态（normal, warning, critical）
	Message     string  `json:"message"`      // 状态消息
}

// PredictRequest 预测请求
type PredictRequest struct {
	Device string `json:"device"` // 设备名称，为空则预测所有磁盘
}

// PredictResponse 预测响应
type PredictResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// DiskListResponse 磁盘列表响应
type DiskListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []DiskInfo  `json:"data,omitempty"`
}

// PredictionListResponse 预测列表响应
type PredictionListResponse struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    []PredictionResult `json:"data,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    *DiskStats  `json:"data,omitempty"`
}

// DiskStats 磁盘统计信息
type DiskStats struct {
	TotalDisks     int `json:"total_disks"`     // 总磁盘数
	HealthyDisks   int `json:"healthy_disks"`   // 健康磁盘数
	WarningDisks   int `json:"warning_disks"`   // 警告磁盘数
	CriticalDisks  int `json:"critical_disks"`  // 临界磁盘数
	FailedDisks    int `json:"failed_disks"`    // 失败磁盘数
	AvgHealthScore float64 `json:"avg_health_score"` // 平均健康评分
}

// AlertInfo 告警信息
type AlertInfo struct {
	Device    string    `json:"device"`     // 设备名称
	Level     string    `json:"level"`      // 告警级别
	Message   string    `json:"message"`    // 告警消息
	CreatedAt time.Time `json:"created_at"` // 创建时间
	Resolved  bool      `json:"resolved"`   // 是否已解决
}

// AlertListResponse 告警列表响应
type AlertListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []AlertInfo `json:"data,omitempty"`
}
