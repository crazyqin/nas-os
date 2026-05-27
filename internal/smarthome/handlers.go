package smarthome

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 智能家居HTTP处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建智能家居HTTP处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册智能家居API路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	sh := rg.Group("/smarthome")
	{
		// 设备管理
		sh.POST("/devices", h.AddDevice)
		sh.GET("/devices", h.ListDevices)
		sh.GET("/devices/:id", h.GetDevice)
		sh.PUT("/devices/:id", h.UpdateDevice)
		sh.DELETE("/devices/:id", h.DeleteDevice)
		sh.PUT("/devices/:id/state", h.UpdateDeviceState)
		sh.GET("/devices/room/:room_id", h.ListDevicesByRoom)
		sh.GET("/devices/type/:type", h.ListDevicesByType)

		// 房间管理
		sh.POST("/rooms", h.AddRoom)
		sh.GET("/rooms", h.ListRooms)
		sh.GET("/rooms/:id", h.GetRoom)
		sh.PUT("/rooms/:id", h.UpdateRoom)
		sh.DELETE("/rooms/:id", h.DeleteRoom)

		// 分组管理
		sh.POST("/groups", h.AddGroup)
		sh.GET("/groups", h.ListGroups)
		sh.GET("/groups/:id", h.GetGroup)
		sh.PUT("/groups/:id", h.UpdateGroup)
		sh.DELETE("/groups/:id", h.DeleteGroup)
		sh.POST("/groups/:id/devices/:device_id", h.AddDeviceToGroup)
		sh.DELETE("/groups/:id/devices/:device_id", h.RemoveDeviceFromGroup)

		// 自动化场景
		sh.POST("/scenes", h.AddScene)
		sh.GET("/scenes", h.ListScenes)
		sh.GET("/scenes/:id", h.GetScene)
		sh.PUT("/scenes/:id", h.UpdateScene)
		sh.DELETE("/scenes/:id", h.DeleteScene)
		sh.POST("/scenes/:id/execute", h.ExecuteScene)
		sh.POST("/scenes/:id/enable", h.EnableScene)
		sh.POST("/scenes/:id/disable", h.DisableScene)

		// 定时任务
		sh.POST("/tasks", h.AddScheduledTask)
		sh.GET("/tasks", h.ListScheduledTasks)
		sh.GET("/tasks/:id", h.GetScheduledTask)
		sh.PUT("/tasks/:id", h.UpdateScheduledTask)
		sh.DELETE("/tasks/:id", h.DeleteScheduledTask)

		// 能耗统计
		sh.GET("/energy/stats", h.GetEnergyStats)
		sh.GET("/energy/device/:device_id", h.GetDeviceEnergyStats)

		// 仪表盘
		sh.GET("/dashboard", h.GetDashboard)

		// 设备发现
		sh.POST("/discover", h.DiscoverDevices)

		// 事件
		sh.GET("/events", h.GetEvents)
	}
}

// ============================================================
// 设备管理 Handlers
// ============================================================

// AddDevice handles POST /api/v1/smarthome/devices
func (h *Handler) AddDevice(c *gin.Context) {
	var device Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddDevice(&device); err != nil {
		if err == ErrDeviceExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, device)
}

// GetDevice handles GET /api/v1/smarthome/devices/:id
func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, device)
}

// ListDevices handles GET /api/v1/smarthome/devices
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// ListDevicesByRoom handles GET /api/v1/smarthome/devices/room/:room_id
func (h *Handler) ListDevicesByRoom(c *gin.Context) {
	roomID := c.Param("room_id")
	devices := h.manager.ListDevicesByRoom(roomID)
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// ListDevicesByType handles GET /api/v1/smarthome/devices/type/:type
func (h *Handler) ListDevicesByType(c *gin.Context) {
	deviceType := DeviceType(c.Param("type"))
	devices := h.manager.ListDevicesByType(deviceType)
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// UpdateDevice handles PUT /api/v1/smarthome/devices/:id
func (h *Handler) UpdateDevice(c *gin.Context) {
	id := c.Param("id")
	var update Device
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateDevice(id, &update); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device updated"})
}

// DeleteDevice handles DELETE /api/v1/smarthome/devices/:id
func (h *Handler) DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDevice(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "device deleted"})
}

// UpdateDeviceState handles PUT /api/v1/smarthome/devices/:id/state
func (h *Handler) UpdateDeviceState(c *gin.Context) {
	id := c.Param("id")
	var state map[string]any
	if err := c.ShouldBindJSON(&state); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateDeviceState(id, state); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device state updated"})
}

// ============================================================
// 房间管理 Handlers
// ============================================================

// AddRoom handles POST /api/v1/smarthome/rooms
func (h *Handler) AddRoom(c *gin.Context) {
	var room Room
	if err := c.ShouldBindJSON(&room); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddRoom(&room); err != nil {
		if err == ErrRoomExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, room)
}

// GetRoom handles GET /api/v1/smarthome/rooms/:id
func (h *Handler) GetRoom(c *gin.Context) {
	id := c.Param("id")
	room, err := h.manager.GetRoom(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, room)
}

// ListRooms handles GET /api/v1/smarthome/rooms
func (h *Handler) ListRooms(c *gin.Context) {
	rooms := h.manager.ListRooms()
	c.JSON(http.StatusOK, gin.H{
		"rooms": rooms,
		"total": len(rooms),
	})
}

// UpdateRoom handles PUT /api/v1/smarthome/rooms/:id
func (h *Handler) UpdateRoom(c *gin.Context) {
	id := c.Param("id")
	var update Room
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateRoom(id, &update); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "room updated"})
}

// DeleteRoom handles DELETE /api/v1/smarthome/rooms/:id
func (h *Handler) DeleteRoom(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRoom(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "room deleted"})
}

// ============================================================
// 分组管理 Handlers
// ============================================================

// AddGroup handles POST /api/v1/smarthome/groups
func (h *Handler) AddGroup(c *gin.Context) {
	var group Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddGroup(&group); err != nil {
		if err == ErrGroupExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, group)
}

// GetGroup handles GET /api/v1/smarthome/groups/:id
func (h *Handler) GetGroup(c *gin.Context) {
	id := c.Param("id")
	group, err := h.manager.GetGroup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// ListGroups handles GET /api/v1/smarthome/groups
func (h *Handler) ListGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// UpdateGroup handles PUT /api/v1/smarthome/groups/:id
func (h *Handler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var update Group
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateGroup(id, &update); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "group updated"})
}

// DeleteGroup handles DELETE /api/v1/smarthome/groups/:id
func (h *Handler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "group deleted"})
}

// AddDeviceToGroup handles POST /api/v1/smarthome/groups/:id/devices/:device_id
func (h *Handler) AddDeviceToGroup(c *gin.Context) {
	groupID := c.Param("id")
	deviceID := c.Param("device_id")

	if err := h.manager.AddDeviceToGroup(deviceID, groupID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device added to group"})
}

// RemoveDeviceFromGroup handles DELETE /api/v1/smarthome/groups/:id/devices/:device_id
func (h *Handler) RemoveDeviceFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	deviceID := c.Param("device_id")

	if err := h.manager.RemoveDeviceFromGroup(deviceID, groupID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device removed from group"})
}

// ============================================================
// 自动化场景 Handlers
// ============================================================

// AddScene handles POST /api/v1/smarthome/scenes
func (h *Handler) AddScene(c *gin.Context) {
	var scene Scene
	if err := c.ShouldBindJSON(&scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddScene(&scene); err != nil {
		if err == ErrSceneExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, scene)
}

// GetScene handles GET /api/v1/smarthome/scenes/:id
func (h *Handler) GetScene(c *gin.Context) {
	id := c.Param("id")
	scene, err := h.manager.GetScene(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scene)
}

// ListScenes handles GET /api/v1/smarthome/scenes
func (h *Handler) ListScenes(c *gin.Context) {
	scenes := h.manager.ListScenes()
	c.JSON(http.StatusOK, gin.H{
		"scenes": scenes,
		"total":  len(scenes),
	})
}

// UpdateScene handles PUT /api/v1/smarthome/scenes/:id
func (h *Handler) UpdateScene(c *gin.Context) {
	id := c.Param("id")
	var update Scene
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateScene(id, &update); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "scene updated"})
}

// DeleteScene handles DELETE /api/v1/smarthome/scenes/:id
func (h *Handler) DeleteScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteScene(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "scene deleted"})
}

// ExecuteScene handles POST /api/v1/smarthome/scenes/:id/execute
func (h *Handler) ExecuteScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ExecuteScene(id); err != nil {
		if err == ErrSceneNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if err == ErrSceneDisabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "scene executed"})
}

// EnableScene handles POST /api/v1/smarthome/scenes/:id/enable
func (h *Handler) EnableScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.EnableScene(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "scene enabled"})
}

// DisableScene handles POST /api/v1/smarthome/scenes/:id/disable
func (h *Handler) DisableScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisableScene(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "scene disabled"})
}

// ============================================================
// 定时任务 Handlers
// ============================================================

// AddScheduledTask handles POST /api/v1/smarthome/tasks
func (h *Handler) AddScheduledTask(c *gin.Context) {
	var task ScheduledTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddScheduledTask(&task); err != nil {
		if err == ErrTaskExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, task)
}

// GetScheduledTask handles GET /api/v1/smarthome/tasks/:id
func (h *Handler) GetScheduledTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetScheduledTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// ListScheduledTasks handles GET /api/v1/smarthome/tasks
func (h *Handler) ListScheduledTasks(c *gin.Context) {
	tasks := h.manager.ListScheduledTasks()
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// UpdateScheduledTask handles PUT /api/v1/smarthome/tasks/:id
func (h *Handler) UpdateScheduledTask(c *gin.Context) {
	id := c.Param("id")
	var update ScheduledTask
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateScheduledTask(id, &update); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "task updated"})
}

// DeleteScheduledTask handles DELETE /api/v1/smarthome/tasks/:id
func (h *Handler) DeleteScheduledTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteScheduledTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
}

// ============================================================
// 能耗统计 Handlers
// ============================================================

// GetEnergyStats handles GET /api/v1/smarthome/energy/stats
func (h *Handler) GetEnergyStats(c *gin.Context) {
	stats := h.manager.GetEnergyStats()
	c.JSON(http.StatusOK, stats)
}

// GetDeviceEnergyStats handles GET /api/v1/smarthome/energy/device/:device_id
func (h *Handler) GetDeviceEnergyStats(c *gin.Context) {
	deviceID := c.Param("device_id")
	stats, err := h.manager.GetDeviceEnergyStats(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ============================================================
// 仪表盘 Handlers
// ============================================================

// GetDashboard handles GET /api/v1/smarthome/dashboard
func (h *Handler) GetDashboard(c *gin.Context) {
	summary := h.manager.GetDashboardSummary()
	c.JSON(http.StatusOK, summary)
}

// ============================================================
// 设备发现 Handlers
// ============================================================

// DiscoverDevices handles POST /api/v1/smarthome/discover
func (h *Handler) DiscoverDevices(c *gin.Context) {
	discovered := h.manager.DiscoverDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": discovered,
		"total":   len(discovered),
	})
}

// ============================================================
// 事件 Handlers
// ============================================================

// GetEvents handles GET /api/v1/smarthome/events
func (h *Handler) GetEvents(c *gin.Context) {
	limit := 50 // 默认返回50条
	events := h.manager.GetEvents(limit)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// ============================================================
// 能耗统计方法
// ============================================================

// GetEnergyStats 获取总能耗统计
func (m *Manager) GetEnergyStats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalKWh := 0.0
	todayKWh := 0.0
	today := time.Now().Truncate(24 * time.Hour)

	for _, readings := range m.energyData {
		for _, r := range readings {
			totalKWh += r.EnergyKWh
			if r.Timestamp.After(today) {
				todayKWh += r.EnergyKWh
			}
		}
	}

	return map[string]any{
		"total_kwh":    totalKWh,
		"today_kwh":    todayKWh,
		"device_count": len(m.energyData),
	}
}

// GetDeviceEnergyStats 获取单设备能耗统计
func (m *Manager) GetDeviceEnergyStats(deviceID string) (*EnergyStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	readings, ok := m.energyData[deviceID]
	if !ok || len(readings) == 0 {
		return nil, ErrDeviceNotFound
	}

	stats := &EnergyStats{
		DeviceID:     deviceID,
		ReadingCount: len(readings),
		StartTime:    readings[0].Timestamp,
		EndTime:      readings[len(readings)-1].Timestamp,
	}

	totalPower := 0.0
	for _, r := range readings {
		stats.TotalKWh += r.EnergyKWh
		totalPower += r.PowerW
		if r.PowerW > stats.PeakPowerW {
			stats.PeakPowerW = r.PowerW
		}
	}

	stats.AvgPowerW = totalPower / float64(len(readings))

	// 计算今日、本周、本月能耗
	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)

	for _, r := range readings {
		if r.Timestamp.After(today) {
			stats.DailyKWh += r.EnergyKWh
		}
		if r.Timestamp.After(weekAgo) {
			stats.WeeklyKWh += r.EnergyKWh
		}
		if r.Timestamp.After(monthAgo) {
			stats.MonthlyKWh += r.EnergyKWh
		}
	}

	return stats, nil
}

// GetDashboardSummary 获取仪表盘摘要
func (m *Manager) GetDashboardSummary() *DashboardSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &DashboardSummary{
		TotalDevices:  len(m.devices),
		TotalRooms:    len(m.rooms),
		TotalScenes:   len(m.scenes),
		DevicesByType: make(map[string]int),
		DevicesByRoom: make(map[string]int),
		UpdatedAt:     time.Now(),
	}

	// 设备统计
	for _, d := range m.devices {
		switch d.Status {
		case DeviceStatusOnline:
			summary.OnlineDevices++
		case DeviceStatusOffline:
			summary.OfflineDevices++
		}
		summary.DevicesByType[string(d.Type)]++
		if d.RoomID != "" {
			summary.DevicesByRoom[d.RoomID]++
		}
	}

	// 场景统计
	for _, s := range m.scenes {
		if s.Enabled {
			summary.ActiveScenes++
		}
	}

	// 能耗统计
	today := time.Now().Truncate(24 * time.Hour)
	for _, readings := range m.energyData {
		for _, r := range readings {
			summary.TotalEnergyKWh += r.EnergyKWh
			if r.Timestamp.After(today) {
				summary.TodayEnergyKWh += r.EnergyKWh
			}
		}
	}

	// 最近事件
	eventCount := 10
	if len(m.events) < eventCount {
		eventCount = len(m.events)
	}
	summary.RecentEvents = m.events[len(m.events)-eventCount:]

	return summary
}
