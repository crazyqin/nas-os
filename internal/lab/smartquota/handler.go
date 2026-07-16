package smartquota

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/quotas", h.handleQuotas)
	mux.HandleFunc("/api/v1/quotas/stats", h.handleStats)
	mux.HandleFunc("/api/v1/quotas/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/quotas/usage", h.handleUsage)
}

func (h *Handler) handleQuotas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listQuotas(w, r)
	case http.MethodPost:
		h.createQuota(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listQuotas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	quotaType := QuotaType(r.URL.Query().Get("type"))
	tenantID := r.URL.Query().Get("tenant_id")

	quotas, err := h.manager.ListQuotas(ctx, quotaType, tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"quotas": quotas,
		"total":  len(quotas),
	})
}

func (h *Handler) createQuota(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var quota Quota
	if err := json.NewDecoder(r.Body).Decode(&quota); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	created, err := h.manager.CreateQuota(ctx, quota)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	tenantID := r.URL.Query().Get("tenant_id")

	stats, err := h.manager.GetStats(ctx, tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getAlerts(w, r)
	case http.MethodPut:
		h.ackAlert(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	unackedOnly := r.URL.Query().Get("unacked_only") == "true"

	alerts := h.manager.GetAlerts(ctx, unackedOnly)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (h *Handler) ackAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		AlertID string `json:"alert_id"`
		AckedBy string `json:"acked_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.AckAlert(ctx, req.AlertID, req.AckedBy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "acked"})
}

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	var req struct {
		QuotaID   string `json:"quota_id"`
		UsedBytes int64  `json:"used_bytes"`
		UsedFiles int64  `json:"used_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.UpdateUsage(ctx, req.QuotaID, req.UsedBytes, req.UsedFiles); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
