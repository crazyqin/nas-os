// Package rdmanetwork 实现RDMA网络支持
// 对标 TrueNAS 25.04 RDMA 功能
// 支持 iSCSI/NFS RDMA、RoCE、InfiniBand
package rdmanetwork

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// RDMAType RDMA类型
type RDMAType int

const (
	// RDMATypeRoCE RoCE (RDMA over Converged Ethernet)
	RDMATypeRoCE RDMAType = iota
	// RDMATypeInfiniBand InfiniBand
	RDMATypeInfiniBand
	// RDMATypeiWARP iWARP (Internet Wide Area RDMA Protocol)
	RDMATypeiWARP
)

// RDMAState RDMA连接状态
type RDMAState int

const (
	RDMAStateDisconnected RDMAState = iota
	RDMAStateConnecting
	RDMAStateConnected
	RDMAStateError
)

// RDMANetworkManager RDMA网络管理器
type RDMANetworkManager struct {
	mu          sync.RWMutex
	config      RDMAConfig
	devices     map[string]*RDMADevice
	connections map[string]*RDMAConnection
	stats       RDMAStats
	running     bool
	stopCh      chan struct{}
}

// RDMAConfig RDMA配置
type RDMAConfig struct {
	Type            RDMAType
	MTU             int
	QueuePairCount  int
	MaxInlineData   int
	MaxSendWr       int
	MaxRecvWr       int
	MaxSge          int
	Timeout         time.Duration
	RetryCount      int
	EnableSRQ       bool // Shared Receive Queue
	EnableAtomic    bool
	EnableRDMAWrite bool
	EnableRDMARead  bool
}

// RDMADevice RDMA设备
type RDMADevice struct {
	Name         string
	GUID         string
	Type         RDMAType
	Ports        []*RDMAPort
	Capabilities []string
	State        RDMAState
	Metadata     map[string]string
}

// RDMAPort RDMA端口
type RDMAPort struct {
	ID           int
	LID          int // Local Identifier
	GID          string
	State        RDMAState
	MTU          int
	ActiveSpeed  int
	ActiveWidth  int
	PhysState    int
	SubnetPrefix string
}

// RDMAConnection RDMA连接
type RDMAConnection struct {
	ID           string
	LocalDevice  string
	LocalPort    int
	RemoteDevice string
	RemotePort   int
	RemoteAddr   net.IP
	State        RDMAState
	QueuePair    *QueuePair
	Stats        ConnectionStats
	CreatedAt    time.Time
	LastActivity time.Time
}

// QueuePair 队列对
type QueuePair struct {
	ID           int
	State        int
	AccessFlags  int
	MaxSendWr    int
	MaxRecvWr    int
	MaxSge       int
	MaxInlineData int
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	SendBytes      int64
	RecvBytes      int64
	SendPackets    int64
	RecvPackets    int64
	SendErrors     int64
	RecvErrors     int64
	Retransmissions int64
	AvgLatency     time.Duration
	MaxLatency     time.Duration
}

// RDMAStats RDMA统计
type RDMAStats struct {
	TotalDevices     int
	ActiveDevices    int
	TotalConnections int
	ActiveConnections int
	TotalSendBytes   int64
	TotalRecvBytes   int64
	TotalSendPackets int64
	TotalRecvPackets int64
	AvgLatency       time.Duration
}

// NewRDMANetworkManager 创建RDMA网络管理器
func NewRDMANetworkManager(config RDMAConfig) *RDMANetworkManager {
	if config.MTU <= 0 {
		config.MTU = 4096
	}
	if config.QueuePairCount <= 0 {
		config.QueuePairCount = 100
	}
	if config.MaxInlineData <= 0 {
		config.MaxInlineData = 256
	}
	if config.MaxSendWr <= 0 {
		config.MaxSendWr = 1000
	}
	if config.MaxRecvWr <= 0 {
		config.MaxRecvWr = 1000
	}
	if config.MaxSge <= 0 {
		config.MaxSge = 1
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryCount <= 0 {
		config.RetryCount = 3
	}

	return &RDMANetworkManager{
		config:      config,
		devices:     make(map[string]*RDMADevice),
		connections: make(map[string]*RDMAConnection),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动RDMA网络管理器
func (m *RDMANetworkManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("RDMA network manager already running")
	}

	m.running = true
	go m.monitorLoop()

	return nil
}

// Stop 停止RDMA网络管理器
func (m *RDMANetworkManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	close(m.stopCh)
	m.running = false

	// 关闭所有连接
	for _, conn := range m.connections {
		m.disconnect(conn.ID)
	}

	return nil
}

// RegisterDevice 注册RDMA设备
func (m *RDMANetworkManager) RegisterDevice(device *RDMADevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[device.Name]; exists {
		return fmt.Errorf("device already registered: %s", device.Name)
	}

	device.State = RDMAStateDisconnected
	m.devices[device.Name] = device
	m.stats.TotalDevices++

	return nil
}

// UnregisterDevice 注销RDMA设备
func (m *RDMANetworkManager) UnregisterDevice(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[name]
	if !exists {
		return fmt.Errorf("device not found: %s", name)
	}

	// 检查是否有活动连接
	for _, conn := range m.connections {
		if conn.LocalDevice == name || conn.RemoteDevice == name {
			return fmt.Errorf("device has active connections: %s", name)
		}
	}

	delete(m.devices, name)
	if device.State == RDMAStateConnected {
		m.stats.ActiveDevices--
	}
	m.stats.TotalDevices--

	return nil
}

// Connect 建立RDMA连接
func (m *RDMANetworkManager) Connect(localDevice, remoteAddr string, remotePort int) (*RDMAConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查本地设备
	local, exists := m.devices[localDevice]
	if !exists {
		return nil, fmt.Errorf("local device not found: %s", localDevice)
	}

	if local.State != RDMAStateConnected {
		return nil, fmt.Errorf("local device not connected: %s", localDevice)
	}

	// 创建连接ID
	connID := fmt.Sprintf("%s-%s-%d", localDevice, remoteAddr, remotePort)

	// 检查连接是否已存在
	if _, exists := m.connections[connID]; exists {
		return nil, fmt.Errorf("connection already exists: %s", connID)
	}

	// 创建连接
	conn := &RDMAConnection{
		ID:           connID,
		LocalDevice:  localDevice,
		LocalPort:    1, // 默认端口
		RemoteAddr:   net.ParseIP(remoteAddr),
		RemotePort:   remotePort,
		State:        RDMAStateConnecting,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		QueuePair: &QueuePair{
			State:         0,
			AccessFlags:   0x07, // Local Write, Remote Write, Remote Read
			MaxSendWr:     m.config.MaxSendWr,
			MaxRecvWr:     m.config.MaxRecvWr,
			MaxSge:        m.config.MaxSge,
			MaxInlineData: m.config.MaxInlineData,
		},
	}

	m.connections[connID] = conn
	m.stats.TotalConnections++

	// 模拟连接建立
	go m.establishConnection(conn)

	return conn, nil
}

// Disconnect 断开RDMA连接
func (m *RDMANetworkManager) Disconnect(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.disconnect(connID)
}

// disconnect 内部断开连接
func (m *RDMANetworkManager) disconnect(connID string) error {
	conn, exists := m.connections[connID]
	if !exists {
		return fmt.Errorf("connection not found: %s", connID)
	}

	if conn.State == RDMAStateConnected {
		m.stats.ActiveConnections--
	}

	delete(m.connections, connID)

	return nil
}

// Send 发送数据
func (m *RDMANetworkManager) Send(connID string, data []byte) error {
	m.mu.RLock()
	conn, exists := m.connections[connID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found: %s", connID)
	}

	if conn.State != RDMAStateConnected {
		return fmt.Errorf("connection not established: %s", connID)
	}

	// 更新统计
	m.mu.Lock()
	conn.Stats.SendBytes += int64(len(data))
	conn.Stats.SendPackets++
	conn.LastActivity = time.Now()
	m.stats.TotalSendBytes += int64(len(data))
	m.stats.TotalSendPackets++
	m.mu.Unlock()

	return nil
}

// Receive 接收数据
func (m *RDMANetworkManager) Receive(connID string) ([]byte, error) {
	m.mu.RLock()
	conn, exists := m.connections[connID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("connection not found: %s", connID)
	}

	if conn.State != RDMAStateConnected {
		return nil, fmt.Errorf("connection not established: %s", connID)
	}

	// 模拟接收数据
	data := make([]byte, 1024)

	// 更新统计
	m.mu.Lock()
	conn.Stats.RecvBytes += int64(len(data))
	conn.Stats.RecvPackets++
	conn.LastActivity = time.Now()
	m.stats.TotalRecvBytes += int64(len(data))
	m.stats.TotalRecvPackets++
	m.mu.Unlock()

	return data, nil
}

// GetConnection 获取连接信息
func (m *RDMANetworkManager) GetConnection(connID string) (*RDMAConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[connID]
	if !exists {
		return nil, fmt.Errorf("connection not found: %s", connID)
	}

	return conn, nil
}

// ListConnections 列出所有连接
func (m *RDMANetworkManager) ListConnections() []*RDMAConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connections := make([]*RDMAConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		connections = append(connections, conn)
	}

	return connections
}

// GetDevice 获取设备信息
func (m *RDMANetworkManager) GetDevice(name string) (*RDMADevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[name]
	if !exists {
		return nil, fmt.Errorf("device not found: %s", name)
	}

	return device, nil
}

// ListDevices 列出所有设备
func (m *RDMANetworkManager) ListDevices() []*RDMADevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*RDMADevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}

	return devices
}

// GetStats 获取统计信息
func (m *RDMANetworkManager) GetStats() RDMAStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// establishConnection 建立连接
func (m *RDMANetworkManager) establishConnection(conn *RDMAConnection) {
	// 模拟连接建立过程
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	defer m.mu.Unlock()

	conn.State = RDMAStateConnected
	m.stats.ActiveConnections++
}

// monitorLoop 监控循环
func (m *RDMANetworkManager) monitorLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.monitor()
		}
	}
}

// monitor 监控
func (m *RDMANetworkManager) monitor() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查设备状态
	for _, device := range m.devices {
		if device.State == RDMAStateError {
			// 尝试恢复
			device.State = RDMAStateDisconnected
		}
	}

	// 检查连接状态
	for connID, conn := range m.connections {
		if conn.State == RDMAStateError {
			// 尝试重连
			conn.State = RDMAStateConnecting
			go m.establishConnection(conn)
		}

		// 检查超时
		if time.Since(conn.LastActivity) > m.config.Timeout {
			m.disconnect(connID)
		}
	}
}

// DefaultRDMAConfig 默认RDMA配置
func DefaultRDMAConfig() RDMAConfig {
	return RDMAConfig{
		Type:            RDMATypeRoCE,
		MTU:             4096,
		QueuePairCount:  100,
		MaxInlineData:   256,
		MaxSendWr:       1000,
		MaxRecvWr:       1000,
		MaxSge:          1,
		Timeout:         30 * time.Second,
		RetryCount:      3,
		EnableSRQ:       true,
		EnableAtomic:    true,
		EnableRDMAWrite: true,
		EnableRDMARead:  true,
	}
}
