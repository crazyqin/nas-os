package smartscrub

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建HTTP处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smartscrub/policies", h.handlePolicies)
	mux.HandleFunc("/api/v1/smartscrub/policies/", h.handlePolicyByID)
	mux.HandleFunc("/api/v1/smartscrub/stats", h.handleStats)
}

func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := h.manager.ListPolicies()
		writeJSON(w, http.StatusOK, map[string]any{"policies": policies})
	case http.MethodPost:
		var req CreatePolicyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
			return
		}
		policy, err := h.manager.CreatePolicy(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, policy)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handlePolicyByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/smartscrub/policies/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少策略ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		policy, err := h.manager.GetPolicy(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case http.MethodDelete:
		if err := h.manager.DeletePolicy(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
	case http.MethodPost:
		result, err := h.manager.RunScrub(id)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
