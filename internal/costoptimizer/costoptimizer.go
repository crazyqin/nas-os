// Package costoptimizer 提供存储成本优化分析功能
// 差异化优势：竞品（TrueNAS/群晖/飞牛）均无此功能
// 分析存储使用效率，提供成本优化建议，预测存储开支
package costoptimizer

import (
	"fmt"
	"sort"
	"time"
)

// CostOptimizer 存储成本优化器
type CostOptimizer struct {
	config      *OptimizerConfig
	profiles    map[StorageTier]CostProfile
	allocations []StorageAllocation
	dedup       *DedupAnalyzer
	compress    *CompressAnalyzer
	tiering     *TieringAnalyzer
}

// NewCostOptimizer 创建成本优化器
func NewCostOptimizer() *CostOptimizer {
	profiles := make(map[StorageTier]CostProfile)
	for k, v := range DefaultCostProfiles {
		profiles[k] = v
	}
	co := &CostOptimizer{
		config:   DefaultOptimizerConfig(),
		profiles: profiles,
	}
	co.dedup = NewDedupAnalyzer(co)
	co.compress = NewCompressAnalyzer(co)
	co.tiering = NewTieringAnalyzer(co)
	return co
}

// NewCostOptimizerWithConfig 使用自定义配置创建成本优化器
func NewCostOptimizerWithConfig(cfg *OptimizerConfig) *CostOptimizer {
	co := NewCostOptimizer()
	co.config = cfg
	return co
}

// SetAllocations 设置存储分配数据
func (co *CostOptimizer) SetAllocations(allocs []StorageAllocation) {
	co.allocations = allocs
}

// SetCostProfile 设置自定义成本画像
func (co *CostOptimizer) SetCostProfile(tier StorageTier, profile CostProfile) {
	co.profiles[tier] = profile
}

// GetDedupAnalyzer 获取去重分析器
func (co *CostOptimizer) GetDedupAnalyzer() *DedupAnalyzer {
	return co.dedup
}

// GetCompressAnalyzer 获取压缩分析器
func (co *CostOptimizer) GetCompressAnalyzer() *CompressAnalyzer {
	return co.compress
}

// GetTieringAnalyzer 获取分层分析器
func (co *CostOptimizer) GetTieringAnalyzer() *TieringAnalyzer {
	return co.tiering
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
		tbUsed := bytesToTB(alloc.UsedBytes)
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

	report.Allocations = co.allocations

	// 分析浪费空间
	wasted := co.calculateWastedSpace()
	if totalSize := co.totalAllocatedBytes(); totalSize > 0 {
		wastePercent := float64(wasted) / float64(totalSize) * 100
		report.WasteAnalysis = &WasteAnalysis{
			TotalWastedBytes: wasted,
			WastePercent:     wastePercent,
		}
	}

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

		tbUsed := bytesToTB(alloc.UsedBytes)

		// 建议1: 低访问数据迁移到廉价存储
		if alloc.AccessCount < 10 && (alloc.Tier == TierNVMe || alloc.Tier == TierSSD) {
			targetTier := TierHDD
			targetProfile := co.profiles[targetTier]
			currentCost := tbUsed * profile.CostPerTBMonth
			targetCost := tbUsed * targetProfile.CostPerTBMonth
			savings := currentCost - targetCost
			id++
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              fmt.Sprintf("OPT-%04d", id),
				Type:            "migrate",
				Priority:        "high",
				Title:           fmt.Sprintf("迁移冷数据 %s 到 HDD", alloc.Path),
				Description:     fmt.Sprintf("该路径月访问仅%d次，存储在%s上浪费。迁移到HDD可显著降低成本。", alloc.AccessCount, profile.Name),
				SourcePath:      alloc.Path,
				TargetTier:      targetTier,
				SavingsPerMonth: savings,
				SavingsPercent:  safePercent(savings, currentCost),
				Effort:          "自动",
				Action:          fmt.Sprintf("将 %s 从 %s 迁移到 %s", alloc.Path, alloc.Tier, targetTier),
			})
		}

		// 建议2: 低使用率数据压缩
		usageRate := float64(0)
		if alloc.SizeBytes > 0 {
			usageRate = float64(alloc.UsedBytes) / float64(alloc.SizeBytes) * 100
		}
		if usageRate < 50 && alloc.SizeBytes > 100*1024*1024*1024 {
			id++
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:              fmt.Sprintf("OPT-%04d", id),
				Type:            "compress",
				Priority:        "medium",
				Title:           fmt.Sprintf("压缩低使用率存储 %s", alloc.Path),
				Description:     fmt.Sprintf("使用率仅 %.1f%%，启用压缩可节省空间。", usageRate),
				SourcePath:      alloc.Path,
				SavingsPerMonth: 0,
				Effort:          "自动",
				Action:          fmt.Sprintf("对 %s 启用 zstd 压缩", alloc.Path),
			})
		}

		// 建议3: 长期未访问数据归档
		if alloc.AccessCount == 0 && alloc.UsedBytes > 10*1024*1024*1024 {
			targetTier := TierCloud
			if profile.CostPerTBMonth > co.profiles[targetTier].CostPerTBMonth {
				currentCost := tbUsed * profile.CostPerTBMonth
				targetCost := tbUsed * co.profiles[targetTier].CostPerTBMonth
				savings := currentCost - targetCost
				id++
				suggestions = append(suggestions, OptimizationSuggestion{
					ID:              fmt.Sprintf("OPT-%04d", id),
					Type:            "archive",
					Priority:        "low",
					Title:           fmt.Sprintf("归档冷数据 %s", alloc.Path),
					Description:     "该数据完全无访问记录，建议归档到云存储。",
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
		if unused > 0 && float64(alloc.UsedBytes)/float64(alloc.SizeBytes) < 0.3 {
			wasted += unused
		}
	}
	return wasted
}

// totalAllocatedBytes 计算总分配字节数
func (co *CostOptimizer) totalAllocatedBytes() int64 {
	var total int64
	for _, alloc := range co.allocations {
		total += alloc.SizeBytes
	}
	return total
}

// ========== 辅助函数 ==========

// bytesToTB 字节转 TB
func bytesToTB(bytes int64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024 * 1024)
}

// safePercent 安全计算百分比，避免除零
func safePercent(part, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (part / total) * 100
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
	return fmt.Sprintf("¥%.2f", cost)
}
