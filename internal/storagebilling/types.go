// Package storagebilling 提供存储计费引擎功能。
// 支持多租户存储用量统计、分层计费、配额管理、账单生成和成本优化建议。
package storagebilling

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrTenantNotFound 租户不存在.
	ErrTenantNotFound = errors.New("租户不存在")
	// ErrQuotaNotFound 配额不存在.
	ErrQuotaNotFound = errors.New("配额不存在")
	// ErrBillNotFound 账单不存在.
	ErrBillNotFound = errors.New("账单不存在")
	// ErrInvalidParams 无效输入参数.
	ErrInvalidParams = errors.New("无效输入参数")
	// ErrQuotaExceeded 配额超限.
	ErrQuotaExceeded = errors.New("存储配额超限")
	// ErrDuplicateTenant 租户已存在.
	ErrDuplicateTenant = errors.New("租户已存在")
)

// ========== 存储类型 ==========

// StorageTier 存储层级类型.
type StorageTier string

const (
	// TierSSD SSD高速存储.
	TierSSD StorageTier = "ssd"
	// TierHDD HDD大容量存储.
	TierHDD StorageTier = "hdd"
	// TierArchive 归档存储.
	TierArchive StorageTier = "archive"
)

// ========== 计费周期 ==========

// BillingCycle 计费周期类型.
type BillingCycle string

const (
	// CycleMonthly 月度计费.
	CycleMonthly BillingCycle = "monthly"
	// CycleQuarterly 季度计费.
	CycleQuarterly BillingCycle = "quarterly"
)

// ========== 账单状态 ==========

// BillStatus 账单状态.
type BillStatus string

const (
	// BillStatusDraft 草稿.
	BillStatusDraft BillStatus = "draft"
	// BillStatusPending 待支付.
	BillStatusPending BillStatus = "pending"
	// BillStatusPaid 已支付.
	BillStatusPaid BillStatus = "paid"
	// BillStatusOverdue 逾期.
	BillStatusOverdue BillStatus = "overdue"
)

// ========== 核心数据结构 ==========

// TierRate 分层费率.
type TierRate struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// RatePerGB 每GB月费率（元）.
	RatePerGB float64 `json:"rate_per_gb"`
	// Description 费率描述.
	Description string `json:"description"`
}

// Tenant 存储租户.
type Tenant struct {
	// ID 租户ID.
	ID string `json:"id"`
	// Name 租户名称.
	Name string `json:"name"`
	// Department 所属部门.
	Department string `json:"department"`
	// Project 所属项目.
	Project string `json:"project"`
	// Contact 联系人.
	Contact string `json:"contact"`
	// Email 联系邮箱.
	Email string `json:"email"`
	// IsActive 是否激活.
	IsActive bool `json:"is_active"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// UsageRecord 存储用量记录.
type UsageRecord struct {
	// TenantID 租户ID.
	TenantID string `json:"tenant_id"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// UsedGB 已使用容量(GB).
	UsedGB float64 `json:"used_gb"`
	// SnapshotTime 快照时间.
	SnapshotTime time.Time `json:"snapshot_time"`
}

// StorageQuota 存储配额.
type StorageQuota struct {
	// ID 配额ID.
	ID string `json:"id"`
	// TenantID 租户ID.
	TenantID string `json:"tenant_id"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// QuotaGB 配额容量(GB).
	QuotaGB float64 `json:"quota_gb"`
	// UsedGB 已使用容量(GB).
	UsedGB float64 `json:"used_gb"`
	// AlertThreshold 告警阈值（0-1之间的比例）.
	AlertThreshold float64 `json:"alert_threshold"`
	// HardLimit 是否硬限制（超限后拒绝写入）.
	HardLimit bool `json:"hard_limit"`
	// IsActive 是否激活.
	IsActive bool `json:"is_active"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// StorageBill 存储账单.
type StorageBill struct {
	// ID 账单ID.
	ID string `json:"id"`
	// TenantID 租户ID.
	TenantID string `json:"tenant_id"`
	// TenantName 租户名称.
	TenantName string `json:"tenant_name"`
	// BillingCycle 计费周期.
	BillingCycle BillingCycle `json:"billing_cycle"`
	// PeriodStart 周期开始时间.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd 周期结束时间.
	PeriodEnd time.Time `json:"period_end"`
	// TierCharges 各层级费用明细.
	TierCharges []TierCharge `json:"tier_charges"`
	// TotalAmount 总费用.
	TotalAmount float64 `json:"total_amount"`
	// Currency 币种.
	Currency string `json:"currency"`
	// Status 账单状态.
	Status BillStatus `json:"status"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// PaidAt 支付时间.
	PaidAt time.Time `json:"paid_at"`
}

// TierCharge 层级费用明细.
type TierCharge struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// UsedGB 使用容量(GB).
	UsedGB float64 `json:"used_gb"`
	// RatePerGB 费率(元/GB/月).
	RatePerGB float64 `json:"rate_per_gb"`
	// Amount 费用.
	Amount float64 `json:"amount"`
}

// CostOptimization 成本优化建议.
type CostOptimization struct {
	// TenantID 租户ID.
	TenantID string `json:"tenant_id"`
	// TenantName 租户名称.
	TenantName string `json:"tenant_name"`
	// CurrentCost 当前月成本.
	CurrentCost float64 `json:"current_cost"`
	// PotentialSavings 潜在节省金额.
	PotentialSavings float64 `json:"potential_savings"`
	// Suggestions 优化建议列表.
	Suggestions []OptimizationSuggestion `json:"suggestions"`
}

// OptimizationSuggestion 优化建议.
type OptimizationSuggestion struct {
	// Type 建议类型: tier_migration/cleanup/compression.
	Type string `json:"type"`
	// Description 建议描述.
	Description string `json:"description"`
	// EstimatedSavings 预计节省金额.
	EstimatedSavings float64 `json:"estimated_savings"`
	// Priority 优先级: high/medium/low.
	Priority string `json:"priority"`
}

// UsageSummary 用量汇总.
type UsageSummary struct {
	// TenantID 租户ID.
	TenantID string `json:"tenant_id"`
	// TenantName 租户名称.
	TenantName string `json:"tenant_name"`
	// Department 所属部门.
	Department string `json:"department"`
	// Project 所属项目.
	Project string `json:"project"`
	// SSDUsage SSD使用量(GB).
	SSDUsage float64 `json:"ssd_usage"`
	// HDDUsage HDD使用量(GB).
	HDDUsage float64 `json:"hdd_usage"`
	// ArchiveUsage 归档使用量(GB).
	ArchiveUsage float64 `json:"archive_usage"`
	// TotalUsage 总使用量(GB).
	TotalUsage float64 `json:"total_usage"`
	// SSDQuota SSD配额(GB).
	SSDQuota float64 `json:"ssd_quota"`
	// HDDQuota HDD配额(GB).
	HDDQuota float64 `json:"hdd_quota"`
	// ArchiveQuota 归档配额(GB).
	ArchiveQuota float64 `json:"archive_quota"`
	// TotalQuota 总配额(GB).
	TotalQuota float64 `json:"total_quota"`
	// QuotaUsageRate 配额使用率.
	QuotaUsageRate float64 `json:"quota_usage_rate"`
	// EstimatedCost 预估月费用.
	EstimatedCost float64 `json:"estimated_cost"`
}
