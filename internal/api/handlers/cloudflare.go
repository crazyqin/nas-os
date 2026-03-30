// Package handlers 提供 Cloudflare Tunnel API 端点
// v2.56.0 - 兵部实现
package handlers

import (
	"context"
	"time"

	"nas-os/internal/api"
	"nas-os/internal/tunnel"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CloudflareHandlers Cloudflare Tunnel API 处理器.
type CloudflareHandlers struct {
	tunnel  *tunnel.CloudflareTunnel
	logger  *zap.Logger
}

// NewCloudflareHandlers 创建 Cloudflare Tunnel 处理器.
func NewCloudflareHandlers(tunnel *tunnel.CloudflareTunnel, logger *zap.Logger) *CloudflareHandlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CloudflareHandlers{
		tunnel: tunnel,
		logger: logger,
	}
}

// RegisterRoutes 注册路由.
func (h *CloudflareHandlers) RegisterRoutes(r *gin.RouterGroup) {
	tunnelGroup := r.Group("/tunnel/cloudflare")
	{
		tunnelGroup.GET("/status", h.GetStatus)
		tunnelGroup.POST("/start", h.Start)
		tunnelGroup.POST("/stop", h.Stop)
		tunnelGroup.PUT("/config", h.UpdateConfig)
	}
}

// GetStatus 获取 Cloudflare Tunnel 状态
// @Summary 获取隧道状态
// @Description 获取 Cloudflare Tunnel 当前运行状态和统计信息
// @Tags tunnel
// @Produce json
// @Success 200 {object} api.Response
// @Router /tunnel/cloudflare/status [get].
func (h *CloudflareHandlers) GetStatus(c *gin.Context) {
	if h.tunnel == nil {
		api.NotFound(c, "Cloudflare Tunnel 未初始化")
		return
	}

	status := h.tunnel.GetStatus()

	api.OK(c, gin.H{
		"running":       status.Running,
		"tunnel_id":     status.TunnelID,
		"public_url":    status.PublicURL,
		"connection":    status.Connection,
		"stats":         status.Stats,
		"start_time":    status.StartTime.Format(time.RFC3339),
		"uptime_seconds": status.UptimeSeconds,
		"last_error":    status.LastError,
	})
}

// StartRequest 启动隧道请求.
type StartRequest struct {
	TimeoutSeconds int `json:"timeout_seconds"` // 启动超时时间（可选）
}

// Start 启动 Cloudflare Tunnel
// @Summary 启动隧道
// @Description 启动 Cloudflare Tunnel 内网穿透服务
// @Tags tunnel
// @Accept json
// @Produce json
// @Param request body StartRequest false "启动参数"
// @Success 200 {object} api.Response
// @Router /tunnel/cloudflare/start [post].
func (h *CloudflareHandlers) Start(c *gin.Context) {
	if h.tunnel == nil {
		api.NotFound(c, "Cloudflare Tunnel 未初始化")
		return
	}

	// 检查是否已运行
	if h.tunnel.IsRunning() {
		api.Conflict(c, "隧道已在运行中")
		return
	}

	var req StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 请求体可选，忽略解析错误
		req.TimeoutSeconds = 30
	}

	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 30
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.TimeoutSeconds)*time.Second)
	defer cancel()

	h.logger.Info("启动 Cloudflare Tunnel",
		zap.Int("timeout_seconds", req.TimeoutSeconds),
	)

	if err := h.tunnel.Start(ctx); err != nil {
		h.logger.Error("启动失败", zap.Error(err))

		// 根据错误类型返回不同响应
		switch err {
		case tunnel.ErrTunnelAlreadyRunning:
			api.Conflict(c, "隧道已在运行中")
		case tunnel.ErrCloudflaredNotFound:
			api.ServiceUnavailable(c, "cloudflared 未安装")
		case tunnel.ErrInvalidCredentials:
			api.Unauthorized(c, "Cloudflare 凭证无效")
		case tunnel.ErrTunnelTokenRequired:
			api.BadRequest(c, "缺少 Tunnel Token")
		default:
			api.InternalError(c, "启动隧道失败: "+err.Error())
		}
		return
	}

	status := h.tunnel.GetStatus()

	api.OKWithMessage(c, "Cloudflare Tunnel 已启动", gin.H{
		"tunnel_id":  status.TunnelID,
		"public_url": status.PublicURL,
		"start_time": status.StartTime.Format(time.RFC3339),
	})
}

// Stop 停止 Cloudflare Tunnel
// @Summary 停止隧道
// @Description 停止 Cloudflare Tunnel 内网穿透服务
// @Tags tunnel
// @Produce json
// @Success 200 {object} api.Response
// @Router /tunnel/cloudflare/stop [post].
func (h *CloudflareHandlers) Stop(c *gin.Context) {
	if h.tunnel == nil {
		api.NotFound(c, "Cloudflare Tunnel 未初始化")
		return
	}

	// 检查是否正在运行
	if !h.tunnel.IsRunning() {
		api.BadRequest(c, "隧道未在运行")
		return
	}

	h.logger.Info("停止 Cloudflare Tunnel")

	if err := h.tunnel.Stop(); err != nil {
		h.logger.Error("停止失败", zap.Error(err))

		switch err {
		case tunnel.ErrTunnelNotRunning:
			api.BadRequest(c, "隧道未在运行")
		default:
			api.InternalError(c, "停止隧道失败: "+err.Error())
		}
		return
	}

	api.OKWithMessage(c, "Cloudflare Tunnel 已停止", gin.H{
		"stopped_at": time.Now().Format(time.RFC3339),
	})
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	Token              string                   `json:"token"`                // Tunnel Token
	APIToken           string                   `json:"api_token"`            // API Token
	AccountID          string                   `json:"account_id"`           // Account ID
	TunnelID           string                   `json:"tunnel_id"`            // Tunnel ID
	TunnelName         string                   `json:"tunnel_name"`          // Tunnel 名称
	ZoneID             string                   `json:"zone_id"`              // Zone ID
	ZoneName           string                   `json:"zone_name"`            // Zone 名称
	Subdomain          string                   `json:"subdomain"`            // 子域名
	LocalServices      []tunnel.CloudflareService `json:"local_services"`    // 本地服务配置
	MetricsPort        int                      `json:"metrics_port"`         // Metrics 端口
	NoAutoUpdate       bool                     `json:"no_auto_update"`       // 禁止自动更新
	ReconnectInterval  int                      `json:"reconnect_interval"`   // 重连间隔(秒)
	MaxReconnectAttempts int                    `json:"max_reconnect_attempts"` // 最大重连次数
	HeartbeatInterval  int                      `json:"heartbeat_interval"`   // 心跳间隔(秒)
}

// UpdateConfig 更新 Cloudflare Tunnel 配置
// @Summary 更新隧道配置
// @Description 更新 Cloudflare Tunnel 配置（需要重启隧道才能生效）
// @Tags tunnel
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "配置参数"
// @Success 200 {object} api.Response
// @Router /tunnel/cloudflare/config [put].
func (h *CloudflareHandlers) UpdateConfig(c *gin.Context) {
	if h.tunnel == nil {
		api.NotFound(c, "Cloudflare Tunnel 未初始化")
		return
	}

	var req UpdateConfigRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 获取当前配置
	status := h.tunnel.GetStatus()
	currentConfig := status.Config

	// 更新配置字段（只更新非空值）
	if req.Token != "" {
		currentConfig.Token = req.Token
	}
	if req.APIToken != "" {
		currentConfig.APIToken = req.APIToken
	}
	if req.AccountID != "" {
		currentConfig.AccountID = req.AccountID
	}
	if req.TunnelID != "" {
		currentConfig.TunnelID = req.TunnelID
	}
	if req.TunnelName != "" {
		currentConfig.TunnelName = req.TunnelName
	}
	if req.ZoneID != "" {
		currentConfig.ZoneID = req.ZoneID
	}
	if req.ZoneName != "" {
		currentConfig.ZoneName = req.ZoneName
	}
	if req.Subdomain != "" {
		currentConfig.Subdomain = req.Subdomain
	}
	if len(req.LocalServices) > 0 {
		currentConfig.LocalServices = req.LocalServices
	}
	if req.MetricsPort > 0 && req.MetricsPort <= 65535 {
		currentConfig.MetricsPort = req.MetricsPort
	}
	currentConfig.NoAutoUpdate = req.NoAutoUpdate
	if req.ReconnectInterval > 0 {
		currentConfig.ReconnectInterval = req.ReconnectInterval
	}
	if req.MaxReconnectAttempts > 0 {
		currentConfig.MaxReconnectAttempts = req.MaxReconnectAttempts
	}
	if req.HeartbeatInterval > 0 {
		currentConfig.HeartbeatInterval = req.HeartbeatInterval
	}

	h.logger.Info("更新 Cloudflare Tunnel 配置",
		zap.String("tunnel_id", currentConfig.TunnelID),
		zap.Int("services", len(currentConfig.LocalServices)),
	)

	// 注意：配置更新后需要重启隧道才能生效
	// 这里只返回更新后的配置，不自动重启
	// 如果隧道正在运行，提示用户需要重启

	needRestart := h.tunnel.IsRunning()

	message := "配置已更新"
	if needRestart {
		message = "配置已更新，请重启隧道使配置生效"
	}

	api.OKWithMessage(c, "配置已更新", gin.H{
		"config":       currentConfig,
		"need_restart": needRestart,
		"message":      message,
	})
}