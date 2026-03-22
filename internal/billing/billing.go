// Package billing 提供计费管理功能
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBillingNotFound 计费记录不存在错误
	ErrBillingNotFound = errors.New("计费记录不存在")
	// ErrInvoiceNotFound 发票不存在错误
	ErrInvoiceNotFound = errors.New("发票不存在")
	// ErrUsageRecordNotFound 用量记录不存在错误
	ErrUsageRecordNotFound = errors.New("用量记录不存在")
	// ErrInvalidBillingPeriod 无效的计费周期错误
	ErrInvalidBillingPeriod = errors.New("无效的计费周期")
	// ErrInvalidPricingModel 无效的计价模式错误
	ErrInvalidPricingModel = errors.New("无效的计价模式")
	// ErrInvoiceAlreadyPaid 发票已支付错误
	ErrInvoiceAlreadyPaid = errors.New("发票已支付")
	// ErrInvoiceAlreadyVoid 发票已作废错误
	ErrInvoiceAlreadyVoid = errors.New("发票已作废")
)

// ========== 计费配置 ==========

// Config 计费配置
type Config struct {
	// 基础配置
	Enabled           bool   `json:"enabled"`              // 是否启用计费
	DefaultCurrency   string `json:"default_currency"`     // 默认货币（CNY, USD 等）
	Cycle             Cycle  `json:"billing_cycle"`        // 计费周期
	BillingDayOfMonth int    `json:"billing_day_of_month"` // 每月账单日（1-28）

	// 存储计费配置
	StoragePricing   StoragePricingConfig   `json:"storage_pricing"`   // 存储计费配置
	BandwidthPricing BandwidthPricingConfig `json:"bandwidth_pricing"` // 带宽计费配置

	// 发票配置
	InvoicePrefix  string  `json:"invoice_prefix"`   // 发票前缀
	InvoiceDueDays int     `json:"invoice_due_days"` // 发票到期天数
	TaxRate        float64 `json:"tax_rate"`         // 税率（如 0.13 表示 13%）
	TaxIncluded    bool    `json:"tax_included"`     // 价格是否含税

	// 提醒配置
	ReminderDays     []int `json:"reminder_days"`      // 提前提醒天数
	OverdueReminder  bool  `json:"overdue_reminder"`   // 逾期提醒
	OverdueGraceDays int   `json:"overdue_grace_days"` // 逾期宽限天数

	// 数据保留配置
	UsageDataRetention int `json:"usage_data_retention"` // 用量数据保留天数
	InvoiceRetention   int `json:"invoice_retention"`    // 发票保留天数

	// 公司信息
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`
	CompanyTaxID   string `json:"company_tax_id"`
	CompanyPhone   string `json:"company_phone"`
	CompanyEmail   string `json:"company_email"`
}

// Cycle 计费周期
type Cycle string

// 计费周期常量
const (
	CycleDaily   Cycle = "daily"   // 日结
	CycleWeekly  Cycle = "weekly"  // 周结
	CycleMonthly Cycle = "monthly" // 月结
	CycleYearly  Cycle = "yearly"  // 年结
)

// StoragePricingConfig 存储计费配置
type StoragePricingConfig struct {
	// 基础存储价格
	BasePricePerGB    float64 `json:"base_price_per_gb"`    // 基础存储价格（元/GB/周期）
	SSDPricePerGB     float64 `json:"ssd_price_per_gb"`     // SSD 存储价格
	HDDPricePerGB     float64 `json:"hdd_price_per_gb"`     // HDD 存储价格
	ArchivePricePerGB float64 `json:"archive_price_per_gb"` // 归档存储价格

	// 存储池定价
	PoolPricing map[string]PoolPricing `json:"pool_pricing"` // 按存储池的定价

	// 免费额度
	FreeStorageGB      float64 `json:"free_storage_gb"`       // 免费存储额度（GB）
	FreeStoragePerUser float64 `json:"free_storage_per_user"` // 每用户免费额度

	// 阶梯定价
	TieredPricing []StorageTier `json:"tiered_pricing"` // 阶梯定价配置
}

// PoolPricing 存储池定价
type PoolPricing struct {
	PoolID          string  `json:"pool_id"`
	PoolName        string  `json:"pool_name"`
	StorageType     string  `json:"storage_type"` // ssd, hdd, archive
	PricePerGB      float64 `json:"price_per_gb"` // 价格（元/GB/周期）
	Currency        string  `json:"currency"`
	MinCommitmentGB float64 `json:"min_commitment_gb"` // 最低承诺量
	DiscountPercent float64 `json:"discount_percent"`  // 折扣百分比
}

// StorageTier 存储阶梯定价
type StorageTier struct {
	MinGB      float64 `json:"min_gb"`       // 起始 GB
	MaxGB      float64 `json:"max_gb"`       // 结束 GB（-1 表示无限）
	PricePerGB float64 `json:"price_per_gb"` // 该阶梯价格
}

// BandwidthPricingConfig 带宽计费配置
type BandwidthPricingConfig struct {
	// 带宽计费模式
	Model BandwidthModel `json:"model"` // 计费模式

	// 按流量计费
	TrafficPricePerGB float64 `json:"traffic_price_per_gb"` // 流量价格（元/GB）

	// 按带宽计费
	BandwidthPriceMbps float64 `json:"bandwidth_price_mbps"` // 带宽价格（元/Mbps/周期）

	// 免费额度
	FreeTrafficGB     float64 `json:"free_traffic_gb"`     // 免费流量额度（GB）
	FreeBandwidthMbps float64 `json:"free_bandwidth_mbps"` // 免费带宽额度（Mbps）

	// 阶梯定价
	TieredPricing []BandwidthTier `json:"tiered_pricing"` // 阶梯定价
}

// BandwidthModel 带宽计费模式
type BandwidthModel string

// 带宽计费模式常量
const (
	BandwidthModelTraffic   BandwidthModel = "traffic"   // 按流量
	BandwidthModelBandwidth BandwidthModel = "bandwidth" // 按带宽峰值
	BandwidthModel95th      BandwidthModel = "95th"      // 95 峰值
)

// BandwidthTier 带宽阶梯定价
type BandwidthTier struct {
	MinGB      float64 `json:"min_gb"`       // 起始 GB
	MaxGB      float64 `json:"max_gb"`       // 结束 GB（-1 表示无限）
	PricePerGB float64 `json:"price_per_gb"` // 该阶梯价格
}

// ========== 用量记录 ==========

// UsageRecord 用量记录
type UsageRecord struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	PoolID   string `json:"pool_id"`
	PoolName string `json:"pool_name"`

	// 时间信息
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	RecordedAt  time.Time `json:"recorded_at"`

	// 存储用量
	StorageUsedBytes    uint64  `json:"storage_used_bytes"`
	StorageUsedGB       float64 `json:"storage_used_gb"`
	StoragePeakBytes    uint64  `json:"storage_peak_bytes"`
	StoragePeakGB       float64 `json:"storage_peak_gb"`
	StorageAverageBytes uint64  `json:"storage_average_bytes"`
	StorageAverageGB    float64 `json:"storage_average_gb"`

	// 带宽用量
	BandwidthInBytes    uint64  `json:"bandwidth_in_bytes"` // 入站流量
	BandwidthInGB       float64 `json:"bandwidth_in_gb"`
	BandwidthOutBytes   uint64  `json:"bandwidth_out_bytes"` // 出站流量
	BandwidthOutGB      float64 `json:"bandwidth_out_gb"`
	BandwidthTotalBytes uint64  `json:"bandwidth_total_bytes"` // 总流量
	BandwidthTotalGB    float64 `json:"bandwidth_total_gb"`
	PeakBandwidthMbps   float64 `json:"peak_bandwidth_mbps"` // 峰值带宽

	// 操作次数
	APIRequests    int64 `json:"api_requests"`    // API 请求次数
	FileOperations int64 `json:"file_operations"` // 文件操作次数

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UsageRecordInput 用量记录输入
type UsageRecordInput struct {
	UserID            string                 `json:"user_id" binding:"required"`
	UserName          string                 `json:"user_name"`
	PoolID            string                 `json:"pool_id"`
	PoolName          string                 `json:"pool_name"`
	PeriodStart       time.Time              `json:"period_start" binding:"required"`
	PeriodEnd         time.Time              `json:"period_end" binding:"required"`
	StorageUsedBytes  uint64                 `json:"storage_used_bytes"`
	StoragePeakBytes  uint64                 `json:"storage_peak_bytes"`
	BandwidthInBytes  uint64                 `json:"bandwidth_in_bytes"`
	BandwidthOutBytes uint64                 `json:"bandwidth_out_bytes"`
	PeakBandwidthMbps float64                `json:"peak_bandwidth_mbps"`
	APIRequests       int64                  `json:"api_requests"`
	FileOperations    int64                  `json:"file_operations"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// UsageSummary 用量汇总
type UsageSummary struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// 存储汇总
	TotalStorageUsedGB float64 `json:"total_storage_used_gb"`
	MaxStorageUsedGB   float64 `json:"max_storage_used_gb"`
	AvgStorageUsedGB   float64 `json:"avg_storage_used_gb"`
	StorageRecords     int     `json:"storage_records"`

	// 带宽汇总
	TotalBandwidthInGB   float64 `json:"total_bandwidth_in_gb"`
	TotalBandwidthOutGB  float64 `json:"total_bandwidth_out_gb"`
	TotalBandwidthGB     float64 `json:"total_bandwidth_gb"`
	MaxPeakBandwidthMbps float64 `json:"max_peak_bandwidth_mbps"`
	BandwidthRecords     int     `json:"bandwidth_records"`

	// 操作汇总
	TotalAPIRequests    int64 `json:"total_api_requests"`
	TotalFileOperations int64 `json:"total_file_operations"`

	// 按存储池汇总
	PoolSummaries map[string]*PoolUsageSummary `json:"pool_summaries"`
}

// PoolUsageSummary 存储池用量汇总
type PoolUsageSummary struct {
	PoolID              string  `json:"pool_id"`
	PoolName            string  `json:"pool_name"`
	TotalStorageUsedGB  float64 `json:"total_storage_used_gb"`
	MaxStorageUsedGB    float64 `json:"max_storage_used_gb"`
	AvgStorageUsedGB    float64 `json:"avg_storage_used_gb"`
	TotalBandwidthInGB  float64 `json:"total_bandwidth_in_gb"`
	TotalBandwidthOutGB float64 `json:"total_bandwidth_out_gb"`
	TotalBandwidthGB    float64 `json:"total_bandwidth_gb"`
	Records             int     `json:"records"`
}

// ========== 发票 ==========

// Invoice 发票
type Invoice struct {
	ID            string `json:"id"`
	InvoiceNumber string `json:"invoice_number"`
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`

	// 时间信息
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	IssuedAt    time.Time  `json:"issued_at"`
	DueAt       time.Time  `json:"due_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`

	// 状态
	Status InvoiceStatus `json:"status"`

	// 金额
	Currency string `json:"currency"`

	// 存储费用
	StorageAmount    float64 `json:"storage_amount"`     // 存储费用
	StorageUsedGB    float64 `json:"storage_used_gb"`    // 存储用量
	StorageUnitPrice float64 `json:"storage_unit_price"` // 存储单价

	// 带宽费用
	BandwidthAmount    float64 `json:"bandwidth_amount"`     // 带宽费用
	BandwidthUsedGB    float64 `json:"bandwidth_used_gb"`    // 带宽用量
	BandwidthUnitPrice float64 `json:"bandwidth_unit_price"` // 带宽单价

	// 其他费用
	OtherAmount float64 `json:"other_amount"` // 其他费用

	// 折扣
	DiscountAmount float64 `json:"discount_amount"` // 折扣金额
	DiscountReason string  `json:"discount_reason"` // 折扣原因

	// 小计
	Subtotal    float64 `json:"subtotal"`     // 小计
	TaxAmount   float64 `json:"tax_amount"`   // 税额
	TotalAmount float64 `json:"total_amount"` // 总计

	// 明细
	LineItems []InvoiceLineItem `json:"line_items"`

	// 支付信息
	PaymentMethod    string `json:"payment_method,omitempty"`
	PaymentReference string `json:"payment_reference,omitempty"`

	// 备注
	Notes string `json:"notes,omitempty"`
	Terms string `json:"terms,omitempty"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// InvoiceStatus 发票状态
type InvoiceStatus string

// 发票状态常量
const (
	InvoiceStatusDraft    InvoiceStatus = "draft"    // 草稿
	InvoiceStatusIssued   InvoiceStatus = "issued"   // 已开具
	InvoiceStatusSent     InvoiceStatus = "sent"     // 已发送
	InvoiceStatusPaid     InvoiceStatus = "paid"     // 已支付
	InvoiceStatusOverdue  InvoiceStatus = "overdue"  // 逾期
	InvoiceStatusVoid     InvoiceStatus = "void"     // 已作废
	InvoiceStatusRefunded InvoiceStatus = "refunded" // 已退款
)

// InvoiceLineItem 发票明细项
type InvoiceLineItem struct {
	ID              string     `json:"id"`
	Description     string     `json:"description"`          // 描述
	Quantity        float64    `json:"quantity"`             // 数量
	Unit            string     `json:"unit"`                 // 单位
	UnitPrice       float64    `json:"unit_price"`           // 单价
	Amount          float64    `json:"amount"`               // 金额
	DiscountPercent float64    `json:"discount_percent"`     // 折扣百分比
	TaxRate         float64    `json:"tax_rate"`             // 税率
	StartDate       *time.Time `json:"start_date,omitempty"` // 开始日期
	EndDate         *time.Time `json:"end_date,omitempty"`   // 结束日期
	PoolID          string     `json:"pool_id,omitempty"`    // 存储池 ID
	PoolName        string     `json:"pool_name,omitempty"`  // 存储池名称
}

// InvoiceInput 发票输入
type InvoiceInput struct {
	UserID         string                 `json:"user_id" binding:"required"`
	UserName       string                 `json:"user_name"`
	PeriodStart    time.Time              `json:"period_start" binding:"required"`
	PeriodEnd      time.Time              `json:"period_end" binding:"required"`
	Currency       string                 `json:"currency"`
	LineItems      []InvoiceLineItemInput `json:"line_items" binding:"required"`
	DiscountAmount float64                `json:"discount_amount"`
	DiscountReason string                 `json:"discount_reason"`
	Notes          string                 `json:"notes"`
	Terms          string                 `json:"terms"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// InvoiceLineItemInput 发票明细项输入
type InvoiceLineItemInput struct {
	Description     string     `json:"description" binding:"required"`
	Quantity        float64    `json:"quantity" binding:"required"`
	Unit            string     `json:"unit"`
	UnitPrice       float64    `json:"unit_price" binding:"required"`
	DiscountPercent float64    `json:"discount_percent"`
	TaxRate         float64    `json:"tax_rate"`
	StartDate       *time.Time `json:"start_date"`
	EndDate         *time.Time `json:"end_date"`
	PoolID          string     `json:"pool_id"`
	PoolName        string     `json:"pool_name"`
}

// InvoiceSummary 发票汇总
type InvoiceSummary struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// 发票数量
	TotalInvoices   int `json:"total_invoices"`
	DraftInvoices   int `json:"draft_invoices"`
	IssuedInvoices  int `json:"issued_invoices"`
	PaidInvoices    int `json:"paid_invoices"`
	OverdueInvoices int `json:"overdue_invoices"`
	VoidInvoices    int `json:"void_invoices"`

	// 金额汇总
	TotalIssuedAmount      float64 `json:"total_issued_amount"`
	TotalPaidAmount        float64 `json:"total_paid_amount"`
	TotalOverdueAmount     float64 `json:"total_overdue_amount"`
	TotalOutstandingAmount float64 `json:"total_outstanding_amount"`

	// 费用明细
	TotalStorageAmount   float64 `json:"total_storage_amount"`
	TotalBandwidthAmount float64 `json:"total_bandwidth_amount"`
	TotalOtherAmount     float64 `json:"total_other_amount"`
	TotalDiscountAmount  float64 `json:"total_discount_amount"`
	TotalTaxAmount       float64 `json:"total_tax_amount"`
}

// ========== 计费统计 ==========

// Stats 计费统计
type Stats struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	GeneratedAt time.Time `json:"generated_at"`

	// 总体统计
	TotalUsers  int `json:"total_users"`
	ActiveUsers int `json:"active_users"`
	TotalPools  int `json:"total_pools"`

	// 用量统计
	TotalStorageUsedGB float64 `json:"total_storage_used_gb"`
	TotalBandwidthGB   float64 `json:"total_bandwidth_gb"`

	// 收入统计
	TotalRevenue     float64 `json:"total_revenue"`
	StorageRevenue   float64 `json:"storage_revenue"`
	BandwidthRevenue float64 `json:"bandwidth_revenue"`
	OtherRevenue     float64 `json:"other_revenue"`

	// 发票统计
	TotalInvoices     int     `json:"total_invoices"`
	PaidInvoices      int     `json:"paid_invoices"`
	OverdueInvoices   int     `json:"overdue_invoices"`
	OutstandingAmount float64 `json:"outstanding_amount"`

	// 按用户统计
	UserStats []UserStats `json:"user_stats"`

	// 按存储池统计
	PoolStats []PoolStats `json:"pool_stats"`
}

// UserStats 用户计费统计
type UserStats struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`

	// 用量
	StorageUsedGB  float64 `json:"storage_used_gb"`
	BandwidthGB    float64 `json:"bandwidth_gb"`
	APIRequests    int64   `json:"api_requests"`
	FileOperations int64   `json:"file_operations"`

	// 费用
	TotalAmount     float64 `json:"total_amount"`
	StorageAmount   float64 `json:"storage_amount"`
	BandwidthAmount float64 `json:"bandwidth_amount"`
	OtherAmount     float64 `json:"other_amount"`
	DiscountAmount  float64 `json:"discount_amount"`

	// 发票
	InvoiceCount      int     `json:"invoice_count"`
	PaidAmount        float64 `json:"paid_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`

	// 存储池
	PoolCount int `json:"pool_count"`
}

// PoolStats 存储池计费统计
type PoolStats struct {
	PoolID      string `json:"pool_id"`
	PoolName    string `json:"pool_name"`
	StorageType string `json:"storage_type"`

	// 用量
	StorageUsedGB    float64 `json:"storage_used_gb"`
	BandwidthInGB    float64 `json:"bandwidth_in_gb"`
	BandwidthOutGB   float64 `json:"bandwidth_out_gb"`
	TotalBandwidthGB float64 `json:"total_bandwidth_gb"`

	// 费用
	TotalAmount     float64 `json:"total_amount"`
	StorageAmount   float64 `json:"storage_amount"`
	BandwidthAmount float64 `json:"bandwidth_amount"`

	// 用户
	UserCount       int `json:"user_count"`
	ActiveUserCount int `json:"active_user_count"`

	// 定价
	PricePerGB float64 `json:"price_per_gb"`
	Currency   string  `json:"currency"`
}

// ========== 计费管理器 ==========

// Manager 计费管理器
type Manager struct {
	config  *Config
	dataDir string
	mu      sync.RWMutex

	// 数据存储
	usageRecords   map[string]*UsageRecord // 用量记录
	invoices       map[string]*Invoice     // 发票
	invoiceCounter int                     // 发票计数器
}

// NewManager 创建计费管理器
func NewManager(config *Config, dataDir string) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	bm := &Manager{
		config:       config,
		dataDir:      dataDir,
		usageRecords: make(map[string]*UsageRecord),
		invoices:     make(map[string]*Invoice),
	}

	// 加载已有数据
	if err := bm.load(); err != nil {
		return nil, fmt.Errorf("加载数据失败: %w", err)
	}

	return bm, nil
}

// DefaultConfig 默认计费配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:            true,
		DefaultCurrency:    "CNY",
		Cycle:              CycleMonthly,
		BillingDayOfMonth:  1,
		InvoicePrefix:      "INV",
		InvoiceDueDays:     30,
		TaxRate:            0.0,
		TaxIncluded:        false,
		ReminderDays:       []int{7, 3, 1},
		OverdueReminder:    true,
		OverdueGraceDays:   7,
		UsageDataRetention: 365,
		InvoiceRetention:   2555, // 7年
		StoragePricing: StoragePricingConfig{
			BasePricePerGB:    0.1,
			SSDPricePerGB:     0.2,
			HDDPricePerGB:     0.05,
			ArchivePricePerGB: 0.01,
			FreeStorageGB:     10,
			TieredPricing: []StorageTier{
				{MinGB: 0, MaxGB: 100, PricePerGB: 0.1},
				{MinGB: 100, MaxGB: 1000, PricePerGB: 0.08},
				{MinGB: 1000, MaxGB: -1, PricePerGB: 0.05},
			},
		},
		BandwidthPricing: BandwidthPricingConfig{
			Model:             BandwidthModelTraffic,
			TrafficPricePerGB: 0.5,
			FreeTrafficGB:     100,
		},
	}
}

// load 加载数据
func (bm *Manager) load() error {
	// 加载用量记录
	usagePath := filepath.Join(bm.dataDir, "usage_records.json")
	if data, err := os.ReadFile(usagePath); err == nil {
		var records []*UsageRecord
		if err := json.Unmarshal(data, &records); err != nil {
			return fmt.Errorf("解析用量记录失败: %w", err)
		}
		for _, r := range records {
			bm.usageRecords[r.ID] = r
		}
	}

	// 加载发票
	invoicePath := filepath.Join(bm.dataDir, "invoices.json")
	if data, err := os.ReadFile(invoicePath); err == nil {
		var invoices []*Invoice
		if err := json.Unmarshal(data, &invoices); err != nil {
			return fmt.Errorf("解析发票数据失败: %w", err)
		}
		for _, inv := range invoices {
			bm.invoices[inv.ID] = inv
			// 更新计数器
			if num := extractInvoiceNumber(inv.InvoiceNumber); num > bm.invoiceCounter {
				bm.invoiceCounter = num
			}
		}
	}

	return nil
}

// save 保存数据
func (bm *Manager) save() error {
	// 保存用量记录
	records := make([]*UsageRecord, 0, len(bm.usageRecords))
	for _, r := range bm.usageRecords {
		records = append(records, r)
	}
	recordsData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用量记录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(bm.dataDir, "usage_records.json"), recordsData, 0600); err != nil {
		return fmt.Errorf("保存用量记录失败: %w", err)
	}

	// 保存发票
	invoices := make([]*Invoice, 0, len(bm.invoices))
	for _, inv := range bm.invoices {
		invoices = append(invoices, inv)
	}
	invoicesData, err := json.MarshalIndent(invoices, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化发票数据失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(bm.dataDir, "invoices.json"), invoicesData, 0600); err != nil {
		return fmt.Errorf("保存发票数据失败: %w", err)
	}

	return nil
}

// ========== 用量记录管理 ==========

// RecordUsage 记录用量
func (bm *Manager) RecordUsage(ctx context.Context, input *UsageRecordInput) (*UsageRecord, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	record := &UsageRecord{
		ID:                  generateID("usage"),
		UserID:              input.UserID,
		UserName:            input.UserName,
		PoolID:              input.PoolID,
		PoolName:            input.PoolName,
		PeriodStart:         input.PeriodStart,
		PeriodEnd:           input.PeriodEnd,
		RecordedAt:          time.Now(),
		StorageUsedBytes:    input.StorageUsedBytes,
		StorageUsedGB:       bytesToGB(input.StorageUsedBytes),
		StoragePeakBytes:    input.StoragePeakBytes,
		StoragePeakGB:       bytesToGB(input.StoragePeakBytes),
		StorageAverageBytes: input.StorageUsedBytes, // 简化处理
		StorageAverageGB:    bytesToGB(input.StorageUsedBytes),
		BandwidthInBytes:    input.BandwidthInBytes,
		BandwidthInGB:       bytesToGB(input.BandwidthInBytes),
		BandwidthOutBytes:   input.BandwidthOutBytes,
		BandwidthOutGB:      bytesToGB(input.BandwidthOutBytes),
		BandwidthTotalBytes: input.BandwidthInBytes + input.BandwidthOutBytes,
		BandwidthTotalGB:    bytesToGB(input.BandwidthInBytes + input.BandwidthOutBytes),
		PeakBandwidthMbps:   input.PeakBandwidthMbps,
		APIRequests:         input.APIRequests,
		FileOperations:      input.FileOperations,
		Metadata:            input.Metadata,
	}

	bm.usageRecords[record.ID] = record

	if err := bm.save(); err != nil {
		return nil, err
	}

	return record, nil
}

// GetUsageRecord 获取用量记录
func (bm *Manager) GetUsageRecord(id string) (*UsageRecord, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	record, ok := bm.usageRecords[id]
	if !ok {
		return nil, ErrUsageRecordNotFound
	}
	return record, nil
}

// ListUsageRecords 列出用量记录
func (bm *Manager) ListUsageRecords(userID string, poolID string, start, end time.Time) ([]*UsageRecord, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var result []*UsageRecord
	for _, r := range bm.usageRecords {
		// 过滤条件
		if userID != "" && r.UserID != userID {
			continue
		}
		if poolID != "" && r.PoolID != poolID {
			continue
		}
		// 时间过滤：记录的周期与查询范围有重叠即可
		if !start.IsZero() && r.PeriodEnd.Before(start) {
			continue
		}
		if !end.IsZero() && r.PeriodStart.After(end) {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

// GetUserUsageSummary 获取用户用量汇总
func (bm *Manager) GetUserUsageSummary(userID string, start, end time.Time) (*UsageSummary, error) {
	records, err := bm.ListUsageRecords(userID, "", start, end)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, ErrUsageRecordNotFound
	}

	summary := &UsageSummary{
		UserID:        userID,
		UserName:      records[0].UserName,
		PeriodStart:   start,
		PeriodEnd:     end,
		PoolSummaries: make(map[string]*PoolUsageSummary),
	}

	for _, r := range records {
		// 存储汇总
		summary.TotalStorageUsedGB += r.StorageUsedGB
		if r.StorageUsedGB > summary.MaxStorageUsedGB {
			summary.MaxStorageUsedGB = r.StorageUsedGB
		}
		summary.StorageRecords++

		// 带宽汇总
		summary.TotalBandwidthInGB += r.BandwidthInGB
		summary.TotalBandwidthOutGB += r.BandwidthOutGB
		summary.TotalBandwidthGB += r.BandwidthTotalGB
		if r.PeakBandwidthMbps > summary.MaxPeakBandwidthMbps {
			summary.MaxPeakBandwidthMbps = r.PeakBandwidthMbps
		}
		summary.BandwidthRecords++

		// 操作汇总
		summary.TotalAPIRequests += r.APIRequests
		summary.TotalFileOperations += r.FileOperations

		// 按存储池汇总
		if r.PoolID != "" {
			poolSummary, ok := summary.PoolSummaries[r.PoolID]
			if !ok {
				poolSummary = &PoolUsageSummary{
					PoolID:   r.PoolID,
					PoolName: r.PoolName,
				}
				summary.PoolSummaries[r.PoolID] = poolSummary
			}
			poolSummary.TotalStorageUsedGB += r.StorageUsedGB
			if r.StorageUsedGB > poolSummary.MaxStorageUsedGB {
				poolSummary.MaxStorageUsedGB = r.StorageUsedGB
			}
			poolSummary.TotalBandwidthInGB += r.BandwidthInGB
			poolSummary.TotalBandwidthOutGB += r.BandwidthOutGB
			poolSummary.TotalBandwidthGB += r.BandwidthTotalGB
			poolSummary.Records++
		}
	}

	// 计算平均值
	if summary.StorageRecords > 0 {
		summary.AvgStorageUsedGB = summary.TotalStorageUsedGB / float64(summary.StorageRecords)
	}
	for _, ps := range summary.PoolSummaries {
		if ps.Records > 0 {
			ps.AvgStorageUsedGB = ps.TotalStorageUsedGB / float64(ps.Records)
		}
	}

	return summary, nil
}

// GetPoolUsageSummary 获取存储池用量汇总
func (bm *Manager) GetPoolUsageSummary(poolID string, start, end time.Time) (*PoolUsageSummary, error) {
	records, err := bm.ListUsageRecords("", poolID, start, end)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, ErrUsageRecordNotFound
	}

	summary := &PoolUsageSummary{
		PoolID:   poolID,
		PoolName: records[0].PoolName,
	}

	for _, r := range records {
		summary.TotalStorageUsedGB += r.StorageUsedGB
		if r.StorageUsedGB > summary.MaxStorageUsedGB {
			summary.MaxStorageUsedGB = r.StorageUsedGB
		}
		summary.TotalBandwidthInGB += r.BandwidthInGB
		summary.TotalBandwidthOutGB += r.BandwidthOutGB
		summary.TotalBandwidthGB += r.BandwidthTotalGB
		summary.Records++
	}

	if summary.Records > 0 {
		summary.AvgStorageUsedGB = summary.TotalStorageUsedGB / float64(summary.Records)
	}

	return summary, nil
}

// ========== 发票管理 ==========

// CreateInvoice 创建发票
func (bm *Manager) CreateInvoice(ctx context.Context, input *InvoiceInput) (*Invoice, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	dueDate := now.AddDate(0, 0, bm.config.InvoiceDueDays)

	currency := input.Currency
	if currency == "" {
		currency = bm.config.DefaultCurrency
	}

	// 生成发票号
	bm.invoiceCounter++
	invoiceNumber := fmt.Sprintf("%s-%s-%04d", bm.config.InvoicePrefix, now.Format("20060102"), bm.invoiceCounter)

	// 创建明细项
	lineItems := make([]InvoiceLineItem, 0, len(input.LineItems))
	var subtotal float64
	var storageAmount, bandwidthAmount, otherAmount float64

	for _, item := range input.LineItems {
		amount := item.Quantity * item.UnitPrice
		if item.DiscountPercent > 0 {
			amount = amount * (1 - item.DiscountPercent/100)
		}

		lineItem := InvoiceLineItem{
			ID:              generateID("item"),
			Description:     item.Description,
			Quantity:        item.Quantity,
			Unit:            item.Unit,
			UnitPrice:       item.UnitPrice,
			Amount:          amount,
			DiscountPercent: item.DiscountPercent,
			TaxRate:         item.TaxRate,
			StartDate:       item.StartDate,
			EndDate:         item.EndDate,
			PoolID:          item.PoolID,
			PoolName:        item.PoolName,
		}
		lineItems = append(lineItems, lineItem)
		subtotal += amount

		// 分类统计
		if item.PoolID != "" || item.PoolName != "" {
			storageAmount += amount
		} else if item.Description == "带宽费用" || item.Description == "流量费用" {
			bandwidthAmount += amount
		} else {
			otherAmount += amount
		}
	}

	// 计算税额
	var taxAmount float64
	if !bm.config.TaxIncluded && bm.config.TaxRate > 0 {
		taxAmount = subtotal * bm.config.TaxRate
	}

	// 计算总计
	totalAmount := subtotal + taxAmount - input.DiscountAmount
	if totalAmount < 0 {
		totalAmount = 0
	}

	invoice := &Invoice{
		ID:              generateID("inv"),
		InvoiceNumber:   invoiceNumber,
		UserID:          input.UserID,
		UserName:        input.UserName,
		PeriodStart:     input.PeriodStart,
		PeriodEnd:       input.PeriodEnd,
		IssuedAt:        now,
		DueAt:           dueDate,
		Status:          InvoiceStatusDraft,
		Currency:        currency,
		StorageAmount:   storageAmount,
		BandwidthAmount: bandwidthAmount,
		OtherAmount:     otherAmount,
		DiscountAmount:  input.DiscountAmount,
		DiscountReason:  input.DiscountReason,
		Subtotal:        subtotal,
		TaxAmount:       taxAmount,
		TotalAmount:     totalAmount,
		LineItems:       lineItems,
		Notes:           input.Notes,
		Terms:           input.Terms,
		Metadata:        input.Metadata,
	}

	bm.invoices[invoice.ID] = invoice

	if err := bm.save(); err != nil {
		return nil, err
	}

	return invoice, nil
}

// GenerateInvoiceFromUsage 根据用量生成发票
func (bm *Manager) GenerateInvoiceFromUsage(ctx context.Context, userID string, start, end time.Time) (*Invoice, error) {
	// 获取用户用量汇总
	summary, err := bm.GetUserUsageSummary(userID, start, end)
	if err != nil {
		return nil, err
	}

	// 创建明细项
	lineItems := []InvoiceLineItemInput{
		{
			Description: "存储费用",
			Quantity:    summary.TotalStorageUsedGB,
			Unit:        "GB",
			UnitPrice:   bm.getStorageUnitPrice(summary.TotalStorageUsedGB),
			StartDate:   &start,
			EndDate:     &end,
		},
		{
			Description: "带宽费用",
			Quantity:    summary.TotalBandwidthGB,
			Unit:        "GB",
			UnitPrice:   bm.getBandwidthUnitPrice(summary.TotalBandwidthGB),
			StartDate:   &start,
			EndDate:     &end,
		},
	}

	// 添加存储池明细
	for poolID, poolSummary := range summary.PoolSummaries {
		poolAmount := bm.calculatePoolStorageCost(poolSummary)
		if poolAmount > 0 {
			lineItems = append(lineItems, InvoiceLineItemInput{
				Description: fmt.Sprintf("存储池 %s 存储费用", poolSummary.PoolName),
				Quantity:    poolSummary.TotalStorageUsedGB,
				Unit:        "GB",
				UnitPrice:   bm.getPoolUnitPrice(poolID),
				StartDate:   &start,
				EndDate:     &end,
				PoolID:      poolID,
				PoolName:    poolSummary.PoolName,
			})
		}
	}

	return bm.CreateInvoice(ctx, &InvoiceInput{
		UserID:      userID,
		UserName:    summary.UserName,
		PeriodStart: start,
		PeriodEnd:   end,
		LineItems:   lineItems,
	})
}

// GetInvoice 获取发票
func (bm *Manager) GetInvoice(id string) (*Invoice, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	invoice, ok := bm.invoices[id]
	if !ok {
		return nil, ErrInvoiceNotFound
	}
	return invoice, nil
}

// GetInvoiceByNumber 按发票号获取发票
func (bm *Manager) GetInvoiceByNumber(number string) (*Invoice, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	for _, inv := range bm.invoices {
		if inv.InvoiceNumber == number {
			return inv, nil
		}
	}
	return nil, ErrInvoiceNotFound
}

// ListInvoices 列出发票
func (bm *Manager) ListInvoices(userID string, status InvoiceStatus, start, end time.Time) ([]*Invoice, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var result []*Invoice
	for _, inv := range bm.invoices {
		// 过滤条件
		if userID != "" && inv.UserID != userID {
			continue
		}
		if status != "" && inv.Status != status {
			continue
		}
		if !start.IsZero() && inv.PeriodStart.Before(start) {
			continue
		}
		if !end.IsZero() && inv.PeriodEnd.After(end) {
			continue
		}
		result = append(result, inv)
	}
	return result, nil
}

// IssueInvoice 开具发票
func (bm *Manager) IssueInvoice(id string) (*Invoice, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	invoice, ok := bm.invoices[id]
	if !ok {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft {
		return nil, fmt.Errorf("只能开具草稿状态的发票")
	}

	invoice.Status = InvoiceStatusIssued
	invoice.IssuedAt = time.Now()

	if err := bm.save(); err != nil {
		return nil, err
	}

	return invoice, nil
}

// MarkInvoicePaid 标记发票已支付
func (bm *Manager) MarkInvoicePaid(id string, paymentMethod, paymentRef string) (*Invoice, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	invoice, ok := bm.invoices[id]
	if !ok {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status == InvoiceStatusPaid {
		return nil, ErrInvoiceAlreadyPaid
	}
	if invoice.Status == InvoiceStatusVoid {
		return nil, ErrInvoiceAlreadyVoid
	}

	now := time.Now()
	invoice.Status = InvoiceStatusPaid
	invoice.PaidAt = &now
	invoice.PaymentMethod = paymentMethod
	invoice.PaymentReference = paymentRef

	if err := bm.save(); err != nil {
		return nil, err
	}

	return invoice, nil
}

// VoidInvoice 作废发票
func (bm *Manager) VoidInvoice(id string) (*Invoice, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	invoice, ok := bm.invoices[id]
	if !ok {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status == InvoiceStatusPaid {
		return nil, fmt.Errorf("已支付的发票不能作废")
	}
	if invoice.Status == InvoiceStatusVoid {
		return nil, ErrInvoiceAlreadyVoid
	}

	invoice.Status = InvoiceStatusVoid

	if err := bm.save(); err != nil {
		return nil, err
	}

	return invoice, nil
}

// ========== 计费统计 ==========

// GetStats 获取计费统计
func (bm *Manager) GetStats(start, end time.Time) (*Stats, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	stats := &Stats{
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now(),
	}

	// 用户统计
	userSet := make(map[string]bool)
	poolSet := make(map[string]bool)
	userAmounts := make(map[string]*UserStats)
	poolAmounts := make(map[string]*PoolStats)

	// 遍历用量记录
	for _, r := range bm.usageRecords {
		// 时间过滤：记录的周期与查询范围有重叠即可
		if !start.IsZero() && r.PeriodEnd.Before(start) {
			continue
		}
		if !end.IsZero() && r.PeriodStart.After(end) {
			continue
		}

		userSet[r.UserID] = true
		if r.PoolID != "" {
			poolSet[r.PoolID] = true
		}

		stats.TotalStorageUsedGB += r.StorageUsedGB
		stats.TotalBandwidthGB += r.BandwidthTotalGB

		// 用户统计
		if _, ok := userAmounts[r.UserID]; !ok {
			userAmounts[r.UserID] = &UserStats{
				UserID:   r.UserID,
				UserName: r.UserName,
			}
		}
		userAmounts[r.UserID].StorageUsedGB += r.StorageUsedGB
		userAmounts[r.UserID].BandwidthGB += r.BandwidthTotalGB
		userAmounts[r.UserID].APIRequests += r.APIRequests
		userAmounts[r.UserID].FileOperations += r.FileOperations

		// 存储池统计
		if r.PoolID != "" {
			if _, ok := poolAmounts[r.PoolID]; !ok {
				poolAmounts[r.PoolID] = &PoolStats{
					PoolID:   r.PoolID,
					PoolName: r.PoolName,
				}
			}
			poolAmounts[r.PoolID].StorageUsedGB += r.StorageUsedGB
			poolAmounts[r.PoolID].BandwidthInGB += r.BandwidthInGB
			poolAmounts[r.PoolID].BandwidthOutGB += r.BandwidthOutGB
			poolAmounts[r.PoolID].TotalBandwidthGB += r.BandwidthTotalGB
		}
	}

	stats.TotalUsers = len(userSet)
	stats.TotalPools = len(poolSet)

	// 遍历发票
	for _, inv := range bm.invoices {
		// 时间过滤：发票的周期与查询范围有重叠即可
		if !start.IsZero() && inv.PeriodEnd.Before(start) {
			continue
		}
		if !end.IsZero() && inv.PeriodStart.After(end) {
			continue
		}

		stats.TotalInvoices++

		switch inv.Status {
		case InvoiceStatusPaid:
			stats.PaidInvoices++
			stats.TotalRevenue += inv.TotalAmount
			stats.StorageRevenue += inv.StorageAmount
			stats.BandwidthRevenue += inv.BandwidthAmount
			stats.OtherRevenue += inv.OtherAmount
		case InvoiceStatusOverdue:
			stats.OverdueInvoices++
			stats.OutstandingAmount += inv.TotalAmount
		case InvoiceStatusIssued, InvoiceStatusSent:
			stats.OutstandingAmount += inv.TotalAmount
		}

		// 用户费用统计
		if userStats, ok := userAmounts[inv.UserID]; ok {
			userStats.TotalAmount += inv.TotalAmount
			userStats.StorageAmount += inv.StorageAmount
			userStats.BandwidthAmount += inv.BandwidthAmount
			userStats.OtherAmount += inv.OtherAmount
			userStats.DiscountAmount += inv.DiscountAmount
			userStats.InvoiceCount++
			switch inv.Status {
			case InvoiceStatusPaid:
				userStats.PaidAmount += inv.TotalAmount
			case InvoiceStatusIssued, InvoiceStatusSent:
				userStats.OutstandingAmount += inv.TotalAmount
			}
		}
	}

	// 计算用户费用
	for _, us := range userAmounts {
		us.TotalAmount = bm.calculateStorageCost(us.StorageUsedGB) + bm.calculateBandwidthCost(us.BandwidthGB)
	}

	// 计算存储池费用
	for poolID, ps := range poolAmounts {
		ps.TotalAmount = bm.calculatePoolStorageCost(&PoolUsageSummary{
			TotalStorageUsedGB: ps.StorageUsedGB,
		})
		ps.StorageAmount = ps.TotalAmount
		ps.PricePerGB = bm.getPoolUnitPrice(poolID)
		ps.Currency = bm.config.DefaultCurrency
	}

	// 转换为列表
	for _, us := range userAmounts {
		stats.UserStats = append(stats.UserStats, *us)
	}
	for _, ps := range poolAmounts {
		stats.PoolStats = append(stats.PoolStats, *ps)
	}

	return stats, nil
}

// GetUserStats 获取用户计费统计
func (bm *Manager) GetUserStats(userID string, start, end time.Time) (*UserStats, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	stats := &UserStats{
		UserID: userID,
	}

	// 遍历用量记录
	for _, r := range bm.usageRecords {
		if r.UserID != userID {
			continue
		}
		// 时间过滤：记录的周期与查询范围有重叠即可
		if !start.IsZero() && r.PeriodEnd.Before(start) {
			continue
		}
		if !end.IsZero() && r.PeriodStart.After(end) {
			continue
		}

		stats.StorageUsedGB += r.StorageUsedGB
		stats.BandwidthGB += r.BandwidthTotalGB
		stats.APIRequests += r.APIRequests
		stats.FileOperations += r.FileOperations
	}

	// 遍历发票
	for _, inv := range bm.invoices {
		if inv.UserID != userID {
			continue
		}
		if !start.IsZero() && inv.PeriodStart.Before(start) {
			continue
		}
		if !end.IsZero() && inv.PeriodEnd.After(end) {
			continue
		}

		stats.TotalAmount += inv.TotalAmount
		stats.StorageAmount += inv.StorageAmount
		stats.BandwidthAmount += inv.BandwidthAmount
		stats.OtherAmount += inv.OtherAmount
		stats.DiscountAmount += inv.DiscountAmount
		stats.InvoiceCount++

		switch inv.Status {
		case InvoiceStatusPaid:
			stats.PaidAmount += inv.TotalAmount
		case InvoiceStatusIssued, InvoiceStatusSent, InvoiceStatusOverdue:
			stats.OutstandingAmount += inv.TotalAmount
		}
	}

	// 计算费用
	if stats.TotalAmount == 0 {
		stats.TotalAmount = bm.calculateStorageCost(stats.StorageUsedGB) + bm.calculateBandwidthCost(stats.BandwidthGB)
	}

	return stats, nil
}

// GetPoolStats 获取存储池计费统计
func (bm *Manager) GetPoolStats(poolID string, start, end time.Time) (*PoolStats, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	stats := &PoolStats{
		PoolID:     poolID,
		PricePerGB: bm.getPoolUnitPrice(poolID),
		Currency:   bm.config.DefaultCurrency,
	}

	// 遍历用量记录
	for _, r := range bm.usageRecords {
		if r.PoolID != poolID {
			continue
		}
		// 时间过滤：记录的周期与查询范围有重叠即可
		if !start.IsZero() && r.PeriodEnd.Before(start) {
			continue
		}
		if !end.IsZero() && r.PeriodStart.After(end) {
			continue
		}

		stats.PoolName = r.PoolName
		stats.StorageUsedGB += r.StorageUsedGB
		stats.BandwidthInGB += r.BandwidthInGB
		stats.BandwidthOutGB += r.BandwidthOutGB
		stats.TotalBandwidthGB += r.BandwidthTotalGB
		stats.UserCount++
	}

	// 计算费用
	stats.TotalAmount = bm.calculatePoolStorageCost(&PoolUsageSummary{
		TotalStorageUsedGB: stats.StorageUsedGB,
	})
	stats.StorageAmount = stats.TotalAmount

	return stats, nil
}

// GetInvoiceSummary 获取发票汇总
func (bm *Manager) GetInvoiceSummary(userID string, start, end time.Time) (*InvoiceSummary, error) {
	invoices, err := bm.ListInvoices(userID, "", start, end)
	if err != nil {
		return nil, err
	}

	summary := &InvoiceSummary{
		UserID:      userID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	for _, inv := range invoices {
		if summary.UserName == "" && inv.UserName != "" {
			summary.UserName = inv.UserName
		}

		summary.TotalInvoices++

		switch inv.Status {
		case InvoiceStatusDraft:
			summary.DraftInvoices++
		case InvoiceStatusIssued, InvoiceStatusSent:
			summary.IssuedInvoices++
			summary.TotalIssuedAmount += inv.TotalAmount
			summary.TotalOutstandingAmount += inv.TotalAmount
		case InvoiceStatusPaid:
			summary.PaidInvoices++
			summary.TotalPaidAmount += inv.TotalAmount
		case InvoiceStatusOverdue:
			summary.OverdueInvoices++
			summary.TotalOverdueAmount += inv.TotalAmount
			summary.TotalOutstandingAmount += inv.TotalAmount
		case InvoiceStatusVoid:
			summary.VoidInvoices++
		}

		summary.TotalStorageAmount += inv.StorageAmount
		summary.TotalBandwidthAmount += inv.BandwidthAmount
		summary.TotalOtherAmount += inv.OtherAmount
		summary.TotalDiscountAmount += inv.DiscountAmount
		summary.TotalTaxAmount += inv.TaxAmount
	}

	return summary, nil
}

// ========== 费用计算 ==========

// calculateStorageCost 计算存储费用
func (bm *Manager) calculateStorageCost(gb float64) float64 {
	if gb <= bm.config.StoragePricing.FreeStorageGB {
		return 0
	}

	gb -= bm.config.StoragePricing.FreeStorageGB

	// 阶梯定价
	if len(bm.config.StoragePricing.TieredPricing) > 0 {
		return calculateTieredCost(gb, bm.config.StoragePricing.TieredPricing)
	}

	return gb * bm.config.StoragePricing.BasePricePerGB
}

// calculateBandwidthCost 计算带宽费用
func (bm *Manager) calculateBandwidthCost(gb float64) float64 {
	if gb <= bm.config.BandwidthPricing.FreeTrafficGB {
		return 0
	}

	gb -= bm.config.BandwidthPricing.FreeTrafficGB

	// 阶梯定价
	if len(bm.config.BandwidthPricing.TieredPricing) > 0 {
		return calculateBandwidthTieredCost(gb, bm.config.BandwidthPricing.TieredPricing)
	}

	return gb * bm.config.BandwidthPricing.TrafficPricePerGB
}

// calculatePoolStorageCost 计算存储池存储费用
func (bm *Manager) calculatePoolStorageCost(summary *PoolUsageSummary) float64 {
	// 检查是否有存储池定价
	if pricing, ok := bm.config.StoragePricing.PoolPricing[summary.PoolID]; ok {
		gb := summary.TotalStorageUsedGB
		if pricing.MinCommitmentGB > 0 && gb < pricing.MinCommitmentGB {
			gb = pricing.MinCommitmentGB
		}
		cost := gb * pricing.PricePerGB
		if pricing.DiscountPercent > 0 {
			cost = cost * (1 - pricing.DiscountPercent/100)
		}
		return cost
	}

	// 使用默认定价
	return bm.calculateStorageCost(summary.TotalStorageUsedGB)
}

// getStorageUnitPrice 获取存储单价
func (bm *Manager) getStorageUnitPrice(gb float64) float64 {
	if len(bm.config.StoragePricing.TieredPricing) > 0 {
		for _, tier := range bm.config.StoragePricing.TieredPricing {
			if gb >= tier.MinGB && (tier.MaxGB < 0 || gb < tier.MaxGB) {
				return tier.PricePerGB
			}
		}
	}
	return bm.config.StoragePricing.BasePricePerGB
}

// getBandwidthUnitPrice 获取带宽单价
func (bm *Manager) getBandwidthUnitPrice(gb float64) float64 {
	if len(bm.config.BandwidthPricing.TieredPricing) > 0 {
		for _, tier := range bm.config.BandwidthPricing.TieredPricing {
			if gb >= tier.MinGB && (tier.MaxGB < 0 || gb < tier.MaxGB) {
				return tier.PricePerGB
			}
		}
	}
	return bm.config.BandwidthPricing.TrafficPricePerGB
}

// getPoolUnitPrice 获取存储池单价
func (bm *Manager) getPoolUnitPrice(poolID string) float64 {
	if pricing, ok := bm.config.StoragePricing.PoolPricing[poolID]; ok {
		return pricing.PricePerGB
	}
	return bm.config.StoragePricing.BasePricePerGB
}

// ========== 辅助函数 ==========

// generateID 生成 ID
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), randomString(6))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}

// bytesToGB 字节转 GB
func bytesToGB(bytes uint64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024)
}

// extractInvoiceNumber 从发票号提取编号
func extractInvoiceNumber(number string) int {
	// 发票号格式: PREFIX-YYYYMMDD-NNNN
	// 提取最后一个数字部分
	parts := strings.Split(number, "-")
	if len(parts) >= 3 {
		var num int
		if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &num); err != nil {
			return 0
		}
		return num
	}
	return 0
}

// calculateTieredCost 阶梯定价计算（存储）
func calculateTieredCost(amount float64, tiers []StorageTier) float64 {
	var total float64
	remaining := amount

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		tierSize := tier.MaxGB - tier.MinGB
		if tier.MaxGB < 0 {
			// 无限阶梯
			total += remaining * tier.PricePerGB
			break
		}

		if remaining <= tierSize {
			total += remaining * tier.PricePerGB
			break
		}

		total += tierSize * tier.PricePerGB
		remaining -= tierSize
	}

	return total
}

// calculateBandwidthTieredCost 阶梯定价计算（带宽）
func calculateBandwidthTieredCost(amount float64, tiers []BandwidthTier) float64 {
	var total float64
	remaining := amount

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		tierSize := tier.MaxGB - tier.MinGB
		if tier.MaxGB < 0 {
			// 无限阶梯
			total += remaining * tier.PricePerGB
			break
		}

		if remaining <= tierSize {
			total += remaining * tier.PricePerGB
			break
		}

		total += tierSize * tier.PricePerGB
		remaining -= tierSize
	}

	return total
}

// ========== 配置管理 ==========

// GetConfig 获取配置
func (bm *Manager) GetConfig() *Config {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.config
}

// UpdateConfig 更新配置
func (bm *Manager) UpdateConfig(config *Config) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.config = config
	return bm.save()
}

// ========== 清理过期数据 ==========

// CleanupOldData 清理过期数据
func (bm *Manager) CleanupOldData() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	usageCutoff := now.AddDate(0, 0, -bm.config.UsageDataRetention)
	invoiceCutoff := now.AddDate(0, 0, -bm.config.InvoiceRetention)

	// 清理过期用量记录
	for id, record := range bm.usageRecords {
		if record.PeriodEnd.Before(usageCutoff) {
			delete(bm.usageRecords, id)
		}
	}

	// 清理过期发票（保留已支付的）
	for id, invoice := range bm.invoices {
		if invoice.Status != InvoiceStatusPaid && invoice.PeriodEnd.Before(invoiceCutoff) {
			delete(bm.invoices, id)
		}
	}

	return bm.save()
}
