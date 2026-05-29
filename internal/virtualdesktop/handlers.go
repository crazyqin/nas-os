package virtualdesktop

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers provides HTTP handlers for VirtualDesktop
type Handlers struct {
	manager *DesktopManager
	logger  *zap.Logger
}

// NewHandlers creates new VirtualDesktop handlers
func NewHandlers(manager *DesktopManager, logger *zap.Logger) *Handlers {
	return &Handlers{manager: manager, logger: logger}
}

// RegisterRoutes registers VirtualDesktop API routes
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	vdi := rg.Group("/vdi")
	{
		vdi.GET("/stats", h.GetStats)
		vdi.GET("/desktops", h.ListDesktops)
		vdi.POST("/desktops", h.CreateDesktop)
		vdi.POST("/desktops/:id/start", h.StartDesktop)
		vdi.POST("/desktops/:id/stop", h.StopDesktop)
		vdi.POST("/sessions", h.ConnectSession)
		vdi.DELETE("/sessions/:id", h.DisconnectSession)
		vdi.GET("/sessions", h.ListSessions)
	}
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.manager.GetStats()})
}

func (h *Handlers) ListDesktops(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.manager.ListDesktops()})
}

func (h *Handlers) CreateDesktop(c *gin.Context) {
	var desktop VirtualDesktop
	if err := c.ShouldBindJSON(&desktop); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.manager.CreateDesktop(&desktop); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "desktop created"})
}

func (h *Handlers) StartDesktop(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartDesktop(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "desktop started"})
}

func (h *Handlers) StopDesktop(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopDesktop(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "desktop stopped"})
}

func (h *Handlers) ConnectSession(c *gin.Context) {
	var req struct {
		DesktopID  string `json:"desktop_id"`
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
		RemoteAddr string `json:"remote_addr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	session, err := h.manager.ConnectSession(c.Request.Context(), req.DesktopID, req.UserID, req.Username, req.RemoteAddr)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": session})
}

func (h *Handlers) DisconnectSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisconnectSession(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "session disconnected"})
}

func (h *Handlers) ListSessions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.manager.ListSessions()})
}
