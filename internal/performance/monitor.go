package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Metrics 性能指标
type Metrics struct {
	// API 性能指标
	RequestCount    int64         `json:"requestCount"`
	SuccessCount    int64         `json:"successCount"`
	ErrorCount      int64         `json:"errorCount"`
	AvgLatency      time.Duration `json:"avgLatency"`
	MaxLatency      time.Duration `json:"maxLatency"`
	MinLatency      time.Duration `json:"minLatency"`
	P50Latency      time.Duration `json:"p50Latency"`
	P95Latency      time.Duration `json:"p95Latency"`
	P99Latency      time.Duration `json:"p99Latency"`

	// 文件操作指标
	FileListCount     int64 `json:"fileListCount"`
	FileListAvgTime   int64 `json:"fileListAvgTime"` // ms
	ThumbnailCount    int64 `json:"thumbnailCount"`
	ThumbnailAvgTime  int64 `json:"thumbnailAvgTime"` // ms
	UploadCount       int64 `json:"uploadCount"`
	UploadBytes       int64 `json:"uploadBytes"`
	DownloadCount     int64 `json:"downloadCount"`
	DownloadBytes     int64 `json:"downloadBytes"`

	// 搜索指标
	SearchCount      int64 `json:"searchCount"`
	SearchAvgTime    int64 `json:"searchAvgTime"` // ms
	IndexCount       int64 `json:"indexCount"`
	IndexAvgTime     int64 `json:"indexAvgTime"` // ms

	// 缓存指标
	CacheHits    int64 `json:"cacheHits"`
	CacheMisses  int64 `json:"cacheMisses"`
	CacheHitRate float64 `json:"cacheHitRate"`

	// 数据库指标
	DBQueries     int64 `json:"dbQueries"`
	DBSlowQueries int64 `json:"dbSlowQueries"`
	DBAvgTime     int64 `json:"dbAvgTime"` // ms

	// 系统指标
	GoroutineCount int64  `json:"goroutineCount"`
	MemoryAllocMB  uint64 `json:"memoryAllocMB"`
	MemorySysMB    uint64 `json:"memorySysMB"`
	GCPauseMs      uint64 `json:"gcPauseMs"`
}

// APIMetric API 调用指标
type APIMetric struct {
	Path       string
	Method     string
	Duration   time.Duration
	StatusCode int
	Error      error
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	logger *zap.Logger

	// 计数器
	requestCount   int64
	successCount   int64
	errorCount     int64

	// 延迟统计
	latencies    []time.Duration
	latencySum   int64
	maxLatency   int64
	minLatency   int64
	latencyMutex sync.RWMutex

	// 文件操作
	fileListCount    int64
	fileListTimeSum  int64
	thumbnailCount   int64
	thumbnailTimeSum int64
	uploadCount      int64
	uploadBytes      int64
	downloadCount    int64
	downloadBytes    int64

	// 搜索
	searchCount     int64
	searchTimeSum   int64
	indexCount      int64
	indexTimeSum    int64

	// 缓存
	cacheHits   int64
	cacheMisses int64

	// 数据库
	dbQueries     int64
	dbSlowQueries int64
	dbTimeSum     int64

	// 慢请求阈值
	slowThreshold time.Duration

	// 采样率 (0-1, 1表示100%采样)
	sampleRate float64
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(logger *zap.Logger) *PerformanceMonitor {
	return &PerformanceMonitor{
		logger:        logger,
		latencies:     make([]time.Duration, 0, 10000),
		minLatency:    int64(time.Hour), // 初始化为最大值
		slowThreshold: 200 * time.Millisecond,
		sampleRate:    1.0,
	}
}

// Middleware 性能监控中间件
func (pm *PerformanceMonitor) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 执行请求
		c.Next()

		// 记录指标
		duration := time.Since(start)
		pm.RecordAPICall(c.Request.URL.Path, c.Request.Method, duration, c.Writer.Status())
	}
}

// RecordAPICall 记录 API 调用
func (pm *PerformanceMonitor) RecordAPICall(path, method string, duration time.Duration, statusCode int) {
	atomic.AddInt64(&pm.requestCount, 1)

	dMs := int64(duration)

	// 更新延迟统计
	pm.latencyMutex.Lock()
	pm.latencies = append(pm.latencies, duration)
	pm.latencySum += dMs
	if dMs > pm.maxLatency {
		pm.maxLatency = dMs
	}
	if dMs < pm.minLatency {
		pm.minLatency = dMs
	}
	// 限制内存使用，保留最近 10000 条
	if len(pm.latencies) > 10000 {
		pm.latencies = pm.latencies[5000:]
	}
	pm.latencyMutex.Unlock()

	// 记录成功/失败
	if statusCode >= 200 && statusCode < 400 {
		atomic.AddInt64(&pm.successCount, 1)
	} else {
		atomic.AddInt64(&pm.errorCount, 1)
	}

	// 慢请求告警
	if duration > pm.slowThreshold {
		pm.logger.Warn("慢请求检测",
			zap.String("path", path),
			zap.String("method", method),
			zap.Duration("duration", duration),
			zap.Int("status", statusCode))
	}
}

// RecordFileList 记录文件列表操作
func (pm *PerformanceMonitor) RecordFileList(duration time.Duration) {
	atomic.AddInt64(&pm.fileListCount, 1)
	atomic.AddInt64(&pm.fileListTimeSum, int64(duration))
}

// RecordThumbnail 记录缩略图生成
func (pm *PerformanceMonitor) RecordThumbnail(duration time.Duration) {
	atomic.AddInt64(&pm.thumbnailCount, 1)
	atomic.AddInt64(&pm.thumbnailTimeSum, int64(duration))
}

// RecordUpload 记录上传
func (pm *PerformanceMonitor) RecordUpload(bytes int64) {
	atomic.AddInt64(&pm.uploadCount, 1)
	atomic.AddInt64(&pm.uploadBytes, bytes)
}

// RecordDownload 记录下载
func (pm *PerformanceMonitor) RecordDownload(bytes int64) {
	atomic.AddInt64(&pm.downloadCount, 1)
	atomic.AddInt64(&pm.downloadBytes, bytes)
}

// RecordSearch 记录搜索
func (pm *PerformanceMonitor) RecordSearch(duration time.Duration) {
	atomic.AddInt64(&pm.searchCount, 1)
	atomic.AddInt64(&pm.searchTimeSum, int64(duration))
}

// RecordIndex 记录索引操作
func (pm *PerformanceMonitor) RecordIndex(duration time.Duration) {
	atomic.AddInt64(&pm.indexCount, 1)
	atomic.AddInt64(&pm.indexTimeSum, int64(duration))
}

// RecordCacheHit 记录缓存命中
func (pm *PerformanceMonitor) RecordCacheHit() {
	atomic.AddInt64(&pm.cacheHits, 1)
}

// RecordCacheMiss 记录缓存未命中
func (pm *PerformanceMonitor) RecordCacheMiss() {
	atomic.AddInt64(&pm.cacheMisses, 1)
}

// RecordDBQuery 记录数据库查询
func (pm *PerformanceMonitor) RecordDBQuery(duration time.Duration, isSlow bool) {
	atomic.AddInt64(&pm.dbQueries, 1)
	atomic.AddInt64(&pm.dbTimeSum, int64(duration))
	if isSlow {
		atomic.AddInt64(&pm.dbSlowQueries, 1)
	}
}

// GetMetrics 获取当前指标
func (pm *PerformanceMonitor) GetMetrics() *Metrics {
	pm.latencyMutex.RLock()
	latencies := make([]time.Duration, len(pm.latencies))
	copy(latencies, pm.latencies)
	pm.latencyMutex.RUnlock()

	// 计算百分位
	var p50, p95, p99 time.Duration
	if len(latencies) > 0 {
		// 简单排序计算百分位
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		pm.quickSort(sorted)
		p50 = sorted[len(sorted)*50/100]
		p95 = sorted[len(sorted)*95/100]
		p99 = sorted[len(sorted)*99/100]
	}

	// 计算缓存命中率
	cacheHits := atomic.LoadInt64(&pm.cacheHits)
	cacheMisses := atomic.LoadInt64(&pm.cacheMisses)
	var cacheHitRate float64
	total := cacheHits + cacheMisses
	if total > 0 {
		cacheHitRate = float64(cacheHits) / float64(total) * 100
	}

	// 获取内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	requestCount := atomic.LoadInt64(&pm.requestCount)
	fileListCount := atomic.LoadInt64(&pm.fileListCount)
	searchCount := atomic.LoadInt64(&pm.searchCount)
	dbQueries := atomic.LoadInt64(&pm.dbQueries)

	var avgLatency time.Duration
	if requestCount > 0 {
		avgLatency = time.Duration(atomic.LoadInt64(&pm.latencySum) / requestCount)
	}

	return &Metrics{
		RequestCount:     requestCount,
		SuccessCount:     atomic.LoadInt64(&pm.successCount),
		ErrorCount:       atomic.LoadInt64(&pm.errorCount),
		AvgLatency:       avgLatency,
		MaxLatency:       time.Duration(atomic.LoadInt64(&pm.maxLatency)),
		MinLatency:       time.Duration(atomic.LoadInt64(&pm.minLatency)),
		P50Latency:       p50,
		P95Latency:       p95,
		P99Latency:       p99,
		FileListCount:    fileListCount,
		FileListAvgTime:  pm.avgTime(pm.fileListTimeSum, fileListCount),
		ThumbnailCount:   atomic.LoadInt64(&pm.thumbnailCount),
		ThumbnailAvgTime: pm.avgTime(pm.thumbnailTimeSum, atomic.LoadInt64(&pm.thumbnailCount)),
		UploadCount:      atomic.LoadInt64(&pm.uploadCount),
		UploadBytes:      atomic.LoadInt64(&pm.uploadBytes),
		DownloadCount:    atomic.LoadInt64(&pm.downloadCount),
		DownloadBytes:    atomic.LoadInt64(&pm.downloadBytes),
		SearchCount:      searchCount,
		SearchAvgTime:    pm.avgTime(pm.searchTimeSum, searchCount),
		IndexCount:       atomic.LoadInt64(&pm.indexCount),
		IndexAvgTime:     pm.avgTime(pm.indexTimeSum, atomic.LoadInt64(&pm.indexCount)),
		CacheHits:        cacheHits,
		CacheMisses:      cacheMisses,
		CacheHitRate:     cacheHitRate,
		DBQueries:        dbQueries,
		DBSlowQueries:    atomic.LoadInt64(&pm.dbSlowQueries),
		DBAvgTime:        pm.avgTime(pm.dbTimeSum, dbQueries),
		GoroutineCount:   int64(runtime.NumGoroutine()),
		MemoryAllocMB:    m.Alloc / 1024 / 1024,
		MemorySysMB:      m.Sys / 1024 / 1024,
		GCPauseMs:        m.PauseTotalNs / 1000000,
	}
}

func (pm *PerformanceMonitor) avgTime(sum, count int64) int64 {
	if count == 0 {
		return 0
	}
	return sum / count / int64(time.Millisecond)
}

// quickSort 快速排序
func (pm *PerformanceMonitor) quickSort(arr []time.Duration) {
	if len(arr) <= 1 {
		return
	}

	pivot := arr[len(arr)/2]
	i, j := 0, len(arr)-1

	for i <= j {
		for arr[i] < pivot {
			i++
		}
		for arr[j] > pivot {
			j--
		}
		if i <= j {
			arr[i], arr[j] = arr[j], arr[i]
			i++
			j--
		}
	}

	if j > 0 {
		pm.quickSort(arr[:j+1])
	}
	if i < len(arr) {
		pm.quickSort(arr[i:])
	}
}

// Reset 重置指标
func (pm *PerformanceMonitor) Reset() {
	atomic.StoreInt64(&pm.requestCount, 0)
	atomic.StoreInt64(&pm.successCount, 0)
	atomic.StoreInt64(&pm.errorCount, 0)

	pm.latencyMutex.Lock()
	pm.latencies = make([]time.Duration, 0, 10000)
	pm.latencySum = 0
	pm.maxLatency = 0
	pm.minLatency = int64(time.Hour)
	pm.latencyMutex.Unlock()

	atomic.StoreInt64(&pm.fileListCount, 0)
	atomic.StoreInt64(&pm.fileListTimeSum, 0)
	atomic.StoreInt64(&pm.thumbnailCount, 0)
	atomic.StoreInt64(&pm.thumbnailTimeSum, 0)
	atomic.StoreInt64(&pm.uploadCount, 0)
	atomic.StoreInt64(&pm.uploadBytes, 0)
	atomic.StoreInt64(&pm.downloadCount, 0)
	atomic.StoreInt64(&pm.downloadBytes, 0)
	atomic.StoreInt64(&pm.searchCount, 0)
	atomic.StoreInt64(&pm.searchTimeSum, 0)
	atomic.StoreInt64(&pm.indexCount, 0)
	atomic.StoreInt64(&pm.indexTimeSum, 0)
	atomic.StoreInt64(&pm.cacheHits, 0)
	atomic.StoreInt64(&pm.cacheMisses, 0)
	atomic.StoreInt64(&pm.dbQueries, 0)
	atomic.StoreInt64(&pm.dbSlowQueries, 0)
	atomic.StoreInt64(&pm.dbTimeSum, 0)
}

// Handlers 性能监控处理器
type Handlers struct {
	monitor *PerformanceMonitor
}

// NewHandlers 创建处理器
func NewHandlers(monitor *PerformanceMonitor) *Handlers {
	return &Handlers{monitor: monitor}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	perf := r.Group("/performance")
	{
		perf.GET("/metrics", h.getMetrics)
		perf.POST("/reset", h.resetMetrics)
		perf.GET("/health", h.healthCheck)
	}
}

// getMetrics 获取性能指标
// @Summary 获取性能指标
// @Description 获取系统性能指标
// @Tags 性能监控
// @Produce json
// @Success 200 {object} Metrics
// @Router /performance/metrics [get]
func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.monitor.GetMetrics()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    metrics,
	})
}

// resetMetrics 重置性能指标
// @Summary 重置性能指标
// @Description 重置所有性能指标
// @Tags 性能监控
// @Success 200 {object} map[string]interface{}
// @Router /performance/reset [post]
func (h *Handlers) resetMetrics(c *gin.Context) {
	h.monitor.Reset()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "指标已重置",
	})
}

// healthCheck 健康检查
// @Summary 健康检查
// @Description 系统健康检查
// @Tags 性能监控
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /performance/health [get]
func (h *Handlers) healthCheck(c *gin.Context) {
	metrics := h.monitor.GetMetrics()

	// 检查系统状态
	status := "healthy"
	issues := []string{}

	// 检查错误率
	errorRate := float64(0)
	if metrics.RequestCount > 0 {
		errorRate = float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
	}
	if errorRate > 10 {
		status = "degraded"
		issues = append(issues, fmt.Sprintf("高错误率: %.2f%%", errorRate))
	}

	// 检查响应时间
	if metrics.P95Latency > 500*time.Millisecond {
		status = "degraded"
		issues = append(issues, fmt.Sprintf("P95响应时间过高: %v", metrics.P95Latency))
	}

	// 检查内存使用
	if metrics.MemoryAllocMB > 500 {
		issues = append(issues, fmt.Sprintf("高内存使用: %d MB", metrics.MemoryAllocMB))
	}

	// 检查 Goroutine 数量
	if metrics.GoroutineCount > 1000 {
		issues = append(issues, fmt.Sprintf("高Goroutine数量: %d", metrics.GoroutineCount))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"status":    status,
			"timestamp": time.Now().Format(time.RFC3339),
			"issues":    issues,
			"metrics": gin.H{
				"requestCount":  metrics.RequestCount,
				"errorRate":     errorRate,
				"avgLatencyMs":  metrics.AvgLatency.Milliseconds(),
				"p95LatencyMs":  metrics.P95Latency.Milliseconds(),
				"memoryMB":      metrics.MemoryAllocMB,
				"goroutines":    metrics.GoroutineCount,
				"cacheHitRate":  metrics.CacheHitRate,
			},
		},
	})
}

// PrometheusExporter Prometheus 指标导出器
type PrometheusExporter struct {
	monitor *PerformanceMonitor
}

// NewPrometheusExporter 创建 Prometheus 导出器
func NewPrometheusExporter(monitor *PerformanceMonitor) *PrometheusExporter {
	return &PrometheusExporter{monitor: monitor}
}

// Handler 返回 Prometheus 格式的指标
func (e *PrometheusExporter) Handler(w http.ResponseWriter, r *http.Request) {
	metrics := e.monitor.GetMetrics()

	// 生成 Prometheus 格式的指标
	output := fmt.Sprintf(`
# HELP nas_requests_total Total number of requests
# TYPE nas_requests_total counter
nas_requests_total %d

# HELP nas_requests_success_total Total number of successful requests
# TYPE nas_requests_success_total counter
nas_requests_success_total %d

# HELP nas_requests_error_total Total number of error requests
# TYPE nas_requests_error_total counter
nas_requests_error_total %d

# HELP nas_request_latency_ms Request latency in milliseconds
# TYPE nas_request_latency_ms gauge
nas_request_latency_ms_avg %d
nas_request_latency_ms_p50 %d
nas_request_latency_ms_p95 %d
nas_request_latency_ms_p99 %d

# HELP nas_file_operations_total Total file operations
# TYPE nas_file_operations_total counter
nas_file_list_total %d
nas_thumbnail_total %d
nas_upload_total %d
nas_download_total %d

# HELP nas_file_bytes_total Total bytes transferred
# TYPE nas_file_bytes_total counter
nas_upload_bytes_total %d
nas_download_bytes_total %d

# HELP nas_search_total Total search operations
# TYPE nas_search_total counter
nas_search_total %d

# HELP nas_cache_hits_total Cache hit count
# TYPE nas_cache_hits_total counter
nas_cache_hits_total %d

# HELP nas_cache_misses_total Cache miss count
# TYPE nas_cache_misses_total counter
nas_cache_misses_total %d

# HELP nas_db_queries_total Database queries total
# TYPE nas_db_queries_total counter
nas_db_queries_total %d
nas_db_slow_queries_total %d

# HELP nas_memory_bytes Memory usage in bytes
# TYPE nas_memory_bytes gauge
nas_memory_alloc_bytes %d
nas_memory_sys_bytes %d

# HELP nas_goroutines Number of goroutines
# TYPE nas_goroutines gauge
nas_goroutines %d
`,
		metrics.RequestCount,
		metrics.SuccessCount,
		metrics.ErrorCount,
		metrics.AvgLatency.Milliseconds(),
		metrics.P50Latency.Milliseconds(),
		metrics.P95Latency.Milliseconds(),
		metrics.P99Latency.Milliseconds(),
		metrics.FileListCount,
		metrics.ThumbnailCount,
		metrics.UploadCount,
		metrics.DownloadCount,
		metrics.UploadBytes,
		metrics.DownloadBytes,
		metrics.SearchCount,
		metrics.CacheHits,
		metrics.CacheMisses,
		metrics.DBQueries,
		metrics.DBSlowQueries,
		metrics.MemoryAllocMB*1024*1024,
		metrics.MemorySysMB*1024*1024,
		metrics.GoroutineCount,
	)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(output))
}

// StartMetricsServer 启动独立的 metrics 服务 (用于 Prometheus 抓取)
func (e *PrometheusExporter) StartMetricsServer(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", e.Handler)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	return server.ListenAndServe()
}

// ToJSON 导出 JSON 格式指标
func (m *Metrics) ToJSON() string {
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}