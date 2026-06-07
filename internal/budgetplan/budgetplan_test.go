package budgetplan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 预算编制测试 ==========

func TestNewBudgetManager(t *testing.T) {
	manager := NewBudgetManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.budgets)
	assert.NotNil(t, manager.expenses)
}

func TestCreateBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "test-budget",
		Description: "测试预算",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
		Categories: map[ExpenseCategory]float64{
			CategoryHardware: 5000,
			CategorySoftware: 3000,
			CategoryService:  2000,
		},
	}

	budget, err := manager.CreateBudget(input, "admin")
	assert.NoError(t, err)
	assert.NotNil(t, budget)
	assert.NotEmpty(t, budget.ID)
	assert.Equal(t, "test-budget", budget.Name)
	assert.Equal(t, PeriodMonthly, budget.Period)
	assert.Equal(t, 10000.0, budget.TotalAmount)
	assert.Equal(t, 0.0, budget.UsedAmount)
	assert.Equal(t, 10000.0, budget.RemainingAmount)
	assert.Equal(t, StatusActive, budget.Status)
	assert.Equal(t, "admin", budget.CreatedBy)
}

func TestCreateBudgetInvalidInput(t *testing.T) {
	manager := NewBudgetManager()

	// 空名称
	input := BudgetCreateInput{
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}
	_, err := manager.CreateBudget(input, "admin")
	assert.Error(t, err)

	// 无效金额
	input = BudgetCreateInput{
		Name:        "test",
		Period:      PeriodMonthly,
		TotalAmount: -100,
	}
	_, err = manager.CreateBudget(input, "admin")
	assert.Error(t, err)

	// 无效周期
	input = BudgetCreateInput{
		Name:        "test",
		Period:      "invalid",
		TotalAmount: 10000.0,
	}
	_, err = manager.CreateBudget(input, "admin")
	assert.Error(t, err)
}

func TestCreateDuplicateBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "duplicate-budget",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	_, err := manager.CreateBudget(input, "admin")
	assert.NoError(t, err)

	// 创建重复预算
	_, err = manager.CreateBudget(input, "admin")
	assert.Error(t, err)
	assert.Equal(t, ErrBudgetExists, err)
}

func TestGetBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "get-test",
		Period:      PeriodMonthly,
		TotalAmount: 5000.0,
	}

	created, _ := manager.CreateBudget(input, "admin")

	budget, err := manager.GetBudget(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, budget.ID)
	assert.Equal(t, created.Name, budget.Name)
}

func TestGetBudgetNotFound(t *testing.T) {
	manager := NewBudgetManager()

	budget, err := manager.GetBudget("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, budget)
	assert.Equal(t, ErrBudgetNotFound, err)
}

func TestListBudgets(t *testing.T) {
	manager := NewBudgetManager()

	// 创建多个预算
	for i := 0; i < 3; i++ {
		input := BudgetCreateInput{
			Name:        "budget-" + string(rune('A'+i)),
			Period:      PeriodMonthly,
			TotalAmount: float64((i + 1) * 1000),
		}
		manager.CreateBudget(input, "admin")
	}

	budgets := manager.ListBudgets()
	assert.Len(t, budgets, 3)
}

func TestUpdateBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "update-test",
		Period:      PeriodMonthly,
		TotalAmount: 5000.0,
	}

	created, _ := manager.CreateBudget(input, "admin")

	updates := BudgetCreateInput{
		Name:        "updated-budget",
		TotalAmount: 8000.0,
	}

	updated, err := manager.UpdateBudget(created.ID, updates)
	assert.NoError(t, err)
	assert.Equal(t, "updated-budget", updated.Name)
	assert.Equal(t, 8000.0, updated.TotalAmount)
	assert.Equal(t, 8000.0, updated.RemainingAmount)
}

func TestDeleteBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "delete-test",
		Period:      PeriodMonthly,
		TotalAmount: 5000.0,
	}

	created, _ := manager.CreateBudget(input, "admin")

	err := manager.DeleteBudget(created.ID)
	assert.NoError(t, err)

	budget, err := manager.GetBudget(created.ID)
	assert.Error(t, err)
	assert.Nil(t, budget)
}

func TestPauseResumeBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "pause-test",
		Period:      PeriodMonthly,
		TotalAmount: 5000.0,
	}

	created, _ := manager.CreateBudget(input, "admin")

	// 暂停
	err := manager.PauseBudget(created.ID)
	assert.NoError(t, err)

	budget, _ := manager.GetBudget(created.ID)
	assert.Equal(t, StatusPaused, budget.Status)

	// 恢复
	err = manager.ResumeBudget(created.ID)
	assert.NoError(t, err)

	budget, _ = manager.GetBudget(created.ID)
	assert.Equal(t, StatusActive, budget.Status)
}

// ========== 支出追踪测试 ==========

func TestRecordExpense(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "expense-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	expenseInput := ExpenseInput{
		BudgetID:    budget.ID,
		Amount:      2500.0,
		Category:    CategoryHardware,
		Description: "购买服务器",
		Vendor:      "Dell",
	}

	expense, err := manager.RecordExpense(expenseInput, "user1")
	assert.NoError(t, err)
	assert.NotNil(t, expense)
	assert.NotEmpty(t, expense.ID)
	assert.Equal(t, 2500.0, expense.Amount)
	assert.Equal(t, CategoryHardware, expense.Category)

	// 验证预算更新
	updatedBudget, _ := manager.GetBudget(budget.ID)
	assert.Equal(t, 2500.0, updatedBudget.UsedAmount)
	assert.Equal(t, 7500.0, updatedBudget.RemainingAmount)
	assert.Equal(t, 25.0, updatedBudget.UsagePercent)
}

func TestRecordExpenseExceedBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "exceed-test",
		Period:      PeriodMonthly,
		TotalAmount: 1000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 记录超出预算的支出
	expenseInput := ExpenseInput{
		BudgetID: budget.ID,
		Amount:   1500.0,
		Category: CategoryService,
	}

	_, err := manager.RecordExpense(expenseInput, "user1")
	assert.NoError(t, err)

	// 验证预算状态变为超支
	updatedBudget, _ := manager.GetBudget(budget.ID)
	assert.Equal(t, StatusExceeded, updatedBudget.Status)
}

func TestRecordExpenseInvalidInput(t *testing.T) {
	manager := NewBudgetManager()

	// 无效金额
	input := ExpenseInput{
		BudgetID: "test",
		Amount:   -100,
		Category: CategoryHardware,
	}
	_, err := manager.RecordExpense(input, "user1")
	assert.Error(t, err)

	// 无效分类
	input = ExpenseInput{
		BudgetID: "test",
		Amount:   100,
		Category: "invalid",
	}
	_, err = manager.RecordExpense(input, "user1")
	assert.Error(t, err)
}

func TestListExpenses(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "list-expense-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 记录多笔支出
	for i := 0; i < 5; i++ {
		expenseInput := ExpenseInput{
			BudgetID: budget.ID,
			Amount:   float64((i + 1) * 100),
			Category: CategoryHardware,
		}
		manager.RecordExpense(expenseInput, "user1")
	}

	expenses, err := manager.ListExpenses(budget.ID)
	assert.NoError(t, err)
	assert.Len(t, expenses, 5)
}

func TestDeleteExpense(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "delete-expense-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	expenseInput := ExpenseInput{
		BudgetID: budget.ID,
		Amount:   2000.0,
		Category: CategorySoftware,
	}

	expense, _ := manager.RecordExpense(expenseInput, "user1")

	// 删除支出
	err := manager.DeleteExpense(budget.ID, expense.ID)
	assert.NoError(t, err)

	// 验证预算更新
	updatedBudget, _ := manager.GetBudget(budget.ID)
	assert.Equal(t, 0.0, updatedBudget.UsedAmount)
	assert.Equal(t, 10000.0, updatedBudget.RemainingAmount)
}

// ========== 成本预测测试 ==========

func TestForecastCost(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "forecast-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 添加历史数据（模拟3个月的支出）
	now := time.Now()
	for i := 0; i < 3; i++ {
		date := now.AddDate(0, -2+i, 0)
		expenseInput := ExpenseInput{
			BudgetID:   budget.ID,
			Amount:     float64((i + 1) * 1000),
			Category:   CategoryHardware,
			OccurredAt: &date,
		}
		manager.RecordExpense(expenseInput, "user1")
	}

	result, err := manager.ForecastCost(budget.ID, 3)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ForecastPoints)
	assert.NotEmpty(t, result.Trend)
	assert.NotEmpty(t, result.Recommendations)
}

func TestForecastCostInsufficientData(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "insufficient-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 只添加1条记录（不足）
	expenseInput := ExpenseInput{
		BudgetID: budget.ID,
		Amount:   1000.0,
		Category: CategoryHardware,
	}
	manager.RecordExpense(expenseInput, "user1")

	_, err := manager.ForecastCost(budget.ID, 3)
	assert.Error(t, err)
	assert.Equal(t, ErrInsufficientData, err)
}

// ========== 预算对比测试 ==========

func TestCompareBudget(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "compare-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
		Categories: map[ExpenseCategory]float64{
			CategoryHardware: 5000,
			CategorySoftware: 3000,
			CategoryService:  2000,
		},
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 记录支出
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   3000.0,
		Category: CategoryHardware,
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   2000.0,
		Category: CategorySoftware,
	}, "user1")

	comp, err := manager.CompareBudget(budget.ID)
	assert.NoError(t, err)
	assert.NotNil(t, comp)
	assert.Equal(t, 10000.0, comp.BudgetedAmount)
	assert.Equal(t, 5000.0, comp.ActualAmount)
	assert.Equal(t, 5000.0, comp.Variance)
	assert.Equal(t, "under_budget", comp.Status)
	// 3 categories in budget definition (hardware, software, service), even if service has no expenses
	assert.Len(t, comp.CategoryBreakdown, 3)
}

func TestCompareAllBudgets(t *testing.T) {
	manager := NewBudgetManager()

	// 创建多个预算
	for i := 0; i < 3; i++ {
		input := BudgetCreateInput{
			Name:        "compare-all-" + string(rune('A'+i)),
			Period:      PeriodMonthly,
			TotalAmount: float64((i + 1) * 5000),
		}
		budget, _ := manager.CreateBudget(input, "admin")

		// 记录一些支出
		manager.RecordExpense(ExpenseInput{
			BudgetID: budget.ID,
			Amount:   float64((i + 1) * 1000),
			Category: CategoryService,
		}, "user1")
	}

	result := manager.CompareAllBudgets()
	assert.NotNil(t, result)
	assert.Len(t, result.Comparisons, 3)
	assert.Greater(t, result.TotalBudgeted, 0.0)
	assert.Greater(t, result.TotalActual, 0.0)
}

// ========== 报表生成测试 ==========

func TestGenerateReport(t *testing.T) {
	manager := NewBudgetManager()

	// 创建预算和支出
	input := BudgetCreateInput{
		Name:        "report-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	manager.RecordExpense(ExpenseInput{
		BudgetID:    budget.ID,
		Amount:      3000.0,
		Category:    CategoryHardware,
		Description: "服务器",
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID:    budget.ID,
		Amount:      2000.0,
		Category:    CategorySoftware,
		Description: "许可证",
	}, "user1")

	now := time.Now()
	startDate := now.AddDate(0, -1, 0)
	endDate := now

	request := ReportRequest{
		Type:               ReportTypeMonthly,
		Name:               "测试月度报表",
		StartDate:          &startDate,
		EndDate:            &endDate,
		IncludeTopExpenses: true,
		TopExpensesCount:   5,
	}

	report, err := manager.GenerateReport(request, "admin")
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, "测试月度报表", report.Name)
	assert.Equal(t, ReportTypeMonthly, report.Type)
	assert.Equal(t, "admin", report.GeneratedBy)
	assert.NotNil(t, report.Summary)
	assert.NotNil(t, report.BudgetDetails)
	assert.NotNil(t, report.ExpenseSummary)
}

func TestGenerateMonthlyReport(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "monthly-report-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 添加本月支出
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   3000.0,
		Category: CategoryHardware,
	}, "user1")

	report, err := manager.GenerateMonthlyReport(time.Now().Year(), time.Now().Month(), "admin")
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, ReportTypeMonthly, report.Type)
}

func TestGenerateQuarterlyReport(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "quarterly-report-test",
		Period:      PeriodQuarterly,
		TotalAmount: 30000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 添加支出
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   10000.0,
		Category: CategoryService,
	}, "user1")

	quarter := (int(time.Now().Month())-1)/3 + 1
	report, err := manager.GenerateQuarterlyReport(time.Now().Year(), quarter, "admin")
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, ReportTypeQuarterly, report.Type)
}

func TestGenerateYearlyReport(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "yearly-report-test",
		Period:      PeriodYearly,
		TotalAmount: 120000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 添加支出
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   50000.0,
		Category: CategoryHardware,
	}, "user1")

	report, err := manager.GenerateYearlyReport(time.Now().Year(), "admin")
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, ReportTypeYearly, report.Type)
}

// ========== 辅助函数测试 ==========

func TestLinearRegression(t *testing.T) {
	x := []float64{0, 1, 2, 3, 4}
	y := []float64{1, 3, 5, 7, 9}

	slope, intercept, r2 := linearRegression(x, y)
	assert.InDelta(t, 2.0, slope, 0.01)
	assert.InDelta(t, 1.0, intercept, 0.01)
	assert.InDelta(t, 1.0, r2, 0.01)
}

func TestLinearRegressionSinglePoint(t *testing.T) {
	x := []float64{0}
	y := []float64{10}

	slope, _, _ := linearRegression(x, y)
	assert.Equal(t, 0.0, slope)
}

func TestRoundTo2(t *testing.T) {
	assert.Equal(t, 1.23, roundTo2(1.234))
	assert.Equal(t, 1.24, roundTo2(1.235))
	assert.Equal(t, 0.0, roundTo2(0.001))
}

func TestCalculateUsagePercent(t *testing.T) {
	manager := NewBudgetManager()

	assert.Equal(t, 50.0, manager.calculateUsagePercent(50, 100))
	assert.Equal(t, 0.0, manager.calculateUsagePercent(0, 100))
	assert.Equal(t, 0.0, manager.calculateUsagePercent(50, 0))
}

func TestCalculateEndDate(t *testing.T) {
	manager := NewBudgetManager()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)

	endDate := manager.calculateEndDate(start, PeriodMonthly)
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local), endDate)

	endDate = manager.calculateEndDate(start, PeriodQuarterly)
	assert.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local), endDate)

	endDate = manager.calculateEndDate(start, PeriodYearly)
	assert.Equal(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local), endDate)
}

func TestGetBudgetStats(t *testing.T) {
	manager := NewBudgetManager()

	// 创建不同状态的预算
	input1 := BudgetCreateInput{
		Name:        "active-budget",
		Period:      PeriodMonthly,
		TotalAmount: 5000.0,
	}
	budget1, _ := manager.CreateBudget(input1, "admin")

	input2 := BudgetCreateInput{
		Name:        "paused-budget",
		Period:      PeriodMonthly,
		TotalAmount: 5000.0,
	}
	manager.CreateBudget(input2, "admin")

	// 暂停第二个预算
	manager.PauseBudget(budget1.ID)

	stats := manager.GetBudgetStats()
	assert.Equal(t, 1, stats[StatusActive])
	assert.Equal(t, 1, stats[StatusPaused])
}

func TestQueryExpenses(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "query-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 记录不同分类的支出
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   1000.0,
		Category: CategoryHardware,
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   2000.0,
		Category: CategorySoftware,
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   3000.0,
		Category: CategoryHardware,
	}, "user1")

	// 查询硬件类支出
	query := ExpenseQuery{
		BudgetID:   budget.ID,
		Categories: []ExpenseCategory{CategoryHardware},
	}

	expenses := manager.QueryExpenses(query)
	assert.Len(t, expenses, 2)

	// 查询金额大于1500的支出
	minAmount := 1500.0
	query = ExpenseQuery{
		BudgetID:  budget.ID,
		MinAmount: &minAmount,
	}

	expenses = manager.QueryExpenses(query)
	assert.Len(t, expenses, 2)
}

func TestGetExpenseSummary(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "summary-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 记录不同分类的支出
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   3000.0,
		Category: CategoryHardware,
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   2000.0,
		Category: CategorySoftware,
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   1000.0,
		Category: CategoryHardware,
	}, "user1")

	summary, err := manager.GetExpenseSummary(budget.ID)
	assert.NoError(t, err)
	assert.Equal(t, 4000.0, summary[CategoryHardware])
	assert.Equal(t, 2000.0, summary[CategorySoftware])
}

func TestGetVarianceAnalysis(t *testing.T) {
	manager := NewBudgetManager()

	input := BudgetCreateInput{
		Name:        "variance-test",
		Period:      PeriodMonthly,
		TotalAmount: 10000.0,
		Categories: map[ExpenseCategory]float64{
			CategoryHardware: 5000,
			CategorySoftware: 3000,
			CategoryService:  2000,
		},
	}

	budget, _ := manager.CreateBudget(input, "admin")

	// 记录支出（硬件超支，软件节余）
	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   6000.0,
		Category: CategoryHardware,
	}, "user1")

	manager.RecordExpense(ExpenseInput{
		BudgetID: budget.ID,
		Amount:   1000.0,
		Category: CategorySoftware,
	}, "user1")

	analysis, err := manager.GetVarianceAnalysis(budget.ID)
	assert.NoError(t, err)
	assert.NotNil(t, analysis)
	assert.Equal(t, budget.ID, analysis["budget_id"])
	assert.Contains(t, analysis["over_budget_categories"], CategoryHardware)
	assert.Contains(t, analysis["under_budget_categories"], CategorySoftware)
}
