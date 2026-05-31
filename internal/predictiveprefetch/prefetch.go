// Package predictiveprefetch provides predictive file prefetching.
//
// Uses machine learning to predict which files users will access next
// and prefetches them into cache for faster access.
//
// Features:
// - Access pattern learning
// - Time-based predictions
// - Sequential access detection
// - Context-aware prefetching
// - Cache management
package predictiveprefetch

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// AccessPattern represents a file access pattern.
type AccessPattern struct {
	FilePath   string    `json:"file_path"`
	UserID     string    `json:"user_id"`
	AccessTime time.Time `json:"access_time"`
	Duration   float64   `json:"duration"` // seconds
	Sequential bool      `json:"sequential"`
	NextFile   string    `json:"next_file,omitempty"`
}

// PrefetchCandidate represents a file that could be prefetched.
type PrefetchCandidate struct {
	FilePath    string  `json:"file_path"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
	Size        int64   `json:"size"`
	Priority    int     `json:"priority"`
	AccessCount int     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
}

// CacheEntry represents a cached file.
type CacheEntry struct {
	FilePath   string    `json:"file_path"`
	Size       int64     `json:"size"`
	CachedAt   time.Time `json:"cached_at"`
	AccessCount int      `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	HitRate     float64  `json:"hit_rate"`
}

// PrefetchStats represents prefetch statistics.
type PrefetchStats struct {
	TotalPredictions  int     `json:"total_predictions"`
	CorrectPredictions int   `json:"correct_predictions"`
	Accuracy          float64 `json:"accuracy"`
	CacheHits         int     `json:"cache_hits"`
	CacheMisses       int     `json:"cache_misses"`
	HitRate           float64 `json:"hit_rate"`
	PrefetchedBytes   int64   `json:"prefetched_bytes"`
	SavedTimeMs       int64   `json:"saved_time_ms"`
}

// PrefetchConfig represents prefetch configuration.
type PrefetchConfig struct {
	MaxCacheSize    int64   `json:"max_cache_size"`    // bytes
	MaxEntries      int     `json:"max_entries"`
	PrefetchThreshold float64 `json:"prefetch_threshold"` // 0-1
	LearningRate    float64 `json:"learning_rate"`
	DecayFactor     float64 `json:"decay_factor"`
	EnableSequential bool   `json:"enable_sequential"`
	EnableTemporal   bool   `json:"enable_temporal"`
	EnableCollaborative bool `json:"enable_collaborative"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() PrefetchConfig {
	return PrefetchConfig{
		MaxCacheSize:     1024 * 1024 * 1024, // 1GB
		MaxEntries:       1000,
		PrefetchThreshold: 0.7,
		LearningRate:     0.1,
		DecayFactor:      0.95,
		EnableSequential: true,
		EnableTemporal:   true,
		EnableCollaborative: false,
	}
}

// PredictivePrefetch manages predictive file prefetching.
type PredictivePrefetch struct {
	mu          sync.RWMutex
	config      PrefetchConfig
	patterns    map[string][]AccessPattern // user -> patterns
	sequences   map[string][]string        // file -> next files
	temporal    map[string]map[int]int     // file -> hour -> count
	cache       *list.List                 // LRU cache
	cacheIndex  map[string]*list.Element
	cacheSize   int64
	stats       PrefetchStats
	enabled     bool
}

// NewPredictivePrefetch creates a new predictive prefetch manager.
func NewPredictivePrefetch(config PrefetchConfig) *PredictivePrefetch {
	return &PredictivePrefetch{
		config:     config,
		patterns:   make(map[string][]AccessPattern),
		sequences:  make(map[string][]string),
		temporal:   make(map[string]map[int]int),
		cache:      list.New(),
		cacheIndex: make(map[string]*list.Element),
		enabled:    true,
	}
}

// RecordAccess records a file access for learning.
func (pp *PredictivePrefetch) RecordAccess(ctx context.Context, userID, filePath string, duration float64) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	pattern := AccessPattern{
		FilePath:   filePath,
		UserID:     userID,
		AccessTime: time.Now(),
		Duration:   duration,
	}

	// Update patterns
	pp.patterns[userID] = append(pp.patterns[userID], pattern)

	// Update sequence if there's a previous access
	patterns := pp.patterns[userID]
	if len(patterns) > 1 {
		prev := patterns[len(patterns)-2]
		prevFile := prev.FilePath
		pp.sequences[prevFile] = append(pp.sequences[prevFile], filePath)
		pattern.Sequential = true
	}

	// Update temporal patterns
	hour := time.Now().Hour()
	if pp.temporal[filePath] == nil {
		pp.temporal[filePath] = make(map[int]int)
	}
	pp.temporal[filePath][hour]++

	// Trim old patterns (keep last 1000 per user)
	if len(pp.patterns[userID]) > 1000 {
		pp.patterns[userID] = pp.patterns[userID][len(pp.patterns[userID])-1000:]
	}

	return nil
}

// Predict predicts which files will be accessed next.
func (pp *PredictivePrefetch) Predict(ctx context.Context, userID string, currentFile string) []PrefetchCandidate {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	candidates := make(map[string]*PrefetchCandidate)

	// Sequential prediction
	if pp.config.EnableSequential {
		pp.predictSequential(currentFile, candidates)
	}

	// Temporal prediction
	if pp.config.EnableTemporal {
		pp.predictTemporal(candidates)
	}

	// Collaborative prediction
	if pp.config.EnableCollaborative {
		pp.predictCollaborative(userID, currentFile, candidates)
	}

	// Convert to slice and sort by score
	result := make([]PrefetchCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Score >= pp.config.PrefetchThreshold {
			result = append(result, *c)
		}
	}

	// Sort by score (descending)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Score > result[i].Score {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Limit to top 10
	if len(result) > 10 {
		result = result[:10]
	}

	pp.stats.TotalPredictions++
	return result
}

// predictSequential predicts based on sequential access patterns.
func (pp *PredictivePrefetch) predictSequential(currentFile string, candidates map[string]*PrefetchCandidate) {
	nextFiles, ok := pp.sequences[currentFile]
	if !ok {
		return
	}

	// Count occurrences
	counts := make(map[string]int)
	for _, f := range nextFiles {
		counts[f]++
	}

	total := len(nextFiles)
	for file, count := range counts {
		score := float64(count) / float64(total)
		if existing, ok := candidates[file]; ok {
			existing.Score = max(existing.Score, score)
			existing.Reason = "sequential"
		} else {
			candidates[file] = &PrefetchCandidate{
				FilePath: file,
				Score:    score,
				Reason:   "sequential",
				Priority: 1,
			}
		}
	}
}

// predictTemporal predicts based on time-of-day patterns.
func (pp *PredictivePrefetch) predictTemporal(candidates map[string]*PrefetchCandidate) {
	hour := time.Now().Hour()

	for file, hourCounts := range pp.temporal {
		total := 0
		for _, c := range hourCounts {
			total += c
		}
		if total == 0 {
			continue
		}

		// Calculate temporal score
		currentCount := hourCounts[hour]
		score := float64(currentCount) / float64(total) * 0.8 // Lower weight than sequential

		if existing, ok := candidates[file]; ok {
			existing.Score = max(existing.Score, score)
		} else {
			candidates[file] = &PrefetchCandidate{
				FilePath: file,
				Score:    score,
				Reason:   "temporal",
				Priority: 2,
			}
		}
	}
}

// predictCollaborative predicts based on other users' patterns.
func (pp *PredictivePrefetch) predictCollaborative(userID, currentFile string, candidates map[string]*PrefetchCandidate) {
	// Find similar users based on access patterns
	for otherUser, patterns := range pp.patterns {
		if otherUser == userID {
			continue
		}

		// Check if other user accessed current file
		for _, p := range patterns {
			if p.FilePath == currentFile {
				// Find what other user accessed after this file
				for _, nextP := range patterns {
					if nextP.AccessTime.After(p.AccessTime) {
						score := 0.5 * pp.config.LearningRate
						if existing, ok := candidates[nextP.FilePath]; ok {
							existing.Score = max(existing.Score, score)
						} else {
							candidates[nextP.FilePath] = &PrefetchCandidate{
								FilePath: nextP.FilePath,
								Score:    score,
								Reason:   "collaborative",
								Priority: 3,
							}
						}
						break
					}
				}
				break
			}
		}
	}
}

// Prefetch prefetches files into cache.
func (pp *PredictivePrefetch) Prefetch(ctx context.Context, candidates []PrefetchCandidate) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	for _, candidate := range candidates {
		// Check if already in cache
		if _, ok := pp.cacheIndex[candidate.FilePath]; ok {
			pp.stats.CacheHits++
			continue
		}

		// Check cache limits
		for pp.cacheSize+candidate.Size > pp.config.MaxCacheSize || pp.cache.Len() >= pp.config.MaxEntries {
			pp.evictOldest()
		}

		// Add to cache
		entry := &CacheEntry{
			FilePath:   candidate.FilePath,
			Size:       candidate.Size,
			CachedAt:   time.Now(),
			AccessCount: 0,
			LastAccess:  time.Now(),
		}
		element := pp.cache.PushFront(entry)
		pp.cacheIndex[candidate.FilePath] = element
		pp.cacheSize += candidate.Size
		pp.stats.PrefetchedBytes += candidate.Size
	}

	return nil
}

// evictOldest evicts the oldest cache entry.
func (pp *PredictivePrefetch) evictOldest() {
	if pp.cache.Len() == 0 {
		return
	}

	element := pp.cache.Back()
	entry := element.Value.(*CacheEntry)

	pp.cache.Remove(element)
	delete(pp.cacheIndex, entry.FilePath)
	pp.cacheSize -= entry.Size
}

// Touch updates cache entry access time.
func (pp *PredictivePrefetch) Touch(filePath string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	if element, ok := pp.cacheIndex[filePath]; ok {
		entry := element.Value.(*CacheEntry)
		entry.AccessCount++
		entry.LastAccess = time.Now()
		pp.cache.MoveToFront(element)
		pp.stats.CacheHits++
	} else {
		pp.stats.CacheMisses++
	}

	// Update hit rate
	total := pp.stats.CacheHits + pp.stats.CacheMisses
	if total > 0 {
		pp.stats.HitRate = float64(pp.stats.CacheHits) / float64(total)
	}
}

// GetCached returns list of cached files.
func (pp *PredictivePrefetch) GetCached() []CacheEntry {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	entries := make([]CacheEntry, 0, pp.cache.Len())
	for e := pp.cache.Front(); e != nil; e = e.Next() {
		entries = append(entries, *e.Value.(*CacheEntry))
	}
	return entries
}

// GetStats returns prefetch statistics.
func (pp *PredictivePrefetch) GetStats() PrefetchStats {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	stats := pp.stats
	if stats.TotalPredictions > 0 {
		stats.Accuracy = float64(stats.CorrectPredictions) / float64(stats.TotalPredictions)
	}
	return stats
}

// SetEnabled enables or disables prefetching.
func (pp *PredictivePrefetch) SetEnabled(enabled bool) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.enabled = enabled
}

// IsEnabled returns whether prefetching is enabled.
func (pp *PredictivePrefetch) IsEnabled() bool {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	return pp.enabled
}

// ClearCache clears the prefetch cache.
func (pp *PredictivePrefetch) ClearCache() {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	pp.cache.Init()
	pp.cacheIndex = make(map[string]*list.Element)
	pp.cacheSize = 0
}

// GetCacheSize returns current cache size.
func (pp *PredictivePrefetch) GetCacheSize() int64 {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	return pp.cacheSize
}

// GetCacheCount returns number of cached entries.
func (pp *PredictivePrefetch) GetCacheCount() int {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	return pp.cache.Len()
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
