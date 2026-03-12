package perf

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Analyzer 性能分析器
type Analyzer struct {
	manager *Manager
}

// NewAnalyzer 创建性能分析器
func NewAnalyzer(m *Manager) *Analyzer {
	return &Analyzer{manager: m}
}

// AnalysisReport 性能分析报告
type AnalysisReport struct {
	Timestamp         time.Time         `json:"timestamp"`
	Summary           *SummaryStats     `json:"summary"`
	TopSlowEndpoints  []*EndpointInfo   `json:"topSlowEndpoints"`
	TopBusyEndpoints  []*EndpointInfo   `json:"topBusyEndpoints"`
	TopErrorEndpoints []*EndpointInfo   `json:"topErrorEndpoints"`
	SlowQueries       []*SlowLogEntry   `json:"slowQueries"`
	Anomalies         []*Anomaly        `json:"anomalies"`
	Recommendations   []*Recommendation `json:"recommendations"`
}

// SummaryStats 汇总统计
type SummaryStats struct {
	TotalRequests    uint64  `json:"totalRequests"`
	TotalErrors      uint64  `json:"totalErrors"`
	ErrorRate        float64 `json:"errorRate"`
	AvgLatencyMs     float64 `json:"avgLatencyMs"`
	P95LatencyMs     float64 `json:"p95LatencyMs"`
	P99LatencyMs     float64 `json:"p99LatencyMs"`
	CurrentRPS       float64 `json:"currentRPS"`
	PeakRPS          float64 `json:"peakRPS"`
	AvgResponseSize  int64   `json:"avgResponseSize"`
	SlowRequestCount int     `json:"slowRequestCount"`
}

// EndpointInfo 端点信息
type EndpointInfo struct {
	Path         string        `json:"path"`
	Method       string        `json:"method"`
	RequestCount uint64        `json:"requestCount"`
	ErrorRate    float64       `json:"errorRate"`
	AvgLatency   time.Duration `json:"avgLatency"`
	P95Latency   time.Duration `json:"p95Latency"`
	Impact       float64       `json:"impact"` // 影响分数 (0-100)
}

// Anomaly 异常检测
type Anomaly struct {
	Type      string    `json:"type"`     // latency, error_rate, throughput
	Severity  string    `json:"severity"` // warning, critical
	Endpoint  string    `json:"endpoint"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Recommendation 优化建议
type Recommendation struct {
	Priority    int       `json:"priority"` // 1-5, 1 最高
	Category    string    `json:"category"` // performance, reliability, capacity
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Impact      string    `json:"impact"` // 预期影响
	Endpoint    string    `json:"endpoint,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Analyze 执行性能分析
func (a *Analyzer) Analyze() *AnalysisReport {
	report := &AnalysisReport{
		Timestamp:         time.Now(),
		TopSlowEndpoints:  make([]*EndpointInfo, 0),
		TopBusyEndpoints:  make([]*EndpointInfo, 0),
		TopErrorEndpoints: make([]*EndpointInfo, 0),
		SlowQueries:       make([]*SlowLogEntry, 0),
		Anomalies:         make([]*Anomaly, 0),
		Recommendations:   make([]*Recommendation, 0),
	}

	// 收集汇总统计
	report.Summary = a.collectSummary()

	// 分析端点
	report.TopSlowEndpoints = a.findSlowEndpoints(10)
	report.TopBusyEndpoints = a.findBusyEndpoints(10)
	report.TopErrorEndpoints = a.findErrorEndpoints(10)

	// 获取慢请求
	report.SlowQueries = a.manager.GetSlowLogs(50)

	// 检测异常
	report.Anomalies = a.detectAnomalies()

	// 生成优化建议
	report.Recommendations = a.generateRecommendations(report)

	return report
}

// collectSummary 收集汇总统计
func (a *Analyzer) collectSummary() *SummaryStats {
	metrics := a.manager.GetMetrics()
	throughput := a.manager.GetThroughputStats()
	baseline := a.manager.GetBaseline()
	slowLogs := a.manager.GetSlowLogs(0)

	s := &SummaryStats{
		TotalRequests:    metrics.TotalRequests,
		TotalErrors:      metrics.TotalErrors,
		AvgLatencyMs:     baseline.AvgResponseTime,
		P95LatencyMs:     baseline.P95ResponseTime,
		P99LatencyMs:     baseline.P99ResponseTime,
		CurrentRPS:       throughput["currentRPS"].(float64),
		PeakRPS:          throughput["peakRPS"].(float64),
		SlowRequestCount: len(slowLogs),
	}

	if metrics.TotalRequests > 0 {
		s.ErrorRate = float64(metrics.TotalErrors) / float64(metrics.TotalRequests) * 100
	}

	return s
}

// findSlowEndpoints 查找最慢的端点
func (a *Analyzer) findSlowEndpoints(limit int) []*EndpointInfo {
	metrics := a.manager.GetMetrics()

	var endpoints []*EndpointInfo
	for _, em := range metrics.Endpoints {
		if em.RequestCount == 0 {
			continue
		}
		info := &EndpointInfo{
			Path:         em.Path,
			Method:       em.Method,
			RequestCount: em.RequestCount,
			AvgLatency:   em.AvgDuration,
			P95Latency:   em.P95Duration,
		}
		if em.RequestCount > 0 {
			info.ErrorRate = float64(em.ErrorCount) / float64(em.RequestCount) * 100
		}
		// 计算影响分数 (延迟 * 请求数)
		info.Impact = float64(em.AvgDuration.Milliseconds()) * float64(em.RequestCount) / 1000
		endpoints = append(endpoints, info)
	}

	// 按平均延迟排序
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].AvgLatency > endpoints[j].AvgLatency
	})

	if len(endpoints) > limit {
		endpoints = endpoints[:limit]
	}

	return endpoints
}

// findBusyEndpoints 查找最繁忙的端点
func (a *Analyzer) findBusyEndpoints(limit int) []*EndpointInfo {
	metrics := a.manager.GetMetrics()

	var endpoints []*EndpointInfo
	for _, em := range metrics.Endpoints {
		if em.RequestCount == 0 {
			continue
		}
		info := &EndpointInfo{
			Path:         em.Path,
			Method:       em.Method,
			RequestCount: em.RequestCount,
			AvgLatency:   em.AvgDuration,
			P95Latency:   em.P95Duration,
		}
		if em.RequestCount > 0 {
			info.ErrorRate = float64(em.ErrorCount) / float64(em.RequestCount) * 100
		}
		endpoints = append(endpoints, info)
	}

	// 按请求数排序
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].RequestCount > endpoints[j].RequestCount
	})

	if len(endpoints) > limit {
		endpoints = endpoints[:limit]
	}

	return endpoints
}

// findErrorEndpoints 查找错误率最高的端点
func (a *Analyzer) findErrorEndpoints(limit int) []*EndpointInfo {
	metrics := a.manager.GetMetrics()

	var endpoints []*EndpointInfo
	for _, em := range metrics.Endpoints {
		if em.RequestCount == 0 || em.ErrorCount == 0 {
			continue
		}
		info := &EndpointInfo{
			Path:         em.Path,
			Method:       em.Method,
			RequestCount: em.RequestCount,
			AvgLatency:   em.AvgDuration,
		}
		info.ErrorRate = float64(em.ErrorCount) / float64(em.RequestCount) * 100
		// 计算影响分数 (错误率 * 请求数)
		info.Impact = info.ErrorRate * float64(em.RequestCount) / 100
		endpoints = append(endpoints, info)
	}

	// 按错误率排序
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ErrorRate > endpoints[j].ErrorRate
	})

	if len(endpoints) > limit {
		endpoints = endpoints[:limit]
	}

	return endpoints
}

// detectAnomalies 检测异常
func (a *Analyzer) detectAnomalies() []*Anomaly {
	anomalies := make([]*Anomaly, 0)
	now := time.Now()

	// 获取基线
	baseline := a.manager.GetBaseline()
	metrics := a.manager.GetMetrics()

	// 检查响应时间异常
	avgLatency := baseline.AvgResponseTime
	if avgLatency > 500 { // 平均响应时间超过 500ms
		severity := "warning"
		if avgLatency > 1000 {
			severity = "critical"
		}
		anomalies = append(anomalies, &Anomaly{
			Type:      "latency",
			Severity:  severity,
			Value:     avgLatency,
			Threshold: 500,
			Message:   fmt.Sprintf("平均响应时间 %.2fms 超过正常阈值", avgLatency),
			Timestamp: now,
		})
	}

	// 检查 P95 响应时间
	p95Latency := baseline.P95ResponseTime
	if p95Latency > 1000 { // P95 超过 1 秒
		anomalies = append(anomalies, &Anomaly{
			Type:      "latency_p95",
			Severity:  "warning",
			Value:     p95Latency,
			Threshold: 1000,
			Message:   fmt.Sprintf("P95 响应时间 %.2fms 过高", p95Latency),
			Timestamp: now,
		})
	}

	// 检查错误率
	errorRate := float64(0)
	if metrics.TotalRequests > 0 {
		errorRate = float64(metrics.TotalErrors) / float64(metrics.TotalRequests) * 100
	}
	if errorRate > 5 { // 错误率超过 5%
		severity := "warning"
		if errorRate > 10 {
			severity = "critical"
		}
		anomalies = append(anomalies, &Anomaly{
			Type:      "error_rate",
			Severity:  severity,
			Value:     errorRate,
			Threshold: 5,
			Message:   fmt.Sprintf("错误率 %.2f%% 超过正常阈值", errorRate),
			Timestamp: now,
		})
	}

	// 检查慢请求比例
	slowLogs := a.manager.GetSlowLogs(0)
	if metrics.TotalRequests > 0 {
		slowRate := float64(len(slowLogs)) / float64(metrics.TotalRequests) * 100
		if slowRate > 1 { // 慢请求超过 1%
			anomalies = append(anomalies, &Anomaly{
				Type:      "slow_requests",
				Severity:  "warning",
				Value:     slowRate,
				Threshold: 1,
				Message:   fmt.Sprintf("慢请求比例 %.2f%% 过高", slowRate),
				Timestamp: now,
			})
		}
	}

	// 检查每个端点的异常
	for key, em := range metrics.Endpoints {
		if em.RequestCount < 10 {
			continue // 样本太少，跳过
		}

		// 检查端点错误率
		endpointErrorRate := float64(em.ErrorCount) / float64(em.RequestCount) * 100
		if endpointErrorRate > 10 {
			anomalies = append(anomalies, &Anomaly{
				Type:      "endpoint_error_rate",
				Severity:  "critical",
				Endpoint:  key,
				Value:     endpointErrorRate,
				Threshold: 10,
				Message:   fmt.Sprintf("端点 %s 错误率 %.2f%% 过高", key, endpointErrorRate),
				Timestamp: now,
			})
		}

		// 检查端点延迟
		avgDuration := em.AvgDuration.Milliseconds()
		if avgDuration > 1000 {
			anomalies = append(anomalies, &Anomaly{
				Type:      "endpoint_latency",
				Severity:  "warning",
				Endpoint:  key,
				Value:     float64(avgDuration),
				Threshold: 1000,
				Message:   fmt.Sprintf("端点 %s 平均响应时间 %dms 过慢", key, avgDuration),
				Timestamp: now,
			})
		}
	}

	return anomalies
}

// generateRecommendations 生成优化建议
func (a *Analyzer) generateRecommendations(report *AnalysisReport) []*Recommendation {
	recs := make([]*Recommendation, 0)
	now := time.Now()

	// 基于整体性能的建议
	if report.Summary.AvgLatencyMs > 300 {
		recs = append(recs, &Recommendation{
			Priority:    1,
			Category:    "performance",
			Title:       "优化整体响应时间",
			Description: fmt.Sprintf("当前平均响应时间 %.2fms 偏高，建议检查数据库查询、缓存策略和网络延迟", report.Summary.AvgLatencyMs),
			Impact:      "提升用户体验，降低请求超时风险",
			Timestamp:   now,
		})
	}

	if report.Summary.ErrorRate > 3 {
		recs = append(recs, &Recommendation{
			Priority:    1,
			Category:    "reliability",
			Title:       "降低错误率",
			Description: fmt.Sprintf("当前错误率 %.2f%% 过高，建议检查日志定位错误根因并修复", report.Summary.ErrorRate),
			Impact:      "提高服务稳定性，减少用户投诉",
			Timestamp:   now,
		})
	}

	// 基于慢端点的建议
	for _, ep := range report.TopSlowEndpoints {
		if ep.AvgLatency > 500*time.Millisecond {
			recs = append(recs, &Recommendation{
				Priority:    2,
				Category:    "performance",
				Title:       "优化慢端点响应",
				Description: fmt.Sprintf("端点 %s %s 平均响应 %v，建议添加缓存或优化查询", ep.Method, ep.Path, ep.AvgLatency),
				Impact:      "减少该端点响应时间，提升吞吐量",
				Endpoint:    ep.Method + ":" + ep.Path,
				Timestamp:   now,
			})
		}
	}

	// 基于高错误端点的建议
	for _, ep := range report.TopErrorEndpoints {
		if ep.ErrorRate > 5 {
			recs = append(recs, &Recommendation{
				Priority:    2,
				Category:    "reliability",
				Title:       "修复高错误率端点",
				Description: fmt.Sprintf("端点 %s %s 错误率 %.2f%%，建议检查错误日志并修复", ep.Method, ep.Path, ep.ErrorRate),
				Impact:      "降低错误率，提高服务可用性",
				Endpoint:    ep.Method + ":" + ep.Path,
				Timestamp:   now,
			})
		}
	}

	// 基于吞吐量的建议
	if report.Summary.PeakRPS > 100 {
		recs = append(recs, &Recommendation{
			Priority:    3,
			Category:    "capacity",
			Title:       "考虑水平扩展",
			Description: fmt.Sprintf("峰值 RPS 达到 %.2f，建议评估是否需要增加实例或优化资源", report.Summary.PeakRPS),
			Impact:      "提高系统容量，应对流量高峰",
			Timestamp:   now,
		})
	}

	// 基于慢请求的建议
	if len(report.SlowQueries) > 10 {
		// 分析慢请求模式
		pathCounts := make(map[string]int)
		for _, sq := range report.SlowQueries {
			pathCounts[sq.Path]++
		}

		var topPath string
		var topCount int
		for path, count := range pathCounts {
			if count > topCount {
				topPath = path
				topCount = count
			}
		}

		if topCount > 5 {
			recs = append(recs, &Recommendation{
				Priority:    2,
				Category:    "performance",
				Title:       "优化高频慢请求路径",
				Description: fmt.Sprintf("路径 %s 有 %d 次慢请求，建议添加索引、缓存或异步处理", topPath, topCount),
				Impact:      "显著减少慢请求数量",
				Endpoint:    topPath,
				Timestamp:   now,
			})
		}
	}

	// 基于异常的建议
	criticalAnomalies := 0
	for _, anomaly := range report.Anomalies {
		if anomaly.Severity == "critical" {
			criticalAnomalies++
		}
	}
	if criticalAnomalies > 0 {
		recs = append(recs, &Recommendation{
			Priority:    1,
			Category:    "reliability",
			Title:       "处理严重异常",
			Description: fmt.Sprintf("检测到 %d 个严重性能异常，建议立即处理", criticalAnomalies),
			Impact:      "防止服务进一步恶化",
			Timestamp:   now,
		})
	}

	// 按优先级排序
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Priority < recs[j].Priority
	})

	return recs
}

// GetHealthScore 计算健康分数 (0-100)
func (a *Analyzer) GetHealthScore() int {
	report := a.Analyze()
	score := 100

	// 响应时间扣分
	if report.Summary.AvgLatencyMs > 100 {
		deduction := int((report.Summary.AvgLatencyMs - 100) / 10)
		if deduction > 20 {
			deduction = 20
		}
		score -= deduction
	}

	// 错误率扣分
	if report.Summary.ErrorRate > 1 {
		deduction := int(report.Summary.ErrorRate * 2)
		if deduction > 30 {
			deduction = 30
		}
		score -= deduction
	}

	// 慢请求扣分
	slowRatio := float64(report.Summary.SlowRequestCount) / float64(max(report.Summary.TotalRequests, 1)) * 100
	if slowRatio > 0.5 {
		deduction := int(slowRatio * 2)
		if deduction > 20 {
			deduction = 20
		}
		score -= deduction
	}

	// 严重异常扣分
	for _, anomaly := range report.Anomalies {
		if anomaly.Severity == "critical" {
			score -= 10
		} else if anomaly.Severity == "warning" {
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// GenerateTextReport 生成文本报告
func (a *Analyzer) GenerateTextReport() string {
	report := a.Analyze()
	var sb strings.Builder

	sb.WriteString("=" + strings.Repeat("=", 60) + "\n")
	sb.WriteString("         NAS-OS 性能分析报告\n")
	sb.WriteString("=" + strings.Repeat("=", 60) + "\n\n")

	sb.WriteString(fmt.Sprintf("生成时间: %s\n", report.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("健康分数: %d/100\n\n", a.GetHealthScore()))

	// 汇总统计
	sb.WriteString("【汇总统计】\n")
	sb.WriteString(fmt.Sprintf("  总请求数: %d\n", report.Summary.TotalRequests))
	sb.WriteString(fmt.Sprintf("  总错误数: %d (%.2f%%)\n", report.Summary.TotalErrors, report.Summary.ErrorRate))
	sb.WriteString(fmt.Sprintf("  平均延迟: %.2fms\n", report.Summary.AvgLatencyMs))
	sb.WriteString(fmt.Sprintf("  P95 延迟: %.2fms\n", report.Summary.P95LatencyMs))
	sb.WriteString(fmt.Sprintf("  当前 RPS: %.2f\n", report.Summary.CurrentRPS))
	sb.WriteString(fmt.Sprintf("  峰值 RPS:  %.2f\n", report.Summary.PeakRPS))
	sb.WriteString(fmt.Sprintf("  慢请求数: %d\n\n", report.Summary.SlowRequestCount))

	// 最慢端点
	if len(report.TopSlowEndpoints) > 0 {
		sb.WriteString("【最慢端点 TOP 5】\n")
		for i, ep := range report.TopSlowEndpoints {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %d. %s %s - 平均: %v, P95: %v\n",
				i+1, ep.Method, ep.Path, ep.AvgLatency, ep.P95Latency))
		}
		sb.WriteString("\n")
	}

	// 高错误端点
	if len(report.TopErrorEndpoints) > 0 {
		sb.WriteString("【高错误率端点】\n")
		for i, ep := range report.TopErrorEndpoints {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %d. %s %s - 错误率: %.2f%% (%d/%d)\n",
				i+1, ep.Method, ep.Path, ep.ErrorRate, ep.RequestCount, ep.RequestCount))
		}
		sb.WriteString("\n")
	}

	// 异常检测
	if len(report.Anomalies) > 0 {
		sb.WriteString("【检测到的异常】\n")
		for _, anomaly := range report.Anomalies {
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", strings.ToUpper(anomaly.Severity), anomaly.Type, anomaly.Message))
		}
		sb.WriteString("\n")
	}

	// 优化建议
	if len(report.Recommendations) > 0 {
		sb.WriteString("【优化建议】\n")
		for i, rec := range report.Recommendations {
			sb.WriteString(fmt.Sprintf("\n  %d. [%s] %s (优先级: %d)\n", i+1, rec.Category, rec.Title, rec.Priority))
			sb.WriteString(fmt.Sprintf("     描述: %s\n", rec.Description))
			sb.WriteString(fmt.Sprintf("     影响: %s\n", rec.Impact))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=" + strings.Repeat("=", 60) + "\n")

	return sb.String()
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
