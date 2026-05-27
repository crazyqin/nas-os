// Package filesearch 提供REST API处理器
package filesearch

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Handlers struct {
	manager *Manager
}

func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	search := r.Group("/filesearch")
	{
		search.GET("", h.Search)
		search.GET("/status", h.IndexStatus)
		search.POST("/index", h.IndexFile)
		search.DELETE("/index", h.RemoveFromIndex)
	}
}

// Search 搜索文件.
func (h *Handlers) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: "query required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := SearchRequest{
		Query:    query,
		Path:     c.Query("path"),
		Type:     FileType(c.Query("type")),
		Tags:     c.QueryArray("tags"),
		Sort:     SortBy(c.DefaultQuery("sort", "relevance")),
		Order:    SortOrder(c.DefaultQuery("order", "desc")),
		Page:     page,
		PageSize: pageSize,
	}

	if minSize := c.Query("min_size"); minSize != "" {
		if v, err := strconv.ParseInt(minSize, 10, 64); err == nil {
			req.MinSize = v
		}
	}
	if maxSize := c.Query("max_size"); maxSize != "" {
		if v, err := strconv.ParseInt(maxSize, 10, 64); err == nil {
			req.MaxSize = v
		}
	}
	if after := c.Query("after"); after != "" {
		if t, err := time.Parse("2006-01-02", after); err == nil {
			req.After = t
		}
	}
	if before := c.Query("before"); before != "" {
		if t, err := time.Parse("2006-01-02", before); err == nil {
			req.Before = t
		}
	}

	result, err := h.manager.Search(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Data: result})
}

// IndexStatus 获取索引状态.
func (h *Handlers) IndexStatus(c *gin.Context) {
	status := h.manager.IndexStatus()
	c.JSON(http.StatusOK, response{Code: 200, Data: status})
}

// IndexFile 索引文件.
func (h *Handlers) IndexFile(c *gin.Context) {
	var result SearchResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}
	h.manager.IndexFile(&result)
	c.JSON(http.StatusOK, response{Code: 200, Message: "indexed"})
}

// RemoveFromIndex 移除索引.
func (h *Handlers) RemoveFromIndex(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: "path required"})
		return
	}
	h.manager.RemoveFromIndex(path)
	c.JSON(http.StatusOK, response{Code: 200, Message: "removed"})
}
