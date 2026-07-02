// Package workflow 智能工作流引擎 - HTTP API
package workflow

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/workflow")
	{
		// 工作流CRUD
		g.GET("", h.ListWorkflows)
		g.POST("", h.CreateWorkflow)
		g.GET("/:id", h.GetWorkflow)
		g.PUT("/:id", h.UpdateWorkflow)
		g.DELETE("/:id", h.DeleteWorkflow)
		g.POST("/:id/enable", h.EnableWorkflow)
		g.POST("/:id/disable", h.DisableWorkflow)

		// 执行
		g.POST("/:id/execute", h.ExecuteWorkflow)
		g.GET("/:id/executions", h.ListExecutions)

		// 版本
		g.GET("/:id/versions", h.GetVersions)
		g.POST("/:id/rollback/:version", h.RollbackVersion)

		// 执行记录
		g.GET("/executions/:execId", h.GetExecution)

		// 模板
		g.GET("/templates", h.ListTemplates)
		g.POST("/templates/:id/create", h.CreateFromTemplate)

		// 统计
		g.GET("/stats", h.GetStats)
	}
}

// ListWorkflows 列出工作流.
func (h *Handlers) ListWorkflows(c *gin.Context) {
	status := WorkflowStatus(c.Query("status"))
	tags := c.QueryArray("tags")
	workflows := h.mgr.ListWorkflows(status, tags)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": workflows, "total": len(workflows)})
}

// CreateWorkflow 创建工作流.
func (h *Handlers) CreateWorkflow(c *gin.Context) {
	var wf Workflow
	if err := c.ShouldBindJSON(&wf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateWorkflow(&wf); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wf})
}

// GetWorkflow 获取工作流.
func (h *Handlers) GetWorkflow(c *gin.Context) {
	wf, err := h.mgr.GetWorkflow(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wf})
}

// UpdateWorkflow 更新工作流.
func (h *Handlers) UpdateWorkflow(c *gin.Context) {
	var wf Workflow
	if err := c.ShouldBindJSON(&wf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	wf.ID = c.Param("id")
	if err := h.mgr.UpdateWorkflow(&wf); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wf})
}

// DeleteWorkflow 删除工作流.
func (h *Handlers) DeleteWorkflow(c *gin.Context) {
	if err := h.mgr.DeleteWorkflow(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// EnableWorkflow 启用工作流.
func (h *Handlers) EnableWorkflow(c *gin.Context) {
	if err := h.mgr.EnableWorkflow(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "enabled"})
}

// DisableWorkflow 禁用工作流.
func (h *Handlers) DisableWorkflow(c *gin.Context) {
	if err := h.mgr.DisableWorkflow(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "disabled"})
}

// ExecuteWorkflow 执行工作流.
func (h *Handlers) ExecuteWorkflow(c *gin.Context) {
	var req struct {
		TriggerType TriggerType `json:"trigger_type"`
		Input       string      `json:"input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认手动触发
		req.TriggerType = TriggerManual
	}
	if req.TriggerType == "" {
		req.TriggerType = TriggerManual
	}

	exec, err := h.mgr.ExecuteWorkflow(c.Param("id"), req.TriggerType, req.Input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": exec})
}

// ListExecutions 列出执行记录.
func (h *Handlers) ListExecutions(c *gin.Context) {
	status := ExecutionStatus(c.Query("status"))
	limit := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	executions := h.mgr.ListExecutions(c.Param("id"), status, limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": executions, "total": len(executions)})
}

// GetExecution 获取执行记录.
func (h *Handlers) GetExecution(c *gin.Context) {
	exec, err := h.mgr.GetExecution(c.Param("execId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": exec})
}

// GetVersions 获取版本历史.
func (h *Handlers) GetVersions(c *gin.Context) {
	versions, err := h.mgr.GetVersions(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": versions, "total": len(versions)})
}

// RollbackVersion 回滚版本.
func (h *Handlers) RollbackVersion(c *gin.Context) {
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid version"})
		return
	}
	if err := h.mgr.RollbackVersion(c.Param("id"), version); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rolled back"})
}

// ListTemplates 列出模板.
func (h *Handlers) ListTemplates(c *gin.Context) {
	category := c.Query("category")
	templates := h.mgr.ListTemplates(category)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": templates, "total": len(templates)})
}

// CreateFromTemplate 从模板创建工作流.
func (h *Handlers) CreateFromTemplate(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	wf, err := h.mgr.CreateFromTemplate(c.Param("id"), req.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wf})
}

// GetStats 获取统计.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
