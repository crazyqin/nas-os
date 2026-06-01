package smbdirect

import (
	"encoding/json"
	"net/http"
)

// Handler SMB Direct HTTP 处理器
type Handler struct {
	mgr *SMBDirectManager
}

// NewHandler 创建处理器
func NewHandler(mgr *SMBDirectManager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smbdirect/status", h.handleStatus)
	mux.HandleFunc("/api/v1/smbdirect/connections", h.handleConnections)
	mux.HandleFunc("/api/v1/smbdirect/connect", h.handleConnect)
	mux.HandleFunc("/api/v1/smbdirect/config", h.handleConfig)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.mgr.GetStatus())
}

func (h *Handler) handleConnections(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		LocalAddr  string `json:"local_addr"`
		RemoteAddr string `json:"remote_addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	conn, err := h.mgr.CreateConnection(req.LocalAddr, req.RemoteAddr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conn)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, DefaultConfig())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
