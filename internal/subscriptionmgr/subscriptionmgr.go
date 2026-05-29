// Package subscriptionmgr 提供云订阅服务管理功能。
// 支持多种云服务类型的订阅管理、费用统计、到期提醒和存储容量计算。
package subscriptionmgr

import (
	"errors"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrSubscriptionNotFound 订阅不存在.
	ErrSubscriptionNotFound = errors.New("订阅不存在")
	// ErrInvalidParams 无效输入参数.
	ErrInvalidParams = errors.New("无效输入参数")
	// ErrDuplicateID 重复的订阅ID.
	ErrDuplicateID = errors.New("订阅ID已存在")
)

// ========== 订阅类型 ==========

// SubscriptionType 云服务订阅类型.
type SubscriptionType string

const (
	// TypeCloudStorage 云存储.
	TypeCloudStorage SubscriptionType = "CloudStorage"
	// TypeCDN CDN加速.
	TypeCDN SubscriptionType = "CDN"
	// TypeVPN VPN服务.
	TypeVPN SubscriptionType = "VPN"
	// TypeEmail 邮件服务.
	TypeEmail SubscriptionType = "Email"
	// TypeDNS DNS服务.
	TypeDNS SubscriptionType = "DNS"
	// TypeBackup 备份服务.
	TypeBackup SubscriptionType = "Backup"
	// TypeOther 其他服务.
	TypeOther SubscriptionType = "Other"
)

// ========== 订阅状态 ==========

// SubscriptionStatus 订阅状态.
type SubscriptionStatus string

const (
	// StatusActive 活跃.
	StatusActive SubscriptionStatus = "active"
	// StatusExpired 已过期.
	StatusExpired SubscriptionStatus = "expired"
	// StatusCancelled 已取消.
	StatusCancelled SubscriptionStatus = "cancelled"
	// StatusSuspended 已暂停.
	StatusSuspended SubscriptionStatus = "suspended"
)

// ========== 核心数据结构 ==========

// Subscription 云订阅服务定义.
type Subscription struct {
	// ID 订阅唯一标识.
	ID string `json:"id"`
	// ServiceName 服务名称.
	ServiceName string `json:"service_name"`
	// Type 订阅类型.
	Type SubscriptionType `json:"type"`
	// Provider 服务提供商.
	Provider string `json:"provider"`
	// Cost 费用（元/周期）.
	Cost float64 `json:"cost"`
	// BillingCycle 计费周期（如 monthly, yearly）.
	BillingCycle string `json:"billing_cycle"`
	// ExpiryDate 到期日期.
	ExpiryDate time.Time `json:"expiry_date"`
	// StorageCapacityGB 存储容量（GB）.
	StorageCapacityGB int64 `json:"storage_capacity_gb"`
	// Status 订阅状态.
	Status SubscriptionStatus `json:"status"`
	// Notes 备注信息.
	Notes string `json:"notes"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// CostSummary 费用汇总.
type CostSummary struct {
	// MonthlyTotal 月度总费用.
	MonthlyTotal float64 `json:"monthly_total"`
	// YearlyTotal 年度总费用.
	YearlyTotal float64 `json:"yearly_total"`
	// ByType 按类型分组的费用.
	ByType map[SubscriptionType]TypeCostSummary `json:"by_type"`
}

// TypeCostSummary 按类型汇总的费用.
type TypeCostSummary struct {
	// Type 订阅类型.
	Type SubscriptionType `json:"type"`
	// MonthlyCost 月度费用.
	MonthlyCost float64 `json:"monthly_cost"`
	// YearlyCost 年度费用.
	YearlyCost float64 `json:"yearly_cost"`
	// Count 订阅数量.
	Count int `json:"count"`
}

// ========== 订阅管理器 ==========

// Manager 订阅管理器.
type Manager struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
}

// NewManager 创建订阅管理器.
func NewManager() *Manager {
	return &Manager{
		subscriptions: make(map[string]*Subscription),
	}
}

// ========== 订阅 CRUD ==========

// AddSubscription 添加云订阅服务.
func (m *Manager) AddSubscription(sub Subscription) error {
	if sub.ID == "" || sub.ServiceName == "" {
		return ErrInvalidParams
	}
	if sub.Type == "" {
		sub.Type = TypeOther
	}
	if sub.Status == "" {
		sub.Status = StatusActive
	}
	if sub.BillingCycle == "" {
		sub.BillingCycle = "monthly"
	}
	now := time.Now()
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = now
	}
	sub.UpdatedAt = now

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.subscriptions[sub.ID]; exists {
		return ErrDuplicateID
	}
	m.subscriptions[sub.ID] = &sub
	return nil
}

// GetSubscription 获取订阅详情.
func (m *Manager) GetSubscription(id string) (*Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sub, ok := m.subscriptions[id]
	if !ok {
		return nil, ErrSubscriptionNotFound
	}
	return sub, nil
}

// ListSubscriptions 列出所有订阅.
func (m *Manager) ListSubscriptions() ([]Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Subscription, 0, len(m.subscriptions))
	for _, s := range m.subscriptions {
		result = append(result, *s)
	}
	return result, nil
}

// UpdateSubscription 更新订阅信息.
func (m *Manager) UpdateSubscription(sub Subscription) error {
	if sub.ID == "" {
		return ErrInvalidParams
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.subscriptions[sub.ID]
	if !ok {
		return ErrSubscriptionNotFound
	}
	if sub.ServiceName != "" {
		existing.ServiceName = sub.ServiceName
	}
	if sub.Type != "" {
		existing.Type = sub.Type
	}
	if sub.Provider != "" {
		existing.Provider = sub.Provider
	}
	if sub.Cost > 0 {
		existing.Cost = sub.Cost
	}
	if sub.BillingCycle != "" {
		existing.BillingCycle = sub.BillingCycle
	}
	if !sub.ExpiryDate.IsZero() {
		existing.ExpiryDate = sub.ExpiryDate
	}
	if sub.StorageCapacityGB > 0 {
		existing.StorageCapacityGB = sub.StorageCapacityGB
	}
	if sub.Status != "" {
		existing.Status = sub.Status
	}
	if sub.Notes != "" {
		existing.Notes = sub.Notes
	}
	existing.UpdatedAt = time.Now()
	return nil
}

// DeleteSubscription 删除订阅.
func (m *Manager) DeleteSubscription(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.subscriptions[id]; !ok {
		return ErrSubscriptionNotFound
	}
	delete(m.subscriptions, id)
	return nil
}

// ========== 费用分析 ==========

// GetCostSummary 获取费用汇总.
func (m *Manager) GetCostSummary() (*CostSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &CostSummary{
		ByType: make(map[SubscriptionType]TypeCostSummary),
	}

	for _, sub := range m.subscriptions {
		if sub.Status != StatusActive {
			continue
		}
		monthly := toMonthlyCost(sub.Cost, sub.BillingCycle)
		summary.MonthlyTotal += monthly
		summary.YearlyTotal += monthly * 12

		ts := summary.ByType[sub.Type]
		ts.Type = sub.Type
		ts.MonthlyCost += monthly
		ts.YearlyCost += monthly * 12
		ts.Count++
		summary.ByType[sub.Type] = ts
	}

	return summary, nil
}

// toMonthlyCost 将不同周期的费用转换为月度费用.
func toMonthlyCost(cost float64, cycle string) float64 {
	switch cycle {
	case "yearly":
		return cost / 12
	case "quarterly":
		return cost / 3
	default: // monthly
		return cost
	}
}

// ========== 到期提醒 ==========

// GetExpiringSubscriptions 获取即将到期的订阅.
// days 参数指定提前多少天提醒.
func (m *Manager) GetExpiringSubscriptions(days int) ([]Subscription, error) {
	if days < 0 {
		return nil, ErrInvalidParams
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	deadline := time.Now().AddDate(0, 0, days)
	var result []Subscription
	for _, sub := range m.subscriptions {
		if sub.Status == StatusActive && !sub.ExpiryDate.IsZero() && sub.ExpiryDate.Before(deadline) {
			result = append(result, *sub)
		}
	}
	return result, nil
}

// ========== 存储容量统计 ==========

// CalculateTotalStorageCapacity 计算所有活跃订阅的总存储容量.
func (m *Manager) CalculateTotalStorageCapacity() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var total int64
	for _, sub := range m.subscriptions {
		if sub.Status == StatusActive {
			total += sub.StorageCapacityGB
		}
	}
	return total, nil
}

// ========== 类型筛选 ==========

// GetSubscriptionsByType 按类型筛选订阅.
func (m *Manager) GetSubscriptionsByType(subType SubscriptionType) ([]Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Subscription
	for _, sub := range m.subscriptions {
		if sub.Type == subType {
			result = append(result, *sub)
		}
	}
	return result, nil
}
