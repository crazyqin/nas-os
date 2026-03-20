package optimizer

import (
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"

	"nas-os/internal/cache"
	"nas-os/pkg/safeguards"
)

// PerformanceOptimizer 性能优化器
type PerformanceOptimizer struct {
	mu          sync.RWMutex
	cache       *cache.Manager
	config      *Config
	stats       *Stats
	stopChan    chan struct{}
	initialized bool
}

// Config 优化配置
type Config struct {
	// 缓存配置
	CacheEnabled  bool          `json:"cache_enabled"`
	CacheCapacity int           `json:"cache_capacity"`
	CacheTTL      time.Duration `json:"cache_ttl"`

	// GC 配置
	GCEnabled  bool          `json:"gc_enabled"`
	GCInterval time.Duration `json:"gc_interval"`
	GCMaxPause time.Duration `json:"gc_max_pause"`

	// 池化配置
	PoolEnabled bool `json:"pool_enabled"`

	// 批处理配置
	BatchEnabled bool          `json:"batch_enabled"`
	BatchSize    int           `json:"batch_size"`
	BatchTimeout time.Duration `json:"batch_timeout"`

	// 并发配置
	MaxGoroutines  int `json:"max_goroutines"`
	WorkerPoolSize int `json:"worker_pool_size"`
}

// Stats 性能统计
type Stats struct {
	// 缓存统计
	CacheHits     int64   `json:"cache_hits"`
	CacheMisses   int64   `json:"cache_misses"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`

	// GC 统计
	GCCount      uint32        `json:"gc_count"`
	GCPauseTotal time.Duration `json:"gc_pause_total"`
	GCPauseAvg   time.Duration `json:"gc_pause_avg"`
	LastGCTime   time.Time     `json:"last_gc_time"`

	// 内存统计
	MemAlloc uint64 `json:"mem_alloc"`
	MemTotal uint64 `json:"mem_total"`
	MemSys   uint64 `json:"mem_sys"`
	MemGC    uint64 `json:"mem_gc"`

	// Goroutine 统计
	Goroutines int `json:"goroutines"`

	// 优化统计
	Optimizations int64         `json:"optimizations"`
	TimeSaved     time.Duration `json:"time_saved"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		CacheEnabled:  true,
		CacheCapacity: 10000,
		CacheTTL:      5 * time.Minute,

		GCEnabled:  true,
		GCInterval: 1 * time.Minute,
		GCMaxPause: 10 * time.Millisecond,

		PoolEnabled: true,

		BatchEnabled: true,
		BatchSize:    100,
		BatchTimeout: 100 * time.Millisecond,

		MaxGoroutines:  1000,
		WorkerPoolSize: runtime.NumCPU() * 2,
	}
}

// NewOptimizer 创建性能优化器
func NewOptimizer(cfg *Config, logger *zap.Logger) *PerformanceOptimizer {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 如果未提供 logger，使用 nop logger
	if logger == nil {
		logger = zap.NewNop()
	}

	opt := &PerformanceOptimizer{
		config:   cfg,
		stats:    &Stats{},
		stopChan: make(chan struct{}),
	}

	// 初始化缓存
	if cfg.CacheEnabled {
		opt.cache = cache.NewManager(cfg.CacheCapacity, cfg.CacheTTL, logger)
	}

	// 初始化 GC 调优
	if cfg.GCEnabled {
		opt.tuneGC()
	}

	// 启动监控
	go opt.startMonitoring()

	opt.initialized = true
	return opt
}

// tuneGC 优化 GC 参数
func (opt *PerformanceOptimizer) tuneGC() {
	// 设置 GC 目标：100% 增长才触发 GC（默认值）
	runtime.GC()
}

// startMonitoring 启动性能监控
func (opt *PerformanceOptimizer) startMonitoring() {
	ticker := time.NewTicker(opt.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			opt.updateStats()
		case <-opt.stopChan:
			return
		}
	}
}

// updateStats 更新统计信息
func (opt *PerformanceOptimizer) updateStats() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	opt.mu.Lock()
	defer opt.mu.Unlock()

	// 内存统计
	opt.stats.MemAlloc = memStats.Alloc
	opt.stats.MemTotal = memStats.TotalAlloc
	opt.stats.MemSys = memStats.Sys
	opt.stats.MemGC = memStats.GCSys

	// GC 统计
	opt.stats.GCCount = memStats.NumGC
	// 安全转换 uint64 到 int64，避免溢出
	pauseTotal, err := safeguards.SafeUint64ToInt64(memStats.PauseTotalNs)
	if err != nil {
		// 溢出时使用 MaxInt64
		pauseTotal = int64(1<<63 - 1)
	}
	opt.stats.GCPauseTotal = time.Duration(pauseTotal)
	if memStats.NumGC > 0 {
		opt.stats.GCPauseAvg = time.Duration(pauseTotal) / time.Duration(memStats.NumGC)
	}

	// Goroutine 统计
	opt.stats.Goroutines = runtime.NumGoroutine()

	// 缓存统计
	if opt.cache != nil {
		cacheStats := opt.cache.GetStats()
		opt.stats.CacheHits = cacheStats.Hits
		opt.stats.CacheMisses = cacheStats.Misses
		if cacheStats.Hits+cacheStats.Misses > 0 {
			opt.stats.CacheHitRatio = float64(cacheStats.Hits) / float64(cacheStats.Hits+cacheStats.Misses)
		}
	}
}

// GetCache 获取缓存管理器
func (opt *PerformanceOptimizer) GetCache() *cache.Manager {
	return opt.cache
}

// CacheGet 从缓存获取（带统计）
func (opt *PerformanceOptimizer) CacheGet(key string) (interface{}, bool) {
	if opt.cache == nil {
		return nil, false
	}
	return opt.cache.Get(key)
}

// CacheSet 设置缓存（带统计）
func (opt *PerformanceOptimizer) CacheSet(key string, value interface{}) {
	if opt.cache == nil {
		return
	}
	opt.cache.Set(key, value)
}

// CacheDelete 删除缓存
func (opt *PerformanceOptimizer) CacheDelete(key string) {
	if opt.cache == nil {
		return
	}
	opt.cache.Delete(key)
}

// GetStats 获取统计信息
func (opt *PerformanceOptimizer) GetStats() *Stats {
	opt.mu.RLock()
	defer opt.mu.RUnlock()

	stats := *opt.stats
	return &stats
}

// OptimizeQuery 优化查询（缓存结果）
func (opt *PerformanceOptimizer) OptimizeQuery(key string, queryFn func() (interface{}, error)) (interface{}, error) {
	// 尝试从缓存获取
	if val, ok := opt.CacheGet(key); ok {
		return val, nil
	}

	// 执行查询
	result, err := queryFn()
	if err != nil {
		return nil, err
	}

	// 缓存结果
	opt.CacheSet(key, result)

	opt.mu.Lock()
	opt.stats.Optimizations++
	opt.mu.Unlock()

	return result, nil
}

// BatchProcess 批处理优化
func (opt *PerformanceOptimizer) BatchProcess(items []interface{}, processFn func(interface{}) error) error {
	if !opt.config.BatchEnabled {
		// 不启用批处理，逐个处理
		for _, item := range items {
			if err := processFn(item); err != nil {
				return err
			}
		}
		return nil
	}

	// 批处理
	batchSize := opt.config.BatchSize
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		for _, item := range batch {
			if err := processFn(item); err != nil {
				return err
			}
		}

		// 批次间短暂暂停，避免阻塞
		if end < len(items) {
			time.Sleep(opt.config.BatchTimeout)
		}
	}

	return nil
}

// WorkerPool 工作池
func (opt *PerformanceOptimizer) WorkerPool(jobs <-chan func(), results chan<- interface{}) {
	poolSize := opt.config.WorkerPoolSize
	var wg sync.WaitGroup

	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				job()
			}
		}()
	}

	wg.Wait()
	close(results)
}

// ForceGC 强制 GC（谨慎使用）
func (opt *PerformanceOptimizer) ForceGC() {
	start := time.Now()
	runtime.GC()

	opt.mu.Lock()
	opt.stats.LastGCTime = start
	opt.mu.Unlock()

	// log.Printf("GC completed in %v", time.Since(start))
}

// Stop 停止优化器
func (opt *PerformanceOptimizer) Stop() {
	close(opt.stopChan)
}

// GetConfig 获取配置
func (opt *PerformanceOptimizer) GetConfig() *Config {
	return opt.config
}

// UpdateConfig 更新配置
func (opt *PerformanceOptimizer) UpdateConfig(cfg *Config) {
	opt.mu.Lock()
	defer opt.mu.Unlock()
	opt.config = cfg
}
