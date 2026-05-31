package nasconnect

import (
	"time"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"  // 在线
	DeviceStatusOffline DeviceStatus = "offline" // 离线
	DeviceStatusUnknown DeviceStatus = "unknown" // 未知
)

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	ConnectionStatusConnected  ConnectionStatus = "connected"  // 已连接
	ConnectionStatusConnecting ConnectionStatus = "connecting" // 连接中
	ConnectionStatusClosed     ConnectionStatus = "closed"     // 已关闭
	ConnectionStatusError      ConnectionStatus = "error"      // 错误
)

// EventType 事件类型
type EventType string

const (
	EventTypeDeviceAdded    EventType = "device_added"    // 设备添加
	EventTypeDeviceRemoved  EventType = "device_removed"  // 设备移除
	EventTypeDeviceOnline   EventType = "device_online"   // 设备上线
	EventTypeDeviceOffline  EventType = "device_offline"  // 设备离线
	EventTypeConnected      EventType = "connected"       // 已连接
	EventTypeDisconnected   EventType = "disconnected"    // 已断开
	EventTypeGroupUpdated   EventType = "group_updated"   // 分组更新
	EventTypeSyncComplete   EventType = "sync_complete"   // 同步完成
)

// NASDevice NAS 设备
type NASDevice struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Host        string       `json:"host"`
	Port        int          `json:"port"`
	Protocol    string       `json:"protocol"` // http, https, ssh
	Status      DeviceStatus `json:"status"`
	Model       string       `json:"model"`
	Version     string       `json:"version"`
	Serial      string       `json:"serial"`
	IPAddress   string       `json:"ip_address"`
	MACAddress  string       `json:"mac_address"`
	OS          string       `json:"os"`
	Hostname    string       `json:"hostname"`
	Uptime      int64        `json:"uptime"` // 秒
	CPUUsage    float64      `json:"cpu_usage"`
	MemUsage    float64      `json:"mem_usage"`
	DiskUsage   float64      `json:"disk_usage"`
	TotalDisk   int64        `json:"total_disk"`   // 字节
	UsedDisk    int64        `json:"used_disk"`    // 字节
	Tags        []string     `json:"tags"`
	Notes       string       `json:"notes"`
	LastSeen    time.Time    `json:"last_seen"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// DeviceGroup 设备分组
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DeviceIDs   []string  `json:"device_ids"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Connection 连接信息
type Connection struct {
	ID          string           `json:"id"`
	DeviceID    string           `json:"device_id"`
	Status      ConnectionStatus `json:"status"`
	Latency     time.Duration    `json:"latency"`
	LastPing    time.Time        `json:"last_ping"`
	ErrorMsg    string           `json:"error_msg,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Credential 凭证
type Credential struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // password, token, ssh_key
	Username  string    `json:"username"`
	Password  string    `json:"password,omitempty"`
	Token     string    `json:"token,omitempty"`
	SSHKey    string    `json:"ssh_key,omitempty"`
	DeviceID  string    `json:"device_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Event 事件
type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Message   string    `json:"message"`
	DeviceID  string    `json:"device_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Stats 统计信息
type Stats struct {
	TotalDevices  int `json:"total_devices"`
	OnlineDevices int `json:"online_devices"`
	OfflineDevices int `json:"offline_devices"`
	UnknownDevices int `json:"unknown_devices"`
	TotalGroups   int `json:"total_groups"`
	TotalConns    int `json:"total_connections"`
	TotalEvents   int `json:"total_events"`
}
