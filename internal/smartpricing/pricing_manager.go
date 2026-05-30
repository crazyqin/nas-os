// Package smartpricing - 智能定价引擎管理器
// 基于使用量和资源消耗的动态计费系统
package smartpricing

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PricingManager 智能定价管理器
type PricingManager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	config       *PricingConfig
	tiers        map[string]*PricingTier
	plans        map[string]*BillingPlan
	priceRules   map[string]*PriceRule
	usageMetrics map[string][]*UsageMetric // userID -> metrics
	invoices     map[string]*Invoice
}

// NewPricingManager 创建智能定价管理器
func NewPricingManager(logger *zap.Logger, config *PricingConfig) *PricingManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultPricingConfig()
	}

	pm := &PricingManager{
		logger:       logger,
		config:       config,
		tiers:        make(map[string]*PricingTier),
		plans:        make(map[string]*BillingPlan),
		priceRules:   make(map[string]*PriceRule),
		usageMetrics: make(map[string][]*UsageMetric),
		invoices:     make(map[string]*Invoice),
	}

	// 初始化默认定价层级
	pm.initDefaultTiers()
	// 初始化默认计费方案
	pm.initDefaultPlans()
	// 初始化默认价格规则
	pm.initDefaultPriceRules()

	return pm
}

// initDefaultTiers 初始化默认定价层级
func (pm *PricingManager) initDefaultTiers() {
	pm.tiers["tier-basic"] = &PricingTier{
		ID:          "tier-basic",
		Name:        "基础版",
		Level:       1,
		BasePrice:   0,
		UnitPrice:   0.1,
		Currency:    "CNY",
		MinUsage:    0,
		MaxUsage:    100,
		Description: "适合个人用户的基础存储方案",
		Features:    []string{"100GB存储", "基础API访问", "邮件支持"},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pm.tiers["tier-standard"] = &PricingTier{
		ID:          "tier-standard",
		Name:        "标准版",
		Level:       2,
		BasePrice:   99,
		UnitPrice:   0.08,
		Currency:    "CNY",
		MinUsage:    100,
		MaxUsage:    1000,
		Description: "适合小型团队的标准存储方案",
		Features:    []string{"1TB存储", "高级API访问", "优先支持", "数据备份"},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pm.tiers["tier-premium"] = &PricingTier{
		ID:          "tier-premium",
		Name:        "高级版",
		Level:       3,
		BasePrice:   399,
		UnitPrice:   0.05,
		Currency:    "CNY",
		MinUsage:    1000,
		MaxUsage:    0,
		Description: "适合大型企业的高级存储方案",
		Features:    []string{"无限存储", "企业级API", "7x24支持", "SLA保障", "专属客户经理"},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// initDefaultPlans 初始化默认计费方案
func (pm *PricingManager) initDefaultPlans() {
	pm.plans["plan-monthly-basic"] = &BillingPlan{
		ID:            "plan-monthly-basic",
		Name:          "基础月付",
		Description:   "按月付费的基础方案",
		TierID:        "tier-basic",
		BillingCycle:  "monthly",
		BaseFee:       0,
		OverageRate:   0.15,
		FreeQuota:     10,
		QuotaUnit:     "GB",
		DiscountRules: []DiscountRule{},
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	pm.plans["plan-monthly-standard"] = &BillingPlan{
		ID:            "plan-monthly-standard",
		Name:          "标准月付",
		Description:   "按月付费的标准方案",
		TierID:        "tier-standard",
		BillingCycle:  "monthly",
		BaseFee:       99,
		OverageRate:   0.12,
		FreeQuota:     100,
		QuotaUnit:     "GB",
		DiscountRules: []DiscountRule{},
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	pm.plans["plan-yearly-premium"] = &BillingPlan{
		ID:            "plan-yearly-premium",
		Name:          "高级年付",
		Description:   "按年付费的高级方案，享受8折优惠",
		TierID:        "tier-premium",
		BillingCycle:  "yearly",
		BaseFee:       3830,
		OverageRate:   0.08,
		FreeQuota:     1000,
		QuotaUnit:     "GB",
		DiscountRules: []DiscountRule{},
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// initDefaultPriceRules 初始化默认价格规则
func (pm *PricingManager) initDefaultPriceRules() {
	pm.priceRules["rule-storage"] = &PriceRule{
		ID:           "rule-storage",
		Name:         "存储价格规则",
		ResourceType: "storage",
		PricingModel: "tiered",
		BaseRate:     0.1,
		Unit:         "GB",
		Tiers: []PriceTierRule{
			{MinValue: 0, MaxValue: 100, Rate: 0.1, Unit: "GB"},
			{MinValue: 100, MaxValue: 1000, Rate: 0.08, Unit: "GB"},
			{MinValue: 1000, MaxValue: 0, Rate: 0.05, Unit: "GB"},
		},
		EffectiveFrom: time.Now().AddDate(-1, 0, 0),
		EffectiveTo:   time.Now().AddDate(1, 0, 0),
		Priority:      1,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	pm.priceRules["rule-compute"] = &PriceRule{
		ID:           "rule-compute",
		Name:         "计算资源价格规则",
		ResourceType: "compute",
		PricingModel: "pay_as_you_go",
		BaseRate:     0.5,
		Unit:         "hour",
		Tiers:        []PriceTierRule{},
		EffectiveFrom: time.Now().AddDate(-1, 0, 0),
		EffectiveTo:   time.Now().AddDate(1, 0, 0),
		Priority:      1,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	pm.priceRules["rule-network"] = &PriceRule{
		ID:           "rule-network",
		Name:         "网络流量价格规则",
		ResourceType: "network",
		PricingModel: "flat",
		BaseRate:     0.8,
		Unit:         "GB",
		Tiers:        []PriceTierRule{},
		EffectiveFrom: time.Now().AddDate(-1, 0, 0),
		EffectiveTo:   time.Now().AddDate(1, 0, 0),
		Priority:      1,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}
}

// CalculatePrice 计算价格
func (pm *PricingManager) CalculatePrice(userID string, resourceType string, usage float64) (*PriceCalculation, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 获取用户计费方案
	plan, err := pm.getUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// 获取价格规则
	rule, ok := pm.priceRules[fmt.Sprintf("rule-%s", resourceType)]
	if !ok {
		return nil, fmt.Errorf("no price rule for resource type: %s", resourceType)
	}

	// 获取层级信息
	tier, ok := pm.tiers[plan.TierID]
	if !ok {
		return nil, fmt.Errorf("tier not found: %s", plan.TierID)
	}

	// 计算基础费用
	baseFee := plan.BaseFee

	// 计算使用费用
	usageFee := 0.0
	if usage > plan.FreeQuota {
		billableUsage := usage - plan.FreeQuota
		usageFee = pm.calculateTieredPrice(rule, billableUsage)
	}

	// 计算超额费用
	overageFee := 0.0
	if tier.MaxUsage > 0 && usage > tier.MaxUsage {
		overage := usage - tier.MaxUsage
		overageFee = overage * plan.OverageRate
	}

	// 应用折扣
	discount := pm.calculateDiscount(plan, usage, usageFee)

	totalAmount := baseFee + usageFee + overageFee - discount

	return &PriceCalculation{
		UserID:       userID,
		ResourceType: resourceType,
		Usage:        usage,
		BaseFee:      baseFee,
		UsageFee:     usageFee,
		OverageFee:   overageFee,
		Discount:     discount,
		TotalAmount:  totalAmount,
		Currency:     pm.config.DefaultCurrency,
		CalculatedAt: time.Now(),
	}, nil
}

// PriceCalculation 价格计算结果
type PriceCalculation struct {
	UserID       string    `json:"user_id"`
	ResourceType string    `json:"resource_type"`
	Usage        float64   `json:"usage"`
	BaseFee      float64   `json:"base_fee"`
	UsageFee     float64   `json:"usage_fee"`
	OverageFee   float64   `json:"overage_fee"`
	Discount     float64   `json:"discount"`
	TotalAmount  float64   `json:"total_amount"`
	Currency     string    `json:"currency"`
	CalculatedAt time.Time `json:"calculated_at"`
}

// calculateTieredPrice 计算阶梯价格
func (pm *PricingManager) calculateTieredPrice(rule *PriceRule, usage float64) float64 {
	if len(rule.Tiers) == 0 {
		return usage * rule.BaseRate
	}

	total := 0.0
	remaining := usage

	for _, tier := range rule.Tiers {
		if remaining <= 0 {
			break
		}

		tierCapacity := tier.MaxValue - tier.MinValue
		if tier.MaxValue == 0 {
			tierCapacity = remaining
		}

		billable := remaining
		if billable > tierCapacity {
			billable = tierCapacity
		}

		total += billable * tier.Rate
		remaining -= billable
	}

	// 如果还有剩余（超出所有层级）
	if remaining > 0 {
		total += remaining * rule.BaseRate
	}

	return total
}

// calculateDiscount 计算折扣
func (pm *PricingManager) calculateDiscount(plan *BillingPlan, usage, usageFee float64) float64 {
	discount := 0.0

	for _, rule := range plan.DiscountRules {
		if !rule.IsActive {
			continue
		}
		if !time.Now().After(rule.StartDate) || !time.Now().Before(rule.EndDate) {
			continue
		}
		if usage < rule.MinUsage || (rule.MaxUsage > 0 && usage > rule.MaxUsage) {
			continue
		}

		switch rule.Type {
		case "percentage":
			discount += usageFee * rule.Value / 100
		case "fixed":
			discount += rule.Value
		}
	}

	return discount
}

// getUserPlan 获取用户计费方案
func (pm *PricingManager) getUserPlan(userID string) (*BillingPlan, error) {
	// 默认返回标准月付方案
	plan, ok := pm.plans["plan-monthly-standard"]
	if !ok {
		return nil, fmt.Errorf("no billing plan found for user: %s", userID)
	}
	return plan, nil
}

// GetUsageMetrics 获取使用量指标
func (pm *PricingManager) GetUsageMetrics(userID string, resourceType string, startTime, endTime time.Time) ([]*UsageMetric, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics, ok := pm.usageMetrics[userID]
	if !ok {
		return []*UsageMetric{}, nil
	}

	var result []*UsageMetric
	for _, m := range metrics {
		if resourceType != "" && m.ResourceType != resourceType {
			continue
		}
		if !startTime.IsZero() && m.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && m.Timestamp.After(endTime) {
			continue
		}
		result = append(result, m)
	}

	return result, nil
}

// ApplyDiscount 应用折扣
func (pm *PricingManager) ApplyDiscount(userID string, discountType string, value float64) (*DiscountApplication, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 获取用户方案
	plan, err := pm.getUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// 验证折扣类型
	if discountType != "percentage" && discountType != "fixed" {
		return nil, fmt.Errorf("invalid discount type: %s", discountType)
	}

	// 验证折扣值
	if discountType == "percentage" && (value <= 0 || value > 100) {
		return nil, fmt.Errorf("percentage discount must be between 0 and 100")
	}
	if discountType == "fixed" && value <= 0 {
		return nil, fmt.Errorf("fixed discount must be positive")
	}

	// 创建折扣规则
	rule := DiscountRule{
		ID:        fmt.Sprintf("discount-%s-%d", userID, time.Now().UnixNano()),
		Name:      fmt.Sprintf("用户折扣-%s", userID),
		Type:      discountType,
		Value:     value,
		MinUsage:  0,
		MaxUsage:  0,
		StartDate: time.Now(),
		EndDate:   time.Now().AddDate(0, 1, 0), // 有效期1个月
		IsActive:  true,
	}

	// 添加到方案
	plan.DiscountRules = append(plan.DiscountRules, rule)
	plan.UpdatedAt = time.Now()

	pm.logger.Info("Discount applied",
		zap.String("userID", userID),
		zap.String("type", discountType),
		zap.Float64("value", value),
	)

	return &DiscountApplication{
		DiscountID: rule.ID,
		UserID:     userID,
		Type:       discountType,
		Value:      value,
		StartDate:  rule.StartDate,
		EndDate:    rule.EndDate,
		AppliedAt:  time.Now(),
	}, nil
}

// DiscountApplication 折扣应用结果
type DiscountApplication struct {
	DiscountID string    `json:"discount_id"`
	UserID     string    `json:"user_id"`
	Type       string    `json:"type"`
	Value      float64   `json:"value"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	AppliedAt  time.Time `json:"applied_at"`
}

// GenerateInvoice 生成发票
func (pm *PricingManager) GenerateInvoice(userID string, periodStart, periodEnd time.Time) (*Invoice, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 获取用户方案
	plan, err := pm.getUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// 获取使用量
	metrics, err := pm.GetUsageMetrics(userID, "", periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	// 按资源类型汇总使用量
	usageByType := make(map[string]float64)
	for _, m := range metrics {
		usageByType[m.ResourceType] += m.Value
	}

	// 计算各项费用
	var items []InvoiceItem
	totalUsageFee := 0.0

	for resourceType, usage := range usageByType {
		calc, err := pm.calculatePriceInternal(plan, resourceType, usage)
		if err != nil {
			continue
		}

		items = append(items, InvoiceItem{
			Description:  fmt.Sprintf("%s 使用费", resourceType),
			ResourceType: resourceType,
			Quantity:     usage,
			Unit:         "GB",
			UnitPrice:    calc.UsageFee / usage,
			Amount:       calc.UsageFee,
		})
		totalUsageFee += calc.UsageFee
	}

	// 获取层级
	tier, _ := pm.tiers[plan.TierID]

	// 计算总费用
	baseFee := plan.BaseFee
	discount := pm.calculateDiscount(plan, totalUsageFee, totalUsageFee)
	tax := (baseFee + totalUsageFee - discount) * pm.config.TaxRate
	totalAmount := baseFee + totalUsageFee - discount + tax

	// 创建发票
	invoice := &Invoice{
		ID:            fmt.Sprintf("INV-%s-%d", userID, time.Now().UnixNano()),
		UserID:        userID,
		BillingPlanID: plan.ID,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		BaseFee:       baseFee,
		UsageFee:      totalUsageFee,
		OverageFee:    0,
		Discount:      discount,
		Tax:           tax,
		TotalAmount:   totalAmount,
		Currency:      pm.config.DefaultCurrency,
		Status:        "pending",
		Items:         items,
		CreatedAt:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, pm.config.GracePeriod),
	}

	// 存储发票
	pm.invoices[invoice.ID] = invoice

	pm.logger.Info("Invoice generated",
		zap.String("invoiceID", invoice.ID),
		zap.String("userID", userID),
		zap.Float64("totalAmount", totalAmount),
		zap.String("tier", tier.Name),
	)

	return invoice, nil
}

// calculatePriceInternal 内部价格计算
func (pm *PricingManager) calculatePriceInternal(plan *BillingPlan, resourceType string, usage float64) (*PriceCalculation, error) {
	rule, ok := pm.priceRules[fmt.Sprintf("rule-%s", resourceType)]
	if !ok {
		return nil, fmt.Errorf("no price rule for resource type: %s", resourceType)
	}

	usageFee := 0.0
	if usage > plan.FreeQuota {
		billableUsage := usage - plan.FreeQuota
		usageFee = pm.calculateTieredPrice(rule, billableUsage)
	}

	return &PriceCalculation{
		ResourceType: resourceType,
		Usage:        usage,
		UsageFee:     usageFee,
	}, nil
}

// GetTiers 获取所有定价层级
func (pm *PricingManager) GetTiers() []*PricingTier {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	tiers := make([]*PricingTier, 0, len(pm.tiers))
	for _, t := range pm.tiers {
		tiers = append(tiers, t)
	}
	return tiers
}

// GetPlans 获取所有计费方案
func (pm *PricingManager) GetPlans() []*BillingPlan {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plans := make([]*BillingPlan, 0, len(pm.plans))
	for _, p := range pm.plans {
		plans = append(plans, p)
	}
	return plans
}

// GetPriceRules 获取所有价格规则
func (pm *PricingManager) GetPriceRules() []*PriceRule {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	rules := make([]*PriceRule, 0, len(pm.priceRules))
	for _, r := range pm.priceRules {
		rules = append(rules, r)
	}
	return rules
}

// GetInvoice 获取发票
func (pm *PricingManager) GetInvoice(invoiceID string) (*Invoice, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	invoice, ok := pm.invoices[invoiceID]
	if !ok {
		return nil, fmt.Errorf("invoice not found: %s", invoiceID)
	}
	return invoice, nil
}

// GetUserInvoices 获取用户发票列表
func (pm *PricingManager) GetUserInvoices(userID string) []*Invoice {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var invoices []*Invoice
	for _, inv := range pm.invoices {
		if inv.UserID == userID {
			invoices = append(invoices, inv)
		}
	}
	return invoices
}
