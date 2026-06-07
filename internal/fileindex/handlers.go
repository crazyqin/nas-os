// handlers.go - 文件索引 HTTP 接口
package fileindex

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 文件索引接口处理器
type Handlers struct {
	logger  *zap.Logger
	indexer *Indexer
}

// NewHandlers 创建文件索引接口处理器
func NewHandlers(logger *zap.Logger, indexer *Indexer) *Handlers {
	return &Handlers{logger: logger, indexer: indexer}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	idx := rg.Group("/fileindex")
	{
		idx.POST("/build", h.buildIndex)
		idx.POST("/search", h.search)
		idx.GET("/stats", h.stats)
		idx.GET("/recent", h.recent)
		idx.GET("/largest", h.largest)
		idx.GET("/entry", h.getEntry)
	}
}

// buildIndex 构建索引
func (h *Handlers) buildIndex(c *gin.Context) {
	stats, err := h.indexer.Build()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "索引构建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// search 搜索文件
func (h *Handlers) search(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的查询参数"})
		return
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}

	results := h.indexer.Search(query)
	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  results,
		"total": len(results),
	})
}

// stats 获取索引统计
func (h *Handlers) stats(c *gin.Context) {
	stats := h.indexer.Stats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// recent 最近修改的文件
func (h *Handlers) recent(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	files := h.indexer.ListRecent(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": files})
}

// largest 最大的文件
func (h *Handlers) largest(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	files := h.indexer.ListLargest(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": files})
}

// getEntry 获取单个文件信息
func (h *Handlers) getEntry(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "缺少 path 参数"})
		return
	}
	entry, ok := h.indexer.GetEntry(path)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "文件未在索引中"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": entry})
}
