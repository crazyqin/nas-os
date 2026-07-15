// Package agentworkflow 提供 AI 代理工作流功能
// 参考群晖 DSM Agent 2.0 的 agentic workflows，实现自然语言任务解析、
// 跨服务工作流编排、多步骤自动化、条件分支和任务状态管理
package agentworkflow

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler AI 代理工作流 API 处理器.
// 注册到 /api/v1/agentworkflow/ 路由.
type Handler struct {
	service *Service
}

// NewHandler 创建 AI 代理工作流处理器.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由到 /api/v1/agentworkflow/.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/agentworkflow")
	{
		g.POST("/parse", h.parseTask)                                       // 解析自然语言任务
		g.POST("/execute", h.executeWorkflow)                               // 执行工作流
		g.POST("/cancel", h.cancelTask)                                     // 取消任务
		g.GET("/tasks", h.listTasks)                                        // 列出任务
		g.GET("/tasks/:taskId", h.getTaskStatus)                            // 获取任务状态
		g.GET("/templates", h.getTemplates)                                 // 获取工作流模板
		g.POST("/templates/:templateId/instantiate", h.instantiateTemplate) // 从模板创建工作流
	}
}

// parseTask 解析自然语言任务.
func (h *Handler) parseTask(c *gin.Context) {
	var req ParseTaskRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.ParseTask(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "任务解析失败")
		return
	}

	api.Created(c, result)
}

// executeWorkflow 执行工作流.
func (h *Handler) executeWorkflow(c *gin.Context) {
	var req ExecuteWorkflowRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.ExecuteWorkflow(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "工作流执行失败")
		return
	}

	api.OK(c, result)
}

// cancelTask 取消任务.
func (h *Handler) cancelTask(c *gin.Context) {
	var req CancelTaskRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CancelTask(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err, "取消任务失败")
		return
	}

	api.OK(c, result)
}

// listTasks 列出所有任务.
func (h *Handler) listTasks(c *gin.Context) {
	result, err := h.service.ListTasks()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// getTaskStatus 获取任务状态.
func (h *Handler) getTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		api.BadRequest(c, "taskId 参数不能为空")
		return
	}

	result, err := h.service.GetTaskStatus(taskID)
	if err != nil {
		api.HandleError(c, err, "任务不存在")
		return
	}

	api.OK(c, result)
}

// getTemplates 获取工作流模板列表.
func (h *Handler) getTemplates(c *gin.Context) {
	result, err := h.service.GetTemplates()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// instantiateTemplate 从模板创建工作流.
func (h *Handler) instantiateTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	if templateID == "" {
		api.BadRequest(c, "templateId 参数不能为空")
		return
	}

	taskID := c.Query("taskId")
	if taskID == "" {
		api.BadRequest(c, "taskId 参数不能为空")
		return
	}

	result, err := h.service.CreateWorkflowFromTemplate(c.Request.Context(), templateID, taskID)
	if err != nil {
		api.HandleError(c, err, "从模板创建工作流失败")
		return
	}

	api.Created(c, result)
}
