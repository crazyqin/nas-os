// Package cost - 多节点成本聚合模块
// 对标TrueNAS企业报告 + 群晖成本管理
package cost

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ========== 多节点核心类型定义 ==========

// NodeInfo 节点信息
type NodeInfo struct {
	// 节点ID
	ID string `json:"id"`

	// 节点名称
	Name string `json:"name"`

	// 节点地址
	Address string `json:"address"`

	// 节点角色 (master/worker/standalone)
	Role string `json:"role"`

	// 节点状态 (online/offline/degraded)
	Status string `json:"status"`

	// 地区/数据中心
	Region string `json:"region"`

	// 机架位置
	Rack string `json:"rack"`

	// 最后心跳时间
	LastHeartbeat time.Time `json:"last_heartbeat"`

	// 标签
	Labels map[string]string `json:"labels"`
}

// NodeCostStats 节点成本统计
type NodeCostStats struct {
	// 节点信息
	Node NodeInfo `json:"node"`

	// 成本汇总
	Summary CostSummary `json:"summary"`

	// 资源信息
	Resources []ResourceCostDetail `json:"resources"`

	// 使用率统计
	UsageStats NodeUsageStats `json:"usage_stats"`

	// 成本趋势（最近N个数据点）
	RecentTrend []TrendData `json:"recent_trend"`

	// 采集时间
	CollectedAt time.Time `json:"collected_at"`
}

// ResourceCostDetail 资源成本明细
type ResourceCostDetail struct {
	// 资源名称
	Name string `json:"name"`

	// 资源类型 (volume/pool/dataset/share)
	Type string `json:"type"`

	// 总容量（字节）
	TotalCapacityBytes uint64 `json:"total_capacity_bytes"`

	// 已用容量（字节）
	UsedCapacityBytes uint64 `json:"used_capacity_bytes"`

	// 月度成本
	MonthlyCost float64 `json:"monthly_cost"`

	// 成本构成
	CostBreakdown CostBreakdown `json:"cost_breakdown"`

	// 效率评分
	EfficiencyScore float64 `json:"efficiency_score"`

	// 节省建议
	Suggestions []string `json:"suggestions"`
}

// CostBreakdown 成本构成
type CostBreakdown struct {
	StorageCost     float64 `json:"storage_cost"`
	ElectricityCost float64 `json:"electricity_cost"`
	NetworkCost     float64 `json:"network_cost"`
	OpsCost         float64 `json:"ops_cost"`
	DepreciationCost float64 `json:"depreciation_cost"`
}

// NodeUsageStats 节点使用率统计
type NodeUsageStats struct {
	// CPU使用率（%）
	CPUUsagePercent float64 `json:"cpu_usage_percent"`

	// 内存使用率（%）
	MemoryUsagePercent float64 `json:"memory_usage_percent"`

	// 存储使用率（%）
	StorageUsagePercent float64 `json:"storage_usage_percent"`

	// 网络吞吐（MB/s）
	NetworkThroughputMB float64 `json:"network_throughput_mb"`

	// IOPS
	IOPS uint64 `json:"iops"`

	// 延迟（ms）
	LatencyMs float64 `json:"latency_ms"`
}

// ClusterCostReport 集群成本报告
type ClusterCostReport struct {
	// 报告ID
	ID string `json:"id"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 时间范围
	TimeRange TimeRange `json:"time_range"`

	// 集群汇总
	ClusterSummary ClusterCostSummary `json:"cluster_summary"`

	// 节点明细
	NodeDetails []NodeCostStats `json:"node_details"`

	// 按类型汇总
	CostByType map[CostType]float64 `json:"cost_by_type"`

	// 按区域汇总
	CostByRegion map[string]float64 `json:"cost_by_region"`

	// 优化建议
	OptimizationSuggestions []ClusterOptimizationSuggestion `json:"optimization_suggestions"`

	// 成本趋势
	CostTrend ClusterCostTrend `json:"cost_trend"`

	// 预测数据
	Forecast ClusterCostForecast `json:"forecast"`
}

// ClusterCostSummary 集群成本汇总
type ClusterCostSummary struct {
	// 节点数量
	TotalNodes int `json:"total_nodes"`

	// 在线节点数
	OnlineNodes int `json:"online_nodes"`

	// 离线节点数
	OfflineNodes int `json:"offline_nodes"`

	// 总容量（字节）
	TotalCapacityBytes uint64 `json:"total_capacity_bytes"`

	// 已用容量（字节）
	UsedCapacityBytes uint64 `json:"used_capacity_bytes"`

	// 总成本（月）
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// 总成本（年）
	TotalCostYearly float64 `json:"total_cost_yearly"`

	// 平均单位成本（元/GB/月）
	AvgCostPerGB float64 `json:"avg_cost_per_gb"`

	// 集群效率评分（0-100）
	EfficiencyScore float64 `json:"efficiency_score"`

	// 潜在节省（月）
	PotentialSavings float64 `json:"potential_savings"`

	// 预算使用情况
	BudgetStatus BudgetStatus `json:"budget_status"`
}

// BudgetStatus 预算状态
type BudgetStatus struct {
	// 月度预算
	MonthlyBudget float64 `json:"monthly_budget"`

	// 已使用
	Used float64 `json:"used"`

	// 剩余
	Remaining float64 `json:"remaining"`

	// 使用比例（%）
	UsagePercent float64 `json:"usage_percent"`

	// 状态 (normal/warning/critical)
	Status string `json:"status"`
}

// ClusterOptimizationSuggestion 集群优化建议
type ClusterOptimizationSuggestion struct {
	// 建议ID
	ID string `json:"id"`

	// 建议类型 (scale_down/scale_up/migrate/archive/consolidate)
	Type string `json:"type"`

	// 优先级 (1-5，1最高)
	Priority int `json:"priority"`

	// 影响节点
	AffectedNodes []string `json:"affected_nodes"`

	// 描述
	Description string `json:"description"`

	// 潜在节省（元/月）
	PotentialSaving float64 `json:"potential_saving"`

	// 实施复杂度 (low/medium/high)
	Complexity string `json:"complexity"`

	// 预计实施时间（小时）
	EstimatedHours int `json:"estimated_hours"`

	// ROI评分（0-100）
	ROIScore float64 `json:"roi_score"`
}

// ClusterCostTrend 集群成本趋势
type ClusterCostTrend struct {
	// 趋势方向 (increasing/decreasing/stable)
	Direction string `json:"direction"`

	// 变化率（%/月）
	ChangeRate float64 `json:"change_rate"`

	// 历史数据点
	DataPoints []TrendData `json:"data_points"`

	// 峰值成本
	PeakCost float64 `json:"peak_cost"`

	// 峰值时间
	PeakTime time.Time `json:"peak_time"`

	// 平均成本
	AvgCost float64 `json:"avg_cost"`
}

// ClusterCostForecast 集群成本预测
type ClusterCostForecast struct {
	// 下月预测
	NextMonthCost float64 `json:"next_month_cost"`

	// 下季度预测
	NextQuarterCost float64 `json:"next_quarter_cost"`

	// 下年度预测
	NextYearCost float64 `json:"next_year_cost"`

	// 置信度（%）
	Confidence float64 `json:"confidence"`

	// 预测模型
	Model string `json:"model"`

	// 预测数据点
	ForecastPoints []TrendData `json:"forecast_points"`

	// 风险提示
	RiskAlerts []string `json:"risk_alerts"`
}

// ========== 多节点聚合服务 ==========

// MultiNodeAggregator 多节点成本聚合器
type MultiNodeAggregator struct {
	mu           sync.RWMutex
	nodes        map[string]*NodeCostStats
	history      map[string][]TrendData // nodeID -> history
	config       MultiNodeConfig
	dashboard    *DashboardService
}

// MultiNodeConfig 多节点配置
type MultiNodeConfig struct {
	// 数据保留天数
	RetentionDays int `json:"retention_days"`

	// 采集间隔（秒）
	CollectIntervalSec int `json:"collect_interval_sec"`

	// 心跳超时（秒）
	HeartbeatTimeoutSec int `json:"heartbeat_timeout_sec"`

	// 月度预算
	MonthlyBudget float64 `json:"monthly_budget"`

	// 低使用率阈值（%）
	LowUsageThreshold float64 `json:"low_usage_threshold"`

	// 高使用率阈值（%）
	HighUsageThreshold float64 `json:"high_usage_threshold"`

	// 警告阈值（%）
	WarningThreshold float64 `json:"warning_threshold"`

	// 严重阈值（%）
	CriticalThreshold float64 `json:"critical_threshold"`
}

// DefaultMultiNodeConfig 默认配置
func DefaultMultiNodeConfig() MultiNodeConfig {
	return MultiNodeConfig{
		RetentionDays:       90,
		CollectIntervalSec:  300,
		HeartbeatTimeoutSec: 60,
		MonthlyBudget:       10000.0,
		LowUsageThreshold:   30.0,
		HighUsageThreshold:  80.0,
		WarningThreshold:    70.0,
		CriticalThreshold:   90.0,
	}
}

// NewMultiNodeAggregator 创建多节点聚合器
func NewMultiNodeAggregator(config MultiNodeConfig, dashboard *DashboardService) *MultiNodeAggregator {
	return &MultiNodeAggregator{
		nodes:     make(map[string]*NodeCostStats),
		history:   make(map[string][]TrendData),
		config:    config,
		dashboard: dashboard,
	}
}

// RegisterNode 注册节点
func (a *MultiNodeAggregator) RegisterNode(node NodeInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	stats, exists := a.nodes[node.ID]
	if !exists {
		stats = &NodeCostStats{
			Node:         node,
			CollectedAt:  now,
			RecentTrend:  make([]TrendData, 0),
		}
		a.nodes[node.ID] = stats
	} else {
		// 更新节点信息
		stats.Node = node
	}
}

// UnregisterNode 注销节点
func (a *MultiNodeAggregator) UnregisterNode(nodeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.nodes, nodeID)
	delete(a.history, nodeID)
}

// UpdateNodeStats 更新节点统计
func (a *MultiNodeAggregator) UpdateNodeStats(nodeID string, summary CostSummary, resources []ResourceCostDetail, usage NodeUsageStats) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	stats, exists := a.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not registered", nodeID)
	}

	stats.Summary = summary
	stats.Resources = resources
	stats.UsageStats = usage
	stats.CollectedAt = time.Now()

	// 记录趋势数据
	trendPoint := TrendData{
		Timestamp:        stats.CollectedAt,
		TotalCost:        summary.TotalCostMonthly,
		StorageCost:      summary.CostByType[CostTypeStorage],
		ElectricityCost:  summary.CostByType[CostTypeElectricity],
		NetworkCost:      summary.CostByType[CostTypeNetwork],
		OperationsCost:   summary.CostByType[CostTypeOperations],
		DepreciationCost: summary.CostByType[CostTypeDepreciation],
		UsagePercent:     usage.StorageUsagePercent,
		CostPerGB:        summary.AvgCostPerGB,
	}

	// 添加到节点趋势
	stats.RecentTrend = append(stats.RecentTrend, trendPoint)
	if len(stats.RecentTrend) > 100 { // 保留最近100个点
		stats.RecentTrend = stats.RecentTrend[1:]
	}

	// 添加到历史记录
	a.history[nodeID] = append(a.history[nodeID], trendPoint)
	a.cleanupHistory(nodeID)

	return nil
}

// GetNodeStats 获取节点统计
func (a *MultiNodeAggregator) GetNodeStats(nodeID string) (*NodeCostStats, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats, exists := a.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	// 返回副本
	copy := *stats
	return &copy, nil
}

// GetAllNodes 获取所有节点
func (a *MultiNodeAggregator) GetAllNodes() []NodeInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	nodes := make([]NodeInfo, 0, len(a.nodes))
	for _, stats := range a.nodes {
		nodes = append(nodes, stats.Node)
	}
	return nodes
}

// GenerateClusterReport 生成集群成本报告
func (a *MultiNodeAggregator) GenerateClusterReport(ctx context.Context, timeRange TimeRange) (*ClusterCostReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := time.Now()
	report := &ClusterCostReport{
		ID:          fmt.Sprintf("cluster_report_%d", now.Unix()),
		GeneratedAt: now,
		TimeRange:   timeRange,
		CostByType:  make(map[CostType]float64),
		CostByRegion: make(map[string]float64),
		NodeDetails: make([]NodeCostStats, 0),
		OptimizationSuggestions: make([]ClusterOptimizationSuggestion, 0),
	}

	// 汇总各节点数据
	var totalCapacity, usedCapacity uint64
	var totalCost float64
	onlineNodes := 0
	offlineNodes := 0

	for nodeID, stats := range a.nodes {
		// 检查节点状态
		if time.Since(stats.Node.LastHeartbeat).Seconds() > float64(a.config.HeartbeatTimeoutSec) {
			stats.Node.Status = "offline"
			offlineNodes++
		} else {
			stats.Node.Status = "online"
			onlineNodes++
		}

		// 累加成本
		totalCost += stats.Summary.TotalCostMonthly

		// 累加容量
		for _, res := range stats.Resources {
			totalCapacity += res.TotalCapacityBytes
			usedCapacity += res.UsedCapacityBytes
		}

		// 按类型汇总
		for costType, amount := range stats.Summary.CostByType {
			report.CostByType[costType] += amount
		}

		// 按区域汇总
		region := stats.Node.Region
		if region == "" {
			region = "default"
		}
		report.CostByRegion[region] += stats.Summary.TotalCostMonthly

		// 添加节点明细
		report.NodeDetails = append(report.NodeDetails, *stats)

		// 收集历史趋势
		for _, point := range a.history[nodeID] {
			report.CostTrend.DataPoints = append(report.CostTrend.DataPoints, point)
		}
	}

	// 填充汇总信息
	report.ClusterSummary = ClusterCostSummary{
		TotalNodes:        len(a.nodes),
		OnlineNodes:       onlineNodes,
		OfflineNodes:      offlineNodes,
		TotalCapacityBytes: totalCapacity,
		UsedCapacityBytes:  usedCapacity,
		TotalCostMonthly:  round(totalCost, 2),
		TotalCostYearly:   round(totalCost*12, 2),
	}

	// 计算平均单位成本
	usedGB := float64(usedCapacity) / (1024 * 1024 * 1024)
	if usedGB > 0 {
		report.ClusterSummary.AvgCostPerGB = round(totalCost/usedGB, 4)
	}

	// 计算效率评分
	report.ClusterSummary.EfficiencyScore = a.calculateClusterEfficiency(report)

	// 计算潜在节省
	report.ClusterSummary.PotentialSavings = a.calculatePotentialSavings(report)

	// 计算预算状态
	report.ClusterSummary.BudgetStatus = BudgetStatus{
		MonthlyBudget: a.config.MonthlyBudget,
		Used:         totalCost,
		Remaining:    a.config.MonthlyBudget - totalCost,
		UsagePercent: round(totalCost/a.config.MonthlyBudget*100, 2),
		Status:       a.getBudgetStatus(totalCost),
	}

	// 分析成本趋势
	report.CostTrend = a.analyzeClusterTrend(report.CostTrend.DataPoints)

	// 生成预测
	report.Forecast = a.generateClusterForecast(report.CostTrend.DataPoints, report.ClusterSummary)

	// 生成优化建议
	report.OptimizationSuggestions = a.generateOptimizationSuggestions(report)

	return report, nil
}

// GetClusterSummary 获取集群汇总（快速查询）
func (a *MultiNodeAggregator) GetClusterSummary() ClusterCostSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var totalCapacity, usedCapacity uint64
	var totalCost float64
	onlineNodes := 0
	offlineNodes := 0

	for _, stats := range a.nodes {
		if time.Since(stats.Node.LastHeartbeat).Seconds() > float64(a.config.HeartbeatTimeoutSec) {
			offlineNodes++
		} else {
			onlineNodes++
		}

		totalCost += stats.Summary.TotalCostMonthly

		for _, res := range stats.Resources {
			totalCapacity += res.TotalCapacityBytes
			usedCapacity += res.UsedCapacityBytes
		}
	}

	usedGB := float64(usedCapacity) / (1024 * 1024 * 1024)
	avgCostPerGB := 0.0
	if usedGB > 0 {
		avgCostPerGB = totalCost / usedGB
	}

	return ClusterCostSummary{
		TotalNodes:         len(a.nodes),
		OnlineNodes:        onlineNodes,
		OfflineNodes:       offlineNodes,
		TotalCapacityBytes: totalCapacity,
		UsedCapacityBytes:  usedCapacity,
		TotalCostMonthly:   round(totalCost, 2),
		TotalCostYearly:    round(totalCost*12, 2),
		AvgCostPerGB:       round(avgCostPerGB, 4),
		BudgetStatus: BudgetStatus{
			MonthlyBudget: a.config.MonthlyBudget,
			Used:         totalCost,
			Remaining:    a.config.MonthlyBudget - totalCost,
			UsagePercent: round(totalCost/a.config.MonthlyBudget*100, 2),
			Status:       a.getBudgetStatus(totalCost),
		},
	}
}

// GetNodesByRegion 按区域获取节点
func (a *MultiNodeAggregator) GetNodesByRegion(region string) []NodeCostStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]NodeCostStats, 0)
	for _, stats := range a.nodes {
		if stats.Node.Region == region {
			result = append(result, *stats)
		}
	}
	return result
}

// GetTopCostNodes 获取成本最高的节点
func (a *MultiNodeAggregator) GetTopCostNodes(limit int) []NodeCostStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 收集所有节点
	nodes := make([]NodeCostStats, 0, len(a.nodes))
	for _, stats := range a.nodes {
		nodes = append(nodes, *stats)
	}

	// 按成本排序（简单冒泡）
	for i := 0; i < len(nodes) && i < limit; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Summary.TotalCostMonthly > nodes[i].Summary.TotalCostMonthly {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	if limit > len(nodes) {
		limit = len(nodes)
	}
	return nodes[:limit]
}

// ========== 私有方法 ==========

// cleanupHistory 清理历史数据
func (a *MultiNodeAggregator) cleanupHistory(nodeID string) {
	history := a.history[nodeID]
	if len(history) == 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -a.config.RetentionDays)
	var validHistory []TrendData

	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			validHistory = append(validHistory, point)
		}
	}

	a.history[nodeID] = validHistory
}

// calculateClusterEfficiency 计算集群效率评分
func (a *MultiNodeAggregator) calculateClusterEfficiency(report *ClusterCostReport) float64 {
	score := 100.0

	// 检查离线节点
	offlineRatio := float64(report.ClusterSummary.OfflineNodes) / float64(report.ClusterSummary.TotalNodes)
	score -= offlineRatio * 20 // 离线节点扣分

	// 检查使用率分布
	for _, node := range report.NodeDetails {
		usage := node.UsageStats.StorageUsagePercent
		if usage < a.config.LowUsageThreshold {
			// 低使用率扣分（资源浪费）
			score -= (a.config.LowUsageThreshold - usage) * 0.3
		}
		if usage > a.config.HighUsageThreshold {
			// 高使用率扣分（风险）
			score -= (usage - a.config.HighUsageThreshold) * 0.2
		}
	}

	// 预算超支扣分
	budgetUsage := report.ClusterSummary.BudgetStatus.UsagePercent
	if budgetUsage > a.config.WarningThreshold {
		score -= (budgetUsage - a.config.WarningThreshold) * 0.3
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return round(score, 1)
}

// calculatePotentialSavings 计算潜在节省
func (a *MultiNodeAggregator) calculatePotentialSavings(report *ClusterCostReport) float64 {
	var savings float64

	for _, node := range report.NodeDetails {
		// 低使用率资源节省
		if node.UsageStats.StorageUsagePercent < a.config.LowUsageThreshold {
			// 假设可以释放到50%使用率
			savings += node.Summary.TotalCostMonthly * 0.3
		}

		// 高使用率风险成本
		if node.UsageStats.StorageUsagePercent > a.config.HighUsageThreshold {
			// 可能需要扩容或优化
			savings += node.Summary.TotalCostMonthly * 0.1
		}
	}

	return round(savings, 2)
}

// getBudgetStatus 获取预算状态
func (a *MultiNodeAggregator) getBudgetStatus(used float64) string {
	usage := used / a.config.MonthlyBudget * 100
	if usage >= a.config.CriticalThreshold {
		return "critical"
	}
	if usage >= a.config.WarningThreshold {
		return "warning"
	}
	return "normal"
}

// analyzeClusterTrend 分析集群趋势
func (a *MultiNodeAggregator) analyzeClusterTrend(dataPoints []TrendData) ClusterCostTrend {
	trend := ClusterCostTrend{
		DataPoints: dataPoints,
	}

	if len(dataPoints) == 0 {
		trend.Direction = "stable"
		return trend
	}

	// 计算统计指标
	var sum, max float64
	var min float64 = -1
	for _, p := range dataPoints {
		sum += p.TotalCost
		if p.TotalCost > max {
			max = p.TotalCost
			trend.PeakCost = p.TotalCost
			trend.PeakTime = p.Timestamp
		}
		if min < 0 || p.TotalCost < min {
			min = p.TotalCost
		}
	}

	trend.AvgCost = round(sum/float64(len(dataPoints)), 2)

	// 计算变化趋势
	if len(dataPoints) >= 2 {
		first := dataPoints[0].TotalCost
		last := dataPoints[len(dataPoints)-1].TotalCost
		if first > 0 {
			trend.ChangeRate = round((last-first)/first*100, 2)
		}

		if trend.ChangeRate > 5 {
			trend.Direction = "increasing"
		} else if trend.ChangeRate < -5 {
			trend.Direction = "decreasing"
		} else {
			trend.Direction = "stable"
		}
	}

	return trend
}

// generateClusterForecast 生成集群预测
func (a *MultiNodeAggregator) generateClusterForecast(dataPoints []TrendData, summary ClusterCostSummary) ClusterCostForecast {
	forecast := ClusterCostForecast{
		Model: "linear_regression",
	}

	if len(dataPoints) < 2 {
		forecast.Confidence = 0
		forecast.RiskAlerts = append(forecast.RiskAlerts, "数据不足，无法进行可靠预测")
		return forecast
	}

	// 计算平均增长率
	var totalGrowth float64
	for i := 1; i < len(dataPoints); i++ {
		if dataPoints[i-1].TotalCost > 0 {
			growth := (dataPoints[i].TotalCost - dataPoints[i-1].TotalCost) / dataPoints[i-1].TotalCost
			totalGrowth += growth
		}
	}
	avgGrowthRate := totalGrowth / float64(len(dataPoints)-1)

	// 当前成本
	currentCost := summary.TotalCostMonthly

	// 预测未来成本
	forecast.NextMonthCost = round(currentCost*(1+avgGrowthRate), 2)
	forecast.NextQuarterCost = round(currentCost*(1+avgGrowthRate*3), 2)
	forecast.NextYearCost = round(currentCost*(1+avgGrowthRate*12), 2)

	// 计算置信度
	confidence := 100.0
	if len(dataPoints) < 7 {
		confidence -= float64(7-len(dataPoints)) * 15
	}
	// 基于波动性调整
	trend := a.analyzeClusterTrend(dataPoints)
	if trend.AvgCost > 0 {
		volatility := (trend.PeakCost - trend.AvgCost) / trend.AvgCost * 100
		confidence -= volatility * 0.5
	}
	if confidence < 20 {
		confidence = 20
	}
	forecast.Confidence = round(confidence, 1)

	// 生成预测数据点
	forecast.ForecastPoints = a.generateForecastPoints(dataPoints, avgGrowthRate)

	// 生成风险提示
	forecast.RiskAlerts = a.generateRiskAlerts(forecast, summary)

	return forecast
}

// generateForecastPoints 生成预测数据点
func (a *MultiNodeAggregator) generateForecastPoints(dataPoints []TrendData, growthRate float64) []TrendData {
	if len(dataPoints) == 0 {
		return nil
	}

	forecast := make([]TrendData, 3)
	last := dataPoints[len(dataPoints)-1]
	now := time.Now()

	// 1个月后
	forecast[0] = TrendData{
		Timestamp: now.AddDate(0, 1, 0),
		TotalCost: round(last.TotalCost*(1+growthRate), 2),
	}

	// 3个月后
	forecast[1] = TrendData{
		Timestamp: now.AddDate(0, 3, 0),
		TotalCost: round(last.TotalCost*(1+growthRate*3), 2),
	}

	// 6个月后
	forecast[2] = TrendData{
		Timestamp: now.AddDate(0, 6, 0),
		TotalCost: round(last.TotalCost*(1+growthRate*6), 2),
	}

	return forecast
}

// generateRiskAlerts 生成风险提示
func (a *MultiNodeAggregator) generateRiskAlerts(forecast ClusterCostForecast, summary ClusterCostSummary) []string {
	alerts := make([]string, 0)

	// 预算风险
	if forecast.NextMonthCost > a.config.MonthlyBudget {
		overage := forecast.NextMonthCost - a.config.MonthlyBudget
		alerts = append(alerts, fmt.Sprintf("下月预计超出预算 %.2f 元", overage))
	}

	// 增长风险
	if forecast.NextYearCost > summary.TotalCostYearly*1.5 {
		alerts = append(alerts, "年增长率超过50%，建议检查成本增长原因")
	}

	// 置信度低
	if forecast.Confidence < 50 {
		alerts = append(alerts, "预测置信度较低，建议收集更多历史数据")
	}

	return alerts
}

// generateOptimizationSuggestions 生成优化建议
func (a *MultiNodeAggregator) generateOptimizationSuggestions(report *ClusterCostReport) []ClusterOptimizationSuggestion {
	suggestions := make([]ClusterOptimizationSuggestion, 0)
	idCounter := 1

	for _, node := range report.NodeDetails {
		// 低使用率节点
		if node.UsageStats.StorageUsagePercent < a.config.LowUsageThreshold {
			savings := node.Summary.TotalCostMonthly * 0.3
			suggestions = append(suggestions, ClusterOptimizationSuggestion{
				ID:             fmt.Sprintf("opt_%d", idCounter),
				Type:           "scale_down",
				Priority:       2,
				AffectedNodes:  []string{node.Node.ID},
				Description:    fmt.Sprintf("节点 %s 使用率 %.1f%% 过低，建议资源整合", node.Node.Name, node.UsageStats.StorageUsagePercent),
				PotentialSaving: round(savings, 2),
				Complexity:     "medium",
				EstimatedHours: 4,
				ROIScore:       round(savings/50*10, 1), // 简单ROI评分
			})
			idCounter++
		}

		// 高使用率节点
		if node.UsageStats.StorageUsagePercent > a.config.HighUsageThreshold {
			suggestions = append(suggestions, ClusterOptimizationSuggestion{
				ID:             fmt.Sprintf("opt_%d", idCounter),
				Type:           "scale_up",
				Priority:       1,
				AffectedNodes:  []string{node.Node.ID},
				Description:    fmt.Sprintf("节点 %s 使用率 %.1f%% 过高，建议扩容", node.Node.Name, node.UsageStats.StorageUsagePercent),
				PotentialSaving: 0, // 扩容没有节省，但避免风险
				Complexity:     "high",
				EstimatedHours: 8,
				ROIScore:       80, // 高优先级
			})
			idCounter++
		}

		// 检查资源优化建议
		for _, res := range node.Resources {
			if res.EfficiencyScore < 50 {
				savings := res.MonthlyCost * 0.2
				suggestions = append(suggestions, ClusterOptimizationSuggestion{
					ID:             fmt.Sprintf("opt_%d", idCounter),
					Type:           "optimize",
					Priority:       3,
					AffectedNodes:  []string{node.Node.ID},
					Description:    fmt.Sprintf("资源 %s 效率评分 %.1f，建议优化", res.Name, res.EfficiencyScore),
					PotentialSaving: round(savings, 2),
					Complexity:     "low",
					EstimatedHours: 2,
					ROIScore:       round(savings/20*10, 1),
				})
				idCounter++
			}
		}
	}

	// 按ROI评分排序（取前10个）
	for i := 0; i < len(suggestions) && i < 10; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].ROIScore > suggestions[i].ROIScore {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	return suggestions
}