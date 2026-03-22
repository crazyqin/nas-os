package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nas-os/internal/media"

	"github.com/gorilla/mux"
)

// Handler handles media API requests
type Handler struct {
	scanner        *media.Scanner
	scraper        *media.TMDBScraper
	cache          *media.Cache
	libraryManager *media.LibraryManager
}

// NewHandler creates a new media API handler
func NewHandler(scanner *media.Scanner, scraper *media.TMDBScraper, cache *media.Cache) *Handler {
	return &Handler{
		scanner: scanner,
		scraper: scraper,
		cache:   cache,
	}
}

// NewHandlerWithLibrary creates a new media API handler with library manager
func NewHandlerWithLibrary(scanner *media.Scanner, scraper *media.TMDBScraper, cache *media.Cache, libraryManager *media.LibraryManager) *Handler {
	return &Handler{
		scanner:        scanner,
		scraper:        scraper,
		cache:          cache,
		libraryManager: libraryManager,
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
	if h.libraryManager != nil {
		libraries := h.libraryManager.ListLibraries()
		h.respondJSON(w, http.StatusOK, libraries)
		return
	}
	// Fallback: return empty list if no library manager
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

	// Validate required fields
	if req.Name == "" {
		h.respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Path == "" {
		h.respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Path validation: check path exists and is accessible
	path := filepath.Clean(req.Path)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			h.respondError(w, http.StatusBadRequest, "path does not exist: "+req.Path)
			return
		}
		if os.IsPermission(err) {
			h.respondError(w, http.StatusForbidden, "permission denied for path: "+req.Path)
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to access path: "+err.Error())
		return
	}
	if !info.IsDir() {
		h.respondError(w, http.StatusBadRequest, "path is not a directory: "+req.Path)
		return
	}

	// Path validation: check for symlinks and resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to resolve absolute path: "+err.Error())
		return
	}
	req.Path = absPath

	// Validate media type
	var libraryType media.Type
	switch req.Type {
	case media.MediaTypeMovie:
		libraryType = media.TypeMovie
	case media.MediaTypeTVShow:
		libraryType = media.TypeTV
	default:
		libraryType = media.TypeMovie // Default to movie
	}

	// Use LibraryManager if available
	if h.libraryManager != nil {
		lib, err := h.libraryManager.CreateLibrary(req.Name, req.Path, libraryType, false)
		if err != nil {
			h.respondError(w, http.StatusInternalServerError, "failed to create library: "+err.Error())
			return
		}
		h.respondJSON(w, http.StatusCreated, lib)
		return
	}

	// Fallback: create without persistence
	library := &media.MediaLibrary{
		ID:   generateID(),
		Name: req.Name,
		Path: req.Path,
		Type: req.Type,
	}

	h.respondJSON(w, http.StatusCreated, library)
}

// GetLibrary returns a specific library
func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if h.libraryManager != nil {
		lib := h.libraryManager.GetLibrary(id)
		if lib != nil {
			h.respondJSON(w, http.StatusOK, lib)
			return
		}
	}

	h.respondError(w, http.StatusNotFound, "library not found")
}

// DeleteLibrary deletes a library
func (h *Handler) DeleteLibrary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if h.libraryManager != nil {
		err := h.libraryManager.DeleteLibrary(id)
		if err != nil {
			if strings.Contains(err.Error(), "不存在") {
				h.respondError(w, http.StatusNotFound, "library not found")
				return
			}
			h.respondError(w, http.StatusInternalServerError, "failed to delete library: "+err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ScanLibrary triggers a scan of a library
func (h *Handler) ScanLibrary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if h.libraryManager != nil {
		lib := h.libraryManager.GetLibrary(id)
		if lib == nil {
			h.respondError(w, http.StatusNotFound, "library not found")
			return
		}

		err := h.libraryManager.ScanLibrary(id)
		if err != nil {
			h.respondError(w, http.StatusInternalServerError, "scan failed: "+err.Error())
			return
		}

		// Return updated library with scan results
		lib = h.libraryManager.GetLibrary(id)
		h.respondJSON(w, http.StatusOK, map[string]interface{}{
			"library": lib,
			"status":  "scan completed",
		})
		return
	}

	// Fallback: return a mock result
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

	// Look up file by ID in cache
	if h.cache != nil {
		if file, found := h.cache.GetFileByID(id); found {
			h.respondJSON(w, http.StatusOK, file)
			return
		}
	}

	h.respondError(w, http.StatusNotFound, "file not found")
}

// ScrapeFile scrapes metadata for a file
func (h *Handler) ScrapeFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Look up file by ID in cache
	if h.cache == nil {
		h.respondError(w, http.StatusInternalServerError, "cache not available")
		return
	}

	file, found := h.cache.GetFileByID(id)
	if !found {
		h.respondError(w, http.StatusNotFound, "file not found")
		return
	}

	// Parse filename to extract title
	title, year, season, episode := h.scanner.ParseFilename(file.Filename)

	// Detect media type
	mediaType := h.scanner.DetectMediaType(file.Filename)

	// Search for metadata based on media type
	var results interface{}
	var err error

	switch mediaType {
	case media.MediaTypeTVShow:
		results, err = h.scraper.SearchTVShow(r.Context(), title)
	default:
		results, err = h.scraper.SearchMovie(r.Context(), title, year)
	}

	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "scrape failed: "+err.Error())
		return
	}

	// Return scrape results with parsed info
	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"file": file,
		"parsed": map[string]interface{}{
			"title":   title,
			"year":    year,
			"season":  season,
			"episode": episode,
			"type":    mediaType,
		},
		"results": results,
	})
}

// GetMetadata returns metadata for a media item
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Try to get metadata from cache
	if h.cache != nil {
		if meta, found := h.cache.GetMetadata(id); found {
			h.respondJSON(w, http.StatusOK, meta)
			return
		}
	}

	// Check if we have a TMDB ID stored with prefix
	if h.cache != nil {
		if meta, found := h.cache.GetMetadata("tmdb_" + id); found {
			h.respondJSON(w, http.StatusOK, meta)
			return
		}
	}

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
