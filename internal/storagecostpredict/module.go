// Package storagecostpredict provides a storage cost prediction engine.
// It predicts future storage costs based on historical data, analyzes capacity
// growth trends, detects cost anomalies, and warns when budget overruns are likely.
package storagecostpredict

import (
	"math"
	"sort"
	"time"
)

// CostPrediction represents a predicted storage cost for a future period.
type CostPrediction struct {
	// PeriodStart is the start time of the predicted period.
	PeriodStart time.Time
	// PeriodEnd is the end time of the predicted period.
	PeriodEnd time.Time
	// PredictedCost is the estimated total cost in CNY.
	PredictedCost float64
	// ConfidenceLevel is the prediction confidence (0-1).
	ConfidenceLevel float64
	// CostBreakdown holds per-category cost estimates.
	CostBreakdown map[string]float64
	// Method is the prediction method used (e.g. "linear", "exponential", "arima").
	Method string
}

// GrowthTrend represents a capacity growth trend over a time range.
type GrowthTrend struct {
	// StartTime is the beginning of the analysed range.
	StartTime time.Time
	// EndTime is the end of the analysed range.
	EndTime time.Time
	// CurrentCapacityTB is the latest known capacity in TB.
	CurrentCapacityTB float64
	// GrowthRatePerMonth is the average monthly growth rate (fraction, e.g. 0.05 = 5%).
	GrowthRatePerMonth float64
	// ProjectedCapacityTB is the projected capacity at the end of the forecast horizon.
	ProjectedCapacityTB float64
	// TrendType describes the trend shape: "linear", "exponential", "logarithmic".
	TrendType string
	// R2Score is the coefficient of determination for the fitted model (0-1).
	R2Score float64
}

// CostAnomaly represents a detected cost anomaly in historical data.
type CostAnomaly struct {
	// DetectedAt is when the anomaly was found.
	DetectedAt time.Time
	// PeriodStart is the start of the anomalous billing period.
	PeriodStart time.Time
	// PeriodEnd is the end of the anomalous billing period.
	PeriodEnd time.Time
	// ExpectedCost is what the cost was expected to be.
	ExpectedCost float64
	// ActualCost is what was actually observed.
	ActualCost float64
	// Deviation is the ratio (ActualCost - ExpectedCost) / ExpectedCost.
	Deviation float64
	// Severity is "low", "medium", or "high".
	Severity string
	// Description is a human-readable explanation.
	Description string
}

// BudgetAlert represents a budget overrun warning or notification.
type BudgetAlert struct {
	// AlertID is a unique identifier for this alert.
	AlertID string
	// TriggeredAt is when the alert was generated.
	TriggeredAt time.Time
	// BudgetLimit is the budget threshold in CNY.
	BudgetLimit float64
	// ProjectedSpend is the projected total spend for the budget period.
	ProjectedSpend float64
	// CurrentSpend is the spend recorded so far in the budget period.
	CurrentSpend float64
	// AlertType is "warning" (approaching limit) or "overrun" (exceeded limit).
	AlertType string
	// Message is a human-readable alert message.
	Message string
}

// HistorySample is a single historical data point used for prediction.
type HistorySample struct {
	Timestamp  time.Time
	CostCNY    float64
	CapacityTB float64
}

// Engine is the core prediction engine.
type Engine struct {
	// History holds all historical samples in chronological order.
	History []HistorySample
	// WarningThreshold is the fraction of budget at which a warning alert fires (e.g. 0.8 = 80%).
	WarningThreshold float64
}

// NewEngine creates a prediction engine pre-loaded with history samples.
func NewEngine(history []HistorySample, warningThreshold float64) *Engine {
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})
	if warningThreshold <= 0 || warningThreshold > 1 {
		warningThreshold = 0.8
	}
	return &Engine{History: history, WarningThreshold: warningThreshold}
}

// linear Regression result.
type linReg struct {
	slope     float64
	intercept float64
	r2        float64
}

// fitLinear fits y = slope*x + intercept via ordinary least squares.
func fitLinear(x, y []float64) linReg {
	n := float64(len(x))
	if n == 0 || len(x) != len(y) {
		return linReg{}
	}
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return linReg{}
	}
	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n
	// R²
	ssRes := 0.0
	ssTot := 0.0
	meanY := sumY / n
	for i := range x {
		pred := slope*x[i] + intercept
		ssRes += (y[i] - pred) * (y[i] - pred)
		ssTot += (y[i] - meanY) * (y[i] - meanY)
	}
	r2 := 1.0
	if ssTot != 0 {
		r2 = 1 - ssRes/ssTot
	}
	return linReg{slope: slope, intercept: intercept, r2: r2}
}

// PredictCost predicts the total storage cost for the given future period
// using linear regression on historical cost data.
func (e *Engine) PredictCost(periodStart, periodEnd time.Time) CostPrediction {
	if len(e.History) < 2 {
		return CostPrediction{
			PeriodStart:      periodStart,
			PeriodEnd:        periodEnd,
			PredictedCost:    0,
			ConfidenceLevel:  0,
			Method:           "insufficient-data",
		}
	}

	// Convert timestamps to days-since-first-sample for regression.
	t0 := e.History[0].Timestamp
	x := make([]float64, len(e.History))
	y := make([]float64, len(e.History))
	for i, s := range e.History {
		x[i] = s.Timestamp.Sub(t0).Hours() / 24
		y[i] = s.CostCNY
	}
	reg := fitLinear(x, y)

	// Predict the daily cost at the midpoint of the target period,
	// then multiply by the number of days.
	days := periodEnd.Sub(periodStart).Hours() / 24
	if days <= 0 {
		days = 30
	}
	midDay := periodStart.Add(periodEnd.Sub(periodStart) / 2).Sub(t0).Hours() / 24
	dailyCost := reg.slope*midDay + reg.intercept
	if dailyCost < 0 {
		dailyCost = 0
	}

	// Confidence: R² scaled, clamp to [0, 0.95] because predictions are never certain.
	conf := reg.r2
	if conf < 0 {
		conf = 0
	}
	if conf > 0.95 {
		conf = 0.95
	}

	return CostPrediction{
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		PredictedCost:    dailyCost * days,
		ConfidenceLevel:  conf,
		CostBreakdown: map[string]float64{
			"hardware":   dailyCost * days * 0.35,
			"electricity": dailyCost * days * 0.20,
			"maintenance": dailyCost * days * 0.15,
			"cloud":      dailyCost * days * 0.20,
			"other":      dailyCost * days * 0.10,
		},
		Method: "linear-regression",
	}
}

// AnalyzeGrowth analyses historical capacity growth and projects future capacity.
// forecastMonths is the number of months ahead to project.
func (e *Engine) AnalyzeGrowth(forecastMonths int) GrowthTrend {
	if len(e.History) < 2 || forecastMonths <= 0 {
		return GrowthTrend{}
	}

	t0 := e.History[0].Timestamp
	x := make([]float64, len(e.History))
	y := make([]float64, len(e.History))
	for i, s := range e.History {
		x[i] = s.Timestamp.Sub(t0).Hours() / 24 / 30 // months
		y[i] = s.CapacityTB
	}
	reg := fitLinear(x, y)

	current := e.History[len(e.History)-1].CapacityTB
	projectedMonth := x[len(x)-1] + float64(forecastMonths)
	projected := reg.slope*projectedMonth + reg.intercept
	if projected < 0 {
		projected = 0
	}

	trendType := "linear"
	growthRate := 0.0
	if current > 0 {
		growthRate = reg.slope / current
	}
	if growthRate < 0 {
		growthRate = 0
	}

	return GrowthTrend{
		StartTime:           t0,
		EndTime:             e.History[len(e.History)-1].Timestamp,
		CurrentCapacityTB:   current,
		GrowthRatePerMonth:  growthRate,
		ProjectedCapacityTB: projected,
		TrendType:           trendType,
		R2Score:             reg.r2,
	}
}

// DetectAnomaly scans historical samples for cost anomalies.
// An anomaly is flagged when the actual cost deviates from the rolling
// average by more than the given threshold (e.g. 0.3 = 30%).
func (e *Engine) DetectAnomaly(threshold float64) []CostAnomaly {
	if len(e.History) < 3 || threshold <= 0 {
		return nil
	}
	if threshold > 1 {
		threshold = 1
	}

	var anomalies []CostAnomaly
	window := 3 // rolling window size
	for i := window; i < len(e.History); i++ {
		// Average of previous `window` samples.
		var sum float64
		for j := i - window; j < i; j++ {
			sum += e.History[j].CostCNY
		}
		expected := sum / float64(window)
		actual := e.History[i].CostCNY

		if expected == 0 {
			continue
		}
		deviation := (actual - expected) / expected
		if math.Abs(deviation) > threshold {
			severity := "low"
			absDev := math.Abs(deviation)
			if absDev > 0.5 {
				severity = "high"
			} else if absDev > 0.3 {
				severity = "medium"
			}
			desc := "Cost spike detected"
			if deviation < 0 {
				desc = "Cost drop detected"
			}
			anomalies = append(anomalies, CostAnomaly{
				DetectedAt:   time.Now(),
				PeriodStart:  e.History[i-1].Timestamp,
				PeriodEnd:    e.History[i].Timestamp,
				ExpectedCost: expected,
				ActualCost:   actual,
				Deviation:    deviation,
				Severity:     severity,
				Description:  desc,
			})
		}
	}
	return anomalies
}

// CheckBudgetOverrun compares projected spend against the budget limit and
// returns a BudgetAlert if the projected spend exceeds the warning threshold.
func (e *Engine) CheckBudgetOverrun(budgetLimit, currentSpend float64, periodStart, periodEnd time.Time) *BudgetAlert {
	if budgetLimit <= 0 {
		return nil
	}

	prediction := e.PredictCost(periodStart, periodEnd)
	projectedSpend := currentSpend + prediction.PredictedCost

	// Already over budget?
	if currentSpend >= budgetLimit {
		return &BudgetAlert{
			AlertID:       "budget-overrun",
			TriggeredAt:   time.Now(),
			BudgetLimit:   budgetLimit,
			ProjectedSpend: projectedSpend,
			CurrentSpend:  currentSpend,
			AlertType:     "overrun",
			Message:       "Budget limit has been exceeded. Immediate action required.",
		}
	}

	// Projected to exceed?
	if projectedSpend >= budgetLimit {
		return &BudgetAlert{
			AlertID:       "budget-projected-overrun",
			TriggeredAt:   time.Now(),
			BudgetLimit:   budgetLimit,
			ProjectedSpend: projectedSpend,
			CurrentSpend:  currentSpend,
			AlertType:     "overrun",
			Message:       "Projected spend will exceed the budget limit before the period ends.",
		}
	}

	// Warning threshold reached?
	if currentSpend >= budgetLimit*e.WarningThreshold {
		return &BudgetAlert{
			AlertID:       "budget-warning",
			TriggeredAt:   time.Now(),
			BudgetLimit:   budgetLimit,
			ProjectedSpend: projectedSpend,
			CurrentSpend:  currentSpend,
			AlertType:     "warning",
			Message:       "Spending has exceeded the warning threshold. Monitor closely.",
		}
	}

	return nil
}