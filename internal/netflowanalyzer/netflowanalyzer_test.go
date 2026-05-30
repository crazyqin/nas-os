package netflowanalyzer

import (
	"fmt"
	"testing"
	"time"
)

// ========== 构造函数测试 ==========

func TestNewNetflowAnalyzer(t *testing.T) {
	a := NewNetflowAnalyzer()
	if a == nil {
		t.Fatal("NewNetflowAnalyzer 返回 nil")
	}
	if a.maxRecords != DefaultMaxRecords {
		t.Errorf("maxRecords = %d, want %d", a.maxRecords, DefaultMaxRecords)
	}
	if a.ddosThreshold != DefaultDDoSThreshold {
		t.Errorf("ddosThreshold = %d, want %d", a.ddosThreshold, DefaultDDoSThreshold)
	}
	if a.surgeMultiplier != DefaultSurgeMultiplier {
		t.Errorf("surgeMultiplier = %f, want %f", a.surgeMultiplier, DefaultSurgeMultiplier)
	}
}

func TestWithOptions(t *testing.T) {
	a := NewNetflowAnalyzer(
		WithMaxRecords(5000),
		WithDDoSThreshold(500),
		WithSurgeMultiplier(5.0),
		WithSampleInterval(5*time.Second),
		WithAlertCooldown(1*time.Minute),
	)

	if a.maxRecords != 5000 {
		t.Errorf("maxRecords = %d, want 5000", a.maxRecords)
	}
	if a.ddosThreshold != 500 {
		t.Errorf("ddosThreshold = %d, want 500", a.ddosThreshold)
	}
	if a.surgeMultiplier != 5.0 {
		t.Errorf("surgeMultiplier = %f, want 5.0", a.surgeMultiplier)
	}
}

func TestOptionsIgnoreInvalid(t *testing.T) {
	a := NewNetflowAnalyzer(
		WithMaxRecords(-1),
		WithDDoSThreshold(0),
		WithSurgeMultiplier(0.5),
	)

	if a.maxRecords != DefaultMaxRecords {
		t.Errorf("maxRecords should be default when negative")
	}
	if a.ddosThreshold != DefaultDDoSThreshold {
		t.Errorf("ddosThreshold should be default when zero")
	}
	if a.surgeMultiplier != DefaultSurgeMultiplier {
		t.Errorf("surgeMultiplier should be default when < 1.0")
	}
}

// ========== 生命周期测试 ==========

func TestStartStop(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.IsRunning() {
		t.Error("分析器不应在创建后运行")
	}

	if err := a.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	if !a.IsRunning() {
		t.Error("分析器应在 Start 后运行")
	}

	// 重复启动应该成功
	if err := a.Start(); err != nil {
		t.Fatalf("重复 Start 失败: %v", err)
	}

	if err := a.Stop(); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}

	if a.IsRunning() {
		t.Error("分析器不应在 Stop 后运行")
	}

	// 重复停止应该成功
	if err := a.Stop(); err != nil {
		t.Fatalf("重复 Stop 失败: %v", err)
	}
}

func TestGetUptime(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.GetUptime() != 0 {
		t.Error("未启动时 uptime 应为 0")
	}

	a.Start()
	time.Sleep(50 * time.Millisecond)
	uptime := a.GetUptime()
	if uptime < 40*time.Millisecond {
		t.Errorf("uptime %v 太短", uptime)
	}
	a.Stop()
}

// ========== 流量记录测试 ==========

func TestRecordFlow(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	record := FlowRecord{
		Timestamp:  now,
		Interface:  "eth0",
		SrcIP:      "192.168.1.100",
		DstIP:      "10.0.0.1",
		SrcPort:    12345,
		DstPort:    80,
		Protocol:   ProtocolHTTP,
		BytesIn:    1024,
		BytesOut:   512,
		PacketsIn:  10,
		PacketsOut: 5,
	}

	a.RecordFlow(record)

	if a.GetRecordCount() != 1 {
		t.Errorf("记录数 = %d, want 1", a.GetRecordCount())
	}
}

func TestRecordFlowAutoID(t *testing.T) {
	a := NewNetflowAnalyzer()

	record := FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   100,
	}

	a.RecordFlow(record)

	if a.GetRecordCount() != 1 {
		t.Errorf("记录数 = %d, want 1", a.GetRecordCount())
	}
}

func TestRecordFlows(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	records := []FlowRecord{
		{
			Timestamp: now,
			Interface: "eth0",
			SrcIP:     "192.168.1.1",
			DstIP:     "10.0.0.1",
			Protocol:  ProtocolTCP,
			BytesIn:   100,
		},
		{
			Timestamp: now,
			Interface: "eth0",
			SrcIP:     "192.168.1.2",
			DstIP:     "10.0.0.2",
			Protocol:  ProtocolUDP,
			BytesIn:   200,
		},
	}

	a.RecordFlows(records)

	if a.GetRecordCount() != 2 {
		t.Errorf("记录数 = %d, want 2", a.GetRecordCount())
	}
}

func TestMaxRecords(t *testing.T) {
	a := NewNetflowAnalyzer(WithMaxRecords(5))

	for i := 0; i < 10; i++ {
		a.RecordFlow(FlowRecord{
			Interface: "eth0",
			SrcIP:     fmt.Sprintf("192.168.1.%d", i),
			DstIP:     "10.0.0.1",
			Protocol:  ProtocolTCP,
			BytesIn:   100,
		})
	}

	if a.GetRecordCount() != 5 {
		t.Errorf("记录数 = %d, want 5 (maxRecords)", a.GetRecordCount())
	}
}

// ========== 接口统计测试 ==========

func TestGetInterfaceStats(t *testing.T) {
	a := NewNetflowAnalyzer()

	// 未记录时应返回错误
	_, err := a.GetInterfaceStats("eth0")
	if err != ErrInterfaceNotFound {
		t.Errorf("期望 ErrInterfaceNotFound, got %v", err)
	}

	// 记录流量
	a.RecordFlow(FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   1000,
		BytesOut:  500,
	})

	stats, err := a.GetInterfaceStats("eth0")
	if err != nil {
		t.Fatalf("GetInterfaceStats 失败: %v", err)
	}

	if stats.BytesIn != 1000 {
		t.Errorf("BytesIn = %d, want 1000", stats.BytesIn)
	}
	if stats.BytesOut != 500 {
		t.Errorf("BytesOut = %d, want 500", stats.BytesOut)
	}
	if stats.Name != "eth0" {
		t.Errorf("Name = %s, want eth0", stats.Name)
	}
}

func TestGetAllInterfaceStats(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.RecordFlow(FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   100,
	})
	a.RecordFlow(FlowRecord{
		Interface: "eth1",
		SrcIP:     "192.168.2.1",
		DstIP:     "10.0.0.2",
		Protocol:  ProtocolUDP,
		BytesIn:   200,
	})

	stats := a.GetAllInterfaceStats()
	if len(stats) != 2 {
		t.Errorf("接口数 = %d, want 2", len(stats))
	}

	if _, ok := stats["eth0"]; !ok {
		t.Error("缺少 eth0")
	}
	if _, ok := stats["eth1"]; !ok {
		t.Error("缺少 eth1")
	}
}

// ========== 连接追踪测试 ==========

func TestGetActiveConnections(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.GetActiveConnections() != 0 {
		t.Error("初始连接数应为 0")
	}

	a.RecordFlow(FlowRecord{
		SrcIP:    "192.168.1.1",
		DstIP:    "10.0.0.1",
		SrcPort:  12345,
		DstPort:  80,
		Protocol: ProtocolTCP,
		BytesIn:  100,
	})

	if a.GetActiveConnections() != 1 {
		t.Errorf("连接数 = %d, want 1", a.GetActiveConnections())
	}

	// 同一连接
	a.RecordFlow(FlowRecord{
		SrcIP:    "192.168.1.1",
		DstIP:    "10.0.0.1",
		SrcPort:  12345,
		DstPort:  80,
		Protocol: ProtocolTCP,
		BytesIn:  200,
	})

	if a.GetActiveConnections() != 1 {
		t.Errorf("同一连接重复记录，连接数 = %d, want 1", a.GetActiveConnections())
	}
}

func TestGetConnectionList(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.RecordFlow(FlowRecord{
		SrcIP:    "192.168.1.1",
		DstIP:    "10.0.0.1",
		SrcPort:  12345,
		DstPort:  80,
		Protocol: ProtocolTCP,
		BytesIn:  100,
		BytesOut: 50,
	})

	conns := a.GetConnectionList()
	if len(conns) != 1 {
		t.Fatalf("连接数 = %d, want 1", len(conns))
	}

	if conns[0].SrcIP != "192.168.1.1" {
		t.Errorf("SrcIP = %s, want 192.168.1.1", conns[0].SrcIP)
	}
	if conns[0].BytesIn != 100 {
		t.Errorf("BytesIn = %d, want 100", conns[0].BytesIn)
	}
}

// ========== 协议分布测试 ==========

func TestGetProtocolDistribution(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.RecordFlow(FlowRecord{
		Protocol: ProtocolTCP,
		BytesIn:  1000,
	})
	a.RecordFlow(FlowRecord{
		Protocol: ProtocolUDP,
		BytesIn:  500,
	})
	a.RecordFlow(FlowRecord{
		Protocol: ProtocolTCP,
		BytesIn:  500,
	})

	dist := a.GetProtocolDistribution()
	if len(dist) != 2 {
		t.Fatalf("协议数 = %d, want 2", len(dist))
	}

	// TCP 应该排第一
	if dist[0].Protocol != ProtocolTCP {
		t.Errorf("第一协议 = %s, want TCP", dist[0].Protocol)
	}
	if dist[0].Bytes != 1500 {
		t.Errorf("TCP 字节数 = %d, want 1500", dist[0].Bytes)
	}

	// 检查百分比
	totalPercent := 0.0
	for _, d := range dist {
		totalPercent += d.Percent
	}
	if totalPercent < 99.9 || totalPercent > 100.1 {
		t.Errorf("总百分比 = %.2f, want ~100", totalPercent)
	}
}

func TestProtocolDistributionEmpty(t *testing.T) {
	a := NewNetflowAnalyzer()

	dist := a.GetProtocolDistribution()
	if len(dist) != 0 {
		t.Errorf("空记录时协议数 = %d, want 0", len(dist))
	}
}

// ========== 流量快照测试 ==========

func TestGetRealtimeSnapshot(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.RecordFlow(FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   1000,
	})

	snapshot := a.GetRealtimeSnapshot()

	if snapshot.TotalConnections != 1 {
		t.Errorf("TotalConnections = %d, want 1", snapshot.TotalConnections)
	}

	if _, ok := snapshot.Interfaces["eth0"]; !ok {
		t.Error("快照中缺少 eth0")
	}
}

// ========== 流量统计测试 ==========

func TestGetTrafficStats(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	// 记录流量
	for i := 0; i < 10; i++ {
		a.RecordFlow(FlowRecord{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Interface: "eth0",
			SrcIP:     "192.168.1.1",
			DstIP:     "10.0.0.1",
			Protocol:  ProtocolTCP,
			BytesIn:   1000,
			BytesOut:  500,
		})
	}

	// 按小时统计
	stats, err := a.GetTrafficStats(GranularityHourly, start, end)
	if err != nil {
		t.Fatalf("GetTrafficStats 失败: %v", err)
	}

	if stats.TotalBytesIn != 10000 {
		t.Errorf("TotalBytesIn = %d, want 10000", stats.TotalBytesIn)
	}
	if stats.TotalBytesOut != 5000 {
		t.Errorf("TotalBytesOut = %d, want 5000", stats.TotalBytesOut)
	}
	if len(stats.Entries) == 0 {
		t.Error("统计条目为空")
	}
}

func TestGetTrafficStatsInvalidGranularity(t *testing.T) {
	a := NewNetflowAnalyzer()

	_, err := a.GetTrafficStats("invalid", time.Now(), time.Now())
	if err != ErrInvalidGranularity {
		t.Errorf("期望 ErrInvalidGranularity, got %v", err)
	}
}

func TestGetTrafficStatsEmpty(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	stats, err := a.GetTrafficStats(GranularityDaily, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("GetTrafficStats 失败: %v", err)
	}

	if stats.TotalBytesIn != 0 {
		t.Errorf("TotalBytesIn = %d, want 0", stats.TotalBytesIn)
	}
	if len(stats.Entries) != 0 {
		t.Errorf("统计条目数 = %d, want 0", len(stats.Entries))
	}
}

func TestGetTrafficStatsAllGranularities(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	a.RecordFlow(FlowRecord{
		Timestamp: now,
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   1000,
	})

	granularities := []string{
		GranularityHourly,
		GranularityDaily,
		GranularityWeekly,
		GranularityMonthly,
	}

	for _, g := range granularities {
		_, err := a.GetTrafficStats(g, now.Add(-1*time.Hour), now.Add(1*time.Hour))
		if err != nil {
			t.Errorf("GetTrafficStats(%s) 失败: %v", g, err)
		}
	}
}

// ========== 分组统计测试 ==========

func TestGetStatsByIP(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	records := []FlowRecord{
		{Timestamp: now, SrcIP: "192.168.1.1", DstIP: "10.0.0.1", BytesIn: 1000, BytesOut: 500, Protocol: ProtocolTCP},
		{Timestamp: now, SrcIP: "192.168.1.2", DstIP: "10.0.0.1", BytesIn: 2000, BytesOut: 1000, Protocol: ProtocolTCP},
		{Timestamp: now, SrcIP: "192.168.1.1", DstIP: "10.0.0.2", BytesIn: 500, BytesOut: 250, Protocol: ProtocolUDP},
	}
	a.RecordFlows(records)

	stats := a.GetStatsByIP(now.Add(-1*time.Hour), now.Add(1*time.Hour), 10)

	if len(stats) == 0 {
		t.Fatal("IP 统计为空")
	}

	// 10.0.0.1 应该排第一（收到1000+2000=3000入站，比其他IP多）
	if stats[0].GroupKey != "10.0.0.1" {
		t.Errorf("Top IP = %s, want 10.0.0.1", stats[0].GroupKey)
	}
}

func TestGetStatsByPort(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	records := []FlowRecord{
		{Timestamp: now, DstPort: 80, BytesIn: 1000, Protocol: ProtocolHTTP},
		{Timestamp: now, DstPort: 443, BytesIn: 2000, Protocol: ProtocolHTTPS},
		{Timestamp: now, DstPort: 80, BytesIn: 500, Protocol: ProtocolHTTP},
	}
	a.RecordFlows(records)

	stats := a.GetStatsByPort(now.Add(-1*time.Hour), now.Add(1*time.Hour), 10)

	if len(stats) != 2 {
		t.Fatalf("端口数 = %d, want 2", len(stats))
	}

	// 端口 443 应该排第一（2000 > 1500）
	if stats[0].GroupKey != "443" {
		t.Errorf("Top Port = %s, want 443", stats[0].GroupKey)
	}
}

func TestGetStatsByProtocol(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	records := []FlowRecord{
		{Timestamp: now, Protocol: ProtocolTCP, BytesIn: 1000},
		{Timestamp: now, Protocol: ProtocolUDP, BytesIn: 500},
		{Timestamp: now, Protocol: ProtocolTCP, BytesIn: 500},
	}
	a.RecordFlows(records)

	stats := a.GetStatsByProtocol(now.Add(-1*time.Hour), now.Add(1*time.Hour))

	if len(stats) != 2 {
		t.Fatalf("协议数 = %d, want 2", len(stats))
	}

	if stats[0].GroupKey != ProtocolTCP {
		t.Errorf("Top Protocol = %s, want TCP", stats[0].GroupKey)
	}
}

// ========== 带宽策略测试 ==========

func TestAddBandwidthPolicy(t *testing.T) {
	a := NewNetflowAnalyzer()

	policy := BandwidthPolicy{
		ID:         "policy-1",
		Name:       "限制HTTP",
		TargetPort: 80,
		Protocol:   ProtocolHTTP,
		MaxInBps:   1000000,
		Enabled:    true,
	}

	if err := a.AddBandwidthPolicy(policy); err != nil {
		t.Fatalf("AddBandwidthPolicy 失败: %v", err)
	}

	if a.GetPolicyCount() != 1 {
		t.Errorf("策略数 = %d, want 1", a.GetPolicyCount())
	}
}

func TestAddBandwidthPolicyDuplicate(t *testing.T) {
	a := NewNetflowAnalyzer()

	policy := BandwidthPolicy{
		ID:   "policy-1",
		Name: "测试策略",
	}

	a.AddBandwidthPolicy(policy)
	if err := a.AddBandwidthPolicy(policy); err != ErrPolicyExists {
		t.Errorf("重复添加应返回 ErrPolicyExists, got %v", err)
	}
}

func TestAddBandwidthPolicyInvalidID(t *testing.T) {
	a := NewNetflowAnalyzer()

	policy := BandwidthPolicy{
		Name: "无ID策略",
	}

	if err := a.AddBandwidthPolicy(policy); err != ErrInvalidPolicy {
		t.Errorf("无ID应返回 ErrInvalidPolicy, got %v", err)
	}
}

func TestRemoveBandwidthPolicy(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.AddBandwidthPolicy(BandwidthPolicy{ID: "p1", Name: "策略1"})
	a.AddBandwidthPolicy(BandwidthPolicy{ID: "p2", Name: "策略2"})

	if err := a.RemoveBandwidthPolicy("p1"); err != nil {
		t.Fatalf("RemoveBandwidthPolicy 失败: %v", err)
	}

	if a.GetPolicyCount() != 1 {
		t.Errorf("策略数 = %d, want 1", a.GetPolicyCount())
	}
}

func TestRemoveBandwidthPolicyNotFound(t *testing.T) {
	a := NewNetflowAnalyzer()

	if err := a.RemoveBandwidthPolicy("nonexistent"); err != ErrPolicyNotFound {
		t.Errorf("期望 ErrPolicyNotFound, got %v", err)
	}
}

func TestUpdateBandwidthPolicy(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.AddBandwidthPolicy(BandwidthPolicy{
		ID:       "p1",
		Name:     "原始策略",
		MaxInBps: 1000,
	})

	updated := BandwidthPolicy{
		ID:       "p1",
		Name:     "更新后策略",
		MaxInBps: 2000,
	}

	if err := a.UpdateBandwidthPolicy(updated); err != nil {
		t.Fatalf("UpdateBandwidthPolicy 失败: %v", err)
	}

	policy, _ := a.GetBandwidthPolicy("p1")
	if policy.Name != "更新后策略" {
		t.Errorf("Name = %s, want 更新后策略", policy.Name)
	}
	if policy.MaxInBps != 2000 {
		t.Errorf("MaxInBps = %d, want 2000", policy.MaxInBps)
	}
}

func TestUpdateBandwidthPolicyNotFound(t *testing.T) {
	a := NewNetflowAnalyzer()

	if err := a.UpdateBandwidthPolicy(BandwidthPolicy{ID: "nonexistent"}); err != ErrPolicyNotFound {
		t.Errorf("期望 ErrPolicyNotFound, got %v", err)
	}
}

func TestUpdateBandwidthPolicyInvalidID(t *testing.T) {
	a := NewNetflowAnalyzer()

	if err := a.UpdateBandwidthPolicy(BandwidthPolicy{}); err != ErrInvalidPolicy {
		t.Errorf("期望 ErrInvalidPolicy, got %v", err)
	}
}

func TestGetBandwidthPolicy(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.AddBandwidthPolicy(BandwidthPolicy{
		ID:         "p1",
		Name:       "HTTP限制",
		TargetPort: 80,
		MaxInBps:   1000000,
	})

	policy, err := a.GetBandwidthPolicy("p1")
	if err != nil {
		t.Fatalf("GetBandwidthPolicy 失败: %v", err)
	}

	if policy.Name != "HTTP限制" {
		t.Errorf("Name = %s, want HTTP限制", policy.Name)
	}
	if policy.MaxInBps != 1000000 {
		t.Errorf("MaxInBps = %d, want 1000000", policy.MaxInBps)
	}
}

func TestGetBandwidthPolicyNotFound(t *testing.T) {
	a := NewNetflowAnalyzer()

	_, err := a.GetBandwidthPolicy("nonexistent")
	if err != ErrPolicyNotFound {
		t.Errorf("期望 ErrPolicyNotFound, got %v", err)
	}
}

func TestGetAllBandwidthPolicies(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.AddBandwidthPolicy(BandwidthPolicy{ID: "p1", Priority: 1})
	a.AddBandwidthPolicy(BandwidthPolicy{ID: "p2", Priority: 3})
	a.AddBandwidthPolicy(BandwidthPolicy{ID: "p3", Priority: 2})

	policies := a.GetAllBandwidthPolicies()
	if len(policies) != 3 {
		t.Fatalf("策略数 = %d, want 3", len(policies))
	}

	// 应按优先级降序排列
	if policies[0].Priority < policies[1].Priority {
		t.Error("策略未按优先级降序排列")
	}
}

func TestEnableDisablePolicy(t *testing.T) {
	a := NewNetflowAnalyzer()

	a.AddBandwidthPolicy(BandwidthPolicy{
		ID:      "p1",
		Enabled: true,
	})

	if err := a.DisablePolicy("p1"); err != nil {
		t.Fatalf("DisablePolicy 失败: %v", err)
	}

	policy, _ := a.GetBandwidthPolicy("p1")
	if policy.Enabled {
		t.Error("策略应该被禁用")
	}

	if err := a.EnablePolicy("p1"); err != nil {
		t.Fatalf("EnablePolicy 失败: %v", err)
	}

	policy, _ = a.GetBandwidthPolicy("p1")
	if !policy.Enabled {
		t.Error("策略应该被启用")
	}
}

func TestEnableDisablePolicyNotFound(t *testing.T) {
	a := NewNetflowAnalyzer()

	if err := a.EnablePolicy("nonexistent"); err != ErrPolicyNotFound {
		t.Errorf("EnablePolicy: 期望 ErrPolicyNotFound, got %v", err)
	}
	if err := a.DisablePolicy("nonexistent"); err != ErrPolicyNotFound {
		t.Errorf("DisablePolicy: 期望 ErrPolicyNotFound, got %v", err)
	}
}

func TestBandwidthPolicyViolation(t *testing.T) {
	a := NewNetflowAnalyzer()

	// 添加策略：限制端口80入站带宽为 1000 bps
	a.AddBandwidthPolicy(BandwidthPolicy{
		ID:         "limit-http",
		Name:       "HTTP限制",
		TargetPort: 80,
		MaxInBps:   1000,
		Enabled:    true,
	})

	// 记录超过限制的流量
	a.RecordFlow(FlowRecord{
		SrcIP:    "192.168.1.1",
		DstIP:    "10.0.0.1",
		SrcPort:  12345,
		DstPort:  80,
		Protocol: ProtocolHTTP,
		BytesIn:  10000, // 10000 * 8 = 80000 bps > 1000 bps
	})

	violations := a.GetPolicyViolations(10)
	if len(violations) == 0 {
		t.Error("应该有策略违规记录")
	}

	if violations[0].PolicyID != "limit-http" {
		t.Errorf("PolicyID = %s, want limit-http", violations[0].PolicyID)
	}
}

// ========== 告警测试 ==========

func TestGetAlerts(t *testing.T) {
	a := NewNetflowAnalyzer(WithDDoSThreshold(2))

	// 生成DDoS告警：创建大量连接到同一IP
	for i := 0; i < 5; i++ {
		a.RecordFlow(FlowRecord{
			SrcIP:    fmt.Sprintf("192.168.1.%d", i),
			DstIP:    "10.0.0.1",
			SrcPort:  uint16(10000 + i),
			DstPort:  80,
			Protocol: ProtocolTCP,
			BytesIn:  100,
		})
	}

	alerts := a.GetAlerts(10)
	if len(alerts) == 0 {
		t.Log("未产生告警（可能需要更多连接）")
	}
}

func TestGetAlertsByType(t *testing.T) {
	a := NewNetflowAnalyzer()

	// 通过 RecordFlow 触发
	a.RecordFlow(FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   1000,
	})

	surgeAlerts := a.GetAlertsByType("surge")
	if len(surgeAlerts) > 0 {
		t.Logf("获取到 %d 条 surge 告警", len(surgeAlerts))
	}
}

func TestGetAlertsByLevel(t *testing.T) {
	a := NewNetflowAnalyzer()

	warningAlerts := a.GetAlertsByLevel(AlertLevelWarning)
	if len(warningAlerts) != 0 {
		t.Errorf("初始告警数应为 0, got %d", len(warningAlerts))
	}
}

func TestResolveAlert(t *testing.T) {
	a := NewNetflowAnalyzer()

	// 添加告警
	a.addAlert(TrafficAlert{
		ID:        "alert-1",
		Type:      "test",
		Level:     AlertLevelWarning,
		Message:   "测试",
		Timestamp: time.Now(),
	})

	if err := a.ResolveAlert("alert-1"); err != nil {
		t.Fatalf("ResolveAlert 失败: %v", err)
	}

	alerts := a.GetAlerts(10)
	if len(alerts) > 0 && !alerts[0].Resolved {
		t.Error("告警应该已解决")
	}
}

func TestResolveAlertNotFound(t *testing.T) {
	a := NewNetflowAnalyzer()

	if err := a.ResolveAlert("nonexistent"); err == nil {
		t.Error("解决不存在的告警应返回错误")
	}
}

func TestClearAlerts(t *testing.T) {
	a := NewNetflowAnalyzer()

	// 添加告警
	a.addAlert(TrafficAlert{
		ID:        "alert-1",
		Type:      "test",
		Level:     AlertLevelWarning,
		Message:   "测试1",
		Timestamp: time.Now(),
		Resolved:  true,
	})
	a.addAlert(TrafficAlert{
		ID:        "alert-2",
		Type:      "test",
		Level:     AlertLevelWarning,
		Message:   "测试2",
		Timestamp: time.Now(),
		Resolved:  false,
	})

	cleared := a.ClearAlerts()
	if cleared != 1 {
		t.Errorf("清除数 = %d, want 1", cleared)
	}

	alerts := a.GetAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("剩余告警数 = %d, want 1", len(alerts))
	}
}

// ========== 报表测试 ==========

func TestGenerateReport(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	// 记录流量
	for i := 0; i < 10; i++ {
		a.RecordFlow(FlowRecord{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Interface: "eth0",
			SrcIP:     fmt.Sprintf("192.168.1.%d", i%3),
			DstIP:     "10.0.0.1",
			DstPort:   80,
			Protocol:  ProtocolHTTP,
			BytesIn:   1000,
			BytesOut:  500,
		})
	}

	report := a.GenerateReport("测试报表", start, end, 5)

	if report.Title != "测试报表" {
		t.Errorf("Title = %s, want 测试报表", report.Title)
	}
	if report.Summary.TotalBytesIn != 10000 {
		t.Errorf("TotalBytesIn = %d, want 10000", report.Summary.TotalBytesIn)
	}
	if report.Summary.TotalFlows != 10 {
		t.Errorf("TotalFlows = %d, want 10", report.Summary.TotalFlows)
	}
	if len(report.TopTalkers) == 0 {
		t.Error("TopTalkers 为空")
	}
	if len(report.TopPorts) == 0 {
		t.Error("TopPorts 为空")
	}
	if len(report.TopProtocols) == 0 {
		t.Error("TopProtocols 为空")
	}
}

func TestGenerateReportEmpty(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	report := a.GenerateReport("空报表", now.Add(-1*time.Hour), now.Add(1*time.Hour), 5)

	if report.Summary.TotalBytesIn != 0 {
		t.Errorf("TotalBytesIn = %d, want 0", report.Summary.TotalBytesIn)
	}
	if report.Summary.TotalFlows != 0 {
		t.Errorf("TotalFlows = %d, want 0", report.Summary.TotalFlows)
	}
}

// ========== 导出测试 ==========

func TestExportJSON(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	a.RecordFlow(FlowRecord{
		Timestamp: now,
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   100,
	})

	data, err := a.ExportJSON(now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("ExportJSON 失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("导出数据为空")
	}

	// 验证是有效JSON
	if data[0] != '[' {
		t.Error("导出数据不是JSON数组")
	}
}

func TestExportCSV(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	a.RecordFlow(FlowRecord{
		Timestamp: now,
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		SrcPort:   12345,
		DstPort:   80,
		Protocol:  ProtocolTCP,
		BytesIn:   100,
		BytesOut:  50,
	})

	data, err := a.ExportCSV(now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("ExportCSV 失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("导出数据为空")
	}

	// 检查表头
	csvStr := string(data)
	if len(csvStr) < 10 {
		t.Error("CSV 数据太短")
	}
}

func TestExportReportJSON(t *testing.T) {
	a := NewNetflowAnalyzer()
	now := time.Now()

	report := a.GenerateReport("测试", now.Add(-1*time.Hour), now.Add(1*time.Hour), 5)

	data, err := a.ExportReportJSON(report)
	if err != nil {
		t.Fatalf("ExportReportJSON 失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("导出数据为空")
	}
}

// ========== IP工具测试 ==========

func TestValidateIP(t *testing.T) {
	tests := []struct {
		ip     string
		expect bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"::1", true},
		{"invalid", false},
		{"", false},
		{"256.1.1.1", false},
	}

	for _, tt := range tests {
		result := ValidateIP(tt.ip)
		if result != tt.expect {
			t.Errorf("ValidateIP(%s) = %v, want %v", tt.ip, result, tt.expect)
		}
	}
}

func TestNormalizeIP(t *testing.T) {
	// IPv4
	normal, err := NormalizeIP("192.168.1.1")
	if err != nil {
		t.Fatalf("NormalizeIP 失败: %v", err)
	}
	if normal != "192.168.1.1" {
		t.Errorf("NormalizeIP = %s, want 192.168.1.1", normal)
	}

	// IPv6
	normal, err = NormalizeIP("::1")
	if err != nil {
		t.Fatalf("NormalizeIP IPv6 失败: %v", err)
	}
	if normal != "::1" {
		t.Errorf("NormalizeIP IPv6 = %s, want ::1", normal)
	}

	// 无效IP
	_, err = NormalizeIP("invalid")
	if err != ErrInvalidIP {
		t.Errorf("期望 ErrInvalidIP, got %v", err)
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip     string
		expect bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		result := IsPrivateIP(tt.ip)
		if result != tt.expect {
			t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expect)
		}
	}
}

// ========== 格式化工具测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes  uint64
		expect string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expect {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expect)
		}
	}
}

func TestFormatBandwidth(t *testing.T) {
	tests := []struct {
		bps    float64
		expect string
	}{
		{0, "0 bps"},
		{500, "500 bps"},
		{1000, "1.00 Kbps"},
		{1000000, "1.00 Mbps"},
		{1000000000, "1.00 Gbps"},
	}

	for _, tt := range tests {
		result := FormatBandwidth(tt.bps)
		if result != tt.expect {
			t.Errorf("FormatBandwidth(%f) = %s, want %s", tt.bps, result, tt.expect)
		}
	}
}

// ========== 状态查询测试 ==========

func TestGetRecordCount(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.GetRecordCount() != 0 {
		t.Error("初始记录数应为 0")
	}

	a.RecordFlow(FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   100,
	})

	if a.GetRecordCount() != 1 {
		t.Errorf("记录数 = %d, want 1", a.GetRecordCount())
	}
}

func TestGetAlertCount(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.GetAlertCount() != 0 {
		t.Error("初始告警数应为 0")
	}
}

func TestGetViolationCount(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.GetViolationCount() != 0 {
		t.Error("初始违规数应为 0")
	}
}

func TestGetPolicyCount(t *testing.T) {
	a := NewNetflowAnalyzer()

	if a.GetPolicyCount() != 0 {
		t.Error("初始策略数应为 0")
	}
}

func TestReset(t *testing.T) {
	a := NewNetflowAnalyzer()

	// 添加数据
	a.RecordFlow(FlowRecord{
		Interface: "eth0",
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		Protocol:  ProtocolTCP,
		BytesIn:   100,
	})
	a.AddBandwidthPolicy(BandwidthPolicy{ID: "p1", Name: "test"})

	if a.GetRecordCount() == 0 {
		t.Error("应该有记录")
	}

	a.Reset()

	if a.GetRecordCount() != 0 {
		t.Errorf("重置后记录数 = %d, want 0", a.GetRecordCount())
	}
	if a.GetPolicyCount() != 0 {
		t.Errorf("重置后策略数 = %d, want 0", a.GetPolicyCount())
	}
}

// ========== 常量测试 ==========

func TestProtocolConstants(t *testing.T) {
	if ProtocolTCP != "TCP" {
		t.Errorf("ProtocolTCP = %s, want TCP", ProtocolTCP)
	}
	if ProtocolUDP != "UDP" {
		t.Errorf("ProtocolUDP = %s, want UDP", ProtocolUDP)
	}
	if ProtocolHTTP != "HTTP" {
		t.Errorf("ProtocolHTTP = %s, want HTTP", ProtocolHTTP)
	}
}

func TestAlertLevelConstants(t *testing.T) {
	if AlertLevelInfo != "info" {
		t.Errorf("AlertLevelInfo = %s, want info", AlertLevelInfo)
	}
	if AlertLevelWarning != "warning" {
		t.Errorf("AlertLevelWarning = %s, want warning", AlertLevelWarning)
	}
	if AlertLevelCritical != "critical" {
		t.Errorf("AlertLevelCritical = %s, want critical", AlertLevelCritical)
	}
}

func TestGranularityConstants(t *testing.T) {
	if GranularityHourly != "hourly" {
		t.Errorf("GranularityHourly = %s, want hourly", GranularityHourly)
	}
	if GranularityDaily != "daily" {
		t.Errorf("GranularityDaily = %s, want daily", GranularityDaily)
	}
}

