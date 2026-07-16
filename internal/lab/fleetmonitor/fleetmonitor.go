// Package fleetmonitor provides fleet-level monitoring for NAS-OS
// fleetmonitor.go - Multi-node monitoring and cluster management
package fleetmonitor

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// NodeStatus represents the status of a node.
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusWarning NodeStatus = "warning"
	NodeStatusError   NodeStatus = "error"
)

// NodeType represents the type of node.
type NodeType string

const (
	NodeTypePrimary   NodeType = "primary"
	NodeTypeSecondary NodeType = "secondary"
	NodeTypeEdge      NodeType = "edge"
	NodeTypeWorker    NodeType = "worker"
)

// Node represents a monitored node.
type Node struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	IPAddress string            `json:"ip_address"`
	Type      NodeType          `json:"type"`
	Status    NodeStatus        `json:"status"`
	CPU       CPUInfo           `json:"cpu"`
	Memory    MemoryInfo        `json:"memory"`
	Disks     []DiskInfo        `json:"disks"`
	Network   NetworkInfo       `json:"network"`
	Uptime    time.Duration     `json:"uptime"`
	LastSeen  time.Time         `json:"last_seen"`
	Metadata  map[string]string `json:"metadata"`
	Tags      []string          `json:"tags"`
}

// CPUInfo represents CPU information.
type CPUInfo struct {
	Model       string     `json:"model"`
	Cores       int        `json:"cores"`
	Threads     int        `json:"threads"`
	Usage       float64    `json:"usage"`       // 0-100
	Temperature float64    `json:"temperature"` // Celsius
	LoadAvg     [3]float64 `json:"load_avg"`    // 1, 5, 15 min
}

// MemoryInfo represents memory information.
type MemoryInfo struct {
	Total     uint64  `json:"total"`      // bytes
	Used      uint64  `json:"used"`       // bytes
	Available uint64  `json:"available"`  // bytes
	Usage     float64 `json:"usage"`      // 0-100
	SwapTotal uint64  `json:"swap_total"` // bytes
	SwapUsed  uint64  `json:"swap_used"`  // bytes
}

// DiskInfo represents disk information.
type DiskInfo struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mount_point"`
	FileSystem  string  `json:"file_system"`
	Total       uint64  `json:"total"`       // bytes
	Used        uint64  `json:"used"`        // bytes
	Available   uint64  `json:"available"`   // bytes
	Usage       float64 `json:"usage"`       // 0-100
	Temperature float64 `json:"temperature"` // Celsius
	Health      string  `json:"health"`
}

// NetworkInfo represents network information.
type NetworkInfo struct {
	Interfaces  []NetworkInterface `json:"interfaces"`
	TotalRx     uint64             `json:"total_rx"` // bytes
	TotalTx     uint64             `json:"total_tx"` // bytes
	Connections int                `json:"connections"`
}

// NetworkInterface represents a network interface.
type NetworkInterface struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	MAC     string `json:"mac"`
	Speed   uint64 `json:"speed"` // Mbps
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
	Status  string `json:"status"`
}

// AlertLevel represents the severity of an alert.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert represents a monitoring alert.
type Alert struct {
	ID         string     `json:"id"`
	NodeID     string     `json:"node_id"`
	Level      AlertLevel `json:"level"`
	Category   string     `json:"category"`
	Message    string     `json:"message"`
	Details    string     `json:"details,omitempty"`
	Timestamp  time.Time  `json:"timestamp"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ClusterStats represents cluster-level statistics.
type ClusterStats struct {
	TotalNodes       int           `json:"total_nodes"`
	OnlineNodes      int           `json:"online_nodes"`
	OfflineNodes     int           `json:"offline_nodes"`
	WarningNodes     int           `json:"warning_nodes"`
	TotalCPU         int           `json:"total_cpu"`
	AvgCPUUsage      float64       `json:"avg_cpu_usage"`
	TotalMemory      uint64        `json:"total_memory"`
	TotalMemoryUsed  uint64        `json:"total_memory_used"`
	AvgMemoryUsage   float64       `json:"avg_memory_usage"`
	TotalStorage     uint64        `json:"total_storage"`
	TotalStorageUsed uint64        `json:"total_storage_used"`
	AvgStorageUsage  float64       `json:"avg_storage_usage"`
	ActiveAlerts     int           `json:"active_alerts"`
	Uptime           time.Duration `json:"uptime"`
}

// Monitor manages fleet monitoring.
type Monitor struct {
	nodes     map[string]*Node
	alerts    map[string]*Alert
	stats     *ClusterStats
	mu        sync.RWMutex
	startTime time.Time
	hooks     []func(*Alert)
}

// NewMonitor creates a new monitor instance.
func NewMonitor() *Monitor {
	m := &Monitor{
		nodes:     make(map[string]*Node),
		alerts:    make(map[string]*Alert),
		stats:     &ClusterStats{},
		startTime: time.Now(),
		hooks:     make([]func(*Alert), 0),
	}
	go m.collectStats()
	return m
}

// RegisterNode registers a new node for monitoring.
func (m *Monitor) RegisterNode(node *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	node.LastSeen = time.Now()
	if node.Status == "" {
		node.Status = NodeStatusOnline
	}

	m.nodes[node.ID] = node
	log.Printf("Node registered: %s (%s)", node.Name, node.ID)
	return nil
}

// UnregisterNode removes a node from monitoring.
func (m *Monitor) UnregisterNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[nodeID]; !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	delete(m.nodes, nodeID)
	log.Printf("Node unregistered: %s", nodeID)
	return nil
}

// UpdateNodeStatus updates a node's status and metrics.
func (m *Monitor) UpdateNodeStatus(nodeID string, node *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	existing.Status = node.Status
	existing.CPU = node.CPU
	existing.Memory = node.Memory
	existing.Disks = node.Disks
	existing.Network = node.Network
	existing.LastSeen = time.Now()

	// Check for alerts
	m.checkNodeAlerts(existing)
	return nil
}

// checkNodeAlerts checks for alert conditions on a node.
func (m *Monitor) checkNodeAlerts(node *Node) {
	// CPU alert
	if node.CPU.Usage > 90 {
		m.createAlert(node.ID, AlertLevelWarning, "cpu",
			fmt.Sprintf("High CPU usage on %s: %.1f%%", node.Name, node.CPU.Usage))
	}

	// Memory alert
	if node.Memory.Usage > 90 {
		m.createAlert(node.ID, AlertLevelWarning, "memory",
			fmt.Sprintf("High memory usage on %s: %.1f%%", node.Name, node.Memory.Usage))
	}

	// Disk alerts
	for _, disk := range node.Disks {
		if disk.Usage > 90 {
			m.createAlert(node.ID, AlertLevelWarning, "disk",
				fmt.Sprintf("High disk usage on %s (%s): %.1f%%", node.Name, disk.Device, disk.Usage))
		}
		if disk.Temperature > 60 {
			m.createAlert(node.ID, AlertLevelError, "disk",
				fmt.Sprintf("High disk temperature on %s (%s): %.1f°C", node.Name, disk.Device, disk.Temperature))
		}
	}
}

// createAlert creates a new alert.
func (m *Monitor) createAlert(nodeID string, level AlertLevel, category, message string) {
	alert := &Alert{
		ID:        fmt.Sprintf("%s-%s-%d", nodeID, category, time.Now().UnixNano()),
		NodeID:    nodeID,
		Level:     level,
		Category:  category,
		Message:   message,
		Timestamp: time.Now(),
	}

	m.alerts[alert.ID] = alert
	log.Printf("Alert created: %s", message)

	// Notify hooks
	for _, hook := range m.hooks {
		go hook(alert)
	}
}

// AddAlertHook adds a hook for alert notifications.
func (m *Monitor) AddAlertHook(hook func(*Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook)
}

// GetNode returns a node by ID.
func (m *Monitor) GetNode(nodeID string) (*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	return node, nil
}

// ListNodes returns all monitored nodes.
func (m *Monitor) ListNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetAlerts returns alerts for a node or all alerts.
func (m *Monitor) GetAlerts(nodeID string, resolved bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, alert := range m.alerts {
		if (nodeID == "" || alert.NodeID == nodeID) && alert.Resolved == resolved {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// ResolveAlert marks an alert as resolved.
func (m *Monitor) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Resolved = true
	now := time.Now()
	alert.ResolvedAt = &now
	return nil
}

// collectStats periodically collects cluster statistics.
func (m *Monitor) collectStats() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		m.stats = &ClusterStats{
			TotalNodes:   len(m.nodes),
			Uptime:       time.Since(m.startTime),
			ActiveAlerts: 0,
		}

		var totalCPU, totalMemUsed, totalMemTotal, totalStorageUsed, totalStorageTotal uint64
		var memCount, storageCount int

		for _, node := range m.nodes {
			switch node.Status {
			case NodeStatusOnline:
				m.stats.OnlineNodes++
			case NodeStatusOffline:
				m.stats.OfflineNodes++
			case NodeStatusWarning:
				m.stats.WarningNodes++
			}

			m.stats.TotalCPU += node.CPU.Cores
			totalCPU += uint64(node.CPU.Usage * float64(node.CPU.Cores))

			totalMemTotal += node.Memory.Total
			totalMemUsed += node.Memory.Used
			memCount++

			for _, disk := range node.Disks {
				totalStorageTotal += disk.Total
				totalStorageUsed += disk.Used
				storageCount++
			}
		}

		if m.stats.TotalCPU > 0 {
			m.stats.AvgCPUUsage = float64(totalCPU) / float64(m.stats.TotalCPU)
		}
		if memCount > 0 {
			m.stats.TotalMemory = totalMemTotal
			m.stats.TotalMemoryUsed = totalMemUsed
			m.stats.AvgMemoryUsage = float64(totalMemUsed) / float64(totalMemTotal) * 100
		}
		if storageCount > 0 {
			m.stats.TotalStorage = totalStorageTotal
			m.stats.TotalStorageUsed = totalStorageUsed
			m.stats.AvgStorageUsage = float64(totalStorageUsed) / float64(totalStorageTotal) * 100
		}

		for _, alert := range m.alerts {
			if !alert.Resolved {
				m.stats.ActiveAlerts++
			}
		}

		m.mu.Unlock()
	}
}

// GetStats returns cluster statistics.
func (m *Monitor) GetStats() *ClusterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// RegisterRoutes registers HTTP routes for the monitor API.
func RegisterRoutes(mux *http.ServeMux, monitor *Monitor) {
	mux.HandleFunc("/api/fleet/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			nodes := monitor.ListNodes()
			json.NewEncoder(w).Encode(nodes)
		case http.MethodPost:
			var node Node
			if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := monitor.RegisterNode(&node); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(node)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/fleet/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := monitor.GetStats()
		json.NewEncoder(w).Encode(stats)
	})

	mux.HandleFunc("/api/fleet/alerts", func(w http.ResponseWriter, r *http.Request) {
		alerts := monitor.GetAlerts("", false)
		json.NewEncoder(w).Encode(alerts)
	})
}
