// Package disk 提供NVMe厂商规格数据库
// Version: v2.388.0 - NVMe健康预测增强
package disk

// ManufacturerTBWSpec 厂商TBW规格
type ManufacturerTBWSpec struct {
	// 品牌信息
	Manufacturer string `json:"manufacturer"` // 品牌名称
	ModelPattern string `json:"modelPattern"` // 型号匹配模式 (正则表达式)

	// TBW规格 (按容量区分)
	TBWByCapacity map[uint64]TBWSpec `json:"tbwByCapacity"` // key: 容量(bytes)

	// 温度规格
	OperatingTempMax   uint8 `json:"operatingTempMax"`   // 最高工作温度
	OperatingTempMin   uint8 `json:"operatingTempMin"`   // 最低工作温度
	WarrantyTempMax    uint8 `json:"warrantyTempMax"`    // 保修最高温度
	Thermal throttlingTemp uint8 `json:"thermalThrottlingTemp"` // 热节流温度

	// 保修信息
	WarrantyYears int    `json:"warrantyYears"` // 保修年限
	WarrantyType  string `json:"warrantyType"`  // 保修类型 (limited/unlimited)

	// 写耐久性评级
	EnduranceRating string `json:"enduranceRating"` // low/medium/high/enterprise
}

// TBWSpec TBW规格详情
type TBWSpec struct {
	CapacityGB   uint64  `json:"capacityGB"`   // 容量(GB)
	TBWTotal     float64 `json:"tbwTotal"`     // 总TBW
	DWPD         float64 `json:"dwpd"`         // 每日写入次数 (Drive Writes Per Day)
	PBW          float64 `json:"pbw"`          // PBW (Petabytes Written)
	WriteSpeedMax float64 `json:"writeSpeedMax"` // 最大写入速度(MB/s)
}

// ManufacturerTBWDatabase 厂商TBW数据库
var ManufacturerTBWDatabase = []ManufacturerTBWSpec{
	// ========== Samsung ========== {
		Manufacturer:     "Samsung",
		ModelPattern:     "^(Samsung|SAMSUNG).*980.*PRO",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024: {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024: {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		OperatingTempMin:     0,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 82,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},
	{
		Manufacturer:     "Samsung",
		ModelPattern:     "^(Samsung|SAMSUNG).*990.*PRO",
		TBWByCapacity: map[uint64]TBWSpec{
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1200, DWPD: 3.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 2400, DWPD: 3.0},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 85,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},
	{
		Manufacturer:     "Samsung",
		ModelPattern:     "^(Samsung|SAMSUNG).*970.*(EVO|Plus)",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024: {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024: {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 80,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},
	{
		Manufacturer:     "Samsung",
		ModelPattern:     "^(Samsung|SAMSUNG).*870.*(EVO|QVO)",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024: {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024: {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.5},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 2400, DWPD: 1.5},
			8192 * 1024 * 1024 * 1024: {CapacityGB: 8192, TBWTotal: 4800, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		WarrantyYears:        5,
		EnduranceRating:      "medium",
	},

	// ========== Western Digital ========== {
		Manufacturer:     "Western Digital",
		ModelPattern:     "^(WDC|Western Digital).*SN850X",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.5},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 2400, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 83,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},
	{
		Manufacturer:     "Western Digital",
		ModelPattern:     "^(WDC|Western Digital).*SN750",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 80,
		WarrantyYears:        5,
		EnduranceRating:      "medium",
	},
	{
		Manufacturer:     "Western Digital",
		ModelPattern:     "^(WDC|Western Digital).*Black.*SN770",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 200, DWPD: 2.0},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 83,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},

	// ========== Intel ========== {
		Manufacturer:     "Intel",
		ModelPattern:     "^Intel.*670p",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 310, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1000, DWPD: 1.0},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		WarrantyYears:        5,
		EnduranceRating:      "medium",
	},
	{
		Manufacturer:     "Intel",
		ModelPattern:     "^Intel.*Optane.*905P",
		TBWByCapacity: map[uint64]TBWSpec{
			480 * 1024 * 1024 * 1024:  {CapacityGB: 480, TBWTotal: 9000, DWPD: 18.0},
			960 * 1024 * 1024 * 1024:  {CapacityGB: 960, TBWTotal: 18000, DWPD: 18.0},
			1500 * 1024 * 1024 * 1024: {CapacityGB: 1500, TBWTotal: 27000, DWPD: 18.0},
		},
		OperatingTempMax:     85,
		WarrantyTempMax:      85,
		WarrantyYears:        5,
		EnduranceRating:      "enterprise",
	},

	// ========== Seagate ========== {
		Manufacturer:     "Seagate",
		ModelPattern:     "^Seagate.*FireCuda.*530",
		TBWByCapacity: map[uint64]TBWSpec{
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 700, DWPD: 3.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1400, DWPD: 3.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 2800, DWPD: 3.5},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 5600, DWPD: 3.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 85,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},
	{
		Manufacturer:     "Seagate",
		ModelPattern:     "^Seagate.*FireCuda.*510",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 250, DWPD: 2.5},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 510, DWPD: 2.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1020, DWPD: 2.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},

	// ========== Kingston ========== {
		Manufacturer:     "Kingston",
		ModelPattern:     "^Kingston.*KC3000",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 800, DWPD: 2.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1600, DWPD: 2.0},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 3200, DWPD: 2.0},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 80,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},
	{
		Manufacturer:     "Kingston",
		ModelPattern:     "^Kingston.*NV2",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1000, DWPD: 1.0},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		WarrantyYears:        3,
		EnduranceRating:      "low",
	},

	// ========== Crucial/Micron ========== {
		Manufacturer:     "Crucial",
		ModelPattern:     "^Crucial.*(CT|P5 Plus)",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 200, DWPD: 2.0},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		WarrantyYears:        5,
		EnduranceRating:      "medium",
	},
	{
		Manufacturer:     "Micron",
		ModelPattern:     "^Micron.*7450",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 2500, DWPD: 10.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 5000, DWPD: 10.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 10000, DWPD: 10.0},
		},
		OperatingTempMax:     85,
		WarrantyTempMax:      85,
		WarrantyYears:        5,
		EnduranceRating:      "enterprise",
	},

	// ========== Corsair ========== {
		Manufacturer:     "Corsair",
		ModelPattern:     "^Corsair.*MP600",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 200, DWPD: 2.0},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 400, DWPD: 2.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 800, DWPD: 2.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1600, DWPD: 2.0},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 3200, DWPD: 2.0},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 80,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},

	// ========== Sabrent ========== {
		Manufacturer:     "Sabrent",
		ModelPattern:     "^Sabrent.*Rocket.*4 Plus",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 500, DWPD: 2.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1000, DWPD: 2.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 2000, DWPD: 2.5},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 4000, DWPD: 2.5},
			8192 * 1024 * 1024 * 1024: {CapacityGB: 8192, TBWTotal: 8000, DWPD: 2.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 85,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},

	// ========== SK Hynix ========== {
		Manufacturer:     "SK Hynix",
		ModelPattern:     "^SK Hynix.*Platinum P41",
		TBWByCapacity: map[uint64]TBWSpec{
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 500, DWPD: 2.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 1000, DWPD: 2.5},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1500, DWPD: 1.5},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		WarrantyYears:        5,
		EnduranceRating:      "high",
	},

	// ========== Kioxia ========== {
		Manufacturer:     "Kioxia",
		ModelPattern:     "^Kioxia.*EXCERIA.*G2",
		TBWByCapacity: map[uint64]TBWSpec{
			250 * 1024 * 1024 * 1024:  {CapacityGB: 250, TBWTotal: 150, DWPD: 1.5},
			500 * 1024 * 1024 * 1024:  {CapacityGB: 500, TBWTotal: 300, DWPD: 1.5},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.5},
		},
		OperatingTempMax:     85,
		WarrantyTempMax:      85,
		WarrantyYears:        5,
		EnduranceRating:      "medium",
	},

	// ========== 默认规格 (未知品牌) ========== {
		Manufacturer:     "Generic",
		ModelPattern:     ".*",
		TBWByCapacity: map[uint64]TBWSpec{
			128 * 1024 * 1024 * 1024:  {CapacityGB: 128, TBWTotal: 100, DWPD: 1.0},
			256 * 1024 * 1024 * 1024:  {CapacityGB: 256, TBWTotal: 150, DWPD: 1.0},
			512 * 1024 * 1024 * 1024:  {CapacityGB: 512, TBWTotal: 300, DWPD: 1.0},
			1024 * 1024 * 1024 * 1024: {CapacityGB: 1024, TBWTotal: 600, DWPD: 1.0},
			2048 * 1024 * 1024 * 1024: {CapacityGB: 2048, TBWTotal: 1200, DWPD: 1.0},
			4096 * 1024 * 1024 * 1024: {CapacityGB: 4096, TBWTotal: 2400, DWPD: 1.0},
			8192 * 1024 * 1024 * 1024: {CapacityGB: 8192, TBWTotal: 4800, DWPD: 1.0},
		},
		OperatingTempMax:     70,
		WarrantyTempMax:      70,
		ThermalThrottlingTemp: 80,
		WarrantyYears:        3,
		EnduranceRating:      "low",
	},
}

// LookupTBWSpec 根据型号和容量查找TBW规格
func LookupTBWSpec(model string, capacityBytes uint64) *TBWSpec {
	for _, spec := range ManufacturerTBWDatabase {
		// 简化的型号匹配 (可用regexp替代)
		if matchModel(model, spec.ModelPattern) {
			// 查找最接近的容量规格
			var closestTBW *TBWSpec
			var minDiff uint64 = uint64(1<<63 - 1) // 最大值

			for capBytes, tbwSpec := range spec.TBWByCapacity {
				diff := absDiff(capacityBytes, capBytes)
				if diff < minDiff {
					minDiff = diff
					closestTBW = &tbwSpec
				}
			}

			if closestTBW != nil {
				return closestTBW
			}
		}
	}

	// 未找到，使用默认规格
	for _, spec := range ManufacturerTBWDatabase {
		if spec.Manufacturer == "Generic" {
			for capBytes, tbwSpec := range spec.TBWByCapacity {
				diff := absDiff(capacityBytes, capBytes)
				if diff < uint64(100*1024*1024*1024) { // 100GB容忍范围
					return &tbwSpec
				}
			}
		}
	}

	// 完全未知，返回保守估计
	capacityGB := capacityBytes / (1024 * 1024 * 1024)
	return &TBWSpec{
		CapacityGB: capacityGB,
		TBWTotal:   float64(capacityGB) * 0.5, // 保守估计: 0.5TBW/GB
		DWPD:       0.5,
	}
}

// GetManufacturerSpec 获取完整厂商规格
func GetManufacturerSpec(model string) *ManufacturerTBWSpec {
	for _, spec := range ManufacturerTBWDatabase {
		if matchModel(model, spec.ModelPattern) {
			return &spec
		}
	}
	return nil
}

// matchModel 简化型号匹配 (避免regexp开销)
func matchModel(model, pattern string) bool {
	// 去除空格和特殊字符
	model = sanitizeModel(model)
	pattern = sanitizeModel(pattern)

	// 检查关键关键词
	patternParts := splitPattern(pattern)
	for _, part := range patternParts {
		if len(part) > 2 && !containsIgnoreCase(model, part) {
			return false
		}
	}
	return true
}

// sanitizeModel 清理型号字符串
func sanitizeModel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	return s
}

// splitPattern 分割匹配模式
func splitPattern(pattern string) []string {
	pattern = strings.TrimPrefix(pattern, "^")
	pattern = strings.TrimSuffix(pattern, ".*")
	pattern = strings.ReplaceAll(pattern, "|", " ")
	pattern = strings.ReplaceAll(pattern, "(", " ")
	pattern = strings.ReplaceAll(pattern, ")", " ")
	pattern = strings.ReplaceAll(pattern, "*", " ")
	return strings.Split(pattern, " ")
}

// containsIgnoreCase 忽略大小写包含检查
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// absDiff 计算绝对差值
func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}