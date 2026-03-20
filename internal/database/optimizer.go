package database

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// validTableNameRegex validates SQL table names to prevent injection
var validTableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateTableName checks if a table name is safe to use in SQL
func validateTableName(table string) error {
	if !validTableNameRegex.MatchString(table) {
		return fmt.Errorf("invalid table name: %s (must match ^[a-zA-Z_][a-zA-Z0-9_]*$)", table)
	}
	if len(table) > 128 {
		return fmt.Errorf("table name too long: %s (max 128 chars)", table)
	}
	return nil
}

// Optimizer handles SQLite performance optimizations
type Optimizer struct {
	db         *sql.DB
	queryCache *QueryCache
	mu         sync.RWMutex
	walEnabled bool
	pragmas    map[string]interface{}

	// Statistics
	queryCount  int64
	cacheHits   int64
	cacheMisses int64
	slowQueries int64

	// 慢查询阈值 (可配置)
	slowThreshold time.Duration

	logger *zap.Logger
}

// QueryCacheEntry represents a cached query result
type QueryCacheEntry struct {
	key       string
	result    interface{}
	expiresAt time.Time
	elem      *list.Element // LRU list element
}

// QueryCache implements query result caching with LRU eviction
type QueryCache struct {
	cache   map[string]*QueryCacheEntry
	lru     *list.List // LRU list, front = most recent
	mu      sync.RWMutex
	ttl     time.Duration
	maxSize int

	// 统计
	hits   int64
	misses int64
}

// NewQueryCache creates a new query cache with LRU eviction
func NewQueryCache(ttl time.Duration, maxSize int) *QueryCache {
	qc := &QueryCache{
		cache:   make(map[string]*QueryCacheEntry),
		lru:     list.New(),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// Start cleanup goroutine
	go qc.startCleanup()

	return qc
}

// Get retrieves a cached query result
func (qc *QueryCache) Get(key string) (interface{}, bool) {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	entry, ok := qc.cache[key]
	if !ok {
		atomic.AddInt64(&qc.misses, 1)
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		qc.removeEntry(entry)
		atomic.AddInt64(&qc.misses, 1)
		return nil, false
	}

	// Move to front (most recently used)
	qc.lru.MoveToFront(entry.elem)
	atomic.AddInt64(&qc.hits, 1)

	return entry.result, true
}

// Set stores a query result in cache with LRU eviction
func (qc *QueryCache) Set(key string, result interface{}) {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	now := time.Now()

	// Check if key already exists
	if entry, ok := qc.cache[key]; ok {
		entry.result = result
		entry.expiresAt = now.Add(qc.ttl)
		qc.lru.MoveToFront(entry.elem)
		return
	}

	// Evict if at capacity (LRU eviction)
	for qc.lru.Len() >= qc.maxSize {
		qc.evictLRU()
	}

	// Add new entry
	entry := &QueryCacheEntry{
		key:       key,
		result:    result,
		expiresAt: now.Add(qc.ttl),
	}
	elem := qc.lru.PushFront(key)
	entry.elem = elem
	qc.cache[key] = entry
}

// Delete removes a key from cache
func (qc *QueryCache) Delete(key string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	if entry, ok := qc.cache[key]; ok {
		qc.removeEntry(entry)
	}
}

// Clear clears all cached entries
func (qc *QueryCache) Clear() {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	qc.cache = make(map[string]*QueryCacheEntry)
	qc.lru = list.New()
}

// Len returns the number of cached entries
func (qc *QueryCache) Len() int {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return qc.lru.Len()
}

// Stats returns cache statistics
func (qc *QueryCache) Stats() CacheStats {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	hits := atomic.LoadInt64(&qc.hits)
	misses := atomic.LoadInt64(&qc.misses)
	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Size:    len(qc.cache),
		MaxSize: qc.maxSize,
		TTL:     qc.ttl,
		Hits:    hits,
		Misses:  misses,
		HitRate: hitRate,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Size    int           `json:"size"`
	MaxSize int           `json:"max_size"`
	TTL     time.Duration `json:"ttl"`
	Hits    int64         `json:"hits"`
	Misses  int64         `json:"misses"`
	HitRate float64       `json:"hitRate"`
}

// removeEntry removes an entry from both the cache and LRU list
func (qc *QueryCache) removeEntry(entry *QueryCacheEntry) {
	delete(qc.cache, entry.key)
	if entry.elem != nil {
		qc.lru.Remove(entry.elem)
	}
}

// evictLRU removes the least recently used entry
func (qc *QueryCache) evictLRU() {
	elem := qc.lru.Back()
	if elem == nil {
		return
	}
	key, ok := elem.Value.(string)
	if !ok {
		return
	}
	if entry, ok := qc.cache[key]; ok {
		qc.removeEntry(entry)
	}
}

// startCleanup periodically removes expired entries
func (qc *QueryCache) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		qc.mu.Lock()
		now := time.Now()
		// Iterate from back (oldest) to front (newest) for efficiency
		for elem := qc.lru.Back(); elem != nil; {
			key, ok := elem.Value.(string)
			if !ok {
				elem = elem.Prev()
				continue
			}
			if entry, ok := qc.cache[key]; ok {
				if now.After(entry.expiresAt) {
					next := elem.Prev()
					qc.removeEntry(entry)
					elem = next
					continue
				}
			}
			elem = elem.Prev()
		}
		qc.mu.Unlock()
	}
}

// NewOptimizer creates a new database optimizer
func NewOptimizer(db *sql.DB, logger *zap.Logger) *Optimizer {
	return &Optimizer{
		db:            db,
		queryCache:    NewQueryCache(5*time.Minute, 1000),
		pragmas:       make(map[string]interface{}),
		slowThreshold: 100 * time.Millisecond,
		logger:        logger,
	}
}

// SetSlowThreshold sets the slow query threshold
func (o *Optimizer) SetSlowThreshold(threshold time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.slowThreshold = threshold
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
		"synchronous":        "NORMAL",  // Faster than FULL, safe with WAL
		"cache_size":         -64000,    // 64MB cache (negative = KB)
		"temp_store":         "MEMORY",  // Store temp tables in memory
		"mmap_size":          268435456, // 256MB memory-mapped I/O
		"wal_autocheckpoint": 1000,      // Checkpoint after 1000 pages
		"busy_timeout":       5000,      // 5 second busy timeout
		"foreign_keys":       "ON",      // Enable foreign key constraints
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
	if err := validateTableName(table); err != nil {
		return err
	}
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
// Note: Returns cached data as slice of maps, not *sql.Rows (which cannot be safely cached)
func (o *Optimizer) QueryWithCache(query string, args ...interface{}) ([]map[string]interface{}, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("%s|%v", query, args)

	// Check cache first
	if cached, ok := o.queryCache.Get(cacheKey); ok {
		atomic.AddInt64(&o.cacheHits, 1)
		result, ok := cached.([]map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid cache entry type")
		}
		return result, nil
	}

	atomic.AddInt64(&o.cacheMisses, 1)

	// Execute query
	rows, err := o.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Read all rows into memory
	var results []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{}'s to represent each column
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Convert to map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Cache result
	o.queryCache.Set(cacheKey, results)

	return results, nil
}

// ExecWithTiming executes a statement and logs slow queries
func (o *Optimizer) ExecWithTiming(query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()

	result, err := o.db.Exec(query, args...)

	duration := time.Since(start)
	atomic.AddInt64(&o.queryCount, 1)

	if duration > o.slowThreshold {
		atomic.AddInt64(&o.slowQueries, 1)
		o.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}

	return result, err
}

// QueryWithTiming executes a query and logs slow queries
func (o *Optimizer) QueryWithTiming(query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()

	rows, err := o.db.Query(query, args...)

	duration := time.Since(start)
	atomic.AddInt64(&o.queryCount, 1)

	if duration > o.slowThreshold {
		atomic.AddInt64(&o.slowQueries, 1)
		o.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}

	return rows, err
}

// Stats returns optimizer statistics
func (o *Optimizer) Stats() OptimizerStats {
	cacheStats := o.queryCache.Stats()

	return OptimizerStats{
		QueryCount:   atomic.LoadInt64(&o.queryCount),
		CacheHits:    cacheStats.Hits,
		CacheMisses:  cacheStats.Misses,
		CacheHitRate: cacheStats.HitRate,
		SlowQueries:  atomic.LoadInt64(&o.slowQueries),
		WALEnabled:   o.walEnabled,
		CacheSize:    cacheStats.Size,
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

// QueryContextWithTiming executes a query with context and logs slow queries
func (o *Optimizer) QueryContextWithTiming(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()

	rows, err := o.db.QueryContext(ctx, query, args...)

	duration := time.Since(start)
	atomic.AddInt64(&o.queryCount, 1)

	if duration > o.slowThreshold {
		atomic.AddInt64(&o.slowQueries, 1)
		o.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}

	return rows, err
}

// ExecContextWithTiming executes a statement with context and logs slow queries
func (o *Optimizer) ExecContextWithTiming(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()

	result, err := o.db.ExecContext(ctx, query, args...)

	duration := time.Since(start)
	atomic.AddInt64(&o.queryCount, 1)

	if duration > o.slowThreshold {
		atomic.AddInt64(&o.slowQueries, 1)
		o.logger.Warn("Slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}

	return result, err
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
	defer func() { _ = rows.Close() }()

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
