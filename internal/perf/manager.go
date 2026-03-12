package perf

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Manager 性能监控管理器
type Manager struct {
	mu sync.RWMutex

	// 性能指标
	metrics *MetricsStore

	// 慢请求日志
	slowLogs      []*SlowLogEntry
	slowLogMax    int
	slowLogPath   string
	slowThreshold time.Duration

	// 吞吐量统计
	throughput *ThroughputTracker

	// 停止信号
	stopChan chan struct{}
}

// MetricsStore 性能指标存储
type MetricsStore struct {
	mu sync.RWMutex

	// 端点统计
	Endpoints map[string]*EndpointMetrics

	// 全局统计
	TotalRequests   uint64
	TotalErrors     uint64
	TotalDuration   time.Duration
	AvgResponseTime time.Duration

	// 时间窗口统计 (最近 1 分钟)
	MinuteWindow *TimeWindow

	// 系统性能基线
	Baseline *PerformanceBaseline
}

// EndpointMetrics 端点指标
type EndpointMetrics struct {
	Path           string        `json:"path"`
	Method         string        `json:"method"`
	RequestCount   uint64        `json:"requestCount"`
	ErrorCount     uint64        `json:"errorCount"`
	TotalDuration  time.Duration `json:"totalDuration"`
	AvgDuration    time.Duration `json:"avgDuration"`
	MinDuration    time.Duration `json:"minDuration"`
	MaxDuration    time.Duration `json:"maxDuration"`
	P50Duration    time.Duration `json:"p50Duration"`
	P95Duration    time.Duration `json:"p95Duration"`
	P99Duration    time.Duration `json:"p99Duration"`
	LastAccessTime time.Time     `json:"lastAccessTime"`

	// 响应时间历史 (用于计算百分位)
	durations  []time.Duration
	maxHistory int
}

// TimeWindow 时间窗口统计
type TimeWindow struct {
	mu sync.RWMutex

	// 每秒请求计数
	RequestsPerSecond map[int64]uint64 // timestamp -> count

	// 每秒错误计数
	ErrorsPerSecond map[int64]uint64

	// 窗口大小 (秒)
	WindowSize int64
}

// SlowLogEntry 慢请求日志条目
type SlowLogEntry struct {
	Timestamp   time.Time     `json:"timestamp"`
	RequestID   string        `json:"requestId"`
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	Query       string        `json:"query"`
	ClientIP    string        `json:"clientIP"`
	Duration    time.Duration `json:"duration"`
	StatusCode  int           `json:"statusCode"`
	UserAgent   string        `json:"userAgent"`
	RequestSize int64         `json:"requestSize"`
}

// ThroughputTracker 吞吐量追踪器
type ThroughputTracker struct {
	mu sync.RWMutex

	// 每分钟统计
	MinuteStats map[int64]*MinuteStat

	// 每小时统计
	HourlyStats map[int64]*HourlyStat

	// 每日统计
	DailyStats map[int64]*DailyStat
}

// MinuteStat 分钟统计
type MinuteStat struct {
	Timestamp    int64   `json:"timestamp"`
	RequestCount uint64  `json:"requestCount"`
	ErrorCount   uint64  `json:"errorCount"`
	TotalBytes   uint64  `json:"totalBytes"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	PeakRPS      float64 `json:"peakRPS"`
}

// HourlyStat 小时统计
type HourlyStat struct {
	Timestamp    int64   `json:"timestamp"`
	RequestCount uint64  `json:"requestCount"`
	ErrorCount   uint64  `json:"errorCount"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	PeakRPS      float64 `json:"peakRPS"`
}

// DailyStat 日统计
type DailyStat struct {
	Timestamp    int64   `json:"timestamp"`
	RequestCount uint64  `json:"requestCount"`
	ErrorCount   uint64  `json:"errorCount"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	PeakRPS      float64 `json:"peakRPS"`
}

// PerformanceBaseline 性能基线
type PerformanceBaseline struct {
	mu sync.RWMutex

	// 响应时间基线 (毫秒)
	AvgResponseTime float64 `json:"avgResponseTime"`
	P95ResponseTime float64 `json:"p95ResponseTime"`
	P99ResponseTime float64 `json:"p99ResponseTime"`

	// 吞吐量基线
	AvgRPS  float64 `json:"avgRPS"`
	PeakRPS float64 `json:"peakRPS"`

	// 错误率基线
	AvgErrorRate float64 `json:"avgErrorRate"`

	// 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// Config 性能监控配置
type Config struct {
	SlowThreshold    time.Duration // 慢请求阈值
	SlowLogMax       int           // 最大慢日志条数
	SlowLogPath      string        // 慢日志文件路径
	EnableBaseline   bool          // 是否启用基线计算
	BaselineInterval time.Duration // 基线更新间隔
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		SlowThreshold:    500 * time.Millisecond,
		SlowLogMax:       1000,
		SlowLogPath:      "/var/log/nas-os/slow.log",
		EnableBaseline:   true,
		BaselineInterval: 5 * time.Minute,
	}
}

// NewManager 创建性能监控管理器
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 确保日志目录存在
	logDir := filepath.Dir(cfg.SlowLogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[WARN] 无法创建慢日志目录: %v", err)
	}

	m := &Manager{
		metrics: &MetricsStore{
			Endpoints: make(map[string]*EndpointMetrics),
			MinuteWindow: &TimeWindow{
				RequestsPerSecond: make(map[int64]uint64),
				ErrorsPerSecond:   make(map[int64]uint64),
				WindowSize:        60,
			},
			Baseline: &PerformanceBaseline{},
		},
		slowLogs:      make([]*SlowLogEntry, 0),
		slowLogMax:    cfg.SlowLogMax,
		slowLogPath:   cfg.SlowLogPath,
		slowThreshold: cfg.SlowThreshold,
		throughput: &ThroughputTracker{
			MinuteStats: make(map[int64]*MinuteStat),
			HourlyStats: make(map[int64]*HourlyStat),
			DailyStats:  make(map[int64]*DailyStat),
		},
		stopChan: make(chan struct{}),
	}

	// 启动后台任务
	go m.cleanupLoop()
	if cfg.EnableBaseline {
		go m.baselineLoop(cfg.BaselineInterval)
	}

	return m, nil
}

// Middleware 性能监控中间件
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID, _ := c.Get("requestID")
		if requestID == nil {
			requestID = "unknown"
		}

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start)
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method
		statusCode := c.Writer.Status()

		// 更新指标
		m.recordMetrics(path, method, duration, statusCode)

		// 记录吞吐量
		m.recordThroughput(duration, statusCode, c.Writer.Size())

		// 检查慢请求
		if duration > m.slowThreshold {
			m.recordSlowLog(&SlowLogEntry{
				Timestamp:   time.Now(),
				RequestID:   requestID.(string),
				Method:      method,
				Path:        path,
				Query:       c.Request.URL.RawQuery,
				ClientIP:    c.ClientIP(),
				Duration:    duration,
				StatusCode:  statusCode,
				UserAgent:   c.Request.UserAgent(),
				RequestSize: c.Request.ContentLength,
			})
		}
	}
}

// recordMetrics 记录性能指标
func (m *Manager) recordMetrics(path, method string, duration time.Duration, statusCode int) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	key := method + ":" + path
	em, exists := m.metrics.Endpoints[key]
	if !exists {
		em = &EndpointMetrics{
			Path:        path,
			Method:      method,
			MinDuration: duration,
			maxHistory:  1000,
			durations:   make([]time.Duration, 0, 1000),
		}
		m.metrics.Endpoints[key] = em
	}

	// 更新统计
	em.RequestCount++
	em.TotalDuration += duration
	em.LastAccessTime = time.Now()

	if duration < em.MinDuration || em.MinDuration == 0 {
		em.MinDuration = duration
	}
	if duration > em.MaxDuration {
		em.MaxDuration = duration
	}

	// 记录响应时间历史
	em.durations = append(em.durations, duration)
	if len(em.durations) > em.maxHistory {
		em.durations = em.durations[1:]
	}

	// 计算百分位
	em.calculatePercentiles()

	// 更新全局统计
	m.metrics.TotalRequests++
	m.metrics.TotalDuration += duration
	if statusCode >= 400 {
		m.metrics.TotalErrors++
		em.ErrorCount++
	}

	// 更新平均响应时间
	if m.metrics.TotalRequests > 0 {
		m.metrics.AvgResponseTime = time.Duration(
			float64(m.metrics.TotalDuration) / float64(m.metrics.TotalRequests),
		)
	}

	// 更新时间窗口
	m.updateTimeWindow(duration, statusCode)
}

// calculatePercentiles 计算百分位响应时间
func (em *EndpointMetrics) calculatePercentiles() {
	if len(em.durations) == 0 {
		return
	}

	// 复制并排序
	sorted := make([]time.Duration, len(em.durations))
	copy(sorted, em.durations)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := len(sorted)
	em.P50Duration = sorted[n*50/100]
	em.P95Duration = sorted[n*95/100]
	em.P99Duration = sorted[n*99/100]
	em.AvgDuration = time.Duration(float64(em.TotalDuration) / float64(em.RequestCount))
}

// updateTimeWindow 更新时间窗口统计
func (m *Manager) updateTimeWindow(duration time.Duration, statusCode int) {
	window := m.metrics.MinuteWindow
	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now().Unix()

	// 更新请求计数
	window.RequestsPerSecond[now]++

	// 更新错误计数
	if statusCode >= 400 {
		window.ErrorsPerSecond[now]++
	}

	// 清理过期数据
	cutoff := now - window.WindowSize
	for ts := range window.RequestsPerSecond {
		if ts < cutoff {
			delete(window.RequestsPerSecond, ts)
		}
	}
	for ts := range window.ErrorsPerSecond {
		if ts < cutoff {
			delete(window.ErrorsPerSecond, ts)
		}
	}
}

// recordThroughput 记录吞吐量
func (m *Manager) recordThroughput(duration time.Duration, statusCode int, responseSize int) {
	m.throughput.mu.Lock()
	defer m.throughput.mu.Unlock()

	now := time.Now()
	minuteKey := now.Unix() / 60
	hourKey := now.Unix() / 3600
	dayKey := now.Unix() / 86400

	// 更新分钟统计
	minStat, exists := m.throughput.MinuteStats[minuteKey]
	if !exists {
		minStat = &MinuteStat{Timestamp: minuteKey * 60}
		m.throughput.MinuteStats[minuteKey] = minStat
	}
	minStat.RequestCount++
	minStat.TotalBytes += uint64(responseSize)
	if statusCode >= 400 {
		minStat.ErrorCount++
	}
	// 更新平均延迟
	latencyMs := float64(duration.Milliseconds())
	minStat.AvgLatencyMs = (minStat.AvgLatencyMs*float64(minStat.RequestCount-1) + latencyMs) / float64(minStat.RequestCount)

	// 更新小时统计
	hourStat, exists := m.throughput.HourlyStats[hourKey]
	if !exists {
		hourStat = &HourlyStat{Timestamp: hourKey * 3600}
		m.throughput.HourlyStats[hourKey] = hourStat
	}
	hourStat.RequestCount++
	if statusCode >= 400 {
		hourStat.ErrorCount++
	}

	// 更新日统计
	dayStat, exists := m.throughput.DailyStats[dayKey]
	if !exists {
		dayStat = &DailyStat{Timestamp: dayKey * 86400}
		m.throughput.DailyStats[dayKey] = dayStat
	}
	dayStat.RequestCount++
	if statusCode >= 400 {
		dayStat.ErrorCount++
	}

	// 清理旧数据 (保留最近 1 小时的分钟数据, 7 天的小时数据, 30 天的日数据)
	m.cleanupThroughputData(now)
}

// recordSlowLog 记录慢请求
func (m *Manager) recordSlowLog(entry *SlowLogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 添加到内存
	m.slowLogs = append(m.slowLogs, entry)
	if len(m.slowLogs) > m.slowLogMax {
		m.slowLogs = m.slowLogs[len(m.slowLogs)-m.slowLogMax:]
	}

	// 写入文件
	go m.writeSlowLogToFile(entry)
}

// writeSlowLogToFile 写入慢日志文件
func (m *Manager) writeSlowLogToFile(entry *SlowLogEntry) {
	f, err := os.OpenFile(m.slowLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[ERROR] 无法打开慢日志文件: %v", err)
		return
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[ERROR] 序列化慢日志失败: %v", err)
		return
	}

	_, _ = f.Write(data)
	_, _ = f.WriteString("\n")
}

// cleanupLoop 清理过期数据
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupThroughputData(time.Now())
		case <-m.stopChan:
			return
		}
	}
}

// cleanupThroughputData 清理吞吐量数据
func (m *Manager) cleanupThroughputData(now time.Time) {
	m.throughput.mu.Lock()
	defer m.throughput.mu.Unlock()

	// 清理分钟数据 (保留 1 小时)
	minuteCutoff := now.Unix()/60 - 60
	for k := range m.throughput.MinuteStats {
		if k < minuteCutoff {
			delete(m.throughput.MinuteStats, k)
		}
	}

	// 清理小时数据 (保留 7 天)
	hourCutoff := now.Unix()/3600 - 24*7
	for k := range m.throughput.HourlyStats {
		if k < hourCutoff {
			delete(m.throughput.HourlyStats, k)
		}
	}

	// 清理日数据 (保留 30 天)
	dayCutoff := now.Unix()/86400 - 30
	for k := range m.throughput.DailyStats {
		if k < dayCutoff {
			delete(m.throughput.DailyStats, k)
		}
	}
}

// baselineLoop 更新性能基线
func (m *Manager) baselineLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateBaseline()
		case <-m.stopChan:
			return
		}
	}
}

// updateBaseline 更新性能基线
func (m *Manager) updateBaseline() {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	baseline := m.metrics.Baseline
	baseline.mu.Lock()
	defer baseline.mu.Unlock()

	// 计算平均响应时间
	if m.metrics.TotalRequests > 0 {
		baseline.AvgResponseTime = float64(m.metrics.TotalDuration.Milliseconds()) / float64(m.metrics.TotalRequests)
	}

	// 计算平均吞吐量
	window := m.metrics.MinuteWindow
	window.mu.RLock()
	totalRequests := uint64(0)
	for _, count := range window.RequestsPerSecond {
		totalRequests += count
	}
	if window.WindowSize > 0 {
		baseline.AvgRPS = float64(totalRequests) / float64(window.WindowSize)
	}
	window.mu.RUnlock()

	// 计算错误率
	if m.metrics.TotalRequests > 0 {
		baseline.AvgErrorRate = float64(m.metrics.TotalErrors) / float64(m.metrics.TotalRequests) * 100
	}

	// 计算 P95/P99
	var allDurations []time.Duration
	for _, em := range m.metrics.Endpoints {
		allDurations = append(allDurations, em.durations...)
	}
	if len(allDurations) > 0 {
		// 排序
		for i := 0; i < len(allDurations)-1; i++ {
			for j := i + 1; j < len(allDurations); j++ {
				if allDurations[i] > allDurations[j] {
					allDurations[i], allDurations[j] = allDurations[j], allDurations[i]
				}
			}
		}
		n := len(allDurations)
		baseline.P95ResponseTime = float64(allDurations[n*95/100].Milliseconds())
		baseline.P99ResponseTime = float64(allDurations[n*99/100].Milliseconds())
	}

	baseline.LastUpdated = time.Now()
}

// Stop 停止性能监控
func (m *Manager) Stop() {
	close(m.stopChan)
}

// ========== API 处理器 ==========

// GetMetrics 获取性能指标
func (m *Manager) GetMetrics() *MetricsStore {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	// 返回副本
	result := &MetricsStore{
		TotalRequests:   m.metrics.TotalRequests,
		TotalErrors:     m.metrics.TotalErrors,
		TotalDuration:   m.metrics.TotalDuration,
		AvgResponseTime: m.metrics.AvgResponseTime,
		Endpoints:       make(map[string]*EndpointMetrics),
	}

	for k, v := range m.metrics.Endpoints {
		result.Endpoints[k] = v
	}

	return result
}

// GetEndpointMetrics 获取特定端点的指标
func (m *Manager) GetEndpointMetrics(path, method string) *EndpointMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	key := method + ":" + path
	return m.metrics.Endpoints[key]
}

// GetSlowLogs 获取慢请求日志
func (m *Manager) GetSlowLogs(limit int) []*SlowLogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.slowLogs) {
		limit = len(m.slowLogs)
	}

	// 返回最新的 N 条
	start := len(m.slowLogs) - limit
	if start < 0 {
		start = 0
	}

	return m.slowLogs[start:]
}

// GetThroughputStats 获取吞吐量统计
func (m *Manager) GetThroughputStats() map[string]interface{} {
	m.throughput.mu.RLock()
	defer m.throughput.mu.RUnlock()

	// 计算当前 RPS
	now := time.Now().Unix()
	minuteKey := now / 60
	currentMin := m.throughput.MinuteStats[minuteKey]

	var currentRPS float64
	if currentMin != nil {
		currentRPS = float64(currentMin.RequestCount) / 60.0
	}

	// 计算峰值 RPS
	var peakRPS float64
	for _, stat := range m.throughput.MinuteStats {
		rps := float64(stat.RequestCount) / 60.0
		if rps > peakRPS {
			peakRPS = rps
		}
	}

	// 获取小时统计
	hourStats := make([]*HourlyStat, 0)
	hourKey := now / 3600
	for k, v := range m.throughput.HourlyStats {
		if k >= hourKey-24 { // 最近 24 小时
			hourStats = append(hourStats, v)
		}
	}

	// 获取日统计
	dayStats := make([]*DailyStat, 0)
	dayKey := now / 86400
	for k, v := range m.throughput.DailyStats {
		if k >= dayKey-7 { // 最近 7 天
			dayStats = append(dayStats, v)
		}
	}

	return map[string]interface{}{
		"currentRPS": currentRPS,
		"peakRPS":    peakRPS,
		"hourly":     hourStats,
		"daily":      dayStats,
	}
}

// GetBaseline 获取性能基线
func (m *Manager) GetBaseline() *PerformanceBaseline {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	return m.metrics.Baseline
}

// GetTimeWindowStats 获取时间窗口统计
func (m *Manager) GetTimeWindowStats() map[string]interface{} {
	window := m.metrics.MinuteWindow
	window.mu.RLock()
	defer window.mu.RUnlock()

	now := time.Now().Unix()
	cutoff := now - window.WindowSize

	var totalRequests, totalErrors uint64
	for ts, count := range window.RequestsPerSecond {
		if ts >= cutoff {
			totalRequests += count
		}
	}
	for ts, count := range window.ErrorsPerSecond {
		if ts >= cutoff {
			totalErrors += count
		}
	}

	avgRPS := float64(totalRequests) / float64(window.WindowSize)
	errorRate := float64(0)
	if totalRequests > 0 {
		errorRate = float64(totalErrors) / float64(totalRequests) * 100
	}

	return map[string]interface{}{
		"windowSize":    window.WindowSize,
		"totalRequests": totalRequests,
		"totalErrors":   totalErrors,
		"avgRPS":        avgRPS,
		"errorRate":     errorRate,
	}
}
