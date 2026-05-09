// Package costoptimizer 提供存储成本优化分析功能
// 差异化优势：竞品（TrueNAS/群晖/飞牛）均无此功能
package costoptimizer

import (
	"time"
)

// StorageTier 存储介质类型（按成本从高到低）
type StorageTier string

const (
	TierNVMe   StorageTier = "nvme"
	TierSSD    StorageTier = "ssd"
	TierHDD    StorageTier = "hdd"
	TierCloud  StorageTier = "cloud"
	TierArchive StorageTier = "archive"
)

// CostProfile 成本画像（每TB每月）
type CostProfile struct {
	Tier           StorageTier `json:"tier"`
	CostPerTBMonth float64     `json:"costPerTBMonth"` // 元/TB/月
	ReadIOPS       int64       `json:"readIOPS"`       // 读IOPS上限
	WriteIOPS      int64       `json:"writeIOPS"`      // 写IOPS上限
	Bandwidth      int64       `json:"bandwidthMBs"`   // 带宽MB/s
	LatencyMs      float64     `json:"latencyMs"`      // 平均延迟ms
	EgressCost     float64     `json:"egressCost"`     // 出口流量成本(元/GB)
}

// DefaultCostProfiles 默认成本画像
var DefaultCostProfiles = map[StorageTier]CostProfile{
	TierNVMe:   {TierNVMe, 150, 500000, 300000, 7000, 0.1, 0},
	TierSSD:    {TierSSD, 80, 100000, 50000, 3000, 0.2, 0},
	TierHDD:    {TierHDD, 15, 200, 150, 200, 10, 0},
	TierCloud:  {TierCloud, 50, 10000, 5000, 1000, 50, 0.5},
	TierArchive: {TierArchive, 5, 100, 50, 100, 500, 1.0},
}

// TierOrder 存储层级从高到低排序
var TierOrder = []StorageTier{TierNVMe, TierSSD, TierHDD, TierCloud, TierArchive}

// StorageAllocation 存储分配信息
type StorageAllocation struct {
	Path         string      `json:"path"`
	Tier         StorageTier `json:"tier"`
	SizeBytes    int64       `json:"sizeBytes"`
	UsedBytes    int64       `json:"usedBytes"`
	AccessCount  int64       `json:"accessCount"`  // 月访问次数
	ReadBytes    int64       `json:"readBytes"`    // 月读取量
	WriteBytes   int64       `json:"writeBytes"`   // 月写入量
	HotDataRatio float64     `json:"hotDataRatio"` // 热数据比例(0-1)
	LastAccess   time.Time   `json:"lastAccess"`   // 最后访问时间
	FileCount    int64       `json:"fileCount"`    // 文件数量
	ContentType  string      `json:"contentType"`  // 内容类型(media/document/archive/etc)
}

// WasteAnalysis 浪费空间分析
type WasteAnalysis struct {
	DuplicateFiles    []DuplicateGroup `json:"duplicateFiles"`
	LargeFiles        []LargeFileInfo  `json:"largeFiles"`
	TempFiles         []TempFileInfo   `json:"tempFiles"`
	EmptyDirs         int64            `json:"emptyDirs"`
	TotalWastedBytes  int64            `json:"totalWastedBytes"`
	DuplicateBytes    int64            `json:"duplicateBytes"`
	TempBytes         int64            `json:"tempBytes"`
	CompressibleBytes int64            `json:"compressibleBytes"`
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	Hash       string   `json:"hash"`
	Paths      []string `json:"paths"`
	SizeBytes  int64    `json:"sizeBytes"`
	Count      int      `json:"count"`
	Wasted     int64    `json:"wasted"` // (count-1)*sizeBytes
}

// LargeFileInfo 大文件信息
type LargeFileInfo struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"sizeBytes"`
	LastMod   time.Time `json:"lastMod"`
	Tier      StorageTier `json:"tier"`
}

// TempFileInfo 临时文件信息
type TempFileInfo struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"sizeBytes"`
	LastMod   time.Time `json:"lastMod"`
}

// CostTrend 成本趋势数据点
type CostTrend struct {
	Date          time.Time `json:"date"`
	TotalCost     float64   `json:"totalCost"`
	UsedTB        float64   `json:"usedTB"`
	CostPerTB     float64   `json:"costPerTB"`
	GrowthPercent float64   `json:"growthPercent"`
}

// CostForecast 成本预测
type CostForecast struct {
	CurrentMonthly   float64          `json:"currentMonthly"`
	ForecastMonths   int              `json:"forecastMonths"`
	Predictions      []CostTrend      `json:"predictions"`
	AnnualProjected  float64          `json:"annualProjected"`
	GrowthRate       float64          `json:"growthRate"`       // 月增长率
	TrendDirection   string           `json:"trendDirection"`   // rising|stable|declining
	Confidence       float64          `json:"confidence"`       // 预测置信度(0-1)
	CostByTier       map[StorageTier]float64 `json:"costByTier"` // 各层级预测成本
}

// ROICalculation ROI计算结果
type ROICalculation struct {
	SourceTier       StorageTier `json:"sourceTier"`
	TargetTier       StorageTier `json:"targetTier"`
	DataBytes        int64       `json:"dataBytes"`
	CurrentCost      float64     `json:"currentCost"`
	TargetCost       float64     `json:"targetCost"`
	MonthlySavings   float64     `json:"monthlySavings"`
	AnnualSavings    float64     `json:"annualSavings"`
	MigrationCost    float64     `json:"migrationCost"`    // 一次性迁移成本
	BreakEvenMonths  float64     `json:"breakEvenMonths"`  // 回本月数
	ROI5Year         float64     `json:"roi5Year"`         // 5年ROI百分比
	PerformanceImpact string     `json:"performanceImpact"` // 性能影响描述
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID              string      `json:"id"`
	Type            string      `json:"type"`     // migrate|dedup|compress|archive|cleanup|tier
	Priority        string      `json:"priority"` // high|medium|low
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	SourcePath      string      `json:"sourcePath"`
	TargetTier      StorageTier `json:"targetTier,omitempty"`
	SavingsPerMonth float64     `json:"savingsPerMonth"`
	SavingsPercent  float64     `json:"savingsPercent"`
	Effort          string      `json:"effort"` // 自动|手动|半自动
	Action          string      `json:"action"`
	ROI             *ROICalculation `json:"roi,omitempty"`
}

// YearlyReport 年度成本报告
type YearlyReport struct {
	Year              int                         `json:"year"`
	MonthlyCosts      [12]float64                 `json:"monthlyCosts"`
	TotalAnnualCost   float64                     `json:"totalAnnualCost"`
	AvgMonthlyCost    float64                     `json:"avgMonthlyCost"`
	PeakMonth         int                         `json:"peakMonth"`
	PeakCost          float64                     `json:"peakCost"`
	GrowthRate        float64                     `json:"growthRate"`
	CostByTier        map[StorageTier]float64     `json:"costByTier"`
	StorageGrowth     [12]float64                 `json:"storageGrowth"` // TB per month
	SavingsAchieved   float64                     `json:"savingsAchieved"`
	OptimizationCount int                         `json:"optimizationCount"`
}

// CostReport 综合成本报告
type CostReport struct {
	GeneratedAt       time.Time                    `json:"generatedAt"`
	TotalMonthlyCost  float64                      `json:"totalMonthlyCost"`
	OptimizedCost     float64                      `json:"optimizedCost"`
	TotalSavings      float64                      `json:"totalSavings"`
	SavingsPercent    float64                      `json:"savingsPercent"`
	CostByTier        map[StorageTier]float64      `json:"costByTier"`
	Allocations       []StorageAllocation          `json:"allocations"`
	Suggestions       []OptimizationSuggestion     `json:"suggestions"`
	WasteAnalysis     *WasteAnalysis               `json:"wasteAnalysis,omitempty"`
	Forecast          *CostForecast                `json:"forecast,omitempty"`
	YearlyReport      *YearlyReport                `json:"yearlyReport,omitempty"`
	DedupPotential    int64                        `json:"dedupPotential"`
	CompressPotential int64                        `json:"compressPotential"`
	ArchivePotential  int64                        `json:"archivePotential"`
	TotalUsedTB       float64                      `json:"totalUsedTB"`
	CostPerTB         float64                      `json:"costPerTB"`
}

// FileSample 用于浪费分析的文件样本
type FileSample struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"sizeBytes"`
	Hash      string    `json:"hash"`
	IsTemp    bool      `json:"isTemp"`
	LastMod   time.Time `json:"lastMod"`
}
