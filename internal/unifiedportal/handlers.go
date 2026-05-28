// Package unifiedportal - HTTP API 处理器
package unifiedportal

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 统一门户 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	portal := rg.Group("/portal")
	{
		// 全局搜索
		portal.POST("/search", h.search)
		portal.GET("/search", h.searchGet)

		// 快捷操作
		portal.GET("/actions", h.getActions)

		// 最近搜索
		portal.GET("/recent", h.getRecentSearches)

		// 门户配置
		portal.GET("/config", h.getConfig)
	}
}

func (h *Handlers) search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp := h.manager.Search(&req)
	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) searchGet(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}
	req := &SearchRequest{
		Query:  query,
		Limit:  20,
	}
	resp := h.manager.Search(req)
	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) getActions(c *gin.Context) {
	actions := h.manager.GetActions()
	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

func (h *Handlers) getRecentSearches(c *gin.Context) {
	searches := h.manager.GetRecentSearches()
	c.JSON(http.StatusOK, gin.H{"searches": searches})
}

func (h *Handlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.config)
}
