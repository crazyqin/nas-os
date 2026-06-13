// Package healthpredictor 提供系统健康预测与自愈功能。
// 基于时间序列的异常检测、故障预测、自动修复策略和健康报告生成。
// 对标 TrueNAS Alert System + 群晖 Active Insight 预测性维护。
package healthpredictor

import "time"

// MetricType 系统指标类型
type MetricType string

const (
	MetricCPUUsage    MetricType = "cpu_usage"
	MetricMemoryUsage MetricType = "memory_usage"
	MetricDiskUsage   MetricType = "disk_usage"
	MetricDiskTemp    MetricType = "disk_temp"
	MetricNetworkIn   MetricType = "network_in"
	MetricNetworkOut  MetricType = "network_out"
	MetricDiskIOPS    MetricType = "disk_iops"
	MetricLoadAvg1    MetricType = "load_avg_1"
	MetricLoadAvg5    MetricType = "load_avg_5"
	MetricLoadAvg15   MetricType = "load_avg_15"
)

// HealthLevel 健康等级
type HealthLevel string

const (
	HealthExcellent HealthLevel = "excellent"
	HealthGood      HealthLevel = "good"
	HealthFair      HealthLevel = "fair"
	HealthPoor      HealthLevel = "poor"
	HealthCritical  HealthLevel = "critical"
)

// AnomalyLevel 异常等级
type AnomalyLevel string

const (
	AnomalyNone     AnomalyLevel = "none"
	AnomalyWarning  AnomalyLevel = "warning"
	AnomalyCritical AnomalyLevel = "critical"
)

// PredictionType 预测类型
type PredictionType string

const (
	PredDiskFailure   PredictionType = "disk_failure"
	PredMemoryLeak    PredictionType = "memory_leak"
	PredCPUSaturation PredictionType = "cpu_saturation"
	PredDiskFull      PredictionType = "disk_full"
	PredNetworkSpike  PredictionType = "network_spike"
)

// HealActionType 修复动作类型
type HealActionType string

const (
	HealRestartService  HealActionType = "restart_service"
	HealClearCache      HealActionType = "clear_cache"
	HealKillProcess     HealActionType = "kill_process"
	HealRotateLog       HealActionType = "rotate_log"
	HealReleaseMemory   HealActionType = "release_memory"
	HealScaleUp         HealActionType = "scale_up"
)

// HealStatus 修复状态
type HealStatus string

const (
	HealPending  HealStatus = "pending"
	HealRunning  HealStatus = "running"
	HealSuccess  HealStatus = "success"
	HealFailed   HealStatus = "failed"
	HealSkipped  HealStatus = "skipped"
)

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      MetricType  `json:"type"`
	Value     float64     `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// SystemMetrics 系统指标快照
type SystemMetrics struct {
	Timestamp    time.Time     `json:"timestamp"`
	CPUUsage     float64       `json:"cpu_usage"`
	MemoryUsage  float64       `json:"memory_usage"`
	MemoryTotal  uint64        `json:"memory_total"`
	MemoryUsed   uint64        `json:"memory_used"`
	DiskUsage    float64       `json:"disk_usage"`
	DiskTotal    uint64        `json:"disk_total"`
	DiskUsed     uint64        `json:"disk_used"`
	DiskTemp     float64       `json:"disk_temp"`
	NetworkIn    float64       `json:"network_in"`
	NetworkOut   float64       `json:"network_out"`
	DiskIOPS     float64       `json:"disk_iops"`
	LoadAvg      [3]float64    `json:"load_avg"`
	TopProcesses []ProcessInfo `json:"top_processes,omitempty"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemRSS     uint64  `json:"mem_rss"`
}

// TimeSeries 时间序列
type TimeSeries struct {
	MetricType MetricType    `json:"metric_type"`
	Points     []MetricPoint `json:"points"`
	WindowSize int           `json:"window_size"`
}

// AnomalyDetectionResult 异常检测结果
type AnomalyDetectionResult struct {
	IsAnomaly   bool        `json:"is_anomaly"`
	Level       AnomalyLevel `json:"level"`
	MetricType  MetricType  `json:"metric_type"`
	Value       float64     `json:"value"`
	Expected    float64     `json:"expected"`
	StdDev      float64     `json:"std_dev"`
	Deviation   float64     `json:"deviation"`
	Description string      `json:"description"`
	Timestamp   time.Time   `json:"timestamp"`
}

// Prediction 预测结果
type Prediction struct {
	ID          string         `json:"id"`
	Type        PredictionType `json:"type"`
	Severity    HealthLevel    `json:"severity"`
	Probability float64        `json:"probability"`
	Confidence  float64        `json:"confidence"`
	MetricType  MetricType     `json:"metric_type"`
	CurrentValue float64       `json:"current_value"`
	PredictedValue float64     `json:"predicted_value"`
	TimeToImpact time.Duration `json:"time_to_impact"`
	Description string         `json:"description"`
	Suggestions []string       `json:"suggestions"`
	CreatedAt   time.Time      `json:"created_at"`
}

// HealAction 修复动作
type HealAction struct {
	ID          string         `json:"id"`
	PredictionID string        `json:"prediction_id"`
	Type        HealActionType `json:"type"`
	Target      string         `json:"target"`
	Description string         `json:"description"`
	Status      HealStatus     `json:"status"`
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// HealthReport 健康报告
type HealthReport struct {
	ID              string               `json:"id"`
	Timestamp       time.Time            `json:"timestamp"`
	OverallHealth   HealthLevel          `json:"overall_health"`
	Score           float64              `json:"score"`
	Metrics         *SystemMetrics       `json:"metrics"`
	Anomalies       []AnomalyDetectionResult `json:"anomalies,omitempty"`
	Predictions     []Prediction         `json:"predictions,omitempty"`
	ActiveHeals     []HealAction         `json:"active_heals,omitempty"`
	Recommendations []Recommendation     `json:"recommendations,omitempty"`
	Summary         string               `json:"summary"`
}

// Recommendation 优化建议
type Recommendation struct {
	ID          string       `json:"id"`
	Category    string       `json:"category"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Priority    HealthLevel  `json:"priority"`
	MetricType  MetricType   `json:"metric_type"`
	AutoHealID  string       `json:"auto_heal_id,omitempty"`
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	CPUWarning    float64 `json:"cpu_warning"`    // CPU 使用率警告阈值
	CPUCritical   float64 `json:"cpu_critical"`   // CPU 使用率危险阈值
	MemWarning    float64 `json:"mem_warning"`    // 内存使用率警告阈值
	MemCritical   float64 `json:"mem_critical"`   // 内存使用率危险阈值
	DiskWarning   float64 `json:"disk_warning"`   // 磁盘使用率警告阈值
	DiskCritical  float64 `json:"disk_critical"`  // 磁盘使用率危险阈值
	DiskTempWarning  float64 `json:"disk_temp_warning"`  // 磁盘温度警告阈值
	DiskTempCritical float64 `json:"disk_temp_critical"` // 磁盘温度危险阈值
	NetSpikePercent  float64 `json:"net_spike_percent"`  // 网络流量突增百分比
}

// DefaultThresholds 默认阈值配置
func DefaultThresholds() ThresholdConfig {
	return ThresholdConfig{
		CPUWarning:       70.0,
		CPUCritical:      90.0,
		MemWarning:       80.0,
		MemCritical:      95.0,
		DiskWarning:      80.0,
		DiskCritical:     95.0,
		DiskTempWarning:  45.0,
		DiskTempCritical: 55.0,
		NetSpikePercent:  200.0,
	}
}

// HealthPredictorConfig 配置
type HealthPredictorConfig struct {
	CollectInterval  time.Duration   `json:"collect_interval"`
	PredictInterval  time.Duration   `json:"predict_interval"`
	MaxHistorySize   int             `json:"max_history_size"`
	AnomalyWindow    int             `json:"anomaly_window"`
	AnomalySigma     float64         `json:"anomaly_sigma"`
	AutoHealEnabled  bool            `json:"auto_heal_enabled"`
	Thresholds       ThresholdConfig `json:"thresholds"`
}

// DefaultConfig 默认配置
func DefaultConfig() HealthPredictorConfig {
	return HealthPredictorConfig{
		CollectInterval: 30 * time.Second,
		PredictInterval: 5 * time.Minute,
		MaxHistorySize:  10000,
		AnomalyWindow:   100,
		AnomalySigma:    2.5,
		AutoHealEnabled: true,
		Thresholds:      DefaultThresholds(),
	}
}
