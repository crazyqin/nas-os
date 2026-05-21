// Package clustermgr 提供分布式集群管理功能
package clustermgr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 集群管理 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cm := r.Group("/cluster-mgr")
	{
		// 集群信息
		cm.GET("/cluster", h.getCluster)
		cm.GET("/cluster/stats", h.getClusterStats)

		// 节点管理
		cm.GET("/nodes", h.listNodes)
		cm.GET("/nodes/:id", h.getNode)
		cm.POST("/nodes/join", h.joinNode)
		cm.POST("/nodes/leave", h.leaveNode)
		cm.POST("/nodes/heartbeat", h.heartbeat)

		// 服务发现
		cm.GET("/services", h.listServices)
		cm.GET("/services/:id", h.getService)
		cm.POST("/services/register", h.registerService)
		cm.DELETE("/services/:id", h.deregisterService)
		cm.GET("/services/healthy", h.getHealthyServices)
		cm.GET("/services/name/:name", h.getServicesByName)

		// 负载均衡
		cm.POST("/balance/select", h.selectNode)
		cm.GET("/balance/strategy", h.getStrategy)
		cm.PUT("/balance/strategy", h.updateStrategy)

		// 配置管理
		cm.GET("/config", h.getConfig)
		cm.PUT("/config", h.updateConfig)

		// 健康检查
		cm.GET("/health", h.healthCheck)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getCluster 获取集群信息.
func (h *Handlers) getCluster(c *gin.Context) {
	cluster := h.manager.GetCluster()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cluster,
	})
}

// getClusterStats 获取集群统计.
func (h *Handlers) getClusterStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// listNodes 列出所有节点.
func (h *Handlers) listNodes(c *gin.Context) {
	nodes := h.manager.ListNodes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(nodes),
			"nodes": nodes,
		},
	})
}

// getNode 获取节点信息.
func (h *Handlers) getNode(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.manager.GetNode(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    node,
	})
}

// joinNode 节点加入集群.
func (h *Handlers) joinNode(c *gin.Context) {
	var req JoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求: " + err.Error(),
		})
		return
	}

	// 验证必填字段
	if req.NodeID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "节点ID不能为空",
		})
		return
	}
	if req.Address == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "节点地址不能为空",
		})
		return
	}

	// 设置默认值
	if req.Weight <= 0 {
		req.Weight = 1
	}
	if req.MaxConns <= 0 {
		req.MaxConns = 1000
	}

	result, err := h.manager.Join(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: result.Message,
		Data:    result,
	})
}

// leaveNode 节点离开集群.
func (h *Handlers) leaveNode(c *gin.Context) {
	var req LeaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求: " + err.Error(),
		})
		return
	}

	if req.NodeID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "节点ID不能为空",
		})
		return
	}

	result, err := h.manager.Leave(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: result.Message,
		Data:    result,
	})
}

// heartbeat 心跳.
func (h *Handlers) heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求: " + err.Error(),
		})
		return
	}

	if req.NodeID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "节点ID不能为空",
		})
		return
	}

	result, err := h.manager.Heartbeat(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: result.Message,
		Data:    result,
	})
}

// listServices 列出所有服务.
func (h *Handlers) listServices(c *gin.Context) {
	services := h.manager.ListServices()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(services),
			"services": services,
		},
	})
}

// getService 获取服务信息.
func (h *Handlers) getService(c *gin.Context) {
	id := c.Param("id")
	service, ok := h.manager.GetService(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "服务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    service,
	})
}

// registerService 注册服务.
func (h *Handlers) registerService(c *gin.Context) {
	var service ServiceInfo
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求: " + err.Error(),
		})
		return
	}

	if service.Name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "服务名称不能为空",
		})
		return
	}
	if service.Address == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "服务地址不能为空",
		})
		return
	}
	if service.Port <= 0 || service.Port > 65535 {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的端口号",
		})
		return
	}

	// 设置默认协议
	if service.Protocol == "" {
		service.Protocol = ProtocolHTTP
	}

	if err := h.manager.RegisterService(&service); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "服务注册成功",
		Data:    &service,
	})
}

// deregisterService 注销服务.
func (h *Handlers) deregisterService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "服务ID不能为空",
		})
		return
	}

	if err := h.manager.DeregisterService(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "服务注销成功",
	})
}

// getHealthyServices 获取健康的服务.
func (h *Handlers) getHealthyServices(c *gin.Context) {
	services := h.manager.GetHealthyServices()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(services),
			"services": services,
		},
	})
}

// getServicesByName 按名称获取服务.
func (h *Handlers) getServicesByName(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "服务名称不能为空",
		})
		return
	}

	services := h.manager.GetServicesByName(name)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"name":     name,
			"total":    len(services),
			"services": services,
		},
	})
}

// selectNodeRequest 选择节点请求.
type selectNodeRequest struct {
	Strategy LoadBalanceStrategy `json:"strategy"` // 负载均衡策略
	Key      string             `json:"key"`      // 会话保持键（可选）
}

// selectNode 选择节点.
func (h *Handlers) selectNode(c *gin.Context) {
	var req selectNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认策略
		req.Strategy = h.manager.GetConfig().LoadBalanceStrategy
	}

	if req.Strategy == "" {
		req.Strategy = h.manager.GetConfig().LoadBalanceStrategy
	}

	node, err := h.manager.SelectNode(req.Strategy, req.Key)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    node,
	})
}

// getStrategy 获取负载均衡策略.
func (h *Handlers) getStrategy(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"strategy":     config.LoadBalanceStrategy,
			"stickySession": config.StickySession,
		},
	})
}

// updateStrategyRequest 更新策略请求.
type updateStrategyRequest struct {
	Strategy     LoadBalanceStrategy `json:"strategy"`     // 负载均衡策略
	StickySession *bool              `json:"stickySession"` // 会话保持
}

// updateStrategy 更新负载均衡策略.
func (h *Handlers) updateStrategy(c *gin.Context) {
	var req updateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求: " + err.Error(),
		})
		return
	}

	config := h.manager.GetConfig()
	if req.Strategy != "" {
		config.LoadBalanceStrategy = req.Strategy
	}
	if req.StickySession != nil {
		config.StickySession = *req.StickySession
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "策略更新成功",
		Data: gin.H{
			"strategy":     config.LoadBalanceStrategy,
			"stickySession": config.StickySession,
		},
	})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// updateConfigRequest 更新配置请求.
type updateConfigRequest struct {
	HeartbeatInterval   *int                 `json:"heartbeatInterval,omitempty"`   // 心跳间隔（秒）
	HeartbeatTimeout    *int                 `json:"heartbeatTimeout,omitempty"`    // 心跳超时（秒）
	FailoverEnabled     *bool                `json:"failoverEnabled,omitempty"`     // 启用故障转移
	FailoverTimeout     *int                 `json:"failoverTimeout,omitempty"`     // 故障转移超时（秒）
	MaxFailoverAttempts *int                 `json:"maxFailoverAttempts,omitempty"` // 最大故障转移尝试次数
	LoadBalanceStrategy *LoadBalanceStrategy `json:"loadBalanceStrategy,omitempty"` // 负载均衡策略
	StickySession       *bool                `json:"stickySession,omitempty"`       // 会话保持
	DiscoveryEnabled    *bool                `json:"discoveryEnabled,omitempty"`    // 启用服务发现
	DiscoveryInterval   *int                 `json:"discoveryInterval,omitempty"`   // 发现间隔（秒）
	HealthCheckInterval *int                 `json:"healthCheckInterval,omitempty"` // 健康检查间隔（秒）
	MaxNodes            *int                 `json:"maxNodes,omitempty"`            // 最大节点数
	AutoRemoveFailed    *bool                `json:"autoRemoveFailed,omitempty"`    // 自动移除故障节点
	FailedNodeTimeout   *int                 `json:"failedNodeTimeout,omitempty"`   // 故障节点超时（秒）
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "无效的请求: " + err.Error(),
		})
		return
	}

	config := h.manager.GetConfig()

	// 更新配置字段
	if req.HeartbeatInterval != nil {
		config.HeartbeatInterval = time.Duration(*req.HeartbeatInterval) * time.Second
	}
	if req.HeartbeatTimeout != nil {
		config.HeartbeatTimeout = time.Duration(*req.HeartbeatTimeout) * time.Second
	}
	if req.FailoverEnabled != nil {
		config.FailoverEnabled = *req.FailoverEnabled
	}
	if req.FailoverTimeout != nil {
		config.FailoverTimeout = time.Duration(*req.FailoverTimeout) * time.Second
	}
	if req.MaxFailoverAttempts != nil {
		config.MaxFailoverAttempts = *req.MaxFailoverAttempts
	}
	if req.LoadBalanceStrategy != nil {
		config.LoadBalanceStrategy = *req.LoadBalanceStrategy
	}
	if req.StickySession != nil {
		config.StickySession = *req.StickySession
	}
	if req.DiscoveryEnabled != nil {
		config.DiscoveryEnabled = *req.DiscoveryEnabled
	}
	if req.DiscoveryInterval != nil {
		config.DiscoveryInterval = time.Duration(*req.DiscoveryInterval) * time.Second
	}
	if req.HealthCheckInterval != nil {
		config.HealthCheckInterval = time.Duration(*req.HealthCheckInterval) * time.Second
	}
	if req.MaxNodes != nil {
		config.MaxNodes = *req.MaxNodes
	}
	if req.AutoRemoveFailed != nil {
		config.AutoRemoveFailed = *req.AutoRemoveFailed
	}
	if req.FailedNodeTimeout != nil {
		config.FailedNodeTimeout = time.Duration(*req.FailedNodeTimeout) * time.Second
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "配置更新成功",
		Data:    config,
	})
}

// healthCheck 健康检查.
func (h *Handlers) healthCheck(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "healthy",
		Data: gin.H{
			"status":    "ok",
			"clusterId": stats["clusterId"],
			"uptime":    stats["uptime"],
		},
	})
}
