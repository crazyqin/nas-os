// Package scheduler 提供定时任务调度 API
// Version: v2.49.0
package scheduler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers API 处理器.
type Handlers struct {
	scheduler *Scheduler
}

// NewHandlers 创建处理器.
func NewHandlers(scheduler *Scheduler) *Handlers {
	return &Handlers{
		scheduler: scheduler,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sched := r.Group("/scheduler")
	{
		// 任务管理
		tasks := sched.Group("/tasks")
		{
			tasks.GET("", h.listTasks)
			tasks.POST("", h.createTask)
			tasks.GET("/stats", h.getStats)
			tasks.GET("/:id", h.getTask)
			tasks.PUT("/:id", h.updateTask)
			tasks.DELETE("/:id", h.deleteTask)
			tasks.POST("/:id/run", h.runTask)
			tasks.POST("/:id/cancel", h.cancelTask)
			tasks.POST("/:id/pause", h.pauseTask)
			tasks.POST("/:id/resume", h.resumeTask)
			tasks.POST("/:id/enable", h.enableTask)
			tasks.POST("/:id/disable", h.disableTask)
			tasks.GET("/:id/next-runs", h.getNextRuns)
		}

		// 执行记录
		executions := sched.Group("/executions")
		{
			executions.GET("", h.listExecutions)
			executions.GET("/:id", h.getExecution)
			executions.DELETE("/:id", h.deleteExecution)
		}

		// 日志
		logs := sched.Group("/logs")
		{
			logs.GET("", h.getLogs)
			logs.GET("/execution/:id", h.getExecutionLogs)
			logs.GET("/task/:id", h.getTaskLogs)
			logs.DELETE("", h.clearLogs)
		}

		// 处理器
		handlers := sched.Group("/handlers")
		{
			handlers.GET("", h.listHandlers)
		}

		// 依赖图
		sched.GET("/dependency-graph", h.getDependencyGraph)

		// Cron 工具
		cron := sched.Group("/cron")
		{
			cron.POST("/validate", h.validateCron)
			cron.POST("/next", h.getNextCronTimes)
			cron.GET("/presets", h.getCronPresets)
		}

		// 重试
		retry := sched.Group("/retry")
		{
			retry.GET("/pending", h.getPendingRetries)
		}
	}
}

// ========== 任务管理 ==========

// listTasks 列出任务
// @Summary 列出任务
// @Description 列出所有定时任务
// @Tags scheduler/tasks
// @Produce json
// @Param status query string false "状态过滤"
// @Param type query string false "类型过滤"
// @Param group query string false "分组过滤"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks [get]
// @Security BearerAuth.
func (h *Handlers) listTasks(c *gin.Context) {
	filter := &TaskFilter{
		Status:   TaskStatus(c.Query("status")),
		Type:     TaskType(c.Query("type")),
		Group:    c.Query("group"),
		Page:     1,
		PageSize: 20,
	}

	if page := c.Query("page"); page != "" {
		var p int
		if _, err := fmt.Sscanf(page, "%d", &p); err == nil {
			filter.Page = p
		}
	}

	if pageSize := c.Query("pageSize"); pageSize != "" {
		var ps int
		if _, err := fmt.Sscanf(pageSize, "%d", &ps); err == nil {
			filter.PageSize = ps
		}
	}

	tasks := h.scheduler.ListTasks(filter)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"tasks": tasks,
			"total": len(tasks),
		},
	})
}

// createTask 创建任务
// @Summary 创建任务
// @Description 创建新的定时任务
// @Tags scheduler/tasks
// @Accept json
// @Produce json
// @Param request body Task true "任务配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks [post]
// @Security BearerAuth.
func (h *Handlers) createTask(c *gin.Context) {
	var task Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.scheduler.AddTask(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已创建",
		"data":    task,
	})
}

// getTask 获取任务
// @Summary 获取任务
// @Description 获取指定任务的详细信息
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id} [get]
// @Security BearerAuth.
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.scheduler.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// updateTask 更新任务
// @Summary 更新任务
// @Description 更新指定任务的配置
// @Tags scheduler/tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body Task true "任务配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id} [put]
// @Security BearerAuth.
func (h *Handlers) updateTask(c *gin.Context) {
	id := c.Param("id")

	var task Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	task.ID = id
	if err := h.scheduler.UpdateTask(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已更新",
		"data":    task,
	})
}

// deleteTask 删除任务
// @Summary 删除任务
// @Description 删除指定任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id} [delete]
// @Security BearerAuth.
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.DeleteTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已删除",
	})
}

// runTask 运行任务
// @Summary 运行任务
// @Description 手动运行指定任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/run [post]
// @Security BearerAuth.
func (h *Handlers) runTask(c *gin.Context) {
	id := c.Param("id")

	execution, err := h.scheduler.RunTask(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已开始运行",
		"data":    execution,
	})
}

// cancelTask 取消任务
// @Summary 取消任务
// @Description 取消正在运行的任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/cancel [post]
// @Security BearerAuth.
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.CancelTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已取消",
	})
}

// pauseTask 暂停任务
// @Summary 暂停任务
// @Description 暂停指定任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/pause [post]
// @Security BearerAuth.
func (h *Handlers) pauseTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.PauseTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已暂停",
	})
}

// resumeTask 恢复任务
// @Summary 恢复任务
// @Description 恢复暂停的任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/resume [post]
// @Security BearerAuth.
func (h *Handlers) resumeTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.ResumeTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已恢复",
	})
}

// enableTask 启用任务
// @Summary 启用任务
// @Description 启用指定任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/enable [post]
// @Security BearerAuth.
func (h *Handlers) enableTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.EnableTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已启用",
	})
}

// disableTask 禁用任务
// @Summary 禁用任务
// @Description 禁用指定任务
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/disable [post]
// @Security BearerAuth.
func (h *Handlers) disableTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.DisableTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已禁用",
	})
}

// getNextRuns 获取下次执行时间
// @Summary 获取下次执行时间
// @Description 获取任务的接下来 N 次执行时间
// @Tags scheduler/tasks
// @Produce json
// @Param id path string true "任务ID"
// @Param n query int false "次数"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/{id}/next-runs [get]
// @Security BearerAuth.
func (h *Handlers) getNextRuns(c *gin.Context) {
	id := c.Param("id")

	n := 5
	if nStr := c.Query("n"); nStr != "" {
		var num int
		if _, err := fmt.Sscanf(nStr, "%d", &num); err == nil && num > 0 {
			n = num
		}
	}

	times := h.scheduler.GetNextRunTimes(id, n)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    times,
	})
}

// getStats 获取统计信息
// @Summary 获取统计信息
// @Description 获取调度器统计信息
// @Tags scheduler/tasks
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/tasks/stats [get]
// @Security BearerAuth.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.scheduler.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ========== 执行记录 ==========

// listExecutions 列出执行记录
// @Summary 列出执行记录
// @Description 列出任务执行记录
// @Tags scheduler/executions
// @Produce json
// @Param taskId query string false "任务ID"
// @Param status query string false "状态过滤"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/executions [get]
// @Security BearerAuth.
func (h *Handlers) listExecutions(c *gin.Context) {
	filter := &ExecutionFilter{
		TaskID:   c.Query("taskId"),
		Status:   ExecutionStatus(c.Query("status")),
		Page:     1,
		PageSize: 20,
	}

	if page := c.Query("page"); page != "" {
		var p int
		if _, err := fmt.Sscanf(page, "%d", &p); err == nil {
			filter.Page = p
		}
	}

	if pageSize := c.Query("pageSize"); pageSize != "" {
		var ps int
		if _, err := fmt.Sscanf(pageSize, "%d", &ps); err == nil {
			filter.PageSize = ps
		}
	}

	executions := h.scheduler.GetLogManager().QueryExecutions(filter)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"executions": executions,
		},
	})
}

// getExecution 获取执行记录
// @Summary 获取执行记录
// @Description 获取指定执行记录的详细信息
// @Tags scheduler/executions
// @Produce json
// @Param id path string true "执行ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/executions/{id} [get]
// @Security BearerAuth.
func (h *Handlers) getExecution(c *gin.Context) {
	id := c.Param("id")

	execution, err := h.scheduler.GetLogManager().GetExecution(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    execution,
	})
}

// deleteExecution 删除执行记录
// @Summary 删除执行记录
// @Description 删除指定执行记录
// @Tags scheduler/executions
// @Produce json
// @Param id path string true "执行ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/executions/{id} [delete]
// @Security BearerAuth.
func (h *Handlers) deleteExecution(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.GetLogManager().DeleteExecution(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "执行记录已删除",
	})
}

// ========== 日志 ==========

// getLogs 获取日志
// @Summary 获取日志
// @Description 获取执行日志
// @Tags scheduler/logs
// @Produce json
// @Param executionId query string false "执行ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/logs [get]
// @Security BearerAuth.
func (h *Handlers) getLogs(c *gin.Context) {
	executionID := c.Query("executionId")

	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "executionId 参数必填",
		})
		return
	}

	logs := h.scheduler.GetLogManager().GetLogs(executionID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    logs,
	})
}

// getExecutionLogs 获取执行日志
// @Summary 获取执行日志
// @Description 获取指定执行的日志
// @Tags scheduler/logs
// @Produce json
// @Param id path string true "执行ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/logs/execution/{id} [get]
// @Security BearerAuth.
func (h *Handlers) getExecutionLogs(c *gin.Context) {
	id := c.Param("id")

	logs := h.scheduler.GetLogManager().GetLogs(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    logs,
	})
}

// getTaskLogs 获取任务日志
// @Summary 获取任务日志
// @Description 获取指定任务的所有执行日志
// @Tags scheduler/logs
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/logs/task/{id} [get]
// @Security BearerAuth.
func (h *Handlers) getTaskLogs(c *gin.Context) {
	id := c.Param("id")

	logs := h.scheduler.GetLogManager().GetTaskLogs(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    logs,
	})
}

// clearLogs 清空日志
// @Summary 清空日志
// @Description 清空所有执行日志
// @Tags scheduler/logs
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/logs [delete]
// @Security BearerAuth.
func (h *Handlers) clearLogs(c *gin.Context) {
	if err := h.scheduler.GetLogManager().Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "日志已清空",
	})
}

// ========== 处理器 ==========

// listHandlers 列出处理器
// @Summary 列出处理器
// @Description 列出所有可用的任务处理器
// @Tags scheduler/handlers
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/handlers [get]
// @Security BearerAuth.
func (h *Handlers) listHandlers(c *gin.Context) {
	handlers := h.scheduler.GetExecutor().ListHandlers()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    handlers,
	})
}

// ========== 依赖图 ==========

// getDependencyGraph 获取依赖图
// @Summary 获取依赖图
// @Description 获取任务依赖关系图
// @Tags scheduler
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/dependency-graph [get]
// @Security BearerAuth.
func (h *Handlers) getDependencyGraph(c *gin.Context) {
	graph := h.scheduler.GetDependencyGraph()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    graph,
	})
}

// ========== Cron 工具 ==========

// validateCron 验证 Cron 表达式
// @Summary 验证 Cron 表达式
// @Description 验证 Cron 表达式是否有效
// @Tags scheduler/cron
// @Accept json
// @Produce json
// @Param request body map[string]string true "Cron 表达式"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/cron/validate [post]
// @Security BearerAuth.
func (h *Handlers) validateCron(c *gin.Context) {
	var req struct {
		Expression string `json:"expression"`
		WithSecond bool   `json:"withSecond"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	valid := IsValidCron(req.Expression, req.WithSecond)

	var desc string
	if valid {
		desc = DescribeCron(req.Expression)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"valid":       valid,
			"description": desc,
		},
	})
}

// getNextCronTimes 获取下次执行时间
// @Summary 获取下次执行时间
// @Description 计算 Cron 表达式的下次执行时间
// @Tags scheduler/cron
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Cron 表达式"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/cron/next [post]
// @Security BearerAuth.
func (h *Handlers) getNextCronTimes(c *gin.Context) {
	var req struct {
		Expression string `json:"expression"`
		WithSecond bool   `json:"withSecond"`
		Count      int    `json:"count"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Count <= 0 {
		req.Count = 5
	}

	expr, err := NewCronExpression(req.Expression, CronParseOptions{Second: req.WithSecond})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	times := expr.NextN(time.Now(), req.Count)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    times,
	})
}

// getCronPresets 获取预设表达式
// @Summary 获取预设表达式
// @Description 获取常用的 Cron 预设表达式
// @Tags scheduler/cron
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/cron/presets [get]
// @Security BearerAuth.
func (h *Handlers) getCronPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    CronPresets,
	})
}

// ========== 重试 ==========

// getPendingRetries 获取等待重试的任务
// @Summary 获取等待重试的任务
// @Description 获取所有等待重试的任务列表
// @Tags scheduler/retry
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /scheduler/retry/pending [get]
// @Security BearerAuth.
func (h *Handlers) getPendingRetries(c *gin.Context) {
	// 这个功能需要从重试管理器获取数据
	// 简化实现
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    []interface{}{},
	})
}
