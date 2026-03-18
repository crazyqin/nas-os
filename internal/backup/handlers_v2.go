// Package backup HTTP API 处理器 v2
// Version: v2.50.0 - 智能备份 API 模块
package backup

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SmartHandlers 智能备份 API 处理器
type SmartHandlers struct {
	manager   *SmartManagerV2
	scheduler *BackupScheduler
	logger    *zap.Logger
}

// NewSmartHandlers 创建智能备份处理器
func NewSmartHandlers(manager *SmartManagerV2, scheduler *BackupScheduler, logger *zap.Logger) *SmartHandlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SmartHandlers{
		manager:   manager,
		scheduler: scheduler,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由
func (h *SmartHandlers) RegisterRoutes(r *gin.RouterGroup) {
	backup := r.Group("/backup")
	{
		// ========== 备份作业管理 ==========
		// POST /api/backup - 创建备份任务
		backup.POST("", h.CreateBackupJob)
		// GET /api/backup - 列出备份任务
		backup.GET("", h.ListBackupJobs)
		// GET /api/backup/:id - 获取备份详情
		backup.GET("/:id", h.GetBackupJob)
		// DELETE /api/backup/:id - 删除备份
		backup.DELETE("/:id", h.DeleteBackupJob)
		// PUT /api/backup/:id - 更新备份配置
		backup.PUT("/:id", h.UpdateBackupJob)

		// ========== 备份执行 ==========
		// POST /api/backup/:id/run - 手动执行备份
		backup.POST("/:id/run", h.RunBackup)
		// POST /api/backup/:id/restore - 恢复备份
		backup.POST("/:id/restore", h.RestoreBackup)

		// ========== 版本管理 ==========
		backup.GET("/versions", h.ListVersions)
		backup.GET("/versions/:id", h.GetVersion)
		backup.DELETE("/versions/:id", h.DeleteVersion)

		// ========== 调度管理 ==========
		schedule := backup.Group("/schedule")
		{
			schedule.GET("", h.GetScheduleConfig)
			schedule.PUT("", h.UpdateScheduleConfig)
			schedule.POST("/jobs", h.ScheduleJob)
			schedule.DELETE("/jobs/:id", h.UnscheduleJob)
			schedule.POST("/jobs/:id/trigger", h.TriggerJob)
		}

		// ========== 统计与健康检查 ==========
		backup.GET("/stats", h.GetStats)
		backup.GET("/health", h.HealthCheck)
	}
}

// ============================================================================
// 请求/响应结构体
// ============================================================================

// CreateBackupJobRequest 创建备份任务请求
type CreateBackupJobRequest struct {
	Name        string   `json:"name" binding:"required"`
	Source      string   `json:"source" binding:"required"`
	Destination string   `json:"destination"`
	Schedule    string   `json:"schedule"`
	Priority    int      `json:"priority"`
	Retention   int      `json:"retention"`
	Enabled     bool     `json:"enabled"`
	Tags        []string `json:"tags"`
}

// RestoreBackupRequest 恢复备份请求
type RestoreBackupRequest struct {
	TargetPath string `json:"target_path" binding:"required"`
	Overwrite  bool   `json:"overwrite"`
}

// ScheduleJobRequest 调度作业请求
type ScheduleJobRequest struct {
	BackupJobID string `json:"backup_job_id" binding:"required"`
	Schedule    string `json:"schedule" binding:"required"`
	Priority    int    `json:"priority"`
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ============================================================================
// 备份作业管理 API
// ============================================================================

// CreateBackupJob 创建备份任务
func (h *SmartHandlers) CreateBackupJob(c *gin.Context) {
	var req CreateBackupJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	job := &SmartBackupJobV2{
		Name:      req.Name,
		Source:    req.Source,
		Schedule:  req.Schedule,
		Priority:  req.Priority,
		Retention: req.Retention,
		Enabled:   req.Enabled,
		Tags:      req.Tags,
	}

	if err := h.manager.CreateJob(job); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "创建备份任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "备份任务创建成功",
		Data:    job,
	})
}

// ListBackupJobs 列出备份任务
func (h *SmartHandlers) ListBackupJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    jobs,
	})
}

// GetBackupJob 获取备份详情
func (h *SmartHandlers) GetBackupJob(c *gin.Context) {
	id := c.Param("id")

	job, err := h.manager.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "备份任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    job,
	})
}

// DeleteBackupJob 删除备份
func (h *SmartHandlers) DeleteBackupJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteJob(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "删除备份任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "备份任务已删除",
	})
}

// UpdateBackupJob 更新备份配置
func (h *SmartHandlers) UpdateBackupJob(c *gin.Context) {
	id := c.Param("id")

	var req CreateBackupJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	job, err := h.manager.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "备份任务不存在",
		})
		return
	}

	// 更新字段
	job.Name = req.Name
	job.Source = req.Source
	job.Schedule = req.Schedule
	job.Priority = req.Priority
	job.Retention = req.Retention
	job.Enabled = req.Enabled
	job.Tags = req.Tags

	if err := h.manager.UpdateJob(id, job); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "更新备份任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "备份任务已更新",
		Data:    job,
	})
}

// ============================================================================
// 备份执行 API
// ============================================================================

// RunBackup 执行备份
func (h *SmartHandlers) RunBackup(c *gin.Context) {
	id := c.Param("id")

	activeJob, err := h.manager.RunBackup(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "执行备份失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "备份任务已启动",
		Data:    activeJob,
	})
}

// RestoreBackup 恢复备份
func (h *SmartHandlers) RestoreBackup(c *gin.Context) {
	backupID := c.Param("id")

	var req RestoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	activeJob, err := h.manager.RestoreBackup(backupID, req.TargetPath, req.Overwrite)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "恢复备份失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "恢复任务已启动",
		Data:    activeJob,
	})
}

// ============================================================================
// 版本管理 API
// ============================================================================

// ListVersions 列出版本
func (h *SmartHandlers) ListVersions(c *gin.Context) {
	name := c.Query("name")
	versions := h.manager.ListVersions(name)
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    versions,
	})
}

// GetVersion 获取版本详情
func (h *SmartHandlers) GetVersion(c *gin.Context) {
	id := c.Param("id")

	version, err := h.manager.GetVersion(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "备份版本不存在",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    version,
	})
}

// DeleteVersion 删除版本
func (h *SmartHandlers) DeleteVersion(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteVersion(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "删除备份版本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "备份版本已删除",
	})
}

// ============================================================================
// 调度管理 API
// ============================================================================

// GetScheduleConfig 获取调度配置
func (h *SmartHandlers) GetScheduleConfig(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "调度器未初始化",
		})
		return
	}

	config := h.scheduler.GetConfig()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    config,
	})
}

// UpdateScheduleConfig 更新调度配置
func (h *SmartHandlers) UpdateScheduleConfig(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "调度器未初始化",
		})
		return
	}

	var config SchedulerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.scheduler.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "更新调度配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "调度配置已更新",
	})
}

// ScheduleJob 调度作业
func (h *SmartHandlers) ScheduleJob(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "调度器未初始化",
		})
		return
	}

	var req ScheduleJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
		})
		return
	}

	job := &ScheduledJob{
		BackupJobID: req.BackupJobID,
		Schedule:    req.Schedule,
		Priority:    req.Priority,
		MaxRetries:  3,
		Timeout:     time.Duration(120) * time.Minute,
	}

	if err := h.scheduler.ScheduleJob(job); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "调度作业失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "作业已调度",
		Data:    job,
	})
}

// UnscheduleJob 取消调度
func (h *SmartHandlers) UnscheduleJob(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "调度器未初始化",
		})
		return
	}

	id := c.Param("id")

	if err := h.scheduler.UnscheduleJob(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "作业调度已取消",
	})
}

// TriggerJob 手动触发作业
func (h *SmartHandlers) TriggerJob(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "调度器未初始化",
		})
		return
	}

	id := c.Param("id")

	if err := h.scheduler.TriggerJob(id, 10); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "触发作业失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "作业已触发",
	})
}

// ============================================================================
// 统计与健康检查 API
// ============================================================================

// GetStats 获取统计信息
func (h *SmartHandlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()

	result := map[string]interface{}{
		"backup": stats,
	}

	if h.scheduler != nil {
		result["scheduler"] = h.scheduler.GetStats()
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// HealthCheck 健康检查
func (h *SmartHandlers) HealthCheck(c *gin.Context) {
	result, err := h.manager.HealthCheck()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "健康检查失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}
