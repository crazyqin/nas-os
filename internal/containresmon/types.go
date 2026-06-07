// Package containresmon provides container resource monitoring and alerting for NAS-OS
// Features: Per-container CPU/memory/disk/NET tracking, resource limits, alerting
// Competitor benchmark: 对标群晖Container Manager, 超越TrueNAS应用资源监控
package containresmon

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ContainerStatus represents container status
type ContainerStatus string

const (
	ContainerRunning ContainerStatus = "running"
	ContainerStopped ContainerStatus = "stopped"
	ContainerPaused  ContainerStatus = "paused"
	ContainerUnknown ContainerStatus = "unknown"
)

// AlertType represents resource alert type
type AlertType string

const (
	AlertCPUHigh     AlertType = "cpu_high"
	AlertMemHigh     AlertType = "memory_high"
	AlertDiskHigh    AlertType = "disk_high"
	AlertNetHigh     AlertType = "network_high"
	AlertRestartLoop AlertType = "restart_loop"
	AlertHealthCheck AlertType = "health_check_failed"
)

// Container represents a monitored container
type Container struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Image        string          `json:"image"`
	Status       ContainerStatus `json:"status"`
	Ports        []string        `json:"ports"`
	Networks     []string        `json:"networks"`
	CPULimit     float64         `json:"cpu_limit_percent"`
	MemLimit     int64           `json:"memory_limit_bytes"`
	DiskLimit    int64           `json:"disk_limit_bytes"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    time.Time       `json:"started_at"`
	RestartCount int             `json:"restart_count"`
}

// ResourceUsage represents resource usage at a point in time
type ResourceUsage struct {
	ContainerID string    `json:"container_id"`
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemUsage    int64     `json:"memory_usage_bytes"`
	MemLimit    int64     `json:"memory_limit_bytes"`
	MemPercent  float64   `json:"memory_percent"`
	DiskRead    int64     `json:"disk_read_bytes"`
	DiskWrite   int64     `json:"disk_write_bytes"`
	NetRx       int64     `json:"net_rx_bytes"`
	NetTx       int64     `json:"net_tx_bytes"`
	PIDs        int       `json:"pids"`
}

// ResourceAlert represents a resource alert
type ResourceAlert struct {
	ID            string    `json:"id"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	Type          AlertType `json:"type"`
	Message       string    `json:"message"`
	Threshold     float64   `json:"threshold"`
	CurrentValue  float64   `json:"current_value"`
	Resolved      bool      `json:"resolved"`
	Timestamp     time.Time `json:"timestamp"`
	ResolvedAt    time.Time `json:"resolved_at,omitempty"`
}

// ResourcePolicy represents resource policy for a container
type ResourcePolicy struct {
	ContainerID      string  `json:"container_id"`
	CPUWarning       float64 `json:"cpu_warning_percent"`
	CPUCritical      float64 `json:"cpu_critical_percent"`
	MemWarning       float64 `json:"mem_warning_percent"`
	MemCritical      float64 `json:"mem_critical_percent"`
	DiskWarning      float64 `json:"disk_warning_percent"`
	NetWarningBps    int64   `json:"net_warning_bps"`
	AutoRestart      bool    `json:"auto_restart"`
	MaxRestarts      int     `json:"max_restarts"`
	AlertCooldownMin int     `json:"alert_cooldown_minutes"`
}

// MonitorStats represents container monitor statistics
type MonitorStats struct {
	TotalContainers   int     `json:"total_containers"`
	RunningContainers int     `json:"running_containers"`
	StoppedContainers int     `json:"stopped_containers"`
	TotalAlerts       int     `json:"total_alerts"`
	UnresolvedAlerts  int     `json:"unresolved_alerts"`
	TotalCPUUsage     float64 `json:"total_cpu_percent"`
	TotalMemUsage     int64   `json:"total_memory_bytes"`
	TotalNetRx        int64   `json:"total_net_rx_bytes"`
	TotalNetTx        int64   `json:"total_net_tx_bytes"`
}

// Config holds container resource monitor configuration
type Config struct {
	Enabled            bool    `json:"enabled"`
	MonitorIntervalSec int     `json:"monitor_interval_seconds"`
	HistoryRetentionH  int     `json:"history_retention_hours"`
	DefaultCPUWarning  float64 `json:"default_cpu_warning"`
	DefaultCPUCritical float64 `json:"default_cpu_critical"`
	DefaultMemWarning  float64 `json:"default_mem_warning"`
	DefaultMemCritical float64 `json:"default_mem_critical"`
	AlertCooldownMin   int     `json:"alert_cooldown_minutes"`
}

// Manager manages container resource monitoring
type Manager struct {
	config     *Config
	containers map[string]*Container
	usage      map[string][]*ResourceUsage
	alerts     []*ResourceAlert
	policies   map[string]*ResourcePolicy
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager creates a new container resource monitor manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:     config,
		containers: make(map[string]*Container),
		usage:      make(map[string][]*ResourceUsage),
		alerts:     make([]*ResourceAlert, 0),
		policies:   make(map[string]*ResourcePolicy),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the container resource monitor
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return fmt.Errorf("container resource monitor is disabled")
	}
	return nil
}

// Stop stops the container resource monitor
func (m *Manager) Stop() {
	m.cancel()
}

// RegisterContainer registers a container for monitoring
func (m *Manager) RegisterContainer(c *Container) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.containers[c.ID] = c
}

// UnregisterContainer unregisters a container
func (m *Manager) UnregisterContainer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.containers, id)
	delete(m.usage, id)
	delete(m.policies, id)
}

// ListContainers returns all monitored containers
func (m *Manager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		containers = append(containers, c)
	}
	return containers
}

// RecordUsage records resource usage for a container
func (m *Manager) RecordUsage(usage *ResourceUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	usage.Timestamp = time.Now()
	m.usage[usage.ContainerID] = append(m.usage[usage.ContainerID], usage)
}

// GetUsageHistory returns usage history for a container
func (m *Manager) GetUsageHistory(containerID string) []*ResourceUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.usage[containerID]
}

// SetPolicy sets a resource policy for a container
func (m *Manager) SetPolicy(policy *ResourcePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies[policy.ContainerID] = policy
}

// ListAlerts returns all resource alerts
func (m *Manager) ListAlerts() []*ResourceAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// ResolveAlert resolves a resource alert
func (m *Manager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id {
			a.Resolved = true
			a.ResolvedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", id)
}

// GetStats returns monitor statistics
func (m *Manager) GetStats() *MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &MonitorStats{
		TotalContainers: len(m.containers),
	}

	for _, c := range m.containers {
		switch c.Status {
		case ContainerRunning:
			stats.RunningContainers++
		case ContainerStopped:
			stats.StoppedContainers++
		}
	}

	stats.TotalAlerts = len(m.alerts)
	for _, a := range m.alerts {
		if !a.Resolved {
			stats.UnresolvedAlerts++
		}
	}

	// Get latest usage for running containers
	for id, usages := range m.usage {
		if len(usages) > 0 {
			c, ok := m.containers[id]
			if ok && c.Status == ContainerRunning {
				last := usages[len(usages)-1]
				stats.TotalCPUUsage += last.CPUPercent
				stats.TotalMemUsage += last.MemUsage
				stats.TotalNetRx += last.NetRx
				stats.TotalNetTx += last.NetTx
			}
		}
	}

	return stats
}
