package sshmanager

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler SSH 管理器 HTTP 处理器.
type Handler struct {
	mgr *Manager
}

// NewHandler 创建处理器.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ssh/keys", h.handleKeys)
	mux.HandleFunc("/api/ssh/keys/", h.handleKeyByID)
	mux.HandleFunc("/api/ssh/sessions", h.handleSessions)
	mux.HandleFunc("/api/ssh/sessions/", h.handleSessionByID)
	mux.HandleFunc("/api/ssh/tunnels", h.handleTunnels)
	mux.HandleFunc("/api/ssh/config", h.handleConfig)
	mux.HandleFunc("/api/ssh/stats", h.handleStats)
}

func (h *Handler) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.ListKeys())
	case http.MethodPost:
		var key SSHKey
		if err := json.NewDecoder(r.Body).Decode(&key); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.mgr.AddKey(&key)
		writeJSON(w, http.StatusCreated, key)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleKeyByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/ssh/keys/"):]
	switch r.Method {
	case http.MethodGet:
		key, ok := h.mgr.GetKey(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
			return
		}
		writeJSON(w, http.StatusOK, key)
	case http.MethodDelete:
		if !h.mgr.DeleteKey(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	activeOnly := r.URL.Query().Get("active") == "true"
	writeJSON(w, http.StatusOK, h.mgr.ListSessions(activeOnly))
}

func (h *Handler) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/ssh/sessions/"):]
	switch r.Method {
	case http.MethodGet:
		sess, ok := h.mgr.GetSession(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		writeJSON(w, http.StatusOK, sess)
	case http.MethodPost: // close
		if !h.mgr.CloseSession(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleTunnels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.ListTunnels())
	case http.MethodPost:
		var req struct {
			Name       string `json:"name"`
			SessionID  string `json:"session_id"`
			LocalAddr  string `json:"local_addr"`
			RemoteAddr string `json:"remote_addr"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		t := h.mgr.CreateTunnel(req.Name, req.SessionID, req.LocalAddr, req.RemoteAddr)
		writeJSON(w, http.StatusCreated, t)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.GetConfig())
	case http.MethodPut:
		var cfg SSHConfig
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

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, h.mgr.GetStats())
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// suppress unused import warning
var _ = strconv.Itoa
