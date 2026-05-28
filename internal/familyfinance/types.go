// Package familyfinance 提供家庭财务中心功能
// 对标个人/家庭财务管理软件，支持多账户管理、智能分类、预算追踪、投资组合、账单提醒
package familyfinance

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrAccountNotFound 账户不存在错误
	ErrAccountNotFound = errors.New("账户不存在")
	// ErrAccountExists 账户已存在错误
	ErrAccountExists = errors.New("账户已存在")
	// ErrTransactionNotFound 交易记录不存在错误
	ErrTransactionNotFound = errors.New("交易记录不存在")
	// ErrInsufficientFunds 余额不足错误
	ErrInsufficientFunds = errors.New("余额不足")
	// ErrBudgetNotFound 预算不存在错误
	ErrBudgetNotFound = errors.New("预算不存在")
	// ErrBudgetExceeded 预算超支错误
	ErrBudgetExceeded = errors.New("预算已超支")
	// ErrInvestmentNotFound 投资不存在错误
	ErrInvestmentNotFound = errors.New("投资记录不存在")
	// ErrBillNotFound 账单不存在错误
	ErrBillNotFound = errors.New("账单不存在")
	// ErrInvalidAmount 无效金额错误
	ErrInvalidAmount = errors.New("无效金额")
	// ErrCategoryNotFound 分类不存在错误
	ErrCategoryNotFound = errors.New("分类不存在")
	// ErrCategoryExists 分类已存在错误
	ErrCategoryExists = errors.New("分类已存在")
)

// ========== 账户类型 ==========

// AccountType 账户类型
type AccountType string

// 账户类型常量
const (
	AccountTypeBank      AccountType = "bank"      // 银行账户
	AccountTypeCredit    AccountType = "credit"    // 信用卡
	AccountTypeCash      AccountType = "cash"      // 现金
	AccountTypeAlipay    AccountType = "alipay"    // 支付宝
	AccountTypeWechat    AccountType = "wechat"    // 微信
	AccountTypeInvestment AccountType = "investment" // 投资账户
	AccountTypeOther     AccountType = "other"     // 其他
)

// ========== 交易类型 ==========

// TransactionType 交易类型
type TransactionType string

// 交易类型常量
const (
	TransactionTypeIncome  TransactionType = "income"  // 收入
	TransactionTypeExpense TransactionType = "expense" // 支出
	TransactionTypeTransfer TransactionType = "transfer" // 转账
)

// ========== 投资类型 ==========

// InvestmentType 投资类型
type InvestmentType string

// 投资类型常量
const (
	InvestmentTypeFund     InvestmentType = "fund"     // 基金
	InvestmentTypeStock    InvestmentType = "stock"    // 股票
	InvestmentTypeCrypto   InvestmentType = "crypto"   // 加密货币
	InvestmentTypeBond     InvestmentType = "bond"     // 债券
	InvestmentTypeDeposit  InvestmentType = "deposit"  // 定期存款
	InvestmentTypeOther    InvestmentType = "other"    // 其他
)

// ========== 账单周期 ==========

// BillCycle 账单周期
type BillCycle string

// 账单周期常量
const (
	BillCycleDaily    BillCycle = "daily"    // 每日
	BillCycleWeekly   BillCycle = "weekly"   // 每周
	BillCycleMonthly  BillCycle = "monthly"  // 每月
	BillCycleQuarterly BillCycle = "quarterly" // 每季度
	BillCycleYearly   BillCycle = "yearly"   // 每年
	BillCycleOnce     BillCycle = "once"     // 一次性
)

// ========== 核心数据结构 ==========

// Account 账户
type Account struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        AccountType `json:"type"`
	Bank        string      `json:"bank,omitempty"`         // 银行名称
	CardNumber  string      `json:"card_number,omitempty"`  // 卡号后四位
	Balance     float64     `json:"balance"`                // 当前余额
	Currency    string      `json:"currency"`               // 货币类型
	IsDefault   bool        `json:"is_default"`             // 是否默认账户
	Icon        string      `json:"icon,omitempty"`         // 图标
	Color       string      `json:"color,omitempty"`        // 颜色
	Tags        []string    `json:"tags,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Transaction 交易记录
type Transaction struct {
	ID            string          `json:"id"`
	AccountID     string          `json:"account_id"`
	ToAccountID   string          `json:"to_account_id,omitempty"` // 转账目标账户
	Type          TransactionType `json:"type"`
	Amount        float64         `json:"amount"`
	CategoryID    string          `json:"category_id"`
	CategoryName  string          `json:"category_name,omitempty"`
	Description   string          `json:"description,omitempty"`
	Note          string          `json:"note,omitempty"`
	Date          time.Time       `json:"date"`
	Tags          []string        `json:"tags,omitempty"`
	Attachments   []string        `json:"attachments,omitempty"`   // 附件路径
	IsRecurring   bool            `json:"is_recurring"`
	RecurringID   string          `json:"recurring_id,omitempty"`  // 周期性交易ID
	CreatedAt     time.Time       `json:"created_at"`
}

// Category 分类
type Category struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"` // 父分类ID
	Icon     string `json:"icon,omitempty"`
	Color    string `json:"color,omitempty"`
	IsIncome bool   `json:"is_income"`            // 是否为收入分类
	Sort     int    `json:"sort"`                 // 排序
}

// Budget 预算
type Budget struct {
	ID           string    `json:"id"`
	CategoryID   string    `json:"category_id,omitempty"`   // 关联分类，空表示总预算
	Amount       float64   `json:"amount"`                  // 预算金额
	Spent        float64   `json:"spent"`                   // 已花费
	Remaining    float64   `json:"remaining"`               // 剩余
	UsagePercent float64   `json:"usage_percent"`           // 使用百分比
	Period       string    `json:"period"`                  // monthly, weekly, yearly
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	AlertPercent float64   `json:"alert_percent"`           // 预警百分比
	IsAlerted    bool      `json:"is_alerted"`              // 是否已触发预警
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Investment 投资记录
type Investment struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`            // 投资名称
	Type          InvestmentType `json:"type"`            // 投资类型
	Code          string         `json:"code,omitempty"`  // 基金代码/股票代码
	Shares        float64        `json:"shares"`          // 持有份额/股数
	CostBasis     float64        `json:"cost_basis"`      // 成本价
	CurrentPrice  float64        `json:"current_price"`   // 当前价格
	TotalCost     float64        `json:"total_cost"`      // 总成本
	CurrentValue  float64        `json:"current_value"`   // 当前市值
	GainLoss      float64        `json:"gain_loss"`       // 盈亏金额
	GainLossPercent float64      `json:"gain_loss_percent"` // 盈亏百分比
	BuyDate       time.Time      `json:"buy_date"`
	AccountID     string         `json:"account_id,omitempty"`
	Note          string         `json:"note,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Bill 账单
type Bill struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Amount      float64   `json:"amount"`
	CategoryID  string    `json:"category_id,omitempty"`
	AccountID   string    `json:"account_id,omitempty"`    // 扣款账户
	Cycle       BillCycle `json:"cycle"`                   // 账单周期
	DueDay      int       `json:"due_day"`                 // 每月几号（1-31）
	NextDueDate time.Time `json:"next_due_date"`           // 下次到期日
	LastPaidAt  *time.Time `json:"last_paid_at,omitempty"` // 上次支付时间
	IsAutoPay   bool      `json:"is_auto_pay"`             // 是否自动支付
	IsOverdue   bool      `json:"is_overdue"`              // 是否逾期
	ReminderDays int      `json:"reminder_days"`           // 提前提醒天数
	Enabled     bool      `json:"enabled"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 查询与统计 ==========

// TransactionQuery 交易查询参数
type TransactionQuery struct {
	AccountID   string              `json:"account_id,omitempty"`
	Type        TransactionType     `json:"type,omitempty"`
	CategoryID  string              `json:"category_id,omitempty"`
	StartDate   *time.Time          `json:"start_date,omitempty"`
	EndDate     *time.Time          `json:"end_date,omitempty"`
	MinAmount   *float64            `json:"min_amount,omitempty"`
	MaxAmount   *float64            `json:"max_amount,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Keyword     string              `json:"keyword,omitempty"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
	SortBy      string              `json:"sort_by"`    // date, amount, category
	SortOrder   string              `json:"sort_order"` // asc, desc
}

// FinancialSummary 财务摘要
type FinancialSummary struct {
	TotalIncome    float64            `json:"total_income"`
	TotalExpense   float64            `json:"total_expense"`
	NetIncome      float64            `json:"net_income"`
	TotalBalance   float64            `json:"total_balance"`
	TotalAssets    float64            `json:"total_assets"`    // 总资产（含投资）
	TotalLiability float64            `json:"total_liability"` // 总负债
	NetWorth       float64            `json:"net_worth"`       // 净资产
	ByCategory     []CategorySummary  `json:"by_category,omitempty"`
	ByAccount      []AccountSummary   `json:"by_account,omitempty"`
	Trend          []TrendPoint       `json:"trend,omitempty"`
}

// CategorySummary 分类统计
type CategorySummary struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Amount       float64 `json:"amount"`
	Percent      float64 `json:"percent"`
	Count        int     `json:"count"`
}

// AccountSummary 账户统计
type AccountSummary struct {
	AccountID   string  `json:"account_id"`
	AccountName string  `json:"account_name"`
	Balance     float64 `json:"balance"`
	Income      float64 `json:"income"`
	Expense     float64 `json:"expense"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date     time.Time `json:"date"`
	Income   float64   `json:"income"`
	Expense  float64   `json:"expense"`
	Net      float64   `json:"net"`
	Balance  float64   `json:"balance"`
}

// CashFlowForecast 现金流预测
type CashFlowForecast struct {
	GeneratedAt    time.Time     `json:"generated_at"`
	Months         int           `json:"months"`
	Predictions    []MonthPrediction `json:"predictions"`
	TotalPredicted float64       `json:"total_predicted"`
	Confidence     float64       `json:"confidence"` // 置信度 0-1
}

// MonthPrediction 月度预测
type MonthPrediction struct {
	Month       string  `json:"month"`        // YYYY-MM
	PredictedIn float64 `json:"predicted_in"` // 预测收入
	PredictedOut float64 `json:"predicted_out"` // 预测支出
	NetFlow     float64 `json:"net_flow"`
	Balance     float64 `json:"balance"`
}

// ========== 默认分类 ==========

// DefaultCategories 默认分类列表
var DefaultCategories = []Category{
	// 支出分类
	{ID: "cat-food", Name: "餐饮美食", Icon: "🍽️", IsIncome: false, Sort: 1},
	{ID: "cat-transport", Name: "交通出行", Icon: "🚗", IsIncome: false, Sort: 2},
	{ID: "cat-shopping", Name: "购物消费", Icon: "🛍️", IsIncome: false, Sort: 3},
	{ID: "cat-housing", Name: "住房缴费", Icon: "🏠", IsIncome: false, Sort: 4},
	{ID: "cat-entertainment", Name: "休闲娱乐", Icon: "🎮", IsIncome: false, Sort: 5},
	{ID: "cat-medical", Name: "医疗健康", Icon: "💊", IsIncome: false, Sort: 6},
	{ID: "cat-education", Name: "教育学习", Icon: "📚", IsIncome: false, Sort: 7},
	{ID: "cat-other-expense", Name: "其他支出", Icon: "📦", IsIncome: false, Sort: 99},
	// 收入分类
	{ID: "cat-salary", Name: "工资薪酬", Icon: "💰", IsIncome: true, Sort: 1},
	{ID: "cat-bonus", Name: "奖金补贴", Icon: "🎁", IsIncome: true, Sort: 2},
	{ID: "cat-investment-income", Name: "投资收益", Icon: "📈", IsIncome: true, Sort: 3},
	{ID: "cat-freelance", Name: "兼职副业", Icon: "💼", IsIncome: true, Sort: 4},
	{ID: "cat-other-income", Name: "其他收入", Icon: "💵", IsIncome: true, Sort: 99},
}
