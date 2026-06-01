// Package smartgallery 提供智能相册 HTTP API 处理器
package smartgallery

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handlers 智能相册 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// 照片管理
	mux.HandleFunc("/api/v1/smartgallery/photos", h.handlePhotos)
	mux.HandleFunc("/api/v1/smartgallery/photos/", h.handlePhotoByID)

	// 相册管理
	mux.HandleFunc("/api/v1/smartgallery/albums", h.handleAlbums)
	mux.HandleFunc("/api/v1/smartgallery/albums/", h.handleAlbumByID)

	// 人物/人脸管理
	mux.HandleFunc("/api/v1/smartgallery/faces", h.handleFaces)
	mux.HandleFunc("/api/v1/smartgallery/faces/", h.handleFaceByID)

	// 场景
	mux.HandleFunc("/api/v1/smartgallery/scenes", h.handleScenes)

	// 搜索
	mux.HandleFunc("/api/v1/smartgallery/search", h.handleSearch)

	// 时间线
	mux.HandleFunc("/api/v1/smartgallery/timeline", h.handleTimeline)

	// 标签
	mux.HandleFunc("/api/v1/smartgallery/tags", h.handleTags)

	// 导入
	mux.HandleFunc("/api/v1/smartgallery/import", h.handleImport)

	// 统计
	mux.HandleFunc("/api/v1/smartgallery/stats", h.handleStats)
}

// apiResponse 标准 API 响应
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// extractIDFromPath 从路径中提取 ID
// 例如 /api/v1/smartgallery/photos/xxx -> xxx
func extractIDFromPath(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, "/")
	// 取第一段作为 ID
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// ---------- 照片 ----------

// handlePhotos 处理照片列表 (GET /api/v1/smartgallery/photos)
func (h *Handlers) handlePhotos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	photos, total := h.manager.ListPhotos(page, pageSize)

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"photos":    photos,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// handlePhotoByID 处理单个照片操作 (GET/PUT/DELETE /api/v1/smartgallery/photos/:id)
func (h *Handlers) handlePhotoByID(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/smartgallery/photos/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "photo id is required")
		return
	}

	// 子路由: /photos/:id/tags, /photos/:id/favorite, /photos/:id/rating
	remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/smartgallery/photos/"+id)
	remaining = strings.TrimPrefix(remaining, "/")

	switch remaining {
	case "tags":
		h.handlePhotoTags(w, r, id)
		return
	case "favorite":
		h.handlePhotoFavorite(w, r, id)
		return
	case "rating":
		h.handlePhotoRating(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		photo, err := h.manager.GetPhoto(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    photo,
		})

	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		photo, err := h.manager.UpdatePhoto(id, updates)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "photo updated",
			Data:    photo,
		})

	case http.MethodDelete:
		if err := h.manager.DeletePhoto(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "photo deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePhotoTags 处理照片标签 (POST/DELETE /api/v1/smartgallery/photos/:id/tags)
func (h *Handlers) handlePhotoTags(w http.ResponseWriter, r *http.Request, photoID string) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Category string `json:"category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "tag name is required")
			return
		}
		if req.Category == "" {
			req.Category = "object"
		}
		tag, err := h.manager.AddTag(photoID, req.Name, req.Category)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "tag added",
			Data:    tag,
		})

	case http.MethodDelete:
		tagID := r.URL.Query().Get("tag_id")
		if tagID == "" {
			writeError(w, http.StatusBadRequest, "tag_id is required")
			return
		}
		if err := h.manager.RemoveTag(photoID, tagID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "tag removed",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePhotoFavorite 处理照片收藏 (POST /api/v1/smartgallery/photos/:id/favorite)
func (h *Handlers) handlePhotoFavorite(w http.ResponseWriter, r *http.Request, photoID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	photo, err := h.manager.ToggleFavorite(photoID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "favorite toggled",
		Data:    photo,
	})
}

// handlePhotoRating 处理照片评分 (POST /api/v1/smartgallery/photos/:id/rating)
func (h *Handlers) handlePhotoRating(w http.ResponseWriter, r *http.Request, photoID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Rating int `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	photo, err := h.manager.SetRating(photoID, req.Rating)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "rating set",
		Data:    photo,
	})
}

// ---------- 相册 ----------

// handleAlbums 处理相册列表和创建 (GET/POST /api/v1/smartgallery/albums)
func (h *Handlers) handleAlbums(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		albums := h.manager.ListAlbums()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    albums,
		})

	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "album name is required")
			return
		}
		if req.Type == "" {
			req.Type = "manual"
		}
		album := h.manager.CreateAlbum(req.Name, req.Description, req.Type)
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "album created",
			Data:    album,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAlbumByID 处理单个相册操作 (GET/PUT/DELETE /api/v1/smartgallery/albums/:id)
func (h *Handlers) handleAlbumByID(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/smartgallery/albums/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "album id is required")
		return
	}

	// 子路由: /albums/:id/photos
	remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/smartgallery/albums/"+id)
	remaining = strings.TrimPrefix(remaining, "/")

	if remaining == "photos" {
		h.handleAlbumPhotos(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		album, err := h.manager.GetAlbum(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    album,
		})

	case http.MethodPut:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		album, err := h.manager.UpdateAlbum(id, req.Name, req.Description)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "album updated",
			Data:    album,
		})

	case http.MethodDelete:
		if err := h.manager.DeleteAlbum(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "album deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAlbumPhotos 处理相册照片管理 (POST/DELETE /api/v1/smartgallery/albums/:id/photos)
func (h *Handlers) handleAlbumPhotos(w http.ResponseWriter, r *http.Request, albumID string) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			PhotoIDs []string `json:"photo_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if len(req.PhotoIDs) == 0 {
			writeError(w, http.StatusBadRequest, "photo_ids is required")
			return
		}
		if err := h.manager.AddPhotosToAlbum(albumID, req.PhotoIDs); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "photos added to album",
		})

	case http.MethodDelete:
		var req struct {
			PhotoIDs []string `json:"photo_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if len(req.PhotoIDs) == 0 {
			writeError(w, http.StatusBadRequest, "photo_ids is required")
			return
		}
		if err := h.manager.RemovePhotosFromAlbum(albumID, req.PhotoIDs); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "photos removed from album",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---------- 人脸/人物 ----------

// handleFaces 处理人脸列表和聚类 (GET/POST /api/v1/smartgallery/faces)
func (h *Handlers) handleFaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		persons := h.manager.ListPersons()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    persons,
		})

	case http.MethodPost:
		// 触发人脸识别聚类
		persons, err := h.manager.ClusterFaces()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "face clustering completed",
			Data:    persons,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleFaceByID 处理单个人物操作
// GET/PUT /api/v1/smartgallery/faces/:id
// POST /api/v1/smartgallery/faces/:id/person (分配人脸给人物)
func (h *Handlers) handleFaceByID(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/smartgallery/faces/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "face id is required")
		return
	}

	// 子路由: /faces/:id/person → 分配人脸给人物
	remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/smartgallery/faces/"+id)
	remaining = strings.TrimPrefix(remaining, "/")

	if remaining == "person" {
		h.handleAssignFaceToPerson(w, r, id)
		return
	}

	// /faces/:id/photos → 获取人物的照片
	if remaining == "photos" {
		h.handlePersonPhotos(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		person, err := h.manager.GetPerson(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    person,
		})

	case http.MethodPut:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		person, err := h.manager.UpdatePerson(id, req.Name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "person updated",
			Data:    person,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAssignFaceToPerson 分配人脸给人物 (POST /api/v1/smartgallery/faces/:id/person)
func (h *Handlers) handleAssignFaceToPerson(w http.ResponseWriter, r *http.Request, faceID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		PersonID string `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.PersonID == "" {
		writeError(w, http.StatusBadRequest, "person_id is required")
		return
	}

	if err := h.manager.AssignFaceToPerson(faceID, req.PersonID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "face assigned to person",
	})
}

// handlePersonPhotos 获取人物照片 (GET /api/v1/smartgallery/faces/:id/photos)
func (h *Handlers) handlePersonPhotos(w http.ResponseWriter, r *http.Request, personID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	photos, err := h.manager.GetPhotosByPerson(personID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    photos,
	})
}

// ---------- 场景 ----------

// handleScenes 处理场景查询 (GET /api/v1/smartgallery/scenes)
func (h *Handlers) handleScenes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sceneLabel := r.URL.Query().Get("label")
	if sceneLabel != "" {
		// 按场景获取照片
		photos := h.manager.GetPhotosByScene(sceneLabel)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    photos,
		})
		return
	}

	// 返回场景分类统计
	counts := h.manager.ClassifyScenes()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    counts,
	})
}

// ---------- 搜索 ----------

// handleSearch 处理照片搜索 (GET/POST /api/v1/smartgallery/search)
func (h *Handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 简单 GET 搜索
		req := &PhotoSearchRequest{
			Query:    r.URL.Query().Get("q"),
			DateFrom: r.URL.Query().Get("date_from"),
			DateTo:   r.URL.Query().Get("date_to"),
		}
		if v := r.URL.Query().Get("page"); v != "" {
			req.Page, _ = strconv.Atoi(v)
		}
		if v := r.URL.Query().Get("page_size"); v != "" {
			req.PageSize, _ = strconv.Atoi(v)
		}
		if v := r.URL.Query().Get("scenes"); v != "" {
			req.Scenes = strings.Split(v, ",")
		}
		if v := r.URL.Query().Get("tags"); v != "" {
			req.Tags = strings.Split(v, ",")
		}
		if v := r.URL.Query().Get("persons"); v != "" {
			req.Persons = strings.Split(v, ",")
		}
		if v := r.URL.Query().Get("is_favorite"); v != "" {
			fav := v == "true"
			req.IsFavorite = &fav
		}

		result := h.manager.SearchPhotos(req)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    result,
		})

	case http.MethodPost:
		// 复杂 POST 搜索
		var req PhotoSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		result := h.manager.SearchPhotos(&req)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    result,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---------- 时间线 ----------

// handleTimeline 处理时间线查询 (GET /api/v1/smartgallery/timeline)
func (h *Handlers) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	period := r.URL.Query().Get("period") // day, month, year
	if period == "" {
		period = "day"
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))

	timelines := h.manager.GetTimeline(period, year, month)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    timelines,
	})
}

// ---------- 标签 ----------

// handleTags 处理标签操作 (GET /api/v1/smartgallery/tags)
func (h *Handlers) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	tags := h.manager.ListTags()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    tags,
	})
}

// ---------- 导入 ----------

// handleImport 处理照片导入 (GET/POST /api/v1/smartgallery/import)
func (h *Handlers) handleImport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 获取导入任务列表，或单个任务状态
		id := r.URL.Query().Get("id")
		if id != "" {
			job, err := h.manager.GetImportJob(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "success",
				Data:    job,
			})
			return
		}

		jobs := h.manager.ListImportJobs()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    jobs,
		})

	case http.MethodPost:
		var req struct {
			Source string `json:"source"` // local, url, upload
			Path   string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.Source == "" {
			req.Source = "local"
		}
		if req.Path == "" {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}

		job, err := h.manager.ImportPhotos(req.Source, req.Path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, apiResponse{
			Code:    0,
			Message: "import started",
			Data:    job,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---------- 统计 ----------

// handleStats 处理统计请求 (GET /api/v1/smartgallery/stats)
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
