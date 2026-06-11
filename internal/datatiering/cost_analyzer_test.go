package datatiering

import (
	"testing"
)

func TestStorageCostAnalyzer_AddTier(t *testing.T) {
	ca := NewStorageCostAnalyzer()

	err := ca.AddTier(CostStorageTier{
		ID: "nvme", Name: "NVMe", Type: "nvme",
		CostPerGBMo: 0.20, TotalGB: 1000, UsedGB: 500,
		ReadMBps: 7000, WriteMBps: 5000, IOPS: 1000000, LatencyMs: 0.02,
	})
	if err != nil {
		t.Fatalf("AddTier failed: %v", err)
	}

	tiers := ca.GetTiers()
	if len(tiers) != 1 {
		t.Errorf("expected 1 tier, got %d", len(tiers))
	}
}

func TestStorageCostAnalyzer_GenerateReport(t *testing.T) {
	ca := NewStorageCostAnalyzer()

	ca.AddTier(CostStorageTier{
		ID: "nvme", Name: "NVMe", Type: "nvme",
		CostPerGBMo: 0.20, TotalGB: 1000, UsedGB: 500,
	})
	ca.AddTier(CostStorageTier{
		ID: "hdd", Name: "HDD", Type: "hdd",
		CostPerGBMo: 0.02, TotalGB: 8000, UsedGB: 4000,
	})

	report := ca.GenerateReport()

	if report.TotalUsedGB != 4500 {
		t.Errorf("expected 4500GB used, got %d", report.TotalUsedGB)
	}
	if report.TotalCostMo <= 0 {
		t.Error("total cost should be positive")
	}
	if report.AvgCostPerGB <= 0 {
		t.Error("avg cost per GB should be positive")
	}
}

func TestStorageCostAnalyzer_Recommendations(t *testing.T) {
	ca := NewStorageCostAnalyzer()

	// Low utilization NVMe - should recommend downsize
	ca.AddTier(CostStorageTier{
		ID: "nvme", Name: "NVMe", Type: "nvme",
		CostPerGBMo: 0.20, TotalGB: 10000, UsedGB: 1000,
	})
	// Cold data on expensive tier
	ca.AddTier(CostStorageTier{
		ID: "hdd", Name: "HDD", Type: "hdd",
		CostPerGBMo: 0.02, TotalGB: 10000, UsedGB: 5000,
	})

	report := ca.GenerateReport()

	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations for low utilization")
	}
	if report.PotentialSaving <= 0 {
		t.Error("potential saving should be positive")
	}
}

func TestStorageCostAnalyzer_UpdateUsage(t *testing.T) {
	ca := NewStorageCostAnalyzer()
	ca.AddTier(CostStorageTier{
		ID: "hdd", Name: "HDD", Type: "hdd",
		CostPerGBMo: 0.02, TotalGB: 8000, UsedGB: 4000,
	})

	err := ca.UpdateTierUsage("hdd", 6000)
	if err != nil {
		t.Fatalf("UpdateTierUsage failed: %v", err)
	}

	tiers := ca.GetTiers()
	for _, tier := range tiers {
		if tier.ID == "hdd" && tier.UsedGB != 6000 {
			t.Errorf("expected 6000GB, got %d", tier.UsedGB)
		}
	}
}

func TestStorageCostAnalyzer_ReportHistory(t *testing.T) {
	ca := NewStorageCostAnalyzer()
	ca.AddTier(CostStorageTier{ID: "hdd", Type: "hdd", CostPerGBMo: 0.02, TotalGB: 1000, UsedGB: 500})

	ca.GenerateReport()
	ca.GenerateReport()

	history := ca.GetReportHistory(10)
	if len(history) != 2 {
		t.Errorf("expected 2 reports, got %d", len(history))
	}
}

func TestStorageCostAnalyzer_EmptyTiers(t *testing.T) {
	ca := NewStorageCostAnalyzer()

	report := ca.GenerateReport()
	if report.TotalCostMo != 0 {
		t.Error("empty tiers should have zero cost")
	}
}
