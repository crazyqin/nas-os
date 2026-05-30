// Package rdmaoptimizer 提供 RDMA 网络传输优化
package rdmaoptimizer

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers RDMA 优化器 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	v1 := r.Group("/v1")
	{
		// 链路管理
		v1.GET("/links", h.detectLinks)
		v1.POST("/links", h.createLink)
		v1.GET("/links/:id", h.getLink)
		v1.DELETE("/links/:id", h.deleteLink)
		v1.PUT("/links/:id/state", h.updateLinkState)
		v1.GET("/links/:id/metrics", h.getLinkMetrics)

		// 路径管理
		v1.POST("/paths", h.createPath)
		v1.GET("/paths", h.listPaths)
		v1.GET("/paths/:id", h.getPath)
		v1.DELETE("/paths/:id", h.deletePath)
		v1.GET("/paths/select", h.selectPath)
		v1.PUT("/paths/:id/congestion", h.updateCongestion)

		// 传输优化
		v1.POST("/optimize", h.optimizeTransfer)
		v1.GET("/metrics/:link_id", h.getMetrics)

		// 统计信息
		v1.GET("/stats", h.getStats)

		// 配置管理
		v1.GET("/config", h.getConfig)
		v1.PUT("/config", h.updateConfig)
	}
}

// detectLinks 检测链路
func (h *Handlers) detectLinks(c *gin.Context) {
	links := h.manager.DetectLinks()

	c.JSON(http.StatusOK, gin.H{"data": links})
}

// createLink 创建链路
func (h *Handlers) createLink(c *gin.Context) {
	var req CreateLinkRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.manager.CreateLink(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": link})
}

// getLink 获取链路详情
func (h *Handlers) getLink(c *gin.Context) {
	id := c.Param("id")

	link, err := h.manager.GetLink(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": link})
}

// deleteLink 删除链路
func (h *Handlers) deleteLink(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteLink(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "link deleted"})
}

// updateLinkState 更新链路状态
func (h *Handlers) updateLinkState(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		State LinkState `json:"state" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateLinkState(id, req.State); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "link state updated"})
}

// getLinkMetrics 获取链路指标
func (h *Handlers) getLinkMetrics(c *gin.Context) {
	id := c.Param("id")

	metrics, err := h.manager.GetLinkMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

// createPath 创建路径
func (h *Handlers) createPath(c *gin.Context) {
	var req CreatePathRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	path, err := h.manager.CreatePath(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": path})
}

// listPaths 列出路径
func (h *Handlers) listPaths(c *gin.Context) {
	sourceDevice := c.Query("source_device")
	destDevice := c.Query("dest_device")

	paths := h.manager.ListPaths(sourceDevice, destDevice)

	c.JSON(http.StatusOK, gin.H{"data": paths})
}

// getPath 获取路径详情
func (h *Handlers) getPath(c *gin.Context) {
	id := c.Param("id")

	path, err := h.manager.GetPath(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": path})
}

// deletePath 删除路径
func (h *Handlers) deletePath(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeletePath(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "path deleted"})
}

// selectPath 选择最佳路径
func (h *Handlers) selectPath(c *gin.Context) {
	sourceDevice := c.Query("source_device")
	destDevice := c.Query("dest_device")

	if sourceDevice == "" || destDevice == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_device and dest_device are required"})
		return
	}

	path, err := h.manager.SelectPath(sourceDevice, destDevice)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": path})
}

// updateCongestion 更新拥塞状态
func (h *Handlers) updateCongestion(c *gin.Context) {
	id := c.Param("id")

	congestionStr := c.Query("congestion")
	congestion, err := strconv.ParseFloat(congestionStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid congestion value"})
		return
	}

	if err := h.manager.UpdateCongestion(id, congestion); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "congestion updated"})
}

// optimizeTransfer 优化传输
func (h *Handlers) optimizeTransfer(c *gin.Context) {
	var req OptimizeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metrics, err := h.manager.OptimizeTransfer(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

// getMetrics 获取传输指标
func (h *Handlers) getMetrics(c *gin.Context) {
	linkID := c.Param("link_id")

	metrics, err := h.manager.GetMetrics(linkID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var config CongestionConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateConfig(config)

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}
