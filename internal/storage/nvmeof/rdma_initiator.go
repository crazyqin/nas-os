// Package nvmeof - NVMe-oF RDMA Initiator 系统实现
// 封装 Linux 内核 nvme-rdma 模块配置
// 通过 nvme-cli 连接远程 NVMe/RDMA Target

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

// ========== RDMA Initiator 常量定义 ==========

const (
	// RDMAInitiatorDefaultPort RDMA Initiator 默认端口
	RDMAInitiatorDefaultPort = 4420

	// RDMAInitiatorDefaultQueueDepth RDMA Initiator 默认队列深度
	RDMAInitiatorDefaultQueueDepth = 128

	// RDMAInitiatorDefaultIOQueues RDMA Initiator 默认 IO 队列数
	RDMAInitiatorDefaultIOQueues = 8

	// RDMAInitiatorDefaultKeepAlive RDMA Initiator 默认 Keep-alive 超时 (秒)
	RDMAInitiatorDefaultKeepAlive = 30

	// RDMAInitiatorDefaultReconnectDelay RDMA Initiator 默认重连延迟 (秒)
	RDMAInitiatorDefaultReconnectDelay = 10

	// RDMAInitiatorTimeout 命令超时时间 (秒)
	RDMAInitiatorTimeout = 60

	// NVMeClassPath NVMe 设备类路径
	NVMeClassPath = "/sys/class/nvme"
)

// ========== RDMA Initiator 系统管理器 ==========

// RDMAInitiatorSysManager RDMA Initiator 系统管理器
type RDMAInitiatorSysManager struct {
	mu sync.RWMutex

	// pkg 层 RDMA 管理器
	pkgRdmaManager *pkgnvmeof.RDMAManager

	// pkg 层 Initiator 管理器
	pkgInitiatorManager *pkgnvmeof.InitiatorManager

	// RDMA 配置
	rdmaConfig *pkgnvmeof.RDMAConfig

	// 控制器列表
	controllers map[string]*RDMAController

	// 运行状态
	running bool
}

// NewRDMAInitiatorSysManager 创建 RDMA Initiator 系统管理器
func NewRDMAInitiatorSysManager(rdmaConfig *pkgnvmeof.RDMAConfig, initiatorManager *pkgnvmeof.InitiatorManager) (*RDMAInitiatorSysManager, error) {
	if rdmaConfig == nil {
		rdmaConfig = pkgnvmeof.DefaultRDMAConfig()
	}

	pkgRdmaManager, err := pkgnvmeof.NewRDMAManager(rdmaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg rdma manager: %w", err)
	}

	m := &RDMAInitiatorSysManager{
		pkgRdmaManager:      pkgRdmaManager,
		pkgInitiatorManager: initiatorManager,
		rdmaConfig:          rdmaConfig,
		controllers:         make(map[string]*RDMAController),
	}

	// 检查 RDMA 模块是否可用
	if err := m.checkRDMAAvailable(); err != nil {
		return nil, fmt.Errorf("rdma initiator not available: %w", err)
	}

	// 加载现有连接
	m.loadExistingConnections()

	return m, nil
}

// checkRDMAAvailable 检查 RDMA Initiator 模块是否可用
func (m *RDMAInitiatorSysManager) checkRDMAAvailable() error {
	// 检查 nvme 类目录是否存在
	if _, err := os.Stat(NVMeClassPath); err != nil {
		// 尝试加载 nvme 模块
		if err := m.loadModule("nvme"); err != nil {
			return fmt.Errorf("nvme module not loaded: %w", err)
		}
	}

	// 检查 RDMA 设备
	if !m.pkgRdmaManager.IsAvailable() {
		return fmt.Errorf("rdma devices not available")
	}

	return nil
}

// loadModule 加载内核模块
func (m *RDMAInitiatorSysManager) loadModule(module string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "modprobe", module)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load module %s: %w, output: %s", module, err, output)
	}
	return nil
}

// loadExistingConnections 加载现有连接
func (m *RDMAInitiatorSysManager) loadExistingConnections() {
	// 遍历 /sys/class/nvme 目录
	entries, err := os.ReadDir(NVMeClassPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		nvmePath := filepath.Join(NVMeClassPath, entry.Name())

		// 检查传输类型
		trTypePath := filepath.Join(nvmePath, "transport")
		if data, err := os.ReadFile(trTypePath); err == nil {
			trType := strings.TrimSpace(string(data))
			if trType != "rdma" {
				continue
			}
		} else {
			continue
		}

		// 读取子系统 NQN
		subsysNQN := ""
		if data, err := os.ReadFile(filepath.Join(nvmePath, "subsysnqn")); err == nil {
			subsysNQN = strings.TrimSpace(string(data))
		}

		// 读取地址
		address := ""
		if data, err := os.ReadFile(filepath.Join(nvmePath, "address")); err == nil {
			// 地址格式: trtype=rdma,traddr=192.168.100.100,trsvcid=4420
			addrStr := strings.TrimSpace(string(data))
			for _, part := range strings.Split(addrStr, ",") {
				if strings.HasPrefix(part, "traddr=") {
					address = strings.TrimPrefix(part, "traddr=")
					break
				}
			}
		}

		// 读取端口
		port := RDMAInitiatorDefaultPort
		if data, err := os.ReadFile(filepath.Join(nvmePath, "address")); err == nil {
			addrStr := strings.TrimSpace(string(data))
			for _, part := range strings.Split(addrStr, ",") {
				if strings.HasPrefix(part, "trsvcid=") {
					svcid := strings.TrimPrefix(part, "trsvcid=")
					if p, err := strconv.Atoi(svcid); err == nil {
						port = p
					}
					break
				}
			}
		}

		// 创建控制器对象
		ctrl := &RDMAController{
			Name:         entry.Name(),
			TargetNQN:    subsysNQN,
			TargetAddress: address,
			TargetPort:   port,
			Transport:    pkgnvmeof.TransportRDMA,
			State:        pkgnvmeof.ControllerStateLive,
			ConnectedAt:  time.Now(),
		}

		m.controllers[ctrl.Name] = ctrl
	}
}

// ========== RDMA Initiator 连接管理 ==========

// ConnectRDMATarget 连接到 RDMA Target
func (m *RDMAInitiatorSysManager) ConnectRDMATarget(ctx context.Context, req *ConnectRDMATargetRequest) (*RDMAController, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证请求
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 检查是否已连接
	for _, ctrl := range m.controllers {
		if ctrl.TargetNQN == req.TargetNQN && ctrl.TargetAddress == req.TargetAddress {
			if ctrl.State == pkgnvmeof.ControllerStateLive {
				return nil, pkgnvmeof.ErrControllerConnected
			}
		}
	}

	// 加载 nvme-rdma 模块
	if err := m.loadModule("nvme-rdma"); err != nil {
		return nil, fmt.Errorf("failed to load nvme-rdma module: %w", err)
	}

	// 构建 nvme connect 命令参数
	args := []string{
		"connect",
		"-t", "rdma",
		"-n", req.TargetNQN,
		"-a", req.TargetAddress,
		"-s", strconv.Itoa(req.TargetPort),
	}

	// 设置队列深度
	if req.QueueDepth > 0 {
		args = append(args, "-q", strconv.Itoa(req.QueueDepth))
	} else {
		args = append(args, "-q", strconv.Itoa(m.rdmaConfig.QueueDepth))
	}

	// 设置 IO 队列数
	if req.IOQueues > 0 {
		args = append(args, "-i", strconv.Itoa(req.IOQueues))
	} else {
		args = append(args, "-i", strconv.Itoa(m.rdmaConfig.PortConfig.ServicePort))
	}

	// 设置 Keep-alive 超时
	if req.KeepAlive > 0 {
		args = append(args, "-k", strconv.Itoa(req.KeepAlive))
	} else {
		args = append(args, "-k", strconv.Itoa(m.rdmaConfig.Reconnect.Timeout))
	}

	// 设置 Host NQN (可选)
	if req.HostNQN != "" {
		args = append(args, "-I", req.HostNQN)
	}

	// 设置 Host ID (可选)
	if req.HostID != "" {
		args = append(args, "-S", req.HostID)
	}

	// 设置控制器丢失超时 (可选)
	if req.CtrlLossTimeout > 0 {
		args = append(args, "--ctrl-loss-tmo", strconv.Itoa(req.CtrlLossTimeout))
	}

	// 设置重连延迟 (可选)
	if req.ReconnectDelay > 0 {
		args = append(args, "-r", strconv.Itoa(req.ReconnectDelay))
	} else {
		args = append(args, "-r", strconv.Itoa(m.rdmaConfig.Reconnect.Delay))
	}

	// 执行 nvme connect 命令
	cmdCtx, cancel := context.WithTimeout(ctx, RDMAInitiatorTimeout*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "nvme", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nvme connect failed: %w, output: %s", err, output)
	}

	// 查找新创建的控制器
	// nvme connect 成功后，控制器会出现在 /sys/class/nvme/
	// 等待一小段时间让内核完成注册
	time.Sleep(500 * time.Millisecond)

	// 遍历查找新控制器
	newCtrlName := ""
	entries, err := os.ReadDir(NVMeClassPath)
	if err == nil {
		for _, entry := range entries {
			nvmePath := filepath.Join(NVMeClassPath, entry.Name())

			// 检查传输类型
			if data, err := os.ReadFile(filepath.Join(nvmePath, "transport")); err == nil {
				trType := strings.TrimSpace(string(data))
				if trType != "rdma" {
					continue
				}
			} else {
				continue
			}

			// 检查子系统 NQN
			if data, err := os.ReadFile(filepath.Join(nvmePath, "subsysnqn")); err == nil {
				subsysNQN := strings.TrimSpace(string(data))
				if subsysNQN == req.TargetNQN {
					// 检查地址
					if data, err := os.ReadFile(filepath.Join(nvmePath, "address")); err == nil {
						addrStr := strings.TrimSpace(string(data))
						for _, part := range strings.Split(addrStr, ",") {
							if strings.HasPrefix(part, "traddr=") {
								addr := strings.TrimPrefix(part, "traddr=")
								if addr == req.TargetAddress {
									newCtrlName = entry.Name()
									break
								}
							}
						}
					}
					break
				}
			}
		}
	}

	if newCtrlName == "" {
		// 无法找到控制器，使用请求信息创建
		newCtrlName = "nvme-rdma-" + req.TargetNQN
	}

	// 创建控制器对象
	ctrl := &RDMAController{
		Name:          newCtrlName,
		TargetNQN:     req.TargetNQN,
		TargetAddress: req.TargetAddress,
		TargetPort:    req.TargetPort,
		Transport:     pkgnvmeof.TransportRDMA,
		State:         pkgnvmeof.ControllerStateLive,
		QueueDepth:    req.QueueDepth,
		IOQueues:      req.IOQueues,
		KeepAlive:     req.KeepAlive,
		ReconnectDelay: req.ReconnectDelay,
		ConnectedAt:   time.Now(),
		Namespaces:    make([]*RDMANamespace, 0),
	}

	// 扫描命名空间
	m.scanNamespaces(ctrl)

	m.controllers[newCtrlName] = ctrl

	return ctrl, nil
}

// DisconnectRDMATarget 断开 RDMA Target 连接
func (m *RDMAInitiatorSysManager) DisconnectRDMATarget(ctx context.Context, ctrlName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctrl, exists := m.controllers[ctrlName]
	if !exists {
		return pkgnvmeof.ErrControllerDisconnected
	}

	// 执行 nvme disconnect 命令
	cmdCtx, cancel := context.WithTimeout(ctx, RDMAInitiatorTimeout*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "nvme", "disconnect", "-n", ctrl.TargetNQN)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nvme disconnect failed: %w, output: %s", err, output)
	}

	// 更新状态
	ctrl.State = pkgnvmeof.ControllerStateDead

	// 删除控制器
	delete(m.controllers, ctrlName)

	return nil
}

// DisconnectAllRDMATargets 断开所有 RDMA Target 连接
func (m *RDMAInitiatorSysManager) DisconnectAllRDMATargets(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 执行 nvme disconnect-all 命令
	cmdCtx, cancel := context.WithTimeout(ctx, RDMAInitiatorTimeout*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "nvme", "disconnect-all")
	_, err := cmd.CombinedOutput()
	if err != nil {
		// 尝逐个断开
		for _, ctrl := range m.controllers {
			if ctrl.Transport == pkgnvmeof.TransportRDMA {
				_ = m.disconnectSingleController(cmdCtx, ctrl)
			}
		}
	}

	// 清空控制器列表
	m.controllers = make(map[string]*RDMAController)

	return nil
}

// disconnectSingleController 断开单个控制器
func (m *RDMAInitiatorSysManager) disconnectSingleController(ctx context.Context, ctrl *RDMAController) error {
	cmd := exec.CommandContext(ctx, "nvme", "disconnect", "-n", ctrl.TargetNQN)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nvme disconnect failed: %w, output: %s", err, output)
	}
	return nil
}

// scanNamespaces 扫描控制器命名空间
func (m *RDMAInitiatorSysManager) scanNamespaces(ctrl *RDMAController) {
	// 遍历控制器下的命名空间
	// /sys/class/nvme/<ctrl>/device/nvme*
	ctrlPath := filepath.Join(NVMeClassPath, ctrl.Name)

	// 检查控制器路径是否存在
	if _, err := os.Stat(ctrlPath); err != nil {
		return
	}

	devicePath := filepath.Join(ctrlPath, "device")
	entries, err := os.ReadDir(devicePath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "nvme") && strings.Contains(name, "n") {
			// 这是命名空间设备
			nsPath := filepath.Join(devicePath, name)

			// 读取命名空间 ID
			nsid := 0
			if data, err := os.ReadFile(filepath.Join(nsPath, "nsid")); err == nil {
				nsid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			}

			// 读取块大小
			blockSize := 512
			if data, err := os.ReadFile(filepath.Join("/sys/block", name, "queue/logical_block_size")); err == nil {
				blockSize, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			}

			// 读取大小
			size := uint64(0)
			if data, err := os.ReadFile(filepath.Join("/sys/block", name, "size")); err == nil {
				sectors, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
				size = sectors * uint64(blockSize)
			}

			ns := &RDMANamespace{
				NSID:       uint32(nsid),
				Name:       name,
				DevicePath: filepath.Join("/dev", name),
				BlockSize:  uint32(blockSize),
				Size:       size,
				Controller: ctrl.Name,
				Online:     true,
			}

			ctrl.Namespaces = append(ctrl.Namespaces, ns)
		}
	}
}

// ListRDMAControllers 列出 RDMA 控制器
func (m *RDMAInitiatorSysManager) ListRDMAControllers() []*RDMAController {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RDMAController, 0, len(m.controllers))
	for _, ctrl := range m.controllers {
		result = append(result, ctrl)
	}
	return result
}

// GetRDMAController 获取 RDMA 控制器
func (m *RDMAInitiatorSysManager) GetRDMAController(name string) (*RDMAController, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctrl, exists := m.controllers[name]
	if !exists {
		return nil, pkgnvmeof.ErrControllerDisconnected
	}
	return ctrl, nil
}

// ========== RDMA Initiator 发现 ==========

// DiscoverRDMATargets 发现 RDMA Target
func (m *RDMAInitiatorSysManager) DiscoverRDMATargets(ctx context.Context, req *DiscoverRDMATargetsRequest) ([]*pkgnvmeof.DiscoveryLogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 加载 nvme-rdma 模块
	if err := m.loadModule("nvme-rdma"); err != nil {
		return nil, fmt.Errorf("failed to load nvme-rdma module: %w", err)
	}

	// 构建 nvme discover 命令参数
	args := []string{
		"discover",
		"-t", "rdma",
		"-a", req.Address,
		"-s", strconv.Itoa(req.Port),
	}

	// 设置 Host NQN (可选)
	if req.HostNQN != "" {
		args = append(args, "-I", req.HostNQN)
	}

	// 设置 Host ID (可选)
	if req.HostID != "" {
		args = append(args, "-S", req.HostID)
	}

	// 执行 nvme discover 命令
	cmdCtx, cancel := context.WithTimeout(ctx, RDMAInitiatorTimeout*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "nvme", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvme discover failed: %w, output: %s", err, output)
	}

	// 解析发现日志
	return m.parseDiscoveryLog(string(output)), nil
}

// parseDiscoveryLog 解析发现日志
func (m *RDMAInitiatorSysManager) parseDiscoveryLog(output string) []*pkgnvmeof.DiscoveryLogEntry {
	entries := make([]*pkgnvmeof.DiscoveryLogEntry, 0)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 查找子系统条目
		// nvme discover 输出格式示例:
		// Discovery Log Number of Entries 2, Generation counter 3
		// =====Discovery Log Entry 0======
		// trtype:  rdma
		// adrfam:  ipv4
		// subtype: nvme
		// treq:    not specified
		// portid:  1
		// trsvcid: 4420
		// subnqn:  nqn.2026-03.org.nas-os:subsystem1
		// traddr:  192.168.100.100

		if strings.HasPrefix(line, "subnqn:") {
			nqn := strings.TrimSpace(strings.TrimPrefix(line, "subnqn:"))
			// 查找同一块中的其他信息
			entry := &pkgnvmeof.DiscoveryLogEntry{
				SubsysNQN: nqn,
				Transport: pkgnvmeof.TransportRDMA,
			}
			entries = append(entries, entry)
		}

		if strings.HasPrefix(line, "traddr:") {
			// 更新最后一条条目的地址
			if len(entries) > 0 {
				addr := strings.TrimSpace(strings.TrimPrefix(line, "traddr:"))
				entries[len(entries)-1].TrAddress = addr
			}
		}

		if strings.HasPrefix(line, "trsvcid:") {
			// 更新最后一条条目的端口
			if len(entries) > 0 {
				port := strings.TrimSpace(strings.TrimPrefix(line, "trsvcid:"))
				entries[len(entries)-1].TrSVCID = port
			}
		}
	}

	return entries
}

// ========== RDMA Initiator 服务管理 ==========

// Start 启动 RDMA Initiator 服务
func (m *RDMAInitiatorSysManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 加载必要内核模块
	if err := m.loadModule("nvme"); err != nil {
		return fmt.Errorf("failed to load nvme module: %w", err)
	}

	if err := m.loadModule("nvme-core"); err != nil {
		// nvme-core 可能已内置
		_ = err
	}

	if err := m.loadModule("nvme-rdma"); err != nil {
		return fmt.Errorf("failed to load nvme-rdma module: %w", err)
	}

	// 加载 RDMA 相关模块
	rdmaModules := []string{"ib_core", "ib_uverbs", "rdma_cm", "rdma_ucm"}
	for _, module := range rdmaModules {
		if err := m.loadModule(module); err != nil {
			// 部分模块可能已内置，忽略错误
			_ = err
		}
	}

	m.running = true

	return m.pkgRdmaManager.Start(ctx)
}

// Stop 停止 RDMA Initiator 服务
func (m *RDMAInitiatorSysManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// 断开所有连接
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = m.DisconnectAllRDMATargets(ctx)

	m.running = false
	return m.pkgRdmaManager.Stop()
}

// IsRunning 检查是否运行中
func (m *RDMAInitiatorSysManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== RDMA Initiator 统计 ==========

// GetRDMAInitiatorStats 获取 RDMA Initiator 统计信息
func (m *RDMAInitiatorSysManager) GetRDMAInitiatorStats() *RDMAInitiatorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &RDMAInitiatorStats{
		Available: m.pkgRdmaManager.IsAvailable(),
		Running:   m.running,
		ControllerCount: len(m.controllers),
	}

	// 计算命名空间数量
	for _, ctrl := range m.controllers {
		stats.NamespaceCount += len(ctrl.Namespaces)
		if ctrl.State == pkgnvmeof.ControllerStateLive {
			stats.ActiveConnections++
		}
	}

	return stats
}

// ========== RDMA 控制器对象 ==========

// RDMAController RDMA 控制器
type RDMAController struct {
	// 控制器名称
	Name string `json:"name"`

	// 目标子系统 NQN
	TargetNQN string `json:"targetNqn"`

	// 目标地址
	TargetAddress string `json:"targetAddress"`

	// 目标端口
	TargetPort int `json:"targetPort"`

	// 传输类型
	Transport pkgnvmeof.TransportType `json:"transport"`

	// 状态
	State pkgnvmeof.ControllerState `json:"state"`

	// 配置
	QueueDepth    int `json:"queueDepth"`
	IOQueues      int `json:"ioQueues"`
	KeepAlive     int `json:"keepAlive"`
	ReconnectDelay int `json:"reconnectDelay"`

	// 命名空间列表
	Namespaces []*RDMANamespace `json:"namespaces"`

	// 连接时间
	ConnectedAt time.Time `json:"connectedAt"`

	// 重连次数
	ReconnectCount int `json:"reconnectCount"`

	// 统计
	Stats pkgnvmeof.ControllerStats `json:"stats"`
}

// RDMANamespace RDMA 命名空间
type RDMANamespace struct {
	// 命名空间 ID
	NSID uint32 `json:"nsid"`

	// 名称
	Name string `json:"name"`

	// 设备路径
	DevicePath string `json:"devicePath"`

	// 块大小
	BlockSize uint32 `json:"blockSize"`

	// 大小 (字节)
	Size uint64 `json:"size"`

	// 所属控制器
	Controller string `json:"controller"`

	// 在线状态
	Online bool `json:"online"`

	// 只读状态
	ReadOnly bool `json:"readOnly"`
}

// RDMAInitiatorStats RDMA Initiator 统计
type RDMAInitiatorStats struct {
	// 可用性
	Available bool `json:"available"`

	// 运行状态
	Running bool `json:"running"`

	// 控制器数量
	ControllerCount int `json:"controllerCount"`

	// 命名空间数量
	NamespaceCount int `json:"namespaceCount"`

	// 活动连接数
	ActiveConnections int `json:"activeConnections"`

	// 总发送字节
	TotalTxBytes uint64 `json:"totalTxBytes"`

	// 总接收字节
	TotalRxBytes uint64 `json:"totalRxBytes"`

	// 总发送 IOPS
	TotalTxIOPS uint64 `json:"totalTxIops"`

	// 总接收 IOPS
	TotalRxIOPS uint64 `json:"totalRxIops"`

	// 错误计数
	ErrorCount uint64 `json:"errorCount"`

	// 重连计数
	ReconnectCount uint64 `json:"reconnectCount"`
}

// ========== RDMA Initiator 请求 ==========

// ConnectRDMATargetRequest 连接 RDMA Target 请求
type ConnectRDMATargetRequest struct {
	// 目标子系统 NQN
	TargetNQN string `json:"targetNqn"`

	// 目标地址
	TargetAddress string `json:"targetAddress"`

	// 目标端口
	TargetPort int `json:"targetPort"`

	// Host NQN
	HostNQN string `json:"hostNqn"`

	// Host ID
	HostID string `json:"hostId"`

	// 队列深度
	QueueDepth int `json:"queueDepth"`

	// IO 队列数
	IOQueues int `json:"ioQueues"`

	// Keep-alive 超时 (秒)
	KeepAlive int `json:"keepAlive"`

	// 重连延迟 (秒)
	ReconnectDelay int `json:"reconnectDelay"`

	// 控制器丢失超时 (秒)
	CtrlLossTimeout int `json:"ctrlLossTimeout"`

	// 是否启用轮询模式
	PollMode bool `json:"pollMode"`

	// DHCHAP 密钥
	DHCHAPKey string `json:"dhchapKey"`
}

// Validate 验证请求
func (r *ConnectRDMATargetRequest) Validate() error {
	if r.TargetNQN == "" {
		return fmt.Errorf("target_nqn is required")
	}

	if r.TargetAddress == "" {
		return fmt.Errorf("target_address is required")
	}

	if r.TargetPort <= 0 || r.TargetPort > 65535 {
		r.TargetPort = RDMAInitiatorDefaultPort
	}

	if r.QueueDepth <= 0 {
		r.QueueDepth = RDMAInitiatorDefaultQueueDepth
	}

	if r.IOQueues <= 0 {
		r.IOQueues = RDMAInitiatorDefaultIOQueues
	}

	if r.KeepAlive <= 0 {
		r.KeepAlive = RDMAInitiatorDefaultKeepAlive
	}

	if r.ReconnectDelay <= 0 {
		r.ReconnectDelay = RDMAInitiatorDefaultReconnectDelay
	}

	return nil
}

// DiscoverRDMATargetsRequest 发现 RDMA Target 请求
type DiscoverRDMATargetsRequest struct {
	// 发现服务地址
	Address string `json:"address"`

	// 发现服务端口
	Port int `json:"port"`

	// Host NQN
	HostNQN string `json:"hostNqn"`

	// Host ID
	HostID string `json:"hostId"`
}

// Validate 验证请求
func (r *DiscoverRDMATargetsRequest) Validate() error {
	if r.Address == "" {
		return fmt.Errorf("address is required")
	}

	if r.Port <= 0 || r.Port > 65535 {
		r.Port = pkgnvmeof.DefaultNVMeOFConfig().Target.DefaultPort
	}

	return nil
}