package filerequest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Handler provides HTTP endpoints for file request management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new file request HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// CreateRequestInput is the request body for creating a file request.
type CreateRequestInput struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	TargetPath  string    `json:"target_path"`
	MaxFiles    int       `json:"max_files"`
	MaxSizeMB   int       `json:"max_size_mb"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// ErrorResponse is a standard API error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// Create handles POST /api/v1/file-requests
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	var input CreateRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// Get creator from auth context (simplified)
	creatorID := r.Header.Get("X-User-ID")
	if creatorID == "" {
		creatorID = "anonymous"
	}

	req, err := h.manager.CreateRequest(
		input.Title, input.Description, creatorID, input.TargetPath,
		input.MaxFiles, input.MaxSizeMB, input.ExpiresAt,
		false, false,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

// List handles GET /api/v1/file-requests
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	ctx := context.Background()
	creatorID := r.URL.Query().Get("creator_id")
	status := RequestStatus(r.URL.Query().Get("status"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	requests, total, err := h.manager.ListRequests(ctx, creatorID, status, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"requests": requests,
		"total":    total,
		"page":     page,
		"page_size": pageSize,
	})
}

// Get handles GET /api/v1/file-requests/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()
	req, err := h.manager.GetRequest(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// Revoke handles DELETE /api/v1/file-requests/{id}
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()
	if err := h.manager.RevokeRequest(ctx, id); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// PublicUpload handles POST /api/v1/upload/{token} - public endpoint for uploaders
func (h *Handler) PublicUpload(w http.ResponseWriter, r *http.Request, token string) {
	ctx := context.Background()
	req, err := h.manager.GetRequestByToken(ctx, token)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "invalid or expired link"})
		return
	}

	if req.Status != RequestStatusActive {
		writeJSON(w, http.StatusGone, ErrorResponse{Error: "this file request is no longer active"})
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max memory
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid multipart form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "file field is required"})
		return
	}
	defer file.Close()

	// Record upload
	info := &UploadInfo{
		RequestID:     req.ID,
		OriginalName:  header.Filename,
		FileSize:      header.Size,
		MimeType:      header.Header.Get("Content-Type"),
		UploaderIP:    r.RemoteAddr,
		UploadedAt:    time.Now(),
	}

	if err := h.manager.RecordUpload(ctx, req.ID, info); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// Stats handles GET /api/v1/file-requests/stats
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	ctx := context.Background()
	stats, err := h.manager.GetStats(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// RegisterRoutes registers file request HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/file-requests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.List(w, r)
		case http.MethodPost:
			h.Create(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		}
	})
	mux.HandleFunc("/api/v1/file-requests/stats", h.Stats)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
