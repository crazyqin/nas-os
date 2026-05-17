// Package webshare 搜索API handlers
// 提供搜索、索引、建议等API接口
// 参考: TrueNAS Spotlight Search
package webshare

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SearchAPIHandler 搜索API处理器
type SearchAPIHandler struct {
	engine *SearchEngine
	logger *zap.Logger
}

// NewSearchAPIHandler 创建搜索API处理器
func NewSearchAPIHandler(engine *SearchEngine, logger *zap.Logger) *SearchAPIHandler {
	return &SearchAPIHandler{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes 注册搜索路由
func (h *SearchAPIHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/search", h.Search)
	r.POST("/index", h.TriggerIndex)
	r.GET("/suggest", h.Suggest)
	r.GET("/search/stats", h.GetStats)
	r.POST("/search/tag", h.AddTag)
	r.DELETE("/search/tag", h.RemoveTag)
}

// Search 搜索API
// GET /api/v1/webshare/search
func (h *SearchAPIHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词缺失"})
		return
	}

	// 解析请求参数
	req := SearchRequest{
		Query:      query,
		Path:       c.Query("path"),
		FileType:   c.Query("type"),
		Content:    c.DefaultQuery("content", "true") == "true",
		Fuzzy:      c.DefaultQuery("fuzzy", "false") == "true",
		Highlight:  c.DefaultQuery("highlight", "true") == "true",
		ExactMatch: c.DefaultQuery("exact", "false") == "true",
		CaseSense:  c.DefaultQuery("case", "false") == "true",
		SortBy:     c.DefaultQuery("sort", "relevance"),
		SortDesc:   c.DefaultQuery("order", "desc") == "desc",
	}

	// 解析扩展名
	if exts := c.Query("ext"); exts != "" {
		req.Extensions = strings.Split(exts, ",")
	}

	// 解析标签
	if tags := c.Query("tags"); tags != "" {
		req.Tags = strings.Split(tags, ",")
	}

	// 解析大小限制
	if minSize := c.Query("minSize"); minSize != "" {
		if v, err := strconv.ParseInt(minSize, 10, 64); err == nil {
			req.MinSize = v
		}
	}
	if maxSize := c.Query("maxSize"); maxSize != "" {
		if v, err := strconv.ParseInt(maxSize, 10, 64); err == nil {
			req.MaxSize = v
		}
	}

	// 解析时间限制
	if fromDate := c.Query("fromDate"); fromDate != "" {
		if t, err := time.Parse("2006-01-02", fromDate); err == nil {
			req.FromDate = &t
		}
	}
	if toDate := c.Query("toDate"); toDate != "" {
		if t, err := time.Parse("2006-01-02", toDate); err == nil {
			req.ToDate = &t
		}
	}

	// 解析分页
	if limit := c.Query("limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil {
			req.MaxResults = v
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil {
			req.Offset = v
		}
	}

	// 执行搜索
	resp, err := h.engine.Search(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("搜索失败", zap.String("query", query), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// TriggerIndex 触发索引
// POST /api/v1/webshare/index
func (h *SearchAPIHandler) TriggerIndex(c *gin.Context) {
	var req struct {
		Path         string `json:"path"`
		Recursive    bool   `json:"recursive"`
		ForceReindex bool   `json:"forceReindex"`
		MaxDepth     int    `json:"maxDepth"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// 尝试从查询参数获取
		req.Path = c.Query("path")
		req.Recursive = c.DefaultQuery("recursive", "true") == "true"
		req.ForceReindex = c.DefaultQuery("force", "false") == "true"
	}

	if req.Path == "" {
		req.Path = h.engine.config.BaseDir
	}

	// 执行索引
	indexReq := IndexRequest{
		Path:         req.Path,
		Recursive:    req.Recursive,
		ForceReindex: req.ForceReindex,
		MaxDepth:     req.MaxDepth,
	}

	resp, err := h.engine.contentIndexer.Index(c.Request.Context(), indexReq)
	if err != nil {
		h.logger.Error("索引失败", zap.String("path", req.Path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "索引失败: " + err.Error()})
		return
	}

	// 重建搜索索引
	h.engine.rebuildSearchIndex()

	c.JSON(http.StatusOK, gin.H{
		"message":       "索引完成",
		"totalFiles":    resp.TotalFiles,
		"indexedFiles":  resp.IndexedFiles,
		"failedFiles":   resp.FailedFiles,
		"took":          resp.Took.String(),
		"errors":        resp.Errors,
	})
}

// Suggest 搜索建议
// GET /api/v1/webshare/suggest
func (h *SearchAPIHandler) Suggest(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "查询参数缺失"})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	req := SuggestRequest{
		Query: query,
		Path:  c.Query("path"),
		Limit: limit,
	}

	// 获取建议
	suggestions := h.engine.getSuggestions(req.Query)

	// 限制数量
	if len(suggestions) > req.Limit {
		suggestions = suggestions[:req.Limit]
	}

	resp := SuggestResponse{
		Query:       req.Query,
		Suggestions: suggestions,
	}

	c.JSON(http.StatusOK, resp)
}

// GetStats 获取搜索统计
// GET /api/v1/webshare/search/stats
func (h *SearchAPIHandler) GetStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, stats)
}

// AddTag 添加标签
// POST /api/v1/webshare/search/tag
func (h *SearchAPIHandler) AddTag(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Tag  string `json:"tag" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	h.engine.contentIndexer.AddTag(req.Path, req.Tag)

	c.JSON(http.StatusOK, gin.H{
		"message": "标签添加成功",
		"path":    req.Path,
		"tag":     req.Tag,
	})
}

// RemoveTag 移除标签
// DELETE /api/v1/webshare/search/tag
func (h *SearchAPIHandler) RemoveTag(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Tag  string `json:"tag" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	h.engine.contentIndexer.RemoveTag(req.Path, req.Tag)

	c.JSON(http.StatusOK, gin.H{
		"message": "标签移除成功",
		"path":    req.Path,
		"tag":     req.Tag,
	})
}
