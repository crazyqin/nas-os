// Package nvmeof provides NVMe over Fabric target service
// 对标TrueNAS 25.10 NVMe/TCP功能
package nvmeof

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"
)

// TransportType defines NVMe-oF transport type
type TransportType string

const (
	TransportTCP  TransportType = "tcp"
	TransportRDMA TransportType = "rdma"
)

// Subsystem represents an NVMe subsystem
type Subsystem struct {
	NQN          string
	MaxNS        int
	Namespaces   []*Namespace
	Ports        []*Port
	AllowAnyHost bool
	AllowedHosts []string
}

// Namespace represents an NVMe namespace
type Namespace struct {
	NSID    int
	Device  string
	Size    uint64
	BlockSize uint
	ANAGroup int
}

// Port represents an NVMe-oF port
type Port struct {
	PortID    int
	Transport TransportType
	IP        string
	PortNum   int
}

// TargetService manages NVMe-oF target
type TargetService struct {
	subsystems map[string]*Subsystem
	transports map[string]*Port
	mu         sync.RWMutex
}

// NewTargetService creates a new NVMe-oF target service
func NewTargetService() *TargetService {
	return &TargetService{
		subsystems: make(map[string]*Subsystem),
		transports: make(map[string]*Port),
	}
}

// CreateSubsystem creates a new NVMe subsystem
func (t *TargetService) CreateSubsystem(ctx context.Context, nqn string, maxNS int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.subsystems[nqn]; exists {
		return fmt.Errorf("subsystem %s already exists", nqn)
	}

	// Create subsystem using nvmet CLI
	cmd := exec.CommandContext(ctx, "nvmetcli", "create", "subsystem", nqn)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create subsystem: %w", err)
	}

	t.subsystems[nqn] = &Subsystem{
		NQN:        nqn,
		MaxNS:      maxNS,
		Namespaces: make([]*Namespace, 0),
		Ports:      make([]*Port, 0),
	}

	return nil
}

// DeleteSubsystem deletes an NVMe subsystem
func (t *TargetService) DeleteSubsystem(ctx context.Context, nqn string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.subsystems[nqn]; !exists {
		return fmt.Errorf("subsystem %s not found", nqn)
	}

	// Delete subsystem
	cmd := exec.CommandContext(ctx, "nvmetcli", "delete", "subsystem", nqn)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete subsystem: %w", err)
	}

	delete(t.subsystems, nqn)
	return nil
}

// AddNamespace adds a namespace to a subsystem
func (t *TargetService) AddNamespace(ctx context.Context, nqn string, device string) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	subsys, exists := t.subsystems[nqn]
	if !exists {
		return 0, fmt.Errorf("subsystem %s not found", nqn)
	}

	nsid := len(subsys.Namespaces) + 1

	// Add namespace using nvmet CLI
	cmd := exec.CommandContext(ctx, "nvmetcli", "add", "namespace",
		"--subsystem", nqn,
		"--nsid", fmt.Sprintf("%d", nsid),
		"--device", device)
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed to add namespace: %w", err)
	}

	ns := &Namespace{
		NSID:   nsid,
		Device: device,
	}
	subsys.Namespaces = append(subsys.Namespaces, ns)

	return nsid, nil
}

// CreateTransport creates a NVMe-oF transport port
func (t *TargetService) CreateTransport(ctx context.Context, transport TransportType, ip string, port int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	portID := len(t.transports) + 1
	key := fmt.Sprintf("%s-%s-%d", transport, ip, port)

	if _, exists := t.transports[key]; exists {
		return fmt.Errorf("transport %s already exists", key)
	}

	// Create port using nvmet CLI
	cmd := exec.CommandContext(ctx, "nvmetcli", "create", "port",
		"--transport", string(transport),
		"--port", fmt.Sprintf("%d", portID),
		"--ip", ip,
		"--portnum", fmt.Sprintf("%d", port))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	t.transports[key] = &Port{
		PortID:    portID,
		Transport: transport,
		IP:        ip,
		PortNum:   port,
	}

	return nil
}

// ListSubsystems returns all subsystems
func (t *TargetService) ListSubsystems() []*Subsystem {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Subsystem, 0, len(t.subsystems))
	for _, subsys := range t.subsystems {
		result = append(result, subsys)
	}
	return result
}

// GetSubsystem returns a specific subsystem
func (t *TargetService) GetSubsystem(nqn string) (*Subsystem, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	subsys, exists := t.subsystems[nqn]
	if !exists {
		return nil, fmt.Errorf("subsystem %s not found", nqn)
	}
	return subsys, nil
}

// CheckRDMAAvailable checks if RDMA is available on the system
func CheckRDMAAvailable() bool {
	// Check for RDMA devices
	if _, err := net.InterfaceByName("ib0"); err == nil {
		return true
	}
	// Check for RoCE devices
	if _, err := net.InterfaceByName("rocep0s8f0"); err == nil {
		return true
	}
	return false
}

// PerformanceStats returns NVMe-oF performance statistics
type PerformanceStats struct {
	ReadIOPS    uint64
	WriteIOPS   uint64
	ReadBps     uint64
	WriteBps    uint64
	LatencyAvg  float64
	Connections int
}

// GetPerformanceStats returns current performance stats
func (t *TargetService) GetPerformanceStats(ctx context.Context) (*PerformanceStats, error) {
	//TODO: Implement actual stats collection from nvmet
	return &PerformanceStats{
		Connections: len(t.transports),
	}, nil
}