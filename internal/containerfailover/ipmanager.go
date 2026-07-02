// Package containerfailover 容器 HA 故障转移模块
package containerfailover

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// IPManager 静态 IP 管理器
// 负责容器 IP 的分配、释放、迁移和 ARP 广播。
// TrueNAS 26 HA 容器要求静态 IP 配置，故障转移时 IP 随容器迁移。
type IPManager struct {
	mu sync.RWMutex
	// allocations IP 分配记录表：IP -> *IPAllocation
	allocations map[string]*IPAllocation
	// containerIPs 容器 ID -> IP 的映射，方便快速查找
	containerIPs map[string]string
}

// NewIPManager 创建 IP 管理器.
func NewIPManager() *IPManager {
	return &IPManager{
		allocations:  make(map[string]*IPAllocation),
		containerIPs: make(map[string]string),
	}
}

// Allocate 为容器分配静态 IP
// ip: 要分配的 IP 地址
// containerID: 容器 ID
// node: 持有该 IP 的节点 ID
// iface: 网络接口名
func (m *IPManager) Allocate(ip, containerID, node, iface string) (*IPAllocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查 IP 是否已被占用
	if alloc, exists := m.allocations[ip]; exists && alloc.Active {
		return nil, fmt.Errorf("IP %s 已被容器 %s 占用", ip, alloc.ContainerID)
	}

	// 检查容器是否已分配过 IP
	if oldIP, exists := m.containerIPs[containerID]; exists {
		// 释放旧 IP 分配
		if oldAlloc, ok := m.allocations[oldIP]; ok {
			oldAlloc.Active = false
		}
	}

	alloc := &IPAllocation{
		IP:          ip,
		ContainerID: containerID,
		Node:        node,
		Interface:   iface,
		AllocatedAt: time.Now(),
		Active:      true,
	}
	m.allocations[ip] = alloc
	m.containerIPs[containerID] = ip

	return alloc, nil
}

// Release 释放容器占用的 IP.
func (m *IPManager) Release(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ip, exists := m.containerIPs[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未分配 IP", containerID)
	}

	if alloc, ok := m.allocations[ip]; ok {
		alloc.Active = false
	}
	delete(m.containerIPs, containerID)

	return nil
}

// Migrate 迁移 IP 到目标节点（模拟 ARP 广播更新）
// 返回新的 IPAllocation 记录.
func (m *IPManager) Migrate(ip, toNode string) (*IPAllocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alloc, exists := m.allocations[ip]
	if !exists {
		return nil, fmt.Errorf("IP %s 未分配", ip)
	}
	if !alloc.Active {
		return nil, fmt.Errorf("IP %s 当前未激活", ip)
	}

	// 更新节点归属
	fromNode := alloc.Node
	alloc.Node = toNode

	// 模拟 ARP 广播：通知网络中其他节点更新 ARP 缓存
	if err := m.arpBroadcast(ip, toNode, alloc.Interface); err != nil {
		// ARP 广播失败不阻断迁移，仅记录警告
		// 实际环境中可能需要重试或告警
		_ = err
	}

	_ = fromNode // fromNode 可用于日志记录
	return alloc, nil
}

// GetAllocation 获取 IP 分配信息.
func (m *IPManager) GetAllocation(ip string) (*IPAllocation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alloc, exists := m.allocations[ip]
	if !exists {
		return nil, fmt.Errorf("IP %s 未分配", ip)
	}
	allocCopy := *alloc
	return &allocCopy, nil
}

// GetContainerIP 获取容器绑定的 IP.
func (m *IPManager) GetContainerIP(containerID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ip, exists := m.containerIPs[containerID]
	if !exists {
		return "", fmt.Errorf("容器 %s 未分配 IP", containerID)
	}
	return ip, nil
}

// ListAllocations 列出所有 IP 分配记录.
func (m *IPManager) ListAllocations() []*IPAllocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*IPAllocation, 0, len(m.allocations))
	for _, alloc := range m.allocations {
		allocCopy := *alloc
		result = append(result, &allocCopy)
	}
	return result
}

// arpBroadcast 模拟 ARP 广播
// 在真实环境中会发送 gratuitous ARP 更新交换机的 MAC 地址表.
func (m *IPManager) arpBroadcast(ip, node, iface string) error {
	// 验证 IP 格式
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("无效的 IP 地址: %s", ip)
	}

	// 模拟 ARP 广播
	// 实际实现可使用 arping 或 Raw socket 发送 gratuitous ARP
	// 此处仅做接口预留
	_ = node
	_ = iface
	return nil
}

// ReleaseAll 释放指定节点上所有容器的 IP（用于节点故障清理）.
func (m *IPManager) ReleaseAll(node string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, alloc := range m.allocations {
		if alloc.Node == node && alloc.Active {
			alloc.Active = false
			delete(m.containerIPs, alloc.ContainerID)
			count++
		}
	}
	return count
}
