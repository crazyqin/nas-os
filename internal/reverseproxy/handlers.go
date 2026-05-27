package reverseproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Handler REST API handler (v1)
type Handler struct {
	manager *Manager
}

// NewHandler creates a REST API handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// handleProxies handles list (GET) and create (POST) for /proxies
func (h *Handler) handleProxies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		proxies := h.manager.ListProxies()
		respondJSON(w, proxies)
	case http.MethodPost:
		var req CreateProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Domain == "" || req.TargetURL == "" {
			http.Error(w, "name, domain and target_url are required", http.StatusBadRequest)
			return
		}
		proxy, err := h.manager.CreateProxy(req)
		if err != nil {
			if err.Error() == fmt.Sprintf("proxy with domain '%s' already exists", req.Domain) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		respondJSON(w, proxy)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleProxyByID handles get/update/delete for /proxies/{id}
func (h *Handler) handleProxyByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/v1/reverse-proxy/proxies/{id}[/rules]
	path := r.URL.Path
	// Trim prefix
	trimmed := path[len("/api/v1/reverse-proxy/proxies/"):]

	// Check if this is a rules request
	if idx := indexOf(trimmed, "/rules"); idx >= 0 {
		proxyID := trimmed[:idx]
		h.handleRules(w, r, proxyID)
		return
	}

	id := trimmed

	switch r.Method {
	case http.MethodGet:
		proxy, err := h.manager.GetProxy(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		respondJSON(w, proxy)
	case http.MethodPut:
		var req UpdateProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdateProxy(id, req); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		proxy, _ := h.manager.GetProxy(id)
		respondJSON(w, proxy)
	case http.MethodDelete:
		if err := h.manager.DeleteProxy(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		respondJSON(w, SuccessResponse{Success: true, Message: "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRules handles rules for a proxy
func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request, proxyID string) {
	switch r.Method {
	case http.MethodGet:
		rules, err := h.manager.GetRules(proxyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		respondJSON(w, rules)
	case http.MethodPost:
		var req AddRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		rule := ProxyRule{
			Path:          req.Path,
			TargetURL:     req.TargetURL,
			LoadBalancing: req.LoadBalancing,
			RateLimit:     req.RateLimit,
		}
		if err := h.manager.AddRule(proxyID, rule); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		respondJSON(w, rule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStats handles stats request
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	respondJSON(w, stats)
}

// handleReload handles config reload
func (h *Handler) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.manager.ReloadConfig()
	respondJSON(w, SuccessResponse{Success: true, Message: "config reloaded"})
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ReverseProxyHandler 反向代理HTTP处理器
type ReverseProxyHandler struct {
	manager *ReverseProxyManager
}

// NewReverseProxyHandler 创建处理器
func NewReverseProxyHandler(manager *ReverseProxyManager) *ReverseProxyHandler {
	return &ReverseProxyHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *ReverseProxyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/reverseproxy/add", h.handleAddProxy)
	mux.HandleFunc("/api/reverseproxy/remove", h.handleRemoveProxy)
	mux.HandleFunc("/api/reverseproxy/update", h.handleUpdateProxy)
	mux.HandleFunc("/api/reverseproxy/get", h.handleGetProxy)
	mux.HandleFunc("/api/reverseproxy/list", h.handleListProxies)
	mux.HandleFunc("/api/reverseproxy/enable", h.handleEnableProxy)
	mux.HandleFunc("/api/reverseproxy/disable", h.handleDisableProxy)
	mux.HandleFunc("/api/reverseproxy/stats", h.handleGetStats)
}

// handleAddProxy 处理添加代理请求
func (h *ReverseProxyHandler) handleAddProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.AddProxy(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, rule)
}

// handleRemoveProxy 处理移除代理请求
func (h *ReverseProxyHandler) handleRemoveProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveProxy(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"status": "removed"})
}

// handleUpdateProxy 处理更新代理请求
func (h *ReverseProxyHandler) handleUpdateProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID   string     `json:"id"`
		Rule ProxyRule  `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.UpdateProxy(req.ID, &req.Rule); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"status": "updated"})
}

// handleGetProxy 处理获取代理请求
func (h *ReverseProxyHandler) handleGetProxy(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	proxy, err := h.manager.GetProxy(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, proxy)
}

// handleListProxies 处理列出代理请求
func (h *ReverseProxyHandler) handleListProxies(w http.ResponseWriter, r *http.Request) {
	proxies := h.manager.ListProxies()
	respondJSON(w, proxies)
}

// handleEnableProxy 处理启用代理请求
func (h *ReverseProxyHandler) handleEnableProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.EnableProxy(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"status": "enabled"})
}

// handleDisableProxy 处理禁用代理请求
func (h *ReverseProxyHandler) handleDisableProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.DisableProxy(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"status": "disabled"})
}

// handleGetStats 处理获取统计请求
func (h *ReverseProxyHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	respondJSON(w, stats)
}

// respondJSON 响应JSON
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
