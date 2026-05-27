package diskspace

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler handles disk space HTTP requests
type Handler struct {
	manager *DiskSpaceManager
}

// NewHandler creates a new disk space handler
func NewHandler(manager *DiskSpaceManager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes registers disk space API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/diskspace/scan", h.handleScan)
	mux.HandleFunc("/api/v1/diskspace/scan/progress", h.handleGetScanProgress)
	mux.HandleFunc("/api/v1/diskspace/usage", h.handleGetDiskUsage)
	mux.HandleFunc("/api/v1/diskspace/tree", h.handleGetDirectoryTree)
	mux.HandleFunc("/api/v1/diskspace/filetypes", h.handleGetFileTypeStats)
	mux.HandleFunc("/api/v1/diskspace/large-files", h.handleFindLargeFiles)
	mux.HandleFunc("/api/v1/diskspace/duplicates", h.handleFindDuplicates)
	mux.HandleFunc("/api/v1/diskspace/treemap", h.handleGetTreemapData)
	mux.HandleFunc("/api/v1/diskspace/growth", h.handleGetGrowthTrend)
	mux.HandleFunc("/api/v1/diskspace/export", h.handleExportReport)
}

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	
	if err := h.manager.StartScan(context.Background(), req.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	writeJSON(w, http.StatusAccepted, SuccessResponse{
		Success: true,
		Data:    map[string]string{"status": "scan started"},
	})
}

func (h *Handler) handleGetScanProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	progress := h.manager.GetScanProgress()
	writeJSON(w, http.StatusOK, progress)
}

func (h *Handler) handleGetDiskUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	
	usage := h.manager.GetDiskUsage(path)
	writeJSON(w, http.StatusOK, usage)
}

func (h *Handler) handleGetDirectoryTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	
	maxDepthStr := r.URL.Query().Get("max_depth")
	maxDepth := 3
	if maxDepthStr != "" {
		if d, err := strconv.Atoi(maxDepthStr); err == nil && d > 0 {
			maxDepth = d
		}
	}
	
	tree := h.manager.GetDirectoryTree(path, maxDepth)
	writeJSON(w, http.StatusOK, tree)
}

func (h *Handler) handleGetFileTypeStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	
	stats := h.manager.GetFileTypeStats(path)
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleFindLargeFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	
	minSizeStr := r.URL.Query().Get("min_size")
	minSize := int64(1024 * 1024) // 1MB default
	if minSizeStr != "" {
		if s, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil && s > 0 {
			minSize = s
		}
	}
	
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	
	files := h.manager.FindLargeFiles(path, minSize, limit)
	writeJSON(w, http.StatusOK, files)
}

func (h *Handler) handleFindDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	
	duplicates, err := h.manager.FindDuplicates(context.Background(), path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	writeJSON(w, http.StatusOK, duplicates)
}

func (h *Handler) handleGetTreemapData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	
	maxDepthStr := r.URL.Query().Get("max_depth")
	maxDepth := 3
	if maxDepthStr != "" {
		if d, err := strconv.Atoi(maxDepthStr); err == nil && d > 0 {
			maxDepth = d
		}
	}
	
	treemap := h.manager.GetTreemapData(path, maxDepth)
	writeJSON(w, http.StatusOK, treemap)
}

func (h *Handler) handleGetGrowthTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}
	
	trend := h.manager.GetGrowthTrend(days)
	writeJSON(w, http.StatusOK, trend)
}

func (h *Handler) handleExportReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	
	if format != "json" && format != "text" {
		writeError(w, http.StatusBadRequest, "unsupported format. Use 'json' or 'text'")
		return
	}
	
	data, err := h.manager.ExportReport(format)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}
