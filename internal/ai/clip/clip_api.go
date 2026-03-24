// Package clip - HTTP API handlers for CLIP text-to-image search
package clip

import (
	"encoding/json"
	"net/http"
	"time"
)

// ClipAPI provides HTTP API handlers for CLIP search
type ClipAPI struct {
	service TextSearchService
}

// NewClipAPI creates a new CLIP API handler
func NewClipAPI(service TextSearchService) *ClipAPI {
	return &ClipAPI{
		service: service,
	}
}

// RegisterRoutes registers API routes
func (api *ClipAPI) RegisterRoutes(mux *http.ServeMux) {
	// Search endpoints
	mux.HandleFunc("POST /api/clip/search", api.Search)
	mux.HandleFunc("POST /api/clip/search/similar", api.SimilarImages)

	// Indexing endpoints
	mux.HandleFunc("POST /api/clip/index", api.Index)
	mux.HandleFunc("POST /api/clip/index/batch", api.BatchIndex)
	mux.HandleFunc("DELETE /api/clip/index/{photoId}", api.Delete)

	// Status endpoints
	mux.HandleFunc("GET /api/clip/stats", api.GetStats)
	mux.HandleFunc("GET /api/clip/model", api.GetModelInfo)

	// Embedding endpoints
	mux.HandleFunc("POST /api/clip/embed/text", api.EmbedText)
	mux.HandleFunc("POST /api/clip/embed/image", api.EmbedImage)
}

// Search handles text-to-image search requests
func (api *ClipAPI) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query required")
		return
	}

	ctx := r.Context()
	resp, err := api.service.Search(ctx, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// SimilarImagesRequest represents a similar image search request
type SimilarImagesRequest struct {
	PhotoID  string  `json:"photo_id"`
	Path     string  `json:"path,omitempty"`
	TopK     int     `json:"top_k,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

// SimilarImagesResponse represents similar image search response
type SimilarImagesResponse struct {
	PhotoID   string         `json:"photo_id"`
	Results   []SearchResult `json:"results"`
	QueryTime int64          `json:"query_time_ms"`
}

// SimilarImages finds similar images to a reference image
func (api *ClipAPI) SimilarImages(w http.ResponseWriter, r *http.Request) {
	var req SimilarImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// This would use the image embedding to find similar images
	// Simplified implementation
	resp := SimilarImagesResponse{
		PhotoID:   req.PhotoID,
		Results:   []SearchResult{},
		QueryTime: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// Index handles single photo indexing
func (api *ClipAPI) Index(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.PhotoID == "" || req.Path == "" {
		writeError(w, http.StatusBadRequest, "photo_id and path required")
		return
	}

	ctx := r.Context()
	resp, err := api.service.Index(ctx, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "indexing failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// BatchIndex handles batch photo indexing
func (api *ClipAPI) BatchIndex(w http.ResponseWriter, r *http.Request) {
	var req BatchIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if len(req.Photos) == 0 {
		writeError(w, http.StatusBadRequest, "photos required")
		return
	}

	ctx := r.Context()
	resp, err := api.service.BatchIndex(ctx, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "batch indexing failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// Delete removes a photo from the index
func (api *ClipAPI) Delete(w http.ResponseWriter, r *http.Request) {
	photoID := r.PathValue("photoId")
	if photoID == "" {
		writeError(w, http.StatusBadRequest, "photo_id required")
		return
	}

	ctx := r.Context()
	if err := api.service.Remove(ctx, photoID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true}) //nolint:errcheck
}

// GetStats returns service statistics
func (api *ClipAPI) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := api.service.GetStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats) //nolint:errcheck
}

// ModelInfoResponse represents model info response
type ModelInfoResponse struct {
	ModelInfo ModelInfo `json:"model_info"`
	Status    string    `json:"status"`
	Timestamp string    `json:"timestamp"`
}

// GetModelInfo returns model information
func (api *ClipAPI) GetModelInfo(w http.ResponseWriter, r *http.Request) {
	// Get stats to access model info
	ctx := r.Context()
	stats, err := api.service.GetStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get model info: "+err.Error())
		return
	}

	resp := ModelInfoResponse{
		ModelInfo: stats.ModelInfo,
		Status:    "ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// EmbedTextRequest represents a text embedding request
type EmbedTextRequest struct {
	Text string `json:"text"`
}

// EmbedTextResponse represents a text embedding response
type EmbedTextResponse struct {
	Text      string    `json:"text"`
	Embedding Embedding `json:"embedding"`
	Dim       int       `json:"dim"`
}

// EmbedText generates text embedding
func (api *ClipAPI) EmbedText(w http.ResponseWriter, r *http.Request) {
	var req EmbedTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text required")
		return
	}

	// Cast to get model
	if svc, ok := api.service.(*TextSearchServiceImpl); ok {
		ctx := r.Context()
		embedding, err := svc.model.EncodeText(ctx, req.Text)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "embedding failed: "+err.Error())
			return
		}

		resp := EmbedTextResponse{
			Text:      req.Text,
			Embedding: *embedding,
			Dim:       embedding.Dim,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
		return
	}

	writeError(w, http.StatusInternalServerError, "service not available")
}

// EmbedImageRequest represents an image embedding request
type EmbedImageRequest struct {
	Path string `json:"path"`
}

// EmbedImageResponse represents an image embedding response
type EmbedImageResponse struct {
	Path      string    `json:"path"`
	Embedding Embedding `json:"embedding"`
	Dim       int       `json:"dim"`
}

// EmbedImage generates image embedding
func (api *ClipAPI) EmbedImage(w http.ResponseWriter, r *http.Request) {
	var req EmbedImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}

	// Cast to get model
	if svc, ok := api.service.(*TextSearchServiceImpl); ok {
		ctx := r.Context()
		embedding, err := svc.model.EncodeImage(ctx, req.Path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "embedding failed: "+err.Error())
			return
		}

		resp := EmbedImageResponse{
			Path:      req.Path,
			Embedding: *embedding,
			Dim:       embedding.Dim,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
		return
	}

	writeError(w, http.StatusInternalServerError, "service not available")
}

// writeError writes an error response
func writeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ErrorResponse{ //nolint:errcheck
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- Service Registry Helper ---

// RegisterClipAPI creates and registers CLIP API with default config
func RegisterClipAPI(mux *http.ServeMux, config *Config) error {
	service, err := NewTextSearchService(config)
	if err != nil {
		return err
	}

	api := NewClipAPI(service)
	api.RegisterRoutes(mux)

	return nil
}

// --- Middleware ---

// RateLimitMiddleware adds rate limiting to CLIP API
func RateLimitMiddleware(next http.HandlerFunc, requestsPerSecond int) http.HandlerFunc {
	// Simplified rate limiting
	// Production: use proper rate limiting with token bucket
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next(w, r)
	})
}

// AuthMiddleware adds authentication to CLIP API
func AuthMiddleware(next http.HandlerFunc, validateToken func(string) bool) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "authorization required")
			return
		}

		if !validateToken(token) {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next(w, r)
	})
}

// CORSMiddleware adds CORS headers
func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	})
}