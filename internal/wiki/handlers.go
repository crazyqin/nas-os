// Package wiki provides HTTP API handlers for Wiki management.
package wiki

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for Wiki management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new Wiki HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers Wiki API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	wiki := rg.Group("/wiki")
	{
		// Space management
		wiki.GET("/spaces", h.ListSpaces)
		wiki.POST("/spaces", h.CreateSpace)
		wiki.GET("/spaces/:id", h.GetSpace)
		wiki.PUT("/spaces/:id", h.UpdateSpace)
		wiki.DELETE("/spaces/:id", h.DeleteSpace)

		// Page management
		wiki.GET("/pages", h.ListPages)
		wiki.POST("/pages", h.CreatePage)
		wiki.GET("/pages/:id", h.GetPage)
		wiki.PUT("/pages/:id", h.UpdatePage)
		wiki.DELETE("/pages/:id", h.DeletePage)

		// Version history
		wiki.GET("/pages/:id/versions", h.GetPageVersions)
		wiki.POST("/pages/:id/rollback", h.RollbackPage)

		// Search
		wiki.GET("/search", h.SearchPages)

		// Comments
		wiki.POST("/pages/:id/comments", h.AddComment)

		// Export
		wiki.POST("/export", h.ExportPages)
	}
}

// ListSpaces handles GET /api/wiki/spaces.
func (h *Handler) ListSpaces(c *gin.Context) {
	spaces := h.manager.ListSpaces()
	c.JSON(http.StatusOK, gin.H{
		"spaces": spaces,
		"total":  len(spaces),
	})
}

// CreateSpace handles POST /api/wiki/spaces.
func (h *Handler) CreateSpace(c *gin.Context) {
	var req CreateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	space, err := h.manager.CreateSpace(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, space)
}

// GetSpace handles GET /api/wiki/spaces/:id.
func (h *Handler) GetSpace(c *gin.Context) {
	spaceID := c.Param("id")

	space, err := h.manager.GetSpace(spaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, space)
}

// UpdateSpace handles PUT /api/wiki/spaces/:id.
func (h *Handler) UpdateSpace(c *gin.Context) {
	spaceID := c.Param("id")

	var req UpdateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	space, err := h.manager.UpdateSpace(spaceID, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, space)
}

// DeleteSpace handles DELETE /api/wiki/spaces/:id.
func (h *Handler) DeleteSpace(c *gin.Context) {
	spaceID := c.Param("id")

	if err := h.manager.DeleteSpace(spaceID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "space deleted"})
}

// ListPages handles GET /api/wiki/pages.
func (h *Handler) ListPages(c *gin.Context) {
	spaceID := c.Query("space_id")
	if spaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "space_id is required"})
		return
	}

	tree := c.Query("tree") == "true"

	pages, err := h.manager.ListPages(spaceID, tree)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pages": pages,
		"total": len(pages),
	})
}

// CreatePage handles POST /api/wiki/pages.
func (h *Handler) CreatePage(c *gin.Context) {
	var req CreatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	page, err := h.manager.CreatePage(req, userID, userName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, page)
}

// GetPage handles GET /api/wiki/pages/:id.
func (h *Handler) GetPage(c *gin.Context) {
	pageID := c.Param("id")

	page, err := h.manager.GetPage(pageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, page)
}

// UpdatePage handles PUT /api/wiki/pages/:id.
func (h *Handler) UpdatePage(c *gin.Context) {
	pageID := c.Param("id")

	var req UpdatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	page, err := h.manager.UpdatePage(pageID, req, userID, userName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, page)
}

// DeletePage handles DELETE /api/wiki/pages/:id.
func (h *Handler) DeletePage(c *gin.Context) {
	pageID := c.Param("id")

	if err := h.manager.DeletePage(pageID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "page deleted"})
}

// GetPageVersions handles GET /api/wiki/pages/:id/versions.
func (h *Handler) GetPageVersions(c *gin.Context) {
	pageID := c.Param("id")

	versions, err := h.manager.GetPageVersions(pageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"versions": versions,
		"total":    len(versions),
	})
}

// RollbackPage handles POST /api/wiki/pages/:id/rollback.
func (h *Handler) RollbackPage(c *gin.Context) {
	pageID := c.Param("id")

	var req struct {
		Version int `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	page, err := h.manager.RollbackPage(pageID, req.Version, userID, userName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, page)
}

// SearchPages handles GET /api/wiki/search.
func (h *Handler) SearchPages(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	spaceID := c.Query("space_id")
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	results := h.manager.SearchPages(query, spaceID, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
		"query":   query,
	})
}

// AddComment handles POST /api/wiki/pages/:id/comments.
func (h *Handler) AddComment(c *gin.Context) {
	pageID := c.Param("id")

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	comment, err := h.manager.AddComment(pageID, userID, userName, req.Content, req.ParentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

// ExportPages handles POST /api/wiki/export.
func (h *Handler) ExportPages(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	content, err := h.manager.ExportPages(req.PageIDs, req.SpaceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set response headers for file download
	filename := "wiki-export.md"
	if req.Format == "html" {
		filename = "wiki-export.html"
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, content)
}
