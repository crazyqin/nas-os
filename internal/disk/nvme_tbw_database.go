// Package disk 提供NVMe厂商规格数据库
// Version: v2.421.0 - NVMe健康预测增强
package disk

import "strings"

// ManufacturerTBWSpec 厂商TBW规格
type ManufacturerTBWSpec struct {
	Manufacturer       string            `json:"manufacturer"`
	ModelPattern       string            `json:"modelPattern"`
	TBWByCapacity      map[uint64]TBWSpec `json:"tbwByCapacity"`
	OperatingTempMax   uint8             `json:"operatingTempMax"`
	OperatingTempMin   uint8             `json:"operatingTempMin"`
	WarrantyTempMax    uint8             `json:"warrantyTempMax"`
	ThermalThrottlingTemp uint8          `json:"thermalThrottlingTemp"`
	WarrantyYears      int               `json:"warrantyYears"`
	WarrantyType       string            `json:"warrantyType"`
	EnduranceRating    string            `json:"enduranceRating"`
}

// TBWSpec TBW规格详情
type TBWSpec struct {
	CapacityGB    uint64  `json:"capacityGB"`
	TBWTotal      float64 `json:"tbwTotal"`
	DWPD          float64 `json:"dwpd"`
	PBW           float64 `json:"pbw"`
	WriteSpeedMax float64 `json:"writeSpeedMax"`
}

// ManufacturerTBWDatabase 厂商TBW数据库
var ManufacturerTBWDatabase = []ManufacturerTBWSpec{
	{
		Manufacturer:     "Samsung",
		ModelPattern:     "980 PRO",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024: {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024: {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  82,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Samsung",
		ModelPattern:     "990 PRO",
		TBWByCapacity: map[uint64]TBWSpec{
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1200, DWPD: 3.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 2400, DWPD: 3.0},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  85,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Western Digital",
		ModelPattern:     "SN850X",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.5},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  83,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Intel",
		ModelPattern:     "Optane 905P",
		TBWByCapacity: map[uint64]TBWSpec{
			480 * 1024 * 1024 * 1024:  {CapacityGB: 480, TBWTotal: 9000, DWPD: 18.0},
			960 * 1024 * 1024 * 1024:  {CapacityGB: 960, TBWTotal: 18000, DWPD: 18.0},
		},
		OperatingTempMax:       85,
		WarrantyTempMax:        85,
		WarrantyYears:          5,
		EnduranceRating:        "enterprise",
	},
	{
		Manufacturer:     "Seagate",
		ModelPattern:     "FireCuda 530",
		TBWByCapacity: map[uint64]TBWSpec{
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 700, DWPD: 3.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1400, DWPD: 3.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 2800, DWPD: 3.5},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  85,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Kingston",
		ModelPattern:     "KC3000",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 800, DWPD: 2.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1600, DWPD: 2.0},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  80,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Crucial",
		ModelPattern:     "P5 Plus",
		TBWByCapacity: map[uint64]TBWSpec{
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		WarrantyYears:          5,
		EnduranceRating:        "medium",
	},
	{
		Manufacturer:     "Corsair",
		ModelPattern:     "MP600",
		TBWByCapacity: map[uint64]TBWSpec{
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 800, DWPD: 2.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1600, DWPD: 2.0},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  80,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Sabrent",
		ModelPattern:     "Rocket 4 Plus",
		TBWByCapacity: map[uint64]TBWSpec{
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1000, DWPD: 2.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 2000, DWPD: 2.5},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  85,
		WarrantyYears:          5,
		EnduranceRating:        "high",
	},
	{
		Manufacturer:     "Generic",
		ModelPattern:     ".*",
		TBWByCapacity: map[uint64]TBWSpec{
			256 * 1024 * 1024 * 1024:  {CapacityGB: 256, TBWTotal: 150, DWPD: 1.0},
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 300, DWPD: 1.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.0},
		},
		OperatingTempMax:       70,
		WarrantyTempMax:        70,
		ThermalThrottlingTemp:  80,
		WarrantyYears:          3,
		EnduranceRating:        "low",
	},
}

// LookupTBWSpec 根据型号和容量查找TBW规格
func LookupTBWSpec(model string, capacityBytes uint64) *TBWSpec {
	for _, spec := range ManufacturerTBWDatabase {
		if strings.Contains(strings.ToUpper(model), strings.ToUpper(spec.ModelPattern)) || spec.ModelPattern == ".*" {
			var closestTBW *TBWSpec
			var minDiff uint64 = uint64(1<<63 - 1)

			for capBytes, tbwSpec := range spec.TBWByCapacity {
				diff := absDiff(capacityBytes, capBytes)
				if diff < minDiff {
					minDiff = diff
					tbwCopy := tbwSpec
					closestTBW = &tbwCopy
				}
			}

			if closestTBW != nil {
				return closestTBW
			}
		}
	}

	// 默认返回
	capacityGB := capacityBytes / (1024 * 1024 * 1024)
	return &TBWSpec{
		CapacityGB: capacityGB,
		TBWTotal:   float64(capacityGB) * 0.5,
		DWPD:       0.5,
	}
}

// GetManufacturerSpec 获取完整厂商规格
func GetManufacturerSpec(model string) *ManufacturerTBWSpec {
	for _, spec := range ManufacturerTBWDatabase {
		if strings.Contains(strings.ToUpper(model), strings.ToUpper(spec.ModelPattern)) {
			return &spec
		}
	}
	return nil
}

// absDiff 计算绝对差值
func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}