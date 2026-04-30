package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// LXCSandboxManager LXC容器沙箱管理器
// 对标 TrueNAS SCALE 的 LXC Sandboxes 功能
// 提供轻量级应用隔离运行环境
type LXCSandboxManager struct {
	mu       sync.RWMutex
	config   *LXCConfig
	sandboxes map[string]*Sandbox
	templates map[string]*Template
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// LXCConfig LXC配置
type LXCConfig struct {
	Enabled       bool   `json:"enabled"`
	StoragePath   string `json:"storage_path"`
	BridgeName    string `json:"bridge_name"`
	SubnetCIDR    string `json:"subnet_cidr"`
	MaxSandboxes  int    `json:"max_sandboxes"`
	DefaultCPU    int    `json:"default_cpu"`    // CPU核心数
	DefaultMemMB  int    `json:"default_mem_mb"` // 内存MB
	DefaultDiskGB int    `json:"default_disk_gb"`
}

// Template LXC模板
type Template struct {
	Name        string            `json:"name"`
	Distro      string            `json:"distro"`      // ubuntu, alpine, debian, etc.
	Version     string            `json:"version"`
	Description string            `json:"description"`
	ImageURL    string            `json:"image_url"`
	Packages    []string          `json:"packages"`
	Metadata    map[string]string `json:"metadata"`
	SizeMB      int               `json:"size_mb"`
}

// Sandbox LXC沙箱实例
type Sandbox struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Template    string            `json:"template"`
	Status      SandboxStatus     `json:"status"`
	IP          string            `json:"ip"`
	CPU         int               `json:"cpu"`
	MemoryMB    int               `json:"memory_mb"`
	DiskGB      int               `json:"disk_gb"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	EnvVars     map[string]string `json:"env_vars"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	Stats       *SandboxStats     `json:"stats,omitempty"`
	RestartPolicy RestartPolicy   `json:"restart_policy"`
}

// SandboxStatus 沙箱状态
type SandboxStatus string

const (
	StatusCreating  SandboxStatus = "creating"
	StatusStopped   SandboxStatus = "stopped"
	StatusRunning   SandboxStatus = "running"
	StatusError     SandboxStatus = "error"
	StatusDeleting  SandboxStatus = "deleting"
)

// RestartPolicy 重启策略
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNever     RestartPolicy = "never"
)

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp, udp
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// SandboxStats 沙箱统计
type SandboxStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsedMB  int     `json:"memory_used_mb"`
	MemoryLimitMB int     `json:"memory_limit_mb"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	NetRxBytes    int64   `json:"net_rx_bytes"`
	NetTxBytes    int64   `json:"net_tx_bytes"`
	PIDs          int     `json:"pids"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewLXCSandboxManager 创建LXC沙箱管理器
func NewLXCSandboxManager(cfg *LXCConfig) *LXCSandboxManager {
	if cfg == nil {
		cfg = &LXCConfig{
			Enabled:       true,
			StoragePath:   "/var/lib/nas-os/lxc",
			BridgeName:    "lxcbr0",
			SubnetCIDR:    "10.0.3.0/24",
			MaxSandboxes:  50,
			DefaultCPU:    2,
			DefaultMemMB:  512,
			DefaultDiskGB: 10,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &LXCSandboxManager{
		config:    cfg,
		sandboxes: make(map[string]*Sandbox),
		templates: defaultTemplates(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func defaultTemplates() map[string]*Template {
	return map[string]*Template{
		"ubuntu-24.04": {
			Name: "ubuntu-24.04", Distro: "ubuntu", Version: "24.04",
			Description: "Ubuntu 24.04 LTS - 通用服务器环境",
			Packages:    []string{"curl", "wget", "vim", "git"},
			SizeMB:      300,
		},
		"alpine-3.20": {
			Name: "alpine-3.20", Distro: "alpine", Version: "3.20",
			Description: "Alpine Linux 3.20 - 轻量级容器环境",
			Packages:    []string{"busybox", "curl"},
			SizeMB:      50,
		},
		"debian-12": {
			Name: "debian-12", Distro: "debian", Version: "12",
			Description: "Debian 12 - 稳定服务器环境",
			Packages:    []string{"curl", "wget", "vim"},
			SizeMB:      250,
		},
	}
}

// Start 启动管理器
func (m *LXCSandboxManager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	if err := os.MkdirAll(m.config.StoragePath, 0755); err != nil {
		return err
	}
	// 确保 LXC 已安装
	if _, err := exec.LookPath("lxc-start"); err != nil {
		return fmt.Errorf("LXC 未安装: %w", err)
	}
	return nil
}

// Stop 停止管理器
func (m *LXCSandboxManager) Stop() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// CreateSandbox 创建沙箱
func (m *LXCSandboxManager) CreateSandbox(name, templateName string, opts *SandboxOptions) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sandboxes) >= m.config.MaxSandboxes {
		return nil, fmt.Errorf("已达到最大沙箱数 %d", m.config.MaxSandboxes)
	}

	tmpl, exists := m.templates[templateName]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateName)
	}

	if opts == nil {
		opts = &SandboxOptions{}
	}
	if opts.CPU == 0 {
		opts.CPU = m.config.DefaultCPU
	}
	if opts.MemoryMB == 0 {
		opts.MemoryMB = m.config.DefaultMemMB
	}
	if opts.DiskGB == 0 {
		opts.DiskGB = m.config.DefaultDiskGB
	}

	sandbox := &Sandbox{
		ID:        fmt.Sprintf("lxc-%s-%d", name, time.Now().UnixNano()),
		Name:      name,
		Template:  templateName,
		Status:    StatusCreating,
		CPU:       opts.CPU,
		MemoryMB:  opts.MemoryMB,
		DiskGB:    opts.DiskGB,
		Ports:     opts.Ports,
		Volumes:   opts.Volumes,
		EnvVars:   opts.EnvVars,
		CreatedAt: time.Now(),
		RestartPolicy: opts.RestartPolicy,
	}

	if sandbox.RestartPolicy == "" {
		sandbox.RestartPolicy = RestartOnFailure
	}

	// 创建沙箱目录
	sandboxDir := filepath.Join(m.config.StoragePath, sandbox.ID)
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		return nil, err
	}

	// 生成 LXC 配置文件
	if err := m.generateLXCConfig(sandbox, tmpl); err != nil {
		os.RemoveAll(sandboxDir)
		return nil, err
	}

	sandbox.Status = StatusStopped
	m.sandboxes[sandbox.ID] = sandbox

	return sandbox, nil
}

// SandboxOptions 沙箱选项
type SandboxOptions struct {
	CPU           int
	MemoryMB      int
	DiskGB        int
	Ports         []PortMapping
	Volumes       []VolumeMount
	EnvVars       map[string]string
	RestartPolicy RestartPolicy
}

// StartSandbox 启动沙箱
func (m *LXCSandboxManager) StartSandbox(id string) error {
	m.mu.Lock()
	sandbox, exists := m.sandboxes[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("沙箱 %s 不存在", id)
	}

	if sandbox.Status == StatusRunning {
		return nil
	}

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lxc-start", "-n", sandbox.ID, "-d")
	if err := cmd.Run(); err != nil {
		sandbox.Status = StatusError
		return fmt.Errorf("启动沙箱失败: %w", err)
	}

	now := time.Now()
	sandbox.StartedAt = &now
	sandbox.Status = StatusRunning

	// 分配IP
	sandbox.IP = m.allocateIP()

	return nil
}

// StopSandbox 停止沙箱
func (m *LXCSandboxManager) StopSandbox(id string) error {
	m.mu.Lock()
	sandbox, exists := m.sandboxes[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("沙箱 %s 不存在", id)
	}

	if sandbox.Status != StatusRunning {
		return nil
	}

	ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lxc-stop", "-n", id)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("停止沙箱失败: %w", err)
	}

	sandbox.Status = StatusStopped
	sandbox.StartedAt = nil
	return nil
}

// DeleteSandbox 删除沙箱
func (m *LXCSandboxManager) DeleteSandbox(id string) error {
	m.mu.Lock()
	sandbox, exists := m.sandboxes[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("沙箱 %s 不存在", id)
	}
	m.mu.Unlock()

	if sandbox.Status == StatusRunning {
		if err := m.StopSandbox(id); err != nil {
			return err
		}
	}

	sandboxDir := filepath.Join(m.config.StoragePath, id)
	if err := os.RemoveAll(sandboxDir); err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.sandboxes, id)
	m.mu.Unlock()

	return nil
}

// ListSandboxes 列出所有沙箱
func (m *LXCSandboxManager) ListSandboxes() []*Sandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Sandbox, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		result = append(result, s)
	}
	return result
}

// GetSandboxStats 获取沙箱统计
func (m *LXCSandboxManager) GetSandboxStats(id string) (*SandboxStats, error) {
	m.mu.RLock()
	sandbox, exists := m.sandboxes[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("沙箱 %s 不存在", id)
	}
	if sandbox.Status != StatusRunning {
		return nil, fmt.Errorf("沙箱 %s 未运行", id)
	}

	stats := &SandboxStats{
		MemoryLimitMB: sandbox.MemoryMB,
		Timestamp:     time.Now(),
	}

	// 读取 cgroup 统计
	cgroupPath := filepath.Join("/sys/fs/cgroup/lxc", id)
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current")); err == nil {
		var memBytes int64
		fmt.Sscanf(string(data), "%d", &memBytes)
		stats.MemoryUsedMB = int(memBytes / 1024 / 1024)
	}

	return stats, nil
}

func (m *LXCSandboxManager) generateLXCConfig(sandbox *Sandbox, tmpl *Template) error {
	configPath := filepath.Join(m.config.StoragePath, sandbox.ID, "config")
	config := fmt.Sprintf(`lxc.uts.name = %s
lxc.arch = amd64
lxc.rootfs.path = dir:%s/rootfs
lxc.net.0.type = veth
lxc.net.0.link = %s
lxc.net.0.flags = up
lxc.cgroup2.cpu.max = %d00000 100000
lxc.cgroup2.memory.max = %d
`,
		sandbox.Name,
		filepath.Join(m.config.StoragePath, sandbox.ID),
		m.config.BridgeName,
		sandbox.CPU,
		sandbox.MemoryMB*1024*1024,
	)

	return os.WriteFile(configPath, []byte(config), 0644)
}

func (m *LXCSandboxManager) allocateIP() string {
	// 简单IP分配：基于现有沙箱数量
	return fmt.Sprintf("10.0.3.%d", 100+len(m.sandboxes))
}

// ExportSandbox 导出沙箱配置为JSON
func (m *LXCSandboxManager) ExportSandbox(id string) ([]byte, error) {
	m.mu.RLock()
	sandbox, exists := m.sandboxes[id]
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("沙箱 %s 不存在", id)
	}
	return json.MarshalIndent(sandbox, "", "  ")
}
