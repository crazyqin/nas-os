package smartrebalance

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 再平衡 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	rebalance := rg.Group("/rebalance")
	{
		// 存储池管理
		rebalance.GET("/pools", h.listPools)
		rebalance.GET("/pools/:id", h.getPool)
		rebalance.GET("/pools/:id/analyze", h.analyzePool)

		// 再平衡任务
		rebalance.POST("/trigger", h.triggerRebalance)
		rebalance.GET("/jobs", h.listJobs)
		rebalance.GET("/jobs/:id", h.getJob)

		// 策略管理
		rebalance.GET("/policies", h.listPolicies)
		rebalance.POST("/policies", h.createPolicy)
		rebalance.GET("/policies/:id", h.getPolicy)

		// 指标
		rebalance.GET("/metrics", h.getMetrics)
	}
}

// listPools 列出存储池
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	c.JSON(http.StatusOK, gin.H{"pools": pools, "total": len(pools)})
}

// getPool 获取存储池
func (h *Handlers) getPool(c *gin.Context) {
	id := c.Param("id")
	pool, err := h.manager.GetPool(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// analyzePool 分析存储池
func (h *Handlers) analyzePool(c *gin.Context) {
	id := c.Param("id")
	analysis, err := h.manager.AnalyzePool(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analysis)
}

// triggerRebalanceRequest 触发再平衡请求
type triggerRebalanceRequest struct {
	PoolID   string            `json:"pool_id" binding:"required"`
	Strategy RebalanceStrategy `json:"strategy"`
}

// triggerRebalance 触发再平衡
func (h *Handlers) triggerRebalance(c *gin.Context) {
	var req triggerRebalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Strategy == "" {
		req.Strategy = StrategyHybrid
	}

	job, err := h.manager.TriggerRebalance(req.PoolID, req.Strategy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

// listJobs 列出任务
func (h *Handlers) listJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": len(jobs)})
}

// getJob 获取任务
func (h *Handlers) getJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.manager.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// listPolicies 列出策略
func (h *Handlers) listPolicies(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()
	policies := make([]*RebalancePolicy, 0, len(h.manager.policies))
	for _, p := range h.manager.policies {
		policies = append(policies, p)
	}
	c.JSON(http.StatusOK, gin.H{"policies": policies, "total": len(policies)})
}

// createPolicy 创建策略
func (h *Handlers) createPolicy(c *gin.Context) {
	var policy RebalancePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CreatePolicy(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// getPolicy 获取策略
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// getMetrics 获取指标
func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.manager.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// parseIntParam 解析整数参数
func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	val := c.Query(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
