package aifilesearch

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers AI 文件搜索 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 AI 文件搜索处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	search := rg.Group("/aifilesearch")
	{
		search.POST("", h.search)
		search.GET("/stats", h.getStats)
		search.POST("/rebuild", h.rebuildIndex)
		search.GET("/suggestions", h.getSuggestions)
	}
}

func (h *Handlers) search(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if query.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "查询不能为空"})
		return
	}

	if query.Limit <= 0 {
		query.Limit = 20
	}

	result := h.manager.Search(query)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetIndexStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

func (h *Handlers) rebuildIndex(c *gin.Context) {
	go h.manager.RebuildIndex()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "索引重建已启动"})
}

func (h *Handlers) getSuggestions(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []string{}})
		return
	}

	searchQuery := SearchQuery{Query: query, Limit: 5}
	result := h.manager.Search(searchQuery)

	suggestions := make([]string, 0)
	for _, r := range result.Results {
		suggestions = append(suggestions, r.Name)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": suggestions})
}
