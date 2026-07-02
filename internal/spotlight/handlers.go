package spotlight

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewHandlers 创建处理器.
func NewHandlers(logger *zap.Logger, mgr *Manager) *Handlers {
	return &Handlers{
		logger: logger,
		mgr:    mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	spotlight := rg.Group("/spotlight")
	{
		// 搜索
		spotlight.GET("/search", h.Search)
		spotlight.POST("/search", h.SearchPost)

		// 搜索建议
		spotlight.GET("/suggest", h.Suggest)

		// 文档管理
		spotlight.POST("/index", h.IndexDocument)
		spotlight.POST("/index/batch", h.IndexBatch)
		spotlight.DELETE("/index/:id", h.RemoveDocument)
		spotlight.GET("/index/:id", h.GetDocument)

		// 索引统计
		spotlight.GET("/stats", h.GetStats)

		// 索引管理
		spotlight.POST("/rebuild", h.RebuildIndex)

		// 热门搜索
		spotlight.GET("/popular", h.GetPopularSearches)
	}
}

// Search GET 搜索.
func (h *Handlers) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "query parameter 'q' is required",
		})
		return
	}

	req := h.parseSearchRequest(c)
	req.Query = query

	result, err := h.mgr.Search(req)
	if err != nil {
		h.logger.Error("search failed", zap.String("query", query), zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "search failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "success",
		Data:    result,
	})
}

// SearchPost POST 搜索.
func (h *Handlers) SearchPost(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.Query == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "query is required",
		})
		return
	}

	result, err := h.mgr.Search(&req)
	if err != nil {
		h.logger.Error("search failed", zap.String("query", req.Query), zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "search failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "success",
		Data:    result,
	})
}

// Suggest 搜索建议.
func (h *Handlers) Suggest(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "query parameter 'q' is required",
		})
		return
	}

	limit := 10
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && l > 0 {
		limit = l
	}

	req := &SuggestRequest{
		Query: query,
		Limit: limit,
	}

	result, err := h.mgr.Suggest(req)
	if err != nil {
		h.logger.Error("suggest failed", zap.String("query", query), zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "suggest failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "success",
		Data:    result,
	})
}

// IndexDocument 索引单个文档.
func (h *Handlers) IndexDocument(c *gin.Context) {
	var doc Document
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}
	if doc.FileType == "" {
		doc.FileType = classifyFileType(doc.Extension)
	}

	if err := h.mgr.IndexDocument(&doc); err != nil {
		h.logger.Error("index document failed", zap.String("path", doc.Path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "index failed: " + err.Error(),
		})
		return
	}

	h.logger.Info("document indexed", zap.String("id", doc.ID), zap.String("path", doc.Path))

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "document indexed",
		Data:    doc,
	})
}

// IndexBatch 批量索引文档.
func (h *Handlers) IndexBatch(c *gin.Context) {
	var docs []*Document
	if err := c.ShouldBindJSON(&docs); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 设置默认值
	for _, doc := range docs {
		if doc.CreatedAt.IsZero() {
			doc.CreatedAt = time.Now()
		}
		if doc.UpdatedAt.IsZero() {
			doc.UpdatedAt = doc.CreatedAt
		}
		if doc.FileType == "" {
			doc.FileType = classifyFileType(doc.Extension)
		}
	}

	indexed, err := h.mgr.IndexDocuments(docs)
	if err != nil {
		h.logger.Error("batch index failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "batch index failed: " + err.Error(),
		})
		return
	}

	h.logger.Info("batch indexed", zap.Int("total", len(docs)), zap.Int("indexed", indexed))

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "batch index completed",
		Data: gin.H{
			"total":   len(docs),
			"indexed": indexed,
		},
	})
}

// RemoveDocument 删除文档.
func (h *Handlers) RemoveDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "document id is required",
		})
		return
	}

	if err := h.mgr.RemoveDocument(id); err != nil {
		h.logger.Error("remove document failed", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "remove failed: " + err.Error(),
		})
		return
	}

	h.logger.Info("document removed", zap.String("id", id))

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "document removed",
	})
}

// GetDocument 获取文档.
func (h *Handlers) GetDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "document id is required",
		})
		return
	}

	doc, exists := h.mgr.GetDocument(id)
	if !exists {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "document not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "success",
		Data:    doc,
	})
}

// GetStats 获取索引统计.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "success",
		Data:    stats,
	})
}

// RebuildIndex 重建索引.
func (h *Handlers) RebuildIndex(c *gin.Context) {
	if err := h.mgr.RebuildIndex(); err != nil {
		h.logger.Error("rebuild index failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "rebuild failed: " + err.Error(),
		})
		return
	}

	h.logger.Info("index rebuilt")

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "index rebuilt",
	})
}

// GetPopularSearches 获取热门搜索.
func (h *Handlers) GetPopularSearches(c *gin.Context) {
	limit := 10
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && l > 0 {
		limit = l
	}

	popular := h.mgr.GetPopularSearches(limit)

	c.JSON(http.StatusOK, response{
		Code:    200,
		Message: "success",
		Data:    popular,
	})
}

// parseSearchRequest 解析搜索请求参数.
func (h *Handlers) parseSearchRequest(c *gin.Context) *SearchRequest {
	req := &SearchRequest{
		Path:      c.Query("path"),
		FileType:  FileType(c.Query("file_type")),
		SortBy:    c.DefaultQuery("sort_by", "relevance"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	// 扩展名
	if ext := c.Query("extensions"); ext != "" {
		req.Extensions = strings.Split(ext, ",")
	}

	// 标签
	if tags := c.Query("tags"); tags != "" {
		req.Tags = strings.Split(tags, ",")
	}

	// 大小
	if minSize, err := strconv.ParseInt(c.DefaultQuery("min_size", "0"), 10, 64); err == nil && minSize > 0 {
		req.MinSize = &minSize
	}
	if maxSize, err := strconv.ParseInt(c.DefaultQuery("max_size", "0"), 10, 64); err == nil && maxSize > 0 {
		req.MaxSize = &maxSize
	}

	// 日期
	if after := c.Query("after"); after != "" {
		if t, err := time.Parse(time.RFC3339, after); err == nil {
			req.After = &t
		}
	}
	if before := c.Query("before"); before != "" {
		if t, err := time.Parse(time.RFC3339, before); err == nil {
			req.Before = &t
		}
	}

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	req.Page = page
	req.PageSize = pageSize

	return req
}

// classifyFileType 根据扩展名分类文件类型.
func classifyFileType(ext string) FileType {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	docExts := map[string]bool{
		"doc": true, "docx": true, "pdf": true, "txt": true, "rtf": true,
		"odt": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true,
		"csv": true, "md": true,
	}
	imgExts := map[string]bool{
		"jpg": true, "jpeg": true, "png": true, "gif": true, "bmp": true,
		"svg": true, "webp": true, "ico": true, "tiff": true,
	}
	videoExts := map[string]bool{
		"mp4": true, "avi": true, "mkv": true, "mov": true, "wmv": true,
		"flv": true, "webm": true, "m4v": true,
	}
	audioExts := map[string]bool{
		"mp3": true, "wav": true, "flac": true, "aac": true, "ogg": true,
		"wma": true, "m4a": true,
	}
	archiveExts := map[string]bool{
		"zip": true, "rar": true, "7z": true, "tar": true, "gz": true,
		"bz2": true, "xz": true,
	}
	codeExts := map[string]bool{
		"go": true, "py": true, "js": true, "ts": true, "java": true,
		"c": true, "cpp": true, "h": true, "hpp": true, "rs": true,
		"rb": true, "php": true, "swift": true, "kt": true,
		"html": true, "css": true, "scss": true, "less": true,
		"json": true, "xml": true, "yaml": true, "yml": true, "toml": true,
		"sql": true, "sh": true, "bash": true, "zsh": true,
	}

	switch {
	case docExts[ext]:
		return FileTypeDocument
	case imgExts[ext]:
		return FileTypeImage
	case videoExts[ext]:
		return FileTypeVideo
	case audioExts[ext]:
		return FileTypeAudio
	case archiveExts[ext]:
		return FileTypeArchive
	case codeExts[ext]:
		return FileTypeCode
	default:
		return FileTypeOther
	}
}
