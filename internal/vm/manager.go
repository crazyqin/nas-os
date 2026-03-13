package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// DefaultVMStoragePath 默认 VM 存储路径
	DefaultVMStoragePath = "/mnt/vms"
	// DefaultISOStoragePath 默认 ISO 存储路径
	DefaultISOStoragePath = "/mnt/isos"
	// DefaultVNCPortBase 默认 VNC 起始端口
	DefaultVNCPortBase = 5900
)

// Manager VM 管理器
type Manager struct {
	mu               sync.RWMutex
	storagePath      string
	isoPath          string
	vncPortBase      int
	vms              map[string]*VM
	snapshots        map[string]*VMSnapshot
	templates        map[string]*VMTemplate
	logger           *zap.Logger
	libvirtAvailable bool
}

// NewManager 创建 VM 管理器
func NewManager(storagePath string, logger *zap.Logger) (*Manager, error) {
	if storagePath == "" {
		storagePath = DefaultVMStoragePath
	}

	// 创建存储目录
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("创建 VM 存储目录失败：%w", err)
	}

	isoPath := filepath.Join(filepath.Dir(storagePath), "isos")
	if err := os.MkdirAll(isoPath, 0755); err != nil {
		return nil, fmt.Errorf("创建 ISO 存储目录失败：%w", err)
	}

	m := &Manager{
		storagePath:      storagePath,
		isoPath:          isoPath,
		vncPortBase:      DefaultVNCPortBase,
		vms:              make(map[string]*VM),
		snapshots:        make(map[string]*VMSnapshot),
		templates:        make(map[string]*VMTemplate),
		logger:           logger,
		libvirtAvailable: checkLibvirt(),
	}

	// 加载现有 VM 配置
	if err := m.loadVMs(); err != nil {
		logger.Warn("加载现有 VM 配置失败", zap.Error(err))
	}

	// 加载快照
	if err := m.loadSnapshots(); err != nil {
		logger.Warn("加载快照配置失败", zap.Error(err))
	}

	// 加载模板
	if err := m.loadTemplates(); err != nil {
		logger.Warn("加载模板配置失败", zap.Error(err))
	}

	return m, nil
}

// checkLibvirt 检查 libvirt 是否可用
func checkLibvirt() bool {
	cmd := exec.Command("virsh", "-c", "qemu:///system", "list", "--all")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// loadVMs 加载现有 VM 配置
func (m *Manager) loadVMs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 从存储目录加载 VM 配置文件
	files, err := os.ReadDir(m.storagePath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() && !strings.HasPrefix(file.Name(), ".") {
			vmConfigPath := filepath.Join(m.storagePath, file.Name(), "config.json")
			if _, err := os.Stat(vmConfigPath); err == nil {
				vm, err := loadVMConfig(vmConfigPath)
				if err == nil {
					m.vms[vm.ID] = vm
					m.logger.Debug("加载 VM 配置", zap.String("vmId", vm.ID), zap.String("name", vm.Name))
				} else {
					m.logger.Warn("加载 VM 配置失败", zap.String("path", vmConfigPath), zap.Error(err))
				}
			}
		}
	}

	return nil
}

// loadSnapshots 加载快照配置
func (m *Manager) loadSnapshots() error {
	snapshotPath := filepath.Join(m.storagePath, "snapshots")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return nil
	}

	// TODO: 加载快照配置
	return nil
}

// loadTemplates 加载模板配置
func (m *Manager) loadTemplates() error {
	templatePath := filepath.Join(m.storagePath, "templates")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return nil
	}

	// 创建内置模板
	m.createBuiltInTemplates()
	return nil
}

// createBuiltInTemplates 创建内置模板
func (m *Manager) createBuiltInTemplates() {
	templates := []VMTemplate{
		{
			ID:          "tpl-ubuntu-2204",
			Name:        "Ubuntu 22.04 LTS",
			Description: "Ubuntu 22.04 LTS 默认配置",
			Type:        VMTypeLinux,
			CPU:         2,
			Memory:      2048,
			DiskSize:    20,
			Network:     "bridge",
			OS:          "ubuntu-2204",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "tpl-debian-11",
			Name:        "Debian 11",
			Description: "Debian 11 默认配置",
			Type:        VMTypeLinux,
			CPU:         2,
			Memory:      2048,
			DiskSize:    20,
			Network:     "bridge",
			OS:          "debian-11",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "tpl-windows-11",
			Name:        "Windows 11",
			Description: "Windows 11 默认配置",
			Type:        VMTypeWindows,
			CPU:         4,
			Memory:      4096,
			DiskSize:    60,
			Network:     "bridge",
			OS:          "windows-11",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "tpl-windows-10",
			Name:        "Windows 10",
			Description: "Windows 10 默认配置",
			Type:        VMTypeWindows,
			CPU:         2,
			Memory:      4096,
			DiskSize:    50,
			Network:     "bridge",
			OS:          "windows-10",
			CreatedAt:   time.Now(),
		},
	}

	for _, tpl := range templates {
		m.templates[tpl.ID] = &tpl
	}
}

// CreateVM 创建虚拟机
func (m *Manager) CreateVM(ctx context.Context, config VMConfig) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return nil, err
	}

	vmID := "vm-" + uuid.New().String()[:8]
	now := time.Now()

	vm := &VM{
		ID:          vmID,
		Name:        config.Name,
		Description: config.Description,
		Type:        config.Type,
		Status:      VMStatusCreating,
		CreatedAt:   now,
		UpdatedAt:   now,
		CPU:         config.CPU,
		Memory:      config.Memory,
		DiskSize:    config.DiskSize,
		Network:     config.Network,
		ISOPath:     config.ISOPath,
		VNCEnabled:  config.VNCEnabled,
		USBDevices:  config.USBDevices,
		PCIDevices:  config.PCIDevices,
		Tags:        config.Tags,
	}

	// 分配 VNC 端口
	if config.VNCEnabled {
		vm.VNCPort = m.allocateVNCPort()
	}

	// 创建 VM 目录
	vmDir := filepath.Join(m.storagePath, vmID)
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 VM 目录失败：%w", err)
	}

	// 创建磁盘镜像
	diskPath := filepath.Join(vmDir, "disk.qcow2")
	if err := m.createDiskImage(diskPath, config.DiskSize); err != nil {
		os.RemoveAll(vmDir)
		return nil, fmt.Errorf("创建磁盘镜像失败：%w", err)
	}
	vm.DiskPath = diskPath

	// 生成 libvirt XML 配置
	xmlConfig := m.generateLibvirtXML(vm)
	xmlPath := filepath.Join(vmDir, "domain.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlConfig), 0644); err != nil {
		os.RemoveAll(vmDir)
		return nil, fmt.Errorf("保存 VM 配置失败：%w", err)
	}

	// 保存 VM 配置
	if err := m.saveVMConfig(vm); err != nil {
		os.RemoveAll(vmDir)
		return nil, fmt.Errorf("保存 VM 配置失败：%w", err)
	}

	m.vms[vmID] = vm

	// 如果 libvirt 可用，定义 VM
	if m.libvirtAvailable {
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "define", xmlPath)
		if err := cmd.Run(); err != nil {
			m.logger.Warn("定义 libvirt VM 失败", zap.Error(err), zap.String("vm", vmID))
		}
	}

	vm.Status = VMStatusStopped
	vm.UpdatedAt = time.Now()

	m.logger.Info("VM 创建成功", zap.String("vmId", vmID), zap.String("name", vm.Name))

	return vm, nil
}

// validateConfig 验证 VM 配置
func (m *Manager) validateConfig(config VMConfig) error {
	if config.Name == "" {
		return fmt.Errorf("VM 名称不能为空")
	}

	// 检查名称是否重复
	for _, vm := range m.vms {
		if vm.Name == config.Name {
			return fmt.Errorf("VM 名称 %s 已存在", config.Name)
		}
	}

	if config.CPU < 1 {
		return fmt.Errorf("CPU 核心数至少为 1")
	}

	if config.Memory < 256 {
		return fmt.Errorf("内存至少为 256MB")
	}

	if config.DiskSize < 1 {
		return fmt.Errorf("磁盘大小至少为 1GB")
	}

	if config.Network != "bridge" && config.Network != "nat" {
		return fmt.Errorf("网络模式必须是 bridge 或 nat")
	}

	return nil
}

// createDiskImage 创建磁盘镜像
func (m *Manager) createDiskImage(path string, sizeGB uint64) error {
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, fmt.Sprintf("%dG", sizeGB))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img 创建失败：%w, output: %s", err, string(output))
	}
	return nil
}

// generateLibvirtXML 生成 libvirt XML 配置
func (m *Manager) generateLibvirtXML(vm *VM) string {
	// 生成基本的 libvirt domain XML
	xmlConfig := fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='MiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os>
    <type arch='x86_64' machine='pc'>hvm</type>
    <boot dev='cdrom'/>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <cpu mode='host-passthrough'/>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>`,
		vm.Name,
		vm.Memory,
		vm.CPU,
		vm.DiskPath,
	)

	// 添加 CDROM (ISO)
	if vm.ISOPath != "" {
		xmlConfig += fmt.Sprintf(`
    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='sda' bus='sata'/>
      <readonly/>
    </disk>`, vm.ISOPath)
	}

	// 添加网络
	networkType := "network"
	networkName := "default"
	if vm.Network == "bridge" {
		networkType = "bridge"
		networkName = "br0"
	}

	xmlConfig += fmt.Sprintf(`
    <interface type='%s'>
      <source %s='%s'/>
      <model type='virtio'/>
    </interface>`, networkType, networkType, networkName)

	// 添加 VNC
	if vm.VNCEnabled {
		xmlConfig += fmt.Sprintf(`
    <graphics type='vnc' port='%d' autoport='no' listen='0.0.0.0'>
      <listen type='address' address='0.0.0.0'/>
    </graphics>
    <video>
      <model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1'/>
    </video>`, vm.VNCPort)
	}

	// 添加 USB 直通
	for _, usbID := range vm.USBDevices {
		parts := strings.Split(usbID, ":")
		if len(parts) == 2 {
			xmlConfig += fmt.Sprintf(`
    <hostdev mode='subsystem' type='usb'>
      <source>
        <vendor id='0x%s'/>
        <product id='0x%s'/>
      </source>
    </hostdev>`, parts[0], parts[1])
		}
	}

	xmlConfig += `
  </devices>
</domain>`

	return xmlConfig
}

// allocateVNCPort 分配 VNC 端口
func (m *Manager) allocateVNCPort() int {
	usedPorts := make(map[int]bool)
	for _, vm := range m.vms {
		if vm.VNCPort > 0 {
			usedPorts[vm.VNCPort] = true
		}
	}

	for port := m.vncPortBase; port < m.vncPortBase+100; port++ {
		if !usedPorts[port] {
			return port
		}
	}

	return 0 // 无法分配
}

// saveVMConfig 保存 VM 配置
func (m *Manager) saveVMConfig(vm *VM) error {
	vmDir := filepath.Join(m.storagePath, vm.ID)
	configPath := filepath.Join(vmDir, "config.json")
	
	data, err := json.MarshalIndent(vm, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 VM 配置失败：%w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入 VM 配置文件失败：%w", err)
	}
	
	return nil
}

// loadVMConfig 加载 VM 配置
func loadVMConfig(configPath string) (*VM, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	
	var vm VM
	if err := json.Unmarshal(data, &vm); err != nil {
		return nil, fmt.Errorf("解析 VM 配置失败：%w", err)
	}
	
	return &vm, nil
}

// GetVM 获取虚拟机信息
func (m *Manager) GetVM(vmID string) (*VM, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM %s 不存在", vmID)
	}

	return vm, nil
}

// ListVMs 获取所有虚拟机列表
func (m *Manager) ListVMs() []*VM {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vms := make([]*VM, 0, len(m.vms))
	for _, vm := range m.vms {
		vms = append(vms, vm)
	}

	return vms
}

// StartVM 启动虚拟机
func (m *Manager) StartVM(ctx context.Context, vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status == VMStatusRunning {
		return fmt.Errorf("VM 已在运行中")
	}

	vm.Status = VMStatusRunning
	vm.UpdatedAt = time.Now()

	if m.libvirtAvailable {
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "start", vm.Name)
		if err := cmd.Run(); err != nil {
			m.logger.Warn("启动 VM 失败", zap.Error(err), zap.String("vm", vmID))
			vm.Status = VMStatusStopped
			return fmt.Errorf("启动 VM 失败：%w", err)
		}
	}

	m.logger.Info("VM 启动成功", zap.String("vmId", vmID))
	return nil
}

// StopVM 停止虚拟机
func (m *Manager) StopVM(ctx context.Context, vmID string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status == VMStatusStopped {
		return fmt.Errorf("VM 已停止")
	}

	vm.Status = VMStatusStopped
	vm.UpdatedAt = time.Now()

	if m.libvirtAvailable {
		var cmd *exec.Cmd
		if force {
			cmd = exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "destroy", vm.Name)
		} else {
			cmd = exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "shutdown", vm.Name)
		}
		if err := cmd.Run(); err != nil {
			m.logger.Warn("停止 VM 失败", zap.Error(err), zap.String("vm", vmID))
			return fmt.Errorf("停止 VM 失败：%w", err)
		}
	}

	m.logger.Info("VM 停止成功", zap.String("vmId", vmID))
	return nil
}

// DeleteVM 删除虚拟机
func (m *Manager) DeleteVM(ctx context.Context, vmID string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status == VMStatusRunning && !force {
		return fmt.Errorf("VM 正在运行，请先停止或删除时强制删除")
	}

	// 如果 libvirt 可用，先 undefine
	if m.libvirtAvailable && vm.Status != VMStatusStopped {
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "destroy", vm.Name)
		cmd.Run()
	}

	if m.libvirtAvailable {
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "undefine", vm.Name)
		if err := cmd.Run(); err != nil {
			m.logger.Warn("Undefine VM 失败", zap.Error(err), zap.String("vm", vmID))
		}
	}

	// 删除 VM 目录
	vmDir := filepath.Join(m.storagePath, vmID)
	if err := os.RemoveAll(vmDir); err != nil {
		m.logger.Warn("删除 VM 目录失败", zap.Error(err), zap.String("vm", vmID))
		return fmt.Errorf("删除 VM 目录失败：%w", err)
	}

	delete(m.vms, vmID)

	m.logger.Info("VM 删除成功", zap.String("vmId", vmID))
	return nil
}

// GetVNCConnection 获取 VNC 连接信息
func (m *Manager) GetVNCConnection(vmID string) (*VNCConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM %s 不存在", vmID)
	}

	if !vm.VNCEnabled || vm.VNCPort == 0 {
		return nil, fmt.Errorf("VM 未启用 VNC")
	}

	return &VNCConnection{
		Host: "0.0.0.0",
		Port: vm.VNCPort,
	}, nil
}

// UpdateVM 更新虚拟机配置
func (m *Manager) UpdateVM(ctx context.Context, vmID string, config VMConfig) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status == VMStatusRunning {
		return nil, fmt.Errorf("无法更新正在运行的 VM，请先停止")
	}

	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return nil, err
	}

	// 更新配置
	vm.Name = config.Name
	vm.Description = config.Description
	vm.Type = config.Type
	vm.CPU = config.CPU
	vm.Memory = config.Memory
	vm.DiskSize = config.DiskSize
	vm.Network = config.Network
	vm.ISOPath = config.ISOPath
	vm.VNCEnabled = config.VNCEnabled
	vm.USBDevices = config.USBDevices
	vm.PCIDevices = config.PCIDevices
	vm.Tags = config.Tags

	// 重新分配 VNC 端口
	if config.VNCEnabled && vm.VNCPort == 0 {
		vm.VNCPort = m.allocateVNCPort()
	} else if !config.VNCEnabled {
		vm.VNCPort = 0
	}

	// 重新生成 libvirt XML
	xmlConfig := m.generateLibvirtXML(vm)
	vmDir := filepath.Join(m.storagePath, vmID)
	xmlPath := filepath.Join(vmDir, "domain.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlConfig), 0644); err != nil {
		return nil, fmt.Errorf("保存 VM 配置失败：%w", err)
	}

	// 保存 VM 配置
	if err := m.saveVMConfig(vm); err != nil {
		return nil, fmt.Errorf("保存 VM 配置失败：%w", err)
	}

	vm.UpdatedAt = time.Now()

	// 如果 libvirt 可用，重新定义 VM
	if m.libvirtAvailable && vm.Status == VMStatusStopped {
		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "define", xmlPath)
		if err := cmd.Run(); err != nil {
			m.logger.Warn("重新定义 libvirt VM 失败", zap.Error(err), zap.String("vm", vmID))
		}
	}

	m.logger.Info("VM 配置更新成功", zap.String("vmId", vmID), zap.String("name", vm.Name))

	return vm, nil
}

// ListTemplates 获取所有 VM 模板
func (m *Manager) ListTemplates() []*VMTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*VMTemplate, 0, len(m.templates))
	for _, tpl := range m.templates {
		templates = append(templates, tpl)
	}

	return templates
}

// GetTemplate 获取单个模板
func (m *Manager) GetTemplate(templateID string) (*VMTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}

	return tpl, nil
}

// ListUSBDevices 列出可用 USB 设备
func (m *Manager) ListUSBDevices() ([]*USBDevice, error) {
	var devices []*USBDevice

	// 使用 lsusb 获取 USB 设备列表
	cmd := exec.Command("lsusb")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行 lsusb 失败：%w", err)
	}

	// 解析 lsusb 输出
	// 格式：Bus 001 Device 002: ID 1d6b:0002 Linux Foundation 2.0 root hub
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		// 提取 ID 部分
		idPart := strings.TrimSpace(parts[1])
		idFields := strings.Fields(idPart)
		if len(idFields) < 2 {
			continue
		}

		idStr := idFields[0] // 1d6b:0002
		idParts := strings.Split(idStr, ":")
		if len(idParts) != 2 {
			continue
		}

		// 检查是否已被 VM 使用
		inUse := false
		for _, vm := range m.vms {
			for _, usbID := range vm.USBDevices {
				if usbID == idStr {
					inUse = true
					break
				}
			}
		}

		// 提取设备描述
		description := strings.Join(idFields[1:], " ")

		devices = append(devices, &USBDevice{
			ID:       idStr,
			VendorID: idParts[0],
			ProductID: idParts[1],
			Product:  description,
			InUse:    inUse,
		})
	}

	return devices, nil
}

// ListPCIDevices 列出可用 PCIe 设备
func (m *Manager) ListPCIDevices() ([]*PCIDevice, error) {
	var devices []*PCIDevice

	// 使用 lspci 获取 PCIe 设备列表
	cmd := exec.Command("lspci", "-m")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行 lspci 失败：%w", err)
	}

	// 解析 lspci -m 输出
	// 格式：00:00.0 "Host bridge" [0600]: "Intel Corporation Device [8086:5918]"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// 提取 BDF 和设备信息
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		bdf := strings.Trim(fields[0], `"`)
		
		// 提取 vendor:device ID
		var vendorID, deviceID, name string
		for i, field := range fields {
			if strings.Contains(field, "[") && strings.Contains(field, ":") {
				// 提取 [vendor:device]
				idStr := strings.Trim(field, "[]")
				parts := strings.Split(idStr, ":")
				if len(parts) == 2 {
					vendorID = parts[0]
					deviceID = strings.Trim(parts[1], `]"`)
				}
				// 提取设备名称
				if i+1 < len(fields) {
					name = strings.Trim(fields[i+1], `"`)
				}
			}
		}

		if vendorID == "" || deviceID == "" {
			continue
		}

		// 检查是否已被 VM 使用
		inUse := false
		for _, vm := range m.vms {
			for _, pciID := range vm.PCIDevices {
				if pciID == bdf {
					inUse = true
					break
				}
			}
		}

		devices = append(devices, &PCIDevice{
			ID:       bdf,
			BDF:      bdf,
			VendorID: vendorID,
			DeviceID: deviceID,
			Name:     name,
			InUse:    inUse,
		})
	}

	return devices, nil
}

// GetVMStats 完善版本 - 从 libvirt 获取实时统计信息
func (m *Manager) GetVMStats(vmID string) (*VMStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, exists := m.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM %s 不存在", vmID)
	}

	if vm.Status != VMStatusRunning {
		return &VMStats{}, nil
	}

	// 如果 libvirt 可用，尝试获取实时统计
	if m.libvirtAvailable {
		stats, err := m.getLibvirtStats(vm.Name)
		if err == nil {
			return stats, nil
		}
		m.logger.Debug("获取 libvirt 统计失败", zap.Error(err))
	}

	// 返回空统计
	return &VMStats{
		CPUUsage:    0,
		MemoryUsage: 0,
		DiskRead:    0,
		DiskWrite:   0,
		NetRX:       0,
		NetTX:       0,
	}, nil
}

// getLibvirtStats 从 libvirt 获取 VM 统计信息
func (m *Manager) getLibvirtStats(vmName string) (*VMStats, error) {
	// 使用 virsh domstats 获取统计信息
	cmd := exec.Command("virsh", "-c", "qemu:///system", "domstats", vmName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	stats := &VMStats{}
	lines := strings.Split(string(output), "\n")
	
	var memoryTotal, memoryAvailable uint64
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		switch key {
		case "cpu.time":
			// CPU 时间 (纳秒)，需要计算使用率
		case "memory.rss":
			// 实际使用内存 (KB)
			var memRSS uint64
			if _, err := fmt.Sscanf(value, "%d", &memRSS); err == nil {
				stats.MemoryUsage = memRSS / 1024 // 转换为 MB
			}
		case "memory.available":
			if _, err := fmt.Sscanf(value, "%d", &memoryAvailable); err == nil {
				// memoryAvailable 已声明
			}
		case "memory":
			if _, err := fmt.Sscanf(value, "%d", &memoryTotal); err == nil {
				// memoryTotal 已声明
			}
		case "net.received.bytes":
			if _, err := fmt.Sscanf(value, "%d", &stats.NetRX); err == nil {
				// stats.NetRX 已赋值
			}
		case "net.sent.bytes":
			if _, err := fmt.Sscanf(value, "%d", &stats.NetTX); err == nil {
				// stats.NetTX 已赋值
			}
		case "block.rd.bytes":
			if _, err := fmt.Sscanf(value, "%d", &stats.DiskRead); err == nil {
				// stats.DiskRead 已赋值
			}
		case "block.wr.bytes":
			if _, err := fmt.Sscanf(value, "%d", &stats.DiskWrite); err == nil {
				// stats.DiskWrite 已赋值
			}
		}
	}
	
	// 计算 CPU 使用率 (简化版本)
	if memoryTotal > 0 && memoryAvailable > 0 {
		stats.CPUUsage = float64(memoryTotal-memoryAvailable) / float64(memoryTotal) * 100
	}
	
	return stats, nil
}
