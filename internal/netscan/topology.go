package netscan

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// TopologyBuilder 拓扑构建器.
type TopologyBuilder struct {
	devices []Device
	mu      sync.RWMutex
}

// NewTopologyBuilder 创建拓扑构建器.
func NewTopologyBuilder() *TopologyBuilder {
	return &TopologyBuilder{
		devices: make([]Device, 0),
	}
}

// AddDevice 添加设备.
func (tb *TopologyBuilder) AddDevice(device Device) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.devices = append(tb.devices, device)
}

// Build 构建拓扑图.
func (tb *TopologyBuilder) Build() *Topology {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	topo := &Topology{
		Nodes:     make([]TopologyNode, 0, len(tb.devices)),
		Edges:     make([]TopologyEdge, 0),
		UpdatedAt: time.Now(),
	}

	// 构建节点
	for _, device := range tb.devices {
		node := TopologyNode{
			ID:       device.IP,
			IP:       device.IP,
			MAC:      device.MAC,
			Hostname: device.Hostname,
			Type:     tb.guessDeviceType(device),
		}

		for _, svc := range device.Services {
			node.Services = append(node.Services, svc.Name)
		}

		topo.Nodes = append(topo.Nodes, node)
	}

	// 构建边（基于相同网段）
	tb.buildEdges(topo)

	return topo
}

// guessDeviceType 猜测设备类型.
func (tb *TopologyBuilder) guessDeviceType(device Device) string {
	// 基于开放端口猜测设备类型
	portMap := make(map[int]bool)
	for _, p := range device.OpenPorts {
		portMap[p.Number] = true
	}

	// 路由器特征
	if portMap[80] && (portMap[53] || portMap[67]) {
		return "router"
	}

	// NAS 特征
	if portMap[5000] || portMap[8080] || portMap[139] || portMap[445] {
		return "nas"
	}

	// Web 服务器
	if portMap[80] || portMap[443] || portMap[8080] {
		return "server"
	}

	// 默认为主机
	return "host"
}

// buildEdges 构建拓扑边.
func (tb *TopologyBuilder) buildEdges(topo *Topology) {
	nodeMap := make(map[string]*TopologyNode)
	for i := range topo.Nodes {
		nodeMap[topo.Nodes[i].IP] = &topo.Nodes[i]
	}

	// 简单策略：同一网段的设备互相连接
	for i := 0; i < len(topo.Nodes); i++ {
		for j := i + 1; j < len(topo.Nodes); j++ {
			if sameSubnet(topo.Nodes[i].IP, topo.Nodes[j].IP) {
				edge := TopologyEdge{
					Source: topo.Nodes[i].IP,
					Target: topo.Nodes[j].IP,
					Weight: 1,
					Label:  "lan",
				}
				topo.Edges = append(topo.Edges, edge)
			}
		}
	}
}

// sameSubnet 判断两个 IP 是否在同一子网（/24）.
func sameSubnet(ip1, ip2 string) bool {
	a := net.ParseIP(ip1)
	b := net.ParseIP(ip2)
	if a == nil || b == nil {
		return false
	}

	// 简单判断：前 24 位相同
	mask := net.CIDRMask(24, 32)
	return a.Mask(mask).Equal(b.Mask(mask))
}

// DiscoverTopology 发现网络拓扑.
func DiscoverTopology(ctx context.Context, network string) (*Topology, error) {
	// 1. 发现设备
	discoverer := NewDiscoverer(DiscoveryConfig{
		Network:    network,
		Timeout:    3 * time.Second,
		Concurrent: 50,
		UseARP:     true,
		UseICMP:    true,
	})

	devices, err := discoverer.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("设备发现失败：%w", err)
	}

	// 2. 对每个在线设备进行端口扫描
	builder := NewTopologyBuilder()
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, device := range devices {
		if device.State != DeviceStateOnline {
			continue
		}

		wg.Add(1)
		go func(d Device) {
			defer wg.Done()

			// 扫描常用端口
			scanner := NewPortScanner(PortScanConfig{
				Target:     d.IP,
				TopPorts:   20,
				Protocol:   ProtocolTCP,
				Timeout:    2 * time.Second,
				Concurrent: 50,
			})

			ports, err := scanner.Scan(ctx)
			if err == nil {
				d.OpenPorts = ports
			}

			// 识别服务
			for _, port := range ports {
				svc := Service{
					Name:  port.Service,
					Port:  port.Number,
					Proto: string(port.Protocol),
				}
				d.Services = append(d.Services, svc)
			}

			mu.Lock()
			builder.AddDevice(d)
			mu.Unlock()
		}(device)
	}

	wg.Wait()

	return builder.Build(), nil
}

// GetLocalTopology 获取本机网络拓扑.
func GetLocalTopology() (*Topology, error) {
	// 获取本机 IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var localIP string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localIP = ipnet.IP.String()
				break
			}
		}
	}

	if localIP == "" {
		return nil, fmt.Errorf("未找到本机 IP")
	}

	// 计算网段
	ip := net.ParseIP(localIP)
	if ip == nil {
		return nil, fmt.Errorf("无效的 IP 地址")
	}

	mask := net.CIDRMask(24, 32)
	network := ip.Mask(mask).String() + "/24"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return DiscoverTopology(ctx, network)
}
