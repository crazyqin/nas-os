// Package edgeorchestrator 提供边缘计算编排 REST API
package edgeorchestrator

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 边缘编排器 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/edgeorch 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	eo := r.Group("/edgeorch")
	{
		// 节点管理
		eo.POST("/nodes", h.registerNode)
		eo.GET("/nodes", h.listNodes)
		eo.GET("/nodes/:id", h.getNode)
		eo.DELETE("/nodes/:id", h.unregisterNode)
		eo.PUT("/nodes/:id/status", h.updateNodeStatus)
		eo.PUT("/nodes/:id/labels", h.updateNodeLabels)
		eo.POST("/nodes/:id/heartbeat", h.nodeHeartbeat)
		eo.GET("/nodes/:id/health", h.getNodeHealth)

		// 任务管理
		eo.POST("/tasks", h.submitTask)
		eo.GET("/tasks", h.listTasks)
		eo.GET("/tasks/:id", h.getTask)
		eo.POST("/tasks/:id/schedule", h.scheduleTask)
		eo.POST("/tasks/:id/start", h.startTask)
		eo.POST("/tasks/:id/complete", h.completeTask)
		eo.POST("/tasks/:id/fail", h.failTask)
		eo.POST("/tasks/:id/cancel", h.cancelTask)

		// AI推理
		eo.POST("/inference", h.submitInference)
		eo.GET("/inference/:id/result", h.getInferenceResult)
		eo.POST("/inference/:id/complete", h.completeInference)

		// 集群监控
		eo.GET("/metrics", h.getClusterMetrics)
		eo.GET("/sync-status", h.getSyncStatus)
		eo.POST("/sync", h.syncNodeStatus)
	}
}

// ========== 节点 Handlers ==========

func (h *Handlers) registerNode(c *gin.Context) {
	var req RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	node, err := h.manager.RegisterNode(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "node registered", Data: node})
}

func (h *Handlers) listNodes(c *gin.Context) {
	status := c.Query("status")
	nodes := h.manager.ListNodes(status)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"nodes": nodes,
			"total": len(nodes),
		},
	})
}

func (h *Handlers) getNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, err := h.manager.GetNode(nodeID)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: node})
}

func (h *Handlers) unregisterNode(c *gin.Context) {
	nodeID := c.Param("id")
	if err := h.manager.UnregisterNode(nodeID); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "node unregistered"})
}

func (h *Handlers) updateNodeStatus(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Status EdgeNodeStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.UpdateNodeStatus(nodeID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "status updated"})
}

func (h *Handlers) updateNodeLabels(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Labels map[string]string `json:"labels" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.UpdateNodeLabels(nodeID, req.Labels); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "labels updated"})
}

func (h *Handlers) nodeHeartbeat(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		CPUUsage     float64 `json:"cpu_usage"`
		MemUsage     float64 `json:"mem_usage"`
		RunningTasks int     `json:"running_tasks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.Heartbeat(nodeID, req.CPUUsage, req.MemUsage, req.RunningTasks); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "heartbeat received"})
}

func (h *Handlers) getNodeHealth(c *gin.Context) {
	nodeID := c.Param("id")
	health, err := h.manager.CheckNodeHealth(nodeID)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: health})
}

// ========== 任务 Handlers ==========

func (h *Handlers) submitTask(c *gin.Context) {
	var req SubmitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	task, err := h.manager.SubmitTask(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "task submitted", Data: task})
}

func (h *Handlers) listTasks(c *gin.Context) {
	filter := &ListTasksRequest{
		Status: c.Query("status"),
		Type:   c.Query("type"),
		NodeID: c.Query("node_id"),
	}

	if v := c.Query("priority"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			filter.Priority = p
		}
	}
	if v := c.Query("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			filter.Limit = l
		}
	}
	if v := c.Query("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil {
			filter.Offset = o
		}
	}

	tasks := h.manager.ListTasks(filter)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"tasks": tasks,
			"total": len(tasks),
		},
	})
}

func (h *Handlers) getTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.manager.GetTask(taskID)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: task})
}

func (h *Handlers) scheduleTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.manager.ScheduleTask(taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "task scheduled", Data: task})
}

func (h *Handlers) startTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.manager.StartTask(taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "task started", Data: task})
}

func (h *Handlers) completeTask(c *gin.Context) {
	taskID := c.Param("id")

	var req struct {
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
		Error    string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result := &TaskResult{
		ExitCode: req.ExitCode,
		Output:   req.Output,
		Error:    req.Error,
	}

	task, err := h.manager.CompleteTask(taskID, result)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "task completed", Data: task})
}

func (h *Handlers) failTask(c *gin.Context) {
	taskID := c.Param("id")

	var req struct {
		Error string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	task, err := h.manager.FailTask(taskID, req.Error)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "task failed", Data: task})
}

func (h *Handlers) cancelTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.manager.CancelTask(taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "task cancelled", Data: task})
}

// ========== AI 推理 Handlers ==========

func (h *Handlers) submitInference(c *gin.Context) {
	var req SubmitInferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	task, err := h.manager.SubmitInference(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "inference task submitted", Data: task})
}

func (h *Handlers) getInferenceResult(c *gin.Context) {
	taskID := c.Param("id")

	// 使用context实现超时控制
	ctx, cancel := context.WithTimeout(c.Request.Context(), taskMaxWaitTime)
	defer cancel()

	resultCh := make(chan *InferenceResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := h.manager.GetInferenceResult(taskID)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-ctx.Done():
		c.JSON(http.StatusRequestTimeout, response{Code: 1, Message: "inference timeout"})
	case err := <-errCh:
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
	case result := <-resultCh:
		c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
	}
}

func (h *Handlers) completeInference(c *gin.Context) {
	taskID := c.Param("id")

	var req InferenceResult
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.CompleteInference(taskID, &req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "inference completed"})
}

// ========== 监控 Handlers ==========

func (h *Handlers) getClusterMetrics(c *gin.Context) {
	metrics := h.manager.GetClusterMetrics()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: metrics})
}

func (h *Handlers) getSyncStatus(c *gin.Context) {
	statuses := h.manager.GetSyncStatus()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"sync_statuses": statuses,
			"total":         len(statuses),
		},
	})
}

func (h *Handlers) syncNodeStatus(c *gin.Context) {
	if err := h.manager.SyncNodeStatus(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "sync completed"})
}

// 常量
const taskMaxWaitTime = 30e9 // 30秒
