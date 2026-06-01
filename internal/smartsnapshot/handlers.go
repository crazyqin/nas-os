package smartsnapshot

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能快照 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	ss := api.Group("/smartsnapshot")
	{
		// 快照管理
		ss.POST("", h.createSnapshot)
		ss.GET("", h.listSnapshots)
		ss.GET("/:id", h.getSnapshot)
		ss.GET("/:id/chain", h.getSnapshotChain)
		ss.DELETE("/:id", h.deleteSnapshot)

		// 快照回滚与恢复
		ss.POST("/rollback", h.rollbackSnapshot)
		ss.POST("/:id/restore", h.restoreSnapshot)

		// 快照克隆
		ss.POST("/:id/clone", h.cloneSnapshot)
		ss.GET("/clones", h.listClones)
		ss.DELETE("/clones/:id", h.destroyClone)

		// 策略管理
		ss.POST("/policies", h.createPolicy)
		ss.GET("/policies", h.listPolicies)
		ss.GET("/policies/:id", h.getPolicy)
		ss.PUT("/policies/:id", h.updatePolicy)
		ss.DELETE("/policies/:id", h.deletePolicy)
		ss.POST("/policies/:id/enable", h.enablePolicy)
		ss.POST("/policies/:id/disable", h.disablePolicy)
		ss.POST("/policies/:id/run", h.runPolicy)

		// 清理与统计
		ss.POST("/cleanup", h.cleanupExpired)
		ss.GET("/stats", h.getStats)
	}
}

// ========== 快照 API ==========

func (h *Handlers) createSnapshot(c *gin.Context) {
	var req CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	snap, err := h.manager.CreateSnapshot(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": snap})
}

func (h *Handlers) listSnapshots(c *gin.Context) {
	datasetPath := c.Query("datasetPath")
	snaps := h.manager.ListSnapshots(datasetPath)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": snaps})
}

func (h *Handlers) getSnapshot(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.manager.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": snap})
}

func (h *Handlers) getSnapshotChain(c *gin.Context) {
	id := c.Param("id")
	chain, err := h.manager.GetSnapshotChain(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": chain})
}

func (h *Handlers) deleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.DeleteSnapshot(id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// ========== 回滚与恢复 ==========

func (h *Handlers) rollbackSnapshot(c *gin.Context) {
	var req RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.RollbackSnapshot(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *Handlers) restoreSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		TargetPath string `json:"targetPath" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.RestoreSnapshot(id, req.TargetPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// ========== 克隆 ==========

func (h *Handlers) cloneSnapshot(c *gin.Context) {
	snapshotID := c.Param("id")
	var req struct {
		ClonePath string `json:"clonePath" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	clone, err := h.manager.CloneSnapshot(snapshotID, req.ClonePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": clone})
}

func (h *Handlers) listClones(c *gin.Context) {
	snapshotID := c.Query("snapshotId")
	clones := h.manager.ListClones(snapshotID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": clones})
}

func (h *Handlers) destroyClone(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DestroyClone(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// ========== 策略 API ==========

func (h *Handlers) createPolicy(c *gin.Context) {
	var policy SnapshotPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.CreatePolicy(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": policy})
}

func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": policies})
}

func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": policy})
}

func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy SnapshotPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.UpdatePolicy(id, &policy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *Handlers) enablePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.EnablePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *Handlers) disablePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisablePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *Handlers) runPolicy(c *gin.Context) {
	id := c.Param("id")
	snaps, err := h.manager.CreateSnapshotWithPolicy(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": snaps})
}

// ========== 清理与统计 ==========

func (h *Handlers) cleanupExpired(c *gin.Context) {
	deleted, err := h.manager.CleanupExpiredSnapshots()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"deleted": deleted}})
}

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats})
}
