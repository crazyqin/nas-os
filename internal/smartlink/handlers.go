package smartlink

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for smartlink.
type Handler struct {
	linker *Linker
}

// NewHandler creates a new Handler.
func NewHandler(linker *Linker) *Handler {
	return &Handler{linker: linker}
}

// RegisterRoutes registers smartlink routes on a router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	smartlink := rg.Group("/smartlink")
	{
		// Link management
		smartlink.POST("/links", h.CreateLink)
		smartlink.POST("/links/batch", h.BatchCreateLinks)
		smartlink.GET("/links/:id", h.GetLink)
		smartlink.GET("/links/file/:fileId", h.ListLinksByFile)
		smartlink.DELETE("/links/:id", h.DeactivateLink)

		// Link access
		smartlink.POST("/access/:token", h.AccessLink)
		smartlink.GET("/access/:token/info", h.GetLinkByToken)

		// Statistics and logs
		smartlink.GET("/links/:id/stats", h.GetLinkStats)
		smartlink.GET("/links/:id/logs", h.GetAccessLogs)

		// Maintenance
		smartlink.POST("/cleanup", h.CleanupExpired)
	}
}

// CreateLink creates a new share link
// POST /smartlink/links.
func (h *Handler) CreateLink(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get creator ID from context (assuming auth middleware sets this)
	creatorID := c.GetString("user_id")
	if creatorID == "" {
		creatorID = "anonymous"
	}

	link, err := h.linker.CreateLink(creatorID, req)
	if err != nil {
		switch err {
		case ErrInvalidPermission:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permission"})
		case ErrPolicyViolation:
			c.JSON(http.StatusForbidden, gin.H{"error": "policy violation"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, h.toLinkResponse(link))
}

// BatchCreateLinks creates multiple share links
// POST /smartlink/links/batch.
func (h *Handler) BatchCreateLinks(c *gin.Context) {
	var req BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	creatorID := c.GetString("user_id")
	if creatorID == "" {
		creatorID = "anonymous"
	}

	response := BatchCreateResponse{
		Links: make([]LinkResponse, 0, len(req.Links)),
	}

	for _, linkReq := range req.Links {
		link, err := h.linker.CreateLink(creatorID, linkReq)
		if err != nil {
			response.Failed++
			response.Errors = append(response.Errors, err.Error())
			continue
		}
		response.Links = append(response.Links, h.toLinkResponse(link))
		response.Success++
	}

	c.JSON(http.StatusCreated, response)
}

// GetLink retrieves a share link by ID
// GET /smartlink/links/:id.
func (h *Handler) GetLink(c *gin.Context) {
	id := c.Param("id")

	link, err := h.linker.GetLink(id)
	if err != nil {
		if err == ErrLinkNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, h.toLinkResponse(link))
}

// ListLinksByFile lists all links for a file
// GET /smartlink/links/file/:fileId.
func (h *Handler) ListLinksByFile(c *gin.Context) {
	fileID := c.Param("fileId")

	links := h.linker.ListLinksByFileID(fileID)

	response := make([]LinkResponse, len(links))
	for i, link := range links {
		response[i] = h.toLinkResponse(link)
	}

	c.JSON(http.StatusOK, response)
}

// DeactivateLink deactivates a share link
// DELETE /smartlink/links/:id.
func (h *Handler) DeactivateLink(c *gin.Context) {
	id := c.Param("id")
	creatorID := c.GetString("user_id")
	if creatorID == "" {
		creatorID = "anonymous"
	}

	err := h.linker.DeactivateLink(id, creatorID)
	if err != nil {
		switch err {
		case ErrLinkNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "link deactivated"})
}

// AccessLink accesses a share link
// POST /smartlink/access/:token.
func (h *Handler) AccessLink(c *gin.Context) {
	token := c.Param("token")

	var req AccessLinkRequest
	c.ShouldBindJSON(&req)

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	link, err := h.linker.AccessLink(token, req.Password, ip, userAgent)
	if err != nil {
		switch err {
		case ErrLinkNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		case ErrLinkExpired:
			c.JSON(http.StatusGone, gin.H{"error": "link expired"})
		case ErrLinkInactive:
			c.JSON(http.StatusGone, gin.H{"error": "link inactive"})
		case ErrMaxVisitsReached:
			c.JSON(http.StatusGone, gin.H{"error": "max visits reached"})
		case ErrInvalidPassword:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, h.toLinkResponse(link))
}

// GetLinkByToken retrieves link info by token
// GET /smartlink/access/:token/info.
func (h *Handler) GetLinkByToken(c *gin.Context) {
	token := c.Param("token")

	link, err := h.linker.GetLinkByToken(token)
	if err != nil {
		if err == ErrLinkNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, h.toLinkResponse(link))
}

// GetLinkStats returns statistics for a link
// GET /smartlink/links/:id/stats.
func (h *Handler) GetLinkStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.linker.GetLinkStats(id)
	if err != nil {
		if err == ErrLinkNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetAccessLogs returns access logs for a link
// GET /smartlink/links/:id/logs.
func (h *Handler) GetAccessLogs(c *gin.Context) {
	id := c.Param("id")

	limit := 50
	offset := 0

	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	logs := h.linker.GetAccessLogs(id, limit, offset)
	c.JSON(http.StatusOK, logs)
}

// CleanupExpired removes expired links
// POST /smartlink/cleanup.
func (h *Handler) CleanupExpired(c *gin.Context) {
	removed := h.linker.CleanupExpiredLinks()
	c.JSON(http.StatusOK, gin.H{"removed": removed})
}

// toLinkResponse converts ShareLink to LinkResponse.
func (h *Handler) toLinkResponse(link *ShareLink) LinkResponse {
	return LinkResponse{
		ID:          link.ID,
		Token:       link.Token,
		FileID:      link.FileID,
		Permission:  link.Permission,
		MaxVisits:   link.MaxVisits,
		VisitCount:  link.VisitCount,
		ExpiresAt:   link.ExpiresAt,
		IsOneTime:   link.IsOneTime,
		IsActive:    link.IsActive,
		CreatedAt:   link.CreatedAt,
		Description: link.Description,
		URL:         "/s/" + link.Token,
	}
}
