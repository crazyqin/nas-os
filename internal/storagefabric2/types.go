// Package storagefabric2 提供存储网络Fabric管理功能
// 管理iSCSI/FC/NVMe-oF存储网络拓扑，支持自动发现、链路监控、多路径管理、带宽聚合
package storagefabric2

import (
	"fmt"
	"sync"
	"time"
)

// LinkProtocol 链路协议类型
type LinkProtocol string

const (
	ProtocolFC      LinkProtocol = "FC"       // 光纤通道
	ProtocolISCSI   LinkProtocol = "iSCSI"    // 互联网小型计算机系统接口
	ProtocolNVMeOF  LinkProtocol = "NVMe-oF"  // NVMe over Fabrics
	ProtocolRDMA    LinkProtocol = "RDMA"      // 远程直接内存访问
)

// LinkState 链路状态
type LinkState string

const (
	LinkStateUp       LinkState = "Up"       // 正常运行
	LinkStateDown     LinkState = "Down"     // 断开
	LinkStateDegraded LinkState = "Degraded" // 性能降级
)

// NodeType 节点类型
type NodeType string

const (
	NodeTypeTarget    NodeType = "Target"    // 目标端（存储设备）
	NodeTypeInitiator NodeType = "Initiator" // 发起端（主机）
)

// MultipathPolicy 多路径策略
type MultipathPolicy string

const (
	MultipathRoundRobin     MultipathPolicy = "RoundRobin"     // 轮询
	MultipathActivePassive  MultipathPolicy = "ActivePassive"  // 主备
	MultipathLeastIO        MultipathPolicy = "LeastIO"        // 最少IO
)

// DiscoveryProtocol 自动发现协议
type DiscoveryProtocol string

const (
	DiscoverySLP   DiscoveryProtocol = "SLP"      // 服务定位协议
	DiscoveryISNS  DiscoveryProtocol = "iSNS"     // 互联网存储名称服务
	DiscoveryFCNS  DiscoveryProtocol = "FCNS"     // FC名称服务
)

// FabricNode 存储Fabric节点
type FabricNode struct {
	ID          string            // 节点唯一标识
	Name        string            // 节点名称
	Type        NodeType          // 节点类型（Target/Initiator）
	IPAddress   string            // IP地址
	WWN         string            // 全球名称（WWN）
	PortCount   int               // 端口数量
	MetaData    map[string]string // 扩展元数据
	LastSeenAt  time.Time         // 最后发现时间
}

// FabricLink 存储Fabric链路
type FabricLink struct {
	ID           string        // 链路唯一标识
	SrcNodeID    string        // 源节点ID
	DstNodeID    string        // 目标节点ID
	Protocol     LinkProtocol  // 协议类型
	State        LinkState     // 链路状态
	Bandwidth    int64         // 带宽（bps）
	LatencyMs    float64       // 延迟（毫秒）
	ThroughputIO  int64        // 当前IOPS
	ErrCount     int64         // 错误计数
	UpSince      time.Time     // 上线时间
}

// FabricZone 分区（Zone）
type FabricZone struct {
	ID        string    // 分区唯一标识
	Name      string    // 分区名称
	NodeIDs   []string  // 包含的节点ID列表
	CreatedAt time.Time // 创建时间
}

// FabricTopology 拓扑可视化数据
type FabricTopology struct {
	Nodes []FabricNode // 所有节点
	Links []FabricLink // 所有链路
	Zones []FabricZone // 所有分区
}

// HealthScore 链路健康评分
type HealthScore struct {
	LinkID    string    // 链路ID
	Score     int       // 健康评分（0-100）
	Reason    string    // 评分依据
	ScoredAt  time.Time // 评分时间
}

// LatencyMonitor 延迟监控器
type LatencyMonitor struct {
	mu          sync.RWMutex
	history     map[string][]float64 // linkID -> 延迟历史
	maxHistory  int                  // 最大历史记录数
}

// NewLatencyMonitor 创建延迟监控器
// maxHistory: 每条链路最多保留的历史记录数
func NewLatencyMonitor(maxHistory int) *LatencyMonitor {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &LatencyMonitor{
		history:    make(map[string][]float64),
		maxHistory: maxHistory,
	}
}

// Record 记录一条延迟数据
func (m *LatencyMonitor) Record(linkID string, latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := m.history[linkID]
	if len(h) >= m.maxHistory {
		h = h[1:]
	}
	m.history[linkID] = append(h, latencyMs)
}

// Average 返回链路平均延迟
func (m *LatencyMonitor) Average(linkID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.history[linkID]
	if len(h) == 0 {
		return 0
	}
	var sum float64
	for _, v := range h {
		sum += v
	}
	return sum / float64(len(h))
}

// Max 返回链路最大延迟
func (m *LatencyMonitor) Max(linkID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.history[linkID]
	if len(h) == 0 {
		return 0
	}
	max := h[0]
	for _, v := range h[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// BandwidthAggregator 带宽聚合器
type BandwidthAggregator struct {
	mu       sync.RWMutex
	links    map[string]int64 // linkID -> 带宽bps
}

// NewBandwidthAggregator 创建带宽聚合器
func NewBandwidthAggregator() *BandwidthAggregator {
	return &BandwidthAggregator{
		links: make(map[string]int64),
	}
}

// Add 添加链路带宽
func (a *BandwidthAggregator) Add(linkID string, bandwidth int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.links[linkID] = bandwidth
}

// Remove 移除链路
func (a *BandwidthAggregator) Remove(linkID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.links, linkID)
}

// Total 返回聚合总带宽
func (a *BandwidthAggregator) Total() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var total int64
	for _, bw := range a.links {
		total += bw
	}
	return total
}

// Count 返回聚合链路数
func (a *BandwidthAggregator) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.links)
}

// AutoDiscovery 自动发现引擎
type AutoDiscovery struct {
	mu        sync.RWMutex
	protocol  DiscoveryProtocol
	nodes     map[string]FabricNode // 发现到的节点
	enabled   bool
}

// NewAutoDiscovery 创建自动发现引擎
func NewAutoDiscovery(protocol DiscoveryProtocol) *AutoDiscovery {
	return &AutoDiscovery{
		protocol: protocol,
		nodes:    make(map[string]FabricNode),
	}
}

// Enable 启用自动发现
func (d *AutoDiscovery) Enable() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = true
}

// Disable 禁用自动发现
func (d *AutoDiscovery) Disable() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = false
}

// IsEnabled 是否已启用
func (d *AutoDiscovery) IsEnabled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.enabled
}

// Protocol 返回发现协议
func (d *AutoDiscovery) Protocol() DiscoveryProtocol {
	return d.protocol
}

// RegisterNode 注册发现的节点
func (d *AutoDiscovery) RegisterNode(node FabricNode) {
	d.mu.Lock()
	defer d.mu.Unlock()
	node.LastSeenAt = time.Now()
	d.nodes[node.ID] = node
}

// DiscoveredNodes 返回所有发现的节点
func (d *AutoDiscovery) DiscoveredNodes() []FabricNode {
	d.mu.RLock()
	defer d.mu.RUnlock()
	nodes := make([]FabricNode, 0, len(d.nodes))
	for _, n := range d.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetNode 按ID获取节点
func (d *AutoDiscovery) GetNode(id string) (FabricNode, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n, ok := d.nodes[id]
	return n, ok
}

// FabricManager 存储Fabric管理器
type FabricManager struct {
	mu          sync.RWMutex
	nodes       map[string]FabricNode
	links       map[string]FabricLink
	zones       map[string]FabricZone
	multipaths  map[string][]string // targetID -> []linkID（多路径链路组）
	policy      MultipathPolicy
	latencyMon  *LatencyMonitor
	bandwidthAgg *BandwidthAggregator
	discovery   *AutoDiscovery
}

// NewFabricManager 创建存储Fabric管理器
func NewFabricManager(policy MultipathPolicy) *FabricManager {
	return &FabricManager{
		nodes:        make(map[string]FabricNode),
		links:        make(map[string]FabricLink),
		zones:        make(map[string]FabricZone),
		multipaths:   make(map[string][]string),
		policy:       policy,
		latencyMon:   NewLatencyMonitor(100),
		bandwidthAgg: NewBandwidthAggregator(),
	}
}

// AddNode 添加节点
func (fm *FabricManager) AddNode(node FabricNode) error {
	if node.ID == "" {
		return fmt.Errorf("节点ID不能为空")
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if _, exists := fm.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在", node.ID)
	}
	fm.nodes[node.ID] = node
	return nil
}

// RemoveNode 移除节点及其关联链路
func (fm *FabricManager) RemoveNode(nodeID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if _, exists := fm.nodes[nodeID]; !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	// 删除关联链路
	for id, link := range fm.links {
		if link.SrcNodeID == nodeID || link.DstNodeID == nodeID {
			delete(fm.links, id)
			fm.bandwidthAgg.Remove(id)
		}
	}
	// 从分区中移除
	for _, zone := range fm.zones {
		for i, nid := range zone.NodeIDs {
			if nid == nodeID {
				zone.NodeIDs = append(zone.NodeIDs[:i], zone.NodeIDs[i+1:]...)
				break
			}
		}
	}
	delete(fm.nodes, nodeID)
	return nil
}

// GetNode 获取节点
func (fm *FabricManager) GetNode(nodeID string) (FabricNode, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	n, ok := fm.nodes[nodeID]
	return n, ok
}

// AddLink 添加链路
func (fm *FabricManager) AddLink(link FabricLink) error {
	if link.ID == "" {
		return fmt.Errorf("链路ID不能为空")
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	// 检查源节点和目标节点是否存在
	if _, ok := fm.nodes[link.SrcNodeID]; !ok {
		return fmt.Errorf("源节点 %s 不存在", link.SrcNodeID)
	}
	if _, ok := fm.nodes[link.DstNodeID]; !ok {
		return fmt.Errorf("目标节点 %s 不存在", link.DstNodeID)
	}
	if _, exists := fm.links[link.ID]; exists {
		return fmt.Errorf("链路 %s 已存在", link.ID)
	}
	link.UpSince = time.Now()
	fm.links[link.ID] = link
	fm.bandwidthAgg.Add(link.ID, link.Bandwidth)
	return nil
}

// RemoveLink 移除链路
func (fm *FabricManager) RemoveLink(linkID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if _, exists := fm.links[linkID]; !exists {
		return fmt.Errorf("链路 %s 不存在", linkID)
	}
	delete(fm.links, linkID)
	fm.bandwidthAgg.Remove(linkID)
	return nil
}

// GetLink 获取链路
func (fm *FabricManager) GetLink(linkID string) (FabricLink, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	l, ok := fm.links[linkID]
	return l, ok
}

// UpdateLinkState 更新链路状态
func (fm *FabricManager) UpdateLinkState(linkID string, state LinkState) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	link, ok := fm.links[linkID]
	if !ok {
		return fmt.Errorf("链路 %s 不存在", linkID)
	}
	link.State = state
	fm.links[linkID] = link
	return nil
}

// AddZone 添加分区
func (fm *FabricManager) AddZone(zone FabricZone) error {
	if zone.ID == "" {
		return fmt.Errorf("分区ID不能为空")
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if _, exists := fm.zones[zone.ID]; exists {
		return fmt.Errorf("分区 %s 已存在", zone.ID)
	}
	// 校验节点存在性
	for _, nid := range zone.NodeIDs {
		if _, ok := fm.nodes[nid]; !ok {
			return fmt.Errorf("节点 %s 不存在", nid)
		}
	}
	zone.CreatedAt = time.Now()
	fm.zones[zone.ID] = zone
	return nil
}

// RemoveZone 移除分区
func (fm *FabricManager) RemoveZone(zoneID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if _, exists := fm.zones[zoneID]; !exists {
		return fmt.Errorf("分区 %s 不存在", zoneID)
	}
	delete(fm.zones, zoneID)
	return nil
}

// Topology 获取当前拓扑
func (fm *FabricManager) Topology() FabricTopology {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	topo := FabricTopology{}
	for _, n := range fm.nodes {
		topo.Nodes = append(topo.Nodes, n)
	}
	for _, l := range fm.links {
		topo.Links = append(topo.Links, l)
	}
	for _, z := range fm.zones {
		topo.Zones = append(topo.Zones, z)
	}
	return topo
}

// SetMultipath 设置多路径策略
func (fm *FabricManager) SetMultipath(targetNodeID string, linkIDs []string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if _, ok := fm.nodes[targetNodeID]; !ok {
		return fmt.Errorf("目标节点 %s 不存在", targetNodeID)
	}
	for _, lid := range linkIDs {
		if _, ok := fm.links[lid]; !ok {
			return fmt.Errorf("链路 %s 不存在", lid)
		}
	}
	fm.multipaths[targetNodeID] = linkIDs
	return nil
}

// GetMultipath 获取目标节点的多路径链路组
func (fm *FabricManager) GetMultipath(targetNodeID string) []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.multipaths[targetNodeID]
}

// Policy 返回当前多路径策略
func (fm *FabricManager) Policy() MultipathPolicy {
	return fm.policy
}

// LatencyMonitor 返回延迟监控器
func (fm *FabricManager) LatencyMonitor() *LatencyMonitor {
	return fm.latencyMon
}

// BandwidthAggregator 返回带宽聚合器
func (fm *FabricManager) BandwidthAggregator() *BandwidthAggregator {
	return fm.bandwidthAgg
}

// SetDiscovery 设置自动发现引擎
func (fm *FabricManager) SetDiscovery(d *AutoDiscovery) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.discovery = d
}

// Discovery 返回自动发现引擎
func (fm *FabricManager) Discovery() *AutoDiscovery {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.discovery
}

// ComputeHealthScore 计算链路健康评分
func (fm *FabricManager) ComputeHealthScore(linkID string) (HealthScore, error) {
	fm.mu.RLock()
	link, ok := fm.links[linkID]
	fm.mu.RUnlock()
	if !ok {
		return HealthScore{}, fmt.Errorf("链路 %s 不存在", linkID)
	}

	score := 100
	reason := "正常"

	// 链路断开直接0分
	if link.State == LinkStateDown {
		return HealthScore{
			LinkID:   linkID,
			Score:    0,
			Reason:   "链路断开",
			ScoredAt: time.Now(),
		}, nil
	}

	// 降级扣分
	if link.State == LinkStateDegraded {
		score -= 30
		reason = "链路降级"
	}

	// 错误计数扣分
	if link.ErrCount > 0 {
		deduction := int(link.ErrCount) * 5
		if deduction > 40 {
			deduction = 40
		}
		score -= deduction
		if reason == "正常" {
			reason = fmt.Sprintf("存在 %d 个错误", link.ErrCount)
		}
	}

	// 延迟扣分
	avgLatency := fm.latencyMon.Average(linkID)
	if avgLatency > 10 {
		score -= 20
		reason += fmt.Sprintf("; 平均延迟 %.1fms 偏高", avgLatency)
	} else if avgLatency > 5 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}

	return HealthScore{
		LinkID:   linkID,
		Score:    score,
		Reason:   reason,
		ScoredAt: time.Now(),
	}, nil
}
