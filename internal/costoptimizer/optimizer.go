// Package costoptimizer 提供存储成本优化分析功能
// 差异化优势：竞品（TrueNAS/群晖/飞牛）均无此功能
// 分析存储使用效率，提供成本优化建议，预测存储开支
package costoptimizer

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// StorageTier 存储介质类型
type StorageTier string

const (
	TierNVMe  StorageTier = "nvme"
	TierSSD   StorageTier = "ssd"
	TierHDD   StorageTier = "hdd"
	TierCloud StorageTier = "cloud"
	TierTape  StorageTier = "tape"
)

// CostProfile 成本画像（每TB每月）
type CostProfile struct {
	Tier          StorageTier `json:"tier"`
	CostPerTBMonth float64    `json:"costPerTBMonth"` // 元/TB/月
	ReadIOPS      int64       `json:"readIOPS"`       // 读IOPS上限
	WriteIOPS     int64       `json:"writeIOPS"`      // 写IOPS上限
	Bandwidth     int64       `json:"bandwidthMBs"`   // 带宽MB/s
	LatencyMs     float64     `json:"latencyMs"`      // 平均延迟ms
}

// DefaultCostProfiles 默认成本画像
var DefaultCostProfiles = map[StorageTier]CostProfile{
	TierNVMe:  {TierNVMe, 150, 500000, 300000, 7000, 0.1},
	TierSSD:   {TierSSD, 80, 100000, 50000, 3000, 0.2},
	TierHDD:   {TierHDD, 15, 200, 150, 200, 10},
	TierCloud: {TierCloud, 50, 10000, 5000, 1000, 50},
}

// StorageAllocation 存储分配
type StorageAllocation struct {
	Path         string      `json:"path"`
	Tier         StorageTier `json:"tier"`
	SizeBytes    int64       `json:"sizeBytes"`
	UsedBytes    int64       `json:"usedBytes"`
	AccessCount  int64       `json:"accessCount"`  // 月访问次数
	ReadBytes    int64       `json:"readBytes"`    // 月读取量
	WriteBytes   int64       `json:"writeBytes"`   // 月写入量
	HotDataRatio float64     `json:"hotDataRatio"` // 热数据比例
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID          string        `json:"id"`
	Type        string        `json:"type"` // migrate|dedup|compress|archive|cleanup
	Priority    string        `json:"priority"` // high|medium|low
	Title       string        `json:"title"`
	Description string        `json:"description"`
	SourcePath  string        `json:"sourcePath"`
	TargetTier  StorageTier   `json:"targetTier,omitempty"`
	SavingsPerMonth float64   `json:"savingsPerMonth"` // 预计每月节省(元)
	SavingsPercent  float64   `json:"savingsPercent"`  // 节省百分比
	Effort      string        `json:"effort"` // 自动|手动|半自动
	Action      string        `json:"action"` // 建议操作
}

// CostReport 成本报告
type CostReport struct {
	GeneratedAt       time.Time              `json:"generatedAt"`
	TotalMonthlyCost  float64                `json:"totalMonthlyCost"`  // 当前月总成本
	OptimizedCost     float64                `json:"optimizedCost"`    // 优化后月成本
	TotalSavings      float64                `json:"totalSavings"`     // 可节省金额
	SavingsPercent    float64                `json:"savingsPercent"`   // 节省百分比
	CostByTier        map[StorageTier]float64 `json:"costByTier"`      // 各层级成本
	Allocations       []StorageAllocation    `json:"allocations"`
	Suggestions       []OptimizationSuggestion `json:"suggestions"`
	WastedSpace       int64                  `json:"wastedSpace"`      // 浪费空间(字节)
	DedupPotential    int64                  `json:"dedupPotential"`   // 去重潜力(字节)
	CompressPotential int64                  `json:"compressPotential"` // 压缩潜力(字节)
	ArchivePotential  int64                  `json:"archivePotential"`  // 归档潜力(字节)
}

// CostOptimizer 存储成本优化器
type CostOptimizer struct {
	profiles    map[StorageTier]CostProfile
	allocations []StorageAllocation
}

// NewCostOptimizer 创建成本优化器
func NewCostOptimizer() *CostOptimizer {
	profiles := make(map[StorageTier]CostProfile)
	for k, v := range DefaultCostProfiles {
		profiles[k] = v
	}
	return &CostOptimizer{profiles: profiles}
}

// SetAllocations 设置存储分配数据
func (co *CostOptimizer) SetAllocations(allocs []StorageAllocation) {
	co.allocations = allocs
}

// GenerateReport 生成成本优化报告
func (co *CostOptimizer) GenerateReport() *CostReport {
	report := &CostReport{
		GeneratedAt: time.Now(),
		CostByTier:  make(map[StorageTier]float64),
	}

	// 计算当前成本
	for _, alloc := range co.allocations {
		profile, ok := co.profiles[alloc.Tier]
		if !ok {
			continue
		}
		tbUsed := float64(alloc.UsedBytes) / (1024 * 1024 * 1024 * 1024)
		cost := tbUsed * profile.CostPerTBMonth
		report.CostByTier[alloc.Tier] += cost
		report.TotalMonthlyCost += cost
	}

	// 生成优化建议
	suggestions := co.analyzeOptimizations()
	report.Suggestions = suggestions

	// 计算节省总额
	for _, s := range suggestions {
		report.TotalSavings += s.SavingsPerMonth
	}
	report.OptimizedCost = report.TotalMonthlyCost - report.TotalSavings
	if report.TotalMonthlyCost > 0 {
		report.SavingsPercent = (report.TotalSavings / report.TotalMonthlyCost) * 100
	}

	// 分析浪费空间和潜力
	report.WastedSpace = co.calculateWastedSpace()
	report.DedupPotential = co.estimateDedupPotential()
	report.CompressPotential = co.estimateCompressPotential()
	report.ArchivePotential = co.estimateArchivePotential()
	report.Allocations = co.allocations

	return report
}

// analyzeOptimizations 分析优化机会
func (co *CostOptimizer) analyzeOptimizations() []OptimizationSuggestion {
	var suggestions []OptimizationSuggestion
	id := 0
	for _, alloc := range co.allocations {
		profile, ok := co.profiles[alloc.Tier]
		if !ok {
			continue
		}
		// 建议1: 低访问数据迁移到廉价存储
		if alloc.AccessCount < 10 && alloc.Tier == TierNVMe {
			targetTier := TierHDD
			targetProfile := co.profiles[targetTier]
			currentCost := float64(alloc.UsedBytes) / (1024 * 1024 * 1024 * 1024) * profile.CostPerTBMonth
			targetCost := float64(alloc.UsedBytes) / (1024 * 1024 * 1024 * 1024) * targetProfile.CostPerTBMonth
			savings := currentCost - targetCost
			id++
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              fmt.Sprintf("OPT-%04d", id),
				Type:            "migrate",
				Priority:        "high",
				Title:           fmt.Sprintf("迁移冷数据 %s 到 HDD", alloc.Path),
				Description:     fmt.Sprintf("该路径月访问仅%d次，存储在NVMe上浪费。迁移到HDD可显著降低成本。", alloc.AccessCount),
				SourcePath:      alloc.Path,
				TargetTier:      targetTier,
				SavingsPerMonth: savings,
				SavingsPercent:  savings / currentCost * 100,
				Effort:          "自动",
				Action:          fmt.Sprintf("将 %s 从 %s 迁移到 %s", alloc.Path, alloc.Tier, targetTier),
			})
		}
		// 建议2: 低使用率数据压缩
		usageRate := float64(0)
		if alloc.SizeBytes > 0 {
			usageRate = float64(alloc.UsedBytes) / float64(alloc.SizeBytes) * 100
		}
		if usageRate < 50 && alloc.SizeBytes > 100*1024*1024*1024 { // 使用率<50%且>100GB
			id++
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              fmt.Sprintf("OPT-%04d", id),
				Type:            "compress",
				Priority:        "medium",
				Title:           fmt.Sprintf("压缩低使用率存储 %s", alloc.Path),
				Description:     fmt.Sprintf("使用率仅 %.1f%%，启用压缩可节省空间。", usageRate),
				SourcePath:      alloc.Path,
				SavingsPerMonth: 0, // 压缩节省的是空间而非直接成本
				Effort:          "自动",
				Action:          fmt.Sprintf("对 %s 启用 zstd 压缩", alloc.Path),
			})
		}
		// 建议3: 长期未访问数据归档
		if alloc.AccessCount == 0 && alloc.UsedBytes > 10*1024*1024*1024 { // 无访问且>10GB
			targetTier := TierCloud
			if profile.CostPerTBMonth > co.profiles[targetTier].CostPerTBMonth {
				currentCost := float64(alloc.UsedBytes) / (1024 * 1024 * 1024 * 1024) * profile.CostPerTBMonth
				targetCost := float64(alloc.UsedBytes) / (1024 * 1024 * 1024 * 1024) * co.profiles[targetTier].CostPerTBMonth
				savings := currentCost - targetCost
				id++
				suggestions = append(suggestions, OptimizationSuggestion{
					ID:              fmt.Sprintf("OPT-%04d", id),
					Type:            "archive",
					Priority:        "low",
					Title:           fmt.Sprintf("归档冷数据 %s", alloc.Path),
					Description:     fmt.Sprintf("该数据完全无访问记录，建议归档到云存储。"),
					SourcePath:      alloc.Path,
					TargetTier:      targetTier,
					SavingsPerMonth: savings,
					Effort:          "半自动",
					Action:          fmt.Sprintf("将 %s 归档到云存储", alloc.Path),
				})
			}
		}
	}
	// 按节省金额排序
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].SavingsPerMonth > suggestions[j].SavingsPerMonth
	})
	return suggestions
}

// calculateWastedSpace 计算浪费空间
func (co *CostOptimizer) calculateWastedSpace() int64 {
	var wasted int64
	for _, alloc := range co.allocations {
		unused := alloc.SizeBytes - alloc.UsedBytes
		if unused > 0 && float64(alloc.UsedBytes)/float64(alloc.SizeBytes) < 0.3 { // 使用率<30%
			wasted += unused
		}
	}
	return wasted
}

// estimateDedupPotential 估算去重潜力
func (co *CostOptimizer) estimateDedupPotential() int64 {
	// 基于使用率估算重复数据比例（简化模型）
	var potential int64
	for _, alloc := range co.allocations {
		if alloc.UsedBytes > 100*1024*1024*1024 { // >100GB
			// 估算10-20%的重复数据
			potential += int64(float64(alloc.UsedBytes) * 0.15)
		}
	}
	return potential
}

// estimateCompressPotential 估算压缩潜力
func (co *CostOptimizer) estimateCompressPotential() int64 {
	// 基于数据类型估算压缩比
	var potential int64
	for _, alloc := range co.allocations {
		// 估算平均2:1压缩比
		potential += alloc.UsedBytes / 2
	}
	return potential
}

// estimateArchivePotential 估算归档潜力
func (co *CostOptimizer) estimateArchivePotential() int64 {
	var potential int64
	for _, alloc := range co.allocations {
		if alloc.AccessCount < 5 { // 低访问数据
			potential += alloc.UsedBytes
		}
	}
	return potential
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatCost 格式化成本
func FormatCost(cost float64) string {
	return fmt.Sprintf("¥%.2f", math.Round(cost*100)/100)
}
