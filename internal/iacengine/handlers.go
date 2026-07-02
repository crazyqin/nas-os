// Package iacengine provides Infrastructure as Code capabilities for managing
// NAS resources and services through declarative configuration templates.
package iacengine

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for IaC engine.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new IaC engine HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers IaC engine API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	iac := rg.Group("/iac")
	{
		// Templates
		iac.POST("/templates", h.CreateTemplate)
		iac.GET("/templates", h.ListTemplates)
		iac.GET("/templates/:id", h.GetTemplate)
		iac.DELETE("/templates/:id", h.DeleteTemplate)

		// Stacks
		iac.POST("/stacks", h.DeployStack)
		iac.GET("/stacks", h.ListStacks)
		iac.GET("/stacks/:id", h.GetStack)
		iac.DELETE("/stacks/:id", h.DestroyStack)

		// Drift
		iac.POST("/stacks/:id/drift", h.DetectDrift)
		iac.GET("/drift-reports", h.ListDriftReports)
		iac.GET("/drift-reports/:id", h.GetDriftReport)

		// Resources
		iac.GET("/stacks/:id/resources", h.ListResources)
		iac.GET("/resources/:id", h.GetResource)
	}
}

// CreateTemplate handles POST /api/v1/iac/templates.
func (h *Handler) CreateTemplate(c *gin.Context) {
	var template IaCTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ParseTemplate(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

// ListTemplates handles GET /api/v1/iac/templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	templates := h.manager.ListTemplates()
	c.JSON(http.StatusOK, gin.H{"templates": templates, "total": len(templates)})
}

// GetTemplate handles GET /api/v1/iac/templates/:id.
func (h *Handler) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	template := h.manager.GetTemplate(id)
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, template)
}

// DeleteTemplate handles DELETE /api/v1/iac/templates/:id.
func (h *Handler) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeleteTemplate(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "template deleted"})
}

// DeployStack handles POST /api/v1/iac/stacks.
func (h *Handler) DeployStack(c *gin.Context) {
	var req DeployStackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stack, err := h.manager.DeployStack(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, stack)
}

// ListStacks handles GET /api/v1/iac/stacks.
func (h *Handler) ListStacks(c *gin.Context) {
	stacks := h.manager.ListStacks()
	c.JSON(http.StatusOK, gin.H{"stacks": stacks, "total": len(stacks)})
}

// GetStack handles GET /api/v1/iac/stacks/:id.
func (h *Handler) GetStack(c *gin.Context) {
	id := c.Param("id")
	stack := h.manager.GetStack(id)
	if stack == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stack not found"})
		return
	}
	c.JSON(http.StatusOK, stack)
}

// DestroyStack handles DELETE /api/v1/iac/stacks/:id.
func (h *Handler) DestroyStack(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DestroyStack(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "stack destruction initiated"})
}

// DetectDrift handles POST /api/v1/iac/stacks/:id/drift.
func (h *Handler) DetectDrift(c *gin.Context) {
	stackID := c.Param("id")
	report, err := h.manager.DetectDrift(c.Request.Context(), stackID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// ListDriftReports handles GET /api/v1/iac/drift-reports.
func (h *Handler) ListDriftReports(c *gin.Context) {
	reports := h.manager.ListDriftReports()
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": len(reports)})
}

// GetDriftReport handles GET /api/v1/iac/drift-reports/:id.
func (h *Handler) GetDriftReport(c *gin.Context) {
	id := c.Param("id")
	report := h.manager.GetDriftReport(id)
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "drift report not found"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// ListResources handles GET /api/v1/iac/stacks/:id/resources.
func (h *Handler) ListResources(c *gin.Context) {
	stackID := c.Param("id")
	resources := h.manager.ListResources(stackID)
	c.JSON(http.StatusOK, gin.H{"resources": resources, "total": len(resources)})
}

// GetResource handles GET /api/v1/iac/resources/:id.
func (h *Handler) GetResource(c *gin.Context) {
	id := c.Param("id")
	resource := h.manager.GetResource(id)
	if resource == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}
	c.JSON(http.StatusOK, resource)
}
