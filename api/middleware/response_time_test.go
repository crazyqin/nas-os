// Package middleware provides tests for response time middleware
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestResponseTimeMiddleware tests the response time middleware.
func TestResponseTimeMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(ResponseTimeMiddleware())

	router.GET("/test", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Check header exists
	header := w.Header().Get("X-Response-Time")
	if header == "" {
		t.Error("Expected X-Response-Time header to be set")
	}

	// Check status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestResponseTimeMiddlewareSkip tests skip paths.
func TestResponseTimeMiddlewareSkip(t *testing.T) {
	config := ResponseTimeConfig{
		HeaderName: "X-Response-Time",
		SkipPaths:  []string{"/health"},
	}

	router := gin.New()
	router.Use(ResponseTimeMiddleware(config))

	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test skipped path
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	header := w.Header().Get("X-Response-Time")
	if header != "" {
		t.Error("Expected no X-Response-Time header for skipped path")
	}

	// Test non-skipped path
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	header = w.Header().Get("X-Response-Time")
	if header == "" {
		t.Error("Expected X-Response-Time header for non-skipped path")
	}
}

// TestResponseTimePrecise tests precise mode.
func TestResponseTimePrecise(t *testing.T) {
	config := ResponseTimeConfig{
		HeaderName: "X-Response-Time",
		Precise:    true,
	}

	router := gin.New()
	router.Use(ResponseTimeMiddleware(config))

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	header := w.Header().Get("X-Response-Time")
	if header == "" {
		t.Error("Expected X-Response-Time header")
	}
	// Precise mode should include µs or ns
}

// TestResponseTimeCollector tests the collector.
func TestResponseTimeCollector(t *testing.T) {
	collector := NewResponseTimeCollector(100)

	// Record some times
	collector.Record(10)
	collector.Record(20)
	collector.Record(30)
	collector.Record(40)
	collector.Record(50)

	stats := collector.GetStats()

	if stats.TotalRequests != 5 {
		t.Errorf("Expected 5 total requests, got %d", stats.TotalRequests)
	}

	if stats.MinResponseTime != 10 {
		t.Errorf("Expected min 10, got %d", stats.MinResponseTime)
	}

	if stats.MaxResponseTime != 50 {
		t.Errorf("Expected max 50, got %d", stats.MaxResponseTime)
	}

	// Average should be 30
	if stats.AvgResponseTime != 30 {
		t.Errorf("Expected avg 30, got %d", stats.AvgResponseTime)
	}
}

// TestResponseTimeCollectorReset tests reset.
func TestResponseTimeCollectorReset(t *testing.T) {
	collector := NewResponseTimeCollector(100)

	collector.Record(10)
	collector.Record(20)

	stats := collector.GetStats()
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 requests, got %d", stats.TotalRequests)
	}

	collector.Reset()

	stats = collector.GetStats()
	if stats.TotalRequests != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", stats.TotalRequests)
	}
}

// TestResponseTimeWithCollector tests middleware with collector.
func TestResponseTimeWithCollector(t *testing.T) {
	collector := NewResponseTimeCollector(100)

	router := gin.New()
	router.Use(ResponseTimeWithCollector(collector))

	router.GET("/test", func(c *gin.Context) {
		time.Sleep(5 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	stats := collector.GetStats()
	if stats.TotalRequests != 5 {
		t.Errorf("Expected 5 requests, got %d", stats.TotalRequests)
	}

	// Average should be at least 5ms
	if stats.AvgResponseTime < 5 {
		t.Errorf("Expected avg >= 5ms, got %d", stats.AvgResponseTime)
	}
}

// TestGetDefaultCollector tests the default collector.
func TestGetDefaultCollector(t *testing.T) {
	collector := GetDefaultCollector()
	if collector == nil {
		t.Error("Expected non-nil default collector")
	}

	// Reset for clean state
	ResetResponseTimeStats()

	collector.Record(10)
	stats := GetResponseTimeStats()

	if stats.TotalRequests != 1 {
		t.Errorf("Expected 1 request, got %d", stats.TotalRequests)
	}

	// Clean up
	ResetResponseTimeStats()
}

// TestPercentile tests percentile calculation.
func TestPercentile(t *testing.T) {
	tests := []struct {
		sorted      []int64
		p           int
		minExpected int64
		maxExpected int64
	}{
		{[]int64{10, 20, 30, 40, 50}, 50, 28, 32},           // P50 should be around 30
		{[]int64{10, 20, 30, 40, 50}, 95, 48, 50},           // P95 should be around 50
		{[]int64{10, 20, 30, 40, 50}, 99, 49, 50},           // P99 should be around 50
		{[]int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 90, 8, 10}, // P90 should be around 9-10
		{[]int64{}, 50, 0, 0},                               // Empty slice
	}

	for _, tt := range tests {
		result := percentile(tt.sorted, tt.p)
		if result < tt.minExpected || result > tt.maxExpected {
			t.Errorf("percentile(%v, %d): expected between %d and %d, got %d", tt.sorted, tt.p, tt.minExpected, tt.maxExpected, result)
		}
	}
}

// TestResponseTimeConfigDefaults tests config defaults.
func TestResponseTimeConfigDefaults(t *testing.T) {
	config := DefaultResponseTimeConfig

	if config.HeaderName != "X-Response-Time" {
		t.Errorf("Expected default header name, got %s", config.HeaderName)
	}

	if config.Precise != false {
		t.Error("Expected Precise to be false by default")
	}

	if len(config.SkipPaths) == 0 {
		t.Error("Expected some default skip paths")
	}
}

// BenchmarkResponseTimeMiddleware benchmarks the middleware.
func BenchmarkResponseTimeMiddleware(b *testing.B) {
	router := gin.New()
	router.Use(ResponseTimeMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkResponseTimeCollector benchmarks the collector.
func BenchmarkResponseTimeCollector(b *testing.B) {
	collector := NewResponseTimeCollector(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.Record(int64(i % 100))
	}
}

// BenchmarkResponseTimeGetStats benchmarks GetStats.
func BenchmarkResponseTimeGetStats(b *testing.B) {
	collector := NewResponseTimeCollector(10000)
	for i := 0; i < 10000; i++ {
		collector.Record(int64(i % 100))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.GetStats()
	}
}
