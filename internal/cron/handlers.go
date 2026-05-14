package cron

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler 定时任务 HTTP 处理器.
type Handler struct {
	mgr *Manager
}

// NewHandler 创建处理器.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/cron/jobs", h.handleJobs)
	mux.HandleFunc("/api/cron/jobs/", h.handleJobByID)
	mux.HandleFunc("/api/cron/history", h.handleHistory)
	mux.HandleFunc("/api/cron/config", h.handleConfig)
	mux.HandleFunc("/api/cron/stats", h.handleStats)
}

func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		enabledOnly := r.URL.Query().Get("enabled") == "true"
		writeJSON(w, http.StatusOK, h.mgr.ListJobs(enabledOnly))
	case http.MethodPost:
		var job CronJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.mgr.AddJob(&job)
		writeJSON(w, http.StatusCreated, job)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleJobByID(w http.ResponseWriter, r *http.Request) {
	// /api/cron/jobs/{id} or /api/cron/jobs/{id}/run or /api/cron/jobs/{id}/enable or /api/cron/jobs/{id}/disable
	path := r.URL.Path[len("/api/cron/jobs/"):]
	parts := splitPath(path)
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "run" && r.Method == http.MethodPost:
		run, err := h.mgr.RunNow(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, run)
	case action == "enable" && r.Method == http.MethodPost:
		if !h.mgr.EnableJob(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
	case action == "disable" && r.Method == http.MethodPost:
		if !h.mgr.DisableJob(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
	case r.Method == http.MethodGet:
		job, ok := h.mgr.GetJob(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, job)
	case r.Method == http.MethodPut:
		var job CronJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !h.mgr.UpdateJob(id, &job) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, job)
	case r.Method == http.MethodDelete:
		if !h.mgr.DeleteJob(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	jobID := r.URL.Query().Get("job_id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	writeJSON(w, http.StatusOK, h.mgr.GetHistory(jobID, limit))
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.GetConfig())
	case http.MethodPut:
		var cfg CronConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.mgr.UpdateConfig(&cfg)
		writeJSON(w, http.StatusOK, cfg)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, h.mgr.GetStats())
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func splitPath(path string) []string {
	parts := make([]string, 0)
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
