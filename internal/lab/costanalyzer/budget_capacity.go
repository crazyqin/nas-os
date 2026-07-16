package costanalyzer

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// BudgetCapacityInput describes a budget and growth scenario for NAS capacity planning.
type BudgetCapacityInput struct {
	MonthlyBudget      float64 `json:"monthly_budget"`         // 月预算，<=0 表示不检查预算
	MonthlyGrowthGB    float64 `json:"monthly_growth_gb"`      // 预计每月新增数据量
	PlanningMonths     int     `json:"planning_months"`        // 规划周期（月）
	TargetUtilization  float64 `json:"target_utilization_pct"` // 目标最高利用率，例如 80
	ExpansionCostPerGB float64 `json:"expansion_cost_per_gb"`  // 扩容 CAPEX，元/GB；<=0 使用默认估算
}

// BudgetCapacityReport combines cost, capacity and ROI signals for a storage pool.
type BudgetCapacityReport struct {
	Currency             string                        `json:"currency"`
	MonthlyBudget        float64                       `json:"monthly_budget"`
	CurrentMonthlyCost   float64                       `json:"current_monthly_cost"`
	ProjectedMonthlyCost float64                       `json:"projected_monthly_cost"`
	BudgetStatus         string                        `json:"budget_status"` // ok/warning/exceeded
	BudgetHeadroom       float64                       `json:"budget_headroom"`
	BudgetUtilizationPct float64                       `json:"budget_utilization_pct"`
	TotalCapacityGB      float64                       `json:"total_capacity_gb"`
	UsedGB               float64                       `json:"used_gb"`
	FreeGB               float64                       `json:"free_gb"`
	CurrentUtilization   float64                       `json:"current_utilization_pct"`
	TargetUtilization    float64                       `json:"target_utilization_pct"`
	MonthsUntilTarget    int                           `json:"months_until_target"` // -1 表示周期内不会触达
	ExpansionNeededGB    float64                       `json:"expansion_needed_gb"`
	ExpansionCost        float64                       `json:"expansion_cost"`
	CostPerUsedGB        float64                       `json:"cost_per_used_gb"`
	PotentialMonthlySave float64                       `json:"potential_monthly_save"`
	PotentialAnnualSave  float64                       `json:"potential_annual_save"`
	QuickROI             *ROIResult                    `json:"quick_roi,omitempty"`
	ByType               []BudgetCapacityTypeBreakdown `json:"by_type"`
	Recommendations      []string                      `json:"recommendations"`
	GeneratedAt          time.Time                     `json:"generated_at"`
}

// BudgetCapacityTypeBreakdown shows per-media capacity/cost utilization.
type BudgetCapacityTypeBreakdown struct {
	StorageType    StorageType `json:"storage_type"`
	CapacityGB     float64     `json:"capacity_gb"`
	UsedGB         float64     `json:"used_gb"`
	MonthlyCost    float64     `json:"monthly_cost"`
	UtilizationPct float64     `json:"utilization_pct"`
	CostPerUsedGB  float64     `json:"cost_per_used_gb"`
}

// AnalyzeBudgetCapacity builds a Synology/TrueNAS-style resource and budget view.
func (a *Analyzer) AnalyzeBudgetCapacity(assets []*StorageAsset, input *BudgetCapacityInput) (*BudgetCapacityReport, error) {
	if input == nil {
		input = &BudgetCapacityInput{}
	}
	if input.PlanningMonths <= 0 {
		input.PlanningMonths = 12
	}
	if input.MonthlyGrowthGB < 0 {
		return nil, fmt.Errorf("monthly growth must be >= 0")
	}
	if input.TargetUtilization <= 0 {
		input.TargetUtilization = 80
	}
	if input.TargetUtilization > 100 {
		return nil, fmt.Errorf("target utilization must be <= 100")
	}
	if input.ExpansionCostPerGB <= 0 {
		input.ExpansionCostPerGB = 1.2
	}

	report := &BudgetCapacityReport{
		Currency:          a.config.DefaultCurrency,
		MonthlyBudget:     round2(input.MonthlyBudget),
		TargetUtilization: round2(input.TargetUtilization),
		GeneratedAt:       time.Now(),
	}

	byTypeMap := map[StorageType]*BudgetCapacityTypeBreakdown{}
	for _, asset := range assets {
		if asset == nil {
			continue
		}
		capacityGB := bytesToGB(asset.CapacityBytes)
		usedGB := bytesToGB(asset.UsedBytes)
		monthlyCost := a.CalculateCostForAsset(asset) + asset.MonthlyOpex

		report.TotalCapacityGB += capacityGB
		report.UsedGB += usedGB
		report.CurrentMonthlyCost += monthlyCost

		bd := byTypeMap[asset.Type]
		if bd == nil {
			bd = &BudgetCapacityTypeBreakdown{StorageType: asset.Type}
			byTypeMap[asset.Type] = bd
		}
		bd.CapacityGB += capacityGB
		bd.UsedGB += usedGB
		bd.MonthlyCost += monthlyCost
	}

	report.TotalCapacityGB = round2(report.TotalCapacityGB)
	report.UsedGB = round2(report.UsedGB)
	report.FreeGB = round2(report.TotalCapacityGB - report.UsedGB)
	report.CurrentMonthlyCost = round2(report.CurrentMonthlyCost)
	if report.TotalCapacityGB > 0 {
		report.CurrentUtilization = round2(report.UsedGB / report.TotalCapacityGB * 100)
	}
	if report.UsedGB > 0 {
		report.CostPerUsedGB = round2(report.CurrentMonthlyCost / report.UsedGB)
	}

	projectedUsed := report.UsedGB + input.MonthlyGrowthGB*float64(input.PlanningMonths)
	report.ProjectedMonthlyCost = round2(a.projectMonthlyCost(projectedUsed, report.UsedGB, report.CurrentMonthlyCost))
	report.MonthsUntilTarget = monthsUntilUtilization(report.UsedGB, report.TotalCapacityGB, input.MonthlyGrowthGB, input.TargetUtilization)

	targetUsableGB := report.TotalCapacityGB * input.TargetUtilization / 100
	if projectedUsed > targetUsableGB {
		report.ExpansionNeededGB = round2(projectedUsed/(input.TargetUtilization/100) - report.TotalCapacityGB)
		report.ExpansionCost = round2(report.ExpansionNeededGB * input.ExpansionCostPerGB)
	}

	if input.MonthlyBudget > 0 {
		report.BudgetHeadroom = round2(input.MonthlyBudget - report.ProjectedMonthlyCost)
		report.BudgetUtilizationPct = round2(report.ProjectedMonthlyCost / input.MonthlyBudget * 100)
		switch {
		case report.ProjectedMonthlyCost > input.MonthlyBudget:
			report.BudgetStatus = "exceeded"
		case report.ProjectedMonthlyCost >= input.MonthlyBudget*0.85:
			report.BudgetStatus = "warning"
		default:
			report.BudgetStatus = "ok"
		}
	} else {
		report.BudgetStatus = "ok"
	}

	report.ByType = make([]BudgetCapacityTypeBreakdown, 0, len(byTypeMap))
	for _, bd := range byTypeMap {
		bd.CapacityGB = round2(bd.CapacityGB)
		bd.UsedGB = round2(bd.UsedGB)
		bd.MonthlyCost = round2(bd.MonthlyCost)
		if bd.CapacityGB > 0 {
			bd.UtilizationPct = round2(bd.UsedGB / bd.CapacityGB * 100)
		}
		if bd.UsedGB > 0 {
			bd.CostPerUsedGB = round2(bd.MonthlyCost / bd.UsedGB)
		}
		report.ByType = append(report.ByType, *bd)
	}
	sort.Slice(report.ByType, func(i, j int) bool {
		return report.ByType[i].MonthlyCost > report.ByType[j].MonthlyCost
	})

	report.PotentialMonthlySave = round2(a.estimateTieringSave(report.ByType))
	report.PotentialAnnualSave = round2(report.PotentialMonthlySave * 12)
	if report.ExpansionCost > 0 && report.PotentialAnnualSave > 0 {
		report.QuickROI, _ = a.CalculateROI(&ROIInput{
			InvestmentCost: report.ExpansionCost,
			AnnualSaving:   report.PotentialAnnualSave,
			ProjectYears:   3,
			DiscountRate:   0.08,
		})
	}

	report.Recommendations = buildBudgetRecommendations(report, input)
	return report, nil
}

func (a *Analyzer) projectMonthlyCost(projectedUsed, currentUsed, currentCost float64) float64 {
	if currentUsed <= 0 || currentCost <= 0 {
		return currentCost
	}
	return currentCost * projectedUsed / currentUsed
}

func (a *Analyzer) estimateTieringSave(breakdowns []BudgetCapacityTypeBreakdown) float64 {
	ssdPrice := priceOrDefault(a.config.PricingRules, StorageTypeSSD, 0.50)
	hddPrice := priceOrDefault(a.config.PricingRules, StorageTypeHDD, 0.20)
	if ssdPrice <= hddPrice {
		return 0
	}

	save := 0.0
	for _, bd := range breakdowns {
		if bd.StorageType == StorageTypeSSD || bd.StorageType == StorageTypeNVMe {
			coldCandidateGB := bd.UsedGB * 0.25
			save += coldCandidateGB * (ssdPrice - hddPrice)
		}
	}
	return save
}

func buildBudgetRecommendations(report *BudgetCapacityReport, input *BudgetCapacityInput) []string {
	recs := make([]string, 0, 4)
	if report.BudgetStatus == "exceeded" {
		recs = append(recs, "预计月成本将超过预算，优先迁移冷数据或下调高成本卷副本策略")
	} else if report.BudgetStatus == "warning" {
		recs = append(recs, "预计月成本接近预算上限，建议提前锁定扩容采购或清理低价值数据")
	}
	if report.MonthsUntilTarget == 0 {
		recs = append(recs, "当前容量已超过目标利用率，建议立即扩容或释放空间")
	} else if report.MonthsUntilTarget > 0 && report.MonthsUntilTarget <= 3 {
		recs = append(recs, "未来 3 个月内将触达目标利用率，建议准备扩容计划")
	}
	if report.PotentialMonthlySave > 0 {
		recs = append(recs, fmt.Sprintf("将约 25%% SSD/NVMe 冷数据迁移到 HDD，每月预计节省 %.2f %s", report.PotentialMonthlySave, report.Currency))
	}
	if len(recs) == 0 {
		recs = append(recs, "预算与容量处于健康区间，保持当前监控即可")
	}
	return recs
}

func monthsUntilUtilization(usedGB, capacityGB, monthlyGrowthGB, targetUtilization float64) int {
	if capacityGB <= 0 {
		return -1
	}
	targetUsed := capacityGB * targetUtilization / 100
	if usedGB >= targetUsed {
		return 0
	}
	if monthlyGrowthGB <= 0 {
		return -1
	}
	return int(math.Ceil((targetUsed - usedGB) / monthlyGrowthGB))
}

func priceOrDefault(rules map[StorageType]PricingRule, storageType StorageType, fallback float64) float64 {
	if rule, ok := rules[storageType]; ok {
		return rule.PricePerGBMonth
	}
	return fallback
}

func bytesToGB(v int64) float64 {
	return float64(v) / (1024 * 1024 * 1024)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
