package aitraffic

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器
type Handler struct {
	analyzer *Analyzer
}

// NewHandler 创建处理器
func NewHandler(analyzer *Analyzer) *Handler {
	return &Handler{analyzer: analyzer}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/aitraffic/stats", h.handleGetStats)
	mux.HandleFunc("/api/aitraffic/anomalies", h.handleGetAnomalies)
	mux.HandleFunc("/api/aitraffic/ingest", h.handleIngest)
}

func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.analyzer.GetStats()
	writeJSON(w, stats)
}

func (h *Handler) handleGetAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	anomalies := h.analyzer.GetAnomalies()
	writeJSON(w, anomalies)
}

func (h *Handler) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var record FlowRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	h.analyzer.IngestFlow(record)
	w.WriteHeader(http.StatusAccepted)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
