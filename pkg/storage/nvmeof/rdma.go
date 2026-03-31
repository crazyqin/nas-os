// Package nvmeof - NVMe over RDMA (NVMe/RDMA) 支持
// 实现 RoCEv2、iWARP、InfiniBand 传输
// 参考：TrueNAS 25.10 实现75GB/s带宽

package nvmeof

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ========== RDMA 特定错误 ==========

var (
	// ErrRDMANotAvailable RDMA 不可用
	ErrRDMANotAvailable = errors.New("rdma transport not available")
	// ErrRDMADeviceNotFound RDMA 设备未找到
	ErrRDMADeviceNotFound = errors.New("rdma device not found")
	// ErrRDMAPortNotFound RDMA 端口未找到
	ErrRDMAPortNotFound = errors.New("rdma port not found")
	// ErrRDMAGIDNotFound RDMA GID 未找到
	ErrRDMAGIDNotFound = errors.New("rdma gid not found")
	// ErrRDMABindFailed RDMA 绑定失败
	ErrRDMABindFailed = errors.New("rdma bind failed")
	// ErrInvalidRDMAConfig 无效 RDMA 配置
	ErrInvalidRDMAConfig = errors.New("invalid rdma configuration")
)

// ========== RDMA 传输类型 ==========

// RDMATransportType RDMA 传输子类型
type RDMATransportType string

const (
	// RDMATransportRoCEv2 RDMA over Converged Ethernet v2
	RDMATransportRoCEv2 RDMATransportType = "rocev2"
	// RDMATransportiWARP Internet Wide Area RDMA Protocol
	RDMATransportiWARP RDMATransportType = "iwarp"
	// RDMATransportIB InfiniBand
	RDMATransportIB RDMATransportType = "ib"
)

// ValidRDMATransports 有效 RDMA 传输类型
var ValidRDMATransports = map[RDMATransportType]bool{
	RDMATransportRoCEv2: true,
	RDMATransportiWARP:  true,
	RDMATransportIB:     true,
}

// ========== RDMA 设备信息 ==========

// RDMADevice RDMA 设备信息
type RDMADevice struct {
	// 设备名称 (如: mlx5_0)
	Name string `json:"name"`

	// 设备类型 (如: RoCE, IB, iWARP)
	TransportType RDMATransportType `json:"transportType"`

	// 固件版本
	FirmwareVersion string `json:"firmwareVersion"`

	// 节点 GUID
	NodeGUID string `json:"nodeGuid"`

	// 端口数量
	Ports int `json:"ports"`

	// 最大传输单元 (MTU)
	MTU int `json:"mtu"`

	// 最大带宽 (Gbps)
	MaxBandwidth int `json:"maxBandwidth"`

	// 状态
	State RDMADeviceState `json:"state"`

	// 端口信息
	PortInfo []RDMAPortInfo `json:"portInfo"`

	// GID 列表
	GIDs []RDMAGID `json:"gids"`

	// 统计
	Stats RDMADeviceStats `json:"stats"`
}

// RDMADeviceState RDMA 设备状态
type RDMADeviceState string

const (
	// RDMADeviceStateUp 设备在线
	RDMADeviceStateUp RDMADeviceState = "up"
	// RDMADeviceStateDown 设备离线
	RDMADeviceStateDown RDMADeviceState = "down"
	// RDMADeviceStateInit 设备初始化中
	RDMADeviceStateInit RDMADeviceState = "init"
	// RDMADeviceStateError 设备错误
	RDMADeviceStateError RDMADeviceState = "error"
)

// RDMAPortInfo RDMA 端口信息
type RDMAPortInfo struct {
	// 端口编号
	PortNum int `json:"portNum"`

	// 端口状态
	State RDMAPortState `json:"state"`

	// 物理状态
	PhysState RDMAPortPhysState `json:"physState"`

	// 链路层 (如: Ethernet, InfiniBand)
	LinkLayer string `json:"linkLayer"`

	// 活动速率 (Gbps)
	ActiveRate int `json:"activeRate"`

	// 活动 MTU
	ActiveMTU int `json:"activeMtu"`

	// 网络设备名称 (如: eth0, ib0)
	NetDevice string `json:"netDevice"`
}

// RDMAPortState RDMA 端口状态
type RDMAPortState string

const (
	// RDMAPortStateUp 端口在线
	RDMAPortStateUp RDMAPortState = "up"
	// RDMAPortStateDown 端口离线
	RDMAPortStateDown RDMAPortState = "down"
	// RDMAPortStateInit 端口初始化中
	RDMAPortStateInit RDMAPortState = "init"
)

// RDMAPortPhysState RDMA 端口物理状态
type RDMAPortPhysState string

const (
	// RDMAPortPhysStateSleep 端口休眠
	RDMAPortPhysStateSleep RDMAPortPhysState = "sleep"
	// RDMAPortPhysStatePolling 端口轮询中
	RDMAPortPhysStatePolling RDMAPortPhysState = "polling"
	// RDMAPortPhysStateDisabled 端口禁用
	RDMAPortPhysStateDisabled RDMAPortPhysState = "disabled"
	// RDMAPortPhysStatePortConfiguration 端口配置中
	RDMAPortPhysStatePortConfiguration RDMAPortPhysState = "port_configuration"
	// RDMAPortPhysStateLinkUp 端口链路建立
	RDMAPortPhysStateLinkUp RDMAPortPhysState = "link_up"
	// RDMAPortPhysStateLinkDown 端口链路断开
	RDMAPortPhysStateLinkDown RDMAPortPhysState = "link_down"
	// RDMAPortPhysStateUnknown 端口状态未知
	RDMAPortPhysStateUnknown RDMAPortPhysState = "unknown"
)

// RDMAGID RDMA 全局标识符 (Global Identifier)
type RDMAGID struct {
	// GID 索引
	Index int `json:"index"`

	// GID 地址 (IPv6 格式)
	GID string `json:"gid"`

	// GID 类型 (如: RoCEv2, IB)
	Type string `json:"type"`

	// 网络前缀长度
	PrefixLen int `json:"prefixLen"`

	// IP 地址 (对于 RoCEv2)
	IPAddress string `json:"ipAddress"`
}

// RDMADeviceStats RDMA 设备统计
type RDMADeviceStats struct {
	// 发送统计
	TxBytes    uint64 `json:"txBytes"`
	TxPackets  uint64 `json:"txPackets"`
	TxErrors   uint64 `json:"txErrors"`
	TxDropped  uint64 `json:"txDropped"`

	// 接收统计
	RxBytes    uint64 `json:"rxBytes"`
	RxPackets  uint64 `json:"rxPackets"`
	RxErrors   uint64 `json:"rxErrors"`
	RxDropped  uint64 `json:"rxDropped"`

	// RDMA 特定统计
	RDMAReadOps   uint64 `json:"rdmaReadOps"`
	RDMAWriteOps  uint64 `json:"rdmaWriteOps"`
	RDMASendOps   uint64 `json:"rdmaSendOps"`
	RDMARecvOps   uint64 `json:"rdmaRecvOps"`

	// 队列统计
	QPCount    int    `json:"qpCount"`
	CQCount    int    `json:"cqCount"`
	SRQCount   int    `json:"srqCount"`
}

// ========== RDMA 配置 ==========

// RDMAConfig RDMA 配置
type RDMAConfig struct {
	// 是否启用 RDMA
	Enabled bool `json:"enabled"`

	// 传输类型 (RoCEv2, iWARP, IB)
	TransportType RDMATransportType `json:"transportType"`

	// 队列深度
	QueueDepth int `json:"queueDepth"`

	// 发送队列深度
	SQDepth int `json:"sqDepth"`

	// 接收队列深度
	RQDepth int `json:"rqDepth"`

	// 完成队列深度
	CQDepth int `json:"cqDepth"`

	// 最大工作请求
	MaxWR int `json:"maxWr"`

	// 最大内联数据大小 (字节)
	MaxInlineData int `json:"maxInlineData"`

	// 是否启用零拷贝
	ZeroCopy bool `json:"zeroCopy"`

	// 是否启用轮询模式
	PollMode bool `json:"pollMode"`

	// CPU 亲和性列表
	CPUAffinity []int `json:"cpuAffinity"`

	// 端口配置
	PortConfig RDMAPortConfig `json:"portConfig"`

	// 性能调优
	Performance RDMAPerformanceConfig `json:"performance"`

	// 重连配置
	Reconnect RDMAReconnectConfig `json:"reconnect"`
}

// RDMAPortConfig RDMA 端口配置
type RDMAPortConfig struct {
	// 设备名称
	Device string `json:"device"`

	// 端口编号
	PortNum int `json:"portNum"`

	// GID 索引
	GIDIndex int `json:"gidIndex"`

	// IP 地址
	IPAddress string `json:"ipAddress"`

	// 服务端口
	ServicePort int `json:"servicePort"`

	// MTU
	MTU int `json:"mtu"`
}

// RDMAPerformanceConfig RDMA 性能配置
type RDMAPerformanceConfig struct {
	// 最大 IO 大小 (KB)
	MaxIOSize int `json:"maxIoSize"`

	// 批量大小
	BatchSize int `json:"batchSize"`

	// 预读大小 (KB)
	ReadAhead int `json:"readAhead"`

	// 是否启用数据包聚合
	PacketAggregation bool `json:"packetAggregation"`

	// 流控制
	FlowControl bool `json:"flowControl"`
}

// RDMAReconnectConfig RDMA 重连配置
type RDMAReconnectConfig struct {
	// 重连延迟 (秒)
	Delay int `json:"delay"`

	// 最大重连次数
	MaxAttempts int `json:"maxAttempts"`

	// 重连超时 (秒)
	Timeout int `json:"timeout"`

	// 是否启用指数退避
	ExponentialBackoff bool `json:"exponentialBackoff"`
}

// DefaultRDMAConfig 默认 RDMA 配置
func DefaultRDMAConfig() *RDMAConfig {
	return &RDMAConfig{
		Enabled:         true,
		TransportType:   RDMATransportRoCEv2,
		QueueDepth:      128,
		SQDepth:         128,
		RQDepth:         128,
		CQDepth:         256,
		MaxWR:           128,
		MaxInlineData:   4096,
		ZeroCopy:        true,
		PollMode:        true,
		CPUAffinity:     []int{},
		PortConfig:      RDMAPortConfig{
			PortNum:    1,
			GIDIndex:   0,
			ServicePort: 4420,
			MTU:        9000,
		},
		Performance:     RDMAPerformanceConfig{
			MaxIOSize:        128,
			BatchSize:        32,
			ReadAhead:        512,
			PacketAggregation: true,
			FlowControl:      true,
		},
		Reconnect:       RDMAReconnectConfig{
			Delay:             10,
			MaxAttempts:       30,
			Timeout:           60,
			ExponentialBackoff: true,
		},
	}
}

// Validate 验证 RDMA 配置
func (c *RDMAConfig) Validate() error {
	if !ValidRDMATransports[c.TransportType] {
		c.TransportType = RDMATransportRoCEv2
	}

	if c.QueueDepth <= 0 {
		c.QueueDepth = 128
	}

	if c.SQDepth <= 0 {
		c.SQDepth = 128
	}

	if c.RQDepth <= 0 {
		c.RQDepth = 128
	}

	if c.CQDepth <= 0 {
		c.CQDepth = 256
	}

	if c.MaxWR <= 0 {
		c.MaxWR = 128
	}

	if c.MaxInlineData < 0 {
		c.MaxInlineData = 0
	}

	if c.PortConfig.PortNum <= 0 {
		c.PortConfig.PortNum = 1
	}

	if c.PortConfig.GIDIndex < 0 {
		c.PortConfig.GIDIndex = 0
	}

	if c.PortConfig.ServicePort <= 0 || c.PortConfig.ServicePort > 65535 {
		c.PortConfig.ServicePort = 4420
	}

	if c.PortConfig.MTU <= 0 {
		c.PortConfig.MTU = 9000
	}

	if c.Reconnect.Delay <= 0 {
		c.Reconnect.Delay = 10
	}

	if c.Reconnect.MaxAttempts <= 0 {
		c.Reconnect.MaxAttempts = 30
	}

	if c.Reconnect.Timeout <= 0 {
		c.Reconnect.Timeout = 60
	}

	return nil
}

// ========== RDMA Target 配置 ==========

// RDMATargetConfig RDMA Target 配置
type RDMATargetConfig struct {
	// 子系统 NQN
	SubsysNQN string `json:"subsysNqn"`

	// RDMA 设备
	Device string `json:"device"`

	// 端口编号
	PortNum int `json:"portNum"`

	// GID 索引
	GIDIndex int `json:"gidIndex"`

	// IP 地址
	IPAddress string `json:"ipAddress"`

	// 服务端口
	ServicePort int `json:"servicePort"`

	// 队列深度配置
	QueueDepth int `json:"queueDepth"`

	// 最大传输单元
	MTU int `json:"mtu"`

	// 是否启用
	Enabled bool `json:"enabled"`
}

// DefaultRDMATargetConfig 默认 RDMA Target 配置
func DefaultRDMATargetConfig() *RDMATargetConfig {
	return &RDMATargetConfig{
		PortNum:    1,
		GIDIndex:   0,
		ServicePort: 4420,
		QueueDepth: 128,
		MTU:        9000,
		Enabled:    true,
	}
}

// Validate 验证 RDMA Target 配置
func (c *RDMATargetConfig) Validate() error {
	if c.SubsysNQN == "" {
		return fmt.Errorf("subsys_nqn is required")
	}

	if c.Device == "" {
		return fmt.Errorf("device is required")
	}

	if c.PortNum <= 0 {
		c.PortNum = 1
	}

	if c.GIDIndex < 0 {
		c.GIDIndex = 0
	}

	if c.IPAddress == "" {
		return fmt.Errorf("ip_address is required")
	}

	if c.ServicePort <= 0 || c.ServicePort > 65535 {
		c.ServicePort = 4420
	}

	if c.QueueDepth <= 0 {
		c.QueueDepth = 128
	}

	if c.MTU <= 0 {
		c.MTU = 9000
	}

	return nil
}

// ========== RDMA Initiator 配置 ==========

// RDMAInitiatorConfig RDMA Initiator 配置
type RDMAInitiatorConfig struct {
	// 目标子系统 NQN
	TargetNQN string `json:"targetNqn"`

	// 目标 IP 地址
	TargetAddress string `json:"targetAddress"`

	// 目标端口
	TargetPort int `json:"targetPort"`

	// 本地 Host NQN
	HostNQN string `json:"hostNqn"`

	// 本地 Host ID
	HostID string `json:"hostId"`

	// 本地 RDMA 设备
	LocalDevice string `json:"localDevice"`

	// 本地 GID 索引
	LocalGIDIndex int `json:"localGidIndex"`

	// 队列深度配置
	QueueDepth int `json:"queueDepth"`

	// IO 队列数量
	IOQueues int `json:"ioQueues"`

	// 重连配置
	ReconnectDelay int `json:"reconnectDelay"`
	MaxReconnect   int `json:"maxReconnect"`

	// 是否启用轮询模式
	PollMode bool `json:"pollMode"`

	// DHCHAP 密钥 (可选)
	DHCHAPKey string `json:"dhchapKey,omitempty"`
}

// DefaultRDMAInitiatorConfig 默认 RDMA Initiator 配置
func DefaultRDMAInitiatorConfig() *RDMAInitiatorConfig {
	return &RDMAInitiatorConfig{
		TargetPort:     4420,
		LocalGIDIndex:  0,
		QueueDepth:     128,
		IOQueues:       8,
		ReconnectDelay: 10,
		MaxReconnect:   30,
		PollMode:       true,
	}
}

// Validate 验证 RDMA Initiator 配置
func (c *RDMAInitiatorConfig) Validate() error {
	if c.TargetNQN == "" {
		return fmt.Errorf("target_nqn is required")
	}

	if c.TargetAddress == "" {
		return fmt.Errorf("target_address is required")
	}

	if c.TargetPort <= 0 || c.TargetPort > 65535 {
		c.TargetPort = 4420
	}

	if c.QueueDepth <= 0 {
		c.QueueDepth = 128
	}

	if c.IOQueues <= 0 {
		c.IOQueues = 8
	}

	if c.ReconnectDelay <= 0 {
		c.ReconnectDelay = 10
	}

	if c.MaxReconnect <= 0 {
		c.MaxReconnect = 30
	}

	return nil
}

// ========== RDMA 性能统计 ==========

// RDMAPerformanceStats RDMA 性能统计
type RDMAPerformanceStats struct {
	// 带宽统计
	ReadBandwidth  uint64 `json:"readBandwidth"`  // bytes/s
	WriteBandwidth uint64 `json:"writeBandwidth"` // bytes/s

	// IOPS 统计
	ReadIOPS  uint64 `json:"readIops"`
	WriteIOPS uint64 `json:"writeIops"`

	// 延迟统计
	AvgLatency uint64 `json:"avgLatency"` // 微秒
	P50Latency uint64 `json:"p50Latency"` // 微秒
	P95Latency uint64 `json:"p95Latency"` // 微秒
	P99Latency uint64 `json:"p99Latency"` // 微秒
	MaxLatency uint64 `json:"maxLatency"` // 微秒

	// 队列统计
	SQDepth   int `json:"sqDepth"`
	RQDepth   int `json:"rqDepth"`
	CQDepth   int `json:"cqDepth"`
	ActiveQP  int `json:"activeQp"`

	// 错误统计
	RDMAErrors  uint64 `json:"rdmaErrors"`
	TxErrors    uint64 `json:"txErrors"`
	RxErrors    uint64 `json:"rxErrors"`
	Timeouts    uint64 `json:"timeouts"`

	// 重连统计
	ReconnectCount uint64 `json:"reconnectCount"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// ========== RDMA 管理器 ==========

// RDMAManager RDMA 管理器
type RDMAManager struct {
	mu sync.RWMutex

	// 配置
	config *RDMAConfig

	// 设备列表
	devices map[string]*RDMADevice

	// Target 配置列表
	targetConfigs map[string]*RDMATargetConfig

	// Initiator 配置列表
	initiatorConfigs map[string]*RDMAInitiatorConfig

	// 可用性
	available bool

	// 运行状态
	running bool

	// 事件通道
	eventCh chan<- NVMeOFEvent
}

// NewRDMAManager 创建 RDMA 管理器
func NewRDMAManager(config *RDMAConfig) (*RDMAManager, error) {
	if config == nil {
		config = DefaultRDMAConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	m := &RDMAManager{
		config:           config,
		devices:          make(map[string]*RDMADevice),
		targetConfigs:    make(map[string]*RDMATargetConfig),
		initiatorConfigs: make(map[string]*RDMAInitiatorConfig),
	}

	// 检测 RDMA 可用性
	m.checkAvailability()

	// 如果可用，扫描设备
	if m.available {
		m.scanDevices()
	}

	return m, nil
}

// checkAvailability 检测 RDMA 可用性
func (m *RDMAManager) checkAvailability() {
	// 实际实现需要检查:
	// - /sys/class/infiniband 目录是否存在
	// - 内核模块 ib_core, ib_uverbs 是否加载
	// - nvmet-rdma 或 nvme-rdma 模块是否可用
	m.available = true
}

// IsAvailable 检查 RDMA 是否可用
func (m *RDMAManager) IsAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.available
}

// scanDevices 扫描 RDMA 设备
func (m *RDMAManager) scanDevices() {
	// 实际实现需要读取 /sys/class/infiniband/*
	// 以及 ibv_devinfo 输出

	// 模拟设备扫描
	m.devices["mlx5_0"] = &RDMADevice{
		Name:          "mlx5_0",
		TransportType: RDMATransportRoCEv2,
		FirmwareVersion: "16.32.1010",
		NodeGUID:      "50:6b:4b:0d:00:00:00:00",
		Ports:         1,
		MTU:           9000,
		MaxBandwidth:  100,
		State:         RDMADeviceStateUp,
		PortInfo: []RDMAPortInfo{
			{
				PortNum:    1,
				State:      RDMAPortStateUp,
				PhysState:  RDMAPortPhysStateLinkUp,
				LinkLayer:  "Ethernet",
				ActiveRate: 100,
				ActiveMTU:  9000,
				NetDevice:  "eth0",
			},
		},
		GIDs: []RDMAGID{
			{
				Index:      0,
				GID:        "fe80:0000:0000:0000:526b:4b0d:0000:0000",
				Type:       "RoCEv2",
				IPAddress:  "192.168.100.100",
			},
		},
	}
}

// GetDevices 获取 RDMA 设备列表
func (m *RDMAManager) GetDevices() []*RDMADevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RDMADevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}
	return result
}

// GetDevice 获取指定 RDMA 设备
func (m *RDMAManager) GetDevice(name string) (*RDMADevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[name]
	if !exists {
		return nil, ErrRDMADeviceNotFound
	}
	return device, nil
}

// GetDeviceByIP 根据IP地址查找RDMA设备
func (m *RDMAManager) GetDeviceByIP(ip string) (*RDMADevice, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, device := range m.devices {
		for _, gid := range device.GIDs {
			if gid.IPAddress == ip {
				return device, gid.Index, nil
			}
		}
	}
	return nil, -1, ErrRDMAGIDNotFound
}

// ========== RDMA Target 管理 ==========

// CreateRDMATarget 创建 RDMA Target 配置
func (m *RDMAManager) CreateRDMATarget(ctx context.Context, req *RDMATargetConfig) (*RDMATargetConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 检查设备是否存在
	device, exists := m.devices[req.Device]
	if !exists {
		return nil, ErrRDMADeviceNotFound
	}

	// 检查设备状态
	if device.State != RDMADeviceStateUp {
		return nil, fmt.Errorf("rdma device %s is not up", req.Device)
	}

	// 检查端口
	if req.PortNum > device.Ports {
		return nil, ErrRDMAPortNotFound
	}

	// 检查 GID 索引
	if req.GIDIndex >= len(device.GIDs) {
		return nil, ErrRDMAGIDNotFound
	}

	// 存储 Target 配置
	m.targetConfigs[req.SubsysNQN] = req

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventSubsystemCreated,
			Message:   fmt.Sprintf("RDMA Target %s created on device %s", req.SubsysNQN, req.Device),
			Subsystem: req.SubsysNQN,
			Time:      time.Now(),
		}
	}

	return req, nil
}

// DeleteRDMATarget 删除 RDMA Target 配置
func (m *RDMAManager) DeleteRDMATarget(ctx context.Context, subsysNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.targetConfigs[subsysNQN]; !exists {
		return ErrSubsystemNotFound
	}

	delete(m.targetConfigs, subsysNQN)

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:      EventSubsystemDeleted,
			Message:   fmt.Sprintf("RDMA Target %s deleted", subsysNQN),
			Subsystem: subsysNQN,
			Time:      time.Now(),
		}
	}

	return nil
}

// ListRDMATargets 列出 RDMA Target 配置
func (m *RDMAManager) ListRDMATargets() []*RDMATargetConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RDMATargetConfig, 0, len(m.targetConfigs))
	for _, c := range m.targetConfigs {
		result = append(result, c)
	}
	return result
}

// ========== RDMA Initiator 管理 ==========

// CreateRDMAInitiator 创建 RDMA Initiator 配置
func (m *RDMAManager) CreateRDMAInitiator(ctx context.Context, req *RDMAInitiatorConfig) (*RDMAInitiatorConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 如果指定了本地设备，检查设备是否存在
	if req.LocalDevice != "" {
		device, exists := m.devices[req.LocalDevice]
		if !exists {
			return nil, ErrRDMADeviceNotFound
		}
		if device.State != RDMADeviceStateUp {
			return nil, fmt.Errorf("rdma device %s is not up", req.LocalDevice)
		}
		if req.LocalGIDIndex >= len(device.GIDs) {
			return nil, ErrRDMAGIDNotFound
		}
	}

	// 生成 ID
	id := req.TargetNQN + "-" + req.TargetAddress

	// 存储 Initiator 配置
	m.initiatorConfigs[id] = req

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:       EventControllerConnected,
			Message:    fmt.Sprintf("RDMA Initiator to %s created", req.TargetNQN),
			Controller: id,
			Time:       time.Now(),
		}
	}

	return req, nil
}

// DeleteRDMAInitiator 删除 RDMA Initiator 配置
func (m *RDMAManager) DeleteRDMAInitiator(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.initiatorConfigs[id]; !exists {
		return ErrControllerDisconnected
	}

	delete(m.initiatorConfigs, id)

	// 发送事件
	if m.eventCh != nil {
		m.eventCh <- NVMeOFEvent{
			Type:       EventControllerDisconnected,
			Message:    fmt.Sprintf("RDMA Initiator %s deleted", id),
			Controller: id,
			Time:       time.Now(),
		}
	}

	return nil
}

// ListRDMAInitiators 列出 RDMA Initiator 配置
func (m *RDMAManager) ListRDMAInitiators() []*RDMAInitiatorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RDMAInitiatorConfig, 0, len(m.initiatorConfigs))
	for _, c := range m.initiatorConfigs {
		result = append(result, c)
	}
	return result
}

// ========== 性能统计 ==========

// GetPerformanceStats 获取 RDMA 性能统计
func (m *RDMAManager) GetPerformanceStats(deviceName string) (*RDMAPerformanceStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceName]
	if !exists {
		return nil, ErrRDMADeviceNotFound
	}

	stats := &RDMAPerformanceStats{
		SQDepth:   m.config.SQDepth,
		RQDepth:   m.config.RQDepth,
		CQDepth:   m.config.CQDepth,
		ActiveQP:  device.Stats.QPCount,
		TxErrors:  device.Stats.TxErrors,
		RxErrors:  device.Stats.RxErrors,
		Timestamp: time.Now(),
	}

	// 计算带宽 (基于设备统计)
	// 实际实现需要读取更详细的性能计数器

	return stats, nil
}

// ========== 辅助方法 ==========

// SetEventChannel 设置事件通道
func (m *RDMAManager) SetEventChannel(ch chan<- NVMeOFEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCh = ch
}

// Start 启动 RDMA 管理器
func (m *RDMAManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 实际实现需要:
	// - 加载 nvmet-rdma 模块
	// - 配置 RDMA 端口
	// - 创建 Target 端点

	m.running = true
	return nil
}

// Stop 停止 RDMA 管理器
func (m *RDMAManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	return nil
}

// GetConfig 获取配置
func (m *RDMAManager) GetConfig() *RDMAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ========== RDMA 工具函数 ==========

// CheckRDMAAvailable 检查 RDMA 是否可用
func CheckRDMAAvailable() (bool, error) {
	// 实际实现需要检查:
	// - /sys/class/infiniband 目录
	// - ibv_devinfo 命令
	// - 内核模块
	return true, nil
}

// GetRDMAKernelModules 获取 RDMA 内核模块
func GetRDMAKernelModules() ([]string, error) {
	// 实际实现需要读取 /proc/modules
	return []string{
		"ib_core",
		"ib_uverbs",
		"ib_cm",
		"rdma_cm",
		"rdma_ucm",
		"mlx5_core",
		"mlx5_ib",
	}, nil
}

// LoadRDMAKernelModules 加载 RDMA 内核模块
func LoadRDMAKernelModules() error {
	// 实际实现需要调用 modprobe
	// modprobe ib_core ib_uverbs rdma_cm mlx5_core mlx5_ib
	return nil
}

// ValidateRDMAGID 验证 RDMA GID
func ValidateRDMAGID(gid string) bool {
	// GID 格式为 IPv6 地址
	// 实际实现需要验证格式
	return len(gid) > 0
}

// ParseGIDIPAddress 从 GID 中解析 IP 地址
func ParseGIDIPAddress(gid string) string {
	// RoCEv2 GID 包含 IPv4 映射的 IPv6 地址
	// 实际实现需要解析 GID 格式
	return ""
}