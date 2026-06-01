// Package smartnasrouter 提供智能NAS路由HTTP接口
package smartnasrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能路由HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	router := r.Group("/smart-router")
	{
		// 节点管理
		router.POST("/nodes", h.handleAddNode)
		router.GET("/nodes", h.handleListNodes)
		router.GET("/nodes/:id", h.handleGetNode)
		router.PUT("/nodes/:id", h.handleUpdateNode)
		router.DELETE("/nodes/:id", h.handleDeleteNode)
		router.PUT("/nodes/:id/metrics", h.handleUpdateMetrics)

		// 路由规则
		router.POST("/rules", h.handleAddRule)
		router.GET("/rules", h.handleListRules)
		router.DELETE("/rules/:id", h.handleDeleteRule)

		// 路由决策
		router.POST("/route", h.handleRoute)

		// 延迟探测
		router.POST("/probe/:id", h.handleProbeNode)
		router.POST("/probe/all", h.handleProbeAll)

		// 故障转移
		router.POST("/failover/:id", h.handleFailover)
		router.POST("/recover/:id", h.handleRecover)

		// 统计
		router.GET("/stats", h.handleGetStats)
		router.GET("/failover-events", h.handleGetFailoverEvents)
	}
}

// handleAddNode 添加节点
func (h *Handlers) handleAddNode(c *gin.Context) {
	var req AddNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node, err := h.manager.AddNode(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, node)
}

// handleListNodes 列出节点
func (h *Handlers) handleListNodes(c *gin.Context) {
	nodes := h.manager.ListNodes()
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// handleGetNode 获取节点
func (h *Handlers) handleGetNode(c *gin.Context) {
	id := c.Param("id")
	node, err := h.manager.GetNode(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

// handleUpdateNode 更新节点
func (h *Handlers) handleUpdateNode(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node, err := h.manager.UpdateNode(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

// handleDeleteNode 删除节点
func (h *Handlers) handleDeleteNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteNode(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "节点已删除"})
}

// handleUpdateMetrics 更新节点指标
func (h *Handlers) handleUpdateMetrics(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		CPU    float64 `json:"cpu"`
		Memory float64 `json:"memory"`
		Disk   float64 `json:"disk"`
		Conns  int     `json:"conns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateNodeMetrics(id, req.CPU, req.Memory, req.Disk, req.Conns); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "指标已更新"})
}

// handleAddRule 添加路由规则
func (h *Handlers) handleAddRule(c *gin.Context) {
	var rule RouteRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.manager.AddRule(rule)
	c.JSON(http.StatusCreated, result)
}

// handleListRules 列出路由规则
func (h *Handlers) handleListRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// handleDeleteRule 删除路由规则
func (h *Handlers) handleDeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "规则已删除"})
}

// handleRoute 路由决策
func (h *Handlers) handleRoute(c *gin.Context) {
	var req RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	decision, err := h.manager.Route(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, decision)
}

// handleProbeNode 探测节点
func (h *Handlers) handleProbeNode(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.ProbeNode(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// handleProbeAll 探测所有节点
func (h *Handlers) handleProbeAll(c *gin.Context) {
	results := h.manager.ProbeAll()
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// handleFailover 触发故障转移
func (h *Handlers) handleFailover(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = "手动触发"
	}

	event, err := h.manager.TriggerFailover(id, req.Reason)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, event)
}

// handleRecover 恢复节点
func (h *Handlers) handleRecover(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RecoverNode(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "节点已恢复"})
}

// handleGetStats 获取统计
func (h *Handlers) handleGetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// handleGetFailoverEvents 获取故障转移事件
func (h *Handlers) handleGetFailoverEvents(c *gin.Context) {
	events := h.manager.GetFailoverEvents()
	c.JSON(http.StatusOK, gin.H{"events": events})
}
