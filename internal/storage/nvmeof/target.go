// Package nvmeof - NVMe-oF Target 系统实现
// 封装 Linux 内核 nvmet 模块，通过 configfs 配置 NVMe-oF Target
package nvmeof

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	pkgnvmeof "nas-os/pkg/storage/nvmeof"
)

// ========== 常量定义 ==========

const (
	// NVMetConfigPath nvmet configfs 路径
	NVMetConfigPath = "/sys/kernel/config/nvmet"
	// NVMetSubsystemsPath 子系统路径
	NVMetSubsystemsPath = "/sys/kernel/config/nvmet/subsystems"
	// NVMetPortsPath 端口路径
	NVMetPortsPath = "/sys/kernel/config/nvmet/ports"
	// NVMetHostsPath 主机路径（子系统下）
	NVMetHostsPath = "allowed_hosts"

	// 默认端口
	DefaultTargetPort = 4420
	// 默认块大小
	DefaultBlockSize = 512
)

// ========== TargetSysManager ==========

// TargetSysManager NVMe-oF Target 系统管理器
// 实际封装 Linux 内核 nvmet 模块
type TargetSysManager struct {
	mu sync.RWMutex

	// pkg 层管理器
	pkgManager *pkgnvmeof.TargetManager

	// 配置
	config *pkgnvmeof.NVMeOFConfig

	// 已分配的端口 ID
	portIDs    map[int]bool
	nextPortID int

	// 运行状态
	running bool
}

// NewTargetSysManager 创建 Target 系统管理器
func NewTargetSysManager(config *pkgnvmeof.NVMeOFConfig) (*TargetSysManager, error) {
	if config == nil {
		config = pkgnvmeof.DefaultNVMeOFConfig()
	}

	pkgManager, err := pkgnvmeof.NewTargetManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg target manager: %w", err)
	}

	m := &TargetSysManager{
		pkgManager: pkgManager,
		config:     config,
		portIDs:    make(map[int]bool),
		nextPortID: 1,
	}

	// 检查 nvmet 模块是否可用
	if err := m.checkNVMetAvailable(); err != nil {
		return nil, fmt.Errorf("nvmet not available: %w", err)
	}

	// 加载现有配置
	m.loadExistingConfig()

	return m, nil
}

// checkNVMetAvailable 检查 nvmet 模块是否可用
func (m *TargetSysManager) checkNVMetAvailable() error {
	// 检查 configfs 是否挂载
	if _, err := os.Stat("/sys/kernel/config"); err != nil {
		return fmt.Errorf("configfs not mounted: %w", err)
	}

	// 检查 nvmet 目录是否存在
	if _, err := os.Stat(NVMetConfigPath); err != nil {
		// 尝试加载 nvmet 模块
		if err := m.loadModule("nvmet"); err != nil {
			return fmt.Errorf("nvmet module not loaded and cannot be loaded: %w", err)
		}
		// 再次检查
		if _, err := os.Stat(NVMetConfigPath); err != nil {
			return fmt.Errorf("nvmet config path not available after loading module: %w", err)
		}
	}

	return nil
}

// loadModule 加载内核模块
func (m *TargetSysManager) loadModule(module string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "modprobe", module)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load module %s: %w, output: %s", module, err, output)
	}
	return nil
}

// loadExistingConfig 加载现有配置
func (m *TargetSysManager) loadExistingConfig() {
	// 读取已存在的子系统
	subsysDir, err := os.ReadDir(NVMetSubsystemsPath)
	if err == nil {
		for _, entry := range subsysDir {
			if entry.IsDir() {
				nqn := entry.Name()
				// 尓创建 pkg 层子系统对象
				_, _ = m.pkgManager.CreateSubsystem(context.Background(), &pkgnvmeof.CreateSubsystemRequest{
					NQN:          nqn,
					Name:         extractNameFromNQN(nqn),
					AllowAnyHost: m.readAllowAnyHost(nqn),
				})
			}
		}
	}

	// 读取已存在的端口
	portsDir, err := os.ReadDir(NVMetPortsPath)
	if err == nil {
		for _, entry := range portsDir {
			if entry.IsDir() {
				portID, _ := strconv.Atoi(entry.Name())
				m.portIDs[portID] = true
				if portID >= m.nextPortID {
					m.nextPortID = portID + 1
				}
			}
		}
	}
}

// readAllowAnyHost 读取子系统是否允许任意主机
func (m *TargetSysManager) readAllowAnyHost(nqn string) bool {
	path := filepath.Join(NVMetSubsystemsPath, nqn, "attr_allow_any_host")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// extractNameFromNQN 从 NQN 提取名称
func extractNameFromNQN(nqn string) string {
	parts := strings.Split(nqn, ":")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return nqn
}

// ========== 子系统管理 ==========

// CreateSubsystem 创建子系统
func (m *TargetSysManager) CreateSubsystem(ctx context.Context, req *pkgnvmeof.CreateSubsystemRequest) (*pkgnvmeof.Subsystem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在 pkg 层创建
	subsystem, err := m.pkgManager.CreateSubsystem(ctx, req)
	if err != nil {
		return nil, err
	}

	// 在内核创建
	if err := m.createSubsystemInKernel(subsystem); err != nil {
		// 回滚 pkg 层
		_ = m.pkgManager.DeleteSubsystem(ctx, req.NQN, true)
		return nil, fmt.Errorf("failed to create subsystem in kernel: %w", err)
	}

	return subsystem, nil
}

// createSubsystemInKernel 在内核创建子系统
func (m *TargetSysManager) createSubsystemInKernel(subsystem *pkgnvmeof.Subsystem) error {
	subsysPath := filepath.Join(NVMetSubsystemsPath, subsystem.NQN)

	// 创建子系统目录
	if err := os.MkdirAll(subsysPath, 0o755); err != nil {
		return fmt.Errorf("failed to create subsystem directory: %w", err)
	}

	// 设置 attr_allow_any_host
	allowAnyPath := filepath.Join(subsysPath, "attr_allow_any_host")
	allowValue := "0"
	if subsystem.AllowAnyHost {
		allowValue = "1"
	}
	if err := os.WriteFile(allowAnyPath, []byte(allowValue), 0o644); err != nil {
		return fmt.Errorf("failed to set allow_any_host: %w", err)
	}

	return nil
}

// DeleteSubsystem 删除子系统
func (m *TargetSysManager) DeleteSubsystem(ctx context.Context, nqn string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先在内核删除
	if err := m.deleteSubsystemFromKernel(nqn, force); err != nil {
		return fmt.Errorf("failed to delete subsystem from kernel: %w", err)
	}

	// 再在 pkg 层删除
	return m.pkgManager.DeleteSubsystem(ctx, nqn, force)
}

// deleteSubsystemFromKernel 从内核删除子系统
func (m *TargetSysManager) deleteSubsystemFromKernel(nqn string, force bool) error {
	subsysPath := filepath.Join(NVMetSubsystemsPath, nqn)

	// 检查是否存在
	if _, err := os.Stat(subsysPath); os.IsNotExist(err) {
		return nil
	}

	// 先删除所有命名空间
	nsDir, err := os.ReadDir(filepath.Join(subsysPath, "namespaces"))
	if err == nil {
		for _, entry := range nsDir {
			if entry.IsDir() {
				nsid := entry.Name()
				// 禁用命名空间
				enablePath := filepath.Join(subsysPath, "namespaces", nsid, "enable")
				_ = os.WriteFile(enablePath, []byte("0"), 0o644)
				// 删除目录
				_ = os.RemoveAll(filepath.Join(subsysPath, "namespaces", nsid))
			}
		}
	}

	// 删除允许的主机
	hostsDir, err := os.ReadDir(filepath.Join(subsysPath, NVMetHostsPath))
	if err == nil {
		for _, entry := range hostsDir {
			_ = os.Remove(filepath.Join(subsysPath, NVMetHostsPath, entry.Name()))
		}
	}

	// 删除子系统目录
	return os.RemoveAll(subsysPath)
}

// ========== 命名空间管理 ==========

// CreateNamespace 创建命名空间
func (m *TargetSysManager) CreateNamespace(ctx context.Context, subsystemNQN string, req *pkgnvmeof.CreateNamespaceRequest) (*pkgnvmeof.Namespace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在 pkg 层创建
	namespace, err := m.pkgManager.CreateNamespace(ctx, subsystemNQN, req)
	if err != nil {
		return nil, err
	}

	// 在内核创建
	if err := m.createNamespaceInKernel(subsystemNQN, namespace); err != nil {
		// 回滚 pkg 层
		_ = m.pkgManager.DeleteNamespace(ctx, subsystemNQN, namespace.NSID)
		return nil, fmt.Errorf("failed to create namespace in kernel: %w", err)
	}

	return namespace, nil
}

// createNamespaceInKernel 在内核创建命名空间
func (m *TargetSysManager) createNamespaceInKernel(subsystemNQN string, namespace *pkgnvmeof.Namespace) error {
	subsysPath := filepath.Join(NVMetSubsystemsPath, subsystemNQN)
	nsidStr := strconv.FormatUint(uint64(namespace.NSID), 10)
	nsPath := filepath.Join(subsysPath, "namespaces", nsidStr)

	// 创建命名空间目录
	if err := os.MkdirAll(nsPath, 0o755); err != nil {
		return fmt.Errorf("failed to create namespace directory: %w", err)
	}

	// 设置后端设备路径
	devicePath := filepath.Join(nsPath, "device_path")
	if err := os.WriteFile(devicePath, []byte(namespace.DevicePath), 0o644); err != nil {
		return fmt.Errorf("failed to set device_path: %w", err)
	}

	// 设置块大小（可选）
	if namespace.BlockSize > 0 {
		blockSizePath := filepath.Join(nsPath, "block_size")
		blockSizeStr := strconv.FormatUint(uint64(namespace.BlockSize), 10)
		if err := os.WriteFile(blockSizePath, []byte(blockSizeStr), 0o644); err != nil {
			// 块大小设置失败不影响创建
		}
	}

	// 启用命名空间
	if namespace.Enabled {
		enablePath := filepath.Join(nsPath, "enable")
		if err := os.WriteFile(enablePath, []byte("1"), 0o644); err != nil {
			return fmt.Errorf("failed to enable namespace: %w", err)
		}
	}

	return nil
}

// DeleteNamespace 删除命名空间
func (m *TargetSysManager) DeleteNamespace(ctx context.Context, subsystemNQN string, nsid uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先在内核删除
	if err := m.deleteNamespaceFromKernel(subsystemNQN, nsid); err != nil {
		return fmt.Errorf("failed to delete namespace from kernel: %w", err)
	}

	// 再在 pkg 层删除
	return m.pkgManager.DeleteNamespace(ctx, subsystemNQN, nsid)
}

// deleteNamespaceFromKernel 从内核删除命名空间
func (m *TargetSysManager) deleteNamespaceFromKernel(subsystemNQN string, nsid uint32) error {
	subsysPath := filepath.Join(NVMetSubsystemsPath, subsystemNQN)
	nsidStr := strconv.FormatUint(uint64(nsid), 10)
	nsPath := filepath.Join(subsysPath, "namespaces", nsidStr)

	// 先禁用
	enablePath := filepath.Join(nsPath, "enable")
	_ = os.WriteFile(enablePath, []byte("0"), 0o644)

	// 删除目录
	return os.RemoveAll(nsPath)
}

// ========== 监听器管理 ==========

// CreateListener 创建监听器（端口）
func (m *TargetSysManager) CreateListener(ctx context.Context, subsystemNQN string, req *pkgnvmeof.CreateListenerRequest) (*pkgnvmeof.Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在 pkg 层创建
	listener, err := m.pkgManager.CreateListener(ctx, subsystemNQN, req)
	if err != nil {
		return nil, err
	}

	// 在内核创建端口并链接到子系统
	if err := m.createListenerInKernel(subsystemNQN, listener); err != nil {
		// 回滚 pkg 层
		_ = m.pkgManager.DeleteListener(ctx, subsystemNQN, listener.ID)
		return nil, fmt.Errorf("failed to create listener in kernel: %w", err)
	}

	return listener, nil
}

// createListenerInKernel 在内核创建监听器
func (m *TargetSysManager) createListenerInKernel(subsystemNQN string, listener *pkgnvmeof.Listener) error {
	// 分配端口 ID
	portID := m.allocatePortID()
	portPath := filepath.Join(NVMetPortsPath, strconv.Itoa(portID))

	// 创建端口目录
	if err := os.MkdirAll(portPath, 0o755); err != nil {
		return fmt.Errorf("failed to create port directory: %w", err)
	}

	// 设置传输类型
	trTypePath := filepath.Join(portPath, "addr_trtype")
	trType := string(listener.Transport)
	if err := os.WriteFile(trTypePath, []byte(trType), 0o644); err != nil {
		return fmt.Errorf("failed to set addr_trtype: %w", err)
	}

	// 设置地址
	addrPath := filepath.Join(portPath, "addr_traddr")
	if err := os.WriteFile(addrPath, []byte(listener.TrAddress), 0o644); err != nil {
		return fmt.Errorf("failed to set addr_traddr: %w", err)
	}

	// 设置端口
	svcidPath := filepath.Join(portPath, "addr_trsvcid")
	if err := os.WriteFile(svcidPath, []byte(listener.TrSVCID), 0o644); err != nil {
		return fmt.Errorf("failed to set addr_trsvcid: %w", err)
	}

	// 链接到子系统
	linkPath := filepath.Join(portPath, "subsystems", subsystemNQN)
	if err := os.MkdirAll(linkPath, 0o755); err != nil {
		return fmt.Errorf("failed to link subsystem to port: %w", err)
	}

	return nil
}

// allocatePortID 分配端口 ID
func (m *TargetSysManager) allocatePortID() int {
	for {
		if !m.portIDs[m.nextPortID] {
			m.portIDs[m.nextPortID] = true
			id := m.nextPortID
			m.nextPortID++
			return id
		}
		m.nextPortID++
	}
}

// DeleteListener 删除监听器
func (m *TargetSysManager) DeleteListener(ctx context.Context, subsystemNQN string, listenerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在 pkg 层删除
	if err := m.pkgManager.DeleteListener(ctx, subsystemNQN, listenerID); err != nil {
		return err
	}

	// 在内核删除需要先找到端口 ID
	// 简化实现：遍历所有端口找到匹配的
	return m.deleteListenerFromKernel(subsystemNQN, listenerID)
}

// deleteListenerFromKernel 从内核删除监听器
func (m *TargetSysManager) deleteListenerFromKernel(subsystemNQN string, listenerID string) error {
	portsDir, err := os.ReadDir(NVMetPortsPath)
	if err != nil {
		return nil
	}

	for _, entry := range portsDir {
		if !entry.IsDir() {
			continue
		}
		portID := entry.Name()
		linkPath := filepath.Join(NVMetPortsPath, portID, "subsystems", subsystemNQN)

		// 取消链接
		_ = os.Remove(linkPath)
	}

	return nil
}

// ========== 主机管理 ==========

// AddHost 添加允许的主机
func (m *TargetSysManager) AddHost(ctx context.Context, subsystemNQN string, req *pkgnvmeof.AddHostRequest) (*pkgnvmeof.Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在 pkg 层添加
	host, err := m.pkgManager.AddHost(ctx, subsystemNQN, req)
	if err != nil {
		return nil, err
	}

	// 在内核添加
	if err := m.addHostInKernel(subsystemNQN, req.NQN); err != nil {
		// 回滚 pkg 层
		_ = m.pkgManager.RemoveHost(ctx, subsystemNQN, req.NQN)
		return nil, fmt.Errorf("failed to add host in kernel: %w", err)
	}

	return host, nil
}

// addHostInKernel 在内核添加主机
func (m *TargetSysManager) addHostInKernel(subsystemNQN string, hostNQN string) error {
	subsysPath := filepath.Join(NVMetSubsystemsPath, subsystemNQN)
	hostPath := filepath.Join(subsysPath, NVMetHostsPath, hostNQN)

	return os.MkdirAll(hostPath, 0o755)
}

// RemoveHost 移除主机
func (m *TargetSysManager) RemoveHost(ctx context.Context, subsystemNQN string, hostNQN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在 pkg 层移除
	if err := m.pkgManager.RemoveHost(ctx, subsystemNQN, hostNQN); err != nil {
		return err
	}

	// 在内核移除
	return m.removeHostFromKernel(subsystemNQN, hostNQN)
}

// removeHostFromKernel 从内核移除主机
func (m *TargetSysManager) removeHostFromKernel(subsystemNQN string, hostNQN string) error {
	subsysPath := filepath.Join(NVMetSubsystemsPath, subsystemNQN)
	hostPath := filepath.Join(subsysPath, NVMetHostsPath, hostNQN)

	return os.Remove(hostPath)
}

// ========== 状态和统计 ==========

// Start 启动 Target 服务
func (m *TargetSysManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 确保 nvmet 模块已加载
	if err := m.loadModule("nvmet"); err != nil {
		return fmt.Errorf("failed to load nvmet module: %w", err)
	}

	// 根据需要加载传输模块
	if m.hasRDMAListeners() {
		if err := m.loadModule("nvmet-rdma"); err != nil {
			return fmt.Errorf("failed to load nvmet-rdma module: %w", err)
		}
	}
	if m.hasTCPListeners() {
		if err := m.loadModule("nvmet-tcp"); err != nil {
			return fmt.Errorf("failed to load nvmet-tcp module: %w", err)
		}
	}

	m.running = true
	return m.pkgManager.Start(ctx)
}

// Stop 停止 Target 服务
func (m *TargetSysManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	return m.pkgManager.Stop()
}

// hasRDMAListeners 检查是否有 RDMA 监听器
func (m *TargetSysManager) hasRDMAListeners() bool {
	for _, subsys := range m.pkgManager.ListSubsystems() {
		for _, listener := range subsys.Listeners {
			if listener.Transport == pkgnvmeof.TransportRDMA {
				return true
			}
		}
	}
	return false
}

// hasTCPListeners 检查是否有 TCP 监听器
func (m *TargetSysManager) hasTCPListeners() bool {
	for _, subsys := range m.pkgManager.ListSubsystems() {
		for _, listener := range subsys.Listeners {
			if listener.Transport == pkgnvmeof.TransportTCP {
				return true
			}
		}
	}
	return false
}

// GetSubsystem 获取子系统
func (m *TargetSysManager) GetSubsystem(nqn string) (*pkgnvmeof.Subsystem, error) {
	return m.pkgManager.GetSubsystem(nqn)
}

// ListSubsystems 列出子系统
func (m *TargetSysManager) ListSubsystems() []*pkgnvmeof.Subsystem {
	return m.pkgManager.ListSubsystems()
}

// GetStats 获取统计
func (m *TargetSysManager) GetStats() *pkgnvmeof.TargetStats {
	return m.pkgManager.GetStats()
}

// ========== 辅助方法 ==========

// GetDeviceInfo 获取 NVMe 设备信息
func (m *TargetSysManager) GetDeviceInfo(devicePath string) (size uint64, blockSize uint32, err error) {
	// 检查设备是否存在
	if _, err = os.Stat(devicePath); err != nil {
		return 0, 0, fmt.Errorf("device not found: %w", err)
	}

	// 使用 nvme-cli 获取设备信息
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvme", "id-ns", devicePath, "-n", "1")
	output, err := cmd.Output()
	if err == nil {
		// 解析输出获取容量
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "nsze") {
				// 解析 nsze (namespace size)
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					nsze, _ := strconv.ParseUint(parts[len(parts)-1], 10, 64)
					// 块大小默认 512
					size = nsze * 512
				}
			}
		}
	}

	// 如果无法获取，尝试读取 block size
	blockSize = DefaultBlockSize
	blockSizeFile := filepath.Join("/sys/block", filepath.Base(devicePath), "queue/logical_block_size")
	if data, err := os.ReadFile(blockSizeFile); err == nil {
		bs, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if bs > 0 {
			blockSize = uint32(bs)
		}
	}

	return size, blockSize, nil
}

// ValidateDevicePath 验证设备路径是否可用于 NVMe-oF
func (m *TargetSysManager) ValidateDevicePath(devicePath string) error {
	// 检查路径是否以 /dev/ 开头
	if !strings.HasPrefix(devicePath, "/dev/") {
		return fmt.Errorf("device path must start with /dev/")
	}

	// 检查设备是否存在
	if _, err := os.Stat(devicePath); err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// 检查是否是块设备
	info, err := os.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("cannot stat device: %w", err)
	}
	if info.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("not a block device")
	}

	return nil
}
