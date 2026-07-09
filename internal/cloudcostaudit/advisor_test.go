package cloudcostaudit

import (
	"testing"
	"time"
)

func TestAnalyze_BudgetOverrun(t *testing.T) {
	recs := Analyze(Signal{
		TotalMonthlyCost: 500,
		BudgetMonthlyUSD: 300,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-budget-overrun" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected cloud-budget-overrun recommendation")
	}
}

func TestAnalyze_DormantAccount(t *testing.T) {
	recs := Analyze(Signal{
		Accounts: []AccountSignal{
			{AccountID: "acc1", DormantDays: 120, MonthlyCostUSD: 50},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-dormant-acc1" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-dormant-acc1 recommendation")
	}
}

func TestAnalyze_HighEgress(t *testing.T) {
	recs := Analyze(Signal{
		TotalEgressGB:  200,
		TotalStorageGB: 1000,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-egress-high" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-egress-high recommendation")
	}
}

func TestAnalyze_MissingTierPolicy(t *testing.T) {
	recs := Analyze(Signal{
		Accounts: []AccountSignal{
			{AccountID: "acc2", StorageGB: 200, TierPolicyOK: false, MonthlyCostUSD: 80},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-tier-acc2" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-tier-acc2 recommendation")
	}
}

func TestAnalyze_R2Migration(t *testing.T) {
	recs := Analyze(Signal{
		TotalEgressGB: 600,
		Accounts: []AccountSignal{
			{Provider: ProviderAWS, AccountID: "aws1"},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-r2-egress" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-r2-egress recommendation")
	}
}

func TestAnalyze_StaleAudit(t *testing.T) {
	recs := Analyze(Signal{
		LastAuditAge: 45 * 24 * time.Hour,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-audit-stale" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-audit-stale recommendation")
	}
}

func TestAnalyze_UnusedAccounts(t *testing.T) {
	recs := Analyze(Signal{
		HasUnusedAccounts: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-unused-accounts" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-unused-accounts recommendation")
	}
}

func TestAnalyze_HighAPICost(t *testing.T) {
	recs := Analyze(Signal{
		Accounts: []AccountSignal{
			{AccountID: "api1", APICostUSD: 60, MonthlyCostUSD: 150, APICallCount: 2000000},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cloud-api-api1" {
			found = true
		}
	}
	if !found {
		t.Error("expected cloud-api-api1 recommendation")
	}
}

func TestAnalyze_EmptySignal(t *testing.T) {
	recs := Analyze(Signal{})
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations for empty signal, got %d", len(recs))
	}
}

func TestAnalyze_PriorityOrdering(t *testing.T) {
	recs := Analyze(Signal{
		TotalMonthlyCost: 500,
		BudgetMonthlyUSD: 300,
		HasUnusedAccounts: true,
		LastAuditAge:      45 * 24 * time.Hour,
	})
	if len(recs) < 2 {
		t.Fatal("expected multiple recommendations")
	}
	for i := 0; i < len(recs)-1; i++ {
		if priorityRank(recs[i].Priority) > priorityRank(recs[i+1].Priority) {
			t.Errorf("recommendations not sorted by priority at index %d", i)
		}
	}
}