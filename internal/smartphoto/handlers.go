package smartphoto

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler handles HTTP requests for smart photo operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new photo handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/photos", h.handlePhotos)
	mux.HandleFunc("/api/v1/photos/", h.handlePhotoByID)
	mux.HandleFunc("/api/v1/photos/search", h.handleSearch)
	mux.HandleFunc("/api/v1/photos/import", h.handleImport)
	mux.HandleFunc("/api/v1/photos/stats", h.handleStats)
	mux.HandleFunc("/api/v1/photos/duplicates", h.handleDuplicates)
	mux.HandleFunc("/api/v1/photos/cleanup", h.handleCleanup)
	mux.HandleFunc("/api/v1/albums", h.handleAlbums)
	mux.HandleFunc("/api/v1/albums/", h.handleAlbumByID)
	mux.HandleFunc("/api/v1/persons", h.handlePersons)
	mux.HandleFunc("/api/v1/persons/", h.handlePersonByID)
	mux.HandleFunc("/api/v1/shares", h.handleShares)
	mux.HandleFunc("/api/v1/shares/", h.handleShareByID)
}

func (h *Handler) handlePhotos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPhotos(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listPhotos(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	req := &SearchRequest{
		Page:     page,
		PageSize: pageSize,
		SortBy:   r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
	}

	if rating := r.URL.Query().Get("rating"); rating != "" {
		req.Rating, _ = strconv.Atoi(rating)
	}

	if isFav := r.URL.Query().Get("is_favorite"); isFav != "" {
		fav := isFav == "true"
		req.IsFavorite = &fav
	}

	result := h.manager.Search(req)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePhotoByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/photos/")
	if id == "" {
		http.Error(w, "Missing photo ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		photo, ok := h.manager.GetPhoto(id)
		if !ok {
			http.Error(w, "Photo not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, photo)
	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		photo, err := h.manager.UpdatePhoto(id, updates)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, photo)
	case http.MethodDelete:
		if err := h.manager.DeletePhoto(id); err != nil {
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

	var req SearchRequest
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

func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	status, err := h.manager.Import(r.Context(), &req)
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

func (h *Handler) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groups := h.manager.FindDuplicates()
	writeJSON(w, http.StatusOK, groups)
}

func (h *Handler) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result := h.manager.Cleanup(&req)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleAlbums(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		albums := h.manager.ListAlbums()
		writeJSON(w, http.StatusOK, albums)
	case http.MethodPost:
		var req struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			IsSmart     bool        `json:"is_smart"`
			SmartRules  *SmartRules `json:"smart_rules,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		album := h.manager.CreateAlbum(req.Name, req.Description, req.IsSmart, req.SmartRules)
		writeJSON(w, http.StatusCreated, album)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleAlbumByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/albums/")
	if id == "" {
		http.Error(w, "Missing album ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	album, ok := h.manager.GetAlbum(id)
	if !ok {
		http.Error(w, "Album not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, album)
}

func (h *Handler) handlePersons(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		persons := h.manager.ListPersons()
		writeJSON(w, http.StatusOK, persons)
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		person := h.manager.CreatePerson(req.Name)
		writeJSON(w, http.StatusCreated, person)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePersonByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/persons/")
	if id == "" {
		http.Error(w, "Missing person ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	person, ok := h.manager.GetPerson(id)
	if !ok {
		http.Error(w, "Person not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (h *Handler) handleShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	share := h.manager.CreateShare(&req)
	writeJSON(w, http.StatusCreated, share)
}

func (h *Handler) handleShareByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/shares/")
	if id == "" {
		http.Error(w, "Missing share ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	share, ok := h.manager.GetShare(id)
	if !ok {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now()) {
		http.Error(w, "Share expired", http.StatusGone)
		return
	}

	writeJSON(w, http.StatusOK, share)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
