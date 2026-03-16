// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"sort"
	"time"
)

// ResourceReporter 资源报告生成器
type ResourceReporter struct {
	config           ResourceReportConfig
	bandwidthHistory map[string][]BandwidthHistoryPoint
}

// NewResourceReporter 创建资源报告生成器
func NewResourceReporter(config ResourceReportConfig) *ResourceReporter {
	if config.StorageWarningThreshold == 0 {
		config = DefaultResourceReportConfig()
	}
	return &ResourceReporter{
		config:           config,
		bandwidthHistory: make(map[string][]BandwidthHistoryPoint),
	}
}

// GenerateOverviewReport 生成总览报告
func (r *ResourceReporter) GenerateOverviewReport(
	storageMetrics []StorageMetrics,
	bandwidthHistory map[string][]BandwidthHistoryPoint,
	userMetrics []UserResourceInfo,
	systemMetrics *SystemResourceOverview,
) *ResourceVisualizationReport {
	now := time.Now()
	report := &ResourceVisualizationReport{
		ID:          "resource_overview_" + now.Format("20060102150405"),
		Type:        ResourceReportOverview,
		Name:        "资源可视化总览报告",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -7),
			EndTime:   now,
		},
		Charts:          make([]ChartData, 0),
		Recommendations: make([]ResourceRecommendation, 0),
		Alerts:          make([]ResourceAlert, 0),
	}

	// 生成存储概览
	if len(storageMetrics) > 0 {
		report.StorageOverview = r.generateStorageOverview(storageMetrics)
		report.Charts = append(report.Charts, r.generateStorageCharts(report.StorageOverview)...)
		report.Alerts = append(report.Alerts, r.detectStorageAlerts(report.StorageOverview)...)
		report.Recommendations = append(report.Recommendations, r.generateStorageRecommendations(report.StorageOverview)...)
	}

	// 生成带宽概览
	if len(bandwidthHistory) > 0 {
		report.BandwidthOverview = r.generateBandwidthOverview(bandwidthHistory)
		report.Charts = append(report.Charts, r.generateBandwidthCharts(report.BandwidthOverview)...)
		report.Alerts = append(report.Alerts, r.detectBandwidthAlerts(report.BandwidthOverview)...)
		report.Recommendations = append(report.Recommendations, r.generateBandwidthRecommendations(report.BandwidthOverview)...)
	}

	// 生成用户概览
	if len(userMetrics) > 0 {
		report.UserOverview = r.generateUserOverview(userMetrics)
		report.Charts = append(report.Charts, r.generateUserCharts(report.UserOverview)...)
		report.Alerts = append(report.Alerts, r.detectUserAlerts(report.UserOverview)...)
	}

	// 生成系统概览
	if systemMetrics != nil {
		report.SystemOverview = systemMetrics
		report.Charts = append(report.Charts, r.generateSystemCharts(systemMetrics)...)
	}

	// 排序建议（按优先级）
	sort.Slice(report.Recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return priorityOrder[report.Recommendations[i].Priority] < priorityOrder[report.Recommendations[j].Priority]
	})

	return report
}

// GenerateStorageReport 生成存储报告
func (r *ResourceReporter) GenerateStorageReport(metrics []StorageMetrics) *ResourceVisualizationReport {
	now := time.Now()
	report := &ResourceVisualizationReport{
		ID:          "storage_" + now.Format("20060102150405"),
		Type:        ResourceReportStorage,
		Name:        "存储使用报告",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -7),
			EndTime:   now,
		},
		Charts:          make([]ChartData, 0),
		Recommendations: make([]ResourceRecommendation, 0),
		Alerts:          make([]ResourceAlert, 0),
	}

	report.StorageOverview = r.generateStorageOverview(metrics)
	report.Charts = r.generateStorageCharts(report.StorageOverview)
	report.Alerts = r.detectStorageAlerts(report.StorageOverview)
	report.Recommendations = r.generateStorageRecommendations(report.StorageOverview)

	return report
}

// GenerateBandwidthReport 生成带宽报告
func (r *ResourceReporter) GenerateBandwidthReport(history map[string][]BandwidthHistoryPoint) *ResourceVisualizationReport {
	now := time.Now()
	report := &ResourceVisualizationReport{
		ID:          "bandwidth_" + now.Format("20060102150405"),
		Type:        ResourceReportBandwidth,
		Name:        "带宽使用报告",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -7),
			EndTime:   now,
		},
		Charts:          make([]ChartData, 0),
		Recommendations: make([]ResourceRecommendation, 0),
		Alerts:          make([]ResourceAlert, 0),
	}

	report.BandwidthOverview = r.generateBandwidthOverview(history)
	report.Charts = r.generateBandwidthCharts(report.BandwidthOverview)
	report.Alerts = r.detectBandwidthAlerts(report.BandwidthOverview)
	report.Recommendations = r.generateBandwidthRecommendations(report.BandwidthOverview)

	return report
}

// GenerateUserReport 生成用户报告
func (r *ResourceReporter) GenerateUserReport(metrics []UserResourceInfo) *ResourceVisualizationReport {
	now := time.Now()
	report := &ResourceVisualizationReport{
		ID:          "user_" + now.Format("20060102150405"),
		Type:        ResourceReportUser,
		Name:        "用户资源报告",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -7),
			EndTime:   now,
		},
		Charts:          make([]ChartData, 0),
		Recommendations: make([]ResourceRecommendation, 0),
		Alerts:          make([]ResourceAlert, 0),
	}

	report.UserOverview = r.generateUserOverview(metrics)
	report.Charts = r.generateUserCharts(report.UserOverview)
	report.Alerts = r.detectUserAlerts(report.UserOverview)

	return report
}

// ========== 存储概览生成 ==========

func (r *ResourceReporter) generateStorageOverview(metrics []StorageMetrics) *StorageOverview {
	overview := &StorageOverview{
		VolumeCount: len(metrics),
		Volumes:     make([]VolumeStorageInfo, 0, len(metrics)),
	}

	var totalCapacity, totalUsed, totalFiles, totalDirs uint64

	for _, m := range metrics {
		totalCapacity += m.TotalCapacityBytes
		totalUsed += m.UsedCapacityBytes
		totalFiles += m.FileCount
		totalDirs += m.DirCount

		volume := VolumeStorageInfo{
			Name:              m.VolumeName,
			TotalCapacity:     m.TotalCapacityBytes,
			UsedCapacity:      m.UsedCapacityBytes,
			AvailableCapacity: m.AvailableCapacityBytes,
			IOPS:              m.IOPS,
			ReadBandwidth:     m.ReadBandwidthBytes,
			WriteBandwidth:    m.WriteBandwidthBytes,
		}

		if m.TotalCapacityBytes > 0 {
			volume.UsagePercent = round(float64(m.UsedCapacityBytes)/float64(m.TotalCapacityBytes)*100, 2)
		}

		overview.Volumes = append(overview.Volumes, volume)
	}

	overview.TotalCapacity = totalCapacity
	overview.UsedCapacity = totalUsed
	overview.AvailableCapacity = totalCapacity - totalUsed
	overview.TotalFiles = totalFiles
	overview.TotalDirectories = totalDirs

	if totalCapacity > 0 {
		overview.UsagePercent = round(float64(totalUsed)/float64(totalCapacity)*100, 2)
	}

	// 人类可读格式
	overview.TotalCapacityHR = formatBytesForResource(totalCapacity)
	overview.UsedCapacityHR = formatBytesForResource(totalUsed)
	overview.AvailableCapacityHR = formatBytesForResource(overview.AvailableCapacity)

	return overview
}

// ========== 带宽概览生成 ==========

func (r *ResourceReporter) generateBandwidthOverview(history map[string][]BandwidthHistoryPoint) *BandwidthOverview {
	overview := &BandwidthOverview{
		InterfaceCount: len(history),
		Interfaces:     make([]InterfaceBandwidthInfo, 0, len(history)),
		Trend:          make([]BandwidthTrendPoint, 0),
	}

	var totalRxBytes, totalTxBytes uint64
	var peakMbps float64
	var peakTime *time.Time
	var peakInterface string

	for iface, points := range history {
		if len(points) == 0 {
			continue
		}

		var ifaceRx, ifaceTx uint64
		var ifaceRxRate, ifaceTxRate uint64

		for _, p := range points {
			ifaceRx += p.RxBytes
			ifaceTx += p.TxBytes
			ifaceRxRate += p.RxRate
			ifaceTxRate += p.TxRate

			// 检查峰值
			totalMbps := float64(p.RxRate+p.TxRate) * 8 / (1024 * 1024)
			if totalMbps > peakMbps {
				peakMbps = totalMbps
				peakTime = &p.Timestamp
				peakInterface = iface
			}
		}

		n := uint64(len(points))
		if n > 0 {
			ifaceRxRate /= n
			ifaceTxRate /= n
		}

		totalRxBytes += ifaceRx
		totalTxBytes += ifaceTx

		overview.Interfaces = append(overview.Interfaces, InterfaceBandwidthInfo{
			Name: iface,
			Rate: BandwidthRate{
				RxBytesPerSec:    ifaceRxRate,
				TxBytesPerSec:    ifaceTxRate,
				TotalBytesPerSec: ifaceRxRate + ifaceTxRate,
				RxMbps:           float64(ifaceRxRate) * 8 / (1024 * 1024),
				TxMbps:           float64(ifaceTxRate) * 8 / (1024 * 1024),
				TotalMbps:        float64(ifaceRxRate+ifaceTxRate) * 8 / (1024 * 1024),
			},
			TotalRx: ifaceRx,
			TotalTx: ifaceTx,
		})
	}

	overview.Summary = BandwidthSummaryInfo{
		TotalRxBytes: totalRxBytes,
		TotalTxBytes: totalTxBytes,
		TotalBytes:   totalRxBytes + totalTxBytes,
		TotalRxGB:    float64(totalRxBytes) / (1024 * 1024 * 1024),
		TotalTxGB:    float64(totalTxBytes) / (1024 * 1024 * 1024),
		TotalGB:      float64(totalRxBytes+totalTxBytes) / (1024 * 1024 * 1024),
	}

	overview.Peak = BandwidthPeakInfo{
		PeakMbps:      round(peakMbps, 2),
		PeakTime:      peakTime,
		PeakInterface: peakInterface,
	}

	// 判断流量模式
	if overview.Summary.TotalRxGB > overview.Summary.TotalTxGB*1.5 {
		overview.TrafficPattern = "download_heavy"
	} else if overview.Summary.TotalTxGB > overview.Summary.TotalRxGB*1.5 {
		overview.TrafficPattern = "upload_heavy"
	} else {
		overview.TrafficPattern = "balanced"
	}

	return overview
}

// ========== 用户概览生成 ==========

func (r *ResourceReporter) generateUserOverview(metrics []UserResourceInfo) *UserResourceOverview {
	overview := &UserResourceOverview{
		TotalUsers: len(metrics),
		TopUsers:   make([]UserResourceInfo, 0),
		UserTrend:  make([]UserTrendPoint, 0),
	}

	var totalUsed, totalQuota uint64
	var overSoft, overHard int
	var totalUsagePercent float64

	// 按 quota 排序获取 top 用户
	sortedMetrics := make([]UserResourceInfo, len(metrics))
	copy(sortedMetrics, metrics)
	sort.Slice(sortedMetrics, func(i, j int) bool {
		return sortedMetrics[i].UsedBytes > sortedMetrics[j].UsedBytes
	})

	topCount := r.config.TopUsersCount
	if topCount > len(sortedMetrics) {
		topCount = len(sortedMetrics)
	}

	for i, m := range sortedMetrics {
		totalUsed += m.UsedBytes
		totalQuota += m.QuotaBytes
		totalUsagePercent += m.UsagePercent

		if m.UsagePercent >= 100 {
			overHard++
		} else if m.UsagePercent >= 80 {
			overSoft++
		}

		if i < topCount {
			overview.TopUsers = append(overview.TopUsers, m)
		}
	}

	overview.TotalQuotas = len(metrics)
	overview.TotalUsed = totalUsed
	overview.TotalQuotaLimit = totalQuota
	overview.OverSoftLimit = overSoft
	overview.OverHardLimit = overHard

	if len(metrics) > 0 {
		overview.AvgUsagePercent = round(totalUsagePercent/float64(len(metrics)), 2)
	}

	return overview
}

// ========== 图表生成 ==========

func (r *ResourceReporter) generateStorageCharts(overview *StorageOverview) []ChartData {
	charts := make([]ChartData, 0)

	// 存储使用率仪表盘
	charts = append(charts, ChartData{
		ID:    "storage_usage_gauge",
		Type:  "gauge",
		Title: "存储使用率",
		Series: []ChartSeries{
			{
				Name: "使用率",
				Data: []ChartPoint{{X: "usage", Y: overview.UsagePercent}},
			},
		},
	})

	// 卷使用量柱状图
	volumeSeries := make([]ChartPoint, 0, len(overview.Volumes))
	for _, v := range overview.Volumes {
		volumeSeries = append(volumeSeries, ChartPoint{
			X: v.Name,
			Y: float64(v.UsedCapacity) / (1024 * 1024 * 1024), // GB
		})
	}
	charts = append(charts, ChartData{
		ID:     "volume_usage_bar",
		Type:   "bar",
		Title:  "卷使用量分布",
		XAxis:  "卷名称",
		YAxis:  "使用量 (GB)",
		Series: []ChartSeries{{Name: "使用量", Data: volumeSeries}},
	})

	// 存储类型分布饼图
	if len(overview.TypeDistribution.ByFileType) > 0 {
		typeSeries := make([]ChartPoint, 0, len(overview.TypeDistribution.ByFileType))
		for _, t := range overview.TypeDistribution.ByFileType {
			typeSeries = append(typeSeries, ChartPoint{
				X: t.Type,
				Y: float64(t.Size) / (1024 * 1024 * 1024), // GB
			})
		}
		charts = append(charts, ChartData{
			ID:     "storage_type_pie",
			Type:   "pie",
			Title:  "存储类型分布",
			Series: []ChartSeries{{Name: "类型", Data: typeSeries}},
		})
	}

	return charts
}

func (r *ResourceReporter) generateBandwidthCharts(overview *BandwidthOverview) []ChartData {
	charts := make([]ChartData, 0)

	// 当前带宽速率
	charts = append(charts, ChartData{
		ID:    "bandwidth_rate_gauge",
		Type:  "gauge",
		Title: "当前带宽利用率",
		Series: []ChartSeries{
			{
				Name: "利用率",
				Data: []ChartPoint{{X: "utilization", Y: overview.Peak.PeakMbps}},
			},
		},
	})

	// 接口流量对比柱状图
	ifaceSeries := make([]ChartPoint, 0, len(overview.Interfaces))
	for _, iface := range overview.Interfaces {
		ifaceSeries = append(ifaceSeries, ChartPoint{
			X: iface.Name,
			Y: iface.Rate.TotalMbps,
		})
	}
	charts = append(charts, ChartData{
		ID:     "interface_bandwidth_bar",
		Type:   "bar",
		Title:  "接口带宽分布",
		XAxis:  "接口",
		YAxis:  "带宽 (Mbps)",
		Series: []ChartSeries{{Name: "带宽", Data: ifaceSeries}},
	})

	// 收发流量对比
	rxTxSeries := []ChartSeries{
		{Name: "接收", Data: []ChartPoint{{X: "总流量", Y: overview.Summary.TotalRxGB}}},
		{Name: "发送", Data: []ChartPoint{{X: "总流量", Y: overview.Summary.TotalTxGB}}},
	}
	charts = append(charts, ChartData{
		ID:     "rx_tx_comparison",
		Type:   "bar",
		Title:  "收发流量对比",
		YAxis:  "流量 (GB)",
		Series: rxTxSeries,
	})

	return charts
}

func (r *ResourceReporter) generateUserCharts(overview *UserResourceOverview) []ChartData {
	charts := make([]ChartData, 0)

	// Top 用户使用量柱状图
	topSeries := make([]ChartPoint, 0, len(overview.TopUsers))
	for _, u := range overview.TopUsers {
		topSeries = append(topSeries, ChartPoint{
			X: u.Username,
			Y: float64(u.UsedBytes) / (1024 * 1024 * 1024), // GB
		})
	}
	charts = append(charts, ChartData{
		ID:     "top_users_bar",
		Type:   "bar",
		Title:  "用户存储使用 Top 10",
		XAxis:  "用户",
		YAxis:  "使用量 (GB)",
		Series: []ChartSeries{{Name: "使用量", Data: topSeries}},
	})

	// 配额状态分布饼图
	quotaSeries := []ChartPoint{
		{X: "正常", Y: float64(overview.TotalUsers - overview.OverSoftLimit - overview.OverHardLimit)},
		{X: "接近限制", Y: float64(overview.OverSoftLimit)},
		{X: "超限", Y: float64(overview.OverHardLimit)},
	}
	charts = append(charts, ChartData{
		ID:     "quota_status_pie",
		Type:   "pie",
		Title:  "配额状态分布",
		Series: []ChartSeries{{Name: "状态", Data: quotaSeries}},
	})

	return charts
}

func (r *ResourceReporter) generateSystemCharts(overview *SystemResourceOverview) []ChartData {
	charts := make([]ChartData, 0)

	// CPU 使用率仪表盘
	charts = append(charts, ChartData{
		ID:    "cpu_usage_gauge",
		Type:  "gauge",
		Title: "CPU 使用率",
		Series: []ChartSeries{
			{Name: "使用率", Data: []ChartPoint{{X: "usage", Y: overview.CPUUsage}}},
		},
	})

	// 内存使用率仪表盘
	charts = append(charts, ChartData{
		ID:    "memory_usage_gauge",
		Type:  "gauge",
		Title: "内存使用率",
		Series: []ChartSeries{
			{Name: "使用率", Data: []ChartPoint{{X: "usage", Y: overview.MemoryUsage}}},
		},
	})

	// 系统负载
	loadSeries := []ChartSeries{
		{Name: "1分钟", Data: []ChartPoint{{X: "负载", Y: overview.LoadAverage.Load1}}},
		{Name: "5分钟", Data: []ChartPoint{{X: "负载", Y: overview.LoadAverage.Load5}}},
		{Name: "15分钟", Data: []ChartPoint{{X: "负载", Y: overview.LoadAverage.Load15}}},
	}
	charts = append(charts, ChartData{
		ID:     "load_average_bar",
		Type:   "bar",
		Title:  "系统负载",
		Series: loadSeries,
	})

	return charts
}

// ========== 告警检测 ==========

func (r *ResourceReporter) detectStorageAlerts(overview *StorageOverview) []ResourceAlert {
	alerts := make([]ResourceAlert, 0)
	now := time.Now()

	// 检查总存储使用率
	if overview.UsagePercent >= r.config.StorageCriticalThreshold {
		alerts = append(alerts, ResourceAlert{
			ID:           "storage_critical_total",
			Type:         "storage",
			Severity:     "critical",
			Title:        "存储使用率严重告警",
			Message:      fmt.Sprintf("总存储使用率达到 %.1f%%，超过严重阈值 %.1f%%", overview.UsagePercent, r.config.StorageCriticalThreshold),
			CurrentValue: overview.UsagePercent,
			Threshold:    r.config.StorageCriticalThreshold,
			Unit:         "%",
			TriggeredAt:  now,
			Resource:     "总存储",
		})
	} else if overview.UsagePercent >= r.config.StorageWarningThreshold {
		alerts = append(alerts, ResourceAlert{
			ID:           "storage_warning_total",
			Type:         "storage",
			Severity:     "warning",
			Title:        "存储使用率警告",
			Message:      fmt.Sprintf("总存储使用率达到 %.1f%%，超过警告阈值 %.1f%%", overview.UsagePercent, r.config.StorageWarningThreshold),
			CurrentValue: overview.UsagePercent,
			Threshold:    r.config.StorageWarningThreshold,
			Unit:         "%",
			TriggeredAt:  now,
			Resource:     "总存储",
		})
	}

	// 检查各卷使用率
	for _, v := range overview.Volumes {
		if v.UsagePercent >= r.config.StorageCriticalThreshold {
			alerts = append(alerts, ResourceAlert{
				ID:           fmt.Sprintf("volume_critical_%s", v.Name),
				Type:         "storage",
				Severity:     "critical",
				Title:        fmt.Sprintf("卷 %s 使用率严重告警", v.Name),
				Message:      fmt.Sprintf("卷 %s 使用率达到 %.1f%%", v.Name, v.UsagePercent),
				CurrentValue: v.UsagePercent,
				Threshold:    r.config.StorageCriticalThreshold,
				Unit:         "%",
				TriggeredAt:  now,
				Resource:     v.Name,
			})
		}
	}

	return alerts
}

func (r *ResourceReporter) detectBandwidthAlerts(overview *BandwidthOverview) []ResourceAlert {
	alerts := make([]ResourceAlert, 0)
	now := time.Now()

	// 检查峰值带宽
	if overview.Peak.PeakMbps > 0 {
		for _, iface := range overview.Interfaces {
			utilization := iface.Rate.TotalMbps
			if iface.BandwidthLimit > 0 {
				utilization = iface.Rate.TotalMbps / iface.BandwidthLimit * 100
			}

			if utilization >= r.config.BandwidthCriticalThreshold {
				alerts = append(alerts, ResourceAlert{
					ID:           fmt.Sprintf("bandwidth_critical_%s", iface.Name),
					Type:         "bandwidth",
					Severity:     "critical",
					Title:        fmt.Sprintf("接口 %s 带宽严重告警", iface.Name),
					Message:      fmt.Sprintf("接口 %s 带宽利用率过高", iface.Name),
					CurrentValue: utilization,
					Threshold:    r.config.BandwidthCriticalThreshold,
					Unit:         "%",
					TriggeredAt:  now,
					Resource:     iface.Name,
				})
			}
		}
	}

	return alerts
}

func (r *ResourceReporter) detectUserAlerts(overview *UserResourceOverview) []ResourceAlert {
	alerts := make([]ResourceAlert, 0)
	now := time.Now()

	// 检查超限用户
	for _, u := range overview.TopUsers {
		if u.UsagePercent >= 100 {
			alerts = append(alerts, ResourceAlert{
				ID:           fmt.Sprintf("user_quota_%s", u.Username),
				Type:         "user",
				Severity:     "critical",
				Title:        fmt.Sprintf("用户 %s 配额超限", u.Username),
				Message:      fmt.Sprintf("用户 %s 已超出配额限制", u.Username),
				CurrentValue: u.UsagePercent,
				Threshold:    100,
				Unit:         "%",
				TriggeredAt:  now,
				Resource:     u.Username,
			})
		}
	}

	return alerts
}

// ========== 建议生成 ==========

func (r *ResourceReporter) generateStorageRecommendations(overview *StorageOverview) []ResourceRecommendation {
	recommendations := make([]ResourceRecommendation, 0)
	now := time.Now()

	if overview.UsagePercent >= r.config.StorageCriticalThreshold {
		recommendations = append(recommendations, ResourceRecommendation{
			ID:             "storage_expand",
			Type:           "storage",
			Priority:       "high",
			Title:          "立即扩容存储",
			Description:    "存储使用率已达到严重水平，建议立即扩容或清理数据",
			CurrentValue:   overview.UsagePercent,
			SuggestedValue: overview.UsagePercent - 20,
			Unit:           "%",
			Impact:         "避免存储空间耗尽导致服务中断",
			Action:         "添加新硬盘或清理无用数据",
			CreatedAt:      now,
		})
	} else if overview.UsagePercent >= r.config.StorageWarningThreshold {
		recommendations = append(recommendations, ResourceRecommendation{
			ID:             "storage_plan",
			Type:           "storage",
			Priority:       "medium",
			Title:          "规划存储扩容",
			Description:    "存储使用率较高，建议规划扩容方案",
			CurrentValue:   overview.UsagePercent,
			SuggestedValue: r.config.StorageWarningThreshold - 10,
			Unit:           "%",
			Impact:         "提前规划，避免被动应对",
			Action:         "评估存储需求，制定扩容计划",
			CreatedAt:      now,
		})
	}

	// 检查是否需要清理
	if overview.StorageEfficiency.SavedSpace > 0 {
		savedPercent := float64(overview.StorageEfficiency.SavedSpace) / float64(overview.TotalCapacity) * 100
		if savedPercent > 10 {
			recommendations = append(recommendations, ResourceRecommendation{
				ID:           "storage_compress",
				Type:         "storage",
				Priority:     "low",
				Title:        "优化存储效率",
				Description:  fmt.Sprintf("压缩和去重已节省 %.1f%% 空间，可继续优化", savedPercent),
				CurrentValue: savedPercent,
				Unit:         "%",
				Impact:       "提高存储利用率",
				Action:       "对适合的数据类型启用压缩",
				CreatedAt:    now,
			})
		}
	}

	return recommendations
}

func (r *ResourceReporter) generateBandwidthRecommendations(overview *BandwidthOverview) []ResourceRecommendation {
	recommendations := make([]ResourceRecommendation, 0)
	now := time.Now()

	if overview.Peak.PeakMbps > 0 {
		for _, iface := range overview.Interfaces {
			if iface.BandwidthLimit > 0 {
				utilization := iface.Rate.TotalMbps / iface.BandwidthLimit * 100

				if utilization >= r.config.BandwidthCriticalThreshold {
					recommendations = append(recommendations, ResourceRecommendation{
						ID:             fmt.Sprintf("bandwidth_upgrade_%s", iface.Name),
						Type:           "bandwidth",
						Priority:       "high",
						Title:          fmt.Sprintf("升级接口 %s 带宽", iface.Name),
						Description:    "带宽利用率持续过高，建议升级带宽",
						CurrentValue:   iface.Rate.TotalMbps,
						SuggestedValue: iface.BandwidthLimit * 1.5,
						Unit:           "Mbps",
						Impact:         "提升网络性能，减少延迟",
						Action:         "联系网络服务商升级带宽",
						CreatedAt:      now,
					})
				} else if utilization >= r.config.BandwidthHighThreshold {
					recommendations = append(recommendations, ResourceRecommendation{
						ID:           fmt.Sprintf("bandwidth_monitor_%s", iface.Name),
						Type:         "bandwidth",
						Priority:     "medium",
						Title:        fmt.Sprintf("监控接口 %s 带宽", iface.Name),
						Description:  "带宽利用率较高，建议持续监控",
						CurrentValue: utilization,
						Unit:         "%",
						Impact:       "提前规划，避免瓶颈",
						Action:       "设置带宽监控告警",
						CreatedAt:    now,
					})
				}
			}
		}
	}

	return recommendations
}

// ========== 辅助函数 ==========

func formatBytesForResource(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// UpdateConfig 更新配置
func (r *ResourceReporter) UpdateConfig(config ResourceReportConfig) {
	r.config = config
}

// GetConfig 获取配置
func (r *ResourceReporter) GetConfig() ResourceReportConfig {
	return r.config
}
