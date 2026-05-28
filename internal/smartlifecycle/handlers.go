// Package smartlifecycle - HTTP API 处理器
package smartlifecycle

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 生命周期管理 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	lc := rg.Group("/lifecycle")
	{
		// 策略管理
		lc.GET("/policies", h.listPolicies)
		lc.POST("/policies", h.createPolicy)
		lc.GET("/policies/:id", h.getPolicy)
		lc.PUT("/policies/:id", h.updatePolicy)
		lc.DELETE("/policies/:id", h.deletePolicy)

		// 扫描与执行
		lc.POST("/scan", h.runScan)
		lc.POST("/policies/:id/execute", h.executePolicy)

		// 统计
		lc.GET("/stats", h.getStats)
		lc.GET("/scans", h.getScanResults)
		lc.GET("/executions", h.getExecutions)
	}
}

func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

func (h *Handlers) createPolicy(c *gin.Context) {
	var policy LifecyclePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreatePolicy(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, policy)
}

func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) updatePolicy(c *gin.Context) {
	var policy LifecyclePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	policy.ID = c.Param("id")
	if err := h.manager.UpdatePolicy(&policy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (h *Handlers) runScan(c *gin.Context) {
	result := h.manager.RunScan()
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) executePolicy(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.ExecutePolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) getScanResults(c *gin.Context) {
	results := h.manager.GetScanResults()
	c.JSON(http.StatusOK, gin.H{"scans": results})
}

func (h *Handlers) getExecutions(c *gin.Context) {
	executions := h.manager.GetExecutions()
	c.JSON(http.StatusOK, gin.H{"executions": executions})
}
