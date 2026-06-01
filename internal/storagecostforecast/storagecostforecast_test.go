package storagecostforecast

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	engine := New()
	if engine == nil {
		t.Fatal("New() returned nil")
	}
}

func TestSetBudgetLimit(t *testing.T) {
	engine := New()
	engine.SetBudgetLimit(1000.0)
	if engine.budgetLimit != 1000.0 {
		t.Errorf("expected budget limit 1000, got %f", engine.budgetLimit)
	}
}

func TestSetPredictionMonths(t *testing.T) {
	engine := New()
	engine.SetPredictionMonths(24)
	if engine.predictionMonths != 24 {
		t.Errorf("expected 24 months, got %d", engine.predictionMonths)
	}
}

func TestAddRecord(t *testing.T) {
	engine := New()
	record := CostRecord{
		Timestamp:   time.Now(),
		Provider:    ProviderAWS,
		Tier:        TierHot,
		StorageGB:   100,
		MonthlyCost: 2.3,
	}
	engine.AddRecord(record)
	if len(engine.records) != 1 {
		t.Errorf("expected 1 record, got %d", len(engine.records))
	}
}

func TestAddRecords(t *testing.T) {
	engine := New()
	records := []CostRecord{
		{Timestamp: time.Now(), Provider: ProviderAWS, Tier: TierHot, StorageGB: 100, MonthlyCost: 2.3},
		{Timestamp: time.Now(), Provider: ProviderAzure, Tier: TierHot, StorageGB: 50, MonthlyCost: 0.9},
	}
	engine.AddRecords(records)
	if len(engine.records) != 2 {
		t.Errorf("expected 2 records, got %d", len(engine.records))
	}
}

func TestSetAlertCallback(t *testing.T) {
	engine := New()
	var mu sync.Mutex
	alerts := make([]BudgetAlert, 0)

	engine.SetAlertCallback(func(alert BudgetAlert) {
		mu.Lock()
		alerts = append(alerts, alert)
		mu.Unlock()
	})

	// Verify callback is set
	if engine.alertCallback == nil {
		t.Error("alert callback not set")
	}
}

func TestStartStop(t *testing.T) {
	engine := New()
	engine.Start()
	time.Sleep(10 * time.Millisecond)
	engine.Stop()
	// Should not panic
}

func TestDefaultPrices(t *testing.T) {
	prices := initDefaultPrices()
	if len(prices) == 0 {
		t.Error("default prices should not be empty")
	}
	if _, ok := prices[ProviderAWS]; !ok {
		t.Error("AWS prices should be defined")
	}
}

func TestProviderConstants(t *testing.T) {
	providers := []CloudProvider{ProviderAWS, ProviderAzure, ProviderGCP, ProviderAlibaba, ProviderLocal}
	for _, p := range providers {
		if string(p) == "" {
			t.Error("provider should not be empty string")
		}
	}
}

func TestTierConstants(t *testing.T) {
	tiers := []StorageTier{TierHot, TierWarm, TierCold, TierArchive}
	for _, tier := range tiers {
		if string(tier) == "" {
			t.Error("tier should not be empty string")
		}
	}
}
