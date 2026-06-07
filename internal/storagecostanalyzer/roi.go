// Package storagecostanalyzer 存储成本分析器 - ROI 分析
package storagecostanalyzer

import (
	"fmt"
	"math"
	"time"
)

// ROIAnalyzer ROI 分析器.
type ROIAnalyzer struct {
	manager *Manager
}

// NewROIAnalyzer 创建 ROI 分析器.
func NewROIAnalyzer(manager *Manager) *ROIAnalyzer {
	return &ROIAnalyzer{manager: manager}
}

// ROIInput ROI 计算输入.
type ROIInput struct {
	// Tier 存储层级.
	Tier StorageTier
	// InvestmentCost 投资成本.
	InvestmentCost float64
	// AnalysisMonths 分析周期（月）.
	AnalysisMonths int
	// ExpectedLifespanMonths 预期使用寿命（月）.
	ExpectedLifespanMonths int
	// CompressionRatio 压缩比例（0-1）.
	CompressionRatio float64
	// DeduplicationRatio 去重比例（0-1）.
	DeduplicationRatio float64
	// PerformanceGainPercent 性能提升百分比.
	PerformanceGainPercent float64
	// DowntimeReductionHours 年减少宕机小时数.
	DowntimeReductionHours float64
	// CostPerDowntimeHour 每小时宕机成本.
	CostPerDowntimeHour float64
}

// DefaultROIInput 默认 ROI 输入.
func DefaultROIInput(tier StorageTier, investmentCost float64, months int) ROIInput {
	return ROIInput{
		Tier:                   tier,
		InvestmentCost:         investmentCost,
		AnalysisMonths:         months,
		ExpectedLifespanMonths: 36,   // 3年
		CompressionRatio:       0.3,  // 30%压缩
		DeduplicationRatio:     0.2,  // 20%去重
		PerformanceGainPercent: 20,   // 20%性能提升
		DowntimeReductionHours: 10,   // 年减少10小时宕机
		CostPerDowntimeHour:    1000, // 每小时宕机成本1000元
	}
}

// CalculateROI 计算投资回报率.
func (a *ROIAnalyzer) CalculateROI(input ROIInput) (*ROIAnalysis, error) {
	a.manager.mu.RLock()
	defer a.manager.mu.RUnlock()

	ts, ok := a.manager.tiers[input.Tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if input.InvestmentCost <= 0 {
		return nil, fmt.Errorf("%w: investment cost must be positive", ErrInvalidConfig)
	}
	if input.AnalysisMonths <= 0 {
		input.AnalysisMonths = 12
	}

	cfg := ts.config
	usedTB := cfg.UsedTB

	// 1. 成本节约（去重+压缩带来的空间节省）
	dataReductionPercent := input.CompressionRatio + input.DeduplicationRatio*(1-input.CompressionRatio)
	spaceSavedTB := usedTB * dataReductionPercent
	costSavings := spaceSavedTB * cfg.CostPerTBMonth * float64(input.AnalysisMonths)

	// 2. 效率提升收益（性能提升带来的业务价值）
	efficiencyGain := usedTB * 50 * (input.PerformanceGainPercent / 100) * float64(input.AnalysisMonths)

	// 3. 宕机减少收益
	downtimeReduction := input.DowntimeReductionHours * input.CostPerDowntimeHour * float64(input.AnalysisMonths) / 12

	totalBenefits := costSavings + efficiencyGain + downtimeReduction
	netBenefit := totalBenefits - input.InvestmentCost

	roiPercent := 0.0
	if input.InvestmentCost > 0 {
		roiPercent = (netBenefit / input.InvestmentCost) * 100
	}

	// 计算回收期
	paybackMonths := 0
	if totalBenefits > 0 && input.AnalysisMonths > 0 {
		monthlyBenefit := totalBenefits / float64(input.AnalysisMonths)
		if monthlyBenefit > 0 {
			paybackMonths = int(math.Ceil(input.InvestmentCost / monthlyBenefit))
		}
	}

	// 年化 ROI
	annualROI := roiPercent
	if input.AnalysisMonths > 0 && input.AnalysisMonths < 12 {
		annualROI = roiPercent * 12 / float64(input.AnalysisMonths)
	}

	// 收益明细
	breakdown := []ROIBenefitItem{
		{
			Type:        "cost_savings",
			Amount:      costSavings,
			Description: fmt.Sprintf("数据优化节省（压缩%.0f%% + 去重%.0f%%）", input.CompressionRatio*100, input.DeduplicationRatio*100),
		},
		{
			Type:        "efficiency",
			Amount:      efficiencyGain,
			Description: fmt.Sprintf("性能提升收益（提升%.0f%%）", input.PerformanceGainPercent),
		},
		{
			Type:        "downtime",
			Amount:      downtimeReduction,
			Description: fmt.Sprintf("宕机减少收益（年减少%.0f小时）", input.DowntimeReductionHours),
		},
	}

	for i := range breakdown {
		if totalBenefits > 0 {
			breakdown[i].Percentage = (breakdown[i].Amount / totalBenefits) * 100
		}
	}

	return &ROIAnalysis{
		GeneratedAt:          a.manager.nowFunc(),
		Tier:                 input.Tier,
		TierName:             cfg.Name,
		AnalysisPeriodMonths: input.AnalysisMonths,
		TotalInvestment:      input.InvestmentCost,
		TotalBenefits:        totalBenefits,
		NetBenefit:           netBenefit,
		ROIPercent:           roiPercent,
		PaybackPeriodMonths:  paybackMonths,
		AnnualROI:            annualROI,
		CostSavings:          costSavings,
		EfficiencyGain:       efficiencyGain,
		DowntimeReduction:    downtimeReduction,
		BenefitBreakdown:     breakdown,
	}, nil
}

// CompareROI 对比多个投资方案的 ROI.
func (a *ROIAnalyzer) CompareROI(inputs []ROIInput) (*ROIComparison, error) {
	if len(inputs) < 2 {
		return nil, fmt.Errorf("%w: at least 2 inputs required", ErrInvalidConfig)
	}

	results := make([]*ROIAnalysis, 0, len(inputs))
	for _, input := range inputs {
		result, err := a.CalculateROI(input)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	// 找出最优方案
	bestROI := 0
	bestPayback := 0
	maxROIPercent := -math.MaxFloat64
	minPayback := math.MaxInt64

	for i, r := range results {
		if r.ROIPercent > maxROIPercent {
			maxROIPercent = r.ROIPercent
			bestROI = i
		}
		if r.PaybackPeriodMonths < minPayback {
			minPayback = r.PaybackPeriodMonths
			bestPayback = i
		}
	}

	return &ROIComparison{
		GeneratedAt: a.manager.nowFunc(),
		Results:     results,
		BestROI:     bestROI,
		BestPayback: bestPayback,
	}, nil
}

// ROIComparison ROI 对比结果.
type ROIComparison struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Results 各方案 ROI 结果.
	Results []*ROIAnalysis `json:"results"`
	// BestROI 最佳 ROI 方案索引.
	BestROI int `json:"bestROI"`
	// BestPayback 最快回收方案索引.
	BestPayback int `json:"bestPayback"`
}

// EstimateDataOptimizationROI 估算数据优化 ROI.
func (a *ROIAnalyzer) EstimateDataOptimizationROI(
	tier StorageTier,
	dedupRatio, compressionRatio float64,
	implementationCostPerTB float64,
) (*DataOptimizationROI, error) {
	a.manager.mu.RLock()
	defer a.manager.mu.RUnlock()

	ts, ok := a.manager.tiers[tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if dedupRatio < 0 || dedupRatio > 1 {
		return nil, fmt.Errorf("%w: dedup ratio must be between 0 and 1", ErrInvalidConfig)
	}
	if compressionRatio < 0 || compressionRatio > 1 {
		return nil, fmt.Errorf("%w: compression ratio must be between 0 and 1", ErrInvalidConfig)
	}

	cfg := ts.config
	originalTB := cfg.UsedTB

	// 计算节省空间
	dedupSavingsTB := originalTB * dedupRatio
	afterDedup := originalTB - dedupSavingsTB
	compressionSavingsTB := afterDedup * compressionRatio
	totalSavingsTB := dedupSavingsTB + compressionSavingsTB

	// 计算成本节省
	monthlyCostSaving := totalSavingsTB * cfg.CostPerTBMonth
	annualCostSaving := monthlyCostSaving * 12

	// 计算实施成本
	implementationCost := totalSavingsTB * implementationCostPerTB

	// 计算 ROI
	netBenefit12Months := annualCostSaving - implementationCost
	roiPercent12Months := 0.0
	if implementationCost > 0 {
		roiPercent12Months = (netBenefit12Months / implementationCost) * 100
	}

	// 回收期
	paybackMonths := 0
	if monthlyCostSaving > 0 {
		paybackMonths = int(math.Ceil(implementationCost / monthlyCostSaving))
	}

	// 3年和5年 ROI
	netBenefit3Years := annualCostSaving*3 - implementationCost
	netBenefit5Years := annualCostSaving*5 - implementationCost

	roiPercent3Years := 0.0
	roiPercent5Years := 0.0
	if implementationCost > 0 {
		roiPercent3Years = (netBenefit3Years / implementationCost) * 100
		roiPercent5Years = (netBenefit5Years / implementationCost) * 100
	}

	return &DataOptimizationROI{
		GeneratedAt:        a.manager.nowFunc(),
		Tier:               tier,
		TierName:           cfg.Name,
		OriginalDataTB:     originalTB,
		DeduplicationRatio: dedupRatio,
		CompressionRatio:   compressionRatio,
		TotalSavingsTB:     totalSavingsTB,
		MonthlyCostSaving:  monthlyCostSaving,
		AnnualCostSaving:   annualCostSaving,
		ImplementationCost: implementationCost,
		PaybackMonths:      paybackMonths,
		ROI12Months:        roiPercent12Months,
		ROI3Years:          roiPercent3Years,
		ROI5Years:          roiPercent5Years,
		NetBenefit12Months: netBenefit12Months,
		NetBenefit3Years:   netBenefit3Years,
		NetBenefit5Years:   netBenefit5Years,
	}, nil
}

// DataOptimizationROI 数据优化 ROI.
type DataOptimizationROI struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// TierName 层级名称.
	TierName string `json:"tierName"`
	// OriginalDataTB 原始数据量（TB）.
	OriginalDataTB float64 `json:"originalDataTB"`
	// DeduplicationRatio 去重比例.
	DeduplicationRatio float64 `json:"deduplicationRatio"`
	// CompressionRatio 压缩比例.
	CompressionRatio float64 `json:"compressionRatio"`
	// TotalSavingsTB 总节省空间（TB）.
	TotalSavingsTB float64 `json:"totalSavingsTB"`
	// MonthlyCostSaving 月成本节省.
	MonthlyCostSaving float64 `json:"monthlyCostSaving"`
	// AnnualCostSaving 年成本节省.
	AnnualCostSaving float64 `json:"annualCostSaving"`
	// ImplementationCost 实施成本.
	ImplementationCost float64 `json:"implementationCost"`
	// PaybackMonths 回收期（月）.
	PaybackMonths int `json:"paybackMonths"`
	// ROI12Months 12个月 ROI（%）.
	ROI12Months float64 `json:"roi12Months"`
	// ROI3Years 3年 ROI（%）.
	ROI3Years float64 `json:"roi3Years"`
	// ROI5Years 5年 ROI（%）.
	ROI5Years float64 `json:"roi5Years"`
	// NetBenefit12Months 12个月净收益.
	NetBenefit12Months float64 `json:"netBenefit12Months"`
	// NetBenefit3Years 3年净收益.
	NetBenefit3Years float64 `json:"netBenefit3Years"`
	// NetBenefit5Years 5年净收益.
	NetBenefit5Years float64 `json:"netBenefit5Years"`
}

// CalculateNPV 计算净现值.
func (a *ROIAnalyzer) CalculateNPV(initialInvestment float64, monthlyCashFlows []float64, annualDiscountRate float64) (float64, error) {
	if initialInvestment < 0 {
		return 0, fmt.Errorf("%w: initial investment must be non-negative", ErrInvalidConfig)
	}
	if annualDiscountRate < 0 || annualDiscountRate > 1 {
		return 0, fmt.Errorf("%w: discount rate must be between 0 and 1", ErrInvalidConfig)
	}

	monthlyDiscountRate := math.Pow(1+annualDiscountRate, 1.0/12) - 1
	npv := -initialInvestment

	for i, cf := range monthlyCashFlows {
		period := float64(i + 1)
		pv := cf / math.Pow(1+monthlyDiscountRate, period)
		npv += pv
	}

	return npv, nil
}

// CalculateIRR 计算内部收益率.
func (a *ROIAnalyzer) CalculateIRR(initialInvestment float64, monthlyCashFlows []float64) (float64, error) {
	if initialInvestment <= 0 {
		return 0, fmt.Errorf("%w: initial investment must be positive", ErrInvalidConfig)
	}
	if len(monthlyCashFlows) == 0 {
		return 0, fmt.Errorf("%w: cash flows cannot be empty", ErrInvalidConfig)
	}

	// 使用二分法求解月化 IRR
	low := -0.5 // 月化收益率下限
	high := 0.5 // 月化收益率上限
	tolerance := 0.0001
	maxIterations := 1000

	for i := 0; i < maxIterations; i++ {
		mid := (low + high) / 2
		// 直接使用月化收益率计算 NPV
		npv := -initialInvestment
		for j, cf := range monthlyCashFlows {
			period := float64(j + 1)
			pv := cf / math.Pow(1+mid, period)
			npv += pv
		}

		if math.Abs(npv) < tolerance {
			// 返回年化 IRR
			monthlyIRR := mid
			annualIRR := math.Pow(1+monthlyIRR, 12) - 1
			return annualIRR, nil
		}

		if npv > 0 {
			low = mid
		} else {
			high = mid
		}
	}

	return 0, fmt.Errorf("%w: IRR calculation did not converge", ErrInsufficientData)
}

// GenerateCashFlows 生成月度现金流.
func (a *ROIAnalyzer) GenerateCashFlows(tier StorageTier, investmentCost float64, months int) ([]float64, error) {
	a.manager.mu.RLock()
	defer a.manager.mu.RUnlock()

	ts, ok := a.manager.tiers[tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if months <= 0 {
		months = 12
	}

	cfg := ts.config
	usedTB := cfg.UsedTB

	// 假设每月收益 = 数据优化节省 + 性能提升价值
	monthlySavings := usedTB * cfg.CostPerTBMonth * 0.2 // 20%成本优化
	monthlyEfficiency := usedTB * 50 * 0.2              // 性能提升价值
	monthlyBenefit := monthlySavings + monthlyEfficiency

	cashFlows := make([]float64, months)
	for i := range cashFlows {
		cashFlows[i] = monthlyBenefit
	}

	return cashFlows, nil
}
