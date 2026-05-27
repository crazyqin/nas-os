package digitalwellbeing

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handler handles HTTP requests for digital wellbeing
type Handler struct {
	manager *Manager
}

// NewHandler creates a new digital wellbeing handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/wellbeing/usage", h.handleUsage)
	mux.HandleFunc("/api/v1/wellbeing/daily", h.handleDaily)
	mux.HandleFunc("/api/v1/wellbeing/goals", h.handleGoals)
	mux.HandleFunc("/api/v1/wellbeing/focus", h.handleFocus)
	mux.HandleFunc("/api/v1/wellbeing/parental", h.handleParental)
	mux.HandleFunc("/api/v1/wellbeing/report", h.handleReport)
	mux.HandleFunc("/api/v1/wellbeing/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/wellbeing/stats", h.handleStats)
}

// handleUsage handles usage recording
func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var record UsageRecord
		if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.RecordUsage(&record); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(record)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDaily handles daily usage requests
func (h *Handler) handleDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	
	dateStr := r.URL.Query().Get("date")
	date := time.Now()
	if dateStr != "" {
		var err error
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
	}
	
	usage := h.manager.GetDailyUsage(userID, date)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usage)
}

// handleGoals handles goals management
func (h *Handler) handleGoals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		h.manager.mu.RLock()
		goals := make([]*UsageGoal, 0)
		for _, g := range h.manager.goals {
			if userID == "" || g.UserID == userID {
				goals = append(goals, g)
			}
		}
		h.manager.mu.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"goals": goals,
			"total": len(goals),
		})
	case http.MethodPost:
		var goal UsageGoal
		if err := json.NewDecoder(r.Body).Decode(&goal); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.SetGoal(&goal); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(goal)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFocus handles focus mode management
func (h *Handler) handleFocus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		modes := make([]*FocusMode, 0)
		for _, m := range h.manager.focusModes {
			modes = append(modes, m)
		}
		h.manager.mu.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"modes": modes,
			"total": len(modes),
		})
	case http.MethodPost:
		var mode FocusMode
		if err := json.NewDecoder(r.Body).Decode(&mode); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.EnableFocusMode(&mode); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(mode)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleParental handles parental control management
func (h *Handler) handleParental(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}
		
		h.manager.mu.RLock()
		control, ok := h.manager.parental[userID]
		h.manager.mu.RUnlock()
		
		if !ok {
			http.Error(w, "No parental control found", http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(control)
	case http.MethodPost:
		var control ParentalControl
		if err := json.NewDecoder(r.Body).Decode(&control); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.SetParentalControl(&control); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(control)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReport handles wellness report generation
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "weekly"
	}
	
	report := h.manager.GenerateReport(userID, period)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleAlerts handles alerts listing
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	userID := r.URL.Query().Get("user_id")
	h.manager.mu.RLock()
	alerts := make([]*WellnessAlert, 0)
	for _, a := range h.manager.alerts {
		if userID == "" || a.UserID == userID {
			alerts = append(alerts, a)
		}
	}
	h.manager.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// handleStats handles statistics requests
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
