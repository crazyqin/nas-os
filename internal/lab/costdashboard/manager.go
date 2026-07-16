// Package costdashboard 提供云存储成本分析核心业务逻辑
package costdashboard

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 成本分析管理器.
type Manager struct {
	providers     map[string]*CloudProvider
	metrics       map[string]*StorageMetrics
	reports       map[string]*CostReport
	alerts        map[string]*CostAlert
	optimizations []*CostOptimization
	mu            sync.RWMutex
}

// NewManager 创建成本分析管理器.
func NewManager() *Manager {
	return &Manager{
		providers:     make(map[string]*CloudProvider),
		metrics:       make(map[string]*StorageMetrics),
		reports:       make(map[string]*CostReport),
		alerts:        make(map[string]*CostAlert),
		optimizations: make([]*CostOptimization, 0),
	}
}

// ========== Provider 管理 ==========

// AddProvider 添加云提供商.
func (m *Manager) AddProvider(req AddProviderRequest) *CloudProvider {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	provider := &CloudProvider{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Type:      req.Type,
		APIKey:    req.APIKey,
		Region:    req.Region,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.providers[provider.ID] = provider
	return provider
}

// RemoveProvider 删除云提供商.
func (m *Manager) RemoveProvider(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[id]; !ok {
		return fmt.Errorf("provider %q not found", id)
	}

	// 清理关联数据
	delete(m.providers, id)
	delete(m.metrics, id)

	// 清理关联告警
	for alertID, alert := range m.alerts {
		if alert.ProviderID == id {
			delete(m.alerts, alertID)
		}
	}

	// 清理关联优化建议
	filtered := make([]*CostOptimization, 0)
	for _, opt := range m.optimizations {
		if opt.ProviderID != id {
			filtered = append(filtered, opt)
		}
	}
	m.optimizations = filtered

	return nil
}

// ListProviders 列出所有云提供商.
func (m *Manager) ListProviders() []*CloudProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]*CloudProvider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].CreatedAt.After(providers[j].CreatedAt)
	})

	return providers
}

// UpdateProvider 更新云提供商.
func (m *Manager) UpdateProvider(id string, req UpdateProviderRequest) (*CloudProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, ok := m.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", id)
	}

	if req.Name != nil {
		provider.Name = *req.Name
	}
	if req.APIKey != nil {
		provider.APIKey = *req.APIKey
	}
	if req.Region != nil {
		provider.Region = *req.Region
	}
	if req.Status != nil {
		provider.Status = *req.Status
	}

	provider.UpdatedAt = time.Now()
	return provider, nil
}

// SyncMetrics 同步指定提供商的存储指标（模拟）.
func (m *Manager) SyncMetrics(providerID string) (*StorageMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, ok := m.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", providerID)
	}

	// 模拟同步指标 - 实际项目中会调用各云平台 API
	metrics := &StorageMetrics{
		ProviderID:   providerID,
		ProviderName: provider.Name,
		UsedBytes:    int64(500+time.Now().Unix()%1000) * 1024 * 1024 * 1024, // 模拟 500-1500 GB
		TotalBytes:   int64(2000) * 1024 * 1024 * 1024,                       // 2 TB
		CostPerGB:    getCostPerGB(provider.Type),
		TransferCost: 12.50,
		SyncedAt:     time.Now(),
	}
	metrics.MonthlyCost = float64(metrics.UsedBytes) / (1024 * 1024 * 1024) * metrics.CostPerGB

	m.metrics[providerID] = metrics
	return metrics, nil
}

// getCostPerGB 根据提供商类型返回每 GB 成本（模拟价格）.
func getCostPerGB(ptype CloudProviderType) float64 {
	switch ptype {
	case ProviderAliyun:
		return 0.12
	case ProviderTencent:
		return 0.11
	case ProviderAWS:
		return 0.023
	case ProviderGDrive:
		return 0.0 // 免费额度
	case ProviderOneDrive:
		return 0.0 // 免费额度
	default:
		return 0.10
	}
}

// ========== 指标分析 ==========

// GetMetrics 获取所有提供商的存储指标.
func (m *Manager) GetMetrics() []*StorageMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]*StorageMetrics, 0, len(m.metrics))
	for _, ms := range m.metrics {
		metrics = append(metrics, ms)
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].MonthlyCost > metrics[j].MonthlyCost
	})

	return metrics
}

// CompareProviders 对比多个提供商的指标.
func (m *Manager) CompareProviders(providerIDs []string) ([]*StorageMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*StorageMetrics, 0, len(providerIDs))
	for _, id := range providerIDs {
		ms, ok := m.metrics[id]
		if !ok {
			return nil, fmt.Errorf("metrics for provider %q not found", id)
		}
		result = append(result, ms)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].MonthlyCost > result[j].MonthlyCost
	})

	return result, nil
}

// GetUsageTrend 获取使用趋势（模拟历史数据）.
func (m *Manager) GetUsageTrend(providerID string, period string) ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.providers[providerID]; !ok {
		return nil, fmt.Errorf("provider %q not found", providerID)
	}

	// 模拟趋势数据
	points := 7
	if period == "monthly" {
		points = 12
	}

	trend := make([]map[string]interface{}, points)
	for i := 0; i < points; i++ {
		t := time.Now().AddDate(0, 0, -(points - 1 - i))
		usage := 400 + float64(i)*15 + math.Sin(float64(i))*50
		cost := usage * 0.12
		trend[i] = map[string]interface{}{
			"date":    t.Format("2006-01-02"),
			"used_gb": usage,
			"cost":    cost,
		}
	}

	return trend, nil
}

// ForecastCost 预测未来成本（简单线性预测）.
func (m *Manager) ForecastCost(providerID string, months int) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ms, ok := m.metrics[providerID]
	if !ok {
		return nil, fmt.Errorf("metrics for provider %q not found", providerID)
	}

	// 简单线性增长预测
	growthRate := 0.05 // 5% 月增长率
	currentCost := ms.MonthlyCost
	forecasts := make([]map[string]interface{}, months)

	for i := 0; i < months; i++ {
		futureCost := currentCost * math.Pow(1+growthRate, float64(i+1))
		futureDate := time.Now().AddDate(0, i+1, 0)
		forecasts[i] = map[string]interface{}{
			"month": futureDate.Format("2006-01"),
			"cost":  math.Round(futureCost*100) / 100,
		}
	}

	return map[string]interface{}{
		"provider_id":  providerID,
		"current_cost": currentCost,
		"growth_rate":  growthRate,
		"forecasts":    forecasts,
	}, nil
}

// ========== 报告生成 ==========

// GenerateReport 生成成本报告.
func (m *Manager) GenerateReport(period ReportPeriod) *CostReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	providers := make([]StorageMetrics, 0, len(m.metrics))
	totalCost := 0.0
	for _, ms := range m.metrics {
		providers = append(providers, *ms)
		totalCost += ms.MonthlyCost
	}

	// 计算趋势（与上次报告对比）
	trend := "stable"
	if len(m.reports) > 0 {
		// 取最新报告
		var latest *CostReport
		for _, r := range m.reports {
			if latest == nil || r.GeneratedAt.After(latest.GeneratedAt) {
				latest = r
			}
		}
		if latest != nil {
			if totalCost > latest.TotalCost*1.05 {
				trend = "up"
			} else if totalCost < latest.TotalCost*0.95 {
				trend = "down"
			}
		}
	}

	report := &CostReport{
		ID:          uuid.New().String(),
		Period:      period,
		Providers:   providers,
		TotalCost:   math.Round(totalCost*100) / 100,
		Trend:       trend,
		GeneratedAt: time.Now(),
	}

	m.reports[report.ID] = report
	return report
}

// GetReport 获取报告.
func (m *Manager) GetReport(id string) (*CostReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %q not found", id)
	}
	return report, nil
}

// ListReports 列出所有报告.
func (m *Manager) ListReports() []*CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*CostReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt.After(reports[j].GeneratedAt)
	})

	return reports
}

// ========== 成本告警 ==========

// SetAlert 设置成本告警.
func (m *Manager) SetAlert(req SetAlertRequest) (*CostAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[req.ProviderID]; !ok {
		return nil, fmt.Errorf("provider %q not found", req.ProviderID)
	}

	alert := &CostAlert{
		ID:         uuid.New().String(),
		ProviderID: req.ProviderID,
		Threshold:  req.Threshold,
		Severity:   req.Severity,
		Acked:      false,
	}

	// 检查是否已触发
	if ms, ok := m.metrics[req.ProviderID]; ok {
		alert.CurrentCost = ms.MonthlyCost
		if ms.MonthlyCost >= req.Threshold {
			alert.TriggeredAt = time.Now()
		}
	}

	m.alerts[alert.ID] = alert
	return alert, nil
}

// CheckAlerts 检查所有告警状态.
func (m *Manager) CheckAlerts() []*CostAlert {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if ms, ok := m.metrics[alert.ProviderID]; ok {
			alert.CurrentCost = ms.MonthlyCost
			if ms.MonthlyCost >= alert.Threshold && alert.TriggeredAt.IsZero() {
				alert.TriggeredAt = time.Now()
			}
		}
	}

	result := make([]*CostAlert, 0, len(m.alerts))
	for _, a := range m.alerts {
		result = append(result, a)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TriggeredAt.After(result[j].TriggeredAt)
	})

	return result
}

// GetAlerts 获取所有告警.
func (m *Manager) GetAlerts() []*CostAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*CostAlert, 0, len(m.alerts))
	for _, a := range m.alerts {
		alerts = append(alerts, a)
	}

	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].TriggeredAt.After(alerts[j].TriggeredAt)
	})

	return alerts
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %q not found", alertID)
	}

	alert.Acked = true
	return nil
}

// ========== 优化建议 ==========

// AnalyzeOptimization 分析并生成优化建议.
func (m *Manager) AnalyzeOptimization() []*CostOptimization {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空旧建议
	m.optimizations = make([]*CostOptimization, 0)

	for providerID, ms := range m.metrics {
		usagePercent := float64(ms.UsedBytes) / float64(ms.TotalBytes) * 100

		// 存储空间使用率过高
		if usagePercent > 80 {
			m.optimizations = append(m.optimizations, &CostOptimization{
				ID:                uuid.New().String(),
				Type:              OptOversized,
				Description:       fmt.Sprintf("存储使用率 %.1f%%，建议扩容或清理", usagePercent),
				PotentialSaving:   0,
				RecommendedAction: "扩容存储容量或清理不必要的文件",
				ProviderID:        providerID,
				GeneratedAt:       time.Now(),
			})
		}

		// 成本过高建议
		if ms.MonthlyCost > 100 {
			saving := ms.MonthlyCost * 0.15 // 预计可节省 15%
			m.optimizations = append(m.optimizations, &CostOptimization{
				ID:                uuid.New().String(),
				Type:              OptInfrequent,
				Description:       fmt.Sprintf("月成本 $%.2f，存在优化空间", ms.MonthlyCost),
				PotentialSaving:   math.Round(saving*100) / 100,
				RecommendedAction: "将不常访问的文件迁移到低频存储层",
				ProviderID:        providerID,
				GeneratedAt:       time.Now(),
			})
		}

		// 传输成本优化
		if ms.TransferCost > 20 {
			saving := ms.TransferCost * 0.3
			m.optimizations = append(m.optimizations, &CostOptimization{
				ID:                uuid.New().String(),
				Type:              OptDuplicate,
				Description:       fmt.Sprintf("传输成本 $%.2f 偏高", ms.TransferCost),
				PotentialSaving:   math.Round(saving*100) / 100,
				RecommendedAction: "启用数据压缩和去重以减少传输量",
				ProviderID:        providerID,
				GeneratedAt:       time.Now(),
			})
		}
	}

	// 按潜在节省金额排序
	sort.Slice(m.optimizations, func(i, j int) bool {
		return m.optimizations[i].PotentialSaving > m.optimizations[j].PotentialSaving
	})

	return m.optimizations
}

// GetRecommendations 获取优化建议.
func (m *Manager) GetRecommendations() []*CostOptimization {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CostOptimization, len(m.optimizations))
	copy(result, m.optimizations)
	return result
}
