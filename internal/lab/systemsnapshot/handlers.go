// Package systemsnapshot HTTP API 处理器
package systemsnapshot

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler HTTP API处理器.
type Handler struct {
	manager *SnapshotManager
}

// NewHandler 创建处理器.
func NewHandler(manager *SnapshotManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/snapshots", h.handleSnapshots)
	mux.HandleFunc(prefix+"/snapshots/", h.handleSnapshotByID)
	mux.HandleFunc(prefix+"/snapshots/create", h.handleCreateSnapshot)
	mux.HandleFunc(prefix+"/snapshots/rollback", h.handleRollback)
	mux.HandleFunc(prefix+"/snapshots/diff", h.handleDiff)
	mux.HandleFunc(prefix+"/snapshots/preview", h.handlePreview)
	mux.HandleFunc(prefix+"/snapshots/stats", h.handleStats)
	mux.HandleFunc(prefix+"/snapshots/cleanup", h.handleCleanup)
	mux.HandleFunc(prefix+"/snapshots/auto/pre-update", h.handlePreUpdate)
	mux.HandleFunc(prefix+"/snapshots/auto/pre-change", h.handlePreChange)
}

// handleSnapshots 处理快照列表和创建.
func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListSnapshots(w, r)
	case http.MethodPost:
		h.handleCreateSnapshot(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListSnapshots 列出快照.
func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	var opts ListOptions

	opts.Type = SnapshotType(r.URL.Query().Get("type"))
	opts.Status = SnapshotStatus(r.URL.Query().Get("status"))
	opts.SortBy = r.URL.Query().Get("sort_by")

	if r.URL.Query().Get("sort_desc") == "true" {
		opts.SortDesc = true
	}

	tags := r.URL.Query().Get("tags")
	if tags != "" {
		opts.Tags = strings.Split(tags, ",")
	}

	snapshots := h.manager.ListSnapshots(opts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshots)
}

// handleCreateSnapshot 创建快照.
func (h *Handler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Type        SnapshotType     `json:"type"`
		Categories  []ConfigCategory `json:"categories"`
		Tags        []string         `json:"tags"`
		Async       bool             `json:"async"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		req.Type = SnapshotTypeManual
	}

	var snapshot *Snapshot
	var err error

	if req.Async {
		snapshot, err = h.manager.CreateSnapshotAsync(r.Context(), req.Name, req.Description, req.Type, req.Categories, req.Tags)
	} else {
		snapshot, err = h.manager.CreateSnapshot(r.Context(), req.Name, req.Description, req.Type, req.Categories, req.Tags)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(snapshot)
}

// handleSnapshotByID 处理单个快照操作.
func (h *Handler) handleSnapshotByID(w http.ResponseWriter, r *http.Request) {
	// 从路径中提取ID
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/systemsnapshot/snapshots/")
	if path == "" {
		http.Error(w, "missing snapshot id", http.StatusBadRequest)
		return
	}

	// 处理子路径
	parts := strings.SplitN(path, "/", 2)
	snapshotID := parts[0]

	switch r.Method {
	case http.MethodGet:
		snapshot, err := h.manager.GetSnapshot(snapshotID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)

	case http.MethodDelete:
		if err := h.manager.DeleteSnapshot(snapshotID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRollback 处理回滚请求.
func (h *Handler) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.manager.RollbackToSnapshot(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleDiff 处理差异对比请求.
func (h *Handler) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshotA := r.URL.Query().Get("snapshot_a")
	snapshotB := r.URL.Query().Get("snapshot_b")

	if snapshotA == "" || snapshotB == "" {
		http.Error(w, "missing snapshot_a or snapshot_b parameter", http.StatusBadRequest)
		return
	}

	diff, err := h.manager.CompareSnapshots(snapshotA, snapshotB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

// handlePreview 处理预览请求.
func (h *Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshotID := r.URL.Query().Get("id")
	if snapshotID == "" {
		http.Error(w, "missing id parameter", http.StatusBadRequest)
		return
	}

	result, err := h.manager.PreviewSnapshot(snapshotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleStats 处理统计信息请求.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleCleanup 处理清理请求.
func (h *Handler) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deleted, err := h.manager.CleanupSnapshots()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"status":  "ok",
	})
}

// handlePreUpdate 处理更新前自动快照.
func (h *Handler) handlePreUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot, err := h.manager.CreatePreUpdateSnapshot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(snapshot)
}

// handlePreChange 处理配置变更前自动快照.
func (h *Handler) handlePreChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Category ConfigCategory `json:"category"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Category == "" {
		req.Category = CategorySystem
	}

	snapshot, err := h.manager.CreatePreChangeSnapshot(r.Context(), req.Category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(snapshot)
}
