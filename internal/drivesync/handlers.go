// Package drivesync 文件同步服务 - HTTP handlers
package drivesync

import (
	"encoding/json"
	"net/http"
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
	mux.HandleFunc("/api/drive/sync", h.handleSync)
	mux.HandleFunc("/api/drive/status", h.handleStatus)
	mux.HandleFunc("/api/drive/conflicts", h.handleConflicts)
	mux.HandleFunc("/api/drive/conflicts/resolve", h.handleResolveConflict)
	mux.HandleFunc("/api/drive/stats", h.handleStats)
	mux.HandleFunc("/api/drive/policy", h.handlePolicy)
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SourcePath string `json:"source_path"`
		TargetPath string `json:"target_path"`
		IsDir      bool   `json:"is_dir"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var task *SyncTask
	var err error

	if req.IsDir {
		task, err = h.manager.SyncDirectory(r.Context(), req.SourcePath, req.TargetPath)
	} else {
		task, err = h.manager.SyncFile(r.Context(), req.SourcePath, req.TargetPath)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(task)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}

	status, err := h.manager.GetSyncStatus(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func (h *Handler) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conflicts, err := h.manager.ListConflicts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"conflicts": conflicts,
	})
}

func (h *Handler) handleResolveConflict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ConflictID string `json:"conflict_id"`
		Resolution string `json:"resolution"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.manager.ResolveConflict(r.Context(), req.ConflictID, req.Resolution); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.manager.GetStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handlePolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// TODO: 返回当前策略
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodPut:
		var policy SyncPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := h.manager.SetPolicy(r.Context(), policy); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
