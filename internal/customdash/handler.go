package customdash

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 仪表盘HTTP处理器.
type Handler struct {
	manager *DashboardManager
}

// NewHandler 创建处理器.
func NewHandler(manager *DashboardManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	dash := rg.Group("/dashboard")
	{
		dash.GET("/list", h.ListDashboards)
		dash.POST("/create", h.CreateDashboard)
		dash.PUT("/:id", h.UpdateDashboard)
		dash.DELETE("/:id", h.DeleteDashboard)
		dash.GET("/:id/widgets", h.GetWidgets)
		dash.POST("/:id/widgets", h.AddWidget)
		dash.PUT("/:id/widgets/:wid", h.UpdateWidget)
		dash.DELETE("/:id/widgets/:wid", h.DeleteWidget)
		dash.POST("/:id/export", h.ExportDashboard)
		dash.POST("/import", h.ImportDashboard)
		dash.GET("/:id/widgets/:wid/data", h.GetWidgetData)
	}
}

// ListDashboards GET /list.
func (h *Handler) ListDashboards(c *gin.Context) {
	dashboards := h.manager.ListDashboards()
	c.JSON(http.StatusOK, gin.H{
		"dashboards": dashboards,
		"total":      len(dashboards),
	})
}

// CreateDashboard POST /create.
func (h *Handler) CreateDashboard(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dash, err := h.manager.CreateDashboard(req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dash)
}

// UpdateDashboard PUT /:id.
func (h *Handler) UpdateDashboard(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dash, err := h.manager.UpdateDashboard(id, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dash)
}

// DeleteDashboard DELETE /:id.
func (h *Handler) DeleteDashboard(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDashboard(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dashboard deleted"})
}

// GetWidgets GET /:id/widgets.
func (h *Handler) GetWidgets(c *gin.Context) {
	id := c.Param("id")
	widgets, err := h.manager.GetWidgets(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"widgets": widgets,
		"total":   len(widgets),
	})
}

// AddWidget POST /:id/widgets.
func (h *Handler) AddWidget(c *gin.Context) {
	id := c.Param("id")
	var w Widget
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	widget, err := h.manager.AddWidget(id, &w)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, widget)
}

// UpdateWidget PUT /:id/widgets/:wid.
func (h *Handler) UpdateWidget(c *gin.Context) {
	id := c.Param("id")
	wid := c.Param("wid")
	var w Widget
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	widget, err := h.manager.UpdateWidget(id, wid, &w)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, widget)
}

// DeleteWidget DELETE /:id/widgets/:wid.
func (h *Handler) DeleteWidget(c *gin.Context) {
	id := c.Param("id")
	wid := c.Param("wid")
	if err := h.manager.DeleteWidget(id, wid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "widget deleted"})
}

// ExportDashboard POST /:id/export.
func (h *Handler) ExportDashboard(c *gin.Context) {
	id := c.Param("id")
	data, err := h.manager.ExportDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// ImportDashboard POST /import.
func (h *Handler) ImportDashboard(c *gin.Context) {
	var data ExportData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dash, err := h.manager.ImportDashboard(&data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dash)
}

// GetWidgetData GET /:id/widgets/:wid/data.
func (h *Handler) GetWidgetData(c *gin.Context) {
	id := c.Param("id")
	wid := c.Param("wid")
	data, err := h.manager.GetWidgetData(id, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
