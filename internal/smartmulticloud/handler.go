package smartmulticloud

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
	mux.HandleFunc("/api/v1/multicloud/accounts", h.handleAccounts)
	mux.HandleFunc("/api/v1/multicloud/costs", h.handleCosts)
	mux.HandleFunc("/api/v1/multicloud/forecast", h.handleForecast)
	mux.HandleFunc("/api/v1/multicloud/recommendations", h.handleRecommendations)
}

func (h *Handler) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		accounts := h.engine.ListAccounts()
		writeJSON(w, http.StatusOK, accounts)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	breakdown := h.engine.GetCostBreakdown()
	writeJSON(w, http.StatusOK, breakdown)
}

func (h *Handler) handleForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		AccountID string `json:"account_id"`
		Period    string `json:"period"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	
	forecast, err := h.engine.ForecastCost(req.AccountID, req.Period)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, forecast)
}

func (h *Handler) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	recs := h.engine.GenerateRecommendations()
	writeJSON(w, http.StatusOK, recs)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
