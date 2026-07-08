package costforecast

import (
	"testing"
)

func TestAnalyze_CapacitySoon(t *testing.T) {
	s := Signal{
		TotalCapacityGB:   1000,
		UsedGB:            900,
		GrowthRateGBPerMo: 50,
		CostPerGBPerMo:    0.01,
	}
	recs, fc := Analyze(s)
	if fc.MonthsToFull > 3 {
		t.Errorf("expected months to full <= 3, got %d", fc.MonthsToFull)
	}
	found := false
	for _, r := range recs {
		if r.ID == "cost-capacity-soon" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cost-capacity-soon recommendation")
	}
}

func TestAnalyze_BudgetOverrun(t *testing.T) {
	s := Signal{
		TotalCapacityGB:   10000,
		UsedGB:            5000,
		GrowthRateGBPerMo: 200,
		CostPerGBPerMo:    0.02,
		BudgetMonthly:     50,
	}
	recs, _ := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "cost-budget-overrun" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cost-budget-overrun recommendation")
	}
}

func TestAnalyze_Tiering(t *testing.T) {
	s := Signal{
		TotalCapacityGB:  2000,
		UsedGB:           1200,
		HasTiering:       false,
		CostPerGBPerMo:   0.01,
		ColdTierCostPerGB: 0.005,
	}
	recs, _ := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "cost-enable-tiering" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cost-enable-tiering recommendation")
	}
}

func TestAnalyze_Dedup(t *testing.T) {
	s := Signal{
		TotalCapacityGB: 5000,
		UsedGB:          2000,
		DedupSavingsGB:  0,
		CostPerGBPerMo:  0.01,
	}
	recs, _ := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "cost-dedup" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cost-dedup recommendation")
	}
}

func TestAnalyze_Compress(t *testing.T) {
	s := Signal{
		TotalCapacityGB:  1000,
		UsedGB:           600,
		CompressSavingsGB: 0,
		CostPerGBPerMo:   0.01,
	}
	recs, _ := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "cost-compress" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cost-compress recommendation")
	}
}

func TestAnalyze_CloudOptimize(t *testing.T) {
	s := Signal{
		TotalCapacityGB:  10000,
		UsedGB:          3000,
		CostPerGBPerMo:  0.01,
		CloudBuckets:    3,
		CloudCostPerMo:  200,
	}
	recs, _ := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "cost-cloud-optimize" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cost-cloud-optimize recommendation")
	}
}

func TestForecast_Recommendation(t *testing.T) {
	s := Signal{
		TotalCapacityGB:   10000,
		UsedGB:            1000,
		GrowthRateGBPerMo: 50,
		CostPerGBPerMo:    0.01,
	}
	_, fc := Analyze(s)
	if fc.MonthsToFull <= 6 {
		t.Error("expected months to full > 6")
	}
	if fc.Recommendation != "Storage growth healthy; maintain current strategy" {
		t.Errorf("unexpected recommendation: %s", fc.Recommendation)
	}
}
