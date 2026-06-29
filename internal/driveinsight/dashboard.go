package driveinsight

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Dashboard 仪表盘数据生成器。
// 聚合所有存储视图：分层计划、容量监控、系统健康，提供统一管理面板。
// 参考群晖 DSM 7.3 One View for All Storage 设计理念。
type Dashboard struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	collector  *Collector
	engine     *TieringEngine
	alertThresholds AlertThresholds
}

// AlertThresholds 容量预警阈值配置。
type AlertThresholds struct {
	WarningPercent  float64 // 警告阈值（使用率百分比）
	CriticalPercent float64 // 严重阈值
	MaxTempC        float64 // 最高温度阈值
	MaxPowerOnHours int64   // 最大通电时间阈值
}

// DefaultAlertThresholds 默认预警阈值。
func DefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{
		WarningPercent:  80.0,
		CriticalPercent: 90.0,
		MaxTempC:        65.0,
		MaxPowerOnHours: 43800, // 5年
	}
}

// NewDashboard 创建仪表盘生成器。
func NewDashboard(collector *Collector, engine *TieringEngine, logger *zap.Logger) *Dashboard {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dashboard{
		logger:          logger,
		collector:       collector,
		engine:          engine,
		alertThresholds: DefaultAlertThresholds(),
	}
}

// SetThresholds 设置预警阈值。
func (d *Dashboard) SetThresholds(thresholds AlertThresholds) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.alertThresholds = thresholds
}

// Generate 生成完整的仪表盘数据。
// 聚合磁盘统计、存储层、成本报告、健康摘要、容量预警和分层计划。
func (d *Dashboard) Generate() *DashboardData {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()

	// 获取所有磁盘
	drives := d.collector.GetAllDrives()

	// 获取所有存储层
	tiers := d.collector.GetTiers()

	// 计算总量
	var totalCapacity, totalUsed, totalFree int64
	for _, drive := range drives {
		totalCapacity += drive.CapacityBytes
		totalUsed += drive.UsedBytes
		totalFree += drive.FreeBytes
	}
	// 存储层数据不重复计入磁盘总量（由注册者保证不重叠）

	usagePercent := 0.0
	if totalCapacity > 0 {
		usagePercent = float64(totalUsed) / float64(totalCapacity) * 100
	}

	// 生成成本报告
	costReport := d.collector.CalculateCostReport()

	// 生成健康摘要
	healthSummary := d.generateHealthSummary(drives)

	// 生成容量预警
	capacityAlerts := d.generateCapacityAlerts(drives, tiers)

	// 获取分层计划
	var plans []TieringPlan
	if d.engine != nil {
		plans = d.engine.ListPlans()
	}

	// 计算待迁移数量
	migrationPending := 0
	if d.engine != nil {
		// 在实际实现中会查询迁移队列
		// 这里简单返回0，因为迁移是基于评估结果实时生成的
	}

	data := &DashboardData{
		GeneratedAt:      now,
		TotalCapacity:    totalCapacity,
		TotalUsed:        totalUsed,
		TotalFree:        totalFree,
		UsagePercent:     usagePercent,
		Tiers:            tiers,
		Drives:           drives,
		CostReport:       costReport,
		CapacityAlerts:   capacityAlerts,
		HealthSummary:    healthSummary,
		TieringPlans:     plans,
		MigrationPending: migrationPending,
	}

	d.logger.Info("生成仪表盘数据",
		zap.Int("drives", len(drives)),
		zap.Int("tiers", len(tiers)),
		zap.Float64("usage_percent", usagePercent),
		zap.Int("alerts", len(capacityAlerts)),
	)

	return data
}

// generateHealthSummary 生成健康摘要。
func (d *Dashboard) generateHealthSummary(drives []DriveStats) HealthSummary {
	summary := HealthSummary{
		TotalDrives: len(drives),
		Details:     make([]DriveHealth, 0, len(drives)),
	}

	var tempSum float64
	var maxTemp float64
	tempCount := 0

	for _, drive := range drives {
		dh := DriveHealth{
			SerialNumber: drive.SerialNumber,
			Model:        drive.Model,
			Status:       drive.HealthStatus,
			TemperatureC: drive.TemperatureC,
			PowerOnHours: drive.PowerOnHours,
			Message:      d.generateDriveMessage(drive),
		}

		switch drive.HealthStatus {
		case HealthGood:
			summary.HealthyDrives++
		case HealthWarning:
			summary.WarningDrives++
		case HealthCritical:
			summary.CriticalDrives++
		}

		if drive.TemperatureC > 0 {
			tempSum += drive.TemperatureC
			tempCount++
			if drive.TemperatureC > maxTemp {
				maxTemp = drive.TemperatureC
			}
		}

		// 检查通电时间
		if drive.PowerOnHours > d.alertThresholds.MaxPowerOnHours {
			if dh.Status == HealthGood {
				dh.Status = HealthWarning
				dh.Message = fmt.Sprintf("磁盘通电时间过长 (%d小时)，建议更换", drive.PowerOnHours)
				summary.HealthyDrives--
				summary.WarningDrives++
			}
		}

		summary.Details = append(summary.Details, dh)
	}

	if tempCount > 0 {
		summary.AvgTempC = tempSum / float64(tempCount)
	}
	summary.MaxTempC = maxTemp

	// 总体健康状态
	switch {
	case summary.CriticalDrives > 0:
		summary.Overall = HealthCritical
	case summary.WarningDrives > 0:
		summary.Overall = HealthWarning
	case summary.TotalDrives > 0:
		summary.Overall = HealthGood
	default:
		summary.Overall = HealthUnknown
	}

	return summary
}

// generateDriveMessage 生成单盘健康消息。
func (d *Dashboard) generateDriveMessage(drive DriveStats) string {
	switch drive.HealthStatus {
	case HealthGood:
		return "磁盘运行正常"
	case HealthWarning:
		if drive.TemperatureC >= d.alertThresholds.MaxTempC {
			return fmt.Sprintf("温度过高: %.1f°C，建议检查散热", drive.TemperatureC)
		}
		return "磁盘存在警告，建议关注"
	case HealthCritical:
		return fmt.Sprintf("磁盘状态严重，建议立即更换 (温度: %.1f°C)", drive.TemperatureC)
	default:
		return "磁盘状态未知"
	}
}

// generateCapacityAlerts 生成容量预警。
func (d *Dashboard) generateCapacityAlerts(drives []DriveStats, tiers []StorageTier) []CapacityAlert {
	alerts := make([]CapacityAlert, 0)

	// 检查磁盘级别预警
	for _, drive := range drives {
		if drive.CapacityBytes == 0 {
			continue
		}
		usage := float64(drive.UsedBytes) / float64(drive.CapacityBytes) * 100

		if usage >= d.alertThresholds.CriticalPercent {
			alerts = append(alerts, CapacityAlert{
				TierName:     string(drive.Type),
				DriveSerial:  drive.SerialNumber,
				UsagePercent: usage,
				Threshold:    d.alertThresholds.CriticalPercent,
				Level:        AlertCritical,
				Message:      fmt.Sprintf("磁盘 %s (%s) 使用率 %.1f%%，已超过严重阈值 %.0f%%", drive.SerialNumber, drive.Model, usage, d.alertThresholds.CriticalPercent),
			})
		} else if usage >= d.alertThresholds.WarningPercent {
			alerts = append(alerts, CapacityAlert{
				TierName:     string(drive.Type),
				DriveSerial:  drive.SerialNumber,
				UsagePercent: usage,
				Threshold:    d.alertThresholds.WarningPercent,
				Level:        AlertWarning,
				Message:      fmt.Sprintf("磁盘 %s (%s) 使用率 %.1f%%，已超过警告阈值 %.0f%%", drive.SerialNumber, drive.Model, usage, d.alertThresholds.WarningPercent),
			})
		}
	}

	// 检查存储层级别预警
	for _, tier := range tiers {
		if tier.CapacityBytes == 0 {
			continue
		}
		usage := float64(tier.UsedBytes) / float64(tier.CapacityBytes) * 100

		if usage >= d.alertThresholds.CriticalPercent {
			alerts = append(alerts, CapacityAlert{
				TierName:     tier.Name,
				DriveSerial:  "",
				UsagePercent: usage,
				Threshold:    d.alertThresholds.CriticalPercent,
				Level:        AlertCritical,
				Message:      fmt.Sprintf("存储层 %s 使用率 %.1f%%，已超过严重阈值", tier.Name, usage),
			})
		} else if usage >= d.alertThresholds.WarningPercent {
			alerts = append(alerts, CapacityAlert{
				TierName:     tier.Name,
				DriveSerial:  "",
				UsagePercent: usage,
				Threshold:    d.alertThresholds.WarningPercent,
				Level:        AlertWarning,
				Message:      fmt.Sprintf("存储层 %s 使用率 %.1f%%，已超过警告阈值", tier.Name, usage),
			})
		}
	}

	return alerts
}

// GetCapacityForecast 获取容量预测。
// 基于当前使用量和历史增长率预测何时达到容量上限。
func (d *Dashboard) GetCapacityForecast(currentUsed int64, growthRatePerDay float64) *CapacityForecast {
	if growthRatePerDay <= 0 {
		return &CapacityForecast{
			CurrentUsage:  float64(currentUsed),
			GrowthRateDay: 0,
			Status:        "稳定",
			Message:       "无增长趋势",
		}
	}

	totalCap := int64(0)
	for _, drive := range d.collector.GetAllDrives() {
		totalCap += drive.CapacityBytes
	}

	if totalCap == 0 {
		return &CapacityForecast{
			CurrentUsage:  float64(currentUsed),
			GrowthRateDay: growthRatePerDay,
			Status:        "未知",
			Message:       "无法获取总容量",
		}
	}

	remaining := float64(totalCap - currentUsed)
	daysRemaining := int(remaining / growthRatePerDay)

	status := "正常"
	message := ""
	switch {
	case daysRemaining < 7:
		status = "紧急"
		message = fmt.Sprintf("预计 %d 天后达到容量上限，需立即扩容", daysRemaining)
	case daysRemaining < 30:
		status = "警告"
		message = fmt.Sprintf("预计 %d 天后达到容量上限，建议规划扩容", daysRemaining)
	case daysRemaining < 90:
		status = "注意"
		message = fmt.Sprintf("预计 %d 天后达到容量上限", daysRemaining)
	default:
		message = fmt.Sprintf("预计 %d 天后达到容量上限", daysRemaining)
	}

	return &CapacityForecast{
		CurrentUsage:  float64(currentUsed),
		TotalCapacity: float64(totalCap),
		GrowthRateDay: growthRatePerDay,
		DaysRemaining: daysRemaining,
		EstimatedDate: time.Now().AddDate(0, 0, daysRemaining),
		Status:        status,
		Message:       message,
	}
}

// CapacityForecast 容量预测。
type CapacityForecast struct {
	CurrentUsage  float64   `json:"current_usage"`   // 当前使用量（字节）
	TotalCapacity  float64   `json:"total_capacity"`  // 总容量（字节）
	GrowthRateDay float64   `json:"growth_rate_day"` // 日增长率（字节/天）
	DaysRemaining int       `json:"days_remaining"`  // 剩余天数
	EstimatedDate time.Time `json:"estimated_date"`  // 预计满盘日期
	Status        string    `json:"status"`          // 状态
	Message       string    `json:"message"`         // 消息
}
