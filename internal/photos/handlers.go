// Package photos 智能相册管理 - HTTP handlers
package photos

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	photos := api.Group("/photos")
	{
		photos.POST("/import", h.handleImport)
		photos.GET("/search", h.handleSearch)
		photos.GET("/timeline", h.handleTimeline)
		photos.GET("/stats", h.handleStats)
		photos.GET("/albums", h.handleAlbums)
		photos.POST("/albums/create", h.handleCreateAlbum)
		photos.POST("/albums/add", h.handleAddToAlbum)
	}
}

func (h *Handler) handleImport(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path"`
		UserID   string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	photo, err := h.manager.ImportPhoto(c.Request.Context(), req.FilePath, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, photo)
}

func (h *Handler) handleSearch(c *gin.Context) {
	query := SearchQuery{
		Keyword:   c.Query("keyword"),
		AlbumID:   c.Query("album_id"),
		Format:    c.Query("format"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	if page, err := strconv.Atoi(c.Query("page")); err == nil {
		query.Page = page
	}
	if pageSize, err := strconv.Atoi(c.Query("page_size")); err == nil {
		query.PageSize = pageSize
	}

	result, err := h.manager.SearchPhotos(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) handleTimeline(c *gin.Context) {
	groupBy := c.DefaultQuery("group_by", "month")

	timeline, err := h.manager.GetTimeline(c.Request.Context(), groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

func (h *Handler) handleStats(c *gin.Context) {
	stats, err := h.manager.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) handleAlbums(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"albums": []Album{}})
}

func (h *Handler) handleCreateAlbum(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     string `json:"owner_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	album, err := h.manager.CreateAlbum(c.Request.Context(), req.Name, req.Description, req.OwnerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, album)
}

func (h *Handler) handleAddToAlbum(c *gin.Context) {
	var req struct {
		PhotoID string `json:"photo_id"`
		AlbumID string `json:"album_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.manager.AddPhotoToAlbum(c.Request.Context(), req.PhotoID, req.AlbumID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
