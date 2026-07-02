// Package wgvdeploy 提供 WireGuard 一键部署 API 处理器
package wgvdeploy

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers WireGuard 部署 API 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	wg := r.Group("/wgvdeploy")
	{
		// 服务状态和控制
		wg.GET("/status", h.getStatus)
		wg.POST("/start", h.startService)
		wg.POST("/stop", h.stopService)

		// 对端管理
		wg.GET("/peers", h.listPeers)
		wg.POST("/peers", h.addPeer)
		wg.GET("/peers/:id", h.getPeer)
		wg.PUT("/peers/:id", h.updatePeer)
		wg.DELETE("/peers/:id", h.deletePeer)
		wg.GET("/peers/:id/config", h.getPeerConfig)
		wg.GET("/peers/:id/qrcode", h.getPeerQRCode)

		// 流量监控
		wg.GET("/traffic", h.getTrafficStats)
		wg.GET("/traffic/history", h.getTrafficHistory)
		wg.GET("/traffic/alerts", h.getTrafficAlerts)

		// 配置模板
		wg.GET("/templates", h.getTemplates)
		wg.GET("/templates/:id", h.getTemplate)

		// 一键部署
		wg.POST("/deploy", h.deploy)

		// 服务端配置
		wg.GET("/server/config", h.getServerConfig)
	}
}

// ============================================================
// 服务状态和控制
// ============================================================

// getStatus 获取服务状态.
func (h *Handlers) getStatus(c *gin.Context) {
	status := h.engine.GetStatus()
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取服务状态成功",
		Data:    status,
	})
}

// startService 启动服务.
func (h *Handlers) startService(c *gin.Context) {
	if err := h.engine.Start(); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Code:    http.StatusConflict,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "WireGuard 服务已启动",
		Data:    h.engine.GetStatus(),
	})
}

// stopService 停止服务.
func (h *Handlers) stopService(c *gin.Context) {
	if err := h.engine.Stop(); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Code:    http.StatusConflict,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "WireGuard 服务已停止",
		Data:    h.engine.GetStatus(),
	})
}

// ============================================================
// 对端管理
// ============================================================

// listPeers 获取对端列表.
func (h *Handlers) listPeers(c *gin.Context) {
	peers := h.engine.ListPeers()
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取对端列表成功",
		Data:    peers,
	})
}

// addPeer 添加对端.
func (h *Handlers) addPeer(c *gin.Context) {
	var req CreatePeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	peer, err := h.engine.AddPeer(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "添加对端失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Code:    http.StatusCreated,
		Message: "添加对端成功",
		Data:    peer,
	})
}

// getPeer 获取对端信息.
func (h *Handlers) getPeer(c *gin.Context) {
	id := c.Param("id")
	peer, err := h.engine.GetPeer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取对端信息成功",
		Data:    peer,
	})
}

// updatePeer 更新对端.
func (h *Handlers) updatePeer(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	peer, err := h.engine.UpdatePeer(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "更新对端成功",
		Data:    peer,
	})
}

// deletePeer 删除对端.
func (h *Handlers) deletePeer(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeletePeer(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "删除对端成功",
	})
}

// getPeerConfig 获取客户端配置.
func (h *Handlers) getPeerConfig(c *gin.Context) {
	id := c.Param("id")
	config, err := h.engine.GenerateClientConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取客户端配置成功",
		Data: PeerConfig{
			Config: config,
		},
	})
}

// getPeerQRCode 获取客户端 QR 码.
func (h *Handlers) getPeerQRCode(c *gin.Context) {
	id := c.Param("id")
	format := c.DefaultQuery("format", "svg")

	// 先获取客户端配置
	config, err := h.engine.GenerateClientConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	// 生成 QR 码
	qrCode, err := GenerateQRCode(config, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "生成 QR 码失败: " + err.Error(),
		})
		return
	}

	// 编码为 Base64
	encoded := base64.StdEncoding.EncodeToString([]byte(qrCode))

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取 QR 码成功",
		Data: PeerQRCode{
			PeerID: id,
			Format: format,
			Base64: encoded,
		},
	})
}

// ============================================================
// 流量监控
// ============================================================

// getTrafficStats 获取流量统计.
func (h *Handlers) getTrafficStats(c *gin.Context) {
	stats := h.engine.GetTrafficStats()
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取流量统计成功",
		Data:    stats,
	})
}

// getTrafficHistory 获取历史流量.
func (h *Handlers) getTrafficHistory(c *gin.Context) {
	var req TrafficHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	history := h.engine.GetTrafficHistory(req)
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取历史流量成功",
		Data:    history,
	})
}

// getTrafficAlerts 获取流量告警.
func (h *Handlers) getTrafficAlerts(c *gin.Context) {
	alerts := h.engine.CheckTrafficAlerts()
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取流量告警成功",
		Data:    alerts,
	})
}

// ============================================================
// 配置模板
// ============================================================

// getTemplates 获取配置模板列表.
func (h *Handlers) getTemplates(c *gin.Context) {
	templates := h.engine.GetTemplates()
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取配置模板列表成功",
		Data:    templates,
	})
}

// getTemplate 获取指定配置模板.
func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")
	template, err := h.engine.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取配置模板成功",
		Data:    template,
	})
}

// ============================================================
// 一键部署
// ============================================================

// deploy 一键部署.
func (h *Handlers) deploy(c *gin.Context) {
	var req DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	result, err := h.engine.Deploy(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "部署失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: result.Message,
		Data:    result,
	})
}

// ============================================================
// 服务端配置
// ============================================================

// getServerConfig 获取服务端配置.
func (h *Handlers) getServerConfig(c *gin.Context) {
	config := h.engine.GetServerConfig()
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "获取服务端配置成功",
		Data:    config,
	})
}
