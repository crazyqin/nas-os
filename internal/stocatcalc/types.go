// Package stocatcalc 提供存储总拥有成本（TCO）计算能力
// 支持不同存储方案（纯HDD、混合、纯SSD）的成本对比分析
package stocatcalc

import "time"

// DiskType 磁盘类型.
type DiskType string

const (
	DiskTypeHDD  DiskType = "hdd"  // 机械硬盘
	DiskTypeSSD  DiskType = "ssd"  // 固态硬盘
	DiskTypeNVMe DiskType = "nvme" // NVMe固态硬盘
)

// StorageScheme 存储方案类型.
type StorageScheme string

const (
	SchemePureHDD  StorageScheme = "pure_hdd"  // 纯HDD方案
	SchemeHybrid   StorageScheme = "hybrid"    // 混合方案（HDD+SSD）
	SchemePureSSD  StorageScheme = "pure_ssd"  // 纯SSD方案
	SchemePureNVMe StorageScheme = "pure_nvme" // 纯NVMe方案
)

// DiskSpec 磁盘规格.
type DiskSpec struct {
	Type       DiskType `json:"type"`        // 磁盘类型
	CapacityTB float64  `json:"capacity_tb"` // 单盘容量（TB）
	Price      float64  `json:"price"`       // 单盘价格（元）
	Quantity   int      `json:"quantity"`    // 数量
}

// CalcRequest 计算请求.
type CalcRequest struct {
	Disks         []DiskSpec `json:"disks"`           // 磁盘配置列表
	PowerPriceKWh float64    `json:"power_price_kwh"` // 电费单价（元/kWh）
	Years         int        `json:"years"`           // 使用年限
	RaidLevel     string     `json:"raid_level"`      // RAID级别（影响可用容量）
}

// CalcResult 单方案计算结果.
type CalcResult struct {
	Scheme           StorageScheme `json:"scheme"`             // 存储方案类型
	HardwareCost     float64       `json:"hardware_cost"`      // 硬件总成本（元）
	PowerCost        float64       `json:"power_cost"`         // 电力总成本（元）
	TotalCost        float64       `json:"total_cost"`         // 总拥有成本（元）
	RawCapacityTB    float64       `json:"raw_capacity_tb"`    // 原始总容量（TB）
	UsableCapacityTB float64       `json:"usable_capacity_tb"` // 可用容量（TB）
	CostPerTB        float64       `json:"cost_per_tb"`        // 每TB成本（元/TB）
	AnnualCost       float64       `json:"annual_cost"`        // 年化成本（元/年）
	MonthlyCost      float64       `json:"monthly_cost"`       // 月度成本（元/月")
	PowerWatts       float64       `json:"power_watts"`        // 总功耗（瓦）
	Disks            []DiskSpec    `json:"disks"`              // 磁盘配置
}

// ComparisonResult 方案对比结果.
type ComparisonResult struct {
	GeneratedAt time.Time    `json:"generated_at"`   // 生成时间
	Years       int          `json:"years"`          // 使用年限
	Results     []CalcResult `json:"results"`        // 各方案结果
	BestByTotal *CalcResult  `json:"best_by_total"`  // 总成本最低方案
	BestByPerTB *CalcResult  `json:"best_by_per_tb"` // 每TB成本最低方案
}

// Template 存储方案模板.
type Template struct {
	ID          string        `json:"id"`          // 模板ID
	Name        string        `json:"name"`        // 模板名称
	Scheme      StorageScheme `json:"scheme"`      // 方案类型
	Description string        `json:"description"` // 描述
	Disks       []DiskSpec    `json:"disks"`       // 磁盘配置
	RaidLevel   string        `json:"raid_level"`  // RAID级别
}

// powerTable 各磁盘类型的典型功耗（瓦/盘）.
var powerTable = map[DiskType]float64{
	DiskTypeHDD:  6.5, // HDD 典型功耗约6.5W
	DiskTypeSSD:  3.0, // SATA SSD 典型功耗约3W
	DiskTypeNVMe: 8.0, // NVMe SSD 典型功耗约8W
}

// raidEfficiencyTable RAID级别对应的可用容量比例.
var raidEfficiencyTable = map[string]float64{
	"none":   1.0,   // 无RAID
	"raid0":  1.0,   // RAID0：100%
	"raid1":  0.5,   // RAID1：50%
	"raid5":  0.75,  // RAID5（4盘）：75%
	"raid6":  0.667, // RAID6（6盘）：约66.7%
	"raid10": 0.5,   // RAID10：50%
}
