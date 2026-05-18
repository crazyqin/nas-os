// Package smartpricing 提供智能存储定价分析功能
// 基于容量、性能、副本策略等维度计算存储成本，并推荐最优存储方案
package smartpricing

import (
	"fmt"
	"sync"
	"time"
)

// StorageTier 存储层级类型.
type StorageTier string

const (
	TierSSD    StorageTier = "ssd"    // 固态硬盘（高性能）
	TierHDD    StorageTier = "hdd"    // 机械硬盘（大容量）
	TierHybrid StorageTier = "hybrid" // 混合存储（SSD缓存 + HDD存储）
)

// ReplicaPolicy 副本策略.
type ReplicaPolicy string

const (
	ReplicaNone   ReplicaPolicy = "none"   // 无副本
	ReplicaMirror ReplicaPolicy = "mirror"  // 镜像（2副本）
	ReplicaRAID5  ReplicaPolicy = "raid5"  // RAID5（单盘容错）
	ReplicaRAID6  ReplicaPolicy = "raid6"  // RAID6（双盘容错）
	ReplicaTriple ReplicaPolicy = "triple" // 三副本
)

// WorkloadType 工作负载类型.
type WorkloadType string

const (
	WorkloadCold    WorkloadType = "cold"    // 冷数据（归档、备份）
	WorkloadWarm    WorkloadType = "warm"    // 温数据（偶尔访问）
	WorkloadHot     WorkloadType = "hot"     // 热数据（频繁读写）
	WorkloadMixed   WorkloadType = "mixed"   // 混合负载
)

// PricingPlan 定价方案.
type PricingPlan struct {
	ID          string      `json:"id"`          // 方案唯一标识
	Name        string      `json:"name"`        // 方案名称
	Tier        StorageTier `json:"tier"`        // 存储层级
	Replica     ReplicaPolicy `json:"replica"`   // 副本策略
	UnitPrice   float64     `json:"unitPrice"`   // 单价（元/GB/月）
	MinCapacity int64       `json:"minCapacity"` // 最小容量（GB）
	MaxCapacity int64       `json:"maxCapacity"` // 最大容量（GB，0=无限制）
	IOPSLimit   int         `json:"iopsLimit"`   // IOPS 上限（0=无限制）
	ThroughputMB int        `json:"throughputMB"` // 吞吐量上限（MB/s，0=无限制）

	// 性能特征
	ReadLatencyMs  float64 `json:"readLatencyMs"`  // 读延迟（毫秒）
	WriteLatencyMs float64 `json:"writeLatencyMs"` // 写延迟（毫秒）

	// 额外费用
	MonthlyBaseFee float64 `json:"monthlyBaseFee"` // 月基础费（元）
	TransferFee    float64 `json:"transferFee"`    // 数据传输费（元/GB）

	Description string `json:"description"` // 方案描述
}

// CostBreakdown 成本明细.
type CostBreakdown struct {
	StorageCost    float64 `json:"storageCost"`    // 存储费用
	ReplicaCost    float64 `json:"replicaCost"`    // 副本费用
	TransferCost   float64 `json:"transferCost"`   // 传输费用
	BaseFee        float64 `json:"baseFee"`        // 基础费用
	TotalCost      float64 `json:"totalCost"`      // 总费用
	EffectivePerGB float64 `json:"effectivePerGB"` // 有效单价（元/GB/月）
}

// CostAnalysis 成本分析结果.
type CostAnalysis struct {
	mu sync.RWMutex `json:"-"`

	// 分析目标
	TotalCapacityGB int64          `json:"totalCapacityGB"` // 总容量需求（GB）
	Tier            StorageTier    `json:"tier"`            // 存储层级
	Replica         ReplicaPolicy  `json:"replica"`         // 副本策略
	Workload        WorkloadType   `json:"workload"`        // 工作负载

	// 月度成本
	MonthlyCost CostBreakdown `json:"monthlyCost"`

	// 年度成本
	AnnualCost CostBreakdown `json:"annualCost"`

	// 三年成本（TCO）
	ThreeYearCost CostBreakdown `json:"threeYearCost"`

	// 性能指标
	EffectiveIOPS      int     `json:"effectiveIOPS"`      // 有效 IOPS
	EffectiveThroughput int    `json:"effectiveThroughput"` // 有效吞吐量（MB/s）
	ReadLatencyMs      float64 `json:"readLatencyMs"`      // 读延迟
	WriteLatencyMs     float64 `json:"writeLatencyMs"`     // 写延迟

	// 副本开销
	ReplicaOverhead float64 `json:"replicaOverhead"` // 副本系数（1.0=无副本，2.0=双副本）
	UsableRatio     float64 `json:"usableRatio"`     // 可用容量比率

	// 元信息
	AnalyzedAt time.Time `json:"analyzedAt"` // 分析时间
	PlanUsed   string    `json:"planUsed"`   // 使用的方案名称
}

// Recommendation 存储推荐.
type Recommendation struct {
	Rank         int            `json:"rank"`         // 排名（1=最优）
	Plan         PricingPlan    `json:"plan"`         // 推荐方案
	TotalCost    float64        `json:"totalCost"`    // 月度总成本
	CostPerGB    float64        `json:"costPerGB"`    // 每GB成本
	Score        float64        `json:"score"`        // 综合评分（0-100）
	Reasons      []string       `json:"reasons"`      // 推荐理由
	Warnings     []string       `json:"warnings"`     // 警告信息
	CostAnalysis *CostAnalysis  `json:"costAnalysis"` // 详细成本分析
}

// OptimizeRequest 优化请求.
type OptimizeRequest struct {
	CapacityGB  int64         `json:"capacityGB"`  // 容量需求（GB）
	Workload    WorkloadType  `json:"workload"`    // 工作负载类型
	Replica     ReplicaPolicy `json:"replica"`     // 副本策略
	MaxBudget   float64       `json:"maxBudget"`   // 最大月预算（元，0=不限）
	MinIOPS     int           `json:"minIOPS"`     // 最低 IOPS 要求
	MaxLatencyMs float64      `json:"maxLatencyMs"` // 最大延迟要求（毫秒）
}

// OptimizeResult 优化结果.
type OptimizeResult struct {
	Request       OptimizeRequest  `json:"request"`       // 原始请求
	Recommendations []Recommendation `json:"recommendations"` // 推荐列表
	BestOption    *Recommendation  `json:"bestOption"`    // 最优选项
	GeneratedAt   time.Time        `json:"generatedAt"`   // 生成时间
}

// ReportType 报告类型.
type ReportType string

const (
	ReportMonthly ReportType = "monthly" // 月度报告
	ReportAnnual  ReportType = "annual"  // 年度报告
)

// CostReport 成本报告.
type CostReport struct {
	ReportID    string     `json:"reportId"`    // 报告ID
	ReportType  ReportType `json:"reportType"`  // 报告类型
	Title       string     `json:"title"`       // 报告标题
	GeneratedAt time.Time  `json:"generatedAt"` // 生成时间
	PeriodStart time.Time  `json:"periodStart"` // 报告周期开始
	PeriodEnd   time.Time  `json:"periodEnd"`   // 报告周期结束

	// 存储概览
	TotalCapacityGB int64 `json:"totalCapacityGB"` // 总容量
	UsedCapacityGB  int64 `json:"usedCapacityGB"`  // 已用容量
	UsageRatio      float64 `json:"usageRatio"`    // 使用率

	// 成本汇总
	TotalCost    float64 `json:"totalCost"`    // 总成本
	StorageCost  float64 `json:"storageCost"`  // 存储费用
	ReplicaCost  float64 `json:"replicaCost"`  // 副本费用
	TransferCost float64 `json:"transferCost"` // 传输费用

	// 各层级成本
	TierBreakdown []TierCostSummary `json:"tierBreakdown"`

	// 优化建议
	Suggestions []string `json:"suggestions"` // 优化建议
}

// TierCostSummary 层级成本摘要.
type TierCostSummary struct {
	Tier         StorageTier `json:"tier"`         // 存储层级
	CapacityGB   int64       `json:"capacityGB"`   // 容量
	Cost         float64     `json:"cost"`         // 成本
	CostPerGB    float64     `json:"costPerGB"`    // 每GB成本
	UsageRatio   float64     `json:"usageRatio"`   // 使用率
}

// EstimateCost 估算成本（根据层级和容量）.
func EstimateCost(tier StorageTier, capacityGB int64, replica ReplicaPolicy) *CostBreakdown {
	// 基础单价（元/GB/月）
	var unitPrice float64
	switch tier {
	case TierSSD:
		unitPrice = 1.5 // SSD 较贵
	case TierHDD:
		unitPrice = 0.3 // HDD 经济实惠
	case TierHybrid:
		unitPrice = 0.8 // 混合存储适中
	default:
		unitPrice = 0.5
	}

	// 副本系数
	var replicaMultiplier float64
	switch replica {
	case ReplicaNone:
		replicaMultiplier = 1.0
	case ReplicaMirror:
		replicaMultiplier = 2.0
	case ReplicaRAID5:
		replicaMultiplier = 1.33 // 约 33% 开销
	case ReplicaRAID6:
		replicaMultiplier = 1.5  // 约 50% 开销
	case ReplicaTriple:
		replicaMultiplier = 3.0
	default:
		replicaMultiplier = 1.0
	}

	storageCost := float64(capacityGB) * unitPrice
	replicaCost := storageCost * (replicaMultiplier - 1.0)
	totalCost := storageCost + replicaCost

	return &CostBreakdown{
		StorageCost:    storageCost,
		ReplicaCost:    replicaCost,
		TransferCost:   0,
		BaseFee:        0,
		TotalCost:      totalCost,
		EffectivePerGB: unitPrice * replicaMultiplier,
	}
}

// GetReplicaOverhead 获取副本开销系数.
func GetReplicaOverhead(replica ReplicaPolicy) float64 {
	switch replica {
	case ReplicaNone:
		return 1.0
	case ReplicaMirror:
		return 2.0
	case ReplicaRAID5:
		return 1.33
	case ReplicaRAID6:
		return 1.5
	case ReplicaTriple:
		return 3.0
	default:
		return 1.0
	}
}

// GetUsableRatio 获取可用容量比率.
func GetUsableRatio(replica ReplicaPolicy) float64 {
	overhead := GetReplicaOverhead(replica)
	if overhead > 0 {
		return 1.0 / overhead
	}
	return 1.0
}

// String 返回存储层级的中文描述.
func (t StorageTier) String() string {
	switch t {
	case TierSSD:
		return "固态硬盘（SSD）"
	case TierHDD:
		return "机械硬盘（HDD）"
	case TierHybrid:
		return "混合存储"
	default:
		return string(t)
	}
}

// String 返回副本策略的中文描述.
func (r ReplicaPolicy) String() string {
	switch r {
	case ReplicaNone:
		return "无副本"
	case ReplicaMirror:
		return "镜像（双副本）"
	case ReplicaRAID5:
		return "RAID5（单盘容错）"
	case ReplicaRAID6:
		return "RAID6（双盘容错）"
	case ReplicaTriple:
		return "三副本"
	default:
		return string(r)
	}
}

// String 返回工作负载类型的中文描述.
func (w WorkloadType) String() string {
	switch w {
	case WorkloadCold:
		return "冷数据"
	case WorkloadWarm:
		return "温数据"
	case WorkloadHot:
		return "热数据"
	case WorkloadMixed:
		return "混合负载"
	default:
		return string(w)
	}
}

// Validate 验证优化请求.
func (r *OptimizeRequest) Validate() error {
	if r.CapacityGB <= 0 {
		return fmt.Errorf("capacityGB must be positive")
	}
	if r.Workload == "" {
		r.Workload = WorkloadMixed
	}
	if r.Replica == "" {
		r.Replica = ReplicaNone
	}
	if r.MaxBudget < 0 {
		r.MaxBudget = 0
	}
	if r.MinIOPS < 0 {
		r.MinIOPS = 0
	}
	if r.MaxLatencyMs < 0 {
		r.MaxLatencyMs = 0
	}
	return nil
}
