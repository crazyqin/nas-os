package smartfederation

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
	mux.HandleFunc("/api/v1/federation/clusters", h.handleClusters)
	mux.HandleFunc("/api/v1/federation/status", h.handleStatus)
	mux.HandleFunc("/api/v1/federation/sync", h.handleSync)
}

func (h *Handler) handleClusters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	clusters := h.engine.ListClusters()
	writeJSON(w, http.StatusOK, clusters)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status := h.engine.GetFederationStatus()
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		SourceID string `json:"source_id"`
		TargetID string `json:"target_id"`
		PolicyID string `json:"policy_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	
	job, err := h.engine.StartSyncJob(req.SourceID, req.TargetID, req.PolicyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
