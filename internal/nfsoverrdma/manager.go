// Package nfsoverrdma 提供NFS over RDMA高性能远程存储访问
package nfsoverrdma

import (
	"errors"
	"net"
	"sync"
	"time"
)

// 错误定义
var (
	ErrExportNotFound    = errors.New("NFS导出不存在")
	ErrExportExists      = errors.New("NFS导出已存在")
	ErrClientNotFound    = errors.New("客户端不存在")
	ErrRDMAUnavailable   = errors.New("RDMA设备不可用")
	ErrConnectionFailed  = errors.New("RDMA连接失败")
	ErrPermissionDenied  = errors.New("权限被拒绝")
	ErrInvalidPath       = errors.New("无效路径")
	ErrMountFailed       = errors.New("挂载失败")
)

// ExportMode 导出模式
type ExportMode string

const (
	ExportModeReadWrite ExportMode = "rw"  // 读写
	ExportModeReadOnly  ExportMode = "ro"  // 只读
)

// SecurityType 安全类型
type SecurityType string

const (
	SecurityNone   SecurityType = "none"   // 无认证
	SecuritySys    SecurityType = "sys"    // 系统认证
	SecurityKrb5   SecurityType = "krb5"   // Kerberos V5
	SecurityKrb5i  SecurityType = "krb5i"  // Kerberos V5 + 完整性
	SecurityKrb5p  SecurityType = "krb5p"  // Kerberos V5 + 隐私
)

// TransportType 传输类型
type TransportType string

const (
	TransportRDMA   TransportType = "rdma"   // RDMA传输
	TransportTCP    TransportType = "tcp"    // TCP传输
	TransportAuto   TransportType = "auto"   // 自动选择
)

// RDMAProvider RDMA提供商
type RDMAProvider string

const (
	ProviderRoCE   RDMAProvider = "roce"   // RoCE v2
	ProviderIB     RDMAProvider = "ib"     // InfiniBand
	ProvideriWARP  RDMAProvider = "iwarp"  // iWARP
	ProviderAuto   RDMAProvider = "auto"   // 自动检测
)

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	ConnStatusDisconnected ConnectionStatus = "disconnected"
	ConnStatusConnecting   ConnectionStatus = "connecting"
	ConnStatusConnected    ConnectionStatus = "connected"
	ConnStatusError        ConnectionStatus = "error"
)

// RDMAConfig RDMA配置
type RDMAConfig struct {
	Enabled     bool         `json:"enabled"`
	Provider    RDMAProvider `json:"provider"`
	DeviceName  string       `json:"device_name"`
	Port        int          `json:"port"`
	MTU         int          `json:"mtu"`
	QueuePair   int          `json:"queue_pair"`
	MaxInline   int          `json:"max_inline"`
	CQSize      int          `json:"cq_size"`
	MaxRecvWR   int          `json:"max_recv_wr"`
	MaxSendWR   int          `json:"max_send_wr"`
	MaxSGE      int          `json:"max_sge"`
}

// NFSExport NFS导出配置
type NFSExport struct {
	ID            string            `json:"id"`
	Path          string            `json:"path"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Mode          ExportMode        `json:"mode"`
	Security      SecurityType      `json:"security"`
	Transport     TransportType     `json:"transport"`
	AllowedHosts  []string          `json:"allowed_hosts"`
	SquashMode    string            `json:"squash_mode"` // none, root, all
	AnonymousUID  int               `json:"anonymous_uid"`
	AnonymousGID  int               `json:"anonymous_gid"`
	SubtreeCheck  bool              `json:"subtree_check"`
	Sync          bool              `json:"sync"`
	ReadOnly      bool              `json:"read_only"`
	MaxReadSize   int64             `json:"max_read_size"`
	MaxWriteSize  int64             `json:"max_write_size"`
	RDMAConfig    *RDMAConfig       `json:"rdma_config,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// NFSClient NFS客户端连接
type NFSClient struct {
	ID           string           `json:"id"`
	ExportID     string           `json:"export_id"`
	ClientIP     net.IP           `json:"client_ip"`
	Hostname     string           `json:"hostname,omitempty"`
	Transport    TransportType    `json:"transport"`
	Status       ConnectionStatus `json:"status"`
	MountPoint   string           `json:"mount_point"`
	BytesRead    int64            `json:"bytes_read"`
	BytesWritten int64            `json:"bytes_written"`
	Operations   int64            `json:"operations"`
	LatencyMs    float64          `json:"latency_ms"`
	BandwidthMBs float64          `json:"bandwidth_mbs"`
	ConnectedAt  time.Time        `json:"connected_at"`
	LastActivity time.Time        `json:"last_activity"`
}

// RDMAStats RDMA统计信息
type RDMAStats struct {
	DeviceName       string  `json:"device_name"`
	PortState        string  `json:"port_state"`
	LinkSpeedGbps    float64 `json:"link_speed_gbps"`
	ActiveQueuePairs int     `json:"active_queue_pairs"`
	TotalSendWR      int64   `json:"total_send_wr"`
	TotalRecvWR      int64   `json:"total_recv_wr"`
	TotalSendBytes   int64   `json:"total_send_bytes"`
	TotalRecvBytes   int64   `json:"total_recv_bytes"`
	SendThroughput   float64 `json:"send_throughput_mbs"`
	RecvThroughput   float64 `json:"recv_throughput_mbs"`
	AvgLatencyUs     float64 `json:"avg_latency_us"`
	ErrorCount       int64   `json:"error_count"`
	RetransmitCount  int64   `json:"retransmit_count"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	ExportID        string    `json:"export_id"`
	Timestamp       time.Time `json:"timestamp"`
	IOPS            int64     `json:"iops"`
	ReadIOPS        int64     `json:"read_iops"`
	WriteIOPS       int64     `json:"write_iops"`
	ThroughputMBs   float64   `json:"throughput_mbs"`
	ReadBandwidth   float64   `json:"read_bandwidth_mbs"`
	WriteBandwidth  float64   `json:"write_bandwidth_mbs"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	P99LatencyMs    float64   `json:"p99_latency_ms"`
	ActiveClients   int       `json:"active_clients"`
	Connections     int       `json:"connections"`
}

// Manager NFS over RDMA管理器
type Manager struct {
	mu          sync.RWMutex
	exports     map[string]*NFSExport
	clients     map[string]*NFSClient
	rdmaConfig  *RDMAConfig
	rdmaStats   *RDMAStats
	performance map[string]*PerformanceMetrics
	startTime   time.Time
}

// NewManager 创建NFS over RDMA管理器
func NewManager() *Manager {
	return &Manager{
		exports:     make(map[string]*NFSExport),
		clients:     make(map[string]*NFSClient),
		rdmaStats:   &RDMAStats{},
		performance: make(map[string]*PerformanceMetrics),
		startTime:   time.Now(),
	}
}

// ConfigureRDMA 配置RDMA
func (m *Manager) ConfigureRDMA(config *RDMAConfig) error {
	if config == nil {
		return errors.New("配置不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.rdmaConfig = config
	m.rdmaStats = &RDMAStats{
		DeviceName:    config.DeviceName,
		PortState:     "active",
		LinkSpeedGbps: 100, // 默认100Gbps
	}

	return nil
}

// CreateExport 创建NFS导出
func (m *Manager) CreateExport(export *NFSExport) error {
	if export == nil || export.ID == "" || export.Path == "" {
		return ErrInvalidPath
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.exports[export.ID]; exists {
		return ErrExportExists
	}

	export.CreatedAt = time.Now()
	export.UpdatedAt = time.Now()

	if export.RDMAConfig == nil && m.rdmaConfig != nil {
		export.RDMAConfig = m.rdmaConfig
	}

	m.exports[export.ID] = export
	return nil
}

// UpdateExport 更新NFS导出
func (m *Manager) UpdateExport(export *NFSExport) error {
	if export == nil || export.ID == "" {
		return ErrInvalidPath
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.exports[export.ID]
	if !exists {
		return ErrExportNotFound
	}

	export.CreatedAt = existing.CreatedAt
	export.UpdatedAt = time.Now()
	m.exports[export.ID] = export

	return nil
}

// DeleteExport 删除NFS导出
func (m *Manager) DeleteExport(exportID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.exports[exportID]; !exists {
		return ErrExportNotFound
	}

	// 断开所有客户端连接
	for id, client := range m.clients {
		if client.ExportID == exportID {
			delete(m.clients, id)
		}
	}

	delete(m.exports, exportID)
	return nil
}

// GetExport 获取NFS导出
func (m *Manager) GetExport(exportID string) (*NFSExport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export, exists := m.exports[exportID]
	if !exists {
		return nil, ErrExportNotFound
	}
	return export, nil
}

// ListExports 列出所有导出
func (m *Manager) ListExports() []*NFSExport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*NFSExport, 0, len(m.exports))
	for _, export := range m.exports {
		result = append(result, export)
	}
	return result
}

// AddClient 添加客户端连接
func (m *Manager) AddClient(client *NFSClient) error {
	if client == nil || client.ID == "" || client.ExportID == "" {
		return ErrInvalidPath
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.exports[client.ExportID]; !exists {
		return ErrExportNotFound
	}

	client.Status = ConnStatusConnected
	client.ConnectedAt = time.Now()
	client.LastActivity = time.Now()
	m.clients[client.ID] = client

	return nil
}

// RemoveClient 移除客户端连接
func (m *Manager) RemoveClient(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[clientID]; !exists {
		return ErrClientNotFound
	}

	delete(m.clients, clientID)
	return nil
}

// GetClient 获取客户端信息
func (m *Manager) GetClient(clientID string) (*NFSClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[clientID]
	if !exists {
		return nil, ErrClientNotFound
	}
	return client, nil
}

// ListClients 列出客户端
func (m *Manager) ListClients(exportID string) []*NFSClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*NFSClient, 0)
	for _, client := range m.clients {
		if exportID == "" || client.ExportID == exportID {
			result = append(result, client)
		}
	}
	return result
}

// GetRDMAStats 获取RDMA统计
func (m *Manager) GetRDMAStats() *RDMAStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.rdmaStats
	stats.ActiveQueuePairs = len(m.clients)
	return &stats
}

// GetPerformanceMetrics 获取性能指标
func (m *Manager) GetPerformanceMetrics(exportID string) *PerformanceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.performance[exportID]; exists {
		return metrics
	}

	_, exists := m.exports[exportID]
	if !exists {
		return nil
	}

	metrics := &PerformanceMetrics{
		ExportID:  exportID,
		Timestamp: time.Now(),
	}

	for _, client := range m.clients {
		if client.ExportID == exportID {
			metrics.ActiveClients++
			metrics.ReadIOPS += int64(client.Operations / 2)
			metrics.WriteIOPS += int64(client.Operations / 2)
			metrics.ReadBandwidth += client.BandwidthMBs / 2
			metrics.WriteBandwidth += client.BandwidthMBs / 2
		}
	}

	metrics.IOPS = metrics.ReadIOPS + metrics.WriteIOPS
	metrics.ThroughputMBs = metrics.ReadBandwidth + metrics.WriteBandwidth
	metrics.Connections = metrics.ActiveClients

	return metrics
}

// SimulateRDMAConnection 模拟RDMA连接测试
func (m *Manager) SimulateRDMAConnection(targetIP string, port int) (*RDMAConnectionTest, error) {
	if targetIP == "" {
		return nil, ErrInvalidPath
	}

	start := time.Now()

	// 模拟RDMA连接测试
	time.Sleep(10 * time.Millisecond)

	duration := time.Since(start)

	return &RDMAConnectionTest{
		TargetIP:    targetIP,
		Port:        port,
		Success:     true,
		LatencyMs:   float64(duration.Milliseconds()),
		Provider:    "roce",
		LinkSpeed:   100,
		MTU:         4096,
		TestedAt:    time.Now(),
	}, nil
}

// RDMAConnectionTest RDMA连接测试结果
type RDMAConnectionTest struct {
	TargetIP  string    `json:"target_ip"`
	Port      int       `json:"port"`
	Success   bool      `json:"success"`
	LatencyMs float64   `json:"latency_ms"`
	Provider  string    `json:"provider"`
	LinkSpeed int       `json:"link_speed_gbps"`
	MTU       int       `json:"mtu"`
	Error     string    `json:"error,omitempty"`
	TestedAt  time.Time `json:"tested_at"`
}

// GetServerStats 获取服务器统计
func (m *Manager) GetServerStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalClients := len(m.clients)
	totalExports := len(m.exports)

	var totalRead, totalWrite int64
	var totalOps int64
	for _, client := range m.clients {
		totalRead += client.BytesRead
		totalWrite += client.BytesWritten
		totalOps += client.Operations
	}

	return map[string]interface{}{
		"total_exports":     totalExports,
		"total_clients":     totalClients,
		"total_bytes_read":  totalRead,
		"total_bytes_written": totalWrite,
		"total_operations":  totalOps,
		"uptime_seconds":    int64(time.Since(m.startTime).Seconds()),
		"rdma_enabled":      m.rdmaConfig != nil && m.rdmaConfig.Enabled,
	}
}

// EnableRDMAOnExport 为导出启用RDMA
func (m *Manager) EnableRDMAOnExport(exportID string, config *RDMAConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	export, exists := m.exports[exportID]
	if !exists {
		return ErrExportNotFound
	}

	export.RDMAConfig = config
	export.Transport = TransportRDMA
	export.UpdatedAt = time.Now()

	return nil
}

// DisableRDMAOnExport 禁用导出的RDMA
func (m *Manager) DisableRDMAOnExport(exportID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	export, exists := m.exports[exportID]
	if !exists {
		return ErrExportNotFound
	}

	export.RDMAConfig = nil
	export.Transport = TransportTCP
	export.UpdatedAt = time.Now()

	return nil
}

// GetExportStats 获取导出统计
func (m *Manager) GetExportStats(exportID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export, exists := m.exports[exportID]
	if !exists {
		return nil
	}

	clientCount := 0
	var totalRead, totalWrite int64
	for _, client := range m.clients {
		if client.ExportID == exportID {
			clientCount++
			totalRead += client.BytesRead
			totalWrite += client.BytesWritten
		}
	}

	return map[string]interface{}{
		"export_id":           export.ID,
		"export_name":         export.Name,
		"path":                export.Path,
		"transport":           export.Transport,
		"rdma_enabled":        export.RDMAConfig != nil && export.RDMAConfig.Enabled,
		"active_clients":      clientCount,
		"total_bytes_read":    totalRead,
		"total_bytes_written": totalWrite,
		"created_at":          export.CreatedAt,
	}
}
