// Package cost - Fleet管理成本评估模块
// 评估多节点Fleet管理的成本效益和运维成本
package cost

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ========== Fleet成本配置 ==========

// FleetCostConfig Fleet成本配置
type FleetCostConfig struct {
	// 节点数量
	NodeCount int `json:"node_count"`

	// 单节点基础成本（元/月）- 硬件+基础运维
	BaseNodeCostMonthly float64 `json:"base_node_cost_monthly"`

	// Fleet管理软件成本（元/月）
	FleetSoftwareCostMonthly float64 `json:"fleet_software_cost_monthly"`

	// 网络互联成本（元/月）- 专线/VPN等
	NetworkInterconnectCost float64 `json:"network_interconnect_cost"`

	// 集群协调成本系数 - 多节点协调额外开销
	CoordinationCostFactor float64 `json:"coordination_cost_factor"`

	// 运维人力成本（元/月）- Fleet专用运维人员
	OpsStaffCostMonthly float64 `json:"ops_staff_cost_monthly"`

	// 监控系统成本（元/月）
	MonitoringCostMonthly float64 `json:"monitoring_cost_monthly"`

	// 备份同步成本（元/月）- 跨节点备份
	BackupSyncCostMonthly float64 `json:"backup_sync_cost_monthly"`

	// 高可用冗余成本系数 - HA额外节点成本
	HARedundancyFactor float64 `json:"ha_redundancy_factor"`

	// 自动化程度系数（0-1，越高自动化成本越低）
	AutomationFactor float64 `json:"automation_factor"`

	// 单节点平均存储容量（TB）
	AvgNodeStorageTB float64 `json:"avg_node_storage_tb"`

	// 单节点平均带宽成本（元/月）
	AvgNodeBandwidthCost float64 `json:"avg_node_bandwidth_cost"`
}

// DefaultFleetCostConfig 默认Fleet成本配置
func DefaultFleetCostConfig() FleetCostConfig {
	return FleetCostConfig{
		NodeCount:                5,
		BaseNodeCostMonthly:      500.0,   // 单节点基础成本500元/月
		FleetSoftwareCostMonthly: 200.0,   // Fleet管理软件200元/月
		NetworkInterconnectCost:  150.0,   // 网络互联150元/月
		CoordinationCostFactor:   0.05,    // 5%协调开销
		OpsStaffCostMonthly:      3000.0,  // 运维人员3000元/月
		MonitoringCostMonthly:    100.0,   // 监控系统100元/月
		BackupSyncCostMonthly:    200.0,   // 跨节点备份200元/月
		HARedundancyFactor:       0.2,     // HA冗余20%额外成本
		AutomationFactor:         0.7,     // 70%自动化程度
		AvgNodeStorageTB:         20.0,    // 单节点20TB
		AvgNodeBandwidthCost:     50.0,    // 单节点带宽50元/月
	}
}

// FleetCostAnalysis Fleet成本分析结果
type FleetCostAnalysis struct {
	// 分析ID
	ID string `json:"id"`

	// 分析时间
	AnalysisTime time.Time `json:"analysis_time"`

	// 配置参数
	Config FleetCostConfig `json:"config"`

	// ========== 直接成本 ==========

	// 基础节点成本（元/月）
	BaseNodesCost float64 `json:"base_nodes_cost"`

	// Fleet软件成本（元/月）
	FleetSoftwareCost float64 `json:"fleet_software_cost"`

	// 网络互联成本（元/月）
	NetworkCost float64 `json:"network_cost"`

	// 运维人力成本（元/月）
	OpsStaffCost float64 `json:"ops_staff_cost"`

	// 监控系统成本（元/月）
	MonitoringCost float64 `json:"monitoring_cost"`

	// 备份同步成本（元/月）
	BackupSyncCost float64 `json:"backup_sync_cost"`

	// 带宽成本（元/月）
	BandwidthCost float64 `json:"bandwidth_cost"`

	// 总直接成本（元/月）
	TotalDirectCost float64 `json:"total_direct_cost"`

	// ========== 间接成本 ==========

	// 协调开销成本（元/月）
	CoordinationCost float64 `json:"coordination_cost"`

	// HA冗余成本（元/月）
	HARedundancyCost float64 `json:"ha_redundancy_cost"`

	// 学习培训成本（一次性，摊销月）
	TrainingCostMonthly float64 `json:"training_cost_monthly"`

	// 故障恢复成本预估（元/月）
	FailureRecoveryCost float64 `json:"failure_recovery_cost"`

	// 安全合规成本（元/月）
	SecurityComplianceCost float64 `json:"security_compliance_cost"`

	// 总间接成本（元/月）
	TotalIndirectCost float64 `json:"total_indirect_cost"`

	// ========== 总成本 ==========

	// 总月度成本（元/月）
	TotalMonthlyCost float64 `json:"total_monthly_cost"`

	// 总年度成本（元/年）
	TotalYearlyCost float64 `json:"total_yearly_cost"`

	// 单节点平均成本（元/月）
	AvgCostPerNode float64 `json:"avg_cost_per_node"`

	// 单TB成本（元/月）
	CostPerTB float64 `json:"cost_per_tb"`

	// ========== 成本节省潜力 ==========

	// 自动化节省（元/月）
	AutomationSavings float64 `json:"automation_savings"`

	// 集群效率节省（元/月）
	EfficiencySavings float64 `json:"efficiency_savings"`

	// 共享资源节省（元/月）
	SharedResourceSavings float64 `json:"shared_resource_savings"`

	// 总节省潜力（元/月）
	TotalSavingsPotential float64 `json:"total_savings_potential"`

	// ========== ROI指标 ==========

	// Fleet管理ROI（对比单节点独立管理）
	FleetROI float64 `json:"fleet_roi"`

	// 成本效益评分（0-100）
	CostBenefitScore float64 `json:"cost_benefit_score"`

	// ========== 对比分析 ==========

	// 单节点独立管理总成本（对比基准）
	StandaloneTotalCost float64 `json:"standalone_total_cost"`

	// Fleet管理成本差异
	FleetCostDifference float64 `json:"fleet_cost_difference"`

	// Fleet管理是否更划算
	FleetMoreCostEffective bool `json:"fleet_more_cost_effective"`

	// ========== 建议 ==========

	// 成本优化建议
	CostOptimizations []FleetCostOptimization `json:"cost_optimizations"`

	// 风险提示
	Risks []string `json:"risks"`

	// 成本明细
	CostBreakdown map[string]float64 `json:"cost_breakdown"`
}

// FleetCostOptimization Fleet成本优化建议
type FleetCostOptimization struct {
	// 建议ID
	ID string `json:"id"`

	// 优化类型
	Type string `json:"type"` // automation/consolidation/sharing/monitoring/backup

	// 描述
	Description string `json:"description"`

	// 当前成本（元/月）
	CurrentCost float64 `json:"current_cost"`

	// 优化后成本（元/月）
	OptimizedCost float64 `json:"optimized_cost"`

	// 节省金额（元/月）
	Savings float64 `json:"savings"`

	// 实施难度
	Difficulty string `json:"difficulty"` // easy/medium/hard

	// 预计实施时间（周）
	EstimatedWeeks int `json:"estimated_weeks"`

	// ROI评分
	ROIScore float64 `json:"roi_score"`
}

// FleetScenarioAnalysis Fleet场景分析
type FleetScenarioAnalysis struct {
	// 场景名称
	Scenario string `json:"scenario"`

	// 节点数量
	NodeCount int `json:"node_count"`

	// Fleet总成本（元/月）
	FleetCost float64 `json:"fleet_cost"`

	// 等效单节点管理成本（元/月）
	StandaloneCost float64 `json:"standalone_cost"`

	// 成本差异（元/月）
	CostDifference float64 `json:"cost_difference"`

	// 节省率（%）
	SavingRate float64 `json:"saving_rate"`

	// ROI（%）
	ROI float64 `json:"roi"`

	// 推荐
	Recommendation string `json:"recommendation"`
}

// FleetCostBenefitCurve Fleet成本效益曲线
type FleetCostBenefitCurve struct {
	// 数据点
	DataPoints []FleetCostCurvePoint `json:"data_points"`

	// 最优节点数
	OptimalNodeCount int `json:"optimal_node_count"`

	// 最优成本（元/月）
	OptimalCost float64 `json:"optimal_cost"`

	// 成本效益拐点
	BreakpointNodeCount int `json:"breakpoint_node_count"`
}

// FleetCostCurvePoint Fleet成本曲线数据点
type FleetCostCurvePoint struct {
	// 节点数
	NodeCount int `json:"node_count"`

	// Fleet总成本（元/月）
	FleetCost float64 `json:"fleet_cost"`

	// 单节点平均成本（元/月）
	CostPerNode float64 `json:"cost_per_node"`

	// 单TB成本（元/月）
	CostPerTB float64 `json:"cost_per_tb"`

	// 效率评分
	EfficiencyScore float64 `json:"efficiency_score"`
}

// ========== Fleet成本计算器 ==========

// FleetCostCalculator Fleet成本计算器
type FleetCostCalculator struct {
	config FleetCostConfig
}

// NewFleetCostCalculator 创建Fleet成本计算器
func NewFleetCostCalculator(config FleetCostConfig) *FleetCostCalculator {
	return &FleetCostCalculator{config: config}
}

// Analyze 执行Fleet成本分析
func (c *FleetCostCalculator) Analyze() *FleetCostAnalysis {
	now := time.Now()
	analysis := &FleetCostAnalysis{
		ID:            fmt.Sprintf("fleet_cost_analysis_%d", now.Unix()),
		AnalysisTime:  now,
		Config:        c.config,
		CostBreakdown: make(map[string]float64),
		CostOptimizations: make([]FleetCostOptimization, 0),
		Risks:         make([]string, 0),
	}

	// ========== 计算直接成本 ==========

	// 基础节点成本
	analysis.BaseNodesCost = c.config.BaseNodeCostMonthly * float64(c.config.NodeCount)
	analysis.CostBreakdown["base_nodes"] = analysis.BaseNodesCost

	// Fleet软件成本
	analysis.FleetSoftwareCost = c.config.FleetSoftwareCostMonthly
	analysis.CostBreakdown["fleet_software"] = analysis.FleetSoftwareCost

	// 网络互联成本
	analysis.NetworkCost = c.config.NetworkInterconnectCost
	analysis.CostBreakdown["network"] = analysis.NetworkCost

	// 运维人力成本（根据自动化程度调整）
	analysis.OpsStaffCost = c.config.OpsStaffCostMonthly * (1 - c.config.AutomationFactor)
	analysis.CostBreakdown["ops_staff"] = analysis.OpsStaffCost

	// 监控系统成本
	analysis.MonitoringCost = c.config.MonitoringCostMonthly
	analysis.CostBreakdown["monitoring"] = analysis.MonitoringCost

	// 备份同步成本
	analysis.BackupSyncCost = c.config.BackupSyncCostMonthly
	analysis.CostBreakdown["backup_sync"] = analysis.BackupSyncCost

	// 带宽成本
	analysis.BandwidthCost = c.config.AvgNodeBandwidthCost * float64(c.config.NodeCount)
	analysis.CostBreakdown["bandwidth"] = analysis.BandwidthCost

	// 总直接成本
	analysis.TotalDirectCost = analysis.BaseNodesCost + analysis.FleetSoftwareCost +
		analysis.NetworkCost + analysis.OpsStaffCost + analysis.MonitoringCost +
		analysis.BackupSyncCost + analysis.BandwidthCost

	// ========== 计算间接成本 ==========

	// 协调开销成本（节点越多协调开销越大）
	analysis.CoordinationCost = analysis.TotalDirectCost * c.config.CoordinationCostFactor * math.Log2(float64(c.config.NodeCount+1))
	analysis.CostBreakdown["coordination"] = analysis.CoordinationCost

	// HA冗余成本
	analysis.HARedundancyCost = c.config.BaseNodeCostMonthly * float64(c.config.NodeCount) * c.config.HARedundancyFactor
	analysis.CostBreakdown["ha_redundancy"] = analysis.HARedundancyCost

	// 培训成本摊销（假设一次性培训成本5000元，摊销12个月）
	analysis.TrainingCostMonthly = 5000.0 / 12.0 * float64(c.config.NodeCount) / 5.0 // 按节点数分摊
	analysis.CostBreakdown["training"] = analysis.TrainingCostMonthly

	// 故障恢复成本预估（假设每月1%概率故障，恢复成本500元）
	analysis.FailureRecoveryCost = float64(c.config.NodeCount) * 0.01 * 500.0
	analysis.CostBreakdown["failure_recovery"] = analysis.FailureRecoveryCost

	// 安全合规成本（假设每节点50元/月）
	analysis.SecurityComplianceCost = float64(c.config.NodeCount) * 50.0
	analysis.CostBreakdown["security_compliance"] = analysis.SecurityComplianceCost

	// 总间接成本
	analysis.TotalIndirectCost = analysis.CoordinationCost + analysis.HARedundancyCost +
		analysis.TrainingCostMonthly + analysis.FailureRecoveryCost + analysis.SecurityComplianceCost

	// ========== 计算总成本 ==========

	analysis.TotalMonthlyCost = analysis.TotalDirectCost + analysis.TotalIndirectCost
	analysis.TotalYearlyCost = analysis.TotalMonthlyCost * 12

	// 单节点平均成本
	analysis.AvgCostPerNode = analysis.TotalMonthlyCost / float64(c.config.NodeCount)

	// 单TB成本
	totalTB := c.config.AvgNodeStorageTB * float64(c.config.NodeCount)
	analysis.CostPerTB = analysis.TotalMonthlyCost / totalTB

	// ========== 计算节省潜力 ==========

	// 自动化节省（假设100%手动运维的成本）
	fullManualOpsCost := c.config.OpsStaffCostMonthly
	analysis.AutomationSavings = fullManualOpsCost - analysis.OpsStaffCost

	// 集群效率节省（资源共享）
	analysis.EfficiencySavings = analysis.BaseNodesCost * 0.1 // 假设10%效率提升

	// 共享资源节省（备份、监控共享）
	standaloneBackupCost := 300.0 * float64(c.config.NodeCount) // 单节点备份300元
	standaloneMonitoringCost := 50.0 * float64(c.config.NodeCount) // 单节点监控50元
	analysis.SharedResourceSavings = (standaloneBackupCost - analysis.BackupSyncCost) +
		(standaloneMonitoringCost - analysis.MonitoringCost)

	// 总节省潜力
	analysis.TotalSavingsPotential = analysis.AutomationSavings + analysis.EfficiencySavings + analysis.SharedResourceSavings

	// ========== 对比单节点独立管理 ==========

	// 单节点独立管理总成本
	standaloneBaseCost := c.config.BaseNodeCostMonthly * float64(c.config.NodeCount)
	standaloneOpsCost := c.config.OpsStaffCostMonthly // 每节点需要独立运维
	standaloneBackupCost := 300.0 * float64(c.config.NodeCount)
	standaloneMonitoringCost := 50.0 * float64(c.config.NodeCount)
	standaloneNetworkCost := c.config.AvgNodeBandwidthCost * float64(c.config.NodeCount)

	analysis.StandaloneTotalCost = standaloneBaseCost + standaloneOpsCost + standaloneBackupCost +
		standaloneMonitoringCost + standaloneNetworkCost

	// Fleet成本差异
	analysis.FleetCostDifference = analysis.StandaloneTotalCost - analysis.TotalMonthlyCost

	// Fleet是否更划算
	analysis.FleetMoreCostEffective = analysis.FleetCostDifference > 0

	// ROI
	if analysis.TotalMonthlyCost > 0 {
		analysis.FleetROI = (analysis.FleetCostDifference / analysis.TotalMonthlyCost) * 100
	}

	// 成本效益评分
	analysis.CostBenefitScore = c.calculateCostBenefitScore(analysis)

	// ========== 生成优化建议 ==========

	analysis.CostOptimizations = c.generateOptimizations(analysis)

	// ========== 风险提示 ==========

	analysis.Risks = c.generateRisks(analysis)

	return analysis
}

// AnalyzeScenario 分析特定场景
func (c *FleetCostCalculator) AnalyzeScenario(nodeCount int) *FleetScenarioAnalysis {
	// 临时调整节点数
	originalCount := c.config.NodeCount
	c.config.NodeCount = nodeCount

	analysis := c.Analyze()

	// 恢复原始配置
	c.config.NodeCount = originalCount

	result := &FleetScenarioAnalysis{
		Scenario:        fmt.Sprintf("%d节点Fleet", nodeCount),
		NodeCount:       nodeCount,
		FleetCost:       fleetCostRound(analysis.TotalMonthlyCost, 2),
		StandaloneCost:  fleetCostRound(analysis.StandaloneTotalCost, 2),
		CostDifference:  fleetCostRound(analysis.FleetCostDifference, 2),
		SavingRate:      fleetCostRound(analysis.FleetROI, 2),
		ROI:             fleetCostRound(analysis.FleetROI, 2),
		Recommendation:  "待评估",
	}

	// 根据ROI判断推荐状态
	if result.ROI > 30 {
		result.Recommendation = "强烈推荐Fleet管理"
	} else if result.ROI > 15 {
		result.Recommendation = "推荐Fleet管理"
	} else if result.ROI > 0 {
		result.Recommendation = "Fleet管理略优"
	} else if result.ROI > -10 {
		result.Recommendation = "成本持平，按需求选择"
	} else {
		result.Recommendation = "单节点管理更划算"
	}

	return result
}

// AnalyzeCostCurve 分析成本效益曲线
func (c *FleetCostCalculator) AnalyzeCostCurve() *FleetCostBenefitCurve {
	curve := &FleetCostBenefitCurve{
		DataPoints: make([]FleetCostCurvePoint, 0),
	}

	// 分析不同节点数下的成本
	nodeCounts := []int{1, 2, 3, 5, 10, 20, 50, 100}

	minCostPerNode := 999999.0
	optimalNodeCount := 1
	minCost := 999999.0

	for _, nodeCount := range nodeCounts {
		c.config.NodeCount = nodeCount
		analysis := c.Analyze()

		point := FleetCostCurvePoint{
			NodeCount:      nodeCount,
			FleetCost:      analysis.TotalMonthlyCost,
			CostPerNode:    analysis.AvgCostPerNode,
			CostPerTB:      analysis.CostPerTB,
			EfficiencyScore: analysis.CostBenefitScore,
		}

		curve.DataPoints = append(curve.DataPoints, point)

		// 找最优节点数（成本效益评分最高）
		if point.CostPerNode < minCostPerNode && nodeCount > 1 {
			minCostPerNode = point.CostPerNode
			optimalNodeCount = nodeCount
			minCost = point.FleetCost
		}
	}

	// 恢复原始配置
	c.config.NodeCount = curve.DataPoints[0].NodeCount

	curve.OptimalNodeCount = optimalNodeCount
	curve.OptimalCost = minCost

	// 计算成本效益拐点（Fleet开始划算的节点数）
	curve.BreakpointNodeCount = c.findBreakpoint(curve)

	return curve
}

// ========== 私有方法 ==========

// calculateCostBenefitScore 计算成本效益评分
func (c *FleetCostCalculator) calculateCostBenefitScore(analysis *FleetCostAnalysis) float64 {
	score := 50.0 // 基础分

	// ROI贡献（最高30分）
	if analysis.FleetROI > 50 {
		score += 30
	} else if analysis.FleetROI > 30 {
		score += 25
	} else if analysis.FleetROI > 20 {
		score += 20
	} else if analysis.FleetROI > 10 {
		score += 15
	} else if analysis.FleetROI > 0 {
		score += 10
	} else if analysis.FleetROI < -20 {
		score -= 20
	}

	// 自动化程度贡献（最高15分）
	score += c.config.AutomationFactor * 15

	// 单节点成本效率（最高10分）
	// 对比行业平均水平（假设600元/月）
 avgIndustryCost := 600.0
	if analysis.AvgCostPerNode < avgIndustryCost*0.7 {
		score += 10
	} else if analysis.AvgCostPerNode < avgIndustryCost*0.9 {
		score += 7
	} else if analysis.AvgCostPerNode < avgIndustryCost {
		score += 5
	}

	// 单TB成本效率（最高5分）
	avgIndustryCostPerTB := 30.0
	if analysis.CostPerTB < avgIndustryCostPerTB*0.5 {
		score += 5
	} else if analysis.CostPerTB < avgIndustryCostPerTB*0.7 {
		score += 3
	}

	return fleetCostRound(math.Max(0, math.Min(100, score)), 1)
}

// generateOptimizations 生成优化建议
func (c *FleetCostCalculator) generateOptimizations(analysis *FleetCostAnalysis) []FleetCostOptimization {
	opts := make([]FleetCostOptimization, 0)
	idCounter := 1

	// 自动化优化
	if c.config.AutomationFactor < 0.8 {
		savings := (1 - c.config.AutomationFactor) * 0.3 * c.config.OpsStaffCostMonthly
		opts = append(opts, FleetCostOptimization{
			ID:            fmt.Sprintf("opt_%d", idCounter),
			Type:          "automation",
			Description:   "提高自动化运维程度，减少人工干预",
			CurrentCost:   analysis.OpsStaffCost,
			OptimizedCost: analysis.OpsStaffCost * 0.7,
			Savings:       fleetCostRound(savings, 2),
			Difficulty:    "medium",
			EstimatedWeeks: 4,
			ROIScore:      savings * 10,
		})
		idCounter++
	}

	// 监控系统优化
	if analysis.MonitoringCost > 150 {
		opts = append(opts, FleetCostOptimization{
			ID:            fmt.Sprintf("opt_%d", idCounter),
			Type:          "monitoring",
			Description:   "优化监控系统，使用开源方案降低成本",
			CurrentCost:   analysis.MonitoringCost,
			OptimizedCost: analysis.MonitoringCost * 0.6,
			Savings:       fleetCostRound(analysis.MonitoringCost*0.4, 2),
			Difficulty:    "easy",
			EstimatedWeeks: 2,
			ROIScore:      analysis.MonitoringCost * 4,
		})
		idCounter++
	}

	// 资源整合优化
	if c.config.NodeCount > 10 {
		savings := analysis.BaseNodesCost * 0.15 // 大Fleet可整合15%
		opts = append(opts, FleetCostOptimization{
			ID:            fmt.Sprintf("opt_%d", idCounter),
			Type:          "consolidation",
			Description:   "整合低利用率节点，提高资源效率",
			CurrentCost:   analysis.BaseNodesCost,
			OptimizedCost: analysis.BaseNodesCost * 0.85,
			Savings:       fleetCostRound(savings, 2),
			Difficulty:    "hard",
			EstimatedWeeks: 8,
			ROIScore:      savings * 5,
		})
		idCounter++
	}

	// 网络优化
	if analysis.NetworkCost > 200 {
		opts = append(opts, FleetCostOptimization{
			ID:            fmt.Sprintf("opt_%d", idCounter),
			Type:          "network",
			Description:   "优化网络架构，减少专线依赖",
			CurrentCost:   analysis.NetworkCost,
			OptimizedCost: analysis.NetworkCost * 0.7,
			Savings:       fleetCostRound(analysis.NetworkCost*0.3, 2),
			Difficulty:    "medium",
			EstimatedWeeks: 4,
			ROIScore:      analysis.NetworkCost * 3,
		})
		idCounter++
	}

	// 备份优化
	if analysis.BackupSyncCost > 300 {
		opts = append(opts, FleetCostOptimization{
			ID:            fmt.Sprintf("opt_%d", idCounter),
			Type:          "backup",
			Description:   "优化备份策略，使用增量备份减少带宽",
			CurrentCost:   analysis.BackupSyncCost,
			OptimizedCost: analysis.BackupSyncCost * 0.6,
			Savings:       fleetCostRound(analysis.BackupSyncCost*0.4, 2),
			Difficulty:    "easy",
			EstimatedWeeks: 2,
			ROIScore:      analysis.BackupSyncCost * 4,
		})
		idCounter++
	}

	// 按ROI评分排序
	for i := 0; i < len(opts) && i < 5; i++ {
		for j := i + 1; j < len(opts); j++ {
			if opts[j].ROIScore > opts[i].ROIScore {
				opts[i], opts[j] = opts[j], opts[i]
			}
		}
	}

	// 最多返回5个建议
	if len(opts) > 5 {
		opts = opts[:5]
	}

	return opts
}

// generateRisks 生成风险提示
func (c *FleetCostCalculator) generateRisks(analysis *FleetCostAnalysis) []string {
	risks := make([]string, 0)

	// 成本风险
	if analysis.FleetROI < 0 {
		risks = append(risks,
			fmt.Sprintf("Fleet管理成本比单节点独立管理高 %.2f 元/月", -analysis.FleetCostDifference))
	}

	// 协调风险
	if analysis.CoordinationCost > analysis.TotalMonthlyCost*0.1 {
		risks = append(risks,
			fmt.Sprintf("协调开销占总成本 %.2f%%，节点数过多", analysis.CoordinationCost/analysis.TotalMonthlyCost*100))
	}

	// 单点故障风险
	risks = append(risks, "Fleet管理器故障会影响整个集群调度")

	// 网络依赖风险
	risks = append(risks, "网络中断会导致跨节点协调失败")

	// 复杂度风险
	if c.config.AutomationFactor < 0.5 {
		risks = append(risks, "低自动化程度增加运维复杂度和出错风险")
	}

	// 成本增长风险
	risks = append(risks, "节点增加时成本非线性增长，需关注拐点")

	return risks
}

// findBreakpoint 找成本效益拐点
func (c *FleetCostCalculator) findBreakpoint(curve *FleetCostBenefitCurve) int {
	for _, point := range curve.DataPoints {
		// Fleet成本效益开始优于单节点的节点数
		standaloneCost := c.config.BaseNodeCostMonthly * float64(point.NodeCount) +
			c.config.OpsStaffCostMonthly + 300.0*float64(point.NodeCount) +
			50.0*float64(point.NodeCount) + c.config.AvgNodeBandwidthCost*float64(point.NodeCount)

		if point.FleetCost < standaloneCost && point.NodeCount > 1 {
			return point.NodeCount
		}
	}
	return 3 // 默认拐点3节点
}

// ========== 工具方法 ==========

// QuickFleetCostCheck 快速Fleet成本检查
func QuickFleetCostCheck(nodeCount int, avgStorageTB float64) string {
	config := DefaultFleetCostConfig()
	config.NodeCount = nodeCount
	config.AvgNodeStorageTB = avgStorageTB

	calc := NewFleetCostCalculator(config)
	analysis := calc.Analyze()

	return fmt.Sprintf("%d节点Fleet: 月成本 %.2f 元，单节点 %.2f 元，ROI %.2f%%",
		nodeCount, analysis.TotalMonthlyCost, analysis.AvgCostPerNode, analysis.FleetROI)
}

// CompareFleetVsStandalone 对比Fleet与单节点管理
func CompareFleetVsStandalone(nodeCount int) string {
	config := DefaultFleetCostConfig()
	config.NodeCount = nodeCount

	calc := NewFleetCostCalculator(config)
	analysis := calc.Analyze()

	result := fmt.Sprintf(`
 Fleet管理 | 单节点管理 | 差异
月成本 | %.2f | %.2f | %.2f
单节点成本 | %.2f | %.2f | -
ROI | %.2f%% | - | -
推荐 | %s | - | -
`, analysis.TotalMonthlyCost, analysis.StandaloneTotalCost, analysis.FleetCostDifference,
		analysis.AvgCostPerNode, analysis.StandaloneTotalCost/float64(nodeCount),
		analysis.FleetROI, fleetCostBoolToStr(analysis.FleetMoreCostEffective))

	return result
}

// EstimateFleetCost 估算Fleet成本
func EstimateFleetCost(nodeCount int, avgStorageTB float64) float64 {
	config := DefaultFleetCostConfig()
	config.NodeCount = nodeCount
	config.AvgNodeStorageTB = avgStorageTB

	calc := NewFleetCostCalculator(config)
	analysis := calc.Analyze()

	return analysis.TotalMonthlyCost
}

// GenerateFleetCostReport 生成Fleet成本报告
func GenerateFleetCostReport(config FleetCostConfig) string {
	calc := NewFleetCostCalculator(config)
	analysis := calc.Analyze()

	report := fmt.Sprintf(`
# Fleet管理成本评估报告

## 配置参数
- 节点数量: %d
- 单节点存储: %.1f TB
- 自动化程度: %.2f
- HA冗余系数: %.2f

## 直接成本（元/月）
| 项目 | 成本 |
|------|------|
| 基础节点成本 | %.2f |
| Fleet软件成本 | %.2f |
| 网络互联成本 | %.2f |
| 运维人力成本 | %.2f |
| 监控系统成本 | %.2f |
| 备份同步成本 | %.2f |
| 带宽成本 | %.2f |
| **总直接成本** | **%.2f** |

## 间接成本（元/月）
| 项目 | 成本 |
|------|------|
| 协调开销 | %.2f |
| HA冗余成本 | %.2f |
| 培训摊销 | %.2f |
| 故障恢复预估 | %.2f |
| 安全合规成本 | %.2f |
| **总间接成本** | **%.2f** |

## 总成本汇总
- 月度总成本: %.2f 元
- 年度总成本: %.2f 元
- 单节点平均成本: %.2f 元/月
- 单TB成本: %.2f 元/月

## 成本对比分析
- 单节点独立管理成本: %.2f 元/月
- Fleet成本差异: %.2f 元/月
- Fleet ROI: %.2f%%
- Fleet是否更划算: %s

## 节省潜力分析
| 项目 | 节省（元/月） |
|------|-------------|
| 自动化节省 | %.2f |
| 集群效率节省 | %.2f |
| 共享资源节省 | %.2f |
| **总节省潜力** | **%.2f** |

## 成本效益评分
%.1f/100

## 优化建议
%s

## 风险提示
%s
`,
		config.NodeCount,
		config.AvgNodeStorageTB,
		config.AutomationFactor,
		config.HARedundancyFactor,
		analysis.BaseNodesCost,
		analysis.FleetSoftwareCost,
		analysis.NetworkCost,
		analysis.OpsStaffCost,
		analysis.MonitoringCost,
		analysis.BackupSyncCost,
		analysis.BandwidthCost,
		analysis.TotalDirectCost,
		analysis.CoordinationCost,
		analysis.HARedundancyCost,
		analysis.TrainingCostMonthly,
		analysis.FailureRecoveryCost,
		analysis.SecurityComplianceCost,
		analysis.TotalIndirectCost,
		analysis.TotalMonthlyCost,
		analysis.TotalYearlyCost,
		analysis.AvgCostPerNode,
		analysis.CostPerTB,
		analysis.StandaloneTotalCost,
		analysis.FleetCostDifference,
		analysis.FleetROI,
		fleetCostBoolToStr(analysis.FleetMoreCostEffective),
		analysis.AutomationSavings,
		analysis.EfficiencySavings,
		analysis.SharedResourceSavings,
		analysis.TotalSavingsPotential,
		analysis.CostBenefitScore,
		fleetCostFormatOptimizations(analysis.CostOptimizations),
		fleetCostJoinList(analysis.Risks),
	)

	return report
}

// ========== 辅助函数 ==========

func fleetCostRound(val float64, precision int) float64 {
	factor := math.Pow10(precision)
	return math.Round(val * factor) / factor
}

func fleetCostBoolToStr(b bool) string {
	if b {
		return "✅ 是"
	}
	return "❌ 否"
}

func fleetCostJoinList(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += "\n"
		}
		result += "- " + item
	}
	return result
}

func fleetCostFormatOptimizations(opts []FleetCostOptimization) string {
	result := ""
	for i, opt := range opts {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("%d. %s: 节省 %.2f 元/月（难度: %s，周期: %d周）",
			i+1, opt.Description, opt.Savings, opt.Difficulty, opt.EstimatedWeeks)
	}
	return result
}