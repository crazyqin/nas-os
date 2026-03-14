package backup

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// Handlers 备份 API 处理器
type Handlers struct {
	manager     *Manager
	syncManager *SyncManager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager, syncManager *SyncManager) *Handlers {
	return &Handlers{
		manager:     manager,
		syncManager: syncManager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	backup := r.Group("/backup")
	{
		// 备份配置
		backup.GET("/configs", h.listConfigs)
		backup.POST("/configs", h.createConfig)
		backup.GET("/configs/:id", h.getConfig)
		backup.PUT("/configs/:id", h.updateConfig)
		backup.DELETE("/configs/:id", h.deleteConfig)
		backup.POST("/configs/:id/enable", h.enableConfig)

		// 配置检查
		backup.GET("/configs/:id/check", h.checkConfig)
		backup.GET("/configs/:id/check-detailed", h.checkConfigDetailed)

		// 备份操作
		backup.POST("/run/:id", h.runBackup)
		backup.POST("/restore", h.restore)

		// 恢复预设
		backup.GET("/restore-presets", h.listRestorePresets)
		backup.POST("/restore/preview", h.previewRestore)

		// 任务管理
		backup.GET("/tasks", h.listTasks)
		backup.GET("/tasks/:id", h.getTask)
		backup.DELETE("/tasks/:id", h.cancelTask)

		// 历史记录
		backup.GET("/history/:configId", h.getHistory)

		// 统计信息
		backup.GET("/stats", h.getStats)

		// 健康检查
		backup.GET("/health", h.healthCheck)

		// 同步任务
		sync := backup.Group("/sync")
		{
			sync.GET("/tasks", h.listSyncTasks)
			sync.POST("/tasks", h.createSyncTask)
			sync.GET("/tasks/:id", h.getSyncTask)
			sync.PUT("/tasks/:id", h.updateSyncTask)
			sync.DELETE("/tasks/:id", h.deleteSyncTask)
			sync.POST("/run/:id", h.runSyncTask)

			// 版本管理
			sync.GET("/versions", h.listVersions)
			sync.POST("/versions/restore", h.restoreVersion)
		}
	}
}

// ========== 配置管理 ==========

func (h *Handlers) listConfigs(c *gin.Context) {
	configs := h.manager.ListConfigs()
	api.OK(c, configs)
}

func (h *Handlers) getConfig(c *gin.Context) {
	id := c.Param("id")
	config, err := h.manager.GetConfig(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, config)
}

func (h *Handlers) createConfig(c *gin.Context) {
	var config JobConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.CreateConfig(config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份配置创建成功", config)
}

func (h *Handlers) updateConfig(c *gin.Context) {
	id := c.Param("id")

	var config JobConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.UpdateConfig(id, config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份配置更新成功", nil)
}

func (h *Handlers) deleteConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteConfig(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份配置已删除", nil)
}

func (h *Handlers) enableConfig(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.EnableConfig(id, req.Enabled); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "状态已更新", nil)
}

// ========== 备份操作 ==========

func (h *Handlers) runBackup(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.RunBackup(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份任务已启动", task)
}

func (h *Handlers) restore(c *gin.Context) {
	var options RestoreOptions
	if err := c.ShouldBindJSON(&options); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	task, err := h.manager.Restore(options)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "恢复任务已启动", task)
}

// ========== 任务管理 ==========

func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	api.OK(c, tasks)
}

func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, task)
}

func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTask(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已取消", nil)
}

// ========== 历史记录 ==========

func (h *Handlers) getHistory(c *gin.Context) {
	configId := c.Param("configId")

	history, err := h.manager.GetHistory(configId)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, history)
}

// ========== 同步任务管理 ==========

func (h *Handlers) listSyncTasks(c *gin.Context) {
	tasks := h.syncManager.ListSyncTasks()
	api.OK(c, tasks)
}

func (h *Handlers) getSyncTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.syncManager.GetSyncTask(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, task)
}

func (h *Handlers) createSyncTask(c *gin.Context) {
	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.syncManager.CreateSyncTask(task); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务创建成功", task)
}

func (h *Handlers) updateSyncTask(c *gin.Context) {
	id := c.Param("id")

	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 先获取现有任务，保留 ID
	existingTask, err := h.syncManager.GetSyncTask(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	task.ID = id
	task.Status = existingTask.Status
	task.LastSync = existingTask.LastSync

	if err := h.syncManager.CreateSyncTask(task); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务更新成功", nil)
}

func (h *Handlers) deleteSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.syncManager.DeleteSyncTask(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务已删除", nil)
}

func (h *Handlers) runSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.syncManager.RunSync(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务已启动", nil)
}

// ========== 版本管理 ==========

func (h *Handlers) listVersions(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		api.BadRequest(c, "文件路径不能为空")
		return
	}

	versions, err := h.syncManager.versionManager.ListVersions(filePath)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, versions)
}

func (h *Handlers) restoreVersion(c *gin.Context) {
	var req struct {
		VersionID  string `json:"versionId"`
		TargetPath string `json:"targetPath"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.syncManager.versionManager.RestoreVersion(req.VersionID, req.TargetPath); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "版本恢复成功", nil)
}

// ========== 配置检查 ==========

func (h *Handlers) checkConfig(c *gin.Context) {
	id := c.Param("id")

	// 获取配置并检查云端连接
	config, err := h.manager.GetConfig(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	result := &ConfigCheckResult{
		ConfigID: id,
		Status:   "pass",
		Checks:   []CheckItem{},
	}

	// 检查云端连接（如果启用）
	if config.CloudBackup && config.CloudConfig != nil {
		cloud, err := NewCloudBackup(*config.CloudConfig)
		if err != nil {
			result.Checks = append(result.Checks, CheckItem{
				Name:    "cloud_connection",
				Status:  "fail",
				Message: fmt.Sprintf("初始化云端客户端失败：%v", err),
			})
			result.Status = "fail"
		} else {
			connResult, err := cloud.CheckConnection()
			if err != nil || !connResult.Success {
				msg := "连接失败"
				if err != nil {
					msg = err.Error()
				} else if connResult != nil {
					msg = connResult.Message
				}
				result.Checks = append(result.Checks, CheckItem{
					Name:    "cloud_connection",
					Status:  "fail",
					Message: msg,
				})
				result.Status = "fail"
			} else {
				result.Checks = append(result.Checks, CheckItem{
					Name:    "cloud_connection",
					Status:  "pass",
					Message: fmt.Sprintf("连接成功，延迟：%dms", connResult.LatencyMs),
				})
			}
		}
	} else {
		result.Checks = append(result.Checks, CheckItem{
			Name:    "cloud_connection",
			Status:  "skip",
			Message: "未启用云端备份",
		})
	}

	api.OK(c, result)
}

func (h *Handlers) checkConfigDetailed(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.CheckConfigDetailed(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// ========== 恢复功能 ==========

func (h *Handlers) listRestorePresets(c *gin.Context) {
	presets := DefaultRestorePresets()
	api.OK(c, presets)
}

func (h *Handlers) previewRestore(c *gin.Context) {
	var options RestoreOptions
	if err := c.ShouldBindJSON(&options); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 预览恢复操作（不实际执行）
	preview, err := h.manager.PreviewRestore(options)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, preview)
}

// ========== 统计信息 ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// ========== 健康检查 ==========

func (h *Handlers) healthCheck(c *gin.Context) {
	result, err := h.manager.HealthCheck()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OK(c, result)
}

// ========== 辅助类型 ==========

// ConfigCheckResult 配置检查结果
type ConfigCheckResult struct {
	ConfigID string      `json:"configId"`
	Status   string      `json:"status"` // pass, warn, fail
	Checks   []CheckItem `json:"checks"`
}

// CheckItem 检查项
type CheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, warn, fail, skip
	Message string `json:"message"`
}
