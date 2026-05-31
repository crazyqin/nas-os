package applifecycle

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/apps", h.handleApps)
	mux.HandleFunc("/api/v1/apps/", h.handleAppByID)
	mux.HandleFunc("/api/v1/apps/config/", h.handleConfig)
}

func (h *Handler) handleApps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		apps := h.manager.ListApps("")
		writeJSON(w, http.StatusOK, apps)
	case http.MethodPost:
		var req struct {
			Name    string            `json:"name"`
			Image   string            `json:"image"`
			Version string            `json:"version"`
			Ports   map[string]string `json:"ports"`
			Volumes []string          `json:"volumes"`
			Env     map[string]string `json:"env"`
			Labels  map[string]string `json:"labels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		app, err := h.manager.Install(req.Name, req.Image, req.Version, InstallOptions{
			Ports:   req.Ports,
			Volumes: req.Volumes,
			Env:     req.Env,
			Labels:  req.Labels,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, app)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleAppByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/apps/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing app id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		app, err := h.manager.GetApp(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, app)
	case http.MethodDelete:
		keepData := r.URL.Query().Get("keep_data") == "true"
		if err := h.manager.Uninstall(id, keepData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/apps/config/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing app id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		history, err := h.manager.GetConfigHistory(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, history)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
