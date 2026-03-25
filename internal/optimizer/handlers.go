package optimizer

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 性能优化 HTTP 处理器.
type Handlers struct {
	optimizer *PerformanceOptimizer
}

// NewHandlers 创建处理器.
func NewHandlers(opt *PerformanceOptimizer) *Handlers {
	return &Handlers{optimizer: opt}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	opt := api.Group("/optimizer")
	{
		opt.GET("/stats", h.getStats)
		opt.GET("/config", h.getConfig)
		opt.PUT("/config", h.updateConfig)
		opt.POST("/gc", h.forceGC)
		opt.GET("/memory", h.getMemoryInfo)
		opt.GET("/goroutines", h.getGoroutines)
		opt.POST("/cache/clear", h.clearCache)
	}
}

// APIResponse 通用响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func apiError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// getStats 获取性能统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.optimizer.GetStats()

	// 添加额外信息
	response := map[string]interface{}{
		"cache": map[string]interface{}{
			"hits":      stats.CacheHits,
			"misses":    stats.CacheMisses,
			"hit_ratio": stats.CacheHitRatio,
		},
		"gc": map[string]interface{}{
			"count":       stats.GCCount,
			"pause_total": stats.GCPauseTotal.String(),
			"pause_avg":   stats.GCPauseAvg.String(),
			"last_gc":     stats.LastGCTime,
		},
		"memory": map[string]interface{}{
			"alloc":  stats.MemAlloc,
			"total":  stats.MemTotal,
			"sys":    stats.MemSys,
			"gc_mem": stats.MemGC,
		},
		"goroutines": stats.Goroutines,
		"optimizations": map[string]interface{}{
			"count":      stats.Optimizations,
			"time_saved": stats.TimeSaved.String(),
		},
	}

	c.JSON(http.StatusOK, success(response))
}

// getConfig 获取优化配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.optimizer.GetConfig()
	c.JSON(http.StatusOK, success(cfg))
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	CacheEnabled   *bool          `json:"cache_enabled"`
	CacheCapacity  *int           `json:"cache_capacity"`
	CacheTTL       *time.Duration `json:"cache_ttl"`
	GCEnabled      *bool          `json:"gc_enabled"`
	GCInterval     *time.Duration `json:"gc_interval"`
	BatchEnabled   *bool          `json:"batch_enabled"`
	BatchSize      *int           `json:"batch_size"`
	MaxGoroutines  *int           `json:"max_goroutines"`
	WorkerPoolSize *int           `json:"worker_pool_size"`
}

// updateConfig 更新优化配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	cfg := h.optimizer.GetConfig()

	// 更新字段
	if req.CacheEnabled != nil {
		cfg.CacheEnabled = *req.CacheEnabled
	}
	if req.CacheCapacity != nil {
		cfg.CacheCapacity = *req.CacheCapacity
	}
	if req.CacheTTL != nil {
		cfg.CacheTTL = *req.CacheTTL
	}
	if req.GCEnabled != nil {
		cfg.GCEnabled = *req.GCEnabled
	}
	if req.GCInterval != nil {
		cfg.GCInterval = *req.GCInterval
	}
	if req.BatchEnabled != nil {
		cfg.BatchEnabled = *req.BatchEnabled
	}
	if req.BatchSize != nil {
		cfg.BatchSize = *req.BatchSize
	}
	if req.MaxGoroutines != nil {
		cfg.MaxGoroutines = *req.MaxGoroutines
	}
	if req.WorkerPoolSize != nil {
		cfg.WorkerPoolSize = *req.WorkerPoolSize
	}

	h.optimizer.UpdateConfig(cfg)

	c.JSON(http.StatusOK, success(cfg))
}

// forceGC 强制 GC.
func (h *Handlers) forceGC(c *gin.Context) {
	h.optimizer.ForceGC()
	c.JSON(http.StatusOK, success(map[string]string{
		"status": "GC triggered",
	}))
}

// getMemoryInfo 获取内存详情.
func (h *Handlers) getMemoryInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	response := map[string]interface{}{
		"alloc":             memStats.Alloc,
		"total_alloc":       memStats.TotalAlloc,
		"sys":               memStats.Sys,
		"heap_alloc":        memStats.HeapAlloc,
		"heap_sys":          memStats.HeapSys,
		"heap_idle":         memStats.HeapIdle,
		"heap_in_use":       memStats.HeapInuse,
		"heap_released":     memStats.HeapReleased,
		"heap_objects":      memStats.HeapObjects,
		"stack_in_use":      memStats.StackInuse,
		"stack_sys":         memStats.StackSys,
		"mspan_in_use":      memStats.MSpanInuse,
		"mspan_sys":         memStats.MSpanSys,
		"mcache_in_use":     memStats.MCacheInuse,
		"mcache_sys":        memStats.MCacheSys,
		"gc_sys":            memStats.GCSys,
		"other_sys":         memStats.OtherSys,
		"num_gc":            memStats.NumGC,
		"gc_pause_total_ns": memStats.PauseTotalNs,
		"last_gc_time":      memStats.LastGC,
	}

	c.JSON(http.StatusOK, success(response))
}

// getGoroutines 获取 Goroutine 详情.
func (h *Handlers) getGoroutines(c *gin.Context) {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)

	c.JSON(http.StatusOK, success(map[string]interface{}{
		"count":  runtime.NumGoroutine(),
		"stacks": string(buf[:n]),
	}))
}

// clearCache 清空缓存.
func (h *Handlers) clearCache(c *gin.Context) {
	cache := h.optimizer.GetCache()
	if cache != nil {
		cache.Clear()
	}
	c.JSON(http.StatusOK, success(map[string]string{
		"status": "cache cleared",
	}))
}

// PerformanceMiddleware 性能监控中间件.
func PerformanceMiddleware(opt *PerformanceOptimizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录耗时
		duration := time.Since(start)

		// 慢请求日志
		if duration > 500*time.Millisecond {
			// log.Printf("[SLOW] %s %s took %v", c.Request.Method, c.Request.URL.Path, duration)
			_ = duration // 预留慢请求日志
		}

		// 添加响应头
		c.Header("X-Response-Time", duration.String())
	}
}
