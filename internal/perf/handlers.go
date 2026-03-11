package perf

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 性能监控 HTTP 处理器
type Handlers struct {
	manager  *Manager
	analyzer *Analyzer
}

// NewHandlers 创建性能监控处理器
func NewHandlers(m *Manager) *Handlers {
	return &Handlers{
		manager:  m,
		analyzer: NewAnalyzer(m),
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	perf := r.Group("/perf")
	{
		// 性能指标
		perf.GET("/metrics", h.getMetrics)
		perf.GET("/metrics/endpoints", h.getEndpointMetrics)
		perf.GET("/metrics/endpoints/:path", h.getEndpointDetail)
		perf.GET("/metrics/throughput", h.getThroughput)
		perf.GET("/metrics/window", h.getTimeWindow)
		perf.GET("/metrics/baseline", h.getBaseline)

		// 慢查询日志
		perf.GET("/slow-logs", h.getSlowLogs)
		perf.DELETE("/slow-logs", h.clearSlowLogs)

		// 性能分析
		perf.GET("/analyze", h.analyze)
		perf.GET("/analyze/report", h.getReport)
		perf.GET("/analyze/health", h.getHealthScore)
		perf.GET("/analyze/anomalies", h.getAnomalies)
		perf.GET("/analyze/recommendations", h.getRecommendations)

		// 性能对比
		perf.GET("/compare", h.compareWithBaseline)

		// 配置
		perf.GET("/config", h.getConfig)
		perf.PUT("/config/threshold", h.updateThreshold)
	}
}

// getMetrics 获取整体性能指标
func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalRequests":   metrics.TotalRequests,
			"totalErrors":     metrics.TotalErrors,
			"totalDuration":   metrics.TotalDuration.String(),
			"avgResponseTime": metrics.AvgResponseTime.String(),
			"endpointCount":   len(metrics.Endpoints),
		},
	})
}

// getEndpointMetrics 获取端点指标列表
func (h *Handlers) getEndpointMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()

	endpoints := make([]gin.H, 0)
	for key, em := range metrics.Endpoints {
		var errorRate float64
		if em.RequestCount > 0 {
			errorRate = float64(em.ErrorCount) / float64(em.RequestCount) * 100
		}

		endpoints = append(endpoints, gin.H{
			"key":          key,
			"path":         em.Path,
			"method":       em.Method,
			"requestCount": em.RequestCount,
			"errorCount":   em.ErrorCount,
			"errorRate":    errorRate,
			"avgDuration":  em.AvgDuration.String(),
			"minDuration":  em.MinDuration.String(),
			"maxDuration":  em.MaxDuration.String(),
			"p50Duration":  em.P50Duration.String(),
			"p95Duration":  em.P95Duration.String(),
			"p99Duration":  em.P99Duration.String(),
			"lastAccess":   em.LastAccessTime.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    endpoints,
	})
}

// getEndpointDetail 获取特定端点详情
func (h *Handlers) getEndpointDetail(c *gin.Context) {
	path := c.Param("path")
	method := c.Query("method")
	if method == "" {
		method = "GET"
	}

	em := h.manager.GetEndpointMetrics(path, method)
	if em == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "端点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"path":         em.Path,
			"method":       em.Method,
			"requestCount": em.RequestCount,
			"errorCount":   em.ErrorCount,
			"avgDuration":  em.AvgDuration.String(),
			"minDuration":  em.MinDuration.String(),
			"maxDuration":  em.MaxDuration.String(),
			"p50Duration":  em.P50Duration.String(),
			"p95Duration":  em.P95Duration.String(),
			"p99Duration":  em.P99Duration.String(),
			"lastAccess":   em.LastAccessTime.Format("2006-01-02 15:04:05"),
		},
	})
}

// getThroughput 获取吞吐量统计
func (h *Handlers) getThroughput(c *gin.Context) {
	stats := h.manager.GetThroughputStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getTimeWindow 获取时间窗口统计
func (h *Handlers) getTimeWindow(c *gin.Context) {
	stats := h.manager.GetTimeWindowStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getBaseline 获取性能基线
func (h *Handlers) getBaseline(c *gin.Context) {
	baseline := h.manager.GetBaseline()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"avgResponseTime": baseline.AvgResponseTime,
			"p95ResponseTime": baseline.P95ResponseTime,
			"p99ResponseTime": baseline.P99ResponseTime,
			"avgRPS":          baseline.AvgRPS,
			"peakRPS":         baseline.PeakRPS,
			"avgErrorRate":    baseline.AvgErrorRate,
			"lastUpdated":     baseline.LastUpdated.Format("2006-01-02 15:04:05"),
		},
	})
}

// getSlowLogs 获取慢请求日志
func (h *Handlers) getSlowLogs(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs := h.manager.GetSlowLogs(limit)

	result := make([]gin.H, 0, len(logs))
	for _, entry := range logs {
		result = append(result, gin.H{
			"timestamp":   entry.Timestamp.Format("2006-01-02 15:04:05.000"),
			"requestId":   entry.RequestID,
			"method":      entry.Method,
			"path":        entry.Path,
			"query":       entry.Query,
			"clientIP":    entry.ClientIP,
			"duration":    entry.Duration.String(),
			"durationMs":  entry.Duration.Milliseconds(),
			"statusCode":  entry.StatusCode,
			"userAgent":   entry.UserAgent,
			"requestSize": entry.RequestSize,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": len(h.manager.GetSlowLogs(0)),
			"logs":  result,
		},
	})
}

// clearSlowLogs 清除慢请求日志
func (h *Handlers) clearSlowLogs(c *gin.Context) {
	h.manager.mu.Lock()
	h.manager.slowLogs = make([]*SlowLogEntry, 0)
	h.manager.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "慢请求日志已清除",
	})
}

// analyze 执行完整分析
func (h *Handlers) analyze(c *gin.Context) {
	report := h.analyzer.Analyze()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// getReport 获取文本报告
func (h *Handlers) getReport(c *gin.Context) {
	report := h.analyzer.GenerateTextReport()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"report": report,
		},
	})
}

// getHealthScore 获取健康分数
func (h *Handlers) getHealthScore(c *gin.Context) {
	score := h.analyzer.GetHealthScore()
	report := h.analyzer.Analyze()

	var status string
	switch {
	case score >= 90:
		status = "excellent"
	case score >= 70:
		status = "good"
	case score >= 50:
		status = "fair"
	case score >= 30:
		status = "poor"
	default:
		status = "critical"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"score":  score,
			"status": status,
			"summary": gin.H{
				"avgLatency":    report.Summary.AvgLatencyMs,
				"errorRate":     report.Summary.ErrorRate,
				"currentRPS":    report.Summary.CurrentRPS,
				"slowRequests":  report.Summary.SlowRequestCount,
				"anomalyCount":  len(report.Anomalies),
			},
		},
	})
}

// getAnomalies 获取异常列表
func (h *Handlers) getAnomalies(c *gin.Context) {
	report := h.analyzer.Analyze()

	anomalies := make([]gin.H, 0, len(report.Anomalies))
	for _, a := range report.Anomalies {
		anomalies = append(anomalies, gin.H{
			"type":      a.Type,
			"severity":  a.Severity,
			"endpoint":  a.Endpoint,
			"value":     a.Value,
			"threshold": a.Threshold,
			"message":   a.Message,
			"timestamp": a.Timestamp.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    anomalies,
	})
}

// getRecommendations 获取优化建议
func (h *Handlers) getRecommendations(c *gin.Context) {
	report := h.analyzer.Analyze()

	recs := make([]gin.H, 0, len(report.Recommendations))
	for _, r := range report.Recommendations {
		recs = append(recs, gin.H{
			"priority":    r.Priority,
			"category":    r.Category,
			"title":       r.Title,
			"description": r.Description,
			"impact":      r.Impact,
			"endpoint":    r.Endpoint,
			"timestamp":   r.Timestamp.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    recs,
	})
}

// compareWithBaseline 与基线对比
func (h *Handlers) compareWithBaseline(c *gin.Context) {
	report := h.analyzer.Analyze()
	baseline := h.manager.GetBaseline()

	// 计算偏差百分比
	var latencyDeviation, errorRateDeviation, rpsDeviation float64

	if baseline.AvgResponseTime > 0 {
		latencyDeviation = (report.Summary.AvgLatencyMs - baseline.AvgResponseTime) / baseline.AvgResponseTime * 100
	}
	if baseline.AvgErrorRate > 0 {
		errorRateDeviation = (report.Summary.ErrorRate - baseline.AvgErrorRate) / baseline.AvgErrorRate * 100
	}
	if baseline.AvgRPS > 0 {
		rpsDeviation = (report.Summary.CurrentRPS - baseline.AvgRPS) / baseline.AvgRPS * 100
	}

	// 判断趋势
	trends := gin.H{
		"latency":  "stable",
		"errorRate": "stable",
		"rps":      "stable",
	}

	if latencyDeviation > 20 {
		trends["latency"] = "degrading"
	} else if latencyDeviation < -20 {
		trends["latency"] = "improving"
	}

	if errorRateDeviation > 20 {
		trends["errorRate"] = "degrading"
	} else if errorRateDeviation < -20 {
		trends["errorRate"] = "improving"
	}

	if rpsDeviation > 20 {
		trends["rps"] = "improving"
	} else if rpsDeviation < -20 {
		trends["rps"] = "degrading"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"current": gin.H{
				"avgLatency": report.Summary.AvgLatencyMs,
				"errorRate":  report.Summary.ErrorRate,
				"rps":        report.Summary.CurrentRPS,
			},
			"baseline": gin.H{
				"avgLatency": baseline.AvgResponseTime,
				"errorRate":  baseline.AvgErrorRate,
				"rps":        baseline.AvgRPS,
			},
			"deviation": gin.H{
				"latency":   latencyDeviation,
				"errorRate": errorRateDeviation,
				"rps":       rpsDeviation,
			},
			"trends": trends,
		},
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"slowThreshold": h.manager.slowThreshold.String(),
			"slowLogMax":    h.manager.slowLogMax,
			"slowLogPath":   h.manager.slowLogPath,
		},
	})
}

// updateThreshold 更新慢请求阈值
func (h *Handlers) updateThreshold(c *gin.Context) {
	var req struct {
		ThresholdMs int64 `json:"thresholdMs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	if req.ThresholdMs < 10 || req.ThresholdMs > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "阈值必须在 10ms 到 60000ms 之间",
		})
		return
	}

	h.manager.mu.Lock()
	h.manager.slowThreshold = DurationFromMs(req.ThresholdMs)
	h.manager.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "阈值已更新",
	})
}

// parseInt 辅助函数：解析整数
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// DurationFromMs 从毫秒创建 Duration
func DurationFromMs(ms int64) time.Duration {
	return time.Duration(ms) * time.Millisecond
}