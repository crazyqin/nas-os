package storagebudget

import (
	"testing"
)

func TestNewAdvisor(t *testing.T) {
	a := NewAdvisor()
	if a == nil {
		t.Fatal("NewAdvisor returned nil")
	}
}

func TestAllocate_BasicShares(t *testing.T) {
	a := NewAdvisor()
	opts := AllocateOptions{
		TotalCapacityGB: 1000,
		Shares: []ShareInfo{
			{Name: "media", CurrentUsageGB: 200, GrowthRate: 10, Priority: 3, Type: "media"},
			{Name: "backup", CurrentUsageGB: 150, GrowthRate: 5, Priority: 5, Type: "backup"},
			{Name: "archive", CurrentUsageGB: 50, GrowthRate: 1, Priority: 1, Type: "archive"},
		},
		BudgetTier: "standard",
	}

	plan, err := a.Allocate(opts)
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if len(plan.ShareAllocations) != 3 {
		t.Errorf("expected 3 share allocations, got %d", len(plan.ShareAllocations))
	}
	if plan.ReservedGB <= 0 {
		t.Error("reserved should be positive")
	}
	if plan.ReservedGB >= 1000 {
		t.Error("reserved should be less than total")
	}
	// backup should get more than archive due to higher priority
	if plan.ShareAllocations["backup"] <= plan.ShareAllocations["archive"] {
		t.Errorf("backup (%.2f) should get more than archive (%.2f)",
			plan.ShareAllocations["backup"], plan.ShareAllocations["archive"])
	}
}

func TestAllocate_WithUsers(t *testing.T) {
	a := NewAdvisor()
	opts := AllocateOptions{
		TotalCapacityGB: 500,
		Shares: []ShareInfo{
			{Name: "shared", CurrentUsageGB: 100, GrowthRate: 2, Priority: 3, Type: "work"},
		},
		Users: []UserInfo{
			{Name: "alice", UsageGB: 80, QuotaGB: 100},
			{Name: "bob", UsageGB: 40, QuotaGB: 50},
		},
		BudgetTier: "premium",
	}

	plan, err := a.Allocate(opts)
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	if len(plan.UserQuotas) != 2 {
		t.Errorf("expected 2 user quotas, got %d", len(plan.UserQuotas))
	}
	// alice uses more, should get more quota
	if plan.UserQuotas["alice"] <= plan.UserQuotas["bob"] {
		t.Errorf("alice (%.2f) should get more than bob (%.2f)",
			plan.UserQuotas["alice"], plan.UserQuotas["bob"])
	}
	// premium tier reserves 15%
	if plan.ReservedGB < 70 || plan.ReservedGB > 80 {
		t.Errorf("expected reserved ~75 GB for premium tier, got %.2f", plan.ReservedGB)
	}
}

func TestAllocate_Errors(t *testing.T) {
	a := NewAdvisor()

	// zero capacity
	_, err := a.Allocate(AllocateOptions{TotalCapacityGB: 0})
	if err == nil {
		t.Error("expected error for zero capacity")
	}

	// no shares or users
	_, err = a.Allocate(AllocateOptions{TotalCapacityGB: 100})
	if err == nil {
		t.Error("expected error for no shares/users")
	}
}

func TestAllocate_EconomyTier(t *testing.T) {
	a := NewAdvisor()
	opts := AllocateOptions{
		TotalCapacityGB: 1000,
		Shares: []ShareInfo{
			{Name: "work", CurrentUsageGB: 100, GrowthRate: 5, Priority: 3, Type: "work"},
		},
		BudgetTier: "economy",
	}
	plan, err := a.Allocate(opts)
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	// economy tier reserves 5%
	if plan.ReservedGB < 45 || plan.ReservedGB > 55 {
		t.Errorf("expected reserved ~50 GB for economy tier, got %.2f", plan.ReservedGB)
	}
}

func TestPredictGrowth_Growing(t *testing.T) {
	a := NewAdvisor()
	// Simulate growing usage over 6 months
	now := int64(1700000000)
	history := []UsagePoint{
		{Timestamp: now, UsageGB: 100},
		{Timestamp: now + 30*24*3600, UsageGB: 120},
		{Timestamp: now + 60*24*3600, UsageGB: 140},
		{Timestamp: now + 90*24*3600, UsageGB: 160},
		{Timestamp: now + 120*24*3600, UsageGB: 180},
		{Timestamp: now + 150*24*3600, UsageGB: 200},
	}

	pred, err := a.PredictGrowth(history)
	if err != nil {
		t.Fatalf("PredictGrowth failed: %v", err)
	}
	if pred.Trend != "growing" {
		t.Errorf("expected growing trend, got %s", pred.Trend)
	}
	if pred.MonthsUntilFull <= 0 {
		t.Error("expected positive months until full")
	}
	if pred.PredictedFullDate == "" {
		t.Error("expected non-empty predicted full date")
	}
	if pred.RecommendedAction == "" {
		t.Error("expected non-empty recommended action")
	}
}

func TestPredictGrowth_Stable(t *testing.T) {
	a := NewAdvisor()
	now := int64(1700000000)
	history := []UsagePoint{
		{Timestamp: now, UsageGB: 100},
		{Timestamp: now + 30*24*3600, UsageGB: 100},
		{Timestamp: now + 60*24*3600, UsageGB: 100},
	}

	pred, err := a.PredictGrowth(history)
	if err != nil {
		t.Fatalf("PredictGrowth failed: %v", err)
	}
	if pred.Trend != "stable" {
		t.Errorf("expected stable trend, got %s", pred.Trend)
	}
}

func TestPredictGrowth_Shrinking(t *testing.T) {
	a := NewAdvisor()
	now := int64(1700000000)
	history := []UsagePoint{
		{Timestamp: now, UsageGB: 200},
		{Timestamp: now + 30*24*3600, UsageGB: 180},
		{Timestamp: now + 60*24*3600, UsageGB: 150},
		{Timestamp: now + 90*24*3600, UsageGB: 120},
	}

	pred, err := a.PredictGrowth(history)
	if err != nil {
		t.Fatalf("PredictGrowth failed: %v", err)
	}
	if pred.Trend != "shrinking" {
		t.Errorf("expected shrinking trend, got %s", pred.Trend)
	}
}

func TestPredictGrowth_Errors(t *testing.T) {
	a := NewAdvisor()
	// too few points
	_, err := a.PredictGrowth([]UsagePoint{{Timestamp: 1, UsageGB: 50}})
	if err == nil {
		t.Error("expected error for insufficient data points")
	}
}

func TestDetectMisallocation_Overallocated(t *testing.T) {
	a := NewAdvisor()
	current := []ShareUsage{
		{Name: "big_share", UsageGB: 10, AllocatedGB: 200, UtilizationPercent: 5},
		{Name: "small_share", UsageGB: 90, AllocatedGB: 100, UtilizationPercent: 90},
	}

	report, err := a.DetectMisallocation(current)
	if err != nil {
		t.Fatalf("DetectMisallocation failed: %v", err)
	}
	if len(report.Overallocated) != 1 {
		t.Errorf("expected 1 overallocated, got %d", len(report.Overallocated))
	}
	if report.Overallocated[0] != "big_share" {
		t.Errorf("expected big_share to be overallocated, got %s", report.Overallocated[0])
	}
	if report.TotalWastedGB <= 0 {
		t.Error("expected positive wasted GB")
	}
}

func TestDetectMisallocation_Underallocated(t *testing.T) {
	a := NewAdvisor()
	current := []ShareUsage{
		{Name: "full_share", UsageGB: 95, AllocatedGB: 100, UtilizationPercent: 95},
		{Name: "ok_share", UsageGB: 50, AllocatedGB: 100, UtilizationPercent: 50},
	}

	report, err := a.DetectMisallocation(current)
	if err != nil {
		t.Fatalf("DetectMisallocation failed: %v", err)
	}
	if len(report.Underallocated) != 1 {
		t.Errorf("expected 1 underallocated, got %d", len(report.Underallocated))
	}
	if report.Underallocated[0] != "full_share" {
		t.Errorf("expected full_share to be underallocated, got %s", report.Underallocated[0])
	}
	// should suggest more allocation
	suggested, ok := report.ReallocationSuggestions["full_share"]
	if !ok {
		t.Error("expected reallocation suggestion for full_share")
	}
	if suggested <= 95 {
		t.Errorf("suggested allocation %.2f should exceed usage 95", suggested)
	}
}

func TestDetectMisallocation_Empty(t *testing.T) {
	a := NewAdvisor()
	_, err := a.DetectMisallocation([]ShareUsage{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestOptimizeBudget_Basic(t *testing.T) {
	a := NewAdvisor()
	constraints := BudgetConstraints{
		TotalBudget: 2000,
		Priorities: map[string]int{
			"shares":   5,
			"backups":  3,
			"archive":  1,
		},
		MinReservedGB: 200,
	}

	result, err := a.OptimizeBudget(constraints)
	if err != nil {
		t.Fatalf("OptimizeBudget failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.RecommendedAllocations) != 3 {
		t.Errorf("expected 3 allocations, got %d", len(result.RecommendedAllocations))
	}
	// shares has highest priority, should get most
	if result.RecommendedAllocations["shares"] <= result.RecommendedAllocations["archive"] {
		t.Error("shares should get more than archive")
	}
	if result.RiskLevel == "" {
		t.Error("risk level should not be empty")
	}
	if len(result.Tradeoffs) == 0 {
		t.Error("expected non-empty tradeoffs")
	}
}

func TestOptimizeBudget_EqualPriorities(t *testing.T) {
	a := NewAdvisor()
	constraints := BudgetConstraints{
		TotalBudget: 900,
		Priorities: map[string]int{
			"a": 2,
			"b": 2,
			"c": 2,
		},
		MinReservedGB: 0,
	}

	result, err := a.OptimizeBudget(constraints)
	if err != nil {
		t.Fatalf("OptimizeBudget failed: %v", err)
	}
	allocA := result.RecommendedAllocations["a"]
	allocB := result.RecommendedAllocations["b"]
	allocC := result.RecommendedAllocations["c"]
	// with equal priorities, allocations should be roughly equal
	if absDiff(allocA, allocB) > 1 || absDiff(allocB, allocC) > 1 {
		t.Errorf("equal priorities should yield equal allocations: a=%.2f b=%.2f c=%.2f", allocA, allocB, allocC)
	}
}

func TestOptimizeBudget_Errors(t *testing.T) {
	a := NewAdvisor()
	// zero budget
	_, err := a.OptimizeBudget(BudgetConstraints{TotalBudget: 0})
	if err == nil {
		t.Error("expected error for zero budget")
	}
	// reserved exceeds budget
	_, err = a.OptimizeBudget(BudgetConstraints{TotalBudget: 100, MinReservedGB: 200})
	if err == nil {
		t.Error("expected error when reserved exceeds budget")
	}
}

func TestOptimizeBudget_RiskLevels(t *testing.T) {
	a := NewAdvisor()
	// all high priority -> high risk
	highRisk := BudgetConstraints{
		TotalBudget: 1000,
		Priorities: map[string]int{
			"a": 5,
			"b": 5,
			"c": 5,
		},
	}
	result, _ := a.OptimizeBudget(highRisk)
	if result.RiskLevel != "high" {
		t.Errorf("expected high risk, got %s", result.RiskLevel)
	}

	// all low priority -> low risk
	lowRisk := BudgetConstraints{
		TotalBudget: 1000,
		Priorities: map[string]int{
			"a": 1,
			"b": 1,
		},
	}
	result, _ = a.OptimizeBudget(lowRisk)
	if result.RiskLevel != "low" {
		t.Errorf("expected low risk, got %s", result.RiskLevel)
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}