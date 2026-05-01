// Package scrubsched 提供智能Scrub调度功能
package scrubsched

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers Scrub调度 HTTP 处理器.
type Handlers struct {
	manager  *Manager
	analyzer *IOAnalyzer
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager:  mgr,
		analyzer: mgr.analyzer,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	scrubsched := api.Group("/scrubsched")
	{
		// 策略管理
		scrubsched.POST("/policies", h.createPolicy)
		scrubsched.GET("/policies", h.listPolicies)
		scrubsched.GET("/policies/:id", h.getPolicy)
		scrubsched.PUT("/policies/:id", h.updatePolicy)
		scrubsched.DELETE("/policies/:id", h.deletePolicy)

		// Scrub控制
		scrubsched.POST("/trigger/:poolId", h.triggerScrub)
		scrubsched.POST("/pause/:poolId", h.pauseScrub)
		scrubsched.POST("/resume/:poolId", h.resumeScrub)
		scrubsched.POST("/cancel/:poolId", h.cancelScrub)

		// 状态查询
		scrubsched.GET("/status", h.getAllStatus)
		scrubsched.GET("/status/:poolId", h.getPoolStatus)

		// 历史记录
		scrubsched.GET("/history", h.getHistory)
		scrubsched.GET("/history/:poolId", h.getPoolHistory)

		// IO负载
		scrubsched.GET("/io-load", h.getCurrentIOLoad)
		scrubsched.GET("/io-load/history", h.getIOHistory)

		// 建议
		scrubsched.GET("/recommendations", h.getRecommendations)
	}
}

// ========== 策略管理 Handlers ==========

// createPolicy 创建调度策略.
func (h *Handlers) createPolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	policy, err := h.manager.CreatePolicy(req)
	if err != nil {
		code := http.StatusInternalServerError
		switch err {
		case ErrPoolNotFound:
			code = http.StatusNotFound
		case ErrPolicyExists:
			code = http.StatusConflict
		case ErrInvalidCronExpr, ErrInvalidThreshold:
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// listPolicies 列出所有策略.
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// getPolicy 获取策略详情.
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// updatePolicy 更新策略.
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")

	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	policy, err := h.manager.UpdatePolicy(id, req)
	if err != nil {
		code := http.StatusInternalServerError
		switch err {
		case ErrPolicyNotFound:
			code = http.StatusNotFound
		case ErrInvalidCronExpr:
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// deletePolicy 删除策略.
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "策略已删除"})
}

// ========== Scrub控制 Handlers ==========

// triggerScrub 手动触发Scrub.
func (h *Handlers) triggerScrub(c *gin.Context) {
	poolID := c.Param("poolId")

	if err := h.manager.TriggerScrub(poolID); err != nil {
		code := http.StatusInternalServerError
		switch err {
		case ErrPoolNotFound:
			code = http.StatusNotFound
		case ErrScrubAlreadyRunning:
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scrub已触发", "pool_id": poolID})
}

// pauseScrub 暂停Scrub.
func (h *Handlers) pauseScrub(c *gin.Context) {
	poolID := c.Param("poolId")

	if err := h.manager.PauseScrub(poolID, "手动暂停"); err != nil {
		code := http.StatusInternalServerError
		if err == ErrScrubNotRunning {
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scrub已暂停", "pool_id": poolID})
}

// resumeScrub 恢复Scrub.
func (h *Handlers) resumeScrub(c *gin.Context) {
	poolID := c.Param("poolId")

	if err := h.manager.ResumeScrub(poolID); err != nil {
		code := http.StatusInternalServerError
		if err == ErrScrubNotRunning {
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scrub已恢复", "pool_id": poolID})
}

// cancelScrub 取消Scrub.
func (h *Handlers) cancelScrub(c *gin.Context) {
	poolID := c.Param("poolId")

	if err := h.manager.CancelScrub(poolID); err != nil {
		code := http.StatusInternalServerError
		if err == ErrScrubNotRunning {
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scrub已取消", "pool_id": poolID})
}

// ========== 状态查询 Handlers ==========

// getAllStatus 获取所有池Scrub状态.
func (h *Handlers) getAllStatus(c *gin.Context) {
	statuses := h.manager.GetScrubStatus()
	c.JSON(http.StatusOK, gin.H{"status": statuses})
}

// getPoolStatus 获取指定池Scrub状态.
func (h *Handlers) getPoolStatus(c *gin.Context) {
	poolID := c.Param("poolId")

	status, err := h.manager.GetPoolScrubStatus(poolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ========== 历史记录 Handlers ==========

// getHistory 获取所有Scrub历史.
func (h *Handlers) getHistory(c *gin.Context) {
	records := h.manager.GetHistory("")
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

// getPoolHistory 获取指定池Scrub历史.
func (h *Handlers) getPoolHistory(c *gin.Context) {
	poolID := c.Param("poolId")

	records := h.manager.GetHistory(poolID)
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

// ========== IO负载 Handlers ==========

// getCurrentIOLoad 获取当前IO负载.
func (h *Handlers) getCurrentIOLoad(c *gin.Context) {
	loads := h.manager.GetCurrentIOLoad()
	c.JSON(http.StatusOK, gin.H{"loads": loads})
}

// getIOHistory 获取IO历史趋势.
func (h *Handlers) getIOHistory(c *gin.Context) {
	poolID := c.Query("pool_id")

	if poolID != "" {
		records := h.manager.GetIOHistory(poolID)
		c.JSON(http.StatusOK, gin.H{
			"pool_id": poolID,
			"records": records,
		})
		return
	}

	// 返回所有池的IO历史
	loads := h.manager.GetCurrentIOLoad()
	allHistory := make(map[string][]*IOLoad)
	for id := range loads {
		allHistory[id] = h.manager.GetIOHistory(id)
	}
	c.JSON(http.StatusOK, gin.H{"history": allHistory})
}

// ========== 建议 Handlers ==========

// getRecommendations 获取Scrub调度建议.
func (h *Handlers) getRecommendations(c *gin.Context) {
	recs := h.manager.GetRecommendations()
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}
