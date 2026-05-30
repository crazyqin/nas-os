// Package tailscale 提供 Tailscale VPN 零配置组网功能
// HTTP API handlers
package tailscale

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler Tailscale HTTP 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ts := rg.Group("/tailscale")
	{
		// 状态
		ts.GET("/status", h.GetStatus)

		// 节点管理
		ts.GET("/nodes", h.ListNodes)
		ts.GET("/nodes/:id", h.GetNode)
		ts.POST("/nodes/:id/approve", h.ApproveNode)

		// ACL 策略
		ts.GET("/acl", h.GetACL)
		ts.PUT("/acl", h.UpdateACL)

		// 子网路由
		ts.GET("/subnets", h.ListSubnets)
		ts.POST("/subnets", h.AddSubnet)
		ts.PUT("/subnets/:id/toggle", h.ToggleSubnet)

		// Exit Node
		ts.GET("/exit-nodes", h.ListExitNodes)
		ts.POST("/exit-nodes/:id/select", h.SelectExitNode)
		ts.POST("/exit-nodes/deselect", h.DeselectExitNode)

		// DNS 配置
		ts.GET("/dns", h.GetDNS)
		ts.PUT("/dns", h.UpdateDNS)

		// 认证密钥
		ts.GET("/authkeys", h.ListAuthKeys)
		ts.POST("/authkeys", h.CreateAuthKey)
		ts.DELETE("/authkeys/:id", h.RevokeAuthKey)

		// 流量统计
		ts.GET("/stats", h.GetStats)
	}
}

// ========== 状态 ==========

// GetStatus handles GET /api/v1/tailscale/status
// 获取 Tailscale 状态
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ========== 节点管理 ==========

// ListNodes handles GET /api/v1/tailscale/nodes
// 列出所有节点
func (h *Handler) ListNodes(c *gin.Context) {
	nodes := h.manager.GetNodes()
	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

// GetNode handles GET /api/v1/tailscale/nodes/:id
// 获取节点详情
func (h *Handler) GetNode(c *gin.Context) {
	id := c.Param("id")

	node, err := h.manager.GetNode(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, node)
}

// ApproveNode handles POST /api/v1/tailscale/nodes/:id/approve
// 批准节点
func (h *Handler) ApproveNode(c *gin.Context) {
	id := c.Param("id")

	var req ApproveNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ApproveNode(id, req.Approved); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 批准节点",
		zap.String("nodeId", id),
		zap.Bool("approved", req.Approved))

	c.JSON(http.StatusOK, gin.H{
		"message":  "节点状态已更新",
		"nodeId":   id,
		"approved": req.Approved,
	})
}

// ========== ACL 策略 ==========

// GetACL handles GET /api/v1/tailscale/acl
// 获取 ACL 策略
func (h *Handler) GetACL(c *gin.Context) {
	policy := h.manager.GetACL()
	c.JSON(http.StatusOK, policy)
}

// UpdateACL handles PUT /api/v1/tailscale/acl
// 更新 ACL 策略
func (h *Handler) UpdateACL(c *gin.Context) {
	var req UpdateACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateACL(req.ACLs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 更新 ACL 策略",
		zap.Int("rules", len(req.ACLs)))

	c.JSON(http.StatusOK, gin.H{
		"message": "ACL 策略已更新",
		"rules":   len(req.ACLs),
	})
}

// ========== 子网路由 ==========

// ListSubnets handles GET /api/v1/tailscale/subnets
// 列出子网路由
func (h *Handler) ListSubnets(c *gin.Context) {
	subnets := h.manager.GetSubnets()
	c.JSON(http.StatusOK, gin.H{
		"subnets": subnets,
		"total":   len(subnets),
	})
}

// AddSubnet handles POST /api/v1/tailscale/subnets
// 添加子网路由
func (h *Handler) AddSubnet(c *gin.Context) {
	var req AddSubnetRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	route, err := h.manager.AddSubnet(req.CIDR, req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 添加子网路由",
		zap.String("cidr", req.CIDR),
		zap.String("nodeId", req.NodeID))

	c.JSON(http.StatusCreated, route)
}

// ToggleSubnet handles PUT /api/v1/tailscale/subnets/:id/toggle
// 启用/禁用子网路由
func (h *Handler) ToggleSubnet(c *gin.Context) {
	id := c.Param("id")

	var req ToggleSubnetRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ToggleSubnet(id, req.Enabled); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 切换子网路由",
		zap.String("id", id),
		zap.Bool("enabled", req.Enabled))

	c.JSON(http.StatusOK, gin.H{
		"message": "子网路由状态已更新",
		"id":      id,
		"enabled": req.Enabled,
	})
}

// ========== Exit Node ==========

// ListExitNodes handles GET /api/v1/tailscale/exit-nodes
// 列出出口节点
func (h *Handler) ListExitNodes(c *gin.Context) {
	nodes := h.manager.GetExitNodes()
	c.JSON(http.StatusOK, gin.H{
		"exitNodes": nodes,
		"total":     len(nodes),
	})
}

// SelectExitNode handles POST /api/v1/tailscale/exit-nodes/:id/select
// 选择出口节点
func (h *Handler) SelectExitNode(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.SelectExitNode(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 选择出口节点", zap.String("nodeId", id))

	c.JSON(http.StatusOK, gin.H{
		"message": "出口节点已选择",
		"nodeId":  id,
	})
}

// DeselectExitNode handles POST /api/v1/tailscale/exit-nodes/deselect
// 取消出口节点选择
func (h *Handler) DeselectExitNode(c *gin.Context) {
	if err := h.manager.DeselectExitNode(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 取消出口节点选择")

	c.JSON(http.StatusOK, gin.H{
		"message": "出口节点已取消",
	})
}

// ========== DNS 配置 ==========

// GetDNS handles GET /api/v1/tailscale/dns
// 获取 DNS 配置
func (h *Handler) GetDNS(c *gin.Context) {
	config := h.manager.GetDNS()
	c.JSON(http.StatusOK, config)
}

// UpdateDNS handles PUT /api/v1/tailscale/dns
// 更新 DNS 配置
func (h *Handler) UpdateDNS(c *gin.Context) {
	var req UpdateDNSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateDNS(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 更新 DNS 配置")

	c.JSON(http.StatusOK, gin.H{
		"message": "DNS 配置已更新",
	})
}

// ========== 认证密钥 ==========

// ListAuthKeys handles GET /api/v1/tailscale/authkeys
// 列出认证密钥
func (h *Handler) ListAuthKeys(c *gin.Context) {
	keys := h.manager.GetAuthKeys()
	c.JSON(http.StatusOK, gin.H{
		"authKeys": keys,
		"total":    len(keys),
	})
}

// CreateAuthKey handles POST /api/v1/tailscale/authkeys
// 创建认证密钥
func (h *Handler) CreateAuthKey(c *gin.Context) {
	var req CreateAuthKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, err := h.manager.CreateAuthKey(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 创建认证密钥",
		zap.String("id", key.ID),
		zap.String("description", req.Description))

	c.JSON(http.StatusCreated, key)
}

// RevokeAuthKey handles DELETE /api/v1/tailscale/authkeys/:id
// 撤销认证密钥
func (h *Handler) RevokeAuthKey(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RevokeAuthKey(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[Tailscale API] 撤销认证密钥", zap.String("id", id))

	c.JSON(http.StatusOK, gin.H{
		"message": "认证密钥已撤销",
		"id":      id,
	})
}

// ========== 流量统计 ==========

// GetStats handles GET /api/v1/tailscale/stats
// 获取流量统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
