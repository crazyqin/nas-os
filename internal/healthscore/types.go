package healthscore

import "time"

// ComponentType defines the type of health component
type ComponentType string

const (
	ComponentDisk      ComponentType = "disk"
	ComponentCPU       ComponentType = "cpu"
	ComponentMemory    ComponentType = "memory"
	ComponentNetwork   ComponentType = "network"
	ComponentRAID      ComponentType = "raid"
	ComponentService   ComponentType = "service"
	ComponentTemperature ComponentType = "temperature"
)

// HealthStatus represents the overall health status
type HealthStatus string

const (
	StatusExcellent HealthStatus = "excellent"
	StatusGood      HealthStatus = "good"
	StatusFair      HealthStatus = "fair"
	StatusPoor      HealthStatus = "poor"
	StatusCritical  HealthStatus = "critical"
)

// ComponentScore represents the score of a single component
type ComponentScore struct {
	Type        ComponentType `json:"type"`
	Score       float64       `json:"score"`       // 0-100
	Weight      float64       `json:"weight"`      // 0-1
	Status      HealthStatus  `json:"status"`
	Message     string        `json:"message"`
	Details     interface{}   `json:"details,omitempty"`
	CollectedAt time.Time     `json:"collected_at"`
}

// HealthReport represents a complete health report
type HealthReport struct {
	OverallScore    float64          `json:"overall_score"`
	OverallStatus   HealthStatus     `json:"overall_status"`
	Components      []ComponentScore `json:"components"`
	Recommendations []Recommendation `json:"recommendations"`
	Trend           string           `json:"trend"` // "improving", "stable", "declining"
	GeneratedAt     time.Time        `json:"generated_at"`
}

// Recommendation represents a health improvement recommendation
type Recommendation struct {
	Priority    string `json:"priority"` // "low", "medium", "high", "critical"
	Component   string `json:"component"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

// ScoreHistory represents historical score data
type ScoreHistory struct {
	Timestamp    time.Time `json:"timestamp"`
	Score        float64   `json:"score"`
	Status       HealthStatus `json:"status"`
	ComponentScores map[ComponentType]float64 `json:"component_scores"`
}

// Weights defines default component weights
var DefaultWeights = map[ComponentType]float64{
	ComponentDisk:        0.25,
	ComponentCPU:         0.15,
	ComponentMemory:      0.15,
	ComponentNetwork:     0.10,
	ComponentRAID:        0.20,
	ComponentService:     0.05,
	ComponentTemperature: 0.10,
}

// CollectorFunc is a function that collects health data for a component
type CollectorFunc func() (*ComponentScore, error)
