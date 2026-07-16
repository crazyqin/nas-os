package tcoshield

import "testing"

func TestEmptySignal(t *testing.T) {
	recs := Analyze(Signal{})
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations for empty signal, got %d", len(recs))
	}
}

func TestWarrantyUpgrade(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:   5000,
		YearsInService:    6,
		HasWarranty:       false,
		TotalCapacityTB:    10,
		UsedCapacityTB:    8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "warranty-upgrade" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected warranty-upgrade recommendation")
	}
}

func TestEnergyOptimization(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:     4000,
		PowerCostPerYearUSD: 800, // 20% of hardware > 15%
		YearsInService:      3,
		HasWarranty:         true,
		TotalCapacityTB:     10,
		UsedCapacityTB:      8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "energy-optimization" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected energy-optimization recommendation")
	}
}

func TestCapacityUnderutilized(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:   5000,
		YearsInService:    3,
		HasWarranty:       true,
		TotalCapacityTB:   100,
		UsedCapacityTB:    20, // 20% < 30%
	})
	found := false
	for _, r := range recs {
		if r.ID == "capacity-underutilized" {
			found = true
			if r.Priority != "low" {
				t.Errorf("expected priority low, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected capacity-underutilized recommendation")
	}
}

func TestAutomationOps(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:           5000,
		MaintenanceCostPerYearUSD: 1000,
		StaffHoursPerWeek:         10,
		StaffHourlyRateUSD:        50, // 10*50*52 = 26000 > 1000
		YearsInService:            3,
		HasWarranty:               true,
		TotalCapacityTB:           10,
		UsedCapacityTB:            8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "automation-ops" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected automation-ops recommendation")
	}
}

func TestCloudComparison(t *testing.T) {
	// Build a scenario with high on-prem TCO
	recs := Analyze(Signal{
		HardwareCostUSD:               10000,
		SoftwareCostUSD:               2000,
		PowerCostPerYearUSD:           600, // 6% of 10000, not >15%
		MaintenanceCostPerYearUSD:     3000,
		YearsInService:                3,
		HasWarranty:                   true,
		TotalCapacityTB:               10,
		UsedCapacityTB:                8,
		StaffHoursPerWeek:             5,
		StaffHourlyRateUSD:            40, // 5*40*52 = 10400 > 3000 → triggers automation
		CloudEquivalentCostPerYearUSD: 5000, // annualTCO = 10000/3 + 2000 + 600 + 0 + 3000 + 0 + 0 + 10400 = 19333.33; 5000 < 0.7*19333 = 13533
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-comparison" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected cloud-comparison recommendation")
	}
}

func TestCoolingOptimization(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:         5000,
		PowerCostPerYearUSD:     1000,
		CoolingCostPerYearUSD:   600, // > 50% of 1000
		YearsInService:          3,
		HasWarranty:             true,
		TotalCapacityTB:         10,
		UsedCapacityTB:          8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cooling-optimization" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected cooling-optimization recommendation")
	}
}

func TestAvailabilityImprovement(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:           5000,
		MaintenanceCostPerYearUSD: 1000,
		DowntimeCostPerYearUSD:    5000, // > 1000
		YearsInService:            3,
		HasWarranty:              true,
		TotalCapacityTB:          10,
		UsedCapacityTB:           8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "availability-improvement" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected priority critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected availability-improvement recommendation")
	}
}

func TestReplacementPlanning(t *testing.T) {
	recs := Analyze(Signal{
		HardwareCostUSD:   5000,
		YearsInService:    4, // > 3
		HasWarranty:       true,
		WarrantyYearsLeft: 0, // < 1
		TotalCapacityTB:   10,
		UsedCapacityTB:    8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "replacement-planning" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected replacement-planning recommendation")
	}
}

func TestPriorityOrdering(t *testing.T) {
	// Trigger multiple recommendations with different priorities
	recs := Analyze(Signal{
		HardwareCostUSD:               10000,
		SoftwareCostUSD:               0,
		PowerCostPerYearUSD:           2000, // > 15% of 10000 → medium (energy)
		CoolingCostPerYearUSD:         1500, // > 50% of 2000 → medium (cooling)
		MaintenanceCostPerYearUSD:     1000,
		DowntimeCostPerYearUSD:        5000, // > 1000 → critical (availability)
		YearsInService:                6,    // > 5 && !HasWarranty → high (warranty)
		HasWarranty:                   false,
		TotalCapacityTB:               100,
		UsedCapacityTB:                20, // 20% < 30% → low (capacity)
		StaffHoursPerWeek:             10,
		StaffHourlyRateUSD:            50, // 10*50*52 = 26000 > 1000 → medium (automation)
		CloudEquivalentCostPerYearUSD: 0,
		WarrantyYearsLeft:             0,
	})

	if len(recs) < 2 {
		t.Fatalf("expected multiple recommendations, got %d", len(recs))
	}

	for i := 1; i < len(recs); i++ {
		if priorityRank(recs[i-1].Priority) > priorityRank(recs[i].Priority) {
			t.Errorf("recommendations not sorted by priority: %s (rank %d) before %s (rank %d)",
				recs[i-1].Priority, priorityRank(recs[i-1].Priority),
				recs[i].Priority, priorityRank(recs[i].Priority))
		}
	}
}