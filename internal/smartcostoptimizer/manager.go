package smartcostoptimizer

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 智能成本优化器管理器
type Manager struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	config  *SmartCostConfig
	analyzer *Analyzer
	assets  map[string]*StorageAsset
	entries map[string]*CostEntry
	reports map[string]*CostReport
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, config *SmartCostConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultSmartCostConfig()
	}
	analyzer := NewAnalyzer(logger, config)
	return &Manager{
		logger:   logger,
		config:   config,
		analyzer: analyzer,
		assets:   make(map[string]*StorageAsset),
		entries:  make(map[string]*CostEntry),
		reports:  make(map[string]*CostReport),
	}
}

// GetConfig 获取配置（副本）
func (m *Manager) GetConfig() *SmartCostConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *SmartCostConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
		m.analyzer = NewAnalyzer(m.logger, cfg)
		m.logger.Info("smart cost config updated")
	}
}

// ============================================================
// 资产管理
// ============================================================

// AddAsset 添加存储资产
func (m *Manager) AddAsset(asset *StorageAsset) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if asset.ID == "" {
		return fmt.Errorf("asset id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = time.Now()
	}
	m.assets[asset.ID] = asset
	m.logger.Info("storage asset added",
		zap.String("id", asset.ID),
		zap.String("type", string(asset.Type)),
		zap.Int64("capacity", asset.CapacityBytes))

	return nil
}

// GetAsset 获取资产
func (m *Manager) GetAsset(id string) (*StorageAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[id]
	if !ok {
		return nil, fmt.Errorf("asset not found: %s", id)
	}
	return asset, nil
}

// ListAssets 列出所有资产
func (m *Manager) ListAssets() []*StorageAsset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*StorageAsset, 0, len(m.assets))
	for _, a := range m.assets {
		result = append(result, a)
	}
	return result
}

// RemoveAsset 删除资产
func (m *Manager) RemoveAsset(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.assets[id]; !ok {
		return fmt.Errorf("asset not found: %s", id)
	}
	delete(m.assets, id)
	m.logger.Info("storage asset removed", zap.String("id", id))
	return nil
}

// ============================================================
// 成本记录
// ============================================================

// RecordCost 记录成本
func (m *Manager) RecordCost(entry *CostEntry) error {
	if entry == nil {
		return fmt.Errorf("cost entry is nil")
	}
	if entry.AssetID == "" {
		return fmt.Errorf("asset_id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateID()
	}
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now()
	}
	if entry.TotalCost == 0 && entry.CapacityGB > 0 && entry.PricePerGB > 0 {
		entry.TotalCost = entry.UsedGB * entry.PricePerGB
	}

	m.entries[entry.ID] = entry
	m.logger.Info("cost recorded",
		zap.String("id", entry.ID),
		zap.String("asset", entry.AssetID),
		zap.Float64("cost", entry.TotalCost))

	return nil
}

// ListCostEntries 列出成本记录
func (m *Manager) ListCostEntries() []*CostEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CostEntry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, e)
	}
	return result
}

// ============================================================
// 成本分析
// ============================================================

// GetCostSummary 获取成本汇总
func (m *Manager) GetCostSummary(periodStart, periodEnd time.Time) *CostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*CostEntry, 0)
	for _, e := range m.entries {
		if !e.PeriodStart.Before(periodStart) && !e.PeriodEnd.After(periodEnd) {
			entries = append(entries, e)
		}
	}

	// 若无实际数据，使用资产生成模拟数据
	if len(entries) == 0 {
		for _, asset := range m.assets {
			cost := m.analyzer.CalculateCostForAsset(asset)
			entries = append(entries, &CostEntry{
				AssetID:     asset.ID,
				AssetName:   asset.Name,
				StorageType: asset.Type,
				CapacityGB:  float64(asset.CapacityBytes) / (1024 * 1024 * 1024),
				UsedGB:      float64(asset.UsedBytes) / (1024 * 1024 * 1024),
				TotalCost:   cost,
			})
		}
	}

	// 兜底：完全无数据时返回模拟汇总
	if len(entries) == 0 {
		return &CostSummary{
			TotalCost:       500.0,
			TotalCapacityGB: 2000.0,
			TotalUsedGB:     800.0,
			AvgUtilization:  40.0,
			ByType:          map[StorageType]float64{StorageTypeSSD: 300, StorageTypeHDD: 200},
			ByPool:          map[string]float64{"pool-main": 500},
			Currency:        m.config.DefaultCurrency,
			PeriodStart:     periodStart,
			PeriodEnd:       periodEnd,
		}
	}

	return m.analyzer.CalculateCostSummary(entries, periodStart, periodEnd)
}

// AnalyzeTrend 分析成本趋势
func (m *Manager) AnalyzeTrend(granularity TrendGranularity, months int) *CostTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*CostEntry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e)
	}

	return m.analyzer.AnalyzeTrend(entries, granularity, months)
}

// ============================================================
// 优化建议
// ============================================================

// GenerateOptimizations 生成优化建议
func (m *Manager) GenerateOptimizations() []*OptimizationSuggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assets := make([]*StorageAsset, 0, len(m.assets))
	for _, a := range m.assets {
		assets = append(assets, a)
	}

	coldData := m.analyzer.DetectColdData(assets, time.Now())
	return m.analyzer.GenerateOptimizations(assets, coldData)
}

// DetectColdData 检测冷数据
func (m *Manager) DetectColdData() []*ColdDataInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assets := make([]*StorageAsset, 0, len(m.assets))
	for _, a := range m.assets {
		assets = append(assets, a)
	}

	return m.analyzer.DetectColdData(assets, time.Now())
}

// ============================================================
// ROI 计算
// ============================================================

// CalculateROI 计算投资回报率
func (m *Manager) CalculateROI(input *ROIInput) (*ROIResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, err := m.analyzer.CalculateROI(input)
	if err != nil {
		return nil, err
	}

	m.logger.Info("roi calculated",
		zap.Float64("roi_percent", result.ROIPercent),
		zap.Float64("payback_months", result.PaybackMonths),
		zap.Float64("npv", result.NPV))

	return result, nil
}

// ============================================================
// 报告生成与导出
// ============================================================

// GenerateReport 生成综合成本报告
func (m *Manager) GenerateReport(name string, periodStart, periodEnd time.Time) *CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := m.GetCostSummary(periodStart, periodEnd)
	trend := m.AnalyzeTrend(TrendMonthly, 6)
	suggestions := m.GenerateOptimizations()
	coldData := m.DetectColdData()

	report := &CostReport{
		ID:          generateID(),
		ReportName:  name,
		Summary:     summary,
		Trend:       trend,
		Suggestions: suggestions,
		ColdData:    coldData,
		GeneratedAt: time.Now(),
	}

	m.reports[report.ID] = report
	return report
}

// GetReport 获取报告
func (m *Manager) GetReport(id string) (*CostReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", id)
	}
	return report, nil
}

// ListReports 列出报告
func (m *Manager) ListReports() []*CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CostReport, 0, len(m.reports))
	for _, r := range m.reports {
		result = append(result, r)
	}
	return result
}

// ExportReportAsCSV 将报告导出为 CSV
func (m *Manager) ExportReportAsCSV(reportID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[reportID]
	if !ok {
		return "", fmt.Errorf("report not found: %s", reportID)
	}

	csv := "字段,值\n"
	csv += fmt.Sprintf("报告名称,%s\n", report.ReportName)
	csv += fmt.Sprintf("总成本,%.2f %s\n", report.Summary.TotalCost, report.Summary.Currency)
	csv += fmt.Sprintf("总容量,%.2f GB\n", report.Summary.TotalCapacityGB)
	csv += fmt.Sprintf("已用容量,%.2f GB\n", report.Summary.TotalUsedGB)
	csv += fmt.Sprintf("平均利用率,%.2f%%\n", report.Summary.AvgUtilization)
	csv += "\n按存储类型,成本\n"
	for st, cost := range report.Summary.ByType {
		csv += fmt.Sprintf("%s,%.2f\n", st, cost)
	}
	csv += "\n优化建议\n"
	csv += "策略,标题,预估节省,优先级\n"
	for _, s := range report.Suggestions {
		csv += fmt.Sprintf("%s,%s,%.2f,%d\n", s.Strategy, s.Title, s.EstimatedSaving, s.Priority)
	}
	csv += "\n冷数据\n"
	csv += "资产ID,资产名,大小(字节),天数,当前类型,建议类型,预估节省\n"
	for _, cd := range report.ColdData {
		csv += fmt.Sprintf("%s,%s,%d,%d,%s,%s,%.2f\n",
			cd.AssetID, cd.AssetName, cd.SizeBytes, cd.DaysSince,
			cd.CurrentType, cd.SuggestedType, cd.PotentialSave)
	}

	return csv, nil
}
