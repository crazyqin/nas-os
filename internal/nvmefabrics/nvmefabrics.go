// Package nvmefabrics NVMe/TCP 和 NVMe/RDMA 存储网络支持
// 灵感来源: TrueNAS 25.10 NVMe over Fabric
package nvmefabrics

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// TransportType 传输类型
type TransportType string

const (
	TransportTCP  TransportType = "tcp"
	TransportRDMA TransportType = "rdma"
)

// NVMeTargetState 目标状态
type NVMeTargetState string

const (
	TargetStateActive   NVMeTargetState = "active"
	TargetStateInactive NVMeTargetState = "inactive"
	TargetStateError    NVMeTargetState = "error"
)

// NVMeTarget NVMe 目标
type NVMeTarget struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Transport     TransportType   `json:"transport"`
	IP            net.IP          `json:"ip"`
	Port          int             `json:"port"`
	SubsystemNQN  string          `json:"subsystem_nqn"`
	State         NVMeTargetState `json:"state"`
	MaxNamespaces int             `json:"max_namespaces"`
	Namespaces    []Namespace     `json:"namespaces"`
	CreatedAt     time.Time       `json:"created_at"`
	ConnectedHosts []string       `json:"connected_hosts"`
}

// Namespace NVMe 命名空间
type Namespace struct {
	ID         int    `json:"id"`
	DevicePath string `json:"device_path"`
	SizeBytes  int64  `json:"size_bytes"`
	BlockSize  int    `json:"block_size"`
	UUID       string `json:"uuid"`
}

// NVMeConnection NVMe 连接
type NVMeConnection struct {
	HostIP      string    `json:"host_ip"`
	TargetID    string    `json:"target_id"`
	Transport   TransportType `json:"transport"`
	ConnectedAt time.Time `json:"connected_at"`
	IOPS        int64     `json:"iops"`
	Bandwidth   int64     `json:"bandwidth_mbps"`
	LatencyUs   float64   `json:"latency_us"`
}

// FabricManager NVMe Fabric 管理器
type FabricManager struct {
	mu          sync.RWMutex
	targets     map[string]*NVMeTarget
	connections map[string][]*NVMeConnection
}

// NewFabricManager 创建 Fabric 管理器
func NewFabricManager() *FabricManager {
	return &FabricManager{
		targets:     make(map[string]*NVMeTarget),
		connections: make(map[string][]*NVMeConnection),
	}
}

// CreateTarget 创建 NVMe 目标
func (fm *FabricManager) CreateTarget(target *NVMeTarget) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.targets[target.ID]; exists {
		return fmt.Errorf("target %s already exists", target.ID)
	}

	target.State = TargetStateActive
	target.CreatedAt = time.Now()
	fm.targets[target.ID] = target
	return nil
}

// DeleteTarget 删除 NVMe 目标
func (fm *FabricManager) DeleteTarget(id string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.targets[id]; !exists {
		return fmt.Errorf("target %s not found", id)
	}

	if conns := fm.connections[id]; len(conns) > 0 {
		return fmt.Errorf("target %s has %d active connections, disconnect first", id, len(conns))
	}

	delete(fm.targets, id)
	return nil
}

// GetTarget 获取目标
func (fm *FabricManager) GetTarget(id string) (*NVMeTarget, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	t, ok := fm.targets[id]
	return t, ok
}

// ListTargets 列出所有目标
func (fm *FabricManager) ListTargets(transport TransportType) []*NVMeTarget {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	targets := make([]*NVMeTarget, 0)
	for _, t := range fm.targets {
		if transport == "" || t.Transport == transport {
			targets = append(targets, t)
		}
	}
	return targets
}

// ConnectHost 连接主机
func (fm *FabricManager) ConnectHost(targetID, hostIP string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	target, exists := fm.targets[targetID]
	if !exists {
		return fmt.Errorf("target %s not found", targetID)
	}

	if target.State != TargetStateActive {
		return fmt.Errorf("target %s is not active", targetID)
	}

	conn := &NVMeConnection{
		HostIP:      hostIP,
		TargetID:    targetID,
		Transport:   target.Transport,
		ConnectedAt: time.Now(),
	}
	fm.connections[targetID] = append(fm.connections[targetID], conn)
	target.ConnectedHosts = append(target.ConnectedHosts, hostIP)
	return nil
}

// DisconnectHost 断开主机
func (fm *FabricManager) DisconnectHost(targetID, hostIP string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	conns := fm.connections[targetID]
	for i, c := range conns {
		if c.HostIP == hostIP {
			fm.connections[targetID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	target, exists := fm.targets[targetID]
	if exists {
		for i, h := range target.ConnectedHosts {
			if h == hostIP {
				target.ConnectedHosts = append(target.ConnectedHosts[:i], target.ConnectedHosts[i+1:]...)
				break
			}
		}
	}
	return nil
}

// GetConnections 获取目标的所有连接
func (fm *FabricManager) GetConnections(targetID string) []*NVMeConnection {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.connections[targetID]
}

// AddNamespace 添加命名空间
func (fm *FabricManager) AddNamespace(targetID string, ns Namespace) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	target, exists := fm.targets[targetID]
	if !exists {
		return fmt.Errorf("target %s not found", targetID)
	}

	if len(target.Namespaces) >= target.MaxNamespaces {
		return fmt.Errorf("target %s reached max namespaces (%d)", targetID, target.MaxNamespaces)
	}

	target.Namespaces = append(target.Namespaces, ns)
	return nil
}

// Stats 统计信息
type Stats struct {
	TotalTargets    int `json:"total_targets"`
	TCPCount        int `json:"tcp_count"`
	RDMACount       int `json:"rdma_count"`
	TotalConnections int `json:"total_connections"`
	TotalNamespaces int `json:"total_namespaces"`
}

// GetStats 获取统计
func (fm *FabricManager) GetStats() Stats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats := Stats{}
	for _, t := range fm.targets {
		stats.TotalTargets++
		switch t.Transport {
		case TransportTCP:
			stats.TCPCount++
		case TransportRDMA:
			stats.RDMACount++
		}
		stats.TotalNamespaces += len(t.Namespaces)
	}
	for _, conns := range fm.connections {
		stats.TotalConnections += len(conns)
	}
	return stats
}
