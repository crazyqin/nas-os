package assetvaluator

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.factors == nil {
		t.Error("factors not initialized")
	}
}

func TestRegisterAsset(t *testing.T) {
	m := NewManager()
	asset := &DigitalAsset{
		ID:        "a1",
		Name:      "Wedding Photo",
		Type:      AssetPhoto,
		Path:      "/photos/wedding.jpg",
		SizeBytes: 5 * 1024 * 1024,
	}
	err := m.RegisterAsset(asset)
	if err != nil {
		t.Fatalf("RegisterAsset failed: %v", err)
	}
}

func TestAddValuation(t *testing.T) {
	m := NewManager()
	m.RegisterAsset(&DigitalAsset{ID: "a1", Name: "Photo", Type: AssetPhoto})
	valuation := &AssetValuation{
		AssetID:    "a1",
		Method:     ValuationSentiment,
		ValueUSD:   150.0,
		Confidence: 0.85,
		Factors:    []string{"sentiment", "rarity"},
	}
	m.AddValuation(valuation)
	got, ok := m.GetValue("a1")
	if !ok {
		t.Fatal("GetValue failed")
	}
	if got.ValueUSD != 150.0 {
		t.Errorf("expected 150.0, got %f", got.ValueUSD)
	}
}

func TestCreatePolicy(t *testing.T) {
	m := NewManager()
	policy := &InsurancePolicy{
		ID:            "p1",
		Name:          "Photo Insurance",
		AssetTypes:    []AssetType{AssetPhoto, AssetVideo},
		TotalValueUSD: 10000,
		CoverageUSD:   8000,
		PremiumUSD:    100,
		Status:        "active",
	}
	m.CreatePolicy(policy)
	stats := m.GetStats()
	if stats["active_policies"] != 1 {
		t.Errorf("expected 1 policy, got %v", stats["active_policies"])
	}
}

func TestCreateEstatePlan(t *testing.T) {
	m := NewManager()
	plan := &EstatePlan{
		ID:      "e1",
		OwnerID: "user1",
		Beneficiaries: []Beneficiary{
			{UserID: "user2", Name: "Child", Share: 0.6},
			{UserID: "user3", Name: "Spouse", Share: 0.4},
		},
		Assets: []string{"a1", "a2"},
		Status: "draft",
	}
	m.CreateEstatePlan(plan)
	stats := m.GetStats()
	if stats["estate_plans"] != 1 {
		t.Errorf("expected 1 plan, got %v", stats["estate_plans"])
	}
}

func TestGenerateReport(t *testing.T) {
	m := NewManager()
	m.RegisterAsset(&DigitalAsset{ID: "a1", Type: AssetPhoto})
	m.RegisterAsset(&DigitalAsset{ID: "a2", Type: AssetVideo})
	m.AddValuation(&AssetValuation{AssetID: "a1", ValueUSD: 100})
	m.AddValuation(&AssetValuation{AssetID: "a2", ValueUSD: 200})
	report := m.GenerateReport()
	if report.TotalAssets != 2 {
		t.Errorf("expected 2 assets, got %d", report.TotalAssets)
	}
	if report.TotalValueUSD != 300 {
		t.Errorf("expected 300 USD, got %f", report.TotalValueUSD)
	}
}

func TestValuationFactors(t *testing.T) {
	m := NewManager()
	if m.factors.SentimentWeight != 0.3 {
		t.Errorf("sentiment weight should be 0.3, got %f", m.factors.SentimentWeight)
	}
}
