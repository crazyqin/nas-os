// Package diskwearlevel 提供磁盘磨损均衡管理功能
// 支持 SMART 监控、磨损均衡策略、磁盘轮换建议、寿命预测
package diskwearlevel

import (
	"time"
)

// DiskType 磁盘类型
type DiskType string

const (
	DiskTypeHDD  DiskType = "hdd"
	DiskTypeSSD  DiskType = "ssd"
	DiskTypeNVMe DiskType = "nvme"
)

// DiskHealth 磁盘健康状态
type DiskHealth string

const (
	HealthExcellent DiskHealth = "excellent"
	HealthGood      DiskHealth = "good"
	HealthFair      DiskHealth = "fair"
	HealthPoor      DiskHealth = "poor"
	HealthCritical  DiskHealth = "critical"
	HealthFailed    DiskHealth = "failed"
)

// WearLevel 磨损等级
type WearLevel string

const (
	WearLevelLow    WearLevel = "low"     // 0-30%
	WearLevelMedium WearLevel = "medium"  // 30-60%
	WearLevelHigh   WearLevel = "high"    // 60-80%
	WearLevelSevere WearLevel = "severe"  // 80-100%
)

// DiskInfo 磁盘信息
type DiskInfo struct {
	ID            string    `json:"id"`
	Device        string    `json:"device"`       // /dev/sda
	Model         string    `json:"model"`
	Serial        string    `json:"serial"`
	Type          DiskType  `json:"type"`
	CapacityBytes int64     `json:"capacityBytes"`
	Health        DiskHealth `json:"health"`
	WearPercent   float64   `json:"wearPercent"`  // 磨损百分比 0-100
	WearLevel     WearLevel `json:"wearLevel"`
	Temperature   int       `json:"temperature"`   // 摄氏度
	PowerOnHours  int64     `json:"powerOnHours"`
	TotalWritesTB float64   `json:"totalWritesTb"` // 总写入TB
	PredictedLife *int      `json:"predictedLifeDays,omitempty"`
	LastSMART     time.Time `json:"lastSmartCheck"`
	CreatedAt     time.Time `json:"createdAt"`
}

// SMARTData SMART 原始数据
type SMARTData struct {
	DiskID        string         `json:"diskId"`
	Temperature   int            `json:"temperature"`
	ReallocatedSectors int64     `json:"reallocatedSectors"`
	PendingSectors      int64    `json:"pendingSectors"`
	OfflineUncorrectable int64   `json:"offlineUncorrectable"`
	SeekErrorRate       int64    `json:"seekErrorRate"`
 SpinRetryCount      int64    `json:"spinRetryCount"`
	ReportedUncorrect   int64    `json:"reportedUncorrect"`
	HighFlyWrites       int64    `json:"highFlyWrites"`
	WearLevelingCount   int64    `json:"wearLevelingCount"` // SSD
	PercentageUsed      int64    `json:"percentageUsed"`     // SSD/NVMe
	AvailableSpare      int64    `json:"availableSpare"`     // NVMe
	CriticalWarning     int      `json:"criticalWarning"`    // NVMe
	CheckedAt           time.Time `json:"checkedAt"`
}

// WearPolicy 磨损均衡策略
type WearPolicy struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Enabled         bool      `json:"enabled"`
	// 策略参数
	MaxWearPercent  float64   `json:"maxWearPercent"`   // 告警阈值
	RebalanceThreshold float64 `json:"rebalanceThreshold"` // 均衡触发阈值
	TempWarningC    int       `json:"tempWarningC"`     // 温度告警
	TempCriticalC   int       `json:"tempCriticalC"`    // 温度临界
	// 轮换策略
	EnableRotation  bool      `json:"enableRotation"`
	RotationIntervalDays int  `json:"rotationIntervalDays"`
	// 通知
	NotifyOnWarning bool      `json:"notifyOnWarning"`
	NotifyOnCritical bool     `json:"notifyOnCritical"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// RebalancePlan 均衡计划
type RebalancePlan struct {
	ID          string              `json:"id"`
	CreatedAt   time.Time           `json:"createdAt"`
	SourceDisk  string              `json:"sourceDisk"`
	TargetDisk  string              `json:"targetDisk"`
	EstBytes    int64               `json:"estimatedBytes"`
	EstDuration time.Duration       `json:"estimatedDuration"`
	Reason      string              `json:"reason"`
	Actions     []*RebalanceAction  `json:"actions"`
	Status      string              `json:"status"` // pending, running, completed, failed
}

// RebalanceAction 均衡动作
type RebalanceAction struct {
	Type       string `json:"type"` // move_data, swap_role, mark_cold
	SourcePath string `json:"sourcePath,omitempty"`
	TargetPath string `json:"targetPath,omitempty"`
	Priority   int    `json:"priority"`
}

// WearStats 磨损统计
type WearStats struct {
	TotalDisks       int            `json:"totalDisks"`
	HealthyDisks     int            `json:"healthyDisks"`
	WarningDisks     int            `json:"warningDisks"`
	CriticalDisks    int            `json:"criticalDisks"`
	AvgWearPercent   float64        `json:"avgWearPercent"`
	MaxWearPercent   float64        `json:"maxWearPercent"`
	MinWearPercent   float64        `json:"minWearPercent"`
	AvgTemperature   float64        `json:"avgTemperature"`
	TotalRebalances  int            `json:"totalRebalances"`
	HealthBreakdown  map[string]int `json:"healthBreakdown"`
}
