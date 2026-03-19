// Package budget 提供预算管理 API
package budget

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 预算管理器 ==========

// Manager 预算管理器
type Manager struct {
	mu             sync.RWMutex
	budgets        map[string]*Budget
	usages         map[string][]*Usage
	alerts         map[string]*Alert
	alertHistory   map[string][]*Alert
	notifier       NotificationService
	costCalculator CostCalculator
}

// NotificationService 通知服务接口
type NotificationService interface {
	SendEmail(to []string, subject, body string) error
	SendWebhook(url string, payload interface{}) error
	SendAlert(alert *Alert) error
}

// CostCalculator 成本计算器接口
type CostCalculator interface {
	CalculateStorageCost(bytes uint64) float64
	CalculateBandwidthCost(bytes uint64) float64
	CalculateComputeCost(duration time.Duration) float64
}

// NewManager 创建预算管理器
func NewManager() *Manager {
	return &Manager{
		budgets:      make(map[string]*Budget),
		usages:       make(map[string][]*Usage),
		alerts:       make(map[string]*Alert),
		alertHistory: make(map[string][]*Alert),
	}
}

// SetNotifier 设置通知服务
func (m *Manager) SetNotifier(notifier NotificationService) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifier = notifier
}

// SetCostCalculator 设置成本计算器
func (m *Manager) SetCostCalculator(calc CostCalculator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.costCalculator = calc
}

// ========== 预算 CRUD 操作 ==========

// CreateBudget 创建预算
func (m *Manager) CreateBudget(input Input, createdBy string) (*Budget, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在同名预算
	for _, b := range m.budgets {
		if b.Name == input.Name && b.Scope == input.Scope && b.TargetID == input.TargetID {
			return nil, ErrBudgetExists
		}
	}

	now := time.Now()
	startDate := now
	if input.StartDate != nil {
		startDate = *input.StartDate
	}

	alertConfig := DefaultAlertConfig()
	if input.AlertConfig != nil {
		alertConfig = *input.AlertConfig
	}

	budget := &Budget{
		ID:           uuid.New().String(),
		Name:         input.Name,
		Description:  input.Description,
		Type:         input.Type,
		Period:       input.Period,
		Scope:        input.Scope,
		TargetID:     input.TargetID,
		TargetName:   input.TargetName,
		Amount:       input.Amount,
		UsedAmount:   0,
		Remaining:    input.Amount,
		UsagePercent: 0,
		StartDate:    startDate,
		EndDate:      input.EndDate,
		LastReset:    startDate,
		Status:       StatusActive,
		AutoReset:    input.AutoReset,
		Rollover:     input.Rollover,
		AlertConfig:  alertConfig,
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    createdBy,
		Tags:         input.Tags,
	}

	// 计算下次重置时间
	budget.NextReset = calculateNextReset(startDate, input.Period)

	m.budgets[budget.ID] = budget
	m.usages[budget.ID] = make([]*Usage, 0)

	return budget, nil
}

// UpdateBudget 更新预算
func (m *Manager) UpdateBudget(id string, input Input) (*Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	if input.Amount > 0 {
		budget.Amount = input.Amount
		budget.Remaining = budget.Amount - budget.UsedAmount
		budget.UsagePercent = calculatePercent(budget.UsedAmount, budget.Amount)
	}

	if input.Name != "" {
		budget.Name = input.Name
	}
	if input.Description != "" {
		budget.Description = input.Description
	}
	if input.AlertConfig != nil {
		budget.AlertConfig = *input.AlertConfig
	}
	if len(input.Tags) > 0 {
		budget.Tags = input.Tags
	}
	if input.EndDate != nil {
		budget.EndDate = input.EndDate
	}

	budget.AutoReset = input.AutoReset
	budget.Rollover = input.Rollover
	budget.UpdatedAt = time.Now()

	// 检查是否需要更新状态
	m.checkStatus(budget)

	return budget, nil
}

// DeleteBudget 删除预算
func (m *Manager) DeleteBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.budgets[id]; !ok {
		return ErrBudgetNotFound
	}

	delete(m.budgets, id)
	delete(m.usages, id)
	delete(m.alerts, id)

	return nil
}

// GetBudget 获取预算
func (m *Manager) GetBudget(id string) (*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	return budget, nil
}

// ListBudgets 列出预算
func (m *Manager) ListBudgets(query BudgetQuery) ([]*Budget, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Budget

	for _, budget := range m.budgets {
		if !matchBudgetQuery(budget, query) {
			continue
		}
		result = append(result, budget)
	}

	// 排序
	sortBudgets(result, query.SortBy, query.SortOrder)

	// 分页
	total := int64(len(result))
	start, end := getPaginationBounds(len(result), query.Page, query.PageSize)

	if start >= len(result) {
		return []*Budget{}, total, nil
	}

	return result[start:end], total, nil
}

// ========== 预算使用追踪 ==========

// RecordUsage 记录使用
func (m *Manager) RecordUsage(input UsageInput) (*Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[input.BudgetID]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	if budget.Status == StatusPaused || budget.Status == StatusArchived {
		return nil, fmt.Errorf("预算状态不允许记录使用: %s", budget.Status)
	}

	// 验证金额
	if input.Amount <= 0 {
		return nil, fmt.Errorf("使用金额必须大于0: %.2f", input.Amount)
	}

	now := time.Now()
	usage := &Usage{
		ID:           uuid.New().String(),
		BudgetID:     input.BudgetID,
		RecordedAt:   now,
		Amount:       input.Amount,
		SourceType:   input.SourceType,
		SourceID:     input.SourceID,
		Description:  input.Description,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		UnitCost:     input.UnitCost,
		Quantity:     input.Quantity,
		Metadata:     input.Metadata,
	}

	// 更新预算使用量
	budget.UsedAmount += input.Amount
	budget.Remaining = budget.Amount - budget.UsedAmount
	if budget.Remaining < 0 {
		budget.Remaining = 0
	}
	budget.UsagePercent = calculatePercent(budget.UsedAmount, budget.Amount)

	usage.Cumulative = budget.UsedAmount
	usage.Remaining = budget.Remaining

	// 检查是否超出预算
	m.checkStatus(budget)

	// 触发预警检查
	m.checkAndCreateAlert(budget)

	// 保存使用记录
	m.usages[input.BudgetID] = append(m.usages[input.BudgetID], usage)

	return usage, nil
}

// GetUsageHistory 获取使用历史
func (m *Manager) GetUsageHistory(budgetID string, query UsageQuery) ([]*Usage, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usages, ok := m.usages[budgetID]
	if !ok {
		return nil, 0, ErrBudgetNotFound
	}

	var result []*Usage

	for _, usage := range usages {
		if !matchUsageQuery(usage, query) {
			continue
		}
		result = append(result, usage)
	}

	total := int64(len(result))
	start, end := getPaginationBounds(len(result), query.Page, query.PageSize)

	if start >= len(result) {
		return []*Usage{}, total, nil
	}

	return result[start:end], total, nil
}

// GetUsageStats 获取使用统计
func (m *Manager) GetUsageStats(budgetID string, startTime, endTime time.Time) (*UsageStatsResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usages, ok := m.usages[budgetID]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	result := &UsageStatsResult{
		BudgetID:       budgetID,
		StartTime:      startTime,
		EndTime:        endTime,
		BySourceType:   make(map[string]float64),
		ByResourceType: make(map[string]float64),
		DailyUsage:     make([]DailyUsage, 0),
	}

	var totalAmount float64
	dailyMap := make(map[string]float64)

	for _, usage := range usages {
		if usage.RecordedAt.Before(startTime) || usage.RecordedAt.After(endTime) {
			continue
		}

		totalAmount += usage.Amount
		result.Count++

		// 按来源类型统计
		result.BySourceType[usage.SourceType] += usage.Amount

		// 按资源类型统计
		if usage.ResourceType != "" {
			result.ByResourceType[usage.ResourceType] += usage.Amount
		}

		// 按日期统计
		dateKey := usage.RecordedAt.Format("2006-01-02")
		dailyMap[dateKey] += usage.Amount
	}

	result.TotalAmount = totalAmount
	if result.Count > 0 {
		result.AvgAmount = totalAmount / float64(result.Count)
	}

	// 转换每日数据
	for date, amount := range dailyMap {
		t, _ := time.Parse("2006-01-02", date)
		result.DailyUsage = append(result.DailyUsage, DailyUsage{
			Date:   t,
			Amount: amount,
		})
	}

	return result, nil
}

// UsageStatsResult 使用统计结果
type UsageStatsResult struct {
	BudgetID       string             `json:"budget_id"`
	StartTime      time.Time          `json:"start_time"`
	EndTime        time.Time          `json:"end_time"`
	TotalAmount    float64            `json:"total_amount"`
	Count          int                `json:"count"`
	AvgAmount      float64            `json:"avg_amount"`
	BySourceType   map[string]float64 `json:"by_source_type"`
	ByResourceType map[string]float64 `json:"by_resource_type"`
	DailyUsage     []DailyUsage       `json:"daily_usage"`
}

// DailyUsage 每日使用量
type DailyUsage struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
}

// ========== 预算预警 ==========

// checkAndCreateAlert 检查并创建预警
func (m *Manager) checkAndCreateAlert(budget *Budget) {
	if !budget.AlertConfig.Enabled {
		return
	}

	// 检查每个阈值
	for i := len(budget.AlertConfig.Thresholds) - 1; i >= 0; i-- {
		threshold := budget.AlertConfig.Thresholds[i]

		if budget.UsagePercent >= threshold.Percent {
			// 检查是否已有相同级别的活跃预警
			existingAlert, exists := m.alerts[budget.ID]
			if exists && existingAlert.Level == threshold.Level {
				continue
			}

			// 创建新预警
			alert := &Alert{
				ID:              uuid.New().String(),
				BudgetID:        budget.ID,
				BudgetName:      budget.Name,
				Level:           threshold.Level,
				Threshold:       threshold.Percent,
				CurrentPercent:  budget.UsagePercent,
				CurrentSpend:    budget.UsedAmount,
				BudgetAmount:    budget.Amount,
				RemainingAmount: budget.Remaining,
				Message:         threshold.Message,
				Status:          StatusActive,
				TriggeredAt:     time.Now(),
				NotifySent:      false,
			}

			m.alerts[budget.ID] = alert
			m.alertHistory[budget.ID] = append(m.alertHistory[budget.ID], alert)

			// 发送通知（带重试机制）
			if m.notifier != nil {
				alertCopy := alert // 复制以避免并发问题
				go m.sendAlertWithRetry(alertCopy)
				alert.NotifySent = true
			}

			break
		}
	}
}

// checkStatus 检查预算状态
func (m *Manager) checkStatus(budget *Budget) {
	switch {
	case budget.UsagePercent >= 100:
		budget.Status = StatusExhausted
	case budget.UsagePercent >= 90:
		budget.Status = StatusExceeded
	default:
		if budget.Status == StatusExceeded || budget.Status == StatusExhausted {
			if budget.UsagePercent < 90 {
				budget.Status = StatusActive
			}
		}
	}
}

// AcknowledgeAlert 确认预警
func (m *Manager) AcknowledgeAlert(alertID string, acknowledgedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for budgetID, alert := range m.alerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Status = StatusAcknowledged
			alert.AcknowledgedAt = &now
			alert.AcknowledgedBy = acknowledgedBy
			m.alerts[budgetID] = alert
			return nil
		}
	}

	return ErrAlertNotFound
}

// ResolveAlert 解决预警
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for budgetID, alert := range m.alerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Status = StatusResolved
			alert.ResolvedAt = &now
			delete(m.alerts, budgetID)
			return nil
		}
	}

	return ErrAlertNotFound
}

// GetActiveAlerts 获取活跃预警
func (m *Manager) GetActiveAlerts(query AlertQuery) ([]*Alert, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Alert

	for _, alert := range m.alerts {
		if !matchAlertQuery(alert, query) {
			continue
		}
		result = append(result, alert)
	}

	total := int64(len(result))
	start, end := getPaginationBounds(len(result), query.Page, query.PageSize)

	if start >= len(result) {
		return []*Alert{}, total, nil
	}

	return result[start:end], total, nil
}

// GetAlertHistory 获取预警历史
func (m *Manager) GetAlertHistory(budgetID string, query AlertQuery) ([]*Alert, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, ok := m.alertHistory[budgetID]
	if !ok {
		return []*Alert{}, 0, nil
	}

	var result []*Alert

	for _, alert := range history {
		if !matchAlertQuery(alert, query) {
			continue
		}
		result = append(result, alert)
	}

	total := int64(len(result))
	start, end := getPaginationBounds(len(result), query.Page, query.PageSize)

	if start >= len(result) {
		return []*Alert{}, total, nil
	}

	return result[start:end], total, nil
}

// ========== 预算重置和结转 ==========

// ResetBudget 重置预算
func (m *Manager) ResetBudget(id string) (*Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	// 结转逻辑
	if budget.Rollover && budget.Remaining > 0 {
		budget.Amount += budget.Remaining
	}

	// 重置使用量
	budget.UsedAmount = 0
	budget.Remaining = budget.Amount
	budget.UsagePercent = 0
	budget.LastReset = time.Now()
	budget.NextReset = calculateNextReset(time.Now(), budget.Period)
	budget.Status = StatusActive

	// 清除活跃预警
	delete(m.alerts, id)

	return budget, nil
}

// CheckAndResetBudgets 检查并重置到期预算
func (m *Manager) CheckAndResetBudgets() ([]*Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var resetBudgets []*Budget

	for _, budget := range m.budgets {
		if !budget.AutoReset {
			continue
		}

		if budget.NextReset != nil && now.After(*budget.NextReset) {
			// 结转
			if budget.Rollover && budget.Remaining > 0 {
				budget.Amount += budget.Remaining
			}

			budget.UsedAmount = 0
			budget.Remaining = budget.Amount
			budget.UsagePercent = 0
			budget.LastReset = now
			budget.NextReset = calculateNextReset(now, budget.Period)
			budget.Status = StatusActive

			delete(m.alerts, budget.ID)
			resetBudgets = append(resetBudgets, budget)
		}
	}

	return resetBudgets, nil
}

// ========== 报告生成 ==========

// GenerateReport 生成预算报告
func (m *Manager) GenerateReport(request ReportRequest) (*Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()

	// 设置默认时间范围
	startTime := now.AddDate(0, -1, 0) // 默认最近一个月
	endTime := now
	if request.StartTime != nil {
		startTime = *request.StartTime
	}
	if request.EndTime != nil {
		endTime = *request.EndTime
	}

	report := &Report{
		ID:              uuid.New().String(),
		Name:            "预算报告",
		GeneratedAt:     now,
		Period:          ReportPeriod{StartTime: startTime, EndTime: endTime},
		BudgetDetails:   make([]BudgetDetail, 0),
		UsageTrend:      make([]UsageTrendPoint, 0),
		TopConsumers:    make([]TopConsumer, 0),
		Alerts:          make([]Alert, 0),
		Recommendations: make([]BudgetRecommendation, 0),
	}

	// 生成摘要和详情
	var totalAmount, totalUsed, totalRemaining float64
	var exceeded, nearLimit int
	var healthScores []int

	for _, budget := range m.budgets {
		// 过滤
		if len(request.BudgetIDs) > 0 && !containsString(request.BudgetIDs, budget.ID) {
			continue
		}
		if len(request.Types) > 0 && !containsType(request.Types, budget.Type) {
			continue
		}
		if len(request.Scopes) > 0 && !containsScope(request.Scopes, budget.Scope) {
			continue
		}

		totalAmount += budget.Amount
		totalUsed += budget.UsedAmount
		totalRemaining += budget.Remaining

		// 状态统计
		if budget.UsagePercent >= 100 {
			exceeded++
		} else if budget.UsagePercent >= 85 {
			nearLimit++
		}

		// 健康评分
		healthScores = append(healthScores, calculateHealthScore(budget))

		// 预算详情
		detail := BudgetDetail{
			BudgetID:      budget.ID,
			BudgetName:    budget.Name,
			Type:          budget.Type,
			Scope:         budget.Scope,
			TargetName:    budget.TargetName,
			Amount:        budget.Amount,
			UsedAmount:    budget.UsedAmount,
			Remaining:     budget.Remaining,
			UsagePercent:  budget.UsagePercent,
			Status:        budget.Status,
			Trend:         calculateTrend(m.usages[budget.ID]),
			DailyAvgUsage: calculateDailyAvgUsage(m.usages[budget.ID], startTime, endTime),
		}

		// 预测期末使用量
		daysRemaining := int(endTime.Sub(now).Hours() / 24)
		if daysRemaining > 0 {
			detail.DaysRemaining = daysRemaining
			detail.ProjectedUsage = budget.UsedAmount + (detail.DailyAvgUsage * float64(daysRemaining))
		}

		// 关联预警
		if alert, ok := m.alerts[budget.ID]; ok {
			detail.Alerts = []Alert{*alert}
		}

		report.BudgetDetails = append(report.BudgetDetails, detail)

		// 消费排行
		report.TopConsumers = append(report.TopConsumers, TopConsumer{
			BudgetID:   budget.ID,
			BudgetName: budget.Name,
			Scope:      budget.Scope,
			TargetName: budget.TargetName,
			UsedAmount: budget.UsedAmount,
			Percent:    budget.UsagePercent,
			Trend:      detail.Trend,
		})
	}

	// 排序消费排行
	sortTopConsumers(report.TopConsumers)

	// 计算摘要
	report.Summary = ReportSummary{
		TotalBudgets:      len(report.BudgetDetails),
		ActiveBudgets:     countActiveBudgets(report.BudgetDetails),
		TotalBudgetAmount: totalAmount,
		TotalUsedAmount:   totalUsed,
		TotalRemaining:    totalRemaining,
		AvgUsagePercent:   calculatePercent(totalUsed, totalAmount),
		ExceededBudgets:   exceeded,
		NearLimitBudgets:  nearLimit,
		ActiveAlerts:      len(m.alerts),
		HealthScore:       avgHealthScore(healthScores),
	}

	// 活跃预警
	for _, alert := range m.alerts {
		report.Alerts = append(report.Alerts, *alert)
	}

	// 生成建议
	report.Recommendations = generateRecommendations(report.BudgetDetails, report.Summary)

	return report, nil
}

// GetStats 获取统计
func (m *Manager) GetStats() *BudgetStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &BudgetStats{
		ByType:  make(map[Type]TypeStats),
		ByScope: make(map[Scope]TypeStats),
	}

	var totalAmount, totalUsed, totalRemaining float64
	var exceeded, nearLimit int

	for _, budget := range m.budgets {
		stats.TotalBudgets++
		if budget.Status == StatusActive {
			stats.ActiveBudgets++
		}

		totalAmount += budget.Amount
		totalUsed += budget.UsedAmount
		totalRemaining += budget.Remaining

		// 按类型统计
		typeStats := stats.ByType[budget.Type]
		typeStats.Count++
		typeStats.Amount += budget.Amount
		typeStats.Used += budget.UsedAmount
		typeStats.Remaining += budget.Remaining
		stats.ByType[budget.Type] = typeStats

		// 按范围统计
		scopeStats := stats.ByScope[budget.Scope]
		scopeStats.Count++
		scopeStats.Amount += budget.Amount
		scopeStats.Used += budget.UsedAmount
		scopeStats.Remaining += budget.Remaining
		stats.ByScope[budget.Scope] = scopeStats

		// 状态统计
		if budget.UsagePercent >= 100 {
			exceeded++
		} else if budget.UsagePercent >= 85 {
			nearLimit++
		}
	}

	stats.TotalAmount = totalAmount
	stats.TotalUsed = totalUsed
	stats.TotalRemaining = totalRemaining
	stats.ExceededCount = exceeded
	stats.NearLimitCount = nearLimit
	stats.ActiveAlertCount = len(m.alerts)
	stats.HealthScore = calculateOverallHealth(stats)

	return stats
}

// ========== 辅助函数 ==========

// calculateNextReset 计算下次重置时间
func calculateNextReset(lastReset time.Time, period Period) *time.Time {
	var next time.Time

	switch period {
	case PeriodDaily:
		next = lastReset.AddDate(0, 0, 1)
	case PeriodWeekly:
		next = lastReset.AddDate(0, 0, 7)
	case PeriodMonthly:
		next = lastReset.AddDate(0, 1, 0)
	case PeriodQuarter:
		next = lastReset.AddDate(0, 3, 0)
	case PeriodYearly:
		next = lastReset.AddDate(1, 0, 0)
	default:
		next = lastReset.AddDate(0, 1, 0)
	}

	return &next
}

// calculatePercent 计算百分比
func calculatePercent(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	return math.Min(100, (used/total)*100)
}

// matchBudgetQuery 匹配预算查询
func matchBudgetQuery(budget *Budget, query BudgetQuery) bool {
	if len(query.IDs) > 0 && !containsString(query.IDs, budget.ID) {
		return false
	}
	if len(query.Types) > 0 && !containsType(query.Types, budget.Type) {
		return false
	}
	if len(query.Scopes) > 0 && !containsScope(query.Scopes, budget.Scope) {
		return false
	}
	if len(query.Statuses) > 0 && !containsStatus(query.Statuses, budget.Status) {
		return false
	}
	if len(query.TargetIDs) > 0 && !containsString(query.TargetIDs, budget.TargetID) {
		return false
	}
	if query.MinAmount != nil && budget.Amount < *query.MinAmount {
		return false
	}
	if query.MaxAmount != nil && budget.Amount > *query.MaxAmount {
		return false
	}
	if query.MinUsage != nil && budget.UsagePercent < *query.MinUsage {
		return false
	}
	if query.MaxUsage != nil && budget.UsagePercent > *query.MaxUsage {
		return false
	}
	return true
}

// matchUsageQuery 匹配使用记录查询
func matchUsageQuery(usage *Usage, query UsageQuery) bool {
	if len(query.SourceTypes) > 0 && !containsString(query.SourceTypes, usage.SourceType) {
		return false
	}
	if query.StartTime != nil && usage.RecordedAt.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && usage.RecordedAt.After(*query.EndTime) {
		return false
	}
	if query.MinAmount != nil && usage.Amount < *query.MinAmount {
		return false
	}
	if query.MaxAmount != nil && usage.Amount > *query.MaxAmount {
		return false
	}
	return true
}

// matchAlertQuery 匹配预警查询
func matchAlertQuery(alert *Alert, query AlertQuery) bool {
	if len(query.BudgetIDs) > 0 && !containsString(query.BudgetIDs, alert.BudgetID) {
		return false
	}
	if len(query.Levels) > 0 && !containsLevel(query.Levels, alert.Level) {
		return false
	}
	if len(query.Statuses) > 0 && !containsStatus(query.Statuses, alert.Status) {
		return false
	}
	return true
}

// getPaginationBounds 获取分页边界
func getPaginationBounds(total, page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}

	return start, end
}

// containsString 检查字符串是否在切片中
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// containsType 检查预算类型是否在切片中
func containsType(slice []Type, t Type) bool {
	for _, item := range slice {
		if item == t {
			return true
		}
	}
	return false
}

// containsScope 检查预算范围是否在切片中
func containsScope(slice []Scope, s Scope) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// containsStatus 检查预算状态是否在切片中
func containsStatus(slice []Status, s Status) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// containsLevel 检查预警级别是否在切片中
func containsLevel(slice []Level, l Level) bool {
	for _, item := range slice {
		if item == l {
			return true
		}
	}
	return false
}

// containsStatus 检查预警状态是否在切片中
func containsStatus(slice []Status, s Status) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// sortBudgets 排序预算列表
func sortBudgets(budgets []*Budget, sortBy, sortOrder string) {
	if len(budgets) == 0 {
		return
	}

	// 简单排序实现
	for i := 0; i < len(budgets)-1; i++ {
		for j := i + 1; j < len(budgets); j++ {
			var shouldSwap bool

			switch sortBy {
			case "amount":
				shouldSwap = budgets[i].Amount < budgets[j].Amount
			case "used_amount":
				shouldSwap = budgets[i].UsedAmount < budgets[j].UsedAmount
			case "usage_percent":
				shouldSwap = budgets[i].UsagePercent < budgets[j].UsagePercent
			case "created_at":
				shouldSwap = budgets[i].CreatedAt.Before(budgets[j].CreatedAt)
			default:
				shouldSwap = budgets[i].Name < budgets[j].Name
			}

			if sortOrder == "desc" {
				shouldSwap = !shouldSwap
			}

			if shouldSwap {
				budgets[i], budgets[j] = budgets[j], budgets[i]
			}
		}
	}
}

// sortTopConsumers 排序消费排行
func sortTopConsumers(consumers []TopConsumer) {
	for i := 0; i < len(consumers)-1; i++ {
		for j := i + 1; j < len(consumers); j++ {
			if consumers[i].UsedAmount < consumers[j].UsedAmount {
				consumers[i], consumers[j] = consumers[j], consumers[i]
			}
		}
	}

	// 设置排名
	for i := range consumers {
		consumers[i].Rank = i + 1
	}
}

// calculateHealthScore 计算健康评分
func calculateHealthScore(budget *Budget) int {
	if budget.UsagePercent >= 100 {
		return 0
	} else if budget.UsagePercent >= 95 {
		return 20
	} else if budget.UsagePercent >= 85 {
		return 50
	} else if budget.UsagePercent >= 70 {
		return 70
	} else if budget.UsagePercent >= 50 {
		return 85
	}
	return 100
}

// calculateTrend 计算趋势
func calculateTrend(usages []*Usage) string {
	if len(usages) < 2 {
		return "stable"
	}

	recent := usages[len(usages)-1:]
	var recentSum float64
	for _, u := range recent {
		recentSum += u.Amount
	}

	older := usages[:len(usages)-1]
	if len(older) == 0 {
		return "stable"
	}

	var olderSum float64
	for _, u := range older {
		olderSum += u.Amount
	}

	if recentSum > olderSum*1.1 {
		return "up"
	} else if recentSum < olderSum*0.9 {
		return "down"
	}
	return "stable"
}

// calculateDailyAvgUsage 计算日均使用量
func calculateDailyAvgUsage(usages []*Usage, start, end time.Time) float64 {
	if len(usages) == 0 {
		return 0
	}

	var total float64
	for _, u := range usages {
		if u.RecordedAt.After(start) && u.RecordedAt.Before(end) {
			total += u.Amount
		}
	}

	days := end.Sub(start).Hours() / 24
	if days <= 0 {
		days = 1
	}

	return total / days
}

// countActiveBudgets 计算活跃预算数
func countActiveBudgets(details []BudgetDetail) int {
	count := 0
	for _, d := range details {
		if d.Status == StatusActive {
			count++
		}
	}
	return count
}

// avgHealthScore 计算平均健康评分
func avgHealthScore(scores []int) int {
	if len(scores) == 0 {
		return 100
	}

	sum := 0
	for _, s := range scores {
		sum += s
	}
	return sum / len(scores)
}

// calculateOverallHealth 计算整体健康度
func calculateOverallHealth(stats *BudgetStats) int {
	if stats.TotalBudgets == 0 {
		return 100
	}

	score := 100
	score -= stats.ExceededCount * 20
	score -= stats.NearLimitCount * 10
	score -= stats.ActiveAlertCount * 5

	if score < 0 {
		score = 0
	}
	return score
}

// generateRecommendations 生成建议
func generateRecommendations(details []BudgetDetail, summary ReportSummary) []BudgetRecommendation {
	var recommendations []BudgetRecommendation

	for _, d := range details {
		// 超支建议
		if d.UsagePercent >= 100 {
			recommendations = append(recommendations, BudgetRecommendation{
				Type:        "increase",
				Priority:    "critical",
				BudgetID:    d.BudgetID,
				BudgetName:  d.BudgetName,
				Title:       "预算已超支",
				Description: fmt.Sprintf("预算 %s 已使用 %.1f%%，建议增加预算或优化使用", d.BudgetName, d.UsagePercent),
				Current:     d.Amount,
				Suggested:   d.Amount * 1.2,
				Action:      "increase_budget",
			})
		} else if d.UsagePercent >= 85 {
			// 接近上限建议
			recommendations = append(recommendations, BudgetRecommendation{
				Type:        "alert",
				Priority:    "high",
				BudgetID:    d.BudgetID,
				BudgetName:  d.BudgetName,
				Title:       "预算接近上限",
				Description: fmt.Sprintf("预算 %s 已使用 %.1f%%，请关注使用情况", d.BudgetName, d.UsagePercent),
				Current:     d.Amount,
				Suggested:   d.Amount,
				Action:      "monitor_usage",
			})
		} else if d.UsagePercent < 30 && d.Trend == "stable" {
			// 预算充足建议
			recommendations = append(recommendations, BudgetRecommendation{
				Type:        "decrease",
				Priority:    "low",
				BudgetID:    d.BudgetID,
				BudgetName:  d.BudgetName,
				Title:       "预算充足",
				Description: fmt.Sprintf("预算 %s 仅使用 %.1f%%，可考虑调减预算", d.BudgetName, d.UsagePercent),
				Current:     d.Amount,
				Suggested:   d.Amount * 0.8,
				Savings:     d.Amount * 0.2,
				Action:      "reduce_budget",
			})
		}
	}

	return recommendations
}

// ========== 通知重试机制 ==========

const (
	// 通知重试配置
	maxNotifyRetries     = 3
	baseRetryDelay       = 1 * time.Second
	maxRetryDelay        = 30 * time.Second
	retryDelayMultiplier = 2.0
)

// sendAlertWithRetry 发送预警通知（带重试机制）
func (m *Manager) sendAlertWithRetry(alert *Alert) {
	if m.notifier == nil {
		return
	}

	var lastErr error
	for attempt := 0; attempt < maxNotifyRetries; attempt++ {
		if err := m.notifier.SendAlert(alert); err != nil {
			lastErr = err
			// 计算指数退避延迟
			delay := time.Duration(float64(baseRetryDelay) * math.Pow(retryDelayMultiplier, float64(attempt)))
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}

			log.Printf("⚠️ 预算通知发送失败 (预算: %s, 尝试: %d/%d, 延迟: %v): %v",
				alert.BudgetName, attempt+1, maxNotifyRetries, delay, err)

			if attempt < maxNotifyRetries-1 {
				time.Sleep(delay)
			}
			continue
		}

		// 发送成功
		if attempt > 0 {
			log.Printf("✅ 预算通知发送成功 (预算: %s, 重试次数: %d)", alert.BudgetName, attempt)
		}
		return
	}

	// 所有重试都失败
	log.Printf("❌ 预算通知发送最终失败 (预算: %s, 级别: %s): %v",
		alert.BudgetName, alert.Level, lastErr)
}
