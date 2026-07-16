package migration

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 迁移 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	migration := r.Group("/migration")
	{
		// 任务管理
		migration.GET("/tasks", h.listTasks)
		migration.POST("/tasks", h.createTask)
		migration.GET("/tasks/:id", h.getTask)
		migration.DELETE("/tasks/:id", h.deleteTask)

		// 任务操作
		migration.POST("/tasks/:id/scan", h.scanTask)
		migration.POST("/tasks/:id/start", h.startTask)
		migration.POST("/tasks/:id/cancel", h.cancelTask)
		migration.POST("/tasks/:id/rollback", h.rollbackTask)
		migration.POST("/tasks/:id/verify", h.verifyTask)
	}
}

// createTask 创建迁移任务.
func (h *Handlers) createTask(c *gin.Context) {
	var req CreateRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	task, err := h.manager.CreateTask(&req)
	if err != nil {
		api.HandleError(c, err, "创建任务失败")
		return
	}
	api.Created(c, task)
}

// listTasks 列出所有迁移任务.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	api.OK(c, tasks)
}

// getTask 获取迁移任务详情.
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		api.HandleError(c, err, "任务不存在")
		return
	}
	api.OK(c, task)
}

// deleteTask 删除迁移任务.
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		api.HandleError(c, err, "删除任务失败")
		return
	}
	api.OKWithMessage(c, "任务已删除", nil)
}

// scanTask 预扫描评估数据量.
func (h *Handlers) scanTask(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.Scan(c.Request.Context(), id)
	if err != nil {
		api.HandleError(c, err, "扫描失败")
		return
	}
	api.OK(c, result)
}

// startTask 启动迁移任务.
func (h *Handlers) startTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Start(id); err != nil {
		api.HandleError(c, err, "启动失败")
		return
	}
	api.Accepted(c, gin.H{"message": fmt.Sprintf("迁移任务 %s 已启动", id)})
}

// cancelTask 取消迁移任务.
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Cancel(id); err != nil {
		api.HandleError(c, err, "取消失败")
		return
	}
	api.OKWithMessage(c, "任务已取消", nil)
}

// rollbackTask 回滚迁移任务.
func (h *Handlers) rollbackTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Rollback(id); err != nil {
		api.HandleError(c, err, "回滚失败")
		return
	}
	api.OKWithMessage(c, "迁移已回滚", nil)
}

// verifyTask 验证迁移数据完整性.
func (h *Handlers) verifyTask(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.Verify(c.Request.Context(), id)
	if err != nil {
		api.HandleError(c, err, "验证失败")
		return
	}
	api.OK(c, result)
}
