// Package teamanalytics 提供 REST API 处理器
package teamanalytics

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 团队效能分析 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	analytics := r.Group("/teamanalytics")
	{
		// DORA 指标
		analytics.POST("/metrics", h.calculateMetrics)
		analytics.GET("/metrics/:team_id", h.getLatestMetrics)

		// 趋势分析
		analytics.POST("/trends", h.getTrends)

		// 团队表现
		analytics.GET("/performance/:team_id", h.getTeamPerformance)

		// 报告
		analytics.POST("/reports", h.generateReport)

		// 目标管理
		analytics.GET("/goals", h.listGoals)
		analytics.POST("/goals", h.setGoal)
		analytics.PUT("/goals/:id/progress", h.updateGoalProgress)
		analytics.DELETE("/goals/:id", h.deleteGoal)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// calculateMetrics 计算 DORA 指标
func (h *Handlers) calculateMetrics(c *gin.Context) {
	var req GetMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	metrics, err := h.manager.CalculateMetrics(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    metrics,
	})
}

// getLatestMetrics 获取最新指标
func (h *Handlers) getLatestMetrics(c *gin.Context) {
	teamID := c.Param("team_id")

	h.manager.mu.RLock()
	metricsList, ok := h.manager.metrics[teamID]
	h.manager.mu.RUnlock()

	if !ok || len(metricsList) == 0 {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "no metrics found for team: " + teamID,
		})
		return
	}

	latest := metricsList[len(metricsList)-1]
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    latest,
	})
}

// getTrends 获取趋势数据
func (h *Handlers) getTrends(c *gin.Context) {
	var req struct {
		TeamID    string       `json:"team_id" binding:"required"`
		Metric    string       `json:"metric" binding:"required"`
		Period    MetricPeriod `json:"period"`
		StartDate time.Time    `json:"start_date"`
		EndDate   time.Time    `json:"end_date"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.Period == "" {
		req.Period = PeriodMonthly
	}
	if req.StartDate.IsZero() {
		req.StartDate = time.Now().AddDate(0, -1, 0)
	}
	if req.EndDate.IsZero() {
		req.EndDate = time.Now()
	}

	trends, err := h.manager.GetTrends(req.TeamID, req.Metric, req.Period, req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    trends,
	})
}

// getTeamPerformance 获取团队表现
func (h *Handlers) getTeamPerformance(c *gin.Context) {
	teamID := c.Param("team_id")

	performance, err := h.manager.GetTeamPerformance(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    performance,
	})
}

// generateReport 生成报告
func (h *Handlers) generateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.Period == "" {
		req.Period = PeriodMonthly
	}
	if req.StartDate.IsZero() {
		req.StartDate = time.Now().AddDate(0, -1, 0)
	}
	if req.EndDate.IsZero() {
		req.EndDate = time.Now()
	}

	report, err := h.manager.GenerateReport(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
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

// listGoals 列出目标
func (h *Handlers) listGoals(c *gin.Context) {
	teamID := c.Query("team_id")
	goals := h.manager.GetGoals(teamID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    goals,
	})
}

// setGoal 设置目标
func (h *Handlers) setGoal(c *gin.Context) {
	var req SetGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	goal, err := h.manager.SetGoals(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "goal created",
		Data:    goal,
	})
}

// updateGoalProgress 更新目标进度
func (h *Handlers) updateGoalProgress(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		CurrentValue float64 `json:"current_value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	goal, err := h.manager.UpdateGoalProgress(id, req.CurrentValue)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "goal progress updated",
		Data:    goal,
	})
}

// deleteGoal 删除目标
func (h *Handlers) deleteGoal(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteGoal(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "goal deleted",
	})
}
