// Package dashboardoverview 提供仪表盘概览功能，对标飞牛fnOS系统仪表盘
package dashboardoverview

import (
	"time"
)

// SystemOverview 系统概览.
type SystemOverview struct {
	System    SystemInfo     `json:"system"`
	CPU       CPUInfo        `json:"cpu"`
	Memory    MemoryInfo     `json:"memory"`
	Storage   []StoragePool  `json:"storage"`
	Network   []NetworkInfo  `json:"network"`
	Services  []ServiceInfo  `json:"services"`
	Recent    []ActivityItem `json:"recent"`
	Alerts    []AlertItem    `json:"alerts"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// SystemInfo 系统信息.
type SystemInfo struct {
	Hostname    string        `json:"hostname"`
	OS          string        `json:"os"`
	Version     string        `json:"version"`
	Uptime      time.Duration `json:"uptime"`
	LoadAvg     [3]float64    `json:"load_avg"`
	Temperature float64       `json:"temperature"`
}

// CPUInfo CPU信息.
type CPUInfo struct {
	Model       string    `json:"model"`
	Cores       int       `json:"cores"`
	Usage       float64   `json:"usage"` // 0-100
	Temperature float64   `json:"temperature"`
	Frequency   float64   `json:"frequency_mhz"`
	PerCore     []float64 `json:"per_core"`
}

// MemoryInfo 内存信息.
type MemoryInfo struct {
	Total     int64   `json:"total_bytes"`
	Used      int64   `json:"used_bytes"`
	Available int64   `json:"available_bytes"`
	Usage     float64 `json:"usage"` // 0-100
	SwapTotal int64   `json:"swap_total_bytes"`
	SwapUsed  int64   `json:"swap_used_bytes"`
}

// StoragePool 存储池信息.
type StoragePool struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"` // healthy, degraded, faulted
	Total     int64   `json:"total_bytes"`
	Used      int64   `json:"used_bytes"`
	Available int64   `json:"available_bytes"`
	Usage     float64 `json:"usage"` // 0-100
	Disks     int     `json:"disk_count"`
	RAIDLevel string  `json:"raid_level"`
	Health    string  `json:"health"`
}

// NetworkInfo 网络信息.
type NetworkInfo struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	MAC     string `json:"mac"`
	Speed   int64  `json:"speed_mbps"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
	RxRate  int64  `json:"rx_rate_bps"`
	TxRate  int64  `json:"tx_rate_bps"`
	IsUp    bool   `json:"is_up"`
}

// ServiceInfo 服务信息.
type ServiceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"` // running, stopped, error
	Port   int    `json:"port,omitempty"`
	Uptime int64  `json:"uptime_seconds"`
	Health string `json:"health"`
	Icon   string `json:"icon,omitempty"`
}

// ActivityItem 最近活动.
type ActivityItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // login, backup, update, error
	Message   string    `json:"message"`
	User      string    `json:"user,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"` // info, warning, error
}

// AlertItem 告警信息.
type AlertItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // disk, cpu, memory, network, service
	Message   string    `json:"message"`
	Severity  string    `json:"severity"` // info, warning, critical
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
}

// WidgetData 组件数据.
type WidgetData struct {
	Type    string      `json:"type"`
	Title   string      `json:"title"`
	Data    interface{} `json:"data"`
	Refresh int         `json:"refresh_seconds"`
}
