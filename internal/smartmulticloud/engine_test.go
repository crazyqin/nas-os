package smartmulticloud

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestAddAccount(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	account := &CloudAccount{
		ID:       "acc-1",
		Provider: ProviderAWS,
		Name:     "test-aws",
		Region:   "us-east-1",
		Enabled:  true,
	}
	
	if err := engine.AddAccount(account); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	got, ok := engine.GetAccount("acc-1")
	if !ok {
		t.Fatal("expected account to be registered")
	}
	if got.Provider != ProviderAWS {
		t.Errorf("expected provider aws, got %s", got.Provider)
	}
}

func TestRecordCost(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.AddAccount(&CloudAccount{ID: "acc-1", Provider: ProviderAWS})
	
	cost := &StorageCost{
		AccountID:   "acc-1",
		Provider:    ProviderAWS,
		Bucket:      "my-bucket",
		Class:       ClassHot,
		SizeBytes:   1024 * 1024 * 1024,
		MonthlyCost: 23.0,
		Currency:    "USD",
	}
	
	if err := engine.RecordCost(cost); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	total := engine.GetCostByProvider(ProviderAWS)
	if total != 23.0 {
		t.Errorf("expected cost 23.0, got %f", total)
	}
}

func TestForecastCost(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.AddAccount(&CloudAccount{ID: "acc-1", Provider: ProviderAWS})
	engine.RecordCost(&StorageCost{AccountID: "acc-1", MonthlyCost: 100})
	
	forecast, err := engine.ForecastCost("acc-1", "monthly")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if forecast.CurrentCost != 100 {
		t.Errorf("expected current cost 100, got %f", forecast.CurrentCost)
	}
	if forecast.ForecastCost <= 0 {
		t.Error("expected positive forecast cost")
	}
}

func TestGenerateRecommendations(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	
	engine.AddAccount(&CloudAccount{ID: "acc-1", Provider: ProviderAWS})
	engine.RecordCost(&StorageCost{
		AccountID:   "acc-1",
		Class:       ClassHot,
		SizeBytes:   200 * 1024 * 1024 * 1024, // 200GB
		MonthlyCost: 46.0,
	})
	
	recs := engine.GenerateRecommendations()
	if len(recs) == 0 {
		t.Error("expected at least one recommendation")
	}
}
