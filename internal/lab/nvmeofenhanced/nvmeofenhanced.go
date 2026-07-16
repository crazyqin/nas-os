package nvmeofenhanced

import (
	"time"
)

// NewNVMeOFManager 创建NVMe-oF管理器.
func NewNVMeOFManager(config ManagerConfig) *NVMeOFManager {
	return &NVMeOFManager{
		subsystems:      make(map[string]*NVMeSubsystem),
		namespaces:      make(map[string]*NVMeNamespace),
		controllers:     make(map[string]*NVMeController),
		interfaces:      make(map[string]*NetworkInterface),
		rdmaConfig:      DefaultRDMAConfig(),
		metrics:         make(map[string]*PerformanceMetrics),
		connectionPools: make(map[string]*ConnectionPool),
		config:          config,
	}
}

// CreateSubsystem 创建NVMe子系统.
func (m *NVMeOFManager) CreateSubsystem(subsystem *NVMeSubsystem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.subsystems[subsystem.ID]; exists {
		return ErrSubsystemExists
	}

	if len(m.subsystems) >= m.config.MaxSubsystems {
		return ErrMaxSubsystems
	}

	subsystem.CreatedAt = time.Now()
	subsystem.UpdatedAt = time.Now()
	subsystem.IsOnline = true
	m.subsystems[subsystem.ID] = subsystem

	return nil
}

// GetSubsystem 获取NVMe子系统.
func (m *NVMeOFManager) GetSubsystem(subsystemID string) (*NVMeSubsystem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsystem, exists := m.subsystems[subsystemID]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	return subsystem, nil
}

// ListSubsystems 列出所有NVMe子系统.
func (m *NVMeOFManager) ListSubsystems(transport TransportType) []*NVMeSubsystem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subsystems := make([]*NVMeSubsystem, 0)
	for _, subsystem := range m.subsystems {
		if transport == "" || subsystem.Transport == transport {
			subsystems = append(subsystems, subsystem)
		}
	}

	return subsystems
}

// UpdateSubsystem 更新NVMe子系统.
func (m *NVMeOFManager) UpdateSubsystem(subsystemID string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem, exists := m.subsystems[subsystemID]
	if !exists {
		return ErrSubsystemNotFound
	}

	if alias, ok := updates["alias"].(string); ok {
		subsystem.Alias = alias
	}
	if desc, ok := updates["description"].(string); ok {
		subsystem.Description = desc
	}
	if online, ok := updates["is_online"].(bool); ok {
		subsystem.IsOnline = online
	}

	subsystem.UpdatedAt = time.Now()

	return nil
}

// DeleteSubsystem 删除NVMe子系统.
func (m *NVMeOFManager) DeleteSubsystem(subsystemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.subsystems[subsystemID]; !exists {
		return ErrSubsystemNotFound
	}

	// 删除关联的命名空间和控制器
	for nsID, ns := range m.namespaces {
		if ns.SubsystemID == subsystemID {
			delete(m.namespaces, nsID)
		}
	}

	for ctrlID, ctrl := range m.controllers {
		if ctrl.SubsystemID == subsystemID {
			delete(m.controllers, ctrlID)
		}
	}

	delete(m.subsystems, subsystemID)

	return nil
}

// CreateNamespace 创建NVMe命名空间.
func (m *NVMeOFManager) CreateNamespace(namespace *NVMeNamespace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查子系统是否存在
	if _, exists := m.subsystems[namespace.SubsystemID]; !exists {
		return ErrSubsystemNotFound
	}

	// 检查命名空间数量限制
	count := 0
	for _, ns := range m.namespaces {
		if ns.SubsystemID == namespace.SubsystemID {
			count++
		}
	}
	if count >= m.config.MaxNamespacesPerSub {
		return ErrMaxNamespaces
	}

	namespace.CreatedAt = time.Now()
	namespace.UpdatedAt = time.Now()
	namespace.IsOnline = true
	m.namespaces[namespace.ID] = namespace

	return nil
}

// GetNamespace 获取NVMe命名空间.
func (m *NVMeOFManager) GetNamespace(namespaceID string) (*NVMeNamespace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	namespace, exists := m.namespaces[namespaceID]
	if !exists {
		return nil, ErrNamespaceNotFound
	}

	return namespace, nil
}

// ListNamespaces 列出所有NVMe命名空间.
func (m *NVMeOFManager) ListNamespaces(subsystemID string) []*NVMeNamespace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	namespaces := make([]*NVMeNamespace, 0)
	for _, ns := range m.namespaces {
		if subsystemID == "" || ns.SubsystemID == subsystemID {
			namespaces = append(namespaces, ns)
		}
	}

	return namespaces
}

// DeleteNamespace 删除NVMe命名空间.
func (m *NVMeOFManager) DeleteNamespace(namespaceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.namespaces[namespaceID]; !exists {
		return ErrNamespaceNotFound
	}

	delete(m.namespaces, namespaceID)

	return nil
}

// ConnectController 连接NVMe控制器.
func (m *NVMeOFManager) ConnectController(controller *NVMeController) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查子系统是否存在
	if _, exists := m.subsystems[controller.SubsystemID]; !exists {
		return ErrSubsystemNotFound
	}

	// 检查控制器数量限制
	count := 0
	for _, ctrl := range m.controllers {
		if ctrl.SubsystemID == controller.SubsystemID {
			count++
		}
	}
	if count >= m.config.MaxControllersPerSub {
		return ErrMaxControllers
	}

	controller.ConnectedAt = time.Now()
	controller.IsOnline = true
	m.controllers[controller.ID] = controller

	return nil
}

// GetController 获取NVMe控制器.
func (m *NVMeOFManager) GetController(controllerID string) (*NVMeController, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	controller, exists := m.controllers[controllerID]
	if !exists {
		return nil, ErrControllerNotFound
	}

	return controller, nil
}

// ListControllers 列出所有NVMe控制器.
func (m *NVMeOFManager) ListControllers(subsystemID string) []*NVMeController {
	m.mu.RLock()
	defer m.mu.RUnlock()

	controllers := make([]*NVMeController, 0)
	for _, ctrl := range m.controllers {
		if subsystemID == "" || ctrl.SubsystemID == subsystemID {
			controllers = append(controllers, ctrl)
		}
	}

	return controllers
}

// DisconnectController 断开NVMe控制器.
func (m *NVMeOFManager) DisconnectController(controllerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	controller, exists := m.controllers[controllerID]
	if !exists {
		return ErrControllerNotFound
	}

	controller.IsOnline = false
	delete(m.controllers, controllerID)

	return nil
}

// AddNetworkInterface 添加网络接口.
func (m *NVMeOFManager) AddNetworkInterface(iface *NetworkInterface) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	iface.UpdatedAt = time.Now()
	m.interfaces[iface.ID] = iface

	return nil
}

// GetNetworkInterface 获取网络接口.
func (m *NVMeOFManager) GetNetworkInterface(ifaceID string) (*NetworkInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iface, exists := m.interfaces[ifaceID]
	if !exists {
		return nil, ErrInterfaceNotFound
	}

	return iface, nil
}

// ListNetworkInterfaces 列出所有网络接口.
func (m *NVMeOFManager) ListNetworkInterfaces(speed NetworkSpeed) []*NetworkInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()

	interfaces := make([]*NetworkInterface, 0)
	for _, iface := range m.interfaces {
		if speed == "" || iface.Speed == speed {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces
}

// UpdateNetworkInterface 更新网络接口.
func (m *NVMeOFManager) UpdateNetworkInterface(ifaceID string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	iface, exists := m.interfaces[ifaceID]
	if !exists {
		return ErrInterfaceNotFound
	}

	if ip, ok := updates["ip_address"].(string); ok {
		iface.IPAddress = ip
	}
	if subnet, ok := updates["subnet"].(string); ok {
		iface.Subnet = subnet
	}
	if gateway, ok := updates["gateway"].(string); ok {
		iface.Gateway = gateway
	}
	if mtu, ok := updates["mtu"].(int); ok {
		iface.MTU = mtu
	}
	if online, ok := updates["is_online"].(bool); ok {
		iface.IsOnline = online
	}

	iface.UpdatedAt = time.Now()

	return nil
}

// ConfigureRDMA 配置RDMA.
func (m *NVMeOFManager) ConfigureRDMA(config RDMAConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.EnableRDMA {
		return ErrRDMANotSupported
	}

	m.rdmaConfig = config

	return nil
}

// GetRDMAConfig 获取RDMA配置.
func (m *NVMeOFManager) GetRDMAConfig() RDMAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.rdmaConfig
}

// UpdateMetrics 更新性能指标.
func (m *NVMeOFManager) UpdateMetrics(subsystemID string, metrics *PerformanceMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics.Timestamp = time.Now()
	m.metrics[subsystemID] = metrics

	return nil
}

// GetMetrics 获取性能指标.
func (m *NVMeOFManager) GetMetrics(subsystemID string) (*PerformanceMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[subsystemID]
	if !exists {
		return nil, ErrSubsystemNotFound
	}

	return metrics, nil
}

// GetSubsystemStats 获取子系统统计.
func (m *NVMeOFManager) GetSubsystemStats(subsystemID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})

	subsystem, exists := m.subsystems[subsystemID]
	if !exists {
		return stats
	}

	stats["id"] = subsystem.ID
	stats["nqn"] = subsystem.NQN
	stats["transport"] = subsystem.Transport
	stats["is_online"] = subsystem.IsOnline

	// 统计命名空间数量
	nsCount := 0
	for _, ns := range m.namespaces {
		if ns.SubsystemID == subsystemID {
			nsCount++
		}
	}
	stats["namespace_count"] = nsCount

	// 统计控制器数量
	ctrlCount := 0
	for _, ctrl := range m.controllers {
		if ctrl.SubsystemID == subsystemID {
			ctrlCount++
		}
	}
	stats["controller_count"] = ctrlCount

	// 获取性能指标
	if metrics, exists := m.metrics[subsystemID]; exists {
		stats["read_iops"] = metrics.ReadIOPS
		stats["write_iops"] = metrics.WriteIOPS
		stats["total_iops"] = metrics.TotalIOPS
		stats["read_throughput_mbps"] = metrics.ReadThroughputMBps
		stats["write_throughput_mbps"] = metrics.WriteThroughputMBps
		stats["avg_latency_us"] = metrics.AvgLatencyUs
	}

	return stats
}

// GetGlobalStats 获取全局统计.
func (m *NVMeOFManager) GetGlobalStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})

	stats["total_subsystems"] = len(m.subsystems)
	stats["total_namespaces"] = len(m.namespaces)
	stats["total_controllers"] = len(m.controllers)
	stats["total_interfaces"] = len(m.interfaces)

	// 统计各传输类型的子系统数量
	transportStats := make(map[string]int)
	for _, subsystem := range m.subsystems {
		transportStats[string(subsystem.Transport)]++
	}
	stats["transport_stats"] = transportStats

	// 统计在线子系统数量
	onlineCount := 0
	for _, subsystem := range m.subsystems {
		if subsystem.IsOnline {
			onlineCount++
		}
	}
	stats["online_subsystems"] = onlineCount

	// 统计400GbE接口数量
	speed400gCount := 0
	for _, iface := range m.interfaces {
		if iface.Speed == Speed400G {
			speed400gCount++
		}
	}
	stats["400gbe_interfaces"] = speed400gCount

	return stats
}

// FormatSpeed 格式化网络速度.
func FormatSpeed(speed NetworkSpeed) string {
	switch speed {
	case Speed10G:
		return "10 GbE"
	case Speed25G:
		return "25 GbE"
	case Speed40G:
		return "40 GbE"
	case Speed100G:
		return "100 GbE"
	case Speed200G:
		return "200 GbE"
	case Speed400G:
		return "400 GbE"
	default:
		return "Unknown"
	}
}

// FormatTransport 格式化传输类型.
func FormatTransport(transport TransportType) string {
	switch transport {
	case TransportTCP:
		return "NVMe/TCP"
	case TransportRDMA:
		return "NVMe/RDMA"
	case TransportFC:
		return "NVMe/FC"
	case TransportIB:
		return "NVMe/IB"
	default:
		return "Unknown"
	}
}
