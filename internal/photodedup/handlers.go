// Package photodedup 提供 REST API 处理器
package photodedup

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 照片去重模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/photo-dedup 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	pd := r.Group("/photo-dedup")
	{
		// 扫描任务管理
		pd.POST("/scan", h.startScan)
		pd.GET("/tasks", h.listTasks)
		pd.GET("/tasks/:taskId", h.getTask)
		pd.POST("/tasks/:taskId/pause", h.pauseTask)
		pd.POST("/tasks/:taskId/resume", h.resumeTask)
		pd.POST("/tasks/:taskId/cancel", h.cancelTask)

		// 重复组
		pd.GET("/tasks/:taskId/groups", h.listDuplicateGroups)
		pd.GET("/tasks/:taskId/groups/:groupId", h.getDuplicateGroup)

		// 保留策略
		pd.PUT("/tasks/:taskId/groups/:groupId/retain", h.setRetain)

		// 清理操作
		pd.POST("/tasks/:taskId/cleanup/preview", h.previewCleanup)
		pd.POST("/tasks/:taskId/cleanup", h.executeCleanup)

		// 统计
		pd.GET("/tasks/:taskId/stats", h.getScanStats)

		// 定时任务
		pd.GET("/schedule", h.getSchedule)
		pd.PUT("/schedule", h.setSchedule)
	}
}

// ========== 扫描任务 Handlers ==========

// startScan 启动扫描任务.
func (h *Handlers) startScan(c *gin.Context) {
	var req StartScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	task, err := h.manager.StartScan(req)
	if err != nil {
		code := http.StatusBadRequest
		if err == ErrInvalidThreshold || err == ErrInvalidHashAlgorithm {
			code = http.StatusUnprocessableEntity
		}
		c.JSON(code, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "扫描任务已启动", Data: task})
}

// listTasks 列出所有扫描任务.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(tasks),
			"tasks": tasks,
		},
	})
}

// getTask 获取扫描任务详情.
func (h *Handlers) getTask(c *gin.Context) {
	taskID := c.Param("taskId")
	task, err := h.manager.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: task})
}

// pauseTask 暂停扫描任务.
func (h *Handlers) pauseTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if err := h.manager.PauseTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "任务已暂停"})
}

// resumeTask 恢复扫描任务.
func (h *Handlers) resumeTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if err := h.manager.ResumeTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "任务已恢复"})
}

// cancelTask 取消扫描任务.
func (h *Handlers) cancelTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if err := h.manager.CancelTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "任务已取消"})
}

// ========== 重复组 Handlers ==========

// listDuplicateGroups 列出任务的重复组.
func (h *Handlers) listDuplicateGroups(c *gin.Context) {
	taskID := c.Param("taskId")
	groups, err := h.manager.GetDuplicateGroups(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(groups),
			"groups": groups,
		},
	})
}

// getDuplicateGroup 获取重复组详情.
func (h *Handlers) getDuplicateGroup(c *gin.Context) {
	taskID := c.Param("taskId")
	groupID := c.Param("groupId")
	group, err := h.manager.GetDuplicateGroup(taskID, groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: group})
}

// ========== 保留策略 Handlers ==========

// setRetain 设置组内保留的照片.
func (h *Handlers) setRetain(c *gin.Context) {
	taskID := c.Param("taskId")
	groupID := c.Param("groupId")

	var req SetRetainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	if err := h.manager.SetRetain(taskID, groupID, req.PhotoID); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "保留策略已更新"})
}

// ========== 清理操作 Handlers ==========

// previewCleanup 预览批量清理结果.
func (h *Handlers) previewCleanup(c *gin.Context) {
	taskID := c.Param("taskId")

	var req BatchCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	preview, err := h.manager.PreviewCleanup(taskID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "预览完成", Data: preview})
}

// executeCleanup 执行批量清理.
func (h *Handlers) executeCleanup(c *gin.Context) {
	taskID := c.Param("taskId")

	var req BatchCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	result, err := h.manager.ExecuteCleanup(taskID, req)
	if err != nil {
		code := http.StatusBadRequest
		if err == ErrBatchNotConfirmed {
			code = http.StatusForbidden
		}
		c.JSON(code, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "清理完成", Data: result})
}

// ========== 统计 Handlers ==========

// getScanStats 获取扫描结果统计.
func (h *Handlers) getScanStats(c *gin.Context) {
	taskID := c.Param("taskId")
	stats, err := h.manager.GetScanStats(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 定时任务 Handlers ==========

// getSchedule 获取定时扫描配置.
func (h *Handlers) getSchedule(c *gin.Context) {
	schedule := h.manager.GetSchedule()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: schedule})
}

// setSchedule 设置定时扫描配置.
func (h *Handlers) setSchedule(c *gin.Context) {
	var cfg ScheduleConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	if err := h.manager.SetSchedule(cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "定时任务已更新", Data: cfg})
}

// ========== 辅助函数 ==========

// parseLimit 解析分页 limit 参数.
func parseLimit(c *gin.Context, defaultVal int) int {
	s := c.DefaultQuery("limit", strconv.Itoa(defaultVal))
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
