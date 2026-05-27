// Package securityadvisor provides REST API handlers for security advisory.
package securityadvisor

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for security advisory.
type Handler struct {
	scanner *Scanner
	logger  *zap.Logger
}

// NewHandler creates a new security advisory HTTP handler.
func NewHandler(scanner *Scanner, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{scanner: scanner, logger: logger}
}

// RegisterRoutes registers security advisory API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	security := rg.Group("/security")
	{
		security.GET("/scan", h.RunScan)
		security.GET("/score", h.GetScore)
		security.GET("/recommendations", h.GetRecommendations)
		security.GET("/status", h.GetStatus)
	}
}

// RunScan handles GET /api/v1/security/scan.
func (h *Handler) RunScan(c *gin.Context) {
	report, err := h.scanner.RunFullScan(c.Request.Context())
	if err != nil {
		h.logger.Error("Security scan failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Security scan failed"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// GetScore handles GET /api/v1/security/score.
func (h *Handler) GetScore(c *gin.Context) {
	report, err := h.scanner.RunFullScan(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get security score", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get security score"})
		return
	}

	score := SecurityScore{
		Overall:    report.OverallScore,
		Level:      report.SecurityLevel,
		Summary:    generateSummary(report),
		UpdatedAt:  report.ScanTime,
	}

	// 计算各类别分数
	checks := report.Checks
	score.Password = CalculateCategoryScore(checks, "password")
	score.Port = CalculateCategoryScore(checks, "port")
	score.Permission = CalculateCategoryScore(checks, "permission")
	score.SSL = CalculateCategoryScore(checks, "ssl")
	score.Update = CalculateCategoryScore(checks, "update")
	score.Malware = CalculateCategoryScore(checks, "malware")
	score.Firewall = CalculateCategoryScore(checks, "firewall")

	c.JSON(http.StatusOK, score)
}

// GetRecommendations handles GET /api/v1/security/recommendations.
func (h *Handler) GetRecommendations(c *gin.Context) {
	report, err := h.scanner.RunFullScan(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get recommendations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recommendations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendations": report.Recommendations,
		"total":           len(report.Recommendations),
	})
}

// GetStatus handles GET /api/v1/security/status.
func (h *Handler) GetStatus(c *gin.Context) {
	report, err := h.scanner.RunFullScan(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get security status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get security status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"level":          report.SecurityLevel,
		"score":          report.OverallScore,
		"total_issues":   report.TotalIssues,
		"critical":       report.CriticalIssues,
		"warning":        report.WarningIssues,
		"info":           report.InfoIssues,
		"last_scan":      report.ScanTime,
		"scan_duration":  report.Duration.String(),
	})
}

// generateSummary 生成摘要
func generateSummary(report *SecurityReport) string {
	switch report.SecurityLevel {
	case "good":
		return "Your system security is good. No critical issues found."
	case "warning":
		return "Some security issues detected. Please review the recommendations."
	case "critical":
		return "Critical security issues found! Immediate action required."
	default:
		return "Security scan completed."
	}
}
