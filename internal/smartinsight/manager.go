// Package smartinsight 提供智能洞察核心管理逻辑
package smartinsight

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Manager 智能洞察管理器
type Manager struct {
	mu              sync.RWMutex
	insights        map[string]*Insight
	recommendations map[string]*Recommendation
	anomalies       map[string]*Anomaly
	reports         map[string]*InsightReport
	lastReport      *InsightReport
}

// NewManager 创建智能洞察管理器
func NewManager() *Manager {
	m := &Manager{
		insights:        make(map[string]*Insight),
		recommendations: make(map[string]*Recommendation),
		anomalies:       make(map[string]*Anomaly),
		reports:         make(map[string]*InsightReport),
	}

	// 初始化模拟数据
	m.seedData()
	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d-%04d", time.Now().UnixNano(), rand.Intn(10000))
}

// seedData 初始化模拟数据
func (m *Manager) seedData() {
	now := time.Now()

	// 模拟洞察数据
	insights := []*Insight{
		{ID: generateID(), Category: CategoryStorage, Severity: SeverityWarning, Title: "存储空间使用率偏高", Summary: "当前存储使用率达到 78%，建议尽快清理或扩容", Score: 78, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: generateID(), Category: CategoryCPU, Severity: SeverityInfo, Title: "CPU 使用率正常", Summary: "过去 24 小时 CPU 平均使用率 35%，处于健康范围", Score: 35, CreatedAt: now.Add(-1 * time.Hour)},
		{ID: generateID(), Category: CategoryMemory, Severity: SeverityWarning, Title: "内存使用率持续上升", Summary: "内存使用率从 60% 上升到 72%，呈持续上升趋势", Score: 72, CreatedAt: now.Add(-30 * time.Minute)},
		{ID: generateID(), Category: CategoryNetwork, Severity: SeverityInfo, Title: "网络带宽使用正常", Summary: "当前网络带宽利用率 45%，无拥塞风险", Score: 45, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: generateID(), Category: CategorySecurity, Severity: SeverityCritical, Title: "检测到异常登录尝试", Summary: "过去 1 小时内检测到来自未知 IP 的 15 次失败登录", Score: 90, CreatedAt: now.Add(-15 * time.Minute)},
	}

	for _, ins := range insights {
		m.insights[ins.ID] = ins
	}

	// 模拟推荐数据
	recommendations := []*Recommendation{
		{ID: generateID(), Type: RecStorageOptimize, Title: "启用数据去重", Description: "检测到大量重复文件，启用去重可节省约 15% 存储空间", Impact: "high", Effort: "low", Score: 92, CreatedAt: now.Add(-1 * time.Hour)},
		{ID: generateID(), Type: RecDeduplication, Title: "压缩冷数据", Description: "超过 6 个月未访问的数据占总量 30%，可压缩节省空间", Impact: "medium", Effort: "low", Score: 85, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: generateID(), Type: RecTiering, Title: "配置自动分层存储", Description: "热数据和冷数据混合存储，建议配置自动分层策略", Impact: "high", Effort: "medium", Score: 78, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: generateID(), Type: RecCacheTuning, Title: "调整缓存策略", Description: "当前缓存命中率 65%，调整缓存大小可提升至 85%", Impact: "medium", Effort: "low", Score: 70, CreatedAt: now.Add(-4 * time.Hour)},
		{ID: generateID(), Type: RecCleanup, Title: "清理临时文件", Description: "检测到 12GB 临时文件占用空间，建议定期清理", Impact: "low", Effort: "low", Score: 60, CreatedAt: now.Add(-5 * time.Hour)},
	}

	for _, rec := range recommendations {
		m.recommendations[rec.ID] = rec
	}

	// 模拟异常数据
	anomalies := []*Anomaly{
		{ID: generateID(), Type: AnomalyFileAccess, Severity: "high", Description: "检测到凌晨 3 点大量文件被批量读取", Resource: "/data/documents", Value: 500, Threshold: 50, DetectedAt: now.Add(-1 * time.Hour)},
		{ID: generateID(), Type: AnomalyResourceSpike, Severity: "medium", Description: "CPU 使用率突然飙升到 95%", Resource: "cpu", Value: 95, Threshold: 80, DetectedAt: now.Add(-2 * time.Hour)},
		{ID: generateID(), Type: AnomalyUnusualIO, Severity: "low", Description: "磁盘 I/O 异常，写入量超出正常范围 3 倍", Resource: "/dev/sda", Value: 300, Threshold: 100, DetectedAt: now.Add(-3 * time.Hour)},
	}

	for _, a := range anomalies {
		m.anomalies[a.ID] = a
	}
}

// AnalyzeUsage 分析系统使用趋势
func (m *Manager) AnalyzeUsage(category string, period string) []*UsageTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trends := make([]*UsageTrend, 0)

	// 模拟趋势数据
	categories := []struct {
		name   string
		unit   string
		base   float64
		latest float64
	}{
		{"storage", "%", 68, 78},
		{"cpu", "%", 30, 35},
		{"memory", "%", 58, 72},
		{"network", "Mbps", 450, 520},
		{"disk_iops", "IOPS", 1200, 1500},
	}

	for _, cat := range categories {
		if category != "" && category != cat.name {
			continue
		}

		changePct := ((cat.latest - cat.base) / cat.base) * 100
		direction := "stable"
		if changePct > 5 {
			direction = "up"
		} else if changePct < -5 {
			direction = "down"
		}

		// 生成趋势数据点
		now := time.Now()
		points := make([]TrendPoint, 0)
		for i := 23; i >= 0; i-- {
			val := cat.base + (cat.latest-cat.base)*float64(23-i)/23 + rand.Float64()*5 - 2.5
			points = append(points, TrendPoint{
				Timestamp: now.Add(-time.Duration(i) * time.Hour),
				Value:     math.Round(val*100) / 100,
			})
		}

		trend := &UsageTrend{
			ID:        generateID(),
			Category:  cat.name,
			Current:   cat.latest,
			Previous:  cat.base,
			Unit:      cat.unit,
			Direction: direction,
			ChangePct: math.Round(changePct*100) / 100,
			TrendData: points,
		}
		trends = append(trends, trend)
	}

	return trends
}

// GetRecommendations 获取智能推荐
func (m *Manager) GetRecommendations(category string) []*Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Recommendation, 0)
	for _, rec := range m.recommendations {
		if category == "" || string(rec.Type) == category {
			result = append(result, rec)
		}
	}
	return result
}

// DetectAnomalies 检测异常行为
func (m *Manager) DetectAnomalies(anomalyType string) []*Anomaly {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Anomaly, 0)
	for _, a := range m.anomalies {
		if anomalyType == "" || string(a.Type) == anomalyType {
			result = append(result, a)
		}
	}
	return result
}

// GenerateReport 生成系统洞察报告
func (m *Manager) GenerateReport() *InsightReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 收集所有洞察
	insights := make([]*Insight, 0, len(m.insights))
	for _, ins := range m.insights {
		insights = append(insights, ins)
	}

	// 收集所有推荐
	recommendations := make([]*Recommendation, 0, len(m.recommendations))
	for _, rec := range m.recommendations {
		recommendations = append(recommendations, rec)
	}

	// 收集所有异常
	anomalies := make([]*Anomaly, 0, len(m.anomalies))
	for _, a := range m.anomalies {
		anomalies = append(anomalies, a)
	}

	// 计算趋势（加锁内直接调用辅助方法）
	trends := m.analyzeUsageInternal("", "")

	// 计算成本分析
	cost := m.analyzeCostInternal()

	// 计算健康分数
	healthScore := m.calculateHealthScore(insights, anomalies)

	report := &InsightReport{
		ID:              generateID(),
		Title:           fmt.Sprintf("系统洞察报告 - %s", now.Format("2006-01-02 15:04")),
		Summary:         fmt.Sprintf("系统整体健康评分 %.1f/100。发现 %d 条洞察、%d 条推荐、%d 个异常事件。", healthScore, len(insights), len(recommendations), len(anomalies)),
		HealthScore:     healthScore,
		Insights:        insights,
		Recommendations: recommendations,
		Trends:          trends,
		Anomalies:       anomalies,
		Cost:            cost,
		GeneratedAt:     now,
	}

	m.reports[report.ID] = report
	m.lastReport = report

	return report
}

// analyzeUsageInternal 内部分析使用趋势（不加锁）
func (m *Manager) analyzeUsageInternal(category string, _ string) []*UsageTrend {
	trends := make([]*UsageTrend, 0)

	categories := []struct {
		name   string
		unit   string
		base   float64
		latest float64
	}{
		{"storage", "%", 68, 78},
		{"cpu", "%", 30, 35},
		{"memory", "%", 58, 72},
		{"network", "Mbps", 450, 520},
	}

	for _, cat := range categories {
		if category != "" && category != cat.name {
			continue
		}

		changePct := ((cat.latest - cat.base) / cat.base) * 100
		direction := "stable"
		if changePct > 5 {
			direction = "up"
		} else if changePct < -5 {
			direction = "down"
		}

		trends = append(trends, &UsageTrend{
			ID:        generateID(),
			Category:  cat.name,
			Current:   cat.latest,
			Previous:  cat.base,
			Unit:      cat.unit,
			Direction: direction,
			ChangePct: math.Round(changePct*100) / 100,
		})
	}

	return trends
}

// analyzeCostInternal 内部成本分析（不加锁）
func (m *Manager) analyzeCostInternal() *CostAnalysis {
	storageUsed := 780.0
	storageTotal := 1000.0
	costPerGB := 0.5
	currentCost := storageUsed * costPerGB
	projectedCost := currentCost * 1.15 // 预计增长 15%

	return &CostAnalysis{
		ID:               generateID(),
		StorageUsedGB:    storageUsed,
		StorageTotalGB:   storageTotal,
		CostPerGB:        costPerGB,
		CurrentCost:      currentCost,
		ProjectedCost:    math.Round(projectedCost*100) / 100,
		SavingsPotential: math.Round(currentCost*0.18*100) / 100, // 潜在节省 18%
		Breakdown: []CostItem{
			{Category: "活跃数据", Cost: 230, Percent: 59},
			{Category: "备份数据", Cost: 100, Percent: 25.6},
			{Category: "临时文件", Cost: 40, Percent: 10.3},
			{Category: "系统日志", Cost: 20, Percent: 5.1},
		},
		CreatedAt: time.Now(),
	}
}

// AnalyzeCost 成本效益分析
func (m *Manager) AnalyzeCost() *CostAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.analyzeCostInternal()
}

// calculateHealthScore 计算系统健康分数
func (m *Manager) calculateHealthScore(insights []*Insight, anomalies []*Anomaly) float64 {
	baseScore := 85.0

	// 根据洞察严重程度调整
	for _, ins := range insights {
		switch ins.Severity {
		case SeverityCritical:
			baseScore -= 15
		case SeverityWarning:
			baseScore -= 5
		}
	}

	// 根据异常调整
	for _, a := range anomalies {
		switch a.Severity {
		case "high":
			baseScore -= 10
		case "medium":
			baseScore -= 5
		case "low":
			baseScore -= 2
		}
	}

	if baseScore < 0 {
		baseScore = 0
	}
	if baseScore > 100 {
		baseScore = 100
	}

	return math.Round(baseScore*10) / 10
}

// GetLatestReport 获取最新报告
func (m *Manager) GetLatestReport() *InsightReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastReport
}

// GetAllReports 获取所有历史报告
func (m *Manager) GetAllReports() []*InsightReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*InsightReport, 0, len(m.reports))
	for _, r := range m.reports {
		result = append(result, r)
	}
	return result
}

// GetStats 获取系统统计概览
func (m *Manager) GetStats() *StatsOverview {
	m.mu.RLock()
	defer m.mu.RUnlock()

	healthScore := 0.0
	lastReportTime := ""
	if m.lastReport != nil {
		healthScore = m.lastReport.HealthScore
		lastReportTime = m.lastReport.GeneratedAt.Format(time.RFC3339)
	}

	return &StatsOverview{
		TotalInsights:        len(m.insights),
		TotalRecommendations: len(m.recommendations),
		TotalAnomalies:       len(m.anomalies),
		HealthScore:          healthScore,
		LastReportTime:       lastReportTime,
	}
}
