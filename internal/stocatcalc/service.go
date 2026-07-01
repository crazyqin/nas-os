// Package stocatcalc 提供存储总拥有成本（TCO）计算能力
package stocatcalc

import (
	"fmt"
	"math"
	"time"
)

// Service 存储TCO计算服务
type Service struct {
	templates []Template
}

// NewService 创建新的计算服务
func NewService() *Service {
	return &Service{
		templates: defaultTemplates(),
	}
}

// Calculate 计算单个方案的成本
func (s *Service) Calculate(req CalcRequest) (CalcResult, error) {
	if len(req.Disks) == 0 {
		return CalcResult{}, fmt.Errorf("磁盘配置不能为空")
	}
	if req.Years <= 0 {
		req.Years = 1 // 默认1年
	}
	if req.PowerPriceKWh < 0 {
		req.PowerPriceKWh = 0.5 // 默认电价0.5元/kWh
	}

	// 计算硬件成本与原始容量
	var hardwareCost, rawCapacityTB, totalWatts float64
	for _, disk := range req.Disks {
		if disk.Quantity <= 0 {
			return CalcResult{}, fmt.Errorf("磁盘数量必须大于0")
		}
		if disk.CapacityTB <= 0 {
			return CalcResult{}, fmt.Errorf("磁盘容量必须大于0")
		}
		if disk.Price < 0 {
			return CalcResult{}, fmt.Errorf("磁盘价格不能为负")
		}
		hardwareCost += disk.Price * float64(disk.Quantity)
		rawCapacityTB += disk.CapacityTB * float64(disk.Quantity)
		// 累加功耗
		totalWatts += powerTable[disk.Type] * float64(disk.Quantity)
	}

	// 计算可用容量（考虑RAID）
	efficiency := raidEfficiencyTable[req.RaidLevel]
	if efficiency == 0 {
		efficiency = 1.0 // 未知RAID级别默认100%
	}
	usableCapacityTB := rawCapacityTB * efficiency

	// 计算电力成本
	// 总功耗(瓦) × 24小时 × 365天 × 年数 / 1000 = kWh
	// kWh × 电价 = 电力成本
	totalKWh := totalWatts * 24 * 365 * float64(req.Years) / 1000
	powerCost := totalKWh * req.PowerPriceKWh

	// 总成本
	totalCost := hardwareCost + powerCost

	// 每TB成本
	var costPerTB float64
	if usableCapacityTB > 0 {
		costPerTB = totalCost / usableCapacityTB
	}

	// 年化成本
	annualCost := totalCost / float64(req.Years)
	monthlyCost := annualCost / 12

	// 推断方案类型
	scheme := inferScheme(req.Disks)

	result := CalcResult{
		Scheme:           scheme,
		HardwareCost:     roundToTwo(hardwareCost),
		PowerCost:        roundToTwo(powerCost),
		TotalCost:        roundToTwo(totalCost),
		RawCapacityTB:    roundToTwo(rawCapacityTB),
		UsableCapacityTB: roundToTwo(usableCapacityTB),
		CostPerTB:        roundToTwo(costPerTB),
		AnnualCost:       roundToTwo(annualCost),
		MonthlyCost:      roundToTwo(monthlyCost),
		PowerWatts:       totalWatts,
		Disks:            req.Disks,
	}

	return result, nil
}

// Compare 对比多个方案
func (s *Service) Compare(requests []CalcRequest) (*ComparisonResult, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("至少需要一个方案")
	}

	years := requests[0].Years
	results := make([]CalcResult, 0, len(requests))

	for _, req := range requests {
		result, err := s.Calculate(req)
		if err != nil {
			return nil, fmt.Errorf("方案计算失败: %w", err)
		}
		results = append(results, result)
	}

	// 找出总成本最低和每TB成本最低的方案
	var bestByTotal, bestByPerTB *CalcResult
	for i := range results {
		if bestByTotal == nil || results[i].TotalCost < bestByTotal.TotalCost {
			bestByTotal = &results[i]
		}
		if bestByPerTB == nil || results[i].CostPerTB < bestByPerTB.CostPerTB {
			bestByPerTB = &results[i]
		}
	}

	return &ComparisonResult{
		GeneratedAt: time.Now(),
		Years:       years,
		Results:     results,
		BestByTotal: bestByTotal,
		BestByPerTB: bestByPerTB,
	}, nil
}

// GetTemplates 获取预置模板
func (s *Service) GetTemplates() []Template {
	return s.templates
}

// inferScheme 根据磁盘配置推断存储方案类型
func inferScheme(disks []DiskSpec) StorageScheme {
	hasHDD, hasSSD, hasNVMe := false, false, false
	for _, d := range disks {
		switch d.Type {
		case DiskTypeHDD:
			hasHDD = true
		case DiskTypeSSD:
			hasSSD = true
		case DiskTypeNVMe:
			hasNVMe = true
		}
	}

	switch {
	case hasNVMe && !hasHDD && !hasSSD:
		return SchemePureNVMe
	case hasSSD && !hasHDD:
		return SchemePureSSD
	case hasHDD && !hasSSD && !hasNVMe:
		return SchemePureHDD
	default:
		return SchemeHybrid
	}
}

// roundToTwo 保留两位小数
func roundToTwo(v float64) float64 {
	return math.Round(v*100) / 100
}

// defaultTemplates 预置存储方案模板
func defaultTemplates() []Template {
	return []Template{
		{
			ID:          "budget-hdd-4bay",
			Name:        "经济型4盘位HDD",
			Scheme:      SchemePureHDD,
			Description: "4块8TB企业级HDD，RAID5，适合大容量冷存储",
			Disks: []DiskSpec{
				{Type: DiskTypeHDD, CapacityTB: 8, Price: 1200, Quantity: 4},
			},
			RaidLevel: "raid5",
		},
		{
			ID:          "hybrid-6bay",
			Name:        "混合6盘位",
			Scheme:      SchemeHybrid,
			Description: "2块NVMe做缓存+4块16TB HDD做数据，RAID5+RAID1",
			Disks: []DiskSpec{
				{Type: DiskTypeNVMe, CapacityTB: 2, Price: 800, Quantity: 2},
				{Type: DiskTypeHDD, CapacityTB: 16, Price: 2200, Quantity: 4},
			},
			RaidLevel: "raid5",
		},
		{
			ID:          "performance-ssd-4bay",
			Name:        "高性能4盘位SSD",
			Scheme:      SchemePureSSD,
			Description: "4块4TB SATA SSD，RAID10，适合高IO场景",
			Disks: []DiskSpec{
				{Type: DiskTypeSSD, CapacityTB: 4, Price: 2000, Quantity: 4},
			},
			RaidLevel: "raid10",
		},
		{
			ID:          "enterprise-nvme-6bay",
			Name:        "企业级6盘位NVMe",
			Scheme:      SchemePureNVMe,
			Description: "6块8TB NVMe SSD，RAID6，极致性能",
			Disks: []DiskSpec{
				{Type: DiskTypeNVMe, CapacityTB: 8, Price: 6000, Quantity: 6},
			},
			RaidLevel: "raid6",
		},
		{
			ID:          "home-2bay",
			Name:        "家用2盘位HDD",
			Scheme:      SchemePureHDD,
			Description: "2块4TB HDD，RAID1，适合家庭备份",
			Disks: []DiskSpec{
				{Type: DiskTypeHDD, CapacityTB: 4, Price: 600, Quantity: 2},
			},
			RaidLevel: "raid1",
		},
	}
}
