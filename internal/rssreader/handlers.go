// Package rssreader 提供 REST API 处理器
package rssreader

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers RSS阅读器模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/rss 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	rss := r.Group("/rss")
	{
		// 订阅源 CRUD
		rss.POST("/feeds", h.createFeed)
		rss.GET("/feeds", h.listFeeds)
		rss.GET("/feeds/:id", h.getFeed)
		rss.PUT("/feeds/:id", h.updateFeed)
		rss.DELETE("/feeds/:id", h.deleteFeed)

		// 订阅源操作
		rss.POST("/feeds/:id/refresh", h.refreshFeed)
		rss.POST("/feeds/:id/mark-read", h.markFeedAsRead)
		rss.GET("/feeds/:id/health", h.getFeedHealth)
		rss.POST("/feeds/:id/check-health", h.checkFeedHealth)

		// 文章管理
		rss.GET("/articles", h.listArticles)
		rss.GET("/articles/:id", h.getArticle)
		rss.PUT("/articles/:id", h.updateArticle)
		rss.POST("/articles/mark-all-read", h.markAllAsRead)

		// 分类管理
		rss.POST("/categories", h.createCategory)
		rss.GET("/categories", h.listCategories)
		rss.GET("/categories/:id", h.getCategory)
		rss.PUT("/categories/:id", h.updateCategory)
		rss.DELETE("/categories/:id", h.deleteCategory)

		// 搜索
		rss.GET("/search", h.searchArticles)

		// 标签
		rss.GET("/tags/:tag", h.listFeedsByTag)

		// OPML 导入/导出
		rss.POST("/opml/import", h.importOPML)
		rss.GET("/opml/export", h.exportOPML)

		// 统计信息
		rss.GET("/stats", h.getStats)
	}
}

// ========== 订阅源 Handlers ==========

func (h *Handlers) createFeed(c *gin.Context) {
	var req CreateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	feed, err := h.manager.CreateFeed(req)
	if err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "创建成功", Data: feed})
}

func (h *Handlers) getFeed(c *gin.Context) {
	id := c.Param("id")
	feed, err := h.manager.GetFeed(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: feed})
}

func (h *Handlers) listFeeds(c *gin.Context) {
	feeds := h.manager.ListFeeds()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(feeds),
			"feeds": feeds,
		},
	})
}

func (h *Handlers) updateFeed(c *gin.Context) {
	id := c.Param("id")
	var req UpdateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	feed, err := h.manager.UpdateFeed(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "更新成功", Data: feed})
}

func (h *Handlers) deleteFeed(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteFeed(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "删除成功"})
}

func (h *Handlers) refreshFeed(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RefreshFeed(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "刷新已开始"})
}

func (h *Handlers) markFeedAsRead(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.MarkFeedAsRead(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "已全部标记为已读"})
}

func (h *Handlers) getFeedHealth(c *gin.Context) {
	id := c.Param("id")
	health, err := h.manager.GetFeedHealth(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: health})
}

func (h *Handlers) checkFeedHealth(c *gin.Context) {
	id := c.Param("id")
	health, err := h.manager.CheckFeedHealth(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "检测完成", Data: health})
}

// ========== 文章 Handlers ==========

func (h *Handlers) listArticles(c *gin.Context) {
	var req ListArticlesRequest

	// 解析查询参数
	req.FeedID = c.Query("feed_id")
	req.CategoryID = c.Query("category_id")

	// 解析布尔参数
	if isReadStr := c.Query("is_read"); isReadStr != "" {
		isRead := isReadStr == "true"
		req.IsRead = &isRead
	}
	if isFavStr := c.Query("is_favorite"); isFavStr != "" {
		isFav := isFavStr == "true"
		req.IsFavorite = &isFav
	}
	if isMarkedStr := c.Query("is_marked"); isMarkedStr != "" {
		isMarked := isMarkedStr == "true"
		req.IsMarked = &isMarked
	}

	// 解析分页参数
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			req.Page = page
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			req.PageSize = pageSize
		}
	}

	articles := h.manager.ListArticles(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(articles),
			"articles": articles,
		},
	})
}

func (h *Handlers) getArticle(c *gin.Context) {
	id := c.Param("id")
	article, err := h.manager.GetArticle(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: article})
}

func (h *Handlers) updateArticle(c *gin.Context) {
	id := c.Param("id")
	var req UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	article, err := h.manager.UpdateArticle(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "更新成功", Data: article})
}

func (h *Handlers) markAllAsRead(c *gin.Context) {
	h.manager.MarkAllAsRead()
	c.JSON(http.StatusOK, response{Code: 0, Message: "已全部标记为已读"})
}

// ========== 分类 Handlers ==========

func (h *Handlers) createCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	cat := h.manager.CreateCategory(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "创建成功", Data: cat})
}

func (h *Handlers) getCategory(c *gin.Context) {
	id := c.Param("id")
	cat, err := h.manager.GetCategory(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cat})
}

func (h *Handlers) listCategories(c *gin.Context) {
	cats := h.manager.ListCategories()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(cats),
			"categories": cats,
		},
	})
}

func (h *Handlers) updateCategory(c *gin.Context) {
	id := c.Param("id")
	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	cat, err := h.manager.UpdateCategory(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "更新成功", Data: cat})
}

func (h *Handlers) deleteCategory(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteCategory(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "删除成功"})
}

// ========== 搜索 Handlers ==========

func (h *Handlers) searchArticles(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "查询参数 'q' 不能为空"})
		return
	}

	req := SearchRequest{
		Query:  query,
		FeedID: c.Query("feed_id"),
	}

	// 解析布尔参数
	if isReadStr := c.Query("is_read"); isReadStr != "" {
		isRead := isReadStr == "true"
		req.IsRead = &isRead
	}
	if isFavStr := c.Query("is_favorite"); isFavStr != "" {
		isFav := isFavStr == "true"
		req.IsFavorite = &isFav
	}

	articles := h.manager.SearchArticles(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(articles),
			"query":    query,
			"articles": articles,
		},
	})
}

// ========== 标签 Handlers ==========

func (h *Handlers) listFeedsByTag(c *gin.Context) {
	tag := c.Param("tag")
	feeds := h.manager.ListFeedsByTag(tag)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(feeds),
			"tag":   tag,
			"feeds": feeds,
		},
	})
}

// ========== OPML Handlers ==========

func (h *Handlers) importOPML(c *gin.Context) {
	var req ImportOPMLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	feeds, err := h.manager.ImportOPML(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "导入成功",
		Data: gin.H{
			"total":          len(feeds),
			"imported_feeds": feeds,
		},
	})
}

func (h *Handlers) exportOPML(c *gin.Context) {
	content := h.manager.ExportOPML()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "导出成功",
		Data: ExportOPMLResponse{
			Content: content,
		},
	})
}

// ========== 统计 Handlers ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
