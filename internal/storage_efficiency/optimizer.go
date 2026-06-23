package storage_efficiency

import (
	"fmt"
)

// Optimizer 优化建议生成引擎.
type Optimizer struct {
	analyzer *Analyzer
}

// NewOptimizer 创建优化建议引擎.
func NewOptimizer(analyzer *Analyzer) *Optimizer {
	return &Optimizer{analyzer: analyzer}
}

// GenerateSuggestions 基于分析结果生成优化建议.
func (o *Optimizer) GenerateSuggestions(path string) ([]Suggestion, error) {
	if path == "" {
		path = "/"
	}

	var suggestions []Suggestion

	compStats, err := o.analyzer.GetCompressionStats(path)
	if err != nil {
		return nil, fmt.Errorf("获取压缩统计失败: %w", err)
	}

	dedupStats, err := o.analyzer.GetDedupStats(path)
	if err != nil {
		return nil, fmt.Errorf("获取去重统计失败: %w", err)
	}

	if comp := o.compressionSuggestions(compStats); len(comp) > 0 {
		suggestions = append(suggestions, comp...)
	}

	if dedup := o.dedupSuggestions(dedupStats); len(dedup) > 0 {
		suggestions = append(suggestions, dedup...)
	}

	if tier := o.tieringSuggestions(path); len(tier) > 0 {
		suggestions = append(suggestions, tier...)
	}

	if cost := o.costSuggestions(path); len(cost) > 0 {
		suggestions = append(suggestions, cost...)
	}

	if suggestions == nil {
		suggestions = []Suggestion{}
	}

	sortSuggestions(suggestions)

	return suggestions, nil
}

// compressionSuggestions 生成压缩优化建议.
func (o *Optimizer) compressionSuggestions(stats *CompressionStats) []Suggestion {
	var suggestions []Suggestion

	if stats.CompressedFiles > 0 && stats.UncompressedFiles > stats.CompressedFiles {
		potentialMB := stats.TotalOriginalSize / 2 / (1024 * 1024)
		if potentialMB > 100 {
			suggestions = append(suggestions, Suggestion{
				ID:       "comp_enable",
				Type:     SuggestTypeCompression,
				Priority: PriorityHigh,
				Title:    "开启文件系统透明压缩",
				Description: fmt.Sprintf(
					"发现 %d 个未压缩文件，远多于已压缩的 %d 个文件。"+
						"建议在存储池上启用透明压缩（如 ZFS LZ4/ZSTD），预计可节省约 %d MB 空间。",
					stats.UncompressedFiles, stats.CompressedFiles, potentialMB),
				PotentialMB: potentialMB,
			})
		}
	}

	if stats.WorstRatio > 0.8 && stats.WorstRatio < 1.0 {
		suggestions = append(suggestions, Suggestion{
			ID:       "comp_algo_upgrade",
			Type:     SuggestTypeCompression,
			Priority: PriorityMedium,
			Title:    "升级压缩算法",
			Description: fmt.Sprintf(
				"当前最差压缩率为 %.2f，部分文件压缩效果不理想。"+
					"建议将压缩算法从 LZ4 升级到 ZSTD（压缩级别 3-6），可提升压缩率约 20-30%%。",
				stats.WorstRatio),
			PotentialMB: (stats.TotalOriginalSize - stats.TotalCompressedSize) / 4 / (1024 * 1024),
		})
	}

	if stats.AverageRatio >= 2.0 {
		suggestions = append(suggestions, Suggestion{
			ID:       "comp_excellent",
			Type:     SuggestTypeCompression,
			Priority: PriorityLow,
			Title:    "压缩效果优秀",
			Description: fmt.Sprintf(
				"当前平均压缩率为 %.2f，压缩效果很好。"+
					"可以考虑对冷数据使用更高压缩级别（ZSTD-9）以进一步节省空间。",
				stats.AverageRatio),
			PotentialMB: stats.TotalCompressedSize / 10 / (1024 * 1024),
		})
	}

	return suggestions
}

// dedupSuggestions 生成去重优化建议.
func (o *Optimizer) dedupSuggestions(stats *DedupStats) []Suggestion {
	var suggestions []Suggestion

	if stats.DedupPercent > 10.0 {
		savedMB := stats.SpaceSavedBytes / (1024 * 1024)
		suggestions = append(suggestions, Suggestion{
			ID:       "dedup_enable",
			Type:     SuggestTypeDedup,
			Priority: PriorityHigh,
			Title:    "开启文件去重",
			Description: fmt.Sprintf(
				"发现 %.1f%% 的文件为重复文件（%d 个重复文件），"+
					"建议开启块级去重功能，预计可节省约 %d MB 空间。",
				stats.DedupPercent, stats.DuplicateFiles, savedMB),
			PotentialMB: savedMB,
		})
	}

	if stats.BlockDedupRatio > 0 && stats.BlockDedupRatio < 0.8 {
		duplicateBlocks := stats.TotalBlocks - stats.UniqueBlocks
		savedMB := int64(duplicateBlocks) * 4096 / (1024 * 1024)
		suggestions = append(suggestions, Suggestion{
			ID:       "dedup_block",
			Type:     SuggestTypeDedup,
			Priority: PriorityMedium,
			Title:    "启用块级去重",
			Description: fmt.Sprintf(
				"块去重率为 %.2f，存在 %d 个重复数据块。"+
					"建议启用块级去重（4KB 块大小），预计可节省 %d MB 空间。",
				stats.BlockDedupRatio, duplicateBlocks, savedMB),
			PotentialMB: savedMB,
		})
	}

	if stats.DedupPercent > 30.0 {
		suggestions = append(suggestions, Suggestion{
			ID:       "dedup_excellent",
			Type:     SuggestTypeDedup,
			Priority: PriorityLow,
			Title:    "去重效果优秀",
			Description: fmt.Sprintf(
				"当前去重率 %.1f%%，去重效果显著。"+
					"建议定期运行去重任务以保持最优存储效率。",
				stats.DedupPercent),
			PotentialMB: 0,
		})
	}

	return suggestions
}

// tieringSuggestions 生成分层存储建议.
func (o *Optimizer) tieringSuggestions(path string) []Suggestion {
	var suggestions []Suggestion

	summary, err := o.analyzer.Analyze(path, 5, false)
	if err != nil {
		return suggestions
	}

	if summary.SpaceSavedPercent < 20.0 && summary.TotalLogicalSize > 10*1024*1024*1024 {
		suggestions = append(suggestions, Suggestion{
			ID:       "tier_cold",
			Type:     SuggestTypeTiering,
			Priority: PriorityMedium,
			Title:    "冷热数据分层",
			Description: fmt.Sprintf(
				"存储效率仅 %.1f%%，总逻辑数据量 %d GB。"+
					"建议将超过 90 天未访问的冷数据迁移到低成本存储层（如 HDD 归档池），"+
					"热数据保留在 SSD 层以保证性能。",
				summary.SpaceSavedPercent, summary.TotalLogicalSize/(1024*1024*1024)),
			PotentialMB: summary.TotalLogicalSize / 5 / (1024 * 1024),
		})
	}

	if summary.CompressionRatio > 2.5 && summary.DedupRatio < 0.5 {
		suggestions = append(suggestions, Suggestion{
			ID:       "tier_balance",
			Type:     SuggestTypeTiering,
			Priority: PriorityLow,
			Title:    "平衡性能与效率",
			Description: fmt.Sprintf(
				"当前压缩率 %.2f，去重率 %.2f，存储效率很好。"+
					"如果系统 CPU 使用率较高，可以考虑降低压缩级别以释放 CPU 资源。",
				summary.CompressionRatio, summary.DedupRatio),
			PotentialMB: 0,
		})
	}

	return suggestions
}

// costSuggestions 生成成本优化建议.
func (o *Optimizer) costSuggestions(path string) []Suggestion {
	var suggestions []Suggestion

	summary, err := o.analyzer.Analyze(path, 10, false)
	if err != nil {
		return suggestions
	}

	// 建议1：如果压缩去重节省空间少于20%，建议优化
	if summary.SpaceSavedPercent < 20.0 && summary.TotalLogicalSize > 5*1024*1024*1024 {
		totalGB := float64(summary.TotalLogicalSize) / (1024 * 1024 * 1024)
		// 假设 HDD 成本 0.1元/GB/月
		currentCost := totalGB * 0.1
		potentialSavings := currentCost * 0.3 // 假设可节省30%

		suggestions = append(suggestions, Suggestion{
			ID:       "cost_optimize",
			Type:     SuggestTypeCost,
			Priority: PriorityMedium,
			Title:    "存储成本优化机会",
			Description: fmt.Sprintf(
				"当前存储效率 %.1f%%，总数据量 %.1f GB。"+
					"启用压缩和去重后，预计每月可节省约 %.1f 元存储成本。",
				summary.SpaceSavedPercent, totalGB, potentialSavings),
			PotentialMB: int64(summary.SpaceSaved * 30 / (1024 * 1024)), // 假设可额外节省30%
		})
	}

	// 建议2：数据量大时建议分层存储降低成本
	if summary.TotalLogicalSize > 100*1024*1024*1024 {
		totalGB := float64(summary.TotalLogicalSize) / (1024 * 1024 * 1024)
		// SSD vs HDD 成本差异
		ssdCost := totalGB * 0.5
		hddCost := totalGB * 0.1

		suggestions = append(suggestions, Suggestion{
			ID:       "cost_tier",
			Type:     SuggestTypeCost,
			Priority: PriorityHigh,
			Title:    "建议使用分层存储降低成本",
			Description: fmt.Sprintf(
				"数据量 %.1f GB，如果全部使用 SSD 存储，月成本约 %.1f 元。"+
					"建议将冷数据迁移到 HDD 层，可将月成本降至约 %.1f 元，节省 %.1f 元/月。",
				totalGB, ssdCost, hddCost*0.6+ssdCost*0.4, ssdCost-(hddCost*0.6+ssdCost*0.4)),
			PotentialMB: 0,
		})
	}

	return suggestions
}

// sortSuggestions 按优先级排序建议（high > medium > low）.
func sortSuggestions(suggestions []Suggestion) {
	priorityOrder := map[string]int{
		PriorityHigh:   0,
		PriorityMedium: 1,
		PriorityLow:    2,
	}

	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			pi, okI := priorityOrder[suggestions[i].Priority]
			pj, okJ := priorityOrder[suggestions[j].Priority]
			if !okI {
				pi = 99
			}
			if !okJ {
				pj = 99
			}
			if pi > pj {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}
}
