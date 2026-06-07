package fastdedup

import (
	"encoding/json"
	"net/http"
)

// APIHandler HTTP API处理器
type APIHandler struct {
	engine *FastDedupEngine
}

// NewAPIHandler 创建API处理器
func NewAPIHandler(engine *FastDedupEngine) *APIHandler {
	return &APIHandler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/fastdedup/status", h.handleStatus)
	mux.HandleFunc(prefix+"/fastdedup/stats", h.handleStats)
	mux.HandleFunc(prefix+"/fastdedup/policies", h.handlePolicies)
	mux.HandleFunc(prefix+"/fastdedup/run", h.handleRun)
	mux.HandleFunc(prefix+"/fastdedup/start", h.handleStart)
	mux.HandleFunc(prefix+"/fastdedup/stop", h.handleStop)
}

func (h *APIHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":  h.engine.IsRunning(),
		"blocks":   h.engine.GetBlockCount(),
		"policies": len(h.engine.ListPolicies()),
	})
}

func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.engine.GetStats())
}

func (h *APIHandler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.engine.ListPolicies())
}

func (h *APIHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Policy string `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.engine.RunDedup(req.Policy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.engine.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *APIHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.engine.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
