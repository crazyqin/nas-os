package multinasmanager

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// API 多NAS管理器API处理器.
type API struct {
	manager *Manager
	logger  *zap.Logger
}

// NewAPI 创建API处理器.
func NewAPI(manager *Manager, logger *zap.Logger) *API {
	return &API{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	// 节点管理.
	nodes := router.Group("/nodes")
	{
		nodes.GET("", api.ListNodes)
		nodes.GET("/:id", api.GetNode)
		nodes.POST("", api.RegisterNode)
		nodes.PUT("/:id/status", api.UpdateNodeStatus)
		nodes.PUT("/:id/metrics", api.UpdateNodeMetrics)
		nodes.DELETE("/:id", api.UnregisterNode)
	}

	// 存储池管理.
	pools := router.Group("/pools")
	{
		pools.GET("", api.ListPools)
		pools.GET("/aggregated", api.GetAggregatedPools)
		pools.GET("/:id", api.GetPool)
		pools.POST("", api.RegisterPool)
		pools.DELETE("/:id", api.UnregisterPool)
	}

	// 告警管理.
	alerts := router.Group("/alerts")
	{
		alerts.GET("", api.ListAlerts)
		alerts.POST("/:id/ack", api.AckAlert)
	}

	// 事件管理.
	events := router.Group("/events")
	{
		events.GET("", api.ListEvents)
	}

	// 迁移任务.
	migrations := router.Group("/migrations")
	{
		migrations.GET("", api.ListMigrations)
		migrations.GET("/:id", api.GetMigration)
		migrations.POST("", api.CreateMigration)
		migrations.PUT("/:id/progress", api.UpdateMigrationProgress)
	}

	// 集群拓扑.
	topology := router.Group("/topology")
	{
		topology.GET("", api.GetTopology)
		topology.PUT("/leader", api.SetLeader)
	}

	// 集群概览.
	router.GET("/overview", api.GetOverview)
}

// 节点相关处理.

// ListNodes 获取节点列表.
func (api *API) ListNodes(c *gin.Context) {
	nodes := api.manager.GetNodes()
	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

// GetNode 获取节点详情.
func (api *API) GetNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, err := api.manager.GetNode(nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

// RegisterNode 注册节点.
func (api *API) RegisterNode(c *gin.Context) {
	var node NASNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := api.manager.RegisterNode(&node); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, node)
}

// UpdateNodeStatus 更新节点状态.
func (api *API) UpdateNodeStatus(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := api.manager.UpdateNodeStatus(nodeID, req.Status); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "状态已更新"})
}

// UpdateNodeMetrics 更新节点指标.
func (api *API) UpdateNodeMetrics(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		CPUUsage    float64 `json:"cpu_usage"`
		MemoryUsage float64 `json:"memory_usage"`
		UsedStorage int64   `json:"used_storage"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := api.manager.UpdateNodeMetrics(nodeID, req.CPUUsage, req.MemoryUsage, req.UsedStorage); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "指标已更新"})
}

// UnregisterNode 注销节点.
func (api *API) UnregisterNode(c *gin.Context) {
	nodeID := c.Param("id")

	if err := api.manager.UnregisterNode(nodeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "节点已注销"})
}

// 存储池相关处理.

// ListPools 获取存储池列表.
func (api *API) ListPools(c *gin.Context) {
	pools := api.manager.GetPools()
	c.JSON(http.StatusOK, gin.H{
		"pools": pools,
		"total": len(pools),
	})
}

// GetAggregatedPools 获取聚合存储池.
func (api *API) GetAggregatedPools(c *gin.Context) {
	pools := api.manager.GetAggregatedPools()
	c.JSON(http.StatusOK, gin.H{
		"pools": pools,
		"total": len(pools),
	})
}

// GetPool 获取存储池详情.
func (api *API) GetPool(c *gin.Context) {
	poolID := c.Param("id")
	pool, err := api.manager.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// RegisterPool 注册存储池.
func (api *API) RegisterPool(c *gin.Context) {
	var pool StoragePool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := api.manager.RegisterPool(&pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, pool)
}

// UnregisterPool 注销存储池.
func (api *API) UnregisterPool(c *gin.Context) {
	poolID := c.Param("id")

	if err := api.manager.UnregisterPool(poolID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "存储池已注销"})
}

// 告警相关处理.

// ListAlerts 获取告警列表.
func (api *API) ListAlerts(c *gin.Context) {
	level := c.Query("level")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	var acked *bool
	if ackedStr := c.Query("acked"); ackedStr != "" {
		v := ackedStr == "true"
		acked = &v
	}

	alerts := api.manager.GetAlerts(level, acked, limit)
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// AckAlert 确认告警.
func (api *API) AckAlert(c *gin.Context) {
	alertID := c.Param("id")

	var req struct {
		AckedBy string `json:"acked_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.AckedBy = "admin"
	}

	if err := api.manager.AckAlert(alertID, req.AckedBy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "告警已确认"})
}

// 事件相关处理.

// ListEvents 获取事件列表.
func (api *API) ListEvents(c *gin.Context) {
	nodeID := c.Query("node_id")
	eventType := c.Query("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	events := api.manager.GetEvents(nodeID, eventType, limit)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// 迁移相关处理.

// ListMigrations 获取迁移任务列表.
func (api *API) ListMigrations(c *gin.Context) {
	status := c.Query("status")
	migrations := api.manager.GetMigrations(status)
	c.JSON(http.StatusOK, gin.H{
		"migrations": migrations,
		"total":      len(migrations),
	})
}

// GetMigration 获取迁移任务详情.
func (api *API) GetMigration(c *gin.Context) {
	taskID := c.Param("id")
	task, err := api.manager.GetMigration(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// CreateMigration 创建迁移任务.
func (api *API) CreateMigration(c *gin.Context) {
	var req struct {
		SourceNodeID string `json:"source_node_id" binding:"required"`
		TargetNodeID string `json:"target_node_id" binding:"required"`
		SourcePath   string `json:"source_path" binding:"required"`
		TargetPath   string `json:"target_path" binding:"required"`
		TotalBytes   int64  `json:"total_bytes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	task, err := api.manager.CreateMigration(req.SourceNodeID, req.TargetNodeID, req.SourcePath, req.TargetPath, req.TotalBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// UpdateMigrationProgress 更新迁移进度.
func (api *API) UpdateMigrationProgress(c *gin.Context) {
	taskID := c.Param("id")

	var req struct {
		CopiedBytes int64  `json:"copied_bytes"`
		Status      string `json:"status" binding:"required"`
		Error       string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := api.manager.UpdateMigrationProgress(taskID, req.CopiedBytes, req.Status, req.Error); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "进度已更新"})
}

// 拓扑相关处理.

// GetTopology 获取集群拓扑.
func (api *API) GetTopology(c *gin.Context) {
	topology := api.manager.GetTopology()
	c.JSON(http.StatusOK, topology)
}

// SetLeader 设置领导节点.
func (api *API) SetLeader(c *gin.Context) {
	var req struct {
		NodeID string `json:"node_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	api.manager.SetLeader(req.NodeID)
	c.JSON(http.StatusOK, gin.H{"message": "领导节点已设置"})
}

// GetOverview 获取集群概览.
func (api *API) GetOverview(c *gin.Context) {
	nodes := api.manager.GetNodes()
	pools := api.manager.GetPools()
	topology := api.manager.GetTopology()

	var totalStorage, usedStorage, freeStorage int64
	for _, node := range nodes {
		totalStorage += node.TotalStorage
		usedStorage += node.UsedStorage
		freeStorage += node.FreeStorage
	}

	var onlineNodes, offlineNodes, degradedNodes int
	for _, node := range nodes {
		switch node.Status {
		case NodeStatusOnline:
			onlineNodes++
		case NodeStatusOffline:
			offlineNodes++
		case NodeStatusDegraded:
			degradedNodes++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"cluster": gin.H{
			"name":           api.manager.config.Name,
			"leader_id":      topology.LeaderID,
			"total_nodes":    len(nodes),
			"online_nodes":   onlineNodes,
			"offline_nodes":  offlineNodes,
			"degraded_nodes": degradedNodes,
		},
		"storage": gin.H{
			"total_pools": len(pools),
			"total_size":  totalStorage,
			"used_size":   usedStorage,
			"free_size":   freeStorage,
		},
	})
}
