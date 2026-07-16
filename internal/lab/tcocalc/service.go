// Package tcocalc 提供TCO总拥有成本分析服务实现
package tcocalc

import (
	"fmt"
	"math"
	"time"
)

// Service TCO计算服务.
type Service struct {
	cloudPricing []CloudPricing
}

// CloudPricing 云服务商定价.
type CloudPricing struct {
	ProviderName    string  `json:"provider_name"`      // 服务商名称
	PricePerGBMonth float64 `json:"price_per_gb_month"` // 每GB每月价格（元）
	EgressPerGB     float64 `json:"egress_per_gb"`      // 出口流量每GB价格（元）
	APIPer10K       float64 `json:"api_per_10k"`        // 每万次API请求价格（元）
	FreeEgressGB    float64 `json:"free_egress_gb"`     // 每月免费出口流量（GB）
	FreeAPI10K      float64 `json:"free_api_10k"`       // 每月免费API调用（万次）
}

// NewService 创建TCO计算服务.
func NewService() *Service {
	return &Service{
		cloudPricing: defaultCloudPricing(),
	}
}

// TCORequest TCO计算请求.
type TCORequest struct {
	Hardware    []HardwareSpec  `json:"hardware"`    // 硬件配置
	Power       PowerSpec       `json:"power"`       // 电力配置
	Maintenance MaintenanceSpec `json:"maintenance"` // 维护配置
	Licenses    []LicenseSpec   `json:"licenses"`    // 软件许可配置
	StorageTB   float64         `json:"storage_tb"`  // 可用存储容量（TB）
	Years       int             `json:"years"`       // 分析年限（默认5）
	// 预估云方案参数
	MonthlyEgressGB float64 `json:"monthly_egress_gb"` // 月出口流量（GB）
	MonthlyAPI10K   float64 `json:"monthly_api_10k"`   // 月API调用（万次）
}

// Calculate 计算TCO分析报告.
func (s *Service) Calculate(req TCORequest) (*TCOReport, error) {
	years := req.Years
	if years <= 0 {
		years = 5
	}
	if years > 20 {
		return nil, fmt.Errorf("分析年限不能超过20年")
	}
	if len(req.Hardware) == 0 {
		return nil, fmt.Errorf("硬件配置不能为空")
	}

	// === 硬件成本 ===
	hardwareBreakdown := s.calcHardware(req.Hardware, years)

	// === 电力成本 ===
	powerBreakdown := s.calcPower(req.Power, years)

	// === 维护成本 ===
	maintenanceBreakdown := s.calcMaintenance(req.Maintenance, years)

	// === 软件许可成本 ===
	licenseBreakdown := s.calcLicenses(req.Licenses, years)

	breakdowns := []CostBreakdown{hardwareBreakdown, powerBreakdown, maintenanceBreakdown, licenseBreakdown}

	// === 汇总 ===
	nasUpfrontTotal := 0.0
	nasAnnualCost := 0.0
	for _, b := range breakdowns {
		for _, item := range b.Items {
			nasUpfrontTotal += item.Yearly * 0 // yearly 已包含年化，upfront 需另行计算
		}
	}

	// 重新计算 upfront: 硬件一次性采购 + 永久许可一次性费用
	nasUpfrontTotal = 0.0
	for _, hw := range req.Hardware {
		if hw.Quantity <= 0 {
			return nil, fmt.Errorf("硬件数量必须大于0")
		}
		nasUpfrontTotal += hw.Price * float64(hw.Quantity)
	}
	for _, lic := range req.Licenses {
		if lic.Type == "perpetual" {
			nasUpfrontTotal += lic.Price * float64(lic.Quantity)
		}
	}

	// 年度成本 = 各分类年度成本之和
	for _, b := range breakdowns {
		nasAnnualCost += b.Yearly
	}

	// 5年总成本 = 一次性 + 年度 × 年数
	nasFiveYearTotal := nasUpfrontTotal + nasAnnualCost*float64(years)
	nasAnnualAverage := nasFiveYearTotal / float64(years)

	// === TCO Items ===
	items := s.buildTCOItems(req, years)

	// === 云方案对比 ===
	cloudComparisons := s.compareCloud(req.StorageTB, req.MonthlyEgressGB, req.MonthlyAPI10K, years)

	// 找最便宜的云方案
	cheapestCloud := math.MaxFloat64
	for _, cc := range cloudComparisons {
		if cc.FiveYearCost < cheapestCloud {
			cheapestCloud = cc.FiveYearCost
		}
	}

	// 判断哪个更经济
	cheaperChoice := "nas"
	savingsAmount := 0.0
	savingsPercent := 0.0
	if cheapestCloud < nasFiveYearTotal {
		cheaperChoice = "cloud"
		savingsAmount = nasFiveYearTotal - cheapestCloud
	} else {
		savingsAmount = cheapestCloud - nasFiveYearTotal
	}
	if cheapestCloud > 0 && nasFiveYearTotal > 0 {
		savingsPercent = savingsAmount / math.Min(cheapestCloud, nasFiveYearTotal) * 100
	}

	// 每TB成本
	nasCostPerTB := 0.0
	cloudCostPerTB := 0.0
	if req.StorageTB > 0 {
		nasCostPerTB = nasAnnualAverage / req.StorageTB
	}
	if cheapestCloud > 0 && req.StorageTB > 0 {
		cloudCostPerTB = (cheapestCloud / float64(years)) / req.StorageTB
	}

	report := &TCOReport{
		GeneratedAt:      time.Now(),
		Years:            years,
		Hardware:         req.Hardware,
		Power:            req.Power,
		Maintenance:      req.Maintenance,
		Licenses:         req.Licenses,
		Breakdowns:       breakdowns,
		Items:            items,
		NASFiveYearTotal: roundToTwo(nasFiveYearTotal),
		NASAnnualAverage: roundToTwo(nasAnnualAverage),
		NASUpfrontTotal:  roundToTwo(nasUpfrontTotal),
		CloudComparisons: cloudComparisons,
		CheaperChoice:    cheaperChoice,
		SavingsAmount:    roundToTwo(savingsAmount),
		SavingsPercent:   roundToTwo(savingsPercent),
		NASCostPerTB:     roundToTwo(nasCostPerTB),
		CloudCostPerTB:   roundToTwo(cloudCostPerTB),
	}

	return report, nil
}

// calcHardware 计算硬件成本.
func (s *Service) calcHardware(hardware []HardwareSpec, years int) CostBreakdown {
	breakdown := CostBreakdown{
		Category: CategoryHardware,
		Items:    []CostItem{},
	}

	for _, hw := range hardware {
		upfront := hw.Price * float64(hw.Quantity)
		// 年化成本 = 一次性成本 / 寿命
		lifespan := hw.Lifespan
		if lifespan <= 0 {
			lifespan = 5
		}
		yearly := upfront / float64(lifespan)
		fiveYear := yearly * float64(years)

		breakdown.Items = append(breakdown.Items, CostItem{
			Name:          hw.Name,
			Yearly:        roundToTwo(yearly),
			FiveYearTotal: roundToTwo(fiveYear),
		})
		breakdown.Yearly += yearly
		breakdown.FiveYearTotal += fiveYear
	}

	breakdown.Yearly = roundToTwo(breakdown.Yearly)
	breakdown.FiveYearTotal = roundToTwo(breakdown.FiveYearTotal)
	return breakdown
}

// calcPower 计算电力成本.
func (s *Service) calcPower(power PowerSpec, years int) CostBreakdown {
	breakdown := CostBreakdown{
		Category: CategoryPower,
		Items:    []CostItem{},
	}

	watts := power.Watts
	if watts <= 0 {
		watts = 50 // 默认50W
	}
	hoursPerDay := power.HoursPerDay
	if hoursPerDay <= 0 {
		hoursPerDay = 24
	}
	priceKWh := power.PriceKWh
	if priceKWh <= 0 {
		priceKWh = 0.5
	}

	// 年度能耗 kWh = 功率(W) × 每日小时 × 365 / 1000
	yearlyKWh := watts * float64(hoursPerDay) * 365 / 1000
	yearlyCost := yearlyKWh * priceKWh
	fiveYearCost := yearlyCost * float64(years)

	breakdown.Items = append(breakdown.Items, CostItem{
		Name:          "电力消耗",
		Yearly:        roundToTwo(yearlyCost),
		FiveYearTotal: roundToTwo(fiveYearCost),
	})
	breakdown.Yearly = roundToTwo(yearlyCost)
	breakdown.FiveYearTotal = roundToTwo(fiveYearCost)

	return breakdown
}

// calcMaintenance 计算维护成本.
func (s *Service) calcMaintenance(maint MaintenanceSpec, years int) CostBreakdown {
	breakdown := CostBreakdown{
		Category: CategoryMaint,
		Items:    []CostItem{},
	}

	yearlyMaint := maint.AnnualCost
	if yearlyMaint < 0 {
		yearlyMaint = 0
	}

	// 硬件更换成本
	replaceInterval := maint.ReplaceInterval
	if replaceInterval <= 0 {
		replaceInterval = years // 默认不换
	}
	replaceCost := maint.ReplacementCost
	if replaceCost < 0 {
		replaceCost = 0
	}

	// 年化更换成本
	yearlyReplace := 0.0
	if replaceInterval > 0 && replaceCost > 0 {
		yearlyReplace = replaceCost / float64(replaceInterval)
	}

	yearlyTotal := yearlyMaint + yearlyReplace
	fiveYearTotal := yearlyTotal * float64(years)

	breakdown.Items = append(breakdown.Items, CostItem{
		Name:          "年度维护",
		Yearly:        roundToTwo(yearlyMaint),
		FiveYearTotal: roundToTwo(yearlyMaint * float64(years)),
	})
	if yearlyReplace > 0 {
		breakdown.Items = append(breakdown.Items, CostItem{
			Name:          "硬件更换",
			Yearly:        roundToTwo(yearlyReplace),
			FiveYearTotal: roundToTwo(yearlyReplace * float64(years)),
		})
	}

	breakdown.Yearly = roundToTwo(yearlyTotal)
	breakdown.FiveYearTotal = roundToTwo(fiveYearTotal)
	return breakdown
}

// calcLicenses 计算软件许可成本.
func (s *Service) calcLicenses(licenses []LicenseSpec, years int) CostBreakdown {
	breakdown := CostBreakdown{
		Category: CategoryLicense,
		Items:    []CostItem{},
	}

	for _, lic := range licenses {
		qty := lic.Quantity
		if qty <= 0 {
			qty = 1
		}

		var yearly, fiveYear float64
		switch lic.Type {
		case "perpetual":
			// 永久许可：一次性费用，年化为 0（已计入 upfront）
			yearly = 0
			fiveYear = 0
		case "subscription":
			// 订阅许可：年费 × 数量 × 年数
			yearly = lic.AnnualFee * float64(qty)
			fiveYear = yearly * float64(years)
		default:
			// 默认按订阅处理
			yearly = lic.AnnualFee * float64(qty)
			fiveYear = yearly * float64(years)
		}

		name := lic.Name
		if name == "" {
			name = "软件许可"
		}

		breakdown.Items = append(breakdown.Items, CostItem{
			Name:          name,
			Yearly:        roundToTwo(yearly),
			FiveYearTotal: roundToTwo(fiveYear),
		})
		breakdown.Yearly += yearly
		breakdown.FiveYearTotal += fiveYear
	}

	breakdown.Yearly = roundToTwo(breakdown.Yearly)
	breakdown.FiveYearTotal = roundToTwo(breakdown.FiveYearTotal)
	return breakdown
}

// buildTCOItems 构建 TCO 单项列表.
func (s *Service) buildTCOItems(req TCORequest, years int) []TCOItem {
	var items []TCOItem

	// 硬件一次性
	for _, hw := range req.Hardware {
		upfront := hw.Price * float64(hw.Quantity)
		items = append(items, TCOItem{
			Category:    CategoryHardware,
			Name:        hw.Name,
			UpfrontCost: roundToTwo(upfront),
			AnnualCost:  0,
		})
	}

	// 电力年度
	watts := req.Power.Watts
	if watts <= 0 {
		watts = 50
	}
	hoursPerDay := req.Power.HoursPerDay
	if hoursPerDay <= 0 {
		hoursPerDay = 24
	}
	priceKWh := req.Power.PriceKWh
	if priceKWh <= 0 {
		priceKWh = 0.5
	}
	yearlyPower := watts * float64(hoursPerDay) * 365 / 1000 * priceKWh
	items = append(items, TCOItem{
		Category:    CategoryPower,
		Name:        "电力消耗",
		UpfrontCost: 0,
		AnnualCost:  roundToTwo(yearlyPower),
	})

	// 维护年度
	items = append(items, TCOItem{
		Category:    CategoryMaint,
		Name:        "年度维护",
		UpfrontCost: 0,
		AnnualCost:  roundToTwo(req.Maintenance.AnnualCost),
	})

	// 许可
	for _, lic := range req.Licenses {
		qty := lic.Quantity
		if qty <= 0 {
			qty = 1
		}
		var upfront, annual float64
		switch lic.Type {
		case "perpetual":
			upfront = lic.Price * float64(qty)
			annual = 0
		case "subscription":
			upfront = 0
			annual = lic.AnnualFee * float64(qty)
		}
		name := lic.Name
		if name == "" {
			name = "软件许可"
		}
		items = append(items, TCOItem{
			Category:    CategoryLicense,
			Name:        name,
			UpfrontCost: roundToTwo(upfront),
			AnnualCost:  roundToTwo(annual),
		})
	}

	return items
}

// compareCloud 云方案对比.
func (s *Service) compareCloud(storageTB, monthlyEgressGB, monthlyAPI10K float64, years int) []CloudComparison {
	var results []CloudComparison

	for _, cp := range s.cloudPricing {
		storageGB := storageTB * 1024
		monthlyStorage := storageGB * cp.PricePerGBMonth

		// 出口流量费
		egressGB := monthlyEgressGB - cp.FreeEgressGB
		if egressGB < 0 {
			egressGB = 0
		}
		monthlyEgress := egressGB * cp.EgressPerGB

		// API 调用费
		api10K := monthlyAPI10K - cp.FreeAPI10K
		if api10K < 0 {
			api10K = 0
		}
		monthlyAPI := api10K * cp.APIPer10K

		monthlyTotal := monthlyStorage + monthlyEgress + monthlyAPI
		annualCost := monthlyTotal * 12
		fiveYearCost := annualCost * float64(years)

		results = append(results, CloudComparison{
			ProviderName:  cp.ProviderName,
			StorageSizeTB: storageTB,
			MonthlyCost:   roundToTwo(monthlyTotal),
			AnnualCost:    roundToTwo(annualCost),
			FiveYearCost:  roundToTwo(fiveYearCost),
			EgressCost:    roundToTwo(monthlyEgress * 12),
			APICallCost:   roundToTwo(monthlyAPI * 12),
		})
	}

	return results
}

// GetCloudPricing 获取云服务商定价列表.
func (s *Service) GetCloudPricing() []CloudPricing {
	return s.cloudPricing
}

// roundToTwo 保留两位小数.
func roundToTwo(v float64) float64 {
	return math.Round(v*100) / 100
}

// defaultCloudPricing 预置云服务商定价.
func defaultCloudPricing() []CloudPricing {
	return []CloudPricing{
		{
			ProviderName:    "AWS S3 Standard",
			PricePerGBMonth: 0.023 / 7.0, // USD 0.023/GB → 约 CNY（汇率7）
			EgressPerGB:     0.09 / 7.0,
			APIPer10K:       0.005 / 7.0,
			FreeEgressGB:    100,
			FreeAPI10K:      2000,
		},
		{
			ProviderName:    "阿里云 OSS 标准",
			PricePerGBMonth: 0.12, // 约 0.12 元/GB/月
			EgressPerGB:     0.50,
			APIPer10K:       0.01,
			FreeEgressGB:    5,
			FreeAPI10K:      100,
		},
		{
			ProviderName:    "腾讯云 COS 标准存储",
			PricePerGBMonth: 0.099,
			EgressPerGB:     0.50,
			APIPer10K:       0.01,
			FreeEgressGB:    10,
			FreeAPI10K:      100,
		},
		{
			ProviderName:    "Backblaze B2",
			PricePerGBMonth: 0.005 / 7.0, // USD 0.005/GB
			EgressPerGB:     0.01 / 7.0,
			APIPer10K:       0.0005 / 7.0,
			FreeEgressGB:    100, // 每日前1GB免费
			FreeAPI10K:      2000,
		},
	}
}
