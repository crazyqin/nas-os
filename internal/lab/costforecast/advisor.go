// Package costforecast projects storage cost trends inspired by Synology
// Storage Analytics cost dashboards and TrueNAS capacity planning.
package costforecast

import (
	"math"
	"sort"
	"strings"
	"time"
)

// TrendPoint is a historical cost/usage data point.
type TrendPoint struct {
	Date      time.Time `json:"date"`
	UsedGB    float64   `json:"used_gb"`
	CostPerGB float64   `json:"cost_per_gb"`
	TotalCost float64   `json:"total_cost"`
}

// Signal describes the current cost and capacity state for forecasting.
type Signal struct {
	PoolName          string
	TotalCapacityGB   float64
	UsedGB            float64
	GrowthRateGBPerMo float64
	CostPerGBPerMo    float64
	HistoricalTrend   []TrendPoint
	BudgetMonthly     float64
	HasTiering        bool
	ColdTierGB        float64
	ColdTierCostPerGB float64
	CloudBuckets      int
	CloudCostPerMo    float64
	DedupSavingsGB    float64
	CompressSavingsGB float64
}

// Forecast is a projected cost outlook.
type Forecast struct {
	MonthsToFull      int       `json:"months_to_full"`
	ProjectedCostMo   float64   `json:"projected_cost_mo"`
	BudgetBreachMonth int       `json:"budget_breach_month,omitempty"`
	ProjectedFullDate time.Time `json:"projected_full_date,omitempty"`
	Recommendation    string    `json:"recommendation"`
}

// Recommendation is an actionable cost optimization suggestion.
type Recommendation struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Priority string `json:"priority"`
	Action  string  `json:"action"`
	Reason  string  `json:"reason"`
	Savings float64 `json:"estimated_savings_mo,omitempty"`
}

// Analyze evaluates cost trends and produces recommendations + forecast.
func Analyze(s Signal) ([]Recommendation, Forecast) {
	var recs []Recommendation
	fc := Forecast{}

	if s.GrowthRateGBPerMo > 0 {
		freeGB := s.TotalCapacityGB - s.UsedGB
		fc.MonthsToFull = int(math.Ceil(freeGB / s.GrowthRateGBPerMo))
		if fc.MonthsToFull < 0 {
			fc.MonthsToFull = 0
		}
	}

	projectedUsed := s.UsedGB + s.GrowthRateGBPerMo*3
	fc.ProjectedCostMo = projectedUsed * s.CostPerGBPerMo

	if s.BudgetMonthly > 0 && fc.ProjectedCostMo > s.BudgetMonthly {
		overrun := fc.ProjectedCostMo - s.BudgetMonthly
		if s.GrowthRateGBPerMo*s.CostPerGBPerMo > 0 {
			fc.BudgetBreachMonth = int(math.Ceil(overrun / (s.GrowthRateGBPerMo * s.CostPerGBPerMo)))
		}
	}

	if fc.MonthsToFull > 0 && fc.MonthsToFull <= 3 {
		recs = append(recs, Recommendation{
			ID:       "cost-capacity-soon",
			Title:    "Storage will be full within 3 months",
			Priority: "high",
			Action:   "Expand pool or enable dedup/compression to reduce usage",
			Reason:   "At current growth rate, free space is under 3 months",
			Savings:  s.DedupSavingsGB * s.CostPerGBPerMo,
		})
	}

	if s.BudgetMonthly > 0 && fc.ProjectedCostMo > s.BudgetMonthly {
		recs = append(recs, Recommendation{
			ID:       "cost-budget-overrun",
			Title:    "Storage cost will exceed budget",
			Priority: "high",
			Action:   "Migrate cold data to low-cost tier or cloud",
			Reason:   "Projected monthly cost exceeds budget",
			Savings:  (s.UsedGB - s.ColdTierGB) * (s.CostPerGBPerMo - s.ColdTierCostPerGB),
		})
	}

	if !s.HasTiering && s.UsedGB > s.TotalCapacityGB*0.5 {
		recs = append(recs, Recommendation{
			ID:       "cost-enable-tiering",
			Title:    "Enable storage tiering",
			Priority: "medium",
			Action:   "Configure hot/cold tiering to move inactive data to cheaper tier",
			Reason:   "Storage over 50% utilized; tiering reduces cost and extends capacity",
			Savings:  s.UsedGB * 0.3 * (s.CostPerGBPerMo - s.ColdTierCostPerGB),
		})
	}

	if s.DedupSavingsGB == 0 && s.UsedGB > 1000 {
		recs = append(recs, Recommendation{
			ID:       "cost-dedup",
			Title:    "Evaluate deduplication benefit",
			Priority: "medium",
			Action:   "Run dedup analysis to estimate space savings",
			Reason:   "Large pools typically benefit 10-30% from dedup",
			Savings:  s.UsedGB * 0.15 * s.CostPerGBPerMo,
		})
	}

	if s.CompressSavingsGB == 0 && s.UsedGB > 500 {
		recs = append(recs, Recommendation{
			ID:       "cost-compress",
			Title:    "Enable compression",
			Priority: "low",
			Action:   "Enable compression for documents and archive data",
			Reason:   "Compression typically saves 30-50% space",
			Savings:  s.UsedGB * 0.35 * s.CostPerGBPerMo,
		})
	}

	if s.CloudBuckets > 0 && s.CloudCostPerMo > fc.ProjectedCostMo*0.3 {
		recs = append(recs, Recommendation{
			ID:       "cost-cloud-optimize",
			Title:    "Optimize cloud storage spend",
			Priority: "medium",
			Action:   "Review cloud bucket lifecycle policies, move cold data to archive tier",
			Reason:   "Cloud storage cost exceeds 30% of projected cost",
			Savings:  s.CloudCostPerMo * 0.4,
		})
	}

	if fc.MonthsToFull > 3 && fc.MonthsToFull <= 6 {
		fc.Recommendation = "Storage growth manageable; plan mid-term expansion"
	} else if fc.MonthsToFull > 6 {
		fc.Recommendation = "Storage growth healthy; maintain current strategy"
	} else if fc.MonthsToFull > 0 {
		fc.Recommendation = "Storage space critical; take immediate action"
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs, fc
}

func priorityRank(p string) int {
	switch strings.ToLower(p) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}
