package smartcacheprefetch

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestConfig() PrefetchConfig {
	return PrefetchConfig{
		MaxCacheSize:       1024 * 1024 * 100, // 100MB
		PrefetchWindow:     10 * time.Second,
		MaxQueueSize:       64,
		MLModelEnabled:     true,
		AggressivePrefetch: false,
		LayerConfigs: []LayerConfig{
			{ID: "l1-nvme", Type: CacheLayerNVMe, Capacity: 1024 * 1024 * 10, Policy: EvictionMLAware},
			{ID: "l2-ssd", Type: CacheLayerSSD, Capacity: 1024 * 1024 * 50, Policy: EvictionARC},
			{ID: "l3-hdd", Type: CacheLayerHDD, Capacity: 1024 * 1024 * 40, Policy: EvictionLFU},
		},
	}
}

func TestNewPrefetchEngine(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	if engine == nil {
		t.Fatal("NewPrefetchEngine returned nil")
	}
	if engine.running {
		t.Error("Engine should not be running after creation")
	}
	if len(engine.cacheLayers) != 3 {
		t.Errorf("Expected 3 cache layers, got %d", len(engine.cacheLayers))
	}
	if engine.config == nil {
		t.Error("Config should not be nil")
	}
	if engine.mlModel == nil {
		t.Error("ML model should not be nil")
	}
}

func TestNewPrefetchEngineDefaultLogger(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, nil)

	if engine == nil {
		t.Fatal("NewPrefetchEngine with nil logger returned nil")
	}
	if engine.logger == nil {
		t.Error("Logger should default to slog.Default()")
	}
}

func TestStartStop(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	// Start engine
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !engine.running {
		t.Error("Engine should be running after Start")
	}

	// Try starting again
	err = engine.Start()
	if err != ErrEngineAlreadyRunning {
		t.Errorf("Expected ErrEngineAlreadyRunning, got %v", err)
	}

	// Stop engine
	err = engine.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if engine.running {
		t.Error("Engine should not be running after Stop")
	}

	// Try stopping again
	err = engine.Stop()
	if err != ErrEngineNotRunning {
		t.Errorf("Expected ErrEngineNotRunning, got %v", err)
	}
}

func TestRecordAccess(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Record access when not running test
	engine.Stop()

	err := engine.RecordAccess("file1", "/data/file1.txt", 1024)
	if err != ErrEngineNotRunning {
		t.Errorf("Expected ErrEngineNotRunning, got %v", err)
	}

	// Restart and test recording
	engine.Start()

	err = engine.RecordAccess("file1", "/data/file1.txt", 1024)
	if err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}

	// Verify access was recorded
	engine.mu.RLock()
	pattern, exists := engine.accessHistory["file1"]
	engine.mu.RUnlock()

	if !exists {
		t.Fatal("Access pattern not recorded")
	}
	if len(pattern.AccessTimes) != 1 {
		t.Errorf("Expected 1 access time, got %d", len(pattern.AccessTimes))
	}
	if len(pattern.ReadSizes) != 1 {
		t.Errorf("Expected 1 read size, got %d", len(pattern.ReadSizes))
	}

	// Record multiple accesses
	for i := 0; i < 5; i++ {
		engine.RecordAccess("file1", "/data/file1.txt", int64(1024*(i+1)))
	}

	engine.mu.RLock()
	pattern = engine.accessHistory["file1"]
	engine.mu.RUnlock()

	if len(pattern.AccessTimes) != 6 {
		t.Errorf("Expected 6 access times, got %d", len(pattern.AccessTimes))
	}
}

func TestRecordAccessCachedFile(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Add a file to cache
	engine.mu.Lock()
	engine.cacheLayers["l1-nvme"].Entries["file1"] = &CacheEntry{
		Key:         "file1",
		Path:        "/data/file1.txt",
		Size:        1024,
		Layer:       "l1-nvme",
		AccessCount: 1,
		LastAccess:  time.Now(),
		CreatedAt:   time.Now(),
	}
	engine.mu.Unlock()

	// Record access - should be a cache hit
	engine.RecordAccess("file1", "/data/file1.txt", 1024)

	metrics := engine.GetMetrics()
	if metrics.SuccessfulHits != 1 {
		t.Errorf("Expected 1 successful hit, got %d", metrics.SuccessfulHits)
	}
}

func TestPredict(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Not running test
	engine.Stop()
	_, err := engine.Predict(10)
	if err != ErrEngineNotRunning {
		t.Errorf("Expected ErrEngineNotRunning, got %v", err)
	}
	engine.Start()

	// Add some access patterns
	for i := 0; i < 10; i++ {
		engine.RecordAccess("file1", "/data/file1.txt", 1024)
		time.Sleep(10 * time.Millisecond)
	}

	for i := 0; i < 5; i++ {
		engine.RecordAccess("file2", "/data/file2.txt", 2048)
		time.Sleep(10 * time.Millisecond)
	}

	// Generate predictions
	predictions, err := engine.Predict(10)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	// Should have predictions for both files
	if len(predictions) == 0 {
		t.Error("Expected at least 1 prediction")
	}

	// Verify predictions are sorted by probability
	for i := 1; i < len(predictions); i++ {
		if predictions[i].Probability > predictions[i-1].Probability {
			t.Error("Predictions should be sorted by probability descending")
		}
	}

	// Verify prediction fields
	for _, pred := range predictions {
		if pred.FileID == "" {
			t.Error("Prediction FileID should not be empty")
		}
		if pred.Probability < 0 || pred.Probability > 1 {
			t.Errorf("Probability out of range: %f", pred.Probability)
		}
		if pred.Confidence < 0 || pred.Confidence > 1 {
			t.Errorf("Confidence out of range: %f", pred.Confidence)
		}
	}
}

func TestPredictWithLimit(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Add many files
	for i := 0; i < 20; i++ {
		fileID := "file" + string(rune('A'+i))
		engine.RecordAccess(fileID, "/data/"+fileID+".txt", 1024)
	}

	predictions, err := engine.Predict(5)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(predictions) > 5 {
		t.Errorf("Expected at most 5 predictions, got %d", len(predictions))
	}
}

func TestPredictDefaultLimit(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	engine.RecordAccess("file1", "/data/file1.txt", 1024)

	// Use default limit (topN <= 0)
	predictions, _ := engine.Predict(0)
	if len(predictions) > 10 {
		t.Errorf("Expected at most 10 predictions with default limit, got %d", len(predictions))
	}
}

func TestPrefetch(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Not running test
	engine.Stop()
	_, err := engine.Prefetch("/data/file1.txt", "l1-nvme", 1024)
	if err != ErrEngineNotRunning {
		t.Errorf("Expected ErrEngineNotRunning, got %v", err)
	}
	engine.Start()

	// Invalid layer test
	_, err = engine.Prefetch("/data/file1.txt", "nonexistent", 1024)
	if err != ErrCacheLayerNotFound {
		t.Errorf("Expected ErrCacheLayerNotFound, got %v", err)
	}

	// Successful prefetch
	task, err := engine.Prefetch("/data/file1.txt", "l1-nvme", 1024)
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	if task == nil {
		t.Fatal("Prefetch task should not be nil")
	}
	if task.Status != TaskPending {
		t.Errorf("Expected task status Pending, got %v", task.Status)
	}
	if task.SourcePath != "/data/file1.txt" {
		t.Errorf("Expected source path '/data/file1.txt', got '%s'", task.SourcePath)
	}
	if task.TargetLayer != "l1-nvme" {
		t.Errorf("Expected target layer 'l1-nvme', got '%s'", task.TargetLayer)
	}
}

func TestPrefetchCacheFull(t *testing.T) {
	config := newTestConfig()
	// Use very small layer capacity
	config.LayerConfigs = []LayerConfig{
		{ID: "l1-nvme", Type: CacheLayerNVMe, Capacity: 100, Policy: EvictionLRU},
	}
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Fill the cache
	engine.mu.Lock()
	engine.cacheLayers["l1-nvme"].Entries["existing"] = &CacheEntry{
		Key:  "existing",
		Size: 90,
	}
	engine.cacheLayers["l1-nvme"].Used = 90
	engine.mu.Unlock()

	// Try to prefetch more than capacity
	_, err := engine.Prefetch("/data/big.txt", "l1-nvme", 50)
	if err != ErrCacheFull {
		t.Errorf("Expected ErrCacheFull, got %v", err)
	}
}

func TestGetCacheStats(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Add some data
	engine.RecordAccess("file1", "/data/file1.txt", 1024)
	engine.RecordAccess("file2", "/data/file2.txt", 2048)

	stats := engine.GetCacheStats()

	if stats == nil {
		t.Fatal("GetCacheStats returned nil")
	}

	layers, ok := stats["layers"].(map[string]interface{})
	if !ok {
		t.Fatal("layers should be a map")
	}
	if len(layers) != 3 {
		t.Errorf("Expected 3 layers, got %d", len(layers))
	}

	if stats["history_count"] != 2 {
		t.Errorf("Expected 2 history entries, got %v", stats["history_count"])
	}
}

func TestGetCacheStatsNotRunning(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	// Stats should still work when not running
	stats := engine.GetCacheStats()
	if stats == nil {
		t.Fatal("GetCacheStats returned nil")
	}
}

func TestOptimizeCache(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Not running test
	engine.Stop()
	err := engine.OptimizeCache()
	if err != ErrEngineNotRunning {
		t.Errorf("Expected ErrEngineNotRunning, got %v", err)
	}
	engine.Start()

	// Add hot and cold files
	engine.mu.Lock()
	engine.accessHistory["hotfile"] = &AccessPattern{
		FileID:    "hotfile",
		Frequency: 0.8,
	}
	engine.accessHistory["coldfile"] = &AccessPattern{
		FileID:    "coldfile",
		Frequency: 0.05,
	}
	engine.mu.Unlock()

	err = engine.OptimizeCache()
	if err != nil {
		t.Fatalf("OptimizeCache failed: %v", err)
	}
}

func TestGetMetrics(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Record some accesses to generate metrics
	for i := 0; i < 10; i++ {
		engine.RecordAccess("file1", "/data/file1.txt", 1024)
	}

	metrics := engine.GetMetrics()

	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}
	if metrics.SuccessfulHits == 0 {
		t.Error("Expected some successful hits")
	}
	if metrics.HitRate < 0 || metrics.HitRate > 1 {
		t.Errorf("HitRate out of range: %f", metrics.HitRate)
	}
	if len(metrics.LayerStats) != 3 {
		t.Errorf("Expected 3 layer stats, got %d", len(metrics.LayerStats))
	}
}

func TestGetMetricsIsolation(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Get metrics twice - should be independent copies
	m1 := engine.GetMetrics()
	m2 := engine.GetMetrics()

	m1.TotalPrefetches = 999
	if m2.TotalPrefetches == 999 {
		t.Error("GetMetrics should return independent copies")
	}
}

func TestEvictionLRU(t *testing.T) {
	config := newTestConfig()
	config.LayerConfigs = []LayerConfig{
		{ID: "l1", Type: CacheLayerNVMe, Capacity: 3000, Policy: EvictionLRU},
	}
	engine := NewPrefetchEngine(config, newTestLogger())

	layer := engine.cacheLayers["l1"]

	// Add entries with different last access times
	now := time.Now()
	layer.Entries["a"] = &CacheEntry{Key: "a", Size: 1000, LastAccess: now.Add(-3 * time.Hour)}
	layer.Entries["b"] = &CacheEntry{Key: "b", Size: 1000, LastAccess: now.Add(-1 * time.Hour)}
	layer.Entries["c"] = &CacheEntry{Key: "c", Size: 1000, LastAccess: now}
	layer.Used = 3000

	// Need to evict 1000 bytes
	engine.evictEntries(layer, 1000)

	if _, exists := layer.Entries["a"]; exists {
		t.Error("LRU should have evicted entry 'a' (oldest)")
	}
	if _, exists := layer.Entries["b"]; !exists {
		t.Error("Entry 'b' should not be evicted")
	}
}

func TestEvictionLFU(t *testing.T) {
	config := newTestConfig()
	config.LayerConfigs = []LayerConfig{
		{ID: "l1", Type: CacheLayerNVMe, Capacity: 3000, Policy: EvictionLFU},
	}
	engine := NewPrefetchEngine(config, newTestLogger())

	layer := engine.cacheLayers["l1"]

	layer.Entries["a"] = &CacheEntry{Key: "a", Size: 1000, AccessCount: 10}
	layer.Entries["b"] = &CacheEntry{Key: "b", Size: 1000, AccessCount: 1}
	layer.Entries["c"] = &CacheEntry{Key: "c", Size: 1000, AccessCount: 5}
	layer.Used = 3000

	engine.evictEntries(layer, 1000)

	if _, exists := layer.Entries["b"]; exists {
		t.Error("LFU should have evicted entry 'b' (least frequently used)")
	}
}

func TestEvictionARC(t *testing.T) {
	config := newTestConfig()
	config.LayerConfigs = []LayerConfig{
		{ID: "l1", Type: CacheLayerNVMe, Capacity: 3000, Policy: EvictionARC},
	}
	engine := NewPrefetchEngine(config, newTestLogger())

	layer := engine.cacheLayers["l1"]

	now := time.Now()
	layer.Entries["a"] = &CacheEntry{Key: "a", Size: 1000, AccessCount: 10, LastAccess: now.Add(-5 * time.Hour)}
	layer.Entries["b"] = &CacheEntry{Key: "b", Size: 1000, AccessCount: 1, LastAccess: now.Add(-1 * time.Hour)}
	layer.Entries["c"] = &CacheEntry{Key: "c", Size: 1000, AccessCount: 5, LastAccess: now}
	layer.Used = 3000

	engine.evictEntries(layer, 1000)

	if len(layer.Entries) != 2 {
		t.Errorf("Expected 2 entries after eviction, got %d", len(layer.Entries))
	}
}

func TestEvictionMLAware(t *testing.T) {
	config := newTestConfig()
	config.LayerConfigs = []LayerConfig{
		{ID: "l1", Type: CacheLayerNVMe, Capacity: 3000, Policy: EvictionMLAware},
	}
	engine := NewPrefetchEngine(config, newTestLogger())

	layer := engine.cacheLayers["l1"]

	layer.Entries["a"] = &CacheEntry{Key: "a", Size: 1000, Priority: 0.9}
	layer.Entries["b"] = &CacheEntry{Key: "b", Size: 1000, Priority: 0.1}
	layer.Entries["c"] = &CacheEntry{Key: "c", Size: 1000, Priority: 0.5}
	layer.Used = 3000

	engine.evictEntries(layer, 1000)

	if _, exists := layer.Entries["b"]; exists {
		t.Error("MLAware should have evicted entry 'b' (lowest priority)")
	}
}

func TestPromoteToFastLayer(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Add a file to slow layer
	engine.mu.Lock()
	engine.cacheLayers["l3-hdd"].Entries["hotfile"] = &CacheEntry{
		Key:  "hotfile",
		Size: 1024,
	}
	engine.cacheLayers["l3-hdd"].Used = 1024
	engine.mu.Unlock()

	engine.promoteToFastLayer("hotfile")

	// Verify file was promoted
	engine.mu.RLock()
	_, inHDD := engine.cacheLayers["l3-hdd"].Entries["hotfile"]
	_, inNVMe := engine.cacheLayers["l1-nvme"].Entries["hotfile"]
	engine.mu.RUnlock()

	if inHDD {
		t.Error("File should have been removed from HDD layer")
	}
	if !inNVMe {
		t.Error("File should have been promoted to NVMe layer")
	}
}

func TestDemoteToSlowLayer(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	// Add a file to fast layer
	engine.mu.Lock()
	engine.cacheLayers["l1-nvme"].Entries["coldfile"] = &CacheEntry{
		Key:  "coldfile",
		Size: 1024,
	}
	engine.cacheLayers["l1-nvme"].Used = 1024
	engine.mu.Unlock()

	engine.demoteToSlowLayer("coldfile")

	engine.mu.RLock()
	_, inNVMe := engine.cacheLayers["l1-nvme"].Entries["coldfile"]
	_, inHDD := engine.cacheLayers["l3-hdd"].Entries["coldfile"]
	engine.mu.RUnlock()

	if inNVMe {
		t.Error("File should have been removed from NVMe layer")
	}
	if !inHDD {
		t.Error("File should have been demoted to HDD layer")
	}
}

func TestSelectStrategy(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	tests := []struct {
		pattern  *AccessPattern
		expected PrefetchStrategy
	}{
		{
			&AccessPattern{Sequential: 0.8, Periodic: 0.1},
			PrefetchSequential,
		},
		{
			&AccessPattern{Sequential: 0.2, Periodic: 0.8},
			PrefetchPredictive,
		},
		{
			&AccessPattern{Sequential: 0.3, Periodic: 0.3},
			PrefetchAdaptive,
		},
	}

	for i, test := range tests {
		result := engine.selectStrategy(test.pattern)
		if result != test.expected {
			t.Errorf("Test %d: expected %v, got %v", i, test.expected, result)
		}
	}
}

func TestAnalyzePattern(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	// Single access - no analysis
	pattern := &AccessPattern{
		FileID:      "file1",
		AccessTimes: []time.Time{time.Now()},
		ReadSizes:   []int64{1024},
	}
	engine.analyzePattern(pattern)
	if pattern.Frequency != 0 {
		t.Error("Frequency should be 0 for single access")
	}

	// Multiple accesses
	now := time.Now()
	pattern = &AccessPattern{
		FileID: "file1",
		AccessTimes: []time.Time{
			now.Add(-2 * time.Hour),
			now.Add(-1 * time.Hour),
			now,
		},
		ReadSizes: []int64{1024, 1024, 2048},
	}
	engine.analyzePattern(pattern)

	if pattern.Frequency <= 0 {
		t.Error("Frequency should be positive for multiple accesses")
	}
	if pattern.Sequential <= 0 {
		t.Error("Sequential should be positive for repeated read sizes")
	}
}

func TestDetectPeriodicity(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	// Periodic pattern (every hour)
	now := time.Now()
	times := []time.Time{
		now.Add(-3 * time.Hour),
		now.Add(-2 * time.Hour),
		now.Add(-1 * time.Hour),
		now,
	}

	periodicity := engine.detectPeriodicity(times)
	if periodicity < 0.5 {
		t.Errorf("Expected high periodicity, got %f", periodicity)
	}

	// Non-periodic pattern
	times = []time.Time{
		now.Add(-10 * time.Hour),
		now.Add(-5 * time.Hour),
		now.Add(-3 * time.Hour),
		now,
	}

	periodicity = engine.detectPeriodicity(times)
	if periodicity > 0.5 {
		t.Errorf("Expected low periodicity, got %f", periodicity)
	}

	// Too few data points
	times = []time.Time{now}
	periodicity = engine.detectPeriodicity(times)
	if periodicity != 0 {
		t.Errorf("Expected 0 periodicity for single data point, got %f", periodicity)
	}
}

func TestEstimateSize(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	pattern := &AccessPattern{
		ReadSizes: []int64{1024, 2048, 3072},
	}

	estimated := engine.estimateSize(pattern)
	if estimated <= 0 {
		t.Errorf("Expected positive estimated size, got %d", estimated)
	}

	// Empty pattern
	pattern = &AccessPattern{ReadSizes: []int64{}}
	estimated = engine.estimateSize(pattern)
	if estimated != 0 {
		t.Errorf("Expected 0 for empty pattern, got %d", estimated)
	}
}

func TestCalculateConfidence(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	// Few data points
	pattern := &AccessPattern{
		AccessTimes: []time.Time{time.Now(), time.Now()},
		Frequency:   0.5,
	}
	conf := engine.calculateConfidence(pattern)
	if conf < 0 || conf > 1 {
		t.Errorf("Confidence out of range: %f", conf)
	}

	// Many data points
	times := make([]time.Time, 30)
	for i := range times {
		times[i] = time.Now()
	}
	pattern = &AccessPattern{
		AccessTimes: times,
		Frequency:   0.8,
	}
	conf = engine.calculateConfidence(pattern)
	if conf < 0 || conf > 1 {
		t.Errorf("Confidence out of range: %f", conf)
	}

	// Single data point
	pattern = &AccessPattern{
		AccessTimes: []time.Time{time.Now()},
	}
	conf = engine.calculateConfidence(pattern)
	if conf != 0 {
		t.Errorf("Expected 0 confidence for single data point, got %f", conf)
	}
}

func TestCalculatePriority(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	// Unknown file
	priority := engine.calculatePriority("unknown")
	if priority != 0.5 {
		t.Errorf("Expected 0.5 for unknown file, got %f", priority)
	}

	// Known file
	engine.accessHistory["known"] = &AccessPattern{
		Frequency:  0.8,
		Sequential: 0.6,
		Periodic:   0.4,
	}
	priority = engine.calculatePriority("known")
	if priority <= 0 {
		t.Errorf("Expected positive priority, got %f", priority)
	}
}

func TestMLPredictor(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	predictor := engine.mlModel

	pattern := &AccessPattern{
		Frequency:  0.8,
		Sequential: 0.6,
		Periodic:   0.4,
		AccessTimes: []time.Time{
			time.Now().Add(-1 * time.Hour),
			time.Now(),
		},
	}

	score := predictor.predict("file1", pattern)
	if score < 0 || score > 1 {
		t.Errorf("Prediction score out of range: %f", score)
	}
	if predictor.predictions != 1 {
		t.Errorf("Expected 1 prediction count, got %d", predictor.predictions)
	}
}

func TestSortPredictions(t *testing.T) {
	predictions := []*Prediction{
		{FileID: "a", Probability: 0.3},
		{FileID: "b", Probability: 0.8},
		{FileID: "c", Probability: 0.1},
		{FileID: "d", Probability: 0.6},
	}

	sortPredictions(predictions)

	for i := 1; i < len(predictions); i++ {
		if predictions[i].Probability > predictions[i-1].Probability {
			t.Error("Predictions should be sorted descending")
		}
	}

	if predictions[0].FileID != "b" {
		t.Errorf("Expected 'b' first, got '%s'", predictions[0].FileID)
	}
}

func TestCacheLayerTypeEnum(t *testing.T) {
	tests := []struct {
		layerType CacheLayerType
		expected  string
	}{
		{CacheLayerNVMe, "NVMe"},
		{CacheLayerSSD, "SSD"},
		{CacheLayerHDD, "HDD"},
		{CacheLayerType(99), "Unknown"},
	}

	for _, test := range tests {
		if test.layerType.String() != test.expected {
			t.Errorf("CacheLayerType(%d).String() = %s, want %s", test.layerType, test.layerType.String(), test.expected)
		}
	}
}

func TestEvictionPolicyEnum(t *testing.T) {
	tests := []struct {
		policy   EvictionPolicy
		expected string
	}{
		{EvictionLRU, "LRU"},
		{EvictionLFU, "LFU"},
		{EvictionARC, "ARC"},
		{EvictionMLAware, "ML_AWARE"},
		{EvictionPolicy(99), "Unknown"},
	}

	for _, test := range tests {
		if test.policy.String() != test.expected {
			t.Errorf("EvictionPolicy(%d).String() = %s, want %s", test.policy, test.policy.String(), test.expected)
		}
	}
}

func TestPrefetchStrategyEnum(t *testing.T) {
	tests := []struct {
		strategy PrefetchStrategy
		expected string
	}{
		{PrefetchSequential, "Sequential"},
		{PrefetchPredictive, "Predictive"},
		{PrefetchAdaptive, "Adaptive"},
		{PrefetchStrategy(99), "Unknown"},
	}

	for _, test := range tests {
		if test.strategy.String() != test.expected {
			t.Errorf("PrefetchStrategy(%d).String() = %s, want %s", test.strategy, test.strategy.String(), test.expected)
		}
	}
}

func TestTaskStatusEnum(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskPending, "Pending"},
		{TaskRunning, "Running"},
		{TaskCompleted, "Completed"},
		{TaskFailed, "Failed"},
		{TaskStatus(99), "Unknown"},
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("TaskStatus(%d).String() = %s, want %s", test.status, test.status.String(), test.expected)
		}
	}
}

func TestDefaultPrefetchConfig(t *testing.T) {
	config := DefaultPrefetchConfig()

	if config.MaxCacheSize <= 0 {
		t.Error("MaxCacheSize should be positive")
	}
	if config.PrefetchWindow <= 0 {
		t.Error("PrefetchWindow should be positive")
	}
	if config.MaxQueueSize <= 0 {
		t.Error("MaxQueueSize should be positive")
	}
	if len(config.LayerConfigs) != 3 {
		t.Errorf("Expected 3 layer configs, got %d", len(config.LayerConfigs))
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())
	engine.Start()
	defer engine.Stop()

	done := make(chan bool, 10)

	// Concurrent record access
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				engine.RecordAccess("file"+string(rune('A'+id)), "/data/file.txt", 1024)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no race conditions
	engine.mu.RLock()
	count := len(engine.accessHistory)
	engine.mu.RUnlock()

	if count != 10 {
		t.Errorf("Expected 10 access patterns, got %d", count)
	}
}

func TestTotalCachedFiles(t *testing.T) {
	config := newTestConfig()
	engine := NewPrefetchEngine(config, newTestLogger())

	engine.cacheLayers["l1-nvme"].Entries["a"] = &CacheEntry{Key: "a"}
	engine.cacheLayers["l1-nvme"].Entries["b"] = &CacheEntry{Key: "b"}
	engine.cacheLayers["l2-ssd"].Entries["c"] = &CacheEntry{Key: "c"}

	total := engine.totalCachedFiles()
	if total != 3 {
		t.Errorf("Expected 3 total cached files, got %d", total)
	}
}
