// Package smarthub provides smart home hub functionality for NAS-OS.
package smarthub

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for smart home hub management.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new smart home hub HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes registers smart home hub API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	smarthub := rg.Group("/smarthub")
	{
		// Stats
		smarthub.GET("/stats", h.GetStats)

		// Device management
		smarthub.POST("/devices/discover", h.DiscoverDevices)
		smarthub.GET("/devices", h.ListDevices)
		smarthub.GET("/devices/:id", h.GetDevice)
		smarthub.POST("/devices", h.CreateDevice)
		smarthub.PUT("/devices/:id", h.UpdateDevice)
		smarthub.DELETE("/devices/:id", h.DeleteDevice)
		smarthub.POST("/devices/:id/control", h.ControlDevice)

		// Gateway management
		smarthub.GET("/gateways", h.ListGateways)
		smarthub.GET("/gateways/:id", h.GetGateway)
		smarthub.POST("/gateways/:id/start", h.StartGateway)
		smarthub.POST("/gateways/:id/stop", h.StopGateway)

		// Group management
		smarthub.GET("/groups", h.ListGroups)
		smarthub.GET("/groups/:id", h.GetGroup)
		smarthub.POST("/groups", h.CreateGroup)
		smarthub.PUT("/groups/:id", h.UpdateGroup)
		smarthub.DELETE("/groups/:id", h.DeleteGroup)

		// Room management
		smarthub.GET("/rooms", h.ListRooms)
		smarthub.POST("/rooms", h.CreateRoom)

		// Scene management
		smarthub.GET("/scenes", h.ListScenes)
		smarthub.GET("/scenes/:id", h.GetScene)
		smarthub.POST("/scenes", h.CreateScene)
		smarthub.PUT("/scenes/:id", h.UpdateScene)
		smarthub.DELETE("/scenes/:id", h.DeleteScene)
		smarthub.POST("/scenes/:id/run", h.RunScene)

		// Energy monitoring
		smarthub.GET("/energy", h.GetAllEnergyStats)
		smarthub.GET("/energy/:device_id", h.GetEnergyStats)

		// Voice control
		smarthub.POST("/voice", h.ProcessVoiceCommand)
		smarthub.GET("/voice/history", h.GetVoiceHistory)
	}
}

// ============================================================
// Stats
// ============================================================

// GetStats handles GET /api/v1/smarthub/stats.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// ============================================================
// Device handlers
// ============================================================

// DiscoverDevices handles POST /api/v1/smarthub/devices/discover.
func (h *Handler) DiscoverDevices(c *gin.Context) {
	var req DiscoverDevicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Methods) == 0 {
		req.Methods = []DiscoveryMethod{DiscoveryMDNS, DiscoverySSDP}
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 30
	}

	result, err := h.manager.DiscoverDevices(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListDevices handles GET /api/v1/smarthub/devices.
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// GetDevice handles GET /api/v1/smarthub/devices/:id.
func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, device)
}

// CreateDevice handles POST /api/v1/smarthub/devices.
func (h *Handler) CreateDevice(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	device, err := h.manager.CreateDevice(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, device)
}

// UpdateDevice handles PUT /api/v1/smarthub/devices/:id.
func (h *Handler) UpdateDevice(c *gin.Context) {
	id := c.Param("id")
	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	device, err := h.manager.UpdateDevice(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, device)
}

// DeleteDevice handles DELETE /api/v1/smarthub/devices/:id.
func (h *Handler) DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDevice(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "device deleted"})
}

// ControlDevice handles POST /api/v1/smarthub/devices/:id/control.
func (h *Handler) ControlDevice(c *gin.Context) {
	id := c.Param("id")
	var req ControlDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	device, err := h.manager.ControlDevice(id, req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, device)
}

// ============================================================
// Gateway handlers
// ============================================================

// ListGateways handles GET /api/v1/smarthub/gateways.
func (h *Handler) ListGateways(c *gin.Context) {
	gateways := h.manager.ListGateways()
	c.JSON(http.StatusOK, gin.H{
		"gateways": gateways,
		"total":    len(gateways),
	})
}

// GetGateway handles GET /api/v1/smarthub/gateways/:id.
func (h *Handler) GetGateway(c *gin.Context) {
	id := c.Param("id")
	gateway, err := h.manager.GetGateway(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gateway)
}

// StartGateway handles POST /api/v1/smarthub/gateways/:id/start.
func (h *Handler) StartGateway(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartGateway(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "gateway started"})
}

// StopGateway handles POST /api/v1/smarthub/gateways/:id/stop.
func (h *Handler) StopGateway(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopGateway(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "gateway stopped"})
}

// ============================================================
// Group handlers
// ============================================================

// ListGroups handles GET /api/v1/smarthub/groups.
func (h *Handler) ListGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// GetGroup handles GET /api/v1/smarthub/groups/:id.
func (h *Handler) GetGroup(c *gin.Context) {
	id := c.Param("id")
	group, err := h.manager.GetGroup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// CreateGroup handles POST /api/v1/smarthub/groups.
func (h *Handler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.manager.CreateGroup(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

// UpdateGroup handles PUT /api/v1/smarthub/groups/:id.
func (h *Handler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.manager.UpdateGroup(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup handles DELETE /api/v1/smarthub/groups/:id.
func (h *Handler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "group deleted"})
}

// ============================================================
// Room handlers
// ============================================================

// ListRooms handles GET /api/v1/smarthub/rooms.
func (h *Handler) ListRooms(c *gin.Context) {
	rooms := h.manager.ListRooms()
	c.JSON(http.StatusOK, gin.H{
		"rooms": rooms,
		"total": len(rooms),
	})
}

// CreateRoom handles POST /api/v1/smarthub/rooms.
func (h *Handler) CreateRoom(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room, err := h.manager.CreateRoom(req.Name)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, room)
}

// ============================================================
// Scene handlers
// ============================================================

// ListScenes handles GET /api/v1/smarthub/scenes.
func (h *Handler) ListScenes(c *gin.Context) {
	scenes := h.manager.ListScenes()
	c.JSON(http.StatusOK, gin.H{
		"scenes": scenes,
		"total":  len(scenes),
	})
}

// GetScene handles GET /api/v1/smarthub/scenes/:id.
func (h *Handler) GetScene(c *gin.Context) {
	id := c.Param("id")
	scene, err := h.manager.GetScene(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scene)
}

// CreateScene handles POST /api/v1/smarthub/scenes.
func (h *Handler) CreateScene(c *gin.Context) {
	var req CreateSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scene, err := h.manager.CreateScene(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scene)
}

// UpdateScene handles PUT /api/v1/smarthub/scenes/:id.
func (h *Handler) UpdateScene(c *gin.Context) {
	id := c.Param("id")
	var req CreateSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scene, err := h.manager.UpdateScene(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scene)
}

// DeleteScene handles DELETE /api/v1/smarthub/scenes/:id.
func (h *Handler) DeleteScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteScene(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "scene deleted"})
}

// RunScene handles POST /api/v1/smarthub/scenes/:id/run.
func (h *Handler) RunScene(c *gin.Context) {
	id := c.Param("id")
	execution, err := h.manager.RunScene(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, execution)
}

// ============================================================
// Energy handlers
// ============================================================

// GetAllEnergyStats handles GET /api/v1/smarthub/energy.
func (h *Handler) GetAllEnergyStats(c *gin.Context) {
	stats := h.manager.GetAllEnergyStats()
	c.JSON(http.StatusOK, gin.H{
		"energy_stats": stats,
		"total":        len(stats),
	})
}

// GetEnergyStats handles GET /api/v1/smarthub/energy/:device_id.
func (h *Handler) GetEnergyStats(c *gin.Context) {
	deviceID := c.Param("device_id")
	stats, err := h.manager.GetEnergyStats(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ============================================================
// Voice control handlers
// ============================================================

// ProcessVoiceCommand handles POST /api/v1/smarthub/voice.
func (h *Handler) ProcessVoiceCommand(c *gin.Context) {
	var req VoiceCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.manager.ProcessVoiceCommand(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetVoiceHistory handles GET /api/v1/smarthub/voice/history.
func (h *Handler) GetVoiceHistory(c *gin.Context) {
	history := h.manager.GetVoiceHistory()
	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
	})
}
