// Package nvmeof provides NVMe over Fabric (NVMe/TCP) target functionality.
// Implements NVMe/TCP protocol for high-performance storage networking.
package nvmeof

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Subsystem represents an NVMe subsystem configuration.
type Subsystem struct {
	NQN            string    `json:"nqn"`             // NVMe Qualified Name
	SerialNumber   string    `json:"serial_number"`
	ModelNumber    string    `json:"model_number"`
	MaxNamespaces  int       `json:"max_namespaces"`
	AllowAnyHost   bool      `json:"allow_any_host"`
	Hosts          []string  `json:"allowed_hosts"`   // Allowed host NQNs
	NamespaceIDs   []int     `json:"namespace_ids"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Namespace represents an NVMe namespace (LUN equivalent).
type Namespace struct {
	ID            int       `json:"id"`
	Path          string    `json:"path"`           // Backing device/file path
	Size          int64     `json:"size"`           // Size in bytes
	BlockSize     int       `json:"block_size"`     // Typically 512 or 4096
	ReadOnly      bool      `json:"read_only"`
	SubsystemNQN  string    `json:"subsystem_nqn"`
	CreatedAt     time.Time `json:"created_at"`
}

// Port represents an NVMe/TCP listener port.
type Port struct {
	ID          string    `json:"id"`
	Transport   string    `json:"transport"`      // "tcp" or "rdma"
	Address     string    `json:"address"`        // IP address
	Port        int       `json:"port"`           // TCP port (default 4420)
	Status      string    `json:"status"`         // "listening", "stopped"
	Connections int       `json:"connections"`
	CreatedAt   time.Time `json:"created_at"`
}

// Connection represents an active NVMe/TCP connection.
type Connection struct {
	ID           string    `json:"id"`
	HostNQN      string    `json:"host_nqn"`
	SubsystemNQN string    `json:"subsystem_nqn"`
	PortID       string    `json:"port_id"`
	RemoteAddr   string    `json:"remote_addr"`
	Status       string    `json:"status"`         // "active", "idle", "disconnected"
	ConnectedAt  time.Time `json:"connected_at"`
	LastActive   time.Time `json:"last_active"`
}

// Config holds NVMe-oF manager configuration.
type Config struct {
	DefaultPort    int    `json:"default_port"`     // Default NVMe/TCP port (4420)
	MaxConnections int    `json:"max_connections"`  // Max concurrent connections
	EnableRDMA     bool   `json:"enable_rdma"`      // Enable NVMe/RDMA (requires hardware)
	LogLevel       string `json:"log_level"`
	DataDir        string `json:"data_dir"`         // Path for config storage
}

// Manager manages NVMe over Fabric targets.
type Manager struct {
	mu          sync.RWMutex
	subsystems  map[string]*Subsystem
	namespaces  map[int]*Namespace
	ports       map[string]*Port
	connections map[string]*Connection
	config      *Config
	logger      *zap.Logger
	configPath  string
	listeners   map[string]net.Listener // Active TCP listeners
}

// NewManager creates a new NVMe-oF manager.
func NewManager(configPath string, logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &Config{
		DefaultPort:    4420,
		MaxConnections: 1024,
		EnableRDMA:     false,
		LogLevel:       "info",
		DataDir:        "/var/lib/nas-os/nvmeof",
	}

	m := &Manager{
		subsystems:  make(map[string]*Subsystem),
		namespaces:  make(map[int]*Namespace),
		ports:       make(map[string]*Port),
		connections: make(map[string]*Connection),
		config:      config,
		logger:      logger,
		configPath:  configPath,
		listeners:   make(map[string]net.Listener),
	}

	// Load existing config
	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return m, nil
}

// CreateSubsystem creates a new NVMe subsystem.
func (m *Manager) CreateSubsystem(ctx context.Context, nqn, serial, model string, maxNS int) (*Subsystem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.subsystems[nqn]; exists {
		return nil, fmt.Errorf("subsystem %s already exists", nqn)
	}

	// Validate NQN format
	if !isValidNQN(nqn) {
		return nil, fmt.Errorf("invalid NQN format: %s", nqn)
	}

	subsys := &Subsystem{
		NQN:           nqn,
		SerialNumber:  serial,
		ModelNumber:   model,
		MaxNamespaces: maxNS,
		AllowAnyHost:  false,
		Hosts:         []string{},
		NamespaceIDs:  []int{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	m.subsystems[nqn] = subsys
	m.logger.Info("Created NVMe subsystem",
		zap.String("nqn", nqn),
		zap.String("serial", serial))

	if err := m.saveConfig(); err != nil {
		m.logger.Error("Failed to save config", zap.Error(err))
	}

	return subsys, nil
}

// DeleteSubsystem removes an NVMe subsystem.
func (m *Manager) DeleteSubsystem(ctx context.Context, nqn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsys, exists := m.subsystems[nqn]
	if !exists {
		return fmt.Errorf("subsystem %s not found", nqn)
	}

	// Check for active namespaces
	if len(subsys.NamespaceIDs) > 0 {
		return fmt.Errorf("subsystem %s has active namespaces, remove them first", nqn)
	}

	// Check for active connections
	for _, conn := range m.connections {
		if conn.SubsystemNQN == nqn && conn.Status == "active" {
			return fmt.Errorf("subsystem %s has active connections", nqn)
		}
	}

	delete(m.subsystems, nqn)
	m.logger.Info("Deleted NVMe subsystem", zap.String("nqn", nqn))

	return m.saveConfig()
}

// AddNamespace adds a namespace to a subsystem.
func (m *Manager) AddNamespace(ctx context.Context, nqn string, path string, size int64, blockSize int) (*Namespace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsys, exists := m.subsystems[nqn]
	if !exists {
		return nil, fmt.Errorf("subsystem %s not found", nqn)
	}

	// Check namespace limit
	if len(subsys.NamespaceIDs) >= subsys.MaxNamespaces {
		return nil, fmt.Errorf("subsystem %s reached max namespaces (%d)", nqn, subsys.MaxNamespaces)
	}

	// Generate namespace ID
	nsID := 1
	for _, id := range subsys.NamespaceIDs {
		if id >= nsID {
			nsID = id + 1
		}
	}

	ns := &Namespace{
		ID:           nsID,
		Path:         path,
		Size:         size,
		BlockSize:    blockSize,
		ReadOnly:     false,
		SubsystemNQN: nqn,
		CreatedAt:    time.Now(),
	}

	m.namespaces[nsID] = ns
	subsys.NamespaceIDs = append(subsys.NamespaceIDs, nsID)
	subsys.UpdatedAt = time.Now()

	m.logger.Info("Added namespace to subsystem",
		zap.String("nqn", nqn),
		zap.Int("nsid", nsID),
		zap.String("path", path))

	return ns, m.saveConfig()
}

// RemoveNamespace removes a namespace from a subsystem.
func (m *Manager) RemoveNamespace(ctx context.Context, nqn string, nsID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsys, exists := m.subsystems[nqn]
	if !exists {
		return fmt.Errorf("subsystem %s not found", nqn)
	}

	// Check namespace exists
	if _, exists := m.namespaces[nsID]; !exists {
		return fmt.Errorf("namespace %d not found", nsID)
	}

	// Remove from subsystem
	newIDs := make([]int, 0, len(subsys.NamespaceIDs)-1)
	for _, id := range subsys.NamespaceIDs {
		if id != nsID {
			newIDs = append(newIDs, id)
		}
	}
	subsys.NamespaceIDs = newIDs
	subsys.UpdatedAt = time.Now()

	delete(m.namespaces, nsID)

	m.logger.Info("Removed namespace from subsystem",
		zap.String("nqn", nqn),
		zap.Int("nsid", nsID))

	return m.saveConfig()
}

// CreatePort creates and starts an NVMe/TCP listener.
func (m *Manager) CreatePort(ctx context.Context, transport, address string, port int) (*Port, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	portID := uuid.New().String()

	p := &Port{
		ID:        portID,
		Transport: transport,
		Address:   address,
		Port:      port,
		Status:    "stopped",
		CreatedAt: time.Now(),
	}

	// Start TCP listener
	if transport == "tcp" {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", address, port))
		if err != nil {
			return nil, fmt.Errorf("failed to start listener: %w", err)
		}
		m.listeners[portID] = listener
		p.Status = "listening"
		m.logger.Info("Started NVMe/TCP listener",
			zap.String("address", address),
			zap.Int("port", port))
	}

	m.ports[portID] = p
	return p, m.saveConfig()
}

// StopPort stops a listener port.
func (m *Manager) StopPort(ctx context.Context, portID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	port, exists := m.ports[portID]
	if !exists {
		return fmt.Errorf("port %s not found", portID)
	}

	if listener, exists := m.listeners[portID]; exists {
		if err := listener.Close(); err != nil {
			m.logger.Warn("Failed to close listener", zap.Error(err))
		}
		delete(m.listeners, portID)
	}

	port.Status = "stopped"
	port.Connections = 0

	m.logger.Info("Stopped NVMe port", zap.String("port_id", portID))
	return m.saveConfig()
}

// AllowHost adds a host NQN to the subsystem's allowed list.
func (m *Manager) AllowHost(ctx context.Context, subsystemNQN, hostNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsys, exists := m.subsystems[subsystemNQN]
	if !exists {
		return fmt.Errorf("subsystem %s not found", subsystemNQN)
	}

	// Check if already allowed
	for _, h := range subsys.Hosts {
		if h == hostNQN {
			return nil // Already allowed
		}
	}

	subsys.Hosts = append(subsys.Hosts, hostNQN)
	subsys.AllowAnyHost = false
	subsys.UpdatedAt = time.Now()

	m.logger.Info("Added allowed host to subsystem",
		zap.String("subsystem", subsystemNQN),
		zap.String("host", hostNQN))

	return m.saveConfig()
}

// GetSubsystem returns a subsystem by NQN.
func (m *Manager) GetSubsystem(nqn string) (*Subsystem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsys, exists := m.subsystems[nqn]
	if !exists {
		return nil, fmt.Errorf("subsystem %s not found", nqn)
	}

	return subsys, nil
}

// ListSubsystems returns all subsystems.
func (m *Manager) ListSubsystems() []*Subsystem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subsystem, 0, len(m.subsystems))
	for _, subsys := range m.subsystems {
		result = append(result, subsys)
	}
	return result
}

// RevokeHost removes a host NQN from the subsystem's allowed list.
func (m *Manager) RevokeHost(ctx context.Context, subsystemNQN, hostNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsys, exists := m.subsystems[subsystemNQN]
	if !exists {
		return fmt.Errorf("subsystem %s not found", subsystemNQN)
	}

	newHosts := make([]string, 0, len(subsys.Hosts))
	found := false
	for _, h := range subsys.Hosts {
		if h == hostNQN {
			found = true
		} else {
			newHosts = append(newHosts, h)
		}
	}

	if !found {
		return fmt.Errorf("host %s not found in subsystem %s", hostNQN, subsystemNQN)
	}

	subsys.Hosts = newHosts
	subsys.UpdatedAt = time.Now()

	m.logger.Info("Revoked host from subsystem",
		zap.String("subsystem", subsystemNQN),
		zap.String("host", hostNQN))

	return m.saveConfig()
}

// ListPorts returns all configured ports.
func (m *Manager) ListPorts() []*Port {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Port, 0, len(m.ports))
	for _, p := range m.ports {
		result = append(result, p)
	}
	return result
}

// RemoveHost is an alias for RevokeHost.
func (m *Manager) RemoveHost(ctx context.Context, subsystemNQN, hostNQN string) error {
	return m.RevokeHost(ctx, subsystemNQN, hostNQN)
}

// GetStats returns NVMe-oF statistics.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeConns := 0
	for _, conn := range m.connections {
		if conn.Status == "active" {
			activeConns++
		}
	}

	listeningPorts := 0
	for _, port := range m.ports {
		if port.Status == "listening" {
			listeningPorts++
		}
	}

	return map[string]interface{}{
		"subsystems":       len(m.subsystems),
		"namespaces":       len(m.namespaces),
		"ports":            len(m.ports),
		"listening_ports":  listeningPorts,
		"connections":      len(m.connections),
		"active_connections": activeConns,
		"rdma_enabled":     m.config.EnableRDMA,
	}
}

// isValidNQN validates NVMe Qualified Name format.
func isValidNQN(nqn string) bool {
	// NQN format: nqn.2014-08.org.nvmexpress:uuid:xxx or nqn.2014-08.org:name
	if len(nqn) < 10 {
		return false
	}
	if !strings.HasPrefix(nqn, "nqn.") {
		return false
	}
	// Basic validation - more detailed checks can be added
	return true
}

// loadConfig loads configuration from disk.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Subsystems  map[string]*Subsystem  `json:"subsystems"`
		Namespaces  map[int]*Namespace     `json:"namespaces"`
		Ports       map[string]*Port       `json:"ports"`
		Connections map[string]*Connection `json:"connections"`
		Config      *Config                `json:"config"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.subsystems = cfg.Subsystems
	m.namespaces = cfg.Namespaces
	m.ports = cfg.Ports
	m.connections = cfg.Connections
	if cfg.Config != nil {
		m.config = cfg.Config
	}

	return nil
}

// saveConfig saves configuration to disk.
func (m *Manager) saveConfig() error {
	cfg := struct {
		Subsystems  map[string]*Subsystem  `json:"subsystems"`
		Namespaces  map[int]*Namespace     `json:"namespaces"`
		Ports       map[string]*Port       `json:"ports"`
		Connections map[string]*Connection `json:"connections"`
		Config      *Config                `json:"config"`
	}{
		Subsystems:  m.subsystems,
		Namespaces:  m.namespaces,
		Ports:       m.ports,
		Connections: m.connections,
		Config:      m.config,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// Close shuts down the manager.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all listeners
	for portID, listener := range m.listeners {
		if err := listener.Close(); err != nil {
			m.logger.Warn("Failed to close listener", zap.Error(err))
		}
		if port, exists := m.ports[portID]; exists {
			port.Status = "stopped"
		}
	}
	m.listeners = make(map[string]net.Listener)

	return m.saveConfig()
}