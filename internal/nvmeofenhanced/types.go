// Package nvmeofenhanced NVMe-oF增强模块
// 支持400GbE网络、RDMA、NVMe/TCP等高性能存储互联
// 对标TrueNAS 25.10 NVMe over Fabric支持
package nvmeofenhanced

import (
	"errors"
	"sync"
	"time"
)

// TransportType 传输类型.
type TransportType string

const (
	TransportTCP  TransportType = "tcp"  // NVMe/TCP
	TransportRDMA TransportType = "rdma" // NVMe/RDMA
	TransportFC   TransportType = "fc"   // Fibre Channel
	TransportIB   TransportType = "ib"   // InfiniBand
)

// NetworkSpeed 网络速度.
type NetworkSpeed string

const (
	Speed10G  NetworkSpeed = "10g"  // 10GbE
	Speed25G  NetworkSpeed = "25g"  // 25GbE
	Speed40G  NetworkSpeed = "40g"  // 40GbE
	Speed100G NetworkSpeed = "100g" // 100GbE
	Speed200G NetworkSpeed = "200g" // 200GbE
	Speed400G NetworkSpeed = "400g" // 400GbE
)

// NVMeSubsystem NVMe子系统.
type NVMeSubsystem struct {
	ID             string        `json:"id"`
	NQN            string        `json:"nqn"` // NVMe Qualified Name
	Alias          string        `json:"alias"`
	Description    string        `json:"description"`
	Transport      TransportType `json:"transport"`
	IPAddress      string        `json:"ip_address"`
	Port           int           `json:"port"`
	MaxNamespaces  int           `json:"max_namespaces"`
	MaxControllers int           `json:"max_controllers"`
	MaxQueueDepth  int           `json:"max_queue_depth"`
	MaxQueuePairs  int           `json:"max_queue_pairs"`
	IsOnline       bool          `json:"is_online"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// NVMeNamespace NVMe命名空间.
type NVMeNamespace struct {
	ID          string    `json:"id"`
	SubsystemID string    `json:"subsystem_id"`
	NSID        int       `json:"nsid"` // Namespace ID
	UUID        string    `json:"uuid"`
	SizeBytes   int64     `json:"size_bytes"`
	BlockSize   int       `json:"block_size"`
	IsShared    bool      `json:"is_shared"`
	IsOnline    bool      `json:"is_online"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NVMeController NVMe控制器.
type NVMeController struct {
	ID            string        `json:"id"`
	SubsystemID   string        `json:"subsystem_id"`
	CNTLID        int           `json:"cntlid"` // Controller ID
	Model         string        `json:"model"`
	Serial        string        `json:"serial"`
	Firmware      string        `json:"firmware"`
	Transport     TransportType `json:"transport"`
	IPAddress     string        `json:"ip_address"`
	Port          int           `json:"port"`
	MaxQueueDepth int           `json:"max_queue_depth"`
	IsOnline      bool          `json:"is_online"`
	ConnectedAt   time.Time     `json:"connected_at"`
}

// NetworkInterface 网络接口.
type NetworkInterface struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Speed     NetworkSpeed `json:"speed"`
	IPAddress string       `json:"ip_address"`
	Subnet    string       `json:"subnet"`
	Gateway   string       `json:"gateway"`
	MTU       int          `json:"mtu"`
	IsRDMA    bool         `json:"is_rdma"`
	IsOnline  bool         `json:"is_online"`
	BytesSent int64        `json:"bytes_sent"`
	BytesRecv int64        `json:"bytes_recv"`
	Errors    int64        `json:"errors"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// RDMAConfig RDMA配置.
type RDMAConfig struct {
	Enabled        bool   `json:"enabled"`
	Device         string `json:"device"`           // RDMA设备名称
	QueuePairCount int    `json:"queue_pair_count"` // Queue Pair数量
	MaxInlineData  int    `json:"max_inline_data"`  // 最大内联数据大小
	MaxSendWR      int    `json:"max_send_wr"`      // 最大发送工作请求
	MaxRecvWR      int    `json:"max_recv_wr"`      // 最大接收工作请求
	MaxSGE         int    `json:"max_sge"`          // 最大分散聚集元素
	UseGRH         bool   `json:"use_grh"`          // 使用全局路由头
}

// DefaultRDMAConfig 默认RDMA配置.
func DefaultRDMAConfig() RDMAConfig {
	return RDMAConfig{
		Enabled:        true,
		QueuePairCount: 256,
		MaxInlineData:  256,
		MaxSendWR:      1024,
		MaxRecvWR:      1024,
		MaxSGE:         16,
		UseGRH:         true,
	}
}

// PerformanceMetrics 性能指标.
type PerformanceMetrics struct {
	SubsystemID  string `json:"subsystem_id"`
	NamespaceID  string `json:"namespace_id"`
	ControllerID string `json:"controller_id"`

	// IOPS指标
	ReadIOPS  int64 `json:"read_iops"`
	WriteIOPS int64 `json:"write_iops"`
	TotalIOPS int64 `json:"total_iops"`

	// 吞吐量指标
	ReadThroughputMBps  float64 `json:"read_throughput_mbps"`
	WriteThroughputMBps float64 `json:"write_throughput_mbps"`
	TotalThroughputMBps float64 `json:"total_throughput_mbps"`

	// 延迟指标
	ReadLatencyUs  float64 `json:"read_latency_us"`
	WriteLatencyUs float64 `json:"write_latency_us"`
	AvgLatencyUs   float64 `json:"avg_latency_us"`
	P99LatencyUs   float64 `json:"p99_latency_us"`

	// 队列指标
	QueueDepth       int     `json:"queue_depth"`
	QueueUtilization float64 `json:"queue_utilization"`

	// 网络指标
	BytesSent   int64 `json:"bytes_sent"`
	BytesRecv   int64 `json:"bytes_recv"`
	PacketsSent int64 `json:"packets_sent"`
	PacketsRecv int64 `json:"packets_recv"`
	Errors      int64 `json:"errors"`

	Timestamp time.Time `json:"timestamp"`
}

// ConnectionPool 连接池.
type ConnectionPool struct {
	ID                string    `json:"id"`
	SubsystemID       string    `json:"subsystem_id"`
	MaxConnections    int       `json:"max_connections"`
	ActiveConnections int       `json:"active_connections"`
	IdleConnections   int       `json:"idle_connections"`
	WaitingRequests   int       `json:"waiting_requests"`
	Timeout           int       `json:"timeout"`      // 超时时间(秒)
	MaxLifetime       int       `json:"max_lifetime"` // 最大生命周期(秒)
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// NVMeOFManager NVMe-oF管理器.
type NVMeOFManager struct {
	mu              sync.RWMutex
	subsystems      map[string]*NVMeSubsystem
	namespaces      map[string]*NVMeNamespace
	controllers     map[string]*NVMeController
	interfaces      map[string]*NetworkInterface
	rdmaConfig      RDMAConfig
	metrics         map[string]*PerformanceMetrics
	connectionPools map[string]*ConnectionPool
	config          ManagerConfig
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	EnableRDMA           bool          `json:"enable_rdma"`
	EnableTCP            bool          `json:"enable_tcp"`
	DefaultTransport     TransportType `json:"default_transport"`
	MaxSubsystems        int           `json:"max_subsystems"`
	MaxNamespacesPerSub  int           `json:"max_namespaces_per_sub"`
	MaxControllersPerSub int           `json:"max_controllers_per_sub"`
	MetricsInterval      int           `json:"metrics_interval"`      // 指标采集间隔(秒)
	HealthCheckInterval  int           `json:"health_check_interval"` // 健康检查间隔(秒)
}

// DefaultManagerConfig 默认管理器配置.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		EnableRDMA:           true,
		EnableTCP:            true,
		DefaultTransport:     TransportTCP,
		MaxSubsystems:        100,
		MaxNamespacesPerSub:  256,
		MaxControllersPerSub: 64,
		MetricsInterval:      10,
		HealthCheckInterval:  30,
	}
}

// 预定义错误.
var (
	ErrSubsystemNotFound  = errors.New("subsystem not found")
	ErrSubsystemExists    = errors.New("subsystem already exists")
	ErrNamespaceNotFound  = errors.New("namespace not found")
	ErrControllerNotFound = errors.New("controller not found")
	ErrInterfaceNotFound  = errors.New("network interface not found")
	ErrInvalidTransport   = errors.New("invalid transport type")
	ErrInvalidSpeed       = errors.New("invalid network speed")
	ErrMaxSubsystems      = errors.New("max subsystems reached")
	ErrMaxNamespaces      = errors.New("max namespaces per subsystem reached")
	ErrMaxControllers     = errors.New("max controllers per subsystem reached")
	ErrRDMANotSupported   = errors.New("RDMA not supported")
	ErrConnectionFailed   = errors.New("connection failed")
)
