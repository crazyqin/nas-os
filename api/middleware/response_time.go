// Package middleware provides HTTP middleware for the API
package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ResponseTimeConfig configures the response time middleware
type ResponseTimeConfig struct {
	// HeaderName is the header name for response time (default: X-Response-Time)
	HeaderName string
	// Precise whether to use microsecond precision
	Precise bool
	// SkipPaths paths to skip
	SkipPaths []string
}

// DefaultResponseTimeConfig default response time configuration
var DefaultResponseTimeConfig = ResponseTimeConfig{
	HeaderName: "X-Response-Time",
	Precise:    false,
	SkipPaths:  []string{"/health", "/metrics"},
}

// ResponseTimeMiddleware creates a response time middleware
func ResponseTimeMiddleware(config ...ResponseTimeConfig) gin.HandlerFunc {
	cfg := DefaultResponseTimeConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Response-Time"
	}

	skipMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		// Skip certain paths
		if skipMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Set header
		if cfg.Precise {
			c.Header(cfg.HeaderName, duration.String())
		} else {
			// Millisecond precision
			c.Header(cfg.HeaderName, duration.Round(time.Millisecond).String())
		}

		// Store in context for logging
		c.Set("responseTime", duration.Milliseconds())
	}
}

// ResponseTimeStats contains response time statistics
type ResponseTimeStats struct {
	TotalRequests   int64         `json:"totalRequests"`
	AvgResponseTime int64         `json:"avgResponseTimeMs"`
	MinResponseTime int64         `json:"minResponseTimeMs"`
	MaxResponseTime int64         `json:"maxResponseTimeMs"`
	P50ResponseTime int64         `json:"p50ResponseTimeMs"`
	P95ResponseTime int64         `json:"p95ResponseTimeMs"`
	P99ResponseTime int64         `json:"p99ResponseTimeMs"`
	TotalDuration   time.Duration `json:"totalDuration"`
}

// ResponseTimeCollector collects response time statistics
type ResponseTimeCollector struct {
	times   []int64
	mu      sync.RWMutex
	maxSize int
}

var defaultCollector *ResponseTimeCollector

func init() {
	defaultCollector = NewResponseTimeCollector(10000)
}

// NewResponseTimeCollector creates a new response time collector
func NewResponseTimeCollector(maxSize int) *ResponseTimeCollector {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &ResponseTimeCollector{
		times:   make([]int64, 0),
		maxSize: maxSize,
	}
}

// Record records a response time
func (c *ResponseTimeCollector) Record(durationMs int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.times) >= c.maxSize {
		c.times = c.times[1:]
	}
	c.times = append(c.times, durationMs)
}

// GetStats returns current statistics
func (c *ResponseTimeCollector) GetStats() ResponseTimeStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.times) == 0 {
		return ResponseTimeStats{}
	}

	// Calculate stats
	var total int64
	var min, max int64 = c.times[0], c.times[0]
	for _, t := range c.times {
		total += t
		if t < min {
			min = t
		}
		if t > max {
			max = t
		}
	}

	// Sort for percentiles
	sorted := make([]int64, len(c.times))
	copy(sorted, c.times)
	sortSlice(sorted)

	return ResponseTimeStats{
		TotalRequests:   int64(len(c.times)),
		AvgResponseTime: total / int64(len(c.times)),
		MinResponseTime: min,
		MaxResponseTime: max,
		P50ResponseTime: percentile(sorted, 50),
		P95ResponseTime: percentile(sorted, 95),
		P99ResponseTime: percentile(sorted, 99),
		TotalDuration:   time.Duration(total) * time.Millisecond,
	}
}

// Reset resets the collector
func (c *ResponseTimeCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.times = c.times[:0]
}

// sortSlice sorts a slice of int64
func sortSlice(s []int64) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// percentile calculates the percentile value
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	// Use linear interpolation for more accurate percentile
	index := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	weight := index - float64(lower)
	return int64(float64(sorted[lower])*(1-weight) + float64(sorted[upper])*weight)
}

// ResponseTimeWithCollector creates middleware that records to a collector
func ResponseTimeWithCollector(collector *ResponseTimeCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		collector.Record(duration.Milliseconds())
		c.Set("responseTime", duration.Milliseconds())
		c.Header("X-Response-Time", duration.Round(time.Millisecond).String())
	}
}

// GetDefaultCollector returns the default response time collector
func GetDefaultCollector() *ResponseTimeCollector {
	return defaultCollector
}

// GetResponseTimeStats returns current response time statistics
func GetResponseTimeStats() ResponseTimeStats {
	return defaultCollector.GetStats()
}

// ResetResponseTimeStats resets the response time statistics
func ResetResponseTimeStats() {
	defaultCollector.Reset()
}
