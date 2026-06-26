package rdmanfs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RDMAProtocol RDMA 协议
type RDMAProtocol string

const (
	ProtocolRoCEv2     RDMAProtocol = "rocev2"
	ProtocolInfiniBand RDMAProtocol = "infiniband"
	ProtocolIWARP      RDMAProtocol = "iwarp"
)

// RDMADeviceState RDMA 设备状态
type RDMADeviceState string

const (
	DeviceUp    RDMADeviceState = "up"
	DeviceDown  RDMADeviceState = "down"
	DeviceError RDMADeviceState = "error"
	DeviceInit  RDMADeviceState = "initializing"
)

// NFSRDMAState NFS over RDMA 服务状态
type NFSRDMAState string

const (
	NFSStateStopped  NFSRDMAState = "stopped"
	NFSStateStarting NFSRDMAState = "starting"
	NFSStateRunning  NFSRDMAState = "running"
	NFSStateError    NFSRDMAState = "error"
)

// RDMADeviceInfo RDMA 网卡信息
type RDMADeviceInfo struct {
	Name      string          `json:"name"`
	State     RDMADeviceState `json:"state"`
	Protocol  RDMAProtocol    `json:"protocol"`
	NodeGUID  string          `json:"nodeGuid"`
	PortGUID  string          `json:"portGuid"`
	PortNum   int             `json:"portNum"`
	Mtu       int             `json:"mtu"`
	Rate      int64           `json:"rate"` // bps
	MAC       string          `json:"mac"`
	Interface string          `json:"interface"` // 关联的网络接口
	Firmware  string          `json:"firmware"`
	LinkLayer string          `json:"linkLayer"`
}

// NFSRDMAConfig NFS over RDMA 配置
type NFSRDMAConfig struct {
	Enabled        bool   `json:"enabled"`
	Device         string `json:"device"`     // RDMA 设备名
	ExportRoot     string `json:"exportRoot"` // NFS 导出根目录
	Port           int    `json:"port"`       // NFS RDMA 端口
	NFSVersion     string `json:"nfsVersion"` // "4.2", "4.1"
	IOTimeout      int    `json:"ioTimeout"`  // seconds
	MaxConnections int    `json:"maxConnections"`
	ReadWriteSize  int    `json:"readWriteSize"` // bytes
	EnableAuth     bool   `json:"enableAuth"`
	AuthType       string `json:"authType"` // "krb5", "sys", "none"
}

// NFSExport NFS 导出配置
type NFSExport struct {
	Path         string `json:"path"`
	Client       string `json:"client"` // "192.168.1.0/24" or "*"
	ReadOnly     bool   `json:"readOnly"`
	NoRootSquash bool   `json:"noRootSquash"`
	Sync         bool   `json:"sync"`
	SecFlavor    string `json:"secFlavor"` // "sys", "krb5", "krb5i", "krb5p"
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	BytesRead          int64     `json:"bytesRead"`
	BytesWritten       int64     `json:"bytesWritten"`
	OpsRead            int64     `json:"opsRead"`
	OpsWrite           int64     `json:"opsWrite"`
	OpsMetadata        int64     `json:"opsMetadata"`
	AvgLatencyMs       float64   `json:"avgLatencyMs"`
	MaxLatencyMs       float64   `json:"maxLatencyMs"`
	CurrentConnections int       `json:"currentConnections"`
	ActiveConnections  int       `json:"activeConnections"`
	CollectAt          time.Time `json:"collectAt"`
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ClientAddr   string    `json:"clientAddr"`
	Device       string    `json:"device"`
	ConnectedAt  time.Time `json:"connectedAt"`
	BytesRead    int64     `json:"bytesRead"`
	BytesWritten int64     `json:"bytesWritten"`
	IsRDMA       bool      `json:"isRdma"`
}

// RDMAServiceStatus RDMA 服务状态概览
type RDMAServiceStatus struct {
	NFSRDMA      NFSRDMAState      `json:"nfsRdmaState"`
	Devices      []RDMADeviceInfo  `json:"devices"`
	ActiveDevice string            `json:"activeDevice"`
	Config       NFSRDMAConfig     `json:"config"`
	Stats        *PerformanceStats `json:"stats"`
	Connections  []ConnectionInfo  `json:"connections"`
	Uptime       *time.Duration    `json:"uptime"`
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	ConfigPath     string `json:"configPath"`
	StatsInterval  int    `json:"statsInterval"` // seconds
	MaxExportPaths int    `json:"maxExportPaths"`
	LogPath        string `json:"logPath"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ConfigPath:     "/var/lib/nas-os/rdmanfs",
		StatsInterval:  10,
		MaxExportPaths: 256,
		LogPath:        "/var/log/nas-os/rdmanfs",
	}
}

// Manager NFS over RDMA 管理器
type Manager struct {
	mu        sync.RWMutex
	config    *ManagerConfig
	nfsConfig NFSRDMAConfig
	devices   []RDMADeviceInfo
	exports   []NFSExport
	stats     *PerformanceStats
	state     NFSRDMAState
	startTime *time.Time
}

// NewManager 创建 NFS over RDMA 管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:  config,
		devices: make([]RDMADeviceInfo, 0),
		exports: make([]NFSExport, 0),
		state:   NFSStateStopped,
		nfsConfig: NFSRDMAConfig{
			Enabled:        false,
			Port:           2049,
			NFSVersion:     "4.2",
			IOTimeout:      30,
			MaxConnections: 128,
			ReadWriteSize:  1048576, // 1MB
			ExportRoot:     "/export",
			AuthType:       "sys",
		},
	}
}

// DetectRDMADevices 检测 RDMA 网卡
func (m *Manager) DetectRDMADevices(ctx context.Context) ([]RDMADeviceInfo, error) {
	// TODO: 实际通过 ibstat / rdma link 检测 RDMA 设备
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟检测
	m.devices = []RDMADeviceInfo{}
	return m.devices, nil
}

// GetDevices 获取 RDMA 设备列表
func (m *Manager) GetDevices() []RDMADeviceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RDMADeviceInfo, len(m.devices))
	copy(result, m.devices)
	return result
}

// GetDeviceByName 按名称获取设备
func (m *Manager) GetDeviceByName(name string) (*RDMADeviceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i, dev := range m.devices {
		if dev.Name == name {
			return &m.devices[i], nil
		}
	}
	return nil, fmt.Errorf("RDMA 设备 %s 未找到", name)
}

// ConfigureNFSRDMA 配置 NFS over RDMA
func (m *Manager) ConfigureNFSRDMA(cfg NFSRDMAConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("无效端口: %d", cfg.Port)
	}
	if cfg.ExportRoot == "" {
		return fmt.Errorf("导出根目录不能为空")
	}

	m.nfsConfig = cfg
	return nil
}

// GetNFSRDMAConfig 获取 NFS over RDMA 配置
func (m *Manager) GetNFSRDMAConfig() NFSRDMAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nfsConfig
}

// StartService 启动 NFS over RDMA 服务
func (m *Manager) StartService(ctx context.Context) error {
	// TODO: 实际启动 nfsrdma 服务
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == NFSStateRunning {
		return fmt.Errorf("服务已在运行")
	}
	if !m.nfsConfig.Enabled {
		return fmt.Errorf("NFS RDMA 未启用")
	}
	if m.nfsConfig.Device == "" {
		return fmt.Errorf("未指定 RDMA 设备")
	}

	m.state = NFSStateRunning
	now := time.Now()
	m.startTime = &now
	return nil
}

// StopService 停止 NFS over RDMA 服务
func (m *Manager) StopService(ctx context.Context) error {
	// TODO: 实际停止 nfsrdma 服务
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != NFSStateRunning {
		return fmt.Errorf("服务未在运行")
	}

	m.state = NFSStateStopped
	m.startTime = nil
	return nil
}

// GetServiceState 获取服务状态
func (m *Manager) GetServiceState() NFSRDMAState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// AddExport 添加 NFS 导出
func (m *Manager) AddExport(export NFSExport) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.exports) >= m.config.MaxExportPaths {
		return fmt.Errorf("已达最大导出路径数 %d", m.config.MaxExportPaths)
	}
	if export.Path == "" {
		return fmt.Errorf("导出路径不能为空")
	}

	m.exports = append(m.exports, export)
	return nil
}

// RemoveExport 移除 NFS 导出
func (m *Manager) RemoveExport(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, exp := range m.exports {
		if exp.Path == path {
			m.exports = append(m.exports[:i], m.exports[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("导出路径 %s 不存在", path)
}

// ListExports 列出导出
func (m *Manager) ListExports() []NFSExport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]NFSExport, len(m.exports))
	copy(result, m.exports)
	return result
}

// CollectStats 收集性能统计
func (m *Manager) CollectStats(ctx context.Context) (*PerformanceStats, error) {
	// TODO: 实际从 /proc/fs/nfsd 或 perf 采集
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if m.stats == nil {
		m.stats = &PerformanceStats{CollectAt: now}
	}
	m.stats.CollectAt = now
	return m.stats, nil
}

// GetStats 获取性能统计
func (m *Manager) GetStats() *PerformanceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetStatus 获取整体状态
func (m *Manager) GetStatus() *RDMAServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &RDMAServiceStatus{
		NFSRDMA: m.state,
		Devices: make([]RDMADeviceInfo, len(m.devices)),
		Config:  m.nfsConfig,
		Stats:   m.stats,
	}
	copy(status.Devices, m.devices)

	if m.startTime != nil && m.state == NFSStateRunning {
		uptime := time.Since(*m.startTime)
		status.Uptime = &uptime
	}

	return status
}
