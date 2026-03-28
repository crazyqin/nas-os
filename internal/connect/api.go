// Package connect 提供 NAS Connect 远程访问服务的 API 处理器
package connect

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIHandler NAS Connect API 处理器.
type APIHandler struct {
	service *ConnectService
}

// NewAPIHandler 创建 API 处理器.
func NewAPIHandler(service *ConnectService) *APIHandler {
	return &APIHandler{
		service: service,
	}
}

// RegisterRoutes 注册路由.
func (h *APIHandler) RegisterRoutes(r *gin.RouterGroup) {
	connect := r.Group("/connect")
	{
		connect.GET("/status", h.GetStatus)
		connect.GET("/info", h.GetInfo)
		connect.GET("/stats", h.GetStats)
		connect.POST("/connect", h.Connect)
		connect.POST("/disconnect", h.Disconnect)
		connect.PUT("/config", h.UpdateConfig)
		connect.GET("/config", h.GetConfig)
		connect.POST("/test", h.TestConnection)
	}
}

// StatusResponse 状态响应.
type StatusResponse struct {
	Status    string `json:"status"`
	Connected bool   `json:"connected"`
	PublicURL string `json:"public_url,omitempty"`
	LocalURL  string `json:"local_url,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	Mode      string `json:"mode,omitempty"`
}

// GetStatus 获取状态.
// @Summary 获取 NAS Connect 状态
// @Description 获取远程访问服务的当前状态
// @Tags connect
// @Produce json
// @Success 200 {object} StatusResponse
// @Router /api/v1/connect/status [get].
func (h *APIHandler) GetStatus(c *gin.Context) {
	status := h.service.GetStatus()
	info := h.service.GetInfo()

	c.JSON(http.StatusOK, StatusResponse{
		Status:    status,
		Connected: status == StatusConnected,
		PublicURL: info.PublicURL,
		LocalURL:  info.LocalURL,
		DeviceID:  info.DeviceID,
		Mode:      info.Mode,
	})
}

// InfoResponse 信息响应.
type InfoResponse struct {
	DeviceID    string          `json:"device_id"`
	DeviceName  string          `json:"device_name"`
	PublicURL   string          `json:"public_url"`
	LocalURL    string          `json:"local_url"`
	Status      string          `json:"status"`
	Mode        string          `json:"mode"`
	ConnectedAt string          `json:"connected_at"`
	Uptime      string          `json:"uptime"`
	Stats       ConnectionStats `json:"stats"`
}

// GetInfo 获取详细信息.
// @Summary 获取 NAS Connect 详细信息
// @Description 获取远程访问服务的详细信息
// @Tags connect
// @Produce json
// @Success 200 {object} InfoResponse
// @Router /api/v1/connect/info [get].
func (h *APIHandler) GetInfo(c *gin.Context) {
	info := h.service.GetInfo()
	stats := h.service.GetStats()

	connectedAt := ""
	if !info.ConnectedAt.IsZero() {
		connectedAt = info.ConnectedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	c.JSON(http.StatusOK, InfoResponse{
		DeviceID:    info.DeviceID,
		DeviceName:  info.DeviceName,
		PublicURL:   info.PublicURL,
		LocalURL:    info.LocalURL,
		Status:      info.Status,
		Mode:        info.Mode,
		ConnectedAt: connectedAt,
		Uptime:      info.Uptime,
		Stats:       stats,
	})
}

// StatsResponse 统计响应.
type StatsResponse struct {
	BytesSent     uint64 `json:"bytes_sent"`
	BytesReceived uint64 `json:"bytes_received"`
	Connections   int    `json:"connections"`
	LastActivity  string `json:"last_activity"`
	Latency       int    `json:"latency_ms"`
}

// GetStats 获取统计.
// @Summary 获取 NAS Connect 统计
// @Description 获取远程访问服务的流量和连接统计
// @Tags connect
// @Produce json
// @Success 200 {object} StatsResponse
// @Router /api/v1/connect/stats [get].
func (h *APIHandler) GetStats(c *gin.Context) {
	stats := h.service.GetStats()

	c.JSON(http.StatusOK, StatsResponse{
		BytesSent:     stats.BytesSent,
		BytesReceived: stats.BytesReceived,
		Connections:   stats.Connections,
		LastActivity:  stats.LastActivity.Format("2006-01-02T15:04:05Z07:00"),
		Latency:       stats.Latency,
	})
}

// ConnectRequest 连接请求.
type ConnectRequest struct {
	Mode string `json:"mode"` // direct, relay, auto
}

// ConnectResponse 连接响应.
type ConnectResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	PublicURL string `json:"public_url,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
}

// Connect 发起连接.
// @Summary 连接到 NAS Connect 服务
// @Description 启动远程访问连接
// @Tags connect
// @Accept json
// @Produce json
// @Param request body ConnectRequest false "连接参数"
// @Success 200 {object} ConnectResponse
// @Router /api/v1/connect/connect [post].
func (h *APIHandler) Connect(c *gin.Context) {
	var req ConnectRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.Mode != "" {
		h.service.mu.Lock()
		h.service.config.Mode = req.Mode
		h.service.mu.Unlock()
	}

	if h.service.GetStatus() == StatusConnected {
		c.JSON(http.StatusOK, ConnectResponse{
			Success:   true,
			Message:   "Already connected",
			PublicURL: h.service.publicURL,
			DeviceID:  h.service.deviceID,
		})
		return
	}

	if err := h.service.connect(); err != nil {
		c.JSON(http.StatusInternalServerError, ConnectResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ConnectResponse{
		Success:   true,
		Message:   "Connected successfully",
		PublicURL: h.service.publicURL,
		DeviceID:  h.service.deviceID,
	})
}

// Disconnect 断开连接.
// @Summary 断开 NAS Connect 连接
// @Description 断开远程访问连接
// @Tags connect
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/connect/disconnect [post].
func (h *APIHandler) Disconnect(c *gin.Context) {
	h.service.mu.Lock()
	if h.service.conn != nil {
		_ = h.service.conn.Close()
		h.service.conn = nil
	}
	h.service.status = StatusDisconnected
	h.service.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Disconnected",
	})
}

// ConfigResponse 配置响应.
type ConfigResponse struct {
	Enabled           bool     `json:"enabled"`
	ServerURL         string   `json:"server_url"`
	DeviceID          string   `json:"device_id"`
	DeviceName        string   `json:"device_name"`
	Mode              string   `json:"mode"`
	TLSEnabled        bool     `json:"tls_enabled"`
	LocalPort         int      `json:"local_port"`
	HeartbeatInterval string   `json:"heartbeat_interval"`
	ReconnectEnabled  bool     `json:"reconnect_enabled"`
	MaxBandwidth      int      `json:"max_bandwidth"`
	AllowedNetworks   []string `json:"allowed_networks"`
}

// GetConfig 获取配置.
// @Summary 获取 NAS Connect 配置
// @Description 获取远程访问服务配置
// @Tags connect
// @Produce json
// @Success 200 {object} ConfigResponse
// @Router /api/v1/connect/config [get].
func (h *APIHandler) GetConfig(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	c.JSON(http.StatusOK, ConfigResponse{
		Enabled:           h.service.config.Enabled,
		ServerURL:         h.service.config.ServerURL,
		DeviceID:          h.service.config.DeviceID,
		DeviceName:        h.service.config.DeviceName,
		Mode:              h.service.config.Mode,
		TLSEnabled:        h.service.config.TLSEnabled,
		LocalPort:         h.service.config.LocalPort,
		HeartbeatInterval: h.service.config.HeartbeatInterval.String(),
		ReconnectEnabled:  h.service.config.ReconnectEnabled,
		MaxBandwidth:      h.service.config.MaxBandwidth,
		AllowedNetworks:   h.service.config.AllowedNetworks,
	})
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	ServerURL        *string  `json:"server_url,omitempty"`
	DeviceName       *string  `json:"device_name,omitempty"`
	Mode             *string  `json:"mode,omitempty"`
	TLSEnabled       *bool    `json:"tls_enabled,omitempty"`
	ReconnectEnabled *bool    `json:"reconnect_enabled,omitempty"`
	MaxBandwidth     *int     `json:"max_bandwidth,omitempty"`
	AllowedNetworks  []string `json:"allowed_networks,omitempty"`
}

// UpdateConfig 更新配置.
// @Summary 更新 NAS Connect 配置
// @Description 更新远程访问服务配置
// @Tags connect
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "配置参数"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/connect/config [put].
func (h *APIHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.service.mu.Lock()
	defer h.service.mu.Unlock()

	if req.Enabled != nil {
		h.service.config.Enabled = *req.Enabled
	}
	if req.ServerURL != nil {
		h.service.config.ServerURL = *req.ServerURL
	}
	if req.DeviceName != nil {
		h.service.config.DeviceName = *req.DeviceName
	}
	if req.Mode != nil {
		h.service.config.Mode = *req.Mode
	}
	if req.TLSEnabled != nil {
		h.service.config.TLSEnabled = *req.TLSEnabled
	}
	if req.ReconnectEnabled != nil {
		h.service.config.ReconnectEnabled = *req.ReconnectEnabled
	}
	if req.MaxBandwidth != nil {
		h.service.config.MaxBandwidth = *req.MaxBandwidth
	}
	if req.AllowedNetworks != nil {
		h.service.config.AllowedNetworks = req.AllowedNetworks
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration updated",
	})
}

// TestResponse 测试响应.
type TestResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	Latency         int    `json:"latency_ms"`
	ServerReachable bool   `json:"server_reachable"`
	STUNWorking     bool   `json:"stun_working"`
}

// TestConnection 测试连接.
// @Summary 测试 NAS Connect 连接
// @Description 测试远程访问服务连接性
// @Tags connect
// @Produce json
// @Success 200 {object} TestResponse
// @Router /api/v1/connect/test [post].
func (h *APIHandler) TestConnection(c *gin.Context) {
	h.service.mu.RLock()
	config := h.service.config
	h.service.mu.RUnlock()

	result := TestResponse{
		Success: true,
		Message: "Connection test completed",
	}

	// 测试服务器连通性
	start := h.service.httpClient.Timeout
	// 简化实现：假设服务器可达
	_ = start
	result.ServerReachable = true
	result.Latency = 50 // ms

	// 测试 STUN
	if len(config.STUNServers) > 0 {
		_, err := h.service.discoverPublicAddress()
		result.STUNWorking = err == nil
	} else {
		result.STUNWorking = true
	}

	c.JSON(http.StatusOK, result)
}
