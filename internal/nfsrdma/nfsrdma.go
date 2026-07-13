// Package nfsrdma 提供 NFS over RDMA 支持，对标 TrueNAS 25.04 企业功能。
// 通过 InfiniBand/RoCE 网络传输 NFS 数据，显著降低延迟、提高吞吐量。
// 兵部开发。
package nfsrdma

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// RDMAStatus RDMA 链路状态.
type RDMAStatus string

const (
	RDMAStatusDown    RDMAStatus = "down"
	RDMAStatusInit    RDMAStatus = "initializing"
	RDMAStatusUp      RDMAStatus = "up"
	RDMAStatusDegraded RDMAStatus = "degraded"
)

// TransportType RDMA 传输协议.
type TransportType string

const (
	TransportRoCEv2  TransportType = "rocev2"
	TransportIB       TransportType = "infiniband"
	TransportIWARP    TransportType = "iwarp"
)

// RDMAInterface RDMA 网络接口.
type RDMAInterface struct {
	Name         string        `json:"name"`
	Device       string        `json:"device"`
	Transport    TransportType `json:"transport"`
	Status       RDMAStatus    `json:"status"`
	SpeedGbps    int           `json:"speed_gbps"`
	Port         int           `json:"port"`
	MTU          int           `json:"mtu"`
	PeerCount    int           `json:"peer_count"`
	ErrorCount   int64         `json:"error_count"`
	RxBytes      int64         `json:"rx_bytes"`
	TxBytes      int64         `json:"tx_bytes"`
}

// NFSRDMAExport NFS over RDMA 导出配置.
type NFSRDMAExport struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	ExportPath    string   `json:"export_path"`
	AllowedHosts  []string `json:"allowed_hosts"`
	Readonly      bool     `json:"readonly"`
	Squash        string   `json:"squash"`
	SecType       string   `json:"sec_type"`
	RDMAOnly      bool     `json:"rdma_only"`
	TransportBoth bool     `json:"transport_both"`
	OutputThrottleBW int64  `json:"output_throttle_bw_mbps"`
}

// NFSRDMAStats 性能统计.
type NFSRDMAStats struct {
	ReadOpsPerSec   float64 `json:"read_ops_sec"`
	WriteOpsPerSec  float64 `json:"write_ops_sec"`
	ReadBWMBps      float64 `json:"read_bw_mbps"`
	WriteBWMBps     float64 `json:"write_bw_mbps"`
	AvgLatencyUs    float64 `json:"avg_latency_us"`
	P99LatencyUs    float64 `json:"p99_latency_us"`
	ActiveConnections int   `json:"active_connections"`
}

// Manager NFS over RDMA 管理器.
type Manager struct {
	mu         sync.RWMutex
	ifaces     map[string]*RDMAInterface
	exports    map[string]*NFSRDMAExport
	stats      map[string]*NFSRDMAStats
	configPath string
}

// NewManager 创建 RDMA 管理器.
func NewManager(configPath string) *Manager {
	return &Manager{
		ifaces:     make(map[string]*RDMAInterface),
		exports:    make(map[string]*NFSRDMAExport),
		stats:      make(map[string]*NFSRDMAStats),
		configPath: configPath,
	}
}

// RegisterInterface 注册 RDMA 接口.
func (m *Manager) RegisterInterface(iface *RDMAInterface) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if iface.Name == "" {
		return fmt.Errorf("interface name cannot be empty")
	}
	if iface.SpeedGbps <= 0 {
		return fmt.Errorf("speed must be positive")
	}
	if iface.MTU == 0 {
		iface.MTU = 9000 // 默认 jumbo frame
	}
	m.ifaces[iface.Name] = iface
	return nil
}

// ListInterfaces 列出所有 RDMA 接口.
func (m *Manager) ListInterfaces() []*RDMAInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*RDMAInterface, 0, len(m.ifaces))
	for _, iface := range m.ifaces {
		result = append(result, iface)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// AddExport 添加 NFS RDMA 导出.
func (m *Manager) AddExport(export *NFSRDMAExport) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if export.Path == "" {
		return fmt.Errorf("export path cannot be empty")
	}
	if export.ID == "" {
		export.ID = fmt.Sprintf("rdma-export-%d", time.Now().UnixMilli())
	}
	m.exports[export.ID] = export
	return nil
}

// ListExports 列出所有导出.
func (m *Manager) ListExports() []*NFSRDMAExport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*NFSRDMAExport, 0, len(m.exports))
	for _, e := range m.exports {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// RemoveExport 移除导出.
func (m *Manager) RemoveExport(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.exports[id]; !ok {
		return fmt.Errorf("export %s not found", id)
	}
	delete(m.exports, id)
	return nil
}

// GetStats 获取性能统计.
func (m *Manager) GetStats(ifaceName string) (*NFSRDMAStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.ifaces[ifaceName]; !ok {
		return nil, fmt.Errorf("interface %s not found", ifaceName)
	}
	stats, ok := m.stats[ifaceName]
	if !ok {
		return &NFSRDMAStats{}, nil
	}
	return stats, nil
}

// HealthCheck 检查 RDMA 健康状态.
func (m *Manager) HealthCheck() map[string]RDMAStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]RDMAStatus)
	for name, iface := range m.ifaces {
		if iface.ErrorCount > 100 {
			result[name] = RDMAStatusDegraded
		} else {
			result[name] = iface.Status
		}
	}
	return result
}

// RecommendExportConfig 根据网络条件推荐导出配置.
func (m *Manager) RecommendExportConfig(path string) *NFSRDMAExport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	export := &NFSRDMAExport{
		Path:          path,
		ExportPath:    fmt.Sprintf("/export%s", path),
		AllowedHosts:  []string{"*"},
		Readonly:      false,
		Squash:        "root_squash",
		SecType:       "sys",
		RDMAOnly:      false,
		TransportBoth: true,
	}
	export.ID = fmt.Sprintf("rdma-rec-%d", time.Now().UnixMilli())
	return export
}