package smarttier2

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
	mux.HandleFunc("/api/smarttier2/stats", h.handleStats)
	mux.HandleFunc("/api/smarttier2/analyze", h.handleAnalyze)
	mux.HandleFunc("/api/smarttier2/data", h.handleData)
	mux.HandleFunc("/api/smarttier2/migrate", h.handleMigrate)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.engine.GetStats())
}

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	recs := h.engine.Analyze()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recommendations": recs,
		"count":           len(recs),
	})
}

func (h *Handler) handleData(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id required")
			return
		}
		dc, exists := h.engine.GetData(id)
		if !exists {
			writeError(w, http.StatusNotFound, "data not found")
			return
		}
		writeJSON(w, http.StatusOK, dc)
	case http.MethodPost:
		var dc DataClass
		if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.engine.AddData(&dc)
		writeJSON(w, http.StatusCreated, dc)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		DataID     string `json:"data_id"`
		TargetTier Tier   `json:"target_tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.engine.Migrate(req.DataID, req.TargetTier); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "migrated"})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
