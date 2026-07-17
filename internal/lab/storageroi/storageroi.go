// Package storageroi 提供存储投资回报率（ROI）可视化分析功能。
// 结合磁盘采购成本、容量利用率、磁盘寿命、TCO（总拥有成本）等数据，
// 计算 ROI 评分并给出优化建议，帮助用户优化存储投资决策。
package storageroi

import "time"

// DiskType 磁盘类型.
type DiskType string

const (
	DiskTypeHDD   DiskType = "hdd"
	DiskTypeSSD   DiskType = "ssd"
	DiskTypeNVMe  DiskType = "nvme"
	DiskTypeSAS   DiskType = "sas"
	DiskTypeSATAS DiskType = "sata"
)

// DiskStatus 磁盘状态.
type DiskStatus string

const (
	DiskStatusActive   DiskStatus = "active"
	DiskStatusDegraded DiskStatus = "degraded"
	DiskStatusFailed   DiskStatus = "failed"
	DiskStatusSpare    DiskStatus = "spare"
	DiskStatusRetired  DiskStatus = "retired"
)

// DiskCostRecord 磁盘采购成本记录.
type DiskCostRecord struct {
	ID            string    `json:"id"`
	SerialNumber  string    `json:"serial_number"`
	Model         string    `json:"model"`
	Vendor        string    `json:"vendor"`
	DiskType      DiskType  `json:"disk_type"`
	CapacityBytes int64     `json:"capacity_bytes"`
	PurchasePrice float64   `json:"purchase_price"` // 采购价格（元）
	Currency      string    `json:"currency"`
	PurchaseDate  time.Time `json:"purchase_date"`
	WarrantyYears int       `json:"warranty_years"`        // 保修年限
	VendorName    string    `json:"vendor_name,omitempty"` // 供应商名称
	InvoiceNumber string    `json:"invoice_number,omitempty"`
}

// CapacityUtilization 容量利用率追踪.
type CapacityUtilization struct {
	DiskID         string    `json:"disk_id"`
	Timestamp      time.Time `json:"timestamp"`
	TotalBytes     int64     `json:"total_bytes"`
	UsedBytes      int64     `json:"used_bytes"`
	ReservedBytes  int64     `json:"reserved_bytes"`
	IOPS           float64   `json:"iops,omitempty"`
	ThroughputMBps float64   `json:"throughput_mbps,omitempty"`
}

// UtilizationPercent 计算利用率百分比.
func (u *CapacityUtilization) UtilizationPercent() float64 {
	if u.TotalBytes <= 0 {
		return 0
	}
	return float64(u.UsedBytes) / float64(u.TotalBytes) * 100.0
}

// AvailableBytes 可用字节数.
func (u *CapacityUtilization) AvailableBytes() int64 {
	avail := u.TotalBytes - u.UsedBytes - u.ReservedBytes
	if avail < 0 {
		return 0
	}
	return avail
}

// LifetimeTracker 磁盘寿命追踪与替换预测.
type LifetimeTracker struct {
	DiskID             string     `json:"disk_id"`
	SerialNumber       string     `json:"serial_number"`
	ManufactureDate    time.Time  `json:"manufacture_date"`
	PurchaseDate       time.Time  `json:"purchase_date"`
	PowerOnHours       float64    `json:"power_on_hours"` // 已通电小时数
	EstimatedTBW       float64    `json:"estimated_tbw"`  // 估计写入寿命（TB）
	ActualTBW          float64    `json:"actual_tbw"`     // 实际已写入量（TB）
	WarrantyEnd        time.Time  `json:"warranty_end"`
	Status             DiskStatus `json:"status"`
	HealthScore        float64    `json:"health_score"` // 健康评分 0-100
	ReallocatedSectors int        `json:"reallocated_sectors"`
	TemperatureMax     float64    `json:"temperature_max"` // 最高温度记录
	LastChecked        time.Time  `json:"last_checked"`
}

// EstimatedRemainingHours 预估剩余寿命（小时）.
func (l *LifetimeTracker) EstimatedRemainingHours() float64 {
	if l.EstimatedTBW > 0 && l.ActualTBW >= l.EstimatedTBW {
		return 0
	}
	// 基于写入量比例估算
	if l.EstimatedTBW > 0 {
		ratio := 1.0 - l.ActualTBW/l.EstimatedTBW
		if l.PowerOnHours > 0 {
			avgWriteRate := l.ActualTBW / l.PowerOnHours
			if avgWriteRate > 0 {
				remaining := (l.EstimatedTBW - l.ActualTBW) / avgWriteRate
				_ = ratio
				return remaining
			}
		}
	}
	return 0
}

// EstimatedReplacementDate 预估替换日期.
func (l *LifetimeTracker) EstimatedReplacementDate() time.Time {
	remaining := l.EstimatedRemainingHours()
	if remaining <= 0 {
		return time.Now()
	}
	// 假设每天平均通电 24 小时
	days := remaining / 24.0
	if days < 1 {
		days = 1
	}
	return time.Now().AddDate(0, 0, int(days))
}

// NeedsReplacement 是否需要替换.
func (l *LifetimeTracker) NeedsReplacement() bool {
	if l.Status == DiskStatusFailed {
		return true
	}
	if l.HealthScore < 30 {
		return true
	}
	if l.EstimatedRemainingHours() < 24*30 { // 少于 30 天
		return true
	}
	if l.ReallocatedSectors > 100 {
		return true
	}
	return false
}

// TCOReport 总拥有成本报告.
type TCOReport struct {
	DiskID          string  `json:"disk_id"`
	SerialNumber    string  `json:"serial_number"`
	PurchaseCost    float64 `json:"purchase_cost"`     // 采购成本
	ElectricityCost float64 `json:"electricity_cost"`  // 电力成本（累计）
	MaintenanceCost float64 `json:"maintenance_cost"`  // 维护成本（累计）
	ReplacementCost float64 `json:"replacement_cost"`  // 替换成本（累计）
	TotalCost       float64 `json:"total_cost"`        // 总拥有成本
	MonthsInService int     `json:"months_in_service"` // 已使用月数
	CostPerMonth    float64 `json:"cost_per_month"`    // 每月成本
	CostPerTB       float64 `json:"cost_per_tb"`       // 每TB成本
	Currency        string  `json:"currency"`
}

// ROIScore ROI评分.
type ROIScore struct {
	Score             float64          `json:"score"`              // 0-100
	Grade             string           `json:"grade"`              // A/B/C/D/F
	CapacityScore     float64          `json:"capacity_score"`     // 容量利用率评分
	CostScore         float64          `json:"cost_score"`         // 成本效率评分
	HealthScore       float64          `json:"health_score"`       // 健康状况评分
	LifetimeScore     float64          `json:"lifetime_score"`     // 寿命评分
	OverallEfficiency float64          `json:"overall_efficiency"` // 综合效率
	Recommendations   []Recommendation `json:"recommendations"`
}

// Recommendation 优化建议.
type Recommendation struct {
	Priority         string  `json:"priority"` // high/medium/low
	Category         string  `json:"category"` // capacity/cost/health/lifetime
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	PotentialSavings float64 `json:"potential_savings,omitempty"` // 潜在节省（元）
}

// ROICalculator 存储投资回报率计算器.
type ROICalculator struct {
	// electricityRate 电费单价（元/kWh）
	electricityRate float64
	// diskPowerWatts 磁盘功耗瓦数（按类型）
	diskPowerWatts map[DiskType]float64
	// annualMaintenanceRate 年维护费率（占采购价格百分比）
	annualMaintenanceRate float64
}

// NewROICalculator 创建 ROI 计算器.
func NewROICalculator() *ROICalculator {
	return &ROICalculator{
		electricityRate: 0.8, // 默认 0.8 元/kWh
		diskPowerWatts: map[DiskType]float64{
			DiskTypeHDD:   8.0,
			DiskTypeSSD:   3.0,
			DiskTypeNVMe:  5.0,
			DiskTypeSAS:   12.0,
			DiskTypeSATAS: 6.0,
		},
		annualMaintenanceRate: 0.02, // 默认年维护费 2%
	}
}

// SetElectricityRate 设置电费单价（元/kWh）.
func (c *ROICalculator) SetElectricityRate(rate float64) {
	if rate > 0 {
		c.electricityRate = rate
	}
}

// SetDiskPowerWatts 设置磁盘类型功耗（瓦）.
func (c *ROICalculator) SetDiskPowerWatts(diskType DiskType, watts float64) {
	if c.diskPowerWatts == nil {
		c.diskPowerWatts = make(map[DiskType]float64)
	}
	c.diskPowerWatts[diskType] = watts
}

// SetAnnualMaintenanceRate 设置年维护费率.
func (c *ROICalculator) SetAnnualMaintenanceRate(rate float64) {
	if rate >= 0 {
		c.annualMaintenanceRate = rate
	}
}

// CalculateTCO 计算总拥有成本.
func (c *ROICalculator) CalculateTCO(cost *DiskCostRecord, lifetime *LifetimeTracker) *TCOReport {
	if cost == nil {
		return &TCOReport{}
	}

	currency := cost.Currency
	if currency == "" {
		currency = "CNY"
	}

	report := &TCOReport{
		DiskID:       cost.ID,
		SerialNumber: cost.SerialNumber,
		PurchaseCost: cost.PurchasePrice,
		Currency:     currency,
	}

	// 计算使用月数
	startDate := cost.PurchaseDate
	if !lifetime.ManufactureDate.IsZero() && lifetime.ManufactureDate.After(startDate) {
		startDate = lifetime.ManufactureDate
	}
	monthsInService := int(time.Since(startDate).Hours() / 24 / 30)
	if monthsInService < 1 {
		monthsInService = 1
	}
	report.MonthsInService = monthsInService

	// 电力成本
	powerWatts := c.diskPowerWatts[cost.DiskType]
	if powerWatts == 0 {
		powerWatts = 8.0 // 默认 8W
	}
	powerKW := powerWatts / 1000.0
	hoursInService := float64(monthsInService) * 30 * 24
	report.ElectricityCost = powerKW * hoursInService * c.electricityRate

	// 维护成本
	report.MaintenanceCost = cost.PurchasePrice * c.annualMaintenanceRate * float64(monthsInService) / 12.0

	// 替换成本（如果磁盘已替换或需要替换）
	if lifetime.Status == DiskStatusFailed || lifetime.Status == DiskStatusRetired {
		report.ReplacementCost = cost.PurchasePrice // 假设替换成本等于采购成本
	}

	// 总成本
	report.TotalCost = report.PurchaseCost + report.ElectricityCost + report.MaintenanceCost + report.ReplacementCost

	// 每月成本
	report.CostPerMonth = report.TotalCost / float64(monthsInService)

	// 每TB成本
	capacityTB := float64(cost.CapacityBytes) / 1e12 // bytes to TB
	if capacityTB > 0 {
		report.CostPerTB = report.TotalCost / capacityTB
	}

	return report
}

// CalculateTCOBatch 批量计算 TCO.
func (c *ROICalculator) CalculateTCOBatch(costs []*DiskCostRecord, lifetimes map[string]*LifetimeTracker) []*TCOReport {
	reports := make([]*TCOReport, 0, len(costs))
	for _, cost := range costs {
		lifetime := lifetimes[cost.ID]
		if lifetime == nil {
			lifetime = &LifetimeTracker{
				PurchaseDate: cost.PurchaseDate,
				Status:       DiskStatusActive,
			}
		}
		reports = append(reports, c.CalculateTCO(cost, lifetime))
	}
	return reports
}

// CalculateROI 计算 ROI 评分.
func (c *ROICalculator) CalculateROI(
	utilizations []*CapacityUtilization,
	tco *TCOReport,
	lifetime *LifetimeTracker,
	cost *DiskCostRecord,
) *ROIScore {
	score := &ROIScore{
		Score:           0,
		Recommendations: []Recommendation{},
	}

	// === 容量利用率评分 (25%) ===
	avgUtilization := 0.0
	if len(utilizations) > 0 {
		for _, u := range utilizations {
			avgUtilization += u.UtilizationPercent()
		}
		avgUtilization /= float64(len(utilizations))
	}
	// 最佳利用率在 60%-85% 之间
	if avgUtilization >= 60 && avgUtilization <= 85 {
		score.CapacityScore = 100
	} else if avgUtilization > 85 {
		// 过高，有风险，线性扣分（每超1%扣4分）
		score.CapacityScore = 100 - (avgUtilization-85)*4
		if score.CapacityScore < 20 {
			score.CapacityScore = 20
		}
	} else if avgUtilization < 30 {
		// 过低，浪费
		score.CapacityScore = avgUtilization * 2
	} else {
		// 30%-60% 之间线性评分
		score.CapacityScore = 40 + (avgUtilization-30)*2
	}
	if score.CapacityScore < 0 {
		score.CapacityScore = 0
	}
	if score.CapacityScore > 100 {
		score.CapacityScore = 100
	}

	// === 成本效率评分 (25%) ===
	costPerTB := tco.CostPerTB
	// 基准线：HDD ~50元/TB, SSD ~200元/TB, NVMe ~400元/TB
	benchmarkPerTB := 100.0
	if cost != nil {
		switch cost.DiskType {
		case DiskTypeHDD:
			benchmarkPerTB = 50
		case DiskTypeSSD:
			benchmarkPerTB = 200
		case DiskTypeNVMe:
			benchmarkPerTB = 400
		case DiskTypeSAS:
			benchmarkPerTB = 150
		default:
			benchmarkPerTB = 100
		}
	}
	if costPerTB > 0 {
		ratio := benchmarkPerTB / costPerTB
		if ratio >= 1 {
			score.CostScore = 100
		} else {
			score.CostScore = ratio * 100
		}
	} else {
		score.CostScore = 50
	}
	if score.CostScore < 0 {
		score.CostScore = 0
	}
	if score.CostScore > 100 {
		score.CostScore = 100
	}

	// === 健康状况评分 (25%) ===
	score.HealthScore = lifetime.HealthScore
	if score.HealthScore <= 0 {
		score.HealthScore = 50 // 默认中等
	}
	if score.HealthScore > 100 {
		score.HealthScore = 100
	}

	// === 寿命评分 (25%) ===
	remainingHours := lifetime.EstimatedRemainingHours()
	if lifetime.Status == DiskStatusFailed {
		score.LifetimeScore = 0
	} else if remainingHours <= 0 {
		score.LifetimeScore = 10
	} else {
		// 5年 = 43800小时为满分基准
		maxHours := 43800.0
		score.LifetimeScore = (remainingHours / maxHours) * 100
		if score.LifetimeScore > 100 {
			score.LifetimeScore = 100
		}
	}

	// === 综合评分 ===
	score.Score = (score.CapacityScore + score.CostScore + score.HealthScore + score.LifetimeScore) / 4.0
	score.OverallEfficiency = score.Score / 100.0

	// === 评级 ===
	score.Grade = scoreToGrade(score.Score)

	// === 优化建议 ===
	score.Recommendations = c.generateRecommendations(score, utilizations, tco, lifetime, cost, avgUtilization)

	return score
}

// calculateAverageUtilization 计算平均利用率.
func calculateAverageUtilization(utilizations []*CapacityUtilization) float64 {
	if len(utilizations) == 0 {
		return 0
	}
	total := 0.0
	for _, u := range utilizations {
		total += u.UtilizationPercent()
	}
	return total / float64(len(utilizations))
}

// generateRecommendations 生成优化建议.
func (c *ROICalculator) generateRecommendations(
	score *ROIScore,
	utilizations []*CapacityUtilization,
	tco *TCOReport,
	lifetime *LifetimeTracker,
	cost *DiskCostRecord,
	avgUtilization float64,
) []Recommendation {
	recs := []Recommendation{}

	// 容量利用率建议
	if avgUtilization > 90 {
		recs = append(recs, Recommendation{
			Priority:    "high",
			Category:    "capacity",
			Title:       "磁盘容量即将耗尽",
			Description: "当前容量利用率超过90%，建议立即扩容或清理不必要的数据，避免写入失败风险。",
		})
	} else if avgUtilization > 85 {
		recs = append(recs, Recommendation{
			Priority:    "medium",
			Category:    "capacity",
			Title:       "磁盘容量利用率偏高",
			Description: "当前容量利用率超过85%，建议规划扩容方案或制定数据归档策略。",
		})
	} else if avgUtilization < 30 {
		potentialSavings := 0.0
		if tco != nil && cost != nil {
			// 估算可通过缩容节省的成本
			potentialSavings = tco.TotalCost * (1 - avgUtilization/60)
			if potentialSavings < 0 {
				potentialSavings = 0
			}
		}
		recs = append(recs, Recommendation{
			Priority:         "medium",
			Category:         "capacity",
			Title:            "磁盘容量利用率过低",
			Description:      "当前容量利用率低于30%，存储资源严重浪费。考虑迁移到更小容量的磁盘以降低成本。",
			PotentialSavings: potentialSavings,
		})
	}

	// 成本优化建议
	if score.CostScore < 60 && cost != nil {
		recs = append(recs, Recommendation{
			Priority:    "medium",
			Category:    "cost",
			Title:       "存储成本效率偏低",
			Description: "当前每TB成本高于行业基准。建议在采购新磁盘时比较不同供应商报价，或考虑性价比更高的存储类型。",
		})
	}
	if tco != nil && tco.ElectricityCost > tco.PurchaseCost*0.5 {
		recs = append(recs, Recommendation{
			Priority:         "low",
			Category:         "cost",
			Title:            "电力成本占比过高",
			Description:      "电力成本已超过采购成本的50%。建议评估低功耗磁盘型号或优化冷却系统。",
			PotentialSavings: tco.ElectricityCost * 0.2,
		})
	}

	// 健康状况建议
	if lifetime.HealthScore < 50 {
		recs = append(recs, Recommendation{
			Priority:    "high",
			Category:    "health",
			Title:       "磁盘健康状况不佳",
			Description: "磁盘健康评分低于50，存在较高故障风险。建议尽快备份数据并准备替换磁盘。",
		})
	}
	if lifetime.ReallocatedSectors > 50 {
		recs = append(recs, Recommendation{
			Priority:    "high",
			Category:    "health",
			Title:       "磁盘存在大量重分配扇区",
			Description: "重分配扇区数量较多，表明磁盘表面存在物理损伤。建议立即更换磁盘。",
		})
	}

	// 寿命建议
	if lifetime.NeedsReplacement() {
		recs = append(recs, Recommendation{
			Priority:    "high",
			Category:    "lifetime",
			Title:       "磁盘需要替换",
			Description: "磁盘已达到替换条件（故障/健康差/寿命不足/坏扇区过多）。建议立即采购替换磁盘。",
		})
	} else if lifetime.EstimatedRemainingHours() < 24*90 { // 3个月
		recs = append(recs, Recommendation{
			Priority:    "medium",
			Category:    "lifetime",
			Title:       "磁盘寿命即将到期",
			Description: "预估剩余寿命不足3个月，建议提前采购替换磁盘并规划数据迁移。",
		})
	}

	if len(recs) == 0 {
		recs = append(recs, Recommendation{
			Priority:    "low",
			Category:    "general",
			Title:       "存储运行状况良好",
			Description: "各项指标均在正常范围内，无需特别操作。建议继续保持定期监控。",
		})
	}

	return recs
}

// scoreToGrade 评分转评级.
func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}
