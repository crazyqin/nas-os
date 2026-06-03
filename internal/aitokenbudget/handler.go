package aitokenbudget

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/ai-token-budget")
	{
		// 预算管理
		group.POST("/budgets", h.CreateBudget)
		group.GET("/budgets", h.ListBudgets)
		group.GET("/budgets/:id/status", h.GetBudgetStatus)
		group.GET("/budgets/:id/alerts", h.GetBudgetAlerts)

		// 用量记录
		group.POST("/usage", h.RecordUsage)

		// 分析
		group.GET("/analysis/cost", h.GetCostAnalysis)
		group.GET("/analysis/models", h.GetModelComparison)

		// 告警
		group.GET("/alerts", h.GetAlerts)
		group.POST("/alerts/:id/dismiss", h.DismissAlert)
	}
}

func (h *Handler) CreateBudget(c *gin.Context) {
	var budget Budget
	if err := c.ShouldBindJSON(&budget); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.manager.CreateBudget(&budget); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": budget})
}

func (h *Handler) ListBudgets(c *gin.Context) {
	budgets := h.manager.ListBudgets()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": budgets, "total": len(budgets)})
}

func (h *Handler) GetBudgetStatus(c *gin.Context) {
	id := c.Param("id")
	summary, err := h.manager.GetBudgetStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

func (h *Handler) GetBudgetAlerts(c *gin.Context) {
	id := c.Param("id")
	alerts := h.manager.GetAlerts(false)
	result := make([]*BudgetAlert, 0)
	for _, a := range alerts {
		if a.BudgetID == id {
			result = append(result, a)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func (h *Handler) RecordUsage(c *gin.Context) {
	var record UsageRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.manager.RecordUsage(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": record})
}

func (h *Handler) GetCostAnalysis(c *gin.Context) {
	period := c.DefaultQuery("period", "monthly")
	analysis := h.manager.GetCostAnalysis(period)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analysis})
}

func (h *Handler) GetModelComparison(c *gin.Context) {
	tokensStr := c.DefaultQuery("tokens", "1000000")
	tokens, err := strconv.Atoi(tokensStr)
	if err != nil {
		tokens = 1000000
	}
	comparison := h.manager.GetModelComparison(tokens)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": comparison})
}

func (h *Handler) GetAlerts(c *gin.Context) {
	dismissedStr := c.DefaultQuery("dismissed", "false")
	dismissed := dismissedStr == "true"
	alerts := h.manager.GetAlerts(dismissed)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": alerts, "total": len(alerts)})
}

func (h *Handler) DismissAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DismissAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "告警已关闭"})
}
