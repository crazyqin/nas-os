package rdmanfs

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler NFS over RDMA HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建 NFS over RDMA 处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	nfsrdma := rg.Group("/nfsrdma")
	{
		nfsrdma.GET("/status", h.GetStatus)
		nfsrdma.GET("/devices", h.ListDevices)
		nfsrdma.POST("/devices/detect", h.DetectDevices)
		nfsrdma.GET("/devices/:name", h.GetDevice)

		nfsrdma.GET("/config", h.GetConfig)
		nfsrdma.PUT("/config", h.UpdateConfig)

		nfsrdma.POST("/start", h.StartService)
		nfsrdma.POST("/stop", h.StopService)

		nfsrdma.GET("/exports", h.ListExports)
		nfsrdma.POST("/exports", h.AddExport)
		nfsrdma.DELETE("/exports", h.RemoveExport)

		nfsrdma.GET("/stats", h.GetStats)
		nfsrdma.POST("/stats/collect", h.CollectStats)
	}
}

// GetStatus 获取整体状态
// GET /api/v1/nfsrdma/status.
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    status,
	})
}

// ListDevices 列出 RDMA 设备
// GET /api/v1/nfsrdma/devices.
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.GetDevices()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    devices,
	})
}

// DetectDevices 检测 RDMA 设备
// POST /api/v1/nfsrdma/devices/detect.
func (h *Handler) DetectDevices(c *gin.Context) {
	devices, err := h.manager.DetectRDMADevices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    devices,
	})
}

// GetDevice 获取单个设备
// GET /api/v1/nfsrdma/devices/:name.
func (h *Handler) GetDevice(c *gin.Context) {
	name := c.Param("name")
	dev, err := h.manager.GetDeviceByName(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    dev,
	})
}

// GetConfig 获取 NFS RDMA 配置
// GET /api/v1/nfsrdma/config.
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.manager.GetNFSRDMAConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    cfg,
	})
}

// UpdateConfig 更新 NFS RDMA 配置
// PUT /api/v1/nfsrdma/config.
func (h *Handler) UpdateConfig(c *gin.Context) {
	var cfg NFSRDMAConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.ConfigureNFSRDMA(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// StartService 启动 NFS RDMA 服务
// POST /api/v1/nfsrdma/start.
func (h *Handler) StartService(c *gin.Context) {
	if err := h.manager.StartService(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// StopService 停止 NFS RDMA 服务
// POST /api/v1/nfsrdma/stop.
func (h *Handler) StopService(c *gin.Context) {
	if err := h.manager.StopService(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// ListExports 列出 NFS 导出
// GET /api/v1/nfsrdma/exports.
func (h *Handler) ListExports(c *gin.Context) {
	exports := h.manager.ListExports()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    exports,
	})
}

// AddExport 添加 NFS 导出
// POST /api/v1/nfsrdma/exports.
func (h *Handler) AddExport(c *gin.Context) {
	var export NFSExport
	if err := c.ShouldBindJSON(&export); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.AddExport(export); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// RemoveExport 移除 NFS 导出
// DELETE /api/v1/nfsrdma/exports?path=/export/data.
func (h *Handler) RemoveExport(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "path 参数不能为空",
		})
		return
	}
	if err := h.manager.RemoveExport(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// GetStats 获取性能统计
// GET /api/v1/nfsrdma/stats.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    stats,
	})
}

// CollectStats 触发性能统计采集
// POST /api/v1/nfsrdma/stats/collect.
func (h *Handler) CollectStats(c *gin.Context) {
	stats, err := h.manager.CollectStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    stats,
	})
}
