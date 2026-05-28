package filewatcher

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建HTTP处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/filewatcher/watchers", h.handleWatchers)
	mux.HandleFunc("/api/v1/filewatcher/watchers/", h.handleWatcherByID)
	mux.HandleFunc("/api/v1/filewatcher/stats", h.handleStats)
}

func (h *Handler) handleWatchers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		watchers := h.manager.ListWatchers()
		writeJSON(w, http.StatusOK, map[string]any{"watchers": watchers})
	case http.MethodPost:
		var req CreateWatcherRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
			return
		}
		watcher, err := h.manager.CreateWatcher(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, watcher)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleWatcherByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/filewatcher/watchers/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少监控器ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		watcher, err := h.manager.GetWatcher(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, watcher)
	case http.MethodDelete:
		if err := h.manager.DeleteWatcher(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
