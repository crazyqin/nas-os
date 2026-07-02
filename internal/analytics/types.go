// Package analytics 提供系统分析面板功能
// 包含系统指标收集、存储分析、用户行为分析、性能分析
package analytics

import (
	"time"
)

// TimeRange 时间范围.
type TimeRange string

const (
	TimeRangeHour  TimeRange = "1h"
	TimeRangeDay   TimeRange = "24h"
	TimeRangeWeek  TimeRange = "7d"
	TimeRangeMonth TimeRange = "30d"
	TimeRangeYear  TimeRange = "1y"
)

// MetricType 指标类型.
type MetricType string

const (
	MetricTypeCPU     MetricType = "cpu"
	MetricTypeMemory  MetricType = "memory"
	MetricTypeDisk    MetricType = "disk"
	MetricTypeNetwork MetricType = "network"
	MetricTypeIOPS    MetricType = "iops"
	MetricTypeLatency MetricType = "latency"
)

// SystemMetrics 系统指标.
type SystemMetrics struct {
	Timestamp time.Time      `json:"timestamp"`
	CPU       CPUMetrics     `json:"cpu"`
	Memory    MemoryMetrics  `json:"memory"`
	Disk      DiskMetrics    `json:"disk"`
	Network   NetworkMetrics `json:"network"`
}

// CPUMetrics CPU指标.
type CPUMetrics struct {
	UsagePercent float64   `json:"usagePercent"`
	PerCore      []float64 `json:"perCore,omitempty"`
	LoadAvg1     float64   `json:"loadAvg1"`
	LoadAvg5     float64   `json:"loadAvg5"`
	LoadAvg15    float64   `json:"loadAvg15"`
	Temperature  float64   `json:"temperature,omitempty"`
	ProcessCount int       `json:"processCount"`
}

// MemoryMetrics 内存指标.
type MemoryMetrics struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	FreeBytes      uint64  `json:"freeBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
	SwapTotalBytes uint64  `json:"swapTotalBytes"`
	SwapUsedBytes  uint64  `json:"swapUsedBytes"`
	SwapUsagePct   float64 `json:"swapUsagePercent"`
	CachedBytes    uint64  `json:"cachedBytes,omitempty"`
	BuffersBytes   uint64  `json:"buffersBytes,omitempty"`
}

// DiskMetrics 磁盘指标.
type DiskMetrics struct {
	Devices []DiskDeviceMetrics `json:"devices"`
	Total   DiskSummaryMetrics  `json:"total"`
}

// DiskDeviceMetrics 磁盘设备指标.
type DiskDeviceMetrics struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mountPoint"`
	TotalBytes   uint64  `json:"totalBytes"`
	UsedBytes    uint64  `json:"usedBytes"`
	FreeBytes    uint64  `json:"freeBytes"`
	UsagePercent float64 `json:"usagePercent"`
	FSType       string  `json:"fsType"`
	ReadBytesPS  uint64  `json:"readBytesPerSec,omitempty"`
	WriteBytesPS uint64  `json:"writeBytesPerSec,omitempty"`
}

// DiskSummaryMetrics 磁盘汇总指标.
type DiskSummaryMetrics struct {
	TotalBytes   uint64  `json:"totalBytes"`
	UsedBytes    uint64  `json:"usedBytes"`
	FreeBytes    uint64  `json:"freeBytes"`
	UsagePercent float64 `json:"usagePercent"`
}

// NetworkMetrics 网络指标.
type NetworkMetrics struct {
	Interfaces []NetworkInterfaceMetrics `json:"interfaces"`
	Total      NetworkSummaryMetrics     `json:"total"`
}

// NetworkInterfaceMetrics 网络接口指标.
type NetworkInterfaceMetrics struct {
	Name        string `json:"name"`
	RXBytesPS   uint64 `json:"rxBytesPerSec"`
	TXBytesPS   uint64 `json:"txBytesPerSec"`
	RXPacketsPS uint64 `json:"rxPacketsPerSec,omitempty"`
	TXPacketsPS uint64 `json:"txPacketsPerSec,omitempty"`
	RXErrors    uint64 `json:"rxErrors,omitempty"`
	TXErrors    uint64 `json:"txErrors,omitempty"`
	Speed       uint64 `json:"speed,omitempty"`
}

// NetworkSummaryMetrics 网络汇总指标.
type NetworkSummaryMetrics struct {
	TotalRXBytesPS uint64 `json:"totalRxBytesPerSec"`
	TotalTXBytesPS uint64 `json:"totalTxBytesPerSec"`
	TotalRXPackets uint64 `json:"totalRxPackets"`
	TotalTXPackets uint64 `json:"totalTxPackets"`
}

// StorageAnalytics 存储分析.
type StorageAnalytics struct {
	Timestamp         time.Time              `json:"timestamp"`
	TotalCapacity     uint64                 `json:"totalCapacity"`
	UsedCapacity      uint64                 `json:"usedCapacity"`
	AvailableCapacity uint64                 `json:"availableCapacity"`
	UsagePercent      float64                `json:"usagePercent"`
	FileTypeDist      []FileTypeDistribution `json:"fileTypeDistribution"`
	GrowthTrend       []StorageGrowthPoint   `json:"growthTrend"`
	GrowthPrediction  *GrowthPrediction      `json:"growthPrediction,omitempty"`
	TopDirectories    []DirectoryUsage       `json:"topDirectories,omitempty"`
}

// FileTypeDistribution 文件类型分布.
type FileTypeDistribution struct {
	Category   string   `json:"category"`
	Extensions []string `json:"extensions,omitempty"`
	FileCount  int64    `json:"fileCount"`
	TotalBytes uint64   `json:"totalBytes"`
	Percent    float64  `json:"percent"`
}

// StorageGrowthPoint 存储增长点.
type StorageGrowthPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	TotalBytes uint64    `json:"totalBytes"`
	UsedBytes  uint64    `json:"usedBytes"`
	FileCount  int64     `json:"fileCount"`
}

// GrowthPrediction 增长预测.
type GrowthPrediction struct {
	DaysToFull        int       `json:"daysToFull"`
	PredictedFullDate time.Time `json:"predictedFullDate"`
	DailyGrowthRate   float64   `json:"dailyGrowthRateBytes"`
	Confidence        float64   `json:"confidence"`
	Methodology       string    `json:"methodology"`
}

// DirectoryUsage 目录使用情况.
type DirectoryUsage struct {
	Path       string  `json:"path"`
	TotalBytes uint64  `json:"totalBytes"`
	FileCount  int64   `json:"fileCount"`
	Percent    float64 `json:"percent"`
}

// UserBehavior 用户行为分析.
type UserBehavior struct {
	Timestamp      time.Time         `json:"timestamp"`
	AccessPatterns []AccessPattern   `json:"accessPatterns"`
	HotFiles       []HotFile         `json:"hotFiles"`
	UsageTrend     []UsageTrendPoint `json:"usageTrend"`
	UserActivity   []UserActivity    `json:"userActivity"`
}

// AccessPattern 访问模式.
type AccessPattern struct {
	Hour         int    `json:"hour"`
	DayOfWeek    int    `json:"dayOfWeek"`
	AccessCount  int64  `json:"accessCount"`
	BytesRead    uint64 `json:"bytesRead"`
	BytesWritten uint64 `json:"bytesWritten"`
}

// HotFile 热门文件.
type HotFile struct {
	Path         string    `json:"path"`
	AccessCount  int64     `json:"accessCount"`
	LastAccessed time.Time `json:"lastAccessed"`
	TotalBytes   uint64    `json:"totalBytes"`
	Users        []string  `json:"users,omitempty"`
}

// UsageTrendPoint 使用趋势点.
type UsageTrendPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	ActiveUsers  int       `json:"activeUsers"`
	AccessCount  int64     `json:"accessCount"`
	DataTransfer uint64    `json:"dataTransfer"`
}

// UserActivity 用户活动.
type UserActivity struct {
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	AccessCount  int64     `json:"accessCount"`
	BytesRead    uint64    `json:"bytesRead"`
	BytesWritten uint64    `json:"bytesWritten"`
	LastActive   time.Time `json:"lastActive"`
	TopFiles     []string  `json:"topFiles,omitempty"`
}

// PerformanceMetrics 性能指标.
type PerformanceMetrics struct {
	Timestamp  time.Time         `json:"timestamp"`
	IOPS       IOPSMetrics       `json:"iops"`
	Latency    LatencyMetrics    `json:"latency"`
	Throughput ThroughputMetrics `json:"throughput"`
}

// IOPSMetrics IOPS指标.
type IOPSMetrics struct {
	ReadIOPS  float64     `json:"readIOPS"`
	WriteIOPS float64     `json:"writeIOPS"`
	TotalIOPS float64     `json:"totalIOPS"`
	Trend     []IOPSPoint `json:"trend,omitempty"`
}

// IOPSPoint IOPS趋势点.
type IOPSPoint struct {
	Timestamp time.Time `json:"timestamp"`
	ReadIOPS  float64   `json:"readIOPS"`
	WriteIOPS float64   `json:"writeIOPS"`
}

// LatencyMetrics 延迟指标.
type LatencyMetrics struct {
	ReadLatencyAvg  float64        `json:"readLatencyAvgMs"`
	ReadLatencyP50  float64        `json:"readLatencyP50Ms"`
	ReadLatencyP99  float64        `json:"readLatencyP99Ms"`
	WriteLatencyAvg float64        `json:"writeLatencyAvgMs"`
	WriteLatencyP50 float64        `json:"writeLatencyP50Ms"`
	WriteLatencyP99 float64        `json:"writeLatencyP99Ms"`
	Trend           []LatencyPoint `json:"trend,omitempty"`
}

// LatencyPoint 延迟趋势点.
type LatencyPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	ReadLatencyAvg  float64   `json:"readLatencyAvgMs"`
	WriteLatencyAvg float64   `json:"writeLatencyAvgMs"`
}

// ThroughputMetrics 吞吐量指标.
type ThroughputMetrics struct {
	ReadBytesPS  uint64            `json:"readBytesPerSec"`
	WriteBytesPS uint64            `json:"writeBytesPerSec"`
	TotalBytesPS uint64            `json:"totalBytesPerSec"`
	Trend        []ThroughputPoint `json:"trend,omitempty"`
}

// ThroughputPoint 吞吐量趋势点.
type ThroughputPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	ReadBytesPS  uint64    `json:"readBytesPerSec"`
	WriteBytesPS uint64    `json:"writeBytesPerSec"`
}

// AnalyticsSummary 分析摘要.
type AnalyticsSummary struct {
	Timestamp     time.Time         `json:"timestamp"`
	SystemHealth  HealthStatus      `json:"systemHealth"`
	StorageStatus StorageStatus     `json:"storageStatus"`
	Performance   PerformanceStatus `json:"performance"`
	Alerts        []AnalyticsAlert  `json:"alerts,omitempty"`
}

// HealthStatus 健康状态.
type HealthStatus struct {
	Status    string  `json:"status"`
	Score     float64 `json:"score"`
	CPUUsage  float64 `json:"cpuUsage"`
	MemUsage  float64 `json:"memUsage"`
	DiskUsage float64 `json:"diskUsage"`
}

// StorageStatus 存储状态.
type StorageStatus struct {
	Status        string  `json:"status"`
	TotalCapacity uint64  `json:"totalCapacity"`
	UsedCapacity  uint64  `json:"usedCapacity"`
	UsagePercent  float64 `json:"usagePercent"`
	DaysUntilFull int     `json:"daysUntilFull,omitempty"`
}

// PerformanceStatus 性能状态.
type PerformanceStatus struct {
	Status       string  `json:"status"`
	TotalIOPS    float64 `json:"totalIOPS"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	ThroughputMB float64 `json:"throughputMBps"`
}

// AnalyticsAlert 分析告警.
type AnalyticsAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value,omitempty"`
	Threshold float64   `json:"threshold,omitempty"`
}

// MetricsHistory 指标历史.
type MetricsHistory struct {
	Type      MetricType    `json:"type"`
	Range     TimeRange     `json:"range"`
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	Points    []interface{} `json:"points"`
}

// CollectorConfig 采集器配置.
type CollectorConfig struct {
	Interval      time.Duration `json:"interval"`
	HistorySize   int           `json:"historySize"`
	EnableCPU     bool          `json:"enableCpu"`
	EnableMemory  bool          `json:"enableMemory"`
	EnableDisk    bool          `json:"enableDisk"`
	EnableNetwork bool          `json:"enableNetwork"`
}

// DefaultCollectorConfig 默认采集器配置.
func DefaultCollectorConfig() CollectorConfig {
	return CollectorConfig{
		Interval:      30 * time.Second,
		HistorySize:   1000,
		EnableCPU:     true,
		EnableMemory:  true,
		EnableDisk:    true,
		EnableNetwork: true,
	}
}
