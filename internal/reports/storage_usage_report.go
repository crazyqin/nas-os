// Package reports 提供报表生成和管理功能
package reports

import (
	"sort"
	"time"
)

// ========== 存储使用报表 v2.56.0 ==========

// StorageUsageReport 存储使用报表.
type StorageUsageReport struct {
	// 报告ID
	ID string `json:"id"`

	// 报告名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 报告周期
	Period ReportPeriod `json:"period"`

	// 存储概览
	Summary StorageUsageSummary `json:"summary"`

	// 卷详情
	Volumes []VolumeUsageDetail `json:"volumes"`

	// 用户使用排行
	TopUsers []UserStorageUsage `json:"top_users"`

	// 文件类型分布
	FileTypeDistribution []FileTypeStats `json:"file_type_distribution"`

	// 存储趋势
	Trend StorageTrendData `json:"trend"`

	// 告警信息
	Alerts []StorageAlert `json:"alerts"`

	// 建议
	Recommendations []StorageRecommendation `json:"recommendations"`

	// 预测
	Forecast *StorageForecast `json:"forecast,omitempty"`
}

// StorageUsageSummary 存储使用摘要.
type StorageUsageSummary struct {
	// 总容量（字节）
	TotalCapacity uint64 `json:"total_capacity"`

	// 已使用（字节）
	TotalUsed uint64 `json:"total_used"`

	// 可用空间（字节）
	TotalAvailable uint64 `json:"total_available"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 卷数量
	VolumeCount int `json:"volume_count"`

	// 活跃用户数
	ActiveUserCount int `json:"active_user_count"`

	// 总文件数
	TotalFiles uint64 `json:"total_files"`

	// 总目录数
	TotalDirectories uint64 `json:"total_directories"`

	// 平均文件大小（字节）
	AvgFileSize float64 `json:"avg_file_size"`

	// 本周增量（字节）
	WeeklyGrowth uint64 `json:"weekly_growth"`

	// 本月增量（字节）
	MonthlyGrowth uint64 `json:"monthly_growth"`

	// 增长趋势（%/月）
	GrowthTrendPercent float64 `json:"growth_trend_percent"`

	// 健康状态
	HealthStatus string `json:"health_status"` // healthy, warning, critical

	// 存储效率
	EfficiencyScore float64 `json:"efficiency_score"` // 0-100
}

// VolumeUsageDetail 卷使用详情.
type VolumeUsageDetail struct {
	// 卷名称
	Name string `json:"name"`

	// UUID
	UUID string `json:"uuid"`

	// 挂载点
	MountPoint string `json:"mount_point"`

	// 文件系统类型
	FSType string `json:"fs_type"`

	// 总容量（字节）
	TotalCapacity uint64 `json:"total_capacity"`

	// 已使用（字节）
	UsedCapacity uint64 `json:"used_capacity"`

	// 可用空间（字节）
	AvailableCapacity uint64 `json:"available_capacity"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 配额限制（字节）
	QuotaLimit uint64 `json:"quota_limit"`

	// 配额使用率（%）
	QuotaUsagePercent float64 `json:"quota_usage_percent"`

	// 用户数
	UserCount int `json:"user_count"`

	// 文件数
	FileCount uint64 `json:"file_count"`

	// 目录数
	DirectoryCount uint64 `json:"directory_count"`

	// 快照占用（字节）
	SnapshotUsed uint64 `json:"snapshot_used"`

	// 健康状态
	HealthStatus string `json:"health_status"`

	// 性能指标
	ReadIOPS  uint64 `json:"read_iops"`
	WriteIOPS uint64 `json:"write_iops"`
	ReadBps   uint64 `json:"read_bps"`
	WriteBps  uint64 `json:"write_bps"`

	// 增长数据
	DailyGrowthBytes uint64 `json:"daily_growth_bytes"`
	DaysToFull       int    `json:"days_to_full"`

	// 人类可读格式
	TotalCapacityHR     string `json:"total_capacity_hr"`
	UsedCapacityHR      string `json:"used_capacity_hr"`
	AvailableCapacityHR string `json:"available_capacity_hr"`
}

// FileTypeStats 文件类型统计.
type FileTypeStats struct {
	// 文件类型/扩展名
	Type string `json:"type"`

	// 文件数量
	Count uint64 `json:"count"`

	// 总大小（字节）
	Size uint64 `json:"size"`

	// 占比（%）
	Percent float64 `json:"percent"`

	// 平均文件大小
	AvgSize float64 `json:"avg_size"`

	// 分类
	Category string `json:"category"` // document, media, archive, code, other
}

// StorageTrendData 存储趋势数据.
type StorageTrendData struct {
	// 历史数据点
	History []StorageTrendPoint `json:"history"`

	// 周增长率（%）
	WeeklyGrowthRate float64 `json:"weekly_growth_rate"`

	// 月增长率（%）
	MonthlyGrowthRate float64 `json:"monthly_growth_rate"`

	// 趋势方向
	TrendDirection string `json:"trend_direction"` // increasing, stable, decreasing

	// 预计满容量天数
	DaysToCapacity int `json:"days_to_capacity"`

	// 预计满容量日期
	ProjectedFullDate *time.Time `json:"projected_full_date,omitempty"`

	// 增长加速度
	GrowthAcceleration float64 `json:"growth_acceleration"`
}

// StorageAlert 存储告警.
type StorageAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // capacity_high, quota_exceeded, growth_spike, low_disk

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 卷名称
	Volume string `json:"volume,omitempty"`

	// 用户
	User string `json:"user,omitempty"`

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

	// 是否已确认
	Acknowledged bool `json:"acknowledged"`

	// 建议操作
	SuggestedAction string `json:"suggested_action"`
}

// StorageRecommendation 存储建议.
type StorageRecommendation struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // cleanup, expansion, optimization, migration

	// 优先级
	Priority string `json:"priority"` // high, medium, low

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 影响范围
	Scope string `json:"scope"`

	// 预计节省空间（字节）
	SavingsBytes uint64 `json:"savings_bytes"`

	// 预计节省空间（GB）
	SavingsGB float64 `json:"savings_gb"`

	// 实施难度
	Effort string `json:"effort"` // easy, medium, hard

	// 预计实施时间
	EstimatedTime string `json:"estimated_time"`

	// 详细步骤
	Steps []string `json:"steps,omitempty"`
}

// StorageForecast 存储预测.
type StorageForecast struct {
	// 预测时间范围
	ForecastDays int `json:"forecast_days"`

	// 预测数据点
	ForecastPoints []StorageForecastPoint `json:"forecast_points"`

	// 预测下月使用量（字节）
	NextMonthUsed uint64 `json:"next_month_used"`

	// 预测下季度使用量（字节）
	NextQuarterUsed uint64 `json:"next_quarter_used"`

	// 预测下年使用量（字节）
	NextYearUsed uint64 `json:"next_year_used"`

	// 预测模型
	Model string `json:"model"` // linear, exponential, arima

	// 置信度
	Confidence float64 `json:"confidence"`

	// 建议扩容量（字节）
	RecommendedExpansion uint64 `json:"recommended_expansion"`
}

// StorageForecastPoint 存储预测数据点.
type StorageForecastPoint struct {
	Date                  time.Time `json:"date"`
	PredictedUsed         uint64    `json:"predicted_used"`
	PredictedUsagePercent float64   `json:"predicted_usage_percent"`
	LowerBound            uint64    `json:"lower_bound"`
	UpperBound            uint64    `json:"upper_bound"`
}

// StorageUsageReporter 存储使用报告生成器.
type StorageUsageReporter struct {
	config StorageReportConfig
}

// StorageReportConfig 存储报告配置.
type StorageReportConfig struct {
	// 高使用率阈值（%）
	HighUsageThreshold float64 `json:"high_usage_threshold"`

	// 危险使用率阈值（%）
	CriticalUsageThreshold float64 `json:"critical_usage_threshold"`

	// 快增长阈值（%/月）
	HighGrowthThreshold float64 `json:"high_growth_threshold"`

	// 趋势历史天数
	TrendHistoryDays int `json:"trend_history_days"`

	// 预测天数
	ForecastDays int `json:"forecast_days"`

	// Top用户数量
	TopUsersCount int `json:"top_users_count"`

	// 是否启用预测
	EnableForecast bool `json:"enable_forecast"`
}

// DefaultStorageReportConfig 默认存储报告配置.
func DefaultStorageReportConfig() StorageReportConfig {
	return StorageReportConfig{
		HighUsageThreshold:     80.0,
		CriticalUsageThreshold: 90.0,
		HighGrowthThreshold:    10.0,
		TrendHistoryDays:       30,
		ForecastDays:           90,
		TopUsersCount:          10,
		EnableForecast:         true,
	}
}

// NewStorageUsageReporter 创建存储使用报告生成器.
func NewStorageUsageReporter(config StorageReportConfig) *StorageUsageReporter {
	return &StorageUsageReporter{config: config}
}

// GenerateReport 生成存储使用报告.
func (r *StorageUsageReporter) GenerateReport(
	volumes []VolumeUsageDetail,
	topUsers []UserStorageUsage,
	fileTypes []FileTypeStats,
	history []StorageTrendPoint,
) *StorageUsageReport {
	now := time.Now()
	report := &StorageUsageReport{
		ID:          "storage_" + now.Format("20060102150405"),
		Name:        "存储使用报表",
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -r.config.TrendHistoryDays),
			EndTime:   now,
		},
		Volumes:              volumes,
		TopUsers:             topUsers,
		FileTypeDistribution: fileTypes,
	}

	// 计算摘要
	report.Summary = r.calculateSummary(volumes, topUsers, history)

	// 计算趋势
	report.Trend = r.calculateTrend(history)

	// 生成告警
	report.Alerts = r.generateAlerts(volumes, topUsers, report.Trend)

	// 生成建议
	report.Recommendations = r.generateRecommendations(report)

	// 预测
	if r.config.EnableForecast && len(history) >= 7 {
		report.Forecast = r.generateForecast(history, volumes)
	}

	return report
}

// calculateSummary 计算摘要.
func (r *StorageUsageReporter) calculateSummary(
	volumes []VolumeUsageDetail,
	users []UserStorageUsage,
	history []StorageTrendPoint,
) StorageUsageSummary {
	summary := StorageUsageSummary{
		VolumeCount:     len(volumes),
		ActiveUserCount: len(users),
	}

	for _, v := range volumes {
		summary.TotalCapacity += v.TotalCapacity
		summary.TotalUsed += v.UsedCapacity
		summary.TotalAvailable += v.AvailableCapacity
		summary.TotalFiles += v.FileCount
		summary.TotalDirectories += v.DirectoryCount
	}

	if summary.TotalCapacity > 0 {
		summary.UsagePercent = round(float64(summary.TotalUsed)/float64(summary.TotalCapacity)*100, 2)
	}

	if summary.TotalFiles > 0 {
		summary.AvgFileSize = float64(summary.TotalUsed) / float64(summary.TotalFiles)
	}

	// 计算增量
	if len(history) >= 7 {
		recent := history[len(history)-1]
		weekAgo := history[len(history)-7]
		summary.WeeklyGrowth = recent.UsedCapacity - weekAgo.UsedCapacity
	}
	if len(history) >= 30 {
		recent := history[len(history)-1]
		monthAgo := history[0]
		summary.MonthlyGrowth = recent.UsedCapacity - monthAgo.UsedCapacity
		if monthAgo.UsedCapacity > 0 {
			summary.GrowthTrendPercent = round(float64(summary.MonthlyGrowth)/float64(monthAgo.UsedCapacity)*100, 2)
		}
	}

	// 健康状态
	summary.HealthStatus = "healthy"
	if summary.UsagePercent >= r.config.CriticalUsageThreshold {
		summary.HealthStatus = "critical"
	} else if summary.UsagePercent >= r.config.HighUsageThreshold {
		summary.HealthStatus = "warning"
	}

	// 效率评分
	summary.EfficiencyScore = r.calculateEfficiencyScore(summary.UsagePercent)

	return summary
}

// calculateTrend 计算趋势.
func (r *StorageUsageReporter) calculateTrend(history []StorageTrendPoint) StorageTrendData {
	trend := StorageTrendData{
		History: history,
	}

	if len(history) < 2 {
		return trend
	}

	// 计算周增长率
	if len(history) >= 7 {
		recent := history[len(history)-1]
		weekAgo := history[len(history)-7]
		if weekAgo.UsedCapacity > 0 {
			trend.WeeklyGrowthRate = round(float64(recent.UsedCapacity-weekAgo.UsedCapacity)/float64(weekAgo.UsedCapacity)*100, 2)
		}
	}

	// 计算月增长率
	if len(history) >= 30 {
		recent := history[len(history)-1]
		monthAgo := history[0]
		if monthAgo.UsedCapacity > 0 {
			trend.MonthlyGrowthRate = round(float64(recent.UsedCapacity-monthAgo.UsedCapacity)/float64(monthAgo.UsedCapacity)*100, 2)
		}
	}

	// 判断趋势方向
	if trend.MonthlyGrowthRate > 5 {
		trend.TrendDirection = "increasing"
	} else if trend.MonthlyGrowthRate < -5 {
		trend.TrendDirection = "decreasing"
	} else {
		trend.TrendDirection = "stable"
	}

	// 计算满容量天数
	if len(history) >= 7 && trend.WeeklyGrowthRate > 0 {
		recent := history[len(history)-1]
		// 假设总容量不变
		totalCapacity := recent.UsedCapacity * 100 / uint64(recent.UsagePercent)
		if totalCapacity > 0 && recent.UsagePercent < 100 {
			remainingPercent := 100 - recent.UsagePercent
			// 周增长率 -> 日增长率
			dailyGrowthRate := trend.WeeklyGrowthRate / 7
			if dailyGrowthRate > 0 {
				trend.DaysToCapacity = int(remainingPercent / dailyGrowthRate)
				fullDate := recent.Timestamp.AddDate(0, 0, trend.DaysToCapacity)
				trend.ProjectedFullDate = &fullDate
			}
		}
	}

	// 计算增长加速度
	if len(history) >= 14 {
		// 最近一周增长率 vs 前一周增长率
		recentWeek := history[len(history)-7:]
		previousWeek := history[len(history)-14 : len(history)-7]

		recentGrowth := float64(recentWeek[len(recentWeek)-1].UsedCapacity - recentWeek[0].UsedCapacity)
		previousGrowth := float64(previousWeek[len(previousWeek)-1].UsedCapacity - previousWeek[0].UsedCapacity)

		if previousGrowth > 0 {
			trend.GrowthAcceleration = round((recentGrowth-previousGrowth)/previousGrowth*100, 2)
		}
	}

	return trend
}

// generateAlerts 生成告警.
func (r *StorageUsageReporter) generateAlerts(
	volumes []VolumeUsageDetail,
	users []UserStorageUsage,
	trend StorageTrendData,
) []StorageAlert {
	alerts := make([]StorageAlert, 0)
	now := time.Now()

	// 卷容量告警
	for _, v := range volumes {
		if v.UsagePercent >= r.config.CriticalUsageThreshold {
			alerts = append(alerts, StorageAlert{
				ID:              "alert_capacity_" + v.Name,
				Type:            "capacity_high",
				Severity:        "critical",
				Volume:          v.Name,
				Message:         "卷 " + v.Name + " 容量使用率已达危险水平",
				CurrentValue:    v.UsagePercent,
				Threshold:       r.config.CriticalUsageThreshold,
				Unit:            "%",
				TriggeredAt:     now,
				SuggestedAction: "立即执行扩容或数据清理",
			})
		} else if v.UsagePercent >= r.config.HighUsageThreshold {
			alerts = append(alerts, StorageAlert{
				ID:              "alert_capacity_" + v.Name,
				Type:            "capacity_high",
				Severity:        "warning",
				Volume:          v.Name,
				Message:         "卷 " + v.Name + " 容量使用率较高",
				CurrentValue:    v.UsagePercent,
				Threshold:       r.config.HighUsageThreshold,
				Unit:            "%",
				TriggeredAt:     now,
				SuggestedAction: "规划扩容或数据清理",
			})
		}
	}

	// 用户配额告警
	for _, u := range users {
		if u.QuotaBytes > 0 {
			usagePercent := float64(u.UsedBytes) / float64(u.QuotaBytes) * 100
			if usagePercent >= 95 {
				alerts = append(alerts, StorageAlert{
					ID:              "alert_quota_" + u.Username,
					Type:            "quota_exceeded",
					Severity:        "critical",
					User:            u.Username,
					Message:         "用户 " + u.Username + " 配额即将用尽",
					CurrentValue:    usagePercent,
					Threshold:       95,
					Unit:            "%",
					TriggeredAt:     now,
					SuggestedAction: "清理数据或增加配额",
				})
			}
		}
	}

	// 增长过快告警
	if trend.MonthlyGrowthRate > r.config.HighGrowthThreshold {
		alerts = append(alerts, StorageAlert{
			ID:              "alert_growth",
			Type:            "growth_spike",
			Severity:        "warning",
			Message:         "存储增长速度过快",
			CurrentValue:    trend.MonthlyGrowthRate,
			Threshold:       r.config.HighGrowthThreshold,
			Unit:            "%/月",
			TriggeredAt:     now,
			SuggestedAction: "审查数据增长来源，规划扩容",
		})
	}

	return alerts
}

// generateRecommendations 生成建议.
func (r *StorageUsageReporter) generateRecommendations(report *StorageUsageReport) []StorageRecommendation {
	recs := make([]StorageRecommendation, 0)

	// 基于使用率
	for _, v := range report.Volumes {
		if v.UsagePercent >= r.config.CriticalUsageThreshold {
			recs = append(recs, StorageRecommendation{
				ID:            "rec_expansion_" + v.Name,
				Type:          "expansion",
				Priority:      "high",
				Title:         "紧急扩容 " + v.Name,
				Description:   "卷 " + v.Name + " 使用率已超过 " + roundStr(r.config.CriticalUsageThreshold, 0) + "%，需要立即扩容",
				Scope:         v.Name,
				SavingsGB:     float64(v.TotalCapacity) / (1024 * 1024 * 1024) * 0.3,
				Effort:        "medium",
				EstimatedTime: "1-3天",
				Steps: []string{
					"1. 评估扩容需求",
					"2. 准备存储资源",
					"3. 执行扩容操作",
					"4. 验证服务正常",
				},
			})
		} else if v.UsagePercent >= r.config.HighUsageThreshold {
			recs = append(recs, StorageRecommendation{
				ID:            "rec_plan_" + v.Name,
				Type:          "expansion",
				Priority:      "medium",
				Title:         "规划扩容 " + v.Name,
				Description:   "卷 " + v.Name + " 使用率较高，建议规划扩容",
				Scope:         v.Name,
				SavingsGB:     float64(v.TotalCapacity) / (1024 * 1024 * 1024) * 0.2,
				Effort:        "medium",
				EstimatedTime: "1-2周",
			})
		}
	}

	// 数据清理建议
	recs = append(recs, StorageRecommendation{
		ID:            "rec_cleanup",
		Type:          "cleanup",
		Priority:      "medium",
		Title:         "数据清理",
		Description:   "识别并清理过期数据、重复文件、临时文件等",
		Scope:         "全局",
		SavingsGB:     float64(report.Summary.TotalUsed) / (1024 * 1024 * 1024) * 0.1,
		Effort:        "easy",
		EstimatedTime: "1-2天",
		Steps: []string{
			"1. 扫描过期文件",
			"2. 查找重复数据",
			"3. 生成清理报告",
			"4. 执行清理操作",
		},
	})

	// 压缩优化建议
	recs = append(recs, StorageRecommendation{
		ID:            "rec_compress",
		Type:          "optimization",
		Priority:      "low",
		Title:         "启用压缩",
		Description:   "对支持的数据类型启用压缩功能，减少存储占用",
		Scope:         "全局",
		SavingsGB:     float64(report.Summary.TotalUsed) / (1024 * 1024 * 1024) * 0.25,
		Effort:        "medium",
		EstimatedTime: "1周",
	})

	// 按优先级排序
	priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.Slice(recs, func(i, j int) bool {
		return priorityOrder[recs[i].Priority] < priorityOrder[recs[j].Priority]
	})

	return recs
}

// generateForecast 生成预测.
func (r *StorageUsageReporter) generateForecast(history []StorageTrendPoint, volumes []VolumeUsageDetail) *StorageForecast {
	if len(history) < 7 {
		return nil
	}

	forecast := &StorageForecast{
		ForecastDays:   r.config.ForecastDays,
		ForecastPoints: make([]StorageForecastPoint, 0),
		Model:          "linear",
		Confidence:     0.7,
	}

	// 计算总容量
	var totalCapacity uint64
	for _, v := range volumes {
		totalCapacity += v.TotalCapacity
	}

	// 线性预测
	latest := history[len(history)-1]

	// 计算日增长率
	var dailyGrowth float64
	if len(history) >= 7 {
		weekAgo := history[len(history)-7]
		dailyGrowth = float64(latest.UsedCapacity-weekAgo.UsedCapacity) / 7.0
	}

	// 生成预测点
	for day := 1; day <= r.config.ForecastDays; day++ {
		date := latest.Timestamp.AddDate(0, 0, day)
		predicted := uint64(float64(latest.UsedCapacity) + dailyGrowth*float64(day))
		if predicted > totalCapacity {
			predicted = totalCapacity
		}

		usagePercent := round(float64(predicted)/float64(totalCapacity)*100, 2)
		margin := predicted / 10 // 10% 置信区间

		forecast.ForecastPoints = append(forecast.ForecastPoints, StorageForecastPoint{
			Date:                  date,
			PredictedUsed:         predicted,
			PredictedUsagePercent: usagePercent,
			LowerBound:            predicted - margin,
			UpperBound:            predicted + margin,
		})

		if day == 30 {
			forecast.NextMonthUsed = predicted
		}
		if day == 90 {
			forecast.NextQuarterUsed = predicted
		}
		if day == 365 {
			forecast.NextYearUsed = predicted
		}
	}

	// 建议扩容量（预测值的 120%）
	if forecast.NextQuarterUsed > 0 {
		forecast.RecommendedExpansion = uint64(float64(forecast.NextQuarterUsed) * 1.2)
	}

	return forecast
}

// calculateEfficiencyScore 计算效率评分.
func (r *StorageUsageReporter) calculateEfficiencyScore(usagePercent float64) float64 {
	// 使用率在 60-80% 之间效率最高
	if usagePercent >= 60 && usagePercent <= 80 {
		return 100
	}

	// 过低使用率
	if usagePercent < 60 {
		return round(usagePercent/60*70, 2)
	}

	// 过高使用率
	return round((100-usagePercent)/20*50+50, 2)
}

// 辅助函数 - 使用 bandwidth_report.go 中的 roundStr
