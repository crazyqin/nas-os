// Package costgovernance 提供多云成本治理功能
package costgovernance

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 成本治理 HTTP 处理器.
type Handlers struct {
	manager  *Manager
	analyzer *Analyzer
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager, analyzer *Analyzer) *Handlers {
	return &Handlers{manager: manager, analyzer: analyzer}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	costGroup := api.Group("/costgovernance")
	{
		// 策略管理
		costGroup.POST("/policies", h.createPolicy)
		costGroup.GET("/policies", h.listPolicies)
		costGroup.GET("/policies/:id", h.getPolicy)
		costGroup.DELETE("/policies/:id", h.deletePolicy)

		// 预算管理
		costGroup.POST("/budgets", h.createBudget)
		costGroup.GET("/budgets", h.listBudgets)
		costGroup.GET("/budgets/:id", h.getBudget)
		costGroup.PUT("/budgets/:id/spent", h.updateBudgetSpent)

		// 告警管理
		costGroup.GET("/alerts", h.listAlerts)
		costGroup.PUT("/alerts/:id/acknowledge", h.acknowledgeAlert)

		// 资源使用
		costGroup.POST("/usages", h.updateResourceUsage)
		costGroup.GET("/usages", h.listResourceUsages)

		// 报表
		costGroup.POST("/reports", h.generateReport)
		costGroup.GET("/summary", h.getCostSummary)

		// 分析
		costGroup.GET("/analysis/utilization", h.getResourceUtilization)
		costGroup.GET("/analysis/capacity-risk", h.getCapacityRisk)
		costGroup.GET("/analysis/suggestions", h.getOptimizationSuggestions)
		costGroup.GET("/analysis/anomalies", h.detectAnomalies)
		costGroup.POST("/analysis/predict", h.predictCost)
	}
}

// createPolicy 创建成本策略.
func (h *Handlers) createPolicy(c *gin.Context) {
	var policy CostPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.CreatePolicy(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, policy)
}

// listPolicies 列出所有策略.
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies, "total": len(policies)})
}

// getPolicy 获取策略详情.
func (h *Handlers) getPolicy(c *gin.Context) {
	policy, err := h.manager.GetPolicy(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// deletePolicy 删除策略.
func (h *Handlers) deletePolicy(c *gin.Context) {
	if err := h.manager.DeletePolicy(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "策略已删除"})
}

// createBudget 创建预算.
func (h *Handlers) createBudget(c *gin.Context) {
	var budget Budget
	if err := c.ShouldBindJSON(&budget); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.CreateBudget(&budget); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, budget)
}

// listBudgets 列出所有预算.
func (h *Handlers) listBudgets(c *gin.Context) {
	budgets := h.manager.ListBudgets()
	c.JSON(http.StatusOK, gin.H{"budgets": budgets, "total": len(budgets)})
}

// getBudget 获取预算详情.
func (h *Handlers) getBudget(c *gin.Context) {
	budget, err := h.manager.GetBudget(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budget)
}

// updateBudgetSpent 更新预算已花费.
func (h *Handlers) updateBudgetSpent(c *gin.Context) {
	var req struct {
		Spent float64 `json:"spent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.UpdateBudgetSpent(c.Param("id"), req.Spent); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "预算已更新"})
}

// listAlerts 列出告警.
func (h *Handlers) listAlerts(c *gin.Context) {
	provider := CloudProvider(c.Query("provider"))
	alerts := h.manager.ListAlerts(provider, false)
	c.JSON(http.StatusOK, gin.H{"alerts": alerts, "total": len(alerts)})
}

// acknowledgeAlert 确认告警.
func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	if err := h.manager.AcknowledgeAlert(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "告警已确认"})
}

// updateResourceUsage 更新资源使用.
func (h *Handlers) updateResourceUsage(c *gin.Context) {
	var usage ResourceUsage
	if err := c.ShouldBindJSON(&usage); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.UpdateResourceUsage(&usage); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, usage)
}

// listResourceUsages 列出资源使用.
func (h *Handlers) listResourceUsages(c *gin.Context) {
	provider := CloudProvider(c.Query("provider"))
	usages := h.manager.ListResourceUsages(provider)
	c.JSON(http.StatusOK, gin.H{"usages": usages, "total": len(usages)})
}

// generateReport 生成报表.
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Provider CloudProvider `json:"provider"`
		Start    time.Time     `json:"start"`
		End      time.Time     `json:"end"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	report := h.manager.GenerateReport(req.Provider, req.Start, req.End)
	c.JSON(http.StatusOK, report)
}

// getCostSummary 获取成本汇总.
func (h *Handlers) getCostSummary(c *gin.Context) {
	provider := CloudProvider(c.Query("provider"))
	summary := h.manager.GetCostSummary(provider)
	c.JSON(http.StatusOK, summary)
}

// getResourceUtilization 获取资源利用率.
func (h *Handlers) getResourceUtilization(c *gin.Context) {
	provider := CloudProvider(c.Query("provider"))
	result := h.analyzer.AnalyzeResourceUtilization(provider)
	c.JSON(http.StatusOK, result)
}

// getCapacityRisk 获取容量与成本风险摘要.
func (h *Handlers) getCapacityRisk(c *gin.Context) {
	provider := CloudProvider(c.Query("provider"))
	summary := h.analyzer.AnalyzeCapacityRisk(provider)
	c.JSON(http.StatusOK, summary)
}

// getOptimizationSuggestions 获取优化建议.
func (h *Handlers) getOptimizationSuggestions(c *gin.Context) {
	provider := CloudProvider(c.Query("provider"))
	suggestions := h.analyzer.GenerateOptimizationSuggestions(provider)
	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions, "total": len(suggestions)})
}

// detectAnomalies 异常检测.
func (h *Handlers) detectAnomalies(c *gin.Context) {
	provider := c.Query("provider")
	anomalies := h.analyzer.DetectAnomalies(provider, 95.0)
	c.JSON(http.StatusOK, gin.H{"anomalies": anomalies, "total": len(anomalies)})
}

// predictCost 成本预测.
func (h *Handlers) predictCost(c *gin.Context) {
	var req struct {
		Provider   string `json:"provider"`
		FutureDays int    `json:"future_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if req.FutureDays <= 0 {
		req.FutureDays = 30
	}
	predictions, err := h.analyzer.PredictCost(req.Provider, req.FutureDays)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"predictions": predictions, "total": len(predictions)})
}
