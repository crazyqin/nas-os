package wanoptimize

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
	mux.HandleFunc("/api/v1/wanoptimize/tunnels", h.handleTunnels)
	mux.HandleFunc("/api/v1/wanoptimize/tunnels/", h.handleTunnelByID)
	mux.HandleFunc("/api/v1/wanoptimize/stats", h.handleStats)
}

func (h *Handler) handleTunnels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tunnels := h.manager.ListTunnels()
		writeJSON(w, http.StatusOK, map[string]any{"tunnels": tunnels})
	case http.MethodPost:
		var req CreateTunnelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
			return
		}
		tunnel, err := h.manager.CreateTunnel(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, tunnel)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleTunnelByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/wanoptimize/tunnels/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少隧道ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		tunnel, err := h.manager.GetTunnel(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, tunnel)
	case http.MethodDelete:
		if err := h.manager.DeleteTunnel(id); err != nil {
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
