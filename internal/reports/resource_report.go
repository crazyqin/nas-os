// Package reports 提供报表生成和管理功能
package reports

import (
	"time"
)

// ========== 资源可视化报告 v2.30.0 ==========

// ResourceReportType 资源报告类型
type ResourceReportType string

const (
	ResourceReportOverview    ResourceReportType = "overview"    // 总览报告
	ResourceReportStorage     ResourceReportType = "storage"     // 存储报告
	ResourceReportBandwidth   ResourceReportType = "bandwidth"   // 带宽报告
	ResourceReportUser        ResourceReportType = "user"        // 用户报告
	ResourceReportCapacity    ResourceReportType = "capacity"    // 容量报告
	ResourceReportPerformance ResourceReportType = "performance" // 性能报告
)

// ResourceVisualizationReport 资源可视化报告
type ResourceVisualizationReport struct {
	// 报告ID
	ID string `json:"id"`

	// 报告类型
	Type ResourceReportType `json:"type"`

	// 报告名称
	Name string `json:"name"`

	// 报告周期
	Period ReportPeriod `json:"period"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 存储概览
	StorageOverview *StorageOverview `json:"storage_overview,omitempty"`

	// 带宽概览
	BandwidthOverview *BandwidthOverview `json:"bandwidth_overview,omitempty"`

	// 用户资源概览
	UserOverview *UserResourceOverview `json:"user_overview,omitempty"`

	// 系统资源概览
	SystemOverview *SystemResourceOverview `json:"system_overview,omitempty"`

	// 图表数据
	Charts []ChartData `json:"charts"`

	// 建议
	Recommendations []ResourceRecommendation `json:"recommendations"`

	// 告警
	Alerts []ResourceAlert `json:"alerts"`
}

// StorageOverview 存储概览
type StorageOverview struct {
	// 总容量（字节）
	TotalCapacity uint64 `json:"total_capacity"`

	// 已使用（字节）
	UsedCapacity uint64 `json:"used_capacity"`

	// 可用空间（字节）
	AvailableCapacity uint64 `json:"available_capacity"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 总容量（人类可读）
	TotalCapacityHR string `json:"total_capacity_hr"`

	// 已使用（人类可读）
	UsedCapacityHR string `json:"used_capacity_hr"`

	// 可用空间（人类可读）
	AvailableCapacityHR string `json:"available_capacity_hr"`

	// 卷数量
	VolumeCount int `json:"volume_count"`

	// 卷详情
	Volumes []VolumeStorageInfo `json:"volumes"`

	// 文件数量
	TotalFiles uint64 `json:"total_files"`

	// 目录数量
	TotalDirectories uint64 `json:"total_directories"`

	// 存储效率
	StorageEfficiency StorageEfficiency `json:"storage_efficiency"`

	// 存储类型分布
	TypeDistribution StorageTypeDistribution `json:"type_distribution"`
}

// VolumeStorageInfo 卷存储信息
type VolumeStorageInfo struct {
	// 卷名称
	Name string `json:"name"`

	// UUID
	UUID string `json:"uuid"`

	// 挂载点
	MountPoint string `json:"mount_point"`

	// 总容量（字节）
	TotalCapacity uint64 `json:"total_capacity"`

	// 已使用（字节）
	UsedCapacity uint64 `json:"used_capacity"`

	// 可用空间（字节）
	AvailableCapacity uint64 `json:"available_capacity"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// RAID 配置
	RAIDProfile string `json:"raid_profile"`

	// 健康状态
	Healthy bool `json:"healthy"`

	// 状态
	Status string `json:"status"` // online, offline, degraded, rebuilding

	// 子卷数量
	SubvolumeCount int `json:"subvolume_count"`

	// 快照数量
	SnapshotCount int `json:"snapshot_count"`

	// IOPS
	IOPS uint64 `json:"iops"`

	// 读写带宽
	ReadBandwidth  uint64 `json:"read_bandwidth"`  // 字节/秒
	WriteBandwidth uint64 `json:"write_bandwidth"` // 字节/秒

	// 历史趋势
	Trend []StorageTrendPoint `json:"trend,omitempty"`
}

// StorageTrendPoint 存储趋势数据点
type StorageTrendPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedCapacity uint64    `json:"used_capacity"`
	UsagePercent float64   `json:"usage_percent"`
	IOPS         uint64    `json:"iops"`
	ReadBps      uint64    `json:"read_bps"`
	WriteBps     uint64    `json:"write_bps"`
}

// StorageEfficiency 存储效率
type StorageEfficiency struct {
	// 压缩率（%）
	CompressionRatio float64 `json:"compression_ratio"`

	// 去重率（%）
	DedupRatio float64 `json:"dedup_ratio"`

	// 实际数据量（字节）
	ActualDataSize uint64 `json:"actual_data_size"`

	// 物理占用（字节）
	PhysicalSize uint64 `json:"physical_size"`

	// 节省空间（字节）
	SavedSpace uint64 `json:"saved_space"`
}

// StorageTypeDistribution 存储类型分布
type StorageTypeDistribution struct {
	// 按文件类型
	ByFileType []FileTypeDistribution `json:"by_file_type"`

	// 按用户
	ByUser []UserStorageDistribution `json:"by_user"`

	// 按目录
	ByDirectory []DirectoryDistribution `json:"by_directory"`

	// 按时间（文件创建时间）
	ByAge []AgeDistribution `json:"by_age"`
}

// FileTypeDistribution 文件类型分布
type FileTypeDistribution struct {
	// 文件类型
	Type string `json:"type"` // document, image, video, audio, archive, other

	// 文件数量
	Count uint64 `json:"count"`

	// 总大小（字节）
	Size uint64 `json:"size"`

	// 占比（%）
	Percent float64 `json:"percent"`
}

// UserStorageDistribution 用户存储分布
type UserStorageDistribution struct {
	// 用户名
	Username string `json:"username"`

	// 使用量（字节）
	UsedBytes uint64 `json:"used_bytes"`

	// 配额限制（字节）
	QuotaBytes uint64 `json:"quota_bytes"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 文件数量
	FileCount uint64 `json:"file_count"`
}

// DirectoryDistribution 目录分布
type DirectoryDistribution struct {
	// 路径
	Path string `json:"path"`

	// 大小（字节）
	Size uint64 `json:"size"`

	// 文件数量
	FileCount uint64 `json:"file_count"`

	// 占比（%）
	Percent float64 `json:"percent"`
}

// AgeDistribution 年龄分布
type AgeDistribution struct {
	// 年龄段
	AgeRange string `json:"age_range"` // 0-7days, 7-30days, 30-90days, 90-365days, 365+days

	// 文件数量
	Count uint64 `json:"count"`

	// 总大小（字节）
	Size uint64 `json:"size"`

	// 占比（%）
	Percent float64 `json:"percent"`
}

// BandwidthOverview 带宽概览
type BandwidthOverview struct {
	// 当前总速率
	CurrentRate BandwidthRate `json:"current_rate"`

	// 接口数量
	InterfaceCount int `json:"interface_count"`

	// 接口详情
	Interfaces []InterfaceBandwidthInfo `json:"interfaces"`

	// 汇总统计
	Summary BandwidthSummaryInfo `json:"summary"`

	// 趋势数据
	Trend []BandwidthTrendPoint `json:"trend"`

	// 峰值信息
	Peak BandwidthPeakInfo `json:"peak"`

	// 流量模式
	TrafficPattern string `json:"traffic_pattern"` // balanced, download_heavy, upload_heavy
}

// BandwidthRate 带宽速率
type BandwidthRate struct {
	// 接收速率（字节/秒）
	RxBytesPerSec uint64 `json:"rx_bytes_per_sec"`

	// 发送速率（字节/秒）
	TxBytesPerSec uint64 `json:"tx_bytes_per_sec"`

	// 总速率（字节/秒）
	TotalBytesPerSec uint64 `json:"total_bytes_per_sec"`

	// 接收速率（Mbps）
	RxMbps float64 `json:"rx_mbps"`

	// 发送速率（Mbps）
	TxMbps float64 `json:"tx_mbps"`

	// 总速率（Mbps）
	TotalMbps float64 `json:"total_mbps"`
}

// InterfaceBandwidthInfo 接口带宽信息
type InterfaceBandwidthInfo struct {
	// 接口名称
	Name string `json:"name"`

	// MAC地址
	MACAddress string `json:"mac_address"`

	// IP地址
	IPAddress string `json:"ip_address"`

	// 状态
	Status string `json:"status"` // up, down

	// 速率
	Rate BandwidthRate `json:"rate"`

	// 累计流量
	TotalRx uint64 `json:"total_rx"` // 累计接收（字节）
	TotalTx uint64 `json:"total_tx"` // 累计发送（字节）

	// 错误和丢包
	Errors  uint64 `json:"errors"`
	Dropped uint64 `json:"dropped"`

	// 利用率（%）
	Utilization float64 `json:"utilization"`

	// 带宽限制（Mbps）
	BandwidthLimit float64 `json:"bandwidth_limit"`
}

// BandwidthSummaryInfo 带宽汇总信息
type BandwidthSummaryInfo struct {
	// 周期内总接收（字节）
	TotalRxBytes uint64 `json:"total_rx_bytes"`

	// 周期内总发送（字节）
	TotalTxBytes uint64 `json:"total_tx_bytes"`

	// 周期内总流量（字节）
	TotalBytes uint64 `json:"total_bytes"`

	// 平均接收速率（Mbps）
	AvgRxMbps float64 `json:"avg_rx_mbps"`

	// 平均发送速率（Mbps）
	AvgTxMbps float64 `json:"avg_tx_mbps"`

	// 总接收（GB）
	TotalRxGB float64 `json:"total_rx_gb"`

	// 总发送（GB）
	TotalTxGB float64 `json:"total_tx_gb"`

	// 总流量（GB）
	TotalGB float64 `json:"total_gb"`
}

// BandwidthTrendPoint 带宽趋势数据点
type BandwidthTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	RxMbps    float64   `json:"rx_mbps"`
	TxMbps    float64   `json:"tx_mbps"`
	TotalMbps float64   `json:"total_mbps"`
}

// BandwidthPeakInfo 带宽峰值信息
type BandwidthPeakInfo struct {
	// 峰值速率（Mbps）
	PeakMbps float64 `json:"peak_mbps"`

	// 峰值时间
	PeakTime *time.Time `json:"peak_time,omitempty"`

	// 峰值接口
	PeakInterface string `json:"peak_interface,omitempty"`
}

// UserResourceOverview 用户资源概览
type UserResourceOverview struct {
	// 用户总数
	TotalUsers int `json:"total_users"`

	// 活跃用户数
	ActiveUsers int `json:"active_users"`

	// 配额总数
	TotalQuotas int `json:"total_quotas"`

	// 总配额限制（字节）
	TotalQuotaLimit uint64 `json:"total_quota_limit"`

	// 总使用量（字节）
	TotalUsed uint64 `json:"total_used"`

	// 平均使用率（%）
	AvgUsagePercent float64 `json:"avg_usage_percent"`

	// 超软限制用户数
	OverSoftLimit int `json:"over_soft_limit"`

	// 超硬限制用户数
	OverHardLimit int `json:"over_hard_limit"`

	// Top 用户
	TopUsers []UserResourceInfo `json:"top_users"`

	// 用户增长趋势
	UserTrend []UserTrendPoint `json:"user_trend"`
}

// UserResourceInfo 用户资源信息
type UserResourceInfo struct {
	// 用户名
	Username string `json:"username"`

	// 显示名称
	DisplayName string `json:"display_name"`

	// 使用量（字节）
	UsedBytes uint64 `json:"used_bytes"`

	// 配额限制（字节）
	QuotaBytes uint64 `json:"quota_bytes"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 文件数量
	FileCount uint64 `json:"file_count"`

	// 最后活跃时间
	LastActive *time.Time `json:"last_active,omitempty"`

	// 状态
	Status string `json:"status"` // active, inactive, over_quota
}

// UserTrendPoint 用户趋势数据点
type UserTrendPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	TotalUsers  int       `json:"total_users"`
	TotalUsed   uint64    `json:"total_used"`
	AvgUsagePct float64   `json:"avg_usage_pct"`
}

// SystemResourceOverview 系统资源概览
type SystemResourceOverview struct {
	// CPU 使用率（%）
	CPUUsage float64 `json:"cpu_usage"`

	// 内存使用率（%）
	MemoryUsage float64 `json:"memory_usage"`

	// 内存信息
	MemoryInfo MemoryInfo `json:"memory_info"`

	// 磁盘 I/O
	DiskIO DiskIOInfo `json:"disk_io"`

	// 网络连接数
	NetworkConnections int `json:"network_connections"`

	// 系统负载
	LoadAverage LoadAverageInfo `json:"load_average"`

	// 运行时间
	Uptime int64 `json:"uptime"` // 秒

	// 系统状态
	SystemStatus string `json:"system_status"` // healthy, warning, critical

	// 预测信息
	Prediction SystemPrediction `json:"prediction"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	// 总内存（字节）
	Total uint64 `json:"total"`

	// 已使用（字节）
	Used uint64 `json:"used"`

	// 可用（字节）
	Available uint64 `json:"available"`

	// 缓存（字节）
	Cached uint64 `json:"cached"`

	// 交换区总量（字节）
	SwapTotal uint64 `json:"swap_total"`

	// 交换区使用（字节）
	SwapUsed uint64 `json:"swap_used"`
}

// DiskIOInfo 磁盘 I/O 信息
type DiskIOInfo struct {
	// 读速率（字节/秒）
	ReadBps uint64 `json:"read_bps"`

	// 写速率（字节/秒）
	WriteBps uint64 `json:"write_bps"`

	// 读 IOPS
	ReadIOPS uint64 `json:"read_iops"`

	// 写 IOPS
	WriteIOPS uint64 `json:"write_iops"`

	// 等待时间（ms）
	Await float64 `json:"await"`

	// 利用率（%）
	Utilization float64 `json:"utilization"`
}

// LoadAverageInfo 负载信息
type LoadAverageInfo struct {
	// 1分钟负载
	Load1 float64 `json:"load_1"`

	// 5分钟负载
	Load5 float64 `json:"load_5"`

	// 15分钟负载
	Load15 float64 `json:"load_15"`
}

// SystemPrediction 系统预测
type SystemPrediction struct {
	// 预计存储增长（字节/天）
	StorageGrowthPerDay float64 `json:"storage_growth_per_day"`

	// 预计达到容量上限天数
	DaysToCapacity int `json:"days_to_capacity"`

	// 预计带宽增长（Mbps/天）
	BandwidthGrowthPerDay float64 `json:"bandwidth_growth_per_day"`

	// 预计新用户增长（用户/天）
	UserGrowthPerDay float64 `json:"user_growth_per_day"`

	// 置信度
	Confidence float64 `json:"confidence"` // 0-1
}

// ChartData 图表数据
type ChartData struct {
	// 图表ID
	ID string `json:"id"`

	// 图表类型
	Type string `json:"type"` // line, bar, pie, gauge, area

	// 标题
	Title string `json:"title"`

	// 子标题
	Subtitle string `json:"subtitle,omitempty"`

	// X轴标签
	XAxis string `json:"x_axis,omitempty"`

	// Y轴标签
	YAxis string `json:"y_axis,omitempty"`

	// 数据系列
	Series []ChartSeries `json:"series"`

	// 配置选项
	Options map[string]interface{} `json:"options,omitempty"`
}

// ChartSeries 图表数据系列
type ChartSeries struct {
	// 系列名称
	Name string `json:"name"`

	// 数据点
	Data []ChartPoint `json:"data"`

	// 颜色
	Color string `json:"color,omitempty"`

	// 类型（覆盖图表类型）
	Type string `json:"type,omitempty"`
}

// ChartPoint 图表数据点
type ChartPoint struct {
	// X值（时间戳或标签）
	X interface{} `json:"x"`

	// Y值
	Y float64 `json:"y"`

	// 额外数据
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ResourceRecommendation 资源建议
type ResourceRecommendation struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // storage, bandwidth, user, capacity, performance

	// 优先级
	Priority string `json:"priority"` // high, medium, low

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 建议值
	SuggestedValue float64 `json:"suggested_value,omitempty"`

	// 单位
	Unit string `json:"unit"`

	// 影响
	Impact string `json:"impact"`

	// 操作建议
	Action string `json:"action"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// ResourceAlert 资源告警
type ResourceAlert struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // storage, bandwidth, user, capacity, performance

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 标题
	Title string `json:"title"`

	// 消息
	Message string `json:"message"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 单位
	Unit string `json:"unit"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`

	// 是否已解决
	Resolved bool `json:"resolved"`

	// 解决时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// 关联资源
	Resource string `json:"resource,omitempty"`
}

// ========== 资源报告配置 ==========

// ResourceReportConfig 资源报告配置
type ResourceReportConfig struct {
	// 存储警告阈值（%）
	StorageWarningThreshold float64 `json:"storage_warning_threshold"`

	// 存储严重阈值（%）
	StorageCriticalThreshold float64 `json:"storage_critical_threshold"`

	// 带宽高利用率阈值（%）
	BandwidthHighThreshold float64 `json:"bandwidth_high_threshold"`

	// 带宽严重利用率阈值（%）
	BandwidthCriticalThreshold float64 `json:"bandwidth_critical_threshold"`

	// 预测天数
	PredictionDays int `json:"prediction_days"`

	// 趋势采样间隔（分钟）
	TrendSampleInterval int `json:"trend_sample_interval"`

	// 历史数据保留天数
	HistoryRetentionDays int `json:"history_retention_days"`

	// 是否启用预测
	EnablePrediction bool `json:"enable_prediction"`

	// 是否启用建议
	EnableRecommendations bool `json:"enable_recommendations"`

	// Top N 用户数量
	TopUsersCount int `json:"top_users_count"`
}

// DefaultResourceReportConfig 默认配置
func DefaultResourceReportConfig() ResourceReportConfig {
	return ResourceReportConfig{
		StorageWarningThreshold:    70.0,
		StorageCriticalThreshold:   85.0,
		BandwidthHighThreshold:     70.0,
		BandwidthCriticalThreshold: 90.0,
		PredictionDays:             30,
		TrendSampleInterval:        5,
		HistoryRetentionDays:       90,
		EnablePrediction:           true,
		EnableRecommendations:      true,
		TopUsersCount:              10,
	}
}
