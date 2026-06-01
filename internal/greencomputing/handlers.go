package greencomputing

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Handler handles HTTP requests for green computing
type Handler struct {
	manager *Manager
}

// NewHandler creates a new green computing handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/green/energy/reading", h.handleRecordReading)
	mux.HandleFunc("/api/v1/green/energy/latest", h.handleLatestReading)
	mux.HandleFunc("/api/v1/green/energy/readings", h.handleGetReadings)
	mux.HandleFunc("/api/v1/green/carbon/footprint", h.handleFootprint)
	mux.HandleFunc("/api/v1/green/carbon/daily", h.handleDailyFootprint)
	mux.HandleFunc("/api/v1/green/carbon/weekly", h.handleWeeklyFootprint)
	mux.HandleFunc("/api/v1/green/carbon/monthly", h.handleMonthlyFootprint)
	mux.HandleFunc("/api/v1/green/sleep/strategies", h.handleStrategies)
	mux.HandleFunc("/api/v1/green/sleep/strategies/", h.handleStrategyByID)
	mux.HandleFunc("/api/v1/green/report", h.handleReport)
	mux.HandleFunc("/api/v1/green/score", h.handleGreenScore)
}

func (h *Handler) handleRecordReading(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reading EnergyReading
	if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.manager.RecordReading(&reading)
	writeJSON(w, http.StatusCreated, reading)
}

func (h *Handler) handleLatestReading(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reading := h.manager.GetLatestReading()
	if reading == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "no data"})
		return
	}
	writeJSON(w, http.StatusOK, reading)
}

func (h *Handler) handleGetReadings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	var start, end time.Time

	now := time.Now()
	switch period {
	case "hour":
		start = now.Add(-1 * time.Hour)
		end = now
	case "day":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1)
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 7)
	default:
		start = now.Add(-1 * time.Hour)
		end = now
	}

	readings := h.manager.GetReadings(start, end)
	writeJSON(w, http.StatusOK, readings)
}

func (h *Handler) handleFootprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	var footprint *CarbonFootprint

	switch period {
	case "weekly":
		footprint = h.manager.GetWeeklyFootprint()
	case "monthly":
		footprint = h.manager.GetMonthlyFootprint()
	default:
		footprint = h.manager.GetDailyFootprint()
	}

	writeJSON(w, http.StatusOK, footprint)
}

func (h *Handler) handleDailyFootprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetDailyFootprint())
}

func (h *Handler) handleWeeklyFootprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetWeeklyFootprint())
}

func (h *Handler) handleMonthlyFootprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetMonthlyFootprint())
}

func (h *Handler) handleStrategies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		strategies := h.manager.ListStrategies()
		writeJSON(w, http.StatusOK, strategies)
	case http.MethodPost:
		var req struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			IdleThreshold int    `json:"idle_threshold"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		strategy := h.manager.CreateStrategy(req.Name, req.Description, time.Duration(req.IdleThreshold)*time.Minute)
		writeJSON(w, http.StatusCreated, strategy)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleStrategyByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/green/sleep/strategies/")
	if id == "" {
		http.Error(w, "Missing strategy ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		strategy, ok := h.manager.GetStrategy(id)
		if !ok {
			http.Error(w, "Strategy not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, strategy)
	case http.MethodPut:
		var updates SleepStrategy
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdateStrategy(id, &updates); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	case http.MethodDelete:
		if err := h.manager.DeleteStrategy(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "daily"
	}

	report := h.manager.GenerateEfficiencyReport(period)
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleGreenScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	score := h.manager.GetGreenScore()
	writeJSON(w, http.StatusOK, score)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
