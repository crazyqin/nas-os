package diskhibernate

import (
	"time"
)

// Types 类型定义

// DiskHealth 磁盘健康状态
type DiskHealth struct {
	DiskID       string    `json:"disk_id"`
	Temperature  int       `json:"temperature"`
	PowerOnHours int64     `json:"power_on_hours"`
	SpinUpCount  int64     `json:"spin_up_count"`
	HealthScore  int       `json:"health_score"` // 0-100
	PredictedLife string   `json:"predicted_life"`
	LastCheck    time.Time `json:"last_check"`
}

// PowerSaving 节能统计
type PowerSaving struct {
	TotalDisks      int     `json:"total_disks"`
	HibernatedDisks int     `json:"hibernated_disks"`
	WattsSaved      float64 `json:"watts_saved"`
	DailyKWh        float64 `json:"daily_kwh_saved"`
	MonthlyCost     float64 `json:"monthly_cost_saved"`
	CO2Reduced      float64 `json:"co2_reduced_kg"`
}

// ScheduleRule 调度规则
type ScheduleRule struct {
	ID        string    `json:"id"`
	DiskID    string    `json:"disk_id"`
	StartTime string    `json:"start_time"` // HH:MM
	EndTime   string    `json:"end_time"`   // HH:MM
	Action    string    `json:"action"`     // hibernate, wake
	Days      []int     `json:"days"`       // 0=Sunday, 6=Saturday
	Enabled   bool      `json:"enabled"`
}

// CalculatePowerSaving 计算节能效果
func CalculatePowerSaving(disks []*Disk) PowerSaving {
	saving := PowerSaving{
		TotalDisks: len(disks),
	}

	for _, disk := range disks {
		if disk.State == StateSleep || disk.State == StateStandby || disk.State == StateSpindown {
			saving.HibernatedDisks++
			saving.WattsSaved += 8.0 // 假设每块硬盘约8W
		}
	}

	// 计算每日节能
	saving.DailyKWh = saving.WattsSaved * 24 / 1000
	// 假设电价0.6元/kWh
	saving.MonthlyCost = saving.DailyKWh * 30 * 0.6
	// 假设每kWh产生0.5kg CO2
	saving.CO2Reduced = saving.DailyKWh * 30 * 0.5

	return saving
}
