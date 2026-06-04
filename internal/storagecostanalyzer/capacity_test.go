package storagecostanalyzer

import (
	"testing"
	"time"
)

func TestNewCapacityPlanner(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	planner := NewCapacityPlanner(manager)

	if planner == nil {
		t.Fatal("NewCapacityPlanner returned nil")
	}
	if planner.manager != manager {
		t.Error("Planner manager mismatch")
	}
}

func TestDefaultCapacityPlanningInput(t *testing.T) {
	input := DefaultCapacityPlanningInput(TierSSD, 12)

	if input.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, input.Tier)
	}
	if input.PlanningMonths != 12 {
		t.Errorf("Expected 12 months, got %d", input.PlanningMonths)
	}
	if input.GrowthModel != "linear" {
		t.Errorf("Expected growth model 'linear', got %s", input.GrowthModel)
	}
	if input.TargetUtilization != 70.0 {
		t.Errorf("Expected target utilization 70, got %f", input.TargetUtilization)
	}
	if input.ExpansionCostPerTB != 500.0 {
		t.Errorf("Expected expansion cost 500, got %f", input.ExpansionCostPerTB)
	}
	if !input.IncludeBuffer {
		t.Error("IncludeBuffer should be true")
	}
	if input.BufferPercent != 20.0 {
		t.Errorf("Expected buffer percent 20, got %f", input.BufferPercent)
	}
}

func TestGenerateCapacityPlan(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AlertThreshold: 80.0,
	}
	manager := NewManager(config)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	// 注册一个高利用率的层级
	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD存储",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         9,
	})

	planner := NewCapacityPlanner(manager)
	input := DefaultCapacityPlanningInput(TierSSD, 12)

	result, err := planner.GenerateCapacityPlan(input)
	if err != nil {
		t.Fatalf("GenerateCapacityPlan failed: %v", err)
	}

	if result == nil {
		t.Fatal("GenerateCapacityPlan returned nil")
	}

	// 验证基本字段
	if result.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, result.Tier)
	}
	if result.CurrentCapacityTB != 10.0 {
		t.Errorf("Expected capacity 10, got %f", result.CurrentCapacityTB)
	}
	if result.CurrentUsedTB != 9.0 {
		t.Errorf("Expected used 9, got %f", result.CurrentUsedTB)
	}
	if result.CurrentUtilization != 90.0 {
		t.Errorf("Expected utilization 90%%, got %f", result.CurrentUtilization)
	}

	// 高利用率应该触发紧急扩容
	if result.Urgency != "critical" {
		t.Errorf("Expected urgency 'critical' for 90%% utilization, got %s", result.Urgency)
	}
	if result.RecommendedAction != "expand" {
		t.Errorf("Expected action 'expand', got %s", result.RecommendedAction)
	}
	if result.RecommendedCapacityTB <= result.CurrentCapacityTB {
		t.Error("Recommended capacity should be greater than current")
	}
	if result.TotalExpansionCost <= 0 {
		t.Error("Expansion cost should be positive")
	}
	if len(result.Steps) == 0 {
		t.Error("Steps should not be empty")
	}
}

func TestGenerateCapacityPlanLowUtilization(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AlertThreshold: 80.0,
	}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	// 注册一个低利用率的层级
	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD存储",
		CostPerTBMonth: 30,
		CapacityTB:     100,
		UsedTB:         20,
	})

	planner := NewCapacityPlanner(manager)
	input := DefaultCapacityPlanningInput(TierHDD, 12)

	result, err := planner.GenerateCapacityPlan(input)
	if err != nil {
		t.Fatalf("GenerateCapacityPlan failed: %v", err)
	}

	// 低利用率应该触发优化建议
	if result.Urgency != "low" {
		t.Errorf("Expected urgency 'low' for 20%% utilization, got %s", result.Urgency)
	}
	if result.RecommendedAction != "optimize" {
		t.Errorf("Expected action 'optimize', got %s", result.RecommendedAction)
	}
	if result.RecommendedCapacityTB >= result.CurrentCapacityTB {
		t.Error("Recommended capacity should be less than current for optimization")
	}
}

func TestForecastCapacity(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     20,
		UsedTB:         10,
	})

	planner := NewCapacityPlanner(manager)
	input := DefaultCapacityPlanningInput(TierSSD, 12)

	result, err := planner.ForecastCapacity(input)
	if err != nil {
		t.Fatalf("ForecastCapacity failed: %v", err)
	}

	if result == nil {
		t.Fatal("ForecastCapacity returned nil")
	}

	if result.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, result.Tier)
	}
	if result.CurrentUsedTB != 10.0 {
		t.Errorf("Expected current used 10, got %f", result.CurrentUsedTB)
	}
	if result.CurrentCapacityTB != 20.0 {
		t.Errorf("Expected current capacity 20, got %f", result.CurrentCapacityTB)
	}
	if len(result.ProjectedUsage) != 12 {
		t.Errorf("Expected 12 projected points, got %d", len(result.ProjectedUsage))
	}

	// 验证预测值
	for i, point := range result.ProjectedUsage {
		if point.ProjectedUsedTB < result.CurrentUsedTB {
			t.Errorf("Month %d: projected used should be >= current", i)
		}
		if point.ProjectedUtilization < 0 || point.ProjectedUtilization > 100 {
			t.Errorf("Month %d: utilization should be 0-100%%, got %f", i, point.ProjectedUtilization)
		}
		if point.LowerBound > point.ProjectedUsedTB {
			t.Errorf("Month %d: lower bound should be <= projected", i)
		}
		if point.UpperBound < point.ProjectedUsedTB {
			t.Errorf("Month %d: upper bound should be >= projected", i)
		}
	}
}

func TestGenerateMultiTierPlan(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AlertThreshold: 80.0,
	}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         8,
	})

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 30,
		CapacityTB:     50,
		UsedTB:         30,
	})

	planner := NewCapacityPlanner(manager)

	result, err := planner.GenerateMultiTierPlan(12, 70.0)
	if err != nil {
		t.Fatalf("GenerateMultiTierPlan failed: %v", err)
	}

	if result == nil {
		t.Fatal("GenerateMultiTierPlan returned nil")
	}

	if len(result.TierPlans) != 2 {
		t.Errorf("Expected 2 tier plans, got %d", len(result.TierPlans))
	}
	if result.PlanningMonths != 12 {
		t.Errorf("Expected 12 months, got %d", result.PlanningMonths)
	}
	if result.TargetUtilization != 70.0 {
		t.Errorf("Expected target utilization 70, got %f", result.TargetUtilization)
	}
	if result.TotalCurrentCapacityTB != 60.0 {
		t.Errorf("Expected total capacity 60, got %f", result.TotalCurrentCapacityTB)
	}
	if result.TotalCurrentUsedTB != 38.0 {
		t.Errorf("Expected total used 38, got %f", result.TotalCurrentUsedTB)
	}
	if result.OverallUtilization <= 0 {
		t.Error("Overall utilization should be positive")
	}
}
