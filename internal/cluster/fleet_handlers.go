// Package cluster 舰队管理 REST API handlers
// /api/v1/cluster/* 路由处理
package cluster

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FleetAPI 舰队管理API处理器
type FleetAPI struct {
	fleet *Fleet
}

// NewFleetAPI 创建舰队API
func NewFleetAPI(fleet *Fleet) *FleetAPI {
	return &FleetAPI{fleet: fleet}
}

// RegisterFleetRoutes 注册舰队管理路由
func (api *FleetAPI) RegisterFleetRoutes(router *gin.RouterGroup) {
	cluster := router.Group("/cluster")
	{
		// 集群概览
		cluster.GET("/summary", api.GetSummary)
		cluster.GET("/health", api.GetClusterHealth)

		// 节点管理
		cluster.GET("/nodes", api.ListNodes)
		cluster.GET("/nodes/:id", api.GetNode)
		cluster.POST("/nodes", api.RegisterNode)
		cluster.PUT("/nodes/:id", api.UpdateNode)
		cluster.DELETE("/nodes/:id", api.UnregisterNode)
		cluster.PUT("/nodes/:id/state", api.UpdateNodeState)
		cluster.PUT("/nodes/:id/role", api.UpdateNodeRole)
		cluster.POST("/nodes/:id/metrics", api.UpdateNodeMetrics)
		cluster.GET("/nodes/:id/health", api.GetNodeHealth)

		// 节点分组
		cluster.GET("/groups", api.ListGroups)
		cluster.GET("/groups/:id", api.GetGroup)
		cluster.POST("/groups", api.CreateGroup)
		cluster.DELETE("/groups/:id", api.DeleteGroup)
		cluster.POST("/groups/:id/nodes/:nodeId", api.AddNodeToGroup)
		cluster.DELETE("/groups/:id/nodes/:nodeId", api.RemoveNodeFromGroup)

		// 跨节点任务
		cluster.GET("/tasks", api.ListTasks)
		cluster.GET("/tasks/:id", api.GetTask)
		cluster.POST("/tasks", api.ScheduleTask)
		cluster.POST("/tasks/:id/cancel", api.CancelTask)

		// 告警
		cluster.GET("/alerts", api.ListAlerts)
		cluster.POST("/alerts/:id/ack", api.AckAlert)
		cluster.POST("/alerts/:id/resolve", api.ResolveAlert)
	}
}

// ========== 集群概览 ==========

// GetSummary 获取集群摘要
func (api *FleetAPI) GetSummary(c *gin.Context) {
	summary := api.fleet.GetFleetSummary()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}

// GetClusterHealth 获取集群健康状态
func (api *FleetAPI) GetClusterHealth(c *gin.Context) {
	api.fleet.mu.RLock()
	nodes := make(map[string]*FleetNode)
	for k, v := range api.fleet.nodes {
		nodes[k] = v
	}
	api.fleet.mu.RUnlock()

	health := api.fleet.healthAgg.GetClusterHealth(nodes)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    health,
	})
}

// ========== 节点管理 ==========

// ListNodes 列出节点
func (api *FleetAPI) ListNodes(c *gin.Context) {
	filter := &NodeFilter{
		Role:  FleetNodeRole(c.Query("role")),
		State: FleetNodeState(c.Query("state")),
		Tag:   c.Query("tag"),
	}

	nodes := api.fleet.ListNodes(filter)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"count":   len(nodes),
	})
}

// GetNode 获取节点
func (api *FleetAPI) GetNode(c *gin.Context) {
	id := c.Param("id")
	node, ok := api.fleet.GetNode(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    node,
	})
}

// RegisterNode 注册节点
func (api *FleetAPI) RegisterNode(c *gin.Context) {
	var node FleetNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := api.fleet.RegisterNode(&node); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    node,
	})
}

// UpdateNode 更新节点
func (api *FleetAPI) UpdateNode(c *gin.Context) {
	id := c.Param("id")
	var node FleetNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}
	node.ID = id

	api.fleet.mu.Lock()
	existing, ok := api.fleet.nodes[id]
	if !ok {
		api.fleet.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "节点不存在",
		})
		return
	}

	// 更新可变字段
	if node.Name != "" {
		existing.Name = node.Name
	}
	if node.Address != "" {
		existing.Address = node.Address
	}
	if node.Port > 0 {
		existing.Port = node.Port
	}
	if len(node.Tags) > 0 {
		existing.Tags = node.Tags
	}
	if len(node.Metadata) > 0 {
		for k, v := range node.Metadata {
			existing.Metadata[k] = v
		}
	}
	api.fleet.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    existing,
	})
}

// UnregisterNode 注销节点
func (api *FleetAPI) UnregisterNode(c *gin.Context) {
	id := c.Param("id")
	if err := api.fleet.UnregisterNode(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点已注销",
	})
}

// UpdateNodeState 更新节点状态
func (api *FleetAPI) UpdateNodeState(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		State FleetNodeState `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效",
		})
		return
	}

	if err := api.fleet.UpdateNodeState(id, req.State); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点状态已更新",
	})
}

// UpdateNodeRole 更新节点角色
func (api *FleetAPI) UpdateNodeRole(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Role FleetNodeRole `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效",
		})
		return
	}

	if err := api.fleet.SetNodeRole(id, req.Role); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点角色已更新",
	})
}

// UpdateNodeMetrics 更新节点指标
func (api *FleetAPI) UpdateNodeMetrics(c *gin.Context) {
	id := c.Param("id")
	var metrics NodeMetrics
	if err := c.ShouldBindJSON(&metrics); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效",
		})
		return
	}

	if err := api.fleet.UpdateNodeMetrics(id, &metrics); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点指标已更新",
	})
}

// GetNodeHealth 获取节点健康
func (api *FleetAPI) GetNodeHealth(c *gin.Context) {
	id := c.Param("id")
	health, ok := api.fleet.healthAgg.GetNodeHealth(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "节点健康数据不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    health,
	})
}

// ========== 节点分组 ==========

// ListGroups 列出分组
func (api *FleetAPI) ListGroups(c *gin.Context) {
	groups := api.fleet.ListGroups()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
		"count":   len(groups),
	})
}

// GetGroup 获取分组
func (api *FleetAPI) GetGroup(c *gin.Context) {
	id := c.Param("id")
	group, ok := api.fleet.GetGroup(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "分组不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    group,
	})
}

// CreateGroup 创建分组
func (api *FleetAPI) CreateGroup(c *gin.Context) {
	var group NodeGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := api.fleet.CreateGroup(&group); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    group,
	})
}

// DeleteGroup 删除分组
func (api *FleetAPI) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := api.fleet.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "分组已删除",
	})
}

// AddNodeToGroup 添加节点到分组
func (api *FleetAPI) AddNodeToGroup(c *gin.Context) {
	groupID := c.Param("id")
	nodeID := c.Param("nodeId")

	if err := api.fleet.AddNodeToGroup(groupID, nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点已添加到分组",
	})
}

// RemoveNodeFromGroup 从分组移除节点
func (api *FleetAPI) RemoveNodeFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	nodeID := c.Param("nodeId")

	if err := api.fleet.RemoveNodeFromGroup(groupID, nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点已从分组移除",
	})
}

// ========== 跨节点任务 ==========

// ListTasks 列出任务
func (api *FleetAPI) ListTasks(c *gin.Context) {
	filter := &TaskFilter{
		Type:   CrossNodeTaskType(c.Query("type")),
		Status: c.Query("status"),
	}

	tasks := api.fleet.ListTasks(filter)

	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
		"count":   len(tasks),
	})
}

// GetTask 获取任务
func (api *FleetAPI) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, ok := api.fleet.GetTask(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    task,
	})
}

// ScheduleTask 创建任务
func (api *FleetAPI) ScheduleTask(c *gin.Context) {
	var task CrossNodeTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := api.fleet.ScheduleTask(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    task,
	})
}

// CancelTask 取消任务
func (api *FleetAPI) CancelTask(c *gin.Context) {
	id := c.Param("id")
	if err := api.fleet.CancelTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "任务已取消",
	})
}

// ========== 告警管理 ==========

// ListAlerts 列出告警
func (api *FleetAPI) ListAlerts(c *gin.Context) {
	filter := &AlertFilter{
		Level:  c.Query("level"),
		NodeID: c.Query("nodeId"),
	}
	if c.Query("unacked") == "true" {
		filter.UnackedOnly = true
	}
	if c.Query("unresolved") == "true" {
		filter.UnresolvedOnly = true
	}

	alerts := api.fleet.alertAgg.GetAlerts(filter)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    alerts,
		"count":   len(alerts),
	})
}

// AckAlert 确认告警
func (api *FleetAPI) AckAlert(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		AckedBy string `json:"ackedBy"`
	}
	c.ShouldBindJSON(&req)
	if req.AckedBy == "" {
		req.AckedBy = "admin"
	}

	if err := api.fleet.alertAgg.AckAlert(id, req.AckedBy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "告警已确认",
	})
}

// ResolveAlert 解决告警
func (api *FleetAPI) ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := api.fleet.alertAgg.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "告警已解决",
	})
}
