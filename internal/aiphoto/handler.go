// Package aiphoto AI相册模块 - HTTP 处理器
package aiphoto

import (
	"encoding/json"
	"net/http"
)

// HTTPHandler HTTP 处理器
type HTTPHandler struct {
	manager *AIPhotoManager
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(manager *AIPhotoManager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/aiphoto/photos", h.handlePhotos)
	mux.HandleFunc("/api/aiphoto/photos/add", h.handleAddPhoto)
	mux.HandleFunc("/api/aiphoto/photos/search", h.handleSearchPhotos)
	mux.HandleFunc("/api/aiphoto/photos/favorite", h.handleToggleFavorite)
	mux.HandleFunc("/api/aiphoto/albums", h.handleAlbums)
	mux.HandleFunc("/api/aiphoto/albums/create", h.handleCreateAlbum)
	mux.HandleFunc("/api/aiphoto/stats", h.handleStats)
}

func (h *HTTPHandler) handlePhotos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取所有照片
	photos := make([]*AlbumPhotoMetadata, 0)
	for _, p := range h.manager.photos {
		photos = append(photos, p)
	}
	json.NewEncoder(w).Encode(photos)
}

func (h *HTTPHandler) handleAddPhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath string `json:"file_path"`
		FileName string `json:"file_name"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	photo, err := h.manager.AddPhoto(req.FilePath, req.FileName, req.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(photo)
}

func (h *HTTPHandler) handleSearchPhotos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule SmartRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	photos := h.manager.SearchPhotos(&rule)
	json.NewEncoder(w).Encode(photos)
}

func (h *HTTPHandler) handleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PhotoID string `json:"photo_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.manager.ToggleFavorite(req.PhotoID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *HTTPHandler) handleAlbums(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.ListAlbums())
}

func (h *HTTPHandler) handleCreateAlbum(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		IsSmart     bool       `json:"is_smart"`
		Rule        *SmartRule `json:"rule,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	album, err := h.manager.CreateAlbum(req.Name, req.Description, req.IsSmart, req.Rule)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(album)
}

func (h *HTTPHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.GetStats())
}
