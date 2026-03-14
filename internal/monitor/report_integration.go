package monitor

import (
	"time"

	"nas-os/internal/reports"
)

// ReportIntegration 监控报告集成服务
type ReportIntegration struct {
	manager   *Manager
	scorer    *HealthScorer
	collector *MetricsCollector
}

// NewReportIntegration 创建报告集成服务
func NewReportIntegration(manager *Manager) *ReportIntegration {
	scorer := NewHealthScorer(manager)
	collector := NewMetricsCollector(manager, scorer)

	return &ReportIntegration{
		manager:   manager,
		scorer:    scorer,
		collector: collector,
	}
}

// GetHealthScorer 获取健康评分器
func (ri *ReportIntegration) GetHealthScorer() *HealthScorer {
	return ri.scorer
}

// GetMetricsCollector 获取指标收集器
func (ri *ReportIntegration) GetMetricsCollector() *MetricsCollector {
	return ri.collector
}

// Start 启动监控和收集
func (ri *ReportIntegration) Start() {
	ri.collector.Start()
}

// Stop 停止监控和收集
func (ri *ReportIntegration) Stop() {
	ri.collector.Stop()
}

// CreateMonitorDataSource 创建监控数据源适配器
func (ri *ReportIntegration) CreateMonitorDataSource() *reports.MonitorDataSource {
	return reports.NewMonitorDataSource(reports.MonitorDataSourceConfig{
		Name: "monitor",
		GetSystemStats: func() (map[string]interface{}, error) {
			stats, err := ri.manager.GetSystemStats()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"cpu_usage":      stats.CPUUsage,
				"memory_usage":   stats.MemoryUsage,
				"memory_total":   stats.MemoryTotal,
				"memory_used":    stats.MemoryUsed,
				"memory_free":    stats.MemoryFree,
				"swap_usage":     stats.SwapUsage,
				"swap_total":     stats.SwapTotal,
				"swap_used":      stats.SwapUsed,
				"uptime":         stats.Uptime,
				"uptime_seconds": stats.UptimeSeconds,
				"load_avg":       stats.LoadAvg,
				"processes":      stats.Processes,
			}, nil
		},
		GetDiskStats: func() ([]map[string]interface{}, error) {
			stats, err := ri.manager.GetDiskStats()
			if err != nil {
				return nil, err
			}

			result := make([]map[string]interface{}, 0, len(stats))
			for _, d := range stats {
				result = append(result, map[string]interface{}{
					"device":        d.Device,
					"mount_point":   d.MountPoint,
					"total":         d.Total,
					"used":          d.Used,
					"free":          d.Free,
					"usage_percent": d.UsagePercent,
					"fs_type":       d.FSType,
				})
			}
			return result, nil
		},
		GetNetworkStats: func() ([]map[string]interface{}, error) {
			stats, err := ri.manager.GetNetworkStats()
			if err != nil {
				return nil, err
			}

			result := make([]map[string]interface{}, 0, len(stats))
			for _, n := range stats {
				result = append(result, map[string]interface{}{
					"interface":  n.Interface,
					"rx_bytes":   n.RXBytes,
					"tx_bytes":   n.TXBytes,
					"rx_packets": n.RXPackets,
					"tx_packets": n.TXPackets,
					"rx_errors":  n.RXErrors,
					"tx_errors":  n.TXErrors,
				})
			}
			return result, nil
		},
		GetHealthScore: func() (map[string]interface{}, error) {
			score := ri.scorer.CalculateScore()
			if score == nil {
				return nil, nil
			}

			issues := make([]map[string]interface{}, 0, len(score.Issues))
			for _, issue := range score.Issues {
				issues = append(issues, map[string]interface{}{
					"component": issue.Component,
					"severity":  issue.Severity,
					"message":   issue.Message,
					"value":     issue.Value,
					"threshold": issue.Threshold,
				})
			}

			return map[string]interface{}{
				"total_score": score.TotalScore,
				"grade":       score.Grade,
				"trend": map[string]interface{}{
					"direction": score.Trend.Direction,
					"change":    score.Trend.Change,
				},
				"components": map[string]interface{}{
					"cpu": map[string]interface{}{
						"score":  score.Components.CPU.Score,
						"status": score.Components.CPU.Status,
					},
					"memory": map[string]interface{}{
						"score":  score.Components.Memory.Score,
						"status": score.Components.Memory.Status,
					},
					"disk": map[string]interface{}{
						"score":  score.Components.Disk.Score,
						"status": score.Components.Disk.Status,
					},
					"network": map[string]interface{}{
						"score":  score.Components.Network.Score,
						"status": score.Components.Network.Status,
					},
				},
				"issues":          issues,
				"recommendations": score.Recommendations,
			}, nil
		},
		GetTrendData: func(period string) ([]map[string]interface{}, error) {
			var trend *TrendData
			switch period {
			case "hourly":
				trend = ri.collector.GetHourlyTrend()
			case "weekly":
				trend = ri.collector.GetWeeklyTrend()
			default:
				trend = ri.collector.GetDailyTrend()
			}

			if trend == nil {
				return []map[string]interface{}{}, nil
			}

			result := make([]map[string]interface{}, 0, len(trend.Data))
			for _, d := range trend.Data {
				result = append(result, map[string]interface{}{
					"timestamp":    d.Timestamp,
					"cpu_usage":    d.CPUUsage,
					"memory_usage": d.MemoryUsage,
					"disk_usage":   d.DiskUsage,
					"health_score": d.HealthScore,
					"load_avg":     d.LoadAvg,
				})
			}
			return result, nil
		},
		GetResourceReport: func(period string) (map[string]interface{}, error) {
			report := ri.collector.GenerateResourceReport(period)
			if report == nil {
				return nil, nil
			}

			diskAnalysis := make([]map[string]interface{}, 0, len(report.DiskAnalysis))
			for _, d := range report.DiskAnalysis {
				diskAnalysis = append(diskAnalysis, map[string]interface{}{
					"device":        d.Device,
					"mount_point":   d.MountPoint,
					"total":         d.Total,
					"used":          d.Used,
					"free":          d.Free,
					"usage_percent": d.UsagePercent,
					"trend":         d.Trend,
					"status":        d.Status,
				})
			}

			return map[string]interface{}{
				"generated_at": report.GeneratedAt,
				"period":       report.Period,
				"system_info": map[string]interface{}{
					"hostname":       report.SystemInfo.Hostname,
					"uptime":         report.SystemInfo.Uptime,
					"uptime_seconds": report.SystemInfo.UptimeSeconds,
				},
				"resource_usage": map[string]interface{}{
					"cpu": map[string]interface{}{
						"percent": report.ResourceUsage.CPU.Percent,
						"average": report.ResourceUsage.CPU.Average,
						"peak":    report.ResourceUsage.CPU.Peak,
						"status":  report.ResourceUsage.CPU.Status,
					},
					"memory": map[string]interface{}{
						"used":    report.ResourceUsage.Memory.Used,
						"total":   report.ResourceUsage.Memory.Total,
						"percent": report.ResourceUsage.Memory.Percent,
						"average": report.ResourceUsage.Memory.Average,
						"peak":    report.ResourceUsage.Memory.Peak,
						"status":  report.ResourceUsage.Memory.Status,
					},
					"network": map[string]interface{}{
						"rx_bytes":   report.ResourceUsage.Network.RXBytes,
						"tx_bytes":   report.ResourceUsage.Network.TXBytes,
						"rx_packets": report.ResourceUsage.Network.RXPackets,
						"tx_packets": report.ResourceUsage.Network.TXPackets,
					},
				},
				"trends": map[string]interface{}{
					"cpu_trend":    report.Trends.CPUTrend,
					"memory_trend": report.Trends.MemoryTrend,
					"disk_trend":   report.Trends.DiskTrend,
					"health_score": report.Trends.HealthScore,
					"health_grade": report.Trends.HealthGrade,
				},
				"disk_analysis":   diskAnalysis,
				"recommendations": report.Recommendations,
			}, nil
		},
	})
}

// CreateResourceReportGenerator 创建资源报告生成器
func (ri *ReportIntegration) CreateResourceReportGenerator() *reports.ResourceReportGenerator {
	return reports.NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		report := ri.collector.GenerateResourceReport(period)
		if report == nil {
			return nil, nil
		}

		diskAnalysis := make([]map[string]interface{}, 0, len(report.DiskAnalysis))
		for _, d := range report.DiskAnalysis {
			diskAnalysis = append(diskAnalysis, map[string]interface{}{
				"device":        d.Device,
				"mount_point":   d.MountPoint,
				"total":         d.Total,
				"used":          d.Used,
				"free":          d.Free,
				"usage_percent": d.UsagePercent,
				"trend":         d.Trend,
				"status":        d.Status,
			})
		}

		return map[string]interface{}{
			"generated_at": report.GeneratedAt,
			"period":       report.Period,
			"system_info": map[string]interface{}{
				"hostname":       report.SystemInfo.Hostname,
				"uptime":         report.SystemInfo.Uptime,
				"uptime_seconds": report.SystemInfo.UptimeSeconds,
			},
			"resource_usage": map[string]interface{}{
				"cpu": map[string]interface{}{
					"percent": report.ResourceUsage.CPU.Percent,
					"average": report.ResourceUsage.CPU.Average,
					"peak":    report.ResourceUsage.CPU.Peak,
					"status":  report.ResourceUsage.CPU.Status,
				},
				"memory": map[string]interface{}{
					"used":    report.ResourceUsage.Memory.Used,
					"total":   report.ResourceUsage.Memory.Total,
					"percent": report.ResourceUsage.Memory.Percent,
					"average": report.ResourceUsage.Memory.Average,
					"peak":    report.ResourceUsage.Memory.Peak,
					"status":  report.ResourceUsage.Memory.Status,
				},
			},
			"trends": map[string]interface{}{
				"cpu_trend":    report.Trends.CPUTrend,
				"memory_trend": report.Trends.MemoryTrend,
				"health_score": report.Trends.HealthScore,
				"health_grade": report.Trends.HealthGrade,
			},
			"disk_analysis":   diskAnalysis,
			"recommendations": report.Recommendations,
		}, nil
	})
}

// GetQuickHealthReport 获取快速健康报告
func (ri *ReportIntegration) GetQuickHealthReport() map[string]interface{} {
	score := ri.scorer.CalculateScore()
	stats, _ := ri.manager.GetSystemStats()

	report := map[string]interface{}{
		"timestamp": time.Now(),
	}

	if score != nil {
		report["health_score"] = score.TotalScore
		report["health_grade"] = score.Grade
		report["trend"] = score.Trend.Direction
		report["recommendations"] = score.Recommendations
	}

	if stats != nil {
		report["cpu_usage"] = stats.CPUUsage
		report["memory_usage"] = stats.MemoryUsage
		report["uptime"] = stats.Uptime
	}

	return report
}
