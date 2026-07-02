// Package cost - RAIDZ扩容成本计算器
// 提供ZFS RAIDZ级别容量计算和扩容成本分析
package cost

import (
	"fmt"
	"math"
)

// ========== RAIDZ类型定义 ==========

// RAIDZLevel RAIDZ级别.
type RAIDZLevel string

const (
	// RAIDZ1 单盘冗余（类似RAID5）.
	RAIDZ1 RAIDZLevel = "raidz1"
	// RAIDZ2 双盘冗余（类似RAID6）.
	RAIDZ2 RAIDZLevel = "raidz2"
	// RAIDZ3 三盘冗余.
	RAIDZ3 RAIDZLevel = "raidz3"
	// Stripe 无冗余（条带化）.
	Stripe RAIDZLevel = "stripe"
	// Mirror 镜像（类似RAID1）.
	Mirror RAIDZLevel = "mirror"
)

// RAIDZConfig RAIDZ配置.
type RAIDZConfig struct {
	// RAIDZ级别
	Level RAIDZLevel `json:"level"`

	// 磁盘数量
	DiskCount int `json:"disk_count"`

	// 单盘容量（字节）
	DiskCapacityBytes uint64 `json:"disk_capacity_bytes"`

	// 磁盘单价（元）
	DiskPrice float64 `json:"disk_price"`
}

// RAIDZCapacityResult 容量计算结果.
type RAIDZCapacityResult struct {
	// 配置信息
	Config RAIDZConfig `json:"config"`

	// 原始总容量（所有磁盘总和）
	RawCapacityBytes uint64 `json:"raw_capacity_bytes"`

	// 可用容量（字节）
	UsableCapacityBytes uint64 `json:"usable_capacity_bytes"`

	// 可用容量（GB）
	UsableCapacityGB float64 `json:"usable_capacity_gb"`

	// 可用容量（TB）
	UsableCapacityTB float64 `json:"usable_capacity_tb"`

	// 冗余盘数
	ParityDisks int `json:"parity_disks"`

	// 空间利用率（%）
	SpaceUtilization float64 `json:"space_utilization"`

	// 存储效率评分（0-100）
	EfficiencyScore float64 `json:"efficiency_score"`

	// 磁盘总成本
	TotalDiskCost float64 `json:"total_disk_cost"`

	// 单位成本（元/GB）
	CostPerGB float64 `json:"cost_per_gb"`

	// 单位成本（元/TB/月）
	CostPerTB float64 `json:"cost_per_tb"`
}

// ExpansionPlan 扩容方案.
type ExpansionPlan struct {
	// 方案ID
	ID string `json:"id"`

	// 当前配置
	CurrentConfig RAIDZConfig `json:"current_config"`

	// 目标配置
	TargetConfig RAIDZConfig `json:"target_config"`

	// 新增磁盘数量
	NewDiskCount int `json:"new_disk_count"`

	// 当前可用容量（GB）
	CurrentCapacityGB float64 `json:"current_capacity_gb"`

	// 扩容后可用容量（GB）
	TargetCapacityGB float64 `json:"target_capacity_gb"`

	// 容量增量（GB）
	CapacityIncreaseGB float64 `json:"capacity_increase_gb"`

	// 容量增长率（%）
	CapacityGrowthPercent float64 `json:"capacity_growth_percent"`

	// 扩容成本（元）
	ExpansionCost float64 `json:"expansion_cost"`

	// 单位扩容成本（元/GB）
	CostPerGBAdded float64 `json:"cost_per_gb_added"`

	// ROI评分（0-100）
	ROIScore float64 `json:"roi_score"`

	// 建议
	Recommendation string `json:"recommendation"`

	// 风险提示
	Warnings []string `json:"warnings"`

	// 方案优先级（1-5，1最高）
	Priority int `json:"priority"`
}

// ExpansionAnalysis 扩容分析结果.
type ExpansionAnalysis struct {
	// 分析ID
	ID string `json:"id"`

	// 当前配置
	Current RAIDZCapacityResult `json:"current"`

	// 目标容量（GB）
	TargetCapacityGB float64 `json:"target_capacity_gb"`

	// 所有可行方案
	Plans []ExpansionPlan `json:"plans"`

	// 最优方案
	BestPlan *ExpansionPlan `json:"best_plan"`

	// 成本效益对比
	CostComparison map[RAIDZLevel]float64 `json:"cost_comparison"`

	// 建议
	Suggestions []string `json:"suggestions"`
}

// RAIDZCalculator RAIDZ计算器.
type RAIDZCalculator struct {
	// 存储单价（元/GB/月）
	StorageCostPerGB float64 `json:"storage_cost_per_gb"`

	// 磁盘均价（元/TB）
	DiskCostPerTB float64 `json:"disk_cost_per_tb"`

	// ZFS元数据开销比例（约1-2%）
	ZFSMetadataOverhead float64 `json:"zfs_metadata_overhead"`

	// 推荐最小盘数
	MinDiskRecommendations map[RAIDZLevel]int `json:"min_disk_recommendations"`
}

// DefaultRAIDZCalculator 默认计算器配置.
func DefaultRAIDZCalculator() *RAIDZCalculator {
	return &RAIDZCalculator{
		StorageCostPerGB:    0.05, // 0.05元/GB/月
		DiskCostPerTB:       300,  // 300元/TB
		ZFSMetadataOverhead: 0.02, // 2%元数据开销
		MinDiskRecommendations: map[RAIDZLevel]int{
			RAIDZ1: 3, // RAIDZ1最少3盘
			RAIDZ2: 4, // RAIDZ2最少4盘
			RAIDZ3: 5, // RAIDZ3最少5盘
			Stripe: 1, // 条带最少1盘
			Mirror: 2, // 镜像最少2盘
		},
	}
}

// ========== 容量计算核心逻辑 ==========

// CalculateCapacity 计算RAIDZ配置的可用容量.
func (c *RAIDZCalculator) CalculateCapacity(config RAIDZConfig) *RAIDZCapacityResult {
	result := &RAIDZCapacityResult{
		Config: config,
	}

	// 原始总容量
	result.RawCapacityBytes = config.DiskCapacityBytes * uint64(config.DiskCount)

	// 计算冗余盘数
	result.ParityDisks = c.getParityDisks(config.Level)

	// 计算可用容量
	usableDiskCount := config.DiskCount - result.ParityDisks
	if usableDiskCount < 0 {
		usableDiskCount = 0
	}

	// 基础可用容量 = (总盘数 - 冗余盘数) × 单盘容量
	baseUsableBytes := uint64(usableDiskCount) * config.DiskCapacityBytes

	// 应用ZFS元数据开销
	result.UsableCapacityBytes = uint64(float64(baseUsableBytes) * (1 - c.ZFSMetadataOverhead))

	// 转换为GB/TB
	result.UsableCapacityGB = float64(result.UsableCapacityBytes) / (1024 * 1024 * 1024)
	result.UsableCapacityTB = result.UsableCapacityGB / 1024

	// 计算空间利用率
	if result.RawCapacityBytes > 0 {
		result.SpaceUtilization = round(float64(result.UsableCapacityBytes)/float64(result.RawCapacityBytes)*100, 2)
	}

	// 计算效率评分
	result.EfficiencyScore = c.calculateEfficiencyScore(config, result.SpaceUtilization)

	// 计算成本
	result.TotalDiskCost = float64(config.DiskCount) * config.DiskPrice
	if result.UsableCapacityGB > 0 {
		result.CostPerGB = round(result.TotalDiskCost/result.UsableCapacityGB, 2)
		result.CostPerTB = round(result.TotalDiskCost/result.UsableCapacityTB, 2)
	}

	return result
}

// getParityDisks 获取冗余盘数.
func (c *RAIDZCalculator) getParityDisks(level RAIDZLevel) int {
	switch level {
	case RAIDZ1:
		return 1
	case RAIDZ2:
		return 2
	case RAIDZ3:
		return 3
	case Stripe:
		return 0
	case Mirror:
		// 镜像: 实际容量为单盘容量，冗余为盘数/2
		return 0 // 特殊处理
	default:
		return 0
	}
}

// CalculateMirrorCapacity 计算镜像容量（特殊处理）.
func (c *RAIDZCalculator) CalculateMirrorCapacity(config RAIDZConfig) *RAIDZCapacityResult {
	result := &RAIDZCapacityResult{
		Config:      config,
		ParityDisks: 0,
	}

	// 镜像可用容量 = 盘数/2 × 单盘容量（假设N-way mirror）
	mirrorGroups := config.DiskCount / 2
	result.RawCapacityBytes = config.DiskCapacityBytes * uint64(config.DiskCount)
	result.UsableCapacityBytes = uint64(mirrorGroups) * config.DiskCapacityBytes

	// 应用ZFS元数据开销
	result.UsableCapacityBytes = uint64(float64(result.UsableCapacityBytes) * (1 - c.ZFSMetadataOverhead))

	result.UsableCapacityGB = float64(result.UsableCapacityBytes) / (1024 * 1024 * 1024)
	result.UsableCapacityTB = result.UsableCapacityGB / 1024

	if result.RawCapacityBytes > 0 {
		result.SpaceUtilization = round(float64(result.UsableCapacityBytes)/float64(result.RawCapacityBytes)*100, 2)
	}

	result.EfficiencyScore = c.calculateEfficiencyScore(config, result.SpaceUtilization)
	result.TotalDiskCost = float64(config.DiskCount) * config.DiskPrice

	if result.UsableCapacityGB > 0 {
		result.CostPerGB = round(result.TotalDiskCost/result.UsableCapacityGB, 2)
		result.CostPerTB = round(result.TotalDiskCost/result.UsableCapacityTB, 2)
	}

	return result
}

// calculateEfficiencyScore 计算效率评分.
func (c *RAIDZCalculator) calculateEfficiencyScore(config RAIDZConfig, utilization float64) float64 {
	score := 100.0

	// 盘数建议检查
	minDisks := c.MinDiskRecommendations[config.Level]
	if config.DiskCount < minDisks {
		// 盘数不足扣分
		score -= float64(minDisks-config.DiskCount) * 15
	}

	// 空间利用率评分
	// RAIDZ1: 最少3盘，利用率约67%，最优约80-90%
	// RAIDZ2: 最少4盘，利用率约50%，最优约70-85%
	// RAIDZ3: 最少5盘，利用率约60%，最优约70-80%

	optimalUtilization := c.getOptimalUtilization(config.Level)
	deviation := math.Abs(utilization - optimalUtilization)
	score -= deviation * 0.5

	// 成本效益评分
	// 盘数越多，单位成本越低（对于同级别RAIDZ）
	if config.DiskCount >= 8 {
		score += 10 // 大规模阵列加分
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return round(score, 1)
}

// getOptimalUtilization 获取最优利用率参考值.
func (c *RAIDZCalculator) getOptimalUtilization(level RAIDZLevel) float64 {
	switch level {
	case RAIDZ1:
		return 85.0 // RAIDZ1最优利用率约85%
	case RAIDZ2:
		return 75.0 // RAIDZ2最优利用率约75%
	case RAIDZ3:
		return 70.0 // RAIDZ3最优利用率约70%
	case Stripe:
		return 98.0 // 条带化98%
	case Mirror:
		return 50.0 // 镜像50%
	default:
		return 80.0
	}
}

// ========== 扩容方案计算 ==========

// AnalyzeExpansion 分析扩容方案.
func (c *RAIDZCalculator) AnalyzeExpansion(currentConfig RAIDZConfig, targetCapacityGB float64) *ExpansionAnalysis {
	analysis := &ExpansionAnalysis{
		ID:               fmt.Sprintf("expansion_%d", currentConfig.DiskCount),
		Current:          *c.CalculateCapacity(currentConfig),
		TargetCapacityGB: targetCapacityGB,
		Plans:            make([]ExpansionPlan, 0),
		CostComparison:   make(map[RAIDZLevel]float64),
		Suggestions:      make([]string, 0),
	}

	// 当前容量
	currentCapacityGB := analysis.Current.UsableCapacityGB

	// 检查是否已达到目标
	if currentCapacityGB >= targetCapacityGB {
		analysis.Suggestions = append(analysis.Suggestions,
			"当前容量已满足目标，无需扩容")
		return analysis
	}

	// 生成扩容方案
	// 方案1: 同级别增加磁盘
	plan1 := c.generateSameLevelPlan(currentConfig, targetCapacityGB)
	if plan1 != nil {
		analysis.Plans = append(analysis.Plans, *plan1)
	}

	// 方案2: 升级到更高级别RAIDZ（如果盘数足够）
	plan2 := c.generateUpgradePlan(currentConfig, targetCapacityGB)
	if plan2 != nil {
		analysis.Plans = append(analysis.Plans, *plan2)
	}

	// 方案3: 使用更大容量磁盘重建
	plan3 := c.generateReplaceDiskPlan(currentConfig, targetCapacityGB)
	if plan3 != nil {
		analysis.Plans = append(analysis.Plans, *plan3)
	}

	// 选择最优方案
	analysis.BestPlan = c.selectBestPlan(analysis.Plans)

	// 成本对比
	for _, plan := range analysis.Plans {
		level := plan.TargetConfig.Level
		if _, exists := analysis.CostComparison[level]; !exists || plan.CostPerGBAdded < analysis.CostComparison[level] {
			analysis.CostComparison[level] = plan.CostPerGBAdded
		}
	}

	// 生成建议
	analysis.Suggestions = c.generateExpansionSuggestions(analysis)

	return analysis
}

// generateSameLevelPlan 生成同级别扩容方案.
func (c *RAIDZCalculator) generateSameLevelPlan(currentConfig RAIDZConfig, targetCapacityGB float64) *ExpansionPlan {
	currentResult := c.CalculateCapacity(currentConfig)
	currentCapacityGB := currentResult.UsableCapacityGB

	// 计算需要达到目标的盘数
	parity := c.getParityDisks(currentConfig.Level)
	if currentConfig.Level == Mirror {
		// 镜像特殊处理
		parity = currentConfig.DiskCount / 2 // 镜像冗余盘数
	}

	// 目标可用容量需要的盘数（考虑元数据开销）
	targetBytes := targetCapacityGB * 1024 * 1024 * 1024
	effectiveDiskCapacity := float64(currentConfig.DiskCapacityBytes) * (1 - c.ZFSMetadataOverhead)

	// 可用盘数 = 目标容量 / 有效单盘容量
	requiredUsableDisks := int(math.Ceil(targetBytes / effectiveDiskCapacity))

	// 实际需要的总盘数
	requiredTotalDisks := requiredUsableDisks + parity

	// 特殊处理镜像
	if currentConfig.Level == Mirror {
		requiredTotalDisks = requiredUsableDisks * 2
	}

	newDiskCount := requiredTotalDisks - currentConfig.DiskCount

	if newDiskCount <= 0 {
		return nil // 无需扩容
	}

	// 创建目标配置
	targetConfig := RAIDZConfig{
		Level:             currentConfig.Level,
		DiskCount:         requiredTotalDisks,
		DiskCapacityBytes: currentConfig.DiskCapacityBytes,
		DiskPrice:         currentConfig.DiskPrice,
	}

	targetResult := c.CalculateCapacity(targetConfig)

	plan := &ExpansionPlan{
		ID:                    fmt.Sprintf("plan_same_%d", newDiskCount),
		CurrentConfig:         currentConfig,
		TargetConfig:          targetConfig,
		NewDiskCount:          newDiskCount,
		CurrentCapacityGB:     currentCapacityGB,
		TargetCapacityGB:      targetResult.UsableCapacityGB,
		CapacityIncreaseGB:    targetResult.UsableCapacityGB - currentCapacityGB,
		CapacityGrowthPercent: round((targetResult.UsableCapacityGB-currentCapacityGB)/currentCapacityGB*100, 2),
		ExpansionCost:         float64(newDiskCount) * currentConfig.DiskPrice,
		CostPerGBAdded:        round(float64(newDiskCount)*currentConfig.DiskPrice/(targetResult.UsableCapacityGB-currentCapacityGB), 2),
		Warnings:              make([]string, 0),
		Priority:              1,
	}

	// 计算ROI评分
	plan.ROIScore = c.calculateROIScore(plan)

	// 生成建议
	plan.Recommendation = fmt.Sprintf("建议增加 %d 块 %dGB 磁盘，扩容成本 %.2f 元",
		newDiskCount,
		currentConfig.DiskCapacityBytes/(1024*1024*1024),
		plan.ExpansionCost)

	// 检查风险
	minDisks := c.MinDiskRecommendations[currentConfig.Level]
	if targetConfig.DiskCount < minDisks+2 {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("扩展后盘数 %d 接近最小建议 %d，建议增加更多磁盘提升可靠性",
				targetConfig.DiskCount, minDisks))
	}

	return plan
}

// generateUpgradePlan 生成升级RAID级别方案.
func (c *RAIDZCalculator) generateUpgradePlan(currentConfig RAIDZConfig, targetCapacityGB float64) *ExpansionPlan {
	// 只有RAIDZ1可以升级到RAIDZ2/3（需要足够盘数）
	if currentConfig.Level != RAIDZ1 {
		return nil
	}

	currentResult := c.CalculateCapacity(currentConfig)
	currentCapacityGB := currentResult.UsableCapacityGB

	// 升级到RAIDZ2需要的最小盘数
	if currentConfig.DiskCount < c.MinDiskRecommendations[RAIDZ2] {
		return nil // 盘数不足
	}

	// 计算RAIDZ2配置
	targetConfig := RAIDZConfig{
		Level:             RAIDZ2,
		DiskCount:         currentConfig.DiskCount,
		DiskCapacityBytes: currentConfig.DiskCapacityBytes,
		DiskPrice:         currentConfig.DiskPrice,
	}

	targetResult := c.CalculateCapacity(targetConfig)

	// RAIDZ2可用容量会减少（多一个冗余盘）
	// 需要额外增加磁盘来达到目标容量

	// 计算需要增加的盘数
	targetBytes := targetCapacityGB * 1024 * 1024 * 1024
	effectiveDiskCapacity := float64(currentConfig.DiskCapacityBytes) * (1 - c.ZFSMetadataOverhead)

	// RAIDZ2: 可用盘数 = 总盘数 - 2
	requiredUsableDisks := int(math.Ceil(targetBytes / effectiveDiskCapacity))
	requiredTotalDisks := requiredUsableDisks + 2

	newDiskCount := requiredTotalDisks - currentConfig.DiskCount
	if newDiskCount < 0 {
		newDiskCount = 0
	}

	// 更新目标配置
	targetConfig.DiskCount = currentConfig.DiskCount + newDiskCount
	targetResult = c.CalculateCapacity(targetConfig)

	plan := &ExpansionPlan{
		ID:                    fmt.Sprintf("plan_upgrade_%s", RAIDZ2),
		CurrentConfig:         currentConfig,
		TargetConfig:          targetConfig,
		NewDiskCount:          newDiskCount,
		CurrentCapacityGB:     currentCapacityGB,
		TargetCapacityGB:      targetResult.UsableCapacityGB,
		CapacityIncreaseGB:    targetResult.UsableCapacityGB - currentCapacityGB,
		CapacityGrowthPercent: round((targetResult.UsableCapacityGB-currentCapacityGB)/currentCapacityGB*100, 2),
		ExpansionCost:         float64(newDiskCount) * currentConfig.DiskPrice,
		CostPerGBAdded:        round(float64(newDiskCount)*currentConfig.DiskPrice/(targetResult.UsableCapacityGB-currentCapacityGB), 2),
		Warnings:              make([]string, 0),
		Priority:              2,
	}

	plan.ROIScore = c.calculateROIScore(plan)

	plan.Recommendation = fmt.Sprintf("升级到RAIDZ2并增加 %d 块磁盘，提升冗余可靠性，成本 %.2f 元",
		newDiskCount, plan.ExpansionCost)

	plan.Warnings = append(plan.Warnings,
		"升级RAID级别需要重建整个阵列，请确保数据已备份")

	return plan
}

// generateReplaceDiskPlan 生成更换更大磁盘方案.
func (c *RAIDZCalculator) generateReplaceDiskPlan(currentConfig RAIDZConfig, targetCapacityGB float64) *ExpansionPlan {
	// 计算需要的单盘容量
	parity := c.getParityDisks(currentConfig.Level)
	usableDiskCount := currentConfig.DiskCount - parity

	if usableDiskCount <= 0 {
		return nil
	}

	// 目标单盘容量（考虑元数据开销）
	targetBytes := targetCapacityGB * 1024 * 1024 * 1024
	targetDiskBytes := uint64(math.Ceil(targetBytes / float64(usableDiskCount) / (1 - c.ZFSMetadataOverhead)))

	// 检查是否有意义
	if targetDiskBytes <= currentConfig.DiskCapacityBytes {
		return nil
	}

	// 计算新磁盘价格（假设线性增长）
	capacityRatio := float64(targetDiskBytes) / float64(currentConfig.DiskCapacityBytes)
	newDiskPrice := currentConfig.DiskPrice * capacityRatio

	targetConfig := RAIDZConfig{
		Level:             currentConfig.Level,
		DiskCount:         currentConfig.DiskCount,
		DiskCapacityBytes: targetDiskBytes,
		DiskPrice:         newDiskPrice,
	}

	currentResult := c.CalculateCapacity(currentConfig)
	targetResult := c.CalculateCapacity(targetConfig)

	plan := &ExpansionPlan{
		ID:                    "plan_replace",
		CurrentConfig:         currentConfig,
		TargetConfig:          targetConfig,
		NewDiskCount:          0, // 不增加盘数
		CurrentCapacityGB:     currentResult.UsableCapacityGB,
		TargetCapacityGB:      targetResult.UsableCapacityGB,
		CapacityIncreaseGB:    targetResult.UsableCapacityGB - currentResult.UsableCapacityGB,
		CapacityGrowthPercent: round((targetResult.UsableCapacityGB-currentResult.UsableCapacityGB)/currentResult.UsableCapacityGB*100, 2),
		ExpansionCost:         float64(currentConfig.DiskCount) * newDiskPrice, // 替换所有磁盘
		CostPerGBAdded:        round(float64(currentConfig.DiskCount)*newDiskPrice/(targetResult.UsableCapacityGB-currentResult.UsableCapacityGB), 2),
		Warnings:              make([]string, 0),
		Priority:              3,
	}

	plan.ROIScore = c.calculateROIScore(plan)

	newDiskGB := float64(targetDiskBytes) / (1024 * 1024 * 1024)
	plan.Recommendation = fmt.Sprintf("替换为 %d 块 %.0fGB 磁盘，容量可达 %.2fTB，成本 %.2f 元",
		currentConfig.DiskCount, newDiskGB, targetResult.UsableCapacityTB, plan.ExpansionCost)

	plan.Warnings = append(plan.Warnings,
		"替换磁盘需要逐盘 resilver，过程较长，请确保有足够维护窗口")

	return plan
}

// calculateROIScore 计算ROI评分.
func (c *RAIDZCalculator) calculateROIScore(plan *ExpansionPlan) float64 {
	score := 100.0

	// 单位成本评分
	if plan.CostPerGBAdded < 1.0 {
		score += 20 // 低成本加分
	} else if plan.CostPerGBAdded > 5.0 {
		score -= 30 // 高成本扣分
	}

	// 容量增长评分
	if plan.CapacityGrowthPercent > 100 {
		score += 15 // 大幅增长加分
	} else if plan.CapacityGrowthPercent < 20 {
		score -= 10 // 小幅增长扣分
	}

	// 新盘数量评分（适度扩展）
	if plan.NewDiskCount >= 2 && plan.NewDiskCount <= 4 {
		score += 10 // 适度扩展加分
	} else if plan.NewDiskCount > 8 {
		score -= 5 // 过多新盘扣分（维护复杂）
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return round(score, 1)
}

// selectBestPlan 选择最优方案.
func (c *RAIDZCalculator) selectBestPlan(plans []ExpansionPlan) *ExpansionPlan {
	if len(plans) == 0 {
		return nil
	}

	best := &plans[0]
	for i := 1; i < len(plans); i++ {
		// 综合ROI评分和成本
		planScore := plans[i].ROIScore - plans[i].CostPerGBAdded
		bestScore := best.ROIScore - best.CostPerGBAdded

		if planScore > bestScore {
			best = &plans[i]
		}
	}

	return best
}

// generateExpansionSuggestions 生成扩容建议.
func (c *RAIDZCalculator) generateExpansionSuggestions(analysis *ExpansionAnalysis) []string {
	suggestions := make([]string, 0)

	if analysis.BestPlan == nil {
		suggestions = append(suggestions, "无法找到合适的扩容方案")
		return suggestions
	}

	// 基于最优方案的建议
	suggestions = append(suggestions, analysis.BestPlan.Recommendation)

	// 成本对比建议
	if len(analysis.CostComparison) > 1 {
		minCost := math.MaxFloat64
		minLevel := RAIDZ1
		for level, cost := range analysis.CostComparison {
			if cost < minCost {
				minCost = cost
				minLevel = level
			}
		}
		suggestions = append(suggestions,
			fmt.Sprintf("最低单位成本方案: %s，单位成本 %.2f 元/GB",
				minLevel, minCost))
	}

	// 可靠性建议
	currentLevel := analysis.Current.Config.Level
	if currentLevel == RAIDZ1 && analysis.BestPlan.TargetConfig.DiskCount >= 6 {
		suggestions = append(suggestions,
			"建议考虑升级到RAIDZ2以提升冗余可靠性")
	}

	// 扩容时机建议
	if analysis.Current.SpaceUtilization < 30 {
		suggestions = append(suggestions,
			"当前利用率较低，建议优先优化数据分布而非扩容")
	} else if analysis.Current.SpaceUtilization > 80 {
		suggestions = append(suggestions,
			"当前利用率较高，建议尽快扩容避免存储压力")
	}

	return suggestions
}

// ========== 辅助方法 ==========

// CompareRAIDZLevels 对比不同RAIDZ级别的成本效益.
func (c *RAIDZCalculator) CompareRAIDZLevels(diskCount int, diskCapacityBytes uint64, diskPrice float64) map[RAIDZLevel]*RAIDZCapacityResult {
	results := make(map[RAIDZLevel]*RAIDZCapacityResult)

	// 只计算盘数满足最小要求的级别
	for level, minDisks := range c.MinDiskRecommendations {
		if diskCount >= minDisks {
			config := RAIDZConfig{
				Level:             level,
				DiskCount:         diskCount,
				DiskCapacityBytes: diskCapacityBytes,
				DiskPrice:         diskPrice,
			}

			if level == Mirror && diskCount%2 == 0 {
				results[level] = c.CalculateMirrorCapacity(config)
			} else if level != Mirror {
				results[level] = c.CalculateCapacity(config)
			}
		}
	}

	return results
}

// GetRecommendedConfig 获取推荐配置.
func (c *RAIDZCalculator) GetRecommendedConfig(targetCapacityGB float64, maxBudget float64, preferredLevel RAIDZLevel) *RAIDZConfig {
	// 基于目标容量和预算计算最优配置

	// 假设4TB磁盘，价格约1200元
	defaultDiskCapacity := uint64(4 * 1024 * 1024 * 1024 * 1024)
	defaultDiskPrice := 1200.0

	// 计算需要的盘数
	parity := c.getParityDisks(preferredLevel)
	effectiveDiskCapacity := float64(defaultDiskCapacity) * (1 - c.ZFSMetadataOverhead)

	targetBytes := targetCapacityGB * 1024 * 1024 * 1024
	requiredUsableDisks := int(math.Ceil(targetBytes / effectiveDiskCapacity))
	requiredTotalDisks := requiredUsableDisks + parity

	minDisks := c.MinDiskRecommendations[preferredLevel]
	if requiredTotalDisks < minDisks {
		requiredTotalDisks = minDisks
	}

	// 检查预算
	totalCost := float64(requiredTotalDisks) * defaultDiskPrice
	if totalCost > maxBudget && maxBudget > 0 {
		// 需要减少盘数或使用更便宜的磁盘
		// 调整盘数为预算上限
		maxDisks := int(maxBudget / defaultDiskPrice)
		if maxDisks >= minDisks {
			requiredTotalDisks = maxDisks
		} else {
			// 预算不足以满足最小要求
			return nil
		}
	}

	config := &RAIDZConfig{
		Level:             preferredLevel,
		DiskCount:         requiredTotalDisks,
		DiskCapacityBytes: defaultDiskCapacity,
		DiskPrice:         defaultDiskPrice,
	}

	return config
}

// ValidateConfig 验证RAIDZ配置合理性.
func (c *RAIDZCalculator) ValidateConfig(config RAIDZConfig) []string {
	issues := make([]string, 0)

	// 检查盘数
	minDisks := c.MinDiskRecommendations[config.Level]
	if config.DiskCount < minDisks {
		issues = append(issues,
			fmt.Sprintf("%s 最少需要 %d 块磁盘，当前配置 %d 块",
				config.Level, minDisks, config.DiskCount))
	}

	// 检查镜像盘数是否为偶数
	if config.Level == Mirror && config.DiskCount%2 != 0 {
		issues = append(issues,
			"镜像模式需要偶数数量的磁盘")
	}

	// 检查磁盘容量
	if config.DiskCapacityBytes == 0 {
		issues = append(issues, "磁盘容量不能为0")
	}

	// 检查磁盘价格
	if config.DiskPrice <= 0 {
		issues = append(issues, "磁盘价格必须大于0")
	}

	// 检查空间效率
	result := c.CalculateCapacity(config)
	if result.SpaceUtilization < 50 {
		issues = append(issues,
			fmt.Sprintf("空间利用率 %.2f%% 过低，建议增加磁盘数量",
				result.SpaceUtilization))
	}

	return issues
}
