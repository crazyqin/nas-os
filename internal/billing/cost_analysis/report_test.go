package cost_analysis

import (
	"testing"
	"time"
)

// Mock implementations for testing
type mockBillingProvider struct {
	storagePrice float64
	bandwidthPrice float64
	stats *BillingStats
}

func (m *mockBillingProvider) GetUsageRecords(userID, poolID string, start, end time.Time) ([]*UsageRecord, error) {
	return []*UsageRecord{}, nil
}

func (m *mockBillingProvider) GetUserUsageSummary(userID string, start, end time.Time) (*UsageSummary, error) {
	return &UsageSummary{}, nil
}

func (m *mockBillingProvider) GetBillingStats(start, end time.Time) (*BillingStats, error) {
	if m.stats != nil {
		return m.stats, nil
	}
	return &BillingStats{
		TotalStorageUsedGB: 1000,
		TotalBandwidthGB:   500,
		TotalRevenue:       150,
		StorageRevenue:     100,
		BandwidthRevenue:   50,
	}, nil
}

func (m *mockBillingProvider) GetStoragePrice(poolID string) float64 {
	return m.storagePrice
}

func (m *mockBillingProvider) GetBandwidthPrice() float64 {
	return m.bandwidthPrice
}

type mockQuotaProvider struct {
	usages []*QuotaUsageInfo
}

func (m *mockQuotaProvider) GetAllUsage() ([]*QuotaUsageInfo, error) {
	if m.usages != nil {
		return m.usages, nil
	}
	return []*QuotaUsageInfo{
		{
			QuotaID:      "quota-1",
			TargetID:     "user1",
			TargetName:   "User One",
			VolumeName:   "pool1",
			HardLimit:    100 * 1024 * 1024 * 1024, // 100GB
			UsedBytes:    80 * 1024 * 1024 * 1024,  // 80GB
			UsagePercent: 80,
		},
		{
			QuotaID:      "quota-2",
			TargetID:     "user2",
			TargetName:   "User Two",
			VolumeName:   "pool1",
			HardLimit:    200 * 1024 * 1024 * 1024, // 200GB
			UsedBytes:    50 * 1024 * 1024 * 1024,  // 50GB
			UsagePercent: 25,
		},
	}, nil
}

func (m *mockQuotaProvider) GetUserUsage(username string) ([]*QuotaUsageInfo, error) {
	return m.usages, nil
}

func (m *mockQuotaProvider) GetPoolUsage(poolID string) (*QuotaUsageInfo, error) {
	return nil, nil
}

func TestNewCostAnalysisEngine(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1", billing, quota, config)
	if engine == nil {
		t.Fatal("Expected engine to be created")
	}

	if engine.config.DefaultCurrency != "CNY" {
		t.Errorf("Expected default currency CNY, got %s", engine.config.DefaultCurrency)
	}
}

func TestGenerateStorageTrendReport(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	report, err := engine.GenerateStorageTrendReport(30)
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	if report.Type != CostReportStorageTrend {
		t.Errorf("Expected report type %s, got %s", CostReportStorageTrend, report.Type)
	}

	if report.Summary.Currency != "CNY" {
		t.Errorf("Expected currency CNY, got %s", report.Summary.Currency)
	}
}

func TestGenerateResourceUtilizationReport(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	report, err := engine.GenerateResourceUtilizationReport()
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	if report.Type != CostReportResourceUtil {
		t.Errorf("Expected report type %s, got %s", CostReportResourceUtil, report.Type)
	}
}

func TestGenerateOptimizationReport(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    5 * 1024 * 1024 * 1024, // 5GB - low utilization
				UsagePercent: 5,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024, // 95GB - high utilization
				UsagePercent: 95,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	report, err := engine.GenerateOptimizationReport()
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	if report.Type != CostReportOptimization {
		t.Errorf("Expected report type %s, got %s", CostReportOptimization, report.Type)
	}
}

func TestBudgetManagement(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	// Create budget
	budgetConfig := BudgetConfig{
		Name:        "Monthly Budget",
		TotalBudget: 1000,
		Period:      "monthly",
		StartDate:   time.Now().AddDate(0, -1, 0),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	}

	budget, err := engine.CreateBudget(budgetConfig)
	if err != nil {
		t.Fatalf("Failed to create budget: %v", err)
	}

	if budget.ID == "" {
		t.Error("Expected budget ID to be generated")
	}

	// List budgets
	budgets := engine.ListBudgets()
	if len(budgets) != 1 {
		t.Errorf("Expected 1 budget, got %d", len(budgets))
	}

	// Get budget
	retrieved, err := engine.GetBudget(budget.ID)
	if err != nil {
		t.Fatalf("Failed to get budget: %v", err)
	}

	if retrieved.Name != "Monthly Budget" {
		t.Errorf("Expected budget name 'Monthly Budget', got %s", retrieved.Name)
	}

	// Update budget
	budgetConfig.TotalBudget = 1500
	updated, err := engine.UpdateBudget(budget.ID, budgetConfig)
	if err != nil {
		t.Fatalf("Failed to update budget: %v", err)
	}

	if updated.TotalBudget != 1500 {
		t.Errorf("Expected total budget 1500, got %f", updated.TotalBudget)
	}

	// Delete budget
	err = engine.DeleteBudget(budget.ID)
	if err != nil {
		t.Fatalf("Failed to delete budget: %v", err)
	}

	budgets = engine.ListBudgets()
	if len(budgets) != 0 {
		t.Errorf("Expected 0 budgets after delete, got %d", len(budgets))
	}
}

func TestGenerateBudgetTrackingReport(t *testing.T) {
	billing := &mockBillingProvider{
		storagePrice: 0.1,
		stats: &BillingStats{
			StorageRevenue:   100,
			BandwidthRevenue: 50,
		},
	}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	// Create budget
	budgetConfig := BudgetConfig{
		Name:        "Test Budget",
		TotalBudget: 500,
		Period:      "monthly",
		StartDate:   time.Now().AddDate(0, 0, -15),
		EndDate:     time.Now().AddDate(0, 0, 15),
		Enabled:     true,
		AlertThresholds: []float64{50, 75, 90, 100},
	}

	budget, _ := engine.CreateBudget(budgetConfig)

	report, err := engine.GenerateBudgetTrackingReport(budget.ID)
	if err != nil {
		t.Fatalf("Failed to generate budget tracking report: %v", err)
	}

	// Check budget tracking details
	if details, ok := report.Details.(*BudgetTrackingReport); ok {
		if details.BudgetID != budget.ID {
			t.Errorf("Expected budget ID %s, got %s", budget.ID, details.BudgetID)
		}

		if details.TotalBudget != 500 {
			t.Errorf("Expected total budget 500, got %f", details.TotalBudget)
		}
	} else {
		t.Error("Report details should be BudgetTrackingReport type")
	}
}

func TestGenerateComprehensiveReport(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	report, err := engine.GenerateComprehensiveReport()
	if err != nil {
		t.Fatalf("Failed to generate comprehensive report: %v", err)
	}

	if report.Type != CostReportComprehensive {
		t.Errorf("Expected report type %s, got %s", CostReportComprehensive, report.Type)
	}
}

func TestCostTrendRecording(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	// Record trend data
	trend := CostTrend{
		Date:          time.Now(),
		StorageCost:   100,
		BandwidthCost: 50,
		TotalCost:     150,
		StorageUsedGB: 1000,
		BandwidthGB:   500,
	}

	engine.RecordTrendData(trend)

	if len(engine.trendData) != 1 {
		t.Errorf("Expected 1 trend data point, got %d", len(engine.trendData))
	}
}

func TestCostAlerts(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	// Add an alert
	alert := CostAlert{
		ID:        "alert-1",
		Type:      "test",
		Severity:  "warning",
		Message:   "Test alert",
		Value:     100,
		Threshold: 80,
		CreatedAt: time.Now(),
	}
	engine.alerts = append(engine.alerts, &alert)

	alerts := engine.GetAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}

	// Acknowledge alert
	err := engine.AcknowledgeAlert("alert-1")
	if err != nil {
		t.Fatalf("Failed to acknowledge alert: %v", err)
	}

	alerts = engine.GetAlerts()
	if !alerts[0].Acknowledged {
		t.Error("Expected alert to be acknowledged")
	}
}

func TestCalculateCostEfficiency(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine("/tmp/test-cost-analysis-1773546342", billing, quota, config)

	tests := []struct {
		usagePercent float64
		expected     float64
	}{
		{70, 1.0},   // Ideal range
		{60, 1.0},   // Lower bound of ideal
		{80, 1.0},   // Upper bound of ideal
		{90, 0.6},   // Above ideal
		{50, 0.833}, // Below ideal
		{30, 0.5},   // Low utilization
	}

	for _, tt := range tests {
		efficiency := engine.calculateCostEfficiency(tt.usagePercent)
		if efficiency < tt.expected-0.1 || efficiency > tt.expected+0.1 {
			t.Errorf("Usage %.0f%%: expected efficiency ~%.2f, got %.2f",
				tt.usagePercent, tt.expected, efficiency)
		}
	}
}

func TestCostSummary(t *testing.T) {
	summary := CostSummary{
		TotalCost:          1000,
		StorageCost:        800,
		BandwidthCost:      200,
		Currency:           "CNY",
		CostChangePercent:  10.5,
		AvgDailyCost:       33.33,
		ProjectedMonthlyCost: 1100,
		BudgetUtilization:  75,
	}

	if summary.TotalCost != 1000 {
		t.Errorf("Expected total cost 1000, got %f", summary.TotalCost)
	}

	if summary.StorageCost+summary.BandwidthCost != summary.TotalCost {
		t.Error("Storage + Bandwidth should equal Total")
	}
}

func TestCostRecommendation(t *testing.T) {
	rec := CostRecommendation{
		ID:               "rec-1",
		Type:             "storage",
		Priority:         "high",
		Title:            "Optimize storage",
		Description:      "Reduce unused storage",
		PotentialSavings: 100,
		Impact:           "Cost reduction",
		Action:           "Clean up unused files",
		Implemented:      false,
	}

	if rec.PotentialSavings != 100 {
		t.Errorf("Expected potential savings 100, got %f", rec.PotentialSavings)
	}

	if rec.Implemented {
		t.Error("Recommendation should not be implemented by default")
	}
}

func TestBudgetTrackingReportStatus(t *testing.T) {
	tests := []struct {
		utilization       float64
		projectedOverrun  float64
		expectedStatus    string
	}{
		{50, 0, "on_track"},
		{90, 100, "at_risk"},
		{110, 100, "over_budget"},
	}

	for _, tt := range tests {
		tracking := BudgetTrackingReport{
			Utilization:        tt.utilization,
			ProjectedOverrun:   tt.projectedOverrun,
		}

		// Determine status based on values
		if tracking.Utilization >= 100 {
			tracking.Status = "over_budget"
		} else if tracking.ProjectedOverrun > 0 {
			tracking.Status = "at_risk"
		} else {
			tracking.Status = "on_track"
		}

		if tracking.Status != tt.expectedStatus {
			t.Errorf("Utilization %.0f%%, Overrun %.0f: expected status %s, got %s",
				tt.utilization, tt.projectedOverrun, tt.expectedStatus, tracking.Status)
		}
	}
}