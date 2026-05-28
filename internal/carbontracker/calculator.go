// Package carbontracker 提供碳排放计算引擎
package carbontracker

import (
	"fmt"
	"math"
	"time"
)

// Calculator 碳排放计算器
type Calculator struct {
	regionIntensities map[string]float64
	defaultRegion     string
}

// NewCalculator 创建碳排放计算器
func NewCalculator(regionIntensities map[string]float64, defaultRegion string) *Calculator {
	if regionIntensities == nil {
		regionIntensities = map[string]float64{
			"CN": 581.0,
			"US": 386.0,
			"EU": 276.0,
			"JP": 462.0,
			"IN": 708.0,
			"AU": 530.0,
		}
	}
	if defaultRegion == "" {
		defaultRegion = "CN"
	}
	return &Calculator{
		regionIntensities: regionIntensities,
		defaultRegion:     defaultRegion,
	}
}

// CalculateCarbon 计算碳排放量
// energyKWh: 能耗 (kWh)
// intensity: 碳强度 (gCO2/kWh)
// 返回碳排放 (kg CO2)
func (c *Calculator) CalculateCarbon(energyKWh, intensity float64) float64 {
	if energyKWh < 0 || intensity < 0 {
		return 0
	}
	return energyKWh * intensity / 1000.0
}

// CalculateCarbonByRegion 按地区计算碳排放
// energyKWh: 能耗 (kWh)
// region: 地区代码
func (c *Calculator) CalculateCarbonByRegion(energyKWh float64, region string) float64 {
	intensity := c.GetIntensity(region)
	return c.CalculateCarbon(energyKWh, intensity)
}

// GetIntensity 获取地区碳强度
func (c *Calculator) GetIntensity(region string) float64 {
	if intensity, ok := c.regionIntensities[region]; ok {
		return intensity
	}
	return c.regionIntensities[c.defaultRegion]
}

// SetIntensity 设置地区碳强度
func (c *Calculator) SetIntensity(region string, intensity float64) {
	c.regionIntensities[region] = intensity
}

// WattsToKWh 功率转能耗
// watts: 功率 (W)
// hours: 时间 (小时)
func (c *Calculator) WattsToKWh(watts, hours float64) float64 {
	return watts * hours / 1000.0
}

// CalculateDeviceCarbon 计算设备碳排放
func (c *Calculator) CalculateDeviceCarbon(device *EnergyConsumption, region string, duration time.Duration) []DeviceCarbonStat {
	if device == nil {
		return nil
	}

	hours := duration.Hours()
	intensity := c.GetIntensity(region)
	totalCarbon := c.CalculateCarbon(c.WattsToKWh(device.TotalWatts, hours), intensity)

	if totalCarbon == 0 {
		return nil
	}

	stats := make([]DeviceCarbonStat, 0)

	devices := []struct {
		id    string
		name  string
		dtype DeviceType
		watts float64
	}{
		{"cpu", "CPU", DeviceTypeCPU, device.CPUWatts},
		{"disk", "Disk", DeviceTypeDisk, device.DiskWatts},
		{"network", "Network", DeviceTypeNetwork, device.NetWatts},
		{"memory", "Memory", DeviceTypeMemory, device.MemoryWatts},
		{"gpu", "GPU", DeviceTypeGPU, device.GPUWatts},
		{"other", "Other", DeviceTypeOther, device.OtherWatts},
	}

	for _, d := range devices {
		if d.watts <= 0 {
			continue
		}
		energyKWh := c.WattsToKWh(d.watts, hours)
		carbonKg := c.CalculateCarbon(energyKWh, intensity)
		stats = append(stats, DeviceCarbonStat{
			DeviceID:   d.id,
			DeviceName: d.name,
			DeviceType: d.dtype,
			EnergyKWh:  math.Round(energyKWh*1000) / 1000,
			CarbonKg:   math.Round(carbonKg*1000) / 1000,
			Percentage: math.Round(carbonKg/totalCarbon*10000) / 100,
		})
	}

	return stats
}

// CalculateIntensity 计算混合碳强度
// gridPct: 电网占比 (0-100)
// solarPct: 太阳能占比
// windPct: 风能占比
// region: 地区
func (c *Calculator) CalculateIntensity(gridPct, solarPct, windPct float64, region string) float64 {
	gridIntensity := c.GetIntensity(region)
	solarIntensity := 41.0  // 太阳能全生命周期排放因子
	windIntensity := 11.0   // 风能全生命周期排放因子

	total := gridPct + solarPct + windPct
	if total == 0 {
		return gridIntensity
	}

	// 归一化
	gridPct /= total
	solarPct /= total
	windPct /= total

	return gridIntensity*gridPct + solarIntensity*solarPct + windIntensity*windPct
}

// EstimateTreeEquivalent 计算等效树木数量
// carbonKg: 碳排放量 (kg CO2)
// 一棵树一年吸收约 21.77 kg CO2
func (c *Calculator) EstimateTreeEquivalent(carbonKg float64) int {
	if carbonKg <= 0 {
		return 0
	}
	return int(math.Ceil(carbonKg / 21.77))
}

// CalculatePUE 计算 PUE (Power Usage Effectiveness)
// totalPower: 总功耗
// itPower: IT 设备功耗
func (c *Calculator) CalculatePUE(totalPower, itPower float64) float64 {
	if itPower <= 0 {
		return 0
	}
	return math.Round(totalPower/itPower*100) / 100
}

// EstimateAnnualCarbon 预估年碳排放
// currentMonthlyKg: 当前月排放 (kg)
func (c *Calculator) EstimateAnnualCarbon(currentMonthlyKg float64) float64 {
	return currentMonthlyKg * 12 / 1000.0 // 转换为吨
}

// CalculateReduction 计算减排量
// baselineKg: 基准排放 (kg)
// currentKg: 当前排放 (kg)
func (c *Calculator) CalculateReduction(baselineKg, currentKg float64) (float64, float64) {
	if baselineKg <= 0 {
		return 0, 0
	}
	reduction := baselineKg - currentKg
	percentage := reduction / baselineKg * 100
	return math.Round(reduction*100) / 100, math.Round(percentage*100) / 100
}

// CalculateTargetProgress 计算目标进度
func (c *Calculator) CalculateTargetProgress(target *CarbonTarget) float64 {
	if target == nil || target.BaselineCarbonT <= 0 {
		return 0
	}
	targetCarbon := target.BaselineCarbonT * (1 - target.ReductionPct/100)
	if target.BaselineCarbonT <= targetCarbon {
		return 100
	}
	current := target.BaselineCarbonT - target.CurrentCarbonT
	required := target.BaselineCarbonT - targetCarbon
	progress := current / required * 100
	if progress > 100 {
		progress = 100
	}
	if progress < 0 {
		progress = 0
	}
	return math.Round(progress*100) / 100
}

// ClassifyIntensity 分类碳强度等级
// intensity: 碳强度 (gCO2/kWh)
// 返回: "very_low", "low", "medium", "high", "very_high"
func (c *Calculator) ClassifyIntensity(intensity float64) string {
	switch {
	case intensity < 50:
		return "very_low"
	case intensity < 150:
		return "low"
	case intensity < 400:
		return "medium"
	case intensity < 600:
		return "high"
	default:
		return "very_high"
	}
}

// CalculateESGScore 计算 ESG 评分
func (c *Calculator) CalculateESGScore(
	totalCarbonT float64,
	greenEnergyPct float64,
	reductionPct float64,
	targetProgress float64,
) *ESGScore {
	// 环境分: 基于碳排放和绿色能源使用
	envScore := 50.0
	if greenEnergyPct > 0 {
		envScore += greenEnergyPct * 0.3
	}
	if reductionPct > 0 {
		envScore += reductionPct * 0.2
	}
	if envScore > 100 {
		envScore = 100
	}

	// 社会分: 固定基础分
	socialScore := 70.0

	// 治理分: 基于目标追踪
	govScore := 60.0 + targetProgress*0.3
	if govScore > 100 {
		govScore = 100
	}

	overall := envScore*0.5 + socialScore*0.25 + govScore*0.25

	rating := "E"
	switch {
	case overall >= 90:
		rating = "A+"
	case overall >= 80:
		rating = "A"
	case overall >= 70:
		rating = "B"
	case overall >= 60:
		rating = "C"
	case overall >= 50:
		rating = "D"
	}

	return &ESGScore{
		Overall:       math.Round(overall*100) / 100,
		Environmental: math.Round(envScore*100) / 100,
		Social:        math.Round(socialScore*100) / 100,
		Governance:    math.Round(govScore*100) / 100,
		Breakdown: map[string]float64{
			"carbon_emission":  envScore,
			"green_energy":     greenEnergyPct,
			"reduction":        reductionPct,
			"target_progress":  targetProgress,
			"social":           socialScore,
			"governance":       govScore,
		},
		Rating: rating,
	}
}

// GenerateReductionTips 生成碳减排建议
func (c *Calculator) GenerateReductionTips(
	avgDailyCarbonKg float64,
	deviceStats []DeviceCarbonStat,
	greenEnergyPct float64,
) []CarbonReductionTip {
	tips := make([]CarbonReductionTip, 0)

	// 基于日均碳排放
	if avgDailyCarbonKg > 5 {
		tips = append(tips, CarbonReductionTip{
			ID:            "tip_schedule",
			Title:         "优化任务调度",
			Description:   "将高负载任务调度到碳强度较低的时段（如夜间或可再生能源发电高峰期）",
			Category:      "scheduling",
			Impact:        "high",
			EstimatedSave: avgDailyCarbonKg * 0.15 * 365,
		})
	}

	// 基于设备碳排放
	for _, ds := range deviceStats {
		if ds.Percentage > 30 && ds.DeviceType == DeviceTypeDisk {
			tips = append(tips, CarbonReductionTip{
				ID:            "tip_disk_sleep",
				Title:         "启用硬盘休眠",
				Description:   "磁盘功耗占比过高，建议启用硬盘自动休眠功能",
				Category:      "hardware",
				Impact:        "medium",
				EstimatedSave: ds.CarbonKg * 0.3 * 365,
			})
		}
		if ds.Percentage > 20 && ds.DeviceType == DeviceTypeCPU {
			tips = append(tips, CarbonReductionTip{
				ID:            "tip_cpu_power",
				Title:         "CPU 功耗管理",
				Description:   "启用 CPU 动态频率调节，低负载时降低频率以节能",
				Category:      "hardware",
				Impact:        "medium",
				EstimatedSave: ds.CarbonKg * 0.2 * 365,
			})
		}
	}

	// 基于绿色能源使用
	if greenEnergyPct < 50 {
		tips = append(tips, CarbonReductionTip{
			ID:            "tip_green_energy",
			Title:         "增加绿色能源比例",
			Description:   "考虑安装太阳能板或购买绿色电力证书",
			Category:      "energy",
			Impact:        "high",
			EstimatedSave: avgDailyCarbonKg * 0.4 * 365,
		})
	}

	// 通用建议
	tips = append(tips, CarbonReductionTip{
		ID:            "tip_virtualization",
		Title:         "提高虚拟化率",
		Description:   "整合工作负载到更少的物理服务器上，减少总体能耗",
		Category:      "infrastructure",
		Impact:        "medium",
		EstimatedSave: avgDailyCarbonKg * 0.1 * 365,
	})

	return tips
}

// GenerateGreenSuggestions 生成绿色能源调度建议
func (c *Calculator) GenerateGreenSuggestions(
	currentIntensity float64,
	region string,
	deviceStats []DeviceCarbonStat,
) []GreenEnergySuggestion {
	suggestions := make([]GreenEnergySuggestion, 0)
	now := time.Now()

	// 当前碳强度高时
	if currentIntensity > 400 {
		suggestions = append(suggestions, GreenEnergySuggestion{
			ID:            generateID(),
			Timestamp:     now,
			Suggestion:    "当前电网碳强度较高，建议推迟非紧急任务",
			Priority:      "high",
			Category:      "scheduling",
			EstimatedSave: 0.5,
			Detail:        fmt.Sprintf("当前碳强度 %.0f gCO2/kWh，建议延迟至碳强度较低的时段执行", currentIntensity),
		})
	}

	// 低功耗时段建议
	hour := now.Hour()
	if hour >= 22 || hour < 6 {
		suggestions = append(suggestions, GreenEnergySuggestion{
			ID:            generateID(),
			Timestamp:     now,
			Suggestion:    "当前为低谷电价时段，适合执行批量数据处理任务",
			Priority:      "medium",
			Category:      "scheduling",
			EstimatedSave: 0.3,
			Detail:        "夜间电价较低，同时部分电网可再生能源比例较高",
		})
	}

	// 基于设备统计
	totalDiskCarbon := 0.0
	for _, ds := range deviceStats {
		if ds.DeviceType == DeviceTypeDisk {
			totalDiskCarbon += ds.CarbonKg
		}
	}
	if totalDiskCarbon > 1.0 {
		suggestions = append(suggestions, GreenEnergySuggestion{
			ID:            generateID(),
			Timestamp:     now,
			Suggestion:    "磁盘能耗较高，建议实施分层存储策略",
			Priority:      "medium",
			Category:      "hardware",
			EstimatedSave: totalDiskCarbon * 0.2,
			Detail:        "将不常用数据迁移至低功耗存储介质",
		})
	}

	return suggestions
}
