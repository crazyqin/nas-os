// Package reports 提供报表生成和管理功能
package reports

import (
	"time"
)

// ========== 云存储成本对比 v2.60.0 ==========

// CloudProvider 云服务商.
type CloudProvider string

const (
	// CloudProviderAWS represents AWS cloud provider.
	CloudProviderAWS CloudProvider = "aws"
	// CloudProviderAliyun represents Aliyun cloud provider.
	CloudProviderAliyun CloudProvider = "aliyun"
	// CloudProviderTencent represents Tencent cloud provider.
	CloudProviderTencent CloudProvider = "tencent"
	// CloudProviderHuawei represents Huawei cloud provider.
	CloudProviderHuawei CloudProvider = "huawei"
	// CloudProviderAzure represents Azure cloud provider.
	CloudProviderAzure CloudProvider = "azure"
	// CloudProviderGoogle represents Google Cloud provider.
	CloudProviderGoogle CloudProvider = "google"
)

// CloudStorageTier 云存储层级.
type CloudStorageTier string

const (
	// TierStandard represents standard storage tier.
	TierStandard CloudStorageTier = "standard"
	// TierIA represents infrequent access storage tier.
	TierIA CloudStorageTier = "ia" // 低频访问
	// TierArchive represents archive storage tier.
	TierArchive CloudStorageTier = "archive" // 归档
	// TierDeepArchive represents deep archive storage tier.
	TierDeepArchive CloudStorageTier = "deep_archive" // 深度归档
)

// CloudPricing 云存储定价.
type CloudPricing struct {
	// 云服务商
	Provider CloudProvider `json:"provider"`

	// 存储层级
	Tier CloudStorageTier `json:"tier"`

	// 存储费用（元/GB/月）
	StoragePricePerGB float64 `json:"storage_price_per_gb"`

	// 请求费用（元/万次）
	RequestPricePer10K float64 `json:"request_price_per_10k"`

	// 下行流量费用（元/GB）
	EgressPricePerGB float64 `json:"egress_price_per_gb"`

	// 最小存储时长（天）
	MinimumStorageDays int `json:"minimum_storage_days"`

	// 数据取回费用（元/GB）
	RetrievalPricePerGB float64 `json:"retrieval_price_per_gb"`

	// 区域
	Region string `json:"region"`

	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// CloudCostEstimate 云存储成本估算.
type CloudCostEstimate struct {
	// 云服务商
	Provider CloudProvider `json:"provider"`

	// 存储层级
	Tier CloudStorageTier `json:"tier"`

	// 存储容量（GB）
	StorageGB float64 `json:"storage_gb"`

	// 存储费用（元/月）
	StorageCost float64 `json:"storage_cost"`

	// 请求费用（元/月）
	RequestCost float64 `json:"request_cost"`

	// 流量费用（元/月）
	EgressCost float64 `json:"egress_cost"`

	// 总费用（元/月）
	TotalCost float64 `json:"total_cost"`

	// 年费用（元/年）
	YearlyCost float64 `json:"yearly_cost"`

	// 单位成本（元/GB/月）
	CostPerGB float64 `json:"cost_per_gb"`

	// 与自建成本对比
	SavingsPercent float64 `json:"savings_percent"` // 节省百分比（负数表示云存储更贵）

	// 说明
	Notes string `json:"notes"`
}

// CloudComparisonReport 云存储对比报告.
type CloudComparisonReport struct {
	// 报告ID
	ID string `json:"id"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 存储容量（GB）
	StorageCapacityGB float64 `json:"storage_capacity_gb"`

	// 月均请求数（万次）
	MonthlyRequests10K float64 `json:"monthly_requests_10k"`

	// 月均下行流量（GB）
	MonthlyEgressGB float64 `json:"monthly_egress_gb"`

	// 自建存储成本
	SelfHostedCost *StorageCost `json:"self_hosted_cost"`

	// 云存储估算列表
	CloudEstimates []CloudCostEstimate `json:"cloud_estimates"`

	// 推荐方案
	Recommendation CloudRecommendation `json:"recommendation"`

	// 对比摘要
	Summary CloudComparisonSummary `json:"summary"`
}

// CloudRecommendation 云存储推荐.
type CloudRecommendation struct {
	// 推荐方案
	RecommendedType string `json:"recommended_type"` // self_hosted, cloud, hybrid

	// 推荐云服务商
	RecommendedProvider CloudProvider `json:"recommended_provider,omitempty"`

	// 推荐存储层级
	RecommendedTier CloudStorageTier `json:"recommended_tier,omitempty"`

	// 节省金额（元/月）
	SavingsPerMonth float64 `json:"savings_per_month"`

	// 节省金额（元/年）
	SavingsPerYear float64 `json:"savings_per_year"`

	// 节省百分比
	SavingsPercent float64 `json:"savings_percent"`

	// 理由
	Reason string `json:"reason"`

	// 注意事项
	Cautions []string `json:"cautions"`
}

// CloudComparisonSummary 云存储对比摘要.
type CloudComparisonSummary struct {
	// 自建月成本
	SelfHostedMonthly float64 `json:"self_hosted_monthly"`

	// 最低云存储月成本
	CloudMinMonthly float64 `json:"cloud_min_monthly"`

	// 最高云存储月成本
	CloudMaxMonthly float64 `json:"cloud_max_monthly"`

	// 平均云存储月成本
	CloudAvgMonthly float64 `json:"cloud_avg_monthly"`

	// 最优方案
	BestOption string `json:"best_option"`

	// 成本差异
	CostDifference float64 `json:"cost_difference"`
}

// CloudPricingRegistry 云存储定价注册表.
type CloudPricingRegistry struct {
	pricings map[string]CloudPricing
}

// NewCloudPricingRegistry 创建定价注册表.
func NewCloudPricingRegistry() *CloudPricingRegistry {
	r := &CloudPricingRegistry{
		pricings: make(map[string]CloudPricing),
	}

	// 初始化默认定价（2024年参考价格，单位：元）
	r.initDefaultPricing()

	return r
}

// initDefaultPricing 初始化默认定价.
func (r *CloudPricingRegistry) initDefaultPricing() {
	now := time.Now()

	// 阿里云 OSS 定价（华东1区域）
	r.SetPricing(CloudPricing{
		Provider:            CloudProviderAliyun,
		Tier:                TierStandard,
		StoragePricePerGB:   0.12, // 元/GB/月
		RequestPricePer10K:  0.01, // 元/万次
		EgressPricePerGB:    0.50, // 元/GB
		MinimumStorageDays:  0,
		RetrievalPricePerGB: 0,
		Region:              "cn-hangzhou",
		UpdatedAt:           now,
	})

	r.SetPricing(CloudPricing{
		Provider:            CloudProviderAliyun,
		Tier:                TierIA,
		StoragePricePerGB:   0.08, // 元/GB/月
		RequestPricePer10K:  0.1,  // 元/万次（低频访问请求费用更高）
		EgressPricePerGB:    0.50,
		MinimumStorageDays:  30,
		RetrievalPricePerGB: 0.032, // 数据取回费用
		Region:              "cn-hangzhou",
		UpdatedAt:           now,
	})

	r.SetPricing(CloudPricing{
		Provider:            CloudProviderAliyun,
		Tier:                TierArchive,
		StoragePricePerGB:   0.033, // 元/GB/月
		RequestPricePer10K:  0.1,
		EgressPricePerGB:    0.50,
		MinimumStorageDays:  60,
		RetrievalPricePerGB: 0.06, // 数据取回费用
		Region:              "cn-hangzhou",
		UpdatedAt:           now,
	})

	// 腾讯云 COS 定价
	r.SetPricing(CloudPricing{
		Provider:            CloudProviderTencent,
		Tier:                TierStandard,
		StoragePricePerGB:   0.118, // 元/GB/月
		RequestPricePer10K:  0.01,
		EgressPricePerGB:    0.50,
		MinimumStorageDays:  0,
		RetrievalPricePerGB: 0,
		Region:              "ap-guangzhou",
		UpdatedAt:           now,
	})

	r.SetPricing(CloudPricing{
		Provider:            CloudProviderTencent,
		Tier:                TierIA,
		StoragePricePerGB:   0.08,
		RequestPricePer10K:  0.05,
		EgressPricePerGB:    0.50,
		MinimumStorageDays:  30,
		RetrievalPricePerGB: 0.032,
		Region:              "ap-guangzhou",
		UpdatedAt:           now,
	})

	// 华为云 OBS 定价
	r.SetPricing(CloudPricing{
		Provider:            CloudProviderHuawei,
		Tier:                TierStandard,
		StoragePricePerGB:   0.099, // 元/GB/月
		RequestPricePer10K:  0.01,
		EgressPricePerGB:    0.50,
		MinimumStorageDays:  0,
		RetrievalPricePerGB: 0,
		Region:              "cn-east-3",
		UpdatedAt:           now,
	})

	// AWS S3 定价（中国区参考）
	r.SetPricing(CloudPricing{
		Provider:            CloudProviderAWS,
		Tier:                TierStandard,
		StoragePricePerGB:   0.18, // 元/GB/月（中国区较贵）
		RequestPricePer10K:  0.01,
		EgressPricePerGB:    0.90,
		MinimumStorageDays:  0,
		RetrievalPricePerGB: 0,
		Region:              "cn-north-1",
		UpdatedAt:           now,
	})

	r.SetPricing(CloudPricing{
		Provider:            CloudProviderAWS,
		Tier:                TierIA,
		StoragePricePerGB:   0.10,
		RequestPricePer10K:  0.01,
		EgressPricePerGB:    0.90,
		MinimumStorageDays:  30,
		RetrievalPricePerGB: 0.03,
		Region:              "cn-north-1",
		UpdatedAt:           now,
	})

	// Azure Blob 定价（中国区参考）
	r.SetPricing(CloudPricing{
		Provider:            CloudProviderAzure,
		Tier:                TierStandard,
		StoragePricePerGB:   0.15,
		RequestPricePer10K:  0.01,
		EgressPricePerGB:    0.80,
		MinimumStorageDays:  0,
		RetrievalPricePerGB: 0,
		Region:              "china-east",
		UpdatedAt:           now,
	})
}

// SetPricing 设置定价.
func (r *CloudPricingRegistry) SetPricing(pricing CloudPricing) {
	key := string(pricing.Provider) + "_" + string(pricing.Tier)
	r.pricings[key] = pricing
}

// GetPricing 获取定价.
func (r *CloudPricingRegistry) GetPricing(provider CloudProvider, tier CloudStorageTier) (CloudPricing, bool) {
	key := string(provider) + "_" + string(tier)
	pricing, ok := r.pricings[key]
	return pricing, ok
}

// GetAllPricings 获取所有定价.
func (r *CloudPricingRegistry) GetAllPricings() []CloudPricing {
	pricings := make([]CloudPricing, 0, len(r.pricings))
	for _, p := range r.pricings {
		pricings = append(pricings, p)
	}
	return pricings
}

// CloudCostCalculator 云存储成本计算器.
type CloudCostCalculator struct {
	registry *CloudPricingRegistry
}

// NewCloudCostCalculator 创建云存储成本计算器.
func NewCloudCostCalculator() *CloudCostCalculator {
	return &CloudCostCalculator{
		registry: NewCloudPricingRegistry(),
	}
}

// EstimateCost 估算云存储成本.
func (c *CloudCostCalculator) EstimateCost(
	provider CloudProvider,
	tier CloudStorageTier,
	storageGB float64,
	monthlyRequests10K float64,
	monthlyEgressGB float64,
) *CloudCostEstimate {
	pricing, ok := c.registry.GetPricing(provider, tier)
	if !ok {
		return nil
	}

	// 计算存储费用
	storageCost := pricing.StoragePricePerGB * storageGB

	// 计算请求费用
	requestCost := pricing.RequestPricePer10K * monthlyRequests10K

	// 计算流量费用
	egressCost := pricing.EgressPricePerGB * monthlyEgressGB

	// 总费用
	totalCost := storageCost + requestCost + egressCost

	// 年费用
	yearlyCost := totalCost * 12

	// 单位成本
	costPerGB := 0.0
	if storageGB > 0 {
		costPerGB = totalCost / storageGB
	}

	// 生成说明
	notes := c.generateNotes(pricing, storageGB, monthlyRequests10K, monthlyEgressGB)

	return &CloudCostEstimate{
		Provider:       provider,
		Tier:           tier,
		StorageGB:      storageGB,
		StorageCost:    round(storageCost, 2),
		RequestCost:    round(requestCost, 2),
		EgressCost:     round(egressCost, 2),
		TotalCost:      round(totalCost, 2),
		YearlyCost:     round(yearlyCost, 2),
		CostPerGB:      round(costPerGB, 4),
		SavingsPercent: 0, // 稍后计算
		Notes:          notes,
	}
}

// generateNotes 生成说明.
func (c *CloudCostCalculator) generateNotes(pricing CloudPricing, storageGB, requests, egress float64) string {
	notes := ""

	if pricing.MinimumStorageDays > 0 {
		notes += "最小存储时长: " + string(rune(pricing.MinimumStorageDays)) + "天; "
	}

	if pricing.RetrievalPricePerGB > 0 {
		notes += "数据取回费用: " + string(rune(pricing.RetrievalPricePerGB)) + "元/GB; "
	}

	if egress > storageGB*0.1 {
		notes += "流量费用较高，建议使用CDN或优化访问模式; "
	}

	return notes
}

// CompareWithCloud 对比自建与云存储成本.
func (c *CloudCostCalculator) CompareWithCloud(
	selfHostedCost *StorageCost,
	storageGB float64,
	monthlyRequests10K float64,
	monthlyEgressGB float64,
) *CloudComparisonReport {
	now := time.Now()

	// 获取所有云服务商定价
	pricings := c.registry.GetAllPricings()

	// 计算各云存储方案成本
	estimates := make([]CloudCostEstimate, 0)
	for _, pricing := range pricings {
		estimate := c.EstimateCost(
			pricing.Provider,
			pricing.Tier,
			storageGB,
			monthlyRequests10K,
			monthlyEgressGB,
		)
		if estimate != nil {
			// 计算与自建的对比
			if selfHostedCost != nil {
				estimate.SavingsPercent = round((selfHostedCost.TotalCost-estimate.TotalCost)/selfHostedCost.TotalCost*100, 1)
			}
			estimates = append(estimates, *estimate)
		}
	}

	// 生成推荐
	recommendation := c.generateRecommendation(selfHostedCost, estimates)

	// 生成摘要
	summary := c.generateSummary(selfHostedCost, estimates)

	return &CloudComparisonReport{
		ID:                 "cloud_compare_" + now.Format("20060102150405"),
		GeneratedAt:        now,
		StorageCapacityGB:  storageGB,
		MonthlyRequests10K: monthlyRequests10K,
		MonthlyEgressGB:    monthlyEgressGB,
		SelfHostedCost:     selfHostedCost,
		CloudEstimates:     estimates,
		Recommendation:     recommendation,
		Summary:            summary,
	}
}

// generateRecommendation 生成推荐.
func (c *CloudCostCalculator) generateRecommendation(selfHostedCost *StorageCost, estimates []CloudCostEstimate) CloudRecommendation {
	rec := CloudRecommendation{
		Cautions: make([]string, 0),
	}

	if len(estimates) == 0 || selfHostedCost == nil {
		rec.RecommendedType = "self_hosted"
		rec.Reason = "无法获取云存储定价信息"
		return rec
	}

	// 找出最低成本的云存储方案
	var minEstimate *CloudCostEstimate
	for i := range estimates {
		if minEstimate == nil || estimates[i].TotalCost < minEstimate.TotalCost {
			minEstimate = &estimates[i]
		}
	}

	selfHostedMonthly := selfHostedCost.TotalCost

	// 判断推荐方案
	if minEstimate.TotalCost < selfHostedMonthly*0.7 {
		// 云存储便宜30%以上
		rec.RecommendedType = "cloud"
		rec.RecommendedProvider = minEstimate.Provider
		rec.RecommendedTier = minEstimate.Tier
		rec.SavingsPerMonth = selfHostedMonthly - minEstimate.TotalCost
		rec.SavingsPerYear = rec.SavingsPerMonth * 12
		rec.Reason = "云存储成本明显更低，可节省约" + string(rune(rec.SavingsPercent)) + "%"
		rec.Cautions = append(rec.Cautions, "需要考虑数据迁移成本和时间")
		rec.Cautions = append(rec.Cautions, "需要考虑网络带宽依赖")
	} else if minEstimate.TotalCost > selfHostedMonthly*1.3 {
		// 自建便宜30%以上
		rec.RecommendedType = "self_hosted"
		rec.SavingsPerMonth = minEstimate.TotalCost - selfHostedMonthly
		rec.SavingsPerYear = rec.SavingsPerMonth * 12
		rec.Reason = "自建存储成本更低，可节省约" + string(rune(int(rec.SavingsPerMonth/selfHostedMonthly*100))) + "%"
		rec.Cautions = append(rec.Cautions, "需要考虑运维成本和人员投入")
		rec.Cautions = append(rec.Cautions, "需要考虑硬件故障风险")
	} else {
		// 成本相近，推荐混合方案
		rec.RecommendedType = "hybrid"
		rec.Reason = "自建与云存储成本相近，建议根据数据特性选择"
		rec.Cautions = append(rec.Cautions, "热数据可使用自建存储")
		rec.Cautions = append(rec.Cautions, "冷数据可使用云归档存储")
		rec.Cautions = append(rec.Cautions, "重要数据建议多云备份")
	}

	return rec
}

// generateSummary 生成摘要.
func (c *CloudCostCalculator) generateSummary(selfHostedCost *StorageCost, estimates []CloudCostEstimate) CloudComparisonSummary {
	summary := CloudComparisonSummary{}

	if selfHostedCost != nil {
		summary.SelfHostedMonthly = selfHostedCost.TotalCost
	}

	if len(estimates) == 0 {
		return summary
	}

	// 计算云存储成本范围
	minCost := estimates[0].TotalCost
	maxCost := estimates[0].TotalCost
	totalCost := 0.0

	for _, e := range estimates {
		if e.TotalCost < minCost {
			minCost = e.TotalCost
		}
		if e.TotalCost > maxCost {
			maxCost = e.TotalCost
		}
		totalCost += e.TotalCost
	}

	summary.CloudMinMonthly = round(minCost, 2)
	summary.CloudMaxMonthly = round(maxCost, 2)
	summary.CloudAvgMonthly = round(totalCost/float64(len(estimates)), 2)

	// 确定最优方案
	if summary.SelfHostedMonthly < minCost {
		summary.BestOption = "self_hosted"
		summary.CostDifference = round(minCost-summary.SelfHostedMonthly, 2)
	} else {
		summary.BestOption = "cloud"
		summary.CostDifference = round(summary.SelfHostedMonthly-minCost, 2)
	}

	return summary
}

// GetAvailableProviders 获取可用云服务商.
func (c *CloudCostCalculator) GetAvailableProviders() []CloudProvider {
	return []CloudProvider{
		CloudProviderAliyun,
		CloudProviderTencent,
		CloudProviderHuawei,
		CloudProviderAWS,
		CloudProviderAzure,
	}
}

// GetAvailableTiers 获取可用存储层级.
func (c *CloudCostCalculator) GetAvailableTiers() []CloudStorageTier {
	return []CloudStorageTier{
		TierStandard,
		TierIA,
		TierArchive,
		TierDeepArchive,
	}
}

// UpdatePricing 更新定价.
func (c *CloudCostCalculator) UpdatePricing(pricing CloudPricing) {
	c.registry.SetPricing(pricing)
}
