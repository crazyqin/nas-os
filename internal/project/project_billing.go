// Package project provides project management functionality
package project

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 计费相关错误 ==========

var (
	// ErrQuotaExceeded 资源配额已超限错误
	ErrQuotaExceeded = errors.New("资源配额已超限")
	// ErrQuotaNotFound 配额配置不存在错误
	ErrQuotaNotFound = errors.New("配额配置不存在")
	// ErrBillingNotFound 计费记录不存在错误
	ErrBillingNotFound = errors.New("计费记录不存在")
	// ErrInvalidCostAmount 无效成本金额错误
	ErrInvalidCostAmount = errors.New("无效的成本金额")
	// ErrInvalidQuotaValue 无效配额值错误
	ErrInvalidQuotaValue = errors.New("无效的配额值")
)

// ========== 资源配额类型 ==========

// ResourceType 资源类型
type ResourceType string

// 资源类型常量
const (
	ResourceTypeCPU     ResourceType = "cpu"     // CPU核心数
	ResourceTypeMemory  ResourceType = "memory"  // 内存(MB)
	ResourceTypeStorage ResourceType = "storage" // 存储(GB)
	ResourceTypeNetwork ResourceType = "network" // 网络带宽(Mbps)
	ResourceTypeGPU     ResourceType = "gpu"     // GPU数量
	ResourceTypeAPI     ResourceType = "api"     // API调用次数
	ResourceTypeUser    ResourceType = "user"    // 用户数
	ResourceTypeProject ResourceType = "project" // 项目数
)

// QuotaScope 配额范围
type QuotaScope string

// 配额范围常量
const (
	QuotaScopeProject QuotaScope = "project" // 项目级别
	QuotaScopeUser    QuotaScope = "user"    // 用户级别
	QuotaScopeTeam    QuotaScope = "team"    // 团队级别
)

// ========== 资源配额定义 ==========

// ResourceQuota 资源配额
type ResourceQuota struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"project_id"`
	ResourceType ResourceType `json:"resource_type"`
	Scope        QuotaScope   `json:"scope"`

	// 配额限制
	HardLimit int64 `json:"hard_limit"` // 硬限制（绝对上限）
	SoftLimit int64 `json:"soft_limit"` // 软限制（告警阈值）
	Used      int64 `json:"used"`       // 已使用量
	Reserved  int64 `json:"reserved"`   // 预留量

	// 计费相关
	UnitPrice    float64 `json:"unit_price"`    // 单价
	Currency     string  `json:"currency"`      // 货币类型
	BillingCycle string  `json:"billing_cycle"` // 计费周期：hourly/daily/monthly

	// 告警配置
	AlertThreshold int      `json:"alert_threshold"` // 告警阈值百分比
	AlertEmails    []string `json:"alert_emails,omitempty"`

	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QuotaUsage 配额使用记录
type QuotaUsage struct {
	ID           string                 `json:"id"`
	ProjectID    string                 `json:"project_id"`
	ResourceType ResourceType           `json:"resource_type"`
	Amount       int64                  `json:"amount"` // 使用量
	Action       string                 `json:"action"` // allocate/release/consume
	Timestamp    time.Time              `json:"timestamp"`
	UserID       string                 `json:"user_id,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ========== 计费记录 ==========

// BillingRecord 计费记录
type BillingRecord struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// 费用明细
	ResourceCosts []ResourceCost `json:"resource_costs"`
	TotalCost     float64        `json:"total_cost"`
	Currency      string         `json:"currency"`
	Discount      float64        `json:"discount,omitempty"` // 折扣比例
	TaxRate       float64        `json:"tax_rate,omitempty"` // 税率
	FinalAmount   float64        `json:"final_amount"`       // 最终金额

	// 状态
	Status  string     `json:"status"` // draft/pending/paid/overdue/cancelled
	PaidAt  *time.Time `json:"paid_at,omitempty"`
	DueDate *time.Time `json:"due_date,omitempty"`

	// 元数据
	InvoiceNumber string `json:"invoice_number,omitempty"`
	Notes         string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResourceCost 资源成本
type ResourceCost struct {
	ResourceType ResourceType `json:"resource_type"`
	Quantity     int64        `json:"quantity"`    // 使用量
	UnitPrice    float64      `json:"unit_price"`  // 单价
	UsageHours   float64      `json:"usage_hours"` // 使用时长（小时）
	Subtotal     float64      `json:"subtotal"`    // 小计
	Description  string       `json:"description,omitempty"`
}

// ========== 成本分析 ==========

// CostAnalysis 成本分析报告
type CostAnalysis struct {
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// 总成本
	TotalCost float64 `json:"total_cost"`
	Currency  string  `json:"currency"`

	// 按资源类型分类
	ByResourceType map[string]ResourceCostSummary `json:"by_resource_type"`

	// 趋势数据
	DailyCost   []DailyCost   `json:"daily_cost,omitempty"`
	MonthlyCost []MonthlyCost `json:"monthly_cost,omitempty"`

	// 预测
	ForecastNextMonth float64 `json:"forecast_next_month,omitempty"`
	ForecastTrend     string  `json:"forecast_trend,omitempty"` // increasing/stable/decreasing

	// 对比
	PreviousPeriod *PeriodComparison `json:"previous_period,omitempty"`

	// 优化建议
	Recommendations []CostRecommendation `json:"recommendations,omitempty"`
}

// ResourceCostSummary 资源成本汇总
type ResourceCostSummary struct {
	ResourceType  ResourceType `json:"resource_type"`
	TotalCost     float64      `json:"total_cost"`
	TotalQuantity int64        `json:"total_quantity"`
	AvgDailyCost  float64      `json:"avg_daily_cost"`
	Percentage    float64      `json:"percentage"` // 占总成本百分比
	Trend         string       `json:"trend"`      // up/down/stable
}

// DailyCost 每日成本
type DailyCost struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

// MonthlyCost 每月成本
type MonthlyCost struct {
	Month string  `json:"month"`
	Cost  float64 `json:"cost"`
}

// PeriodComparison 周期对比
type PeriodComparison struct {
	PreviousCost  float64 `json:"previous_cost"`
	ChangeAmount  float64 `json:"change_amount"`
	ChangePercent float64 `json:"change_percent"`
}

// CostRecommendation 成本优化建议
type CostRecommendation struct {
	Type             string  `json:"type"` // reduce/optimize/resize/schedule
	ResourceType     string  `json:"resource_type"`
	CurrentCost      float64 `json:"current_cost"`
	PotentialSavings float64 `json:"potential_savings"`
	Description      string  `json:"description"`
	Priority         string  `json:"priority"` // high/medium/low
}

// ========== 资源使用报告 ==========

// ResourceUsageReport 资源使用报告
type ResourceUsageReport struct {
	ProjectID    string    `json:"project_id"`
	ProjectName  string    `json:"project_name"`
	ReportPeriod string    `json:"report_period"`
	GeneratedAt  time.Time `json:"generated_at"`

	// 资源使用概览
	Resources      []ResourceUsageDetail `json:"resources"`
	TotalResources int                   `json:"total_resources"`

	// 配额使用情况
	QuotaUsage []QuotaUsageSummary `json:"quota_usage"`
	OverQuota  []OverQuotaAlert    `json:"over_quota,omitempty"`

	// 使用统计
	PeakUsage       map[string]int64   `json:"peak_usage"`       // 各资源峰值使用
	AvgUsage        map[string]float64 `json:"avg_usage"`        // 各资源平均使用
	UtilizationRate map[string]float64 `json:"utilization_rate"` // 利用率

	// 趋势数据
	UsageTrend []UsageTrendPoint `json:"usage_trend,omitempty"`

	// 成本关联
	TotalCost float64 `json:"total_cost"`
	Currency  string  `json:"currency"`
}

// ResourceUsageDetail 资源使用详情
type ResourceUsageDetail struct {
	ResourceType    ResourceType `json:"resource_type"`
	Allocated       int64        `json:"allocated"`        // 分配量
	Used            int64        `json:"used"`             // 使用量
	Reserved        int64        `json:"reserved"`         // 预留量
	Available       int64        `json:"available"`        // 可用量
	UtilizationRate float64      `json:"utilization_rate"` // 利用率
	QuotaLimit      int64        `json:"quota_limit"`      // 配额限制
	QuotaUsed       int64        `json:"quota_used"`       // 配额已用
	QuotaPercent    float64      `json:"quota_percent"`    // 配额使用百分比
}

// QuotaUsageSummary 配额使用汇总
type QuotaUsageSummary struct {
	ResourceType ResourceType `json:"resource_type"`
	HardLimit    int64        `json:"hard_limit"`
	SoftLimit    int64        `json:"soft_limit"`
	Used         int64        `json:"used"`
	Reserved     int64        `json:"reserved"`
	Available    int64        `json:"available"`
	UsagePercent float64      `json:"usage_percent"`
	Status       string       `json:"status"` // normal/warning/critical/exceeded
}

// OverQuotaAlert 配额超限告警
type OverQuotaAlert struct {
	ResourceType   ResourceType `json:"resource_type"`
	CurrentUsage   int64        `json:"current_usage"`
	QuotaLimit     int64        `json:"quota_limit"`
	ExceededAmount int64        `json:"exceeded_amount"`
	ExceededAt     time.Time    `json:"exceeded_at"`
	Severity       string       `json:"severity"` // warning/critical
}

// UsageTrendPoint 使用趋势数据点
type UsageTrendPoint struct {
	Timestamp string           `json:"timestamp"`
	Resources map[string]int64 `json:"resources"`
}

// ========== BillingManager 计费管理器 ==========

// BillingManager 计费管理器
type BillingManager struct {
	mu             sync.RWMutex
	quotas         map[string]*ResourceQuota          // quotaID -> quota
	projectQuotas  map[string]map[ResourceType]string // projectID -> resourceType -> quotaID
	usageRecords   map[string][]*QuotaUsage           // projectID -> usage records
	billingRecords map[string]*BillingRecord          // billingID -> record
	projectBills   map[string][]string                // projectID -> billingIDs
	manager        *Manager                           // 项目管理器引用
}

// NewBillingManager 创建计费管理器
func NewBillingManager(projectManager *Manager) *BillingManager {
	return &BillingManager{
		quotas:         make(map[string]*ResourceQuota),
		projectQuotas:  make(map[string]map[ResourceType]string),
		usageRecords:   make(map[string][]*QuotaUsage),
		billingRecords: make(map[string]*BillingRecord),
		projectBills:   make(map[string][]string),
		manager:        projectManager,
	}
}

// ========== 配额管理方法 ==========

// SetQuota 设置资源配额
func (bm *BillingManager) SetQuota(projectID string, resourceType ResourceType, hardLimit, softLimit int64, unitPrice float64, currency string) (*ResourceQuota, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// 验证项目存在
	if _, exists := bm.manager.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	// 验证配额值
	if hardLimit <= 0 || softLimit < 0 {
		return nil, ErrInvalidQuotaValue
	}
	if softLimit > hardLimit {
		softLimit = hardLimit
	}

	now := time.Now()

	// 检查是否已存在配额
	var quota *ResourceQuota
	if projectQuotas, ok := bm.projectQuotas[projectID]; ok {
		if quotaID, exists := projectQuotas[resourceType]; exists {
			quota = bm.quotas[quotaID]
		}
	}

	if quota != nil {
		// 更新现有配额
		quota.HardLimit = hardLimit
		quota.SoftLimit = softLimit
		quota.UnitPrice = unitPrice
		quota.Currency = currency
		quota.UpdatedAt = now
	} else {
		// 创建新配额
		quota = &ResourceQuota{
			ID:             uuid.New().String(),
			ProjectID:      projectID,
			ResourceType:   resourceType,
			Scope:          QuotaScopeProject,
			HardLimit:      hardLimit,
			SoftLimit:      softLimit,
			UnitPrice:      unitPrice,
			Currency:       currency,
			BillingCycle:   "monthly",
			AlertThreshold: 80,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		bm.quotas[quota.ID] = quota

		// 建立项目-配额映射
		if _, ok := bm.projectQuotas[projectID]; !ok {
			bm.projectQuotas[projectID] = make(map[ResourceType]string)
		}
		bm.projectQuotas[projectID][resourceType] = quota.ID
	}

	return quota, nil
}

// GetQuota 获取资源配额
func (bm *BillingManager) GetQuota(projectID string, resourceType ResourceType) (*ResourceQuota, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	projectQuotas, ok := bm.projectQuotas[projectID]
	if !ok {
		return nil, ErrQuotaNotFound
	}

	quotaID, exists := projectQuotas[resourceType]
	if !exists {
		return nil, ErrQuotaNotFound
	}

	quota, ok := bm.quotas[quotaID]
	if !ok {
		return nil, ErrQuotaNotFound
	}

	return quota, nil
}

// ListQuotas 列出项目所有配额
func (bm *BillingManager) ListQuotas(projectID string) []*ResourceQuota {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*ResourceQuota, 0)

	projectQuotas, ok := bm.projectQuotas[projectID]
	if !ok {
		return result
	}

	for _, quotaID := range projectQuotas {
		if quota, exists := bm.quotas[quotaID]; exists {
			result = append(result, quota)
		}
	}

	return result
}

// AllocateResource 分配资源
func (bm *BillingManager) AllocateResource(projectID string, resourceType ResourceType, amount int64, userID, description string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// 获取配额
	projectQuotas, ok := bm.projectQuotas[projectID]
	if !ok {
		return ErrQuotaNotFound
	}

	quotaID, exists := projectQuotas[resourceType]
	if !exists {
		return ErrQuotaNotFound
	}

	quota, ok := bm.quotas[quotaID]
	if !ok {
		return ErrQuotaNotFound
	}

	// 检查配额
	newUsed := quota.Used + amount
	if newUsed > quota.HardLimit {
		return ErrQuotaExceeded
	}

	// 更新使用量
	quota.Used = newUsed
	quota.UpdatedAt = time.Now()

	// 记录使用
	usage := &QuotaUsage{
		ID:           uuid.New().String(),
		ProjectID:    projectID,
		ResourceType: resourceType,
		Amount:       amount,
		Action:       "allocate",
		Timestamp:    time.Now(),
		UserID:       userID,
		Description:  description,
	}
	bm.usageRecords[projectID] = append(bm.usageRecords[projectID], usage)

	return nil
}

// ReleaseResource 释放资源
func (bm *BillingManager) ReleaseResource(projectID string, resourceType ResourceType, amount int64, userID, description string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	projectQuotas, ok := bm.projectQuotas[projectID]
	if !ok {
		return ErrQuotaNotFound
	}

	quotaID, exists := projectQuotas[resourceType]
	if !exists {
		return ErrQuotaNotFound
	}

	quota, ok := bm.quotas[quotaID]
	if !ok {
		return ErrQuotaNotFound
	}

	// 更新使用量
	quota.Used -= amount
	if quota.Used < 0 {
		quota.Used = 0
	}
	quota.UpdatedAt = time.Now()

	// 记录使用
	usage := &QuotaUsage{
		ID:           uuid.New().String(),
		ProjectID:    projectID,
		ResourceType: resourceType,
		Amount:       -amount,
		Action:       "release",
		Timestamp:    time.Now(),
		UserID:       userID,
		Description:  description,
	}
	bm.usageRecords[projectID] = append(bm.usageRecords[projectID], usage)

	return nil
}

// GetQuotaUsage 获取配额使用记录
func (bm *BillingManager) GetQuotaUsage(projectID string, limit int) []*QuotaUsage {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	records := bm.usageRecords[projectID]
	if records == nil {
		return []*QuotaUsage{}
	}

	// 按时间倒序
	result := make([]*QuotaUsage, len(records))
	copy(result, records)

	// 简单排序
	for i := 0; i < len(result)-1; i++ {
		for j := 0; j < len(result)-i-1; j++ {
			if result[j].Timestamp.Before(result[j+1].Timestamp) {
				result[j], result[j+1] = result[j+1], result[j]
			}
		}
	}

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result
}

// ========== 计费管理方法 ==========

// CreateBillingRecord 创建计费记录
func (bm *BillingManager) CreateBillingRecord(projectID string, periodStart, periodEnd time.Time, resourceCosts []ResourceCost, discount, taxRate float64) (*BillingRecord, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// 验证项目存在
	if _, exists := bm.manager.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()

	// 计算总成本
	var totalCost float64
	for _, rc := range resourceCosts {
		totalCost += rc.Subtotal
	}

	// 计算最终金额
	finalAmount := totalCost * (1 - discount) * (1 + taxRate)

	record := &BillingRecord{
		ID:            uuid.New().String(),
		ProjectID:     projectID,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		ResourceCosts: resourceCosts,
		TotalCost:     totalCost,
		Currency:      "CNY",
		Discount:      discount,
		TaxRate:       taxRate,
		FinalAmount:   finalAmount,
		Status:        "draft",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	bm.billingRecords[record.ID] = record
	bm.projectBills[projectID] = append(bm.projectBills[projectID], record.ID)

	return record, nil
}

// GetBillingRecord 获取计费记录
func (bm *BillingManager) GetBillingRecord(billingID string) (*BillingRecord, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	record, exists := bm.billingRecords[billingID]
	if !exists {
		return nil, ErrBillingNotFound
	}

	return record, nil
}

// ListBillingRecords 列出项目计费记录
func (bm *BillingManager) ListBillingRecords(projectID string, limit, offset int) []*BillingRecord {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	billIDs := bm.projectBills[projectID]
	result := make([]*BillingRecord, 0)

	// 收集记录
	for _, id := range billIDs {
		if record, exists := bm.billingRecords[id]; exists {
			result = append(result, record)
		}
	}

	// 按创建时间倒序排序
	for i := 0; i < len(result)-1; i++ {
		for j := 0; j < len(result)-i-1; j++ {
			if result[j].CreatedAt.Before(result[j+1].CreatedAt) {
				result[j], result[j+1] = result[j+1], result[j]
			}
		}
	}

	// 分页
	if offset > len(result) {
		offset = len(result)
	}
	end := offset + limit
	if limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end]
}

// UpdateBillingStatus 更新计费状态
func (bm *BillingManager) UpdateBillingStatus(billingID string, status string, paidAt *time.Time) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	record, exists := bm.billingRecords[billingID]
	if !exists {
		return ErrBillingNotFound
	}

	record.Status = status
	if paidAt != nil {
		record.PaidAt = paidAt
	}
	record.UpdatedAt = time.Now()

	return nil
}

// ========== 成本分析方法 ==========

// GetCostAnalysis 获取成本分析
func (bm *BillingManager) GetCostAnalysis(projectID string, periodStart, periodEnd time.Time) (*CostAnalysis, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// 验证项目存在
	project, exists := bm.manager.projects[projectID]
	if !exists {
		return nil, ErrProjectNotFound
	}

	analysis := &CostAnalysis{
		ProjectID:      projectID,
		ProjectName:    project.Name,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		ByResourceType: make(map[string]ResourceCostSummary),
		Currency:       "CNY",
	}

	// 收集计费记录
	billIDs := bm.projectBills[projectID]
	var totalCost float64
	resourceCosts := make(map[ResourceType][]ResourceCost)

	for _, id := range billIDs {
		record, exists := bm.billingRecords[id]
		if !exists {
			continue
		}

		// 检查时间范围
		if record.PeriodStart.Before(periodStart) || record.PeriodEnd.After(periodEnd) {
			// 部分重叠的情况，简化处理
			continue
		}

		for _, rc := range record.ResourceCosts {
			resourceCosts[rc.ResourceType] = append(resourceCosts[rc.ResourceType], rc)
			totalCost += rc.Subtotal
		}
	}

	analysis.TotalCost = totalCost

	// 按资源类型汇总
	for rt, costs := range resourceCosts {
		var typeTotal float64
		var typeQuantity int64
		for _, c := range costs {
			typeTotal += c.Subtotal
			typeQuantity += c.Quantity
		}

		days := periodEnd.Sub(periodStart).Hours() / 24
		avgDaily := typeTotal / days
		if days <= 0 {
			avgDaily = typeTotal
		}

		percentage := 0.0
		if totalCost > 0 {
			percentage = typeTotal / totalCost * 100
		}

		analysis.ByResourceType[string(rt)] = ResourceCostSummary{
			ResourceType:  rt,
			TotalCost:     typeTotal,
			TotalQuantity: typeQuantity,
			AvgDailyCost:  avgDaily,
			Percentage:    percentage,
			Trend:         "stable",
		}
	}

	// 生成每日成本数据
	analysis.DailyCost = bm.generateDailyCost(projectID, periodStart, periodEnd)

	// 生成优化建议
	analysis.Recommendations = bm.generateCostRecommendations(analysis)

	return analysis, nil
}

// generateDailyCost 生成每日成本数据
func (bm *BillingManager) generateDailyCost(projectID string, start, end time.Time) []DailyCost {
	dailyCosts := make([]DailyCost, 0)

	// 按天遍历
	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		dayEnd := dayStart.Add(24 * time.Hour)

		var dayCost float64

		// 收集当天的成本
		billIDs := bm.projectBills[projectID]
		for _, id := range billIDs {
			record, exists := bm.billingRecords[id]
			if !exists {
				continue
			}

			// 简化：如果账单周期覆盖这一天，则按天数分摊
			if (record.PeriodStart.Before(dayEnd) || record.PeriodStart.Equal(dayStart)) &&
				(record.PeriodEnd.After(dayStart) || record.PeriodEnd.Equal(dayEnd)) {
				days := record.PeriodEnd.Sub(record.PeriodStart).Hours() / 24
				if days > 0 {
					dayCost += record.TotalCost / days
				}
			}
		}

		dailyCosts = append(dailyCosts, DailyCost{
			Date: dayStart.Format("2006-01-02"),
			Cost: dayCost,
		})
	}

	return dailyCosts
}

// generateCostRecommendations 生成成本优化建议
func (bm *BillingManager) generateCostRecommendations(analysis *CostAnalysis) []CostRecommendation {
	recommendations := make([]CostRecommendation, 0)

	for rt, summary := range analysis.ByResourceType {
		// 检查高成本资源
		if summary.Percentage > 50 {
			recommendations = append(recommendations, CostRecommendation{
				Type:             "optimize",
				ResourceType:     rt,
				CurrentCost:      summary.TotalCost,
				PotentialSavings: summary.TotalCost * 0.2, // 假设可节省20%
				Description:      "资源成本占比较高，建议优化使用策略",
				Priority:         "high",
			})
		}

		// 检查趋势
		if summary.Trend == "up" {
			recommendations = append(recommendations, CostRecommendation{
				Type:             "monitor",
				ResourceType:     rt,
				CurrentCost:      summary.TotalCost,
				PotentialSavings: 0,
				Description:      "资源成本呈上升趋势，建议关注",
				Priority:         "medium",
			})
		}
	}

	return recommendations
}

// ========== 资源使用报告方法 ==========

// GetResourceUsageReport 获取资源使用报告
func (bm *BillingManager) GetResourceUsageReport(projectID string, periodStart, periodEnd time.Time) (*ResourceUsageReport, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// 验证项目存在
	project, exists := bm.manager.projects[projectID]
	if !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	report := &ResourceUsageReport{
		ProjectID:       projectID,
		ProjectName:     project.Name,
		ReportPeriod:    periodStart.Format("2006-01-02") + " - " + periodEnd.Format("2006-01-02"),
		GeneratedAt:     now,
		Resources:       make([]ResourceUsageDetail, 0),
		QuotaUsage:      make([]QuotaUsageSummary, 0),
		OverQuota:       make([]OverQuotaAlert, 0),
		PeakUsage:       make(map[string]int64),
		AvgUsage:        make(map[string]float64),
		UtilizationRate: make(map[string]float64),
		Currency:        "CNY",
	}

	// 收集配额信息
	projectQuotas := bm.projectQuotas[projectID]
	for rt, quotaID := range projectQuotas {
		quota, exists := bm.quotas[quotaID]
		if !exists {
			continue
		}

		// 资源使用详情
		available := quota.HardLimit - quota.Used - quota.Reserved
		utilRate := 0.0
		if quota.HardLimit > 0 {
			utilRate = float64(quota.Used) / float64(quota.HardLimit) * 100
		}

		detail := ResourceUsageDetail{
			ResourceType:    rt,
			Allocated:       quota.HardLimit,
			Used:            quota.Used,
			Reserved:        quota.Reserved,
			Available:       available,
			UtilizationRate: utilRate,
			QuotaLimit:      quota.HardLimit,
			QuotaUsed:       quota.Used,
			QuotaPercent:    utilRate,
		}
		report.Resources = append(report.Resources, detail)

		// 配额使用汇总
		status := "normal"
		if quota.Used >= quota.HardLimit {
			status = "exceeded"
		} else if quota.Used >= quota.SoftLimit {
			status = "warning"
		} else if quota.Used >= quota.HardLimit*80/100 {
			status = "critical"
		}

		quotaSummary := QuotaUsageSummary{
			ResourceType: rt,
			HardLimit:    quota.HardLimit,
			SoftLimit:    quota.SoftLimit,
			Used:         quota.Used,
			Reserved:     quota.Reserved,
			Available:    available,
			UsagePercent: utilRate,
			Status:       status,
		}
		report.QuotaUsage = append(report.QuotaUsage, quotaSummary)

		// 超限告警
		if quota.Used >= quota.SoftLimit {
			severity := "warning"
			if quota.Used >= quota.HardLimit {
				severity = "critical"
			}
			alert := OverQuotaAlert{
				ResourceType:   rt,
				CurrentUsage:   quota.Used,
				QuotaLimit:     quota.HardLimit,
				ExceededAmount: quota.Used - quota.SoftLimit,
				ExceededAt:     now,
				Severity:       severity,
			}
			report.OverQuota = append(report.OverQuota, alert)
		}

		// 统计数据
		report.PeakUsage[string(rt)] = quota.Used               // 简化：当前值作为峰值
		report.AvgUsage[string(rt)] = float64(quota.Used) * 0.7 // 简化：假设平均使用70%
		report.UtilizationRate[string(rt)] = utilRate
	}

	report.TotalResources = len(report.Resources)

	// 计算总成本
	billIDs := bm.projectBills[projectID]
	for _, id := range billIDs {
		record, exists := bm.billingRecords[id]
		if exists {
			report.TotalCost += record.TotalCost
		}
	}

	return report, nil
}

// DeleteQuota 删除资源配额
func (bm *BillingManager) DeleteQuota(projectID string, resourceType ResourceType) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	projectQuotas, ok := bm.projectQuotas[projectID]
	if !ok {
		return ErrQuotaNotFound
	}

	quotaID, exists := projectQuotas[resourceType]
	if !exists {
		return ErrQuotaNotFound
	}

	delete(bm.quotas, quotaID)
	delete(projectQuotas, resourceType)

	return nil
}
