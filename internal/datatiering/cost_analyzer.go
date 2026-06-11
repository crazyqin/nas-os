package datatiering

import (
	"fmt"
	"sync"
	"time"
)

// CostStorageTier represents a storage tier with cost and performance characteristics.
type CostStorageTier struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"` // nvme, ssd, hdd, tape, cloud
	CostPerGBMo  float64 `json:"cost_per_gb_month"` // cost in USD per GB per month
	ReadMBps     float64 `json:"read_mbps"`
	WriteMBps    float64 `json:"write_mbps"`
	IOPS         int     `json:"iops"`
	LatencyMs    float64 `json:"latency_ms"`
	Reliability  float64 `json:"reliability"` // 0-1, annual durability
	TotalGB      int64   `json:"total_gb"`
	UsedGB       int64   `json:"used_gb"`
}

// CostTieringRule defines when data should move between tiers.
type CostTieringRule struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	SourceTierID  string  `json:"source_tier_id"`
	TargetTierID  string  `json:"target_tier_id"`
	DaysInactive  int     `json:"days_inactive"` // move after N days of inactivity
	AccessFreqMin float64 `json:"access_freq_min"` // min access frequency to stay
	Enabled       bool    `json:"enabled"`
}

// CostAnalysisReport contains storage cost analysis.
type CostAnalysisReport struct {
	Timestamp      time.Time              `json:"timestamp"`
	TotalCostMo    float64                `json:"total_cost_monthly"`
	TierCosts      map[string]float64     `json:"tier_costs"` // tierID -> monthly cost
	TotalUsedGB    int64                  `json:"total_used_gb"`
	TotalAllocGB   int64                  `json:"total_allocated_gb"`
	AvgCostPerGB   float64                `json:"avg_cost_per_gb"`
	PotentialSaving float64               `json:"potential_saving_monthly"`
	SavingPct      float64                `json:"saving_percentage"`
	ROI            float64                `json:"roi"`
	Recommendations []CostOptRecommendation  `json:"recommendations"`
}

// CostOptRecommendation suggests a cost optimization action.
type CostOptRecommendation struct {
	Action      string  `json:"action"`
	Description string  `json:"description"`
	SavingMo    float64 `json:"saving_monthly"`
	Priority    string  `json:"priority"` // high, medium, low
	Effort      string  `json:"effort"`   // easy, moderate, complex
}

// StorageCostAnalyzer analyzes storage costs and provides optimization recommendations.
type StorageCostAnalyzer struct {
	mu      sync.RWMutex
	tiers   map[string]*CostStorageTier
	rules   []*CostTieringRule
	reports []*CostAnalysisReport
}

// NewStorageCostAnalyzer creates a new cost analyzer.
func NewStorageCostAnalyzer() *StorageCostAnalyzer {
	return &StorageCostAnalyzer{
		tiers:   make(map[string]*CostStorageTier),
		rules:   make([]*CostTieringRule, 0),
		reports: make([]*CostAnalysisReport, 0),
	}
}

// AddTier adds a storage tier.
func (ca *StorageCostAnalyzer) AddTier(tier CostStorageTier) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if tier.ID == "" {
		return fmt.Errorf("tier ID cannot be empty")
	}
	ca.tiers[tier.ID] = &tier
	return nil
}

// UpdateTierUsage updates the usage for a tier.
func (ca *StorageCostAnalyzer) UpdateTierUsage(tierID string, usedGB int64) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	tier, exists := ca.tiers[tierID]
	if !exists {
		return fmt.Errorf("tier not found: %s", tierID)
	}
	tier.UsedGB = usedGB
	return nil
}

// AddRule adds a tiering rule.
func (ca *StorageCostAnalyzer) AddRule(rule CostTieringRule) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.rules = append(ca.rules, &rule)
}

// GenerateReport generates a cost analysis report.
func (ca *StorageCostAnalyzer) GenerateReport() *CostAnalysisReport {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	report := &CostAnalysisReport{
		Timestamp: time.Now(),
		TierCosts: make(map[string]float64),
	}

	for _, tier := range ca.tiers {
		cost := float64(tier.UsedGB) * tier.CostPerGBMo
		report.TierCosts[tier.ID] = cost
		report.TotalCostMo += cost
		report.TotalUsedGB += tier.UsedGB
		report.TotalAllocGB += tier.TotalGB
	}

	if report.TotalUsedGB > 0 {
		report.AvgCostPerGB = report.TotalCostMo / float64(report.TotalUsedGB)
	}

	// Generate recommendations
	report.Recommendations = ca.generateRecommendations()
	for _, rec := range report.Recommendations {
		report.PotentialSaving += rec.SavingMo
	}
	if report.TotalCostMo > 0 {
		report.SavingPct = (report.PotentialSaving / report.TotalCostMo) * 100
	}
	if report.PotentialSaving > 0 {
		report.ROI = report.PotentialSaving * 12 / report.TotalCostMo * 100
	}

	ca.reports = append(ca.reports, report)
	return report
}

// generateRecommendations generates cost optimization recommendations.
func (ca *StorageCostAnalyzer) generateRecommendations() []CostOptRecommendation {
	var recs []CostOptRecommendation

	// Check for underutilized tiers
	for _, tier := range ca.tiers {
		if tier.TotalGB == 0 {
			continue
		}
		utilization := float64(tier.UsedGB) / float64(tier.TotalGB) * 100
		if utilization < 30 && tier.CostPerGBMo > 0.1 {
			monthlySaving := float64(tier.TotalGB-tier.UsedGB) * tier.CostPerGBMo * 0.5
			recs = append(recs, CostOptRecommendation{
				Action:      "downsize",
				Description: fmt.Sprintf("存储层 %s 利用率仅 %.0f%%，建议缩减容量", tier.Name, utilization),
				SavingMo:    monthlySaving,
				Priority:    "high",
				Effort:      "easy",
			})
		}
	}

	// Check for cold data on expensive tiers
	for _, tier := range ca.tiers {
		if tier.CostPerGBMo > 0.05 && tier.UsedGB > 100 {
			// Estimate 30% of data could be cold
			coldGB := int64(float64(tier.UsedGB) * 0.3)
			cheapestHDD := ca.findCheapestTier("hdd")
			if cheapestHDD != nil && cheapestHDD.CostPerGBMo < tier.CostPerGBMo {
				saving := float64(coldGB) * (tier.CostPerGBMo - cheapestHDD.CostPerGBMo)
				recs = append(recs, CostOptRecommendation{
					Action:      "tier_down",
					Description: fmt.Sprintf("将 %s 中 %dGB 冷数据迁移到 HDD 层可降低成本", tier.Name, coldGB),
					SavingMo:    saving,
					Priority:    "medium",
					Effort:      "moderate",
				})
			}
		}
	}

	// Check if more flash would improve performance
	flashTier := ca.findCheapestTier("nvme")
	if flashTier == nil {
		flashTier = ca.findCheapestTier("ssd")
	}
	if flashTier != nil && flashTier.UsedGB > int64(float64(flashTier.TotalGB)*0.8) {
		recs = append(recs, CostOptRecommendation{
			Action:      "expand_flash",
			Description: "闪存层使用率超过80%，建议扩容以保持性能",
			Priority:    "medium",
			Effort:      "moderate",
		})
	}

	return recs
}

func (ca *StorageCostAnalyzer) findCheapestTier(tierType string) *CostStorageTier {
	var cheapest *CostStorageTier
	for _, tier := range ca.tiers {
		if tier.Type == tierType {
			if cheapest == nil || tier.CostPerGBMo < cheapest.CostPerGBMo {
				cheapest = tier
			}
		}
	}
	return cheapest
}

// GetReportHistory returns recent reports.
func (ca *StorageCostAnalyzer) GetReportHistory(limit int) []*CostAnalysisReport {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if limit <= 0 || limit > len(ca.reports) {
		limit = len(ca.reports)
	}
	start := len(ca.reports) - limit
	result := make([]*CostAnalysisReport, limit)
	copy(result, ca.reports[start:])
	return result
}

// GetTiers returns all storage tiers.
func (ca *StorageCostAnalyzer) GetTiers() []CostStorageTier {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	result := make([]CostStorageTier, 0, len(ca.tiers))
	for _, t := range ca.tiers {
		result = append(result, *t)
	}
	return result
}
