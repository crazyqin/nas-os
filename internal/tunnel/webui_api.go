// Package tunnel 提供内网穿透 WebUI API
// 实现零配置远程访问管理界面后端
package tunnel

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebUIHandler WebUI API处理器
type WebUIHandler struct {
	frpManager    *FRPManager
	tunnelService *TunnelService
	logger        *zap.Logger
}

// NewWebUIHandler 创建WebUI处理器
func NewWebUIHandler(frpManager *FRPManager, tunnelService *TunnelService, logger *zap.Logger) *WebUIHandler {
	return &WebUIHandler{
		frpManager:    frpManager,
		tunnelService: tunnelService,
		logger:        logger,
	}
}

// RegisterRoutes 注册WebUI路由
func (h *WebUIHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 内网穿透管理
	tunnel := r.Group("/tunnel")
	{
		// 仪表盘
		tunnel.GET("/dashboard", h.GetDashboard)

		// FRP管理
		frp := tunnel.Group("/frp")
		{
			frp.GET("/status", h.GetFRPStatus)
			frp.POST("/start", h.StartFRP)
			frp.POST("/stop", h.StopFRP)
			frp.POST("/restart", h.RestartFRP)
			frp.GET("/config", h.GetFRPConfig)
			frp.PUT("/config", h.UpdateFRPConfig)

			// 代理管理
			frp.GET("/proxies", h.ListProxies)
			frp.POST("/proxies", h.CreateProxy)
			frp.GET("/proxies/:name", h.GetProxy)
			frp.PUT("/proxies/:name", h.UpdateProxy)
			frp.DELETE("/proxies/:name", h.DeleteProxy)

			// 零配置
			frp.POST("/quick-connect", h.QuickConnect)
			frp.DELETE("/quick-disconnect/:name", h.QuickDisconnect)
		}

		// P2P隧道管理
		p2p := tunnel.Group("/p2p")
		{
			p2p.GET("/status", h.GetP2PStatus)
			p2p.POST("/connect", h.ConnectP2P)
			p2p.GET("/connections", h.ListConnections)
		}

		// NAT检测
		tunnel.POST("/detect-nat", h.DetectNAT)
		tunnel.GET("/public-ip", h.GetPublicIP)

		// 预设服务
		tunnel.GET("/presets", h.GetPresetServices)
	}

	// 向后兼容：保留原有路由
	r.GET("/tunnel/status", h.GetDashboard)
	r.POST("/tunnel/connect", h.StartFRP)
	r.POST("/tunnel/disconnect", h.StopFRP)
}

// ========== 仪表盘 ==========

// GetDashboard 获取仪表盘数据
func (h *WebUIHandler) GetDashboard(c *gin.Context) {
	// 获取FRP仪表盘数据
	frpDashboard := h.frpManager.GetDashboardData()

	// 获取P2P状态
	p2pStatus := h.tunnelService.manager.GetStatus()

	// 构建响应
	response := DashboardResponse{
		OverallStatus: OverallStatus{
			Connected:   frpDashboard.Status.Connected || p2pStatus.State == StateConnected,
			PrimaryMode: "frp",
			Uptime:      frpDashboard.Status.Uptime,
			LastChecked: time.Now(),
			PublicIP:    p2pStatus.PublicIP,
		},
		FRP: FRPDashboardResponse{
			Enabled:      h.frpManager.config.Enabled,
			Connected:    frpDashboard.Status.Connected,
			ServerAddr:   frpDashboard.Status.ServerAddr,
			DeviceID:     frpDashboard.Status.DeviceID,
			ProxyCount:   frpDashboard.ProxyCount,
			TotalTraffic: frpDashboard.TotalTraffic,
			Uptime:       frpDashboard.Status.Uptime,
			Proxies:      frpDashboard.Proxies,
		},
		P2P: P2PDashboardResponse{
			Connected:   p2pStatus.State == StateConnected,
			TunnelCount: p2pStatus.ActiveTunnels,
			NATType:     string(p2pStatus.NATType),
			PublicIP:    p2pStatus.PublicIP,
		},
	}

	c.JSON(http.StatusOK, response)
}

// DashboardResponse 仪表盘响应
type DashboardResponse struct {
	OverallStatus OverallStatus        `json:"overallStatus"`
	FRP           FRPDashboardResponse `json:"frp"`
	P2P           P2PDashboardResponse `json:"p2p"`
}

// OverallStatus 整体状态
type OverallStatus struct {
	Connected   bool          `json:"connected"`
	PrimaryMode string        `json:"primaryMode"` // frp, p2p, auto
	Uptime      time.Duration `json:"uptime"`
	LastChecked time.Time     `json:"lastChecked"`
	PublicIP    string        `json:"publicIp"`
}

// FRPDashboardResponse FRP仪表盘响应
type FRPDashboardResponse struct {
	Enabled      bool             `json:"enabled"`
	Connected    bool             `json:"connected"`
	ServerAddr   string           `json:"serverAddr"`
	DeviceID     string           `json:"deviceId"`
	ProxyCount   int              `json:"proxyCount"`
	TotalTraffic uint64           `json:"totalTraffic"`
	Uptime       time.Duration    `json:"uptime"`
	Proxies      []FRPProxyStatus `json:"proxies"`
}

// P2PDashboardResponse P2P仪表盘响应
type P2PDashboardResponse struct {
	Connected   bool   `json:"connected"`
	TunnelCount int    `json:"tunnelCount"`
	NATType     string `json:"natType"`
	PublicIP    string `json:"publicIp"`
}

// ========== FRP管理 ==========

// GetFRPStatus 获取FRP状态
func (h *WebUIHandler) GetFRPStatus(c *gin.Context) {
	status := h.frpManager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// StartFRP 启动FRP
func (h *WebUIHandler) StartFRP(c *gin.Context) {
	if err := h.frpManager.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "FRP服务已启动"})
}

// StopFRP 停止FRP
func (h *WebUIHandler) StopFRP(c *gin.Context) {
	if err := h.frpManager.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "FRP服务已停止"})
}

// RestartFRP 重启FRP
func (h *WebUIHandler) RestartFRP(c *gin.Context) {
	if err := h.frpManager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "FRP服务已重启"})
}

// GetFRPConfig 获取FRP配置
func (h *WebUIHandler) GetFRPConfig(c *gin.Context) {
	config := h.frpManager.config
	c.JSON(http.StatusOK, config)
}

// UpdateFRPConfig 更新FRP配置
func (h *WebUIHandler) UpdateFRPConfig(c *gin.Context) {
	var config FRPConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.frpManager.config = &config
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// ========== 代理管理 ==========

// ListProxies 列出代理
func (h *WebUIHandler) ListProxies(c *gin.Context) {
	proxies := h.frpManager.ListProxies()
	c.JSON(http.StatusOK, proxies)
}

// CreateProxy 创建代理
func (h *WebUIHandler) CreateProxy(c *gin.Context) {
	var proxy FRPProxyConfig
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.frpManager.AddProxy(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, proxy)
}

// GetProxy 获取代理详情
func (h *WebUIHandler) GetProxy(c *gin.Context) {
	name := c.Param("name")
	proxies := h.frpManager.ListProxies()

	for _, p := range proxies {
		if p.Name == name {
			c.JSON(http.StatusOK, p)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "代理不存在"})
}

// UpdateProxy 更新代理
func (h *WebUIHandler) UpdateProxy(c *gin.Context) {
	name := c.Param("name")

	var proxy FRPProxyConfig
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proxy.Name = name

	// 先删除再添加
	_ = h.frpManager.RemoveProxy(name)
	if err := h.frpManager.AddProxy(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, proxy)
}

// DeleteProxy 删除代理
func (h *WebUIHandler) DeleteProxy(c *gin.Context) {
	name := c.Param("name")

	if err := h.frpManager.RemoveProxy(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "代理已删除"})
}

// ========== 零配置API ==========

// QuickConnect 快速连接
func (h *WebUIHandler) QuickConnect(c *gin.Context) {
	var req QuickConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.frpManager.QuickConnect(req.LocalPort, req.ServiceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// QuickConnectRequest 快速连接请求
type QuickConnectRequest struct {
	LocalPort   int    `json:"localPort" binding:"required,min=1,max=65535"`
	ServiceName string `json:"serviceName"` // 服务名称（可选，用于生成代理名）
}

// QuickDisconnect 快速断开
func (h *WebUIHandler) QuickDisconnect(c *gin.Context) {
	name := c.Param("name")

	if err := h.frpManager.RemoveProxy(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已断开连接"})
}

// ========== P2P管理 ==========

// GetP2PStatus 获取P2P状态
func (h *WebUIHandler) GetP2PStatus(c *gin.Context) {
	status := h.tunnelService.manager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ConnectP2P 连接P2P
func (h *WebUIHandler) ConnectP2P(c *gin.Context) {
	var req ConnectP2PRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	_, err := h.tunnelService.ConnectToPeer(ctx, req.PeerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "P2P连接已建立"})
}

// ConnectP2PRequest P2P连接请求
type ConnectP2PRequest struct {
	PeerID string `json:"peerId" binding:"required"`
}

// ListConnections 列出P2P连接
func (h *WebUIHandler) ListConnections(c *gin.Context) {
	status := h.tunnelService.manager.GetStatus()
	c.JSON(http.StatusOK, status.Tunnels)
}

// ========== NAT检测 ==========

// DetectNAT 检测NAT类型
func (h *WebUIHandler) DetectNAT(c *gin.Context) {
	// 使用STUN检测NAT
	status := h.tunnelService.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"natType":  string(status.NATType),
		"publicIp": status.PublicIP,
	})
}

// GetPublicIP 获取公网IP
func (h *WebUIHandler) GetPublicIP(c *gin.Context) {
	status := h.tunnelService.manager.GetStatus()
	if status.PublicIP == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "无法获取公网IP"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"publicIp": status.PublicIP})
}

// ========== 预设服务 ==========

// PresetService 预设服务配置
type PresetService struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LocalPort   int    `json:"localPort"`
	Protocol    string `json:"protocol"`
	Icon        string `json:"icon"`
}

// GetPresetServices 获取预设服务列表
func (h *WebUIHandler) GetPresetServices(c *gin.Context) {
	presets := []PresetService{
		{Name: "web", Description: "Web管理界面", LocalPort: 80, Protocol: "tcp", Icon: "web"},
		{Name: "https", Description: "HTTPS服务", LocalPort: 443, Protocol: "tcp", Icon: "lock"},
		{Name: "ssh", Description: "SSH远程登录", LocalPort: 22, Protocol: "tcp", Icon: "terminal"},
		{Name: "smb", Description: "SMB文件共享", LocalPort: 445, Protocol: "tcp", Icon: "folder"},
		{Name: "ftp", Description: "FTP文件传输", LocalPort: 21, Protocol: "tcp", Icon: "upload"},
		{Name: "webdav", Description: "WebDAV服务", LocalPort: 5005, Protocol: "tcp", Icon: "cloud"},
		{Name: "mysql", Description: "MySQL数据库", LocalPort: 3306, Protocol: "tcp", Icon: "database"},
		{Name: "postgresql", Description: "PostgreSQL数据库", LocalPort: 5432, Protocol: "tcp", Icon: "database"},
		{Name: "redis", Description: "Redis缓存", LocalPort: 6379, Protocol: "tcp", Icon: "memory"},
		{Name: "transmission", Description: "Transmission下载", LocalPort: 9091, Protocol: "tcp", Icon: "download"},
	}
	c.JSON(http.StatusOK, presets)
}
