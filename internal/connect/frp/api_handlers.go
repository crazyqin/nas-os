// Package frp provides FRP client implementation
// FRP WebUI API Handlers - 隧道管理 REST API
package frp

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// APIHandlers FRP WebUI API处理器
// 提供隧道列表/状态查询、配置CRUD、WebSocket状态推送
type APIHandlers struct {
	manager     *ClientManager
	logger      *zap.Logger
	wsUpgrader  websocket.Upgrader
	wsClients   map[string]*wsClient
	wsMu        sync.RWMutex
	wsBroadcast chan ClientEvent
}

// wsClient WebSocket客户端连接
type wsClient struct {
	conn      *websocket.Conn
	send      chan []byte
	closeChan chan struct{}
}

// NewAPIHandlers 创建API处理器
func NewAPIHandlers(manager *ClientManager, logger *zap.Logger) *APIHandlers {
	h := &APIHandlers{
		manager:    manager,
		logger:     logger,
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境应限制
			},
		},
		wsClients:   make(map[string]*wsClient),
		wsBroadcast: make(chan ClientEvent, 200),
	}

	// 启动事件广播循环
	go h.broadcastLoop()

	// 订阅管理器事件
	go h.subscribeEvents()

	return h
}

// RegisterRoutes 注册路由
func (h *APIHandlers) RegisterRoutes(r *gin.RouterGroup) {
	frpg := r.Group("/frp")
	{
		// 隧道列表/状态查询
		frpg.GET("/tunnels", h.ListTunnels)
		frpg.GET("/tunnels/:id", h.GetTunnel)
		frpg.GET("/tunnels/:id/status", h.GetTunnelStatus)
		frpg.GET("/clients/status", h.GetAllClientStatus)
		frpg.GET("/clients/:id/status", h.GetClientStatus)

		// 隧道配置CRUD
		frpg.POST("/tunnels", h.CreateTunnel)
		frpg.PUT("/tunnels/:id", h.UpdateTunnel)
		frpg.DELETE("/tunnels/:id", h.DeleteTunnel)
		frpg.POST("/tunnels/:id/start", h.StartTunnel)
		frpg.POST("/tunnels/:id/stop", h.StopTunnel)

		// 一键连接
		frpg.POST("/quick-connect", h.QuickConnect)
		frpg.POST("/disconnect/:id", h.DisconnectClient)
		frpg.POST("/disconnect-all", h.DisconnectAll)

		// 节点管理
		frpg.GET("/nodes", h.GetNodes)
		frpg.GET("/nodes/:region", h.GetNodesByRegion)
		frpg.GET("/nodes/best", h.GetBestNode)
		frpg.POST("/nodes/health-check", h.HealthCheckNodes)

		// WebSocket状态推送
		frpg.GET("/ws", h.WebSocketHandler)
	}
}

// ================== 隧道列表/状态查询 ==================

// ListTunnelsRequest 请求结构
type ListTunnelsRequest struct {
	ClientID string `form:"client_id"` // 可选：按客户端过滤
	Status   string `form:"status"`    // 可选：按状态过滤
	Type     string `form:"type"`      // 可选：按类型过滤
	Limit    int    `form:"limit"`
	Offset   int    `form:"offset"`
}

// ListTunnelsResponse 响应结构
type ListTunnelsResponse struct {
	Total   int               `json:"total"`
	Tunnels []TunnelListEntry `json:"tunnels"`
}

// TunnelListEntry 隧道列表条目
type TunnelListEntry struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         TunnelType `json:"type"`
	ClientID     string     `json:"client_id"`
	Status       string     `json:"status"`
	LocalAddr    string     `json:"local_addr"`
	RemoteAddr   string     `json:"remote_addr,omitempty"`
	PublicURL    string     `json:"public_url,omitempty"`
	BytesSent    uint64     `json:"bytes_sent"`
	BytesRecv    uint64     `json:"bytes_recv"`
	Connections  int        `json:"connections"`
	LastActive   time.Time  `json:"last_active"`
	Enabled      bool       `json:"enabled"`
}

// ListTunnels 列出所有隧道
// @Summary 列出隧道
// @Description 获取所有隧道列表，支持过滤
// @Tags frp
// @Accept json
// @Produce json
// @Param client_id query string false "客户端ID过滤"
// @Param status query string false "状态过滤"
// @Param type query string false "类型过滤"
// @Success 200 {object} ListTunnelsResponse
// @Router /api/v1/frp/tunnels [get]
func (h *APIHandlers) ListTunnels(c *gin.Context) {
	var req ListTunnelsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 获取所有客户端状态
	allStatus := h.manager.GetAllClientStatus()

	// 构建隧道列表
	var tunnels []TunnelListEntry
	for _, clientStatus := range allStatus {
		// 过滤客户端
		if req.ClientID != "" && clientStatus.ClientID != req.ClientID {
			continue
		}

		for _, tunnel := range clientStatus.Tunnels {
			// 过滤状态
			if req.Status != "" && tunnel.Status != req.Status {
				continue
			}
			// 过滤类型
			if req.Type != "" && string(tunnel.Type) != req.Type {
				continue
			}

			entry := TunnelListEntry{
				ID:          tunnel.ID,
				Name:        tunnel.Name,
				Type:        tunnel.Type,
				ClientID:    clientStatus.ClientID,
				Status:      tunnel.Status,
				LocalAddr:   tunnel.LocalAddr,
				RemoteAddr:  tunnel.RemoteAddr,
				PublicURL:   tunnel.PublicURL,
				BytesSent:   tunnel.BytesSent,
				BytesRecv:   tunnel.BytesRecv,
				Connections: tunnel.Connections,
				LastActive:  tunnel.LastActive,
				Enabled:     tunnel.Status == "running",
			}
			tunnels = append(tunnels, entry)
		}
	}

	// 分页
	total := len(tunnels)
	if req.Limit > 0 {
		start := req.Offset
		end := req.Offset + req.Limit
		if start > total {
			tunnels = []TunnelListEntry{}
		} else if end > total {
			tunnels = tunnels[start:]
		} else {
			tunnels = tunnels[start:end]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": ListTunnelsResponse{
			Total:   total,
			Tunnels: tunnels,
		},
	})
}

// GetTunnel 获取隧道详情
// @Summary 获取隧道详情
// @Description 获取指定隧道的详细信息
// @Tags frp
// @Produce json
// @Param id path string true "隧道ID"
// @Success 200 {object} TunnelDetailResponse
// @Router /api/v1/frp/tunnels/{id} [get]
func (h *APIHandlers) GetTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	if tunnelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "tunnel id required",
		})
		return
	}

	// 搜索隧道
	allStatus := h.manager.GetAllClientStatus()
	for _, clientStatus := range allStatus {
		for _, tunnel := range clientStatus.Tunnels {
			if tunnel.ID == tunnelID {
				c.JSON(http.StatusOK, gin.H{
					"code":    0,
					"message": "success",
					"data": TunnelDetailResponse{
						Tunnel:    tunnel,
						ClientID:  clientStatus.ClientID,
						NodeName:  clientStatus.NodeName,
						NodeRegion: clientStatus.Region,
					},
				})
				return
			}
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "tunnel not found",
	})
}

// TunnelDetailResponse 隧道详情响应
type TunnelDetailResponse struct {
	Tunnel     TunnelStatus `json:"tunnel"`
	ClientID   string       `json:"client_id"`
	NodeName   string       `json:"node_name"`
	NodeRegion NodeRegion   `json:"node_region"`
}

// GetTunnelStatus 获取隧道状态
// @Summary 获取隧道状态
// @Description 获取指定隧道的实时状态
// @Tags frp
// @Produce json
// @Param id path string true "隧道ID"
// @Success 200 {object} TunnelStatus
// @Router /api/v1/frp/tunnels/{id}/status [get]
func (h *APIHandlers) GetTunnelStatus(c *gin.Context) {
	tunnelID := c.Param("id")

	allStatus := h.manager.GetAllClientStatus()
	for _, clientStatus := range allStatus {
		for _, tunnel := range clientStatus.Tunnels {
			if tunnel.ID == tunnelID {
				c.JSON(http.StatusOK, gin.H{
					"code":    0,
					"message": "success",
					"data":    tunnel,
				})
				return
			}
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "tunnel not found",
	})
}

// GetAllClientStatus 获取所有客户端状态
// @Summary 获取客户端状态
// @Description 获取所有FRP客户端的状态信息
// @Tags frp
// @Produce json
// @Success 200 {object} ClientStatusListResponse
// @Router /api/v1/frp/clients/status [get]
func (h *APIHandlers) GetAllClientStatus(c *gin.Context) {
	statuses := h.manager.GetAllClientStatus()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": ClientStatusListResponse{
			Total:   len(statuses),
			Clients: statuses,
		},
	})
}

// ClientStatusListResponse 客户端状态列表响应
type ClientStatusListResponse struct {
	Total   int                `json:"total"`
	Clients []*ClientStatusInfo `json:"clients"`
}

// GetClientStatus 获取单个客户端状态
// @Summary 获取客户端状态
// @Description 获取指定客户端的状态信息
// @Tags frp
// @Produce json
// @Param id path string true "客户端ID"
// @Success 200 {object} ClientStatusInfo
// @Router /api/v1/frp/clients/{id}/status [get]
func (h *APIHandlers) GetClientStatus(c *gin.Context) {
	clientID := c.Param("id")
	status := h.manager.GetClientStatus(clientID)

	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "client not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// ================== 隧道配置CRUD ==================

// CreateTunnelRequest 创建隧道请求
type CreateTunnelRequest struct {
	ClientID     string     `json:"client_id" binding:"required"` // 使用指定客户端
	NodeID       string     `json:"node_id"`                     // 或指定节点自动创建
	Name         string     `json:"name" binding:"required"`
	Type         TunnelType `json:"type" binding:"required"`
	LocalIP      string     `json:"local_ip"`
	LocalPort    int        `json:"local_port" binding:"required"`
	RemotePort   int        `json:"remote_port"`
	SubDomain    string     `json:"sub_domain"`
	CustomDomains []string  `json:"custom_domains"`
	Sk           string     `json:"sk"` // STCP密钥
	Enabled      bool       `json:"enabled"`
}

// CreateTunnel 创建隧道
// @Summary 创建隧道
// @Description 创建新的FRP隧道
// @Tags frp
// @Accept json
// @Produce json
// @Param request body CreateTunnelRequest true "创建请求"
// @Success 200 {object} CreateTunnelResponse
// @Router /api/v1/frp/tunnels [post]
func (h *APIHandlers) CreateTunnel(c *gin.Context) {
	var req CreateTunnelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 验证参数
	if req.LocalIP == "" {
		req.LocalIP = "127.0.0.1"
	}
	if req.Enabled {
		req.Enabled = true
	}

	// 构建隧道配置
	tunnel := TunnelConfig{
		ID:            generateTunnelID(),
		Name:          req.Name,
		Type:          req.Type,
		LocalIP:       req.LocalIP,
		LocalPort:     req.LocalPort,
		RemotePort:    req.RemotePort,
		SubDomain:     req.SubDomain,
		CustomDomains: req.CustomDomains,
		Sk:            req.Sk,
		Enabled:       req.Enabled,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 获取客户端并添加隧道
	client := h.manager.GetClient(req.ClientID)
	if client == nil {
		// 如果指定了节点，尝试一键连接
		if req.NodeID != "" {
			quickConfig := QuickConnectConfig{
				NodeID:     req.NodeID,
				LocalPort:  req.LocalPort,
				RemotePort: req.RemotePort,
				TunnelName: req.Name,
				TunnelType: req.Type,
			}
			result, err := h.manager.QuickConnect(&quickConfig)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "quick connect failed: " + err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "tunnel created and connected",
				"data": CreateTunnelResponse{
					TunnelID:  result.TunnelID,
					ClientID:  req.NodeID,
					PublicURL: result.PublicURL,
					Node:      result.Node,
					Status:    "running",
				},
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "client not found, specify node_id for auto-connect",
		})
		return
	}

	// 添加隧道到现有客户端
	if err := client.AddTunnel(tunnel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to add tunnel: " + err.Error(),
		})
		return
	}

	// 获取隧道状态
	status := client.GetTunnelStatus(tunnel.ID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "tunnel created",
		"data": CreateTunnelResponse{
			TunnelID:  tunnel.ID,
			ClientID:  req.ClientID,
			Status:    status.Status,
			PublicURL: status.PublicURL,
		},
	})
}

// CreateTunnelResponse 创建隧道响应
type CreateTunnelResponse struct {
	TunnelID  string    `json:"tunnel_id"`
	ClientID  string    `json:"client_id"`
	Status    string    `json:"status"`
	PublicURL string    `json:"public_url,omitempty"`
	Node      *FreeNode `json:"node,omitempty"`
}

// UpdateTunnelRequest 更新隧道请求
type UpdateTunnelRequest struct {
	Name          string     `json:"name,omitempty"`
	Type          TunnelType `json:"type,omitempty"`
	LocalIP       string     `json:"local_ip,omitempty"`
	LocalPort     int        `json:"local_port,omitempty"`
	RemotePort    int        `json:"remote_port,omitempty"`
	SubDomain     string     `json:"sub_domain,omitempty"`
	CustomDomains []string   `json:"custom_domains,omitempty"`
	Sk            string     `json:"sk,omitempty"`
	Enabled       *bool      `json:"enabled,omitempty"`
}

// UpdateTunnel 更新隧道配置
// @Summary 更新隧道
// @Description 更新隧道配置
// @Tags frp
// @Accept json
// @Produce json
// @Param id path string true "隧道ID"
// @Param request body UpdateTunnelRequest true "更新请求"
// @Success 200 {object} TunnelStatus
// @Router /api/v1/frp/tunnels/{id} [put]
func (h *APIHandlers) UpdateTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	var req UpdateTunnelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 找到包含该隧道的客户端
	allClients := h.manager.GetAllClients()
	var targetClient *Client
	var existingTunnel *TunnelConfig

	for _, client := range allClients {
		for i := range client.config.Tunnels {
			if client.config.Tunnels[i].ID == tunnelID {
				targetClient = client
				existingTunnel = &client.config.Tunnels[i]
				break
			}
		}
		if targetClient != nil {
			break
		}
	}

	if targetClient == nil || existingTunnel == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "tunnel not found",
		})
		return
	}

	// 更新配置
	updated := *existingTunnel
	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.Type != "" {
		updated.Type = req.Type
	}
	if req.LocalIP != "" {
		updated.LocalIP = req.LocalIP
	}
	if req.LocalPort > 0 {
		updated.LocalPort = req.LocalPort
	}
	if req.RemotePort > 0 {
		updated.RemotePort = req.RemotePort
	}
	if req.SubDomain != "" {
		updated.SubDomain = req.SubDomain
	}
	if req.CustomDomains != nil {
		updated.CustomDomains = req.CustomDomains
	}
	if req.Sk != "" {
		updated.Sk = req.Sk
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	updated.UpdatedAt = time.Now()

	// 应用更新
	if !targetClient.config.UpdateTunnel(updated) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to update tunnel config",
		})
		return
	}

	// 如果隧道正在运行，需要重启
	status := targetClient.GetTunnelStatus(tunnelID)
	if status != nil && status.Status == "running" {
		// TODO: 重启隧道逻辑
		h.logger.Info("tunnel config updated, may need restart",
			zap.String("tunnel_id", tunnelID))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "tunnel updated",
		"data":    targetClient.GetTunnelStatus(tunnelID),
	})
}

// DeleteTunnel 删除隧道
// @Summary 删除隧道
// @Description 删除指定隧道
// @Tags frp
// @Produce json
// @Param id path string true "隧道ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/frp/tunnels/{id} [delete]
func (h *APIHandlers) DeleteTunnel(c *gin.Context) {
	tunnelID := c.Param("id")

	// 找到包含该隧道的客户端
	allClients := h.manager.GetAllClients()
	var targetClientID string
	var targetClient *Client

	for clientID, client := range allClients {
		for _, t := range client.config.Tunnels {
			if t.ID == tunnelID {
				targetClientID = clientID
				targetClient = client
				break
			}
		}
		if targetClient != nil {
			break
		}
	}

	if targetClient == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "tunnel not found",
		})
		return
	}

	// 停止并删除隧道
	if err := targetClient.RemoveTunnel(tunnelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to remove tunnel: " + err.Error(),
		})
		return
	}

	// 从配置中删除
	targetClient.config.RemoveTunnel(tunnelID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "tunnel deleted",
		"data": gin.H{
			"tunnel_id": tunnelID,
			"client_id": targetClientID,
		},
	})
}

// StartTunnel 启动隧道
// @Summary 启动隧道
// @Description 启动指定隧道
// @Tags frp
// @Produce json
// @Param id path string true "隧道ID"
// @Success 200 {object} TunnelStatus
// @Router /api/v1/frp/tunnels/{id}/start [post]
func (h *APIHandlers) StartTunnel(c *gin.Context) {
	tunnelID := c.Param("id")

	// 找到隧道
	allClients := h.manager.GetAllClients()
	var targetClient *Client
	var targetTunnel *TunnelConfig

	for _, client := range allClients {
		for i := range client.config.Tunnels {
			if client.config.Tunnels[i].ID == tunnelID {
				targetClient = client
				targetTunnel = &client.config.Tunnels[i]
				break
			}
		}
		if targetClient != nil {
			break
		}
	}

	if targetClient == nil || targetTunnel == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "tunnel not found",
		})
		return
	}

	// 检查客户端连接状态
	if targetClient.GetStatus() != "connected" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "client not connected",
		})
		return
	}

	// 启动隧道
	targetTunnel.Enabled = true
	if err := targetClient.startTunnel(*targetTunnel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to start tunnel: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "tunnel started",
		"data":    targetClient.GetTunnelStatus(tunnelID),
	})
}

// StopTunnel 停止隧道
// @Summary 停止隧道
// @Description 停止指定隧道
// @Tags frp
// @Produce json
// @Param id path string true "隧道ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/frp/tunnels/{id}/stop [post]
func (h *APIHandlers) StopTunnel(c *gin.Context) {
	tunnelID := c.Param("id")

	allClients := h.manager.GetAllClients()
	var targetClient *Client

	for _, client := range allClients {
		for _, t := range client.config.Tunnels {
			if t.ID == tunnelID {
				targetClient = client
				break
			}
		}
		if targetClient != nil {
			break
		}
	}

	if targetClient == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "tunnel not found",
		})
		return
	}

	// 停止隧道
	if err := targetClient.RemoveTunnel(tunnelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to stop tunnel: " + err.Error(),
		})
		return
	}

	// 标记为禁用
	for i := range targetClient.config.Tunnels {
		if targetClient.config.Tunnels[i].ID == tunnelID {
			targetClient.config.Tunnels[i].Enabled = false
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "tunnel stopped",
		"data": gin.H{
			"tunnel_id": tunnelID,
			"status":    "stopped",
		},
	})
}

// ================== 一键连接 ==================

// QuickConnectRequest 一键连接请求
type QuickConnectRequest struct {
	NodeID     string     `json:"node_id"`      // 可选：指定节点
	Region     NodeRegion `json:"region"`       // 可选：指定区域
	LocalPort  int        `json:"local_port" binding:"required"`
	RemotePort int        `json:"remote_port"`  // 可选：远程端口
	TunnelName string     `json:"tunnel_name"`  // 可选：隧道名称
	TunnelType TunnelType `json:"tunnel_type"`  // 可选：隧道类型
}

// QuickConnectAPIHandler 一键连接API
// @Summary 一键连接
// @Description 使用免费节点一键创建FRP隧道
// @Tags frp
// @Accept json
// @Produce json
// @Param request body QuickConnectRequest true "连接请求"
// @Success 200 {object} QuickConnectResult
// @Router /api/v1/frp/quick-connect [post]
func (h *APIHandlers) QuickConnect(c *gin.Context) {
	var req QuickConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 设置默认隧道类型
	if req.TunnelType == "" {
		req.TunnelType = TunnelTypeTCP
	}

	config := QuickConnectConfig{
		NodeID:     req.NodeID,
		Region:     req.Region,
		LocalPort:  req.LocalPort,
		RemotePort: req.RemotePort,
		TunnelName: req.TunnelName,
		TunnelType: req.TunnelType,
	}

	result, err := h.manager.QuickConnect(&config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "connect failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "connected successfully",
		"data":    result,
	})
}

// DisconnectClient 断开客户端
// @Summary 断开客户端
// @Description 断开指定FRP客户端连接
// @Tags frp
// @Produce json
// @Param id path string true "客户端ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/frp/disconnect/{id} [post]
func (h *APIHandlers) DisconnectClient(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.manager.Disconnect(clientID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "client disconnected",
		"data": gin.H{
			"client_id": clientID,
		},
	})
}

// DisconnectAll 断开所有连接
// @Summary 断开所有连接
// @Description 断开所有FRP客户端连接
// @Tags frp
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/frp/disconnect-all [post]
func (h *APIHandlers) DisconnectAll(c *gin.Context) {
	if err := h.manager.DisconnectAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "all clients disconnected",
	})
}

// ================== 节点管理 ==================

// GetNodes 获取所有节点
// @Summary 获取节点列表
// @Description 获取所有可用的FRP节点
// @Tags frp
// @Produce json
// @Success 200 {object} NodesListResponse
// @Router /api/v1/frp/nodes [get]
func (h *APIHandlers) GetNodes(c *gin.Context) {
	nodes := h.manager.GetNodes()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": NodesListResponse{
			Total: len(nodes),
			Nodes: nodes,
		},
	})
}

// NodesListResponse 节点列表响应
type NodesListResponse struct {
	Total int         `json:"total"`
	Nodes []*FreeNode `json:"nodes"`
}

// GetNodesByRegion 按区域获取节点
// @Summary 按区域获取节点
// @Description 获取指定区域的FRP节点
// @Tags frp
// @Produce json
// @Param region path string true "区域(cn/us/eu)"
// @Success 200 {object} NodesListResponse
// @Router /api/v1/frp/nodes/{region} [get]
func (h *APIHandlers) GetNodesByRegion(c *gin.Context) {
	region := NodeRegion(c.Param("region"))
	nodes := h.manager.GetNodesByRegion(region)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": NodesListResponse{
			Total: len(nodes),
			Nodes: nodes,
		},
	})
}

// GetBestNode 获取最优节点
// @Summary 获取最优节点
// @Description 自动选择最优的FRP节点
// @Tags frp
// @Produce json
// @Param region query string false "可选区域限制"
// @Success 200 {object} FreeNode
// @Router /api/v1/frp/nodes/best [get]
func (h *APIHandlers) GetBestNode(c *gin.Context) {
	regionStr := c.Query("region")
	var node *FreeNode

	if regionStr != "" {
		node = h.manager.GetBestNode(NodeRegion(regionStr))
	} else {
		node = h.manager.GetBestNode()
	}

	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "no available node",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    node,
	})
}

// HealthCheckNodes 健康检查节点
// @Summary 健康检查节点
// @Description 检查所有节点的连通性
// @Tags frp
// @Produce json
// @Success 200 {object} HealthCheckResponse
// @Router /api/v1/frp/nodes/health-check [post]
func (h *APIHandlers) HealthCheckNodes(c *gin.Context) {
	results := h.manager.HealthCheck(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "health check completed",
		"data": HealthCheckResponse{
			Total:   len(results),
			Results: results,
		},
	})
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Total   int                       `json:"total"`
	Results map[string]*NodeHealthResult `json:"results"`
}

// ================== WebSocket状态推送 ==================

// WebSocketHandler WebSocket处理器
// @Summary WebSocket状态推送
// @Description 实时推送FRP隧道状态变化
// @Tags frp
// @Success 101 "Switching Protocols"
// @Router /api/v1/frp/ws [get]
func (h *APIHandlers) WebSocketHandler(c *gin.Context) {
	conn, err := h.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	// 创建客户端
	clientID := generateTunnelID()
	client := &wsClient{
		conn:      conn,
		send:      make(chan []byte, 100),
		closeChan: make(chan struct{}),
	}

	h.wsMu.Lock()
	h.wsClients[clientID] = client
	h.wsMu.Unlock()

	h.logger.Info("websocket client connected", zap.String("client_id", clientID))

	// 发送初始状态
	initialStatus := h.manager.GetAllClientStatus()
	data, _ := encodeJSON(initialStatus)
	client.send <- data

	// 读协程
	go func() {
		defer func() {
			close(client.closeChan)
			h.wsMu.Lock()
			delete(h.wsClients, clientID)
			h.wsMu.Unlock()
			conn.Close()
			h.logger.Info("websocket client disconnected", zap.String("client_id", clientID))
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	// 写协程
	go func() {
		for {
			select {
			case <-client.closeChan:
				return
			case data := <-client.send:
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					h.logger.Debug("websocket write error", zap.Error(err))
					return
				}
			}
		}
	}()
}

// subscribeEvents 订阅管理器事件
func (h *APIHandlers) subscribeEvents() {
	events := h.manager.Events()
	for event := range events {
		h.wsBroadcast <- event
	}
}

// broadcastLoop 广播事件到所有WebSocket客户端
func (h *APIHandlers) broadcastLoop() {
	for event := range h.wsBroadcast {
		data, err := encodeJSON(event)
		if err != nil {
			continue
		}

		h.wsMu.RLock()
		for _, client := range h.wsClients {
			select {
			case client.send <- data:
			default:
				// 缓冲区满，跳过
			}
		}
		h.wsMu.RUnlock()
	}
}

// encodeJSON JSON编码辅助函数
func encodeJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}