package smbguard

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler SMB 防护 HTTP handler.
type Handler struct {
	engine *Engine
}

// NewHandler 创建 handler.
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smbguard/connections", h.handleConnections)
	mux.HandleFunc("/api/v1/smbguard/policies", h.handlePolicies)
	mux.HandleFunc("/api/v1/smbguard/policies/", h.handlePolicyByID)
	mux.HandleFunc("/api/v1/smbguard/blocked", h.handleBlockedIPs)
	mux.HandleFunc("/api/v1/smbguard/unblock/", h.handleUnblock)
	mux.HandleFunc("/api/v1/smbguard/whitelist", h.handleWhitelist)
	mux.HandleFunc("/api/v1/smbguard/blacklist", h.handleBlacklist)
	mux.HandleFunc("/api/v1/smbguard/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/smbguard/alerts/", h.handleAcknowledgeAlert)
	mux.HandleFunc("/api/v1/smbguard/cleanup", h.handleCleanup)
}

func (h *Handler) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conns := h.engine.ListConnections()
	writeJSON(w, http.StatusOK, conns)
}

func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := h.engine.ListPolicies()
		writeJSON(w, http.StatusOK, policies)
	case http.MethodPost:
		var policy BlockPolicy
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
	policyID := r.URL.Path[len("/api/v1/smbguard/policies/"):]
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
		var policy BlockPolicy
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
		if err := h.engine.DeletePolicy(policyID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleBlockedIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	blocked := h.engine.GetBlockedIPs()
	writeJSON(w, http.StatusOK, blocked)
}

func (h *Handler) handleUnblock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ipStr := r.URL.Path[len("/api/v1/smbguard/unblock/"):]
	if ipStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "IP required"})
		return
	}

	if err := h.engine.UnblockIP(ipStr); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

func (h *Handler) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := h.engine.ListWhitelist()
		writeJSON(w, http.StatusOK, entries)
	case http.MethodPost:
		var req struct {
			IP      string `json:"ip"`
			Reason  string `json:"reason"`
			AddedBy string `json:"added_by"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := h.engine.AddToWhitelist(req.IP, req.Reason, req.AddedBy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
	case http.MethodDelete:
		var req struct {
			IP string `json:"ip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := h.engine.RemoveFromWhitelist(req.IP); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := h.engine.ListBlacklist()
		writeJSON(w, http.StatusOK, entries)
	case http.MethodPost:
		var req struct {
			IP              string `json:"ip"`
			Reason          string `json:"reason"`
			AddedBy         string `json:"added_by"`
			DurationSeconds int    `json:"duration_seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := h.engine.AddToBlacklist(req.IP, req.Reason, req.AddedBy, req.DurationSeconds); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
	case http.MethodDelete:
		var req struct {
			IP string `json:"ip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := h.engine.RemoveFromBlacklist(req.IP); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	unack := r.URL.Query().Get("unacknowledged") == "true"
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	alerts := h.engine.GetAlerts(limit, unack)
	writeJSON(w, http.StatusOK, alerts)
}

func (h *Handler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alertID := r.URL.Path[len("/api/v1/smbguard/alerts/"):]
	if alertID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "alert ID required"})
		return
	}

	if err := h.engine.AcknowledgeAlert(alertID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func (h *Handler) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count := h.engine.CleanupExpired()
	writeJSON(w, http.StatusOK, map[string]int{"cleaned": count})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
