package backup

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
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

// listConfigs 列出备份配置
// @Summary 列出备份配置
// @Description 获取所有备份配置列表
// @Tags backup
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]JobConfig}
// @Router /backup/configs [get]
// @Security BearerAuth
func (h *Handlers) listConfigs(c *gin.Context) {
	configs := h.manager.ListConfigs()
	api.OK(c, configs)
}

// getConfig 获取备份配置
// @Summary 获取备份配置
// @Description 获取指定备份配置的详细信息
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=JobConfig}
// @Failure 404 {object} api.Response
// @Router /backup/configs/{id} [get]
// @Security BearerAuth
func (h *Handlers) getConfig(c *gin.Context) {
	id := c.Param("id")
	config, err := h.manager.GetConfig(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, config)
}

// createConfig 创建备份配置
// @Summary 创建备份配置
// @Description 创建新的备份配置
// @Tags backup
// @Accept json
// @Produce json
// @Param config body JobConfig true "备份配置"
// @Success 200 {object} api.Response{data=JobConfig}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/configs [post]
// @Security BearerAuth
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

// updateConfig 更新备份配置
// @Summary 更新备份配置
// @Description 更新指定的备份配置
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Param config body JobConfig true "备份配置"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/configs/{id} [put]
// @Security BearerAuth
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

// deleteConfig 删除备份配置
// @Summary 删除备份配置
// @Description 删除指定的备份配置
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/configs/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteConfig(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份配置已删除", nil)
}

// enableConfig 启用/禁用备份配置
// @Summary 启用/禁用备份配置
// @Description 启用或禁用指定的备份配置
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Param request body object{enabled=bool} true "启用状态"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/configs/{id}/enable [post]
// @Security BearerAuth
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

// runBackup 执行备份
// @Summary 执行备份任务
// @Description 手动执行指定的备份任务
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=Task}
// @Failure 500 {object} api.Response
// @Router /backup/run/{id} [post]
// @Security BearerAuth
func (h *Handlers) runBackup(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.RunBackup(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份任务已启动", task)
}

// restore 执行恢复
// @Summary 执行恢复操作
// @Description 从备份恢复数据
// @Tags backup
// @Accept json
// @Produce json
// @Param options body RestoreOptions true "恢复选项"
// @Success 200 {object} api.Response{data=Task}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/restore [post]
// @Security BearerAuth
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

// listTasks 列出备份任务
// @Summary 列出备份任务
// @Description 获取备份任务列表
// @Tags backup
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]Task}
// @Router /backup/tasks [get]
// @Security BearerAuth
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	api.OK(c, tasks)
}

// getTask 获取任务详情
// @Summary 获取任务详情
// @Description 获取指定备份任务的详细信息
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response{data=Task}
// @Failure 404 {object} api.Response
// @Router /backup/tasks/{id} [get]
// @Security BearerAuth
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, task)
}

// cancelTask 取消任务
// @Summary 取消备份任务
// @Description 取消正在执行的备份任务
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/tasks/{id} [delete]
// @Security BearerAuth
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTask(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已取消", nil)
}

// ========== 历史记录 ==========

// getHistory 获取历史记录
// @Summary 获取备份历史
// @Description 获取指定配置的备份历史记录
// @Tags backup
// @Accept json
// @Produce json
// @Param configId path string true "配置 ID"
// @Success 200 {object} api.Response{data=[]HistoryEntry}
// @Failure 500 {object} api.Response
// @Router /backup/history/{configId} [get]
// @Security BearerAuth
func (h *Handlers) getHistory(c *gin.Context) {
	configID := c.Param("configId")

	history, err := h.manager.GetHistory(configID)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, history)
}

// ========== 同步任务管理 ==========

// listSyncTasks 列出同步任务
// @Summary 列出同步任务
// @Description 获取所有同步任务列表
// @Tags backup
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]SyncTask}
// @Router /backup/sync/tasks [get]
// @Security BearerAuth
func (h *Handlers) listSyncTasks(c *gin.Context) {
	tasks := h.syncManager.ListSyncTasks()
	api.OK(c, tasks)
}

// getSyncTask 获取同步任务
// @Summary 获取同步任务详情
// @Description 获取指定同步任务的详细信息
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response{data=SyncTask}
// @Failure 404 {object} api.Response
// @Router /backup/sync/tasks/{id} [get]
// @Security BearerAuth
func (h *Handlers) getSyncTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.syncManager.GetSyncTask(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, task)
}

// createSyncTask 创建同步任务
// @Summary 创建同步任务
// @Description 创建新的同步任务
// @Tags backup
// @Accept json
// @Produce json
// @Param task body SyncTask true "同步任务配置"
// @Success 200 {object} api.Response{data=SyncTask}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/sync/tasks [post]
// @Security BearerAuth
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

// updateSyncTask 更新同步任务
// @Summary 更新同步任务
// @Description 更新指定的同步任务配置
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Param task body SyncTask true "同步任务配置"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/sync/tasks/{id} [put]
// @Security BearerAuth
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

// deleteSyncTask 删除同步任务
// @Summary 删除同步任务
// @Description 删除指定的同步任务
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/sync/tasks/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.syncManager.DeleteSyncTask(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务已删除", nil)
}

// runSyncTask 执行同步任务
// @Summary 执行同步任务
// @Description 手动执行指定的同步任务
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/sync/run/{id} [post]
// @Security BearerAuth
func (h *Handlers) runSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.syncManager.RunSync(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务已启动", nil)
}

// ========== 版本管理 ==========

// listVersions 列出版本
// @Summary 列出文件版本
// @Description 获取文件的版本历史列表
// @Tags backup
// @Accept json
// @Produce json
// @Param path query string true "文件路径"
// @Success 200 {object} api.Response{data=[]Version}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/sync/versions [get]
// @Security BearerAuth
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

// restoreVersion 恢复版本
// @Summary 恢复文件版本
// @Description 恢复指定的文件版本
// @Tags backup
// @Accept json
// @Produce json
// @Param request body object{versionId=string,targetPath=string} true "恢复选项"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/sync/versions/restore [post]
// @Security BearerAuth
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

// checkConfig 检查配置
// @Summary 检查备份配置
// @Description 检查备份配置的有效性
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=ConfigCheckResult}
// @Failure 404 {object} api.Response
// @Router /backup/configs/{id}/check [get]
// @Security BearerAuth
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

// checkConfigDetailed 详细检查配置
// @Summary 详细检查备份配置
// @Description 详细检查备份配置的所有方面
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=DetailedCheckResult}
// @Failure 500 {object} api.Response
// @Router /backup/configs/{id}/check-detailed [get]
// @Security BearerAuth
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

// listRestorePresets 列出恢复预设
// @Summary 列出恢复预设
// @Description 获取可用的恢复预设模板列表
// @Tags backup
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]RestorePreset}
// @Router /backup/restore-presets [get]
// @Security BearerAuth
func (h *Handlers) listRestorePresets(c *gin.Context) {
	presets := DefaultRestorePresets()
	api.OK(c, presets)
}

// previewRestore 预览恢复
// @Summary 预览恢复操作
// @Description 预览恢复操作将要执行的内容（不实际执行）
// @Tags backup
// @Accept json
// @Produce json
// @Param options body RestoreOptions true "恢复选项"
// @Success 200 {object} api.Response{data=RestorePreview}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /backup/restore/preview [post]
// @Security BearerAuth
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

// getStats 获取统计信息
// @Summary 获取备份统计
// @Description 获取备份系统的统计信息
// @Tags backup
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=BackupStats}
// @Router /backup/stats [get]
// @Security BearerAuth
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// ========== 健康检查 ==========

// healthCheck 健康检查
// @Summary 备份系统健康检查
// @Description 检查备份系统的健康状态
// @Tags backup
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=HealthCheckResult}
// @Failure 500 {object} api.Response
// @Router /backup/health [get]
// @Security BearerAuth
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
