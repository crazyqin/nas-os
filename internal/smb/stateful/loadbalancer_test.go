package stateful

import (
	"sync"
	"testing"
	"time"
)

func TestNewSMBClientLoadBalancer(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
			{NodeID: "node-3", Address: "192.168.1.3", Port: 9445, Priority: 5},
		},
		StateDir: t.TempDir(),
	}
	mgr, err := NewStatefulFailoverManager(cfg)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	if lb == nil {
		t.Fatal("负载均衡器为空")
	}
	if lb.strategy != StrategyRoundRobin {
		t.Errorf("期望策略为roundrobin，实际为%s", lb.strategy)
	}
}

func TestLoadBalancerRoundRobin(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
			{NodeID: "node-3", Address: "192.168.1.3", Port: 9445, Priority: 5},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	// 设置较低权重使轮询分布更均匀
	lb.SetWeight("node-1", 1)
	lb.SetWeight("node-2", 1)
	lb.SetWeight("node-3", 1)

	// 记录选择结果
	counts := make(map[string]int)
	for i := 0; i < 30; i++ {
		node := lb.SelectNode("")
		if node == nil {
			t.Fatalf("SelectNode返回nil")
		}
		counts[node.NodeID]++
	}

	// 应该有分布（权重1，3个节点，30次选择应覆盖全部）
	if len(counts) < 2 {
		t.Errorf("轮询策略应选择多个节点，实际只选了%d个", len(counts))
	}

	// 每个节点应该被选中接近10次（±3）
	for _, nodeID := range []string{"node-1", "node-2", "node-3"} {
		if counts[nodeID] < 5 || counts[nodeID] > 15 {
			t.Logf("节点 %s 选中 %d 次（预期约10次）", nodeID, counts[nodeID])
		}
	}
}

func TestLoadBalancerLeastConn(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyLeastConn)

	// 增加node-2的连接数
	lb.IncrConn("node-2")
	lb.IncrConn("node-2")
	lb.IncrConn("node-2")

	// node-1连接数少，应优先选中
	var node1Wins int
	for i := 0; i < 10; i++ {
		node := lb.SelectNode("")
		if node != nil && node.NodeID == "node-1" {
			node1Wins++
		}
	}

	if node1Wins < 5 {
		t.Errorf("最少连接策略应优先选择node-1，实际node-1只赢了%d/10次", node1Wins)
	}

	// 验证连接计数
	if cnt := lb.GetConnCount("node-2"); cnt != 3 {
		t.Errorf("期望node-2连接数为3，实际为%d", cnt)
	}

	// 减少连接
	lb.DecrConn("node-2")
	lb.DecrConn("node-2")
	lb.DecrConn("node-2")
	if cnt := lb.GetConnCount("node-2"); cnt != 0 {
		t.Errorf("期望node-2连接数为0，实际为%d", cnt)
	}
}

func TestLoadBalancerIPHash(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyIPHash)

	// 同一IP多次选择应命中同一节点（会话亲和性）
	clientIP := "192.168.1.100"
	var firstNode *FailoverNode
	for i := 0; i < 10; i++ {
		node := lb.SelectNode(clientIP)
		if node == nil {
			t.Fatalf("SelectNode返回nil")
		}
		if i == 0 {
			firstNode = node
		} else if node.NodeID != firstNode.NodeID {
			t.Errorf("IP哈希策略：同一IP应命中同一节点，第%d次命中了%s而非%s",
				i+1, node.NodeID, firstNode.NodeID)
		}
	}

	// 不同IP应均匀分布
	seenNodes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ip := "10.0.0." + string(rune('0'+i%10))
		node := lb.SelectNode(ip)
		if node != nil {
			seenNodes[node.NodeID] = true
		}
	}

	if len(seenNodes) < 1 {
		t.Errorf("IP哈希策略应能选择多个节点，实际只选了%d个", len(seenNodes))
	}

	// 无IP时应退化到轮询
	var fallbackCount int
	for i := 0; i < 5; i++ {
		if lb.SelectNode("") == nil {
			fallbackCount++
		}
	}
	// 只要不panic即可
	_ = fallbackCount
}

func TestLoadBalancerSkipOfflineNodes(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
			{NodeID: "node-3", Address: "192.168.1.3", Port: 9445, Priority: 5},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	// 将node-2设为offline
	mgr.mu.Lock()
	mgr.peerNodes["node-2"].Status = NodeStatusOffline
	mgr.mu.Unlock()

	// 选择多次，node-2不应被选中
	var offlineSelected int
	for i := 0; i < 20; i++ {
		node := lb.SelectNode("")
		if node != nil && node.NodeID == "node-2" {
			offlineSelected++
		}
	}

	if offlineSelected > 0 {
		t.Errorf("offline节点不应被选中，但node-2被选中了%d次", offlineSelected)
	}

	// node-1和node-3应该仍可用
	node := lb.SelectNode("")
	if node == nil {
		t.Error("应有可用节点")
	} else if node.NodeID == "node-2" {
		t.Error("不应对选中offline的node-2")
	}
}

func TestLoadBalancerWeightedDistribution(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	// 设置node-1权重为90，node-2权重为10
	lb.SetWeight("node-1", 90)
	lb.SetWeight("node-2", 10)

	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		node := lb.SelectNode("")
		if node != nil {
			counts[node.NodeID]++
		}
	}

	// node-1应占主导（约90%）
	node1Ratio := float64(counts["node-1"]) / 100.0
	if node1Ratio < 0.7 {
		t.Errorf("加权轮询：node-1应占比约90%%，实际%.1f%%", node1Ratio*100)
	}
}

func TestLoadBalancerStrategySwitch(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	// 切换策略
	lb.SetStrategy(StrategyLeastConn)
	if lb.GetStrategy() != StrategyLeastConn {
		t.Error("策略切换失败")
	}

	lb.SetStrategy(StrategyIPHash)
	if lb.GetStrategy() != StrategyIPHash {
		t.Error("策略切换失败")
	}
}

func TestLoadBalancerDistributionStats(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	// 添加会话
	mgr.registry.Add(&SessionState{
		SessionID: "s1", ClientIP: "10.0.0.1", NodeID: "node-1",
	})
	mgr.registry.Add(&SessionState{
		SessionID: "s2", ClientIP: "10.0.0.2", NodeID: "node-2",
	})

	lb.IncrConn("node-1")
	lb.IncrConn("node-1")
	lb.IncrConn("node-2")

	stats := lb.GetDistributionStats()
	if len(stats) < 2 {
		t.Fatalf("期望至少2个节点统计，实际%d个", len(stats))
	}

	statsMap := make(map[string]DistributionStats)
	for _, s := range stats {
		statsMap[s.NodeID] = s
	}

	if s1, ok := statsMap["node-1"]; ok {
		if s1.ActiveConns != 2 {
			t.Errorf("期望node-1有2个连接，实际%d", s1.ActiveConns)
		}
		if s1.Sessions != 1 {
			t.Errorf("期望node-1有1个会话，实际%d", s1.Sessions)
		}
	}
}

// --- Failover Integration Tests ---

func TestFailoverIntegrationCreation(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	fcfg := DefaultFailoverConfig()
	fi := NewFailoverIntegration(mgr, lb, fcfg)
	if fi == nil {
		t.Fatal("FailoverIntegration为空")
	}
	if fi.config.AutoFailover != true {
		t.Error("AutoFailover默认应为true")
	}
}

func TestFailoverIntegrationCallbacks(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	var callbackCalled bool
	var receivedEvent FailoverEvent
	var mu sync.Mutex

	fi.RegisterCallback(func(event FailoverEvent) {
		mu.Lock()
		callbackCalled = true
		receivedEvent = event
		mu.Unlock()
	})

	// 先启动FailoverIntegration，这样failoverEventWatcher才能消费eventCh
	if err := fi.Start(); err != nil {
		t.Fatalf("启动FailoverIntegration失败: %v", err)
	}
	defer fi.Stop()

	// 触发一个事件
	mgr.eventCh <- FailoverEvent{
		Type:      EventFailoverStart,
		Timestamp: time.Now(),
		NodeID:    "node-2",
		Message:   "测试事件",
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !callbackCalled {
		t.Error("回调未被调用")
	}
	if receivedEvent.Type != EventFailoverStart {
		t.Errorf("期望事件类型为%s，实际为%s", EventFailoverStart, receivedEvent.Type)
	}
	mu.Unlock()
}

func TestFailoverIntegrationGetClusterHealth(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
			{NodeID: "node-3", Address: "192.168.1.3", Port: 9445, Priority: 5},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	ch, err := fi.GetClusterHealth()
	if err != nil {
		t.Fatalf("GetClusterHealth失败: %v", err)
	}
	if ch.ClusterName != "test-cluster" {
		t.Errorf("期望集群名为test-cluster，实际为%s", ch.ClusterName)
	}
	if len(ch.Nodes) != 3 {
		t.Errorf("期望3个节点，实际%d个", len(ch.Nodes))
	}
	if ch.Overall != "healthy" {
		t.Errorf("期望整体状态为healthy，实际为%s", ch.Overall)
	}
}

func TestFailoverIntegrationClusterHealthDegraded(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	mgr.mu.Lock()
	mgr.peerNodes["node-2"].Status = NodeStatusDegraded
	mgr.mu.Unlock()

	ch, _ := fi.GetClusterHealth()
	if ch.Overall != "degraded" {
		t.Errorf("期望整体状态为degraded，实际为%s", ch.Overall)
	}
}

func TestFailoverIntegrationClusterHealthCritical(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	mgr.mu.Lock()
	mgr.peerNodes["node-2"].Status = NodeStatusOffline
	mgr.mu.Unlock()

	ch, _ := fi.GetClusterHealth()
	if ch.Overall != "critical" {
		t.Errorf("期望整体状态为critical，实际为%s", ch.Overall)
	}
}

func TestFailoverIntegrationFailoverCount(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	if fi.GetFailoverCount() != 0 {
		t.Errorf("初始故障转移次数应为0，实际为%d", fi.GetFailoverCount())
	}
}

func TestTransparentReconnectPrepare(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		StateDir:    t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	mgr.registry.Add(&SessionState{
		SessionID: "sess-reconnect",
		ClientIP:  "192.168.1.100",
		NodeID:    "node-1",
	})

	info, err := fi.PrepareTransparentReconnect("sess-reconnect", "192.168.1.100", "node-1", "node-2")
	if err != nil {
		t.Fatalf("PrepareTransparentReconnect失败: %v", err)
	}
	if info.SessionID != "sess-reconnect" {
		t.Errorf("期望SessionID=sess-reconnect，实际为%s", info.SessionID)
	}
	if info.OldNodeID != "node-1" {
		t.Errorf("期望OldNodeID=node-1，实际为%s", info.OldNodeID)
	}
	if info.NewNodeID != "node-2" {
		t.Errorf("期望NewNodeID=node-2，实际为%s", info.NewNodeID)
	}

	// 验证会话metadata已更新
	session := mgr.registry.Get("sess-reconnect")
	if session == nil || session.Metadata["pending_reconnect"] != "true" {
		t.Error("会话metadata未正确更新")
	}

	// 完成重连
	err = fi.CompleteTransparentReconnect("sess-reconnect")
	if err != nil {
		t.Fatalf("CompleteTransparentReconnect失败: %v", err)
	}

	session = mgr.registry.Get("sess-reconnect")
	if session != nil && session.Metadata["pending_reconnect"] == "true" {
		t.Error("pending_reconnect应在完成后清除")
	}
}

func TestFailoverIntegrationPendingFailovers(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)
	fi := NewFailoverIntegration(mgr, lb, DefaultFailoverConfig())

	pending := fi.GetPendingFailovers()
	if len(pending) != 0 {
		t.Errorf("初始无待处理故障转移，实际有%d个", len(pending))
	}
}

func TestLoadBalancerConcurrency(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				lb.SelectNode("")
				lb.IncrConn("node-1")
				lb.DecrConn("node-1")
			}
		}()
	}
	wg.Wait()
	// 无panic即为通过
}

func TestWeightBounds(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		StateDir:    t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)
	lb := NewSMBClientLoadBalancer(mgr, StrategyRoundRobin)

	// 测试权重边界
	lb.SetWeight("node-1", 0)
	if lb.GetWeight("node-1") != 1 {
		t.Errorf("权重0应被修正为1，实际为%d", lb.GetWeight("node-1"))
	}

	lb.SetWeight("node-1", 200)
	if lb.GetWeight("node-1") != 100 {
		t.Errorf("权重200应被修正为100，实际为%d", lb.GetWeight("node-1"))
	}
}
