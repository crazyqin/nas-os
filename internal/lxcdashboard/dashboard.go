package lxcdashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ContainerState 容器状态
type ContainerState string

const (
	StateRunning  ContainerState = "RUNNING"
	StateStopped  ContainerState = "STOPPED"
	StatePaused   ContainerState = "PAUSED"
	StateError    ContainerState = "ERROR"
	StateCreating ContainerState = "CREATING"
)

// Container 容器信息
type Container struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	State       ContainerState `json:"state"`
	Image       string         `json:"image"`
	IP          string         `json:"ip"`
	CPUPercent  float64        `json:"cpu_percent"`
	MemoryMB    int64          `json:"memory_mb"`
	MemoryLimit int64          `json:"memory_limit_mb"`
	DiskMB      int64          `json:"disk_mb"`
	NetworkIn   int64          `json:"network_in_bytes"`
	NetworkOut  int64          `json:"network_out_bytes"`
	Uptime      int64          `json:"uptime_seconds"`
	CreatedAt   time.Time      `json:"created_at"`
	Tags        []string       `json:"tags"`
	Privileged  bool           `json:"privileged"`
	Profile     string         `json:"security_profile"`
}

// Template 容器模板
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	CPULimit    float64           `json:"cpu_limit"`
	MemoryMB    int64             `json:"memory_mb"`
	DiskMB      int64             `json:"disk_mb"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	EnvVars     map[string]string `json:"env_vars"`
	Tags        []string          `json:"tags"`
}

type PortMapping struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// ResourceQuota 资源配额
type ResourceQuota struct {
	MaxCPU        float64 `json:"max_cpu"`
	MaxMemoryMB   int64   `json:"max_memory_mb"`
	MaxDiskMB     int64   `json:"max_disk_mb"`
	MaxContainers int     `json:"max_containers"`
}

// DashboardStats 仪表盘统计
type DashboardStats struct {
	TotalContainers   int     `json:"total_containers"`
	RunningContainers int     `json:"running_containers"`
	StoppedContainers int     `json:"stopped_containers"`
	TotalCPUUsage     float64 `json:"total_cpu_usage"`
	TotalMemoryMB     int64   `json:"total_memory_mb"`
	TotalDiskMB       int64   `json:"total_disk_mb"`
}

// LXCDashboard LXC容器仪表盘
type LXCDashboard struct {
	containers map[string]*Container
	templates  []*Template
	quotas     *ResourceQuota
	dataPath   string
	mu         sync.RWMutex
}

// NewLXCDashboard 创建仪表盘
func NewLXCDashboard(dataPath string) *LXCDashboard {
	os.MkdirAll(dataPath, 0755)
	d := &LXCDashboard{
		containers: make(map[string]*Container),
		dataPath:   dataPath,
		quotas: &ResourceQuota{
			MaxCPU:        4.0,
			MaxMemoryMB:   8192,
			MaxDiskMB:     102400,
			MaxContainers: 20,
		},
	}
	d.loadState()
	d.initTemplates()
	return d
}

// CreateContainer 创建容器
func (d *LXCDashboard) CreateContainer(name, image string, template *Template) (*Container, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.containers) >= d.quotas.MaxContainers {
		return nil, fmt.Errorf("max containers reached: %d", d.quotas.MaxContainers)
	}
	container := &Container{
		ID:        fmt.Sprintf("lxc-%d", time.Now().UnixNano()),
		Name:      name,
		State:     StateCreating,
		Image:     image,
		CreatedAt: time.Now(),
	}
	if template != nil {
		container.MemoryLimit = template.MemoryMB
		container.DiskMB = template.DiskMB
		container.Tags = template.Tags
	}
	d.containers[container.ID] = container
	d.saveState()
	return container, nil
}

// StartContainer 启动容器
func (d *LXCDashboard) StartContainer(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, ok := d.containers[id]
	if !ok {
		return fmt.Errorf("container not found")
	}
	c.State = StateRunning
	d.saveState()
	return nil
}

// StopContainer 停止容器
func (d *LXCDashboard) StopContainer(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, ok := d.containers[id]
	if !ok {
		return fmt.Errorf("container not found")
	}
	c.State = StateStopped
	d.saveState()
	return nil
}

// DeleteContainer 删除容器
func (d *LXCDashboard) DeleteContainer(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.containers, id)
	d.saveState()
	return nil
}

// UpdateMetrics 更新容器指标
func (d *LXCDashboard) UpdateMetrics(id string, cpu float64, memMB int64, netIn, netOut int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, ok := d.containers[id]
	if !ok {
		return
	}
	c.CPUPercent = cpu
	c.MemoryMB = memMB
	c.NetworkIn = netIn
	c.NetworkOut = netOut
}

// GetStats 获取统计
func (d *LXCDashboard) GetStats() DashboardStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	stats := DashboardStats{}
	for _, c := range d.containers {
		stats.TotalContainers++
		if c.State == StateRunning {
			stats.RunningContainers++
			stats.TotalCPUUsage += c.CPUPercent
			stats.TotalMemoryMB += c.MemoryMB
		} else {
			stats.StoppedContainers++
		}
		stats.TotalDiskMB += c.DiskMB
	}
	return stats
}

// ListContainers 列出容器
func (d *LXCDashboard) ListContainers(state *ContainerState) []*Container {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var result []*Container
	for _, c := range d.containers {
		if state != nil && c.State != *state {
			continue
		}
		result = append(result, c)
	}
	return result
}

// GetContainer 获取容器
func (d *LXCDashboard) GetContainer(id string) (*Container, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	c, ok := d.containers[id]
	return c, ok
}

// GetTemplates 获取模板
func (d *LXCDashboard) GetTemplates() []*Template {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.templates
}

func (d *LXCDashboard) initTemplates() {
	d.templates = []*Template{
		{ID: "web-server", Name: "Web服务器", Description: "Nginx/Apache Web服务器", Image: "nginx:alpine", CPULimit: 1.0, MemoryMB: 512, DiskMB: 2048},
		{ID: "database", Name: "数据库", Description: "PostgreSQL/MySQL数据库", Image: "postgres:16", CPULimit: 2.0, MemoryMB: 2048, DiskMB: 10240},
		{ID: "dev-env", Name: "开发环境", Description: "通用开发环境", Image: "ubuntu:22.04", CPULimit: 2.0, MemoryMB: 4096, DiskMB: 20480},
		{ID: "sandbox", Name: "安全沙箱", Description: "隔离的安全测试环境", Image: "alpine:latest", CPULimit: 0.5, MemoryMB: 256, DiskMB: 1024},
	}
}

func (d *LXCDashboard) saveState() {
	data, _ := json.MarshalIndent(d.containers, "", "  ")
	os.WriteFile(d.dataPath+"/containers.json", data, 0644)
}

func (d *LXCDashboard) loadState() {
	data, err := os.ReadFile(d.dataPath + "/containers.json")
	if err != nil {
		return
	}
	var containers map[string]*Container
	json.Unmarshal(data, &containers)
	if containers != nil {
		d.containers = containers
	}
}
