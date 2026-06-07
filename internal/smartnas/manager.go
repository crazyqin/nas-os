// Package smartnas provides a unified NAS health scoring and recommendation system.
// It aggregates health metrics from all subsystems (storage, network, security, performance)
// into a single composite score with actionable recommendations.
package smartnas

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// SubsystemType identifies a NAS subsystem.
type SubsystemType string

const (
	SubsystemStorage     SubsystemType = "storage"
	SubsystemNetwork     SubsystemType = "network"
	SubsystemSecurity    SubsystemType = "security"
	SubsystemPerformance SubsystemType = "performance"
	SubsystemHardware    SubsystemType = "hardware"
	SubsystemBackup      SubsystemType = "backup"
)

// HealthLevel represents the health status.
type HealthLevel string

const (
	HealthExcellent HealthLevel = "excellent"
	HealthGood      HealthLevel = "good"
	HealthFair      HealthLevel = "fair"
	HealthPoor      HealthLevel = "poor"
	HealthCritical  HealthLevel = "critical"
)

// Severity for recommendations.
type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
)

// SubsystemHealth holds the health data for a single subsystem.
type SubsystemHealth struct {
	Type        SubsystemType `json:"type"`
	Score       float64       `json:"score"` // 0.0 to 100.0
	Level       HealthLevel   `json:"level"`
	Metrics     []Metric      `json:"metrics"`
	LastUpdated time.Time     `json:"last_updated"`
	Message     string        `json:"message,omitempty"`
}

// Metric is a single health measurement.
type Metric struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Threshold float64 `json:"threshold"` // warning threshold
	Status    string  `json:"status"`    // ok, warning, critical
}

// Recommendation is an actionable suggestion to improve NAS health.
type Recommendation struct {
	ID          string        `json:"id"`
	Subsystem   SubsystemType `json:"subsystem"`
	Severity    Severity      `json:"severity"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Action      string        `json:"action"` // suggested action
	CreatedAt   time.Time     `json:"created_at"`
	Dismissed   bool          `json:"dismissed"`
	Manual      bool          `json:"manual,omitempty"` // manually added
}

// NASHealthScore is the overall NAS health assessment.
type NASHealthScore struct {
	Overall     float64                            `json:"overall"` // 0-100
	Level       HealthLevel                        `json:"level"`
	Subsystems  map[SubsystemType]*SubsystemHealth `json:"subsystems"`
	TopIssues   []Recommendation                   `json:"top_issues"`
	LastUpdated time.Time                          `json:"last_updated"`
	Uptime      time.Duration                      `json:"uptime"`
	Trend       string                             `json:"trend"` // improving, stable, declining
}

// Manager orchestrates health scoring across all subsystems.
type Manager struct {
	mu              sync.RWMutex
	subsystems      map[SubsystemType]*SubsystemHealth
	recommendations []Recommendation
	startTime       time.Time
	history         []NASHealthScore
	maxHistory      int
	weights         map[SubsystemType]float64
}

// NewManager creates a new SmartNAS health manager.
func NewManager() *Manager {
	return &Manager{
		subsystems:      make(map[SubsystemType]*SubsystemHealth),
		recommendations: make([]Recommendation, 0, 50),
		startTime:       time.Now(),
		history:         make([]NASHealthScore, 0, 100),
		maxHistory:      100,
		weights: map[SubsystemType]float64{
			SubsystemStorage:     0.30,
			SubsystemNetwork:     0.15,
			SubsystemSecurity:    0.20,
			SubsystemPerformance: 0.15,
			SubsystemHardware:    0.10,
			SubsystemBackup:      0.10,
		},
	}
}

// UpdateSubsystem updates the health data for a specific subsystem.
func (m *Manager) UpdateSubsystem(subsystem SubsystemType, health *SubsystemHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	health.LastUpdated = time.Now()
	m.subsystems[subsystem] = health
	m.evaluateRecommendations(subsystem, health)
}

// GetScore calculates and returns the overall NAS health score.
func (m *Manager) GetScore() *NASHealthScore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	score := &NASHealthScore{
		Subsystems:  make(map[SubsystemType]*SubsystemHealth),
		LastUpdated: time.Now(),
		Uptime:      time.Since(m.startTime),
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for subType, health := range m.subsystems {
		clone := *health
		score.Subsystems[subType] = &clone
		weight := m.weights[subType]
		weightedSum += health.Score * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		score.Overall = math.Round(weightedSum/totalWeight*100) / 100
	}
	score.Level = scoreToLevel(score.Overall)

	// Get top 5 non-dismissed issues
	count := 0
	for _, rec := range m.recommendations {
		if !rec.Dismissed && count < 5 {
			score.TopIssues = append(score.TopIssues, rec)
			count++
		}
	}

	// Determine trend from history
	if len(m.history) >= 3 {
		recent := m.history[len(m.history)-3:]
		if recent[2].Overall > recent[0].Overall+2 {
			score.Trend = "improving"
		} else if recent[2].Overall < recent[0].Overall-2 {
			score.Trend = "declining"
		} else {
			score.Trend = "stable"
		}
	} else {
		score.Trend = "stable"
	}

	return score
}

// GetSubsystem returns health data for a specific subsystem.
func (m *Manager) GetSubsystem(subsystem SubsystemType) (*SubsystemHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.subsystems[subsystem]
	return h, ok
}

// GetRecommendations returns all active recommendations.
func (m *Manager) GetRecommendations(includeDismissed bool) []Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if includeDismissed {
		return m.recommendations
	}
	var active []Recommendation
	for _, r := range m.recommendations {
		if !r.Dismissed {
			active = append(active, r)
		}
	}
	return active
}

// DismissRecommendation marks a recommendation as dismissed.
func (m *Manager) DismissRecommendation(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.recommendations {
		if m.recommendations[i].ID == id {
			m.recommendations[i].Dismissed = true
			return true
		}
	}
	return false
}

// RecordSnapshot saves the current score to history.
func (m *Manager) RecordSnapshot() {
	score := m.GetScore()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, *score)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// GetHistory returns historical health scores.
func (m *Manager) GetHistory(limit int) []NASHealthScore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	start := len(m.history) - limit
	return m.history[start:]
}

// RefreshAll triggers a health check on all subsystems.
// In production, this would query actual system metrics.
func (m *Manager) RefreshAll(ctx context.Context) error {
	m.mu.RLock()
	subs := make([]SubsystemType, 0, len(m.subsystems))
	for k := range m.subsystems {
		subs = append(subs, k)
	}
	m.mu.RUnlock()

	for _, sub := range subs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = sub // In production: call actual subsystem health check
	}

	m.RecordSnapshot()
	return nil
}

// evaluateRecommendations generates recommendations based on health data.
func (m *Manager) evaluateRecommendations(subsystem SubsystemType, health *SubsystemHealth) {
	// Clean old auto-generated recommendations for this subsystem
	var filtered []Recommendation
	for _, r := range m.recommendations {
		if r.Subsystem != subsystem || r.Manual {
			filtered = append(filtered, r)
		}
	}
	m.recommendations = filtered

	for _, metric := range health.Metrics {
		if metric.Status == "critical" {
			m.recommendations = append(m.recommendations, Recommendation{
				ID:          string(subsystem) + "-" + metric.Name + "-critical",
				Subsystem:   subsystem,
				Severity:    SevCritical,
				Title:       metric.Name + " is critically high",
				Description: metric.Name + " is at " + formatFloat(metric.Value) + metric.Unit + ", exceeding threshold of " + formatFloat(metric.Threshold) + metric.Unit,
				Action:      "Immediate attention required for " + metric.Name,
				CreatedAt:   time.Now(),
			})
		} else if metric.Status == "warning" {
			m.recommendations = append(m.recommendations, Recommendation{
				ID:          string(subsystem) + "-" + metric.Name + "-warning",
				Subsystem:   subsystem,
				Severity:    SevWarning,
				Title:       metric.Name + " needs attention",
				Description: metric.Name + " is at " + formatFloat(metric.Value) + metric.Unit + ", approaching threshold of " + formatFloat(metric.Threshold) + metric.Unit,
				Action:      "Monitor " + metric.Name + " and plan corrective action",
				CreatedAt:   time.Now(),
			})
		}
	}
}

func scoreToLevel(score float64) HealthLevel {
	switch {
	case score >= 90:
		return HealthExcellent
	case score >= 75:
		return HealthGood
	case score >= 60:
		return HealthFair
	case score >= 40:
		return HealthPoor
	default:
		return HealthCritical
	}
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
}
