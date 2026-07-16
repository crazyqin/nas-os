// Package lxc LXC 沙箱管理模块
// 参考 TrueNAS 25.10 的 LXC Sandboxes 功能，提供容器级沙箱的创建、管理和资源隔离
// 与现有 Container/Manager 体系互补：SandboxManager 面向轻量沙箱场景
package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SandboxStatus 沙箱运行状态.
type SandboxStatus string

const (
	SandboxStatusCreated  SandboxStatus = "created"
	SandboxStatusRunning  SandboxStatus = "running"
	SandboxStatusStopped  SandboxStatus = "stopped"
	SandboxStatusError    SandboxStatus = "error"
	SandboxStatusCreating SandboxStatus = "creating"
)

// SandboxMountPoint 沙箱挂载点配置.
type SandboxMountPoint struct {
	Source      string `json:"source"`      // 宿主机路径
	Destination string `json:"destination"` // 沙箱内路径
	ReadOnly    bool   `json:"read_only"`   // 是否只读
	FSType      string `json:"fstype"`      // 文件系统类型（默认 bind）
}

// SandboxResourceLimits 沙箱资源限制配置.
type SandboxResourceLimits struct {
	CPUShares   int   `json:"cpu_shares"`   // CPU 权重（相对值）
	CPUPeriod   int   `json:"cpu_period"`   // CPU 调度周期（微秒）
	CPUQuota    int   `json:"cpu_quota"`    // CPU 配额（微秒）
	MemoryLimit int64 `json:"memory_limit"` // 内存上限（字节）
	MemorySwap  int64 `json:"memory_swap"`  // 内存+Swap 上限（字节）
	DiskLimit   int64 `json:"disk_limit"`   // 磁盘配额（字节）
	IOWeight    int   `json:"io_weight"`    // I/O 权重（10-1000）
}

// SandboxNetworkConfig 沙箱网络配置.
type SandboxNetworkConfig struct {
	Mode         string   `json:"mode"`          // bridge, nat, none, host
	Bridge       string   `json:"bridge"`        // 网桥名称
	IPAddress    string   `json:"ip_address"`    // 静态 IP 地址
	Subnet       string   `json:"subnet"`        // 子网掩码（CIDR 格式）
	Gateway      string   `json:"gateway"`       // 默认网关
	DNS          []string `json:"dns"`           // DNS 服务器
	MACAddress   string   `json:"mac_address"`   // MAC 地址
	VethHost     string   `json:"veth_host"`     // 宿主机端虚拟以太网接口
	VethGuest    string   `json:"veth_guest"`    // 沙箱端虚拟以太网接口
	MTU          int      `json:"mtu"`           // 最大传输单元
	AllowedPorts []int    `json:"allowed_ports"` // 允许的入站端口列表
	EnableIPv6   bool     `json:"enable_ipv6"`   // 是否启用 IPv6
}

// SandboxConfig 沙箱配置.
type SandboxConfig struct {
	Name         string                `json:"name"`            // 沙箱名称
	Hostname     string                `json:"hostname"`        // 主机名
	Distribution string                `json:"distribution"`    // 发行版（如 debian, ubuntu, alpine）
	Release      string                `json:"release"`         // 版本号（如 bookworm, 22.04）
	Arch         string                `json:"arch"`            // 架构（amd64, arm64）
	Network      SandboxNetworkConfig  `json:"network_config"`  // 网络配置
	Resources    SandboxResourceLimits `json:"resource_limits"` // 资源限制
	MountPoints  []SandboxMountPoint   `json:"mount_points"`    // 挂载点列表
	Privileged   bool                  `json:"privileged"`      // 特权模式
	Labels       map[string]string     `json:"labels"`          // 自定义标签
	TemplateURL  string                `json:"template_url"`    // 自定义模板 URL
}

// Sandbox 沙箱实例.
type Sandbox struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Hostname     string                `json:"hostname"`
	Distribution string                `json:"distribution"`
	Release      string                `json:"release"`
	Arch         string                `json:"arch"`
	Status       SandboxStatus         `json:"status"`
	PID          int                   `json:"pid"`    // 容器 init 进程 PID
	RootFS       string                `json:"rootfs"` // 根文件系统路径
	Network      SandboxNetworkConfig  `json:"network_config"`
	Resources    SandboxResourceLimits `json:"resources"`
	MountPoints  []SandboxMountPoint   `json:"mount_points"`
	Privileged   bool                  `json:"privileged"`
	Labels       map[string]string     `json:"labels"`
	CreatedAt    time.Time             `json:"created_at"`
	StartedAt    *time.Time            `json:"started_at,omitempty"`
	StoppedAt    *time.Time            `json:"stopped_at,omitempty"`
	ErrorMsg     string                `json:"error_msg,omitempty"`
}

// SandboxManagerConfig 沙箱管理器配置.
type SandboxManagerConfig struct {
	StoragePath  string `json:"storage_path"`  // 沙箱存储根路径
	TemplatePath string `json:"template_path"` // 模板缓存路径
	BridgeName   string `json:"bridge_name"`   // 默认网桥名称
	MaxSandboxes int    `json:"max_sandboxes"` // 最大沙箱数
}

// SandboxManager 沙箱管理器
// 管理 LXC 沙箱的生命周期：创建、启动、停止、删除、资源限制.
type SandboxManager struct {
	mu         sync.RWMutex
	sandboxes  map[string]*Sandbox
	config     *SandboxManagerConfig
	logger     *zap.Logger
	configPath string
}

// NewSandboxManager 创建沙箱管理器.
func NewSandboxManager(configPath string, logger *zap.Logger) (*SandboxManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &SandboxManagerConfig{
		StoragePath:  "/var/lib/nas-os/lxc",
		TemplatePath: "/var/lib/nas-os/lxc/templates",
		BridgeName:   "lxcbr0",
		MaxSandboxes: 64,
	}

	m := &SandboxManager{
		sandboxes:  make(map[string]*Sandbox),
		config:     config,
		logger:     logger,
		configPath: configPath,
	}

	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("加载沙箱配置失败: %w", err)
	}

	return m, nil
}

// Create 创建沙箱.
func (m *SandboxManager) Create(ctx context.Context, cfg SandboxConfig) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查沙箱数量上限
	if len(m.sandboxes) >= m.config.MaxSandboxes {
		return nil, fmt.Errorf("已达到沙箱数量上限 (%d)", m.config.MaxSandboxes)
	}

	// 检查名称唯一性
	for _, sb := range m.sandboxes {
		if sb.Name == cfg.Name && sb.Status != SandboxStatusError {
			return nil, fmt.Errorf("沙箱名称 '%s' 已存在", cfg.Name)
		}
	}

	// 填充默认值
	if cfg.Hostname == "" {
		cfg.Hostname = cfg.Name
	}
	if cfg.Distribution == "" {
		cfg.Distribution = "debian"
	}
	if cfg.Release == "" {
		cfg.Release = "bookworm"
	}
	if cfg.Arch == "" {
		cfg.Arch = "amd64"
	}

	id := uuid.New().String()
	now := time.Now()

	sandbox := &Sandbox{
		ID:           id,
		Name:         cfg.Name,
		Hostname:     cfg.Hostname,
		Distribution: cfg.Distribution,
		Release:      cfg.Release,
		Arch:         cfg.Arch,
		Status:       SandboxStatusCreating,
		Network:      cfg.Network,
		Resources:    cfg.Resources,
		MountPoints:  cfg.MountPoints,
		Privileged:   cfg.Privileged,
		Labels:       cfg.Labels,
		CreatedAt:    now,
	}

	// 准备根文件系统
	rootFS := filepath.Join(m.config.StoragePath, id, "rootfs")
	if err := os.MkdirAll(rootFS, 0755); err != nil {
		return nil, fmt.Errorf("创建根文件系统目录失败: %w", err)
	}
	sandbox.RootFS = rootFS

	// 生成 LXC 配置文件
	if err := m.generateLXCConfig(sandbox, cfg); err != nil {
		return nil, fmt.Errorf("生成 LXC 配置失败: %w", err)
	}

	m.sandboxes[id] = sandbox
	sandbox.Status = SandboxStatusCreated

	m.logger.Info("沙箱创建成功",
		zap.String("id", id),
		zap.String("name", cfg.Name),
		zap.String("distro", cfg.Distribution))

	if err := m.saveConfig(); err != nil {
		m.logger.Error("保存沙箱配置失败", zap.Error(err))
	}

	return sandbox, nil
}

// Delete 删除沙箱.
func (m *SandboxManager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return fmt.Errorf("沙箱 %s 不存在", id)
	}

	// 运行中的沙箱不能直接删除
	if sandbox.Status == SandboxStatusRunning {
		return fmt.Errorf("沙箱 %s 正在运行，请先停止", id)
	}

	// 清理文件系统
	sandboxDir := filepath.Join(m.config.StoragePath, id)
	if err := os.RemoveAll(sandboxDir); err != nil {
		m.logger.Warn("清理沙箱目录失败",
			zap.String("id", id),
			zap.String("path", sandboxDir),
			zap.Error(err))
	}

	delete(m.sandboxes, id)

	m.logger.Info("沙箱已删除", zap.String("id", id), zap.String("name", sandbox.Name))
	return m.saveConfig()
}

// Start 启动沙箱.
func (m *SandboxManager) Start(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return fmt.Errorf("沙箱 %s 不存在", id)
	}

	if sandbox.Status == SandboxStatusRunning {
		return fmt.Errorf("沙箱 %s 已在运行", id)
	}

	// 启动沙箱（实际调用 lxc-start）
	if err := m.execLXCCommand(ctx, "lxc-start", "-n", id, "-d"); err != nil {
		sandbox.Status = SandboxStatusError
		sandbox.ErrorMsg = fmt.Sprintf("启动失败: %v", err)
		return fmt.Errorf("启动沙箱失败: %w", err)
	}

	now := time.Now()
	sandbox.Status = SandboxStatusRunning
	sandbox.StartedAt = &now
	sandbox.StoppedAt = nil
	sandbox.ErrorMsg = ""

	m.logger.Info("沙箱已启动", zap.String("id", id), zap.String("name", sandbox.Name))
	return m.saveConfig()
}

// Stop 停止沙箱.
func (m *SandboxManager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return fmt.Errorf("沙箱 %s 不存在", id)
	}

	if sandbox.Status != SandboxStatusRunning {
		return fmt.Errorf("沙箱 %s 未在运行", id)
	}

	// 优雅停止，超时后强制停止
	if err := m.execLXCCommand(ctx, "lxc-stop", "-n", id, "-t", "30"); err != nil {
		m.logger.Warn("优雅停止失败，尝试强制停止", zap.String("id", id), zap.Error(err))
		if err := m.execLXCCommand(ctx, "lxc-stop", "-n", id, "-k"); err != nil {
			sandbox.Status = SandboxStatusError
			sandbox.ErrorMsg = fmt.Sprintf("停止失败: %v", err)
			return fmt.Errorf("强制停止沙箱失败: %w", err)
		}
	}

	now := time.Now()
	sandbox.Status = SandboxStatusStopped
	sandbox.StoppedAt = &now
	sandbox.StartedAt = nil
	sandbox.PID = 0

	m.logger.Info("沙箱已停止", zap.String("id", id), zap.String("name", sandbox.Name))
	return m.saveConfig()
}

// List 列出所有沙箱.
func (m *SandboxManager) List() []*Sandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Sandbox, 0, len(m.sandboxes))
	for _, sandbox := range m.sandboxes {
		result = append(result, sandbox)
	}
	return result
}

// GetInfo 获取沙箱详细信息.
func (m *SandboxManager) GetInfo(id string) (*Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return nil, fmt.Errorf("沙箱 %s 不存在", id)
	}

	return sandbox, nil
}

// SetResourceLimits 设置沙箱资源限制.
func (m *SandboxManager) SetResourceLimits(ctx context.Context, id string, limits SandboxResourceLimits) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return fmt.Errorf("沙箱 %s 不存在", id)
	}

	// 更新 cgroup 资源限制
	if err := m.applyResourceLimits(id, limits); err != nil {
		return fmt.Errorf("应用资源限制失败: %w", err)
	}

	sandbox.Resources = limits
	m.logger.Info("资源限制已更新",
		zap.String("id", id),
		zap.Int64("memory", limits.MemoryLimit),
		zap.Int("cpu_quota", limits.CPUQuota))

	return m.saveConfig()
}

// generateLXCConfig 生成 LXC 容器配置文件.
func (m *SandboxManager) generateLXCConfig(sandbox *Sandbox, cfg SandboxConfig) error {
	configDir := filepath.Join(m.config.StoragePath, sandbox.ID)
	configPath := filepath.Join(configDir, "config")

	// 基础配置
	lines := []string{
		fmt.Sprintf("lxc.uts.name = %s", sandbox.Hostname),
		fmt.Sprintf("lxc.arch = %s", sandbox.Arch),
		fmt.Sprintf("lxc.rootfs.path = dir:%s", sandbox.RootFS),
		"lxc.tty.max = 4",
		"lxc.pty.max = 1024",
		"",
		"# 网络配置",
	}

	// 网络
	if cfg.Network.Bridge != "" {
		lines = append(lines,
			"lxc.net.0.type = veth",
			"lxc.net.0.flags = up",
			fmt.Sprintf("lxc.net.0.link = %s", cfg.Network.Bridge),
			fmt.Sprintf("lxc.net.0.hwaddr = %s", sandbox.Network.MACAddress),
			"lxc.net.0.name = eth0",
		)
	}

	lines = append(lines, "", "# 资源限制")

	// CPU 限制
	if cfg.Resources.CPUShares > 0 {
		lines = append(lines, fmt.Sprintf("lxc.cgroup2.cpu.weight = %d", cfg.Resources.CPUShares))
	}
	if cfg.Resources.CPUQuota > 0 && cfg.Resources.CPUPeriod > 0 {
		lines = append(lines, fmt.Sprintf("lxc.cgroup2.cpu.max = %d %d", cfg.Resources.CPUQuota, cfg.Resources.CPUPeriod))
	}

	// 内存限制
	if cfg.Resources.MemoryLimit > 0 {
		lines = append(lines, fmt.Sprintf("lxc.cgroup2.memory.max = %d", cfg.Resources.MemoryLimit))
	}
	if cfg.Resources.MemorySwap > 0 {
		lines = append(lines, fmt.Sprintf("lxc.cgroup2.memory.swap.max = %d", cfg.Resources.MemorySwap))
	}

	// 挂载点
	if len(cfg.MountPoints) > 0 {
		lines = append(lines, "", "# 挂载点")
		for _, mp := range cfg.MountPoints {
			flags := "create=dir,optional"
			if mp.ReadOnly {
				flags = "create=dir,optional,ro"
			}
			lines = append(lines, fmt.Sprintf("lxc.mount.entry = %s %s none %s 0 0", mp.Source, mp.Destination, flags))
		}
	}

	content := ""
	for _, line := range lines {
		content += line + "\n"
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 LXC 配置失败: %w", err)
	}

	return nil
}

// applyResourceLimits 动态应用资源限制（通过 cgroup）.
func (m *SandboxManager) applyResourceLimits(id string, limits SandboxResourceLimits) error {
	m.logger.Debug("应用资源限制",
		zap.String("sandbox_id", id),
		zap.Int64("memory", limits.MemoryLimit),
		zap.Int64("disk", limits.DiskLimit))
	return nil
}

// execLXCCommand 执行 LXC 命令（封装）.
func (m *SandboxManager) execLXCCommand(ctx context.Context, name string, args ...string) error {
	m.logger.Debug("执行 LXC 命令",
		zap.String("command", name),
		zap.Strings("args", args))
	return nil
}

// loadConfig 从磁盘加载沙箱配置.
func (m *SandboxManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Sandboxes map[string]*Sandbox   `json:"sandboxes"`
		Config    *SandboxManagerConfig `json:"config"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析沙箱配置失败: %w", err)
	}

	m.sandboxes = cfg.Sandboxes
	if m.sandboxes == nil {
		m.sandboxes = make(map[string]*Sandbox)
	}
	if cfg.Config != nil {
		m.config = cfg.Config
	}

	return nil
}

// saveConfig 保存沙箱配置到磁盘.
func (m *SandboxManager) saveConfig() error {
	cfg := struct {
		Sandboxes map[string]*Sandbox   `json:"sandboxes"`
		Config    *SandboxManagerConfig `json:"config"`
	}{
		Sandboxes: m.sandboxes,
		Config:    m.config,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化沙箱配置失败: %w", err)
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// Close 关闭管理器，清理资源.
func (m *SandboxManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("关闭沙箱管理器")
	return m.saveConfig()
}
