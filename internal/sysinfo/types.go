// Package sysinfo 提供系统信息看板功能。
// 汇集 CPU、内存、磁盘、网络、温度等系统指标，提供一站式系统概览。
// 参考群晖资源监控和 TrueNAS Dashboard，提供更友好的中文展示。
package sysinfo

import (
	"fmt"
	"time"
)

// SystemInfo 系统信息总览.
type SystemInfo struct {
	CollectedAt time.Time  `json:"collected_at"`
	Hostname    string     `json:"hostname"`
	OS          string     `json:"os"`
	Kernel      string     `json:"kernel"`
	Arch        string     `json:"arch"`
	Uptime      int64      `json:"uptime_seconds"`
	CPU         CPUInfo    `json:"cpu"`
	Memory      MemInfo    `json:"memory"`
	Disks       []DiskInfo `json:"disks"`
	Network     []NetInfo  `json:"network"`
	LoadAvg     LoadAvg    `json:"load_avg"`
}

// CPUInfo CPU 信息.
type CPUInfo struct {
	Model       string    `json:"model"`
	Cores       int       `json:"cores"`
	Usage       float64   `json:"usage_percent"`
	TempCelsius float64   `json:"temp_celsius"`
	Frequencies []float64 `json:"frequencies_mhz"`
}

// MemInfo 内存信息.
type MemInfo struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	CachedBytes    int64   `json:"cached_bytes"`
	SwapTotalBytes int64   `json:"swap_total_bytes"`
	SwapUsedBytes  int64   `json:"swap_used_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskInfo 磁盘信息.
type DiskInfo struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mount_point"`
	FileSystem   string  `json:"file_system"`
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	FreeBytes    int64   `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	// Health: healthy / warning / critical / unknown
	Health      string  `json:"health"`
	Model       string  `json:"model"`
	Serial      string  `json:"serial"`
	TempCelsius float64 `json:"temp_celsius"`
}

// NetInfo 网络接口信息.
type NetInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Speed     int64  `json:"speed_mbps"`
	RxBytes   int64  `json:"rx_bytes"`
	TxBytes   int64  `json:"tx_bytes"`
	RxPackets int64  `json:"rx_packets"`
	TxPackets int64  `json:"tx_packets"`
	IsUp      bool   `json:"is_up"`
}

// LoadAvg 系统负载.
type LoadAvg struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// FormatBytes 将字节数格式化为人类可读的字符串。
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// CalcDiskHealth 根据使用率判断磁盘健康状态。
func CalcDiskHealth(usagePercent float64) string {
	switch {
	case usagePercent >= 95:
		return "critical"
	case usagePercent >= 85:
		return "warning"
	default:
		return "healthy"
	}
}

// CalcMemUsagePercent 计算内存使用率。
func CalcMemUsagePercent(total, used int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
