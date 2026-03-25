package perf

import (
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PerformanceAnalyzer analyzes performance bottlenecks.
type PerformanceAnalyzer struct {
	slowQueries []SlowQuery
	hotspots    []Hotspot
	bottlenecks []Bottleneck
	mu          sync.RWMutex

	slowQueryThreshold time.Duration
	maxSlowQueries     int

	logger *zap.Logger
}

// SlowQuery represents a slow database query.
type SlowQuery struct {
	Query     string        `json:"query"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Source    string        `json:"source"`
	Params    []interface{} `json:"params,omitempty"`
}

// Hotspot represents a performance hotspot.
type Hotspot struct {
	Name        string        `json:"name"`
	Type        string        `json:"type"` // "function", "endpoint", "query"
	AvgDuration time.Duration `json:"avg_duration"`
	CallCount   int64         `json:"call_count"`
	TotalTime   time.Duration `json:"total_time"`
	Percent     float64       `json:"percent"` // percentage of total time
}

// Bottleneck represents a system bottleneck.
type Bottleneck struct {
	Resource    string    `json:"resource"` // "cpu", "memory", "disk", "network"
	Severity    string    `json:"severity"` // "low", "medium", "high", "critical"
	Description string    `json:"description"`
	Usage       float64   `json:"usage"`
	Threshold   float64   `json:"threshold"`
	Timestamp   time.Time `json:"timestamp"`
}

// ToResponse converts Bottleneck to BottleneckResponse.
func (b *Bottleneck) ToResponse() *BottleneckResponse {
	return &BottleneckResponse{
		Resource:    b.Resource,
		Severity:    b.Severity,
		Description: b.Description,
		Usage:       b.Usage,
		Threshold:   b.Threshold,
		Timestamp:   b.Timestamp,
	}
}

// EndpointStats holds endpoint performance stats.
type EndpointStats struct {
	Path         string        `json:"path"`
	Method       string        `json:"method"`
	AvgDuration  time.Duration `json:"avg_duration"`
	MinDuration  time.Duration `json:"min_duration"`
	MaxDuration  time.Duration `json:"max_duration"`
	P50          time.Duration `json:"p50"`
	P95          time.Duration `json:"p95"`
	P99          time.Duration `json:"p99"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
}

// FunctionStats holds function performance stats.
type FunctionStats struct {
	Name        string        `json:"name"`
	Package     string        `json:"package"`
	AvgDuration time.Duration `json:"avg_duration"`
	CallCount   int64         `json:"call_count"`
	TotalTime   time.Duration `json:"total_time"`
}

// NewPerformanceAnalyzer creates a new performance analyzer.
func NewPerformanceAnalyzer(slowQueryThreshold time.Duration, maxSlowQueries int, logger *zap.Logger) *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		slowQueries:        make([]SlowQuery, 0),
		hotspots:           make([]Hotspot, 0),
		bottlenecks:        make([]Bottleneck, 0),
		slowQueryThreshold: slowQueryThreshold,
		maxSlowQueries:     maxSlowQueries,
		logger:             logger,
	}
}

// RecordSlowQuery records a slow query.
func (a *PerformanceAnalyzer) RecordSlowQuery(query string, duration time.Duration, source string, params ...interface{}) {
	if duration < a.slowQueryThreshold {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	sq := SlowQuery{
		Query:     query,
		Duration:  duration,
		Timestamp: time.Now(),
		Source:    source,
		Params:    params,
	}

	a.slowQueries = append(a.slowQueries, sq)

	// Trim if exceeds max
	if len(a.slowQueries) > a.maxSlowQueries {
		a.slowQueries = a.slowQueries[len(a.slowQueries)-a.maxSlowQueries:]
	}

	if a.logger != nil {
		a.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration),
			zap.String("source", source))
	}
}

// RecordEndpoint records endpoint performance.
func (a *PerformanceAnalyzer) RecordEndpoint(path, method string, duration time.Duration, isError bool) {
	// This would be called from HTTP middleware
	// Implementation depends on your HTTP framework
}

// AnalyzeHotspots analyzes performance hotspots.
func (a *PerformanceAnalyzer) AnalyzeHotspots() []Hotspot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Group slow queries by pattern
	queryStats := make(map[string]*Hotspot)
	var totalTime time.Duration

	for _, sq := range a.slowQueries {
		// Normalize query (remove specific values)
		normalized := a.normalizeQuery(sq.Query)

		if _, ok := queryStats[normalized]; !ok {
			queryStats[normalized] = &Hotspot{
				Name: normalized,
				Type: "query",
			}
		}

		stats := queryStats[normalized]
		stats.CallCount++
		stats.TotalTime += sq.Duration
		totalTime += sq.Duration
	}

	// Calculate averages and percentages
	hotspots := make([]Hotspot, 0, len(queryStats))
	for _, stats := range queryStats {
		if stats.CallCount > 0 {
			stats.AvgDuration = stats.TotalTime / time.Duration(stats.CallCount)
			if totalTime > 0 {
				stats.Percent = float64(stats.TotalTime) / float64(totalTime) * 100
			}
			hotspots = append(hotspots, *stats)
		}
	}

	// Sort by total time
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].TotalTime > hotspots[j].TotalTime
	})

	return hotspots
}

// DetectBottlenecks detects system bottlenecks.
func (a *PerformanceAnalyzer) DetectBottlenecks(cpu, mem float64, diskIO, netIO uint64) []Bottleneck {
	a.mu.Lock()
	defer a.mu.Unlock()

	var bottlenecks []Bottleneck
	now := time.Now()

	// CPU bottleneck
	if cpu > 90 {
		bottlenecks = append(bottlenecks, Bottleneck{
			Resource:    "cpu",
			Severity:    "critical",
			Description: "CPU usage critically high",
			Usage:       cpu,
			Threshold:   90,
			Timestamp:   now,
		})
	} else if cpu > 75 {
		bottlenecks = append(bottlenecks, Bottleneck{
			Resource:    "cpu",
			Severity:    "high",
			Description: "CPU usage high",
			Usage:       cpu,
			Threshold:   75,
			Timestamp:   now,
		})
	}

	// Memory bottleneck
	if mem > 90 {
		bottlenecks = append(bottlenecks, Bottleneck{
			Resource:    "memory",
			Severity:    "critical",
			Description: "Memory usage critically high",
			Usage:       mem,
			Threshold:   90,
			Timestamp:   now,
		})
	} else if mem > 80 {
		bottlenecks = append(bottlenecks, Bottleneck{
			Resource:    "memory",
			Severity:    "high",
			Description: "Memory usage high",
			Usage:       mem,
			Threshold:   80,
			Timestamp:   now,
		})
	}

	a.bottlenecks = append(a.bottlenecks, bottlenecks...)

	// Trim old bottlenecks
	if len(a.bottlenecks) > 100 {
		a.bottlenecks = a.bottlenecks[len(a.bottlenecks)-100:]
	}

	return bottlenecks
}

// GetSlowQueries returns recorded slow queries.
func (a *PerformanceAnalyzer) GetSlowQueries() []SlowQuery {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]SlowQuery, len(a.slowQueries))
	copy(result, a.slowQueries)
	return result
}

// GetBottlenecks returns detected bottlenecks.
func (a *PerformanceAnalyzer) GetBottlenecks() []Bottleneck {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]Bottleneck, len(a.bottlenecks))
	copy(result, a.bottlenecks)
	return result
}

// GetSummary returns performance summary.
func (a *PerformanceAnalyzer) GetSummary() PerformanceSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := PerformanceSummary{
		TotalSlowQueries: len(a.slowQueries),
		TotalBottlenecks: len(a.bottlenecks),
	}

	if len(a.slowQueries) > 0 {
		var totalDuration time.Duration
		for _, sq := range a.slowQueries {
			totalDuration += sq.Duration
		}
		summary.AvgSlowQueryDuration = totalDuration / time.Duration(len(a.slowQueries))
	}

	return summary
}

// PerformanceSummary holds performance summary.
type PerformanceSummary struct {
	TotalSlowQueries     int           `json:"total_slow_queries"`
	AvgSlowQueryDuration time.Duration `json:"avg_slow_query_duration"`
	TotalBottlenecks     int           `json:"total_bottlenecks"`
	CriticalBottlenecks  int           `json:"critical_bottlenecks"`
}

// ClearSlowQueries clears slow query log.
func (a *PerformanceAnalyzer) ClearSlowQueries() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.slowQueries = make([]SlowQuery, 0)
}

// ClearBottlenecks clears bottleneck history.
func (a *PerformanceAnalyzer) ClearBottlenecks() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bottlenecks = make([]Bottleneck, 0)
}

// normalizeQuery normalizes a SQL query by removing specific values.
func (a *PerformanceAnalyzer) normalizeQuery(query string) string {
	// Simple normalization - in production, use proper SQL parser
	normalized := query

	// Replace string literals
	// This is a simplified version
	return normalized
}

// CalculatePercentile calculates percentile from durations.
func CalculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	index := int(float64(len(durations)) * percentile / 100)
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}

// Timer provides easy timing for code blocks.
type Timer struct {
	start    time.Time
	name     string
	callback func(time.Duration)
}

// NewTimer creates a new timer.
func NewTimer(name string, callback func(time.Duration)) *Timer {
	return &Timer{
		start:    time.Now(),
		name:     name,
		callback: callback,
	}
}

// Stop stops the timer and calls callback.
func (t *Timer) Stop() time.Duration {
	duration := time.Since(t.start)
	if t.callback != nil {
		t.callback(duration)
	}
	return duration
}

// TimeFunc times a function execution.
func TimeFunc(name string, fn func()) time.Duration {
	start := time.Now()
	fn()
	duration := time.Since(start)
	return duration
}

// TimeFuncWithResult times a function with error result.
func TimeFuncWithResult(name string, fn func() error) (time.Duration, error) {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	return duration, err
}
