package homemedia

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler handles HTTP requests for home media operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new media handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/media", h.handleMedia)
	mux.HandleFunc("/api/v1/media/", h.handleMediaByID)
	mux.HandleFunc("/api/v1/media/search", h.handleSearch)
	mux.HandleFunc("/api/v1/media/scan", h.handleScan)
	mux.HandleFunc("/api/v1/media/stats", h.handleStats)
	mux.HandleFunc("/api/v1/collections", h.handleCollections)
	mux.HandleFunc("/api/v1/collections/", h.handleCollectionByID)
	mux.HandleFunc("/api/v1/playlists", h.handlePlaylists)
	mux.HandleFunc("/api/v1/playlists/", h.handlePlaylistByID)
	mux.HandleFunc("/api/v1/playback", h.handlePlayback)
	mux.HandleFunc("/api/v1/playback/", h.handlePlaybackByID)
}

func (h *Handler) handleMedia(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listMedia(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listMedia(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	req := &MediaSearchRequest{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
		Type:      r.URL.Query().Get("type"),
	}

	if rating := r.URL.Query().Get("rating"); rating != "" {
		req.Rating, _ = strconv.ParseFloat(rating, 64)
	}

	result := h.manager.Search(req)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleMediaByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/media/")
	if id == "" {
		http.Error(w, "Missing media ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		media, ok := h.manager.GetMedia(id)
		if !ok {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, media)
	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		media, err := h.manager.UpdateMedia(id, updates)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, media)
	case http.MethodDelete:
		if err := h.manager.DeleteMedia(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MediaSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	result := h.manager.Search(&req)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	status, err := h.manager.Scan(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, status)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleCollections(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		collections := h.manager.ListCollections()
		writeJSON(w, http.StatusOK, collections)
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		collection := h.manager.CreateCollection(req.Name, req.Description, req.Type)
		writeJSON(w, http.StatusCreated, collection)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCollectionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/collections/")
	if id == "" {
		http.Error(w, "Missing collection ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collection, ok := h.manager.GetCollection(id)
	if !ok {
		http.Error(w, "Collection not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

func (h *Handler) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		playlists := h.manager.ListPlaylists()
		writeJSON(w, http.StatusOK, playlists)
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		playlist := h.manager.CreatePlaylist(req.Name, req.Description)
		writeJSON(w, http.StatusCreated, playlist)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePlaylistByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/playlists/")
	if id == "" {
		http.Error(w, "Missing playlist ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		playlist, ok := h.manager.GetPlaylist(id)
		if !ok {
			http.Error(w, "Playlist not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, playlist)
	case http.MethodPost:
		var req struct {
			MediaID string `json:"media_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.AddToPlaylist(id, req.MediaID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePlayback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		MediaID    string `json:"media_id"`
		UserID     string `json:"user_id"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.manager.StartPlayback(req.MediaID, req.UserID, req.DeviceID, req.DeviceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) handlePlaybackByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/playback/")
	if id == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			CurrentTime int `json:"current_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdatePlaybackProgress(id, req.CurrentTime); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	case http.MethodDelete:
		if err := h.manager.StopPlayback(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
