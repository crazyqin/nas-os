// Package unifiedsearch 提供 REST API 处理器
package unifiedsearch

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 统一搜索 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	search := r.Group("/search")
	{
		// 搜索查询
		search.POST("/query", h.search)
		search.GET("/query", h.searchGet)

		// 搜索建议
		search.POST("/suggest", h.suggest)
		search.GET("/suggest", h.suggestGet)

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

		// 索引管理
		search.POST("/index/build", h.buildIndex)
		search.POST("/index/incremental", h.incrementalUpdate)
		search.POST("/index/pause", h.pauseIndex)
		search.POST("/index/resume", h.resumeIndex)
		search.POST("/index/rebuild", h.rebuildIndex)
		search.GET("/index/stats", h.getIndexStats)
		search.GET("/index/tasks", h.listTasks)
		search.GET("/index/tasks/:id", h.getTask)

		// 配置
		search.GET("/config", h.getConfig)
		search.PUT("/config", h.updateConfig)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// search POST /search/query
func (h *Handlers) search(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
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

// searchGet GET /search/query?q=...&page=1&page_size=20
func (h *Handlers) searchGet(c *gin.Context) {
	queryStr := c.Query("q")
	if queryStr == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "query parameter 'q' is required",
		})
		return
	}

	query := SearchQuery{
		Query:    queryStr,
		Page:     1,
		PageSize: 20,
	}

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

	if typesStr := c.Query("types"); typesStr != "" {
		for _, t := range splitAndTrim(typesStr) {
			ct := ContentType(t)
			if IsValidContentType(ct) {
				query.Types = append(query.Types, ct)
			}
		}
	}

	if tagsStr := c.Query("tags"); tagsStr != "" {
		query.Tags = splitAndTrim(tagsStr)
	}

	if path := c.Query("path"); path != "" {
		query.Path = path
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		so := SortOrder(sortBy)
		if IsValidSortOrder(so) {
			query.SortBy = so
		}
	}

	if boolOp := c.Query("boolean_op"); boolOp != "" {
		op := BooleanOp(boolOp)
		if IsValidBooleanOp(op) {
			query.BooleanOp = op
		}
	}

	if fuzzyStr := c.Query("fuzzy"); fuzzyStr == "true" || fuzzyStr == "1" {
		query.Fuzzy = true
	}

	if highlightStr := c.Query("highlight"); highlightStr == "true" || highlightStr == "1" {
		query.Highlight = true
	} else if highlightStr == "" {
		query.Highlight = true // 默认开启
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

// suggest POST /search/suggest
func (h *Handlers) suggest(c *gin.Context) {
	var req SuggestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	suggestions := h.manager.GetSuggestions(req.Query, req.Limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: SuggestResponse{
			Suggestions: suggestions,
		},
	})
}

// suggestGet GET /search/suggest?q=...&limit=10
func (h *Handlers) suggestGet(c *gin.Context) {
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

// getHistory GET /search/history?limit=50
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

// clearHistory DELETE /search/history
func (h *Handlers) clearHistory(c *gin.Context) {
	h.manager.ClearSearchHistory()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "history cleared",
	})
}

// getHotSearches GET /search/hot?limit=10
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

// listDocuments GET /search/documents?type=file&limit=50
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

// getDocument GET /search/documents/:id
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

// addDocument POST /search/documents
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

// updateDocument PUT /search/documents/:id
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

// deleteDocument DELETE /search/documents/:id
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

// buildIndex POST /search/index/build
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

// incrementalUpdate POST /search/index/incremental
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

// pauseIndex POST /search/index/pause
func (h *Handlers) pauseIndex(c *gin.Context) {
	if err := h.manager.PauseIndex(); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "index paused",
	})
}

// resumeIndex POST /search/index/resume
func (h *Handlers) resumeIndex(c *gin.Context) {
	if err := h.manager.ResumeIndex(); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "index resumed",
	})
}

// rebuildIndex POST /search/index/rebuild
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

// getIndexStats GET /search/index/stats
func (h *Handlers) getIndexStats(c *gin.Context) {
	stats := h.manager.GetIndexStats()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// listTasks GET /search/index/tasks
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tasks,
	})
}

// getTask GET /search/index/tasks/:id
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

// getConfig GET /search/config
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig PUT /search/config
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

// splitAndTrim 按逗号分割并去除空白
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

// splitString 分割字符串
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

// trimSpace 去除空白
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

// isSpace 检查是否为空白字符
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
