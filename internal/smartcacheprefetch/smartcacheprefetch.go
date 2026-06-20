package smartcacheprefetch

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// PrefetchEngine 智能缓存预取引擎
type PrefetchEngine struct {
	mu            sync.RWMutex
	cacheLayers   map[string]*CacheLayer    // 多级缓存层
	accessHistory map[string]*AccessPattern // 访问模式记录
	predictions   map[string]*Prediction    // 预取预测结果
	prefetchQueue chan *PrefetchTask        // 预取任务队列
	mlModel       *AccessPredictor          // ML预测模型
	metrics       *PrefetchMetrics          // 性能指标
	config        *PrefetchConfig           // 配置
	logger        *slog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	running       bool
}

// NewPrefetchEngine creates a new prefetch engine with the given configuration
func NewPrefetchEngine(config PrefetchConfig, logger *slog.Logger) *PrefetchEngine {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &PrefetchEngine{
		cacheLayers:   make(map[string]*CacheLayer),
		accessHistory: make(map[string]*AccessPattern),
		predictions:   make(map[string]*Prediction),
		prefetchQueue: make(chan *PrefetchTask, config.MaxQueueSize),
		mlModel: &AccessPredictor{
			weights: map[string]float64{
				"frequency":  0.3,
				"recency":    0.3,
				"sequential": 0.2,
				"periodic":   0.2,
			},
			features: []string{"frequency", "recency", "sequential", "periodic"},
		},
		metrics: &PrefetchMetrics{
			LayerStats: make(map[string]*LayerStats),
		},
		config:  &config,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		running: false,
	}

	// Initialize cache layers from config
	for _, lc := range config.LayerConfigs {
		engine.cacheLayers[lc.ID] = &CacheLayer{
			ID:        lc.ID,
			Name:      fmt.Sprintf("Cache Layer %s", lc.Type),
			Type:      lc.Type,
			Capacity:  lc.Capacity,
			Used:      0,
			Entries:   make(map[string]*CacheEntry),
			Policy:    lc.Policy,
			HitRate:   0,
			CreatedAt: time.Now(),
		}
		engine.metrics.LayerStats[lc.ID] = &LayerStats{}
	}

	return engine
}

// Start starts the prefetch engine background workers
func (e *PrefetchEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return ErrEngineAlreadyRunning
	}

	e.running = true
	e.logger.Info("Starting SmartCachePrefetch engine",
		"layers", len(e.cacheLayers),
		"queue_size", e.config.MaxQueueSize,
		"ml_enabled", e.config.MLModelEnabled,
	)

	// Start prefetch worker
	go e.prefetchWorker()

	// Start cache optimizer
	go e.cacheOptimizer()

	// Start ML model updater if enabled
	if e.config.MLModelEnabled {
		go e.mlModelUpdater()
	}

	return nil
}

// Stop gracefully stops the prefetch engine
func (e *PrefetchEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return ErrEngineNotRunning
	}

	e.logger.Info("Stopping SmartCachePrefetch engine")
	e.cancel()
	e.running = false

	return nil
}

// RecordAccess records a file access and updates the access pattern
func (e *PrefetchEngine) RecordAccess(fileID string, path string, size int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return ErrEngineNotRunning
	}

	now := time.Now()

	// Update or create access pattern
	pattern, exists := e.accessHistory[fileID]
	if !exists {
		pattern = &AccessPattern{
			FileID:      fileID,
			AccessTimes: []time.Time{now},
			ReadSizes:   []int64{size},
			LastUpdated: now,
		}
		e.accessHistory[fileID] = pattern
	} else {
		pattern.AccessTimes = append(pattern.AccessTimes, now)
		pattern.ReadSizes = append(pattern.ReadSizes, size)
		pattern.LastUpdated = now

		// Keep only recent access history (last 100 entries)
		if len(pattern.AccessTimes) > 100 {
			pattern.AccessTimes = pattern.AccessTimes[len(pattern.AccessTimes)-100:]
			pattern.ReadSizes = pattern.ReadSizes[len(pattern.ReadSizes)-100:]
		}
	}

	// Update pattern analysis
	e.analyzePattern(pattern)

	// Check if file is already cached
	cached := false
	for _, layer := range e.cacheLayers {
		if entry, ok := layer.Entries[fileID]; ok {
			entry.AccessCount++
			entry.LastAccess = now
			cached = true
			break
		}
	}

	if cached {
		// Update metrics - cache hit
		e.metrics.SuccessfulHits++
		e.updateHitRate()
	}

	e.logger.Debug("Recorded file access",
		"file_id", fileID,
		"path", path,
		"size", size,
		"cached", cached,
	)

	return nil
}

// Predict predicts which files are likely to be accessed next
func (e *PrefetchEngine) Predict(topN int) ([]*Prediction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.running {
		return nil, ErrEngineNotRunning
	}

	if topN <= 0 {
		topN = 10
	}

	predictions := make([]*Prediction, 0, topN)

	for fileID, pattern := range e.accessHistory {
		prob := e.mlModel.predict(fileID, pattern)
		if prob < 0.1 {
			continue
		}

		strategy := e.selectStrategy(pattern)

		pred := &Prediction{
			FileID:       fileID,
			Probability:  prob,
			ExpectedTime: time.Now().Add(e.config.PrefetchWindow),
			ExpectedSize: e.estimateSize(pattern),
			Confidence:   e.calculateConfidence(pattern),
			Strategy:     strategy,
		}

		predictions = append(predictions, pred)
		e.predictions[fileID] = pred
	}

	// Sort by probability descending
	sortPredictions(predictions)

	// Return top N
	if len(predictions) > topN {
		predictions = predictions[:topN]
	}

	e.logger.Info("Generated predictions", "count", len(predictions))
	return predictions, nil
}

// Prefetch executes a prefetch operation for the given file
func (e *PrefetchEngine) Prefetch(sourcePath string, targetLayer string, size int64) (*PrefetchTask, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil, ErrEngineNotRunning
	}

	// Check if target layer exists
	layer, exists := e.cacheLayers[targetLayer]
	if !exists {
		return nil, ErrCacheLayerNotFound
	}

	// Check if layer has capacity
	if layer.Used+size > layer.Capacity {
		// Try to evict entries
		e.evictEntries(layer, size)
		if layer.Used+size > layer.Capacity {
			return nil, ErrCacheFull
		}
	}

	// Create prefetch task
	task := &PrefetchTask{
		ID:          fmt.Sprintf("prefetch-%d", time.Now().UnixNano()),
		SourcePath:  sourcePath,
		TargetLayer: targetLayer,
		Size:        size,
		Priority:    e.calculatePriority(sourcePath),
		Deadline:    time.Now().Add(e.config.PrefetchWindow),
		Status:      TaskPending,
		CreatedAt:   time.Now(),
	}

	// Add to queue
	select {
	case e.prefetchQueue <- task:
		e.metrics.TotalPrefetches++
		e.logger.Info("Prefetch task queued",
			"task_id", task.ID,
			"source", sourcePath,
			"target", targetLayer,
			"size", size,
		)
	default:
		return nil, ErrQueueFull
	}

	return task, nil
}

// GetCacheStats returns cache statistics for all layers
func (e *PrefetchEngine) GetCacheStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := make(map[string]interface{})
	layers := make(map[string]interface{})

	for id, layer := range e.cacheLayers {
		layerStats := map[string]interface{}{
			"id":          layer.ID,
			"name":        layer.Name,
			"type":        layer.Type.String(),
			"capacity":    layer.Capacity,
			"used":        layer.Used,
			"utilization": float64(layer.Used) / float64(layer.Capacity) * 100,
			"entries":     len(layer.Entries),
			"policy":      layer.Policy.String(),
			"hit_rate":    layer.HitRate,
		}
		layers[id] = layerStats
	}

	stats["layers"] = layers
	stats["total_cached_files"] = e.totalCachedFiles()
	stats["predictions_count"] = len(e.predictions)
	stats["history_count"] = len(e.accessHistory)
	stats["queue_length"] = len(e.prefetchQueue)

	return stats
}

// OptimizeCache optimizes cache layout based on access patterns and predictions
func (e *PrefetchEngine) OptimizeCache() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return ErrEngineNotRunning
	}

	e.logger.Info("Starting cache optimization")

	// Step 1: Identify hot and cold data
	hotFiles := make([]string, 0)
	coldFiles := make([]string, 0)

	for fileID, pattern := range e.accessHistory {
		if pattern.Frequency > 0.5 {
			hotFiles = append(hotFiles, fileID)
		} else if pattern.Frequency < 0.1 {
			coldFiles = append(coldFiles, fileID)
		}
	}

	// Step 2: Move hot data to faster layers
	for _, fileID := range hotFiles {
		e.promoteToFastLayer(fileID)
	}

	// Step 3: Demote cold data to slower layers
	for _, fileID := range coldFiles {
		e.demoteToSlowLayer(fileID)
	}

	// Step 4: Update ML model weights based on recent performance
	e.updateMLWeights()

	e.logger.Info("Cache optimization completed",
		"hot_files", len(hotFiles),
		"cold_files", len(coldFiles),
	)

	return nil
}

// GetMetrics returns current prefetch performance metrics
func (e *PrefetchEngine) GetMetrics() *PrefetchMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return a copy
	metrics := &PrefetchMetrics{
		TotalPrefetches: e.metrics.TotalPrefetches,
		SuccessfulHits:  e.metrics.SuccessfulHits,
		Misses:          e.metrics.Misses,
		HitRate:         e.metrics.HitRate,
		BytesSaved:      e.metrics.BytesSaved,
		AvgPrefetchTime: e.metrics.AvgPrefetchTime,
		CacheEfficiency: e.metrics.CacheEfficiency,
		LayerStats:      make(map[string]*LayerStats),
	}

	for id, ls := range e.metrics.LayerStats {
		metrics.LayerStats[id] = &LayerStats{
			HitCount:   ls.HitCount,
			MissCount:  ls.MissCount,
			HitRate:    ls.HitRate,
			AvgLatency: ls.AvgLatency,
			Evictions:  ls.Evictions,
		}
	}

	return metrics
}

// prefetchWorker processes prefetch tasks from the queue
func (e *PrefetchEngine) prefetchWorker() {
	e.logger.Info("Prefetch worker started")
	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("Prefetch worker stopped")
			return
		case task := <-e.prefetchQueue:
			e.executePrefetch(task)
		}
	}
}

// executePrefetch executes a single prefetch task
func (e *PrefetchEngine) executePrefetch(task *PrefetchTask) {
	e.mu.Lock()
	task.Status = TaskRunning
	e.mu.Unlock()

	start := time.Now()

	// Simulate prefetch operation
	// In real implementation, this would read from source and write to cache layer
	e.mu.Lock()
	layer, exists := e.cacheLayers[task.TargetLayer]
	if !exists {
		task.Status = TaskFailed
		e.metrics.Misses++
		e.mu.Unlock()
		return
	}

	// Create cache entry
	entry := &CacheEntry{
		Key:         task.SourcePath,
		Path:        task.SourcePath,
		Size:        task.Size,
		Layer:       task.TargetLayer,
		AccessCount: 0,
		LastAccess:  time.Now(),
		CreatedAt:   time.Now(),
		Priority:    task.Priority,
		TTL:         1 * time.Hour,
	}

	layer.Entries[task.SourcePath] = entry
	layer.Used += task.Size
	task.Status = TaskCompleted
	e.metrics.BytesSaved += task.Size

	elapsed := time.Since(start)
	if e.metrics.AvgPrefetchTime == 0 {
		e.metrics.AvgPrefetchTime = elapsed
	} else {
		e.metrics.AvgPrefetchTime = (e.metrics.AvgPrefetchTime + elapsed) / 2
	}

	e.mu.Unlock()

	e.logger.Info("Prefetch completed",
		"task_id", task.ID,
		"source", task.SourcePath,
		"duration", elapsed,
	)
}

// cacheOptimizer periodically optimizes cache layout
func (e *PrefetchEngine) cacheOptimizer() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if err := e.OptimizeCache(); err != nil {
				e.logger.Error("Cache optimization failed", "error", err)
			}
		}
	}
}

// mlModelUpdater periodically retrains the ML model
func (e *PrefetchEngine) mlModelUpdater() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.mu.Lock()
			e.trainModel()
			e.mu.Unlock()
		}
	}
}

// analyzePattern analyzes an access pattern and updates its metrics
func (e *PrefetchEngine) analyzePattern(pattern *AccessPattern) {
	if len(pattern.AccessTimes) < 2 {
		return
	}

	// Calculate frequency (accesses per hour)
	duration := pattern.AccessTimes[len(pattern.AccessTimes)-1].Sub(pattern.AccessTimes[0])
	if duration > 0 {
		pattern.Frequency = float64(len(pattern.AccessTimes)) / duration.Hours()
		// Normalize to 0-1 range (cap at 10 accesses/hour)
		pattern.Frequency = math.Min(pattern.Frequency/10.0, 1.0)
	}

	// Calculate sequential access probability
	seqCount := 0
	for i := 1; i < len(pattern.ReadSizes); i++ {
		if pattern.ReadSizes[i] == pattern.ReadSizes[i-1] {
			seqCount++
		}
	}
	if len(pattern.ReadSizes) > 1 {
		pattern.Sequential = float64(seqCount) / float64(len(pattern.ReadSizes)-1)
	}
	pattern.Random = 1.0 - pattern.Sequential

	// Detect periodic pattern
	pattern.Periodic = e.detectPeriodicity(pattern.AccessTimes)
}

// detectPeriodicity detects if access pattern is periodic
func (e *PrefetchEngine) detectPeriodicity(times []time.Time) float64 {
	if len(times) < 3 {
		return 0
	}

	// Calculate intervals between accesses
	intervals := make([]time.Duration, len(times)-1)
	for i := 1; i < len(times); i++ {
		intervals[i-1] = times[i].Sub(times[i-1])
	}

	// Check if intervals are consistent
	if len(intervals) < 2 {
		return 0
	}

	avgInterval := time.Duration(0)
	for _, interval := range intervals {
		avgInterval += interval
	}
	avgInterval /= time.Duration(len(intervals))

	// Calculate variance
	variance := float64(0)
	for _, interval := range intervals {
		diff := float64(interval - avgInterval)
		variance += diff * diff
	}
	variance /= float64(len(intervals))

	// Lower variance means more periodic
	// Normalize to 0-1 range
	if avgInterval > 0 {
		normalizedVariance := variance / float64(avgInterval*avgInterval)
		return math.Max(0, 1.0-normalizedVariance)
	}

	return 0
}

// selectStrategy selects the best prefetch strategy for a pattern
func (e *PrefetchEngine) selectStrategy(pattern *AccessPattern) PrefetchStrategy {
	if pattern.Sequential > 0.7 {
		return PrefetchSequential
	}
	if pattern.Periodic > 0.7 {
		return PrefetchPredictive
	}
	return PrefetchAdaptive
}

// estimateSize estimates the next access size based on history
func (e *PrefetchEngine) estimateSize(pattern *AccessPattern) int64 {
	if len(pattern.ReadSizes) == 0 {
		return 0
	}

	// Use weighted average (recent accesses have higher weight)
	total := float64(0)
	weightSum := float64(0)
	for i, size := range pattern.ReadSizes {
		weight := float64(i + 1)
		total += float64(size) * weight
		weightSum += weight
	}

	if weightSum > 0 {
		return int64(total / weightSum)
	}
	return pattern.ReadSizes[len(pattern.ReadSizes)-1]
}

// calculateConfidence calculates prediction confidence
func (e *PrefetchEngine) calculateConfidence(pattern *AccessPattern) float64 {
	if len(pattern.AccessTimes) < 2 {
		return 0
	}

	// More data points = higher confidence
	dataConfidence := math.Min(float64(len(pattern.AccessTimes))/20.0, 1.0)

	// Higher frequency = higher confidence
	freqConfidence := pattern.Frequency

	// Combine factors
	return (dataConfidence + freqConfidence) / 2.0
}

// calculatePriority calculates prefetch priority for a file
func (e *PrefetchEngine) calculatePriority(sourcePath string) float64 {
	pattern, exists := e.accessHistory[sourcePath]
	if !exists {
		return 0.5
	}

	return (pattern.Frequency*0.4 + pattern.Sequential*0.3 + pattern.Periodic*0.3)
}

// evictEntries evicts cache entries to make room for new data
func (e *PrefetchEngine) evictEntries(layer *CacheLayer, neededSize int64) {
	freedSize := int64(0)

	switch layer.Policy {
	case EvictionLRU:
		freedSize = e.evictLRU(layer, neededSize)
	case EvictionLFU:
		freedSize = e.evictLFU(layer, neededSize)
	case EvictionARC:
		freedSize = e.evictARC(layer, neededSize)
	case EvictionMLAware:
		freedSize = e.evictMLAware(layer, neededSize)
	}

	if stats, ok := e.metrics.LayerStats[layer.ID]; ok {
		stats.Evictions++
	}

	e.logger.Debug("Evicted cache entries",
		"layer", layer.ID,
		"freed", freedSize,
		"needed", neededSize,
	)
}

// evictLRU evicts least recently used entries
func (e *PrefetchEngine) evictLRU(layer *CacheLayer, neededSize int64) int64 {
	freed := int64(0)
	for freed < neededSize && len(layer.Entries) > 0 {
		var oldest *CacheEntry
		for _, entry := range layer.Entries {
			if oldest == nil || entry.LastAccess.Before(oldest.LastAccess) {
				oldest = entry
			}
		}
		if oldest != nil {
			delete(layer.Entries, oldest.Key)
			layer.Used -= oldest.Size
			freed += oldest.Size
		}
	}
	return freed
}

// evictLFU evicts least frequently used entries
func (e *PrefetchEngine) evictLFU(layer *CacheLayer, neededSize int64) int64 {
	freed := int64(0)
	for freed < neededSize && len(layer.Entries) > 0 {
		var leastUsed *CacheEntry
		for _, entry := range layer.Entries {
			if leastUsed == nil || entry.AccessCount < leastUsed.AccessCount {
				leastUsed = entry
			}
		}
		if leastUsed != nil {
			delete(layer.Entries, leastUsed.Key)
			layer.Used -= leastUsed.Size
			freed += leastUsed.Size
		}
	}
	return freed
}

// evictARC evicts using Adaptive Replacement Cache policy
func (e *PrefetchEngine) evictARC(layer *CacheLayer, neededSize int64) int64 {
	// Simplified ARC: balance between LRU and LFU
	freed := int64(0)
	for freed < neededSize && len(layer.Entries) > 0 {
		var victim *CacheEntry
		bestScore := math.MaxFloat64

		for _, entry := range layer.Entries {
			// Score combines recency and frequency
			recency := time.Since(entry.LastAccess).Seconds()
			frequency := float64(entry.AccessCount)
			score := recency / (frequency + 1)
			if score < bestScore {
				bestScore = score
				victim = entry
			}
		}
		if victim != nil {
			delete(layer.Entries, victim.Key)
			layer.Used -= victim.Size
			freed += victim.Size
		}
	}
	return freed
}

// evictMLAware evicts entries using ML-based priority
func (e *PrefetchEngine) evictMLAware(layer *CacheLayer, neededSize int64) int64 {
	freed := int64(0)
	for freed < neededSize && len(layer.Entries) > 0 {
		var victim *CacheEntry
		lowestPriority := math.MaxFloat64

		for _, entry := range layer.Entries {
			if entry.Priority < lowestPriority {
				lowestPriority = entry.Priority
				victim = entry
			}
		}
		if victim != nil {
			delete(layer.Entries, victim.Key)
			layer.Used -= victim.Size
			freed += victim.Size
		}
	}
	return freed
}

// promoteToFastLayer promotes a file to faster cache layer
func (e *PrefetchEngine) promoteToFastLayer(fileID string) {
	var currentEntry *CacheEntry
	var currentLayer string

	// Find current location
	for layerID, layer := range e.cacheLayers {
		if entry, ok := layer.Entries[fileID]; ok {
			currentEntry = entry
			currentLayer = layerID
			break
		}
	}

	if currentEntry == nil {
		return
	}

	// Find fastest layer with capacity
	for _, layer := range e.cacheLayers {
		if layer.Type == CacheLayerNVMe && layer.ID != currentLayer {
			if layer.Used+currentEntry.Size <= layer.Capacity {
				// Move entry
				delete(e.cacheLayers[currentLayer].Entries, fileID)
				e.cacheLayers[currentLayer].Used -= currentEntry.Size

				currentEntry.Layer = layer.ID
				layer.Entries[fileID] = currentEntry
				layer.Used += currentEntry.Size

				e.logger.Debug("Promoted file to faster layer",
					"file_id", fileID,
					"from", currentLayer,
					"to", layer.ID,
				)
				return
			}
		}
	}
}

// demoteToSlowLayer demotes a file to slower cache layer
func (e *PrefetchEngine) demoteToSlowLayer(fileID string) {
	var currentEntry *CacheEntry
	var currentLayer string

	for layerID, layer := range e.cacheLayers {
		if entry, ok := layer.Entries[fileID]; ok {
			currentEntry = entry
			currentLayer = layerID
			break
		}
	}

	if currentEntry == nil {
		return
	}

	for _, layer := range e.cacheLayers {
		if layer.Type == CacheLayerHDD && layer.ID != currentLayer {
			if layer.Used+currentEntry.Size <= layer.Capacity {
				delete(e.cacheLayers[currentLayer].Entries, fileID)
				e.cacheLayers[currentLayer].Used -= currentEntry.Size

				currentEntry.Layer = layer.ID
				layer.Entries[fileID] = currentEntry
				layer.Used += currentEntry.Size

				e.logger.Debug("Demoted file to slower layer",
					"file_id", fileID,
					"from", currentLayer,
					"to", layer.ID,
				)
				return
			}
		}
	}
}

// updateHitRate updates overall hit rate
func (e *PrefetchEngine) updateHitRate() {
	total := e.metrics.SuccessfulHits + e.metrics.Misses
	if total > 0 {
		e.metrics.HitRate = float64(e.metrics.SuccessfulHits) / float64(total)
	}
}

// totalCachedFiles returns total number of cached files across all layers
func (e *PrefetchEngine) totalCachedFiles() int {
	total := 0
	for _, layer := range e.cacheLayers {
		total += len(layer.Entries)
	}
	return total
}

// updateMLWeights updates ML model weights based on prediction accuracy
func (e *PrefetchEngine) updateMLWeights() {
	if e.mlModel.predictions == 0 {
		return
	}

	accuracy := float64(e.mlModel.hits) / float64(e.mlModel.predictions)

	// Adjust weights based on accuracy
	if accuracy > 0.8 {
		// Model is performing well, maintain weights
		return
	}

	// Slightly randomize weights to explore better configurations
	for key := range e.mlModel.weights {
		e.mlModel.weights[key] *= (0.9 + 0.2*float64(time.Now().UnixNano()%100)/100.0)
	}

	e.mlModel.accuracy = accuracy
	e.mlModel.trainedAt = time.Now()
}

// trainModel retrains the ML model with accumulated data
func (e *PrefetchEngine) trainModel() {
	e.logger.Info("Training ML model",
		"patterns", len(e.accessHistory),
		"predictions", e.mlModel.predictions,
	)

	// Simplified model training
	// In production, this would use actual ML algorithms
	e.mlModel.trainedAt = time.Now()

	// Calculate feature importance based on historical accuracy
	if e.mlModel.predictions > 0 {
		e.mlModel.accuracy = float64(e.mlModel.hits) / float64(e.mlModel.predictions)
	}
}

// predict calculates access probability for a file
func (p *AccessPredictor) predict(fileID string, pattern *AccessPattern) float64 {
	score := 0.0

	// Frequency factor
	score += pattern.Frequency * p.weights["frequency"]

	// Recency factor
	if len(pattern.AccessTimes) > 0 {
		recency := 1.0 / (1.0 + time.Since(pattern.AccessTimes[len(pattern.AccessTimes)-1]).Hours())
		score += recency * p.weights["recency"]
	}

	// Sequential factor
	score += pattern.Sequential * p.weights["sequential"]

	// Periodic factor
	score += pattern.Periodic * p.weights["periodic"]

	p.predictions++
	return math.Min(score, 1.0)
}

// sortPredictions sorts predictions by probability descending
func sortPredictions(predictions []*Prediction) {
	for i := 1; i < len(predictions); i++ {
		key := predictions[i]
		j := i - 1
		for j >= 0 && predictions[j].Probability < key.Probability {
			predictions[j+1] = predictions[j]
			j--
		}
		predictions[j+1] = key
	}
}
