package nasdiscovery

import (
	"errors"
	"time"
)

// 错误定义
var (
	ErrDeviceNotFound = errors.New("设备未找到")
	ErrAlreadyRunning = errors.New("发现服务已在运行")
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusUnknown DeviceStatus = "unknown"
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// NASDevice 发现的 NAS 设备
type NASDevice struct {
	ID         string       `json:"id"`
	IP         string       `json:"ip"`
	Hostname   string       `json:"hostname"`
	MAC        string       `json:"mac,omitempty"`
	OpenPorts  []int        `json:"open_ports"`
	IsNAS      bool         `json:"is_nas"`
	SSLEnabled bool         `json:"ssl_enabled"`
	Trusted    bool         `json:"trusted"`
	ManualAdd  bool         `json:"manual_add"`
	Status     DeviceStatus `json:"status"`
	FirstSeen  time.Time    `json:"first_seen"`
	LastSeen   time.Time    `json:"last_seen"`
	Vendor     string       `json:"vendor,omitempty"`
	Model      string       `json:"model,omitempty"`
	Version    string       `json:"version,omitempty"`
}

// DiscoveryConfig 设备发现配置
type DiscoveryConfig struct {
	Enabled         bool     `json:"enabled"`
	ScanInterval    int      `json:"scan_interval"` // 秒
	UDPPort         int      `json:"udp_port"`
	MDNSEnabled     bool     `json:"mdns_enabled"`
	SSDPEnabled     bool     `json:"ssdp_enabled"`
	BroadcastAddr   string   `json:"broadcast_addr"`
	AutoAddDevices  bool     `json:"auto_add_devices"`
	TrustedNetworks []string `json:"trusted_networks"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID            string     `json:"id"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	Duration      string     `json:"duration"`
	Status        ScanStatus `json:"status"`
	DevicesFound  int        `json:"devices_found"`
	TotalDevices  int        `json:"total_devices"`
	Error         string     `json:"error,omitempty"`
}

// DiscoveryMessage UDP 发现消息
type DiscoveryMessage struct {
	Type      string `json:"type"`      // "discovery" 或 "response"
	Hostname  string `json:"hostname"`
	Timestamp int64  `json:"timestamp"`
	Version   string `json:"version"`
	MAC       string `json:"mac,omitempty"`
}
