package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Optimizer handles SQLite performance optimizations
type Optimizer struct {
	db           *sql.DB
	queryCache   *QueryCache
	mu           sync.RWMutex
	walEnabled   bool
	pragmas      map[string]interface{}
	
	// Statistics
	queryCount    int64
	cacheHits     int64
	cacheMisses   int64
	slowQueries   int64
	
	logger *zap.Logger
}

// QueryCacheEntry represents a cached query result
type QueryCacheEntry struct {
	Result    interface{}
	ExpiresAt time.Time
}

// QueryCache implements query result caching
type QueryCache struct {
	cache  map[string]*QueryCacheEntry
	mu     sync.RWMutex
	ttl    time.Duration
	maxSize int
}

// NewQueryCache creates a new query cache
func NewQueryCache(ttl time.Duration, maxSize int) *QueryCache {
	qc := &QueryCache{
		cache:   make(map[string]*QueryCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
	
	// Start cleanup goroutine
	go qc.startCleanup()
	
	return qc
}

// Get retrieves a cached query result
func (qc *QueryCache) Get(key string) (interface{}, bool) {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	
	entry, ok := qc.cache[key]
	if !ok {
		return nil, false
	}
	
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	
	return entry.Result, true
}

// Set stores a query result in cache
func (qc *QueryCache) Set(key string, result interface{}) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	
	// Evict if at capacity
	if len(qc.cache) >= qc.maxSize {
		qc.evictOldest()
	}
	
	qc.cache[key] = &QueryCacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(qc.ttl),
	}
}

// Delete removes a key from cache
func (qc *QueryCache) Delete(key string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	delete(qc.cache, key)
}

// Clear clears all cached entries
func (qc *QueryCache) Clear() {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.cache = make(map[string]*QueryCacheEntry)
}

// Len returns the number of cached entries
func (qc *QueryCache) Len() int {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return len(qc.cache)
}

// Stats returns cache statistics
func (qc *QueryCache) Stats() CacheStats {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	
	return CacheStats{
		Size:    len(qc.cache),
		MaxSize: qc.maxSize,
		TTL:     qc.ttl,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Size    int           `json:"size"`
	MaxSize int           `json:"max_size"`
	TTL     time.Duration `json:"ttl"`
}

// evictOldest removes the oldest entry (simple implementation)
func (qc *QueryCache) evictOldest() {
	// For simplicity, just remove a random entry
	// In production, use LRU or similar
	for key := range qc.cache {
		delete(qc.cache, key)
		return
	}
}

// startCleanup periodically removes expired entries
func (qc *QueryCache) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		qc.mu.Lock()
		now := time.Now()
		for key, entry := range qc.cache {
			if now.After(entry.ExpiresAt) {
				delete(qc.cache, key)
			}
		}
		qc.mu.Unlock()
	}
}

// NewOptimizer creates a new database optimizer
func NewOptimizer(db *sql.DB, logger *zap.Logger) *Optimizer {
	return &Optimizer{
		db:         db,
		queryCache: NewQueryCache(5*time.Minute, 1000),
		pragmas:    make(map[string]interface{}),
		logger:     logger,
	}
}

// EnableWAL enables Write-Ahead Logging mode
func (o *Optimizer) EnableWAL() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	_, err := o.db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("failed to enable WAL: %w", err)
	}
	
	o.walEnabled = true
	o.logger.Info("SQLite WAL mode enabled")
	return nil
}

// ConfigurePerformance applies performance-optimized PRAGMAs
func (o *Optimizer) ConfigurePerformance() error {
	pragmas := map[string]interface{}{
		"synchronous":        "NORMAL",      // Faster than FULL, safe with WAL
		"cache_size":         -64000,       // 64MB cache (negative = KB)
		"temp_store":         "MEMORY",     // Store temp tables in memory
		"mmap_size":          268435456,    // 256MB memory-mapped I/O
		"wal_autocheckpoint": 1000,         // Checkpoint after 1000 pages
		"busy_timeout":       5000,         // 5 second busy timeout
		"foreign_keys":       "ON",         // Enable foreign key constraints
	}
	
	for pragma, value := range pragmas {
		query := fmt.Sprintf("PRAGMA %s = %v", pragma, value)
		if _, err := o.db.Exec(query); err != nil {
			o.logger.Warn("Failed to set PRAGMA", 
				zap.String("pragma", pragma), 
				zap.Error(err))
		} else {
			o.pragmas[pragma] = value
		}
	}
	
	o.logger.Info("Database performance PRAGMAs configured")
	return nil
}

// CreateIndex creates an index if it doesn't exist
func (o *Optimizer) CreateIndex(table, name, columns string) error {
	query := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
		name, table, columns,
	)
	
	_, err := o.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", name, err)
	}
	
	o.logger.Info("Index created", 
		zap.String("table", table),
		zap.String("name", name),
		zap.String("columns", columns))
	
	return nil
}

// CreateCompositeIndex creates a composite index
func (o *Optimizer) CreateCompositeIndex(table, name string, columns ...string) error {
	return o.CreateIndex(table, name, joinColumns(columns))
}

// AnalyzeTable runs ANALYZE on a table
func (o *Optimizer) AnalyzeTable(table string) error {
	_, err := o.db.Exec(fmt.Sprintf("ANALYZE %s", table))
	if err != nil {
		return fmt.Errorf("failed to analyze table %s: %w", table, err)
	}
	
	o.logger.Info("Table analyzed", zap.String("table", table))
	return nil
}

// AnalyzeAll runs ANALYZE on all tables
func (o *Optimizer) AnalyzeAll() error {
	_, err := o.db.Exec("ANALYZE")
	if err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}
	
	o.logger.Info("Database analyzed")
	return nil
}

// QueryWithCache executes a query with caching
func (o *Optimizer) QueryWithCache(query string, args ...interface{}) (*sql.Rows, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("%s|%v", query, args)
	
	// Check cache first
	if cached, ok := o.queryCache.Get(cacheKey); ok {
		o.mu.Lock()
		o.cacheHits++
		o.mu.Unlock()
		return cached.(*sql.Rows), nil
	}
	
	o.mu.Lock()
	o.cacheMisses++
	o.mu.Unlock()
	
	// Execute query
	rows, err := o.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	
	// Cache result (note: this is simplified, real implementation would cache data)
	o.queryCache.Set(cacheKey, rows)
	
	return rows, nil
}

// ExecWithTiming executes a statement and logs slow queries
func (o *Optimizer) ExecWithTiming(query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	
	result, err := o.db.Exec(query, args...)
	
	duration := time.Since(start)
	o.mu.Lock()
	o.queryCount++
	if duration > 100*time.Millisecond {
		o.slowQueries++
		o.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}
	o.mu.Unlock()
	
	return result, err
}

// QueryWithTiming executes a query and logs slow queries
func (o *Optimizer) QueryWithTiming(query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	
	rows, err := o.db.Query(query, args...)
	
	duration := time.Since(start)
	o.mu.Lock()
	o.queryCount++
	if duration > 100*time.Millisecond {
		o.slowQueries++
		o.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}
	o.mu.Unlock()
	
	return rows, err
}

// Stats returns optimizer statistics
func (o *Optimizer) Stats() OptimizerStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	cacheHitRate := float64(0)
	total := o.cacheHits + o.cacheMisses
	if total > 0 {
		cacheHitRate = float64(o.cacheHits) / float64(total) * 100
	}
	
	return OptimizerStats{
		QueryCount:    o.queryCount,
		CacheHits:     o.cacheHits,
		CacheMisses:   o.cacheMisses,
		CacheHitRate:  cacheHitRate,
		SlowQueries:   o.slowQueries,
		WALEnabled:    o.walEnabled,
		CacheSize:     o.queryCache.Len(),
	}
}

// OptimizerStats holds optimizer statistics
type OptimizerStats struct {
	QueryCount   int64   `json:"query_count"`
	CacheHits    int64   `json:"cache_hits"`
	CacheMisses  int64   `json:"cache_misses"`
	CacheHitRate float64 `json:"cache_hit_rate"`
	SlowQueries  int64   `json:"slow_queries"`
	WALEnabled   bool    `json:"wal_enabled"`
	CacheSize    int     `json:"cache_size"`
}

// Vacuum runs VACUUM to reclaim space
func (o *Optimizer) Vacuum() error {
	_, err := o.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	
	o.logger.Info("Database vacuumed")
	return nil
}

// Checkpoint runs a WAL checkpoint
func (o *Optimizer) Checkpoint() error {
	if !o.walEnabled {
		return nil
	}
	
	_, err := o.db.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	if err != nil {
		return fmt.Errorf("failed to checkpoint: %w", err)
	}
	
	o.logger.Debug("WAL checkpoint completed")
	return nil
}

// GetIndexes returns all indexes in the database
func (o *Optimizer) GetIndexes() ([]string, error) {
	rows, err := o.db.Query(`
		SELECT name, tbl_name, sql 
		FROM sqlite_master 
		WHERE type='index' AND sql IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var indexes []string
	for rows.Next() {
		var name, tblName string
		var sql sql.NullString
		if err := rows.Scan(&name, &tblName, &sql); err != nil {
			return nil, err
		}
		indexes = append(indexes, fmt.Sprintf("%s on %s", name, tblName))
	}
	
	return indexes, rows.Err()
}

// Close closes the optimizer
func (o *Optimizer) Close() {
	o.queryCache.Clear()
}

func joinColumns(cols []string) string {
	result := ""
	for i, col := range cols {
		if i > 0 {
			result += ", "
		}
		result += col
	}
	return result
}
