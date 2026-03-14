package monitor

import (
	"sync"
	"time"
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
