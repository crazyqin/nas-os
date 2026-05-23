// Package diskhealth 提供 SMART 磁盘健康监测和故障预测功能
package diskhealth

import (
	"time"
)

// RiskLevel 风险等级
type RiskLevel string

const (
	// RiskCritical 严重风险，磁盘可能即将故障
	RiskCritical RiskLevel = "Critical"
	// RiskWarning 警告风险，磁盘存在潜在问题
	RiskWarning RiskLevel = "Warning"
	// RiskNormal 正常，磁盘有轻微异常但不影响使用
	RiskNormal RiskLevel = "Normal"
	// RiskHealthy 健康，磁盘状态良好
	RiskHealthy RiskLevel = "Healthy"
)

// DiskHealthStatus 磁盘健康状态
type DiskHealthStatus struct {
	// Device 设备名称（如 sda, nvme0n1）
	Device string `json:"device"`
	// Model 磁盘型号
	Model string `json:"model"`
	// Serial 磁盘序列号
	Serial string `json:"serial"`
	// Capacity 磁盘容量（字节）
	Capacity uint64 `json:"capacity"`
	// HealthScore 健康评分 0-100
	HealthScore int `json:"health_score"`
	// RiskLevel 风险等级
	RiskLevel RiskLevel `json:"risk_level"`
	// SmartAttributes SMART 属性列表
	SmartAttributes []SmartAttribute `json:"smart_attributes"`
	// PredictedLifeDays 预测剩余寿命（天）
	PredictedLifeDays int `json:"predicted_life_days"`
	// PredictedFailureDate 预测故障日期
	PredictedFailureDate *time.Time `json:"predicted_failure_date,omitempty"`
	// LastScanTime 最后扫描时间
	LastScanTime time.Time `json:"last_scan_time"`
	// IsSMARTEnabled SMART 是否启用
	IsSMARTEnabled bool `json:"is_smart_enabled"`
	// Temperature 当前温度（摄氏度）
	Temperature int `json:"temperature"`
	// PowerOnHours 通电时间（小时）
	PowerOnHours uint64 `json:"power_on_hours"`
	// WarningMessage 警告信息
	WarningMessage string `json:"warning_message,omitempty"`
}

// SmartAttribute SMART 属性
type SmartAttribute struct {
	// ID 属性 ID
	ID int `json:"id"`
	// Name 属性名称
	Name string `json:"name"`
	// Value 当前值（0-253，值越低越差）
	Value int `json:"value"`
	// Worst 历史最差值
	Worst int `json:"worst"`
	// Threshold 阈值（低于此值视为异常）
	Threshold int `json:"threshold"`
	// RawValue 原始值
	RawValue uint64 `json:"raw_value"`
	// IsCritical 是否为关键属性
	IsCritical bool `json:"is_critical"`
	// IsFailed 是否已失败
	IsFailed bool `json:"is_failed"`
}

// HealthHistory 健康历史记录
type HealthHistory struct {
	// Device 设备名称
	Device string `json:"device"`
	// Records 历史记录
	Records []HealthRecord `json:"records"`
	// TrendScore 健康评分趋势（负值表示下降）
	TrendScore float64 `json:"trend_score"`
	// TrendTemperature 温度趋势
	TrendTemperature float64 `json:"trend_temperature"`
}

// HealthRecord 健康记录点
type HealthRecord struct {
	// Timestamp 记录时间
	Timestamp time.Time `json:"timestamp"`
	// HealthScore 当时的健康评分
	HealthScore int `json:"health_score"`
	// Temperature 当时的温度
	Temperature int `json:"temperature"`
	// ReallocatedSectors 重分配扇区数
	ReallocatedSectors uint64 `json:"reallocated_sectors"`
	// CurrentPendingSectors 当前待处理扇区数
	CurrentPendingSectors uint64 `json:"current_pending_sectors"`
	// OfflineUncorrectable 离线不可纠正扇区数
	OfflineUncorrectable uint64 `json:"offline_uncorrectable"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	// Devices 指定设备列表（为空则扫描所有）
	Devices []string `json:"devices,omitempty"`
	// Force 是否强制重新扫描
	Force bool `json:"force"`
}

// ScanResponse 扫描响应
type ScanResponse struct {
	// Status 扫描状态
	Status string `json:"status"`
	// Message 消息
	Message string `json:"message"`
	// DevicesScanned 扫描的设备数
	DevicesScanned int `json:"devices_scanned"`
}

// DiskHealthResponse 磁盘健康 API 响应
type DiskHealthResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Message 消息
	Message string `json:"message"`
	// Data 数据
	Data interface{} `json:"data,omitempty"`
}

// LinearRegressionResult 线性回归结果
type LinearRegressionResult struct {
	// Slope 斜率（每天的变化量）
	Slope float64
	// Intercept 截距
	Intercept float64
	// RSquared R² 决定系数
	RSquared float64
	// PredictedValue 预测值
	PredictedValue float64
}
