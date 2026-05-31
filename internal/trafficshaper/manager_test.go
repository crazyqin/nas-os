package trafficshaper

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.config == nil {
		t.Fatal("config is nil")
	}
	if manager.rules == nil {
		t.Fatal("rules map is nil")
	}
	if manager.classes == nil {
		t.Fatal("classes map is nil")
	}
	if manager.stats == nil {
		t.Fatal("stats map is nil")
	}
	if manager.events == nil {
		t.Fatal("events slice is nil")
	}
}

func TestCreateRule(t *testing.T) {
	manager := NewManager()

	rule := &TrafficRule{
		Name:                "test-rule",
		Direction:           DirectionInbound,
		Priority:            5,
		Protocol:            ProtocolTCP,
		MaxBandwidth:        1000000,
		GuaranteedBandwidth: 500000,
		BurstSize:           2000000,
		Action:              ActionShape,
	}

	result, err := manager.CreateRule(rule)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}

	if result.ID == "" {
		t.Fatal("rule ID is empty")
	}
	if result.Name != "test-rule" {
		t.Fatalf("expected name 'test-rule', got '%s'", result.Name)
	}
	if result.Direction != DirectionInbound {
		t.Fatalf("expected direction '%s', got '%s'", DirectionInbound, result.Direction)
	}
	if result.Priority != 5 {
		t.Fatalf("expected priority 5, got %d", result.Priority)
	}
	if result.Protocol != ProtocolTCP {
		t.Fatalf("expected protocol '%s', got '%s'", ProtocolTCP, result.Protocol)
	}
	if result.MaxBandwidth != 1000000 {
		t.Fatalf("expected max bandwidth 1000000, got %d", result.MaxBandwidth)
	}
	if result.Action != ActionShape {
		t.Fatalf("expected action '%s', got '%s'", ActionShape, result.Action)
	}
	if !result.Enabled {
		t.Fatal("expected rule to be enabled")
	}
	if result.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
}

func TestCreateRuleInvalidDirection(t *testing.T) {
	manager := NewManager()

	rule := &TrafficRule{
		Name:      "test-rule",
		Direction: "invalid",
		Priority:  5,
		Action:    ActionShape,
	}

	_, err := manager.CreateRule(rule)
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}
}

func TestCreateRuleInvalidPriority(t *testing.T) {
	manager := NewManager()

	rule := &TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  11,
		Action:    ActionShape,
	}

	_, err := manager.CreateRule(rule)
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
}

func TestCreateRuleInvalidAction(t *testing.T) {
	manager := NewManager()

	rule := &TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    "invalid",
	}

	_, err := manager.CreateRule(rule)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestCreateRuleInvalidProtocol(t *testing.T) {
	manager := NewManager()

	rule := &TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  5,
		Protocol:  "invalid",
		Action:    ActionShape,
	}

	_, err := manager.CreateRule(rule)
	if err == nil {
		t.Fatal("expected error for invalid protocol")
	}
}

func TestListRules(t *testing.T) {
	manager := NewManager()

	// 初始状态应该为空
	rules := manager.ListRules()
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}

	// 创建两个规则
	manager.CreateRule(&TrafficRule{
		Name:      "rule-1",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    ActionShape,
	})
	manager.CreateRule(&TrafficRule{
		Name:      "rule-2",
		Direction: DirectionOutbound,
		Priority:  3,
		Action:    ActionBlock,
	})

	rules = manager.ListRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestUpdateRule(t *testing.T) {
	manager := NewManager()

	rule, _ := manager.CreateRule(&TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    ActionShape,
	})

	updated, err := manager.UpdateRule(rule.ID, &TrafficRule{
		Name:       "updated-rule",
		Priority:   8,
		MaxBandwidth: 2000000,
	})
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}

	if updated.Name != "updated-rule" {
		t.Fatalf("expected name 'updated-rule', got '%s'", updated.Name)
	}
	if updated.Priority != 8 {
		t.Fatalf("expected priority 8, got %d", updated.Priority)
	}
	if updated.MaxBandwidth != 2000000 {
		t.Fatalf("expected max bandwidth 2000000, got %d", updated.MaxBandwidth)
	}
}

func TestUpdateRuleNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.UpdateRule("nonexistent", &TrafficRule{
		Name: "test",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}

func TestDeleteRule(t *testing.T) {
	manager := NewManager()

	rule, _ := manager.CreateRule(&TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    ActionShape,
	})

	err := manager.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	// 验证规则已删除
	rules := manager.ListRules()
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after deletion, got %d", len(rules))
	}

	// 验证统计也已删除
	_, err = manager.GetRuleStats(rule.ID)
	if err == nil {
		t.Fatal("expected error for deleted rule stats")
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.DeleteRule("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}

func TestToggleRule(t *testing.T) {
	manager := NewManager()

	rule, _ := manager.CreateRule(&TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    ActionShape,
	})

	// 初始状态应该是启用的
	if !rule.Enabled {
		t.Fatal("expected rule to be initially enabled")
	}

	// 切换为禁用
	toggled, err := manager.ToggleRule(rule.ID)
	if err != nil {
		t.Fatalf("ToggleRule failed: %v", err)
	}
	if toggled.Enabled {
		t.Fatal("expected rule to be disabled after toggle")
	}

	// 再次切换为启用
	toggled, err = manager.ToggleRule(rule.ID)
	if err != nil {
		t.Fatalf("ToggleRule failed: %v", err)
	}
	if !toggled.Enabled {
		t.Fatal("expected rule to be enabled after second toggle")
	}
}

func TestToggleRuleNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.ToggleRule("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}

func TestCreateClass(t *testing.T) {
	manager := NewManager()

	class := &TrafficClass{
		Name:                "video",
		Priority:            8,
		MaxBandwidth:        50000000,
		GuaranteedBandwidth: 20000000,
		Description:         "Video streaming traffic",
	}

	result, err := manager.CreateClass(class)
	if err != nil {
		t.Fatalf("CreateClass failed: %v", err)
	}

	if result.ID == "" {
		t.Fatal("class ID is empty")
	}
	if result.Name != "video" {
		t.Fatalf("expected name 'video', got '%s'", result.Name)
	}
	if result.Priority != 8 {
		t.Fatalf("expected priority 8, got %d", result.Priority)
	}
	if result.MaxBandwidth != 50000000 {
		t.Fatalf("expected max bandwidth 50000000, got %d", result.MaxBandwidth)
	}
	if result.GuaranteedBandwidth != 20000000 {
		t.Fatalf("expected guaranteed bandwidth 20000000, got %d", result.GuaranteedBandwidth)
	}
	if result.Description != "Video streaming traffic" {
		t.Fatalf("expected description 'Video streaming traffic', got '%s'", result.Description)
	}
}

func TestListClasses(t *testing.T) {
	manager := NewManager()

	// 初始状态应该为空
	classes := manager.ListClasses()
	if len(classes) != 0 {
		t.Fatalf("expected 0 classes, got %d", len(classes))
	}

	// 创建两个类别
	manager.CreateClass(&TrafficClass{
		Name:     "video",
		Priority: 8,
	})
	manager.CreateClass(&TrafficClass{
		Name:     "audio",
		Priority: 6,
	})

	classes = manager.ListClasses()
	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}
}

func TestUpdateClass(t *testing.T) {
	manager := NewManager()

	class, _ := manager.CreateClass(&TrafficClass{
		Name:         "video",
		Priority:     8,
		MaxBandwidth: 50000000,
		Description:  "Video streaming",
	})

	updated, err := manager.UpdateClass(class.ID, &TrafficClass{
		Name:         "video-hd",
		Priority:     9,
		MaxBandwidth: 100000000,
		Description:  "HD Video streaming",
	})
	if err != nil {
		t.Fatalf("UpdateClass failed: %v", err)
	}

	if updated.Name != "video-hd" {
		t.Fatalf("expected name 'video-hd', got '%s'", updated.Name)
	}
	if updated.Priority != 9 {
		t.Fatalf("expected priority 9, got %d", updated.Priority)
	}
	if updated.MaxBandwidth != 100000000 {
		t.Fatalf("expected max bandwidth 100000000, got %d", updated.MaxBandwidth)
	}
	if updated.Description != "HD Video streaming" {
		t.Fatalf("expected description 'HD Video streaming', got '%s'", updated.Description)
	}
}

func TestUpdateClassNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.UpdateClass("nonexistent", &TrafficClass{
		Name: "test",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent class")
	}
}

func TestDeleteClass(t *testing.T) {
	manager := NewManager()

	class, _ := manager.CreateClass(&TrafficClass{
		Name:     "video",
		Priority: 8,
	})

	err := manager.DeleteClass(class.ID)
	if err != nil {
		t.Fatalf("DeleteClass failed: %v", err)
	}

	// 验证类别已删除
	classes := manager.ListClasses()
	if len(classes) != 0 {
		t.Fatalf("expected 0 classes after deletion, got %d", len(classes))
	}
}

func TestDeleteClassNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.DeleteClass("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent class")
	}
}

func TestGetGlobalStats(t *testing.T) {
	manager := NewManager()

	// 创建两个规则
	manager.CreateRule(&TrafficRule{
		Name:      "rule-1",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    ActionShape,
	})
	manager.CreateRule(&TrafficRule{
		Name:      "rule-2",
		Direction: DirectionOutbound,
		Priority:  3,
		Action:    ActionBlock,
	})

	stats := manager.GetGlobalStats()
	if stats == nil {
		t.Fatal("GetGlobalStats returned nil")
	}
	if stats.RuleID != "global" {
		t.Fatalf("expected rule ID 'global', got '%s'", stats.RuleID)
	}
}

func TestGetRuleStats(t *testing.T) {
	manager := NewManager()

	rule, _ := manager.CreateRule(&TrafficRule{
		Name:      "test-rule",
		Direction: DirectionInbound,
		Priority:  5,
		Action:    ActionShape,
	})

	stats, err := manager.GetRuleStats(rule.ID)
	if err != nil {
		t.Fatalf("GetRuleStats failed: %v", err)
	}

	if stats.RuleID != rule.ID {
		t.Fatalf("expected rule ID '%s', got '%s'", rule.ID, stats.RuleID)
	}
}

func TestGetRuleStatsNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetRuleStats("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent rule stats")
	}
}

func TestGetAllocation(t *testing.T) {
	manager := NewManager()

	// 创建类别
	manager.CreateClass(&TrafficClass{
		Name:         "video",
		Priority:     8,
		MaxBandwidth: 50000000,
	})
	manager.CreateClass(&TrafficClass{
		Name:         "audio",
		Priority:     6,
		MaxBandwidth: 20000000,
	})

	allocation := manager.GetAllocation()
	if allocation == nil {
		t.Fatal("GetAllocation returned nil")
	}

	if allocation.TotalBandwidth != 1000000000 {
		t.Fatalf("expected total bandwidth 1000000000, got %d", allocation.TotalBandwidth)
	}
	if allocation.AllocatedBandwidth != 70000000 {
		t.Fatalf("expected allocated bandwidth 70000000, got %d", allocation.AllocatedBandwidth)
	}
	if allocation.FreeBandwidth != 930000000 {
		t.Fatalf("expected free bandwidth 930000000, got %d", allocation.FreeBandwidth)
	}
	if len(allocation.Classes) != 2 {
		t.Fatalf("expected 2 class allocations, got %d", len(allocation.Classes))
	}
}

func TestRebalance(t *testing.T) {
	manager := NewManager()

	// 创建两个类别
	manager.CreateClass(&TrafficClass{
		Name:     "video",
		Priority: 8,
	})
	manager.CreateClass(&TrafficClass{
		Name:     "audio",
		Priority: 2,
	})

	allocation := manager.Rebalance()
	if allocation == nil {
		t.Fatal("Rebalance returned nil")
	}

	// 验证带宽按优先级分配
	// 总优先级 = 8 + 2 = 10
	// video: 8/10 * 1000000000 = 800000000
	// audio: 2/10 * 1000000000 = 200000000

	classes := manager.ListClasses()
	for _, class := range classes {
		switch class.Name {
		case "video":
			if class.MaxBandwidth != 800000000 {
				t.Fatalf("expected video bandwidth 800000000, got %d", class.MaxBandwidth)
			}
		case "audio":
			if class.MaxBandwidth != 200000000 {
				t.Fatalf("expected audio bandwidth 200000000, got %d", class.MaxBandwidth)
			}
		}
	}
}

func TestGetEvents(t *testing.T) {
	manager := NewManager()

	// 初始状态应该为空
	events := manager.GetEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestSimulateTraffic(t *testing.T) {
	manager := NewManager()

	// 创建规则
	rule, _ := manager.CreateRule(&TrafficRule{
		Name:         "test-rule",
		Direction:    DirectionInbound,
		Priority:     5,
		Protocol:     ProtocolTCP,
		MaxBandwidth: 1000000,
		Action:       ActionShape,
	})

	// 模拟流量
	manager.SimulateTraffic()

	// 验证统计已更新
	stats, _ := manager.GetRuleStats(rule.ID)
	if stats.BytesIn == 0 && stats.BytesOut == 0 {
		t.Fatal("expected traffic data to be simulated")
	}
	if stats.PacketsIn == 0 && stats.PacketsOut == 0 {
		t.Fatal("expected packet data to be simulated")
	}

	// 验证事件已生成
	events := manager.GetEvents()
	if len(events) == 0 {
		t.Fatal("expected events to be generated")
	}
}

func TestGetConfig(t *testing.T) {
	manager := NewManager()

	config := manager.GetConfig()
	if config == nil {
		t.Fatal("GetConfig returned nil")
	}
	if config.TotalBandwidth != 1000000000 {
		t.Fatalf("expected total bandwidth 1000000000, got %d", config.TotalBandwidth)
	}
}

func TestUpdateConfig(t *testing.T) {
	manager := NewManager()

	newConfig := &TrafficShaperConfig{
		Enabled:         true,
		TotalBandwidth:  2000000000,
		DefaultMaxBps:   200000000,
		StatsInterval:   30,
		MaxEvents:       5000,
	}

	manager.UpdateConfig(newConfig)

	config := manager.GetConfig()
	if config.TotalBandwidth != 2000000000 {
		t.Fatalf("expected total bandwidth 2000000000, got %d", config.TotalBandwidth)
	}
	if config.DefaultMaxBps != 200000000 {
		t.Fatalf("expected default max bps 200000000, got %d", config.DefaultMaxBps)
	}
}

func TestDefaultTrafficShaperConfig(t *testing.T) {
	config := DefaultTrafficShaperConfig()
	if config == nil {
		t.Fatal("DefaultTrafficShaperConfig returned nil")
	}
	if config.TotalBandwidth != 1000000000 {
		t.Fatalf("expected total bandwidth 1000000000, got %d", config.TotalBandwidth)
	}
	if config.DefaultMaxBps != 100000000 {
		t.Fatalf("expected default max bps 100000000, got %d", config.DefaultMaxBps)
	}
	if config.StatsInterval != 60 {
		t.Fatalf("expected stats interval 60, got %d", config.StatsInterval)
	}
	if config.MaxEvents != 10000 {
		t.Fatalf("expected max events 10000, got %d", config.MaxEvents)
	}
}

func TestIsValidDirection(t *testing.T) {
	tests := []struct {
		direction TrafficDirection
		expected  bool
	}{
		{DirectionInbound, true},
		{DirectionOutbound, true},
		{DirectionBoth, true},
		{"invalid", false},
	}

	for _, test := range tests {
		result := IsValidDirection(test.direction)
		if result != test.expected {
			t.Fatalf("IsValidDirection(%s) = %v, expected %v", test.direction, result, test.expected)
		}
	}
}

func TestIsValidProtocol(t *testing.T) {
	tests := []struct {
		protocol Protocol
		expected bool
	}{
		{ProtocolTCP, true},
		{ProtocolUDP, true},
		{ProtocolAny, true},
		{"invalid", false},
	}

	for _, test := range tests {
		result := IsValidProtocol(test.protocol)
		if result != test.expected {
			t.Fatalf("IsValidProtocol(%s) = %v, expected %v", test.protocol, result, test.expected)
		}
	}
}

func TestIsValidAction(t *testing.T) {
	tests := []struct {
		action   TrafficAction
		expected bool
	}{
		{ActionShape, true},
		{ActionBlock, true},
		{ActionAllow, true},
		{"invalid", false},
	}

	for _, test := range tests {
		result := IsValidAction(test.action)
		if result != test.expected {
			t.Fatalf("IsValidAction(%s) = %v, expected %v", test.action, result, test.expected)
		}
	}
}

func TestIsValidPriority(t *testing.T) {
	tests := []struct {
		priority int
		expected bool
	}{
		{1, true},
		{5, true},
		{10, true},
		{0, false},
		{11, false},
		{-1, false},
	}

	for _, test := range tests {
		result := IsValidPriority(test.priority)
		if result != test.expected {
			t.Fatalf("IsValidPriority(%d) = %v, expected %v", test.priority, result, test.expected)
		}
	}
}
