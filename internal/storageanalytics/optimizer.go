package storageanalytics

import (
	"fmt"
	"sort"
	"time"
)

// Optimizer 存储优化建议引擎.
type Optimizer struct {
	config       *Config
	costAnalyzer *CostAnalyzer
}

// NewOptimizer 创建优化建议引擎.
func NewOptimizer(config *Config, costAnalyzer *CostAnalyzer) *Optimizer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Optimizer{
		config:       config,
		costAnalyzer: costAnalyzer,
	}
}

// GenerateRecommendations 生成存储优化建议.
func (o *Optimizer) GenerateRecommendations(result *CollectResult, report *StorageReport, costReport *StorageCostReport) []OptimizationRecommendation {
	var recommendations []OptimizationRecommendation

	// 1. 层级优化建议
	tierRecs := o.generateTierRecommendations(result, report, costReport)
	recommendations = append(recommendations, tierRecs...)

	// 2. 去重建议
	dedupRecs := o.generateDedupRecommendations(result, report)
	recommendations = append(recommendations, dedupRecs...)

	// 3. 压缩建议
	compressRecs := o.generateCompressionRecommendations(result, report)
	recommendations = append(recommendations, compressRecs...)

	// 4. 生命周期管理建议
	lifecycleRecs := o.generateLifecycleRecommendations(result, report)
	recommendations = append(recommendations, lifecycleRecs...)

	// 5. 清理建议
	cleanupRecs := o.generateCleanupRecommendations(result, report)
	recommendations = append(recommendations, cleanupRecs...)

	// 按优先级和节省金额排序
	sort.Slice(recommendations, func(i, j int) bool {
		// 先按优先级
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		pi := priorityOrder[recommendations[i].Priority]
		pj := priorityOrder[recommendations[j].Priority]
		if pi != pj {
			return pi < pj
		}
		// 再按节省金额
		return recommendations[i].SavingCost > recommendations[j].SavingCost
	})

	// 为每条建议生成ID
	for i := range recommendations {
		recommendations[i].ID = fmt.Sprintf("OPT-%03d", i+1)
	}

	return recommendations
}

// generateTierRecommendations 生成存储层级优化建议.
func (o *Optimizer) generateTierRecommendations(result *CollectResult, report *StorageReport, costReport *StorageCostReport) []OptimizationRecommendation {
	var recs []OptimizationRecommendation

	if costReport == nil || len(costReport.TierBreakdown) == 0 {
		return recs
	}

	// 检查是否有数据在错误的层级
	for _, bd := range costReport.TierBreakdown {
		if bd.Tier == TierNVMe && bd.UsedTB > 2 {
			// 大量数据在NVMe上，建议迁移到SSD
			migrateTB := bd.UsedTB * 0.5 // 建议迁移50%
			savingPerMonth := migrateTB * (bd.CostPerTB - o.costAnalyzer.getTierCost(TierSSD))

			if savingPerMonth > 10 { // 节省超过10元/月才提建议
				recs = append(recs, OptimizationRecommendation{
					Category:    "tier",
					Priority:    "high",
					Title:       "NVMe层级数据迁移建议",
					Description: fmt.Sprintf("NVMe层级有 %.1fTB 数据，建议将非频繁访问数据迁移到SSD层级", bd.UsedTB),
					Impact:      fmt.Sprintf("每月可节省 %.0f 元", savingPerMonth),
					SavingCost:  round2(savingPerMonth),
					Effort:      "medium",
					Steps: []string{
						"1. 分析NVMe层级中访问频率较低的文件",
						"2. 创建数据迁移计划",
						"3. 使用存储分层策略自动迁移",
						"4. 监控迁移后的性能影响",
					},
				})
			}
		}

		if bd.Tier == TierSSD && bd.UsedTB > 10 {
			// 大量数据在SSD上，建议迁移到HDD
			migrateTB := bd.UsedTB * 0.3
			savingPerMonth := migrateTB * (bd.CostPerTB - o.costAnalyzer.getTierCost(TierHDD))

			if savingPerMonth > 20 {
				recs = append(recs, OptimizationRecommendation{
					Category:    "tier",
					Priority:    "medium",
					Title:       "SSD层级容量优化",
					Description: fmt.Sprintf("SSD层级使用 %.1fTB，建议将冷数据迁移到HDD", bd.UsedTB),
					Impact:      fmt.Sprintf("每月可节省 %.0f 元", savingPerMonth),
					SavingCost:  round2(savingPerMonth),
					Effort:      "easy",
					Steps: []string{
						"1. 启用自动分层存储策略",
						"2. 设置访问频率阈值（如30天未访问）",
						"3. 配置自动迁移到HDD层级",
					},
				})
			}
		}
	}

	return recs
}

// generateDedupRecommendations 生成去重建议.
func (o *Optimizer) generateDedupRecommendations(result *CollectResult, report *StorageReport) []OptimizationRecommendation {
	var recs []OptimizationRecommendation

	if report == nil || report.Health.RedundancyRate < 0.05 {
		return recs
	}

	redundantBytes := int64(float64(result.TotalSize) * report.Health.RedundancyRate)
	if redundantBytes < 100*1024*1024 { // < 100MB 不值得去重
		return recs
	}

	savingTB := float64(redundantBytes) / float64(1024*1024*1024*1024)
	savingPerMonth := savingTB * o.costAnalyzer.getTierCost(TierHDD)

	priority := "low"
	if report.Health.RedundancyRate > 0.2 {
		priority = "high"
	} else if report.Health.RedundancyRate > 0.1 {
		priority = "medium"
	}

	recs = append(recs, OptimizationRecommendation{
		Category:    "dedup",
		Priority:    priority,
		Title:       "数据去重优化",
		Description: fmt.Sprintf("检测到 %.1f%% 冗余率，约 %s 数据可去重", report.Health.RedundancyRate*100, formatBytes(redundantBytes)),
		Impact:      fmt.Sprintf("可释放 %s 存储空间", formatBytes(redundantBytes)),
		SavingBytes: redundantBytes,
		SavingCost:  round2(savingPerMonth),
		Effort:      "medium",
		Steps: []string{
			"1. 运行去重扫描工具",
			"2. 审核去重候选文件列表",
			"3. 配置去重策略（哈希算法、最小文件大小）",
			"4. 执行去重操作",
			"5. 验证数据完整性",
		},
	})

	return recs
}

// generateCompressionRecommendations 生成压缩建议.
func (o *Optimizer) generateCompressionRecommendations(result *CollectResult, report *StorageReport) []OptimizationRecommendation {
	var recs []OptimizationRecommendation

	// 统计可压缩的大文件
	var compressibleSize int64
	var compressibleCount int
	compressibleTypes := map[FileType]bool{
		FileTypeDocument: true,
		FileTypeCode:     true,
		FileTypeOther:    true,
	}

	for _, f := range result.Files {
		if compressibleTypes[f.FileType] && f.Size > 10*1024*1024 { // > 10MB
			compressibleSize += f.Size
			compressibleCount++
		}
	}

	if compressibleSize < 500*1024*1024 { // < 500MB 不值得压缩
		return recs
	}

	// 假设30%压缩率
	compressionSaving := float64(compressibleSize) * 0.3
	savingTB := compressionSaving / float64(1024*1024*1024*1024)
	savingPerMonth := savingTB * o.costAnalyzer.getTierCost(TierHDD)

	recs = append(recs, OptimizationRecommendation{
		Category:    "compression",
		Priority:    "medium",
		Title:       "文件压缩优化",
		Description: fmt.Sprintf("发现 %d 个大文件（共 %s）可压缩存储", compressibleCount, formatBytes(compressibleSize)),
		Impact:      fmt.Sprintf("预计可节省 %s 空间（30%%压缩率）", formatBytes(int64(compressionSaving))),
		SavingBytes: int64(compressionSaving),
		SavingCost:  round2(savingPerMonth),
		Effort:      "easy",
		Steps: []string{
			"1. 启用透明压缩功能",
			"2. 设置压缩级别（建议平衡模式）",
			"3. 对现有大文件执行离线压缩",
			"4. 监控压缩对性能的影响",
		},
	})

	return recs
}

// generateLifecycleRecommendations 生成生命周期管理建议.
func (o *Optimizer) generateLifecycleRecommendations(result *CollectResult, report *StorageReport) []OptimizationRecommendation {
	var recs []OptimizationRecommendation

	// 统计不同年龄段的数据
	var coldDataSize int64
	var staleDataSize int64
	var tempDataSize int64

	for _, f := range result.Files {
		age := time.Since(f.AccessTime)
		if age > 365*24*time.Hour {
			coldDataSize += f.Size
		} else if age > 90*24*time.Hour {
			staleDataSize += f.Size
		}

		if age > 30*24*time.Hour && o.config != nil {
			for _, pattern := range o.config.WastePatterns {
				if matchPattern(f.Path, pattern) {
					tempDataSize += f.Size
					break
				}
			}
		}
	}

	// 冷数据归档建议
	if coldDataSize > 1*1024*1024*1024 { // > 1GB
		savingTB := float64(coldDataSize) / float64(1024*1024*1024*1024)
		savingPerMonth := savingTB * (o.costAnalyzer.getTierCost(TierHDD) - o.costAnalyzer.getTierCost(TierCold))

		recs = append(recs, OptimizationRecommendation{
			Category:    "lifecycle",
			Priority:    "high",
			Title:       "冷数据归档策略",
			Description: fmt.Sprintf("超过1年未访问的数据有 %s，建议归档到冷存储", formatBytes(coldDataSize)),
			Impact:      fmt.Sprintf("每月可节省 %.0f 元存储成本", savingPerMonth),
			SavingBytes: coldDataSize,
			SavingCost:  round2(savingPerMonth),
			Effort:      "easy",
			Steps: []string{
				"1. 配置自动归档策略（访问超过365天）",
				"2. 设置归档目标为冷存储层",
				"3. 配置归档前的数据完整性校验",
				"4. 设置归档通知和审批流程",
			},
		})
	}

	// 陈旧数据建议
	if staleDataSize > 500*1024*1024 { // > 500MB
		recs = append(recs, OptimizationRecommendation{
			Category:    "lifecycle",
			Priority:    "medium",
			Title:       "陈旧数据清理",
			Description: fmt.Sprintf("超过90天未访问的数据有 %s，建议定期清理或归档", formatBytes(staleDataSize)),
			Impact:      "释放存储空间，提升存储效率",
			SavingBytes: staleDataSize,
			Effort:      "easy",
			Steps: []string{
				"1. 审核陈旧数据列表",
				"2. 标记需要保留的数据",
				"3. 清理或归档无用数据",
				"4. 设置自动清理策略",
			},
		})
	}

	return recs
}

// generateCleanupRecommendations 生成清理建议.
func (o *Optimizer) generateCleanupRecommendations(result *CollectResult, report *StorageReport) []OptimizationRecommendation {
	var recs []OptimizationRecommendation

	if report == nil {
		return recs
	}

	// 检查是否有浪费空间
	if report.Insights.WastedSpace > 100*1024*1024 { // > 100MB
		wasteType := "临时文件和缓存"
		if report.Insights.WastedSpace > 1*1024*1024*1024 {
			wasteType = "大量临时文件、日志和缓存"
		}

		savingTB := float64(report.Insights.WastedSpace) / float64(1024*1024*1024*1024)
		savingPerMonth := savingTB * o.costAnalyzer.getTierCost(TierHDD)

		recs = append(recs, OptimizationRecommendation{
			Category:    "cleanup",
			Priority:    "high",
			Title:       "存储空间清理",
			Description: fmt.Sprintf("检测到 %s %s占用空间", formatBytes(report.Insights.WastedSpace), wasteType),
			Impact:      fmt.Sprintf("可释放 %s 存储空间", formatBytes(report.Insights.WastedSpace)),
			SavingBytes: report.Insights.WastedSpace,
			SavingCost:  round2(savingPerMonth),
			Effort:      "easy",
			Steps: []string{
				"1. 扫描并列出临时文件",
				"2. 审核清理候选列表",
				"3. 执行清理操作",
				"4. 配置自动清理策略（如定期清理7天前的临时文件）",
			},
		})
	}

	// 检查大文件
	largeFileCount := 0
	var largeFileSize int64
	for _, f := range result.Files {
		if f.Size > 1*1024*1024*1024 { // > 1GB
			largeFileCount++
			largeFileSize += f.Size
		}
	}

	if largeFileCount > 0 {
		recs = append(recs, OptimizationRecommendation{
			Category:    "cleanup",
			Priority:    "low",
			Title:       "大文件审查",
			Description: fmt.Sprintf("发现 %d 个超过1GB的大文件（共 %s），建议审查是否需要保留", largeFileCount, formatBytes(largeFileSize)),
			Impact:      "释放大量存储空间",
			SavingBytes: largeFileSize / 2, // 假设50%可以清理
			Effort:      "medium",
			Steps: []string{
				"1. 列出所有大文件",
				"2. 审核文件用途和最后访问时间",
				"3. 删除或归档不需要的大文件",
				"4. 配置大文件监控告警",
			},
		})
	}

	return recs
}

// matchPattern 简单的模式匹配.
func matchPattern(path, pattern string) bool {
	// 简化实现：检查路径是否包含模式
	return contains(path, pattern)
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
