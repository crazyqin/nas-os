package costanalyzer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAnalyzerAnalyzeBudgetCapacity(t *testing.T) {
	analyzer := NewAnalyzer(zap.NewNop(), DefaultSmartCostConfig())
	assets := []*StorageAsset{
		{
			ID:            "ssd-hot",
			Name:          "SSD Hot Pool",
			Type:          StorageTypeSSD,
			CapacityBytes: 1000 << 30,
			UsedBytes:     700 << 30,
			MonthlyOpex:   20,
			PurchaseDate:  time.Now().AddDate(-1, 0, 0),
		},
		{
			ID:            "hdd-archive",
			Name:          "HDD Archive",
			Type:          StorageTypeHDD,
			CapacityBytes: 2000 << 30,
			UsedBytes:     1000 << 30,
			MonthlyOpex:   30,
			PurchaseDate:  time.Now().AddDate(-2, 0, 0),
		},
	}

	report, err := analyzer.AnalyzeBudgetCapacity(assets, &BudgetCapacityInput{
		MonthlyBudget:      700,
		MonthlyGrowthGB:    120,
		PlanningMonths:     6,
		TargetUtilization:  80,
		ExpansionCostPerGB: 1.5,
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "CNY", report.Currency)
	assert.Equal(t, 3000.0, report.TotalCapacityGB)
	assert.Equal(t, 1700.0, report.UsedGB)
	assert.Equal(t, 600.0, report.CurrentMonthlyCost)
	assert.Equal(t, 854.12, report.ProjectedMonthlyCost)
	assert.Equal(t, "exceeded", report.BudgetStatus)
	assert.Equal(t, 6, report.MonthsUntilTarget)
	assert.Equal(t, 25.0, report.ExpansionNeededGB)
	assert.Equal(t, 37.5, report.ExpansionCost)
	assert.Equal(t, 52.5, report.PotentialMonthlySave)
	assert.NotNil(t, report.QuickROI)
	require.Len(t, report.ByType, 2)
	assert.Equal(t, StorageTypeSSD, report.ByType[0].StorageType)
	assert.NotEmpty(t, report.Recommendations)
}

func TestAnalyzerAnalyzeBudgetCapacityInvalidInput(t *testing.T) {
	analyzer := NewAnalyzer(zap.NewNop(), nil)

	_, err := analyzer.AnalyzeBudgetCapacity(nil, &BudgetCapacityInput{MonthlyGrowthGB: -1})
	assert.Error(t, err)

	_, err = analyzer.AnalyzeBudgetCapacity(nil, &BudgetCapacityInput{TargetUtilization: 101})
	assert.Error(t, err)
}

func TestManagerAnalyzeBudgetCapacity(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)
	require.NoError(t, manager.AddAsset(&StorageAsset{
		ID:            "a1",
		Name:          "NVMe Pool",
		Type:          StorageTypeNVMe,
		CapacityBytes: 500 << 30,
		UsedBytes:     400 << 30,
		MonthlyOpex:   10,
		PurchaseDate:  time.Now().AddDate(0, -6, 0),
	}))

	report, err := manager.AnalyzeBudgetCapacity(&BudgetCapacityInput{
		MonthlyBudget:     500,
		MonthlyGrowthGB:   20,
		PlanningMonths:    3,
		TargetUtilization: 85,
	})

	require.NoError(t, err)
	assert.Equal(t, 500.0, report.TotalCapacityGB)
	assert.Equal(t, 400.0, report.UsedGB)
	assert.Equal(t, 1, len(report.ByType))
	assert.Contains(t, []string{"ok", "warning", "exceeded"}, report.BudgetStatus)
}

func TestMonthsUntilUtilization(t *testing.T) {
	assert.Equal(t, 0, monthsUntilUtilization(90, 100, 10, 80))
	assert.Equal(t, -1, monthsUntilUtilization(50, 100, 0, 80))
	assert.Equal(t, 3, monthsUntilUtilization(50, 100, 10, 80))
}
