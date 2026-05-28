// Package familyfinance 提供家庭财务中心功能
// familyfinance_test.go - 完整测试
package familyfinance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupFinanceEngine(t *testing.T) *FinanceEngine {
	logger, _ := zap.NewDevelopment()
	engine := NewFinanceEngine(logger)
	return engine
}

func setupTestEnvironment(t *testing.T) (*FinanceEngine, *BudgetManager, *InvestmentManager, *BillManager, *AnalyticsEngine) {
	logger, _ := zap.NewDevelopment()
	engine := NewFinanceEngine(logger)
	budgetMgr := NewBudgetManager(logger, engine)
	investMgr := NewInvestmentManager(logger)
	billMgr := NewBillManager(logger, engine)
	analytics := NewAnalyticsEngine(logger, engine)

	// 创建测试账户
	account := &Account{
		ID:       "acc-test",
		Name:     "测试银行账户",
		Type:     AccountTypeBank,
		Balance:  10000,
		Currency: "CNY",
	}
	_ = engine.CreateAccount(account)

	return engine, budgetMgr, investMgr, billMgr, analytics
}

// ========== 账户管理测试 ==========

func TestCreateAccount(t *testing.T) {
	t.Run("成功创建账户", func(t *testing.T) {
		engine := setupFinanceEngine(t)

		account := &Account{
			Name:     "工资卡",
			Type:     AccountTypeBank,
			Bank:     "工商银行",
			Balance:  5000,
			Currency: "CNY",
		}

		err := engine.CreateAccount(account)
		require.NoError(t, err)
		assert.NotEmpty(t, account.ID)
		assert.Equal(t, "工资卡", account.Name)
	})

	t.Run("重复ID创建失败", func(t *testing.T) {
		engine := setupFinanceEngine(t)

		account := &Account{
			ID:   "acc-1",
			Name: "账户1",
			Type: AccountTypeBank,
		}

		err := engine.CreateAccount(account)
		require.NoError(t, err)

		err = engine.CreateAccount(account)
		assert.ErrorIs(t, err, ErrAccountExists)
	})
}

func TestGetAccount(t *testing.T) {
	t.Run("获取存在的账户", func(t *testing.T) {
		engine := setupFinanceEngine(t)

		account := &Account{
			ID:   "acc-1",
			Name: "测试账户",
			Type: AccountTypeBank,
		}
		_ = engine.CreateAccount(account)

		got, err := engine.GetAccount("acc-1")
		require.NoError(t, err)
		assert.Equal(t, "测试账户", got.Name)
	})

	t.Run("获取不存在的账户", func(t *testing.T) {
		engine := setupFinanceEngine(t)

		_, err := engine.GetAccount("nonexistent")
		assert.ErrorIs(t, err, ErrAccountNotFound)
	})
}

func TestDeleteAccount(t *testing.T) {
	t.Run("删除无关联交易的账户", func(t *testing.T) {
		engine := setupFinanceEngine(t)

		account := &Account{
			ID:   "acc-1",
			Name: "测试账户",
			Type: AccountTypeBank,
		}
		_ = engine.CreateAccount(account)

		err := engine.DeleteAccount("acc-1")
		require.NoError(t, err)

		_, err = engine.GetAccount("acc-1")
		assert.ErrorIs(t, err, ErrAccountNotFound)
	})

	t.Run("删除不存在的账户", func(t *testing.T) {
		engine := setupFinanceEngine(t)

		err := engine.DeleteAccount("nonexistent")
		assert.ErrorIs(t, err, ErrAccountNotFound)
	})
}

func TestListAccounts(t *testing.T) {
	engine := setupFinanceEngine(t)

	_ = engine.CreateAccount(&Account{ID: "acc-1", Name: "账户1", Type: AccountTypeBank})
	_ = engine.CreateAccount(&Account{ID: "acc-2", Name: "账户2", Type: AccountTypeCash})

	accounts := engine.ListAccounts()
	assert.Len(t, accounts, 2)
}

func TestGetTotalBalance(t *testing.T) {
	engine := setupFinanceEngine(t)

	_ = engine.CreateAccount(&Account{ID: "acc-1", Name: "账户1", Type: AccountTypeBank, Balance: 5000})
	_ = engine.CreateAccount(&Account{ID: "acc-2", Name: "账户2", Type: AccountTypeBank, Balance: 3000})

	total := engine.GetTotalBalance()
	assert.Equal(t, float64(8000), total)
}

// ========== 交易管理测试 ==========

func TestAddTransaction(t *testing.T) {
	t.Run("添加收入交易", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)

		tx := &Transaction{
			AccountID:    "acc-test",
			Type:         TransactionTypeIncome,
			Amount:       5000,
			CategoryID:   "cat-salary",
			Description:  "工资",
		}

		err := engine.AddTransaction(tx)
		require.NoError(t, err)

		account, _ := engine.GetAccount("acc-test")
		assert.Equal(t, float64(15000), account.Balance)
	})

	t.Run("添加支出交易", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)

		tx := &Transaction{
			AccountID:   "acc-test",
			Type:        TransactionTypeExpense,
			Amount:      2000,
			CategoryID:  "cat-food",
			Description: "餐饮",
		}

		err := engine.AddTransaction(tx)
		require.NoError(t, err)

		account, _ := engine.GetAccount("acc-test")
		assert.Equal(t, float64(8000), account.Balance)
	})

	t.Run("余额不足的支出", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)

		tx := &Transaction{
			AccountID: "acc-test",
			Type:      TransactionTypeExpense,
			Amount:    20000,
		}

		err := engine.AddTransaction(tx)
		assert.Error(t, err)
	})
}

func TestTransferTransaction(t *testing.T) {
	engine := setupFinanceEngine(t)

	_ = engine.CreateAccount(&Account{ID: "acc-1", Name: "账户1", Type: AccountTypeBank, Balance: 10000})
	_ = engine.CreateAccount(&Account{ID: "acc-2", Name: "账户2", Type: AccountTypeBank, Balance: 0})

	tx := &Transaction{
		AccountID:   "acc-1",
		ToAccountID: "acc-2",
		Type:        TransactionTypeTransfer,
		Amount:      3000,
		Description: "转账",
	}

	err := engine.AddTransaction(tx)
	require.NoError(t, err)

	acc1, _ := engine.GetAccount("acc-1")
	acc2, _ := engine.GetAccount("acc-2")
	assert.Equal(t, float64(7000), acc1.Balance)
	assert.Equal(t, float64(3000), acc2.Balance)
}

func TestQueryTransactions(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)

	// 添加多条交易
	for i := 0; i < 5; i++ {
		_ = engine.AddTransaction(&Transaction{
			AccountID: "acc-test",
			Type:      TransactionTypeExpense,
			Amount:    float64((i + 1) * 100),
			CategoryID: "cat-food",
		})
	}

	query := TransactionQuery{
		AccountID: "acc-test",
		Page:      1,
		PageSize:  10,
	}
	result := engine.QueryTransactions(query)
	assert.Len(t, result, 5)
}

// ========== 预算管理测试 ==========

func TestCreateBudget(t *testing.T) {
	_, budgetMgr, _, _, _ := setupTestEnvironment(t)

	budget := &Budget{
		CategoryID:   "cat-food",
		Amount:       3000,
		Period:       "monthly",
		AlertPercent: 80,
	}

	err := budgetMgr.CreateBudget(budget)
	require.NoError(t, err)
	assert.NotEmpty(t, budget.ID)
	assert.Equal(t, float64(3000), budget.Amount)
}

func TestRecordExpenseToBudget(t *testing.T) {
	_, budgetMgr, _, _, _ := setupTestEnvironment(t)

	budget := &Budget{
		CategoryID:   "cat-food",
		Amount:       3000,
		Period:       "monthly",
		AlertPercent: 80,
	}
	_ = budgetMgr.CreateBudget(budget)

	// 记录支出
	alerted, err := budgetMgr.RecordExpense("cat-food", 2500)
	require.NoError(t, err)
	assert.True(t, alerted) // 应该触发预警

	got, _ := budgetMgr.GetBudget(budget.ID)
	assert.Equal(t, float64(2500), got.Spent)
	assert.True(t, got.IsAlerted)
}

func TestBudgetExceeded(t *testing.T) {
	_, budgetMgr, _, _, _ := setupTestEnvironment(t)

	budget := &Budget{
		CategoryID: "cat-food",
		Amount:     3000,
		Period:     "monthly",
	}
	_ = budgetMgr.CreateBudget(budget)

	_, err := budgetMgr.RecordExpense("cat-food", 3500)
	assert.ErrorIs(t, err, ErrBudgetExceeded)
}

func TestGetBudgetStatus(t *testing.T) {
	_, budgetMgr, _, _, _ := setupTestEnvironment(t)

	_ = budgetMgr.CreateBudget(&Budget{CategoryID: "cat-food", Amount: 3000, Period: "monthly"})
	_ = budgetMgr.CreateBudget(&Budget{CategoryID: "cat-transport", Amount: 1000, Period: "monthly"})

	status := budgetMgr.GetBudgetStatus()
	assert.Equal(t, 2, status["total_budgets"])
}

// ========== 投资管理测试 ==========

func TestAddInvestment(t *testing.T) {
	_, _, investMgr, _, _ := setupTestEnvironment(t)

	inv := &Investment{
		Name:         "沪深300ETF",
		Type:         InvestmentTypeFund,
		Code:         "510300",
		Shares:       1000,
		CostBasis:    4.5,
		CurrentPrice: 4.8,
		BuyDate:      time.Now().AddDate(0, -3, 0),
	}

	err := investMgr.AddInvestment(inv)
	require.NoError(t, err)
	assert.NotEmpty(t, inv.ID)
	assert.InDelta(t, 4800, inv.CurrentValue, 0.01)
	assert.InDelta(t, 300, inv.GainLoss, 0.01)
}

func TestUpdatePrice(t *testing.T) {
	_, _, investMgr, _, _ := setupTestEnvironment(t)

	inv := &Investment{
		Name:         "测试基金",
		Type:         InvestmentTypeFund,
		Shares:       1000,
		CostBasis:    4.5,
		CurrentPrice: 4.5,
		BuyDate:      time.Now(),
	}
	_ = investMgr.AddInvestment(inv)

	err := investMgr.UpdatePrice(inv.ID, 5.0)
	require.NoError(t, err)

	got, _ := investMgr.GetInvestment(inv.ID)
	assert.Equal(t, float64(5.0), got.CurrentPrice)
	assert.InDelta(t, 500, got.GainLoss, 0.01)
}

func TestGetPortfolioSummary(t *testing.T) {
	_, _, investMgr, _, _ := setupTestEnvironment(t)

	_ = investMgr.AddInvestment(&Investment{
		Name: "基金A", Type: InvestmentTypeFund,
		Shares: 1000, CostBasis: 4.5, CurrentPrice: 4.8, BuyDate: time.Now(),
	})
	_ = investMgr.AddInvestment(&Investment{
		Name: "股票B", Type: InvestmentTypeStock,
		Shares: 100, CostBasis: 50, CurrentPrice: 55, BuyDate: time.Now(),
	})

	summary := investMgr.GetPortfolioSummary()
	assert.Equal(t, 2, summary["count"])
	assert.True(t, summary["total_value"].(float64) > 0)
}

func TestGetInvestmentRanking(t *testing.T) {
	_, _, investMgr, _, _ := setupTestEnvironment(t)

	_ = investMgr.AddInvestment(&Investment{
		Name: "低收益", Type: InvestmentTypeFund,
		Shares: 1000, CostBasis: 10, CurrentPrice: 10.5, BuyDate: time.Now(),
	})
	_ = investMgr.AddInvestment(&Investment{
		Name: "高收益", Type: InvestmentTypeStock,
		Shares: 100, CostBasis: 100, CurrentPrice: 150, BuyDate: time.Now(),
	})

	ranking := investMgr.GetInvestmentRanking()
	assert.Len(t, ranking, 2)
	assert.Equal(t, "高收益", ranking[0].Name)
}

// ========== 账单管理测试 ==========

func TestCreateBill(t *testing.T) {
	_, _, _, billMgr, _ := setupTestEnvironment(t)

	bill := &Bill{
		Name:     "房租",
		Amount:   3000,
		CategoryID: "cat-housing",
		Cycle:    BillCycleMonthly,
		DueDay:   1,
	}

	err := billMgr.CreateBill(bill)
	require.NoError(t, err)
	assert.NotEmpty(t, bill.ID)
	assert.False(t, bill.NextDueDate.IsZero())
}

func TestPayBill(t *testing.T) {
	engine, _, _, billMgr, _ := setupTestEnvironment(t)

	bill := &Bill{
		Name:     "水电费",
		Amount:   200,
		Cycle:    BillCycleMonthly,
		DueDay:   15,
	}
	_ = billMgr.CreateBill(bill)

	err := billMgr.PayBill(bill.ID, "acc-test")
	require.NoError(t, err)

	account, _ := engine.GetAccount("acc-test")
	assert.Equal(t, float64(9800), account.Balance)
}

func TestGetOverdueBills(t *testing.T) {
	_, _, _, billMgr, _ := setupTestEnvironment(t)

	// 创建一个已过期的账单
	bill := &Bill{
		Name:        "过期账单",
		Amount:      100,
		Cycle:       BillCycleMonthly,
		DueDay:      1,
		NextDueDate: time.Now().AddDate(0, -1, 0), // 上个月
	}
	_ = billMgr.CreateBill(bill)

	overdue := billMgr.GetOverdueBills()
	assert.Len(t, overdue, 1)
}

func TestGetBillSummary(t *testing.T) {
	_, _, _, billMgr, _ := setupTestEnvironment(t)

	_ = billMgr.CreateBill(&Bill{
		Name: "房租", Amount: 3000, Cycle: BillCycleMonthly, DueDay: 1,
	})
	_ = billMgr.CreateBill(&Bill{
		Name: "水电", Amount: 200, Cycle: BillCycleMonthly, DueDay: 15,
	})

	summary := billMgr.GetBillSummary()
	assert.Equal(t, 2, summary["total_bills"])
	assert.Equal(t, float64(3200), summary["total_monthly"])
}

// ========== 分析引擎测试 ==========

func TestGetFinancialSummary(t *testing.T) {
	engine, _, _, _, analytics := setupTestEnvironment(t)

	// 添加一些交易
	_ = engine.AddTransaction(&Transaction{
		AccountID: "acc-test", Type: TransactionTypeIncome,
		Amount: 8000, CategoryID: "cat-salary",
	})
	_ = engine.AddTransaction(&Transaction{
		AccountID: "acc-test", Type: TransactionTypeExpense,
		Amount: 3000, CategoryID: "cat-food",
	})

	start := time.Now().AddDate(0, -1, 0)
	end := time.Now()
	summary := analytics.GetFinancialSummary(start, end)

	assert.Equal(t, float64(8000), summary.TotalIncome)
	assert.Equal(t, float64(3000), summary.TotalExpense)
	assert.Equal(t, float64(5000), summary.NetIncome)
}

func TestGetSpendingByCategory(t *testing.T) {
	engine, _, _, _, analytics := setupTestEnvironment(t)

	_ = engine.AddTransaction(&Transaction{
		AccountID: "acc-test", Type: TransactionTypeExpense,
		Amount: 2000, CategoryID: "cat-food", CategoryName: "餐饮",
	})
	_ = engine.AddTransaction(&Transaction{
		AccountID: "acc-test", Type: TransactionTypeExpense,
		Amount: 1000, CategoryID: "cat-transport", CategoryName: "交通",
	})

	start := time.Now().AddDate(0, -1, 0)
	end := time.Now()
	categories := analytics.GetSpendingByCategory(start, end)

	assert.Len(t, categories, 2)
	assert.Equal(t, "cat-food", categories[0].CategoryID) // 按金额排序
}

func TestPredictCashFlow(t *testing.T) {
	engine, _, _, _, analytics := setupTestEnvironment(t)

	// 添加历史交易
	for i := 0; i < 6; i++ {
		_ = engine.AddTransaction(&Transaction{
			AccountID: "acc-test", Type: TransactionTypeIncome,
			Amount: 8000, CategoryID: "cat-salary",
			Date: time.Now().AddDate(0, -i-1, 0),
		})
		_ = engine.AddTransaction(&Transaction{
			AccountID: "acc-test", Type: TransactionTypeExpense,
			Amount: 5000, CategoryID: "cat-food",
			Date: time.Now().AddDate(0, -i-1, 0),
		})
	}

	forecast := analytics.PredictCashFlow(3)
	assert.Len(t, forecast.Predictions, 3)
	assert.True(t, forecast.Confidence > 0)
}
