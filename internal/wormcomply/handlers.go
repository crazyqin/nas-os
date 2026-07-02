package wormcomply

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler WORM 合规引擎 HTTP handler.
type Handler struct {
	engine *Engine
}

// NewHandler 创建 handler.
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/wormcomply/policies", h.handlePolicies)
	mux.HandleFunc("/api/v1/wormcomply/policies/", h.handlePolicyByID)
	mux.HandleFunc("/api/v1/wormcomply/check", h.handleCheckAccess)
	mux.HandleFunc("/api/v1/wormcomply/violations", h.handleViolations)
	mux.HandleFunc("/api/v1/wormcomply/violations/", h.handleResolveViolation)
	mux.HandleFunc("/api/v1/wormcomply/audit", h.handleAuditLog)
	mux.HandleFunc("/api/v1/wormcomply/report", h.handleReport)
	mux.HandleFunc("/api/v1/wormcomply/suspend/", h.handleSuspendPolicy)
	mux.HandleFunc("/api/v1/wormcomply/activate/", h.handleActivatePolicy)
}

func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := h.engine.ListPolicies()
		writeJSON(w, http.StatusOK, policies)
	case http.MethodPost:
		var policy RetentionPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := h.engine.CreatePolicy(&policy); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, policy)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePolicyByID(w http.ResponseWriter, r *http.Request) {
	policyID := r.URL.Path[len("/api/v1/wormcomply/policies/"):]
	if policyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy ID required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		policy, err := h.engine.GetPolicy(policyID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case http.MethodPut:
		var policy RetentionPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := h.engine.UpdatePolicy(policyID, &policy); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case http.MethodDelete:
		userID := r.URL.Query().Get("user_id")
		if err := h.engine.DeletePolicy(policyID, userID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCheckAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath  string `json:"file_path"`
		Operation string `json:"operation"`
		UserID    string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	err := h.engine.CheckFileAccess(req.FilePath, req.Operation, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"allowed": false,
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"allowed": true,
	})
}

func (h *Handler) handleViolations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	unresolved := r.URL.Query().Get("unresolved") == "true"
	violations := h.engine.GetViolations(unresolved)
	writeJSON(w, http.StatusOK, violations)
}

func (h *Handler) handleResolveViolation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	violationID := r.URL.Path[len("/api/v1/wormcomply/violations/"):]
	if violationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "violation ID required"})
		return
	}

	var req struct {
		ResolvedBy string `json:"resolved_by"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.engine.ResolveViolation(violationID, req.ResolvedBy); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (h *Handler) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var limit int
	var policyID string
	if v := r.URL.Query().Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		limit = parsed
	}
	policyID = r.URL.Query().Get("policy_id")

	log := h.engine.GetAuditLog(limit, policyID)
	writeJSON(w, http.StatusOK, log)
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "daily"
	}

	report := h.engine.GenerateReport(period)
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleSuspendPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	policyID := r.URL.Path[len("/api/v1/wormcomply/suspend/"):]
	if policyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy ID required"})
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.engine.SuspendPolicy(policyID, req.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "suspended"})
}

func (h *Handler) handleActivatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	policyID := r.URL.Path[len("/api/v1/wormcomply/activate/"):]
	if policyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy ID required"})
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.engine.ActivatePolicy(policyID, req.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
