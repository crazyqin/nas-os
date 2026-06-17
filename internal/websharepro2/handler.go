package websharepro2

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP API 处理器。
type Handler struct {
	engine *Engine
}

// NewHandler 创建新的 HTTP 处理器。
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册 HTTP 路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/websharepro2/shares", h.handleShares)
	mux.HandleFunc("/api/websharepro2/share", h.handleShare)
	mux.HandleFunc("/api/websharepro2/access", h.handleAccess)
	mux.HandleFunc("/api/websharepro2/collab", h.handleCollab)
	mux.HandleFunc("/api/websharepro2/stats", h.handleStats)
}

func (h *Handler) handleShares(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.engine.ListShares())
	case http.MethodPost:
		var share Share
		if err := json.NewDecoder(r.Body).Decode(&share); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.engine.CreateShare(&share)
		writeJSON(w, http.StatusCreated, share)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleShare(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		share, exists := h.engine.GetShare(id)
		if !exists {
			writeError(w, http.StatusNotFound, "share not found")
			return
		}
		writeJSON(w, http.StatusOK, share)
	case http.MethodPut:
		var updates Share
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.engine.UpdateShare(id, &updates); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	case http.MethodDelete:
		if err := h.engine.DeleteShare(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var access ShareAccess
	if err := json.NewDecoder(r.Body).Decode(&access); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.engine.RecordAccess(access)
	writeJSON(w, http.StatusCreated, access)
}

func (h *Handler) handleCollab(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ShareID string   `json:"share_id"`
		Users   []string `json:"users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	collab := h.engine.StartCollaboration(req.ShareID, req.Users)
	writeJSON(w, http.StatusCreated, collab)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.engine.GetStats())
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
