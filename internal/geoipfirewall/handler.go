package geoipfirewall

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler provides HTTP API handlers for GeoIP firewall.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new GeoIP firewall handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers GeoIP firewall API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// v1 routes (legacy)
	mux.HandleFunc("/api/v1/geoip/check", h.handleCheck)
	mux.HandleFunc("/api/v1/geoip/rules", h.handleRules)
	mux.HandleFunc("/api/v1/geoip/rules/", h.handleRuleByID)
	mux.HandleFunc("/api/v1/geoip/stats", h.handleStats)
	mux.HandleFunc("/api/v1/geoip/blocked", h.handleBlocked)
	mux.HandleFunc("/api/v1/geoip/countries/block", h.handleBlockCountry)
	mux.HandleFunc("/api/v1/geoip/countries/unblock", h.handleUnblockCountry)
	mux.HandleFunc("/api/v1/geoip/update", h.handleUpdateDB)

	// New canonical routes
	mux.HandleFunc("/api/geoipfirewall/rules", h.handleRules)
	mux.HandleFunc("/api/geoipfirewall/rules/", h.handleRuleByID)
	mux.HandleFunc("/api/geoipfirewall/stats", h.handleStats)
	mux.HandleFunc("/api/geoipfirewall/blocked", h.handleBlocked)
	mux.HandleFunc("/api/geoipfirewall/lookup", h.handleLookup)
	mux.HandleFunc("/api/geoipfirewall/countries", h.handleCountries)
}

func (h *Handler) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip parameter required"})
		return
	}

	result, err := h.manager.CheckIP(ip)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := h.manager.ListRules()
		writeJSON(w, http.StatusOK, rules)
	case http.MethodPost:
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := h.manager.AddRule(&rule); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, rule)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/geoip/rules/")
	if id == "" {
		http.Error(w, "rule ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rule, err := h.manager.GetRule(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, rule)
	case http.MethodPut:
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rule.ID = id
		if err := h.manager.UpdateRule(&rule); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, rule)
	case http.MethodDelete:
		if err := h.manager.DeleteRule(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleBlocked(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	blocked := h.manager.GetBlockedConnections(100)
	writeJSON(w, http.StatusOK, blocked)
}

func (h *Handler) handleBlockCountry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.manager.BlockCountry(req.Code); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "blocked", "code": req.Code})
}

func (h *Handler) handleUnblockCountry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.manager.UnblockCountry(req.Code); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked", "code": req.Code})
}

func (h *Handler) handleUpdateDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.manager.UpdateGeoDB(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "update initiated"})
}

func (h *Handler) handleLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.IP == "" {
		writeErr(w, http.StatusBadRequest, "ip is required")
		return
	}

	entry, err := h.manager.LookupIP(req.IP)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *Handler) handleCountries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	countries := h.manager.GetCountries()
	writeJSON(w, http.StatusOK, countries)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
