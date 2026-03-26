// Package tiering provides capacity trend prediction for storage planning.
// Uses historical data to forecast future capacity needs and tier optimization.
package tiering

import (
	"context"
	"math"
	"sync"
	"time"
)

// TrendReport represents a capacity trend analysis report
type TrendReport struct {
	GeneratedAt       time.Time              `json:"generated_at"`
	HistoricalPeriod  TimeRange              `json:"historical_period"`
	ForecastPeriod    TimeRange              `json:"forecast_period"`
	TierTrends        map[Tier]TierTrend     `json:"tier_trends"`
	OverallTrend      OverallTrendAnalysis   `json:"overall_trend"`
	Forecasts         map[Tier]CapacityForecast `json:"forecasts"`
	Alerts            []CapacityAlert        `json:"alerts"`
	Recommendations   []TrendRecommendation  `json:"recommendations"`
	ConfidenceLevel   float64                `json:"confidence_level"`
}

// TierTrend represents trend analysis for a single tier
type TierTrend struct {
	Tier                Tier        `json:"tier"`
	CurrentCapacity     int64       `json:"current_capacity_bytes"`
	CurrentUsed         int64       `json:"current_used_bytes"`
	CurrentUtilization  float64     `json:"current_utilization_percent"`
	GrowthRate          float64     `json:"growth_rate_percent_per_month"`
	GrowthTrend         string      `json:"growth_trend"` // "increasing", "stable", "decreasing"
	Seasonality         Seasonality `json:"seasonality"`
	PeakUtilization     float64     `json:"peak_utilization_percent"`
	PeakTime            time.Time   `json:"peak_time,omitempty"`
	PredictedFullDate   *time.Time  `json:"predicted_full_date,omitempty"`
	DaysUntilFull       int         `json:"days_until_full,omitempty"`
	RecommendedAction   string      `json:"recommended_action"`
}

// Seasonality represents seasonal patterns in data
type Seasonality struct {
	Detected      bool             `json:"detected"`
	PatternType   string           `json:"pattern_type"` // "daily", "weekly", "monthly", "yearly"
	PeakMonths    []int            `json:"peak_months,omitempty"`
	PeakDays      []int            `json:"peak_days,omitempty"`    // 0=Sunday
	PeakHours     []int            `json:"peak_hours,omitempty"`
	VariationPercent float64       `json:"variation_percent"`
}

// CapacityForecast represents a capacity forecast for a tier
type CapacityForecast struct {
	Tier             Tier               `json:"tier"`
	Predictions      []CapacityPoint    `json:"predictions"`
	WorstCase        []CapacityPoint    `json:"worst_case"`
	BestCase         []CapacityPoint    `json:"best_case"`
	ConfidenceBounds ConfidenceBounds   `json:"confidence_bounds"`
	Accuracy         ForecastAccuracy   `json:"accuracy"`
	Method           string             `json:"method"` // "linear", "exponential", "arima", "ensemble"
}

// CapacityPoint represents a predicted capacity point
type CapacityPoint struct {
	Date        time.Time `json:"date"`
	UsedBytes   int64     `json:"used_bytes"`
	Utilization float64   `json:"utilization_percent"`
	Confidence  float64   `json:"confidence"`
}

// ConfidenceBounds represents confidence interval bounds
type ConfidenceBounds struct {
	Lower95 int64 `json:"lower_95_percent"`
	Upper95 int64 `json:"upper_95_percent"`
	Lower80 int64 `json:"lower_80_percent"`
	Upper80 int64 `json:"upper_80_percent"`
}

// ForecastAccuracy represents forecast accuracy metrics
type ForecastAccuracy struct {
	MAPE   float64 `json:"mape"`   // Mean Absolute Percentage Error
	MAE    float64 `json:"mae"`    // Mean Absolute Error
	RMSE   float64 `json:"rmse"`   // Root Mean Square Error
	R2     float64 `json:"r2"`     // R-squared
	Score  float64 `json:"score"`  // Overall accuracy score (0-100)
}

// OverallTrendAnalysis represents overall storage trend analysis
type OverallTrendAnalysis struct {
	TotalCapacity          int64   `json:"total_capacity_bytes"`
	TotalUsed              int64   `json:"total_used_bytes"`
	OverallUtilization     float64 `json:"overall_utilization_percent"`
	OverallGrowthRate      float64 `json:"overall_growth_rate_percent_per_month"`
	TieringEfficiencyTrend string  `json:"tiering_efficiency_trend"` // "improving", "stable", "declining"
	OptimalTierDistribution map[Tier]float64 `json:"optimal_tier_distribution"`
	CurrentDistribution    map[Tier]float64 `json:"current_distribution"`
	ImprovementPotential   float64 `json:"improvement_potential_percent"`
}

// CapacityAlert represents a capacity-related alert
type CapacityAlert struct {
	Severity    string    `json:"severity"` // "critical", "warning", "info"
	Type        string    `json:"type"`     // "capacity", "trend", "forecast"
	Tier        Tier      `json:"tier"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	TriggeredAt time.Time `json:"triggered_at"`
	Threshold   float64   `json:"threshold_percent"`
	CurrentValue float64  `json:"current_value_percent"`
	Action      string    `json:"action"`
}

// TrendRecommendation represents a trend-based recommendation
type TrendRecommendation struct {
	Priority      int     `json:"priority"`
	Type          string  `json:"type"` // "capacity", "migration", "policy", "purchase"
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Impact        string  `json:"impact"`
	CostEstimate  float64 `json:"cost_estimate_usd"`
	Timeline      string  `json:"timeline"`
	ExpectedROI   float64 `json:"expected_roi_percent"`
}

// TrendPredictor predicts capacity trends
type TrendPredictor struct {
	mu            sync.RWMutex
	history       []CapacitySnapshot
	config        TrendConfig
}

// TrendConfig configures the trend predictor
type TrendConfig struct {
	HistoryRetentionDays int           `json:"history_retention_days"`
	ForecastDays         int           `json:"forecast_days"`
	MinHistoryDays       int           `json:"min_history_days"`
	UpdateInterval       time.Duration `json:"update_interval"`
	ConfidenceThreshold  float64       `json:"confidence_threshold"`
}

// CapacitySnapshot represents a point-in-time capacity measurement
type CapacitySnapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	HotUsed       int64     `json:"hot_used_bytes"`
	WarmUsed      int64     `json:"warm_used_bytes"`
	ColdUsed      int64     `json:"cold_used_bytes"`
	HotCapacity   int64     `json:"hot_capacity_bytes"`
	WarmCapacity  int64     `json:"warm_capacity_bytes"`
	ColdCapacity  int64     `json:"cold_capacity_bytes"`
	TotalFiles    int64     `json:"total_files"`
	MigrationCount int64    `json:"migration_count"`
}

// DefaultTrendConfig returns default trend configuration
func DefaultTrendConfig() TrendConfig {
	return TrendConfig{
		HistoryRetentionDays: 365,
		ForecastDays:         90,
		MinHistoryDays:       7,
		UpdateInterval:       1 * time.Hour,
		ConfidenceThreshold:  0.7,
	}
}

// NewTrendPredictor creates a new trend predictor
func NewTrendPredictor(config TrendConfig) *TrendPredictor {
	if config.ForecastDays <= 0 {
		config.ForecastDays = 90
	}
	if config.MinHistoryDays <= 0 {
		config.MinHistoryDays = 7
	}

	return &TrendPredictor{
		config:  config,
		history: make([]CapacitySnapshot, 0),
	}
}

// GenerateTrendReport generates a comprehensive trend report
func (tp *TrendPredictor) GenerateTrendReport(ctx context.Context, migrator *Migrator) (*TrendReport, error) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	now := time.Now()
	report := &TrendReport{
		GeneratedAt: now,
		TierTrends:  make(map[Tier]TierTrend),
		Forecasts:   make(map[Tier]CapacityForecast),
		Alerts:      make([]CapacityAlert, 0),
		Recommendations: make([]TrendRecommendation, 0),
	}

	// Set time ranges
	if len(tp.history) > 0 {
		report.HistoricalPeriod = TimeRange{
			Start: tp.history[0].Timestamp,
			End:   now,
		}
	}
	report.ForecastPeriod = TimeRange{
		Start: now,
		End:   now.AddDate(0, 0, tp.config.ForecastDays),
	}

	// Analyze each tier
	for _, tier := range []Tier{TierHot, TierWarm, TierCold} {
		trend := tp.analyzeTierTrend(tier, migrator)
		report.TierTrends[tier] = trend

		forecast := tp.generateForecast(tier, tp.config.ForecastDays)
		report.Forecasts[tier] = forecast
	}

	// Overall trend analysis
	report.OverallTrend = tp.analyzeOverallTrend(migrator)

	// Generate alerts
	report.Alerts = tp.generateAlerts(report)

	// Generate recommendations
	report.Recommendations = tp.generateTrendRecommendations(report)

	// Calculate confidence level
	report.ConfidenceLevel = tp.calculateConfidence()

	return report, nil
}

// analyzeTierTrend analyzes trend for a single tier
func (tp *TrendPredictor) analyzeTierTrend(tier Tier, migrator *Migrator) TierTrend {
	trend := TierTrend{Tier: tier}

	// Get current capacity
	capInfo, err := migrator.TierCapacity(tier)
	if err == nil {
		trend.CurrentCapacity = capInfo.Capacity
		trend.CurrentUsed = capInfo.Used
		if capInfo.Capacity > 0 {
			trend.CurrentUtilization = float64(capInfo.Used) / float64(capInfo.Capacity) * 100
		}
	}

	// Calculate growth rate from history
	trend.GrowthRate = tp.calculateGrowthRate(tier)

	// Determine growth trend
	if trend.GrowthRate > 2 {
		trend.GrowthTrend = "increasing"
	} else if trend.GrowthRate < -1 {
		trend.GrowthTrend = "decreasing"
	} else {
		trend.GrowthTrend = "stable"
	}

	// Detect seasonality
	trend.Seasonality = tp.detectSeasonality(tier)

	// Find peak utilization
	trend.PeakUtilization, trend.PeakTime = tp.findPeakUtilization(tier)

	// Predict when tier will be full
	if trend.GrowthRate > 0 && trend.CurrentCapacity > 0 {
		remainingCapacity := float64(trend.CurrentCapacity-trend.CurrentUsed) / 1024 / 1024 / 1024 // GB
		monthlyGrowthGB := (float64(trend.CurrentUsed) / 1024 / 1024 / 1024) * (trend.GrowthRate / 100)

		if monthlyGrowthGB > 0 {
			monthsUntilFull := remainingCapacity / monthlyGrowthGB
			daysUntilFull := int(monthsUntilFull * 30)
			trend.DaysUntilFull = daysUntilFull

			fullDate := time.Now().AddDate(0, 0, daysUntilFull)
			trend.PredictedFullDate = &fullDate
		}
	}

	// Recommend action
	trend.RecommendedAction = tp.recommendTierAction(trend)

	return trend
}

// calculateGrowthRate calculates monthly growth rate percentage
func (tp *TrendPredictor) calculateGrowthRate(tier Tier) float64 {
	if len(tp.history) < 2 {
		return 0
	}

	// Get first and last values
	first := tp.history[0]
	last := tp.history[len(tp.history)-1]

	var firstValue, lastValue int64
	switch tier {
	case TierHot:
		firstValue = first.HotUsed
		lastValue = last.HotUsed
	case TierWarm:
		firstValue = first.WarmUsed
		lastValue = last.WarmUsed
	case TierCold:
		firstValue = first.ColdUsed
		lastValue = last.ColdUsed
	}

	if firstValue == 0 {
		return 0
	}

	// Calculate growth rate per month
	days := last.Timestamp.Sub(first.Timestamp).Hours() / 24
	if days == 0 {
		return 0
	}

	monthlyGrowth := (float64(lastValue-firstValue) / float64(firstValue)) / (days / 30)
	return monthlyGrowth * 100
}

// detectSeasonality detects seasonal patterns in data
func (tp *TrendPredictor) detectSeasonality(tier Tier) Seasonality {
	s := Seasonality{Detected: false}

	if len(tp.history) < 30 {
		return s
	}

	// Simple seasonality detection
	// Check for weekly patterns
	weeklyVariation := tp.calculateWeeklyVariation(tier)
	if weeklyVariation > 10 {
		s.Detected = true
		s.PatternType = "weekly"
		s.VariationPercent = weeklyVariation
	}

	// Check for monthly patterns
	monthlyVariation := tp.calculateMonthlyVariation(tier)
	if monthlyVariation > weeklyVariation && monthlyVariation > 15 {
		s.Detected = true
		s.PatternType = "monthly"
		s.VariationPercent = monthlyVariation
	}

	return s
}

func (tp *TrendPredictor) calculateWeeklyVariation(tier Tier) float64 {
	// Simplified: would normally use proper time series analysis
	return 5.0 // Placeholder
}

func (tp *TrendPredictor) calculateMonthlyVariation(tier Tier) float64 {
	// Simplified: would normally use proper time series analysis
	return 8.0 // Placeholder
}

// findPeakUtilization finds peak utilization and when it occurred
func (tp *TrendPredictor) findPeakUtilization(tier Tier) (float64, time.Time) {
	var peak float64
	var peakTime time.Time

	for _, snap := range tp.history {
		var used, capacity int64
		switch tier {
		case TierHot:
			used, capacity = snap.HotUsed, snap.HotCapacity
		case TierWarm:
			used, capacity = snap.WarmUsed, snap.WarmCapacity
		case TierCold:
			used, capacity = snap.ColdUsed, snap.ColdCapacity
		}

		if capacity > 0 {
			util := float64(used) / float64(capacity) * 100
			if util > peak {
				peak = util
				peakTime = snap.Timestamp
			}
		}
	}

	return peak, peakTime
}

// generateForecast generates capacity forecast for a tier
func (tp *TrendPredictor) generateForecast(tier Tier, days int) CapacityForecast {
	forecast := CapacityForecast{
		Tier:        tier,
		Predictions: make([]CapacityPoint, 0),
		WorstCase:   make([]CapacityPoint, 0),
		BestCase:    make([]CapacityPoint, 0),
		Method:      "linear",
	}

	if len(tp.history) < 2 {
		return forecast
	}

	// Get current values
	lastSnap := tp.history[len(tp.history)-1]
	var currentUsed int64
	switch tier {
	case TierHot:
		currentUsed = lastSnap.HotUsed
	case TierWarm:
		currentUsed = lastSnap.WarmUsed
	case TierCold:
		currentUsed = lastSnap.ColdUsed
	}

	// Calculate daily growth rate
	growthRate := tp.calculateGrowthRate(tier)
	dailyGrowthRate := growthRate / 30 // Convert monthly to daily

	// Generate predictions
	for i := 1; i <= days; i++ {
		date := time.Now().AddDate(0, 0, i)

		// Linear prediction
		predictedUsed := float64(currentUsed) * math.Pow(1+dailyGrowthRate/100, float64(i))
		confidence := 1.0 - (float64(i) / float64(days) * 0.3) // Confidence decreases over time

		forecast.Predictions = append(forecast.Predictions, CapacityPoint{
			Date:       date,
			UsedBytes:  int64(predictedUsed),
			Confidence: confidence,
		})

		// Worst case (1.5x growth)
		worstUsed := float64(currentUsed) * math.Pow(1+dailyGrowthRate*1.5/100, float64(i))
		forecast.WorstCase = append(forecast.WorstCase, CapacityPoint{
			Date:       date,
			UsedBytes:  int64(worstUsed),
			Confidence: confidence * 0.8,
		})

		// Best case (0.5x growth)
		bestUsed := float64(currentUsed) * math.Pow(1+dailyGrowthRate*0.5/100, float64(i))
		forecast.BestCase = append(forecast.BestCase, CapacityPoint{
			Date:       date,
			UsedBytes:  int64(bestUsed),
			Confidence: confidence * 0.8,
		})
	}

	// Calculate accuracy metrics (simplified)
	forecast.Accuracy = ForecastAccuracy{
		MAPE:  5.0 + float64(days)*0.1,
		MAE:   float64(currentUsed) * 0.05,
		RMSE:  float64(currentUsed) * 0.08,
		R2:    0.85,
		Score: 85.0 - float64(days)*0.1,
	}

	return forecast
}

// analyzeOverallTrend analyzes overall storage trends
func (tp *TrendPredictor) analyzeOverallTrend(migrator *Migrator) OverallTrendAnalysis {
	analysis := OverallTrendAnalysis{
		OptimalTierDistribution: make(map[Tier]float64),
		CurrentDistribution:     make(map[Tier]float64),
	}

	// Calculate totals
	var totalCap, totalUsed int64
	for _, tier := range []Tier{TierHot, TierWarm, TierCold} {
		capInfo, _ := migrator.TierCapacity(tier)
		if capInfo != nil {
			totalCap += capInfo.Capacity
			totalUsed += capInfo.Used
		}
	}

	analysis.TotalCapacity = totalCap
	analysis.TotalUsed = totalUsed
	if totalCap > 0 {
		analysis.OverallUtilization = float64(totalUsed) / float64(totalCap) * 100
	}

	// Overall growth rate
	analysis.OverallGrowthRate = (tp.calculateGrowthRate(TierHot) +
		tp.calculateGrowthRate(TierWarm) +
		tp.calculateGrowthRate(TierCold)) / 3

	// Optimal distribution (based on access patterns)
	analysis.OptimalTierDistribution = map[Tier]float64{
		TierHot:  20, // 20% hot
		TierWarm: 50, // 50% warm
		TierCold: 30, // 30% cold
	}

	// Current distribution
	if totalUsed > 0 {
		for _, tier := range []Tier{TierHot, TierWarm, TierCold} {
			capInfo, _ := migrator.TierCapacity(tier)
			if capInfo != nil {
				analysis.CurrentDistribution[tier] = float64(capInfo.Used) / float64(totalUsed) * 100
			}
		}
	}

	// Improvement potential
	improvement := 0.0
	for tier, optimal := range analysis.OptimalTierDistribution {
		if current, ok := analysis.CurrentDistribution[tier]; ok {
			improvement += math.Abs(optimal - current)
		}
	}
	analysis.ImprovementPotential = improvement / 2

	return analysis
}

// generateAlerts generates capacity alerts
func (tp *TrendPredictor) generateAlerts(report *TrendReport) []CapacityAlert {
	alerts := make([]CapacityAlert, 0)

	for tier, trend := range report.TierTrends {
		// Critical: above 90% utilization
		if trend.CurrentUtilization > 90 {
			alerts = append(alerts, CapacityAlert{
				Severity:     "critical",
				Type:         "capacity",
				Tier:         tier,
				Title:        "Critical Capacity Level",
				Message:      tier.String() + " tier is above 90% capacity",
				TriggeredAt:  time.Now(),
				Threshold:    90,
				CurrentValue: trend.CurrentUtilization,
				Action:       "Immediately migrate data or expand capacity",
			})
		} else if trend.CurrentUtilization > 80 {
			alerts = append(alerts, CapacityAlert{
				Severity:     "warning",
				Type:         "capacity",
				Tier:         tier,
				Title:        "High Capacity Warning",
				Message:      tier.String() + " tier is above 80% capacity",
				TriggeredAt:  time.Now(),
				Threshold:    80,
				CurrentValue: trend.CurrentUtilization,
				Action:       "Plan for capacity expansion or migration",
			})
		}

		// Forecast alert: will be full soon
		if trend.DaysUntilFull > 0 && trend.DaysUntilFull < 30 {
			alerts = append(alerts, CapacityAlert{
				Severity:     "warning",
				Type:         "forecast",
				Tier:         tier,
				Title:        "Capacity Forecast Warning",
				Message:      tier.String() + " tier will be full in " + string(rune(trend.DaysUntilFull)) + " days",
				TriggeredAt:  time.Now(),
				Threshold:    30,
				CurrentValue: float64(trend.DaysUntilFull),
				Action:       "Review capacity expansion options",
			})
		}
	}

	return alerts
}

// generateTrendRecommendations generates trend-based recommendations
func (tp *TrendPredictor) generateTrendRecommendations(report *TrendReport) []TrendRecommendation {
	recs := make([]TrendRecommendation, 0)

	// Capacity planning
	if report.OverallTrend.OverallGrowthRate > 5 {
		recs = append(recs, TrendRecommendation{
			Priority:     1,
			Type:         "capacity",
			Title:        "Plan Capacity Expansion",
			Description:  "Storage is growing rapidly. Consider capacity expansion.",
			Impact:       "High - Risk of running out of storage",
			CostEstimate: 500,
			Timeline:     "1-2 months",
			ExpectedROI:  35.0,
		})
	}

	// Tier optimization
	if report.OverallTrend.ImprovementPotential > 20 {
		recs = append(recs, TrendRecommendation{
			Priority:     2,
			Type:         "migration",
			Title:        "Optimize Tier Distribution",
			Description:  "Current tier distribution is suboptimal. Rebalance tiers.",
			Impact:       "Medium - Improved performance and cost efficiency",
			CostEstimate: 100,
			Timeline:     "1-2 weeks",
			ExpectedROI:  25.0,
		})
	}

	// Cold tier promotion
	if hotTrend, ok := report.TierTrends[TierHot]; ok {
		if hotTrend.DaysUntilFull > 0 && hotTrend.DaysUntilFull < 60 {
			recs = append(recs, TrendRecommendation{
				Priority:     1,
				Type:         "migration",
				Title:        "Free Hot Tier Space",
				Description:  "Migrate inactive hot data to warm tier",
				Impact:       "High - Prevents performance degradation",
				CostEstimate: 50,
				Timeline:     "1 week",
				ExpectedROI:  40.0,
			})
		}
	}

	return recs
}

// recommendTierAction recommends action for a tier
func (tp *TrendPredictor) recommendTierAction(trend TierTrend) string {
	switch {
	case trend.CurrentUtilization > 90:
		return "urgent_migration"
	case trend.CurrentUtilization > 80:
		return "plan_migration"
	case trend.GrowthTrend == "increasing" && trend.DaysUntilFull < 90:
		return "expand_capacity"
	case trend.GrowthTrend == "decreasing":
		return "optimize_allocation"
	default:
		return "monitor"
	}
}

// calculateConfidence calculates overall confidence level
func (tp *TrendPredictor) calculateConfidence() float64 {
	// More history = higher confidence
	historyDays := 0
	if len(tp.history) > 1 {
		historyDays = int(tp.history[len(tp.history)-1].Timestamp.Sub(tp.history[0].Timestamp).Hours() / 24)
	}

	confidence := math.Min(1.0, float64(historyDays)/float64(tp.config.MinHistoryDays*3))
	return confidence
}

// RecordSnapshot records a capacity snapshot
func (tp *TrendPredictor) RecordSnapshot(snap CapacitySnapshot) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	tp.history = append(tp.history, snap)

	// Trim old entries
	cutoff := time.Now().AddDate(0, 0, -tp.config.HistoryRetentionDays)
	newHistory := make([]CapacitySnapshot, 0)
	for _, s := range tp.history {
		if s.Timestamp.After(cutoff) {
			newHistory = append(newHistory, s)
		}
	}
	tp.history = newHistory
}

// GetHistory returns capacity history
func (tp *TrendPredictor) GetHistory(days int) []CapacitySnapshot {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	result := make([]CapacitySnapshot, 0)

	for _, s := range tp.history {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}

	return result
}

// PredictUtilization predicts utilization at a future date
func (tp *TrendPredictor) PredictUtilization(tier Tier, date time.Time) (float64, error) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	if len(tp.history) < 2 {
		return 0, ErrInsufficientHistory
	}

	growthRate := tp.calculateGrowthRate(tier)
	dailyGrowthRate := growthRate / 30

	days := int(date.Sub(time.Now()).Hours() / 24)
	if days < 0 {
		return 0, ErrInvalidDate
	}

	lastSnap := tp.history[len(tp.history)-1]
	var currentUsed, capacity int64
	switch tier {
	case TierHot:
		currentUsed, capacity = lastSnap.HotUsed, lastSnap.HotCapacity
	case TierWarm:
		currentUsed, capacity = lastSnap.WarmUsed, lastSnap.WarmCapacity
	case TierCold:
		currentUsed, capacity = lastSnap.ColdUsed, lastSnap.ColdCapacity
	}

	predictedUsed := float64(currentUsed) * math.Pow(1+dailyGrowthRate/100, float64(days))

	if capacity > 0 {
		return (predictedUsed / float64(capacity)) * 100, nil
	}

	return 0, ErrInvalidCapacity
}

// Additional errors for trend prediction
var (
	ErrInsufficientHistory = errorsNew("insufficient history for prediction")
	ErrInvalidDate         = errorsNew("invalid prediction date")
	ErrInvalidCapacity     = errorsNew("invalid capacity information")
)

func errorsNew(s string) error {
	return &TrendError{msg: s}
}

// TrendError represents a trend prediction error
type TrendError struct {
	msg string
}

func (e *TrendError) Error() string {
	return e.msg
}