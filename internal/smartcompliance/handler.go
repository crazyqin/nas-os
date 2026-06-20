package smartcompliance

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
	mux.HandleFunc("/api/v1/compliance/rules", h.handleRules)
	mux.HandleFunc("/api/v1/compliance/audit", h.handleAudit)
	mux.HandleFunc("/api/v1/compliance/status", h.handleStatus)
	mux.HandleFunc("/api/v1/compliance/access-check", h.handleAccessCheck)
}

func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rules := h.engine.ListRules("")
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Standard ComplianceStandard `json:"standard"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	
	result, err := h.engine.RunAudit(req.Standard)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status := h.engine.GetComplianceStatus()
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleAccessCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Subject  string `json:"subject"`
		Resource string `json:"resource"`
		Action   string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	
	allowed := h.engine.CheckAccess(req.Subject, req.Resource, req.Action)
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": allowed})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
