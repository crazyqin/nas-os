// Package reports 提供财务报告生成功能
package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ========== 错误定义 ==========

var (
	ErrFinancialReportNotFound = errors.New("财务报告不存在")
	ErrReportGenerationFailed  = errors.New("报告生成失败")
	ErrInvalidReportParams     = errors.New("无效的报告参数")
)

// ========== 财务报告类型定义 ==========

// FinancialReportType 财务报告类型
type FinancialReportType string

const (
	FinancialReportTypeIncome       FinancialReportType = "income"        // 收入报告
	FinancialReportTypeExpense      FinancialReportType = "expense"       // 支出报告
	FinancialReportTypeCashFlow     FinancialReportType = "cash_flow"     // 现金流报告
	FinancialReportTypeBudget       FinancialReportType = "budget"        // 预算执行报告
	FinancialReportTypeCostAnalysis FinancialReportType = "cost_analysis" // 成本分析报告
	FinancialReportTypeProfitLoss   FinancialReportType = "profit_loss"   // 损益报告
	FinancialReportTypeBalanceSheet FinancialReportType = "balance_sheet" // 资产负债表
	FinancialReportTypeCustom       FinancialReportType = "custom"        // 自定义报告
)

// ReportStatus 报告状态
type ReportStatus string

const (
	ReportStatusPending    ReportStatus = "pending"    // 待生成
	ReportStatusGenerating ReportStatus = "generating" // 生成中
	ReportStatusCompleted  ReportStatus = "completed"  // 已完成
	ReportStatusFailed     ReportStatus = "failed"     // 失败
)

// Currency 货币类型
type Currency string

const (
	CurrencyCNY Currency = "CNY" // 人民币
	CurrencyUSD Currency = "USD" // 美元
	CurrencyEUR Currency = "EUR" // 欧元
	CurrencyJPY Currency = "JPY" // 日元
)

// ========== 财务报告数据结构 ==========

// FinancialReport 财务报告
type FinancialReport struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        FinancialReportType `json:"type"`
	Status      ReportStatus        `json:"status"`
	Description string              `json:"description"`
	Currency    Currency            `json:"currency"`

	// 时间范围
	Period ReportPeriod `json:"period"`

	// 报告数据
	Summary  FinancialSummary `json:"summary"`
	Sections []ReportSection  `json:"sections"`
	Charts   []FinChartConfig `json:"charts,omitempty"`

	// 元数据
	GeneratedAt time.Time `json:"generated_at"`
	GeneratedBy string    `json:"generated_by"`
	Duration    int64     `json:"duration"` // 生成耗时(ms)
	Version     string    `json:"version"`
	Tags        []string  `json:"tags"`

	// 导出信息
	ExportPath   string       `json:"export_path,omitempty"`
	ExportFormat ExportFormat `json:"export_format"`

	// 审计信息
	AuditTrail []AuditEntry `json:"audit_trail,omitempty"`
	ApprovedBy string       `json:"approved_by,omitempty"`
	ApprovedAt *time.Time   `json:"approved_at,omitempty"`

	// 错误信息
	ErrorMessage string `json:"error_message,omitempty"`
}

// FinancialSummary 财务摘要
type FinancialSummary struct {
	// 收入
	TotalRevenue     float64 `json:"total_revenue"`
	OperatingRevenue float64 `json:"operating_revenue"`
	OtherRevenue     float64 `json:"other_revenue"`

	// 支出
	TotalExpense     float64 `json:"total_expense"`
	OperatingExpense float64 `json:"operating_expense"`
	OtherExpense     float64 `json:"other_expense"`

	// 利润
	GrossProfit     float64 `json:"gross_profit"`
	OperatingProfit float64 `json:"operating_profit"`
	NetProfit       float64 `json:"net_profit"`

	// 现金流
	NetCashFlow       float64 `json:"net_cash_flow"`
	OperatingCashFlow float64 `json:"operating_cash_flow"`
	InvestingCashFlow float64 `json:"investing_cash_flow"`
	FinancingCashFlow float64 `json:"financing_cash_flow"`

	// 预算
	BudgetAmount    float64 `json:"budget_amount"`
	BudgetUsed      float64 `json:"budget_used"`
	BudgetVariance  float64 `json:"budget_variance"`
	BudgetUsageRate float64 `json:"budget_usage_rate"`

	// 比率
	ProfitMargin       float64 `json:"profit_margin"`
	ExpenseRatio       float64 `json:"expense_ratio"`
	ReturnOnInvestment float64 `json:"return_on_investment"`

	// 同比/环比
	YoYGrowth float64 `json:"yoy_growth"` // 同比增长
	MoMGrowth float64 `json:"mom_growth"` // 环比增长
}

// SectionType 区块类型
type SectionType string

const (
	SectionTypeSummary SectionType = "summary"
	SectionTypeItems   SectionType = "items"
	SectionTypeChart   SectionType = "chart"
	SectionTypeTable   SectionType = "table"
	SectionTypeText    SectionType = "text"
)

// ReportSection 报告区块
type ReportSection struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Type      SectionType        `json:"type"`
	Position  int                `json:"position"`
	Visible   bool               `json:"visible"`
	Summary   string             `json:"summary,omitempty"`
	Data      []SectionDataRow   `json:"data,omitempty"`
	Subtotals map[string]float64 `json:"subtotals,omitempty"`
	Children  []ReportSection    `json:"children,omitempty"`
	Style     SectionStyle       `json:"style,omitempty"`
}

// SectionDataRow 区块数据行
type SectionDataRow struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Values   map[string]float64     `json:"values"`
	Percent  float64                `json:"percent,omitempty"`
	Trend    string                 `json:"trend,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SectionStyle 区块样式
type SectionStyle struct {
	Highlight bool   `json:"highlight"`
	Color     string `json:"color,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Format    string `json:"format,omitempty"`
}

// FinChartConfig 财务图表配置
type FinChartConfig struct {
	ID         string                 `json:"id"`
	Type       FinChartType           `json:"type"`
	Title      string                 `json:"title"`
	DataSource string                 `json:"data_source"`
	Options    map[string]interface{} `json:"options"`
	Position   FinChartPosition       `json:"position"`
}

// FinChartType 财务图表类型
type FinChartType string

const (
	FinChartTypeLine    FinChartType = "line"
	FinChartTypeBar     FinChartType = "bar"
	FinChartTypePie     FinChartType = "pie"
	FinChartTypeArea    FinChartType = "area"
	FinChartTypeScatter FinChartType = "scatter"
	FinChartTypeTable   FinChartType = "table"
)

// FinChartPosition 财务图表位置
type FinChartPosition struct {
	Row    int `json:"row"`
	Col    int `json:"col"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// AuditEntry 审计条目
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	User      string    `json:"user"`
	Details   string    `json:"details"`
}

// ========== 报告生成配置 ==========

// FinancialReportConfig 报告配置
type FinancialReportConfig struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Type          FinancialReportType `json:"type"`
	Description   string              `json:"description"`
	TemplateID    string              `json:"template_id,omitempty"`
	Sections      []SectionConfig     `json:"sections"`
	DataSources   []DataSourceConfig  `json:"data_sources"`
	Filters       []ReportFilter      `json:"filters,omitempty"`
	Schedule      *ScheduleConfig     `json:"schedule,omitempty"`
	ExportFormats []ExportFormat      `json:"export_formats"`
	Recipients    []string            `json:"recipients,omitempty"`
	IsPublic      bool                `json:"is_public"`
	CreatedAt     time.Time           `json:"created_at"`
	CreatedBy     string              `json:"created_by"`
}

// SectionConfig 区块配置
type SectionConfig struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Type       string   `json:"type"`
	DataSource string   `json:"data_source"`
	Query      string   `json:"query,omitempty"`
	Fields     []string `json:"fields"`
	SortBy     string   `json:"sort_by,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Visible    bool     `json:"visible"`
}

// DataSourceConfig 数据源配置
type DataSourceConfig struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Endpoint string                 `json:"endpoint,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// ReportFilter 报告过滤器
type ReportFilter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Frequency string     `json:"frequency"`
	Time      string     `json:"time"`
	Timezone  string     `json:"timezone"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	Enabled   bool       `json:"enabled"`
}

// ========== 报告请求 ==========

// FinancialReportRequest 财务报告请求
type FinancialReportRequest struct {
	Name          string              `json:"name"`
	Type          FinancialReportType `json:"type"`
	PeriodStart   time.Time           `json:"period_start"`
	PeriodEnd     time.Time           `json:"period_end"`
	Currency      Currency            `json:"currency"`
	Sections      []string            `json:"sections,omitempty"` // 要包含的区块
	Filters       []ReportFilter      `json:"filters,omitempty"`
	ComparePeriod bool                `json:"compare_period"` // 是否对比上期
	IncludeCharts bool                `json:"include_charts"`
	ExportFormat  ExportFormat        `json:"export_format"`
	GenerateBy    string              `json:"generate_by"`
	Tags          []string            `json:"tags"`
}

// FinancialReportQuery 财务报告查询
type FinancialReportQuery struct {
	IDs         []string              `json:"ids,omitempty"`
	Types       []FinancialReportType `json:"types,omitempty"`
	Statuses    []ReportStatus        `json:"statuses,omitempty"`
	StartDate   *time.Time            `json:"start_date,omitempty"`
	EndDate     *time.Time            `json:"end_date,omitempty"`
	GeneratedBy []string              `json:"generated_by,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"page_size"`
	SortBy      string                `json:"sort_by"`
	SortOrder   string                `json:"sort_order"`
}

// ========== 财务报告生成器 ==========

// FinancialReportGenerator 财务报告生成器
type FinancialReportGenerator struct {
	mu           sync.RWMutex
	reports      map[string]*FinancialReport
	configs      map[string]*FinancialReportConfig
	storagePath  string
	dataProvider FinancialDataProvider
}

// FinancialDataProvider 财务数据提供者接口
type FinancialDataProvider interface {
	// 获取收入数据
	GetRevenueData(ctx context.Context, start, end time.Time) ([]RevenueRecord, error)
	// 获取支出数据
	GetExpenseData(ctx context.Context, start, end time.Time) ([]ExpenseRecord, error)
	// 获取预算数据
	GetBudgetData(ctx context.Context, budgetIDs []string) ([]BudgetRecord, error)
	// 获取现金流数据
	GetCashFlowData(ctx context.Context, start, end time.Time) ([]CashFlowRecord, error)
	// 获取成本数据
	GetCostData(ctx context.Context, start, end time.Time) ([]CostRecord, error)
}

// RevenueRecord 收入记录
type RevenueRecord struct {
	ID          string    `json:"id"`
	Date        time.Time `json:"date"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Source      string    `json:"source"`
	Description string    `json:"description"`
}

// ExpenseRecord 支出记录
type ExpenseRecord struct {
	ID          string    `json:"id"`
	Date        time.Time `json:"date"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Department  string    `json:"department"`
	Description string    `json:"description"`
}

// BudgetRecord 预算记录
type BudgetRecord struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"`
	Used      float64 `json:"used"`
	Remaining float64 `json:"remaining"`
	Period    string  `json:"period"`
}

// CashFlowRecord 现金流记录
type CashFlowRecord struct {
	ID          string    `json:"id"`
	Date        time.Time `json:"date"`
	Type        string    `json:"type"` // inflow, outflow
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
}

// CostRecord 成本记录
type CostRecord struct {
	ID       string    `json:"id"`
	Date     time.Time `json:"date"`
	Resource string    `json:"resource"`
	Type     string    `json:"type"`
	Amount   float64   `json:"amount"`
	Quantity float64   `json:"quantity"`
	UnitCost float64   `json:"unit_cost"`
}

// NewFinancialReportGenerator 创建财务报告生成器
func NewFinancialReportGenerator(dataProvider FinancialDataProvider, storagePath string) (*FinancialReportGenerator, error) {
	if storagePath == "" {
		storagePath = "./data/financial_reports"
	}

	gen := &FinancialReportGenerator{
		reports:      make(map[string]*FinancialReport),
		configs:      make(map[string]*FinancialReportConfig),
		storagePath:  storagePath,
		dataProvider: dataProvider,
	}

	// 确保存储目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 加载已有报告
	if err := gen.loadReports(); err != nil {
		return nil, fmt.Errorf("加载报告失败: %w", err)
	}

	return gen, nil
}

// loadReports 加载已保存的报告
func (g *FinancialReportGenerator) loadReports() error {
	files, err := filepath.Glob(filepath.Join(g.storagePath, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var report FinancialReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}

		g.reports[report.ID] = &report
	}

	return nil
}

// GenerateReport 生成财务报告
func (g *FinancialReportGenerator) GenerateReport(ctx context.Context, req FinancialReportRequest) (*FinancialReport, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	startTime := time.Now()
	reportID := uuid.New().String()

	report := &FinancialReport{
		ID:          reportID,
		Name:        req.Name,
		Type:        req.Type,
		Status:      ReportStatusGenerating,
		Description: fmt.Sprintf("%s (%s ~ %s)", req.Name, req.PeriodStart.Format("2006-01-02"), req.PeriodEnd.Format("2006-01-02")),
		Currency:    req.Currency,
		Period: ReportPeriod{
			StartTime: req.PeriodStart,
			EndTime:   req.PeriodEnd,
		},
		GeneratedAt:  startTime,
		GeneratedBy:  req.GenerateBy,
		Version:      "1.0",
		Tags:         req.Tags,
		ExportFormat: req.ExportFormat,
	}

	// 保存初始状态
	g.reports[reportID] = report

	// 根据报告类型生成内容
	var err error
	switch req.Type {
	case FinancialReportTypeIncome:
		err = g.generateIncomeReport(ctx, report, req)
	case FinancialReportTypeExpense:
		err = g.generateExpenseReport(ctx, report, req)
	case FinancialReportTypeCashFlow:
		err = g.generateCashFlowReport(ctx, report, req)
	case FinancialReportTypeBudget:
		err = g.generateBudgetReport(ctx, report, req)
	case FinancialReportTypeCostAnalysis:
		err = g.generateCostAnalysisReport(ctx, report, req)
	case FinancialReportTypeProfitLoss:
		err = g.generateProfitLossReport(ctx, report, req)
	default:
		err = g.generateComprehensiveReport(ctx, report, req)
	}

	if err != nil {
		report.Status = ReportStatusFailed
		report.ErrorMessage = err.Error()
		report.Duration = time.Since(startTime).Milliseconds()
		g.saveReport(report)
		return report, fmt.Errorf("生成报告失败: %w", err)
	}

	report.Status = ReportStatusCompleted
	report.Duration = time.Since(startTime).Milliseconds()

	// 保存报告
	if err := g.saveReport(report); err != nil {
		return report, fmt.Errorf("保存报告失败: %w", err)
	}

	// 导出报告
	if req.ExportFormat != "" {
		exportPath, err := g.exportReport(report, req.ExportFormat)
		if err == nil {
			report.ExportPath = exportPath
		}
	}

	return report, nil
}

// generateIncomeReport 生成收入报告
func (g *FinancialReportGenerator) generateIncomeReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	data, err := g.dataProvider.GetRevenueData(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return err
	}

	// 计算摘要
	summary := FinancialSummary{}
	categoryTotals := make(map[string]float64)
	sourceTotals := make(map[string]float64)

	for _, r := range data {
		summary.TotalRevenue += r.Amount
		categoryTotals[r.Category] += r.Amount
		sourceTotals[r.Source] += r.Amount
	}

	summary.OperatingRevenue = categoryTotals["operating"]
	summary.OtherRevenue = categoryTotals["other"]
	summary.GrossProfit = summary.TotalRevenue

	report.Summary = summary

	// 构建区块
	report.Sections = []ReportSection{
		{
			ID:       "revenue_summary",
			Title:    "收入概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("本期总收入: %.2f %s", summary.TotalRevenue, report.Currency),
		},
		{
			ID:        "revenue_by_category",
			Title:     "收入分类明细",
			Type:      SectionTypeItems,
			Position:  2,
			Visible:   true,
			Data:      g.buildCategoryData(categoryTotals, summary.TotalRevenue),
			Subtotals: map[string]float64{"total": summary.TotalRevenue},
		},
		{
			ID:        "revenue_by_source",
			Title:     "收入来源明细",
			Type:      SectionTypeItems,
			Position:  3,
			Visible:   true,
			Data:      g.buildCategoryData(sourceTotals, summary.TotalRevenue),
			Subtotals: map[string]float64{"total": summary.TotalRevenue},
		},
	}

	// 添加图表配置
	if req.IncludeCharts {
		report.Charts = []FinChartConfig{
			{
				ID:         "revenue_pie",
				Type:       FinChartTypePie,
				Title:      "收入分类占比",
				DataSource: "revenue_by_category",
				Position:   FinChartPosition{Row: 1, Col: 1, Width: 6, Height: 4},
			},
			{
				ID:         "revenue_trend",
				Type:       FinChartTypeLine,
				Title:      "收入趋势",
				DataSource: "revenue_trend",
				Position:   FinChartPosition{Row: 1, Col: 7, Width: 6, Height: 4},
			},
		}
	}

	return nil
}

// generateExpenseReport 生成支出报告
func (g *FinancialReportGenerator) generateExpenseReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	data, err := g.dataProvider.GetExpenseData(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return err
	}

	summary := FinancialSummary{}
	categoryTotals := make(map[string]float64)
	departmentTotals := make(map[string]float64)

	for _, e := range data {
		summary.TotalExpense += e.Amount
		categoryTotals[e.Category] += e.Amount
		departmentTotals[e.Department] += e.Amount
	}

	summary.OperatingExpense = categoryTotals["operating"]
	summary.OtherExpense = categoryTotals["other"]

	if summary.TotalExpense > 0 {
		summary.ExpenseRatio = summary.OperatingExpense / summary.TotalExpense * 100
	}

	report.Summary = summary

	report.Sections = []ReportSection{
		{
			ID:       "expense_summary",
			Title:    "支出概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("本期总支出: %.2f %s", summary.TotalExpense, report.Currency),
		},
		{
			ID:        "expense_by_category",
			Title:     "支出分类明细",
			Type:      SectionTypeItems,
			Position:  2,
			Visible:   true,
			Data:      g.buildCategoryData(categoryTotals, summary.TotalExpense),
			Subtotals: map[string]float64{"total": summary.TotalExpense},
		},
		{
			ID:        "expense_by_department",
			Title:     "部门支出明细",
			Type:      SectionTypeItems,
			Position:  3,
			Visible:   true,
			Data:      g.buildCategoryData(departmentTotals, summary.TotalExpense),
			Subtotals: map[string]float64{"total": summary.TotalExpense},
		},
	}

	if req.IncludeCharts {
		report.Charts = []FinChartConfig{
			{
				ID:         "expense_pie",
				Type:       FinChartTypePie,
				Title:      "支出分类占比",
				DataSource: "expense_by_category",
				Position:   FinChartPosition{Row: 1, Col: 1, Width: 6, Height: 4},
			},
			{
				ID:         "expense_bar",
				Type:       FinChartTypeBar,
				Title:      "部门支出对比",
				DataSource: "expense_by_department",
				Position:   FinChartPosition{Row: 1, Col: 7, Width: 6, Height: 4},
			},
		}
	}

	return nil
}

// generateCashFlowReport 生成现金流报告
func (g *FinancialReportGenerator) generateCashFlowReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	data, err := g.dataProvider.GetCashFlowData(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return err
	}

	summary := FinancialSummary{}
	inflowByCategory := make(map[string]float64)
	outflowByCategory := make(map[string]float64)

	for _, cf := range data {
		if cf.Type == "inflow" {
			summary.OperatingCashFlow += cf.Amount
			inflowByCategory[cf.Category] += cf.Amount
		} else {
			summary.OperatingCashFlow -= cf.Amount
			outflowByCategory[cf.Category] += cf.Amount
		}
	}

	summary.NetCashFlow = summary.OperatingCashFlow + summary.InvestingCashFlow + summary.FinancingCashFlow

	report.Summary = summary

	report.Sections = []ReportSection{
		{
			ID:       "cashflow_summary",
			Title:    "现金流概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("净现金流: %.2f %s", summary.NetCashFlow, report.Currency),
		},
		{
			ID:       "cashflow_inflow",
			Title:    "现金流入明细",
			Type:     SectionTypeItems,
			Position: 2,
			Visible:  true,
			Data:     g.buildCategoryData(inflowByCategory, summary.OperatingCashFlow),
		},
		{
			ID:       "cashflow_outflow",
			Title:    "现金流出明细",
			Type:     SectionTypeItems,
			Position: 3,
			Visible:  true,
			Data:     g.buildCategoryData(outflowByCategory, 0),
		},
	}

	return nil
}

// generateBudgetReport 生成预算执行报告
func (g *FinancialReportGenerator) generateBudgetReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	data, err := g.dataProvider.GetBudgetData(ctx, nil)
	if err != nil {
		return err
	}

	summary := FinancialSummary{}
	budgetDetails := []SectionDataRow{}

	for _, b := range data {
		summary.BudgetAmount += b.Amount
		summary.BudgetUsed += b.Used

		usageRate := 0.0
		if b.Amount > 0 {
			usageRate = b.Used / b.Amount * 100
		}

		budgetDetails = append(budgetDetails, SectionDataRow{
			ID:      b.ID,
			Label:   b.Name,
			Values:  map[string]float64{"budget": b.Amount, "used": b.Used, "remaining": b.Remaining},
			Percent: usageRate,
		})
	}

	if summary.BudgetAmount > 0 {
		summary.BudgetUsageRate = summary.BudgetUsed / summary.BudgetAmount * 100
		summary.BudgetVariance = summary.BudgetAmount - summary.BudgetUsed
	}

	report.Summary = summary

	report.Sections = []ReportSection{
		{
			ID:       "budget_summary",
			Title:    "预算执行概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("预算使用率: %.1f%%", summary.BudgetUsageRate),
		},
		{
			ID:        "budget_details",
			Title:     "预算执行明细",
			Type:      SectionTypeItems,
			Position:  2,
			Visible:   true,
			Data:      budgetDetails,
			Subtotals: map[string]float64{"budget": summary.BudgetAmount, "used": summary.BudgetUsed},
		},
	}

	if req.IncludeCharts {
		report.Charts = []FinChartConfig{
			{
				ID:         "budget_usage",
				Type:       FinChartTypeBar,
				Title:      "预算使用情况",
				DataSource: "budget_details",
				Position:   FinChartPosition{Row: 1, Col: 1, Width: 12, Height: 4},
			},
		}
	}

	return nil
}

// generateCostAnalysisReport 生成成本分析报告
func (g *FinancialReportGenerator) generateCostAnalysisReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	data, err := g.dataProvider.GetCostData(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return err
	}

	summary := FinancialSummary{}
	resourceTotals := make(map[string]float64)
	typeTotals := make(map[string]float64)

	for _, c := range data {
		summary.TotalExpense += c.Amount
		resourceTotals[c.Resource] += c.Amount
		typeTotals[c.Type] += c.Amount
	}

	report.Summary = summary

	report.Sections = []ReportSection{
		{
			ID:       "cost_summary",
			Title:    "成本概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("总成本: %.2f %s", summary.TotalExpense, report.Currency),
		},
		{
			ID:       "cost_by_resource",
			Title:    "资源成本明细",
			Type:     SectionTypeItems,
			Position: 2,
			Visible:  true,
			Data:     g.buildCategoryData(resourceTotals, summary.TotalExpense),
		},
		{
			ID:       "cost_by_type",
			Title:    "成本类型明细",
			Type:     SectionTypeItems,
			Position: 3,
			Visible:  true,
			Data:     g.buildCategoryData(typeTotals, summary.TotalExpense),
		},
	}

	return nil
}

// generateProfitLossReport 生成损益报告
func (g *FinancialReportGenerator) generateProfitLossReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	// 获取收入数据
	revenueData, err := g.dataProvider.GetRevenueData(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return err
	}

	// 获取支出数据
	expenseData, err := g.dataProvider.GetExpenseData(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return err
	}

	summary := FinancialSummary{}

	// 计算收入
	for _, r := range revenueData {
		summary.TotalRevenue += r.Amount
		if r.Category == "operating" {
			summary.OperatingRevenue += r.Amount
		} else {
			summary.OtherRevenue += r.Amount
		}
	}

	// 计算支出
	for _, e := range expenseData {
		summary.TotalExpense += e.Amount
		if e.Category == "operating" {
			summary.OperatingExpense += e.Amount
		} else {
			summary.OtherExpense += e.Amount
		}
	}

	// 计算利润
	summary.GrossProfit = summary.TotalRevenue - summary.OperatingExpense
	summary.OperatingProfit = summary.GrossProfit - summary.OtherExpense
	summary.NetProfit = summary.OperatingProfit

	// 计算利润率
	if summary.TotalRevenue > 0 {
		summary.ProfitMargin = summary.NetProfit / summary.TotalRevenue * 100
	}

	report.Summary = summary

	report.Sections = []ReportSection{
		{
			ID:       "pl_summary",
			Title:    "损益概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("净利润: %.2f %s (利润率: %.1f%%)", summary.NetProfit, report.Currency, summary.ProfitMargin),
		},
		{
			ID:       "pl_revenue",
			Title:    "收入",
			Type:     SectionTypeItems,
			Position: 2,
			Visible:  true,
			Data: []SectionDataRow{
				{ID: "operating_revenue", Label: "营业收入", Values: map[string]float64{"amount": summary.OperatingRevenue}},
				{ID: "other_revenue", Label: "其他收入", Values: map[string]float64{"amount": summary.OtherRevenue}},
				{ID: "total_revenue", Label: "收入合计", Values: map[string]float64{"amount": summary.TotalRevenue}},
			},
		},
		{
			ID:       "pl_expense",
			Title:    "支出",
			Type:     SectionTypeItems,
			Position: 3,
			Visible:  true,
			Data: []SectionDataRow{
				{ID: "operating_expense", Label: "营业成本", Values: map[string]float64{"amount": summary.OperatingExpense}},
				{ID: "other_expense", Label: "其他支出", Values: map[string]float64{"amount": summary.OtherExpense}},
				{ID: "total_expense", Label: "支出合计", Values: map[string]float64{"amount": summary.TotalExpense}},
			},
		},
		{
			ID:       "pl_profit",
			Title:    "利润",
			Type:     SectionTypeItems,
			Position: 4,
			Visible:  true,
			Data: []SectionDataRow{
				{ID: "gross_profit", Label: "毛利润", Values: map[string]float64{"amount": summary.GrossProfit}},
				{ID: "operating_profit", Label: "营业利润", Values: map[string]float64{"amount": summary.OperatingProfit}},
				{ID: "net_profit", Label: "净利润", Values: map[string]float64{"amount": summary.NetProfit}},
			},
		},
	}

	return nil
}

// generateComprehensiveReport 生成综合财务报告
func (g *FinancialReportGenerator) generateComprehensiveReport(ctx context.Context, report *FinancialReport, req FinancialReportRequest) error {
	// 获取所有数据
	revenueData, _ := g.dataProvider.GetRevenueData(ctx, req.PeriodStart, req.PeriodEnd)
	expenseData, _ := g.dataProvider.GetExpenseData(ctx, req.PeriodStart, req.PeriodEnd)
	budgetData, _ := g.dataProvider.GetBudgetData(ctx, nil)
	cashFlowData, _ := g.dataProvider.GetCashFlowData(ctx, req.PeriodStart, req.PeriodEnd)
	costData, _ := g.dataProvider.GetCostData(ctx, req.PeriodStart, req.PeriodEnd)

	summary := FinancialSummary{}

	// 计算收入
	for _, r := range revenueData {
		summary.TotalRevenue += r.Amount
	}

	// 计算支出
	for _, e := range expenseData {
		summary.TotalExpense += e.Amount
	}

	// 计算现金流
	for _, cf := range cashFlowData {
		if cf.Type == "inflow" {
			summary.OperatingCashFlow += cf.Amount
		} else {
			summary.OperatingCashFlow -= cf.Amount
		}
	}

	// 计算预算
	for _, b := range budgetData {
		summary.BudgetAmount += b.Amount
		summary.BudgetUsed += b.Used
	}

	// 计算利润
	summary.NetProfit = summary.TotalRevenue - summary.TotalExpense
	summary.NetCashFlow = summary.OperatingCashFlow

	// 计算比率
	if summary.TotalRevenue > 0 {
		summary.ProfitMargin = summary.NetProfit / summary.TotalRevenue * 100
	}
	if summary.BudgetAmount > 0 {
		summary.BudgetUsageRate = summary.BudgetUsed / summary.BudgetAmount * 100
	}

	report.Summary = summary

	// 构建综合区块
	report.Sections = []ReportSection{
		{
			ID:       "overview",
			Title:    "财务概览",
			Type:     SectionTypeSummary,
			Position: 1,
			Visible:  true,
			Summary:  fmt.Sprintf("收入: %.2f | 支出: %.2f | 净利润: %.2f", summary.TotalRevenue, summary.TotalExpense, summary.NetProfit),
		},
		{
			ID:       "kpi",
			Title:    "关键指标",
			Type:     SectionTypeItems,
			Position: 2,
			Visible:  true,
			Data: []SectionDataRow{
				{ID: "revenue", Label: "总收入", Values: map[string]float64{"amount": summary.TotalRevenue}},
				{ID: "expense", Label: "总支出", Values: map[string]float64{"amount": summary.TotalExpense}},
				{ID: "profit", Label: "净利润", Values: map[string]float64{"amount": summary.NetProfit}},
				{ID: "cashflow", Label: "净现金流", Values: map[string]float64{"amount": summary.NetCashFlow}},
				{ID: "margin", Label: "利润率", Percent: summary.ProfitMargin},
				{ID: "budget_usage", Label: "预算使用率", Percent: summary.BudgetUsageRate},
			},
		},
	}

	// 添加成本分析
	if len(costData) > 0 {
		costTotals := make(map[string]float64)
		for _, c := range costData {
			costTotals[c.Resource] += c.Amount
		}
		report.Sections = append(report.Sections, ReportSection{
			ID:       "cost_analysis",
			Title:    "成本分析",
			Type:     SectionTypeItems,
			Position: 3,
			Visible:  true,
			Data:     g.buildCategoryData(costTotals, summary.TotalExpense),
		})
	}

	return nil
}

// buildCategoryData 构建分类数据
func (g *FinancialReportGenerator) buildCategoryData(totals map[string]float64, grandTotal float64) []SectionDataRow {
	rows := []SectionDataRow{}
	for category, amount := range totals {
		percent := 0.0
		if grandTotal > 0 {
			percent = amount / grandTotal * 100
		}
		rows = append(rows, SectionDataRow{
			ID:      category,
			Label:   category,
			Values:  map[string]float64{"amount": amount},
			Percent: percent,
		})
	}
	return rows
}

// GetReport 获取报告
func (g *FinancialReportGenerator) GetReport(ctx context.Context, id string) (*FinancialReport, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	report, exists := g.reports[id]
	if !exists {
		return nil, ErrFinancialReportNotFound
	}

	return report, nil
}

// QueryReports 查询报告
func (g *FinancialReportGenerator) QueryReports(ctx context.Context, query FinancialReportQuery) ([]*FinancialReport, int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var results []*FinancialReport

	for _, report := range g.reports {
		// 应用过滤条件
		if len(query.IDs) > 0 {
			found := false
			for _, id := range query.IDs {
				if report.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if report.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Statuses) > 0 {
			found := false
			for _, s := range query.Statuses {
				if report.Status == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if query.StartDate != nil && report.GeneratedAt.Before(*query.StartDate) {
			continue
		}

		if query.EndDate != nil && report.GeneratedAt.After(*query.EndDate) {
			continue
		}

		results = append(results, report)
	}

	// 排序
	g.sortReports(results, query.SortBy, query.SortOrder)

	// 分页
	total := len(results)
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		return []*FinancialReport{}, total, nil
	}
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

// sortReports 排序报告
func (g *FinancialReportGenerator) sortReports(reports []*FinancialReport, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "generated_at"
	}

	for i := 0; i < len(reports)-1; i++ {
		for j := i + 1; j < len(reports); j++ {
			var swap bool
			switch sortBy {
			case "name":
				swap = reports[i].Name > reports[j].Name
			case "type":
				swap = reports[i].Type > reports[j].Type
			default:
				swap = reports[i].GeneratedAt.After(reports[j].GeneratedAt)
			}

			if sortOrder == "asc" {
				swap = !swap
			}

			if swap {
				reports[i], reports[j] = reports[j], reports[i]
			}
		}
	}
}

// DeleteReport 删除报告
func (g *FinancialReportGenerator) DeleteReport(ctx context.Context, id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.reports[id]; !exists {
		return ErrFinancialReportNotFound
	}

	// 删除文件
	filePath := filepath.Join(g.storagePath, id+".json")
	os.Remove(filePath)

	delete(g.reports, id)

	return nil
}

// ExportReport 导出报告
func (g *FinancialReportGenerator) exportReport(report *FinancialReport, format ExportFormat) (string, error) {
	switch format {
	case ExportJSON:
		return g.exportToJSON(report)
	case ExportCSV:
		return g.exportToCSV(report)
	case ExportExcel:
		return g.exportToExcel(report)
	default:
		return g.exportToJSON(report)
	}
}

// exportToJSON 导出为JSON
func (g *FinancialReportGenerator) exportToJSON(report *FinancialReport) (string, error) {
	filename := fmt.Sprintf("%s_%s.json", report.Type, report.ID)
	filePath := filepath.Join(g.storagePath, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

// exportToCSV 导出为CSV
func (g *FinancialReportGenerator) exportToCSV(report *FinancialReport) (string, error) {
	filename := fmt.Sprintf("%s_%s.csv", report.Type, report.ID)
	filePath := filepath.Join(g.storagePath, filename)

	var lines []string
	lines = append(lines, "区块,项目,金额,百分比")

	for _, section := range report.Sections {
		for _, row := range section.Data {
			amount := row.Values["amount"]
			lines = append(lines, fmt.Sprintf("%s,%s,%.2f,%.1f%%", section.Title, row.Label, amount, row.Percent))
		}
	}

	data := []byte{}
	for _, line := range lines {
		data = append(data, []byte(line+"\n")...)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

// exportToExcel 导出为Excel
func (g *FinancialReportGenerator) exportToExcel(report *FinancialReport) (string, error) {
	filename := fmt.Sprintf("%s_%s.xlsx", report.Type, report.ID)
	filePath := filepath.Join(g.storagePath, filename)

	f := excelize.NewFile()
	defer f.Close()

	sheet := "财务报告"
	f.SetSheetName("Sheet1", sheet)

	// 标题
	f.SetCellValue(sheet, "A1", report.Name)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("生成时间: %s", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	f.SetCellValue(sheet, "A3", fmt.Sprintf("报告周期: %s ~ %s", report.Period.StartTime.Format("2006-01-02"), report.Period.EndTime.Format("2006-01-02")))

	// 数据
	row := 5
	for _, section := range report.Sections {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), section.Title)
		row++
		for _, data := range section.Data {
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), data.Label)
			f.SetCellValue(sheet, fmt.Sprintf("C%d", row), data.Values["amount"])
			f.SetCellValue(sheet, fmt.Sprintf("D%d", row), fmt.Sprintf("%.1f%%", data.Percent))
			row++
		}
		row++
	}

	if err := f.SaveAs(filePath); err != nil {
		return "", err
	}

	return filePath, nil
}

// saveReport 保存报告
func (g *FinancialReportGenerator) saveReport(report *FinancialReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(g.storagePath, report.ID+".json")
	return os.WriteFile(filePath, data, 0644)
}

// ========== 报告模板 ==========

// FinancialReportTemplate 财务报告模板
type FinancialReportTemplate struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        FinancialReportType `json:"type"`
	Description string              `json:"description"`
	Sections    []SectionTemplate   `json:"sections"`
	Charts      []ChartTemplate     `json:"charts,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	IsDefault   bool                `json:"is_default"`
}

// SectionTemplate 区块模板
type SectionTemplate struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	DataSource string `json:"data_source"`
	Required   bool   `json:"required"`
	Position   int    `json:"position"`
}

// ChartTemplate 图表模板
type ChartTemplate struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

// GetFinReportDefaultTemplates 获取财务报告默认模板
func GetFinReportDefaultTemplates() []FinancialReportTemplate {
	return []FinancialReportTemplate{
		{
			ID:          "income_standard",
			Name:        "标准收入报告",
			Type:        FinancialReportTypeIncome,
			Description: "标准收入报告模板",
			Sections: []SectionTemplate{
				{ID: "summary", Title: "收入概览", Type: "summary", Position: 1, Required: true},
				{ID: "by_category", Title: "分类明细", Type: "items", Position: 2, Required: true},
				{ID: "by_source", Title: "来源明细", Type: "items", Position: 3, Required: false},
			},
			IsDefault: true,
		},
		{
			ID:          "expense_standard",
			Name:        "标准支出报告",
			Type:        FinancialReportTypeExpense,
			Description: "标准支出报告模板",
			Sections: []SectionTemplate{
				{ID: "summary", Title: "支出概览", Type: "summary", Position: 1, Required: true},
				{ID: "by_category", Title: "分类明细", Type: "items", Position: 2, Required: true},
				{ID: "by_department", Title: "部门明细", Type: "items", Position: 3, Required: false},
			},
			IsDefault: true,
		},
		{
			ID:          "budget_standard",
			Name:        "预算执行报告",
			Type:        FinancialReportTypeBudget,
			Description: "标准预算执行报告模板",
			Sections: []SectionTemplate{
				{ID: "summary", Title: "预算概览", Type: "summary", Position: 1, Required: true},
				{ID: "details", Title: "执行明细", Type: "items", Position: 2, Required: true},
			},
			IsDefault: true,
		},
		{
			ID:          "pl_standard",
			Name:        "损益报告",
			Type:        FinancialReportTypeProfitLoss,
			Description: "标准损益报告模板",
			Sections: []SectionTemplate{
				{ID: "summary", Title: "损益概览", Type: "summary", Position: 1, Required: true},
				{ID: "revenue", Title: "收入", Type: "items", Position: 2, Required: true},
				{ID: "expense", Title: "支出", Type: "items", Position: 3, Required: true},
				{ID: "profit", Title: "利润", Type: "items", Position: 4, Required: true},
			},
			IsDefault: true,
		},
	}
}
