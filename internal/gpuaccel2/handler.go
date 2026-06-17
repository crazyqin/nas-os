package gpuaccel2

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP API 处理器。
type Handler struct {
	engine *Engine
}

// NewHandler 创建新的 HTTP 处理器。
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册 HTTP 路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/gpuaccel2/gpus", h.handleGPUs)
	mux.HandleFunc("/api/gpuaccel2/models", h.handleModels)
	mux.HandleFunc("/api/gpuaccel2/infer", h.handleInfer)
	mux.HandleFunc("/api/gpuaccel2/stats", h.handleStats)
}

func (h *Handler) handleGPUs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			gpu, exists := h.engine.GetGPU(id)
			if !exists {
				writeError(w, http.StatusNotFound, "GPU not found")
				return
			}
			writeJSON(w, http.StatusOK, gpu)
			return
		}
		writeJSON(w, http.StatusOK, h.engine.ListGPUs())
	case http.MethodPost:
		var gpu GPUDevice
		if err := json.NewDecoder(r.Body).Decode(&gpu); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.engine.AddGPU(&gpu)
		writeJSON(w, http.StatusCreated, gpu)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.engine.ListModels())
	case http.MethodPost:
		var model Model
		if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.engine.LoadModel(&model); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, model)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id required")
			return
		}
		if err := h.engine.UnloadModel(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "unloaded"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleInfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.engine.Infer(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.engine.GetStats())
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
