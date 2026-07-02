// Package energydashboard 提供 REST API 处理器
package energydashboard

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 能耗仪表盘 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	energy := r.Group("/energy")
	{
		// 仪表盘总览
		energy.GET("/summary", h.getSummary)
		energy.GET("/dashboard", h.getDashboard)

		// 实时功耗监控
		energy.GET("/power/latest", h.getLatestPower)
		energy.GET("/power/history", h.getPowerHistory)
		energy.POST("/power/reading", h.recordPowerReading)

		// 电价管理
		energy.GET("/rates", h.listRates)
		energy.POST("/rates", h.createRate)
		energy.GET("/rates/:id", h.getRate)
		energy.PUT("/rates/:id", h.updateRate)
		energy.DELETE("/rates/:id", h.deleteRate)

		// 能耗费用计算
		energy.GET("/cost", h.calculateCost)

		// 能效评分
		energy.GET("/efficiency", h.getEfficiencyScore)

		// 碳排放估算
		energy.GET("/carbon", h.estimateCarbon)

		// 能耗报表
		energy.GET("/reports", h.listReports)
		energy.POST("/reports", h.generateReport)
		energy.GET("/reports/:id", h.getReport)

		// 休眠计划管理
		energy.GET("/schedules", h.listSchedules)
		energy.POST("/schedules", h.createSchedule)
		energy.GET("/schedules/:id", h.getSchedule)
		energy.PUT("/schedules/:id", h.updateSchedule)
		energy.DELETE("/schedules/:id", h.deleteSchedule)
		energy.POST("/schedules/:id/toggle", h.toggleSchedule)

		// 监控控制
		energy.POST("/monitor/start", h.startMonitor)
		energy.POST("/monitor/stop", h.stopMonitor)
		energy.GET("/monitor/status", h.getMonitorStatus)

		// 配置
		energy.GET("/config", h.getConfig)
		energy.PUT("/config", h.updateConfig)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getSummary 获取仪表盘总览.
func (h *Handlers) getSummary(c *gin.Context) {
	summary := h.manager.GetDashboardSummary()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    summary,
	})
}

// getDashboard 获取仪表盘（同 summary）.
func (h *Handlers) getDashboard(c *gin.Context) {
	summary := h.manager.GetDashboardSummary()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    summary,
	})
}

// getLatestPower 获取最新功耗快照.
func (h *Handlers) getLatestPower(c *gin.Context) {
	snapshot := h.manager.GetLatestSnapshot()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    snapshot,
	})
}

// getPowerHistory 获取功耗历史.
func (h *Handlers) getPowerHistory(c *gin.Context) {
	sinceStr := c.DefaultQuery("since", "")
	limitStr := c.DefaultQuery("limit", "100")

	var since time.Time
	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{
				Code:    1,
				Message: "invalid 'since' parameter, use RFC3339 format",
			})
			return
		}
	} else {
		since = time.Now().Add(-24 * time.Hour)
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	snapshots := h.manager.GetSnapshots(since, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    snapshots,
	})
}

// recordPowerReading 记录功耗读数.
func (h *Handlers) recordPowerReading(c *gin.Context) {
	var reading PowerReading
	if err := c.ShouldBindJSON(&reading); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.RecordPowerReading(&reading)
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "reading recorded",
	})
}

// listRates 列出电价配置.
func (h *Handlers) listRates(c *gin.Context) {
	rates := h.manager.ListRates()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rates,
	})
}

// createRate 创建电价配置.
func (h *Handlers) createRate(c *gin.Context) {
	var rate ElectricityRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateRate(&rate)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "rate created",
		Data:    result,
	})
}

// getRate 获取电价配置.
func (h *Handlers) getRate(c *gin.Context) {
	id := c.Param("id")
	rate, err := h.manager.GetRate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rate,
	})
}

// updateRate 更新电价配置.
func (h *Handlers) updateRate(c *gin.Context) {
	id := c.Param("id")
	var rate ElectricityRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.UpdateRate(id, &rate)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rate updated",
		Data:    result,
	})
}

// deleteRate 删除电价配置.
func (h *Handlers) deleteRate(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRate(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rate deleted",
	})
}

// calculateCost 计算能耗费用.
func (h *Handlers) calculateCost(c *gin.Context) {
	period := EnergyReportPeriod(c.DefaultQuery("period", "daily"))
	rateID := c.DefaultQuery("rate_id", "rate-cn-default")

	cost, err := h.manager.CalculateEnergyCost(c.Request.Context(), period, rateID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cost,
	})
}

// getEfficiencyScore 获取能效评分.
func (h *Handlers) getEfficiencyScore(c *gin.Context) {
	score := h.manager.CalculateEfficiencyScore()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    score,
	})
}

// estimateCarbon 碳排放估算.
func (h *Handlers) estimateCarbon(c *gin.Context) {
	period := EnergyReportPeriod(c.DefaultQuery("period", "daily"))

	estimate, err := h.manager.EstimateCarbon(c.Request.Context(), period)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    estimate,
	})
}

// listReports 列出能耗报表.
func (h *Handlers) listReports(c *gin.Context) {
	period := EnergyReportPeriod(c.DefaultQuery("period", ""))
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	reports := h.manager.ListReports(period, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    reports,
	})
}

// generateReport 生成能耗报表.
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Period EnergyReportPeriod `json:"period" binding:"required"`
		RateID string             `json:"rate_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.RateID == "" {
		req.RateID = "rate-cn-default"
	}

	report, err := h.manager.GenerateReport(c.Request.Context(), req.Period, req.RateID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "report generated",
		Data:    report,
	})
}

// getReport 获取能耗报表.
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// listSchedules 列出休眠计划.
func (h *Handlers) listSchedules(c *gin.Context) {
	schedules := h.manager.ListSchedules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    schedules,
	})
}

// createSchedule 创建休眠计划.
func (h *Handlers) createSchedule(c *gin.Context) {
	var sched SleepSchedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateSchedule(&sched)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "schedule created",
		Data:    result,
	})
}

// getSchedule 获取休眠计划.
func (h *Handlers) getSchedule(c *gin.Context) {
	id := c.Param("id")
	sched, err := h.manager.GetSchedule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sched,
	})
}

// updateSchedule 更新休眠计划.
func (h *Handlers) updateSchedule(c *gin.Context) {
	id := c.Param("id")
	var sched SleepSchedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.UpdateSchedule(id, &sched)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "schedule updated",
		Data:    result,
	})
}

// deleteSchedule 删除休眠计划.
func (h *Handlers) deleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "schedule deleted",
	})
}

// toggleSchedule 切换休眠计划状态.
func (h *Handlers) toggleSchedule(c *gin.Context) {
	id := c.Param("id")
	sched, err := h.manager.ToggleSchedule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "schedule toggled",
		Data:    sched,
	})
}

// startMonitor 启动监控.
func (h *Handlers) startMonitor(c *gin.Context) {
	if err := h.manager.Start(c.Request.Context()); err != nil {
		c.JSON(http.StatusConflict, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "monitor started",
	})
}

// stopMonitor 停止监控.
func (h *Handlers) stopMonitor(c *gin.Context) {
	h.manager.Stop()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "monitor stopped",
	})
}

// getMonitorStatus 获取监控状态.
func (h *Handlers) getMonitorStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"running":  h.manager.IsRunning(),
			"interval": h.manager.config.MonitorInterval,
		},
	})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg DashboardConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}
