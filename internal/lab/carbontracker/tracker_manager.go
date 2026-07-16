// Package carbontracker - 碳足迹追踪管理器
// 监控 NAS 设备能耗并计算碳足迹
package carbontracker

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TrackerManager 碳足迹追踪管理器.
type TrackerManager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *CarbonTrackerManagerConfig
	calculator    *Calculator
	energySources map[string]*EnergySource
	footprints    map[string][]*CarbonFootprint // deviceID -> footprints
	emissions     []*EmissionRecord
	scores        map[string]*GreenScore // deviceID -> score
}

// NewTrackerManager 创建碳足迹追踪管理器.
func NewTrackerManager(logger *zap.Logger, config *CarbonTrackerManagerConfig) *TrackerManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultCarbonTrackerManagerConfig()
	}

	tm := &TrackerManager{
		logger:        logger,
		config:        config,
		calculator:    NewCalculator(nil, config.DefaultRegion),
		energySources: make(map[string]*EnergySource),
		footprints:    make(map[string][]*CarbonFootprint),
		emissions:     make([]*EmissionRecord, 0),
		scores:        make(map[string]*GreenScore),
	}

	// 初始化默认能源来源
	tm.initDefaultEnergySources()

	return tm
}

// initDefaultEnergySources 初始化默认能源来源.
func (tm *TrackerManager) initDefaultEnergySources() {
	tm.energySources["source-grid"] = &EnergySource{
		ID:              "source-grid",
		Name:            "电网供电",
		Type:            CarbonSourceGrid,
		Region:          tm.config.DefaultRegion,
		CarbonIntensity: tm.calculator.GetIntensity(tm.config.DefaultRegion),
		CostPerKWh:      0.55,
		Percentage:      70,
		IsRenewable:     false,
		Description:     "来自国家电网的电力供应",
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	tm.energySources["source-solar"] = &EnergySource{
		ID:              "source-solar",
		Name:            "太阳能",
		Type:            CarbonSourceSolar,
		Region:          tm.config.DefaultRegion,
		CarbonIntensity: 41.0,
		CostPerKWh:      0.30,
		Percentage:      20,
		IsRenewable:     true,
		Description:     "屋顶太阳能光伏板",
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	tm.energySources["source-wind"] = &EnergySource{
		ID:              "source-wind",
		Name:            "风电",
		Type:            CarbonSourceWind,
		Region:          tm.config.DefaultRegion,
		CarbonIntensity: 11.0,
		CostPerKWh:      0.40,
		Percentage:      10,
		IsRenewable:     true,
		Description:     "风力发电",
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// CalculateFootprint 计算碳足迹.
func (tm *TrackerManager) CalculateFootprint(deviceID string, deviceName string, consumption *EnergyConsumption, duration time.Duration) (*CarbonFootprint, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if consumption == nil {
		return nil, fmt.Errorf("energy consumption data is required")
	}

	// 计算混合碳强度
	mixedIntensity := tm.calculateMixedIntensity()

	// 计算能耗
	hours := duration.Hours()
	totalEnergyKWh := tm.calculator.WattsToKWh(consumption.TotalWatts, hours)

	// 计算碳排放
	carbonKg := tm.calculator.CalculateCarbon(totalEnergyKWh, mixedIntensity)

	// 创建碳足迹记录
	footprint := &CarbonFootprint{
		ID:              fmt.Sprintf("fp-%s-%d", deviceID, time.Now().UnixNano()),
		DeviceID:        deviceID,
		DeviceName:      deviceName,
		Timestamp:       time.Now(),
		EnergyKWh:       math.Round(totalEnergyKWh*1000) / 1000,
		CarbonKg:        math.Round(carbonKg*1000) / 1000,
		CarbonIntensity: math.Round(mixedIntensity*100) / 100,
		Source:          CarbonSourceGrid,
		Region:          tm.config.DefaultRegion,
		Period:          "hourly",
		Details: &FootprintDetails{
			CPUWatts:     consumption.CPUWatts,
			DiskWatts:    consumption.DiskWatts,
			NetworkWatts: consumption.NetWatts,
			MemoryWatts:  consumption.MemoryWatts,
			GPUWatts:     consumption.GPUWatts,
			OtherWatts:   consumption.OtherWatts,
			TotalWatts:   consumption.TotalWatts,
			RuntimeHours: hours,
		},
	}

	// 存储记录
	tm.footprints[deviceID] = append(tm.footprints[deviceID], footprint)

	// 同时添加到排放记录
	tm.emissions = append(tm.emissions, &EmissionRecord{
		ID:              fmt.Sprintf("em-%s-%d", deviceID, time.Now().UnixNano()),
		Timestamp:       footprint.Timestamp,
		DeviceID:        deviceID,
		DeviceName:      deviceName,
		SourceType:      CarbonSourceGrid,
		EnergyKWh:       footprint.EnergyKWh,
		CarbonKg:        footprint.CarbonKg,
		CarbonIntensity: mixedIntensity,
		Region:          tm.config.DefaultRegion,
	})

	// 限制历史记录数量
	maxRecords := tm.config.RetentionDays * 24 // 每小时一条记录
	if len(tm.footprints[deviceID]) > maxRecords {
		tm.footprints[deviceID] = tm.footprints[deviceID][len(tm.footprints[deviceID])-maxRecords:]
	}

	tm.logger.Info("Carbon footprint calculated",
		zap.String("deviceID", deviceID),
		zap.Float64("energyKWh", footprint.EnergyKWh),
		zap.Float64("carbonKg", footprint.CarbonKg),
		zap.Float64("intensity", footprint.CarbonIntensity),
	)

	return footprint, nil
}

// calculateMixedIntensity 计算混合碳强度.
func (tm *TrackerManager) calculateMixedIntensity() float64 {
	totalPercentage := 0.0
	weightedIntensity := 0.0

	for _, source := range tm.energySources {
		if !source.IsActive {
			continue
		}
		totalPercentage += source.Percentage
		weightedIntensity += source.CarbonIntensity * source.Percentage
	}

	if totalPercentage == 0 {
		return tm.calculator.GetIntensity(tm.config.DefaultRegion)
	}

	return weightedIntensity / totalPercentage
}

// GetGreenScore 获取绿色评分.
func (tm *TrackerManager) GetGreenScore(deviceID string) (*GreenScore, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 如果有缓存的评分
	if score, ok := tm.scores[deviceID]; ok {
		return score, nil
	}

	// 计算评分
	score := tm.calculateGreenScore(deviceID)
	tm.scores[deviceID] = score

	return score, nil
}

// calculateGreenScore 计算绿色评分.
func (tm *TrackerManager) calculateGreenScore(deviceID string) *GreenScore {
	footprints, ok := tm.footprints[deviceID]
	if !ok || len(footprints) == 0 {
		return &GreenScore{
			Overall:   0,
			Grade:     "N/A",
			UpdatedAt: time.Now(),
		}
	}

	// 计算平均碳排放
	totalCarbon := 0.0
	totalEnergy := 0.0
	for _, fp := range footprints {
		totalCarbon += fp.CarbonKg
		totalEnergy += fp.EnergyKWh
	}

	avgCarbonPerDay := totalCarbon / float64(len(footprints)) * 24

	// 计算可再生能源使用率
	renewablePct := tm.getRenewablePercentage()

	// 计算能源效率
	efficiency := 100.0
	if totalEnergy > 0 {
		// 每kWh碳排放越低，效率越高
		carbonPerKWh := totalCarbon / totalEnergy
		if carbonPerKWh < 0.1 {
			efficiency = 100
		} else if carbonPerKWh < 0.3 {
			efficiency = 80
		} else if carbonPerKWh < 0.5 {
			efficiency = 60
		} else {
			efficiency = 40
		}
	}

	// 计算碳减排评分
	reductionScore := 50.0
	if avgCarbonPerDay < 1.0 {
		reductionScore = 100
	} else if avgCarbonPerDay < 3.0 {
		reductionScore = 80
	} else if avgCarbonPerDay < 5.0 {
		reductionScore = 60
	} else {
		reductionScore = 40
	}

	// 综合评分
	overall := efficiency*0.4 + renewablePct*0.3 + reductionScore*0.3
	grade := tm.calculateGrade(overall)

	// 生成建议
	recommendations := tm.generateGreenRecommendations(deviceID, avgCarbonPerDay, renewablePct)

	return &GreenScore{
		Overall:          math.Round(overall*100) / 100,
		Grade:            grade,
		EnergyEfficiency: math.Round(efficiency*100) / 100,
		RenewableUsage:   math.Round(renewablePct*100) / 100,
		CarbonReduction:  math.Round(reductionScore*100) / 100,
		DeviceScores: []DeviceGreenScore{
			{
				DeviceID:   deviceID,
				DeviceName: footprints[len(footprints)-1].DeviceName,
				Score:      overall,
				Grade:      grade,
				EnergyKWh:  totalEnergy,
				CarbonKg:   totalCarbon,
			},
		},
		Recommendations: recommendations,
		UpdatedAt:       time.Now(),
	}
}

// getRenewablePercentage 获取可再生能源占比.
func (tm *TrackerManager) getRenewablePercentage() float64 {
	pct := 0.0
	for _, source := range tm.energySources {
		if source.IsActive && source.IsRenewable {
			pct += source.Percentage
		}
	}
	return pct
}

// calculateGrade 计算等级.
func (tm *TrackerManager) calculateGrade(score float64) string {
	switch {
	case score >= 90:
		return "A+"
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "E"
	}
}

// generateGreenRecommendations 生成绿色建议.
func (tm *TrackerManager) generateGreenRecommendations(deviceID string, avgCarbonPerDay, renewablePct float64) []GreenRecommendation {
	recommendations := make([]GreenRecommendation, 0)

	if avgCarbonPerDay > 3.0 {
		recommendations = append(recommendations, GreenRecommendation{
			ID:            fmt.Sprintf("rec-schedule-%s", deviceID),
			Title:         "优化任务调度",
			Description:   "将高负载任务调度到碳强度较低的时段（如夜间或可再生能源发电高峰期）",
			Category:      "software",
			Impact:        "high",
			EstimatedSave: avgCarbonPerDay * 0.2 * 365,
		})
	}

	if renewablePct < 50 {
		recommendations = append(recommendations, GreenRecommendation{
			ID:            fmt.Sprintf("rec-green-%s", deviceID),
			Title:         "增加可再生能源比例",
			Description:   "考虑安装太阳能板或购买绿色电力证书，提高可再生能源使用率",
			Category:      "hardware",
			Impact:        "high",
			EstimatedSave: avgCarbonPerDay * 0.4 * 365,
		})
	}

	recommendations = append(recommendations, GreenRecommendation{
		ID:            fmt.Sprintf("rec-sleep-%s", deviceID),
		Title:         "启用硬盘休眠",
		Description:   "在空闲时段启用硬盘自动休眠功能，降低待机功耗",
		Category:      "hardware",
		Impact:        "medium",
		EstimatedSave: avgCarbonPerDay * 0.1 * 365,
	})

	return recommendations
}

// SetEnergySource 设置能源来源.
func (tm *TrackerManager) SetEnergySource(source *EnergySource) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if source.ID == "" {
		source.ID = fmt.Sprintf("source-%d", time.Now().UnixNano())
	}

	source.UpdatedAt = time.Now()
	tm.energySources[source.ID] = source

	// 清除缓存的评分
	tm.scores = make(map[string]*GreenScore)

	tm.logger.Info("Energy source updated",
		zap.String("sourceID", source.ID),
		zap.String("name", source.Name),
		zap.Float64("percentage", source.Percentage),
	)

	return nil
}

// GetEnergySources 获取所有能源来源.
func (tm *TrackerManager) GetEnergySources() []*EnergySource {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	sources := make([]*EnergySource, 0, len(tm.energySources))
	for _, s := range tm.energySources {
		sources = append(sources, s)
	}
	return sources
}

// GetHistory 获取碳排放历史.
func (tm *TrackerManager) GetHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*CarbonFootprint, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	footprints, ok := tm.footprints[deviceID]
	if !ok {
		return []*CarbonFootprint{}, nil
	}

	var result []*CarbonFootprint
	for _, fp := range footprints {
		if !startTime.IsZero() && fp.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && fp.Timestamp.After(endTime) {
			continue
		}
		result = append(result, fp)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 限制返回数量
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// GetEmissions 获取排放记录.
func (tm *TrackerManager) GetEmissions(startTime, endTime time.Time, limit int) []*EmissionRecord {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*EmissionRecord
	for _, em := range tm.emissions {
		if !startTime.IsZero() && em.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && em.Timestamp.After(endTime) {
			continue
		}
		result = append(result, em)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 限制返回数量
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetDeviceSummary 获取设备碳排放汇总.
func (tm *TrackerManager) GetDeviceSummary(deviceID string, days int) (*DeviceSummary, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	footprints, ok := tm.footprints[deviceID]
	if !ok || len(footprints) == 0 {
		return nil, fmt.Errorf("no data for device: %s", deviceID)
	}

	// 计算时间范围
	startTime := time.Now().AddDate(0, 0, -days)
	var periodFootprints []*CarbonFootprint
	for _, fp := range footprints {
		if fp.Timestamp.After(startTime) {
			periodFootprints = append(periodFootprints, fp)
		}
	}

	if len(periodFootprints) == 0 {
		return nil, fmt.Errorf("no data in the last %d days", days)
	}

	totalEnergy := 0.0
	totalCarbon := 0.0
	maxCarbon := 0.0
	minCarbon := math.MaxFloat64

	for _, fp := range periodFootprints {
		totalEnergy += fp.EnergyKWh
		totalCarbon += fp.CarbonKg
		if fp.CarbonKg > maxCarbon {
			maxCarbon = fp.CarbonKg
		}
		if fp.CarbonKg < minCarbon {
			minCarbon = fp.CarbonKg
		}
	}

	avgCarbon := totalCarbon / float64(len(periodFootprints))

	// 获取绿色评分
	score := tm.calculateGreenScore(deviceID)

	return &DeviceSummary{
		DeviceID:       deviceID,
		DeviceName:     periodFootprints[len(periodFootprints)-1].DeviceName,
		PeriodDays:     days,
		TotalEnergyKWh: math.Round(totalEnergy*1000) / 1000,
		TotalCarbonKg:  math.Round(totalCarbon*1000) / 1000,
		AvgCarbonKg:    math.Round(avgCarbon*1000) / 1000,
		MaxCarbonKg:    math.Round(maxCarbon*1000) / 1000,
		MinCarbonKg:    math.Round(minCarbon*1000) / 1000,
		GreenScore:     score,
		DataPoints:     len(periodFootprints),
	}, nil
}

// DeviceSummary 设备碳排放汇总.
type DeviceSummary struct {
	DeviceID       string      `json:"device_id"`
	DeviceName     string      `json:"device_name"`
	PeriodDays     int         `json:"period_days"`
	TotalEnergyKWh float64     `json:"total_energy_kwh"`
	TotalCarbonKg  float64     `json:"total_carbon_kg"`
	AvgCarbonKg    float64     `json:"avg_carbon_kg"`
	MaxCarbonKg    float64     `json:"max_carbon_kg"`
	MinCarbonKg    float64     `json:"min_carbon_kg"`
	GreenScore     *GreenScore `json:"green_score"`
	DataPoints     int         `json:"data_points"`
}

// GetDashboardData 获取仪表盘数据.
func (tm *TrackerManager) GetDashboardData() *DashboardData {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 汇总今日数据
	today := time.Now().Truncate(24 * time.Hour)
	todayCarbon := 0.0
	todayEnergy := 0.0

	// 汇总本月数据
	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())
	monthCarbon := 0.0
	monthEnergy := 0.0

	// 汇总所有设备
	deviceBreakdown := make(map[string]float64)

	for _, footprints := range tm.footprints {
		for _, fp := range footprints {
			if fp.Timestamp.After(today) {
				todayCarbon += fp.CarbonKg
				todayEnergy += fp.EnergyKWh
			}
			if fp.Timestamp.After(monthStart) {
				monthCarbon += fp.CarbonKg
				monthEnergy += fp.EnergyKWh
			}
			deviceBreakdown[fp.DeviceID] += fp.CarbonKg
		}
	}

	// 计算混合强度
	mixedIntensity := tm.calculateMixedIntensity()
	renewablePct := tm.getRenewablePercentage()

	// 趋势数据（最近7天）
	trend := make([]TrendData, 0)
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		dayStart := date.Truncate(24 * time.Hour)
		dayEnd := dayStart.Add(24 * time.Hour)

		dayCarbon := 0.0
		dayEnergy := 0.0
		for _, footprints := range tm.footprints {
			for _, fp := range footprints {
				if fp.Timestamp.After(dayStart) && fp.Timestamp.Before(dayEnd) {
					dayCarbon += fp.CarbonKg
					dayEnergy += fp.EnergyKWh
				}
			}
		}

		trend = append(trend, TrendData{
			Date:      dayStart,
			EnergyKWh: dayEnergy,
			CarbonKg:  dayCarbon,
		})
	}

	// 估算年排放
	yearEstimate := monthCarbon * 12 / 1000 // 吨

	return &DashboardData{
		TodayCarbonKg:   math.Round(todayCarbon*1000) / 1000,
		TodayEnergyKWh:  math.Round(todayEnergy*1000) / 1000,
		MonthCarbonKg:   math.Round(monthCarbon*1000) / 1000,
		MonthEnergyKWh:  math.Round(monthEnergy*1000) / 1000,
		YearCarbonT:     math.Round(yearEstimate*100) / 100,
		CarbonIntensity: math.Round(mixedIntensity*100) / 100,
		GreenEnergyPct:  math.Round(renewablePct*100) / 100,
		Trend:           trend,
		Timestamp:       time.Now(),
	}
}

// DashboardData 仪表盘数据.
type DashboardData struct {
	TodayCarbonKg   float64     `json:"today_carbon_kg"`
	TodayEnergyKWh  float64     `json:"today_energy_kwh"`
	MonthCarbonKg   float64     `json:"month_carbon_kg"`
	MonthEnergyKWh  float64     `json:"month_energy_kwh"`
	YearCarbonT     float64     `json:"year_carbon_t"`
	CarbonIntensity float64     `json:"carbon_intensity"`
	GreenEnergyPct  float64     `json:"green_energy_pct"`
	Trend           []TrendData `json:"trend"`
	Timestamp       time.Time   `json:"timestamp"`
}

// TrendData 趋势数据.
type TrendData struct {
	Date      time.Time `json:"date"`
	EnergyKWh float64   `json:"energy_kwh"`
	CarbonKg  float64   `json:"carbon_kg"`
}
