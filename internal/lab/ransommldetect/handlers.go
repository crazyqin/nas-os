package ransommldetect

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers provides HTTP handlers for ransomware ML detection.
type Handlers struct {
	detector *Detector
	logger   *zap.Logger
}

// NewHandlers creates new ransomware detection handlers.
func NewHandlers(detector *Detector, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{detector: detector, logger: logger}
}

// RegisterRoutes registers all ransomware detection API routes.
// Routes are mounted under /api/v1/ransomware (provided by the caller's RouterGroup).
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	rm := rg.Group("/ransomware")
	{
		// Read operations
		rm.GET("/status", h.getStatus)
		rm.GET("/events", h.getEvents)
		rm.GET("/alerts", h.getAlerts)
		rm.GET("/config", h.getConfig)

		// Write / action operations
		rm.PUT("/config", h.updateConfig)
		rm.POST("/record", h.recordActivity)
		rm.POST("/scan", h.triggerScan)
		rm.POST("/quarantine", h.quarantine)
	}
}

// ============================================================
// GET /api/v1/ransomware/status
// ============================================================

func (h *Handlers) getStatus(c *gin.Context) {
	status := h.detector.GetThreatStatus()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

// ============================================================
// GET /api/v1/ransomware/events
// ============================================================

func (h *Handlers) getEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	events := h.detector.GetThreatEvents(page, perPage)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"events":   events,
			"page":     page,
			"per_page": perPage,
		},
	})
}

// ============================================================
// POST /api/v1/ransomware/scan
// ============================================================

// ScanRequest is the body for POST /scan.
type ScanRequest struct {
	// Optional: list of paths to feed entropy samples for.
	// If empty, the scan uses already-recorded data.
	Paths []string `json:"paths,omitempty"`
}

// ScanResponse is the result of an on-demand scan.
type ScanResponse struct {
	Prediction Prediction   `json:"prediction"`
	Status     ThreatStatus `json:"status"`
}

func (h *Handlers) triggerScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty body is OK – just trigger an immediate scan.
		_ = err
	}

	pred := h.detector.ScanNow()
	status := h.detector.GetThreatStatus()

	h.logger.Info("手动扫描完成",
		zap.Float64("score", pred.Score),
		zap.String("threat_level", pred.ThreatLevel.String()),
	)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": ScanResponse{
			Prediction: pred,
			Status:     status,
		},
	})
}

// ============================================================
// POST /api/v1/ransomware/quarantine
// ============================================================

// QuarantineRequest specifies files/directories to quarantine.
type QuarantineRequest struct {
	Paths       []string `json:"paths" binding:"required"`
	Reason      string   `json:"reason"`
	ThreatLevel string   `json:"threat_level"`
}

// QuarantineResult describes the outcome of a quarantine operation.
type QuarantineResult struct {
	Quarantined []string `json:"quarantined"`
	Failed      []string `json:"failed,omitempty"`
	Message     string   `json:"message"`
}

func (h *Handlers) quarantine(c *gin.Context) {
	var req QuarantineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	// Delegate to IncidentResponse for actual quarantine logic.
	ir := NewIncidentResponse(h.detector, h.logger)
	result := ir.QuarantinePaths(req.Paths, req.Reason)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// ============================================================
// GET /api/v1/ransomware/alerts
// ============================================================

func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.detector.GetAlerts()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": alerts})
}

// ============================================================
// GET /api/v1/ransomware/config
// ============================================================

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.detector.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": config})
}

// ============================================================
// PUT /api/v1/ransomware/config
// ============================================================

func (h *Handlers) updateConfig(c *gin.Context) {
	var config DetectorConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.detector.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}

// ============================================================
// POST /api/v1/ransomware/record
// ============================================================

func (h *Handlers) recordActivity(c *gin.Context) {
	var activity FileActivity
	if err := c.ShouldBindJSON(&activity); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if activity.Timestamp.IsZero() {
		activity.Timestamp = time.Now()
	}
	h.detector.RecordActivity(activity)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "活动已记录"})
}
