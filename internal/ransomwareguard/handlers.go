// Package ransomwareguard HTTP API 处理器
package ransomwareguard

import (
	"net/http"

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
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/ransomware-guard")
	{
		group.GET("/status", h.GetStatus)
		group.POST("/enable", h.Enable)
		group.POST("/disable", h.Disable)
		group.GET("/alerts", h.ListAlerts)
		group.POST("/alerts/:id/resolve", h.ResolveAlert)
		group.GET("/honeypots", h.ListHoneypots)
		group.POST("/honeypots/deploy", h.DeployHoneypots)
		group.POST("/honeypots/reset", h.ResetHoneypots)
		group.POST("/monitored-paths", h.AddPath)
		group.DELETE("/monitored-paths", h.RemovePath)
	}
}

func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

func (h *Handler) Enable(c *gin.Context) {
	h.manager.Enable()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "防护已启用"})
}

func (h *Handler) Disable(c *gin.Context) {
	h.manager.Disable()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "防护已禁用"})
}

func (h *Handler) ListAlerts(c *gin.Context) {
	resolved := c.Query("resolved") == "true"
	alerts := h.manager.GetAlerts(resolved)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": alerts})
}

func (h *Handler) ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) ListHoneypots(c *gin.Context) {
	honeypots := h.manager.GetHoneypots()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": honeypots})
}

func (h *Handler) DeployHoneypots(c *gin.Context) {
	var req struct {
		Path  string `json:"path"`
		Count int    `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.Count <= 0 {
		req.Count = 5
	}
	deployed := h.manager.DeployHoneypots(req.Path, req.Count)
	c.JSON(http.StatusCreated, gin.H{"success": true, "deployed": deployed})
}

func (h *Handler) ResetHoneypots(c *gin.Context) {
	h.manager.ResetHoneypots()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "蜜罐已重置"})
}

func (h *Handler) AddPath(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	h.manager.AddMonitoredPath(req.Path)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) RemovePath(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	h.manager.RemoveMonitoredPath(req.Path)
	c.JSON(http.StatusOK, gin.H{"success": true})
}
