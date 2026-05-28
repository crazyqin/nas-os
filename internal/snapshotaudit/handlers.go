// Package snapshotaudit - HTTP API 处理器
package snapshotaudit

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 快照审计 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	audit := rg.Group("/snapshot-audit")
	{
		// 快照管理
		audit.GET("/snapshots", h.listSnapshots)
		audit.POST("/snapshots", h.registerSnapshot)

		// 策略管理
		audit.GET("/policies", h.listPolicies)
		audit.POST("/policies", h.createPolicy)

		// 审计执行
		audit.POST("/run/:policyId", h.runAudit)

		// 报告与日志
		audit.GET("/reports", h.getReports)
		audit.GET("/logs", h.getLogs)
		audit.GET("/stats", h.getStats)
	}
}

func (h *Handlers) listSnapshots(c *gin.Context) {
	snaps := h.manager.ListSnapshots()
	c.JSON(http.StatusOK, gin.H{"snapshots": snaps})
}

func (h *Handlers) registerSnapshot(c *gin.Context) {
	var snap SnapshotRecord
	if err := c.ShouldBindJSON(&snap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.RegisterSnapshot(&snap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, snap)
}

func (h *Handlers) listPolicies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"policies": h.manager.policies})
}

func (h *Handlers) createPolicy(c *gin.Context) {
	var policy AuditPolicy
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

func (h *Handlers) runAudit(c *gin.Context) {
	policyID := c.Param("policyId")
	report, err := h.manager.RunAudit(policyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) getReports(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"reports": h.manager.GetReports()})
}

func (h *Handlers) getLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": h.manager.GetLogs()})
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetStats())
}
