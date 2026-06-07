package vmmanager

import (
	"fmt"
	"sync"
	"time"
)

// VMState 虚拟机状态
type VMState string

const (
	VMStateRunning  VMState = "running"
	VMStateStopped  VMState = "stopped"
	VMStatePaused   VMState = "paused"
	VMStateCreating VMState = "creating"
	VMStateError    VMState = "error"
)

// VMOSType 操作系统类型
type VMOSType string

const (
	OSLinux   VMOSType = "linux"
	OSWindows VMOSType = "windows"
	OSFreeBSD VMOSType = "freebsd"
	OSMacOS   VMOSType = "macos"
)

// VM 虚拟机
type VM struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	OSType         VMOSType  `json:"osType"`
	State          VMState   `json:"state"`
	CPUCores       int       `json:"cpuCores"`
	MemMB          int       `json:"memMb"`
	DiskGB         int       `json:"diskGb"`
	DiskPath       string    `json:"diskPath"`
	ISOPath        string    `json:"isoPath"`
	MACAddress     string    `json:"macAddress"`
	VNCPort        int       `json:"vncPort"`
	BootOrder      []string  `json:"bootOrder"`
	UEFI           bool      `json:"uefi"`
	TPM            bool      `json:"tpm"`
	GPUPassthrough bool      `json:"gpuPassthrough"`
	USBDevices     []string  `json:"usbDevices"`
	CreatedAt      time.Time `json:"createdAt"`
	StartedAt      time.Time `json:"startedAt"`
}

// VMSnapshot 虚拟机快照
type VMSnapshot struct {
	ID        string    `json:"id"`
	VMID      string    `json:"vmId"`
	Name      string    `json:"name"`
	Current   bool      `json:"current"`
	CreatedAt time.Time `json:"createdAt"`
}

// Manager 虚拟机管理器
type Manager struct {
	mu        sync.RWMutex
	vms       map[string]*VM
	snapshots map[string][]*VMSnapshot
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		vms:       make(map[string]*VM),
		snapshots: make(map[string][]*VMSnapshot),
	}
}

// ListVMs 列出虚拟机
func (m *Manager) ListVMs() []*VM {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vms := make([]*VM, 0, len(m.vms))
	for _, vm := range m.vms {
		vms = append(vms, vm)
	}
	return vms
}

// GetVM 获取虚拟机详情
func (m *Manager) GetVM(id string) (*VM, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vm, ok := m.vms[id]
	if !ok {
		return nil, fmt.Errorf("vm %s not found", id)
	}
	return vm, nil
}

// CreateVM 创建虚拟机
func (m *Manager) CreateVM(name string, osType VMOSType, cores, memMB, diskGB int) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm := &VM{
		ID:        fmt.Sprintf("vm-%d", len(m.vms)+1),
		Name:      name,
		OSType:    osType,
		State:     VMStateStopped,
		CPUCores:  cores,
		MemMB:     memMB,
		DiskGB:    diskGB,
		BootOrder: []string{"disk", "cdrom"},
		CreatedAt: time.Now(),
	}
	m.vms[vm.ID] = vm
	return vm, nil
}

// DeleteVM 删除虚拟机
func (m *Manager) DeleteVM(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vms[id]; !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	delete(m.vms, id)
	delete(m.snapshots, id)
	return nil
}

// StartVM 启动虚拟机
func (m *Manager) StartVM(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	vm.State = VMStateRunning
	vm.StartedAt = time.Now()
	return nil
}

// StopVM 停止虚拟机
func (m *Manager) StopVM(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	vm.State = VMStateStopped
	return nil
}

// PauseVM 暂停虚拟机
func (m *Manager) PauseVM(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	vm.State = VMStatePaused
	return nil
}

// ResumeVM 恢复虚拟机
func (m *Manager) ResumeVM(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	vm.State = VMStateRunning
	return nil
}

// CreateVMSnapshot 创建虚拟机快照
func (m *Manager) CreateVMSnapshot(vmID, name string) (*VMSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vms[vmID]; !ok {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}
	snap := &VMSnapshot{
		ID:        fmt.Sprintf("snap-%s-%d", vmID, len(m.snapshots[vmID])+1),
		VMID:      vmID,
		Name:      name,
		Current:   true,
		CreatedAt: time.Now(),
	}
	for _, s := range m.snapshots[vmID] {
		s.Current = false
	}
	m.snapshots[vmID] = append(m.snapshots[vmID], snap)
	return snap, nil
}

// GetVMSnapshots 获取虚拟机快照
func (m *Manager) GetVMSnapshots(vmID string) ([]*VMSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.vms[vmID]; !ok {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}
	snaps := m.snapshots[vmID]
	if snaps == nil {
		return []*VMSnapshot{}, nil
	}
	return snaps, nil
}

// RollbackVMSnapshot 回滚虚拟机快照
func (m *Manager) RollbackVMSnapshot(vmID, snapID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	snaps, ok := m.snapshots[vmID]
	if !ok {
		return fmt.Errorf("vm %s has no snapshots", vmID)
	}
	for _, s := range snaps {
		if s.ID == snapID {
			for _, ss := range snaps {
				ss.Current = false
			}
			s.Current = true
			return nil
		}
	}
	return fmt.Errorf("snapshot %s not found", snapID)
}

// UpdateVM 更新虚拟机配置
func (m *Manager) UpdateVM(id string, cores, memMB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	if cores > 0 {
		vm.CPUCores = cores
	}
	if memMB > 0 {
		vm.MemMB = memMB
	}
	return nil
}

// AttachGPU 直通GPU
func (m *Manager) AttachGPU(vmID, gpuID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[vmID]
	if !ok {
		return fmt.Errorf("vm %s not found", vmID)
	}
	vm.GPUPassthrough = true
	return nil
}

// AttachUSB 直通USB设备
func (m *Manager) AttachUSB(vmID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[vmID]
	if !ok {
		return fmt.Errorf("vm %s not found", vmID)
	}
	vm.USBDevices = append(vm.USBDevices, deviceID)
	return nil
}
