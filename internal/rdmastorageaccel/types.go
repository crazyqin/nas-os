// Package rdmastorageaccel 提供 RDMA 存储加速功能，支持 RoCEv2/iWARP 协议，
// 提供高性能存储网络加速，包括 iSCSI over RDMA、NFS over RDMA 等。
package rdmastorageaccel

import "time"

// RDMAProtocol RDMA 协议类型
type RDMAProtocol string

const (
	ProtocolRoCEv2 RDMAProtocol = "roce_v2"
	ProtocolIWARP  RDMAProtocol = "iwarp"
)

// DeviceStatus RDMA 设备状态
type DeviceStatus string

const (
	DeviceStatusActive   DeviceStatus = "active"
	DeviceStatusInactive DeviceStatus = "inactive"
	DeviceStatusError    DeviceStatus = "error"
	DeviceStatusUnknown  DeviceStatus = "unknown"
)

// StorageTargetType 存储目标类型
type StorageTargetType string

const (
	TargetTypeISCSI StorageTargetType = "iscsi"
	TargetTypeNFS   StorageTargetType = "nfs"
	TargetTypeNVMe  StorageTargetType = "nvme"
)

// TargetStatus 存储目标状态
type TargetStatus string

const (
	TargetStatusActive      TargetStatus = "active"
	TargetStatusInactive    TargetStatus = "inactive"
	TargetStatusConnecting  TargetStatus = "connecting"
	TargetStatusError       TargetStatus = "error"
)

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	ConnectionStatusConnected    ConnectionStatus = "connected"
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
	ConnectionStatusConnecting   ConnectionStatus = "connecting"
	ConnectionStatusError        ConnectionStatus = "error"
)

// TuningProfileType 性能调优预设类型
type TuningProfileType string

const (
	ProfileLowLatency  TuningProfileType = "low_latency"
	ProfileHighThroughput TuningProfileType = "high_throughput"
	ProfileBalanced    TuningProfileType = "balanced"
	ProfileCustom      TuningProfileType = "custom"
)

// CongestionAlgorithm 拥塞控制算法
type CongestionAlgorithm string

const (
	CongestionECN   CongestionAlgorithm = "ecn"
	CongestionDCTCP CongestionAlgorithm = "dctcp"
	CongestionDCQCN CongestionAlgorithm = "dcqcn"
	CongestionNone  CongestionAlgorithm = "none"
)

// RDMADevice RDMA 设备信息
type RDMADevice struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	PCIAddress  string       `json:"pci_address"`
	PortCount   int          `json:"port_count"`
	Speed       string       `json:"speed"` // 例如 "100Gb/s", "200Gb/s"
	Status      DeviceStatus `json:"status"`
	Driver      string       `json:"driver"`
	FirmwareVer string       `json:"firmware_version,omitempty"`
	NodeGUID    string       `json:"node_guid,omitempty"`
	SystemImage string       `json:"system_image,omitempty"`
	Ports       []RDMAPort   `json:"ports,omitempty"`
	Capabilities []string    `json:"capabilities,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// RDMAPort RDMA 端口信息
type RDMAPort struct {
	PortNum     int    `json:"port_num"`
	LID         int    `json:"lid"`
	GID         string `json:"gid"`
	State       string `json:"state"`
	PhysState   string `json:"phys_state"`
	Speed       string `json:"speed"`
	Width       int    `json:"width"`
	SMPLID      int    `json:"smp_lid,omitempty"`
	CapMask     int    `json:"cap_mask,omitempty"`
}

// RDMAConfig RDMA 配置
type RDMAConfig struct {
	ID                string              `json:"id"`
	Protocol          RDMAProtocol        `json:"protocol"`
	MTU               int                 `json:"mtu"` // 字节，例如 4096
	CongestionControl CongestionAlgorithm `json:"congestion_control"`
	QoS               QoSConfig           `json:"qos"`
	NetworkDetection  NetworkDetection    `json:"network_detection"`
	Advanced          AdvancedConfig      `json:"advanced"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// QoSConfig QoS 配置
type QoSConfig struct {
	Enabled         bool `json:"enabled"`
	Priority        int  `json:"priority"`         // 0-7
	ServiceLevel    int  `json:"service_level"`    // 0-15
	TrafficClass    int  `json:"traffic_class"`    // 0-7
	MaxSGLen        int  `json:"max_sg_len"`       // 最大 scatter-gather 列表长度
	MaxInlineData   int  `json:"max_inline_data"`  // 最大内联数据大小（字节）
	Timeout         int  `json:"timeout"`          // 超时（毫秒）
	RetryCount      int  `json:"retry_count"`      // 重试次数
	RNRRetry        int  `json:"rnr_retry"`        // RNR 重试次数
}

// NetworkDetection 网络拓扑检测配置
type NetworkDetection struct {
	Enabled          bool `json:"enabled"`
	AutoDetectMTU    bool `json:"auto_detect_mtu"`
	AutoDetectSpeed  bool `json:"auto_detect_speed"`
	DetectionTimeout int  `json:"detection_timeout"` // 秒
}

// AdvancedConfig 高级配置
type AdvancedConfig struct {
	MaxQueuePairs     int  `json:"max_queue_pairs"`
	MaxCQEntries      int  `json:"max_cq_entries"`
	MaxMRSize         int  `json:"max_mr_size"`      // 最大内存注册大小（字节）
	UseEventfd        bool `json:"use_eventfd"`
	NumaAware         bool `json:"numa_aware"`
	CompletionVector  int  `json:"completion_vector"`
	MaxSendWR         int  `json:"max_send_wr"`      // 最大发送工作请求数
	MaxRecvWR         int  `json:"max_recv_wr"`      // 最大接收工作请求数
	MaxRDMAReadAtomic int  `json:"max_rdma_read_atomic"`
}

// StorageTarget 存储目标
type StorageTarget struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        StorageTargetType `json:"type"`
	Status      TargetStatus      `json:"status"`
	RDMAConfig  *RDMAConfig       `json:"rdma_config,omitempty"`
	DeviceID    string            `json:"device_id"`
	TargetAddr  string            `json:"target_addr"`
	Port        int               `json:"port"`
	LUNMappings []LUNMapping      `json:"lun_mappings,omitempty"`
	NFSSettings *NFSSettings      `json:"nfs_settings,omitempty"`
	ISCSISettings *ISCSISettings  `json:"iscsi_settings,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// LUNMapping LUN 映射
type LUNMapping struct {
	LUN       int    `json:"lun"`
	DevicePath string `json:"device_path"`
	SizeBytes int64  `json:"size_bytes"`
	WWN       string `json:"wwn,omitempty"`
	Alias     string `json:"alias,omitempty"`
	ReadOnly  bool   `json:"read_only"`
}

// NFSSettings NFS 特定配置
type NFSSettings struct {
	Version      string `json:"version"`       // "3", "4.0", "4.1", "4.2"
	ExportPath   string `json:"export_path"`
	MountOptions string `json:"mount_options"`
	RDMAEnabled  bool   `json:"rdma_enabled"`
	RDMAPort     int    `json:"rdma_port"`
}

// ISCSISettings iSCSI 特定配置
type ISCSISettings struct {
	TargetIQN    string `json:"target_iqn"`
	InitiatorIQN string `json:"initiator_iqn"`
	CHAPUser     string `json:"chap_user,omitempty"`
	CHAPSecret   string `json:"chap_secret,omitempty"`
	HeaderDigest bool   `json:"header_digest"`
	DataDigest   bool   `json:"data_digest"`
	ImmediateData bool  `json:"immediate_data"`
	MaxRecvSegLen int   `json:"max_recv_segment_length"`
	MaxBurstLen   int   `json:"max_burst_length"`
	FirstBurstLen int   `json:"first_burst_length"`
	RDMAEnabled   bool  `json:"rdma_enabled"`
}

// PerfMetrics 性能指标
type PerfMetrics struct {
	ID           string        `json:"id"`
	DeviceID     string        `json:"device_id"`
	TargetID     string        `json:"target_id,omitempty"`
	BandwidthMBs float64       `json:"bandwidth_mbs"`     // 带宽（MB/s）
	ReadBandwidthMBs  float64  `json:"read_bandwidth_mbs"`
	WriteBandwidthMBs float64  `json:"write_bandwidth_mbs"`
	LatencyUs    float64       `json:"latency_us"`        // 延迟（微秒）
	ReadLatencyUs  float64    `json:"read_latency_us"`
	WriteLatencyUs float64    `json:"write_latency_us"`
	IOPS         int64         `json:"iops"`              // IOPS
	ReadIOPS     int64         `json:"read_iops"`
	WriteIOPS    int64         `json:"write_iops"`
	QueueDepth   int           `json:"queue_depth"`       // 当前队列深度
	MaxQueueDepth int          `json:"max_queue_depth"`
	CPUUsage     float64       `json:"cpu_usage"`         // CPU 使用率（%）
	MemoryUsageMB int64        `json:"memory_usage_mb"`
	CongestionEvents int64     `json:"congestion_events"` // 拥塞事件计数
	Retransmissions  int64     `json:"retransmissions"`  // 重传计数
	Timestamp    time.Time     `json:"timestamp"`
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ID              string           `json:"id"`
	SourceDeviceID  string           `json:"source_device_id"`
	SourceDevice    string           `json:"source_device"`
	TargetDeviceID  string           `json:"target_device_id"`
	TargetDevice    string           `json:"target_device"`
	Status          ConnectionStatus `json:"status"`
	Protocol        RDMAProtocol     `json:"protocol"`
	LocalAddr       string           `json:"local_addr"`
	RemoteAddr      string           `json:"remote_addr"`
	LocalPort       int              `json:"local_port"`
	RemotePort      int              `json:"remote_port"`
	QueuePairNum    int              `json:"queue_pair_num"`
	BytesSent       int64            `json:"bytes_sent"`
	BytesReceived   int64            `json:"bytes_received"`
	PacketsSent     int64            `json:"packets_sent"`
	PacketsReceived int64            `json:"packets_received"`
	Errors          int64            `json:"errors"`
	EstablishedAt   time.Time        `json:"established_at"`
	LastActivity    time.Time        `json:"last_activity"`
}

// TuningProfile 性能调优预设
type TuningProfile struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Type        TuningProfileType  `json:"type"`
	Description string             `json:"description,omitempty"`
	MTU         int                `json:"mtu"`
	QueueDepth  int                `json:"queue_depth"`
	MaxSendWR   int                `json:"max_send_wr"`
	MaxRecvWR   int                `json:"max_recv_wr"`
	Concurrency int                `json:"concurrency"`
	Congestion  CongestionAlgorithm `json:"congestion"`
	QoS         QoSConfig          `json:"qos"`
	IsDefault   bool               `json:"is_default"`
	CreatedAt   time.Time          `json:"created_at"`
}

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	ID          string `json:"id"`
	DeviceID    string `json:"device_id"`
	TargetID    string `json:"target_id,omitempty"`
	TestType    string `json:"test_type"` // "bandwidth", "latency", "iops", "full"
	Duration    int    `json:"duration"`  // 秒
	BlockSize   int    `json:"block_size"` // 字节
	QueueDepth  int    `json:"queue_depth"`
	NumThreads  int    `json:"num_threads"`
	ReadWrite   string `json:"read_write"` // "read", "write", "randread", "randwrite", "rw"
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	ID           string       `json:"id"`
	Config       BenchmarkConfig `json:"config"`
	BandwidthMBs float64      `json:"bandwidth_mbs"`
	ReadBandwidthMBs  float64 `json:"read_bandwidth_mbs"`
	WriteBandwidthMBs float64 `json:"write_bandwidth_mbs"`
	LatencyUs    float64      `json:"latency_us"`
	ReadLatencyUs  float64    `json:"read_latency_us"`
	WriteLatencyUs float64   `json:"write_latency_us"`
	IOPS         int64        `json:"iops"`
	ReadIOPS     int64        `json:"read_iops"`
	WriteIOPS    int64        `json:"write_iops"`
	CPUUsage     float64      `json:"cpu_usage"`
	CompletedAt  time.Time    `json:"completed_at"`
	Duration     time.Duration `json:"duration"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	DeviceID     string    `json:"device_id"`
	TargetID     string    `json:"target_id,omitempty"`
	Healthy      bool      `json:"healthy"`
	Status       string    `json:"status"`
	LatencyMs    float64   `json:"latency_ms"`
	PacketLoss   float64   `json:"packet_loss"` // 百分比
	LastChecked  time.Time `json:"last_checked"`
	Details      map[string]string `json:"details,omitempty"`
}

// APIResponse 统一 API 响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// DefaultRDMAConfig 默认 RDMA 配置
func DefaultRDMAConfig() *RDMAConfig {
	return &RDMAConfig{
		ID:                "default",
		Protocol:          ProtocolRoCEv2,
		MTU:               4096,
		CongestionControl: CongestionDCQCN,
		QoS: QoSConfig{
			Enabled:         true,
			Priority:        3,
			ServiceLevel:    0,
			TrafficClass:    0,
			MaxSGLen:        32,
			MaxInlineData:   256,
			Timeout:         20,
			RetryCount:      7,
			RNRRetry:        7,
		},
		NetworkDetection: NetworkDetection{
			Enabled:          true,
			AutoDetectMTU:    true,
			AutoDetectSpeed:  true,
			DetectionTimeout: 30,
		},
		Advanced: AdvancedConfig{
			MaxQueuePairs:     256,
			MaxCQEntries:      65536,
			MaxMRSize:         1073741824, // 1GB
			UseEventfd:        true,
			NumaAware:         true,
			CompletionVector:  0,
			MaxSendWR:         1024,
			MaxRecvWR:         1024,
			MaxRDMAReadAtomic: 16,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// DefaultTuningProfiles 默认性能调优预设
func DefaultTuningProfiles() []TuningProfile {
	return []TuningProfile{
		{
			ID:          "profile-low-latency",
			Name:        "低延迟模式",
			Type:        ProfileLowLatency,
			Description: "针对延迟敏感型应用优化，如数据库、虚拟化",
			MTU:         4096,
			QueueDepth:  64,
			MaxSendWR:   256,
			MaxRecvWR:   256,
			Concurrency: 4,
			Congestion:  CongestionDCTCP,
			QoS: QoSConfig{
				Enabled:       true,
				Priority:      5,
				ServiceLevel:  0,
				TrafficClass:  0,
				MaxSGLen:      16,
				MaxInlineData: 512,
				Timeout:       10,
				RetryCount:    5,
				RNRRetry:      5,
			},
			IsDefault: false,
			CreatedAt: time.Now(),
		},
		{
			ID:          "profile-high-throughput",
			Name:        "高吞吐模式",
			Type:        ProfileHighThroughput,
			Description: "针对大文件传输优化，如备份、媒体处理",
			MTU:         4096,
			QueueDepth:  256,
			MaxSendWR:   1024,
			MaxRecvWR:   1024,
			Concurrency: 16,
			Congestion:  CongestionDCQCN,
			QoS: QoSConfig{
				Enabled:       true,
				Priority:      3,
				ServiceLevel:  0,
				TrafficClass:  0,
				MaxSGLen:      64,
				MaxInlineData: 256,
				Timeout:       20,
				RetryCount:    7,
				RNRRetry:      7,
			},
			IsDefault: false,
			CreatedAt: time.Now(),
		},
		{
			ID:          "profile-balanced",
			Name:        "平衡模式",
			Type:        ProfileBalanced,
			Description: "兼顾延迟和吞吐，适合通用场景",
			MTU:         4096,
			QueueDepth:  128,
			MaxSendWR:   512,
			MaxRecvWR:   512,
			Concurrency: 8,
			Congestion:  CongestionDCQCN,
			QoS: QoSConfig{
				Enabled:       true,
				Priority:      4,
				ServiceLevel:  0,
				TrafficClass:  0,
				MaxSGLen:      32,
				MaxInlineData: 256,
				Timeout:       15,
				RetryCount:    6,
				RNRRetry:      6,
			},
			IsDefault: true,
			CreatedAt: time.Now(),
		},
	}
}

// IsValidProtocol 检查协议是否有效
func IsValidProtocol(p RDMAProtocol) bool {
	return p == ProtocolRoCEv2 || p == ProtocolIWARP
}

// IsValidTargetType 检查存储目标类型是否有效
func IsValidTargetType(t StorageTargetType) bool {
	return t == TargetTypeISCSI || t == TargetTypeNFS || t == TargetTypeNVMe
}

// IsValidCongestionAlgorithm 检查拥塞控制算法是否有效
func IsValidCongestionAlgorithm(a CongestionAlgorithm) bool {
	return a == CongestionECN || a == CongestionDCTCP || a == CongestionDCQCN || a == CongestionNone
}

// IsValidProfileType 检查调优预设类型是否有效
func IsValidProfileType(p TuningProfileType) bool {
	return p == ProfileLowLatency || p == ProfileHighThroughput || p == ProfileBalanced || p == ProfileCustom
}