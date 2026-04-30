package acl

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for ACL management.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new ACL handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers ACL API routes.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	acl := rg.Group("/acl")
	{
		acl.GET("/rules", h.listRules)
		acl.POST("/rules", h.addRule)
		acl.PUT("/rules/:id", h.updateRule)
		acl.DELETE("/rules/:id", h.removeRule)
		acl.GET("/check", h.checkAccess)
		acl.GET("/effective", h.effectivePermissions)
	}
}

func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rules})
}

func (h *Handlers) addRule(c *gin.Context) {
	var rule ACLRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.AddRule(rule)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已添加"})
}

func (h *Handlers) updateRule(c *gin.Context) {
	var rule ACLRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	rule.ID = c.Param("id")
	if err := h.manager.UpdateRule(rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已更新"})
}

func (h *Handlers) removeRule(c *gin.Context) {
	id := c.Param("id")
	h.manager.RemoveRule(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已删除"})
}

func (h *Handlers) checkAccess(c *gin.Context) {
	subject := c.Query("subject")
	path := c.Query("path")
	permission := c.Query("permission")
	if subject == "" || path == "" || permission == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "missing subject, path, or permission"})
		return
	}
	allowed := h.manager.CheckAccess(subject, path, permission)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"allowed": allowed}})
}

func (h *Handlers) effectivePermissions(c *gin.Context) {
	subject := c.Query("subject")
	path := c.Query("path")
	if subject == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "missing subject or path"})
		return
	}
	perms := h.manager.GetEffectivePermissions(subject, path)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": perms})
}
