package storageml

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handlers provides HTTP handlers for the storage ML API
type Handlers struct {
	manager *StorageML
}

// NewHandlers creates new HTTP handlers
func NewHandlers(manager *StorageML) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/storageml/predict", h.handlePredict)
	mux.HandleFunc("/api/v1/storageml/trend", h.handleTrend)
	mux.HandleFunc("/api/v1/storageml/anomalies", h.handleAnomalies)
	mux.HandleFunc("/api/v1/storageml/summary", h.handleSummary)
	mux.HandleFunc("/api/v1/storageml/expansion", h.handleExpansion)
	mux.HandleFunc("/api/v1/storageml/pools", h.handlePools)
	mux.HandleFunc("/api/v1/storageml/collect", h.handleCollect)
}

// handlePredict handles prediction requests
func (h *Handlers) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	metricType := MetricType(r.URL.Query().Get("metric_type"))
	daysStr := r.URL.Query().Get("days")

	if poolID == "" || metricType == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	days := 30 // Default 30 days
	if daysStr != "" {
		// Parse days from string
		// Simplified for brevity
		_ = daysStr
	}

	futureDate := time.Now().AddDate(0, 0, days)
	prediction, err := h.manager.GetPredictor().Predict(poolID, metricType, futureDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prediction)
}

// handleTrend handles trend analysis requests
func (h *Handlers) handleTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	metricType := MetricType(r.URL.Query().Get("metric_type"))

	if poolID == "" || metricType == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	trend, err := h.manager.GetAnalyzer().AnalyzeTrend(poolID, metricType, 30*24*time.Hour)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trend)
}

// handleAnomalies handles anomaly detection requests
func (h *Handlers) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	metricType := MetricType(r.URL.Query().Get("metric_type"))

	if poolID == "" || metricType == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	anomalies, err := h.manager.GetAnalyzer().DetectAnomalies(poolID, metricType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(anomalies)
}

// handleSummary handles summary requests
func (h *Handlers) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		http.Error(w, "Missing pool_id parameter", http.StatusBadRequest)
		return
	}

	summary := h.manager.GetAnalyzer().GetUsageSummary(poolID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// handleExpansion handles expansion recommendation requests
func (h *Handlers) handleExpansion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		http.Error(w, "Missing pool_id parameter", http.StatusBadRequest)
		return
	}

	recommendation, err := h.manager.GetPredictor().PredictExpansion(poolID, 90)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recommendation)
}

// handlePools handles pool management requests
func (h *Handlers) handlePools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List pools
		h.manager.mu.RLock()
		pools := make([]PoolConfig, 0, len(h.manager.poolConfigs))
		for _, config := range h.manager.poolConfigs {
			pools = append(pools, config)
		}
		h.manager.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pools)

	case http.MethodPost:
		// Register new pool
		var config PoolConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		h.manager.RegisterPool(config)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(config)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCollect handles manual data collection requests
func (h *Handlers) handleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		PoolID     string     `json:"pool_id"`
		MetricType MetricType `json:"metric_type"`
		Value      float64    `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.manager.GetCollector().CollectManual(request.PoolID, request.MetricType, request.Value)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "collected"})
}
