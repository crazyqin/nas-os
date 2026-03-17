// Package reports 提供报表生成和管理功能
package reports

import (
	"time"
)

// ========== 报告与配额集成 v2.31.0 ==========

// QuotaIntegration 配额集成接口
type QuotaIntegration interface {
	// GetQuotaUsage 获取配额使用情况
	GetQuotaUsage(volumeName string) ([]QuotaUsageInfo, error)

	// GetQuotaAlerts 获取配额告警
	GetQuotaAlerts() ([]QuotaAlertInfo, error)

	// GetQuotaTrends 获取配额趋势
	GetQuotaTrends(quotaID string, duration time.Duration) ([]QuotaTrendPoint, error)
}

// QuotaUsageInfo 配额使用信息
type QuotaUsageInfo struct {
	QuotaID      string    `json:"quota_id"`
	Type         string    `json:"type"`          // user, group, directory
	TargetID     string    `json:"target_id"`     // 用户名/组名/路径
	TargetName   string    `json:"target_name"`   // 显示名称
	VolumeName   string    `json:"volume_name"`   // 卷名
	HardLimit    uint64    `json:"hard_limit"`    // 硬限制（字节）
	SoftLimit    uint64    `json:"soft_limit"`    // 软限制（字节）
	UsedBytes    uint64    `json:"used_bytes"`    // 已使用（字节）
	Available    uint64    `json:"available"`     // 可用（字节）
	UsagePercent float64   `json:"usage_percent"` // 使用率（%）
	IsOverSoft   bool      `json:"is_over_soft"`  // 是否超过软限制
	IsOverHard   bool      `json:"is_over_hard"`  // 是否超过硬限制
	LastChecked  time.Time `json:"last_checked"`
}

// QuotaAlertInfo 配额告警信息
type QuotaAlertInfo struct {
	ID           string     `json:"id"`
	QuotaID      string     `json:"quota_id"`
	Type         string     `json:"type"`     // soft_limit, hard_limit
	Severity     string     `json:"severity"` // info, warning, critical, emergency
	Status       string     `json:"status"`   // active, resolved, silenced
	TargetID     string     `json:"target_id"`
	TargetName   string     `json:"target_name"`
	VolumeName   string     `json:"volume_name"`
	UsedBytes    uint64     `json:"used_bytes"`
	LimitBytes   uint64     `json:"limit_bytes"`
	UsagePercent float64    `json:"usage_percent"`
	Threshold    float64    `json:"threshold"`
	Message      string     `json:"message"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// QuotaTrendPoint 配额趋势数据点
type QuotaTrendPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    uint64    `json:"used_bytes"`
	UsagePercent float64   `json:"usage_percent"`
}

// QuotaReportIntegrator 配额报告集成器
type QuotaReportIntegrator struct {
	quotaProvider QuotaIntegration
	reporter      *ResourceReporter
	planner       *CapacityPlanner
}

// NewQuotaReportIntegrator 创建配额报告集成器
func NewQuotaReportIntegrator(
	quotaProvider QuotaIntegration,
	reporter *ResourceReporter,
	planner *CapacityPlanner,
) *QuotaReportIntegrator {
	return &QuotaReportIntegrator{
		quotaProvider: quotaProvider,
		reporter:      reporter,
		planner:       planner,
	}
}

// GenerateQuotaIntegratedReport 生成配额集成报告
func (i *QuotaReportIntegrator) GenerateQuotaIntegratedReport(volumeName string) (*QuotaIntegratedReport, error) {
	now := time.Now()

	report := &QuotaIntegratedReport{
		ID:          "quota_integrated_" + now.Format("20060102150405"),
		VolumeName:  volumeName,
		GeneratedAt: now,
		Period: ReportPeriod{
			StartTime: now.AddDate(0, 0, -7),
			EndTime:   now,
		},
	}

	// 获取配额使用情况
	if i.quotaProvider != nil {
		usages, err := i.quotaProvider.GetQuotaUsage(volumeName)
		if err == nil {
			report.QuotaUsages = usages
		}

		// 获取配额告警
		alerts, err := i.quotaProvider.GetQuotaAlerts()
		if err == nil {
			// 过滤当前卷的告警
			for _, alert := range alerts {
				if volumeName == "" || alert.VolumeName == volumeName {
					report.QuotaAlerts = append(report.QuotaAlerts, alert)
				}
			}
		}
	}

	// 生成汇总
	report.Summary = i.generateQuotaSummary(report.QuotaUsages, report.QuotaAlerts)

	// 生成预测分析
	if i.planner != nil && len(report.QuotaUsages) > 0 {
		report.Predictions = i.generateQuotaPredictions(report.QuotaUsages)
	}

	// 生成建议
	report.Recommendations = i.generateQuotaRecommendations(report)

	return report, nil
}

// generateQuotaSummary 生成配额汇总
func (i *QuotaReportIntegrator) generateQuotaSummary(usages []QuotaUsageInfo, alerts []QuotaAlertInfo) QuotaReportSummary {
	summary := QuotaReportSummary{
		TotalQuotas: len(usages),
		ByType:      make(map[string]int),
		ByVolume:    make(map[string]VolumeQuotaSummary),
	}

	for _, usage := range usages {
		// 按类型统计
		summary.ByType[usage.Type]++

		// 累计总量
		summary.TotalLimitBytes += usage.HardLimit
		summary.TotalUsedBytes += usage.UsedBytes
		summary.TotalAvailableBytes += usage.Available

		// 统计超限情况
		if usage.IsOverSoft {
			summary.OverSoftLimit++
		}
		if usage.IsOverHard {
			summary.OverHardLimit++
		}

		// 按卷统计
		volSum := summary.ByVolume[usage.VolumeName]
		volSum.VolumeName = usage.VolumeName
		volSum.TotalLimit += usage.HardLimit
		volSum.TotalUsed += usage.UsedBytes
		volSum.QuotaCount++
		summary.ByVolume[usage.VolumeName] = volSum
	}

	// 计算平均使用率
	if summary.TotalLimitBytes > 0 {
		summary.AverageUsagePercent = float64(summary.TotalUsedBytes) / float64(summary.TotalLimitBytes) * 100
	}

	// 统计告警
	summary.ActiveAlerts = len(alerts)
	for _, alert := range alerts {
		switch alert.Severity {
		case "critical", "emergency":
			summary.CriticalAlerts++
		case "warning":
			summary.WarningAlerts++
		}
	}

	// 计算健康评分
	summary.HealthScore = i.calculateHealthScore(&summary)

	return summary
}

// calculateHealthScore 计算健康评分
func (i *QuotaReportIntegrator) calculateHealthScore(summary *QuotaReportSummary) float64 {
	score := 100.0

	// 使用率扣分
	if summary.AverageUsagePercent > 90 {
		score -= 30
	} else if summary.AverageUsagePercent > 80 {
		score -= 20
	} else if summary.AverageUsagePercent > 70 {
		score -= 10
	}

	// 超软限制扣分
	score -= float64(summary.OverSoftLimit) * 2

	// 超硬限制扣分（更严重）
	score -= float64(summary.OverHardLimit) * 5

	// 告警扣分
	score -= float64(summary.CriticalAlerts) * 10
	score -= float64(summary.WarningAlerts) * 3

	// 确保分数在 0-100 之间
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return round(score, 1)
}

// generateQuotaPredictions 生成配额预测
func (i *QuotaReportIntegrator) generateQuotaPredictions(usages []QuotaUsageInfo) []QuotaPrediction {
	predictions := make([]QuotaPrediction, 0)

	for _, usage := range usages {
		// 获取趋势数据
		trends, err := i.quotaProvider.GetQuotaTrends(usage.QuotaID, 7*24*time.Hour)
		if err != nil || len(trends) < 2 {
			continue
		}

		prediction := QuotaPrediction{
			QuotaID:    usage.QuotaID,
			TargetName: usage.TargetName,
			VolumeName: usage.VolumeName,
		}

		// 计算增长率
		first := trends[0]
		last := trends[len(trends)-1]
		days := last.Timestamp.Sub(first.Timestamp).Hours() / 24

		if days > 0 {
			growthBytes := float64(last.UsedBytes - first.UsedBytes)
			prediction.DailyGrowthBytes = growthBytes / days
			prediction.WeeklyGrowthBytes = growthBytes / days * 7

			// 预测满容量时间
			if prediction.DailyGrowthBytes > 0 {
				remainingBytes := float64(usage.HardLimit - usage.UsedBytes)
				daysToFull := remainingBytes / prediction.DailyGrowthBytes
				prediction.DaysToFull = int(daysToFull)

				if daysToFull > 0 && daysToFull < 365 {
					fullDate := time.Now().AddDate(0, 0, int(daysToFull))
					prediction.ExpectedFullDate = &fullDate
				}
			}
		}

		// 确定风险级别
		if prediction.DaysToFull > 0 && prediction.DaysToFull <= 7 {
			prediction.RiskLevel = "critical"
		} else if prediction.DaysToFull > 0 && prediction.DaysToFull <= 30 {
			prediction.RiskLevel = "high"
		} else if prediction.DaysToFull > 0 && prediction.DaysToFull <= 90 {
			prediction.RiskLevel = "medium"
		} else {
			prediction.RiskLevel = "low"
		}

		predictions = append(predictions, prediction)
	}

	return predictions
}

// generateQuotaRecommendations 生成配额建议
func (i *QuotaReportIntegrator) generateQuotaRecommendations(report *QuotaIntegratedReport) []QuotaRecommendation {
	recommendations := make([]QuotaRecommendation, 0)
	now := time.Now()

	// 基于使用率建议
	for _, usage := range report.QuotaUsages {
		if usage.UsagePercent >= 95 {
			recommendations = append(recommendations, QuotaRecommendation{
				Type:        "quota_increase",
				Priority:    "critical",
				QuotaID:     usage.QuotaID,
				TargetName:  usage.TargetName,
				Title:       "紧急增加配额",
				Description: "配额使用率已超过 95%，需要立即增加配额或清理数据",
				Action:      "增加配额限制或清理无用数据",
				CreatedAt:   now,
			})
		} else if usage.UsagePercent >= 85 {
			recommendations = append(recommendations, QuotaRecommendation{
				Type:        "quota_increase",
				Priority:    "high",
				QuotaID:     usage.QuotaID,
				TargetName:  usage.TargetName,
				Title:       "计划增加配额",
				Description: "配额使用率已超过 85%，建议规划扩容",
				Action:      "评估存储需求，适当增加配额",
				CreatedAt:   now,
			})
		}
	}

	// 基于预测建议
	for _, pred := range report.Predictions {
		switch pred.RiskLevel {
		case "critical":
			recommendations = append(recommendations, QuotaRecommendation{
				Type:        "capacity_planning",
				Priority:    "critical",
				QuotaID:     pred.QuotaID,
				TargetName:  pred.TargetName,
				Title:       "紧急容量规划",
				Description: "预计 7 天内将填满配额",
				Action:      "立即扩容或迁移数据",
				CreatedAt:   now,
			})
		case "high":
			recommendations = append(recommendations, QuotaRecommendation{
				Type:        "capacity_planning",
				Priority:    "high",
				QuotaID:     pred.QuotaID,
				TargetName:  pred.TargetName,
				Title:       "容量规划提醒",
				Description: "预计 30 天内将填满配额",
				Action:      "规划扩容方案",
				CreatedAt:   now,
			})
		}
	}

	return recommendations
}

// QuotaIntegratedReport 配额集成报告
type QuotaIntegratedReport struct {
	ID              string                `json:"id"`
	VolumeName      string                `json:"volume_name"`
	GeneratedAt     time.Time             `json:"generated_at"`
	Period          ReportPeriod          `json:"period"`
	QuotaUsages     []QuotaUsageInfo      `json:"quota_usages"`
	QuotaAlerts     []QuotaAlertInfo      `json:"quota_alerts"`
	Predictions     []QuotaPrediction     `json:"predictions"`
	Summary         QuotaReportSummary    `json:"summary"`
	Recommendations []QuotaRecommendation `json:"recommendations"`
}

// QuotaReportSummary 配额报告汇总
type QuotaReportSummary struct {
	TotalQuotas         int                           `json:"total_quotas"`
	TotalLimitBytes     uint64                        `json:"total_limit_bytes"`
	TotalUsedBytes      uint64                        `json:"total_used_bytes"`
	TotalAvailableBytes uint64                        `json:"total_available_bytes"`
	AverageUsagePercent float64                       `json:"average_usage_percent"`
	OverSoftLimit       int                           `json:"over_soft_limit"`
	OverHardLimit       int                           `json:"over_hard_limit"`
	ActiveAlerts        int                           `json:"active_alerts"`
	CriticalAlerts      int                           `json:"critical_alerts"`
	WarningAlerts       int                           `json:"warning_alerts"`
	HealthScore         float64                       `json:"health_score"` // 0-100
	ByType              map[string]int                `json:"by_type"`
	ByVolume            map[string]VolumeQuotaSummary `json:"by_volume"`
}

// VolumeQuotaSummary 卷配额汇总
type VolumeQuotaSummary struct {
	VolumeName string `json:"volume_name"`
	TotalLimit uint64 `json:"total_limit"`
	TotalUsed  uint64 `json:"total_used"`
	QuotaCount int    `json:"quota_count"`
}

// QuotaPrediction 配额预测
type QuotaPrediction struct {
	QuotaID           string     `json:"quota_id"`
	TargetName        string     `json:"target_name"`
	VolumeName        string     `json:"volume_name"`
	DailyGrowthBytes  float64    `json:"daily_growth_bytes"`
	WeeklyGrowthBytes float64    `json:"weekly_growth_bytes"`
	DaysToFull        int        `json:"days_to_full"`
	ExpectedFullDate  *time.Time `json:"expected_full_date,omitempty"`
	RiskLevel         string     `json:"risk_level"` // low, medium, high, critical
}

// QuotaRecommendation 配额建议
type QuotaRecommendation struct {
	Type        string    `json:"type"`     // quota_increase, capacity_planning, cleanup
	Priority    string    `json:"priority"` // critical, high, medium, low
	QuotaID     string    `json:"quota_id"`
	TargetName  string    `json:"target_name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Action      string    `json:"action"`
	CreatedAt   time.Time `json:"created_at"`
}

// ========== 配额告警预测集成 ==========

// QuotaAlertPredictor 配额告警预测器
type QuotaAlertPredictor struct {
	integrator *QuotaReportIntegrator
	config     QuotaAlertPredictConfig
}

// QuotaAlertPredictConfig 配额告警预测配置
type QuotaAlertPredictConfig struct {
	// 预警阈值
	WarningThreshold   float64 `json:"warning_threshold"`   // 警告阈值（%）
	CriticalThreshold  float64 `json:"critical_threshold"`  // 严重阈值（%）
	EmergencyThreshold float64 `json:"emergency_threshold"` // 紧急阈值（%）

	// 预测参数
	PredictionDays       int  `json:"prediction_days"`        // 预测天数
	EnableProactiveAlert bool `json:"enable_proactive_alert"` // 是否启用主动告警
}

// DefaultQuotaAlertPredictConfig 默认配置
func DefaultQuotaAlertPredictConfig() QuotaAlertPredictConfig {
	return QuotaAlertPredictConfig{
		WarningThreshold:     70.0,
		CriticalThreshold:    85.0,
		EmergencyThreshold:   95.0,
		PredictionDays:       30,
		EnableProactiveAlert: true,
	}
}

// NewQuotaAlertPredictor 创建配额告警预测器
func NewQuotaAlertPredictor(integrator *QuotaReportIntegrator, config QuotaAlertPredictConfig) *QuotaAlertPredictor {
	return &QuotaAlertPredictor{
		integrator: integrator,
		config:     config,
	}
}

// PredictAlerts 预测即将发生的告警
func (p *QuotaAlertPredictor) PredictAlerts(volumeName string) ([]PredictedAlert, error) {
	report, err := p.integrator.GenerateQuotaIntegratedReport(volumeName)
	if err != nil {
		return nil, err
	}

	predictedAlerts := make([]PredictedAlert, 0)

	for _, pred := range report.Predictions {
		if !p.config.EnableProactiveAlert {
			continue
		}

		// 检查是否会达到各级阈值
		for _, usage := range report.QuotaUsages {
			if usage.QuotaID != pred.QuotaID {
				continue
			}

			// 预测未来使用率
			futureUsed := float64(usage.UsedBytes) + pred.DailyGrowthBytes*float64(p.config.PredictionDays)
			futurePercent := futureUsed / float64(usage.HardLimit) * 100

			if futurePercent >= p.config.EmergencyThreshold {
				predictedAlerts = append(predictedAlerts, PredictedAlert{
					QuotaID:        usage.QuotaID,
					TargetName:     usage.TargetName,
					VolumeName:     usage.VolumeName,
					CurrentUsage:   usage.UsagePercent,
					PredictedUsage: futurePercent,
					Threshold:      p.config.EmergencyThreshold,
					Severity:       "emergency",
					DaysUntilAlert: pred.DaysToFull,
					PredictedDate:  pred.ExpectedFullDate,
				})
			} else if futurePercent >= p.config.CriticalThreshold {
				predictedAlerts = append(predictedAlerts, PredictedAlert{
					QuotaID:        usage.QuotaID,
					TargetName:     usage.TargetName,
					VolumeName:     usage.VolumeName,
					CurrentUsage:   usage.UsagePercent,
					PredictedUsage: futurePercent,
					Threshold:      p.config.CriticalThreshold,
					Severity:       "critical",
					DaysUntilAlert: pred.DaysToFull,
					PredictedDate:  pred.ExpectedFullDate,
				})
			} else if futurePercent >= p.config.WarningThreshold {
				predictedAlerts = append(predictedAlerts, PredictedAlert{
					QuotaID:        usage.QuotaID,
					TargetName:     usage.TargetName,
					VolumeName:     usage.VolumeName,
					CurrentUsage:   usage.UsagePercent,
					PredictedUsage: futurePercent,
					Threshold:      p.config.WarningThreshold,
					Severity:       "warning",
					DaysUntilAlert: pred.DaysToFull,
					PredictedDate:  pred.ExpectedFullDate,
				})
			}
		}
	}

	return predictedAlerts, nil
}

// PredictedAlert 预测告警
type PredictedAlert struct {
	QuotaID        string     `json:"quota_id"`
	TargetName     string     `json:"target_name"`
	VolumeName     string     `json:"volume_name"`
	CurrentUsage   float64    `json:"current_usage"`
	PredictedUsage float64    `json:"predicted_usage"`
	Threshold      float64    `json:"threshold"`
	Severity       string     `json:"severity"`
	DaysUntilAlert int        `json:"days_until_alert"`
	PredictedDate  *time.Time `json:"predicted_date,omitempty"`
}
