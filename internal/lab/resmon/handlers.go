package resmon

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler 资源监控 HTTP 处理器.
type Handler struct {
	mgr *Manager
}

// NewHandler 创建处理器.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/resmon/latest", h.handleLatest)
	mux.HandleFunc("/api/resmon/history", h.handleHistory)
	mux.HandleFunc("/api/resmon/alerts", h.handleAlerts)
	mux.HandleFunc("/api/resmon/alerts/ack/", h.handleAckAlert)
	mux.HandleFunc("/api/resmon/config", h.handleConfig)
}

func (h *Handler) handleLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	latest := h.mgr.GetLatest()
	if latest == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "no data"})
		return
	}
	writeJSON(w, http.StatusOK, latest)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	hours := 1
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			hours = v
		}
	}
	history := h.mgr.GetHistory(hours)
	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	unacked := r.URL.Query().Get("unacked") == "true"
	alerts := h.mgr.GetAlerts(unacked)
	writeJSON(w, http.StatusOK, alerts)
}

func (h *Handler) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := r.URL.Path[len("/api/resmon/alerts/ack/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing alert id"})
		return
	}
	if h.mgr.AckAlert(id) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "acked"})
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert not found"})
	}
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.GetConfig())
	case http.MethodPut:
		var cfg MonitorConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.mgr.UpdateConfig(&cfg)
		writeJSON(w, http.StatusOK, cfg)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
