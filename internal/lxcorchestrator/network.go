package lxcorchestrator

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NetworkManager 网络管理器
type NetworkManager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	orchestrator *Orchestrator
	networks     map[string]*NetworkConfig
	containers   map[string]string // container_id -> network_id
	ipPool       map[string]*IPPool
}

// IPPool IP 地址池
type IPPool struct {
	Subnet     *net.IPNet
	Gateway    net.IP
	Allocated  map[string]bool // IP -> 是否已分配
	NextIP     net.IP
}

// NewNetworkManager 创建网络管理器
func NewNetworkManager(logger *zap.Logger, orchestrator *Orchestrator) *NetworkManager {
	return &NetworkManager{
		logger:       logger,
		orchestrator: orchestrator,
		networks:     make(map[string]*NetworkConfig),
		containers:   make(map[string]string),
		ipPool:       make(map[string]*IPPool),
	}
}

// InitDefaultNetwork 初始化默认网络
func (nm *NetworkManager) InitDefaultNetwork(ctx context.Context) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 检查是否已存在默认网络
	if _, exists := nm.networks["lxc-bridge"]; exists {
		return nil
	}

	// 创建默认桥接网络
	defaultNetwork := &NetworkConfig{
		ID:         "lxc-bridge",
		Name:       "lxc-bridge",
		Mode:       NetworkBridge,
		Subnet:     "10.0.3.0/24",
		Gateway:    "10.0.3.1",
		BridgeName: "lxcbr0",
		DNS:        []string{"8.8.8.8", "8.8.4.4"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := nm.createNetwork(ctx, defaultNetwork); err != nil {
		return fmt.Errorf("failed to create default network: %w", err)
	}

	nm.networks["lxc-bridge"] = defaultNetwork

	nm.logger.Info("default network initialized",
		zap.String("id", defaultNetwork.ID),
		zap.String("subnet", defaultNetwork.Subnet),
	)

	return nil
}

// CreateNetwork 创建网络
func (nm *NetworkManager) CreateNetwork(ctx context.Context, config *NetworkConfig) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// 检查名称重复
	for _, n := range nm.networks {
		if n.Name == config.Name {
			return fmt.Errorf("network name already exists: %s", config.Name)
		}
	}

	// 验证子网
	if config.Subnet != "" {
		_, _, err := net.ParseCIDR(config.Subnet)
		if err != nil {
			return fmt.Errorf("invalid subnet: %w", err)
		}
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if err := nm.createNetwork(ctx, config); err != nil {
		return err
	}

	nm.networks[config.ID] = config

	nm.logger.Info("network created",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("mode", string(config.Mode)),
	)

	return nil
}

// DeleteNetwork 删除网络
func (nm *NetworkManager) DeleteNetwork(ctx context.Context, networkID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	network, exists := nm.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found: %s", networkID)
	}

	// 检查是否有容器使用此网络
	for _, containerID := range network.Containers {
		return fmt.Errorf("network in use by container: %s", containerID)
	}

	if err := nm.deleteNetwork(ctx, network); err != nil {
		return err
	}

	delete(nm.networks, networkID)
	delete(nm.ipPool, networkID)

	nm.logger.Info("network deleted", zap.String("id", networkID))
	return nil
}

// GetNetwork 获取网络配置
func (nm *NetworkManager) GetNetwork(networkID string) (*NetworkConfig, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	network, exists := nm.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found: %s", networkID)
	}

	return network, nil
}

// ListNetworks 列出网络
func (nm *NetworkManager) ListNetworks() []*NetworkConfig {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	networks := make([]*NetworkConfig, 0, len(nm.networks))
	for _, n := range nm.networks {
		networks = append(networks, n)
	}

	return networks
}

// ConfigureContainer 配置容器网络
func (nm *NetworkManager) ConfigureContainer(ctx context.Context, container *ContainerInstance) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	networkID := nm.getNetworkForContainer(container)

	network, exists := nm.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found: %s", networkID)
	}

	// 分配 IP 地址
	ip, err := nm.allocateIP(networkID)
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %w", err)
	}

	// 记录容器网络关联
	nm.containers[container.Config.ID] = networkID
	network.Containers = append(network.Containers, container.Config.ID)

	container.IPAddress = ip.String()

	nm.logger.Info("container network configured",
		zap.String("container_id", container.Config.ID),
		zap.String("network_id", networkID),
		zap.String("ip", container.IPAddress),
	)

	return nil
}

// RemoveContainer 移除容器网络配置
func (nm *NetworkManager) RemoveContainer(ctx context.Context, container *ContainerInstance) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	networkID, exists := nm.containers[container.Config.ID]
	if !exists {
		return
	}

	// 释放 IP 地址
	if container.IPAddress != "" {
		ip := net.ParseIP(container.IPAddress)
		if ip != nil {
			nm.releaseIP(networkID, ip)
		}
	}

	// 从网络中移除容器
	if network, ok := nm.networks[networkID]; ok {
		for i, id := range network.Containers {
			if id == container.Config.ID {
				network.Containers = append(network.Containers[:i], network.Containers[i+1:]...)
				break
			}
		}
	}

	delete(nm.containers, container.Config.ID)

	nm.logger.Info("container network removed",
		zap.String("container_id", container.Config.ID),
		zap.String("network_id", networkID),
	)
}

// GetContainerIP 获取容器 IP 地址
func (nm *NetworkManager) GetContainerIP(containerID string) string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// 从编排器获取容器信息
	container, err := nm.orchestrator.GetContainer(containerID)
	if err != nil {
		return ""
	}

	return container.IPAddress
}

// GetContainerNetwork 获取容器所在网络
func (nm *NetworkManager) GetContainerNetwork(containerID string) (*NetworkConfig, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	networkID, exists := nm.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("container not in any network: %s", containerID)
	}

	network, exists := nm.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found: %s", networkID)
	}

	return network, nil
}

// ConnectContainer 连接容器到网络
func (nm *NetworkManager) ConnectContainer(ctx context.Context, containerID, networkID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	container, err := nm.orchestrator.GetContainer(containerID)
	if err != nil {
		return err
	}

	network, exists := nm.networks[networkID]
	if !exists {
		return fmt.Errorf("network not found: %s", networkID)
	}

	// 检查是否已在网络中
	if currentNetwork, ok := nm.containers[containerID]; ok && currentNetwork == networkID {
		return fmt.Errorf("container already in network: %s", networkID)
	}

	// 分配 IP
	ip, err := nm.allocateIP(networkID)
	if err != nil {
		return err
	}

	nm.containers[containerID] = networkID
	network.Containers = append(network.Containers, containerID)
	container.IPAddress = ip.String()

	nm.logger.Info("container connected to network",
		zap.String("container_id", containerID),
		zap.String("network_id", networkID),
		zap.String("ip", ip.String()),
	)

	return nil
}

// DisconnectContainer 断开容器与网络的连接
func (nm *NetworkManager) DisconnectContainer(ctx context.Context, containerID, networkID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	currentNetwork, exists := nm.containers[containerID]
	if !exists || currentNetwork != networkID {
		return fmt.Errorf("container not in network: %s", networkID)
	}

	container, err := nm.orchestrator.GetContainer(containerID)
	if err != nil {
		return err
	}

	// 释放 IP
	if container.IPAddress != "" {
		ip := net.ParseIP(container.IPAddress)
		if ip != nil {
			nm.releaseIP(networkID, ip)
		}
	}

	// 从网络中移除
	if network, ok := nm.networks[networkID]; ok {
		for i, id := range network.Containers {
			if id == containerID {
				network.Containers = append(network.Containers[:i], network.Containers[i+1:]...)
				break
			}
		}
	}

	delete(nm.containers, containerID)
	container.IPAddress = ""

	nm.logger.Info("container disconnected from network",
		zap.String("container_id", containerID),
		zap.String("network_id", networkID),
	)

	return nil
}

// getNetworkForContainer 获取容器应该使用的网络
func (nm *NetworkManager) getNetworkForContainer(container *ContainerInstance) string {
	// 如果指定了网络模式
	switch container.Config.NetworkMode {
	case NetworkHost:
		return "host"
	case NetworkNone:
		return "none"
	case NetworkCustom:
		// 使用默认网络
		return nm.orchestrator.config.DefaultNetwork
	default:
		// 使用默认桥接网络
		return nm.orchestrator.config.DefaultNetwork
	}
}

// allocateIP 分配 IP 地址
func (nm *NetworkManager) allocateIP(networkID string) (net.IP, error) {
	pool, exists := nm.ipPool[networkID]
	if !exists {
		// 初始化 IP 池
		network, ok := nm.networks[networkID]
		if !ok {
			return nil, fmt.Errorf("network not found: %s", networkID)
		}

		if network.Subnet == "" {
			return nil, fmt.Errorf("network has no subnet: %s", networkID)
		}

		_, subnet, err := net.ParseCIDR(network.Subnet)
		if err != nil {
			return nil, fmt.Errorf("invalid subnet: %w", err)
		}

		gateway := net.ParseIP(network.Gateway)
		if gateway == nil {
			// 使用子网的第一个 IP 作为网关
			gateway = make(net.IP, len(subnet.IP))
			copy(gateway, subnet.IP)
			gateway[len(gateway)-1]++
		}

		pool = &IPPool{
			Subnet:    subnet,
			Gateway:   gateway,
			Allocated: make(map[string]bool),
			NextIP:    nextIP(subnet.IP),
		}

		// 跳过网关 IP
		pool.NextIP = nextIP(gateway)

		nm.ipPool[networkID] = pool
	}

	// 查找可用 IP
	ip := pool.NextIP
	for {
		if !pool.Subnet.Contains(ip) {
			return nil, fmt.Errorf("no available IP in network: %s", networkID)
		}

		ipStr := ip.String()
		if !pool.Allocated[ipStr] && !ip.Equal(pool.Gateway) {
			pool.Allocated[ipStr] = true
			pool.NextIP = nextIP(ip)
			return ip, nil
		}

		ip = nextIP(ip)
		if ip.Equal(pool.NextIP) {
			return nil, fmt.Errorf("no available IP in network: %s", networkID)
		}
	}
}

// releaseIP 释放 IP 地址
func (nm *NetworkManager) releaseIP(networkID string, ip net.IP) {
	pool, exists := nm.ipPool[networkID]
	if !exists {
		return
	}

	delete(pool.Allocated, ip.String())
}

// createNetwork 创建网络 (系统调用)
func (nm *NetworkManager) createNetwork(ctx context.Context, config *NetworkConfig) error {
	// 模拟创建 Linux 桥接网络
	// 实际实现需要调用 ip link add, brctl addif 等命令
	nm.logger.Debug("creating network",
		zap.String("id", config.ID),
		zap.String("mode", string(config.Mode)),
		zap.String("bridge", config.BridgeName),
	)

	return nil
}

// deleteNetwork 删除网络 (系统调用)
func (nm *NetworkManager) deleteNetwork(ctx context.Context, config *NetworkConfig) error {
	// 模拟删除网络
	nm.logger.Debug("deleting network", zap.String("id", config.ID))
	return nil
}

// nextIP 获取下一个 IP 地址
func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}

// GetNetworkStats 获取网络统计信息
func (nm *NetworkManager) GetNetworkStats(networkID string) (map[string]interface{}, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	network, exists := nm.networks[networkID]
	if !exists {
		return nil, fmt.Errorf("network not found: %s", networkID)
	}

	stats := map[string]interface{}{
		"id":         network.ID,
		"name":       network.Name,
		"mode":       network.Mode,
		"subnet":     network.Subnet,
		"gateway":    network.Gateway,
		"containers": len(network.Containers),
		"isolated":   network.Isolated,
	}

	// IP 池统计
	if pool, ok := nm.ipPool[networkID]; ok {
		stats["allocated_ips"] = len(pool.Allocated)
		stats["available_ips"] = calculateAvailableIPs(pool.Subnet) - len(pool.Allocated) - 1 // 减去网关
	}

	return stats, nil
}

// calculateAvailableIPs 计算子网中可用的 IP 数量
func calculateAvailableIPs(subnet *net.IPNet) int {
	ones, bits := subnet.Mask.Size()
	return 1 << (uint(bits) - uint(ones))
}
