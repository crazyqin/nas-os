package rdma

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler RDMA HTTP处理器.
type Handler struct {
	manager *RDMAManager
}

// NewHandler 创建RDMA处理器.
func NewHandler(manager *RDMAManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rdma := rg.Group("/rdma")
	{
		rdma.GET("/status", h.GetStatus)
		rdma.GET("/connections", h.GetConnections)
		rdma.GET("/stats", h.GetStats)
		rdma.GET("/devices", h.GetDevices)
		rdma.POST("/config", h.UpdateConfig)
		rdma.POST("/enable", h.Enable)
		rdma.POST("/disable", h.Disable)
		rdma.GET("/multipath", h.GetMultipath)
	}
}

// GetStatus 获取RDMA状态
// GET /api/v1/rdma/status.
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    status,
	})
}

// GetConnections 获取活跃连接列表
// GET /api/v1/rdma/connections.
func (h *Handler) GetConnections(c *gin.Context) {
	conns := h.manager.GetConnections()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    conns,
	})
}

// GetStats 获取性能统计
// GET /api/v1/rdma/stats.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    stats,
	})
}

// GetDevices 获取RDMA设备列表
// GET /api/v1/rdma/devices.
func (h *Handler) GetDevices(c *gin.Context) {
	devices := h.manager.GetDevices()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    devices,
	})
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	Enabled          *bool             `json:"enabled,omitempty"`
	DefaultTransport *TransportType    `json:"defaultTransport,omitempty"`
	FallbackToTCP    *bool             `json:"fallbackToTcp,omitempty"`
	MaxLatencyMs     *float64          `json:"maxLatencyMs,omitempty"`
	MaxPacketLoss    *float64          `json:"maxPacketLoss,omitempty"`
	MaxQueueDepth    *int              `json:"maxQueueDepth,omitempty"`
	RateLimit        *RateLimitConfig  `json:"rateLimit,omitempty"`
	Congestion       *CongestionConfig `json:"congestion,omitempty"`
	Failover         *FailoverConfig   `json:"failover,omitempty"`
}

// UpdateConfig 更新RDMA配置
// POST /api/v1/rdma/config.
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.mu.Lock()
	config := h.manager.config

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.DefaultTransport != nil {
		config.DefaultTransport = *req.DefaultTransport
	}
	if req.FallbackToTCP != nil {
		config.FallbackToTCP = *req.FallbackToTCP
	}
	if req.MaxLatencyMs != nil {
		config.MaxLatencyMs = *req.MaxLatencyMs
	}
	if req.MaxPacketLoss != nil {
		config.MaxPacketLoss = *req.MaxPacketLoss
	}
	if req.MaxQueueDepth != nil {
		config.MaxQueueDepth = *req.MaxQueueDepth
	}
	if req.RateLimit != nil {
		config.RateLimit = *req.RateLimit
	}
	if req.Congestion != nil {
		config.Congestion = *req.Congestion
	}
	if req.Failover != nil {
		config.Failover = *req.Failover
	}

	h.manager.config = config
	h.manager.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "config updated",
		"data":    config,
	})
}

// Enable 启用RDMA
// POST /api/v1/rdma/enable.
func (h *Handler) Enable(c *gin.Context) {
	if err := h.manager.EnableRDMA(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to enable RDMA: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "RDMA enabled",
	})
}

// Disable 禁用RDMA
// POST /api/v1/rdma/disable.
func (h *Handler) Disable(c *gin.Context) {
	if err := h.manager.DisableRDMA(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to disable RDMA: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "RDMA disabled",
	})
}

// GetMultipath 获取多路径状态
// GET /api/v1/rdma/multipath.
func (h *Handler) GetMultipath(c *gin.Context) {
	groups := h.manager.GetMultipathStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    groups,
	})
}
