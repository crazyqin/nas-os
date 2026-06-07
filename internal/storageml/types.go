package storageml

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInsufficientData = errors.New("insufficient data points for prediction")
	ErrPoolNotFound     = errors.New("storage pool not found")
	ErrInvalidMetric    = errors.New("invalid metric type")
)

// MetricType defines the type of storage metric
type MetricType string

const (
	MetricCapacity   MetricType = "capacity"
	MetricIOPS       MetricType = "iops"
	MetricThroughput MetricType = "throughput"
	MetricLatency    MetricType = "latency"
)

// DataPoint represents a single metric measurement
type DataPoint struct {
	Timestamp time.Time  `json:"timestamp"`
	Value     float64    `json:"value"`
	Type      MetricType `json:"type"`
	PoolID    string     `json:"pool_id"`
}

// PredictionResult represents a storage prediction
type PredictionResult struct {
	PoolID         string     `json:"pool_id"`
	MetricType     MetricType `json:"metric_type"`
	CurrentValue   float64    `json:"current_value"`
	PredictedValue float64    `json:"predicted_value"`
	PredictedDate  time.Time  `json:"predicted_date"`
	Confidence     float64    `json:"confidence"`
	TrendDirection string     `json:"trend_direction"` // "increasing", "decreasing", "stable"
	SeasonalFactor float64    `json:"seasonal_factor"`
}

// ExpansionRecommendation represents a storage expansion recommendation
type ExpansionRecommendation struct {
	PoolID          string    `json:"pool_id"`
	RecommendedSize float64   `json:"recommended_size_gb"`
	UrgencyLevel    string    `json:"urgency_level"` // "low", "medium", "high", "critical"
	EstimatedDate   time.Time `json:"estimated_date"`
	Reasoning       string    `json:"reasoning"`
	CostEstimate    float64   `json:"cost_estimate"`
}

// PoolConfig represents configuration for a storage pool
type PoolConfig struct {
	PoolID            string  `json:"pool_id"`
	Name              string  `json:"name"`
	TotalCapacity     float64 `json:"total_capacity_gb"`
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
}

// LinearRegression represents a simple linear regression model
type LinearRegression struct {
	Slope     float64
	Intercept float64
	R2        float64
}

// SeasonalComponent represents seasonal patterns
type SeasonalComponent struct {
	Period    time.Duration
	Amplitude float64
	Phase     float64
}

// StorageML manages the storage ML prediction system

type StorageML struct {
	mu            sync.RWMutex
	dataPoints    map[string][]DataPoint // poolID -> data points
	poolConfigs   map[string]PoolConfig
	models        map[string]*LinearRegression
	seasonal      map[string]*SeasonalComponent
	maxDataPoints int
	retentionDays int
}

// NewStorageML creates a new StorageML instance
func NewStorageML() *StorageML {
	return &StorageML{
		dataPoints:    make(map[string][]DataPoint),
		poolConfigs:   make(map[string]PoolConfig),
		models:        make(map[string]*LinearRegression),
		seasonal:      make(map[string]*SeasonalComponent),
		maxDataPoints: 10000,
		retentionDays: 365,
	}
}

// AddDataPoint adds a data point
func (ml *StorageML) AddDataPoint(dp DataPoint) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.dataPoints[dp.PoolID] = append(ml.dataPoints[dp.PoolID], dp)
	if len(ml.dataPoints[dp.PoolID]) > ml.maxDataPoints {
		ml.dataPoints[dp.PoolID] = ml.dataPoints[dp.PoolID][len(ml.dataPoints[dp.PoolID])-ml.maxDataPoints:]
	}
}

// GetDataPoints returns data points for a pool
func (ml *StorageML) GetDataPoints(poolID string) []DataPoint {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	points := ml.dataPoints[poolID]
	result := make([]DataPoint, len(points))
	copy(result, points)
	return result
}

// RegisterPool registers a storage pool
func (ml *StorageML) RegisterPool(config PoolConfig) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.poolConfigs[config.PoolID] = config
}

// GetPoolConfig returns pool config
func (ml *StorageML) GetPoolConfig(poolID string) (PoolConfig, bool) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	config, exists := ml.poolConfigs[poolID]
	return config, exists
}
