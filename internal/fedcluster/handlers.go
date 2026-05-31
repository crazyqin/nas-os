// Package fedcluster 提供联邦集群管理功能.
package fedcluster

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 集群管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	clusterGroup := api.Group("/cluster")
	{
		// 集群管理
		clusterGroup.POST("", h.createCluster)
		clusterGroup.GET("", h.listClusters)
		clusterGroup.GET("/:id", h.getCluster)
		clusterGroup.GET("/:id/stats", h.getClusterStats)
		clusterGroup.GET("/:id/health", h.getClusterHealth)
		clusterGroup.GET("/:id/events", h.getClusterEvents)

		// 节点管理
		clusterGroup.POST("/:id/nodes", h.joinNode)
		clusterGroup.DELETE("/:id/nodes/:nodeId", h.removeNode)
		clusterGroup.PUT("/:id/nodes/:nodeId/promote", h.promoteNode)
		clusterGroup.PUT("/:id/nodes/:nodeId/maintenance", h.setMaintenance)
		clusterGroup.POST("/:id/nodes/:nodeId/heartbeat", h.updateHeartbeat)

		// 同步管理
		clusterGroup.POST("/:id/sync", h.startSync)
		clusterGroup.GET("/:id/sync", h.listSyncJobs)
		clusterGroup.GET("/:id/sync/:jobId", h.getSyncJob)

		// 负载均衡
		clusterGroup.GET("/:id/select-node", h.selectNode)
		clusterGroup.PUT("/:id/lb-config", h.updateLBConfig)

		// 集群操作
		clusterGroup.POST("/:id/failover", h.failover)
		clusterGroup.POST("/:id/rebalance", h.rebalance)
	}
}

// createCluster 创建集群
func (h *Handlers) createCluster(c *gin.Context) {
	var req ClusterCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	cluster, err := h.manager.CreateCluster(req.Name, req.Description, req.SyncPolicy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cluster)
}

// listClusters 列出集群
func (h *Handlers) listClusters(c *gin.Context) {
	clusters := h.manager.ListClusters()

	// 转换为摘要信息
	infos := make([]*ClusterInfo, 0, len(clusters))
	for _, cluster := range clusters {
		onlineNodes := 0
		totalStorage := 0.0
		usedStorage := 0.0

		for _, node := range cluster.Nodes {
			if node.Status == NodeOnline {
				onlineNodes++
			}
			totalStorage += node.StorageTB
			usedStorage += node.UsedStorageTB
		}

		status := "healthy"
		if onlineNodes < len(cluster.Nodes) {
			status = "degraded"
		}
		if onlineNodes == 0 {
			status = "critical"
		}

		infos = append(infos, &ClusterInfo{
			ID:          cluster.ID,
			Name:        cluster.Name,
			NodeCount:   len(cluster.Nodes),
			OnlineNodes: onlineNodes,
			TotalTB:     totalStorage,
			UsedTB:      usedStorage,
			Status:      status,
			CreatedAt:   cluster.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, infos)
}

// getCluster 获取集群详情
func (h *Handlers) getCluster(c *gin.Context) {
	clusterID := c.Param("id")

	cluster, err := h.manager.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// getClusterStats 获取集群统计
func (h *Handlers) getClusterStats(c *gin.Context) {
	clusterID := c.Param("id")

	stats, err := h.manager.GetClusterStats(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// getClusterHealth 获取集群健康状态
func (h *Handlers) getClusterHealth(c *gin.Context) {
	clusterID := c.Param("id")

	cluster, err := h.manager.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	health := h.manager.HealthCheck(clusterID)

	overallStatus := "healthy"
	issues := make([]string, 0)

	for nodeID, healthy := range health {
		if !healthy {
			overallStatus = "degraded"
			if node, ok := cluster.Nodes[nodeID]; ok {
				issues = append(issues, "节点 "+node.Name+" 离线")
			}
		}
	}

	if len(issues) == len(cluster.Nodes) {
		overallStatus = "critical"
	}

	result := &ClusterHealth{
		ClusterID:     clusterID,
		OverallStatus: overallStatus,
		NodeHealth:    health,
		SyncStatus:    "idle",
		LastCheck:     cluster.UpdatedAt,
		Issues:        issues,
	}

	c.JSON(http.StatusOK, result)
}

// getClusterEvents 获取集群事件
func (h *Handlers) getClusterEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	events := h.manager.GetEventLog(limit)
	c.JSON(http.StatusOK, events)
}

// joinNode 节点加入集群
func (h *Handlers) joinNode(c *gin.Context) {
	clusterID := c.Param("id")

	var req NodeJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	node := &ClusterNode{
		Name:     req.Name,
		Hostname: req.Hostname,
		Port:     req.Port,
		Role:     req.Role,
		Tags:     req.Tags,
	}

	if node.Port == 0 {
		node.Port = 8080
	}

	if node.Name == "" {
		node.Name = req.Hostname
	}

	if node.Role == "" {
		node.Role = RoleWorker
	}

	if err := h.manager.JoinNode(clusterID, node); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, node)
}

// removeNode 移除节点
func (h *Handlers) removeNode(c *gin.Context) {
	clusterID := c.Param("id")
	nodeID := c.Param("nodeId")

	if err := h.manager.RemoveNode(clusterID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "节点已移除"})
}

// promoteNode 提升节点为master
func (h *Handlers) promoteNode(c *gin.Context) {
	clusterID := c.Param("id")
	nodeID := c.Param("nodeId")

	if err := h.manager.PromoteNode(clusterID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "节点已提升为master"})
}

// setMaintenance 设置维护模式
func (h *Handlers) setMaintenance(c *gin.Context) {
	clusterID := c.Param("id")
	nodeID := c.Param("nodeId")

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.SetMaintenanceMode(clusterID, nodeID, req.Enable); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "维护模式已更新"})
}

// updateHeartbeat 更新心跳
func (h *Handlers) updateHeartbeat(c *gin.Context) {
	clusterID := c.Param("id")
	nodeID := c.Param("nodeId")

	if err := h.manager.UpdateNodeHeartbeat(clusterID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "心跳已更新"})
}

// startSync 启动同步
func (h *Handlers) startSync(c *gin.Context) {
	clusterID := c.Param("id")

	var req SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	job, err := h.manager.StartSync(clusterID, req.SourceNode, req.TargetNode, req.SourcePath, req.TargetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

// listSyncJobs 列出同步任务
func (h *Handlers) listSyncJobs(c *gin.Context) {
	clusterID := c.Param("id")

	jobs := h.manager.ListSyncJobs(clusterID)
	c.JSON(http.StatusOK, jobs)
}

// getSyncJob 获取同步任务
func (h *Handlers) getSyncJob(c *gin.Context) {
	jobID := c.Param("jobId")

	job, err := h.manager.GetSyncJob(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, job)
}

// selectNode 选择节点
func (h *Handlers) selectNode(c *gin.Context) {
	clusterID := c.Param("id")

	node, err := h.manager.SelectNodeForRequest(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, node)
}

// updateLBConfig 更新负载均衡配置
func (h *Handlers) updateLBConfig(c *gin.Context) {
	var req LoadBalancerConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	h.manager.lbConfig = &req
	c.JSON(http.StatusOK, gin.H{"message": "负载均衡配置已更新"})
}

// failover 故障转移
func (h *Handlers) failover(c *gin.Context) {
	clusterID := c.Param("id")

	var req FailoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// 获取集群
	cluster, err := h.manager.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 如果自动选择目标节点
	if req.AutoSelect {
		// 选择一个健康的worker节点
		for _, node := range cluster.Nodes {
			if node.ID != req.FailedNode && node.Status == NodeOnline && node.Role == RoleWorker {
				req.TargetNode = node.ID
				break
			}
		}
	}

	if req.TargetNode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未指定目标节点"})
		return
	}

	// 执行故障转移
	if err := h.manager.PromoteNode(clusterID, req.TargetNode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "故障转移完成",
		"new_master":  req.TargetNode,
	})
}

// rebalance 重新平衡
func (h *Handlers) rebalance(c *gin.Context) {
	clusterID := c.Param("id")

	var req RebalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认dry_run为true
		req.DryRun = true
	}

	cluster, err := h.manager.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 计算重新平衡动作
	actions := make([]*RebalanceAction, 0)
	avgUsage := 0.0
	totalStorage := 0.0
	usedStorage := 0.0

	for _, node := range cluster.Nodes {
		totalStorage += node.StorageTB
		usedStorage += node.UsedStorageTB
	}
	avgUsage = usedStorage / totalStorage

	// 找出需要迁移的数据
	for _, node := range cluster.Nodes {
		usage := node.UsedStorageTB / node.StorageTB
		if usage > avgUsage*1.2 { // 超过平均值20%
			// 计算需要迁移的数据量
			excess := node.UsedStorageTB - (node.StorageTB * avgUsage)
			if excess > 0 {
				actions = append(actions, &RebalanceAction{
					Type:       "migrate",
					SourceNode: node.ID,
					Path:       "/data",
					SizeBytes:  int64(excess * 1024 * 1024 * 1024), // TB to bytes
					Priority:   1,
				})
			}
		}
	}

	result := &RebalanceResult{
		ClusterID:     clusterID,
		DryRun:        req.DryRun,
		Actions:       actions,
		EstimatedTime: len(actions) * 300, // 每个动作估计5分钟
	}

	c.JSON(http.StatusOK, result)
}
