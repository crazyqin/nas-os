// Package opscenter 提供统一运维中心功能
// 参考群晖 CMS、TrueCommand 的多NAS集中管理
package opscenter

import (
	"time"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeOnline     NodeStatus = "online"
	NodeOffline    NodeStatus = "offline"
	NodeDegraded   NodeStatus = "degraded"
	NodeMaintenance NodeStatus = "maintenance"
)

// Severity 告警级别
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
	SeverityFatal    Severity = "fatal"
)

// NASNode NAS节点信息
type NASNode struct {
	ID           string     `json:"id"`
	Hostname     string     `json:"hostname"`
	IP           string     `json:"ip"`
	Version      string     `json:"version"`
	Status       NodeStatus `json:"status"`
	Uptime       int64      `json:"uptime_seconds"`
	CPUPercent   float64    `json:"cpu_percent"`
	MemPercent   float64    `json:"mem_percent"`
	DiskTotalTB  float64    `json:"disk_total_tb"`
	DiskUsedTB   float64    `json:"disk_used_tb"`
	Temperature  float64    `json:"temperature_c"`
	LastSeen     time.Time  `json:"last_seen"`
	RegisteredAt time.Time  `json:"registered_at"`
}

// Alert 告警
type Alert struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Severity  Severity  `json:"severity"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Acked     bool      `json:"acked"`
	AckedBy   string    `json:"acked_by,omitempty"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	NodeID    string            `json:"node_id"`
	Checks    []CheckResult     `json:"checks"`
	Score     float64           `json:"score"`
	CheckedAt time.Time         `json:"checked_at"`
}

// CheckResult 单项检查结果
type CheckResult struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Value   float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Message string  `json:"message"`
}

// Dashboard 运维仪表盘
type Dashboard struct {
	TotalNodes    int            `json:"total_nodes"`
	OnlineNodes   int            `json:"online_nodes"`
	OfflineNodes  int            `json:"offline_nodes"`
	TotalAlerts   int            `json:"total_alerts"`
	CriticalAlerts int           `json:"critical_alerts"`
	AvgCPU        float64        `json:"avg_cpu"`
	AvgMemory     float64        `json:"avg_memory"`
	TotalStorageTB float64       `json:"total_storage_tb"`
	UsedStorageTB  float64       `json:"used_storage_tb"`
	Nodes         []*NASNode     `json:"nodes"`
	RecentAlerts  []*Alert       `json:"recent_alerts"`
}

// Config 运维中心配置
type Config struct {
	Enabled          bool          `json:"enabled"`
	HeartbeatSec     int           `json:"heartbeat_sec"`
	AlertRetention   time.Duration `json:"alert_retention"`
	MaxNodes         int           `json:"max_nodes"`
	AutoHeal         bool          `json:"auto_heal"`
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:        true,
		HeartbeatSec:   30,
		AlertRetention: 30 * 24 * time.Hour,
		MaxNodes:       100,
		AutoHeal:       false,
	}
}
