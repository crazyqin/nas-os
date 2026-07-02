package lxcmkt

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers provides HTTP handlers for the template market.
type Handlers struct {
	logger *zap.Logger
	mgr    *Manager
}

// NewHandlers creates new template market handlers.
func NewHandlers(logger *zap.Logger, mgr *Manager) *Handlers {
	return &Handlers{
		logger: logger,
		mgr:    mgr,
	}
}

// RegisterRoutes registers template market routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	templates := rg.Group("/templates")
	{
		templates.GET("", h.ListTemplates)
		templates.GET("/search", h.SearchTemplates)
		templates.GET("/stats", h.GetStats)
		templates.GET("/:id", h.GetTemplate)
		templates.POST("/:id/deploy", h.DeployContainer)
		templates.POST("/:id/rate", h.RateTemplate)
	}
}

// ListTemplates returns all templates.
func (h *Handlers) ListTemplates(c *gin.Context) {
	templates := h.mgr.GetAll()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// SearchTemplates searches templates by query parameters.
func (h *Handlers) SearchTemplates(c *gin.Context) {
	var q SearchQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := h.mgr.Search(q)
	c.JSON(http.StatusOK, results)
}

// GetTemplate returns a single template by ID.
func (h *Handlers) GetTemplate(c *gin.Context) {
	id := c.Param("id")

	t, err := h.mgr.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, t)
}

// DeployContainer deploys an LXC container from a template.
func (h *Handlers) DeployContainer(c *gin.Context) {
	templateID := c.Param("id")

	var req DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Override template ID from path
	req.TemplateID = templateID

	// Verify template exists
	t, err := h.mgr.GetByID(req.TemplateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Increment download count
	if err := h.mgr.IncrementDownloads(req.TemplateID); err != nil {
		h.logger.Error("failed to increment downloads", zap.Error(err))
	}

	// Use version from request or default to template version
	version := req.Version
	if version == "" {
		version = t.Version
	}

	// In production, this would actually create the LXC container
	// For now, we return a mock response
	response := DeployResponse{
		ContainerID:   "lxc-" + req.Name,
		ContainerName: req.Name,
		TemplateID:    req.TemplateID,
		TemplateVer:   version,
		Status:        "creating",
		CreatedAt:     t.CreatedAt,
	}

	h.logger.Info("container deployment initiated",
		zap.String("template", req.TemplateID),
		zap.String("name", req.Name),
	)

	c.JSON(http.StatusCreated, response)
}

// RateTemplate rates a template.
func (h *Handlers) RateTemplate(c *gin.Context) {
	templateID := c.Param("id")

	var req RatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Override template ID from path
	req.TemplateID = templateID

	if err := h.mgr.Rate(req.TemplateID, req.Score); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, _ := h.mgr.GetByID(req.TemplateID)
	response := RatingResponse{
		TemplateID:  req.TemplateID,
		AverageRate: t.Rating,
		TotalRates:  t.RatingCount,
	}

	c.JSON(http.StatusOK, response)
}

// GetStats returns catalog statistics.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, stats)
}

// Helper to parse pagination params.
func parsePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}
