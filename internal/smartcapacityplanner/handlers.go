// Package smartcapacityplanner 提供 REST API 处理器
package smartcapacityplanner

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 智能容量规划 API 处理器.
type Handlers struct {
	manager *PlannerManager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *PlannerManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cp := r.Group("/storage/capacity")
	{
		// 快照接口
		cp.POST("/snapshots", h.RecordUsage)
		cp.GET("/snapshots", h.ListSnapshots)
		cp.GET("/snapshots/latest", h.GetLatestSnapshot)

		// 预测接口
		cp.POST("/forecast", h.ForecastCapacity)
		cp.GET("/forecasts", h.GetForecasts)

		// 趋势接口
		cp.GET("/trend", h.GetGrowthTrend)

		// 规划接口
		cp.POST("/plan", h.GeneratePlan)
		cp.GET("/plans", h.GetPlans)

		// 告警接口
		cp.GET("/alerts", h.ListAlerts)
		cp.POST("/alerts/trigger", h.TriggerAlert)
		cp.PUT("/alerts/:id/read", h.MarkAlertRead)
		cp.DELETE("/alerts", h.ClearAlerts)
		cp.GET("/alerts/thresholds", h.GetAlertThresholds)
		cp.PUT("/alerts/thresholds", h.SetAlertThresholds)

		// 清除历史
		cp.DELETE("/history", h.ClearHistory)
	}
}

// ========== 快照接口 ==========

// RecordUsage 记录使用量.
func (h *Handlers) RecordUsage(c *gin.Context) {
	var req RecordUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	snapshot, err := h.manager.RecordUsage(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "recorded", Data: snapshot})
}

// ListSnapshots 列出快照.
func (h *Handlers) ListSnapshots(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	snapshots := h.manager.ListSnapshots(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(snapshots),
			"snapshots": snapshots,
		},
	})
}

// GetLatestSnapshot 获取最新快照.
func (h *Handlers) GetLatestSnapshot(c *gin.Context) {
	mountPoint := c.Query("mount_point")

	snapshot, err := h.manager.GetLatestSnapshot(mountPoint)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: snapshot})
}

// ========== 预测接口 ==========

// ForecastCapacity 预测容量.
func (h *Handlers) ForecastCapacity(c *gin.Context) {
	var req ForecastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result, err := h.manager.ForecastCapacity(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "forecast completed", Data: result})
}

// GetForecasts 获取预测结果.
func (h *Handlers) GetForecasts(c *gin.Context) {
	forecasts := h.manager.GetForecasts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(forecasts),
			"forecasts": forecasts,
		},
	})
}

// ========== 趋势接口 ==========

// GetGrowthTrend 获取增长趋势.
func (h *Handlers) GetGrowthTrend(c *gin.Context) {
	mountPoint := c.Query("mount_point")
	period := c.DefaultQuery("period", "daily")

	trend, err := h.manager.GetGrowthTrend(mountPoint, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: trend})
}

// ========== 规划接口 ==========

// GeneratePlan 生成规划建议.
func (h *Handlers) GeneratePlan(c *gin.Context) {
	var req PlanRequest
	// 请求体是可选的
	_ = c.ShouldBindJSON(&req)

	plan, err := h.manager.GeneratePlan(req.MountPoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "plan generated", Data: plan})
}

// GetPlans 获取规划建议.
func (h *Handlers) GetPlans(c *gin.Context) {
	plans := h.manager.GetPlans()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(plans),
			"plans": plans,
		},
	})
}

// ========== 告警接口 ==========

// ListAlerts 列出告警.
func (h *Handlers) ListAlerts(c *gin.Context) {
	unreadOnlyStr := c.DefaultQuery("unread_only", "false")
	unreadOnly := unreadOnlyStr == "true"

	alerts := h.manager.ListAlerts(unreadOnly)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

// TriggerAlert 触发告警检查.
func (h *Handlers) TriggerAlert(c *gin.Context) {
	mountPoint := c.Query("mount_point")

	alerts, err := h.manager.TriggerAlert(mountPoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "alert check completed",
		Data: gin.H{
			"new_alerts": len(alerts),
			"alerts":     alerts,
		},
	})
}

// MarkAlertRead 标记告警为已读.
func (h *Handlers) MarkAlertRead(c *gin.Context) {
	alertID := c.Param("id")

	if err := h.manager.MarkAlertRead(alertID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "marked as read"})
}

// ClearAlerts 清除告警.
func (h *Handlers) ClearAlerts(c *gin.Context) {
	h.manager.ClearAlerts()
	c.JSON(http.StatusOK, response{Code: 0, Message: "alerts cleared"})
}

// GetAlertThresholds 获取告警阈值.
func (h *Handlers) GetAlertThresholds(c *gin.Context) {
	warning, critical := h.manager.GetAlertThresholds()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"warning_threshold":  warning,
			"critical_threshold": critical,
		},
	})
}

// SetAlertThresholds 设置告警阈值.
func (h *Handlers) SetAlertThresholds(c *gin.Context) {
	var req AlertConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.SetAlertThresholds(req.WarningThreshold, req.CriticalThreshold); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "thresholds updated"})
}

// ClearHistory 清除历史.
func (h *Handlers) ClearHistory(c *gin.Context) {
	h.manager.ClearHistory()
	c.JSON(http.StatusOK, response{Code: 0, Message: "history cleared"})
}
