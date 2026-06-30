// Package smbhafailover 提供 SMB 有状态高可用故障转移功能。
package smbhafailover

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器
type Handler struct {
	manager *FailoverManager
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *FailoverManager) *Handler {
	return &Handler{manager: manager}
}

// SnapshotRequest 创建快照请求体
type SnapshotRequest struct {
	SourceNode string `json:"source_node,omitempty"`
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/smbha/snapshot", h.Snapshot)
	mux.HandleFunc("/api/smbha/sessions", h.Sessions)
	mux.HandleFunc("/api/smbha/restore", h.Restore)
	mux.HandleFunc("/api/smbha/state", h.State)
}

// Snapshot 处理 POST /api/smbha/snapshot
func (h *Handler) Snapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}

	var req SnapshotRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
			return
		}
	}

	snap, err := h.manager.CreateSnapshot(req.SourceNode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// Sessions 处理 GET /api/smbha/sessions
func (h *Handler) Sessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}
	sessions := h.manager.ListSessions()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// Restore 处理 POST /api/smbha/restore
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}

	var req struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	if req.SnapshotID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("snapshot_id is required"))
		return
	}

	restored, err := h.manager.RestoreSnapshot(req.SnapshotID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"restored":  true,
		"sessions":  restored,
		"count":     len(restored),
	})
}

// State 处理 GET /api/smbha/state
func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetState())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}