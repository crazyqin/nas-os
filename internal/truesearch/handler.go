package truesearch

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 搜索引擎HTTP处理器
type Handler struct {
	engine *SearchEngine
}

// NewHandler 创建处理器
func NewHandler(engine *SearchEngine) *Handler {
	return &Handler{engine: engine}
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

// IndexRequest 索引请求
type IndexRequest struct {
	ID         string   `json:"id" binding:"required"`
	FilePath   string   `json:"filePath" binding:"required"`
	FileName   string   `json:"fileName" binding:"required"`
	FileType   string   `json:"fileType"`
	Size       int64    `json:"size"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
}

// Search 搜索
func (h *Handler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	results := h.engine.Search(query, limit)
	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"results": results,
		"total":   len(results),
	})
}

// Suggest 搜索建议
func (h *Handler) Suggest(c *gin.Context) {
	prefix := c.Query("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parameter 'prefix' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	suggestions := h.engine.GetSuggestions(prefix, limit)
	c.JSON(http.StatusOK, gin.H{
		"prefix":      prefix,
		"suggestions": suggestions,
	})
}

// IndexDocument 索引文档
func (h *Handler) IndexDocument(c *gin.Context) {
	var req IndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entry := &IndexEntry{
		ID:       req.ID,
		FilePath: req.FilePath,
		FileName: req.FileName,
		FileType: req.FileType,
		Size:     req.Size,
		Content:  req.Content,
		Tags:     req.Tags,
	}

	h.engine.IndexDocument(entry)
	c.JSON(http.StatusCreated, gin.H{"message": "document indexed", "id": req.ID})
}

// RemoveDocument 移除文档
func (h *Handler) RemoveDocument(c *gin.Context) {
	id := c.Param("id")
	h.engine.RemoveDocument(id)
	c.JSON(http.StatusOK, gin.H{"message": "document removed", "id": id})
}

// RebuildIndex 重建索引
func (h *Handler) RebuildIndex(c *gin.Context) {
	h.engine.RebuildIndex()
	c.JSON(http.StatusOK, gin.H{"message": "index rebuilt"})
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, stats)
}
