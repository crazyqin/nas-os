package healthscore

import (
	"time"
)

// Analyzer provides analysis of health scores.
type Analyzer struct {
	hs *HealthScore
}

// NewAnalyzer creates a new analyzer.
func NewAnalyzer(hs *HealthScore) *Analyzer {
	return &Analyzer{hs: hs}
}

// AnalyzeTrend analyzes the trend of health scores over time.
func (a *Analyzer) AnalyzeTrend(duration time.Duration) *TrendAnalysis {
	a.hs.mu.RLock()
	defer a.hs.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var recent []ScoreHistory
	for _, h := range a.hs.history {
		if h.Timestamp.After(cutoff) {
			recent = append(recent, h)
		}
	}

	if len(recent) < 2 {
		return &TrendAnalysis{
			Direction:  "stable",
			DataPoints: len(recent),
		}
	}

	// Calculate trend
	firstHalf := recent[:len(recent)/2]
	secondHalf := recent[len(recent)/2:]

	firstAvg := averageScoreHistory(firstHalf)
	secondAvg := averageScoreHistory(secondHalf)

	direction := "stable"
	changeRate := secondAvg - firstAvg
	if changeRate > 5 {
		direction = "improving"
	} else if changeRate < -5 {
		direction = "declining"
	}

	return &TrendAnalysis{
		Direction:  direction,
		ChangeRate: changeRate,
		FirstAvg:   firstAvg,
		SecondAvg:  secondAvg,
		DataPoints: len(recent),
		StartTime:  recent[0].Timestamp,
		EndTime:    recent[len(recent)-1].Timestamp,
	}
}

// GetWorstComponents returns the components with lowest scores.
func (a *Analyzer) GetWorstComponents(limit int) []ComponentScore {
	a.hs.mu.RLock()
	defer a.hs.mu.RUnlock()

	if a.hs.currentReport == nil {
		return nil
	}

	components := make([]ComponentScore, len(a.hs.currentReport.Components))
	copy(components, a.hs.currentReport.Components)

	// Sort by score (simple bubble sort for small dataset)
	for i := 0; i < len(components)-1; i++ {
		for j := 0; j < len(components)-i-1; j++ {
			if components[j].Score > components[j+1].Score {
				components[j], components[j+1] = components[j+1], components[j]
			}
		}
	}

	if limit > len(components) {
		limit = len(components)
	}
	return components[:limit]
}

// GetScoreDistribution returns the distribution of component scores.
func (a *Analyzer) GetScoreDistribution() map[HealthStatus]int {
	a.hs.mu.RLock()
	defer a.hs.mu.RUnlock()

	distribution := map[HealthStatus]int{
		StatusExcellent: 0,
		StatusGood:      0,
		StatusFair:      0,
		StatusPoor:      0,
		StatusCritical:  0,
	}

	if a.hs.currentReport == nil {
		return distribution
	}

	for _, comp := range a.hs.currentReport.Components {
		distribution[comp.Status]++
	}

	return distribution
}

// TrendAnalysis represents trend analysis results.
type TrendAnalysis struct {
	Direction  string    `json:"direction"`
	ChangeRate float64   `json:"change_rate"`
	FirstAvg   float64   `json:"first_avg"`
	SecondAvg  float64   `json:"second_avg"`
	DataPoints int       `json:"data_points"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
}

// averageScoreHistory calculates average score from history.
func averageScoreHistory(history []ScoreHistory) float64 {
	if len(history) == 0 {
		return 0
	}
	sum := 0.0
	for _, h := range history {
		sum += h.Score
	}
	return sum / float64(len(history))
}
