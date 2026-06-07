// Package vmm 虚拟机管理模块
// 学习群晖 VMM (Virtual Machine Manager)
package vmm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// VirtualMachine 虚拟机
type VirtualMachine struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Status        string            `json:"status"`  // running, stopped, paused, suspended, error
	OSType        string            `json:"os_type"` // linux, windows, macos
	OSVersion     string            `json:"os_version"`
	CPU           CPUConfig         `json:"cpu"`
	Memory        MemoryConfig      `json:"memory"`
	Disks         []DiskConfig      `json:"disks"`
	Networks      []NetConfig       `json:"networks"`
	Display       DisplayConfig     `json:"display"`
	BootOrder     []string          `json:"boot_order"`
	AutoStart     bool              `json:"auto_start"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     time.Time         `json:"started_at,omitempty"`
	StoppedAt     time.Time         `json:"stopped_at,omitempty"`
	Template      string            `json:"template,omitempty"`
	Labels        map[string]string `json:"labels"`
	OwnerID       string            `json:"owner_id"`
	Snapshots     []Snapshot        `json:"snapshots"`
	CurrentSnapID string            `json:"current_snapshot_id,omitempty"`
}

// CPUConfig CPU 配置
type CPUConfig struct {
	Cores   int     `json:"cores"`
	Threads int     `json:"threads"`
	Sockets int     `json:"sockets"`
	Model   string  `json:"model,omitempty"`
	Speed   float64 `json:"speed"` // GHz
}

// MemoryConfig 内存配置
type MemoryConfig struct {
	Size      int64 `json:"size"` // bytes
	MaxSize   int64 `json:"max_size"`
	Balloon   bool  `json:"balloon"` // 内存气球
	HugePages bool  `json:"huge_pages"`
}

// DiskConfig 磁盘配置
type DiskConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Format    string `json:"format"`     // qcow2, raw, vmdk
	Bus       string `json:"bus"`        // virtio, ide, scsi
	CacheMode string `json:"cache_mode"` // none, writeback, writethrough
	BootOrder int    `json:"boot_order"`
	IsCDROM   bool   `json:"is_cdrom"`
	ImagePath string `json:"image_path,omitempty"`
}

// NetConfig 网络配置
type NetConfig struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Model       string   `json:"model"` // virtio, e1000, rtl8139
	MACAddress  string   `json:"mac_address"`
	NetworkID   string   `json:"network_id"`
	IPAddresses []string `json:"ip_addresses,omitempty"`
	Connected   bool     `json:"connected"`
}

// DisplayConfig 显示配置
type DisplayConfig struct {
	Type     string `json:"type"` // vnc, spice, none
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Port     int    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
}

// Snapshot 快照
type Snapshot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Size        int64     `json:"size"`
	IsCurrent   bool      `json:"is_current"`
	ParentID    string    `json:"parent_id,omitempty"`
	Children    []string  `json:"children,omitempty"`
}

// Network 虚拟网络
type Network struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"` // bridge, nat, isolated
	Bridge    string     `json:"bridge"`
	Subnet    string     `json:"subnet"`
	Gateway   string     `json:"gateway"`
	DHCP      DHCPConfig `json:"dhcp"`
	Connected []string   `json:"connected_vms"`
}

// DHCPConfig DHCP 配置
type DHCPConfig struct {
	Enabled    bool   `json:"enabled"`
	RangeStart string `json:"range_start"`
	RangeEnd   string `json:"range_end"`
	LeaseTime  int    `json:"lease_time"` // seconds
}

// Template 虚拟机模板
type Template struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	OSType      string       `json:"os_type"`
	OSVersion   string       `json:"os_version"`
	CPU         CPUConfig    `json:"cpu"`
	Memory      MemoryConfig `json:"memory"`
	Disks       []DiskConfig `json:"disks"`
	Networks    []NetConfig  `json:"networks"`
	Icon        string       `json:"icon"`
	MinCPU      int          `json:"min_cpu"`
	MinMemory   int64        `json:"min_memory"`
	MinDisk     int64        `json:"min_disk"`
	Author      string       `json:"author"`
	Downloads   int          `json:"downloads"`
}

// Manager 虚拟机管理器
type Manager struct {
	mu        sync.RWMutex
	vms       map[string]*VirtualMachine
	networks  map[string]*Network
	templates map[string]*Template
}

// NewManager 创建虚拟机管理器
func NewManager() *Manager {
	m := &Manager{
		vms:       make(map[string]*VirtualMachine),
		networks:  make(map[string]*Network),
		templates: make(map[string]*Template),
	}
	m.loadDefaultTemplates()
	m.createDefaultNetworks()
	return m
}

// ListVMs 列出虚拟机
func (m *Manager) ListVMs(ctx context.Context, all bool) ([]VirtualMachine, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vms := make([]VirtualMachine, 0)
	for _, vm := range m.vms {
		if all || vm.Status == "running" {
			vms = append(vms, *vm)
		}
	}

	return vms, nil
}

// GetVM 获取虚拟机详情
func (m *Manager) GetVM(ctx context.Context, id string) (*VirtualMachine, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, exists := m.vms[id]
	if !exists {
		return nil, fmt.Errorf("VM not found: %s", id)
	}

	return vm, nil
}

// CreateVM 创建虚拟机
func (m *Manager) CreateVM(ctx context.Context, name, osType string, opts ...VMOption) (*VirtualMachine, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := &VirtualMachine{
		ID:        generateID(),
		Name:      name,
		OSType:    osType,
		Status:    "stopped",
		CreatedAt: time.Now(),
		CPU: CPUConfig{
			Cores:   2,
			Threads: 1,
			Sockets: 1,
		},
		Memory: MemoryConfig{
			Size:    2 * 1024 * 1024 * 1024, // 2GB
			MaxSize: 4 * 1024 * 1024 * 1024, // 4GB
		},
		Disks: []DiskConfig{
			{
				ID:        generateID(),
				Name:      "disk0",
				Size:      50 * 1024 * 1024 * 1024, // 50GB
				Format:    "qcow2",
				Bus:       "virtio",
				CacheMode: "writeback",
				BootOrder: 1,
			},
		},
		Networks: []NetConfig{
			{
				ID:        generateID(),
				Name:      "net0",
				Model:     "virtio",
				NetworkID: "default",
				Connected: true,
			},
		},
		Display: DisplayConfig{
			Type:   "vnc",
			Width:  1024,
			Height: 768,
		},
		BootOrder: []string{"disk", "cdrom", "network"},
		Labels:    make(map[string]string),
		Snapshots: []Snapshot{},
	}

	// 应用选项
	for _, opt := range opts {
		opt(vm)
	}

	m.vms[vm.ID] = vm
	log.Printf("VM created: %s (%s)", name, vm.ID)

	return vm, nil
}

// StartVM 启动虚拟机
func (m *Manager) StartVM(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM not found: %s", id)
	}

	if vm.Status == "running" {
		return fmt.Errorf("VM already running: %s", id)
	}

	vm.Status = "running"
	vm.StartedAt = time.Now()
	log.Printf("VM started: %s", vm.Name)

	return nil
}

// StopVM 停止虚拟机
func (m *Manager) StopVM(ctx context.Context, id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM not found: %s", id)
	}

	if vm.Status != "running" {
		return fmt.Errorf("VM not running: %s", id)
	}

	vm.Status = "stopped"
	vm.StoppedAt = time.Now()
	log.Printf("VM stopped: %s (force=%v)", vm.Name, force)

	return nil
}

// RestartVM 重启虚拟机
func (m *Manager) RestartVM(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM not found: %s", id)
	}

	vm.Status = "running"
	vm.StartedAt = time.Now()
	log.Printf("VM restarted: %s", vm.Name)

	return nil
}

// PauseVM 暂停虚拟机
func (m *Manager) PauseVM(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM not found: %s", id)
	}

	if vm.Status != "running" {
		return fmt.Errorf("VM not running: %s", id)
	}

	vm.Status = "paused"
	log.Printf("VM paused: %s", vm.Name)

	return nil
}

// ResumeVM 恢复虚拟机
func (m *Manager) ResumeVM(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM not found: %s", id)
	}

	if vm.Status != "paused" {
		return fmt.Errorf("VM not paused: %s", id)
	}

	vm.Status = "running"
	log.Printf("VM resumed: %s", vm.Name)

	return nil
}

// RemoveVM 删除虚拟机
func (m *Manager) RemoveVM(ctx context.Context, id string, deleteDisks bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM not found: %s", id)
	}

	if vm.Status == "running" {
		return fmt.Errorf("VM is running, stop it first")
	}

	delete(m.vms, id)
	log.Printf("VM removed: %s (deleteDisks=%v)", vm.Name, deleteDisks)

	return nil
}

// CreateSnapshot 创建快照
func (m *Manager) CreateSnapshot(ctx context.Context, vmID, name, description string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM not found: %s", vmID)
	}

	snapshot := &Snapshot{
		ID:          generateID(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		IsCurrent:   true,
	}

	// 将其他快照标记为非当前
	for i := range vm.Snapshots {
		vm.Snapshots[i].IsCurrent = false
	}

	vm.Snapshots = append(vm.Snapshots, *snapshot)
	vm.CurrentSnapID = snapshot.ID

	log.Printf("Snapshot created: %s for VM %s", name, vm.Name)
	return snapshot, nil
}

// RestoreSnapshot 恢复快照
func (m *Manager) RestoreSnapshot(ctx context.Context, vmID, snapID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM not found: %s", vmID)
	}

	found := false
	for _, snap := range vm.Snapshots {
		if snap.ID == snapID {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("snapshot not found: %s", snapID)
	}

	vm.CurrentSnapID = snapID
	log.Printf("Snapshot restored: %s for VM %s", snapID, vm.Name)

	return nil
}

// DeleteSnapshot 删除快照
func (m *Manager) DeleteSnapshot(ctx context.Context, vmID, snapID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM not found: %s", vmID)
	}

	for i, snap := range vm.Snapshots {
		if snap.ID == snapID {
			vm.Snapshots = append(vm.Snapshots[:i], vm.Snapshots[i+1:]...)
			break
		}
	}

	log.Printf("Snapshot deleted: %s for VM %s", snapID, vm.Name)
	return nil
}

// ListNetworks 列出网络
func (m *Manager) ListNetworks(ctx context.Context) ([]Network, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	networks := make([]Network, 0, len(m.networks))
	for _, n := range m.networks {
		networks = append(networks, *n)
	}

	return networks, nil
}

// CreateNetwork 创建网络
func (m *Manager) CreateNetwork(ctx context.Context, name, netType string) (*Network, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	network := &Network{
		ID:   generateID(),
		Name: name,
		Type: netType,
	}

	m.networks[network.ID] = network
	log.Printf("Network created: %s", name)

	return network, nil
}

// ListTemplates 列出模板
func (m *Manager) ListTemplates(ctx context.Context) ([]Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]Template, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, *t)
	}

	return templates, nil
}

// CreateFromTemplate 从模板创建虚拟机
func (m *Manager) CreateFromTemplate(ctx context.Context, templateID, name string) (*VirtualMachine, error) {
	m.mu.RLock()
	template, exists := m.templates[templateID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	vm := &VirtualMachine{
		ID:        generateID(),
		Name:      name,
		OSType:    template.OSType,
		OSVersion: template.OSVersion,
		Status:    "stopped",
		CreatedAt: time.Now(),
		CPU:       template.CPU,
		Memory:    template.Memory,
		Disks:     template.Disks,
		Networks:  template.Networks,
		Display: DisplayConfig{
			Type:   "vnc",
			Width:  1024,
			Height: 768,
		},
		Template: templateID,
		Labels:   make(map[string]string),
	}

	m.mu.Lock()
	m.vms[vm.ID] = vm
	m.mu.Unlock()

	log.Printf("VM created from template: %s (%s)", name, vm.ID)
	return vm, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running := 0
	stopped := 0
	paused := 0
	totalCPU := 0
	totalMemory := int64(0)

	for _, vm := range m.vms {
		switch vm.Status {
		case "running":
			running++
		case "stopped":
			stopped++
		case "paused":
			paused++
		}
		totalCPU += vm.CPU.Cores * vm.CPU.Sockets
		totalMemory += vm.Memory.Size
	}

	return map[string]interface{}{
		"total_vms":    len(m.vms),
		"running":      running,
		"stopped":      stopped,
		"paused":       paused,
		"total_cpu":    totalCPU,
		"total_memory": totalMemory,
		"networks":     len(m.networks),
		"templates":    len(m.templates),
	}, nil
}

// VMOption 虚拟机选项
type VMOption func(*VirtualMachine)

// WithCPU 设置 CPU
func WithCPU(cores, threads, sockets int) VMOption {
	return func(vm *VirtualMachine) {
		vm.CPU = CPUConfig{
			Cores:   cores,
			Threads: threads,
			Sockets: sockets,
		}
	}
}

// WithMemory 设置内存
func WithMemory(size, maxSize int64) VMOption {
	return func(vm *VirtualMachine) {
		vm.Memory = MemoryConfig{
			Size:    size,
			MaxSize: maxSize,
		}
	}
}

// WithDisks 设置磁盘
func WithDisks(disks []DiskConfig) VMOption {
	return func(vm *VirtualMachine) {
		vm.Disks = disks
	}
}

// WithNetworks 设置网络
func WithNetworks(networks []NetConfig) VMOption {
	return func(vm *VirtualMachine) {
		vm.Networks = networks
	}
}

// WithDisplay 设置显示
func WithDisplay(display DisplayConfig) VMOption {
	return func(vm *VirtualMachine) {
		vm.Display = display
	}
}

func (m *Manager) loadDefaultTemplates() {
	defaultTemplates := []*Template{
		{
			ID:          "ubuntu-24.04",
			Name:        "Ubuntu 24.04 LTS",
			Description: "Ubuntu Server 24.04 LTS",
			OSType:      "linux",
			OSVersion:   "24.04",
			CPU:         CPUConfig{Cores: 2, Threads: 1, Sockets: 1},
			Memory:      MemoryConfig{Size: 2 * 1024 * 1024 * 1024},
			MinCPU:      1,
			MinMemory:   1 * 1024 * 1024 * 1024,
			MinDisk:     25 * 1024 * 1024 * 1024,
			Author:      "Canonical",
		},
		{
			ID:          "debian-12",
			Name:        "Debian 12",
			Description: "Debian GNU/Linux 12 (Bookworm)",
			OSType:      "linux",
			OSVersion:   "12",
			CPU:         CPUConfig{Cores: 1, Threads: 1, Sockets: 1},
			Memory:      MemoryConfig{Size: 1 * 1024 * 1024 * 1024},
			MinCPU:      1,
			MinMemory:   512 * 1024 * 1024,
			MinDisk:     20 * 1024 * 1024 * 1024,
			Author:      "Debian",
		},
		{
			ID:          "windows-11",
			Name:        "Windows 11",
			Description: "Windows 11 Pro",
			OSType:      "windows",
			OSVersion:   "11",
			CPU:         CPUConfig{Cores: 2, Threads: 2, Sockets: 1},
			Memory:      MemoryConfig{Size: 4 * 1024 * 1024 * 1024},
			MinCPU:      2,
			MinMemory:   4 * 1024 * 1024 * 1024,
			MinDisk:     64 * 1024 * 1024 * 1024,
			Author:      "Microsoft",
		},
	}

	for _, t := range defaultTemplates {
		m.templates[t.ID] = t
	}
}

func (m *Manager) createDefaultNetworks() {
	defaultNet := &Network{
		ID:      "default",
		Name:    "Default Network",
		Type:    "nat",
		Bridge:  "virbr0",
		Subnet:  "192.168.122.0/24",
		Gateway: "192.168.122.1",
		DHCP: DHCPConfig{
			Enabled:    true,
			RangeStart: "192.168.122.100",
			RangeEnd:   "192.168.122.200",
			LeaseTime:  3600,
		},
	}

	m.networks[defaultNet.ID] = defaultNet
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Export 导出虚拟机信息
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := map[string]interface{}{
		"vms":       m.vms,
		"networks":  m.networks,
		"templates": m.templates,
	}

	return json.MarshalIndent(data, "", "  ")
}
