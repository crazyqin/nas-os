package storageml

import (
	"math"
	"sort"
	"time"
)

// Predictor handles storage prediction using linear regression and seasonal analysis
type Predictor struct {
	ml *StorageML
}

// NewPredictor creates a new predictor
func NewPredictor(ml *StorageML) *Predictor {
	return &Predictor{ml: ml}
}

// Predict makes a prediction for a pool's metric at a future date
func (p *Predictor) Predict(poolID string, metricType MetricType, futureDate time.Time) (*PredictionResult, error) {
	p.ml.mu.RLock()
	dataPoints := p.ml.dataPoints[poolID]
	p.ml.mu.RUnlock()

	// Filter data points for the specific metric type
	filtered := p.filterByType(dataPoints, metricType)
	if len(filtered) < 2 {
		return nil, ErrInsufficientData
	}

	// Sort by timestamp
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	// Perform linear regression
	regression := p.linearRegression(filtered)

	// Detect seasonal component
	seasonal := p.detectSeasonal(filtered)

	// Make prediction
	baseTime := filtered[0].Timestamp
	futureOffset := futureDate.Sub(baseTime).Hours()
	predictedValue := regression.Slope*futureOffset + regression.Intercept

	// Apply seasonal adjustment if detected
	if seasonal != nil {
		seasonalOffset := math.Sin(2*math.Pi*futureOffset/seasonal.Period.Hours() + seasonal.Phase)
		predictedValue += seasonal.Amplitude * seasonalOffset
	}

	// Calculate confidence
	confidence := p.calculateConfidence(filtered, regression)

	// Determine trend direction
	trendDirection := "stable"
	if regression.Slope > 0.01 {
		trendDirection = "increasing"
	} else if regression.Slope < -0.01 {
		trendDirection = "decreasing"
	}

	// Calculate seasonal factor
	seasonalFactor := 0.0
	if seasonal != nil {
		seasonalFactor = seasonal.Amplitude
	}

	return &PredictionResult{
		PoolID:         poolID,
		MetricType:     metricType,
		CurrentValue:   filtered[len(filtered)-1].Value,
		PredictedValue: predictedValue,
		PredictedDate:  futureDate,
		Confidence:     confidence,
		TrendDirection: trendDirection,
		SeasonalFactor: seasonalFactor,
	}, nil
}

// filterByType filters data points by metric type
func (p *Predictor) filterByType(points []DataPoint, metricType MetricType) []DataPoint {
	var filtered []DataPoint
	for _, dp := range points {
		if dp.Type == metricType {
			filtered = append(filtered, dp)
		}
	}
	return filtered
}

// linearRegression performs simple linear regression
func (p *Predictor) linearRegression(points []DataPoint) *LinearRegression {
	n := float64(len(points))
	if n < 2 {
		return &LinearRegression{}
	}

	baseTime := points[0].Timestamp
	var sumX, sumY, sumXY, sumX2 float64

	for _, dp := range points {
		x := dp.Timestamp.Sub(baseTime).Hours()
		y := dp.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	// Calculate R-squared
	meanY := sumY / n
	var ssRes, ssTot float64
	for _, dp := range points {
		x := dp.Timestamp.Sub(baseTime).Hours()
		predicted := slope*x + intercept
		ssRes += (dp.Value - predicted) * (dp.Value - predicted)
		ssTot += (dp.Value - meanY) * (dp.Value - meanY)
	}

	r2 := 0.0
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}

	return &LinearRegression{
		Slope:     slope,
		Intercept: intercept,
		R2:        r2,
	}
}

// detectSeasonal detects seasonal patterns in the data
func (p *Predictor) detectSeasonal(points []DataPoint) *SeasonalComponent {
	if len(points) < 24 { // Need at least 24 data points
		return nil
	}

	// Try common periods: daily (24h), weekly (168h), monthly (720h)
	periods := []float64{24, 168, 720}

	bestPeriod := 0.0
	bestAmplitude := 0.0
	bestScore := 0.0

	baseTime := points[0].Timestamp

	for _, period := range periods {
		// Calculate correlation with sinusoidal pattern
		var sumSin, sumCos, sumVal float64
		n := float64(len(points))

		for _, dp := range points {
			t := dp.Timestamp.Sub(baseTime).Hours()
			angle := 2 * math.Pi * t / period
			sumSin += math.Sin(angle) * dp.Value
			sumCos += math.Cos(angle) * dp.Value
			sumVal += dp.Value
		}

		amplitude := math.Sqrt(sumSin*sumSin+sumCos*sumCos) / n
		score := amplitude / (sumVal/n + 0.001) // Normalized amplitude

		if score > bestScore {
			bestScore = score
			bestPeriod = period
			bestAmplitude = amplitude
		}
	}

	if bestScore < 0.05 { // Threshold for seasonal detection
		return nil
	}

	return &SeasonalComponent{
		Period:    time.Duration(bestPeriod * float64(time.Hour)),
		Amplitude: bestAmplitude,
		Phase:     0,
	}
}

// calculateConfidence calculates prediction confidence
func (p *Predictor) calculateConfidence(points []DataPoint, regression *LinearRegression) float64 {
	// Confidence based on R-squared and data quantity
	dataQuality := math.Min(float64(len(points))/100.0, 1.0)
	r2Score := regression.R2

	// Combine factors
	confidence := (dataQuality*0.4 + r2Score*0.6) * 100
	return math.Min(math.Max(confidence, 0), 100)
}

// PredictExpansion recommends when to expand storage
func (p *Predictor) PredictExpansion(poolID string, daysAhead int) (*ExpansionRecommendation, error) {
	p.ml.mu.RLock()
	config, exists := p.ml.poolConfigs[poolID]
	p.ml.mu.RUnlock()
	if !exists {
		return nil, ErrPoolNotFound
	}

	futureDate := time.Now().AddDate(0, 0, daysAhead)
	prediction, err := p.Predict(poolID, MetricCapacity, futureDate)
	if err != nil {
		return nil, err
	}

	// Determine urgency
	urgencyLevel := "low"
	usagePercent := prediction.PredictedValue / config.TotalCapacity * 100

	switch {
	case usagePercent >= config.CriticalThreshold:
		urgencyLevel = "critical"
	case usagePercent >= config.WarningThreshold:
		urgencyLevel = "high"
	case usagePercent >= config.WarningThreshold*0.8:
		urgencyLevel = "medium"
	}

	// Calculate recommended size (20% headroom)
	recommendedSize := prediction.PredictedValue * 1.2

	// Estimate when threshold will be hit
	estimatedDate := futureDate
	if prediction.TrendDirection == "increasing" {
		daysToThreshold := (config.WarningThreshold*config.TotalCapacity/100 - prediction.CurrentValue) / (prediction.PredictedValue - prediction.CurrentValue) * float64(daysAhead)
		estimatedDate = time.Now().AddDate(0, 0, int(daysToThreshold))
	}

	return &ExpansionRecommendation{
		PoolID:          poolID,
		RecommendedSize: recommendedSize,
		UrgencyLevel:    urgencyLevel,
		EstimatedDate:   estimatedDate,
		Reasoning:       p.generateReasoning(prediction, config),
		CostEstimate:    recommendedSize * 0.1, // $0.10 per GB estimate
	}, nil
}

// generateReasoning generates a human-readable reasoning for the recommendation
func (p *Predictor) generateReasoning(prediction *PredictionResult, config PoolConfig) string {
	usagePercent := prediction.PredictedValue / config.TotalCapacity * 100

	if usagePercent >= config.CriticalThreshold {
		return "存储容量即将达到临界阈值，建议立即扩容"
	}
	if usagePercent >= config.WarningThreshold {
		return "存储容量将在近期达到警告阈值，建议规划扩容"
	}
	if prediction.TrendDirection == "increasing" {
		return "存储使用量呈上升趋势，建议提前规划扩容"
	}
	return "存储使用量稳定，暂无需扩容"
}
