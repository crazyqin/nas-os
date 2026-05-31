package wanreplanner

import (
	"testing"
	"time"
)

// ============================================================
// 辅助函数
// ============================================================

func newTestPlanner(strategy LoadBalanceStrategy) *WANPlanner {
	cfg := DefaultPlannerConfig()
	cfg.Strategy = strategy
	cfg.HealthCheckInterval = 100 * time.Millisecond
	return NewWANPlanner(cfg)
}

func addTestLinks(p *WANPlanner) {
	p.AddLink(&WANLink{ID: "wan1", Name: "电信", Gateway: "10.0.1.1", Weight: 3, Bandwidth: 100_000_000, Status: LinkStatusUp, Score: 90})
	p.AddLink(&WANLink{ID: "wan2", Name: "联通", Gateway: "10.0.2.1", Weight: 2, Bandwidth: 50_000_000, Status: LinkStatusUp, Score: 75})
	p.AddLink(&WANLink{ID: "wan3", Name: "移动", Gateway: "10.0.3.1", Weight: 1, Bandwidth: 30_000_000, Status: LinkStatusUp, Score: 60})
}

// ============================================================
// 基础功能测试
// ============================================================

func TestNewWANPlanner(t *testing.T) {
	cfg := DefaultPlannerConfig()
	p := NewWANPlanner(cfg)
	if p == nil {
		t.Fatal("NewWANPlanner returned nil")
	}
	if p.IsRunning() {
		t.Error("planner should not be running after creation")
	}
}

func TestStartStop(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !p.IsRunning() {
		t.Error("planner should be running after Start")
	}
	// 重复 Start 不应报错
	if err := p.Start(); err != nil {
		t.Fatalf("double Start failed: %v", err)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if p.IsRunning() {
		t.Error("planner should not be running after Stop")
	}
	// 重复 Stop 不应报错
	if err := p.Stop(); err != nil {
		t.Fatalf("double Stop failed: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultPlannerConfig()
	if cfg.Strategy != StrategyRoundRobin {
		t.Errorf("expected round_robin, got %s", cfg.Strategy)
	}
	if cfg.HealthCheckInterval != 10*time.Second {
		t.Errorf("expected 10s health check interval, got %v", cfg.HealthCheckInterval)
	}
	if cfg.FailoverDelay != 3*time.Second {
		t.Errorf("expected 3s failover delay, got %v", cfg.FailoverDelay)
	}
}

// ============================================================
// 链路管理测试
// ============================================================

func TestAddRemoveLink(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)

	// 添加链路
	err := p.AddLink(&WANLink{ID: "wan1", Name: "电信", Status: LinkStatusUp})
	if err != nil {
		t.Fatalf("AddLink failed: %v", err)
	}

	// 重复添加
	err = p.AddLink(&WANLink{ID: "wan1", Name: "电信"})
	if err != ErrLinkAlreadyExists {
		t.Errorf("expected ErrLinkAlreadyExists, got %v", err)
	}

	// 获取链路
	link, err := p.GetLink("wan1")
	if err != nil {
		t.Fatalf("GetLink failed: %v", err)
	}
	if link.Name != "电信" {
		t.Errorf("expected 电信, got %s", link.Name)
	}

	// 列出链路
	links := p.ListLinks()
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}

	// 移除链路
	err = p.RemoveLink("wan1")
	if err != nil {
		t.Fatalf("RemoveLink failed: %v", err)
	}

	// 移除不存在的链路
	err = p.RemoveLink("wan1")
	if err != ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestGetActiveLinks(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Status: LinkStatusUp})
	p.AddLink(&WANLink{ID: "wan2", Status: LinkStatusDown})
	p.AddLink(&WANLink{ID: "wan3", Status: LinkStatusUp})

	active := p.GetActiveLinks()
	if len(active) != 2 {
		t.Errorf("expected 2 active links, got %d", len(active))
	}
}

func TestSetLinkStatus(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Status: LinkStatusUp})

	err := p.SetLinkStatus("wan1", LinkStatusDown)
	if err != nil {
		t.Fatalf("SetLinkStatus failed: %v", err)
	}

	link, _ := p.GetLink("wan1")
	if link.Status != LinkStatusDown {
		t.Errorf("expected down, got %s", link.Status)
	}

	// 不存在的链路
	err = p.SetLinkStatus("wan99", LinkStatusUp)
	if err != ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestUpdateLinkScore(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Status: LinkStatusUp})

	err := p.UpdateLinkScore("wan1", 20*time.Millisecond, 0.01, 5.0)
	if err != nil {
		t.Fatalf("UpdateLinkScore failed: %v", err)
	}

	link, _ := p.GetLink("wan1")
	if link.Score <= 0 || link.Score > 100 {
		t.Errorf("invalid score: %f", link.Score)
	}
	if link.Latency != 20*time.Millisecond {
		t.Errorf("expected 20ms latency, got %v", link.Latency)
	}
}

func TestConnectionTracking(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Status: LinkStatusUp})

	p.IncrementConn("wan1")
	p.IncrementConn("wan1")
	link, _ := p.GetLink("wan1")
	if link.ActiveConns != 2 {
		t.Errorf("expected 2 conns, got %d", link.ActiveConns)
	}

	p.DecrementConn("wan1")
	link, _ = p.GetLink("wan1")
	if link.ActiveConns != 1 {
		t.Errorf("expected 1 conn, got %d", link.ActiveConns)
	}

	// 不允许负数
	p.DecrementConn("wan1")
	p.DecrementConn("wan1")
	link, _ = p.GetLink("wan1")
	if link.ActiveConns != 0 {
		t.Errorf("expected 0 conns, got %d", link.ActiveConns)
	}
}

// ============================================================
// 负载均衡测试
// ============================================================

func TestRoundRobin(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	addTestLinks(p)

	seen := map[string]int{}
	for i := 0; i < 30; i++ {
		link, err := p.SelectLink("")
		if err != nil {
			t.Fatalf("SelectLink failed: %v", err)
		}
		seen[link.ID]++
	}
	// 每条链路应被选中约 10 次
	for id, count := range seen {
		if count < 5 || count > 15 {
			t.Errorf("link %s selected %d times, expected ~10", id, count)
		}
	}
}

func TestWeighted(t *testing.T) {
	p := newTestPlanner(StrategyWeighted)
	addTestLinks(p) // 权重 3:2:1

	counts := map[string]int{}
	for i := 0; i < 60; i++ {
		link, err := p.SelectLink("")
		if err != nil {
			t.Fatalf("SelectLink failed: %v", err)
		}
		counts[link.ID]++
	}
	// wan1 (权重3) 应该最多, wan3 (权重1) 应该最少
	if counts["wan1"] <= counts["wan2"] {
		t.Errorf("wan1 (%d) should be selected more than wan2 (%d)", counts["wan1"], counts["wan2"])
	}
	if counts["wan2"] <= counts["wan3"] {
		t.Errorf("wan2 (%d) should be selected more than wan3 (%d)", counts["wan2"], counts["wan3"])
	}
}

func TestLeastConn(t *testing.T) {
	p := newTestPlanner(StrategyLeastConn)
	addTestLinks(p)

	// 给 wan1 增加连接数
	p.IncrementConn("wan1")
	p.IncrementConn("wan1")
	p.IncrementConn("wan1")

	// 给 wan2 增加连接数
	p.IncrementConn("wan2")

	// 应该优先选择 wan3 (0 connections)
	link, err := p.SelectLink("")
	if err != nil {
		t.Fatalf("SelectLink failed: %v", err)
	}
	if link.ID != "wan3" {
		t.Errorf("expected wan3 (least conns), got %s", link.ID)
	}
}

func TestSourceHash(t *testing.T) {
	p := newTestPlanner(StrategySourceHash)
	addTestLinks(p)

	// 同一源 IP 应该总是选择同一链路
	link1, _ := p.SelectLink("192.168.1.100")
	link2, _ := p.SelectLink("192.168.1.100")
	if link1.ID != link2.ID {
		t.Errorf("same source IP should get same link: %s vs %s", link1.ID, link2.ID)
	}

	// 不同源 IP 可能选择不同链路
	link3, _ := p.SelectLink("10.0.0.1")
	// 不强制不同，但验证不会 panic
	_ = link3
}

func TestNoAvailableLinks(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Status: LinkStatusDown})

	_, err := p.SelectLink("")
	if err != ErrNoAvailableLinks {
		t.Errorf("expected ErrNoAvailableLinks, got %v", err)
	}
}

func TestSetStrategy(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)

	err := p.SetStrategy(StrategyWeighted)
	if err != nil {
		t.Fatalf("SetStrategy failed: %v", err)
	}
	cfg := p.GetConfig()
	if cfg.Strategy != StrategyWeighted {
		t.Errorf("expected weighted, got %s", cfg.Strategy)
	}

	// 无效策略
	err = p.SetStrategy("invalid")
	if err != ErrInvalidStrategy {
		t.Errorf("expected ErrInvalidStrategy, got %v", err)
	}
}

// ============================================================
// QoS 测试
// ============================================================

func TestAddRemoveQoSRule(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)

	rule := &QoSRule{
		ID:       "r1",
		Name:     "HTTP",
		Priority: QoSPriorityMedium,
		Protocol: "tcp",
		DstPort:  80,
		Enabled:  true,
	}
	err := p.AddQoSRule(rule)
	if err != nil {
		t.Fatalf("AddQoSRule failed: %v", err)
	}

	// 重复添加
	err = p.AddQoSRule(rule)
	if err == nil {
		t.Error("expected error for duplicate rule")
	}

	// 获取规则
	r, err := p.GetQoSRule("r1")
	if err != nil {
		t.Fatalf("GetQoSRule failed: %v", err)
	}
	if r.Name != "HTTP" {
		t.Errorf("expected HTTP, got %s", r.Name)
	}

	// 列出规则
	rules := p.ListQoSRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	// 移除规则
	err = p.RemoveQoSRule("r1")
	if err != nil {
		t.Fatalf("RemoveQoSRule failed: %v", err)
	}
}

func TestClassifyTraffic(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)

	p.AddQoSRule(&QoSRule{ID: "r1", Name: "HTTP", Priority: QoSPriorityMedium, Protocol: "tcp", DstPort: 80, Enabled: true})
	p.AddQoSRule(&QoSRule{ID: "r2", Name: "SSH", Priority: QoSPriorityHigh, Protocol: "tcp", DstPort: 22, Enabled: true})
	p.AddQoSRule(&QoSRule{ID: "r3", Name: "All", Priority: QoSPriorityLow, Protocol: "any", Enabled: true})

	// 匹配 SSH (更高优先级)
	rule := p.ClassifyTraffic("tcp", 0, 22)
	if rule == nil || rule.Name != "SSH" {
		t.Errorf("expected SSH rule, got %v", rule)
	}

	// 匹配 HTTP
	rule = p.ClassifyTraffic("tcp", 0, 80)
	if rule == nil || rule.Name != "HTTP" {
		t.Errorf("expected HTTP rule, got %v", rule)
	}

	// 匹配 any
	rule = p.ClassifyTraffic("udp", 12345, 53)
	if rule == nil || rule.Name != "All" {
		t.Errorf("expected All rule, got %v", rule)
	}
}

func TestEnableDisableQoSRule(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddQoSRule(&QoSRule{ID: "r1", Name: "HTTP", Priority: QoSPriorityMedium, Protocol: "tcp", DstPort: 80, Enabled: true})

	p.DisableQoSRule("r1")
	rule, _ := p.GetQoSRule("r1")
	if rule.Enabled {
		t.Error("rule should be disabled")
	}

	p.EnableQoSRule("r1")
	rule, _ = p.GetQoSRule("r1")
	if !rule.Enabled {
		t.Error("rule should be enabled")
	}
}

func TestGetEffectiveBandwidth(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Bandwidth: 100_000_000, Status: LinkStatusUp})
	p.AddQoSRule(&QoSRule{ID: "r1", Priority: QoSPriorityLow, MaxBandwidth: 10_000_000, Enabled: true})

	bw, err := p.GetEffectiveBandwidth("wan1", QoSPriorityLow)
	if err != nil {
		t.Fatalf("GetEffectiveBandwidth failed: %v", err)
	}
	if bw != 10_000_000 {
		t.Errorf("expected 10Mbps, got %d", bw)
	}

	// 无限速规则的优先级应返回链路带宽
	bw, err = p.GetEffectiveBandwidth("wan1", QoSPriorityCritical)
	if err != nil {
		t.Fatalf("GetEffectiveBandwidth failed: %v", err)
	}
	if bw != 100_000_000 {
		t.Errorf("expected 100Mbps, got %d", bw)
	}
}

// ============================================================
// 故障切换测试
// ============================================================

func TestExecuteFailover(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	addTestLinks(p)

	event, err := p.ExecuteFailover("wan1", "wan2", "test failover")
	if err != nil {
		t.Fatalf("ExecuteFailover failed: %v", err)
	}
	if event.FromLinkID != "wan1" || event.ToLinkID != "wan2" {
		t.Errorf("unexpected failover: %s -> %s", event.FromLinkID, event.ToLinkID)
	}

	// wan1 应该被标记为 down
	link, _ := p.GetLink("wan1")
	if link.Status != LinkStatusDown {
		t.Errorf("expected wan1 down, got %s", link.Status)
	}

	// 故障切换日志
	log := p.GetFailoverLog()
	if len(log) != 1 {
		t.Errorf("expected 1 failover event, got %d", len(log))
	}
}

func TestExecuteFailoverErrors(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	addTestLinks(p)

	// 到不存在的链路
	_, err := p.ExecuteFailover("wan1", "wan99", "test")
	if err == nil {
		t.Error("expected error for non-existent target")
	}

	// 到 down 的链路
	p.SetLinkStatus("wan3", LinkStatusDown)
	_, err = p.ExecuteFailover("wan1", "wan3", "test")
	if err == nil {
		t.Error("expected error for down target")
	}
}

func TestExecuteFallback(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	addTestLinks(p)

	// 先故障切换
	p.ExecuteFailover("wan1", "wan2", "test")
	// 恢复 wan1
	p.SetLinkStatus("wan1", LinkStatusUp)

	event, err := p.ExecuteFallback("wan1", "wan2")
	if err != nil {
		t.Fatalf("ExecuteFallback failed: %v", err)
	}
	if !event.IsFallback {
		t.Error("expected IsFallback=true")
	}
}

func TestGetFailoverChain(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	addTestLinks(p)

	p.ExecuteFailover("wan1", "wan2", "fail1")
	p.SetLinkStatus("wan1", LinkStatusUp)
	p.ExecuteFailover("wan2", "wan3", "fail2")

	chain := p.GetFailoverChain("wan1")
	if len(chain) < 2 {
		t.Errorf("expected chain length >= 2, got %d", len(chain))
	}
}

func TestAutoRecover(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Status: LinkStatusDegraded, Score: 70, LastCheck: time.Now()})

	recovered := p.AutoRecover()
	if len(recovered) != 1 || recovered[0] != "wan1" {
		t.Errorf("expected wan1 recovered, got %v", recovered)
	}
	link, _ := p.GetLink("wan1")
	if link.Status != LinkStatusUp {
		t.Errorf("expected up, got %s", link.Status)
	}
}

func TestFailoverConfig(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	fo, fb := p.GetFailoverConfig()
	if fo != 3*time.Second {
		t.Errorf("expected 3s failover delay, got %v", fo)
	}
	if fb != 30*time.Second {
		t.Errorf("expected 30s fallback delay, got %v", fb)
	}
}

// ============================================================
// 带宽预测测试
// ============================================================

func TestPredictBandwidth(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Bandwidth: 100_000_000, Status: LinkStatusUp})

	// 添加足够的历史数据
	now := time.Now()
	for i := 0; i < 20; i++ {
		p.RecordSample(BandwidthSample{
			Timestamp:   now.Add(-time.Duration(20-i) * time.Minute),
			LinkID:      "wan1",
			BytesIn:     int64(50_000_000 + i*1_000_000),
			BytesOut:    int64(10_000_000),
			Utilization: 0.5 + float64(i)*0.01,
		})
	}

	result, err := p.PredictBandwidth("wan1")
	if err != nil {
		t.Fatalf("PredictBandwidth failed: %v", err)
	}
	if result.LinkID != "wan1" {
		t.Errorf("expected wan1, got %s", result.LinkID)
	}
	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Errorf("invalid confidence: %f", result.Confidence)
	}
	if result.EstimatedBW < 0 {
		t.Errorf("negative estimated bandwidth: %d", result.EstimatedBW)
	}
}

func TestPredictInsufficientData(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Bandwidth: 100_000_000, Status: LinkStatusUp})

	// 只添加 3 个样本（不足 5 个）
	now := time.Now()
	for i := 0; i < 3; i++ {
		p.RecordSample(BandwidthSample{
			Timestamp:   now.Add(-time.Duration(3-i) * time.Minute),
			LinkID:      "wan1",
			Utilization: 0.5,
		})
	}

	_, err := p.PredictBandwidth("wan1")
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestPredictAllLinks(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddLink(&WANLink{ID: "wan1", Bandwidth: 100_000_000, Status: LinkStatusUp})
	p.AddLink(&WANLink{ID: "wan2", Bandwidth: 50_000_000, Status: LinkStatusUp})

	now := time.Now()
	for i := 0; i < 10; i++ {
		p.RecordSample(BandwidthSample{
			Timestamp:   now.Add(-time.Duration(10-i) * time.Minute),
			LinkID:      "wan1",
			Utilization: 0.5,
		})
		p.RecordSample(BandwidthSample{
			Timestamp:   now.Add(-time.Duration(10-i) * time.Minute),
			LinkID:      "wan2",
			Utilization: 0.3,
		})
	}

	results := p.PredictAllLinks()
	if len(results) != 2 {
		t.Errorf("expected 2 predictions, got %d", len(results))
	}
}

func TestGetUtilizationTrend(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	now := time.Now()

	for i := 0; i < 10; i++ {
		p.RecordSample(BandwidthSample{
			Timestamp:   now.Add(-time.Duration(10-i) * time.Minute),
			LinkID:      "wan1",
			Utilization: float64(i) / 10.0,
		})
	}

	trend := p.GetUtilizationTrend("wan1", 15*time.Minute)
	if len(trend) != 10 {
		t.Errorf("expected 10 samples, got %d", len(trend))
	}
	// 验证排序
	for i := 1; i < len(trend); i++ {
		if trend[i].Timestamp.Before(trend[i-1].Timestamp) {
			t.Error("trend not sorted by time")
		}
	}
}

func TestGetPeakUsage(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	now := time.Now()

	for i := 0; i < 10; i++ {
		p.RecordSample(BandwidthSample{
			Timestamp:   now.Add(-time.Duration(10-i) * time.Minute),
			LinkID:      "wan1",
			Utilization: float64(i) / 10.0,
		})
	}

	peak, _ := p.GetPeakUsage("wan1", 15*time.Minute)
	if peak < 0.8 {
		t.Errorf("expected peak >= 0.8, got %f", peak)
	}
}

// ============================================================
// 统计与采样测试
// ============================================================

func TestRecordSample(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	now := time.Now()

	p.RecordSample(BandwidthSample{
		Timestamp:   now,
		LinkID:      "wan1",
		BytesIn:     1000,
		BytesOut:    500,
		Utilization: 0.5,
	})

	history := p.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 sample, got %d", len(history))
	}
}

func TestGetStats(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	addTestLinks(p)

	stats := p.GetStats()
	if stats.TotalLinks != 3 {
		t.Errorf("expected 3 total links, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 3 {
		t.Errorf("expected 3 active links, got %d", stats.ActiveLinks)
	}
	if stats.AvgScore < 50 || stats.AvgScore > 100 {
		t.Errorf("unexpected avg score: %f", stats.AvgScore)
	}

	str := stats.String()
	if str == "" {
		t.Error("expected non-empty stats string")
	}
}

func TestGetHistory(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	if len(p.GetHistory()) != 0 {
		t.Error("expected empty history")
	}
}

// ============================================================
// QoS 边界测试
// ============================================================

func TestAddQoSRuleNoID(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	err := p.AddQoSRule(&QoSRule{Name: "test"})
	if err == nil {
		t.Error("expected error for empty rule ID")
	}
}

func TestUpdateQoSRule(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddQoSRule(&QoSRule{ID: "r1", Name: "HTTP", Enabled: true})

	err := p.UpdateQoSRule(&QoSRule{ID: "r1", Name: "HTTPS", DstPort: 443, Enabled: true})
	if err != nil {
		t.Fatalf("UpdateQoSRule failed: %v", err)
	}
	rule, _ := p.GetQoSRule("r1")
	if rule.Name != "HTTPS" {
		t.Errorf("expected HTTPS, got %s", rule.Name)
	}

	// 更新不存在的规则
	err = p.UpdateQoSRule(&QoSRule{ID: "r99", Name: "X"})
	if err != ErrRuleNotFound {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestGetTrafficClasses(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddQoSRule(&QoSRule{ID: "r1", Name: "Web", Priority: QoSPriorityMedium, Enabled: true})
	p.AddQoSRule(&QoSRule{ID: "r2", Name: "SSH", Priority: QoSPriorityHigh, Enabled: true})

	classes := p.GetTrafficClasses()
	if len(classes) != 2 {
		t.Errorf("expected 2 classes, got %d", len(classes))
	}
}

func TestClassifyTrafficDisabled(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	p.AddQoSRule(&QoSRule{ID: "r1", Name: "HTTP", Priority: QoSPriorityMedium, Protocol: "tcp", DstPort: 80, Enabled: false})

	rule := p.ClassifyTraffic("tcp", 0, 80)
	if rule != nil {
		t.Error("disabled rule should not match")
	}
}

func TestGetEffectiveBandwidthNoLink(t *testing.T) {
	p := newTestPlanner(StrategyRoundRobin)
	_, err := p.GetEffectiveBandwidth("wan99", QoSPriorityLow)
	if err != ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

// ============================================================
// 探测测试
// ============================================================

func TestExecuteProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network probe in short mode")
	}
	link := &WANLink{ID: "wan1", Gateway: "127.0.0.1"}
	target := ProbeTarget{Type: ProbeTCP, Host: "127.0.0.1", Port: 1}
	result := ExecuteProbe(link, target, 1*time.Second)
	// 端口 1 通常不可达
	if result.LinkID != "wan1" {
		t.Errorf("expected wan1, got %s", result.LinkID)
	}
}

func TestCalculateScore(t *testing.T) {
	// 低延迟、低丢包、低抖动 → 高分
	score := calculateScore(10*time.Millisecond, 0.001, 2.0)
	if score < 80 {
		t.Errorf("expected high score, got %f", score)
	}

	// 高延迟、高丢包、高抖动 → 低分
	score = calculateScore(500*time.Millisecond, 0.5, 100.0)
	if score > 55 {
		t.Errorf("expected low score, got %f", score)
	}
}

func TestLinearRegression(t *testing.T) {
	samples := make([]BandwidthSample, 10)
	for i := range samples {
		samples[i] = BandwidthSample{Utilization: float64(i) * 0.1}
	}
	slope, intercept := linearRegression(samples)
	if slope <= 0 {
		t.Errorf("expected positive slope, got %f", slope)
	}
	if intercept < 0 {
		t.Errorf("expected non-negative intercept, got %f", intercept)
	}
}

func TestCalculateConfidence(t *testing.T) {
	// 稳定数据 → 高置信度
	stable := make([]BandwidthSample, 20)
	for i := range stable {
		stable[i] = BandwidthSample{Utilization: 0.5}
	}
	conf := calculateConfidence(stable)
	if conf < 0.3 {
		t.Errorf("expected high confidence for stable data, got %f", conf)
	}

	// 波动数据 → 低置信度
	volatile := make([]BandwidthSample, 20)
	for i := range volatile {
		volatile[i] = BandwidthSample{Utilization: float64(i%2) * 0.5}
	}
	conf2 := calculateConfidence(volatile)
	if conf2 > conf {
		t.Errorf("volatile confidence (%f) should be lower than stable (%f)", conf2, conf)
	}
}
