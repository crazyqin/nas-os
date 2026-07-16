// Package smartpricing - 智能定价引擎类型定义
// 基于使用量和资源消耗的动态计费系统
package smartpricing

import (
	"time"
)

// PricingTier 定价层级.
type PricingTier struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Level       int       `json:"level"`      // 层级等级，1=基础, 2=标准, 3=高级
	BasePrice   float64   `json:"base_price"` // 基础价格
	UnitPrice   float64   `json:"unit_price"` // 单位价格
	Currency    string    `json:"currency"`   // 货币类型
	MinUsage    float64   `json:"min_usage"`  // 最小使用量
	MaxUsage    float64   `json:"max_usage"`  // 最大使用量，0=无限制
	Description string    `json:"description"`
	Features    []string  `json:"features"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UsageMetric 使用量指标.
type UsageMetric struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	ResourceType string            `json:"resource_type"` // storage, compute, network, api
	Value        float64           `json:"value"`         // 使用量值
	Unit         string            `json:"unit"`          // GB, hours, requests
	Timestamp    time.Time         `json:"timestamp"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// BillingPlan 计费方案.
type BillingPlan struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	TierID        string         `json:"tier_id"`
	BillingCycle  string         `json:"billing_cycle"` // monthly, quarterly, yearly
	BaseFee       float64        `json:"base_fee"`
	OverageRate   float64        `json:"overage_rate"` // 超额费率
	FreeQuota     float64        `json:"free_quota"`   // 免费额度
	QuotaUnit     string         `json:"quota_unit"`
	DiscountRules []DiscountRule `json:"discount_rules,omitempty"`
	IsActive      bool           `json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// DiscountRule 折扣规则.
type DiscountRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // percentage, fixed, tiered
	Value     float64   `json:"value"`
	MinUsage  float64   `json:"min_usage"`
	MaxUsage  float64   `json:"max_usage"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsActive  bool      `json:"is_active"`
}

// PriceRule 价格规则.
type PriceRule struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	ResourceType  string          `json:"resource_type"`
	PricingModel  string          `json:"pricing_model"` // flat, tiered, volume, pay_as_you_go
	BaseRate      float64         `json:"base_rate"`
	Unit          string          `json:"unit"`
	Tiers         []PriceTierRule `json:"tiers,omitempty"`
	EffectiveFrom time.Time       `json:"effective_from"`
	EffectiveTo   time.Time       `json:"effective_to"`
	Priority      int             `json:"priority"`
	IsActive      bool            `json:"is_active"`
	CreatedAt     time.Time       `json:"created_at"`
}

// PriceTierRule 价格层级规则.
type PriceTierRule struct {
	MinValue float64 `json:"min_value"`
	MaxValue float64 `json:"max_value"` // 0=无限制
	Rate     float64 `json:"rate"`
	Unit     string  `json:"unit"`
}

// Invoice 发票/账单.
type Invoice struct {
	ID            string        `json:"id"`
	UserID        string        `json:"user_id"`
	BillingPlanID string        `json:"billing_plan_id"`
	PeriodStart   time.Time     `json:"period_start"`
	PeriodEnd     time.Time     `json:"period_end"`
	BaseFee       float64       `json:"base_fee"`
	UsageFee      float64       `json:"usage_fee"`
	OverageFee    float64       `json:"overage_fee"`
	Discount      float64       `json:"discount"`
	Tax           float64       `json:"tax"`
	TotalAmount   float64       `json:"total_amount"`
	Currency      string        `json:"currency"`
	Status        string        `json:"status"` // draft, pending, paid, overdue
	Items         []InvoiceItem `json:"items"`
	CreatedAt     time.Time     `json:"created_at"`
	DueDate       time.Time     `json:"due_date"`
	PaidAt        *time.Time    `json:"paid_at,omitempty"`
}

// InvoiceItem 发票明细项.
type InvoiceItem struct {
	Description  string  `json:"description"`
	ResourceType string  `json:"resource_type"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	UnitPrice    float64 `json:"unit_price"`
	Amount       float64 `json:"amount"`
}

// PricingConfig 智能定价配置.
type PricingConfig struct {
	Enabled         bool    `json:"enabled"`
	DefaultCurrency string  `json:"default_currency"`
	TaxRate         float64 `json:"tax_rate"`
	GracePeriod     int     `json:"grace_period"` // 宽限期（天）
	MaxOverage      float64 `json:"max_overage"`  // 最大超额比例
}

// DefaultPricingConfig 默认定价配置.
func DefaultPricingConfig() *PricingConfig {
	return &PricingConfig{
		Enabled:         true,
		DefaultCurrency: "CNY",
		TaxRate:         0.06,
		GracePeriod:     7,
		MaxOverage:      0.2,
	}
}
