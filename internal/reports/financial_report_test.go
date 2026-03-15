// Package reports 提供财务报告生成功能测试
package reports

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFinancialDataProvider 模拟财务数据提供者
type MockFinancialDataProvider struct {
	revenue   []RevenueRecord
	expense   []ExpenseRecord
	budget    []BudgetRecord
	cashFlow  []CashFlowRecord
	cost      []CostRecord
}

func (m *MockFinancialDataProvider) GetRevenueData(ctx context.Context, start, end time.Time) ([]RevenueRecord, error) {
	return m.revenue, nil
}

func (m *MockFinancialDataProvider) GetExpenseData(ctx context.Context, start, end time.Time) ([]ExpenseRecord, error) {
	return m.expense, nil
}

func (m *MockFinancialDataProvider) GetBudgetData(ctx context.Context, budgetIDs []string) ([]BudgetRecord, error) {
	return m.budget, nil
}

func (m *MockFinancialDataProvider) GetCashFlowData(ctx context.Context, start, end time.Time) ([]CashFlowRecord, error) {
	return m.cashFlow, nil
}

func (m *MockFinancialDataProvider) GetCostData(ctx context.Context, start, end time.Time) ([]CostRecord, error) {
	return m.cost, nil
}

func createMockProvider() *MockFinancialDataProvider {
	return &MockFinancialDataProvider{
		revenue: []RevenueRecord{
			{ID: "r1", Date: time.Now(), Category: "operating", Amount: 10000, Source: "服务收入"},
			{ID: "r2", Date: time.Now(), Category: "other", Amount: 2000, Source: "其他收入"},
		},
		expense: []ExpenseRecord{
			{ID: "e1", Date: time.Now(), Category: "operating", Amount: 5000, Department: "技术部"},
			{ID: "e2", Date: time.Now(), Category: "other", Amount: 1000, Department: "行政部"},
		},
		budget: []BudgetRecord{
			{ID: "b1", Name: "月度预算", Amount: 20000, Used: 15000, Remaining: 5000},
		},
		cashFlow: []CashFlowRecord{
			{ID: "cf1", Date: time.Now(), Type: "inflow", Category: "销售", Amount: 10000},
			{ID: "cf2", Date: time.Now(), Type: "outflow", Category: "采购", Amount: 5000},
		},
		cost: []CostRecord{
			{ID: "c1", Date: time.Now(), Resource: "存储", Type: "storage", Amount: 2000, Quantity: 100, UnitCost: 20},
		},
	}
}

func TestNewFinancialReportGenerator(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()

	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)
	require.NotNil(t, gen)
	assert.Equal(t, tmpDir, gen.storagePath)
}

func TestGenerateIncomeReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:         "测试收入报告",
		Type:         FinancialReportTypeIncome,
		PeriodStart:  time.Now().AddDate(0, -1, 0),
		PeriodEnd:    time.Now(),
		Currency:     CurrencyCNY,
		GenerateBy:   "test-user",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	assert.Equal(t, FinancialReportTypeIncome, report.Type)
	assert.Equal(t, 12000.0, report.Summary.TotalRevenue)
	assert.NotEmpty(t, report.Sections)
}

func TestGenerateExpenseReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试支出报告",
		Type:        FinancialReportTypeExpense,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
		GenerateBy:  "test-user",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	assert.Equal(t, 6000.0, report.Summary.TotalExpense)
	assert.NotEmpty(t, report.Sections)
}

func TestGenerateCashFlowReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试现金流报告",
		Type:        FinancialReportTypeCashFlow,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
		GenerateBy:  "test-user",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	assert.Equal(t, 5000.0, report.Summary.NetCashFlow) // 10000 - 5000
}

func TestGenerateBudgetReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试预算报告",
		Type:        FinancialReportTypeBudget,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
		GenerateBy:  "test-user",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	assert.Equal(t, 20000.0, report.Summary.BudgetAmount)
	assert.Equal(t, 15000.0, report.Summary.BudgetUsed)
	assert.InDelta(t, 75.0, report.Summary.BudgetUsageRate, 0.1)
}

func TestGenerateProfitLossReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试损益报告",
		Type:        FinancialReportTypeProfitLoss,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
		GenerateBy:  "test-user",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	// 净利润 = 收入 - 支出 = 12000 - 6000 = 6000
	assert.Equal(t, 6000.0, report.Summary.NetProfit)
	// 利润率 = 6000 / 12000 * 100 = 50%
	assert.InDelta(t, 50.0, report.Summary.ProfitMargin, 0.1)
}

func TestGenerateCostAnalysisReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试成本分析报告",
		Type:        FinancialReportTypeCostAnalysis,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
		GenerateBy:  "test-user",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	assert.Equal(t, 2000.0, report.Summary.TotalExpense)
}

func TestGenerateComprehensiveReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:          "测试综合报告",
		Type:          FinancialReportTypeCustom,
		PeriodStart:   time.Now().AddDate(0, -1, 0),
		PeriodEnd:     time.Now(),
		Currency:      CurrencyCNY,
		GenerateBy:    "test-user",
		IncludeCharts: true,
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, ReportStatusCompleted, report.Status)
	assert.NotEmpty(t, report.Sections)
	// 验证关键指标区块存在
	found := false
	for _, section := range report.Sections {
		if section.ID == "kpi" {
			found = true
			break
		}
	}
	assert.True(t, found, "应该包含关键指标区块")
}

func TestGetReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试获取",
		Type:        FinancialReportTypeIncome,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
	}

	created, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	// 获取报告
	retrieved, err := gen.GetReport(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)

	// 获取不存在的报告
	_, err = gen.GetReport(ctx, "non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrFinancialReportNotFound, err)
}

func TestQueryReports(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// 创建多个报告
	for i := 0; i < 5; i++ {
		req := FinancialReportRequest{
			Name:        string(rune('A' + i)),
			Type:        FinancialReportTypeIncome,
			PeriodStart: time.Now().AddDate(0, -1, 0),
			PeriodEnd:   time.Now(),
			Currency:    CurrencyCNY,
		}
		_, err := gen.GenerateReport(ctx, req)
		require.NoError(t, err)
	}

	// 查询所有
	results, total, err := gen.QueryReports(ctx, FinancialReportQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, results, 5)

	// 按类型查询
	results, total, err = gen.QueryReports(ctx, FinancialReportQuery{
		Types: []FinancialReportType{FinancialReportTypeIncome},
	})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// 分页查询
	results, total, err = gen.QueryReports(ctx, FinancialReportQuery{
		Page:     1,
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, results, 2)
}

func TestDeleteReport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试删除",
		Type:        FinancialReportTypeIncome,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	// 删除
	err = gen.DeleteReport(ctx, report.ID)
	require.NoError(t, err)

	// 确认删除
	_, err = gen.GetReport(ctx, report.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrFinancialReportNotFound, err)
}

func TestReportExport(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:         "测试导出",
		Type:         FinancialReportTypeIncome,
		PeriodStart:  time.Now().AddDate(0, -1, 0),
		PeriodEnd:    time.Now(),
		Currency:     CurrencyCNY,
		ExportFormat: ExportJSON,
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, report.ExportPath)

	// 测试Excel导出
	req.ExportFormat = ExportExcel
	report2, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, report2.ExportPath)
}

func TestReportWithCharts(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:          "测试图表",
		Type:          FinancialReportTypeIncome,
		PeriodStart:   time.Now().AddDate(0, -1, 0),
		PeriodEnd:     time.Now(),
		Currency:      CurrencyCNY,
		IncludeCharts: true,
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, report.Charts)
}

func TestReportPersistence(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()

	// 创建第一个生成器
	gen1, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "持久化测试",
		Type:        FinancialReportTypeIncome,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
	}

	report, err := gen1.GenerateReport(ctx, req)
	require.NoError(t, err)
	reportID := report.ID

	// 创建新的生成器，应该能加载之前的报告
	gen2, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	// 验证数据持久化
	retrieved, err := gen2.GetReport(ctx, reportID)
	require.NoError(t, err)
	assert.Equal(t, "持久化测试", retrieved.Name)
}

func TestDefaultTemplates(t *testing.T) {
	templates := GetFinReportDefaultTemplates()
	assert.NotEmpty(t, templates)

	// 检查包含必要类型
	types := make(map[FinancialReportType]bool)
	for _, tmpl := range templates {
		types[tmpl.Type] = true
	}

	assert.True(t, types[FinancialReportTypeIncome])
	assert.True(t, types[FinancialReportTypeExpense])
	assert.True(t, types[FinancialReportTypeBudget])
	assert.True(t, types[FinancialReportTypeProfitLoss])
}

func TestReportSections(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试区块",
		Type:        FinancialReportTypeProfitLoss,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)

	// 验证区块结构
	assert.NotEmpty(t, report.Sections)

	for _, section := range report.Sections {
		assert.NotEmpty(t, section.ID)
		assert.NotEmpty(t, section.Title)
		assert.NotEmpty(t, section.Type)
	}
}

func TestReportDuration(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:        "测试耗时",
		Type:        FinancialReportTypeIncome,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		Currency:    CurrencyCNY,
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)
	assert.Greater(t, report.Duration, int64(0))
}

func TestReportAuditTrail(t *testing.T) {
	provider := createMockProvider()
	tmpDir := t.TempDir()
	gen, err := NewFinancialReportGenerator(provider, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	req := FinancialReportRequest{
		Name:       "测试审计",
		Type:       FinancialReportTypeIncome,
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:  time.Now(),
		Currency:   CurrencyCNY,
		GenerateBy: "auditor",
	}

	report, err := gen.GenerateReport(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "auditor", report.GeneratedBy)
	assert.NotZero(t, report.GeneratedAt)
}