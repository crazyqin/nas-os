package storagebilling

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager 存储计费引擎管理器.
type Manager struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant
	usages   map[string][]*UsageRecord // tenantID -> records
	quotas   map[string][]*StorageQuota // tenantID -> quotas
	bills    map[string]*StorageBill
	rates    map[StorageTier]float64
	nextBill int64
}

// NewManager 创建存储计费管理器.
func NewManager() *Manager {
	return &Manager{
		tenants: make(map[string]*Tenant),
		usages:  make(map[string][]*UsageRecord),
		quotas:  make(map[string][]*StorageQuota),
		bills:   make(map[string]*StorageBill),
		rates: map[StorageTier]float64{
			TierSSD:     1.5, // 1.5元/GB/月
			TierHDD:     0.5, // 0.5元/GB/月
			TierArchive: 0.1, // 0.1元/GB/月
		},
		nextBill: 1,
	}
}

// ========== 费率管理 ==========

// GetTierRates 获取所有层级费率.
func (m *Manager) GetTierRates() []TierRate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	descriptions := map[StorageTier]string{
		TierSSD:     "SSD高速存储 - 适用于热数据、数据库、虚拟机",
		TierHDD:     "HDD大容量存储 - 适用于温数据、文件共享、备份",
		TierArchive: "归档存储 - 适用于冷数据、合规归档、历史备份",
	}

	result := make([]TierRate, 0, len(m.rates))
	for tier, rate := range m.rates {
		result = append(result, TierRate{
			Tier:        tier,
			RatePerGB:   rate,
			Description: descriptions[tier],
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].RatePerGB > result[j].RatePerGB
	})
	return result
}

// SetTierRate 设置层级费率.
func (m *Manager) SetTierRate(tier StorageTier, ratePerGB float64) error {
	if tier == "" {
		return ErrInvalidParams
	}
	if ratePerGB < 0 {
		return ErrInvalidParams
	}

	m.mu.Lock()
	m.rates[tier] = ratePerGB
	m.mu.Unlock()
	return nil
}

// ========== 租户管理 ==========

// CreateTenant 创建租户.
func (m *Manager) CreateTenant(t *Tenant) error {
	if t.ID == "" || t.Name == "" {
		return ErrInvalidParams
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tenants[t.ID]; exists {
		return ErrDuplicateTenant
	}

	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	t.IsActive = true

	m.tenants[t.ID] = t
	return nil
}

// GetTenant 获取租户.
func (m *Manager) GetTenant(id string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[id]
	if !ok {
		return nil, ErrTenantNotFound
	}
	return t, nil
}

// ListTenants 列出所有租户.
func (m *Manager) ListTenants() []*Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		result = append(result, t)
	}
	return result
}

// ListTenantsByDepartment 按部门列出租户.
func (m *Manager) ListTenantsByDepartment(dept string) []*Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Tenant
	for _, t := range m.tenants {
		if t.Department == dept {
			result = append(result, t)
		}
	}
	return result
}

// UpdateTenant 更新租户.
func (m *Manager) UpdateTenant(id string, updates *Tenant) (*Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[id]
	if !ok {
		return nil, ErrTenantNotFound
	}

	if updates.Name != "" {
		t.Name = updates.Name
	}
	if updates.Department != "" {
		t.Department = updates.Department
	}
	if updates.Project != "" {
		t.Project = updates.Project
	}
	if updates.Contact != "" {
		t.Contact = updates.Contact
	}
	if updates.Email != "" {
		t.Email = updates.Email
	}
	t.UpdatedAt = time.Now()

	return t, nil
}

// DeleteTenant 删除租户.
func (m *Manager) DeleteTenant(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tenants[id]; !ok {
		return ErrTenantNotFound
	}

	delete(m.tenants, id)
	delete(m.usages, id)
	delete(m.quotas, id)
	return nil
}

// ========== 用量统计 ==========

// RecordUsage 记录存储用量.
func (m *Manager) RecordUsage(tenantID string, tier StorageTier, usedGB float64) error {
	if tenantID == "" || tier == "" {
		return ErrInvalidParams
	}
	if usedGB < 0 {
		return ErrInvalidParams
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证租户存在
	if _, ok := m.tenants[tenantID]; !ok {
		return ErrTenantNotFound
	}

	record := &UsageRecord{
		TenantID:     tenantID,
		Tier:         tier,
		UsedGB:       usedGB,
		SnapshotTime: time.Now(),
	}

	m.usages[tenantID] = append(m.usages[tenantID], record)

	// 更新配额使用量
	for _, q := range m.quotas[tenantID] {
		if q.Tier == tier && q.IsActive {
			q.UsedGB = usedGB
			q.UpdatedAt = time.Now()
		}
	}

	return nil
}

// GetUsageSummary 获取租户用量汇总.
func (m *Manager) GetUsageSummary(tenantID string) (*UsageSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	summary := &UsageSummary{
		TenantID:   tenantID,
		TenantName: t.Name,
		Department: t.Department,
		Project:    t.Project,
	}

	// 获取各层级最新用量
	latestUsage := m.getLatestUsage(tenantID)
	summary.SSDUsage = latestUsage[TierSSD]
	summary.HDDUsage = latestUsage[TierHDD]
	summary.ArchiveUsage = latestUsage[TierArchive]
	summary.TotalUsage = summary.SSDUsage + summary.HDDUsage + summary.ArchiveUsage

	// 获取配额信息
	for _, q := range m.quotas[tenantID] {
		if !q.IsActive {
			continue
		}
		switch q.Tier {
		case TierSSD:
			summary.SSDQuota += q.QuotaGB
		case TierHDD:
			summary.HDDQuota += q.QuotaGB
		case TierArchive:
			summary.ArchiveQuota += q.QuotaGB
		}
	}
	summary.TotalQuota = summary.SSDQuota + summary.HDDQuota + summary.ArchiveQuota

	// 计算配额使用率
	if summary.TotalQuota > 0 {
		summary.QuotaUsageRate = summary.TotalUsage / summary.TotalQuota
	}

	// 计算预估月费用
	summary.EstimatedCost = m.calculateCost(latestUsage)

	return summary, nil
}

// getLatestUsage 获取各层级最新用量（内部方法，调用前需持有锁）.
func (m *Manager) getLatestUsage(tenantID string) map[StorageTier]float64 {
	result := make(map[StorageTier]float64)
	records := m.usages[tenantID]

	// 取每个层级最新的记录
	latest := make(map[StorageTier]time.Time)
	for _, r := range records {
		if prev, ok := latest[r.Tier]; !ok || r.SnapshotTime.After(prev) {
			latest[r.Tier] = r.SnapshotTime
			result[r.Tier] = r.UsedGB
		}
	}

	return result
}

// calculateCost 计算费用（内部方法，调用前需持有锁）.
func (m *Manager) calculateCost(usage map[StorageTier]float64) float64 {
	var total float64
	for tier, gb := range usage {
		if rate, ok := m.rates[tier]; ok {
			total += gb * rate
		}
	}
	return total
}

// GetUsageHistory 获取用量历史记录.
func (m *Manager) GetUsageHistory(tenantID string, tier StorageTier, since time.Time) []*UsageRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*UsageRecord
	for _, r := range m.usages[tenantID] {
		if (tier == "" || r.Tier == tier) && r.SnapshotTime.After(since) {
			result = append(result, r)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SnapshotTime.Before(result[j].SnapshotTime)
	})

	return result
}

// ========== 配额管理 ==========

// CreateQuota 创建存储配额.
func (m *Manager) CreateQuota(q *StorageQuota) error {
	if q.ID == "" || q.TenantID == "" || q.Tier == "" {
		return ErrInvalidParams
	}
	if q.QuotaGB <= 0 {
		return ErrInvalidParams
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证租户存在
	if _, ok := m.tenants[q.TenantID]; !ok {
		return ErrTenantNotFound
	}

	if q.AlertThreshold <= 0 || q.AlertThreshold > 1 {
		q.AlertThreshold = 0.85
	}

	now := time.Now()
	if q.CreatedAt.IsZero() {
		q.CreatedAt = now
	}
	q.UpdatedAt = now
	q.IsActive = true

	m.quotas[q.TenantID] = append(m.quotas[q.TenantID], q)
	return nil
}

// GetQuotas 获取租户的所有配额.
func (m *Manager) GetQuotas(tenantID string) ([]*StorageQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.tenants[tenantID]; !ok {
		return nil, ErrTenantNotFound
	}

	quotas := m.quotas[tenantID]
	result := make([]*StorageQuota, len(quotas))
	copy(result, quotas)
	return result, nil
}

// UpdateQuota 更新配额.
func (m *Manager) UpdateQuota(quotaID string, updates *StorageQuota) (*StorageQuota, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, quotas := range m.quotas {
		for _, q := range quotas {
			if q.ID == quotaID {
				if updates.QuotaGB > 0 {
					q.QuotaGB = updates.QuotaGB
				}
				if updates.AlertThreshold > 0 && updates.AlertThreshold <= 1 {
					q.AlertThreshold = updates.AlertThreshold
				}
				q.HardLimit = updates.HardLimit
				q.UpdatedAt = time.Now()
				return q, nil
			}
		}
	}

	return nil, ErrQuotaNotFound
}

// DeleteQuota 删除配额.
func (m *Manager) DeleteQuota(quotaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for tenantID, quotas := range m.quotas {
		for i, q := range quotas {
			if q.ID == quotaID {
				m.quotas[tenantID] = append(quotas[:i], quotas[i+1:]...)
				return nil
			}
		}
	}

	return ErrQuotaNotFound
}

// CheckQuotaExceeded 检查配额超限情况.
func (m *Manager) CheckQuotaExceeded() []*StorageQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var exceeded []*StorageQuota
	for _, quotas := range m.quotas {
		for _, q := range quotas {
			if q.IsActive && q.HardLimit && q.UsedGB > q.QuotaGB {
				exceeded = append(exceeded, q)
			}
		}
	}
	return exceeded
}

// CheckQuotaAlerts 检查配额告警（接近超限）.
func (m *Manager) CheckQuotaAlerts() []*StorageQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []*StorageQuota
	for _, quotas := range m.quotas {
		for _, q := range quotas {
			if !q.IsActive || q.QuotaGB == 0 {
				continue
			}
			usageRate := q.UsedGB / q.QuotaGB
			if usageRate >= q.AlertThreshold {
				alerts = append(alerts, q)
			}
		}
	}
	return alerts
}

// ========== 账单生成 ==========

// GenerateBill 生成账单.
func (m *Manager) GenerateBill(tenantID string, cycle BillingCycle, periodStart, periodEnd time.Time) (*StorageBill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	// 计算周期内的平均用量
	avgUsage := m.calculateAverageUsage(tenantID, periodStart, periodEnd)

	// 计算各层级费用
	var tierCharges []TierCharge
	var totalAmount float64

	for tier, avgGB := range avgUsage {
		rate := m.rates[tier]
		amount := avgGB * rate
		tierCharges = append(tierCharges, TierCharge{
			Tier:      tier,
			UsedGB:    avgGB,
			RatePerGB: rate,
			Amount:    amount,
		})
		totalAmount += amount
	}

	// 按费率排序
	sort.Slice(tierCharges, func(i, j int) bool {
		return tierCharges[i].RatePerGB > tierCharges[j].RatePerGB
	})

	billID := fmt.Sprintf("bill-%d", m.nextBill)
	m.nextBill++

	bill := &StorageBill{
		ID:           billID,
		TenantID:     tenantID,
		TenantName:   t.Name,
		BillingCycle: cycle,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TierCharges:  tierCharges,
		TotalAmount:  totalAmount,
		Currency:     "CNY",
		Status:       BillStatusDraft,
		CreatedAt:    time.Now(),
	}

	m.bills[billID] = bill
	return bill, nil
}

// calculateAverageUsage 计算周期内平均用量（内部方法，调用前需持有写锁）.
func (m *Manager) calculateAverageUsage(tenantID string, start, end time.Time) map[StorageTier]float64 {
	records := m.usages[tenantID]
	tierRecords := make(map[StorageTier][]float64)

	for _, r := range records {
		if !r.SnapshotTime.Before(start) && !r.SnapshotTime.After(end) {
			tierRecords[r.Tier] = append(tierRecords[r.Tier], r.UsedGB)
		}
	}

	result := make(map[StorageTier]float64)
	for tier, values := range tierRecords {
		if len(values) == 0 {
			continue
		}
		var sum float64
		for _, v := range values {
			sum += v
		}
		result[tier] = sum / float64(len(values))
	}

	// 如果周期内没有记录，取最新用量
	if len(result) == 0 {
		latest := m.getLatestUsage(tenantID)
		for tier, gb := range latest {
			result[tier] = gb
		}
	}

	return result
}

// GetBill 获取账单.
func (m *Manager) GetBill(billID string) (*StorageBill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bill, ok := m.bills[billID]
	if !ok {
		return nil, ErrBillNotFound
	}
	return bill, nil
}

// ListBills 列出租户账单.
func (m *Manager) ListBills(tenantID string) []*StorageBill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*StorageBill
	for _, bill := range m.bills {
		if tenantID == "" || bill.TenantID == tenantID {
			result = append(result, bill)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// UpdateBillStatus 更新账单状态.
func (m *Manager) UpdateBillStatus(billID string, status BillStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bill, ok := m.bills[billID]
	if !ok {
		return ErrBillNotFound
	}

	bill.Status = status
	if status == BillStatusPaid {
		bill.PaidAt = time.Now()
	}

	return nil
}

// GenerateMonthlyBills 生成所有租户的月度账单.
func (m *Manager) GenerateMonthlyBills(year int, month time.Month) []*StorageBill {
	m.mu.Lock()
	tenantIDs := make([]string, 0, len(m.tenants))
	for id := range m.tenants {
		tenantIDs = append(tenantIDs, id)
	}
	m.mu.Unlock()

	periodStart := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var bills []*StorageBill
	for _, tenantID := range tenantIDs {
		bill, err := m.GenerateBill(tenantID, CycleMonthly, periodStart, periodEnd)
		if err == nil {
			bills = append(bills, bill)
		}
	}

	return bills
}

// GenerateQuarterlyBills 生成所有租户的季度账单.
func (m *Manager) GenerateQuarterlyBills(year int, quarter int) []*StorageBill {
	if quarter < 1 || quarter > 4 {
		return nil
	}

	startMonth := time.Month((quarter-1)*3 + 1)
	periodStart := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
	periodEnd := periodStart.AddDate(0, 3, 0).Add(-time.Nanosecond)

	m.mu.Lock()
	tenantIDs := make([]string, 0, len(m.tenants))
	for id := range m.tenants {
		tenantIDs = append(tenantIDs, id)
	}
	m.mu.Unlock()

	var bills []*StorageBill
	for _, tenantID := range tenantIDs {
		bill, err := m.GenerateBill(tenantID, CycleQuarterly, periodStart, periodEnd)
		if err == nil {
			bills = append(bills, bill)
		}
	}

	return bills
}

// ========== 成本优化 ==========

// AnalyzeCostOptimization 分析成本优化建议.
func (m *Manager) AnalyzeCostOptimization(tenantID string) (*CostOptimization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	latestUsage := m.getLatestUsage(tenantID)
	currentCost := m.calculateCost(latestUsage)

	optimization := &CostOptimization{
		TenantID:   tenantID,
		TenantName: t.Name,
		CurrentCost: currentCost,
	}

	// 分析建议
	var suggestions []OptimizationSuggestion
	var totalSavings float64

	// 1. 冷数据迁移到归档存储
	ssdUsage := latestUsage[TierSSD]
	hddUsage := latestUsage[TierHDD]

	// 假设SSD中30%数据可以迁移到HDD
	if ssdUsage > 10 {
		migratableSSD := ssdUsage * 0.3
		savings := migratableSSD * (m.rates[TierSSD] - m.rates[TierHDD])
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:             "tier_migration",
			Description:      fmt.Sprintf("%.1fGB SSD数据可迁移到HDD存储，适用于不常访问的数据", migratableSSD),
			EstimatedSavings: savings,
			Priority:         "high",
		})
		totalSavings += savings
	}

	// 2. HDD中50%冷数据可以归档
	if hddUsage > 20 {
		archivable := hddUsage * 0.5
		savings := archivable * (m.rates[TierHDD] - m.rates[TierArchive])
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:             "tier_migration",
			Description:      fmt.Sprintf("%.1fGB HDD数据可归档存储，适用于历史备份和合规数据", archivable),
			EstimatedSavings: savings,
			Priority:         "medium",
		})
		totalSavings += savings
	}

	// 3. 数据压缩建议
	totalUsage := latestUsage[TierSSD] + latestUsage[TierHDD] + latestUsage[TierArchive]
	if totalUsage > 100 {
		compressionSavings := totalUsage * 0.2 * m.rates[TierHDD] // 假设压缩率20%，按HDD费率计算
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:             "compression",
			Description:      "启用数据压缩功能，预计可减少20%存储占用",
			EstimatedSavings: compressionSavings,
			Priority:         "medium",
		})
		totalSavings += compressionSavings
	}

	// 4. 配额优化建议
	quotas := m.quotas[tenantID]
	for _, q := range quotas {
		if q.IsActive && q.QuotaGB > 0 && q.UsedGB < q.QuotaGB*0.3 {
			wastedQuota := q.QuotaGB - q.UsedGB
			savings := wastedQuota * m.rates[q.Tier] * 0.5 // 建议缩减50%未使用配额
			suggestions = append(suggestions, OptimizationSuggestion{
				Type:             "quota_optimization",
				Description:      fmt.Sprintf("%s存储配额使用率低于30%%，建议缩减配额以节省成本", q.Tier),
				EstimatedSavings: savings,
				Priority:         "low",
			})
			totalSavings += savings
		}
	}

	optimization.PotentialSavings = totalSavings
	optimization.Suggestions = suggestions

	return optimization, nil
}

// GetAllTenantSummaries 获取所有租户的用量汇总.
func (m *Manager) GetAllTenantSummaries() []*UsageSummary {
	m.mu.RLock()
	tenantIDs := make([]string, 0, len(m.tenants))
	for id := range m.tenants {
		tenantIDs = append(tenantIDs, id)
	}
	m.mu.RUnlock()

	var summaries []*UsageSummary
	for _, id := range tenantIDs {
		summary, err := m.GetUsageSummary(id)
		if err == nil {
			summaries = append(summaries, summary)
		}
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].TotalUsage > summaries[j].TotalUsage
	})

	return summaries
}
