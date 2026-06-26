package perfmon

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewManager(t *testing.T) {
	cfg := &Config{Enabled: true, Interval: 1 * time.Second, MaxSamples: 100, LatencyWindowSize: 50}
	m := NewManager(cfg, nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config != cfg {
		t.Error("config not set correctly")
	}
}

func TestNewManagerDefaults(t *testing.T) {
	m := NewManager(nil, nil)
	if m == nil {
		t.Fatal("NewManager with nil config returned nil")
	}
	if m.config == nil {
		t.Fatal("default config should not be nil")
	}
	if !m.config.Enabled {
		t.Error("default config should be enabled")
	}
	if m.config.Interval != 5*time.Second {
		t.Errorf("expected default interval 5s, got %v", m.config.Interval)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("default config should be enabled")
	}
	if cfg.Interval != 5*time.Second {
		t.Errorf("expected 5s interval, got %v", cfg.Interval)
	}
	if cfg.MaxSamples != 720 {
		t.Errorf("expected max samples 720, got %d", cfg.MaxSamples)
	}
	if cfg.LatencyWindowSize != 1000 {
		t.Errorf("expected latency window 1000, got %d", cfg.LatencyWindowSize)
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := &Config{Enabled: true, Interval: 1 * time.Second, MaxSamples: 10, LatencyWindowSize: 10}
	m := NewManager(cfg, zap.NewNop())

	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("manager should be running after Start")
	}

	// Double start should be idempotent
	err = m.Start()
	if err != nil {
		t.Fatalf("double Start failed: %v", err)
	}

	err = m.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("manager should be stopped after Stop")
	}

	// Double stop should be idempotent
	err = m.Stop()
	if err != nil {
		t.Fatalf("double Stop failed: %v", err)
	}
}

func TestManagerStartDisabled(t *testing.T) {
	cfg := &Config{Enabled: false, Interval: 1 * time.Second}
	m := NewManager(cfg, nil)

	err := m.Start()
	if err == nil {
		t.Fatal("expected error when starting disabled manager")
	}
}

func TestManagerIsRunning(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	if m.IsRunning() {
		t.Fatal("new manager should not be running")
	}
}

func TestGetIOPSStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetIOPSStats()
	if stats == nil {
		t.Fatal("GetIOPSStats should not return nil")
	}
	if stats.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
}

func TestGetLatencyStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetLatencyStats()
	if stats == nil {
		t.Fatal("GetLatencyStats should not return nil")
	}
	if stats.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
}

func TestGetBandwidthStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetBandwidthStats()
	if stats == nil {
		t.Fatal("GetBandwidthStats should not return nil")
	}
	if stats.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
}

func TestGetCPUDetailStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetCPUDetailStats()
	if stats == nil {
		t.Fatal("GetCPUDetailStats should not return nil")
	}
	if stats.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
}

func TestGetMemoryDetailStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetMemoryDetailStats()
	if stats == nil {
		t.Fatal("GetMemoryDetailStats should not return nil")
	}
	if stats.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
}

func TestGetDiskIOStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetDiskIOStats()
	if stats == nil {
		t.Fatal("GetDiskIOStats should not return nil")
	}
	if len(stats) != 0 {
		t.Errorf("expected empty slice, got %d items", len(stats))
	}
}

func TestGetNetIOStatsEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	stats := m.GetNetIOStats()
	if stats == nil {
		t.Fatal("GetNetIOStats should not return nil")
	}
	if len(stats) != 0 {
		t.Errorf("expected empty slice, got %d items", len(stats))
	}
}

func TestGetSummaryEmpty(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	summary := m.GetSummary()
	if summary == nil {
		t.Fatal("GetSummary should not return nil")
	}
	if summary.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
}

func TestCollectCycle(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          100 * time.Millisecond,
		MaxSamples:        10,
		LatencyWindowSize: 10,
	}
	m := NewManager(cfg, zap.NewNop())

	var collected bool
	var mu sync.Mutex
	m.OnCollect(func(s *PerfSummary) {
		mu.Lock()
		collected = true
		mu.Unlock()
	})

	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for at least one collection cycle
	time.Sleep(300 * time.Millisecond)

	m.Stop()

	mu.Lock()
	wasCollected := collected
	mu.Unlock()

	if !wasCollected {
		t.Error("OnCollect callback was not invoked")
	}

	// After collection, stats should be populated
	iops := m.GetIOPSStats()
	if iops == nil {
		t.Error("IOPS stats should not be nil after collection")
	}
}

func TestLatencySamples(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)

	// Add some read latency samples
	m.AddReadLatencySample(1.5)
	m.AddReadLatencySample(2.0)
	m.AddReadLatencySample(3.5)
	m.AddReadLatencySample(0.8)
	m.AddReadLatencySample(5.0)

	// Add some write latency samples
	m.AddWriteLatencySample(0.5)
	m.AddWriteLatencySample(1.0)
	m.AddWriteLatencySample(2.0)

	stats := m.GetLatencyStats()
	// Latency stats won't be computed until collect() runs,
	// but the samples should be stored internally
	m.mu.RLock()
	readLen := len(m.readLatencySamples)
	writeLen := len(m.writeLatencySamples)
	m.mu.RUnlock()

	if readLen != 5 {
		t.Errorf("expected 5 read samples, got %d", readLen)
	}
	if writeLen != 3 {
		t.Errorf("expected 3 write samples, got %d", writeLen)
	}
	if stats == nil {
		t.Fatal("latency stats should not be nil")
	}
}

func TestLatencyWindowSizeOverflow(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          1 * time.Second,
		MaxSamples:        10,
		LatencyWindowSize: 3,
	}
	m := NewManager(cfg, nil)

	m.AddReadLatencySample(1.0)
	m.AddReadLatencySample(2.0)
	m.AddReadLatencySample(3.0)
	m.AddReadLatencySample(4.0) // Should evict 1.0

	m.mu.RLock()
	samples := make([]float64, len(m.readLatencySamples))
	for i, s := range m.readLatencySamples {
		samples[i] = s.Value
	}
	m.mu.RUnlock()

	if len(samples) != 3 {
		t.Errorf("expected 3 samples (window size), got %d", len(samples))
	}
	// Oldest should have been evicted
	if samples[0] != 2.0 {
		t.Errorf("expected first sample to be 2.0, got %f", samples[0])
	}
}

func TestConcurrentAccess(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          50 * time.Millisecond,
		MaxSamples:        100,
		LatencyWindowSize: 100,
	}
	m := NewManager(cfg, zap.NewNop())

	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				m.GetIOPSStats()
				m.GetLatencyStats()
				m.GetBandwidthStats()
				m.GetDiskIOStats()
				m.GetNetIOStats()
				m.GetCPUDetailStats()
				m.GetMemoryDetailStats()
				m.GetSummary()
				m.IsRunning()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				m.AddReadLatencySample(float64(j))
				m.AddWriteLatencySample(float64(j))
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
	m.Stop()
}

func TestMultipleCallbacks(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          100 * time.Millisecond,
		MaxSamples:        10,
		LatencyWindowSize: 10,
	}
	m := NewManager(cfg, zap.NewNop())

	var count1, count2 int
	var mu sync.Mutex

	m.OnCollect(func(s *PerfSummary) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	m.OnCollect(func(s *PerfSummary) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	m.Start()
	time.Sleep(350 * time.Millisecond)
	m.Stop()

	mu.Lock()
	c1, c2 := count1, count2
	mu.Unlock()

	if c1 == 0 || c2 == 0 {
		t.Errorf("expected both callbacks to be invoked, got count1=%d count2=%d", c1, c2)
	}
}

func TestSummaryContainsAllMetrics(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          100 * time.Millisecond,
		MaxSamples:        10,
		LatencyWindowSize: 10,
	}
	m := NewManager(cfg, zap.NewNop())
	m.Start()
	time.Sleep(250 * time.Millisecond)
	m.Stop()

	summary := m.GetSummary()
	if summary == nil {
		t.Fatal("summary should not be nil")
	}
	if summary.IOPS == nil {
		t.Error("summary.IOPS should not be nil")
	}
	if summary.Latency == nil {
		t.Error("summary.Latency should not be nil")
	}
	if summary.Bandwidth == nil {
		t.Error("summary.Bandwidth should not be nil")
	}
	if summary.CPU == nil {
		t.Error("summary.CPU should not be nil")
	}
	if summary.Memory == nil {
		t.Error("summary.Memory should not be nil")
	}
	if summary.Timestamp == 0 {
		t.Error("summary.Timestamp should be set")
	}
}

func TestCPUDetailStatsValues(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          100 * time.Millisecond,
		MaxSamples:        10,
		LatencyWindowSize: 10,
	}
	m := NewManager(cfg, zap.NewNop())
	m.Start()
	time.Sleep(250 * time.Millisecond)
	m.Stop()

	cpu := m.GetCPUDetailStats()
	total := cpu.UserPercent + cpu.SystemPercent + cpu.IOWaitPercent +
		cpu.IRQPercent + cpu.SoftIRQPercent + cpu.IdlePercent + cpu.StealPercent

	// Total should roughly equal 100% (allowing for floating point)
	if total < 99.0 || total > 101.0 {
		t.Errorf("CPU percentages should sum to ~100, got %.2f", total)
	}
}

func TestMemoryDetailStatsValues(t *testing.T) {
	cfg := &Config{
		Enabled:           true,
		Interval:          100 * time.Millisecond,
		MaxSamples:        10,
		LatencyWindowSize: 10,
	}
	m := NewManager(cfg, zap.NewNop())
	m.Start()
	time.Sleep(250 * time.Millisecond)
	m.Stop()

	mem := m.GetMemoryDetailStats()
	if mem.TotalBytes == 0 {
		t.Error("total memory should not be 0")
	}
	if mem.UsedPercent < 0 || mem.UsedPercent > 100 {
		t.Errorf("used percent should be 0-100, got %f", mem.UsedPercent)
	}
}

// --- Handler Tests ---

func setupTestHandler(t *testing.T) (*Handler, *Manager) {
	t.Helper()
	cfg := &Config{
		Enabled:           true,
		Interval:          50 * time.Millisecond,
		MaxSamples:        10,
		LatencyWindowSize: 10,
	}
	m := NewManager(cfg, zap.NewNop())
	m.Start()
	time.Sleep(200 * time.Millisecond) // Let it collect at least once

	h := NewHandler(m, zap.NewNop())
	return h, m
}

func TestHandlerGetIOPS(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/iops", nil)

	h.GetIOPS(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Error("response body should not be empty")
	}
}

func TestHandlerGetLatency(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/latency", nil)

	h.GetLatency(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetBandwidth(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/bandwidth", nil)

	h.GetBandwidth(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetDiskIO(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/diskio", nil)

	h.GetDiskIO(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetNetIO(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/netio", nil)

	h.GetNetIO(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetCPU(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/cpu", nil)

	h.GetCPU(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetMemory(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/memory", nil)

	h.GetMemory(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetSummary(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/perf/summary", nil)

	h.GetSummary(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Error("summary response body should not be empty")
	}
}

func TestHandlerRegisterRoutes(t *testing.T) {
	h, m := setupTestHandler(t)
	defer m.Stop()

	router := gin.New()
	rg := router.Group("/api/v1")
	h.RegisterRoutes(rg)

	routes := router.Routes()
	expectedPaths := map[string]bool{
		"/api/v1/perf/iops":      false,
		"/api/v1/perf/latency":   false,
		"/api/v1/perf/bandwidth": false,
		"/api/v1/perf/diskio":    false,
		"/api/v1/perf/netio":     false,
		"/api/v1/perf/cpu":       false,
		"/api/v1/perf/memory":    false,
		"/api/v1/perf/summary":   false,
	}

	for _, r := range routes {
		if _, ok := expectedPaths[r.Path]; ok {
			expectedPaths[r.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("route %s not registered", path)
		}
	}
}

func TestHandlerNilLogger(t *testing.T) {
	m := NewManager(DefaultConfig(), nil)
	h := NewHandler(m, nil)
	if h == nil {
		t.Fatal("NewHandler with nil logger should not return nil")
	}
	if h.logger == nil {
		t.Fatal("handler logger should be nop logger, not nil")
	}
}

// --- Utility function tests ---

func TestRound2(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{3.14159, 3.14},
		{2.555, 2.56},
		{1.0, 1.0},
		{0.0, 0.0},
	}
	for _, tt := range tests {
		result := round2(tt.input)
		if result != tt.expected {
			t.Errorf("round2(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestAvg(t *testing.T) {
	if avg(nil) != 0 {
		t.Error("avg of nil should be 0")
	}
	if avg([]float64{}) != 0 {
		t.Error("avg of empty slice should be 0")
	}
	result := avg([]float64{1.0, 2.0, 3.0, 4.0})
	if result != 2.5 {
		t.Errorf("avg([1,2,3,4]) = %f, want 2.5", result)
	}
}

func TestPercentile(t *testing.T) {
	if percentile(nil, 99) != 0 {
		t.Error("percentile of nil should be 0")
	}
	if percentile([]float64{}, 99) != 0 {
		t.Error("percentile of empty slice should be 0")
	}

	vals := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	p99 := percentile(vals, 99)
	if p99 != 10.0 {
		t.Errorf("p99 of [1..10] = %f, want 10.0", p99)
	}

	p50 := percentile(vals, 50)
	if p50 < 1.0 || p50 > 10.0 {
		t.Errorf("p50 of [1..10] = %f, out of range", p50)
	}
}

func TestMaxVal(t *testing.T) {
	if maxVal(nil) != 0 {
		t.Error("max of nil should be 0")
	}
	if maxVal([]float64{}) != 0 {
		t.Error("max of empty slice should be 0")
	}
	result := maxVal([]float64{1.0, 5.0, 3.0, 2.0})
	if result != 5.0 {
		t.Errorf("max([1,5,3,2]) = %f, want 5.0", result)
	}
}
