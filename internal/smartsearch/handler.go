package smartsearch

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
	mux.HandleFunc("/api/v1/search", h.handleSearch)
	mux.HandleFunc("/api/v1/search/index", h.handleIndex)
	mux.HandleFunc("/api/v1/search/stats", h.handleStats)
	mux.HandleFunc("/api/v1/search/rebuild", h.handleRebuild)
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.manager.Search(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.indexFile(w, r)
	case http.MethodDelete:
		h.removeFromIndex(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) indexFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var entry IndexEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.IndexFile(ctx, entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "indexed"})
}

func (h *Handler) removeFromIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fileID := r.URL.Query().Get("file_id")
	if fileID == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveFromIndex(ctx, fileID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	stats, err := h.manager.GetIndexStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	if err := h.manager.RebuildIndex(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "rebuilding"})
}
