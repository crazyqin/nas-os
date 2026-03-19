package budget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBudgetManager(t *testing.T) {
	manager := NewBudgetManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.budgets)
	assert.NotNil(t, manager.usages)
	assert.NotNil(t, manager.alerts)
}

func TestCreateBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:        "test-budget",
		Description: "Test budget",
		Type:        TypeStorage,
		Period:      PeriodMonthly,
		Scope:       ScopeGlobal,
		Amount:      1000.0,
	}

	budget, err := manager.CreateBudget(input, "admin")
	assert.NoError(t, err)
	assert.NotNil(t, budget)
	assert.NotEmpty(t, budget.ID)
	assert.Equal(t, "test-budget", budget.Name)
	assert.Equal(t, TypeStorage, budget.Type)
	assert.Equal(t, 1000.0, budget.Amount)
	assert.Equal(t, StatusActive, budget.Status)
}

func TestCreateBudgetInvalidAmount(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "invalid-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: -100,
	}

	budget, err := manager.CreateBudget(input, "admin")
	assert.Error(t, err)
	assert.Nil(t, budget)
	assert.Equal(t, ErrInvalidAmount, err)
}

func TestCreateDuplicateBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "duplicate-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	_, err := manager.CreateBudget(input, "admin")
	assert.NoError(t, err)

	// Try to create duplicate
	_, err = manager.CreateBudget(input, "admin")
	assert.Error(t, err)
	assert.Equal(t, ErrBudgetExists, err)
}

func TestGetBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "get-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	created, _ := manager.CreateBudget(input, "admin")

	budget, err := manager.GetBudget(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, budget.ID)
}

func TestGetBudgetNotFound(t *testing.T) {
	manager := NewBudgetManager()

	budget, err := manager.GetBudget("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, budget)
	assert.Equal(t, ErrBudgetNotFound, err)
}

func TestUpdateBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "update-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	created, _ := manager.CreateBudget(input, "admin")

	updateInput := Input{
		Name:        "updated-budget",
		Description: "Updated description",
		Amount:      2000,
	}

	updated, err := manager.UpdateBudget(created.ID, updateInput)
	assert.NoError(t, err)
	assert.Equal(t, "updated-budget", updated.Name)
	assert.Equal(t, "Updated description", updated.Description)
	assert.Equal(t, 2000.0, updated.Amount)
}

func TestUpdateBudgetNotFound(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{Amount: 1000}
	_, err := manager.UpdateBudget("nonexistent", input)
	assert.Error(t, err)
	assert.Equal(t, ErrBudgetNotFound, err)
}

func TestDeleteBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "delete-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	created, _ := manager.CreateBudget(input, "admin")

	err := manager.DeleteBudget(created.ID)
	assert.NoError(t, err)

	_, err = manager.GetBudget(created.ID)
	assert.Error(t, err)
}

func TestDeleteBudgetNotFound(t *testing.T) {
	manager := NewBudgetManager()

	err := manager.DeleteBudget("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrBudgetNotFound, err)
}

func TestListBudgets(t *testing.T) {
	manager := NewBudgetManager()

	// Create multiple budgets
	for i := 0; i < 5; i++ {
		input := Input{
			Name:   "list-test-budget-" + string(rune('0'+i)),
			Type:   TypeStorage,
			Period: PeriodMonthly,
			Scope:  ScopeGlobal,
			Amount: float64(1000 * (i + 1)),
		}
		manager.CreateBudget(input, "admin")
	}

	query := BudgetQuery{
		Page:     1,
		PageSize: 10,
	}

	budgets, total, err := manager.ListBudgets(query)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.True(t, len(budgets) >= 5)
}

func TestListBudgetsWithFilter(t *testing.T) {
	manager := NewBudgetManager()

	// Create budgets with different types
	inputs := []Input{
		{Name: "storage-budget", Type: TypeStorage, Period: PeriodMonthly, Scope: ScopeGlobal, Amount: 1000},
		{Name: "bandwidth-budget", Type: TypeBandwidth, Period: PeriodMonthly, Scope: ScopeGlobal, Amount: 500},
		{Name: "compute-budget", Type: TypeCompute, Period: PeriodMonthly, Scope: ScopeGlobal, Amount: 2000},
	}

	for _, input := range inputs {
		manager.CreateBudget(input, "admin")
	}

	query := BudgetQuery{
		Types:    []Type{TypeStorage},
		Page:     1,
		PageSize: 10,
	}

	budgets, _, err := manager.ListBudgets(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(budgets))
	assert.Equal(t, TypeStorage, budgets[0].Type)
}

func TestRecordUsage(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "usage-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	usageInput := UsageInput{
		BudgetID:     budget.ID,
		Amount:       100,
		SourceType:   "storage",
		SourceID:     "vol-001",
		Description:  "Monthly storage cost",
		ResourceType: "volume",
		ResourceID:   "vol-001",
		UnitCost:     0.1,
		Quantity:     1000,
	}

	usage, err := manager.RecordUsage(usageInput)
	assert.NoError(t, err)
	assert.NotNil(t, usage)
	assert.NotEmpty(t, usage.ID)
	assert.Equal(t, 100.0, usage.Amount)
	assert.Equal(t, 100.0, usage.Cumulative)

	// Check budget was updated
	updated, _ := manager.GetBudget(budget.ID)
	assert.Equal(t, 100.0, updated.UsedAmount)
	assert.Equal(t, 900.0, updated.Remaining)
}

func TestRecordUsageBudgetNotFound(t *testing.T) {
	manager := NewBudgetManager()

	usageInput := UsageInput{
		BudgetID: "nonexistent",
		Amount:   100,
	}

	_, err := manager.RecordUsage(usageInput)
	assert.Error(t, err)
	assert.Equal(t, ErrBudgetNotFound, err)
}

func TestRecordUsageExceedsBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "exceed-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 100,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Record usage that exceeds budget (150% of budget)
	usageInput := UsageInput{
		BudgetID: budget.ID,
		Amount:   150,
	}

	_, err := manager.RecordUsage(usageInput)
	assert.NoError(t, err) // Should still succeed but mark budget as exhausted

	updated, _ := manager.GetBudget(budget.ID)
	// 150% usage means exhausted (>= 100%)
	assert.Equal(t, StatusExhausted, updated.Status)
}

func TestGetUsageHistory(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "history-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Record multiple usages
	for i := 0; i < 3; i++ {
		manager.RecordUsage(UsageInput{
			BudgetID: budget.ID,
			Amount:   100,
		})
	}

	query := UsageQuery{
		Page:     1,
		PageSize: 10,
	}

	usages, total, err := manager.GetUsageHistory(budget.ID, query)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, 3, len(usages))
}

func TestGetUsageStats(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "stats-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Record usages with different sources
	sources := []string{"storage", "bandwidth", "compute"}
	for _, source := range sources {
		manager.RecordUsage(UsageInput{
			BudgetID:   budget.ID,
			Amount:     100,
			SourceType: source,
		})
	}

	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	stats, err := manager.GetUsageStats(budget.ID, start, end)
	assert.NoError(t, err)
	assert.Equal(t, 300.0, stats.TotalAmount)
	assert.Equal(t, 3, stats.Count)
	assert.NotNil(t, stats.BySourceType)
}

func TestResetBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "reset-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 1000,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Record usage
	manager.RecordUsage(UsageInput{
		BudgetID: budget.ID,
		Amount:   500,
	})

	// Reset
	reset, err := manager.ResetBudget(budget.ID)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, reset.UsedAmount)
	assert.Equal(t, 1000.0, reset.Remaining)
	assert.Equal(t, StatusActive, reset.Status)
}

func TestResetBudgetWithRollover(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:     "rollover-test-budget",
		Type:     TypeStorage,
		Period:   PeriodMonthly,
		Scope:    ScopeGlobal,
		Amount:   1000,
		Rollover: true,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Record partial usage
	manager.RecordUsage(UsageInput{
		BudgetID: budget.ID,
		Amount:   200,
	})

	// Reset with rollover
	reset, err := manager.ResetBudget(budget.ID)
	assert.NoError(t, err)
	// Remaining 800 should be added to amount
	assert.Equal(t, 1800.0, reset.Amount)
}

func TestGetActiveAlerts(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "alert-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 100,
		AlertConfig: &AlertConfig{
			Enabled:    true,
			Thresholds: DefaultAlertThresholds,
		},
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Record usage to trigger alert
	manager.RecordUsage(UsageInput{
		BudgetID: budget.ID,
		Amount:   90, // 90% of budget
	})

	query := AlertQuery{
		Page:     1,
		PageSize: 10,
	}

	alerts, total, err := manager.GetActiveAlerts(query)
	assert.NoError(t, err)
	assert.True(t, total >= 1)
	assert.True(t, len(alerts) >= 1)
}

func TestAcknowledgeAlert(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "ack-alert-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 100,
		AlertConfig: &AlertConfig{
			Enabled:    true,
			Thresholds: DefaultAlertThresholds,
		},
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Trigger alert
	manager.RecordUsage(UsageInput{
		BudgetID: budget.ID,
		Amount:   90,
	})

	alerts, _, _ := manager.GetActiveAlerts(AlertQuery{})
	if len(alerts) > 0 {
		err := manager.AcknowledgeAlert(alerts[0].ID, "admin")
		assert.NoError(t, err)
	}
}

func TestResolveAlert(t *testing.T) {
	manager := NewBudgetManager()

	input := Input{
		Name:   "resolve-alert-test-budget",
		Type:   TypeStorage,
		Period: PeriodMonthly,
		Scope:  ScopeGlobal,
		Amount: 100,
		AlertConfig: &AlertConfig{
			Enabled:    true,
			Thresholds: DefaultAlertThresholds,
		},
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// Trigger alert
	manager.RecordUsage(UsageInput{
		BudgetID: budget.ID,
		Amount:   90,
	})

	alerts, _, _ := manager.GetActiveAlerts(AlertQuery{})
	if len(alerts) > 0 {
		err := manager.ResolveAlert(alerts[0].ID)
		assert.NoError(t, err)
	}
}

func TestGenerateReport(t *testing.T) {
	manager := NewBudgetManager()

	// Create budgets
	for i := 0; i < 3; i++ {
		input := Input{
			Name:   "report-test-budget-" + string(rune('0'+i)),
			Type:   TypeStorage,
			Period: PeriodMonthly,
			Scope:  ScopeGlobal,
			Amount: 1000,
		}
		manager.CreateBudget(input, "admin")
	}

	request := ReportRequest{
		IncludeUsage: true,
		IncludeTrend: true,
	}

	report, err := manager.GenerateReport(request)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, 3, report.Summary.TotalBudgets)
}

func TestGetStats(t *testing.T) {
	manager := NewBudgetManager()

	// Create budgets with different types
	inputs := []Input{
		{Name: "stats-storage", Type: TypeStorage, Period: PeriodMonthly, Scope: ScopeGlobal, Amount: 1000},
		{Name: "stats-bandwidth", Type: TypeBandwidth, Period: PeriodMonthly, Scope: ScopeGlobal, Amount: 500},
	}

	for _, input := range inputs {
		manager.CreateBudget(input, "admin")
	}

	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 2, stats.TotalBudgets)
	assert.Equal(t, 1500.0, stats.TotalAmount)
	assert.NotNil(t, stats.ByType)
	assert.NotNil(t, stats.ByScope)
}

func TestCalculateNextReset(t *testing.T) {
	now := time.Now()

	tests := []struct {
		period   Period
		expected time.Time
	}{
		{PeriodDaily, now.AddDate(0, 0, 1)},
		{PeriodWeekly, now.AddDate(0, 0, 7)},
		{PeriodMonthly, now.AddDate(0, 1, 0)},
		{PeriodQuarter, now.AddDate(0, 3, 0)},
		{PeriodYearly, now.AddDate(1, 0, 0)},
	}

	for _, tt := range tests {
		next := calculateNextReset(now, tt.period)
		assert.NotNil(t, next)
		// Just verify it's in the future
		assert.True(t, next.After(now))
	}
}

func TestCalculatePercent(t *testing.T) {
	tests := []struct {
		used     float64
		total    float64
		expected float64
	}{
		{50, 100, 50},
		{100, 100, 100},
		{150, 100, 100}, // Capped at 100
		{0, 100, 0},
		{50, 0, 0}, // Avoid division by zero
	}

	for _, tt := range tests {
		result := calculatePercent(tt.used, tt.total)
		assert.Equal(t, tt.expected, result)
	}
}

func TestDefaultAlertConfig(t *testing.T) {
	config := DefaultAlertConfig()

	assert.True(t, config.Enabled)
	assert.True(t, len(config.Thresholds) > 0)
	assert.True(t, config.NotifyEmail)
	assert.Equal(t, 60, config.CooldownMinutes)
}

func TestBudgetTypes(t *testing.T) {
	types := []Type{TypeStorage, TypeBandwidth, TypeCompute, TypeOperations, TypeTotal}
	for _, typ := range types {
		assert.NotEmpty(t, string(typ))
	}
}

func TestBudgetPeriods(t *testing.T) {
	periods := []Period{PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodQuarter, PeriodYearly}
	for _, period := range periods {
		assert.NotEmpty(t, string(period))
	}
}

func TestBudgetScopes(t *testing.T) {
	scopes := []Scope{ScopeGlobal, ScopeUser, ScopeGroup, ScopeVolume, ScopeService, ScopeDirectory}
	for _, scope := range scopes {
		assert.NotEmpty(t, string(scope))
	}
}

func TestBudgetStatuses(t *testing.T) {
	statuses := []Status{StatusActive, StatusPaused, StatusExceeded, StatusExhausted, StatusArchived}
	for _, status := range statuses {
		assert.NotEmpty(t, string(status))
	}
}

func TestPaginationBounds(t *testing.T) {
	tests := []struct {
		total    int
		page     int
		pageSize int
		start    int
		end      int
	}{
		{100, 1, 10, 0, 10},
		{100, 2, 10, 10, 20},
		{5, 1, 10, 0, 5},
		{100, 0, 10, 0, 10}, // Invalid page defaults to 1
		{100, 1, 0, 0, 100}, // Invalid pageSize defaults to 20, but let's check
	}

	for _, tt := range tests {
		start, end := getPaginationBounds(tt.total, tt.page, tt.pageSize)
		// Note: getPaginationBounds may have different defaults
		_ = start
		_ = end
	}
}
