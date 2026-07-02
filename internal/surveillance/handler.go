// Package surveillance 提供监控中心 HTTP API 处理器
package surveillance

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers 监控中心 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandler 创建处理器（兼容 web/server.go 调用）.
var NewHandler = NewHandlers

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// 摄像头管理
	mux.HandleFunc("/api/v1/surveillance/cameras", h.handleCameras)
	mux.HandleFunc("/api/v1/surveillance/cameras/", h.handleCameraRoutes)

	// 录像管理
	mux.HandleFunc("/api/v1/surveillance/recordings", h.handleRecordings)
	mux.HandleFunc("/api/v1/surveillance/recordings/", h.handleRecordingByID)

	// 移动侦测
	mux.HandleFunc("/api/v1/surveillance/motions", h.handleMotions)

	// 告警管理
	mux.HandleFunc("/api/v1/surveillance/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/surveillance/alerts/", h.handleAlertRoutes)

	// 录像计划
	mux.HandleFunc("/api/v1/surveillance/schedules", h.handleSchedules)

	// 统计信息
	mux.HandleFunc("/api/v1/surveillance/stats", h.handleStats)
}

// ========== 通用响应 ==========

// apiResponse 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应.
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// ========== 摄像头管理 ==========

// handleCameras 处理 /api/v1/surveillance/cameras.
func (h *Handlers) handleCameras(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出所有摄像头
		cameras := h.manager.ListCameras()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    cameras,
		})

	case http.MethodPost:
		// 添加摄像头
		var cam Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if err := h.manager.AddCamera(&cam); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "camera added",
			Data:    cam,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleCameraRoutes 处理 /api/v1/surveillance/cameras/* 路由.
func (h *Handlers) handleCameraRoutes(w http.ResponseWriter, r *http.Request) {
	// 解析路径: /api/v1/surveillance/cameras/{id}[/stream|/snapshot]
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/surveillance/cameras/")
	parts := strings.SplitN(path, "/", 2)

	cameraID := parts[0]
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "camera ID required")
		return
	}

	// 检查是否有子路由
	if len(parts) > 1 {
		switch parts[1] {
		case "stream":
			h.handleCameraStream(w, r, cameraID)
			return
		case "snapshot":
			h.handleCameraSnapshot(w, r, cameraID)
			return
		}
	}

	// 摄像头 CRUD
	switch r.Method {
	case http.MethodGet:
		cam, err := h.manager.GetCamera(cameraID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    cam,
		})

	case http.MethodPut:
		var cam Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		cam.ID = cameraID

		if err := h.manager.UpdateCamera(&cam); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "camera updated",
			Data:    cam,
		})

	case http.MethodDelete:
		if err := h.manager.RemoveCamera(cameraID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "camera deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleCameraStream 处理 /api/v1/surveillance/cameras/{id}/stream.
func (h *Handlers) handleCameraStream(w http.ResponseWriter, r *http.Request, cameraID string) {
	switch r.Method {
	case http.MethodGet:
		// 获取流信息
		stream, err := h.manager.GetStream(cameraID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    stream,
		})

	case http.MethodPost:
		// 开始流
		stream, err := h.manager.StartStream(cameraID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "stream started",
			Data:    stream,
		})

	case http.MethodDelete:
		// 停止流
		h.manager.StopStream(cameraID)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "stream stopped",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleCameraSnapshot 处理 /api/v1/surveillance/cameras/{id}/snapshot.
func (h *Handlers) handleCameraSnapshot(w http.ResponseWriter, r *http.Request, cameraID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 抓取快照
	snapshot, err := h.manager.TakeSnapshot(cameraID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, apiResponse{
		Code:    0,
		Message: "snapshot taken",
		Data:    snapshot,
	})
}

// ========== 录像管理 ==========

// handleRecordings 处理 /api/v1/surveillance/recordings.
func (h *Handlers) handleRecordings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出录像
		cameraID := r.URL.Query().Get("cameraId")
		recordings := h.manager.ListRecordings(cameraID)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    recordings,
		})

	case http.MethodPost:
		// 开始录像
		var req struct {
			CameraID string        `json:"cameraId"`
			Mode     RecordingMode `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		recording, err := h.manager.StartRecording(req.CameraID, req.Mode)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "recording started",
			Data:    recording,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleRecordingByID 处理 /api/v1/surveillance/recordings/{id}.
func (h *Handlers) handleRecordingByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/surveillance/recordings/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取录像详情
		recording, err := h.manager.GetRecording(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    recording,
		})

	case http.MethodDelete:
		// 停止录像
		if err := h.manager.StopRecording(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "recording stopped",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 移动侦测 ==========

// handleMotions 处理 /api/v1/surveillance/motions.
func (h *Handlers) handleMotions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 获取移动侦测事件列表
		cameraID := r.URL.Query().Get("cameraId")
		events := h.manager.GetMotionEvents(cameraID, 100)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    events,
		})

	case http.MethodPost:
		// 模拟触发移动侦测事件
		var req struct {
			CameraID string `json:"cameraId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		event, err := h.manager.SimulateMotionEvent(req.CameraID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "motion event triggered",
			Data:    event,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 告警管理 ==========

// handleAlerts 处理 /api/v1/surveillance/alerts.
func (h *Handlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 获取告警列表
	cameraID := r.URL.Query().Get("cameraId")
	status := AlertStatus(r.URL.Query().Get("status"))
	alerts := h.manager.GetAlerts(cameraID, status, 100)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    alerts,
	})
}

// handleAlertRoutes 处理 /api/v1/surveillance/alerts/* 路由.
func (h *Handlers) handleAlertRoutes(w http.ResponseWriter, r *http.Request) {
	// 解析路径: /api/v1/surveillance/alerts/{id}/ack
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/surveillance/alerts/")
	parts := strings.SplitN(path, "/", 2)

	alertID := parts[0]
	if alertID == "" {
		writeError(w, http.StatusBadRequest, "alert ID required")
		return
	}

	// 检查是否是 ack 路由
	if len(parts) > 1 && parts[1] == "ack" {
		h.handleAlertAck(w, r, alertID)
		return
	}

	writeError(w, http.StatusNotFound, "unknown alert route")
}

// handleAlertAck 处理 /api/v1/surveillance/alerts/{id}/ack.
func (h *Handlers) handleAlertAck(w http.ResponseWriter, r *http.Request, alertID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		AckedBy string `json:"ackedBy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.AckedBy == "" {
		req.AckedBy = "admin"
	}

	if err := h.manager.AckAlert(alertID, req.AckedBy); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "alert acknowledged",
	})
}

// ========== 录像计划 ==========

// handleSchedules 处理 /api/v1/surveillance/schedules.
func (h *Handlers) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出计划
		cameraID := r.URL.Query().Get("cameraId")
		schedules := h.manager.ListSchedules(cameraID)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    schedules,
		})

	case http.MethodPost:
		// 添加计划
		var schedule RecordingSchedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if err := h.manager.AddSchedule(&schedule); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "schedule added",
			Data:    schedule,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 统计信息 ==========

// handleStats 处理 /api/v1/surveillance/stats.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
