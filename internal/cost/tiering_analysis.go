// Package cost provides cost analysis and tiering optimization
package cost

import (
	"context"
	"time"
)

// TieringCostAnalysis provides tiering cost-benefit analysis.
type TieringCostAnalysis struct {
	HotStorageCost  float64 // cost per GB per month
	WarmStorageCost float64
	ColdStorageCost float64
	HotAccessCost   float64 // access cost per operation
	WarmAccessCost  float64
	ColdAccessCost  float64
}

// StorageTier represents a storage tier with its costs.
type StorageTier struct {
	Name          string
	CostPerGB     float64
	AccessCost    float64
	AccessFreq    float64 // average access frequency per day
	DataVolume    float64 // GB stored
	MigrationCost float64 // cost to migrate data
}

// CostBenefitReport represents tiering analysis result.
type CostBenefitReport struct {
	CurrentCost     float64
	OptimizedCost   float64
	Savings         float64
	SavingsPercent  float64
	Recommendations []TieringRecommendation
	GeneratedAt     time.Time
}

// TieringRecommendation represents a tiering recommendation.
type TieringRecommendation struct {
	Path          string
	CurrentTier   string
	SuggestedTier string
	Reason        string
	Savings       float64
}

// TieringAnalyzer analyzes storage tiering options.
type TieringAnalyzer struct {
	tiers    []StorageTier
	analysis TieringCostAnalysis
}

// NewTieringAnalyzer creates a new tiering analyzer.
func NewTieringAnalyzer(analysis TieringCostAnalysis) *TieringAnalyzer {
	return &TieringAnalyzer{
		analysis: analysis,
		tiers: []StorageTier{
			{Name: "hot", CostPerGB: analysis.HotStorageCost, AccessCost: analysis.HotAccessCost},
			{Name: "warm", CostPerGB: analysis.WarmStorageCost, AccessCost: analysis.WarmAccessCost},
			{Name: "cold", CostPerGB: analysis.ColdStorageCost, AccessCost: analysis.ColdAccessCost},
		},
	}
}

// AnalyzeDataTiering analyzes optimal tiering for data.
func (a *TieringAnalyzer) AnalyzeDataTiering(ctx context.Context, accessPatterns []AccessPattern) CostBenefitReport {
	currentCost := 0.0
	optimizedCost := 0.0
	recommendations := make([]TieringRecommendation, 0)

	for _, pattern := range accessPatterns {
		// Calculate current cost (assume all hot for now)
		currentTier := "hot"
		currentDataCost := pattern.DataVolumeGB * a.analysis.HotStorageCost
		currentAccessCost := pattern.AccessCount * a.analysis.HotAccessCost
		currentCost += currentDataCost + currentAccessCost

		// Determine optimal tier based on access frequency
		suggestedTier := a.determineOptimalTier(pattern)
		tierCost := a.calculateTierCost(pattern, suggestedTier)
		optimizedCost += tierCost

		if suggestedTier != currentTier {
			savings := currentDataCost + currentAccessCost - tierCost
			recommendations = append(recommendations, TieringRecommendation{
				Path:          pattern.Path,
				CurrentTier:   currentTier,
				SuggestedTier: suggestedTier,
				Reason:        a.getReason(pattern, suggestedTier),
				Savings:       savings,
			})
		}
	}

	savingsPercent := 0.0
	if currentCost > 0 {
		savingsPercent = ((currentCost - optimizedCost) / currentCost) * 100
	}

	return CostBenefitReport{
		CurrentCost:     currentCost,
		OptimizedCost:   optimizedCost,
		Savings:         currentCost - optimizedCost,
		SavingsPercent:  savingsPercent,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}
}

// AccessPattern represents data access pattern.
type AccessPattern struct {
	Path         string
	DataVolumeGB float64
	AccessCount  float64
	AccessFreq   float64 // accesses per day
	LastAccess   time.Time
	ContentType  string
}

// determineOptimalTier determines the best tier for data.
func (a *TieringAnalyzer) determineOptimalTier(pattern AccessPattern) string {
	// High frequency: hot tier (>10 accesses/day)
	if pattern.AccessFreq >= 10 {
		return "hot"
	}
	// Medium frequency: warm tier (1-10 accesses/day)
	if pattern.AccessFreq >= 1 {
		return "warm"
	}
	// Low frequency: cold tier (<1 access/day)
	return "cold"
}

// calculateTierCost calculates cost for a specific tier.
func (a *TieringAnalyzer) calculateTierCost(pattern AccessPattern, tier string) float64 {
	var storageCost, accessCost float64
	switch tier {
	case "hot":
		storageCost = pattern.DataVolumeGB * a.analysis.HotStorageCost
		accessCost = pattern.AccessCount * a.analysis.HotAccessCost
	case "warm":
		storageCost = pattern.DataVolumeGB * a.analysis.WarmStorageCost
		accessCost = pattern.AccessCount * a.analysis.WarmAccessCost
	case "cold":
		storageCost = pattern.DataVolumeGB * a.analysis.ColdStorageCost
		accessCost = pattern.AccessCount * a.analysis.ColdAccessCost
	}
	return storageCost + accessCost
}

// getReason explains why tiering is recommended.
func (a *TieringAnalyzer) getReason(pattern AccessPattern, tier string) string {
	switch tier {
	case "cold":
		return "Low access frequency indicates archival candidate"
	case "warm":
		return "Moderate access frequency suitable for warm tier"
	case "hot":
		return "High access frequency requires hot tier performance"
	}
	return "Optimal tiering based on access patterns"
}

// CompareCloudStorage compares local vs cloud storage costs.
func (a *TieringAnalyzer) CompareCloudStorage(ctx context.Context, dataVolumeGB float64, localCostPerGB float64) CloudComparisonReport {
	// Typical cloud storage costs (approximate)
	cloudHotCost := 0.023   // AWS S3 Standard per GB/month
	cloudWarmCost := 0.0125 // AWS S3 IA per GB/month
	cloudColdCost := 0.004  // AWS S3 Glacier per GB/month

	localMonthlyCost := dataVolumeGB * localCostPerGB
	cloudHotMonthlyCost := dataVolumeGB * cloudHotCost
	cloudWarmMonthlyCost := dataVolumeGB * cloudWarmCost
	cloudColdMonthlyCost := dataVolumeGB * cloudColdCost

	return CloudComparisonReport{
		LocalCost:      localMonthlyCost,
		CloudHotCost:   cloudHotMonthlyCost,
		CloudWarmCost:  cloudWarmMonthlyCost,
		CloudColdCost:  cloudColdMonthlyCost,
		BestOption:     "local",
		SavingsIfCloud: localMonthlyCost - cloudColdMonthlyCost,
		Recommendation: "Local storage recommended for high-frequency access data",
	}
}

// CloudComparisonReport represents cloud storage comparison.
type CloudComparisonReport struct {
	LocalCost      float64
	CloudHotCost   float64
	CloudWarmCost  float64
	CloudColdCost  float64
	BestOption     string
	SavingsIfCloud float64
	Recommendation string
}
