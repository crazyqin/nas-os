// Package netflow - 流量分析HTTP处理器
// 注册到 /api/v1/netflow/ 路由
package netflow

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 流量分析HTTP处理器.
type Handler struct {
	collector *Collector
	analyzer  *Analyzer
	logger    *zap.Logger
}

// NewHandler 创建流量分析HTTP处理器.
func NewHandler(collector *Collector, analyzer *Analyzer, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		collector: collector,
		analyzer:  analyzer,
		logger:    logger,
	}
}

// RegisterRoutes 注册流量分析路由到 /api/v1/netflow/.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	nf := rg.Group("/netflow")
	{
		// 采集器控制
		nf.POST("/collector/start", h.StartCollector)
		nf.POST("/collector/stop", h.StopCollector)
		nf.GET("/collector/status", h.GetCollectorStatus)

		// 流量统计
		nf.GET("/stats", h.GetTrafficStats)
		nf.GET("/stats/protocols", h.GetProtocolStats)
		nf.GET("/stats/hosts", h.GetTopHosts)
		nf.GET("/stats/bandwidth", h.GetBandwidthHistory)

		// TopN分析
		nf.GET("/top/hosts", h.TopHosts)
		nf.GET("/top/protocols", h.TopProtocols)
		nf.GET("/top/conversations", h.TopConversations)

		// 异常检测
		nf.POST("/analyze", h.RunAnalysis)
		nf.GET("/alerts", h.GetAlerts)
		nf.GET("/alerts/stats", h.GetAlertStats)
		nf.PUT("/alerts/:id/resolve", h.ResolveAlert)

		// 配置
		nf.PUT("/config/thresholds", h.UpdateThresholds)
	}
}

// StartCollector handles POST /api/v1/netflow/collector/start.
func (h *Handler) StartCollector(c *gin.Context) {
	if h.collector.IsRunning() {
		c.JSON(http.StatusOK, gin.H{"message": "收集器已在运行中"})
		return
	}

	if err := h.collector.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "收集器已启动"})
}

// StopCollector handles POST /api/v1/netflow/collector/stop.
func (h *Handler) StopCollector(c *gin.Context) {
	if !h.collector.IsRunning() {
		c.JSON(http.StatusOK, gin.H{"message": "收集器未在运行"})
		return
	}

	h.collector.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "收集器已停止"})
}

// GetCollectorStatus handles GET /api/v1/netflow/collector/status.
func (h *Handler) GetCollectorStatus(c *gin.Context) {
	stats := h.collector.GetTrafficStats()
	c.JSON(http.StatusOK, gin.H{
		"running":            h.collector.IsRunning(),
		"total_bytes_in":     stats.TotalBytesIn,
		"total_bytes_out":    stats.TotalBytesOut,
		"total_packets_in":   stats.TotalPacketsIn,
		"total_packets_out":  stats.TotalPacketsOut,
		"active_connections": stats.ActiveConnections,
		"config":             h.collector.config,
	})
}

// GetTrafficStats handles GET /api/v1/netflow/stats.
func (h *Handler) GetTrafficStats(c *gin.Context) {
	stats := h.collector.GetTrafficStats()
	protocols := h.collector.GetProtocolStats()
	topHosts := h.collector.GetTopHosts(10)

	c.JSON(http.StatusOK, TrafficStatsResponse{
		Stats:     stats,
		Protocols: protocols,
		TopHosts:  topHosts,
	})
}

// GetProtocolStats handles GET /api/v1/netflow/stats/protocols.
func (h *Handler) GetProtocolStats(c *gin.Context) {
	protocols := h.collector.GetProtocolStats()
	c.JSON(http.StatusOK, gin.H{
		"protocols": protocols,
		"total":     len(protocols),
	})
}

// GetTopHosts handles GET /api/v1/netflow/stats/hosts.
func (h *Handler) GetTopHosts(c *gin.Context) {
	n := 10
	if nStr := c.Query("limit"); nStr != "" {
		if v, err := strconv.Atoi(nStr); err == nil && v > 0 {
			n = v
		}
	}

	hosts := h.collector.GetTopHosts(n)
	c.JSON(http.StatusOK, gin.H{
		"hosts": hosts,
		"total": len(hosts),
	})
}

// GetBandwidthHistory handles GET /api/v1/netflow/stats/bandwidth.
func (h *Handler) GetBandwidthHistory(c *gin.Context) {
	history := h.collector.GetBandwidthHistory()
	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
	})
}

// TopHosts handles GET /api/v1/netflow/top/hosts.
func (h *Handler) TopHosts(c *gin.Context) {
	n := 10
	if nStr := c.Query("limit"); nStr != "" {
		if v, err := strconv.Atoi(nStr); err == nil && v > 0 {
			n = v
		}
	}

	result := h.analyzer.TopHosts(n)
	c.JSON(http.StatusOK, result)
}

// TopProtocols handles GET /api/v1/netflow/top/protocols.
func (h *Handler) TopProtocols(c *gin.Context) {
	n := 10
	if nStr := c.Query("limit"); nStr != "" {
		if v, err := strconv.Atoi(nStr); err == nil && v > 0 {
			n = v
		}
	}

	result := h.analyzer.TopProtocols(n)
	c.JSON(http.StatusOK, result)
}

// TopConversations handles GET /api/v1/netflow/top/conversations.
func (h *Handler) TopConversations(c *gin.Context) {
	n := 10
	if nStr := c.Query("limit"); nStr != "" {
		if v, err := strconv.Atoi(nStr); err == nil && v > 0 {
			n = v
		}
	}

	result := h.analyzer.TopConversations(n)
	c.JSON(http.StatusOK, result)
}

// RunAnalysis handles POST /api/v1/netflow/analyze.
func (h *Handler) RunAnalysis(c *gin.Context) {
	alerts := h.analyzer.Analyze()
	c.JSON(http.StatusOK, gin.H{
		"new_alerts": alerts,
		"total":      len(alerts),
	})
}

// GetAlerts handles GET /api/v1/netflow/alerts.
func (h *Handler) GetAlerts(c *gin.Context) {
	limit := 100
	if lStr := c.Query("limit"); lStr != "" {
		if v, err := strconv.Atoi(lStr); err == nil && v > 0 {
			limit = v
		}
	}

	severity := c.Query("severity")
	anomalyType := c.Query("type")

	alerts := h.analyzer.GetAlerts(limit, severity, anomalyType)
	c.JSON(http.StatusOK, AlertListResponse{
		Alerts: alerts,
		Total:  len(alerts),
	})
}

// GetAlertStats handles GET /api/v1/netflow/alerts/stats.
func (h *Handler) GetAlertStats(c *gin.Context) {
	stats := h.analyzer.GetAlertStats()
	c.JSON(http.StatusOK, stats)
}

// ResolveAlert handles PUT /api/v1/netflow/alerts/:id/resolve.
func (h *Handler) ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.analyzer.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "告警已标记为已解决"})
}

// updateThresholdsReq 更新阈值请求.
type updateThresholdsReq struct {
	SpikeThresholdMBPS    *float64 `json:"spike_threshold_mbps"`
	PortScanThreshold     *int     `json:"port_scan_threshold"`
	DNSFloodThreshold     *int     `json:"dns_flood_threshold"`
	HighConnRateThreshold *int     `json:"high_conn_rate_threshold"`
}

// UpdateThresholds handles PUT /api/v1/netflow/config/thresholds.
func (h *Handler) UpdateThresholds(c *gin.Context) {
	var req updateThresholdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SpikeThresholdMBPS != nil {
		h.analyzer.SetSpikeThreshold(*req.SpikeThresholdMBPS)
	}
	if req.PortScanThreshold != nil {
		h.analyzer.SetPortScanThreshold(*req.PortScanThreshold)
	}
	if req.DNSFloodThreshold != nil {
		h.analyzer.SetDNSFloodThreshold(*req.DNSFloodThreshold)
	}
	if req.HighConnRateThreshold != nil {
		h.analyzer.SetHighConnRateThreshold(*req.HighConnRateThreshold)
	}

	c.JSON(http.StatusOK, gin.H{"message": "阈值配置已更新"})
}
