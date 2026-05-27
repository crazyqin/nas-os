package reverseproxy

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler handles reverse proxy HTTP requests
type Handler struct {
	manager *Manager
}

// NewHandler creates a new reverse proxy handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes registers reverse proxy API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/reverse-proxy/proxies", h.handleProxies)
	mux.HandleFunc("/api/v1/reverse-proxy/proxies/", h.handleProxyByID)
	mux.HandleFunc("/api/v1/reverse-proxy/stats", h.handleStats)
	mux.HandleFunc("/api/v1/reverse-proxy/reload", h.handleReload)
}

func (h *Handler) handleProxies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listProxies(w, r)
	case http.MethodPost:
		h.createProxy(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listProxies(w http.ResponseWriter, r *http.Request) {
	proxies := h.manager.ListProxies()
	writeJSON(w, http.StatusOK, proxies)
}

func (h *Handler) createProxy(w http.ResponseWriter, r *http.Request) {
	var req CreateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if req.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "target_url is required")
		return
	}

	proxy, err := h.manager.CreateProxy(req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, proxy)
}

func (h *Handler) handleProxyByID(w http.ResponseWriter, r *http.Request) {
	// Extract proxy ID from path: /api/v1/reverse-proxy/proxies/{id}
	path := r.URL.Path
	prefix := "/api/v1/reverse-proxy/proxies/"
	if !strings.HasPrefix(path, prefix) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	remaining := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remaining, "/")
	id := parts[0]

	if id == "" {
		writeError(w, http.StatusBadRequest, "proxy ID is required")
		return
	}

	// Check for sub-resource: /rules
	if len(parts) > 1 && parts[1] == "rules" {
		h.handleProxyRules(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getProxy(w, r, id)
	case http.MethodPut:
		h.updateProxy(w, r, id)
	case http.MethodDelete:
		h.deleteProxy(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getProxy(w http.ResponseWriter, r *http.Request, id string) {
	proxy, err := h.manager.GetProxy(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proxy)
}

func (h *Handler) updateProxy(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.UpdateProxy(id, req); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	proxy, err := h.manager.GetProxy(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proxy)
}

func (h *Handler) deleteProxy(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.DeleteProxy(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *Handler) handleProxyRules(w http.ResponseWriter, r *http.Request, proxyID string) {
	switch r.Method {
	case http.MethodGet:
		h.getRules(w, r, proxyID)
	case http.MethodPost:
		h.addRule(w, r, proxyID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getRules(w http.ResponseWriter, r *http.Request, proxyID string) {
	rules, err := h.manager.GetRules(proxyID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) addRule(w http.ResponseWriter, r *http.Request, proxyID string) {
	var req AddRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if req.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "target_url is required")
		return
	}

	rule := ProxyRule{
		Path:          req.Path,
		TargetURL:     req.TargetURL,
		LoadBalancing: req.LoadBalancing,
		HealthCheck:   req.HealthCheck,
		RateLimit:     req.RateLimit,
		IPWhitelist:   req.IPWhitelist,
	}

	if err := h.manager.AddRule(proxyID, rule); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.manager.ReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}
