package sharedlabels

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
	mux.HandleFunc("/api/v1/labels", h.handleLabels)
	mux.HandleFunc("/api/v1/labels/stats", h.handleStats)
	mux.HandleFunc("/api/v1/files/labels", h.handleFileLabels)
}

func (h *Handler) handleLabels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listLabels(w, r)
	case http.MethodPost:
		h.createLabel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listLabels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	labelType := LabelType(r.URL.Query().Get("type"))
	tenantID := r.URL.Query().Get("tenant_id")

	labels, err := h.manager.ListLabels(ctx, labelType, tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"labels": labels,
		"total":  len(labels),
	})
}

func (h *Handler) createLabel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var label Label
	if err := json.NewDecoder(r.Body).Decode(&label); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	created, err := h.manager.CreateLabel(ctx, label)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	tenantID := r.URL.Query().Get("tenant_id")

	stats, err := h.manager.GetStats(ctx, tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleFileLabels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getFileLabels(w, r)
	case http.MethodPost:
		h.applyLabel(w, r)
	case http.MethodDelete:
		h.removeLabel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getFileLabels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fileID := r.URL.Query().Get("file_id")
	if fileID == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}

	labels, err := h.manager.GetFileLabels(ctx, fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_id": fileID,
		"labels":  labels,
	})
}

func (h *Handler) applyLabel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		FileID    string `json:"file_id"`
		FilePath  string `json:"file_path"`
		LabelID   string `json:"label_id"`
		AppliedBy string `json:"applied_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.ApplyLabel(ctx, req.FileID, req.FilePath, req.LabelID, req.AppliedBy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) removeLabel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fileID := r.URL.Query().Get("file_id")
	labelID := r.URL.Query().Get("label_id")
	if fileID == "" || labelID == "" {
		http.Error(w, "file_id and label_id are required", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveLabel(ctx, fileID, labelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
