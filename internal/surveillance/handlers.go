package surveillance

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 摄像头管理
	mux.HandleFunc("/api/surveillance/cameras", h.handleCameras)
	mux.HandleFunc("/api/surveillance/cameras/", h.handleCameraByID)

	// 实时流
	mux.HandleFunc("/api/surveillance/streams", h.handleStreams)
	mux.HandleFunc("/api/surveillance/streams/", h.handleStreamByID)

	// 录像
	mux.HandleFunc("/api/surveillance/recordings", h.handleRecordings)
	mux.HandleFunc("/api/surveillance/recordings/", h.handleRecordingByID)

	// 录像计划
	mux.HandleFunc("/api/surveillance/schedules", h.handleSchedules)

	// 移动侦测
	mux.HandleFunc("/api/surveillance/motion", h.handleMotion)

	// 事件
	mux.HandleFunc("/api/surveillance/events", h.handleEvents)
	mux.HandleFunc("/api/surveillance/events/", h.handleEventByID)

	// 联动规则
	mux.HandleFunc("/api/surveillance/actions", h.handleActions)

	// 分组
	mux.HandleFunc("/api/surveillance/groups", h.handleGroups)
	mux.HandleFunc("/api/surveillance/groups/", h.handleGroupByID)

	// 存储
	mux.HandleFunc("/api/surveillance/storage", h.handleStorage)

	// 状态
	mux.HandleFunc("/api/surveillance/status", h.handleStatus)
}

// ========== 通用响应 ==========

// APIResponse API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// respondJSON 返回 JSON 响应
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondSuccess 成功响应
func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// respondError 错误响应
func respondError(w http.ResponseWriter, status int, err string) {
	respondJSON(w, status, APIResponse{
		Success: false,
		Error:   err,
	})
}

// ========== 摄像头管理 ==========

// handleCameras 处理 /api/surveillance/cameras
func (h *Handler) handleCameras(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出所有摄像头
		cameras := h.manager.ListCameras()
		respondSuccess(w, cameras)

	case http.MethodPost:
		// 添加摄像头
		var cam Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := h.manager.AddCamera(&cam); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    cam,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleCameraByID 处理 /api/surveillance/cameras/{id}
func (h *Handler) handleCameraByID(w http.ResponseWriter, r *http.Request) {
	// 提取 ID
	id := r.URL.Path[len("/api/surveillance/cameras/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "camera ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取摄像头
		cam, err := h.manager.GetCamera(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, cam)

	case http.MethodPut:
		// 更新摄像头
		var cam Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		cam.ID = id

		if err := h.manager.UpdateCamera(&cam); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, cam)

	case http.MethodDelete:
		// 删除摄像头
		if err := h.manager.RemoveCamera(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"deleted": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 实时流管理 ==========

// handleStreams 处理 /api/surveillance/streams
func (h *Handler) handleStreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	streams := h.manager.ListStreams()
	respondSuccess(w, streams)
}

// handleStreamByID 处理 /api/surveillance/streams/{cameraId}
func (h *Handler) handleStreamByID(w http.ResponseWriter, r *http.Request) {
	cameraID := r.URL.Path[len("/api/surveillance/streams/"):]
	if cameraID == "" {
		respondError(w, http.StatusBadRequest, "camera ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取流信息
		stream, err := h.manager.GetStream(cameraID)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, stream)

	case http.MethodPost:
		// 开始流
		stream, err := h.manager.StartStream(cameraID)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    stream,
		})

	case http.MethodDelete:
		// 停止流
		h.manager.StopStream(cameraID)
		respondSuccess(w, map[string]string{"stopped": cameraID})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 录像管理 ==========

// handleRecordings 处理 /api/surveillance/recordings
func (h *Handler) handleRecordings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出录像
		cameraID := r.URL.Query().Get("cameraId")
		recordings := h.manager.ListRecordings(cameraID)
		respondSuccess(w, recordings)

	case http.MethodPost:
		// 开始录像
		var req struct {
			CameraID string        `json:"cameraId"`
			Mode     RecordingMode `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		recording, err := h.manager.StartRecording(req.CameraID, req.Mode)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    recording,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleRecordingByID 处理 /api/surveillance/recordings/{id}
func (h *Handler) handleRecordingByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/surveillance/recordings/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取录像
		recording, err := h.manager.GetRecording(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, recording)

	case http.MethodDelete:
		// 停止录像
		if err := h.manager.StopRecording(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"stopped": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 录像计划 ==========

// handleSchedules 处理 /api/surveillance/schedules
func (h *Handler) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出计划
		cameraID := r.URL.Query().Get("cameraId")
		schedules := h.manager.ListSchedules(cameraID)
		respondSuccess(w, schedules)

	case http.MethodPost:
		// 添加计划
		var schedule RecordingSchedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := h.manager.AddSchedule(&schedule); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    schedule,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 移动侦测 ==========

// handleMotion 处理 /api/surveillance/motion
func (h *Handler) handleMotion(w http.ResponseWriter, r *http.Request) {
	cameraID := r.URL.Query().Get("cameraId")
	if cameraID == "" {
		respondError(w, http.StatusBadRequest, "cameraId required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取配置
		config, err := h.manager.GetMotionDetection(cameraID)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, config)

	case http.MethodPut:
		// 更新配置
		var config MotionDetection
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		config.CameraID = cameraID

		if err := h.manager.SetMotionDetection(&config); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondSuccess(w, config)

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 事件管理 ==========

// handleEvents 处理 /api/surveillance/events
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cameraID := r.URL.Query().Get("cameraId")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	events := h.manager.GetEvents(cameraID, limit)
	respondSuccess(w, events)
}

// handleEventByID 处理 /api/surveillance/events/{id}
func (h *Handler) handleEventByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/surveillance/events/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "event ID required")
		return
	}

	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 确认事件
	var req struct {
		AckedBy string `json:"ackedBy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.AckEvent(id, req.AckedBy); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondSuccess(w, map[string]string{"acked": id})
}

// ========== 联动规则 ==========

// handleActions 处理 /api/surveillance/actions
func (h *Handler) handleActions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出规则
		cameraID := r.URL.Query().Get("cameraId")
		rules := h.manager.ListActionRules(cameraID)
		respondSuccess(w, rules)

	case http.MethodPost:
		// 添加规则
		var rule ActionRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := h.manager.AddActionRule(&rule); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    rule,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 分组管理 ==========

// handleGroups 处理 /api/surveillance/groups
func (h *Handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出分组
		groups := h.manager.ListGroups()
		respondSuccess(w, groups)

	case http.MethodPost:
		// 创建分组
		var group CameraGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := h.manager.CreateGroup(&group); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    group,
		})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGroupByID 处理 /api/surveillance/groups/{id}
func (h *Handler) handleGroupByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/surveillance/groups/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "group ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取分组
		group, err := h.manager.GetGroup(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, group)

	case http.MethodPut:
		// 更新分组
		var group CameraGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		group.ID = id

		if err := h.manager.UpdateGroup(&group); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, group)

	case http.MethodDelete:
		// 删除分组
		if err := h.manager.DeleteGroup(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"deleted": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 存储管理 ==========

// handleStorage 处理 /api/surveillance/storage
func (h *Handler) handleStorage(w http.ResponseWriter, r *http.Request) {
	cameraID := r.URL.Query().Get("cameraId")
	if cameraID == "" {
		respondError(w, http.StatusBadRequest, "cameraId required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取配额
		quota, err := h.manager.GetStorageQuota(cameraID)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, quota)

	case http.MethodPut:
		// 设置配额
		var quota StorageQuota
		if err := json.NewDecoder(r.Body).Decode(&quota); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		quota.CameraID = cameraID

		if err := h.manager.SetStorageQuota(&quota); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondSuccess(w, quota)

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 系统状态 ==========

// handleStatus 处理 /api/surveillance/status
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := h.manager.GetStatus()
	respondSuccess(w, status)
}

// ========== 中间件 ==========

// LoggingMiddleware 日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[surveillance] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("[surveillance] %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// CORSMiddleware CORS 中间件
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
