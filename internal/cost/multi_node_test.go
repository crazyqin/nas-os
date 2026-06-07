// Package cost - 多节点成本聚合测试
package cost

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMultiNodeAggregator_RegisterNode(t *testing.T) {
	config := DefaultMultiNodeConfig()
	agg := NewMultiNodeAggregator(config, nil)

	node := NodeInfo{
		ID:            "node-001",
		Name:          "test-node",
		Address:       "192.168.1.100",
		Role:          "master",
		Status:        "online",
		Region:        "beijing",
		LastHeartbeat: time.Now(),
	}

	agg.RegisterNode(node)

	nodes := agg.GetAllNodes()
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}

	if nodes[0].ID != "node-001" {
		t.Errorf("expected node ID node-001, got %s", nodes[0].ID)
	}
}

func TestMultiNodeAggregator_UnregisterNode(t *testing.T) {
	config := DefaultMultiNodeConfig()
	agg := NewMultiNodeAggregator(config, nil)

	node := NodeInfo{
		ID:            "node-001",
		Name:          "test-node",
		LastHeartbeat: time.Now(),
	}

	agg.RegisterNode(node)
	agg.UnregisterNode("node-001")

	nodes := agg.GetAllNodes()
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after unregister, got %d", len(nodes))
	}
}

func TestMultiNodeAggregator_UpdateNodeStats(t *testing.T) {
	config := DefaultMultiNodeConfig()
	agg := NewMultiNodeAggregator(config, nil)

	node := NodeInfo{
		ID:            "node-001",
		Name:          "test-node",
		LastHeartbeat: time.Now(),
	}
	agg.RegisterNode(node)

	summary := CostSummary{
		TotalCostMonthly: 500.0,
		CostByType: map[CostType]float64{
			CostTypeStorage:     300.0,
			CostTypeElectricity: 100.0,
			CostTypeNetwork:     50.0,
			CostTypeOperations:  50.0,
		},
	}

	resources := []ResourceCostDetail{
		{
			Name:               "pool-main",
			Type:               "pool",
			TotalCapacityBytes: 10 * 1024 * 1024 * 1024 * 1024, // 10TB
			UsedCapacityBytes:  5 * 1024 * 1024 * 1024 * 1024,  // 5TB
			MonthlyCost:        300.0,
		},
	}

	usage := NodeUsageStats{
		CPUUsagePercent:     45.0,
		MemoryUsagePercent:  60.0,
		StorageUsagePercent: 50.0,
	}

	err := agg.UpdateNodeStats("node-001", summary, resources, usage)
	if err != nil {
		t.Errorf("UpdateNodeStats failed: %v", err)
	}

	stats, err := agg.GetNodeStats("node-001")
	if err != nil {
		t.Errorf("GetNodeStats failed: %v", err)
	}

	if stats.Summary.TotalCostMonthly != 500.0 {
		t.Errorf("expected cost 500.0, got %.2f", stats.Summary.TotalCostMonthly)
	}

	if len(stats.RecentTrend) != 1 {
		t.Errorf("expected 1 trend point, got %d", len(stats.RecentTrend))
	}
}

func TestMultiNodeAggregator_GetClusterSummary(t *testing.T) {
	config := DefaultMultiNodeConfig()
	config.MonthlyBudget = 1000.0
	agg := NewMultiNodeAggregator(config, nil)

	// 注册多个节点
	for i := 1; i <= 3; i++ {
		node := NodeInfo{
			ID:            fmt.Sprintf("node-%03d", i),
			Name:          fmt.Sprintf("node%d", i),
			LastHeartbeat: time.Now(),
		}
		agg.RegisterNode(node)

		summary := CostSummary{
			TotalCostMonthly: 300.0,
		}
		resources := []ResourceCostDetail{
			{
				TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
				UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
			},
		}
		usage := NodeUsageStats{StorageUsagePercent: 50.0}

		agg.UpdateNodeStats(fmt.Sprintf("node-%03d", i), summary, resources, usage)
	}

	summary := agg.GetClusterSummary()

	if summary.TotalNodes != 3 {
		t.Errorf("expected 3 nodes, got %d", summary.TotalNodes)
	}

	if summary.TotalCostMonthly != 900.0 {
		t.Errorf("expected total cost 900.0, got %.2f", summary.TotalCostMonthly)
	}

	if summary.BudgetStatus.UsagePercent != 90.0 {
		t.Errorf("expected budget usage 90%%, got %.2f%%", summary.BudgetStatus.UsagePercent)
	}
}

func TestMultiNodeAggregator_GenerateClusterReport(t *testing.T) {
	config := DefaultMultiNodeConfig()
	agg := NewMultiNodeAggregator(config, nil)

	node := NodeInfo{
		ID:            "node-001",
		Name:          "test-node",
		Region:        "shanghai",
		LastHeartbeat: time.Now(),
	}
	agg.RegisterNode(node)

	summary := CostSummary{
		TotalCostMonthly: 500.0,
		CostByType: map[CostType]float64{
			CostTypeStorage: 300.0,
		},
	}
	resources := []ResourceCostDetail{
		{
			TotalCapacityBytes: 2 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  1 * 1024 * 1024 * 1024 * 1024,
		},
	}
	usage := NodeUsageStats{StorageUsagePercent: 50.0}

	agg.UpdateNodeStats("node-001", summary, resources, usage)

	timeRange := TimeRange{
		StartTime: time.Now().Add(-24 * time.Hour),
		EndTime:   time.Now(),
	}

	ctx := context.Background()
	report, err := agg.GenerateClusterReport(ctx, timeRange)
	if err != nil {
		t.Errorf("GenerateClusterReport failed: %v", err)
	}

	if report.ClusterSummary.TotalNodes != 1 {
		t.Errorf("expected 1 node in report, got %d", report.ClusterSummary.TotalNodes)
	}

	if report.CostByRegion["shanghai"] != 500.0 {
		t.Errorf("expected region cost 500.0, got %.2f", report.CostByRegion["shanghai"])
	}
}

func TestMultiNodeAggregator_GetTopCostNodes(t *testing.T) {
	config := DefaultMultiNodeConfig()
	agg := NewMultiNodeAggregator(config, nil)

	// 注册多个节点，不同成本
	costs := []float64{100.0, 500.0, 300.0, 800.0}
	for i, cost := range costs {
		nodeID := fmt.Sprintf("node-%03d", i+1)
		node := NodeInfo{
			ID:            nodeID,
			Name:          fmt.Sprintf("node%d", i+1),
			LastHeartbeat: time.Now(),
		}
		agg.RegisterNode(node)

		summary := CostSummary{TotalCostMonthly: cost}
		agg.UpdateNodeStats(nodeID, summary, nil, NodeUsageStats{})
	}

	topNodes := agg.GetTopCostNodes(2)

	if len(topNodes) != 2 {
		t.Errorf("expected 2 top nodes, got %d", len(topNodes))
	}

	// 最高成本应该是 node-004 (800.0)
	if topNodes[0].Summary.TotalCostMonthly != 800.0 {
		t.Errorf("expected highest cost 800.0, got %.2f", topNodes[0].Summary.TotalCostMonthly)
	}
}

func TestMultiNodeAggregator_GetNodesByRegion(t *testing.T) {
	config := DefaultMultiNodeConfig()
	agg := NewMultiNodeAggregator(config, nil)

	regions := []string{"beijing", "shanghai", "beijing"}
	for i, region := range regions {
		nodeID := fmt.Sprintf("node-%03d", i+1)
		node := NodeInfo{
			ID:            nodeID,
			Name:          fmt.Sprintf("node%d", i+1),
			Region:        region,
			LastHeartbeat: time.Now(),
		}
		agg.RegisterNode(node)
	}

	beijingNodes := agg.GetNodesByRegion("beijing")

	if len(beijingNodes) != 2 {
		t.Errorf("expected 2 beijing nodes, got %d", len(beijingNodes))
	}
}

func TestBudgetStatus(t *testing.T) {
	config := DefaultMultiNodeConfig()
	config.MonthlyBudget = 1000.0
	config.WarningThreshold = 70.0
	config.CriticalThreshold = 90.0
	agg := NewMultiNodeAggregator(config, nil)

	node := NodeInfo{
		ID:            "node-001",
		LastHeartbeat: time.Now(),
	}
	agg.RegisterNode(node)

	// 测试正常状态 - 成本低于70%
	summary := CostSummary{TotalCostMonthly: 500.0}
	agg.UpdateNodeStats("node-001", summary, nil, NodeUsageStats{})

	clusterSummary := agg.GetClusterSummary()
	if clusterSummary.BudgetStatus.Status != "normal" {
		t.Errorf("expected normal status at 50%% budget, got %s", clusterSummary.BudgetStatus.Status)
	}

	// 测试警告状态 - 成本在70%-90%
	summary.TotalCostMonthly = 750.0
	agg.UpdateNodeStats("node-001", summary, nil, NodeUsageStats{})

	clusterSummary = agg.GetClusterSummary()
	if clusterSummary.BudgetStatus.Status != "warning" {
		t.Errorf("expected warning status at 75%% budget, got %s", clusterSummary.BudgetStatus.Status)
	}

	// 测试严重状态 - 成本超过90%
	summary.TotalCostMonthly = 950.0
	agg.UpdateNodeStats("node-001", summary, nil, NodeUsageStats{})

	clusterSummary = agg.GetClusterSummary()
	if clusterSummary.BudgetStatus.Status != "critical" {
		t.Errorf("expected critical status at 95%% budget, got %s", clusterSummary.BudgetStatus.Status)
	}
}

func TestOptimizationSuggestions(t *testing.T) {
	config := DefaultMultiNodeConfig()
	config.LowUsageThreshold = 30.0
	config.HighUsageThreshold = 80.0
	agg := NewMultiNodeAggregator(config, nil)

	// 低使用率节点
	node1 := NodeInfo{ID: "node-001", Name: "low-usage", LastHeartbeat: time.Now()}
	agg.RegisterNode(node1)
	agg.UpdateNodeStats("node-001", CostSummary{TotalCostMonthly: 100.0}, nil, NodeUsageStats{StorageUsagePercent: 20.0})

	// 高使用率节点
	node2 := NodeInfo{ID: "node-002", Name: "high-usage", LastHeartbeat: time.Now()}
	agg.RegisterNode(node2)
	agg.UpdateNodeStats("node-002", CostSummary{TotalCostMonthly: 200.0}, nil, NodeUsageStats{StorageUsagePercent: 90.0})

	timeRange := TimeRange{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}

	ctx := context.Background()
	report, _ := agg.GenerateClusterReport(ctx, timeRange)

	// 检查建议类型
	foundScaleDown := false
	foundScaleUp := false
	for _, s := range report.OptimizationSuggestions {
		if s.Type == "scale_down" {
			foundScaleDown = true
		}
		if s.Type == "scale_up" {
			foundScaleUp = true
		}
	}

	if !foundScaleDown {
		t.Error("expected scale_down suggestion for low usage node")
	}
	if !foundScaleUp {
		t.Error("expected scale_up suggestion for high usage node")
	}
}
