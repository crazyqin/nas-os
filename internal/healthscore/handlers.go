package healthscore

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handlers provides HTTP handlers for the health score API.
type Handlers struct {
	hs *HealthScore
}

// NewHandlers creates new HTTP handlers.
func NewHandlers(hs *HealthScore) *Handlers {
	return &Handlers{hs: hs}
}

// RegisterRoutes registers the HTTP routes.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/healthscore/report", h.handleReport)
	mux.HandleFunc("/api/v1/healthscore/history", h.handleHistory)
	mux.HandleFunc("/api/v1/healthscore/trend", h.handleTrend)
	mux.HandleFunc("/api/v1/healthscore/worst", h.handleWorst)
	mux.HandleFunc("/api/v1/healthscore/distribution", h.handleDistribution)
	mux.HandleFunc("/api/v1/healthscore/weights", h.handleWeights)
}

// handleReport handles report generation requests.
func (h *Handlers) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	report, err := h.hs.GenerateReport()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleHistory handles history requests.
func (h *Handlers) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100 // Default limit
	history := h.hs.GetHistory(limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// handleTrend handles trend analysis requests.
func (h *Handlers) handleTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	duration := 7 * 24 * time.Hour // Default 7 days
	trend := h.hs.GetAnalyzer().AnalyzeTrend(duration)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trend)
}

// handleWorst handles worst components requests.
func (h *Handlers) handleWorst(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 3 // Default top 3 worst
	worst := h.hs.GetAnalyzer().GetWorstComponents(limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(worst)
}

// handleDistribution handles distribution requests.
func (h *Handlers) handleDistribution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	distribution := h.hs.GetAnalyzer().GetScoreDistribution()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(distribution)
}

// handleWeights handles weight configuration requests.
func (h *Handlers) handleWeights(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.hs.mu.RLock()
		weights := make(map[ComponentType]float64)
		for k, v := range h.hs.weights {
			weights[k] = v
		}
		h.hs.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(weights)

	case http.MethodPost:
		var weights map[ComponentType]float64
		if err := json.NewDecoder(r.Body).Decode(&weights); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		h.hs.SetWeights(weights)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
