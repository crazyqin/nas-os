// Package storagesetupwizard 提供HTTP API处理器
package storagesetupwizard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/storage-setup")
	{
		group.POST("/sessions", h.CreateSession)
		group.GET("/sessions", h.ListSessions)
		group.GET("/sessions/:id", h.GetSession)
		group.DELETE("/sessions/:id", h.DeleteSession)
		group.PUT("/sessions/:id/step", h.UpdateStep)
		group.POST("/sessions/:id/pool", h.SetPoolConfig)
		group.POST("/sessions/:id/volume", h.SetVolumeConfig)
		group.POST("/sessions/:id/share", h.SetShareConfig)
		group.POST("/sessions/:id/complete", h.CompleteSession)
		group.GET("/sessions/:id/recommendations", h.GetRecommendations)
		group.GET("/sessions/:id/capacity", h.EstimateCapacity)
	}
}

// CreateSession 创建设置会话
func (h *Handler) CreateSession(c *gin.Context) {
	var req struct {
		Disks []DiskInfo `json:"disks" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.manager.CreateSession(req.Disks)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// ListSessions 列出会话
func (h *Handler) ListSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// GetSession 获取会话
func (h *Handler) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// DeleteSession 删除会话
func (h *Handler) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "会话已删除"})
}

// UpdateStep 更新步骤
func (h *Handler) UpdateStep(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Step Step `json:"step" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateStep(id, req.Step); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "步骤已更新"})
}

// SetPoolConfig 设置存储池配置
func (h *Handler) SetPoolConfig(c *gin.Context) {
	id := c.Param("id")
	var config PoolConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SetPoolConfig(id, config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "存储池配置已保存"})
}

// SetVolumeConfig 设置卷配置
func (h *Handler) SetVolumeConfig(c *gin.Context) {
	id := c.Param("id")
	var config VolumeConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SetVolumeConfig(id, config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "卷配置已保存"})
}

// SetShareConfig 设置共享配置
func (h *Handler) SetShareConfig(c *gin.Context) {
	id := c.Param("id")
	var config ShareConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SetShareConfig(id, config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "共享配置已保存"})
}

// CompleteSession 完成会话
func (h *Handler) CompleteSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CompleteSession(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "设置完成"})
}

// GetRecommendations 获取RAID推荐
func (h *Handler) GetRecommendations(c *gin.Context) {
	id := c.Param("id")
	priority := c.DefaultQuery("priority", "balanced")

	recommendations, err := h.manager.GetRecommendations(id, priority)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"recommendations": recommendations})
}

// EstimateCapacity 估算容量
func (h *Handler) EstimateCapacity(c *gin.Context) {
	id := c.Param("id")
	raidType := RAIDType(c.DefaultQuery("raid_type", "raid1"))

	estimation, err := h.manager.EstimateCapacity(id, raidType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, estimation)
}
