package smarthome

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Handler handles HTTP requests for smart home.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new smart home handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smarthome/devices", h.handleDevices)
	mux.HandleFunc("/api/v1/smarthome/device/", h.handleDevice)
	mux.HandleFunc("/api/v1/smarthome/devices/room/", h.handleDevicesByRoom)
	mux.HandleFunc("/api/v1/smarthome/devices/type/", h.handleDevicesByType)
	mux.HandleFunc("/api/v1/smarthome/rooms", h.handleRooms)
	mux.HandleFunc("/api/v1/smarthome/room/", h.handleRoom)
	mux.HandleFunc("/api/v1/smarthome/groups", h.handleGroups)
	mux.HandleFunc("/api/v1/smarthome/group/", h.handleGroup)
	mux.HandleFunc("/api/v1/smarthome/scenes", h.handleScenes)
	mux.HandleFunc("/api/v1/smarthome/scene/", h.handleScene)
	mux.HandleFunc("/api/v1/smarthome/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/smarthome/task/", h.handleTask)
	mux.HandleFunc("/api/v1/smarthome/energy/stats", h.handleEnergyStats)
	mux.HandleFunc("/api/v1/smarthome/energy/device/", h.handleDeviceEnergyStats)
	mux.HandleFunc("/api/v1/smarthome/dashboard", h.handleDashboard)
	mux.HandleFunc("/api/v1/smarthome/discover", h.handleDiscover)
	mux.HandleFunc("/api/v1/smarthome/events", h.handleEvents)
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ============================================================
// 设备管理
// ============================================================

func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices := h.manager.ListDevices()
		jsonOK(w, map[string]any{"devices": devices, "total": len(devices)})
	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.AddDevice(&device); err != nil {
			if err == ErrDeviceExists {
				jsonErr(w, http.StatusConflict, err.Error())
			} else {
				jsonErr(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, device)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/device/")
	if id == "" {
		jsonErr(w, http.StatusBadRequest, "device id required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		device, err := h.manager.GetDevice(id)
		if err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, device)
	case http.MethodPut:
		var update Device
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.UpdateDevice(id, &update); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "device updated"})
	case http.MethodDelete:
		if err := h.manager.DeleteDevice(id); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "device deleted"})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleDevicesByRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	roomID := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/devices/room/")
	devices := h.manager.ListDevicesByRoom(roomID)
	jsonOK(w, map[string]any{"devices": devices, "total": len(devices)})
}

func (h *Handler) handleDevicesByType(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceType := DeviceType(strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/devices/type/"))
	devices := h.manager.ListDevicesByType(deviceType)
	jsonOK(w, map[string]any{"devices": devices, "total": len(devices)})
}

// ============================================================
// 房间管理
// ============================================================

func (h *Handler) handleRooms(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rooms := h.manager.ListRooms()
		jsonOK(w, map[string]any{"rooms": rooms, "total": len(rooms)})
	case http.MethodPost:
		var room Room
		if err := json.NewDecoder(r.Body).Decode(&room); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.AddRoom(&room); err != nil {
			if err == ErrRoomExists {
				jsonErr(w, http.StatusConflict, err.Error())
			} else {
				jsonErr(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, room)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleRoom(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/room/")
	if id == "" {
		jsonErr(w, http.StatusBadRequest, "room id required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		room, err := h.manager.GetRoom(id)
		if err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, room)
	case http.MethodPut:
		var update Room
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.UpdateRoom(id, &update); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "room updated"})
	case http.MethodDelete:
		if err := h.manager.DeleteRoom(id); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "room deleted"})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ============================================================
// 分组管理
// ============================================================

func (h *Handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups := h.manager.ListGroups()
		jsonOK(w, map[string]any{"groups": groups, "total": len(groups)})
	case http.MethodPost:
		var group Group
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.AddGroup(&group); err != nil {
			if err == ErrGroupExists {
				jsonErr(w, http.StatusConflict, err.Error())
			} else {
				jsonErr(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, group)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleGroup(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/group/")
	parts := strings.Split(path, "/")
	id := parts[0]
	if id == "" {
		jsonErr(w, http.StatusBadRequest, "group id required")
		return
	}

	// /api/v1/smarthome/group/{id}/devices/{device_id}
	if len(parts) >= 3 && parts[1] == "devices" {
		deviceID := parts[2]
		switch r.Method {
		case http.MethodPost:
			if err := h.manager.AddDeviceToGroup(deviceID, id); err != nil {
				jsonErr(w, http.StatusNotFound, err.Error())
				return
			}
			jsonOK(w, map[string]string{"message": "device added to group"})
		case http.MethodDelete:
			if err := h.manager.RemoveDeviceFromGroup(deviceID, id); err != nil {
				jsonErr(w, http.StatusNotFound, err.Error())
				return
			}
			jsonOK(w, map[string]string{"message": "device removed from group"})
		default:
			jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		group, err := h.manager.GetGroup(id)
		if err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, group)
	case http.MethodPut:
		var update Group
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.UpdateGroup(id, &update); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "group updated"})
	case http.MethodDelete:
		if err := h.manager.DeleteGroup(id); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "group deleted"})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ============================================================
// 自动化场景
// ============================================================

func (h *Handler) handleScenes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		scenes := h.manager.ListScenes()
		jsonOK(w, map[string]any{"scenes": scenes, "total": len(scenes)})
	case http.MethodPost:
		var scene Scene
		if err := json.NewDecoder(r.Body).Decode(&scene); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.AddScene(&scene); err != nil {
			if err == ErrSceneExists {
				jsonErr(w, http.StatusConflict, err.Error())
			} else {
				jsonErr(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, scene)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleScene(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/scene/")
	parts := strings.Split(path, "/")
	id := parts[0]
	if id == "" {
		jsonErr(w, http.StatusBadRequest, "scene id required")
		return
	}

	// /api/v1/smarthome/scene/{id}/execute|enable|disable
	if len(parts) >= 2 {
		action := parts[1]
		switch action {
		case "execute":
			if r.Method != http.MethodPost {
				jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if err := h.manager.ExecuteScene(id); err != nil {
				switch err {
				case ErrSceneNotFound:
					jsonErr(w, http.StatusNotFound, err.Error())
				case ErrSceneDisabled:
					jsonErr(w, http.StatusBadRequest, err.Error())
				default:
					jsonErr(w, http.StatusInternalServerError, err.Error())
				}
				return
			}
			jsonOK(w, map[string]string{"message": "scene executed"})
			return
		case "enable":
			if r.Method != http.MethodPost {
				jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if err := h.manager.EnableScene(id); err != nil {
				jsonErr(w, http.StatusNotFound, err.Error())
				return
			}
			jsonOK(w, map[string]string{"message": "scene enabled"})
			return
		case "disable":
			if r.Method != http.MethodPost {
				jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if err := h.manager.DisableScene(id); err != nil {
				jsonErr(w, http.StatusNotFound, err.Error())
				return
			}
			jsonOK(w, map[string]string{"message": "scene disabled"})
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		scene, err := h.manager.GetScene(id)
		if err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, scene)
	case http.MethodPut:
		var update Scene
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.UpdateScene(id, &update); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "scene updated"})
	case http.MethodDelete:
		if err := h.manager.DeleteScene(id); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "scene deleted"})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ============================================================
// 定时任务
// ============================================================

func (h *Handler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks := h.manager.ListScheduledTasks()
		jsonOK(w, map[string]any{"tasks": tasks, "total": len(tasks)})
	case http.MethodPost:
		var task ScheduledTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.AddScheduledTask(&task); err != nil {
			if err == ErrTaskExists {
				jsonErr(w, http.StatusConflict, err.Error())
			} else {
				jsonErr(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, task)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/task/")
	if id == "" {
		jsonErr(w, http.StatusBadRequest, "task id required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		task, err := h.manager.GetScheduledTask(id)
		if err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, task)
	case http.MethodPut:
		var update ScheduledTask
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.manager.UpdateScheduledTask(id, &update); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "task updated"})
	case http.MethodDelete:
		if err := h.manager.DeleteScheduledTask(id); err != nil {
			jsonErr(w, http.StatusNotFound, err.Error())
			return
		}
		jsonOK(w, map[string]string{"message": "task deleted"})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ============================================================
// 能耗统计
// ============================================================

func (h *Handler) handleEnergyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	stats := h.manager.GetEnergyStats()
	jsonOK(w, stats)
}

func (h *Handler) handleDeviceEnergyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID := strings.TrimPrefix(r.URL.Path, "/api/v1/smarthome/energy/device/")
	stats, err := h.manager.GetDeviceEnergyStats(deviceID)
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, stats)
}

// ============================================================
// 仪表盘
// ============================================================

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	summary := h.manager.GetDashboardSummary()
	jsonOK(w, summary)
}

// ============================================================
// 设备发现
// ============================================================

func (h *Handler) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	discovered := h.manager.DiscoverDevices()
	jsonOK(w, map[string]any{"devices": discovered, "total": len(discovered)})
}

// ============================================================
// 事件
// ============================================================

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	limit := 50
	events := h.manager.GetEvents(limit)
	jsonOK(w, map[string]any{"events": events, "total": len(events)})
}

// ============================================================
// Manager 扩展方法（能耗统计、仪表盘）
// ============================================================

// GetEnergyStats 获取总能耗统计.
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

// GetDeviceEnergyStats 获取单设备能耗统计.
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

// GetDashboardSummary 获取仪表盘摘要.
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

	for _, s := range m.scenes {
		if s.Enabled {
			summary.ActiveScenes++
		}
	}

	today := time.Now().Truncate(24 * time.Hour)
	for _, readings := range m.energyData {
		for _, r := range readings {
			summary.TotalEnergyKWh += r.EnergyKWh
			if r.Timestamp.After(today) {
				summary.TodayEnergyKWh += r.EnergyKWh
			}
		}
	}

	eventCount := 10
	if len(m.events) < eventCount {
		eventCount = len(m.events)
	}
	summary.RecentEvents = m.events[len(m.events)-eventCount:]

	return summary
}
