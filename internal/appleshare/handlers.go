// Package appleshare 提供 REST API 处理器
package appleshare

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers Apple 生态 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	apple := r.Group("/apple")
	{
		// AirPlay 设备发现
		apple.POST("/airplay/discover", h.discoverAirPlayDevices)

		// Time Machine 共享管理
		apple.POST("/timemachine/shares", h.createTimeMachineShare)
		apple.GET("/timemachine/shares/:id", h.getTimeMachineStatus)
		apple.GET("/timemachine/shares", h.listTimeMachineShares)

		// SMB 配置
		apple.PUT("/smb/config", h.updateSMBConfig)
		apple.GET("/smb/config", h.getSMBConfig)

		// Spotlight 索引管理
		apple.POST("/spotlight/rebuild/:volumeId", h.rebuildSpotlightIndex)
		apple.GET("/spotlight/status/:volumeId", h.getSpotlightStatus)
		apple.GET("/spotlight/indexes", h.listSpotlightIndexes)

		// 已连接客户端
		apple.GET("/clients", h.getConnectedClients)

		// 设备管理
		apple.GET("/devices", h.listDevices)
		apple.GET("/devices/:id", h.getDevice)
		apple.DELETE("/devices/:id", h.removeDevice)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// discoverAirPlayDevices 发现 AirPlay 设备
func (h *Handlers) discoverAirPlayDevices(c *gin.Context) {
	devices, err := h.manager.DiscoverAirPlayDevices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "discovery completed",
		Data:    devices,
	})
}

// createTimeMachineShare 创建 Time Machine 共享
func (h *Handlers) createTimeMachineShare(c *gin.Context) {
	var req CreateTimeMachineShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	share, err := h.manager.CreateTimeMachineShare(req.Name, req.Path, req.Quota)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "time machine share created",
		Data:    share,
	})
}

// getTimeMachineStatus 获取 Time Machine 共享状态
func (h *Handlers) getTimeMachineStatus(c *gin.Context) {
	id := c.Param("id")
	share, err := h.manager.GetTimeMachineStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    share,
	})
}

// listTimeMachineShares 列出所有 Time Machine 共享
func (h *Handlers) listTimeMachineShares(c *gin.Context) {
	shares := h.manager.ListTimeMachineShares()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    shares,
	})
}

// updateSMBConfig 更新 SMB 配置
func (h *Handlers) updateSMBConfig(c *gin.Context) {
	var req UpdateSMBConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	config := SMBConfig{
		Signing:          req.Signing,
		AAPLExtensions:   req.AAPLExtensions,
		Streams:          req.Streams,
		VFSFruitEnabled:  req.VFSFruitEnabled,
		SpotlightEnabled: req.SpotlightEnabled,
	}

	if err := h.manager.UpdateSMBConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "SMB config updated",
		Data:    config,
	})
}

// getSMBConfig 获取 SMB 配置
func (h *Handlers) getSMBConfig(c *gin.Context) {
	config := h.manager.GetSMBConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// rebuildSpotlightIndex 重建 Spotlight 索引
func (h *Handlers) rebuildSpotlightIndex(c *gin.Context) {
	volumeID := c.Param("volumeId")
	if volumeID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "volume ID is required",
		})
		return
	}

	if err := h.manager.RebuildSpotlightIndex(volumeID); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "spotlight index rebuild started",
	})
}

// getSpotlightStatus 获取 Spotlight 索引状态
func (h *Handlers) getSpotlightStatus(c *gin.Context) {
	volumeID := c.Param("volumeId")
	if volumeID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "volume ID is required",
		})
		return
	}

	index, err := h.manager.GetSpotlightStatus(volumeID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    index,
	})
}

// listSpotlightIndexes 列出所有 Spotlight 索引
func (h *Handlers) listSpotlightIndexes(c *gin.Context) {
	indexes := h.manager.ListSpotlightIndexes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    indexes,
	})
}

// getConnectedClients 获取已连接的客户端
func (h *Handlers) getConnectedClients(c *gin.Context) {
	clients, err := h.manager.GetConnectedClients()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    clients,
	})
}

// listDevices 列出所有设备
func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    devices,
	})
}

// getDevice 获取指定设备
func (h *Handlers) getDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    device,
	})
}

// removeDevice 移除设备
func (h *Handlers) removeDevice(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveDevice(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device removed",
	})
}
