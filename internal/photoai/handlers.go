// Package photoai HTTP API handlers
package photoai

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 照片管理
	mux.HandleFunc("/api/v1/photo/photos", h.handlePhotos)
	mux.HandleFunc("/api/v1/photo/photos/", h.handlePhotoByID)

	// 搜索
	mux.HandleFunc("/api/v1/photo/search", h.handleSearch)

	// 扫描 & 导入
	mux.HandleFunc("/api/v1/photo/scan", h.handleScan)
	mux.HandleFunc("/api/v1/photo/import", h.handleImport)

	// AI 分析
	mux.HandleFunc("/api/v1/photo/analyze/", h.handleAnalyze)
	mux.HandleFunc("/api/v1/photo/analyze-all", h.handleAnalyzeAll)

	// 人物
	mux.HandleFunc("/api/v1/photo/persons", h.handlePersons)
	mux.HandleFunc("/api/v1/photo/persons/", h.handlePersonByID)

	// 相册
	mux.HandleFunc("/api/v1/photo/albums", h.handleAlbums)
	mux.HandleFunc("/api/v1/photo/albums/", h.handleAlbumByID)
	mux.HandleFunc("/api/v1/photo/albums-refresh", h.handleRefreshAlbums)

	// 去重
	mux.HandleFunc("/api/v1/photo/duplicates", h.handleDuplicates)

	// 分享
	mux.HandleFunc("/api/v1/photo/share", h.handleShare)
	mux.HandleFunc("/api/v1/photo/share/", h.handleShareByToken)

	// 统计
	mux.HandleFunc("/api/v1/photo/stats", h.handleStats)
	mux.HandleFunc("/api/v1/photo/categories", h.handleCategories)
	mux.HandleFunc("/api/v1/photo/timeline", h.handleTimeline)
}

// ========== 通用响应 ==========

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, apiResponse{Success: true, Data: data})
}

func respondCreated(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusCreated, apiResponse{Success: true, Data: data})
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, apiResponse{Success: false, Error: msg})
}

// ========== 照片管理 ==========

func (h *Handler) handlePhotos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 20
		}
		photos, total := h.manager.ListPhotos(page, pageSize)
		respondSuccess(w, map[string]interface{}{
			"photos":    photos,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})

	case http.MethodPost:
		var photo Photo
		if err := json.NewDecoder(r.Body).Decode(&photo); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.manager.AddPhoto(&photo); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondCreated(w, photo)

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handlePhotoByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/photo/photos/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "photo ID required")
		return
	}

	// 处理子路径
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		photoID := parts[0]
		action := parts[1]
		h.handlePhotoAction(w, r, photoID, action)
		return
	}

	switch r.Method {
	case http.MethodGet:
		photo, err := h.manager.GetPhoto(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, photo)

	case http.MethodPut:
		var photo Photo
		if err := json.NewDecoder(r.Body).Decode(&photo); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		photo.ID = id
		if err := h.manager.UpdatePhoto(&photo); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, photo)

	case http.MethodDelete:
		if err := h.manager.DeletePhoto(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"deleted": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handlePhotoAction(w http.ResponseWriter, r *http.Request, photoID, action string) {
	switch action {
	case "favorite":
		if r.Method != http.MethodPut {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req FavoriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.manager.SetFavorite(photoID, req.IsFavorite); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"updated": photoID})

	case "analyze":
		if r.Method != http.MethodPost {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		result, err := h.manager.AnalyzePhoto(photoID)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, result)

	case "tags":
		if r.Method != http.MethodPut {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req BatchTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.PhotoIDs = []string{photoID}
		count, err := h.manager.BatchTag(&req)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondSuccess(w, map[string]int{"updated": count})

	default:
		respondError(w, http.StatusNotFound, "unknown action")
	}
}

// ========== 搜索 ==========

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var query SearchQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := h.manager.SearchPhotos(&query)
	respondSuccess(w, result)
}

// ========== 扫描 & 导入 ==========

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.manager.Scan(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondSuccess(w, result)
}

func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.manager.ImportPhotos(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondSuccess(w, result)
}

// ========== AI 分析 ==========

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	photoID := extractID(r.URL.Path, "/api/v1/photo/analyze/")
	if photoID == "" {
		respondError(w, http.StatusBadRequest, "photo ID required")
		return
	}

	result, err := h.manager.AnalyzePhoto(photoID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondSuccess(w, result)
}

func (h *Handler) handleAnalyzeAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	success, failed, err := h.manager.AnalyzePending()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondSuccess(w, map[string]int{"success": success, "failed": failed})
}

// ========== 人物 ==========

func (h *Handler) handlePersons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	persons := h.manager.GetPersons()
	respondSuccess(w, persons)
}

func (h *Handler) handlePersonByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/photo/persons/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "person ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		person, err := h.manager.GetPerson(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, person)

	case http.MethodPut:
		var req RenamePersonRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.manager.RenamePerson(id, req.Name); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"renamed": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ========== 相册 ==========

func (h *Handler) handleAlbums(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		albums := h.manager.ListAlbums()
		respondSuccess(w, albums)

	case http.MethodPost:
		var req AlbumRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		album, err := h.manager.CreateAlbum(&req)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondCreated(w, album)

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleAlbumByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/photo/albums/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "album ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		album, err := h.manager.GetAlbum(id)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, album)

	case http.MethodPut:
		var req AlbumRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		album, err := h.manager.UpdateAlbum(id, &req)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, album)

	case http.MethodDelete:
		if err := h.manager.DeleteAlbum(id); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondSuccess(w, map[string]string{"deleted": id})

	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleRefreshAlbums(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	count := h.manager.RefreshAlbums()
	respondSuccess(w, map[string]int{"refreshed": count})
}

// ========== 去重 ==========

func (h *Handler) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	groups := h.manager.DetectDuplicates()
	respondSuccess(w, groups)
}

// ========== 分享 ==========

func (h *Handler) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	link, err := h.manager.CreateShareLink(&req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondCreated(w, link)
}

func (h *Handler) handleShareByToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := extractID(r.URL.Path, "/api/v1/photo/share/")
	if token == "" {
		respondError(w, http.StatusBadRequest, "share token required")
		return
	}

	link, err := h.manager.GetShareLink(token)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	// 获取分享的照片
	var photos []*Photo
	for _, pid := range link.PhotoIDs {
		if p, err := h.manager.GetPhoto(pid); err == nil {
			photos = append(photos, p)
		}
	}

	link.ViewCount++
	respondSuccess(w, map[string]interface{}{
		"link":   link,
		"photos": photos,
	})
}

// ========== 统计 ==========

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	stats := h.manager.GetStats()
	respondSuccess(w, stats)
}

func (h *Handler) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	categories := h.manager.GetCategoryStats()
	respondSuccess(w, categories)
}

func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	timeline := h.manager.GetTimeline()
	respondSuccess(w, timeline)
}

// ========== 工具函数 ==========

func extractID(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}
