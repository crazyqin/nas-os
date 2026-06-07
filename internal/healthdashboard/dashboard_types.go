// Package healthdashboard provides a unified health monitoring dashboard
// with configurable panels, real-time metrics collection, and alerting.
package healthdashboard

import (
	"sync"
	"time"
)

// PanelType represents the type of monitoring panel.
type PanelType string

const (
	PanelCPU     PanelType = "cpu"
	PanelMemory  PanelType = "memory"
	PanelDisk    PanelType = "disk"
	PanelNetwork PanelType = "network"
	PanelTemp    PanelType = "temperature"
)

// PanelPosition defines the position of a panel in the dashboard grid.
type PanelPosition struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// PanelSize defines the size of a panel in the dashboard grid.
type PanelSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Panel represents a configurable monitoring panel in the dashboard.
type Panel struct {
	ID              string            `json:"id"`
	Type            PanelType         `json:"type"`
	Title           string            `json:"title"`
	Position        PanelPosition     `json:"position"`
	Size            PanelSize         `json:"size"`
	RefreshInterval time.Duration     `json:"refresh_interval"`
	Visible         bool              `json:"visible"`
	Config          map[string]string `json:"config,omitempty"`
}

// DashboardConfig represents the configuration for the health dashboard.
type DashboardConfig struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Panels           []*Panel         `json:"panels"`
	RefreshInterval  time.Duration    `json:"refresh_interval"`
	RetentionDays    int              `json:"retention_days"`
	AlertEnabled     bool             `json:"alert_enabled"`
	HealthThresholds HealthThresholds `json:"health_thresholds"`
}

// HealthThresholds defines threshold values for health scoring.
type HealthThresholds struct {
	CPUWarning   float64 `json:"cpu_warning"`
	CPUCritical  float64 `json:"cpu_critical"`
	MemWarning   float64 `json:"mem_warning"`
	MemCritical  float64 `json:"mem_critical"`
	DiskWarning  float64 `json:"disk_warning"`
	DiskCritical float64 `json:"disk_critical"`
	NetWarning   float64 `json:"net_warning"`
	NetCritical  float64 `json:"net_critical"`
	TempWarning  float64 `json:"temp_warning"`
	TempCritical float64 `json:"temp_critical"`
}

// MetricPoint represents a single metric data point with tags.
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// AlertOperator represents comparison operators for alert rules.
type AlertOperator string

const (
	OpGreaterThan    AlertOperator = "gt"
	OpLessThan       AlertOperator = "lt"
	OpEqual          AlertOperator = "eq"
	OpGreaterOrEqual AlertOperator = "gte"
	OpLessOrEqual    AlertOperator = "lte"
)

// AlertSeverity represents the severity level of an alert.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertRule defines a configurable alert rule for metrics.
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	MetricName  string        `json:"metric_name"`
	Operator    AlertOperator `json:"operator"`
	Threshold   float64       `json:"threshold"`
	Severity    AlertSeverity `json:"severity"`
	Enabled     bool          `json:"enabled"`
	Duration    time.Duration `json:"duration"`
	Description string        `json:"description"`
}

// HealthScoreDetail represents the health score for a specific component.
type HealthScoreDetail struct {
	Score     int       `json:"score"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DashboardHealthScore represents the overall system health score.
type DashboardHealthScore struct {
	Overall   int               `json:"overall"`
	CPU       HealthScoreDetail `json:"cpu"`
	Memory    HealthScoreDetail `json:"memory"`
	Disk      HealthScoreDetail `json:"disk"`
	Network   HealthScoreDetail `json:"network"`
	Timestamp time.Time         `json:"timestamp"`
}

// MetricStore stores metric data points with thread-safe access.
type MetricStore struct {
	mu        sync.RWMutex
	metrics   map[string][]*MetricPoint
	maxPoints int
}

// NewMetricStore creates a new metric store with the specified max points per metric.
func NewMetricStore(maxPoints int) *MetricStore {
	return &MetricStore{
		metrics:   make(map[string][]*MetricPoint),
		maxPoints: maxPoints,
	}
}

// Add adds a metric point to the store.
func (ms *MetricStore) Add(name string, point *MetricPoint) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.metrics[name] = append(ms.metrics[name], point)
	if len(ms.metrics[name]) > ms.maxPoints {
		ms.metrics[name] = ms.metrics[name][len(ms.metrics[name])-ms.maxPoints:]
	}
}

// Get returns metric points for the given name.
func (ms *MetricStore) Get(name string) []*MetricPoint {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	points, ok := ms.metrics[name]
	if !ok {
		return nil
	}
	result := make([]*MetricPoint, len(points))
	copy(result, points)
	return result
}

// GetRange returns metric points within the given time range.
func (ms *MetricStore) GetRange(name string, start, end time.Time) []*MetricPoint {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	points, ok := ms.metrics[name]
	if !ok {
		return nil
	}

	var result []*MetricPoint
	for _, p := range points {
		if p.Timestamp.After(start) && p.Timestamp.Before(end) {
			result = append(result, p)
		}
	}
	return result
}

// Clear removes all metric data.
func (ms *MetricStore) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.metrics = make(map[string][]*MetricPoint)
}

// MetricNames returns all available metric names.
func (ms *MetricStore) MetricNames() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	names := make([]string, 0, len(ms.metrics))
	for name := range ms.metrics {
		names = append(names, name)
	}
	return names
}
