package featureroadmap

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 功能路线图HTTP处理器.
type Handler struct {
	rm *FeatureRoadmap
}

// NewHandler 创建处理器.
func NewHandler(rm *FeatureRoadmap) *Handler {
	return &Handler{rm: rm}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/featureroadmap")
	{
		group.GET("/features", h.GetFeatures)
		group.GET("/milestones", h.GetMilestones)
		group.GET("/stats", h.GetStats)
		group.POST("/feature", h.AddFeature)
		group.PUT("/feature/:id", h.UpdateFeature)
		group.POST("/milestone", h.AddMilestone)
	}
}

// GetFeatures 获取功能列表.
func (h *Handler) GetFeatures(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.rm.GetFeatures()})
}

// GetMilestones 获取里程碑.
func (h *Handler) GetMilestones(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.rm.GetMilestones()})
}

// GetStats 获取统计.
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.rm.GetStats()})
}

// AddFeature 添加功能.
func (h *Handler) AddFeature(c *gin.Context) {
	var feature Feature
	if err := c.ShouldBindJSON(&feature); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := h.rm.AddFeature(feature)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": result})
}

// UpdateFeature 更新功能.
func (h *Handler) UpdateFeature(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.rm.UpdateFeature(id, updates) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "feature not found"})
	}
}

// AddMilestone 添加里程碑.
func (h *Handler) AddMilestone(c *gin.Context) {
	var ms Milestone
	if err := c.ShouldBindJSON(&ms); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := h.rm.AddMilestone(ms)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": result})
}
