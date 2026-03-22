package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"nas-os/internal/media"
)

// Handler handles media API requests
type Handler struct {
	scanner *media.Scanner
	scraper *media.TMDBScraper
	cache   *media.Cache
}

// NewHandler creates a new media API handler
func NewHandler(scanner *media.Scanner, scraper *media.TMDBScraper, cache *media.Cache) *Handler {
	return &Handler{
		scanner: scanner,
		scraper: scraper,
		cache:   cache,
	}
}

// RegisterRoutes registers media API routes
func (h *Handler) RegisterRoutes(r *mux.Router) {
	api := r.PathPrefix("/api/media").Subrouter()

	// Library management
	api.HandleFunc("/libraries", h.ListLibraries).Methods("GET")
	api.HandleFunc("/libraries", h.CreateLibrary).Methods("POST")
	api.HandleFunc("/libraries/{id}", h.GetLibrary).Methods("GET")
	api.HandleFunc("/libraries/{id}", h.DeleteLibrary).Methods("DELETE")
	api.HandleFunc("/libraries/{id}/scan", h.ScanLibrary).Methods("POST")

	// Media files
	api.HandleFunc("/files", h.ListFiles).Methods("GET")
	api.HandleFunc("/files/{id}", h.GetFile).Methods("GET")
	api.HandleFunc("/files/{id}/scrape", h.ScrapeFile).Methods("POST")

	// Metadata
	api.HandleFunc("/metadata/{id}", h.GetMetadata).Methods("GET")
	api.HandleFunc("/metadata/search", h.SearchMetadata).Methods("GET")

	// Posters and artwork
	api.HandleFunc("/posters/{id}", h.GetPoster).Methods("GET")
	api.HandleFunc("/backdrops/{id}", h.GetBackdrop).Methods("GET")
}

// ListLibraries returns all media libraries
func (h *Handler) ListLibraries(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement library storage
	libraries := []*media.MediaLibrary{}
	h.respondJSON(w, http.StatusOK, libraries)
}

// CreateLibrary creates a new media library
func (h *Handler) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string          `json:"name"`
		Path string          `json:"path"`
		Type media.MediaType `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate path exists
	// TODO: Add path validation

	library := &media.MediaLibrary{
		ID:   generateID(),
		Name: req.Name,
		Path: req.Path,
		Type: req.Type,
	}

	// TODO: Save to storage

	h.respondJSON(w, http.StatusCreated, library)
}

// GetLibrary returns a specific library
func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Get from storage
	_ = id

	h.respondError(w, http.StatusNotFound, "library not found")
}

// DeleteLibrary deletes a library
func (h *Handler) DeleteLibrary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Delete from storage
	_ = id

	w.WriteHeader(http.StatusNoContent)
}

// ScanLibrary triggers a scan of a library
func (h *Handler) ScanLibrary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Get library and scan
	_ = id

	// For now, return a mock result
	result := &media.ScanResult{
		LibraryID:  id,
		TotalFiles: 0,
		NewFiles:   0,
	}

	h.respondJSON(w, http.StatusOK, result)
}

// ListFiles lists all media files
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	files := h.scanner.GetVideoFiles()
	h.respondJSON(w, http.StatusOK, files)
}

// GetFile returns a specific file
func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Implement file lookup by ID
	_ = id

	h.respondError(w, http.StatusNotFound, "file not found")
}

// ScrapeFile scrapes metadata for a file
func (h *Handler) ScrapeFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Get file by ID and scrape
	_ = id

	h.respondError(w, http.StatusNotFound, "file not found")
}

// GetMetadata returns metadata for a media item
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Get from cache/storage
	_ = id

	h.respondError(w, http.StatusNotFound, "metadata not found")
}

// SearchMetadata searches for metadata
func (h *Handler) SearchMetadata(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	mediaType := r.URL.Query().Get("type")
	yearStr := r.URL.Query().Get("year")

	if query == "" {
		h.respondError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	year, _ := strconv.Atoi(yearStr)

	var results interface{}
	var err error

	switch mediaType {
	case "movie":
		results, err = h.scraper.SearchMovie(r.Context(), query, year)
	case "tv":
		results, err = h.scraper.SearchTVShow(r.Context(), query)
	default:
		// Try both
		movie, movieErr := h.scraper.SearchMovie(r.Context(), query, year)
		if movieErr == nil {
			results = movie
		} else {
			results, err = h.scraper.SearchTVShow(r.Context(), query)
		}
	}

	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, results)
}

// GetPoster returns a poster image
func (h *Handler) GetPoster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	size := r.URL.Query().Get("size")

	posterURL := h.scraper.GetPosterURL(id, size)
	if posterURL == "" {
		h.respondError(w, http.StatusNotFound, "poster not found")
		return
	}

	// Redirect to TMDB image URL
	http.Redirect(w, r, posterURL, http.StatusFound)
}

// GetBackdrop returns a backdrop image
func (h *Handler) GetBackdrop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	size := r.URL.Query().Get("size")

	if size == "" {
		size = "w1280"
	}

	backdropURL := "https://image.tmdb.org/t/p/" + size + id
	http.Redirect(w, r, backdropURL, http.StatusFound)
}

// respondJSON sends a JSON response
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error but don't modify response as headers already sent
		// In production, use structured logging
		_ = err // explicitly ignore for now
	}
}

// respondError sends an error response
func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
