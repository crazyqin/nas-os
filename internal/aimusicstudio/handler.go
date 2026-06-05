package aimusicstudio

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/musicstudio/generate", h.handleGenerate)
	mux.HandleFunc("/api/v1/musicstudio/compositions", h.handleListCompositions)
	mux.HandleFunc("/api/v1/musicstudio/composition", h.handleGetComposition)
	mux.HandleFunc("/api/v1/musicstudio/mix", h.handleMix)
	mux.HandleFunc("/api/v1/musicstudio/export", h.handleExport)
	mux.HandleFunc("/api/v1/musicstudio/analyze", h.handleAnalyze)
}

// handleGenerate 处理AI作曲请求
func (h *Handler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 设置默认值
	if req.Duration == 0 {
		req.Duration = 60.0 // 默认60秒
	}
	if req.BPM == 0 {
		req.BPM = 120
	}
	if len(req.Instruments) == 0 {
		req.Instruments = []string{"piano", "guitar", "bass", "drums"}
	}

	comp, err := h.service.GenerateComposition(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// handleListCompositions 列出作品
func (h *Handler) handleListCompositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	genre := MusicGenre(r.URL.Query().Get("genre"))
	mood := Mood(r.URL.Query().Get("mood"))

	compositions := h.service.ListCompositions(genre, mood)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"compositions": compositions,
		"total":        len(compositions),
	})
}

// handleGetComposition 获取作品详情
func (h *Handler) handleGetComposition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing composition id", http.StatusBadRequest)
		return
	}

	comp, err := h.service.GetComposition(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// handleMix 处理混音请求
func (h *Handler) handleMix(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var mix MixConfig
		if err := json.NewDecoder(r.Body).Decode(&mix); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.service.UpdateMix(&mix); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	case http.MethodGet:
		compositionID := r.URL.Query().Get("composition_id")
		if compositionID == "" {
			http.Error(w, "Missing composition_id", http.StatusBadRequest)
			return
		}

		mix, err := h.service.GetMix(compositionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mix)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleExport 导出作品
func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	format := r.URL.Query().Get("format")
	if id == "" {
		http.Error(w, "Missing composition id", http.StatusBadRequest)
		return
	}
	if format == "" {
		format = "json"
	}

	data, err := h.service.ExportComposition(r.Context(), id, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleAnalyzecomposition 分析作品
func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing composition id", http.StatusBadRequest)
		return
	}

	analysis, err := h.service.AnalyzeComposition(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}
