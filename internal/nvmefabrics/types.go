// Package nvmefabrics 提供 NVMe over Fabrics 功能
// 支持 RoCEv2 和 TCP 传输
package nvmefabrics

import (
	"net"
	"time"
)

// TransportType 传输类型
type TransportType string

const (
	TransportTCP   TransportType = "tcp"
	TransportRoCEv2 TransportType = "rocev2"
)

// TargetState 目标状态
type TargetState string

const (
	TargetStateActive   TargetState = "active"
	TargetStateInactive TargetState = "inactive"
	TargetStateError    TargetState = "error"
)

// SubsystemState 子系统状态
type SubsystemState string

const (
	SubsystemStateOnline  SubsystemState = "online"
	SubsystemStateOffline SubsystemState = "offline"
	SubsystemStateError   SubsystemState = "error"
)

// NVMfTarget NVMe over Fabrics 目标
type NVMfTarget struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Transport     TransportType     `json:"transport"`
	IP            net.IP            `json:"ip"`
	Port          int               `json:"port"`
	State         TargetState       `json:"state"`
	MaxNamespaces int               `json:"max_namespaces"`
	Subsystems    []NVMfSubsystem   `json:"subsystems"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// NVMfSubsystem NVMe 子系统
type NVMfSubsystem struct {
	NQN          string          `json:"nqn"`          // NVMe Qualified Name
	TargetID     string          `json:"target_id"`
	State        SubsystemState  `json:"state"`
	AllowAnyHost bool            `json:"allow_any_host"`
	Hosts        []string        `json:"hosts"`         // Allowed host NQNs
	Namespaces   []NVMfNamespace `json:"namespaces"`
	Controllers  []NVMfController `json:"controllers"`
	MaxNamespaces int            `json:"max_namespaces"`
	CreatedAt    time.Time       `json:"created_at"`
}

// NVMfNamespace NVMe 命名空间
type NVMfNamespace struct {
	ID         int    `json:"id"`
	SubsystemNQN string `json:"subsystem_nqn"`
	DevicePath string `json:"device_path"`
	SizeBytes  int64  `json:"size_bytes"`
	BlockSize  int    `json:"block_size"`
	UUID       string `json:"uuid"`
	NGUID      string `json:"nguid,omitempty"`
}

// NVMfController NVMe 控制器
type NVMfController struct {
	ID            string    `json:"id"`
	SubsystemNQN  string    `json:"subsystem_nqn"`
	HostNQN       string    `json:"host_nqn"`
	HostAddress   string    `json:"host_address"`
	Transport     TransportType `json:"transport"`
	State         string    `json:"state"`
	ConnectedAt   time.Time `json:"connected_at"`
	IOQueues      int       `json:"io_queues"`
	QueueDepth    int       `json:"queue_depth"`
}

// NVMfTransport NVMe 传输配置
type NVMfTransport struct {
	Type      TransportType `json:"type"`
	IP        net.IP        `json:"ip"`
	Port      int           `json:"port"`
	Adrfam    string        `json:"adrfam"`    // ipv4, ipv6
	TRSVCID   string        `json:"trsvcid"`
	Interface string        `json:"interface"` // Network interface
}

// ControllerStats 控制器统计
type ControllerStats struct {
	ControllerID  string  `json:"controller_id"`
	IOPS          int64   `json:"iops"`
	ReadIOPS      int64   `json:"read_iops"`
	WriteIOPS     int64   `json:"write_iops"`
	Bandwidth     int64   `json:"bandwidth_mbps"`
	ReadBandwidth int64   `json:"read_bandwidth_mbps"`
	WriteBandwidth int64  `json:"write_bandwidth_mbps"`
	AvgLatencyUs  float64 `json:"avg_latency_us"`
	ReadLatencyUs float64 `json:"read_latency_us"`
	WriteLatencyUs float64 `json:"write_latency_us"`
	Commands      int64   `json:"commands"`
}

// TargetStats 目标统计
type TargetStats struct {
	TotalTargets     int `json:"total_targets"`
	ActiveTargets    int `json:"active_targets"`
	TCPCount         int `json:"tcp_count"`
	RoCEv2Count      int `json:"rocev2_count"`
	TotalSubsystems  int `json:"total_subsystems"`
	TotalNamespaces  int `json:"total_namespaces"`
	TotalControllers int `json:"total_controllers"`
}

// CreateTargetRequest 创建目标请求
type CreateTargetRequest struct {
	Name          string        `json:"name" binding:"required"`
	Transport     TransportType `json:"transport" binding:"required"`
	IP            string        `json:"ip" binding:"required"`
	Port          int           `json:"port" binding:"required"`
	MaxNamespaces int           `json:"max_namespaces"`
}

// CreateSubsystemRequest 创建子系统请求
type CreateSubsystemRequest struct {
	NQN           string `json:"nqn" binding:"required"`
	AllowAnyHost  bool   `json:"allow_any_host"`
	MaxNamespaces int    `json:"max_namespaces"`
}

// AddNamespaceRequest 添加命名空间请求
type AddNamespaceRequest struct {
	DevicePath string `json:"device_path" binding:"required"`
	SizeBytes  int64  `json:"size_bytes" binding:"required"`
	BlockSize  int    `json:"block_size"`
}

// ConnectHostRequest 连接主机请求
type ConnectHostRequest struct {
	HostNQN     string `json:"host_nqn" binding:"required"`
	HostAddress string `json:"host_address" binding:"required"`
}
