// Package homewidgets provides REST API handlers for home dashboard widget management.
package homewidgets

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for home widget management.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new home widget HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes registers home widget API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	widgets := rg.Group("/home-widgets")
	{
		widgets.GET("/widgets", h.ListWidgets)
		widgets.POST("/widgets", h.CreateWidget)
		widgets.PUT("/widgets/:id", h.UpdateWidget)
		widgets.DELETE("/widgets/:id", h.DeleteWidget)

		widgets.PUT("/layout", h.UpdateLayout)

		widgets.GET("/templates", h.ListTemplates)
		widgets.POST("/templates/:id/apply", h.ApplyTemplate)
	}
}

// ListWidgets handles GET /api/v1/home-widgets/widgets.
func (h *Handler) ListWidgets(c *gin.Context) {
	widgets := h.manager.GetWidgets()
	c.JSON(http.StatusOK, gin.H{
		"widgets": widgets,
		"total":   len(widgets),
	})
}

// createWidgetReq is the request body for creating a widget.
type createWidgetReq struct {
	ID       string            `json:"id" binding:"required"`
	Type     WidgetType        `json:"type" binding:"required"`
	Title    string            `json:"title" binding:"required"`
	Position Position          `json:"position"`
	Size     WidgetSize        `json:"size"`
	Config   map[string]string `json:"config,omitempty"`
}

// CreateWidget handles POST /api/v1/home-widgets/widgets.
func (h *Handler) CreateWidget(c *gin.Context) {
	var req createWidgetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	widget := &Widget{
		ID:       req.ID,
		Type:     req.Type,
		Title:    req.Title,
		Position: req.Position,
		Size:     req.Size,
		Config:   req.Config,
	}

	if err := h.manager.AddWidget(widget); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, widget)
}

// updateWidgetReq is the request body for updating a widget.
type updateWidgetReq struct {
	Title    string            `json:"title"`
	Position Position          `json:"position"`
	Size     WidgetSize        `json:"size"`
	Config   map[string]string `json:"config,omitempty"`
	Enabled  *bool             `json:"enabled"`
}

// UpdateWidget handles PUT /api/v1/home-widgets/widgets/:id.
func (h *Handler) UpdateWidget(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.manager.GetWidget(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req updateWidgetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Position.Row != 0 || req.Position.Col != 0 {
		existing.Position = req.Position
	}
	if req.Size != "" {
		existing.Size = req.Size
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.manager.UpdateWidget(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existing)
}

// DeleteWidget handles DELETE /api/v1/home-widgets/widgets/:id.
func (h *Handler) DeleteWidget(c *gin.Context) {
	id := c.Param("id")

	if !h.manager.DeleteWidget(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "widget not found: " + id})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "widget deleted"})
}

// updateLayoutReq is the request body for updating layout.
type updateLayoutReq struct {
	Columns int      `json:"columns"`
	Widgets []Widget `json:"widgets" binding:"required"`
}

// UpdateLayout handles PUT /api/v1/home-widgets/layout.
func (h *Handler) UpdateLayout(c *gin.Context) {
	var req updateLayoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	layout := Layout{
		Columns: req.Columns,
		Widgets: req.Widgets,
	}
	if layout.Columns <= 0 {
		layout.Columns = 2
	}

	if err := h.manager.UpdateLayout(layout); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	widgets := h.manager.GetWidgets()
	c.JSON(http.StatusOK, gin.H{
		"layout": Layout{
			Columns: layout.Columns,
			Widgets: widgets,
		},
	})
}

// ListTemplates handles GET /api/v1/home-widgets/templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	templates := h.manager.GetTemplates()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// ApplyTemplate handles POST /api/v1/home-widgets/templates/:id/apply.
func (h *Handler) ApplyTemplate(c *gin.Context) {
	templateID := c.Param("id")

	layout, err := h.manager.ApplyTemplate(templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"layout":  layout,
		"message": "template applied",
	})
}
