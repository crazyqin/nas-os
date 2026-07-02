// Package homelab handlers - HTTP API
package homelab

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/homelab")
	{
		g.GET("/services", h.ListServices)
		g.POST("/services", h.CreateService)
		g.GET("/services/:id", h.GetService)
		g.POST("/services/:id/start", h.StartService)
		g.POST("/services/:id/stop", h.StopService)
		g.POST("/services/:id/restart", h.RestartService)
		g.DELETE("/services/:id", h.DeleteService)

		g.GET("/stacks", h.ListStacks)
		g.POST("/stacks", h.CreateStack)
		g.GET("/stacks/:id", h.GetStack)
		g.POST("/stacks/:id/start", h.StartStack)
		g.POST("/stacks/:id/stop", h.StopStack)

		g.GET("/templates", h.ListTemplates)
		g.POST("/templates/:id/deploy", h.DeployFromTemplate)

		g.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) ListServices(c *gin.Context) {
	svcType := ServiceType(c.Query("type"))
	status := ServiceStatus(c.Query("status"))
	services := h.mgr.ListServices(svcType, status)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": services, "total": len(services)})
}

func (h *Handlers) CreateService(c *gin.Context) {
	var svc Service
	if err := c.ShouldBindJSON(&svc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateService(&svc); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": svc})
}

func (h *Handlers) GetService(c *gin.Context) {
	svc, err := h.mgr.GetService(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": svc})
}

func (h *Handlers) StartService(c *gin.Context) {
	if err := h.mgr.StartService(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "started"})
}

func (h *Handlers) StopService(c *gin.Context) {
	if err := h.mgr.StopService(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "stopped"})
}

func (h *Handlers) RestartService(c *gin.Context) {
	if err := h.mgr.RestartService(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "restarted"})
}

func (h *Handlers) DeleteService(c *gin.Context) {
	if err := h.mgr.DeleteService(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *Handlers) ListStacks(c *gin.Context) {
	stacks := h.mgr.ListStacks()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stacks, "total": len(stacks)})
}

func (h *Handlers) CreateStack(c *gin.Context) {
	var stack Stack
	if err := c.ShouldBindJSON(&stack); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateStack(&stack); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stack})
}

func (h *Handlers) GetStack(c *gin.Context) {
	stack, err := h.mgr.GetStack(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stack})
}

func (h *Handlers) StartStack(c *gin.Context) {
	if err := h.mgr.StartStack(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "started"})
}

func (h *Handlers) StopStack(c *gin.Context) {
	if err := h.mgr.StopStack(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "stopped"})
}

func (h *Handlers) ListTemplates(c *gin.Context) {
	category := c.Query("category")
	templates := h.mgr.ListTemplates(category)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": templates, "total": len(templates)})
}

func (h *Handlers) DeployFromTemplate(c *gin.Context) {
	var req struct {
		Name string            `json:"name"`
		Env  map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	svc, err := h.mgr.DeployFromTemplate(c.Param("id"), req.Name, req.Env)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": svc})
}

func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
