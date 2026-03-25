// Package reports 提供报表生成和管理功能
package reports

import (
	"math"
	"time"
)

// ========== 容量规划 ==========

// GrowthModel 增长模型类型.
type GrowthModel string

const (
	// GrowthModelLinear represents linear growth model.
	GrowthModelLinear GrowthModel = "linear" // 线性增长
	// GrowthModelExponential represents exponential growth model.
	GrowthModelExponential GrowthModel = "exponential" // 指数增长
	// GrowthModelLogarithmic represents logarithmic growth model.
	GrowthModelLogarithmic GrowthModel = "logarithmic" // 对数增长
)

// CapacityHistory 容量历史数据点.
type CapacityHistory struct {
	Timestamp      time.Time `json:"timestamp"`
	TotalBytes     uint64    `json:"total_bytes"`
	UsedBytes      uint64    `json:"used_bytes"`
	AvailableBytes uint64    `json:"available_bytes"`
	UsagePercent   float64   `json:"usage_percent"`
}

// CapacityForecast 容量预测.
type CapacityForecast struct {
	// 预测时间点
	Timestamp time.Time `json:"timestamp"`

	// 预测使用量（字节）
	ForecastUsedBytes uint64 `json:"forecast_used_bytes"`

	// 预测使用率（%）
	ForecastUsagePercent float64 `json:"forecast_usage_percent"`

	// 置信区间下限
	ConfidenceLower uint64 `json:"confidence_lower"`

	// 置信区间上限
	ConfidenceUpper uint64 `json:"confidence_upper"`

	// 预测模型
	Model GrowthModel `json:"model"`
}

// CapacityPlanningConfig 容量规划配置.
type CapacityPlanningConfig struct {
	// 预警阈值（%）
	AlertThreshold float64 `json:"alert_threshold"`

	// 紧急阈值（%）
	CriticalThreshold float64 `json:"critical_threshold"`

	// 预测天数
	ForecastDays int `json:"forecast_days"`

	// 增长模型
	GrowthModel GrowthModel `json:"growth_model"`

	// 扩容提前期（天）
	ExpansionLeadTime int `json:"expansion_lead_time"`

	// 安全缓冲（%）
	SafetyBuffer float64 `json:"safety_buffer"`
}

// CapacityPlanningReport 容量规划报告.
type CapacityPlanningReport struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	VolumeName      string                   `json:"volume_name"`
	Period          ReportPeriod             `json:"period"`
	Config          CapacityPlanningConfig   `json:"config"`
	Current         CapacityStatus           `json:"current"`
	Forecasts       []CapacityForecast       `json:"forecasts"`
	Milestones      []CapacityMilestone      `json:"milestones"`
	Recommendations []CapacityRecommendation `json:"recommendations"`
	Summary         CapacityPlanningSummary  `json:"summary"`
	GeneratedAt     time.Time                `json:"generated_at"`
}

// CapacityStatus 当前容量状态.
type CapacityStatus struct {
	TotalBytes     uint64    `json:"total_bytes"`
	UsedBytes      uint64    `json:"used_bytes"`
	AvailableBytes uint64    `json:"available_bytes"`
	UsagePercent   float64   `json:"usage_percent"`
	Status         string    `json:"status"` // healthy, warning, critical
	LastUpdated    time.Time `json:"last_updated"`
}

// CapacityMilestone 容量里程碑.
type CapacityMilestone struct {
	Name           string    `json:"name"`            // 如 "70%预警线"
	Threshold      float64   `json:"threshold"`       // 阈值百分比
	ExpectedDate   time.Time `json:"expected_date"`   // 预计到达日期
	DaysRemaining  int       `json:"days_remaining"`  // 剩余天数
	CapacityNeeded uint64    `json:"capacity_needed"` // 达到此阈值所需容量
	ActionRequired string    `json:"action_required"` // 建议行动
}

// CapacityRecommendation 容量建议.
type CapacityRecommendation struct {
	Type        string `json:"type"`     // expansion, cleanup, optimization, migration
	Priority    string `json:"priority"` // high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`     // 预期影响
	Effort      string `json:"effort"`     // 实施难度
	SavingsGB   uint64 `json:"savings_gb"` // 预计节省/增加容量（GB）
}

// CapacityPlanningSummary 容量规划汇总.
type CapacityPlanningSummary struct {
	// 当前使用率
	CurrentUsagePercent float64 `json:"current_usage_percent"`

	// 月增长率（%）
	MonthlyGrowthRate float64 `json:"monthly_growth_rate"`

	// 预计满容量日期
	FullCapacityDate *time.Time `json:"full_capacity_date,omitempty"`

	// 距满容量天数
	DaysToFullCapacity int `json:"days_to_full_capacity"`

	// 建议扩容量（GB）
	RecommendedExpansionGB uint64 `json:"recommended_expansion_gb"`

	// 紧急程度
	Urgency string `json:"urgency"` // low, medium, high, critical

	// 趋势
	Trend string `json:"trend"` // growing, stable, declining
}

// CapacityPlanner 容量规划器.
type CapacityPlanner struct {
	config CapacityPlanningConfig
}

// NewCapacityPlanner 创建容量规划器.
func NewCapacityPlanner(config CapacityPlanningConfig) *CapacityPlanner {
	// 设置默认值
	if config.AlertThreshold == 0 {
		config.AlertThreshold = 70.0
	}
	if config.CriticalThreshold == 0 {
		config.CriticalThreshold = 85.0
	}
	if config.ForecastDays == 0 {
		config.ForecastDays = 90
	}
	if config.GrowthModel == "" {
		config.GrowthModel = GrowthModelLinear
	}
	if config.ExpansionLeadTime == 0 {
		config.ExpansionLeadTime = 30
	}
	if config.SafetyBuffer == 0 {
		config.SafetyBuffer = 20.0
	}

	return &CapacityPlanner{config: config}
}

// Analyze 分析容量并生成规划报告.
func (p *CapacityPlanner) Analyze(history []CapacityHistory, volumeName string) *CapacityPlanningReport {
	if len(history) == 0 {
		return nil
	}

	// 获取当前状态
	current := p.getCurrentStatus(history)

	// 生成预测
	forecasts := p.generateForecasts(history)

	// 计算里程碑
	milestones := p.calculateMilestones(current, forecasts)

	// 生成建议
	recommendations := p.generateRecommendations(current, forecasts)

	// 计算汇总
	summary := p.calculateSummary(history, forecasts)

	now := time.Now()
	return &CapacityPlanningReport{
		ID:              "cap_" + now.Format("20060102150405"),
		Name:            "容量规划报告",
		VolumeName:      volumeName,
		Period:          ReportPeriod{StartTime: history[0].Timestamp, EndTime: now},
		Config:          p.config,
		Current:         current,
		Forecasts:       forecasts,
		Milestones:      milestones,
		Recommendations: recommendations,
		Summary:         summary,
		GeneratedAt:     now,
	}
}

// getCurrentStatus 获取当前状态.
func (p *CapacityPlanner) getCurrentStatus(history []CapacityHistory) CapacityStatus {
	latest := history[len(history)-1]

	status := "healthy"
	if latest.UsagePercent >= p.config.CriticalThreshold {
		status = "critical"
	} else if latest.UsagePercent >= p.config.AlertThreshold {
		status = "warning"
	}

	return CapacityStatus{
		TotalBytes:     latest.TotalBytes,
		UsedBytes:      latest.UsedBytes,
		AvailableBytes: latest.AvailableBytes,
		UsagePercent:   latest.UsagePercent,
		Status:         status,
		LastUpdated:    latest.Timestamp,
	}
}

// generateForecasts 生成预测.
func (p *CapacityPlanner) generateForecasts(history []CapacityHistory) []CapacityForecast {
	if len(history) < 2 {
		return nil
	}

	forecasts := make([]CapacityForecast, 0)
	latest := history[len(history)-1]

	// 计算增长参数
	growthRate := p.calculateGrowthRate(history)
	dailyGrowth := p.calculateDailyGrowth(history, growthRate)

	// 生成未来预测
	for day := 1; day <= p.config.ForecastDays; day++ {
		forecastDate := latest.Timestamp.AddDate(0, 0, day)

		// 根据模型预测
		var forecastUsed uint64
		switch p.config.GrowthModel {
		case GrowthModelExponential:
			forecastUsed = p.exponentialForecast(latest.UsedBytes, growthRate, day)
		case GrowthModelLogarithmic:
			forecastUsed = p.logarithmicForecast(history, day)
		default:
			forecastUsed = p.linearForecast(latest.UsedBytes, dailyGrowth, day)
		}

		// 确保不超过总容量
		if forecastUsed > latest.TotalBytes {
			forecastUsed = latest.TotalBytes
		}

		usagePercent := float64(forecastUsed) / float64(latest.TotalBytes) * 100

		// 计算置信区间（简化：±10%）
		confidenceMargin := forecastUsed / 10

		forecasts = append(forecasts, CapacityForecast{
			Timestamp:            forecastDate,
			ForecastUsedBytes:    forecastUsed,
			ForecastUsagePercent: round(usagePercent, 2),
			ConfidenceLower:      forecastUsed - confidenceMargin,
			ConfidenceUpper:      forecastUsed + confidenceMargin,
			Model:                p.config.GrowthModel,
		})
	}

	return forecasts
}

// calculateGrowthRate 计算增长率.
func (p *CapacityPlanner) calculateGrowthRate(history []CapacityHistory) float64 {
	if len(history) < 2 {
		return 0
	}

	// 计算复合月增长率
	first := history[0]
	last := history[len(history)-1]

	if first.UsedBytes == 0 {
		return 0
	}

	// 计算月数
	months := last.Timestamp.Sub(first.Timestamp).Hours() / (24 * 30)
	if months == 0 {
		return 0
	}

	// 复合增长率 = (终值/初值)^(1/月数) - 1
	ratio := float64(last.UsedBytes) / float64(first.UsedBytes)
	growthRate := math.Pow(ratio, 1/months) - 1

	return growthRate
}

// calculateDailyGrowth 计算日增长量.
func (p *CapacityPlanner) calculateDailyGrowth(history []CapacityHistory, growthRate float64) uint64 {
	if len(history) < 2 {
		return 0
	}

	// 使用线性回归计算平均日增长
	first := history[0]
	last := history[len(history)-1]

	days := last.Timestamp.Sub(first.Timestamp).Hours() / 24
	if days == 0 {
		return 0
	}

	return uint64(float64(last.UsedBytes-first.UsedBytes) / days)
}

// linearForecast 线性预测.
func (p *CapacityPlanner) linearForecast(currentBytes uint64, dailyGrowth uint64, days int) uint64 {
	return currentBytes + dailyGrowth*uint64(days)
}

// exponentialForecast 指数预测.
func (p *CapacityPlanner) exponentialForecast(currentBytes uint64, dailyRate float64, days int) uint64 {
	// 将月增长率转换为日增长率
	dailyRateAdjusted := dailyRate / 30.0
	factor := math.Pow(1+dailyRateAdjusted, float64(days))
	return uint64(float64(currentBytes) * factor)
}

// logarithmicForecast 对数预测.
func (p *CapacityPlanner) logarithmicForecast(history []CapacityHistory, days int) uint64 {
	if len(history) < 2 {
		return 0
	}

	latest := history[len(history)-1]
	// 对数增长：增长速度逐渐减慢
	// y = a * ln(x) + b
	// 简化：假设增长趋于平稳
	growthRate := p.calculateGrowthRate(history)
	adjustedRate := growthRate * (1.0 - math.Log(float64(days)+1)/10.0)

	return uint64(float64(latest.UsedBytes) * (1 + adjustedRate*float64(days)/30.0))
}

// calculateMilestones 计算里程碑.
func (p *CapacityPlanner) calculateMilestones(current CapacityStatus, forecasts []CapacityForecast) []CapacityMilestone {
	milestones := make([]CapacityMilestone, 0)

	// 定义里程碑阈值
	thresholds := []struct {
		name      string
		threshold float64
		action    string
	}{
		{"70%预警线", 70.0, "启动容量评估，规划扩容"},
		{"80%警戒线", 80.0, "执行扩容或数据清理"},
		{"90%危险线", 90.0, "紧急扩容或迁移数据"},
		{"95%满载线", 95.0, "立即扩容，停止非必要写入"},
	}

	now := time.Now()

	for _, t := range thresholds {
		// 如果已经超过此阈值，跳过
		if current.UsagePercent >= t.threshold {
			continue
		}

		milestone := CapacityMilestone{
			Name:           t.name,
			Threshold:      t.threshold,
			CapacityNeeded: uint64(float64(current.TotalBytes) * t.threshold / 100),
			ActionRequired: t.action,
		}

		// 在预测中查找到达此阈值的日期
		for _, f := range forecasts {
			if f.ForecastUsagePercent >= t.threshold {
				milestone.ExpectedDate = f.Timestamp
				milestone.DaysRemaining = int(f.Timestamp.Sub(now).Hours() / 24)
				break
			}
		}

		if !milestone.ExpectedDate.IsZero() {
			milestones = append(milestones, milestone)
		}
	}

	return milestones
}

// generateRecommendations 生成建议.
func (p *CapacityPlanner) generateRecommendations(current CapacityStatus, forecasts []CapacityForecast) []CapacityRecommendation {
	recommendations := make([]CapacityRecommendation, 0)

	// 基于当前状态和建议
	if current.UsagePercent >= p.config.CriticalThreshold {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:        "expansion",
			Priority:    "critical",
			Title:       "紧急扩容",
			Description: "当前使用率已超过危险阈值，需要立即扩容",
			Impact:      "避免存储空间耗尽导致服务中断",
			Effort:      "中",
			SavingsGB:   uint64(float64(current.TotalBytes) * 0.3 / (1024 * 1024 * 1024)),
		})
	} else if current.UsagePercent >= p.config.AlertThreshold {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:        "expansion",
			Priority:    "high",
			Title:       "计划扩容",
			Description: "当前使用率已超过预警阈值，建议尽快扩容",
			Impact:      "确保未来3-6个月的存储需求",
			Effort:      "中",
			SavingsGB:   uint64(float64(current.TotalBytes) * 0.2 / (1024 * 1024 * 1024)),
		})
	}

	// 基于预测的建议
	if len(forecasts) > 0 {
		// 检查是否在预测期内会达到危险阈值
		for _, f := range forecasts {
			if f.ForecastUsagePercent >= p.config.CriticalThreshold {
				daysRemaining := int(time.Until(f.Timestamp).Hours() / 24)

				if daysRemaining <= p.config.ExpansionLeadTime {
					recommendations = append(recommendations, CapacityRecommendation{
						Type:        "expansion",
						Priority:    "high",
						Title:       "提前扩容",
						Description: "预测在近期将达到危险阈值，需要提前启动扩容",
						Impact:      "避免紧急情况",
						Effort:      "中",
						SavingsGB:   uint64(float64(current.TotalBytes) * 0.25 / (1024 * 1024 * 1024)),
					})
				}
				break
			}
		}
	}

	// 通用优化建议
	recommendations = append(recommendations,
		CapacityRecommendation{
			Type:        "cleanup",
			Priority:    "medium",
			Title:       "清理冗余数据",
			Description: "识别并删除重复文件、过期备份、临时文件等",
			Impact:      "可回收5-15%的存储空间",
			Effort:      "低",
			SavingsGB:   uint64(float64(current.UsedBytes) * 0.1 / (1024 * 1024 * 1024)),
		},
		CapacityRecommendation{
			Type:        "optimization",
			Priority:    "medium",
			Title:       "启用压缩和去重",
			Description: "对支持的数据类型启用压缩和去重功能",
			Impact:      "可节省20-40%的存储空间",
			Effort:      "中",
			SavingsGB:   uint64(float64(current.UsedBytes) * 0.3 / (1024 * 1024 * 1024)),
		},
	)

	return recommendations
}

// calculateSummary 计算汇总.
func (p *CapacityPlanner) calculateSummary(history []CapacityHistory, forecasts []CapacityForecast) CapacityPlanningSummary {
	summary := CapacityPlanningSummary{}

	if len(history) == 0 {
		return summary
	}

	latest := history[len(history)-1]
	summary.CurrentUsagePercent = latest.UsagePercent

	// 计算月增长率
	summary.MonthlyGrowthRate = p.calculateGrowthRate(history) * 100

	// 判断趋势
	if summary.MonthlyGrowthRate > 5 {
		summary.Trend = "growing"
	} else if summary.MonthlyGrowthRate < -1 {
		summary.Trend = "declining"
	} else {
		summary.Trend = "stable"
	}

	// 查找满容量日期
	for _, f := range forecasts {
		if f.ForecastUsagePercent >= 95 {
			summary.FullCapacityDate = &f.Timestamp
			summary.DaysToFullCapacity = int(time.Until(f.Timestamp).Hours() / 24)
			break
		}
	}

	// 计算建议扩容量（考虑安全缓冲）
	safetyFactor := 1 + p.config.SafetyBuffer/100.0
	recommendedGB := float64(latest.UsedBytes) * safetyFactor / (1024 * 1024 * 1024)
	if latest.UsagePercent >= p.config.AlertThreshold {
		recommendedGB = recommendedGB * 1.5 // 超过预警时额外扩容
	}
	summary.RecommendedExpansionGB = uint64(recommendedGB)

	// 判断紧急程度
	if latest.UsagePercent >= p.config.CriticalThreshold {
		summary.Urgency = "critical"
	} else if latest.UsagePercent >= p.config.AlertThreshold {
		summary.Urgency = "high"
	} else if summary.DaysToFullCapacity > 0 && summary.DaysToFullCapacity <= p.config.ExpansionLeadTime {
		summary.Urgency = "medium"
	} else {
		summary.Urgency = "low"
	}

	return summary
}

// PredictCapacityNeeds 预测容量需求.
func (p *CapacityPlanner) PredictCapacityNeeds(history []CapacityHistory, targetMonths int) (uint64, error) {
	if len(history) < 2 {
		return 0, nil
	}

	latest := history[len(history)-1]
	growthRate := p.calculateGrowthRate(history)

	// 预测目标月份的使用量
	var predictedUsed float64
	switch p.config.GrowthModel {
	case GrowthModelExponential:
		factor := math.Pow(1+growthRate, float64(targetMonths))
		predictedUsed = float64(latest.UsedBytes) * factor
	default:
		// 线性增长
		dailyGrowth := p.calculateDailyGrowth(history, growthRate)
		predictedUsed = float64(latest.UsedBytes) + float64(dailyGrowth)*float64(targetMonths*30)
	}

	// 加上安全缓冲
	predictedWithBuffer := predictedUsed * (1 + p.config.SafetyBuffer/100.0)

	return uint64(predictedWithBuffer), nil
}

// UpdateConfig 更新配置.
func (p *CapacityPlanner) UpdateConfig(config CapacityPlanningConfig) {
	p.config = config
}

// GetConfig 获取配置.
func (p *CapacityPlanner) GetConfig() CapacityPlanningConfig {
	return p.config
}
