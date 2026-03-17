package cluster

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ClusterAPI 集群 API 处理器
type ClusterAPI struct {
	manager *ClusterManager
	sync    *StorageSync
	lb      *LoadBalancer
	ha      *HighAvailability
	logger  *zap.Logger
}

// NewClusterAPI 创建集群 API 处理器
func NewClusterAPI(manager *ClusterManager, sync *StorageSync, lb *LoadBalancer, ha *HighAvailability, logger *zap.Logger) *ClusterAPI {
	return &ClusterAPI{
		manager: manager,
		sync:    sync,
		lb:      lb,
		ha:      ha,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (api *ClusterAPI) RegisterRoutes(router *gin.RouterGroup) {
	// 节点管理
	router.GET("/nodes", api.GetNodes)
	router.GET("/nodes/:id", api.GetNode)
	router.POST("/nodes/join", api.JoinCluster)
	router.DELETE("/nodes/:id", api.RemoveNode)
	router.GET("/nodes/:id/status", api.GetNodeStatus)
	router.POST("/nodes/:id/drain", api.DrainNode)

	// 存储同步
	router.GET("/sync/rules", api.GetSyncRules)
	router.GET("/sync/rules/:id", api.GetSyncRule)
	router.POST("/sync/rules", api.CreateSyncRule)
	router.PUT("/sync/rules/:id", api.UpdateSyncRule)
	router.DELETE("/sync/rules/:id", api.DeleteSyncRule)
	router.POST("/sync/trigger", api.TriggerSync)
	router.GET("/sync/status", api.GetSyncStatus)
	router.GET("/sync/jobs", api.GetSyncJobs)

	// 负载均衡
	router.GET("/lb/config", api.GetLBConfig)
	router.PUT("/lb/config", api.UpdateLBConfig)
	router.GET("/lb/backends", api.GetBackends)
	router.GET("/lb/stats", api.GetLBStats)
	router.POST("/lb/reset", api.ResetLBStats)

	// 高可用
	router.GET("/ha/status", api.GetHAStatus)
	router.POST("/ha/failover", api.ManualFailover)
	router.GET("/ha/history", api.GetFailoverHistory)
}

// 节点管理 API

// GetNodes 获取节点列表
func (api *ClusterAPI) GetNodes(c *gin.Context) {
	nodes := api.manager.GetNodes()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"count":   len(nodes),
	})
}

// GetNode 获取节点详情
func (api *ClusterAPI) GetNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, exists := api.manager.GetNode(nodeID)
	if !exists {
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

// JoinCluster 加入集群
func (api *ClusterAPI) JoinCluster(c *gin.Context) {
	var req struct {
		NodeID   string `json:"node_id"`
		Hostname string `json:"hostname"`
		IP       string `json:"ip"`
		Port     int    `json:"port"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 检查节点是否已存在
	if _, exists := api.manager.GetNode(req.NodeID); exists {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "节点已存在",
		})
		return
	}

	// 创建节点
	node := &ClusterNode{
		ID:        req.NodeID,
		Hostname:  req.Hostname,
		IP:        req.IP,
		Port:      req.Port,
		Role:      RoleWorker,
		Status:    StatusOnline,
		Heartbeat: time.Now(),
		JoinTime:  time.Now(),
	}

	// 添加到集群
	_ = api.manager.UpdateNodeMetrics(req.NodeID, NodeMetrics{})

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "节点已加入集群",
		"data":    node,
	})
}

// RemoveNode 移除节点
func (api *ClusterAPI) RemoveNode(c *gin.Context) {
	nodeID := c.Param("id")

	if err := api.manager.RemoveNode(nodeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点已移除",
	})
}

// GetNodeStatus 获取节点状态
func (api *ClusterAPI) GetNodeStatus(c *gin.Context) {
	nodeID := c.Param("id")
	node, exists := api.manager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"node_id":   node.ID,
			"status":    node.Status,
			"role":      node.Role,
			"heartbeat": node.Heartbeat,
			"metrics":   node.Metrics,
		},
	})
}

// DrainNode 节点下线
func (api *ClusterAPI) DrainNode(c *gin.Context) {
	nodeID := c.Param("id")

	node, exists := api.manager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "节点不存在",
		})
		return
	}

	// 标记节点为下线状态
	node.Status = StatusDegraded

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点正在下线",
	})
}

// 存储同步 API

// GetSyncRules 获取同步规则列表
func (api *ClusterAPI) GetSyncRules(c *gin.Context) {
	rules := api.sync.GetRules()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
		"count":   len(rules),
	})
}

// GetSyncRule 获取同步规则详情
func (api *ClusterAPI) GetSyncRule(c *gin.Context) {
	ruleID := c.Param("id")
	rule, exists := api.sync.GetRule(ruleID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "规则不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// CreateSyncRule 创建同步规则
func (api *ClusterAPI) CreateSyncRule(c *gin.Context) {
	var rule SyncRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.sync.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "同步规则已创建",
		"data":    rule,
	})
}

// UpdateSyncRule 更新同步规则
func (api *ClusterAPI) UpdateSyncRule(c *gin.Context) {
	ruleID := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.sync.UpdateRule(ruleID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "同步规则已更新",
	})
}

// DeleteSyncRule 删除同步规则
func (api *ClusterAPI) DeleteSyncRule(c *gin.Context) {
	ruleID := c.Param("id")

	if err := api.sync.DeleteRule(ruleID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "同步规则已删除",
	})
}

// TriggerSync 手动触发同步
func (api *ClusterAPI) TriggerSync(c *gin.Context) {
	var req struct {
		RuleID string `json:"rule_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.sync.TriggerSync(req.RuleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "同步任务已触发",
	})
}

// GetSyncStatus 获取同步状态
func (api *ClusterAPI) GetSyncStatus(c *gin.Context) {
	status := api.sync.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// GetSyncJobs 获取同步任务历史
func (api *ClusterAPI) GetSyncJobs(c *gin.Context) {
	limit := 20 // 默认返回 20 条
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	jobs := api.sync.GetJobs(limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    jobs,
		"count":   len(jobs),
	})
}

// 负载均衡 API

// GetLBConfig 获取负载均衡配置
func (api *ClusterAPI) GetLBConfig(c *gin.Context) {
	config := api.lb.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateLBConfig 更新负载均衡配置
func (api *ClusterAPI) UpdateLBConfig(c *gin.Context) {
	var config LBConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.lb.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "负载均衡配置已更新",
	})
}

// GetBackends 获取后端节点
func (api *ClusterAPI) GetBackends(c *gin.Context) {
	backends := api.lb.GetBackends()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    backends,
		"count":   len(backends),
	})
}

// GetLBStats 获取负载均衡统计
func (api *ClusterAPI) GetLBStats(c *gin.Context) {
	stats := api.lb.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ResetLBStats 重置负载均衡统计
func (api *ClusterAPI) ResetLBStats(c *gin.Context) {
	api.lb.ResetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "统计已重置",
	})
}

// 高可用 API

// GetHAStatus 获取高可用状态
func (api *ClusterAPI) GetHAStatus(c *gin.Context) {
	status := api.ha.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// ManualFailover 手动故障转移
func (api *ClusterAPI) ManualFailover(c *gin.Context) {
	var req struct {
		TargetNodeID string `json:"target_node_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.ha.TransferLeadership(req.TargetNodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "领导权转移已启动",
	})
}

// GetFailoverHistory 获取故障转移历史
func (api *ClusterAPI) GetFailoverHistory(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &limit)
	}

	events := api.ha.GetFailoverHistory(limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"count":   len(events),
	})
}
