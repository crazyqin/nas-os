package budgetalert

import (
	"testing"
	"time"
)

func TestNewBudgetAlertManager(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	if mgr == nil {
		t.Fatal("NewBudgetAlertManager returned nil")
	}
	if mgr.config.DefaultCurrency != "CNY" {
		t.Errorf("expected CNY, got %s", mgr.config.DefaultCurrency)
	}
}

func TestCreateBudget(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	budget, err := mgr.CreateBudget("用户存储", CategoryUser, 100*1024*1024*1024, &BudgetOptions{
		CostPerGB: 0.5,
		AlertAt:   80,
		CriticalAt: 95,
	})
	if err != nil {
		t.Fatalf("create budget failed: %v", err)
	}
	if budget.Name != "用户存储" {
		t.Errorf("expected 用户存储, got %s", budget.Name)
	}
	if budget.Period != PeriodMonthly {
		t.Errorf("expected monthly period, got %s", budget.Period)
	}
}

func TestUpdateUsage(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	budget, _ := mgr.CreateBudget("test", CategoryTotal, 1000, nil)
	if err := mgr.UpdateUsage(budget.ID, 500); err != nil {
		t.Fatalf("update usage failed: %v", err)
	}
	if err := mgr.UpdateUsage("nonexistent", 500); err == nil {
		t.Error("expected error for non-existent budget")
	}
}

func TestGetReport(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	mgr.CreateBudget("test1", CategoryUser, 1000, &BudgetOptions{CostPerGB: 1.0})
	mgr.CreateBudget("test2", CategoryBackup, 2000, &BudgetOptions{CostPerGB: 0.5})
	report := mgr.GetReport()
	if len(report) != 2 {
		t.Errorf("expected 2 reports, got %d", len(report))
	}
}

func TestGetCostSummary(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	mgr.CreateBudget("test", CategoryTotal, 1000, &BudgetOptions{CostPerGB: 1.0})
	summary := mgr.GetCostSummary()
	if summary == nil {
		t.Fatal("GetCostSummary returned nil")
	}
	if summary.Currency != "CNY" {
		t.Errorf("expected CNY, got %s", summary.Currency)
	}
}

func TestGetAlertsEmpty(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	alerts := mgr.GetAlerts(false)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestDeleteBudget(t *testing.T) {
	mgr := NewBudgetAlertManager(nil)
	budget, _ := mgr.CreateBudget("test", CategoryTotal, 1000, nil)
	if err := mgr.DeleteBudget(budget.ID); err != nil {
		t.Fatalf("delete budget failed: %v", err)
	}
	if err := mgr.DeleteBudget("nonexistent"); err == nil {
		t.Error("expected error for non-existent budget")
	}
}

func TestBudgetAlertManagerStartStop(t *testing.T) {
	mgr := NewBudgetAlertManager(&BudgetConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Hour,
	})
	if err := mgr.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := mgr.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}
