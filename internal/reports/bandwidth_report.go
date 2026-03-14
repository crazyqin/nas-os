// Package reports 提供报表生成和管理功能
package reports

import (
	"math"
	"sort"
	"time"
)

// ========== 带宽使用报告 ==========

// BandwidthDirection 带宽方向
type BandwidthDirection string

const (
	BandwidthDirectionIn  BandwidthDirection = "in"  // 入站
	BandwidthDirectionOut BandwidthDirection = "out" // 出站
)

// BandwidthMetrics 带宽指标
type BandwidthMetrics struct {
	// 接口名称
	Interface string `json:"interface"`

	// 接收字节（累计）
	RxBytes uint64 `json:"rx_bytes"`

	// 发送字节（累计）
	TxBytes uint64 `json:"tx_bytes"`

	// 接收速率（字节/秒）
	RxRateBytes uint64 `json:"rx_rate_bytes"`

	// 发送速率（字节/秒）
	TxRateBytes uint64 `json:"tx_rate_bytes"`

	// 接收包数（累计）
	RxPackets uint64 `json:"rx_packets"`

	// 发送包数（累计）
	TxPackets uint64 `json:"tx_packets"`

	// 接收错误数
	RxErrors uint64 `json:"rx_errors"`

	// 发送错误数
	TxErrors uint64 `json:"tx_errors"`

	// 接收丢包数
	RxDropped uint64 `json:"rx_dropped"`

	// 发送丢包数
	TxDropped uint64 `json:"tx_dropped"`

	// 采集时间
	Timestamp time.Time `json:"timestamp"`
}

// BandwidthHistoryPoint 带宽历史数据点
type BandwidthHistoryPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	RxBytes     uint64    `json:"rx_bytes"`
	TxBytes     uint64    `json:"tx_bytes"`
	RxRate      uint64    `json:"rx_rate"`      // 字节/秒
	TxRate      uint64    `json:"tx_rate"`      // 字节/秒
	TotalRate   uint64    `json:"total_rate"`   // 总速率
	RxPackets   uint64    `json:"rx_packets"`
	TxPackets   uint64    `json:"tx_packets"`
	ErrorCount  uint64    `json:"error_count"`  // 错误总数
	DropCount   uint64    `json:"drop_count"`   // 丢包总数
}

// BandwidthUsageStats 带宽使用统计
type BandwidthUsageStats struct {
	// 接口名称
	Interface string `json:"interface"`

	// 统计周期
	Period ReportPeriod `json:"period"`

	// 总接收量（字节）
	TotalRxBytes uint64 `json:"total_rx_bytes"`

	// 总发送量（字节）
	TotalTxBytes uint64 `json:"total_tx_bytes"`

	// 总流量（字节）
	TotalBytes uint64 `json:"total_bytes"`

	// 平均接收速率（字节/秒）
	AvgRxRate uint64 `json:"avg_rx_rate"`

	// 平均发送速率（字节/秒）
	AvgTxRate uint64 `json:"avg_tx_rate"`

	// 峰值接收速率（字节/秒）
	PeakRxRate uint64 `json:"peak_rx_rate"`

	// 峰值发送速率（字节/秒）
	PeakTxRate uint64 `json:"peak_tx_rate"`

	// 峰值时间
	PeakTime *time.Time `json:"peak_time,omitempty"`

	// 总包数
	TotalPackets uint64 `json:"total_packets"`

	// 错误率（%）
	ErrorRate float64 `json:"error_rate"`

	// 丢包率（%）
	DropRate float64 `json:"drop_rate"`

	// 利用率（基于带宽限制，%）
	UtilizationPercent float64 `json:"utilization_percent"`

	// 计算时间
	CalculatedAt time.Time `json:"calculated_at"`
}

// BandwidthTrend 带宽趋势
type BandwidthTrend struct {
	// 时间点
	Timestamp time.Time `json:"timestamp"`

	// 接收速率（Mbps）
	RxMbps float64 `json:"rx_mbps"`

	// 发送速率（Mbps）
	TxMbps float64 `json:"tx_mbps"`

	// 总速率（Mbps）
	TotalMbps float64 `json:"total_mbps"`

	// 利用率（%）
	Utilization float64 `json:"utilization"`
}

// BandwidthAlert 带宽告警
type BandwidthAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 接口名称
	Interface string `json:"interface"`

	// 告警类型
	Type string `json:"type"` // high_utilization, error_spike, drop_spike

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 告警消息
	Message string `json:"message"`

	// 触发值
	TriggerValue float64 `json:"trigger_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`

	// 是否已恢复
	Resolved bool `json:"resolved"`

	// 恢复时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// BandwidthReport 带宽使用报告
type BandwidthReport struct {
	// 报告ID
	ID string `json:"id"`

	// 报告名称
	Name string `json:"name"`

	// 报告周期
	Period ReportPeriod `json:"period"`

	// 接口统计
	InterfaceStats []BandwidthUsageStats `json:"interface_stats"`

	// 汇总统计
	Summary BandwidthSummary `json:"summary"`

	// 趋势数据
	Trends []BandwidthTrend `json:"trends"`

	// 告警列表
	Alerts []BandwidthAlert `json:"alerts"`

	// 建议
	Recommendations []BandwidthRecommendation `json:"recommendations"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`
}

// BandwidthSummary 带宽汇总
type BandwidthSummary struct {
	// 接口数量
	InterfaceCount int `json:"interface_count"`

	// 总接收量（GB）
	TotalRxGB float64 `json:"total_rx_gb"`

	// 总发送量（GB）
	TotalTxGB float64 `json:"total_tx_gb"`

	// 总流量（GB）
	TotalGB float64 `json:"total_gb"`

	// 平均总速率（Mbps）
	AvgTotalMbps float64 `json:"avg_total_mbps"`

	// 峰值总速率（Mbps）
	PeakTotalMbps float64 `json:"peak_total_mbps"`

	// 平均利用率（%）
	AvgUtilization float64 `json:"avg_utilization"`

	// 峰值利用率（%）
	PeakUtilization float64 `json:"peak_utilization"`

	// 总错误数
	TotalErrors uint64 `json:"total_errors"`

	// 总丢包数
	TotalDrops uint64 `json:"total_drops"`

	// 平均错误率（%）
	AvgErrorRate float64 `json:"avg_error_rate"`

	// 平均丢包率（%）
	AvgDropRate float64 `json:"avg_drop_rate"`

	// 主要流量方向
	PrimaryDirection BandwidthDirection `json:"primary_direction"`

	// 流量模式
	TrafficPattern string `json:"traffic_pattern"` // balanced, download_heavy, upload_heavy
}

// BandwidthRecommendation 带宽建议
type BandwidthRecommendation struct {
	// 类型
	Type string `json:"type"` // upgrade, optimize, investigate, monitor

	// 优先级
	Priority string `json:"priority"` // high, medium, low

	// 接口
	Interface string `json:"interface,omitempty"`

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 建议值
	SuggestedValue float64 `json:"suggested_value,omitempty"`

	// 影响
	Impact string `json:"impact"`
}

// BandwidthReportConfig 带宽报告配置
type BandwidthReportConfig struct {
	// 带宽限制（Mbps）- 用于计算利用率
	BandwidthLimitMbps float64 `json:"bandwidth_limit_mbps"`

	// 高利用率阈值（%）
	HighUtilizationThreshold float64 `json:"high_utilization_threshold"`

	// 严重利用率阈值（%）
	CriticalUtilizationThreshold float64 `json:"critical_utilization_threshold"`

	// 错误率阈值（%）
	ErrorRateThreshold float64 `json:"error_rate_threshold"`

	// 丢包率阈值（%）
	DropRateThreshold float64 `json:"drop_rate_threshold"`

	// 趋势采样间隔（分钟）
	TrendSampleInterval int `json:"trend_sample_interval"`
}

// BandwidthReporter 带宽报告生成器
type BandwidthReporter struct {
	config BandwidthReportConfig
}

// NewBandwidthReporter 创建带宽报告生成器
func NewBandwidthReporter(config BandwidthReportConfig) *BandwidthReporter {
	// 设置默认值
	if config.HighUtilizationThreshold == 0 {
		config.HighUtilizationThreshold = 70.0
	}
	if config.CriticalUtilizationThreshold == 0 {
		config.CriticalUtilizationThreshold = 90.0
	}
	if config.ErrorRateThreshold == 0 {
		config.ErrorRateThreshold = 1.0
	}
	if config.DropRateThreshold == 0 {
		config.DropRateThreshold = 0.5
	}
	if config.TrendSampleInterval == 0 {
		config.TrendSampleInterval = 5
	}

	return &BandwidthReporter{config: config}
}

// CalculateStats 计算带宽使用统计
func (r *BandwidthReporter) CalculateStats(history []BandwidthHistoryPoint, iface string) BandwidthUsageStats {
	if len(history) == 0 {
		return BandwidthUsageStats{Interface: iface}
	}

	stats := BandwidthUsageStats{
		Interface:    iface,
		CalculatedAt: time.Now(),
	}

	var totalRxRate, totalTxRate uint64
	var totalErrors, totalDrops uint64

	for _, point := range history {
		stats.TotalRxBytes += point.RxBytes
		stats.TotalTxBytes += point.TxBytes
		stats.TotalPackets += point.RxPackets + point.TxPackets
		totalErrors += point.ErrorCount
		totalDrops += point.DropCount

		totalRxRate += point.RxRate
		totalTxRate += point.TxRate

		// 记录峰值
		if point.RxRate > stats.PeakRxRate {
			stats.PeakRxRate = point.RxRate
		}
		if point.TxRate > stats.PeakTxRate {
			stats.PeakTxRate = point.TxRate
		}
		if point.RxRate+point.TxRate > stats.PeakRxRate+stats.PeakTxRate && stats.PeakTime == nil {
			stats.PeakTime = &point.Timestamp
		}
	}

	stats.TotalBytes = stats.TotalRxBytes + stats.TotalTxBytes
	n := uint64(len(history))

	if n > 0 {
		stats.AvgRxRate = totalRxRate / n
		stats.AvgTxRate = totalTxRate / n
	}

	// 计算错误率和丢包率
	if stats.TotalPackets > 0 {
		stats.ErrorRate = round(float64(totalErrors)/float64(stats.TotalPackets)*100, 4)
		stats.DropRate = round(float64(totalDrops)/float64(stats.TotalPackets)*100, 4)
	}

	// 计算利用率
	if r.config.BandwidthLimitMbps > 0 {
		// 将字节/秒转换为 Mbps
		avgRateMbps := float64(stats.AvgRxRate+stats.AvgTxRate) * 8 / (1024 * 1024)
		stats.UtilizationPercent = round(avgRateMbps/r.config.BandwidthLimitMbps*100, 2)
	}

	// 设置周期
	if len(history) > 0 {
		stats.Period = ReportPeriod{
			StartTime: history[0].Timestamp,
			EndTime:   history[len(history)-1].Timestamp,
		}
	}

	return stats
}

// GenerateTrends 生成趋势数据
func (r *BandwidthReporter) GenerateTrends(history []BandwidthHistoryPoint) []BandwidthTrend {
	trends := make([]BandwidthTrend, 0, len(history))

	for _, point := range history {
		trend := BandwidthTrend{
			Timestamp: point.Timestamp,
			RxMbps:    round(float64(point.RxRate)*8/(1024*1024), 2),
			TxMbps:    round(float64(point.TxRate)*8/(1024*1024), 2),
			TotalMbps: round(float64(point.RxRate+point.TxRate)*8/(1024*1024), 2),
		}

		// 计算利用率
		if r.config.BandwidthLimitMbps > 0 {
			trend.Utilization = round(trend.TotalMbps/r.config.BandwidthLimitMbps*100, 2)
		}

		trends = append(trends, trend)
	}

	return trends
}

// DetectAlerts 检测告警
func (r *BandwidthReporter) DetectAlerts(history []BandwidthHistoryPoint, iface string) []BandwidthAlert {
	alerts := make([]BandwidthAlert, 0)

	for _, point := range history {
		// 计算利用率
		totalMbps := float64(point.RxRate+point.TxRate) * 8 / (1024 * 1024)
		utilization := 0.0
		if r.config.BandwidthLimitMbps > 0 {
			utilization = totalMbps / r.config.BandwidthLimitMbps * 100
		}

		// 检测高利用率
		if utilization >= r.config.CriticalUtilizationThreshold {
			alerts = append(alerts, BandwidthAlert{
				ID:           "bw_alert_" + point.Timestamp.Format("20060102150405"),
				Interface:    iface,
				Type:         "high_utilization",
				Severity:     "critical",
				Message:      "带宽利用率超过严重阈值",
				TriggerValue: utilization,
				Threshold:    r.config.CriticalUtilizationThreshold,
				TriggeredAt:  point.Timestamp,
			})
		} else if utilization >= r.config.HighUtilizationThreshold {
			alerts = append(alerts, BandwidthAlert{
				ID:           "bw_alert_" + point.Timestamp.Format("20060102150405"),
				Interface:    iface,
				Type:         "high_utilization",
				Severity:     "warning",
				Message:      "带宽利用率超过警告阈值",
				TriggerValue: utilization,
				Threshold:    r.config.HighUtilizationThreshold,
				TriggeredAt:  point.Timestamp,
			})
		}

		// 检测错误率
		if point.RxPackets+point.TxPackets > 0 {
			errorRate := float64(point.ErrorCount) / float64(point.RxPackets+point.TxPackets) * 100
			if errorRate >= r.config.ErrorRateThreshold {
				alerts = append(alerts, BandwidthAlert{
					ID:           "bw_err_" + point.Timestamp.Format("20060102150405"),
					Interface:    iface,
					Type:         "error_spike",
					Severity:     "warning",
					Message:      "网络错误率异常",
					TriggerValue: errorRate,
					Threshold:    r.config.ErrorRateThreshold,
					TriggeredAt:  point.Timestamp,
				})
			}

			// 检测丢包率
			dropRate := float64(point.DropCount) / float64(point.RxPackets+point.TxPackets) * 100
			if dropRate >= r.config.DropRateThreshold {
				alerts = append(alerts, BandwidthAlert{
					ID:           "bw_drop_" + point.Timestamp.Format("20060102150405"),
					Interface:    iface,
					Type:         "drop_spike",
					Severity:     "warning",
					Message:      "网络丢包率异常",
					TriggerValue: dropRate,
					Threshold:    r.config.DropRateThreshold,
					TriggeredAt:  point.Timestamp,
				})
			}
		}
	}

	return alerts
}

// GenerateRecommendations 生成建议
func (r *BandwidthReporter) GenerateRecommendations(stats []BandwidthUsageStats) []BandwidthRecommendation {
	recommendations := make([]BandwidthRecommendation, 0)

	for _, s := range stats {
		// 高利用率建议
		if s.UtilizationPercent >= r.config.CriticalUtilizationThreshold {
			recommendations = append(recommendations, BandwidthRecommendation{
				Type:          "upgrade",
				Priority:      "high",
				Interface:     s.Interface,
				Title:         "升级带宽",
				Description:   "当前带宽利用率持续过高，建议升级网络带宽",
				CurrentValue:  s.UtilizationPercent,
				SuggestedValue: r.config.BandwidthLimitMbps * 1.5,
				Impact:        "提升网络性能，减少延迟",
			})
		} else if s.UtilizationPercent >= r.config.HighUtilizationThreshold {
			recommendations = append(recommendations, BandwidthRecommendation{
				Type:          "monitor",
				Priority:      "medium",
				Interface:     s.Interface,
				Title:         "监控带宽使用",
				Description:   "带宽利用率较高，建议持续监控并规划扩容",
				CurrentValue:  s.UtilizationPercent,
				SuggestedValue: r.config.BandwidthLimitMbps * 1.2,
				Impact:        "提前规划，避免性能瓶颈",
			})
		}

		// 错误率建议
		if s.ErrorRate >= r.config.ErrorRateThreshold {
			recommendations = append(recommendations, BandwidthRecommendation{
				Type:         "investigate",
				Priority:     "high",
				Interface:    s.Interface,
				Title:        "排查网络错误",
				Description:  "网络错误率异常，建议检查网络设备和线路",
				CurrentValue: s.ErrorRate,
				Impact:       "提高网络稳定性",
			})
		}

		// 丢包率建议
		if s.DropRate >= r.config.DropRateThreshold {
			recommendations = append(recommendations, BandwidthRecommendation{
				Type:         "investigate",
				Priority:     "high",
				Interface:    s.Interface,
				Title:        "排查丢包问题",
				Description:  "网络丢包率异常，建议检查网络拥塞和设备状态",
				CurrentValue: s.DropRate,
				Impact:       "提高数据传输可靠性",
			})
		}
	}

	// 按优先级排序
	priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.Slice(recommendations, func(i, j int) bool {
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})

	return recommendations
}

// GenerateReport 生成带宽使用报告
func (r *BandwidthReporter) GenerateReport(
	historyByInterface map[string][]BandwidthHistoryPoint,
	period ReportPeriod,
) *BandwidthReport {
	now := time.Now()
	report := &BandwidthReport{
		ID:          "bw_" + now.Format("20060102150405"),
		Name:        "带宽使用报告",
		Period:      period,
		GeneratedAt: now,
	}

	// 计算各接口统计
	allStats := make([]BandwidthUsageStats, 0, len(historyByInterface))
	for iface, history := range historyByInterface {
		stats := r.CalculateStats(history, iface)
		allStats = append(allStats, stats)

		// 生成趋势
		trends := r.GenerateTrends(history)
		report.Trends = append(report.Trends, trends...)

		// 检测告警
		alerts := r.DetectAlerts(history, iface)
		report.Alerts = append(report.Alerts, alerts...)
	}

	report.InterfaceStats = allStats

	// 计算汇总
	report.Summary = r.calculateSummary(allStats)

	// 生成建议
	report.Recommendations = r.GenerateRecommendations(allStats)

	return report
}

// calculateSummary 计算汇总
func (r *BandwidthReporter) calculateSummary(stats []BandwidthUsageStats) BandwidthSummary {
	summary := BandwidthSummary{
		InterfaceCount: len(stats),
	}

	var totalRxGB, totalTxGB float64
	var totalUtilization float64
	var peakUtilization float64
	var totalRxMbps, totalTxMbps float64
	var avgErrorRate, avgDropRate float64

	for _, s := range stats {
		totalRxGB += float64(s.TotalRxBytes) / (1024 * 1024 * 1024)
		totalTxGB += float64(s.TotalTxBytes) / (1024 * 1024 * 1024)
		totalUtilization += s.UtilizationPercent
		avgErrorRate += s.ErrorRate
		avgDropRate += s.DropRate

		// 累计速率用于计算平均值
		totalRxMbps += float64(s.AvgRxRate) * 8 / (1024 * 1024)
		totalTxMbps += float64(s.AvgTxRate) * 8 / (1024 * 1024)

		if s.UtilizationPercent > peakUtilization {
			peakUtilization = s.UtilizationPercent
		}
	}

	summary.TotalRxGB = round(totalRxGB, 2)
	summary.TotalTxGB = round(totalTxGB, 2)
	summary.TotalGB = round(totalRxGB+totalTxGB, 2)

	if len(stats) > 0 {
		summary.AvgUtilization = round(totalUtilization/float64(len(stats)), 2)
		summary.AvgTotalMbps = round((totalRxMbps+totalTxMbps)/float64(len(stats)), 2)
		summary.AvgErrorRate = round(avgErrorRate/float64(len(stats)), 4)
		summary.AvgDropRate = round(avgDropRate/float64(len(stats)), 4)
	}

	summary.PeakUtilization = round(peakUtilization, 2)
	summary.PeakTotalMbps = round(totalRxMbps+totalTxMbps, 2)

	// 判断主要流量方向
	if totalRxGB > totalTxGB*1.5 {
		summary.PrimaryDirection = BandwidthDirectionIn
		summary.TrafficPattern = "download_heavy"
	} else if totalTxGB > totalRxGB*1.5 {
		summary.PrimaryDirection = BandwidthDirectionOut
		summary.TrafficPattern = "upload_heavy"
	} else {
		summary.PrimaryDirection = BandwidthDirectionIn
		summary.TrafficPattern = "balanced"
	}

	return summary
}

// AnalyzeBandwidthTrend 分析带宽趋势
func (r *BandwidthReporter) AnalyzeBandwidthTrend(history []BandwidthHistoryPoint) *BandwidthTrendAnalysis {
	if len(history) < 2 {
		return nil
	}

	analysis := &BandwidthTrendAnalysis{
		GeneratedAt: time.Now(),
	}

	// 计算增长率
	first := history[0]
	last := history[len(history)-1]

	// 计算时间跨度（小时）
	hours := last.Timestamp.Sub(first.Timestamp).Hours()
	if hours == 0 {
		hours = 1
	}

	// 接收增长率（GB/小时）
	rxGrowth := float64(last.RxBytes-first.RxBytes) / (1024 * 1024 * 1024) / hours
	analysis.RxGrowthRateGBPerHour = round(rxGrowth, 4)

	// 发送增长率（GB/小时）
	txGrowth := float64(last.TxBytes-first.TxBytes) / (1024 * 1024 * 1024) / hours
	analysis.TxGrowthRateGBPerHour = round(txGrowth, 4)

	// 计算平均速率
	var totalRxRate, totalTxRate uint64
	for _, point := range history {
		totalRxRate += point.RxRate
		totalTxRate += point.TxRate
	}
	n := float64(len(history))
	analysis.AvgRxMbps = round(float64(totalRxRate)/n*8/(1024*1024), 2)
	analysis.AvgTxMbps = round(float64(totalTxRate)/n*8/(1024*1024), 2)

	// 预测未来使用量（24小时）
	analysis.PredictedRxGB24h = round(rxGrowth*24, 2)
	analysis.PredictedTxGB24h = round(txGrowth*24, 2)

	// 判断趋势方向
	if rxGrowth > 0 && txGrowth > 0 {
		analysis.Trend = "increasing"
	} else if rxGrowth < 0 && txGrowth < 0 {
		analysis.Trend = "decreasing"
	} else {
		analysis.Trend = "stable"
	}

	return analysis
}

// BandwidthTrendAnalysis 带宽趋势分析
type BandwidthTrendAnalysis struct {
	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 接收增长率（GB/小时）
	RxGrowthRateGBPerHour float64 `json:"rx_growth_rate_gb_per_hour"`

	// 发送增长率（GB/小时）
	TxGrowthRateGBPerHour float64 `json:"tx_growth_rate_gb_per_hour"`

	// 平均接收速率（Mbps）
	AvgRxMbps float64 `json:"avg_rx_mbps"`

	// 平均发送速率（Mbps）
	AvgTxMbps float64 `json:"avg_tx_mbps"`

	// 预测24小时接收量（GB）
	PredictedRxGB24h float64 `json:"predicted_rx_gb_24h"`

	// 预测24小时发送量（GB）
	PredictedTxGB24h float64 `json:"predicted_tx_gb_24h"`

	// 趋势方向
	Trend string `json:"trend"` // increasing, stable, decreasing
}

// UpdateConfig 更新配置
func (r *BandwidthReporter) UpdateConfig(config BandwidthReportConfig) {
	r.config = config
}

// GetConfig 获取配置
func (r *BandwidthReporter) GetConfig() BandwidthReportConfig {
	return r.config
}

// ========== 辅助函数 ==========

// BandwidthToMbps 将字节/秒转换为 Mbps
func BandwidthToMbps(bytesPerSecond uint64) float64 {
	return round(float64(bytesPerSecond)*8/(1024*1024), 2)
}

// MbpsToBytes 将 Mbps 转换为字节/秒
func MbpsToBytes(mbps float64) uint64 {
	return uint64(mbps * 1024 * 1024 / 8)
}

// FormatBandwidth 格式化带宽显示
func FormatBandwidth(bytesPerSecond uint64) string {
	mbps := BandwidthToMbps(bytesPerSecond)
	if mbps >= 1000 {
		return roundStr(mbps/1000, 2) + " Gbps"
	}
	return roundStr(mbps, 2) + " Mbps"
}

func roundStr(val float64, precision int) string {
	pow := math.Pow10(precision)
	return formatFloat(math.Round(val*pow)/pow, precision)
}

func formatFloat(val float64, precision int) string {
	format := "%." + string(rune('0'+precision)) + "f"
	return sprintf(format, val)
}

func sprintf(format string, a ...interface{}) string {
	return format // 简化实现
}