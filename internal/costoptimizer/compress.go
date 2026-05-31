package costoptimizer

import (
	"fmt"
	"sort"
)

// CompressAnalyzer 压缩收益估算器
type CompressAnalyzer struct {
	optimizer *CostOptimizer
}

// NewCompressAnalyzer 创建压缩分析器
func NewCompressAnalyzer(co *CostOptimizer) *CompressAnalyzer {
	return &CompressAnalyzer{optimizer: co}
}

// CompressAlgorithm 压缩算法
type CompressAlgorithm string

const (
	CompressLZ4  CompressAlgorithm = "lz4"
	CompressZSTD CompressAlgorithm = "zstd"
	CompressZLIB CompressAlgorithm = "zlib"
)

// CompressProfile 压缩算法特性
type CompressProfile struct {
	Algorithm     CompressAlgorithm `json:"algorithm"`
	Name          string            `json:"name"`
	CompressRatio float64           `json:"compress_ratio"`  // 典型压缩比
	SpeedMBps     int               `json:"speed_mbps"`      // 压缩速度 MB/s
	CPUPercent    float64           `json:"cpu_percent"`     // CPU 使用率
	Level         int               `json:"level"`           // 压缩级别
	Description   string            `json:"description"`
}

// DefaultCompressProfiles 默认压缩算法特性
var DefaultCompressProfiles = map[CompressAlgorithm]CompressProfile{
	CompressLZ4: {
		Algorithm:     CompressLZ4,
		Name:          "LZ4",
		CompressRatio: 0.50, // 2:1 压缩比
		SpeedMBps:     800,
		CPUPercent:    10,
		Level:         1,
		Description:   "极速压缩，适合实时场景，压缩比适中",
	},
	CompressZSTD: {
		Algorithm:     CompressZSTD,
		Name:          "Zstandard",
		CompressRatio: 0.35, // ~2.8:1 压缩比
		SpeedMBps:     400,
		CPUPercent:    25,
		Level:         3,
		Description:   "均衡压缩，压缩比和速度兼顾，推荐默认使用",
	},
	CompressZLIB: {
		Algorithm:     CompressZLIB,
		Name:          "zlib",
		CompressRatio: 0.30, // ~3.3:1 压缩比
		SpeedMBps:     150,
		CPUPercent:    40,
		Level:         6,
		Description:   "高压缩比，适合冷数据归档，CPU 开销较大",
	},
}

// DataTypeCompressFactor 各数据类型的压缩系数调整
var DataTypeCompressFactor = map[DataType]float64{
	DataTypeDocuments: 0.8,  // 文档压缩效果好
	DataTypeMedia:     1.8,  // 媒体文件已压缩，压缩效果差
	DataTypeBackup:    0.7,  // 备份数据压缩效果好
	DataTypeArchive:   0.85, // 归档数据压缩效果不错
	DataTypeSystem:    1.2,  // 系统文件压缩效果一般
	DataTypeCache:     1.0,  // 缓存数据压缩效果中等
}

// CompressResult 压缩分析结果
type CompressResult struct {
	TotalDataBytes       int64              `json:"total_data_bytes"`
	ByAlgorithm          []CompressByAlgo   `json:"by_algorithm"`
	ByDataType           []CompressByType   `json:"by_data_type"`
	RecommendedAlgo      CompressAlgorithm  `json:"recommended_algo"`
	RecommendedSavings   int64              `json:"recommended_savings_bytes"`
	RecommendedCostSave  float64            `json:"recommended_savings_cost"`
	Recommendations      []CompressRecommend `json:"recommendations"`
}

// CompressByAlgo 按算法的压缩分析
type CompressByAlgo struct {
	Algorithm    CompressAlgorithm `json:"algorithm"`
	AlgorithmName string           `json:"algorithm_name"`
	CompressRatio float64          `json:"compress_ratio"`
	SavingsBytes  int64            `json:"savings_bytes"`
	SavingsCost   float64          `json:"savings_cost"`
	SpeedMBps     int              `json:"speed_mbps"`
	CPUPercent    float64          `json:"cpu_percent"`
	Score         float64          `json:"score"` // 综合评分
}

// CompressByType 按数据类型的压缩分析
type CompressByType struct {
	DataType      DataType `json:"data_type"`
	TotalBytes    int64    `json:"total_bytes"`
	CompressRatio float64  `json:"compress_ratio"`
	SavingsBytes  int64    `json:"savings_bytes"`
	Recommended   CompressAlgorithm `json:"recommended_algo"`
}

// CompressRecommend 压缩建议
type CompressRecommend struct {
	Priority    string            `json:"priority"`
	Algorithm   CompressAlgorithm `json:"algorithm"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	SavingsGB   float64           `json:"savings_gb"`
	SavingsCost float64           `json:"savings_cost"`
}

// EstimateCompressBenefit 估算压缩收益
func (ca *CompressAnalyzer) EstimateCompressBenefit() *CompressResult {
	allocs := ca.optimizer.allocations
	profiles := ca.optimizer.profiles

	result := &CompressResult{
		ByAlgorithm:     make([]CompressByAlgo, 0),
		ByDataType:      make([]CompressByType, 0),
		Recommendations: make([]CompressRecommend, 0),
	}

	if len(allocs) == 0 {
		return result
	}

	// 按数据类型汇总
	typeMap := make(map[DataType]*CompressByType)
	for _, alloc := range allocs {
		result.TotalDataBytes += alloc.UsedBytes
		dt, ok := typeMap[alloc.DataType]
		if !ok {
			dt = &CompressByType{DataType: alloc.DataType}
			typeMap[alloc.DataType] = dt
		}
		dt.TotalBytes += alloc.UsedBytes
	}

	// 计算各数据类型的压缩效果
	for _, dt := range typeMap {
		bestRatio := 1.0
		var bestAlgo CompressAlgorithm
		for algo, profile := range DefaultCompressProfiles {
			factor, ok := DataTypeCompressFactor[dt.DataType]
			if !ok {
				factor = 1.0
			}
			effectiveRatio := profile.CompressRatio * factor
			if effectiveRatio < 0.1 {
				effectiveRatio = 0.1
			}
			if effectiveRatio > 1.0 {
				effectiveRatio = 1.0
			}
			if effectiveRatio < bestRatio {
				bestRatio = effectiveRatio
				bestAlgo = algo
			}
		}
		dt.CompressRatio = bestRatio
		dt.SavingsBytes = int64(float64(dt.TotalBytes) * (1 - bestRatio))
		dt.Recommended = bestAlgo
		result.ByDataType = append(result.ByDataType, *dt)
	}

	// 计算各算法的整体效果
	totalSavingsByAlgo := make(map[CompressAlgorithm]int64)
	for _, alloc := range allocs {
		factor, ok := DataTypeCompressFactor[alloc.DataType]
		if !ok {
			factor = 1.0
		}
		for algo, profile := range DefaultCompressProfiles {
			effectiveRatio := profile.CompressRatio * factor
			if effectiveRatio < 0.1 {
				effectiveRatio = 0.1
			}
			if effectiveRatio > 1.0 {
				effectiveRatio = 1.0
			}
			savings := int64(float64(alloc.UsedBytes) * (1 - effectiveRatio))
			totalSavingsByAlgo[algo] += savings
		}
	}

	// 计算总成本和各算法的综合评分
	var totalCost float64
	for _, alloc := range allocs {
		if profile, ok := profiles[alloc.Tier]; ok {
			totalCost += bytesToTB(alloc.UsedBytes) * profile.CostPerTBMonth
		}
	}

	bestScore := 0.0
	for algo, profile := range DefaultCompressProfiles {
		savings := totalSavingsByAlgo[algo]
		savingsCost := bytesToTB(savings) * (totalCost / max(bytesToTB(result.TotalDataBytes), 0.001))

		// 综合评分 = 节省空间 * 0.4 + 压缩速度 * 0.3 + CPU效率 * 0.3
		normSavings := float64(savings) / float64(max(result.TotalDataBytes, 1))
		normSpeed := float64(profile.SpeedMBps) / 800.0
		normCPU := 1.0 - profile.CPUPercent/100.0
		score := normSavings*0.4 + normSpeed*0.3 + normCPU*0.3

		algoResult := CompressByAlgo{
			Algorithm:     algo,
			AlgorithmName: profile.Name,
			CompressRatio: profile.CompressRatio,
			SavingsBytes:  savings,
			SavingsCost:   savingsCost,
			SpeedMBps:     profile.SpeedMBps,
			CPUPercent:    profile.CPUPercent,
			Score:         score,
		}
		result.ByAlgorithm = append(result.ByAlgorithm, algoResult)

		if score > bestScore {
			bestScore = score
			result.RecommendedAlgo = algo
			result.RecommendedSavings = savings
			result.RecommendedCostSave = savingsCost
		}
	}

	// 按评分排序
	sort.Slice(result.ByAlgorithm, func(i, j int) bool {
		return result.ByAlgorithm[i].Score > result.ByAlgorithm[j].Score
	})

	// 生成建议
	result.Recommendations = ca.generateCompressRecommendations(result)

	return result
}

// generateCompressRecommendations 生成压缩建议
func (ca *CompressAnalyzer) generateCompressRecommendations(result *CompressResult) []CompressRecommend {
	var recs []CompressRecommend

	// 推荐最优算法
	bestProfile := DefaultCompressProfiles[result.RecommendedAlgo]
	recs = append(recs, CompressRecommend{
		Priority:    "high",
		Algorithm:   result.RecommendedAlgo,
		Title:       fmt.Sprintf("使用 %s 压缩算法", bestProfile.Name),
		Description: bestProfile.Description,
		SavingsGB:   float64(result.RecommendedSavings) / (1024 * 1024 * 1024),
		SavingsCost: result.RecommendedCostSave,
	})

	// 针对不同数据类型的建议
	for _, dt := range result.ByDataType {
		if dt.SavingsBytes > 10*1024*1024*1024 && dt.CompressRatio < 0.8 { // >10GB可压缩
			algoProfile := DefaultCompressProfiles[dt.Recommended]
			recs = append(recs, CompressRecommend{
				Priority:    "medium",
				Algorithm:   dt.Recommended,
				Title:       fmt.Sprintf("对 %s 数据使用 %s 压缩", dt.DataType, algoProfile.Name),
				Description: fmt.Sprintf("预计压缩比 %.1f:1，可节省 %s", 1/dt.CompressRatio, FormatBytes(dt.SavingsBytes)),
				SavingsGB:   float64(dt.SavingsBytes) / (1024 * 1024 * 1024),
			})
		}
	}

	return recs
}
