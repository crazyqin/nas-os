// Package containersched 提供 REST API 处理器
package containersched

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 智能容器调度模块 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/containersched 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cs := r.Group("/containersched")
	{
		// 节点管理
		cs.POST("/nodes", h.createNode)
		cs.GET("/nodes", h.listNodes)
		cs.GET("/nodes/:id", h.getNode)
		cs.PUT("/nodes/:id", h.updateNode)
		cs.DELETE("/nodes/:id", h.deleteNode)
		cs.PUT("/nodes/:id/resources", h.updateNodeResources)

		// 调度
		cs.POST("/schedule", h.schedule)
		cs.POST("/queue", h.enqueue)
		cs.GET("/queue", h.getQueueStatus)
		cs.POST("/queue/dequeue", h.dequeue)

		// 容器放置
		cs.GET("/placements", h.listPlacements)
		cs.GET("/placements/:container_id", h.getPlacement)
		cs.DELETE("/placements/:container_id", h.removePlacement)

		// 自动扩缩容
		cs.POST("/autoscale/:container_name", h.createAutoScalePolicy)
		cs.GET("/autoscale/:container_name", h.getAutoScalePolicy)
		cs.PUT("/autoscale/:container_name", h.updateAutoScalePolicy)
		cs.POST("/autoscale/:container_name/evaluate", h.evaluateAutoScale)

		// 节能模式
		cs.GET("/powersave", h.getPowerSaveConfig)
		cs.PUT("/powersave", h.updatePowerSaveConfig)
		cs.POST("/powersave/evaluate", h.evaluatePowerSave)

		// 统计
		cs.GET("/stats", h.getStats)
	}
}

// ========== 节点 Handlers ==========

func (h *Handlers) createNode(c *gin.Context) {
	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	node, err := h.manager.CreateNode(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, Response{Code: 0, Message: "created", Data: node})
}

func (h *Handlers) listNodes(c *gin.Context) {
	nodes := h.manager.ListNodes()
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: nodes})
}

func (h *Handlers) getNode(c *gin.Context) {
	id := c.Param("id")
	node, err := h.manager.GetNode(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: node})
}

func (h *Handlers) updateNode(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	node, err := h.manager.UpdateNode(id, req)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "updated", Data: node})
}

func (h *Handlers) deleteNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteNode(id); err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusConflict, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "deleted"})
}

func (h *Handlers) updateNodeResources(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNodeResourcesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	node, err := h.manager.UpdateNodeResources(id, req.Resources)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "updated", Data: node})
}

// ========== 调度 Handlers ==========

func (h *Handlers) schedule(c *gin.Context) {
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result, err := h.manager.Schedule(&req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "scheduled", Data: result})
}

func (h *Handlers) enqueue(c *gin.Context) {
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	item, err := h.manager.Enqueue(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, Response{Code: 0, Message: "enqueued", Data: item})
}

func (h *Handlers) getQueueStatus(c *gin.Context) {
	items := h.manager.GetQueueStatus()
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: items})
}

func (h *Handlers) dequeue(c *gin.Context) {
	item := h.manager.Dequeue()
	if item == nil {
		c.JSON(http.StatusNotFound, Response{Code: 1, Message: "queue is empty"})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "dequeued", Data: item})
}

// ========== 容器放置 Handlers ==========

func (h *Handlers) listPlacements(c *gin.Context) {
	placements := h.manager.ListPlacements()
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: placements})
}

func (h *Handlers) getPlacement(c *gin.Context) {
	containerID := c.Param("container_id")
	placement, err := h.manager.GetPlacement(containerID)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: placement})
}

func (h *Handlers) removePlacement(c *gin.Context) {
	containerID := c.Param("container_id")
	if err := h.manager.RemovePlacement(containerID); err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "removed"})
}

// ========== 自动扩缩容 Handlers ==========

func (h *Handlers) createAutoScalePolicy(c *gin.Context) {
	containerName := c.Param("container_name")
	var policy AutoScalePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	created, err := h.manager.CreateAutoScalePolicy(containerName, &policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, Response{Code: 0, Message: "created", Data: created})
}

func (h *Handlers) getAutoScalePolicy(c *gin.Context) {
	containerName := c.Param("container_name")
	policy, err := h.manager.GetAutoScalePolicy(containerName)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: policy})
}

func (h *Handlers) updateAutoScalePolicy(c *gin.Context) {
	containerName := c.Param("container_name")
	var req UpdateAutoScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	policy, err := h.manager.UpdateAutoScalePolicy(containerName, req)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "updated", Data: policy})
}

func (h *Handlers) evaluateAutoScale(c *gin.Context) {
	containerName := c.Param("container_name")
	action, replicas, reason, err := h.manager.EvaluateAutoScale(containerName)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "evaluated",
		Data: gin.H{
			"action":   action,
			"replicas": replicas,
			"reason":   reason,
		},
	})
}

// ========== 节能模式 Handlers ==========

func (h *Handlers) getPowerSaveConfig(c *gin.Context) {
	config := h.manager.GetPowerSaveConfig()
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: config})
}

func (h *Handlers) updatePowerSaveConfig(c *gin.Context) {
	var req UpdatePowerSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	config := h.manager.UpdatePowerSaveConfig(req)
	c.JSON(http.StatusOK, Response{Code: 0, Message: "updated", Data: config})
}

func (h *Handlers) evaluatePowerSave(c *gin.Context) {
	nodesToDrain, nodesToKeep, err := h.manager.EvaluatePowerSave()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "evaluated",
		Data: gin.H{
			"nodes_to_drain": nodesToDrain,
			"nodes_to_keep":  nodesToKeep,
		},
	})
}

// ========== 统计 Handlers ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: stats})
}

// ========== 辅助函数 ==========

func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	valStr := c.Query(name)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseFloatParam(c *gin.Context, name string, defaultVal float64) float64 {
	valStr := c.Query(name)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
