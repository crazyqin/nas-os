// Package datatiering HTTP API 处理器
package datatiering

import (
	"net/http"
	"time"

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
	group := router.Group("/data-tiering")
	{
		group.GET("/stats", h.GetStats)
		group.GET("/report", h.GetReport)
		group.GET("/policies", h.ListPolicies)
		group.POST("/policies", h.CreatePolicy)
		group.PUT("/policies/:id", h.UpdatePolicy)
		group.DELETE("/policies/:id", h.DeletePolicy)
		group.POST("/migrate", h.RunMigration)
		group.GET("/jobs", h.ListJobs)
		group.GET("/jobs/:id", h.GetJob)
	}
}

func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetTierStats()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *Handler) GetReport(c *gin.Context) {
	report := h.manager.GetReport()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": policies})
}

func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy TierPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if policy.ID == "" {
		policy.ID = "policy_" + time.Now().Format("20060102150405")
	}
	if err := h.manager.AddPolicy(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": policy})
}

func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy TierPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	policy.ID = id
	if err := h.manager.UpdatePolicy(&policy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": policy})
}

func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeletePolicy(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "策略不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) RunMigration(c *gin.Context) {
	var req struct {
		PolicyID string `json:"policy_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	job, err := h.manager.AnalyzeAndMigrate(req.PolicyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": job})
}

func (h *Handler) ListJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": jobs})
}

func (h *Handler) GetJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.manager.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": job})
}
