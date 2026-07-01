// Package tcocalc 提供TCO总拥有成本分析能力
// 参考群晖"买硬件送软件"策略与云订阅模式对比，
// 支持硬件成本、电力成本、维护成本、软件许可成本计算，
// 5年TCO预测及与云存储方案对比分析
package tcocalc

import "time"

// TCOCategory TCO成本分类
type TCOCategory string

const (
	CategoryHardware  TCOCategory = "hardware"    // 硬件成本
	CategoryPower     TCOCategory = "power"       // 电力成本
	CategoryMaint     TCOCategory = "maintenance"  // 维护成本
	CategoryLicense   TCOCategory = "license"      // 软件许可成本
	CategoryCloud     TCOCategory = "cloud"       // 云订阅成本
)

// HardwareSpec 硬件规格
type HardwareSpec struct {
	Name     string  `json:"name"`      // 硬件名称（如 NAS主机、磁盘扩展柜）
	Price    float64 `json:"price"`     // 采购价格（元）
	Quantity int     `json:"quantity"`  // 数量
	Lifespan int     `json:"lifespan"`  // 预期寿命（年）
}

// PowerSpec 电力规格
type PowerSpec struct {
	Watts      float64 `json:"watts"`        // 功耗（瓦）
	HoursPerDay int    `json:"hours_per_day"` // 每日运行小时数
	PriceKWh   float64 `json:"price_kwh"`    // 电费单价（元/kWh）
}

// MaintenanceSpec 维护规格
type MaintenanceSpec struct {
	AnnualCost   float64 `json:"annual_cost"`    // 年度维护费用（元）
	ReplacementCost float64 `json:"replacement_cost"` // 硬件更换费用（元）
	ReplaceInterval int   `json:"replace_interval"`  // 更换周期（年）
}

// LicenseSpec 软件许可规格
type LicenseSpec struct {
	Name       string  `json:"name"`        // 许可名称
	Type       string  `json:"type"`        // 许可类型：perpetual（永久）/ subscription（订阅）
	Price      float64 `json:"price"`       // 许可费用（元）
	AnnualFee  float64 `json:"annual_fee"`  // 年度订阅费（元），仅订阅类型
	Quantity   int     `json:"quantity"`    // 数量
}

// CostBreakdown 成本明细
type CostBreakdown struct {
	Category    TCOCategory `json:"category"`      // 成本分类
	Yearly       float64     `json:"yearly"`       // 年度成本（元）
	FiveYearTotal float64    `json:"five_year_total"` // 5年累计成本（元）
	Items        []CostItem  `json:"items"`        // 成本子项
}

// CostItem 成本子项
type CostItem struct {
	Name   string  `json:"name"`    // 项目名称
	Yearly float64 `json:"yearly"`  // 年度成本（元）
	FiveYearTotal float64 `json:"five_year_total"` // 5年累计成本（元）
}

// CloudComparison 云存储方案对比
type CloudComparison struct {
	ProviderName   string  `json:"provider_name"`    // 云服务商名称
	StorageSizeTB  float64 `json:"storage_size_tb"`  // 存储容量（TB）
	MonthlyCost    float64 `json:"monthly_cost"`      // 月度费用（元/月）
	AnnualCost    float64 `json:"annual_cost"`        // 年度费用（元/年）
	FiveYearCost  float64 `json:"five_year_cost"`     // 5年总费用（元）
	EgressCost    float64 `json:"egress_cost"`        // 预计出口流量费（元/年）
	APICallCost   float64 `json:"api_call_cost"`      // API请求费（元/年）
}

// TCOItem TCO单项
type TCOItem struct {
	Category    TCOCategory `json:"category"`     // 成本分类
	Name        string      `json:"name"`         // 项目名称
	UpfrontCost float64     `json:"upfront_cost"`  // 一次性 upfront 成本（元）
	AnnualCost  float64     `json:"annual_cost"`   // 年度成本（元）
}

// TCOReport TCO分析报告
type TCOReport struct {
	GeneratedAt     time.Time        `json:"generated_at"`      // 生成时间
	Years           int             `json:"years"`             // 分析年限
	Hardware        []HardwareSpec   `json:"hardware"`          // 硬件配置
	Power           PowerSpec        `json:"power"`             // 电力配置
	Maintenance     MaintenanceSpec   `json:"maintenance"`      // 维护配置
	Licenses        []LicenseSpec    `json:"licenses"`         // 软件许可配置

	// 成本明细
	Breakdowns      []CostBreakdown  `json:"breakdowns"`       // 各分类成本明细
	Items           []TCOItem         `json:"items"`            // TCO单项列表

	// 汇总
	NASFiveYearTotal float64    `json:"nas_five_year_total"`  // NAS方案5年总成本（元）
	NASAnnualAverage float64    `json:"nas_annual_average"`  // NAS方案年均成本（元/年）
	NASUpfrontTotal  float64    `json:"nas_upfront_total"`   // NAS方案一次性成本（元）

	// 云方案对比
	CloudComparisons []CloudComparison `json:"cloud_comparisons"` // 云存储方案对比
	CheaperChoice    string             `json:"cheaper_choice"`     // 更经济的选择：nas / cloud
	SavingsAmount    float64            `json:"savings_amount"`     // 5年节省金额（元）
	SavingsPercent   float64            `json:"savings_percent"`    // 节省百分比

	// 每TB成本
	NASCostPerTB     float64 `json:"nas_cost_per_tb"`     // NAS每TB成本（元/TB/年）
	CloudCostPerTB   float64 `json:"cloud_cost_per_tb"`   // 云每TB成本（元/TB/年）
}