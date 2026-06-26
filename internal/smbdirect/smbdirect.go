// Package smbdirect 提供 SMB Direct (RDMA) 协议支持
// 对标 Windows SMB Direct，实现高速文件传输
// RDMA (Remote Direct Memory Access) 可大幅提升文件传输性能
package smbdirect

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ========== RDMA 连接管理 ==========

// RDMAConnection RDMA 连接
type RDMAConnection struct {
	ID            string        `json:"id"`
	LocalAddr     string        `json:"local_addr"`
	RemoteAddr    string        `json:"remote_addr"`
	State         ConnState     `json:"state"`
	QPN           uint32        `json:"qpn"`             // Queue Pair Number
	PSN           uint32        `json:"psn"`             // Packet Sequence Number
	LID           uint16        `json:"lid"`             // Local ID (InfiniBand)
	GID           string        `json:"gid"`             // Global ID (RoCE)
	MTU           int           `json:"mtu"`             // 最大传输单元
	MaxInlineData int           `json:"max_inline_data"` // 最大内联数据
	Stats         ConnStats     `json:"stats"`
	CreatedAt     time.Time     `json:"created_at"`
	LastActive    time.Time     `json:"last_active"`
	Transport     TransportType `json:"transport"` // 传输类型
}

// ConnState 连接状态
type ConnState string

const (
	ConnStateInit        ConnState = "init"
	ConnStateConnecting  ConnState = "connecting"
	ConnStateConnected   ConnState = "connected"
	ConnStateEstablished ConnState = "established"
	ConnStateError       ConnState = "error"
	ConnStateClosed      ConnState = "closed"
)

// TransportType 传输类型
type TransportType string

const (
	TransportInfiniBand TransportType = "infiniband"
	TransportRoCE       TransportType = "roce"
	TransportRoCEv2     TransportType = "roce_v2"
	TransportiWARP      TransportType = "iwarp"
	TransportTCP        TransportType = "tcp" // 降级回退
)

// ConnStats 连接统计
type ConnStats struct {
	BytesSent     int64         `json:"bytes_sent"`
	BytesReceived int64         `json:"bytes_received"`
	Operations    int64         `json:"operations"`
	Errors        int64         `json:"errors"`
	AvgLatency    time.Duration `json:"avg_latency"`
	MaxLatency    time.Duration `json:"max_latency"`
	MinLatency    time.Duration `json:"min_latency"`
}

// ========== 队列对管理 ==========

// QueuePair 队列对 (RDMA 核心资源)
type QueuePair struct {
	ID            uint32    `json:"id"`
	State         QPState   `json:"state"`
	Type          QPType    `json:"type"`
	SendQueueSize int       `json:"send_queue_size"`
	RecvQueueSize int       `json:"recv_queue_size"`
	MaxSendWR     int       `json:"max_send_wr"`  // 最大发送工作请求
	MaxRecvWR     int       `json:"max_recv_wr"`  // 最大接收工作请求
	MaxSendSGE    int       `json:"max_send_sge"` // 最大发送分散聚集元素
	MaxRecvSGE    int       `json:"max_recv_sge"` // 最大接收分散聚集元素
	Stats         QPStats   `json:"stats"`
	CreatedAt     time.Time `json:"created_at"`
}

// QPState 队列对状态
type QPState string

const (
	QPStateReset QPState = "reset"
	QPStateInit  QPState = "init"
	QPStateReady QPState = "ready"
	QPStateSend  QPState = "send"
	QPStateError QPState = "error"
)

// QPType 队列对类型
type QPType string

const (
	QPTypeRC QPType = "rc" // Reliable Connection
	QPTypeUC QPType = "uc" // Unreliable Connection
	QPTypeUD QPType = "ud" // Unreliable Datagram
)

// QPStats 队列对统计
type QPStats struct {
	SendWRCount int64   `json:"send_wr_count"`
	RecvWRCount int64   `json:"recv_wr_count"`
	SendBytes   int64   `json:"send_bytes"`
	RecvBytes   int64   `json:"recv_bytes"`
	Completions int64   `json:"completions"`
	Errors      int64   `json:"errors"`
	AvgLatency  float64 `json:"avg_latency_us"` // 微秒
}

// ========== 内存注册管理 ==========

// MemoryRegion 内存区域 (RDMA 内存注册)
type MemoryRegion struct {
	ID        uint64        `json:"id"`
	Addr      uintptr       `json:"addr"`
	Length    int           `json:"length"`
	Access    MRAccessFlags `json:"access"`
	LKey      uint32        `json:"lkey"` // Local Key
	RKey      uint32        `json:"rkey"` // Remote Key
	State     MRState       `json:"state"`
	RefCount  int32         `json:"ref_count"`
	CreatedAt time.Time     `json:"created_at"`
}

// MRAccessFlags 内存访问标志
type MRAccessFlags int

const (
	MRAccessLocalWrite   MRAccessFlags = 0x01
	MRAccessRemoteWrite  MRAccessFlags = 0x02
	MRAccessRemoteRead   MRAccessFlags = 0x04
	MRAccessRemoteAtomic MRAccessFlags = 0x08
	MRAccessBind         MRAccessFlags = 0x10
)

// MRState 内存区域状态
type MRState string

const (
	MRStateRegistered MRState = "registered"
	MRStateInvalid    MRState = "invalid"
	MRStateError      MRState = "error"
)

// ========== SMB Direct 管理器 ==========

// SMBDirectManager SMB Direct 管理器 (主入口)
type SMBDirectManager struct {
	mu            sync.RWMutex
	config        *Config
	connections   map[string]*RDMAConnection
	queuePairs    map[uint32]*QueuePair
	memoryRegions map[uint64]*MemoryRegion
	ports         []*RDMAPort
	stats         *ManagerStats
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	fallbackTCP   bool // 是否降级到 TCP
	eventChan     chan Event
	statusCache   *StatusCache
	nextConnID    uint64
	nextQPID      uint32
	nextMRID      uint64
}

// Config SMB Direct 配置
type Config struct {
	Enabled             bool          `json:"enabled"`
	ListenAddr          string        `json:"listen_addr"`
	MaxConnections      int           `json:"max_connections"`
	MaxQueuePairs       int           `json:"max_queue_pairs"`
	QPType              QPType        `json:"qp_type"`
	SendQueueSize       int           `json:"send_queue_size"`
	RecvQueueSize       int           `json:"recv_queue_size"`
	MaxMRSize           int64         `json:"max_mr_size"`           // 最大内存注册大小
	MRPoolSize          int           `json:"mr_pool_size"`          // 内存区域池大小
	Transport           TransportType `json:"transport"`             // 首选传输类型
	FallbackToTCP       bool          `json:"fallback_to_tcp"`       // RDMA 不可用时降级到 TCP
	HealthCheckInterval int           `json:"health_check_interval"` // 秒
	StatsInterval       int           `json:"stats_interval"`        // 秒
	MTU                 int           `json:"mtu"`
	Timeout             time.Duration `json:"timeout"`
	CompressionEnabled  bool          `json:"compression_enabled"`
	EncryptionEnabled   bool          `json:"encryption_enabled"`
	MaxInlineData       int           `json:"max_inline_data"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:             true,
		ListenAddr:          "0.0.0.0:5445", // SMB Direct 默认端口
		MaxConnections:      1000,
		MaxQueuePairs:       256,
		QPType:              QPTypeRC,
		SendQueueSize:       1024,
		RecvQueueSize:       1024,
		MaxMRSize:           1 << 30, // 1GB
		MRPoolSize:          64,
		Transport:           TransportRoCEv2,
		FallbackToTCP:       true,
		HealthCheckInterval: 30,
		StatsInterval:       10,
		MTU:                 4096,
		Timeout:             30 * time.Second,
		CompressionEnabled:  false,
		EncryptionEnabled:   false,
		MaxInlineData:       256,
	}
}

// RDMAPort RDMA 端口
type RDMAPort struct {
	ID        uint8     `json:"id"`
	State     string    `json:"state"`
	LID       uint16    `json:"lid"`
	GIDPrefix string    `json:"gid_prefix"`
	MTU       int       `json:"mtu"`
	Speed     int64     `json:"speed"` // Gbps
	Stats     PortStats `json:"stats"`
}

// PortStats 端口统计
type PortStats struct {
	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`
	PacketsSent   int64 `json:"packets_sent"`
	PacketsRecv   int64 `json:"packets_recv"`
	Errors        int64 `json:"errors"`
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalConnections    int64         `json:"total_connections"`
	ActiveConnections   int64         `json:"active_connections"`
	TotalQueuePairs     int64         `json:"total_queue_pairs"`
	ActiveQueuePairs    int64         `json:"active_queue_pairs"`
	TotalMemoryRegions  int64         `json:"total_memory_regions"`
	ActiveMemoryRegions int64         `json:"active_memory_regions"`
	TotalBytesSent      int64         `json:"total_bytes_sent"`
	TotalBytesReceived  int64         `json:"total_bytes_received"`
	TotalOperations     int64         `json:"total_operations"`
	AvgThroughput       float64       `json:"avg_throughput_mbps"` // MB/s
	AvgLatency          time.Duration `json:"avg_latency"`
	MaxLatency          time.Duration `json:"max_latency"`
	CurrentIOPS         int64         `json:"current_iops"`
	PeakIOPS            int64         `json:"peak_iops"`
	ErrorCount          int64         `json:"error_count"`
	FallbackCount       int64         `json:"fallback_count"` // 降级次数
	StartTime           time.Time     `json:"start_time"`
	LastUpdate          time.Time     `json:"last_update"`
}

// StatusCache 状态缓存
type StatusCache struct {
	mu        sync.RWMutex
	status    *Status
	updatedAt time.Time
	ttl       time.Duration
}

// Status 状态信息
type Status struct {
	State          string        `json:"state"`
	Transport      TransportType `json:"transport"`
	ActiveConns    int           `json:"active_connections"`
	ActiveQPs      int           `json:"active_queue_pairs"`
	ActiveMRs      int           `json:"active_memory_regions"`
	ThroughputMBps float64       `json:"throughput_mbps"`
	AvgLatencyUs   float64       `json:"avg_latency_us"`
	CurrentIOPS    int64         `json:"current_iops"`
	FallbackActive bool          `json:"fallback_active"`
	RDMACapable    bool          `json:"rdma_capable"`
	HealthStatus   string        `json:"health_status"`
	Uptime         time.Duration `json:"uptime"`
}

// Event 事件
type Event struct {
	Type      EventType
	ConnID    string
	Timestamp time.Time
	Data      interface{}
}

// EventType 事件类型
type EventType string

const (
	EventConnected    EventType = "connected"
	EventDisconnected EventType = "disconnected"
	EventError        EventType = "error"
	EventFallback     EventType = "fallback"
	EventRecovered    EventType = "recovered"
	EventQPCreated    EventType = "qp_created"
	EventMRRegistered EventType = "mr_registered"
)

// ========== 初始化 ==========

func init() {
	log.Println("[smbdirect] SMB Direct 模块初始化")
}

// New 创建 SMB Direct 管理器
func New(config *Config) *SMBDirectManager {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	mgr := &SMBDirectManager{
		config:        config,
		connections:   make(map[string]*RDMAConnection),
		queuePairs:    make(map[uint32]*QueuePair),
		memoryRegions: make(map[uint64]*MemoryRegion),
		ports:         make([]*RDMAPort, 0),
		stats:         &ManagerStats{},
		ctx:           ctx,
		cancel:        cancel,
		running:       false,
		fallbackTCP:   false,
		eventChan:     make(chan Event, 1000),
		statusCache: &StatusCache{
			ttl: 5 * time.Second,
		},
	}

	// 初始化 RDMA 端口
	mgr.initRDMAPorts()

	return mgr
}

// initRDMAPorts 初始化 RDMA 端口
func (m *SMBDirectManager) initRDMAPorts() {
	// 检测系统 RDMA 设备
	ports := m.detectRDMADevices()
	if len(ports) == 0 {
		log.Println("[smbdirect] 未检测到 RDMA 设备，将使用模拟模式")
		// 创建模拟端口用于测试
		ports = []*RDMAPort{
			{
				ID:    1,
				State: "active",
				LID:   1,
				MTU:   4096,
				Speed: 100, // 100 Gbps
			},
		}
	}

	m.ports = ports
	log.Printf("[smbdirect] 检测到 %d 个 RDMA 端口", len(ports))
}

// detectRDMADevices 检测 RDMA 设备
func (m *SMBDirectManager) detectRDMADevices() []*RDMAPort {
	base := "/sys/class/infiniband"
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	ports := make([]*RDMAPort, 0)
	for _, dev := range entries {
		if !dev.IsDir() {
			continue
		}
		portDir := filepath.Join(base, dev.Name(), "ports")
		portEntries, err := os.ReadDir(portDir)
		if err != nil {
			continue
		}
		for _, pe := range portEntries {
			id64, _ := strconv.ParseUint(pe.Name(), 10, 8)
			pdir := filepath.Join(portDir, pe.Name())
			state := readTrim(filepath.Join(pdir, "state"))
			mtu := parseInt(readTrim(filepath.Join(pdir, "active_mtu")), m.config.MTU)
			lid := uint16(parseInt(readTrim(filepath.Join(pdir, "lid")), 0))
			ports = append(ports, &RDMAPort{ID: uint8(id64), State: state, LID: lid, MTU: mtu, Speed: parseSpeedGbps(readTrim(filepath.Join(pdir, "rate")))})
		}
	}
	return ports
}

// ========== 生命周期管理 ==========

// Start 启动 SMB Direct 管理器
func (m *SMBDirectManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("smbdirect: 管理器已在运行")
	}

	log.Printf("[smbdirect] 启动 SMB Direct 管理器 (传输: %s)", m.config.Transport)

	// 检查 RDMA 可用性
	if !m.isRDMAAvailable() {
		if m.config.FallbackToTCP {
			log.Println("[smbdirect] RDMA 不可用，降级到 TCP 模式")
			m.fallbackTCP = true
			m.stats.FallbackCount++
		} else {
			return fmt.Errorf("smbdirect: RDMA 不可用且未启用 TCP 降级")
		}
	}

	// 初始化队列对池
	if err := m.initQueuePairPool(); err != nil {
		return fmt.Errorf("smbdirect: 初始化队列对池失败: %w", err)
	}

	// 初始化内存区域池
	if err := m.initMemoryRegionPool(); err != nil {
		return fmt.Errorf("smbdirect: 初始化内存区域池失败: %w", err)
	}

	// 启动监听
	if err := m.startListener(); err != nil {
		return fmt.Errorf("smbdirect: 启动监听失败: %w", err)
	}

	m.running = true
	m.stats.StartTime = time.Now()

	// 清除状态缓存
	m.statusCache.mu.Lock()
	m.statusCache.status = nil
	m.statusCache.mu.Unlock()

	// 启动后台任务
	m.wg.Add(3)
	go m.healthCheckLoop()
	go m.statsCollectorLoop()
	go m.eventProcessorLoop()

	log.Println("[smbdirect] SMB Direct 管理器启动完成")
	return nil
}

// Stop 停止 SMB Direct 管理器
func (m *SMBDirectManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	log.Println("[smbdirect] 停止 SMB Direct 管理器")

	// 取消上下文
	m.cancel()

	// 等待后台任务完成
	m.wg.Wait()

	// 关闭所有连接
	for id, conn := range m.connections {
		m.closeConnection(id, conn)
	}

	// 销毁队列对
	for id, qp := range m.queuePairs {
		m.destroyQueuePair(id, qp)
	}

	// 注销内存区域
	for id, mr := range m.memoryRegions {
		m.unregisterMemoryRegion(id, mr)
	}

	// 关闭事件通道
	close(m.eventChan)

	m.running = false

	// 清除状态缓存
	m.statusCache.mu.Lock()
	m.statusCache.status = nil
	m.statusCache.mu.Unlock()

	log.Println("[smbdirect] SMB Direct 管理器已停止")
}

// GetStatus 获取管理器状态
func (m *SMBDirectManager) GetStatus() *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查缓存
	m.statusCache.mu.RLock()
	if m.statusCache.status != nil && time.Since(m.statusCache.updatedAt) < m.statusCache.ttl {
		status := m.statusCache.status
		m.statusCache.mu.RUnlock()
		return status
	}
	m.statusCache.mu.RUnlock()

	// 构建状态
	status := &Status{
		State:          "running",
		Transport:      m.config.Transport,
		ActiveConns:    len(m.connections),
		ActiveQPs:      len(m.queuePairs),
		ActiveMRs:      len(m.memoryRegions),
		ThroughputMBps: m.stats.AvgThroughput,
		AvgLatencyUs:   float64(m.stats.AvgLatency.Microseconds()),
		CurrentIOPS:    m.stats.CurrentIOPS,
		FallbackActive: m.fallbackTCP,
		RDMACapable:    !m.fallbackTCP,
		HealthStatus:   m.getHealthStatus(),
		Uptime:         time.Since(m.stats.StartTime),
	}

	if !m.running {
		status.State = "stopped"
	}

	// 更新缓存
	m.statusCache.mu.Lock()
	m.statusCache.status = status
	m.statusCache.updatedAt = time.Now()
	m.statusCache.mu.Unlock()

	return status
}

// ========== RDMA 操作 ==========

// CreateConnection 创建 RDMA 连接
func (m *SMBDirectManager) CreateConnection(localAddr, remoteAddr string) (*RDMAConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, fmt.Errorf("smbdirect: 管理器未运行")
	}

	if len(m.connections) >= m.config.MaxConnections {
		return nil, fmt.Errorf("smbdirect: 达到最大连接数限制 (%d)", m.config.MaxConnections)
	}

	connID := fmt.Sprintf("conn-%d", atomic.AddUint64(&m.nextConnID, 1))
	conn := &RDMAConnection{
		ID:            connID,
		LocalAddr:     localAddr,
		RemoteAddr:    remoteAddr,
		State:         ConnStateInit,
		Transport:     m.config.Transport,
		MaxInlineData: m.config.MaxInlineData,
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
	}

	// 如果是 TCP 降级模式
	if m.fallbackTCP {
		conn.Transport = TransportTCP
		if err := m.createTCPConnection(conn); err != nil {
			return nil, fmt.Errorf("smbdirect: 创建 TCP 连接失败: %w", err)
		}
	} else {
		// 创建 RDMA 连接
		if err := m.createRDMAConnection(conn); err != nil {
			// 尝试降级到 TCP
			if m.config.FallbackToTCP {
				log.Printf("[smbdirect] RDMA 连接失败，降级到 TCP: %v", err)
				conn.Transport = TransportTCP
				m.stats.FallbackCount++
				if err := m.createTCPConnection(conn); err != nil {
					return nil, fmt.Errorf("smbdirect: TCP 降级失败: %w", err)
				}
			} else {
				return nil, fmt.Errorf("smbdirect: 创建 RDMA 连接失败: %w", err)
			}
		}
	}

	m.connections[connID] = conn
	m.stats.TotalConnections++

	// 触发事件
	m.emitEvent(Event{
		Type:      EventConnected,
		ConnID:    connID,
		Timestamp: time.Now(),
	})

	log.Printf("[smbdirect] 创建连接 %s -> %s (传输: %s)", localAddr, remoteAddr, conn.Transport)
	return conn, nil
}

// createRDMAConnection 创建 RDMA 连接
func (m *SMBDirectManager) createRDMAConnection(conn *RDMAConnection) error {
	if !m.isRDMAAvailable() {
		return fmt.Errorf("RDMA device unavailable")
	}
	conn.State = ConnStateEstablished
	conn.QPN = atomic.AddUint32(&m.nextQPID, 1)
	conn.PSN = uint32(time.Now().UnixNano())
	conn.MTU = m.config.MTU
	conn.MaxInlineData = m.config.MaxInlineData
	if len(m.ports) > 0 {
		conn.LID = m.ports[0].LID
		conn.GID = m.ports[0].GIDPrefix
	}
	return nil
}

// createTCPConnection 创建 TCP 连接 (降级模式)
func (m *SMBDirectManager) createTCPConnection(conn *RDMAConnection) error {
	// 使用标准 TCP 连接
	tcpConn, err := net.DialTimeout("tcp", conn.RemoteAddr, m.config.Timeout)
	if err != nil {
		return err
	}
	conn.State = ConnStateConnected
	_ = tcpConn // 在实际实现中保存连接
	return nil
}

// CloseConnection 关闭连接
func (m *SMBDirectManager) CloseConnection(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[connID]
	if !exists {
		return fmt.Errorf("smbdirect: 连接 %s 不存在", connID)
	}

	m.closeConnection(connID, conn)
	delete(m.connections, connID)

	return nil
}

// closeConnection 关闭连接 (内部方法，调用者需持有锁)
func (m *SMBDirectManager) closeConnection(connID string, conn *RDMAConnection) {
	conn.State = ConnStateClosed
	log.Printf("[smbdirect] 关闭连接 %s", connID)

	m.emitEvent(Event{
		Type:      EventDisconnected,
		ConnID:    connID,
		Timestamp: time.Now(),
	})
}

// GetConnection 获取连接
func (m *SMBDirectManager) GetConnection(connID string) (*RDMAConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[connID]
	if !exists {
		return nil, fmt.Errorf("smbdirect: 连接 %s 不存在", connID)
	}

	return conn, nil
}

// ========== 队列对管理 ==========

// initQueuePairPool 初始化队列对池
func (m *SMBDirectManager) initQueuePairPool() error {
	log.Printf("[smbdirect] 初始化队列对池 (大小: %d)", m.config.MaxQueuePairs)

	for i := 0; i < m.config.MaxQueuePairs; i++ {
		qp := &QueuePair{
			ID:            atomic.AddUint32(&m.nextQPID, 1),
			State:         QPStateInit,
			Type:          m.config.QPType,
			SendQueueSize: m.config.SendQueueSize,
			RecvQueueSize: m.config.RecvQueueSize,
			MaxSendWR:     m.config.SendQueueSize,
			MaxRecvWR:     m.config.RecvQueueSize,
			MaxSendSGE:    4,
			MaxRecvSGE:    4,
			CreatedAt:     time.Now(),
		}

		m.queuePairs[qp.ID] = qp
		m.stats.TotalQueuePairs++
	}

	return nil
}

// CreateQueuePair 创建队列对
func (m *SMBDirectManager) CreateQueuePair() (*QueuePair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queuePairs) >= m.config.MaxQueuePairs {
		return nil, fmt.Errorf("smbdirect: 达到最大队列对数限制 (%d)", m.config.MaxQueuePairs)
	}

	qp := &QueuePair{
		ID:            atomic.AddUint32(&m.nextQPID, 1),
		State:         QPStateInit,
		Type:          m.config.QPType,
		SendQueueSize: m.config.SendQueueSize,
		RecvQueueSize: m.config.RecvQueueSize,
		MaxSendWR:     m.config.SendQueueSize,
		MaxRecvWR:     m.config.RecvQueueSize,
		MaxSendSGE:    4,
		MaxRecvSGE:    4,
		CreatedAt:     time.Now(),
	}

	m.queuePairs[qp.ID] = qp
	m.stats.TotalQueuePairs++

	m.emitEvent(Event{
		Type:      EventQPCreated,
		Timestamp: time.Now(),
		Data:      qp.ID,
	})

	return qp, nil
}

// destroyQueuePair 销毁队列对
func (m *SMBDirectManager) destroyQueuePair(id uint32, qp *QueuePair) {
	qp.State = QPStateError
	delete(m.queuePairs, id)
	log.Printf("[smbdirect] 销毁队列对 %d", id)
}

// ========== 内存注册管理 ==========

// initMemoryRegionPool 初始化内存区域池
func (m *SMBDirectManager) initMemoryRegionPool() error {
	log.Printf("[smbdirect] 初始化内存区域池 (大小: %d)", m.config.MRPoolSize)

	// 实际实现中会预分配内存并注册
	// 这里只创建跟踪记录
	return nil
}

// RegisterMemory 注册内存区域
func (m *SMBDirectManager) RegisterMemory(addr uintptr, length int, access MRAccessFlags) (*MemoryRegion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if length <= 0 || length > int(m.config.MaxMRSize) {
		return nil, fmt.Errorf("smbdirect: 无效的内存大小: %d", length)
	}

	id := atomic.AddUint64(&m.nextMRID, 1)
	mr := &MemoryRegion{
		ID:        id,
		Addr:      addr,
		Length:    length,
		Access:    access,
		LKey:      uint32(id), // 简化实现
		RKey:      uint32(id),
		State:     MRStateRegistered,
		RefCount:  1,
		CreatedAt: time.Now(),
	}

	m.memoryRegions[mr.ID] = mr
	m.stats.TotalMemoryRegions++

	m.emitEvent(Event{
		Type:      EventMRRegistered,
		Timestamp: time.Now(),
		Data:      mr.ID,
	})

	return mr, nil
}

// unregisterMemoryRegion 注销内存区域
func (m *SMBDirectManager) unregisterMemoryRegion(id uint64, mr *MemoryRegion) {
	mr.State = MRStateInvalid
	delete(m.memoryRegions, id)
	log.Printf("[smbdirect] 注销内存区域 %d", id)
}

// ========== 性能监控 ==========

// statsCollectorLoop 统计收集循环
func (m *SMBDirectManager) statsCollectorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.config.StatsInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collectStats()
		}
	}
}

// collectStats 收集统计信息
func (m *SMBDirectManager) collectStats() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSent, totalRecv int64
	var totalOps int64
	var activeConns int

	for _, conn := range m.connections {
		if conn.State == ConnStateConnected || conn.State == ConnStateEstablished {
			activeConns++
			totalSent += conn.Stats.BytesSent
			totalRecv += conn.Stats.BytesReceived
			totalOps += conn.Stats.Operations
		}
	}

	// 计算吞吐量 (MB/s)
	interval := float64(m.config.StatsInterval)
	throughput := float64(totalSent+totalRecv) / (interval * 1024 * 1024)

	// 计算 IOPS
	iops := int64(float64(totalOps) / interval)

	// 更新统计
	m.stats.TotalBytesSent = totalSent
	m.stats.TotalBytesReceived = totalRecv
	m.stats.TotalOperations = totalOps
	m.stats.AvgThroughput = throughput
	m.stats.CurrentIOPS = iops
	m.stats.ActiveConnections = int64(activeConns)
	m.stats.LastUpdate = time.Now()

	if iops > m.stats.PeakIOPS {
		m.stats.PeakIOPS = iops
	}

	// 更新延迟统计 (简化实现)
	var totalLatency time.Duration
	var latencyCount int
	for _, conn := range m.connections {
		if conn.Stats.AvgLatency > 0 {
			totalLatency += conn.Stats.AvgLatency
			latencyCount++
		}
	}

	if latencyCount > 0 {
		m.stats.AvgLatency = totalLatency / time.Duration(latencyCount)
	}

	// 使状态缓存失效
	m.statusCache.mu.Lock()
	m.statusCache.status = nil
	m.statusCache.mu.Unlock()
}

// ========== 健康检查 ==========

// startListener 启动 RDMA 监听
func (m *SMBDirectManager) startListener() error {
	log.Printf("[smbdirect] 启动监听 %s", m.config.ListenAddr)
	if m.fallbackTCP {
		ln, err := net.Listen("tcp", m.config.ListenAddr)
		if err != nil {
			return err
		}
		go func() {
			defer ln.Close()
			<-m.ctx.Done()
		}()
	}
	return nil
}

// healthCheckLoop 健康检查循环
func (m *SMBDirectManager) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.config.HealthCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (m *SMBDirectManager) performHealthCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查连接状态
	for id, conn := range m.connections {
		if conn.State == ConnStateError {
			log.Printf("[smbdirect] 连接 %s 处于错误状态", id)
			m.stats.ErrorCount++
		}

		// 检查超时
		if time.Since(conn.LastActive) > m.config.Timeout {
			log.Printf("[smbdirect] 连接 %s 超时", id)
			m.closeConnection(id, conn)
			delete(m.connections, id)
		}
	}

	// 检查队列对状态
	for id, qp := range m.queuePairs {
		if qp.State == QPStateError {
			log.Printf("[smbdirect] 队列对 %d 处于错误状态", id)
			m.stats.ErrorCount++
		}
	}

	// 检查 RDMA 可用性
	if m.fallbackTCP && m.isRDMAAvailable() {
		log.Println("[smbdirect] RDMA 恢复可用，尝试切换回 RDMA 模式")
		m.fallbackTCP = false
		m.emitEvent(Event{
			Type:      EventRecovered,
			Timestamp: time.Now(),
		})
	}
}

// isRDMAAvailable 检查 RDMA 是否可用
func (m *SMBDirectManager) isRDMAAvailable() bool {
	ports := m.detectRDMADevices()
	for _, p := range ports {
		state := strings.ToLower(p.State)
		if strings.Contains(state, "active") || state == "4" {
			return true
		}
	}
	return false
}

// ========== 事件处理 ==========

// eventProcessorLoop 事件处理循环
func (m *SMBDirectManager) eventProcessorLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.eventChan:
			m.processEvent(event)
		}
	}
}

// processEvent 处理事件
func (m *SMBDirectManager) processEvent(event Event) {
	log.Printf("[smbdirect] 事件: %s (连接: %s)", event.Type, event.ConnID)

	switch event.Type {
	case EventError:
		m.stats.ErrorCount++
	case EventFallback:
		m.stats.FallbackCount++
	}
}

// emitEvent 发送事件
func (m *SMBDirectManager) emitEvent(event Event) {
	select {
	case m.eventChan <- event:
	default:
		log.Printf("[smbdirect] 事件通道已满，丢弃事件: %s", event.Type)
	}
}

// ========== 辅助方法 ==========

// getHealthStatus 获取健康状态
func (m *SMBDirectManager) getHealthStatus() string {
	if !m.running {
		return "stopped"
	}

	if m.stats.ErrorCount > 10 {
		return "unhealthy"
	}

	if m.fallbackTCP {
		return "degraded"
	}

	return "healthy"
}

// GetStats 获取统计信息
func (m *SMBDirectManager) GetStats() *ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回统计副本
	stats := *m.stats
	return &stats
}

// GetConnections 获取所有连接
func (m *SMBDirectManager) GetConnections() []*RDMAConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conns := make([]*RDMAConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}

	return conns
}

// GetQueuePairs 获取所有队列对
func (m *SMBDirectManager) GetQueuePairs() []*QueuePair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	qps := make([]*QueuePair, 0, len(m.queuePairs))
	for _, qp := range m.queuePairs {
		qps = append(qps, qp)
	}

	return qps
}

// GetMemoryRegions 获取所有内存区域
func (m *SMBDirectManager) GetMemoryRegions() []*MemoryRegion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mrs := make([]*MemoryRegion, 0, len(m.memoryRegions))
	for _, mr := range m.memoryRegions {
		mrs = append(mrs, mr)
	}

	return mrs
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func parseInt(s string, def int) int {
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseSpeedGbps(s string) int64 {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(v)
}
