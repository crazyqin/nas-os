package ransomml2

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
	mux.HandleFunc("/api/ransomml2/activity", h.handleActivity)
	mux.HandleFunc("/api/ransomml2/threats", h.handleThreats)
	mux.HandleFunc("/api/ransomml2/threat", h.handleThreat)
	mux.HandleFunc("/api/ransomml2/model", h.handleModel)
	mux.HandleFunc("/api/ransomml2/stats", h.handleStats)
	mux.HandleFunc("/api/ransomml2/config", h.handleConfig)
}

func (h *Handler) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var activity FileActivity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.engine.RecordActivity(activity)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *Handler) handleThreats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.engine.ListThreats())
}

func (h *Handler) handleThreat(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		threat, exists := h.engine.GetThreat(id)
		if !exists {
			writeError(w, http.StatusNotFound, "threat not found")
			return
		}
		writeJSON(w, http.StatusOK, threat)
	case http.MethodPut:
		if err := h.engine.ResolveThreat(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.engine.GetModel())
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.engine.GetStats())
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.engine.GetConfig())
	case http.MethodPut:
		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.engine.UpdateConfig(config)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
