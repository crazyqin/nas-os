// Package predictivefailure 实现预测性故障分析模块
// 收集硬盘 SMART 数据、温度、读写错误率，基于历史趋势预测故障概率
// 支持内存/CPU 异常检测、风险评分、维护建议、定期扫描和实时告警
package predictivefailure

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNotRunning 引擎未运行.
	ErrNotRunning = errors.New("predictive failure engine not running")
	// ErrAlreadyRunning 引擎已在运行.
	ErrAlreadyRunning = errors.New("predictive failure engine already running")
	// ErrDiskNotFound 磁盘不存在.
	ErrDiskNotFound = errors.New("disk not found")
	// ErrPredictionNotFound 预测记录不存在.
	ErrPredictionNotFound = errors.New("prediction record not found")
	// ErrInvalidConfig 配置无效.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrScanInProgress 扫描正在进行.
	ErrScanInProgress = errors.New("scan already in progress")
)

// ========== 风险等级 ==========

// RiskLevel 风险等级.
type RiskLevel string

const (
	// RiskCritical 严重风险 (80-100).
	RiskCritical RiskLevel = "critical"
	// RiskHigh 高风险 (60-79).
	RiskHigh RiskLevel = "high"
	// RiskMedium 中等风险 (30-59).
	RiskMedium RiskLevel = "medium"
	// RiskLow 低风险 (0-29).
	RiskLow RiskLevel = "low"
)

// ========== 组件类型 ==========

// ComponentType 系统组件类型.
type ComponentType string

const (
	// ComponentDisk 磁盘.
	ComponentDisk ComponentType = "disk"
	// ComponentMemory 内存.
	ComponentMemory ComponentType = "memory"
	// ComponentCPU CPU.
	ComponentCPU ComponentType = "cpu"
)

// ========== SMART 属性 ==========

// SMARTAttribute SMART 属性.
type SMARTAttribute struct {
	// ID 属性ID.
	ID int `json:"id"`
	// Name 属性名称.
	Name string `json:"name"`
	// Value 当前值.
	Value int64 `json:"value"`
	// Worst 最差值.
	Worst int64 `json:"worst"`
	// Threshold 阈值.
	Threshold int64 `json:"threshold"`
	// RawValue 原始值.
	RawValue int64 `json:"rawValue"`
	// Flag 标志位.
	Flag string `json:"flag"`
	// Updated 更新时间.
	Updated time.Time `json:"updated"`
}

// ========== 磁盘健康数据 ==========

// DiskHealthData 磁盘健康数据.
type DiskHealthData struct {
	// Device 设备路径 (如 /dev/sda).
	Device string `json:"device"`
	// Model 型号.
	Model string `json:"model"`
	// Serial 序列号.
	Serial string `json:"serial"`
	// CapacityGB 容量(GB).
	CapacityGB float64 `json:"capacityGB"`
	// Temperature 温度(℃).
	Temperature float64 `json:"temperature"`
	// PowerOnHours 通电时长(小时).
	PowerOnHours int64 `json:"powerOnHours"`
	// ReallocatedSectors 重分配扇区数.
	ReallocatedSectors int64 `json:"reallocatedSectors"`
	// CurrentPendingSectors 当前待处理扇区.
	CurrentPendingSectors int64 `json:"currentPendingSectors"`
	// OfflineUncorrectable 离线不可纠正扇区.
	OfflineUncorrectable int64 `json:"offlineUncorrectable"`
	// UDMAErrors UDMA CRC 错误.
	UDMAErrors int64 `json:"udmaErrors"`
	// ReadErrorRate 读取错误率.
	ReadErrorRate float64 `json:"readErrorRate"`
	// SeekErrorRate 寻道错误率.
	SeekErrorRate float64 `json:"seekErrorRate"`
	// SpinRetryCount 主轴重试次数.
	SpinRetryCount int64 `json:"spinRetryCount"`
	// SMARTAttributes SMART 属性列表.
	SMARTAttributes []SMARTAttribute `json:"smartAttributes"`
	// HealthStatus 健康状态.
	HealthStatus string `json:"healthStatus"`
	// CollectedAt 采集时间.
	CollectedAt time.Time `json:"collectedAt"`
}

// ========== 系统资源数据 ==========

// SystemResourceData 系统资源数据.
type SystemResourceData struct {
	// CPUUsagePercent CPU 使用率(%).
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
	// CPUTemperature CPU 温度(℃).
	CPUTemperature float64 `json:"cpuTemperature"`
	// MemoryTotalMB 内存总量(MB).
	MemoryTotalMB float64 `json:"memoryTotalMB"`
	// MemoryUsedMB 内存已用(MB).
	MemoryUsedMB float64 `json:"memoryUsedMB"`
	// MemoryUsagePercent 内存使用率(%).
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	// SwapTotalMB Swap 总量(MB).
	SwapTotalMB float64 `json:"swapTotalMB"`
	// SwapUsedMB Swap 已用(MB).
	SwapUsedMB float64 `json:"swapUsedMB"`
	// LoadAverage1Min 1分钟负载均值.
	LoadAverage1Min float64 `json:"loadAverage1Min"`
	// LoadAverage5Min 5分钟负载均值.
	LoadAverage5Min float64 `json:"loadAverage5Min"`
	// LoadAverage15Min 15分钟负载均值.
	LoadAverage15Min float64 `json:"loadAverage15Min"`
	// CollectedAt 采集时间.
	CollectedAt time.Time `json:"collectedAt"`
}

// ========== 预测记录 ==========

// PredictionRecord 预测记录.
type PredictionRecord struct {
	// ID 记录ID.
	ID string `json:"id"`
	// ComponentType 组件类型.
	ComponentType ComponentType `json:"componentType"`
	// ComponentID 组件标识 (设备路径或 "memory"/"cpu").
	ComponentID string `json:"componentId"`
	// RiskScore 风险评分 (0-100).
	RiskScore float64 `json:"riskScore"`
	// RiskLevel 风险等级.
	RiskLevel RiskLevel `json:"riskLevel"`
	// FailureProbability 故障概率 (0-1).
	FailureProbability float64 `json:"failureProbability"`
	// PredictedFailureDate 预测故障日期（可为nil表示无法预测）.
	PredictedFailureDate *time.Time `json:"predictedFailureDate,omitempty"`
	// Factors 影响因素列表.
	Factors []RiskFactor `json:"factors"`
	// Recommendations 维护建议.
	Recommendations []Recommendation `json:"recommendations"`
	// PredictedAt 预测时间.
	PredictedAt time.Time `json:"predictedAt"`
	// ActualFailure 是否实际发生故障（用于准确率统计）.
	ActualFailure *bool `json:"actualFailure,omitempty"`
	// VerifiedAt 验证时间.
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
}

// RiskFactor 风险因素.
type RiskFactor struct {
	// Name 因素名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// Weight 权重 (0-1).
	Weight float64 `json:"weight"`
	// Score 因素评分 (0-100).
	Score float64 `json:"score"`
	// Trend 趋势 (improving/stable/degrading).
	Trend string `json:"trend"`
}

// Recommendation 维护建议.
type Recommendation struct {
	// Priority 优先级 (1-5, 1最高).
	Priority int `json:"priority"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Action 建议操作.
	Action string `json:"action"`
	// Urgency 紧急程度 (immediate/soon/routine).
	Urgency string `json:"urgency"`
}

// ========== 告警 ==========

// Alert 告警记录.
type Alert struct {
	// ID 告警ID.
	ID string `json:"id"`
	// ComponentType 组件类型.
	ComponentType ComponentType `json:"componentType"`
	// ComponentID 组件标识.
	ComponentID string `json:"componentId"`
	// Level 告警级别.
	Level RiskLevel `json:"level"`
	// Title 标题.
	Title string `json:"title"`
	// Message 消息.
	Message string `json:"message"`
	// RiskScore 风险评分.
	RiskScore float64 `json:"riskScore"`
	// Acknowledged 是否已确认.
	Acknowledged bool `json:"acknowledged"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// AcknowledgedAt 确认时间.
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

// ========== 配置 ==========

// Config 预测性故障分析配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// ScanIntervalMinutes 扫描间隔(分钟).
	ScanIntervalMinutes int `json:"scanIntervalMinutes"`
	// AlertThreshold 告警阈值(风险评分超过此值触发告警).
	AlertThreshold float64 `json:"alertThreshold"`
	// TemperatureWarnThreshold 温度告警阈值(℃).
	TemperatureWarnThreshold float64 `json:"temperatureWarnThreshold"`
	// TemperatureCriticalThreshold 温度严重阈值(℃).
	TemperatureCriticalThreshold float64 `json:"temperatureCriticalThreshold"`
	// CPUPercentWarnThreshold CPU 使用率告警阈值(%).
	CPUPercentWarnThreshold float64 `json:"cpuPercentWarnThreshold"`
	// MemoryPercentWarnThreshold 内存使用率告警阈值(%).
	MemoryPercentWarnThreshold float64 `json:"memoryPercentWarnThreshold"`
	// MaxHistoryDays 历史数据保留天数.
	MaxHistoryDays int `json:"maxHistoryDays"`
	// NotifyWebhook 告警 Webhook URL.
	NotifyWebhook string `json:"notifyWebhook"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() Config {
	return Config{
		Enabled:                     true,
		ScanIntervalMinutes:         60,
		AlertThreshold:              60,
		TemperatureWarnThreshold:    50,
		TemperatureCriticalThreshold: 60,
		CPUPercentWarnThreshold:     80,
		MemoryPercentWarnThreshold:  85,
		MaxHistoryDays:              90,
	}
}

// ========== 历史数据点 ==========

// DataPoint 时序数据点.
type DataPoint struct {
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// Value 值.
	Value float64 `json:"value"`
}

// DiskHistory 磁盘历史数据.
type DiskHistory struct {
	// Device 设备路径.
	Device string `json:"device"`
	// TemperatureHistory 温度历史.
	TemperatureHistory []DataPoint `json:"temperatureHistory"`
	// ReadErrorRateHistory 读取错误率历史.
	ReadErrorRateHistory []DataPoint `json:"readErrorRateHistory"`
	// ReallocatedSectorHistory 重分配扇区历史.
	ReallocatedSectorHistory []DataPoint `json:"reallocatedSectorHistory"`
	// PendingSectorHistory 待处理扇区历史.
	PendingSectorHistory []DataPoint `json:"pendingSectorHistory"`
}

// ========== 扫描结果 ==========

// ScanResult 扫描结果.
type ScanResult struct {
	// ID 结果ID.
	ID string `json:"id"`
	// ScanTime 扫描时间.
	ScanTime time.Time `json:"scanTime"`
	// Duration 扫描耗时.
	Duration time.Duration `json:"duration"`
	// DiskPredictions 磁盘预测列表.
	DiskPredictions []PredictionRecord `json:"diskPredictions"`
	// MemoryPrediction 内存预测.
	MemoryPrediction *PredictionRecord `json:"memoryPrediction,omitempty"`
	// CPUPrediction CPU 预测.
	CPUPrediction *PredictionRecord `json:"cpuPrediction,omitempty"`
	// Alerts 产生的告警.
	Alerts []Alert `json:"alerts"`
	// OverallRiskScore 整体风险评分.
	OverallRiskScore float64 `json:"overallRiskScore"`
	// OverallRiskLevel 整体风险等级.
	OverallRiskLevel RiskLevel `json:"overallRiskLevel"`
}

// ========== 统计数据 ==========

// PredictionStats 预测统计.
type PredictionStats struct {
	// TotalPredictions 总预测次数.
	TotalPredictions int `json:"totalPredictions"`
	// VerifiedPredictions 已验证预测数.
	VerifiedPredictions int `json:"verifiedPredictions"`
	// CorrectPredictions 正确预测数.
	CorrectPredictions int `json:"correctPredictions"`
	// Accuracy 准确率.
	Accuracy float64 `json:"accuracy"`
	// CurrentAlerts 当前未确认告警数.
	CurrentAlerts int `json:"currentAlerts"`
	// CriticalDisks 严重风险磁盘数.
	CriticalDisks int `json:"criticalDisks"`
	// AverageRiskScore 平均风险评分.
	AverageRiskScore float64 `json:"averageRiskScore"`
	// LastScanTime 上次扫描时间.
	LastScanTime time.Time `json:"lastScanTime"`
	// ScansTotal 总扫描次数.
	ScansTotal int `json:"scansTotal"`
}

// ========== 风险等级工具函数 ==========

// ScoreToRiskLevel 根据评分返回风险等级.
func ScoreToRiskLevel(score float64) RiskLevel {
	switch {
	case score >= 80:
		return RiskCritical
	case score >= 60:
		return RiskHigh
	case score >= 30:
		return RiskMedium
	default:
		return RiskLow
	}
}
