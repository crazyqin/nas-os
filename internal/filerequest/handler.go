package filerequest

import (
	"encoding/json"
	"net/http"
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
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	TargetPath     string    `json:"target_path"`
	MaxFiles       int       `json:"max_files"`
	MaxSizeMB      int       `json:"max_size_mb"`
	ExpiresAt      time.Time `json:"expires_at"`
	AllowOverwrite bool      `json:"allow_overwrite"`
	RequireAuth    bool      `json:"require_auth"`
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
		input.AllowOverwrite, input.RequireAuth,
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

	creatorID := r.Header.Get("X-User-ID")
	if creatorID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "X-User-ID header required"})
		return
	}

	requests := h.manager.ListRequests(creatorID)
	if requests == nil {
		requests = []*Request{}
	}
	writeJSON(w, http.StatusOK, requests)
}

// Get handles GET /api/v1/file-requests/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request, id string) {
	req, err := h.manager.GetRequest(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// Revoke handles DELETE /api/v1/file-requests/{id}
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.RevokeRequest(id); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// PublicUpload handles POST /api/v1/upload/{token} - public endpoint for uploaders
func (h *Handler) PublicUpload(w http.ResponseWriter, r *http.Request, token string) {
	req, err := h.manager.GetRequestByToken(token)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "invalid or expired link"})
		return
	}

	if req.Status != StatusActive {
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

	upload, err := h.manager.RecordUpload(
		req.ID, header.Filename, header.Size,
		header.Header.Get("Content-Type"), r.RemoteAddr,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, upload)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
