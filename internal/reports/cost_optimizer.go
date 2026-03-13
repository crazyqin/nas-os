// Package reports 提供报表生成和管理功能
package reports

import (
	"sort"
	"time"
)

// ========== 成本优化分析 ==========

// WasteType 浪费类型
type WasteType string

const (
	WasteTypeDuplicate   WasteType = "duplicate"   // 重复文件
	WasteTypeOrphan      WasteType = "orphan"      // 孤立文件
	WasteTypeExpired     WasteType = "expired"     // 过期数据
	WasteTypeTemp        WasteType = "temp"        // 临时文件
	WasteTypeUnused      WasteType = "unused"      // 未使用配额
	WasteTypeOverProvisioned WasteType = "over_provisioned" // 过度分配
	WasteTypeSnapshot    WasteType = "snapshot"    // 过多快照
	WasteTypeOldVersion  WasteType = "old_version" // 旧版本文件
)

// OptimizationType 优化类型
type OptimizationType string

const (
	OptimizationTypeCleanup    OptimizationType = "cleanup"    // 清理
	OptimizationTypeCompress   OptimizationType = "compress"   // 压缩
	OptimizationTypeDedupe     OptimizationType = "dedupe"     // 去重
	OptimizationTypeTiering    OptimizationType = "tiering"    // 分层存储
	OptimizationTypeQuota      OptimizationType = "quota"      // 配额调整
	OptimizationTypeArchive    OptimizationType = "archive"    // 归档
	OptimizationTypeResize     OptimizationType = "resize"     // 卷调整
)

// WasteItem 浪费项
type WasteItem struct {
	// 类型
	Type WasteType `json:"type"`

	// 名称/路径
	Name string `json:"name"`

	// 描述
	Description string `json:"description"`

	// 浪费空间（字节）
	WastedBytes uint64 `json:"wasted_bytes"`

	// 影响的用户/卷
	AffectedTarget string `json:"affected_target"`

	// 发现时间
	DiscoveredAt time.Time `json:"discovered_at"`

	// 可回收
	Recoverable bool `json:"recoverable"`

	// 回收风险（low, medium, high）
	Risk string `json:"risk"`
}

// OptimizationOpportunity 优化机会
type OptimizationOpportunity struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type OptimizationType `json:"type"`

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 影响范围
	Scope string `json:"scope"` // volume, user, system-wide

	// 目标名称
	TargetName string `json:"target_name"`

	// 当前成本（元/月）
	CurrentCostMonthly float64 `json:"current_cost_monthly"`

	// 优化后成本（元/月）
	OptimizedCostMonthly float64 `json:"optimized_cost_monthly"`

	// 节省金额（元/月）
	SavingsMonthly float64 `json:"savings_monthly"`

	// 节省空间（字节）
	SavingsBytes uint64 `json:"savings_bytes"`

	// 节省比例（%）
	SavingsPercent float64 `json:"savings_percent"`

	// 实施难度（easy, medium, hard）
	Implementation string `json:"implementation"`

	// 预计实施时间
	EstimatedTime string `json:"estimated_time"`

	// 优先级（1-10，10最高）
	Priority int `json:"priority"`

	// 风险评估
	RiskAssessment string `json:"risk_assessment"`

	// 实施步骤
	Steps []string `json:"steps"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// CostOptimizationReport 成本优化报告
type CostOptimizationReport struct {
	// ID
	ID string `json:"id"`

	// 名称
	Name string `json:"name"`

	// 报告周期
	Period ReportPeriod `json:"period"`

	// 浪费项列表
	WasteItems []WasteItem `json:"waste_items"`

	// 浪费汇总
	WasteSummary WasteSummary `json:"waste_summary"`

	// 优化机会列表
	Opportunities []OptimizationOpportunity `json:"opportunities"`

	// 优化汇总
	OptimizationSummary OptimizationSummary `json:"optimization_summary"`

	// 推荐行动计划
	ActionPlan []ActionItem `json:"action_plan"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`
}

// WasteSummary 浪费汇总
type WasteSummary struct {
	// 总浪费空间（字节）
	TotalWastedBytes uint64 `json:"total_wasted_bytes"`

	// 按类型统计
	ByType map[WasteType]uint64 `json:"by_type"`

	// 浪费占比（%）
	WastePercent float64 `json:"waste_percent"`

	// 潜在月度成本节省（元）
	PotentialSavingsMonthly float64 `json:"potential_savings_monthly"`

	// 项目数量
	ItemCounts map[WasteType]int `json:"item_counts"`
}

// OptimizationSummary 优化汇总
type OptimizationSummary struct {
	// 总优化机会数
	TotalOpportunities int `json:"total_opportunities"`

	// 总月度节省（元）
	TotalSavingsMonthly float64 `json:"total_savings_monthly"`

	// 总节省空间（字节）
	TotalSavingsBytes uint64 `json:"total_savings_bytes"`

	// 平均节省比例
	AvgSavingsPercent float64 `json:"avg_savings_percent"`

	// 按类型统计节省
	SavingsByType map[OptimizationType]float64 `json:"savings_by_type"`

	// 按难度统计
	ByDifficulty map[string]int `json:"by_difficulty"`

	// 快速见效项目数（易实施且高节省）
	QuickWinCount int `json:"quick_win_count"`
}

// ActionItem 行动项
type ActionItem struct {
	// 序号
	Sequence int `json:"sequence"`

	// 优化机会ID
	OpportunityID string `json:"opportunity_id"`

	// 行动标题
	Title string `json:"title"`

	// 行动描述
	Description string `json:"description"`

	// 预计节省
	ExpectedSavings float64 `json:"expected_savings"`

	// 实施难度
	Difficulty string `json:"difficulty"`

	// 建议时间表
	Timeline string `json:"timeline"`

	// 负责人
	Assignee string `json:"assignee,omitempty"`
}

// CostOptimizer 成本优化器
type CostOptimizer struct {
	config StorageCostConfig
}

// NewCostOptimizer 创建成本优化器
func NewCostOptimizer(config StorageCostConfig) *CostOptimizer {
	return &CostOptimizer{config: config}
}

// AnalyzeWaste 分析浪费
func (o *CostOptimizer) AnalyzeWaste(items []WasteItem, totalCapacity uint64) WasteSummary {
	summary := WasteSummary{
		ByType:    make(map[WasteType]uint64),
		ItemCounts: make(map[WasteType]int),
	}

	for _, item := range items {
		summary.TotalWastedBytes += item.WastedBytes
		summary.ByType[item.Type] += item.WastedBytes
		summary.ItemCounts[item.Type]++
	}

	if totalCapacity > 0 {
		summary.WastePercent = round(float64(summary.TotalWastedBytes)/float64(totalCapacity)*100, 2)
	}

	// 计算潜在节省
	wastedGB := float64(summary.TotalWastedBytes) / (1024 * 1024 * 1024)
	summary.PotentialSavingsMonthly = round(wastedGB*o.config.CostPerGBMonthly, 2)

	return summary
}

// IdentifyOpportunities 识别优化机会
func (o *CostOptimizer) IdentifyOpportunities(
	wasteItems []WasteItem,
	volumeMetrics []StorageMetrics,
	currentCosts []StorageCostResult,
) []OptimizationOpportunity {
	opportunities := make([]OptimizationOpportunity, 0)
	now := time.Now()

	// 1. 分析重复文件优化
	dupOpp := o.analyzeDuplicates(wasteItems, now)
	if dupOpp != nil {
		opportunities = append(opportunities, *dupOpp)
	}

	// 2. 分析过期数据清理
	expiredOpp := o.analyzeExpiredData(wasteItems, now)
	if expiredOpp != nil {
		opportunities = append(opportunities, *expiredOpp)
	}

	// 3. 分析未使用配额
	quotaOpp := o.analyzeUnusedQuota(wasteItems, now)
	if quotaOpp != nil {
		opportunities = append(opportunities, *quotaOpp)
	}

	// 4. 分析压缩机会
	compressOpp := o.analyzeCompression(volumeMetrics, now)
	if compressOpp != nil {
		opportunities = append(opportunities, *compressOpp)
	}

	// 5. 分析去重机会
	dedupeOpp := o.analyzeDeduplication(wasteItems, now)
	if dedupeOpp != nil {
		opportunities = append(opportunities, *dedupeOpp)
	}

	// 6. 分析分层存储机会
	tieringOpp := o.analyzeTiering(volumeMetrics, currentCosts, now)
	if tieringOpp != nil {
		opportunities = append(opportunities, *tieringOpp)
	}

	// 7. 分析卷调整机会
	resizeOpp := o.analyzeResize(volumeMetrics, currentCosts, now)
	if resizeOpp != nil {
		opportunities = append(opportunities, *resizeOpp)
	}

	// 按优先级排序
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].Priority > opportunities[j].Priority
	})

	return opportunities
}

// analyzeDuplicates 分析重复文件优化
func (o *CostOptimizer) analyzeDuplicates(items []WasteItem, now time.Time) *OptimizationOpportunity {
	var totalDupBytes uint64
	var dupCount int

	for _, item := range items {
		if item.Type == WasteTypeDuplicate {
			totalDupBytes += item.WastedBytes
			dupCount++
		}
	}

	if totalDupBytes == 0 {
		return nil
	}

	savingsGB := float64(totalDupBytes) / (1024 * 1024 * 1024)
	savingsMonthly := round(savingsGB*o.config.CostPerGBMonthly, 2)

	return &OptimizationOpportunity{
		ID:                   "opt_dup_" + now.Format("20060102"),
		Type:                 OptimizationTypeDedupe,
		Title:                "启用数据去重",
		Description:          "发现重复文件，建议启用去重功能或手动清理",
		Scope:                "system-wide",
		SavingsMonthly:       savingsMonthly,
		SavingsBytes:         totalDupBytes,
		SavingsPercent:       30.0, // 去重通常节省30%
		Implementation:       "medium",
		EstimatedTime:        "1-2周",
		Priority:             8,
		RiskAssessment:       "低风险，建议先备份",
		CurrentCostMonthly:   savingsMonthly / 0.3,
		OptimizedCostMonthly: savingsMonthly / 0.3 * 0.7,
		Steps: []string{
			"1. 扫描识别重复文件",
			"2. 生成重复文件报告",
			"3. 审核并确认清理范围",
			"4. 启用去重功能或手动删除",
			"5. 验证存储空间回收",
		},
		CreatedAt: now,
	}
}

// analyzeExpiredData 分析过期数据清理
func (o *CostOptimizer) analyzeExpiredData(items []WasteItem, now time.Time) *OptimizationOpportunity {
	var totalExpiredBytes uint64
	var expiredCount int

	for _, item := range items {
		if item.Type == WasteTypeExpired || item.Type == WasteTypeOldVersion {
			totalExpiredBytes += item.WastedBytes
			expiredCount++
		}
	}

	if totalExpiredBytes == 0 {
		return nil
	}

	savingsGB := float64(totalExpiredBytes) / (1024 * 1024 * 1024)
	savingsMonthly := round(savingsGB*o.config.CostPerGBMonthly, 2)

	return &OptimizationOpportunity{
		ID:                   "opt_exp_" + now.Format("20060102"),
		Type:                 OptimizationTypeCleanup,
		Title:                "清理过期数据",
		Description:          "发现过期文件和旧版本文件，建议按保留策略清理",
		Scope:                "system-wide",
		SavingsMonthly:       savingsMonthly,
		SavingsBytes:         totalExpiredBytes,
		SavingsPercent:       100.0,
		Implementation:       "easy",
		EstimatedTime:        "1-3天",
		Priority:             9,
		RiskAssessment:       "低风险，建议先归档",
		CurrentCostMonthly:   savingsMonthly,
		OptimizedCostMonthly: 0,
		Steps: []string{
			"1. 识别过期数据（超过保留期限）",
			"2. 生成清理报告",
			"3. 审核并确认清理列表",
			"4. 执行清理或归档",
			"5. 更新保留策略",
		},
		CreatedAt: now,
	}
}

// analyzeUnusedQuota 分析未使用配额
func (o *CostOptimizer) analyzeUnusedQuota(items []WasteItem, now time.Time) *OptimizationOpportunity {
	var totalUnusedBytes uint64
	var unusedCount int

	for _, item := range items {
		if item.Type == WasteTypeOverProvisioned || item.Type == WasteTypeUnused {
			totalUnusedBytes += item.WastedBytes
			unusedCount++
		}
	}

	if totalUnusedBytes == 0 {
		return nil
	}

	savingsGB := float64(totalUnusedBytes) / (1024 * 1024 * 1024)
	savingsMonthly := round(savingsGB*o.config.CostPerGBMonthly, 2)

	return &OptimizationOpportunity{
		ID:                   "opt_quota_" + now.Format("20060102"),
		Type:                 OptimizationTypeQuota,
		Title:                "调整配额分配",
		Description:          "发现过度分配的配额，建议回收或重新分配",
		Scope:                "user",
		SavingsMonthly:       savingsMonthly,
		SavingsBytes:         totalUnusedBytes,
		SavingsPercent:       50.0,
		Implementation:       "easy",
		EstimatedTime:        "1天",
		Priority:             7,
		RiskAssessment:       "低风险，需通知用户",
		CurrentCostMonthly:   savingsMonthly,
		OptimizedCostMonthly: savingsMonthly * 0.5,
		Steps: []string{
			"1. 分析配额使用情况",
			"2. 识别过度分配的用户/组",
			"3. 生成配额调整建议",
			"4. 通知相关用户",
			"5. 执行配额调整",
		},
		CreatedAt: now,
	}
}

// analyzeCompression 分析压缩机会
func (o *CostOptimizer) analyzeCompression(metrics []StorageMetrics, now time.Time) *OptimizationOpportunity {
	var totalCompressibleBytes uint64

	for _, m := range metrics {
		// 假设50%的数据可压缩
		totalCompressibleBytes += m.UsedCapacityBytes / 2
	}

	if totalCompressibleBytes == 0 {
		return nil
	}

	// 压缩通常节省40-60%，取50%
	savingsBytes := totalCompressibleBytes / 2
	savingsGB := float64(savingsBytes) / (1024 * 1024 * 1024)
	savingsMonthly := round(savingsGB*o.config.CostPerGBMonthly, 2)

	return &OptimizationOpportunity{
		ID:                   "opt_comp_" + now.Format("20060102"),
		Type:                 OptimizationTypeCompress,
		Title:                "启用数据压缩",
		Description:          "对适合的数据类型启用压缩，减少存储占用",
		Scope:                "system-wide",
		SavingsMonthly:       savingsMonthly,
		SavingsBytes:         savingsBytes,
		SavingsPercent:       50.0,
		Implementation:       "medium",
		EstimatedTime:        "1-2周",
		Priority:             6,
		RiskAssessment:       "低风险，可能影响性能",
		CurrentCostMonthly:   savingsMonthly / 0.5,
		OptimizedCostMonthly: savingsMonthly,
		Steps: []string{
			"1. 分析数据类型和压缩潜力",
			"2. 选择适合压缩的卷/目录",
			"3. 评估性能影响",
			"4. 启用压缩功能",
			"5. 监控压缩率和性能",
		},
		CreatedAt: now,
	}
}

// analyzeDeduplication 分析去重机会
func (o *CostOptimizer) analyzeDeduplication(items []WasteItem, now time.Time) *OptimizationOpportunity {
	// 这个方法在analyzeDuplicates中已处理
	return nil
}

// analyzeTiering 分析分层存储机会
func (o *CostOptimizer) analyzeTiering(metrics []StorageMetrics, costs []StorageCostResult, now time.Time) *OptimizationOpportunity {
	// 查找低使用率但高成本的卷
	var totalColdBytes uint64
	var targetVolumes []string

	for i, m := range metrics {
		if i < len(costs) && costs[i].UsagePercent < 30 {
			// 低使用率卷，可能适合分层
			totalColdBytes += m.UsedCapacityBytes
			targetVolumes = append(targetVolumes, m.VolumeName)
		}
	}

	if len(targetVolumes) == 0 {
		return nil
	}

	// 分层到低成本存储可节省约50%
	savingsBytes := totalColdBytes / 2
	savingsGB := float64(savingsBytes) / (1024 * 1024 * 1024)
	savingsMonthly := round(savingsGB*o.config.CostPerGBMonthly*0.5, 2)

	return &OptimizationOpportunity{
		ID:                   "opt_tier_" + now.Format("20060102"),
		Type:                 OptimizationTypeTiering,
		Title:                "实施分层存储",
		Description:          "将冷数据迁移到低成本存储层",
		Scope:                "volume",
		TargetName:           "多个卷",
		SavingsMonthly:       savingsMonthly,
		SavingsBytes:         savingsBytes,
		SavingsPercent:       50.0,
		Implementation:       "hard",
		EstimatedTime:        "2-4周",
		Priority:             5,
		RiskAssessment:       "中风险，需测试迁移",
		CurrentCostMonthly:   savingsMonthly * 2,
		OptimizedCostMonthly: savingsMonthly,
		Steps: []string{
			"1. 分析数据访问模式",
			"2. 识别冷数据（低访问频率）",
			"3. 配置分层策略",
			"4. 执行数据迁移",
			"5. 验证数据可访问性",
		},
		CreatedAt: now,
	}
}

// analyzeResize 分析卷调整机会
func (o *CostOptimizer) analyzeResize(metrics []StorageMetrics, costs []StorageCostResult, now time.Time) *OptimizationOpportunity {
	// 查找过度分配的卷
	var overAllocatedBytes uint64
	var targetVolumes []string

	for _, m := range metrics {
		if m.AvailableCapacityBytes > m.TotalCapacityBytes/2 {
			// 超过50%空闲，可能过度分配
			overAllocatedBytes += m.AvailableCapacityBytes / 2
			targetVolumes = append(targetVolumes, m.VolumeName)
		}
	}

	if len(targetVolumes) == 0 {
		return nil
	}

	savingsGB := float64(overAllocatedBytes) / (1024 * 1024 * 1024)
	savingsMonthly := round(savingsGB*o.config.CostPerGBMonthly, 2)

	return &OptimizationOpportunity{
		ID:                   "opt_resize_" + now.Format("20060102"),
		Type:                 OptimizationTypeResize,
		Title:                "调整卷大小",
		Description:          "收缩过度分配的卷，释放未使用空间",
		Scope:                "volume",
		TargetName:           "多个卷",
		SavingsMonthly:       savingsMonthly,
		SavingsBytes:         overAllocatedBytes,
		SavingsPercent:       25.0,
		Implementation:       "medium",
		EstimatedTime:        "1-2周",
		Priority:             4,
		RiskAssessment:       "中风险，需确保足够预留",
		CurrentCostMonthly:   savingsMonthly / 0.25,
		OptimizedCostMonthly: savingsMonthly / 0.25 * 0.75,
		Steps: []string{
			"1. 分析卷使用趋势",
			"2. 计算合理的卷大小",
			"3. 规划收缩时间窗口",
			"4. 执行卷收缩操作",
			"5. 验证数据完整性",
		},
		CreatedAt: now,
	}
}

// GenerateReport 生成成本优化报告
func (o *CostOptimizer) GenerateReport(
	wasteItems []WasteItem,
	volumeMetrics []StorageMetrics,
	currentCosts []StorageCostResult,
	totalCapacity uint64,
	period ReportPeriod,
) *CostOptimizationReport {
	now := time.Now()

	// 分析浪费
	wasteSummary := o.AnalyzeWaste(wasteItems, totalCapacity)

	// 识别优化机会
	opportunities := o.IdentifyOpportunities(wasteItems, volumeMetrics, currentCosts)

	// 计算优化汇总
	optimizationSummary := o.calculateOptimizationSummary(opportunities)

	// 生成行动计划
	actionPlan := o.generateActionPlan(opportunities)

	return &CostOptimizationReport{
		ID:                  "opt_report_" + now.Format("20060102150405"),
		Name:                "成本优化报告",
		Period:              period,
		WasteItems:          wasteItems,
		WasteSummary:        wasteSummary,
		Opportunities:       opportunities,
		OptimizationSummary: optimizationSummary,
		ActionPlan:          actionPlan,
		GeneratedAt:         now,
	}
}

// calculateOptimizationSummary 计算优化汇总
func (o *CostOptimizer) calculateOptimizationSummary(opportunities []OptimizationOpportunity) OptimizationSummary {
	summary := OptimizationSummary{
		TotalOpportunities: len(opportunities),
		SavingsByType:      make(map[OptimizationType]float64),
		ByDifficulty:       make(map[string]int),
	}

	for _, opp := range opportunities {
		summary.TotalSavingsMonthly += opp.SavingsMonthly
		summary.TotalSavingsBytes += opp.SavingsBytes
		summary.SavingsByType[opp.Type] += opp.SavingsMonthly
		summary.ByDifficulty[opp.Implementation]++
	}

	if len(opportunities) > 0 {
		summary.AvgSavingsPercent = round(summary.TotalSavingsMonthly/float64(len(opportunities)), 2)
	}

	// 统计快速见效项目
	for _, opp := range opportunities {
		if opp.Implementation == "easy" && opp.SavingsMonthly > 100 {
			summary.QuickWinCount++
		}
	}

	summary.TotalSavingsMonthly = round(summary.TotalSavingsMonthly, 2)

	return summary
}

// generateActionPlan 生成行动计划
func (o *CostOptimizer) generateActionPlan(opportunities []OptimizationOpportunity) []ActionItem {
	// 按优先级和难度排序
	sorted := make([]OptimizationOpportunity, len(opportunities))
	copy(sorted, opportunities)

	sort.Slice(sorted, func(i, j int) bool {
		// 优先级高的优先，难度低的优先
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		// 简单优先
		difficultyOrder := map[string]int{"easy": 0, "medium": 1, "hard": 2}
		return difficultyOrder[sorted[i].Implementation] < difficultyOrder[sorted[j].Implementation]
	})

	actionPlan := make([]ActionItem, 0, len(sorted))
	for i, opp := range sorted {
		timeline := opp.EstimatedTime
		if i > 0 {
			// 累积时间线
			timeline = "待排期"
		}

		actionPlan = append(actionPlan, ActionItem{
			Sequence:        i + 1,
			OpportunityID:   opp.ID,
			Title:           opp.Title,
			Description:     opp.Description,
			ExpectedSavings: opp.SavingsMonthly,
			Difficulty:      opp.Implementation,
			Timeline:        timeline,
		})
	}

	return actionPlan
}

// UpdateConfig 更新配置
func (o *CostOptimizer) UpdateConfig(config StorageCostConfig) {
	o.config = config
}

// GetConfig 获取配置
func (o *CostOptimizer) GetConfig() StorageCostConfig {
	return o.config
}