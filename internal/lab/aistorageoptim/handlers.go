package aistorageoptim

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler AI存储优化HTTP处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/storage/optimization")
	{
		group.GET("", h.GetOptimization)
		group.POST("", h.PostOptimization)
		group.GET("/stats", h.GetStats)
		group.GET("/scores", h.GetScores)
		group.GET("/files", h.GetAllFiles)
		group.GET("/files/:path", h.GetFileStats)
		group.POST("/record", h.RecordAccess)
		group.GET("/migrations", h.GetMigrations)
		group.GET("/policy", h.GetPolicy)
		group.POST("/policy", h.UpdatePolicy)
		group.POST("/start", h.Start)
		group.POST("/stop", h.Stop)
	}
}

// RecordAccessRequest 访问记录请求.
type RecordAccessRequest struct {
	FilePath     string `json:"filePath" binding:"required"`
	FileSize     int64  `json:"fileSize"`
	FileType     string `json:"fileType"`
	BytesRead    int64  `json:"bytesRead"`
	BytesWritten int64  `json:"bytesWritten"`
}

// OptimizationRequest 优化请求.
type OptimizationRequest struct {
	Path   string `json:"path"`
	Force  bool   `json:"force"`
	DryRun bool   `json:"dryRun"`
}

// PolicyUpdateRequest 策略更新请求.
type PolicyUpdateRequest struct {
	AccessFrequencyWeight *float64 `json:"accessFrequencyWeight"`
	FileSizeWeight        *float64 `json:"fileSizeWeight"`
	IOPatternWeight       *float64 `json:"ioPatternWeight"`
	TimeDecayWeight       *float64 `json:"timeDecayWeight"`
	NVMePromoteThreshold  *float64 `json:"nvmePromoteThreshold"`
	SSDPromoteThreshold   *float64 `json:"ssdPromoteThreshold"`
	HDDDemoteThreshold    *float64 `json:"hddDemoteThreshold"`
	AnalysisInterval      *int     `json:"analysisInterval"` // 秒
	BatchSize             *int     `json:"batchSize"`
	SmallFileThreshold    *int64   `json:"smallFileThreshold"`
	LargeFileThreshold    *int64   `json:"largeFileThreshold"`
}

// GetOptimization GET /api/v1/storage/optimization.
func (h *Handler) GetOptimization(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data": OptimizationResponse{
			Status:    "ok",
			Stats:     stats,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})
}

// PostOptimization POST /api/v1/storage/optimization.
func (h *Handler) PostOptimization(c *gin.Context) {
	var req OptimizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	decisions, stats := h.manager.AnalyzeAndOptimize(req.Path, req.DryRun)

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data": OptimizationResponse{
			Status:    "ok",
			Decisions: decisions,
			Stats:     stats,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		"count": len(decisions),
	})
}

// GetStats GET /api/v1/storage/optimization/stats.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   stats,
	})
}

// GetScores GET /api/v1/storage/optimization/scores.
func (h *Handler) GetScores(c *gin.Context) {
	scores := h.manager.GetOptimizationScores()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   scores,
		"total":  len(scores),
	})
}

// GetAllFiles GET /api/v1/storage/optimization/files.
func (h *Handler) GetAllFiles(c *gin.Context) {
	files := h.manager.GetAllFileStats()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   files,
		"total":  len(files),
	})
}

// GetFileStats GET /api/v1/storage/optimization/files/:path.
func (h *Handler) GetFileStats(c *gin.Context) {
	filePath := c.Param("path")
	stats := h.manager.GetFileStats(filePath)
	if stats == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   stats,
	})
}

// RecordAccess POST /api/v1/storage/optimization/record.
func (h *Handler) RecordAccess(c *gin.Context) {
	var req RecordAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.RecordAccess(req.FilePath, req.FileSize, req.FileType, req.BytesRead, req.BytesWritten)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "access recorded",
	})
}

// GetMigrations GET /api/v1/storage/optimization/migrations.
func (h *Handler) GetMigrations(c *gin.Context) {
	migrations := h.manager.GetMigrationHistory()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   migrations,
		"total":  len(migrations),
	})
}

// GetPolicy GET /api/v1/storage/optimization/policy.
func (h *Handler) GetPolicy(c *gin.Context) {
	policy := h.manager.GetPolicy()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   policy,
	})
}

// UpdatePolicy POST /api/v1/storage/optimization/policy.
func (h *Handler) UpdatePolicy(c *gin.Context) {
	var req PolicyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := h.manager.GetPolicy()

	if req.AccessFrequencyWeight != nil {
		policy.AccessFrequencyWeight = *req.AccessFrequencyWeight
	}
	if req.FileSizeWeight != nil {
		policy.FileSizeWeight = *req.FileSizeWeight
	}
	if req.IOPatternWeight != nil {
		policy.IOPatternWeight = *req.IOPatternWeight
	}
	if req.TimeDecayWeight != nil {
		policy.TimeDecayWeight = *req.TimeDecayWeight
	}
	if req.NVMePromoteThreshold != nil {
		policy.NVMePromoteThreshold = *req.NVMePromoteThreshold
	}
	if req.SSDPromoteThreshold != nil {
		policy.SSDPromoteThreshold = *req.SSDPromoteThreshold
	}
	if req.HDDDemoteThreshold != nil {
		policy.HDDDemoteThreshold = *req.HDDDemoteThreshold
	}
	if req.AnalysisInterval != nil {
		policy.AnalysisInterval = time.Duration(*req.AnalysisInterval) * time.Second
	}
	if req.BatchSize != nil {
		policy.BatchSize = *req.BatchSize
	}
	if req.SmallFileThreshold != nil {
		policy.SmallFileThreshold = *req.SmallFileThreshold
	}
	if req.LargeFileThreshold != nil {
		policy.LargeFileThreshold = *req.LargeFileThreshold
	}

	h.manager.UpdatePolicy(policy)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "policy updated",
	})
}

// Start POST /api/v1/storage/optimization/start.
func (h *Handler) Start(c *gin.Context) {
	h.manager.Start()
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "optimization started",
	})
}

// Stop POST /api/v1/storage/optimization/stop.
func (h *Handler) Stop(c *gin.Context) {
	h.manager.Stop()
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "optimization stopped",
	})
}
