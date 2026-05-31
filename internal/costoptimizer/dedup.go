package costoptimizer

import (
	"fmt"
	"sort"
)

// DedupAnalyzer 去重潜力分析器
type DedupAnalyzer struct {
	optimizer *CostOptimizer
}

// NewDedupAnalyzer 创建去重分析器
func NewDedupAnalyzer(co *CostOptimizer) *DedupAnalyzer {
	return &DedupAnalyzer{optimizer: co}
}

// DedupResult 去重分析结果
type DedupResult struct {
	TotalDataBytes      int64              `json:"total_data_bytes"`
	EstimatedDedupBytes int64              `json:"estimated_dedup_bytes"`
	DedupRatio          float64            `json:"dedup_ratio"`           // 去重率 (0-1)
	SavingsBytes        int64              `json:"savings_bytes"`         // 预计节省空间
	SavingsCost         float64            `json:"savings_cost"`          // 预计节省成本（元/月）
	ByDataType          []DedupByDataType  `json:"by_data_type"`
	ByTier              []DedupByTier      `json:"by_tier"`
	Recommendations     []DedupRecommend   `json:"recommendations"`
}

// DedupByDataType 按数据类型的去重分析
type DedupByDataType struct {
	DataType     DataType `json:"data_type"`
	TotalBytes   int64    `json:"total_bytes"`
	DedupBytes   int64    `json:"dedup_bytes"`
	DedupRatio   float64  `json:"dedup_ratio"`
	SavingsBytes int64    `json:"savings_bytes"`
}

// DedupByTier 按存储层的去重分析
type DedupByTier struct {
	Tier         StorageTier `json:"tier"`
	TotalBytes   int64       `json:"total_bytes"`
	DedupBytes   int64       `json:"dedup_bytes"`
	DedupRatio   float64     `json:"dedup_ratio"`
	SavingsBytes int64       `json:"savings_bytes"`
	SavingsCost  float64     `json:"savings_cost"`
}

// DedupRecommend 去重建议
type DedupRecommend struct {
	Priority    string  `json:"priority"` // high/medium/low
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingsGB   float64 `json:"savings_gb"`
	SavingsCost float64 `json:"savings_cost"`
}

// EstimateDedupPotential 估算去重潜力
func (da *DedupAnalyzer) EstimateDedupPotential() *DedupResult {
	allocs := da.optimizer.allocations
	profiles := da.optimizer.profiles

	result := &DedupResult{
		ByDataType:      make([]DedupByDataType, 0),
		ByTier:          make([]DedupByTier, 0),
		Recommendations: make([]DedupRecommend, 0),
	}

	if len(allocs) == 0 {
		return result
	}

	// 按数据类型汇总
	dataTypeMap := make(map[DataType]*DedupByDataType)
	tierMap := make(map[StorageTier]*DedupByTier)

	for _, alloc := range allocs {
		result.TotalDataBytes += alloc.UsedBytes

		// 根据数据类型估算去重比
		dedupRatio := da.estimateDedupRatio(alloc.DataType, alloc.UsedBytes)
		dedupBytes := int64(float64(alloc.UsedBytes) * dedupRatio)
		savings := dedupBytes

		result.EstimatedDedupBytes += dedupBytes
		result.SavingsBytes += savings

		// 按数据类型聚合
		dt, ok := dataTypeMap[alloc.DataType]
		if !ok {
			dt = &DedupByDataType{DataType: alloc.DataType}
			dataTypeMap[alloc.DataType] = dt
		}
		dt.TotalBytes += alloc.UsedBytes
		dt.DedupBytes += dedupBytes
		dt.SavingsBytes += savings

		// 按存储层聚合
		tier, ok := tierMap[alloc.Tier]
		if !ok {
			tier = &DedupByTier{Tier: alloc.Tier}
			tierMap[alloc.Tier] = tier
		}
		tier.TotalBytes += alloc.UsedBytes
		tier.DedupBytes += dedupBytes
		tier.SavingsBytes += savings
	}

	// 计算去重率
	if result.TotalDataBytes > 0 {
		result.DedupRatio = float64(result.EstimatedDedupBytes) / float64(result.TotalDataBytes)
	}

	// 计算各类型去重率
	for _, dt := range dataTypeMap {
		if dt.TotalBytes > 0 {
			dt.DedupRatio = float64(dt.DedupBytes) / float64(dt.TotalBytes)
		}
		result.ByDataType = append(result.ByDataType, *dt)
	}

	// 计算各层去重率和节省成本
	for tier, t := range tierMap {
		if t.TotalBytes > 0 {
			t.DedupRatio = float64(t.DedupBytes) / float64(t.TotalBytes)
		}
		if profile, ok := profiles[tier]; ok {
			t.SavingsCost = bytesToTB(t.SavingsBytes) * profile.CostPerTBMonth
		}
		result.SavingsCost += t.SavingsCost
		result.ByTier = append(result.ByTier, *t)
	}

	// 按节省空间排序
	sort.Slice(result.ByDataType, func(i, j int) bool {
		return result.ByDataType[i].SavingsBytes > result.ByDataType[j].SavingsBytes
	})
	sort.Slice(result.ByTier, func(i, j int) bool {
		return result.ByTier[i].SavingsBytes > result.ByTier[j].SavingsBytes
	})

	// 生成建议
	result.Recommendations = da.generateDedupRecommendations(result)

	return result
}

// estimateDedupRatio 根据数据类型和大小估算去重比
func (da *DedupAnalyzer) estimateDedupRatio(dataType DataType, sizeBytes int64) float64 {
	// 不同数据类型的典型去重比
	baseRatio := map[DataType]float64{
		DataTypeDocuments: 0.25, // 文档去重潜力高（大量相似文件）
		DataTypeMedia:     0.05, // 媒体文件已压缩，去重潜力低
		DataTypeBackup:    0.40, // 备份数据去重潜力最高
		DataTypeArchive:   0.30, // 归档数据也有不错去重潜力
		DataTypeSystem:    0.15, // 系统文件有一定重复
		DataTypeCache:     0.35, // 缓存数据重复度高
	}

	ratio, ok := baseRatio[dataType]
	if !ok {
		ratio = 0.15 // 默认15%
	}

	// 大文件去重效果更好（块级去重）
	if sizeBytes > 500*1024*1024*1024 { // >500GB
		ratio *= 1.3
	} else if sizeBytes > 100*1024*1024*1024 { // >100GB
		ratio *= 1.15
	}

	// 上限
	if ratio > 0.5 {
		ratio = 0.5
	}

	return ratio
}

// generateDedupRecommendations 生成去重建议
func (da *DedupAnalyzer) generateDedupRecommendations(result *DedupResult) []DedupRecommend {
	var recs []DedupRecommend

	// 高去重潜力的数据类型
	for _, dt := range result.ByDataType {
		if dt.DedupRatio > 0.2 && dt.SavingsBytes > 10*1024*1024*1024 { // >10GB且去重率>20%
			priority := "medium"
			if dt.DedupRatio > 0.3 {
				priority = "high"
			}
			recs = append(recs, DedupRecommend{
				Priority:    priority,
				Title:       fmt.Sprintf("对 %s 类型数据启用去重", dt.DataType),
				Description: fmt.Sprintf("该类型数据预计去重率 %.1f%%，可节省 %s 空间", dt.DedupRatio*100, FormatBytes(dt.SavingsBytes)),
				SavingsGB:   float64(dt.SavingsBytes) / (1024 * 1024 * 1024),
			})
		}
	}

	// 高价值存储层
	for _, t := range result.ByTier {
		if t.SavingsCost > 10 && t.SavingsBytes > 5*1024*1024*1024 {
			recs = append(recs, DedupRecommend{
				Priority:    "high",
				Title:       fmt.Sprintf("在 %s 层启用去重节省成本", t.Tier),
				Description: fmt.Sprintf("预计可节省 ¥%.2f/月", t.SavingsCost),
				SavingsGB:   float64(t.SavingsBytes) / (1024 * 1024 * 1024),
				SavingsCost: t.SavingsCost,
			})
		}
	}

	return recs
}
