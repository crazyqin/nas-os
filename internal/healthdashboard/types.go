// Package healthdashboard provides storage health monitoring and alerting.
package healthdashboard

import "time"

// HealthScore represents the overall system health score.
type HealthScore struct {
	Overall   int                    `json:"overall"`
	Storage   int                    `json:"storage"`
	CPU       int                    `json:"cpu"`
	Memory    int                    `json:"memory"`
	Network   int                    `json:"network"`
	UpdatedAt time.Time              `json:"updated_at"`
	Details   map[string]interface{} `json:"details"`
}

// HealthMetric represents a single health metric data point.
type HealthMetric struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Status    string    `json:"status"` // normal, warning, critical
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthAlertRule defines an alerting rule for health metrics.
type HealthAlertRule struct {
	ID        string  `json:"id"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"` // gt, lt, eq
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity"` // info, warning, critical
	Enabled   bool    `json:"enabled"`
}

// TrendDataPoint represents a data point in a trend series.
type TrendDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// TrendSeries represents a time series of metric data.
type TrendSeries struct {
	Metric string            `json:"metric"`
	Unit   string            `json:"unit"`
	Period string            `json:"period"` // 7d, 30d, 90d
	Points []*TrendDataPoint `json:"points"`
}

// AlertEvent represents a triggered alert.
type AlertEvent struct {
	RuleID    string    `json:"rule_id"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// OverviewResponse is the response for the dashboard overview endpoint.
type OverviewResponse struct {
	Score     *HealthScore    `json:"score"`
	Metrics   []*HealthMetric `json:"metrics"`
	Alerts    []*AlertEvent   `json:"alerts"`
	AlertCount int            `json:"alert_count"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// APIResponse is the standard API response wrapper.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
