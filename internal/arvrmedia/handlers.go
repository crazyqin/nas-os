package arvrmedia

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler 处理 AR/VR 媒体 HTTP 请求
type Handler struct {
	manager *Manager
}

// NewHandler 创建新的 AR/VR 媒体处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/arvr/panoramas", HandlePanoramas(h.manager))
	mux.HandleFunc("/api/v1/arvr/panoramas/", HandlePanoramaByID(h.manager))
	mux.HandleFunc("/api/v1/arvr/models", HandleModels(h.manager))
	mux.HandleFunc("/api/v1/arvr/models/", HandleModelByID(h.manager))
	mux.HandleFunc("/api/v1/arvr/galleries", HandleGalleries(h.manager))
	mux.HandleFunc("/api/v1/arvr/galleries/", HandleGalleryByID(h.manager))
	mux.HandleFunc("/api/v1/arvr/audio-configs", HandleAudioConfigs(h.manager))
	mux.HandleFunc("/api/v1/arvr/audio-configs/", HandleAudioConfigByID(h.manager))
	mux.HandleFunc("/api/v1/arvr/theaters", HandleTheaters(h.manager))
	mux.HandleFunc("/api/v1/arvr/theaters/", HandleTheaterByID(h.manager))
	mux.HandleFunc("/api/v1/arvr/sessions", HandleSessions(h.manager))
	mux.HandleFunc("/api/v1/arvr/sessions/", HandleSessionByID(h.manager))
	mux.HandleFunc("/api/v1/arvr/import", HandleImport(h.manager))
	mux.HandleFunc("/api/v1/arvr/import/", HandleImportStatus(h.manager))
	mux.HandleFunc("/api/v1/arvr/stats", HandleStats(h.manager))
	mux.HandleFunc("/api/v1/arvr/webxr/manifest", HandleWebXRManifest())
}

// HandlePanoramas 处理全景媒体列表/创建
func HandlePanoramas(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mediaType := r.URL.Query().Get("type")
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			panoramas, total := m.ListPanoramas(mediaType, page, pageSize)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"panoramas": panoramas,
				"total":     total,
				"page":      page,
				"page_size": pageSize,
				"has_more":  page*pageSize < total,
			})

		case http.MethodPost:
			var req PanoramaMedia
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			media, err := m.CreatePanorama(&req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, media)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandlePanoramaByID 处理单个全景媒体操作
func HandlePanoramaByID(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/panoramas/")
		if id == "" {
			http.Error(w, "Missing panorama ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			media, ok := m.GetPanorama(id)
			if !ok {
				http.Error(w, "Panorama not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, media)

		case http.MethodPut:
			var updates map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			media, err := m.UpdatePanorama(id, updates)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, media)

		case http.MethodDelete:
			if err := m.DeletePanorama(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleModels 处理3D模型列表/创建
func HandleModels(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			format := r.URL.Query().Get("format")
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			models, total := m.ListModels(format, page, pageSize)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"models":    models,
				"total":     total,
				"page":      page,
				"page_size": pageSize,
				"has_more":  page*pageSize < total,
			})

		case http.MethodPost:
			var req Model3D
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			model, err := m.CreateModel(&req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, model)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleModelByID 处理单个3D模型操作
func HandleModelByID(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/models/")
		if id == "" {
			http.Error(w, "Missing model ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			model, ok := m.GetModel(id)
			if !ok {
				http.Error(w, "Model not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, model)

		case http.MethodDelete:
			if err := m.DeleteModel(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleGalleries 处理VR画廊列表/创建
func HandleGalleries(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			galleries := m.ListGalleries()
			writeJSON(w, http.StatusOK, galleries)

		case http.MethodPost:
			var req VREntry
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			gallery, err := m.CreateGallery(&req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, gallery)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleGalleryByID 处理单个VR画廊操作
func HandleGalleryByID(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/galleries/")
		if id == "" {
			http.Error(w, "Missing gallery ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			gallery, ok := m.GetGallery(id)
			if !ok {
				http.Error(w, "Gallery not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, gallery)

		case http.MethodPut:
			var updates map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			gallery, err := m.UpdateGallery(id, updates)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, gallery)

		case http.MethodDelete:
			if err := m.DeleteGallery(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleAudioConfigs 处理空间音频配置列表/创建
func HandleAudioConfigs(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configs := m.ListAudioConfigs()
			writeJSON(w, http.StatusOK, configs)

		case http.MethodPost:
			var req SpatialAudioConfig
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			config, err := m.CreateAudioConfig(&req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, config)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleAudioConfigByID 处理单个空间音频配置操作
func HandleAudioConfigByID(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/audio-configs/")
		if id == "" {
			http.Error(w, "Missing audio config ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			config, ok := m.GetAudioConfig(id)
			if !ok {
				http.Error(w, "Audio config not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, config)

		case http.MethodPut:
			var updates map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			config, err := m.UpdateAudioConfig(id, updates)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, config)

		case http.MethodDelete:
			if err := m.DeleteAudioConfig(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleTheaters 处理沉浸式影院列表/创建
func HandleTheaters(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			theaters := m.ListTheaters()
			writeJSON(w, http.StatusOK, theaters)

		case http.MethodPost:
			var req ImmersiveTheater
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			theater, err := m.CreateTheater(&req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, theater)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleTheaterByID 处理单个沉浸式影院操作
func HandleTheaterByID(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/theaters/")
		if id == "" {
			http.Error(w, "Missing theater ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			theater, ok := m.GetTheater(id)
			if !ok {
				http.Error(w, "Theater not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, theater)

		case http.MethodDelete:
			if err := m.DeleteTheater(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleSessions 处理WebXR会话列表/创建
func HandleSessions(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			sessions := m.ListActiveSessions()
			writeJSON(w, http.StatusOK, sessions)

		case http.MethodPost:
			var req struct {
				Mode     XRMode `json:"mode"`
				DeviceID string `json:"device_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			session, err := m.CreateSession(req.Mode, req.DeviceID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, session)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleSessionByID 处理单个WebXR会话操作
func HandleSessionByID(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/sessions/")
		if id == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			session, ok := m.GetSession(id)
			if !ok {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, session)

		case http.MethodPut:
			var req struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			session, err := m.UpdateSessionStatus(id, req.Status)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, session)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// HandleImport 处理媒体导入请求
func HandleImport(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			SourcePath string    `json:"source_path"`
			MediaType  MediaType `json:"media_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		task, err := m.ImportMedia(r.Context(), req.SourcePath, req.MediaType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusAccepted, task)
	}
}

// HandleImportStatus 处理导入状态查询
func HandleImportStatus(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/v1/arvr/import/")
		if id == "" {
			http.Error(w, "Missing import ID", http.StatusBadRequest)
			return
		}

		task, ok := m.GetImportTask(id)
		if !ok {
			http.Error(w, "Import task not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, task)
	}
}

// HandleStats 处理统计信息请求
func HandleStats(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		stats := m.GetStats()
		writeJSON(w, http.StatusOK, stats)
	}
}

// HandleWebXRManifest 处理WebXR清单请求
func HandleWebXRManifest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		manifest := map[string]interface{}{
			"name":             "NAS-OS AR/VR Media",
			"short_name":       "ARVR",
			"start_url":        "/api/v1/arvr/webxr",
			"display":          "immersive-vr",
			"xr":               map[string]interface{}{
				"optional_features": []string{
					"local-floor",
					"bounded-floor",
					"hand-tracking",
					"hit-test",
					"anchors",
					"plane-detection",
					"mesh-detection",
					"depth-sensing",
				},
			},
			"supported_modes": []string{"immersive-vr", "immersive-ar", "inline"},
		}

		writeJSON(w, http.StatusOK, manifest)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
