// Package datapipeline handlers - HTTP API
package datapipeline

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/datapipeline")
	{
		// 管道管理
		g.GET("/pipelines", h.ListPipelines)
		g.POST("/pipelines", h.CreatePipeline)
		g.GET("/pipelines/:id", h.GetPipeline)
		g.PUT("/pipelines/:id", h.UpdatePipeline)
		g.DELETE("/pipelines/:id", h.DeletePipeline)
		g.POST("/pipelines/:id/start", h.StartPipeline)
		g.POST("/pipelines/:id/stop", h.StopPipeline)
		g.POST("/pipelines/:id/trigger", h.TriggerExecution)
		g.GET("/pipelines/:id/health", h.GetPipelineHealth)

		// 执行历史
		g.GET("/pipelines/:id/executions", h.GetExecutions)
		g.GET("/pipelines/:id/executions/:eid", h.GetExecution)

		// 全局执行历史
		g.GET("/executions", h.ListAllExecutions)

		// 死信队列
		g.GET("/dlq", h.GetDLQ)
		g.POST("/dlq/:eid/retry", h.RetryDLQEntry)
		g.DELETE("/dlq", h.ClearDLQ)

		// 统计
		g.GET("/stats", h.GetStats)
	}
}

// ListPipelines 列出管道
func (h *Handlers) ListPipelines(c *gin.Context) {
	status := PipelineStatus(c.Query("status"))
	tag := c.Query("tag")
	pipelines := h.mgr.ListPipelines(status, tag)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pipelines, "total": len(pipelines)})
}

// CreatePipeline 创建管道
func (h *Handlers) CreatePipeline(c *gin.Context) {
	var p Pipeline
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreatePipeline(&p); err != nil {
		if err == ErrPipelineExists {
			c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

// GetPipeline 获取管道
func (h *Handlers) GetPipeline(c *gin.Context) {
	p, err := h.mgr.GetPipeline(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

// UpdatePipeline 更新管道
func (h *Handlers) UpdatePipeline(c *gin.Context) {
	var update Pipeline
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.UpdatePipeline(c.Param("id"), &update); err != nil {
		if err == ErrPipelineNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		} else if err == ErrPipelineRunning {
			c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		}
		return
	}
	p, _ := h.mgr.GetPipeline(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

// DeletePipeline 删除管道
func (h *Handlers) DeletePipeline(c *gin.Context) {
	if err := h.mgr.DeletePipeline(c.Param("id")); err != nil {
		if err == ErrPipelineNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		} else if err == ErrPipelineRunning {
			c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// StartPipeline 启动管道
func (h *Handlers) StartPipeline(c *gin.Context) {
	if err := h.mgr.StartPipeline(c.Param("id")); err != nil {
		if err == ErrPipelineNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		} else if err == ErrPipelineRunning {
			c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "started"})
}

// StopPipeline 停止管道
func (h *Handlers) StopPipeline(c *gin.Context) {
	if err := h.mgr.StopPipeline(c.Param("id")); err != nil {
		if err == ErrPipelineNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		} else if err == ErrPipelineNotRunning {
			c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "stopped"})
}

// TriggerExecution 手动触发执行
func (h *Handlers) TriggerExecution(c *gin.Context) {
	exec, err := h.mgr.TriggerExecution(c.Param("id"))
	if err != nil {
		if err == ErrPipelineNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		} else {
			c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": exec})
}

// GetPipelineHealth 获取管道健康状态
func (h *Handlers) GetPipelineHealth(c *gin.Context) {
	health, err := h.mgr.GetPipelineHealth(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": health})
}

// GetExecutions 获取执行历史
func (h *Handlers) GetExecutions(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	execs := h.mgr.GetExecutions(c.Param("id"), limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": execs, "total": len(execs)})
}

// GetExecution 获取单个执行记录
func (h *Handlers) GetExecution(c *gin.Context) {
	exec, err := h.mgr.GetExecution(c.Param("id"), c.Param("eid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": exec})
}

// ListAllExecutions 列出所有执行历史
func (h *Handlers) ListAllExecutions(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	execs := h.mgr.ListAllExecutions(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": execs, "total": len(execs)})
}

// GetDLQ 获取死信队列
func (h *Handlers) GetDLQ(c *gin.Context) {
	pipelineID := c.Query("pipeline_id")
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	entries := h.mgr.GetDLQ(pipelineID, limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": entries, "total": len(entries)})
}

// RetryDLQEntry 重试死信队列条目
func (h *Handlers) RetryDLQEntry(c *gin.Context) {
	if err := h.mgr.RetryDLQEntry(c.Param("eid")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "retrying"})
}

// ClearDLQ 清空死信队列
func (h *Handlers) ClearDLQ(c *gin.Context) {
	pipelineID := c.Query("pipeline_id")
	removed := h.mgr.ClearDLQ(pipelineID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "cleared", "removed": removed})
}

// GetStats 获取统计信息
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
