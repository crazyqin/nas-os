// Package smartbudget 提供智能预算管理 HTTP handlers
package smartbudget

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	budgets := r.Group("/smartbudget")
	{
		// 预算计划
		budgets.GET("/plans", h.ListPlans)
		budgets.POST("/plans", h.CreatePlan)
		budgets.GET("/plans/:id", h.GetPlan)
		budgets.POST("/plans/:id/usage", h.UpdatePlanUsage)

		// 成本明细
		budgets.GET("/costs", h.GetCosts)
		budgets.POST("/costs", h.AddCost)

		// 优化建议
		budgets.GET("/optimization", h.GetOptimizations)
		budgets.POST("/optimization/generate", h.GenerateOptimizations)

		// 趋势分析
		budgets.GET("/trends", h.GetTrends)
		budgets.GET("/forecast", h.GetForecast)

		// 告警
		budgets.GET("/alerts", h.GetAlerts)

		// 月度报告
		budgets.GET("/reports/:month", h.GetMonthlyReport)
	}
}

// ListPlans 获取预算计划列表
// @Summary 获取预算计划列表
// @Tags smartbudget
// @Produce json
// @Success 200 {array} BudgetPlan
// @Router /smartbudget/plans [get].
func (h *Handlers) ListPlans(c *gin.Context) {
	plans := h.manager.ListPlans()
	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
		"total": len(plans),
	})
}

// CreatePlan 创建预算计划
// @Summary 创建预算计划
// @Tags smartbudget
// @Accept json
// @Produce json
// @Param input body CreatePlanRequest true "预算计划信息"
// @Success 201 {object} BudgetPlan
// @Failure 400 {object} ErrorResponse
// @Router /smartbudget/plans [post].
func (h *Handlers) CreatePlan(c *gin.Context) {
	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   err.Error(),
			Code:    "INVALID_INPUT",
			Details: "请检查必填字段",
		})
		return
	}

	plan, err := h.manager.CreatePlan(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
			Code:  "CREATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, plan)
}

// GetPlan 获取预算计划详情
// @Summary 获取预算计划详情
// @Tags smartbudget
// @Produce json
// @Param id path string true "计划ID"
// @Success 200 {object} BudgetPlan
// @Failure 404 {object} ErrorResponse
// @Router /smartbudget/plans/{id} [get].
func (h *Handlers) GetPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetPlan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: err.Error(),
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// UpdatePlanUsage 更新预算使用量
// @Summary 更新预算使用量
// @Tags smartbudget
// @Accept json
// @Produce json
// @Param id path string true "计划ID"
// @Param input body object true "使用量信息"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /smartbudget/plans/{id}/usage [post].
func (h *Handlers) UpdatePlanUsage(c *gin.Context) {
	id := c.Param("id")

	var input struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   err.Error(),
			Code:    "INVALID_INPUT",
			Details: "金额必须大于0",
		})
		return
	}

	if err := h.manager.UpdatePlanUsage(id, input.Amount); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
			Code:  "UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "使用量更新成功",
	})
}

// GetCosts 获取成本明细
// @Summary 获取成本明细
// @Tags smartbudget
// @Produce json
// @Param department query string false "部门"
// @Param project query string false "项目"
// @Param provider query string false "云提供商"
// @Param category query string false "类别"
// @Param start_date query string false "开始日期"
// @Param end_date query string false "结束日期"
// @Success 200 {array} CostBreakdown
// @Router /smartbudget/costs [get].
func (h *Handlers) GetCosts(c *gin.Context) {
	var query CostQueryRequest
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
			Code:  "INVALID_QUERY",
		})
		return
	}

	costs := h.manager.GetCostBreakdowns(query)
	c.JSON(http.StatusOK, gin.H{
		"costs": costs,
		"total": len(costs),
	})
}

// AddCost 添加成本记录
// @Summary 添加成本记录
// @Tags smartbudget
// @Accept json
// @Produce json
// @Param input body object true "成本信息"
// @Success 201 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /smartbudget/costs [post].
func (h *Handlers) AddCost(c *gin.Context) {
	var input struct {
		Department string  `json:"department" binding:"required"`
		Category   string  `json:"category" binding:"required"`
		Amount     float64 `json:"amount" binding:"required,gt=0"`
		Provider   string  `json:"provider,omitempty"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   err.Error(),
			Code:    "INVALID_INPUT",
			Details: "请检查必填字段",
		})
		return
	}

	trend := TrendStable
	if input.Amount > 1000 {
		trend = TrendUp
	}

	h.manager.AddCostBreakdown(input.Department, CostBreakdown{
		Category:   input.Category,
		Amount:     input.Amount,
		Percentage: 0,
		Trend:      trend,
		Provider:   input.Provider,
	})

	c.JSON(http.StatusCreated, SuccessResponse{
		Message: "成本记录添加成功",
	})
}

// GetOptimizations 获取优化建议
// @Summary 获取优化建议
// @Tags smartbudget
// @Produce json
// @Success 200 {array} CostOptimization
// @Router /smartbudget/optimization [get].
func (h *Handlers) GetOptimizations(c *gin.Context) {
	opts := h.manager.GetOptimizations()
	c.JSON(http.StatusOK, gin.H{
		"optimizations": opts,
		"total":         len(opts),
	})
}

// GenerateOptimizations 生成优化建议
// @Summary 生成优化建议
// @Tags smartbudget
// @Accept json
// @Produce json
// @Param input body object true "部门信息"
// @Success 201 {array} CostOptimization
// @Failure 400 {object} ErrorResponse
// @Router /smartbudget/optimization/generate [post].
func (h *Handlers) GenerateOptimizations(c *gin.Context) {
	var input struct {
		Department string `json:"department" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   err.Error(),
			Code:    "INVALID_INPUT",
			Details: "请提供部门名称",
		})
		return
	}

	opts := h.manager.GenerateOptimizationSuggestions(input.Department)
	c.JSON(http.StatusCreated, gin.H{
		"optimizations": opts,
		"total":         len(opts),
	})
}

// GetTrends 获取成本趋势
// @Summary 获取成本趋势
// @Tags smartbudget
// @Produce json
// @Param department query string false "部门"
// @Param category query string false "类别"
// @Param months query int false "月数"
// @Success 200 {array} CostTrend
// @Router /smartbudget/trends [get].
func (h *Handlers) GetTrends(c *gin.Context) {
	var query TrendQueryRequest
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
			Code:  "INVALID_QUERY",
		})
		return
	}

	trends := h.manager.GetCostTrends(query)
	c.JSON(http.StatusOK, gin.H{
		"trends": trends,
		"total":  len(trends),
	})
}

// GetForecast 获取成本预测
// @Summary 获取成本预测
// @Tags smartbudget
// @Produce json
// @Param months query int false "预测月数"
// @Success 200 {array} CostForecast
// @Router /smartbudget/forecast [get].
func (h *Handlers) GetForecast(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "3")
	months, err := strconv.Atoi(monthsStr)
	if err != nil || months <= 0 {
		months = 3
	}

	forecasts := h.manager.ForecastCost(months)
	c.JSON(http.StatusOK, gin.H{
		"forecasts": forecasts,
		"total":     len(forecasts),
	})
}

// GetAlerts 获取告警列表
// @Summary 获取告警列表
// @Tags smartbudget
// @Produce json
// @Success 200 {array} BudgetAlert
// @Router /smartbudget/alerts [get].
func (h *Handlers) GetAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// GetMonthlyReport 获取月度报告
// @Summary 获取月度报告
// @Tags smartbudget
// @Produce json
// @Param month path string true "月份 (YYYY-MM)"
// @Success 200 {object} MonthlyReport
// @Failure 400 {object} ErrorResponse
// @Router /smartbudget/reports/{month} [get].
func (h *Handlers) GetMonthlyReport(c *gin.Context) {
	month := c.Param("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	report := h.manager.GenerateMonthlyReport(month)
	c.JSON(http.StatusOK, report)
}
