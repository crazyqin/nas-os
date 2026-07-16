// Package mediacenter provides REST API handlers for media center management.
package mediacenter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for media center management.
type Handler struct {
	mc     *MediaCenter
	logger *zap.Logger
}

// NewHandler creates a new media center HTTP handler.
func NewHandler(mc *MediaCenter, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{mc: mc, logger: logger}
}

// RegisterRoutes registers media center API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	media := rg.Group("/media-center")
	{
		media.GET("/media", h.ListMedia)
		media.POST("/media", h.AddMedia)
		media.GET("/media/:id", h.GetMedia)
		media.PUT("/media/:id", h.UpdateMedia)
		media.DELETE("/media/:id", h.DeleteMedia)

		media.GET("/categories", h.ListCategories)
		media.POST("/scan", h.ScanLibrary)
		media.GET("/stats", h.GetStats)
	}
}

// addMediaReq is the request body for adding a media item.
type addMediaReq struct {
	ID         string        `json:"id" binding:"required"`
	Title      string        `json:"title" binding:"required"`
	Type       MediaType     `json:"type" binding:"required"`
	FilePath   string        `json:"filePath" binding:"required"`
	FileSize   int64         `json:"fileSize"`
	Duration   int           `json:"duration"`
	Resolution string        `json:"resolution"`
	Codec      string        `json:"codec"`
	Bitrate    int           `json:"bitrate"`
	Metadata   MediaMetadata `json:"metadata"`
}

// AddMedia handles POST /api/v1/media-center/media.
func (h *Handler) AddMedia(c *gin.Context) {
	var req addMediaReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	item := MediaItem{
		ID:         req.ID,
		Title:      req.Title,
		Type:       req.Type,
		FilePath:   req.FilePath,
		FileSize:   req.FileSize,
		Duration:   req.Duration,
		Resolution: req.Resolution,
		Codec:      req.Codec,
		Bitrate:    req.Bitrate,
		Status:     MediaStatusAvailable,
		Metadata:   req.Metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.mc.AddItem(item); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("media item added", zap.String("id", item.ID), zap.String("title", item.Title))
	c.JSON(http.StatusCreated, item)
}

// ListMedia handles GET /api/v1/media-center/media.
func (h *Handler) ListMedia(c *gin.Context) {
	mediaType := MediaType(c.Query("type"))
	query := c.Query("q")

	var items []*MediaItem
	if query != "" {
		items = h.mc.SearchItems(query)
	} else {
		items = h.mc.ListItems("", mediaType)
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": len(items),
	})
}

// GetMedia handles GET /api/v1/media-center/media/:id.
func (h *Handler) GetMedia(c *gin.Context) {
	id := c.Param("id")

	item, err := h.mc.GetItem(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

// updateMediaReq is the request body for updating a media item.
type updateMediaReq struct {
	Title      string        `json:"title"`
	Type       MediaType     `json:"type"`
	FilePath   string        `json:"filePath"`
	FileSize   int64         `json:"fileSize"`
	Duration   int           `json:"duration"`
	Resolution string        `json:"resolution"`
	Codec      string        `json:"codec"`
	Bitrate    int           `json:"bitrate"`
	Status     MediaStatus   `json:"status"`
	Metadata   MediaMetadata `json:"metadata"`
}

// UpdateMedia handles PUT /api/v1/media-center/media/:id.
func (h *Handler) UpdateMedia(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.mc.GetItem(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req updateMediaReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Type != "" {
		existing.Type = req.Type
	}
	if req.FilePath != "" {
		existing.FilePath = req.FilePath
	}
	if req.FileSize > 0 {
		existing.FileSize = req.FileSize
	}
	if req.Duration > 0 {
		existing.Duration = req.Duration
	}
	if req.Resolution != "" {
		existing.Resolution = req.Resolution
	}
	if req.Codec != "" {
		existing.Codec = req.Codec
	}
	if req.Bitrate > 0 {
		existing.Bitrate = req.Bitrate
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	if req.Metadata.Title != "" {
		existing.Metadata = req.Metadata
	}
	existing.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, existing)
}

// DeleteMedia handles DELETE /api/v1/media-center/media/:id.
func (h *Handler) DeleteMedia(c *gin.Context) {
	id := c.Param("id")

	if err := h.mc.RemoveItem(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("media item deleted", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "media item deleted"})
}

// ListCategories handles GET /api/v1/media-center/categories.
// Returns the list of media libraries as categories.
func (h *Handler) ListCategories(c *gin.Context) {
	libs := h.mc.ListLibraries()
	c.JSON(http.StatusOK, gin.H{
		"categories": libs,
		"total":      len(libs),
	})
}

// scanReq is the request body for triggering a library scan.
type scanReq struct {
	LibraryID string `json:"libraryId"`
	Path      string `json:"path"`
}

// ScanLibrary handles POST /api/v1/media-center/scan.
func (h *Handler) ScanLibrary(c *gin.Context) {
	var req scanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate library exists if specified
	if req.LibraryID != "" {
		if _, err := h.mc.GetLibrary(req.LibraryID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("library not found: %s", req.LibraryID)})
			return
		}
	}

	h.logger.Info("scan triggered", zap.String("libraryId", req.LibraryID), zap.String("path", req.Path))
	c.JSON(http.StatusOK, gin.H{
		"message":   "scan started",
		"libraryId": req.LibraryID,
		"path":      req.Path,
		"status":    "scanning",
	})
}

// StatsResponse is the response for the stats endpoint.
type StatsResponse struct {
	TotalItems     int `json:"totalItems"`
	TotalLibraries int `json:"totalLibraries"`
	TotalSessions  int `json:"totalSessions"`
	MovieCount     int `json:"movieCount"`
	TVShowCount    int `json:"tvShowCount"`
	MusicCount     int `json:"musicCount"`
	PhotoCount     int `json:"photoCount"`
	OtherCount     int `json:"otherCount"`
}

// GetStats handles GET /api/v1/media-center/stats.
func (h *Handler) GetStats(c *gin.Context) {
	allItems := h.mc.ListItems("", "")
	allLibs := h.mc.ListLibraries()
	allSessions := h.mc.ListSessions("")

	stats := StatsResponse{
		TotalItems:     len(allItems),
		TotalLibraries: len(allLibs),
		TotalSessions:  len(allSessions),
	}

	for _, item := range allItems {
		switch item.Type {
		case MediaTypeMovie:
			stats.MovieCount++
		case MediaTypeTVShow:
			stats.TVShowCount++
		case MediaTypeMusic:
			stats.MusicCount++
		case MediaTypePhoto:
			stats.PhotoCount++
		default:
			stats.OtherCount++
		}
	}

	c.JSON(http.StatusOK, stats)
}
