// Package cost - 多节点成本聚合模块
// 对标TrueNAS企业报告 + 群晖成本管理
package cost

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

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
	StorageCost      float64 `json:"storage_cost"`
	ElectricityCost  float64 `json:"electricity_cost"`
	NetworkCost      float64 `json:"network_cost"`
	OpsCost          float64 `json:"ops_cost"`
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
	mu        sync.RWMutex
	nodes     map[string]*NodeCostStats
	history   map[string][]TrendData // nodeID -> history
	config    MultiNodeConfig
	dashboard *DashboardService
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
			Node:        node,
			CollectedAt: now,
			RecentTrend: make([]TrendData, 0),
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
		ID:                      fmt.Sprintf("cluster_report_%d", now.Unix()),
		GeneratedAt:             now,
		TimeRange:               timeRange,
		CostByType:              make(map[CostType]float64),
		CostByRegion:            make(map[string]float64),
		NodeDetails:             make([]NodeCostStats, 0),
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
		TotalNodes:         len(a.nodes),
		OnlineNodes:        onlineNodes,
		OfflineNodes:       offlineNodes,
		TotalCapacityBytes: totalCapacity,
		UsedCapacityBytes:  usedCapacity,
		TotalCostMonthly:   round(totalCost, 2),
		TotalCostYearly:    round(totalCost*12, 2),
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
		Used:          totalCost,
		Remaining:     a.config.MonthlyBudget - totalCost,
		UsagePercent:  round(totalCost/a.config.MonthlyBudget*100, 2),
		Status:        a.getBudgetStatus(totalCost),
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
			Used:          totalCost,
			Remaining:     a.config.MonthlyBudget - totalCost,
			UsagePercent:  round(totalCost/a.config.MonthlyBudget*100, 2),
			Status:        a.getBudgetStatus(totalCost),
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
				ID:              fmt.Sprintf("opt_%d", idCounter),
				Type:            "scale_down",
				Priority:        2,
				AffectedNodes:   []string{node.Node.ID},
				Description:     fmt.Sprintf("节点 %s 使用率 %.1f%% 过低，建议资源整合", node.Node.Name, node.UsageStats.StorageUsagePercent),
				PotentialSaving: round(savings, 2),
				Complexity:      "medium",
				EstimatedHours:  4,
				ROIScore:        round(savings/50*10, 1), // 简单ROI评分
			})
			idCounter++
		}

		// 高使用率节点
		if node.UsageStats.StorageUsagePercent > a.config.HighUsageThreshold {
			suggestions = append(suggestions, ClusterOptimizationSuggestion{
				ID:              fmt.Sprintf("opt_%d", idCounter),
				Type:            "scale_up",
				Priority:        1,
				AffectedNodes:   []string{node.Node.ID},
				Description:     fmt.Sprintf("节点 %s 使用率 %.1f%% 过高，建议扩容", node.Node.Name, node.UsageStats.StorageUsagePercent),
				PotentialSaving: 0, // 扩容没有节省，但避免风险
				Complexity:      "high",
				EstimatedHours:  8,
				ROIScore:        80, // 高优先级
			})
			idCounter++
		}

		// 检查资源优化建议
		for _, res := range node.Resources {
			if res.EfficiencyScore < 50 {
				savings := res.MonthlyCost * 0.2
				suggestions = append(suggestions, ClusterOptimizationSuggestion{
					ID:              fmt.Sprintf("opt_%d", idCounter),
					Type:            "optimize",
					Priority:        3,
					AffectedNodes:   []string{node.Node.ID},
					Description:     fmt.Sprintf("资源 %s 效率评分 %.1f，建议优化", res.Name, res.EfficiencyScore),
					PotentialSaving: round(savings, 2),
					Complexity:      "low",
					EstimatedHours:  2,
					ROIScore:        round(savings/20*10, 1),
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

// ========== 跨节点资源统计功能 ==========

// CrossNodeResourceStats 跨节点资源统计
type CrossNodeResourceStats struct {
	// 统计ID
	ID string `json:"id"`

	// 统计时间
	StatTime time.Time `json:"stat_time"`

	// 总资源数量
	TotalResourceCount int `json:"total_resource_count"`

	// 按类型统计
	ResourceByType map[string]int `json:"resource_by_type"`

	// 按节点统计
	ResourceByNode map[string]int `json:"resource_by_node"`

	// 按区域统计
	ResourceByRegion map[string]int `json:"resource_by_region"`

	// 跨节点资源分布
	ResourceDistribution []ResourceDistribution `json:"resource_distribution"`

	// 资源健康度统计
	HealthStats ResourceHealthStats `json:"health_stats"`

	// 容量利用率统计
	CapacityStats CrossNodeCapacityStats `json:"capacity_stats"`

	// 成本分布
	CostDistribution CostDistributionStats `json:"cost_distribution"`

	// 异常资源列表
	AbnormalResources []AbnormalResource `json:"abnormal_resources"`

	// 优化建议
	Recommendations []string `json:"recommendations"`
}

// ResourceDistribution 资源分布
type ResourceDistribution struct {
	// 资源类型
	Type string `json:"type"`

	// 节点分布（节点ID -> 数量）
	NodeDistribution map[string]int `json:"node_distribution"`

	// 区域分布（区域 -> 数量）
	RegionDistribution map[string]int `json:"region_distribution"`

	// 是否均衡
	IsBalanced bool `json:"is_balanced"`

	// 均衡度评分（0-100）
	BalanceScore float64 `json:"balance_score"`

	// 建议
	Suggestions []string `json:"suggestions"`
}

// ResourceHealthStats 资源健康度统计
type ResourceHealthStats struct {
	// 健康资源数
	HealthyCount int `json:"healthy_count"`

	// 警告资源数
	WarningCount int `json:"warning_count"`

	// 异常资源数
	CriticalCount int `json:"critical_count"`

	// 离线资源数
	OfflineCount int `json:"offline_count"`

	// 健康率（%）
	HealthRate float64 `json:"health_rate"`

	// 平均健康评分
	AvgHealthScore float64 `json:"avg_health_score"`

	// 按类型健康统计
	HealthByType map[string]TypeHealthStats `json:"health_by_type"`
}

// TypeHealthStats 类型健康统计
type TypeHealthStats struct {
	// 总数
	Total int `json:"total"`

	// 健康数
	Healthy int `json:"healthy"`

	// 警告数
	Warning int `json:"warning"`

	// 异常数
	Critical int `json:"critical"`

	// 健康率
	HealthRate float64 `json:"health_rate"`
}

// CrossNodeCapacityStats 跨节点容量统计
type CrossNodeCapacityStats struct {
	// 总容量（字节）
	TotalCapacityBytes uint64 `json:"total_capacity_bytes"`

	// 总已用容量（字节）
	TotalUsedBytes uint64 `json:"total_used_bytes"`

	// 总可用容量（字节）
	TotalAvailableBytes uint64 `json:"total_available_bytes"`

	// 平均使用率（%）
	AvgUsagePercent float64 `json:"avg_usage_percent"`

	// 最高使用率节点
	MaxUsageNode string `json:"max_usage_node"`

	// 最高使用率（%）
	MaxUsagePercent float64 `json:"max_usage_percent"`

	// 最低使用率节点
	MinUsageNode string `json:"min_usage_node"`

	// 最低使用率（%）
	MinUsagePercent float64 `json:"min_usage_percent"`

	// 使用率分布区间
	UsageDistribution []UsageRange `json:"usage_distribution"`

	// 容量热点（高使用率资源）
	CapacityHotspots []CapacityHotspot `json:"capacity_hotspots"`

	// 容量冷点（低使用率资源）
	CapacityColdspots []CapacityColdspot `json:"capacity_coldspots"`
}

// UsageRange 使用率区间
type UsageRange struct {
	// 区间名称
	Name string `json:"name"`

	// 下限（%）
	MinPercent float64 `json:"min_percent"`

	// 上限（%）
	MaxPercent float64 `json:"max_percent"`

	// 资源数量
	Count int `json:"count"`

	// 容量占比（%）
	CapacityPercent float64 `json:"capacity_percent"`
}

// CapacityHotspot 容量热点
type CapacityHotspot struct {
	// 节点ID
	NodeID string `json:"node_id"`

	// 节点名称
	NodeName string `json:"node_name"`

	// 资源名称
	ResourceName string `json:"resource_name"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 剩余容量（GB）
	RemainingGB float64 `json:"remaining_gb"`

	// 预计耗尽时间（天）
	EstimatedDaysToFull int `json:"estimated_days_to_full"`

	// 风险级别
	RiskLevel string `json:"risk_level"`

	// 建议
	Suggestion string `json:"suggestion"`
}

// CapacityColdspot 容量冷点
type CapacityColdspot struct {
	// 节点ID
	NodeID string `json:"node_id"`

	// 节点名称
	NodeName string `json:"node_name"`

	// 资源名称
	ResourceName string `json:"resource_name"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 闲置容量（GB）
	IdleGB float64 `json:"idle_gb"`

	// 潜在节省成本（元/月）
	PotentialSavings float64 `json:"potential_savings"`

	// 建议
	Suggestion string `json:"suggestion"`
}

// CostDistributionStats 成本分布统计
type CostDistributionStats struct {
	// 总成本（元/月）
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// 按节点分布
	CostByNode map[string]float64 `json:"cost_by_node"`

	// 按类型分布
	CostByType map[string]float64 `json:"cost_by_type"`

	// 按区域分布
	CostByRegion map[string]float64 `json:"cost_by_region"`

	// 成本集中度（最高成本节点占比）
	ConcentrationRate float64 `json:"concentration_rate"`

	// 成本最高节点
	HighestCostNode string `json:"highest_cost_node"`

	// 最高节点成本
	HighestNodeCost float64 `json:"highest_node_cost"`

	// 成本最低节点
	LowestCostNode string `json:"lowest_cost_node"`

	// 最低节点成本
	LowestNodeCost float64 `json:"lowest_node_cost"`

	// 平均节点成本
	AvgNodeCost float64 `json:"avg_node_cost"`

	// 成本方差
	CostVariance float64 `json:"cost_variance"`
}

// AbnormalResource 异常资源
type AbnormalResource struct {
	// 节点ID
	NodeID string `json:"node_id"`

	// 节点名称
	NodeName string `json:"node_name"`

	// 资源名称
	ResourceName string `json:"resource_name"`

	// 资源类型
	ResourceType string `json:"resource_type"`

	// 异常类型（high_usage/low_usage/offline/error）
	AbnormalType string `json:"abnormal_type"`

	// 异常描述
	Description string `json:"description"`

	// 异常时间
	DetectedAt time.Time `json:"detected_at"`

	// 影响程度（1-5）
	ImpactLevel int `json:"impact_level"`

	// 建议处理方案
	SuggestedAction string `json:"suggested_action"`
}

// GetCrossNodeResourceStats 获取跨节点资源统计
func (a *MultiNodeAggregator) GetCrossNodeResourceStats() *CrossNodeResourceStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := time.Now()
	stats := &CrossNodeResourceStats{
		ID:                   fmt.Sprintf("cross_node_stats_%d", now.Unix()),
		StatTime:             now,
		ResourceByType:       make(map[string]int),
		ResourceByNode:       make(map[string]int),
		ResourceByRegion:     make(map[string]int),
		ResourceDistribution: make([]ResourceDistribution, 0),
		HealthStats: ResourceHealthStats{
			HealthByType: make(map[string]TypeHealthStats),
		},
		CapacityStats: CrossNodeCapacityStats{
			UsageDistribution: make([]UsageRange, 0),
			CapacityHotspots:  make([]CapacityHotspot, 0),
			CapacityColdspots: make([]CapacityColdspot, 0),
		},
		CostDistribution: CostDistributionStats{
			CostByNode:   make(map[string]float64),
			CostByType:   make(map[string]float64),
			CostByRegion: make(map[string]float64),
		},
		AbnormalResources: make([]AbnormalResource, 0),
		Recommendations:   make([]string, 0),
	}

	// 收集所有资源信息
	allResources := make(map[string][]ResourceCostDetail)
	totalResources := 0

	for nodeID, nodeStats := range a.nodes {
		nodeResources := nodeStats.Resources
		allResources[nodeID] = nodeResources
		totalResources += len(nodeResources)

		// 按节点统计
		stats.ResourceByNode[nodeID] = len(nodeResources)

		// 按区域统计
		region := nodeStats.Node.Region
		if region == "" {
			region = "default"
		}
		stats.ResourceByRegion[region] += len(nodeResources)

		// 按类型统计
		for _, res := range nodeResources {
			stats.ResourceByType[res.Type]++

			// 成本按节点
			stats.CostDistribution.CostByNode[nodeID] += res.MonthlyCost

			// 成本按类型
			stats.CostDistribution.CostByType[res.Type] += res.MonthlyCost

			// 成本按区域
			stats.CostDistribution.CostByRegion[region] += res.MonthlyCost

			// 累加总成本
			stats.CostDistribution.TotalCostMonthly += res.MonthlyCost
		}

		// 累加容量
		for _, res := range nodeResources {
			stats.CapacityStats.TotalCapacityBytes += res.TotalCapacityBytes
			stats.CapacityStats.TotalUsedBytes += res.UsedCapacityBytes
		}
	}

	stats.TotalResourceCount = totalResources

	// 计算可用容量
	stats.CapacityStats.TotalAvailableBytes = stats.CapacityStats.TotalCapacityBytes - stats.CapacityStats.TotalUsedBytes

	// 计算平均使用率
	if stats.CapacityStats.TotalCapacityBytes > 0 {
		stats.CapacityStats.AvgUsagePercent = round(float64(stats.CapacityStats.TotalUsedBytes)/
			float64(stats.CapacityStats.TotalCapacityBytes)*100, 2)
	}

	// 分析资源分布均衡度
	stats.ResourceDistribution = a.analyzeResourceDistribution(allResources)

	// 分析健康度
	stats.HealthStats = a.analyzeResourceHealth(allResources)

	// 分析容量使用情况
	stats.CapacityStats = a.analyzeCapacityUsage(allResources, stats.CapacityStats)

	// 分析成本分布
	stats.CostDistribution = a.analyzeCostDistribution(stats.CostDistribution)

	// 识别异常资源
	stats.AbnormalResources = a.identifyAbnormalResources(allResources)

	// 生成建议
	stats.Recommendations = a.generateCrossNodeRecommendations(stats)

	return stats
}

// analyzeResourceDistribution 分析资源分布均衡度
func (a *MultiNodeAggregator) analyzeResourceDistribution(allResources map[string][]ResourceCostDetail) []ResourceDistribution {
	distributions := make([]ResourceDistribution, 0)

	// 按类型分析分布
	typeNodeMap := make(map[string]map[string]int)
	typeRegionMap := make(map[string]map[string]int)

	for nodeID, resources := range allResources {
		node := a.nodes[nodeID]
		region := "default"
		if node != nil && node.Node.Region != "" {
			region = node.Node.Region
		}

		for _, res := range resources {
			if typeNodeMap[res.Type] == nil {
				typeNodeMap[res.Type] = make(map[string]int)
				typeRegionMap[res.Type] = make(map[string]int)
			}
			typeNodeMap[res.Type][nodeID]++
			typeRegionMap[res.Type][region]++
		}
	}

	for resType, nodeDist := range typeNodeMap {
		dist := ResourceDistribution{
			Type:               resType,
			NodeDistribution:   nodeDist,
			RegionDistribution: typeRegionMap[resType],
			Suggestions:        make([]string, 0),
		}

		// 计算均衡度评分
		dist.BalanceScore, dist.IsBalanced = a.calculateBalanceScore(nodeDist, len(allResources))

		// 生成建议
		if !dist.IsBalanced {
			dist.Suggestions = append(dist.Suggestions,
				fmt.Sprintf("%s 类型资源分布不均衡，建议调整分布", resType))
		}

		distributions = append(distributions, dist)
	}

	return distributions
}

// calculateBalanceScore 计算均衡度评分
func (a *MultiNodeAggregator) calculateBalanceScore(distribution map[string]int, totalNodes int) (float64, bool) {
	if len(distribution) == 0 || totalNodes == 0 {
		return 0, false
	}

	// 计算平均值
	total := 0
	for _, count := range distribution {
		total += count
	}
	avg := float64(total) / float64(len(distribution))

	// 计算方差
	var variance float64
	for _, count := range distribution {
		diff := float64(count) - avg
		variance += diff * diff
	}
	variance /= float64(len(distribution))

	// 标准差
	stdDev := math.Sqrt(variance)

	// 均衡度评分：标准差越小评分越高
	// 评分 = 100 - (标准差/平均值 * 50)
	score := 100.0
	if avg > 0 {
		score = 100.0 - (stdDev / avg * 50)
	}

	if score < 0 {
		score = 0
	}

	// 均衡阈值：评分>=70认为是均衡的
	isBalanced := score >= 70.0

	return round(score, 1), isBalanced
}

// analyzeResourceHealth 分析资源健康度
func (a *MultiNodeAggregator) analyzeResourceHealth(allResources map[string][]ResourceCostDetail) ResourceHealthStats {
	stats := ResourceHealthStats{
		HealthByType: make(map[string]TypeHealthStats),
	}

	totalHealthScore := 0.0

	for nodeID, resources := range allResources {
		node := a.nodes[nodeID]
		nodeStatus := "online"
		if node != nil {
			nodeStatus = node.Node.Status
		}

		for _, res := range resources {
			// 判断健康状态基于效率评分
			healthStatus := "healthy"
			if nodeStatus == "offline" {
				healthStatus = "offline"
				stats.OfflineCount++
			} else if res.EfficiencyScore < 30 {
				healthStatus = "critical"
				stats.CriticalCount++
			} else if res.EfficiencyScore < 60 {
				healthStatus = "warning"
				stats.WarningCount++
			} else {
				stats.HealthyCount++
			}

			// 累加健康评分
			if healthStatus != "offline" {
				totalHealthScore += res.EfficiencyScore
			}

			// 按类型统计
			typeStats := stats.HealthByType[res.Type]
			typeStats.Total++
			if healthStatus == "healthy" {
				typeStats.Healthy++
			} else if healthStatus == "warning" {
				typeStats.Warning++
			} else if healthStatus == "critical" {
				typeStats.Critical++
			}
			if typeStats.Total > 0 {
				typeStats.HealthRate = round(float64(typeStats.Healthy)/float64(typeStats.Total)*100, 2)
			}
			stats.HealthByType[res.Type] = typeStats
		}
	}

	// 计算总体健康率
	totalActive := stats.HealthyCount + stats.WarningCount + stats.CriticalCount
	if totalActive > 0 {
		stats.HealthRate = round(float64(stats.HealthyCount)/float64(totalActive)*100, 2)
		stats.AvgHealthScore = round(totalHealthScore/float64(totalActive), 1)
	}

	return stats
}

// analyzeCapacityUsage 分析容量使用情况
func (a *MultiNodeAggregator) analyzeCapacityUsage(allResources map[string][]ResourceCostDetail, capacityStats CrossNodeCapacityStats) CrossNodeCapacityStats {
	maxUsage := 0.0
	minUsage := 100.0
	maxUsageNode := ""
	minUsageNode := ""

	// 使用率区间统计
	usageRanges := []UsageRange{
		{Name: "低使用率", MinPercent: 0, MaxPercent: 30},
		{Name: "正常使用率", MinPercent: 30, MaxPercent: 70},
		{Name: "高使用率", MinPercent: 70, MaxPercent: 90},
		{Name: "危险使用率", MinPercent: 90, MaxPercent: 100},
	}

	for i := range usageRanges {
		usageRanges[i].Count = 0
		usageRanges[i].CapacityPercent = 0
	}

	// 分析每个资源
	for nodeID, resources := range allResources {
		node := a.nodes[nodeID]
		nodeName := "unknown"
		if node != nil {
			nodeName = node.Node.Name
		}

		for _, res := range resources {
			if res.TotalCapacityBytes == 0 {
				continue
			}

			usagePercent := float64(res.UsedCapacityBytes) / float64(res.TotalCapacityBytes) * 100

			// 更新最大最小使用率
			if usagePercent > maxUsage {
				maxUsage = usagePercent
				maxUsageNode = nodeID
			}
			if usagePercent < minUsage {
				minUsage = usagePercent
				minUsageNode = nodeID
			}

			// 统计使用率区间
			for i, rangeDef := range usageRanges {
				if usagePercent >= rangeDef.MinPercent && usagePercent < rangeDef.MaxPercent {
					usageRanges[i].Count++
					usageRanges[i].CapacityPercent += float64(res.TotalCapacityBytes) /
						float64(capacityStats.TotalCapacityBytes) * 100
				}
			}

			// 识别热点（高使用率）
			if usagePercent > a.config.HighUsageThreshold {
				remainingGB := float64(res.TotalCapacityBytes-res.UsedCapacityBytes) / (1024 * 1024 * 1024)
				hotspot := CapacityHotspot{
					NodeID:       nodeID,
					NodeName:     nodeName,
					ResourceName: res.Name,
					UsagePercent: round(usagePercent, 2),
					RemainingGB:  round(remainingGB, 2),
					RiskLevel:    "high",
					Suggestion:   "建议尽快扩容或迁移数据",
				}

				// 简单估算耗尽时间（假设每月增长10%）
				if usagePercent > 90 {
					hotspot.EstimatedDaysToFull = 30 // 高风险
					hotspot.RiskLevel = "critical"
				} else {
					hotspot.EstimatedDaysToFull = 90 // 中风险
				}

				capacityStats.CapacityHotspots = append(capacityStats.CapacityHotspots, hotspot)
			}

			// 识别冷点（低使用率）
			if usagePercent < a.config.LowUsageThreshold {
				idleGB := float64(res.TotalCapacityBytes-res.UsedCapacityBytes) / (1024 * 1024 * 1024)
				coldspot := CapacityColdspot{
					NodeID:           nodeID,
					NodeName:         nodeName,
					ResourceName:     res.Name,
					UsagePercent:     round(usagePercent, 2),
					IdleGB:           round(idleGB, 2),
					PotentialSavings: round(idleGB*0.05, 2), // 假设0.05元/GB
					Suggestion:       "建议释放闲置资源或重新分配",
				}
				capacityStats.CapacityColdspots = append(capacityStats.CapacityColdspots, coldspot)
			}
		}
	}

	capacityStats.MaxUsagePercent = round(maxUsage, 2)
	capacityStats.MaxUsageNode = maxUsageNode
	capacityStats.MinUsagePercent = round(minUsage, 2)
	capacityStats.MinUsageNode = minUsageNode

	// 调整区间统计
	for i := range usageRanges {
		usageRanges[i].CapacityPercent = round(usageRanges[i].CapacityPercent, 2)
	}
	capacityStats.UsageDistribution = usageRanges

	return capacityStats
}

// analyzeCostDistribution 分析成本分布
func (a *MultiNodeAggregator) analyzeCostDistribution(costStats CostDistributionStats) CostDistributionStats {
	if len(costStats.CostByNode) == 0 {
		return costStats
	}

	// 找出最高/最低成本节点
	maxCost := 0.0
	minCost := -1.0
	maxNode := ""
	minNode := ""
	var sum float64

	for nodeID, cost := range costStats.CostByNode {
		sum += cost
		if cost > maxCost {
			maxCost = cost
			maxNode = nodeID
		}
		if minCost < 0 || cost < minCost {
			minCost = cost
			minNode = nodeID
		}
	}

	costStats.HighestCostNode = maxNode
	costStats.HighestNodeCost = round(maxCost, 2)
	costStats.LowestCostNode = minNode
	costStats.LowestNodeCost = round(minCost, 2)

	nodeCount := len(costStats.CostByNode)
	costStats.AvgNodeCost = round(sum/float64(nodeCount), 2)

	// 计算成本集中度
	costStats.ConcentrationRate = round(maxCost/costStats.TotalCostMonthly*100, 2)

	// 计算成本方差
	var variance float64
	for _, cost := range costStats.CostByNode {
		diff := cost - costStats.AvgNodeCost
		variance += diff * diff
	}
	costStats.CostVariance = round(variance/float64(nodeCount), 2)

	return costStats
}

// identifyAbnormalResources 识别异常资源
func (a *MultiNodeAggregator) identifyAbnormalResources(allResources map[string][]ResourceCostDetail) []AbnormalResource {
	abnormal := make([]AbnormalResource, 0)
	now := time.Now()

	for nodeID, resources := range allResources {
		node := a.nodes[nodeID]
		nodeName := "unknown"
		nodeStatus := "online"
		if node != nil {
			nodeName = node.Node.Name
			nodeStatus = node.Node.Status
		}

		for _, res := range resources {
			// 检查节点状态
			if nodeStatus == "offline" {
				abnormal = append(abnormal, AbnormalResource{
					NodeID:          nodeID,
					NodeName:        nodeName,
					ResourceName:    res.Name,
					ResourceType:    res.Type,
					AbnormalType:    "offline",
					Description:     "节点离线，资源不可访问",
					DetectedAt:      now,
					ImpactLevel:     5,
					SuggestedAction: "检查节点连接状态",
				})
				continue
			}

			// 检查使用率
			if res.TotalCapacityBytes > 0 {
				usagePercent := float64(res.UsedCapacityBytes) / float64(res.TotalCapacityBytes) * 100

				if usagePercent > a.config.HighUsageThreshold {
					abnormal = append(abnormal, AbnormalResource{
						NodeID:       nodeID,
						NodeName:     nodeName,
						ResourceName: res.Name,
						ResourceType: res.Type,
						AbnormalType: "high_usage",
						Description: fmt.Sprintf("使用率 %.2f%% 超过阈值 %.2f%%",
							usagePercent, a.config.HighUsageThreshold),
						DetectedAt:      now,
						ImpactLevel:     4,
						SuggestedAction: "扩容或迁移数据",
					})
				}

				if usagePercent < a.config.LowUsageThreshold {
					abnormal = append(abnormal, AbnormalResource{
						NodeID:       nodeID,
						NodeName:     nodeName,
						ResourceName: res.Name,
						ResourceType: res.Type,
						AbnormalType: "low_usage",
						Description: fmt.Sprintf("使用率 %.2f%% 低于阈值 %.2f%%，资源浪费",
							usagePercent, a.config.LowUsageThreshold),
						DetectedAt:      now,
						ImpactLevel:     2,
						SuggestedAction: "释放闲置空间或重新分配",
					})
				}
			}

			// 检查效率评分
			if res.EfficiencyScore < 30 {
				abnormal = append(abnormal, AbnormalResource{
					NodeID:          nodeID,
					NodeName:        nodeName,
					ResourceName:    res.Name,
					ResourceType:    res.Type,
					AbnormalType:    "low_efficiency",
					Description:     fmt.Sprintf("效率评分 %.1f 过低", res.EfficiencyScore),
					DetectedAt:      now,
					ImpactLevel:     3,
					SuggestedAction: "优化资源配置或迁移",
				})
			}
		}
	}

	return abnormal
}

// generateCrossNodeRecommendations 生成跨节点建议
func (a *MultiNodeAggregator) generateCrossNodeRecommendations(stats *CrossNodeResourceStats) []string {
	recommendations := make([]string, 0)

	// 基于健康度的建议
	if stats.HealthStats.HealthRate < 80 {
		recommendations = append(recommendations,
			fmt.Sprintf("整体健康率 %.2f%% 较低，建议优先处理 %d 个异常资源",
				stats.HealthStats.HealthRate, stats.HealthStats.CriticalCount))
	}

	// 基于容量使用的建议
	if len(stats.CapacityStats.CapacityHotspots) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("发现 %d 个容量热点，建议优先扩容或数据迁移",
				len(stats.CapacityStats.CapacityHotspots)))
	}

	if len(stats.CapacityStats.CapacityColdspots) > 0 {
		potentialSavings := 0.0
		for _, cs := range stats.CapacityStats.CapacityColdspots {
			potentialSavings += cs.PotentialSavings
		}
		recommendations = append(recommendations,
			fmt.Sprintf("发现 %d 个容量冷点，潜在节省 %.2f 元/月",
				len(stats.CapacityStats.CapacityColdspots), potentialSavings))
	}

	// 基于成本分布的建议
	if stats.CostDistribution.ConcentrationRate > 50 {
		recommendations = append(recommendations,
			fmt.Sprintf("成本集中度 %.2f%% 过高，建议分散资源降低风险",
				stats.CostDistribution.ConcentrationRate))
	}

	if stats.CostDistribution.CostVariance > stats.CostDistribution.AvgNodeCost*0.5 {
		recommendations = append(recommendations,
			"节点成本差异较大，建议调整资源分布实现成本均衡")
	}

	// 基于资源分布的建议
	for _, dist := range stats.ResourceDistribution {
		if !dist.IsBalanced {
			recommendations = append(recommendations,
				fmt.Sprintf("%s 类型资源分布不均衡（评分 %.1f），建议优化分布",
					dist.Type, dist.BalanceScore))
		}
	}

	// 异常资源处理建议
	highImpactCount := 0
	for _, abnormal := range stats.AbnormalResources {
		if abnormal.ImpactLevel >= 4 {
			highImpactCount++
		}
	}
	if highImpactCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("有 %d 个高影响异常资源需要紧急处理", highImpactCount))
	}

	return recommendations
}
