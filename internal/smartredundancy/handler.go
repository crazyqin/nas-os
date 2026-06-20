package smartredundancy

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器.
type Handler struct {
	engine *Engine
}

// NewHandler 创建HTTP处理器.
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/redundancy/nodes", h.handleNodes)
	mux.HandleFunc("/api/v1/redundancy/status", h.handleStatus)
	mux.HandleFunc("/api/v1/redundancy/placement", h.handlePlacement)
}

func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nodes := h.engine.ListNodes()
		writeJSON(w, http.StatusOK, nodes)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status := h.engine.GetClusterStatus()
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handlePlacement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Policy   RedundancyPolicy `json:"policy"`
		DataSize int64            `json:"data_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	placement, err := h.engine.CalculatePlacement(&req.Policy, req.DataSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, placement)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
