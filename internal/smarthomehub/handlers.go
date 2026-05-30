package smarthomehub

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建新的处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	smarthome := r.Group("/smarthome")
	{
		// 设备管理
		smarthome.GET("/devices", h.ListDevices)
		smarthome.POST("/devices/discover", h.DiscoverDevices)
		smarthome.POST("/devices", h.AddDevice)
		smarthome.GET("/devices/:id", h.GetDevice)
		smarthome.POST("/devices/:id/control", h.ControlDevice)

		// 场景管理
		smarthome.POST("/scenes", h.CreateScene)
		smarthome.POST("/scenes/:id/activate", h.ActivateScene)

		// 自动化管理
		smarthome.POST("/automations", h.CreateAutomation)

		// 房间管理
		smarthome.POST("/rooms", h.AddRoom)
		smarthome.GET("/rooms", h.GetRooms)

		// 状态查询
		smarthome.GET("/status", h.GetHubStatus)
	}
}

// ListDevices 列出设备
func (h *Handler) ListDevices(c *gin.Context) {
	roomID := c.Query("roomId")

	devices, err := h.manager.ListDevices(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// DiscoverDevices 发现设备
func (h *Handler) DiscoverDevices(c *gin.Context) {
	var req struct {
		Timeout int `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	devices, err := h.manager.DiscoverDevices(ctx, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices, "count": len(devices)})
}

// AddDevice 添加设备
func (h *Handler) AddDevice(c *gin.Context) {
	var device Device

	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的设备数据"})
		return
	}

	if device.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "设备ID不能为空"})
		return
	}

	if err := h.manager.AddDevice(device); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"device": device})
}

// GetDevice 获取设备
func (h *Handler) GetDevice(c *gin.Context) {
	deviceID := c.Param("id")

	device, err := h.manager.GetDevice(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"device": device})
}

// ControlDevice 控制设备
func (h *Handler) ControlDevice(c *gin.Context) {
	deviceID := c.Param("id")

	var req struct {
		Command    string                 `json:"command"`
		Parameters map[string]interface{} `json:"parameters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的控制命令"})
		return
	}

	if req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "命令不能为空"})
		return
	}

	if err := h.manager.ControlDevice(deviceID, req.Command, req.Parameters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "device_id": deviceID, "command": req.Command})
}

// CreateScene 创建场景
func (h *Handler) CreateScene(c *gin.Context) {
	var scene Scene

	if err := c.ShouldBindJSON(&scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的场景数据"})
		return
	}

	if scene.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "场景ID不能为空"})
		return
	}

	if err := h.manager.CreateScene(scene); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"scene": scene})
}

// ActivateScene 激活场景
func (h *Handler) ActivateScene(c *gin.Context) {
	sceneID := c.Param("id")

	if err := h.manager.ActivateScene(sceneID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "scene_id": sceneID})
}

// CreateAutomation 创建自动化规则
func (h *Handler) CreateAutomation(c *gin.Context) {
	var automation Automation

	if err := c.ShouldBindJSON(&automation); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的自动化数据"})
		return
	}

	if automation.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "自动化ID不能为空"})
		return
	}

	if err := h.manager.CreateAutomation(automation); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"automation": automation})
}

// AddRoom 添加房间
func (h *Handler) AddRoom(c *gin.Context) {
	var room Room

	if err := c.ShouldBindJSON(&room); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的房间数据"})
		return
	}

	if room.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "房间ID不能为空"})
		return
	}

	if err := h.manager.AddRoom(room); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"room": room})
}

// GetRooms 获取所有房间
func (h *Handler) GetRooms(c *gin.Context) {
	rooms, err := h.manager.GetRooms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rooms": rooms})
}

// GetHubStatus 获取中枢状态
func (h *Handler) GetHubStatus(c *gin.Context) {
	status, err := h.manager.GetHubStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}
