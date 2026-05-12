// Package costdashboard 提供 REST API 处理器
package costdashboard

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 成本分析模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/cost 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cost := r.Group("/cost")
	{
		// 云提供商管理
		cost.POST("/providers", h.addProvider)
		cost.GET("/providers", h.listProviders)
		cost.PUT("/providers/:id", h.updateProvider)
		cost.DELETE("/providers/:id", h.removeProvider)
		cost.POST("/providers/:id/sync", h.syncMetrics)

		// 指标分析
		cost.GET("/metrics", h.getMetrics)
		cost.GET("/metrics/compare", h.compareProviders)
		cost.GET("/trend", h.getUsageTrend)
		cost.GET("/forecast/:id", h.forecastCost)

		// 报告生成
		cost.POST("/reports/generate", h.generateReport)
		cost.GET("/reports", h.listReports)
		cost.GET("/reports/:id", h.getReport)

		// 成本告警
		cost.GET("/alerts", h.getAlerts)
		cost.POST("/alerts", h.setAlert)
		cost.POST("/alerts/ack", h.acknowledgeAlert)

		// 优化建议
		cost.GET("/optimization", h.getRecommendations)
		cost.POST("/optimization/analyze", h.analyzeOptimization)
	}
}

// ========== 云提供商 Handlers ==========

func (h *Handlers) addProvider(c *gin.Context) {
	var req AddProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	// 校验 ProviderType
	validTypes := map[CloudProviderType]bool{
		ProviderAliyun:  true,
		ProviderTencent: true,
		ProviderAWS:     true,
		ProviderGDrive:  true,
		ProviderOneDrive: true,
	}
	if !validTypes[req.Type] {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid provider type, must be one of: aliyun, tencent, aws, gdrive, onedrive"})
		return
	}

	provider := h.manager.AddProvider(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: provider})
}

func (h *Handlers) listProviders(c *gin.Context) {
	providers := h.manager.ListProviders()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(providers),
			"providers": providers,
		},
	})
}

func (h *Handlers) updateProvider(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	provider, err := h.manager.UpdateProvider(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: provider})
}

func (h *Handlers) removeProvider(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveProvider(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) syncMetrics(c *gin.Context) {
	id := c.Param("id")
	metrics, err := h.manager.SyncMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "synced", Data: metrics})
}

// ========== 指标分析 Handlers ==========

func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(metrics),
			"metrics": metrics,
		},
	})
}

func (h *Handlers) compareProviders(c *gin.Context) {
	idsStr := c.Query("provider_ids")
	if idsStr == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'provider_ids' is required (comma-separated)"})
		return
	}

	ids := strings.Split(idsStr, ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}

	metrics, err := h.manager.CompareProviders(ids)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(metrics),
			"metrics": metrics,
		},
	})
}

func (h *Handlers) getUsageTrend(c *gin.Context) {
	providerID := c.Query("provider_id")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'provider_id' is required"})
		return
	}

	period := c.DefaultQuery("period", "weekly")

	trend, err := h.manager.GetUsageTrend(providerID, period)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"provider_id": providerID,
			"period":      period,
			"trend":       trend,
		},
	})
}

func (h *Handlers) forecastCost(c *gin.Context) {
	id := c.Param("id")
	monthsStr := c.DefaultQuery("months", "3")
	months, err := strconv.Atoi(monthsStr)
	if err != nil || months <= 0 {
		months = 3
	}

	forecast, err := h.manager.ForecastCost(id, months)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: forecast})
}

// ========== 报告生成 Handlers ==========

func (h *Handlers) generateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	validPeriods := map[ReportPeriod]bool{
		PeriodMonthly: true,
		PeriodWeekly:  true,
		PeriodDaily:   true,
	}
	if !validPeriods[req.Period] {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid period, must be one of: monthly, weekly, daily"})
		return
	}

	report := h.manager.GenerateReport(req.Period)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "generated", Data: report})
}

func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(reports),
			"reports": reports,
		},
	})
}

func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// ========== 成本告警 Handlers ==========

func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.manager.CheckAlerts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

func (h *Handlers) setAlert(c *gin.Context) {
	var req SetAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	validSeverities := map[AlertSeverity]bool{
		SeverityWarning:  true,
		SeverityCritical: true,
	}
	if !validSeverities[req.Severity] {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid severity, must be one of: warning, critical"})
		return
	}

	alert, err := h.manager.SetAlert(req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: alert})
}

func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	var req AcknowledgeAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.AcknowledgeAlert(req.AlertID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "acknowledged"})
}

// ========== 优化建议 Handlers ==========

func (h *Handlers) getRecommendations(c *gin.Context) {
	recs := h.manager.GetRecommendations()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":           len(recs),
			"recommendations": recs,
		},
	})
}

func (h *Handlers) analyzeOptimization(c *gin.Context) {
	recs := h.manager.AnalyzeOptimization()

	totalSaving := 0.0
	for _, r := range recs {
		totalSaving += r.PotentialSaving
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "analyzed",
		Data: gin.H{
			"total":           len(recs),
			"total_saving":    totalSaving,
			"recommendations": recs,
		},
	})
}
