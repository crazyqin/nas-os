package drdrill

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 容灾演练 HTTP 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager, logger *zap.Logger) *Handlers {
	return &Handlers{manager: mgr, logger: logger}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	dr := api.Group("/dr-drill")
	{
		// 演练计划
		dr.GET("/plans", h.listPlans)
		dr.POST("/plans", h.createPlan)
		dr.GET("/plans/:id", h.getPlan)
		dr.POST("/plans/:id/execute", h.executePlan)

		// 演练执行
		dr.GET("/executions", h.listExecutions)
		dr.GET("/executions/:id", h.getExecution)
		dr.GET("/executions/:id/report", h.getReport)

		// 指标
		dr.GET("/metrics", h.getMetrics)
	}
}

// listPlans GET /api/v1/dr-drill/plans.
func (h *Handlers) listPlans(c *gin.Context) {
	plans := h.manager.ListPlans()
	c.JSON(http.StatusOK, successResp(plans))
}

// createPlan POST /api/v1/dr-drill/plans.
func (h *Handlers) createPlan(c *gin.Context) {
	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, err.Error()))
		return
	}

	plan, err := h.manager.CreatePlan(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, successResp(plan))
}

// getPlan GET /api/v1/dr-drill/plans/:id.
func (h *Handlers) getPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetPlan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, errResp(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, successResp(plan))
}

// executePlan POST /api/v1/dr-drill/plans/:id/execute.
func (h *Handlers) executePlan(c *gin.Context) {
	id := c.Param("id")
	exec, err := h.manager.ExecutePlan(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errResp(404, err.Error()))
		return
	}
	c.JSON(http.StatusAccepted, successResp(exec))
}

// listExecutions GET /api/v1/dr-drill/executions.
func (h *Handlers) listExecutions(c *gin.Context) {
	execs := h.manager.ListExecutions()
	c.JSON(http.StatusOK, successResp(execs))
}

// getExecution GET /api/v1/dr-drill/executions/:id.
func (h *Handlers) getExecution(c *gin.Context) {
	id := c.Param("id")
	exec, err := h.manager.GetExecution(id)
	if err != nil {
		c.JSON(http.StatusNotFound, errResp(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, successResp(exec))
}

// getReport GET /api/v1/dr-drill/executions/:id/report.
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, errResp(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, successResp(report))
}

// getMetrics GET /api/v1/dr-drill/metrics.
func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()
	c.JSON(http.StatusOK, successResp(metrics))
}
