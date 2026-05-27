// Package containersec provides container image vulnerability scanning and security policy enforcement.
package containersec

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for container security
type Handler struct {
	scanner *Scanner
	policy  *PolicyEngine
	logger  *zap.Logger
}

// NewHandler creates a new container security HTTP handler
func NewHandler(scanner *Scanner, policy *PolicyEngine, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		scanner: scanner,
		policy:  policy,
		logger:  logger,
	}
}

// RegisterRoutes registers container security API routes
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	cs := rg.Group("/containersec")
	{
		// Scanning
		cs.POST("/scan", h.ScanImage)
		cs.GET("/scan/:image", h.GetScanResult)
		cs.DELETE("/cache", h.ClearCache)

		// Policies
		cs.GET("/policies", h.ListPolicies)
		cs.GET("/policies/:id", h.GetPolicy)
		cs.POST("/policies", h.CreatePolicy)
		cs.PUT("/policies/:id", h.UpdatePolicy)
		cs.DELETE("/policies/:id", h.DeletePolicy)

		// Reports
		cs.GET("/report/:image", h.GetImageReport)
		cs.POST("/evaluate", h.EvaluateImage)
	}
}

// ScanImage handles POST /api/v1/containersec/scan
func (h *Handler) ScanImage(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate scan ID
	scanID := generateScanID()

	// Start scan in background
	go func() {
		result, err := h.scanner.ScanImage(c.Request.Context(), req.Image, req.Registry, req.ForceRescan)
		if err != nil {
			h.logger.Error("scan failed", zap.String("image", req.Image), zap.Error(err))
			return
		}

		// Evaluate against policies
		violations := h.policy.EvaluateImage(result)
		result.PolicyViolations = violations
		result.Compliant = len(violations) == 0

		h.logger.Info("scan completed",
			zap.String("scan_id", scanID),
			zap.String("image", req.Image),
			zap.Int("violations", len(violations)))
	}()

	c.JSON(http.StatusAccepted, ScanResponse{
		ScanID: scanID,
		Image:  req.Image,
		Status: "queued",
	})
}

// GetScanResult handles GET /api/v1/containersec/scan/:image
func (h *Handler) GetScanResult(c *gin.Context) {
	image := c.Param("image")
	if image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image parameter required"})
		return
	}

	result := h.scanner.GetCachedResult(image)
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no scan result found for image"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ClearCache handles DELETE /api/v1/containersec/cache
func (h *Handler) ClearCache(c *gin.Context) {
	h.scanner.ClearCache()
	c.JSON(http.StatusOK, gin.H{"message": "cache cleared"})
}

// ListPolicies handles GET /api/v1/containersec/policies
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.policy.ListPolicies()
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// GetPolicy handles GET /api/v1/containersec/policies/:id
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy := h.policy.GetPolicy(id)
	if policy == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// CreatePolicy handles POST /api/v1/containersec/policies
func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy SecurityPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if policy.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy ID is required"})
		return
	}

	// Check if already exists
	if existing := h.policy.GetPolicy(policy.ID); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "policy already exists, use PUT to update"})
		return
	}

	h.policy.AddPolicy(policy)
	c.JSON(http.StatusCreated, policy)
}

// UpdatePolicy handles PUT /api/v1/containersec/policies/:id
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy SecurityPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy.ID = id
	h.policy.AddPolicy(policy)
	c.JSON(http.StatusOK, policy)
}

// DeletePolicy handles DELETE /api/v1/containersec/policies/:id
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if !h.policy.RemovePolicy(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}

// GetImageReport handles GET /api/v1/containersec/report/:image
func (h *Handler) GetImageReport(c *gin.Context) {
	image := c.Param("image")
	if image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image parameter required"})
		return
	}

	result := h.scanner.GetCachedResult(image)
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no scan result found for image"})
		return
	}

	approved, suggestions := h.policy.ApproveImage(result)

	report := ImageReport{
		Image:      result.Image,
		ScanResult: result,
		Approved:   approved,
		ScannedAt:  result.ScanTime,
		ExpiresAt:  result.ScanTime.Add(24 * time.Hour),
	}

	// Add policy IDs
	for _, p := range h.policy.ListPolicies() {
		if p.Enabled {
			report.Policies = append(report.Policies, p.ID)
		}
	}

	response := gin.H{
		"report":      report,
		"suggestions": suggestions,
	}

	c.JSON(http.StatusOK, response)
}

// EvaluateImage handles POST /api/v1/containersec/evaluate
func (h *Handler) EvaluateImage(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.scanner.GetCachedResult(req.Image)
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no scan result found, run scan first"})
		return
	}

	approved, suggestions := h.policy.ApproveImage(result)
	violations := h.policy.EvaluateImage(result)

	c.JSON(http.StatusOK, gin.H{
		"image":       req.Image,
		"approved":    approved,
		"violations":  violations,
		"suggestions": suggestions,
		"summary":     result.Summary,
	})
}

// generateScanID creates a unique scan ID
func generateScanID() string {
	return "scan-" + time.Now().Format("20060102-150405") + "-" + randomHex(4)
}

// randomHex generates a random hex string
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}
