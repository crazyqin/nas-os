package storagefabric2

import (
	"testing"
	"time"
)

// TestNewFabricManager 测试创建Fabric管理器
func TestNewFabricManager(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	if fm == nil {
		t.Fatal("NewFabricManager 返回 nil")
	}
	if fm.Policy() != MultipathRoundRobin {
		t.Fatalf("策略期望 RoundRobin，实际 %s", fm.Policy())
	}
}

// TestAddAndGetNode 测试添加和获取节点
func TestAddAndGetNode(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	node := FabricNode{
		ID:        "node-1",
		Name:      "storage-01",
		Type:      NodeTypeTarget,
		IPAddress: "10.0.0.1",
		WWN:       "20:00:00:25:b5:01:00:01",
		PortCount: 4,
		MetaData:  map[string]string{"rack": "A1"},
	}

	if err := fm.AddNode(node); err != nil {
		t.Fatalf("AddNode 失败: %v", err)
	}

	got, ok := fm.GetNode("node-1")
	if !ok {
		t.Fatal("GetNode 未找到已添加的节点")
	}
	if got.Name != "storage-01" {
		t.Fatalf("节点名称期望 storage-01，实际 %s", got.Name)
	}
	if got.Type != NodeTypeTarget {
		t.Fatalf("节点类型期望 Target，实际 %s", got.Type)
	}
}

// TestAddNodeDuplicate 测试重复添加节点报错
func TestAddNodeDuplicate(t *testing.T) {
	fm := NewFabricManager(MultipathActivePassive)
	node := FabricNode{ID: "dup-1", Name: "test", Type: NodeTypeInitiator}
	if err := fm.AddNode(node); err != nil {
		t.Fatalf("首次 AddNode 不应报错: %v", err)
	}
	if err := fm.AddNode(node); err == nil {
		t.Fatal("重复添加节点应返回错误")
	}
}

// TestAddNodeEmptyID 测试空ID节点报错
func TestAddNodeEmptyID(t *testing.T) {
	fm := NewFabricManager(MultipathLeastIO)
	node := FabricNode{ID: "", Name: "empty"}
	if err := fm.AddNode(node); err == nil {
		t.Fatal("空ID节点应返回错误")
	}
}

// TestAddLinkAndStateUpdate 测试添加链路和状态更新
func TestAddLinkAndStateUpdate(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	// 添加两个节点
	fm.AddNode(FabricNode{ID: "src", Name: "src", Type: NodeTypeInitiator})
	fm.AddNode(FabricNode{ID: "dst", Name: "dst", Type: NodeTypeTarget})

	link := FabricLink{
		ID:        "link-1",
		SrcNodeID: "src",
		DstNodeID: "dst",
		Protocol:  ProtocolFC,
		State:     LinkStateUp,
		Bandwidth: 10_000_000_000, // 10Gbps
	}
	if err := fm.AddLink(link); err != nil {
		t.Fatalf("AddLink 失败: %v", err)
	}

	got, ok := fm.GetLink("link-1")
	if !ok {
		t.Fatal("GetLink 未找到已添加的链路")
	}
	if got.State != LinkStateUp {
		t.Fatalf("初始状态应为 Up，实际 %s", got.State)
	}

	// 更新为降级
	if err := fm.UpdateLinkState("link-1", LinkStateDegraded); err != nil {
		t.Fatalf("UpdateLinkState 失败: %v", err)
	}
	got, _ = fm.GetLink("link-1")
	if got.State != LinkStateDegraded {
		t.Fatalf("更新后期望 Degraded，实际 %s", got.State)
	}
}

// TestAddLinkMissingNode 测试链路引用不存在的节点报错
func TestAddLinkMissingNode(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	link := FabricLink{ID: "bad", SrcNodeID: "no-such", DstNodeID: "also-no", Protocol: ProtocolISCSI, State: LinkStateDown}
	if err := fm.AddLink(link); err == nil {
		t.Fatal("引用不存在节点的链路应返回错误")
	}
}

// TestRemoveNodeCascades 测试删除节点级联删除链路
func TestRemoveNodeCascades(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	fm.AddNode(FabricNode{ID: "a", Name: "a", Type: NodeTypeInitiator})
	fm.AddNode(FabricNode{ID: "b", Name: "b", Type: NodeTypeTarget})
	fm.AddLink(FabricLink{ID: "l1", SrcNodeID: "a", DstNodeID: "b", Protocol: ProtocolRDMA, State: LinkStateUp, Bandwidth: 25_000_000_000})

	if err := fm.RemoveNode("a"); err != nil {
		t.Fatalf("RemoveNode 失败: %v", err)
	}
	if _, ok := fm.GetNode("a"); ok {
		t.Fatal("节点应已被删除")
	}
	if _, ok := fm.GetLink("l1"); ok {
		t.Fatal("关联链路应被级联删除")
	}
}

// TestZoneManagement 测试分区管理
func TestZoneManagement(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	fm.AddNode(FabricNode{ID: "z1", Name: "z1", Type: NodeTypeTarget})
	fm.AddNode(FabricNode{ID: "z2", Name: "z2", Type: NodeTypeTarget})

	zone := FabricZone{
		ID:      "zone-1",
		Name:    "prod-zone",
		NodeIDs: []string{"z1", "z2"},
	}
	if err := fm.AddZone(zone); err != nil {
		t.Fatalf("AddZone 失败: %v", err)
	}

	topo := fm.Topology()
	if len(topo.Zones) != 1 {
		t.Fatalf("拓扑中应有1个分区，实际 %d", len(topo.Zones))
	}

	// 删除分区
	if err := fm.RemoveZone("zone-1"); err != nil {
		t.Fatalf("RemoveZone 失败: %v", err)
	}
	topo = fm.Topology()
	if len(topo.Zones) != 0 {
		t.Fatalf("删除后应无分区，实际 %d", len(topo.Zones))
	}
}

// TestAddZoneWithMissingNode 测试分区引用不存在节点报错
func TestAddZoneWithMissingNode(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	zone := FabricZone{ID: "z", Name: "bad", NodeIDs: []string{"ghost"}}
	if err := fm.AddZone(zone); err == nil {
		t.Fatal("引用不存在节点的分区应返回错误")
	}
}

// TestMultipath 测试多路径设置
func TestMultipath(t *testing.T) {
	fm := NewFabricManager(MultipathLeastIO)
	fm.AddNode(FabricNode{ID: "h1", Name: "host", Type: NodeTypeInitiator})
	fm.AddNode(FabricNode{ID: "s1", Name: "storage", Type: NodeTypeTarget})
	fm.AddLink(FabricLink{ID: "lp1", SrcNodeID: "h1", DstNodeID: "s1", Protocol: ProtocolISCSI, State: LinkStateUp, Bandwidth: 1_000_000_000})
	fm.AddLink(FabricLink{ID: "lp2", SrcNodeID: "h1", DstNodeID: "s1", Protocol: ProtocolISCSI, State: LinkStateUp, Bandwidth: 1_000_000_000})

	if err := fm.SetMultipath("s1", []string{"lp1", "lp2"}); err != nil {
		t.Fatalf("SetMultipath 失败: %v", err)
	}

	paths := fm.GetMultipath("s1")
	if len(paths) != 2 {
		t.Fatalf("多路径应有2条，实际 %d", len(paths))
	}
}

// TestLatencyMonitor 测试延迟监控器
func TestLatencyMonitor(t *testing.T) {
	m := NewLatencyMonitor(10)
	m.Record("link-x", 1.5)
	m.Record("link-x", 2.5)
	m.Record("link-x", 3.0)

	avg := m.Average("link-x")
	expected := (1.5 + 2.5 + 3.0) / 3.0
	if avg != expected {
		t.Fatalf("平均延迟期望 %.2f，实际 %.2f", expected, avg)
	}

	maxLat := m.Max("link-x")
	if maxLat != 3.0 {
		t.Fatalf("最大延迟期望 3.0，实际 %.2f", maxLat)
	}

	// 不存在的链路返回0
	if m.Average("no-such") != 0 {
		t.Fatal("不存在的链路平均延迟应为0")
	}
}

// TestBandwidthAggregator 测试带宽聚合
func TestBandwidthAggregator(t *testing.T) {
	agg := NewBandwidthAggregator()
	agg.Add("a", 10_000_000_000)
	agg.Add("b", 25_000_000_000)

	if agg.Count() != 2 {
		t.Fatalf("链路数期望 2，实际 %d", agg.Count())
	}
	total := agg.Total()
	if total != 35_000_000_000 {
		t.Fatalf("总带宽期望 35Gbps，实际 %d", total)
	}

	agg.Remove("a")
	if agg.Count() != 1 {
		t.Fatalf("移除后期望 1，实际 %d", agg.Count())
	}
}

// TestAutoDiscovery 测试自动发现引擎
func TestAutoDiscovery(t *testing.T) {
	d := NewAutoDiscovery(DiscoveryISNS)
	if d.IsEnabled() {
		t.Fatal("初始状态应为禁用")
	}
	if d.Protocol() != DiscoveryISNS {
		t.Fatalf("协议期望 iSNS，实际 %s", d.Protocol())
	}

	d.Enable()
	if !d.IsEnabled() {
		t.Fatal("Enable 后应为启用")
	}

	node := FabricNode{ID: "disc-1", Name: "found", Type: NodeTypeTarget, IPAddress: "192.168.1.10"}
	d.RegisterNode(node)

	nodes := d.DiscoveredNodes()
	if len(nodes) != 1 {
		t.Fatalf("发现节点数期望 1，实际 %d", len(nodes))
	}

	got, ok := d.GetNode("disc-1")
	if !ok {
		t.Fatal("GetNode 未找到发现的节点")
	}
	if got.LastSeenAt.IsZero() {
		t.Fatal("LastSeenAt 应被自动设置")
	}

	d.Disable()
	if d.IsEnabled() {
		t.Fatal("Disable 后应为禁用")
	}
}

// TestComputeHealthScore 测试健康评分
func TestComputeHealthScore(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	fm.AddNode(FabricNode{ID: "hc-src", Name: "s", Type: NodeTypeInitiator})
	fm.AddNode(FabricNode{ID: "hc-dst", Name: "d", Type: NodeTypeTarget})
	fm.AddLink(FabricLink{
		ID:        "hc-link",
		SrcNodeID: "hc-src",
		DstNodeID: "hc-dst",
		Protocol:  ProtocolFC,
		State:     LinkStateUp,
		Bandwidth: 10_000_000_000,
		ErrCount:  2,
	})

	// 正常链路+2个错误 => 100 - 10 = 90
	score, err := fm.ComputeHealthScore("hc-link")
	if err != nil {
		t.Fatalf("ComputeHealthScore 失败: %v", err)
	}
	if score.Score != 90 {
		t.Fatalf("健康评分期望 90，实际 %d", score.Score)
	}

	// 断开链路 => 0分
	fm.UpdateLinkState("hc-link", LinkStateDown)
	score, _ = fm.ComputeHealthScore("hc-link")
	if score.Score != 0 {
		t.Fatalf("断开链路评分期望 0，实际 %d", score.Score)
	}

	// 不存在的链路
	_, err = fm.ComputeHealthScore("no-such")
	if err == nil {
		t.Fatal("不存在的链路应返回错误")
	}
}

// TestTopology 测试拓扑获取
func TestTopology(t *testing.T) {
	fm := NewFabricManager(MultipathRoundRobin)
	fm.AddNode(FabricNode{ID: "t1", Name: "n1", Type: NodeTypeTarget})
	fm.AddNode(FabricNode{ID: "t2", Name: "n2", Type: NodeTypeInitiator})
	fm.AddLink(FabricLink{ID: "tl1", SrcNodeID: "t1", DstNodeID: "t2", Protocol: ProtocolNVMeOF, State: LinkStateUp, Bandwidth: 100_000_000_000})
	fm.AddZone(FabricZone{ID: "tz1", Name: "z", NodeIDs: []string{"t1", "t2"}, CreatedAt: time.Now()})

	topo := fm.Topology()
	if len(topo.Nodes) != 2 {
		t.Fatalf("拓扑节点期望 2，实际 %d", len(topo.Nodes))
	}
	if len(topo.Links) != 1 {
		t.Fatalf("拓扑链路期望 1，实际 %d", len(topo.Links))
	}
	if len(topo.Zones) != 1 {
		t.Fatalf("拓扑分区期望 1，实际 %d", len(topo.Zones))
	}
}

// TestFabricManagerWithDiscovery 测试管理器集成自动发现
func TestFabricManagerWithDiscovery(t *testing.T) {
	fm := NewFabricManager(MultipathActivePassive)
	d := NewAutoDiscovery(DiscoverySLP)
	d.Enable()
	fm.SetDiscovery(d)

	disc := fm.Discovery()
	if disc == nil {
		t.Fatal("Discovery 不应为 nil")
	}
	if disc.Protocol() != DiscoverySLP {
		t.Fatalf("协议期望 SLP，实际 %s", disc.Protocol())
	}
}

// TestLatencyMonitorHistoryLimit 测试延迟监控历史限制
func TestLatencyMonitorHistoryLimit(t *testing.T) {
	m := NewLatencyMonitor(3)
	m.Record("lim", 1.0)
	m.Record("lim", 2.0)
	m.Record("lim", 3.0)
	m.Record("lim", 4.0) // 应淘汰 1.0

	if m.Average("lim") != (2.0+3.0+4.0)/3.0 {
		t.Fatalf("超出限制后平均值计算错误，实际 %.2f", m.Average("lim"))
	}
	if m.Max("lim") != 4.0 {
		t.Fatalf("最大值期望 4.0，实际 %.2f", m.Max("lim"))
	}
}
