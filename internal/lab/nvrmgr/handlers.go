// Package nvrmgr 提供 NVR 视频管理 HTTP API
package nvrmgr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 摄像头管理
	mux.HandleFunc("/api/nvrmgr/cameras", h.handleCameras)
	mux.HandleFunc("/api/nvrmgr/cameras/", h.handleCameraByID)

	// 录像管理
	mux.HandleFunc("/api/nvrmgr/recordings", h.handleRecordings)
	mux.HandleFunc("/api/nvrmgr/recordings/", h.handleRecordingByID)

	// 时间线
	mux.HandleFunc("/api/nvrmgr/timeline", h.handleTimeline)

	// 移动侦测
	mux.HandleFunc("/api/nvrmgr/motion", h.handleMotion)

	// 告警管理
	mux.HandleFunc("/api/nvrmgr/alerts", h.handleAlerts)
	mux.HandleFunc("/api/nvrmgr/alerts/", h.handleAlertByID)

	// 存储策略
	mux.HandleFunc("/api/nvrmgr/storage-plans", h.handleStoragePlans)
	mux.HandleFunc("/api/nvrmgr/storage-plans/", h.handleStoragePlanByID)

	// 统计信息
	mux.HandleFunc("/api/nvrmgr/stats", h.handleStats)
}

// ========== 通用响应 ==========

// APIResponse API 响应.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Total   int         `json:"total,omitempty"`
	Page    int         `json:"page,omitempty"`
	Size    int         `json:"size,omitempty"`
}

// respondJSON 返回 JSON 响应.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondSuccess 成功响应.
func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// respondError 错误响应.
func respondError(w http.ResponseWriter, status int, err string) {
	respondJSON(w, status, APIResponse{
		Success: false,
		Error:   err,
	})
}

// ========== 摄像头管理 ==========

// handleCameras 处理 /api/nvrmgr/cameras.
func (h *Handler) handleCameras(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cameras, err := h.manager.ListCameras()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondSuccess(w, cameras)

	case http.MethodPost:
		var cam Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			respondError(w, http.StatusBadRequest, "请求体无效")
			return
		}

		result, err := h.manager.AddCamera(&cam)
		if err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    result,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleCameraByID 处理 /api/nvrmgr/cameras/{id}.
func (h *Handler) handleCameraByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/nvrmgr/cameras/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "摄像头 ID 不能为空")
		return
	}

	switch r.Method {
	case http.MethodGet:
		cam, err := h.manager.GetCamera(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, cam)

	case http.MethodPut:
		var cam Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			respondError(w, http.StatusBadRequest, "请求体无效")
			return
		}

		result, err := h.manager.UpdateCamera(id, &cam)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, result)

	case http.MethodDelete:
		if err := h.manager.DeleteCamera(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"deleted": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// ========== 录像管理 ==========

// handleRecordings 处理 /api/nvrmgr/recordings.
func (h *Handler) handleRecordings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cameraID := r.URL.Query().Get("cameraId")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

		var from, to time.Time
		if fromStr := r.URL.Query().Get("from"); fromStr != "" {
			from, _ = time.Parse(time.RFC3339, fromStr)
		}
		if toStr := r.URL.Query().Get("to"); toStr != "" {
			to, _ = time.Parse(time.RFC3339, toStr)
		}

		recordings, total, err := h.manager.GetRecordings(cameraID, from, to, page, pageSize)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if page < 1 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 20
		}

		respondJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    recordings,
			Total:   total,
			Page:    page,
			Size:    pageSize,
		})

	case http.MethodPost:
		var req struct {
			CameraID string `json:"cameraId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "请求体无效")
			return
		}

		if err := h.manager.StartRecording(req.CameraID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "录像已开始"},
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleRecordingByID 处理 /api/nvrmgr/recordings/{id}.
func (h *Handler) handleRecordingByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/nvrmgr/recordings/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "录像 ID 不能为空")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := h.manager.DeleteRecording(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"deleted": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// ========== 时间线 ==========

// handleTimeline 处理 /api/nvrmgr/timeline.
func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	cameraID := r.URL.Query().Get("cameraId")
	if cameraID == "" {
		respondError(w, http.StatusBadRequest, "摄像头 ID 不能为空")
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "日期格式无效，应为 YYYY-MM-DD")
		return
	}

	timeline, err := h.manager.GetTimeline(cameraID, date)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondSuccess(w, timeline)
}

// ========== 移动侦测 ==========

// handleMotion 处理 /api/nvrmgr/motion.
func (h *Handler) handleMotion(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cameraID := r.URL.Query().Get("cameraId")

		var from, to time.Time
		if fromStr := r.URL.Query().Get("from"); fromStr != "" {
			from, _ = time.Parse(time.RFC3339, fromStr)
		}
		if toStr := r.URL.Query().Get("to"); toStr != "" {
			to, _ = time.Parse(time.RFC3339, toStr)
		}

		events, err := h.manager.GetMotionEvents(cameraID, from, to)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondSuccess(w, events)

	case http.MethodPost:
		var req struct {
			CameraID    string  `json:"cameraId"`
			Zone        string  `json:"zone"`
			Sensitivity float64 `json:"sensitivity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "请求体无效")
			return
		}

		if err := h.manager.AddMotionRule(req.CameraID, req.Zone, req.Sensitivity); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "移动侦测规则已添加"},
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// ========== 告警管理 ==========

// handleAlerts 处理 /api/nvrmgr/alerts.
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		unread := r.URL.Query().Get("unread") == "true"
		alerts, err := h.manager.ListAlerts(unread)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondSuccess(w, alerts)

	case http.MethodPost:
		var alert Alert
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			respondError(w, http.StatusBadRequest, "请求体无效")
			return
		}

		result, err := h.manager.CreateAlert(&alert)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    result,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleAlertByID 处理 /api/nvrmgr/alerts/{id}.
func (h *Handler) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/nvrmgr/alerts/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "告警 ID 不能为空")
		return
	}

	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	if err := h.manager.AcknowledgeAlert(id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondSuccess(w, map[string]string{"acknowledged": id})
}

// ========== 存储策略 ==========

// handleStoragePlans 处理 /api/nvrmgr/storage-plans.
func (h *Handler) handleStoragePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 列出所有存储计划
	h.manager.mu.RLock()
	plans := make([]StoragePlan, 0, len(h.manager.storagePlans))
	for _, plan := range h.manager.storagePlans {
		plans = append(plans, *plan)
	}
	h.manager.mu.RUnlock()

	respondSuccess(w, plans)
}

// handleStoragePlanByID 处理 /api/nvrmgr/storage-plans/{id}.
func (h *Handler) handleStoragePlanByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/nvrmgr/storage-plans/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "存储策略 ID 不能为空")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		plan, exists := h.manager.storagePlans[id]
		h.manager.mu.RUnlock()

		if !exists {
			respondError(w, http.StatusNotFound, fmt.Sprintf("存储策略 %s 不存在", id))
			return
		}
		respondSuccess(w, plan)

	case http.MethodPut:
		var req struct {
			CameraIDs []string `json:"cameraIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "请求体无效")
			return
		}

		if err := h.manager.ApplyStoragePlan(id, req.CameraIDs); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondSuccess(w, map[string]string{"message": "存储策略已应用"})

	default:
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// ========== 统计信息 ==========

// handleStats 处理 /api/nvrmgr/stats.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	cameraStatus, err := h.manager.GetCameraStatus()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	storageUsage, err := h.manager.GetStorageUsage()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 获取告警统计
	alerts, _ := h.manager.ListAlerts(false)
	unreadAlerts, _ := h.manager.ListAlerts(true)

	stats := map[string]interface{}{
		"cameras":         cameraStatus,
		"storage":         storageUsage,
		"totalAlerts":     len(alerts),
		"unreadAlerts":    len(unreadAlerts),
		"totalRecordings": len(h.manager.recordings),
	}

	respondSuccess(w, stats)
}
