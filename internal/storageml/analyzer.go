package storageml

import (
	"math"
	"sort"
	"time"
)

// Analyzer provides analysis of storage metrics
type Analyzer struct {
	ml *StorageML
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer(ml *StorageML) *Analyzer {
	return &Analyzer{ml: ml}
}

// TrendAnalysis represents trend analysis results
type TrendAnalysis struct {
	PoolID        string     `json:"pool_id"`
	MetricType    MetricType `json:"metric_type"`
	Direction     string     `json:"direction"`
	AvgChangeRate float64    `json:"avg_change_rate"`
	MaxChangeRate float64    `json:"max_change_rate"`
	Volatility    float64    `json:"volatility"`
	DataPoints    int        `json:"data_points"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       time.Time  `json:"end_time"`
}

// Anomaly represents an anomalous data point
type Anomaly struct {
	DataPoint   DataPoint `json:"data_point"`
	Expected    float64   `json:"expected"`
	Deviation   float64   `json:"deviation"`
	Severity    string    `json:"severity"` // "low", "medium", "high"
	Description string    `json:"description"`
}

// AnalyzeTrend analyzes the trend of a specific metric for a pool
func (a *Analyzer) AnalyzeTrend(poolID string, metricType MetricType, duration time.Duration) (*TrendAnalysis, error) {
	points := a.ml.GetDataPoints(poolID)

	// Filter by type and time range
	cutoff := time.Now().Add(-duration)
	var filtered []DataPoint
	for _, dp := range points {
		if dp.Type == metricType && dp.Timestamp.After(cutoff) {
			filtered = append(filtered, dp)
		}
	}

	if len(filtered) < 2 {
		return nil, ErrInsufficientData
	}

	// Sort by time
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	// Calculate change rates
	var changes []float64
	for i := 1; i < len(filtered); i++ {
		duration := filtered[i].Timestamp.Sub(filtered[i-1].Timestamp).Hours()
		if duration > 0 {
			changeRate := (filtered[i].Value - filtered[i-1].Value) / duration
			changes = append(changes, changeRate)
		}
	}

	// Calculate statistics
	avgChange := average(changes)
	maxChange := maxAbs(changes)
	vol := volatility(changes)

	// Determine direction
	direction := "stable"
	if avgChange > 0.01 {
		direction = "increasing"
	} else if avgChange < -0.01 {
		direction = "decreasing"
	}

	return &TrendAnalysis{
		PoolID:        poolID,
		MetricType:    metricType,
		Direction:     direction,
		AvgChangeRate: avgChange,
		MaxChangeRate: maxChange,
		Volatility:    vol,
		DataPoints:    len(filtered),
		StartTime:     filtered[0].Timestamp,
		EndTime:       filtered[len(filtered)-1].Timestamp,
	}, nil
}

// DetectAnomalies detects anomalies in storage metrics
func (a *Analyzer) DetectAnomalies(poolID string, metricType MetricType) ([]Anomaly, error) {
	points := a.ml.GetDataPoints(poolID)

	// Filter by type
	var filtered []DataPoint
	for _, dp := range points {
		if dp.Type == metricType {
			filtered = append(filtered, dp)
		}
	}

	if len(filtered) < 10 {
		return nil, ErrInsufficientData
	}

	// Calculate mean and standard deviation
	mean, stddev := meanStddev(filtered)

	var anomalies []Anomaly
	for _, dp := range filtered {
		deviation := (dp.Value - mean) / stddev
		if deviation > 2.0 || deviation < -2.0 {
			severity := "low"
			if deviation > 3.0 || deviation < -3.0 {
				severity = "high"
			} else if deviation > 2.5 || deviation < -2.5 {
				severity = "medium"
			}

			anomalies = append(anomalies, Anomaly{
				DataPoint:   dp,
				Expected:    mean,
				Deviation:   deviation,
				Severity:    severity,
				Description: generateAnomalyDescription(dp, mean, deviation),
			})
		}
	}

	return anomalies, nil
}

// GetUsageSummary returns a summary of storage usage
func (a *Analyzer) GetUsageSummary(poolID string) map[string]interface{} {
	points := a.ml.GetDataPoints(poolID)
	config, _ := a.ml.GetPoolConfig(poolID)

	summary := map[string]interface{}{
		"pool_id": poolID,
		"name":    config.Name,
	}

	// Get latest capacity
	var latestCapacity float64
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Type == MetricCapacity {
			latestCapacity = points[i].Value
			break
		}
	}

	summary["current_usage_gb"] = latestCapacity
	summary["total_capacity_gb"] = config.TotalCapacity
	summary["usage_percent"] = (latestCapacity / config.TotalCapacity) * 100
	summary["free_gb"] = config.TotalCapacity - latestCapacity
	summary["data_points"] = len(points)

	return summary
}

// Helper functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func maxAbs(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := 0.0
	for _, v := range values {
		abs := v
		if abs < 0 {
			abs = -abs
		}
		if abs > max {
			max = abs
		}
	}
	return max
}

func volatility(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	_, stddev := meanStddevRaw(values)
	return stddev
}

func meanStddev(points []DataPoint) (float64, float64) {
	values := make([]float64, len(points))
	for i, p := range points {
		values[i] = p.Value
	}
	return meanStddevRaw(values)
}

func meanStddevRaw(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	variance := sumSq / float64(len(values))
	return mean, math.Sqrt(variance)
}

func generateAnomalyDescription(dp DataPoint, expected, deviation float64) string {
	if deviation > 0 {
		return "值异常偏高"
	}
	return "值异常偏低"
}
