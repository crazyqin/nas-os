// Package widgets 提供Dashboard Widget类型定义
// types.go - Widget数据类型定义
package widgets

import (
	"time"
)

// SystemLoadWidgetData 系统负载Widget数据.
type SystemLoadWidgetData struct {
	Timestamp    time.Time `json:"timestamp"`
	LoadAvg1     float64   `json:"loadAvg1"`
	LoadAvg5     float64   `json:"loadAvg5"`
	LoadAvg15    float64   `json:"loadAvg15"`
	ProcessCount int       `json:"processCount"`
	RunningCount int       `json:"runningCount"`
	BlockedCount int       `json:"blockedCount"`
	CPUCount     int       `json:"cpuCount"`
	Status       string    `json:"status"` // normal, warning, critical
}

// StorageIOWidgetData 存储IO Widget数据.
type StorageIOWidgetData struct {
	Timestamp  time.Time         `json:"timestamp"`
	Devices    []StorageIODevice `json:"devices"`
	TotalRead  uint64            `json:"totalRead"`
	TotalWrite uint64            `json:"totalWrite"`
	ReadRate   float64           `json:"readRate"`  // MB/s
	WriteRate  float64           `json:"writeRate"` // MB/s
}

// StorageIODevice 存储设备IO数据.
type StorageIODevice struct {
	Device      string  `json:"device"`
	ReadBytes   uint64  `json:"readBytes"`
	WriteBytes  uint64  `json:"writeBytes"`
	ReadRate    float64 `json:"readRate"`  // MB/s
	WriteRate   float64 `json:"writeRate"` // MB/s
	IOPS        uint64  `json:"iops"`
	Utilization float64 `json:"utilization"` // % 利用率
}

// NetworkTrafficWidgetData 网络流量Widget数据.
type NetworkTrafficWidgetData struct {
	Timestamp  time.Time                 `json:"timestamp"`
	Interfaces []NetworkTrafficInterface `json:"interfaces"`
	TotalRX    uint64                    `json:"totalRX"`
	TotalTX    uint64                    `json:"totalTX"`
	RXRate     float64                   `json:"rxRate"` // MB/s
	TXRate     float64                   `json:"txRate"` // MB/s
	Trend      []TrafficTrendPoint       `json:"trend,omitempty"`
}

// NetworkTrafficInterface 网络接口流量数据.
type NetworkTrafficInterface struct {
	Name    string  `json:"name"`
	RXBytes uint64  `json:"rxBytes"`
	TXBytes uint64  `json:"txBytes"`
	RXRate  float64 `json:"rxRate"` // MB/s
	TXRate  float64 `json:"txRate"` // MB/s
	Speed   uint64  `json:"speed"`  // Mbps (链路速度)
}

// TrafficTrendPoint 流量趋势点.
type TrafficTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	RXRate    float64   `json:"rxRate"`
	TXRate    float64   `json:"txRate"`
}

// AlertSummaryWidgetData 告警汇总Widget数据.
type AlertSummaryWidgetData struct {
	Timestamp     time.Time    `json:"timestamp"`
	TotalAlerts   int          `json:"totalAlerts"`
	CriticalCount int          `json:"criticalCount"`
	WarningCount  int          `json:"warningCount"`
	InfoCount     int          `json:"infoCount"`
	ActiveAlerts  []AlertEntry `json:"activeAlerts"`
	RecentAlerts  []AlertEntry `json:"recentAlerts"` // 最近24小时
	Status        string       `json:"status"`       // ok, warning, critical
	LastAlertTime time.Time    `json:"lastAlertTime,omitempty"`
}

// AlertEntry 告警条目.
type AlertEntry struct {
	ID         string    `json:"id"`
	Level      string    `json:"level"`  // critical, warning, info
	Source     string    `json:"source"` // 来源（CPU、内存、磁盘等）
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Resolved   bool      `json:"resolved"`
	ResolvedAt time.Time `json:"resolvedAt,omitempty"`
	Value      float64   `json:"value,omitempty"`
	Threshold  float64   `json:"threshold,omitempty"`
}
