// Package cost - Dashboard服务测试
package cost

import (
	"context"
	"testing"
	"time"
)

func TestDashboardService_CalculateStorageCost(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB: 0.69,
	}

	service := NewDashboardService(config)

	resource := ResourceInfo{
		Name:              "pool-main",
		Type:              "zfs-pool",
		TotalCapacityBytes: 10 * 1024 * 1024 * 1024 * 1024, // 10TB
		UsedCapacityBytes:  5 * 1024 * 1024 * 1024 * 1024,  // 5TB = 5120GB
	}

	cost := service.CalculateStorageCost(resource)

	expectedCost := 5120.0 * 0.69 // 3532.8
	if cost.Amount != round(expectedCost, 2) {
		t.Errorf("expected storage cost %.2f, got %.2f", expectedCost, cost.Amount)
	}

	if cost.Type != CostTypeStorage {
		t.Errorf("expected cost type storage, got %s", cost.Type)
	}
}

func TestDashboardService_CalculateElectricityCost(t *testing.T) {
	config := DashboardConfig{
		ElectricityCostPerKWh: 0.5,
		DefaultDevicePowerWatts: 100.0,
	}

	service := NewDashboardService(config)

	resource := ResourceInfo{
		Name:        "server-01",
		PowerWatts:  200.0, // 200W
	}

	cost := service.CalculateElectricityCost(resource)

	// 200W = 0.2kW, 月耗电 = 0.2 * 24 * 30 = 144kWh
	// 月电费 = 144 * 0.5 = 72元
	expectedCost := 0.2 * 24 * 30 * 0.5
	if cost.Amount != round(expectedCost, 2) {
		t.Errorf("expected electricity cost %.2f, got %.2f", expectedCost, cost.Amount)
	}
}

func TestDashboardService_GenerateCostSummary(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB:       0.69,
		ElectricityCostPerKWh:  0.5,
		DefaultDevicePowerWatts: 100.0,
		OpsCostMonthly:         200.0,
		HardwareCost:           10000.0,
		DepreciationYears:      5,
		BudgetLimitMonthly:     5000.0,
		LowUsageThreshold:      30.0,
		HighUsageThreshold:     80.0,
	}

	service := NewDashboardService(config)

	resources := []ResourceInfo{
		{
			Name:              "pool-ssd",
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024, // 1TB
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,      // 500GB
			PowerWatts:        50.0,
		},
		{
			Name:              "pool-hdd",
			TotalCapacityBytes: 10 * 1024 * 1024 * 1024 * 1024, // 10TB
			UsedCapacityBytes:  2 * 1024 * 1024 * 1024 * 1024,  // 2TB
			PowerWatts:        150.0,
		},
	}

	summary := service.GenerateCostSummary(resources)

	if summary.ResourceCount != 2 {
		t.Errorf("expected 2 resources, got %d", summary.ResourceCount)
	}

	// 检查成本类型汇总
	if summary.CostByType[CostTypeStorage] <= 0 {
		t.Error("expected storage cost in summary")
	}

	if summary.CostByType[CostTypeElectricity] <= 0 {
		t.Error("expected electricity cost in summary")
	}

	// 检查年成本计算
	if summary.TotalCostYearly != round(summary.TotalCostMonthly*12, 2) {
		t.Errorf("yearly cost calculation mismatch")
	}

	// 检查预算使用率
	if summary.BudgetUsagePercent <= 0 {
		t.Error("expected budget usage percent")
	}
}

func TestDashboardService_TrendAnalysis(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB: 0.69,
		TrendRetentionDays: 30,
	}

	service := NewDashboardService(config)

	resources := []ResourceInfo{
		{
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
		},
	}

	// 记录多个趋势点
	for i := 0; i < 5; i++ {
		service.RecordTrendPoint(resources)
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	}

	timeRange := TimeRange{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Granularity: "hour",
	}

	ctx := context.Background()
	result, err := service.AnalyzeTrend(ctx, resources, timeRange)
	if err != nil {
		t.Errorf("AnalyzeTrend failed: %v", err)
	}

	if len(result.DataPoints) < 5 {
		t.Errorf("expected at least 5 data points, got %d", len(result.DataPoints))
	}

	// 检查统计计算
	if result.Statistics.AvgCost <= 0 {
		t.Error("expected average cost")
	}

	// 检查预测
	if result.Forecast.ModelType != "linear" {
		t.Errorf("expected linear model, got %s", result.Forecast.ModelType)
	}
}

func TestDashboardService_RecordTrendPoint(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB: 0.69,
		TrendRetentionDays: 7,
	}

	service := NewDashboardService(config)

	resources := []ResourceInfo{
		{
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
		},
	}

	// 记录趋势点
	service.RecordTrendPoint(resources)

	trendData := service.GetTrendData()
	if len(trendData) != 1 {
		t.Errorf("expected 1 trend point, got %d", len(trendData))
	}

	point := trendData[0]
	if point.TotalCost <= 0 {
		t.Error("expected total cost in trend point")
	}

	if point.Trend != "stable" {
		t.Errorf("first point should be stable, got %s", point.Trend)
	}

	// 记录第二个点，成本增加
	resources[0].UsedCapacityBytes = 800 * 1024 * 1024 * 1024
	service.RecordTrendPoint(resources)

	trendData = service.GetTrendData()
	if len(trendData) != 2 {
		t.Errorf("expected 2 trend points, got %d", len(trendData))
	}

	// 第二个点应该是上升趋势
	lastPoint := trendData[len(trendData)-1]
	if lastPoint.Trend != "up" {
		t.Errorf("expected up trend after cost increase, got %s", lastPoint.Trend)
	}
}

func TestDashboardService_CalculateEfficiencyScore(t *testing.T) {
	config := DashboardConfig{
		LowUsageThreshold:  30.0,
		HighUsageThreshold: 80.0,
		BudgetLimitMonthly: 1000.0,
	}

	service := NewDashboardService(config)

	// 测试正常使用率 (50%)
	resources := []ResourceInfo{
		{
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
		},
	}

	summary := service.GenerateCostSummary(resources)
	if summary.EfficiencyScore < 80 {
		t.Errorf("expected high efficiency for normal usage, got %.1f", summary.EfficiencyScore)
	}

	// 测试低使用率 (10%) - 应该扣分但不是特别低
	resources[0].UsedCapacityBytes = 100 * 1024 * 1024 * 1024
	summary = service.GenerateCostSummary(resources)
	if summary.EfficiencyScore > 95 {
		t.Errorf("expected some penalty for low usage, got %.1f", summary.EfficiencyScore)
	}

	// 测试高使用率 (95%) - 应该扣分
	resources[0].UsedCapacityBytes = 950 * 1024 * 1024 * 1024
	summary = service.GenerateCostSummary(resources)
	if summary.EfficiencyScore > 95 {
		t.Errorf("expected penalty for high usage risk, got %.1f", summary.EfficiencyScore)
	}
}

func TestDashboardService_PotentialSavings(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB: 0.69,
		LowUsageThreshold: 30.0,
	}

	service := NewDashboardService(config)

	// 低使用率资源应该有节省空间
	resources := []ResourceInfo{
		{
			TotalCapacityBytes: 10 * 1024 * 1024 * 1024 * 1024, // 10TB
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,      // 500GB (5% 使用率)
		},
	}

	summary := service.GenerateCostSummary(resources)

	if summary.PotentialSavings <= 0 {
		t.Error("expected potential savings for low usage resource")
	}
}

func TestDashboardService_CleanupOldData(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB: 0.69,
		TrendRetentionDays: 1, // 只保留1天
	}

	service := NewDashboardService(config)

	resources := []ResourceInfo{
		{
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
		},
	}

	// 记录10个点
	for i := 0; i < 10; i++ {
		service.RecordTrendPoint(resources)
	}

	trendData := service.GetTrendData()
	// 所有点应该都在保留范围内
	if len(trendData) != 10 {
		t.Errorf("expected 10 trend points within retention, got %d", len(trendData))
	}
}

func TestCostItem_PercentCalculation(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB:      0.69,
		ElectricityCostPerKWh: 0.5,
		DefaultDevicePowerWatts: 100.0,
	}

	service := NewDashboardService(config)

	resources := []ResourceInfo{
		{
			Name:              "pool-1",
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
			PowerWatts:        100.0,
		},
		{
			Name:              "pool-2",
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  500 * 1024 * 1024 * 1024,
			PowerWatts:        100.0,
		},
	}

	summary := service.GenerateCostSummary(resources)

	// 检查成本项占比计算
	var totalPercent float64
	for _, item := range summary.CostItems {
		totalPercent += item.Percent
	}

	// 占比总和应该接近100%（考虑精度）
	if totalPercent < 99.9 || totalPercent > 100.1 {
		t.Errorf("expected percent sum ~100%%, got %.2f%%", totalPercent)
	}
}

func TestDashboardService_GetConfig(t *testing.T) {
	config := DashboardConfig{
		StorageCostPerGB: 1.0,
		ElectricityCostPerKWh: 0.6,
	}

	service := NewDashboardService(config)

	// 更新配置
	newConfig := DashboardConfig{
		StorageCostPerGB: 2.0,
	}
	service.UpdateConfig(newConfig)

	retrieved := service.GetConfig()
	if retrieved.StorageCostPerGB != 2.0 {
		t.Errorf("expected updated config, got %.2f", retrieved.StorageCostPerGB)
	}
}