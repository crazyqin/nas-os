// Package storage_efficiency 提供存储效率分析功能，包括压缩率统计、去重分析、
// 空间节省计算和优化建议。
package storage_efficiency

import (
	"time"
)

// EfficiencySummary 存储效率总览.
type EfficiencySummary struct {
	TotalLogicalSize  int64     `json:"totalLogicalSize"`  // 逻辑大小（字节）
	TotalPhysicalSize int64     `json:"totalPhysicalSize"` // 物理大小（字节）
	CompressionRatio  float64   `json:"compressionRatio"`  // 压缩率（逻辑/物理）
	DedupRatio        float64   `json:"dedupRatio"`        // 去重率（去重后/去重前）
	SpaceSaved        int64     `json:"spaceSaved"`        // 节省空间（字节）
	SpaceSavedPercent float64   `json:"spaceSavedPercent"` // 节省百分比
	UpdatedAt         time.Time `json:"updatedAt"`         // 更新时间
}

// CompressionStats 压缩统计详情.
type CompressionStats struct {
	CompressedFiles     int     `json:"compressedFiles"`     // 已压缩文件数
	UncompressedFiles   int     `json:"uncompressedFiles"`   // 未压缩文件数
	AverageRatio        float64 `json:"averageRatio"`        // 平均压缩率
	BestRatio           float64 `json:"bestRatio"`           // 最佳压缩率
	WorstRatio          float64 `json:"worstRatio"`          // 最差压缩率
	TotalOriginalSize   int64   `json:"totalOriginalSize"`   // 压缩前总大小
	TotalCompressedSize int64   `json:"totalCompressedSize"` // 压缩后总大小
}

// DedupStats 去重统计详情.
type DedupStats struct {
	TotalFiles      int     `json:"totalFiles"`      // 总文件数
	UniqueFiles     int     `json:"uniqueFiles"`     // 唯一文件数
	DuplicateFiles  int     `json:"duplicateFiles"`  // 重复文件数
	DedupPercent    float64 `json:"dedupPercent"`    // 去重百分比
	TotalBlocks     int     `json:"totalBlocks"`     // 总块数
	UniqueBlocks    int     `json:"uniqueBlocks"`    // 唯一块数
	BlockDedupRatio float64 `json:"blockDedupRatio"` // 块去重率
	SpaceSavedBytes int64   `json:"spaceSavedBytes"` // 去重节省字节
}

// Suggestion 优化建议.
type Suggestion struct {
	ID          string `json:"id"`          // 建议唯一标识
	Type        string `json:"type"`        // 建议类型：compression/dedup/tiering/cost
	Priority    string `json:"priority"`    // 优先级：high/medium/low
	Title       string `json:"title"`       // 建议标题
	Description string `json:"description"` // 建议详情
	PotentialMB int64  `json:"potentialMB"` // 潜在节省空间（MB）
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Date             string  `json:"date"`             // 日期（YYYY-MM-DD）
	LogicalSize      int64   `json:"logicalSize"`      // 当日逻辑大小
	PhysicalSize     int64   `json:"physicalSize"`     // 当日物理大小
	SpaceSaved       int64   `json:"spaceSaved"`       // 当日节省空间
	CompressionRatio float64 `json:"compressionRatio"` // 当日压缩率
	DedupRatio       float64 `json:"dedupRatio"`       // 当日去重率
}

// TrendData 趋势数据集.
type TrendData struct {
	Days   int          `json:"days"`   // 天数
	Points []TrendPoint `json:"points"` // 数据点列表
}

// AnalyzeRequest 触发分析请求.
type AnalyzeRequest struct {
	Path       string `json:"path"`                 // 分析路径（可选，默认全部）
	SampleRate int    `json:"sampleRate,omitempty"` // 采样率（1-100，默认10）
	DeepScan   bool   `json:"deepScan,omitempty"`   // 是否深度扫描
}

// AnalyzeResult 分析任务结果.
type AnalyzeResult struct {
	TaskID    string    `json:"taskId"`    // 任务ID
	Status    string    `json:"status"`    // 任务状态：running/completed/failed
	StartedAt time.Time `json:"startedAt"` // 开始时间
	Path      string    `json:"path"`      // 分析路径
	Message   string    `json:"message"`   // 状态消息
}

// FileCompressInfo 单文件压缩信息.
type FileCompressInfo struct {
	Path             string  `json:"path"`             // 文件路径
	OriginalSize     int64   `json:"originalSize"`     // 原始大小
	CompressedSize   int64   `json:"compressedSize"`   // 压缩后大小
	CompressionRatio float64 `json:"compressionRatio"` // 压缩率
	Algorithm        string  `json:"algorithm"`        // 压缩算法
}

// FileDedupInfo 单文件去重信息.
type FileDedupInfo struct {
	Path           string `json:"path"`           // 文件路径
	Size           int64  `json:"size"`           // 文件大小
	Hash           string `json:"hash"`           // 文件哈希
	DuplicateCount int    `json:"duplicateCount"` // 重复次数
}

// internalRecord 内部效率记录，用于趋势存储.
type internalRecord struct {
	Timestamp        time.Time `json:"timestamp"`
	LogicalSize      int64     `json:"logicalSize"`
	PhysicalSize     int64     `json:"physicalSize"`
	CompressionRatio float64   `json:"compressionRatio"`
	DedupRatio       float64   `json:"dedupRatio"`
	SpaceSaved       int64     `json:"spaceSaved"`
}

// suggestType 建议类型常量.
const (
	SuggestTypeCompression = "compression"
	SuggestTypeDedup       = "dedup"
	SuggestTypeTiering     = "tiering"
	SuggestTypeCost        = "cost"
)

// priorityLevel 优先级常量.
const (
	PriorityHigh   = "high"
	PriorityMedium = "medium"
	PriorityLow    = "low"
)

// taskStatus 任务状态常量.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// ========== 新增：去重检测和清理建议类型 ==========

// DuplicateGroup 重复文件组.
type DuplicateGroup struct {
	Hash      string            `json:"hash"`      // 文件哈希
	Size      int64             `json:"size"`      // 文件大小
	Count     int               `json:"count"`     // 重复数量
	TotalSize int64             `json:"totalSize"` // 总占用空间
	Files     []DuplicateFile   `json:"files"`     // 重复文件列表
	CanDelete []CleanupSuggestion `json:"canDelete"` // 可删除建议
}

// DuplicateFile 重复文件信息.
type DuplicateFile struct {
	Path    string    `json:"path"`    // 文件路径
	Size    int64     `json:"size"`    // 文件大小
	ModTime time.Time `json:"modTime"` // 修改时间
	IsOldest bool     `json:"isOldest"` // 是否最旧（建议保留）
}

// CleanupSuggestion 清理建议.
type CleanupSuggestion struct {
	FilePath    string `json:"filePath"`    // 建议删除的文件
	Reason      string `json:"reason"`      // 删除原因
	SavedBytes  int64  `json:"savedBytes"`  // 可节省空间
	KeepFile    string `json:"keepFile"`    // 建议保留的文件
}

// DuplicateDetectionResult 去重检测结果.
type DuplicateDetectionResult struct {
	TotalFiles      int               `json:"totalFiles"`      // 扫描文件总数
	DuplicateGroups int               `json:"duplicateGroups"` // 重复组数
	DuplicateFiles  int               `json:"duplicateFiles"`  // 重复文件数
	TotalWasted     int64             `json:"totalWasted"`     // 浪费空间总计
	Groups          []DuplicateGroup  `json:"groups"`          // 重复文件组
	Suggestions     []CleanupSuggestion `json:"suggestions"` // 清理建议
}

// ========== 新增：存储空间使用分析类型 ==========

// UsageByType 按文件类型统计.
type UsageByType struct {
	Extension    string  `json:"extension"`    // 文件扩展名
	FileCount    int     `json:"fileCount"`    // 文件数量
	TotalSize    int64   `json:"totalSize"`    // 总大小
	Percent      float64 `json:"percent"`      // 占总空间百分比
	AvgSize      int64   `json:"avgSize"`      // 平均文件大小
	LargestSize  int64   `json:"largestSize"`  // 最大文件大小
	LargestPath  string  `json:"largestPath"`  // 最大文件路径
}

// UsageByUser 按用户统计.
type UsageByUser struct {
	Username    string  `json:"username"`    // 用户名
	UID         int     `json:"uid"`         // 用户ID
	FileCount   int     `json:"fileCount"`   // 文件数量
	TotalSize   int64   `json:"totalSize"`   // 总大小
	Percent     float64 `json:"percent"`     // 占总空间百分比
	HomeDir     string  `json:"homeDir"`     // 主目录
}

// UsageByTime 按时间统计.
type UsageByTime struct {
	Period      string `json:"period"`      // 时间段（如 "2024-01", "2024-W01"）
	FileCount   int    `json:"fileCount"`   // 文件数量
	TotalSize   int64  `json:"totalSize"`   // 总大小
	NewFiles    int    `json:"newFiles"`    // 新增文件数
	DeletedFiles int   `json:"deletedFiles"` // 删除文件数（估算）
}

// StorageUsageAnalysis 存储空间使用分析结果.
type StorageUsageAnalysis struct {
	ScanPath      string          `json:"scanPath"`      // 扫描路径
	TotalFiles    int             `json:"totalFiles"`    // 文件总数
	TotalSize     int64           `json:"totalSize"`     // 总大小
	ByType        []UsageByType   `json:"byType"`        // 按类型统计
	ByUser        []UsageByUser   `json:"byUser"`        // 按用户统计
	ByTime        []UsageByTime   `json:"byTime"`        // 按时间统计
	ScanTime      time.Time       `json:"scanTime"`      // 扫描时间
	Duration      time.Duration   `json:"duration"`      // 扫描耗时
}

// UsageAnalysisRequest 使用分析请求.
type UsageAnalysisRequest struct {
	Path      string `json:"path"`                // 分析路径
	GroupBy   string `json:"groupBy,omitempty"`   // 分组方式：type/user/time/all
	TimeGranularity string `json:"timeGranularity,omitempty"` // 时间粒度：day/week/month
}

// ========== 新增：存储成本估算类型 ==========

// StorageTier 存储层.
type StorageTier string

const (
	TierSSD   StorageTier = "ssd"   // SSD 层
	TierHDD   StorageTier = "hdd"   // HDD 层
	TierCloud StorageTier = "cloud" // 云存储层
	TierTape  StorageTier = "tape"  // 磁带归档层
)

// TierCost 存储层成本.
type TierCost struct {
	Tier         StorageTier `json:"tier"`         // 存储层
	CostPerGB    float64     `json:"costPerGB"`    // 每GB月成本（元）
	CostPerTB    float64     `json:"costPerTB"`    // 每TB月成本（元）
	TotalSizeGB  float64     `json:"totalSizeGB"`  // 该层总容量(GB)
	MonthlyCost  float64     `json:"monthlyCost"`  // 月度成本
	YearlyCost   float64     `json:"yearlyCost"`   // 年度成本
}

// CostEstimate 存储成本估算结果.
type CostEstimate struct {
	TotalSizeGB     float64     `json:"totalSizeGB"`     // 总存储量(GB)
	EffectiveSizeGB float64     `json:"effectiveSizeGB"` // 有效存储量(GB，压缩去重后)
	SavingsPercent  float64     `json:"savingsPercent"`  // 节省百分比
	Tiers           []TierCost  `json:"tiers"`           // 各层成本
	TotalMonthly    float64     `json:"totalMonthly"`    // 总月度成本
	TotalYearly     float64     `json:"totalYearly"`     // 总年度成本
	SavingsMonthly  float64     `json:"savingsMonthly"`  // 月度节省
	SavingsYearly   float64     `json:"savingsYearly"`   // 年度节省
	Currency        string      `json:"currency"`        // 货币单位
	EstimatedAt     time.Time   `json:"estimatedAt"`     // 估算时间
}

// CostEstimateRequest 成本估算请求.
type CostEstimateRequest struct {
	Path        string             `json:"path"`        // 分析路径
	TierCosts   map[string]float64 `json:"tierCosts"`   // 各层每GB月成本（可选）
	Currency    string             `json:"currency"`    // 货币单位
}
