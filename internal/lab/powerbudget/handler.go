// Package powerbudget 提供 REST API 处理器
package powerbudget

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 功率预算 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	power := r.Group("/power")
	{
		power.GET("/readings", h.getReadings)
		power.POST("/budget", h.setBudget)
		power.GET("/budget", h.listBudgets)
		power.GET("/budget/:id", h.getBudget)
		power.DELETE("/budget/:id", h.deleteBudget)
		power.POST("/cost", h.calculateCost)
		power.POST("/savings", h.createSavingsPlan)
		power.GET("/savings", h.listPlans)
		power.GET("/savings/:id", h.getPlan)
		power.DELETE("/savings/:id", h.deletePlan)
		power.GET("/alerts", h.getAlerts)
		power.GET("/config", h.getConfig)
		power.PUT("/config", h.updateConfig)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getReadings 获取功率读数.
func (h *Handlers) getReadings(c *gin.Context) {
	deviceID := c.Query("device_id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	readings, err := h.manager.GetReadings(deviceID, limit)
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
		Data:    readings,
	})
}

// setBudget 设置功率预算.
func (h *Handlers) setBudget(c *gin.Context) {
	var budget PowerBudget
	if err := c.ShouldBindJSON(&budget); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.SetBudget(&budget)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "budget set",
		Data:    result,
	})
}

// listBudgets 列出预算.
func (h *Handlers) listBudgets(c *gin.Context) {
	budgets := h.manager.ListBudgets()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    budgets,
	})
}

// getBudget 获取预算.
func (h *Handlers) getBudget(c *gin.Context) {
	id := c.Param("id")
	budget, err := h.manager.GetBudget(id)
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
		Data:    budget,
	})
}

// deleteBudget 删除预算.
func (h *Handlers) deleteBudget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteBudget(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "budget deleted",
	})
}

// calculateCost 计算能源成本.
func (h *Handlers) calculateCost(c *gin.Context) {
	var req struct {
		PeriodStart string `json:"period_start" binding:"required"`
		PeriodEnd   string `json:"period_end" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid period_start format, use YYYY-MM-DD",
		})
		return
	}

	periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid period_end format, use YYYY-MM-DD",
		})
		return
	}

	energyCost, err := h.manager.CalculateCost(periodStart, periodEnd)
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
		Data:    energyCost,
	})
}

// createSavingsPlan 创建节能计划.
func (h *Handlers) createSavingsPlan(c *gin.Context) {
	var plan SavingsPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateSavingsPlan(&plan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "savings plan created",
		Data:    result,
	})
}

// listPlans 列出节能计划.
func (h *Handlers) listPlans(c *gin.Context) {
	plans := h.manager.ListPlans()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    plans,
	})
}

// getPlan 获取节能计划.
func (h *Handlers) getPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetPlan(id)
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
		Data:    plan,
	})
}

// deletePlan 删除节能计划.
func (h *Handlers) deletePlan(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePlan(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "plan deleted",
	})
}

// getAlerts 获取告警.
func (h *Handlers) getAlerts(c *gin.Context) {
	level := AlertLevel(c.Query("level"))
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	alerts, err := h.manager.GetAlerts(level, limit)
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
		Data:    alerts,
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
	var cfg PowerBudgetConfig
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
