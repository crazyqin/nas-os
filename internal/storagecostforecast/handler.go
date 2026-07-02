package storagecostforecast

import (
	"encoding/json"
	"net/http"
)

// Handler 存储成本预测 HTTP 处理器.
type Handler struct {
	engine *CostForecastEngine
}

// NewHandler 创建处理器.
func NewHandler(engine *CostForecastEngine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/storagecostforecast/forecast", h.handleForecast)
	mux.HandleFunc("/api/v1/storagecostforecast/records", h.handleRecords)
	mux.HandleFunc("/api/v1/storagecostforecast/budget", h.handleBudget)
	mux.HandleFunc("/api/v1/storagecostforecast/alerts", h.handleAlerts)
}

func (h *Handler) handleForecast(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var record CostRecord
		if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		h.engine.AddRecord(record)
		writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Limit float64 `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		h.engine.SetBudgetLimit(req.Limit)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
