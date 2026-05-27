package photoenhance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

// Handler handles HTTP requests for photo enhancement
type Handler struct {
	manager *Manager
}

// NewHandler creates a new photo enhancement handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/photo-enhance/enhance", h.handleEnhance)
	mux.HandleFunc("/api/v1/photo-enhance/batch", h.handleBatch)
	mux.HandleFunc("/api/v1/photo-enhance/job", h.handleJob)
	mux.HandleFunc("/api/v1/photo-enhance/jobs", h.handleJobs)
	mux.HandleFunc("/api/v1/photo-enhance/stats", h.handleStats)
	mux.HandleFunc("/api/v1/photo-enhance/presets", h.handlePresets)
}

// handleEnhance handles single photo enhancement
func (h *Handler) handleEnhance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req EnhancementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if req.SourcePath == "" {
		http.Error(w, "source_path is required", http.StatusBadRequest)
		return
	}
	
	if req.OutputPath == "" {
		ext := filepath.Ext(req.SourcePath)
		req.OutputPath = req.SourcePath[:len(req.SourcePath)-len(ext)] + "_enhanced" + ext
	}
	
	if req.ID == "" {
		req.ID = fmt.Sprintf("enh_%d", time.Now().UnixNano())
	}
	req.CreatedAt = time.Now()
	
	result, err := h.manager.EnhancePhoto(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleBatch handles batch enhancement requests
func (h *Handler) handleBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Name     string               `json:"name"`
		Requests []*EnhancementRequest `json:"requests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if len(req.Requests) == 0 {
		http.Error(w, "requests array is required", http.StatusBadRequest)
		return
	}
	
	if req.Name == "" {
		req.Name = fmt.Sprintf("Batch_%s", time.Now().Format("2006-01-02_15:04"))
	}
	
	job := h.manager.CreateBatchJob(req.Name, req.Requests)
	
	// Process job in background
	go h.manager.ProcessBatchJob(r.Context(), job.ID)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(job)
}

// handleJob handles job status requests
func (h *Handler) handleJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "id parameter is required", http.StatusBadRequest)
		return
	}
	
	job, err := h.manager.GetJob(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// handleJobs handles jobs listing requests
func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.manager.ListJobs()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// handleStats handles statistics requests
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handlePresets returns available enhancement presets
func (h *Handler) handlePresets(w http.ResponseWriter, r *http.Request) {
	presets := []map[string]interface{}{
		{
			"id":          "quick_enhance",
			"name":        "快速增强",
			"description": "一键智能增强，平衡速度和质量",
			"type":        EnhanceSuperRes,
			"quality":     QualityFast,
			"scale":       2,
		},
		{
			"id":          "photo_restore",
			"name":        "老照片修复",
			"description": "修复划痕、折痕、褪色的老照片",
			"type":        EnhanceRepair,
			"quality":     QualityBest,
		},
		{
			"id":          "colorize_bw",
			"name":        "黑白上色",
			"description": "为黑白照片智能添加色彩",
			"type":        EnhanceColorize,
			"quality":     QualityBalance,
		},
		{
			"id":          "face_enhance",
			"name":        "人脸增强",
			"description": "修复模糊、低分辨率的人脸",
			"type":        EnhanceFace,
			"quality":     QualityBest,
		},
		{
			"id":          "super_res_4x",
			"name":        "4K超分辨率",
			"description": "4倍放大，适合打印和大屏显示",
			"type":        EnhanceSuperRes,
			"quality":     QualityBest,
			"scale":       4,
		},
		{
			"id":          "dehaze",
			"name":        "去雾增强",
			"description": "去除雾霾，恢复清晰度",
			"type":        EnhanceDehaze,
			"quality":     QualityBalance,
		},
		{
			"id":          "hdr_enhance",
			"name":        "HDR增强",
			"description": "提升动态范围和色彩表现",
			"type":        EnhanceHDR,
			"quality":     QualityBalance,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"presets": presets,
		"total":   len(presets),
	})
}
