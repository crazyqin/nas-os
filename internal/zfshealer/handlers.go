package zfshealer

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler provides HTTP endpoints for ZFS integrity management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new Handler with the given manager.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers all ZFSHealer HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/zfshealer/health", h.handleHealth)
	mux.HandleFunc("/api/v1/zfshealer/scrub", h.handleScrub)
	mux.HandleFunc("/api/v1/zfshealer/results", h.handleResults)
	mux.HandleFunc("/api/v1/zfshealer/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/zfshealer/schedule", h.handleSchedule)
	mux.HandleFunc("/api/v1/zfshealer/status", h.handleStatus)
}

// GET /api/v1/zfshealer/health - List all dataset health info.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	health := h.manager.ListDatasetHealth()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"datasets": health,
		"count":    len(health),
	})
}

// POST /api/v1/zfshealer/scrub - Start a scrub operation.
func (h *Handler) handleScrub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Dataset string `json:"dataset"`
		Level   string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Dataset == "" {
		http.Error(w, "dataset is required", http.StatusBadRequest)
		return
	}

	level := IntegrityStandard
	if req.Level != "" {
		level = IntegrityLevel(req.Level)
	}

	result, err := h.manager.RunScrub(r.Context(), req.Dataset, level)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GET /api/v1/zfshealer/results - Get recent scrub results.
func (h *Handler) handleResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	results := h.manager.GetResults(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// GET /api/v1/zfshealer/alerts - Get recent integrity alerts.
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	alerts := h.manager.GetAlerts(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// GET/PUT /api/v1/zfshealer/schedule - Get or update scrub schedule.
func (h *Handler) handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		schedule := h.manager.GetSchedule()
		writeJSON(w, http.StatusOK, schedule)
	case http.MethodPut:
		var schedule ScrubSchedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		h.manager.UpdateSchedule(schedule)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/v1/zfshealer/status - Get current healer status.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	datasets := h.manager.ListDatasetHealth()
	totalErrors := int64(0)
	for _, d := range datasets {
		totalErrors += d.ChecksumErrors + d.ScanErrors
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":           h.manager.IsRunning(),
		"monitored_datasets": len(datasets),
		"total_errors":      totalErrors,
		"schedule":          h.manager.GetSchedule(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
