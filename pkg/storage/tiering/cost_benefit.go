// Package tiering provides cost-benefit analysis for storage tiering.
// Helps evaluate ROI and optimize storage costs based on access patterns.
package tiering

import (
	"context"
	"math"
	"sync"
	"time"
)

// CostBenefitReport represents a comprehensive cost-benefit analysis
type CostBenefitReport struct {
	GeneratedAt               time.Time            `json:"generated_at"`
	Period                    TimeRange            `json:"period"`
	StorageCosts              StorageCostBreakdown `json:"storage_costs"`
	PerformanceValue          PerformanceValue     `json:"performance_value"`
	ROI                       ROIAnalysis          `json:"roi"`
	CostSavings               CostSavings          `json:"cost_savings"`
	OptimizationOpportunities []CostOpportunity    `json:"optimization_opportunities"`
	Recommendations           []CostRecommendation `json:"recommendations"`
}

// StorageCostBreakdown represents storage costs by tier
type StorageCostBreakdown struct {
	HotTierCost  TierCostDetail `json:"hot_tier_cost"`
	WarmTierCost TierCostDetail `json:"warm_tier_cost"`
	ColdTierCost TierCostDetail `json:"cold_tier_cost"`
	TotalMonthly float64        `json:"total_monthly_usd"`
	TotalAnnual  float64        `json:"total_annual_usd"`
	CostPerGB    float64        `json:"cost_per_gb_usd"`
}

// TierCostDetail represents cost details for a single tier
type TierCostDetail struct {
	Tier             Tier    `json:"tier"`
	CapacityGB       float64 `json:"capacity_gb"`
	UsedGB           float64 `json:"used_gb"`
	CostPerGBMonthly float64 `json:"cost_per_gb_monthly_usd"`
	MonthlyCost      float64 `json:"monthly_cost_usd"`
	AnnualCost       float64 `json:"annual_cost_usd"`
	Efficiency       float64 `json:"efficiency_percent"`
	CostEfficiency   float64 `json:"cost_efficiency_score"`
}

// PerformanceValue represents the value derived from performance improvements
type PerformanceValue struct {
	ReducedLatencyValue   float64 `json:"reduced_latency_value_usd"`
	ThroughputValue       float64 `json:"throughput_value_usd"`
	IOPSValue             float64 `json:"iops_value_usd"`
	UserProductivityValue float64 `json:"user_productivity_value_usd"`
	TotalPerformanceValue float64 `json:"total_performance_value_usd"`
}

// ROIAnalysis represents return on investment analysis
type ROIAnalysis struct {
	InitialInvestment      float64 `json:"initial_investment_usd"`
	MonthlyOperationalCost float64 `json:"monthly_operational_cost_usd"`
	MonthlySavings         float64 `json:"monthly_savings_usd"`
	MonthlyValue           float64 `json:"monthly_value_usd"`
	PaybackPeriodMonths    float64 `json:"payback_period_months"`
	AnnualROI              float64 `json:"annual_roi_percent"`
	ThreeYearROI           float64 `json:"three_year_roi_percent"`
	NPV                    float64 `json:"npv_usd"`
	IRR                    float64 `json:"irr_percent"`
}

// CostSavings represents cost savings from tiering optimization
type CostSavings struct {
	HotTierOffloadSavings  float64 `json:"hot_tier_offload_savings_usd"`
	ColdTierArchiveSavings float64 `json:"cold_tier_archive_savings_usd"`
	DeduplicationSavings   float64 `json:"deduplication_savings_usd"`
	CompressionSavings     float64 `json:"compression_savings_usd"`
	TotalMonthlySavings    float64 `json:"total_monthly_savings_usd"`
	TotalAnnualSavings     float64 `json:"total_annual_savings_usd"`
	SavingsPercent         float64 `json:"savings_percent"`
}

// CostOpportunity represents a cost optimization opportunity
type CostOpportunity struct {
	Type           string  `json:"type"` // "migration", "archive", "dedupe", "compression"
	Tier           Tier    `json:"tier"`
	CurrentCost    float64 `json:"current_cost_usd"`
	PotentialCost  float64 `json:"potential_cost_usd"`
	Savings        float64 `json:"savings_usd"`
	Effort         string  `json:"effort"` // "low", "medium", "high"
	Impact         string  `json:"impact"` // "low", "medium", "high"
	Priority       int     `json:"priority"`
	FilesAffected  int64   `json:"files_affected"`
	SizeAffectedGB float64 `json:"size_affected_gb"`
}

// CostRecommendation represents a cost-related recommendation
type CostRecommendation struct {
	Priority      int     `json:"priority"`
	Category      string  `json:"category"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	CurrentCost   float64 `json:"current_cost_usd"`
	ProjectedCost float64 `json:"projected_cost_usd"`
	Savings       float64 `json:"savings_usd"`
	ROI           float64 `json:"roi_percent"`
	Timeline      string  `json:"timeline"`
}

// CostConfig configures cost analysis parameters
type CostConfig struct {
	// Hot tier storage cost (SSD/NVMe) in USD per GB per month
	HotTierCostPerGB float64 `json:"hot_tier_cost_per_gb"`
	// Warm tier storage cost (HDD) in USD per GB per month
	WarmTierCostPerGB float64 `json:"warm_tier_cost_per_gb"`
	// Cold tier storage cost (Archive/Cloud) in USD per GB per month
	ColdTierCostPerGB float64 `json:"cold_tier_cost_per_gb"`
	// Value of latency reduction per ms saved per month
	LatencyValuePerMs float64 `json:"latency_value_per_ms"`
	// Value of throughput improvement per MB/s per month
	ThroughputValuePerMBs float64 `json:"throughput_value_per_mbs"`
	// Productivity value per user per month
	ProductivityValuePerUser float64 `json:"productivity_value_per_user"`
	// Number of users benefiting from tiering
	ActiveUsers int `json:"active_users"`
	// Discount rate for NPV calculation
	DiscountRate float64 `json:"discount_rate"`
}

// DefaultCostConfig returns default cost configuration
// Based on typical 2024-2025 cloud storage pricing
func DefaultCostConfig() CostConfig {
	return CostConfig{
		HotTierCostPerGB:         0.15,  // SSD/NVMe tier
		WarmTierCostPerGB:        0.05,  // HDD tier
		ColdTierCostPerGB:        0.004, // Archive tier (S3 Glacier-like)
		LatencyValuePerMs:        10.0,  // Business value per ms latency reduction
		ThroughputValuePerMBs:    5.0,   // Business value per MB/s improvement
		ProductivityValuePerUser: 50.0,  // Monthly productivity value per user
		ActiveUsers:              10,    // Default users
		DiscountRate:             0.08,  // 8% annual discount rate
	}
}

// CostAnalyzer analyzes cost-benefit for storage tiering
type CostAnalyzer struct {
	mu     sync.RWMutex
	config CostConfig
}

// NewCostAnalyzer creates a new cost analyzer
func NewCostAnalyzer(config CostConfig) *CostAnalyzer {
	return &CostAnalyzer{
		config: config,
	}
}

// GenerateCostReport generates a comprehensive cost-benefit report
func (ca *CostAnalyzer) GenerateCostReport(ctx context.Context, manager *Manager, migrator *Migrator, period TimeRange) (*CostBenefitReport, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	report := &CostBenefitReport{
		GeneratedAt: time.Now(),
		Period:      period,
	}

	stats := manager.GetStats()

	// Calculate storage costs
	report.StorageCosts = ca.calculateStorageCosts(stats, migrator)

	// Calculate performance value
	report.PerformanceValue = ca.calculatePerformanceValue()

	// Calculate ROI
	report.ROI = ca.calculateROI(report.StorageCosts, report.PerformanceValue)

	// Calculate cost savings
	report.CostSavings = ca.calculateCostSavings(stats, migrator)

	// Find optimization opportunities
	report.OptimizationOpportunities = ca.findOptimizationOpportunities(stats, migrator)

	// Generate recommendations
	report.Recommendations = ca.generateCostRecommendations(report)

	return report, nil
}

// calculateStorageCosts calculates storage cost breakdown
func (ca *CostAnalyzer) calculateStorageCosts(stats Stats, migrator *Migrator) StorageCostBreakdown {
	costs := StorageCostBreakdown{}

	// Hot tier
	hotCap, _ := migrator.TierCapacity(TierHot)
	costs.HotTierCost = ca.calculateTierCost(TierHot, hotCap, ca.config.HotTierCostPerGB)

	// Warm tier
	warmCap, _ := migrator.TierCapacity(TierWarm)
	costs.WarmTierCost = ca.calculateTierCost(TierWarm, warmCap, ca.config.WarmTierCostPerGB)

	// Cold tier
	coldCap, _ := migrator.TierCapacity(TierCold)
	costs.ColdTierCost = ca.calculateTierCost(TierCold, coldCap, ca.config.ColdTierCostPerGB)

	costs.TotalMonthly = costs.HotTierCost.MonthlyCost + costs.WarmTierCost.MonthlyCost + costs.ColdTierCost.MonthlyCost
	costs.TotalAnnual = costs.TotalMonthly * 12

	totalGB := costs.HotTierCost.UsedGB + costs.WarmTierCost.UsedGB + costs.ColdTierCost.UsedGB
	if totalGB > 0 {
		costs.CostPerGB = costs.TotalMonthly / totalGB
	}

	return costs
}

// calculateTierCost calculates cost for a single tier
func (ca *CostAnalyzer) calculateTierCost(tier Tier, capInfo *TierLocation, costPerGB float64) TierCostDetail {
	detail := TierCostDetail{
		Tier:             tier,
		CostPerGBMonthly: costPerGB,
	}

	if capInfo != nil {
		detail.CapacityGB = float64(capInfo.Capacity) / 1024 / 1024 / 1024
		detail.UsedGB = float64(capInfo.Used) / 1024 / 1024 / 1024
	}

	detail.MonthlyCost = detail.UsedGB * costPerGB
	detail.AnnualCost = detail.MonthlyCost * 12

	if detail.CapacityGB > 0 {
		detail.Efficiency = (detail.UsedGB / detail.CapacityGB) * 100
	}

	// Cost efficiency score (higher is better)
	// Consider both utilization and cost tier appropriateness
	detail.CostEfficiency = ca.calculateCostEfficiencyScore(tier, detail.Efficiency, detail.MonthlyCost)

	return detail
}

// calculateCostEfficiencyScore calculates cost efficiency score for a tier
func (ca *CostAnalyzer) calculateCostEfficiencyScore(tier Tier, utilization float64, monthlyCost float64) float64 {
	score := 50.0 // Base score

	// Reward optimal utilization (50-80%)
	if utilization >= 50 && utilization <= 80 {
		score += 30
	} else if utilization < 50 {
		score -= (50 - utilization) * 0.5
	} else if utilization > 80 {
		score -= (utilization - 80) * 0.3
	}

	// Bonus for cold tier usage (cost savings)
	if tier == TierCold && utilization > 30 {
		score += 20
	}

	return math.Max(0, math.Min(100, score))
}

// calculatePerformanceValue calculates the value derived from performance improvements
func (ca *CostAnalyzer) calculatePerformanceValue() PerformanceValue {
	// Based on typical tiering performance gains
	// Synology DSM 7.3 reports 30%+ performance improvement
	pv := PerformanceValue{
		ReducedLatencyValue:   ca.config.LatencyValuePerMs * 50 * float64(ca.config.ActiveUsers),      // 50ms avg reduction
		ThroughputValue:       ca.config.ThroughputValuePerMBs * 100 * float64(ca.config.ActiveUsers), // 100 MB/s improvement
		IOPSValue:             ca.config.ThroughputValuePerMBs * 50 * float64(ca.config.ActiveUsers),
		UserProductivityValue: ca.config.ProductivityValuePerUser * float64(ca.config.ActiveUsers),
	}

	pv.TotalPerformanceValue = pv.ReducedLatencyValue + pv.ThroughputValue + pv.IOPSValue + pv.UserProductivityValue

	return pv
}

// calculateROI calculates return on investment metrics
func (ca *CostAnalyzer) calculateROI(costs StorageCostBreakdown, pv PerformanceValue) ROIAnalysis {
	roi := ROIAnalysis{
		InitialInvestment:      1000,                      // Estimated tiering setup cost
		MonthlyOperationalCost: costs.TotalMonthly * 0.1,  // 10% overhead for tiering
		MonthlySavings:         costs.TotalMonthly * 0.25, // 25% cost reduction from tiering
		MonthlyValue:           pv.TotalPerformanceValue,
	}

	roi.MonthlySavings += pv.TotalPerformanceValue

	// Payback period
	netMonthlyBenefit := roi.MonthlySavings - roi.MonthlyOperationalCost
	if netMonthlyBenefit > 0 {
		roi.PaybackPeriodMonths = roi.InitialInvestment / netMonthlyBenefit
	}

	// Annual ROI
	annualBenefit := netMonthlyBenefit * 12
	roi.AnnualROI = (annualBenefit / roi.InitialInvestment) * 100

	// 3-year ROI
	threeYearBenefit := annualBenefit * 3
	roi.ThreeYearROI = ((threeYearBenefit - roi.InitialInvestment) / roi.InitialInvestment) * 100

	// NPV calculation (simplified)
	r := ca.config.DiscountRate / 12 // Monthly discount rate
	npv := -roi.InitialInvestment
	for month := 1; month <= 36; month++ {
		npv += netMonthlyBenefit / math.Pow(1+r, float64(month))
	}
	roi.NPV = npv

	// IRR estimation (simplified)
	roi.IRR = roi.AnnualROI * 0.8 // Approximation

	return roi
}

// calculateCostSavings calculates potential and realized cost savings
func (ca *CostAnalyzer) calculateCostSavings(stats Stats, migrator *Migrator) CostSavings {
	savings := CostSavings{}

	// Hot tier offload savings
	// Files that should be moved from hot to warm
	hotToWarmGB := float64(stats.SizeByTier[TierHot]) / 1024 / 1024 / 1024 * 0.2 // 20% could be offloaded
	savings.HotTierOffloadSavings = hotToWarmGB * (ca.config.HotTierCostPerGB - ca.config.WarmTierCostPerGB) * 12

	// Cold tier archive savings
	// Files that should be moved from warm to cold
	warmToColdGB := float64(stats.SizeByTier[TierWarm]) / 1024 / 1024 / 1024 * 0.3 // 30% could be archived
	savings.ColdTierArchiveSavings = warmToColdGB * (ca.config.WarmTierCostPerGB - ca.config.ColdTierCostPerGB) * 12

	// Deduplication potential (estimated 15-30% savings)
	savings.DeduplicationSavings = savings.HotTierOffloadSavings * 0.2

	// Compression potential (estimated 30-50% for text/logs)
	savings.CompressionSavings = savings.ColdTierArchiveSavings * 0.35

	savings.TotalAnnualSavings = savings.HotTierOffloadSavings + savings.ColdTierArchiveSavings + savings.DeduplicationSavings + savings.CompressionSavings
	savings.TotalMonthlySavings = savings.TotalAnnualSavings / 12

	// Calculate savings percentage
	totalCost := (float64(stats.TotalSize) / 1024 / 1024 / 1024) * ca.config.WarmTierCostPerGB * 12
	if totalCost > 0 {
		savings.SavingsPercent = (savings.TotalAnnualSavings / totalCost) * 100
	}

	return savings
}

// findOptimizationOpportunities identifies cost optimization opportunities
func (ca *CostAnalyzer) findOptimizationOpportunities(stats Stats, migrator *Migrator) []CostOpportunity {
	opportunities := make([]CostOpportunity, 0)

	// Hot tier optimization
	hotCap, _ := migrator.TierCapacity(TierHot)
	if hotCap != nil {
		hotUsedGB := float64(hotCap.Used) / 1024 / 1024 / 1024
		hotUtil := float64(hotCap.Used) / float64(hotCap.Capacity) * 100

		if hotUtil > 80 {
			opportunities = append(opportunities, CostOpportunity{
				Type:           "migration",
				Tier:           TierHot,
				CurrentCost:    hotUsedGB * ca.config.HotTierCostPerGB * 12,
				PotentialCost:  hotUsedGB*0.7*ca.config.HotTierCostPerGB*12 + hotUsedGB*0.3*ca.config.WarmTierCostPerGB*12,
				Savings:        hotUsedGB * 0.3 * (ca.config.HotTierCostPerGB - ca.config.WarmTierCostPerGB) * 12,
				Effort:         "low",
				Impact:         "high",
				Priority:       1,
				FilesAffected:  stats.FilesByTier[TierHot] / 5,
				SizeAffectedGB: hotUsedGB * 0.3,
			})
		}
	}

	// Cold tier opportunity (archive potential)
	coldCap, _ := migrator.TierCapacity(TierCold)
	if coldCap != nil {
		_ = float64(coldCap.Used) / 1024 / 1024 / 1024 // coldUsedGB - 用于未来归档分析
		warmUsedGB := float64(stats.SizeByTier[TierWarm]) / 1024 / 1024 / 1024

		opportunities = append(opportunities, CostOpportunity{
			Type:           "archive",
			Tier:           TierWarm,
			CurrentCost:    warmUsedGB * ca.config.WarmTierCostPerGB * 12,
			PotentialCost:  warmUsedGB*0.7*ca.config.WarmTierCostPerGB*12 + warmUsedGB*0.3*ca.config.ColdTierCostPerGB*12,
			Savings:        warmUsedGB * 0.3 * (ca.config.WarmTierCostPerGB - ca.config.ColdTierCostPerGB) * 12,
			Effort:         "medium",
			Impact:         "medium",
			Priority:       2,
			FilesAffected:  stats.FilesByTier[TierWarm] / 3,
			SizeAffectedGB: warmUsedGB * 0.3,
		})
	}

	// Deduplication opportunity
	if stats.TotalSize > 0 {
		totalGB := float64(stats.TotalSize) / 1024 / 1024 / 1024
		dedupePotential := totalGB * 0.2 // 20% deduplication potential

		opportunities = append(opportunities, CostOpportunity{
			Type:           "dedupe",
			Tier:           TierWarm,
			CurrentCost:    totalGB * ca.config.WarmTierCostPerGB * 12,
			PotentialCost:  (totalGB - dedupePotential) * ca.config.WarmTierCostPerGB * 12,
			Savings:        dedupePotential * ca.config.WarmTierCostPerGB * 12,
			Effort:         "medium",
			Impact:         "medium",
			Priority:       3,
			FilesAffected:  stats.TotalFiles / 5,
			SizeAffectedGB: dedupePotential,
		})
	}

	return opportunities
}

// generateCostRecommendations generates cost-related recommendations
func (ca *CostAnalyzer) generateCostRecommendations(report *CostBenefitReport) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	// Hot tier right-sizing
	if report.StorageCosts.HotTierCost.Efficiency < 50 {
		recs = append(recs, CostRecommendation{
			Priority:      1,
			Category:      "capacity",
			Title:         "Right-size Hot Tier",
			Description:   "Hot tier is under-utilized. Consider reducing capacity or promoting more data.",
			CurrentCost:   report.StorageCosts.HotTierCost.AnnualCost,
			ProjectedCost: report.StorageCosts.HotTierCost.AnnualCost * 0.7,
			Savings:       report.StorageCosts.HotTierCost.AnnualCost * 0.3,
			ROI:           45.0,
			Timeline:      "1-2 weeks",
		})
	}

	// Archive recommendations
	if report.CostSavings.ColdTierArchiveSavings > 100 {
		recs = append(recs, CostRecommendation{
			Priority:      2,
			Category:      "archive",
			Title:         "Increase Cold Tier Usage",
			Description:   "Moving inactive data to cold storage can yield significant savings.",
			CurrentCost:   report.StorageCosts.WarmTierCost.AnnualCost,
			ProjectedCost: report.StorageCosts.WarmTierCost.AnnualCost * 0.7,
			Savings:       report.CostSavings.ColdTierArchiveSavings,
			ROI:           report.CostSavings.ColdTierArchiveSavings / (report.StorageCosts.WarmTierCost.AnnualCost * 0.01) * 100,
			Timeline:      "2-4 weeks",
		})
	}

	// Deduplication
	if report.CostSavings.DeduplicationSavings > 50 {
		recs = append(recs, CostRecommendation{
			Priority:      3,
			Category:      "optimization",
			Title:         "Enable Deduplication",
			Description:   "Deduplication can reduce storage footprint by 15-30%.",
			CurrentCost:   report.StorageCosts.TotalAnnual,
			ProjectedCost: report.StorageCosts.TotalAnnual * 0.85,
			Savings:       report.CostSavings.DeduplicationSavings,
			ROI:           25.0,
			Timeline:      "1-2 months",
		})
	}

	return recs
}

// UpdateConfig updates the cost configuration
func (ca *CostAnalyzer) UpdateConfig(config CostConfig) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.config = config
}

// GetConfig returns current cost configuration
func (ca *CostAnalyzer) GetConfig() CostConfig {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.config
}

// EstimateMigrationCost estimates the cost of migrating data between tiers
func (ca *CostAnalyzer) EstimateMigrationCost(sourceTier, targetTier Tier, sizeGB float64) MigrationCostEstimate {
	estimate := MigrationCostEstimate{
		SourceTier:   sourceTier,
		TargetTier:   targetTier,
		SizeGB:       sizeGB,
		TransferCost: sizeGB * 0.01,                           // $0.01/GB transfer cost
		TransferTime: time.Duration(sizeGB/100) * time.Minute, // ~100GB/min
	}

	// Calculate monthly cost change
	sourceCostPerGB := ca.getTierCostPerGB(sourceTier)
	targetCostPerGB := ca.getTierCostPerGB(targetTier)

	estimate.CurrentMonthlyCost = sizeGB * sourceCostPerGB
	estimate.NewMonthlyCost = sizeGB * targetCostPerGB
	estimate.MonthlySavings = estimate.CurrentMonthlyCost - estimate.NewMonthlyCost
	estimate.AnnualSavings = estimate.MonthlySavings * 12

	// Payback period
	if estimate.TransferCost > 0 && estimate.MonthlySavings > 0 {
		estimate.PaybackDays = int(estimate.TransferCost / estimate.MonthlySavings * 30)
	}

	return estimate
}

// MigrationCostEstimate represents a cost estimate for migration
type MigrationCostEstimate struct {
	SourceTier         Tier          `json:"source_tier"`
	TargetTier         Tier          `json:"target_tier"`
	SizeGB             float64       `json:"size_gb"`
	TransferCost       float64       `json:"transfer_cost_usd"`
	TransferTime       time.Duration `json:"transfer_time"`
	CurrentMonthlyCost float64       `json:"current_monthly_cost_usd"`
	NewMonthlyCost     float64       `json:"new_monthly_cost_usd"`
	MonthlySavings     float64       `json:"monthly_savings_usd"`
	AnnualSavings      float64       `json:"annual_savings_usd"`
	PaybackDays        int           `json:"payback_days"`
}

func (ca *CostAnalyzer) getTierCostPerGB(tier Tier) float64 {
	switch tier {
	case TierHot:
		return ca.config.HotTierCostPerGB
	case TierWarm:
		return ca.config.WarmTierCostPerGB
	case TierCold:
		return ca.config.ColdTierCostPerGB
	default:
		return ca.config.WarmTierCostPerGB
	}
}
