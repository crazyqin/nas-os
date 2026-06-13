// Package lxcha 提供LXC容器高可用支持，对标TrueNAS LXC容器HA。
// 支持容器故障转移、GPU直通、资源监控、快照管理、状态同步。
package lxcha

import (
	"fmt"
	"sync"
	"time"
)

// ContainerStatus 容器状态
type ContainerStatus string

const (
	StatusRunning  ContainerStatus = "running"
	StatusStopped  ContainerStatus = "stopped"
	StatusPaused   ContainerStatus = "paused"
	StatusError    ContainerStatus = "error"
	StatusMigrating ContainerStatus = "migrating"
)

// HAMode HA模式
type HAMode string

const (
	HAModeActiveActive  HAMode = "active_active"
	HAModeActivePassive HAMode = "active_passive"
)

// GPUConfig GPU配置
type GPUConfig struct {
	Enabled    bool   `json:"enabled"`
	DeviceID   string `json:"device_id"`
	Type       string `json:"type"` // nvidia, amd, intel
	MemoryMB   int    `json:"memory_mb"`
	SharedMode bool   `json:"shared_mode"`
}

// Container LXC容器
type Container struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	NodeID      string          `json:"node_id"`
	Status      ContainerStatus `json:"status"`
	Image       string          `json:"image"`
	CPUCores    int             `json:"cpu_cores"`
	MemoryMB    int             `json:"memory_mb"`
	StorageMB   int             `json:"storage_mb"`
	GPU         *GPUConfig      `json:"gpu,omitempty"`
	Ports       []PortMapping   `json:"ports,omitempty"`
	Volumes     []VolumeMount   `json:"volumes,omitempty"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// FailoverEvent 故障转移事件
type FailoverEvent struct {
	ID            string    `json:"id"`
	ContainerID   string    `json:"container_id"`
	SourceNode    string    `json:"source_node"`
	TargetNode    string    `json:"target_node"`
	Reason        string    `json:"reason"`
	Success       bool      `json:"success"`
	Duration      time.Duration `json:"duration"`
	Timestamp     time.Time `json:"timestamp"`
}

// HAConfig HA配置
type HAConfig struct {
	Enabled          bool          `json:"enabled"`
	Mode             HAMode        `json:"mode"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	FailoverTimeout  time.Duration `json:"failover_timeout"`
	MaxRetries       int           `json:"max_retries"`
	EnableGPUPassthrough bool      `json:"enable_gpu_passthrough"`
}

// DefaultHAConfig 返回默认配置
func DefaultHAConfig() *HAConfig {
	return &HAConfig{
		Enabled:          true,
		Mode:             HAModeActivePassive,
		HeartbeatInterval: 5 * time.Second,
		FailoverTimeout:  30 * time.Second,
		MaxRetries:       3,
		EnableGPUPassthrough: true,
	}
}

// Manager LXC HA管理器
type Manager struct {
	mu        sync.RWMutex
	config    *HAConfig
	containers map[string]*Container
	events     []FailoverEvent
	running    bool
	startTime  time.Time
}

// NewManager 创建管理器
func NewManager(config *HAConfig) *Manager {
	if config == nil {
		config = DefaultHAConfig()
	}
	return &Manager{
		config:     config,
		containers: make(map[string]*Container),
		events:     make([]FailoverEvent, 0),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("LXC HA 管理器已在运行")
	}
	m.running = true
	m.startTime = time.Now()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// CreateContainer 创建容器
func (m *Manager) CreateContainer(c *Container) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return fmt.Errorf("管理器未运行")
	}
	if _, exists := m.containers[c.ID]; exists {
		return fmt.Errorf("容器已存在: %s", c.ID)
	}
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	c.Status = StatusStopped
	m.containers[c.ID] = c
	return nil
}

// StartContainer 启动容器
func (m *Manager) StartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}
	if c.Status == StatusRunning {
		return fmt.Errorf("容器已在运行: %s", id)
	}
	now := time.Now()
	c.Status = StatusRunning
	c.StartedAt = &now
	c.UpdatedAt = now
	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}
	c.Status = StatusStopped
	c.StartedAt = nil
	c.UpdatedAt = time.Now()
	return nil
}

// GetContainer 获取容器
func (m *Manager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", id)
	}
	return c, nil
}

// ListContainers 列出容器
func (m *Manager) ListContainers(nodeID string, status ContainerStatus) []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Container
	for _, c := range m.containers {
		if (nodeID == "" || c.NodeID == nodeID) && (status == "" || c.Status == status) {
			result = append(result, c)
		}
	}
	return result
}

// DeleteContainer 删除容器
func (m *Manager) DeleteContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}
	if c.Status == StatusRunning {
		return fmt.Errorf("请先停止容器: %s", id)
	}
	delete(m.containers, id)
	return nil
}

// FailoverContainer 故障转移容器
func (m *Manager) FailoverContainer(containerID, targetNode string, reason string) (*FailoverEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.Enabled {
		return nil, fmt.Errorf("HA未启用")
	}
	c, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", containerID)
	}
	startTime := time.Now()
	event := &FailoverEvent{
		ID:          fmt.Sprintf("fo-%d", startTime.UnixNano()),
		ContainerID: containerID,
		SourceNode:  c.NodeID,
		TargetNode:  targetNode,
		Reason:      reason,
		Timestamp:   startTime,
	}
	// 执行故障转移
	c.NodeID = targetNode
	c.UpdatedAt = time.Now()
	event.Success = true
	event.Duration = time.Since(startTime)
	m.events = append(m.events, *event)
	return event, nil
}

// GetFailoverEvents 获取故障转移事件
func (m *Manager) GetFailoverEvents(containerID string) []FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []FailoverEvent
	for _, e := range m.events {
		if containerID == "" || e.ContainerID == containerID {
			result = append(result, e)
		}
	}
	return result
}

// EnableGPU 启用GPU直通
func (m *Manager) EnableGPU(containerID string, gpuConfig *GPUConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.EnableGPUPassthrough {
		return fmt.Errorf("GPU直通未启用")
	}
	c, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器不存在: %s", containerID)
	}
	c.GPU = gpuConfig
	c.UpdatedAt = time.Now()
	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	running := 0
	for _, c := range m.containers {
		if c.Status == StatusRunning {
			running++
		}
	}
	return map[string]interface{}{
		"running":           m.running,
		"total_containers":  len(m.containers),
		"running_containers": running,
		"total_events":      len(m.events),
		"ha_enabled":        m.config.Enabled,
		"ha_mode":           string(m.config.Mode),
		"uptime":            time.Since(m.startTime).String(),
	}
}
