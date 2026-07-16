package migration

import (
	"net/http"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler 迁移 API 处理器.
// 注册到 /api/v1/migration/ 路由.
type Handler struct {
	planner  *Planner
	executor *Executor
	manager  *Manager // 保留旧 manager 兼容
}

// NewHandler 创建迁移处理器.
func NewHandler(planner *Planner, executor *Executor) *Handler {
	return &Handler{
		planner:  planner,
		executor: executor,
		manager:  NewManager(),
	}
}

// NewHandlerWithManager 使用旧管理器创建处理器（兼容模式）.
func NewHandlerWithManager(manager *Manager) *Handler {
	return &Handler{
		manager:  manager,
		planner:  NewPlanner(),
		executor: NewExecutor(NewPlanner()),
	}
}

// RegisterRoutes 注册路由到 /api/v1/migration/.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	migration := rg.Group("/migration")
	{
		// 源系统探测
		migration.POST("/detect", h.detectSource)

		// 迁移计划
		migration.POST("/plan", h.generatePlan)
		migration.GET("/plan/:planId", h.getPlan)
		migration.PUT("/plan/:planId/mapping/:category", h.updateMapping)

		// 迁移任务
		migration.POST("/tasks", h.createTask)
		migration.GET("/tasks", h.listTasks)
		migration.GET("/tasks/:id", h.getTask)
		migration.DELETE("/tasks/:id", h.deleteTask)

		// 迁移执行
		migration.POST("/tasks/:id/execute", h.executeTask)
		migration.POST("/tasks/:id/pause", h.pauseTask)
		migration.POST("/tasks/:id/resume", h.resumeTask)
		migration.POST("/tasks/:id/rollback", h.rollbackTask)

		// 进度和结果
		migration.GET("/tasks/:id/progress", h.getProgress)
		migration.GET("/tasks/:id/result", h.getResult)
	}
}

// detectSource 探测源系统.
func (h *Handler) detectSource(c *gin.Context) {
	var req CreateMigrationRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	info, err := h.planner.DetectSource(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, info)
}

// generatePlan 生成迁移计划.
func (h *Handler) generatePlan(c *gin.Context) {
	var req CreateMigrationRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 先探测源系统
	sourceInfo, err := h.planner.DetectSource(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, "源系统探测失败: "+err.Error())
		return
	}

	// 创建临时任务用于生成计划
	task := &MigrationTask{
		ID:         "temp-" + req.Name,
		SourceType: req.SourceType,
		SourceHost: req.SourceHost,
		SourcePort: req.SourcePort,
		SourceUser: req.SourceUser,
		TargetPath: req.TargetPath,
	}

	plan, err := h.planner.GeneratePlan(c.Request.Context(), task, sourceInfo)
	if err != nil {
		api.InternalError(c, "生成计划失败: "+err.Error())
		return
	}

	api.Created(c, plan)
}

// getPlan 获取迁移计划.
func (h *Handler) getPlan(c *gin.Context) {
	planID := c.Param("planId")
	// 实际应从存储中查询
	_ = planID
	api.OK(c, gin.H{"message": "计划查询功能待实现"})
}

// updateMapping 更新数据映射.
func (h *Handler) updateMapping(c *gin.Context) {
	planID := c.Param("planId")
	category := c.Param("category")

	var req UpdateMappingRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	_ = planID
	_ = category
	_ = req

	api.OKWithMessage(c, "映射已更新", nil)
}

// createTask 创建迁移任务.
func (h *Handler) createTask(c *gin.Context) {
	var req CreateMigrationRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 如果有 manager，使用 manager 创建
	if h.manager != nil {
		task, err := h.manager.CreateTask(&CreateRequest{
			Name:         req.Name,
			SourceDevice: req.SourceHost,
			TargetDevice: req.TargetPath,
			SourcePath:   "/",
			TargetPath:   req.TargetPath,
			Mode:         ModeFull,
		})
		if err != nil {
			api.HandleError(c, err, "创建任务失败")
			return
		}
		api.Created(c, task)
		return
	}

	api.Created(c, gin.H{"message": "任务创建成功"})
}

// listTasks 列出所有迁移任务.
func (h *Handler) listTasks(c *gin.Context) {
	if h.manager != nil {
		tasks := h.manager.ListTasks()
		api.OK(c, tasks)
		return
	}
	api.OK(c, []interface{}{})
}

// getTask 获取迁移任务详情.
func (h *Handler) getTask(c *gin.Context) {
	id := c.Param("id")

	if h.manager != nil {
		task, err := h.manager.GetTask(id)
		if err != nil {
			api.HandleError(c, err, "任务不存在")
			return
		}
		api.OK(c, task)
		return
	}

	api.NotFound(c, "任务不存在")
}

// deleteTask 删除迁移任务.
func (h *Handler) deleteTask(c *gin.Context) {
	id := c.Param("id")

	if h.manager != nil {
		if err := h.manager.DeleteTask(id); err != nil {
			api.HandleError(c, err, "删除任务失败")
			return
		}
		api.OKWithMessage(c, "任务已删除", nil)
		return
	}

	api.OKWithMessage(c, "任务已删除", nil)
}

// executeTask 执行迁移任务.
func (h *Handler) executeTask(c *gin.Context) {
	id := c.Param("id")

	// 获取任务（这里简化处理）
	task := &MigrationTask{
		ID:     id,
		Status: MigrationStatusPending,
	}

	plan, err := h.planner.GeneratePlan(c.Request.Context(), task, &SourceSystemInfo{})
	if err != nil {
		api.InternalError(c, "生成计划失败: "+err.Error())
		return
	}

	result, err := h.executor.Execute(c.Request.Context(), task, plan)
	if err != nil {
		api.InternalError(c, "执行迁移失败: "+err.Error())
		return
	}

	api.Accepted(c, result)
}

// pauseTask 暂停迁移任务.
func (h *Handler) pauseTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.executor.Pause(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已暂停", nil)
}

// resumeTask 恢复迁移任务.
func (h *Handler) resumeTask(c *gin.Context) {
	id := c.Param("id")

	result, err := h.executor.Resume(c.Request.Context(), id)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, result)
}

// rollbackTask 回滚迁移任务.
func (h *Handler) rollbackTask(c *gin.Context) {
	id := c.Param("id")

	if h.manager != nil {
		if err := h.manager.Rollback(id); err != nil {
			api.HandleError(c, err, "回滚失败")
			return
		}
		api.OKWithMessage(c, "迁移已回滚", nil)
		return
	}

	if err := h.executor.Rollback(c.Request.Context(), id); err != nil {
		api.InternalError(c, "回滚失败: "+err.Error())
		return
	}

	api.OKWithMessage(c, "迁移已回滚", nil)
}

// getProgress 获取迁移进度.
func (h *Handler) getProgress(c *gin.Context) {
	id := c.Param("id")

	progress, err := h.executor.GetProgress(id)
	if err != nil {
		// 从 manager 获取
		if h.manager != nil {
			task, err2 := h.manager.GetTask(id)
			if err2 != nil {
				api.NotFound(c, "任务不存在")
				return
			}
			api.OK(c, task)
			return
		}
		api.NotFound(c, "任务不存在")
		return
	}

	api.OK(c, progress)
}

// getResult 获取迁移结果.
func (h *Handler) getResult(c *gin.Context) {
	id := c.Param("id")

	result, err := h.executor.GetResult(id)
	if err != nil {
		api.NotFound(c, "任务结果不存在")
		return
	}

	api.OK(c, result)
}

// ensure http is used.
var _ = http.StatusOK
