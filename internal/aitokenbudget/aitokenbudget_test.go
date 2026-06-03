package aitokenbudget

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:       true,
		DefaultPeriod: PeriodMonthly,
		Currency:      "USD",
	}
	m := NewManager(config)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config.WarnThreshold != 0.8 {
		t.Errorf("expected WarnThreshold 0.8, got %f", m.config.WarnThreshold)
	}
}

func TestCreateBudget(t *testing.T) {
	m := NewManager(&Config{DefaultPeriod: PeriodMonthly})

	budget := &Budget{
		Name:      "测试预算",
		OwnerType: "user",
		OwnerID:   "user-001",
		Period:    PeriodMonthly,
		MaxTokens: 10000000,
		MaxCost:   100.0,
	}

	if err := m.CreateBudget(budget); err != nil {
		t.Fatalf("CreateBudget failed: %v", err)
	}
	if budget.ID == "" {
		t.Error("budget ID should be auto-generated")
	}
	if !budget.Enabled {
		t.Error("budget should be enabled by default")
	}
}

func TestRecordUsage(t *testing.T) {
	m := NewManager(&Config{DefaultPeriod: PeriodMonthly})

	budget := &Budget{
		Name:      "测试预算",
		OwnerType: "user",
		OwnerID:   "user-001",
		Period:    PeriodMonthly,
		MaxTokens: 1000000,
		MaxCost:   50.0,
	}
	m.CreateBudget(budget)

	record := &UsageRecord{
		BudgetID:         budget.ID,
		UserID:           "user-001",
		ServiceName:      "chat",
		ModelID:          "gpt-4o-mini",
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}

	if err := m.RecordUsage(record); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}
	if record.Cost <= 0 {
		t.Error("cost should be calculated")
	}
}

func TestBudgetStatus(t *testing.T) {
	m := NewManager(&Config{DefaultPeriod: PeriodMonthly})

	budget := &Budget{
		Name:          "状态测试",
		OwnerType:     "user",
		OwnerID:       "user-002",
		Period:        PeriodMonthly,
		MaxTokens:     10000,
		WarnThreshold: 0.8,
	}
	m.CreateBudget(budget)

	// 记录 90% 用量
	m.RecordUsage(&UsageRecord{
		BudgetID:         budget.ID,
		UserID:           "user-002",
		ModelID:          "gpt-4o-mini",
		PromptTokens:     5000,
		CompletionTokens: 4000,
		TotalTokens:      9000,
	})

	summary, err := m.GetBudgetStatus(budget.ID)
	if err != nil {
		t.Fatalf("GetBudgetStatus failed: %v", err)
	}

	if summary.Status != StatusWarning && summary.Status != StatusCritical {
		t.Errorf("expected warning or critical status, got %s", summary.Status)
	}
}

func TestGetCostAnalysis(t *testing.T) {
	m := NewManager(&Config{DefaultPeriod: PeriodMonthly})

	budget := &Budget{
		Name:      "分析测试",
		OwnerType: "user",
		OwnerID:   "user-003",
		Period:    PeriodMonthly,
		MaxTokens: 1000000,
	}
	m.CreateBudget(budget)

	// 记录多条用量
	models := []string{"gpt-4o", "gpt-4o-mini", "deepseek-v3"}
	for _, model := range models {
		m.RecordUsage(&UsageRecord{
			BudgetID:         budget.ID,
			UserID:           "user-003",
			ModelID:          model,
			PromptTokens:     10000,
			CompletionTokens: 5000,
			TotalTokens:      15000,
		})
	}

	analysis := m.GetCostAnalysis("monthly")
	if analysis.TotalCost <= 0 {
		t.Error("total cost should be positive")
	}
	if len(analysis.CostByModel) != 3 {
		t.Errorf("expected 3 models in cost analysis, got %d", len(analysis.CostByModel))
	}
}

func TestGetModelComparison(t *testing.T) {
	m := NewManager(&Config{})

	comparison := m.GetModelComparison(1000000)
	if len(comparison) == 0 {
		t.Error("model comparison should not be empty")
	}

	// 验证本地模型成本为 0
	for _, model := range comparison {
		if model["modelId"] == "local" {
			if model["totalCost"].(float64) != 0 {
				t.Error("local model cost should be 0")
			}
		}
	}
}

func TestAlerts(t *testing.T) {
	m := NewManager(&Config{
		DefaultPeriod:     PeriodMonthly,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
	})

	budget := &Budget{
		Name:          "告警测试",
		OwnerType:     "user",
		OwnerID:       "user-004",
		Period:        PeriodMonthly,
		MaxTokens:     1000,
		WarnThreshold: 0.8,
		HardLimit:     true,
	}
	m.CreateBudget(budget)

	// 触发告警
	m.RecordUsage(&UsageRecord{
		BudgetID:         budget.ID,
		UserID:           "user-004",
		ModelID:          "gpt-4o",
		PromptTokens:     500,
		CompletionTokens: 400,
		TotalTokens:      900,
	})

	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Error("expected alerts to be generated")
	}

	// 关闭告警
	if len(alerts) > 0 {
		err := m.DismissAlert(alerts[0].ID)
		if err != nil {
			t.Fatalf("DismissAlert failed: %v", err)
		}
		activeAlerts := m.GetAlerts(false)
		if len(activeAlerts) != 0 {
			t.Errorf("expected 0 active alerts after dismiss, got %d", len(activeAlerts))
		}
	}
}

func TestPeriodRanges(t *testing.T) {
	m := NewManager(&Config{})

	// 测试日周期
	start, end := m.getPeriodRange(PeriodDaily)
	if end.Sub(start) != 24*time.Hour {
		t.Errorf("daily period should be 24h, got %v", end.Sub(start))
	}

	// 测试周周期
	start, end = m.getPeriodRange(PeriodWeekly)
	if end.Sub(start) != 7*24*time.Hour {
		t.Errorf("weekly period should be 7 days, got %v", end.Sub(start))
	}

	// 测试月周期
	start, end = m.getPeriodRange(PeriodMonthly)
	if end.Month() == start.Month() {
		t.Error("monthly period should span different months")
	}
}

func TestListBudgets(t *testing.T) {
	m := NewManager(&Config{DefaultPeriod: PeriodMonthly})

	for i := 0; i < 3; i++ {
		m.CreateBudget(&Budget{
			Name:      "预算",
			OwnerType: "user",
			OwnerID:   "user-00" + string(rune('1'+i)),
			MaxTokens: 1000000,
		})
	}

	budgets := m.ListBudgets()
	if len(budgets) != 3 {
		t.Errorf("expected 3 budgets, got %d", len(budgets))
	}
}
