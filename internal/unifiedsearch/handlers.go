// Package unifiedsearch 统一搜索 API 处理器
// 提供 REST API 接口，支持文件系统、照片库、文档、邮件的跨模块搜索
package unifiedsearch

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 统一搜索 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	search := r.Group("/search")
	{
		// 统一搜索接口
		search.GET("", h.unifiedSearch)
		search.GET("/", h.unifiedSearch)

		// 索引管理
		search.POST("/index/rebuild", h.rebuildIndex)
		search.POST("/index/build", h.buildIndex)
		search.POST("/index/incremental", h.incrementalUpdate)
		search.GET("/index/stats", h.getIndexStats)

		// 搜索建议
		search.GET("/suggestions", h.getSuggestions)

		// 搜索历史
		search.GET("/history", h.getHistory)
		search.DELETE("/history", h.clearHistory)

		// 热门搜索
		search.GET("/hot", h.getHotSearches)

		// 文档管理
		search.GET("/documents", h.listDocuments)
		search.GET("/documents/:id", h.getDocument)
		search.POST("/documents", h.addDocument)
		search.PUT("/documents/:id", h.updateDocument)
		search.DELETE("/documents/:id", h.deleteDocument)

		// 索引任务
		search.GET("/index/tasks", h.listTasks)
		search.GET("/index/tasks/:id", h.getTask)

		// 配置
		search.GET("/config", h.getConfig)
		search.PUT("/config", h.updateConfig)

		// 跨模块搜索
		search.GET("/files", h.searchFiles)
		search.GET("/photos", h.searchPhotos)
		search.GET("/documents/search", h.searchDocuments)
		search.GET("/emails", h.searchEmails)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// unifiedSearch GET /api/v1/search?q=keyword
// 统一搜索接口，支持所有内容类型的跨模块搜索.
func (h *Handlers) unifiedSearch(c *gin.Context) {
	queryStr := c.Query("q")
	if queryStr == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	query := SearchQuery{
		Query:     queryStr,
		Page:      1,
		PageSize:  20,
		Highlight: true,
	}

	// 解析分页参数
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			query.Page = page
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			query.PageSize = pageSize
		}
	}

	// 解析内容类型过滤
	if typesStr := c.Query("types"); typesStr != "" {
		for _, t := range splitAndTrim(typesStr) {
			ct := ContentType(t)
			if IsValidContentType(ct) {
				query.Types = append(query.Types, ct)
			}
		}
	}

	// 解析标签过滤
	if tagsStr := c.Query("tags"); tagsStr != "" {
		query.Tags = splitAndTrim(tagsStr)
	}

	// 解析路径过滤
	if path := c.Query("path"); path != "" {
		query.Path = path
	}

	// 解析排序
	if sortBy := c.Query("sort_by"); sortBy != "" {
		so := SortOrder(sortBy)
		if IsValidSortOrder(so) {
			query.SortBy = so
		}
	}

	// 解析布尔操作符
	if boolOp := c.Query("boolean_op"); boolOp != "" {
		op := BooleanOp(boolOp)
		if IsValidBooleanOp(op) {
			query.BooleanOp = op
		}
	}

	// 解析模糊搜索
	if fuzzyStr := c.Query("fuzzy"); fuzzyStr == "true" || fuzzyStr == "1" {
		query.Fuzzy = true
	}

	// 解析高亮
	if highlightStr := c.Query("highlight"); highlightStr == "false" || highlightStr == "0" {
		query.Highlight = false
	}

	// 解析正则表达式
	if regexStr := c.Query("regex"); regexStr == "true" || regexStr == "1" {
		query.UseRegex = true
	}

	// 解析大小过滤
	if sizeMinStr := c.Query("size_min"); sizeMinStr != "" {
		if sizeMin, err := strconv.ParseInt(sizeMinStr, 10, 64); err == nil {
			query.SizeMin = &sizeMin
		}
	}
	if sizeMaxStr := c.Query("size_max"); sizeMaxStr != "" {
		if sizeMax, err := strconv.ParseInt(sizeMaxStr, 10, 64); err == nil {
			query.SizeMax = &sizeMax
		}
	}

	// 解析日期过滤
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			query.DateFrom = &dateFrom
		}
	}
	if dateToStr := c.Query("date_to"); dateToStr != "" {
		if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			query.DateTo = &dateTo
		}
	}

	result, err := h.manager.Search(&query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// rebuildIndex POST /api/v1/search/index/rebuild
// 重建索引.
func (h *Handlers) rebuildIndex(c *gin.Context) {
	if err := h.manager.RebuildIndex(); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "index rebuild started",
	})
}

// buildIndex POST /api/v1/search/index/build.
func (h *Handlers) buildIndex(c *gin.Context) {
	var req IndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	task, err := h.manager.BuildIndex(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, response{
		Code:    0,
		Message: "index build started",
		Data: IndexResponse{
			TaskID:  task.ID,
			Message: "index build task created",
		},
	})
}

// incrementalUpdate POST /api/v1/search/index/incremental.
func (h *Handlers) incrementalUpdate(c *gin.Context) {
	var req IndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	task, err := h.manager.IncrementalUpdate(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, response{
		Code:    0,
		Message: "incremental update started",
		Data: IndexResponse{
			TaskID:  task.ID,
			Message: "incremental update task created",
		},
	})
}

// getIndexStats GET /api/v1/search/stats
// 获取索引统计信息.
func (h *Handlers) getIndexStats(c *gin.Context) {
	stats := h.manager.GetIndexStats()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getSuggestions GET /api/v1/search/suggestions?q=keyword
// 获取搜索建议.
func (h *Handlers) getSuggestions(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	suggestions := h.manager.GetSuggestions(query, limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: SuggestResponse{
			Suggestions: suggestions,
		},
	})
}

// getHistory GET /api/v1/search/history.
func (h *Handlers) getHistory(c *gin.Context) {
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history := h.manager.GetSearchHistory(limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// clearHistory DELETE /api/v1/search/history.
func (h *Handlers) clearHistory(c *gin.Context) {
	h.manager.ClearSearchHistory()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "history cleared",
	})
}

// getHotSearches GET /api/v1/search/hot.
func (h *Handlers) getHotSearches(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	hot := h.manager.GetHotSearches(limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    hot,
	})
}

// listDocuments GET /api/v1/search/documents.
func (h *Handlers) listDocuments(c *gin.Context) {
	var contentType ContentType
	if typeStr := c.Query("type"); typeStr != "" {
		ct := ContentType(typeStr)
		if IsValidContentType(ct) {
			contentType = ct
		}
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	docs := h.manager.ListDocuments(contentType, limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    docs,
	})
}

// getDocument GET /api/v1/search/documents/:id.
func (h *Handlers) getDocument(c *gin.Context) {
	id := c.Param("id")
	doc, err := h.manager.GetDocument(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    doc,
	})
}

// addDocument POST /api/v1/search/documents.
func (h *Handlers) addDocument(c *gin.Context) {
	var doc SearchIndex
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddDocument(&doc); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "document added",
		Data:    doc,
	})
}

// updateDocument PUT /api/v1/search/documents/:id.
func (h *Handlers) updateDocument(c *gin.Context) {
	id := c.Param("id")
	var req UpdateIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	req.ID = id
	if err := h.manager.UpdateDocument(&req); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "document updated",
	})
}

// deleteDocument DELETE /api/v1/search/documents/:id.
func (h *Handlers) deleteDocument(c *gin.Context) {
	id := c.Param("id")
	doc, err := h.manager.GetDocument(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	if err := h.manager.RemoveDocument(doc.Path); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "document deleted",
	})
}

// listTasks GET /api/v1/search/index/tasks.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tasks,
	})
}

// getTask GET /api/v1/search/index/tasks/:id.
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    task,
	})
}

// getConfig GET /api/v1/search/config.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig PUT /api/v1/search/config.
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg SearchConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// searchFiles GET /api/v1/search/files?q=keyword
// 文件系统搜索.
func (h *Handlers) searchFiles(c *gin.Context) {
	queryStr := c.Query("q")
	if queryStr == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	query := SearchQuery{
		Query:     queryStr,
		Types:     []ContentType{ContentTypeFile},
		Page:      1,
		PageSize:  20,
		Highlight: true,
	}

	applyPaginationParams(c, &query)

	result, err := h.manager.Search(&query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// searchPhotos GET /api/v1/search/photos?q=keyword
// 照片库搜索.
func (h *Handlers) searchPhotos(c *gin.Context) {
	queryStr := c.Query("q")
	if queryStr == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	query := SearchQuery{
		Query:     queryStr,
		Types:     []ContentType{ContentTypePhoto},
		Page:      1,
		PageSize:  20,
		Highlight: true,
	}

	applyPaginationParams(c, &query)

	result, err := h.manager.Search(&query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// searchDocuments GET /api/v1/search/documents/search?q=keyword
// 文档内容搜索.
func (h *Handlers) searchDocuments(c *gin.Context) {
	queryStr := c.Query("q")
	if queryStr == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	query := SearchQuery{
		Query:     queryStr,
		Types:     []ContentType{ContentTypeDocument},
		Page:      1,
		PageSize:  20,
		Highlight: true,
	}

	applyPaginationParams(c, &query)

	result, err := h.manager.Search(&query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// searchEmails GET /api/v1/search/emails?q=keyword
// 邮件内容搜索.
func (h *Handlers) searchEmails(c *gin.Context) {
	queryStr := c.Query("q")
	if queryStr == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	query := SearchQuery{
		Query:     queryStr,
		Types:     []ContentType{ContentTypeEmail},
		Page:      1,
		PageSize:  20,
		Highlight: true,
	}

	applyPaginationParams(c, &query)

	result, err := h.manager.Search(&query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// applyPaginationParams 应用分页参数.
func applyPaginationParams(c *gin.Context, query *SearchQuery) {
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			query.Page = page
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			query.PageSize = pageSize
		}
	}
}

// splitAndTrim 按逗号分割并去除空白.
func splitAndTrim(s string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, ",") {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString 分割字符串.
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// trimSpace 去除空白.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isSpace 检查是否为空白字符.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
