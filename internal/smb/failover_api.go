package smb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// FailoverHandlers 故障转移API处理器
type FailoverHandlers struct {
	failover    *FailoverState
	sync        *StateSynchronizer
	health      *HealthChecker
}

// NewFailoverHandlers 创建故障转移API处理器
func NewFailoverHandlers(failover *FailoverState, sync *StateSynchronizer, health *HealthChecker) *FailoverHandlers {
	return &FailoverHandlers{
		failover: failover,
		sync:     sync,
		health:   health,
	}
}

// RegisterRoutes 注册故障转移API路由
func (h *FailoverHandlers) RegisterRoutes(api *gin.RouterGroup) {
	fo := api.Group("/smb/failover")
	{
		// 故障转移状态
		fo.GET("/status", h.getStatus)
		fo.GET("/nodes", h.listNodes)
		fo.POST("/nodes", h.registerNode)
		fo.POST("/initiate", h.initiateFailover)
		fo.GET("/events", h.getEvents)
		fo.GET("/sessions", h.listSessions)
		fo.GET("/sessions/:id", h.getSession)
		fo.GET("/sessions/client/:ip", h.getSessionsByClient)
		fo.GET("/sessions/user/:username", h.getSessionsByUser)
		fo.GET("/sessions/share/:name", h.getSessionsByShare)

		// 状态同步
		fo.GET("/sync/status", h.getSyncStatus)
		fo.GET("/sync/metrics", h.getSyncMetrics)
		fo.GET("/sync/active", h.getActiveSyncs)
		fo.GET("/sync/nodes", h.getSyncNodeStatus)
		fo.POST("/sync", h.handleSyncRequest)
		fo.POST("/sync/trigger", h.triggerSync)

		// 健康检查
		fo.GET("/health", h.getClusterHealth)
		fo.GET("/health/nodes", h.getAllNodeHealth)
		fo.GET("/health/nodes/:id", h.getNodeHealth)
		fo.GET("/health/stats", h.getHealthStats)
		fo.POST("/health/check/:id", h.performHealthCheck)

		// VIP管理
		fo.GET("/vip", h.getVIPStatus)
	}
}

// getStatus 获取故障转移状态
func (h *FailoverHandlers) getStatus(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	status := h.failover.GetStatus()
	c.JSON(http.StatusOK, Success(status))
}

// listNodes 列出所有集群节点
func (h *FailoverHandlers) listNodes(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	status := h.failover.GetStatus()
	c.JSON(http.StatusOK, Success(status.ClusterNodes))
}

// registerNode 注册新集群节点
func (h *FailoverHandlers) registerNode(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	var node NodeState
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, fmt.Sprintf("无效的节点数据: %v", err)))
		return
	}

	// 节点注册逻辑
	logInfo("通过API注册节点", "node_id", node.NodeID, "host", node.Host)
	c.JSON(http.StatusOK, Success(gin.H{"message": "节点已注册", "node_id": node.NodeID}))
}

// initiateFailover 手动触发故障转移
func (h *FailoverHandlers) initiateFailover(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	var req struct {
		TargetNode string `json:"target_node"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, fmt.Sprintf("无效的请求: %v", err)))
		return
	}

	if req.TargetNode == "" {
		c.JSON(http.StatusBadRequest, Error(400, "目标节点不能为空"))
		return
	}

	h.failover.triggerFailover(req.TargetNode)
	c.JSON(http.StatusOK, Success(gin.H{"message": "故障转移已触发", "target_node": req.TargetNode}))
}

// getEvents 获取故障转移事件
func (h *FailoverHandlers) getEvents(c *gin.Context) {
	// 返回故障转移事件列表
	c.JSON(http.StatusOK, Success([]FailoverEvent{}))
}

// listSessions 列出所有SMB会话
func (h *FailoverHandlers) listSessions(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	sessions := h.failover.ListSessions()
	c.JSON(http.StatusOK, Success(gin.H{"sessions": sessions, "total": len(sessions)}))
}

// getSession 获取单个会话
func (h *FailoverHandlers) getSession(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	sessionID := c.Param("id")
	session, err := h.failover.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, fmt.Sprintf("会话不存在: %s", sessionID)))
		return
	}

	c.JSON(http.StatusOK, Success(session))
}

// getSessionsByClient 按客户端IP获取会话
func (h *FailoverHandlers) getSessionsByClient(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	clientIP := c.Param("ip")
	sessions := h.failover.GetSessionsByClient(clientIP)
	c.JSON(http.StatusOK, Success(gin.H{"sessions": sessions, "total": len(sessions)}))
}

// getSessionsByUser 按用户名获取会话
func (h *FailoverHandlers) getSessionsByUser(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	username := c.Param("username")
	sessions := h.failover.GetSessionsByUser(username)
	c.JSON(http.StatusOK, Success(gin.H{"sessions": sessions, "total": len(sessions)}))
}

// getSessionsByShare 按共享名获取会话
func (h *FailoverHandlers) getSessionsByShare(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	shareName := c.Param("name")
	sessions := h.failover.GetSessionsByShare(shareName)
	c.JSON(http.StatusOK, Success(gin.H{"sessions": sessions, "total": len(sessions)}))
}

// getSyncStatus 获取同步状态
func (h *FailoverHandlers) getSyncStatus(c *gin.Context) {
	if h.sync == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "状态同步器未初始化"))
		return
	}

	status := gin.H{
		"running":        h.sync.IsRunning(),
		"last_sync_time": h.sync.GetLastSyncTime(),
		"node_count":     len(h.sync.GetNodeSyncStatus()),
	}

	c.JSON(http.StatusOK, Success(status))
}

// getSyncMetrics 获取同步指标
func (h *FailoverHandlers) getSyncMetrics(c *gin.Context) {
	if h.sync == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "状态同步器未初始化"))
		return
	}

	metrics := h.sync.GetSyncMetrics()
	c.JSON(http.StatusOK, Success(metrics))
}

// getActiveSyncs 获取活跃同步操作
func (h *FailoverHandlers) getActiveSyncs(c *gin.Context) {
	if h.sync == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "状态同步器未初始化"))
		return
	}

	syncs := h.sync.GetActiveSyncs()
	c.JSON(http.StatusOK, Success(syncs))
}

// getSyncNodeStatus 获取节点同步状态
func (h *FailoverHandlers) getSyncNodeStatus(c *gin.Context) {
	if h.sync == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "状态同步器未初始化"))
		return
	}

	nodes := h.sync.GetNodeSyncStatus()
	c.JSON(http.StatusOK, Success(nodes))
}

// handleSyncRequest 处理同步请求
func (h *FailoverHandlers) handleSyncRequest(c *gin.Context) {
	if h.sync == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "状态同步器未初始化"))
		return
	}

	var request SyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, fmt.Sprintf("无效的同步请求: %v", err)))
		return
	}

	response, err := h.sync.HandleSyncRequest(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, fmt.Sprintf("处理同步请求失败: %v", err)))
		return
	}

	c.JSON(http.StatusOK, Success(response))
}

// triggerSync 手动触发同步
func (h *FailoverHandlers) triggerSync(c *gin.Context) {
	if h.sync == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "状态同步器未初始化"))
		return
	}

	var req struct {
		TargetNode string `json:"target_node"`
		Type       string `json:"type"` // "full" | "incremental"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, fmt.Sprintf("无效的请求: %v", err)))
		return
	}

	if req.TargetNode == "" {
		c.JSON(http.StatusBadRequest, Error(400, "目标节点不能为空"))
		return
	}

	// 获取当前会话并同步
	sessions := h.failover.ListSessions()
	sessionData := make(map[string][]byte, len(sessions))
	for _, session := range sessions {
		data, _ := json.Marshal(session)
		sessionData[session.SessionID] = data
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.sync.SyncSessions(ctx, req.TargetNode, sessionData); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, fmt.Sprintf("同步失败: %v", err)))
		return
	}

	c.JSON(http.StatusOK, Success(gin.H{"message": "同步已触发", "target_node": req.TargetNode, "sessions": len(sessionData)}))
}

// getClusterHealth 获取集群健康状态
func (h *FailoverHandlers) getClusterHealth(c *gin.Context) {
	if h.health == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "健康检查器未初始化"))
		return
	}

	health := h.health.GetClusterHealth()
	c.JSON(http.StatusOK, Success(health))
}

// getAllNodeHealth 获取所有节点健康状态
func (h *FailoverHandlers) getAllNodeHealth(c *gin.Context) {
	if h.health == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "健康检查器未初始化"))
		return
	}

	nodes := h.health.GetAllNodeHealth()
	c.JSON(http.StatusOK, Success(nodes))
}

// getNodeHealth 获取单个节点健康状态
func (h *FailoverHandlers) getNodeHealth(c *gin.Context) {
	if h.health == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "健康检查器未初始化"))
		return
	}

	nodeID := c.Param("id")
	node, ok := h.health.GetNodeHealth(nodeID)
	if !ok {
		c.JSON(http.StatusNotFound, Error(404, fmt.Sprintf("节点不存在: %s", nodeID)))
		return
	}

	c.JSON(http.StatusOK, Success(node))
}

// getHealthStats 获取健康检查统计
func (h *FailoverHandlers) getHealthStats(c *gin.Context) {
	if h.health == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "健康检查器未初始化"))
		return
	}

	stats := h.health.GetHealthStats()
	c.JSON(http.StatusOK, Success(stats))
}

// performHealthCheck 执行即时健康检查
func (h *FailoverHandlers) performHealthCheck(c *gin.Context) {
	if h.health == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "健康检查器未初始化"))
		return
	}

	nodeID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := h.health.PerformImmediateCheck(ctx, nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, fmt.Sprintf("健康检查失败: %v", err)))
		return
	}

	c.JSON(http.StatusOK, Success(result))
}

// getVIPStatus 获取VIP状态
func (h *FailoverHandlers) getVIPStatus(c *gin.Context) {
	if h.failover == nil {
		c.JSON(http.StatusServiceUnavailable, Error(503, "故障转移未初始化"))
		return
	}

	status := h.failover.GetStatus()
	c.JSON(http.StatusOK, Success(gin.H{
		"cluster_ip": h.failover.config.ClusterIP,
		"is_primary": status.IsPrimary,
		"local_node": status.LocalNode,
	}))
}

// registerFailoverMetrics 注册故障转移监控指标
func (h *FailoverHandlers) registerFailoverMetrics(api *gin.RouterGroup) {
	api.GET("/smb/failover/metrics", func(c *gin.Context) {
		metrics := gin.H{
			"timestamp": time.Now(),
		}

		// 故障转移指标
		if h.failover != nil {
			status := h.failover.GetStatus()
			metrics["failover"] = gin.H{
				"enabled":         status.Enabled,
				"is_primary":      status.IsPrimary,
				"active_sessions": status.ActiveSessions,
				"failover_count":  status.FailoverCount,
				"healthy_count":   status.HealthyCount,
			}
		}

		// 同步指标
		if h.sync != nil {
			syncMetrics := h.sync.GetSyncMetrics()
			metrics["sync"] = gin.H{
				"total_syncs":      syncMetrics.TotalSyncs,
				"successful_syncs": syncMetrics.SuccessfulSyncs,
				"failed_syncs":     syncMetrics.FailedSyncs,
				"total_bytes":      syncMetrics.TotalBytes,
				"average_duration": syncMetrics.AverageDuration.String(),
			}
		}

		// 健康检查指标
		if h.health != nil {
			healthStats := h.health.GetHealthStats()
			clusterHealth := h.health.GetClusterHealth()
			metrics["health"] = gin.H{
				"total_checks":      healthStats.TotalChecks,
				"total_failures":    healthStats.TotalFailures,
				"average_latency":   healthStats.AverageLatency.String(),
				"cluster_healthy":   clusterHealth.ClusterHealthy,
				"healthy_nodes":     clusterHealth.HealthyNodes,
				"unhealthy_nodes":   clusterHealth.UnhealthyNodes,
			}
		}

		c.JSON(http.StatusOK, Success(metrics))
	})
}

// parseIntParam 解析整数参数
func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	if val := c.Query(name); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
