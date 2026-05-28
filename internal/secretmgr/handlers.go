package secretmgr

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
	mux.HandleFunc("/api/v1/secretmgr/secrets", h.handleSecrets)
	mux.HandleFunc("/api/v1/secretmgr/secrets/", h.handleSecretByID)
	mux.HandleFunc("/api/v1/secretmgr/stats", h.handleStats)
}

func (h *Handler) handleSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		secrets := h.manager.ListSecrets()
		writeJSON(w, http.StatusOK, map[string]any{"secrets": secrets})
	case http.MethodPost:
		var req CreateSecretRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
			return
		}
		secret, err := h.manager.CreateSecret(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, secret)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持的方法"})
	}
}

func (h *Handler) handleSecretByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/secretmgr/secrets/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少密钥ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		secret, err := h.manager.GetSecret(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, secret)
	case http.MethodDelete:
		if err := h.manager.DeleteSecret(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
	case http.MethodPut:
		var req struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求"})
			return
		}
		if err := h.manager.UpdateSecret(id, req.Value); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已更新"})
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
