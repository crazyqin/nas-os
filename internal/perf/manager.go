package perf

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// ==================== v2.4.0 性能监控增强 ====================

// AlertRule 性能告警规则
type AlertRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`

	// 触发条件
	Metric    string        `json:"metric"`    // 指标名: response_time, error_rate, rps, cpu, memory
	Operator  string        `json:"operator"`  // 操作符: >, <, >=, <=, ==
	Threshold float64       `json:"threshold"` // 阈值
	Duration  time.Duration `json:"duration"`  // 持续时间

	// 严重程度
	Severity string `json:"severity"` // info, warning, critical

	// 通知配置
	NotifyEmail   []string `json:"notifyEmail,omitempty"`
	NotifyWebhook string   `json:"notifyWebhook,omitempty"`

	// 触发动作
	Action string `json:"action,omitempty"` // 自定义动作

	// 元数据
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	LastTriggered time.Time `json:"lastTriggered,omitempty"`
	TriggerCount  int       `json:"triggerCount"`
}

// AlertInstance 告警实例
type AlertInstance struct {
	RuleID      string    `json:"ruleId"`
	RuleName    string    `json:"ruleName"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	TriggeredAt time.Time `json:"triggeredAt"`
	ResolvedAt  time.Time `json:"resolvedAt,omitempty"`
	Resolved    bool      `json:"resolved"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportFormatJSON     ExportFormat = "json"
	ExportFormatCSV      ExportFormat = "csv"
	ExportFormatHTML     ExportFormat = "html"
	ExportFormatMarkdown ExportFormat = "markdown"
)

// PerformanceReport 性能报告
type PerformanceReport struct {
	GeneratedAt     time.Time         `json:"generatedAt"`
	TimeRange       string            `json:"timeRange"`
	Summary         *ReportSummary    `json:"summary"`
	Endpoints       []*EndpointReport `json:"endpoints"`
	SlowRequests    []*SlowLogEntry   `json:"slowRequests,omitempty"`
	Throughput      *ThroughputReport `json:"throughput"`
	Alerts          []*AlertInstance  `json:"alerts,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalRequests   uint64  `json:"totalRequests"`
	TotalErrors     uint64  `json:"totalErrors"`
	ErrorRate       float64 `json:"errorRate"`
	AvgResponseTime float64 `json:"avgResponseTime"`
	P50ResponseTime float64 `json:"p50ResponseTime"`
	P95ResponseTime float64 `json:"p95ResponseTime"`
	P99ResponseTime float64 `json:"p99ResponseTime"`
	PeakRPS         float64 `json:"peakRPS"`
	AvgRPS          float64 `json:"avgRPS"`
}

// EndpointReport 端点报告
type EndpointReport struct {
	Path         string  `json:"path"`
	Method       string  `json:"method"`
	RequestCount uint64  `json:"requestCount"`
	ErrorCount   uint64  `json:"errorCount"`
	ErrorRate    float64 `json:"errorRate"`
	AvgDuration  float64 `json:"avgDuration"`
	P50Duration  float64 `json:"p50Duration"`
	P95Duration  float64 `json:"p95Duration"`
	P99Duration  float64 `json:"p99Duration"`
}

// ThroughputReport 吞吐量报告
type ThroughputReport struct {
	CurrentRPS float64       `json:"currentRPS"`
	PeakRPS    float64       `json:"peakRPS"`
	Hourly     []*HourlyStat `json:"hourly"`
	Daily      []*DailyStat  `json:"daily"`
}

// alertManager 告警管理器（内部使用）
type alertManager struct {
	mu      sync.RWMutex
	rules   map[string]*AlertRule
	alerts  []*AlertInstance
	enabled bool
}

// 告警管理器实例
var globalAlertManager = &alertManager{
	rules:   make(map[string]*AlertRule),
	alerts:  make([]*AlertInstance, 0),
	enabled: true,
}

// SetAlertRule 设置性能告警规则
func (m *Manager) SetAlertRule(rule AlertRule) (*AlertRule, error) {
	if rule.Name == "" {
		return nil, fmt.Errorf("告警规则名称不能为空")
	}

	// 验证指标名
	validMetrics := map[string]bool{
		"response_time": true,
		"error_rate":    true,
		"rps":           true,
		"cpu":           true,
		"memory":        true,
	}
	if !validMetrics[rule.Metric] {
		return nil, fmt.Errorf("无效的指标名: %s", rule.Metric)
	}

	// 验证操作符
	validOperators := map[string]bool{
		">":  true,
		"<":  true,
		">=": true,
		"<=": true,
		"==": true,
	}
	if !validOperators[rule.Operator] {
		return nil, fmt.Errorf("无效的操作符: %s", rule.Operator)
	}

	// 验证严重程度
	validSeverities := map[string]bool{
		"info":     true,
		"warning":  true,
		"critical": true,
	}
	if !validSeverities[rule.Severity] {
		rule.Severity = "warning" // 默认
	}

	// 设置ID和时间
	if rule.ID == "" {
		rule.ID = "alert_" + time.Now().Format("20060102150405")
	}
	rule.UpdatedAt = time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}

	// 保存规则
	globalAlertManager.mu.Lock()
	globalAlertManager.rules[rule.ID] = &rule
	globalAlertManager.mu.Unlock()

	// 启动告警检查（如果启用）
	if rule.Enabled {
		go m.startAlertCheck(&rule)
	}

	return &rule, nil
}

// GetAlertRules 获取所有告警规则
func (m *Manager) GetAlertRules() []*AlertRule {
	globalAlertManager.mu.RLock()
	defer globalAlertManager.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(globalAlertManager.rules))
	for _, rule := range globalAlertManager.rules {
		rules = append(rules, rule)
	}
	return rules
}

// DeleteAlertRule 删除告警规则
func (m *Manager) DeleteAlertRule(id string) error {
	globalAlertManager.mu.Lock()
	defer globalAlertManager.mu.Unlock()

	if _, ok := globalAlertManager.rules[id]; !ok {
		return fmt.Errorf("告警规则不存在: %s", id)
	}

	delete(globalAlertManager.rules, id)
	return nil
}

// GetActiveAlerts 获取活跃告警
func (m *Manager) GetActiveAlerts() []*AlertInstance {
	globalAlertManager.mu.RLock()
	defer globalAlertManager.mu.RUnlock()

	alerts := make([]*AlertInstance, 0)
	for _, alert := range globalAlertManager.alerts {
		if !alert.Resolved {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// startAlertCheck 启动告警检查
func (m *Manager) startAlertCheck(rule *AlertRule) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	checkDuration := rule.Duration
	if checkDuration == 0 {
		checkDuration = time.Minute
	}

	var triggerStartTime time.Time
	var triggered bool

	for {
		select {
		case <-ticker.C:
			if !rule.Enabled {
				return
			}

			// 获取当前指标值
			value := m.getMetricValue(rule.Metric)

			// 检查条件
			conditionMet := m.checkCondition(value, rule.Operator, rule.Threshold)

			if conditionMet {
				if !triggered {
					triggerStartTime = time.Now()
					triggered = true
				}

				// 检查是否持续足够时间
				if time.Since(triggerStartTime) >= checkDuration {
					m.triggerAlert(rule, value)
					rule.LastTriggered = time.Now()
					rule.TriggerCount++
				}
			} else {
				triggered = false
				triggerStartTime = time.Time{}
			}
		case <-m.stopChan:
			return
		}
	}
}

// getMetricValue 获取指标值
func (m *Manager) getMetricValue(metric string) float64 {
	switch metric {
	case "response_time":
		return float64(m.metrics.AvgResponseTime.Milliseconds())
	case "error_rate":
		if m.metrics.TotalRequests > 0 {
			return float64(m.metrics.TotalErrors) / float64(m.metrics.TotalRequests) * 100
		}
		return 0
	case "rps":
		stats := m.GetTimeWindowStats()
		if rps, ok := stats["avgRPS"].(float64); ok {
			return rps
		}
		return 0
	default:
		return 0
	}
}

// checkCondition 检查条件
func (m *Manager) checkCondition(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		return false
	}
}

// triggerAlert 触发告警
func (m *Manager) triggerAlert(rule *AlertRule, value float64) {
	alert := &AlertInstance{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Severity:    rule.Severity,
		Message:     fmt.Sprintf("%s: %.2f %s %.2f", rule.Name, value, rule.Operator, rule.Threshold),
		Value:       value,
		Threshold:   rule.Threshold,
		TriggeredAt: time.Now(),
	}

	globalAlertManager.mu.Lock()
	globalAlertManager.alerts = append(globalAlertManager.alerts, alert)
	// 保留最近100条告警
	if len(globalAlertManager.alerts) > 100 {
		globalAlertManager.alerts = globalAlertManager.alerts[len(globalAlertManager.alerts)-100:]
	}
	globalAlertManager.mu.Unlock()

	// 记录日志
	log.Printf("[ALERT] %s: %s", rule.Severity, alert.Message)

	// 发送通知
	go sendAlertNotification(alert)
}

// sendAlertNotification 发送告警通知
func sendAlertNotification(alert *AlertInstance) {
	// 检查通知配置
	if notificationConfig == nil {
		return
	}

	// 发送邮件通知
	if notificationConfig.Email != nil && notificationConfig.Email.Enabled {
		if err := sendEmailNotification(alert); err != nil {
			log.Printf("发送邮件通知失败：%v", err)
		}
	}

	// 发送 Webhook 通知
	if notificationConfig.Webhook != nil && notificationConfig.Webhook.Enabled {
		if err := sendWebhookNotification(alert); err != nil {
			log.Printf("发送 Webhook 通知失败：%v", err)
		}
	}
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Email   *EmailNotificationConfig   `json:"email,omitempty"`
	Webhook *WebhookNotificationConfig `json:"webhook,omitempty"`
}

// EmailNotificationConfig 邮件通知配置
type EmailNotificationConfig struct {
	Enabled  bool     `json:"enabled"`
	SMTPHost string   `json:"smtpHost"`
	SMTPPort int      `json:"smtpPort"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	UseTLS   bool     `json:"useTLS"`
}

// WebhookNotificationConfig Webhook 通知配置
type WebhookNotificationConfig struct {
	Enabled bool              `json:"enabled"`
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
}

// notificationConfig 通知配置（全局）
var notificationConfig *NotificationConfig

// SetNotificationConfig 设置通知配置
func SetNotificationConfig(config *NotificationConfig) {
	notificationConfig = config
}

// sendEmailNotification 发送邮件通知
func sendEmailNotification(alert *AlertInstance) error {
	if notificationConfig == nil || notificationConfig.Email == nil {
		return fmt.Errorf("邮件通知未配置")
	}
	config := notificationConfig.Email

	// 构建邮件内容
	subject := fmt.Sprintf("[NAS-OS Alert] %s - %s", alert.Severity, alert.RuleName)
	body := fmt.Sprintf(`
告警类型：%s
告警级别：%s
告警消息：%s
触发时间：%s
当前值：%.2f
阈值：%.2f

请及时处理。
`, alert.RuleName, alert.Severity, alert.Message, alert.TriggeredAt.Format("2006-01-02 15:04:05"), alert.Value, alert.Threshold)

	// 这里简化实现，实际应使用 smtp.SendMail
	log.Printf("[EMAIL] 发送告警邮件：%s -> %v", subject, config.To)
	_ = body // 避免未使用警告
	return nil
}

// sendWebhookNotification 发送 Webhook 通知
func sendWebhookNotification(alert *AlertInstance) error {
	if notificationConfig == nil || notificationConfig.Webhook == nil {
		return fmt.Errorf("Webhook 通知未配置")
	}
	config := notificationConfig.Webhook

	// 构建 payload
	payload := map[string]interface{}{
		"alert":         alert,
		"timestamp":     time.Now().Unix(),
		"severity":      alert.Severity,
		"message":       alert.Message,
		"rule_name":     alert.RuleName,
		"current_value": alert.Value,
		"threshold":     alert.Threshold,
	}

	// 这里简化实现，实际应使用 http.Client 发送请求
	log.Printf("[WEBHOOK] 发送告警到：%s, payload: %+v", config.URL, payload)
	return nil
}

// ExportReport 导出性能报告
func (m *Manager) ExportReport(format string) ([]byte, error) {
	exportFormat := ExportFormat(format)
	if exportFormat == "" {
		exportFormat = ExportFormatJSON
	}

	// 生成报告数据
	report := m.generateReport()

	switch exportFormat {
	case ExportFormatJSON:
		return m.exportJSON(report)
	case ExportFormatCSV:
		return m.exportCSV(report)
	case ExportFormatHTML:
		return m.exportHTML(report)
	case ExportFormatMarkdown:
		return m.exportMarkdown(report)
	default:
		return m.exportJSON(report)
	}
}

// generateReport 生成报告数据
func (m *Manager) generateReport() *PerformanceReport {
	report := &PerformanceReport{
		GeneratedAt:     time.Now(),
		TimeRange:       "最近24小时",
		Summary:         &ReportSummary{},
		Endpoints:       make([]*EndpointReport, 0),
		Throughput:      &ThroughputReport{},
		Alerts:          m.GetActiveAlerts(),
		Recommendations: make([]string, 0),
	}

	// 填充摘要
	m.metrics.mu.RLock()
	report.Summary.TotalRequests = m.metrics.TotalRequests
	report.Summary.TotalErrors = m.metrics.TotalErrors
	if m.metrics.TotalRequests > 0 {
		report.Summary.ErrorRate = float64(m.metrics.TotalErrors) / float64(m.metrics.TotalRequests) * 100
	}
	report.Summary.AvgResponseTime = float64(m.metrics.AvgResponseTime.Milliseconds())
	report.Summary.P95ResponseTime = m.metrics.Baseline.P95ResponseTime
	report.Summary.P99ResponseTime = m.metrics.Baseline.P99ResponseTime
	report.Summary.PeakRPS = m.metrics.Baseline.PeakRPS
	report.Summary.AvgRPS = m.metrics.Baseline.AvgRPS

	// 填充端点数据
	for key, em := range m.metrics.Endpoints {
		parts := strings.SplitN(key, ":", 2)
		method := ""
		path := key
		if len(parts) == 2 {
			method = parts[0]
			path = parts[1]
		}

		epReport := &EndpointReport{
			Path:         path,
			Method:       method,
			RequestCount: em.RequestCount,
			ErrorCount:   em.ErrorCount,
			AvgDuration:  float64(em.AvgDuration.Milliseconds()),
			P50Duration:  float64(em.P50Duration.Milliseconds()),
			P95Duration:  float64(em.P95Duration.Milliseconds()),
			P99Duration:  float64(em.P99Duration.Milliseconds()),
		}
		if em.RequestCount > 0 {
			epReport.ErrorRate = float64(em.ErrorCount) / float64(em.RequestCount) * 100
		}
		report.Endpoints = append(report.Endpoints, epReport)
	}
	m.metrics.mu.RUnlock()

	// 填充吞吐量
	throughputStats := m.GetThroughputStats()
	if currentRPS, ok := throughputStats["currentRPS"].(float64); ok {
		report.Throughput.CurrentRPS = currentRPS
	}
	if peakRPS, ok := throughputStats["peakRPS"].(float64); ok {
		report.Throughput.PeakRPS = peakRPS
	}
	if hourly, ok := throughputStats["hourly"].([]*HourlyStat); ok {
		report.Throughput.Hourly = hourly
	}
	if daily, ok := throughputStats["daily"].([]*DailyStat); ok {
		report.Throughput.Daily = daily
	}

	// 添加慢请求
	report.SlowRequests = m.GetSlowLogs(50)

	// 生成建议
	report.Recommendations = m.generateRecommendations(report)

	return report
}

// generateRecommendations 生成性能建议
func (m *Manager) generateRecommendations(report *PerformanceReport) []string {
	var recommendations []string

	// 高错误率
	if report.Summary.ErrorRate > 5 {
		recommendations = append(recommendations,
			fmt.Sprintf("错误率较高 (%.2f%%)，建议检查日志排查问题", report.Summary.ErrorRate))
	}

	// 响应时间
	if report.Summary.P95ResponseTime > 500 {
		recommendations = append(recommendations,
			fmt.Sprintf("P95响应时间较高 (%.0fms)，建议优化慢接口", report.Summary.P95ResponseTime))
	}

	// 慢请求
	if len(report.SlowRequests) > 10 {
		recommendations = append(recommendations,
			fmt.Sprintf("存在%d个慢请求，建议进行性能优化", len(report.SlowRequests)))
	}

	// 活跃告警
	if len(report.Alerts) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("有%d个活跃告警需要处理", len(report.Alerts)))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统性能良好，继续保持")
	}

	return recommendations
}

// exportJSON 导出JSON格式
func (m *Manager) exportJSON(report *PerformanceReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// exportCSV 导出CSV格式
func (m *Manager) exportCSV(report *PerformanceReport) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入摘要
	_ = writer.Write([]string{"指标", "值"})
	_ = writer.Write([]string{"总请求数", fmt.Sprintf("%d", report.Summary.TotalRequests)})
	_ = writer.Write([]string{"总错误数", fmt.Sprintf("%d", report.Summary.TotalErrors)})
	_ = writer.Write([]string{"错误率", fmt.Sprintf("%.2f%%", report.Summary.ErrorRate)})
	_ = writer.Write([]string{"平均响应时间", fmt.Sprintf("%.0fms", report.Summary.AvgResponseTime)})
	_ = writer.Write([]string{"P95响应时间", fmt.Sprintf("%.0fms", report.Summary.P95ResponseTime)})
	_ = writer.Write([]string{"峰值RPS", fmt.Sprintf("%.2f", report.Summary.PeakRPS)})
	_ = writer.Write([]string{""}) // 空行

	// 写入端点数据
	_ = writer.Write([]string{"端点", "方法", "请求数", "错误数", "错误率", "平均响应时间", "P95响应时间"})
	for _, ep := range report.Endpoints {
		_ = writer.Write([]string{
			ep.Path,
			ep.Method,
			fmt.Sprintf("%d", ep.RequestCount),
			fmt.Sprintf("%d", ep.ErrorCount),
			fmt.Sprintf("%.2f%%", ep.ErrorRate),
			fmt.Sprintf("%.0fms", ep.AvgDuration),
			fmt.Sprintf("%.0fms", ep.P95Duration),
		})
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// exportHTML 导出HTML格式
func (m *Manager) exportHTML(report *PerformanceReport) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>NAS-OS 性能报告</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        h2 { color: #666; border-bottom: 1px solid #ccc; padding-bottom: 5px; }
        table { border-collapse: collapse; width: 100%; margin: 10px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
        .summary { background-color: #e7f3fe; padding: 15px; border-radius: 5px; }
        .alert { background-color: #fff3cd; padding: 10px; margin: 5px 0; border-left: 4px solid #ffc107; }
        .recommendation { background-color: #d4edda; padding: 10px; margin: 5px 0; border-left: 4px solid #28a745; }
    </style>
</head>
<body>
    <h1>NAS-OS 性能报告</h1>
    <p>生成时间: ` + report.GeneratedAt.Format("2006-01-02 15:04:05") + `</p>
`)

	// 摘要
	buf.WriteString(`<div class="summary">
    <h2>性能摘要</h2>
    <table>
        <tr><th>指标</th><th>值</th></tr>
        <tr><td>总请求数</td><td>` + fmt.Sprintf("%d", report.Summary.TotalRequests) + `</td></tr>
        <tr><td>总错误数</td><td>` + fmt.Sprintf("%d", report.Summary.TotalErrors) + `</td></tr>
        <tr><td>错误率</td><td>` + fmt.Sprintf("%.2f%%", report.Summary.ErrorRate) + `</td></tr>
        <tr><td>平均响应时间</td><td>` + fmt.Sprintf("%.0fms", report.Summary.AvgResponseTime) + `</td></tr>
        <tr><td>P95响应时间</td><td>` + fmt.Sprintf("%.0fms", report.Summary.P95ResponseTime) + `</td></tr>
        <tr><td>峰值RPS</td><td>` + fmt.Sprintf("%.2f", report.Summary.PeakRPS) + `</td></tr>
    </table>
</div>
`)

	// 端点
	buf.WriteString(`<h2>端点性能</h2>
<table>
    <tr><th>路径</th><th>方法</th><th>请求数</th><th>错误率</th><th>平均响应时间</th><th>P95响应时间</th></tr>
`)
	for _, ep := range report.Endpoints {
		fmt.Fprintf(&buf, `<tr>
        <td>%s</td><td>%s</td><td>%d</td><td>%.2f%%</td><td>%.0fms</td><td>%.0fms</td>
    </tr>
`, ep.Path, ep.Method, ep.RequestCount, ep.ErrorRate, ep.AvgDuration, ep.P95Duration)
	}
	buf.WriteString(`</table>
`)

	// 告警
	if len(report.Alerts) > 0 {
		buf.WriteString(`<h2>活跃告警</h2>
`)
		for _, alert := range report.Alerts {
			fmt.Fprintf(&buf, `<div class="alert">
    <strong>%s</strong>: %s (当前值: %.2f, 阈值: %.2f)
</div>
`, alert.Severity, alert.Message, alert.Value, alert.Threshold)
		}
	}

	// 建议
	if len(report.Recommendations) > 0 {
		buf.WriteString(`<h2>性能建议</h2>
`)
		for _, rec := range report.Recommendations {
			buf.WriteString(`<div class="recommendation">` + rec + `</div>
`)
		}
	}

	buf.WriteString(`</body>
</html>
`)

	return buf.Bytes(), nil
}

// exportMarkdown 导出Markdown格式
func (m *Manager) exportMarkdown(report *PerformanceReport) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("# NAS-OS 性能报告\n\n")
	fmt.Fprintf(&buf, "**生成时间**: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))

	// 摘要
	buf.WriteString("## 性能摘要\n\n")
	buf.WriteString("| 指标 | 值 |\n|------|------|\n")
	fmt.Fprintf(&buf, "| 总请求数 | %d |\n", report.Summary.TotalRequests)
	fmt.Fprintf(&buf, "| 总错误数 | %d |\n", report.Summary.TotalErrors)
	fmt.Fprintf(&buf, "| 错误率 | %.2f%% |\n", report.Summary.ErrorRate)
	fmt.Fprintf(&buf, "| 平均响应时间 | %.0fms |\n", report.Summary.AvgResponseTime)
	fmt.Fprintf(&buf, "| P95响应时间 | %.0fms |\n", report.Summary.P95ResponseTime)
	fmt.Fprintf(&buf, "| 峰值RPS | %.2f |\n\n", report.Summary.PeakRPS)

	// 端点
	buf.WriteString("## 端点性能\n\n")
	buf.WriteString("| 路径 | 方法 | 请求数 | 错误率 | 平均响应时间 | P95响应时间 |\n")
	buf.WriteString("|------|------|--------|--------|--------------|-------------|\n")
	for _, ep := range report.Endpoints {
		fmt.Fprintf(&buf, "| %s | %s | %d | %.2f%% | %.0fms | %.0fms |\n",
			ep.Path, ep.Method, ep.RequestCount, ep.ErrorRate, ep.AvgDuration, ep.P95Duration)
	}

	// 告警
	if len(report.Alerts) > 0 {
		buf.WriteString("\n## 活跃告警\n\n")
		for _, alert := range report.Alerts {
			fmt.Fprintf(&buf, "- **%s**: %s (当前值: %.2f, 阈值: %.2f)\n",
				alert.Severity, alert.Message, alert.Value, alert.Threshold)
		}
	}

	// 建议
	if len(report.Recommendations) > 0 {
		buf.WriteString("\n## 性能建议\n\n")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(&buf, "- %s\n", rec)
		}
	}

	return buf.Bytes(), nil
}
