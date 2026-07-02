// Package smartbilling 智能计费系统
// 支持家庭多用户资源使用计费，包括存储、带宽、CPU、内存等资源的按量计费，
// 预算管理，账单生成等功能。
package smartbilling

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ResourceType 资源类型.
type ResourceType string

const (
	ResourceStorage   ResourceType = "storage"   // 存储资源
	ResourceBandwidth ResourceType = "bandwidth" // 带宽资源
	ResourceCPU       ResourceType = "cpu"       // CPU资源
	ResourceMemory    ResourceType = "memory"    // 内存资源
)

// TierConfig 阶梯定价配置.
type TierConfig struct {
	Limit float64 // 阶梯上限（不含此值）
	Price float64 // 单位价格
}

// PricingStrategy 定价策略.
type PricingStrategy struct {
	ResourceType ResourceType // 资源类型
	Unit         string       // 计量单位（如 GB、MB/s、核、GB）
	Tiers        []TierConfig // 阶梯定价配置
}

// BudgetConfig 预算配置.
type BudgetConfig struct {
	Limit   float64 // 预算上限
	Period  string  // 预算周期（monthly、weekly、daily）
	Enabled bool    // 是否启用预算管理
}

// Account 用户账户.
type Account struct {
	ID          string       // 用户ID
	Name        string       // 用户名
	Budget      BudgetConfig // 预算配置
	TotalCost   float64      // 累计费用
	IsSuspended bool         // 是否暂停
	CreatedAt   time.Time    // 创建时间
	UpdatedAt   time.Time    // 更新时间
}

// UsageRecord 使用记录.
type UsageRecord struct {
	ID           string       // 记录ID
	AccountID    string       // 用户ID
	ResourceType ResourceType // 资源类型
	Amount       float64      // 使用量
	Cost         float64      // 费用
	Timestamp    time.Time    // 时间戳
}

// Invoice 账单.
type Invoice struct {
	ID        string        // 账单ID
	AccountID string        // 用户ID
	Period    string        // 账单周期
	Items     []InvoiceItem // 账单明细
	Total     float64       // 总费用
	CreatedAt time.Time     // 生成时间
}

// InvoiceItem 账单明细项.
type InvoiceItem struct {
	ResourceType ResourceType // 资源类型
	Usage        float64      // 使用量
	Unit         string       // 单位
	Cost         float64      // 费用
}

// BillingStats 计费统计信息.
type BillingStats struct {
	TotalRevenue   float64                  // 总收入
	AccountCount   int                      // 账户数量
	RecordCount    int                      // 使用记录数量
	ByResource     map[ResourceType]float64 // 各资源类型费用
	ByAccount      map[string]float64       // 各账户费用
	AvgCostPerUser float64                  // 平均每用户费用
}

// SmartBilling 智能计费系统主结构体.
type SmartBilling struct {
	mu              sync.RWMutex
	accounts        map[string]*Account
	usageRecords    []UsageRecord
	pricingStrategy map[ResourceType]*PricingStrategy
	nextRecordID    int
	nextInvoiceID   int
}

// NewSmartBilling 创建新的智能计费系统实例.
func NewSmartBilling() *SmartBilling {
	return &SmartBilling{
		accounts:        make(map[string]*Account),
		usageRecords:    make([]UsageRecord, 0),
		pricingStrategy: make(map[ResourceType]*PricingStrategy),
		nextRecordID:    1,
		nextInvoiceID:   1,
	}
}

// roundTo2 四舍五入保留2位小数.
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

// AddAccount 添加用户账户
// 如果账户ID已存在，返回错误.
func (sb *SmartBilling) AddAccount(id, name string, budget BudgetConfig) (*Account, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if _, exists := sb.accounts[id]; exists {
		return nil, fmt.Errorf("账户 %s 已存在", id)
	}

	now := time.Now()
	account := &Account{
		ID:          id,
		Name:        name,
		Budget:      budget,
		TotalCost:   0,
		IsSuspended: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	sb.accounts[id] = account

	// 返回副本
	copy := *account
	return &copy, nil
}

// GetAccount 获取用户账户信息
// 如果账户不存在，返回错误.
func (sb *SmartBilling) GetAccount(id string) (*Account, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	account, exists := sb.accounts[id]
	if !exists {
		return nil, fmt.Errorf("账户 %s 不存在", id)
	}

	copy := *account
	return &copy, nil
}

// ListAccounts 列出所有用户账户
// 返回账户列表的副本.
func (sb *SmartBilling) ListAccounts() []*Account {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	accounts := make([]*Account, 0, len(sb.accounts))
	for _, a := range sb.accounts {
		copy := *a
		accounts = append(accounts, &copy)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].ID < accounts[j].ID
	})
	return accounts
}

// SetPricingStrategy 设置定价策略.
func (sb *SmartBilling) SetPricingStrategy(strategy *PricingStrategy) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.pricingStrategy[strategy.ResourceType] = strategy
}

// GetPricingStrategy 获取定价策略.
func (sb *SmartBilling) GetPricingStrategy(resourceType ResourceType) (*PricingStrategy, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	strategy, exists := sb.pricingStrategy[resourceType]
	if !exists {
		return nil, fmt.Errorf("未找到资源类型 %s 的定价策略", resourceType)
	}

	copy := *strategy
	return &copy, nil
}

// RecordUsage 记录资源使用
// 如果账户暂停或不存在，返回错误.
func (sb *SmartBilling) RecordUsage(accountID string, resourceType ResourceType, amount float64) (*UsageRecord, error) {
	if amount < 0 {
		return nil, fmt.Errorf("使用量不能为负数")
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	account, exists := sb.accounts[accountID]
	if !exists {
		return nil, fmt.Errorf("账户 %s 不存在", accountID)
	}

	if account.IsSuspended {
		return nil, fmt.Errorf("账户 %s 已暂停", accountID)
	}

	strategy, exists := sb.pricingStrategy[resourceType]
	if !exists {
		return nil, fmt.Errorf("未找到资源类型 %s 的定价策略", resourceType)
	}

	// 计算费用
	cost := sb.calculateCost(strategy, amount)
	cost = roundTo2(cost)

	record := UsageRecord{
		ID:           fmt.Sprintf("UR-%d", sb.nextRecordID),
		AccountID:    accountID,
		ResourceType: resourceType,
		Amount:       amount,
		Cost:         cost,
		Timestamp:    time.Now(),
	}
	sb.nextRecordID++

	sb.usageRecords = append(sb.usageRecords, record)

	// 更新账户累计费用
	account.TotalCost = roundTo2(account.TotalCost + cost)
	account.UpdatedAt = time.Now()

	return &record, nil
}

// calculateCost 内部费用计算方法（使用阶梯定价）.
func (sb *SmartBilling) calculateCost(strategy *PricingStrategy, amount float64) float64 {
	if len(strategy.Tiers) == 0 {
		return 0
	}

	totalCost := 0.0
	remaining := amount

	for _, tier := range strategy.Tiers {
		if remaining <= 0 {
			break
		}

		var tierUsage float64
		if tier.Limit <= 0 {
			// 无限阶梯
			tierUsage = remaining
		} else {
			tierUsage = math.Min(remaining, tier.Limit)
		}

		totalCost += tierUsage * tier.Price
		remaining -= tierUsage
	}

	// 如果还有剩余（所有阶梯都有上限），按最后阶梯价格计算
	if remaining > 0 && len(strategy.Tiers) > 0 {
		totalCost += remaining * strategy.Tiers[len(strategy.Tiers)-1].Price
	}

	return totalCost
}

// CalculateBill 计算指定账户在指定时间段的费用
// 返回各项资源的使用量和费用明细.
func (sb *SmartBilling) CalculateBill(accountID string, start, end time.Time) (map[ResourceType]InvoiceItem, float64, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if _, exists := sb.accounts[accountID]; !exists {
		return nil, 0, fmt.Errorf("账户 %s 不存在", accountID)
	}

	// 按资源类型聚合使用量
	usageByType := make(map[ResourceType]float64)
	for _, record := range sb.usageRecords {
		if record.AccountID == accountID &&
			!record.Timestamp.Before(start) &&
			!record.Timestamp.After(end) {
			usageByType[record.ResourceType] += record.Amount
		}
	}

	totalCost := 0.0
	items := make(map[ResourceType]InvoiceItem)

	for rType, usage := range usageByType {
		strategy, exists := sb.pricingStrategy[rType]
		if !exists {
			continue
		}

		cost := roundTo2(sb.calculateCost(strategy, usage))
		totalCost += cost

		items[rType] = InvoiceItem{
			ResourceType: rType,
			Usage:        roundTo2(usage),
			Unit:         strategy.Unit,
			Cost:         cost,
		}
	}

	totalCost = roundTo2(totalCost)
	return items, totalCost, nil
}

// SetBudget 设置用户预算.
func (sb *SmartBilling) SetBudget(accountID string, budget BudgetConfig) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	account, exists := sb.accounts[accountID]
	if !exists {
		return fmt.Errorf("账户 %s 不存在", accountID)
	}

	account.Budget = budget
	account.UpdatedAt = time.Now()
	return nil
}

// CheckBudget 检查用户预算
// 返回：是否在预算内，已用金额，预算上限，是否超额.
func (sb *SmartBilling) CheckBudget(accountID string) (withinBudget bool, used float64, limit float64, exceeded bool, err error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	account, exists := sb.accounts[accountID]
	if !exists {
		return false, 0, 0, false, fmt.Errorf("账户 %s 不存在", accountID)
	}

	if !account.Budget.Enabled {
		return true, account.TotalCost, 0, false, nil
	}

	used = account.TotalCost
	limit = account.Budget.Limit
	exceeded = used > limit
	withinBudget = !exceeded

	return withinBudget, used, limit, exceeded, nil
}

// GenerateInvoice 生成账单
// 根据指定时间段生成用户账单.
func (sb *SmartBilling) GenerateInvoice(accountID, period string, start, end time.Time) (*Invoice, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	_, exists := sb.accounts[accountID]
	if !exists {
		return nil, fmt.Errorf("账户 %s 不存在", accountID)
	}

	// 按资源类型聚合使用量
	usageByType := make(map[ResourceType]float64)
	for _, record := range sb.usageRecords {
		if record.AccountID == accountID &&
			!record.Timestamp.Before(start) &&
			!record.Timestamp.After(end) {
			usageByType[record.ResourceType] += record.Amount
		}
	}

	var items []InvoiceItem
	total := 0.0

	// 按资源类型排序
	resourceTypes := make([]ResourceType, 0, len(usageByType))
	for rType := range usageByType {
		resourceTypes = append(resourceTypes, rType)
	}
	sort.Slice(resourceTypes, func(i, j int) bool {
		return string(resourceTypes[i]) < string(resourceTypes[j])
	})

	for _, rType := range resourceTypes {
		usage := usageByType[rType]
		strategy, exists := sb.pricingStrategy[rType]
		if !exists {
			continue
		}

		cost := roundTo2(sb.calculateCost(strategy, usage))
		total += cost

		items = append(items, InvoiceItem{
			ResourceType: rType,
			Usage:        roundTo2(usage),
			Unit:         strategy.Unit,
			Cost:         cost,
		})
	}

	total = roundTo2(total)

	invoice := &Invoice{
		ID:        fmt.Sprintf("INV-%d", sb.nextInvoiceID),
		AccountID: accountID,
		Period:    period,
		Items:     items,
		Total:     total,
		CreatedAt: time.Now(),
	}
	sb.nextInvoiceID++

	return invoice, nil
}

// GetStats 获取计费统计信息.
func (sb *SmartBilling) GetStats() *BillingStats {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	stats := &BillingStats{
		TotalRevenue: 0,
		AccountCount: len(sb.accounts),
		RecordCount:  len(sb.usageRecords),
		ByResource:   make(map[ResourceType]float64),
		ByAccount:    make(map[string]float64),
	}

	for _, record := range sb.usageRecords {
		stats.TotalRevenue += record.Cost
		stats.ByResource[record.ResourceType] += record.Cost
		stats.ByAccount[record.AccountID] += record.Cost
	}

	// 四舍五入
	stats.TotalRevenue = roundTo2(stats.TotalRevenue)
	for k := range stats.ByResource {
		stats.ByResource[k] = roundTo2(stats.ByResource[k])
	}
	for k := range stats.ByAccount {
		stats.ByAccount[k] = roundTo2(stats.ByAccount[k])
	}

	if stats.AccountCount > 0 {
		stats.AvgCostPerUser = roundTo2(stats.TotalRevenue / float64(stats.AccountCount))
	}

	return stats
}

// SuspendAccount 暂停用户账户.
func (sb *SmartBilling) SuspendAccount(accountID string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	account, exists := sb.accounts[accountID]
	if !exists {
		return fmt.Errorf("账户 %s 不存在", accountID)
	}

	account.IsSuspended = true
	account.UpdatedAt = time.Now()
	return nil
}

// ActivateAccount 激活用户账户.
func (sb *SmartBilling) ActivateAccount(accountID string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	account, exists := sb.accounts[accountID]
	if !exists {
		return fmt.Errorf("账户 %s 不存在", accountID)
	}

	account.IsSuspended = false
	account.UpdatedAt = time.Now()
	return nil
}

// GetUsageRecords 获取指定账户的使用记录.
func (sb *SmartBilling) GetUsageRecords(accountID string, start, end time.Time) []UsageRecord {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	var records []UsageRecord
	for _, record := range sb.usageRecords {
		if record.AccountID == accountID &&
			!record.Timestamp.Before(start) &&
			!record.Timestamp.After(end) {
			records = append(records, record)
		}
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records
}
