// Package lxcha HTTP 处理器
// 提供 LXC HA 故障转移管理的 REST API
package lxcha

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler LXC HA 故障转移管理 API 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建 API 处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由到指定路由组
// 路由前缀: /api/v1/lxcha
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/api/v1/lxcha")
	{
		// 节点管理
		g.GET("/nodes", h.listNodes)
		g.GET("/nodes/:nodeId", h.getNode)
		g.POST("/nodes", h.registerNode)
		g.DELETE("/nodes/:nodeId", h.removeNode)
		g.POST("/nodes/:nodeId/heartbeat", h.heartbeat)

		// 容器管理
		g.GET("/containers", h.listContainers)
		g.GET("/containers/:containerId", h.getContainer)
		g.POST("/containers/register", h.registerContainer)
		g.DELETE("/containers/:containerId", h.unregisterContainer)
		g.PUT("/containers/:containerId/state", h.updateState)

		// 策略管理
		g.GET("/containers/:containerId/policy", h.getPolicy)
		g.PUT("/containers/:containerId/policy", h.updatePolicy)

		// 故障转移
		g.POST("/failover/trigger", h.triggerFailover)
		g.GET("/failover/state/:containerId", h.getFailoverState)
		g.GET("/failover/events", h.listFailoverEvents)
		g.GET("/failover/events/:containerId", h.getContainerFailoverEvents)
		g.GET("/failover/history", h.getHistory)
		g.POST("/failover/auto", h.autoFailover)

		// 容器迁移
		g.POST("/migrate", h.migrateContainer)

		// 健康
		g.GET("/health/check", h.healthCheck)

		// IP 管理
		g.GET("/ip/reservations", h.listIPReservations)
		g.POST("/ip/reserve", h.reserveIP)
		g.DELETE("/ip/release/:ip", h.releaseIP)
		g.GET("/ip/check", h.checkIPConflict)

		// 状态总览
		g.GET("/status", h.getStatus)
	}
}

// ========== 节点管理 ==========

// listNodes 列出所有 HA 节点
// GET /api/v1/lxcha/nodes
func (h *Handler) listNodes(c *gin.Context) {
	nodes := h.service.GetNodes()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: nodes})
}

// getNode 获取单个节点信息
// GET /api/v1/lxcha/nodes/:nodeId
func (h *Handler) getNode(c *gin.Context) {
	nodeID := c.Param("nodeId")
	node, err := h.service.GetNode(nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: node})
}

// registerNode 注册 HA 节点
// POST /api/v1/lxcha/nodes
func (h *Handler) registerNode(c *gin.Context) {
	var node HANode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	if err := h.service.RegisterNode(&node); err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, APIResponse{Success: true, Message: "节点注册成功", Data: node})
}

// removeNode 移除 HA 节点
// DELETE /api/v1/lxcha/nodes/:nodeId
func (h *Handler) removeNode(c *gin.Context) {
	nodeID := c.Param("nodeId")
	if err := h.service.RemoveNode(nodeID); err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "节点已移除"})
}

// heartbeat 更新节点心跳
// POST /api/v1/lxcha/nodes/:nodeId/heartbeat
func (h *Handler) heartbeat(c *gin.Context) {
	nodeID := c.Param("nodeId")
	if err := h.service.UpdateNodeHeartbeat(nodeID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "心跳已更新"})
}

// ========== 容器管理 ==========

// listContainers 列出所有 HA 容器
// GET /api/v1/lxcha/containers
func (h *Handler) listContainers(c *gin.Context) {
	containers := h.service.GetContainers()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: containers})
}

// getContainer 获取单个容器信息
// GET /api/v1/lxcha/containers/:containerId
func (h *Handler) getContainer(c *gin.Context) {
	containerID := c.Param("containerId")
	container, err := h.service.GetContainer(containerID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: container})
}

// registerContainer 注册容器到 HA 管理
// POST /api/v1/lxcha/containers/register
func (h *Handler) registerContainer(c *gin.Context) {
	var req RegisterContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	container, err := h.service.RegisterContainer(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, APIResponse{Success: true, Message: "容器已注册到 HA", Data: container})
}

// unregisterContainer 取消容器 HA 注册
// DELETE /api/v1/lxcha/containers/:containerId
func (h *Handler) unregisterContainer(c *gin.Context) {
	containerID := c.Param("containerId")
	if err := h.service.UnregisterContainer(containerID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "容器已取消 HA 注册"})
}

// updateState 更新容器运行状态
// PUT /api/v1/lxcha/containers/:containerId/state
func (h *Handler) updateState(c *gin.Context) {
	containerID := c.Param("containerId")
	var body struct {
		State ContainerState `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	if err := h.service.UpdateContainerState(containerID, body.State); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "容器状态已更新"})
}

// ========== 策略管理 ==========

// getPolicy 获取容器故障转移策略
// GET /api/v1/lxcha/containers/:containerId/policy
func (h *Handler) getPolicy(c *gin.Context) {
	containerID := c.Param("containerId")
	policy, err := h.service.GetPolicy(containerID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: policy})
}

// updatePolicy 更新故障转移策略
// PUT /api/v1/lxcha/containers/:containerId/policy
func (h *Handler) updatePolicy(c *gin.Context) {
	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	req.ContainerID = c.Param("containerId")
	policy, err := h.service.UpdatePolicy(&req)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "策略已更新", Data: policy})
}

// ========== 故障转移 ==========

// triggerFailover 手动触发故障转移
// POST /api/v1/lxcha/failover/trigger
func (h *Handler) triggerFailover(c *gin.Context) {
	var req TriggerFailoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	result, err := h.service.TriggerFailover(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "故障转移完成", Data: result})
}

// getFailoverState 获取容器故障转移状态
// GET /api/v1/lxcha/failover/state/:containerId
func (h *Handler) getFailoverState(c *gin.Context) {
	containerID := c.Param("containerId")
	state, err := h.service.GetFailoverState(containerID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: state})
}

// listFailoverEvents 列出所有故障转移事件
// GET /api/v1/lxcha/failover/events
func (h *Handler) listFailoverEvents(c *gin.Context) {
	events := h.service.GetFailoverEvents("")
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: events})
}

// getContainerFailoverEvents 获取指定容器的故障转移事件
// GET /api/v1/lxcha/failover/events/:containerId
func (h *Handler) getContainerFailoverEvents(c *gin.Context) {
	containerID := c.Param("containerId")
	events := h.service.GetFailoverEvents(containerID)
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: events})
}

// getHistory 获取故障转移历史
// GET /api/v1/lxcha/failover/history
func (h *Handler) getHistory(c *gin.Context) {
	history := h.service.GetHistory()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: history})
}

// autoFailover 自动故障转移（内部触发）
// POST /api/v1/lxcha/failover/auto
func (h *Handler) autoFailover(c *gin.Context) {
	var req struct {
		ContainerID  string `json:"containerId" binding:"required"`
		FailedNodeID string `json:"failedNodeId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	result, err := h.service.AutoFailover(req.ContainerID, req.FailedNodeID)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "自动故障转移完成", Data: result})
}

// ========== 容器迁移 ==========

// migrateContainer 执行容器迁移
// POST /api/v1/lxcha/migrate
func (h *Handler) migrateContainer(c *gin.Context) {
	var req MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	result, err := h.service.MigrateContainer(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "迁移完成", Data: result})
}

// ========== 健康检查 ==========

// healthCheck 执行节点健康检查
// GET /api/v1/lxcha/health/check
func (h *Handler) healthCheck(c *gin.Context) {
	failedNodes := h.service.CheckNodeHealth()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: failedNodes})
}

// ========== IP 管理 ==========

// listIPReservations 列出所有 IP 预留
// GET /api/v1/lxcha/ip/reservations
func (h *Handler) listIPReservations(c *gin.Context) {
	reservations := h.service.GetIPReservations()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: reservations})
}

// reserveIP 预留静态 IP
// POST /api/v1/lxcha/ip/reserve
func (h *Handler) reserveIP(c *gin.Context) {
	var req ReserveIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "请求参数无效: " + err.Error()})
		return
	}
	reservation, err := h.service.ReserveIP(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, APIResponse{Success: true, Message: "IP 预留成功", Data: reservation})
}

// releaseIP 释放 IP 预留
// DELETE /api/v1/lxcha/ip/release/:ip
func (h *Handler) releaseIP(c *gin.Context) {
	ip := c.Param("ip")
	if err := h.service.ReleaseIP(ip); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "IP 预留已释放"})
}

// checkIPConflict 检查 IP 冲突
// GET /api/v1/lxcha/ip/check?ip=...&containerId=...&nodeId=...
func (h *Handler) checkIPConflict(c *gin.Context) {
	ip := c.Query("ip")
	containerID := c.Query("containerId")
	nodeID := c.Query("nodeId")
	if ip == "" || containerID == "" || nodeID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "参数 ip, containerId, nodeId 均为必填"})
		return
	}
	conflict := h.service.CheckIPConflict(ip, containerID, nodeID)
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]bool{"conflict": conflict}})
}

// ========== 状态总览 ==========

// getStatus 获取 HA 集群状态总览
// GET /api/v1/lxcha/status
func (h *Handler) getStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: status})
}