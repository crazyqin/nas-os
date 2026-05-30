// Package photos 智能相册管理 - HTTP handlers
package photos

import (
	"net/http"
	"path/filepath"
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
		photos.POST("/upload", h.handleUpload)
		photos.POST("/upload/batch", h.handleUploadBatch)
		photos.POST("/upload/session", h.handleCreateUploadSession)
		photos.GET("", h.handleListPhotos)
		photos.GET("/search", h.handleSearch)
		photos.GET("/timeline", h.handleTimeline)
		photos.GET("/stats", h.handleStats)
		photos.PUT("/stats", h.handleUpdateStats)
		photos.GET("/persons", h.handleListPersons)
		photos.POST("/persons", h.handleCreatePerson)
		photos.DELETE("/persons/:id", h.handleDeletePerson)

		photos.GET("/albums", h.handleListAlbums)
		photos.POST("/albums", h.handleCreateAlbum)
		photos.GET("/albums/:id", h.handleGetAlbum)
		photos.DELETE("/albums/:id", h.handleDeleteAlbum)
		photos.POST("/albums/add", h.handleAddToAlbum)

		photos.GET("/:id", h.handleGetPhoto)
		photos.GET("/:id/thumbnail", h.handleGetThumbnail)
		photos.GET("/:id/download", h.handleDownloadPhoto)
		photos.POST("/:id/favorite", h.handleToggleFavorite)

		// AI routes
		ai := photos.Group("/ai")
		{
			ai.GET("/stats", h.handleAIStats)
			ai.GET("/tasks", h.handleAITasks)
			ai.GET("/smart-albums", h.handleSmartAlbums)
			ai.GET("/memories", h.handleMemories)
		}
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

func (h *Handler) handleUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file"})
		return
	}
	defer file.Close()

	// Check file format
	ext := filepath.Ext(header.Filename)
	supported := false
	for _, fmt := range h.manager.config.SupportedFormats {
		if ext == fmt {
			supported = true
			break
		}
	}
	if !supported {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) handleListPhotos(c *gin.Context) {
	userID := c.Query("userId")
	query := &PhotoQuery{
		UserID: userID,
	}
	photos, total, err := h.manager.QueryPhotos(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"photos": photos, "total": total})
}

func (h *Handler) handleGetPhoto(c *gin.Context) {
	id := c.Param("id")
	photo, err := h.manager.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
		return
	}
	c.JSON(http.StatusOK, photo)
}

func (h *Handler) handleToggleFavorite(c *gin.Context) {
	id := c.Param("id")
	photo, err := h.manager.ToggleFavorite(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
		return
	}
	c.JSON(http.StatusOK, photo)
}

func (h *Handler) handleListAlbums(c *gin.Context) {
	userID := c.Query("userId")
	var albums []*Album
	if userID != "" {
		albums = h.manager.ListAlbums(userID)
	} else {
		albums = h.manager.ListAlbums()
	}
	c.JSON(http.StatusOK, gin.H{"albums": albums})
}

func (h *Handler) handleGetAlbum(c *gin.Context) {
	id := c.Param("id")
	album, err := h.manager.GetAlbum(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
		return
	}
	c.JSON(http.StatusOK, album)
}

func (h *Handler) handleDeleteAlbum(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAlbum(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) handleListPersons(c *gin.Context) {
	persons := h.manager.ListPersons()
	c.JSON(http.StatusOK, gin.H{"persons": persons})
}

func (h *Handler) handleCreatePerson(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	person, err := h.manager.CreatePerson(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, person)
}

func (h *Handler) handleDeletePerson(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePerson(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) handleGetThumbnail(c *gin.Context) {
	id := c.Param("id")
	photo, err := h.manager.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"thumbnail": photo.Thumbnail})
}

func (h *Handler) handleDownloadPhoto(c *gin.Context) {
	id := c.Param("id")
	photo, err := h.manager.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": photo.Path})
}

func (h *Handler) handleUploadBatch(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form"})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"uploaded": len(files)})
}

func (h *Handler) handleCreateUploadSession(c *gin.Context) {
	filename := c.Query("filename")
	totalSize := c.Query("totalSize")
	if filename == "" || totalSize == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename and totalSize required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessionId": "session-123"})
}

func (h *Handler) handleUpdateStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) handleAIStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"stats": map[string]interface{}{}})
}

func (h *Handler) handleAITasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tasks": []interface{}{}})
}

func (h *Handler) handleSmartAlbums(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"albums": []interface{}{}})
}

func (h *Handler) handleMemories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"memories": []interface{}{}})
}

func (h *Handler) handleCreateAlbum(c *gin.Context) {
	name := c.Query("name")
	description := c.Query("description")
	ownerID := c.Query("userId")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	album, err := h.manager.CreateAlbum(c.Request.Context(), name, description, ownerID)
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
