// Package dataverifier 提供数据完整性校验 HTTP 处理器
package dataverifier

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建HTTP处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/dataverifier/jobs", h.handleJobs)
	mux.HandleFunc("/api/v1/dataverifier/jobs/", h.handleJobByID)
	mux.HandleFunc("/api/v1/dataverifier/stats", h.handleStats)
	mux.HandleFunc("/api/v1/dataverifier/verify", h.handleVerify)
}

func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jobs := h.manager.ListJobs()
		writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
	case http.MethodPost:
		var req CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
			return
		}
		job, err := h.manager.CreateJob(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, job)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleJobByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/dataverifier/jobs/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少任务ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		job, err := h.manager.GetJob(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, job)
	case http.MethodDelete:
		if err := h.manager.DeleteJob(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
	case http.MethodPost:
		// 触发运行
		result, err := h.manager.RunJob(id)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
		return
	}

	var req struct {
		Path      string          `json:"path"`
		Hash      string          `json:"hash"`
		Algorithm VerifyAlgorithm `json:"algorithm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
		return
	}

	ok, err := h.manager.VerifyChecksum(req.Path, req.Hash, req.Algorithm)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"valid": ok})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
