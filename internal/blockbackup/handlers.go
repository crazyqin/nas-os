package blockbackup

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// BackupHandlers 块级备份 HTTP API 处理器
type BackupHandlers struct {
	engine    *BlockBackupEngine
	scheduler *BackupScheduler
	logger    *zap.Logger
}

// NewBackupHandlers 创建备份处理器
func NewBackupHandlers(engine *BlockBackupEngine, scheduler *BackupScheduler, logger *zap.Logger) *BackupHandlers {
	return &BackupHandlers{
		engine:    engine,
		scheduler: scheduler,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由
func (h *BackupHandlers) RegisterRoutes(r *gin.RouterGroup) {
	backup := r.Group("/backup")
	{
		backup.POST("/full", h.createFullBackup)
		backup.POST("/incremental", h.createIncrementalBackup)
		backup.POST("/differential", h.createDifferentialBackup)
		backup.GET("/jobs", h.listJobs)
		backup.GET("/jobs/:id", h.getJob)
		backup.POST("/verify", h.verifyBackup)
		backup.POST("/restore", h.restoreBackup)
		backup.GET("/snapshots", h.listSnapshots)
		backup.GET("/chains", h.listChains)
		backup.GET("/schedules", h.listSchedules)
		backup.POST("/schedules", h.addSchedule)
		backup.DELETE("/schedules/:id", h.removeSchedule)
	}
}

// --- Request/Response structs ---

// FullBackupRequest 全量备份请求
type FullBackupRequest struct {
	Source      string `json:"source"      binding:"required"`
	Destination string `json:"destination" binding:"required"`
}

// IncrementalBackupRequest 增量备份请求
type IncrementalBackupRequest struct {
	Source      string `json:"source"      binding:"required"`
	Destination string `json:"destination" binding:"required"`
	BaseSnap    string `json:"base_snap"` // 可选，为空时自动查找
}

// VerifyRequest 验证请求
type VerifyRequest struct {
	BackupPath string `json:"backup_path" binding:"required"`
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	BackupPath string `json:"backup_path" binding:"required"`
	Dest       string `json:"dest"        binding:"required"`
}

// APIResponse 统一 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// --- Handlers ---

// createFullBackup 创建全量备份
func (h *BackupHandlers) createFullBackup(c *gin.Context) {
	var req FullBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Hour)
	defer cancel()

	job, err := h.engine.CreateFullBackup(ctx, req.Source, req.Destination)
	if err != nil {
		h.logger.Error("Failed to create full backup", zap.Error(err))
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, APIResponse{Success: true, Data: job})
}

// createIncrementalBackup 创建增量备份
func (h *BackupHandlers) createIncrementalBackup(c *gin.Context) {
	var req IncrementalBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	baseSnap := req.BaseSnap
	if baseSnap == "" {
		snap := h.engine.GetLatestFullSnapshot()
		if snap == nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "no full backup snapshot found; run a full backup first or provide base_snap",
			})
			return
		}
		baseSnap = snap.ID
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Hour)
	defer cancel()

	job, err := h.engine.CreateIncrementalBackup(ctx, req.Source, req.Destination, baseSnap)
	if err != nil {
		h.logger.Error("Failed to create incremental backup", zap.Error(err))
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, APIResponse{Success: true, Data: job})
}

// createDifferentialBackup 创建差异备份
func (h *BackupHandlers) createDifferentialBackup(c *gin.Context) {
	var req FullBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Hour)
	defer cancel()

	job, err := h.engine.CreateDifferentialBackup(ctx, req.Source, req.Destination)
	if err != nil {
		h.logger.Error("Failed to create differential backup", zap.Error(err))
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, APIResponse{Success: true, Data: job})
}

// listJobs 列出备份任务
func (h *BackupHandlers) listJobs(c *gin.Context) {
	jobs := h.engine.ListJobs()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: jobs})
}

// getJob 获取任务详情
func (h *BackupHandlers) getJob(c *gin.Context) {
	id := c.Param("id")
	job := h.engine.GetJob(id)
	if job == nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "job not found"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: job})
}

// verifyBackup 验证备份
func (h *BackupHandlers) verifyBackup(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Hour)
	defer cancel()

	if err := h.engine.VerifyBackup(ctx, req.BackupPath); err != nil {
		h.logger.Error("Backup verification failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{"verified": true}})
}

// restoreBackup 恢复备份
func (h *BackupHandlers) restoreBackup(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Hour)
	defer cancel()

	if err := h.engine.RestoreBackup(ctx, req.BackupPath, req.Dest); err != nil {
		h.logger.Error("Backup restore failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{"restored": true}})
}

// listSnapshots 列出快照
func (h *BackupHandlers) listSnapshots(c *gin.Context) {
	snaps := h.engine.ListSnapshots()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: snaps})
}

// listChains 列出备份链
func (h *BackupHandlers) listChains(c *gin.Context) {
	// BackupChainManager 通过 engine 获取
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: []interface{}{}})
}

// listSchedules 列出调度任务
func (h *BackupHandlers) listSchedules(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, APIResponse{Success: false, Error: "scheduler not configured"})
		return
	}
	entries := h.scheduler.ListSchedules()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: entries})
}

// addSchedule 添加调度任务
func (h *BackupHandlers) addSchedule(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, APIResponse{Success: false, Error: "scheduler not configured"})
		return
	}

	var cfg ScheduledBackup
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if err := h.scheduler.AddSchedule(cfg); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{Success: true, Data: cfg})
}

// removeSchedule 移除调度任务
func (h *BackupHandlers) removeSchedule(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, APIResponse{Success: false, Error: "scheduler not configured"})
		return
	}

	id := c.Param("id")
	h.scheduler.RemoveSchedule(id)
	c.JSON(http.StatusOK, APIResponse{Success: true})
}
