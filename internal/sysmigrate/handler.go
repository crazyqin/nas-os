// Package sysmigrate 提供系统迁移向导功能
// 引导用户完成 NAS 系统从源平台到当前平台的完整迁移流程
package sysmigrate

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler 系统迁移 API 处理器.
// 注册到 /api/v1/sysmigrate/ 路由.
type Handler struct {
	service *Service
}

// NewHandler 创建系统迁移处理器.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由到 /api/v1/sysmigrate/.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/sysmigrate")
	{
		g.POST("/assess", h.assess)     // 迁移评估
		g.POST("/plan", h.plan)         // 迁移计划
		g.POST("/execute", h.execute)   // 迁移执行
		g.POST("/rollback", h.rollback) // 迁移回滚
		g.GET("/status", h.status)      // 迁移状态
	}
}

// assess 迁移评估 - 检查源系统兼容性.
func (h *Handler) assess(c *gin.Context) {
	var req AssessRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.Assess(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, result)
}

// plan 生成迁移计划.
func (h *Handler) plan(c *gin.Context) {
	var req PlanRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.Plan(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "迁移计划生成失败")
		return
	}

	api.OK(c, result)
}

// execute 执行迁移.
func (h *Handler) execute(c *gin.Context) {
	var req ExecuteRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.Execute(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "迁移执行失败")
		return
	}

	api.OK(c, result)
}

// rollback 回滚迁移.
func (h *Handler) rollback(c *gin.Context) {
	var req RollbackRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.Rollback(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "迁移回滚失败")
		return
	}

	api.OK(c, result)
}

// status 查询迁移状态.
func (h *Handler) status(c *gin.Context) {
	taskID := c.Query("taskId")
	if taskID == "" {
		api.BadRequest(c, "taskId 参数不能为空")
		return
	}

	result, err := h.service.GetStatus(taskID)
	if err != nil {
		api.HandleError(c, err, "迁移任务不存在")
		return
	}

	api.OK(c, result)
}
