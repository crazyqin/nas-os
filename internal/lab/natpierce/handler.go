// Package natpierce API handlers
package natpierce

import (
	"encoding/json"
	"net/http"
)

// Handler API处理器.
type Handler struct {
	client *PierceClient
}

// NewHandler 创建API处理器.
func NewHandler(client *PierceClient) *Handler {
	return &Handler{client: client}
}

// HandleStatus GET /api/natpierce/status.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	status := h.client.GetStatus()
	_ = json.NewEncoder(w).Encode(status)
}

// HandleConfig GET/POST /api/natpierce/config.
func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(h.client.config)
	case http.MethodPost:
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// 更新配置并重启
		_ = h.client.Stop()
		h.client.config = &cfg
		if err := h.client.Start(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleConnect POST /api/natpierce/connect.
func (h *Handler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	if err := h.client.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleDisconnect POST /api/natpierce/disconnect.
func (h *Handler) HandleDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.client.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
