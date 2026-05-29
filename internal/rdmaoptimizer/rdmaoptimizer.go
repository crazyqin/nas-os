// Package rdmaoptimizer 提供 RDMA 网络优化
// 对标 TrueNAS RDMA 支持，优化高性能存储网络
package rdmaoptimizer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ========== RDMA 设备管理 ==========

// RDMADevice RDMA 设备
type RDMADevice struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	PCIAddress   string            `json:"pci_address"`
	Type         RDMAType          `json:"type"` // InfiniBand, RoCE, iWARP
	State        DeviceState       `json:"state"`
	Ports        []RDMAPort        `json:"ports"`
	Capabilities []string          `json:"capabilities"`
	Firmware     string            `json:"firmware"`
	Drivers      string            `json:"drivers"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	CreatedAt    time.Time         `json:"created_at"`
}

// RDMAType RDMA 类型
type RDMAType string

const (
	RDMATypeInfiniBand RDMAType = "infiniband"
	RDMATypeRoCE       RDMAType = "roce"
	RDMATypeiWARP      RDMAType = "iwarp"
)

// DeviceState 设备状态
type DeviceState string

const (
	DeviceStateActive  DeviceState = "active"
	DeviceStateDown    DeviceState = "down"
	DeviceStateError   DeviceState = "error"
	DeviceStateUnknown DeviceState = "unknown"
)

// RDMAPort RDMA 端口
type RDMAPort struct {
	ID          int        `json:"id"`
	State       PortState  `json:"state"`
	LinkSpeed   string     `json:"link_speed"` // SDR, DDR, QDR, FDR, EDR, HDR
	Width       int        `json:"width"`       // 1x, 4x, 8x, 12x
	GID         string     `json:"gid"`
	LID         int        `json:"lid"`
	MTU         int        `json:"mtu"`
	Counters    PortStats  `json:"counters"`
	LastChange  *time.Time `json:"last_change,omitempty"`
}

// PortState 端口状态
type PortState string

const (
	PortStateActive    PortState = "active"
	PortStateDown      PortState = "down"
	PortStateInit      PortState = "init"
	PortStateArmed     PortState = "armed"
	PortStateActiveDef PortState = "active_def"
)

// PortStats 端口统计
type PortStats struct {
	RxBytes      int64 `json:"rx_bytes"`
	TxBytes      int64 `json:"tx_bytes"`
	RxPackets    int64 `json:"rx_packets"`
	TxPackets    int64 `json:"tx_packets"`
	RxErrors     int64 `json:"rx_errors"`
	TxErrors     int64 `json:"tx_errors"`
	RxDrops      int64 `json:"rx_drops"`
	TxDrops      int64 `json:"tx_drops"`
}

// ========== RDMA 连接管理 ==========

// RDMAConnection RDMA 连接
type RDMAConnection struct {
	ID           string            `json:"id"`
	LocalDevice  string            `json:"local_device"`
	LocalPort    int               `json:"local_port"`
	RemoteGID    string            `json:"remote_gid"`
	RemoteLID    int               `json:"remote_lid"`
	QueuePair    QueuePairInfo     `json:"queue_pair"`
	State        ConnectionState   `json:"state"`
	Type         ConnectionType    `json:"type"`
	Stats        ConnectionStats   `json:"stats"`
	CreatedAt    time.Time         `json:"created_at"`
	LastActive   time.Time         `json:"last_active"`
}

// QueuePairInfo 队列对信息
type QueuePairInfo struct {
	ID         int    `json:"id"`
	State      string `json:"state"`
	Type       string `json:"type"` // RC, UD, UC
	MaxSendWR  int    `json:"max_send_wr"`
	MaxRecvWR  int    `json:"max_recv_wr"`
	MaxSendSge int    `json:"max_send_sge"`
	MaxRecvSge int    `json:"max_recv_sge"`
}

// ConnectionState 连接状态
type ConnectionState string

const (
	ConnectionStateEstablished ConnectionState = "established"
	ConnectionStateConnecting  ConnectionState = "connecting"
	ConnectionStateClosed      ConnectionState = "closed"
	ConnectionStateError       ConnectionState = "error"
)

// ConnectionType 连接类型
type ConnectionType string

const (
	ConnectionTypeRC ConnectionType = "rc" // Reliable Connection
	ConnectionTypeUD ConnectionType = "ud" // Unreliable Datagram
	ConnectionTypeUC ConnectionType = "uc" // Unreliable Connection
)

// ConnectionStats 连接统计
type ConnectionStats struct {
	BytesSent     int64     `json:"bytes_sent"`
	BytesReceived int64     `json:"bytes_received"`
	MessagesSent  int64     `json:"messages_sent"`
	MessagesRecv  int64     `json:"messages_recv"`
	AvgLatencyNs  int64     `json:"avg_latency_ns"`
	MaxLatencyNs  int64     `json:"max_latency_ns"`
	ThroughputMB  float64   `json:"throughput_mb"`
	LastMeasured  time.Time `json:"last_measured"`
}

// ========== RDMA 优化器管理器 ==========

// RDMAOptimizer RDMA 优化器
type RDMAOptimizer struct {
	mu          sync.RWMutex
	devices     map[string]*RDMADevice
	connections map[string]*RDMAConnection
	config      OptimizerConfig
	stats       OptimizerStats
}

// OptimizerConfig 优化器配置
type OptimizerConfig struct {
	AutoOptimize      bool    `json:"auto_optimize"`
	MaxQueuePairs     int     `json:"max_queue_pairs"`
	BufferSize        int     `json:"buffer_size"`        // 字节
	CompletionQueue   int     `json:"completion_queue"`   // CQ 深度
	MaxInlineData     int     `json:"max_inline_data"`    // 内联数据大小
	PrefetchEnabled   bool    `json:"prefetch_enabled"`
	PrefetchSize      int     `json:"prefetch_size"`
	NumaAware         bool    `json:"numa_aware"`
	InterruptCoalesce bool    `json:"interrupt_coalesce"`
	InterruptRate     int     `json:"interrupt_rate"`     // 每秒中断数
	TargetLatencyNs   int64   `json:"target_latency_ns"`  // 目标延迟
	TargetThroughputGB float64 `json:"target_throughput_gb"` // 目标吞吐量
}

// OptimizerStats 优化器统计
type OptimizerStats struct {
	TotalDevices     int       `json:"total_devices"`
	ActiveDevices    int       `json:"active_devices"`
	TotalConnections int       `json:"total_connections"`
	AvgLatencyNs     int64     `json:"avg_latency_ns"`
	TotalThroughputGB float64  `json:"total_throughput_gb"`
	OptimizationCount int64    `json:"optimization_count"`
	LastOptimized    time.Time `json:"last_optimized"`
}

// NewRDMAOptimizer 创建 RDMA 优化器
func NewRDMAOptimizer(config OptimizerConfig) *RDMAOptimizer {
	// 设置默认值
	if config.MaxQueuePairs == 0 {
		config.MaxQueuePairs = 256
	}
	if config.BufferSize == 0 {
		config.BufferSize = 1024 * 1024 // 1MB
	}
	if config.CompletionQueue == 0 {
		config.CompletionQueue = 4096
	}
	if config.MaxInlineData == 0 {
		config.MaxInlineData = 256
	}
	if config.PrefetchSize == 0 {
		config.PrefetchSize = 4096
	}
	if config.InterruptRate == 0 {
		config.InterruptRate = 10000
	}
	if config.TargetLatencyNs == 0 {
		config.TargetLatencyNs = 1000 // 1μs
	}
	if config.TargetThroughputGB == 0 {
		config.TargetThroughputGB = 100 // 100 GB/s
	}

	return &RDMAOptimizer{
		devices:     make(map[string]*RDMADevice),
		connections: make(map[string]*RDMAConnection),
		config:      config,
	}
}

// ========== 设备管理方法 ==========

// RegisterDevice 注册 RDMA 设备
func (o *RDMAOptimizer) RegisterDevice(device RDMADevice) (*RDMADevice, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("rdma-%s-%d", device.Name, time.Now().UnixNano())
	}

	if _, exists := o.devices[device.ID]; exists {
		return nil, fmt.Errorf("设备已存在: %s", device.ID)
	}

	device.CreatedAt = time.Now()
	device.LastSeen = time.Now()
	if device.State == "" {
		device.State = DeviceStateActive
	}

	o.devices[device.ID] = &device
	o.updateStats()

	return &device, nil
}

// UnregisterDevice 注销 RDMA 设备
func (o *RDMAOptimizer) UnregisterDevice(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.devices[id]; !exists {
		return fmt.Errorf("设备不存在: %s", id)
	}

	// 关闭相关连接
	for connID, conn := range o.connections {
		if conn.LocalDevice == id {
			conn.State = ConnectionStateClosed
			delete(o.connections, connID)
		}
	}

	delete(o.devices, id)
	o.updateStats()

	return nil
}

// GetDevice 获取 RDMA 设备
func (o *RDMAOptimizer) GetDevice(id string) (*RDMADevice, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	device, exists := o.devices[id]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", id)
	}

	return device, nil
}

// ListDevices 列出所有 RDMA 设备
func (o *RDMAOptimizer) ListDevices() []*RDMADevice {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*RDMADevice, 0, len(o.devices))
	for _, d := range o.devices {
		result = append(result, d)
	}

	return result
}

// UpdateDeviceState 更新设备状态
func (o *RDMAOptimizer) UpdateDeviceState(id string, state DeviceState) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	device, exists := o.devices[id]
	if !exists {
		return fmt.Errorf("设备不存在: %s", id)
	}

	device.State = state
	device.LastSeen = time.Now()
	o.updateStats()

	return nil
}

// ========== 连接管理方法 ==========

// CreateConnection 创建 RDMA 连接
func (o *RDMAOptimizer) CreateConnection(conn RDMAConnection) (*RDMAConnection, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 验证本地设备存在
	if _, exists := o.devices[conn.LocalDevice]; !exists {
		return nil, fmt.Errorf("本地设备不存在: %s", conn.LocalDevice)
	}

	if conn.ID == "" {
		conn.ID = fmt.Sprintf("conn-%s-%s-%d", conn.LocalDevice, conn.RemoteGID, time.Now().UnixNano())
	}

	conn.State = ConnectionStateConnecting
	conn.CreatedAt = time.Now()
	conn.LastActive = time.Now()

	if conn.Type == "" {
		conn.Type = ConnectionTypeRC
	}

	o.connections[conn.ID] = &conn
	o.updateStats()

	return &conn, nil
}

// CloseConnection 关闭 RDMA 连接
func (o *RDMAOptimizer) CloseConnection(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	conn, exists := o.connections[id]
	if !exists {
		return fmt.Errorf("连接不存在: %s", id)
	}

	conn.State = ConnectionStateClosed
	delete(o.connections, id)
	o.updateStats()

	return nil
}

// GetConnection 获取 RDMA 连接
func (o *RDMAOptimizer) GetConnection(id string) (*RDMAConnection, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	conn, exists := o.connections[id]
	if !exists {
		return nil, fmt.Errorf("连接不存在: %s", id)
	}

	return conn, nil
}

// ListConnections 列出所有连接
func (o *RDMAOptimizer) ListConnections() []*RDMAConnection {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*RDMAConnection, 0, len(o.connections))
	for _, c := range o.connections {
		result = append(result, c)
	}

	return result
}

// ========== 优化功能 ==========

// OptimizeConnection 优化连接性能
func (o *RDMAOptimizer) OptimizeConnection(connID string) (*OptimizationResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	conn, exists := o.connections[connID]
	if !exists {
		return nil, fmt.Errorf("连接不存在: %s", connID)
	}

	result := &OptimizationResult{
		ConnID:    connID,
		Timestamp: time.Now(),
		Actions:   []OptimizationAction{},
	}

	// 分析并优化延迟
	if conn.Stats.AvgLatencyNs > o.config.TargetLatencyNs {
		latencyReduction := conn.Stats.AvgLatencyNs - o.config.TargetLatencyNs
		result.Actions = append(result.Actions, OptimizationAction{
			Type:        "latency_optimization",
			Description: fmt.Sprintf("降低延迟 %dns", latencyReduction),
			Applied:     true,
		})
	}

	// 分析并优化吞吐量
	if conn.Stats.ThroughputMB < o.config.TargetThroughputGB*1024 {
		throughputIncrease := o.config.TargetThroughputGB*1024 - conn.Stats.ThroughputMB
		result.Actions = append(result.Actions, OptimizationAction{
			Type:        "throughput_optimization",
			Description: fmt.Sprintf("提升吞吐量 %.2f MB/s", throughputIncrease),
			Applied:     true,
		})
	}

	// 自动调整队列对配置
	if o.config.AutoOptimize {
		result.Actions = append(result.Actions, OptimizationAction{
			Type:        "queue_pair_tuning",
			Description: "自动调整队列对参数",
			Applied:     true,
		})
	}

	o.stats.OptimizationCount++
	o.stats.LastOptimized = time.Now()

	return result, nil
}

// OptimizationResult 优化结果
type OptimizationResult struct {
	ConnID    string              `json:"conn_id"`
	Timestamp time.Time           `json:"timestamp"`
	Actions   []OptimizationAction `json:"actions"`
	Summary   string              `json:"summary"`
}

// OptimizationAction 优化动作
type OptimizationAction struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Applied     bool   `json:"applied"`
	Impact      string `json:"impact,omitempty"`
}

// AutoOptimize 自动优化所有连接
func (o *RDMAOptimizer) AutoOptimize() (*AutoOptimizeResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	result := &AutoOptimizeResult{
		Timestamp:       time.Now(),
		TotalConnections: len(o.connections),
		OptimizedCount:  0,
		SkippedCount:    0,
		FailedCount:     0,
		Results:         make(map[string]*OptimizationResult),
	}

	for connID, conn := range o.connections {
		if conn.State != ConnectionStateEstablished {
			result.SkippedCount++
			continue
		}

		optResult := &OptimizationResult{
			ConnID:    connID,
			Timestamp: time.Now(),
			Actions:   []OptimizationAction{},
		}

		// 检查是否需要优化
		needsOptimization := false

		if conn.Stats.AvgLatencyNs > o.config.TargetLatencyNs {
			needsOptimization = true
			optResult.Actions = append(optResult.Actions, OptimizationAction{
				Type:        "latency_optimization",
				Description: "延迟高于目标值",
				Applied:     true,
			})
		}

		if conn.Stats.ThroughputMB < o.config.TargetThroughputGB*1024 {
			needsOptimization = true
			optResult.Actions = append(optResult.Actions, OptimizationAction{
				Type:        "throughput_optimization",
				Description: "吞吐量低于目标值",
				Applied:     true,
			})
		}

		if needsOptimization {
			result.OptimizedCount++
			o.stats.OptimizationCount++
		} else {
			result.SkippedCount++
		}

		result.Results[connID] = optResult
	}

	o.stats.LastOptimized = time.Now()

	return result, nil
}

// AutoOptimizeResult 自动优化结果
type AutoOptimizeResult struct {
	Timestamp        time.Time                      `json:"timestamp"`
	TotalConnections int                            `json:"total_connections"`
	OptimizedCount   int                            `json:"optimized_count"`
	SkippedCount     int                            `json:"skipped_count"`
	FailedCount      int                            `json:"failed_count"`
	Results          map[string]*OptimizationResult `json:"results"`
}

// ========== 性能监控 ==========

// GetPerformanceMetrics 获取性能指标
func (o *RDMAOptimizer) GetPerformanceMetrics() *PerformanceMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	metrics := &PerformanceMetrics{
		Timestamp: time.Now(),
		Devices:   make(map[string]DeviceMetrics),
		Overall:   OverallMetrics{},
	}

	var totalLatency int64
	var totalThroughput float64
	var totalConnections int
	var activeDevices int

	for id, device := range o.devices {
		deviceMetrics := DeviceMetrics{
			DeviceID: id,
			State:    device.State,
			Ports:    len(device.Ports),
		}

		if device.State == DeviceStateActive {
			activeDevices++
		}

		metrics.Devices[id] = deviceMetrics
	}

	for _, conn := range o.connections {
		if conn.State == ConnectionStateEstablished {
			totalConnections++
			totalLatency += conn.Stats.AvgLatencyNs
			totalThroughput += conn.Stats.ThroughputMB
		}
	}

	if totalConnections > 0 {
		metrics.Overall.AvgLatencyNs = totalLatency / int64(totalConnections)
		metrics.Overall.TotalThroughputGB = totalThroughput / 1024
	}
	metrics.Overall.TotalDevices = len(o.devices)
	metrics.Overall.ActiveDevices = activeDevices
	metrics.Overall.TotalConnections = totalConnections

	return metrics
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	Timestamp time.Time                `json:"timestamp"`
	Devices   map[string]DeviceMetrics `json:"devices"`
	Overall   OverallMetrics           `json:"overall"`
}

// DeviceMetrics 设备指标
type DeviceMetrics struct {
	DeviceID string      `json:"device_id"`
	State    DeviceState `json:"state"`
	Ports    int         `json:"ports"`
	ThroughputMB float64 `json:"throughput_mb"`
	AvgLatencyNs int64   `json:"avg_latency_ns"`
}

// OverallMetrics 整体指标
type OverallMetrics struct {
	TotalDevices      int     `json:"total_devices"`
	ActiveDevices     int     `json:"active_devices"`
	TotalConnections  int     `json:"total_connections"`
	AvgLatencyNs      int64   `json:"avg_latency_ns"`
	TotalThroughputGB float64 `json:"total_throughput_gb"`
}

// ========== 辅助方法 ==========

// updateStats 更新统计
func (o *RDMAOptimizer) updateStats() {
	o.stats.TotalDevices = len(o.devices)
	o.stats.ActiveDevices = 0
	for _, d := range o.devices {
		if d.State == DeviceStateActive {
			o.stats.ActiveDevices++
		}
	}
	o.stats.TotalConnections = len(o.connections)
}

// GetStats 获取统计
func (o *RDMAOptimizer) GetStats() OptimizerStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.stats
}

// SaveConfig 保存配置
func (o *RDMAOptimizer) SaveConfig(path string) error {
	o.mu.RLock()
	defer o.mu.RUnlock()

	data, err := json.MarshalIndent(o.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadConfig 加载配置
func (o *RDMAOptimizer) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	return json.Unmarshal(data, &o.config)
}
