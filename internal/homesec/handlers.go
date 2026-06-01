// Package homesec 提供家庭安防系统 HTTP API 处理器
package homesec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Handler HTTP API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建 HTTP API 处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/homesec/devices", h.handleDevices)
	mux.HandleFunc("/api/homesec/devices/", h.handleDevice)
	mux.HandleFunc("/api/homesec/zones", h.handleZones)
	mux.HandleFunc("/api/homesec/zones/", h.handleZone)
	mux.HandleFunc("/api/homesec/events", h.handleEvents)
	mux.HandleFunc("/api/homesec/rules", h.handleRules)
	mux.HandleFunc("/api/homesec/rules/", h.handleRule)
	mux.HandleFunc("/api/homesec/schedules", h.handleSchedules)
	mux.HandleFunc("/api/homesec/schedules/", h.handleSchedule)
	mux.HandleFunc("/api/homesec/panel", h.handlePanel)
	mux.HandleFunc("/api/homesec/arm", h.handleArm)
	mux.HandleFunc("/api/homesec/disarm", h.handleDisarm)
	mux.HandleFunc("/api/homesec/score", h.handleScore)
}

// jsonResponse JSON 响应
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// errorResponse 错误响应
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// handleDevices 处理设备列表
func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices, err := h.manager.ListDevices()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, devices)

	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.AddDevice(&device)
		if err != nil {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, result)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleDevice 处理单个设备
func (h *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	// 提取设备 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/homesec/devices/")
	id := strings.Split(path, "/")[0]

	if id == "" {
		errorResponse(w, http.StatusBadRequest, "缺少设备 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		device, err := h.manager.GetDevice(id)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, device)

	case http.MethodPut:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.UpdateDevice(id, &device)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, result)

	case http.MethodDelete:
		if err := h.manager.DeleteDevice(id); err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "设备已删除"})

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleZones 处理区域列表
func (h *Handler) handleZones(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 获取所有区域
		h.manager.mu.RLock()
		zones := make([]Zone, 0, len(h.manager.zones))
		for _, zone := range h.manager.zones {
			zones = append(zones, *zone)
		}
		h.manager.mu.RUnlock()
		jsonResponse(w, http.StatusOK, zones)

	case http.MethodPost:
		var zone Zone
		if err := json.NewDecoder(r.Body).Decode(&zone); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.CreateZone(&zone)
		if err != nil {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, result)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleZone 处理单个区域
func (h *Handler) handleZone(w http.ResponseWriter, r *http.Request) {
	// 提取区域 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/homesec/zones/")
	id := strings.Split(path, "/")[0]

	if id == "" {
		errorResponse(w, http.StatusBadRequest, "缺少区域 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		zone, exists := h.manager.zones[id]
		h.manager.mu.RUnlock()

		if !exists {
			errorResponse(w, http.StatusNotFound, fmt.Sprintf("区域 %s 不存在", id))
			return
		}
		jsonResponse(w, http.StatusOK, zone)

	case http.MethodPut:
		var zone Zone
		if err := json.NewDecoder(r.Body).Decode(&zone); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.UpdateZone(id, &zone)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, result)

	case http.MethodDelete:
		if err := h.manager.DeleteZone(id); err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "区域已删除"})

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleEvents 处理事件列表
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 解析查询参数
	query := r.URL.Query()
	zoneID := query.Get("zone_id")

	var from, to time.Time
	var err error

	if fromStr := query.Get("from"); fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的 from 参数")
			return
		}
	}

	if toStr := query.Get("to"); toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的 to 参数")
			return
		}
	}

	limit := 100 // 默认限制
	if limitStr := query.Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	events, err := h.manager.GetEvents(zoneID, from, to, limit)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, events)
}

// handleRules 处理报警规则列表
func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		rules := make([]AlarmRule, 0, len(h.manager.rules))
		for _, rule := range h.manager.rules {
			rules = append(rules, *rule)
		}
		h.manager.mu.RUnlock()
		jsonResponse(w, http.StatusOK, rules)

	case http.MethodPost:
		var rule AlarmRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.CreateAlarmRule(&rule)
		if err != nil {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, result)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleRule 处理单个报警规则
func (h *Handler) handleRule(w http.ResponseWriter, r *http.Request) {
	// 提取规则 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/homesec/rules/")
	id := strings.Split(path, "/")[0]

	if id == "" {
		errorResponse(w, http.StatusBadRequest, "缺少规则 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		rule, exists := h.manager.rules[id]
		h.manager.mu.RUnlock()

		if !exists {
			errorResponse(w, http.StatusNotFound, fmt.Sprintf("规则 %s 不存在", id))
			return
		}
		jsonResponse(w, http.StatusOK, rule)

	case http.MethodPut:
		var rule AlarmRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.UpdateAlarmRule(id, &rule)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, result)

	case http.MethodDelete:
		if err := h.manager.DeleteAlarmRule(id); err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "规则已删除"})

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleSchedules 处理布防计划列表
func (h *Handler) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		schedules := make([]Schedule, 0, len(h.manager.schedules))
		for _, schedule := range h.manager.schedules {
			schedules = append(schedules, *schedule)
		}
		h.manager.mu.RUnlock()
		jsonResponse(w, http.StatusOK, schedules)

	case http.MethodPost:
		var schedule Schedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.CreateSchedule(&schedule)
		if err != nil {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, result)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleSchedule 处理单个布防计划
func (h *Handler) handleSchedule(w http.ResponseWriter, r *http.Request) {
	// 提取计划 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/homesec/schedules/")
	id := strings.Split(path, "/")[0]

	if id == "" {
		errorResponse(w, http.StatusBadRequest, "缺少计划 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		schedule, exists := h.manager.schedules[id]
		h.manager.mu.RUnlock()

		if !exists {
			errorResponse(w, http.StatusNotFound, fmt.Sprintf("计划 %s 不存在", id))
			return
		}
		jsonResponse(w, http.StatusOK, schedule)

	case http.MethodPut:
		var schedule Schedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		result, err := h.manager.UpdateSchedule(id, &schedule)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, result)

	case http.MethodDelete:
		if err := h.manager.DeleteSchedule(id); err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "计划已删除"})

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handlePanel 处理面板状态
func (h *Handler) handlePanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	panel, err := h.manager.GetPanelStatus()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, panel)
}

// handleArm 处理布防请求
func (h *Handler) handleArm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req struct {
		ZoneID string  `json:"zone_id"`
		Mode   ArmMode `json:"mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.ZoneID == "" {
		// 全部布防
		if err := h.manager.ArmAll(req.Mode); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "全部区域已布防"})
	} else {
		// 单个区域布防
		if err := h.manager.ArmZone(req.ZoneID, req.Mode); err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "区域已布防"})
	}
}

// handleDisarm 处理撤防请求
func (h *Handler) handleDisarm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req struct {
		ZoneID string `json:"zone_id"`
		Code   string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.ZoneID == "" {
		// 全部撤防
		if err := h.manager.DisarmAll(req.Code); err != nil {
			errorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "全部区域已撤防"})
	} else {
		// 单个区域撤防
		if err := h.manager.DisarmZone(req.ZoneID, req.Code); err != nil {
			errorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "区域已撤防"})
	}
}

// handleScore 处理安防评分请求
func (h *Handler) handleScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	score, details, err := h.manager.GetSecurityScore()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, SecurityScore{
		Score:   score,
		Details: details,
	})
}
