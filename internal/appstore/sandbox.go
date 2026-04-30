// Package appstore 应用沙箱隔离
// 基于Docker资源限制实现应用沙箱
package appstore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ========== 沙箱管理器 ==========

// SandboxManager 应用沙箱管理器
type SandboxManager struct {
	mu       sync.RWMutex
	sandboxes map[string]*Sandbox
	config    *SandboxConfig
}

// SandboxConfig 沙箱全局配置
type SandboxConfig struct {
	DefaultCPUQuota    float64 `json:"defaultCpuQuota"`    // 默认CPU配额（核心数）
	DefaultMemoryMB    int64   `json:"defaultMemoryMB"`    // 默认内存限制（MB）
	DefaultDiskGB      int64   `json:"defaultDiskGB"`      // 默认磁盘限制（GB）
	MaxCPUQuota        float64 `json:"maxCpuQuota"`        // 最大CPU配额
	MaxMemoryMB        int64   `json:"maxMemoryMB"`        // 最大内存限制
	MaxDiskGB          int64   `json:"maxDiskGB"`          // 最大磁盘限制
	EnableNetIsolation bool    `json:"enableNetIsolation"` // 启用网络隔离
	EnablePIDLimit     bool    `json:"enablePidLimit"`     // 启用PID限制
	MaxPIDPerSandbox   int     `json:"maxPidPerSandbox"`   // 每个沙箱最大PID数
	SeccompProfile     string  `json:"seccompProfile"`     // seccomp配置文件
	AppArmorProfile    string  `json:"appArmorProfile"`    // AppArmor配置文件
	ReadOnlyRootFS     bool    `json:"readOnlyRootFS"`     // 只读根文件系统
}

// DefaultSandboxConfig 默认沙箱配置
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		DefaultCPUQuota:    1.0,
		DefaultMemoryMB:    512,
		DefaultDiskGB:      10,
		MaxCPUQuota:        8.0,
		MaxMemoryMB:        8192,
		MaxDiskGB:          500,
		EnableNetIsolation: false,
		EnablePIDLimit:     true,
		MaxPIDPerSandbox:   256,
		ReadOnlyRootFS:     false,
	}
}

// Sandbox 应用沙箱实例
type Sandbox struct {
	ID            string            `json:"id"`
	AppID         string            `json:"appId"`
	ContainerIDs  []string          `json:"containerIds"`
	State         SandboxState      `json:"state"`
	ResourceLimits *ResourceLimits  `json:"resourceLimits"`
	NetworkPolicy *NetworkPolicy    `json:"networkPolicy,omitempty"`
	SecurityCtx   *SecurityContext  `json:"securityContext,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	Violations    []ResourceViolation `json:"violations,omitempty"`
}

// SandboxState 沙箱状态
type SandboxState string

const (
	SandboxStateCreating  SandboxState = "creating"
	SandboxStateRunning   SandboxState = "running"
	SandboxStatePaused    SandboxState = "paused"
	SandboxStateStopping  SandboxState = "stopping"
	SandboxStateStopped   SandboxState = "stopped"
	SandboxStateError     SandboxState = "error"
)

// NetworkPolicy 网络策略
type NetworkPolicy struct {
	Isolation      bool     `json:"isolation"`      // 完全隔离
	AllowInternet  bool     `json:"allowInternet"`  // 允许访问外网
	AllowLAN       bool     `json:"allowLan"`       // 允许访问局域网
	AllowedPorts   []int    `json:"allowedPorts"`   // 允许的端口
	BlockedPorts   []int    `json:"blockedPorts"`   // 禁止的端口
	AllowedDomains []string `json:"allowedDomains"` // 允许访问的域名
	BandwidthMbps  int      `json:"bandwidthMbps"`  // 带宽限制（Mbps）
}

// SecurityContext 安全上下文
type SecurityContext struct {
	Privileged    bool     `json:"privileged"`    // 特权模式
	ReadOnlyRoot  bool     `json:"readOnlyRoot"`  // 只读根文件系统
	NoNewPrivs    bool     `json:"noNewPrivs"`    // 禁止提权
	DropCaps      []string `json:"dropCaps"`      // 移除的Linux capabilities
	AddCaps       []string `json:"addCaps"`       // 添加的Linux capabilities
	SeccompProfile string  `json:"seccompProfile"` // seccomp配置
	AppArmorProfile string `json:"appArmorProfile"` // AppArmor配置
	UserNS        bool     `json:"userNs"`        // 用户命名空间隔离
}

// ResourceViolation 资源违规记录
type ResourceViolation struct {
	Timestamp   time.Time `json:"timestamp"`
	Resource    string    `json:"resource"`    // "cpu", "memory", "disk", "network"
	Limit       string    `json:"limit"`
	Actual      string    `json:"actual"`
	Action      string    `json:"action"`      // "warn", "throttle", "kill"
	Description string    `json:"description"`
}

// NewSandboxManager 创建沙箱管理器
func NewSandboxManager(config *SandboxConfig) *SandboxManager {
	if config == nil {
		config = DefaultSandboxConfig()
	}
	return &SandboxManager{
		sandboxes: make(map[string]*Sandbox),
		config:    config,
	}
}

// CreateSandbox 创建应用沙箱
func (sm *SandboxManager) CreateSandbox(ctx context.Context, appID string, resourceLimits *ResourceLimits) (*Sandbox, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否已有沙箱
	for _, sb := range sm.sandboxes {
		if sb.AppID == appID && sb.State != SandboxStateStopped {
			return nil, fmt.Errorf("应用 %s 已有活跃沙箱: %s", appID, sb.ID)
		}
	}

	// 验证和限制资源
	limits := sm.validateResourceLimits(resourceLimits)

	sandbox := &Sandbox{
		ID:             fmt.Sprintf("sandbox-%s-%d", appID, time.Now().UnixMilli()),
		AppID:          appID,
		State:          SandboxStateCreating,
		ResourceLimits: limits,
		NetworkPolicy: &NetworkPolicy{
			AllowInternet: true,
			AllowLAN:      true,
		},
		SecurityCtx: &SecurityContext{
			ReadOnlyRoot: sm.config.ReadOnlyRootFS,
			NoNewPrivs:   true,
			DropCaps: []string{
				"SYS_ADMIN", "SYS_PTRACE", "SYS_MODULE",
				"DAC_READ_SEARCH", "NET_ADMIN",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sm.sandboxes[sandbox.ID] = sandbox

	// 模拟沙箱创建过程
	sandbox.State = SandboxStateRunning
	sandbox.UpdatedAt = time.Now()

	return sandbox, nil
}

// GetSandbox 获取沙箱信息
func (sm *SandboxManager) GetSandbox(sandboxID string) (*Sandbox, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sb, ok := sm.sandboxes[sandboxID]
	return sb, ok
}

// GetSandboxByApp 按应用ID获取沙箱
func (sm *SandboxManager) GetSandboxByApp(appID string) (*Sandbox, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, sb := range sm.sandboxes {
		if sb.AppID == appID && sb.State != SandboxStateStopped {
			return sb, true
		}
	}
	return nil, false
}

// ListSandboxes 列出所有沙箱
func (sm *SandboxManager) ListSandboxes() []*Sandbox {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Sandbox, 0, len(sm.sandboxes))
	for _, sb := range sm.sandboxes {
		result = append(result, sb)
	}
	return result
}

// UpdateResourceLimits 更新沙箱资源限制
func (sm *SandboxManager) UpdateResourceLimits(sandboxID string, limits *ResourceLimits) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sb, ok := sm.sandboxes[sandboxID]
	if !ok {
		return fmt.Errorf("沙箱 %s 不存在", sandboxID)
	}
	if sb.State != SandboxStateRunning {
		return fmt.Errorf("沙箱 %s 未在运行状态", sandboxID)
	}

	sb.ResourceLimits = sm.validateResourceLimits(limits)
	sb.UpdatedAt = time.Now()

	return nil
}

// PauseSandbox 暂停沙箱
func (sm *SandboxManager) PauseSandbox(sandboxID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sb, ok := sm.sandboxes[sandboxID]
	if !ok {
		return fmt.Errorf("沙箱 %s 不存在", sandboxID)
	}
	if sb.State != SandboxStateRunning {
		return fmt.Errorf("沙箱 %s 未在运行状态", sandboxID)
	}

	sb.State = SandboxStatePaused
	sb.UpdatedAt = time.Now()
	return nil
}

// ResumeSandbox 恢复沙箱
func (sm *SandboxManager) ResumeSandbox(sandboxID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sb, ok := sm.sandboxes[sandboxID]
	if !ok {
		return fmt.Errorf("沙箱 %s 不存在", sandboxID)
	}
	if sb.State != SandboxStatePaused {
		return fmt.Errorf("沙箱 %s 未在暂停状态", sandboxID)
	}

	sb.State = SandboxStateRunning
	sb.UpdatedAt = time.Now()
	return nil
}

// DestroySandbox 销毁沙箱
func (sm *SandboxManager) DestroySandbox(sandboxID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sb, ok := sm.sandboxes[sandboxID]
	if !ok {
		return fmt.Errorf("沙箱 %s 不存在", sandboxID)
	}

	sb.State = SandboxStateStopped
	sb.UpdatedAt = time.Now()
	delete(sm.sandboxes, sandboxID)

	return nil
}

// GetResourceUsage 获取沙箱资源使用情况
func (sm *SandboxManager) GetResourceUsage(sandboxID string) (*ResourceUsage, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sb, ok := sm.sandboxes[sandboxID]
	if !ok {
		return nil, fmt.Errorf("沙箱 %s 不存在", sandboxID)
	}

	// 模拟资源使用数据
	usage := &ResourceUsage{
		SandboxID: sandboxID,
		CPUUsage:  0.3,
		MemoryMB:  sb.ResourceLimits.MemoryMB / 4,
		DiskMB:    sb.ResourceLimits.DiskGB * 100,
		NetRxMB:   10,
		NetTxMB:   5,
		PIDCount:  15,
		Timestamp: time.Now(),
	}

	return usage, nil
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	SandboxID string    `json:"sandboxId"`
	CPUUsage  float64   `json:"cpuUsage"`   // CPU使用率 (0-1)
	MemoryMB  int64     `json:"memoryMB"`   // 内存使用 (MB)
	DiskMB    int64     `json:"diskMB"`     // 磁盘使用 (MB)
	NetRxMB   int64     `json:"netRxMB"`    // 网络接收 (MB)
	NetTxMB   int64     `json:"netTxMB"`    // 网络发送 (MB)
	PIDCount  int       `json:"pidCount"`   // 进程数
	Timestamp time.Time `json:"timestamp"`
}

// validateResourceLimits 验证和限制资源
func (sm *SandboxManager) validateResourceLimits(limits *ResourceLimits) *ResourceLimits {
	if limits == nil {
		limits = &ResourceLimits{}
	}

	// CPU
	if limits.CPUCores <= 0 {
		limits.CPUCores = sm.config.DefaultCPUQuota
	}
	if limits.CPUCores > sm.config.MaxCPUQuota {
		limits.CPUCores = sm.config.MaxCPUQuota
	}

	// Memory
	if limits.MemoryMB <= 0 {
		limits.MemoryMB = sm.config.DefaultMemoryMB
	}
	if limits.MemoryMB > sm.config.MaxMemoryMB {
		limits.MemoryMB = sm.config.MaxMemoryMB
	}

	// Disk
	if limits.DiskGB <= 0 {
		limits.DiskGB = sm.config.DefaultDiskGB
	}
	if limits.DiskGB > sm.config.MaxDiskGB {
		limits.DiskGB = sm.config.MaxDiskGB
	}

	return limits
}

// GenerateDockerResourceArgs 生成Docker资源限制参数
func GenerateDockerResourceArgs(limits *ResourceLimits) []string {
	var args []string

	if limits == nil {
		return args
	}

	if limits.CPUCores > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", limits.CPUCores))
	}

	if limits.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", limits.MemoryMB))
		args = append(args, "--memory-swap", fmt.Sprintf("%dm", limits.MemoryMB*2))
	}

	if limits.IOPSRead > 0 || limits.IOPSWrite > 0 {
		args = append(args, "--device-read-iops", fmt.Sprintf("/dev/sda:%d", limits.IOPSRead))
		args = append(args, "--device-write-iops", fmt.Sprintf("/dev/sda:%d", limits.IOPSWrite))
	}

	return args
}

// GenerateSecurityArgs 生成Docker安全参数
func GenerateSecurityArgs(ctx *SecurityContext) []string {
	var args []string

	if ctx == nil {
		return args
	}

	if ctx.Privileged {
		args = append(args, "--privileged")
	}

	if ctx.ReadOnlyRoot {
		args = append(args, "--read-only")
	}

	if ctx.NoNewPrivs {
		args = append(args, "--security-opt", "no-new-privileges:true")
	}

	for _, cap := range ctx.DropCaps {
		args = append(args, "--cap-drop", cap)
	}

	for _, cap := range ctx.AddCaps {
		args = append(args, "--cap-add", cap)
	}

	if ctx.SeccompProfile != "" {
		args = append(args, "--security-opt", fmt.Sprintf("seccomp=%s", ctx.SeccompProfile))
	}

	if ctx.AppArmorProfile != "" {
		args = append(args, "--security-opt", fmt.Sprintf("apparmor=%s", ctx.AppArmorProfile))
	}

	return args
}
