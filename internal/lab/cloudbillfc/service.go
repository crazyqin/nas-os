// Package cloudbillfc 提供云存储成本预测服务实现
package cloudbillfc

import (
	"fmt"
	"math"
	"time"
)

// Service 云存储成本预测服务.
type Service struct {
	providers []ProviderPricing
}

// NewService 创建云存储成本预测服务.
func NewService() *Service {
	return &Service{
		providers: defaultProviders(),
	}
}

// Forecast 预测云存储成本.
func (s *Service) Forecast(config ForecastConfig) (*CloudBillingForecast, error) {
	if config.Months <= 0 {
		config.Months = 12 // 默认12个月
	}
	if config.Months > 60 {
		return nil, fmt.Errorf("预测月数不能超过60")
	}
	if config.StorageGB < 0 {
		return nil, fmt.Errorf("存储用量不能为负")
	}

	forecasts := make([]MonthlyForecast, 0, len(s.providers))

	for _, provider := range s.providers {
		// 如果指定了 provider，只计算该 provider
		if config.Provider != "" && config.Provider != provider.ProviderName {
			continue
		}

		forecast := s.forecastSingle(provider, config)
		forecasts = append(forecasts, forecast)
	}

	if len(forecasts) == 0 {
		return nil, fmt.Errorf("未找到匹配的云服务商: %s", config.Provider)
	}

	// 找最优 provider（预测期总费用最低）
	bestProvider := ""
	bestCost := math.MaxFloat64
	for _, f := range forecasts {
		if f.TotalCost < bestCost {
			bestCost = f.TotalCost
			bestProvider = f.ProviderName
		}
	}

	// 生成优化建议
	optimizations := s.generateOptimizations(config, forecasts)

	report := &CloudBillingForecast{
		GeneratedAt:   time.Now(),
		Months:        config.Months,
		Config:        config,
		Forecasts:     forecasts,
		BestProvider:  bestProvider,
		Optimizations: optimizations,
	}

	return report, nil
}

// forecastSingle 预测单个服务商.
func (s *Service) forecastSingle(provider ProviderPricing, config ForecastConfig) MonthlyForecast {
	trends := make([]CostTrend, 0, config.Months+1)

	storageGB := config.StorageGB
	egressGB := config.MonthlyEgressGB
	api10K := config.MonthlyAPI10K
	growthRate := config.GrowthRateMonth
	if growthRate < 0 {
		growthRate = 0
	}

	now := time.Now()
	totalCost := 0.0
	peakMonthly := 0.0

	for month := 0; month <= config.Months; month++ {
		// 计算存储费用
		storageCost := s.calcStorageCost(provider, storageGB)

		// 计算出口流量费
		egressCost := 0.0
		actualEgress := egressGB - provider.FreeEgressGB
		if actualEgress > 0 {
			egressCost = actualEgress * provider.EgressPerGB
		}

		// 计算 API 调用费
		apiCost := 0.0
		actualAPI := api10K - provider.FreeAPI10K
		if actualAPI > 0 {
			apiCost = actualAPI * provider.APIPer10K
		}

		// 请求费（使用 API 费率）
		requestCost := 0.0
		if provider.RequestPer10K > 0 {
			actualReq := api10K - provider.FreeAPI10K
			if actualReq > 0 {
				requestCost = actualReq * provider.RequestPer10K
			}
		}

		monthlyCost := storageCost + egressCost + apiCost + requestCost
		if provider.MinMonthlyCharge > 0 && monthlyCost < provider.MinMonthlyCharge {
			monthlyCost = provider.MinMonthlyCharge
		}

		date := now.AddDate(0, month, 0).Format("2006-01")
		trend := CostTrend{
			Month:       month,
			Date:        date,
			StorageGB:   roundToTwo(storageGB),
			MonthlyCost: roundToTwo(monthlyCost),
			EgressCost:  roundToTwo(egressCost),
			APICost:     roundToTwo(apiCost + requestCost),
			StorageCost: roundToTwo(storageCost),
		}
		trends = append(trends, trend)

		totalCost += monthlyCost
		if monthlyCost > peakMonthly {
			peakMonthly = monthlyCost
		}

		// 增长
		storageGB *= (1 + growthRate)
		egressGB *= (1 + growthRate)
		api10K *= (1 + growthRate)
	}

	avgMonthly := totalCost / float64(config.Months+1)

	initialCost := trends[0].MonthlyCost
	finalCost := trends[len(trends)-1].MonthlyCost
	growthPercent := 0.0
	if initialCost > 0 {
		growthPercent = (finalCost - initialCost) / initialCost * 100
	}

	return MonthlyForecast{
		ProviderName:       provider.ProviderName,
		Config:             config,
		Trends:             trends,
		TotalCost:          roundToTwo(totalCost),
		AvgMonthly:         roundToTwo(avgMonthly),
		PeakMonthly:        roundToTwo(peakMonthly),
		GrowthPercent:      roundToTwo(growthPercent),
		FinalMonthlyCost:   roundToTwo(finalCost),
		InitialMonthlyCost: roundToTwo(initialCost),
	}
}

// calcStorageCost 计算存储费用（支持分级定价）.
func (s *Service) calcStorageCost(provider ProviderPricing, storageGB float64) float64 {
	if provider.TieredPricing && len(provider.Tiers) > 0 {
		return s.calcTieredStorageCost(provider.Tiers, storageGB)
	}
	return storageGB * provider.StoragePerGB
}

// calcTieredStorageCost 分级存储费用计算.
func (s *Service) calcTieredStorageCost(tiers []PriceTier, storageGB float64) float64 {
	total := 0.0
	remaining := storageGB

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		tierRange := tier.MaxGB - tier.MinGB
		if tier.MaxGB < 0 {
			tierRange = remaining // 无上限
		}

		usage := remaining
		if usage > tierRange {
			usage = tierRange
		}

		total += usage * tier.PriceGB
		remaining -= usage
	}

	return total
}

// CompareProviders 对比多个服务商.
func (s *Service) CompareProviders(storageGB, monthlyEgressGB, monthlyAPI10K float64) []ProviderComparison {
	var results []ProviderComparison

	for _, provider := range s.providers {
		storageCost := s.calcStorageCost(provider, storageGB)

		egressCost := 0.0
		actualEgress := monthlyEgressGB - provider.FreeEgressGB
		if actualEgress > 0 {
			egressCost = actualEgress * provider.EgressPerGB
		}

		apiCost := 0.0
		actualAPI := monthlyAPI10K - provider.FreeAPI10K
		if actualAPI > 0 {
			apiCost = actualAPI * provider.APIPer10K
		}

		monthlyCost := storageCost + egressCost + apiCost
		if provider.MinMonthlyCharge > 0 && monthlyCost < provider.MinMonthlyCharge {
			monthlyCost = provider.MinMonthlyCharge
		}

		costPerGB := 0.0
		if storageGB > 0 {
			costPerGB = monthlyCost / storageGB
		}

		results = append(results, ProviderComparison{
			ProviderName: provider.ProviderName,
			MonthlyCost:  roundToTwo(monthlyCost),
			YearlyCost:   roundToTwo(monthlyCost * 12),
			FiveYearCost: roundToTwo(monthlyCost * 60),
			CostPerGB:    roundToTwo(costPerGB),
		})
	}

	return results
}

// generateOptimizations 生成优化建议.
func (s *Service) generateOptimizations(config ForecastConfig, forecasts []MonthlyForecast) []OptimizationTip {
	var tips []OptimizationTip

	// 存储生命周期优化
	if config.GrowthRateMonth > 0.03 {
		tips = append(tips, OptimizationTip{
			Type:          "lifecycle",
			Title:         "启用存储生命周期管理",
			Description:   fmt.Sprintf("月增长率%.1f%%较高，建议配置生命周期规则自动转换冷热数据层级", config.GrowthRateMonth*100),
			SavingPercent: 40,
			SavingAmount:  roundToTwo(forecasts[0].FinalMonthlyCost * 0.4),
		})
	}

	// 出口流量优化
	if config.MonthlyEgressGB > 100 {
		tips = append(tips, OptimizationTip{
			Type:          "egress",
			Title:         "使用CDN减少出口流量",
			Description:   fmt.Sprintf("月出口%.0fGB，使用CDN回源可减少出口流量费", config.MonthlyEgressGB),
			SavingPercent: 60,
			SavingAmount:  roundToTwo(config.MonthlyEgressGB * 0.5 * 0.6),
		})
	}

	// API 调用优化
	if config.MonthlyAPI10K > 500 {
		tips = append(tips, OptimizationTip{
			Type:          "api",
			Title:         "批量API请求合并",
			Description:   fmt.Sprintf("月%.0f万次API调用，建议使用批量接口减少请求次数", config.MonthlyAPI10K),
			SavingPercent: 30,
			SavingAmount:  roundToTwo(config.MonthlyAPI10K * 0.01 * 0.3),
		})
	}

	// 分级存储优化
	if config.StorageGB > 1000 {
		tips = append(tips, OptimizationTip{
			Type:          "tier",
			Title:         "冷热数据分层存储",
			Description:   fmt.Sprintf("存储量%.0f GB, 频繁访问数据占比通常<20%%, 分层可大幅降低成本", config.StorageGB),
			SavingPercent: 50,
			SavingAmount:  roundToTwo(config.StorageGB * 0.12 * 0.5),
		})
	}

	// 存储压缩/去重
	tips = append(tips, OptimizationTip{
		Type:          "storage",
		Title:         "启用数据压缩与去重",
		Description:   "对存储数据启用压缩和去重可将存储用量降低30-50%",
		SavingPercent: 35,
		SavingAmount:  roundToTwo(config.StorageGB * 0.12 * 0.35),
	})

	return tips
}

// GetProviders 获取服务商列表.
func (s *Service) GetProviders() []ProviderPricing {
	return s.providers
}

// roundToTwo 保留两位小数.
func roundToTwo(v float64) float64 {
	return math.Round(v*100) / 100
}

// defaultProviders 预置云服务商定价.
func defaultProviders() []ProviderPricing {
	return []ProviderPricing{
		{
			ProviderName:     "AWS S3 Standard",
			StoragePerGB:     0.0033, // 约 0.023 USD / 7
			EgressPerGB:      0.0129,
			APIPer10K:        0.0007,
			RequestPer10K:    0.0006,
			FreeEgressGB:     100,
			FreeAPI10K:       2000,
			MinMonthlyCharge: 0,
			TieredPricing:    false,
		},
		{
			ProviderName:     "阿里云 OSS 标准存储",
			StoragePerGB:     0.12,
			EgressPerGB:      0.50,
			APIPer10K:        0.01,
			RequestPer10K:    0.01,
			FreeEgressGB:     5,
			FreeAPI10K:       100,
			MinMonthlyCharge: 0,
			TieredPricing:    true,
			Tiers: []PriceTier{
				{MinGB: 0, MaxGB: 1024, PriceGB: 0.12},
				{MinGB: 1024, MaxGB: 10240, PriceGB: 0.099},
				{MinGB: 10240, MaxGB: 51200, PriceGB: 0.085},
				{MinGB: 51200, MaxGB: -1, PriceGB: 0.079},
			},
		},
		{
			ProviderName:     "腾讯云 COS 标准存储",
			StoragePerGB:     0.099,
			EgressPerGB:      0.50,
			APIPer10K:        0.01,
			RequestPer10K:    0.01,
			FreeEgressGB:     10,
			FreeAPI10K:       100,
			MinMonthlyCharge: 0,
			TieredPricing:    true,
			Tiers: []PriceTier{
				{MinGB: 0, MaxGB: 1024, PriceGB: 0.099},
				{MinGB: 1024, MaxGB: 51200, PriceGB: 0.082},
				{MinGB: 51200, MaxGB: -1, PriceGB: 0.076},
			},
		},
		{
			ProviderName:     "Backblaze B2",
			StoragePerGB:     0.0007, // 约 0.005 USD / 7
			EgressPerGB:      0.0014,
			APIPer10K:        0.0001,
			RequestPer10K:    0.0,
			FreeEgressGB:     100,
			FreeAPI10K:       2000,
			MinMonthlyCharge: 0,
			TieredPricing:    false,
		},
		{
			ProviderName:     "华为云 OBS 标准存储",
			StoragePerGB:     0.099,
			EgressPerGB:      0.50,
			APIPer10K:        0.01,
			RequestPer10K:    0.01,
			FreeEgressGB:     5,
			FreeAPI10K:       100,
			MinMonthlyCharge: 0,
			TieredPricing:    false,
		},
	}
}
