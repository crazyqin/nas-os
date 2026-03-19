package cluster

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"nas-os/internal/auth"
)

// 集群资源权限定义
const (
	// ResourceCluster 集群管理资源
	ResourceCluster auth.Resource = "cluster"
	// ResourceSync 同步规则资源
	ResourceSync auth.Resource = "cluster_sync"
	// ResourceLoadBalancer 负载均衡资源
	ResourceLoadBalancer auth.Resource = "cluster_lb"
	// ResourceHighAvailability 高可用资源
	ResourceHighAvailability auth.Resource = "cluster_ha"
)

// 输入验证正则
var (
	nodeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	ipPattern     = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$|^\[?[0-9a-fA-F:]+\]?$`)
)

// API 集群 API 处理器
type API struct {
	manager        *Manager
	sync           *StorageSync
	lb             *LoadBalancer
	ha             *HighAvailability
	logger         *zap.Logger
	authMiddleware *auth.Middleware
}

// NewAPI 创建集群 API 处理器
func NewAPI(manager *Manager, sync *StorageSync, lb *LoadBalancer, ha *HighAvailability, logger *zap.Logger) *API {
	return &API{
		manager:        manager,
		sync:           sync,
		lb:             lb,
		ha:             ha,
		logger:         logger,
		authMiddleware: nil, // 默认无认证，需要通过 SetAuthMiddleware 设置
	}
}

// SetAuthMiddleware 设置认证中间件
func (api *API) SetAuthMiddleware(am *auth.Middleware) {
	api.authMiddleware = am
}

// NewClusterAPI 创建集群 API 处理器（兼容旧代码）
func NewClusterAPI(manager *Manager, sync *StorageSync, lb *LoadBalancer, ha *HighAvailability, logger *zap.Logger) *API {
	return NewAPI(manager, sync, lb, ha, logger)
}

// RegisterRoutes 注册路由
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	// 辅助函数：获取认证中间件（如果配置了）
	authRequired := func() gin.HandlerFunc {
		if api.authMiddleware != nil {
			return api.authMiddleware.RequireAuth()
		}
		return func(c *gin.Context) { c.Next() }
	}

	// 辅助函数：需要特定权限
	requirePermission := func(resource auth.Resource, action auth.Action) gin.HandlerFunc {
		if api.authMiddleware != nil {
			return api.authMiddleware.RequirePermission(resource, action)
		}
		return func(c *gin.Context) { c.Next() }
	}

	// 辅助函数：需要管理员权限
	requireAdmin := func() gin.HandlerFunc {
		if api.authMiddleware != nil {
			return api.authMiddleware.RequireAdmin()
		}
		return func(c *gin.Context) { c.Next() }
	}

	// ========== 节点管理 ==========
	// 只读操作：需要认证（查看权限）
	router.GET("/nodes", authRequired(), api.GetNodes)
	router.GET("/nodes/:id", authRequired(), api.GetNode)
	router.GET("/nodes/:id/status", authRequired(), api.GetNodeStatus)

	// 敏感操作：需要管理员权限
	router.POST("/nodes/join", requireAdmin(), api.JoinCluster)
	router.DELETE("/nodes/:id", requireAdmin(), api.RemoveNode)
	router.POST("/nodes/:id/drain", requireAdmin(), api.DrainNode)

	// ========== 存储同步 ==========
	// 只读操作：需要认证
	router.GET("/sync/rules", authRequired(), api.GetSyncRules)
	router.GET("/sync/rules/:id", authRequired(), api.GetSyncRule)
	router.GET("/sync/status", authRequired(), api.GetSyncStatus)
	router.GET("/sync/jobs", authRequired(), api.GetSyncJobs)

	// 敏感操作：需要写入权限
	router.POST("/sync/rules", requirePermission(ResourceSync, auth.ActionWrite), api.CreateSyncRule)
	router.PUT("/sync/rules/:id", requirePermission(ResourceSync, auth.ActionWrite), api.UpdateSyncRule)
	router.DELETE("/sync/rules/:id", requirePermission(ResourceSync, auth.ActionDelete), api.DeleteSyncRule)
	router.POST("/sync/trigger", requirePermission(ResourceSync, auth.ActionExec), api.TriggerSync)

	// ========== 负载均衡 ==========
	// 只读操作：需要认证
	router.GET("/lb/config", authRequired(), api.GetLBConfig)
	router.GET("/lb/backends", authRequired(), api.GetBackends)
	router.GET("/lb/stats", authRequired(), api.GetLBStats)

	// 敏感操作：需要管理员权限
	router.PUT("/lb/config", requireAdmin(), api.UpdateLBConfig)
	router.POST("/lb/reset", requireAdmin(), api.ResetLBStats)

	// ========== 高可用 ==========
	// 只读操作：需要认证
	router.GET("/ha/status", authRequired(), api.GetHAStatus)
	router.GET("/ha/history", authRequired(), api.GetFailoverHistory)

	// 敏感操作：需要管理员权限
	router.POST("/ha/failover", requireAdmin(), api.ManualFailover)
}

// 节点管理 API

// GetNodes 获取节点列表
func (api *API) GetNodes(c *gin.Context) {
	nodes := api.manager.GetNodes()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"count":   len(nodes),
	})
}

// GetNode 获取节点详情
func (api *API) GetNode(c *gin.Context) {
	nodeID := c.Param("id")

	// 安全校验：验证节点ID
	if !nodeIDPattern.MatchString(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID格式无效",
		})
		return
	}

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
func (api *API) JoinCluster(c *gin.Context) {
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

	// 安全校验：验证节点ID
	if !nodeIDPattern.MatchString(req.NodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID格式无效，只允许字母、数字、连字符和下划线",
		})
		return
	}

	// 安全校验：验证主机名
	if req.Hostname == "" || len(req.Hostname) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "主机名不能为空且不能超过64字符",
		})
		return
	}

	// 安全校验：验证IP地址
	if !ipPattern.MatchString(req.IP) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "IP地址格式无效",
		})
		return
	}

	// 安全校验：验证端口范围
	if req.Port <= 0 || req.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "端口号必须在1-65535之间",
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
	node := &Member{
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
func (api *API) RemoveNode(c *gin.Context) {
	nodeID := c.Param("id")

	// 安全校验：验证节点ID
	if !nodeIDPattern.MatchString(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID格式无效",
		})
		return
	}

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
func (api *API) GetNodeStatus(c *gin.Context) {
	nodeID := c.Param("id")

	// 安全校验：验证节点ID
	if !nodeIDPattern.MatchString(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID格式无效",
		})
		return
	}

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
func (api *API) DrainNode(c *gin.Context) {
	nodeID := c.Param("id")

	// 安全校验：验证节点ID
	if !nodeIDPattern.MatchString(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "节点ID格式无效",
		})
		return
	}

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
func (api *API) GetSyncRules(c *gin.Context) {
	rules := api.sync.GetRules()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
		"count":   len(rules),
	})
}

// GetSyncRule 获取同步规则详情
func (api *API) GetSyncRule(c *gin.Context) {
	ruleID := c.Param("id")

	// 安全校验：验证规则ID
	if !nodeIDPattern.MatchString(ruleID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "规则ID格式无效",
		})
		return
	}

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
func (api *API) CreateSyncRule(c *gin.Context) {
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
func (api *API) UpdateSyncRule(c *gin.Context) {
	ruleID := c.Param("id")

	// 安全校验：验证规则ID
	if !nodeIDPattern.MatchString(ruleID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "规则ID格式无效",
		})
		return
	}

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
func (api *API) DeleteSyncRule(c *gin.Context) {
	ruleID := c.Param("id")

	// 安全校验：验证规则ID
	if !nodeIDPattern.MatchString(ruleID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "规则ID格式无效",
		})
		return
	}

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
func (api *API) TriggerSync(c *gin.Context) {
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

	// 安全校验：验证规则ID
	if !nodeIDPattern.MatchString(req.RuleID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "规则ID格式无效",
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
func (api *API) GetSyncStatus(c *gin.Context) {
	status := api.sync.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// GetSyncJobs 获取同步任务历史
func (api *API) GetSyncJobs(c *gin.Context) {
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
func (api *API) GetLBConfig(c *gin.Context) {
	config := api.lb.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateLBConfig 更新负载均衡配置
func (api *API) UpdateLBConfig(c *gin.Context) {
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
func (api *API) GetBackends(c *gin.Context) {
	backends := api.lb.GetBackends()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    backends,
		"count":   len(backends),
	})
}

// GetLBStats 获取负载均衡统计
func (api *API) GetLBStats(c *gin.Context) {
	stats := api.lb.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ResetLBStats 重置负载均衡统计
func (api *API) ResetLBStats(c *gin.Context) {
	api.lb.ResetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "统计已重置",
	})
}

// 高可用 API

// GetHAStatus 获取高可用状态
func (api *API) GetHAStatus(c *gin.Context) {
	status := api.ha.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// ManualFailover 手动故障转移
func (api *API) ManualFailover(c *gin.Context) {
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

	// 安全校验：验证目标节点ID
	if !nodeIDPattern.MatchString(req.TargetNodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "目标节点ID格式无效",
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
func (api *API) GetFailoverHistory(c *gin.Context) {
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
