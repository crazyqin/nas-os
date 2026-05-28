package smartbackupsched

import (
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 智能备份调度 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sched := r.Group("/smart-backup-sched")
	{
		// 调度配置管理
		sched.GET("/configs", h.listConfigs)
		sched.POST("/configs", h.createConfig)
		sched.GET("/configs/:id", h.getConfig)
		sched.PUT("/configs/:id", h.updateConfig)
		sched.DELETE("/configs/:id", h.deleteConfig)
		sched.POST("/configs/:id/enable", h.enableConfig)

		// 备份操作
		sched.POST("/configs/:id/run", h.runBackup)

		// 任务管理
		sched.GET("/tasks", h.listTasks)
		sched.GET("/tasks/:id", h.getTask)
		sched.DELETE("/tasks/:id", h.cancelTask)

		// AI 策略推荐
		sched.GET("/configs/:id/recommend", h.recommendStrategy)

		// 风险评估
		sched.GET("/configs/:id/risk", h.assessRisk)

		// 容量规划
		sched.GET("/configs/:id/capacity", h.forecastCapacity)

		// 变更模式
		sched.GET("/configs/:id/pattern", h.getChangePattern)

		// 调度窗口
		sched.GET("/configs/:id/window", h.checkWindow)

		// 审计日志
		sched.GET("/audit", h.getAuditLog)

		// 统计信息
		sched.GET("/stats", h.getStats)

		// 健康检查
		sched.GET("/health", h.healthCheck)

		// 清理
		sched.POST("/cleanup", h.cleanupTasks)
	}
}

// ========== 调度配置管理 ==========

// listConfigs 列出调度配置
// @Summary 列出调度配置
// @Description 获取所有智能备份调度配置
// @Tags smart-backup-sched
// @Produce json
// @Success 200 {object} api.Response{data=[]ScheduleConfig}
// @Router /smart-backup-sched/configs [get].
func (h *Handlers) listConfigs(c *gin.Context) {
	configs := h.manager.ListConfigs()
	api.OK(c, configs)
}

// getConfig 获取调度配置
// @Summary 获取调度配置详情
// @Description 获取指定调度配置
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=ScheduleConfig}
// @Failure 404 {object} api.Response
// @Router /smart-backup-sched/configs/{id} [get].
func (h *Handlers) getConfig(c *gin.Context) {
	id := c.Param("id")
	config, err := h.manager.GetConfig(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, config)
}

// createConfig 创建调度配置
// @Summary 创建调度配置
// @Description 创建新的智能备份调度配置
// @Tags smart-backup-sched
// @Accept json
// @Produce json
// @Param config body ScheduleConfig true "调度配置"
// @Success 200 {object} api.Response{data=ScheduleConfig}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /smart-backup-sched/configs [post].
func (h *Handlers) createConfig(c *gin.Context) {
	var config ScheduleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.CreateConfig(config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "调度配置创建成功", config)
}

// updateConfig 更新调度配置
// @Summary 更新调度配置
// @Description 更新指定调度配置
// @Tags smart-backup-sched
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Param config body ScheduleConfig true "调度配置"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /smart-backup-sched/configs/{id} [put].
func (h *Handlers) updateConfig(c *gin.Context) {
	id := c.Param("id")

	var config ScheduleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.UpdateConfig(id, config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "调度配置更新成功", nil)
}

// deleteConfig 删除调度配置
// @Summary 删除调度配置
// @Description 删除指定调度配置
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /smart-backup-sched/configs/{id} [delete].
func (h *Handlers) deleteConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteConfig(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "调度配置已删除", nil)
}

// enableConfig 启用/禁用调度配置
// @Summary 启用/禁用调度配置
// @Description 启用或禁用指定调度配置
// @Tags smart-backup-sched
// @Accept json
// @Produce json
// @Param id path string true "配置 ID"
// @Param request body object{enabled=bool} true "启用状态"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /smart-backup-sched/configs/{id}/enable [post].
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

// runBackup 手动触发备份
// @Summary 手动触发备份
// @Description 手动触发指定配置的备份任务
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=BackupTask}
// @Failure 500 {object} api.Response
// @Router /smart-backup-sched/configs/{id}/run [post].
func (h *Handlers) runBackup(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.RunBackup(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "备份任务已启动", task)
}

// ========== 任务管理 ==========

// listTasks 列出任务
// @Summary 列出备份任务
// @Description 获取所有备份任务
// @Tags smart-backup-sched
// @Produce json
// @Success 200 {object} api.Response{data=[]BackupTask}
// @Router /smart-backup-sched/tasks [get].
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	api.OK(c, tasks)
}

// getTask 获取任务详情
// @Summary 获取任务详情
// @Description 获取指定备份任务详情
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response{data=BackupTask}
// @Failure 404 {object} api.Response
// @Router /smart-backup-sched/tasks/{id} [get].
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
// @Description 取消指定备份任务
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /smart-backup-sched/tasks/{id} [delete].
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTask(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已取消", nil)
}

// ========== AI 策略推荐 ==========

// recommendStrategy 获取策略推荐
// @Summary 获取 AI 策略推荐
// @Description 基于数据变更模式获取备份策略推荐
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=StrategyRecommendation}
// @Router /smart-backup-sched/configs/{id}/recommend [get].
func (h *Handlers) recommendStrategy(c *gin.Context) {
	id := c.Param("id")

	// 验证配置存在
	if _, err := h.manager.GetConfig(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	rec := h.manager.RecommendStrategy(id)
	api.OK(c, rec)
}

// ========== 风险评估 ==========

// assessRisk 风险评估
// @Summary 备份风险评估
// @Description 评估指定配置的备份风险
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=RiskAssessment}
// @Failure 404 {object} api.Response
// @Router /smart-backup-sched/configs/{id}/risk [get].
func (h *Handlers) assessRisk(c *gin.Context) {
	id := c.Param("id")

	if _, err := h.manager.GetConfig(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	assessment := h.manager.AssessRisk(id)
	api.OK(c, assessment)
}

// ========== 容量规划 ==========

// forecastCapacity 容量预测
// @Summary 存储容量预测
// @Description 预测备份存储容量使用趋势
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=CapacityForecast}
// @Failure 404 {object} api.Response
// @Router /smart-backup-sched/configs/{id}/capacity [get].
func (h *Handlers) forecastCapacity(c *gin.Context) {
	id := c.Param("id")

	forecast, err := h.manager.ForecastCapacity(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, forecast)
}

// ========== 变更模式 ==========

// getChangePattern 获取变更模式
// @Summary 获取变更模式数据
// @Description 获取指定配置的数据变更模式分析
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=ChangePattern}
// @Failure 404 {object} api.Response
// @Router /smart-backup-sched/configs/{id}/pattern [get].
func (h *Handlers) getChangePattern(c *gin.Context) {
	id := c.Param("id")

	pattern, err := h.manager.GetChangePattern(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, pattern)
}

// ========== 调度窗口 ==========

// checkWindow 检查备份窗口
// @Summary 检查当前是否在备份窗口内
// @Description 检查当前时间是否在指定配置的备份窗口内
// @Tags smart-backup-sched
// @Produce json
// @Param id path string true "配置 ID"
// @Success 200 {object} api.Response{data=object{inWindow=bool,windowName=string}}
// @Failure 404 {object} api.Response
// @Router /smart-backup-sched/configs/{id}/window [get].
func (h *Handlers) checkWindow(c *gin.Context) {
	id := c.Param("id")

	if _, err := h.manager.GetConfig(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	inWindow, windowName := h.manager.IsInBackupWindow(id)
	api.OK(c, gin.H{
		"inWindow":   inWindow,
		"windowName": windowName,
	})
}

// ========== 审计日志 ==========

// getAuditLog 获取审计日志
// @Summary 获取审计日志
// @Description 获取备份调度审计日志
// @Tags smart-backup-sched
// @Produce json
// @Param configId query string false "配置 ID 过滤"
// @Param limit query int false "返回条数限制"
// @Success 200 {object} api.Response{data=[]AuditEntry}
// @Router /smart-backup-sched/audit [get].
func (h *Handlers) getAuditLog(c *gin.Context) {
	configID := c.Query("configId")
	limit := 100
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	logs := h.manager.GetAuditLog(configID, limit)
	api.OK(c, logs)
}

// ========== 统计信息 ==========

// getStats 获取统计信息
// @Summary 获取调度器统计
// @Description 获取智能备份调度器统计信息
// @Tags smart-backup-sched
// @Produce json
// @Success 200 {object} api.Response{data=SchedulerStats}
// @Router /smart-backup-sched/stats [get].
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// ========== 健康检查 ==========

// healthCheck 健康检查
// @Summary 健康检查
// @Description 检查智能备份调度器健康状态
// @Tags smart-backup-sched
// @Produce json
// @Success 200 {object} api.Response{data=HealthCheckResult}
// @Router /smart-backup-sched/health [get].
func (h *Handlers) healthCheck(c *gin.Context) {
	result := h.manager.HealthCheck()
	api.OK(c, result)
}

// ========== 清理 ==========

// cleanupTasks 清理已完成任务
// @Summary 清理已完成任务
// @Description 清理已完成、失败或已取消的任务
// @Tags smart-backup-sched
// @Produce json
// @Success 200 {object} api.Response{data=object{cleaned=int}}
// @Router /smart-backup-sched/cleanup [post].
func (h *Handlers) cleanupTasks(c *gin.Context) {
	cleaned := h.manager.CleanupCompletedTasks()
	api.OKWithMessage(c, "清理完成", gin.H{"cleaned": cleaned})
}
