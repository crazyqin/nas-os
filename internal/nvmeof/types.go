// Package nvmeof - NVMe Health Monitoring Enhancement Types
// 温度监控、寿命预测、性能基准测试 - 参考TrueNAS 25.10 NVMe Optimizations
package nvmeof

import (
	"time"
)

// ============================================================
// 温度监控类型
// ============================================================

// TemperatureReading 单次温度读数
type TemperatureReading struct {
	Device       string    `json:"device"`        // 设备路径 e.g. /dev/nvme0n1
	SubsystemNQN string    `json:"subsystem_nqn"` // 所属子系统
	Temperature  float64   `json:"temperature"`   // 当前温度 (°C)
	Timestamp    time.Time `json:"timestamp"`
}

// TemperatureConfig 温度监控配置
type TemperatureConfig struct {
	Enabled           bool    `json:"enabled"`
	IntervalSec       int     `json:"interval_sec"`       // 采集间隔 (秒), 默认 60
	WarningThreshold  float64 `json:"warning_threshold"`  // 警告阈值 (°C), 默认 70
	CriticalThreshold float64 `json:"critical_threshold"` // 严重阈值 (°C), 默认 85
	MaxHistoryLen     int     `json:"max_history_len"`    // 最大历史记录数, 默认 1440 (24h)
}

// DefaultTemperatureConfig 默认温度监控配置
func DefaultTemperatureConfig() TemperatureConfig {
	return TemperatureConfig{
		Enabled:           true,
		IntervalSec:       60,
		WarningThreshold:  70,
		CriticalThreshold: 85,
		MaxHistoryLen:     1440,
	}
}

// TemperatureAlert 温度告警
type TemperatureAlert struct {
	Device       string    `json:"device"`
	SubsystemNQN string    `json:"subsystem_nqn"`
	Temperature  float64   `json:"temperature"`
	Threshold    float64   `json:"threshold"`
	Level        string    `json:"level"` // "warning", "critical"
	Timestamp    time.Time `json:"timestamp"`
}

// DeviceTemperatureStatus 设备温度状态汇总
type DeviceTemperatureStatus struct {
	Device       string               `json:"device"`
	SubsystemNQN string               `json:"subsystem_nqn"`
	CurrentTemp  float64              `json:"current_temp"`
	MinTemp      float64              `json:"min_temp"`
	MaxTemp      float64              `json:"max_temp"`
	AvgTemp      float64              `json:"avg_temp"`
	Status       string               `json:"status"` // "normal", "warning", "critical"
	History      []TemperatureReading `json:"history,omitempty"`
	AlertCount   int                  `json:"alert_count"`
	LastAlert    *TemperatureAlert    `json:"last_alert,omitempty"`
	LastUpdated  time.Time            `json:"last_updated"`
}

// ============================================================
// 寿命预测类型
// ============================================================

// LifePredictionConfig 寿命预测配置
type LifePredictionConfig struct {
	Enabled               bool    `json:"enabled"`
	TempDegradationRate   float64 `json:"temp_degradation_rate"`   // 每超过阈值1°C的寿命缩减率, 默认 0.02
	WriteDegradationRate  float64 `json:"write_degradation_rate"`  // 每1% TBW的寿命缩减率, 默认 1.0
	MaxWriteAmplification float64 `json:"max_write_amplification"` // 最大写放大因子, 默认 3.0
}

// DefaultLifePredictionConfig 默认寿命预测配置
func DefaultLifePredictionConfig() LifePredictionConfig {
	return LifePredictionConfig{
		Enabled:               true,
		TempDegradationRate:   0.02,
		WriteDegradationRate:  1.0,
		MaxWriteAmplification: 3.0,
	}
}

// DeviceLifePrediction 设备寿命预测结果
type DeviceLifePrediction struct {
	Device       string `json:"device"`
	SubsystemNQN string `json:"subsystem_nqn"`

	// 基础信息
	Model                string  `json:"model"`
	Serial               string  `json:"serial"`
	TotalWriteCapacityTB float64 `json:"total_write_capacity_tb"` // 厂商TBW容量
	TotalWrittenTB       float64 `json:"total_written_tb"`        // 已写入TB

	// 预测结果
	RemainingLifePercent float64   `json:"remaining_life_percent"` // 0-100
	EstimatedDaysLeft    int       `json:"estimated_days_left"`
	EstimatedEndDate     time.Time `json:"estimated_end_date"`
	ConfidenceLevel      string    `json:"confidence_level"` // "high", "medium", "low"

	// 影响因子
	WriteAmplification float64 `json:"write_amplification"`
	DailyWriteRateGB   float64 `json:"daily_write_rate_gb"`
	TempDegradation    float64 `json:"temp_degradation"`  // 0-1
	WriteDegradation   float64 `json:"write_degradation"` // 0-1
	WearLevel          string  `json:"wear_level"`        // "low", "medium", "high", "critical"

	// SMART数据
	PercentageUsed  int    `json:"percentage_used"`
	AvailableSpare  int    `json:"available_spare"`
	PowerOnHours    uint64 `json:"power_on_hours"`
	UnsafeShutdowns uint64 `json:"unsafe_shutdowns"`
	MediaErrors     uint64 `json:"media_errors"`

	PredictedAt time.Time `json:"predicted_at"`
}

// WritePattern 写入模式分析
type WritePattern struct {
	Device             string    `json:"device"`
	SubsystemNQN       string    `json:"subsystem_nqn"`
	DailyWriteAvgGB    float64   `json:"daily_write_avg_gb"`
	WeeklyWriteAvgGB   float64   `json:"weekly_write_avg_gb"`
	PeakWriteRateGBps  float64   `json:"peak_write_rate_gbps"`
	WriteAmplification float64   `json:"write_amplification"`
	TotalWriteTB       float64   `json:"total_write_tb"`
	TotalReadTB        float64   `json:"total_read_tb"`
	SamplePeriodDays   int       `json:"sample_period_days"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ============================================================
// 性能基准测试类型
// ============================================================

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	DevicePath   string   `json:"device_path"`
	SubsystemNQN string   `json:"subsystem_nqn"`
	BlockSizeKB  int      `json:"block_size_kb"` // 块大小 (KB), 默认 64
	FileSizeMB   int      `json:"file_size_mb"`  // 测试文件大小 (MB), 默认 256
	DurationSec  int      `json:"duration_sec"`  // 测试持续时间 (秒), 默认 30
	NumThreads   int      `json:"num_threads"`   // 并发线程数, 默认 1
	TestTypes    []string `json:"test_types"`    // "seq_read", "seq_write", "rand_read", "rand_write"
}

// DefaultBenchmarkConfig 默认基准测试配置
func DefaultBenchmarkConfig(devicePath, subsystemNQN string) BenchmarkConfig {
	return BenchmarkConfig{
		DevicePath:   devicePath,
		SubsystemNQN: subsystemNQN,
		BlockSizeKB:  64,
		FileSizeMB:   256,
		DurationSec:  30,
		NumThreads:   1,
		TestTypes:    []string{"seq_read", "seq_write", "rand_read", "rand_write"},
	}
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	ID          string            `json:"id"`
	Config      BenchmarkConfig   `json:"config"`
	Status      string            `json:"status"` // "pending", "running", "completed", "failed"
	Results     *BenchmarkMetrics `json:"results,omitempty"`
	Error       string            `json:"error,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Duration    time.Duration     `json:"duration"`
}

// BenchmarkMetrics 基准测试指标
type BenchmarkMetrics struct {
	// 顺序读
	SeqReadMBps float64 `json:"seq_read_mbps"`
	// 顺序写
	SeqWriteMBps float64 `json:"seq_write_mbps"`
	// 随机读 4K
	RandomReadIOPS float64 `json:"random_read_iops"`
	// 随机写 4K
	RandomWriteIOPS float64 `json:"random_write_iops"`
	// 延迟
	LatencyAvgMs float64 `json:"latency_avg_ms"`
	LatencyP50Ms float64 `json:"latency_p50_ms"`
	LatencyP95Ms float64 `json:"latency_p95_ms"`
	LatencyP99Ms float64 `json:"latency_p99_ms"`
	// 吞吐量
	TotalThroughputMBps float64 `json:"total_throughput_mbps"`
	// 综合评分 (0-100)
	OverallScore float64 `json:"overall_score"`
}
