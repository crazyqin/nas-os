// 存储成本计算器
package smartlifebackup

import (
	"fmt"
	"time"
)

// CostCalculator 存储成本计算器
type CostCalculator struct {
	costConfig *StorageCost
}

// NewCostCalculator 创建成本计算器
func NewCostCalculator(config *StorageCost) *CostCalculator {
	if config == nil {
		config = DefaultStorageCost()
	}
	return &CostCalculator{
		costConfig: config,
	}
}

// CalculateCost 计算存储成本
func (c *CostCalculator) CalculateCost(tier StorageTier, sizeGB float64) float64 {
	var costPerGB float64

	switch tier {
	case StorageTierHot:
		costPerGB = c.costConfig.HotCostPerGB
	case StorageTierWarm:
		costPerGB = c.costConfig.WarmCostPerGB
	case StorageTierCold:
		costPerGB = c.costConfig.ColdCostPerGB
	case StorageTierArchive:
		costPerGB = c.costConfig.ArchiveCostPerGB
	default:
		costPerGB = c.costConfig.HotCostPerGB
	}

	return costPerGB * sizeGB
}

// CalculateTransferCost 计算传输成本
func (c *CostCalculator) CalculateTransferCost(sizeGB float64) float64 {
	return c.costConfig.TransferCostPerGB * sizeGB
}

// CalculateRequestCost 计算请求成本
func (c *CostCalculator) CalculateRequestCost(requests int) float64 {
	return c.costConfig.RequestCostPer1000 * float64(requests) / 1000
}

// CalculateTotalCost 计算总成本
func (c *CostCalculator) CalculateTotalCost(item *BackupItem) float64 {
	sizeGB := float64(item.Size) / (1024 * 1024 * 1024)

	// 存储成本（按月计算）
	storageCost := c.CalculateCost(item.Tier, sizeGB)

	return storageCost
}

// EstimateSavings 估算迁移到其他层级的节省
func (c *CostCalculator) EstimateSavings(item *BackupItem, targetTier StorageTier) float64 {
	sizeGB := float64(item.Size) / (1024 * 1024 * 1024)

	currentCost := c.CalculateCost(item.Tier, sizeGB)
	targetCost := targetTierCost(sizeGB, targetTier, c.costConfig)

	return currentCost - targetCost
}

// targetTierCost 计算目标层级成本
func targetTierCost(sizeGB float64, tier StorageTier, config *StorageCost) float64 {
	var costPerGB float64

	switch tier {
	case StorageTierHot:
		costPerGB = config.HotCostPerGB
	case StorageTierWarm:
		costPerGB = config.WarmCostPerGB
	case StorageTierCold:
		costPerGB = config.ColdCostPerGB
	case StorageTierArchive:
		costPerGB = config.ArchiveCostPerGB
	default:
		costPerGB = config.HotCostPerGB
	}

	return costPerGB * sizeGB
}

// GenerateReport 生成成本报告
func (c *CostCalculator) GenerateReport(backups map[string]*BackupItem) *CostReport {
	report := &CostReport{
		GeneratedAt:   time.Now(),
		Period:        "monthly",
		TierBreakdown: make([]TierCost, 0),
		Suggestions:   make([]CostSuggestion, 0),
	}

	// 按层级统计
	tierStats := make(map[StorageTier]*TierCost)
	for _, tier := range []StorageTier{StorageTierHot, StorageTierWarm, StorageTierCold, StorageTierArchive} {
		tierStats[tier] = &TierCost{
			Tier: tier,
		}
	}

	for _, item := range backups {
		stat, ok := tierStats[item.Tier]
		if !ok {
			continue
		}

		sizeGB := float64(item.Size) / (1024 * 1024 * 1024)
		stat.StorageGB += sizeGB
		stat.BackupCount++
	}

	// 计算各层级成本
	for tier, stat := range tierStats {
		stat.Cost = c.CalculateCost(tier, stat.StorageGB)
		report.TierBreakdown = append(report.TierBreakdown, *stat)
		report.TotalStorageGB += stat.StorageGB
		report.TotalCost += stat.Cost
	}

	// 生成优化建议
	report.Suggestions = c.generateSuggestions(backups, tierStats)

	return report
}

// generateSuggestions 生成优化建议
func (c *CostCalculator) generateSuggestions(backups map[string]*BackupItem, tierStats map[StorageTier]*TierCost) []CostSuggestion {
	var suggestions []CostSuggestion
	now := time.Now()

	// 建议1: 将超过30天的备份迁移到冷存储
	var hotOldSize float64
	var hotOldCount int
	for _, item := range backups {
		if item.Tier == StorageTierHot {
			age := now.Sub(item.CreatedAt)
			if age > 30*24*time.Hour {
				hotOldSize += float64(item.Size) / (1024 * 1024 * 1024)
				hotOldCount++
			}
		}
	}
	if hotOldCount > 0 {
		savings := c.CalculateCost(StorageTierHot, hotOldSize) - c.CalculateCost(StorageTierCold, hotOldSize)
		suggestions = append(suggestions, CostSuggestion{
			Type:        "tier_migration",
			Description: fmt.Sprintf("有 %d 个超过30天的热存储备份（%.2f GB），建议迁移到冷存储", hotOldCount, hotOldSize),
			Savings:     savings,
			Priority:    1,
		})
	}

	// 建议2: 将超过1年的备份归档
	var coldOldSize float64
	var coldOldCount int
	for _, item := range backups {
		if item.Tier == StorageTierCold || item.Tier == StorageTierWarm {
			age := now.Sub(item.CreatedAt)
			if age > 365*24*time.Hour {
				coldOldSize += float64(item.Size) / (1024 * 1024 * 1024)
				coldOldCount++
			}
		}
	}
	if coldOldCount > 0 {
		savings := c.CalculateCost(StorageTierCold, coldOldSize) - c.CalculateCost(StorageTierArchive, coldOldSize)
		suggestions = append(suggestions, CostSuggestion{
			Type:        "archive",
			Description: fmt.Sprintf("有 %d 个超过1年的备份（%.2f GB），建议归档", coldOldCount, coldOldSize),
			Savings:     savings,
			Priority:    2,
		})
	}

	// 建议3: 启用压缩
	var uncompressedSize float64
	var uncompressedCount int
	for _, item := range backups {
		if !item.Compressed && item.Size > 10*1024*1024 { // 大于10MB
			uncompressedSize += float64(item.Size) / (1024 * 1024 * 1024)
			uncompressedCount++
		}
	}
	if uncompressedCount > 0 {
		// 假设压缩率40%
		potentialSavings := uncompressedSize * 0.4 * c.costConfig.HotCostPerGB
		suggestions = append(suggestions, CostSuggestion{
			Type:        "compression",
			Description: fmt.Sprintf("有 %d 个未压缩的备份（%.2f GB），启用压缩可节省约40%%存储", uncompressedCount, uncompressedSize),
			Savings:     potentialSavings,
			Priority:    3,
		})
	}

	// 建议4: 清理重复备份
	var dedupSavings float64
	checksums := make(map[string][]*BackupItem)
	for _, item := range backups {
		if item.Checksum != "" {
			checksums[item.Checksum] = append(checksums[item.Checksum], item)
		}
	}
	for _, items := range checksums {
		if len(items) > 1 {
			// 保留最新，其他可删除
			for _, item := range items[1:] {
				sizeGB := float64(item.Size) / (1024 * 1024 * 1024)
				dedupSavings += c.CalculateCost(item.Tier, sizeGB)
			}
		}
	}
	if dedupSavings > 0 {
		suggestions = append(suggestions, CostSuggestion{
			Type:        "deduplication",
			Description: "发现重复备份，清理可节省存储空间",
			Savings:     dedupSavings,
			Priority:    4,
		})
	}

	return suggestions
}

// GetCostBreakdown 获取成本分解
func (c *CostCalculator) GetCostBreakdown(backups map[string]*BackupItem) map[string]interface{} {
	result := map[string]interface{}{
		"tiers": make(map[string]interface{}),
	}

	tiers := result["tiers"].(map[string]interface{})

	for _, tier := range []StorageTier{StorageTierHot, StorageTierWarm, StorageTierCold, StorageTierArchive} {
		var totalSize float64
		var count int
		var totalCost float64

		for _, item := range backups {
			if item.Tier == tier {
				sizeGB := float64(item.Size) / (1024 * 1024 * 1024)
				totalSize += sizeGB
				count++
				totalCost += c.CalculateCost(tier, sizeGB)
			}
		}

		tiers[string(tier)] = map[string]interface{}{
			"size_gb":      totalSize,
			"count":        count,
			"monthly_cost": totalCost,
		}
	}

	return result
}

// CompareStrategies 比较不同策略的成本
func (c *CostCalculator) CompareStrategies(backups map[string]*BackupItem, strategies []BackupPolicy) []map[string]interface{} {
	var results []map[string]interface{}

	for _, strategy := range strategies {
		totalCost := 0.0

		for _, item := range backups {
			sizeGB := float64(item.Size) / (1024 * 1024 * 1024)

			// 根据策略确定应该在哪个层级
			age := time.Since(item.CreatedAt)
			var targetTier StorageTier

			for _, rule := range strategy.RetentionRules {
				if rule.RetainDays > 0 && age <= time.Duration(rule.RetainDays)*24*time.Hour {
					targetTier = rule.StorageTier
					break
				}
			}

			if targetTier == "" {
				targetTier = StorageTierArchive
			}

			totalCost += c.CalculateCost(targetTier, sizeGB)
		}

		results = append(results, map[string]interface{}{
			"strategy_id":   strategy.ID,
			"strategy_name": strategy.Name,
			"monthly_cost":  totalCost,
		})
	}

	return results
}

// FormatCost 格式化成本显示
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	} else if cost < 1 {
		return fmt.Sprintf("$%.3f", cost)
	} else if cost < 100 {
		return fmt.Sprintf("$%.2f", cost)
	} else {
		return fmt.Sprintf("$%.0f", cost)
	}
}

// CalculateROI 计算投资回报率
func CalculateROI(savings float64, investmentCost float64) float64 {
	if investmentCost == 0 {
		return 0
	}
	return (savings - investmentCost) / investmentCost * 100
}

// EstimateYearlyCost 估算年成本
func (c *CostCalculator) EstimateYearlyCost(backups map[string]*BackupItem) float64 {
	monthlyReport := c.GenerateReport(backups)
	return monthlyReport.TotalCost * 12
}
