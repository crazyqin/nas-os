// Package containerguardian provides container security scanning, runtime monitoring,
// image signature verification, CIS Docker Benchmark compliance, sensitive data leak
// detection, security grading (A/B/C/D/F), auto-remediation, and security report generation.
package containerguardian

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for container guardian API
type Handler struct {
	guardian *Guardian
	logger   *zap.Logger
}

// NewHandler creates a new container guardian HTTP handler
func NewHandler(guardian *Guardian, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		guardian: guardian,
		logger:   logger,
	}
}

// RegisterRoutes registers container guardian API routes
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	guardian := rg.Group("/containerguardian")
	{
		// Scans
		guardian.POST("/scan", h.StartScan)
		guardian.GET("/scan/:id", h.GetScanResult)
		guardian.GET("/scans", h.ListScanResults)
		guardian.DELETE("/scan/:id", h.DeleteScanResult)

		// Vulnerabilities
		guardian.GET("/scan/:id/vulns", h.GetVulnerabilities)

		// Security Score
		guardian.GET("/scan/:id/score", h.GetSecurityScore)

		// Reports
		guardian.POST("/scan/:id/report", h.GenerateReport)

		// Runtime monitoring
		guardian.POST("/runtime/:id/monitor", h.MonitorContainer)

		// Resource limits
		guardian.POST("/runtime/:id/resources", h.CheckResourceLimits)

		// Audit log
		guardian.GET("/audit", h.GetAuditLog)
	}
}

// StartScan handles POST /api/v1/containerguardian/scan
func (h *Handler) StartScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.guardian.ScanImage(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, ScanResponse{
		ScanID: result.ID,
		Image:  req.Image,
		Status: ScanStatusCompleted,
	})
}

// GetScanResult handles GET /api/v1/containerguardian/scan/:id
func (h *Handler) GetScanResult(c *gin.Context) {
	id := c.Param("id")
	result := h.guardian.GetScanResult(id)
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan result not found"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListScanResults handles GET /api/v1/containerguardian/scans
func (h *Handler) ListScanResults(c *gin.Context) {
	results := h.guardian.ListScanResults()
	c.JSON(http.StatusOK, gin.H{"scans": results, "total": len(results)})
}

// DeleteScanResult handles DELETE /api/v1/containerguardian/scan/:id
func (h *Handler) DeleteScanResult(c *gin.Context) {
	id := c.Param("id")
	if !h.guardian.DeleteScanResult(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan result not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "scan result deleted"})
}

// GetVulnerabilities handles GET /api/v1/containerguardian/scan/:id/vulns
func (h *Handler) GetVulnerabilities(c *gin.Context) {
	id := c.Param("id")
	minSeverity := c.Query("min_severity")

	vulns, err := h.guardian.GetVulnerabilities(id, minSeverity)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"vulnerabilities": vulns, "total": len(vulns)})
}

// GetSecurityScore handles GET /api/v1/containerguardian/scan/:id/score
func (h *Handler) GetSecurityScore(c *gin.Context) {
	id := c.Param("id")

	score, err := h.guardian.GetSecurityScore(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, score)
}

// GenerateReport handles POST /api/v1/containerguardian/scan/:id/report
func (h *Handler) GenerateReport(c *gin.Context) {
	id := c.Param("id")

	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Format = ReportFormatJSON
	}
	if req.Format == "" {
		req.Format = ReportFormatJSON
	}

	data, err := h.guardian.GenerateReport(id, req.Format)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	contentType := "application/json"
	if req.Format == ReportFormatHTML {
		contentType = "text/html; charset=utf-8"
	}

	c.Data(http.StatusOK, contentType, data)
}

// MonitorContainer handles POST /api/v1/containerguardian/runtime/:id/monitor
func (h *Handler) MonitorContainer(c *gin.Context) {
	id := c.Param("id")

	status, err := h.guardian.MonitorContainer(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// CheckResourceLimits handles POST /api/v1/containerguardian/runtime/:id/resources
func (h *Handler) CheckResourceLimits(c *gin.Context) {
	id := c.Param("id")

	var limits ResourceLimits
	if err := c.ShouldBindJSON(&limits); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rules, err := h.guardian.CheckResourceLimits(c.Request.Context(), id, &limits)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules, "total": len(rules)})
}

// GetAuditLog handles GET /api/v1/containerguardian/audit
func (h *Handler) GetAuditLog(c *gin.Context) {
	image := c.Query("image")
	entries := h.guardian.GetAuditLog(image)
	c.JSON(http.StatusOK, gin.H{"entries": entries, "total": len(entries)})
}
