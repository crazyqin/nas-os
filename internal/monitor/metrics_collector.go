package monitor

import (
	"sync"
	"time"

	"nas-os/pkg/safeguards"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	mu         sync.RWMutex
	manager    *Manager
	scorer     *HealthScorer
	metrics    []*CollectedMetrics
	maxMetrics int
	interval   time.Duration
	stopChan   chan struct{}
	running    bool
}

// CollectedMetrics 收集的指标
type CollectedMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	MemoryUsed   uint64    `json:"memory_used"`
	MemoryTotal  uint64    `json:"memory_total"`
	SwapUsage    float64   `json:"swap_usage"`
	LoadAvg1     float64   `json:"load_avg_1"`
	LoadAvg5     float64   `json:"load_avg_5"`
	LoadAvg15    float64   `json:"load_avg_15"`
	ProcessCount int       `json:"process_count"`

	DiskMetrics   []DiskMetric  `json:"disk_metrics"`
	NetworkMetric NetworkMetric `json:"network_metric"`
	HealthScore   float64       `json:"health_score"`
}

// DiskMetric 磁盘指标
type DiskMetric struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mount_point"`
	UsagePercent float64 `json:"usage_percent"`
	UsedBytes    uint64  `json:"used_bytes"`
	TotalBytes   uint64  `json:"total_bytes"`
	ReadBytes    uint64  `json:"read_bytes"`
	WriteBytes   uint64  `json:"write_bytes"`
}

// NetworkMetric 网络指标
type NetworkMetric struct {
	RXBytes   uint64 `json:"rx_bytes"`
	TXBytes   uint64 `json:"tx_bytes"`
	RXPackets uint64 `json:"rx_packets"`
	TXPackets uint64 `json:"tx_packets"`
}

// TrendData 趋势数据
type TrendData struct {
	StartTime  time.Time    `json:"start_time"`
	EndTime    time.Time    `json:"end_time"`
	Interval   string       `json:"interval"`
	DataPoints int          `json:"data_points"`
	Summary    TrendSummary `json:"summary"`
	Data       []TrendPoint `json:"data"`
}

// TrendSummary 趋势摘要
type TrendSummary struct {
	CPUAvg    float64 `json:"cpu_avg"`
	CPUMax    float64 `json:"cpu_max"`
	CPUMin    float64 `json:"cpu_min"`
	MemoryAvg float64 `json:"memory_avg"`
	MemoryMax float64 `json:"memory_max"`
	MemoryMin float64 `json:"memory_min"`
	DiskAvg   float64 `json:"disk_avg"`
	DiskMax   float64 `json:"disk_max"`
	HealthAvg float64 `json:"health_avg"`
	PeakTime  string  `json:"peak_time"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	DiskUsage   float64   `json:"disk_usage"`
	HealthScore float64   `json:"health_score"`
	LoadAvg     float64   `json:"load_avg"`
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(manager *Manager, scorer *HealthScorer) *MetricsCollector {
	return &MetricsCollector{
		manager:    manager,
		scorer:     scorer,
		metrics:    make([]*CollectedMetrics, 0),
		maxMetrics: 10080, // 约 1 周的数据（每分钟一次）
		interval:   time.Minute,
		stopChan:   make(chan struct{}),
	}
}

// Start 启动收集
func (mc *MetricsCollector) Start() {
	mc.mu.Lock()
	if mc.running {
		mc.mu.Unlock()
		return
	}
	mc.running = true
	mc.mu.Unlock()

	go mc.collectLoop()
}

// Stop 停止收集
func (mc *MetricsCollector) Stop() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.running {
		close(mc.stopChan)
		mc.running = false
		// 重新创建 stopChan 以支持重启
		mc.stopChan = make(chan struct{})
	}
}

// collectLoop 收集循环
func (mc *MetricsCollector) collectLoop() {
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopChan:
			return
		case <-ticker.C:
			mc.collect()
		}
	}
}

// collect 执行收集
func (mc *MetricsCollector) collect() {
	metric := &CollectedMetrics{
		Timestamp: time.Now(),
	}

	// 收集系统指标
	stats, err := mc.manager.GetSystemStats()
	if err == nil {
		metric.CPUUsage = stats.CPUUsage
		metric.MemoryUsage = stats.MemoryUsage
		metric.MemoryUsed = stats.MemoryUsed
		metric.MemoryTotal = stats.MemoryTotal
		metric.SwapUsage = stats.SwapUsage
		metric.ProcessCount = stats.Processes
		if len(stats.LoadAvg) >= 3 {
			metric.LoadAvg1 = stats.LoadAvg[0]
			metric.LoadAvg5 = stats.LoadAvg[1]
			metric.LoadAvg15 = stats.LoadAvg[2]
		}
	}

	// 收集磁盘指标
	diskStats, err := mc.manager.GetDiskStats()
	if err == nil {
		metric.DiskMetrics = make([]DiskMetric, 0, len(diskStats))
		for _, d := range diskStats {
			if d.FSType == "tmpfs" || d.FSType == "devtmpfs" {
				continue
			}
			metric.DiskMetrics = append(metric.DiskMetrics, DiskMetric{
				Device:       d.Device,
				MountPoint:   d.MountPoint,
				UsagePercent: d.UsagePercent,
				UsedBytes:    d.Used,
				TotalBytes:   d.Total,
			})
		}
	}

	// 收集网络指标
	netStats, err := mc.manager.GetNetworkStats()
	if err == nil {
		for _, n := range netStats {
			metric.NetworkMetric.RXBytes += n.RXBytes
			metric.NetworkMetric.TXBytes += n.TXBytes
			metric.NetworkMetric.RXPackets += n.RXPackets
			metric.NetworkMetric.TXPackets += n.TXPackets
		}
	}

	// 获取健康评分
	if mc.scorer != nil {
		if score := mc.scorer.GetLastScore(); score != nil {
			metric.HealthScore = score.TotalScore
		}
	}

	// 保存
	mc.mu.Lock()
	mc.metrics = append(mc.metrics, metric)
	if len(mc.metrics) > mc.maxMetrics {
		mc.metrics = mc.metrics[1:]
	}
	mc.mu.Unlock()
}

// GetTrendData 获取趋势数据
func (mc *MetricsCollector) GetTrendData(duration time.Duration, interval time.Duration) *TrendData {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if len(mc.metrics) == 0 {
		return &TrendData{
			DataPoints: 0,
		}
	}

	now := time.Now()
	cutoff := now.Add(-duration)

	// 筛选时间范围内的数据
	var filtered []*CollectedMetrics
	for _, m := range mc.metrics {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}

	if len(filtered) == 0 {
		return &TrendData{
			StartTime:  cutoff,
			EndTime:    now,
			DataPoints: 0,
		}
	}

	// 按间隔采样
	data := make([]TrendPoint, 0)
	var lastSample time.Time

	for _, m := range filtered {
		if lastSample.IsZero() || m.Timestamp.Sub(lastSample) >= interval {
			data = append(data, TrendPoint{
				Timestamp:   m.Timestamp,
				CPUUsage:    m.CPUUsage,
				MemoryUsage: m.MemoryUsage,
				HealthScore: m.HealthScore,
				LoadAvg:     m.LoadAvg1,
			})

			// 计算平均磁盘使用率
			if len(m.DiskMetrics) > 0 {
				var totalUsage float64
				for _, d := range m.DiskMetrics {
					totalUsage += d.UsagePercent
				}
				data[len(data)-1].DiskUsage = totalUsage / float64(len(m.DiskMetrics))
			}

			lastSample = m.Timestamp
		}
	}

	// 计算摘要
	summary := TrendSummary{}
	var cpuSum, memSum, diskSum, healthSum float64
	var maxCPU, maxMem float64
	var peakTime time.Time

	for _, d := range data {
		cpuSum += d.CPUUsage
		memSum += d.MemoryUsage
		diskSum += d.DiskUsage
		healthSum += d.HealthScore

		if d.CPUUsage > maxCPU {
			maxCPU = d.CPUUsage
			peakTime = d.Timestamp
		}
		if d.MemoryUsage > maxMem {
			maxMem = d.MemoryUsage
		}
	}

	if len(data) > 0 {
		summary.CPUAvg = cpuSum / float64(len(data))
		summary.MemoryAvg = memSum / float64(len(data))
		summary.DiskAvg = diskSum / float64(len(data))
		summary.HealthAvg = healthSum / float64(len(data))
		summary.CPUMax = maxCPU
		summary.MemoryMax = maxMem
		summary.PeakTime = peakTime.Format("2006-01-02 15:04:05")
	}

	// 计算最小值
	if len(data) > 0 {
		summary.CPUMin = data[0].CPUUsage
		summary.MemoryMin = data[0].MemoryUsage
		summary.DiskMax = data[0].DiskUsage
		for _, d := range data {
			if d.CPUUsage < summary.CPUMin {
				summary.CPUMin = d.CPUUsage
			}
			if d.MemoryUsage < summary.MemoryMin {
				summary.MemoryMin = d.MemoryUsage
			}
		}
	}

	return &TrendData{
		StartTime:  cutoff,
		EndTime:    now,
		Interval:   interval.String(),
		DataPoints: len(data),
		Summary:    summary,
		Data:       data,
	}
}

// GetHourlyTrend 获取小时趋势
func (mc *MetricsCollector) GetHourlyTrend() *TrendData {
	return mc.GetTrendData(time.Hour, time.Minute*5)
}

// GetDailyTrend 获取日趋势
func (mc *MetricsCollector) GetDailyTrend() *TrendData {
	return mc.GetTrendData(24*time.Hour, time.Minute*15)
}

// GetWeeklyTrend 获取周趋势
func (mc *MetricsCollector) GetWeeklyTrend() *TrendData {
	return mc.GetTrendData(7*24*time.Hour, time.Hour)
}

// GetLatestMetrics 获取最新指标
func (mc *MetricsCollector) GetLatestMetrics() *CollectedMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if len(mc.metrics) == 0 {
		return nil
	}

	return mc.metrics[len(mc.metrics)-1]
}

// GetMetricsHistory 获取历史指标
func (mc *MetricsCollector) GetMetricsHistory(limit int) []*CollectedMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if limit <= 0 || limit > len(mc.metrics) {
		limit = len(mc.metrics)
	}

	start := len(mc.metrics) - limit
	result := make([]*CollectedMetrics, limit)
	copy(result, mc.metrics[start:])

	return result
}

// ResourceUsageReport 资源使用报告
type ResourceUsageReport struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	Period          string           `json:"period"`
	SystemInfo      SystemReportInfo `json:"system_info"`
	ResourceUsage   ResourceSummary  `json:"resource_usage"`
	Trends          TrendAnalysis    `json:"trends"`
	DiskAnalysis    []DiskAnalysis   `json:"disk_analysis"`
	Alerts          []AlertSummary   `json:"alerts"`
	Recommendations []string         `json:"recommendations"`
}

// SystemReportInfo 系统报告信息
type SystemReportInfo struct {
	Hostname      string `json:"hostname"`
	Uptime        string `json:"uptime"`
	UptimeSeconds uint64 `json:"uptime_seconds"`
}

// ResourceSummary 资源摘要
type ResourceSummary struct {
	CPU       ResourceMetric `json:"cpu"`
	Memory    ResourceMetric `json:"memory"`
	Swap      ResourceMetric `json:"swap"`
	TotalDisk ResourceMetric `json:"total_disk"`
	Network   NetworkSummary `json:"network"`
}

// ResourceMetric 资源指标
type ResourceMetric struct {
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
	Average float64 `json:"average"`
	Peak    float64 `json:"peak"`
	Status  string  `json:"status"`
}

// NetworkSummary 网络摘要
type NetworkSummary struct {
	RXBytes   uint64 `json:"rx_bytes"`
	TXBytes   uint64 `json:"tx_bytes"`
	RXPackets uint64 `json:"rx_packets"`
	TXPackets uint64 `json:"tx_packets"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	CPUTrend    string  `json:"cpu_trend"`
	MemoryTrend string  `json:"memory_trend"`
	DiskTrend   string  `json:"disk_trend"`
	HealthScore float64 `json:"health_score"`
	HealthGrade string  `json:"health_grade"`
}

// DiskAnalysis 磁盘分析
type DiskAnalysis struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mount_point"`
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
	Trend        string  `json:"trend"`
	Status       string  `json:"status"`
}

// AlertSummary 告警摘要
type AlertSummary struct {
	Type     string `json:"type"`
	Count    int    `json:"count"`
	LastTime string `json:"last_time"`
}

// GenerateResourceReport 生成资源使用报告
func (mc *MetricsCollector) GenerateResourceReport(period string) *ResourceUsageReport {
	report := &ResourceUsageReport{
		GeneratedAt:     time.Now(),
		Period:          period,
		Recommendations: make([]string, 0),
	}

	// 获取系统信息
	stats, _ := mc.manager.GetSystemStats()
	if stats != nil {
		report.SystemInfo = SystemReportInfo{
			Hostname:      mc.manager.GetHostname(),
			Uptime:        stats.Uptime,
			UptimeSeconds: stats.UptimeSeconds,
		}

		report.ResourceUsage.CPU = ResourceMetric{
			Percent: stats.CPUUsage,
			Status:  getResourceStatus(stats.CPUUsage, 70, 90),
		}
		report.ResourceUsage.Memory = ResourceMetric{
			Used:    stats.MemoryUsed,
			Total:   stats.MemoryTotal,
			Percent: stats.MemoryUsage,
			Status:  getResourceStatus(stats.MemoryUsage, 75, 90),
		}
		report.ResourceUsage.Swap = ResourceMetric{
			Used:    stats.SwapUsed,
			Total:   stats.SwapTotal,
			Percent: stats.SwapUsage,
		}
	}

	// 获取趋势
	var trend *TrendData
	switch period {
	case "hourly":
		trend = mc.GetHourlyTrend()
	case "daily":
		trend = mc.GetDailyTrend()
	case "weekly":
		trend = mc.GetWeeklyTrend()
	default:
		trend = mc.GetDailyTrend()
	}

	if trend != nil && len(trend.Data) > 0 {
		report.ResourceUsage.CPU.Average = trend.Summary.CPUAvg
		report.ResourceUsage.CPU.Peak = trend.Summary.CPUMax
		report.ResourceUsage.Memory.Average = trend.Summary.MemoryAvg
		report.ResourceUsage.Memory.Peak = trend.Summary.MemoryMax
		report.Trends.HealthScore = trend.Summary.HealthAvg

		// 分析趋势方向
		report.Trends.CPUTrend = analyzeTrend(trend.Data, "cpu")
		report.Trends.MemoryTrend = analyzeTrend(trend.Data, "memory")
		report.Trends.DiskTrend = analyzeTrend(trend.Data, "disk")
	}

	// 获取健康评分等级
	if mc.scorer != nil {
		if score := mc.scorer.GetLastScore(); score != nil {
			report.Trends.HealthGrade = score.Grade
		}
	}

	// 磁盘分析
	diskStats, _ := mc.manager.GetDiskStats()
	if diskStats != nil {
		var totalDiskUsed, totalDiskTotal uint64
		report.DiskAnalysis = make([]DiskAnalysis, 0)

		for _, d := range diskStats {
			if d.FSType == "tmpfs" || d.FSType == "devtmpfs" || d.FSType == "overlay" {
				continue
			}

			analysis := DiskAnalysis{
				Device:       d.Device,
				MountPoint:   d.MountPoint,
				Total:        d.Total,
				Used:         d.Used,
				Free:         d.Free,
				UsagePercent: d.UsagePercent,
				Status:       getResourceStatus(d.UsagePercent, 80, 95),
			}
			report.DiskAnalysis = append(report.DiskAnalysis, analysis)

			totalDiskUsed += d.Used
			totalDiskTotal += d.Total
		}

		var diskPercent float64
		if totalDiskTotal > 0 {
			diskPercent = float64(totalDiskUsed) / float64(totalDiskTotal) * 100
		}
		report.ResourceUsage.TotalDisk = ResourceMetric{
			Used:    totalDiskUsed,
			Total:   totalDiskTotal,
			Percent: diskPercent,
		}
	}

	// 网络统计
	netStats, _ := mc.manager.GetNetworkStats()
	for _, n := range netStats {
		report.ResourceUsage.Network.RXBytes += n.RXBytes
		report.ResourceUsage.Network.TXBytes += n.TXBytes
		report.ResourceUsage.Network.RXPackets += n.RXPackets
		report.ResourceUsage.Network.TXPackets += n.TXPackets
	}

	// 生成建议
	report.Recommendations = mc.generateRecommendations(report)

	return report
}

// getResourceStatus 获取资源状态
func getResourceStatus(percent, warning, critical float64) string {
	if percent >= critical {
		return "critical"
	} else if percent >= warning {
		return "warning"
	}
	return "healthy"
}

// analyzeTrend 分析趋势
func analyzeTrend(data []TrendPoint, metric string) string {
	if len(data) < 2 {
		return "insufficient_data"
	}

	var values []float64
	for _, d := range data {
		switch metric {
		case "cpu":
			values = append(values, d.CPUUsage)
		case "memory":
			values = append(values, d.MemoryUsage)
		case "disk":
			values = append(values, d.DiskUsage)
		}
	}

	// 简单线性趋势判断
	first := values[0]
	last := values[len(values)-1]
	change := last - first

	if change > 10 {
		return "increasing"
	} else if change < -10 {
		return "decreasing"
	}
	return "stable"
}

// BackupMonitoringData 备份监控数据 (v2.59.0)
type BackupMonitoringData struct {
	Timestamp          time.Time `json:"timestamp"`
	TotalBackups       int       `json:"total_backups"`
	FullBackups        int       `json:"full_backups"`
	IncrementalBackups int       `json:"incremental_backups"`
	DatabaseBackups    int       `json:"database_backups"`
	ConfigBackups      int       `json:"config_backups"`
	TotalSizeBytes     uint64    `json:"total_size_bytes"`
	OldestBackupAge    int       `json:"oldest_backup_age_hours"`
	LatestBackupAge    int       `json:"latest_backup_age_hours"`
	BackupSpaceUsed    uint64    `json:"backup_space_used"`
	BackupSpaceTotal   uint64    `json:"backup_space_total"`
	BackupSpaceAvail   uint64    `json:"backup_space_available"`
	BackupHealthy      bool      `json:"backup_healthy"`
	LastBackupTime     time.Time `json:"last_backup_time"`
	Errors             []string  `json:"errors,omitempty"`
}

// DiskHealthMetrics 磁盘健康指标 (v2.59.0)
type DiskHealthMetrics struct {
	Timestamp          time.Time            `json:"timestamp"`
	TotalDisks         int                  `json:"total_disks"`
	HealthyDisks       int                  `json:"healthy_disks"`
	WarningDisks       int                  `json:"warning_disks"`
	CriticalDisks      int                  `json:"critical_disks"`
	UnknownDisks       int                  `json:"unknown_disks"`
	AverageTemperature int                  `json:"average_temperature"`
	AverageHealthScore float64              `json:"average_health_score"`
	TotalErrors        int                  `json:"total_errors"`
	Disks              []DiskHealthMetric   `json:"disks"`
	Summary            DiskHealthSummaryV25 `json:"summary"`
}

// DiskHealthMetric 单个磁盘健康指标
type DiskHealthMetric struct {
	Device             string    `json:"device"`
	Model              string    `json:"model"`
	Serial             string    `json:"serial_number"`
	IsSSD              bool      `json:"is_ssd"`
	Temperature        int       `json:"temperature"`
	HealthScore        int       `json:"health_score"`
	HealthStatus       string    `json:"health_status"`
	ReallocatedSectors int       `json:"reallocated_sectors"`
	PendingSectors     int       `json:"pending_sectors"`
	CRCErrors          int       `json:"crc_errors"`
	PowerOnHours       uint64    `json:"power_on_hours"`
	PowerCycles        uint64    `json:"power_cycles"`
	LastCheck          time.Time `json:"last_check"`
}

// DiskHealthSummaryV25 磁盘健康摘要
type DiskHealthSummaryV25 struct {
	MaxTemperature int    `json:"max_temperature"`
	MinTemperature int    `json:"min_temperature"`
	TotalCapacity  uint64 `json:"total_capacity_bytes"`
	SSDCount       int    `json:"ssd_count"`
	HDDCount       int    `json:"hdd_count"`
}

// ExtendedMetrics 扩展指标 (v2.59.0)
type ExtendedMetrics struct {
	Timestamp       time.Time             `json:"timestamp"`
	System          *CollectedMetrics     `json:"system"`
	Backup          *BackupMonitoringData `json:"backup,omitempty"`
	DiskHealth      *DiskHealthMetrics    `json:"disk_health,omitempty"`
	AlertCount      int                   `json:"alert_count"`
	CriticalAlerts  int                   `json:"critical_alerts"`
	WarningAlerts   int                   `json:"warning_alerts"`
	OverallStatus   string                `json:"overall_status"`
	Recommendations []string              `json:"recommendations,omitempty"`
}

// generateRecommendations 生成建议
func (mc *MetricsCollector) generateRecommendations(report *ResourceUsageReport) []string {
	recs := make([]string, 0)

	// CPU 建议
	if report.ResourceUsage.CPU.Percent > 80 {
		recs = append(recs, "CPU 使用率较高，建议检查占用进程或考虑扩展资源")
	}
	if report.Trends.CPUTrend == "increasing" {
		recs = append(recs, "CPU 使用呈上升趋势，建议关注系统负载变化")
	}

	// 内存建议
	if report.ResourceUsage.Memory.Percent > 85 {
		recs = append(recs, "内存使用率过高，建议增加内存或优化应用")
	}
	if report.ResourceUsage.Swap.Percent > 50 {
		recs = append(recs, "Swap 使用率较高，系统可能内存不足")
	}

	// 磁盘建议
	for _, d := range report.DiskAnalysis {
		if d.UsagePercent > 90 {
			recs = append(recs, "磁盘 "+d.MountPoint+" 空间严重不足，请立即处理")
		} else if d.UsagePercent > 80 {
			recs = append(recs, "磁盘 "+d.MountPoint+" 空间紧张，建议清理")
		}
	}

	// 健康评分建议
	if report.Trends.HealthScore < 60 {
		recs = append(recs, "系统健康评分较低，建议进行全面检查")
	}

	return recs
}

// CollectBackupMetrics 收集备份指标 (v2.59.0)
func (mc *MetricsCollector) CollectBackupMetrics() *BackupMonitoringData {
	metrics := &BackupMonitoringData{
		Timestamp: time.Now(),
		Errors:    make([]string, 0),
	}

	// 从监控管理器获取备份数据（如果实现了接口）
	// 这里提供默认实现框架
	backupStats, err := mc.manager.GetBackupStats()
	if err != nil {
		metrics.Errors = append(metrics.Errors, "无法获取备份统计: "+err.Error())
		metrics.BackupHealthy = false
		return metrics
	}

	if backupStats != nil {
		metrics.TotalBackups = backupStats.TotalCount
		metrics.FullBackups = backupStats.FullCount
		metrics.IncrementalBackups = backupStats.IncrementalCount
		metrics.DatabaseBackups = backupStats.DatabaseCount
		metrics.ConfigBackups = backupStats.ConfigCount
		metrics.TotalSizeBytes = backupStats.TotalSize
		metrics.BackupSpaceUsed = backupStats.SpaceUsed
		metrics.BackupSpaceTotal = backupStats.SpaceTotal
		metrics.BackupSpaceAvail = backupStats.SpaceAvailable

		if backupStats.LatestBackup != nil {
			metrics.LatestBackupAge = int(time.Since(backupStats.LatestBackup.Timestamp).Hours())
			metrics.LastBackupTime = backupStats.LatestBackup.Timestamp
		}

		if backupStats.OldestBackup != nil {
			metrics.OldestBackupAge = int(time.Since(backupStats.OldestBackup.Timestamp).Hours())
		}

		// 判断备份健康状态
		metrics.BackupHealthy = mc.evaluateBackupHealth(metrics)
	}

	return metrics
}

// BackupStats 备份统计信息
type BackupStats struct {
	TotalCount       int
	FullCount        int
	IncrementalCount int
	DatabaseCount    int
	ConfigCount      int
	TotalSize        uint64
	SpaceUsed        uint64
	SpaceTotal       uint64
	SpaceAvailable   uint64
	LatestBackup     *BackupInfo
	OldestBackup     *BackupInfo
}

// BackupInfo 备份信息
type BackupInfo struct {
	Timestamp time.Time
	Size      uint64
	Type      string
	Path      string
}

// evaluateBackupHealth 评估备份健康状态
func (mc *MetricsCollector) evaluateBackupHealth(metrics *BackupMonitoringData) bool {
	// 无备份则不健康
	if metrics.TotalBackups == 0 {
		metrics.Errors = append(metrics.Errors, "无备份文件")
		return false
	}

	// 最新备份超过24小时则不健康
	if metrics.LatestBackupAge > 24 {
		metrics.Errors = append(metrics.Errors, "备份过期，最新备份超过24小时")
		return false
	}

	// 空间不足警告
	if metrics.BackupSpaceTotal > 0 {
		availPercent := float64(metrics.BackupSpaceAvail) / float64(metrics.BackupSpaceTotal) * 100
		if availPercent < 10 {
			metrics.Errors = append(metrics.Errors, "备份空间不足10%")
			return false
		}
	}

	return true
}

// CollectDiskHealthMetrics 收集磁盘健康指标 (v2.59.0)
func (mc *MetricsCollector) CollectDiskHealthMetrics() *DiskHealthMetrics {
	metrics := &DiskHealthMetrics{
		Timestamp: time.Now(),
		Disks:     make([]DiskHealthMetric, 0),
	}

	// 从磁盘健康监控器获取数据
	diskMonitor := mc.manager.GetDiskHealthMonitor()
	if diskMonitor == nil {
		return metrics
	}

	allDisks := diskMonitor.GetAllDisksHealth()
	metrics.TotalDisks = len(allDisks)

	var totalTemp, totalScore int
	var maxTemp, minTemp = -1, 999
	var totalCapacity uint64
	var ssdCount, hddCount int

	for _, disk := range allDisks {
		metric := DiskHealthMetric{
			Device:             disk.Device,
			Model:              disk.Model,
			Serial:             disk.SerialNumber,
			IsSSD:              disk.IsSSD,
			Temperature:        disk.Temperature,
			HealthScore:        disk.HealthScore,
			HealthStatus:       string(disk.HealthStatus),
			ReallocatedSectors: 0,
			PendingSectors:     0,
			CRCErrors:          0,
			PowerOnHours:       disk.PowerOnHours,
			PowerCycles:        disk.PowerCycleCount,
			LastCheck:          disk.LastCheck,
		}

		// 提取 SMART 属性
		if attr, ok := disk.SmartAttributes["Reallocated_Sector_Ct"]; ok {
			if val, err := safeguards.SafeUint64ToInt(attr.RawValue); err == nil {
				metric.ReallocatedSectors = val
			}
		}
		if attr, ok := disk.SmartAttributes["Current_Pending_Sector"]; ok {
			if val, err := safeguards.SafeUint64ToInt(attr.RawValue); err == nil {
				metric.PendingSectors = val
			}
		}
		if attr, ok := disk.SmartAttributes["UDMA_CRC_Error_Count"]; ok {
			if val, err := safeguards.SafeUint64ToInt(attr.RawValue); err == nil {
				metric.CRCErrors = val
			}
		}

		metrics.Disks = append(metrics.Disks, metric)

		// 统计
		totalTemp += disk.Temperature
		totalScore += disk.HealthScore
		metrics.TotalErrors += len(disk.Errors)
		totalCapacity += disk.Capacity

		if disk.Temperature > maxTemp {
			maxTemp = disk.Temperature
		}
		if disk.Temperature < minTemp {
			minTemp = disk.Temperature
		}

		if disk.IsSSD {
			ssdCount++
		} else {
			hddCount++
		}

		// 按状态计数
		switch disk.HealthStatus {
		case HealthStatusHealthy:
			metrics.HealthyDisks++
		case HealthStatusWarning:
			metrics.WarningDisks++
		case HealthStatusDegraded, HealthStatusFailed:
			metrics.CriticalDisks++
		default:
			metrics.UnknownDisks++
		}
	}

	// 计算平均值
	if metrics.TotalDisks > 0 {
		metrics.AverageTemperature = totalTemp / metrics.TotalDisks
		metrics.AverageHealthScore = float64(totalScore) / float64(metrics.TotalDisks)
	}

	// 填充摘要
	metrics.Summary = DiskHealthSummaryV25{
		MaxTemperature: maxTemp,
		MinTemperature: minTemp,
		TotalCapacity:  totalCapacity,
		SSDCount:       ssdCount,
		HDDCount:       hddCount,
	}

	return metrics
}

// CollectExtendedMetrics 收集扩展指标 (v2.59.0)
func (mc *MetricsCollector) CollectExtendedMetrics() *ExtendedMetrics {
	ext := &ExtendedMetrics{
		Timestamp:       time.Now(),
		Recommendations: make([]string, 0),
	}

	// 收集系统指标
	ext.System = mc.GetLatestMetrics()

	// 收集备份指标
	ext.Backup = mc.CollectBackupMetrics()

	// 收集磁盘健康指标
	ext.DiskHealth = mc.CollectDiskHealthMetrics()

	// 统计告警
	if alertMgr := mc.manager.GetAlertingManager(); alertMgr != nil {
		alerts := alertMgr.GetActiveAlerts()
		ext.AlertCount = len(alerts)
		for _, alert := range alerts {
			switch alert.Level {
			case "critical":
				ext.CriticalAlerts++
			case "warning":
				ext.WarningAlerts++
			}
		}
	}

	// 计算总体状态
	ext.OverallStatus = mc.calculateOverallStatus(ext)

	// 生成建议
	ext.Recommendations = mc.generateExtendedRecommendations(ext)

	return ext
}

// calculateOverallStatus 计算总体状态
func (mc *MetricsCollector) calculateOverallStatus(ext *ExtendedMetrics) string {
	// 有严重告警
	if ext.CriticalAlerts > 0 {
		return "critical"
	}

	// 磁盘健康问题
	if ext.DiskHealth != nil && ext.DiskHealth.CriticalDisks > 0 {
		return "critical"
	}

	// 备份不健康
	if ext.Backup != nil && !ext.Backup.BackupHealthy {
		return "warning"
	}

	// 有警告告警
	if ext.WarningAlerts > 0 {
		return "warning"
	}

	// 磁盘警告
	if ext.DiskHealth != nil && ext.DiskHealth.WarningDisks > 0 {
		return "warning"
	}

	return "healthy"
}

// generateExtendedRecommendations 生成扩展建议
func (mc *MetricsCollector) generateExtendedRecommendations(ext *ExtendedMetrics) []string {
	recs := make([]string, 0)

	// 备份建议
	if ext.Backup != nil {
		if !ext.Backup.BackupHealthy {
			if ext.Backup.TotalBackups == 0 {
				recs = append(recs, "建议立即创建备份，当前无备份文件")
			} else if ext.Backup.LatestBackupAge > 24 {
				recs = append(recs, "备份已过期，建议立即执行备份任务")
			}
		}
		if ext.Backup.BackupSpaceAvail > 0 && ext.Backup.BackupSpaceTotal > 0 {
			availPercent := float64(ext.Backup.BackupSpaceAvail) / float64(ext.Backup.BackupSpaceTotal) * 100
			if availPercent < 20 {
				recs = append(recs, "备份空间不足20%，建议清理旧备份或扩展存储")
			}
		}
	}

	// 磁盘健康建议
	if ext.DiskHealth != nil {
		for _, disk := range ext.DiskHealth.Disks {
			switch disk.HealthStatus {
			case "failed", "critical":
				recs = append(recs, "磁盘 "+disk.Device+" 健康状态异常，建议立即更换")
			case "warning", "degraded":
				recs = append(recs, "磁盘 "+disk.Device+" 状态下降，建议关注")
			}
			if disk.Temperature > 55 {
				recs = append(recs, "磁盘 "+disk.Device+" 温度过高("+string(rune(disk.Temperature))+"°C)，建议检查散热")
			}
			if disk.ReallocatedSectors > 0 {
				recs = append(recs, "磁盘 "+disk.Device+" 存在重分配扇区，建议监控")
			}
			if disk.PendingSectors > 0 {
				recs = append(recs, "磁盘 "+disk.Device+" 存在待定扇区，建议检查数据完整性")
			}
		}
	}

	// 告警建议
	if ext.CriticalAlerts > 0 {
		recs = append(recs, "存在严重告警，建议立即处理")
	}

	return recs
}

// GetBackupMetricsHistory 获取备份指标历史
func (mc *MetricsCollector) GetBackupMetricsHistory(limit int) []*BackupMonitoringData {
	// 这个方法可以扩展为从持久化存储读取历史数据
	// 目前返回空列表
	return make([]*BackupMonitoringData, 0)
}

// GetDiskHealthTrend 获取磁盘健康趋势
func (mc *MetricsCollector) GetDiskHealthTrend(duration time.Duration) map[string][]DiskHealthMetric {
	// 这个方法可以扩展为从持久化存储读取历史趋势
	// 目前返回空 map
	return make(map[string][]DiskHealthMetric)
}
