// Package containerscanner provides container image security scanning with vulnerability
// detection, compliance checking, scheduled scans, and security report generation.
package containerscanner

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for container security scanner
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new container security scanner HTTP handler
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers container security scanner API routes
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	scanner := rg.Group("/containerscanner")
	{
		// Scans
		scanner.POST("/scan", h.StartScan)
		scanner.GET("/scan/:id", h.GetScanResult)
		scanner.GET("/scans", h.ListScanResults)
		scanner.DELETE("/scan/:id", h.DeleteScanResult)

		// Vulnerabilities
		scanner.GET("/scan/:id/vulns", h.GetVulnerabilities)

		// Reports
		scanner.POST("/scan/:id/report", h.GenerateReport)

		// Schedules
		scanner.POST("/schedules", h.CreateSchedule)
		scanner.GET("/schedules", h.ListSchedules)
		scanner.GET("/schedules/:id", h.GetSchedule)
		scanner.DELETE("/schedules/:id", h.DeleteSchedule)

		// Policies
		scanner.POST("/policies", h.CreatePolicy)
		scanner.GET("/policies", h.ListPolicies)
		scanner.GET("/policies/:id", h.GetPolicy)
		scanner.DELETE("/policies/:id", h.DeletePolicy)
	}
}

// StartScan handles POST /api/v1/containerscanner/scan
func (h *Handler) StartScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.manager.ScanImage(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, ScanResponse{
		ScanID: result.ID,
		Image:  req.Image,
		Status: ScanStatusQueued,
	})
}

// GetScanResult handles GET /api/v1/containerscanner/scan/:id
func (h *Handler) GetScanResult(c *gin.Context) {
	id := c.Param("id")
	result := h.manager.GetScanResult(id)
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan result not found"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListScanResults handles GET /api/v1/containerscanner/scans
func (h *Handler) ListScanResults(c *gin.Context) {
	results := h.manager.ListScanResults()
	c.JSON(http.StatusOK, gin.H{"scans": results, "total": len(results)})
}

// DeleteScanResult handles DELETE /api/v1/containerscanner/scan/:id
func (h *Handler) DeleteScanResult(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeleteScanResult(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan result not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "scan result deleted"})
}

// GetVulnerabilities handles GET /api/v1/containerscanner/scan/:id/vulns
func (h *Handler) GetVulnerabilities(c *gin.Context) {
	id := c.Param("id")
	minSeverity := c.Query("min_severity")

	vulns, err := h.manager.GetVulnerabilities(id, minSeverity)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"vulnerabilities": vulns, "total": len(vulns)})
}

// GenerateReport handles POST /api/v1/containerscanner/scan/:id/report
func (h *Handler) GenerateReport(c *gin.Context) {
	id := c.Param("id")

	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to JSON format
		req.Format = ReportFormatJSON
	}

	data, err := h.manager.GenerateReport(id, req.Format)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

// CreateSchedule handles POST /api/v1/containerscanner/schedules
func (h *Handler) CreateSchedule(c *gin.Context) {
	var schedule ScanSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ScheduleScan(&schedule); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, schedule)
}

// ListSchedules handles GET /api/v1/containerscanner/schedules
func (h *Handler) ListSchedules(c *gin.Context) {
	schedules := h.manager.ListSchedules()
	c.JSON(http.StatusOK, gin.H{"schedules": schedules, "total": len(schedules)})
}

// GetSchedule handles GET /api/v1/containerscanner/schedules/:id
func (h *Handler) GetSchedule(c *gin.Context) {
	id := c.Param("id")
	schedule := h.manager.GetSchedule(id)
	if schedule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// DeleteSchedule handles DELETE /api/v1/containerscanner/schedules/:id
func (h *Handler) DeleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeleteSchedule(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule deleted"})
}

// CreatePolicy handles POST /api/v1/containerscanner/policies
func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy ScanPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddPolicy(&policy); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// ListPolicies handles GET /api/v1/containerscanner/policies
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies, "total": len(policies)})
}

// GetPolicy handles GET /api/v1/containerscanner/policies/:id
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy := h.manager.GetPolicy(id)
	if policy == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// DeletePolicy handles DELETE /api/v1/containerscanner/policies/:id
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeletePolicy(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}
