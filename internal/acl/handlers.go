package acl

import (
	"net/http"
	"strconv"

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
		// ACL Management
		acl.GET("", h.listACLs)
		acl.POST("", h.createACL)

		// Access Control
		acl.POST("/check", h.checkAccess)
		acl.GET("/effective", h.effectivePermissions)

		// Permission Groups
		acl.GET("/groups", h.getPermissionGroups)

		// Audit
		acl.GET("/audit", h.getAuditLog)

		// Backward compatibility
		acl.GET("/rules", h.listRules)
		acl.POST("/rules", h.addRule)
		acl.PUT("/rules/:id", h.updateRule)
		acl.DELETE("/rules/:id", h.removeRule)

		// ACL Operations by path (use specific sub-paths)
		acl.POST("/ace", h.addACEByPath)
		acl.PUT("/ace", h.updateACEByPath)
		acl.DELETE("/ace", h.removeACEByPath)
		acl.POST("/propagate", h.propagateByPath)
		acl.PUT("/owner", h.setOwnerByPath)
		acl.PUT("/group", h.setGroupByPath)

		// ACL by path - use specific path patterns
		acl.GET("/path/*path", h.getACLByPath)
		acl.PUT("/path/*path", h.updateACLByPath)
		acl.DELETE("/path/*path", h.deleteACLByPath)
	}
}

// listACLs returns all ACLs.
func (h *Handlers) listACLs(c *gin.Context) {
	acls := h.manager.ListACLs()
	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  acls,
		"total": len(acls),
	})
}

// createACL creates a new ACL.
func (h *Handlers) createACL(c *gin.Context) {
	var req CreateACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	acl, err := h.manager.CreateACL(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"data":    acl,
		"message": "ACL created successfully",
	})
}

// getACLByPath returns the ACL for a path.
func (h *Handlers) getACLByPath(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		path = "/"
	}

	acl, err := h.manager.GetACL(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": acl,
	})
}

// updateACLByPath updates an existing ACL.
func (h *Handlers) updateACLByPath(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		path = "/"
	}

	var req UpdateACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	acl, err := h.manager.UpdateACL(path, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    acl,
		"message": "ACL updated successfully",
	})
}

// deleteACLByPath deletes an ACL.
func (h *Handlers) deleteACLByPath(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Path is required",
		})
		return
	}

	if err := h.manager.DeleteACL(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ACL deleted successfully",
	})
}

// addACEByPath adds an ACE to an ACL by path in query.
func (h *Handlers) addACEByPath(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	var req AddACERequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	ace, err := h.manager.AddACE(path, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"data":    ace,
		"message": "ACE added successfully",
	})
}

// updateACEByPath updates an ACE.
func (h *Handlers) updateACEByPath(c *gin.Context) {
	path := c.Query("path")
	aceID := c.Query("aceId")

	var req UpdateACERequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	ace, err := h.manager.UpdateACE(path, aceID, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    ace,
		"message": "ACE updated successfully",
	})
}

// removeACEByPath removes an ACE.
func (h *Handlers) removeACEByPath(c *gin.Context) {
	path := c.Query("path")
	aceID := c.Query("aceId")

	if err := h.manager.RemoveACE(path, aceID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ACE removed successfully",
	})
}

// checkAccess checks if a subject has a specific permission on a path.
func (h *Handlers) checkAccess(c *gin.Context) {
	var req CheckAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Try query parameters
		req = CheckAccessRequest{
			Subject:    c.Query("subject"),
			Path:       c.Query("path"),
			Permission: Permission(c.Query("permission")),
		}
		if req.Subject == "" || req.Path == "" || req.Permission == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Missing required fields: subject, path, permission",
			})
			return
		}
	}

	result := h.manager.CheckAccess(req)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": result,
	})
}

// effectivePermissions returns all effective permissions for a subject on a path.
func (h *Handlers) effectivePermissions(c *gin.Context) {
	subject := c.Query("subject")
	path := c.Query("path")
	if subject == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Missing required query parameters: subject, path",
		})
		return
	}

	result := h.manager.GetEffectivePermissions(subject, path)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": result,
	})
}

// propagateByPath propagates inheritance to child paths.
func (h *Handlers) propagateByPath(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	if err := h.manager.PropagateInheritance(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Inheritance propagated successfully",
	})
}

// setOwnerByPath sets the owner of an ACL.
func (h *Handlers) setOwnerByPath(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	var req struct {
		Owner string `json:"owner" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	if err := h.manager.SetOwner(path, req.Owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Owner set successfully",
	})
}

// setGroupByPath sets the group of an ACL.
func (h *Handlers) setGroupByPath(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	var req struct {
		Group string `json:"group" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	if err := h.manager.SetGroup(path, req.Group); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Group set successfully",
	})
}

// getPermissionGroups returns predefined permission groups.
func (h *Handlers) getPermissionGroups(c *gin.Context) {
	groups := GetPermissionGroups()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": groups,
	})
}

// getAuditLog returns the audit log.
func (h *Handlers) getAuditLog(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	entries := h.manager.GetAuditLog(limit)
	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  entries,
		"total": len(entries),
	})
}

// Backward compatibility handlers

// listRules returns all ACL rules.
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rules})
}

// addRule adds an ACL rule.
func (h *Handlers) addRule(c *gin.Context) {
	var rule ACLRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.AddRule(rule)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已添加"})
}

// updateRule updates an ACL rule.
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

// removeRule removes an ACL rule.
func (h *Handlers) removeRule(c *gin.Context) {
	id := c.Param("id")
	h.manager.RemoveRule(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已删除"})
}
