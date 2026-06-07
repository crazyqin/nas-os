package healthscore

import (
	"sync"
	"time"
)

// HealthScore manages the health scoring system
type HealthScore struct {
	mu            sync.RWMutex
	weights       map[ComponentType]float64
	collectors    map[ComponentType]CollectorFunc
	history       []ScoreHistory
	maxHistory    int
	currentReport *HealthReport
	calculator    *ScoreCalculator
	analyzer      *Analyzer
}

// NewHealthScoreManager creates a new HealthScore manager
func NewHealthScoreManager() *HealthScore {
	hs := &HealthScore{
		weights:    make(map[ComponentType]float64),
		collectors: make(map[ComponentType]CollectorFunc),
		history:    make([]ScoreHistory, 0),
		maxHistory: 1000,
	}
	hs.calculator = NewScoreCalculator(hs)
	hs.analyzer = NewAnalyzer(hs)

	// Register default collectors
	dc := NewDefaultCollectors(hs)
	dc.RegisterDefaultCollectors()

	return hs
}

// RegisterCollector registers a collector for a component type
func (hs *HealthScore) RegisterCollector(compType ComponentType, collector CollectorFunc) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.collectors[compType] = collector
}

// SetWeights sets custom weights for components
func (hs *HealthScore) SetWeights(weights map[ComponentType]float64) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	for k, v := range weights {
		hs.weights[k] = v
	}
}

// GetCalculator returns the score calculator
func (hs *HealthScore) GetCalculator() *ScoreCalculator {
	return hs.calculator
}

// GetAnalyzer returns the analyzer
func (hs *HealthScore) GetAnalyzer() *Analyzer {
	return hs.analyzer
}

// GenerateReport generates a complete health report
func (hs *HealthScore) GenerateReport() (*HealthReport, error) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	var components []ComponentScore

	// Collect data from all collectors
	for compType, collector := range hs.collectors {
		score, err := collector()
		if err != nil {
			// Log error and continue
			components = append(components, ComponentScore{
				Type:    compType,
				Score:   0,
				Status:  StatusCritical,
				Message: "数据收集失败",
			})
			continue
		}
		components = append(components, *score)
	}

	// Calculate overall score
	overallScore := hs.calculator.CalculateOverallScore(components)
	overallStatus := hs.calculator.DetermineStatus(overallScore)

	// Generate recommendations
	recommendations := hs.calculator.GenerateRecommendations(components)

	// Determine trend
	trend := hs.calculator.DetermineTrend(hs.history)

	report := &HealthReport{
		OverallScore:    overallScore,
		OverallStatus:   overallStatus,
		Components:      components,
		Recommendations: recommendations,
		Trend:           trend,
		GeneratedAt:     time.Now(),
	}

	// Store current report
	hs.currentReport = report

	// Add to history
	hs.history = append(hs.history, ScoreHistory{
		Timestamp: time.Now(),
		Score:     overallScore,
		Status:    overallStatus,
		ComponentScores: func() map[ComponentType]float64 {
			scores := make(map[ComponentType]float64)
			for _, c := range components {
				scores[c.Type] = c.Score
			}
			return scores
		}(),
	})

	// Trim history if needed
	if len(hs.history) > hs.maxHistory {
		hs.history = hs.history[len(hs.history)-hs.maxHistory:]
	}

	return report, nil
}

// GetHistory returns score history
func (hs *HealthScore) GetHistory(limit int) []ScoreHistory {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if limit > len(hs.history) {
		limit = len(hs.history)
	}

	result := make([]ScoreHistory, limit)
	copy(result, hs.history[len(hs.history)-limit:])
	return result
}

// GetCurrentReport returns the current health report
func (hs *HealthScore) GetCurrentReport() *HealthReport {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.currentReport
}
