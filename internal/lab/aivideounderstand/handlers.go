// Package aivideounderstand 提供 AI 视频理解功能.
package aivideounderstand

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 视频理解 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	videoGroup := api.Group("/video")
	{
		videoGroup.POST("/analyze", h.analyzeVideo)
		videoGroup.GET("/analyses", h.listAnalyses)
		videoGroup.GET("/analyses/:id", h.getAnalysis)
		videoGroup.GET("/analyses/:id/scenes", h.getScenes)
		videoGroup.GET("/analyses/:id/objects", h.getObjects)
		videoGroup.POST("/search", h.searchVideos)
		videoGroup.GET("/analyses/:id/highlights", h.getHighlights)
		videoGroup.DELETE("/analyses/:id", h.deleteAnalysis)
		videoGroup.GET("/stats", h.getStats)
	}
}

// analyzeVideo 提交视频分析.
func (h *Handlers) analyzeVideo(c *gin.Context) {
	var req struct {
		VideoPath string `json:"video_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	analysis, err := h.manager.AnalyzeVideo(req.VideoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分析失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "视频分析已提交",
		"analysis": analysis,
	})
}

// listAnalyses 列出所有分析.
func (h *Handlers) listAnalyses(c *gin.Context) {
	analyses := h.manager.ListAnalyses()
	c.JSON(http.StatusOK, gin.H{
		"analyses": analyses,
		"total":    len(analyses),
	})
}

// getAnalysis 获取分析详情.
func (h *Handlers) getAnalysis(c *gin.Context) {
	id := c.Param("id")

	analysis, err := h.manager.GetAnalysis(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分析未找到"})
		return
	}

	scenes, _ := h.manager.GetScenes(id)
	objects, _ := h.manager.GetObjects(id)

	c.JSON(http.StatusOK, gin.H{
		"analysis": analysis,
		"scenes":   scenes,
		"objects":  objects,
	})
}

// getScenes 获取场景列表.
func (h *Handlers) getScenes(c *gin.Context) {
	id := c.Param("id")

	scenes, err := h.manager.GetScenes(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分析未找到"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scenes": scenes,
		"total":  len(scenes),
	})
}

// getObjects 获取物体列表.
func (h *Handlers) getObjects(c *gin.Context) {
	id := c.Param("id")

	objects, err := h.manager.GetObjects(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分析未找到"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"objects": objects,
		"total":   len(objects),
	})
}

// searchVideos 语义搜索视频.
func (h *Handlers) searchVideos(c *gin.Context) {
	var query VideoSearchQuery

	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if query.MaxResults <= 0 {
		query.MaxResults = 20
	}

	results := h.manager.SearchVideos(&query)

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

// getHighlights 获取高光时刻.
func (h *Handlers) getHighlights(c *gin.Context) {
	id := c.Param("id")

	highlights, err := h.manager.GetHighlights(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分析未找到"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"highlights": highlights,
		"total":      len(highlights),
	})
}

// deleteAnalysis 删除分析.
func (h *Handlers) deleteAnalysis(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteAnalysis(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分析未找到"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "分析已删除",
	})
}

// getStats 获取统计信息.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// ParseMaxResults 从查询参数解析 max_results.
func ParseMaxResults(c *gin.Context, defaultVal int) int {
	if v := c.Query("max_results"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}
