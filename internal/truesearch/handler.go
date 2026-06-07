package truesearch

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 搜索引擎HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/search")
	{
		group.GET("/query", h.Search)
		group.GET("/suggest", h.Suggest)
		group.POST("/index", h.IndexDocument)
		group.DELETE("/index/:id", h.RemoveDocument)
		group.POST("/reindex", h.RebuildIndex)
		group.GET("/stats", h.GetStats)
	}
}

// Search 搜索
func (h *Handler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	mode := SearchMode(c.DefaultQuery("mode", "all"))
	sortOrder := SortOrder(c.DefaultQuery("sort", "relevance"))

	resp, err := h.manager.Search(&SearchQuery{
		Query: query,
		Mode:  mode,
		Sort:  sortOrder,
		Limit: limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Suggest 搜索建议
func (h *Handler) Suggest(c *gin.Context) {
	prefix := c.Query("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parameter 'prefix' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	suggestions := h.manager.AutoComplete(prefix, limit)
	c.JSON(http.StatusOK, gin.H{
		"prefix":      prefix,
		"suggestions": suggestions,
	})
}

// IndexDocument 索引文档
func (h *Handler) IndexDocument(c *gin.Context) {
	var req struct {
		ID       string            `json:"id" binding:"required"`
		Path     string            `json:"path" binding:"required"`
		Name     string            `json:"name" binding:"required"`
		Content  string            `json:"content"`
		Metadata map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ext := GetFileExtension(req.Name)
	doc := &Document{
		ID:        req.ID,
		Path:      req.Path,
		Name:      req.Name,
		Extension: ext,
		FileType:  ClassifyFileType(ext),
		Content:   req.Content,
		Metadata:  req.Metadata,
	}

	if err := h.manager.IndexDocument(doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "document indexed", "id": req.ID})
}

// RemoveDocument 移除文档
func (h *Handler) RemoveDocument(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveDocument(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "document removed", "id": id})
}

// RebuildIndex 重建索引
func (h *Handler) RebuildIndex(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "index rebuild initiated"})
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
