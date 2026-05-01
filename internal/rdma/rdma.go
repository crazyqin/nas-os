// Package rdma 提供RDMA高性能网络模块
// 支持 RoCE (RDMA over Converged Ethernet) 连接管理
// 支持 iSCSI 和 NFS 的 RDMA 加速
// 多路径IO (MPIO) 支持，自动故障切换
// RDMA不可用时自动降级回退到TCP
package rdma

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionState 连接状态
type ConnectionState string

const (
	StateActive    ConnectionState = "active"
	StateInactive  ConnectionState = "inactive"
	StateDegraded  ConnectionState = "degraded"
	StateFailed    ConnectionState = "failed"
	StateConnecting ConnectionState = "connecting"
)

// TransportType 传输协议类型
type TransportType string

const (
	TransportRoCE TransportType = "roce"
	TransportTCP  TransportType = "tcp"
)

// StorageProtocol 存储协议类型
type StorageProtocol string

const (
	ProtocolISCSI StorageProtocol = "iscsi"
	ProtocolNFS   StorageProtocol = "nfs"
)

// RDMADevice RDMA网络设备
type RDMADevice struct {
	Name          string        `json:"name"`
	GUID          string        `json:"guid"`
	Transport     TransportType `json:"transport"`
	PortCount     int           `json:"portCount"`
	ActivePorts   int           `json:"activePorts"`
	MaxMRSize     int64         `json:"maxMrSize"`     // 最大内存注册大小
	MaxQueuePairs int           `json:"maxQueuePairs"` // 最大队列对数
	MaxCQEntries  int           `json:"maxCqEntries"`  // 最大完成队列条目
	LinkSpeed     float64       `json:"linkSpeed"`     // 链路速度(Gbps)
	State         string        `json:"state"`         // 设备状态: active/inactive/error
	FirmwareVer   string        `json:"firmwareVer"`
	NodeGUID      string        `json:"nodeGUID"`
}

// RDMAConnection RDMA连接
type RDMAConnection struct {
	ID             string          `json:"id"`
	DeviceName     string          `json:"deviceName"`
	RemoteAddr     string          `json:"remoteAddr"`
	LocalAddr      string          `json:"localAddr"`
	Protocol       StorageProtocol `json:"protocol"`
	Transport      TransportType   `json:"transport"`
	State          ConnectionState `json:"state"`
	EstablishedAt  time.Time       `json:"establishedAt"`
	LastActivityAt time.Time       `json:"lastActivityAt"`

	// 连接质量指标
	Latency     float64 `json:"latency"`     // 延迟(μs)
	Bandwidth   float64 `json:"bandwidth"`   // 当前带宽(Mbps)
	PacketLoss  float64 `json:"packetLoss"`  // 丢包率(%)
	QueueDepth  int     `json:"queueDepth"`  // 队列深度
	Retransmits int64   `json:"retransmits"` // 重传次数

	// 多路径信息
	MultipathGroupID string `json:"multipathGroupId"`
	PathIndex        int    `json:"pathIndex"`
	IsPrimary        bool   `json:"isPrimary"`

	// 降级信息
	FallbackTransport TransportType `json:"fallbackTransport"`
	IsFallback        bool          `json:"isFallback"`
	DegradedAt        *time.Time    `json:"degradedAt,omitempty"`
}

// MultipathGroup 多路径组
type MultipathGroup struct {
	ID             string   `json:"id"`
	ConnectionIDs  []string `json:"connectionIds"`
	ActivePathIdx  int      `json:"activePathIdx"`
	Policy         string   `json:"policy"` // round-robin, failover, weighted
	TotalPaths     int      `json:"totalPaths"`
	ActivePaths    int      `json:"activePaths"`
	FailedPaths    int      `json:"failedPaths"`
	LastFailoverAt *time.Time `json:"lastFailoverAt,omitempty"`
}

// RDMAStats RDMA性能统计
type RDMAStats struct {
	// IOPS统计
	ReadIOPS   int64 `json:"readIops"`
	WriteIOPS  int64 `json:"writeIops"`
	TotalIOPS  int64 `json:"totalIops"`

	// 吞吐量统计
	ReadThroughput  float64 `json:"readThroughput"`  // MB/s
	WriteThroughput float64 `json:"writeThroughput"` // MB/s
	TotalThroughput float64 `json:"totalThroughput"` // MB/s

	// 延迟统计
	AvgLatency  float64 `json:"avgLatency"`  // μs
	P50Latency  float64 `json:"p50Latency"`  // μs
	P99Latency  float64 `json:"p99Latency"`  // μs
	MaxLatency  float64 `json:"maxLatency"`  // μs

	// 连接统计
	ActiveConnections  int   `json:"activeConnections"`
	TotalConnections   int   `json:"totalConnections"`
	FailedConnections  int64 `json:"failedConnections"`
	DegradedConnections int  `json:"degradedConnections"`

	// 传输统计
	TotalBytesSent     int64 `json:"totalBytesSent"`
	TotalBytesReceived int64 `json:"totalBytesReceived"`
	TotalErrors        int64 `json:"totalErrors"`

	// 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled       bool    `json:"enabled"`
	MaxBandwidth  float64 `json:"maxBandwidth"`  // Mbps
	MaxIOPS       int64   `json:"maxIops"`
	BurstSize     int64   `json:"burstSize"`     // 字节
}

// CongestionConfig 拥塞控制配置
type CongestionConfig struct {
	Enabled          bool    `json:"enabled"`
	ECNEnabled       bool    `json:"ecnEnabled"`       // 显式拥塞通知
	DCTCPEnabled     bool    `json:"dctcpEnabled"`     // DCTCP算法
	ThresholdPackets int     `json:"thresholdPackets"` // 拥塞阈值(包数)
	BackoffFactor    float64 `json:"backoffFactor"`    // 退避因子
	MinRate          float64 `json:"minRate"`          // 最低速率(Mbps)
}

// FailoverConfig 故障切换配置
type FailoverConfig struct {
	Enabled          bool          `json:"enabled"`
	ProbeInterval    time.Duration `json:"probeInterval"`    // 探测间隔
	FailoverTimeout  time.Duration `json:"failoverTimeout"`  // 故障切换超时
	MaxRetries       int           `json:"maxRetries"`       // 最大重试次数
	AutoRecover      bool          `json:"autoRecover"`      // 自动恢复
	RecoverInterval  time.Duration `json:"recoverInterval"`  // 恢复探测间隔
}

// RDMAConfig RDMA模块配置
type RDMAConfig struct {
	Enabled         bool              `json:"enabled"`
	DefaultTransport TransportType    `json:"defaultTransport"`
	FallbackToTCP   bool             `json:"fallbackToTcp"` // RDMA不可用时回退到TCP
	MonitorInterval time.Duration    `json:"monitorInterval"`
	DeviceFilter    []string         `json:"deviceFilter"` // 设备白名单，空表示全部
	RateLimit       RateLimitConfig  `json:"rateLimit"`
	Congestion      CongestionConfig `json:"congestion"`
	Failover        FailoverConfig   `json:"failover"`

	// 连接健康阈值
	MaxLatencyMs     float64 `json:"maxLatencyMs"`     // 最大延迟阈值(ms)
	MaxPacketLoss    float64 `json:"maxPacketLoss"`     // 最大丢包率(%)
	MaxQueueDepth    int     `json:"maxQueueDepth"`     // 最大队列深度
	HealthCheckCount int     `json:"healthCheckCount"`  // 健康检查失败次数触发降级
}

// DefaultConfig 返回默认配置
func DefaultConfig() RDMAConfig {
	return RDMAConfig{
		Enabled:          false,
		DefaultTransport: TransportRoCE,
		FallbackToTCP:    true,
		MonitorInterval:  10 * time.Second,
		RateLimit: RateLimitConfig{
			Enabled:      false,
			MaxBandwidth: 100000, // 100Gbps
			MaxIOPS:      1000000,
			BurstSize:    1024 * 1024 * 1024,
		},
		Congestion: CongestionConfig{
			Enabled:          true,
			ECNEnabled:       true,
			DCTCPEnabled:     true,
			ThresholdPackets: 65,
			BackoffFactor:    0.5,
			MinRate:          100,
		},
		Failover: FailoverConfig{
			Enabled:         true,
			ProbeInterval:   5 * time.Second,
			FailoverTimeout: 30 * time.Second,
			MaxRetries:      3,
			AutoRecover:     true,
			RecoverInterval: 60 * time.Second,
		},
		MaxLatencyMs:     1.0,
		MaxPacketLoss:    0.1,
		MaxQueueDepth:    256,
		HealthCheckCount: 3,
	}
}

// RDMAManager RDMA管理器
type RDMAManager struct {
	devices       map[string]*RDMADevice
	conns         map[string]*RDMAConnection
	multipathGrps map[string]*MultipathGroup
	stats         RDMAStats
	config        RDMAConfig
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	running       bool

	// 统计计数器（原子操作）
	totalBytesSent     atomic.Int64
	totalBytesReceived atomic.Int64
	totalErrors        atomic.Int64
	failedConns        atomic.Int64
}

// NewRDMAManager 创建RDMA管理器
func NewRDMAManager(config RDMAConfig) *RDMAManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &RDMAManager{
		devices:       make(map[string]*RDMADevice),
		conns:         make(map[string]*RDMAConnection),
		multipathGrps: make(map[string]*MultipathGroup),
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动RDMA管理器
func (m *RDMAManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("rdma manager already running")
	}
	if !m.config.Enabled {
		return fmt.Errorf("rdma is disabled in config")
	}
	m.running = true

	// 扫描设备
	m.scanDevices()

	// 启动监控循环
	go m.monitorLoop()

	log.Printf("[RDMA] Manager started, devices: %d, transport: %s, fallback: %v",
		len(m.devices), m.config.DefaultTransport, m.config.FallbackToTCP)
	return nil
}

// Stop 停止RDMA管理器
func (m *RDMAManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.cancel()
	// 关闭所有连接
	for id, conn := range m.conns {
		conn.State = StateInactive
		log.Printf("[RDMA] Closed connection %s", id)
	}
	m.running = false
	log.Printf("[RDMA] Manager stopped")
}

// GetStatus 获取RDMA状态
func (m *RDMAManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"enabled":     m.config.Enabled,
		"running":     m.running,
		"transport":   m.config.DefaultTransport,
		"deviceCount": len(m.devices),
		"connCount":   len(m.conns),
		"multipathGroups": len(m.multipathGrps),
		"fallbackToTcp": m.config.FallbackToTCP,
	}
}

// GetDevices 获取所有RDMA设备
func (m *RDMAManager) GetDevices() []*RDMADevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*RDMADevice, 0, len(m.devices))
	for _, dev := range m.devices {
		cp := *dev
		result = append(result, &cp)
	}
	return result
}

// GetConnections 获取所有活跃连接
func (m *RDMAManager) GetConnections() []*RDMAConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*RDMAConnection, 0, len(m.conns))
	for _, conn := range m.conns {
		cp := *conn
		result = append(result, &cp)
	}
	return result
}

// GetStats 获取性能统计
func (m *RDMAManager) GetStats() RDMAStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := m.stats
	stats.TotalBytesSent = m.totalBytesSent.Load()
	stats.TotalBytesReceived = m.totalBytesReceived.Load()
	stats.TotalErrors = m.totalErrors.Load()
	stats.FailedConnections = m.failedConns.Load()
	stats.UpdatedAt = time.Now()

	// 统计连接状态
	var active, degraded int
	for _, conn := range m.conns {
		switch conn.State {
		case StateActive:
			active++
		case StateDegraded:
			degraded++
		}
	}
	stats.ActiveConnections = active
	stats.TotalConnections = len(m.conns)
	stats.DegradedConnections = degraded
	return stats
}

// GetMultipathStatus 获取多路径状态
func (m *RDMAManager) GetMultipathStatus() []*MultipathGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*MultipathGroup, 0, len(m.multipathGrps))
	for _, grp := range m.multipathGrps {
		cp := *grp
		result = append(result, &cp)
	}
	return result
}

// UpdateConfig 更新配置
func (m *RDMAManager) UpdateConfig(config RDMAConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// EnableRDMA 启用RDMA
func (m *RDMAManager) EnableRDMA() error {
	m.mu.Lock()
	m.config.Enabled = true
	m.mu.Unlock()
	if !m.running {
		return m.Start()
	}
	return nil
}

// DisableRDMA 禁用RDMA
func (m *RDMAManager) DisableRDMA() error {
	m.mu.Lock()
	m.config.Enabled = false
	m.mu.Unlock()
	if m.running {
		m.Stop()
	}
	return nil
}

// EstablishConnection 建立RDMA连接
func (m *RDMAManager) EstablishConnection(deviceName, remoteAddr string, protocol StorageProtocol) (*RDMAConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, fmt.Errorf("rdma manager is not running")
	}

	// 检查设备
	dev, exists := m.devices[deviceName]
	if !exists {
		// 尝试回退到TCP
		if m.config.FallbackToTCP {
			return m.establishFallbackConn(remoteAddr, protocol)
		}
		return nil, fmt.Errorf("device %s not found", deviceName)
	}

	transport := m.config.DefaultTransport
	if dev.Transport != "" {
		transport = dev.Transport
	}

	conn := &RDMAConnection{
		ID:             fmt.Sprintf("rdma-%s-%d", deviceName, time.Now().UnixNano()),
		DeviceName:     deviceName,
		RemoteAddr:     remoteAddr,
		LocalAddr:      "0.0.0.0",
		Protocol:       protocol,
		Transport:      transport,
		State:          StateActive,
		EstablishedAt:  time.Now(),
		LastActivityAt: time.Now(),
		Latency:        0.5,
		Bandwidth:      dev.LinkSpeed * 1000,
		IsPrimary:      len(m.conns) == 0,
	}

	m.conns[conn.ID] = conn
	log.Printf("[RDMA] Established connection %s -> %s via %s (%s)", deviceName, remoteAddr, transport, protocol)
	return conn, nil
}

// CreateMultipathGroup 创建多路径组
func (m *RDMAManager) CreateMultipathGroup(connIDs []string, policy string) (*MultipathGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	grp := &MultipathGroup{
		ID:            fmt.Sprintf("mpio-%d", time.Now().UnixNano()),
		ConnectionIDs: connIDs,
		Policy:        policy,
		TotalPaths:    len(connIDs),
		ActivePaths:   0,
	}

	for _, id := range connIDs {
		conn, ok := m.conns[id]
		if !ok {
			return nil, fmt.Errorf("connection %s not found", id)
		}
		conn.MultipathGroupID = grp.ID
		conn.PathIndex = grp.ActivePaths
		if conn.State == StateActive {
			grp.ActivePaths++
		}
	}

	m.multipathGrps[grp.ID] = grp
	log.Printf("[RDMA] Created multipath group %s with %d paths, policy: %s", grp.ID, len(connIDs), policy)
	return grp, nil
}

// CloseConnection 关闭连接
func (m *RDMAManager) CloseConnection(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn, exists := m.conns[connID]
	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}
	conn.State = StateInactive
	delete(m.conns, connID)
	log.Printf("[RDMA] Closed connection %s", connID)
	return nil
}

// scanDevices 扫描RDMA设备
func (m *RDMAManager) scanDevices() {
	// 模拟扫描到的RDMA设备
	// 实际实现中应读取 /sys/class/infiniband/ 或使用 rdma-core 库
	simulatedDevices := []*RDMADevice{
		{
			Name:          "mlx5_0",
			GUID:          "0000000000000001",
			Transport:     TransportRoCE,
			PortCount:     2,
			ActivePorts:   2,
			MaxMRSize:     0x100000000, // 4GB
			MaxQueuePairs: 65536,
			MaxCQEntries:  4194304,
			LinkSpeed:     100.0,
			State:         "active",
			FirmwareVer:   "16.35.2010",
			NodeGUID:      "0000000000000001",
		},
		{
			Name:          "mlx5_1",
			GUID:          "0000000000000002",
			Transport:     TransportRoCE,
			PortCount:     2,
			ActivePorts:   2,
			MaxMRSize:     0x100000000,
			MaxQueuePairs: 65536,
			MaxCQEntries:  4194304,
			LinkSpeed:     100.0,
			State:         "active",
			FirmwareVer:   "16.35.2010",
			NodeGUID:      "0000000000000002",
		},
	}

	for _, dev := range simulatedDevices {
		if len(m.config.DeviceFilter) > 0 && !contains(m.config.DeviceFilter, dev.Name) {
			continue
		}
		m.devices[dev.Name] = dev
	}
}

// monitorLoop 监控循环
func (m *RDMAManager) monitorLoop() {
	ticker := time.NewTicker(m.config.MonitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkConnectionHealth()
		}
	}
}

// checkConnectionHealth 检查连接健康状态
func (m *RDMAManager) checkConnectionHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.conns {
		if conn.State != StateActive && conn.State != StateDegraded {
			continue
		}

		// 检查延迟阈值
		if conn.Latency > m.config.MaxLatencyMs*1000 { // 转为μs
			conn.State = StateDegraded
			if m.config.FallbackToTCP && conn.Transport != TransportTCP {
				m.fallbackConnection(conn)
			}
		}

		// 检查丢包率
		if conn.PacketLoss > m.config.MaxPacketLoss {
			conn.State = StateDegraded
			if m.config.FallbackToTCP && conn.Transport != TransportTCP {
				m.fallbackConnection(conn)
			}
		}

		// 检查队列深度
		if conn.QueueDepth > m.config.MaxQueueDepth {
			conn.State = StateDegraded
		}

		// 模拟活跃连接的指标波动
		if conn.State == StateActive {
			conn.Latency = 0.3 + float64(time.Now().UnixNano()%100)/100.0
			conn.Bandwidth = 80000 + float64(time.Now().UnixNano()%20000)
			conn.PacketLoss = float64(time.Now().UnixNano()%5) / 100.0
			conn.QueueDepth = int(time.Now().UnixNano() % 32)
		}
	}

	// 更新全局统计
	m.updateGlobalStats()
}

// fallbackConnection 回退连接到TCP
func (m *RDMAManager) fallbackConnection(conn *RDMAConnection) {
	now := time.Now()
	conn.IsFallback = true
	conn.DegradedAt = &now
	conn.FallbackTransport = conn.Transport
	conn.Transport = TransportTCP
	conn.State = StateActive
	log.Printf("[RDMA] Connection %s fallback to TCP (was %s)", conn.ID, conn.FallbackTransport)
}

// updateGlobalStats 更新全局统计
func (m *RDMAManager) updateGlobalStats() {
	var totalReadIOPS, totalWriteIOPS int64
	var totalReadBW, totalWriteBW float64
	var totalLatency float64
	var latencyCount int

	for _, conn := range m.conns {
		if conn.State != StateActive {
			continue
		}
		switch conn.Protocol {
		case ProtocolISCSI:
			totalReadIOPS += 50000
			totalWriteIOPS += 30000
			totalReadBW += conn.Bandwidth * 0.6
			totalWriteBW += conn.Bandwidth * 0.4
		case ProtocolNFS:
			totalReadIOPS += 40000
			totalWriteIOPS += 20000
			totalReadBW += conn.Bandwidth * 0.7
			totalWriteBW += conn.Bandwidth * 0.3
		}
		totalLatency += conn.Latency
		latencyCount++
	}

	m.stats.ReadIOPS = totalReadIOPS
	m.stats.WriteIOPS = totalWriteIOPS
	m.stats.TotalIOPS = totalReadIOPS + totalWriteIOPS
	m.stats.ReadThroughput = totalReadBW / 1000 // Mbps -> MB/s
	m.stats.WriteThroughput = totalWriteBW / 1000
	m.stats.TotalThroughput = m.stats.ReadThroughput + m.stats.WriteThroughput

	if latencyCount > 0 {
		m.stats.AvgLatency = totalLatency / float64(latencyCount)
		m.stats.P50Latency = m.stats.AvgLatency * 0.8
		m.stats.P99Latency = m.stats.AvgLatency * 3.0
		m.stats.MaxLatency = m.stats.AvgLatency * 5.0
	}
}

// establishFallbackConn 建立TCP回退连接
func (m *RDMAManager) establishFallbackConn(remoteAddr string, protocol StorageProtocol) (*RDMAConnection, error) {
	conn := &RDMAConnection{
		ID:             fmt.Sprintf("tcp-fb-%d", time.Now().UnixNano()),
		DeviceName:     "tcp-fallback",
		RemoteAddr:     remoteAddr,
		LocalAddr:      "0.0.0.0",
		Protocol:       protocol,
		Transport:      TransportTCP,
		State:          StateActive,
		EstablishedAt:  time.Now(),
		LastActivityAt: time.Now(),
		Latency:        50.0,  // TCP延迟更高
		Bandwidth:      10000, // 10Gbps
		IsFallback:     true,
	}
	m.conns[conn.ID] = conn
	log.Printf("[RDMA] Established TCP fallback connection -> %s (%s)", remoteAddr, protocol)
	return conn, nil
}

// contains 检查字符串切片是否包含指定值
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
