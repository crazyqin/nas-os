package smartdownloader

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP API处理器.
type Handler struct {
	manager *DownloadManager
}

// NewHandler 创建处理器.
func NewHandler(manager *DownloadManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/download", h.handleDownload)
	mux.HandleFunc(prefix+"/download/status", h.handleDownloadStatus)
	mux.HandleFunc(prefix+"/download/pause", h.handlePause)
	mux.HandleFunc(prefix+"/download/resume", h.handleResume)
	mux.HandleFunc(prefix+"/download/cancel", h.handleCancel)
	mux.HandleFunc(prefix+"/download/delete", h.handleDelete)
	mux.HandleFunc(prefix+"/downloads", h.handleListDownloads)
	mux.HandleFunc(prefix+"/queue", h.handleQueue)
	mux.HandleFunc(prefix+"/history", h.handleHistory)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
	mux.HandleFunc(prefix+"/speed-limit", h.handleSpeedLimit)
	mux.HandleFunc(prefix+"/notify", h.handleNotifyConfig)
}

// handleDownload 处理下载请求.
func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	item, err := h.manager.AddDownload(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

// handleDownloadStatus 处理下载状态查询.
func (h *Handler) handleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing download id", http.StatusBadRequest)
		return
	}

	item, ok := h.manager.GetDownload(id)
	if !ok {
		http.Error(w, "download not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// handlePause 处理暂停请求.
func (h *Handler) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.PauseDownload(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

// handleResume 处理恢复请求.
func (h *Handler) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.ResumeDownload(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

// handleCancel 处理取消请求.
func (h *Handler) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.CancelDownload(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

// handleDelete 处理删除请求.
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.DeleteDownload(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// handleListDownloads 处理下载列表.
func (h *Handler) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	downloads := h.manager.ListDownloads()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(downloads)
}

// handleQueue 处理队列查询.
func (h *Handler) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queue := h.manager.GetQueue()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(queue)
}

// handleHistory 处理历史查询.
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := h.manager.GetHistory()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// handleStats 处理统计查询.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleSpeedLimit 处理限速配置.
func (h *Handler) handleSpeedLimit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"global_download": h.manager.speedLimit.GlobalDownload,
			"global_upload":   h.manager.speedLimit.GlobalUpload,
			"per_task":        h.manager.speedLimit.PerTask,
			"schedule_start":  h.manager.speedLimit.ScheduleStart,
			"schedule_end":    h.manager.speedLimit.ScheduleEnd,
			"schedule_limit":  h.manager.speedLimit.ScheduleLimit,
		})

	case http.MethodPut, http.MethodPost:
		var config SpeedLimitConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		h.manager.UpdateSpeedLimit(config)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNotifyConfig 处理通知配置.
func (h *Handler) handleNotifyConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.manager.notifyConfig)

	case http.MethodPut, http.MethodPost:
		var config NotifyConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		h.manager.UpdateNotifyConfig(config)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
