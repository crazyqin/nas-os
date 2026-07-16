// Package contentseo 提供 HTTP 处理器
package contentseo

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers HTTP 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器实例.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	seo := r.Group("/contentseo")
	{
		seo.POST("/search", h.Search)
		seo.GET("/stats", h.GetStats)
		seo.POST("/index/rebuild", h.RebuildIndex)
		seo.GET("/index/status", h.GetIndexStatus)
	}
}

// Search 全文搜索.
func (h *Handlers) Search(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	result, err := h.engine.Search(query)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code: 200,
		Data: result,
	})
}

// GetStats 获取搜索统计.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, response{
		Code: 200,
		Data: stats,
	})
}

// RebuildIndex 重建索引.
func (h *Handlers) RebuildIndex(c *gin.Context) {
	var req RebuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认全量重建
		req.FullRebuild = true
	}

	if err := h.engine.RebuildIndex(req.FullRebuild); err != nil {
		c.JSON(http.StatusConflict, response{
			Code:    409,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "索引重建已启动",
	})
}

// GetIndexStatus 获取索引状态.
func (h *Handlers) GetIndexStatus(c *gin.Context) {
	status := h.engine.GetIndexStatus()
	c.JSON(http.StatusOK, response{
		Code: 200,
		Data: status,
	})
}
