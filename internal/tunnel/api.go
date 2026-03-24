// Package tunnel 提供内网穿透服务 - HTTP API
package tunnel

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// APIHandler 隧道 API 处理器
type APIHandler struct {
	service *TunnelService
	logger  *zap.Logger
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler(service *TunnelService, logger *zap.Logger) *APIHandler {
	return &APIHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes 注册 API 路由
func (h *APIHandler) RegisterRoutes(r *gin.RouterGroup) {
	tunnel := r.Group("/tunnel")
	{
		// 状态接口
		tunnel.GET("/status", h.GetStatus)

		// 连接管理
		tunnel.POST("/connect", h.Connect)
		tunnel.POST("/disconnect", h.Disconnect)

		// 隧道管理
		tunnel.GET("/tunnels", h.ListTunnels)
		tunnel.GET("/tunnels/:id", h.GetTunnel)
		tunnel.POST("/tunnels", h.CreateTunnel)
		tunnel.DELETE("/tunnels/:id", h.DeleteTunnel)

		// 配置
		tunnel.GET("/config", h.GetConfig)
		tunnel.PUT("/config", h.UpdateConfig)

		// NAT 检测
		tunnel.POST("/detect-nat", h.DetectNAT)
	}
}

// GetStatus 获取隧道服务状态
// @Summary 获取隧道服务状态
// @Tags tunnel
// @Produce json
// @Success 200 {object} TunnelServiceStatus
// @Router /tunnel/status [get]
func (h *APIHandler) GetStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// PeerConnectRequest P2P连接请求
type PeerConnectRequest struct {
	PeerID  string `json:"peer_id" binding:"required"`
	Mode    string `json:"mode"`
	Timeout int    `json:"timeout"` // 秒
}

// Connect 连接到对端
// @Summary 连接到对端设备
// @Tags tunnel
// @Accept json
// @Produce json
// @Param request body ConnectRequest true "连接请求"
// @Success 200 {object} ConnectionStatus
// @Router /tunnel/connect [post]
func (h *APIHandler) Connect(c *gin.Context) {
	var req PeerConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	timeout := time.Duration(30) * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctx, cancel := c.Request.Context(), func() {}
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline || deadline.Sub(time.Now()) > timeout {
		var cancelCtx context.CancelFunc
		ctx, cancelCtx = context.WithTimeout(ctx, timeout)
		defer cancelCtx()
	} else {
		defer cancel()
	}

	conn, err := h.service.ConnectToPeer(ctx, req.PeerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	sent, recv := conn.P2PConn.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         conn.ID,
			"peer_id":    req.PeerID,
			"state":      conn.P2PConn.session.State,
			"bytes_sent": sent,
			"bytes_recv": recv,
			"connected":  conn.P2PConn.IsConnected(),
		},
	})
}

// Disconnect 断开连接
// @Summary 断开隧道连接
// @Tags tunnel
// @Accept json
// @Produce json
// @Param request body DisconnectRequest true "断开请求"
// @Success 200 {object} gin.H
// @Router /tunnel/disconnect [post]
func (h *APIHandler) Disconnect(c *gin.Context) {
	var req DisconnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	h.service.mu.Lock()
	conn, exists := h.service.activeConns[req.TunnelID]
	if exists {
		delete(h.service.activeConns, req.TunnelID)
	}
	h.service.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "connection not found"})
		return
	}

	if err := conn.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "disconnected"})
}

// ListTunnels 列出所有隧道
// @Summary 列出所有隧道
// @Tags tunnel
// @Produce json
// @Success 200 {array} TunnelStatus
// @Router /tunnel/tunnels [get]
func (h *APIHandler) ListTunnels(c *gin.Context) {
	tunnels := h.service.manager.ListTunnels()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tunnels,
	})
}

// GetTunnel 获取单个隧道信息
// @Summary 获取单个隧道信息
// @Tags tunnel
// @Produce json
// @Param id path string true "隧道ID"
// @Success 200 {object} TunnelStatus
// @Router /tunnel/tunnels/{id} [get]
func (h *APIHandler) GetTunnel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tunnel id required"})
		return
	}

	status, err := h.service.manager.GetTunnelStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// CreateTunnelRequest 创建隧道请求
type CreateTunnelRequest struct {
	Name        string `json:"name" binding:"required"`
	Mode        string `json:"mode"`
	LocalPort   int    `json:"local_port" binding:"required,min=1,max=65535"`
	RemotePort  int    `json:"remote_port"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
}

// CreateTunnel 创建隧道
// @Summary 创建新隧道
// @Tags tunnel
// @Accept json
// @Produce json
// @Param request body CreateTunnelRequest true "创建请求"
// @Success 201 {object} ConnectResponse
// @Router /tunnel/tunnels [post]
func (h *APIHandler) CreateTunnel(c *gin.Context) {
	var req CreateTunnelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	mode := TunnelMode(req.Mode)
	if mode == "" {
		mode = ModeAuto
	}

	connectReq := ConnectRequest{
		Name:       req.Name,
		Mode:       mode,
		LocalPort:  req.LocalPort,
		RemotePort: req.RemotePort,
		Protocol:   req.Protocol,
	}

	if connectReq.Protocol == "" {
		connectReq.Protocol = "tcp"
	}

	resp, err := h.service.manager.Connect(c.Request.Context(), connectReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    resp,
	})
}

// DeleteTunnel 删除隧道
func (h *APIHandler) DeleteTunnel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tunnel id required"})
		return
	}

	if err := h.service.manager.Disconnect(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "tunnel deleted",
	})
}

// GetConfig 获取配置
// @Summary 获取隧道配置
// @Tags tunnel
// @Produce json
// @Success 200 {object} Config
// @Router /tunnel/config [get]
func (h *APIHandler) GetConfig(c *gin.Context) {
	h.service.mu.RLock()
	config := h.service.config
	h.service.mu.RUnlock()

	// 隐藏敏感信息
	displayConfig := gin.H{
		"server_addr":         config.ServerAddr,
		"server_port":         config.ServerPort,
		"device_id":           config.DeviceID,
		"device_name":         config.DeviceName,
		"mode":                config.Mode,
		"heartbeat_int":       config.HeartbeatInt,
		"reconnect_int":       config.ReconnectInt,
		"max_reconnect":       config.MaxReconnect,
		"timeout":             config.Timeout,
		"enable_port_mapping": config.EnablePortMapping,
		"local_ports":         config.LocalPorts,
		"remote_ports":        config.RemotePorts,
		"enable_tls":          config.EnableTLS,
		"stun_servers":        config.STUNServers,
		"turn_servers":        config.TURNServers,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    displayConfig,
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	DeviceName  string   `json:"device_name"`
	Mode        string   `json:"mode"`
	STUNServers []string `json:"stun_servers"`
	TURNServers []string `json:"turn_servers"`
}

// UpdateConfig 更新配置
// @Summary 更新隧道配置
// @Tags tunnel
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "更新请求"
// @Success 200 {object} gin.H
// @Router /tunnel/config [put]
func (h *APIHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	h.service.mu.Lock()
	if req.DeviceName != "" {
		h.service.config.DeviceName = req.DeviceName
	}
	if req.Mode != "" {
		h.service.config.Mode = TunnelMode(req.Mode)
	}
	if len(req.STUNServers) > 0 {
		h.service.config.STUNServers = req.STUNServers
	}
	if len(req.TURNServers) > 0 {
		h.service.config.TURNServers = req.TURNServers
	}
	h.service.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "config updated",
	})
}

// DetectNAT 检测 NAT 类型
// @Summary 检测 NAT 类型
// @Tags tunnel
// @Produce json
// @Success 200 {object} gin.H
// @Router /tunnel/detect-nat [post]
func (h *APIHandler) DetectNAT(c *gin.Context) {
	stun := NewSTUNProtocol(h.service.config.STUNServers, h.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	natType, publicIP, publicPort, err := stun.DetectNATType(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"nat_type":    natType,
			"public_ip":   publicIP,
			"public_port": publicPort,
			"p2p_capable": natType != NATTypeSymmetric,
		},
	})
}

// WebSocketHandler WebSocket 处理器（用于实时状态推送）
func (h *APIHandler) WebSocketHandler(c *gin.Context) {
	conn, err := websocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Debug("websocket upgrade failed", zap.Error(err))
		return
	}
	defer func() { _ = conn.Close() }()

	// 注册事件回调
	eventCh := make(chan Event, 10)
	h.service.manager.OnEvent(func(event Event) {
		select {
		case eventCh <- event:
		default:
		}
	})

	// 发送初始状态
	status := h.service.GetStatus()
	_ = conn.WriteJSON(gin.H{"type": "status", "data": status})

	// 定期发送状态更新
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-eventCh:
			_ = conn.WriteJSON(gin.H{"type": "event", "data": event})
		case <-ticker.C:
			status := h.service.GetStatus()
			_ = conn.WriteJSON(gin.H{"type": "status", "data": status})
		case <-c.Request.Context().Done():
			return
		}
	}
}

// websocketUpgrader WebSocket 升级器
var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
