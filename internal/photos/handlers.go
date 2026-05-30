// Package photos 智能相册管理 - HTTP handlers
package photos

import (
	"encoding/json"
	"net/http"
	"strconv"
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
	mux.HandleFunc("/api/photos/import", h.handleImport)
	mux.HandleFunc("/api/photos/search", h.handleSearch)
	mux.HandleFunc("/api/photos/timeline", h.handleTimeline)
	mux.HandleFunc("/api/photos/stats", h.handleStats)
	mux.HandleFunc("/api/photos/albums", h.handleAlbums)
	mux.HandleFunc("/api/photos/albums/create", h.handleCreateAlbum)
	mux.HandleFunc("/api/photos/albums/add", h.handleAddToAlbum)
}

func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath string `json:"file_path"`
		UserID   string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	photo, err := h.manager.ImportPhoto(r.Context(), req.FilePath, req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(photo)
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := SearchQuery{
		Keyword:  r.URL.Query().Get("keyword"),
		AlbumID:  r.URL.Query().Get("album_id"),
		Format:   r.URL.Query().Get("format"),
		SortBy:   r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
	}

	if page, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil {
		query.Page = page
	}
	if pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil {
		query.PageSize = pageSize
	}

	result, err := h.manager.SearchPhotos(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "month"
	}

	timeline, err := h.manager.GetTimeline(r.Context(), groupBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(timeline)
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

func (h *Handler) handleAlbums(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: 获取相册列表
	json.NewEncoder(w).Encode(map[string]interface{}{
		"albums": []Album{},
	})
}

func (h *Handler) handleCreateAlbum(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     string `json:"owner_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	album, err := h.manager.CreateAlbum(r.Context(), req.Name, req.Description, req.OwnerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(album)
}

func (h *Handler) handleAddToAlbum(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PhotoID string `json:"photo_id"`
		AlbumID string `json:"album_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.manager.AddPhotoToAlbum(r.Context(), req.PhotoID, req.AlbumID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
